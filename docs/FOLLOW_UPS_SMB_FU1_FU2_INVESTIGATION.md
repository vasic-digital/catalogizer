# SMB-FU-1 / SMB-FU-2 investigation (read-only)

Repo `main` 8153cd27, 2026-11-11. No code was changed. Items not proven by running code are marked UNCONFIRMED.
Paths are relative to `catalog-api/` unless stated otherwise.

## Naming correction (matters for the conclusion)

There are two `handlers` packages, and FU-1 mixes them:

- Root package `catalogizer/handlers` (`handlers/download.go`, `handlers/copy.go`). Imported in `main.go:11` as `root_handlers`.
- Production package `catalogizer/internal/handlers` (`internal/handlers/copy.go`, `download.go`).
  - `CopyToSMB`, `ListSMBPath` (the `/smb/list` handler) and `GetSMBHosts` are methods of this package's `CopyHandler`, not of the root one.
  - The root `CopyHandler` method is named `CopyToSmb` (`handlers/copy.go:49`).

## Part 1 - dead or alive (SMB-FU-1)

### 1a. Root `handlers/download.go` and `handlers/copy.go`: DEAD in production. Evidence:

- The only constructor calls in non-test code are `main.go:699` and `main.go:702`. They use the `handlers.` alias, which is `catalogizer/internal/handlers` (`main.go:16`).
  - Those constructors take 6 and 4 arguments (`catalogService, smbService, ...`).
  - The root constructors have the signatures `NewDownloadHandler(fileRepo, tempDir, maxArchiveSize, chunkSize)` (`handlers/download.go:33`) and `NewCopyHandler(fileRepo, tempDir)` (`handlers/copy.go:29`).
  - A grep for `root_handlers.(New)?(Download|Copy)` in `main.go` finds nothing. `root_handlers` is used only for Subtitle (`main.go:692`), Auth (`main.go:705`) and similar.
- A grep for `NewDownloadHandler|NewCopyHandler` across `catalog-api` finds, outside the root package's own tests, only the internal-package calls.
- The only other importers of `catalogizer/handlers` are `internal/handlers/eventbus_bridge.go`, `internal/handlers/wf22_r3_bridge_test.go` and `tests/recommendation_handler_test.go`. None references Download or Copy.
- The two root files carry no build tags. They compile into the binary only because the package is imported; no route reaches them.
- Referenced by tests: `handlers/download_test.go`, `copy_test.go`, `copy_security_test.go`, `download_info_cat001_test.go`.

History:

- Both files were added together in `16843b13` ("Implementations", 2025-10-04), the same commit that added the internal-package twins.
  - At that commit `main.go` imports only `catalog-api/internal/handlers` (`main.go:5`) and routes `/copy/smb`, `/smb/list/*path` and `/smb/hosts` to the internal handler.
  - In that commit the root files were never routed.
  - They are a parallel earlier-API copy. Evidence: `handlers/copy.go:48` documents `@Router /api/copy/smb` (no `/v1`), while the internal copy documents `/api/v1/copy/smb`.
- Later edits touched them: `e8772b52` (scanner identity), `23f2bf4f` (copy-handler path containment) and `33d9f352` (download 501 for non-SMB). All were hardening or test-driven. None ever wired a route, so they were never "alive".
- UNCONFIRMED: whether the root package was intended as the successor to the internal one. Nothing in the git log says so.

Conclusion: dead (unrouted) but tested. Recommendation: KEEP for now and do not delete under §11.4.124. Re-evaluate removal as a deliberate step:

- A removal PR must carry the tests that go with it: roughly 4 root test files plus `copy_security_test.go`.
- The Go test coverage of the `catalogizer/smb` confinement paths in these files should be reproduced elsewhere first (`internal/handlers/smb_subdir_routed_test.go`).
- Low-risk alternative: add a `// Deprecated: not routed; see internal/handlers` header and a CONTINUATION note.

### 1b. `CopyToSMB`, `ListSMBPath`, `GetSMBHosts` in `internal/handlers/copy.go`: unrouted, live code in a routed type.

- Defined at `internal/handlers/copy.go:43`, `:304` and `:371`.
- Routed originally at `main.go` in `16843b13` (`POST /copy/smb`, `GET /smb/list/*path`, `GET /smb/hosts`).
- Route removed in `5a38b782` (2025-10-07, "Protocols added", author Milos Vasic). That commit introduced the protocol-agnostic filesystem layer (`filesystem/factory.go` and others).
  - It replaced the routes with `/copy/storage`, `/storage/list/*path` and `/storage/roots`, and it replaced `smbService` with `fileSystemService` in `NewCopyHandler`.
  - The handlers were left behind. Today `main.go:1262-1264` routes only `CopyToStorage`, `CopyToLocal` and `CopyFromLocal`.
