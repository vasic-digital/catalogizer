# Comprehensive Completion Design Spec

**Date:** 2026-04-03
**Version:** v2.2.0 Target
**Scope:** Full project completion — zero unfinished, undocumented, broken, or disabled items
**Approach:** 10 layered phases, each independently completable and verifiable

---

## Table of Contents

1. [Audit Summary](#1-audit-summary)
2. [Phase 1: Foundation Fixes](#phase-1-foundation-fixes)
3. [Phase 2: Safety Hardening](#phase-2-safety-hardening)
4. [Phase 3: Security Scanning & Remediation](#phase-3-security-scanning-and-remediation)
5. [Phase 4: Test Coverage Maximization](#phase-4-test-coverage-maximization)
6. [Phase 5: Stress & Performance Testing](#phase-5-stress-and-performance-testing)
7. [Phase 6: Challenge Bank Expansion](#phase-6-challenge-bank-expansion)
8. [Phase 7: Dead Feature Activation](#phase-7-dead-feature-activation)
9. [Phase 8: Documentation Completion](#phase-8-documentation-completion)
10. [Phase 9: Website & Video Courses](#phase-9-website-and-video-courses)
11. [Phase 10: Final Verification & Release Prep](#phase-10-final-verification-and-release-prep)
12. [Cross-Cutting Concerns](#cross-cutting-concerns)
13. [Verification Gates](#verification-gates-summary)
14. [Risk Register](#risk-register)

---

## 1. Audit Summary

### 1.1 Current State Metrics

| Metric | Value |
|--------|-------|
| Go backend packages | 44 with tests |
| Go submodules | 23 (all compile) |
| Frontend test files | 130+ |
| Frontend tests | 2,330+ |
| Challenges registered | 492 |
| HelixQA test cases | 1,228 |
| k6 load test scripts | 7 |
| Documentation files | 1,532+ |
| Docker Compose files | 8 |
| Shell scripts | 80+ |

### 1.2 Critical Findings

**Dead/Unconnected Features (6):**
- Android OfflineRepository — implemented, never injected into DependencyContainer
- Android Biometric Auth — BIOMETRIC_ENABLED_KEY + Flow defined, never used in UI
- Android WebSocket — WebSocketEvent sealed class defined, no WS initialization
- Panoptic GenerateAnalytics — TODO stub at `Challenges/Panoptic/internal/cloud/manager.go:1005`
- Panoptic SaveReport — TODO stub at `Challenges/Panoptic/internal/cloud/manager.go:1020`
- challenges_bank.json — empty array, challenges only hardcoded in Go

**Policy Violations (3):**
- 23 console.error statements in catalog-web violating zero-warning policy
- deploy.sh hardcodes "docker" — must dynamically detect runtime (Podman > Docker > others)
- 6 deprecated ioutil usages in catalog-api/tests/automation/storage_operations_test.go

**Memory/Lifecycle Issues (4):**
- Android appScope (SupervisorJob) never cancelled on app termination
- SyncWorker continues after user logout
- OkHttpClient singleton never closed
- MainActivity postDelayed callback not cleaned on destroy

**Test Coverage Gaps:**
- 19/23 Go submodules lack edge case tests
- 10/23 Go submodules lack integration tests
- 4 TS library packages with 0 component tests (14 untested components total)
- catalogizer-androidtv: ZERO instrumentation tests
- 12 duplicate test files in catalogizer-android (TestN/Test2N/Test3N pattern)
- ExampleStateViewModelTest is dead test code

**Infrastructure Issues:**
- Go submodule version inconsistency (1.24.0 / 1.25.0 / 1.25.7)
- docker-compose.yml GO_VERSION arg defaults to 1.21 (should be 1.25)
- Android Compose BOM outdated (2023.10)
- Gson vs Kotlinx Serialization inconsistency between Android apps
- SonarQube scanner needs sonar-project.properties
- Security scans stale (last: 2026-02-10)

---

## Phase 1: Foundation Fixes

**Goal:** Eliminate all policy violations, deprecated APIs, version inconsistencies, and dead test code. Establish dynamic container runtime detection.

### 1.1 Dynamic Container Runtime Detection

**File:** `scripts/lib/container-runtime.sh` (NEW)

Create a reusable shell library that all scripts source for container operations:

```
Detection priority:
1. Podman (podman + podman-compose)
2. Docker (docker + docker-compose)
3. nerdctl (containerd)
4. Fail with clear error message
```

**Functions to provide:**
- `detect_container_runtime()` — sets CONTAINER_CMD and COMPOSE_CMD globals
- `container_run()` — wraps $CONTAINER_CMD run
- `container_build()` — wraps $CONTAINER_CMD build
- `container_compose()` — wraps $COMPOSE_CMD
- `container_available()` — returns 0 if any runtime found

**Files to update:**
- `scripts/deploy.sh` — replace all docker calls with dynamic runtime variables
- `scripts/release-build.sh` — already uses Podman, add fallback via sourcing new lib
- `scripts/security-scan.sh` — replace docker references
- `scripts/run-sonarqube-scan.sh` — replace podman-compose with dynamic compose
- All `scripts/run-helixqa*.sh` — use dynamic detection
- `scripts/services-up.sh`, `services-down.sh` — use dynamic detection
- Every other script in scripts/ that references podman or docker directly

**Constraint:** Podman remains the documented preferred runtime. The dynamic detection is a fallback mechanism, not a policy change.

### 1.2 Deprecated API Cleanup

**File:** `catalog-api/tests/automation/storage_operations_test.go`

| Line | Old | New |
|------|-----|-----|
| 39, 60, 93 | ioutil.ReadAll() | io.ReadAll() |
| 133 | ioutil.TempDir() | os.MkdirTemp() |
| 139 | ioutil.WriteFile() | os.WriteFile() |
| 148 | ioutil.ReadDir() | os.ReadDir() |

Remove io/ioutil import, add io and os imports as needed.

### 1.3 Console.error Elimination (catalog-web)

Replace all 23 console.error statements with proper error handling:

**Strategy by component type:**
- **Auth forms** (LoginForm, RegisterForm): Replace with form-level error state displayed in UI
- **AI components** (AIAnalytics, AIMetadata, AIComponents): Replace with React Query onError callbacks + toast notifications
- **Collection components** (CollectionSettings, BulkOperations, CollectionExport): Replace with toast notifications via existing notification system
- **Dashboard** (ActivityFeed): Replace with error state rendering
- **Layout** (Header): Replace with toast notification
- **Playlist components**: Replace with error state + toast
- **Subtitle components**: Replace with modal error state
- **ErrorBoundary**: Keep console.error — this is the expected location for error logging
- **Hooks** (useCollections): Replace with silent fallback or toast
- **WebSocket-Client-TS**: Keep console.error — library-level error reporting is appropriate

**Net result:** Remove ~21 console.error calls, keep 2 (ErrorBoundary + WebSocket library).

### 1.4 Version Alignment

**Go submodule go.mod files — align all to Go 1.25:**

| Module | Current | Target |
|--------|---------|--------|
| All 20 modules at 1.24.0 | 1.24.0 | 1.25 |
| Storage | 1.25.0 | 1.25 |
| Lazy | 1.25.7 | 1.25 |

**docker-compose.yml line 71:**
```yaml
GO_VERSION: ${GO_VERSION:-1.25}
```

**catalog-api/Dockerfile:**
Verify FROM golang:1.25 matches.

### 1.5 Dead Test Code Cleanup (Android)

- **Delete** ExampleStateViewModelTest.kt — tests non-existent functionality
- **Consolidate** 12 duplicate test files (MainViewModelTest 1/2/3 etc.) — merge unique assertions into single test files, delete duplicates
- **Files affected:** MainViewModelTest{,2,3}.kt, SearchViewModelTest{,2,3}.kt, AuthViewModelTest{,2,3}.kt, HomeViewModelTest{,2,3}.kt

### 1.6 Compose BOM Update (Android)

- Update compose.bom from 2023.10 to latest stable (2024.x or 2025.x)
- Update kotlinCompilerExtensionVersion to match BOM
- Align serialization: migrate androidtv from Gson to Kotlinx Serialization (matching android app)

### 1.7 SonarQube Project Properties

**File:** `sonar-project.properties` (NEW, project root)

```properties
sonar.projectKey=catalogizer
sonar.projectName=Catalogizer
sonar.projectVersion=2.2.0
sonar.sources=catalog-api,catalog-web/src
sonar.tests=catalog-api,catalog-web/src
sonar.test.inclusions=**/*_test.go,**/*.test.ts,**/*.test.tsx,**/*.spec.ts
sonar.go.coverage.reportPaths=catalog-api/coverage.out
sonar.javascript.lcov.reportPaths=catalog-web/coverage/lcov.info
sonar.exclusions=**/node_modules/**,**/vendor/**,**/build/**,**/*.min.js
```

### Phase 1 Verification Gate

- [ ] `go build ./...` passes in catalog-api
- [ ] `npm run build && npm run lint && npm run type-check` passes in catalog-web
- [ ] Zero ioutil imports in codebase
- [ ] Zero console.error in catalog-web src (except ErrorBoundary + libraries)
- [ ] All Go submodules at go 1.25 in go.mod
- [ ] detect_container_runtime() returns Podman on current host
- [ ] scripts/deploy.sh uses dynamic runtime, never hardcodes "docker"
- [ ] Android builds pass with updated BOM
- [ ] No duplicate test files in catalogizer-android

---

## Phase 2: Safety Hardening

**Goal:** Eliminate all memory leaks, fix lifecycle issues, add deadlock prevention, strengthen race condition safety.

### 2.1 Android Lifecycle Fixes

**CatalogizerApplication.kt — appScope cancellation:**
```kotlin
override fun onTerminate() {
    super.onTerminate()
    appScope.cancel()
}
```
Also register ProcessLifecycleOwner observer to cancel on ON_DESTROY.

**SyncManager.kt — cancel sync on logout:**
Add cancelSync() method that calls WorkManager.cancelUniqueWork(SYNC_WORK_NAME).
Wire into AuthRepository.logout() flow.

**DependencyContainer.kt — OkHttpClient cleanup:**
Add shutdown() method that calls okHttpClient.dispatcher.executorService.shutdown() and okHttpClient.connectionPool.evictAll().
Call from CatalogizerApplication.onTerminate().

**MainActivity.kt — postDelayed cleanup:**
Store Runnable reference, remove via handler.removeCallbacks(runnable) in onDestroy().

### 2.2 Go Backend Concurrency Audit

**Transaction deadlock prevention** (database/transaction.go:200-230):
- Add timeout to RecordEnd() call inside locked section
- Add deadlock detection logging when lock acquisition takes >1s
- Pattern: tryLockWithTimeout(mu, 5*time.Second)

**Large buffered channels monitoring** (10K capacity in watcher.go, enhanced_watcher.go):
- Add Prometheus gauge metrics for channel buffer utilization: catalogizer_channel_depth{name="change_queue"}
- Add high-water-mark alerting at 80% capacity
- Log warning when channel reaches 90% capacity

**Goroutine tracking for background tasks:**
- handlers/service_adapters.go:269 — add WaitGroup tracking
- handlers/media_entity_handler.go:1033 — add WaitGroup tracking
- Ensure all are drained on graceful shutdown

### 2.3 Frontend Memory Safety

**ConnectionStatus.tsx — unstable dependency fix:**
Wrap getConnectionState in useCallback at the WebSocket provider level, or use useRef pattern to avoid effect re-runs.

**Modal/Form error boundaries:**
Wrap SubtitleUploadModal, SubtitleSyncModal, LoginForm, RegisterForm with ErrorBoundary in their parent render sites.

### 2.4 Lazy Loading and Semaphore Enhancements

**Go backend:**
- Audit all service constructors in main.go — identify which can use LazyServiceRegistry from internal/lifecycle/
- Services that do not need eager init: SubtitleService, ConversionService, PlaylistService, RecommendationService, SyncService
- Add semaphore control to parallel scan operations (already in internal/concurrency/ — verify all scan paths use it)
- Add semaphore to media analysis queue (cap concurrent analysis workers)

**Frontend:**
- Verify all routes use React.lazy() + Suspense (already in place per App.tsx — confirm no regressions)
- Add React.lazy() for heavy modal components (SubtitleSyncModal, CollectionExport, BulkOperations)
- Verify React Query staleTime and cacheTime are optimal (not refetching unnecessarily)

**Android:**
- Verify ViewModels use viewModelScope (not custom scopes) for automatic lifecycle management
- Add lazy initialization to DependencyContainer services that are not needed at startup

### 2.5 Non-Blocking Mechanism Audit

**Go backend:**
- Verify all HTTP handlers use context timeout (Gin built-in context)
- Verify no blocking I/O in request handlers without timeout
- Audit SMB client for blocking operations — ensure circuit breaker prevents hangs
- Verify Redis operations have timeouts set

**Frontend:**
- Verify all API calls use AbortController for cancelation on unmount
- Verify WebSocket reconnection uses exponential backoff (already in WebSocket-Client-TS — confirm)

### Phase 2 Verification Gate

- [ ] `go test -race ./...` passes in catalog-api (no data races)
- [ ] Android app lifecycle: launch to background to kill to relaunch with no leaked resources
- [ ] SyncWorker stops within 30s of logout
- [ ] All goroutines tracked and drain on SIGTERM
- [ ] Channel buffer metrics visible in Prometheus /metrics
- [ ] No useEffect cleanup warnings in browser console
- [ ] Semaphore-bounded scan operations (configurable max concurrency)

---

## Phase 3: Security Scanning and Remediation

**Goal:** Execute full security scanning suite, analyze all findings, remediate everything critical/high/medium.

### 3.1 SonarQube Full Scan

**Prerequisites:**
- sonar-project.properties in place (Phase 1)
- SonarQube container healthy

**Execution:**
```bash
./scripts/run-sonarqube-scan.sh
```

**Post-scan:**
- Export findings to docs/security/sonarqube-report-2026-04-03.json
- Categorize: Bugs, Vulnerabilities, Code Smells, Security Hotspots
- Remediate all Critical and High severity items
- Document accepted Medium/Low risks with justification

### 3.2 Snyk Dependency Scan

**Execution via docker-compose.security.yml:**
```bash
podman-compose -f docker-compose.security.yml --profile snyk-scan run --rm snyk-cli
```

**Coverage:**
- Go dependencies (catalog-api/go.mod)
- Node dependencies (catalog-web/package.json, all TS libraries)
- Container images (catalog-api Dockerfile, catalog-web Dockerfile)
- IaC (docker-compose files)

**Remediation:**
- Upgrade vulnerable dependencies
- Apply patches where upgrades are not possible
- Document false positives with .snyk file

### 3.3 Semgrep SAST Scan

**Execution:**
```bash
podman-compose -f docker-compose.security.yml --profile semgrep-scan run --rm semgrep-scanner
```

**Focus areas:**
- SQL injection patterns (Go string concatenation in queries)
- XSS patterns (unsafe HTML rendering)
- SSRF patterns (user-controlled URLs in HTTP clients)
- Hardcoded secrets (API keys, passwords in source)
- Insecure crypto (weak algorithms, static IVs)

### 3.4 OWASP Dependency Check

```bash
podman-compose -f docker-compose.security.yml --profile dependency-check run --rm dependency-check
```

### 3.5 Trivy Filesystem Scan

```bash
podman-compose -f docker-compose.security.yml --profile trivy-scan run --rm trivy
```

### 3.6 govulncheck (Go)

```bash
cd catalog-api && govulncheck ./...
```

### 3.7 npm audit (Frontend)

```bash
cd catalog-web && npm audit --production
```

### 3.8 Findings Consolidation

Create `docs/security/2026-04-03-security-scan-consolidated.md`:
- All tools findings merged and deduplicated
- Severity matrix (Critical/High/Medium/Low/Info)
- Remediation status for each finding
- Accepted risks with justification
- Comparison with previous scan (2026-02-10)

### Phase 3 Verification Gate

- [ ] SonarQube: 0 Critical, 0 High bugs/vulnerabilities
- [ ] Snyk: 0 Critical/High vulnerabilities in production dependencies
- [ ] Semgrep: 0 Critical/High SAST findings
- [ ] govulncheck: 0 known vulnerabilities
- [ ] npm audit: 0 critical/high in production
- [ ] Consolidated report written and committed

---

## Phase 4: Test Coverage Maximization

**Goal:** Achieve theoretical maximum test coverage across all modules, libraries, and applications.

### 4.1 Go Submodule Edge Case Tests

Add edge case test files (*_edge_test.go) to all 19 submodules currently lacking them:

**Per-module edge cases to test:**
- **Assets**: Empty asset paths, concurrent access, nil defaults
- **Auth**: Expired tokens, malformed JWTs, empty credentials, unicode usernames
- **Cache**: TTL boundary (0, negative, max), concurrent eviction, cache stampede
- **Concurrency**: Zero-size pool, pool overflow, panic in worker, context cancel mid-task
- **Config**: Malformed JSON, missing required fields, type mismatch, empty env vars
- **Containers**: Unreachable container daemon, permission denied, port conflicts
- **Database**: Max connections exhausted, concurrent migrations, dialect edge cases
- **Entities**: Unicode titles, empty strings, extremely long titles, special chars (/, backslash, null bytes)
- **EventBus**: Nil handlers, concurrent subscribe/unsubscribe, publish during shutdown
- **Filesystem**: Path traversal attempts, symlink loops, permission denied, empty paths
- **Lazy**: Concurrent initialization race, panic in initializer, nil value
- **Media**: Zero-byte files, corrupted headers, extremely long filenames
- **Memory**: Allocator exhaustion, concurrent leak detection, false positive scenarios
- **Middleware**: Malformed headers, extremely large headers, empty body, slow clients
- **Observability**: Metric overflow, label cardinality explosion, nil logger
- **RateLimiter**: Zero rate, negative burst, concurrent limit changes
- **Security**: SQL injection attempts, XSS payloads, path traversal, null bytes
- **Storage**: Full disk, permission denied, concurrent writes, symbolic links
- **Streaming**: Seek past EOF, zero-length stream, client disconnect mid-stream

### 4.2 Go Submodule Integration Tests

Add integration test files (*_integration_test.go) to the 10 submodules missing them:

- **Assets**: Test with real filesystem asset loading
- **Config**: Test config loading from file + env var override chain
- **Entities**: Test title parsing pipeline end-to-end
- **Filesystem**: Test local filesystem operations (no network needed)
- **Lazy**: Test concurrent lazy init under load
- **Middleware**: Test middleware chain composition
- **RateLimiter**: Test rate limiting under concurrent requests
- **Watcher**: Test file change detection with real filesystem events

### 4.3 Go Backend Coverage Gaps

**Packages needing additional test coverage:**

| Package | Gap | Tests to Add |
|---------|-----|-------------|
| internal/cache | 1 test file for 2 source files | Add cache eviction, TTL, concurrent access tests |
| internal/media root | 1 test for 2 source files | Add media pipeline integration test |
| internal/models | 1 test for 2 source files | Add model validation, serialization tests |
| internal/modules | 1 test for 2 source files | Add module registration, dependency tests |
| internal/recovery | 2 tests for 3 source files | Add circuit breaker state machine edge cases |
| repository/ | 3 tests for 6 source files | Add CRUD tests for all repository methods |
| challenges/ | 13 tests for 88 source files | Add challenge execution unit tests |

### 4.4 TypeScript Library Component Tests

**Collection-Manager-React (4 new test files):**
- `__tests__/CollectionCard.test.tsx` — render, click handlers, empty/loading states
- `__tests__/CollectionForm.test.tsx` — validation, submission, error states
- `__tests__/CollectionList.test.tsx` — pagination, filtering, empty state
- `__tests__/SmartRuleBuilder.test.tsx` — rule creation, editing, deletion

**Dashboard-Analytics-React (4 new test files):**
- `__tests__/ActivityFeed.test.tsx` — feed rendering, pagination, empty state
- `__tests__/EntityStatsGrid.test.tsx` — grid layout, stat display, loading
- `__tests__/MediaDistributionBar.test.tsx` — chart rendering, data formatting
- `__tests__/StatsCard.test.tsx` — value display, trend indicators

**Media-Player-React (2 new test files):**
- `__tests__/PlayerControls.test.tsx` — play/pause, seek, volume, fullscreen
- `__tests__/useMediaPlayer.test.ts` — hook state management, event handling

**Media-Browser-React (2 new test files):**
- `__tests__/EntityCard.test.tsx` — render, click, hover, image loading
- `__tests__/EntityGrid.test.tsx` — grid layout, responsive behavior, loading

### 4.5 Android TV Instrumentation Tests

**New test files in catalogizer-androidtv/app/src/androidTest/:**
- DpadNavigationTest.kt — D-pad up/down/left/right/center navigation
- LeanbackBrowseTest.kt — Browse fragment row navigation
- MediaPlaybackTest.kt — ExoPlayer controls via remote
- TVProviderChannelTest.kt — Home screen channel content
- SearchTest.kt — Voice and text search on TV
- FocusManagementTest.kt — Focus traversal across all screens

### 4.6 Installer Wizard Integration Tests

**New test files:**
- NetworkScanIntegration.test.tsx — Mock network discovery, timeout handling
- ProtocolConnectionTest.test.tsx — SMB/FTP/NFS/WebDAV mock connections
- ConfigurationPersistence.test.tsx — Save/load configuration via Tauri IPC

### 4.7 Test Bank Framework Integration

All new tests must also be registered as HelixQA test bank entries in appropriate bank files under HelixQA/banks/:
- banks/unit-tests.json — unit test expectations
- banks/integration-tests.json — integration test expectations
- banks/edge-cases.json — edge case test expectations
- banks/platform-specific.json — Android TV, desktop, wizard tests

### Phase 4 Verification Gate

- [ ] `go test -cover ./...` shows >85% coverage in catalog-api
- [ ] All 23 Go submodules have edge case tests
- [ ] All 23 Go submodules have integration tests
- [ ] All TS library components have test files (0 untested exports)
- [ ] npm run test:coverage shows >85% branch coverage
- [ ] Android TV has 6 or more instrumentation test files
- [ ] No duplicate test files remain
- [ ] HelixQA bank updated with all new test entries

---

## Phase 5: Stress and Performance Testing

**Goal:** Execute stress tests, collect monitoring metrics, optimize based on data. Ensure lazy loading, semaphores, and non-blocking patterns across the system.

### 5.1 k6 Load Test Execution

Execute all 7 existing k6 test scripts against containerized stack:

```bash
podman-compose -f docker-compose.dev.yml up -d
# Run each test
for test in load_test auth_load_test entity_browse_load_test mixed_workload_test soak_test spike_test stress_test; do
    podman run --rm --network host \
        -v $(pwd)/tests/k6:/scripts \
        docker.io/grafana/k6:latest run /scripts/${test}.js
done
```

**Metrics to collect:**
- p50, p95, p99 response times
- Error rates under load
- Max concurrent connections before degradation
- Memory usage growth over soak test duration
- CPU utilization per service

### 5.2 New Monitoring Tests

**File:** `tests/k6/monitoring_test.js` (NEW)
- Verify Prometheus metrics endpoint responds under load
- Verify metric values are accurate (request count matches actual)
- Verify no metric cardinality explosion

**File:** `tests/k6/websocket_stress_test.js` (NEW)
- 100 concurrent WebSocket connections
- Message throughput measurement
- Reconnection storm simulation
- Memory usage per connection

**File:** `tests/k6/database_stress_test.js` (NEW)
- Concurrent read/write operations
- Connection pool exhaustion behavior
- Query timeout behavior under load
- Transaction deadlock detection

**File:** `tests/k6/media_scan_stress_test.js` (NEW)
- Concurrent scan initiation
- Large directory tree scanning (10K+ files)
- Scan while serving API requests
- Memory growth during scan

### 5.3 Monitoring Dashboard Enhancement

**Grafana dashboard** (monitoring/grafana/dashboards/catalogizer-overview.json):

Add panels:
- **Goroutine count** — track goroutine leaks over time
- **Channel buffer utilization** — per named channel
- **Connection pool stats** — active, idle, waiting connections
- **Scan progress** — files scanned/second, queue depth
- **WebSocket connections** — active count, message rate
- **Memory allocation rate** — heap growth over time
- **GC pause times** — p50, p99 GC pauses
- **HTTP handler latency heatmap** — per endpoint

### 5.4 Lazy Loading Implementation Audit

**Go backend services to lazy-initialize:**

| Service | Current Init | Change |
|---------|-------------|--------|
| SubtitleService | Eager in main.go | Lazy via LazyServiceRegistry |
| ConversionService | Eager | Lazy |
| PlaylistService | Eager | Lazy |
| RecommendationService | Eager | Lazy |
| SyncService | Eager | Lazy |
| MediaAnalyzer | Eager | Lazy (start workers on first request) |

**Frontend lazy-loaded modals (React.lazy):**
- SubtitleSyncModal
- SubtitleUploadModal
- CollectionExport
- BulkOperations
- AIAnalytics, AIMetadata, AIComponents

### 5.5 Semaphore Implementation Audit

**Existing semaphore usage to verify:**
- internal/concurrency/semaphore.go — confirm used in scan paths
- Confirm max parallel scan workers are capped

**New semaphore controls:**
- Media analysis pipeline — cap concurrent TMDB/OMDB API calls (prevent rate limiting)
- File conversion pipeline — cap concurrent ffmpeg processes
- WebSocket broadcast — cap concurrent client sends

### 5.6 Performance Optimization Based on Metrics

After k6 execution, analyze and optimize:
- **Slow queries** — add indexes if EXPLAIN shows full scans
- **High memory** — reduce buffer sizes, add pooling
- **High CPU** — profile with pprof, optimize hot paths
- **Slow endpoints** — add caching via Redis for expensive computations

### Phase 5 Verification Gate

- [ ] All 7 existing k6 tests pass thresholds (p95 < 500ms)
- [ ] 4 new k6 tests created and passing
- [ ] Grafana dashboard has all new panels
- [ ] Lazy-loaded services do not initialize until first use
- [ ] Semaphore-bounded external API calls
- [ ] Soak test (30min) shows stable memory (no growth >10%)
- [ ] Spike test (sudden 10x) recovers within 30s

---

## Phase 6: Challenge Bank Expansion

**Goal:** Populate challenges_bank.json, add new challenge varieties covering all test types, integrate with test bank framework.

### 6.1 Populate challenges_bank.json

Export all 492 registered challenges from Go code into JSON format:

```json
{
  "version": "2.0.0",
  "name": "Catalogizer Challenges",
  "challenges": [
    {
      "id": "CH-001",
      "name": "Database Connectivity",
      "category": "infrastructure",
      "type": "validation",
      "severity": "critical",
      "timeout": "30s",
      "description": "Verify database connection and schema"
    }
  ]
}
```

**Categories to include:**
- infrastructure (CH-001 to CH-010)
- storage (CH-011 to CH-020)
- scanning (CH-021 to CH-030)
- entity (CH-031 to CH-040)
- api (CH-041 to CH-050)
- security-performance (CH-051 to CH-060)
- module-verification (MOD-001 to MOD-015)
- userflow-api (UF-API-001 to UF-API-049)
- userflow-web (UF-WEB-001 to UF-WEB-059)
- userflow-desktop (UF-DESK-001 to UF-DESK-028)
- userflow-mobile (UF-MOB-001 to UF-MOB-038)

### 6.2 New Challenge Categories

**Stress Challenges (STRESS-001 to STRESS-020):**
- STRESS-001: API responds under 50 concurrent users
- STRESS-002: WebSocket handles 100 concurrent connections
- STRESS-003: Database handles 1000 concurrent queries
- STRESS-004: File scan completes 10K files under 60s
- STRESS-005: Media analysis pipeline handles burst of 100 items
- STRESS-006: Redis cache handles 5000 ops/sec
- STRESS-007: Authentication handles 200 concurrent logins
- STRESS-008: Entity browser handles 100 concurrent page loads
- STRESS-009: Collection operations under 50 concurrent users
- STRESS-010: Subtitle processing handles 20 concurrent uploads
- STRESS-011: Download endpoint handles 30 concurrent streams
- STRESS-012: Search handles 100 concurrent queries
- STRESS-013: Playlist operations under 50 concurrent users
- STRESS-014: Admin endpoints under 20 concurrent users
- STRESS-015: Metrics endpoint handles 100 concurrent scrapes
- STRESS-016: Health check responds under 10ms at any load
- STRESS-017: Graceful degradation at 2x capacity
- STRESS-018: Recovery within 30s after overload
- STRESS-019: Memory stays stable under sustained load (1hr)
- STRESS-020: No goroutine leaks after 1000 request cycles

**Integration Challenges (INT-001 to INT-020):**
- INT-001: Full scan to entity creation pipeline
- INT-002: User registration to login to browse to play flow
- INT-003: Collection create to add items to export to import
- INT-004: WebSocket events fire on scan completion
- INT-005: Redis cache invalidation on data change
- INT-006: JWT refresh during active session
- INT-007: Multi-protocol scan (SMB + local) in parallel
- INT-008: Entity hierarchy build (show to seasons to episodes)
- INT-009: Metadata enrichment from multiple providers
- INT-010: User preferences persist across sessions
- INT-011: File rename detection updates entities
- INT-012: Duplicate detection merges entities correctly
- INT-013: Role-based access control enforcement
- INT-014: Error reporting captures and stores errors
- INT-015: Log management rotation and cleanup
- INT-016: Configuration wizard completes full setup
- INT-017: Subtitle upload to sync to playback
- INT-018: Playlist create to reorder to play
- INT-019: Cover art assignment to entity display
- INT-020: Stats calculation accuracy validation

**Security Challenges (SEC-001 to SEC-015):**
- SEC-001: SQL injection blocked on all string parameters
- SEC-002: XSS payloads sanitized in all text fields
- SEC-003: JWT token expiration enforced
- SEC-004: Rate limiting active on all auth endpoints
- SEC-005: CORS headers properly configured
- SEC-006: Path traversal blocked in file endpoints
- SEC-007: CSRF protection on state-changing endpoints
- SEC-008: Brute force protection on login
- SEC-009: API keys not exposed in responses
- SEC-010: HTTP security headers present (HSTS, X-Frame, etc.)
- SEC-011: Sensitive data not logged
- SEC-012: File upload validation (type, size, content)
- SEC-013: WebSocket authentication required
- SEC-014: Admin endpoints require admin role
- SEC-015: Password hashing uses bcrypt with proper cost

**Documentation Challenges (DOC-001 to DOC-010):**
- DOC-001: All API endpoints documented in OpenAPI
- DOC-002: All Go packages have godoc comments
- DOC-003: All TS libraries have JSDoc
- DOC-004: Architecture diagrams exist for all components
- DOC-005: User manual covers all features
- DOC-006: All configuration options documented
- DOC-007: All Docker Compose services documented
- DOC-008: All scripts have usage documentation
- DOC-009: CHANGELOG covers all versions
- DOC-010: Video course covers all modules

### 6.3 HelixQA Test Bank Extensions

**New bank files:**
- HelixQA/banks/stress-tests.json — stress test expectations
- HelixQA/banks/security-tests.json — security validation steps
- HelixQA/banks/documentation-tests.json — documentation completeness checks

**Extend existing banks:**
- Add negative test cases for all happy paths
- Add boundary value tests for all numeric inputs
- Add cross-platform validation steps

### Phase 6 Verification Gate

- [ ] challenges_bank.json contains all 492+ challenges
- [ ] 20 stress challenges registered and passing
- [ ] 20 integration challenges registered and passing
- [ ] 15 security challenges registered and passing
- [ ] 10 documentation challenges registered and passing
- [ ] HelixQA banks extended with new entries
- [ ] Total challenge count: 557+

---

## Phase 7: Dead Feature Activation

**Goal:** Wire all unconnected features into their applications, implement all TODO stubs.

### 7.1 Android OfflineRepository Activation

**DependencyContainer.kt:**
```kotlin
val offlineRepository: OfflineRepository by lazy {
    OfflineRepository(database, preferences)
}
```

**Wire into ViewModels:**
- HomeViewModel — use OfflineRepository when network unavailable
- SearchViewModel — search offline cache when offline
- MediaViewModel — serve cached media metadata

**Add offline detection:**
- Use ConnectivityManager.NetworkCallback for real-time network state
- Toggle between API and OfflineRepository based on connectivity

### 7.2 Android Biometric Authentication

**AuthRepository.kt — activate biometric flow:**
- Implement BiometricPrompt in LoginScreen
- Store encrypted token after successful login
- On subsequent launches, offer biometric unlock
- Respect biometricEnabled preference toggle

**New files:**
- BiometricHelper.kt — wraps AndroidX Biometric API
- Update SettingsScreen.kt — add biometric toggle

### 7.3 Android WebSocket Integration

**Wire WebSocketEvent into application:**
- Initialize OkHttp WebSocket in DependencyContainer
- Create WebSocketRepository that emits WebSocketEvent via StateFlow
- Connect to ws://SERVER/api/v1/ws with JWT auth header
- Wire into HomeViewModel for real-time scan updates
- Wire into MediaViewModel for real-time playback events

### 7.4 Panoptic Analytics Implementation

**GenerateAnalytics** (Challenges/Panoptic/internal/cloud/manager.go:1005):

Replace TODO stub with real implementation:
- Accept []TestResult, compute:
  - Total/passed/failed/skipped counts
  - Success rate percentage
  - Average execution time
  - Slowest tests (top 10)
  - Most common failure patterns
  - Trend comparison with previous runs
- Return structured AnalyticsResult (not interface{})

**SaveReport** (Challenges/Panoptic/internal/cloud/manager.go:1020):

Replace TODO stub with real implementation:
- Accept AnalyticsResult and path
- Marshal to JSON with indentation
- Write to file with atomic write (write to temp, rename)
- Include metadata (timestamp, version, host)

### 7.5 Challenge Bank Dynamic Loading

Wire challenges_bank.json into the challenge registration system:
- Add LoadFromJSON(path string) ([]challenge.Challenge, error) to challenge loader
- Register JSON-defined challenges alongside Go-defined ones
- Allow runtime challenge updates via JSON without recompilation

### Phase 7 Verification Gate

- [ ] Android offline mode works: airplane mode results in cached data displayed
- [ ] Biometric prompt appears on login screen (when enabled)
- [ ] WebSocket connection established on Android app launch
- [ ] Real-time scan updates appear in Android HomeScreen
- [ ] GenerateAnalytics() returns valid analytics (not error)
- [ ] SaveReport() writes valid JSON file
- [ ] JSON-defined challenges load and execute correctly
- [ ] All previously dead code now has active callers

---

## Phase 8: Documentation Completion

**Goal:** 100% documentation coverage — every module, component, function, type, endpoint, and configuration option documented.

### 8.1 Go Package Documentation (godoc)

**Add package-level doc comments to all packages:**
- Every `package X` declaration must have a preceding doc comment
- Every exported function/type/constant must have godoc comment
- Focus: catalog-api packages first, then all 23 submodules

### 8.2 TypeScript JSDoc

**Add JSDoc to all exported components/hooks/types:**

| Library | Files Needing JSDoc |
|---------|-------------------|
| Collection-Manager-React | CollectionCard, CollectionForm, CollectionList, SmartRuleBuilder |
| Dashboard-Analytics-React | ActivityFeed, EntityStatsGrid, MediaDistributionBar, StatsCard |
| Media-Player-React | PlayerControls, useMediaPlayer |
| Media-Browser-React | EntityCard, EntityGrid, Pagination, TypeSelector |

**Format:**
```tsx
/**
 * Displays a media entity card with thumbnail, title, and metadata.
 *
 * @param entity - The media entity to display
 * @param onClick - Callback when card is clicked
 * @param variant - Display variant: 'compact' | 'full'
 */
```

### 8.3 Android KDoc

Add class-level and public method documentation to all 29 source files in catalogizer-android:
- All ViewModels: describe state management approach
- All Repositories: describe data source strategy
- All Screens: describe UI composition
- All data classes: describe field semantics

Similarly for catalogizer-androidtv source files.

### 8.4 Architecture Diagrams

**New/Updated diagrams in docs/architecture/:**

| Diagram | Format | Content |
|---------|--------|---------|
| system-overview.mmd | Mermaid | All 7 components + data flow |
| api-request-flow.mmd | Mermaid | HTTP request to handler to service to repo to DB |
| media-pipeline.mmd | Mermaid | Scan to detect to analyze to entity creation |
| auth-flow.mmd | Mermaid | Login to JWT to refresh to role check |
| websocket-events.mmd | Mermaid | Event bus to WS handler to clients |
| android-architecture.mmd | Mermaid | MVVM layers + offline mode |
| challenge-system.mmd | Mermaid | Challenge registration to execution to reporting |
| build-pipeline.mmd | Mermaid | 7-component build orchestration |
| deployment-topology.mmd | Mermaid | Production container layout |
| database-erd.mmd | Mermaid | All tables + relationships |

### 8.5 SQL Definitions Update

**File:** docs/architecture/SQL_MIGRATIONS.md

Update to cover all 9 migration versions:
- v1-v7: existing schema (verify accuracy)
- v8: media entity tables (media_types, media_items, media_files, media_collections, etc.)
- v9: performance indexes + media_files deduplication

Include:
- Complete CREATE TABLE statements for both SQLite and PostgreSQL
- Index definitions
- Foreign key relationships
- Column descriptions
- Sample queries for common operations

### 8.6 API Documentation Update

**File:** docs/api/API_DOCUMENTATION.md

Ensure every registered endpoint is documented:
- All 256+ handler methods
- Request/response schemas (JSON)
- Authentication requirements
- Rate limiting info
- Error response codes
- Example curl commands

### 8.7 User Manuals

**Comprehensive user manual** (docs/guides/USER_MANUAL.md):

| Chapter | Content |
|---------|---------|
| 1. Getting Started | Installation, first login, initial setup |
| 2. Storage Configuration | Adding SMB/FTP/NFS/WebDAV/local storage roots |
| 3. Scanning and Detection | Initiating scans, understanding detection pipeline |
| 4. Browsing Media | Entity browser, search, filtering, sorting |
| 5. Collections | Creating, managing, smart rules, import/export |
| 6. Playback | Media player, playlists, subtitles, streaming |
| 7. Android App | Mobile-specific features, offline mode, sync |
| 8. Android TV App | TV navigation, leanback UI, voice search |
| 9. Desktop App | System tray, local scanning, Tauri features |
| 10. Administration | User management, roles, system settings |
| 11. Monitoring | Prometheus metrics, Grafana dashboards, alerts |
| 12. Troubleshooting | Common issues, FAQ, diagnostic commands |

**Platform-specific quick-start guides:**
- docs/guides/QUICKSTART_WEB.md
- docs/guides/QUICKSTART_ANDROID.md
- docs/guides/QUICKSTART_ANDROIDTV.md
- docs/guides/QUICKSTART_DESKTOP.md

### 8.8 Troubleshooting Fix

**File:** docs/TROUBLESHOOTING_GUIDE.md
- Replace placeholder +1-XXX-XXX-XXXX with actual support channel (GitHub Issues URL)

### Phase 8 Verification Gate

- [ ] go doc ./... produces output for all exported symbols
- [ ] All TS library exports have JSDoc (verified by TypeDoc or eslint-plugin-jsdoc)
- [ ] All Android source files have KDoc headers
- [ ] 10 architecture diagrams created/updated
- [ ] SQL_MIGRATIONS.md covers all 9 migration versions
- [ ] API_DOCUMENTATION.md covers all registered endpoints
- [ ] User manual covers all 12 chapters
- [ ] 4 platform quickstart guides created
- [ ] No placeholder text remaining in any documentation

---

## Phase 9: Website and Video Courses

**Goal:** Sync VitePress website with all documentation, extend video course scripts for new features.

### 9.1 Website Content Sync

**VitePress site** (Website/):

Sync content from main docs/ to website structure:

| Website Page | Source | Status |
|-------------|--------|--------|
| index.md | Project overview | Verify current |
| features.md | Feature list | Update with new features |
| getting-started.md | Installation guide | Update for v2.2.0 |
| download.md | Download links | Update with latest builds |
| documentation.md | Docs index | Sync with docs/ structure |
| faq.md | Common questions | Extend with new Q&A |
| changelog.md | Version history | Add v2.2.0 changes |
| course.md | Video course index | Update with new modules |
| support.md | Support channels | Verify links |

**Guides to update:**

| Guide | Content |
|-------|---------|
| guides/web-app.md | Full web UI walkthrough |
| guides/desktop.md | Desktop app features |
| guides/android.md | Android mobile guide |
| guides/android-tv.md | Android TV guide with D-pad navigation |
| guides/configuration.md | Configuration wizard + manual config |
| guides/security.md | Security features + scanning |
| guides/monitoring.md | Prometheus + Grafana setup |

**Developer docs to update:**

| Doc | Content |
|-----|---------|
| developer/architecture.md | System architecture with new diagrams |
| developer/api.md | API reference (sync with API_DOCUMENTATION.md) |
| developer/testing.md | Test strategy + running tests |
| developer/contributing.md | Contribution guidelines |

### 9.2 Video Course Script Extension

**File:** docs/VIDEO_COURSE_SCRIPTS.md

**Existing modules to update:**
- Module 1: Introduction — update for v2.2.0 features
- Module 2: Installation — update for dynamic container runtime
- Module 3: Configuration — add offline mode, biometric setup
- Module 4: Storage Setup — current (verify accuracy)
- Module 5: Media Management — add entity browser enhancements
- Module 6: Collections — add smart rules, import/export
- Module 7: Playback — add subtitle sync, playlist features

**New modules to add:**

| Module | Title | Duration (est.) | Content |
|--------|-------|-----------------|---------|
| 8 | Android Mobile App | 15 min | Setup, offline mode, sync, biometric auth |
| 9 | Android TV App | 12 min | D-pad navigation, voice search, channels |
| 10 | Desktop App | 10 min | System tray, local scan, Tauri IPC |
| 11 | Monitoring and Observability | 15 min | Prometheus, Grafana, alerting |
| 12 | Security and Hardening | 12 min | Security scanning, HTTPS/QUIC, JWT |
| 13 | Challenge System | 10 min | Running challenges, interpreting results |
| 14 | Development Setup | 15 min | Dev environment, testing, contributing |
| 15 | API Integration | 12 min | API client, WebSocket, authentication |
| 16 | Deployment and Operations | 15 min | Production deployment, backup, scaling |
| 17 | Troubleshooting | 10 min | Common issues, diagnostics, recovery |

**Each module script includes:**
- Learning objectives
- Prerequisites
- Step-by-step narration script
- Screen recording instructions
- Key timestamps
- Summary and next steps

### 9.3 Website Build Verification

```bash
cd Website && npm install && npm run build
```

Verify:
- All internal links resolve
- All images load
- Navigation works correctly
- Search functionality works
- Mobile responsive layout

### Phase 9 Verification Gate

- [ ] Website builds with zero warnings
- [ ] All pages have current content (no stale v1.x references)
- [ ] All internal links resolve (no 404s)
- [ ] Video course scripts cover all 17 modules
- [ ] Each guide covers its platform completely
- [ ] Developer docs match current codebase
- [ ] Changelog includes v2.2.0 entry

---

## Phase 10: Final Verification and Release Prep

**Goal:** Full end-to-end validation of everything. Zero broken, disabled, undocumented, or unfinished items.

### 10.1 Full Test Suite Execution

```bash
# Go backend (resource-limited)
cd catalog-api && GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -count=1

# Frontend
cd catalog-web && npm run test && npm run test:e2e

# API Client
cd Catalogizer-API-Client-TS && npm run test

# Desktop
cd catalogizer-desktop && npm run test

# Installer
cd installer-wizard && npm run test

# Android (if JDK 17 available)
cd catalogizer-android && ./gradlew test
cd catalogizer-androidtv && ./gradlew test
```

### 10.2 Challenge Suite Execution

```bash
# Start services
podman-compose -f docker-compose.dev.yml up -d

# Run all challenges via API
curl -X POST http://localhost:8080/api/v1/challenges/run-all \
  -H "Authorization: Bearer $TOKEN"
```

Verify all 557+ challenges pass.

### 10.3 Build Pipeline Verification

```bash
./scripts/release-build.sh --container --force --skip-tests
```

All 7 components must build successfully.

### 10.4 Security Re-scan

Run all security tools one final time:
- SonarQube: 0 critical/high
- govulncheck: 0 vulnerabilities
- npm audit: 0 critical/high
- Semgrep: 0 critical findings

### 10.5 Documentation Completeness Checklist

- [ ] Every Go package has godoc
- [ ] Every TS export has JSDoc
- [ ] Every Android class has KDoc
- [ ] Every API endpoint documented
- [ ] Every configuration option documented
- [ ] Every Docker Compose service documented
- [ ] Every script has header comment explaining usage
- [ ] Architecture diagrams cover all components
- [ ] User manual covers all features
- [ ] Video course covers all topics
- [ ] Website content is current

### 10.6 Zero-Warning Verification

- [ ] Go: go vet ./... clean
- [ ] Go: go build ./... zero warnings
- [ ] Frontend: npm run lint zero warnings (--max-warnings 0)
- [ ] Frontend: npm run type-check zero errors
- [ ] Frontend: Browser console zero errors/warnings
- [ ] Android: ./gradlew lint zero critical warnings
- [ ] All API endpoints return valid responses (no 5xx)
- [ ] Zero failed network requests in browser

### 10.7 Version Bump

- Update versions.json (Build framework) to v2.2.0
- Update all package.json files to v2.2.0
- Update Android versionCode and versionName
- Update CLAUDE.md with current metrics

### Phase 10 Verification Gate

- [ ] ALL tests pass (Go, Frontend, Desktop, Installer, Android, Android TV)
- [ ] ALL 557+ challenges pass
- [ ] ALL 7 components build in container
- [ ] ALL security scans clean
- [ ] ALL documentation complete
- [ ] Zero warnings across all components
- [ ] Version bumped to v2.2.0
- [ ] CLAUDE.md and AGENTS.md updated

---

## Cross-Cutting Concerns

### Resource Limits (Mandatory)

All work must respect the 30-40% host resource limit:
- Go tests: GOMAXPROCS=3 go test ./... -p 2 -parallel 2
- Container limits per CLAUDE.md specifications
- No parallel challenge execution
- Monitor: podman stats --no-stream and cat /proc/loadavg

### Non-Interactive Execution

**No process may request interactive input (sudo, password, etc.):**
- All scripts must use non-interactive flags
- Container operations must not require TTY
- Build scripts must handle missing dependencies gracefully (warn and skip)

### Git Constraints

- No GitHub Actions workflow files
- Push to all 6 remotes after completion
- Releases and reports are gitignored
- Submodule changes committed independently

### Existing Functionality Protection

**Every change must be verified to not break existing behavior:**
- Run full test suite before and after each phase
- Use feature flags for gradual activation if needed
- Rollback plan: git revert to pre-phase commit

---

## Verification Gates Summary

| Phase | Key Metric | Pass Criteria |
|-------|-----------|---------------|
| 1 | Policy compliance | Zero violations |
| 2 | Safety | Zero races, leaks, deadlocks |
| 3 | Security | Zero critical/high findings |
| 4 | Test coverage | >85% across all modules |
| 5 | Performance | p95 < 500ms, stable memory |
| 6 | Challenges | 557+ registered and passing |
| 7 | Feature completeness | Zero dead features |
| 8 | Documentation | 100% coverage |
| 9 | Website | Builds, all links resolve |
| 10 | Release readiness | Everything passes |

---

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| Android Compose BOM upgrade breaks UI | HIGH | Test on device before committing |
| SonarQube scan reveals 100+ issues | MEDIUM | Prioritize critical/high, batch medium/low |
| k6 stress test reveals performance cliff | HIGH | Profile with pprof, optimize hot paths |
| Biometric API differs across Android versions | MEDIUM | Use AndroidX Biometric (handles compat) |
| WebSocket Android integration causes battery drain | MEDIUM | Implement connection pooling, heartbeat optimization |
| Panoptic analytics implementation complex | LOW | Already has full executor pipeline, just needs data aggregation |
| Website content diverges from docs | LOW | Automated sync script or shared content source |
| Go 1.25 alignment breaks submodule builds | LOW | Test each submodule individually |
