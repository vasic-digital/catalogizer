#!/usr/bin/env bash
# image_no_worse.sh - constitution 11.4.293 comparator: is the new flashing image LIVE-proven
# no worse than the previously flashed image on the same board/harness/suite/command?
#
# Usage:
#   image_no_worse.sh --baseline-verdicts F --new-verdicts F --baseline-meta F --new-meta F
#                     [--targets F] [--out RECORD]
#
# Verdict file (TSV, machine-written by the harness): one line per test, `test_id<TAB>verdict`,
#   verdict in PASS FAIL INCONCLUSIVE SKIPPED BLIND ERROR. No CR, no duplicate ids, >=1 row.
# Meta file (key=value lines): board= harness= suite= command= (each exactly once) and
#   one or more `image=<file> <version> <sha256>` lines (the image identity).
# Targets file (optional): one test id per line that must flip FAIL (baseline) -> PASS (new).
#
# Verdict word on the last output line: `VERDICT: <word>`; exit codes:
#   0  NO-WORSE-PROVEN
#   10 REGRESSED (reason codes baseline-pass-lost, issue-count-higher, targeted-flip-missing)
#   11 INCOMPARABLE-BASELINE-MISSING (baseline absent/empty, or board/harness/suite/command/
#      image identity differ - a blocker, never a pass)
#   2  usage error or BLIND (malformed new input, instrument cannot decide); never a pass
# Only baseline-PASS tests count as must-hold; a baseline-PASS test that is not PASS in the new
# run (FAIL, INCONCLUSIVE, SKIPPED, BLIND, ERROR or absent) is a regression. New tests (not in
# the baseline) are reported separately. The record (--out) is key=value with sha256 of all inputs.
set -u
blind() { echo "BLIND: $*" >&2; echo "VERDICT: BLIND"; exit 2; }
BV=""; NV=""; BM=""; NM=""; TG=""; OUT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --baseline-verdicts) { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; BV="$2"; shift 2 ;;
    --new-verdicts)      { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; NV="$2"; shift 2 ;;
    --baseline-meta)     { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; BM="$2"; shift 2 ;;
    --new-meta)          { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; NM="$2"; shift 2 ;;
    --targets)           { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; TG="$2"; shift 2 ;;
    --out)               { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; OUT="$2"; shift 2 ;;
    *) blind "unknown option $1" ;;
  esac
done
[ -n "$NV" ] && [ -n "$NM" ] && [ -n "$BV" ] && [ -n "$BM" ] || blind "all four input files are required"

VALID=" PASS FAIL INCONCLUSIVE SKIPPED BLIND ERROR "
declare -A BVD NVD
# load_verdicts <file> <assoc-name> ; sets LV_TOTAL, LV_ORDER. returns 0 ok, 1 malformed (msg in LV_ERR)
load_verdicts() {
  local f="$1"; local line id v n=0 lines; LV_ERR=""; LV_TOTAL=0; LV_ORDER=()
  local -n arr="$2"
  while IFS= read -r line || [ -n "$line" ]; do
    [[ "$line" == *$'\r'* ]] && { LV_ERR="CR in $f"; return 1; }
    [ -n "$line" ] || { LV_ERR="blank line in $f"; return 1; }
    [[ "$line" == *$'\t'* ]] || { LV_ERR="not TSV in $f: $line"; return 1; }
    id="${line%%$'\t'*}"; v="${line#*$'\t'}"
    [[ "$v" != *$'\t'* ]] && [ -n "$id" ] || { LV_ERR="bad row in $f: $line"; return 1; }
    [[ "$VALID" == *" $v "* ]] || { LV_ERR="unknown verdict '$v' in $f"; return 1; }
    [ -z "${arr[$id]+x}" ] || { LV_ERR="duplicate test id '$id' in $f"; return 1; }
    # shellcheck disable=SC2004  # associative array: $id is required
    arr[$id]="$v"; LV_ORDER+=("$id"); n=$((n+1))
  done <"$f"
  [ "$n" -gt 0 ] || { LV_ERR="no rows in $f"; return 1; }
  # control needle: independent row count must match what the parser counted
  lines="$(grep -c '' "$f")"
  [ "$lines" = "$n" ] || { LV_ERR="row count mismatch in $f ($lines vs $n)"; return 1; }
  LV_TOTAL="$n"; return 0
}
# meta_get <file> <key> -> value if the key appears exactly once
meta_get() { local c; c="$(grep -c "^$2=" "$1")"; [ "$c" = 1 ] || return 1; grep "^$2=" "$1" | cut -d= -f2-; }

[ -r "$NV" ] || blind "new verdicts unreadable: $NV"
[ -r "$NM" ] || blind "new meta unreadable: $NM"
load_verdicts "$NV" NVD || blind "$LV_ERR"
NTOTAL="$LV_TOTAL"; NORDER=("${LV_ORDER[@]}")
for k in board harness suite command; do
  meta_get "$NM" "$k" >/dev/null || blind "new meta lacks exactly one '$k='"
