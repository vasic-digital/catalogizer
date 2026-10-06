#!/usr/bin/env bash
# T053 - failing-first parser test of tools/evidence/wrap-cargo.sh. fixtures/cargo_*.txt are AUTHORED from the stable libtest text format
# (`test NAME ... ok|FAILED|ignored`, `test result:` lines); NO real cargo run exists here (IMG-RUST is WP-14), so they are NOT recorded
# outputs: UNCONFIRMED against a live cargo until the live leg (blocked-unavailable below) runs. The cut fixtures are line prefixes.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/wraplib.sh"; wsetup wrap-cargo.sh
wcheck "authored pass: 3 tests, 2 passed, 1 ignored"                0 --from-report "$FX/cargo_pass.txt" --rc 0 :: 'runner=cargo' 'status=pass' 'tests=3' 'passed=2' 'skipped=1'
wcheck "authored seeded failure"                                     1 --from-report "$FX/cargo_fail.txt" --rc 101 :: 'status=fail' 'failed=1' 'failed .*seeded_failure'
wcheck "hidden failure behind a lost summary is still found"        1 --from-report "$FX/cargo_fail_truncated.txt" --rc 101 :: 'status=fail' 'failed .*seeded_failure'
wcheck "a FAILED line before a later ok summary is still a failure" 1 --from-report "$FX/cargo_fail_last_ok.txt" --rc 0 :: 'status=fail' 'failed .*seeded_failure'
wcheck "a compile error is an error, not a test failure"            126 --from-report "$FX/cargo_build_fail.txt" --rc 101 :: 'status=error' 'reason=build_failed'
wcheck "running 0 tests is an error"                                 126 --from-report "$FX/cargo_none.txt" --rc 0 :: 'status=error' 'reason=no_tests'
wcheck "exit 101 with no FAILED line and no summary is a failure"   1 --from-report "$FX/cargo_pass.txt" --rc 101 :: 'status=fail' 'reason=exit_without_fail_line'
: >"$S/e.txt"; wcheck "an empty report is an error"                 126 --from-report "$S/e.txt" --rc 0 :: 'status=error' 'reason=empty_report'
wrefuse_r "no --run-token is refused (run_token_missing)" run_token_missing --from-report "$FX/cargo_pass.txt"
wrefuse_r "a malformed --run-token is refused (run_token_malformed)" run_token_malformed --run-token 0 --from-report "$FX/cargo_pass.txt"
o=$("$W" --run-token "$TOK" --from-report "$FX/cargo_pass.txt" 2>/dev/null); printf '%s\n' "$o" | grep -q "$TOK" && bad "the wrapper printed the run token" || ok "the wrapper does not print the run token"
echo "blocked-unavailable: live seeded-failure leg (IMG-RUST absent until WP-14; run on the remote host)"
wdone
