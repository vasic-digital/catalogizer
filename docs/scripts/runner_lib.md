# runner_lib.sh and the run_*.sh wrappers - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | committed in a7cfc6d3 (T119, T120); revision 2 is the fix round r1 for the independent review (uncommitted until the owner commits it): one lock across envelope read and registration, capped allowance, scrubbed RUNP_*, interrupt-safe, wall-clock enforced; its row in `docs/scripts/README.md` is still owed |
| Source | `scripts/containers/runner_lib.sh` (sourced body), wrappers `run_go.sh`, `run_node.sh`, `run_docs.sh`, `run_scan.sh`, `run_playwright.sh`, `run_testutil.sh`; test `scripts/containers/tests/test_runners.sh` |

## Purpose

Every build and test command runs in a pinned rootless container started THROUGH `scripts/containers/run_pinned.sh` (RUNP, reused, never modified).
A wrapper adds what a run owes on top of it (docs/16 sections 6.3, 8, 13):

1. the image must be allowed for the wrapper, present in the lock and pinned by a `sha256:` digest, else `REFUSED`; there is no fallback to another image;
2. the digest `podman image inspect` reports for the image equals the lock digest (or the lock `platform_digest`);
3. the anti-mess sweep (`scripts/anti-mess/sweep.sh --stage cadence --only AM-P1,AM-P2,AM-P3`, the runtime plane) runs first; drift (exit 10) or a
   refusal or blind detector (any other non-zero) refuses the run (11.4.233 C, E); a missing sweep script refuses too (fail closed);
4. the envelope (`envelope.sh`) fixes memory, cpus and pids, handed to RUNP as `RUNP_MEMORY`, `RUNP_CPUS`, `RUNP_PIDS`; a caller may ask for LESS. The
   nominal allowance above the live reading (2% of memory, 1 cpu: the envelope follows `MemAvailable` and the registry and a caller read it moments
   earlier) is capped at the head-room under the 0.60 ceiling once the live operations are counted (`ceiling - used`), at `MemAvailable - reserve` (the
   reserve is never entered) and at the live cpu budget; since the envelope is itself the minimum of those terms, the allowance is arithmetically 0 today
   (review F2: it used to push the aggregate above the ceiling and into the reserve);
5. the envelope read, the limit checks, the op-exists check and the REGISTRATION as a long operation (`scripts/longops/register.sh`: single owner per
   purpose, 11.4.232 A, B) all happen under ONE `flock` (`<registry>/.envelope-budget.lock`, held ~0.3 s): two starts can never both be handed the same
   head-room (review F1: N concurrent starts used to receive the full budget each, aggregate 2x the ceiling). Without `--memory`, the first of several
   concurrent lanes takes the whole 60% budget and the others are refused `envelope_refused`; lanes that must run together ask for LESS (`--memory`).
   The op is registered BEFORE the first container starts and heartbeats while it runs: the progress offset is the byte count of the run's stdout and
   stderr PLUS the bytes under `/out` (review F9: a run that writes only `/out` is progressing), elapsed time is reported, `--wall-s N` is ENFORCED (TERM to
   the container client by exact identity, exit 124, op `failed` / `wall_clock_exceeded`; review F8), the loop ends with its wrapper (a SIGKILLed wrapper's
   container client is terminated by identity and the op released `failed` / `wrapper_died`; review F10), and the run ends in a terminal state
   (`complete` on exit 0, `failed` otherwise, the verdict is `rc=<n>`);
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
| `run_scan.sh` | IMG-SHELLCHECK (default) or `--image` one of IMG-SCAN-TRIVY, -GITLEAKS, -TRUFFLEHOG, -SEMGREP, -GOSEC, -HADOLINT, -SYFT, -SONAR-SCANNER, -DEPCHECK | none | IMG-SHELLCHECK: `shellcheck --version` (entrypoint-only image: version line only, the three write probes are recorded `n/a:no_shell`) | IMG-SHELLCHECK runs; every other scanner image is refused `probe_not_defined` until a reviewed row defines its version command |
| `run_node.sh` | IMG-NODE | none | `node --version` | runs while the lock has IMG-NODE; else `image_not_in_lock` |
| `run_playwright.sh` | IMG-PW | none | `npx playwright --version` | runs while the lock has IMG-PW; else `image_not_in_lock` |
| `run_docs.sh` | IMG-DOCS | none | `python3 --version` | runs while the lock has IMG-DOCS (the real lock pins it, T106); else `image_not_in_lock` with a `BLOCKED:` message |

