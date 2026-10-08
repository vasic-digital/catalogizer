#!/usr/bin/env bash
# heartbeat.sh - record liveness of a registered operation (11.4.232 C; docs/16 13.2 rule C).
#
# Usage   heartbeat.sh --op-id <id> [--progress-offset <n> | --sample-log] [--pid <pid>] [--elapsed-ms <n>]
#         --elapsed-ms  the op's own elapsed time on ITS clock (a dispatched build reports its build host's clock): classify.sh marks the op hung once it passes budget.wall_clock_s (T089a)
# Progress advances only when the offset GROWS (a log byte offset: --sample-log reads the size of the op's log file) or, with neither
#         option, when the operation writes its own heartbeat (the sequence number is the monotone proof). A repeated flat offset is a
#         heartbeat that proves nothing: `classify.sh` reports it hung once no_progress_s passes. The first call moves registered to running.
#         Progress is recorded on BOTH clocks (last_progress_epoch for people, last_progress_mono from /proc/uptime for the judgement, LO-D5).
#         --pid        rebinds the op's owner identity (pid, /proc start time, boot id AND cmdline, recorded together) AND the identity of the purpose claim when that claim is this op's own: an integer > 1 naming a
#                      live process (not a zombie, not a kernel thread) that exists now. The OP is written FIRST and the holder second (LO-F2): a failed holder write leaves a live op beside a stale holder, which the
#                      live-op guard of `reap.sh --purpose` protects; the script then exits 1 (the rebind did not complete) and a retry converges.
#         Every numeric option is validated BEFORE any write (a non-number never reaches a record, WF11 F5/F6/F13): 2 usage otherwise.
# Exits   0 recorded; 1 a write failed / the rebind target vanished; 2 usage; 4 the op is terminal or unknown; 20 the op record is unreadable (fails the record shape) or a numeric field of it is not a number; 70 lock wait.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
opid=""; off=""; sample=0; pid=""; elapsed=""
while [ $# -gt 0 ]; do
  case "$1" in --op-id) lo_need "$@"; opid=$2; shift 2 ;; --progress-offset) lo_need "$@"; off=$2; shift 2 ;; --sample-log) sample=1; shift ;; --pid) lo_need "$@"; pid=$2; shift 2 ;; --elapsed-ms) lo_need "$@"; elapsed=$2; shift 2 ;;
    *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac
done
[ -n "$opid" ] && lo_safe_name "$opid" || lo_die usage_error "--op-id required"
[ -z "$pid" ] || lo_pid_ok "$pid" || lo_die usage_error "--pid must be an integer > 1 naming a process that exists now"
[ -z "$elapsed" ] || lo_uint "$elapsed" || lo_die usage_error "--elapsed-ms must be a canonical non-negative integer of at most 15 digits"
[ -z "$off" ] || lo_uint "$off" || lo_die usage_error "--progress-offset must be a canonical non-negative integer of at most 15 digits"
lo_load_op "$opid"
_hb() {
  local j now mono cur new prog pst pbid
  j=$(cat "$f"); case "$(jq -r .state <<<"$j")" in complete|failed|reaped|handoff|blocked-escape) echo "op $opid is terminal" >&2; return "$RC_CAS" ;; esac
  now=$(lo_now); mono=$(lo_mono); cur=$(jq -r .progress_offset <<<"$j"); lo_uint "$cur" || { echo "op $opid: progress_offset of the record is not a number" >&2; return "$RC_REFUSE"; }
  if [ "$sample" = 1 ]; then off=$(stat -c %s -- "$(jq -r .log_path <<<"$j")" 2>/dev/null || echo "$cur"); fi
  prog=0
  if [ -n "$off" ]; then [ "$off" -gt "$cur" ] && prog=1; new=$off; else prog=1; new=$cur; fi
  pst=""; pbid=""
  if [ -n "$pid" ]; then
    pst=$(lo_pstart "$pid"); pbid=$(lo_boot_id)
    { [ -n "$pst" ] && [ -n "$pbid" ]; } || { echo "op $opid: pid $pid vanished before the rebind (pid_vanished); nothing was written" >&2; return 1; }
  fi
  j=$(jq -c --argjson now "$now" --argjson mono "$mono" --arg u "$(lo_utc "$now")" --argjson off "$new" --argjson prog "$prog" --arg pid "$pid" --arg st "$pst" --arg bid "$pbid" --arg cmd "${pid:+$(lo_cmdline "$pid")}" --arg el "$elapsed" '
      .heartbeat_seq+=1 | .last_heartbeat_utc=$u | .progress_offset=$off | (if $prog==1 then .last_progress_epoch=$now | .last_progress_mono=$mono else . end)
      | (if .state=="registered" then .state="running" else . end) | (if $pid!="" then .pid=($pid|tonumber)|.start_time=$st|.boot_id=$bid|.cmdline=$cmd else . end) | (if $el!="" then .elapsed_ms=($el|tonumber) else . end)' <<<"$j")
  # WF14 R2-3: a rebind moves the CLAIM HOLDER with the op owner (a launcher that hands over to a worker and exits must not leave a dead holder beside a live op: reap.sh --purpose would free the claim
  # of a live op and a second owner could register). Only a holder that is THIS op's own claim (run id equal) is rewritten. The op record is written FIRST (LO-F2): the holder write is the second step.
  lo_wjson "$f" "$j" || return 1
  if [ -n "$pid" ]; then
    local hf h; hf=$(lo_holder_file "$purpose")
    if [ -r "$hf" ]; then
      h=$(cat "$hf" 2>/dev/null)
      if [ "$(jq -r '.run_id // ""' <<<"$h" 2>/dev/null)" = "$(jq -r .run_id <<<"$j")" ]; then
        h=$(jq -c --argjson pid "$pid" --arg st "$pst" --arg bid "$pbid" --arg c "$(lo_cmdline "$pid")" '.pid=$pid|.start_time=$st|.boot_id=$bid|.cmdline=$c' <<<"$h") && lo_wjson "$hf" "$h" || { echo "op $opid: the op owner was rebound but the purpose holder could not be rewritten (holder_write_failed); retry the rebind" >&2; return 1; }
      fi
    fi
  fi
}
lo_with_lock "$purpose" _hb; rc=$?
[ $rc -eq 0 ] && lo_event heartbeat --arg op "$opid"
exit $rc
