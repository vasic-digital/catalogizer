#!/usr/bin/env bash
# acquire.sh - the purpose lock and every transition of its holder record (T088/T089; 11.4.232 B; CENTRAL C2).
#
# Usage   acquire.sh --purpose <key> --run-id <id> [--pid <pid>]          claim a purpose (atomic mkdir); the record carries the holder's
#                                                                         run id, process id and process start time
#         acquire.sh --suspend <run_id> --builds <id,id,..> [--purpose commit_push] [--resume-ttl <s>] [--pid <pid>]
#                                                                         swap process -> suspended-run (the CPA process exits 16, its builds live)
#         acquire.sh --update <run_id> [--purpose commit_push] [--callback-state none|claimed|running|done] [--state suspended|ready_to_resume]
#                                                                         the hub's transition: `--callback-state done --state ready_to_resume` in one swap
#         acquire.sh --adopt <run_id> [--purpose commit_push] [--pid <pid>]   swap suspended-run -> process (the resumed run's pid, start time)
#         acquire.sh --expire <purpose> [--op-id <id>]                    release a holder in ready_to_resume past resume_ttl (compare-and-swap only;
#                                                                         with --adopt this pair is the whole cancel of a suspended run, CENTRAL C2)
# Locking every transition is a compare-and-swap on the expected prior holder (run id and state) under ONE flock on <purpose>.lock;
#         the holder record is replaced by temp-then-rename, so a reader never sees `none` across a suspend, update or adopt.
#         An --adopt racing --expire has exactly one winner: the loser sees the other's result as a cas_mismatch (4).
# --expire record: `resume_expired.json` naming the expired run goes into $CPA_RUN when CPA_RUN_ID and CPA_RUN are set (the calling run's report),
#         otherwise under .audit/out/<op_id>/ (--op-id required, else 20 usage_error), never into the expired run's directory, never under .audit/commit-push/.
# CENTRAL C2   purpose commit_push: CPA_APPROVED_DIR unset gives 20 helper_not_approved FIRST, before the --op-id usage check (--expire).
# Exits   0; 2 usage; 3 purpose_conflict; 4 cas_mismatch / stale / not expired; 20 refusal (helper_not_approved, usage_error under --expire).
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
mode=claim; purpose=""; run=""; pid=$PPID; builds=""; ttl=""; cbs=""; nst=""; opid=""
while [ $# -gt 0 ]; do
  case "$1" in
    --purpose) purpose=${2:-}; shift 2 ;; --run-id) run=${2:-}; shift 2 ;; --pid) pid=${2:-}; shift 2 ;; --builds) builds=${2:-}; shift 2 ;;
    --resume-ttl) ttl=${2:-}; shift 2 ;; --callback-state) cbs=${2:-}; shift 2 ;; --state) nst=${2:-}; shift 2 ;; --op-id) opid=${2:-}; shift 2 ;;
    --suspend) mode=suspend; run=${2:-}; shift 2 ;; --adopt) mode=adopt; run=${2:-}; shift 2 ;; --update) mode=update; run=${2:-}; shift 2 ;;
    --expire) mode=expire; purpose=${2:-}; shift 2 ;;
    *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;;
  esac
