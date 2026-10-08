# gen_env.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/test-infra/gen_env.sh`; tests `tests/infra/test_compose_files.sh` |

## Purpose

Writes the per-run credentials and random host ports of ONE compose project of the test-infrastructure stack (docs/16 section 10.4), so that no credential and no
host port is a literal in `docker-compose.test-infra.yml` or `docker-compose.build.yml` (finding D-06).

## Usage

```bash
scripts/test-infra/gen_env.sh --build-id <id> [--op-id <id>] [--env-out FILE] [--ports-out FILE]
```

`<id>` matches `^[a-z0-9][a-z0-9-]{0,30}$`; the compose project is `catalogizer-test-<id>`. Default outputs are `<repo>/.audit/test-infra/<project>/env` and `ports.env`
(`TI_STATE_DIR` relocates `.audit/test-infra`).

## Files

| File | Mode | Content |
|---|---|---|
| `env` | 0600 | `TI_PROJECT`, `TI_OP_ID`, `TI_DATA_DIR`, six `TI_PORT_*` (random free loopback ports), generated `TI_*_USER` / `TI_*_PASSWORD` for postgres, redis, ftp, smb, webdav and the (blocked) minio. Inside the gitignored `.audit/` tree; secrets; never copied into evidence. |
| `ports.env` | 0644 | the non-secret subset (`TI_PROJECT`, `TI_OP_ID`, `TI_PORT_*`): safe to copy into evidence. |

Passwords are 24 to 28 characters of `[A-Za-z0-9]` from `/dev/urandom`; two runs never give the same value. The values are never printed.

## Exits

0 written; 2 usage / invalid build id; 1 failure (no free ports, cannot write). The env file is written to a temp file in the same directory and renamed, with `umask 077`.

## WF12 review fixes (revision 2)

- F12: the header said "five distinct free TCP ports"; six are chosen (postgres, redis, ftp, smb, webdav, nfs). The draw is still bind(0)+close; the lifecycle script retries a start whose port was taken in between (up.sh, F16).

## WF17 fix round 5 (revision 3)

- Inputs: every valued option needs a value; `--env-out` / `--ports-out` are made absolute against the caller's cwd; an `--env-out` inside this checkout must be git-ignored (the credential file never lands in a tracked path) and may not equal `--ports-out` (that used to destroy the env file: the second write overwrote it with mode 644 and zero credential lines); the state path may not hold characters the env file or the volume syntax cannot carry; the op id is a longops safe name. The output now includes `env=` and `ports=` lines (callers read them instead of rebuilding paths), the env file carries `TI_ROOT_HASH` and its mode 0600 is verified after writing. Withdrawn: the documented direct use of this script with a bare compose call (TI-C6).
