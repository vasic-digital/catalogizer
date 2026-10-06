# test_race_detector.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T18:00:00Z |
| Status | new in the working tree (T124, test-first part), not yet committed; independent review owed (constitution 11.4.142) |
| Source | `scripts/containers/tests/test_race_detector.sh` |

## Purpose

The Go race detector must run inside the pinned IMG-GO container through `run_go.sh`, and a seeded data race must make the run FAIL while the fixed copy
passes (T124). The fixture packages (a racy one and its mutex-fixed copy) are written by the test into a temporary directory at run time and never
committed (a deliberate-violation fixture for the anti-bluff ratchet).

## Legs

| Leg | What it asserts | State |
|---|---|---|
| instrument | the `jq` reader of `/out/go-test.jsonl` sees a seeded `DATA RACE` line and not a clean one (control needle) | GREEN |
| L1 | the racy copy: `GOMAXPROCS=3 go test -race -p 2 -parallel 2 -count=1 -json ./... > /out/go-test.jsonl` through `run_go.sh` in IMG-GO exits non-zero; the jsonl has a `DATA RACE` output event and a failed test event; the fixture itself printed `race=enabled` and `gomaxprocs=3` from inside the container; the registered long op ended `failed`; the fixed copy exits 0, no race event, package passed, op `complete`; negative control: the racy copy WITHOUT `-race` exits 0 with `race=disabled` | GREEN |
| L2 | with a `run_pinned.sh` shim the lane command reaches RUNP through `run_go.sh` as `env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1 sh -c 'go test -race -p 2 -parallel 2 ...'` with the envelope limits | GREEN |
| L3 | `scripts/run-race-detector.sh` runs NO bare-host `go` and starts a container run of `go test` on the IMG-GO digest with `--memory` | RED today: the script still calls the bare-host `go`; GREEN only when the script is moved to `run_go.sh` (T124 migration, remote lane through `scripts/build/dispatch.sh`, blocked on T121a / T005b) |
| wrap-go | the race report through `tools/evidence/wrap-go.sh` | SKIP, BLOCKED-ON-T051 (the wrapper does not exist) |

`RACE_TEST_SKIP_L3=1` runs L1 and L2 only. `RACE_SUT` names the script L3 tests. `RACE_RESULT_JSON` receives a machine-readable summary.

## Honest boundary (11.4.6)

L1 and L2 prove the containerized race run and its limits on the fixture; they do not prove `catalog-api` is race-free. The sweep is a shim in this
test (the container census would see other agents' containers); the real sweep wiring is asserted by `test_runners.sh`.
