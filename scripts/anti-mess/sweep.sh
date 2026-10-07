#!/usr/bin/env bash
# sweep.sh - the anti-mess control plane sweep: re-derive the ACTUAL persistent state and diff it against the declared invariant
# catalogue (T090/T091; constitution 11.4.233 B, C, E, F; docs/12 section 17; docs/16 section 13.3). Level-triggered: it reads the
# state, never an event. It detects and, only on request and only for catalogued auto-safe classes, reconciles; it never builds, commits,
# merges or pushes (11.4.233 F: control plane and data plane stay apart). It contacts a remote ONLY in INV-9 (`git ls-remote`, to prove a held commit is on every reachable remote tip of main before a finished
# commit-push run directory is reported removable or removed, WF14 R2-D2); every other invariant reads the local state only.
#
# Usage   sweep.sh [--stage S0|S7|cadence] [--paths-from FILE] [--repo PATH] [--reconcile] [--json OUT] [--only AM-R1,AM-R2,...]
#   --stage        S0 (commit-push preflight: AM-R1 excludes the declared change set; a blocking core.hooksPath is refused with 20),
#                  S7 (verify_clean: nothing excluded), cadence (default: every catalogued invariant, the standing run)
#   --paths-from   the declared change set (one path per line relative to the main repository root, optionally TAB and a verdict path); S0 only
#   --repo         scope AM-R1 to this repository path (a --repo run)
#   --reconcile    repair the catalogued AM classes whose `reconcile` is auto-safe (stale git locks, dead-owner registry rows, orphan labelled
#                  containers, leftover build temp dirs, uninitialised submodules, finished commit-push runs beyond retention); everything else is reported only
#   --json OUT     write the report (anti-mess-sweep/1); the table always goes to stdout
# Every detector is a guard (11.4.201): before it is trusted on the real state it is run on a seeded fixture (a control needle: the seeded
#   drift MUST be reported) and on a golden-false-with-carrier fixture (the carrier MUST NOT be reported); a detector that fails either is
#   BLIND and the sweep exits 20 rather than print a clean report. A catalogued invariant with no detector is `not_evaluated` with its reason, never clean.
# Env     ANTIMESS_ROOT (repository to sweep, default this one), LONGOPS_* (registry, see scripts/longops/lib.sh), ANTIMESS_OWNED_ORGS (fixtures),
#         ANTIMESS_LOCK_MIN_AGE (seconds a git lock must exist before it can be stale, default 60), ANTIMESS_ORPHAN_AGE_S (default 300),
#         CPA_APPROVED_DIR (the approved copy of the CPA run being served: S0, S7), PODMAN via LONGOPS_PODMAN.
#         ANTIMESS_TEST_BEFORE_ACTION (a script run before every reconcile action, with ANTIMESS_TEST_MODE=1 only) lets a test move the state between detection and action.
# Exits   0 no drift; 10 drift reported; 11 a source could not be read and no drift was found (never read as clean: a corrupt op record, podman failing, the verifier
#         giving no report); 20 refusal (usage, an unknown --only id, a blind detector, a blocking core.hooksPath at S0). Reading only unless --reconcile.
# Reconcile  a container is stopped ONLY when its op is in THIS registry and terminal (WF14 R2-11): the registry is per checkout and the podman namespace is per user, so a container whose op is
#         absent here may belong to another checkout, track or scratch copy; it is reported (orphan_container) and never stopped. Absence from one registry is not proof of staleness (11.4.232 E).
#         every action RE-VERIFIES its precondition at action time (WF11 F10): rmlock, stopcontainer (the container's op class is re-derived from the current registry),
#         rmdir (build_tmp: owner still gone; finished_run: report.json still finished and every held commit still on a remote tip), initsub (still uninitialised), reapop
#         (reap.sh re-classifies under the purpose lock). A `handoff` op is re-adoptable (11.4.232 D): its container is never stopped.
set -u
LC_ALL=C
SD=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TOOLS=$(cd "$SD/../.." && pwd)
AM_ROOT=${ANTIMESS_ROOT:-$TOOLS}
export LONGOPS_REPO=${LONGOPS_REPO:-$AM_ROOT}
. "$TOOLS/scripts/longops/lib.sh"
CATALOGUE=${ANTIMESS_CATALOGUE:-$SD/catalogue.yaml}
UNREAD=0
STAGE=cadence; PATHS_FROM=""; ARG_REPO=""; RECON=0; JSON_OUT=""; ONLY=""
die() { echo "sweep: $1: $2" >&2; exit 20; }
while [ $# -gt 0 ]; do
  case "$1" in
    --stage) STAGE=${2:-}; shift 2 ;; --paths-from) PATHS_FROM=${2:-}; shift 2 ;; --repo) ARG_REPO=${2:-}; shift 2 ;;
    --reconcile) RECON=1; shift ;; --json) JSON_OUT=${2:-}; shift 2 ;; --only) ONLY=,${2:-}, ; shift 2 ;;
    *) die usage "unknown argument $(printf '%q' "$1")" ;;
  esac
