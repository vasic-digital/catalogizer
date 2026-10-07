# client/nas_proto_nfs.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T17:25:20Z |
| Status | new in the working tree (not committed); the independent review is owed (constitution 11.4.142 / 11.4.209) |
| Source | `scripts/test-infra/client/nas_proto_nfs.sh`; tests `tests/infra/test_nas_protocols.sh` (verb scan); driver `nas_protocols.md` |

## Purpose

Bounded READ-ONLY walk of the NFS exports of ONE host through the libnfs user-space client inside `IMG-INFRA-CLIENT` (no kernel mount): `nas_proto_nfs.sh <ip> <1-7> <exports file> <3|4|3,4>`. The exports file is written by
`nas_proto.py rpc` and lists the exported paths and any path a MNT attempt proved mountable. The only commands are `nfs-ls` and `nfs-cat` (the test fails on any other `nfs-*` verb).

Per export and version: one listing for the top level, then breadth-first to depth 3, at most 30 listing requests and 2000 entries, one request per second; counts, sha256 of the top-level names, extension and permission-bit histograms;
the first 1 MiB of ONE file read through `nfs-cat | head -c` and discarded. Timings include the MOUNT round trip (one `nfs-ls` = MOUNT + READDIR). Output `/out/nfs-<n>.tsv` (`R`, `LAT`, `EXT`, `MODEF`, `MODED` records),
assembled into JSON by `nas_protocols.sh`.

Verified against the unprivileged user-space NFS fixture of the test infrastructure (v3 walk and 1 MiB read; a path that does not exist reports `MNT3ERR_NOENT`; v4 is not offered by the fixture and is recorded as refused).
On the real hosts no path is mountable from this client (see `docs/infrastructure/synology-hosts.md`), so the walker has not run against a real NAS.
