# wrap-go.sh, wrap-bash.sh, wrap-vitest.sh, wrap-gradle.sh, wrap-cargo.sh, evparse.py: runner wrappers - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T00:00:00Z |
| Status | tracked from WP-05 T051 and T053; independent review owed (constitution 11.4.142, T058) |
| Source | `tools/evidence/wrap-go.sh`, `tools/evidence/wrap-bash.sh`, `tools/evidence/wrap-vitest.sh`, `tools/evidence/wrap-gradle.sh`, `tools/evidence/wrap-cargo.sh`, `tools/evidence/lib/wrap_common.sh`, `tools/evidence/evparse.py` |

> `$EV` means `specs/001-full-project-audit-remediation/evidence`.

## Purpose
A runner wrapper runs a test runner, judges its machine report with a parser, and exits with a status the recorder maps to a verdict: `0` pass, `1` at least one test failed, `126` NO TEST OUTCOME (usage error, runner absent, zero tests, build failure, a stream cut before any result could be confirmed). A wrapper never exits 64 or 127: `evrec` would read 64 as a failing assertion (1..125) and 127 hides the cause, and a RED that did not compile or ran zero tests must not count as a genuine RED (docs/06 section 16, "runner reports success without running").

## Usage
```
wrap-go.sh     --run-token TOKEN [opts] -- GO_TEST_ARGS...        go test -json -count=1 ARGS   (never a cached result)
wrap-bash.sh   --run-token TOKEN [opts] -- COMMAND ARGS...        a script printing PASS:/FAIL:/SKIP: or ok/not ok lines
wrap-vitest.sh --run-token TOKEN [opts] -- VITEST_ARGS...         vitest run --reporter=json --outputFile=<tmp>
wrap-gradle.sh --run-token TOKEN [--results-dir DIR] [opts] -- GRADLE_ARGS...   ./gradlew ARGS, then the JUnit XML under DIR
wrap-cargo.sh  --run-token TOKEN [opts] -- CARGO_TEST_ARGS...     cargo test ARGS (colour off)
opts: --from-report PATH (parse a recorded report instead of running; with --rc N the recorded exit status takes part) --min-tests N --no-raw
```
`--run-token` is REQUIRED (32 lowercase hex from `evrec token`): a run without it cannot pass (T051a, constitution 7.1). The wrapper exports it to the test as `EVREC_RUN_TOKEN` and never prints it. Typical use: `evrec run CAT-123 GREEN 1 shell_script <ref> --oracle ... -- tools/evidence/wrap-go.sh --run-token "$(evrec token)" -- ./pkg/...` (the token is generated before the call and passed after `--run-token`; the recorded argv holds it).
Output: `wrap: runner=... status=... tests=N passed=P failed=F skipped=S`, one `wrap: failed <name>` line per failure, `wrap: reason=<name>`, `wrap: note=<...>`, `wrap: rc=<runner exit or unknown>`, then `wrap: raw-begin` and the runner's own report (the stream the recorder stores, so the captured post-state of a state-delta test is in it).
Test overrides of the runner binary: `WRAP_GO_BIN`, `WRAP_VITEST_BIN` (default `npx vitest`), `WRAP_GRADLE_BIN` (default `./gradlew`), `WRAP_CARGO_BIN`.

## What the parsers refuse to be fooled by (the T051 acceptance)
The per-test results are the evidence, never the final summary. A stream cut before its summary still shows its failures; a fake `ok` line printed by a test is only text inside an output event; a suite header that says `failures=0` over a failing test case does not hide it (gradle, vitest); a bash summary that disagrees with the lines (a line was lost, `plan_mismatch`) is a failure; a stream with no confirmable end (a test started and never finished, a package without a summary event, a cut report with no failure shown) is `truncated_report`, an error, never a pass; a non-zero runner exit with no failing line is `exit_without_fail_line`, a failure.
Reasons: `empty_report`, `unparsable`, `no_tests`, `too_few_tests`, `build_failed`, `package_failed`, `truncated_report`, `header_failures`, `summary_mismatch`, `plan_mismatch`, `summary_reports_failures`, `exit_without_fail_line`, `no_reports`, `runner_not_executable`, `run_token_missing`, `run_token_malformed`, `usage_error`.

## Tests and fixtures
`tools/evidence/tests/test_wrap_go.sh`, `test_wrap_bash.sh`, `test_wrap_vitest.sh`, `test_wrap_gradle.sh`, `test_wrap_cargo.sh` with `tests/fixtures/`. Fixtures that are REAL recorded outputs: Go (`go_*.json`, go1.25.14 in IMG-GO, from `fixtures/seeds/gomod`), bash (`bash_*.out`, IMG-KCOV, from `fixtures/seeds/bash_*.sh`), vitest (`vt_*.json`, vitest 1.6.1 in IMG-NODE, a scratch install, from `fixtures/seeds/vitest`). Fixtures that are AUTHORED from the documented format and NOT recorded: gradle (`gradle_*/`), cargo (`cargo_*.txt`); they are UNCONFIRMED against a live run. The `*_truncated*`, `*_summary_cut`, `*_started_only` files are byte or line prefixes of recorded reports. The live legs of vitest, gradle and cargo are `blocked-unavailable` until the project toolchains exist (WP-11 for IMG-NODE with catalog-web, WP-14 for IMG-ANDROID and IMG-RUST).

## Stated limits (11.4.6)
The wrappers judge a report format named above; a runner switched to another reporter yields `unparsable`, an error. Go package-level failures without a failing test (TestMain exit, panic outside a test, timeout) are errors (`package_failed`), not test failures. vitest `numPassedTests` was observed to count a skipped test as passed (vitest 1.6.1); the parser counts assertions, not the header, and only a header FAILURE count above the assertions is a finding.
