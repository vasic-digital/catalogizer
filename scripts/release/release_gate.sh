#!/usr/bin/env bash
# release_gate.sh - constitution 11.4.298 per-product release gate record.
#
# Usage: release_gate.sh --product ID --hardware-record F --live-run-record F
#                        --comparison-record F --out GATE_RECORD
# Records are key=value files:
#   hardware (read from the connected target): product= target_id= connected=yes read_from=target
#   live run:   product= suite= complete=yes tests_total=N tests_reported=N verdicts_sha256=
#   comparison: the image_no_worse.sh record (verdict=, new_verdicts_sha256=)
# States (last line `GATE-STATE: <state>`), exit code:
#   RELEASABLE            0   live run complete on connected hardware of this product + NO-WORSE-PROVEN
#   BLOCKED-NO-HARDWARE   20  no live-run/hardware record, not connected, wrong product, emulator,
#                             incomplete live run
#   BLOCKED-NO-BASELINE   21  no comparison, INCOMPARABLE, or comparison not about this live run
#   REFUSED-REGRESSED     22  comparison verdict REGRESSED
#   usage error/unreadable-unknown input: 2. Only RELEASABLE exits 0.
# The gate record (--out) is rewritten on every run (no stale RELEASABLE survives).
set -u
die() { echo "BLIND: $*" >&2; echo "GATE-STATE: BLIND"; exit 2; }
P=""; HW=""; LR=""; CR=""; OUT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --product) [ $# -ge 2 ] || die "$1 needs a value"; P="$2"; shift 2 ;;
    --hardware-record) [ $# -ge 2 ] || die "$1 needs a value"; HW="$2"; shift 2 ;;
    --live-run-record) [ $# -ge 2 ] || die "$1 needs a value"; LR="$2"; shift 2 ;;
    --comparison-record) [ $# -ge 2 ] || die "$1 needs a value"; CR="$2"; shift 2 ;;
    --out) [ $# -ge 2 ] || die "$1 needs a value"; OUT="$2"; shift 2 ;;
    *) die "unknown option $1" ;;
  esac
done
[ -n "$P" ] && [ -n "$OUT" ] && [ -n "$HW" ] && [ -n "$LR" ] && [ -n "$CR" ] || die "all options required and non-empty"
get() { local c; c="$(grep -c "^$2=" "$1" 2>/dev/null)"; [ "$c" = 1 ] || return 1; grep "^$2=" "$1" | cut -d= -f2-; }
sha() { sha256sum -- "$1" | cut -d' ' -f1; }
finish() { # finish <state> <rc> <reason>
  {
    echo "schema=release_gate/1"; echo "product=$P"; echo "state=$1"; echo "reason=$3"
    echo "hardware_target_id=$(get "$HW" target_id 2>/dev/null || echo none)"
    echo "hardware_record_sha256=$( [ -r "$HW" ] && sha "$HW" || echo absent)"
    echo "live_run_record_sha256=$( [ -r "$LR" ] && sha "$LR" || echo absent)"
    echo "comparison_record_sha256=$( [ -r "$CR" ] && sha "$CR" || echo absent)"
  } >"$OUT" || die "cannot write $OUT"
  echo "reason: $3"; echo "GATE-STATE: $1"; exit "$2"; }

[ -r "$LR" ] || finish BLOCKED-NO-HARDWARE 20 "no live-run record"
[ -r "$HW" ] || finish BLOCKED-NO-HARDWARE 20 "no hardware record"
[ "$(get "$HW" product)" = "$P" ] || finish BLOCKED-NO-HARDWARE 20 "hardware record is not for product $P"
[ "$(get "$HW" connected)" = yes ] || finish BLOCKED-NO-HARDWARE 20 "target not connected"
[ "$(get "$HW" read_from)" = target ] || finish BLOCKED-NO-HARDWARE 20 "hardware identity not read from the target"
[ -n "$(get "$HW" target_id)" ] || finish BLOCKED-NO-HARDWARE 20 "no target identity"
[ "$(get "$LR" product)" = "$P" ] || finish BLOCKED-NO-HARDWARE 20 "live-run record is not for product $P"
[ "$(get "$LR" complete)" = yes ] || finish BLOCKED-NO-HARDWARE 20 "live run incomplete"
tt="$(get "$LR" tests_total)"; tr="$(get "$LR" tests_reported)"
[[ "$tt" =~ ^[0-9]+$ && "$tr" =~ ^[0-9]+$ && "$tt" -gt 0 && "$tt" = "$tr" ]] || finish BLOCKED-NO-HARDWARE 20 "live run reported $tr of $tt tests"
[ -r "$CR" ] || finish BLOCKED-NO-BASELINE 21 "no comparison record"
v="$(get "$CR" verdict)" || finish BLOCKED-NO-BASELINE 21 "comparison record has no verdict"
case "$v" in
  NO-WORSE-PROVEN|REGRESSED|INCOMPARABLE-BASELINE-MISSING) ;;
  *) die "unknown comparison verdict '$v'" ;;
esac
[ "$v" != INCOMPARABLE-BASELINE-MISSING ] || finish BLOCKED-NO-BASELINE 21 "comparison is INCOMPARABLE-BASELINE-MISSING"
[ "$(get "$CR" new_verdicts_sha256)" = "$(get "$LR" verdicts_sha256)" ] || finish BLOCKED-NO-BASELINE 21 "comparison is not about this live run"
[ "$v" != REGRESSED ] || finish REFUSED-REGRESSED 22 "comparison verdict REGRESSED"
finish RELEASABLE 0 "live run complete, NO-WORSE-PROVEN"
