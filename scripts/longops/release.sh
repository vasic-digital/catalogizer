#!/usr/bin/env bash
# release.sh - end a long operation in a terminal state and release its purpose claim (11.4.232 A, D; docs/16 13.2).
#
# Usage   release.sh --op-id <id> --state complete|failed|reaped|handoff|blocked-escape [--verdict <v>] [--evidence-path <p>]
#         release.sh --purpose <key> --run-id <id>        release a claim that has no op record (an acquire.sh lock)
# Effect  the op record gets its terminal state (success is the verdict the operation wrote, never a process exit code); the claim is
#         removed by compare-and-swap on the holder's run id under the purpose flock, and ONLY when this op holds it: a claim that now belongs to another run (the successor that re-adopted a
#         `handoff` purpose, another op of the purpose) is left untouched and is not an error, the record write having succeeded (WF14 R2-10). `handoff` records a re-adoptable stop (11.4.232 D).
# Terminal states are IMMUTABLE (WF11 F11): a record already complete, failed, reaped or blocked-escape is never rewritten (4 already_terminal; the same state and
#         verdict again is an idempotent no-op, 0); `handoff` is the one re-adoptable state and may be resolved into any terminal state.
# Exits   0; 2 usage; 4 already_terminal (or unknown op); 20 op_record_unreadable. `--purpose --run-id` keeps its compare-and-swap: 4 cas_mismatch when the holder is another run.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
opid=""; state=""; verdict=""; evp=""; purpose=""; runid=""
while [ $# -gt 0 ]; do
  case "$1" in --op-id) opid=${2:-}; shift 2 ;; --state) state=${2:-}; shift 2 ;; --verdict) verdict=${2:-}; shift 2 ;; --evidence-path) evp=${2:-}; shift 2 ;;
    --purpose) purpose=${2:-}; shift 2 ;; --run-id) runid=${2:-}; shift 2 ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac
done
if [ -n "$opid" ]; then
  lo_safe_name "$opid" || lo_die usage_error "unsafe op id"
  case "$state" in complete|failed|reaped|handoff|blocked-escape) ;; *) lo_die usage_error "--state must be a terminal state" ;; esac
  lo_load_op "$opid"
  runid=$(jq -r .run_id "$f")
  _rel() {
    local j cur; j=$(cat "$f"); cur=$(jq -r .state <<<"$j")
    case "$cur" in
      complete|failed|reaped|blocked-escape)
        if [ "$cur" = "$state" ] && [ "$(jq -r .verdict <<<"$j")" = "$verdict" ]; then return 0; fi
        echo "already_terminal: op $opid is $cur (verdict '$(jq -r .verdict <<<"$j")'); a terminal state is never rewritten" >&2; return "$RC_CAS" ;;
    esac
    j=$(jq -c --arg s "$state" --arg v "$verdict" --arg e "$evp" --arg u "$(lo_utc "$(lo_now)")" '.state=$s|.verdict=$v|.evidence_path=$e|.last_heartbeat_utc=$u' <<<"$j")
    lo_wjson "$f" "$j" && lo_unclaim_own "$purpose" "$runid"
  }
  lo_with_lock "$purpose" _rel; rc=$?
  [ $rc -eq 0 ] && lo_event released --arg op "$opid" --arg state "$state"
  exit $rc
fi
[ -n "$purpose" ] && [ -n "$runid" ] || lo_die usage_error "--op-id, or --purpose with --run-id, is required"
lo_with_lock "$purpose" lo_unclaim "$purpose" "$runid"
