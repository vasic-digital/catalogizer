#!/bin/bash -p
# commit-push-all.sh (CPA) - the dedicated commit and push script: hook validations as explicit stages, the commit/push mechanism always unblocked
# (constitution 11.4.234; docs/16 section 12; tasks.md T042, T042a). It never forces: no --force, no --force-with-lease, no +refspec, no --no-verify,
# no rebase, no reset, no history rewrite (11.4.113); every merge is a plain `git merge --no-ff` made by integrate_merge.sh.
#
# Start     only through the host entry point: scripts/repo/host_entry/cpa-host copies the owner-approved files into
#           .audit/commit-push/<run id>/released/ and executes this script from there (CPA_ROOT, CPA_RUN_ID, CPA_ADOPTION_COMMIT in the environment).
#           Started any other way the self-test below refuses with 20 not_via_host_entry (it writes a minimal report only into a run directory whose run.pid names this process).
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
#           lock records under .audit/longops/; the tracked tree is never written. report.json is written on every exit path that a handler can reach (S8); the exceptions are listed under Exits.
# Exits     0 clean: every stage ran and NOTHING is owed; 10 a check failed; 11 push rejected/remote unreachable/remote moved; 12 integration blocked
#           (diverged remotes, conflict, local changes); 13 scope refused or verification dirty; 14 a recorded deferral, held push or a gate that did not
#           run (never a clean pass: SKIP_LONG, SWEEP_ABSENT, LOCAL_ONLY, CHECK_PENDING_RELEASE, CHECKS_DEFERRED = a `deferred` registry row,
#           GATES_NOT_BUILT = a row of scripts/repo/owed_gates.tsv or an absent long-gate list, CHECKS_NOT_JUDGED = a declared path no S3 check judged; each listed in
#           report.json `deferred_gates`; the flags recorded at S0 (SKIP_LONG, SWEEP_ABSENT, LOCAL_ONLY, GATES_NOT_BUILT) are also in the Deferred-Gates line of EVERY commit
#           of the run, an S1 merge commit included; the flags found at S3 (CHECKS_DEFERRED, CHECK_PENDING_RELEASE, CHECKS_NOT_JUDGED) are in the commits made at S5, and
#           in report.json and deferrals.tsv, but not in an S1 merge commit that was made before S3 ran);
#           15 pointer drift without a pending move; 20 refusal or internal error (any helper exit outside its documented set, a `set -u` error, and ANY catchable terminating
#           signal: each ends the run with the report written and the status 20, the signal's own status in the detail; a report that could not be written, a lock that could not be
#           released and a helper that outlived the bounded wait are 20 as well). The few intervals no handler can cover (SIGKILL, SIGSTOP, signals 32 and 33, the milliseconds between the
#           exec and the first statement) leave a run directory without a report: the NEXT run proves its process gone from run.pid, closes it (report.json with closed_by, interrupted.json)
#           and lists it in interrupted_runs; nothing is ever deleted.
# Environment  a run is steered by the owner's approvals only, through an ALLOWLIST: `#!/bin/bash -p` (imported functions, SHELLOPTS, BASHOPTS, BASH_ENV, ENV ignored by bash itself, an
#           absolute interpreter path), the signal block, then the allowlist block (the same text in cpa-host): any exported name outside HOME PATH TMPDIR CPA_HOST_STATE SKIP_LONG SSH_AUTH_SOCK
#           the commit identity LONGOPS_ALLOW_TMPFS LC_ALL PWD SHLVL CPA_ROOT CPA_RUN_ID CPA_ADOPTION_COMMIT re-execs the script once through `env -i`; HOME, CPA_HOST_STATE and TMPDIR must be
#           absolute and every PATH element absolute (20 env_value_unsafe); python runs `-I`. git settings come from the owner's HOME (~/.gitconfig) and the repository's own config, which an
#           S0 gate holds to the approved key table (repo_config_allow.tsv). HOME, PATH, TMPDIR, CPA_HOST_STATE, SKIP_LONG and SSH_AUTH_SOCK are the owner's own process environment.
# Honest boundary (UNCONFIRMED / owed, see docs/scripts/commit-push-all.md): the gates of scripts/repo/owed_gates.tsv (secret and private-key folds, path
#           gates, G-PIN accepted pins, verdict provenance, ratchet baselines, remote checks and exit 16, container S3) and --resume are NOT built here;
#           a run that reaches their stage says so with exit 14 and GATES_NOT_BUILT, it never reads as exit 0.
set -u
# --- cpa signals begin ---
# Every catchable signal that terminates by default ends the process through its EXIT trap with the status 128+n (WF14 N2). This block is the FIRST statement after `set -u`
# (WF17-cpa SIG-1/SIG-2: no interval of the start-up is left without a handler): ONE `kill -l` table read, no fork per number. Each handler FIRST ignores every one of the numbers (a second
# signal can never cut the exit path short, SIG-3), then exits 128+n. The signals that do not terminate (CHLD, CONT, STOP, TSTP, TTIN, TTOU, URG, WINCH) are left alone; KILL and STOP cannot be
# caught and glibc reserves 32 and 33 (bash cannot trap them: status 160/161, no report; the next run closes that run directory).
CPA_SIGS=(); while read -r n nm; do case "$nm" in KILL|STOP|CHLD|CONT|TSTP|TTIN|TTOU|URG|WINCH) continue ;; esac; CPA_SIGS+=("$n"); done \
  < <(n=0; for x in $(kill -l); do case "$x" in *')') n="${x%)}" ;; SIG*) echo "$n ${x#SIG}" ;; esac; done)
for n in "${CPA_SIGS[@]}"; do trap "trap '' ${CPA_SIGS[*]}; exit $((128+n))" "$n" 2>/dev/null; done
cpa_signals_ignore() { trap '' "${CPA_SIGS[@]}" 2>/dev/null; }
# --- cpa signals end ---
# --- cpa env scrub begin ---
# What a run executes and where it pushes is decided by the owner's approvals, never by the caller's environment (WF11 F1/F11, WF14 N1/N3, WF17-cpa ENV). An ALLOWLIST replaces the denylist of
# names: the denylist ran as a script line, after bash had already imported functions, SHELLOPTS and BASHOPTS, and it did not name TMOUT, PYTHONPATH or LD_PRELOAD. PWD and SHLVL are exported by
# bash itself even under `env -i`, so they are listed (bash recomputes them; they carry no caller choice). Any other exported name: the script re-execs itself ONCE through `env -i` with exactly
# the allowed names; the internal first argument marks the re-executed process, and a caller who passes it with a dirty environment is REFUSED, never passed. This text is the same in
# scripts/commit-push-all.sh (matrix section 11).
CPA_ENV_ALLOW=" HOME PATH TMPDIR CPA_HOST_STATE SKIP_LONG SSH_AUTH_SOCK GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL LONGOPS_ALLOW_TMPFS LC_ALL PWD SHLVL CPA_ROOT CPA_RUN_ID CPA_ADOPTION_COMMIT "
_clean=1; _bad=""; for _v in $(compgen -e); do case "$CPA_ENV_ALLOW" in *" $_v "*) ;; *) _clean=0; _bad="$_v" ;; esac; done
if [ "$_clean" = 0 ]; then
  [ "${1:-}" != --cpa-env-cleaned ] || { printf '{"refusal":"env_not_clean","reason":"%s"}\n' "$_bad" >&2; exit 20; }
  _e=(); for _v in $CPA_ENV_ALLOW; do case "$_v" in PWD|SHLVL) continue ;; esac; [ -n "${!_v+x}" ] && _e+=("$_v=${!_v}"); done
  exec /usr/bin/env -i "${_e[@]}" LC_ALL=C /bin/bash -p -- "$0" --cpa-env-cleaned "$@"
