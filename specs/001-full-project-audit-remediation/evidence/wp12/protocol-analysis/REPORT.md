# identity: WP-12 protocol-usage analysis (read-only), storage protocols in catalog-api
# head: 65e5f339 (main), working tree dirty (unrelated files); no repo file other than this report was written
# run_at: 2026-10-07; author: Sonnet worker; independent review (11.4.142): OWED, not yet performed
# scope: READ-ONLY. No NAS contact, no git write, no credential read or printed.
# honesty: every FACT cites file:line or a command run this session; everything else is marked UNCONFIRMED.

## 0. Top 10 findings

1. **FTP, NFS and WebDAV scanners are empty bodies that return nil** (`catalog-api/internal/services/universal_scanner.go:1044-1047`, `:1076-1079`, `:1108-1111`; each only a comment). `processScanJob` then sets status `completed` (`:342`) and publishes a success event. A scan of an FTP/NFS/WebDAV root reports success with 0 files: a PASS-bluff at the product layer (tasks T361/T362, ODG-20). Only Local, SMB scanners have bodies (`:540`, `:620`).
2. **SFTP (and FTPS) do not exist at all.** No `sftp`/`ssh` dependency (`catalog-api/go.mod:91,92,104` list only go-smb2, jlaffaye/ftp, gowebdav), the factory supports exactly `smb, ftp, nfs, webdav, local` (`filesystem/factory.go:91`), and `grep -i sftp` over spec.md/tasks.md/plan.md/research.md returns nothing. The owner now has SFTP and FTPS on all 7 NAS hosts, so two of the seven requested protocols are un-specified, not merely unimplemented.
3. **The NFS path is a kernel `mount(2)` and cannot work in the rootless target.** `filesystem/nfs_client.go:65` (`syscall.Mount`), `submodules/filesystem/pkg/nfs/nfs.go`; `isMounted()` is a stub that returns `true` after opening `/proc/mounts` (`nfs_client.go:102-111`), so `IsConnected` is true whenever the bool flags are set. Corroborated by `evidence/wp12/nfs-codepath.md`. A pure-user-space NFSv3 client removes the privilege requirement.
4. **Settings-key mismatches silently break NFS and FTP roots.** Scanner side writes `settings["export_path"]` for NFS (`universal_scanner.go` `storageRootToSettings`, nfs case), the factory reads `"path"` (`filesystem/factory.go:42`), so the NFS export is always `""`. The FTP case never forwards `Path`, WebDAV likewise (only `url`, `username`, `password`). Proven only by reading both sides (UNCONFIRMED at runtime until T361 runs).
5. **No connection reuse, no concurrency, no host budget.** Every scan job creates a new client, connects, scans serially, disconnects (`universal_scanner.go:310-338`); SMB recursion is a single goroutine, `ParallelDirectories:false` (`:678-686`); the only pool is SMB-only and lives in `catalog-api/smb/types.go:50-60` (default 10) used by download/copy handlers (`handlers/download.go:25,35`), unused by the scanner. The generic `ConnectionPool` interface (`submodules/filesystem/pkg/client/client.go:107`) has no implementation outside tests (grep, non-test).
6. **FTP client is serial, single control connection, no TLS, no MLSD tuning, fake mtime.** `ftp_client.go:42` `ftp.Dial` with only a timeout, no `DialWithExplicitTLS/TLS`, no `DialWithDisabledMLSD`; no mutex although scanner/handlers may share a client; `GetFileInfo` returns `ModTime: time.Now()` (`:145-146`), so change detection through it is meaningless, and it issues `FileSize` + a full `List` per stat (`:144,155`). Pinned `jlaffaye/ftp v0.2.0` while v0.2.4 (2026-08-22) is current.
7. **Read-only enforcement is absent at the client layer.** `FileSystemClient` exposes `WriteFile/CreateDirectory/DeleteFile/DeleteDirectory/CopyFile` on every protocol (`filesystem/interface.go:25`, `smb_client.go:148,227,239,251,263`, `ftp_client.go:115,232,245,258,271`). The NAS hosts are read-only by server config only. A `ReadOnly` decorator must make a mis-wired scan or move handler incapable of writing, independent of server ACLs.
8. **SMB: signing/encryption/multichannel unmanaged; NTLM only; one session per job, no keepalive/reconnect.** `smb_client.go:48-53` builds `smb2.Dialer` with `NTLMInitiator` only (no `Negotiator`, no Kerberos, no encryption requirement), no read-ahead sizing, no `Stat`-based change token. go-smb2 v1.1.0 is the pinned and latest tag; the upstream README (read via `gh api`) now lists AES-128/256 CCM/GCM encryption, DFS, Kerberos, SMB over QUIC, zero-copy I/O: which of these exist in v1.1.0 versus master is UNCONFIRMED. Multichannel and compound requests are NOT claimed by that README.
9. **Duplicate and divergent client implementations.** `catalog-api/filesystem/{ftp,smb,nfs,webdav}_client.go` and `submodules/filesystem/pkg/{ftp,smb,nfs,webdav}` are parallel copies (file sizes 305-518 vs 279-514 lines); `catalog-api/services/webdav_client.go` (392 lines) is a third, different WebDAV client (T376 BC-21 already lists the duplicate). `filesystem/interface.go:16-47` only aliases the submodule's types. Three copies guarantee drift; the protocol work must pick one home (recommendation: the submodule, consumed by reference).
10. **Test coverage of the real wire is thin and partly mocked.** `catalog-api/tests/mocks/smb_mock_server.go` and the NFS "mock" (`internal/tests/protocol_helper.go:157-171`) exist; the container fixtures (pure-ftpd, Samba, WebDAV in `docker-compose.test-infra.yml`, NFS via T134/T134a) and the 7-host SMB read-only survey (`evidence/wp12/nas-survey`, `nas-readonly-leg.json`: 0 writes, max 2 req/s) are SMB-only evidence for the NAS. No FTP/FTPS/SFTP/NFS evidence against the NAS exists yet.

