# nas_readonly_leg.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T05:00:00Z |
| Status | committed in 6d5ebb64; revised after the WF12 independent review (NO-GO); a fresh independent review of the revision is owed (constitution 11.4.142) |
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

## WF12 review fixes (revision 2)

- F3: the leg's container sees a scratch view with the client script only, never the repository (the real `.env` is in it); `test_nas_readonly_leg.sh` samples the container mounts during the sentinel run and fails when the repository is among them (mutation `repo_mounted`).
- F11: the record's `dialect SMB3` became `max_protocol_requested` (the `-m SMB3` ceiling asked for; the negotiated dialect is never read) and `requests_per_second_max` became `request_rate_policy_max_per_second` (a policy limit enforced by the 1 s sleep). `writes_performed 0` holds by construction (section A scans for write verbs).
- F14: the test removes only the out directory of its own run (`nas-ro-<pid>`), never a glob (a real leg of another stream keeps its auth file). F18: the no-IP check matches any dotted IPv4, with a control needle.
