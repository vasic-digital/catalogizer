# run_node.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T03:30:00Z |
| Status | committed in a7cfc6d3 (T119, T120); revision 2 is the fix round r1 for the independent review (committed in 96779242): state line corrected (the real lock pins IMG-NODE); its row in `docs/scripts/README.md` is still owed |
| Source | `scripts/containers/run_node.sh` (a few lines: it sets the `RUNNER_*` variables and sources `runner_lib.sh`); test `scripts/containers/tests/test_runners.sh` |

## Purpose

Node (catalog-web build and unit tests, TypeScript modules, `catalogizer-api-client`). Image: IMG-NODE. State now: runs while the lock has IMG-NODE, else refused `image_not_in_lock`. The whole contract (usage, limits from the envelope, anti-mess sweep, registered long operation,
toolchain record, refusals, exit codes, test hooks) is in [`runner_lib.md`](runner_lib.md).

## Usage

```bash
scripts/containers/run_node.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID] -- <command word>...
```

## Honest boundary (11.4.6)

See [`runner_lib.md`](runner_lib.md). A wrapper never falls back to another image or to the bare host.
