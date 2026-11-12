#!/usr/bin/env bash
# Test suite for image_no_worse.sh (constitution 11.4.293 comparator).
# CLASS: gate-logic unit tests (11.4.289(C)(2)). The verdict/meta files below are fixture INPUT
# data for the comparator; this suite proves the comparator decides correctly on a declared input.
# It is NOT evidence that any real image is no worse. CMP=<path> runs against another copy.
# shellcheck disable=SC2015  # A && ok || bad is intentional (ok never fails)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CMP="${CMP:-$HERE/image_no_worse.sh}"
BASE="${NOWORSE_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/nowc-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/test.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/test.*) ;; *) echo "odd ROOT" >&2; exit 2 ;; esac
SAFE_RM="$HERE/../../constitution/scripts/lib/safe_rm.sh"
[ -r "$SAFE_RM" ] && . "$SAFE_RM"
cleanup() { if declare -F safe_rm_rf >/dev/null; then safe_rm_rf "${ROOT:?}"; else echo "note: safe_rm.sh unavailable; ${ROOT:?} left in place" >&2; fi; }
trap cleanup EXIT
PASS=0; FAIL=0
HA="$(printf 'a%.0s' {1..64})"; HB="$(printf 'b%.0s' {1..64})"; HC="$(printf 'c%.0s' {1..64})"
ok() { PASS=$((PASS+1)); }
bad() { FAIL=$((FAIL+1)); echo "  FAIL: $1"; }

meta() { # meta <file> <ver> <sha> [board] [harness] [suite] [command]
  printf 'board=%s\nharness=%s\nsuite=%s\ncommand=%s\nimage=system.img %s %s\n' \
    "${4:-orangepi-5}" "${5:-h1}" "${6:-full}" "${7:-run_suite.sh --all}" "$2" "$3" >"$1"; }
vd() { local f="$1" kv; shift; : >"$f"; for kv in "$@"; do printf '%s\t%s\n' "${kv%%=*}" "${kv#*=}" >>"$f"; done; }

setup() { D="$(mktemp -d "$ROOT/c.XXXXXX")"
  meta "$D/bm" 1.0 "$HA"; meta "$D/nm" 1.1 "$HB"
  vd "$D/b" t1=PASS t2=PASS t3=FAIL
  vd "$D/n" t1=PASS t2=PASS t3=PASS t4=PASS; }
run() { "$CMP" --baseline-verdicts "$D/b" --new-verdicts "$D/n" --baseline-meta "$D/bm" --new-meta "$D/nm" --out "$D/rec" "$@"; }
# expect <label> <rc> <needle-in-output> [args...]
expect() { local label="$1" rc="$2" needle="$3"; shift 3; local out got
  out="$(run "$@" 2>&1)"; got=$?
  [ "$got" = "$rc" ] || { bad "$label: rc=$got want $rc ($out)"; return; }
  printf '%s\n' "$out" | grep -qF -- "$needle" || { bad "$label: output lacks '$needle'"; return; }
  ok; }

# golden-good: all baseline PASS stay PASS, t3 improves, new t4 reported separately
setup; echo "t3" >"$D/tg"
expect "golden-good" 0 "VERDICT: NO-WORSE-PROVEN" --targets "$D/tg"
grep -q '^new_tests=t4$' "$D/rec" && ok || bad "new tests not reported separately"
grep -q "^baseline_verdicts_sha256=$(sha256sum "$D/b" | cut -d' ' -f1)$" "$D/rec" && ok || bad "record lacks baseline sha"
grep -q "^new_verdicts_sha256=$(sha256sum "$D/n" | cut -d' ' -f1)$" "$D/rec" && ok || bad "record lacks new sha"
grep -qx "image_new=system.img 1.1 $HB" "$D/rec" && ok || bad "record lacks image identity"
# control needle: counts equal independently known counts (3 baseline tests, 2 PASS)
grep -q '^baseline_total=3$' "$D/rec" && grep -q '^baseline_pass=2$' "$D/rec" && ok || bad "control needle: counts wrong"

# golden-bad: each non-PASS flavour on a baseline-PASS test is a regression
for v in FAIL INCONCLUSIVE SKIPPED BLIND ERROR; do
  setup; vd "$D/n" t1=PASS t2=$v t3=PASS
  expect "t2=$v regresses" 10 "baseline-pass-lost"
