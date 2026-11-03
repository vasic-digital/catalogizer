> **SUPERSEDED** — This document was a planning artifact from an earlier phase. The work described here has been completed or superseded by the [2026-03-26 Project Completion Plan](../../superpowers/plans/2026-03-26-project-completion-plan.md). Retained for historical reference.

---

# Comprehensive Remediation, Coverage & Documentation Plan

**Date:** 2026-03-07
**Scope:** Full project audit, remediation, test coverage maximization, documentation completion, security hardening, performance optimization, and content expansion
**Status:** DESIGN DOCUMENT - Pending Approval

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Audit](#2-current-state-audit)
3. [Findings Registry](#3-findings-registry)
4. [Phased Implementation Plan](#4-phased-implementation-plan)
5. [Test Coverage Strategy](#5-test-coverage-strategy)
6. [Challenge Expansion Plan](#6-challenge-expansion-plan)
7. [Documentation Completion Matrix](#7-documentation-completion-matrix)
8. [Security Remediation Plan](#8-security-remediation-plan)
9. [Performance Optimization Plan](#9-performance-optimization-plan)
10. [Monitoring & Metrics Plan](#10-monitoring--metrics-plan)
11. [Content Expansion Plan](#11-content-expansion-plan)
12. [Verification & Acceptance Criteria](#12-verification--acceptance-criteria)

---

## 1. Executive Summary

### 1.1 Project Inventory

| Component | Language | Test Files | Test Functions | Status |
|-----------|----------|-----------|----------------|--------|
| catalog-api | Go 1.24 | 242 | 3,738 + 74 bench | Operational |
| catalog-web | React 18/TS | 102 + 25 E2E | ~3,085 | Operational |
| catalogizer-desktop | Tauri/Rust+React | 15 | ~150 | Operational |
| installer-wizard | Tauri/Rust+React | 19 | 178 | Operational |
| catalogizer-android | Kotlin/Compose | 45 | ~400 | Operational |
| catalogizer-androidtv | Kotlin/Compose | 27 | ~250 | Operational |
| catalogizer-api-client | TypeScript | 8 | ~80 | Operational |
| **32 Submodules** | Go/TS/Kotlin | Various | Various | Operational |

**Total registered challenges:** 239 (50 original CH + 174 userflow UF + 15 module MOD)

### 1.2 Audit Verdict

| Category | Verdict | Critical Items |
|----------|---------|---------------|
| Broken/Disabled Tests | CLEAN | 0 unconditional skips, all 143 skips are conditional |
| Dead Code | 7 HIGH findings | 2 unused handlers, 2 unused services, 1 unused repo, 1 orphaned package, stale artifacts |
| Documentation | 95% complete | 3 active TODOs, 2 submodules missing API refs, feature gaps |
| Concurrency Safety | 15 findings | 5 HIGH (goroutine leaks, race conditions, resource leaks) |
| Security | 28 findings | 3 CRITICAL, 8 HIGH, 9 MEDIUM, 8 LOW |
| Test Coverage Gaps | Significant | Web components 6%, no fuzz/property/visual regression tests |
| Infrastructure | Excellent | SonarQube, Snyk, OWASP, Trivy, Prometheus, Grafana all configured |
| Performance | Needs work | Limited lazy loading, no stress test baselines, no optimization metrics |

### 1.3 Scale of Work

| Dimension | Items |
|-----------|-------|
| Phases | 10 |
| Total tasks | 187 |
| New test files to create | ~45 |
| New challenge definitions | ~60 |
| Documentation files to create/update | ~85 |
| Code files to fix/refactor | ~35 |
| New monitoring dashboards | 4 |
| Video course modules to extend | 12 |
| Website pages to create/update | 15 |

---

## 2. Current State Audit

### 2.1 Dead Code Inventory

| ID | Type | Location | Lines | Severity |
|----|------|----------|-------|----------|
| DC-001 | Unused Handler | `handlers/search.go` - `SearchHandler` | ~352 | HIGH |
| DC-002 | Unused Handler | `handlers/browse.go` - `BrowseHandler` | ~150 | HIGH |
| DC-003 | Unused Service | `services/sync_service.go` - `SyncService` | ~250 | HIGH |
| DC-004 | Unused Service | `services/webdav_client.go` - duplicate of `filesystem/webdav_client.go` | ~200 | HIGH |
| DC-005 | Unused Repository | `repository/sync_repository.go` - `SyncRepository` | ~150 | HIGH |
| DC-006 | Orphaned Package | `smb/` (top-level) - duplicate of `internal/smb/` | ~400 | HIGH |
| DC-007 | Empty Directory | `pkg/` - completely empty | 0 | LOW |
| DC-008 | Stale Artifacts | `old_results/` - 150+ stale JSON files | N/A | LOW |
| DC-009 | Stale Artifacts | `test-results/` - stale test output | N/A | LOW |
| DC-010 | Coverage Boosters | 8 `coverage_boost*_test.go` files with minimal real value | ~800 | MEDIUM |
| DC-011 | Manual Tests | `tests/manual/test_db.go`, `test_auth.go` - standalone binaries | ~200 | LOW |
| DC-012 | Unused Config | `config.go` CertFile/KeyFile fields (dynamic cert gen used instead) | ~10 | LOW |

**Decision Required for DC-001, DC-002:**
- Option A: Wire `SearchHandler` and `BrowseHandler` into router (connect dead features)
- Option B: Delete them (they duplicate functionality in `CatalogHandler` and `MediaBrowseHandler`)
- **Recommendation:** Option A - Wire them in. They provide dedicated search and browse UX that enhances the API.

**Decision Required for DC-003, DC-004, DC-005:**
- Option A: Wire `SyncService` into router (enable cloud sync feature: S3, GCS, WebDAV)
- Option B: Delete as dead code
- **Recommendation:** Option A - Wire them in. Cloud sync is a valuable feature already fully implemented.

### 2.2 Concurrency Safety Issues

| ID | Type | File | Line(s) | Risk |
|----|------|------|---------|------|
| CS-001 | Goroutine Leak | `internal/services/universal_scanner.go` | 223-229 | HIGH |
| CS-002 | Goroutine Leak | `internal/services/universal_scanner.go` | 289-296 | HIGH |
| CS-003 | Context Missing | `internal/media/realtime/watcher.go` | 277 | MEDIUM |
| CS-004 | Timer Leak | `internal/media/realtime/watcher.go` | 214-256 | MEDIUM-HIGH |
| CS-005 | Data Race | `internal/services/cache_service.go` | 679-702 | HIGH |
| CS-006 | Unbounded Channel | `internal/services/universal_scanner.go` | 106 | MEDIUM |
| CS-007 | Concurrent Map | `internal/services/cache_service.go` | 595-599 | MEDIUM |
| CS-008 | Resource Leak | `internal/services/aggregation_service.go` | 125-145 | HIGH |
| CS-009 | Untracked Timers | `internal/media/realtime/watcher.go` | 231-247 | HIGH |
| CS-010 | Lock Contention | `handlers/websocket_handler.go` | 105-113 | MEDIUM |
| CS-011 | Tx Cleanup | `internal/services/universal_scanner.go` | 666 | MEDIUM |
| CS-012 | Deadlock Risk | `internal/services/cache_service.go` | 108-109 | MEDIUM |
| CS-013 | React Leak | `catalog-web: UploadManager.tsx` | 46 | LOW-MEDIUM |
| CS-014 | React Leak | `catalog-web: MediaPlayer.tsx, Dashboard.tsx, CollectionRealTime.tsx` | Various | LOW-MEDIUM |
| CS-015 | Recursive Watch | `internal/media/realtime/enhanced_watcher.go` | 157-167 | MEDIUM |

### 2.3 Security Findings

#### CRITICAL (3)

| ID | Finding | Location |
|----|---------|----------|
| SEC-C01 | Default weak JWT secret (`aaa...`) | `catalog-api/.env:11` |
| SEC-C02 | Default weak admin password (`admin123`) | `catalog-api/.env:13` |
| SEC-C03 | MD5 used for security hashing | 6 files in `internal/services/` |

#### HIGH (8)

| ID | Finding | Location |
|----|---------|----------|
| SEC-H01 | Dynamic SQL via `fmt.Sprintf` | `internal/auth/service.go` |
| SEC-H02 | Command injection risk in conversions | `services/conversion_service.go` |
| SEC-H03 | Weak CORS defaults (localhost) | `handlers/log_management_handler.go` |
| SEC-H04 | Missing security headers (HSTS, CSP, X-Frame) | `main.go` |
| SEC-H05 | Weak test JWT secrets | `config/config_test.go` |
| SEC-H06 | Incomplete input validation coverage | `middleware/input_validation.go` |
| SEC-H07 | Self-signed TLS without cert caching | `main.go` |
| SEC-H08 | Credentials in .env.security (SonarQube/Snyk) | `.env.security` |

#### MEDIUM (9)

| ID | Finding | Location |
|----|---------|----------|
| SEC-M01 | .env in version control | `catalog-api/.env` |
| SEC-M02 | Sensitive data in error messages | Multiple handlers |
| SEC-M03 | Auth gaps on some endpoints | Various handlers |
| SEC-M04 | Password change without current password | `internal/auth/service.go` |
| SEC-M05 | Session management weaknesses | `internal/auth/service.go` |
| SEC-M06 | Insufficient security event logging | Audit logging system |
| SEC-M07 | Frontend XSS protection gaps | `catalog-web/src/` |
| SEC-M08 | Missing auth rate limiting | Auth endpoints |
| SEC-M09 | Database encryption not enforced | Database config |

#### LOW (8)

| ID | Finding | Location |
|----|---------|----------|
| SEC-L01 | HTTPS not enforced in dev | `.env:19` |
| SEC-L02 | Default NULL in migrations | Database migrations |
| SEC-L03 | Content-Type via extension only | Upload handlers |
| SEC-L04 | `math/rand` instead of `crypto/rand` | Multiple services |
| SEC-L05 | No CSRF tokens | Form handlers |
| SEC-L06 | Debug logs may contain secrets | Logging system |
| SEC-L07 | Dependency vulnerabilities (requires scan) | `go.mod`, `package.json` |
| SEC-L08 | No API versioning deprecation strategy | `/api/v1/` routes |

### 2.4 Test Coverage Gaps

| Area | Current | Target | Gap |
|------|---------|--------|-----|
| Go unit tests (catalog-api) | ~75% estimated | 95%+ | ~20% |
| catalog-web components | ~6% file coverage | 90%+ | ~84% |
| catalog-web types | ~10% file coverage | 100% | ~90% |
| Fuzz testing | 4 files exist | 15+ files | 11 new files |
| Property-based testing | 0 | 10+ files | 10 new files |
| Visual regression | 0 | 20+ screenshots | 20 new baselines |
| Contract/OpenAPI testing | 2 partial files | Full API coverage | Complete rewrite |
| Accessibility testing | 0 | Full WCAG 2.1 AA | New framework |
| Performance regression | Benchmarks exist, no baselines | Automated regression | New framework |
| Chaos engineering | 128 functions | 200+ functions | Extension |
| Submodule tests (Lazy) | 2 files | 5+ files | 3 new files |
| Submodule tests (Recovery) | 6 files | 10+ files | 4 new files |
| Snapshot tests | 1 file, 5 assertions | 30+ assertions | Extension |
| Android instrumented tests | Minimal | Full Compose UI | New suite |

### 2.5 Documentation Gaps

| Area | Current | Target | Gap |
|------|---------|--------|-----|
| Submodule API references | 1/3 complete (Memory) | 3/3 | Lazy, Recovery |
| Submodule changelogs | 1/3 complete (Memory) | 3/3 | Lazy, Recovery |
| Subtitle API guide | Referenced, not written | Complete guide | New document |
| Recommendation engine docs | Not documented | Architecture + API | New document |
| Advanced search docs | Not documented | User + developer guide | New document |
| Advanced analytics docs | Not documented | Dashboard guide | New document |
| OAuth/OIDC integration | Not documented | Setup guide | New document |
| Caching strategy | Not documented | Architecture doc | New document |
| Query optimization | Not documented | Developer guide | New document |
| Large collection handling | Not documented | Operations guide | New document |
| Prometheus metrics reference | TODO in deployment guide | Complete reference | New document |
| Alerting rules | Not documented | Runbook | New document |
| Video course (Modules 9-12) | Scripts exist | Scripts + exercises + slides | Extension |
| Website developer guides | 1 page (architecture) | 4 pages | 3 new pages |

---

## 3. Findings Registry

All findings are tracked with unique IDs for traceability:

- **DC-xxx**: Dead Code findings (12 items)
- **CS-xxx**: Concurrency Safety findings (15 items)
- **SEC-Cxx**: Security Critical findings (3 items)
- **SEC-Hxx**: Security High findings (8 items)
- **SEC-Mxx**: Security Medium findings (9 items)
- **SEC-Lxx**: Security Low findings (8 items)
- **TC-xxx**: Test Coverage gaps (14 areas)
- **DOC-xxx**: Documentation gaps (14 areas)

**Total tracked findings: 83**

---

## 4. Phased Implementation Plan

### Phase 1: Critical Security Hardening & Dead Code Resolution
**Duration estimate:** Focused work session
**Priority:** CRITICAL - Must be done first
**Dependencies:** None

#### 1.1 Security Critical Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 1.1.1 | Replace default JWT secret with 64-char crypto/rand generated value in `.env.example` | SEC-C01 | `catalog-api/.env`, `.env.example` |
| 1.1.2 | Replace default admin password with secure random in `.env.example`, add first-login password change requirement | SEC-C02 | `catalog-api/.env`, `internal/auth/service.go` |
| 1.1.3 | Replace all MD5 usage with SHA-256 in security contexts | SEC-C03 | `internal/services/cache_service.go`, `internal/media/realtime/enhanced_watcher.go`, `internal/services/book_recognition_provider.go`, `internal/services/music_recognition_provider.go`, `internal/services/game_software_recognition_provider.go`, `internal/services/cover_art_service.go` |
| 1.1.4 | Add `.env` to `.gitignore`, create `.env.example` with placeholders | SEC-M01 | `.gitignore`, `.env.example` |
| 1.1.5 | Remove hardcoded credentials from `.env.security`, use placeholders | SEC-H08 | `.env.security` |

#### 1.2 Security High Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 1.2.1 | Fix dynamic SQL construction - use parameterized queries exclusively | SEC-H01 | `internal/auth/service.go` |
| 1.2.2 | Sanitize file paths in conversion service, validate extensions, use `cmd.Args` array form | SEC-H02 | `services/conversion_service.go` |
| 1.2.3 | Fix CORS: remove localhost defaults, require explicit config, validate origin format | SEC-H03 | `handlers/log_management_handler.go`, `internal/handlers/media_player_handlers.go`, `internal/handlers/localization_handlers.go` |
| 1.2.4 | Add security headers middleware: HSTS, X-Content-Type-Options, X-Frame-Options, CSP, Referrer-Policy | SEC-H04 | New: `middleware/security_headers.go`, `main.go` |
| 1.2.5 | Apply input validation middleware to ALL endpoints, not just select ones | SEC-H06 | `middleware/input_validation.go`, `main.go` |
| 1.2.6 | Cache generated TLS certificates across restarts | SEC-H07 | `main.go` |
| 1.2.7 | Replace `math/rand` with `crypto/rand` in security contexts | SEC-L04 | Multiple service files |

#### 1.3 Dead Code Resolution

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 1.3.1 | Wire `SearchHandler` into router at `/api/v1/search/*` | DC-001 | `main.go`, `handlers/search.go` |
| 1.3.2 | Wire `BrowseHandler` into router at `/api/v1/browse/*` | DC-002 | `main.go`, `handlers/browse.go` |
| 1.3.3 | Wire `SyncService` + `SyncRepository` into router at `/api/v1/sync/*` | DC-003, DC-005 | `main.go`, `services/sync_service.go`, `repository/sync_repository.go` |
| 1.3.4 | Delete duplicate `services/webdav_client.go` (keep `filesystem/webdav_client.go`) | DC-004 | Delete `services/webdav_client.go` |
| 1.3.5 | Delete orphaned `smb/` top-level package (keep `internal/smb/`) | DC-006 | Delete `smb/` directory |
| 1.3.6 | Delete empty `pkg/` directory | DC-007 | Delete `pkg/` |
| 1.3.7 | Delete stale `old_results/` and `test-results/` directories, add to `.gitignore` | DC-008, DC-009 | Delete directories, update `.gitignore` |
| 1.3.8 | Replace coverage booster tests with meaningful tests or delete | DC-010 | 8 `coverage_boost*_test.go` files |
| 1.3.9 | Convert manual test files to proper Go test functions or document as dev tools | DC-011 | `tests/manual/test_db.go`, `tests/manual/test_auth.go` |

#### 1.4 Verification

| Step | Task |
|------|------|
| 1.4.1 | Run `GOMAXPROCS=3 go test ./... -p 2 -parallel 2` - all must pass |
| 1.4.2 | Run `go vet ./...` - zero warnings |
| 1.4.3 | Run `cd catalog-web && npm test` - all must pass |
| 1.4.4 | Verify newly wired endpoints respond correctly via curl/httpie |
| 1.4.5 | Run `govulncheck ./...` - zero vulnerabilities |

---

### Phase 2: Concurrency Safety & Memory Leak Fixes
**Priority:** HIGH
**Dependencies:** Phase 1 complete

#### 2.1 Goroutine Leak Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.1.1 | Add WaitGroup tracking to deferred cleanup goroutine in `processScanJob()` | CS-001 | `internal/services/universal_scanner.go:223-229` |
| 2.1.2 | Add WaitGroup tracking to post-scan aggregation goroutine | CS-002 | `internal/services/universal_scanner.go:289-296` |
| 2.1.3 | Add WaitGroup tracking to all debounce timer callbacks | CS-009 | `internal/media/realtime/watcher.go:231-247` |
| 2.1.4 | Implement graceful shutdown with context cancellation for all background goroutines | CS-001, CS-002, CS-009 | `internal/services/universal_scanner.go` |

#### 2.2 Race Condition Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.2.1 | Add sync guard between `recordCacheActivity()` goroutine and `Close()` teardown | CS-005 | `internal/services/cache_service.go:679-702` |
| 2.2.2 | Protect CacheStats map writes with sync.RWMutex | CS-007 | `internal/services/cache_service.go:595-599` |
| 2.2.3 | Add timeout to `s.wg.Wait()` in `Close()` to prevent deadlock on stuck goroutines | CS-012 | `internal/services/cache_service.go:108-109` |

#### 2.3 Resource Leak Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.3.1 | Ensure `sql.Rows` closed on ALL code paths in aggregation loop | CS-008 | `internal/services/aggregation_service.go:125-145` |
| 2.3.2 | Add max debounce map size with eviction policy | CS-004 | `internal/media/realtime/watcher.go:214-256` |
| 2.3.3 | Add max watch depth limit for recursive directory watching | CS-015 | `internal/media/realtime/enhanced_watcher.go:157-167` |

#### 2.4 Context & Concurrency Improvements

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.4.1 | Propagate parent context through watcher `processChange()` | CS-003 | `internal/media/realtime/watcher.go:277` |
| 2.4.2 | Reduce lock scope in WebSocket broadcast - release before `writeMessage` | CS-010 | `handlers/websocket_handler.go:105-113` |
| 2.4.3 | Add backpressure to scan queue (reduce buffer or add feedback) | CS-006 | `internal/services/universal_scanner.go:106` |
| 2.4.4 | Add proper transaction error handling with logged rollback failures | CS-011 | `internal/services/universal_scanner.go:666` |

#### 2.5 React Memory Leak Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.5.1 | Add setTimeout cleanup in UploadManager useEffect | CS-013 | `catalog-web/src/components/upload/UploadManager.tsx:46` |
| 2.5.2 | Add useEffect cleanup (removeEventListener, clearInterval) in MediaPlayer, Dashboard, CollectionRealTime | CS-014 | Multiple catalog-web component files |

#### 2.6 Verification

| Step | Task |
|------|------|
| 2.6.1 | Run `GOMAXPROCS=3 go test -race ./... -p 2 -parallel 2` - zero races |
| 2.6.2 | Run stress tests: `go test -run TestMemoryLeakDetection ./tests/stress/` |
| 2.6.3 | Run `cd catalog-web && npm test` - all pass |
| 2.6.4 | Manual verification: start server, trigger scans, verify graceful shutdown |

---

### Phase 3: Lazy Loading, Semaphores & Non-Blocking Optimizations
**Priority:** HIGH
**Dependencies:** Phase 2 complete

#### 3.1 Lazy Initialization Expansion

| Step | Task | Files |
|------|------|-------|
| 3.1.1 | Wrap all service constructors in `main.go` with `lazy.Service[T]` from Lazy module - only initialize when first request arrives | `main.go`, all handler/service constructors |
| 3.1.2 | Lazy-load database migrations - only run on first DB access, not on startup | `database/connection.go`, `database/migrations.go` |
| 3.1.3 | Lazy-load TMDB/OMDB/IMDB provider clients - only when media recognition requested | `internal/media/providers/*.go` |
| 3.1.4 | Lazy-load Redis connection - only when caching is actually used | `internal/services/cache_service.go` |
| 3.1.5 | Lazy-load WebSocket hub - only when first WS connection arrives | `handlers/websocket_handler.go` |
| 3.1.6 | Lazy-load filesystem protocol clients (FTP, SMB, NFS, WebDAV) - only on first access | `filesystem/factory.go` |
| 3.1.7 | Implement lazy component loading in catalog-web with React.lazy() and Suspense for all route-level pages | `catalog-web/src/App.tsx`, all page imports |
| 3.1.8 | Add lazy image loading (IntersectionObserver) for media thumbnails and cover art | `catalog-web/src/components/media/` |

#### 3.2 Semaphore & Rate Control

| Step | Task | Files |
|------|------|-------|
| 3.2.1 | Add semaphore to database connection pool - limit concurrent queries to configurable max | `database/connection.go` |
| 3.2.2 | Add semaphore to file conversion pipeline - limit concurrent ffmpeg/LibreOffice processes | `services/conversion_service.go` |
| 3.2.3 | Add semaphore to media recognition pipeline - limit concurrent provider API calls | `internal/media/detector/engine.go` |
| 3.2.4 | Add semaphore to WebSocket broadcast - limit concurrent client writes | `handlers/websocket_handler.go` |
| 3.2.5 | Add semaphore to aggregation service - limit concurrent post-scan processing | `internal/services/aggregation_service.go` |
| 3.2.6 | Implement request-level semaphore in middleware for global concurrency cap | New: `middleware/concurrency_limiter.go` |

#### 3.3 Non-Blocking Patterns

| Step | Task | Files |
|------|------|-------|
| 3.3.1 | Convert synchronous scan status polling to WebSocket push notifications | `handlers/scan_handler.go`, `internal/services/universal_scanner.go` |
| 3.3.2 | Use non-blocking channel sends with select/default for all event bus publishing | `internal/eventbus/eventbus.go` |
| 3.3.3 | Implement async write-behind for analytics/access logging (buffer + periodic flush) | `handlers/analytics_handler.go`, `repository/analytics_repository.go` |
| 3.3.4 | Add request timeout middleware with configurable per-route timeouts | New: `middleware/timeout.go` |
| 3.3.5 | Implement connection draining for graceful HTTP server shutdown | `main.go` |
| 3.3.6 | Use React Query's `staleTime` and `cacheTime` to reduce redundant API calls | `catalog-web/src/` (all useQuery hooks) |
| 3.3.7 | Add virtualized scrolling for large media lists (react-window or react-virtuoso) | `catalog-web/src/components/media/` |

#### 3.4 Verification

| Step | Task |
|------|------|
| 3.4.1 | Run full test suite - zero regressions |
| 3.4.2 | Measure startup time before/after lazy loading (target: 50% reduction) |
| 3.4.3 | Measure memory usage at idle before/after (target: 30% reduction) |
| 3.4.4 | Run stress tests to verify semaphores prevent overload |
| 3.4.5 | Verify graceful shutdown completes within 30 seconds under load |

---

### Phase 4: Test Coverage Maximization
**Priority:** HIGH
**Dependencies:** Phases 1-3 complete (code changes stabilized)

#### 4.1 Go Unit Test Expansion

| Step | Task | Target Coverage |
|------|------|----------------|
| 4.1.1 | Add tests for newly wired `SearchHandler` endpoints | 95%+ |
| 4.1.2 | Add tests for newly wired `BrowseHandler` endpoints | 95%+ |
| 4.1.3 | Add tests for newly wired `SyncService` + `SyncRepository` | 95%+ |
| 4.1.4 | Add tests for security headers middleware | 100% |
| 4.1.5 | Add tests for concurrency limiter middleware | 100% |
| 4.1.6 | Add tests for timeout middleware | 100% |
| 4.1.7 | Expand database dialect tests for all rewrite paths | 100% |
| 4.1.8 | Add tests for all error paths in aggregation service | 95%+ |
| 4.1.9 | Add tests for graceful shutdown sequences | 95%+ |
| 4.1.10 | Add tests for lazy service initialization patterns | 100% |

#### 4.2 Go Fuzz Testing

| Step | Task | File |
|------|------|------|
| 4.2.1 | Add fuzz tests for input validation middleware (SQL injection, XSS, path traversal patterns) | New: `middleware/input_validation_fuzz_test.go` |
| 4.2.2 | Add fuzz tests for JWT token parsing | New: `internal/auth/jwt_fuzz_test.go` |
| 4.2.3 | Add fuzz tests for media entity title parser (all 11 media types) | Extend: `internal/services/title_parser_fuzz_test.go` |
| 4.2.4 | Add fuzz tests for URL/path parsing in filesystem clients | New: `filesystem/path_fuzz_test.go` |
| 4.2.5 | Add fuzz tests for WebSocket message parsing | New: `handlers/websocket_fuzz_test.go` |
| 4.2.6 | Add fuzz tests for search query parsing | New: `handlers/search_fuzz_test.go` |
| 4.2.7 | Add fuzz tests for configuration file parsing | New: `config/config_fuzz_test.go` |
| 4.2.8 | Add fuzz tests for sync endpoint URL validation | New: `services/sync_service_fuzz_test.go` |
| 4.2.9 | Add fuzz tests for analytics event parsing | New: `handlers/analytics_fuzz_test.go` |
| 4.2.10 | Add fuzz tests for collection name/metadata validation | New: `handlers/collection_fuzz_test.go` |
| 4.2.11 | Add fuzz tests for subtitle file format parsing | New: `internal/services/subtitle_fuzz_test.go` |

#### 4.3 Go Benchmark Expansion

| Step | Task | File |
|------|------|------|
| 4.3.1 | Add benchmarks for security headers middleware | New: `middleware/security_headers_bench_test.go` |
| 4.3.2 | Add benchmarks for input validation middleware | New: `middleware/input_validation_bench_test.go` |
| 4.3.3 | Add benchmarks for lazy service initialization | New: `Lazy/pkg/lazy/lazy_bench_test.go` |
| 4.3.4 | Add benchmarks for circuit breaker operations | New: `Recovery/pkg/breaker/breaker_bench_test.go` |
| 4.3.5 | Add benchmarks for memory store operations | New: `Memory/pkg/store/store_bench_test.go` |
| 4.3.6 | Add benchmarks for knowledge graph traversal | New: `Memory/pkg/graph/graph_bench_test.go` |
| 4.3.7 | Add benchmarks for WebSocket broadcast throughput | New: `handlers/websocket_bench_test.go` |
| 4.3.8 | Add benchmarks for sync service operations | New: `services/sync_service_bench_test.go` |

#### 4.4 React/TypeScript Test Expansion

| Step | Task | Target |
|------|------|--------|
| 4.4.1 | Add unit tests for ALL 62 catalog-web components (currently 4 tested) | 90%+ file coverage |
| 4.4.2 | Add unit tests for ALL 10 type definition files | 100% |
| 4.4.3 | Add snapshot tests for all UI components (extend from 5 to 60+ assertions) | Full visual baseline |
| 4.4.4 | Add React Hook tests for all custom hooks | 100% |
| 4.4.5 | Add Zustand store tests | 100% |
| 4.4.6 | Add React Query integration tests | 90%+ |
| 4.4.7 | Add form validation tests (Zod schema coverage) | 100% |

#### 4.5 Visual Regression Testing

| Step | Task | File |
|------|------|------|
| 4.5.1 | Configure Playwright `toHaveScreenshot()` in `playwright.config.ts` | `catalog-web/playwright.config.ts` |
| 4.5.2 | Add visual regression tests for login page | New: `e2e/visual/login.spec.ts` |
| 4.5.3 | Add visual regression tests for dashboard | New: `e2e/visual/dashboard.spec.ts` |
| 4.5.4 | Add visual regression tests for media browser | New: `e2e/visual/media-browser.spec.ts` |
| 4.5.5 | Add visual regression tests for collection manager | New: `e2e/visual/collections.spec.ts` |
| 4.5.6 | Add visual regression tests for settings/admin pages | New: `e2e/visual/settings.spec.ts` |
| 4.5.7 | Add visual regression tests for media player | New: `e2e/visual/player.spec.ts` |
| 4.5.8 | Add visual regression tests for responsive layouts (mobile, tablet, desktop) | New: `e2e/visual/responsive.spec.ts` |

#### 4.6 Accessibility Testing

| Step | Task | File |
|------|------|------|
| 4.6.1 | Install axe-core Playwright integration | `catalog-web/package.json` |
| 4.6.2 | Add WCAG 2.1 AA accessibility tests for all pages | New: `e2e/accessibility/pages.spec.ts` |
| 4.6.3 | Add keyboard navigation tests | New: `e2e/accessibility/keyboard.spec.ts` |
| 4.6.4 | Add screen reader compatibility tests | New: `e2e/accessibility/screen-reader.spec.ts` |
| 4.6.5 | Add color contrast verification tests | New: `e2e/accessibility/contrast.spec.ts` |

#### 4.7 Contract/OpenAPI Testing

| Step | Task | File |
|------|------|------|
| 4.7.1 | Generate Go client from `docs/api/openapi.yaml` for contract verification | New: `tests/contract/openapi_client_test.go` |
| 4.7.2 | Add response schema validation tests for ALL 80+ endpoints | Extend: `tests/integration/contract_test.go` |
| 4.7.3 | Add request validation tests (invalid payloads rejected) | New: `tests/contract/request_validation_test.go` |
| 4.7.4 | Add backward compatibility tests (no breaking changes) | New: `tests/contract/backward_compat_test.go` |

#### 4.8 Property-Based Testing

| Step | Task | File |
|------|------|------|
| 4.8.1 | Add property tests for database dialect rewriting (roundtrip correctness) | New: `database/dialect_property_test.go` |
| 4.8.2 | Add property tests for filesystem path normalization | New: `filesystem/path_property_test.go` |
| 4.8.3 | Add property tests for media type detection (invariants) | New: `internal/media/detector/property_test.go` |
| 4.8.4 | Add property tests for title parser (parse/format roundtrip) | New: `internal/services/title_parser_property_test.go` |
| 4.8.5 | Add property tests for JWT token generation/validation | New: `internal/auth/jwt_property_test.go` |
| 4.8.6 | Add property tests for collection sorting/filtering | New: `handlers/collection_property_test.go` |
| 4.8.7 | Add property tests for pagination (monotonicity, completeness) | New: `handlers/pagination_property_test.go` |
| 4.8.8 | Add property tests for rate limiter (fairness, bounded latency) | New: `middleware/rate_limiter_property_test.go` |

#### 4.9 Submodule Test Expansion

| Step | Task | File |
|------|------|------|
| 4.9.1 | Add edge case tests for Lazy `Value[T]` (concurrent Reset, nil factory) | New: `Lazy/pkg/lazy/lazy_edge_test.go` |
| 4.9.2 | Add stress tests for Lazy `Service[T]` (1000 concurrent getters) | New: `Lazy/pkg/lazy/lazy_stress_test.go` |
| 4.9.3 | Add fuzz tests for Lazy factory functions | New: `Lazy/pkg/lazy/lazy_fuzz_test.go` |
| 4.9.4 | Add integration tests for Recovery facade (breaker + health combined) | New: `Recovery/pkg/facade/facade_integration_test.go` |
| 4.9.5 | Add stress tests for Recovery circuit breaker state transitions | New: `Recovery/pkg/breaker/breaker_stress_test.go` |
| 4.9.6 | Add fuzz tests for Memory entity extraction patterns | New: `Memory/pkg/entity/entity_fuzz_test.go` |
| 4.9.7 | Add stress tests for Memory knowledge graph (10K nodes) | New: `Memory/pkg/graph/graph_stress_test.go` |

#### 4.10 Verification

| Step | Task |
|------|------|
| 4.10.1 | Run `go test -coverprofile=coverage.out ./...` - generate coverage report |
| 4.10.2 | Verify Go coverage >= 95% for all packages |
| 4.10.3 | Run `npm run test:coverage` - verify catalog-web coverage >= 90% |
| 4.10.4 | Run all fuzz tests: `go test -fuzz=. -fuzztime=30s ./...` |
| 4.10.5 | Run visual regression: `npm run test:e2e -- --grep visual` |
| 4.10.6 | Run accessibility: `npm run test:e2e -- --grep accessibility` |

---

### Phase 5: Stress, Integration & Monitoring Tests
**Priority:** HIGH
**Dependencies:** Phase 4 complete

#### 5.1 Stress Test Expansion

| Step | Task | File |
|------|------|------|
| 5.1.1 | Add stress test: 1000 concurrent API requests with auth | Extend: `tests/stress/api_load_test.go` |
| 5.1.2 | Add stress test: 500 concurrent WebSocket connections with message broadcast | Extend: `tests/stress/websocket_stress_test.go` |
| 5.1.3 | Add stress test: 100 concurrent file scans across 4 protocols | New: `tests/stress/multi_protocol_stress_test.go` |
| 5.1.4 | Add stress test: Database connection pool exhaustion and recovery | Extend: `tests/stress/database_stress_test.go` |
| 5.1.5 | Add stress test: Memory pressure with 10K media entities | Extend: `tests/stress/memory_pressure_test.go` |
| 5.1.6 | Add stress test: Rate limiter correctness under sustained load | Extend: `tests/stress/rate_limiter_stress_test.go` |
| 5.1.7 | Add stress test: Graceful degradation under resource starvation | Extend: `tests/stress/responsiveness_test.go` |
| 5.1.8 | Add stress test: Cache service under thundering herd | New: `tests/stress/cache_stress_test.go` |
| 5.1.9 | Add stress test: Sync service with concurrent cloud operations | New: `tests/stress/sync_stress_test.go` |
| 5.1.10 | Add stress test: Search service with complex query patterns | New: `tests/stress/search_stress_test.go` |

#### 5.2 Integration Test Expansion

| Step | Task | File |
|------|------|------|
| 5.2.1 | Add integration test: Full user flow (register -> login -> scan -> browse -> collect -> sync) | Extend: `tests/integration/user_flows_test.go` |
| 5.2.2 | Add integration test: Multi-protocol scan + aggregation + entity creation | New: `tests/integration/scan_to_entity_test.go` |
| 5.2.3 | Add integration test: WebSocket event delivery during concurrent scans | New: `tests/integration/websocket_events_test.go` |
| 5.2.4 | Add integration test: Search across all media types with pagination | New: `tests/integration/search_integration_test.go` |
| 5.2.5 | Add integration test: Collection management with real database | New: `tests/integration/collection_integration_test.go` |
| 5.2.6 | Add integration test: Analytics data collection and reporting | New: `tests/integration/analytics_integration_test.go` |
| 5.2.7 | Add integration test: Subtitle search, download, and verification | New: `tests/integration/subtitle_integration_test.go` |
| 5.2.8 | Add integration test: Cover art retrieval and caching | New: `tests/integration/cover_art_integration_test.go` |
| 5.2.9 | Add integration test: Configuration wizard complete flow | New: `tests/integration/config_wizard_integration_test.go` |
| 5.2.10 | Add integration test: Error reporting and log management | New: `tests/integration/error_reporting_integration_test.go` |

#### 5.3 Monitoring & Metrics Tests

| Step | Task | File |
|------|------|------|
| 5.3.1 | Add test: Prometheus metrics export format correctness | New: `tests/monitoring/prometheus_export_test.go` |
| 5.3.2 | Add test: HTTP request metrics accuracy (count, duration, status) | New: `tests/monitoring/http_metrics_test.go` |
| 5.3.3 | Add test: Runtime metrics collection (goroutines, memory) | Extend: `tests/monitoring/resource_monitor_test.go` |
| 5.3.4 | Add test: Database query metrics tracking | New: `tests/monitoring/database_metrics_test.go` |
| 5.3.5 | Add test: SMB health metrics reporting | New: `tests/monitoring/smb_health_metrics_test.go` |
| 5.3.6 | Add test: WebSocket connection metrics | New: `tests/monitoring/websocket_metrics_test.go` |
| 5.3.7 | Add test: Custom metric creation and recording | New: `tests/monitoring/custom_metrics_test.go` |
| 5.3.8 | Add test: Metrics under load (high-cardinality label prevention) | New: `tests/monitoring/metrics_load_test.go` |
| 5.3.9 | Add test: Grafana dashboard JSON validity and data source references | New: `tests/monitoring/grafana_dashboard_test.go` |
| 5.3.10 | Add test: Alert rule evaluation (threshold triggers) | New: `tests/monitoring/alert_rules_test.go` |

#### 5.4 Chaos Engineering Expansion

| Step | Task | File |
|------|------|------|
| 5.4.1 | Add chaos test: Database connection drop during active transaction | Extend: `tests/integration/chaos_test.go` |
| 5.4.2 | Add chaos test: Redis unavailability with graceful fallback | New: `tests/integration/chaos_redis_test.go` |
| 5.4.3 | Add chaos test: WebSocket disconnect/reconnect storm | New: `tests/integration/chaos_websocket_test.go` |
| 5.4.4 | Add chaos test: File system permission changes mid-scan | New: `tests/integration/chaos_filesystem_test.go` |
| 5.4.5 | Add chaos test: Clock skew affecting JWT validation | New: `tests/integration/chaos_time_test.go` |
| 5.4.6 | Add chaos test: Concurrent service shutdown and restart | New: `tests/integration/chaos_lifecycle_test.go` |

#### 5.5 Verification

| Step | Task |
|------|------|
| 5.5.1 | Run all stress tests: `go test -run TestStress ./tests/stress/ -timeout 10m` |
| 5.5.2 | Run all integration tests: `go test ./tests/integration/ -timeout 15m` |
| 5.5.3 | Run all monitoring tests: `go test ./tests/monitoring/` |
| 5.5.4 | Collect metrics during stress run - verify Prometheus scraping works |
| 5.5.5 | Verify Grafana dashboard renders correctly with collected data |

---

### Phase 6: Security Scanning Execution & Remediation
**Priority:** HIGH
**Dependencies:** Phases 1-5 complete

#### 6.1 Run Security Scans

| Step | Task | Command |
|------|------|---------|
| 6.1.1 | Start SonarQube infrastructure | `podman-compose -f docker-compose.security.yml up -d sonarqube sonarqube-db` |
| 6.1.2 | Wait for SonarQube readiness | Poll `http://localhost:9090/api/system/status` until UP |
| 6.1.3 | Run SonarQube analysis | `sonar-scanner` (uses `sonar-project.properties`) |
| 6.1.4 | Run Snyk dependency scan | `podman-compose -f docker-compose.security.yml --profile snyk-scan run --rm snyk-cli` |
| 6.1.5 | Run OWASP Dependency Check | `podman-compose -f docker-compose.security.yml --profile dependency-check run --rm dependency-check` |
| 6.1.6 | Run Trivy filesystem scan | `podman-compose -f docker-compose.security.yml --profile trivy-scan run --rm trivy-scanner` |
| 6.1.7 | Run `govulncheck ./...` in catalog-api | `cd catalog-api && govulncheck ./...` |
| 6.1.8 | Run `npm audit` in all JS/TS projects | `cd catalog-web && npm audit --production` (repeat for all) |

#### 6.2 Analyze & Remediate Findings

| Step | Task |
|------|------|
| 6.2.1 | Collect all scan reports from `reports/` directory |
| 6.2.2 | Triage findings by severity: Critical > High > Medium > Low |
| 6.2.3 | Fix all Critical and High dependency vulnerabilities |
| 6.2.4 | Fix all SonarQube bugs and code smells marked Critical/Major |
| 6.2.5 | Update dependencies with known vulnerabilities (`go get -u`, `npm audit fix`) |
| 6.2.6 | Add suppression rules for false positives with documented justification |
| 6.2.7 | Re-run all scans to verify remediation |

#### 6.3 Security Test Automation

| Step | Task | File |
|------|------|------|
| 6.3.1 | Add test: SQL injection attempts on all endpoints | Extend: `tests/security/injection_test.go` |
| 6.3.2 | Add test: XSS payload rejection on all input fields | New: `tests/security/xss_test.go` |
| 6.3.3 | Add test: Path traversal prevention | New: `tests/security/path_traversal_test.go` |
| 6.3.4 | Add test: Authentication bypass attempts | New: `tests/security/auth_bypass_test.go` |
| 6.3.5 | Add test: CORS policy enforcement | New: `tests/security/cors_test.go` |
| 6.3.6 | Add test: Security headers presence on all responses | New: `tests/security/headers_test.go` |
| 6.3.7 | Add test: Rate limiting enforcement on auth endpoints | New: `tests/security/rate_limit_test.go` |
| 6.3.8 | Add test: JWT token expiry and refresh flow | New: `tests/security/jwt_lifecycle_test.go` |
| 6.3.9 | Add test: File upload content-type validation (magic bytes) | New: `tests/security/upload_validation_test.go` |
| 6.3.10 | Add test: Command injection prevention in conversion | New: `tests/security/command_injection_test.go` |

#### 6.4 Verification

| Step | Task |
|------|------|
| 6.4.1 | All security scans produce zero Critical/High findings |
| 6.4.2 | All security tests pass |
| 6.4.3 | `govulncheck` reports zero vulnerabilities |
| 6.4.4 | `npm audit --production` reports zero critical/high vulnerabilities |

---

### Phase 7: Challenge Expansion
**Priority:** MEDIUM
**Dependencies:** Phases 1-6 complete (all new features and fixes in place)

#### 7.1 New Challenges for Newly Wired Features

| ID | Challenge | Category | Validates |
|----|-----------|----------|-----------|
| CH-061 | Search API basic query | search | DC-001 wiring |
| CH-062 | Search API duplicate detection | search | DC-001 wiring |
| CH-063 | Search API advanced filters | search | DC-001 wiring |
| CH-064 | Browse API storage roots | browse | DC-002 wiring |
| CH-065 | Browse API directory listing | browse | DC-002 wiring |
| CH-066 | Sync API endpoint creation | sync | DC-003 wiring |
| CH-067 | Sync API cloud providers (S3, GCS) | sync | DC-003 wiring |
| CH-068 | Sync API user endpoints management | sync | DC-003 wiring |

#### 7.2 Security Validation Challenges

| ID | Challenge | Category | Validates |
|----|-----------|----------|-----------|
| CH-069 | Security headers present on all responses | security | SEC-H04 fix |
| CH-070 | CORS policy rejects unauthorized origins | security | SEC-H03 fix |
| CH-071 | Input validation rejects injection attempts | security | SEC-H06 fix |
| CH-072 | Rate limiting enforced on auth endpoints | security | SEC-M08 fix |
| CH-073 | JWT token lifecycle (issue, refresh, expire, revoke) | security | SEC-C01 fix |
| CH-074 | File upload validates content-type via magic bytes | security | SEC-L03 fix |
| CH-075 | Conversion service rejects path traversal | security | SEC-H02 fix |

#### 7.3 Performance & Resilience Challenges

| ID | Challenge | Category | Validates |
|----|-----------|----------|-----------|
| CH-076 | API response time under 100ms for simple queries | performance | Phase 3 optimizations |
| CH-077 | API handles 500 concurrent requests without errors | stress | Phase 3 semaphores |
| CH-078 | Graceful degradation under resource starvation | resilience | Phase 3 non-blocking |
| CH-079 | Memory usage stable during sustained load | memory | Phase 2 leak fixes |
| CH-080 | Database connection pool recovery after exhaustion | resilience | Phase 5 chaos |
| CH-081 | WebSocket reconnection after server restart | resilience | Phase 2 goroutine fixes |
| CH-082 | Lazy service initialization on first request | lazy | Phase 3 lazy loading |
| CH-083 | Semaphore prevents concurrent overload | concurrency | Phase 3 semaphores |

#### 7.4 Monitoring Challenges

| ID | Challenge | Category | Validates |
|----|-----------|----------|-----------|
| CH-084 | Prometheus metrics endpoint returns valid data | monitoring | Phase 5 metrics tests |
| CH-085 | HTTP request metrics increment on API calls | monitoring | Phase 5 metrics tests |
| CH-086 | Runtime metrics (goroutines, memory) are current | monitoring | Phase 5 metrics tests |
| CH-087 | Database query duration metrics are recorded | monitoring | Phase 5 metrics tests |
| CH-088 | Grafana dashboard loads and displays data | monitoring | Phase 5 dashboard tests |

#### 7.5 Module Verification Challenges

| ID | Challenge | Category | Validates |
|----|-----------|----------|-----------|
| MOD-016 | Lazy Value[T] concurrent access safety | module | Lazy module |
| MOD-017 | Lazy Service[T] singleton guarantee | module | Lazy module |
| MOD-018 | Recovery CircuitBreaker state transitions | module | Recovery module |
| MOD-019 | Recovery HealthChecker periodic verification | module | Recovery module |
| MOD-020 | Memory LeakDetector goroutine tracking | module | Memory module |
| MOD-021 | Memory KnowledgeGraph BFS traversal | module | Memory module |

#### 7.6 Registration & Verification

| Step | Task |
|------|------|
| 7.6.1 | Implement all challenge structs in `catalog-api/challenges/` |
| 7.6.2 | Register all new challenges in `catalog-api/challenges/register.go` |
| 7.6.3 | Add challenge tests in `catalog-api/challenges/ch061_090_test.go` |
| 7.6.4 | Verify total registered = 239 (existing) + 34 (new) = 273 challenges |
| 7.6.5 | Run: `go test -run TestChallengeRegistration ./challenges/` |

---

### Phase 8: Documentation Completion
**Priority:** MEDIUM
**Dependencies:** Phases 1-7 complete (all features finalized)

#### 8.1 Submodule Documentation

| Step | Task | File |
|------|------|------|
| 8.1.1 | Write Lazy module API reference | `Lazy/docs/API_REFERENCE.md` |
| 8.1.2 | Write Lazy module changelog | `Lazy/docs/CHANGELOG.md` |
| 8.1.3 | Write Recovery module API reference | `Recovery/docs/API_REFERENCE.md` |
| 8.1.4 | Write Recovery module changelog | `Recovery/docs/CHANGELOG.md` |
| 8.1.5 | Update Memory module docs with any Phase 2 changes | `Memory/docs/` |

#### 8.2 Feature Documentation

| Step | Task | File |
|------|------|------|
| 8.2.1 | Write Search API documentation (endpoints, query syntax, examples) | `docs/api/SEARCH_API.md` |
| 8.2.2 | Write Browse API documentation | `docs/api/BROWSE_API.md` |
| 8.2.3 | Write Sync API documentation (cloud providers, endpoint config) | `docs/api/SYNC_API.md` |
| 8.2.4 | Write Subtitle API guide (search, download, translate) | `docs/guides/SUBTITLE_GUIDE.md` |
| 8.2.5 | Write Recommendation engine documentation | `docs/architecture/RECOMMENDATION_ENGINE.md` |
| 8.2.6 | Write Advanced search documentation | `docs/guides/ADVANCED_SEARCH.md` |
| 8.2.7 | Write Advanced analytics documentation | `docs/guides/ANALYTICS_GUIDE.md` |
| 8.2.8 | Write Caching strategy architecture doc | `docs/architecture/CACHING_STRATEGY.md` |
| 8.2.9 | Write Query optimization guide | `docs/architecture/QUERY_OPTIMIZATION.md` |
| 8.2.10 | Write Large collection handling operations guide | `docs/guides/LARGE_COLLECTIONS.md` |

#### 8.3 Security Documentation

| Step | Task | File |
|------|------|------|
| 8.3.1 | Write comprehensive security headers documentation | `docs/security/SECURITY_HEADERS.md` |
| 8.3.2 | Write CORS configuration guide | `docs/security/CORS_CONFIGURATION.md` |
| 8.3.3 | Write secrets management guide | `docs/security/SECRETS_MANAGEMENT.md` |
| 8.3.4 | Write OAuth/OIDC integration guide | `docs/guides/OAUTH_INTEGRATION.md` |
| 8.3.5 | Update security audit report with remediation results | `docs/security/COMPREHENSIVE_SECURITY_AUDIT.md` |
| 8.3.6 | Write security scanning runbook (how to run Snyk, SonarQube, Trivy, OWASP) | `docs/security/SECURITY_SCANNING_RUNBOOK.md` |

#### 8.4 Monitoring Documentation

| Step | Task | File |
|------|------|------|
| 8.4.1 | Write Prometheus metrics reference (all custom metrics, labels, types) | `docs/guides/PROMETHEUS_METRICS_REFERENCE.md` |
| 8.4.2 | Write alerting rules documentation and runbook | `docs/guides/ALERTING_RULES.md` |
| 8.4.3 | Write Grafana dashboard guide (how to use, customize, extend) | `docs/guides/GRAFANA_DASHBOARD_GUIDE.md` |
| 8.4.4 | Write log aggregation strategy | `docs/architecture/LOG_AGGREGATION.md` |
| 8.4.5 | Remove TODO from production deployment guide ("Implement Prometheus metrics export") | `docs/deployment/PRODUCTION_DEPLOYMENT_GUIDE.md` |

#### 8.5 Performance Documentation

| Step | Task | File |
|------|------|------|
| 8.5.1 | Write lazy loading architecture guide | `docs/architecture/LAZY_LOADING.md` |
| 8.5.2 | Write concurrency control guide (semaphores, rate limiting) | `docs/architecture/CONCURRENCY_CONTROL.md` |
| 8.5.3 | Write performance tuning guide (resource limits, connection pools) | `docs/guides/PERFORMANCE_TUNING.md` |
| 8.5.4 | Write stress test results report | `docs/testing/STRESS_TEST_RESULTS.md` |

#### 8.6 Test Documentation Updates

| Step | Task | File |
|------|------|------|
| 8.6.1 | Update testing guide with all new test types (fuzz, property, visual, accessibility) | `docs/testing/TESTING.md` |
| 8.6.2 | Write fuzz testing guide with examples | `docs/testing/FUZZ_TESTING_GUIDE.md` (update) |
| 8.6.3 | Write property-based testing guide | `docs/testing/PROPERTY_TESTING_GUIDE.md` |
| 8.6.4 | Write visual regression testing guide | `docs/testing/VISUAL_REGRESSION_GUIDE.md` |
| 8.6.5 | Write accessibility testing guide | `docs/testing/ACCESSIBILITY_TESTING_GUIDE.md` |
| 8.6.6 | Update test coverage report with final numbers | `docs/testing/TESTING_REPORT.md` |
| 8.6.7 | Update challenge map with CH-061 to CH-088, MOD-016 to MOD-021 | `docs/testing/challenge-map.md` |
| 8.6.8 | Remove Android TODOs from test implementation summary | `docs/testing/TEST_IMPLEMENTATION_SUMMARY.md` |

#### 8.7 SQL & Schema Documentation

| Step | Task | File |
|------|------|------|
| 8.7.1 | Update complete SQL schema with sync tables | `docs/architecture/SQL_COMPLETE_SCHEMA.md` |
| 8.7.2 | Update ER diagram with sync relationships | `docs/diagrams/ER_DIAGRAM.md` |
| 8.7.3 | Update database schema doc with new indexes from Phase 3 | `docs/architecture/DATABASE_SCHEMA.md` |

#### 8.8 Diagram Updates

| Step | Task | File |
|------|------|------|
| 8.8.1 | Update architecture diagram with sync service, search handler, browse handler | `docs/diagrams/ARCHITECTURE_DIAGRAM.md` |
| 8.8.2 | Update component diagram with lazy loading boundaries | `docs/diagrams/COMPONENT_DIAGRAM.md` |
| 8.8.3 | Add sequence diagram for sync flow | `docs/diagrams/SEQUENCE_DIAGRAMS.md` |
| 8.8.4 | Add sequence diagram for security header flow | `docs/diagrams/SEQUENCE_DIAGRAMS.md` |
| 8.8.5 | Generate updated SVG diagrams for all modified markdown | `docs/diagrams/*.svg` |

---

### Phase 9: User Manuals, Video Courses & Content Expansion
**Priority:** MEDIUM
**Dependencies:** Phase 8 complete

#### 9.1 User Manual Updates

| Step | Task | File |
|------|------|------|
| 9.1.1 | Update USER_GUIDE.md with search feature documentation | `docs/USER_GUIDE.md` |
| 9.1.2 | Update USER_GUIDE.md with browse feature documentation | `docs/USER_GUIDE.md` |
| 9.1.3 | Update USER_GUIDE.md with cloud sync setup and usage | `docs/USER_GUIDE.md` |
| 9.1.4 | Update ADMIN_GUIDE.md with security headers configuration | `docs/ADMIN_GUIDE.md` |
| 9.1.5 | Update ADMIN_GUIDE.md with monitoring/alerting setup | `docs/ADMIN_GUIDE.md` |
| 9.1.6 | Update DEVELOPER_GUIDE.md with lazy loading patterns | `docs/DEVELOPER_GUIDE.md` |
| 9.1.7 | Update DEVELOPER_GUIDE.md with semaphore usage | `docs/DEVELOPER_GUIDE.md` |
| 9.1.8 | Update DEVELOPER_GUIDE.md with new test types | `docs/DEVELOPER_GUIDE.md` |
| 9.1.9 | Update INSTALLATION_GUIDE.md with security scanning setup | `docs/INSTALLATION_GUIDE.md` |
| 9.1.10 | Update DEPLOYMENT_GUIDE.md with monitoring stack deployment | `docs/DEPLOYMENT_GUIDE.md` |
| 9.1.11 | Update CONFIGURATION_GUIDE.md with sync, search, browse config options | `docs/CONFIGURATION_GUIDE.md` |
| 9.1.12 | Update TROUBLESHOOTING_GUIDE.md with monitoring troubleshooting | `docs/TROUBLESHOOTING_GUIDE.md` |

#### 9.2 Video Course Extension (Modules 9-12)

| Step | Task | File |
|------|------|------|
| 9.2.1 | Extend Module 9 script: Security Hardening & Best Practices (add security headers, CORS, input validation, scanning with Snyk/SonarQube/Trivy) | `docs/video-course/MODULE_9_*.md` |
| 9.2.2 | Extend Module 10 script: Performance Optimization (add lazy loading, semaphores, non-blocking patterns, React.lazy, virtualized scrolling) | `docs/video-course/MODULE_10_*.md` |
| 9.2.3 | Extend Module 11 script: Monitoring & Observability (add Prometheus metrics reference, Grafana dashboard customization, alerting rules, log aggregation) | `docs/video-course/MODULE_11_*.md` |
| 9.2.4 | Extend Module 12 script: Advanced Testing (add fuzz testing, property-based testing, visual regression, accessibility testing, chaos engineering) | `docs/video-course/MODULE_12_*.md` |
| 9.2.5 | Add Module 13 script: Cloud Sync & Search Features (search API, browse API, sync setup with S3/GCS/WebDAV, endpoint management) | New: `docs/video-course/MODULE_13_SYNC_SEARCH.md` |
| 9.2.6 | Add Module 14 script: Challenge System Deep Dive (writing challenges, running challenges, interpreting results, extending the framework) | New: `docs/video-course/MODULE_14_CHALLENGES.md` |

#### 9.3 Online Course Extension (Modules 1-8 Updates + Slides)

| Step | Task | File |
|------|------|------|
| 9.3.1 | Update Module 1 slides with current architecture (new services wired) | `docs/courses/slides/MODULE_1_SLIDES.md` |
| 9.3.2 | Update Module 5 slides with security hardening content | `docs/courses/slides/MODULE_5_SLIDES.md` |
| 9.3.3 | Update Module 6 slides with new test types | `docs/courses/slides/MODULE_6_SLIDES.md` |
| 9.3.4 | Update Module 7 slides with monitoring/metrics content | `docs/courses/slides/MODULE_7_SLIDES.md` |
| 9.3.5 | Update Module 8 slides with sync/search deployment | `docs/courses/slides/MODULE_8_SLIDES.md` |
| 9.3.6 | Update exercises with new features (search, sync, monitoring) | `docs/courses/EXERCISES.md` |
| 9.3.7 | Update assessment criteria with security and performance topics | `docs/courses/ASSESSMENT.md` |

#### 9.4 Platform-Specific Guide Updates

| Step | Task | File |
|------|------|------|
| 9.4.1 | Update Android guide with cloud sync features | `docs/guides/ANDROID_GUIDE.md` |
| 9.4.2 | Update Android TV guide with search/browse features | `docs/guides/ANDROID_TV_GUIDE.md` |
| 9.4.3 | Update Desktop guide with sync and search features | `docs/guides/DESKTOP_GUIDE.md` |
| 9.4.4 | Update Web app guide with new features | `docs/guides/WEB_APP_GUIDE.md` |

---

### Phase 10: Website Content & Final Integration
**Priority:** MEDIUM
**Dependencies:** Phase 9 complete

#### 10.1 Website Content Updates

| Step | Task | File |
|------|------|------|
| 10.1.1 | Update features page with search, browse, sync, monitoring features | `Website/features.md` |
| 10.1.2 | Update getting started page with current setup steps | `Website/getting-started.md` |
| 10.1.3 | Update FAQ with new feature questions | `Website/faq.md` |
| 10.1.4 | Update changelog with all Phase 1-9 changes | `Website/changelog.md` |
| 10.1.5 | Update download page with current version info | `Website/download.md` |
| 10.1.6 | Update course page with new modules (13-14) | `Website/course.md` |
| 10.1.7 | Add developer guide: API reference page | `Website/developer/api.md` (update) |
| 10.1.8 | Add developer guide: Testing page | `Website/developer/testing.md` (update) |
| 10.1.9 | Add developer guide: Contributing page | `Website/developer/contributing.md` (update) |
| 10.1.10 | Update security guide with scanning runbook | `Website/guides/security.md` |
| 10.1.11 | Update monitoring guide with Grafana screenshots | `Website/guides/monitoring.md` |
| 10.1.12 | Add configuration guide for sync feature | `Website/guides/configuration.md` |
| 10.1.13 | Update VitePress sidebar config with new pages | `Website/.vitepress/config.ts` |
| 10.1.14 | Build and verify website: `cd Website && npm run build` | Website build |

#### 10.2 OpenAPI Spec Update

| Step | Task | File |
|------|------|------|
| 10.2.1 | Add search endpoints to OpenAPI spec | `docs/api/openapi.yaml` |
| 10.2.2 | Add browse endpoints to OpenAPI spec | `docs/api/openapi.yaml` |
| 10.2.3 | Add sync endpoints to OpenAPI spec | `docs/api/openapi.yaml` |
| 10.2.4 | Verify OpenAPI spec validates: `npx @apidevtools/swagger-cli validate docs/api/openapi.yaml` | Validation |

#### 10.3 CHANGELOG & README Updates

| Step | Task | File |
|------|------|------|
| 10.3.1 | Update CHANGELOG.md with all Phase 1-10 changes | `docs/CHANGELOG.md` |
| 10.3.2 | Update README.md feature list with search, browse, sync, monitoring | `README.md` |
| 10.3.3 | Update CLAUDE.md with new endpoints, test types, documentation paths | `CLAUDE.md` |
| 10.3.4 | Update AGENTS.md with new services and constraints | `AGENTS.md` |

#### 10.4 Final Verification & Acceptance

| Step | Task |
|------|------|
| 10.4.1 | Run full Go test suite with race detection and coverage |
| 10.4.2 | Run full frontend test suite with coverage |
| 10.4.3 | Run all E2E tests |
| 10.4.4 | Run all security scans (SonarQube, Snyk, OWASP, Trivy, govulncheck) |
| 10.4.5 | Run all stress tests |
| 10.4.6 | Run all challenge tests |
| 10.4.7 | Verify all documentation links resolve |
| 10.4.8 | Build website and verify all pages |
| 10.4.9 | Build all 7 components with release pipeline |
| 10.4.10 | Generate final status report |

---

## 5. Test Coverage Strategy

### 5.1 Supported Test Types (Complete Inventory)

| # | Test Type | Framework | Location | Status |
|---|-----------|-----------|----------|--------|
| 1 | Go Unit Tests | `testing` | `*_test.go` beside source | Active (3,738 functions) |
| 2 | Go Table-Driven Tests | `testing` + subtests | Widespread | Active |
| 3 | Go Benchmark Tests | `testing.B` | `*_bench_test.go` | Active (74 functions) |
| 4 | Go Fuzz Tests | `testing.F` | `*_fuzz_test.go` | Active (4 files), Expanding to 15+ |
| 5 | Go Property-Based Tests | `testing/quick` or custom | `*_property_test.go` | **NEW** - Phase 4 |
| 6 | Go Integration Tests | `testing` + test DB | `tests/integration/` | Active (52 functions) |
| 7 | Go Stress Tests | `testing` + goroutines | `tests/stress/` | Active (31 functions) |
| 8 | Go Performance Tests | `testing` | `tests/performance/` | Active (3 + 57 bench) |
| 9 | Go Chaos Tests | `testing` + fault injection | `tests/integration/chaos_*.go` | Active (128 functions) |
| 10 | Go Contract Tests | `testing` + HTTP | `tests/integration/contract_test.go` | Active, Expanding |
| 11 | Go Security Tests | `testing` + attack patterns | `tests/security/` | Active (13), Expanding to 30+ |
| 12 | Go Monitoring Tests | `testing` + metrics | `tests/monitoring/` | Active (64), Expanding |
| 13 | Go Race Detection | `-race` flag | All test packages | Active |
| 14 | React Unit Tests | Vitest + Testing Library | `src/**/__tests__/` | Active (3,085 cases), Expanding |
| 15 | React Snapshot Tests | Vitest snapshots | `__snapshots__/` | Active (5), Expanding to 60+ |
| 16 | React E2E Tests | Playwright | `e2e/` | Active (25 files) |
| 17 | Visual Regression Tests | Playwright screenshots | `e2e/visual/` | **NEW** - Phase 4 |
| 18 | Accessibility Tests | axe-core + Playwright | `e2e/accessibility/` | **NEW** - Phase 4 |
| 19 | Kotlin Unit Tests | JUnit5 | `app/src/test/` | Active |
| 20 | Kotlin Instrumented Tests | AndroidJUnit | `app/src/androidTest/` | Minimal, Expanding |
| 21 | Challenge Tests | Challenge Framework | `challenges/` | Active (239), Expanding to 273+ |
| 22 | API Client Tests | Vitest | `catalogizer-api-client/` | Active (8 files) |
| 23 | Desktop Tests | Jest/Vitest | `catalogizer-desktop/` | Active (15 files) |
| 24 | Installer Tests | Jest/Vitest | `installer-wizard/` | Active (19 files) |

### 5.2 Coverage Targets

| Component | Current Estimate | Target | Method |
|-----------|-----------------|--------|--------|
| catalog-api (Go) | ~75% line | 95%+ line | Unit + integration + fuzz |
| catalog-web (React) | ~45% file, ~60% line | 90%+ file, 85%+ line | Unit + snapshot + E2E |
| catalogizer-desktop | ~70% | 85%+ | Unit + integration |
| installer-wizard | ~75% | 85%+ | Unit + integration |
| catalogizer-android | ~60% | 80%+ | Unit + instrumented |
| catalogizer-androidtv | ~55% | 80%+ | Unit + instrumented |
| catalogizer-api-client | ~80% | 95%+ | Unit |
| Lazy module | ~90% | 100% | Unit + fuzz + stress |
| Memory module | ~85% | 95%+ | Unit + fuzz + stress |
| Recovery module | ~90% | 100% | Unit + edge + stress |

---

## 6. Challenge Expansion Plan

### 6.1 Challenge Distribution After Expansion

| Category | Existing | New | Total |
|----------|----------|-----|-------|
| Original (CH-001 to CH-050) | 50 | 0 | 50 |
| New Features (CH-051 to CH-060) | 10 | 0 | 10 |
| New Features (CH-061 to CH-068) | 0 | 8 | 8 |
| Security (CH-069 to CH-075) | 0 | 7 | 7 |
| Performance (CH-076 to CH-083) | 0 | 8 | 8 |
| Monitoring (CH-084 to CH-088) | 0 | 5 | 5 |
| Userflow API (UF-*) | 49 | 0 | 49 |
| Userflow Web (UF-*) | 59 | 0 | 59 |
| Userflow Desktop (UF-*) | 28 | 0 | 28 |
| Userflow Mobile (UF-*) | 38 | 0 | 38 |
| Module (MOD-001 to MOD-015) | 15 | 0 | 15 |
| Module (MOD-016 to MOD-021) | 0 | 6 | 6 |
| **TOTAL** | **249** | **34** | **283** |

Note: 10 challenges (CH-051 to CH-060) are in the git status as new files, bringing the tracked total from 239 to 249 existing + 34 new = 283.

---

## 7. Documentation Completion Matrix

### 7.1 Documents to Create (New)

| # | Document | Path | Est. Lines |
|---|----------|------|-----------|
| 1 | Search API reference | `docs/api/SEARCH_API.md` | 400 |
| 2 | Browse API reference | `docs/api/BROWSE_API.md` | 300 |
| 3 | Sync API reference | `docs/api/SYNC_API.md` | 600 |
| 4 | Subtitle feature guide | `docs/guides/SUBTITLE_GUIDE.md` | 500 |
| 5 | Recommendation engine arch | `docs/architecture/RECOMMENDATION_ENGINE.md` | 400 |
| 6 | Advanced search guide | `docs/guides/ADVANCED_SEARCH.md` | 350 |
| 7 | Analytics guide | `docs/guides/ANALYTICS_GUIDE.md` | 400 |
| 8 | Caching strategy | `docs/architecture/CACHING_STRATEGY.md` | 500 |
| 9 | Query optimization | `docs/architecture/QUERY_OPTIMIZATION.md` | 400 |
| 10 | Large collections guide | `docs/guides/LARGE_COLLECTIONS.md` | 350 |
| 11 | Security headers doc | `docs/security/SECURITY_HEADERS.md` | 300 |
| 12 | CORS configuration | `docs/security/CORS_CONFIGURATION.md` | 250 |
| 13 | Secrets management | `docs/security/SECRETS_MANAGEMENT.md` | 400 |
| 14 | OAuth/OIDC guide | `docs/guides/OAUTH_INTEGRATION.md` | 500 |
| 15 | Security scanning runbook | `docs/security/SECURITY_SCANNING_RUNBOOK.md` | 600 |
| 16 | Prometheus metrics ref | `docs/guides/PROMETHEUS_METRICS_REFERENCE.md` | 500 |
| 17 | Alerting rules | `docs/guides/ALERTING_RULES.md` | 400 |
| 18 | Grafana dashboard guide | `docs/guides/GRAFANA_DASHBOARD_GUIDE.md` | 350 |
| 19 | Log aggregation strategy | `docs/architecture/LOG_AGGREGATION.md` | 300 |
| 20 | Lazy loading architecture | `docs/architecture/LAZY_LOADING.md` | 400 |
| 21 | Concurrency control guide | `docs/architecture/CONCURRENCY_CONTROL.md` | 500 |
| 22 | Performance tuning | `docs/guides/PERFORMANCE_TUNING.md` | 500 |
| 23 | Stress test results | `docs/testing/STRESS_TEST_RESULTS.md` | 300 |
| 24 | Property testing guide | `docs/testing/PROPERTY_TESTING_GUIDE.md` | 350 |
| 25 | Visual regression guide | `docs/testing/VISUAL_REGRESSION_GUIDE.md` | 300 |
| 26 | Accessibility testing guide | `docs/testing/ACCESSIBILITY_TESTING_GUIDE.md` | 350 |
| 27 | Lazy module API ref | `Lazy/docs/API_REFERENCE.md` | 300 |
| 28 | Lazy module changelog | `Lazy/docs/CHANGELOG.md` | 100 |
| 29 | Recovery module API ref | `Recovery/docs/API_REFERENCE.md` | 400 |
| 30 | Recovery module changelog | `Recovery/docs/CHANGELOG.md` | 100 |
| 31 | Video course Module 13 | `docs/video-course/MODULE_13_SYNC_SEARCH.md` | 800 |
| 32 | Video course Module 14 | `docs/video-course/MODULE_14_CHALLENGES.md` | 800 |

### 7.2 Documents to Update (Existing)

| # | Document | Updates Needed |
|---|----------|---------------|
| 1 | `docs/USER_GUIDE.md` | Search, browse, sync features |
| 2 | `docs/ADMIN_GUIDE.md` | Security headers, monitoring |
| 3 | `docs/DEVELOPER_GUIDE.md` | Lazy loading, semaphores, new test types |
| 4 | `docs/INSTALLATION_GUIDE.md` | Security scanning setup |
| 5 | `docs/DEPLOYMENT_GUIDE.md` | Monitoring stack deployment |
| 6 | `docs/CONFIGURATION_GUIDE.md` | Sync, search, browse config |
| 7 | `docs/TROUBLESHOOTING_GUIDE.md` | Monitoring troubleshooting |
| 8 | `docs/CHANGELOG.md` | All Phase 1-10 changes |
| 9 | `docs/testing/TESTING.md` | New test types |
| 10 | `docs/testing/FUZZ_TESTING_GUIDE.md` | New fuzz targets |
| 11 | `docs/testing/challenge-map.md` | CH-061 to CH-088, MOD-016 to MOD-021 |
| 12 | `docs/testing/TEST_IMPLEMENTATION_SUMMARY.md` | Remove Android TODOs |
| 13 | `docs/architecture/SQL_COMPLETE_SCHEMA.md` | Sync tables |
| 14 | `docs/architecture/DATABASE_SCHEMA.md` | New indexes |
| 15 | `docs/diagrams/ARCHITECTURE_DIAGRAM.md` | New services |
| 16 | `docs/diagrams/COMPONENT_DIAGRAM.md` | Lazy boundaries |
| 17 | `docs/diagrams/SEQUENCE_DIAGRAMS.md` | Sync, security flows |
| 18 | `docs/diagrams/ER_DIAGRAM.md` | Sync relationships |
| 19 | `docs/deployment/PRODUCTION_DEPLOYMENT_GUIDE.md` | Remove Prometheus TODO |
| 20 | `docs/api/openapi.yaml` | Search, browse, sync endpoints |
| 21 | `docs/api/API_DOCUMENTATION.md` | New endpoints |
| 22 | `docs/security/COMPREHENSIVE_SECURITY_AUDIT.md` | Remediation results |
| 23 | `docs/testing/TESTING_REPORT.md` | Final coverage numbers |
| 24-31 | `docs/courses/slides/MODULE_*_SLIDES.md` | Updated content per module |
| 32-35 | `docs/video-course/MODULE_9-12_*.md` | Extended content |
| 36-39 | `docs/guides/ANDROID*.md`, `DESKTOP*.md`, `WEB*.md` | New features |
| 40 | `README.md` | Feature list update |
| 41 | `CLAUDE.md` | New endpoints, test types, docs |
| 42 | `AGENTS.md` | New services, constraints |
| 43-55 | `Website/*.md` | All website content pages |

---

## 8. Security Remediation Plan

### 8.1 Remediation Priority Matrix

| Priority | Findings | Phase | Action |
|----------|----------|-------|--------|
| P0 (Immediate) | SEC-C01, SEC-C02, SEC-C03 | Phase 1 | Fix before any deployment |
| P1 (High) | SEC-H01 through SEC-H08 | Phase 1 | Fix in same session |
| P2 (Medium) | SEC-M01 through SEC-M09, SEC-L04 | Phase 1-2 | Fix during security phase |
| P3 (Low) | SEC-L01 through SEC-L08 | Phase 6 | Fix during scanning phase |

### 8.2 Scan Schedule

| Tool | When | Report Location |
|------|------|-----------------|
| govulncheck | Phase 1 (pre), Phase 6 (post) | Terminal output |
| npm audit | Phase 1 (pre), Phase 6 (post) | Terminal output |
| SonarQube | Phase 6 | `reports/sonarqube-report.json` |
| Snyk (4 scan types) | Phase 6 | `reports/snyk-*.json` |
| OWASP Dependency Check | Phase 6 | `reports/dependency-check/` |
| Trivy | Phase 6 | `reports/trivy-results.json` |

---

## 9. Performance Optimization Plan

### 9.1 Optimization Targets

| Metric | Current (Estimated) | Target | Method |
|--------|---------------------|--------|--------|
| Server startup time | ~2-3 seconds | <1 second | Lazy loading (Phase 3) |
| Idle memory usage | ~50-80 MB | <30 MB | Lazy init + deferred connections |
| Simple API response (p50) | ~5-15 ms | <5 ms | Connection pooling, caching |
| Simple API response (p99) | ~50-100 ms | <25 ms | Semaphores, non-blocking |
| Concurrent request capacity | ~200-500 | 1000+ | Semaphores, connection management |
| WebSocket broadcast latency | ~10-50 ms | <5 ms | Lock-free broadcast |
| Frontend initial load | ~2-3 seconds | <1 second | React.lazy, code splitting |
| Frontend LCP | ~1.5-2.5 seconds | <1 second | Lazy images, virtual scroll |
| Graceful shutdown time | Unknown | <30 seconds | Context propagation, WaitGroup timeout |

### 9.2 Monitoring Dashboard Expansion

| Dashboard | Panels | Purpose |
|-----------|--------|---------|
| `catalogizer-overview` (existing) | 8 | HTTP, WebSocket, memory, goroutines |
| `catalogizer-database` (new) | 6 | Query duration, connection pool, transactions |
| `catalogizer-scanning` (new) | 6 | Scan rate, queue depth, aggregation time |
| `catalogizer-security` (new) | 4 | Auth attempts, rate limit hits, blocked requests |
| `catalogizer-performance` (new) | 8 | Latency percentiles, throughput, cache hit rate |

---

## 10. Monitoring & Metrics Plan

### 10.1 New Custom Metrics to Add

| Metric Name | Type | Labels | Purpose |
|-------------|------|--------|---------|
| `catalogizer_scan_duration_seconds` | histogram | `protocol`, `status` | Scan timing |
| `catalogizer_scan_files_total` | counter | `protocol`, `media_type` | File counts |
| `catalogizer_scan_queue_depth` | gauge | none | Queue backpressure |
| `catalogizer_cache_hits_total` | counter | `cache_type` | Cache effectiveness |
| `catalogizer_cache_misses_total` | counter | `cache_type` | Cache misses |
| `catalogizer_auth_attempts_total` | counter | `result` (success/fail) | Auth monitoring |
| `catalogizer_rate_limit_hits_total` | counter | `endpoint` | Rate limit monitoring |
| `catalogizer_lazy_init_duration_seconds` | histogram | `service` | Lazy load timing |
| `catalogizer_semaphore_wait_seconds` | histogram | `resource` | Semaphore contention |
| `catalogizer_aggregation_duration_seconds` | histogram | `storage_root` | Aggregation timing |
| `catalogizer_sync_operations_total` | counter | `provider`, `status` | Sync monitoring |
| `catalogizer_conversion_duration_seconds` | histogram | `format` | Conversion timing |

### 10.2 Alerting Rules

| Alert | Condition | Severity |
|-------|-----------|----------|
| HighErrorRate | `rate(http_requests_total{status=~"5.."}[5m]) > 0.05` | Critical |
| HighLatency | `histogram_quantile(0.99, http_request_duration_seconds_bucket) > 1` | Warning |
| GoroutineLeak | `go_goroutines > 1000` | Warning |
| MemoryHigh | `go_memstats_alloc_bytes > 500000000` (500MB) | Warning |
| DatabaseSlow | `histogram_quantile(0.95, db_query_duration_seconds_bucket) > 0.5` | Warning |
| ScanQueueFull | `catalogizer_scan_queue_depth > 900` | Warning |
| AuthBruteForce | `rate(catalogizer_auth_attempts_total{result="fail"}[5m]) > 10` | Critical |

---

## 11. Content Expansion Plan

### 11.1 Website Pages Summary

| Page | Status | Updates |
|------|--------|---------|
| `index.md` | Exists | Update hero, stats, feature highlights |
| `features.md` | Exists | Add search, sync, monitoring, security features |
| `download.md` | Exists | Update version, checksums |
| `faq.md` | Exists | Add search, sync, monitoring FAQs |
| `support.md` | Exists | No changes needed |
| `documentation.md` | Exists | Add links to new docs |
| `changelog.md` | Exists | Add all new changes |
| `getting-started.md` | Exists | Update with current steps |
| `course.md` | Exists | Add Modules 13-14 |
| `guides/web-app.md` | Exists | Add search, sync features |
| `guides/desktop.md` | Exists | Add sync features |
| `guides/android.md` | Exists | Add sync features |
| `guides/android-tv.md` | Exists | Add search features |
| `guides/configuration.md` | Exists | Add sync, search config |
| `guides/security.md` | Exists | Add scanning runbook |
| `guides/monitoring.md` | Exists | Add dashboard screenshots, alerts |
| `developer/architecture.md` | Exists | Add lazy loading, concurrency |
| `developer/api.md` | Exists | Add search, sync endpoints |
| `developer/testing.md` | Exists | Add new test types |
| `developer/contributing.md` | Exists | Update conventions |

### 11.2 Video Course Module Summary

| Module | Status | Content |
|--------|--------|---------|
| 1-8 | Exists (scripts) | Slides to update for Modules 1, 5-8 |
| 9 | Exists (script) | Extend: security headers, CORS, scanning |
| 10 | Exists (script) | Extend: lazy loading, semaphores |
| 11 | Exists (script) | Extend: Prometheus ref, Grafana, alerts |
| 12 | Exists (script) | Extend: fuzz, property, visual, a11y |
| 13 | **NEW** | Cloud sync, search, browse features |
| 14 | **NEW** | Challenge system deep dive |

---

## 12. Verification & Acceptance Criteria

### 12.1 Phase Completion Gates

Each phase must pass these gates before proceeding:

| Gate | Criteria |
|------|----------|
| Build | All 7 components build without errors |
| Tests | All existing tests pass (zero failures) |
| Race | `go test -race` reports zero races |
| Vet | `go vet ./...` reports zero warnings |
| Lint | ESLint and Prettier pass for all TS/JS |
| Security | No new Critical/High findings introduced |
| Coverage | Coverage does not decrease from previous phase |

### 12.2 Final Acceptance Criteria

| # | Criterion | Verification Command |
|---|-----------|---------------------|
| 1 | Zero unconditional test skips | `grep -r "t.Skip(" --include="*_test.go" \| grep -v "testing.Short\|isReachable\|os.Getenv"` returns empty |
| 2 | Zero dead code | All handlers wired, all services instantiated, no orphaned packages |
| 3 | Zero security critical/high findings | All 4 security scanners report clean |
| 4 | Zero race conditions | `go test -race ./...` passes |
| 5 | Zero goroutine leaks | All goroutines tracked with WaitGroup, graceful shutdown verified |
| 6 | Go test coverage >= 95% | `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` |
| 7 | Frontend test coverage >= 90% | `npm run test:coverage` |
| 8 | All 283 challenges registered | `go test -run TestChallengeRegistration` |
| 9 | All 24 test types operational | Each test type has passing examples |
| 10 | All documentation TODOs resolved | `grep -r "TODO" docs/ --include="*.md"` returns only historical/intentional |
| 11 | All 32 new documents created | File existence check |
| 12 | All 55 existing documents updated | Git diff shows modifications |
| 13 | Website builds without errors | `cd Website && npm run build` |
| 14 | OpenAPI spec validates | `npx swagger-cli validate docs/api/openapi.yaml` |
| 15 | All diagrams regenerated | SVG files updated |
| 16 | Video course modules 13-14 written | File existence and content check |
| 17 | All course slides updated | Git diff shows modifications |
| 18 | Startup time < 1 second | Timed measurement with lazy loading |
| 19 | p99 latency < 25ms under load | Stress test measurement |
| 20 | Graceful shutdown < 30 seconds | Shutdown timing test |

### 12.3 Non-Regression Checklist

Before committing any phase:

- [ ] `GOMAXPROCS=3 go test ./... -p 2 -parallel 2` passes
- [ ] `cd catalog-web && npm test` passes
- [ ] `cd catalogizer-desktop && npm test` passes
- [ ] `cd installer-wizard && npm test` passes
- [ ] `cd catalogizer-api-client && npm test` passes
- [ ] `go vet ./...` clean
- [ ] `govulncheck ./...` clean
- [ ] No new browser console errors (zero warning policy)
- [ ] Resource usage within 30-40% host limits

---

## Appendix A: File Index

### New Files to Create (~45 test files + ~32 doc files + ~5 code files)

**New Code Files:**
1. `catalog-api/middleware/security_headers.go`
2. `catalog-api/middleware/concurrency_limiter.go`
3. `catalog-api/middleware/timeout.go`
4. `monitoring/grafana/dashboards/catalogizer-database.json`
5. `monitoring/grafana/dashboards/catalogizer-scanning.json`
6. `monitoring/grafana/dashboards/catalogizer-security.json`
7. `monitoring/grafana/dashboards/catalogizer-performance.json`
8. `monitoring/prometheus/alert_rules.yml`

**New Test Files (Go):**
- 11 fuzz test files (`*_fuzz_test.go`)
- 8 property test files (`*_property_test.go`)
- 8 benchmark test files (`*_bench_test.go`)
- 10 monitoring test files
- 6 chaos test files
- 10 security test files
- 10 integration test files
- 3 stress test files
- 7 submodule test files

**New Test Files (TypeScript):**
- ~58 component test files
- 7 visual regression test files
- 4 accessibility test files

**New Documentation Files:** 32 (see Section 7.1)

### Files to Delete

1. `catalog-api/smb/` (entire directory - orphaned duplicate)
2. `catalog-api/services/webdav_client.go` (duplicate)
3. `catalog-api/pkg/` (empty directory)
4. `catalog-api/old_results/` (stale artifacts)
5. `catalog-api/test-results/` (stale artifacts)

---

## Appendix B: Constraints & Rules

All work MUST comply with:

1. **CLAUDE.md** - All conventions, resource limits, container requirements
2. **AGENTS.md** - Agent coordination guidelines, code style
3. **Zero Warning Policy** - Zero console errors, zero failed requests
4. **Resource Limits** - 30-40% max host resources (GOMAXPROCS=3, -p 2, -parallel 2)
5. **Container Runtime** - Podman exclusively, `--network host` for builds
6. **HTTP/3 + Brotli** - All network communication
7. **No GitHub Actions** - All CI/CD local
8. **No Interactive Processes** - No sudo, no root password prompts
9. **Challenge Execution** - Via system deliverables only, never scripts/curl
10. **Git** - 6 push targets, `GIT_SSH_COMMAND="ssh -o BatchMode=yes"`
