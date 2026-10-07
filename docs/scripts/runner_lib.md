# runner_lib.sh and the run_*.sh wrappers - Companion Guide

| Field | Value |
|---|---|
| Revision | 4 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T08:00:00Z |
| Status | committed (a7cfc6d3, 96779242; revision 3 = round 3 in d9162b7d); revision 4 is fix round r4 for the WF15 review of d9162b7d (uncommitted until the owner commits it): a start whose container is not up yet is ended too (B1), paused and created containers count as up (B1/P10), the guard loop reads the owner's liveness as the registry does (I2), `OMP_*` caps never reach `run_pinned.sh` (m1); independent re-review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is still owed (11.4.212) |
| Source | `scripts/containers/runner_lib.sh` (sourced body), wrappers `run_go.sh`, `run_node.sh`, `run_docs.sh`, `run_scan.sh`, `run_playwright.sh`, `run_testutil.sh`; test `scripts/containers/tests/test_runners.sh` |

## Purpose

Every build and test command runs in a pinned rootless container started THROUGH `scripts/containers/run_pinned.sh` (RUNP, reused, never modified).
A wrapper adds what a run owes on top of it (docs/16 sections 6.3, 8, 13):

1. the image must be allowed for the wrapper, present in the lock and pinned by a `sha256:` digest, else `REFUSED`; there is no fallback to another image;
2. the digest `podman image inspect` reports for the image equals the lock digest (or the lock `platform_digest`);
3. the anti-mess sweep (`scripts/anti-mess/sweep.sh --stage cadence --only AM-P1,AM-P2,AM-P3`, the runtime plane) runs first; drift (exit 10) or a
   refusal or blind detector (any other non-zero) refuses the run (11.4.233 C, E); a missing sweep script refuses too (fail closed);
4. the envelope (`envelope.sh`) fixes memory, cpus and pids, handed to RUNP as `RUNP_MEMORY`, `RUNP_CPUS`, `RUNP_PIDS`; a caller may ask for LESS, a request
   above the live reading is refused `limit_exceeds_envelope`. **This wrapper is the only source of the limits** (WF13 I1): it reads the envelope once, under the
   budget lock, and `scripts/test-in-container.sh` passes NO number (it used to pass 98% of its own earlier reading; a budget that fell by more than 2%, or another
   op registering, in the seconds between the two readings refused a valid lane, about 1 start in 5 on a loaded host). The nominal allowance above the reading is
   gone: it was arithmetically 0 after the F2 fix;