- Present references: `internal/handlers/copy_test.go` (including `GetSMBHosts_NilService` and `/smb/list` cases), `coverage_test.go:785-900`, and `copy_security_test.go:45-46`. The last one, `newProductionCopyRouterWith`, registers `/copy/smb` and `/smb/list/*path` on a test router.
  - Registration there comes from the test, not production routing.
- Docs that still advertise them: `README.md:368`, `docs/README.md:43`, `docs/examples.md:233`, and `docs/phases/PHASE_2_DEAD_CODE.md:126`, where `CopyToSMB()` is already flagged "Redirects to non-existent endpoint". No client uses them (`catalog-web` and the Kotlin clients do not call `/copy/smb` or `/smb/list`).
- Security state: `ListSMBPath` calls `pathsafe.SafeRemotePath` (`copy.go:316`) and `smbService.ListFiles`, which goes through confined `withShare`. So the code is safe even if re-wired.

Conclusion: dead as an HTTP route since 2025-10-07; the code is still compiled and tested. Recommendation:
(a) KEEP as is for this phase.
(b) Open an operator decision (§11.4.66) to remove `CopyToSMB`, `ListSMBPath`, `GetSMBHosts` and fix the three docs. Do not route them: the storage-root routes are the replacement, they sit behind `adminGate`, and a new unrouted-to-routed move would be a new attack surface.

## Part 2 - `internal/services/smb.go` and `smb_discovery.go` (SMB-FU-2)

### 2a. `smb.go` is NOT affected (the FU note is imprecise)

Every call goes through `withShare` (`internal/services/smb.go:161`). It looks the host up in `s.config.SMB.Hosts`, calls `smb.SplitShare(smbHost.Share)` (`:172`), and wraps the share in `confinedShare` (`:199`), which applies `smb.ResolveSub` on all methods (`:150-157`). The share and sub-directory come from server configuration, not from the request body.
Call sites that use `withShare`: `ListFiles` :203, `StreamFile` :221, `DownloadFile` :269, `UploadFile` :319, `FileExists` :422, `CreateDirectory` :546, `DeleteDirectory` :566, `DirectoryExists` :600.
Residual by contract: `CreateRemoteDir` (`:392`) takes a raw `*smb2.Share` and is unconfined by design (also noted as SMB-FU-7); it has no HTTP caller found.

### 2b. `smb_discovery.go`: the real instance of FU-2

Unconfined mounts that take request-body values:

| Call site | Mounts | Reachable via |
|---|---|---|
| `internal/services/smb_discovery.go:167` `TestConnection` | `session.Mount(config.Share)` | `POST /api/v1/smb/test` (`main.go:1388`) |
| `:215` `BrowseShare` | `session.Mount(config.Share)`, then `share.ReadDir(path)` at `:222` | `POST /api/v1/smb/browse` (`main.go:1390`) |
| `:124` `testShareAccess` | `session.Mount(shareName)` | no caller found (unused helper) |
| `:57` `DiscoverShares` | none (SRVSVC share-name listing only) | `POST /smb/discover` |

Request fields are in `internal/handlers/smb_discovery.go`: `Share` (required) and `Path` at `:61` and `:65`; `BrowseShare` handler at `:156`.

### 2c. Can `ResolveSub` be applied? Yes, partly

