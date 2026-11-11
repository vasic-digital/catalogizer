# Changelog

All notable changes to the Catalogizer project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security (TRUSTED_PROXIES)
- **`TRUSTED_PROXIES=auto` is deprecated and refused in release mode** (CAT-TRUSTEDPROXIES-AUTO-1): it trusts all RFC-1918/ULA ranges, so a client inside one of them could send `X-Forwarded-For: 9.9.9.9` and have the proxy append its own address, making `ClientIP()` 9.9.9.9 (login rate-limit bypass; gin skips trusted entries from the right and has no "trust only the direct hop" mode). With `GIN_MODE=release`, `auto` now fails unless `TRUSTED_PROXIES_ALLOW_AUTO=true`; a startup warning is logged whenever `auto` is used. A rejected or invalid `TRUSTED_PROXIES` now **fails closed** (no proxy trusted): previously a refused `auto` left gin's trust-everything default in place, and an invalid CIDR list left whatever gin had applied before the error (possibly a partial list). **Behaviour change:** deployments relying on `auto` in release mode must list explicit proxy CIDRs (or set the acknowledgement). `catalog-api/.env.example` no longer enables `auto`.
- **Production compose files now set `TRUSTED_PROXIES`** (CAT-TRUSTEDPROXIES-PROD-1): `docker-compose.yml` (nginx, `production` profile) and `deployment/docker-compose.yml` (`web`) pin the proxy to a fixed address on a fixed-subnet network (`172.28.0.10` / `172.29.0.10`) and the API trusts exactly that `/32`; previously every client behind nginx shared one login rate-limit bucket. Dynamic containers are allocated only from the upper half of each subnet (`ip_range` `172.28.0.128/25` / `172.29.0.128/25`), so the pinned address cannot be taken by another container. Overrides: `CATALOGIZER_SUBNET`, `CATALOGIZER_IP_RANGE`, `NGINX_IP` (root file) and `DOCKER_SUBNET`, `DOCKER_IP_RANGE`, `WEB_IP` (deployment file); when you change the address also set `TRUSTED_PROXIES`. A subnet that overlaps a host network fails `up` with a "Pool overlaps"/address-in-use error: pick another subnet via those variables.
- **Upgrade / migration (existing installs):** compose reuses an existing network of the same name and ignores new ipam settings, so the pinned address would fail or be ignored. Before the first `up` of this version run `compose down`: for the root file the network is project-scoped (`<project>_catalogizer-network`) and `compose down` removes it (do not remove a bare `catalogizer-network`, it belongs to another project). The `deployment` file's default network name changed from the fixed `catalogizer-network` to `catalogizer-frontdoor-net` (override `DOCKER_NETWORK`) so it does not collide with an older network; remove the old `catalogizer-network` only if no other stack uses it.
- `deployment/docker-compose.yml` `web` now sets `API_UPSTREAM: catalogizer-server:8080`. The web image's nginx template (`catalog-web/nginx.conf.template`, rendered by the official nginx entrypoint, `NGINX_ENVSUBST_FILTER=^API_`) proxies to `API_UPSTREAM`, whose default `host.containers.internal:8080` goes via the host and would hide the pinned web address from the API. Its `/ws` location now forwards `X-Real-IP`/`X-Forwarded-For` too, and `config/nginx.conf` `/health` does the same (a test scans every `proxy_pass` location).
- A startup warning is logged when `TRUSTED_PROXIES` contains `0.0.0.0/0` or `::/0`.
- Verification: `ip_range` reaching network creation was confirmed with a throwaway podman-compose project (pinned `.10` outside the range, a dynamic container got `.129`). Not verified live: the real compose stacks were only rendered (`podman-compose config`), not started; that the published `catalogizer/web` image is built from `catalog-web/Dockerfile` is assumed.
- Tests pin every `auto` range with both edges, the forgery hazard, release-mode refusal, fail-closed behaviour, and the compose-to-policy link (CAT-TRUSTEDPROXIES-FU-1).

