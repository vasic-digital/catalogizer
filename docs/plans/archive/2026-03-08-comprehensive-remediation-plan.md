> **SUPERSEDED** — This document was a planning artifact from an earlier phase. The work described here has been completed or superseded by the [2026-03-26 Project Completion Plan](../../superpowers/plans/2026-03-26-project-completion-plan.md). Retained for historical reference.

---

# Comprehensive Remediation, Coverage, Documentation & Content Plan

**Date:** 2026-03-08
**Scope:** Full project audit, dead code removal, concurrency safety, security hardening, test coverage maximization, documentation completion, video course extension, website content, challenge expansion
**Status:** DESIGN DOCUMENT - Pending Approval

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Audit](#2-current-state-audit)
3. [Findings Registry](#3-findings-registry)
4. [Phase 1: Dead Code Cleanup & Security Hardening](#phase-1)
5. [Phase 2: Concurrency Safety & Memory Leak Fixes](#phase-2)
6. [Phase 3: Lazy Loading, Semaphores & Non-Blocking Optimization](#phase-3)
7. [Phase 4: Test Coverage Maximization](#phase-4)
8. [Phase 5: Stress, Integration & Monitoring Tests](#phase-5)
9. [Phase 6: Security Scanning Execution (SonarQube, Snyk, Trivy, OWASP)](#phase-6)
10. [Phase 7: Challenge Expansion](#phase-7)
11. [Phase 8: Documentation Completion](#phase-8)
12. [Phase 9: User Manuals, Video Courses & Content Extension](#phase-9)
13. [Phase 10: Website, OpenAPI & Final Validation](#phase-10)
14. [Test Type Inventory](#14-test-type-inventory)
15. [Verification & Acceptance Criteria](#15-verification--acceptance-criteria)

---

## 1. Executive Summary

### 1.1 Project Inventory

| Component | Language | Source Files | Test Files | Status |
|-----------|----------|-------------|-----------|--------|
| catalog-api | Go 1.24 | 247 | 242 | Operational |
| catalog-web | React 18/TS | 109 | 103 | Operational |
| catalogizer-desktop | Tauri/Rust+React | 21 | 15 | Operational |
| installer-wizard | Tauri/Rust+React | 24 | 19 | Operational |
| catalogizer-android | Kotlin/Compose | 28-35 | 51 | Operational (JDK 17 required) |
| catalogizer-androidtv | Kotlin/Compose | 52+ | 38 | Operational (JDK 17 required) |
| catalogizer-api-client | TypeScript | 7 | 7 | Operational |
| **32 Submodules** | Go/TS/Kotlin | ~1500+ | ~400+ | Operational |
| **Website** | VitePress | 22 pages | N/A | Built & deployed |

**Total registered challenges:** ~249 (50 CH + 174 UF + 15 MOD + 10 CH-051-060)
**Go modules wired:** 23 replace directives in `catalog-api/go.mod`
**Submodules:** 32 on `main` branch + Build/ + Upstreams/
**Documentation:** 287+ files across 18 categories (95/100 rating)

### 1.2 Comprehensive Audit Verdict

| Category | Findings | Critical Items |
|----------|---------|---------------|
| Dead Code & Stubs | 12 items | 4 orphaned files/dirs, 4 stub functions, 4 coverage boost tests |
| Concurrency Safety | 14 findings | 7 CRITICAL (goroutine leaks, deadlocks, race conditions) |
| Security | 28 findings | 3 CRITICAL, 8 HIGH, 9 MEDIUM, 8 LOW |
| Test Coverage Gaps | Significant | 9 Go files, 9 TS type files, 14 Android TV files untested |
| Documentation | 95% complete | 11 CLAUDE.md missing, 32 new docs to create |
| Infrastructure | Excellent | SonarQube, Snyk, OWASP, Trivy, Prometheus, Grafana configured |
| Static Analysis | Partial | Missing golangci-lint, ESLint security plugin |
| Container Security | Partial | Missing non-root USER directive, SBOM generation |
| Performance | Needs work | Limited lazy loading expansion, no stress baselines |

### 1.3 Scale of Work

| Dimension | Items |
|-----------|-------|
| Phases | 10 |
| Total tasks | 215+ |
| New test files to create | ~55 |
| New challenge definitions | 34 |
| Documentation files to create | 32 |
| Documentation files to update | 55 |
| Code files to fix/refactor | ~40 |
| Video course modules to add/extend | 6 |
| Website pages to create/update | 15 |

---

## 2. Current State Audit

### 2.1 Dead Code & Stubs Inventory

| ID | Type | Location | Lines | Severity | Status |
|----|------|----------|-------|----------|--------|
| DC-001 | Unused Handler | `handlers/search.go` | ~352 | ~~HIGH~~ | **RESOLVED** - Wired in main.go:500,588-590 |
| DC-002 | Unused Handler | `handlers/browse.go` | ~150 | ~~HIGH~~ | **RESOLVED** - Wired in main.go:501,823-827 |
| DC-003 | Unused Service | `services/sync_service.go` + SyncHandler | ~250 | ~~HIGH~~ | **RESOLVED** - Wired in main.go:506,833-843 |
| DC-004 | Duplicate WebDAV | `services/webdav_client.go` (duplicate of `filesystem/webdav_client.go`) | ~200 | HIGH | **OPEN** |
| DC-005 | Unused Repository | `repository/sync_repository.go` | ~150 | ~~HIGH~~ | **RESOLVED** - Used by SyncHandler |
| DC-006 | Orphaned Package | `catalog-api/smb/` (duplicate of `internal/smb/`) | ~400 | HIGH | **OPEN** |
| DC-007 | Empty Directory | `pkg/` | 0 | ~~LOW~~ | **RESOLVED** - Deleted |
| DC-008 | Stale Artifacts | `old_results/` | N/A | ~~LOW~~ | **RESOLVED** - Deleted |
| DC-009 | Stale Artifacts | `test-results/` | N/A | ~~LOW~~ | **RESOLVED** - Deleted |
| DC-010 | Coverage Boosters | 4 `coverage_boost*_test.go` files | ~800 | MEDIUM | **OPEN** |
| DC-011 | Manual Tests | `tests/manual/test_db.go`, `test_auth.go` | ~200 | LOW | **OPEN** |
| DC-012 | Email Stubs | `services/error_reporting_service.go:491-501` | 10 | MEDIUM | **OPEN** |
| DC-013 | SMB Resilience Placeholders | `internal/smb/resilience.go:382-400,481-498` | ~36 | MEDIUM | **OPEN** |
| DC-014 | Panoptic Analytics Stubs | `Challenges/Panoptic/internal/cloud/manager.go:1005-1027` | 22 | LOW | **OPEN** |
| DC-015 | iOS Recording Placeholder | `Challenges/Panoptic/internal/platforms/mobile.go:308` | 3 | LOW | **OPEN** |

**Summary:** 5 resolved, 7 open (2 HIGH, 3 MEDIUM, 2 LOW)

### 2.2 Concurrency Safety Issues

| ID | Type | File | Line(s) | Severity |
|----|------|------|---------|----------|
| CS-001 | Goroutine Leak | `internal/services/universal_scanner.go` | 222-233 | CRITICAL |
| CS-002 | Goroutine Leak | `internal/services/universal_scanner.go` | 289-296 | CRITICAL |
| CS-003 | Context Missing | `internal/media/realtime/watcher.go` | 53 | CRITICAL |
| CS-004 | Unbuffered stopCh | `internal/media/realtime/watcher.go` | 30-32 | CRITICAL |
| CS-005 | Race Condition | `internal/services/cache_service.go` | 679-702 | CRITICAL |
| CS-006 | Per-Request Goroutine | `middleware/advanced_rate_limiter.go` | 120-131 | CRITICAL |
| CS-007 | Broadcast Deadlock | `handlers/websocket_handler.go` | 106-111 | CRITICAL |
| CS-008 | Concurrent Map | `internal/services/cache_service.go` | 595-599 | HIGH |
| CS-009 | Resource Leak | `internal/services/aggregation_service.go` | 125-145 | HIGH |
| CS-010 | Timer Leak | `internal/media/realtime/watcher.go` | 214-256 | HIGH |
| CS-011 | CacheService closeMu | `internal/services/cache_service.go` | 107-123 | MEDIUM |
| CS-012 | Lazy Reset() Race | `Lazy/pkg/lazy/lazy.go` | 48-50 | MEDIUM |
| CS-013 | React Timer Leak | `catalog-web: UploadManager.tsx` | 36-42 | MEDIUM |
| CS-014 | React WebSocket Leak | `catalog-web: WebSocketContext.tsx` | 32-45 | MEDIUM |

### 2.3 Security Findings

#### CRITICAL (3)

| ID | Finding | Location |
|----|---------|----------|
| SEC-C01 | Default weak JWT secret in `.env` template | `catalog-api/.env.example` |
| SEC-C02 | Default weak admin password in `.env` template | `catalog-api/.env.example` |
| SEC-C03 | MD5 usage for security hashing | **VERIFIED CLEAN** - No `md5.` calls in production code |

#### HIGH (8)

| ID | Finding | Location | Verified |
|----|---------|----------|----------|
| SEC-H01 | Dynamic SQL via `fmt.Sprintf` | `internal/auth/service.go:520` | Column name interpolation |
| SEC-H02 | Dynamic SQL in 4 more locations | `database.go:165,233`, `cache_service.go:659`, `favorites_repo:257` | Table/clause interpolation |
| SEC-H03 | Weak CORS defaults | Various handler files | Needs origin validation |
| SEC-H04 | Missing security headers middleware | `main.go` | Middleware exists but verify wiring |
| SEC-H05 | Missing input validation on all endpoints | `middleware/input_validation.go` | Partial coverage |
| SEC-H06 | Self-signed TLS without cert caching | `main.go` | Regenerated on restart |
| SEC-H07 | Missing golangci-lint config | Project root | No `.golangci.yml` |
| SEC-H08 | Missing ESLint security plugin | `catalog-web/.eslintrc.js` | No `eslint-plugin-security` |

#### MEDIUM (9)

| ID | Finding | Location |
|----|---------|----------|
| SEC-M01 | `.env` in version control history | Git history (now gitignored) |
| SEC-M02 | Sensitive data in error messages | Multiple handlers |
| SEC-M03 | Auth gaps on some endpoints | Various |
| SEC-M04 | Password change without current password | `internal/auth/service.go` |
| SEC-M05 | Session management weaknesses | `internal/auth/service.go` |
| SEC-M06 | Missing auth rate limiting on login | Auth endpoints |
| SEC-M07 | Frontend XSS protection gaps | `catalog-web/src/` |
| SEC-M08 | No non-root USER in Dockerfiles | `catalog-api/Dockerfile`, `catalog-web/Dockerfile` |
| SEC-M09 | No SBOM generation | Build pipeline |

#### LOW (8)

| ID | Finding | Location |
|----|---------|----------|
| SEC-L01 | HTTPS not enforced in dev | Development config |
| SEC-L02 | Default NULL in migrations | Database migrations |
| SEC-L03 | Content-Type via extension only | Upload handlers |
| SEC-L04 | `math/rand` in non-crypto contexts | Verified safe (v2, with comments) |
| SEC-L05 | No CSRF tokens | Form handlers |
| SEC-L06 | Debug logs may contain secrets | Logging system |
| SEC-L07 | Dependency vulnerabilities (requires scan) | `go.mod`, `package.json` |
| SEC-L08 | No API versioning deprecation strategy | `/api/v1/` routes |

### 2.4 Dynamic SQL Analysis (Verified)

| File | Line | Pattern | Risk | Fix |
|------|------|---------|------|-----|
| `internal/auth/service.go` | 520 | `fmt.Sprintf("UPDATE users SET %s", setParts)` | Moderate | Column names from code, not user input |
| `internal/media/database/database.go` | 165 | `fmt.Sprintf("CREATE TABLE backup.%s AS SELECT * FROM main.%s", table, table)` | Low | Table names from internal code |
| `internal/media/database/database.go` | 233 | `fmt.Sprintf("SELECT COUNT(*) FROM %s", table)` | Low | Table names from internal code |
| `internal/services/cache_service.go` | 659 | `fmt.Sprintf("DELETE FROM %s WHERE expires_at <= CURRENT_TIMESTAMP", table)` | Low | Table names from internal code |
| `repository/favorites_repository.go` | 257 | `fmt.Sprintf("SELECT COUNT(*) FROM favorites %s", whereClause)` | Moderate | Where clause from builder |

### 2.5 Test Coverage Gaps

#### Go Source Files WITHOUT Test Files (9 critical)

| File | Category |
|------|----------|
| `handlers/challenge.go` | Handler |
| `handlers/sync_handler.go` | Handler |
| `middleware/concurrency_limiter.go` | Middleware |
| `middleware/security_headers.go` | Middleware |
| `middleware/timeout.go` | Middleware |
| `repository/media_collection_repository.go` | Repository |
| `filesystem/nfs_client_darwin.go` | Platform-specific |
| `filesystem/nfs_client_windows.go` | Platform-specific |
| `internal/media/realtime/watcher.go` | Internal service |

#### React Type Files WITHOUT Tests (9 files)

| File |
|------|
| `src/types/admin.ts` |
| `src/types/auth.ts` |
| `src/types/collections.ts` |
| `src/types/collection.ts` |
| `src/types/conversion.ts` |
| `src/types/dashboard.ts` |
| `src/types/favorites.ts` |
| `src/types/media.ts` |
| `src/types/subtitles.ts` |

#### Android TV Files WITHOUT Tests (14 files)

| File |
|------|
| `data/media/MediaPlaybackService.kt` |
| `data/remote/CatalogizerApi.kt` |
| `data/tv/CatalogizerTvProvider.kt` |
| `data/tv/CatalogizerTvProviderImpl.kt` |
| `ui/components/MediaCard.kt` |
| `ui/components/MediaCarousel.kt` |
| `ui/components/TopBar.kt` |
| `ui/MainActivity.kt` |
| `ui/navigation/TVNavigation.kt` |
| `ui/player/MediaPlayerActivity.kt` |
| `ui/screens/media/MediaDetailScreen.kt` |
| `ui/screens/player/MediaPlayerScreen.kt` |
| `ui/theme/Theme.kt` |
| `ui/theme/Type.kt` |

#### Android Files WITHOUT Tests (6 files)

| File |
|------|
| `data/local/CatalogizerDatabase.kt` |
| `data/models/Auth.kt` |
| `data/remote/CatalogizerApi.kt` |
| `ui/MainActivity.kt` |
| `ui/navigation/CatalogizerNavigation.kt` |
| `ui/theme/Theme.kt` |

#### Missing Test Types (Project-Wide)

| Test Type | Current | Target | Gap |
|-----------|---------|--------|-----|
| Go Fuzz Tests | 4 files | 15+ files | 11 new files |
| Property-Based Tests | 0 | 10+ files | 10 new files |
| Visual Regression | 0 | 7+ specs | 7 new specs |
| Accessibility Tests | 0 | 4+ specs | 4 new specs |
| Contract/OpenAPI Tests | 2 partial | Full API | Rewrite needed |
| Snapshot Tests | 1 file (5 asserts) | 30+ assertions | Extension |
| Go Benchmarks (expanded) | 7 files | 15+ files | 8 new files |
| Android Instrumented | 6 files | 15+ files | 9 new files |

### 2.6 Documentation Gaps

| Area | Current | Target | Gap |
|------|---------|--------|-----|
| CLAUDE.md files | 27/38 modules | 38/38 | 11 missing |
| Submodule API refs | 1/3 new modules | 3/3 | Lazy, Recovery missing |
| Video course modules | 12 | 14 | Modules 13-14 |
| New feature docs | 0 | 10 | Search, Browse, Sync, Subtitle, etc. |
| Security docs | Partial | Complete | 6 new documents |
| Monitoring docs | Partial | Complete | 5 new documents |
| Performance docs | Partial | Complete | 4 new documents |
| Test type guides | Partial | Complete | 4 new guides |
| Website pages | 9 | 15+ | 6+ new/updated pages |

### 2.7 Infrastructure Status

| Tool | Compose Config | Scripts | Status |
|------|---------------|---------|--------|
| SonarQube | `docker-compose.security.yml` | `run-security-scan.sh`, `setup-security-scanning.sh` | Ready (needs token config) |
| Snyk | `docker-compose.security.yml` | `run-security-scan.sh` | Ready (freemium mode) |
| OWASP Dependency Check | `docker-compose.security.yml` | `run-security-scan.sh` | Ready |
| Trivy | `docker-compose.security.yml` | `run-security-scan.sh` | Ready |
| govulncheck | N/A (CLI) | `quick-security-scan.sh` | Ready |
| npm audit | N/A (CLI) | `quick-security-scan.sh` | Ready |
| gosec | N/A (CLI) | `gosec-scan.sh` | Ready |
| nancy | N/A (CLI) | `nancy-scan.sh` | Ready |
| Prometheus | `docker-compose.yml` (monitoring profile) | N/A | Ready |
| Grafana | `docker-compose.yml` (monitoring profile) | N/A | Ready (1 dashboard) |
| Pre-commit hooks | `.pre-commit-config.yaml` | N/A | Ready (secret detection, go-fmt, eslint) |

---

## 3. Findings Registry

All findings tracked with unique IDs:

- **DC-001 to DC-015**: Dead Code findings (15 items, 5 resolved, 7 open)
- **CS-001 to CS-014**: Concurrency Safety findings (14 items)
- **SEC-C01 to SEC-C03**: Security Critical (3 items, 1 verified clean)
- **SEC-H01 to SEC-H08**: Security High (8 items)
- **SEC-M01 to SEC-M09**: Security Medium (9 items)
- **SEC-L01 to SEC-L08**: Security Low (8 items)

**Total tracked findings: 57 open items**

---

## Phase 1: Dead Code Cleanup & Security Hardening {#phase-1}

**Priority:** CRITICAL
**Dependencies:** None
**Constraint:** All changes must NOT break any existing working functionality

### 1.1 Delete Orphaned Code

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 1.1.1 | Delete duplicate `services/webdav_client.go` | DC-004 | Delete file |
| 1.1.2 | Delete orphaned `catalog-api/smb/` top-level package | DC-006 | Delete directory (4 files) |
| 1.1.3 | Replace 4 coverage_boost test files with meaningful tests | DC-010 | Rewrite `coverage_boost*_test.go` |
| 1.1.4 | Convert manual test files to proper Go tests or delete | DC-011 | `tests/manual/test_db.go`, `test_auth.go` |

### 1.2 Implement Stub Functions

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 1.2.1 | Implement real email notification (or log-based placeholder with clear interface) | DC-012 | `services/error_reporting_service.go:491-501` |
| 1.2.2 | Implement real SMB connection/health in resilience.go (replace simulation) | DC-013 | `internal/smb/resilience.go:382-400,481-498` |
| 1.2.3 | Document Panoptic analytics/iOS stubs as intentional feature gaps | DC-014, DC-015 | `Challenges/Panoptic/` |

### 1.3 Security Critical & High Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 1.3.1 | Generate 64-char crypto/rand JWT secret in `.env.example` | SEC-C01 | `.env.example` |
| 1.3.2 | Generate secure random admin password in `.env.example` | SEC-C02 | `.env.example` |
| 1.3.3 | Fix dynamic SQL in auth service - use parameterized column whitelist | SEC-H01 | `internal/auth/service.go:520` |
| 1.3.4 | Fix dynamic SQL in favorites repo - validate WHERE clause builder | SEC-H02 | `repository/favorites_repository.go:257` |
| 1.3.5 | Fix CORS: require explicit origin config, validate format | SEC-H03 | Handler files with CORS |
| 1.3.6 | Verify security headers middleware is wired on ALL routes | SEC-H04 | `main.go`, `middleware/security_headers.go` |
| 1.3.7 | Apply input validation middleware to ALL endpoints | SEC-H05 | `middleware/input_validation.go`, `main.go` |
| 1.3.8 | Cache TLS certificates across restarts | SEC-H06 | `main.go` |
| 1.3.9 | Create `.golangci.yml` with gosec, govet, staticcheck, gocritic | SEC-H07 | New: `.golangci.yml` |
| 1.3.10 | Add `eslint-plugin-security` to catalog-web ESLint config | SEC-H08 | `catalog-web/.eslintrc.js` |

### 1.4 Security Medium Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 1.4.1 | Require current password for password change | SEC-M04 | `internal/auth/service.go` |
| 1.4.2 | Add auth rate limiting on login/register endpoints | SEC-M06 | `main.go`, `middleware/` |
| 1.4.3 | Add non-root USER directive to Dockerfiles | SEC-M08 | `catalog-api/Dockerfile`, `catalog-web/Dockerfile` |
| 1.4.4 | Add DOMPurify or sanitize HTML rendering in React components | SEC-M07 | `catalog-web/src/` |

### 1.5 Verification

| Step | Task |
|------|------|
| 1.5.1 | `GOMAXPROCS=3 go test ./... -p 2 -parallel 2` - all must pass |
| 1.5.2 | `go vet ./...` - zero warnings |
| 1.5.3 | `cd catalog-web && npm test` - all must pass |
| 1.5.4 | `govulncheck ./...` - zero vulnerabilities |
| 1.5.5 | Verify deleted code has no remaining references |

---

## Phase 2: Concurrency Safety & Memory Leak Fixes {#phase-2}

**Priority:** HIGH
**Dependencies:** Phase 1 complete

### 2.1 CRITICAL: Goroutine Leak Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.1.1 | Replace 60s `time.After` with context-aware cancellation in deferred cleanup goroutine | CS-001 | `internal/services/universal_scanner.go:222-233` |
| 2.1.2 | Add WaitGroup tracking to post-scan aggregation goroutine | CS-002 | `internal/services/universal_scanner.go:289-296` |
| 2.1.3 | Propagate server context instead of `context.Background()` to watcher | CS-003 | `internal/media/realtime/watcher.go:53` |
| 2.1.4 | Make `stopCh` buffered (1) to match `changeQueue` signaling pattern | CS-004 | `internal/media/realtime/watcher.go:30-32` |

### 2.2 CRITICAL: Race Condition & Deadlock Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.2.1 | Add sync guard between `recordCacheActivity()` goroutine and `Close()` | CS-005 | `internal/services/cache_service.go:679-702` |
| 2.2.2 | Move rate limiter cleanup to single init-time goroutine (not per-request) | CS-006 | `middleware/advanced_rate_limiter.go:120-131` |
| 2.2.3 | Use copy-then-release pattern for WebSocket broadcast (release lock before write) | CS-007 | `handlers/websocket_handler.go:106-111` |

### 2.3 HIGH: Resource Leak Fixes

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.3.1 | Protect CacheStats map writes with sync.RWMutex | CS-008 | `internal/services/cache_service.go:595-599` |
| 2.3.2 | Ensure `sql.Rows` closed on ALL code paths in aggregation loop | CS-009 | `internal/services/aggregation_service.go:125-145` |
| 2.3.3 | Add max debounce map size with eviction; stop timers on shutdown | CS-010 | `internal/media/realtime/watcher.go:214-256` |

### 2.4 MEDIUM: Defensive Improvements

| Step | Task | Finding | Files |
|------|------|---------|-------|
| 2.4.1 | Use atomic.Bool or sync.Once for CacheService shutdown flag | CS-011 | `internal/services/cache_service.go:107-123` |
| 2.4.2 | Protect Lazy `Reset()` with sync.Mutex against concurrent `Get()` | CS-012 | `Lazy/pkg/lazy/lazy.go:48-50` |
| 2.4.3 | Add setTimeout cleanup in UploadManager useEffect return | CS-013 | `catalog-web/src/components/upload/UploadManager.tsx:36-42` |
| 2.4.4 | Ensure WebSocket hook reference is stable or cleanup listeners on reconnect | CS-014 | `catalog-web/src/contexts/WebSocketContext.tsx:32-45` |

### 2.5 Verification

| Step | Task |
|------|------|
| 2.5.1 | `GOMAXPROCS=3 go test -race ./... -p 2 -parallel 2` - zero races |
| 2.5.2 | Run stress tests: `go test -run TestStress ./tests/stress/ -timeout 10m` |
| 2.5.3 | `cd catalog-web && npm test` - all pass |
| 2.5.4 | Manual: start server, trigger scans, verify graceful shutdown completes within 30s |

---

## Phase 3: Lazy Loading, Semaphores & Non-Blocking Optimization {#phase-3}

**Priority:** HIGH
**Dependencies:** Phase 2 complete

### 3.1 Lazy Initialization Expansion

| Step | Task | Files |
|------|------|-------|
| 3.1.1 | Wrap service constructors in `main.go` with `lazy.Service[T]` from Lazy module | `main.go`, service constructors |
| 3.1.2 | Lazy-load TMDB/OMDB/IMDB provider clients (only when media recognition requested) | `internal/media/providers/*.go` |
| 3.1.3 | Lazy-load Redis connection (only when caching is used) | `internal/services/cache_service.go` |
| 3.1.4 | Lazy-load WebSocket hub (only when first WS connection arrives) | `handlers/websocket_handler.go` |
| 3.1.5 | Lazy-load filesystem protocol clients (FTP, SMB, NFS, WebDAV) per first access | `filesystem/factory.go` |
| 3.1.6 | Add lazy image loading (IntersectionObserver) for media thumbnails | `catalog-web/src/components/media/` |
| 3.1.7 | Verify React.lazy() used for ALL route-level pages (already 12+ done) | `catalog-web/src/App.tsx` |

### 3.2 Semaphore & Rate Control

| Step | Task | Files |
|------|------|-------|
| 3.2.1 | Add semaphore to file conversion pipeline (limit concurrent ffmpeg) | `services/conversion_service.go` |
| 3.2.2 | Add semaphore to media recognition pipeline (limit concurrent API calls) | `internal/media/detector/engine.go` |
| 3.2.3 | Add semaphore to WebSocket broadcast (limit concurrent client writes) | `handlers/websocket_handler.go` |
| 3.2.4 | Add semaphore to aggregation service (limit concurrent post-scan processing) | `internal/services/aggregation_service.go` |
| 3.2.5 | Implement global request concurrency cap middleware | New: `middleware/concurrency_limiter.go` (verify existing) |

### 3.3 Non-Blocking Patterns

| Step | Task | Files |
|------|------|-------|
| 3.3.1 | Use non-blocking channel sends with `select/default` for event bus publishing | `internal/eventbus/` |
| 3.3.2 | Implement async write-behind for analytics/access logging | `handlers/analytics_handler.go` |
| 3.3.3 | Add request timeout middleware with configurable per-route timeouts | `middleware/timeout.go` (verify existing) |
| 3.3.4 | Implement connection draining for graceful HTTP server shutdown | `main.go` |
| 3.3.5 | Tune React Query `staleTime`/`cacheTime` to reduce redundant API calls | `catalog-web/src/` |
| 3.3.6 | Add virtualized scrolling for large media lists (react-window/react-virtuoso) | `catalog-web/src/components/media/` |

### 3.4 Verification

| Step | Task |
|------|------|
| 3.4.1 | Full test suite - zero regressions |
| 3.4.2 | Measure startup time before/after lazy loading (target: 50% reduction) |
| 3.4.3 | Measure memory usage at idle before/after (target: 30% reduction) |
| 3.4.4 | Run stress tests to verify semaphores prevent overload |
| 3.4.5 | Verify graceful shutdown completes within 30 seconds under load |

---

## Phase 4: Test Coverage Maximization {#phase-4}

**Priority:** HIGH
**Dependencies:** Phases 1-3 complete (code changes stabilized)

### 4.1 Go Unit Test Expansion (9 missing files)

| Step | Task | Target |
|------|------|--------|
| 4.1.1 | Add tests for `handlers/challenge.go` | 95%+ |
| 4.1.2 | Add tests for `handlers/sync_handler.go` | 95%+ |
| 4.1.3 | Add tests for `middleware/concurrency_limiter.go` | 100% |
| 4.1.4 | Add tests for `middleware/security_headers.go` | 100% |
| 4.1.5 | Add tests for `middleware/timeout.go` | 100% |
| 4.1.6 | Add tests for `repository/media_collection_repository.go` | 95%+ |
| 4.1.7 | Add tests for `internal/media/realtime/watcher.go` | 90%+ |
| 4.1.8 | Add tests for `filesystem/nfs_client_darwin.go` (build-tagged) | 80%+ |
| 4.1.9 | Add tests for `filesystem/nfs_client_windows.go` (build-tagged) | 80%+ |

### 4.2 Go Fuzz Testing (11 new files)

| Step | File | Target |
|------|------|--------|
| 4.2.1 | `middleware/input_validation_fuzz_test.go` | SQL injection, XSS, path traversal |
| 4.2.2 | `internal/auth/jwt_fuzz_test.go` | JWT token parsing |
| 4.2.3 | `internal/services/title_parser_fuzz_test.go` (extend) | All 11 media types |
| 4.2.4 | `filesystem/path_fuzz_test.go` | URL/path parsing |
| 4.2.5 | `handlers/websocket_fuzz_test.go` | WebSocket message parsing |
| 4.2.6 | `handlers/search_fuzz_test.go` | Search query parsing |
| 4.2.7 | `config/config_fuzz_test.go` | Configuration parsing |
| 4.2.8 | `services/sync_service_fuzz_test.go` | Sync endpoint URL validation |
| 4.2.9 | `handlers/analytics_fuzz_test.go` | Analytics event parsing |
| 4.2.10 | `handlers/collection_fuzz_test.go` | Collection name/metadata |
| 4.2.11 | `internal/services/subtitle_fuzz_test.go` | Subtitle format parsing |

### 4.3 Go Property-Based Testing (8 new files)

| Step | File | Invariant |
|------|------|-----------|
| 4.3.1 | `database/dialect_property_test.go` | Roundtrip correctness |
| 4.3.2 | `filesystem/path_property_test.go` | Path normalization |
| 4.3.3 | `internal/media/detector/property_test.go` | Detection invariants |
| 4.3.4 | `internal/services/title_parser_property_test.go` | Parse/format roundtrip |
| 4.3.5 | `internal/auth/jwt_property_test.go` | Token gen/validation |
| 4.3.6 | `handlers/collection_property_test.go` | Sorting/filtering |
| 4.3.7 | `handlers/pagination_property_test.go` | Monotonicity, completeness |
| 4.3.8 | `middleware/rate_limiter_property_test.go` | Fairness, bounded latency |

### 4.4 Go Benchmark Expansion (8 new files)

| Step | File |
|------|------|
| 4.4.1 | `middleware/security_headers_bench_test.go` |
| 4.4.2 | `middleware/input_validation_bench_test.go` |
| 4.4.3 | `Lazy/pkg/lazy/lazy_bench_test.go` |
| 4.4.4 | `Recovery/pkg/breaker/breaker_bench_test.go` |
| 4.4.5 | `Memory/pkg/store/store_bench_test.go` |
| 4.4.6 | `Memory/pkg/graph/graph_bench_test.go` |
| 4.4.7 | `handlers/websocket_bench_test.go` |
| 4.4.8 | `services/sync_service_bench_test.go` |

### 4.5 React/TypeScript Test Expansion

| Step | Task | Target |
|------|------|--------|
| 4.5.1 | Add tests for 9 untested type definition files | 100% |
| 4.5.2 | Expand snapshot tests from 5 to 30+ assertions | All UI components |
| 4.5.3 | Add React Hook tests for custom hooks (verify existing coverage) | 100% |
| 4.5.4 | Add Zustand store tests (if stores exist, otherwise verify React Query coverage) | 100% |
| 4.5.5 | Add form validation tests (Zod schema coverage) | 100% |

### 4.6 Visual Regression Testing (Playwright)

| Step | File |
|------|------|
| 4.6.1 | `e2e/visual/login.spec.ts` |
| 4.6.2 | `e2e/visual/dashboard.spec.ts` |
| 4.6.3 | `e2e/visual/media-browser.spec.ts` |
| 4.6.4 | `e2e/visual/collections.spec.ts` |
| 4.6.5 | `e2e/visual/settings.spec.ts` |
| 4.6.6 | `e2e/visual/player.spec.ts` |
| 4.6.7 | `e2e/visual/responsive.spec.ts` (mobile, tablet, desktop) |

### 4.7 Accessibility Testing

| Step | File |
|------|------|
| 4.7.1 | Install `@axe-core/playwright` in catalog-web |
| 4.7.2 | `e2e/accessibility/pages.spec.ts` - WCAG 2.1 AA for all pages |
| 4.7.3 | `e2e/accessibility/keyboard.spec.ts` - Keyboard navigation |
| 4.7.4 | `e2e/accessibility/contrast.spec.ts` - Color contrast |

### 4.8 Contract/OpenAPI Testing

| Step | File |
|------|------|
| 4.8.1 | `tests/contract/openapi_validation_test.go` - Response schema validation for ALL endpoints |
| 4.8.2 | `tests/contract/request_validation_test.go` - Invalid payloads rejected |
| 4.8.3 | `tests/contract/backward_compat_test.go` - No breaking changes |

### 4.9 Submodule Test Expansion

| Step | File |
|------|------|
| 4.9.1 | `Lazy/pkg/lazy/lazy_edge_test.go` - Concurrent Reset, nil factory |
| 4.9.2 | `Lazy/pkg/lazy/lazy_stress_test.go` - 1000 concurrent getters |
| 4.9.3 | `Lazy/pkg/lazy/lazy_fuzz_test.go` - Factory functions |
| 4.9.4 | `Recovery/pkg/facade/facade_integration_test.go` - Breaker + health |
| 4.9.5 | `Recovery/pkg/breaker/breaker_stress_test.go` - State transitions |
| 4.9.6 | `Memory/pkg/entity/entity_fuzz_test.go` - Extraction patterns |
| 4.9.7 | `Memory/pkg/graph/graph_stress_test.go` - 10K nodes |

### 4.10 Android Test Expansion

| Step | Task | Files |
|------|------|-------|
| 4.10.1 | Add tests for 6 untested Android files | `CatalogizerDatabase.kt`, `Auth.kt`, etc. |
| 4.10.2 | Add tests for 14 untested Android TV files | `MediaPlaybackService.kt`, `CatalogizerTvProvider.kt`, etc. |
| 4.10.3 | Expand instrumented tests (Compose UI tests) | `app/src/androidTest/` |

### 4.11 Desktop & Installer Test Expansion

| Step | Task | Target |
|------|------|--------|
| 4.11.1 | Add 6 missing desktop test files (Tauri IPC) | 85%+ |
| 4.11.2 | Add 5 missing installer wizard test files | 85%+ |

### 4.12 Verification

| Step | Task |
|------|------|
| 4.12.1 | `go test -coverprofile=coverage.out ./...` - generate coverage report |
| 4.12.2 | Verify Go coverage >= 90% for all packages |
| 4.12.3 | `npm run test:coverage` - verify catalog-web coverage >= 85% |
| 4.12.4 | Run fuzz tests: `go test -fuzz=. -fuzztime=30s` for each new fuzz file |
| 4.12.5 | Run visual regression: `npm run test:e2e -- --grep visual` |
| 4.12.6 | Run accessibility: `npm run test:e2e -- --grep accessibility` |

---

## Phase 5: Stress, Integration & Monitoring Tests {#phase-5}

**Priority:** HIGH
**Dependencies:** Phase 4 complete

### 5.1 Stress Test Expansion

| Step | File | Scenario |
|------|------|----------|
| 5.1.1 | `tests/stress/api_load_test.go` (extend) | 1000 concurrent authenticated API requests |
| 5.1.2 | `tests/stress/websocket_stress_test.go` (extend) | 500 concurrent WS connections with broadcast |
| 5.1.3 | `tests/stress/multi_protocol_stress_test.go` (new) | 100 concurrent scans across 4 protocols |
| 5.1.4 | `tests/stress/database_stress_test.go` (extend) | Connection pool exhaustion + recovery |
| 5.1.5 | `tests/stress/memory_pressure_test.go` (extend) | 10K media entities |
| 5.1.6 | `tests/stress/cache_stress_test.go` (new) | Thundering herd on cache |
| 5.1.7 | `tests/stress/sync_stress_test.go` (new) | Concurrent cloud sync operations |
| 5.1.8 | `tests/stress/search_stress_test.go` (new) | Complex query patterns under load |

### 5.2 Integration Test Expansion

| Step | File | Flow |
|------|------|------|
| 5.2.1 | `tests/integration/user_flows_test.go` (extend) | Register -> login -> scan -> browse -> collect -> sync |
| 5.2.2 | `tests/integration/scan_to_entity_test.go` (new) | Multi-protocol scan + aggregation + entity creation |
| 5.2.3 | `tests/integration/websocket_events_test.go` (new) | WS event delivery during concurrent scans |
| 5.2.4 | `tests/integration/search_integration_test.go` (new) | Search across all media types with pagination |
| 5.2.5 | `tests/integration/collection_integration_test.go` (new) | Collection CRUD with real database |
| 5.2.6 | `tests/integration/subtitle_integration_test.go` (new) | Subtitle search, download, verify |
| 5.2.7 | `tests/integration/cover_art_integration_test.go` (new) | Cover art retrieval + caching |
| 5.2.8 | `tests/integration/config_wizard_integration_test.go` (new) | Config wizard flow |

### 5.3 Monitoring & Metrics Tests

| Step | File | Validates |
|------|------|-----------|
| 5.3.1 | `tests/monitoring/prometheus_export_test.go` (new) | Metrics export format |
| 5.3.2 | `tests/monitoring/http_metrics_test.go` (new) | Request count, duration, status |
| 5.3.3 | `tests/monitoring/resource_monitor_test.go` (extend) | Goroutines, memory metrics |
| 5.3.4 | `tests/monitoring/database_metrics_test.go` (new) | Query duration tracking |
| 5.3.5 | `tests/monitoring/websocket_metrics_test.go` (new) | WS connection metrics |
| 5.3.6 | `tests/monitoring/grafana_dashboard_test.go` (new) | Dashboard JSON validity |
| 5.3.7 | `tests/monitoring/alert_rules_test.go` (new) | Alert threshold evaluation |

### 5.4 Chaos Engineering Expansion

| Step | File | Scenario |
|------|------|----------|
| 5.4.1 | `tests/integration/chaos_test.go` (extend) | DB connection drop mid-transaction |
| 5.4.2 | `tests/integration/chaos_redis_test.go` (new) | Redis unavailability + fallback |
| 5.4.3 | `tests/integration/chaos_websocket_test.go` (new) | WS disconnect/reconnect storm |
| 5.4.4 | `tests/integration/chaos_filesystem_test.go` (new) | FS permission changes mid-scan |
| 5.4.5 | `tests/integration/chaos_time_test.go` (new) | Clock skew + JWT validation |
| 5.4.6 | `tests/integration/chaos_lifecycle_test.go` (new) | Concurrent shutdown/restart |

### 5.5 Verification

| Step | Task |
|------|------|
| 5.5.1 | Run all stress tests: `GOMAXPROCS=3 go test -run TestStress ./tests/stress/ -p 2 -parallel 2 -timeout 10m` |
| 5.5.2 | Run all integration tests: `GOMAXPROCS=3 go test ./tests/integration/ -p 2 -parallel 2 -timeout 15m` |
| 5.5.3 | Run all monitoring tests: `go test ./tests/monitoring/` |
| 5.5.4 | Verify Prometheus scraping works during stress run |

---

## Phase 6: Security Scanning Execution {#phase-6}

**Priority:** HIGH
**Dependencies:** Phases 1-5 complete

### 6.1 Run Security Scans

| Step | Tool | Command |
|------|------|---------|
| 6.1.1 | govulncheck | `cd catalog-api && govulncheck ./...` |
| 6.1.2 | npm audit (all 4 projects) | `cd catalog-web && npm audit --production` (repeat for desktop, wizard, api-client) |
| 6.1.3 | SonarQube | `podman-compose -f docker-compose.security.yml up -d sonarqube sonarqube-db` + wait + scan |
| 6.1.4 | Snyk | `podman-compose -f docker-compose.security.yml --profile snyk-scan run --rm snyk-cli` |
| 6.1.5 | OWASP DC | `podman-compose -f docker-compose.security.yml --profile dependency-check run --rm dependency-check` |
| 6.1.6 | Trivy | `podman-compose -f docker-compose.security.yml --profile trivy-scan run --rm trivy-scanner` |
| 6.1.7 | golangci-lint | `golangci-lint run ./...` (with new `.golangci.yml`) |

### 6.2 Analyze & Remediate

| Step | Task |
|------|------|
| 6.2.1 | Collect all scan reports from `reports/` |
| 6.2.2 | Triage by severity: Critical > High > Medium > Low |
| 6.2.3 | Fix all Critical and High dependency vulnerabilities |
| 6.2.4 | Fix all SonarQube bugs/smells marked Critical/Major |
| 6.2.5 | Update dependencies: `go get -u`, `npm audit fix` |
| 6.2.6 | Add suppression rules for verified false positives |
| 6.2.7 | Re-run all scans to verify zero Critical/High |

### 6.3 Security Test Automation

| Step | File | Validates |
|------|------|-----------|
| 6.3.1 | `tests/security/injection_test.go` (extend) | SQL injection on all endpoints |
| 6.3.2 | `tests/security/xss_test.go` (new) | XSS payload rejection |
| 6.3.3 | `tests/security/path_traversal_test.go` (new) | Path traversal prevention |
| 6.3.4 | `tests/security/auth_bypass_test.go` (new) | Auth bypass attempts |
| 6.3.5 | `tests/security/cors_test.go` (new) | CORS policy enforcement |
| 6.3.6 | `tests/security/headers_test.go` (new) | Security headers presence |
| 6.3.7 | `tests/security/rate_limit_test.go` (new) | Rate limiting on auth |
| 6.3.8 | `tests/security/jwt_lifecycle_test.go` (new) | JWT token lifecycle |
| 6.3.9 | `tests/security/upload_validation_test.go` (new) | Magic bytes validation |
| 6.3.10 | `tests/security/command_injection_test.go` (new) | Conversion service |

### 6.4 Verification

| Step | Task |
|------|------|
| 6.4.1 | All security scans: zero Critical/High findings |
| 6.4.2 | All security tests pass |
| 6.4.3 | govulncheck: zero vulnerabilities |
| 6.4.4 | npm audit: zero critical/high |

---

## Phase 7: Challenge Expansion {#phase-7}

**Priority:** MEDIUM
**Dependencies:** Phases 1-6 complete

### 7.1 Feature Validation Challenges (8 new)

| ID | Challenge | Validates |
|----|-----------|-----------|
| CH-061 | Search API basic query | DC-001 wiring |
| CH-062 | Search API duplicate detection | DC-001 wiring |
| CH-063 | Search API advanced filters | DC-001 wiring |
| CH-064 | Browse API storage roots | DC-002 wiring |
| CH-065 | Browse API directory listing | DC-002 wiring |
| CH-066 | Sync API endpoint creation | DC-003 wiring |
| CH-067 | Sync API cloud providers | DC-003 wiring |
| CH-068 | Sync API user endpoints | DC-003 wiring |

### 7.2 Security Validation Challenges (7 new)

| ID | Challenge | Validates |
|----|-----------|-----------|
| CH-069 | Security headers on all responses | SEC-H04 fix |
| CH-070 | CORS rejects unauthorized origins | SEC-H03 fix |
| CH-071 | Input validation rejects injection | SEC-H05 fix |
| CH-072 | Rate limiting on auth endpoints | SEC-M06 fix |
| CH-073 | JWT token lifecycle | SEC-C01 fix |
| CH-074 | File upload magic bytes validation | SEC-L03 fix |
| CH-075 | Conversion rejects path traversal | SEC-H02 fix |

### 7.3 Performance & Resilience Challenges (8 new)

| ID | Challenge | Validates |
|----|-----------|-----------|
| CH-076 | API response < 100ms for simple queries | Phase 3 optimizations |
| CH-077 | API handles 500 concurrent requests | Phase 3 semaphores |
| CH-078 | Graceful degradation under starvation | Phase 3 non-blocking |
| CH-079 | Memory stable during sustained load | Phase 2 leak fixes |
| CH-080 | DB pool recovery after exhaustion | Phase 5 chaos |
| CH-081 | WebSocket reconnection after restart | Phase 2 goroutine fixes |
| CH-082 | Lazy initialization on first request | Phase 3 lazy loading |
| CH-083 | Semaphore prevents overload | Phase 3 semaphores |

### 7.4 Monitoring Challenges (5 new)

| ID | Challenge | Validates |
|----|-----------|-----------|
| CH-084 | Prometheus metrics endpoint valid | Phase 5 metrics |
| CH-085 | HTTP request metrics increment | Phase 5 metrics |
| CH-086 | Runtime metrics current | Phase 5 metrics |
| CH-087 | DB query duration tracked | Phase 5 metrics |
| CH-088 | Grafana dashboard renders | Phase 5 dashboard |

### 7.5 Module Verification Challenges (6 new)

| ID | Challenge | Validates |
|----|-----------|-----------|
| MOD-016 | Lazy Value[T] concurrent access | Lazy module |
| MOD-017 | Lazy Service[T] singleton guarantee | Lazy module |
| MOD-018 | Recovery CircuitBreaker transitions | Recovery module |
| MOD-019 | Recovery HealthChecker verification | Recovery module |
| MOD-020 | Memory LeakDetector tracking | Memory module |
| MOD-021 | Memory KnowledgeGraph BFS | Memory module |

### 7.6 Registration & Verification

| Step | Task |
|------|------|
| 7.6.1 | Implement all challenge structs in `catalog-api/challenges/` |
| 7.6.2 | Register in `catalog-api/challenges/register.go` |
| 7.6.3 | Add tests in `catalog-api/challenges/ch061_090_test.go` |
| 7.6.4 | Verify total: 249 (existing) + 34 (new) = **283 challenges** |

---

## Phase 8: Documentation Completion {#phase-8}

**Priority:** MEDIUM
**Dependencies:** Phases 1-7 complete

### 8.1 New Documents to Create (32 files)

#### API Documentation (3)
| # | Document | Path |
|---|----------|------|
| 1 | Search API reference | `docs/api/SEARCH_API.md` |
| 2 | Browse API reference | `docs/api/BROWSE_API.md` |
| 3 | Sync API reference | `docs/api/SYNC_API.md` |

#### Feature Guides (7)
| # | Document | Path |
|---|----------|------|
| 4 | Subtitle feature guide | `docs/guides/SUBTITLE_GUIDE.md` |
| 5 | Recommendation engine architecture | `docs/architecture/RECOMMENDATION_ENGINE.md` |
| 6 | Advanced search guide | `docs/guides/ADVANCED_SEARCH.md` |
| 7 | Analytics guide | `docs/guides/ANALYTICS_GUIDE.md` |
| 8 | Caching strategy | `docs/architecture/CACHING_STRATEGY.md` |
| 9 | Query optimization | `docs/architecture/QUERY_OPTIMIZATION.md` |
| 10 | Large collections guide | `docs/guides/LARGE_COLLECTIONS.md` |

#### Security Documentation (6)
| # | Document | Path |
|---|----------|------|
| 11 | Security headers doc | `docs/security/SECURITY_HEADERS.md` |
| 12 | CORS configuration | `docs/security/CORS_CONFIGURATION.md` |
| 13 | Secrets management | `docs/security/SECRETS_MANAGEMENT.md` |
| 14 | OAuth/OIDC guide | `docs/guides/OAUTH_INTEGRATION.md` |
| 15 | Security scanning runbook | `docs/security/SECURITY_SCANNING_RUNBOOK.md` |
| 16 | golangci-lint configuration guide | `docs/guides/STATIC_ANALYSIS.md` |

#### Monitoring Documentation (5)
| # | Document | Path |
|---|----------|------|
| 17 | Prometheus metrics reference | `docs/guides/PROMETHEUS_METRICS_REFERENCE.md` |
| 18 | Alerting rules | `docs/guides/ALERTING_RULES.md` |
| 19 | Grafana dashboard guide | `docs/guides/GRAFANA_DASHBOARD_GUIDE.md` |
| 20 | Log aggregation strategy | `docs/architecture/LOG_AGGREGATION.md` |

#### Performance Documentation (4)
| # | Document | Path |
|---|----------|------|
| 21 | Lazy loading architecture | `docs/architecture/LAZY_LOADING.md` |
| 22 | Concurrency control guide | `docs/architecture/CONCURRENCY_CONTROL.md` |
| 23 | Performance tuning | `docs/guides/PERFORMANCE_TUNING.md` |
| 24 | Stress test results | `docs/testing/STRESS_TEST_RESULTS.md` |

#### Test Documentation (4)
| # | Document | Path |
|---|----------|------|
| 25 | Property testing guide | `docs/testing/PROPERTY_TESTING_GUIDE.md` |
| 26 | Visual regression guide | `docs/testing/VISUAL_REGRESSION_GUIDE.md` |
| 27 | Accessibility testing guide | `docs/testing/ACCESSIBILITY_TESTING_GUIDE.md` |

#### Submodule Documentation (4)
| # | Document | Path |
|---|----------|------|
| 28 | Lazy module API reference | `Lazy/docs/API_REFERENCE.md` |
| 29 | Lazy module changelog | `Lazy/docs/CHANGELOG.md` |
| 30 | Recovery module API reference | `Recovery/docs/API_REFERENCE.md` |
| 31 | Recovery module changelog | `Recovery/docs/CHANGELOG.md` |

#### CLAUDE.md Files (11 missing)
| # | Module | Path |
|---|--------|------|
| 32 | Auth-Context-React | `Auth-Context-React/CLAUDE.md` |
| 33 | catalogizer-api-client | `catalogizer-api-client/CLAUDE.md` |
| 34 | Catalogizer-API-Client-TS | `Catalogizer-API-Client-TS/CLAUDE.md` |
| 35 | catalogizer-desktop | `catalogizer-desktop/CLAUDE.md` |
| 36 | Collection-Manager-React | `Collection-Manager-React/CLAUDE.md` |
| 37 | Dashboard-Analytics-React | `Dashboard-Analytics-React/CLAUDE.md` |
| 38 | installer-wizard | `installer-wizard/CLAUDE.md` |
| 39 | Media-Browser-React | `Media-Browser-React/CLAUDE.md` |
| 40 | Media-Player-React | `Media-Player-React/CLAUDE.md` |
| 41 | Media-Types-TS | `Media-Types-TS/CLAUDE.md` |
| 42 | Website | `Website/CLAUDE.md` |

### 8.2 Documents to Update (55 files)

#### Core Guides (12)
- `docs/USER_GUIDE.md` - Search, browse, sync features
- `docs/ADMIN_GUIDE.md` - Security headers, monitoring
- `docs/DEVELOPER_GUIDE.md` - Lazy loading, semaphores, new test types
- `docs/INSTALLATION_GUIDE.md` - Security scanning setup
- `docs/DEPLOYMENT_GUIDE.md` - Monitoring stack
- `docs/CONFIGURATION_GUIDE.md` - Sync, search, browse config
- `docs/TROUBLESHOOTING_GUIDE.md` - Monitoring troubleshooting
- `docs/CHANGELOG.md` - All changes
- `docs/CONTRIBUTING.md` - New test types, golangci-lint
- `README.md` - Feature list
- `CLAUDE.md` - New endpoints, test types, docs
- `AGENTS.md` - New services, constraints

#### Architecture & Schema (6)
- `docs/architecture/SQL_COMPLETE_SCHEMA.md` - Sync tables
- `docs/architecture/DATABASE_SCHEMA.md` - New indexes
- `docs/diagrams/ARCHITECTURE_DIAGRAM.md` - New services
- `docs/diagrams/COMPONENT_DIAGRAM.md` - Lazy boundaries
- `docs/diagrams/SEQUENCE_DIAGRAMS.md` - Sync, security flows
- `docs/diagrams/ER_DIAGRAM.md` - Sync relationships

#### API & Testing (8)
- `docs/api/openapi.yaml` - Search, browse, sync endpoints
- `docs/api/API_DOCUMENTATION.md` - New endpoints
- `docs/testing/TESTING.md` - New test types
- `docs/testing/FUZZ_TESTING_GUIDE.md` - New fuzz targets
- `docs/testing/challenge-map.md` - CH-061 to CH-088, MOD-016 to MOD-021
- `docs/testing/TEST_IMPLEMENTATION_SUMMARY.md` - Remove Android TODOs
- `docs/testing/TESTING_REPORT.md` - Final coverage numbers
- `docs/deployment/PRODUCTION_DEPLOYMENT_GUIDE.md` - Remove Prometheus TODO

#### Security (2)
- `docs/security/COMPREHENSIVE_SECURITY_AUDIT.md` - Remediation results
- `docs/security/SECURITY_SCAN_REPORT.md` - Updated scan results

#### Platform Guides (4)
- `docs/guides/ANDROID_GUIDE.md` - Cloud sync features
- `docs/guides/ANDROID_TV_GUIDE.md` - Search/browse
- `docs/guides/DESKTOP_GUIDE.md` - Sync and search
- `docs/guides/WEB_APP_GUIDE.md` - New features

#### Course Materials (8)
- `docs/courses/slides/MODULE_1_SLIDES.md` through `MODULE_8_SLIDES.md` - Updated content

#### Video Course (4)
- `docs/video-course/MODULE9_APPS_SCRIPT.md` through `MODULE12_DEPLOYMENT_SCRIPT.md` - Extended content

---

## Phase 9: User Manuals, Video Courses & Content Extension {#phase-9}

**Priority:** MEDIUM
**Dependencies:** Phase 8 complete

### 9.1 Video Course Extension

| Step | Module | File | Content |
|------|--------|------|---------|
| 9.1.1 | Module 9 extension | `MODULE9_APPS_SCRIPT.md` | Add search/sync in mobile apps |
| 9.1.2 | Module 10 extension | `MODULE10_TESTING_SCRIPT.md` | Add fuzz, property, visual, accessibility testing |
| 9.1.3 | Module 11 extension | `MODULE11_SECURITY_SCRIPT.md` | Add Snyk/SonarQube/Trivy scanning, golangci-lint |
| 9.1.4 | Module 12 extension | `MODULE12_DEPLOYMENT_SCRIPT.md` | Add monitoring stack deployment, alerting |
| 9.1.5 | Module 13 (NEW) | `docs/video-course/MODULE13_SYNC_SEARCH.md` | Search API, Browse API, Cloud Sync (S3/GCS/WebDAV) |
| 9.1.6 | Module 14 (NEW) | `docs/video-course/MODULE14_CHALLENGES.md` | Challenge system deep dive, writing/running challenges |

### 9.2 Online Course Extension

| Step | Task | File |
|------|------|------|
| 9.2.1 | Add Module 9 slides | `docs/courses/slides/MODULE_9_SLIDES.md` (new) |
| 9.2.2 | Add Module 10 slides | `docs/courses/slides/MODULE_10_SLIDES.md` (new) |
| 9.2.3 | Update Module 5 slides (security) | `docs/courses/slides/MODULE_5_SLIDES.md` |
| 9.2.4 | Update Module 6 slides (testing) | `docs/courses/slides/MODULE_6_SLIDES.md` |
| 9.2.5 | Update Module 7 slides (deployment) | `docs/courses/slides/MODULE_7_SLIDES.md` |
| 9.2.6 | Update exercises | `docs/courses/EXERCISES.md` |
| 9.2.7 | Update assessment | `docs/courses/ASSESSMENT.md` |
| 9.2.8 | Update course outline | `docs/courses/COURSE_OUTLINE.md` |

### 9.3 Step-by-Step User Manuals

| Step | Guide | Coverage |
|------|-------|----------|
| 9.3.1 | Complete Web App user manual | Login, search, browse, collections, sync, settings |
| 9.3.2 | Complete Desktop app user manual | Installation, scanning, browsing, sync |
| 9.3.3 | Complete Android user manual | Setup, browsing, offline sync |
| 9.3.4 | Complete Android TV user manual | Leanback UI, media playback |
| 9.3.5 | Complete Administrator manual | Security config, monitoring, backup |
| 9.3.6 | Complete Developer manual | API usage, testing, contributing |

---

## Phase 10: Website, OpenAPI & Final Validation {#phase-10}

**Priority:** MEDIUM
**Dependencies:** Phase 9 complete

### 10.1 Website Content Updates

| Step | Page | Updates |
|------|------|---------|
| 10.1.1 | `Website/features.md` | Search, browse, sync, monitoring features |
| 10.1.2 | `Website/getting-started.md` | Current setup steps |
| 10.1.3 | `Website/faq.md` | New feature questions |
| 10.1.4 | `Website/changelog.md` | All Phase 1-9 changes |
| 10.1.5 | `Website/download.md` | Current version info |
| 10.1.6 | `Website/course.md` | Modules 13-14 |
| 10.1.7 | `Website/documentation.md` | New documentation pages |
| 10.1.8 | `Website/support.md` | Updated support resources |
| 10.1.9 | `Website/docs/developer-guide/testing-strategy.md` | New test types |
| 10.1.10 | `Website/docs/getting-started/index.md` | Updated quickstart |
| 10.1.11 | New: `Website/docs/developer-guide/security.md` | Security guide |
| 10.1.12 | New: `Website/docs/developer-guide/monitoring.md` | Monitoring guide |
| 10.1.13 | New: `Website/docs/developer-guide/api-reference.md` | API reference |
| 10.1.14 | Update `Website/.vitepress/config.ts` | Sidebar with new pages |
| 10.1.15 | Build and verify: `cd Website && npm run build` | Zero errors |

### 10.2 OpenAPI Spec Update

| Step | Task |
|------|------|
| 10.2.1 | Add search endpoints to `docs/api/openapi.yaml` |
| 10.2.2 | Add browse endpoints to `docs/api/openapi.yaml` |
| 10.2.3 | Add sync endpoints to `docs/api/openapi.yaml` |
| 10.2.4 | Validate: `npx @apidevtools/swagger-cli validate docs/api/openapi.yaml` |

### 10.3 Final Validation & Acceptance

| Step | Task | Command |
|------|------|---------|
| 10.3.1 | Go tests with race detection + coverage | `GOMAXPROCS=3 go test -race -coverprofile=coverage.out ./... -p 2 -parallel 2` |
| 10.3.2 | Frontend tests with coverage | `cd catalog-web && npm run test:coverage` |
| 10.3.3 | E2E tests | `cd catalog-web && npm run test:e2e` |
| 10.3.4 | Desktop tests | `cd catalogizer-desktop && npm test` |
| 10.3.5 | Installer tests | `cd installer-wizard && npm test` |
| 10.3.6 | API client tests | `cd catalogizer-api-client && npm test` |
| 10.3.7 | Security scans (all tools) | `./scripts/run-security-scan.sh` |
| 10.3.8 | Stress tests | `go test -run TestStress ./tests/stress/ -timeout 10m` |
| 10.3.9 | Challenge tests | `go test -run TestChallengeRegistration ./challenges/` |
| 10.3.10 | Documentation links validation | Check all internal links |
| 10.3.11 | Website build | `cd Website && npm run build` |
| 10.3.12 | All 7 components build | `./scripts/release-build.sh --container --force --skip-tests` |
| 10.3.13 | Generate final status report | `docs/status/FINAL_STATUS_2026-03-08.md` |

---

## 14. Test Type Inventory

### Complete List of Supported Test Types (24)

| # | Test Type | Framework | Location | Status |
|---|-----------|-----------|----------|--------|
| 1 | Go Unit Tests | `testing` | `*_test.go` beside source | Active (3,738+ functions) |
| 2 | Go Table-Driven Tests | `testing` + subtests | Widespread | Active |
| 3 | Go Benchmark Tests | `testing.B` | `*_bench_test.go` | Active (74 functions), expanding to 82+ |
| 4 | Go Fuzz Tests | `testing.F` | `*_fuzz_test.go` | Active (4 files), expanding to 15+ |
| 5 | Go Property-Based Tests | `testing/quick` | `*_property_test.go` | **NEW** - Phase 4 (8 files) |
| 6 | Go Integration Tests | `testing` + test DB | `tests/integration/` | Active (52+ functions), expanding |
| 7 | Go Stress Tests | `testing` + goroutines | `tests/stress/` | Active (31+ functions), expanding |
| 8 | Go Performance Tests | `testing` | `tests/performance/` | Active |
| 9 | Go Chaos Tests | `testing` + fault injection | `tests/integration/chaos_*.go` | Active (128+ functions), expanding |
| 10 | Go Contract Tests | `testing` + HTTP + OpenAPI | `tests/contract/` | Expanding |
| 11 | Go Security Tests | `testing` + attack patterns | `tests/security/` | Active (13+), expanding to 30+ |
| 12 | Go Monitoring Tests | `testing` + metrics | `tests/monitoring/` | Active (64+), expanding |
| 13 | Go Race Detection | `-race` flag | All test packages | Active |
| 14 | React Unit Tests | Vitest + Testing Library | `src/**/__tests__/` | Active (3,085+ cases), expanding |
| 15 | React Snapshot Tests | Vitest snapshots | `__snapshots__/` | Active (5), expanding to 30+ |
| 16 | React E2E Tests | Playwright | `e2e/` | Active (25 files) |
| 17 | Visual Regression Tests | Playwright screenshots | `e2e/visual/` | **NEW** - Phase 4 (7 files) |
| 18 | Accessibility Tests | axe-core + Playwright | `e2e/accessibility/` | **NEW** - Phase 4 (4 files) |
| 19 | Kotlin Unit Tests | JUnit5 | `app/src/test/` | Active |
| 20 | Kotlin Instrumented Tests | AndroidJUnit | `app/src/androidTest/` | Active, expanding |
| 21 | Challenge Tests | Challenge Framework | `challenges/` | Active (249), expanding to 283 |
| 22 | API Client Tests | Vitest | `catalogizer-api-client/` | Active (7 files) |
| 23 | Desktop Tests | Vitest | `catalogizer-desktop/` | Active (15 files), expanding |
| 24 | Installer Tests | Vitest | `installer-wizard/` | Active (19 files), expanding |

### Coverage Targets

| Component | Current | Target |
|-----------|---------|--------|
| catalog-api (Go) | ~75% line | 95%+ |
| catalog-web (React) | ~60% line | 85%+ |
| catalogizer-desktop | ~70% | 85%+ |
| installer-wizard | ~75% | 85%+ |
| catalogizer-android | ~78% | 85%+ |
| catalogizer-androidtv | ~63% | 80%+ |
| catalogizer-api-client | ~80% | 95%+ |
| Lazy module | ~90% | 100% |
| Memory module | ~85% | 95%+ |
| Recovery module | ~90% | 100% |

---

## 15. Verification & Acceptance Criteria

### Phase Gate Criteria

Each phase must satisfy ALL of these before proceeding:

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | Zero compilation errors | `go build ./...`, `npm run build`, `tsc --noEmit` |
| 2 | Zero test regressions | Full test suite passes (all components) |
| 3 | Zero race conditions | `go test -race ./...` |
| 4 | Zero console warnings | Browser console clean |
| 5 | Zero failed network requests | All API calls succeed |
| 6 | All existing challenges still pass | Challenge runner verification |
| 7 | Resource limits respected | `podman stats` shows < 40% host usage |

### Final Acceptance Criteria

| # | Criterion | Command |
|---|-----------|---------|
| 1 | All Go tests pass (38+ packages) | `GOMAXPROCS=3 go test ./... -p 2 -parallel 2` |
| 2 | All Go tests pass with race detector | `GOMAXPROCS=3 go test -race ./... -p 2 -parallel 2` |
| 3 | Go coverage >= 90% | `go test -coverprofile=coverage.out ./...` |
| 4 | All frontend tests pass | `npm run test` in all 4 JS/TS projects |
| 5 | Frontend coverage >= 85% | `npm run test:coverage` |
| 6 | E2E tests pass | `npm run test:e2e` |
| 7 | Visual regression: zero diffs | Playwright screenshots match baselines |
| 8 | Accessibility: zero violations | axe-core reports clean |
| 9 | govulncheck: zero vulnerabilities | `govulncheck ./...` |
| 10 | npm audit: zero critical/high | `npm audit --production` |
| 11 | SonarQube: zero Critical/Blocker | Quality gate passes |
| 12 | Snyk: zero Critical/High | Scan clean |
| 13 | All 283 challenges registered | Challenge count verification |
| 14 | All documentation links resolve | Link checker passes |
| 15 | Website builds successfully | `cd Website && npm run build` |
| 16 | All 7 components build | Release pipeline succeeds |
| 17 | Zero TODO/FIXME in production code | `grep` verification |
| 18 | All 38 CLAUDE.md files present | File existence check |
| 19 | All 287+ docs up to date | Content verification |
| 20 | OpenAPI spec validates | `swagger-cli validate` |

---

## Appendix A: File Counts Summary

| Metric | Before | After (Target) |
|--------|--------|----------------|
| Go test files | 242 | 297+ |
| React test files | 103 | 118+ |
| Fuzz test files | 4 | 15 |
| Property test files | 0 | 8 |
| Visual regression specs | 0 | 7 |
| Accessibility specs | 0 | 4 |
| Security test files | 13 | 23+ |
| Stress test files | 7 | 15+ |
| Integration test files | 11 | 19+ |
| Monitoring test files | 1 | 8+ |
| Chaos test files | 1 | 7 |
| Challenge definitions | 249 | 283 |
| Documentation files | 287 | 340+ |
| Video course modules | 12 | 14 |
| Online course slides | 8 | 10 |
| Website pages | 9 | 15+ |
| CLAUDE.md files | 27 | 38 |

## Appendix B: Resource Budget (30-40% Host Maximum)

All operations MUST respect host resource limits:

| Operation | CPU Limit | Memory Limit |
|-----------|-----------|-------------|
| Go tests | `GOMAXPROCS=3 -p 2 -parallel 2` | Natural |
| Go fuzz tests | `GOMAXPROCS=2 -fuzztime=30s` | Natural |
| SonarQube + DB | 3 CPUs | 3 GB |
| Snyk scan | 1 CPU | 1 GB |
| Trivy scan | 1 CPU | 1 GB |
| Total containers | 4 CPUs max | 8 GB max |

## Appendix C: Constraints Compliance

| Constraint | Source | Compliance |
|-----------|--------|------------|
| No GitHub Actions | CLAUDE.md | All CI/CD local |
| Podman only (no Docker) | CLAUDE.md | All containers via Podman |
| HTTP/3 + Brotli | CLAUDE.md | Maintained |
| 30-40% host resources | CLAUDE.md | All commands resource-limited |
| No interactive processes (no sudo) | User request | No sudo/root commands |
| Zero warning/zero error policy | CLAUDE.md | Enforced at each phase gate |
| Challenge system via service only | CLAUDE.md | All challenges through catalog-api |
| Container builds only | CLAUDE.md | `--network host`, fully qualified images |
