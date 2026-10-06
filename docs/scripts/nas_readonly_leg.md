# nas_readonly_leg.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T132 (real-NAS leg)), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/nas_readonly_leg.sh`, `scripts/test-infra/client/nas_smb_ro.sh`; tests `tests/infra/test_nas_readonly_leg.sh` |

## Purpose

The clearly separated READ-ONLY leg against the owner's real Synology hosts (`docs/infrastructure/synology-hosts.md`; ODG-08, ODG-01). Not part of the deterministic suite (the in-container Samba is).
Per host: the share list, one depth-1 listing of the root of one DATA share, and at most ONE bounded read (the smallest regular file of at most 64 KiB). At most 2 requests per second. Never a write,
delete, rename, mkdir or any other change; the client script holds only `ls` and `get` as smbclient commands, and the test scans for any other verb.

## Credentials

Only from the gitignored env file (`<repo>/.env`, or `TI_ENV_FILE`): `SYNOLOGY_SMB_USER`, `SYNOLOGY_SMB_PASSWORD`, `SYNOLOGY_IP_<n>`. The file is parsed (`NAME=value` lines), never sourced. The values go
to an auth file (mode 0600, inside the run's out directory) read by the client and deleted on every exit path; never argv, never the environment of any process, never logs or evidence. Entry names and
file content are never recorded: only counts, the sha256 of the sorted names, and the size and sha256 of the one small file. No IP address is recorded.

## Usage

```bash
scripts/test-infra/nas_readonly_leg.sh [--hosts 3,4] [--json-out FILE]
```

Without the env file: `SKIP nas_readonly reason=env_absent` (exit 0). With the file but missing variables: `SKIP nas_readonly reason=credentials_absent variables: <NAMES>` (exit 0). Exit 1 when a host leg failed.