## 1. Inventory: what catalog-api implements per protocol

Legend: REAL = wire code with a body; STUB = compiles, does nothing; MOCK = test-only.

| Protocol | Client (REAL?) | Scanner | Move handler | Config / factory | Tests found | Verdict |
|---|---|---|---|---|---|---|
| local | `filesystem/local_client.go` (284 l) REAL | `LocalScanner` body `universal_scanner.go:540-590` REAL | `LocalProtocolHandler` `protocol_handlers.go:12-62` (PerformMove is an intentional no-op, `:30-34`) | `base_path` | `local_client_test.go` | wired |
| smb | `filesystem/smb_client.go` (305 l, go-smb2) REAL; `OpenSeekable` `:135` | `SMBScanner` `:620-676` REAL (serial; logs `Warn` per directory `:655`) | `SMBProtocolHandler` `:64-160` | host,port,share,user,pass,domain; identity-index resolution in `storageRootToSettings` | `smb_client_test.go`, `smb_discovery_real_test.go`, `smb_probe_real_test.go`, mock server | wired; best covered |
| ftp | `filesystem/ftp_client.go` (310 l, jlaffaye/ftp v0.2.0) REAL but minimal | `FTPScanner` **STUB** `:1044-1047` | `FTPProtocolHandler` `:165-250` | `Path` not forwarded by scanner (`universal_scanner.go` ftp case) | `ftp_client_test.go`, `tests/mocks` | client usable, scan broken |
| nfs | `filesystem/nfs_client.go` (316 l, `//go:build linux`) kernel mount; darwin/windows variants (`nfs_client_darwin.go` 305 l, `nfs_client_windows.go` 104 l) | `NFSScanner` **STUB** `:1076-1079` (declares `ParallelDirectories:true`, `ChecksumCalculation:true` while doing nothing, `:1081-1090`) | `NFSProtocolHandler` `:254-345` | key mismatch `export_path` vs `path` | `nfs_client_test.go`, mock | broken, unprivileged runs impossible |
| webdav | `filesystem/webdav_client.go` (518 l, own net/http PROPFIND Depth 1, 30 s timeout, `:43,238`) REAL; `submodules/filesystem/pkg/webdav` (514 l) | `WebDAVScanner` **STUB** `:1108-1111` | `WebDAVProtocolHandler` `:346-436` | `url,username,password`; `path` not forwarded | `webdav_client_test.go`, `webdav_httptest_test.go` | client usable, scan broken |
| sftp | none | none | none | none | none | **missing** |
| ftps | none (jlaffaye/ftp has `DialWithExplicitTLS` / `DialWithTLS`, `ftp.go:295`, unused) | none | none | none | none | **missing** |

Other facts:
- Only caller of the factory in the app: `main.go:597` `filesystem.NewDefaultClientFactory()`; `processScanJob` (`universal_scanner.go:310`) is the sole path from storage root to client.
- `scanner.RegisterProtocolScanner` registers all four stubs/real scanners unconditionally (`:121-123`), so the API advertises protocols it cannot scan.
- Whether `sync_service.go:387` (`case "s3"`) or the configuration wizard (`configuration_wizard_service.go:273`) offer S3 while no S3 client exists is outside this scope; noted only because the wizard is the UI face of protocol choice (UNCONFIRMED relevance).