done
setup; vd "$D/n" t1=PASS t3=PASS   # t2 absent
expect "absent baseline-PASS test regresses" 10 "baseline-pass-lost"
setup; vd "$D/n" t1=FAIL t2=PASS t3=FAIL
expect "issue-count-higher listed" 10 "issue-count-higher"
setup; vd "$D/n" t1=PASS t2=PASS t3=FAIL; echo t3 >"$D/tg"
expect "targeted-flip-missing (still FAIL)" 10 "targeted-flip-missing" --targets "$D/tg"
setup; echo t9 >"$D/tg"
expect "targeted id absent => missing" 10 "targeted-flip-missing" --targets "$D/tg"
setup; echo t1 >"$D/tg"
expect "target already PASS in baseline is not a flip" 10 "targeted-flip-missing" --targets "$D/tg"

# INCOMPARABLE
setup; rm "$D/b"
expect "baseline missing" 11 "INCOMPARABLE-BASELINE-MISSING"
setup; : >"$D/b"
expect "baseline empty" 11 "INCOMPARABLE-BASELINE-MISSING"
setup; rm "$D/bm"
expect "baseline meta missing" 11 "INCOMPARABLE-BASELINE-MISSING"
setup; meta "$D/nm" 1.1 "$HB" other-board
expect "different board" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.1 "$HB" orangepi-5 h2
expect "different harness" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.1 "$HB" orangepi-5 h1 other
expect "different suite" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.1 "$HB" orangepi-5 h1 full "run_suite.sh --some"
expect "different command" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.0 "$HA"
expect "same image identity as baseline" 11 "INCOMPARABLE"

# malformed input => exit 2, never a pass
setup; printf 't1\tMAYBE\n' >"$D/n"
expect "unknown verdict word" 2 "BLIND"
setup; printf 't1\tPASS\nt1\tFAIL\n' >"$D/n"
expect "duplicate id" 2 "BLIND"
setup; printf 't1 PASS\n' >"$D/n"
expect "not TSV" 2 "BLIND"
setup; : >"$D/n"
expect "new verdicts empty" 2 "BLIND"
setup; rm "$D/n"
expect "new verdicts missing" 2 "BLIND"
setup; printf 'board=x\n' >"$D/nm"
expect "new meta incomplete" 11 "INCOMPARABLE"
setup; "$CMP" --bogus >/dev/null 2>&1; [ $? = 2 ] && ok || bad "unknown option must exit 2"
setup; printf 't1\tPASS\r\nt2\tPASS\r\nt3\tPASS\r\n' >"$D/n"
expect "CR in verdict file" 2 "BLIND"
setup; printf 't1\r\tPASS\nt2\tPASS\nt3\tPASS\n' >"$D/n"
expect "CR inside test id" 2 "BLIND"
setup; vd "$D/n" t1=FAIL t2=PASS t3=PASS; if run >/dev/null 2>&1; then bad "REGRESSED exited 0"; else ok; fi

# ---- review round 1 (I-1..I-5, M-1..M-4, M-6) ----
metaraw() { printf '%s\n' "$@" >"$2"; }
for k in board harness suite command; do
  setup; sed -i "s/^$k=.*/$k=/" "$D/bm"; sed -i "s/^$k=.*/$k=/" "$D/nm"
  expect "empty $k= on both sides" 11 "INCOMPARABLE"
  setup; sed -i "/^$k=/d" "$D/nm"; expect "missing $k= in new meta" 11 "INCOMPARABLE"
  setup; sed -i "/^$k=/d" "$D/bm"; expect "missing $k= in baseline meta" 11 "INCOMPARABLE"
  setup; echo "$k=dup" >>"$D/nm"; expect "duplicate $k= (R1)" 11 "INCOMPARABLE"
  setup; v="$(grep "^$k=" "$D/nm" | cut -d= -f2-)"; echo "$k=$v" >>"$D/nm"; echo "$k=$v" >>"$D/bm"; expect "duplicate identical $k= on both sides (R1)" 11 "INCOMPARABLE"
