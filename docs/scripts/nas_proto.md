# client/nas_proto.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T17:25:20Z |
| Status | new in the working tree (not committed); the independent review is owed (constitution 11.4.142 / 11.4.209) |
| Source | `scripts/test-infra/client/nas_proto.py`; tests `tests/infra/nas_proto_unit.py`, `tests/infra/test_nas_protocols.sh`; driver `nas_protocols.md` |

## Purpose

The python client (stdlib only) of the NAS protocol survey, run inside `IMG-GO` through `scripts/containers/run_pinned.sh` by `nas_protocols.sh`: `nas_proto.py <rpc|ftp|ftps|sftp> <1-7> <ip> <out dir>`.
It writes `<out>/<proto>-<n>.json` (schema `wp12-nas-protocol/1`); `rpc` also writes `exports-<n>.txt` (the exported or proven-mountable paths for the NFS walker).

## Design notes

- ONC RPC over TCP with record marking, written out (portmap DUMP, NFS NULL, MOUNT EXPORT/MNT/UMNT): the only procedures called are 0, 1, 3, 4, 5.
- SFTP v3 is spoken directly over `ssh -s sftp` (INIT, OPENDIR, READDIR, OPEN with the READ flag only, READ, CLOSE) so that the 1 MiB read bound and the latency of each request are exact; the password comes from `SSH_ASKPASS` (a script that prints line 2 of the 0600 `cred` file).
- FTP/FTPS: `ftplib`; MLSD when offered, else `LIST`; the abandoned transfer after the read sample closes the control connection, which is reopened (and re-logged-in, paced) when needed.
- Error records keep the exception type and the three-digit reply code only, because a server message can echo an entry name (found and fixed by the unit oracle).
- `Pacer` enforces one request per second; bounds are module constants (`MAX_REQ_PER_SHARE` 30, `MAX_DEPTH` 3, `MAX_ENTRIES` 2000, `READ_BYTES` 1 MiB); `PORTMAP_PORT` / `NFS_PORT` are module attributes so the unit test can point the RPC code at a local fixture.
