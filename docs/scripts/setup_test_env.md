# setup-test-env.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T05:00:00Z |
| Status | WF12 F1 fix (consumer of the T129 compose contract), independent review owed (constitution 11.4.142) |
| Source | `scripts/setup-test-env.sh`; test `tests/infra/test_consumers.sh` |

## Purpose

Starts REAL test infrastructure for integration tests through `scripts/test-infra/up.sh` (per-run credentials, random loopback ports, lease, protocol-level readiness) and prints the op_id, the ports and the exact `down.sh` command. A failed start exits non-zero with up.sh's diagnostics: it is never reported as "not available". Removed: the stub `docker-compose.test-infra.yml` generator (fixed ports 1445/2121, literal `testuser`/`testpass`) and the silent `|| echo "Could not start"` fallbacks; the compose file is a tracked, digest-pinned part of the repository.

## Usage

```bash
scripts/setup-test-env.sh [--build-id <id>] [--services postgres,redis,ftp,smb,webdav,nfs]
```