fi
[ "${1:-}" != --cpa-env-cleaned ] || shift
export LC_ALL=C
unset _clean _bad _v _e
# --- cpa env scrub end ---
# until the run directory is reported by finish below, a status outside the documented set (a `set -u` error, a signal) is a refusal 20, never the shell's own status; the run directory this
# process owns (run.pid names it) gets a minimal report so that no later run is blocked by a reportless one (WF17-cpa RES-1)
core_report() { # core_report <failed stage> <reason> <detail>
  local rd rp rs; [ -n "${CPA_ROOT:-}" ] && [[ "${CPA_RUN_ID:-}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || return 0
  rd="$CPA_ROOT/.audit/commit-push/$CPA_RUN_ID"; [ -d "$rd" ] && [ ! -e "$rd/report.json" ] || return 0
  { read -r rp rs < "$rd/run.pid"; } 2>/dev/null; [ "${rp:-}" = "$$" ] || return 0
  jq -nc --arg id "$CPA_RUN_ID" --arg dir ".audit/commit-push/$CPA_RUN_ID/" --arg fs "$1" --arg r "$2" --arg d "$3" \
    '{schema:1,run_id:$id,run_dir:$dir,exit:20,failed_stage:$fs,reason:$r,detail:$d,mode:{},lock:{held:false,released:false},deferred_gates:[],check_pending_release:false,held:false,held_commits:[],nested_unsettled:[],unjudged_paths:[],commits:[],stages:[{id:"S0",name:"preflight"}],interrupted_runs:[],live_runs:[],files:{}}' \
    > "$rd/report.json.tmp" 2>/dev/null && mv -f "$rd/report.json.tmp" "$rd/report.json" 2>/dev/null \
    || printf '{"schema":1,"run_id":"%s","exit":20,"failed_stage":"%s","reason":"%s"}\n' "$CPA_RUN_ID" "$1" "$2" > "$rd/report.json" 2>/dev/null
}
trap '_rc=$?; case "$_rc" in 0|10|11|12|13|14|15|20) ;; *) printf "cpa: internal_error exit %s before the run was opened\n" "$_rc" >&2; core_report self_test internal_error "exit $_rc"; exit 20 ;; esac' EXIT
SELF="$(realpath -- "$0" 2>/dev/null)" || SELF="$0"
D="$(dirname "$SELF")"; RD="$D/repo"; LO="$D/longops"

# ---- the self-test (CENTRAL C3 (iv)): writes nothing, refuses with 20 -------------------------------------------------------------------
selfrefuse() { jq -nc --arg r "$1" --arg w "$2" '{refusal:$r,reason:$w,see:"scripts/repo/host_entry/INSTALL.md"}' >&2; core_report self_test "$1" "$2"; exit 20; }
# the values of the owner's own inputs: absolute (HOME and CPA_HOST_STATE may be empty, state_unresolved is the host entry's refusal), every PATH element absolute (WF17-cpa ENV-8/ENV-9)
for _n in HOME CPA_HOST_STATE TMPDIR; do
  _v="${!_n-}"; [ -z "$_v" ] || case "$_v" in /*) ;; *) selfrefuse env_value_unsafe "$_n must be an absolute path" ;; esac
done
case ":$PATH:" in *::*) selfrefuse env_value_unsafe "an empty PATH element" ;; esac
_ifs="$IFS"; IFS=:; for _e in $PATH; do case "$_e" in /*) ;; *) IFS="$_ifs"; selfrefuse env_value_unsafe "a relative PATH element" ;; esac; done; IFS="$_ifs"; unset _n _v _e _ifs
for t in git jq sha256sum realpath python3; do command -v "$t" >/dev/null 2>&1 || selfrefuse tool_missing "$t"; done
[ -n "${CPA_ROOT:-}" ] && [ -n "${CPA_RUN_ID:-}" ] || selfrefuse not_via_host_entry "CPA_ROOT or CPA_RUN_ID unset"
EXPECT="$CPA_ROOT/.audit/commit-push/$CPA_RUN_ID/released/scripts/commit-push-all.sh"
[ "$(realpath -- "$EXPECT" 2>/dev/null)" = "$SELF" ] || selfrefuse not_via_host_entry "own path is not the released copy of this run"
SNAP="$(dirname "$D")/trust-snapshot.json"
[ -r "$SNAP" ] || selfrefuse not_via_host_entry "no trust-snapshot.json beside the released copy"
want="$(jq -r '.entries[]|select(.path=="scripts/commit-push-all.sh").sha256' "$SNAP" 2>/dev/null)"
[ -n "$want" ] && [ "$want" = "$(sha256sum "$SELF" | cut -d' ' -f1)" ] || selfrefuse not_via_host_entry "own sha256 differs from the snapshot entry"
ents="$(jq -cS '.entries|sort_by(.path)' "$SNAP" 2>/dev/null)"; msha="$(printf '%s' "$ents" | sha256sum | cut -d' ' -f1)"
# the approved sha256 of every file, held in memory (TOCTOU-6): each helper is re-hashed against THIS table immediately before it runs, and the file is read-only besides
declare -A APPROVED=(); while IFS=$'\t' read -r _p _h; do APPROVED["$_p"]="$_h"; done < <(jq -r '.entries[]|[.path,.sha256]|@tsv' "$SNAP" 2>/dev/null); unset _p _h
approved_ok() { # approved_ok <absolute path of an approved file>: its sha256 is the snapshot's entry for it NOW
  local rel="scripts/${1#"$D"/}" want="" got; want="${APPROVED[$rel]-}"; [ -n "$want" ] || return 1
  got="$(sha256sum -- "$1" 2>/dev/null | cut -d' ' -f1)"; [ "$got" = "$want" ]
}
[ "$msha" = "$(jq -r .manifest_sha256 "$SNAP" 2>/dev/null)" ] || selfrefuse snapshot_not_trusted "manifest_sha256 of the snapshot is not the sha256 of its entries"
STATE="${CPA_HOST_STATE:-${HOME:-}/.local/state/cpa-host}"; TRUSTF="$STATE/trust.json"
pkey="$(git -C "$CPA_ROOT" rev-list --first-parent --max-parents=0 HEAD 2>/dev/null | head -1)"
[ -s "$TRUSTF" ] || selfrefuse snapshot_not_trusted "trust file unreadable or empty"
if jq -e --arg k "$pkey" --arg m "$msha" '.projects[$k].history|map(select(.op=="revoke")|.manifest_sha256)|index($m)!=null' "$TRUSTF" >/dev/null 2>&1; then selfrefuse snapshot_not_trusted revoked; fi
jq -e --arg k "$pkey" --arg m "$msha" '.projects[$k].history|map(select(.op!="revoke")|.manifest_sha256)|index($m)!=null' "$TRUSTF" >/dev/null 2>&1 \
  || selfrefuse snapshot_not_trusted "no approval of the project records this manifest"

# ---- the body: one function, so that bash PARSES the whole script before running any of it (TOCTOU-6: a later edit of this file changes nothing of this run) ---------
# shellcheck source=scripts/repo/lib_safe.sh
approved_ok "$RD/lib_safe.sh" || selfrefuse approved_copy_changed "scripts/repo/lib_safe.sh"
. "$RD/lib_safe.sh"
cpa_main() {
exec 8>&2      # the run's own stderr, kept apart: a helper call is written `helper_run X >file 2>&1`, and a refusal raised INSIDE that call (approved_check) must still reach the operator
# ---- helpers of this script --------------------------------------------------------------------------------------------------------------
# jq 1.6 (the WP-09 images) exits 0 under -e when its input is EMPTY, jq 1.7+ does not: emptiness is therefore checked here, never left to jq
json_is() { # json_is <jq -e filter> <file>: the file is non-empty and the filter holds
  [ -s "$2" ] || return 1; jq -e "$1" "$2" >/dev/null 2>&1; }
ROOT="$CPA_ROOT"; RUN_ID="$CPA_RUN_ID"; AUDITD="$ROOT/.audit/commit-push"; CPA_RUN="$AUDITD/$RUN_ID"; ADOPT="${CPA_ADOPTION_COMMIT:-}"
cd "$ROOT" || selfrefuse internal "cannot enter $ROOT"; unset OLDPWD     # bash exports OLDPWD after a cd: it is no input of the helpers
LOG="$CPA_RUN/log.txt"
log() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" >> "$LOG" 2>/dev/null; }
# no hook of .git/hooks/ or .git/modules/*/hooks/ runs in any git call of this script or of a helper (CENTRAL C4), and no program named by a config key does (WF17-cpa REPO-2 belt:
# the repository-config gate at S0 refuses such keys, these seven settings make the gap between two checks harmless)
export GIT_CONFIG_COUNT=7 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0="$CPA_RUN/no-hooks"      # the only env config of a run: the allowlist above dropped the caller's
export GIT_CONFIG_KEY_1=core.fsmonitor GIT_CONFIG_VALUE_1=false GIT_CONFIG_KEY_2=commit.gpgSign GIT_CONFIG_VALUE_2=false GIT_CONFIG_KEY_3=tag.gpgSign GIT_CONFIG_VALUE_3=false
export GIT_CONFIG_KEY_4=credential.helper GIT_CONFIG_VALUE_4= GIT_CONFIG_KEY_5=core.askPass GIT_CONFIG_VALUE_5= GIT_CONFIG_KEY_6=protocol.ext.allow GIT_CONFIG_VALUE_6=never
export GIT_TERMINAL_PROMPT=0 LONGOPS_REPO="$ROOT" CPA_APPROVED_DIR="$CPA_RUN/released" CPA_RUN CPA_RUN_ID

STG_ID=(); STG_NAME=(); CUR=""; FAILSTAGE=""; FAILREASON=""; FAILDETAIL=""; LOCKED=0; LOCK_ACQ=0; LOCK_REL=0; HELD=0; DEFERRED=(); PENDING_CHECKS=0; NESTED=(); CPA_CHILD=""; HELPER_STILL=""
MSG=""; PATHS=""; AWAIT=""; REPO=""; RESOLVE=""; LOCAL_ONLY=""; CBI=""; TARGET="$ROOT"; BR=""; OWNED=""; START_HEAD=""; HELDLINES=(); HELDKEYS=(); RC_FINAL=0
stage() { CUR="$1"; STG_ID+=("$1"); STG_NAME+=("$2"); log "stage $1 $2"; }
fail() { FAILSTAGE="$1"; FAILREASON="$3"; FAILDETAIL="${4:-}"; log "FAIL $1 $2 $3 $4"; printf 'cpa: FAIL %s %s %s%s\n' "$1" "$2" "$3" "${4:+ ($4)}" >&8; exit "$2"; }
# an approved file is executed only when its sha256 is the snapshot's entry NOW (TOCTOU-6): the released copy is read-only, and this check is the second line
approved_check() { approved_ok "$1" || fail "${CUR:-S0}" 20 approved_copy_changed "scripts/${1#"$D"/}"; }
approved_run() { approved_check "$1"; "$@"; }          # a short synchronous helper
# helper_bg: the form of every helper call that can take long or that a signal must be able to interrupt (SIG-5): the helper runs in the background and this shell WAITS, so a trapped
# signal is delivered at once instead of after the helper returns; finish forwards TERM to it. Redirections of the caller apply to the child.
helper_bg() { "$@" & CPA_CHILD=$!; wait "$CPA_CHILD"; local hrc=$?; CPA_CHILD=""; return "$hrc"; }
helper_run() { approved_check "$1"; helper_bg "$@"; }
defer() { # defer <flag> <reason> [stage]: one row in the run directory (record_deferral.sh); a failed write is 20, never a 14 without its row
  approved_run "$RD/record_deferral.sh" --run-dir "$CPA_RUN" --flag "$1" --reason "$2" >>"$LOG" 2>&1 || fail "${3:-$CUR}" 20 deferral_write_failed "$1"
  DEFERRED+=("$1")
}
child_alive() { local st; st="$(sed -e 's/^.*) //' "/proc/$1/stat" 2>/dev/null | cut -c1)"; [ -n "$st" ] && [ "$st" != Z ]; }
# descendants <pid>: the pids of every descendant of <pid> (a /proc walk, parent pid = field 4 of /proc/<pid>/stat; never a pgrep match)
descendants() {
  local f line pid par q c; local -A P=()
  for f in /proc/[0-9]*/stat; do { read -r line < "$f"; } 2>/dev/null || continue; pid="${f#/proc/}"; pid="${pid%%/*}"; line="${line##*) }"; par="${line#* }"; par="${par%% *}"; P[$pid]="$par"; done
  q=("$1"); for c in "${q[@]}"; do for pid in "${!P[@]}"; do [ "${P[$pid]}" = "$c" ] && { q+=("$pid"); printf '%s\n' "$pid"; }; done; done
}
# write_report: report.json from the CURRENT state, atomically; 0 when the file exists afterwards and is non-empty (REPORT-1)
write_report() {
  local files commits interrupted live deferrals d nested unj
  [ -d "$CPA_RUN" ] || return 1
  files="$(cd "$CPA_RUN" && find . -type f ! -path ./report.json ! -name 'report.json.tmp' -print0 2>/dev/null | sort -z | xargs -0 -r sha256sum -z 2>/dev/null \
    | jq -Rs 'split("\u0000")|map(select(length>0)|{(.[66:]|ltrimstr("./")):.[0:64]})|add // {}' 2>/dev/null)" || files='{}'
  [ -n "$files" ] || files='{}'
  commits="$([ -f "$CPA_RUN/commits.tsv" ] && jq -R -s 'split("\n")|map(select(length>0)|split("\t")|{repo:.[0],sha:.[1],cpa_run:.[2]})' "$CPA_RUN/commits.tsv" 2>/dev/null || echo '[]')"
  # ONE liveness model (MODEL-1): cpa_rundir_state. A run directory is INTERRUPTED when its process is gone and nothing recorded its end; the ones THIS run closed at S0 are listed too.
  interrupted="$( { cat "$CPA_RUN/closed.txt" 2>/dev/null; cd "$AUDITD" 2>/dev/null && { shopt -s nullglob; for d in */; do d="${d%/}"; [ "$d" = "$RUN_ID" ] && continue; [ "$(cpa_rundir_state "$AUDITD/$d")" = interrupted ] && echo "$d"; done; }; } | sort -u | jq -R -s 'split("\n")|map(select(length>0))' 2>/dev/null || echo '[]')"
  live="$(cd "$AUDITD" 2>/dev/null && { shopt -s nullglob; for d in */; do d="${d%/}"; [ "$d" = "$RUN_ID" ] && continue; [ "$(cpa_rundir_state "$AUDITD/$d")" = live ] && echo "$d"; done; } | jq -R -s 'split("\n")|map(select(length>0))' 2>/dev/null || echo '[]')"
  deferrals="$(approved_ok "$RD/record_deferral.sh" && "$RD/record_deferral.sh" --run-dir "$CPA_RUN" --list 2>/dev/null)"
  nested="$(printf '%s\0' "${NESTED[@]+"${NESTED[@]}"}" | jq -Rs 'split("\u0000")|map(select(length>0))' 2>/dev/null || echo '[]')"
  unj="$(cat "$CPA_RUN/unjudged.tsv" 2>/dev/null)"
  jq -n --arg id "$RUN_ID" --arg dir ".audit/commit-push/$RUN_ID/" --argjson rc "$RC_FINAL" --arg fs "$FAILSTAGE" --arg fr "$FAILREASON" --arg fd "$FAILDETAIL" \
    --arg repo "$REPO" --arg lo "$LOCAL_ONLY" --arg pf "$PATHS" --arg aw "$AWAIT" --arg rm "$RESOLVE" --arg cbi "$CBI" --arg deferred "${deferrals:-}" \
    --argjson held "$HELD" --argjson pend "$PENDING_CHECKS" --argjson lacq "$LOCK_ACQ" --argjson lrel "$LOCK_REL" --argjson live "${live:-[]}" --arg trust "$TRUSTF" --arg stages "$(for i in "${!STG_ID[@]}"; do printf '%s:%s\n' "${STG_ID[$i]}" "${STG_NAME[$i]}"; done)" \
    --argjson commits "${commits:-[]}" --argjson interrupted "${interrupted:-[]}" --argjson files "$files" --argjson nested "${nested:-[]}" --arg held_lines "$(printf '%s\n' "${HELDLINES[@]:-}")" --arg unj "$unj" \
    '{schema:1,run_id:$id,run_dir:$dir,exit:$rc,
      failed_stage:(if $fs=="" then null else $fs end),reason:(if $fr=="" then null else $fr end),detail:(if $fd=="" then null else $fd end),
      mode:{repo:(if $repo=="" then null else $repo end),local_only:($lo!=""),paths_from:(if $pf=="" then null else "paths.txt" end),awaits_review:(if $aw=="" then null else $aw end),
            resolve_merge:(if $rm=="" then null else $rm end),commit_before_integrate:($cbi!="")},
      lock:{held:($lacq>0),released:($lrel>0)},trust_file:$trust,
      deferred_gates:($deferred|split(",")|map(select(length>0))),check_pending_release:($pend>0),held:($held>0),held_commits:($held_lines|split("\n")|map(select(length>0))),
      nested_unsettled:$nested,
      unjudged_paths:($unj|split("\n")|map(select(length>0)|split("\t")|{path:.[0],check:.[1],deferred:(.[2]=="yes")})),
      commits:$commits,stages:($stages|split("\n")|map(select(length>0)|split(":")|{id:.[0],name:.[1]})),
      interrupted_runs:$interrupted,live_runs:$live,files:$files}' > "$CPA_RUN/report.json.tmp" 2>>"$LOG" && mv -f "$CPA_RUN/report.json.tmp" "$CPA_RUN/report.json"
  [ -f "$CPA_RUN/report.json" ] && [ -s "$CPA_RUN/report.json" ]
}
# shellcheck disable=SC2329  # invoked through `trap finish EXIT`
finish() { # S8: the report on every exit path BEFORE the lock is released (MODEL-4), the lock released, the summary line
  local rc=$? rl=0 w=0
  cpa_signals_ignore; trap - EXIT      # every catchable signal is now IGNORED, and the ignore is inherited by release.sh, jq, find and sha256sum (SIG-3/SIG-4); SIGKILL cannot be handled
  # a helper still running when the run ends: TERM it and wait a bounded time; one that outlives the bound keeps the lock held and the report says so (SIG-5)
  if [ -n "${CPA_CHILD:-}" ] && child_alive "$CPA_CHILD"; then
    for w in $(descendants "$CPA_CHILD"); do [[ "$w" =~ ^[0-9]+$ ]] && [ "$w" -gt 1 ] && [ "$w" != "$$" ] && kill -TERM "$w" 2>/dev/null; done     # the helper's own children too (a git call it waits for); only pids of this process tree, never a group
    kill -TERM "$CPA_CHILD" 2>/dev/null; w=0
    while child_alive "$CPA_CHILD" && [ "$w" -lt 300 ]; do sleep 0.1; w=$((w+1)); done
    if child_alive "$CPA_CHILD"; then HELPER_STILL="$CPA_CHILD"; else wait "$CPA_CHILD" 2>/dev/null; fi
  fi
  case "$rc" in 0|10|11|12|13|14|15|20) ;; *) FAILSTAGE="${FAILSTAGE:-${CUR:-S0}}"; FAILREASON="${FAILREASON:-internal_error}"; FAILDETAIL="${FAILDETAIL:-exit $rc}"; rc=20 ;; esac
  if [ -n "$HELPER_STILL" ]; then rc=20; FAILSTAGE="${FAILSTAGE:-${CUR:-S0}}"; FAILREASON=helper_still_running; FAILDETAIL="pid $HELPER_STILL did not end within 30 s of TERM; the lock stays held"; fi
  if [ -d "$CPA_RUN" ]; then
    RC_FINAL="$rc"; write_report || { rc=20; log "report.json could not be written"; }
    if [ "$LOCKED" != 0 ] && [ -z "$HELPER_STILL" ]; then
      if approved_ok "$LO/release.sh"; then "$LO/release.sh" --purpose commit_push --run-id "$RUN_ID" >>"$LOG" 2>&1; rl=$?; else rl=99; fi
      case "$LOCKED:$rl" in
        *:0) LOCK_ACQ=1; LOCK_REL=1 ;;
        2:4) : ;;                                  # the acquire never took the claim: nothing to release
        *) log "release.sh failed ($rl)"; LOCK_REL=0
           case "$rc" in 0|14) FAILSTAGE="${FAILSTAGE:-${CUR:-S0}}"; FAILREASON=lock_release_failed; FAILDETAIL="release.sh exit $rl${FAILDETAIL:+ (before: $FAILDETAIL)}"; rc=20 ;;
             *) FAILDETAIL="${FAILDETAIL:+$FAILDETAIL; }lock not released: release.sh exit $rl" ;; esac ;;
      esac
      RC_FINAL="$rc"; write_report || rc=20
    fi
  else rc=20; printf 'cpa: the run directory %s is gone\n' "$CPA_RUN" >&8; fi
  { [ -f "$CPA_RUN/report.json" ] && [ -s "$CPA_RUN/report.json" ]; } || { rc=20; printf 'cpa: report_write_failed (%s)\n' "$CPA_RUN" >&8; }
  printf 'cpa: exit=%s run=%s stage=%s reason=%s\n' "$rc" "$RUN_ID" "${FAILSTAGE:-${CUR:-}}" "${FAILREASON:-}" >&8
  exit "$rc"
}
trap finish EXIT       # the signal traps (above) stay: each ends the run through finish with 128+n