done
[ -n "$purpose" ] || purpose=commit_push
lo_safe_name "$purpose" || lo_die usage_error "unsafe purpose"
[[ "$pid" =~ ^[0-9]+$ ]] || lo_die usage_error "--pid must be an integer"
[ "$mode" != expire ] || lo_require_approved "$purpose"           # CENTRAL C2: checked first
[ "$mode" = claim ] || [ "$mode" = expire ] || lo_require_approved "$purpose"
if [ "$mode" != expire ]; then [ -n "$run" ] || lo_die usage_error "a run id is required"; [[ "$run" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] || lo_die usage_error "unsafe run id"; fi
lo_init
_holder() { cat "$(lo_holder_file "$purpose")" 2>/dev/null; }
_expect() {  # _expect <state>: the holder must be this run in this state, else cas_mismatch
  local h; h=$(_holder) || true
  [ -n "$h" ] || { echo "cas_mismatch: no holder for $purpose" >&2; return "$RC_CAS"; }
  [ "$(jq -r .run_id <<<"$h")" = "$run" ] && [ "$(jq -r .state <<<"$h")" = "$1" ] || { echo "cas_mismatch: holder is $(jq -c '{run_id,state,kind}' <<<"$h"), expected run=$run state=$1" >&2; return "$RC_CAS"; }
}
_swap() { lo_wjson "$(lo_holder_file "$purpose")" "$1"; }
case "$mode" in
  claim) lo_with_lock "$purpose" lo_claim "$purpose" "$(lo_holder_json "$purpose" "$run" "$pid")"; rc=$?; [ $rc -eq 0 ] && lo_event acquired --arg purpose "$purpose" --arg run "$run"; exit $rc ;;
  suspend)
    [ -n "$builds" ] || lo_die usage_error "--suspend needs --builds"
    _s() { _expect running || return $?; lo_cs_pause
      _swap "$(jq -c --arg b "$builds" --arg t "$ttl" --argjson now "$(lo_now)" '.kind="suspended-run"|.state="suspended"|.builds=($b|split(",")|map(select(length>0)))|.callback_state="none"|.suspended_at=$now|(if $t!="" then .resume_ttl=($t|tonumber) else . end)' <<<"$(_holder)")"; }
    lo_with_lock "$purpose" _s ;;
  update)
    _u() { local h; h=$(_holder) || true; [ -n "$h" ] && [ "$(jq -r .run_id <<<"$h")" = "$run" ] || { echo "cas_mismatch: holder run differs from $run" >&2; return "$RC_CAS"; }
      [ "$(jq -r .kind <<<"$h")" = suspended-run ] || { echo "cas_mismatch: holder is not a suspended-run" >&2; return "$RC_CAS"; }
      lo_cs_pause
      _swap "$(jq -c --arg c "$cbs" --arg s "$nst" --argjson now "$(lo_now)" '(if $c!="" then .callback_state=$c else . end)|(if $s!="" then .state=$s else . end)|(if $s=="ready_to_resume" then .ready_at=$now else . end)' <<<"$h")"; }
    lo_with_lock "$purpose" _u ;;
  adopt)
    _a() { local h; h=$(_holder) || true; [ -n "$h" ] && [ "$(jq -r .run_id <<<"$h")" = "$run" ] && [ "$(jq -r .kind <<<"$h")" = suspended-run ] || { echo "cas_mismatch: no suspended-run holder for $run" >&2; return "$RC_CAS"; }
      lo_cs_pause
      _swap "$(lo_holder_json "$purpose" "$run" "$pid")"; }
    lo_with_lock "$purpose" _a ;;
  expire)
    if [ -z "${CPA_RUN_ID:-}" ] || [ -z "${CPA_RUN:-}" ]; then [ -n "$opid" ] && lo_safe_name "$opid" || lo_die usage_error "--expire outside a CPA run needs --op-id" "$RC_REFUSE"; fi
    _e() { local h rdy ttl2; h=$(_holder) || true; [ -n "$h" ] || { echo "cas_mismatch: no holder for $purpose" >&2; return "$RC_CAS"; }
      [ "$(jq -r .state <<<"$h")" = ready_to_resume ] || { echo "not_expired: holder state is $(jq -r .state <<<"$h")" >&2; return "$RC_CAS"; }
      rdy=$(jq -r '.ready_at // 0' <<<"$h"); ttl2=$(lo_resume_ttl "$purpose" "$h") || return "$RC_REFUSE"
      [ "$(lo_now)" -ge $((rdy + ttl2)) ] || { echo "not_expired: ready_to_resume window has $((rdy + ttl2 - $(lo_now)))s left" >&2; return "$RC_CAS"; }
      lo_cs_pause
      rm -rf -- "$LD/claims/$purpose"
      local d; if [ -n "${CPA_RUN_ID:-}" ] && [ -n "${CPA_RUN:-}" ]; then d=$CPA_RUN; else d=$AUDIT_DIR/out/$opid; fi
      mkdir -p "$d" && lo_wjson "$d/resume_expired.json" "$(jq -c --arg p "$purpose" --argjson ttl "$ttl2" --argjson now "$(lo_now)" '{record:"resume_expired",purpose:$p,expired_run:.run_id,ready_at:.ready_at,resume_ttl:$ttl,expired_at:$now}' <<<"$h")"; }
    lo_with_lock "$purpose" _e ;;
esac
rc=$?; [ $rc -eq 0 ] && lo_event "$mode" --arg purpose "$purpose" --arg run "${run:-}"
exit $rc
