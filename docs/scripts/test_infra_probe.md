# probe.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T128), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/probe.sh`, `scripts/test-infra/client/probe_*.sh`; tests `tests/infra/test_probes.sh` |

## Purpose

Protocol-level probes of a running project: PostgreSQL `SELECT 1` answers 1; Redis authenticated `PING` answers `PONG` and the unauthenticated one is refused; FTP login, passive listing and a
passive transfer whose sha256 equals the corpus manifest; SMB share list shows `testshare` and the share lists the corpus; WebDAV `PROPFIND` answers 207 with the corpus and the unauthenticated request
gets 401; NFS (only when named) MOUNT and READDIR of `/export` through libnfs. An open TCP port is not a pass: a postgres probe pointed at a listener that accepts TCP and says nothing FAILs.
MinIO is reported `BLOCKED` (no obtainable image), never `PASS`.

## Usage

```bash
scripts/test-infra/probe.sh --build-id <id> [--services postgres,redis,minio,ftp,smb,webdav,nfs | all]
```

Output: `PROBE <proto> PASS|FAIL|BLOCKED <detail>` per protocol, then `PROBES pass=<n> fail=<n> blocked=<n>`. Every client runs in IMG-INFRA-CLIENT through `run_client.sh`: never a host client, never an
`exec` into a service container. `TI_PROBE_RUNNER` set to anything but `container` is refused (`probe_runner_not_container`). `TI_PROBE_HOST_<PROTO>` overrides a service host (the carrier fixture).

## Exits

0 no probe failed and at least one passed; 1 a probe failed; 3 every requested probe is BLOCKED; 2 usage.
