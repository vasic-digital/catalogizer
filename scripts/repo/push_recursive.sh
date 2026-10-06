#!/usr/bin/env bash
# T041 helper: push_recursive.sh - CPA stage S6 push: per repository and remote the longest releasable prefix, never a force.
#
# Usage   push_recursive.sh --main-root <repo> --branch <b> --run-dir <dir> [--audit-dir <dir>] [--repo <path> ...] [--recursive]
#                           [--schema <path>] [--timeout <s>]
#   --main-root  the main repository; verdict files and the verdict schema are read from ITS committed HEAD, whatever repository is pushed
#   --branch     the branch pushed in every repository (`<sha>:refs/heads/<branch>`)
#   --run-dir    the run directory (must exist); its parent is the audit directory unless --audit-dir is given: `<audit>/<run id>/commits.tsv`
#                holds the rows (repository, sha, run id) of the runs' own commits (clause (a) of the T042 CPA-commit predicate)
#   --repo       push only this repository (a path relative to the main root, a literal canonical path or `.` for the main repository;
#                repeatable; a directory that is not the root of its own work tree, e.g. an uninitialised submodule, is `not_a_repository`); --recursive: the main repository and every
#                submodule at every depth, found from the gitlinks (never from .gitmodules), deepest first; default: the main repository
#   --schema     review-verdict schema path relative to the main root (default specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json)
#   --timeout    seconds each `git ls-remote` and each `git push` may take (default 60); a call that exceeds it is an unreachable remote / a failed push
# Per repository   outgoing U = `git rev-list --topo-order --reverse <branch> --not <every live remote tip held locally>`:
#   - a commit of U that is not a CPA commit (trailer `CPA-Run: <id>` AND a row with its sha in `<audit>/<id>/commits.tsv`; a missing,
#     copied or forged trailer proves nothing) withholds every push of the repository: exit 20 `unrecorded_local_commit` naming it
#     (the "no GO committed in HEAD lists it" clause of the predicate is T042's, not built here);
#   - a commit with `Awaits-Review: <verdict path>` is held until that verdict, read from the main HEAD (`git show HEAD:<path>`), validates
#     against the schema (also read from HEAD), holds `verdict: GO`, `blocking_findings` 0 and lists the commit's run in `covers_runs`;
#     an unreadable, invalid or uncommitted verdict keeps the hold; the target is the newest commit below the first unreleased held one,
#     and while U holds an unreleased merge commit (more than one parent) nothing of U is pushed to any remote;
#   - per remote: its live tip is read with `git ls-remote`; the target is pushed as `git push -- <remote> <sha>:refs/heads/<branch>` only
#     when that tip is an ancestor of the target (a remote without the branch has none and is pushed); a tip that is not an ancestor, or
#     whose objects are not held locally, gives no push call and `remote_moved_since_s1`; a remote that already holds the target is not pushed.
# Unreachable remote  its live tip cannot be read. What it last held is taken from the last fetched tip refs/remotes/<remote>/<branch> when this clone has
#         one (never "it holds nothing"); an unreachable remote with NO such tip is unknown: while a non-CPA local-only commit exists, nothing is
#         pushed and every unreachable remote is reported `NOPUSH <repo> <remote> remote_unreachable` (exit 11), not `unrecorded_local_commit`
#         (WF3 review I-1). With every remote unreachable the same NOPUSH is reported for all of them.
# Moved remote (WF6 W6-1)  a live tip whose objects are not held locally means the remote moved after S1. The stand-in for what it holds is the last
#         FETCHED tip refs/remotes/<remote>/<branch>, which S1's fetch (`--refmap=`, no ref written) never updates, so after an S1 fast-forward it is STALE and
#         a commit that S1 itself integrated and every remote already holds would look unrecorded. So, CONSERVATIVELY: whenever non-CPA local commits
#         are found AND any remote has moved, nothing is pushed and each moved or unreachable remote is `NOPUSH <repo> <remote> remote_moved_since_s1` /
#         `remote_unreachable` (exit 11): the commits are judged again by S1 once the moved tips are fetched. Exit 20 `unrecorded_local_commit` remains
#         ONLY when no remote moved and none is unknown. RESIDUAL (stated, not hidden): a genuinely unrecorded commit is reported 11 instead of 20
#         while a remote has moved. Whether S1 should write refs/remotes/<r>/<branch> after a successful fetch (changes the T032 "no ref written"
#         form) is the owner decision F2 (wp04e-notes.md owed item 1); this helper does not decide it.
# Output  PUSHED <repo> <remote> <sha> | PUSH_FAILED <repo> <remote> <sha> <error> | NOPUSH <repo> <remote> <reason> (already_holds_tip,
#         remote_moved_since_s1, remote_unreachable, no_target, held_below_remote_tip: the remote's tip is already inside the local branch and
#         the releasable target lies below it because a held commit is unreleased; counted as the hold, exit 14, not as a moved remote) | HELD <repo> <sha> <verdict> <reason> | REFUSED <repo> <reason> <detail>
# Exits   0 done or nothing to do; 11 a remote moved, was unreachable or rejected the push; 14 only holds withheld commits;
#         20 refusal (force-like option, unsafe value, unrecorded_local_commit, unsafe remote name or URL, unsafe gitlink path under --recursive,
#         and `git_listing_failed`: a gitlink listing, the remote list, the outgoing-commit list (`rev-list`) or a commit's parent list that git
#         could not produce is never read as "nothing there"; WF5 F3, WF6 W6-2, W6-12). Precedence 20, 11, 14, 0.
#         Exit codes 11 and 14 follow T042; their use for a rejected push and for holds is UNCONFIRMED until the T042 owner accepts it.
# Never   a force, a delete, a mirror, a rebase, a reset; every remote name and URL is validated, every operand follows `--`; the
#         tests use local bare remotes only.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$D/lib_safe.sh"
die() { echo "push_recursive: $1: $2" >&2; exit 20; }
for a in "$@"; do
  case "$a" in --force|--force=*|--force-with-lease|--force-with-lease=*|-f|+*|--mirror|--delete|-d|--prune|--all|--tags) die force_refused "$(printf '%q' "$a")" ;; esac