### Governance
- **Spec Kit constitution 2.5.0** (2026-10-21): single-source flat layout (owner mandate: no nested submodule chains); canon pin moved `4bcb512` → `a3d80ce` (canon branch `feat/hub-engines-single-source` merged and pushed to canon `main`) and then to its follow-up `5f8503b` (pushed to canon `main`); `llmctl` `f42bad6` (cleaned, `CM-LLMCTL-SUBMODULE-NONRECURSIVE` PASS), root `helix_qa` `ad8b12b`; `claude_toolkit` `1f6dad3` (cleaned: nests only the third-party Go toolchain, ships `helix-deps.yaml`; at `a3d80ce` it was `223c702` with both gates RED); `CM-HUB-ENGINE-NO-OWNORG-NESTING`, `CM-HELIX-DEPS-MANIFEST-PRESENT` and `scripts/verify_helix_deps_manifests.sh` PASS (pass 54 / fail 0); index scope re-rendered from DATA (`derive_scope.sh`, `scope_to_lumen_json.py`, `scope_render.py`, `scope_drift_check.sh --regen`), drift check passes; root `helix-deps.yaml` refs re-synced to the staged gitlinks; `scripts/safe_submodule_init.sh` warns about leftover directories of dropped gitlinks; safe init through `scripts/safe_submodule_init.sh`; 27 flat tool submodules; Known Conflicts items 21(d), 25 to 27 and 29 updated (27 and 29 RESOLVED), items 30 to 32 added. Earlier in the day, **2.4.0**: canon pin moved `94b8836` → `4bcb512`; new section "Constitution Hub Engines" binding the vendored `llmctl`, `claude_toolkit` and `open_design` (rules HE-1 to HE-8, appendix Part 3); Known Conflicts items 21(c)/(d) corrected and items 25 to 29 added (29: code-index scope data) (canon's `CM-LLMCTL-SUBMODULE-NONRECURSIVE` gate fails at the new pin; `claude_toolkit` and `open_design` are not in canon's manifest). See `docs/GOVERNANCE.md`.
- Documentation: safe, non-recursive submodule initialisation through `constitution/` in `README.md`, `GETTING_STARTED.md`, `docs/DEVELOPER_GUIDE.md` and `docs/SUBMODULE_DEPENDENCIES.md`; new `docs/GOVERNANCE.md`.

### Changed (FTP CopyFile, filesystem package)
- **FTP `CopyFile` no longer deletes or overwrites the destination in place** (`catalog-api/filesystem/ftp_client.go`). The data is now stored under a temp name beside the destination, `.<base>.ftpcopy-<16 hex>.part` (base cut to 200 bytes on a rune boundary, so the whole name stays under NAME_MAX), and renamed onto the destination (RNFR/RNTO) only after the source transfer is confirmed. **Behaviour change:** a server that refuses rename onto an existing file (some proftpd/pure-ftpd configurations) now makes every copy onto an existing destination fail, with the destination left unchanged and the temp removed; previously the destination was overwritten in place (plain STOR), which such servers allowed. Servers with MoveFile-style rename semantics (possibly Windows/IIS FTP, UNCONFIRMED) may be affected as well. A failed or cancelled copy never touches a pre-existing destination.
- **Orphan reporting contract:** if the temp cannot be removed after a failure or cancellation, the returned error names it (`temporary file <path> may remain on the server`; typed `*orphanError`, reachable with `errors.As`, also on the cancel path) and a log line is written. Whether a failed cleanup DELE means "the temp never existed" is decided from the 550 reply text: only a text naming a missing file/directory (no such file, no such directory, not found, does not exist) counts, and any mention of permission, privilege, denial or read-only vetoes it (`550 No such privilege` and `550 File not found or permission denied` are reported as possible orphans). The heuristic is imperfect (servers word it differently, localised servers differ); an unrecognised or mixed wording is reported as a possible orphan rather than ignored, so a miss errs towards reporting.
- **Known limits:** once the transfer is confirmed the cancel watcher is disarmed, so a cancel during the final rename does not close the connection at once (a responsive server still commits). A rename that then gets no answer is cut off after `renameGraceAfterCancel` (5s after the cancel): the session is closed and the error says the rename "may or may not have been applied". A connection that dies during the rename is reported the same way, not as "left unchanged". The single-session spool fallback has no such cut-off. Evidence is unit-class against a fixture server; no live-NAS proof.
- **Scanner:** the catalogue scanner (`internal/services/generic_scanner.go`, FTP storage only) skips *files* named exactly `.<anything>.ftpcopy-<16 lowercase hex>.part` (in-flight or stale `CopyFile` temps; the pattern also matches names containing a newline); a user's own look-alike names such as `.x.part` are still catalogued. `FTPClient.ListDirectory` itself reports every entry, so callers that list, move and remove a directory (`moveDirectory` in `internal/services/protocol_handlers.go`) see a stale temp instead of failing the final remove with a confusing "not empty". Stale temps left by a crashed copy are not removed automatically.
- The single-session fallback spool directory is configurable (`FTPConfig.SpoolDir`, default the OS temp dir); free space is computed from `f_frsize` on linux and `f_bsize` on darwin/freebsd (build-tag split, so darwin builds); other platforms report free space as unknown. A negative `f_bavail` counts as 0.
- Evidence class: unit tests against a hand-written FTP responder; no live-NAS run yet (owed).

