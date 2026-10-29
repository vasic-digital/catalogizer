# Changelog

All notable changes to the Catalogizer project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

## [2.4.0] - 2026-04-14

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