done
setup; sed -i 's/^board=.*/board=orangepi-5\r/' "$D/nm"; expect "CR in meta value" 11 "INCOMPARABLE"
setup; sed -i 's/^board=.*/board=orangepi-5\r/' "$D/nm" "$D/bm"; expect "CR in meta value on both sides (M-7)" 11 "INCOMPARABLE"
setup; sed -i 's/^image=.*/image=foo/' "$D/nm"; sed -i 's/^image=.*/image=bar/' "$D/bm"
expect "image=foo vs image=bar" 11 "INCOMPARABLE"
setup; sed -i '/^image=/d' "$D/nm"; expect "new meta without image= (R9)" 11 "INCOMPARABLE"
setup; sed -i '/^image=/d' "$D/bm"; expect "baseline meta without image= (R10)" 11 "INCOMPARABLE"
setup; sed -i 's/^image=.*/image=system.img 1.1 xyz/' "$D/nm"; expect "short sha in new image" 11 "INCOMPARABLE"
setup; sed -i "s/^image=.*/image=system.img 1.1 ${HB^^}/" "$D/nm"; expect "uppercase sha not accepted" 11 "INCOMPARABLE"
setup; echo "image=extra.img 1.1 foo" >>"$D/nm"; expect "one bad image= line among good ones" 11 "INCOMPARABLE"
setup; meta "$D/nm" 9.9 "$HA"; expect "same sha256 relabelled version (I-2)" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.1 "$HB"; printf 'image=boot.img 1.1 %s\n' "$HC" >>"$D/nm"; meta "$D/bm" 1.0 "$HA"; printf 'image=boot.img 1.0 %s\n' "$HC" >>"$D/bm"; sed -i 's/^image=boot.img 1.1 .*/image=boot.img 1.1 '"$HC"'/' "$D/nm"
expect "different sets sharing one file sha is comparable" 0 "NO-WORSE-PROVEN"
# M-7/I-5: BLIND and INCOMPARABLE paths rewrite --out (no stale record)
setup; echo "verdict=NO-WORSE-PROVEN" >"$D/rec"; printf 't1\tMAYBE\n' >"$D/n"; run >/dev/null 2>&1
grep -qx 'verdict=BLIND' "$D/rec" && ok || bad "BLIND exit left a stale record"
setup; echo "verdict=NO-WORSE-PROVEN" >"$D/rec"; sed -i '/^image=/d' "$D/nm"; run >/dev/null 2>&1
grep -qx 'verdict=INCOMPARABLE-BASELINE-MISSING' "$D/rec" && ok || bad "INCOMPARABLE left stale record"
setup; echo "verdict=NO-WORSE-PROVEN" >"$D/rec"; "$CMP" --bogus --out "$D/rec" >/dev/null 2>&1
grep -qx 'verdict=BLIND' "$D/rec" && ok || bad "usage error left stale record"
# M-1: unwritable --out fails closed, never prints NO-WORSE-PROVEN
setup; out="$("$CMP" --baseline-verdicts "$D/b" --new-verdicts "$D/n" --baseline-meta "$D/bm" --new-meta "$D/nm" --out "$D/nodir/rec" 2>&1)"; rc=$?
if [ "$rc" = 2 ] && ! printf '%s' "$out" | grep -q 'VERDICT: NO-WORSE-PROVEN'; then ok; else bad "unwritable --out: rc=$rc"; fi
# M-1b: write failure after a successful open (/dev/full) must fail closed
setup; out="$("$CMP" --baseline-verdicts "$D/b" --new-verdicts "$D/n" --baseline-meta "$D/bm" --new-meta "$D/nm" --out /dev/full 2>&1)"; rc=$?
if [ "$rc" = 2 ] && ! printf '%s' "$out" | grep -q 'VERDICT: NO-WORSE-PROVEN'; then ok; else bad "--out /dev/full: rc=$rc"; fi
# M-2: unreadable targets
setup; expect "unreadable --targets file (R8)" 2 "BLIND" --targets "$D/nonexistent"
setup; : >"$D/tg"; expect "empty --targets file" 2 "BLIND" --targets "$D/tg"
# M-3: targets=absent
setup; run >/dev/null; grep -qx 'targets=absent' "$D/rec" && ok || bad "targets=absent not recorded"
# M-4: per-test diff, flips, new-test verdicts, paths
setup; echo t3 >"$D/tg"; run --targets "$D/tg" >/dev/null
grep -qx 't3_placeholder' /dev/null; grep -qx 'diff=t3 FAIL PASS' "$D/rec" && ok || bad "diff line missing"
grep -qx 'flip=t3 FAIL PASS' "$D/rec" && ok || bad "flip line missing"
grep -qx 'new_test_verdict=t4 PASS' "$D/rec" && ok || bad "new test verdict missing"
grep -qx "baseline_verdicts_path=$D/b" "$D/rec" && grep -qx "new_verdicts_path=$D/n" "$D/rec" && ok || bad "verdict paths missing"
# R5: equal not-PASS count is not issue-count-higher (loss offset by gain, count equal)
setup; vd "$D/b" t1=PASS t2=FAIL; vd "$D/n" t1=FAIL t2=PASS
expect "equal count: only baseline-pass-lost" 10 "reasons: baseline-pass-lost lost="

