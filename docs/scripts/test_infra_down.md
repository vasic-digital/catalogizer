# down.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/test-infra/down.sh`; tests `tests/infra/test_up_down.sh` |

## Purpose

Tears down ONE project by its label: containers, network and volumes carrying `catalogizer.test_project=<project>` (and `project=catalogizer`), nothing else. An unlabelled container whose name
starts with the project name survives (the carrier case); another project's resources survive. Then the registered operation gets the terminal state `complete`, the lease keeper exits by itself (its
file is removed; no signal is ever sent), and the state directory is removed with `podman unshare rm -rf` (the data files are owned by sub-uids of the container user namespace).

## Usage

```bash
scripts/test-infra/down.sh --build-id <id> [--op-id <op id>] [--keep-state] [--keep-logs]
```

Idempotent: a project that is not up is a no-op (exit 0). Never uses a name pattern, never `podman system prune`, never signals a process.

## Exits

0; 2 usage; 1 a resource of this project could not be removed.

## WF12 review fixes (revision 2)

- Ownership (F6): a project whose lease is held by a LIVE holder is torn down only by the caller that names its operation (`--op-id`, the `op_id=` line up.sh printed); any other caller gets `REFUSED reason=not_lease_owner`, exit 5, and nothing is touched. A holder proven dead (`scripts/longops/reap.sh --purpose <project> --dry-run`) is reaped by any caller: `reap.sh --op-id` records `reaped`, no signal is sent. No claim: no owner to check.
- Leaks (F2): the empty unlabelled pod `pod_<project>` is removed (a pod that still holds a container is left alone and said so), and so are `<repo>/.audit/out/<project>-client` and `<project>-seed` (exactly those two names of exactly that project; `<project>-logs` stays).
- Usage now: `down.sh --build-id <id> [--op-id <op id>] [--keep-state] [--keep-logs]`; exits: 0; 2 usage; 1 a resource could not be removed; 5 not the lease owner.
- The removal logic lives in `lib.sh` (`ti_rm_resources`, `ti_rm_out_dirs`) so `up.sh`'s retry reuses it.

## WF17 fix round 5 (revision 3)

- Exits are now: 0; 2 usage; 1 a resource could not be removed, or a podman query failed (the state is UNKNOWN: nothing is released or deleted); 5 REFUSED with one of `not_lease_owner` (the refusal does not print the owner's operation id), `holder_record_unreadable`, `start_in_progress` (a start of the project holds the project lock), `foreign_owner` (a resource of the project is not this checkout's), `project_lock_busy`.
- `--outcome complete|failed` and `--reason <code>` record how the operation ends (a failed start passes `failed`); a proven-dead holder is always `reaped`; an operation with no claim is reaped or closed, never left non-terminal. The lease is released LAST (after resources, output directories, logs, the state directory and the holder's keeper file), under the per-user project lock held for the whole run, so a new owner can never be destroyed by a teardown that already released.
- Ownership is the predicate in `lib.sh` (`ti_scan`): `project=catalogizer` + `catalogizer.test_project` + `catalogizer.test_root` (this checkout) + an operation in THIS registry; an unknown podman state means refuse, never "nothing there". `--keep-state` writes a `.keep-state` marker that `sweep_leaks.sh` honours.
