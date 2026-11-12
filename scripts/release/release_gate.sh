#!/usr/bin/env bash
# release_gate.sh - constitution 11.4.298 per-product release gate record.
#
# Usage: release_gate.sh --product ID --hardware-record F --live-run-record F
#                        --comparison-record F --out GATE_RECORD
# Records are key=value files (every key exactly once unless noted, value non-empty):
#   hardware (read from the connected target):
#     product= target_id= connected=yes read_from=target kind=physical
#     build_sha256=<64-hex>   (one line per artifact; read from the target at run time)
#   live run:   product= suite= complete=yes tests_total=N (N>0) tests_reported=N verdicts_sha256=<64-hex>
#   comparison: the image_no_worse.sh record (verdict=, new_verdicts_sha256=<64-hex>, board=, suite=,
#               image_new=<file> <version> <sha256> lines); its board= (the meta board= value) MUST
#               equal the hardware record's target_id.
# States (last line `GATE-STATE: <state>`), exit code:
#   RELEASABLE            0   live run complete on connected PHYSICAL hardware of this product, bound to
#                             the comparison (same verdict-file sha256, same target, same suite, the
#                             build read from the target == the compared new image) + NO-WORSE-PROVEN
#   BLOCKED-NO-HARDWARE   20  no live-run/hardware record, not connected, kind not physical (emulator/host),
#                             wrong product, incomplete live run
#   BLOCKED-NO-BASELINE   21  no comparison, INCOMPARABLE, or comparison not bound to this run/target/build
#   REFUSED-REGRESSED     22  comparison verdict REGRESSED
#   BLIND                 2   usage error / unknown or malformed input. Only RELEASABLE exits 0.
# The gate record (--out) is rewritten on EVERY exit path (state=BLIND on BLIND) so no stale
# RELEASABLE survives. It carries the build identity (image_new= lines) per 11.4.298(F).
set -u
OUT=""
HW0=""; LR0=""; CR0=""
# pre-scan --out AND every record option so a usage error cannot clobber an input named earlier (F1)
for ((i=1;i<=$#;i++)); do j=$((i+1)); case "${!i}" in
  --out) OUT="${!j:-}" ;; --hardware-record) HW0="${!j:-}" ;; --live-run-record) LR0="${!j:-}" ;; --comparison-record) CR0="${!j:-}" ;; esac; done
P=""; HW=""; LR=""; CR=""
# san: replace every control character (newline included) so option/product/reason text can never
# add or duplicate a record or stdout line (F2)
san() { printf '%s' "$1" | LC_ALL=C tr '\000-\037\177' '?'; }
sha() { sha256sum -- "$1" | cut -d' ' -f1; }
get() { local c v; c="$(grep -c "^$2=" "$1" 2>/dev/null)"; [ "$c" = 1 ] || return 1
  v="$(grep "^$2=" "$1" | cut -d= -f2-)"; [ -n "$v" ] && [[ "$v" != *$'\r'* ]] || return 1; printf '%s\n' "$v"; }
HEX='^[0-9a-f]{64}$'
# out_clash: --out realpath-resolves to, or is the same inode (-ef: hardlink) as, an input record (M-c, F1)
out_clash() { local o f; [ -n "$OUT" ] || return 1; o="$(realpath -m -- "$OUT" 2>/dev/null)" || return 0
  for f in "$HW0" "$LR0" "$CR0"; do [ -n "$f" ] || continue
    [ "$(realpath -m -- "$f" 2>/dev/null)" = "$o" ] && return 0
    [ "$OUT" -ef "$f" ] && return 0; done; return 1; }
# getz: like get but the value may be empty (exactly one line required)
getz() { local c v; c="$(grep -c "^$2=" "$1" 2>/dev/null)"; [ "$c" = 1 ] || return 1
  v="$(grep "^$2=" "$1" | cut -d= -f2-)"; [[ "$v" != *$'\r'* ]] || return 1; printf '%s\n' "$v"; }