### Fixed
- **Challenge framework**: `RunSequence` now performs Kahn topological sort on dependencies, fixing `RunByCategory` alphabetical-order failures
- **Challenge streaming**: `RunAll` and `RunByCategory` handlers support NDJSON streaming mode (`?stream=true`) to eliminate tens-of-MB buffer blocking
- **Challenge timeout**: per-challenge timeout reduced from 72h to 30min as a hard ceiling distinct from stale-progress detection
- **database-connectivity hang**: `DatabaseConnectivityChallenge` now implements progress reporting, enabling 5-min stale detection
- **Android TV lint**: 49 pre-existing lint errors fixed via `@SuppressLint` for legitimate `androidx.tvprovider` and Media3 API usage
- **Android TV package**: challenge config corrected from `com.vasic.catalogizer.tv` to `com.catalogizer.androidtv`
- **Security**: axios SSRF vulnerability patched in `Catalogizer-API-Client-TS`
- **PlaybackTracker**: added `ensureActive()` and re-throw `CancellationException` to prevent coroutine leaks
- **PlaybackTrackerTest**: stabilized by removing shared `TestScope`, eliminating `UncompletedCoroutinesError`

### Changed
- Version bump: 2.3.0 → 2.4.0 (build 25) across all components

## [2.4.0] - pre-release (candidate; tag `v2.4.0` is cut only after live validation; date set at tag time)

Release class: **pre-release** (provisional-eligible, constitution 11.4.290(G)). Candidate state, computed from the repository at `17ec3c1f` (about 159 first-parent commits since 2026-10-28; no git tag exists yet).
Version files: `versions.json` (global 2.4.0), `docs/api/openapi.yaml` (info.version 2.4.0). Components `catalog-web`, `catalogizer-desktop`, `installer-wizard` (package.json, tauri.conf.json, Cargo.toml) and the Android / Android TV `versionName` were already 2.4.0. Versioning is SemVer (operator decision 2026-11-03); the decimal-carry rule of 11.4.235(E.5) does not apply.

### VERIFIED (live evidence on QA stack 3, machine evidence under `docs/qa/`)
- Stack 3 rebuilt from `main` `d9238b47`: 54 PASS / 0 FAIL / 1 SKIP, 3 identical rounds; Yandex WebDAV and Synology SMB/FTP read-only suites 3/3 (`docs/qa/stack3-rebuild-live-20261105/`). Promoted to VERIFIED: conversion D1, D2, D10; PDF pages D3; document full-text search D4; D5; subtitle D6 and DELETE route; admin role change D9; D7, D8, D11a, D11b; storage-client live suites (SMB/FTP/WebDAV).
- NEW-1 cover thumbnails (batch endpoint): generated 2 then 0, `cover_art` rows on PostgreSQL (`docs/qa/new1-cover-thumbnails/green_main5c48.txt`).
- On image `main-5c480072` (`docs/qa/stack3-live-20261105/`): SECQ-1 WebDAV storage-root SSRF guard, SECQ-5 (GET 410, 401/403), SECQ-4/4b smart-collection cap (51 creates at 16 concurrency gave exactly 50 x 201 + 1 x 409, 3 rounds), subtitle DELETE non-owner 404, subtitle contract.
- Live browser pass (`docs/qa/stack3-ui-20261105/`): 11 app pages render real data, website 47 pages OK, theme toggle.
- Live UI rerun on `main-1db4760f` (`docs/qa/ui-live-rerun-20261105/`, N=3 identical): D5/D6 axe, /admin at 360px, D8 search, /conversion, /identities, /entity/:id PASS.

