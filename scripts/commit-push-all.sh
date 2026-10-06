#!/usr/bin/env bash
# commit-push-all.sh (CPA) - the dedicated commit and push script: hook validations as explicit stages, the commit/push mechanism always unblocked
# (constitution 11.4.234; docs/16 section 12; tasks.md T042, T042a). It never forces: no --force, no --force-with-lease, no +refspec, no --no-verify,
# no rebase, no reset, no history rewrite (11.4.113); every merge is a plain `git merge --no-ff` made by integrate_merge.sh.
#
# Start     only through the host entry point: scripts/repo/host_entry/cpa-host copies the owner-approved files into
#           .audit/commit-push/<run id>/released/ and executes this script from there (CPA_ROOT, CPA_RUN_ID, CPA_ADOPTION_COMMIT in the environment).
#           Started any other way the self-test below refuses with 20 not_via_host_entry and writes nothing.
# Usage     scripts/commit-push-all.sh [--paths-from FILE] [--awaits-review VERDICT] [--local-only] [--repo PATH] [--resolve-merge DIR]
#                                      [--commit-before-integrate] ["commit message"]          (always via cpa-host)
#   --paths-from FILE   the declared change set: one path per line relative to the main repository root, optionally a TAB and the review verdict
#                       path that must be GO (committed in the main HEAD, naming the commit's run in covers_runs) before the path may be pushed.
#                       Without it the run commits nothing and only integrates, pushes and verifies.
#   --awaits-review V   gives V to every declared path that carries no verdict of its own
#   --local-only        stages S0 to S5, S6 skipped, S7 with --no-remote; exit 14 with LOCAL_ONLY in the Deferred-Gates line of the commits
#   --repo PATH         one owned repository at any depth (T042a): commits, integrates and pushes only there, records the pending pin move in
#                       .audit/pending_pins.tsv, never moves a parent's HEAD or gitlink
#   --resolve-merge D   the resolution of a merge that an earlier run refused with merge_conflict (.audit/merge-resolution/<run id>/ only)
#   --commit-before-integrate   S1a (objects, S1a backup under backup/s1a/), S2 to S5, then S1b (integrate on top of the run's commits)
#   SKIP_LONG=<reason>  environment: the long gates are not run; recorded at S0, carried by every commit of the run, exit 14
# Stages    S0 preflight, S1 fetch+integrate (S1a/S1b), S2 scope_check, S3 validate_cheap, S4 long gates, S5 commit, S6 push, S7 verify, S8 report.
# Outputs   every file of a run lies under $CPA_RUN (.audit/commit-push/<run id>/, git-ignored) except .audit/pending_pins.tsv and the lock
#           records under .audit/longops/; the tracked tree is never written. report.json is written on EVERY exit path (S8).
# Exits     0 clean; 10 a check failed; 11 push rejected/remote unreachable/remote moved; 12 integration blocked (diverged remotes, conflict, local
#           changes); 13 scope refused or verification dirty; 14 recorded deferral or held push (never a clean pass); 15 pointer drift without a
#           pending move; 20 refusal or internal error (any helper exit outside its documented set included).
# Honest boundary (UNCONFIRMED / owed, see docs/scripts/commit-push-all.md): path gates, G-PIN accepted pins, verdict provenance, the covers_runs
#           completeness checks, remote checks (exit 16), --resume, the S2 secret fold and the CHECK_PENDING_RELEASE trailer are NOT built here.
set -u
LC_ALL=C
SELF="$(realpath -- "$0" 2>/dev/null)" || SELF="$0"
D="$(dirname "$SELF")"; RD="$D/repo"; LO="$D/longops"

# ---- the self-test (CENTRAL C3 (iv)): writes nothing, refuses with 20 -------------------------------------------------------------------
selfrefuse() { jq -nc --arg r "$1" --arg w "$2" '{refusal:$r,reason:$w,see:"scripts/repo/host_entry/INSTALL.md"}' >&2; exit 20; }
for t in git jq sha256sum realpath python3; do command -v "$t" >/dev/null 2>&1 || selfrefuse tool_missing "$t"; done
[ -n "${CPA_ROOT:-}" ] && [ -n "${CPA_RUN_ID:-}" ] || selfrefuse not_via_host_entry "CPA_ROOT or CPA_RUN_ID unset"
EXPECT="$CPA_ROOT/.audit/commit-push/$CPA_RUN_ID/released/scripts/commit-push-all.sh"
[ "$(realpath -- "$EXPECT" 2>/dev/null)" = "$SELF" ] || selfrefuse not_via_host_entry "own path is not the released copy of this run"
SNAP="$(dirname "$D")/trust-snapshot.json"
[ -r "$SNAP" ] || selfrefuse not_via_host_entry "no trust-snapshot.json beside the released copy"
want="$(jq -r '.entries[]|select(.path=="scripts/commit-push-all.sh").sha256' "$SNAP" 2>/dev/null)"
[ -n "$want" ] && [ "$want" = "$(sha256sum "$SELF" | cut -d' ' -f1)" ] || selfrefuse not_via_host_entry "own sha256 differs from the snapshot entry"
ents="$(jq -cS '.entries|sort_by(.path)' "$SNAP" 2>/dev/null)"; msha="$(printf '%s' "$ents" | sha256sum | cut -d' ' -f1)"
[ "$msha" = "$(jq -r .manifest_sha256 "$SNAP" 2>/dev/null)" ] || selfrefuse snapshot_not_trusted "manifest_sha256 of the snapshot is not the sha256 of its entries"
STATE="${CPA_HOST_STATE:-$HOME/.local/state/cpa-host}"; TRUSTF="$STATE/trust.json"
pkey="$(git -C "$CPA_ROOT" rev-list --first-parent --max-parents=0 HEAD 2>/dev/null | head -1)"
[ -s "$TRUSTF" ] || selfrefuse snapshot_not_trusted "trust file unreadable or empty"
if jq -e --arg k "$pkey" --arg m "$msha" '.projects[$k].history|map(select(.op=="revoke")|.manifest_sha256)|index($m)!=null' "$TRUSTF" >/dev/null 2>&1; then selfrefuse snapshot_not_trusted revoked; fi
jq -e --arg k "$pkey" --arg m "$msha" '.projects[$k].history|map(select(.op!="revoke")|.manifest_sha256)|index($m)!=null' "$TRUSTF" >/dev/null 2>&1 \
  || selfrefuse snapshot_not_trusted "no approval of the project records this manifest"