done
MAIN=""; BR=""; RUND=""; AUD=""; REPOS=(); REC=0; TMO=60; SCHEMA="specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json"
while [ $# -gt 0 ]; do
  case "$1" in
    --main-root) [ $# -ge 2 ] || die usage "--main-root needs a value"; MAIN="$2"; shift 2 ;;
    --branch) [ $# -ge 2 ] || die usage "--branch needs a value"; BR="$2"; shift 2 ;;
    --run-dir) [ $# -ge 2 ] || die usage "--run-dir needs a value"; RUND="$2"; shift 2 ;;
    --audit-dir) [ $# -ge 2 ] || die usage "--audit-dir needs a value"; AUD="$2"; shift 2 ;;
    --repo) [ $# -ge 2 ] || die usage "--repo needs a value"; REPOS+=("$2"); shift 2 ;;
    --recursive) REC=1; shift ;;
    --schema) [ $# -ge 2 ] || die usage "--schema needs a value"; SCHEMA="$2"; shift 2 ;;
    --timeout) [ $# -ge 2 ] || die usage "--timeout needs a value"; TMO="$2"; shift 2 ;;
    *) die usage "unknown argument $(printf '%q' "$1")" ;;
  esac
done
safe_dir_arg "$MAIN" || die unsafe_main_root "$(printf '%q' "$MAIN")"
[[ "$TMO" =~ ^[1-9][0-9]*$ ]] || die usage "--timeout must be a positive integer"
safe_branch "$BR" || die unsafe_branch "$(printf '%q' "$BR")"
safe_dir_arg "$RUND" || die unsafe_run_dir "$(printf '%q' "$RUND")"
[ -d "$RUND" ] || die run_dir_absent "$(printf '%q' "$RUND")"
RUND="$(cd "$RUND" && pwd)"; [ -n "$AUD" ] || AUD="$(dirname "$RUND")"
safe_dir_arg "$AUD" || die unsafe_audit_dir "$(printf '%q' "$AUD")"; [ -d "$AUD" ] || die audit_dir_absent "$(printf '%q' "$AUD")"
safe_relpath "$SCHEMA" || die unsafe_schema "$(printf '%q' "$SCHEMA")"
[ -d "$MAIN" ] || die not_a_repository "$(printf '%q' "$MAIN")"
MAIN="$(cd "$MAIN" && git rev-parse --show-toplevel 2>/dev/null)" || die not_a_repository "main root"
for r in "${REPOS[@]+"${REPOS[@]}"}"; do [ "$r" = . ] || safe_declpath "$r" || die unsafe_repo "$(printf '%q' "$r")"; done
export GIT_ALLOW_PROTOCOL=file:ssh:git:https GIT_TERMINAL_PROMPT=0   # no GIT_LITERAL_PATHSPECS: no git call here takes a declared path, and it would leak into pre-push hooks
tm() { timeout "$TMO" "$@"; }
# is_root <dir>: the directory is the root of its own work tree (an uninitialised submodule is an empty directory inside the parent, and
# `git -C <it>` would silently judge the PARENT; WF2 review I4)
is_root() { [ -d "$1" ] && [ "$(git -C "$1" rev-parse --show-toplevel 2>/dev/null)" = "$(cd "$1" 2>/dev/null && pwd -P)" ]; }
W="$(mktemp -d "${TMPDIR:-/tmp}/push_recursive.XXXXXX")" || die internal mktemp; trap 'rm -rf "$W"' EXIT; trap 'exit 143' TERM; trap 'exit 130' INT
# repositories: explicit, recursive (gitlinks), or the main repository; deepest first
# discover runs in THIS shell (no command or process substitution around it): a refusal (`die`) is effective, and a failing listing is reported through
# die itself, never through a marker line that shares a stream with the submodule paths (WF6 W6-3, W6-4). Each listing goes to a file first so its
# status is read (a pipe or a process substitution would lose it).
DISC=()
discover() { # discover <dir> <prefix>: append every initialised submodule below <dir> to DISC
  local dir="$1" pre="$2" sp e lf nm subs=()
  nm="${pre%/}"; nm="${nm:-.}"
  lf="$W/ls.$RANDOM.$RANDOM"
  git -C "$dir" ls-files -s -z >"$lf" 2>/dev/null || { rm -f "$lf"; die git_listing_failed "$(printf '%q' "$nm") (gitlink listing)"; }
  while IFS= read -r -d '' e; do case "$e" in 160000\ *) subs+=("${e#*$'\t'}") ;; esac; done < "$lf"; rm -f "$lf"
  for sp in "${subs[@]+"${subs[@]}"}"; do
    safe_relpath "$sp" || die unsafe_repo "gitlink $(printf '%q' "$sp")"
    is_root "$dir/$sp" || continue
    DISC+=("$pre$sp"); discover "$dir/$sp" "$pre$sp/"
  done
}
if [ "${#REPOS[@]}" -gt 0 ]; then LIST=("${REPOS[@]}")
elif [ "$REC" = 1 ]; then LIST=("."); discover "$MAIN" ""; LIST+=("${DISC[@]+"${DISC[@]}"}")
else LIST=("."); fi
mapfile -t LIST < <(for r in "${LIST[@]}"; do if [ "$r" = . ]; then d=0; else sl="${r//[^\/]/}"; d=$((${#sl}+1)); fi; printf '%s\t%s\n' "$d" "$r"; done | sort -t$'\t' -k1,1nr -k2,2 | cut -f2)
EXIT=0
bump() { # bump <code>: precedence 20 > 11 > 14 > 0
  case "$1:$EXIT" in 20:*) EXIT=20 ;; 11:20) ;; 11:*) EXIT=11 ;; 14:0) EXIT=14 ;; esac
}
# release state of a verdict path for a run id: prints ok or a reason
release_state() { # release_state <verdict path> <run id>
  local vp="$1" rid="$2"
  safe_relpath "$vp" || { echo unsafe_path; return; }
  git -C "$MAIN" cat-file -e "HEAD:$vp" 2>/dev/null || { echo absent; return; }
  git -C "$MAIN" cat-file -e "HEAD:$SCHEMA" 2>/dev/null || { echo schema_absent; return; }
  git -C "$MAIN" cat-file blob "HEAD:$vp" > "$W/verdict.json" 2>/dev/null; git -C "$MAIN" cat-file blob "HEAD:$SCHEMA" > "$W/schema.json" 2>/dev/null
  python3 - "$W/schema.json" "$W/verdict.json" "$rid" <<'PY'
import json, sys
try:
    schema = json.load(open(sys.argv[1])); v = json.load(open(sys.argv[2]))
except Exception: print('invalid'); sys.exit()
try:
    import jsonschema; jsonschema.validate(v, schema)
except ImportError:
    for k in schema.get('required', []):
        if k not in v: print('invalid'); sys.exit()
except Exception: print('invalid'); sys.exit()
if v.get('verdict') != 'GO' or v.get('blocking_findings') != 0: print('not_go'); sys.exit()
print('ok' if any(isinstance(e, dict) and e.get('cpa_run') == sys.argv[3] for e in (v.get('covers_runs') or [])) else 'not_covered')
PY
}
trailer() { git -C "$1" log -1 --format=%B "$2" 2>/dev/null | awk -v k="$3" 'index($0,k": ")==1 {v=substr($0,length(k)+3)} END{print v}'; }
for key in "${LIST[@]}"; do
  if [ "$key" = . ]; then dir="$MAIN"; else dir="$MAIN/$key"; fi
  is_root "$dir" || { printf 'REFUSED\t%s\tnot_a_repository\t-\n' "$key"; bump 20; continue; }
  g() { git -C "$dir" "$@"; }
  # remotes and their URLs, validated before any use
  REM=(); bad=0
  # the remote list is read first, so its status is not lost (a failing `git remote` is no "no remotes"; WF6 W6-12)
  if ! rl="$(g remote 2>/dev/null)"; then printf 'REFUSED\t%s\tgit_listing_failed\tremote list\n' "$key"; bump 20; continue; fi
  while IFS= read -r r; do
    [ -n "$r" ] || continue
    if ! safe_remote "$r"; then printf 'REFUSED\t%s\tunsafe_remote_name\t%s\n' "$key" "$(printf '%q' "$r")"; bad=1; continue; fi
    while IFS= read -r u; do
      safe_url "$u" || { printf 'REFUSED\t%s\tunsafe_remote_url\t%s %s\n' "$key" "$r" "$(printf '%q' "$u")"; bad=1; }
    done < <(g config --get-all "remote.$r.url" 2>/dev/null; g config --get-all "remote.$r.pushurl" 2>/dev/null)
    REM+=("$r")
  done <<< "$rl"
  [ "$bad" = 0 ] || { bump 20; continue; }
  if ! g rev-parse --verify -q "refs/heads/$BR" >/dev/null; then printf 'NOPUSH\t%s\t-\tbranch_absent\n' "$key"; continue; fi
  LT="$(g rev-parse "refs/heads/$BR")"
  # live tips
  declare -A TIP=(); declare -A UNR=(); declare -A MOVED=(); KNOWN=(); UNK=0
  for r in "${REM[@]+"${REM[@]}"}"; do
    if ! lr="$(tm git -C "$dir" ls-remote -- "$r" "refs/heads/$BR" 2>"$W/lserr")"; then
      UNR["$r"]=1; kt="$(known_tip "$dir" "$r" "$BR")"; if [ -n "$kt" ]; then KNOWN+=("$kt"); else UNK=1; fi; continue; fi
    t="$(printf '%s\n' "$lr" | lr_exact "$BR")"; TIP["$r"]="$t"
    if [ -n "$t" ] && safe_sha "$t" && g cat-file -e "$t^{commit}" 2>/dev/null; then KNOWN+=("$t")
    elif [ -n "$t" ]; then
      # a live tip whose objects are not held locally (the remote moved after S1; fetch is not allowed here) is never left out silently:
      # stand in with the last fetched tip of that remote, or, with none, mark the outgoing set not computable (WF5 review F1)
      MOVED["$r"]=1; kt="$(known_tip "$dir" "$r" "$BR")"; if [ -n "$kt" ]; then KNOWN+=("$kt"); else UNK=1; fi
    fi
  done
  # every remote unreachable: nothing is known about what any of them holds, so no outgoing set can be computed; naming the whole history
  # as unrecorded would be a false refusal (WF2 review I8). Each remote is reported unreachable and nothing is pushed.
  if [ "${#REM[@]}" -gt 0 ] && [ "${#UNR[@]}" -eq "${#REM[@]}" ]; then
    for r in "${REM[@]}"; do printf 'NOPUSH\t%s\t%s\tremote_unreachable\n' "$key" "$r"; done
    bump 11; unset TIP UNR MOVED; continue
  fi
  # outgoing commits and the CPA predicate
  # captured first: `mapfile < <(...)` would lose rev-list's status and a failing listing would read as "nothing outgoing" (WF6 W6-2)
  if ! ul="$(g rev-list --topo-order --reverse "refs/heads/$BR" ${KNOWN[@]+--not "${KNOWN[@]}"} -- 2>/dev/null)"; then
    printf 'REFUSED\t%s\tgit_listing_failed\toutgoing commit list\n' "$key"; bump 20; unset TIP UNR MOVED; continue
  fi
  U=(); [ -z "$ul" ] || mapfile -t U <<< "$ul"
  UREC=()
  for c in "${U[@]+"${U[@]}"}"; do
    rid="$(cpa_runid "$dir" "$c")" && cpa_row "$AUD/$rid/commits.tsv" "$c" || UREC+=("$c")
  done
  if [ "${#UREC[@]}" -gt 0 ]; then
    if [ "$UNK" = 1 ] || [ "${#MOVED[@]}" -gt 0 ]; then
      # an unreachable remote with no known tip may hold these commits, and a remote whose live tip is not held locally has moved since S1 (the stand-in
      # tip is only the last FETCHED one, which S1's refmap-less fetch never updates): nothing can be said about them, nothing is pushed (WF3 review I-1;
      # WF6 W6-1: a published commit must never be named unrecorded, so unrecorded commits plus a moved remote are 11, not 20)
      for r in "${REM[@]}"; do
        if [ -n "${UNR[$r]+x}" ]; then printf 'NOPUSH\t%s\t%s\tremote_unreachable\n' "$key" "$r"
        elif [ -n "${MOVED[$r]+x}" ]; then printf 'NOPUSH\t%s\t%s\tremote_moved_since_s1\n' "$key" "$r"; fi
      done; bump 11
    else
      for c in "${UREC[@]}"; do printf 'REFUSED\t%s\tunrecorded_local_commit\t%s\n' "$key" "$c"; done; bump 20
    fi
    unset TIP UNR MOVED; continue
  fi
  # holds
  cut=-1; mergeblock=0; idx=0; held_any=0
  for c in "${U[@]+"${U[@]}"}"; do
    vp="$(trailer "$dir" "$c" Awaits-Review)"
    if [ -n "$vp" ]; then
      rid="$(trailer "$dir" "$c" CPA-Run)"; st="$(release_state "$vp" "$rid")"
      if [ "$st" != ok ]; then
        printf 'HELD\t%s\t%s\t%s\t%s\n' "$key" "$c" "$vp" "$st"; held_any=1
        [ "$cut" -ge 0 ] || cut="$idx"
        if ! pl="$(g rev-list --parents -n1 "$c" 2>/dev/null)"; then printf 'REFUSED\t%s\tgit_listing_failed\tparents of %s\n' "$key" "$c"; bump 20; unset TIP UNR MOVED; continue 2; fi
      [ "$(printf '%s\n' "$pl" | awk '{print NF-1}')" -le 1 ] || mergeblock=1
      fi
    fi
    idx=$((idx+1))
  done
  target=""
  if [ "${#U[@]}" -eq 0 ]; then target="$LT"
  elif [ "$mergeblock" = 1 ] || [ "$cut" = 0 ]; then target="$(g rev-parse -q --verify "${U[0]}^" 2>/dev/null || true)"
  elif [ "$cut" -gt 0 ]; then target="${U[$((cut-1))]}"
  else target="${U[$((${#U[@]}-1))]}"; fi
  [ "$mergeblock" = 1 ] && [ "$cut" -gt 0 ] && target="$(g rev-parse -q --verify "${U[0]}^" 2>/dev/null || true)"
  for r in "${REM[@]+"${REM[@]}"}"; do
    if [ -n "${UNR[$r]+x}" ]; then printf 'NOPUSH\t%s\t%s\tremote_unreachable\n' "$key" "$r"; bump 11; continue; fi
    [ -n "$target" ] || { printf 'NOPUSH\t%s\t%s\tno_target\n' "$key" "$r"; continue; }
    t="${TIP[$r]-}"
    if [ -n "$t" ]; then
      if [ "$t" = "$target" ]; then printf 'NOPUSH\t%s\t%s\talready_holds_tip\n' "$key" "$r"; continue; fi
      if ! { safe_sha "$t" && g cat-file -e "$t^{commit}" 2>/dev/null && g merge-base --is-ancestor "$t" "$target" 2>/dev/null; }; then
        # a remote whose tip the local branch already contains (a held merge of that very tip) has not moved: the target is only held below
        # it, which is a hold (14), never remote_moved_since_s1 (WF2 review I9)
        if [ "$held_any" = 1 ] && safe_sha "$t" && g cat-file -e "$t^{commit}" 2>/dev/null && g merge-base --is-ancestor "$t" "$LT" 2>/dev/null; then
          printf 'NOPUSH\t%s\t%s\theld_below_remote_tip\n' "$key" "$r"; continue; fi
        printf 'NOPUSH\t%s\t%s\tremote_moved_since_s1\n' "$key" "$r"; bump 11; continue; fi
    fi
    if err="$(tm git -C "$dir" push -q -- "$r" "$target:refs/heads/$BR" 2>&1)"; then printf 'PUSHED\t%s\t%s\t%s\n' "$key" "$r" "$target"
    else printf 'PUSH_FAILED\t%s\t%s\t%s\t%s\n' "$key" "$r" "$target" "$(printf '%s' "$err" | tr '\n\t' '  ' | head -c 200)"; bump 11; fi
  done
  [ "$held_any" = 0 ] || bump 14
  unset TIP UNR MOVED
done
exit "$EXIT"
