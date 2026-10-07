# check_licence_swallow.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T17:00:00Z |
| Status | applied in the working tree (not committed); the independent review is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp11/owner-0107-*` |
| Source | `scripts/check_licence_swallow.sh`; test `scripts/tests/test_check_licence_swallow.sh` |

## Purpose

Owner decision 2026-10-07 (1): a licence-acceptance step (`sdkmanager --licenses`) must fail loudly. `yes | sdkmanager --licenses ... || true` hid a missing licence until the
later `sdkmanager "platform-tools" ...` step failed with a confusing message (or the build shipped without the SDK). The check keeps that class out of the tree.

## Usage

```bash
scripts/check_licence_swallow.sh [--root DIR] [--list]
```

Scans `Dockerfile*`, `Containerfile*`, `docker-compose*.yml|yaml`, `compose*.yml` and `*.sh` under the root (pruned: `.git`, `node_modules`, `submodules`, `.audit`,
`scripts/containers/tests`, `scripts/tests`, whose fixtures hold the pattern on purpose). Backslash continuations are joined into one logical command. A command that names
`licenses`/`licences` and ends in `|| true` or `|| :` is a finding. Full-line comments are skipped (a carrier that only mentions the pattern does not fire).

| Exit | Meaning |
|---|---|
| 0 | clean |
| 1 | at least one swallow, each printed as `file:line: command` |
| 2 | usage |
| 3 | nothing scanned (blind instrument; a zero would be a lie, 11.4.201) |

`--list` prints the scanned files (the control needle of the test: the instrument must see `docker/Dockerfile.android4` before its zero is believed).

## Why the pipeline status is enough

The Dockerfiles use `SHELL ["/bin/bash", "-c"]` without `-o pipefail`, so the status of `yes | sdkmanager --licenses` is sdkmanager's; `yes` dying of SIGPIPE does not fail it.
`scripts/android/setup.sh` has no `pipefail` either and tests the pipeline in an `if !`.

## Test

`scripts/tests/test_check_licence_swallow.sh` (golden-bad shapes, golden-FALSE carriers, control needles, a REAL-TREE assertion, a paired mutation that re-injects the swallow
into a copy of the real `docker/Dockerfile.android`). Run it in the pinned image:
`scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- bash scripts/tests/test_check_licence_swallow.sh`.

## Honest limits

Only licence acceptance is in scope. Other deliberate swallows exist (`snyk ... || true` in `docker-compose.security.yml`, `avdmanager create avd ... || true` in
`docker-compose.test.yml`) and are NOT judged here; they need their own owner decision. The Android image build with the new RUN line was not executed here.
