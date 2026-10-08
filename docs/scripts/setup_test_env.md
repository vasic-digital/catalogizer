# setup-test-env.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/setup-test-env.sh`; test `tests/infra/test_consumers.sh` |

## Purpose

Starts REAL test infrastructure for integration tests through `scripts/test-infra/up.sh` (per-run credentials, random loopback ports, lease, protocol-level readiness) and prints the op_id, the ports and the exact `down.sh` command. A failed start exits non-zero with up.sh's diagnostics: it is never reported as "not available". Removed: the stub `docker-compose.test-infra.yml` generator (fixed ports 1445/2121, literal `testuser`/`testpass`) and the silent `|| echo "Could not start"` fallbacks; the compose file is a tracked, digest-pinned part of the repository.

## Usage

```bash
scripts/setup-test-env.sh [--build-id <id>] [--services postgres,redis,ftp,smb,webdav,nfs]
```

## WF17 fix round 5 (revision 3)

- The default build id carries the checkout hash (unique per second AND per checkout); valued options are validated; the exit-class messages say what is LEFT (3 another owner's stack, 4 a stale lease, 2 usage). INT, TERM and HUP tear the start down through `up.sh`'s own EXIT trap (`tests/infra/test_signals.sh`).