# the sweep judges THIS repository under the owner's git settings only (WF14 N3): its root is given here, never inherited (its owned set is the approved own_orgs.txt, as ours), and the run's own
# env config (the no-hooks setting and the belt) is taken off it, because the sweep's planted control needle must see a blocking core.hooksPath to prove it can see one
sweep_run() { # sweep_run <sweep.sh arguments>
  approved_check "$D/anti-mess/sweep.sh"
  helper_bg env -u GIT_CONFIG_COUNT -u GIT_CONFIG_KEY_0 -u GIT_CONFIG_VALUE_0 -u GIT_CONFIG_KEY_1 -u GIT_CONFIG_VALUE_1 -u GIT_CONFIG_KEY_2 -u GIT_CONFIG_VALUE_2 -u GIT_CONFIG_KEY_3 -u GIT_CONFIG_VALUE_3 \
    -u GIT_CONFIG_KEY_4 -u GIT_CONFIG_VALUE_4 -u GIT_CONFIG_KEY_5 -u GIT_CONFIG_VALUE_5 -u GIT_CONFIG_KEY_6 -u GIT_CONFIG_VALUE_6 ANTIMESS_ROOT="$ROOT" "$D/anti-mess/sweep.sh" "$@"
}
# the gates that are NOT built (scripts/repo/owed_gates.tsv of the approved copy): a run without them is a deferral, never a clean pass (WF11 review F3). Read at S0, so
# an S1 merge commit carries the flag too (WF14 N5). An absent or unreadable list is itself owed; a row with another condition is 20.
owed_gates() {
  local f="$RD/owed_gates.tsv" g w n names=""
  [ -r "$f" ] || { defer GATES_NOT_BUILT "owed_gates.tsv absent from the approved copy: the set of unbuilt gates cannot be read" S0; return 0; }
  while IFS=$'\t' read -r g w n || [ -n "$g" ]; do
    case "$g" in ''|'#'*) continue ;; esac
    [[ "$g" =~ ^[A-Z][A-Z0-9_]*$ ]] || fail S0 20 owed_gates_invalid "gate name $(printf '%q' "$g")"
    case "$w" in always) ;; declared_paths) [ -s "$CPA_RUN/paths.list" ] || continue ;; *) fail S0 20 owed_gates_invalid "$g: when=$(printf '%q' "$w")" ;; esac
    names="${names:+$names,}$g"
  done < "$f"
  [ -z "$names" ] || defer GATES_NOT_BUILT "unbuilt gates, the run went on without them: $names" S0
}
# listing <file> <what> <command...>: a git listing is written to a file and its STATUS is read; a failed or cut-short listing is never read as an empty one (LIST)
listing() { local f="$1" what="$2"; shift 2; "$@" >"$f" 2>>"$LOG" || fail "${CUR:-S0}" 20 git_listing_failed "$what"; }
# head_verdict_state <verdict path>: ok | not_go | invalid | absent for the verdict file COMMITTED in the main HEAD, judged by the ONE predicate (verdict_go.py; VERDICT)
head_verdict_state() {
  local vp="$1" sch="specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json"
  git -C "$ROOT" cat-file blob "HEAD:$vp" > "$CPA_RUN/vtmp.json" 2>/dev/null || { echo absent; return 0; }
  git -C "$ROOT" cat-file blob "HEAD:$sch" > "$CPA_RUN/vschema.json" 2>/dev/null || { echo invalid; return 0; }
  approved_check "$RD/verdict_go.py"; python3 -I "$RD/verdict_go.py" --schema "$CPA_RUN/vschema.json" --verdict "$CPA_RUN/vtmp.json" 2>>"$LOG" || echo invalid
}

