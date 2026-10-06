# run_docs.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | committed in a7cfc6d3 (T119, T120); revision 2 is the fix round r1 for the independent review (uncommitted until the owner commits it): state line corrected (the real lock pins IMG-DOCS); its row in `docs/scripts/README.md` is still owed |
| Source | `scripts/containers/run_docs.sh` (a few lines: it sets the `RUNNER_*` variables and sources `runner_lib.sh`); test `scripts/containers/tests/test_runners.sh` |

## Purpose

Documentation checks and renders (link crawl, headers, fingerprints, exports, diagrams). Image: IMG-DOCS. State now: runs while the lock has IMG-DOCS (the real lock pins it, T106; the real leg of `test_runners.sh` runs `python3 --version` in it), else refused `image_not_in_lock` with a `BLOCKED:` message; nothing faked. The whole contract (usage, limits from the envelope, anti-mess sweep, registered long operation,
toolchain record, refusals, exit codes, test hooks) is in [`runner_lib.md`](runner_lib.md).

## Usage

```bash
scripts/containers/run_docs.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID] -- <command word>...
```

## Honest boundary (11.4.6)

See [`runner_lib.md`](runner_lib.md). A wrapper never falls back to another image or to the bare host.