### PROVISIONAL (landed on `main`, review GO-with-follow-ups; counts as done for planning, NOT verified)
Pending job for every row: the verifier named in the row runs on the rebuilt stack or device, N=3, machine evidence under `docs/qa/`; due before the `v2.4.0` tag. Command is the one recorded in `docs/CONTINUATION.md` section 3 unless stated.
- UI contrast D7 (class-wide dark-gray surfaces, rounds 1-2) and D7 remaining-contrast: target rebuilt web image on stack 3; command `node docs/qa/ui-d5d6-20261105/axe_live.cjs` (D7 check: `d7_axe.cjs`), expecting 0 violations in light and dark on /ai, /media, /analytics, /admin.
- UI-D1/D2/D3 (conversion page live shape, identities route, entity 404 handling), UI-D5/D6 accessible names, UI-D7/D8: PASS on the live rerun above for D1, D2, D3, D5, D6, D8; D7 pending as above.
- Tauri desktop: QA boot tooling and login fixes, SSRF hardening (TAURI-FU-5), backend-enforced confirmation on server URL change: target native Tauri boot of the desktop app (container build exists; native boot not run); command per `docs/qa/` Tauri boot notes.
- Android and Android TV QA boot (JDK 17 build, emulators; evidence `docs/qa/android-boot-20261105/`): target emulator/device live run of the full banks.
- S3 storage protocol (aws-sdk-go-v2): unit-class evidence plus live N=3 on a real Garage v1.0.1 server only; target other S3 implementations.
- HelixQA stack 3 API smoke wrapper: API leg only; the UI Vision leg is not implemented (HELIXQA-FU-1).
- Authz and security series (SSRF resolvers, authz round 2, route permissions, CSRF LAN toggle, pprof/metrics gating SECQ-3/6, WebDAV hardening WD-F1, pathguard confinement series): unit and package-live evidence; SECQ-3 pprof is UNCONFIRMED live (needs a restart with `HELIX_PPROF_ENABLED=true`); NEW-1 SSRF-F2..F5 through the cover-art product path not yet live.
- Remaining landed series with review GO-with-follow-ups and no separate live rerun recorded: login/validator and password hashing, CAT-001 honest UI/API (R1-R6), CAT-002 isolated PDF text extraction, CAT-021 default credentials removal, smart collections, registration approval and converter tools, converter hardening, FTP `CopyFile` temp-name semantics (no live-NAS run), SQLite busy handler, Redis rate limiter, nil-slice `[]` serialisation, pool-deadlock class fix, live scaling/stress/chaos suite, docs link repair (111 links), runner scripts honest failure reporting.

### REOPENED
- None recorded since the last review. Any live-rerun FAIL reopens its item and is listed here at tag time (UI-D7 is the one live check that failed on the last rerun and has since been re-fixed, see PROVISIONAL).

### OVERDUE
- None: no verification job has a due date that has passed (no due dates were set before this candidate). Every PROVISIONAL row becomes overdue if it is still unverified when the tag is attempted; such a release would carry the label `verification OVERDUE`.

### Open follow-ups (not fixed; tracked in `docs/FOLLOW_UPS_NEXT_PHASE.md`, which is the authoritative list)
- Live gaps: LV3-1 non-privileged cross-user conversion isolation (SKIP); LV3-2 storage idle-deadline and cut-recovery against real Synology (loopback/unit only); LV3-3 D5 bursty-audio fixture; LV3-4 stale fixture rows; LV3-5 PostgreSQL multi-process last-admin race; STK3L2-FU-1 no delete API for storage roots; STK3L2-FU-2 `/smb/discover` returns 500 on timeout.
- UI: UI-FAV-1 favourites link the wrong entity (pre-existing, important); UI-D4 `/ws` idle closes (unreproduced) and login double-connect; UI-D1-FU-1/2 conversion cancel verb and request shape; UI-D2-FU-1 identity CRUD routes missing; UI-D3-FU-1/4 retries on 404.
- Security residuals: path-confinement TOCTOU (F4, `os.Root` sinks) and hard-link policy (operator decision); secrets scanner residuals; key-rotation waiver is an operator-accepted open risk with no expiry.
- Process: container-hygiene guard parser gaps; HelixQA web Vision bank and CDP wiring (needs host capacity decision); upstream `helix_qa` credential option; operator decisions pending listed in the follow-ups file (amber podman, `thinker.local`, `catalogizer/server` image).
- Not part of this release: SonarQube analysis (blocked on image access).