# ---- helpers of this script --------------------------------------------------------------------------------------------------------------
# jq 1.6 (the WP-09 images) exits 0 under -e when its input is EMPTY, jq 1.7+ does not: emptiness is therefore checked here, never left to jq
json_is() { # json_is <jq -e filter> <file>: the file is non-empty and the filter holds
  [ -s "$2" ] || return 1; jq -e "$1" "$2" >/dev/null 2>&1; }
head_json_is() { # head_json_is <jq -e filter> <path>: the committed HEAD blob exists, is non-empty and the filter holds
  local blob; blob="$(git -C "$ROOT" show "HEAD:$2" 2>/dev/null)" || return 1; [ -n "$blob" ] || return 1
  printf '%s' "$blob" | jq -e "$1" >/dev/null 2>&1; }
# shellcheck source=scripts/repo/lib_safe.sh
. "$RD/lib_safe.sh"
ROOT="$CPA_ROOT"; RUN_ID="$CPA_RUN_ID"; AUDITD="$ROOT/.audit/commit-push"; CPA_RUN="$AUDITD/$RUN_ID"; ADOPT="${CPA_ADOPTION_COMMIT:-}"
cd "$ROOT" || selfrefuse internal "cannot enter $ROOT"
LOG="$CPA_RUN/log.txt"
log() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" >> "$LOG" 2>/dev/null; }
# no hook of .git/hooks/ or .git/modules/*/hooks/ runs in any git call of this script or of a helper (CENTRAL C4)
_n="${GIT_CONFIG_COUNT:-0}"; export "GIT_CONFIG_KEY_$_n=core.hooksPath" "GIT_CONFIG_VALUE_$_n=$CPA_RUN/no-hooks"; export GIT_CONFIG_COUNT=$((_n+1))
export GIT_TERMINAL_PROMPT=0 LONGOPS_REPO="$ROOT" CPA_APPROVED_DIR="$CPA_RUN/released" CPA_RUN CPA_RUN_ID

STG_ID=(); STG_NAME=(); CUR=""; FAILSTAGE=""; FAILREASON=""; FAILDETAIL=""; LOCKED=0; HELD=0; DEFERRED=(); PENDING_CHECKS=0; NESTED=()
MSG=""; PATHS=""; AWAIT=""; REPO=""; RESOLVE=""; LOCAL_ONLY=""; CBI=""; TARGET="$ROOT"; BR=""; OWNED=""; START_HEAD=""; HELDLINES=()
stage() { CUR="$1"; STG_ID+=("$1"); STG_NAME+=("$2"); log "stage $1 $2"; }
fail() { FAILSTAGE="$1"; FAILREASON="$3"; FAILDETAIL="${4:-}"; log "FAIL $1 $2 $3 $4"; printf 'cpa: FAIL %s %s %s%s\n' "$1" "$2" "$3" "${4:+ ($4)}" >&2; exit "$2"; }
defer() { # defer <flag> <reason> [stage]: one row in the run directory (record_deferral.sh); a failed write is 20, never a 14 without its row
  "$RD/record_deferral.sh" --run-dir "$CPA_RUN" --flag "$1" --reason "$2" >>"$LOG" 2>&1 || fail "${3:-$CUR}" 20 deferral_write_failed "$1"
  DEFERRED+=("$1")
}
# shellcheck disable=SC2329  # invoked through `trap finish EXIT`
finish() { # S8: the report on every exit path, the lock released, the summary line
  local rc=$? files commits interrupted deferrals
  trap - EXIT
  case "$rc" in 0|10|11|12|13|14|15|20) ;; *) FAILSTAGE="${FAILSTAGE:-${CUR:-S0}}"; FAILREASON="${FAILREASON:-internal_error}"; FAILDETAIL="${FAILDETAIL:-exit $rc}"; rc=20 ;; esac
  if [ "$LOCKED" = 1 ]; then "$LO/release.sh" --purpose commit_push --run-id "$RUN_ID" >>"$LOG" 2>&1 || log "release.sh failed"; fi
  if [ -d "$CPA_RUN" ]; then
    files="$(cd "$CPA_RUN" && find . -type f ! -path ./report.json ! -name 'report.json.tmp' -print0 2>/dev/null | sort -z | xargs -0 -r sha256sum 2>/dev/null \
      | awk '{h=$1; $1=""; sub(/^ +/,""); sub(/^\.\//,""); print h "\t" $0}' | jq -R -s 'split("\n")|map(select(length>0)|split("\t")|{(.[1]):.[0]})|add // {}' 2>/dev/null)" || files='{}'
    [ -n "$files" ] || files='{}'
    commits="$([ -f "$CPA_RUN/commits.tsv" ] && jq -R -s 'split("\n")|map(select(length>0)|split("\t")|{repo:.[0],sha:.[1],cpa_run:.[2]})' "$CPA_RUN/commits.tsv" 2>/dev/null || echo '[]')"
    interrupted="$(cd "$AUDITD" 2>/dev/null && for d in */; do d="${d%/}"; [ "$d" = "$RUN_ID" ] && continue; [ -f "$d/report.json" ] || echo "$d"; done | jq -R -s 'split("\n")|map(select(length>0))' 2>/dev/null || echo '[]')"
    deferrals="$("$RD/record_deferral.sh" --run-dir "$CPA_RUN" --list 2>/dev/null)"
    jq -n --arg id "$RUN_ID" --arg dir ".audit/commit-push/$RUN_ID/" --argjson rc "$rc" --arg fs "$FAILSTAGE" --arg fr "$FAILREASON" --arg fd "$FAILDETAIL" \
      --arg repo "$REPO" --arg lo "$LOCAL_ONLY" --arg pf "$PATHS" --arg aw "$AWAIT" --arg rm "$RESOLVE" --arg cbi "$CBI" --arg deferred "${deferrals:-}" \
      --argjson held "$HELD" --argjson pend "$PENDING_CHECKS" --arg trust "$TRUSTF" --arg stages "$(for i in "${!STG_ID[@]}"; do printf '%s:%s\n' "${STG_ID[$i]}" "${STG_NAME[$i]}"; done)" \
      --argjson commits "${commits:-[]}" --argjson interrupted "${interrupted:-[]}" --argjson files "$files" --arg nested "${NESTED[*]:-}" --arg held_lines "$(printf '%s\n' "${HELDLINES[@]:-}")" \
      '{schema:1,run_id:$id,run_dir:$dir,exit:$rc,
        failed_stage:(if $fs=="" then null else $fs end),reason:(if $fr=="" then null else $fr end),detail:(if $fd=="" then null else $fd end),
        mode:{repo:(if $repo=="" then null else $repo end),local_only:($lo!=""),paths_from:(if $pf=="" then null else "paths.txt" end),awaits_review:(if $aw=="" then null else $aw end),
              resolve_merge:(if $rm=="" then null else $rm end),commit_before_integrate:($cbi!="")},
        lock:{held:true},trust_file:$trust,
        deferred_gates:($deferred|split(",")|map(select(length>0))),check_pending_release:($pend>0),held:($held>0),held_commits:($held_lines|split("\n")|map(select(length>0))),
        nested_unsettled:($nested|split(" ")|map(select(length>0))),
        commits:$commits,stages:($stages|split("\n")|map(select(length>0)|split(":")|{id:.[0],name:.[1]})),
        interrupted_runs:$interrupted,files:$files}' > "$CPA_RUN/report.json.tmp" 2>>"$LOG" && mv -f "$CPA_RUN/report.json.tmp" "$CPA_RUN/report.json"
  fi
  printf 'cpa: exit=%s run=%s stage=%s reason=%s\n' "$rc" "$RUN_ID" "${FAILSTAGE:-${CUR:-}}" "${FAILREASON:-}" >&2
  exit "$rc"
}
trap finish EXIT
trap 'exit 143' TERM; trap 'exit 130' INT

