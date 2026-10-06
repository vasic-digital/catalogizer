# nfs_fallback_state.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T134a), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/nfs_fallback_state.sh`; tests `tests/infra/test_nfs_fallback_state.sh` |

## Purpose

Records the terminal state of the owner-host NFS fallback leg (T134a, ODG-08): `not_needed` only when the T134 record's `state` field is `pass` (the record cites that file's sha256 and state);
`blocked` with reason `odg08_unanswered` otherwise.

## Usage

```bash
scripts/test-in-container.sh tooling unit -- bash /src/scripts/test-infra/nfs_fallback_state.sh --attempt-json F --state not_needed|blocked [--out F]
```

Refusals (exit 1): `nfs_attempt_not_pass` (`not_needed` claimed while the T134 state is not `pass`), `fallback_not_owed` (`blocked` claimed while it is `pass`), `attempt_record_missing`, `attempt_record_malformed`.
