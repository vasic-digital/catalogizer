# run_qa.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:45:00Z |
| Status | new in the working tree (WP-24), not yet committed; independent review owed (constitution 11.4.142, T219); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/containers/run_qa.sh` (T212, a few lines over `runner_lib.sh`); test `scripts/containers/tests/test_run_qa.sh` |

## Purpose

The QA wrapper: image `IMG-QA`. It runs one command (the run-profile wrapper, the floor regeneration, the analyzer self-test, the QA tooling tests) through `run_pinned.sh` under the T120 runner contract (toolchain record, long-op registration, anti-mess sweep at start, envelope limits) documented in [`runner_lib.md`](runner_lib.md). An `IMG-QA` entry that is absent is refused `image_not_in_lock`, one without a digest `image_unpinned`, one whose digest podman does not report `image_digest_mismatch`; never another image.

## Usage

```bash
scripts/containers/run_qa.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID] -- <command word>...
```

## Honest boundary

IMG-QA is not built (T211 blocked), so against the real lock every run is refused `image_not_in_lock`. The `qa` rows of `scripts/containers/lanes.tsv` are NOT added by this change: `scripts/test-in-container.sh` itself refuses the app `qa` (line 71) and `test_test_in_container.sh` asserts it; both belong to a change that edits those committed files. The wrapper-level paired mutation (image IMG-QA changed to IMG-GO) is observed failing; the lanes-row mutation of T212 is owed with the lanes.