# ---- review round 3 ----
# I-7: verdict closed set is an exact match, not a substring
setup; printf 't1\tPASS\nt2\tSKIPPED BLIND\nt3\tFAIL\n' >"$D/n"
expect "I-7 new multi-word verdict is BLIND (new side)" 2 "BLIND"
setup; printf 't1\tPASS\nt2\tSKIPPED BLIND\nt3\tFAIL\n' >"$D/b"; printf 't1\tPASS\nt2\tERROR\nt3\tFAIL\n' >"$D/n"
expect "I-7 reviewer scenario baseline 'SKIPPED BLIND' vs new ERROR" 11 "INCOMPARABLE"
setup; printf 't1\tPASS\nt2\tSKIPPED BLIND\nt3\tFAIL\n' >"$D/b"
expect "I-7 baseline multi-word verdict is INCOMPARABLE" 11 "INCOMPARABLE"
setup; printf 't1\tPASS\nt2\tPASS FAIL\nt3\tFAIL\n' >"$D/n"; expect "I-7 'PASS FAIL' on new side" 2 "BLIND"
setup; printf 't1\tpass\nt2\tPASS\nt3\tFAIL\n' >"$D/n"; expect "I-7 lowercase verdict" 2 "BLIND"
setup; printf 't1\tPASS \nt2\tPASS\nt3\tFAIL\n' >"$D/b"; expect "I-7 trailing space verdict (baseline)" 11 "INCOMPARABLE"
setup; printf 't1\tPASS\t\nt2\tPASS\nt3\tFAIL\n' >"$D/n"; expect "I-7 extra empty third field" 2 "BLIND"
# M-b: ids with comma, space or glob characters fail closed, never NO-WORSE-PROVEN
for badid in 'a,b' 'a b' 'a*' 'a?' 'a[1]'; do
  setup; printf '%s\tPASS\nt2\tPASS\nt3\tFAIL\n' "$badid" >"$D/n"; printf 't1\tPASS\nt2\tPASS\nt3\tFAIL\n' >"$D/b"
  out="$(run 2>&1)"; rc=$?
  { [ "$rc" = 2 ] && ! grep -q 'NO-WORSE-PROVEN' "$D/rec"; } && ok || bad "M-b new id '$badid': rc=$rc"
  setup; printf '%s\tPASS\nt2\tPASS\nt3\tFAIL\n' "$badid" >"$D/b"
  out="$(run 2>&1)"; rc=$?
  { [ "$rc" = 11 ] && ! grep -q 'NO-WORSE-PROVEN' "$D/rec"; } && ok || bad "M-b baseline id '$badid': rc=$rc"
  setup; printf '%s\n' "$badid" >"$D/tg"; out="$(run --targets "$D/tg" 2>&1)"; rc=$?
  { [ "$rc" = 2 ] && ! grep -q 'NO-WORSE-PROVEN' "$D/rec"; } && ok || bad "M-b target id '$badid': rc=$rc"
done
# M-c: --out may not be an input (realpath), input stays intact
for f in b n bm nm tg; do setup; echo t3 >"$D/tg"; cp "$D/$f" "$D/$f.orig"
  "$CMP" --baseline-verdicts "$D/b" --new-verdicts "$D/n" --baseline-meta "$D/bm" --new-meta "$D/nm" --targets "$D/tg" --out "$D/$f" >/dev/null 2>&1; rc=$?
  { [ "$rc" = 2 ] && cmp -s "$D/$f" "$D/$f.orig"; } && ok || bad "--out == $f: rc=$rc or input clobbered"; done
