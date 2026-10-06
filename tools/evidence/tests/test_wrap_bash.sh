#!/usr/bin/env bash
# T051 - failing-first test of tools/evidence/wrap-bash.sh. Recorded legs: fixtures/bash_*.out (+ .rc) are REAL outputs of the seeds in
# fixtures/seeds/bash_*.sh, run in IMG-KCOV (`scripts/containers/run_pinned.sh IMG-KCOV -- bash seed`), stdout+stderr merged, the
# runner exit status in the .rc file. Live leg: the wrapper runs the seeds itself (host bash or IMG-KCOV).
set -u
. "$(dirname "${BASH_SOURCE[0]}")/wraplib.sh"; wsetup wrap-bash.sh
rcof() { cat "$FX/$1.rc"; }
wcheck "recorded pass (PASS:, ok and SKIP: lines, summary agrees)" 0 --from-report "$FX/bash_pass.out" --rc "$(rcof bash_pass)" :: 'runner=bash' 'status=pass' 'tests=3' 'passed=2' 'failed=0' 'skipped=1'
wcheck "recorded seeded failure"                                1 --from-report "$FX/bash_fail.out" --rc "$(rcof bash_fail)" :: 'status=fail' 'failed=1' 'failed .*seeded failure'
wcheck "a FAIL line wins over a summary that says FAIL=0 and over exit 0" 1 --from-report "$FX/bash_liar.out" --rc "$(rcof bash_liar)" :: 'status=fail' 'failed=1' 'check 41 broke' 'summary_mismatch'
wcheck "death under set -e: no FAIL line, no summary, exit 1 is still a failure" 1 --from-report "$FX/bash_silent.out" --rc "$(rcof bash_silent)" :: 'status=fail' 'reason=exit_without_fail_line' '!status=pass'
wcheck "zero tests and exit 0 is an error, not a pass"           126 --from-report "$FX/bash_none.out" --rc "$(rcof bash_none)" :: 'status=error' 'reason=no_tests'
printf 'ok   a\nok   b\nSummary: PASS=3 FAIL=0 SKIP=0\n' >"$S/lost.out"
wcheck "a lost line (summary counts 3, two seen) is a failure"   1 --from-report "$S/lost.out" --rc 0 :: 'status=fail' 'reason=summary_mismatch'
printf 'PASS: a [evidence: x]\nPASS: b [evidence: y]\nSummary: PASS=2 FAIL=1 SKIP=0\n' >"$S/sumfail.out"
wcheck "a summary that reports a failure the lines do not show fails" 1 --from-report "$S/sumfail.out" --rc 1 :: 'status=fail'
printf 'not ok 3 - tap style failure\nok 1 - a\nok 2 - b\n' >"$S/tap.out"
wcheck "TAP style 'not ok' counts as a failure"                   1 --from-report "$S/tap.out" --rc 1 :: 'status=fail' 'failed=1' 'tap style failure'
printf 'ok 1 - a\nok 2 - b\n1..3\n' >"$S/plan.out"
wcheck "a TAP plan larger than the lines seen is a failure"       1 --from-report "$S/plan.out" --rc 0 :: 'status=fail' 'reason=plan_mismatch'
printf '\033[32mPASS: coloured [evidence: z]\033[0m\nSummary: PASS=1 FAIL=0 SKIP=0\n' >"$S/ansi.out"
wcheck "ANSI colour around a result line does not hide it"        0 --from-report "$S/ansi.out" --rc 0 :: 'status=pass' 'tests=1'
wcheck "--rc missing with --from-report is refused as unknown (no outcome claimed from lines alone when the exit is unknown)" 0 --from-report "$FX/bash_pass.out" :: 'status=pass' 'rc=unknown'
: >"$S/e.out"; wcheck "an empty report with exit 0 is an error"  126 --from-report "$S/e.out" --rc 0 :: 'status=error'
wrefuse_r "no --run-token is refused (run_token_missing)" run_token_missing --from-report "$FX/bash_pass.out"
wrefuse_r "a malformed --run-token is refused (run_token_malformed)" run_token_malformed --run-token abc --from-report "$FX/bash_pass.out"
# --- live leg: the wrapper runs the seeds (bash is present everywhere this test runs)
SD=$FX/seeds
wcheck "live: seeded pass"          0 -- "$SD/bash_pass.sh"   :: 'status=pass' 'tests=3'
wcheck "live: seeded failure"       1 -- "$SD/bash_fail.sh"   :: 'status=fail' 'failed=1'
wcheck "live: liar"                 1 -- "$SD/bash_liar.sh"   :: 'status=fail' 'failed=1'
wcheck "live: silent death"         1 -- "$SD/bash_silent.sh" :: 'status=fail' 'reason=exit_without_fail_line'
wcheck "live: no tests"             126 -- "$SD/bash_none.sh" :: 'status=error' 'reason=no_tests'
wrefuse "live: a command that does not exist is an error (126), never a test failure" --run-token "$TOK" -- "$S/no-such-script.sh"
grep -q 'reason=runner_not_executable' "$S/e" && ok "live: the refusal names runner_not_executable" || bad "live: reason not named"
cat >"$S/tok.sh" <<'G'
#!/usr/bin/env bash
[ "${#EVREC_RUN_TOKEN}" = 32 ] || { echo "FAIL: EVREC_RUN_TOKEN missing"; exit 1; }
echo "PASS: nonce $EVREC_RUN_TOKEN reached the test [evidence: env]"
G
chmod +x "$S/tok.sh"
wcheck "live: EVREC_RUN_TOKEN is exported to the test"            0 -- "$S/tok.sh" :: 'status=pass'
o=$("$W" --run-token "$TOK" -- "$S/tok.sh" 2>/dev/null); printf '%s' "$o" | sed '1,/^wrap: raw-begin$/d' | grep -q "nonce $TOK" && ok "live: the test's own output carries the token after the raw marker" || bad "live: token missing from the test output"
printf '%s' "$o" | sed '/^wrap: raw-begin$/,$d' | grep -q "$TOK" && bad "live: the wrapper itself printed the token" || ok "live: the wrapper does not print the token in its summary"
wdone