### Skipped live tests
- **Synology SMB / FTP / NFS live tests are SKIPPED (`hardware_not_present`): the Synology drives were powered off by the operator on 2026-11-05 19:09 CET.** No test double was substituted; the earlier read-only Synology suites (3/3 on `d9238b47`) predate the power-off. NFS was already `topology_unsupported` (no exports).

## [2.4.0-dev.20260414] - 2026-04-14 (historical heading, never tagged; folded into 2.4.0)

### Fixed
- **Goroutine leak**: `WorkerPool.SubmitAsync()` now tracks spawned goroutines via WaitGroup (catalog-api)
- **Goroutine leak**: `Throttler` run goroutine now tracked and awaited on Stop() (catalog-api)
- **Deadlock risk**: `Debouncer.Flush()` executes pending function outside the lock to prevent reentrant deadlock (catalog-api)
- **Race condition**: WebSocket handler uses reservation pattern to prevent over-admission under concurrent connections (catalog-api)
- **Resource leak**: SMBChangeWatcher.Stop() drains all debounce timers before closing stop channel (catalog-api)
- **Context propagation**: CacheService cleanup now cancels on service shutdown instead of using detached Background context (catalog-api)
- **Memory leak**: WebSocketContext no longer causes reconnection storms from stale dependency array (catalog-web)
- **Duplicate state update**: AuthContext consolidated from two permission effects into one (catalog-web)
- **Timer leak**: UploadManager clears pending timer before scheduling new one (catalog-web)
- **Sync on logout**: SyncManager.cancelSync() now cancels in-flight manual sync via Job tracking and ensureActive() (catalogizer-android)
- **Indefinite suspension**: MediaRepository.getRecentMedia/getPopularMedia use withTimeoutOrNull(5s) on Flow.first() (catalogizer-android)
- **DataStore contention**: AuthRepository shares a single DataStore flow via shareIn() instead of 7 independent collectors (catalogizer-android)
- **Room DB lifecycle**: CatalogizerDatabase.closeDatabase() companion method for proper cleanup (catalogizer-android)
- **Version alignment**: All component package.json/build.gradle versions aligned to 2.3.0 (all components)

### Added
- **Episode navigation**: Android TV VLCPlayerActivity.playNextEpisode/playPreviousEpisode now fetch real episodes via /api/v1/entities/:id/children API (catalogizer-androidtv)
- **React lazy loading**: Sub-component lazy imports for MediaPlayer, MediaDetailModal, UploadManager, Dashboard charts, 9 AI components (catalog-web)
- **Custom Semgrep rules**: `.semgrep.yml` with 17 security rules for Go SQL injection, XSS, command injection, secrets detection
- **Hadolint**: Dockerfile linting service added to docker-compose.security.yml
- **govulncheck integration**: Added to scripts/security-scan.sh for Go vulnerability scanning
- **Android SDK 35**: Dockerfile.builder now installs both SDK 34 and 35 platforms
- **AGENTS.md**: Created for 6 components (catalog-web, android, androidtv, desktop, api-client, wizard)
- **SQL Migration Reference**: Complete 1323-line docs/database/SQL_MIGRATION_REFERENCE.md covering all 15 migrations
- **Performance Tuning Guide**: 905-line docs/guides/PERFORMANCE_TUNING.md with concrete values from source code
- **Video courses**: 6 new course scripts (35 modules, ~24 hours content) for all core components
- **Architecture diagrams**: 4 new Mermaid diagrams (security-scanning-pipeline, lazy-init-flow, semaphore-control, test-coverage-matrix)
- **HelixQA banks**: Security-comprehensive (30 entries), DDoS-ratelimit-comprehensive (20 entries), benchmarking-baselines (15 entries), fixes-validation (29 entries)
- **Go benchmarks**: Benchmark tests for WorkerPool, Throttler, JWT, TitleParser, RateLimiter
- **Rust tests**: Tests for all installer-wizard and catalogizer-desktop Tauri backend modules
- **Test coverage**: Fixed 6 skipped catalog-web tests, completed API client tests, expanded TS submodule coverage

