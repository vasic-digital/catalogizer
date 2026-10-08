# roundtrip.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
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

## WF17 fix round 5 (revision 3)

- `--record` takes an absolute path (made absolute against the caller's cwd); `--iteration` is a strict integer; a recorded pass must have GROWN the ledger by exactly one record (`record_not_appended` otherwise). Exits with `--record`: 0 PASS and one record; 65 the run did not pass (evrec refuses a GREEN record, nothing appended); 1 the ledger did not grow by one.
- The SMB `list_shows_upload` step now requires a directory-entry row for exactly that name (smbclient prints `NT_STATUS_NO_SUCH_FILE listing \<name>` on stdout for an absent file, which a substring match "found"); the FTP wrong-password step requires `Login failed: 530 ` at a line start. `tests/infra/test_roundtrip.sh` adopts the reviewer mutant RT1 (upload to another name) and an FTP refusal text `500 ... 530 bytes`, each pinned to the step that must fail.
- The readable per-run file is named `roundtrip-<proto>-plain-run<i>.txt`: it is a second, UNRECORDED run with its own nonce, not ledger record `<i>` (the ledger holds the recorded streams as blobs).
