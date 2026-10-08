# nfs_attempt.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/test-infra/nfs_attempt.sh`; tests `tests/infra/test_nfs_terminal_state.sh`, `tests/infra/test_roundtrip_nfs.sh` |

## Purpose

Runs the unprivileged NFS attempt end to end and records its terminal state: build the image (`nfs_build.sh`), start the project with the `nfs` service, three round trips each recorded as an `ev/1`
record, tear the project down by label, then `nfs_terminal_state.sh` (through `TIC tooling unit`) reads the client verdict first and writes `nfs-attempt.json`.

## Usage

```bash
scripts/test-infra/nfs_attempt.sh --ev-dir <dir> --client-json <wp11/nfs-client.json> [--build-id <id>]
```

Exit 0 when a terminal state was recorded (the state is in the record), 1 when none could be chosen.

## WF17 fix round 5 (revision 3)

- Correction (TI-I5): the attempt starts from an EMPTY record set (the previous observed file, scratch file and ledger are removed first) and writes `nfs-attempt-observed.json` (schema `nfs-attempt-observed/1`) only from this run. Each round trip cites the ledger sequence the recorder reported (`recorded seq=<n>`); a failed round trip is `"stored": false, "record": null` (evrec stores no GREEN record for a failing run, exit 65) and nothing is fabricated. The stale statement "three round trips each recorded" holds only when all three pass. `--attempt-json-out` was never an option of this script (the parser rejects it) and is no longer in the usage line.
- `--ev-dir` and `--client-json` must lie inside the checkout (`path_outside_repo`, exit 2); the default build id carries the checkout hash; INT/TERM/HUP tear the stack down (`ti_exit_on_signals`, EXIT trap `nfs_down`, which reads the op id from `<state>/op_id`). `tests/infra/test_nfs_attempt.sh` drives it against the real NFS stack with a fault stub at the roundtrip edge.
