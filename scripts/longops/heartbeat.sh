#!/usr/bin/env bash
# heartbeat.sh - record liveness of a registered operation (11.4.232 C; docs/16 13.2 rule C).
#
# Usage   heartbeat.sh --op-id <id> [--progress-offset <n> | --sample-log] [--pid <pid>] [--elapsed-ms <n>]
#         --elapsed-ms  the op's own elapsed MONOTONIC time (a dispatched build reports its build host's clock): classify.sh marks the op hung once it passes budget.wall_clock_s (T089a)
# Progress advances only when the offset GROWS (a log byte offset: --sample-log reads the size of the op's log file) or, with neither
#         option, when the operation writes its own heartbeat (the sequence number is the monotone proof). A repeated flat offset is a
#         heartbeat that proves nothing: `classify.sh` reports it hung once no_progress_s passes. The first call moves registered to running.
# Exits   0 recorded; 2 usage; 4 the op is terminal or unknown.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
opid=""; off=""; sample=0; pid=""; elapsed=""
while [ $# -gt 0 ]; do
  case "$1" in --op-id) opid=${2:-}; shift 2 ;; --progress-offset) off=${2:-}; shift 2 ;; --sample-log) sample=1; shift ;; --pid) pid=${2:-}; shift 2 ;; --elapsed-ms) elapsed=${2:-}; shift 2 ;;
    *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac
done
[ -n "$opid" ] && lo_safe_name "$opid" || lo_die usage_error "--op-id required"
f=$(lo_op_file "$opid"); [ -s "$f" ] || lo_die unknown_op "$opid" "$RC_CAS"
purpose=$(jq -r .purpose_key "$f")
_hb() {
  local j now cur new seq prog
  j=$(cat "$f"); case "$(jq -r .state <<<"$j")" in complete|failed|reaped|handoff|blocked-escape) echo "op $opid is terminal" >&2; return "$RC_CAS" ;; esac
  now=$(lo_now); cur=$(jq -r .progress_offset <<<"$j")
  if [ "$sample" = 1 ]; then off=$(stat -c %s -- "$(jq -r .log_path <<<"$j")" 2>/dev/null || echo "$cur"); fi
  prog=0
  [ -z "$elapsed" ] || [[ "$elapsed" =~ ^[0-9]+$ ]] || return "$RC_USAGE"
  if [ -n "$off" ]; then [[ "$off" =~ ^[0-9]+$ ]] || return "$RC_USAGE"; [ "$off" -gt "$cur" ] && prog=1; new=$off; else prog=1; new=$cur; fi
  j=$(jq -c --argjson now "$now" --arg u "$(lo_utc "$now")" --argjson off "$new" --argjson prog "$prog" --arg pid "$pid" --arg st "${pid:+$(lo_pstart "$pid")}" --arg el "$elapsed" '
      .heartbeat_seq+=1 | .last_heartbeat_utc=$u | .progress_offset=$off | (if $prog==1 then .last_progress_epoch=$now else . end)
      | (if .state=="registered" then .state="running" else . end) | (if $pid!="" then .pid=($pid|tonumber)|.start_time=$st else . end) | (if $el!="" then .elapsed_ms=($el|tonumber) else . end)' <<<"$j")
  lo_wjson "$f" "$j"
}
lo_with_lock "$purpose" _hb; rc=$?
[ $rc -eq 0 ] && lo_event heartbeat --arg op "$opid"
exit $rc