# ---- S0 preflight ---------------------------------------------------------------------------------------------------------------------------
stage S0 preflight
printf '%s %s\n' "$$" "$(sed -e 's/^.*) //' "/proc/$$/stat" 2>/dev/null | awk '{print $20}')" > "$CPA_RUN/run.pid" 2>/dev/null || log "run.pid not written"
while [ $# -gt 0 ]; do
  case "$1" in
    --local-only) LOCAL_ONLY=1; shift ;;
    --commit-before-integrate) CBI=1; shift ;;
    --awaits-review) [ $# -ge 2 ] || fail S0 20 usage_error "--awaits-review needs a verdict file"; [ -z "$AWAIT" ] || fail S0 20 usage_error "--awaits-review given twice"; AWAIT="$2"; shift 2 ;;
    --repo) [ $# -ge 2 ] || fail S0 20 usage_error "--repo needs a path"; [ -z "$REPO" ] || fail S0 20 usage_error "--repo given twice"; REPO="$2"; shift 2 ;;
    --paths-from) [ $# -ge 2 ] || fail S0 20 usage_error "--paths-from needs a file"; [ -z "$PATHS" ] || fail S0 20 usage_error "--paths-from given twice"; PATHS="$2"; shift 2 ;;
    --resolve-merge) [ $# -ge 2 ] || fail S0 20 usage_error "--resolve-merge needs a directory"; [ -z "$RESOLVE" ] || fail S0 20 usage_error "--resolve-merge given twice"; RESOLVE="$2"; shift 2 ;;
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
# an option that would change nothing is refused, never ignored (INPUT)
[ -z "$AWAIT" ] || [ -n "$PATHS" ] || fail S0 20 usage_error "--awaits-review needs --paths-from"
[ -z "$MSG" ] || [ -n "$PATHS" ] || fail S0 20 usage_error "a message without --paths-from commits nothing"
# a message line that reads like one of the run's own trailers is refused: push_recursive and the merge path read the LAST such line (MSG)
[[ $'\n'"$MSG" =~ $'\n'(CPA-Run|Deferred-Gates|Awaits-Review|Foreign-Commit):\  ]] && fail S0 20 usage_error "message_impersonates_trailer"
if [ -n "$PATHS" ]; then
  [ -f "$PATHS" ] && [ -r "$PATHS" ] || fail S0 20 usage_error "--paths-from file unreadable or no regular file"
  [ "$(stat -c %s -- "$PATHS" 2>/dev/null || echo 99999999)" -le 1048576 ] || fail S0 20 usage_error "--paths-from file larger than 1 MiB"
  [ -n "$MSG" ] || fail S0 20 usage_error "a commit message is required with --paths-from"
  cp -- "$PATHS" "$CPA_RUN/paths.txt" || fail S0 20 usage_error "cannot copy the change set"
  python3 -I -c 'import sys; sys.stdin.buffer.read().decode("utf-8")' < "$CPA_RUN/paths.txt" 2>/dev/null || fail S0 20 unsafe_path "the change set is not UTF-8"
else : > "$CPA_RUN/paths.txt"; fi
[ -z "$AWAIT" ] || safe_declpath "$AWAIT" || fail S0 20 usage_error "--awaits-review: unsafe verdict path"

# the owned organisations have ONE source: the approved scripts/audit/own_orgs.txt of the released copy (no caller-environment override, WF11 review F1)
OWNED="$( [ -r "$D/audit/own_orgs.txt" ] && grep -v '^[[:space:]]*#' "$D/audit/own_orgs.txt" | awk 'NF{print $1}' | paste -sd, - )"
is_owned() { approved_check "$D/audit/org_of.py"; repo_owned "$1" "$OWNED"; }     # EVERY url of EVERY remote names an own organisation (REPO-3a)
# the initialised submodules at every depth, NUL-safe, deepest first (NAME-1); the listing status is read
SUBS=()
list_submodules() {
  local sp rec dd; SUBS=()
  listing "$CPA_RUN/subs.z" "submodule listing" git -C "$ROOT" submodule foreach --quiet --recursive 'printf "%s\0" "$displaypath"'
  while IFS= read -r -d '' sp; do dd="${sp//[^\/]/}"; printf '%s\t%s\0' "${#dd}" "$sp"; done < "$CPA_RUN/subs.z" | sort -z -t$'\t' -k1,1nr > "$CPA_RUN/subs.sorted.z"
  while IFS= read -r -d '' rec; do sp="${rec#*$'\t'}"; if safe_relpath "$sp"; then SUBS+=("$sp"); else log "submodule path $(printf '%q' "$sp") is unsafe, skipped"; fi; done < "$CPA_RUN/subs.sorted.z"
}
if [ -n "$REPO" ]; then
  safe_declpath "$REPO" || fail S0 20 usage_error "--repo: unsafe path"; REPO="${REPO%/}"
  [ -d "$ROOT/$REPO" ] || fail S0 20 repo_not_found "$REPO"
  [ "$(realpath -- "$(git -C "$ROOT/$REPO" rev-parse --show-toplevel 2>/dev/null)" 2>/dev/null)" = "$(realpath -- "$ROOT/$REPO")" ] || fail S0 20 repo_not_found "$REPO is not the root of its own work tree"
  list_submodules; ins=1; for sp in "${SUBS[@]+"${SUBS[@]}"}"; do [ "$sp" = "$REPO" ] && ins=0; done
  [ "$ins" = 0 ] || fail S0 20 repo_not_found "$REPO is not a submodule of this repository"
  [ -z "$(git -C "$ROOT/$REPO" remote 2>/dev/null)" ] || is_owned "$ROOT/$REPO" || fail S0 20 repo_not_owned "$REPO: a --repo run targets an own-organisation repository only (every remote url and push url must name one)"
  TARGET="$ROOT/$REPO"
fi
# the repository's OWN configuration holds only approved keys (REPO-2/REPO-3b): a key that names a program or rewrites a url is refused before anything runs
approved_check "$RD/repo_config_allow.tsv"
cg="$(repo_config_gate "$TARGET" "$RD/repo_config_allow.tsv")" || fail S0 20 repo_config_not_approved "$cg"
# the main repository is held to the same ownership rule as a --repo target: pushing is decided by the approved own_orgs.txt, so EVERY url and push url of EVERY remote must name an own organisation
# (an extra push url that names a third party is refused before anything is committed; REPO-3a/3b). Without an approved list nothing is judged (documented in INSTALL.md).
[ -z "$OWNED" ] || [ -n "$REPO" ] || [ -z "$(git -C "$TARGET" remote 2>/dev/null)" ] || is_owned "$TARGET" || fail S0 20 repo_not_owned "the main repository has a remote url or push url that names no own organisation (scripts/audit/own_orgs.txt)"
# a branch is a full ref: a tag or a branch with the same short name can never stand in for it (NAME-4)
ref="$(git -C "$TARGET" symbolic-ref -q HEAD)" || fail S0 20 wrong_branch "detached HEAD in ${REPO:-the main repository}"
case "$ref" in refs/heads/*) BR="${ref#refs/heads/}" ;; *) fail S0 20 wrong_branch "HEAD is $ref" ;; esac
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
  p="${line%%$'\t'*}"; v=""; if [ "$p" != "$line" ]; then v="${line#*$'\t'}"; [ -n "$v" ] || fail S0 20 unsafe_path "empty verdict after TAB: $(printf '%q' "$p")"; fi; [ -n "$v" ] || v="$AWAIT"
  safe_declpath "$p" || fail S0 20 unsafe_path "$(printf '%q' "$p")"
  # a name made only of whitespace is committed by the core but was dropped by a stripping parser (SKIP-1): refused here, the other parsers keep every line verbatim
  IFS=/ read -r -a _comp <<< "$p"; for _c in "${_comp[@]}"; do [[ "$_c" =~ ^[[:space:]]+$ ]] && fail S0 20 unsafe_path "whitespace-only name: $(printf '%q' "$p")"; done
  [ -z "$v" ] || safe_declpath "$v" || fail S0 20 unsafe_path "verdict $(printf '%q' "$v")"
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

# remotes: no remote at all, or every remote unreachable, gives 20 no_remote_reachable (nothing to judge S5 and S6 against), --local-only included
RL="$(git -C "$TARGET" remote 2>/dev/null)" || fail S0 20 git_listing_failed "remote list"
[ -n "$RL" ] || fail S0 20 no_remote_reachable "${REPO:-the main repository} has no remote"
reach=0; for r in $RL; do safe_remote "$r" || continue; timeout 60 git -C "$TARGET" ls-remote -- "$r" >/dev/null 2>&1 && reach=1; done
[ "$reach" = 1 ] || fail S0 20 no_remote_reachable "none of the remotes of ${REPO:-the main repository} could be read"

LOCKED=2     # marked BEFORE the acquire: a signal that arrives while acquire.sh runs still reaches the compare-and-swap release in finish (RES-2)
approved_check "$LO/acquire.sh"; helper_bg "$LO/acquire.sh" --purpose commit_push --run-id "$RUN_ID" --pid "$$" >"$CPA_RUN/lock.out" 2>&1; lrc=$?
# a stale claim (its holder PROVEN dead) is reaped by this run and the acquire retried once (RES-2 e, 11.4.232 E, 11.4.234 D); a live or unreadable holder stays a refusal
reapnote=""
if [ "$lrc" = 4 ] && grep -q '^stale_claim' "$CPA_RUN/lock.out" 2>/dev/null; then
  approved_check "$LO/reap.sh"; helper_bg "$LO/reap.sh" --purpose commit_push >"$CPA_RUN/reap.out" 2>&1; rrc=$?
  log "S0 stale claim: reap.sh --purpose commit_push exit $rrc: $(head -c 200 "$CPA_RUN/reap.out" | tr '\n\t' '  ')"
  reapnote="; S0 ran reap.sh --purpose commit_push (exit $rrc: $(head -c 160 "$CPA_RUN/reap.out" | tr '\n\t' '  '))"
  helper_bg "$LO/acquire.sh" --purpose commit_push --run-id "$RUN_ID" --pid "$$" >"$CPA_RUN/lock.out" 2>&1; lrc=$?
fi
case "$lrc" in
  0) LOCKED=1; LOCK_ACQ=1 ;;
  3|4|5|6) LOCKED=0; lk="$(head -c 300 "$CPA_RUN/lock.out" | tr '\n\t' '  ')"
     # a dead holder's claim is no live holder: holder.sh says `none` for it, so the cause is read from acquire.sh itself and the remediation is named
     if [ "$lrc" = 4 ] && grep -q '^stale_claim' "$CPA_RUN/lock.out" 2>/dev/null; then fail S0 20 lock_stale_claim "a dead run left its claim and it could not be reaped, no run is live: $lk$reapnote"; fi
     h="$(approved_run "$LO/holder.sh" commit_push 2>/dev/null)"
     fail S0 20 lock_held "holder run $(jq -r '.run_id // "?"' <<< "$h" 2>/dev/null) pid $(jq -r '.pid // "?"' <<< "$h" 2>/dev/null); acquire.sh exit $lrc: $lk; this run wrote only $CPA_RUN" ;;
  *) fail S0 20 lock_error "acquire.sh exit $lrc: $(head -c 200 "$CPA_RUN/lock.out")" ;;
esac
# RES-1 c: every run directory that has no report and whose process is PROVEN gone (run.pid pid and start ticks against /proc) is closed by this run, under the lock: interrupted.json records
# who closed it and when, and a report.json (closed_by) states the end, so the sweep and every later run read it as finished. Nothing is deleted. A directory without run.pid (a
# leftover of an older version) is closed only when it is older than this run; the private names of a host that died before its rename are removed (they hold nothing).
: > "$CPA_RUN/closed.txt"
rdstate_of() { # rdstate_of <run dir>: RDS = the state of the run directory; a state that cannot be read (the predicate failed, or said something unknown) is a 20, never "not live" (RES)
  RDS="$(cpa_rundir_state "$1")" || fail S0 20 internal_error "the state of the run directory $1 could not be read (cpa_rundir_state failed)"
  case "$RDS" in finished|closed|live|interrupted) ;; *) fail S0 20 internal_error "the state of the run directory $1 is unknown: $RDS" ;; esac
}
for dd in "$AUDITD"/.new.*; do [ -d "$dd" ] || continue; rdstate_of "$dd"; [ "$RDS" = live ] || rm -rf -- "$dd"; done
for dd in "$AUDITD"/*/; do
  dd="${dd%/}"; d="${dd##*/}"; [ -d "$dd" ] || continue; [ "$d" = "$RUN_ID" ] && continue
  rdstate_of "$dd"; [ "$RDS" = interrupted ] || continue
  if [ ! -s "$dd/run.pid" ] && ! [ "$dd" -ot "$CPA_RUN/run.pid" ]; then continue; fi
  last="$(awk '$2=="stage"{s=$3} END{print s}' "$dd/log.txt" 2>/dev/null)"; opid="$(cut -d' ' -f1 "$dd/run.pid" 2>/dev/null)"; now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  jq -nc --arg id "$d" --arg by "$RUN_ID" --arg at "$now" --arg pid "$opid" --arg ls "${last:-}" '{schema:1,run_id:$id,closed_by:$by,closed_at:$at,run_pid:$pid,last_stage:$ls,reason:"no_report_process_gone"}' > "$dd/interrupted.json.tmp" 2>>"$LOG" \
    && mv -f "$dd/interrupted.json.tmp" "$dd/interrupted.json" \
    && jq -nc --arg id "$d" --arg by "$RUN_ID" --arg at "$now" --arg ls "${last:-}" '{schema:1,run_id:$id,run_dir:(".audit/commit-push/"+$id+"/"),exit:20,failed_stage:(if $ls=="" then null else $ls end),reason:"no_report_process_gone",detail:("the process of this run was gone and no report was written; closed by "+$by+" at "+$at),closed_by:$by,mode:{},lock:{held:false,released:false},deferred_gates:[],check_pending_release:false,held:false,held_commits:[],nested_unsettled:[],unjudged_paths:[],commits:[],stages:[],interrupted_runs:[],live_runs:[],files:{}}' > "$dd/report.json.tmp" 2>>"$LOG" \
    && mv -f "$dd/report.json.tmp" "$dd/report.json" || fail S0 20 close_dead_run_failed "$d"
  printf '%s\n' "$d" >> "$CPA_RUN/closed.txt"; log "S0 closed the dead run $d (last stage ${last:-none})"
done
export DISK_HEADROOM_OUT_DIR="$CPA_RUN/disk/"
approved_check "$LO/check_no_build_writing_tracked.sh"; helper_bg "$LO/check_no_build_writing_tracked.sh" >"$CPA_RUN/registry.out" 2>&1; rrc=$?
case "$rrc" in 0) ;; 1) fail S0 20 evidence_writer_active "$(head -c 300 "$CPA_RUN/registry.out" | tr '\n\t' '  ')" ;; *) fail S0 20 registry_check_error "rc=$rrc" ;; esac
[ -z "${SKIP_LONG:-}" ] || defer SKIP_LONG "$SKIP_LONG" S0     # recorded at S0, so an S1 merge commit carries it too
if [ -x "$D/anti-mess/sweep.sh" ]; then
  sweep_run --stage S0 --paths-from "$CPA_RUN/paths.txt" ${REPO:+--repo "$REPO"} >"$CPA_RUN/sweep-s0.out" 2>&1; src=$?
  case "$src" in 0) ;; 10) fail S0 20 sweep_finding "an undeclared change or another catalogued drift (see sweep-s0.out)" ;; *) fail S0 20 sweep_error "rc=$src" ;; esac
