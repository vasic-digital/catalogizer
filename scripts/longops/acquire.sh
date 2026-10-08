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
# Every numeric option and enumerated state is validated BEFORE any write; --pid is an integer > 1 naming a process that exists now (WF11 F5/F6).
# Every mode reads the holder through ONE reader (A-S3): a holder that is absent is a cas_mismatch (4); a holder that EXISTS but is empty, unreadable, a directory or fails the holder shape is `holder_unreadable`
#         (20), never a cas_mismatch, never expired (LO-A5, LO-A6).
# Locking every transition is a compare-and-swap on the expected prior holder (run id and state) under ONE flock on <purpose>.lock;
#         the holder record is replaced by temp-then-rename, so a reader never sees `none` across a suspend, update or adopt.
#         An --adopt racing --expire has exactly one winner: the loser sees the other's result as a cas_mismatch (4).
# --expire record: `resume_expired.json` naming the expired run goes into $CPA_RUN when CPA_RUN_ID and CPA_RUN are set (the calling run's report),
#         otherwise under .audit/out/<op_id>/ (--op-id required, else 20 usage_error), never into the expired run's directory, never under .audit/commit-push/.
# CENTRAL C2   purpose commit_push: CPA_APPROVED_DIR unset gives 20 helper_not_approved FIRST, before the --op-id usage check (--expire).
# Exits   0; 1 a registry write failed; 2 usage; 3 purpose_conflict; 4 cas_mismatch / stale / not expired; 20 refusal (helper_not_approved, usage_error under --expire, holder_unreadable); 70 lock wait.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
mode=claim; purpose=""; run=""; pid=$PPID; builds=""; ttl=""; cbs=""; nst=""; opid=""
while [ $# -gt 0 ]; do
  case "$1" in
    --purpose) lo_need "$@"; purpose=$2; shift 2 ;; --run-id) lo_need "$@"; run=$2; shift 2 ;; --pid) lo_need "$@"; pid=$2; shift 2 ;; --builds) lo_need "$@"; builds=$2; shift 2 ;;
    --resume-ttl) lo_need "$@"; ttl=$2; shift 2 ;; --callback-state) lo_need "$@"; cbs=$2; shift 2 ;; --state) lo_need "$@"; nst=$2; shift 2 ;; --op-id) lo_need "$@"; opid=$2; shift 2 ;;
    --suspend) lo_need "$@"; mode=suspend; run=$2; shift 2 ;; --adopt) lo_need "$@"; mode=adopt; run=$2; shift 2 ;; --update) lo_need "$@"; mode=update; run=$2; shift 2 ;;
    --expire) lo_need "$@"; mode=expire; purpose=$2; shift 2 ;;
    *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;;
  esac
