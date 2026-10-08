# blocked_external.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
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

## WF17 fix round 5 (revision 3)

- The env file is read by `dotenv_get.py` (CRLF, `export`, spaces around `=`, quotes, inline comments, last duplicate wins), the same reader as the NAS leg, so the two scripts can no longer disagree about a file. An IP variable that is not an IPv4 address is `credentials_invalid` (the leg refuses it too). The fallback record is validated (closed state set, reason, boolean `proves_kernel_mount_path`) and `derived_from` is its real path. `--out` needs a value and a failed write exits 1.
