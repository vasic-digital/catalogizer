# runner_lib.sh and the run_*.sh wrappers - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T18:00:00Z |
| Status | new in the working tree (T120), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/containers/runner_lib.sh` (sourced body), wrappers `run_go.sh`, `run_node.sh`, `run_docs.sh`, `run_scan.sh`, `run_playwright.sh`, `run_testutil.sh`; test `scripts/containers/tests/test_runners.sh` |

## Purpose

Every build and test command runs in a pinned rootless container started THROUGH `scripts/containers/run_pinned.sh` (RUNP, reused, never modified).
A wrapper adds what a run owes on top of it (docs/16 sections 6.3, 8, 13):

1. the image must be allowed for the wrapper, present in the lock and pinned by a `sha256:` digest, else `REFUSED`; there is no fallback to another image;
2. the digest `podman image inspect` reports for the image equals the lock digest (or the lock `platform_digest`);
3. the anti-mess sweep (`scripts/anti-mess/sweep.sh --stage cadence --only AM-P1,AM-P2,AM-P3`, the runtime plane) runs first; drift (exit 10) or a
   refusal or blind detector (any other non-zero) refuses the run (11.4.233 C, E); a missing sweep script refuses too (fail closed);
4. the envelope (`envelope.sh`) fixes memory, cpus and pids, handed to RUNP as `RUNP_MEMORY`, `RUNP_CPUS`, `RUNP_PIDS`; a caller may ask for LESS, and for
   at most 2% more memory / 1 more cpu than the live reading (the envelope follows `MemAvailable` and the registry and the caller read it moments
   earlier), never above the 0.60 ceilings of 12.6;
5. the run is registered as a long operation BEFORE the first container starts (`scripts/longops/register.sh`: single owner per purpose, 11.4.232 A, B),
   heartbeats while it runs (the progress offset is the byte count of the run's output) and ends in a terminal state (`complete` on exit 0, `failed`
   otherwise, the verdict is `rc=<n>`);
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
| `run_docs.sh` | IMG-DOCS | none | `python3 --version` | BLOCKED: IMG-DOCS has no lock entry (T106); refused `image_not_in_lock` with a `BLOCKED:` message, nothing faked |

## Refusals and exits

`<wrapper>: REFUSED reason=<code>` on stderr, exit 1: `image_not_allowed`, `image_not_in_lock`, `image_unpinned`, `image_not_present_locally`,
`image_digest_mismatch`, `lock_unreadable`, `anti_mess_drift`, `anti_mess_blind`, `anti_mess_sweep_missing`, `envelope_refused`,
`limit_exceeds_envelope`, `purpose_conflict`, `register_failed`, `probe_not_defined`, `probe_failed`, `probe_blind`, `out_not_writable`,
`cache_not_writable`, `record_unwritable`, `dependency_missing`, `test_hook_outside_test_mode`. A run that started exits with the container's exit
code (a container may also exit 1: the `REFUSED` line tells a refusal from a container exit). Usage errors exit 2; a TERM, INT or HUP ends the run
with 130 and the op `failed` with verdict `interrupted` (the container client process is signalled, never a group, never a pid <= 1).

## Environment and test hooks

`RUNP_LOCK` (lock file, as RUNP), `RUNNER_HEARTBEAT_S` (1..60, default 5), `RUNNER_LOG_DIR`, `LONGOPS_*`. `RUNNER_RUNP` (a `run_pinned.sh`
replacement) and `RUNNER_SWEEP` (a sweep replacement) are test hooks honoured only with `RUNNER_TEST_MODE=1`; otherwise `test_hook_outside_test_mode`.

## Test

`scripts/containers/tests/test_runners.sh`: two independent oracles (a `run_pinned.sh` shim that logs argv, the `RUNP_*` limits and the registry state at
call time; the real registry read back with jq), every refusal, the toolchain record, the sweep gate (shim and the real sweep), the explicit-limit slack,
single-owner purposes, heartbeats, and a real container leg (IMG-TESTUTIL, IMG-GO, IMG-SHELLCHECK and, while the lock has them, IMG-NODE and IMG-PW),
then 27 paired mutations (`RUNNER_MUTATION_RECORD`), among them the removal of `--memory`: the limit is not handed to `run_pinned.sh`.

## Honest boundary (11.4.6)

The toolchain record proves the image is the pinned one and the probe could see a failure; it does not prove the build passes. IMG-DOCS and the
scanner images other than IMG-SHELLCHECK are not exercised. A transient `probe_failed` was seen once while another agent was using podman
(`UNCONFIRMED:` cause; the refusal now carries the probe's stderr).