done
case "$STAGE" in S0|S7|cadence) ;; *) die usage "--stage must be S0, S7 or cadence" ;; esac
[ -z "${ANTIMESS_TEST_BEFORE_ACTION:-}" ] || [ "${ANTIMESS_TEST_MODE:-}" = 1 ] || die test_hook_outside_test_mode "ANTIMESS_TEST_BEFORE_ACTION is a test hook and needs ANTIMESS_TEST_MODE=1"
[ -z "$PATHS_FROM" ] || [ "$STAGE" = S0 ] || die usage "--paths-from is for --stage S0 only"
[ -z "$PATHS_FROM" ] || [ -r "$PATHS_FROM" ] || die usage "--paths-from file unreadable"
[ -d "$AM_ROOT/.git" ] || [ -f "$AM_ROOT/.git" ] || die not_a_repository "$AM_ROOT"
W=$(mktemp -d "${TMPDIR:-/tmp}/antimess.XXXXXX") || die internal mktemp
trap 'rm -rf "$W"' EXIT
F=$'\t'
emit() { printf '%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" "${5:-}"; }   # severity class subject evidence [action]
now() { lo_now; }
mkrepo() { git init -q "$1" 2>/dev/null && git -C "$1" config user.email a@a && git -C "$1" config user.name a && git -C "$1" config commit.gpgsign false; }
commit_all() { git -C "$1" add -A && git -C "$1" -c core.hooksPath=/dev/null commit -qm "${2:-fx}" 2>/dev/null; }

# ---------- shared: the repository verifier (the single home of dirty/excepted, cached per root) ----------
VR_RC=0
vr_json() {
  local out="$W/vr.$(printf '%s' "$AM_ROOT" | sha256sum | cut -c1-12).json"
  if [ ! -s "$out" ]; then
    local args=(--root "$AM_ROOT" --no-remote --quiet --json "$out" --exceptions "${AM_EXC:-$TOOLS/scripts/repo/exceptions.tsv}")
    [ -z "${ANTIMESS_OWNED_ORGS:-}" ] || args+=(--owned-orgs "$ANTIMESS_OWNED_ORGS")
    ( cd "$AM_ROOT" && bash "$TOOLS/scripts/repo/verify_repos.sh" "${args[@]}" >/dev/null 2>"$W/vr.err" ); VR_RC=$?
  fi
  [ -s "$out" ] && jq -e '.repos' "$out" >/dev/null 2>&1 && { echo "$out"; return 0; }
  return 1
}
declared_paths() { [ -n "$PATHS_FROM" ] && cut -f1 "$PATHS_FROM" | sed '/^[[:space:]]*$/d;/^#/d'; }

# ---------- AM-R1 clean working tree ----------
det_AM_R1() {
  local j; j=$(vr_json) || { emit unread verifier - "scripts/repo/verify_repos.sh gave no report (rc=$VR_RC): $(head -c 160 "$W/vr.err" 2>/dev/null)"; return; }
  local path tr un decl="" f rel rest n=0
  [ -z "$PATHS_FROM" ] || decl=$(declared_paths)
  while IFS=$'\t' read -r path tr un; do
    if [ -n "$ARG_REPO" ]; then case "$path/" in "$ARG_REPO"/|"$ARG_REPO"/*) ;; *) continue ;; esac; fi
    if [ "$STAGE" = S0 ] && [ -n "$PATHS_FROM" ]; then
      rest=""; n=0
      while IFS= read -r -d '' ent; do
        f=${ent:3}; case "${ent:0:1}${ent:1:1}" in R*|C*|*R|*C) IFS= read -r -d '' _skip ;; esac
        [ "$path" = . ] && rel=$f || rel=$path/$f
        grep -qxF -- "$rel" <<<"$decl" || { n=$((n+1)); [ "$n" -le 5 ] && rest="$rest $rel"; }
      done < <(git -C "$AM_ROOT/$path" status --porcelain=v1 -z --ignore-submodules=all -uall 2>/dev/null)
      if [ "$n" -gt 0 ]; then emit drift dirty_outside_change_set "$path" "$n file(s) outside the declared change set, first:$rest"
      else emit info declared_change_set_only "$path" "every change is in the declared change set"; fi
    else
      emit drift dirty_repository "$path" "tracked=$tr untracked=$un (not excepted by scripts/repo/exceptions.tsv)"
    fi
  done < <(jq -r '.repos[]|select(.dirty==true and .excepted!=true)|[.path,.dirty_tracked,.dirty_untracked]|@tsv' <<<"$(cat "$j")")
  local pp="$AM_ROOT/.audit/pending_pins.tsv" st
  while IFS=$'\t' read -r path st; do
    if [ -s "$pp" ] && cut -f1 "$pp" | grep -qxF -- "$path"; then emit info pending_pin_move "$path" "pin_state=$st equals a row of .audit/pending_pins.tsv (a pending pin move, never dirt)"
    else emit info "pin_state_$st" "$path" "pointer state belongs to the pin rules, not to AM-R1"; fi
  done < <(jq -r '.repos[]|select((.pin_state//"ok")!="ok")|[.path,.pin_state]|@tsv' <<<"$(cat "$j")")
}
needle_AM_R1() {
  local n="$W/nd-r1"; mkdir -p "$n"; mkrepo "$n/a"; echo 1 >"$n/a/f.txt"; commit_all "$n/a"; echo 2 >"$n/a/f.txt"
  mkrepo "$n/b"; echo 1 >"$n/b/f.txt"; commit_all "$n/b"
  local o
  o=$( AM_ROOT=$n/a; ANTIMESS_OWNED_ORGS=needleorg; STAGE=cadence; PATHS_FROM=""; ARG_REPO=""; det_AM_R1 ); [ "$(grep -c '^drift' <<<"$o")" -eq 1 ] || { echo "seeded dirty tree not reported: [$o]"; return 1; }
  o=$( AM_ROOT=$n/b; ANTIMESS_OWNED_ORGS=needleorg; STAGE=cadence; PATHS_FROM=""; ARG_REPO=""; det_AM_R1 ); [ "$(grep -c '^drift' <<<"$o")" -eq 0 ] || { echo "clean tree reported: [$o]"; return 1; }
  printf 'f.txt\n' >"$n/cs"
  o=$( AM_ROOT=$n/a; ANTIMESS_OWNED_ORGS=needleorg; STAGE=S0; PATHS_FROM=$n/cs; ARG_REPO=""; det_AM_R1 ); [ "$(grep -c '^drift' <<<"$o")" -eq 0 ] || { echo "S0 declared change set reported: [$o]"; return 1; }
  printf 'other.txt\n' >"$n/cs2"
  o=$( AM_ROOT=$n/a; ANTIMESS_OWNED_ORGS=needleorg; STAGE=S0; PATHS_FROM=$n/cs2; ARG_REPO=""; det_AM_R1 ); [ "$(grep -c '^drift' <<<"$o")" -eq 1 ] || { echo "S0 undeclared change not reported: [$o]"; return 1; }
}

# ---------- AM-R2 stale git locks ----------
_open_by_any() {  # _open_by_any <file>: prints `pid cmdline` of a process holding it open (own uid; /proc fd links), nothing otherwise
  local l p; l=$(find /proc/[0-9]*/fd -maxdepth 1 -lname "$1" -print -quit 2>/dev/null); [ -n "$l" ] || return 1
  p=${l#/proc/}; p=${p%%/*}; echo "$p $(lo_cmdline "$p")"
}
_git_active() {  # count of git processes whose working directory is inside the swept repository (resolved from /proc/<pid>/cwd)
  local f p c n=0; for f in $(grep -l -E '^git($|-)' /proc/[0-9]*/comm 2>/dev/null); do p=${f#/proc/}; p=${p%%/*}; c=$(readlink "/proc/$p/cwd" 2>/dev/null) || continue
    case "$c/" in "$AM_ROOT"/*) n=$((n+1)) ;; esac; done; echo "$n"
}
det_AM_R2() {
  local gd; gd=$(git -C "$AM_ROOT" rev-parse --absolute-git-dir 2>/dev/null) || { emit unread git_dir "$AM_ROOT" "git rev-parse --absolute-git-dir failed"; return; }
  local minage=${ANTIMESS_LOCK_MIN_AGE:-60} lk age h act now_; now_=$(now); act=$(_git_active)
  while IFS= read -r -d '' lk; do
    age=$(( now_ - $(stat -c %Y -- "$lk") ))
    if h=$(_open_by_any "$lk"); then emit info lock_held_open "$lk" "held open by pid ${h%% *} cmdline '${h#* }' (a live holder)"
    elif [ "$act" -gt 0 ]; then emit info lock_with_git_activity "$lk" "$act git process(es) running; holder not provable dead, left alone (conservative-safe)"
    elif [ "$age" -lt "$minage" ]; then emit info lock_young "$lk" "age ${age}s < ${minage}s"
    else emit drift stale_git_lock "$lk" "age ${age}s, no process has it open, no git process is running (resolved from /proc)" "rmlock:$lk"; fi
  done < <(find "$gd" -name '*.lock' -type f -print0 2>/dev/null)
}
needle_AM_R2() {
  local n="$W/nd-r2"; mkdir -p "$n"; mkrepo "$n/a"; echo 1 >"$n/a/f"; commit_all "$n/a"
  : >"$n/a/.git/index.lock"; touch -d '10 minutes ago' "$n/a/.git/index.lock"
  mkdir -p "$n/a/.git/modules/m"; : >"$n/a/.git/modules/m/HEAD.lock"; touch -d '10 minutes ago' "$n/a/.git/modules/m/HEAD.lock"
  local o; o=$( AM_ROOT=$n/a; _git_active() { echo 0; }; det_AM_R2 ); [ "$(grep -c '^drift' <<<"$o")" -eq 2 ] || { echo "seeded stale locks (index + submodule gitdir) not both reported: [$o]"; return 1; }
  # carrier 1: a lock held open by a live process; carrier 2: a young lock; carrier 3: a file named like a lock but not under .git
  mkrepo "$n/b"; echo 1 >"$n/b/f"; commit_all "$n/b"; : >"$n/b/.git/index.lock"; touch -d '10 minutes ago' "$n/b/.git/index.lock"
  exec 7<"$n/b/.git/index.lock"
  o=$( AM_ROOT=$n/b; _git_active() { echo 0; }; det_AM_R2 ); exec 7<&-
  [ "$(grep -c '^drift' <<<"$o")" -eq 0 ] && grep -q lock_held_open <<<"$o" || { echo "a lock held open by a live process was reported stale: [$o]"; return 1; }
  mkrepo "$n/c"; echo 1 >"$n/c/f"; commit_all "$n/c"; : >"$n/c/.git/index.lock"; : >"$n/c/decoy.lock"
  o=$( AM_ROOT=$n/c; _git_active() { echo 0; }; det_AM_R2 ); [ "$(grep -c '^drift' <<<"$o")" -eq 0 ] || { echo "young lock or a non-git .lock file reported: [$o]"; return 1; }
}

# ---------- AM-R5 uninitialised submodules ----------
det_AM_R5() {
  local l; l=$(git -C "$AM_ROOT" submodule status --recursive 2>&1) || { emit unread submodule_status - "git submodule status failed: ${l:0:160}"; return; }
  local line sp
  while IFS= read -r line; do [ "${line:0:1}" = - ] || continue; sp=$(awk '{print $2}' <<<"$line"); emit drift uninitialised_submodule "$sp" "declared in .gitmodules, not checked out (git submodule status leading '-')" "initsub:$sp"; done <<<"$l"
}
needle_AM_R5() {
  local n="$W/nd-r5"; mkdir -p "$n"; mkrepo "$n/sub"; echo 1 >"$n/sub/f"; commit_all "$n/sub"; mkrepo "$n/par"
  git -C "$n/par" -c protocol.file.allow=always submodule add -q "$n/sub" sm 2>/dev/null; commit_all "$n/par" par
  git clone -q "$n/par" "$n/clone" 2>/dev/null
  local o; o=$( AM_ROOT=$n/clone; det_AM_R5 ); [ "$(grep -c '^drift' <<<"$o")" -eq 1 ] || { echo "seeded uninitialised submodule not reported: [$o]"; return 1; }
  o=$( AM_ROOT=$n/par; det_AM_R5 ); [ "$(grep -c '^drift' <<<"$o")" -eq 0 ] || { echo "an initialised submodule reported: [$o]"; return 1; }
}

# ---------- AM-G2 no CI pipeline + non-blocking core.hooksPath ----------
_blocking_hooks_dir() {  # _blocking_hooks_dir <dir>: prints the first executable client-side hook found
  local h; for h in pre-commit prepare-commit-msg commit-msg pre-merge-commit pre-push pre-rebase pre-applypatch applypatch-msg; do [ -f "$1/$h" ] && [ -x "$1/$h" ] && { echo "$1/$h"; return 0; }; done; return 1
}
_g2_roots() { local j; echo "$AM_ROOT"; j=$(vr_json) && jq -r --arg r "$AM_ROOT" '.repos[]|select(.owned==true and .path!=".")|$r+"/"+.path' "$j"; }
det_AM_G2() {
  local r hp d b
  while IFS= read -r r; do
    [ -e "$r/.git" ] || continue
    hp=$(git -C "$r" config --get core.hooksPath 2>/dev/null) || hp=""
    [ -n "$hp" ] || continue
    case "$hp" in /*) d=$hp ;; *) d=$r/$hp ;; esac
    if [ -d "$d" ] && b=$(_blocking_hooks_dir "$d"); then emit drift blocking_hooks_path "$r" "core.hooksPath=$hp holds the executable hook $b (11.4.234: hooks must not gate commit/push)"; fi
  done < <(_g2_roots)
  [ "$STAGE" = cadence ] || return 0
  local roots=() out rc; while IFS= read -r r; do [ -e "$r/.git" ] && roots+=(--root "$r"); done < <(_g2_roots)
  out=$(bash "$TOOLS/scripts/repo/check_no_ci.sh" "${roots[@]}" 2>"$W/ci.err"); rc=$?
  case $rc in
    0) ;;
    10) while IFS=$'\t' read -r _k root rel; do emit drift ci_pipeline_definition "$root/$rel" "11.4.156: a CI pipeline definition (scripts/repo/check_no_ci.sh, the single home of the condition)"; done <<<"$out" ;;
    *) emit unread check_no_ci - "scripts/repo/check_no_ci.sh exit $rc: $(head -c 160 "$W/ci.err")" ;;
  esac
}
needle_AM_G2() {
  local n="$W/nd-g2"; mkdir -p "$n"; mkrepo "$n/a"; echo 1 >"$n/a/f"; mkdir -p "$n/a/.github/workflows" "$n/a/blk"; echo 'name: x' >"$n/a/.github/workflows/ci.yml"
  printf '#!/bin/sh\nexit 1\n' >"$n/a/blk/pre-push"; chmod +x "$n/a/blk/pre-push"; commit_all "$n/a"; git -C "$n/a" config core.hooksPath blk
  local o; o=$( AM_ROOT=$n/a; ANTIMESS_OWNED_ORGS=needleorg; STAGE=cadence; det_AM_G2 )
  grep -q ci_pipeline_definition <<<"$o" && grep -q blocking_hooks_path <<<"$o" || { echo "planted workflow file or blocking hooksPath not reported: [$o]"; return 1; }
  mkrepo "$n/b"; echo 1 >"$n/b/f"; mkdir -p "$n/b/.github/workflows" "$n/b/nohooks"; echo 'docs only' >"$n/b/.github/workflows/README.md"; commit_all "$n/b"; git -C "$n/b" config core.hooksPath nohooks
  o=$( AM_ROOT=$n/b; ANTIMESS_OWNED_ORGS=needleorg; STAGE=cadence; det_AM_G2 ); [ -z "$(grep '^drift\|^unread' <<<"$o")" ] || { echo "golden-false carrier (workflows/README.md, empty hooks dir) reported: [$o]"; return 1; }
  o=$( AM_ROOT=$n/a; ANTIMESS_OWNED_ORGS=needleorg; STAGE=S0; det_AM_G2 ); grep -q blocking_hooks_path <<<"$o" && ! grep -q ci_pipeline <<<"$o" || { echo "S0 must run only the hooksPath half: [$o]"; return 1; }
}

# ---------- registry: AM-P1, AM-P2 ----------
# _ops: every readable op record, one compact JSON per line, each FILE parsed on its own (a corrupt record never hides the records after it, WF11 F4);
# _ops_unread: the files that are empty, unparsable or have no string state: reported `unread`, never skipped silently.
# a record is READABLE only when it names its op, its purpose and its state as strings (WF14 class D: valid JSON with one missing or mistyped field is unread, never skipped, never clean)
_OPS_OK='type=="object" and (.op_id|type=="string") and (.purpose_key|type=="string") and (.state|type=="string")'
_ops() { local f; for f in "$LD"/ops/*.json; do [ -e "$f" ] || continue; jq -ce "select($_OPS_OK)" "$f" 2>/dev/null; done; }
_ops_unread() { local f; for f in "$LD"/ops/*.json; do [ -e "$f" ] || continue; jq -e "$_OPS_OK" "$f" >/dev/null 2>&1 || echo "$f"; done; }
_emit_unread_ops() { local f; while IFS= read -r f; do [ -n "$f" ] || continue; emit unread corrupt_op_record "$f" "the op record is empty, unparsable or has no string state; the invariant is evaluated on the readable records only"; done < <(_ops_unread); }
# the label the launcher really sets is catalogizer.op_id (scripts/containers/run_pinned.sh); op_id is accepted too
_P1_JQ='.[]?|[(.Id|.[0:12]),((.Labels // {})["catalogizer.op_id"] // (.Labels // {}).op_id // "-"),(.Created // 0)]|@tsv'
_p1_list() { local ps; ps=$(timeout 60 "$PODMAN" ps --filter label=project=catalogizer --format json 2>"$W/pod.err") || return 1; [ -n "$ps" ] || ps='[]'; jq -r "$_P1_JQ" <<<"$ps"; }
# _p1_class <op> <created>: terminal | handoff | live | orphan | young | nolabel | unreadable, from the CURRENT registry (also the re-verification of a stopcontainer action)
# _op_file_of_label <label value>: the record of THIS registry that owns that container label value: the op id itself (runner_lib.sh: label = op id) or a record whose container_label names it
# (dispatch.sh: op id <build id>, label catalogizer.op_id=dispatch-<build id>; WF14 R2-4). Nothing when this registry has no such op.
_op_file_of_label() {
  local v=$1 f
  lo_safe_name "$v" || return 1
  f=$(lo_op_file "$v"); if [ -e "$f" ]; then echo "$f"; return 0; fi
  for f in "$LD"/ops/*.json; do [ -e "$f" ] || continue
    jq -e --arg v "$v" 'type=="object" and ((.container_label // "")|type=="string") and ((.container_label // "") as $c | $c==$v or $c=="catalogizer.op_id="+$v or $c=="op_id="+$v)' "$f" >/dev/null 2>&1 && { echo "$f"; return 0; }
  done; return 1
}
_p1_class() {
  local op=$1 created=$2 f st age
  [ "$op" != - ] && [ -n "$op" ] || { echo nolabel; return; }
  if f=$(_op_file_of_label "$op"); then st=$(jq -r 'if type=="object" then .state // "" else "" end' "$f" 2>/dev/null)
    case "$st" in complete|failed|reaped|blocked-escape) echo terminal ;; handoff) echo handoff ;; "") echo unreadable ;; *) echo live ;; esac; return; fi
  age=$(( $(now) - ${created:-0} ))
  if [ "$age" -gt "${ANTIMESS_ORPHAN_AGE_S:-300}" ]; then echo orphan; else echo young; fi
}
det_AM_P1() {
  local j cls ev id age budget=${ANTIMESS_ORPHAN_AGE_S:-300} line
  _emit_unread_ops
  while IFS= read -r j; do [ -n "$j" ] || continue
    id=$(jq -r .op_id <<<"$j"); line=$(lo_classify_op "$j"); cls=$(sed -n 1p <<<"$line"); ev=$(sed -n 2p <<<"$line")
    case "$cls" in
      hung) emit drift hung_op "$id" "purpose $(jq -r .purpose_key <<<"$j"): $ev" ;;
      dead_owner) emit drift registry_row_dead_owner "$id" "purpose $(jq -r .purpose_key <<<"$j"): $ev" "reapop:$id" ;;
      unreadable) emit unread corrupt_op_record "$id" "$ev" ;;
    esac
  done < <(_ops)
  local lst rc cid op created cc; lst=$(_p1_list); rc=$?
  if [ $rc -ne 0 ]; then emit unread containers_unread podman "podman ps failed or was unparsable (rc $rc): $(head -c 160 "$W/pod.err" 2>/dev/null); the containers were NOT evaluated (never clean)"; return; fi
  while IFS=$'\t' read -r cid op created; do
    [ -n "$cid" ] || continue
    cc=$(_p1_class "$op" "$created"); age=$(( $(now) - ${created:-0} ))
    case "$cc" in
      nolabel) emit drift container_without_op_label "$cid" "labelled project=catalogizer but carries neither a catalogizer.op_id nor an op_id label (cannot be matched to a registry row)" ;;
      terminal) emit drift container_of_terminal_op "$cid" "op $op is terminal but its labelled container still runs" "stopcontainer:$cid:terminal" ;;
      handoff) emit info container_of_handoff_op "$cid" "op $op is handed off (re-adoptable, 11.4.232 D): its container is never stopped by the sweep" ;;
      orphan) emit drift orphan_container "$cid" "op_id=$op has no row in THIS checkout's registry; age ${age}s > ${budget}s; reported only: absence from one registry is not proof of staleness (another checkout, track or scratch copy may own it, 11.4.232 E), stop it by hand once its owner is known" ;;
      young) emit info orphan_container_young "$cid" "op_id=$op has no registry row yet; age ${age}s <= ${budget}s" ;;
      live|unreadable) ;;
    esac
  done <<<"$lst"
}
needle_AM_P1() {
  local n="$W/nd-p1" id1 id2 id3; mkdir -p "$n"
  sleep 300 >/dev/null 2>&1 & local P1=$!; sleep 300 >/dev/null 2>&1 & local P2=$!
  local o ok=0
  o=$( LD=$n/ld; mkdir -p "$LD/ops"; LONGOPS_NOW=1000
       fake_op() { jq -nc --arg id "$1" --arg p "$2" --argjson pid "$3" --arg st "$(lo_pstart "$3")" --argjson lp "$4" --arg s "$5" '{op_id:$id,purpose_key:$p,run_id:$id,pid:$pid,start_time:$st,state:$s,last_progress_epoch:$lp,progress_offset:0,budget:{no_progress_s:30}}' >"$LD/ops/$1.json"; }
       fake_op hungop pa "$P1" 100 running; fake_op goodop pb "$P2" 990 running; fake_op doneop pc "$P2" 1 complete
       printf '{"op_id":"trunc",' >"$LD/ops/00corrupt.json"   # a corrupt record sorted BEFORE the others: it must not hide them (WF11 F4)
       PODMAN=$n/podman-none; det_AM_P1 )
  lo_kill_child "$P1"; lo_kill_child "$P2"
  grep -q "hung_op${F}hungop" <<<"$o" || { echo "seeded hung op (flat offset, live owner) not reported (a corrupt record hid it?): [$o]"; return 1; }
  grep -q "^unread${F}corrupt_op_record${F}.*00corrupt.json" <<<"$o" || { echo "seeded corrupt op record not reported unread: [$o]"; return 1; }
  [ "$(grep -c "^drift" <<<"$o")" -eq 1 ] || { echo "golden-false carriers (advancing op, terminal op) were reported: [$o]"; return 1; }
  # containers (a unit-level stand-in for podman: the real one runs on the real state)
  cat >"$n/podman" <<'PE'
#!/usr/bin/env bash
cat <<'J'
[{"Id":"aaaaaaaaaaaaaaaaaaaa","Labels":{"op_id":"ghost","project":"catalogizer"},"Created":100},{"Id":"bbbbbbbbbbbbbbbbbbbb","Labels":{"op_id":"liveop","project":"catalogizer"},"Created":100},{"Id":"cccccccccccccccccccc","Labels":{"catalogizer.op_id":"liveop","project":"catalogizer"},"Created":100},{"Id":"dddddddddddddddddddd","Labels":{"project":"catalogizer"},"Created":100}]
J
PE
  chmod +x "$n/podman"
  o=$( LD=$n/ld2; mkdir -p "$LD/ops"; LONGOPS_NOW=1000; echo '{"op_id":"liveop","purpose_key":"lp","state":"running","pid":0,"start_time":"","budget":{"no_progress_s":0}}' >"$LD/ops/liveop.json"; PODMAN=$n/podman; det_AM_P1 )
  grep -q "orphan_container${F}aaaaaaaaaaaa" <<<"$o" && ! grep -q "bbbbbbbbbbbb\|cccccccccccc" <<<"$(grep '^drift' <<<"$o" | grep -v registry_row_dead_owner)" || { echo "orphan container not reported or a registered one (op_id or catalogizer.op_id label) reported: [$o]"; return 1; }
  grep -q "container_without_op_label${F}dddddddddddd" <<<"$o" || { echo "a container with no op label at all not reported: [$o]"; return 1; }
}
det_AM_P2() {
  local p
  _emit_unread_ops
  while IFS= read -r p; do [ -n "$p" ] || continue
    emit drift duplicate_owner "$p" "more than one non-terminal operation holds purpose_key $p: $(_dup_ops "$p")"
  done < <(_ops | jq -rs 'map(select(.state as $s|["complete","failed","reaped","handoff","blocked-escape"]|index($s)|not)|select((.attached_to//"")=="" and (.superseded_by//"")==""))|group_by(.purpose_key)|map(select(length>1)|.[0].purpose_key)|.[]')
}
_dup_ops() { _ops | jq -rs --arg p "$1" 'map(select(.purpose_key==$p and (.state as $s|["complete","failed","reaped","handoff","blocked-escape"]|index($s)|not)))|map(.op_id)|join(",")'; }
needle_AM_P2() {
  local o; o=$( LD=$W/nd-p2; mkdir -p "$LD/ops"
    echo '{"op_id":"a1","purpose_key":"dup","state":"running"}' >"$LD/ops/a1.json"; echo '{"op_id":"a2","purpose_key":"dup","state":"running"}' >"$LD/ops/a2.json"
    echo '{"op_id":"b1","purpose_key":"ok","state":"running"}' >"$LD/ops/b1.json"; echo '{"op_id":"c1","purpose_key":"att","state":"running"}' >"$LD/ops/c1.json"; echo '{"op_id":"c2","purpose_key":"att","state":"running","attached_to":"c1"}' >"$LD/ops/c2.json"
    echo '{"op_id":"d1","purpose_key":"term","state":"complete"}' >"$LD/ops/d1.json"; echo '{"op_id":"d2","purpose_key":"term","state":"running"}' >"$LD/ops/d2.json"
    printf '{"op_id":"trunc",' >"$LD/ops/00corrupt.json"
    det_AM_P2 )
  [ "$(grep -c '^drift' <<<"$o")" -eq 1 ] && grep -q "duplicate_owner${F}dup" <<<"$o" || { echo "seeded duplicate owner not reported (a corrupt record hid it?), or a carrier (attached op, terminal twin) reported: [$o]"; return 1; }
  grep -q "^unread${F}corrupt_op_record" <<<"$o" || { echo "seeded corrupt op record not reported unread: [$o]"; return 1; }
}

# ---------- AM-P4 purpose claims and un-adopted handoffs (WF11 F9; 11.4.232 B, D, F) ----------
det_AM_P4() {
  local d p s rc age minage=${ANTIMESS_LOCK_MIN_AGE:-60} op j
  _emit_unread_ops
  for d in "$LD"/claims/*/; do [ -d "$d" ] || continue; p=${d%/}; p=${p##*/}
    s=$(lo_holder_status "$p" 2>"$W/p4.err"); rc=$?
    if [ "$rc" -eq "$RC_REFUSE" ]; then emit unread claim_holder_unread "$p" "the holder of $p could not be judged: $(head -c 160 "$W/p4.err")"; continue; fi
    if [ "$rc" -ne 0 ]; then   # a claim directory with no holder record: a crash between mkdir and the record, unless it is being written right now
      age=$(( $(now) - $(stat -c %Y -- "${d%/}" 2>/dev/null || echo 0) ))
      if [ "$age" -lt "$minage" ]; then emit info claim_young "$p" "claim directory with no holder record, age ${age}s < ${minage}s (a registration in progress)"
      else emit drift claim_without_holder "$p" "claim directory with no holder record, age ${age}s: the purpose is blocked and nothing owns it; release is scripts/longops/reap.sh --purpose $p"; fi
      continue
    fi
    case "$s" in
      dead) emit drift stale_claim "$p" "the holder is dead (resolved from /proc): the purpose is blocked (register exits 4); release is scripts/longops/reap.sh --purpose $p" ;;
      unreadable) emit drift claim_unreadable "$p" "the holder record cannot be read: the purpose is blocked and the claim is never taken over; resolve it by hand" ;;
    esac
  done
  # a handoff op is ADOPTED when the real producer re-registered the purpose: scripts/build/dispatch.sh reg_adopt registers `<id>-aN` with the same purpose_key, started no earlier than the handoff op
  # (WF14 R2-2: no producer writes superseded_by / attached_to / adopted_by; those fields stay accepted when someone does write them). Resolving the op by hand also clears it (it is no longer `handoff`).
  local all; all=$(_ops)
  while IFS= read -r j; do [ -n "$j" ] || continue
    [ "$(jq -r .state <<<"$j")" = handoff ] || continue
    [ -z "$(jq -r '(.superseded_by // "") + (.attached_to // "") + (.adopted_by // "")' <<<"$j")" ] || continue
    op=$(jq -r .op_id <<<"$j")
    [ -n "$(jq -rs --arg id "$op" --arg p "$(jq -r .purpose_key <<<"$j")" --arg s "$(jq -r '.started_utc // ""' <<<"$j")" 'map(select(.purpose_key==$p and .op_id!=$id and ((.started_utc // "") >= $s)))|map(.op_id)|first // empty' <<<"$all")" ] && continue
    emit drift handoff_unadopted "$op" "op $op was handed off (11.4.232 D: re-adoptable) and no later op of the same purpose re-adopted it and nothing resolves it: scripts/longops/release.sh --op-id $op --state <terminal> resolves it"
  done < <(_ops)
}
needle_AM_P4() {
  local n="$W/nd-p4" o; mkdir -p "$n"
  o=$( LD=$n/ld; mkdir -p "$LD/ops" "$LD/claims/stalep" "$LD/claims/nohold" "$LD/claims/badhold" "$LD/claims/livep"; LONGOPS_NOW=$(date +%s)
       sleep 300 >/dev/null 2>&1 & P=$!
       jq -nc '{kind:"process",purpose:"stalep",run_id:"r1",pid:999999,start_time:"1",state:"running"}' >"$LD/claims/stalep/holder.json"
       printf '{"kind":"proc' >"$LD/claims/badhold/holder.json"
       jq -nc --argjson pid "$P" --arg st "$(lo_pstart "$P")" '{kind:"process",purpose:"livep",run_id:"r2",pid:$pid,start_time:$st,state:"running"}' >"$LD/claims/livep/holder.json"
       touch -d '10 minutes ago' "$LD/claims/nohold"
       echo '{"op_id":"h1","purpose_key":"h","state":"handoff"}' >"$LD/ops/h1.json"; echo '{"op_id":"h2","purpose_key":"h2","state":"handoff","superseded_by":"x"}' >"$LD/ops/h2.json"
       echo '{"op_id":"c1","purpose_key":"c","state":"complete"}' >"$LD/ops/c1.json"
       # the REAL re-adoption (dispatch.sh reg_adopt): a later op of the same purpose adopts the handoff op (h3); an EARLIER completed op of the purpose does not (h4)
       echo '{"op_id":"h3","purpose_key":"p3","state":"handoff","started_utc":"2026-01-01T00:00:00Z"}' >"$LD/ops/h3.json"; echo '{"op_id":"h3-a2","purpose_key":"p3","state":"running","started_utc":"2026-01-01T00:00:05Z"}' >"$LD/ops/h3-a2.json"
       echo '{"op_id":"h4","purpose_key":"p4","state":"handoff","started_utc":"2026-01-01T00:00:05Z"}' >"$LD/ops/h4.json"; echo '{"op_id":"p4old","purpose_key":"p4","state":"complete","started_utc":"2026-01-01T00:00:00Z"}' >"$LD/ops/p4old.json"
       det_AM_P4; lo_kill_child "$P" )
  [ "$(grep -c '^drift' <<<"$o")" -eq 5 ] && grep -q "handoff_unadopted${F}h4" <<<"$o" && grep -q "stale_claim${F}stalep" <<<"$o" && grep -q "claim_without_holder${F}nohold" <<<"$o" && grep -q "claim_unreadable${F}badhold" <<<"$o" && grep -q "handoff_unadopted${F}h1" <<<"$o" \
    || { echo "seeded stale claim / holderless claim / unreadable holder / un-adopted handoff (and the one with only an EARLIER op of its purpose) not all reported: [$o]"; return 1; }
  grep '^drift' <<<"$o" | grep -q "livep\|h2\|c1\|h3" && { echo "a golden-false carrier (live holder, superseded handoff, complete op) was reported: [$o]"; return 1; }
  return 0
}

# ---------- AM-P3 builds ----------
det_AM_P3() {
  [ -d "$BUILDS_DIR" ] || return 0
  local d id suf pid st hub=0
  if [ -s "$BUILDS_DIR/hub.json" ]; then lo_alive "$(jq -r .pid "$BUILDS_DIR/hub.json" 2>/dev/null)" "$(jq -r .start_time "$BUILDS_DIR/hub.json" 2>/dev/null)" && hub=1; fi
  for d in "$BUILDS_DIR"/*/*.tmp-*-*; do [ -e "$d" ] || continue
    if [[ "$d" =~ \.tmp-([0-9]+)-([0-9]+)$ ]]; then pid=${BASH_REMATCH[1]}; st=${BASH_REMATCH[2]}
      if lo_alive "$pid" "$st"; then emit info build_tmp_live "$d" "owner pid $pid still running"; else emit drift stale_build_tmp "$d" "owner pid $pid start $st is gone (resolved from /proc, 11.4.180)" "rmdir:build_tmp:$d"; fi
    fi
  done
  for d in "$BUILDS_DIR"/*/; do d=${d%/}; id=${d##*/}; [ -d "$d" ] || continue; case "$id" in .*|groups|hub*) continue ;; esac
    [ -d "$d/terminal" ] && continue
    [ "$hub" = 1 ] || emit drift open_builds_without_hub "$id" "no terminal/ and no live hub record ($BUILDS_DIR/hub.json); the sweep starts no process (T005b)"
    grep -lq "\"op_id\":\"$id\"\|\"build_id\":\"$id\"" "$LD"/ops/*.json 2>/dev/null || emit drift build_without_registry_row "$id" "no registry row names this build (UNCONFIRMED mapping: op_id or build_id equals the build id)"
  done
}
needle_AM_P3() {
  local n="$W/nd-p3"; mkdir -p "$n/b/x1" "$n/b/x2/terminal" "$n/b/x3"; mkdir "$n/b/x3/work.tmp-999999-1" "$n/ld"; mkdir "$n/b/x3/work2.tmp-$$-$(lo_pstart $$)"
  local o; o=$( BUILDS_DIR=$n/b; LD=$n/ld; det_AM_P3 )
  grep -q "stale_build_tmp${F}$n/b/x3/work.tmp-999999-1" <<<"$o" && grep -q "open_builds_without_hub${F}x1" <<<"$o" || { echo "seeded leftover tmp dir or open build without hub not reported: [$o]"; return 1; }
  grep -q "work2.tmp" <<<"$(grep '^drift' <<<"$o")" && { echo "a tmp dir of a live process reported: [$o]"; return 1; }
  grep -q "open_builds_without_hub${F}x2" <<<"$o" && { echo "a terminal build reported open: [$o]"; return 1; }
  return 0
}

# ---------- INV-9 commit-push run directories ----------
CP_HOLDER=""   # JSON of the commit_push holder or "unread:<reason>"
read_cp_holder() {
  if [ -n "${CPA_APPROVED_DIR:-}" ]; then CP_HOLDER=$(CPA_APPROVED_DIR=$CPA_APPROVED_DIR bash "$TOOLS/scripts/longops/holder.sh" commit_push 2>/dev/null) || CP_HOLDER="unread:holder.sh refused (exit $?)"
  elif command -v cpa-host >/dev/null 2>&1; then CP_HOLDER=$(cpa-host --exec-approved scripts/longops/holder.sh commit_push 2>/dev/null) || CP_HOLDER="unread:project_not_trusted (cpa-host --exec-approved refused; before T046a this is expected)"
  else CP_HOLDER="unread:helper_not_approved (CPA_APPROVED_DIR unset and no cpa-host on PATH; the commit_push holder is not read, CENTRAL C2)"; fi
  [ -n "$CP_HOLDER" ] || CP_HOLDER="unread:holder.sh printed nothing"
}
_conf_val() {  # _conf_val <key>: the larger of the HEAD value and the approved value; empty when neither file gives one
  local k=$1 a="" b="" f
  for f in "$AM_ROOT/scripts/repo/commit_push.conf" "${CPA_APPROVED_DIR:+$CPA_APPROVED_DIR/scripts/repo/commit_push.conf}"; do
    [ -n "$f" ] && [ -r "$f" ] || continue; b=$(sed -n "s/^[[:space:]]*$k[[:space:]]*[=:][[:space:]]*\\([0-9][0-9]*\\).*/\\1/p" "$f" | head -1)
    [ -n "$b" ] && { [ -z "$a" ] || [ "$b" -gt "$a" ] && a=$b; }
  done; echo "${a:-}"
}
_held_ok() {  # _held_ok <commits.tsv>: 0 when every listed commit is on every reachable remote tip of main/master; 1 otherwise or when unproven
  local f=$1 repo sha rem tip br any ok
  while IFS=$'\t' read -r repo sha _; do
    [ -n "$sha" ] || continue; case "$repo" in \#*) continue ;; esac
    [ "$repo" = . ] && rp=$AM_ROOT || rp=$AM_ROOT/$repo
    any=0
    for rem in $(git -C "$rp" remote 2>/dev/null); do
      tip=""; for br in main master; do tip=$(git -C "$rp" ls-remote "$rem" "refs/heads/$br" 2>/dev/null | awk -v r="refs/heads/$br" '$2==r{print $1;exit}'); [ -n "$tip" ] && break; done
      [ -n "$tip" ] || continue; any=1
      git -C "$rp" cat-file -e "$tip^{commit}" 2>/dev/null && git -C "$rp" merge-base --is-ancestor "$sha" "$tip" 2>/dev/null || return 1
    done
    [ "$any" = 1 ] || return 1
  done <"$f"
  return 0
}
det_INV_9() {
  local base="$AUDIT_DIR/commit-push"; [ -d "$base" ] || return 0
  read_cp_holder
  local hstat="" hrun="" hst="" hunread=""
  case "$CP_HOLDER" in unread:*) hunread=1; emit info commit_push_holder_unread commit_push "${CP_HOLDER#unread:}" ;; none) ;; *) hrun=$(jq -r .run_id <<<"$CP_HOLDER" 2>/dev/null); hst=$(jq -r .state <<<"$CP_HOLDER" 2>/dev/null); hstat=$(jq -r .status <<<"$CP_HOLDER" 2>/dev/null) ;; esac
  local d id rep st rr rd mrepo mpid mst gitp ts pid now_=$(now) n=0 finished=()
  local retain_runs retain_days; retain_runs=$(_conf_val retain_runs); retain_days=$(_conf_val retain_days)
  for d in "$base"/*/; do d=${d%/}; [ -d "$d" ] || continue; id=${d##*/}; rep=$d/report.json
    if [ ! -e "$rep" ]; then
      if [ -s "$d/merge.json" ]; then
        mrepo=$(jq -r '.repo // "."' "$d/merge.json"); mpid=$(jq -r '.pid // 0' "$d/merge.json"); mst=$(jq -r '.start_time // ""' "$d/merge.json")
        [ "$mrepo" = . ] && rr=$AM_ROOT || rr=$AM_ROOT/$mrepo
        gitp=$(git -C "$rr" rev-parse --git-path MERGE_HEAD 2>/dev/null); case "$gitp" in /*) ;; *) gitp=$rr/$gitp ;; esac
        if [ -n "$gitp" ] && [ -e "$gitp" ]; then
          if lo_alive "$mpid" "$mst"; then emit info live_merge_holder "$id" "pid $mpid cmdline '$(lo_cmdline "$mpid")' is the live lock holder paused inside its merge; no remediation, never resolved by the sweep"
          else emit drift interrupted_merge "$id" "repository $mrepo has MERGE_HEAD and the merge process (pid $mpid start $mst) is gone; remediation (T042 S0): git merge --abort in that repository, then compare it with its backup/worktree/ list"; fi
          continue
        fi
      fi
      if [ -n "$hrun" ] && [ "$hrun" = "$id" ]; then
        case "$hst" in ready_to_resume) emit info ready_to_resume "$id" "holder state ready_to_resume (status $hstat); not interrupted, suspended or stale"; [ "$hstat" = expired ] && emit drift ready_to_resume_expired "$id" "ready_to_resume window ran out; reported only: release is acquire.sh --expire commit_push" ;;
          *) if [ "$hstat" = live ]; then emit info suspended_run "$id" "suspended-run holder is live"; else emit info holder_not_live "$id" "holder run state $hst status $hstat"; fi ;;
        esac; continue
      fi
      if [[ "$id" =~ ^[0-9]{8}T[0-9]{6}Z-([0-9]+)- ]]; then pid=${BASH_REMATCH[1]}
        if [ -r "/proc/$pid/cmdline" ] && lo_cmdline "$pid" | grep -q 'commit-push-all'; then emit info live_run "$id" "pid $pid cmdline '$(lo_cmdline "$pid")'"
        else emit drift interrupted_run "$id" "no report.json and process pid $pid (from the run id) is gone or not commit-push-all (resolved from /proc/$pid/cmdline); never removed by the sweep"; fi
      else emit drift owner_unresolved "$id" "no report.json and the owning process cannot be resolved (no merge.json, not the holder, no pid in the run id)"; fi
    else
      st=$(jq -r '.status // ""' "$rep" 2>/dev/null)
      case "$st" in
        awaiting_remote_checks) if [ "$hrun" = "$id" ] && [ "$hstat" = live ]; then emit info suspended_run "$id" "report.json awaiting_remote_checks and the suspended-run holder is live: reported suspended, never interrupted, never reaped"
          elif [ -n "$hunread" ]; then emit unread suspended_run_holder_unread "$id" "report.json says awaiting_remote_checks but the commit_push holder could not be read (${CP_HOLDER#unread:}): whether the run is suspended or abandoned is NOT known"
          else emit drift suspended_run_without_live_holder "$id" "report.json says awaiting_remote_checks but the suspended-run holder is not live (holder: ${hrun:-none}/${hstat:-none}); reported, never removed"; fi ;;
        ready_to_resume) emit info ready_to_resume "$id" "report.json ready_to_resume"; [ "$hrun" = "$id" ] && [ "$hstat" = expired ] && emit drift ready_to_resume_expired "$id" "ready_to_resume window ran out; reported only: release is acquire.sh --expire commit_push" ;;
        *) finished+=("$d") ;;
      esac
    fi
  done
  [ "${#finished[@]}" -gt 0 ] || return 0
  if [ -z "$retain_runs" ] || [ -z "$retain_days" ]; then emit info retention_unread commit_push "scripts/repo/commit_push.conf (T042) gives no retain_runs/retain_days: no finished run directory is removed"; return 0; fi
  local sorted i=0 age_d
  sorted=$(for d in "${finished[@]}"; do printf '%s\t%s\n' "$(stat -c %Y -- "$d/report.json")" "$d"; done | sort -rn)
  while IFS=$'\t' read -r ts d; do i=$((i+1)); age_d=$(( (now_ - ts) / 86400 ))
    [ "$i" -gt "$retain_runs" ] && [ "$age_d" -gt "$retain_days" ] || continue
    if [ ! -s "$d/commits.tsv" ] || _held_ok "$d/commits.tsv"; then emit drift removable_finished_run "${d##*/}" "beyond the newest $retain_runs runs and older than $retain_days days (age ${age_d}d); every commit is on every reachable remote tip of main" "rmdir:finished_run:$d"
    else emit info kept_held_commit "${d##*/}" "a commit of commits.tsv is missing from a reachable remote tip of main: its record stays while it awaits its GO"; fi
  done <<<"$sorted"
}
needle_INV_9() {
  local n="$W/nd-i9" o; mkdir -p "$n"; mkrepo "$n/r"; echo 1 >"$n/r/f"; commit_all "$n/r"; mkdir -p "$n/r/.audit/commit-push"
  local b=$n/r/.audit/commit-push; mkdir -p "$b/20260101T000000Z-999999-aaaa" "$b/20260101T000001Z-1-bbbb"
  # interrupted merge: MERGE_HEAD present + merge.json naming a dead process
  mkdir -p "$b/mrg"; echo '{"repo":".","pid":999998,"start_time":"1"}' >"$b/mrg/merge.json"; git -C "$n/r" rev-parse HEAD >"$n/r/.git/MERGE_HEAD"
  o=$( AM_ROOT=$n/r; AUDIT_DIR=$n/r/.audit; CPA_APPROVED_DIR=""; read_cp_holder() { CP_HOLDER=none; }; det_INV_9 )
  grep -q "interrupted_run${F}20260101T000000Z-999999-aaaa" <<<"$o" && grep -q "interrupted_merge${F}mrg" <<<"$o" || { echo "seeded interrupted run / interrupted merge not reported: [$o]"; return 1; }
  # carriers: a live merge holder (this shell's pid and start time), a finished dir inside retention
  sleep 300 >/dev/null 2>&1 & local P=$!
  echo "{\"repo\":\".\",\"pid\":$P,\"start_time\":\"$(lo_pstart "$P")\"}" >"$b/mrg/merge.json"; rm -rf "$b/20260101T000000Z-999999-aaaa" "$b/20260101T000001Z-1-bbbb"
  mkdir -p "$b/fin"; echo '{"status":"complete"}' >"$b/fin/report.json"
  o=$( AM_ROOT=$n/r; AUDIT_DIR=$n/r/.audit; CPA_APPROVED_DIR=""; read_cp_holder() { CP_HOLDER=none; }; det_INV_9 ); lo_kill_child "$P"
  [ "$(grep -c '^drift' <<<"$o")" -eq 0 ] && grep -q "live_merge_holder${F}mrg" <<<"$o" || { echo "a live merge holder or a finished dir inside retention reported as drift: [$o]"; return 1; }
}

