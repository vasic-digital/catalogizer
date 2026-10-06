#!/usr/bin/env bash
# reap.sh - reap a long operation ONLY on proven staleness (11.4.232 E; 11.4.196 D; 11.4.263).
#
# Usage   reap.sh --op-id <id> [--dry-run]
#         reap.sh --purpose <key> [--dry-run]     release the claim of a dead holder with no op record (stale_claim)
# Rules   dead_owner  the recorded pid no longer matches /proc (start time): no signal at all, the record becomes `reaped`, the claim is released.
#         hung        the owner lives, its identity is re-resolved from /proc/<pid>/cmdline (must equal the recorded cmdline) and the
#                     op's labelled container is resolved with `podman ps --filter label=op_id=<id>`; then SIGTERM goes to that ONE pid
#                     through lo_signal, which refuses pid or pgid <= 1 and never signals a process group. A bare `pgrep` is never used.
#         advancing   refused (exit 5): a live advancing op is never reaped.
#         identity unresolvable (cmdline differs) refused, exit 6: conservative-safe (11.4.201 4), the evidence is printed.
# Exits   0 reaped (or would be, with --dry-run); 5 live advancing; 6 identity mismatch; 7 unsafe signal target; 4 cas; 2 usage.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
opid=""; purpose=""; dry=0
while [ $# -gt 0 ]; do case "$1" in --op-id) opid=${2:-}; shift 2 ;; --purpose) purpose=${2:-}; shift 2 ;; --dry-run) dry=1; shift ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
if [ -n "$purpose" ]; then
  lo_safe_name "$purpose" || lo_die usage_error "unsafe purpose"
  s=$(lo_holder_status "$purpose") || s=nohold
  case "$s" in live|expired) echo "refused: holder of $purpose is $s" >&2; exit "$RC_LIVE" ;; esac
  [ "$dry" = 1 ] && { echo "would release stale claim $purpose ($s)"; exit 0; }
  _rc() { rm -rf -- "$LD/claims/$purpose"; }; lo_with_lock "$purpose" _rc; lo_event reaped_claim --arg purpose "$purpose" --arg was "$s"; echo "released stale claim $purpose ($s)"; exit 0
fi
[ -n "$opid" ] && lo_safe_name "$opid" || lo_die usage_error "--op-id or --purpose required"
f=$(lo_op_file "$opid"); [ -s "$f" ] || lo_die unknown_op "$opid" "$RC_CAS"
j=$(cat "$f"); r=$(lo_classify_op "$j"); cls=$(sed -n 1p <<<"$r"); ev=$(sed -n 2p <<<"$r")
purpose=$(jq -r .purpose_key <<<"$j"); pid=$(jq -r .pid <<<"$j")
case "$cls" in
  terminal) echo "nothing to reap: $opid is already terminal"; exit 0 ;;
  advancing) echo "refused: $opid is live and advancing ($ev)" >&2; exit "$RC_LIVE" ;;
  hung)
    want=$(jq -r .cmdline <<<"$j"); have=$(lo_cmdline "$pid")
    if [ "$want" != "$have" ]; then echo "refused: identity of pid $pid unresolved: recorded '$want' but /proc says '$have'" >&2; exit "$RC_IDENT"; fi
    cid=$("$PODMAN" ps --filter "label=op_id=$opid" --format '{{.ID}}' 2>/dev/null | head -1)
    [ "$dry" = 1 ] && { echo "would reap hung $opid pid=$pid cmdline='$have' container='${cid:-none}' ($ev)"; exit 0; }
    lo_signal TERM "$pid"; rc=$?; [ $rc -eq 0 ] || { echo "refused: unsafe or failed signal target pid=$pid (rc $rc)" >&2; [ $rc -eq "$RC_UNSAFE" ] && exit "$RC_UNSAFE"; }
    [ -z "$cid" ] || "$PODMAN" stop -t 5 "$cid" >/dev/null 2>&1
    for _ in 1 2 3 4 5 6 7 8 9 10; do lo_alive "$pid" "$(jq -r .start_time <<<"$j")" || break; sleep 0.2; done ;;
  dead_owner) [ "$dry" = 1 ] && { echo "would reap dead $opid ($ev)"; exit 0; } ;;
esac
_rp() { local k; k=$(jq -c --arg u "$(lo_utc "$(lo_now)")" '.state="reaped"|.verdict="reaped_"+"'"$cls"'"|.last_heartbeat_utc=$u' "$f"); lo_wjson "$f" "$k" && lo_unclaim "$purpose" "$(jq -r .run_id "$f")"; }
lo_with_lock "$purpose" _rp; rc=$?
[ $rc -eq 0 ] && { lo_event reaped --arg op "$opid" --arg cls "$cls"; echo "reaped $opid ($cls: $ev)"; }
exit $rc
