# nfs_terminal_state.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/test-infra/nfs_terminal_state.sh`; tests `tests/infra/test_nfs_terminal_state.sh` |

## Purpose

Chooses the ONE terminal state of the unprivileged NFS attempt (docs/16 DR-16-2, finding D-10) and writes `nfs-attempt.json` (schema `nfs-attempt/1`, `finding: D-10`). The client verdict file
(`wp11/nfs-client.json`) is read FIRST.

## Usage

```bash
scripts/test-in-container.sh tooling unit -- bash /src/scripts/test-infra/nfs_terminal_state.sh --client-json F [--attempt-json F] [--out F]
```

| Condition | State |
|---|---|
| client verdict UNVERIFIED or AMBIGUOUS | `blocked`, reason `nfs_client_unverified`, no round trip attempted, cites the sha256 of the client file and its verdict, owed to T134a / ODG-08 |
| verdict VERIFIED and >= 3 round trips, all ok | `pass` (states what it proves, a protocol round trip, and what it does not, the application's kernel-mount path) |
| verdict VERIFIED, failing step on the SERVER side, `nfs-ls` transcript attached | `structural_impossibility`, reason `rootless_cannot_provide_kernel_nfs`, scope bounded to the NFS server side |

Refusals (exit 1): `nfs_client_verdict_missing`, `nfs_client_verdict_unreadable`, `attempt_record_missing`, `attempt_record_malformed`, `client_side_failure_is_an_image_defect` (a failing CLIENT step is a defect of
IMG-INFRA-CLIENT fixed by a reviewed change, never a terminal state), `client_version_transcript_missing`, `attempt_inconclusive`.

## WF17 fix round 5 (revision 3)

- The record is validated, not trusted: schema `nfs-attempt-observed/1`, integer `iteration` and boolean `ok` per round trip, iterations exactly 1..n, and (for a pass) every cited record must resolve in `<attempt dir>/ledger-nfs/ledger.jsonl` to an entry with that `seq`, verdict `pass`, polarity GREEN and evidence class runtime (`record_not_in_ledger` otherwise). The client-version transcript must hold a line `^nfs-ls .*<digits>.<digits>`. The emitted record names the real repository-relative path of every input it read. Adversarial fixtures are in `tests/infra/test_nfs_terminal_state.sh`.
