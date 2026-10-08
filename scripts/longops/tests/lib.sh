#!/usr/bin/env bash
# lib.sh - shared helpers of the scripts/longops tests (WP-08, T088/T089). Real processes, real flock, real /proc, scratch state only:
# no test writes the real .audit/. HOST-SIDE RUN NOTICE (UNCONFIRMED container leg): RUNP/IMG-TESTUTIL (T007/T008) do not exist yet,
# so the tests run on the host with coreutils, jq, flock and git; the identity header says so.
TROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
S=${LONGOPS_SCRIPTS:-$TROOT/scripts/longops}
PASS=0; FAIL=0; KILLME=(); KILL9ME=()   # KILL9ME: a process a test made ignore TERM on purpose (killed with KILL, one pid, never a group)
FX=$(mktemp -d "${TMPDIR:-/tmp}/longops-test.XXXXXX")
cleanup() { local p; for p in "${KILLME[@]:-}"; do [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && kill "$p" 2>/dev/null; done; for p in "${KILL9ME[@]:-}"; do [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && kill -9 "$p" 2>/dev/null; done; rm -rf "$FX"; }
trap cleanup EXIT
ok()  { PASS=$((PASS+1)); echo "ok   $*"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $*"; [ "${MUT_FAILFAST:-0}" != 1 ] || { echo "RESULT pass=$PASS fail=$FAIL (fail-fast: a mutant run stops at its first failure)"; exit 1; }; }
assert_eq() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 :: got [$2] want [$3]"; fi; }
assert_rc() { if [ "$2" -eq "$3" ]; then ok "$1"; else bad "$1 :: rc=$2 want $3"; fi; }
finish() { echo "RESULT pass=$PASS fail=$FAIL"; [ "$FAIL" -eq 0 ]; }
# ident_header <task>: the identity of EVERY input of the run (11.4.276 round 5, LO-P2 a): the scripts under test, the sweep and its catalogue, every test file and runner of both trees, this library, and every producer
# a test extracts verbatim (scripts/build/dispatch.sh reg_adopt, scripts/repo/{verify_repos,check_no_ci}.sh). A transcript is evidence of the files it names; a file it does not name is not covered.
ident_header() {
  echo "# identity: task=${1:-?} utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) scripts=$S"
  echo "# git_head=$(git -C "$TROOT" rev-parse HEAD 2>/dev/null) (scripts and tests may be uncommitted: the content hashes below identify them)"
  local f; for f in lib register acquire release heartbeat holder classify reap check_no_build_writing_tracked require_verdicts; do
    printf '# sha256 %s.sh=%s\n' "$f" "$(sha256sum "$S/$f.sh" 2>/dev/null | cut -c1-64)"; done
  local d; for d in "$TROOT/scripts/longops/tests" "$TROOT/scripts/anti-mess/tests"; do
    for f in "$d"/*.sh; do [ -e "$f" ] || continue; printf '# sha256 %s=%s\n' "${f#"$TROOT"/}" "$(sha256sum "$f" | cut -c1-64)"; done; done
  for f in scripts/anti-mess/sweep.sh scripts/anti-mess/catalogue.yaml scripts/build/dispatch.sh scripts/repo/verify_repos.sh scripts/repo/check_no_ci.sh; do
    printf '# sha256 %s=%s\n' "$f" "$(sha256sum "$TROOT/$f" 2>/dev/null | cut -c1-64)"; done
  echo "# jq=$(jq --version) flock=$(flock --version 2>&1 | head -1) container_image_digest=UNCONFIRMED (RUNP/IMG-TESTUTIL absent: host-side run, T007/T008 pending)"
}
# fixture: new isolated state; sets LONGOPS_* for the process
newfx() {
  FXN=$FX/$1; rm -rf "$FXN"; mkdir -p "$FXN/repo/.audit" "$FXN/repo/specs/001-full-project-audit-remediation/evidence" "$FXN/repo/specs/001-full-project-audit-remediation/audit" "$FXN/repo/tracked"
  export LONGOPS_REPO=$FXN/repo LONGOPS_DIR=$FXN/repo/.audit/longops LONGOPS_AUDIT=$FXN/repo/.audit LONGOPS_ALLOW_TMPFS=1
  unset LONGOPS_NOW LONGOPS_MONO LONGOPS_TEST_SLEEP_IN_CS CPA_APPROVED_DIR CPA_RUN CPA_RUN_ID LONGOPS_BUILDS LONGOPS_EV LONGOPS_AUD LONGOPS_RESUME_TTL LONGOPS_REAP_GRACE_S LONGOPS_PODMAN_TIMEOUT_S
  # WF14 R2-T2: NO test ever reaches the real podman (a mutant that drops a label filter would `podman stop` the first operator container of the host): the default is a null stub
  # that lists nothing and records any other call; a test that needs a listing sets its own LONGOPS_PODMAN.
  printf '#!/bin/sh\ncase "$1" in ps) exit 0 ;; *) echo "$*" >>"%s/podman-null.calls" ;; esac\nexit 0\n' "$FXN" >"$FXN/podman-null"; chmod +x "$FXN/podman-null"; export LONGOPS_PODMAN=$FXN/podman-null
  # round 5 (LO-T1, LO-D4): the reap grace is NO LONGER forced to 2 s for every fixture: the DEFAULT (15 s) is what runs, a test that waits for a survivor sets its own short grace with LONGOPS_REAP_GRACE_S
  git -C "$FXN/repo" init -q 2>/dev/null; echo t >"$FXN/repo/tracked/a.txt"
  git -C "$FXN/repo" -c user.email=t@t -c user.name=t add tracked/a.txt; git -C "$FXN/repo" -c user.email=t@t -c user.name=t commit -qm fx
}
# mkll: start a sleeper, set LL to its pid (fds redirected: never inside a $(...) capture, the 11.4.201(12) pipe footgun)
mkll() { sleep 600 >/dev/null 2>&1 & LL=$!; KILLME+=("$LL"); }
pstart() { local s; { IFS= read -r -d '' s; } 2>/dev/null <"/proc/$1/stat"; s=${s##*) }; read -r -a _pf <<<"$s"; echo "${_pf[19]:-}"; }
bootid() { local b; IFS= read -r b </proc/sys/kernel/random/boot_id; echo "$b"; }
# mkop <id> <purpose> <state> [pid] [last_progress] [extra-jq-filter]: write a COMPLETE, shape-valid op record straight into the fixture registry (a hand-written producer-shaped record, for states no script produces).
# pid defaults to a dead pid; last_progress is both the epoch and the monotonic value.
mkop() {
  local id=$1 p=$2 st=$3 pid=${4:-999999} lp=${5:-0} extra=${6:-.} sst
  sst=$(pstart "$pid"); [ -n "$sst" ] || sst=1
  mkdir -p "$LONGOPS_DIR/ops"
  jq -nc --arg id "$id" --arg p "$p" --arg s "$st" --argjson pid "$pid" --arg st "$sst" --arg bid "$(bootid)" --argjson lp "$lp" \
    '{op_id:$id,purpose_key:$p,owner:"fx",run_id:$id,pid:$pid,pgid:0,start_time:$st,boot_id:$bid,cmdline:"fx",container_id:"",container_label:$id,state:$s,started_utc:"2026-01-01T00:00:00Z",last_heartbeat_utc:"2026-01-01T00:00:00Z",heartbeat_seq:0,progress_offset:0,last_progress_epoch:$lp,last_progress_mono:$lp,log_path:"",budget:{memory_bytes:0,cpus:0,wall_clock_s:0,no_progress_s:30},write_paths:[],verdict:"",evidence_path:""}' | jq -c "$extra" >"$LONGOPS_DIR/ops/$id.json"
}
# mkholder <purpose> <run_id> <pid> [extra-jq-filter]: a COMPLETE process holder record (claims/<purpose>/holder.json)
mkholder() {
  local p=$1 run=$2 pid=$3 extra=${4:-.} sst; sst=$(pstart "$pid"); [ -n "$sst" ] || sst=1
  mkdir -p "$LONGOPS_DIR/claims/$p"
  jq -nc --arg p "$p" --arg r "$run" --argjson pid "$pid" --arg st "$sst" --arg bid "$(bootid)" '{kind:"process",purpose:$p,run_id:$r,pid:$pid,start_time:$st,boot_id:$bid,cmdline:"fx",state:"running",since:0}' | jq -c "$extra" >"$LONGOPS_DIR/claims/$p/holder.json"
}
# upt: the monotonic clock in whole seconds (what the registry judges liveness on); monoago <s>: that clock <s> seconds ago (a record whose progress is <s> old when the fixture does not set LONGOPS_NOW)
upt() { local u; IFS=. read -r u _ </proc/uptime; echo "$u"; }
monoago() { echo $(( $(upt) - $1 )); }
# a podman stand-in with STATE (registry side): `ps --filter label=L` lists the ids of the files $PODDIR/c-<id> whose lines contain L exactly, `stop` removes the container unless $PODDIR/stubborn exists (and
# exits 125 when $PODDIR/stop-rc125 exists), `ps` fails when $PODDIR/ps-fails exists; every call is logged to $PODDIR/calls.
cat >"$FX/podman-files" <<'PE'
#!/usr/bin/env bash
echo "$*" >>"$PODDIR/calls"
case "$1" in
  ps) [ ! -e "$PODDIR/ps-fails" ] || exit 1
      lab=""; for a in "$@"; do case "$a" in label=*) lab=${a#label=} ;; esac; done
      for f in "$PODDIR"/c-*; do [ -e "$f" ] || continue; grep -qx -- "$lab" "$f" && echo "${f##*/c-}"; done; exit 0 ;;
  stop) id=${@: -1}; [ ! -e "$PODDIR/stop-sleep" ] || sleep "$(cat "$PODDIR/stop-sleep")"; [ -e "$PODDIR/stubborn" ] || rm -f "$PODDIR/c-$id"; [ ! -e "$PODDIR/stop-rc125" ] || exit 125; exit 0 ;;
esac
exit 0
PE
chmod +x "$FX/podman-files"
fpod() { export PODDIR=$FXN/pod LONGOPS_PODMAN=$FX/podman-files; mkdir -p "$PODDIR"; : >"$PODDIR/calls"; }
# fcont <id> <label>...: a running container carrying those labels
fcont() { local id=$1; shift; printf '%s\n' "$@" >"$PODDIR/c-$id"; }