else defer SWEEP_ABSENT "sweep not yet in the approved manifest (tasks.md T090, T093)" S0; fi
[ -z "$LOCAL_ONLY" ] || defer LOCAL_ONLY "push owed: --local-only" S0
owed_gates
[ -n "${SKIP_LONG:-}" ] || [ -f "$RD/long_gates.txt" ] || defer GATES_NOT_BUILT "S4 long gates: scripts/repo/long_gates.txt is absent from the approved copy, no long gate was run" S0

# the approved manifest as {"path":"sha256"} for the integration helpers' G-GATE routing
jq -c '[.entries[]|{(.path):.sha256}]|add // {}' "$SNAP" > "$CPA_RUN/approved.json" 2>/dev/null || echo '{}' > "$CPA_RUN/approved.json"

# ---- S1 / S1a / S1b ---------------------------------------------------------------------------------------------------------------------------
# the reason of a helper refusal is the one the helper STATES (a `REASON<TAB>code` line, or push_recursive's own REFUSED line), never a word picked from a fixed list the helper does not share (REASON)
helper_reason() { # helper_reason <output file> <fallback grep list or ''> <default>
  local r; r="$(awk -F'\t' '$1=="REASON"{print $2; exit}' "$1" 2>/dev/null)"
  [ -n "$r" ] || r="$(awk -F'\t' '$1=="REFUSED"{print $3; exit}' "$1" 2>/dev/null)"
  [ -n "$r" ] || { [ -z "$2" ] || r="$(grep -oE "$2" "$1" 2>/dev/null | head -1 | sed 's/.*://')"; }
  printf '%s' "${r:-$3}"
}
s1_integrate() { # s1_integrate <stage id> <with change set: 1|0>
  local st="$1" cs="$2" rc reason ffreason ff=() mf=() from to c cl
  ff=(--main-only --report-behind-submodules --root "$TARGET" --branch "$BR" --run-dir "$CPA_RUN" --json "$CPA_RUN/integrate-$st.json" --approved "$CPA_RUN/approved.json")
  [ "$cs" = 1 ] && ff+=(--changeset-from "$CPA_RUN/paths.list")
  [ -z "$OWNED" ] || ff+=(--owned-orgs "$OWNED"); [ -z "$REPO" ] && [ -n "$ADOPT" ] && ff+=(--adoption-commit "$ADOPT")
  helper_run "$RD/integrate_ff_only.sh" "${ff[@]}" >"$CPA_RUN/integrate-$st.out" 2>&1; rc=$?
  reason="$(jq -r '.reason // ""' "$CPA_RUN/integrate-$st.json" 2>/dev/null)"
  case "$rc" in
    0) from="$(jq -r '.moved.from // empty' "$CPA_RUN/integrate-$st.json" 2>/dev/null)"; to="$(jq -r '.moved.to // empty' "$CPA_RUN/integrate-$st.json" 2>/dev/null)"
       if [ -n "$from" ] && [ -n "$to" ]; then
         if [ -z "$REPO" ] && [ -n "$ADOPT" ]; then
           listing "$CPA_RUN/foreign-revs.$st" "incoming commit list" git -C "$TARGET" rev-list "$to" --not "$from" "$ADOPT"
           while IFS= read -r c; do
             [ -n "$c" ] || continue
             id="$(cpa_runid "$TARGET" "$c")" && cpa_row "$AUDITD/$id/commits.tsv" "$c" && continue
             printf '%s\n' "$c" >> "$CPA_RUN/foreign.txt"
           done < "$CPA_RUN/foreign-revs.$st"
         fi
         # a fast-forward that moved recorded gitlinks: settled by scripts/repo/record_pending_pin.sh (T435a) when it is in the approved copy, else
         # the changed paths are unsettled and S7 gives 15 for exactly them (reason nested_settle_unavailable)
         if [ "$(jq '.moved.gitlinks|length' "$CPA_RUN/integrate-$st.json" 2>/dev/null)" -gt 0 ] 2>/dev/null; then
           if [ -x "$RD/record_pending_pin.sh" ]; then
             mkdir -p "$CPA_RUN/settle"; helper_run "$RD/record_pending_pin.sh" --settle-nested "$ROOT" "$from" "$to" --in-hold "$RUN_ID" --out "$CPA_RUN/settle/" >>"$LOG" 2>&1 \
               || while IFS= read -r gp; do NESTED+=("$gp:settle_failed"); done < <(jq -r '.moved.gitlinks[].path' "$CPA_RUN/integrate-$st.json")
           else while IFS= read -r gp; do NESTED+=("$gp:nested_settle_unavailable"); done < <(jq -r '.moved.gitlinks[].path' "$CPA_RUN/integrate-$st.json"); fi
         fi
       fi; return 0 ;;
    11) fail "$st" 11 "${reason:-blocked}" "no remote could be read or a local-only commit cannot be cleared (see integrate-$st.json)" ;;
    12) case "$reason" in
          diverged|remotes_diverged|merge_path_required) ;;
          *) fail "$st" 12 "${reason:-blocked}" "see integrate-$st.json; a re-run of the window with --commit-before-integrate or CPA --repo <path> clears ff_blocked_by_local_changes / submodule_local_commits" ;;
        esac ;;
    20) fail "$st" 20 "${reason:-$(helper_reason "$CPA_RUN/integrate-$st.out" '' refused)}" "see integrate-$st.json" ;;
    *) fail "$st" 20 internal_error "integrate_ff_only.sh exit $rc" ;;
  esac
  ffreason="$reason"
  mf=(--root "$TARGET" --branch "$BR" --run-dir "$CPA_RUN" --audit-dir "$AUDITD" --repo-key "${REPO:-.}" --approved "$CPA_RUN/approved.json" --run-pid "$$")
  [ -z "$REPO" ] && [ -n "$ADOPT" ] && mf+=(--adoption-commit "$ADOPT" --anchor "$ADOPT")
  [ -z "$RESOLVE" ] || mf+=(--resolution "$RESOLVE")
  MERGE_RAN=1
  helper_run "$RD/integrate_merge.sh" "${mf[@]}" >"$CPA_RUN/integrate-merge-$st.out" 2>&1; rc=$?
  case "$rc" in
    0) # integrate_ff_only.sh sent this repository to the merge path, but integrate_merge.sh only merges tips that fast-forward cannot take (a tip that
       # descends from HEAD is skipped as ff-able): "nothing to merge" here is NOT an integration. Continuing would read an unresolved window as clean
       # (11.4.201(4): conservative-safe), so the run stops with 12 naming the ff_only reason; owed request to the T040 helpers (docs/scripts/commit-push-all.md).
       if grep -q nothing_to_merge "$CPA_RUN/integrate-merge-$st.out" 2>/dev/null; then
         fail "$st" 12 "${ffreason:-merge_path_required}" "integrate_ff_only.sh could not fast-forward and integrate_merge.sh found nothing to merge (a behind repository with diverged or gated remote tips); nothing moved"
       fi ;;
    10) fail "$st" 10 resolved_file_holds_marker "see integrate-merge-$st.out" ;;
    11) fail "$st" 11 remote_unreachable "see integrate-merge-$st.out" ;;
    12) reason="$(helper_reason "$CPA_RUN/integrate-merge-$st.out" 'merge_conflict|remotes_diverged|ff_blocked_by_local_changes|merge_target_moved' merge_blocked)"
        fail "$st" 12 "$reason" "the conflicting files with their markers are under $CPA_RUN/conflicts/ (the run's commits stay local, pushed nowhere); resolve through --resolve-merge" ;;
    13) fail "$st" 13 secret_fold_refused "see integrate-merge-$st.out" ;;
    20) reason="$(helper_reason "$CPA_RUN/integrate-merge-$st.out" 'resolution_dir_invalid|merge_resolver_not_pinned|store_not_rerecorded|secret_fold_unavailable|backup_failed|merge_in_progress|unrecorded_local_commit|resolution_invalid|wrong_branch' merge_refused)"
        fail "$st" 20 "$reason" "$(tail -c 200 "$CPA_RUN/integrate-merge-$st.out" | tr '\n' ' ')" ;;
    *) fail "$st" 20 internal_error "integrate_merge.sh exit $rc" ;;
  esac
}
s1a_backup() { # objects only, the unrecorded-commit guard, and the 9.2 backup (work-tree copy and, for a non-empty local range, a verified bundle)
  local bk="$CPA_RUN/backup/s1a" r t ts=() c f id xy ol
  mkdir -p "$bk/worktree" || fail S1a 20 backup_failed "cannot create $bk"
  for r in $RL; do safe_remote "$r" || continue
    git -C "$TARGET" fetch -q --no-tags --no-write-fetch-head --refmap= --no-recurse-submodules -- "$r" "$BR" >>"$LOG" 2>&1 || echo "fetch_failed:$r" >> "$CPA_RUN/s1a-fetch.txt"
    t="$(timeout 60 git -C "$TARGET" ls-remote -- "$r" "refs/heads/$BR" 2>/dev/null | lr_exact "$BR")"
    if [ -n "$t" ] && safe_sha "$t" && git -C "$TARGET" cat-file -e "$t^{commit}" 2>/dev/null; then ts+=("$t"); fi
  done
  listing "$CPA_RUN/s1a-outgoing" "outgoing commit list" git -C "$TARGET" rev-list "refs/heads/$BR" ${ts[@]:+--not "${ts[@]}"} --
  while IFS= read -r c; do
    [ -n "$c" ] || continue
    id="$(cpa_runid "$TARGET" "$c")" && cpa_row "$AUDITD/$id/commits.tsv" "$c" && continue
    fail S1a 20 unrecorded_local_commit "$c: a commit that is not a CPA commit and that no remote holds; nothing moved or committed"
  done < "$CPA_RUN/s1a-outgoing"
  listing "$CPA_RUN/s1a-status.z" "git status" git -C "$TARGET" status --porcelain=v1 -z -uall --ignore-submodules=all
  : > "$bk/nested.tsv"
  while IFS= read -r -d '' f; do
    xy="${f:0:2}"; f="${f:3}"
    case "$xy" in R*|C*|?R|?C) IFS= read -r -d '' ol ;; esac        # a rename or copy record is followed by a bare record holding the OLD path (LIST-11)
    if [ -d "$TARGET/$f" ] && [ ! -L "$TARGET/$f" ]; then
      # an untracked nested repository is not copied (cp -p cannot copy a directory; LIST-9): its identity is recorded instead, so the backup states what it did not copy
      if [ -e "$TARGET/${f%/}/.git" ]; then
        printf 'nested_repo\t%s\t%s\t%s\n' "${f%/}" "$(git -C "$TARGET/${f%/}" rev-parse HEAD 2>/dev/null || echo none)" "$(git -C "$TARGET/${f%/}" status --porcelain=v1 -z 2>/dev/null | sha256sum | cut -d' ' -f1)" >> "$bk/nested.tsv"
        continue
      fi
      fail S1a 20 backup_failed "a directory entry that is no repository: $(printf '%q' "$f")"
    fi
    if [ -L "$TARGET/$f" ]; then       # a symlink (a dangling one included) is copied as a link and its target recorded; sha256sum would follow it
      mkdir -p "$bk/worktree/$(dirname "$f")" && cp -P -p -- "$TARGET/$f" "$bk/worktree/$f" || fail S1a 20 backup_failed "link copy of $(printf '%q' "$f")"
      printf 'symlink\t%s\t%s\n' "$f" "$(readlink -- "$TARGET/$f")" >> "$bk/nested.tsv"; continue
    fi
    [ -e "$TARGET/$f" ] || continue
    # shellcheck disable=SC2015  # A && B || fail: a failure of either is the refusal
    mkdir -p "$bk/worktree/$(dirname "$f")" && cp -p -- "$TARGET/$f" "$bk/worktree/$f" || fail S1a 20 backup_failed "work-tree copy of $(printf '%q' "$f")"
    ( cd "$bk/worktree" && sha256sum -- "$f" ) >> "$bk/worktree.sha256" || fail S1a 20 backup_failed "sha256 of $(printf '%q' "$f")"
  done < "$CPA_RUN/s1a-status.z"
  [ -f "$bk/worktree.sha256" ] || : > "$bk/worktree.sha256"
  if [ -s "$CPA_RUN/s1a-outgoing" ]; then
    # shellcheck disable=SC2015  # create && verify || fail: a failure of either is the refusal
    git -C "$TARGET" bundle create "$bk/local.bundle" "refs/heads/$BR" ${ts[@]:+--not "${ts[@]}"} >>"$LOG" 2>&1 && git -C "$TARGET" bundle verify "$bk/local.bundle" >>"$LOG" 2>&1 \
      || fail S1a 20 backup_failed "git bundle create or git bundle verify failed"
    jq -n --arg h "$START_HEAD" --arg b "$bk/local.bundle" '{head:$h,bundle:$b,reason:null,worktree_sha256:"worktree.sha256"}' > "$bk/backup.json"
  else
    jq -n --arg h "$START_HEAD" '{head:$h,bundle:null,reason:"local_range_empty",worktree_sha256:"worktree.sha256"}' > "$bk/backup.json"
  fi
}
if [ -n "$CBI" ]; then stage S1a fetch_objects; s1a_backup
else stage S1 fetch_integrate; s1_integrate S1 1; fi
[ -z "$RESOLVE" ] || [ -n "${MERGE_RAN:-}" ] || fail S1 20 usage_error "--resolve-merge given but no merge ran (the integration needed none)"