done
[ -n "$purpose" ] || purpose=commit_push
lo_safe_name "$purpose" || lo_die usage_error "unsafe purpose"
lo_pid_ok "$pid" || lo_die usage_error "--pid must be an integer > 1 naming a process that exists now"
[ -z "$ttl" ] || lo_uint "$ttl" || lo_die usage_error "--resume-ttl must be a non-negative integer of at most 15 digits"
case "$cbs" in ""|none|claimed|running|done) ;; *) lo_die usage_error "--callback-state is none|claimed|running|done" ;; esac
case "$nst" in ""|suspended|ready_to_resume) ;; *) lo_die usage_error "--state is suspended|ready_to_resume" ;; esac
[ "$mode" != expire ] || lo_require_approved "$purpose"           # CENTRAL C2: checked first
[ "$mode" = claim ] || [ "$mode" = expire ] || lo_require_approved "$purpose"
if [ "$mode" != expire ]; then [ -n "$run" ] || lo_die usage_error "a run id is required"; [[ "$run" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] || lo_die usage_error "unsafe run id"; fi
lo_init
# _hget: sets H to the holder text of the purpose. 4 (cas_mismatch) when there is NO holder; 20 (holder_unreadable) when the file exists but cannot be read or fails LO_HOLDER_SHAPE.
_hget() {
  H=$(_lo_read_holder "$purpose"); local rr=$?
  case $rr in
    1) echo "cas_mismatch: no holder for $purpose" >&2; return "$RC_CAS" ;;
    2) echo "holder_unreadable: the holder record of $purpose exists but cannot be read" >&2; return "$RC_REFUSE" ;;
  esac
  jq -e "$LO_HOLDER_SHAPE" >/dev/null 2>&1 <<<"$H" || { echo "holder_unreadable: the holder record of $purpose fails the holder shape (docs/scripts/longops.md FIELDS)" >&2; return "$RC_REFUSE"; }
}
_expect() {  # _expect <state>: the holder must be this run in this state, else cas_mismatch
  _hget || return $?
  [ "$(jq -r .run_id <<<"$H")" = "$run" ] && [ "$(jq -r .state <<<"$H")" = "$1" ] || { echo "cas_mismatch: holder is $(jq -c '{run_id,state,kind}' <<<"$H"), expected run=$run state=$1" >&2; return "$RC_CAS"; }
}
_swap() { lo_wjson "$(lo_holder_file "$purpose")" "$1"; }
case "$mode" in
  claim) lo_with_lock "$purpose" lo_claim "$purpose" "$(lo_holder_json "$purpose" "$run" "$pid")"; rc=$?; [ $rc -eq 0 ] && lo_event acquired --arg purpose "$purpose" --arg run "$run"; exit $rc ;;
  suspend)
    [ -n "$builds" ] || lo_die usage_error "--suspend needs --builds"
    _s() { _expect running || return $?; lo_cs_pause
      _swap "$(jq -c --arg b "$builds" --arg t "$ttl" --argjson now "$(lo_now)" '.kind="suspended-run"|.state="suspended"|.builds=($b|split(",")|map(select(length>0)))|.callback_state="none"|.suspended_at=$now|(if $t!="" then .resume_ttl=($t|tonumber) else . end)' <<<"$H")"; }
    lo_with_lock "$purpose" _s ;;
  update)
    _u() { _hget || return $?
      [ "$(jq -r .run_id <<<"$H")" = "$run" ] || { echo "cas_mismatch: holder run differs from $run" >&2; return "$RC_CAS"; }
      [ "$(jq -r .kind <<<"$H")" = suspended-run ] || { echo "cas_mismatch: holder is not a suspended-run" >&2; return "$RC_CAS"; }
      lo_cs_pause
      _swap "$(jq -c --arg c "$cbs" --arg s "$nst" --argjson now "$(lo_now)" '(if $c!="" then .callback_state=$c else . end)|(if $s!="" then .state=$s else . end)|(if $s=="ready_to_resume" then .ready_at=$now else . end)' <<<"$H")"; }
    lo_with_lock "$purpose" _u ;;
  adopt)
    _a() { _hget || return $?
      [ "$(jq -r .run_id <<<"$H")" = "$run" ] && [ "$(jq -r .kind <<<"$H")" = suspended-run ] || { echo "cas_mismatch: no suspended-run holder for $run" >&2; return "$RC_CAS"; }
      lo_cs_pause
      _swap "$(lo_holder_json "$purpose" "$run" "$pid")"; }
    lo_with_lock "$purpose" _a ;;
  expire)
    if [ -z "${CPA_RUN_ID:-}" ] || [ -z "${CPA_RUN:-}" ]; then [ -n "$opid" ] && lo_safe_name "$opid" || lo_die usage_error "--expire outside a CPA run needs --op-id" "$RC_REFUSE"; fi
    _e() { local rdy ttl2; _hget || return $?
      [ "$(jq -r .state <<<"$H")" = ready_to_resume ] || { echo "not_expired: holder state is $(jq -r .state <<<"$H")" >&2; return "$RC_CAS"; }
      rdy=$(jq -r '.ready_at' <<<"$H"); lo_uint "$rdy" || { echo "holder_unreadable: ready_at of the holder is not a number" >&2; return "$RC_REFUSE"; }
      ttl2=$(lo_resume_ttl "$purpose" "$H") || return "$RC_REFUSE"
      [ "$(lo_now)" -ge $((rdy + ttl2)) ] || { echo "not_expired: ready_to_resume window has $((rdy + ttl2 - $(lo_now)))s left" >&2; return "$RC_CAS"; }
      lo_cs_pause
      rm -rf -- "$LD/claims/$purpose"; lo_fsync_dir "$LD/claims"
      local d; if [ -n "${CPA_RUN_ID:-}" ] && [ -n "${CPA_RUN:-}" ]; then d=$CPA_RUN; else d=$AUDIT_DIR/out/$opid; fi
      mkdir -p "$d" && lo_wjson "$d/resume_expired.json" "$(jq -c --arg p "$purpose" --argjson ttl "$ttl2" --argjson now "$(lo_now)" '{record:"resume_expired",purpose:$p,expired_run:.run_id,ready_at:.ready_at,resume_ttl:$ttl,expired_at:$now}' <<<"$H")"; }
    lo_with_lock "$purpose" _e ;;
esac
rc=$?; [ $rc -eq 0 ] && lo_event "$mode" --arg purpose "$purpose" --arg run "${run:-}"
exit $rc
