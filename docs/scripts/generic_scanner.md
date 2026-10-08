# generic_scanner - Companion Guide (PA-03, WP-12)

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08T00:00:00Z |
| Status | in the working tree (PA-03 / tasks T361, T362, ODG-20), not yet committed. Revision 2 is the fix round for the independent review WF22 (NO-GO, 17 IMPORTANT findings); a fresh independent review of revision 2 (11.4.142 / 11.4.209) is owed; its row in `docs/scripts/README.md` is owed (that file is not touched by this change) |
| Source | `catalog-api/internal/services/generic_scanner.go`, `scan_redact.go`, `universal_scanner.go` (job, `insertFileRecord`); unit tests `generic_scanner_test.go`, `generic_scanner_job_test.go`, `generic_scanner_regression_test.go`, `generic_scanner_fake_test.go`, `wf22_fix_r2_test.go`, `wf22_createclient_gate_test.go`, `wf22_review_*_test.go`; real-service tests `catalog-api/tests/realfs/` (build tag `realfs`); WebDAV `catalog-api/filesystem/webdav_propfind.go`, `webdav_client.go`; FTP `ftp_client.go`; list limit `list_limit.go` |
| Evidence | `specs/001-full-project-audit-remediation/evidence/wp12/scanner/` |

## Purpose

One breadth-first scanner over the `filesystem.FileSystemClient` interface. It replaces the FTP, NFS and WebDAV scanner bodies that were empty (`return nil`), which let `processScanJob`
set the status `completed` for a populated root with 0 files found. Every protocol registered through `filesystem.RegisterProtocol` (sftp, ftps, nfs3, ...) is scanned by the same code.

## Guarantees

| Guarantee | Mechanism |
|---|---|
| Read only | the client is wrapped in `decorators.ReadOnly`; a mutating call returns `ErrReadOnly` and never reaches the server |
| Host safety | every operation takes a slot of the per-host `fabric.HostBudget` (default `fabric.NASBudget()`: 4 concurrent, 10 starts/s), shared by the generic scanners for one host key (`host[:port]`, or the URL host). The SMB and local scanners take no budget, and two spellings of one NAS (IP and name) get two budgets |
| Retries | `fabric.Retrying` on reads only; positively transient failures (reset, timeout, EOF) are retried (default 3 attempts, 100 ms base, 2 s cap); an authentication failure is never retried |
| Bounded | depth = `job.MaxDepth` (default 10 when unset, `<= 0` means the default) capped at 64; at most 5,000,000 entries per scan and 1,000,000 per directory. **Hitting a bound FAILS the scan (`limit_exceeded`), it never truncates silently.** Depth: the first directory BELOW the bound is listed once; content there fails the scan. Entries: the remaining budget travels to the client as a list limit (`filesystem.WithListLimit`; the WebDAV client stops decoding at it), a wave of at most `Workers` directories is listed and processed before the next wave, so the scan stops within one wave of the bound and never holds a whole level. The WebDAV answer is bounded to 64 MiB (an error, not a truncation) |
| Honest failure | a `*ScanError` with `Kind` `root_unreadable`, `transport_error`, `empty_root`, `limit_exceeded`, `storage_failure`, `partial_scan`, `timeout` or `cancelled`; `processScanJob` ends the job `failed` (or `cancelled`) and stores the text in `ScanStatus.Reason` (API field `reason`). A deadline is `failed` with kind `timeout`; only a cancellation of the job context is `cancelled` |
| Empty root | by default an empty answer FAILS (`empty_root`): it is also what an unreachable or mis-addressed source looks like. A root that is expected to be empty sets `allow_empty` (column `storage_roots.allow_empty`, default false, migration v21, API field on `POST /storage/roots`). A share holding only `@eaDir` / `#recycle` / `#snapshot` is empty in the same sense |
| Unreadable sub-directories | skipped (counted in `ErrorCount`, named in the status note `reason` of the completed scan) only when positively "not readable by this account"; more than half of the sub-directories unreadable FAILS the scan (`partial_scan`, naming the first five). More than half of the records unstorable FAILS it (`storage_failure`) |
| Incremental | `ChangeToken` = mtime(ns) + size + kind (+ etag, quoted, + inode when the client implements `TokenSource`); the string form is injective. With `ScanType` `incremental` and a token store, unchanged entries are examined but not re-recorded. **A scanner without a token store does not claim to be incremental** (`SupportsIncrementalScan()` is false for every production scanner until a persistent store exists) |
| Progress / cancel | `status.CurrentPath` and the counters advance per directory and entry; the context is checked between entries, between waves and while a listing is in flight: a client that ignores the context (the FTP library) no longer holds the scan - the abandoned call finishes on its own and its answer is dropped |
| Deterministic | entries are processed sorted by name; the catalog result does not depend on `Workers` |
| Catalog paths | slash paths relative to the root WITHOUT a leading slash (`music/b.flac`), the form the SMB and local scanners write and `ensureDirectoryPathExists` builds parents in. The scanner works with slash-rooted paths internally and converts at the database seam (`catalogPath`). Names are stored exactly as the server reports them (NFC and NFD spellings stay two paths); a name that is empty, `.`/`..`, contains `/` or a control character, or is longer than 255 bytes is rejected and counted as an error; `.` and `..` entries (MLSD `cdir`/`pdir`) are dropped silently |
| Stored once | `insertFileRecord` UPDATEs the row of `(storage_root_id, path)` and INSERTs only when there is none: a rescan keeps `files.id` (media files point at it), `created_at` and hashes, revives a soft-deleted row and updates what changed. (`INSERT OR REPLACE` renumbered the ids; `ON CONFLICT DO UPDATE` is not available: the SQLite bundled with go-sqlcipher predates 3.24) |
| No credential in a reason | the clients redact URLs they put in errors (`url.URL.Redacted`), and `ScanStatus.fail/cancel/setNote` plus the event payload scrub URL userinfo and the root's literal secrets (password, URL password) |

