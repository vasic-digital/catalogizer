# run_go.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | committed in a7cfc6d3 (T119, T120); revision 2 is the fix round r1 for the independent review (uncommitted until the owner commits it): contract moved to runner_lib.md revision 2; its row in `docs/scripts/README.md` is still owed |
| Source | `scripts/containers/run_go.sh` (a few lines: it sets the `RUNNER_*` variables and sources `runner_lib.sh`); test `scripts/containers/tests/test_runners.sh` |

## Purpose

Go build and test (catalog-api, Go submodules, race, bench, fuzz). The command is prefixed `env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1` (docs/16 6.2). Image: IMG-GO. State now: runs. The whole contract (usage, limits from the envelope, anti-mess sweep, registered long operation,
toolchain record, refusals, exit codes, test hooks) is in [`runner_lib.md`](runner_lib.md).

## Usage

```bash
scripts/containers/run_go.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID] -- <command word>...
```

## Honest boundary (11.4.6)

See [`runner_lib.md`](runner_lib.md). A wrapper never falls back to another image or to the bare host.