# ---- S2 scope check ----------------------------------------------------------------------------------------------------------------------------
stage S2 scope_check
# a declared path below a gitlink (held or not) is refused before any commit: it would commit inside the submodule and leave the parent stale
gitlinks_ok "$TARGET" || fail S2 20 git_listing_failed "gitlink listing"
while IFS= read -r -d '' gl; do
  while IFS= read -r p; do
    case "$p" in "$gl"/*) fail S2 20 path_in_submodule "$p lies below the gitlink $gl: commit it with CPA --repo $gl, then move the pin through G-PIN" ;; esac
  done < "$CPA_RUN/paths.list"
done < <(gitlinks "$TARGET")
# the JUDGED bytes are the COMMITTED bytes (TOCTOU-1..3): a private index is built from HEAD and the declared paths through git itself (the same filters, attributes and encoding as S5; only approved
# ones are left by the S0 configuration gate), the blob of every declared path is recorded in judged.tsv and materialised under judged/, and S2 and S3 read THAT tree. S5 commits only when
# the blobs it is about to commit are the recorded ones (commit_recursive.sh --expect-index).
JI="$CPA_RUN/judge.index"; JT="$CPA_RUN/judged"; : > "$CPA_RUN/judged.tsv"; mkdir -p "$JT"
if [ -s "$CPA_RUN/paths.list" ]; then
  if git -C "$TARGET" rev-parse -q --verify HEAD >/dev/null 2>&1; then GIT_INDEX_FILE="$JI" git -C "$TARGET" read-tree HEAD >>"$LOG" 2>&1; else GIT_INDEX_FILE="$JI" git -C "$TARGET" read-tree --empty >>"$LOG" 2>&1; fi \
    || fail S2 20 judged_index_failed "read-tree"
  while IFS= read -r p; do printf ':(literal)%s\0' "$p"; done < "$CPA_RUN/paths.list" > "$CPA_RUN/pathspec.z"
  GIT_INDEX_FILE="$JI" git -C "$TARGET" add -A -f --pathspec-from-file="$CPA_RUN/pathspec.z" --pathspec-file-nul >>"$LOG" 2>&1 || fail S2 20 judged_index_failed "git add of the declared paths: $(tail -c 200 "$LOG" | tr '\n' ' ')"
  xargs -0 -r -a "$CPA_RUN/pathspec.z" env GIT_INDEX_FILE="$JI" git -C "$TARGET" ls-files -s -z -- > "$CPA_RUN/judged.z" 2>>"$LOG" || fail S2 20 judged_index_failed "ls-files"
  : > "$CPA_RUN/judged.seen"
  while IFS= read -r -d '' e; do
    meta="${e%%$'\t'*}"; p="${e#*$'\t'}"; mode="${meta%% *}"; rest="${meta#* }"; sha="${rest%% *}"
    printf '%s\t%s\t%s\n' "$mode" "$sha" "$p" >> "$CPA_RUN/judged.tsv"; printf '%s\n' "$p" >> "$CPA_RUN/judged.seen"
    case "$mode" in
      100644|100755) mkdir -p "$JT/$(dirname "$p")" && git -C "$TARGET" cat-file blob "$sha" > "$JT/$p" 2>>"$LOG" || fail S2 20 judged_index_failed "blob of $(printf '%q' "$p")" ;;
      120000) mkdir -p "$JT/$(dirname "$p")" && ln -s -- "$(git -C "$TARGET" cat-file blob "$sha" 2>>"$LOG")" "$JT/$p" || fail S2 20 judged_index_failed "link of $(printf '%q' "$p")" ;;
      160000) : ;;
      *) fail S2 20 judged_index_failed "mode $mode of $(printf '%q' "$p")" ;;
    esac
  done < "$CPA_RUN/judged.z"
  # a declared path with no entry in the judged index is a deletion (or a path that git ignores): recorded as such, so S5 can hold the commit to it
  while IFS= read -r p; do grep -qxF -- "$p" "$CPA_RUN/judged.seen" || printf 'deleted\t-\t%s\n' "$p" >> "$CPA_RUN/judged.tsv"; done < "$CPA_RUN/paths.list"
fi
helper_run "$RD/scope_check.sh" --root "$TARGET" --content-root "$JT" --paths-from "$CPA_RUN/paths.list" >"$CPA_RUN/scope.out" 2>&1; rc=$?
case "$rc" in 0) ;; 13) fail S2 13 scope_refused "$(head -c 300 "$CPA_RUN/scope.out" | tr '\n' ' ')" ;; 20) fail S2 20 scope_error "$(head -c 300 "$CPA_RUN/scope.out" | tr '\n' ' ')" ;; *) fail S2 20 internal_error "scope_check.sh exit $rc" ;; esac

# ---- S3 cheap checks ---------------------------------------------------------------------------------------------------------------------------
stage S3 validate_cheap
# the registry that DECIDES which checks run is the APPROVED one: a row deleted or edited in the work tree is pending, never silently dropped (REPO-4, in validate_cheap.sh)
vc=(--root "$TARGET" --content-root "$JT" --verdict-root "$ROOT" --files-from "$CPA_RUN/paths.list" --code-root "$D/.." --trusted-tables "$RD" --out "$CPA_RUN/validate")
[ -f "$ROOT/scripts/repo/validate_checks.tsv" ] && vc+=(--registry "$ROOT/scripts/repo/validate_checks.tsv" --approved-registry "$RD/validate_checks.tsv")
[ -s "$CPA_RUN/held.tsv" ] && vc+=(--held-from "$CPA_RUN/held.tsv")
mkdir -p "$CPA_RUN/validate"
helper_run "$RD/validate_cheap.sh" "${vc[@]}" >"$CPA_RUN/validate.out" 2>&1; rc=$?
case "$rc" in
  0) ;;
  10) fail S3 10 check_failed "$(grep -E '^(fail|class_table_unreviewed|legacy_row_not_dropped)' "$CPA_RUN/validate.out" | head -3 | tr '\n\t' '  ')" ;;
  14) PENDING_CHECKS=1     # check_pending_release rows: a registry row the approved copy lacks is not run; recorded below as CHECK_PENDING_RELEASE
      pc="$(awk -F'\t' '$1=="check_pending_release" && NF>=2 {print $2}' "$CPA_RUN/validate.out" | sort -u | paste -sd, -)"
      defer CHECK_PENDING_RELEASE "registry rows the approved copy lacks, not run: ${pc:-see validate.out}" S3 ;;
  20) fail S3 20 check_error "$(head -c 300 "$CPA_RUN/validate.out" | tr '\n' ' ')" ;;
  *) fail S3 20 internal_error "validate_cheap.sh exit $rc" ;;
esac
# a declared path that no S3 check judged is reported by name, and it is a deferral when nothing at all judged it: a symlink of a non-evidence class, a deletion, a non-regular file, or a file the
# language check that its suffix selects left out; a file only the generic text checks left out (an image) is reported, owes nothing (WF14 N4). One row per (path, check): a path is
# `deferred` when ANY row of it is (REPORT-5).
: > "$CPA_RUN/unjudged.tsv"
awk -F'\t' 'NF>=2 && $1=="symlink_not_judged" {print $2 "\tsymlink\tyes"} NF>=3 && $1=="not_judged" {print $3 "\t" $2 "\t" ($2=="deletion" ? "no" : "yes")} NF>=3 && $1=="left_out" {print $3 "\t" $2 "\tno"}' "$CPA_RUN/validate.out" \
  | awk -F'\t' '{k=$1 "\t" $2; if (!(k in d)) {o[++n]=k}; if ($3=="yes") d[k]="yes"; else if (!(k in d)) d[k]="no"} END{for (i=1;i<=n;i++) print o[i] "\t" d[o[i]]}' | sort -u > "$CPA_RUN/unjudged.tsv"
nj="$(awk -F'\t' '$3=="yes" {print $1}' "$CPA_RUN/unjudged.tsv" | sort -u | paste -sd, -)"
[ -z "$nj" ] || defer CHECKS_NOT_JUDGED "declared paths no check judged: $nj" S3
# a registry row whose mode is `deferred` did not run either, and validate_cheap.sh still exits 0 for it: read from its stdout, never assumed (WF11 review F3)
dc="$(awk -F'\t' '$1=="deferred" && NF>=2 {print $2}' "$CPA_RUN/validate.out" | sort -u | paste -sd, -)"
[ -z "$dc" ] || defer CHECKS_DEFERRED "registry rows with mode deferred, not run: $dc" S3

# ---- S4 long gates -----------------------------------------------------------------------------------------------------------------------------
stage S4 validate_long
if [ -n "${SKIP_LONG:-}" ]; then log "S4 skipped: SKIP_LONG recorded at S0"
elif [ -f "$RD/long_gates.txt" ]; then
  # the list of the approved copy (comment and blank lines skipped) is the decision: an EMPTY list is an explicit "no long gates", an ABSENT one is not (below)
  lf=(); while IFS= read -r g || [ -n "$g" ]; do case "$g" in ''|'#'*) continue ;; esac; lf+=(--file "$ROOT/$g"); done < "$RD/long_gates.txt"
  if [ "${#lf[@]}" -gt 0 ]; then
    helper_run "$LO/require_verdicts.sh" "${lf[@]}" >"$CPA_RUN/long.out" 2>&1; rc=$?
    case "$rc" in 0) ;; 1) fail S4 10 long_gate_verdict_missing "$(head -c 300 "$CPA_RUN/long.out" | tr '\n' ' ')" ;; *) fail S4 20 internal_error "require_verdicts.sh exit $rc" ;; esac
  else log "S4: the approved long-gate list names no gate"; fi
else log "S4: no long-gate list in the approved copy (GATES_NOT_BUILT recorded at S0)"; fi

# ---- S5 commit ---------------------------------------------------------------------------------------------------------------------------------
stage S5 commit
commit_group() { # commit_group <paths file> [held.tsv]
  local pf="$1" hf="${2:-}" cr=(--repo "$TARGET" --run-dir "$CPA_RUN" --run-id "$RUN_ID" --message "$MSG" --repo-key "${REPO:-.}" --branch "$BR" --expect-index "$CPA_RUN/judged.tsv") rc
  [ -s "$CPA_RUN/foreign.txt" ] && cr+=(--foreign-from "$CPA_RUN/foreign.txt")
  [ -z "$hf" ] || cr+=(--held-from "$hf")
  mapfile -t _ps < "$pf"; printf '%s\0' "${_ps[@]}" > "$CPA_RUN/commit-paths.z"
  helper_run "$RD/commit_recursive.sh" "${cr[@]}" --paths-from0 "$CPA_RUN/commit-paths.z" >>"$CPA_RUN/commit.out" 2>&1; rc=$?
  case "$rc" in 0) ;; *)
    # a commit made but not recorded (a signal, a disk error between the two steps) is recorded HERE, so the next run does not stop on an unrecorded local commit of this very run (RES-3 f)
    hn="$(git -C "$TARGET" rev-parse HEAD 2>/dev/null)"
    if [ -n "$hn" ] && [ "$hn" != "$START_HEAD" ] && [ "$(cpa_runid "$TARGET" "$hn")" = "$RUN_ID" ] && ! cpa_row "$CPA_RUN/commits.tsv" "$hn" && ! grep -q 'changed_since_validation' "$CPA_RUN/commit.out"; then
      printf '%s\t%s\t%s\n' "${REPO:-.}" "$hn" "$RUN_ID" >> "$CPA_RUN/commits.tsv" && fail S5 20 commit_unrecorded_recovered "$hn: commit_recursive.sh exit $rc after the commit, before its row; the row was written here"
    fi
    fail S5 20 "$(helper_reason "$CPA_RUN/commit.out" 'changed_since_validation|branch_moved' commit_failed)" "commit_recursive.sh exit $rc: $(tail -c 300 "$CPA_RUN/commit.out" | tr '\n' ' ')" ;;
  esac
}
if [ -s "$CPA_RUN/paths.list" ]; then
  # a hold on a verdict that already holds GO in the main HEAD, or a change to such a verdict file, is refused before any commit (verdict_already_go); GO is the ONE predicate
  while IFS=$'\t' read -r hp hv; do
    [ -n "$hv" ] || continue
    [ "$(head_verdict_state "$hv")" = ok ] && fail S5 20 verdict_already_go "$hp is held on $hv, which holds GO in the main HEAD: a GO file is final; hold on a new review iteration"
    if [ "$(cat "$CPA_RUN/judged/$hv" 2>/dev/null | jq -r '.verdict // empty' 2>/dev/null)" = GO ] && grep -qxF -- "$hv" "$CPA_RUN/paths.list"; then
      fail S5 20 verdict_already_go "$hv is declared with GO in the same change set as $hp, which is held on it"; fi
  done < "$CPA_RUN/held.tsv"
  while IFS= read -r dp; do
    [ -z "$REPO" ] || continue
    case "$dp" in *.json) ;; *) continue ;; esac            # a review verdict is a JSON file: no other path is looked at
    [ "$(head_verdict_state "$dp")" = ok ] && fail S5 20 verdict_already_go "$dp holds GO in the main HEAD and is changed by this change set"
  done < "$CPA_RUN/paths.list"
  # unheld paths first, then one commit per verdict
  cut -f1 "$CPA_RUN/held.tsv" > "$CPA_RUN/paths.held"
  grep -vxFf "$CPA_RUN/paths.held" "$CPA_RUN/paths.list" > "$CPA_RUN/paths.unheld" || true     # an empty pattern file matches nothing: every path is unheld
  [ ! -s "$CPA_RUN/paths.unheld" ] || commit_group "$CPA_RUN/paths.unheld"
  if [ -s "$CPA_RUN/paths.held" ]; then
    # one LOCAL_ONLY row per distinct verdict (REPORT-6): the deferrals record every verdict a commit of this run waits for
    while IFS= read -r hv; do
      approved_run "$RD/record_deferral.sh" --run-dir "$CPA_RUN" --flag LOCAL_ONLY --reason "held on review: push owed until the verdict is GO" --awaits-review "$hv" >>"$LOG" 2>&1 \
        || fail S5 20 deferral_write_failed LOCAL_ONLY
    done < <(cut -f2 "$CPA_RUN/held.tsv" | sort -u)
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
  out="$(git -C "$d" rev-list "refs/heads/$BR" ${held[@]:+--not "${held[@]}"} -- 2>/dev/null)" || return 0
  [ -n "$out" ]
}
stage S6 push
if [ -z "$LOCAL_ONLY" ]; then
  pr=(--main-root "$ROOT" --branch "$BR" --run-dir "$CPA_RUN"); [ -z "$OWNED" ] || pr+=(--owned-orgs "$OWNED")
  if [ -n "$REPO" ]; then pr+=(--repo "$REPO")
  else
    pr+=(--repo .)
    # owned submodules that carry the run branch AND commits that some remote lacks, deepest first: a submodule without that local branch is left out,
    # and so is one that is merely behind its remote (needs_update, never pushed and never moved by a run of the main repository)
    list_submodules
    for sp in "${SUBS[@]+"${SUBS[@]}"}"; do
      is_owned "$ROOT/$sp" || continue
      git -C "$ROOT/$sp" rev-parse -q --verify "refs/heads/$BR" >/dev/null 2>&1 || { log "S6: $sp has no local branch $BR, not pushed"; continue; }
      cg="$(repo_config_gate "$ROOT/$sp" "$RD/repo_config_allow.tsv")" || fail S6 20 repo_config_not_approved "$cg"
      sub_outgoing "$ROOT/$sp" || { log "S6: $sp holds no commit that a remote lacks, no push call"; continue; }
      pr+=(--repo "$sp")
    done
  fi
  helper_run "$RD/push_recursive.sh" "${pr[@]}" >"$CPA_RUN/push.out" 2>"$CPA_RUN/push.err"; rc=$?
  mapfile -t HELDLINES < <(awk -F'\t' '$1=="HELD"{print $3 " " $4 " " $5}' "$CPA_RUN/push.out")
  mapfile -t HELDKEYS < <(awk -F'\t' '$1=="HELD"{print $2}' "$CPA_RUN/push.out" | sort -u)
  grep -E '^(HELD|NOPUSH|PUSH_FAILED|REFUSED)' "$CPA_RUN/push.out" | tr '\t' ' ' >&2
  case "$rc" in
    0) ;;
    14) HELD=1 ;;
    11) fail S6 11 "$(grep -oE 'remote_moved_since_s1|remote_unreachable|submodule_commit_not_held|PUSH_FAILED' "$CPA_RUN/push.out" | head -1)" "commits are safe locally; see push.out" ;;
    20) cat "$CPA_RUN/push.err" >> "$CPA_RUN/push.out" 2>/dev/null; fail S6 20 "$(helper_reason "$CPA_RUN/push.out" '' push_refused)" "see push.out" ;;
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
helper_run "$RD/verify_repos.sh" "${vf[@]}" >"$CPA_RUN/verify.out" 2>&1; vrc=$?
json_is '.summary | (.pin_drift|numbers) and (.ahead|numbers) and (.dirty|numbers)' "$CPA_RUN/verify.json" \
  || fail S7 20 verifier_report_unreadable "verifier exit $vrc; a missing or unreadable report is never read as clean"
# pointer drift: a row that matches .audit/pending_pins.tsv exactly (same path, head equal to the row's sha, drifted) AND whose sha DESCENDS from the pin the parent records is reported pending,
# never failing; a row that matches but is behind or unrelated to the recorded pin is stale and is dropped under the lock (STALE-1)
PP="$ROOT/.audit/pending_pins.tsv"; [ -f "$PP" ] || PP=/dev/null
pin_row_valid() { # pin_row_valid <path> <sha>: the pin the parent HEAD records for <path> is an ancestor of (or equal to) <sha>
  local rec; rec="$(git -C "$ROOT" ls-tree HEAD -- "$1" 2>/dev/null | awk '{print $3}')"
  [ -n "$rec" ] || return 1
  [ "$rec" = "$2" ] || git -C "$ROOT/$1" merge-base --is-ancestor "$rec" "$2" 2>/dev/null
}
jq -r '.repos[]|select(.pin_state!="ok")|[.path,(.head//""),.pin_state]|@tsv' "$CPA_RUN/verify.json" 2>/dev/null > "$CPA_RUN/drift.tsv"
: > "$CPA_RUN/pins.tsv"; : > "$CPA_RUN/pins.unmatched"; : > "$CPA_RUN/pins.stale"
while IFS=$'\t' read -r dpath dhead dstate; do
  [ -n "$dpath" ] || continue
  if [ "$dstate" = drifted ] && awk -F'\t' -v p="$dpath" -v h="$dhead" '$1==p && $2==h {f=1} END{exit !f}' "$PP"; then
    if pin_row_valid "$dpath" "$dhead"; then printf '%s\t%s\tpending\n' "$dpath" "$dhead" >> "$CPA_RUN/pins.tsv"
    else printf '%s\t%s\t%s\n' "$dpath" "$dhead" "$dstate" >> "$CPA_RUN/pins.unmatched"; printf '%s\t%s\n' "$dpath" "$dhead" >> "$CPA_RUN/pins.stale"; fi
  else printf '%s\t%s\t%s\n' "$dpath" "$dhead" "$dstate" >> "$CPA_RUN/pins.unmatched"; fi
done < "$CPA_RUN/drift.tsv"
if [ -s "$CPA_RUN/pins.stale" ] && [ "$PP" != /dev/null ]; then
  awk -F'\t' 'NR==FNR{s[$1 "\t" $2]=1; next} !(($1 "\t" $2) in s)' "$CPA_RUN/pins.stale" "$PP" > "$ROOT/.audit/pending_pins.tsv.$RUN_ID" && mv -f "$ROOT/.audit/pending_pins.tsv.$RUN_ID" "$PP" \
    && log "S7: dropped the stale pending pin row(s): $(tr '\t\n' '  ' < "$CPA_RUN/pins.stale")" || fail S7 20 pending_pin_write_failed "$PP"
fi
jq -n --rawfile p "$CPA_RUN/pins.tsv" --rawfile u "$CPA_RUN/pins.unmatched" '{pending:($p|split("\n")|map(select(length>0)|split("\t")|{path:.[0],head:.[1]})),unmatched:($u|split("\n")|map(select(length>0)|split("\t")|{path:.[0],head:.[1],state:.[2]}))}' > "$CPA_RUN/pins.json" 2>/dev/null
held_explains_verifier() { # the held remainder explains verifier 11 only when NOTHING is unproven and every repository that is ahead holds a held commit (REPORT-4)
  [ "$HELD" = 1 ] || return 1
  [ "$(jq -r '.summary.unproven // 1' "$CPA_RUN/verify.json" 2>/dev/null)" = 0 ] || return 1
  local ap k found
  while IFS= read -r ap; do
    [ -n "$ap" ] || continue; found=0
    # in a --repo run the verifier's root is the submodule, so its own row is "." and the held key is the repository path
    if [ -n "$REPO" ]; then if [ "$ap" = . ]; then ap="$REPO"; else ap="$REPO/${ap#./}"; fi; fi
    for k in "${HELDKEYS[@]+"${HELDKEYS[@]}"}"; do [ "$k" = "$ap" ] && found=1; done
    [ "$found" = 1 ] || return 1
  done < <(jq -r '.repos[]|select(.problems|index("ahead"))|.path' "$CPA_RUN/verify.json" 2>/dev/null)
  return 0
}
case "$vrc" in
  0) ;;
  11) if held_explains_verifier; then log "S7: verifier 11 explained by the held remainder (review_pending)"; else fail S7 11 "$([ "$HELD" = 1 ] && echo remote_unproven || echo commits_not_on_a_remote)" "after S6; see verify.json"; fi ;;
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
  sweep_run --stage S7 ${REPO:+--repo "$REPO"} >"$CPA_RUN/sweep-s7.out" 2>&1; src=$?
  case "$src" in 0) ;; *) fail S7 20 sweep_finding "after verify (rc=$src)" ;; esac
fi

# ---- S8 happens in the EXIT trap: the report is written on every path that a handler can reach ------------------------------------------------------
stage S8 report
if [ "${#DEFERRED[@]}" -gt 0 ] || [ "$HELD" = 1 ] || [ "$PENDING_CHECKS" = 1 ]; then
  if [ "$HELD" = 1 ]; then FAILREASON="review_pending"; FAILDETAIL="${HELDLINES[*]:-}"; else FAILREASON="deferral_recorded"; FAILDETAIL="${DEFERRED[*]:-}"; fi
  exit 14
fi
exit 0
}
cpa_main "$@"; exit $?