## Refusals and exits

`<wrapper>: REFUSED reason=<code>` on stderr, exit 1: `image_not_allowed`, `image_not_in_lock`, `image_unpinned`, `image_not_present_locally`,
`image_digest_mismatch`, `lock_unreadable`, `anti_mess_drift`, `anti_mess_blind`, `anti_mess_sweep_missing`, `envelope_refused`,
`limit_exceeds_envelope`, `purpose_conflict`, `register_failed`, `probe_not_defined`, `probe_failed`, `probe_blind`, `out_not_writable`,
`cache_not_writable`, `record_unwritable`, `dependency_missing`, `test_hook_outside_test_mode`, and (fix round r1) `op_exists` (an `--op-id` that is already registered: refused BEFORE any log file is opened, so the other run's logs are never truncated, review F5; `purpose_conflict` is now only a held purpose), `version_probe_failed` (the version command exited non-zero in the container; its error text is no longer recorded as a version, review F11), `registry_library_missing`, `registry_unusable`, `budget_lock_unavailable`, `budget_lock_timeout`. A run that started exits with the container's exit
code (a container may also exit 1: the `REFUSED` line tells a refusal from a container exit); 124 when `--wall-s` was exceeded. Usage errors exit 2;
a TERM, INT or HUP ends the run with 130 and the op `failed` with verdict `interrupted` (the container client process is signalled by identity: pid > 1 and
the start time recorded at spawn, never a group). The trap is installed BEFORE anything is registered and the toolchain probe runs as a background child, so a
signal during the probe now ends the run at once with the op released (review F6: it used to leave a registered op with a dead owner, which made every later
wrapper start refuse `anti_mess_drift`).

## Environment and test hooks

`RUNP_LOCK` (lock file, as RUNP), `RUNNER_HEARTBEAT_S` (1..60, default 5), `RUNNER_LOG_DIR`, `LONGOPS_*` (relocating the registry needs `ENVELOPE_TEST_MODE=1`, see `envelope.md`). `RUNNER_RUNP` (a `run_pinned.sh`
replacement) and `RUNNER_SWEEP` (a sweep replacement) are test hooks honoured only with `RUNNER_TEST_MODE=1`; otherwise `test_hook_outside_test_mode`.

## Test

`scripts/containers/tests/test_runners.sh`: two independent oracles (a `run_pinned.sh` shim that logs argv, the `RUNP_*` limits and the registry state at
call time; the real registry read back with jq), every refusal, the toolchain record, the sweep gate (shim and the real sweep), the explicit-limit slack,
single-owner purposes, heartbeats, and a real container leg (IMG-TESTUTIL, IMG-GO, IMG-SHELLCHECK and, while the lock has them, IMG-NODE and IMG-PW),
then 29 paired mutations (`RUNNER_MUTATION_RECORD`), among them the removal of `--memory`: the limit is not handed to `run_pinned.sh`. The fix round r1 adds [`test_runners_hardening.md`](test_runners_hardening.md) (F1, F2, F4, F5, F6, F8, F9, F10, F11 with their own mutants) and keeps the real legs' disk-headroom records out of the real `evidence/disk` (`DISK_HEADROOM_OUT_DIR` in scratch, asserted).

## Honest boundary (11.4.6)

The toolchain record proves the image is the pinned one and the probe could see a failure; it does not prove the build passes. IMG-DOCS, IMG-NODE and
IMG-PW run for real in `test_runners.sh` while the lock has them; the scanner images other than IMG-SHELLCHECK are not exercised. OWED (not done in r1, other agents' or owner's scope): `run_pinned.sh` reads `nproc` with `OMP_NUM_THREADS` honoured (review F7: the same input the envelope no longer trusts); the toolchain record is written into the writable `/out` and the run can rewrite it (F12, owner decision); the record stores the lock digest when podman reports the platform digest (F13); `jobs` is computed and nothing consumes it, and docs/16 8.1 cgroup inputs and the 8.4 `--ulimit nofile` / tmpfs size cap are not implemented (F15); TIC computes `--memory` before the wrapper recomputes it, so a TIC lane can be refused `limit_exceeds_envelope` when MemAvailable moves (F16). A transient `probe_failed` was seen once while another agent was using podman
(`UNCONFIRMED:` cause; the refusal now carries the probe's stderr).