### Removed
- **Dead middleware**: Deleted advanced_rate_limiter.go, enhanced_rate_limiter.go (never used)
- **Deprecated stub**: Removed NewStubHandler() function
- **Dead hooks**: Deleted usePlayerState, useLazyImage, usePlaylistReorder, useVirtualScroll (unused)
- **Mock APIs**: Deleted mockCollectionsApi.ts, removed conversionApi.ts mock fallbacks
- **Unused config**: Removed CertFile/KeyFile from ServerConfig (self-signed certs generated at runtime)

### Changed
- **SonarQube**: Moved to profile-based activation (no longer auto-starts consuming resources)
- **.env.example**: AI/Vision env vars clearly labeled as HelixQA-only, SNYK_TOKEN added
- **Go submodules**: LLM modules bumped from Go 1.24.x to 1.25
- **useDebounce**: Wired into MediaBrowser search replacing inline debounce utility

## [2.3.0] - 2026-04-04

### Added

#### Media Entity System (Phases 1-10)
- Complete media entity aggregation pipeline: scanner post-scan hook triggers title parsing, MediaItem creation, hierarchy building, and duplicate detection
- 11 media types seeded in `media_types` table: movie, tv_show, tv_season, tv_episode, music_artist, music_album, song, game, software, book, comic
- Entity hierarchy with parent_id self-reference: TV Show -> seasons -> episodes, Music Artist -> albums -> songs
- Entity API (`/api/v1/entities`) with 17 endpoints: list, get, children, files, metadata, duplicates, stream, download, install-info, browse by type, entity types, entity stats, duplicate groups, user metadata, metadata refresh
- Title parser (`internal/services/title_parser.go`) with regex-based extraction for movie, TV, music, game, and software naming conventions
- AggregationService (`internal/services/aggregation_service.go`) for post-scan entity creation with automatic hierarchy building
- MediaItemRepository, MediaFileRepository, ExternalMetadataRepository, UserMetadataRepository, DirectoryAnalysisRepository for entity data access
- Entity browser frontend components in `catalog-web`: browse page, entity detail page, type-based navigation
- User metadata system: user ratings, watch status, favorites, personal notes, and tags per entity

#### Asset Management System
- Asset management submodule (`digital.vasic.assets`) integrated for cover art, thumbnails, and media artwork
- Lazy asset resolution pipeline: request -> background resolution -> WebSocket notification -> client refresh
- Three asset resolvers chained: CachedFileResolver, ExternalMetadataResolver, LocalScanResolver
- Default placeholder provider via `defaults.NewEmbeddedProvider()` for assets pending resolution
- Asset serving endpoint (`GET /api/v1/assets/:id`) with `X-Asset-Status` header (ready/pending)
- Asset event bridge: asset resolution events broadcast to WebSocket clients for real-time UI updates

#### Challenge-Based Testing Framework (CH-001 to CH-020)
- `digital.vasic.challenges` Go submodule for structured test scenario definition and execution
- 20 challenges covering infrastructure (CH-001 to CH-003), browsing (CH-004 to CH-005), assets (CH-006 to CH-007), auth (CH-008), scanning/storage (CH-009 to CH-015), and entity system (CH-016 to CH-020)
- Challenge REST API (`/api/v1/challenges`) with list, get, run single, run all, run by category, and get results endpoints
- Progress-based liveness detection with 5-minute stale threshold
- 117 assertions across all 20 challenges, all passing

#### Container Discovery Submodule
- `digital.vasic.containers` Go submodule for TCP-based service discovery
- Dynamic port binding: API server finds available port at startup, writes to `.service-port` file
- Frontend dev server reads `.service-port` for automatic API proxy configuration

#### Submodule Architecture
- 10 actively integrated Go submodules via `replace` directives in `go.mod`: Challenges, Assets, Containers, Concurrency, Config, Filesystem, Auth, Cache, Entities, EventBus
- 9 TypeScript/React submodules linked via `file:../` in `catalog-web/package.json`
- ADR-002 documenting submodule integration decisions and exclusion rationale for 10 unused Go submodules

