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
# Lock    EVERY decision is made UNDER the purpose lock and re-derived there (WF11 F2/F3): classification, identity, signal and the terminal write are one critical
#         section, so a heartbeat that landed before the lock is seen and an op that became advancing is never signalled. `--purpose` re-reads the holder under
#         the lock; an unreadable holder record, an unreadable conf (commit_push) or an unset CPA_APPROVED_DIR REFUSES (20), it is never read as a stale claim.
# Survivor The op is recorded `reaped` ONLY when the process is gone after the grace period. A process that ignores TERM keeps its record (state unchanged, `reap_survived_utc`
#         set), keeps its claim (the purpose still has ONE owner) and the script exits 8: an operator decision, never two live owners (WF11 F1).
# Containers resolved by label `op_id=<id>` AND `catalogizer.op_id=<id>` (the label scripts/containers/run_pinned.sh really sets).
# Test hook LONGOPS_TEST_SLEEP_BEFORE_LOCK (seconds) pauses before the lock so a decision outside the lock is observable.
# Exits   0 reaped (or would be, with --dry-run); 5 live advancing; 6 identity mismatch; 7 unsafe signal target; 8 the process survived TERM (record and claim kept);
#         4 cas / unknown op; 2 usage; 20 refusal (unreadable record or holder, helper not approved).
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
opid=""; purpose=""; dry=0
while [ $# -gt 0 ]; do case "$1" in --op-id) opid=${2:-}; shift 2 ;; --purpose) purpose=${2:-}; shift 2 ;; --dry-run) dry=1; shift ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
if [ -n "$purpose" ]; then
  lo_safe_name "$purpose" || lo_die usage_error "unsafe purpose"
  lo_require_approved "$purpose"
  lo_test_pause
  _rc() {   # under the purpose lock: the holder is re-read HERE; a live, expired or unreadable holder is never released
    local s rc
    [ -d "$LD/claims/$purpose" ] || { echo "nothing to release: $purpose has no claim"; return 0; }
    s=$(lo_holder_status "$purpose"); rc=$?
    [ "$rc" -ne "$RC_REFUSE" ] || return "$RC_REFUSE"
    [ "$rc" -eq 0 ] || s=nohold          # a claim directory with no holder record: stale (a crash between mkdir and the record)
    case "$s" in
      live|expired) echo "refused: holder of $purpose is $s" >&2; return "$RC_LIVE" ;;
      unreadable) echo "refused: the holder record of $purpose cannot be read (holder_unreadable); never released as stale" >&2; return "$RC_REFUSE" ;;
    esac
    [ "$dry" = 1 ] && { echo "would release stale claim $purpose ($s)"; return 0; }
    rm -rf -- "$LD/claims/$purpose"; lo_event reaped_claim --arg purpose "$purpose" --arg was "$s"; echo "released stale claim $purpose ($s)"
  }
  lo_with_lock "$purpose" _rc; exit $?
fi
[ -n "$opid" ] && lo_safe_name "$opid" || lo_die usage_error "--op-id or --purpose required"
lo_load_op "$opid"
lo_test_pause
_reap_op() {   # one critical section under the purpose lock
  local j r cls ev pid pst want have cid c k rec
  j=$(cat "$f"); r=$(lo_classify_op "$j"); cls=$(sed -n 1p <<<"$r"); ev=$(sed -n 2p <<<"$r")
  pid=$(jq -r .pid <<<"$j"); pst=$(jq -r .start_time <<<"$j")
  case "$cls" in
    terminal) echo "nothing to reap: $opid is already terminal"; return 0 ;;
    unreadable) echo "refused: the record of $opid is unreadable ($ev)" >&2; return "$RC_REFUSE" ;;
    advancing) echo "refused: $opid is live and advancing ($ev)" >&2; return "$RC_LIVE" ;;
    hung)
      want=$(jq -r .cmdline <<<"$j"); have=$(lo_cmdline "$pid")
      if [ "$want" != "$have" ]; then echo "refused: identity of pid $pid unresolved: recorded '$want' but /proc says '$have'" >&2; return "$RC_IDENT"; fi
      cid=""; for c in "op_id=$opid" "catalogizer.op_id=$opid"; do cid=$("$PODMAN" ps --filter "label=$c" --format '{{.ID}}' 2>/dev/null | head -1); [ -z "$cid" ] || break; done
      [ "$dry" = 1 ] && { echo "would reap hung $opid pid=$pid cmdline='$have' container='${cid:-none}' ($ev)"; return 0; }
      lo_signal TERM "$pid"; rc=$?
      [ "$rc" -ne "$RC_UNSAFE" ] || { echo "refused: unsafe signal target pid=$pid (rc $rc)" >&2; return "$RC_UNSAFE"; }
      [ -z "$cid" ] || "$PODMAN" stop -t 5 "$cid" >/dev/null 2>&1
      for _ in 1 2 3 4 5 6 7 8 9 10; do lo_alive "$pid" "$pst" || break; sleep 0.2; done
      if lo_alive "$pid" "$pst"; then
        rec=$(jq -c --arg u "$(lo_utc "$(lo_now)")" '.reap_survived_utc=$u|.reap_attempts=((.reap_attempts // 0) + 1)' <<<"$j"); lo_wjson "$f" "$rec"
        lo_event reap_survived --arg op "$opid" --argjson pid "$pid"
        echo "refused: pid $pid is still alive after TERM (signal rc $rc); $opid stays '$(jq -r .state <<<"$j")' and keeps its claim: an operator decision (reap_survived)" >&2; return "$RC_SURVIVED"
      fi ;;
    dead_owner) [ "$dry" = 1 ] && { echo "would reap dead $opid ($ev)"; return 0; } ;;
  esac
  k=$(jq -c --arg u "$(lo_utc "$(lo_now)")" --arg cls "$cls" '.state="reaped"|.verdict="reaped_"+$cls|.last_heartbeat_utc=$u' <<<"$j")
  lo_wjson "$f" "$k" && lo_unclaim "$purpose" "$(jq -r .run_id <<<"$j")" || return $?
  lo_event reaped --arg op "$opid" --arg cls "$cls"; echo "reaped $opid ($cls: $ev)"
}
lo_with_lock "$purpose" _reap_op; exit $?