setup; ln -s "$D/n" "$D/lnk"; cp "$D/n" "$D/n.orig"
"$CMP" --baseline-verdicts "$D/b" --new-verdicts "$D/n" --baseline-meta "$D/bm" --new-meta "$D/nm" --out "$D/lnk" >/dev/null 2>&1; rc=$?
{ [ "$rc" = 2 ] && cmp -s "$D/n" "$D/n.orig"; } && ok || bad "--out symlink to input: rc=$rc"
# M-d: duplicate image lines / same file with two shas
setup; printf 'image=system.img 1.1 %s\n' "$HB" >>"$D/nm"; expect "M-d duplicate image line" 11 "INCOMPARABLE"
setup; printf 'image=system.img 1.1 %s\n' "$HC" >>"$D/nm"; expect "M-d same file two shas" 11 "INCOMPARABLE"
setup; printf 'image=system.img 1.0 %s\n' "$HC" >>"$D/bm"; expect "M-d same file two shas (baseline)" 11 "INCOMPARABLE"
# r1: multi-file image set, reordered self-compare is INCOMPARABLE (sort must stay)
setup; printf 'image=boot.img 1.1 %s\n' "$HC" >>"$D/nm"
{ printf 'board=orangepi-5\nharness=h1\nsuite=full\ncommand=run_suite.sh --all\n'; printf 'image=boot.img 1.1 %s\nimage=system.img 1.1 %s\n' "$HC" "$HB"; } >"$D/bm"
expect "r1 reordered multi-file self-compare" 11 "INCOMPARABLE"
# multi-file but genuinely different new image passes
setup; printf 'image=boot.img 1.1 %s\n' "$HC" >>"$D/nm"; expect "multi-file differing set" 0 "NO-WORSE-PROVEN"
# TOCTOU: record shas are those of the snapshot taken at start; the record verdict reflects the compared content
setup; run >/dev/null; grep -q "^new_verdicts_sha256=$(sha256sum "$D/n" | cut -d' ' -f1)$" "$D/rec" && ok || bad "snapshot sha differs from input sha"

# ---- WP03R3 follow-ups (F1..F4, F7) ----
# F1: --out that is a HARDLINK to any input must be refused and the input left intact
for f in b n bm nm tg; do setup; echo t3 >"$D/tg"; ln "$D/$f" "$D/hl"; cp "$D/$f" "$D/$f.orig"
  "$CMP" --baseline-verdicts "$D/b" --new-verdicts "$D/n" --baseline-meta "$D/bm" --new-meta "$D/nm" --targets "$D/tg" --out "$D/hl" >/dev/null 2>&1; rc=$?
  { [ "$rc" = 2 ] && cmp -s "$D/$f" "$D/$f.orig"; } && ok || bad "F1 hardlink --out to $f: rc=$rc or input clobbered"; done
# F1 class member: a usage error raised BEFORE all inputs are parsed must not clobber an input named earlier
for opt in --new-verdicts --baseline-verdicts --new-meta --baseline-meta --targets; do setup; echo t3 >"$D/tg"
  case "$opt" in --new-verdicts) f=n ;; --baseline-verdicts) f=b ;; --new-meta) f=nm ;; --baseline-meta) f=bm ;; --targets) f=tg ;; esac
  cp "$D/$f" "$D/$f.orig"; "$CMP" "$opt" "$D/$f" --out "$D/$f" --bogus >/dev/null 2>&1; rc=$?
  { [ "$rc" = 2 ] && cmp -s "$D/$f" "$D/$f.orig"; } && ok || bad "F1 early usage error clobbered $f (rc=$rc)"; done