#### HTTP/3 (QUIC) Support
- HTTP/3 server on UDP port 8443 via `quic-go/http3`
- HTTPS/HTTP2 server on TCP port 8443 as fallback
- Self-signed TLS certificate generation at startup for development
- Alt-Svc header advertising HTTP/3 availability to clients
- Brotli compression middleware (`andybalholm/brotli`) for all API responses

#### Advanced Statistics
- 9 statistics endpoints under `/api/v1/stats`: overall, per-storage-root, file types, size distribution, duplicates, duplicate groups, access patterns, growth trends, scan history
- Media browse endpoints (`/api/v1/media/search`, `/api/v1/media/stats`) backed by real database queries

#### Frontend Component Library
- React 18 with TypeScript, Vite, Tailwind CSS, React Query, Zustand
- Entity browser with type-based navigation, grid/list views, and entity detail pages
- Collection manager UI with create, edit, delete operations
- Dashboard analytics page with statistics visualization
- Media player components with subtitle support
- Path aliases: `@/components`, `@/hooks`, `@/lib`, `@/types`, `@/services`, `@/store`, `@/pages`, `@/assets`
- Build output split into vendor chunks: vendor (react), router, ui, charts, utils

#### Documentation (Phase 6)
- OpenAPI 3.0 specification (`docs/api/openapi.yaml`) covering all `/api/v1/*` endpoints with Bearer JWT auth, request/response schemas, and proper HTTP status codes
- ADR-001: Dual Database Dialect (SQLite + PostgreSQL abstraction)
- ADR-002: Submodule Architecture and Integration Decisions
- ADR-003: HTTP/3 (QUIC) with Brotli Compression Requirement
- ADR-004: Challenge-Based Testing Framework
- ADR-005: Container-Only Build and Runtime Policy
- ADR-006: Zero Warning / Zero Error Policy
- Comprehensive CHANGELOG entries for all post-1.0.0 work

### Changed
- Database migrations expanded to 8 versions (v7 = challenge/scan tables, v8 = media entity tables)
- Universal scanner enhanced with post-scan aggregation hook for automatic entity creation
- Config precedence clarified: env vars > `.env` > `config.json` > defaults
- Challenge runner timeout set to 72 hours with 5-minute stale threshold for stuck detection
- `config.json` `write_timeout` increased to 900 seconds to support long-running challenge RunAll operations
- Containerized build pipeline uses `--network host` for SSL reliability

### Deprecated

### Removed
- 21 unused Go/Android submodules cleaned up (Auth, Cache, Database, Discovery, Media, Middleware, Observability, RateLimiter, Security, Storage, Streaming, Watcher, and Android-specific modules)
- Deprecated `smb_roots` table references replaced by `storage_roots` multi-protocol table

### Fixed
- Database schema and scanner foreign key issues in entity tables
- Auth mock types, API key generation, and LoginResponse field mismatches in test suite
- All failing tests resolved: Go 35/35 packages pass, Frontend 102/102 files (1643 tests) pass
- All 20 challenges passing with 117/117 assertions

### Security
- JWT secret generation: cryptographically secure random secret generated at startup when not configured
- Password hashing uses bcrypt with per-user salt
- Rate limiting on auth endpoints (5 requests/minute) and general API (100 requests/minute)
- Input validation middleware with configurable rules
- Request ID middleware for audit trail correlation
- Redis-backed distributed rate limiting (optional, falls back to in-memory)

---

## [1.0.0] - 2026-02-02

### Added

#### Multi-Platform Support
- Go/Gin backend API (`catalog-api`) with Handler, Service, and Repository layers
- React/TypeScript web frontend (`catalog-web`) with React Query state management
- Tauri/Rust desktop application (`catalogizer-desktop`) with IPC commands/events
- Tauri-based installer wizard (`installer-wizard`) for guided setup
- Kotlin/Compose Android application (`catalogizer-android`) with MVVM architecture
- Kotlin/Compose Android TV application (`catalogizer-androidtv`) with lean-back UI
- TypeScript API client library (`catalogizer-api-client`)

#### Protocol Support
- SMB/CIFS network share scanning with circuit breaker and offline cache
- FTP file system support
- NFS network filesystem support
- WebDAV protocol support with sync and backup capabilities
- Local filesystem scanning