# ---- S0 preflight ---------------------------------------------------------------------------------------------------------------------------
stage S0 preflight
while [ $# -gt 0 ]; do
  case "$1" in
    --local-only) LOCAL_ONLY=1; shift ;;
    --commit-before-integrate) CBI=1; shift ;;
    --awaits-review) [ $# -ge 2 ] || fail S0 20 usage_error "--awaits-review needs a verdict file"; AWAIT="$2"; shift 2 ;;
    --repo) [ $# -ge 2 ] || fail S0 20 usage_error "--repo needs a path"; REPO="$2"; shift 2 ;;
    --paths-from) [ $# -ge 2 ] || fail S0 20 usage_error "--paths-from needs a file"; PATHS="$2"; shift 2 ;;
    --resolve-merge) [ $# -ge 2 ] || fail S0 20 usage_error "--resolve-merge needs a directory"; RESOLVE="$2"; shift 2 ;;
    --) shift; break ;;
    -*) fail S0 20 usage_error "unknown option $1" ;;
    *) break ;;
  esac
done
[ $# -le 1 ] || fail S0 20 usage_error "unexpected argument after the message: $2"   # never an ignored option
MSG="${1:-}"
if [ -n "$CBI" ]; then
  [ -n "$PATHS" ] || fail S0 20 usage_error "--commit-before-integrate needs --paths-from"
  [ -z "$RESOLVE" ] || fail S0 20 usage_error "--commit-before-integrate together with --resolve-merge"
fi
if [ -n "$PATHS" ]; then
  [ -r "$PATHS" ] || fail S0 20 usage_error "--paths-from file unreadable"
  [ -n "$MSG" ] || fail S0 20 usage_error "a commit message is required with --paths-from"
  cp -- "$PATHS" "$CPA_RUN/paths.txt" || fail S0 20 usage_error "cannot copy the change set"
else : > "$CPA_RUN/paths.txt"; fi
[ -z "$AWAIT" ] || safe_relpath "$AWAIT" || fail S0 20 usage_error "--awaits-review: unsafe verdict path"

OWNED="${CPA_OWNED_ORGS:-}"; [ -n "$OWNED" ] || OWNED="$( [ -r "$D/audit/own_orgs.txt" ] && grep -v '^[[:space:]]*#' "$D/audit/own_orgs.txt" | awk 'NF{print $1}' | paste -sd, - )"
is_owned() { # is_owned <dir>: some remote URL of the repository names an organisation of the own list (scripts/audit/org_of.py, the one shared parser)
  local d="$1" r u org own=0; [ -n "$OWNED" ] || return 1
  for r in $(git -C "$d" remote 2>/dev/null); do
    u="$(git -C "$d" remote get-url "$r" 2>/dev/null)" || continue
    org="$(python3 -I "$D/audit/org_of.py" "$u" 2>/dev/null | head -1 | tr '[:upper:]' '[:lower:]')"; [ -n "$org" ] || continue
    case ",$(printf '%s' "$OWNED" | tr '[:upper:]' '[:lower:]')," in *",$org,"*) own=1 ;; esac
  done; [ "$own" = 1 ]
}
if [ -n "$REPO" ]; then
  safe_declpath "$REPO" || fail S0 20 usage_error "--repo: unsafe path"; REPO="${REPO%/}"
  [ -d "$ROOT/$REPO" ] || fail S0 20 repo_not_found "$REPO"
  [ "$(realpath -- "$(git -C "$ROOT/$REPO" rev-parse --show-toplevel 2>/dev/null)" 2>/dev/null)" = "$(realpath -- "$ROOT/$REPO")" ] || fail S0 20 repo_not_found "$REPO is not the root of its own work tree"
  git -C "$ROOT" submodule status --recursive 2>/dev/null | awk '{print $2}' | grep -qxF -- "$REPO" || fail S0 20 repo_not_found "$REPO is not a submodule of this repository"
  is_owned "$ROOT/$REPO" || fail S0 20 repo_not_owned "$REPO: a --repo run targets an own-organisation repository only"
  TARGET="$ROOT/$REPO"
fi
BR="$(git -C "$TARGET" symbolic-ref --short -q HEAD)" || fail S0 20 wrong_branch "detached HEAD in ${REPO:-the main repository}"
case "$BR" in main|master) ;; *) fail S0 20 wrong_branch "on $BR; a CPA run works on main or master" ;; esac
START_HEAD="$(git -C "$TARGET" rev-parse HEAD 2>/dev/null)"
if [ -n "$RESOLVE" ]; then
  rrd="$(cd "$RESOLVE" 2>/dev/null && pwd -P)" || fail S0 20 resolution_dir_invalid "$RESOLVE"
  case "$rrd" in "$(cd "$ROOT" && pwd -P)/.audit/merge-resolution/"?*) ;; *) fail S0 20 resolution_dir_invalid "$rrd" ;; esac
