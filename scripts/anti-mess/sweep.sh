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
# Completeness (11.4.276 round 5, class B) Every detector runs in a SUBSHELL and must reach its end (a sentinel proves it): a fatal expansion error (an arithmetic operand that vanished, an unset variable) used to abandon the
#   whole driver loop and print a short report with exit 0. Now the detector is `blind` (exit 20) with its stderr, the report must hold exactly ONE row per selected invariant or the sweep exits 20 `driver_incomplete`, and
#   non-empty detector stderr is recorded in the row (`stderr`) as information.
# Inputs  (class A) every op record and holder record is read through the registry's ONE shape (scripts/longops/lib.sh LO_OP_SHAPE / LO_HOLDER_SHAPE): a record that fails it is `unread`, never skipped, never clean;
#   a mtime or a podman `Created` that cannot be read as a canonical integer is a skipped entry (the entry moved meanwhile) or `unread`, never "age = now".
# Findings stream (class C) `emit` escapes backslash, TAB, LF and CR in all five fields, so a row can neither be split nor forged by a name; an action is attached only to a subject that passes lo_safe_name or whose path
#   is free of control characters.
# Env     ANTIMESS_ROOT (repository to sweep, default this one), LONGOPS_* (registry, see scripts/longops/lib.sh), ANTIMESS_OWNED_ORGS (fixtures),
#         ANTIMESS_LOCK_MIN_AGE (seconds a git lock must exist before it can be stale, default 60), ANTIMESS_ORPHAN_AGE_S (default 300),
#         ANTIMESS_ACQUIRE_PURPOSES (comma list, default commit_push: the purposes whose claim is made by acquire.sh and has NO op record, consumer DATA),
#         CPA_APPROVED_DIR (the approved copy of the CPA run being served: S0, S7), PODMAN via LONGOPS_PODMAN.
#         ANTIMESS_TEST_BEFORE_ACTION (a script run before every reconcile action, with ANTIMESS_TEST_MODE=1 only) lets a test move the state between detection and action.
#         ANTIMESS_CATALOGUE (the catalogue; a path other than the shipped one needs ANTIMESS_TEST_MODE=1) lets a test point an invariant at a test-only detector.
# Exits   0 no drift; 10 drift reported; 11 a source could not be read and no drift was found (never read as clean: a corrupt op record, podman failing, the verifier
#         giving no report); 20 refusal (usage, an unknown --only id, a blind detector, driver_incomplete, a blocking core.hooksPath at S0). Reading only unless --reconcile.
#         The exit status is derived from the FINAL state: after --reconcile the detectors run again (at most 3 passes, an action is never repeated) and the report carries `detected` (pass 1) and the final status.
# Reconcile  a container is stopped ONLY when its op is in THIS registry and terminal (WF14 R2-11): the registry is per checkout and the podman namespace is per user, so a container whose op is
#         absent here may belong to another checkout, track or scratch copy; it is reported (orphan_container) and never stopped. Absence from one registry is not proof of staleness (11.4.232 E).
#         every action is BOUND to its invariant (catalogue `actions:`) and RE-VERIFIES its FULL predicate at action time on CANONICAL paths (WF11 F10, 11.4.276 class C): rmlock (a regular non-symlink *.lock under the
#         git dir, no holder, no git activity, old enough), stopcontainer (the container's lineage class is re-derived from the current registry), rmdir (build_tmp: exactly <builds>/<dir>/<name>.tmp-<pid>-<start>,
#         owner still gone; finished_run: the canonical parent is <audit>/commit-push, report.json still finished, every held commit still on a remote tip), initsub (still uninitialised), reapop (reap.sh --expect dead_owner:
#         the class under the purpose lock must still be dead_owner). A `handoff` op is re-adoptable (11.4.232 D): its container is never stopped.
set -u
export LC_ALL=C
SD=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TOOLS=$(cd "$SD/../.." && pwd)
UNREAD=0
die() { echo "sweep: $1: $2" >&2; exit 20; }
AM_ROOT=$(realpath -e -- "${ANTIMESS_ROOT:-$TOOLS}" 2>/dev/null) || die not_a_repository "${ANTIMESS_ROOT:-$TOOLS}"
export LONGOPS_REPO=${LONGOPS_REPO:-$AM_ROOT}
. "$TOOLS/scripts/longops/lib.sh"
CATALOGUE=${ANTIMESS_CATALOGUE:-$SD/catalogue.yaml}
STAGE=cadence; PATHS_FROM=""; ARG_REPO=""; RECON=0; JSON_OUT=""; ONLY=""
while [ $# -gt 0 ]; do
  case "$1" in
    --stage) [ $# -ge 2 ] || die usage "$1 requires a value"; STAGE=$2; shift 2 ;; --paths-from) [ $# -ge 2 ] || die usage "$1 requires a value"; PATHS_FROM=$2; shift 2 ;;
    --repo) [ $# -ge 2 ] || die usage "$1 requires a value"; ARG_REPO=$2; shift 2 ;; --reconcile) RECON=1; shift ;; --json) [ $# -ge 2 ] || die usage "$1 requires a value"; JSON_OUT=$2; shift 2 ;;
    --only) [ $# -ge 2 ] || die usage "$1 requires a value"; ONLY=,$2, ; shift 2 ;;
    *) die usage "unknown argument $(printf '%q' "$1")" ;;
  esac