# ---------- driver ----------
IDS=(); declare -A DET STG REC NDL RSN ALI PLN
while IFS=$'\037' read -r id det stg rec ndl pln ali rsn; do
  IDS+=("$id"); DET[$id]=$det; STG[$id]=$stg; REC[$id]=$rec; NDL[$id]=$ndl; PLN[$id]=$pln; ALI[$id]=$ali; RSN[$id]=$rsn
done < <(python3 -I - "$CATALOGUE" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
assert d.get("schema") == "anti-mess-catalogue/1", "catalogue schema"
for i in d["invariants"]:
    print("\x1f".join([i["id"], i["detector"], ",".join(i["stages"]), i["reconcile"], ("yes" if i["needle"] in (True, "yes") else "no"), i["plane"], ",".join(i.get("aliases", [])), i.get("not_evaluated_reason", "")]))
PY
) || die catalogue "cannot read $CATALOGUE"
[ "${#IDS[@]}" -gt 0 ] || die catalogue "no invariants"
if [ -n "$ONLY" ]; then   # an unknown id (a typo, a lower-case id) is a usage refusal, never an empty successful sweep (WF11 F12)
  _o=${ONLY#,}; _o=${_o%,}; IFS=, read -r -a _ol <<<"$_o"
  [ "${#_ol[@]}" -gt 0 ] || die usage "--only names no invariant id"
  for _i in "${_ol[@]}"; do _k=0; for id in "${IDS[@]}"; do [ "$id" = "$_i" ] && _k=1; done; [ "$_k" = 1 ] || die usage "--only names an unknown invariant id '$_i' (catalogue ids: ${IDS[*]})"; done
fi
RESULTS="$W/results"; : >"$RESULTS"; REFUSE=0; DRIFT=0
reconcile_action() {  # <action> -> prints a result word; every action re-verifies its own precondition AT ACTION TIME (WF11 F2/F10)
  local a=${1%%:*} arg=${1#*:}
  [ -z "${ANTIMESS_TEST_BEFORE_ACTION:-}" ] || bash "$ANTIMESS_TEST_BEFORE_ACTION" "$1" >/dev/null 2>&1
  case "$a" in
    rmlock) local age; age=$(( $(now) - $(stat -c %Y -- "$arg" 2>/dev/null || echo 0) ))
      if [ -e "$arg" ] && ! _open_by_any "$arg" >/dev/null && [ "$(_git_active)" -eq 0 ] && [ "$age" -ge "${ANTIMESS_LOCK_MIN_AGE:-60}" ]; then rm -f -- "$arg" && echo removed || echo failed; else echo skipped_not_provably_stale; fi ;;
    reapop) bash "$TOOLS/scripts/longops/reap.sh" --op-id "$arg" >/dev/null 2>&1 && echo reaped || echo refused ;;
    stopcontainer) local cid=${arg%%:*} want=${arg#*:} lst c3 op3 cr3 now_class=gone
      lst=$(_p1_list) || { echo skipped_containers_unreadable; return; }
      while IFS=$'\t' read -r c3 op3 cr3; do [ "$c3" = "$cid" ] || continue; now_class=$(_p1_class "$op3" "$cr3"); done <<<"$lst"
      if [ "$now_class" != "$want" ]; then echo "skipped_precondition_changed_${now_class}"; else
        timeout 60 "$PODMAN" stop -t 5 "$cid" >/dev/null 2>&1 && echo stopped || echo failed; fi ;;
    rmdir) local kind=${arg%%:*} path=${arg#*:}
      case "$kind" in
        build_tmp)
          if [[ "$path" =~ \.tmp-([0-9]+)-([0-9]+)$ ]] && case "$path" in "$BUILDS_DIR"/*) true ;; *) false ;; esac && [ -d "$path" ] && ! lo_alive "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"; then rm -rf -- "$path" && echo removed || echo failed
          else echo skipped_precondition_changed; fi ;;
        finished_run)
          local rep="$path/report.json" st4
          case "$path" in "$AUDIT_DIR"/commit-push/*) ;; *) echo refused_outside_commit_push; return ;; esac
          [ -s "$rep" ] || { echo skipped_precondition_changed; return; }
          st4=$(jq -r '.status // ""' "$rep" 2>/dev/null); case "$st4" in awaiting_remote_checks|ready_to_resume) echo skipped_precondition_changed; return ;; esac
          if [ -s "$path/commits.tsv" ] && ! _held_ok "$path/commits.tsv"; then echo skipped_held_commit_not_on_remote; return; fi
          rm -rf -- "$path" && echo removed || echo failed ;;
        *) echo unknown_rmdir_kind ;;
      esac ;;
    initsub) if git -C "$AM_ROOT" submodule status --recursive -- "$arg" 2>/dev/null | grep -q '^-'; then git -C "$AM_ROOT" submodule update --init -- "$arg" >/dev/null 2>&1 && echo initialised || echo failed; else echo skipped_already_initialised; fi ;;
    *) echo unknown_action ;;
  esac
}
for id in "${IDS[@]}"; do
  [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) continue ;; esac
  status=""; needle=not_applicable; reason=""; fl="$W/f.$id"; : >"$fl"
  case ",${STG[$id]}," in *",$STAGE,"*) ;; *) status=skipped_stage; reason="not evaluated at stage $STAGE (catalogue stages: ${STG[$id]})" ;; esac
  if [ -z "$status" ] && [ "${DET[$id]}" = none ]; then status=not_evaluated; reason=${RSN[$id]}; fi
  if [ -z "$status" ]; then
    fn=${DET[$id]}; declare -F "$fn" >/dev/null || { status=blind; reason="detector function $fn is missing"; }
    if [ -z "$status" ] && [ "${NDL[$id]}" = yes ]; then
      nfn=needle_${id//-/_}; if out=$($nfn 2>&1); then needle=seen; else needle=blind; status=blind; reason="control needle failed: $out"; fi
    fi
    if [ -z "$status" ]; then
      "$fn" >"$fl" 2>"$W/err.$id"
      if grep -q '^drift' "$fl"; then status=drift; elif grep -q '^unread' "$fl"; then status=unread; else status=clean; fi
      grep -q '^unread' "$fl" && reason=$(grep '^unread' "$fl" | cut -f4 | head -1)
    fi
  fi
  rec_json='[]'
  if [ "$RECON" = 1 ] && [ "${REC[$id]}" = auto-safe ] && [ "$status" = drift ]; then
    while IFS=$'\t' read -r sev cls sub ev act; do [ -n "${act:-}" ] || continue; res=$(reconcile_action "$act"); rec_json=$(jq -c --arg a "$act" --arg r "$res" '. + [{action:$a,result:$r}]' <<<"$rec_json"); done <"$fl"
  fi
  [ "$status" = drift ] && DRIFT=$((DRIFT+1)); [ "$status" = blind ] && REFUSE=1
  { [ "$status" = unread ] || grep -q '^unread' "$fl"; } && UNREAD=$((UNREAD+1))
  grep -q "^drift${F}blocking_hooks_path" "$fl" && [ "$STAGE" = S0 ] && REFUSE=1
  findings=$(jq -R -s -c 'split("\n")|map(select(length>0)|split("\t"))|map({severity:.[0],class:.[1],subject:.[2],evidence:.[3],action:(.[4]//"")})' <"$fl")
  jq -nc --arg id "$id" --arg al "${ALI[$id]}" --arg pl "${PLN[$id]}" --arg st "$status" --arg nd "$needle" --arg rs "$reason" --arg rc "${REC[$id]}" --argjson f "$findings" --argjson r "$rec_json" \
    '{id:$id,aliases:($al|split(",")|map(select(length>0))),plane:$pl,status:$st,control_needle:$nd,reason:$rs,reconcile_class:$rc,findings:$f,reconciled:$r}' >>"$RESULTS"
done
REPORT=$(jq -s -c --arg stage "$STAGE" --arg root "$AM_ROOT" --arg u "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --argjson rec "$RECON" --arg pf "$PATHS_FROM" --arg repo "$ARG_REPO" --arg cat "$CATALOGUE" '
  {schema:"anti-mess-sweep/1",generated_utc:$u,stage:$stage,root:$root,catalogue:$cat,reconcile:($rec==1),paths_from:$pf,repo:$repo,invariants:.,
   summary:{drift:(map(select(.status=="drift"))|length),clean:(map(select(.status=="clean"))|length),not_evaluated:(map(select(.status=="not_evaluated"))|length),
            unread:(map(select(.status=="unread"))|length),blind:(map(select(.status=="blind"))|length),skipped_stage:(map(select(.status=="skipped_stage"))|length)}}' <"$RESULTS")
[ -z "$JSON_OUT" ] || { printf '%s\n' "$REPORT" | jq . >"$JSON_OUT.tmp" && mv -f "$JSON_OUT.tmp" "$JSON_OUT"; }
printf '%-7s %-14s %-9s %s\n' ID STATUS NEEDLE DETAIL
jq -r '.invariants[]|[.id,.status,.control_needle,(if .status=="drift" then ((.findings|map(select(.severity=="drift"))|length|tostring)+" drift finding(s): "+(.findings|map(select(.severity=="drift")|.class+" "+.subject)|.[0:3]|join("; ")))
  elif .status=="clean" then "no drift ("+((.findings|length)|tostring)+" informational)" else .reason[0:150] end)]|@tsv' <<<"$REPORT" | awk -F'\t' '{printf "%-7s %-14s %-9s %s\n",$1,$2,$3,$4}'
jq -r '"SUMMARY drift=\(.summary.drift) clean=\(.summary.clean) not_evaluated=\(.summary.not_evaluated) unread=\(.summary.unread) blind=\(.summary.blind) skipped_stage=\(.summary.skipped_stage)"' <<<"$REPORT"
[ "$REFUSE" = 1 ] && exit 20
[ "$DRIFT" -gt 0 ] && exit 10
[ "$UNREAD" -gt 0 ] && exit 11
exit 0