fi

# the declared change set: paths relative to the committing repository, verdicts relative to the main root
: > "$CPA_RUN/paths.list"; : > "$CPA_RUN/held.tsv"
while IFS= read -r line || [ -n "$line" ]; do
  [ -n "$line" ] || continue; case "$line" in '#'*) continue ;; esac
  p="${line%%$'\t'*}"; v=""; [ "$p" != "$line" ] && v="${line#*$'\t'}"; [ -n "$v" ] || v="$AWAIT"
  safe_declpath "$p" || fail S0 20 unsafe_path "$(printf '%q' "$p")"
  [ -z "$v" ] || safe_relpath "$v" || fail S0 20 unsafe_path "verdict $(printf '%q' "$v")"
  if [ -n "$REPO" ]; then case "$p" in "$REPO"/*) p="${p#"$REPO"/}" ;; *) fail S0 20 path_outside_repo "$p is outside $REPO/" ;; esac; fi
  printf '%s\n' "$p" >> "$CPA_RUN/paths.list"; [ -z "$v" ] || printf '%s\t%s\n' "$p" "$v" >> "$CPA_RUN/held.tsv"
done < "$CPA_RUN/paths.txt"

# a merge in progress: always refused (CPA makes every merge itself); a live holder is named lock_held, an interrupted run merge_in_progress
if [ -e "$(git -C "$TARGET" rev-parse --path-format=absolute --git-path MERGE_HEAD 2>/dev/null)" ]; then
  # a run is named interrupted only when the process its merge.json names is gone (process id and start time against /proc, never a pgrep match)
  holder=""; interrupted=""
  for mj in "$AUDITD"/*/merge.json; do
    [ -f "$mj" ] || continue; od="$(dirname "$mj")"; [ -f "$od/report.json" ] && continue; [ "$od" = "$CPA_RUN" ] && continue
    mp="$(jq -r '.pid // empty' "$mj" 2>/dev/null)"; mst="$(jq -r '.pid_start // empty' "$mj" 2>/dev/null)"
    if [ -n "$mp" ] && [ -r "/proc/$mp/stat" ] && [ "$(awk '{print $22}' "/proc/$mp/stat" 2>/dev/null)" = "$mst" ]; then holder="$(basename "$od") pid $mp"; else interrupted="${interrupted:+$interrupted }$(basename "$od")"; fi
  done
  [ -z "$holder" ] || fail S0 20 lock_held "a live run is mid-merge: $holder; no remediation, let it finish"
  fail S0 20 merge_in_progress "MERGE_HEAD present${interrupted:+, left by the interrupted run $interrupted}: git merge --abort, then compare the uncommitted files with the sha256 list of that run's backup/worktree/"
fi

# remotes: every remote unreachable gives 20 no_remote_reachable (nothing to judge S5 and S6 against), --local-only included
RL="$(git -C "$TARGET" remote 2>/dev/null)"
if [ -n "$RL" ]; then
  reach=0; for r in $RL; do safe_remote "$r" || continue; timeout 60 git -C "$TARGET" ls-remote -- "$r" >/dev/null 2>&1 && reach=1; done
  [ "$reach" = 1 ] || fail S0 20 no_remote_reachable "none of the remotes of ${REPO:-the main repository} could be read"
fi

"$LO/acquire.sh" --purpose commit_push --run-id "$RUN_ID" --pid "$$" >"$CPA_RUN/lock.out" 2>&1; lrc=$?
case "$lrc" in
  0) LOCKED=1 ;;
  3|4|5|6) h="$("$LO/holder.sh" commit_push 2>/dev/null)"; fail S0 20 lock_held "holder run $(jq -r '.run_id // "?"' <<< "$h" 2>/dev/null) pid $(jq -r '.pid // "?"' <<< "$h" 2>/dev/null); this run wrote only $CPA_RUN" ;;
  *) fail S0 20 lock_error "acquire.sh exit $lrc: $(head -c 200 "$CPA_RUN/lock.out")" ;;
esac
export DISK_HEADROOM_OUT_DIR="$CPA_RUN/disk/"
"$LO/check_no_build_writing_tracked.sh" >"$CPA_RUN/registry.out" 2>&1; rrc=$?
case "$rrc" in 0) ;; 1) fail S0 20 evidence_writer_active "$(head -c 300 "$CPA_RUN/registry.out" | tr '\n\t' '  ')" ;; *) fail S0 20 registry_check_error "rc=$rrc" ;; esac
[ -z "${SKIP_LONG:-}" ] || defer SKIP_LONG "$SKIP_LONG" S0     # recorded at S0, so an S1 merge commit carries it too
if [ -x "$D/anti-mess/sweep.sh" ]; then
  "$D/anti-mess/sweep.sh" --stage S0 --paths-from "$CPA_RUN/paths.txt" ${REPO:+--repo "$REPO"} >"$CPA_RUN/sweep-s0.out" 2>&1; src=$?
  case "$src" in 0) ;; 10) fail S0 20 sweep_finding "an undeclared change or another catalogued drift (see sweep-s0.out)" ;; *) fail S0 20 sweep_error "rc=$src" ;; esac
else defer SWEEP_ABSENT "sweep not yet in the approved manifest (tasks.md T090, T093)" S0; fi
[ -z "$LOCAL_ONLY" ] || defer LOCAL_ONLY "push owed: --local-only" S0

# the approved manifest as {"path":"sha256"} for the integration helpers' G-GATE routing
jq -c '[.entries[]|{(.path):.sha256}]|add // {}' "$SNAP" > "$CPA_RUN/approved.json" 2>/dev/null || echo '{}' > "$CPA_RUN/approved.json"