# F2: newline/control characters in option or path text never duplicate record lines
NL=$'\n'
setup; "$CMP" "--bogus${NL}verdict=NO-WORSE-PROVEN" --out "$D/rec" >/dev/null 2>&1
{ [ "$(grep -c '^verdict=' "$D/rec")" = 1 ] && grep -qx 'verdict=BLIND' "$D/rec"; } && ok || bad "F2 newline in option text forged a verdict= line"
setup; vd "$D/n" t1=FAIL t2=PASS t3=PASS; cp "$D/b" "$D/b${NL}verdict=NO-WORSE-PROVEN"
"$CMP" --baseline-verdicts "$D/b${NL}verdict=NO-WORSE-PROVEN" --new-verdicts "$D/n" --baseline-meta "$D/bm" --new-meta "$D/nm" --out "$D/rec" >/dev/null 2>&1
{ [ "$(grep -c '^verdict=' "$D/rec")" = 1 ] && grep -qx 'verdict=REGRESSED' "$D/rec"; } && ok || bad "F2 newline in input path forged a verdict= line"
setup; "$CMP" --baseline-verdicts "$D/b${NL}verdict=NO-WORSE-PROVEN" --new-verdicts "$D/n" --baseline-meta "$D/bm" --new-meta "$D/nm" --out "$D/rec" >/dev/null 2>&1
[ "$(grep -c '^verdict=' "$D/rec")" = 1 ] && ok || bad "F2 newline in missing input path forged a verdict= line"
setup; "$CMP" "--bog$(printf '\033')us" --out "$D/rec" >/dev/null 2>&1
grep -q "$(printf '\033')" "$D/rec" && bad "F2 control char survived into record" || ok
# F3 / RM6: a target whose baseline verdict is not FAIL (SKIPPED, INCONCLUSIVE, BLIND, ERROR, absent) is no flip
for bv in SKIPPED INCONCLUSIVE BLIND ERROR; do setup; vd "$D/b" t1=PASS t2=PASS t3=$bv; echo t3 >"$D/tg"
  expect "RM6 baseline t3=$bv then PASS is not a flip" 10 "targeted-flip-missing" --targets "$D/tg"; done
setup; vd "$D/b" t1=PASS t2=PASS; echo t4 >"$D/tg"
expect "RM6 baseline-absent target then PASS is not a flip" 10 "targeted-flip-missing" --targets "$D/tg"
# F3 / RM4: multi-image sets where only the 2nd (sorted) sha differs
setup; meta "$D/bm" 1.0 "$HA"; printf 'image=boot.img 1.0 %s\n' "$HB" >>"$D/bm"
meta "$D/nm" 1.1 "$HA"; printf 'image=boot.img 1.1 %s\n' "$HB" >>"$D/nm"
expect "RM4 identical multi-image sets (different versions) are INCOMPARABLE" 11 "INCOMPARABLE"
setup; meta "$D/bm" 1.0 "$HA"; printf 'image=boot.img 1.0 %s\n' "$HB" >>"$D/bm"
meta "$D/nm" 1.1 "$HA"; printf 'image=boot.img 1.1 %s\n' "$HC" >>"$D/nm"
expect "RM4 only the 2nd sha differs is comparable" 0 "NO-WORSE-PROVEN"
setup; meta "$D/bm" 1.0 "$HA"; printf 'image=boot.img 1.0 %s\n' "$HB" >>"$D/bm"
meta "$D/nm" 1.1 "$HA"; printf 'image=boot.img 1.1 %s\nimage=extra.img 1.1 %s\n' "$HB" "$HC" >>"$D/nm"
expect "RM4 superset new set is comparable" 0 "NO-WORSE-PROVEN"
# F7: duplicate target id yields one flip= line
setup; printf 't3\nt3\n' >"$D/tg"; run --targets "$D/tg" >/dev/null
[ "$(grep -c '^flip=t3 ' "$D/rec")" = 1 ] && ok || bad "F7 duplicate target id duplicated flip= lines"
setup; printf 't9\nt9\n' >"$D/tg"; run --targets "$D/tg" >/dev/null 2>&1
grep -qx 'targets_missing=t9' "$D/rec" && ok || bad "F7 duplicate missing target listed twice"
# F4: the snapshot dir is removed on exit without recursive delete (TMPDIR probe) and the source has no rm -rf
setup; mkdir "$D/tmp"; TMPDIR="$D/tmp" run >/dev/null 2>&1
[ -z "$(ls -A "$D/tmp")" ] && ok || bad "F4 snapshot dir left behind"
setup; mkdir "$D/tmp"; TMPDIR="$D/tmp" run --bogus >/dev/null 2>&1
[ -z "$(ls -A "$D/tmp")" ] && ok || bad "F4 snapshot dir left behind on usage error"
if grep -nE 'rm +-[a-zA-Z]*[rR]' "$CMP" >/dev/null; then bad "F4 tool still uses a recursive rm"; else ok; fi
grep -qE 'rm +-rf' <<<'rm -rf x' && ok || bad "F4 lint control needle blind"

echo "image_no_worse tests: pass=$PASS fail=$FAIL"
[ "$FAIL" = 0 ]