record() { # record <state> <reason>
  [ -n "$OUT" ] || return 1
  if out_clash; then return 0; fi
  {
    echo "schema=release_gate/1"; echo "product=$(san "$P")"; echo "state=$1"; echo "reason=$(san "$2")"
    echo "hardware_target_id=$(san "$(get "$HW" target_id 2>/dev/null || echo none)")"
    echo "hardware_record_sha256=$( [ -r "$HW" ] && sha "$HW" || echo absent)"
    echo "live_run_record_sha256=$( [ -r "$LR" ] && sha "$LR" || echo absent)"
    echo "comparison_record_sha256=$( [ -r "$CR" ] && sha "$CR" || echo absent)"
    [ -r "$CR" ] && grep '^image_new=' "$CR" | sed 's/^image_new=/build_identity=/'
    [ -r "$HW" ] && grep '^build_sha256=' "$HW" | sed 's/^build_sha256=/target_build_sha256=/'
    true
  } >"$OUT" 2>/dev/null
}
die() { echo "BLIND: $(san "$*")" >&2; record BLIND "$*"; echo "GATE-STATE: BLIND"; exit 2; }
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
[ "$(san "$P")" = "$P" ] || die "product id contains a control character"
HW0="$HW"; LR0="$LR"; CR0="$CR"
if out_clash; then OUT_BAD="$OUT"; die "--out $OUT_BAD resolves to an input record"; fi
# TOCTOU: snapshot the records into a private dir; all later reads use the snapshot
SNAP="$(mktemp -d)" || die "cannot create snapshot dir"
# F4: no recursive delete - remove the known snapshot files, then the (now empty) directory
trap 'rm -f -- "${SNAP:?}/hw" "${SNAP:?}/lr" "${SNAP:?}/cr"; rmdir -- "${SNAP:?}" 2>/dev/null' EXIT
snap() { local src="$1" dst="$SNAP/$2"; if [ -r "$src" ] && [ -f "$src" ]; then cp -- "$src" "$dst" && printf '%s' "$dst" || printf '%s' "$src"; else printf '%s' "$src"; fi; }
HW="$(snap "$HW0" hw)"; LR="$(snap "$LR0" lr)"; CR="$(snap "$CR0" cr)"
finish() { # finish <state> <rc> <reason>
  record "$1" "$3" || die "cannot write $OUT"
  echo "reason: $(san "$3")"; echo "GATE-STATE: $1"; exit "$2"; }

[ -r "$LR" ] || finish BLOCKED-NO-HARDWARE 20 "no live-run record"
[ -r "$HW" ] || finish BLOCKED-NO-HARDWARE 20 "no hardware record"
[ "$(get "$HW" product)" = "$P" ] || finish BLOCKED-NO-HARDWARE 20 "hardware record is not for product $P"
[ "$(get "$HW" connected)" = yes ] || finish BLOCKED-NO-HARDWARE 20 "target not connected"
[ "$(get "$HW" read_from)" = target ] || finish BLOCKED-NO-HARDWARE 20 "hardware identity not read from the target"
[ "$(get "$HW" kind)" = physical ] || finish BLOCKED-NO-HARDWARE 20 "hardware kind is not physical (emulator/host/unknown)"
TID="$(get "$HW" target_id)" || finish BLOCKED-NO-HARDWARE 20 "no target identity"
[ "$(get "$LR" product)" = "$P" ] || finish BLOCKED-NO-HARDWARE 20 "live-run record is not for product $P"
[ "$(get "$LR" complete)" = yes ] || finish BLOCKED-NO-HARDWARE 20 "live run incomplete"
tt="$(get "$LR" tests_total)"; tr="$(get "$LR" tests_reported)"
[[ "$tt" =~ ^[0-9]+$ && "$tr" =~ ^[0-9]+$ && "$tt" -gt 0 && "$tt" = "$tr" ]] || finish BLOCKED-NO-HARDWARE 20 "live run reported $tr of $tt tests"
LSUITE="$(get "$LR" suite)" || finish BLOCKED-NO-HARDWARE 20 "live-run record has no suite"
LSHA="$(get "$LR" verdicts_sha256)" && [[ "$LSHA" =~ $HEX ]] || finish BLOCKED-NO-HARDWARE 20 "live-run verdicts_sha256 missing or not 64-hex"
THASH="$(grep '^build_sha256=' "$HW" | cut -d= -f2- | sort)"
if [ -z "$THASH" ] || [ "$(printf '%s\n' "$THASH" | grep -cvE "$HEX")" != 0 ]; then
  finish BLOCKED-NO-HARDWARE 20 "hardware record build_sha256 missing or not 64-hex"
