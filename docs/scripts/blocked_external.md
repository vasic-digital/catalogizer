# blocked_external.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T05:00:00Z |
| Status | committed in 6d5ebb64; revised after the WF12 independent review (NO-GO); a fresh independent review of the revision is owed (constitution 11.4.142) |
| Source | `scripts/test-infra/blocked_external.sh`; test `tests/infra/test_blocked_external.sh` |

## Purpose

Records the legs of the real-service stack that need an external credential or device: `available`, or `blocked-unavailable` with the reason and the variable NAMES (never values), BLOCKED-ON ODG-01 for
credentials: `nas_smb_readonly` (`SYNOLOGY_SMB_USER`, `SYNOLOGY_SMB_PASSWORD`, `SYNOLOGY_IP_3`, `SYNOLOGY_IP_4` in the gitignored env file), `minio` (`image_unavailable`), `nfs_owner_host` (`odg08_unanswered`).
A record never says `pass`.

## Usage

```bash
scripts/test-infra/blocked_external.sh [--out FILE]      # env: TI_ENV_FILE (default <repo>/.env)
```

Exit 0 the record was written; 2 usage. Needs bash and jq.

## WF12 review fixes (revision 2)

- F7: the `nfs_owner_host` leg is DERIVED from the T134a record (`TI_NFS_FALLBACK`, default `specs/001-full-project-audit-remediation/evidence/wp10/nfs-fallback.json`): `blocked` -> `blocked-unavailable` with the record's reason and its `unconfirmed` path and a `derived_from` sha256; `not_needed` -> `not_needed`; absent or unreadable -> `blocked-unavailable` / `nfs_fallback_record_missing`. It is no longer hard-coded, so the two records cannot disagree.
