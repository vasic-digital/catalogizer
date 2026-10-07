# down.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T05:00:00Z |
| Status | committed in 6d5ebb64; revised after the WF12 independent review (NO-GO); a fresh independent review of the revision is owed (constitution 11.4.142) |
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
