#!/usr/bin/env bash
# T053 - failing-first parser test of tools/evidence/wrap-gradle.sh. fixtures/gradle_*/ are JUnit XML reports AUTHORED from the Ant/JUnit
# XML format that Gradle writes to build/test-results/test/TEST-*.xml; NO real Gradle run exists here (IMG-ANDROID is WP-14), so
# they are NOT recorded outputs: UNCONFIRMED against a live Gradle until the live leg (blocked-unavailable below) runs.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/wraplib.sh"; wsetup wrap-gradle.sh
wcheck "authored pass: 3 tests, 2 passed, 1 skipped"                0 --from-report "$FX/gradle_pass" :: 'runner=gradle' 'status=pass' 'tests=3' 'passed=2' 'skipped=1'
wcheck "authored seeded failure across two result files"            1 --from-report "$FX/gradle_fail" :: 'status=fail' 'tests=4' 'failed=1' 'failed .*seededFailure'
wcheck "a suite header that says failures=0 over a failing testcase: the testcase wins" 1 --from-report "$FX/gradle_header_lies" :: 'status=fail' 'failed=1' 'failed .*crashes'
wcheck "a truncated XML report still shows the failure"             1 --from-report "$FX/gradle_truncated" :: 'status=fail' 'failed .*seededFailure' 'truncated'
wcheck "a suite with zero tests is an error"                        126 --from-report "$FX/gradle_none" :: 'status=error' 'reason=no_tests'
mkdir -p "$S/emptydir"; wcheck "a directory without result files is an error" 126 --from-report "$S/emptydir" :: 'status=error' 'reason=no_reports'
mkdir -p "$S/one"; cp "$FX/gradle_pass/TEST-com.example.CalcTest.xml" "$S/one/"; wcheck "a single file report path works" 0 --from-report "$S/one/TEST-com.example.CalcTest.xml" :: 'status=pass' 'tests=3'
wrefuse_r "no --run-token is refused (run_token_missing)" run_token_missing --from-report "$FX/gradle_pass"
wrefuse_r "a malformed --run-token is refused (run_token_malformed)" run_token_malformed --run-token 123 --from-report "$FX/gradle_pass"
o=$("$W" --run-token "$TOK" --from-report "$FX/gradle_pass" 2>/dev/null); printf '%s\n' "$o" | grep -q "$TOK" && bad "the wrapper printed the run token" || ok "the wrapper does not print the run token"
echo "blocked-unavailable: live seeded-failure leg (IMG-ANDROID absent until WP-14; run on the remote host)"
wdone
