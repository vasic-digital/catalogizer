#!/usr/bin/env bash
# test_scaling_live_classify.sh - gate-logic test of the verdict classification in scaling_live.sh
# (--classify mode). The logs below are INPUTS to the real classifier, not stand-ins for the live
# system (this tests how a go-test log is turned into an exit code; the live run itself is
# scripts/test-infra/scaling_live.sh). Exit 0 = every case behaved.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUT="$HERE/scaling_live.sh"
W="$(mktemp -d "${TMPDIR:-/tmp}/scaling_classify.XXXXXX")" || exit 1
trap 'rm -rf -- "${W:?}"' EXIT
fails=0
check() { # name expected_code log_content go_rc
  local name="$1" want="$2" content="$3" rc="$4" got
  printf '%s' "$content" >"$W/log"
  bash "$SUT" --classify "$W/log" "$rc" >/dev/null 2>&1; got=$?
  if [ "$got" -eq "$want" ]; then echo "ok   $name -> $got"; else echo "FAIL $name: want $want got $got"; fails=$((fails+1)); fi
}
P=$'--- PASS: TestLiveScaling (10.00s)\n--- PASS: TestLiveStress (5.00s)\n'
S=$'--- SKIP: TestLiveChaosKillRestart (0.10s)\n'
F=$'--- FAIL: TestLiveStress (1.00s)\n'
B=$'--- FAIL: TestLiveScaling (0.50s)\n    scaling_live_test.go:93: FAIL: cannot build catalog-api (the tree under test does not compile)\n'
check "all pass"                         0  "$P"        0
check "pass + skip is PARTIAL (78)"      78 "$P$S"      0
check "only skips is SKIP (77)"          77 "$S"        0
check "a failed test is FAIL"            1  "$P$F"      1
check "pass + skip + fail is FAIL"       1  "$P$S$F"    1
check "build failure is FAIL not SKIP"   1  "$B"        1
check "non-zero go rc without FAIL line" 1  "$P"        2
check "empty log is FAIL"                1  ""          0
check "fail line with rc 0 is FAIL"      1  "$P$F"      0
bash "$SUT" --classify >/dev/null 2>&1; [ $? -eq 64 ] && echo "ok   usage -> 64" || { echo "FAIL usage"; fails=$((fails+1)); }
[ "$fails" -eq 0 ] && echo "ALL OK" || { echo "$fails case(s) failed"; exit 1; }
