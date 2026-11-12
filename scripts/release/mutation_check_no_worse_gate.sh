#!/usr/bin/env bash
# Paired mutation runner (constitution 1.1 / 11.4.224) for image_no_worse.sh and release_gate.sh.
# Each mutant copies the tool to a temp dir, applies ONE exact textual mutation (old must occur
# exactly once), runs that tool's test suite with CMP=/GATE=<mutant>, and requires the suite to FAIL.
# Exit 0 only if every mutant is killed. Format: tool|name|||old|||new
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="${NOWORSE_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/nowc-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/mut.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/mut.*) ;; *) echo "odd ROOT" >&2; exit 2 ;; esac
trap 'rm -rf "${ROOT:?}"' EXIT
# shellcheck disable=SC2016  # mutation texts contain literal $ on purpose
MUTANTS=(
'cmp|baseline-pass-lost never recorded|||[ "$nv" = PASS ] || { LOST=|||true || { LOST='
'cmp|INCONCLUSIVE treated as PASS|||[ "$nv" = PASS ] || { LOST="$LOST${LOST:+,}$id($nv)"; }|||[ "$nv" = PASS ] || [ "$nv" = INCONCLUSIVE ] || { LOST="$LOST${LOST:+,}$id($nv)"; }'
'cmp|absent treated as PASS|||nv="${NVD[$id]:-ABSENT}"|||nv="${NVD[$id]:-PASS}"'
'cmp|issue-count check removed|||[ "$NNOTPASS" -le "$BNOTPASS" ] |||||true ||'
'cmp|target flip not required|||[ "${BVD[$t]:-}" = FAIL ] && [ "${NVD[$t]:-}" = PASS ] |||||true ||'
'cmp|target may start as non-FAIL|||[ "${BVD[$t]:-}" = FAIL ] && [ "${NVD[$t]:-}" = PASS ]|||[ "${NVD[$t]:-}" = PASS ]'
'cmp|board difference ignored|||[ "$b" = "$(meta_get "$NM" "$k")" ] ||||||true ||'
'cmp|identical image accepted|||if [ "$(grep '"'"'^image='"'"' "$BM" | sort)" = |||if false && [ "$(grep '"'"'^image='"'"' "$BM" | sort)" = '
'cmp|empty baseline not incomparable|||[ -r "$BV" ] && [ -s "$BV" ] |||||true ||'
'cmp|unknown verdict word accepted|||[[ "$VALID" == *" $v "* ]] ||||||true ||'
'cmp|duplicate id accepted|||[ -z "${arr[$id]+x}" ] ||||||true ||'
'cmp|CR accepted|||[[ "$line" == *$'"'"'\r'"'"'* ]] && {|||false && {'
'cmp|REGRESSED exits 0|||echo "VERDICT: REGRESSED"; exit 10|||echo "VERDICT: REGRESSED"; exit 0'
'cmp|INCOMPARABLE exits 0|||echo "VERDICT: INCOMPARABLE-BASELINE-MISSING"; exit 11|||echo "VERDICT: INCOMPARABLE-BASELINE-MISSING"; exit 0'
'cmp|new tests not separated|||[ -n "${BVD[$id]+x}" ] || NEWT=|||true || NEWT='
'cmp|row-count control needle disabled|||[ "$lines" = "$n" ] ||||||true ||'
'cmp|new verdict sha not recorded|||echo "new_verdicts_sha256=$(sha "$NV")"|||echo "new_verdicts_sha256=none"'
'gate|no live-run record not blocked|||[ -r "$LR" ] || finish BLOCKED-NO-HARDWARE 20 "no live-run record"|||:'
'gate|not-connected accepted|||[ "$(get "$HW" connected)" = yes ] ||||||true ||'
'gate|emulator identity accepted|||[ "$(get "$HW" read_from)" = target ] ||||||true ||'
'gate|other product hardware accepted|||[ "$(get "$HW" product)" = "$P" ] ||||||true ||'
'gate|incomplete live run accepted|||[ "$(get "$LR" complete)" = yes ] ||||||true ||'
'gate|partial report accepted|||"$tt" = "$tr" ]]|||"$tt" -ge 0 ]]'
'gate|comparison for other run accepted|||[ "$(get "$CR" new_verdicts_sha256)" = "$(get "$LR" verdicts_sha256)" ] ||||||true ||'
'gate|regressed releasable|||[ "$v" != REGRESSED ] ||||||true ||'
'gate|incomparable releasable|||[ "$v" != INCOMPARABLE-BASELINE-MISSING ] ||||||true ||'
'gate|blocked exits 0|||finish BLOCKED-NO-BASELINE 21 "no comparison record"|||finish BLOCKED-NO-BASELINE 0 "no comparison record"'
'gate|unknown verdict tolerated|||*) die "unknown comparison verdict|||*) : "unknown comparison verdict'
'gate|stale gate record kept|||} >"$OUT" || die "cannot write $OUT"|||} >/dev/null'
)
fail=0; n=0
for m in "${MUTANTS[@]}"; do
  n=$((n+1)); tool="${m%%|*}"; rest="${m#*|}"; name="${rest%%|||*}"; rest="${rest#*|||}"; old="${rest%%|||*}"; new="${rest#*|||}"
  d="$ROOT/m$n"; mkdir -p "$d"
  case "$tool" in cmp) src=image_no_worse.sh; suite=test_image_no_worse.sh; var=CMP ;; gate) src=release_gate.sh; suite=test_release_gate.sh; var=GATE ;; esac
  cp "$HERE/$src" "$HERE/$suite" "$d/"
  if ! python3 -I - "$d/$src" "$old" "$new" <<'PY'
import sys
p, old, new = sys.argv[1:4]
s = open(p, encoding="utf-8").read()
if s.count(old) != 1:
    sys.exit(3)
open(p, "w", encoding="utf-8").write(s.replace(old, new))
PY
  then echo "BADMUTANT [$n] $name (old text not exactly once)"; fail=$((fail+1)); continue; fi
  cmp -s "$HERE/$src" "$d/$src" && { echo "BADMUTANT [$n] $name (no change)"; fail=$((fail+1)); continue; }
  if env "$var=$d/$src" NOWORSE_TMP="$BASE" "$d/$suite" >"$d/out" 2>&1; then
    echo "SURVIVED [$n] $tool: $name"; fail=$((fail+1))
  else echo "killed   [$n] $tool: $name"; fi
done
# cmp-verified restore: sources untouched
echo "mutants=$n survivors_or_bad=$fail"
[ "$fail" = 0 ]
