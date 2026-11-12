#!/usr/bin/env bash
# Paired mutation runner (constitution 1.1 / 11.4.224) for image_no_worse.sh and release_gate.sh.
# Each mutant copies the tool to a temp dir, applies ONE exact textual mutation (old must occur
# exactly once), runs that tool's test suite with CMP=/GATE=<mutant>, and requires the suite to FAIL.
# Exit 0 only if every mutant is killed. Format: tool@@@name@@@old@@@new
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="${NOWORSE_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/nowc-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/mut.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/mut.*) ;; *) echo "odd ROOT" >&2; exit 2 ;; esac
trap 'rm -rf "${ROOT:?}"' EXIT
MUTANTS=(
$'cmp@@@baseline-pass-lost never recorded@@@[ "$nv" = PASS ] || { LOST=@@@true || { LOST='
$'cmp@@@INCONCLUSIVE treated as PASS@@@[ "$nv" = PASS ] || { LOST="$LOST@@@[ "$nv" = PASS ] || [ "$nv" = INCONCLUSIVE ] || { LOST="$LOST'
$'cmp@@@absent treated as PASS@@@nv="${NVD[$id]:-ABSENT}"@@@nv="${NVD[$id]:-PASS}"'
$'cmp@@@issue-count check removed@@@[ "$NNOTPASS" -le "$BNOTPASS" ] ||@@@true ||'
$'cmp@@@R5 equal count is higher@@@"$NNOTPASS" -le "$BNOTPASS"@@@"$NNOTPASS" -lt "$BNOTPASS"'
$'cmp@@@target flip not required@@@if [ "${BVD[$t]:-}" = FAIL ] && [ "${NVD[$t]:-}" = PASS ]; then FLIPS@@@if true; then FLIPS'
$'cmp@@@target may start as non-FAIL@@@[ "${BVD[$t]:-}" = FAIL ] && [ "${NVD[$t]:-}" = PASS ]; then FLIPS@@@[ "${NVD[$t]:-}" = PASS ]; then FLIPS'
$'cmp@@@meta difference ignored@@@[ "$(meta_get "$BM" "$k")" = "$(meta_get "$NM" "$k")" ] ||@@@true ||'
$'cmp@@@I-1 empty meta value accepted@@@[ -n "$v" ] && [[ "$v" != *$\'\\r\'* ]] || return 1; printf@@@true || return 1; printf'
$'cmp@@@M-7 CR in meta value accepted@@@[ -n "$v" ] && [[ "$v" != *$\'\\r\'* ]] || return 1; printf@@@[ -n "$v" ] || return 1; printf'
$'cmp@@@R1 duplicate meta key accepted@@@[ "$c" = 1 ] || return 1\n  v="$(grep "^$2=" "$1" | cut@@@[ "$c" -ge 1 ] || return 1\n  v="$(grep "^$2=" "$1" | cut'
$'cmp@@@image shape unchecked@@@[ "$bad" = 0 ] || return 1@@@true || return 1'
$'cmp@@@image presence unchecked@@@[ "$n" -ge 1 ] || return 1@@@true || return 1'
$'cmp@@@R9 new meta image check dropped@@@NSET="$(meta_images "$NM")" || incomparable@@@NSET="$(meta_images "$NM")" || : incomparable'
$'cmp@@@R10 baseline meta image check dropped@@@BSET="$(meta_images "$BM")" || incomparable@@@BSET="$(meta_images "$BM")" || : incomparable'
$'cmp@@@same-image check removed@@@[ "$NSET" != "$BSET" ] ||@@@true ||'
$'cmp@@@I-2 identity compared with version@@@awk \'{print $3}\' | sort; }@@@awk \'{print $2 $3}\' | sort; }'
$'cmp@@@unknown verdict word accepted@@@*) LV_ERR="unknown verdict \'$v\' in $f"; return 1 ;; esac@@@*) ;; esac'
$'cmp@@@I-7 verdict substring match@@@case "$v" in PASS|FAIL|INCONCLUSIVE|SKIPPED|BLIND|ERROR) ;;@@@case "$v" in *PASS*|*FAIL*|*INCONCLUSIVE*|*SKIPPED*|*BLIND*|*ERROR*) ;;'
$'cmp@@@duplicate id accepted@@@[ -z "${arr[$id]+x}" ] ||@@@true ||'
$'cmp@@@CR in verdict accepted@@@[[ "$line" == *$\'\\r\'* ]] && {@@@false && {'
$'cmp@@@REGRESSED exits 0@@@emit REGRESSED 10@@@emit REGRESSED 0'
$'cmp@@@INCOMPARABLE exits 0@@@emit INCOMPARABLE-BASELINE-MISSING 11@@@emit INCOMPARABLE-BASELINE-MISSING 0'
$'cmp@@@new tests not separated@@@[ -n "${BVD[$id]+x}" ] || NEWT=@@@true || NEWT='
$'cmp@@@new verdict sha not recorded@@@echo "new_verdicts_sha256=$(sha "$NV")"@@@echo "new_verdicts_sha256=none"'
$'cmp@@@M-1 write failure not fatal@@@write_record "$1" "$3" "$4" "$5" "$6" || blind@@@write_record "$1" "$3" "$4" "$5" "$6" || : blind'
$'cmp@@@BLIND leaves stale record@@@[ -z "$OUT" ] || printf \'schema@@@true || printf \'schema'
$'cmp@@@empty targets file accepted@@@[ "$ntg" -gt 0 ] || blind@@@true || blind'
$'cmp@@@M-4 diff lines not recorded@@@if [ "${BVD[$id]}" != "${NVD[$id]:-ABSENT}" ]; then echo "diff=@@@if false; then echo "diff='
$'cmp@@@M-4 flip lines not recorded@@@for id in ${FLIPS:-}; do echo "flip=@@@for id in; do echo "flip='
$'cmp@@@M-3 targets=absent not recorded@@@else echo "targets=absent"; fi@@@else :; fi'
$'gate@@@no live-run record not blocked@@@[ -r "$LR" ] || finish BLOCKED-NO-HARDWARE 20 "no live-run record"@@@:'
$'gate@@@not-connected accepted@@@[ "$(get "$HW" connected)" = yes ] ||@@@true ||'
$'gate@@@emulator identity accepted@@@[ "$(get "$HW" read_from)" = target ] ||@@@true ||'
$'gate@@@I-4 kind unchecked@@@[ "$(get "$HW" kind)" = physical ] ||@@@true ||'
$'gate@@@other product hardware accepted@@@[ "$(get "$HW" product)" = "$P" ] ||@@@true ||'
$'gate@@@incomplete live run accepted@@@[ "$(get "$LR" complete)" = yes ] ||@@@true ||'
$'gate@@@partial report accepted@@@"$tt" = "$tr" ]]@@@"$tt" -ge 0 ]]'
$'gate@@@R6 zero tests accepted@@@"$tt" -gt 0 &&@@@"$tt" -ge 0 &&'
$'gate@@@R7 empty target_id accepted@@@TID="$(get "$HW" target_id)" || finish@@@TID="$(get "$HW" target_id)" || : finish'
$'gate@@@I-3 live sha hex unchecked@@@&& [[ "$LSHA" =~ $HEX ]] ||@@@||'
$'gate@@@target build hex unchecked@@@if [ -z "$THASH" ] || [ "$(printf \'%s\\n\' "$THASH" | grep -cvE "$HEX")" != 0 ]; then@@@if false; then'
$'gate@@@comparison for other run accepted@@@[ "$CSHA" = "$LSHA" ] ||@@@true ||'
$'gate@@@I-4 board binding dropped@@@[ "$(get "$CR" board)" = "$TID" ] ||@@@true ||'
$'gate@@@I-4 suite binding dropped@@@[ "$(get "$CR" suite)" = "$LSUITE" ] ||@@@true ||'
$'gate@@@I-4 build-vs-image binding dropped@@@[ "$CIMG" = "$THASH" ] ||@@@true ||'
$'gate@@@regressed releasable@@@[ "$v" != REGRESSED ] ||@@@true ||'
$'gate@@@incomparable releasable@@@[ "$v" != INCOMPARABLE-BASELINE-MISSING ] ||@@@true ||'
$'gate@@@blocked exits 0@@@finish BLOCKED-NO-BASELINE 21 "no comparison record"@@@finish BLOCKED-NO-BASELINE 0 "no comparison record"'
$'gate@@@unknown verdict tolerated@@@*) die "unknown comparison verdict@@@*) : "unknown comparison verdict'
$'gate@@@gate record never written@@@} >"$OUT" 2>/dev/null@@@} >/dev/null 2>&1'
$'gate@@@I-5 die leaves stale record@@@die() { echo "BLIND: $*" >&2; record BLIND "$*";@@@die() { echo "BLIND: $*" >&2; :;'
$'gate@@@M-1 record write failure ignored@@@record "$1" "$3" || die "cannot write $OUT"@@@record "$1" "$3" || :'
$'gate@@@build identity not recorded@@@sed \'s/^image_new=/build_identity=/\'@@@sed \'s/^image_new=/zz=/\''
$'cmp@@@M-b unsafe id accepted (verdicts)@@@glob character)"; return 1 ;;@@@glob character)"; : ;;'
$'cmp@@@M-b unsafe id accepted (targets)@@@blind "unsafe target id@@@: "unsafe target id'
$'cmp@@@M-c out_clash disabled@@@o="$(realpath -m -- "$OUT" 2>/dev/null)" || return 0@@@o="__none__" || return 0'
$'cmp@@@M-c clash not refused@@@if out_clash; then OUT_BAD="$OUT"; OUT=""; blind@@@if false; then OUT_BAD="$OUT"; OUT=""; blind'
$'cmp@@@M-d duplicate image line accepted@@@[ "$(grep \'^image=\' "$1" | sort | uniq -d | grep -c \'\')" = 0 ] || return 1@@@true || return 1'
$'cmp@@@M-d same file twice accepted@@@[ "$(grep \'^image=\' "$1" | awk \'{print $1}\' | sort | uniq -d | grep -c \'\')" = 0 ] || return 1@@@true || return 1'
$'cmp@@@r1 image set not sorted@@@awk \'{print $3}\' | sort; }@@@awk \'{print $3}\'; }'
$'gate@@@r2 target hash not sorted@@@cut -d= -f2- | sort)"\nif@@@cut -d= -f2-)"\nif'
$'gate@@@r2 compared hash not sorted@@@awk \'{print $3}\' | sort)"\nif@@@awk \'{print $3}\')"\nif'
$'gate@@@I-6 schema unchecked@@@[ "$(get "$CR" schema)" = "image_no_worse/1" ] ||@@@true ||'
$'gate@@@I-6 truncated suite accepted@@@[ "$NT" = "$tt" ] && [ "$NT" = "$tr" ] ||@@@true ||'
$'gate@@@I-6 image_new shape unchecked@@@[ "$(grep \'^image_new=\' "$CR" | grep -cvE \'^image_new=[^ ]+ [^ ]+ [0-9a-f]{64}$\')" = 0 ] ||@@@true ||'
$'gate@@@M-d duplicate image_new accepted@@@[ "$(grep \'^image_new=\' "$CR" | sort | uniq -d | grep -c \'\')" = 0 ] ||@@@true ||'
$'gate@@@M-d same file twice accepted@@@[ "$(grep \'^image_new=\' "$CR" | awk \'{print $1}\' | sort | uniq -d | grep -c \'\')" = 0 ] ||@@@true ||'
$'gate@@@M-a lost/reasons/targets_missing unchecked@@@for k in reasons lost targets_missing; do@@@for k in reasons; do'
$'gate@@@M-a not-pass count unchecked@@@"$NNP" -le "$BNP" ]]@@@"$NNP" -ge 0 ]]'
$'gate@@@M-a PASS-lost diff unchecked@@@$2=="PASS" && $3!="PASS"@@@$2=="XX" && $3!="PASS"'
$'gate@@@M-c clash not refused@@@if out_clash; then OUT_BAD="$OUT"; die@@@if false; then OUT_BAD="$OUT"; die'
$'gate@@@M-c record clobbers input@@@if out_clash; then return 0; fi@@@if false; then return 0; fi'
)
# Equivalent (not run): row-count control needle (parser and grep -c cannot differ on accepted input); comparison
# sha hex check (it must equal the already hex-validated live sha); gate new_total integer regex (the equality with the
# validated tests_total catches it); the input snapshot (TOCTOU, not unit-testable: registered as follow-up WP03-6). Also: removing the explicit unreadable-targets / unreadable-new-meta guards - the follow-on
# guards (empty targets => BLIND; unreadable meta => missing key => INCOMPARABLE) give the same outcome.
mkdir -p "$ROOT/orig"; cp "$HERE/image_no_worse.sh" "$HERE/release_gate.sh" "$HERE/test_image_no_worse.sh" "$HERE/test_release_gate.sh" "$ROOT/orig/"
fail=0; n=0
for m in "${MUTANTS[@]}"; do
  n=$((n+1))
  if [ -n "${MUT_ONLY:-}" ] && [[ " $MUT_ONLY " != *" $n "* ]]; then continue; fi
  tool="${m%%@@@*}"; rest="${m#*@@@}"; name="${rest%%@@@*}"; rest="${rest#*@@@}"; old="${rest%%@@@*}"; new="${rest#*@@@}"
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
  bash -n "$d/$src" 2>/dev/null || { echo "BADMUTANT [$n] $name (mutant does not parse: a kill would be vacuous)"; fail=$((fail+1)); continue; }
  cmp -s "$HERE/$src" "$d/$src" && { echo "BADMUTANT [$n] $name (no change)"; fail=$((fail+1)); continue; }
  if env "$var=$d/$src" NOWORSE_TMP="$BASE" "$d/$suite" >"$d/out" 2>&1; then
    echo "SURVIVED [$n] $tool: $name"; fail=$((fail+1))
  else echo "killed   [$n] $tool: $name"; fi
done
# cmp-verified restore: the real sources are never mutated; prove it byte-for-byte
for f in image_no_worse.sh release_gate.sh test_image_no_worse.sh test_release_gate.sh; do
  cmp -s "$HERE/$f" "$ROOT/orig/$f" || { echo "RESTORE-FAIL $f changed during the run"; fail=$((fail+1)); }
done
echo "mutants=$n survivors_or_bad=$fail"
[ "$fail" = 0 ]