# ---- S1 / S1a / S1b ---------------------------------------------------------------------------------------------------------------------------
s1_integrate() { # s1_integrate <stage id> <with change set: 1|0>
  local st="$1" cs="$2" rc reason ffreason ff=() mf=() from to c
  ff=(--main-only --report-behind-submodules --root "$TARGET" --branch "$BR" --run-dir "$CPA_RUN" --json "$CPA_RUN/integrate-$st.json" --approved "$CPA_RUN/approved.json")
  [ "$cs" = 1 ] && ff+=(--changeset-from "$CPA_RUN/paths.list")
  [ -z "$OWNED" ] || ff+=(--owned-orgs "$OWNED"); [ -z "$REPO" ] && [ -n "$ADOPT" ] && ff+=(--adoption-commit "$ADOPT")
  "$RD/integrate_ff_only.sh" "${ff[@]}" >"$CPA_RUN/integrate-$st.out" 2>&1; rc=$?
  reason="$(jq -r '.reason // ""' "$CPA_RUN/integrate-$st.json" 2>/dev/null)"
  case "$rc" in
    0) from="$(jq -r '.moved.from // empty' "$CPA_RUN/integrate-$st.json" 2>/dev/null)"; to="$(jq -r '.moved.to // empty' "$CPA_RUN/integrate-$st.json" 2>/dev/null)"
       if [ -n "$from" ] && [ -n "$to" ]; then
         if [ -z "$REPO" ] && [ -n "$ADOPT" ]; then
           for c in $(git -C "$TARGET" rev-list "$to" --not "$from" "$ADOPT" 2>/dev/null); do
             id="$(cpa_runid "$TARGET" "$c")" && cpa_row "$AUDITD/$id/commits.tsv" "$c" && continue
             printf '%s\n' "$c" >> "$CPA_RUN/foreign.txt"
           done
         fi
         # a fast-forward that moved recorded gitlinks: settled by scripts/repo/record_pending_pin.sh (T435a) when it is in the approved copy, else
         # the changed paths are unsettled and S7 gives 15 for exactly them (reason nested_settle_unavailable)
         if [ "$(jq '.moved.gitlinks|length' "$CPA_RUN/integrate-$st.json" 2>/dev/null)" -gt 0 ] 2>/dev/null; then
           if [ -x "$RD/record_pending_pin.sh" ]; then
             mkdir -p "$CPA_RUN/settle"; "$RD/record_pending_pin.sh" --settle-nested "$ROOT" "$from" "$to" --in-hold "$RUN_ID" --out "$CPA_RUN/settle/" >>"$LOG" 2>&1 \
               || while IFS= read -r gp; do NESTED+=("$gp:settle_failed"); done < <(jq -r '.moved.gitlinks[].path' "$CPA_RUN/integrate-$st.json")
           else while IFS= read -r gp; do NESTED+=("$gp:nested_settle_unavailable"); done < <(jq -r '.moved.gitlinks[].path' "$CPA_RUN/integrate-$st.json"); fi
         fi
       fi; return 0 ;;
    11) fail "$st" 11 "${reason:-blocked}" "no remote could be read or a local-only commit cannot be cleared (see integrate-$st.json)" ;;
    12) case "$reason" in
          diverged|remotes_diverged|merge_path_required) ;;
          *) fail "$st" 12 "${reason:-blocked}" "see integrate-$st.json; a re-run of the window with --commit-before-integrate or CPA --repo <path> clears ff_blocked_by_local_changes / submodule_local_commits" ;;
        esac ;;
    20) fail "$st" 20 "${reason:-refused}" "see integrate-$st.json" ;;
    *) fail "$st" 20 internal_error "integrate_ff_only.sh exit $rc" ;;
  esac
  ffreason="$reason"
  mf=(--root "$TARGET" --branch "$BR" --run-dir "$CPA_RUN" --audit-dir "$AUDITD" --repo-key "${REPO:-.}" --approved "$CPA_RUN/approved.json" --run-pid "$$")
  [ -z "$REPO" ] && [ -n "$ADOPT" ] && mf+=(--adoption-commit "$ADOPT" --anchor "$ADOPT")
  [ -z "$RESOLVE" ] || mf+=(--resolution "$RESOLVE")
  "$RD/integrate_merge.sh" "${mf[@]}" >"$CPA_RUN/integrate-merge-$st.out" 2>&1; rc=$?
  case "$rc" in
    0) # integrate_ff_only.sh sent this repository to the merge path, but integrate_merge.sh only merges tips that fast-forward cannot take (a tip that
       # descends from HEAD is skipped as ff-able): "nothing to merge" here is NOT an integration. Continuing would read an unresolved window as clean
       # (11.4.201(4): conservative-safe), so the run stops with 12 naming the ff_only reason; owed request to the T040 helpers (docs/scripts/commit-push-all.md).
       if grep -q nothing_to_merge "$CPA_RUN/integrate-merge-$st.out" 2>/dev/null; then
         fail "$st" 12 "${ffreason:-merge_path_required}" "integrate_ff_only.sh could not fast-forward and integrate_merge.sh found nothing to merge (a behind repository with diverged or gated remote tips); nothing moved"
       fi ;;
    10) fail "$st" 10 resolved_file_holds_marker "see integrate-merge-$st.out" ;;
    11) fail "$st" 11 remote_unreachable "see integrate-merge-$st.out" ;;
    12) reason="$(grep -oE 'merge_conflict|remotes_diverged|ff_blocked_by_local_changes|merge_target_moved' "$CPA_RUN/integrate-merge-$st.out" | head -1)"
        fail "$st" 12 "${reason:-merge_blocked}" "the conflicting files with their markers are under $CPA_RUN/conflicts/ (the run's commits stay local, pushed nowhere); resolve through --resolve-merge" ;;
    13) fail "$st" 13 secret_fold_refused "see integrate-merge-$st.out" ;;
    20) reason="$(grep -oE 'resolution_dir_invalid|merge_resolver_not_pinned|store_not_rerecorded|secret_fold_unavailable|backup_failed|merge_in_progress|unrecorded_local_commit|resolution_invalid|wrong_branch' "$CPA_RUN/integrate-merge-$st.out" | head -1)"
        fail "$st" 20 "${reason:-merge_refused}" "$(tail -c 200 "$CPA_RUN/integrate-merge-$st.out" | tr '\n' ' ')" ;;
    *) fail "$st" 20 internal_error "integrate_merge.sh exit $rc" ;;
  esac
}
s1a_backup() { # objects only, the unrecorded-commit guard, and the 9.2 backup (work-tree copy and, for a non-empty local range, a verified bundle)
  local bk="$CPA_RUN/backup/s1a" r t ts=() c f id
  mkdir -p "$bk/worktree" || fail S1a 20 backup_failed "cannot create $bk"
  for r in $RL; do safe_remote "$r" || continue
    git -C "$TARGET" fetch -q --no-tags --no-write-fetch-head --refmap= --no-recurse-submodules -- "$r" "$BR" >>"$LOG" 2>&1 || echo "fetch_failed:$r" >> "$CPA_RUN/s1a-fetch.txt"
    t="$(timeout 60 git -C "$TARGET" ls-remote -- "$r" "refs/heads/$BR" 2>/dev/null | lr_exact "$BR")"
    if [ -n "$t" ] && safe_sha "$t" && git -C "$TARGET" cat-file -e "$t^{commit}" 2>/dev/null; then ts+=("$t"); fi
  done
  for c in $(git -C "$TARGET" rev-list "$BR" ${ts[@]:+--not "${ts[@]}"} 2>/dev/null); do
    id="$(cpa_runid "$TARGET" "$c")" && cpa_row "$AUDITD/$id/commits.tsv" "$c" && continue
    fail S1a 20 unrecorded_local_commit "$c: a commit that is not a CPA commit and that no remote holds; nothing moved or committed"
  done
  while IFS= read -r -d '' f; do
    f="${f:3}"; [ -e "$TARGET/$f" ] || continue
    # shellcheck disable=SC2015  # A && B || fail: a failure of either is the refusal
    mkdir -p "$bk/worktree/$(dirname "$f")" && cp -p -- "$TARGET/$f" "$bk/worktree/$f" || fail S1a 20 backup_failed "work-tree copy of $(printf '%q' "$f")"
    ( cd "$bk/worktree" && sha256sum -- "$f" ) >> "$bk/worktree.sha256" || fail S1a 20 backup_failed "sha256 of $(printf '%q' "$f")"
  done < <(git -C "$TARGET" status --porcelain=v1 -z -uall --ignore-submodules=all 2>/dev/null)
  [ -f "$bk/worktree.sha256" ] || : > "$bk/worktree.sha256"
  if [ "$(git -C "$TARGET" rev-list "$BR" ${ts[@]:+--not "${ts[@]}"} 2>/dev/null | wc -l)" -gt 0 ]; then
    # shellcheck disable=SC2015  # create && verify || fail: a failure of either is the refusal
    git -C "$TARGET" bundle create "$bk/local.bundle" "$BR" ${ts[@]:+--not "${ts[@]}"} >>"$LOG" 2>&1 && git -C "$TARGET" bundle verify "$bk/local.bundle" >>"$LOG" 2>&1 \
      || fail S1a 20 backup_failed "git bundle create or git bundle verify failed"
    jq -n --arg h "$START_HEAD" --arg b "$bk/local.bundle" '{head:$h,bundle:$b,reason:null,worktree_sha256:"worktree.sha256"}' > "$bk/backup.json"
  else
    jq -n --arg h "$START_HEAD" '{head:$h,bundle:null,reason:"local_range_empty",worktree_sha256:"worktree.sha256"}' > "$bk/backup.json"
  fi
}
if [ -n "$CBI" ]; then stage S1a fetch_objects; s1a_backup
else stage S1 fetch_integrate; s1_integrate S1 1; fi

