#!/usr/bin/env bash
# test_run_all_status_honesty.sh - register CAT-013: run-all-tests-comprehensive.sh must not print green for failing or
# skipped k6 / security stages, and main()'s final message must distinguish pass, pass-with-skips and fail.
# Drives the REAL script (sourced via its main-guard) in a mktemp sandbox. Substitutes are toolchain shims (k6, the security
# scan script) that exit with a chosen status: a test of the RUNNER's failure propagation (gate logic, 11.4.289(C)(2)),
# NOT evidence that performance or security pass.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
T="$(mktemp -d "${TMPDIR:-/tmp}/runall-status.XXXXXX")"; : "${T:?}"
trap 'rm -rf -- "${T:?}"' EXIT
FAILS=0
ok()  { echo "PASS: $1"; }
bad() { echo "FAIL: $1"; FAILS=$((FAILS+1)); }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }

BASH_BIN="$(command -v bash)"
SH="$T/shims"; mkdir -p "$SH"
printf '#!/usr/bin/env bash\nexit "${SHIM_K6_RC:-0}"\n' >"$SH/k6"; chmod +x "$SH/k6"
# minimal PATH without k6 for the tool_absent case
NOK6="$T/nok6"; mkdir -p "$NOK6"; for t in mkdir date cat; do ln -s "$(command -v $t)" "$NOK6/$t"; done
C="$T/cwd"; mkdir -p "$C/tests/performance" "$C/scripts"; : >"$C/tests/performance/load-test.js"

# stage: run one stage function; prints "OVERALL=<n> STATUS=<s>"
stage() { # $1 = function, $2 = status var, rest = env
  local fn=$1 var=$2; shift 2
  ( cd "$C" && env "$@" "$BASH_BIN" -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; '"$fn"' >"$2/stage.out" 2>&1; echo "OVERALL=$OVERALL_EXIT_CODE STATUS=${'"$var"'}"' _ "$REPO" "$T" )
}
P="PATH=$SH:$PATH"
check "perf: failing k6 sets OVERALL=1 and FAILED" "$(stage run_performance_tests PERF_STATUS "$P" SHIM_K6_RC=1)" "OVERALL=1 STATUS=FAILED"
grep -q 'Performance tests passed' "$T/stage.out" && bad "perf: printed passed for failing k6" || ok "perf: no green message for failing k6"
check "perf: passing k6 keeps OVERALL=0 and PASSED" "$(stage run_performance_tests PERF_STATUS "$P" SHIM_K6_RC=0)" "OVERALL=0 STATUS=PASSED"
check "perf: k6 absent is SKIPPED tool_absent" "$(stage run_performance_tests PERF_STATUS PATH="$NOK6")" "OVERALL=0 STATUS=SKIPPED (reason: tool_absent)"
grep -q 'Performance tests passed' "$T/stage.out" && bad "perf: printed passed on skip" || ok "perf: skip prints no passed message"

printf '#!/usr/bin/env bash\nexit 1\n' >"$C/scripts/security-scan-comprehensive.sh"; chmod +x "$C/scripts/security-scan-comprehensive.sh"
check "sec: failing scan sets OVERALL=1 and FAILED" "$(stage run_security_tests SEC_STATUS "$P")" "OVERALL=1 STATUS=FAILED"
grep -q 'Security tests passed\|Security tests completed' "$T/stage.out" && bad "sec: green message for failing scan" || ok "sec: no green message for failing scan"
printf '#!/usr/bin/env bash\nexit 0\n' >"$C/scripts/security-scan-comprehensive.sh"
check "sec: passing scan is PASSED" "$(stage run_security_tests SEC_STATUS "$P")" "OVERALL=0 STATUS=PASSED"
rm -f "$C/scripts/security-scan-comprehensive.sh"
check "sec: missing scan script is SKIPPED" "$(stage run_security_tests SEC_STATUS "$P")" "OVERALL=0 STATUS=SKIPPED (reason: script_not_found)"

# summary honesty
sumry() { ( cd "$C" && env "$@" "$BASH_BIN" -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; generate_test_summary >/dev/null 2>&1; cat "${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"' _ "$REPO" ); }
S="$(sumry PERF_STATUS=x)"; : "$S"
S="$( cd "$C" && bash -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; E2E_STATUS=PASSED; PERF_STATUS="SKIPPED (reason: tool_absent)"; SEC_STATUS=FAILED; OVERALL_EXIT_CODE=1; generate_test_summary >/dev/null 2>&1; cat "${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"' _ "$REPO" )"
grep -q '✅ Performance Tests' <<<"$S" && bad "summary ticks skipped perf" || ok "summary does not tick skipped perf"
grep -q '✅ Security Tests' <<<"$S" && bad "summary ticks failed security" || ok "summary does not tick failed security"
grep -q 'Security Tests: FAILED' <<<"$S" && ok "summary names FAILED security" || bad "summary lacks security status"

# main() final-message branches (run_* replaced by status setters; the branch logic under test is main's own)
mainrun() { # $1 = E2E $2 = PERF $3 = SEC $4 = overall ; prints message tail + rc
  ( cd "$C" && M_E2E="$1" M_PERF="$2" M_SEC="$3" M_OVERALL="$4" bash -c '
      source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1
      run_go_unit_tests(){ :; }; run_go_integration_tests(){ :; }; run_ts_unit_tests(){ :; }; run_contract_tests(){ :; }; run_linting(){ :; }; run_build_tests(){ :; }; generate_test_summary(){ :; }
      run_e2e_tests(){ E2E_STATUS="$M_E2E"; }; run_performance_tests(){ PERF_STATUS="$M_PERF"; }; run_security_tests(){ SEC_STATUS="$M_SEC"; OVERALL_EXIT_CODE="$M_OVERALL"; }
      main' _ "$REPO" 2>&1; echo "RC=$?" ) | sed 's/\x1b\[[0-9;]*m//g'
}
O="$(mainrun PASSED PASSED PASSED 0)"
grep -q 'All tests passed successfully!' <<<"$O" && grep -q '^RC=0' <<<"$O" && ok "main: all PASSED -> success message, rc 0" || bad "main: pass branch: $O"
O="$(mainrun PASSED 'SKIPPED (reason: tool_absent)' PASSED 0)"
grep -q 'not a full pass' <<<"$O" && ! grep -q 'All tests passed successfully' <<<"$O" && grep -q '^RC=0' <<<"$O" && ok "main: perf skipped -> pass-with-skips message, rc 0" || bad "main: skip branch: $O"
O="$(mainrun 'SKIPPED (reason: credentials_absent)' PASSED PASSED 0)"
grep -q 'not a full pass' <<<"$O" && ok "main: e2e skipped -> pass-with-skips message" || bad "main: e2e skip branch: $O"
O="$(mainrun PASSED PASSED FAILED 1)"
grep -q 'Some tests failed!' <<<"$O" && ! grep -q 'All tests passed successfully' <<<"$O" && grep -q '^RC=1' <<<"$O" && ok "main: failure -> failed message, rc 1" || bad "main: fail branch: $O"

echo "failures: $FAILS"; [ "$FAILS" -eq 0 ]
