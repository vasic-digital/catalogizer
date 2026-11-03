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
# PATH without k6: the real PATH with every directory that holds k6 replaced by a symlink mirror without it (robust against
# what the script sources: dirname, readlink, ...)
NOK6="$T/nok6"; mkdir -p "$NOK6"; NOK6_PATH=""
_i=0
IFS=: read -ra _dirs <<<"$PATH"
for _d in "${_dirs[@]}"; do
  [ -d "$_d" ] || continue
  if [ -e "$_d/k6" ]; then
    _i=$((_i+1)); _m="$NOK6/d$_i"; mkdir -p "$_m"
    for _f in "$_d"/*; do [ "${_f##*/}" = k6 ] || ln -s "$_f" "$_m/${_f##*/}" 2>/dev/null; done
    _d="$_m"
  fi
  NOK6_PATH="${NOK6_PATH:+$NOK6_PATH:}$_d"
done
C="$T/cwd"; mkdir -p "$C/tests/performance" "$C/scripts"; : >"$C/tests/performance/load-test.js"

# stage: run one stage function; prints "OVERALL=<n> STATUS=<s>"
stage() { # $1 = function, $2 = status var, rest = env
  rm -f -- "${T:?}/stage.out"
  local fn=$1 var=$2; shift 2
  ( cd "$C" && env "$@" "$BASH_BIN" -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; '"$fn"' >"$2/stage.out" 2>&1; echo "OVERALL=$OVERALL_EXIT_CODE STATUS=${'"$var"'}"' _ "$REPO" "$T" )
}
P="PATH=$SH:$PATH"
check "perf: failing k6 sets OVERALL=1 and FAILED" "$(stage run_performance_tests PERF_STATUS "$P" SHIM_K6_RC=1)" "OVERALL=1 STATUS=FAILED"
grep -q 'Performance tests passed' "$T/stage.out" && bad "perf: printed passed for failing k6" || ok "perf: no green message for failing k6"
check "perf: passing k6 keeps OVERALL=0 and PASSED" "$(stage run_performance_tests PERF_STATUS "$P" SHIM_K6_RC=0)" "OVERALL=0 STATUS=PASSED"
check "perf: k6 absent is SKIPPED tool_absent" "$(stage run_performance_tests PERF_STATUS PATH="$NOK6_PATH")" "OVERALL=0 STATUS=SKIPPED (reason: tool_absent)"
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

# F4: integration / contract failures are failures, not warnings; their summary lines are not ticked
G="$T/gobin"; mkdir -p "$G"
printf '#!/usr/bin/env bash\n[ "${SHIM_GO_FAIL_ON:-}" = "$*" ] && exit 1\nexit 0\n' >"$G/go"; chmod +x "$G/go"
mkdir -p "$C/catalog-api" "$C/contracts"
check "F4 integration: failing go test -tags=integration sets OVERALL=1 and FAILED" "$(stage run_go_integration_tests INT_STATUS PATH="$G:$PATH" SHIM_GO_FAIL_ON='test ./... -tags=integration -v')" "OVERALL=1 STATUS=FAILED"
check "F4 integration: passing run is PASSED" "$(stage run_go_integration_tests INT_STATUS PATH="$G:$PATH")" "OVERALL=0 STATUS=PASSED"
check "F4 contract: failing contract tests set OVERALL=1 and FAILED" "$(stage run_contract_tests CONTRACT_STATUS PATH="$G:$PATH" SHIM_GO_FAIL_ON='test ./...')" "OVERALL=1 STATUS=FAILED"
check "F4 contract: passing run is PASSED" "$(stage run_contract_tests CONTRACT_STATUS PATH="$G:$PATH")" "OVERALL=0 STATUS=PASSED"
rm -rf -- "${C:?}/contracts"
check "F4 contract: missing contracts/ is SKIPPED, not passed" "$(stage run_contract_tests CONTRACT_STATUS PATH="$G:$PATH")" "OVERALL=0 STATUS=SKIPPED (reason: script_not_found)"
S="$( cd "$C" && bash -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; GO_STATUS=PASSED; INT_STATUS=FAILED; TS_STATUS=PASSED; E2E_STATUS=PASSED; CONTRACT_STATUS="SKIPPED (reason: script_not_found)"; PERF_STATUS=PASSED; LINT_STATUS=PASSED; SEC_STATUS=PASSED; BUILD_STATUS=PASSED; OVERALL_EXIT_CODE=1; generate_test_summary >/dev/null 2>&1; cat "${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"' _ "$REPO" )"
grep -q '✅ Go Integration' <<<"$S" && bad "F4 summary ticks a failed integration stage" || ok "F4 summary does not tick a failed integration stage"
grep -q '✅ Contract' <<<"$S" && bad "F4 summary ticks a skipped contract stage" || ok "F4 summary does not tick a skipped contract stage"
grep -q '✅ Go Unit' <<<"$S" && grep -q '✅ Build Verification' <<<"$S" && ok "F4 golden-good rows still ticked" || bad "F4 golden-good rows lost their tick"
S="$( cd "$C" && bash -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; GO_STATUS=FAILED; OVERALL_EXIT_CODE=1; generate_test_summary >/dev/null 2>&1; cat "${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"' _ "$REPO" )"
grep -q '✅ Go Unit' <<<"$S" && bad "F4 summary ticks failed Go unit tests" || ok "F4 summary does not tick failed Go unit tests"

# F5: a skipped security stage alone is pass-with-skips in main()
O="$(mainrun PASSED PASSED 'SKIPPED (reason: script_not_found)' 0)"
grep -q 'not a full pass' <<<"$O" && grep -q '^RC=0' <<<"$O" && ok "F5 main: security skipped -> pass-with-skips message, rc 0" || bad "F5 main: sec skip branch: $O"

# R3-F3: go vet / ESLint / type-check in run_linting
lint() { # $1 = go fail-on args, $2 = npm fail-on subcommand ; prints OVERALL and LINT_STATUS
  rm -f -- "${T:?}/stage.out"
  ( cd "$C" && env PATH="$G:$SH:$PATH" SHIM_GO_FAIL_ON="$1" SHIM_NPM_FAIL_ON="$2" "$BASH_BIN" -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; run_linting >"$2/stage.out" 2>&1; echo "OVERALL=$OVERALL_EXIT_CODE STATUS=$LINT_STATUS"' _ "$REPO" "$T" )
}
printf '#!/usr/bin/env bash\n[ -n "${SHIM_NPM_FAIL_ON:-}" ] && { [ "${2:-}" = "$SHIM_NPM_FAIL_ON" ] || [ "${1:-}" = "$SHIM_NPM_FAIL_ON" ]; } && exit 1\nexit 0\n' >"$SH/npm"; chmod +x "$SH/npm"
mkdir -p "$C/catalog-web"
check "R3-F3 lint: go vet failure sets OVERALL=1 and a FAILED row" "$(lint 'vet ./...' '')" "OVERALL=1 STATUS=FAILED (go vet)"
check "R3-F3 lint: type-check failure -> FAILED" "$(lint '' 'type-check')" "OVERALL=1 STATUS=FAILED"
check "R3-F3 lint: ESLint issues alone are surfaced in the row, not fatal" "$(lint '' 'lint')" "OVERALL=0 STATUS=PASSED (eslint issues)"
check "R3-F3 lint: golden-good -> PASSED" "$(lint '' '')" "OVERALL=0 STATUS=PASSED"
S="$( cd "$C" && bash -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; LINT_STATUS="FAILED (go vet)"; generate_test_summary >/dev/null 2>&1; cat "${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"' _ "$REPO" )"
grep -q '✅ Linting' <<<"$S" && bad "R3-F3 summary ticks a failed lint row" || ok "R3-F3 summary does not tick a failed lint row"
S="$( cd "$C" && bash -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; LINT_STATUS="PASSED (eslint issues)"; generate_test_summary >/dev/null 2>&1; cat "${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"' _ "$REPO" )"
grep -q '✅ Linting.*eslint issues' <<<"$S" && ok "R3-F3 summary ticks a PASSED (eslint issues) row and says so" || bad "R3-F3 eslint row wrong"
# R3-F8: Status line and final warning name every skipped stage, including contract
S="$( cd "$C" && bash -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; CONTRACT_STATUS="SKIPPED (reason: script_not_found)"; PERF_STATUS=PASSED; generate_test_summary >/dev/null 2>&1; cat "${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"' _ "$REPO" )"
grep -q 'PASSED WITH SKIPS (CONTRACT: SKIPPED (reason: script_not_found))' <<<"$S" && ok "R3-F8 Status line names the skipped contract stage" || bad "R3-F8 status line: $(grep Status <<<"$S")"
echo "failures: $FAILS"; [ "$FAILS" -eq 0 ]
