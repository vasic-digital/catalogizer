#!/usr/bin/env bash
# image_no_worse.sh - constitution 11.4.293 comparator: is the new flashing image LIVE-proven
# no worse than the previously flashed image on the same board/harness/suite/command?
#
# Usage:
#   image_no_worse.sh --baseline-verdicts F --new-verdicts F --baseline-meta F --new-meta F
#                     [--targets F] [--out RECORD]
#
# Verdict file (TSV, harness-written): one line per test, `test_id<TAB>verdict`,
#   verdict in PASS FAIL INCONCLUSIVE SKIPPED BLIND ERROR. No CR, no duplicate ids, >=1 row.
# Meta file (key=value lines): board= harness= suite= command= (each exactly once, NON-EMPTY, no CR;
#   `board=` must carry the target identity the release gate binds to) and one or more
#   `image=<file> <version> <sha256-64-hex>` lines (the image identity; every image= line must match).
# Targets file (optional): one test id per line that must flip FAIL (baseline) -> PASS (new).
#
# Verdict word on the last output line: `VERDICT: <word>`; exit codes:
#   0  NO-WORSE-PROVEN
#   10 REGRESSED (reason codes baseline-pass-lost, issue-count-higher, targeted-flip-missing)
#   11 INCOMPARABLE-BASELINE-MISSING (baseline absent/empty/malformed, any meta identity field
#      missing/empty/malformed on either side, board/harness/suite/command differ, or the new image
#      file-sha256 set equals the baseline's) - a blocker, never a pass
#   2  usage error or BLIND (malformed new verdicts, unreadable targets, --out not writable);
#      never a pass
# --out is rewritten on EVERY exit path (verdict=BLIND on a BLIND exit) so no stale record survives.
# A baseline-PASS test that is not PASS in the new run (FAIL, INCONCLUSIVE, SKIPPED, BLIND, ERROR or
# absent) is a regression. New tests are reported separately. When --targets is omitted the record
# says targets=absent (no flip was demanded).
set -u
OUT=""
# pre-scan --out so that even a usage error rewrites the record
for ((i=1;i<=$#;i++)); do if [ "${!i}" = "--out" ]; then j=$((i+1)); OUT="${!j:-}"; fi; done
blind() {
  echo "BLIND: $*" >&2
  [ -z "$OUT" ] || printf 'schema=image_no_worse/1\nverdict=BLIND\nreasons=%s\n' "$*" >"$OUT" 2>/dev/null
  echo "VERDICT: BLIND"; exit 2; }
BV=""; NV=""; BM=""; NM=""; TG=""; TGSET=0
while [ $# -gt 0 ]; do
  case "$1" in
    --baseline-verdicts) { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; BV="$2"; shift 2 ;;
    --new-verdicts)      { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; NV="$2"; shift 2 ;;
    --baseline-meta)     { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; BM="$2"; shift 2 ;;
    --new-meta)          { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; NM="$2"; shift 2 ;;
    --targets)           { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; TG="$2"; TGSET=1; shift 2 ;;
    --out)               { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "$1 needs a value"; OUT="$2"; shift 2 ;;
    *) blind "unknown option $1" ;;
  esac
done
[ -n "$NV" ] && [ -n "$NM" ] && [ -n "$BV" ] && [ -n "$BM" ] || blind "all four input files are required"
if [ -n "$OUT" ]; then : >"$OUT" 2>/dev/null || { echo "BLIND: cannot write --out $OUT" >&2; echo "VERDICT: BLIND"; exit 2; }; fi

VALID=" PASS FAIL INCONCLUSIVE SKIPPED BLIND ERROR "
declare -A BVD NVD
# load_verdicts <file> <assoc-name>; sets LV_TOTAL, LV_ORDER; 0 ok, 1 malformed (LV_ERR)
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
# meta_get <file> <key>: value only if the key appears exactly once, non-empty, no CR
meta_get() { local c v; c="$(grep -c "^$2=" "$1")"; [ "$c" = 1 ] || return 1
  v="$(grep "^$2=" "$1" | cut -d= -f2-)"; [ -n "$v" ] && [[ "$v" != *$'\r'* ]] || return 1; printf '%s\n' "$v"; }
# meta_images <file>: prints sorted sha256 list; fails unless >=1 image= line and all match the shape
IMG_RE='^image=[^ ]+ [^ ]+ [0-9a-f]{64}$'
meta_images() { local n bad; n="$(grep -c '^image=' "$1")"; [ "$n" -ge 1 ] || return 1
  bad="$(grep '^image=' "$1" | grep -cvE "$IMG_RE")"; [ "$bad" = 0 ] || return 1
  grep '^image=' "$1" | awk '{print $3}' | sort; }
sha() { sha256sum -- "$1" | cut -d' ' -f1; }

write_record() { # verdict reasons lost newt missing-targets ; returns non-zero if the write fails
  [ -n "$OUT" ] || return 0
  {
    echo "schema=image_no_worse/1"
    echo "verdict=$1"
    echo "reasons=$2"
    echo "baseline_verdicts_path=$BV"
    echo "new_verdicts_path=$NV"
    echo "baseline_verdicts_sha256=$( [ -r "$BV" ] && sha "$BV" || echo absent)"
    echo "new_verdicts_sha256=$(sha "$NV")"
    echo "baseline_meta_sha256=$( [ -r "$BM" ] && sha "$BM" || echo absent)"
    echo "new_meta_sha256=$( [ -r "$NM" ] && sha "$NM" || echo absent)"
    if [ "$TGSET" = 1 ]; then echo "targets_sha256=$( [ -r "$TG" ] && sha "$TG" || echo absent)"; else echo "targets=absent"; fi
    for k in board harness suite command; do echo "$k=$(meta_get "$NM" "$k" 2>/dev/null || echo UNRESOLVED)"; done
    [ -r "$BM" ] && grep '^image=' "$BM" | sed 's/^image=/image_baseline=/'
    [ -r "$NM" ] && grep '^image=' "$NM" | sed 's/^image=/image_new=/'
    echo "baseline_total=${BTOTAL:-0}"
    echo "baseline_pass=${BPASS:-0}"
    echo "baseline_notpass=${BNOTPASS:-0}"
    echo "new_total=$NTOTAL"
    echo "new_notpass_on_baseline_set=${NNOTPASS:-0}"
    echo "lost=${3:-}"
    echo "new_tests=${4:-}"
    echo "targets_missing=${5:-}"
    local id
    for id in "${BORDER[@]+"${BORDER[@]}"}"; do
      if [ "${BVD[$id]}" != "${NVD[$id]:-ABSENT}" ]; then echo "diff=$id ${BVD[$id]} ${NVD[$id]:-ABSENT}"; fi
    done
    for id in ${FLIPS:-}; do echo "flip=$id ${BVD[$id]} ${NVD[$id]}"; done
    for id in ${NEWT//,/ }; do echo "new_test_verdict=$id ${NVD[$id]}"; done
  } >"$OUT" 2>/dev/null
}
emit() { # emit <verdict> <rc> <reasons> <lost> <newt> <missing>
  write_record "$1" "$3" "$4" "$5" "$6" || blind "cannot write --out $OUT"
  echo "VERDICT: $1"; exit "$2"; }
incomparable() { echo "INCOMPARABLE: $1" >&2; emit INCOMPARABLE-BASELINE-MISSING 11 "$1" "" "" ""; }

[ -r "$NV" ] || blind "new verdicts unreadable: $NV"
load_verdicts "$NV" NVD || blind "$LV_ERR"
NTOTAL="$LV_TOTAL"; NORDER=("${LV_ORDER[@]}")
BORDER=(); NEWT=""; FLIPS=""

# ---- identity / comparability (any problem on either side => INCOMPARABLE, 11.4.293(C))
[ -r "$NM" ] || incomparable "new meta missing"
[ -r "$BM" ] || incomparable "baseline meta missing"
for k in board harness suite command; do
  meta_get "$NM" "$k" >/dev/null || incomparable "new meta '$k=' missing, duplicated, empty or has CR"
  meta_get "$BM" "$k" >/dev/null || incomparable "baseline meta '$k=' missing, duplicated, empty or has CR"
  [ "$(meta_get "$BM" "$k")" = "$(meta_get "$NM" "$k")" ] || incomparable "$k differs between runs"
done
NSET="$(meta_images "$NM")" || incomparable "new meta image= identity missing or malformed (need '<file> <version> <sha256-64-hex>')"
BSET="$(meta_images "$BM")" || incomparable "baseline meta image= identity missing or malformed"
[ "$NSET" != "$BSET" ] || incomparable "new image file-sha256 set equals the baseline's (same image)"
[ -r "$BV" ] && [ -s "$BV" ] || incomparable "baseline verdicts missing or empty"
load_verdicts "$BV" BVD || incomparable "baseline malformed: $LV_ERR"
BTOTAL="$LV_TOTAL"; BORDER=("${LV_ORDER[@]}")

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
MISSING=""; FLIPS=""
if [ "$TGSET" = 1 ]; then
  [ -r "$TG" ] || blind "targets file unreadable: $TG"
  ntg=0
  while IFS= read -r t || [ -n "$t" ]; do
    [ -n "$t" ] || continue
    ntg=$((ntg+1))
    if [ "${BVD[$t]:-}" = FAIL ] && [ "${NVD[$t]:-}" = PASS ]; then FLIPS="$FLIPS $t"; else MISSING="$MISSING${MISSING:+,}$t"; fi
  done <"$TG"
  [ "$ntg" -gt 0 ] || blind "targets file has no test ids: $TG"
  [ -z "$MISSING" ] || REASONS="$REASONS${REASONS:+,}targeted-flip-missing"
fi
if [ -n "$REASONS" ]; then
  echo "reasons: $REASONS lost=[$LOST] targets_missing=[$MISSING]"
  emit REGRESSED 10 "$REASONS" "$LOST" "$NEWT" "$MISSING"
fi
echo "baseline_total=$BTOTAL baseline_pass=$BPASS new_tests=[$NEWT]"
emit NO-WORSE-PROVEN 0 "" "" "$NEWT" ""
