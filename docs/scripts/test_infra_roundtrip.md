# roundtrip.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T132, T134), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/roundtrip.sh`, `scripts/test-infra/client/roundtrip_*.sh`; tests `tests/infra/test_roundtrip.sh` and the wrappers `tests/infra/test_roundtrip_<protocol>.sh` |

## Purpose

One real round trip against one protocol of a running project, from IMG-INFRA-CLIENT: PostgreSQL (create, insert, read, update, duplicate key refused, delete, drop, wrong password refused), Redis
(SET, GET, INCR, EXPIRE, DEL, unauthenticated refused), FTP in passive mode, SMB and WebDAV (corpus files incl. unicode names compared with the manifest sha256, upload, download back, delete, wrong
credential refused), NFS (libnfs user-space client against the unfs3 server: write and read back of 0 B, 1 B, 20000 B, 1 MiB and a unicode name, listing sizes, second CREATE refused with NFS3ERR_EXIST,
unexported path refused). Every step prints `STEP <name> ok|FAIL`; the verdict is the last line `PASS|FAIL roundtrip <proto> steps=<n>`. MinIO: `BLOCKED roundtrip minio reason=image_unavailable`, exit 3.

## Usage

```bash
scripts/test-infra/roundtrip.sh --build-id <id> --protocol postgres|redis|ftp|smb|webdav|nfs|minio [--record <evidence-dir> --iteration <n>]
```

`--record` runs the round trip through `tools/evidence/evrec` (item `RUN-132`, `RUN-134` for nfs; polarity GREEN; evidence class runtime; oracle `specified`; test source the client script): one `ev/1`
record is appended to `<evidence-dir>/ledger.jsonl` (anchor and blobs beside it), verified with `tools/evidence/verify`. `tools/evidence/wrap-bash.sh` does not exist yet (T051); `evrec run` is used directly.

## Exits

0 PASS; 1 FAIL; 3 BLOCKED; 2 usage.