fi
[ -r "$CR" ] || finish BLOCKED-NO-BASELINE 21 "no comparison record"
v="$(get "$CR" verdict)" || finish BLOCKED-NO-BASELINE 21 "comparison record has no verdict"
case "$v" in
  NO-WORSE-PROVEN|REGRESSED|INCOMPARABLE-BASELINE-MISSING) ;;
  *) die "unknown comparison verdict '$v'" ;;
esac
[ "$v" != INCOMPARABLE-BASELINE-MISSING ] || finish BLOCKED-NO-BASELINE 21 "comparison is INCOMPARABLE-BASELINE-MISSING"
CSHA="$(get "$CR" new_verdicts_sha256)" && [[ "$CSHA" =~ $HEX ]] || finish BLOCKED-NO-BASELINE 21 "comparison new_verdicts_sha256 missing or not 64-hex"
[ "$(get "$CR" schema)" = "image_no_worse/1" ] || finish BLOCKED-NO-BASELINE 21 "comparison record schema is not image_no_worse/1"
[ "$CSHA" = "$LSHA" ] || finish BLOCKED-NO-BASELINE 21 "comparison is not about this live run"
[ "$(get "$CR" board)" = "$TID" ] || finish BLOCKED-NO-BASELINE 21 "comparison board is not the hardware target_id"
[ "$(get "$CR" suite)" = "$LSUITE" ] || finish BLOCKED-NO-BASELINE 21 "comparison suite differs from the live-run suite"
# truncated-suite cross-check (I-6): comparison new_total == live tests_total == tests_reported
NT="$(get "$CR" new_total)" && [[ "$NT" =~ ^[0-9]+$ && "$NT" -gt 0 ]] || finish BLOCKED-NO-BASELINE 21 "comparison new_total missing or not a positive integer"
[ "$NT" = "$tt" ] && [ "$NT" = "$tr" ] || finish BLOCKED-NO-BASELINE 21 "comparison new_total $NT != live tests_total $tt / tests_reported $tr (truncated suite)"
# strict image_new shape: exactly '<file> <version> <64hex>', no duplicates, no repeated file
[ "$(grep '^image_new=' "$CR" | grep -cvE '^image_new=[^ ]+ [^ ]+ [0-9a-f]{64}$')" = 0 ] || finish BLOCKED-NO-BASELINE 21 "comparison image_new line is not '<file> <version> <sha256-64-hex>'"
[ "$(grep '^image_new=' "$CR" | sort | uniq -d | grep -c '')" = 0 ] || finish BLOCKED-NO-BASELINE 21 "comparison has duplicate image_new lines"
[ "$(grep '^image_new=' "$CR" | awk '{print $1}' | sort | uniq -d | grep -c '')" = 0 ] || finish BLOCKED-NO-BASELINE 21 "comparison lists one image file twice"
# diff fields (M-a): a NO-WORSE-PROVEN record must be internally consistent
if [ "$v" = NO-WORSE-PROVEN ]; then
  for k in reasons lost targets_missing; do
    [ -z "$(getz "$CR" "$k")" ] && getz "$CR" "$k" >/dev/null || finish BLOCKED-NO-BASELINE 21 "NO-WORSE-PROVEN record has missing or non-empty '$k='"
  done
  BNP="$(get "$CR" baseline_notpass)"; NNP="$(get "$CR" new_notpass_on_baseline_set)"
  [[ "$BNP" =~ ^[0-9]+$ && "$NNP" =~ ^[0-9]+$ && "$NNP" -le "$BNP" ]] || finish BLOCKED-NO-BASELINE 21 "NO-WORSE-PROVEN record has missing or inconsistent not-pass counts"
  [ "$(grep '^diff=' "$CR" | awk '$2=="PASS" && $3!="PASS"' | grep -c '')" = 0 ] || finish BLOCKED-NO-BASELINE 21 "NO-WORSE-PROVEN record carries a PASS-lost diff line"
fi
CIMG="$(grep '^image_new=' "$CR" | awk '{print $3}' | sort)"
if [ -z "$CIMG" ] || [ "$(printf '%s\n' "$CIMG" | grep -cvE "$HEX")" != 0 ]; then
  finish BLOCKED-NO-BASELINE 21 "comparison has no well-formed image_new identity"
fi
[ "$CIMG" = "$THASH" ] || finish BLOCKED-NO-BASELINE 21 "build read from the target is not the compared new image"
[ "$v" != REGRESSED ] || finish REFUSED-REGRESSED 22 "comparison verdict REGRESSED"
finish RELEASABLE 0 "live run complete, NO-WORSE-PROVEN"
