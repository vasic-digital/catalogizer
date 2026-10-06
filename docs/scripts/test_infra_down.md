# down.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T131), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/down.sh`; tests `tests/infra/test_up_down.sh` |

## Purpose

Tears down ONE project by its label: containers, network and volumes carrying `catalogizer.test_project=<project>` (and `project=catalogizer`), nothing else. An unlabelled container whose name
starts with the project name survives (the carrier case); another project's resources survive. Then the registered operation gets the terminal state `complete`, the lease keeper exits by itself (its
file is removed; no signal is ever sent), and the state directory is removed with `podman unshare rm -rf` (the data files are owned by sub-uids of the container user namespace).

## Usage

```bash
scripts/test-infra/down.sh --build-id <id> [--keep-state] [--keep-logs]
```

Idempotent: a project that is not up is a no-op (exit 0). Never uses a name pattern, never `podman system prune`, never signals a process.

## Exits

0; 2 usage; 1 a resource of this project could not be removed.