### 1.1 Against tasks.md and the spec
- **T361/T362** (`tasks.md:886-887`): RED expects `completed` with `files_found: 0` for seeded FTP/WebDAV/NFS trees (40 files, 5 dirs), then implement `FTPScanner`, `WebDAVScanner`, `NFSScanner` at `universal_scanner.go:1044-1111`. T362 is BLOCKED-ON ODG-20 (status `Operator-blocked`, `decisions/owner-decisions.yaml:527-538`; the answer `implement` was relayed by the conductor and is explicitly "not a verbatim owner quotation", `:1235`) and, for NFS, ODG-08.
- **T230** (`:599`) scanner detection against real SMB, FTP, WebDAV, NFS; **T346** (`:866`) traversal corpus over the clients; **T407** (`:943`) installer-side protocol testers (Rust, separate); **T127-T135** (`:382-392`) the WP-13 stack: FTP/SMB/WebDAV round trips exist; NFS per DR-16-2 (user-space NFS server container, T134, owner-host fallback T134a).
- **Not covered by any task: SFTP, FTPS, per-host capability probing, connection pooling, parallel listing, change detection, read-only decorator, host-safety budget, the settings-key mismatch fix.** These need either new tasks or an explicit scope extension of T361/T362. The owner's current ask (all protocols used maximally, tested against the 7 NAS hosts with SMB, NFS, FTP, FTPS, SFTP read-only) is wider than ODG-20.

## 2. Gaps versus best practice, per protocol

Format: finding, evidence, UNCONFIRMED marks where the claim is about a library's behaviour I did not run.

### 2.1 Cross-cutting
| Area | State | Gap |
|---|---|---|
| Connection reuse | none for scans; SMB pool only in `catalog-api/smb` | add a generic per-host pool keyed by (protocol, host, credential-id), idle timeout, max lifetime, health probe |
| Parallel listing | none (`ParallelDirectories:false` for SMB/FTP/WebDAV; the `true` on NFS is aspirational, no code) | bounded work-queue of directories (breadth-first), worker count from the per-host budget |
| Retries/backoff | none seen in the four clients (`Connect` fails once) | exponential backoff with jitter, retry only idempotent reads, classify errors (auth vs network vs not-found; cf. I-10 in T407) |
| Resume | none | ranged reads: SMB `Seek` exists; WebDAV `ReadStreamRange` exists in gowebdav (`client.go:355`) but the app's own client does not use it; FTP `REST`; SFTP offset reads |
| Change detection | `SupportsIncrementalScan()` returns constants (`:689`, `:1060`) with no implementation; FTP mtime fabricated | per-protocol change token: SMB (mtime+size, `FileID` if exposed, UNCONFIRMED in go-smb2 API), NFS (fattr3 mtime/ctime/fileid via READDIRPLUS), FTP MLSD `modify`+`size` (+ `unique` when offered), SFTP `Attrs` mtime/size, WebDAV `getetag`/`getlastmodified`; persist token, diff on rescan |
| Rate limiting / host safety | the NAS survey self-limited to 2 req/s (`nas-readonly-leg.json: request_rate_policy_max_per_second`), the application has no limiter | per-host token bucket + concurrency cap, defaults tuned for a NAS (low), config override; honour the repo's host-safety rules |
| Read-only | none | `ReadOnlyClient` decorator returning `ErrReadOnly` from every mutating method, mandatory for NAS-class roots; fuzz that no mutating call reaches the inner client |
| Secrets | passwords live in `StorageConfig.Settings map[string]interface{}` (`factory.go:16-52`) and root rows; SMB identities already resolve from `CATALOGIZER_IDENTITY_N_*` env (`universal_scanner.go` smb case) | extend the identity-indirection to every protocol; ensure `GetConfig()` (e.g. `ftp_client.go:308`) and logs never return or print the password (UNCONFIRMED: not audited line by line) |
| Path confinement | `strings.ReplaceAll("..")` style traversal handling called out in T346 | one shared `confine(root, rel)` used by all clients |
| Duplicates | three WebDAV, two FTP/SMB/NFS copies | single implementation (see 3.3) |

### 2.2 SMB (go-smb2 v1.1.0)
- Present: NTLM auth, share mount, `ReadDir`, `Open`+`Seek` (range-friendly, `smb_client.go:135-146`).
- Gaps: no explicit dialect/signing/encryption policy (`:48-53`; Synology may require signing, the survey used `max_protocol_requested: SMB3`, `nas-readonly-leg.json`); single TCP connection per job so SMB multichannel (not offered by the library per its README, UNCONFIRMED for v1.1.0) cannot help, and credits/compound requests are internal to the library, not tunable by the app (UNCONFIRMED). The practical lever is **several sessions in parallel** (pool) and **large sequential reads with read-ahead**. No `Stat` after `ReadDir` per entry is needed (ReadDir already returns `FileInfo`), good. `ListDirectory` builds the path via `filepath.Join` (OS-specific separator, `universal_scanner.go:640`): wrong on Windows hosts for SMB paths, minor. Kerberos/QUIC available in newer upstream, unused.
- Change notify (`SMB2 CHANGE_NOTIFY`) not exposed in the app interface, `RealTimeChangeDetection:false`. UNCONFIRMED whether go-smb2 v1.1.0 exposes it.

