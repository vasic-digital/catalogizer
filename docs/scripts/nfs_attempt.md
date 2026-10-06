# nfs_attempt.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T134), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/nfs_attempt.sh`; tests `tests/infra/test_nfs_terminal_state.sh`, `tests/infra/test_roundtrip_nfs.sh` |

## Purpose

Runs the unprivileged NFS attempt end to end and records its terminal state: build the image (`nfs_build.sh`), start the project with the `nfs` service, three round trips each recorded as an `ev/1`
record, tear the project down by label, then `nfs_terminal_state.sh` (through `TIC tooling unit`) reads the client verdict first and writes `nfs-attempt.json`.

## Usage

```bash
scripts/test-infra/nfs_attempt.sh --ev-dir <dir> --client-json <wp11/nfs-client.json> [--build-id <id>]
```

Exit 0 when a terminal state was recorded (the state is in the record), 1 when none could be chosen.
