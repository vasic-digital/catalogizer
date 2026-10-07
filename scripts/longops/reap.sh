#!/usr/bin/env bash
# reap.sh - reap a long operation ONLY on proven staleness (11.4.232 E; 11.4.196 D; 11.4.263).
#
# Usage   reap.sh --op-id <id> [--dry-run]
#         reap.sh --purpose <key> [--dry-run]     release the claim of a dead holder with no live op (stale_claim)
# Rules   dead_owner  the recorded pid no longer matches /proc (start time): no signal at all, the record becomes `reaped`, the claim is released.
#         hung        the owner lives, its identity is re-resolved from /proc/<pid>/cmdline (must equal the recorded cmdline) and the
#                     op's labelled container is resolved with `podman ps --filter label=...`; then SIGTERM goes to that ONE pid
#                     through lo_signal, which refuses pid or pgid <= 1 and never signals a process group. A bare `pgrep` is never used.
#         advancing   refused (exit 5): a live advancing op is never reaped.
#         identity unresolvable (cmdline differs) refused, exit 6: conservative-safe (11.4.201 4), the evidence is printed.
# Lock    The purpose lock is held for DECISIONS and WRITES only, never across waiting or a container runtime (WF14 R2-1, R2-8). Three steps, the middle one outside the lock:
#           A (locked)    classify, identity, re-check the owner's start time, send TERM to the one pid. Nothing slow runs here.
#           wait (unlocked) stop the op's container (`timeout 30 podman stop`) and wait up to LONGOPS_REAP_GRACE_S (default 15) for the owner to exit. The owner can finish its own exit path,
#                         which may itself take the lock: the dispatch pump's TERM trap runs `release.sh --state handoff`, the runner wrapper stops its container before `release.sh`.
#           B (locked)    RE-DERIVE from the record: the owner released the op itself -> report that state (exit 0, never "survived"); the owner is gone -> `reaped`; still alive -> survivor.
#         `--purpose` re-reads the holder under the lock; an unreadable holder record, an unreadable conf (commit_push) or an unset CPA_APPROVED_DIR REFUSES (20), it is never read as a stale
#         claim; a claim whose purpose has a non-terminal op with a LIVE owner is never released (5): the claim is the op's even when its holder record is stale (WF14 R2-3).
# Survivor The op is recorded `reaped` ONLY when the process is gone after the grace period. A process that ignores TERM keeps its record (state unchanged, `reap_survived_utc`
#         set), keeps its claim (the purpose still has ONE owner) and the script exits 8: an operator decision, never two live owners (WF11 F1).
# Containers resolved by label `op_id=<id>` AND `catalogizer.op_id=<id>` (the label scripts/containers/run_pinned.sh really sets) AND the op record's own `container_label`
#         (scripts/build/dispatch.sh records `catalogizer.op_id=dispatch-<build id>`, the op id is `<build id>`; WF14 R2-4).
# Test hook LONGOPS_TEST_SLEEP_BEFORE_LOCK (seconds) pauses before the lock so a decision outside the lock is observable.
# Exits   0 reaped / released itself (or would be, with --dry-run); 5 live advancing (or a live op owns the purpose); 6 identity mismatch; 7 unsafe signal target; 8 the process survived TERM (record and claim kept);
#         4 cas / unknown op; 2 usage; 20 refusal (unreadable record or holder, helper not approved).
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
opid=""; purpose=""; dry=0
while [ $# -gt 0 ]; do case "$1" in --op-id) opid=${2:-}; shift 2 ;; --purpose) purpose=${2:-}; shift 2 ;; --dry-run) dry=1; shift ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
GRACE=${LONGOPS_REAP_GRACE_S:-15}; lo_uint "$GRACE" || lo_die usage_error "LONGOPS_REAP_GRACE_S must be a non-negative integer"
if [ -n "$purpose" ]; then
  lo_safe_name "$purpose" || lo_die usage_error "unsafe purpose"
  lo_require_approved "$purpose"
  lo_test_pause
  _live_op_of_purpose() {   # prints the id of a non-terminal op of this purpose whose owner lives (advancing or hung) or whose record cannot be judged; return 1 when none
    local f c
    for f in "$LD"/ops/*.json; do [ -e "$f" ] || continue
      jq -e --arg p "$purpose" 'type=="object" and .purpose_key==$p' "$f" >/dev/null 2>&1 || continue
      c=$(lo_classify_op "$(cat "$f" 2>/dev/null)" | head -1)
      case "$c" in advancing|hung) echo "live:$(jq -r .op_id "$f")"; return 0 ;; unreadable) echo "unreadable:$(jq -r '.op_id // "?"' "$f")"; return 0 ;; esac
    done; return 1
  }
  _rc() {   # under the purpose lock: the holder is re-read HERE; a live, expired or unreadable holder is never released
    local s rc lo
    [ -d "$LD/claims/$purpose" ] || { echo "nothing to release: $purpose has no claim"; return 0; }
    s=$(lo_holder_status "$purpose"); rc=$?
    [ "$rc" -ne "$RC_REFUSE" ] || return "$RC_REFUSE"
    [ "$rc" -eq 0 ] || s=nohold          # a claim directory with no holder record: stale (a crash between mkdir and the record)
    case "$s" in
      live|expired) echo "refused: holder of $purpose is $s" >&2; return "$RC_LIVE" ;;
      unreadable) echo "refused: the holder record of $purpose cannot be read (holder_unreadable); never released as stale" >&2; return "$RC_REFUSE" ;;
    esac
    if lo=$(_live_op_of_purpose); then
      case "$lo" in
        live:*) echo "refused: the holder record of $purpose is $s but op ${lo#live:} of that purpose has a LIVE owner: its claim is never released (WF14 R2-3)" >&2; return "$RC_LIVE" ;;
        *) echo "refused: op ${lo#unreadable:} of purpose $purpose cannot be judged (unreadable record); its claim is never released as stale" >&2; return "$RC_REFUSE" ;;
      esac
    fi
    [ "$dry" = 1 ] && { echo "would release stale claim $purpose ($s)"; return 0; }
    rm -rf -- "$LD/claims/$purpose"; lo_event reaped_claim --arg purpose "$purpose" --arg was "$s"; echo "released stale claim $purpose ($s)"
  }
  lo_with_lock "$purpose" _rc; exit $?
fi
[ -n "$opid" ] && lo_safe_name "$opid" || lo_die usage_error "--op-id or --purpose required"
lo_load_op "$opid"
lo_test_pause
SF=$(mktemp "${TMPDIR:-/tmp}/reap-state.XXXXXX") || lo_die internal "cannot create a scratch file" 1
trap 'rm -f "$SF"' EXIT

# the containers of the op, resolved OUTSIDE the lock (a wedged container runtime must not hold the purpose lock, WF14 R2-8). Only an op that looks hung now needs them.
cid=""
if [ "$(lo_classify_op "$(cat "$f")" | head -1)" = hung ]; then
  clab=$(jq -r '.container_label // ""' "$f"); cands=("op_id=$opid" "catalogizer.op_id=$opid")
  case "$clab" in "$opid"|"") ;; *=*) cands+=("$clab") ;; *) cands+=("catalogizer.op_id=$clab" "op_id=$clab") ;; esac
  for c in "${cands[@]}"; do cid=$(timeout 30 "$PODMAN" ps --filter "label=$c" --format '{{.ID}}' 2>/dev/null | head -1); [ -z "$cid" ] || break; done
fi

_reap_write() {   # _reap_write <record-json> <class> <evidence>: the terminal write + the claim release (only this op's own claim, WF14 R2-10) + the event
  local k; k=$(jq -c --arg u "$(lo_utc "$(lo_now)")" --arg cls "$2" '.state="reaped"|.verdict="reaped_"+$cls|.last_heartbeat_utc=$u' <<<"$1")
  lo_wjson "$f" "$k" && lo_unclaim_own "$purpose" "$(jq -r .run_id <<<"$1")" || return $?
  lo_event reaped --arg op "$opid" --arg cls "$2"; echo "reaped $opid ($2: $3)"
}
_reap_a() {   # step A, under the purpose lock: decide, and for a hung op send TERM to the one pid. Writes "<pid> <start>" to $SF when the caller must wait for the owner.
  local j r cls ev pid pst want have rc
  j=$(cat "$f"); r=$(lo_classify_op "$j"); cls=$(sed -n 1p <<<"$r"); ev=$(sed -n 2p <<<"$r")
  pid=$(jq -r .pid <<<"$j"); pst=$(jq -r .start_time <<<"$j")
  case "$cls" in
    terminal) echo "nothing to reap: $opid is already terminal"; return 0 ;;
    unreadable) echo "refused: the record of $opid is unreadable ($ev)" >&2; return "$RC_REFUSE" ;;
    advancing) echo "refused: $opid is live and advancing ($ev)" >&2; return "$RC_LIVE" ;;
    hung)
      want=$(jq -r .cmdline <<<"$j"); have=$(lo_cmdline "$pid")
      if [ "$want" != "$have" ]; then echo "refused: identity of pid $pid unresolved: recorded '$want' but /proc says '$have'" >&2; return "$RC_IDENT"; fi
      [ "$dry" = 1 ] && { echo "would reap hung $opid pid=$pid cmdline='$have' container='${cid:-none}' ($ev)"; return 0; }
      # the pid must still be the process that was judged (a recycled pid inside the window must never receive the signal, WF14 R2-9)
      [ "$(lo_pstart "$pid")" = "$pst" ] || { echo "refused: pid $pid is no longer the process that was judged (start time changed before the signal)" >&2; return "$RC_IDENT"; }
      lo_signal TERM "$pid"; rc=$?
      [ "$rc" -ne "$RC_UNSAFE" ] || { echo "refused: unsafe signal target pid=$pid (rc $rc)" >&2; return "$RC_UNSAFE"; }
      printf '%s %s\n' "$pid" "$pst" >"$SF"; return 0 ;;
    dead_owner) [ "$dry" = 1 ] && { echo "would reap dead $opid ($ev)"; return 0; }
      _reap_write "$j" "$cls" "$ev"; return $? ;;
  esac
}
_reap_b() {   # step B, under the purpose lock: the record is re-read and re-classified; nothing from step A is trusted
  local j r cls ev
  j=$(cat "$f"); r=$(lo_classify_op "$j"); cls=$(sed -n 1p <<<"$r"); ev=$(sed -n 2p <<<"$r")
  case "$cls" in
    terminal) echo "reaped $opid: the owner released the op itself after TERM (state $(jq -r .state <<<"$j"), verdict '$(jq -r .verdict <<<"$j")')"; return 0 ;;
    unreadable) echo "refused: the record of $opid became unreadable after TERM ($ev)" >&2; return "$RC_REFUSE" ;;
    dead_owner) _reap_write "$j" hung "TERM delivered, the owner is gone; $ev"; return $? ;;
    *) lo_wjson "$f" "$(jq -c --arg u "$(lo_utc "$(lo_now)")" '.reap_survived_utc=$u|.reap_attempts=((.reap_attempts // 0) + 1)' <<<"$j")"
       lo_event reap_survived --arg op "$opid" --argjson pid "$(jq -r .pid <<<"$j")"
       echo "refused: pid $(jq -r .pid <<<"$j") is still alive ${GRACE}s after TERM; $opid stays '$(jq -r .state <<<"$j")' and keeps its claim: an operator decision (reap_survived)" >&2; return "$RC_SURVIVED" ;;
  esac
}
lo_with_lock "$purpose" _reap_a; rc=$?
[ "$rc" -eq 0 ] && [ -s "$SF" ] || exit "$rc"
read -r pid pst <"$SF"
[ -z "$cid" ] || timeout 30 "$PODMAN" stop -t 5 "$cid" >/dev/null 2>&1
n=0; while [ "$n" -lt $((GRACE * 5)) ]; do
  lo_alive "$pid" "$pst" || break
  case "$(jq -r '.state // ""' "$f" 2>/dev/null)" in complete|failed|reaped|handoff|blocked-escape) break ;; esac
  sleep 0.2; n=$((n+1))
done
lo_with_lock "$purpose" _reap_b; exit $?