5. the envelope read, the limit checks, the op-exists check and the REGISTRATION as a long operation (`scripts/longops/register.sh`: single owner per
   purpose, 11.4.232 A, B) all happen under ONE `flock` (`<registry>/.envelope-budget.lock`, held ~0.3 s): two starts can never both be handed the same
   head-room (review F1: N concurrent starts used to receive the full budget each, aggregate 2x the ceiling). Without `--memory`, the first of several
   concurrent lanes takes the whole 60% budget and the others are refused `envelope_refused`; lanes that must run together ask for LESS (`--memory`).
   The op is registered BEFORE the first container starts and a guard loop starts at registration (so it covers the toolchain probe too): it heartbeats, the
   progress offset is the byte count of the run's stdout and stderr PLUS the bytes under `/out` (review F9: a run that writes only `/out` is progressing),
   the elapsed time of the run is reported. **A run is ended by stopping its CONTAINER, never by signalling the `podman run` client** (WF13 B1: the client
   proxies TERM to the container and PID 1 of a container ignores a TERM it has no handler for - `sleep`, `sh`, `python3`, `node` - so a signal to the client left
   the run alive while the wrapper reported it terminated). The container is found by the labels `run_pinned.sh` sets (`catalogizer.op_id=<op id>`,
   `project=catalogizer`) and proven this run's by its `/out` mount (the resolved `--out`); it gets `podman stop --time <grace>` (TERM, then KILL after
   `RUNNER_STOP_GRACE_S`, default 10), and the client is signalled only while no container exists yet. The same procedure serves `--wall-s N` (the clock of the
   run, from the spawn of the main command; the toolchain probe is guarded but not counted; exit 124, op `failed` / `wall_clock_exceeded`), TERM/INT/HUP at any
   point (also while waiting for the budget lock, at most 1 s late, WF13 m2) and the death of the wrapper itself (SIGKILL included: the guard loop stops the
   container and releases the op `failed` / `wrapper_died`, WF13 m1). **The op is released only when no container of the run is Up any more AND the `podman run` client is gone** (the registry never
   reports the budget freed while it runs; "up" is every state but finished: running, paused, created - `podman ps` without `-a` lists running ones only, `podman stop` fails on a paused container and is a no-op on a created one, so the call is chosen by state: stop, then kill, then `rm -f`; a start whose client is alive while its container is not listed yet (a window of 2.4-2.8 s measured) is ended through the client, never read as "down", WF15 B1); a container that survives the stop and the KILL keeps the op registered, the wrapper exits 125 and names it (the
   anti-mess sweep then reports the container). The run ends in a terminal state (`complete` on exit 0, `failed` otherwise, the verdict is `rc=<n>`). The per-op
   marker files (`.stop`, `.stop.wall`, `.ch`, `.run`) are named only after the wrapper registered the op id (WF13 I2: a start refused as a duplicate used to delete
   the running run's stop and wall files);
5a. inherited `RUNP_*` controls the wrapper does not own are UNSET with a note on stderr (review F4: `RUNP_PRINT_ARGV=1` made `run_pinned.sh` print its
   argv and exit 0, which the version-probe mode of `run_scan` recorded as the tool version `podman`, with no container started); `RUNP_LOCK` and
   `RUNP_USER` pass through;
6. `toolchain.json` (schema `toolchain-record/1`) is written into the out directory from a probe container run first: wrapper, image id, lock and
   inspected digests, `digest_match`, the tool version, the envelope, and three probes: a write to `/out`, a write to the cache directory (both must
   succeed) and a write to the READ-ONLY `/src` mount that must FAIL (the control needle of 11.4.201: if it succeeds the probe is blind and the run
   is refused as `probe_blind`).

## Usage

```bash
scripts/containers/run_<x>.sh [--image IMG-ID] [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES]
                              [--purpose KEY] [--op-id ID] [--no-progress-s N] [--wall-s N] -- <command word>...
```

`--image` exists only for `run_scan.sh`. `--out` must be absolute (default `$PWD/.audit/out/<op id>`); the default purpose is
`container:<wrapper>:<sha256 of cwd, image and command>` so the same command in the same checkout is single-owner. The run's stdout and stderr are
collected in `$PWD/.audit/runner-logs/<op id>.{out,err}` and replayed to the caller's stdout and stderr when the run ends; `/out` is live.

## The wrappers

| Wrapper | Image | Command prefix | Version probe | State now |
|---|---|---|---|---|
| `run_go.sh` | IMG-GO | `env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1` (docs/16 6.2) | `go version` | runs (real leg of the test) |
| `run_testutil.sh` | IMG-TESTUTIL | none | `python3 --version` | runs |
| `run_scan.sh` | IMG-SHELLCHECK (default) or `--image` one of IMG-SCAN-TRIVY, -GITLEAKS, -TRUFFLEHOG, -SEMGREP, -GOSEC, -HADOLINT, -SYFT, -SONAR-SCANNER, -DEPCHECK | none | IMG-SHELLCHECK: `shellcheck --version` (entrypoint-only image: the first output line with a dotted version number, e.g. `version: 0.10.0`, WF13 m5; the three write probes are recorded `n/a:no_shell`) | IMG-SHELLCHECK runs; every other scanner image is refused `probe_not_defined` until a reviewed row defines its version command |
| `run_node.sh` | IMG-NODE | none | `node --version` | runs while the lock has IMG-NODE; else `image_not_in_lock` |
| `run_playwright.sh` | IMG-PW | none | `npx playwright --version` | runs while the lock has IMG-PW; else `image_not_in_lock` |
| `run_docs.sh` | IMG-DOCS | none | `python3 --version` | runs while the lock has IMG-DOCS (the real lock pins it, T106); else `image_not_in_lock` with a `BLOCKED:` message |

## Refusals and exits

`<wrapper>: REFUSED reason=<code>` on stderr, exit 1: `image_not_allowed`, `image_not_in_lock`, `image_unpinned`, `image_not_present_locally`,
`image_digest_mismatch`, `lock_unreadable`, `anti_mess_drift`, `anti_mess_blind`, `anti_mess_sweep_missing`, `envelope_refused`,
`limit_exceeds_envelope`, `purpose_conflict`, `register_failed`, `probe_not_defined`, `probe_failed`, `probe_blind`, `out_not_writable`,
`cache_not_writable`, `record_unwritable`, `dependency_missing`, `test_hook_outside_test_mode`, and (fix round r1) `op_exists` (an `--op-id` that is already registered: refused BEFORE any log file is opened, so the other run's logs are never truncated, review F5; `purpose_conflict` is now only a held purpose), `version_probe_failed` (the version command exited non-zero in the container; its error text is no longer recorded as a version, review F11), `registry_library_missing`, `registry_unusable`, `budget_lock_unavailable`, `budget_lock_timeout`. A run that started exits with the container's exit
code (a container may also exit 1: the `REFUSED` line tells a refusal from a container exit); 124 when `--wall-s` was exceeded; 125 when a container of the run could not be stopped (the op stays registered). Usage errors exit 2;
a TERM, INT or HUP ends the run with 130 and the op `failed` with verdict `interrupted` (the container is stopped by its label and `/out` mount; the client is signalled only by identity: pid > 1 and
the start time recorded by the child itself, never a group). The guard loop judges the wrapper by the same rule as the registry (same start time AND not a zombie, WF15 I2: a SIGKILLed wrapper whose parent has not reaped it used to be read alive by the loop and dead by the registry, with the container running under a 10 GB limit and the envelope handing its budget to the next start). `OMP_NUM_THREADS` / `OMP_THREAD_LIMIT` are removed from the environment of `run_pinned.sh` (WF15 m1: GNU `nproc` honours them, so `OMP_NUM_THREADS=1` refused every lane `probe_failed`). The trap is installed BEFORE anything is registered and the toolchain probe runs as a background child, so a
signal during the probe now ends the run at once with the op released (review F6: it used to leave a registered op with a dead owner, which made every later
wrapper start refuse `anti_mess_drift`).

## Environment and test hooks

`RUNP_LOCK` (lock file, as RUNP), `RUNNER_HEARTBEAT_S` (1..60, default 5), `RUNNER_STOP_GRACE_S` (seconds between TERM and KILL when a container is stopped, 1..120, default 10), `RUNNER_LOG_DIR`, `LONGOPS_*` (relocating the registry needs `ENVELOPE_TEST_MODE=1`, see `envelope.md`). `RUNNER_RUNP` (a `run_pinned.sh`
replacement) and `RUNNER_SWEEP` (a sweep replacement) are test hooks honoured only with `RUNNER_TEST_MODE=1`; otherwise `test_hook_outside_test_mode`.

## Test

`scripts/containers/tests/test_runners.sh`: two independent oracles (a `run_pinned.sh` shim that logs argv, the `RUNP_*` limits and the registry state at
call time; the real registry read back with jq), every refusal, the toolchain record, the sweep gate (shim and the real sweep), the explicit limits (no allowance above the reading),
single-owner purposes, heartbeats, and a real container leg (IMG-TESTUTIL, IMG-GO, IMG-SHELLCHECK and, while the lock has them, IMG-NODE and IMG-PW),
then 29 paired mutations (`RUNNER_MUTATION_RECORD`), among them the removal of `--memory`: the limit is not handed to `run_pinned.sh`. The fix rounds r1 and r2 add [`test_runners_hardening.md`](test_runners_hardening.md) (F1, F2, F4, F5, F6, F8, F9, F10, F11 and, in round 2, B1 (real containers whose PID 1 ignores TERM), F10b, F12, F13, F14, F15, each with its own mutants and a negative control) and keeps the real legs' disk-headroom records out of the real `evidence/disk` (`DISK_HEADROOM_OUT_DIR` in scratch, asserted).

## Honest boundary (11.4.6)

The toolchain record proves the image is the pinned one and the probe could see a failure; it does not prove the build passes. IMG-DOCS, IMG-NODE and
IMG-PW run for real in `test_runners.sh` while the lock has them; the scanner images other than IMG-SHELLCHECK are not exercised. OWED (not done in r1, other agents' or owner's scope): `run_pinned.sh` reads `nproc` with `OMP_NUM_THREADS` honoured (review F7; since WF15 m1 the wrapper removes the `OMP_*` caps from the environment of `run_pinned.sh`, so no wrapper lane can hit it; a direct `run_pinned.sh` caller still can); the toolchain record is written into the writable `/out` and the run can rewrite it (F12, owner decision); the record stores the lock digest when podman reports the platform digest (F13); `jobs` is computed and nothing consumes it, and docs/16 8.1 cgroup inputs and the 8.4 `--ulimit nofile` / tmpfs size cap are not implemented (F15); (F16, closed in round 2: TIC no longer computes a limit.) A container started without a wrapper (direct `run_pinned.sh` callers) is invisible to the budget; the anti-mess sweep is the backstop (WF13 i2, owner decision). A transient `probe_failed` was seen once while another agent was using podman
(`UNCONFIRMED:` cause; the refusal now carries the probe's stderr).