# ---- S2 scope check ----------------------------------------------------------------------------------------------------------------------------
stage S2 scope_check
# a declared path below a gitlink (held or not) is refused before any commit: it would commit inside the submodule and leave the parent stale
while IFS= read -r -d '' gl; do
  while IFS= read -r p; do
    case "$p" in "$gl"/*) fail S2 20 path_in_submodule "$p lies below the gitlink $gl: commit it with CPA --repo $gl, then move the pin through G-PIN" ;; esac
  done < "$CPA_RUN/paths.list"
done < <(gitlinks "$TARGET")
"$RD/scope_check.sh" --root "$TARGET" --paths-from "$CPA_RUN/paths.list" >"$CPA_RUN/scope.out" 2>&1; rc=$?
case "$rc" in 0) ;; 13) fail S2 13 scope_refused "$(head -c 300 "$CPA_RUN/scope.out" | tr '\n' ' ')" ;; 20) fail S2 20 scope_error "$(head -c 300 "$CPA_RUN/scope.out" | tr '\n' ' ')" ;; *) fail S2 20 internal_error "scope_check.sh exit $rc" ;; esac

# ---- S3 cheap checks ---------------------------------------------------------------------------------------------------------------------------
stage S3 validate_cheap
vc=(--root "$TARGET" --files-from "$CPA_RUN/paths.list" --code-root "$D/.." --trusted-tables "$RD" --out "$CPA_RUN/validate")
[ -f "$ROOT/scripts/repo/validate_checks.tsv" ] && vc+=(--registry "$ROOT/scripts/repo/validate_checks.tsv" --approved-registry "$RD/validate_checks.tsv")
[ -s "$CPA_RUN/held.tsv" ] && vc+=(--held-from "$CPA_RUN/held.tsv")
mkdir -p "$CPA_RUN/validate"
"$RD/validate_cheap.sh" "${vc[@]}" >"$CPA_RUN/validate.out" 2>&1; rc=$?
case "$rc" in
  0) ;;
  10) fail S3 10 check_failed "$(grep -E '^(fail|class_table_unreviewed|legacy_row_not_dropped)' "$CPA_RUN/validate.out" | head -3 | tr '\n\t' '  ')" ;;
  14) PENDING_CHECKS=1 ;;     # check_pending_release rows: a deferral of class 14 (the CHECK_PENDING_RELEASE trailer needs a closed-set change of record_deferral.sh: owed)
  20) fail S3 20 check_error "$(head -c 300 "$CPA_RUN/validate.out" | tr '\n' ' ')" ;;
  *) fail S3 20 internal_error "validate_cheap.sh exit $rc" ;;
esac

# ---- S4 long gates -----------------------------------------------------------------------------------------------------------------------------
stage S4 validate_long
if [ -n "${SKIP_LONG:-}" ]; then log "S4 skipped: SKIP_LONG recorded at S0"
elif [ -s "$RD/long_gates.txt" ]; then
  lf=(); while IFS= read -r g; do [ -n "$g" ] && lf+=(--file "$ROOT/$g"); done < "$RD/long_gates.txt"
  "$LO/require_verdicts.sh" "${lf[@]}" >"$CPA_RUN/long.out" 2>&1; rc=$?
  case "$rc" in 0) ;; 1) fail S4 10 long_gate_verdict_missing "$(head -c 300 "$CPA_RUN/long.out" | tr '\n' ' ')" ;; *) fail S4 20 internal_error "require_verdicts.sh exit $rc" ;; esac
else log "S4: no long gates configured in the approved copy"; fi

# ---- S5 commit ---------------------------------------------------------------------------------------------------------------------------------
stage S5 commit
commit_group() { # commit_group <paths file> [held.tsv]
  local pf="$1" hf="${2:-}" cr=(--repo "$TARGET" --run-dir "$CPA_RUN" --run-id "$RUN_ID" --message "$MSG" --repo-key "${REPO:-.}") rc
  [ -s "$CPA_RUN/foreign.txt" ] && cr+=(--foreign-from "$CPA_RUN/foreign.txt")
  [ -z "$hf" ] || cr+=(--held-from "$hf")
  mapfile -t _ps < "$pf"
  "$RD/commit_recursive.sh" "${cr[@]}" -- "${_ps[@]}" >>"$CPA_RUN/commit.out" 2>&1; rc=$?
  case "$rc" in 0) ;; *) fail S5 20 commit_failed "commit_recursive.sh exit $rc: $(tail -c 300 "$CPA_RUN/commit.out" | tr '\n' ' ')" ;; esac
}
if [ -s "$CPA_RUN/paths.list" ]; then
  # a hold on a verdict that already holds GO in the main HEAD, or a change to such a verdict file, is refused before any commit (verdict_already_go)
  while IFS=$'\t' read -r hp hv; do
    [ -n "$hv" ] || continue
    head_json_is '.verdict=="GO"' "$hv" && fail S5 20 verdict_already_go "$hp is held on $hv, which holds GO in the main HEAD: a GO file is final; hold on a new review iteration"
    if json_is '.verdict=="GO"' "$ROOT/$hv" && grep -qxF -- "$hv" "$CPA_RUN/paths.list"; then
      fail S5 20 verdict_already_go "$hv is declared with GO in the same change set as $hp, which is held on it"; fi
  done < "$CPA_RUN/held.tsv"
  while IFS= read -r dp; do
    [ -z "$REPO" ] || continue
    head_json_is '.schema=="review-verdict/1" and .verdict=="GO"' "$dp" \
      && fail S5 20 verdict_already_go "$dp holds GO in the main HEAD and is changed by this change set"
  done < "$CPA_RUN/paths.list"
  # unheld paths first, then one commit per verdict
  cut -f1 "$CPA_RUN/held.tsv" > "$CPA_RUN/paths.held"
  grep -vxFf "$CPA_RUN/paths.held" "$CPA_RUN/paths.list" > "$CPA_RUN/paths.unheld" || true     # an empty pattern file matches nothing: every path is unheld
  [ ! -s "$CPA_RUN/paths.unheld" ] || commit_group "$CPA_RUN/paths.unheld"
  if [ -s "$CPA_RUN/paths.held" ]; then
    "$RD/record_deferral.sh" --run-dir "$CPA_RUN" --flag LOCAL_ONLY --reason "held on review: push owed until the verdict is GO" --awaits-review "$(head -1 "$CPA_RUN/held.tsv" | cut -f2)" >>"$LOG" 2>&1 \
      || fail S5 20 deferral_write_failed LOCAL_ONLY
    DEFERRED+=(LOCAL_ONLY); commit_group "$CPA_RUN/paths.held" "$CPA_RUN/held.tsv"
  fi
fi
if [ -n "$CBI" ]; then stage S1b integrate; s1_integrate S1b 0; fi

# the pending pin move of a --repo run: one row per repository, replaced whenever the run moved that repository's HEAD
if [ -n "$REPO" ]; then
  nh="$(git -C "$TARGET" rev-parse HEAD)"
  if [ "$nh" != "$START_HEAD" ]; then
    pp="$ROOT/.audit/pending_pins.tsv"; tmp="$ROOT/.audit/pending_pins.tsv.$RUN_ID"
    # shellcheck disable=SC2015  # the group and the rename are one write; a failure of either is the refusal
    { [ -f "$pp" ] && awk -F'\t' -v p="$REPO" '$1!=p' "$pp"; printf '%s\t%s\t%s\n' "$REPO" "$nh" "$RUN_ID"; } > "$tmp" && mv -f "$tmp" "$pp" || fail S5 20 pending_pin_write_failed "$pp"
  fi
fi

# ---- S6 push -----------------------------------------------------------------------------------------------------------------------------------
sub_outgoing() { # sub_outgoing <dir>: true when the run branch holds a commit that a live remote tip (held locally) lacks, or a remote tip cannot be judged
  local d="$1" r t held=() out
  for r in $(git -C "$d" remote 2>/dev/null); do safe_remote "$r" || return 0
    t="$(timeout 60 git -C "$d" ls-remote -- "$r" "refs/heads/$BR" 2>/dev/null | lr_exact "$BR")" || return 0
    [ -n "$t" ] || { held=(); break; }
    if safe_sha "$t" && git -C "$d" cat-file -e "$t^{commit}" 2>/dev/null; then held+=("$t"); else return 0; fi
  done
  out="$(git -C "$d" rev-list "$BR" ${held[@]:+--not "${held[@]}"} 2>/dev/null | head -1)"; [ -n "$out" ]
}
stage S6 push
if [ -z "$LOCAL_ONLY" ]; then
  pr=(--main-root "$ROOT" --branch "$BR" --run-dir "$CPA_RUN")
  if [ -n "$REPO" ]; then pr+=(--repo "$REPO")
  else
    pr+=(--repo .)
    # owned submodules that carry the run branch AND commits that some remote lacks, deepest first: a submodule without that local branch is left out,
    # and so is one that is merely behind its remote (needs_update, never pushed and never moved by a run of the main repository)
    while IFS= read -r sp; do
      [ -n "$sp" ] || continue; is_owned "$ROOT/$sp" || continue
      git -C "$ROOT/$sp" rev-parse -q --verify "refs/heads/$BR" >/dev/null 2>&1 || { log "S6: $sp has no local branch $BR, not pushed"; continue; }
      sub_outgoing "$ROOT/$sp" || { log "S6: $sp holds no commit that a remote lacks, no push call"; continue; }
      pr+=(--repo "$sp")
    done < <(git -C "$ROOT" submodule status --recursive 2>/dev/null | awk '$1 !~ /^-/ {print $2}' | awk -F/ '{print NF "\t" $0}' | sort -rn | cut -f2-)
  fi
  "$RD/push_recursive.sh" "${pr[@]}" >"$CPA_RUN/push.out" 2>"$CPA_RUN/push.err"; rc=$?
  mapfile -t HELDLINES < <(awk -F'\t' '$1=="HELD"{print $3 " " $4 " " $5}' "$CPA_RUN/push.out")
  grep -E '^(HELD|NOPUSH|PUSH_FAILED|REFUSED)' "$CPA_RUN/push.out" | tr '\t' ' ' >&2
  case "$rc" in
    0) ;;
    14) HELD=1 ;;
    11) fail S6 11 "$(grep -oE 'remote_moved_since_s1|remote_unreachable|submodule_commit_not_held|PUSH_FAILED' "$CPA_RUN/push.out" | head -1)" "commits are safe locally; see push.out" ;;
    20) fail S6 20 "$(grep -oE 'unrecorded_local_commit|git_listing_failed|submodule_git_unreadable' "$CPA_RUN/push.out" "$CPA_RUN/push.err" | head -1 | sed 's/.*://')" "see push.out" ;;
    *) fail S6 20 internal_error "push_recursive.sh exit $rc" ;;
  esac
  ! grep -qE '^HELD' "$CPA_RUN/push.out" || HELD=1
else log "S6 skipped: --local-only"; fi

# ---- S7 verify ---------------------------------------------------------------------------------------------------------------------------------
stage S7 verify_clean
VR="$ROOT"; EXC="$RD/exceptions.tsv"
if [ -n "$REPO" ]; then
  VR="$ROOT/$REPO"; : > "$CPA_RUN/exceptions.tsv"
  [ -f "$EXC" ] && awk -F'\t' -v OFS='\t' -v p="$REPO/" 'index($1,p)==1 {$1=substr($1,length(p)+1); print}' "$EXC" > "$CPA_RUN/exceptions.tsv"
  EXC="$CPA_RUN/exceptions.tsv"
fi
[ -f "$EXC" ] || EXC=/dev/null
vf=(--root "$VR" --json "$CPA_RUN/verify.json" --exceptions "$EXC" --quiet --jobs 2 --timeout 60); [ -z "$OWNED" ] || vf+=(--owned-orgs "$OWNED")
if [ -n "$LOCAL_ONLY" ]; then vf+=(--no-remote); else vf+=(--fetch); fi
"$RD/verify_repos.sh" "${vf[@]}" >"$CPA_RUN/verify.out" 2>&1; vrc=$?
json_is '.summary | (.pin_drift|numbers) and (.ahead|numbers) and (.dirty|numbers)' "$CPA_RUN/verify.json" \
  || fail S7 20 verifier_report_unreadable "verifier exit $vrc; a missing or unreadable report is never read as clean"
# pointer drift: a row that matches .audit/pending_pins.tsv exactly (same path, head equal to the row's sha, drifted) is reported pending, never failing
PP="$ROOT/.audit/pending_pins.tsv"; [ -f "$PP" ] || PP=/dev/null
jq -r '.repos[]|select(.pin_state!="ok")|[.path,(.head//""),.pin_state]|@tsv' "$CPA_RUN/verify.json" 2>/dev/null > "$CPA_RUN/drift.tsv"
: > "$CPA_RUN/pins.tsv"; : > "$CPA_RUN/pins.unmatched"
while IFS=$'\t' read -r dpath dhead dstate; do
  [ -n "$dpath" ] || continue
  if [ "$dstate" = drifted ] && awk -F'\t' -v p="$dpath" -v h="$dhead" '$1==p && $2==h {f=1} END{exit !f}' "$PP"; then printf '%s\t%s\tpending\n' "$dpath" "$dhead" >> "$CPA_RUN/pins.tsv"
  else printf '%s\t%s\t%s\n' "$dpath" "$dhead" "$dstate" >> "$CPA_RUN/pins.unmatched"; fi
done < "$CPA_RUN/drift.tsv"
jq -n --rawfile p "$CPA_RUN/pins.tsv" --rawfile u "$CPA_RUN/pins.unmatched" '{pending:($p|split("\n")|map(select(length>0)|split("\t")|{path:.[0],head:.[1]})),unmatched:($u|split("\n")|map(select(length>0)|split("\t")|{path:.[0],head:.[1],state:.[2]}))}' > "$CPA_RUN/pins.json" 2>/dev/null
case "$vrc" in
  0) ;;
  11) if [ "$HELD" = 1 ]; then log "S7: verifier 11 explained by the held remainder (review_pending)"; else fail S7 11 commits_not_on_a_remote "after S6; see verify.json"; fi ;;
  14) fail S7 11 remote_unproven "a remote could not be proven after the push (verifier 14 is never a CPA 14)" ;;
  15) ;;
  12|13) fail S7 "$vrc" verification_not_clean "verifier exit $vrc; classes in verify.json" ;;
  20) fail S7 20 verifier_blind "verifier exit 20" ;;
  *) fail S7 20 internal_error "unexpected verifier exit $vrc" ;;
esac
if [ -s "$CPA_RUN/pins.unmatched" ]; then fail S7 15 pointer_drift "no pending pin move matches: $(cut -f1,3 "$CPA_RUN/pins.unmatched" | tr '\t\n' ': ')"; fi
if [ "$vrc" = 15 ] && [ ! -s "$CPA_RUN/pins.tsv" ]; then fail S7 15 pointer_drift "verifier 15"; fi
if [ "${#NESTED[@]}" -gt 0 ]; then fail S7 15 nested_unsettled "${NESTED[*]} (remediation per path: git submodule update --init -- <path> for a third-party path, docs/11 section 6 through ST-SUB for an owned one)"; fi
if [ -x "$D/anti-mess/sweep.sh" ]; then
  "$D/anti-mess/sweep.sh" --stage S7 ${REPO:+--repo "$REPO"} >"$CPA_RUN/sweep-s7.out" 2>&1; src=$?
  case "$src" in 0) ;; *) fail S7 20 sweep_finding "after verify (rc=$src)" ;; esac
fi

# ---- S8 happens in the EXIT trap: the report is written on every path --------------------------------------------------------------------------------
stage S8 report
if [ "${#DEFERRED[@]}" -gt 0 ] || [ "$HELD" = 1 ] || [ "$PENDING_CHECKS" = 1 ]; then
  if [ "$HELD" = 1 ]; then FAILREASON="review_pending"; FAILDETAIL="${HELDLINES[*]:-}"; else FAILREASON="deferral_recorded"; FAILDETAIL="${DEFERRED[*]:-}${PENDING_CHECKS:+ check_pending_release}"; fi
  exit 14
fi
exit 0