done
grep -q '^image=' "$NM" || blind "new meta lacks image= identity"

sha() { sha256sum -- "$1" | cut -d' ' -f1; }
incomparable() { # incomparable <reason>
  echo "INCOMPARABLE: $1" >&2
  write_record "INCOMPARABLE-BASELINE-MISSING" "$1" "" "" "" ""
  echo "VERDICT: INCOMPARABLE-BASELINE-MISSING"; exit 11; }

write_record() { # verdict reasons lost newt missing-targets issuesinfo
  [ -n "$OUT" ] || return 0
  {
    echo "schema=image_no_worse/1"
    echo "verdict=$1"
    echo "reasons=$2"
    echo "baseline_verdicts_sha256=$( [ -r "$BV" ] && sha "$BV" || echo absent)"
    echo "new_verdicts_sha256=$(sha "$NV")"
    echo "baseline_meta_sha256=$( [ -r "$BM" ] && sha "$BM" || echo absent)"
    echo "new_meta_sha256=$(sha "$NM")"
    [ -z "$TG" ] || echo "targets_sha256=$( [ -r "$TG" ] && sha "$TG" || echo absent)"
    for k in board harness suite command; do echo "$k=$(meta_get "$NM" "$k")"; done
    [ -r "$BM" ] && grep '^image=' "$BM" | sed 's/^image=/image_baseline=/'
    grep '^image=' "$NM" | sed 's/^image=/image_new=/'
    echo "baseline_total=${BTOTAL:-0}"
    echo "baseline_pass=${BPASS:-0}"
    echo "baseline_notpass=${BNOTPASS:-0}"
    echo "new_total=$NTOTAL"
    echo "new_notpass_on_baseline_set=${NNOTPASS:-0}"
    echo "lost=${3:-}"
    echo "new_tests=${4:-}"
    echo "targets_missing=${5:-}"
  } >"$OUT"
}

# ---- baseline presence / comparability
[ -r "$BV" ] && [ -s "$BV" ] || incomparable "baseline verdicts missing or empty"
[ -r "$BM" ] || incomparable "baseline meta missing"
load_verdicts "$BV" BVD || incomparable "baseline malformed: $LV_ERR"
BTOTAL="$LV_TOTAL"; BORDER=("${LV_ORDER[@]}")
for k in board harness suite command; do
  b="$(meta_get "$BM" "$k")" || incomparable "baseline meta lacks exactly one '$k='"
  [ "$b" = "$(meta_get "$NM" "$k")" ] || incomparable "$k differs between runs"
done
grep -q '^image=' "$BM" || incomparable "baseline meta lacks image= identity"
if [ "$(grep '^image=' "$BM" | sort)" = "$(grep '^image=' "$NM" | sort)" ]; then
  incomparable "baseline and new image identity are identical"
fi

# ---- comparison
BPASS=0; BNOTPASS=0; NNOTPASS=0; LOST=""; REASONS=""
for id in "${BORDER[@]}"; do
  bv="${BVD[$id]}"; nv="${NVD[$id]:-ABSENT}"
  if [ "$bv" = PASS ]; then
    BPASS=$((BPASS+1))
    [ "$nv" = PASS ] || { LOST="$LOST${LOST:+,}$id($nv)"; }
  else
    BNOTPASS=$((BNOTPASS+1))
  fi
  [ "$nv" = PASS ] || NNOTPASS=$((NNOTPASS+1))
done
[ -z "$LOST" ] || REASONS="baseline-pass-lost"
[ "$NNOTPASS" -le "$BNOTPASS" ] || REASONS="$REASONS${REASONS:+,}issue-count-higher"
NEWT=""
for id in "${NORDER[@]}"; do [ -n "${BVD[$id]+x}" ] || NEWT="$NEWT${NEWT:+,}$id"; done
MISSING=""
if [ -n "$TG" ]; then
  [ -r "$TG" ] || blind "targets file unreadable: $TG"
  while IFS= read -r t || [ -n "$t" ]; do
    [ -n "$t" ] || continue
    [ "${BVD[$t]:-}" = FAIL ] && [ "${NVD[$t]:-}" = PASS ] || MISSING="$MISSING${MISSING:+,}$t"
  done <"$TG"
  [ -z "$MISSING" ] || REASONS="$REASONS${REASONS:+,}targeted-flip-missing"
fi
if [ -n "$REASONS" ]; then
  write_record REGRESSED "$REASONS" "$LOST" "$NEWT" "$MISSING"
  echo "reasons: $REASONS lost=[$LOST] targets_missing=[$MISSING]"
  echo "VERDICT: REGRESSED"; exit 10
fi
write_record NO-WORSE-PROVEN "" "" "$NEWT" ""
echo "baseline_total=$BTOTAL baseline_pass=$BPASS new_tests=[$NEWT]"
echo "VERDICT: NO-WORSE-PROVEN"; exit 0