done
case "$STAGE" in S0|S7|cadence) ;; *) die usage "--stage must be S0, S7 or cadence" ;; esac
[ -z "${ANTIMESS_TEST_BEFORE_ACTION:-}" ] || [ "${ANTIMESS_TEST_MODE:-}" = 1 ] || die test_hook_outside_test_mode "ANTIMESS_TEST_BEFORE_ACTION is a test hook and needs ANTIMESS_TEST_MODE=1"
[ "$CATALOGUE" = "$SD/catalogue.yaml" ] || [ "${ANTIMESS_TEST_MODE:-}" = 1 ] || die test_hook_outside_test_mode "ANTIMESS_CATALOGUE other than the shipped catalogue needs ANTIMESS_TEST_MODE=1"
[ -z "$PATHS_FROM" ] || [ "$STAGE" = S0 ] || die usage "--paths-from is for --stage S0 only"
[ -z "$PATHS_FROM" ] || [ -r "$PATHS_FROM" ] || die usage "--paths-from file unreadable"
[ -d "$AM_ROOT/.git" ] || [ -f "$AM_ROOT/.git" ] || die not_a_repository "$AM_ROOT"
W=$(mktemp -d "${TMPDIR:-/tmp}/antimess.XXXXXX") || die internal mktemp
trap 'rm -rf "$W"' EXIT
F=$'\t'
# _esc <var> <value>: backslash, TAB, LF and CR escaped, so that a field can neither split a row nor start a forged one (class C)
_esc() { local -n _o=$1; local s=$2; s=${s//\\/\\\\}; s=${s//$'\t'/\\t}; s=${s//$'\n'/\\n}; s=${s//$'\r'/\\r}; _o=$s; }
# _unesc <var> <value>: the inverse of _esc (an action is unescaped before it is run)
_unesc() { local -n _u=$1; local s=$2 out="" c; while [[ "$s" == *\\* ]]; do out+=${s%%\\*}; s=${s#*\\}; c=${s:0:1}; s=${s:1}; case "$c" in t) out+=$'\t' ;; n) out+=$'\n' ;; r) out+=$'\r' ;; \\) out+='\' ;; *) out+="\\$c" ;; esac; done; _u=$out$s; }
emit() { local a b c d e; _esc a "$1"; _esc b "$2"; _esc c "$3"; _esc d "$4"; _esc e "${5:-}"; printf '%s\t%s\t%s\t%s\t%s\n' "$a" "$b" "$c" "$d" "$e"; }   # severity class subject evidence [action]
# _plain <s>: 0 when s holds no control character (a path that may carry an action)
_plain() { [[ "$1" != *[[:cntrl:]]* ]]; }
now() { lo_now; }
mkrepo() { git init -q "$1" 2>/dev/null && git -C "$1" config user.email a@a && git -C "$1" config user.name a && git -C "$1" config commit.gpgsign false; }
commit_all() { git -C "$1" add -A && git -C "$1" -c core.hooksPath=/dev/null commit -qm "${2:-fx}" 2>/dev/null; }
# _mkop <ops-dir> <op_id> <purpose> <state> <pid> <last_progress_mono> [<no_progress_s>]: a COMPLETE, shape-valid op record for a control needle (the ONE shape of scripts/longops/lib.sh)
_mkop() {
  local st; st=$(lo_pstart "$5" 2>/dev/null); [ -n "$st" ] || st=1
  jq -nc --arg id "$2" --arg p "$3" --arg s "$4" --argjson pid "$5" --arg st "$st" --arg bid "$(lo_boot_id)" --argjson lp "$6" --argjson np "${7:-30}" \
    '{op_id:$id,purpose_key:$p,owner:"needle",run_id:$id,pid:$pid,start_time:$st,boot_id:$bid,cmdline:"needle",container_id:"",container_label:$id,state:$s,started_utc:"2026-01-01T00:00:00Z",
      last_heartbeat_utc:"2026-01-01T00:00:00Z",heartbeat_seq:0,progress_offset:0,last_progress_epoch:$lp,last_progress_mono:$lp,log_path:"",budget:{memory_bytes:0,cpus:0,wall_clock_s:0,no_progress_s:$np},write_paths:[],verdict:"",evidence_path:""}' >"$1/$2.json"
}

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
  local path tr un decl="" f rel rest n=0 src p
  [ -z "$PATHS_FROM" ] || decl=$(declared_paths)
  while IFS=$'\t' read -r path tr un; do
    if [ -n "$ARG_REPO" ]; then case "$path/" in "$ARG_REPO"/|"$ARG_REPO"/*) ;; *) continue ;; esac; fi
    if [ "$STAGE" = S0 ] && [ -n "$PATHS_FROM" ]; then
      rest=""; n=0
      while IFS= read -r -d '' ent; do
        f=${ent:3}; src=""
        # a rename (or copy) entry of `status -z` is `XY new NUL old NUL`: the SOURCE of a rename is DELETED, so it must be declared too (LO-I1)
        case "${ent:0:1}${ent:1:1}" in R*|C*|*R|*C) IFS= read -r -d '' src ;; esac
        for p in "$f" ${src:+"$src"}; do
          [ "$path" = . ] && rel=$p || rel=$path/$p
          grep -qxF -- "$rel" <<<"$decl" || { n=$((n+1)); [ "$n" -le 5 ] && rest="$rest $rel"; }
        done
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
  # a staged rename whose NEW path is declared but whose SOURCE is not: the source is deleted, so it must be reported (LO-I1)
  mkrepo "$n/c"; echo 1 >"$n/c/f.txt"; commit_all "$n/c"; git -C "$n/c" mv f.txt g.txt; printf 'g.txt\n' >"$n/cs3"
  o=$( AM_ROOT=$n/c; ANTIMESS_OWNED_ORGS=needleorg; STAGE=S0; PATHS_FROM=$n/cs3; ARG_REPO=""; det_AM_R1 ); [ "$(grep -c '^drift' <<<"$o")" -eq 1 ] || { echo "S0 rename with an undeclared source not reported: [$o]"; return 1; }
  printf 'g.txt\nf.txt\n' >"$n/cs4"
  o=$( AM_ROOT=$n/c; ANTIMESS_OWNED_ORGS=needleorg; STAGE=S0; PATHS_FROM=$n/cs4; ARG_REPO=""; det_AM_R1 ); [ "$(grep -c '^drift' <<<"$o")" -eq 0 ] || { echo "S0 rename with both paths declared reported: [$o]"; return 1; }
}

# ---------- AM-R2 stale git locks ----------
_open_by_any() {  # _open_by_any <file>: prints `pid cmdline` of a process holding it open (own uid; /proc fd links), nothing otherwise
  local l p; l=$(find /proc/[0-9]*/fd -maxdepth 1 -lname "$1" -print -quit 2>/dev/null); [ -n "$l" ] || return 1
  p=${l#/proc/}; p=${p%%/*}; echo "$p $(lo_cmdline "$p")"
}
_git_active() {  # count of git processes whose CANONICAL working directory is inside the swept repository (/proc/<pid>/cwd is a physical path: it is compared with the canonical AM_ROOT, LO-H2)
  local f p c n=0; for f in $(grep -l -E '^git($|-)' /proc/[0-9]*/comm 2>/dev/null); do p=${f#/proc/}; p=${p%%/*}; c=$(readlink -f "/proc/$p/cwd" 2>/dev/null) || continue
    case "$c/" in "$AM_ROOT"/*) n=$((n+1)) ;; esac; done; echo "$n"
}
det_AM_R2() {
  local gd; gd=$(git -C "$AM_ROOT" rev-parse --absolute-git-dir 2>/dev/null) && gd=$(realpath -e -- "$gd" 2>/dev/null) || { emit unread git_dir "$AM_ROOT" "git rev-parse --absolute-git-dir failed"; return; }
  local minage=${ANTIMESS_LOCK_MIN_AGE:-60} lk age mt h act now_; now_=$(now); act=$(_git_active)
  while IFS= read -r -d '' lk; do
    # a lock that vanished between `find` and `stat` (the ordinary `git status` index.lock race) was released meanwhile: skip it, never "age = now" (class A, LO-B1)
    mt=$(stat -c %Y -- "$lk" 2>/dev/null) || continue
    lo_uint "$mt" || continue
    age=$(( now_ - mt ))
    if h=$(_open_by_any "$lk"); then emit info lock_held_open "$lk" "held open by pid ${h%% *} cmdline '${h#* }' (a live holder)"
    elif [ "$act" -gt 0 ]; then emit info lock_with_git_activity "$lk" "$act git process(es) running; holder not provable dead, left alone (conservative-safe)"
    elif [ "$age" -lt "$minage" ]; then emit info lock_young "$lk" "age ${age}s < ${minage}s"
    elif _plain "$lk"; then emit drift stale_git_lock "$lk" "age ${age}s, no process has it open, no git process is running (resolved from /proc)" "rmlock:$lk"
    else emit unread git_lock_name "$lk" "a lock file whose name holds a control character: reported, no action attached"; fi
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
  while IFS= read -r line; do [ "${line:0:1}" = - ] || continue; sp=$(awk '{print $2}' <<<"$line")
    if _plain "$sp"; then emit drift uninitialised_submodule "$sp" "declared in .gitmodules, not checked out (git submodule status leading '-')" "initsub:$sp"
    else emit unread submodule_path "$sp" "a submodule path that holds a control character: reported, no action attached"; fi
  done <<<"$l"
}
needle_AM_R5() {
  local n="$W/nd-r5"; mkdir -p "$n"; mkrepo "$n/sub"; echo 1 >"$n/sub/f"; commit_all "$n/sub"; mkrepo "$n/par"
  git -C "$n/par" -c protocol.file.allow=always submodule add -q "$n/sub" sm 2>/dev/null; commit_all "$n/par" par
  git clone -q "$n/par" "$n/clone" 2>/dev/null
  local o; o=$( AM_ROOT=$n/clone; det_AM_R5 ); [ "$(grep -c '^drift' <<<"$o")" -eq 1 ] || { echo "seeded uninitialised submodule not reported: [$o]"; return 1; }
  o=$( AM_ROOT=$n/par; det_AM_R5 ); [ "$(grep -c '^drift' <<<"$o")" -eq 0 ] || { echo "an initialised submodule reported: [$o]"; return 1; }
}

# ---------- AM-G2 no CI pipeline + non-blocking core.hooksPath ----------
# githooks(5): the client-side hooks whose NON-ZERO exit aborts the operation they guard (post-* hooks never abort). reference-transaction aborts every ref update in its `prepared` phase (a real commit exits 128).
_blocking_hooks_dir() {  # _blocking_hooks_dir <dir>: prints the first executable client-side hook found
  local h; for h in pre-commit prepare-commit-msg commit-msg pre-merge-commit pre-push pre-rebase pre-applypatch applypatch-msg reference-transaction pre-auto-gc; do [ -f "$1/$h" ] && [ -x "$1/$h" ] && { echo "$1/$h"; return 0; }; done; return 1
}
_g2_roots() { local j; echo "$AM_ROOT"; j=$(vr_json) && jq -r --arg r "$AM_ROOT" '.repos[]|select(.owned==true and .path!=".")|$r+"/"+.path' "$j"; }
det_AM_G2() {
  local r hp d b
  while IFS= read -r r; do
    [ -e "$r/.git" ] || continue
    # `--type=path`: git itself expands ~ and ~user, exactly as it does when it runs the hook (class I: ask git, do not re-implement it)
    hp=$(git -C "$r" config --type=path --get core.hooksPath 2>/dev/null) || hp=""
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
  # a hooksPath written with a tilde (git expands it) holding a blocking hook (LO-I2), and a reference-transaction hook (aborts every ref update)
  mkrepo "$n/c"; echo 1 >"$n/c/f"; commit_all "$n/c"; mkdir -p "$n/home/hk"; printf '#!/bin/sh\nexit 1\n' >"$n/home/hk/pre-commit"; chmod +x "$n/home/hk/pre-commit"; git -C "$n/c" config core.hooksPath '~/hk'
  o=$( HOME=$n/home; AM_ROOT=$n/c; ANTIMESS_OWNED_ORGS=needleorg; STAGE=S0; det_AM_G2 ); grep -q blocking_hooks_path <<<"$o" || { echo "a tilde core.hooksPath holding a blocking hook not reported: [$o]"; return 1; }
  mkrepo "$n/d"; echo 1 >"$n/d/f"; commit_all "$n/d"; mkdir -p "$n/d/rt"; printf '#!/bin/sh\nexit 1\n' >"$n/d/rt/reference-transaction"; chmod +x "$n/d/rt/reference-transaction"; git -C "$n/d" config core.hooksPath rt
  o=$( AM_ROOT=$n/d; ANTIMESS_OWNED_ORGS=needleorg; STAGE=S0; det_AM_G2 ); grep -q blocking_hooks_path <<<"$o" || { echo "a blocking reference-transaction hook not reported: [$o]"; return 1; }
}

# ---------- registry: AM-P1, AM-P2 ----------
# Every record is read through the registry snapshot (scripts/longops/lib.sh lo_ops_snapshot: ONE python3 and ONE jq for the whole registry, the ONE shape LO_OP_SHAPE): a record that fails the shape is reported `unread`,
# never skipped silently and never clean (WF11 F4, WF14 class D, 11.4.276 class A).
_unread_op_record() { emit unread corrupt_op_record "$1" "the op record fails the record shape (docs/scripts/longops.md FIELDS): empty, unparsable, mistyped or an unknown state; the invariant is evaluated on the readable records only"; }
# (round 5: the former _emit_unread_ops helper had three callers at 350372a8 (lines 226, 277, 297); the snapshot loops now report each bad row inline through _unread_op_record, so the helper was unreferenced and is removed; git history: 350372a8)
# the label the launcher really sets is catalogizer.op_id (scripts/containers/run_pinned.sh); op_id is accepted too (the real client container carries BOTH, with different values: both are resolved, LO-E3)
_P1_JQ='def lab(k): ((.Labels // {})[k] // "-") | if type=="string" and . != "" then . else "-" end;
  .[]?|[(.Id|.[0:12]),lab("catalogizer.op_id"),lab("op_id"),((.Created // 0)|tostring)]|@tsv'
_p1_list() { local ps; ps=$(timeout "$LO_PODMAN_TIMEOUT" "$PODMAN" ps --filter label=project=catalogizer --format json 2>"$W/pod.err") || return 1; [ -n "$ps" ] || ps='[]'; jq -r "$_P1_JQ" <<<"$ps"; }
# _p1_class <catalogizer.op_id label> <op_id label> <created>: terminal | handoff | live | unreadable | orphan | young | nolabel, from the CURRENT registry (also the re-verification of a stopcontainer action).
# Each label is resolved through the lineage map (lo_lineage_build; a dispatched build is <id>, <id>-a2 ... all with one label, the NUMERICALLY latest attempt decides). `live` if ANY label resolves live; `unreadable`
# if any resolves unreadable; `terminal` only if EVERY resolving label is terminal; `orphan`/`young` only when no label resolves at all.
_p1_class() {
  local l c any_live=0 any_unread=0 any_hand=0 any_term=0 n=0 age
  for l in "$1" "$2"; do
    [ "$l" != - ] && [ -n "$l" ] || continue
    n=$((n+1)); c=$(lo_lineage_class "$l")
    case "$c" in live) any_live=1 ;; unreadable) any_unread=1 ;; handoff) any_hand=1 ;; terminal) any_term=1 ;; esac
  done
  [ "$n" -gt 0 ] || { echo nolabel; return; }
  if [ "$any_live" = 1 ]; then echo live; return; fi
  if [ "$any_unread" = 1 ]; then echo unreadable; return; fi
  if [ "$any_hand" = 1 ]; then echo handoff; return; fi
  if [ "$any_term" = 1 ]; then echo terminal; return; fi
  lo_uint "$3" || { echo unreadable; return; }
  age=$(( $(now) - $3 ))
  if [ "$age" -gt "${ANTIMESS_ORPHAN_AGE_S:-300}" ]; then echo orphan; else echo young; fi
}
det_AM_P1() {
  local kind file state oid pk json cls ev age budget=${ANTIMESS_ORPHAN_AGE_S:-300} line op
  while IFS=$'\t' read -r kind file state oid pk json; do
    if [ "$kind" = bad ]; then _unread_op_record "$file"; continue; fi
    case "$state" in registered|running) ;; *) continue ;; esac
    line=$(lo_classify_op "$json"); cls=$(sed -n 1p <<<"$line"); ev=$(sed -n 2p <<<"$line")
    case "$cls" in
      hung) emit drift hung_op "$oid" "purpose $pk: $ev" ;;
      dead_owner) emit drift registry_row_dead_owner "$oid" "purpose $pk: $ev" "reapop:$oid" ;;
      unreadable) emit unread corrupt_op_record "$oid" "$ev" ;;
    esac
  done < <(lo_ops_snapshot)
  local lst rc cid cat opl created cc; lst=$(_p1_list); rc=$?
  if [ $rc -ne 0 ]; then emit unread containers_unread podman "podman ps failed or was unparsable (rc $rc): $(head -c 160 "$W/pod.err" 2>/dev/null); the containers were NOT evaluated (never clean)"; return; fi
  lo_lineage_build
  while IFS=$'\t' read -r cid cat opl created; do
    [ -n "$cid" ] || continue
    if ! lo_uint "$created"; then emit unread containers_unread "$cid" "the container's Created '$created' is not a canonical integer: its age cannot be judged (never read as 'now')"; continue; fi
    op=$cat; [ "$op" != - ] || op=$opl
    cc=$(_p1_class "$cat" "$opl" "$created"); age=$(( $(now) - created ))
    case "$cc" in
      nolabel) emit drift container_without_op_label "$cid" "labelled project=catalogizer but carries neither a catalogizer.op_id nor an op_id label (cannot be matched to a registry row)" ;;
      terminal) if lo_safe_name "$cid"; then emit drift container_of_terminal_op "$cid" "op $op is terminal but its labelled container still runs" "stopcontainer:$cid:terminal"; else emit drift container_of_terminal_op "$cid" "op $op is terminal but its labelled container still runs (the container id is not a safe name: no action attached)"; fi ;;
      handoff) emit info container_of_handoff_op "$cid" "op $op is handed off (re-adoptable, 11.4.232 D): its container is never stopped by the sweep" ;;
      orphan) emit drift orphan_container "$cid" "op_id=$op has no row in THIS checkout's registry; age ${age}s > ${budget}s; reported only: absence from one registry is not proof of staleness (another checkout, track or scratch copy may own it, 11.4.232 E), stop it by hand once its owner is known" ;;
      young) emit info orphan_container_young "$cid" "op_id=$op has no registry row yet; age ${age}s <= ${budget}s" ;;
      live) ;;
      unreadable) emit unread container_label_unresolved "$cid" "the label of this container (catalogizer.op_id '$cat', op_id '$opl') cannot be resolved to ONE op lineage (a record of ops/ fails the record shape, or the label belongs to more than one lineage): the container is neither judged nor stopped, and never reported clean" ;;
    esac
  done <<<"$lst"
}
needle_AM_P1() {
  local n="$W/nd-p1"; mkdir -p "$n"
  sleep 300 >/dev/null 2>&1 & local P1=$!; sleep 300 >/dev/null 2>&1 & local P2=$!
  local o ok=0
  o=$( LD=$n/ld; mkdir -p "$LD/ops"; LONGOPS_NOW=1000
       _mkop "$LD/ops" hungop pa running "$P1" 100 30; _mkop "$LD/ops" goodop pb running "$P2" 990 30; _mkop "$LD/ops" doneop pc complete "$P2" 1
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
[{"Id":"aaaaaaaaaaaaaaaaaaaa","Labels":{"op_id":"ghost","project":"catalogizer"},"Created":100},{"Id":"bbbbbbbbbbbbbbbbbbbb","Labels":{"op_id":"liveop","project":"catalogizer"},"Created":100},{"Id":"cccccccccccccccccccc","Labels":{"catalogizer.op_id":"liveop","project":"catalogizer"},"Created":100},{"Id":"dddddddddddddddddddd","Labels":{"project":"catalogizer"},"Created":100},{"Id":"eeeeeeeeeeeeeeeeeeee","Labels":{"catalogizer.op_id":"unregistered-client","op_id":"liveop","project":"catalogizer"},"Created":100}]
J
PE
  chmod +x "$n/podman"
  o=$( LD=$n/ld2; mkdir -p "$LD/ops"; LONGOPS_NOW=1000; _mkop "$LD/ops" liveop lp running "$$" 1000 0; PODMAN=$n/podman; det_AM_P1 )
  grep -q "orphan_container${F}aaaaaaaaaaaa" <<<"$o" && ! grep -q "bbbbbbbbbbbb\|cccccccccccc\|eeeeeeeeeeee" <<<"$(grep '^drift' <<<"$o" | grep -v registry_row_dead_owner)" || { echo "orphan container not reported or a registered one (op_id or catalogizer.op_id label, or both labels with different values) reported: [$o]"; return 1; }
  grep -q "container_without_op_label${F}dddddddddddd" <<<"$o" || { echo "a container with no op label at all not reported: [$o]"; return 1; }
}
det_AM_P2() {
  local kind file state oid pk json recs="" p ids
  while IFS=$'\t' read -r kind file state oid pk json; do
    if [ "$kind" = bad ]; then _unread_op_record "$file"; continue; fi
    recs+="$json"$'\n'
  done < <(lo_ops_snapshot)
  [ -n "$recs" ] || return 0
  # an op is excused from the duplicate count ONLY when its attached_to / superseded_by names an EXISTING non-terminal op of the SAME purpose; a field that names nothing (0, "ghost", []) hides nothing (LO-A8)
  while IFS=$'\t' read -r p ids; do [ -n "$p" ] || continue
    emit drift duplicate_owner "$p" "more than one non-terminal operation holds purpose_key $p: $ids"
  done < <(jq -rs '
    def nonterm: (.state|IN("registered","running"));
    . as $all
    | [ .[] | select(nonterm) ] as $nt
    | def attached: . as $o | ([$o.attached_to, $o.superseded_by] | map(select(. != null)) | any(. as $a | $all | any(.[]; .op_id==$a and .purpose_key==$o.purpose_key and nonterm)));
      [ $nt[] | select(attached | not) ] | group_by(.purpose_key) | map(select(length>1)) | .[] | [.[0].purpose_key, (map(.op_id)|join(","))] | @tsv' <<<"$recs")
}
needle_AM_P2() {
  local o; o=$( LD=$W/nd-p2; mkdir -p "$LD/ops"; LONGOPS_NOW=1000
    _mkop "$LD/ops" a1 dup running "$$" 1000; _mkop "$LD/ops" a2 dup running "$$" 1000; _mkop "$LD/ops" b1 ok running "$$" 1000
    _mkop "$LD/ops" c1 att running "$$" 1000; _mkop "$LD/ops" c2 att running "$$" 1000; jq -c '.attached_to="c1"' "$LD/ops/c2.json" >"$LD/ops/c2.tmp" && mv "$LD/ops/c2.tmp" "$LD/ops/c2.json"
    _mkop "$LD/ops" d1 term complete "$$" 1; _mkop "$LD/ops" d2 term running "$$" 1000
    _mkop "$LD/ops" g1 ghost running "$$" 1000; _mkop "$LD/ops" g2 ghost running "$$" 1000; jq -c '.attached_to="nobody"' "$LD/ops/g2.json" >"$LD/ops/g2.tmp" && mv "$LD/ops/g2.tmp" "$LD/ops/g2.json"
    printf '{"op_id":"trunc",' >"$LD/ops/00corrupt.json"
    det_AM_P2 )
  [ "$(grep -c '^drift' <<<"$o")" -eq 2 ] && grep -q "duplicate_owner${F}dup" <<<"$o" && grep -q "duplicate_owner${F}ghost" <<<"$o" || { echo "seeded duplicate owner (and the one whose attached_to names no op) not reported (a corrupt record hid it?), or a carrier (attached op, terminal twin) reported: [$o]"; return 1; }
  grep -q "^unread${F}corrupt_op_record" <<<"$o" || { echo "seeded corrupt op record not reported unread: [$o]"; return 1; }
}

# ---------- AM-P4 purpose claims and un-adopted handoffs (WF11 F9; 11.4.232 B, D, F) ----------
det_AM_P4() {
  local d p s rc age mt minage=${ANTIMESS_LOCK_MIN_AGE:-60} op oid pk kind file state json all="" hrun
  declare -A OPSTATE=()
  local acq=",${ANTIMESS_ACQUIRE_PURPOSES:-commit_push}," anybad=0
  while IFS=$'\t' read -r kind file state oid pk json; do
    if [ "$kind" = bad ]; then anybad=1; _unread_op_record "$file"; continue; fi
    all+="$json"$'\n'; OPSTATE[$oid]=$state
  done < <(lo_ops_snapshot)
  for d in "$LD"/claims/*/; do [ -d "$d" ] || continue; p=${d%/}; p=${p##*/}
    s=$(lo_holder_status "$p" 2>"$W/p4.err"); rc=$?
    if [ "$rc" -eq "$RC_REFUSE" ]; then emit unread claim_holder_unread "$p" "the holder of $p could not be judged: $(head -c 160 "$W/p4.err")"; continue; fi
    if [ "$rc" -ne 0 ]; then   # a claim directory with no holder record: a crash between mkdir and the record, unless it is being written right now
      # the claim directory can be released between the check above and this read: a vanished directory is a released claim, not a claim of age "now" (class A, LO-A11)
      mt=$(stat -c %Y -- "${d%/}" 2>/dev/null) && lo_uint "$mt" && [ -d "${d%/}" ] || continue
      age=$(( $(now) - mt ))
      if [ "$age" -lt "$minage" ]; then emit info claim_young "$p" "claim directory with no holder record, age ${age}s < ${minage}s (a registration in progress)"
      else emit drift claim_without_holder "$p" "claim directory with no holder record, age ${age}s: the purpose is blocked and nothing owns it; release is scripts/longops/reap.sh --purpose $p"; fi
      continue
    fi
    case "$s" in
      dead) emit drift stale_claim "$p" "the holder is dead (resolved from /proc): the purpose is blocked (register exits 4); release is scripts/longops/reap.sh --purpose $p" ;;
      unreadable) emit drift claim_unreadable "$p" "the holder record cannot be read: the purpose is blocked and the claim is never taken over; resolve it by hand" ;;
      live|expired)
        # a claim of a purpose made by register.sh always has an op record (the record is written FIRST, LO-G1); a live holder whose run names no op is a claim nothing accounts for. A claim made by acquire.sh has none.
        case "$acq" in *",$p,"*) continue ;; esac
        hrun=$(jq -r '.run_id // ""' "$(lo_holder_file "$p")" 2>/dev/null)
        if [ -z "$hrun" ] || [ -z "${OPSTATE[$hrun]:-}" ]; then [ "$s" != live ] || [ "$anybad" = 1 ] || emit drift claim_without_op "$p" "the live holder's run '$hrun' names no op record and $p is not an acquire.sh purpose (${ANTIMESS_ACQUIRE_PURPOSES:-commit_push}): a claim nothing accounts for"
        else case "${OPSTATE[$hrun]}" in registered|running) ;; *) emit drift claim_of_terminal_op "$p" "the holder's op $hrun is ${OPSTATE[$hrun]} (terminal) but its claim still stands: the purpose is blocked; release is scripts/longops/reap.sh --purpose $p" ;; esac; fi ;;
    esac
  done
  # a handoff op is ADOPTED when the real producer re-registered the purpose: scripts/build/dispatch.sh reg_adopt registers `<base>-a<k+1>` with the same purpose_key, started no earlier than the handoff op
  # (WF14 R2-2); an explicit superseded_by / attached_to / adopted_by is accepted when it names an EXISTING op of the SAME purpose. NO other op of the purpose is an adopter (LO-E4: a later unrelated op
  # reusing a deterministic purpose key adopts nothing). Resolving the op by hand also clears it (it is no longer `handoff`).
  [ -n "$all" ] || return 0
  while IFS= read -r op; do [ -n "$op" ] || continue
    emit drift handoff_unadopted "$op" "op $op was handed off (11.4.232 D: re-adoptable) and no later attempt of the same build/purpose re-adopted it and nothing resolves it: scripts/longops/release.sh --op-id $op --state <terminal> resolves it"
  done < <(jq -rs '
    def base: .op_id | sub("-a[0-9]+$"; "");
    def attempt: (.op_id | ((capture("-a(?<n>[0-9]+)$") | .n | tonumber) // 1));
    . as $all | .[] | select(.state=="handoff") | . as $x
    | select(([$x.superseded_by, $x.attached_to, $x.adopted_by] | map(select(. != null)) | any(. as $a | $all | any(.[]; .op_id==$a and .purpose_key==$x.purpose_key))) | not)
    | select((($x|base) + "-a" + ((($x|attempt) + 1)|tostring)) as $nx | ($all | any(.[]; .op_id==$nx and .purpose_key==$x.purpose_key and .started_utc >= $x.started_utc)) | not)
    | $x.op_id' <<<"$all")
}
needle_AM_P4() {
  local n="$W/nd-p4" o; mkdir -p "$n"
  o=$( LD=$n/ld; mkdir -p "$LD/ops" "$LD/claims/stalep" "$LD/claims/nohold" "$LD/claims/badhold" "$LD/claims/livep" "$LD/claims/orphanp" "$LD/claims/termp" "$LD/claims/commit_push"; LONGOPS_NOW=$(date +%s)
       sleep 300 >/dev/null 2>&1 & P=$!
       jq -nc --arg b "$(lo_boot_id)" '{kind:"process",purpose:"stalep",run_id:"r1",pid:999999,start_time:"1",boot_id:$b,state:"running"}' >"$LD/claims/stalep/holder.json"
       printf '{"kind":"proc' >"$LD/claims/badhold/holder.json"
       jq -nc --argjson pid "$P" --arg st "$(lo_pstart "$P")" --arg b "$(lo_boot_id)" '{kind:"process",purpose:"livep",run_id:"r2",pid:$pid,start_time:$st,boot_id:$b,state:"running"}' >"$LD/claims/livep/holder.json"
       _mkop "$LD/ops" r2 livep running "$P" "$LONGOPS_NOW"
       jq -nc --argjson pid "$P" --arg st "$(lo_pstart "$P")" --arg b "$(lo_boot_id)" '{kind:"process",purpose:"orphanp",run_id:"nobody",pid:$pid,start_time:$st,boot_id:$b,state:"running"}' >"$LD/claims/orphanp/holder.json"
       jq -nc --argjson pid "$P" --arg st "$(lo_pstart "$P")" --arg b "$(lo_boot_id)" '{kind:"process",purpose:"commit_push",run_id:"cpa-run",pid:$pid,start_time:$st,boot_id:$b,state:"running"}' >"$LD/claims/commit_push/holder.json"
       jq -nc --argjson pid "$P" --arg st "$(lo_pstart "$P")" --arg b "$(lo_boot_id)" '{kind:"process",purpose:"termp",run_id:"t1",pid:$pid,start_time:$st,boot_id:$b,state:"running"}' >"$LD/claims/termp/holder.json"
       _mkop "$LD/ops" t1 termp complete "$P" 1
       touch -d '10 minutes ago' "$LD/claims/nohold"
       _mkop "$LD/ops" h1 h handoff "$P" 1; _mkop "$LD/ops" h2 h2 handoff "$P" 1; jq -c '.superseded_by="x2"' "$LD/ops/h2.json" >"$LD/ops/h2.tmp" && mv "$LD/ops/h2.tmp" "$LD/ops/h2.json"; _mkop "$LD/ops" x2 h2 complete "$P" 1
       _mkop "$LD/ops" c1 c complete "$P" 1
       # the REAL re-adoption (dispatch.sh reg_adopt): the next attempt <base>-a2 of the same purpose adopts the handoff op (h3); an EARLIER completed op of the purpose does not (h4), nor does an unrelated later op (h5)
       _mkop "$LD/ops" h3 p3 handoff "$P" 1; _mkop "$LD/ops" h3-a2 p3 running "$P" "$LONGOPS_NOW"; jq -c '.started_utc="2026-01-01T00:00:05Z"' "$LD/ops/h3-a2.json" >"$LD/ops/h3-a2.tmp" && mv "$LD/ops/h3-a2.tmp" "$LD/ops/h3-a2.json"
       _mkop "$LD/ops" h4 p4 handoff "$P" 1; jq -c '.started_utc="2026-01-01T00:00:05Z"' "$LD/ops/h4.json" >"$LD/ops/h4.tmp" && mv "$LD/ops/h4.tmp" "$LD/ops/h4.json"; _mkop "$LD/ops" p4old p4 complete "$P" 1
       _mkop "$LD/ops" h5 p5 handoff "$P" 1; _mkop "$LD/ops" later5 p5 complete "$P" 1; jq -c '.started_utc="2026-02-01T00:00:00Z"' "$LD/ops/later5.json" >"$LD/ops/later5.tmp" && mv "$LD/ops/later5.tmp" "$LD/ops/later5.json"
       det_AM_P4; lo_kill_child "$P" )
  [ "$(grep -c '^drift' <<<"$o")" -eq 8 ] && grep -q "handoff_unadopted${F}h4" <<<"$o" && grep -q "handoff_unadopted${F}h5" <<<"$o" && grep -q "stale_claim${F}stalep" <<<"$o" && grep -q "claim_without_holder${F}nohold" <<<"$o" && grep -q "claim_unreadable${F}badhold" <<<"$o" && grep -q "handoff_unadopted${F}h1" <<<"$o" \
    && grep -q "claim_without_op${F}orphanp" <<<"$o" && grep -q "claim_of_terminal_op${F}termp" <<<"$o" \
    || { echo "seeded stale claim / holderless claim / unreadable holder / claim without op / claim of a terminal op / un-adopted handoff (and the ones with only an EARLIER or an UNRELATED op of their purpose) not all reported: [$o]"; return 1; }
  grep '^drift' <<<"$o" | cut -f3 | grep -qx -e livep -e h2 -e c1 -e h3 -e commit_push && { echo "a golden-false carrier (live holder with its op, superseded handoff, complete op, acquire.sh purpose) was reported: [$o]"; return 1; }
  return 0
}

# ---------- AM-P3 builds ----------
det_AM_P3() {
  [ -d "$BUILDS_DIR" ] || return 0
  local d id suf pid st hub=0
  if [ -s "$BUILDS_DIR/hub.json" ]; then lo_alive "$(jq -r .pid "$BUILDS_DIR/hub.json" 2>/dev/null)" "$(jq -r .start_time "$BUILDS_DIR/hub.json" 2>/dev/null)" && hub=1; fi
  for d in "$BUILDS_DIR"/*/*.tmp-*-*; do [ -e "$d" ] || continue
    if [[ "$d" =~ \.tmp-([0-9]+)-([0-9]+)$ ]]; then pid=${BASH_REMATCH[1]}; st=${BASH_REMATCH[2]}
      if lo_alive "$pid" "$st"; then emit info build_tmp_live "$d" "owner pid $pid still running"
      elif _plain "$d"; then emit drift stale_build_tmp "$d" "owner pid $pid start $st is gone (resolved from /proc, 11.4.180)" "rmdir:build_tmp:$d"
      else emit unread build_tmp_name "$d" "a build temp directory whose name holds a control character: reported, no action attached"; fi
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
# _held_ok <commits.tsv>: 0 ONLY when the file is readable, EVERY non-comment row is `<repo> TAB <40- or 64-hex sha> [TAB ...]` and every listed commit is on every reachable remote tip of main/master; 1 otherwise
# or when unproven. An unreadable file or a row that does not parse (a space-delimited row, a missing sha) is NEVER read as "no held commit" (LO-A10: the run directory would be deleted with its commits un-pushed).
_held_ok() {
  local f=$1 repo sha rem tip br any rp ok=1
  [ -f "$f" ] && [ -r "$f" ] || return 1
  { : <"$f"; } 2>/dev/null || return 1
  while IFS=$'\t' read -r repo sha _ || [ -n "${repo:-}" ]; do
    case "$repo" in ""|\#*) continue ;; esac
    [[ "${sha:-}" =~ ^([0-9a-f]{40}|[0-9a-f]{64})$ ]] || { ok=0; break; }
    [ "$repo" = . ] && rp=$AM_ROOT || rp=$AM_ROOT/$repo
    any=0
    for rem in $(git -C "$rp" remote 2>/dev/null); do
      tip=""; for br in main master; do tip=$(git -C "$rp" ls-remote "$rem" "refs/heads/$br" </dev/null 2>/dev/null | awk -v r="refs/heads/$br" '$2==r{print $1;exit}'); [ -n "$tip" ] && break; done
      [ -n "$tip" ] || continue; any=1
      git -C "$rp" cat-file -e "$tip^{commit}" 2>/dev/null && git -C "$rp" merge-base --is-ancestor "$sha" "$tip" 2>/dev/null || { ok=0; break 2; }
    done
    [ "$any" = 1 ] || { ok=0; break; }
  done 2>/dev/null <"$f"
  [ "$ok" = 1 ]
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
        # the merge record is judged by its fields (class A): an absent, mistyped or non-canonical pid / start_time is NOT "the merge process is gone" (a default of 0 read as a dead owner); `repo` may be absent (the main repository)
        mrec=$(python3 -I -c 'import json, re, sys
try: d = json.load(sys.stdin)
except Exception: sys.exit(1)
if not isinstance(d, dict): sys.exit(1)
repo = d.get("repo", "."); pid = d.get("pid"); st = d.get("start_time")
if not isinstance(repo, str) or re.search(r"[\t\r\n]", repo) or isinstance(pid, bool) or not isinstance(pid, int) or not (1 < pid < 10**15) or not isinstance(st, str) or not re.fullmatch(r"(0|[1-9][0-9]*)", st): sys.exit(1)
print("\t".join([repo, str(pid), st]))' <"$d/merge.json" 2>/dev/null)
        if [ -z "$mrec" ]; then emit unread merge_record_unreadable "$id" "merge.json fails the record shape ({repo?, pid > 1, start_time}: absent, mistyped or non-canonical field, or not JSON); the merge process cannot be judged, nothing is asserted about it"; continue; fi
        IFS=$'\t' read -r mrepo mpid mst <<<"$mrec"
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
  local sorted="" i=0 age_d rows=""
  # a report.json that vanished meanwhile (a concurrent --reconcile removed the directory) is skipped, never "age = now" (class A, LO-B1)
  for d in "${finished[@]}"; do ts=$(stat -c %Y -- "$d/report.json" 2>/dev/null) && lo_uint "$ts" || continue; rows+="$ts"$'\t'"$d"$'\n'; done
  sorted=$(printf '%s' "$rows" | sort -rn)
  [ -n "$sorted" ] || return 0
  while IFS=$'\t' read -r ts d; do i=$((i+1)); age_d=$(( (now_ - ts) / 86400 ))
    [ "$i" -gt "$retain_runs" ] && [ "$age_d" -gt "$retain_days" ] || continue
    _plain "$d" || { emit unread finished_run_name "${d##*/}" "a finished run directory whose name holds a control character: reported, no action attached"; continue; }
    if [ ! -e "$d/commits.tsv" ] || _held_ok "$d/commits.tsv"; then emit drift removable_finished_run "${d##*/}" "beyond the newest $retain_runs runs and older than $retain_days days (age ${age_d}d); every commit is on every reachable remote tip of main" "rmdir:finished_run:$d"
    else emit info kept_held_commit "${d##*/}" "a commit of commits.tsv is missing from a reachable remote tip of main (or the file cannot be read as <repo> TAB <sha> rows): its record stays while it awaits its GO"; fi
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
  # a held-commit file that cannot be read as <repo> TAB <sha> rows is NEVER 'no held commit' (LO-A10): beyond both bounds the run is kept
  mkdir -p "$n/r/scripts/repo"; printf 'retain_runs=0\nretain_days=0\n' >"$n/r/scripts/repo/commit_push.conf"; mkdir -p "$b/spc"; echo '{"status":"complete"}' >"$b/spc/report.json"; touch -d '5 days ago' "$b/spc" "$b/spc/report.json"
  printf '. %s\n' "$(git -C "$n/r" rev-parse HEAD)" >"$b/spc/commits.tsv"
  o=$( AM_ROOT=$n/r; AUDIT_DIR=$n/r/.audit; CPA_APPROVED_DIR=""; read_cp_holder() { CP_HOLDER=none; }; det_INV_9 )
  grep -q "removable_finished_run${F}spc" <<<"$o" && { echo "a run whose commits.tsv is space-delimited (no sha column) was reported removable: [$o]"; return 1; }
  grep -q "kept_held_commit${F}spc" <<<"$o" || { echo "an unparsable commits.tsv was not kept: [$o]"; return 1; }
  return 0
}

# test-only detectors (ANTIMESS_TEST_MODE=1 only): a file defining extra det_* functions that a test catalogue (ANTIMESS_CATALOGUE) can point an invariant at, so that a detector that exits, reads an unset variable or
# breaks an arithmetic expansion is exercised through the real driver
if [ -n "${ANTIMESS_TEST_DETECTORS:-}" ]; then
  [ "${ANTIMESS_TEST_MODE:-}" = 1 ] || die test_hook_outside_test_mode "ANTIMESS_TEST_DETECTORS is a test hook and needs ANTIMESS_TEST_MODE=1"
  . "$ANTIMESS_TEST_DETECTORS" || die test_detectors "cannot source $ANTIMESS_TEST_DETECTORS"
fi

# ---------- driver ----------
IDS=(); declare -A DET STG REC NDL RSN ALI PLN ACT
while IFS=$'\037' read -r id det stg rec ndl pln ali acts rsn; do
  IDS+=("$id"); DET[$id]=$det; STG[$id]=$stg; REC[$id]=$rec; NDL[$id]=$ndl; PLN[$id]=$pln; ALI[$id]=$ali; ACT[$id]=$acts; RSN[$id]=$rsn
done < <(python3 -I - "$CATALOGUE" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
assert d.get("schema") == "anti-mess-catalogue/1", "catalogue schema"
for i in d["invariants"]:
    print("\x1f".join([i["id"], i["detector"], ",".join(i["stages"]), i["reconcile"], ("yes" if i["needle"] in (True, "yes") else "no"), i["plane"], ",".join(i.get("aliases", [])), ",".join(i.get("actions", [])), i.get("not_evaluated_reason", "")]))
PY
) || die catalogue "cannot read $CATALOGUE"
[ "${#IDS[@]}" -gt 0 ] || die catalogue "no invariants"
if [ -n "$ONLY" ]; then   # an unknown id (a typo, a lower-case id) is a usage refusal, never an empty successful sweep (WF11 F12)
  _o=${ONLY#,}; _o=${_o%,}; IFS=, read -r -a _ol <<<"$_o"
  [ "${#_ol[@]}" -gt 0 ] || die usage "--only names no invariant id"
  for _i in "${_ol[@]}"; do _k=0; for id in "${IDS[@]}"; do [ "$id" = "$_i" ] && _k=1; done; [ "$_k" = 1 ] || die usage "--only names an unknown invariant id '$_i' (catalogue ids: ${IDS[*]})"; done
fi
RESULTS="$W/results"; : >"$RESULTS"; REFUSE=0; DRIFT=0; NSEL=0
# _action_of_invariant <id> <action>: 0 when the catalogue lists the action's head for that invariant (the action is BOUND to the invariant that emitted it, class C)
_action_of_invariant() { local x al; IFS=, read -r -a al <<<"${ACT[$1]}"; for x in "${al[@]:-}"; do [ -n "$x" ] || continue; case "$2" in "$x":*) return 0 ;; esac; done; return 1; }
reconcile_action() {  # <id> <action> -> prints a result word; the action must belong to the invariant, and every action re-verifies its FULL predicate AT ACTION TIME on CANONICAL paths (WF11 F2/F10, class C)
  local id=$1 a arg; _action_of_invariant "$id" "$2" || { echo refused_action_not_of_invariant; return; }
  a=${2%%:*}; arg=${2#*:}
  [ -z "${ANTIMESS_TEST_BEFORE_ACTION:-}" ] || bash "$ANTIMESS_TEST_BEFORE_ACTION" "$2" >/dev/null 2>&1
  case "$a" in
    rmlock) local p gd mt age
      [ ! -L "$arg" ] || { echo refused_not_a_git_lock; return; }
      p=$(realpath -e -- "$arg" 2>/dev/null) || { echo skipped_precondition_changed; return; }
      gd=$(git -C "$AM_ROOT" rev-parse --absolute-git-dir 2>/dev/null) && gd=$(realpath -e -- "$gd" 2>/dev/null) || { echo refused_not_a_git_lock; return; }
      case "$p" in "$gd"/*.lock) ;; *) echo refused_not_a_git_lock; return ;; esac
      [ -f "$p" ] || { echo refused_not_a_git_lock; return; }
      mt=$(stat -c %Y -- "$p" 2>/dev/null) && lo_uint "$mt" || { echo skipped_precondition_changed; return; }
      age=$(( $(now) - mt ))
      if [ -e "$p" ] && ! _open_by_any "$p" >/dev/null && [ "$(_git_active)" -eq 0 ] && [ "$age" -ge "${ANTIMESS_LOCK_MIN_AGE:-60}" ]; then rm -f -- "$p" && echo removed || echo failed; else echo skipped_not_provably_stale; fi ;;
    reapop) lo_safe_name "$arg" || { echo refused_unsafe_op_id; return; }
      bash "$TOOLS/scripts/longops/reap.sh" --op-id "$arg" --expect dead_owner >/dev/null 2>&1 && echo reaped || echo refused ;;
    stopcontainer) local cid=${arg%%:*} want=${arg#*:} lst c3 cat3 op3 cr3 now_class=gone
      lo_safe_name "$cid" || { echo refused_unsafe_container_id; return; }
      lst=$(_p1_list) || { echo skipped_containers_unreadable; return; }
      lo_lineage_build
      while IFS=$'\t' read -r c3 cat3 op3 cr3; do [ "$c3" = "$cid" ] || continue; now_class=$(_p1_class "$cat3" "$op3" "$cr3"); done <<<"$lst"
      if [ "$now_class" != "$want" ]; then echo "skipped_precondition_changed_${now_class}"; else
        timeout "$LO_PODMAN_TIMEOUT" "$PODMAN" stop -t 5 "$cid" >/dev/null 2>&1 && echo stopped || echo failed; fi ;;
    rmdir) local kind=${arg%%:*} path=${arg#*:} pc bd rel
      [ ! -L "$path" ] || { echo refused_symlink; return; }
      case "$kind" in
        build_tmp)
          pc=$(realpath -e -- "$path" 2>/dev/null) && bd=$(realpath -e -- "$BUILDS_DIR" 2>/dev/null) || { echo skipped_precondition_changed; return; }
          case "$pc" in "$bd"/*) rel=${pc#"$bd"/} ;; *) echo refused_outside_builds; return ;; esac
          [[ "$rel" =~ ^[^/]+/[^/]+\.tmp-([0-9]+)-([0-9]+)$ ]] || { echo refused_not_a_build_tmp; return; }
          if [ -d "$pc" ] && ! lo_alive "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"; then rm -rf -- "$pc" && echo removed || echo failed
          else echo skipped_precondition_changed; fi ;;
        finished_run)
          local rep st4 cb
          pc=$(realpath -e -- "$path" 2>/dev/null) && cb=$(realpath -e -- "$AUDIT_DIR/commit-push" 2>/dev/null) || { echo skipped_precondition_changed; return; }
          [ "$(dirname "$pc")" = "$cb" ] || { echo refused_outside_commit_push; return; }
          rep="$pc/report.json"
          [ -s "$rep" ] || { echo skipped_precondition_changed; return; }
          st4=$(jq -r '.status // ""' "$rep" 2>/dev/null); case "$st4" in awaiting_remote_checks|ready_to_resume) echo skipped_precondition_changed; return ;; esac
          if [ -e "$pc/commits.tsv" ] && ! _held_ok "$pc/commits.tsv"; then echo skipped_held_commit_not_on_remote; return; fi
          rm -rf -- "$pc" && echo removed || echo failed ;;
        *) echo unknown_rmdir_kind ;;
      esac ;;
    initsub) case "$arg" in ""|-*) echo refused_unsafe_path; return ;; esac; [[ "/$arg/" != */../* ]] || { echo refused_unsafe_path; return; }
      if git -C "$AM_ROOT" submodule status --recursive -- "$arg" 2>/dev/null | grep -q '^-'; then git -C "$AM_ROOT" submodule update --init -- "$arg" >/dev/null 2>&1 && echo initialised || echo failed; else echo skipped_already_initialised; fi ;;
    *) echo unknown_action ;;
  esac
}
# _run_det <id> <out> : run the detector of <id> in a SUBSHELL and prove it reached its end (sentinel). 0 completed; 1 abnormal end (a fatal expansion error, an unbound variable, an exit): the detector is BLIND.
_run_det() { local fn=${DET[$1]}; rm -f "$W/ok.$1"; ( "$fn"; : >"$W/ok.$1" ) >"$2" 2>"$W/err.$1"; [ -e "$W/ok.$1" ]; }
_status_of() { if grep -q '^drift' "$1"; then echo drift; elif grep -q '^unread' "$1"; then echo unread; else echo clean; fi; }
_findings_json() { jq -R -s -c 'split("\n")|map(select(length>0)|split("\t"))|map({severity:.[0],class:.[1],subject:.[2],evidence:.[3],action:(.[4]//"")})' <"$1"; }
for id in "${IDS[@]}"; do
  [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) continue ;; esac
  NSEL=$((NSEL+1))
  status=""; needle=not_applicable; reason=""; fl="$W/f.$id"; : >"$fl"; stderr_txt=""; detected_json=""
  case ",${STG[$id]}," in *",$STAGE,"*) ;; *) status=skipped_stage; reason="not evaluated at stage $STAGE (catalogue stages: ${STG[$id]})" ;; esac
  if [ -z "$status" ] && [ "${DET[$id]}" = none ]; then status=not_evaluated; reason=${RSN[$id]}; fi
  if [ -z "$status" ]; then
    fn=${DET[$id]}; declare -F "$fn" >/dev/null || { status=blind; reason="detector function $fn is missing"; }
    if [ -z "$status" ] && [ "${NDL[$id]}" = yes ]; then
      nfn=needle_${id//-/_}; if out=$($nfn 2>&1); then needle=seen; else needle=blind; status=blind; reason="control needle failed: $out"; fi
    fi
    if [ -z "$status" ]; then
      if _run_det "$id" "$fl"; then
        status=$(_status_of "$fl")
      else status=blind; reason="the detector ended abnormally before its end (a fatal expansion error or an exit): $(head -c 200 "$W/err.$id" 2>/dev/null)"; fi
      grep -q '^unread' "$fl" && [ "$status" != blind ] && reason=$(grep '^unread' "$fl" | cut -f4 | head -1)
    fi
  fi
  detected_json=$(jq -nc --arg st "$status" --argjson f "$(_findings_json "$fl")" '{status:$st,findings:$f}')
  rec_json='[]'
  if [ "$RECON" = 1 ] && [ "${REC[$id]}" = auto-safe ] && [ "$status" = drift ]; then
    # reconcile, then re-derive the state: at most 3 passes, an action is never repeated; the FINAL status and exit come from the LAST pass (class B: the exit code is not computed from the detection pass)
    declare -A DONE=(); pass=1
    while [ "$pass" -le 3 ]; do
      changed=0
      while IFS= read -r line; do
        act=${line##*$'\t'}; [ -n "$act" ] && [ "$line" != "$act" ] || continue
        [ -z "${DONE[$act]:-}" ] || continue; DONE[$act]=1
        _unesc act_run "$act"; res=$(reconcile_action "$id" "$act_run" </dev/null)
        rec_json=$(jq -c --arg a "$act" --arg r "$res" '. + [{action:$a,result:$r}]' <<<"$rec_json")
        case "$res" in removed|reaped|stopped|initialised) changed=1 ;; esac
      done <"$fl"
      [ "$changed" = 1 ] || break
      pass=$((pass+1)); [ "$pass" -le 3 ] || break
      if _run_det "$id" "$fl"; then status=$(_status_of "$fl"); else status=blind; reason="the detector ended abnormally on pass $pass: $(head -c 200 "$W/err.$id" 2>/dev/null)"; break; fi
      grep -q '^unread' "$fl" && reason=$(grep '^unread' "$fl" | cut -f4 | head -1)
      [ "$status" = drift ] || break
    done
    unset DONE
  fi
  [ -s "$W/err.$id" ] && stderr_txt=$(head -c 300 "$W/err.$id")
  [ "$status" = drift ] && DRIFT=$((DRIFT+1)); [ "$status" = blind ] && REFUSE=1
  { [ "$status" = unread ] || grep -q '^unread' "$fl"; } && UNREAD=$((UNREAD+1))
  grep -q "^drift${F}blocking_hooks_path" "$fl" && [ "$STAGE" = S0 ] && REFUSE=1
  findings=$(_findings_json "$fl")
  jq -nc --arg id "$id" --arg al "${ALI[$id]}" --arg pl "${PLN[$id]}" --arg st "$status" --arg nd "$needle" --arg rs "$reason" --arg rc "${REC[$id]}" --arg se "$stderr_txt" --argjson det "$detected_json" --argjson f "$findings" --argjson r "$rec_json" \
    '{id:$id,aliases:($al|split(",")|map(select(length>0))),plane:$pl,status:$st,detected:$det,control_needle:$nd,reason:$rs,reconcile_class:$rc,stderr:$se,findings:$f,reconciled:$r}' >>"$RESULTS"
done
# completeness: ONE result row per selected invariant (a driver loop abandoned half way used to print a short report with exit 0)
NROWS=$(wc -l <"$RESULTS" 2>/dev/null) || NROWS=-1
if [ "$NROWS" -ne "$NSEL" ]; then echo "sweep: driver_incomplete: $NROWS result row(s) for $NSEL selected invariant(s)" >&2; exit 20; fi
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
