# blocked_external.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T00:00:00Z |
| Status | new in the working tree (T135), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
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
