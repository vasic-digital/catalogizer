# test_runners_hardening.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T03:30:00Z |
| Status | fix round r1 committed in 96779242; revision 2 is the fix round r2 for the WF13 review (blocks B1, F10b, F12, F13, F14, F15, a fake podman that knows containers, a negative control for the mutation harness; uncommitted until the owner commits it); independent re-review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/containers/tests/test_runners_hardening.sh`; subject `scripts/containers/runner_lib.sh` and the `run_*.sh` wrappers; sibling suites `test_runners.sh`, `test_envelope.sh`, `test_test_in_container.sh`, `test_race_detector.sh` |

## Purpose

RED-first oracle for the defects the independent review of the resource envelope and the runner wrappers found (WF10 p1-envelope-wrappers). One block per finding:

| Block | Finding | What it asserts |
|---|---|---|
| F1 | concurrent starts took the full budget each (2x the 0.60 ceiling) | six simultaneous starts asking 4e9 each: exactly four admitted, the sum of live budgets within the ceiling, two refused `limit_exceeds_envelope`; four starts without `--memory`: one admitted, three `envelope_refused` |
| F2 | the 2% / +1 cpu allowance went above the head-room | with a live 2e9 / 9 cpu op, `--memory` and `--cpus` above (ceiling - used) / the live cpu budget are refused, the exact values accepted; with MemAvailable binding, 1 byte above the live envelope (the reserve) is refused |
| F4 | an inherited `RUNP_PRINT_ARGV=1` faked a version probe | `run_pinned.sh` (a shim that records its environment) sees `RUNP_PRINT_ARGV`, `RUNP_TEST_MODE`, `RUNP_MEMINFO`, `RUNP_ULIMIT_U` unset and `RUNP_USER` kept; the scrub is announced on stderr |
| F5 | a reused `--op-id` truncated the other run's logs and was labelled `purpose_conflict` | a finished and a running run keep byte-identical logs, the reason is `op_exists`, the running run's replayed stdout is exactly its own output (no NUL bytes) |
| F6 | TERM/INT/HUP during the toolchain probe left a registered op with a dead owner | each signal ends the wrapper 130 with the op `failed` / `interrupted`, no `dead_owner` in the registry; TERM during the run likewise |
| F8 | `--wall-s` was accepted and never enforced | a 12 s run with `--wall-s 2` ends in under 9 s with exit 124, op `failed` / `wall_clock_exceeded`, `elapsed_ms` recorded; a run inside its wall clock is untouched |
| F9 | a run that writes only `/out` was classified `hung` | with `--no-progress-s 2` the run stays `advancing`; a run that writes nothing anywhere IS `hung` (the control needle, 11.4.201) |
| F10 | a SIGKILLed wrapper left its heartbeat loop and container client running | the container client (by pid and start time) is gone, the op is `failed` / `wrapper_died`, the heartbeat sequence stops advancing |
| F11 | a failing version command was recorded as the tool version | a REAL IMG-TESTUTIL run whose version command does not exist is refused `version_probe_failed` and the main run does not start; the control (`python3 --version`) runs and records `Python 3.x` |
| B1 | wall clock, TERM/INT/HUP mid-run and parent death signalled only the `podman run` client; a container whose PID 1 ignores TERM kept running while the wrapper said it was terminated and freed the budget (WF13 B1) | REAL containers (IMG-TESTUTIL `sleep 40`, IMG-GO `sh -c 'sleep 25; echo ...'`) observed through the real `podman ps` by the op's label (with a control needle that sees the container up first): `--wall-s 4` ends 124 in under 22 s with the container gone and the command's output never printed; TERM to the wrapper ends 130 within 12 s; SIGKILL to the wrapper releases the op `failed` / `wrapper_died` with the container already gone and the envelope's `used_mem_bytes` back to 0 |
| F6/F8/F10 (round 2) | the shim client died on TERM, which no container's PID 1 does (the shim hid B1) | the shim now ignores TERM and ends only when the fake `podman stop` kills it; F6 asserts the stop was issued by label and id (`stop --time 2 -- <id>`), F8 a tight bound (<= 6 s for `--wall-s 2`, `elapsed_ms` 2000-4500: a wall clock 3x too long fails) and that the run did not complete its work, F10 that the op is released only after the container is gone |
| F10b | a wrapper killed during its toolchain probe left the op `registered` with a dead owner (WF13 m1) | the guard loop starts at registration: the op is `failed` / `wrapper_died`, the probe container gone, no `dead_owner` |
| F12 | TERM while waiting for the budget lock was honoured only when the lock freed, up to 120 s (WF13 m2) | with the lock held elsewhere, TERM ends the wrapper 130 within 4 s, nothing registered |
| F13 | a start refused for a duplicate `--op-id` deleted the running run's stop and wall files (WF13 I2) | three refused duplicates leave the running run's wall marker alone; a burst of duplicates across the end of the run does not hang it |
| F14 | TIC's 98% margin did not cover the drift between its read and the wrapper's read (WF13 I1) | the real TIC + the real wrapper while a sweep shim changes the host in the window between them: MemAvailable falling 3.9%, another op registering 6 cpus, and a 12-start stress loop (3 at a time, MemAvailable changing every 0.1 s): no `limit_exceeds_envelope`, the container gets the wrapper's own locked reading |
| F15 | the op was released although its container was still Up | a container that survives `podman stop` and KILL (a separate "container" process) keeps the op registered; exit 125, "could not be stopped" on stderr |
| TREE | the test suites wrote disk-headroom records into the real `evidence/disk` | the real legs run with `DISK_HEADROOM_OUT_DIR` in scratch; no record carrying this run's token appears in the real tree |

The envelope-side findings F3 (fail-closed registry, stray `LONGOPS_*`) and F7 (`OMP_NUM_THREADS`) and the harness finding M1 are in `test_envelope.sh`.

## Method

Two independent observers per behaviour: a `run_pinned.sh` shim that logs argv and the `RUNP_*` environment (and can hold the run at a gate file), and the REAL long-op registry (`scripts/longops`, fixture state) read back with `jq` and `classify.sh`; process identity is read from `/proc` (pid plus start time, never `pgrep`). The wrapper is started in the background with SIGINT restored to its default (a plain `&` makes bash ignore SIGINT, which would make the INT case untestable).

RED was captured against an archive of the committed scripts (`R1_SUT_DIR`), GREEN against the working tree three times. Mutants (`runner_lib.sh` and, for F14, `test-in-container.sh`; one expression per copy, run against only the blocks that cover them) count as caught only when a FAILING CHECK NAMES the cause, and an UNMUTATED copy placed like a mutant must pass those blocks first (the negative control, WF13 m4; without it a harness that is blind, or that fails for an environmental reason such as an inherited SIGHUP disposition, counts every mutant as caught): budget lock dropped, the lock wait made foreground, the lock released before the registration (reviewer mutant N2), a request above the envelope accepted, the scrub dropped, the op-exists check dropped, the trap not installed, the interrupt verdict flipped, the probe checkpoint dropped, the wall-clock enforcement dropped, the wall clock 3x too long (N3), the wall branch or the signal handler not stopping the container, the second `wait` after a signal dropped (N4: with the container stopped by the handler first this one is now an equivalent mutant; the handler mutant is its observable form), the op released while its container is Up, the parent-death path not stopping the container, the guard loop not started at registration, the per-op marker named before the duplicate check (I2), `/out` bytes removed from the progress proof, the progress offset forced to 0, the parent-liveness check dropped, the dispatcher passing `--memory` or `--cpus` again (I1).

## Usage

```bash
scripts/containers/tests/test_runners_hardening.sh                    # tests and mutations
R1_TEST_NO_MUTATIONS=1 scripts/containers/tests/test_runners_hardening.sh
R1_TEST_NO_REAL=1 ...        # skip the real container blocks (B1, F11, TREE)
R1_ONLY="F1 F2" ...          # only those blocks
R1_SUT_DIR=<containers dir> ...   # test another copy (a RED run points it at an archive of the committed scripts)
```

## Honest boundary (11.4.6)

The shim proves what the wrapper hands to `run_pinned.sh` and when; only the B1, F11 and TREE blocks run a real container (the B1 observers use the real `podman`, not the fake one the shim blocks put first on PATH; a first version of the observers read through the fake and every "container gone" passed for nothing: the control needle "the observer saw the container up" now precedes every such check). The F1 outcome (one lane takes the whole budget unless it asks for less) is a consequence of the 0.60 ceiling with `jobs = 1` until a profile is measured, not a performance claim. A signal sent to the whole process group of a wrapper is not tested (the wrappers never signal a group). Starting a wrapper at the instant a TERM arrives between the register command finishing and its record being read is not exercised: `UNCONFIRMED:` for that window.
