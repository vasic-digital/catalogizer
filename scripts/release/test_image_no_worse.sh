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
trap 'rm -rf "${ROOT:?}"' EXIT
PASS=0; FAIL=0
ok() { PASS=$((PASS+1)); }
bad() { FAIL=$((FAIL+1)); echo "  FAIL: $1"; }

meta() { # meta <file> <ver> <sha> [board] [harness] [suite] [command]
  printf 'board=%s\nharness=%s\nsuite=%s\ncommand=%s\nimage=system.img %s %s\n' \
    "${4:-orangepi-5}" "${5:-h1}" "${6:-full}" "${7:-run_suite.sh --all}" "$2" "$3" >"$1"; }
vd() { local f="$1" kv; shift; : >"$f"; for kv in "$@"; do printf '%s\t%s\n' "${kv%%=*}" "${kv#*=}" >>"$f"; done; }

setup() { D="$(mktemp -d "$ROOT/c.XXXXXX")"
  meta "$D/bm" 1.0 aaaa; meta "$D/nm" 1.1 bbbb
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
grep -q '^image_new=system.img 1.1 bbbb$' "$D/rec" && ok || bad "record lacks image identity"
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
setup; meta "$D/nm" 1.1 bbbb other-board
expect "different board" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.1 bbbb orangepi-5 h2
expect "different harness" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.1 bbbb orangepi-5 h1 other
expect "different suite" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.1 bbbb orangepi-5 h1 full "run_suite.sh --some"
expect "different command" 11 "INCOMPARABLE"
setup; meta "$D/nm" 1.0 aaaa
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
expect "new meta incomplete" 2 "BLIND"
setup; "$CMP" --bogus >/dev/null 2>&1; [ $? = 2 ] && ok || bad "unknown option must exit 2"
setup; printf 't1\tPASS\r\nt2\tPASS\r\nt3\tPASS\r\n' >"$D/n"
expect "CR in verdict file" 2 "BLIND"
setup; printf 't1\r\tPASS\nt2\tPASS\nt3\tPASS\n' >"$D/n"
expect "CR inside test id" 2 "BLIND"
setup; vd "$D/n" t1=FAIL t2=PASS t3=PASS; if run >/dev/null 2>&1; then bad "REGRESSED exited 0"; else ok; fi

echo "image_no_worse tests: pass=$PASS fail=$FAIL"
[ "$FAIL" = 0 ]
