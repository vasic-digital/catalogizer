#!/usr/bin/env bash
# T051 - failing-first test of tools/evidence/wrap-go.sh (docs/06 s17 step 2, s16 "runner reports success without running").
# Recorded legs: fixtures/go_*.json are REAL `go test -json -count=1` outputs recorded in IMG-GO (go1.25.14, fixtures/go_version.txt)
# from the seeded module fixtures/seeds/gomod; the *_truncated, *_summary_cut and *_started_only files are byte prefixes of go_bad.json.
# Live leg: needs `go` (run it inside IMG-GO through scripts/containers/run_go.sh); without go it prints a skip line, never a pass.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/wraplib.sh"; wsetup wrap-go.sh
R="--from-report"
wcheck "recorded pass: 3 tests, 2 passed, 1 skipped, exit 0"      0   $R "$FX/go_pass.json"  :: 'runner=go' 'status=pass' 'tests=3' 'passed=2' 'failed=0' 'skipped=1'
wcheck "recorded seeded failure is reported through the parser"   1   $R "$FX/go_bad.json"   :: 'status=fail' 'failed=3' 'failed .*TestSeededFailure' 'failed .*TestSubtests/hidden'
wcheck "hidden failure behind a CUT summary is still found"       1   $R "$FX/go_bad_summary_cut.json" :: 'status=fail' 'failed .*TestSeededFailure'
wcheck "hidden failure behind a TRUNCATED stream (last line cut mid-JSON) is still found" 1 $R "$FX/go_bad_truncated.json" :: 'status=fail' 'failed .*TestSeededFailure' 'truncated'
wcheck "a test that started and never finished is no pass and no failure verdict (126)" 126 $R "$FX/go_bad_started_only.json" :: 'status=error' 'reason=truncated_report' '!status=pass'
wcheck "a package that does not compile is an error, not a test failure" 126 $R "$FX/go_build.json" :: 'status=error' 'reason=build_failed'
wcheck "a package without test files ran zero tests: error"       126  $R "$FX/go_empty.json" :: 'status=error' 'reason=no_tests'
wcheck "a fake ok line printed by a failing test does not hide it" 1   $R "$FX/go_mixed.json" :: 'status=fail' 'failed .*TestNoisyThenFails'
: >"$S/empty.json"; wcheck "an empty report is an error"          126  $R "$S/empty.json" :: 'status=error' 'reason=empty_report'
printf 'this is not a report\n' >"$S/junk.json"; wcheck "an unparsable report is an error" 126 $R "$S/junk.json" :: 'status=error' 'reason=unparsable'
wcheck "--min-tests 4 turns a 3-test pass into an error"           126  $R "$FX/go_pass.json" --min-tests 4 :: 'status=error' 'reason=too_few_tests'
wrefuse_r "no --run-token is refused (run_token_missing)" run_token_missing                                 --from-report "$FX/go_pass.json"
wrefuse_r "a malformed --run-token is refused (run_token_malformed)" run_token_malformed                        --run-token xyz --from-report "$FX/go_pass.json"
wrefuse "an unreadable --from-report file is refused"               --run-token "$TOK" --from-report "$S/does-not-exist"
# the wrapper's stdout carries the raw report after the summary (the captured post-state of a state-delta test, T051a)
out=$("$W" --run-token "$TOK" $R "$FX/go_pass.json" 2>/dev/null); printf '%s\n' "$out" | grep -q '^wrap: raw-begin$' && printf '%s\n' "$out" | grep -q 'TestAdd' && ok "raw report follows the summary" || bad "raw report does not follow the summary"
# the token is never printed by the wrapper itself (the deriver must find it in what the TEST printed)
printf '%s\n' "$out" | grep -q "$TOK" && bad "the wrapper printed the run token" || ok "the wrapper does not print the run token"
# --- live leg (needs go): a seeded failing test, a passing one, a build failure, and EVREC_RUN_TOKEN reaching the test
if [ -z "${T051_SKIP_LIVE:-}" ] && command -v go >/dev/null 2>&1; then
  M=$S/gomod; mkdir -p "$M"; cp -r "$FX/seeds/gomod/." "$M/"
  cd "$M"; export GOTOOLCHAIN=local GOFLAGS=-mod=mod GOCACHE=$S/gocache GOPATH=$S/gopath HOME=$S/home; mkdir -p "$S/home"
  wcheck "live: passing package exits 0"                    0 --  ./good/  :: 'status=pass' 'tests=3'
  wcheck "live: seeded failing test exits 1 and is named"   1 --  ./bad/   :: 'status=fail' 'failed .*TestSeededFailure'
  wcheck "live: noisy package with a failure behind noise exits 1" 1 -- ./mixed/ :: 'status=fail' 'TestNoisyThenFails'
  wcheck "live: build failure exits 126"                     126 -- ./build/ :: 'status=error' 'reason=build_failed'
  wcheck "live: package without tests exits 126"             126 -- ./empty/ :: 'status=error' 'reason=no_tests'
  cat >"$M/good/token_test.go" <<'G'
package good

import (
	"os"
	"testing"
)

func TestTokenReachesTheTest(t *testing.T) {
	if len(os.Getenv("EVREC_RUN_TOKEN")) != 32 {
		t.Fatalf("EVREC_RUN_TOKEN not exported to the test: %q", os.Getenv("EVREC_RUN_TOKEN"))
	}
	t.Logf("post-state nonce %s", os.Getenv("EVREC_RUN_TOKEN"))
}
G
  wcheck "live: the wrapper exports EVREC_RUN_TOKEN to the test" 0 -- -run TestTokenReachesTheTest -v ./good/ :: 'status=pass' 'tests=1'
  o=$("$W" --run-token "$TOK" -- -run TestTokenReachesTheTest -v ./good/ 2>/dev/null); printf '%s' "$o" | grep -q "post-state nonce $TOK" && ok "live: the test's own output carries the token after the raw marker" || bad "live: token not in the test output"
cd "$S"
else echo "skip live leg: go is not in PATH or T051_SKIP_LIVE is set (11.4.173: the live leg compiles and runs go test, so it runs inside IMG-GO: scripts/containers/run_pinned.sh IMG-GO -- bash tools/evidence/tests/test_wrap_go.sh)"; fi
wdone