Directory listing errors below the root are skipped only when they are positively "not readable by this account" (`os.ErrPermission`/`ErrNotExist` by type, FTP 550/553, "permission denied", "STATUS_ACCESS_DENIED", HTTP 403/404, ...).
Anything else fails the scan: skipping an unrecognised failure turns an outage into a smaller catalog. `@eaDir`, `#recycle` and `#snapshot` directories (Synology) are never entered, at any depth.

## Through the API (revision 2)

`POST /api/v1/storage/roots` takes `url`, `mount_point`, `options` and `allow_empty` besides the earlier fields; the protocol must be one the client factory builds (the built-ins plus every protocol registered through
`filesystem.RegisterProtocol`); a webdav root needs a `url`; `port` must be 1-65535. `POST /api/v1/scans` loads every column the settings contract consumes (`models.StorageRootConnColumns`). `GET /storage/roots` returns the
url with its password redacted. `GET /scans[/:id]` returns `reason`. The cover-art thumbnail copy builds its client through the same single mapping.

## Not done here (tracked, not claimed)

- Change tokens are kept in memory (`MemoryTokenStore`); a database-backed store (a migration) is a separate work item. Deleted files are not detected (`FilesDeleted` stays 0). Hence no production scanner is incremental.
- A cancel endpoint (`ScanJob.Context` is `context.Background()` in the only producer, so a user cannot cancel), the `reason` field in `catalog-web` (`scansApi.ts`) and a UI notification for `scan.cancelled` are tracked, not done (outside `catalog-api`).
- NFS: no real-target evidence. The NFS client mounts with `syscall.Mount` (needs CAP_SYS_ADMIN), which the rootless / no-sudo mandate makes structurally unavailable on the build host (11.4.112); its row is `blocked-unavailable`. The unit tests cover its settings and scanner behaviour, not a real mount.
- `job.IncludePatterns` / `job.ExcludePatterns` are not applied (the LocalScanner does not apply them either).
- ETag / inode extras exist as the optional `TokenSource` interface; no client implements it yet (PA-04/05/06/09).
- Defaults of `Workers`: ftp 1 (one control connection), webdav 1, nfs 4, pluggable protocols 1. Raising them is PA-07.

## Real-service run

```bash
scripts/test-infra/up.sh --build-id <id> --services ftp,webdav
# build the test binary in IMG-GO into the writable scratch area, run it on the stack network in IMG-INFRA-CLIENT
scripts/containers/run_pinned.sh --rw .audit/scratch IMG-GO -- bash -c 'cd catalog-api && CGO_ENABLED=1 go test -tags realfs -c -o /src/.audit/scratch/<dir>/realfs.test ./tests/realfs/'
scripts/test-infra/run_client.sh --build-id <id> -- /src/.audit/scratch/<dir>/realfs.test -test.v -test.count=1
scripts/test-infra/down.sh --build-id <id> --op-id <the op_id up.sh printed>
```

The tests FAIL (never skip) without the stack environment (`TI_FTP_USER`, `TI_FTP_PASSWORD`, `TI_WEBDAV_USER`, `TI_WEBDAV_PASSWORD`, the manifest `/manifest.sha256`). The oracle is the corpus manifest:
the scan must find exactly its files. Revision 2 adds the database half (`TestRealDB_*`: one row per entry, slashless paths, correct parents, ids stable across a rescan, no error counted for a clean corpus), the FTP root path in four
spellings, the WebDAV URL path joined with the root path, the credential-free reason, and `allow_empty` on real empty directories. Not covered by any real test: NFS (see above), the HTTP handler (handlers package tests drive it
against an httptest WebDAV server and a real SQLite database).