#### Media Detection and Management
- Automated media detection pipeline: detector, analyzer, and metadata providers
- TMDB, IMDB, and other metadata provider integrations
- Comprehensive media type classification (movies, TV shows, music, games, software, and 30+ additional types)
- Duplicate detection with configurable hash algorithms (MD5, SHA256, SHA1, BLAKE3, quick hash)
- Quality profile comparison and ranking (4K/UHD, 1080p, 720p, DVD, etc.)
- Media collection grouping for series, albums, and franchises
- File rename detection system
- Advanced AI features for media analysis (Phase 3.2.8)
- Performance optimization system (Phase 3.2.7)
- Advanced collection management system (Phase 3.2.5-3.2.6)

#### Media Player Features
- Subtitle track management with multi-language support
- Subtitle sync verification and offset adjustment
- Subtitle download tracking and caching
- Audio track management for multi-language audio
- Chapter support for video bookmarks
- Cover art fetching from MusicBrainz, Last.fm, Spotify, iTunes, Discogs
- Lyrics data with synced lyrics support
- Playback session tracking with resume capability
- Playlist management including smart playlists
- Translation cache for subtitles and lyrics
- Language preference management per content type

#### Real-Time Features
- WebSocket-based real-time event bus for live updates
- Real-time media change notifications
- File system monitoring and change tracking

#### Authentication and Authorization
- JWT-based authentication with session management
- Role-based access control (Admin, Moderator, User, Viewer)
- User permission management with custom permission grants
- Authentication audit logging
- Account lockout protection with failed login tracking
- Session management with device tracking

#### Media Conversion
- Format conversion job queue with priority support
- Support for video, audio, image, and document formats
- Batch conversion with progress tracking
- Quality settings per conversion job

#### Database and Storage
- SQLite database with automatic migrations on startup
- PostgreSQL support for production deployments
- 6-version migration system with tracking table
- Virtual paths for unified file access across protocols
- Scan history tracking with detailed statistics
- External API response caching

#### Administration and Deployment
- Docker and Docker Compose support for development and production
- Comprehensive configuration via environment variables, .env files, and config.json
- Installation wizard with step-by-step guided setup
- Backup and recovery documentation
- Scaling guide for production environments
- Production runbook for operations teams

#### Documentation
- Architecture documentation for all components (Go backend, React frontend, Android, Tauri IPC)
- API documentation with endpoint reference
- WebSocket events documentation
- Database schema documentation
- User guides for web, desktop, Android, and Android TV
- Deployment guide with Docker setup
- Monitoring guide with metrics
- Contributing guide with code standards
- Configuration guide
- Troubleshooting guide
- Security testing guide
- QA testing guide and execution reports
- Test strategy documentation

#### Testing
- Comprehensive test suite across all modules (~1000+ tests across phases)
- Table-driven tests for Go backend
- Benchmark tests for performance-critical paths
- Security scanning and remediation (Phase 6)
- Cross-cutting error handling tests
- WebSocket integration tests
- Auth flow tests
- Accessibility tests for frontend

### Changed
- Migrated storage model from SMB-only (`smb_roots`) to multi-protocol (`storage_roots`)
- Updated media items schema for Android TV compatibility (migration 006)
- Refactored subtitle foreign keys from `media_items` to `files` table reference
- Reorganized root directory structure: moved ~65 files into config/, scripts/, tests/, docs/ subdirectories

### Fixed
- Critical build blockers and security issues (Phase 0)
- Memory leaks and race conditions (Phase 0)
- Subtitle foreign key references pointing to wrong table (migration 015/v6)
- Disconnected features across all modules (Phase 2)

### Security
- Security hardening and stability fixes (Phase 1)
- Security scanning and remediation across all modules (Phase 6)
- JWT token management with secure session handling
- Password hashing with salt
- Input validation with Zod on frontend
- CORS and rate limiting middleware
- Freemium security setup documentation

---

## Release History

This changelog was created as part of the Phase 8 documentation completion effort. Prior development was tracked through auto-commits and phase-based commit messages. Future releases will follow this structured format.

[Unreleased]: https://github.com/catalogizer/catalogizer/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/catalogizer/catalogizer/releases/tag/v1.0.0
