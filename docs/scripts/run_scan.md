# run_scan.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T18:00:00Z |
| Status | new in the working tree (T120), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/containers/run_scan.sh` (a few lines: it sets the `RUNNER_*` variables and sources `runner_lib.sh`); test `scripts/containers/tests/test_runners.sh` |

## Purpose

Scanners over the read-only source, results in `/out`. Image: IMG-SHELLCHECK (default) or `--image` a scanner image of the lock. State now: IMG-SHELLCHECK runs; other scanner images are refused `probe_not_defined` until a reviewed row defines their version command. The whole contract (usage, limits from the envelope, anti-mess sweep, registered long operation,
toolchain record, refusals, exit codes, test hooks) is in [`runner_lib.md`](runner_lib.md).

## Usage

```bash
scripts/containers/run_scan.sh [--image IMG-ID] [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID] -- <command word>...
```

## Honest boundary (11.4.6)

See [`runner_lib.md`](runner_lib.md). A wrapper never falls back to another image or to the bare host.