- `BrowseShare` has two inputs: `Share` (a bare name today) and `Path`. If `SplitShare` were applied to `Share` the result `(share, sub)` could feed `confinedShare` and `ResolveSub(sub, path)` could cover `Path`, exactly as `withShare` does.
- Verified in the dependency (go-smb2 v1.1.0 `client.go:98-107`, `path.go:98-109`): `Mount("share/sub")` normalises `/` to `\`, treats it as already containing a backslash, and fails `validateMountPath`. So today "share/sub" in the request fails to mount; a sub-directory is simply not supported. (Not run live: UNCONFIRMED.)
- `Path` is passed verbatim to `ReadDir`. go-smb2 `validatePath` only checks the leading backslash and `/` use; `..` is not rejected client-side. Whether the server rejects `..` is server-dependent: UNCONFIRMED.

### 2d. Security exposure

Can a share or sub-directory escape be reached? Not as a privilege boundary, in my reading:

- Routes are admin-only (`smbGroup.Use(adminGate)`, `main.go:1383-1384`).
- The caller supplies the SMB host, username and password in the same request (`BrowseShareRequest`, `smb_discovery.go:58-66`). The code never uses stored credentials for these routes.
- Whatever the request reaches is bounded by that credential's own rights on that server. No catalogizer-held secret is exposed.
- There is no stored sub-directory root for these requests, so there is nothing to escape from.

Concrete requests (code-reading only, no live attack):
1. `POST /api/v1/smb/browse {"host":"nas","share":"corpus","username":"u","password":"p","path":"../.."}`.
   Reaches `share.ReadDir("../..")` at `smb_discovery.go:222`. Client-side not rejected. Outcome depends on the server (Windows/Samba normally refuse `..`; UNCONFIRMED). The credential could read anything it was granted anyway.
2. `{"share":"corpus/nested"}` fails at Mount (see 2c) and returns the HTTP 500 "Failed to browse share" (`smb_discovery.go:187`).
3. `{"share":"IPC$"}` or `{"share":"C$"}` on `/smb/browse` or `/smb/test` mounts the admin share, because `SplitShare` is not applied.
   The response leaks the error text (`"Failed to browse share: " + err.Error()` at `:187`).
   Where the supplied credential has the rights, `BrowseShare` lists the contents (for `C$` the whole drive).
4. SSRF / internal port probing: only `ProbeHost` (`:237-249`) calls `isHostAllowed` (blocks loopback, link-local, multicast, metadata addresses). `DiscoverShares` (`:79`), `TestConnection` (`:110`) and `BrowseShare` (`:156`) do not. `Port` is attacker-chosen in `TestConnection` and `BrowseShare` (default 445). The response differs between connect failure and success, so an admin can use them to scan hosts and ports reachable from the server. This is a distinct, larger finding than the share/sub one.
   Severity: MEDIUM for an admin-only route; the admin is already trusted with the host's storage and network per the comment at `main.go:1255-1259`.

Severity summary:
- Sub-directory / `..` confinement gap: LOW. Admin-only, caller-owned credentials, no stored sub-root to break. It is an inconsistency with the confined storage-root path (§11.4.6: not a proven escape).
- Admin-share mounting (`IPC$`, `C$`) and SSRF without `isHostAllowed`: LOW to MEDIUM. Admin-only. Recommend fixing because `SplitShare` already encodes the policy and `isHostAllowed` already exists.

### 2e. Minimal fix design (not applied)

In `BrowseShare` and `TestConnection` of `internal/services/smb_discovery.go`:
1. `shareName, sub, err := smb.SplitShare(config.Share)`; on error return it, which the handler maps to a 400. This also rejects `IPC$`, `ADMIN$`, `C$`, device names, `\`, control characters.
2. `session.Mount(shareName)`; wrap `raw` in a `confinedShare{in: smb2ShareAPI{sh}, sub: sub}` (both are in package `services`) when `sub != ""`.
3. Replace `share.ReadDir(path)` with the confined `ReadDir(path)`. Normalise `Path` through `ResolveSub`, or `pathsafe.SafeRemotePath` for the no-sub case, so `..` never reaches the server.
4. In the handlers add the same `isHostAllowed(req.Host)` check that `ProbeHost` uses to `DiscoverShares`, `TestConnection` and `BrowseShare`, and validate `Port` (for example 1..65535, preferably 445 only).
5. Return generic errors instead of `err.Error()` for the share-mount failure.
6. Tests to add: golden-bad requests (`share:"IPC$"`, `path:"../.."`, `host:"127.0.0.1"`, `port:22`) expecting 400, plus a golden-good `share:"corpus/nested"`. Use `NewSMBServiceWithOpener`-style seam or a small `openShare` seam in the discovery service for unit class; live proof stays with the existing `docs/qa/smb-subdir-20261110/run.sh` pattern (§11.4.289).

## Open items

- UNCONFIRMED: server-side behaviour on `..` and `IPC$` listing; needs a live run on a real SMB server (the cattsrv-smb rig).
- UNCONFIRMED: intent behind the root `handlers/` duplicate; ask the operator before any removal.
- Other `/smb` handlers: `internal/handlers/smb.go` (`SMBHandler`, with its own `TestConnection` at `:187`) is not constructed in `main.go` (no `smbHandler` match); not examined further.