### 2.3 NFS
- Today: kernel mount (finding 3). Needs `CAP_SYS_ADMIN`; cannot be used in the rootless fixture, is a stub-check in `isMounted`, leaves stale mounts after a crash (no unmount guard seen on panic paths; UNCONFIRMED), and mount options default `vers=3` (`factory.go:46`).
- Best practice for a catalog crawler: user-space client speaking NFSv3 (MOUNT + READDIRPLUS gives name, type, size, mtime, fileid in one round trip, no per-file GETATTR) with `AUTH_SYS` uid/gid control, and 1 MiB `rsize` where the server allows. NFSv4 compound ops (PUTFH/LOOKUP/GETATTR/READDIR in one RPC), delegations, and `pnfs` need an NFSv4 client; no maintained pure-Go NFSv4 client was found (section 3.4), so v4 stays kernel-mount-only (optional, privileged) or via libnfs (cgo, `nfs-ls` CLI from IMG-INFRA-CLIENT as an oracle in tests, not as a runtime dependency).
- Synology specifics (UNCONFIRMED, to probe read-only): NFSv3 requires the host's allowed-client rules; share-level squash; export list via MOUNT EXPORT call.

### 2.4 FTP / FTPS (jlaffaye/ftp)
- `List` uses MLSD when the server advertises it (library default; `DialWithDisabledMLSD`, `ftp.go:242-248`, exists for buggy servers): good, but unused tuning, no fallback policy, no `UTF8` policy (`DialWithDisabledUTF8`), no EPSV policy (`DialWithDisabledEPSV`, `:220`), and passive-IP trust (`DialWithTrustPasvIP`, `:229`) matters behind NAT/pasta (the repo's own fixture uses passive range 30000-30009, `docker-compose.test-infra.yml:98`).
- No TLS: FTP credentials and data cross the LAN in clear text (`ftp_client.go:42-47`). FTPS needs explicit TLS (AUTH TLS) with `PBSZ 0` + `PROT P`, **TLS session resumption** so data-channel handshakes reuse the control-channel session (many servers, including pure-ftpd and proftpd configurations, require it; UNCONFIRMED for the Synology FTPS service, probe it) via `tls.Config{ClientSessionCache: tls.NewLRUClientSessionCache(n)}`. `ServerName` must be set; certificate verification policy for self-signed NAS certificates must be explicit per host (pin SHA-256 fingerprint rather than `InsecureSkipVerify`).
- One control connection, one data transfer at a time: parallelism needs one control connection per worker, bounded by the server's `max clients per IP` (the fixture uses 5 per IP, `FTP_MAX_CONNECTIONS`).
- `GetFileInfo`: fabricated mtime and 2 round trips (finding 6). Use `GetEntry` (MLST) when the library/server supports it, else list the parent.
- Path injection: CRLF in names/paths (I-04 in T407 concerns the Rust installer; the Go client's handling is UNCONFIRMED).

### 2.5 SFTP (not implemented)
- Use `golang.org/x/crypto/ssh` (already a dependency, `go.mod:108`) + `github.com/pkg/sftp`. Throughput is dominated by request pipelining: pkg/sftp has `MaxConcurrentRequestsPerFile`, `MaxPacket`, `UseConcurrentWrites` options (`client.go:91-112`; concurrent reads are default behaviour of `File.WriteTo`/`ReadFrom`, UNCONFIRMED for the pinned version, verify). Directory listing: `ReadDir` pages `READDIR` replies; no READDIRPLUS equivalent, attributes come with each entry (good).
- Host key policy is mandatory: `ssh.FixedHostKey` / known-hosts file per root (never `InsecureIgnoreHostKey`). Auth: password and key (Synology may disable password SFTP, UNCONFIRMED).
- Synology SFTP chroots users to their home/share view and may expose `@eaDir`; the existing scanner exclusion list must cover it (UNCONFIRMED how the app handles `.DS_Store`/`@eaDir` today; the survey histogram shows 17 `.ds_store` files in one sampled share, `survey-1.json`).

### 2.6 WebDAV (gowebdav v0.12.0 pinned, v0.13.0 current 2026-07-09; plus own client)
- App client: `http.Client{Timeout:30s}` with default transport (`webdav_client.go:43`), no connection-pool tuning (`MaxIdleConnsPerHost`), no HTTP/2 consideration, PROPFIND Depth 1 only (correct; Depth `infinity` is commonly disabled), minimal request body (`:241`); ETag/`getlastmodified` parsing not verified (UNCONFIRMED), no `Range` for resume (finding in 2.1). I-07 (status parsing, https credentials) in T407 is the Rust analogue.
- Synology WebDAV Server package exposes Basic/Digest over HTTP and HTTPS; UNCONFIRMED whether the 7 hosts run it (the owner listed SMB, NFS, FTP, FTPS, SFTP only, so **WebDAV is not testable on the NAS**; keep it on the container fixture).

## 3. Design: protocol selection / abstraction layer

### 3.1 Goals
Pick, per host, the best available read protocol; fall back automatically; keep a per-host budget so a NAS is never overloaded; make every capability testable and honest (no scanner that returns success without scanning).

### 3.2 Components (new package `submodules/filesystem/pkg/fabric`, consumed from `catalog-api/internal/services`)
1. `Capability` struct (immutable, probed): `Protocol`, `Reachable`, `AuthOK`, `ReadOnly` (observed, never assumed), `ListMode` (`mlsd|readdirplus|propfind1|readdir|list`), `ChangeToken` kind, `RangeRead`, `MaxParallel` (observed ceiling), `TLS` (none|explicit|implicit), `HostKeyPinned`, `LatencyP50`, `ThroughputProbe` (bytes/s from one bounded ranged read of a known-small file, optional), `ProbedAt`.
2. `Prober`: per protocol, **read-only**, bounded (<= 6 requests per protocol, honours the host rate limit), records results in a `host_capabilities` table (new migration under `catalog-api/migrations/`), TTL-based refresh. NAS survey methodology (names hashed, 0 writes, `nas-readonly-leg.json`) is the model.
3. `Selector`: input = host capabilities + policy; output = ordered list. Default scoring (data, not code; stored in config): 
   `sftp (pipelined, encrypted, attributes inline) ~ smb3 (encrypted if offered) > nfsv3-userspace (READDIRPLUS, LAN only, no auth) > ftps > webdav-https > ftp`. Rules: prefer encrypted over clear text unless the root is flagged `lan_trusted`; prefer the protocol with inline attributes for listing and ranged reads for content; the owner-chosen protocol in the root row always wins if reachable; **fallback on classified failures only** (network/timeout/protocol-unsupported), **never on auth failure** (avoids lockout), never on `not found`. The chosen protocol and the fallback reason are persisted and logged (no secrets).
4. `HostBudget`: per (host) semaphore + token bucket: `MaxConns` (default 4 for NAS class), `MaxReqPerSec` (default 10; survey policy was 2/s, make it configurable per root), shared across protocols for the same host so SMB+SFTP together do not exceed the cap.
5. `Pool`: implements the existing `client.ConnectionPool` interface (`GetClient/ReturnClient/CloseAll`, `client.go:107`) with health check on borrow and `MaxLifetime/IdleTimeout` (copy the semantics from `catalog-api/smb/types.go:50-90`, then delete that SMB-only pool).
6. Decorators (composable `FileSystemClient` wrappers): `ReadOnly`, `Limited` (budget), `Retrying` (reads only), `Confined` (path), `Metered` (latency/bytes counters to the existing `internal/metrics`).
7. `Scanner` rewrite: ONE generic breadth-first scanner over the abstract client (`List` returns attrs) replacing the four near-identical bodies; protocol differences live in `ScanStrategy` data only. Incremental scan compares persisted `ChangeToken`. This implements T362 for FTP/WebDAV/NFS by construction, and SFTP for free.
8. Config (explicit, validated, no silent default): `settings` schema per protocol with a single key vocabulary (`host, port, path, username, credential_ref, tls_mode, host_key_fingerprint, cert_sha256`); migrate `export_path` to `path`; reject unknown keys at API validation time.

### 3.3 Home of the code
Consolidate on `submodules/filesystem` (the repo already consumes it through `replace digital.vasic.filesystem => ../submodules/filesystem`, `go.mod:15`), delete the `catalog-api/filesystem` duplicate clients and `catalog-api/services/webdav_client.go` after the submodule versions pass the same tests. This is a removal of seemingly dead/duplicate code: per 11.4.124/11.4.122 it needs git-history evidence and an operator keep-or-remove decision first; do not delete in the same change that adds features. Submodule work needs the 11.4.26 workflow (commit in submodule, bump pointer).

### 3.4 Go library recommendations and existence-verdict register (11.4.270)
Evidence = `gh api` calls made 2026-10-07 (repo metadata, release/tag, LICENSE file size/name). "Fit" is not asserted (existence is the floor, A-012). Licence compatibility with Apache-2.0 is stated for the licence identifier GitHub reports; confirm with the project's licence scanner before adoption.

| Dependency | Use | Version evidence | Licence (GitHub SPDX) | Apache-2.0-compatible? | Verdict |
|---|---|---|---|---|---|
| `github.com/hirochachacha/go-smb2` | SMB (keep) | repo active (pushed 2026-10-05), latest tag v1.1.0 (== `go.mod:91`) | BSD-2-Clause | yes (permissive) | VERIFIED. Newer README features (QUIC, Kerberos, encryption, DFS) vs v1.1.0: UNCONFIRMED |
| `github.com/jlaffaye/ftp` | FTP/FTPS (keep, upgrade) | v0.2.4 released 2026-08-22; app pins v0.2.0 (`go.mod:92`); `DialWithTLS`, `DialWithExplicitTLS`(name not re-read; `DialWithTLS` seen `ftp.go:295`), `DialWithDisabledMLSD` seen | ISC | yes (permissive) | VERIFIED. Explicit-TLS option name UNCONFIRMED (read source before coding) |
| `github.com/pkg/sftp` | SFTP (new) | v1.13.11, 2026-07-12; `MaxConcurrentRequestsPerFile`, `MaxPacket`, `UseConcurrentWrites` seen `client.go:40-112` | BSD-2-Clause | yes | VERIFIED |
| `golang.org/x/crypto/ssh` | SSH transport for SFTP | already `go.mod:108` v0.52.0 | BSD-3-Clause (UNCONFIRMED here, standard for x/crypto) | yes | VERIFIED (present in go.mod) |
| `github.com/studio-b12/gowebdav` | WebDAV (keep, upgrade) | v0.13.0, 2026-07-09; app pins v0.12.0 (`go.mod:104`); has `ReadDir`, `Stat`, `ReadStreamRange` (`client.go:117,184,355`) | BSD-3-Clause | yes | VERIFIED |
| `github.com/vmware/go-nfs-client` | pure-Go NFSv3 client (candidate) | last push 2023-11-21, **archived**, no releases, `nfs` package | GitHub says NOASSERTION; repo ships `LICENSE_BSD-2.txt` and `NOTICE.txt` | probably (BSD-2) but the licence is not machine-declared | **AMBIGUOUS** (exists, archived, licence ambiguous). Needs a licence review and a fork/vendoring decision; **adoption refused until resolved (11.4.270(C))** |
| `github.com/Cyberax/go-nfs-client` | maintained-ish fork of the above | v1.0.0 2021-10-09, last push 2021-11-03 | Apache-2.0 | yes | VERIFIED existence; stale (5 years); fitness UNCONFIRMED |
| `github.com/willscott/go-nfs` | NFSv3 **server** (for test fixtures only, not a client) | v0.0.4, active 2026-09-26 | Apache-2.0 | yes | VERIFIED. Relevant to T134: a user-space NFS server container for CI; NOT a runtime dependency |
| `github.com/smallfz/libnfs-go` | NFS (v4?) user-space implementation | last push 2026-03-16, 52 stars | MIT | yes | UNVERIFIED for client capability (not read); do not adopt without reading the code |
| libnfs (C) `nfs-ls`, `nfs-cat` | oracle in tests (already in IMG-INFRA-CLIENT per `nfs-codepath.md`) | not re-verified this session | LGPL (UNCONFIRMED) | **do not link**; CLI use as external test oracle only | UNVERIFIED licence |

No library verified for a **pure-Go NFSv4 client**: record as a gap; NFSv4 = kernel mount only (privileged) or out of scope.

### 3.5 Per-protocol efficiency design notes (what "maximal use" means concretely)
- SMB: pool of N sessions per host (N from `HostBudget`), directory work-queue across sessions, sequential 1 MiB reads with read-ahead for content, `Stat` avoided (ReadDir attrs), signing/encryption policy explicit per host, timeouts on the dialer (`net.DialTimeout` exists, `smb_client.go:42`; the negotiate/session phases have none).
- SFTP: one SSH connection, many channels (sftp sessions) up to `MaxSessions` observed (OpenSSH default 10; Synology value UNCONFIRMED, probe), pipelined ranged reads, host key pinned.
- NFS: user-space v3 READDIRPLUS listing, parallel by directory, no content reads for cataloging unless needed (sample ranged reads).
- FTP/FTPS: MLSD listing, one control connection per worker within the server cap, TLS session cache, no per-file `SIZE`/`MDTM` when MLSD already supplied them.
- WebDAV: PROPFIND Depth 1, keep-alive transport with raised `MaxIdleConnsPerHost`, ETag as the change token.

## 4. Task breakdown (TDD; every task starts with a failing test, run x3, evidence via the repo's wrapper)

Naming: `PA-nn`. Paths relative to repo root. Existing project mechanics apply (build inside containers per 11.4.173, remote build host, `tools/evidence` wrapper). Test lanes: U unit, I integration against the WP-13 fixtures (`docker-compose.test-infra.yml`: pure-ftpd, Samba, WebDAV; NFS via T134), N real-NAS read-only leg (extend `scripts/test-infra/nas_readonly_leg.sh`; 0 writes, <= 2 req/s, names hashed, credentials from env only), B benchmark, C chaos/fault injection, F fuzz.

| Id | Task | Files (new unless noted) | Tests | Depends |
|---|---|---|---|---|
| PA-01 | Fix settings-key contract: single vocabulary, `export_path`->`path`, forward `path` for ftp/webdav; strict unknown-key rejection | edit `catalog-api/internal/services/universal_scanner.go` (storageRootToSettings), `catalog-api/filesystem/factory.go`; new `catalog-api/filesystem/settings_contract_test.go` | U table test round-tripping scanner settings through the factory (RED today for nfs/ftp path) | none |
| PA-02 | `ReadOnly` decorator + mutating-method fuzz | `submodules/filesystem/pkg/decorators/readonly.go`, `_test.go` | U (each mutator returns ErrReadOnly, inner never called), F | none |
| PA-03 | Generic breadth-first `Scanner` + `ChangeToken` | `catalog-api/internal/services/generic_scanner.go`, `_test.go`; edit `universal_scanner.go:1044-1111` to delegate | U with in-memory fake client **(unit only)**; I against FTP and WebDAV fixtures expecting 40 records (T361 corpus); N SMB | PA-01 |
| PA-04 | FTP client hardening: pin v0.2.4, MLSD/MLST attrs (no fabricated mtime), per-worker connections, mutex, TLS options (explicit TLS, session cache, fingerprint pin) | `submodules/filesystem/pkg/ftp/ftp.go` (edit), `ftp_tls.go`, tests | I vs pure-ftpd (MLSD present) and a fixture with MLSD disabled; plain-text negative test; N FTP + FTPS read-only | PA-02 |
| PA-05 | SFTP client (new) | `submodules/filesystem/pkg/sftp/sftp.go`, `sftp_test.go`; add to factory + `go.mod` | I vs an OpenSSH container (new fixture, `docker-compose.test-infra.yml` + digest pin per the pin tooling); F path confinement; N SFTP read-only | PA-02 |
| PA-06 | User-space NFSv3 client (pending licence/adoption decision on go-nfs-client; else vendored minimal RPC/XDR client) | `submodules/filesystem/pkg/nfs3/*.go` | I vs `willscott/go-nfs` server container (fixture only) and the T134 fixture; compare listing to libnfs `nfs-ls` oracle; N NFSv3 read-only | licence decision (owner), PA-02 |
| PA-07 | Pool, HostBudget, decorators `Limited/Retrying/Confined/Metered` | `submodules/filesystem/pkg/fabric/{pool,budget,decorators}.go`, tests | U (race detector), C (drop connection mid-list, server resets, timeout; assert retry class, no retry on auth failure), B (listing throughput vs worker count) | PA-02 |
| PA-08 | SMB: dialer policy (signing/encryption/dialect), multi-session scan, keepalive/reconnect, `filepath`->`path` fix | edit `catalog-api/filesystem/smb_client.go` or submodule copy; tests | I vs Samba fixture (signing on/off); N 7 NAS hosts read-only (extend `nas_readonly_leg.sh`); C session expiry | PA-07 |
| PA-09 | WebDAV: single client, keep-alive transport, ETag token, ranged read; upgrade gowebdav | `submodules/filesystem/pkg/webdav/webdav.go` (edit) | I vs WebDAV fixture; remove `catalog-api/services/webdav_client.go` only after 11.4.124/11.4.122 decision | PA-07 |
| PA-10 | Prober + capability table + Selector + fallback rules | `submodules/filesystem/pkg/fabric/{prober,selector}.go`, `catalog-api/migrations/NNN_host_capabilities.sql`, handler `catalog-api/internal/handlers/host_capabilities.go` | U (selector truth table incl. no-fallback-on-auth), I (all fixtures), N (7 hosts: record chosen protocol per host, hashed) | PA-04,05,06,08,09 |
| PA-11 | Secrets: credential_ref for all protocols, no password in `GetConfig`/logs | edit client `GetConfig()` methods, `catalog-api/internal/services/smb_identity_resolver.go` generalised | U (log capture has no secret), sink-side check on API responses | PA-01 |
| PA-12 | Benchmarks and report | `catalog-api/tests/benchmark/protocol_bench_test.go`, `scripts/test-infra/protocol_bench.sh` | B per protocol: files/s, bytes/s, p50/p95 on fixtures (container) and NAS (read-only, throttled); N>=3 identical canonical hashes of the *listing* (not timings) | PA-03..09 |
| PA-13 | Chaos/fault injection | extend `catalog-api/tests/integration/chaos_test.go` (T484), toxiproxy-style container in test-infra | C latency, reset, half-open, slow-loris; scan must end `failed` (never `completed` with 0 files) | PA-07 |
| PA-14 | Spec/task updates: add SFTP/FTPS to spec FRs and tasks (T361/T362 scope), record ODG-20 decision | `specs/001-.../tasks.md`, `spec.md`, `decisions/` (owner-owned, via the speckit workflow) | doc consistency gate | none |
| PA-15 | Independent review (11.4.142) and reviewer-authored mutations per task | `specs/001-.../reviews/` | per project review rules (Opus xhigh; Sonnet fallback only with a recorded unavailability fact) | each PA |

Anti-bluff acceptance for every scanner task: the RED must show `completed` with 0 files (existing behaviour), GREEN must show the seeded 40 records, AND a negative control (empty export / unreachable host / wrong credentials) must end `failed` or `completed` with an explicit 0 and a reason, never silent.

### 4.1 Parallelisation: disjoint file scopes
| Worker | Scope (exclusive write set) | Tasks |
|---|---|---|
| W-A | `submodules/filesystem/pkg/decorators/**` | PA-02 |
| W-B | `submodules/filesystem/pkg/ftp/**` | PA-04 (after PA-02 interface settles; the decorator interface is the only shared contract: freeze it first) |
| W-C | `submodules/filesystem/pkg/sftp/**` + SFTP compose fixture file `docker-compose.test-infra.sftp.yml` | PA-05 |
| W-D | `submodules/filesystem/pkg/nfs3/**` | PA-06 (blocked on owner licence decision) |
| W-E | `submodules/filesystem/pkg/fabric/**` (pool, budget, selector, prober) | PA-07, then PA-10 |
| W-F | `catalog-api/internal/services/generic_scanner*.go`, `universal_scanner.go`, `catalog-api/filesystem/factory.go`, settings contract test | PA-01, PA-03 |
| W-G | `catalog-api/filesystem/smb_client.go`, `submodules/filesystem/pkg/smb/**` | PA-08 |
| W-H | `submodules/filesystem/pkg/webdav/**`, `catalog-api/services/webdav_client*.go` | PA-09 |
| W-I | `catalog-api/tests/benchmark/**`, `scripts/test-infra/protocol_bench.sh`, `catalog-api/tests/integration/chaos_test.go` | PA-12, PA-13 |
| W-J | `scripts/test-infra/nas_readonly_leg.sh` extensions + evidence dirs (NAS legs only, read-only, serial: single owner of the NAS, 11.4.119) | all N lanes |
| W-K | `specs/001-.../{tasks,spec}.md` | PA-14 |
Conflict notes: `go.mod`/`go.sum` (catalog-api and submodule) are shared write targets: serialise all dependency bumps through ONE integrator after workers finish (W-B, W-C, W-D, W-H each propose a diff). `universal_scanner.go` is touched only by W-F. Submodule commits need the 11.4.26 pointer-bump step by the conductor, not by workers.

## 5. Open questions for the owner (no default is invented)
1. ODG-20 is recorded as relayed, not verbatim; confirm `implement` including SFTP and FTPS (not in any task today).
2. NFS client choice: adopt/fork an archived library with an ambiguous licence, write a minimal NFSv3 client, or stay on kernel mount (ODG-08 related).
3. Clear-text FTP: forbid, or allow only for flagged trusted-LAN roots?
4. Certificate and SSH host-key trust model for the NAS hosts (pin per host).
5. Per-host budget defaults for a production NAS (the survey used 2 req/s).
6. Which of the three duplicated client trees is canonical, and may the other two be removed (11.4.122/11.4.124)?

## 6. What was NOT done / UNCONFIRMED
- No runtime test, no build, no NAS access, no `go vet`; every behavioural statement about library internals is from READMEs/headers via `gh api` or marked UNCONFIRMED.
- Duplicate-code comparison between the three client trees was by file size and structure, not a diff.
- The independent review required by 11.4.142 has not run; this report is analysis, not a verified verdict.
