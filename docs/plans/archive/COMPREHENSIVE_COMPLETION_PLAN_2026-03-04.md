> **SUPERSEDED** — This document was a planning artifact from an earlier phase. The work described here has been completed or superseded by the [2026-03-26 Project Completion Plan](../../superpowers/plans/2026-03-26-project-completion-plan.md). Retained for historical reference.

---

# Catalogizer Comprehensive Completion Plan

**Date:** 2026-03-04
**Scope:** Full project audit, unfinished work inventory, and phased implementation plan
**Objective:** Zero unfinished, undocumented, broken, or disabled items across all 7 components + infrastructure

---

## PART 1: COMPREHENSIVE AUDIT — CURRENT STATE

### 1.1 Overall Health Summary

| Component | Build | Tests | Coverage | Status |
|-----------|-------|-------|----------|--------|
| catalog-api (Go) | PASS | 42 packages pass | 30.7% avg | Production |
| catalog-web (React) | PASS | 1,623 tests pass | ~45% | Production |
| catalog-web E2E | PASS | 314 tests (25 specs) | N/A | Production |
| catalogizer-desktop | PASS | 15 test files | ~36% | Production |
| installer-wizard | PASS | 178 tests | ~42% | Production |
| catalogizer-android | PASS | 45 test files | ~78% | Production |
| catalogizer-androidtv | PASS | 27 test files | ~80% | Production |
| catalogizer-api-client | PASS | 7 test files | ~37% | Production |
| WebSocket-Client-TS | PASS | 4 test files | ~18% | Production |
| UI-Components-React | PASS | 11 test files | ~26% | Production |

### 1.2 Test Results (Verified 2026-03-04)

**Go Backend:** All 42 packages pass (5 [no test files] packages: `cmd/boot`, `internal/cache`, `internal/eventbus`, `internal/media`, `tests/testutils`)

**Known Test Skips (by design):**
- Filesystem integration tests (FTP, NFS, WebDAV, SMB) — require live protocol servers
- NFS tests — require root privileges for kernel module
- Protocol rename tests — FTP/NFS/WebDAV client implementations pending
- Automation/stress tests — skipped in `-short` mode
- User flow tests — skip when no auth token available

### 1.3 Coverage Gaps (Go Backend — Packages Below 50%)

| Package | Current | Target | Gap |
|---------|---------|--------|-----|
| `challenges` | 4.2% | 60% | +55.8% |
| `smb` | 16.7% | 60% | +43.3% |
| `services` | 24.6% | 60% | +35.4% |
| `filesystem` | 29.4% | 60% | +30.6% |
| `handlers` | 30.4% | 60% | +29.6% |
| `internal/services` | 30.3% | 60% | +29.7% |
| `internal/handlers` | 31.0% | 60% | +29.0% |
| `internal/media/realtime` | 31.0% | 60% | +29.0% |
| `database` | 40.4% | 60% | +19.6% |
| `internal/media/analyzer` | 40.7% | 60% | +19.3% |
| `repository` | 48.6% | 60% | +11.4% |

### 1.4 Packages With No Test Files

| Package | Type | Action Required |
|---------|------|----------------|
| `cmd/boot` | Entry point | Add boot sequence tests |
| `internal/cache` | Caching | Add cache operation tests |
| `internal/eventbus` | Event system | Add event pub/sub tests |
| `internal/media` | Media namespace | Add package-level tests |
| `tests/testutils` | Test helpers | No tests needed (helper package) |

### 1.5 Dead Code / Unconnected Features

#### 14 Unwired Go Submodules
These submodules exist at the project root with full code, tests, and docs but are NOT imported by catalog-api via `replace` directives. The CLAUDE.md and Decoupling Plan confirm this is by design — they were extracted for reusability but catalog-api uses its own internal implementations:

| Submodule | Status | Notes |
|-----------|--------|-------|
| Database/ | Unwired | catalog-api uses own `database/` package |
| Discovery/ | Unwired | catalog-api uses own `internal/services/` |
| Media/ | Unwired | catalog-api uses own `internal/media/` |
| Middleware/ | Unwired | catalog-api uses own `middleware/` |
| Observability/ | Unwired | catalog-api uses own `internal/metrics/` |
| RateLimiter/ | Unwired | catalog-api uses own `middleware/` |
| Security/ | Unwired | catalog-api uses own `internal/auth/` |
| Storage/ | Unwired | catalog-api uses own `internal/services/` |
| Streaming/ | Unwired | catalog-api uses own `internal/services/` |
| Watcher/ | Unwired | catalog-api uses own `internal/media/realtime/` |
| Auth-Context-React/ | Linked | Used by catalog-web via `file:../` |
| Media-Browser-React/ | Linked | Used by catalog-web via `file:../` |
| Media-Player-React/ | Linked | Used by catalog-web via `file:../` |
| Collection-Manager-React/ | Linked | Used by catalog-web via `file:../` |
| Dashboard-Analytics-React/ | Linked | Used by catalog-web via `file:../` |
| Media-Types-TS/ | Linked | Used by catalog-web via `file:../` |
| Catalogizer-API-Client-TS/ | Linked | Used by catalog-web via `file:../` |

**Verdict:** The 10 unwired Go submodules are intentionally standalone libraries. They are not dead code — they are available for future consumers. No action required beyond ensuring they build and pass their own tests.

#### Protocol Client Implementations Pending
From `tests/integration/protocol_rename_tests.go`:
- FTP rename detection: `t.Skip("FTP client implementation pending")`
- NFS rename detection: `t.Skip("NFS client implementation pending")`
- WebDAV rename detection: `t.Skip("WebDAV client implementation pending")`

These represent incomplete protocol-specific rename tracking features.

#### console.error Statements (Frontend)
37 `console.error()` calls in production frontend code. These are legitimate error logging in catch blocks but should be replaced with a structured logging service for production.

### 1.6 Known Issues from Final Report (2026-02-23)

1. **SMB test timing**: Minor non-deterministic goroutine cleanup timing under race detector — no actual race, passes on retry
2. **NFS test container**: Requires root for `modprobe nfs` — NFS integration tests skipped without root

### 1.7 Security Scanning Status

| Tool | Last Run | Status | Issues |
|------|----------|--------|--------|
| govulncheck | 2026-02-23 | PASS | 0 vulnerabilities |
| npm audit | 2026-02-23 | PASS | 0 critical/production |
| Snyk | 2026-02-10 | PASS | Reports in `docs/security/` |
| SonarQube | Configured | Available | `docker-compose.security.yml` ready |
| gosec | Available | Script ready | `scripts/gosec-scan.sh` |

### 1.8 Documentation Inventory

| Category | Files | Status |
|----------|-------|--------|
| Architecture docs | 22 | Complete |
| ADRs | 6 | Complete |
| API documentation | 2 | Complete |
| Testing docs | 25+ | Complete |
| Deployment docs | 15+ | Complete |
| User guides | 20+ | Complete |
| Tutorials | 5 | Complete |
| Diagrams | 4 + images | Complete |
| Video course scripts | 6 modules | Scripts + slides written |
| Video course recordings | 0 | NOT RECORDED |
| Website pages | 8 | Built (VitePress) |
| Status reports | 25+ | Complete |
| Security docs | 5+ | Complete |

### 1.9 Monitoring & Metrics Status

| Component | Status |
|-----------|--------|
| Prometheus config | Configured (`monitoring/prometheus.yml`) |
| Grafana dashboards | Configured (`monitoring/grafana/`) |
| /metrics endpoint | Active (Prometheus client) |
| /api/v1/health | Active |
| Memory leak detector | Implemented (`pkg/memory/`) |
| Semaphore package | Implemented (`pkg/semaphore/`) |
| Lazy loading | Implemented (`pkg/lazy/`) |

---

## PART 2: UNFINISHED ITEMS INVENTORY

### Category A: Test Coverage Gaps

| ID | Item | Component | Priority |
|----|------|-----------|----------|
| A1 | Go backend overall coverage 30.7% → target 60%+ | catalog-api | HIGH |
| A2 | 5 packages with no test files | catalog-api | HIGH |
| A3 | `challenges` package at 4.2% coverage | catalog-api | HIGH |
| A4 | `services` package at 24.6% coverage | catalog-api | MEDIUM |
| A5 | `filesystem` package at 29.4% coverage | catalog-api | MEDIUM |
| A6 | `handlers` package at 30.4% coverage | catalog-api | MEDIUM |
| A7 | WebSocket-Client-TS at ~18% test ratio | WebSocket-Client-TS | MEDIUM |
| A8 | UI-Components-React at ~26% test ratio | UI-Components-React | MEDIUM |
| A9 | catalogizer-api-client at ~37% test ratio | catalogizer-api-client | MEDIUM |
| A10 | No dedicated stress tests for frontend | catalog-web | LOW |

### Category B: Missing Tests Types

| ID | Item | Component | Priority |
|----|------|-----------|----------|
| B1 | No contract/consumer tests for API client ↔ API | Cross-component | MEDIUM |
| B2 | No mutation testing configured | All | LOW |
| B3 | No fuzz testing for parsers (title_parser, media detector) | catalog-api | MEDIUM |
| B4 | No snapshot tests for React components | catalog-web | LOW |
| B5 | No accessibility tests (a11y) automated | catalog-web | MEDIUM |
| B6 | Protocol rename detection tests permanently skipped (FTP/NFS/WebDAV) | catalog-api | LOW |
| B7 | No chaos/fault-injection tests | catalog-api | LOW |
| B8 | No load testing with sustained traffic patterns | catalog-api | MEDIUM |

### Category C: Code Quality & Safety

| ID | Item | Component | Priority |
|----|------|-----------|----------|
| C1 | 37 console.error calls in production frontend (no structured logging) | catalog-web | MEDIUM |
| C2 | SMB test timing flake under race detector | catalog-api | LOW |
| C3 | Snyk scan reports dated 2026-02-10 (24 days old) | All | MEDIUM |
| C4 | SonarQube not recently run (compose ready but no recent report) | All | MEDIUM |
| C5 | No automated security scanning in CI pipeline (GitHub Actions disabled) | All | LOW |

### Category D: Documentation Gaps

| ID | Item | Component | Priority |
|----|------|-----------|----------|
| D1 | Video course recordings not produced | docs/courses | MEDIUM |
| D2 | Build/ submodule not documented in main CLAUDE.md (FIXED 2026-03-04) | Root | DONE |
| D3 | v3.0 Roadmap features listed as "In Planning" need status update | docs/roadmap | LOW |
| D4 | No API changelog documenting endpoint evolution | docs/api | LOW |
| D5 | Submodule integration decision rules not in main docs | docs | LOW |
| D6 | Challenge system lacks end-user-facing documentation | docs/guides | MEDIUM |
| D7 | No troubleshooting guide for common container build failures | docs | MEDIUM |
| D8 | Website changelog.md needs updating with latest work | Website | MEDIUM |

### Category E: Performance & Optimization

| ID | Item | Component | Priority |
|----|------|-----------|----------|
| E1 | No formal performance baseline established | catalog-api | MEDIUM |
| E2 | No response time SLA monitoring | catalog-api | LOW |
| E3 | No frontend bundle size monitoring/budget | catalog-web | LOW |
| E4 | No database query performance monitoring tests | catalog-api | MEDIUM |
| E5 | Lazy initialization not applied to all services at startup | catalog-api | LOW |

### Category F: Challenge System Gaps

| ID | Item | Component | Priority |
|----|------|-----------|----------|
| F1 | Challenge coverage test (verify all features have challenges) | catalog-api | MEDIUM |
| F2 | No challenge for security scanning validation | catalog-api | LOW |
| F3 | No challenge for performance regression detection | catalog-api | MEDIUM |
| F4 | No challenge for documentation completeness validation | catalog-api | LOW |

---

## PART 3: PHASED IMPLEMENTATION PLAN

### Phase 0: Security Scanning & Safety Verification (Estimated: 1 session)

**Objective:** Run all available security scanners, analyze findings, resolve issues.

**Steps:**
1. Run SonarQube scan via `docker-compose.security.yml`
   - `podman-compose -f docker-compose.security.yml up -d`
   - Wait for SonarQube to be ready on port 9000
   - Execute `./scripts/sonarqube-scan.sh`
   - Download and analyze report
2. Run Snyk scan via `./scripts/snyk-scan.sh`
   - Update all 5 component reports (Go, web, desktop, installer, api-client)
   - Analyze new findings since 2026-02-10
3. Run govulncheck on latest Go dependencies
   - `cd catalog-api && govulncheck ./...`
4. Run npm audit on all Node.js components
   - `cd catalog-web && npm audit --production`
   - `cd catalogizer-desktop && npm audit --production`
   - `cd installer-wizard && npm audit --production`
   - `cd catalogizer-api-client && npm audit --production`
5. Run gosec static analysis
   - `./scripts/gosec-scan.sh`
6. Analyze all findings, triage by severity
7. Fix all CRITICAL and HIGH findings immediately
8. Document all findings and resolutions in `docs/security/SECURITY_SCAN_REPORT_2026-03-04.md`

**Deliverables:**
- Updated Snyk reports in `docs/security/`
- SonarQube quality gate report
- govulncheck verification
- npm audit clean results
- gosec clean results
- Security scan report document

**Constraints:**
- Do NOT run any command requiring root/sudo
- All scanning via containers with resource limits (max 4 CPUs, 8 GB RAM)
- Use `podman-compose` (not docker-compose)
- Use `--network host` for container builds

---

### Phase 1: Test Coverage Expansion — Go Backend (Estimated: 3-4 sessions)

**Objective:** Raise Go backend coverage from 30.7% to 60%+ overall, eliminate all no-test-file packages.

**Steps:**

#### 1.1 Add Tests for No-Test-File Packages
- `cmd/boot/boot_test.go` — Test boot sequence initialization
- `internal/cache/cache_test.go` — Test cache get/set/evict/TTL
- `internal/eventbus/eventbus_test.go` — Test publish/subscribe/unsubscribe
- `internal/media/media_test.go` — Test package-level utilities

#### 1.2 Increase `challenges` Coverage (4.2% → 60%+)
- Test challenge registration completeness (all 209 registered)
- Test challenge configuration loading
- Test challenge result serialization
- Test userflow challenge factory functions
- Test challenge bank loading from JSON/YAML

#### 1.3 Increase `services` Coverage (24.6% → 60%+)
- Add unit tests for ConversionService
- Add unit tests for RecommendationService
- Add unit tests for SubtitleService
- Add unit tests for ConfigurationWizardService
- Add unit tests for MusicPlayerService
- Add unit tests for VideoPlayerService
- Use mock database connections (sqlmock)

#### 1.4 Increase `handlers` Coverage (30.4% → 60%+)
- Add HTTP handler tests using httptest.NewRecorder
- Test all handler error paths
- Test input validation
- Test response shapes and status codes

#### 1.5 Increase `internal/services` Coverage (30.3% → 60%+)
- Add unit tests for AggregationService
- Add unit tests for UniversalScanner (mocked filesystem)
- Add unit tests for TitleParser edge cases
- Add unit tests for RenameTracker

#### 1.6 Increase `filesystem` Coverage (29.4% → 60%+)
- Add unit tests for factory.go
- Add unit tests for local_client.go
- Expand mock-based protocol client tests (without live servers)

#### 1.7 Increase `database` Coverage (40.4% → 60%+)
- Test dialect rewriting edge cases
- Test migration execution (both SQLite and PostgreSQL variants)
- Test InsertReturningID with both dialects
- Test connection pooling configuration

#### 1.8 Increase `repository` Coverage (48.6% → 60%+)
- Add tests for MediaItemRepository hierarchy queries
- Add tests for AnalyticsRepository aggregation
- Add tests for FavoritesRepository

**Constraints:**
- All tests must use `database.WrapDB(sqlDB, DialectSQLite)` for in-memory testing
- No live infrastructure required (mock everything)
- Resource-limited: `GOMAXPROCS=3 go test ./... -p 2 -parallel 2`
- Tests must pass with `-race` flag
- Follow table-driven test pattern

**Deliverables:**
- 0 packages with [no test files] (except tests/testutils, tests/mocks, tests/manual)
- Overall coverage 60%+
- All tests pass with `-race` flag
- Coverage report in `docs/testing/COVERAGE_REPORT_2026-03.md`

---

### Phase 2: Test Coverage Expansion — Frontend & Libraries (Estimated: 2-3 sessions)

**Objective:** Increase test coverage for TypeScript/React components.

**Steps:**

#### 2.1 WebSocket-Client-TS (~18% → 50%+)
- Add tests for reconnection logic with exponential backoff
- Add tests for message queuing during offline
- Add tests for channel subscription/unsubscription
- Add tests for React hooks (useWebSocket, useChannel, useMessage)
- Add tests for type-safe message generics

#### 2.2 UI-Components-React (~26% → 50%+)
- Add tests for all form components (input, select, checkbox, radio)
- Add tests for layout components (grid, stack, container)
- Add tests for feedback components (toast, alert, spinner)
- Add tests for accessibility (aria attributes, keyboard navigation)

#### 2.3 catalogizer-api-client (~37% → 50%+)
- Add tests for all API service methods
- Add tests for error handling and retry logic
- Add tests for HTTP/3 fallback behavior
- Add tests for WebSocket utilities

#### 2.4 catalog-web Structured Logging
- Replace 37 `console.error()` calls with a logging service
- Create `src/lib/logger.ts` with structured log levels
- Ensure ErrorBoundary uses logger
- Ensure all catch blocks use logger

#### 2.5 Frontend Accessibility Tests
- Add axe-core integration to Vitest
- Test all interactive components for WCAG 2.1 AA compliance
- Test keyboard navigation flows
- Test screen reader compatibility

**Deliverables:**
- All library test ratios above 50%
- Zero `console.error` in production code (replaced with logger)
- Accessibility test suite
- Coverage reports per component

---

### Phase 3: Advanced Test Types (Estimated: 2-3 sessions)

**Objective:** Add missing test categories to the test bank.

**Steps:**

#### 3.1 Fuzz Testing (Go)
- Add fuzz tests for `internal/services/title_parser.go`
  - `FuzzParseMovieTitle`
  - `FuzzParseTVShowTitle`
  - `FuzzParseMusicTitle`
- Add fuzz tests for `internal/media/detector/` pattern matching
- Add fuzz tests for `database/dialect.go` SQL rewriting

#### 3.2 Contract Tests
- Add API contract tests validating catalog-api responses match catalogizer-api-client type definitions
- Test all REST endpoints return shapes matching TypeScript interfaces
- Validate WebSocket event payloads match TS event types

#### 3.3 Performance Baseline Tests
- Establish response time baselines for all critical endpoints:
  - `GET /api/v1/health` < 10ms
  - `GET /api/v1/catalog/files` < 100ms
  - `GET /api/v1/entities` < 100ms
  - `POST /api/v1/auth/login` < 200ms
  - `GET /api/v1/scans/:id/status` < 50ms
- Store baselines in `tests/performance/baselines.json`
- Add regression detection (fail if >20% slower)

#### 3.4 Load & Stress Tests Enhancement
- Enhance `tests/stress/api_load_test.go` with sustained traffic patterns (5-minute runs)
- Add graduated load testing (ramp from 10 to 500 concurrent users)
- Add memory monitoring during stress tests via `pkg/memory/`
- Add goroutine count monitoring during stress tests
- Add database connection pool monitoring during stress tests

#### 3.5 Integration Tests Enhancement
- Add multi-service integration test (API + WebSocket + DB)
- Add end-to-end scan → aggregate → browse flow test
- Add auth token refresh flow integration test

#### 3.6 Chaos/Fault-Injection Tests
- Test API behavior when database is unavailable
- Test API behavior when Redis is unavailable
- Test WebSocket recovery when connection drops
- Test scanner behavior when filesystem becomes read-only

**Deliverables:**
- Fuzz test suite (3+ fuzz targets)
- Contract test suite
- Performance baseline JSON and regression tests
- Enhanced stress test suite (sustained + graduated)
- Integration test suite enhancement
- Chaos test suite

---

### Phase 4: Challenge System Expansion (Estimated: 1-2 sessions)

**Objective:** Add new challenges validating security, performance, and completeness.

**Steps:**

#### 4.1 Security Validation Challenges
- CH-036: Verify all API endpoints require authentication (except /health, /login, /register)
- CH-037: Verify JWT token expiration is enforced
- CH-038: Verify rate limiting is active on auth endpoints
- CH-039: Verify CORS headers are correctly set
- CH-040: Verify no sensitive data in API error responses

#### 4.2 Performance Regression Challenges
- CH-041: Health endpoint responds < 10ms
- CH-042: File listing endpoint responds < 200ms under 50 concurrent requests
- CH-043: Entity search responds < 500ms for 10,000+ entities
- CH-044: WebSocket message broadcast latency < 50ms

#### 4.3 Documentation Completeness Challenges
- CH-045: Verify all API endpoints are documented in API_DOCUMENTATION.md
- CH-046: Verify all database tables are documented in DATABASE_SCHEMA.md
- CH-047: Verify all configuration options are documented

#### 4.4 System Resilience Challenges
- CH-048: API continues serving after database restart
- CH-049: Scanner recovers from temporary filesystem unavailability
- CH-050: WebSocket reconnects after server restart

**Deliverables:**
- 15 new challenges (CH-036 to CH-050)
- All challenges registered in `register.go`
- Challenge bank config files in `challenges/config/`
- All 224 challenges (209 + 15) passing

---

### Phase 5: Safety Hardening (Estimated: 2 sessions)

**Objective:** Comprehensive memory leak, deadlock, and race condition prevention.

**Steps:**

#### 5.1 Memory Leak Prevention
- Audit all goroutine launches (30+ files) — verify each has context cancellation or WaitGroup tracking
- Add goroutine leak detection test that monitors `runtime.NumGoroutine()` before and after each test
- Verify all `time.NewTicker` calls have corresponding `ticker.Stop()` in defer
- Verify all channel-based patterns have proper close semantics
- Add memory growth tests using `pkg/memory/leak_detector.go`

#### 5.2 Deadlock Prevention
- Audit all mutex usage (170+ files) for lock ordering consistency
- Verify no nested lock acquisitions across different mutexes
- Add deadlock detection test with `-timeout 30s` flag
- Review channel operations for potential blocking on full/empty channels

#### 5.3 Race Condition Prevention
- Run full test suite with `-race` flag: `GOMAXPROCS=3 go test -race ./... -p 2 -parallel 2`
- Fix any remaining race detector findings
- Add concurrent access tests for all shared-state components:
  - WebSocket client map
  - Cache service
  - Rate limiter buckets
  - Scanner active scans map
  - Media analyzer cache

#### 5.4 Lazy Loading & Non-Blocking Improvements
- Audit service initialization order in `main.go` — apply lazy initialization where safe
- Ensure all database queries use context with timeouts
- Ensure all HTTP client calls use context with timeouts
- Verify all file operations use non-blocking patterns where possible
- Apply semaphore pattern (`pkg/semaphore/`) for resource-bounded operations

#### 5.5 Frontend Safety
- Audit all `useEffect` hooks for proper cleanup functions
- Verify all event listeners are removed on unmount
- Verify all subscriptions are unsubscribed on unmount
- Verify all timers are cleared on unmount
- Add React strict mode double-render testing

**Deliverables:**
- Memory leak test suite
- Deadlock detection tests
- Race condition tests (full `-race` pass)
- Lazy initialization audit report
- Frontend cleanup audit report
- Zero race conditions, zero memory leaks, zero deadlocks

---

### Phase 6: Monitoring, Metrics & Optimization (Estimated: 1-2 sessions)

**Objective:** Production-ready monitoring with optimization based on collected metrics.

**Steps:**

#### 6.1 Metrics Collection Tests
- Add test verifying all Prometheus metrics are properly registered
- Add test verifying /metrics endpoint returns valid Prometheus format
- Add test verifying metric labels are correct
- Add test verifying histogram bucket boundaries are appropriate

#### 6.2 Performance Optimization
- Profile top 10 most-called API endpoints with `pprof`
- Identify and optimize N+1 query patterns in repository layer
- Add database query explain-plan tests for complex queries
- Optimize response serialization (pre-allocate JSON buffers)
- Verify Brotli compression ratio is optimal

#### 6.3 Frontend Performance
- Verify all route-level code splitting is active
- Verify React.lazy is used for heavy components (already in LazyComponents.tsx)
- Add bundle size monitoring (track vendor chunk sizes)
- Verify image lazy loading
- Verify WebSocket reconnection uses exponential backoff

#### 6.4 Monitoring Dashboard Validation
- Verify Grafana dashboards display all key metrics
- Add dashboard for:
  - API latency percentiles (p50, p95, p99)
  - Error rate by endpoint
  - Database connection pool utilization
  - WebSocket active connections
  - Memory and goroutine counts
  - Cache hit/miss ratio

**Deliverables:**
- Metrics test suite
- pprof-based optimization report
- Frontend bundle analysis
- Updated Grafana dashboards
- Performance optimization summary

---

### Phase 7: Documentation Completion (Estimated: 2-3 sessions)

**Objective:** Complete, extend, and update all documentation to cover every feature.

**Steps:**

#### 7.1 Update Existing Documentation
- Update `docs/roadmap/CATALOGIZER_ROADMAP_V3.md` — mark completed features, update statuses
- Update `Website/changelog.md` with all work since last update
- Update `docs/architecture/DATABASE_SCHEMA.md` with v9 migration changes
- Update `docs/architecture/SQL_COMPLETE_SCHEMA.md` with latest tables
- Update `docs/architecture/SQL_MIGRATIONS.md` with v8 and v9 details
- Update `docs/testing/TESTING_GUIDE.md` with new test types (fuzz, contract, chaos)
- Update `docs/api/API_DOCUMENTATION.md` with any new endpoints

#### 7.2 New Documentation
- Create `docs/guides/CHALLENGE_USER_GUIDE.md` — end-user guide for running challenges
- Create `docs/guides/CONTAINER_BUILD_TROUBLESHOOTING.md` — common Podman build issues and fixes
- Create `docs/api/API_CHANGELOG.md` — endpoint evolution history
- Create `docs/testing/FUZZ_TESTING_GUIDE.md` — how to write and run fuzz tests
- Create `docs/testing/STRESS_TESTING_GUIDE.md` — how to run load/stress tests
- Create `docs/testing/CONTRACT_TESTING_GUIDE.md` — API contract testing approach
- Create `docs/architecture/LAZY_LOADING_PATTERNS.md` — lazy loading implementation guide
- Create `docs/architecture/CONCURRENCY_SAFETY_GUIDE.md` — mutex, semaphore, and channel patterns

#### 7.3 Extend Diagrams
- Update `docs/diagrams/ARCHITECTURE_DIAGRAM.md` with challenge system and userflow
- Update `docs/diagrams/COMPONENT_DIAGRAM.md` with Build/ submodule
- Update `docs/diagrams/ER_DIAGRAM.md` with v8/v9 tables
- Update `docs/diagrams/SEQUENCE_DIAGRAMS.md` with scan→aggregate→browse flow
- Add new diagram: `docs/diagrams/SECURITY_ARCHITECTURE.md` — auth flow, JWT lifecycle, rate limiting

#### 7.4 Update SQL Definitions
- Consolidate and verify `docs/architecture/SQL_COMPLETE_SCHEMA.md` matches actual migrations
- Document all indexes (v9 performance migration)
- Document all foreign key relationships
- Document all trigger definitions (if any)

#### 7.5 Extend Video Course Materials
- Update Module 1 script with latest installation steps (container build pipeline)
- Update Module 3 script with media entity system details
- Update Module 5 script with challenge system usage
- Update Module 6 script with new test types and safety patterns
- Add Module 7: `docs/courses/scripts/MODULE_7_SECURITY_AND_MONITORING.md` — security scanning, monitoring setup
- Add Module 7 slides: `docs/courses/slides/MODULE_7_SLIDES.md`
- Add Module 8: `docs/courses/scripts/MODULE_8_ADVANCED_FEATURES.md` — lazy loading, WebSocket, performance
- Add Module 8 slides: `docs/courses/slides/MODULE_8_SLIDES.md`
- Update `docs/courses/COURSE_OUTLINE.md` with new modules

#### 7.6 Website Updates
- Update `Website/features.md` with challenge system, userflow automation, security hardening
- Update `Website/changelog.md` with comprehensive version history
- Update `Website/faq.md` with new common questions (challenge system, monitoring, security scanning)
- Update `Website/documentation.md` with links to new guides
- Rebuild website: `cd Website && npm run build`

**Deliverables:**
- All existing docs updated with latest information
- 8+ new documentation files
- Updated diagrams (5 files)
- Updated SQL schema documentation
- 2 new video course modules (Module 7 & 8)
- Updated website (all 8 pages)

---

### Phase 8: Final Verification & Integration (Estimated: 1 session)

**Objective:** Validate everything works together, all tests pass, all docs are accurate.

**Steps:**

#### 8.1 Full Test Suite Execution
```bash
# Go backend (resource-limited)
cd catalog-api && GOMAXPROCS=3 go test -race ./... -p 2 -parallel 2 -cover

# Frontend unit tests
cd catalog-web && npm run test

# Frontend E2E tests
cd catalog-web && npm run test:e2e

# Installer wizard tests
cd installer-wizard && npm run test

# Desktop tests
cd catalogizer-desktop && npm run test

# API client tests
cd catalogizer-api-client && npm run test

# Android tests (if JDK 17 available)
cd catalogizer-android && ./gradlew test
cd catalogizer-androidtv && ./gradlew test

# Lint + typecheck
cd catalog-web && npm run lint && npm run type-check
```

#### 8.2 Security Scan Rerun
```bash
cd catalog-api && govulncheck ./...
cd catalog-web && npm audit --production
./scripts/snyk-scan.sh
./scripts/sonarqube-scan.sh
```

#### 8.3 Build Verification
```bash
cd catalog-api && go build -o catalog-api
cd catalog-web && npm run build
cd catalogizer-api-client && npm run build
```

#### 8.4 Challenge Suite Execution
- Start catalog-api service
- Run all 224 challenges via `/api/v1/challenges/run-all`
- Verify 224/224 PASSED

#### 8.5 Documentation Cross-Reference Validation
- Verify all internal links in documentation resolve
- Verify all code examples in docs compile/run
- Verify all CLI commands in docs execute correctly
- Verify Website builds without errors

#### 8.6 Final Verification Checklist
- [ ] `go build ./...` succeeds with zero errors
- [ ] `go test -race ./...` passes with zero failures
- [ ] `go vet ./...` reports zero issues
- [ ] `npm run build` succeeds in catalog-web
- [ ] `npm run test` passes in catalog-web (1,623+ tests)
- [ ] `npm run lint && npm run type-check` pass in catalog-web
- [ ] `npm run test` passes in installer-wizard (178+ tests)
- [ ] `npm run test` passes in catalogizer-desktop
- [ ] `npm run test` passes in catalogizer-api-client
- [ ] govulncheck reports 0 vulnerabilities
- [ ] npm audit reports 0 critical production vulnerabilities
- [ ] SonarQube quality gate passes
- [ ] All 224+ challenges pass
- [ ] Overall Go test coverage 60%+
- [ ] Zero TODO/FIXME/HACK in production code (verified: 0 found)
- [ ] Zero disabled or skipped tests (except infrastructure-dependent ones)
- [ ] All documentation updated and cross-referenced
- [ ] Website rebuilt with latest content
- [ ] Video course materials complete (8 modules)

**Deliverables:**
- Final verification report: `docs/status/COMPREHENSIVE_VERIFICATION_2026-03.md`
- All test results archived
- All security scan results archived
- Project declared complete

---

## PART 4: SUMMARY

### Work Items by Phase

| Phase | Items | Sessions | Priority |
|-------|-------|----------|----------|
| Phase 0: Security Scanning | 6 scan types, analyze, fix | 1 | CRITICAL |
| Phase 1: Go Test Coverage | 8 sub-tasks, ~150 test functions | 3-4 | HIGH |
| Phase 2: Frontend Test Coverage | 5 sub-tasks, ~80 test functions | 2-3 | HIGH |
| Phase 3: Advanced Test Types | 6 sub-tasks (fuzz, contract, chaos) | 2-3 | MEDIUM |
| Phase 4: Challenge Expansion | 15 new challenges | 1-2 | MEDIUM |
| Phase 5: Safety Hardening | 5 sub-tasks (leaks, deadlocks, races) | 2 | HIGH |
| Phase 6: Monitoring & Optimization | 4 sub-tasks | 1-2 | MEDIUM |
| Phase 7: Documentation | 6 sub-tasks, 8+ new files | 2-3 | MEDIUM |
| Phase 8: Final Verification | 6 verification steps | 1 | CRITICAL |
| **TOTAL** | **49 sub-tasks** | **15-21 sessions** | |

### Constraints (from CLAUDE.md and AGENTS.md)

- **Resources:** Max 30-40% host resources. `GOMAXPROCS=3`, `-p 2 -parallel 2` for Go tests.
- **Containers:** Podman only, `--network host`, fully qualified image names, max 4 CPUs / 8 GB RAM.
- **CI/CD:** GitHub Actions PERMANENTLY DISABLED. All CI/CD runs locally.
- **Protocol:** HTTP/3 (QUIC) + Brotli mandatory. Fallback: HTTP/2 + gzip.
- **Database:** Write via dialect abstraction only. Use `database.WrapDB()` for test DBs.
- **Challenges:** Sequential only, never parallel. RunAll is blocking.
- **Security:** No root/sudo. No interactive prompts.
- **Git:** Push to all 6 remotes after committing: `GIT_SSH_COMMAND="ssh -o BatchMode=yes" git push origin main`
- **Zero-warning policy:** No console warnings, no failed network requests, no deprecation warnings.

### Success Criteria

1. **Zero broken components** — all 7 components build and all tests pass
2. **60%+ Go backend coverage** — up from 30.7%
3. **Zero security vulnerabilities** — govulncheck, npm audit, Snyk, SonarQube all clean
4. **Zero race conditions** — `-race` flag passes on full suite
5. **Zero memory leaks** — verified via leak detector tests
6. **224+ challenges passing** — 209 existing + 15 new
7. **Complete documentation** — 8 video course modules, updated website, all guides current
8. **Zero TODO/FIXME/HACK** — already verified clean (0 found in production code)
9. **All test types represented** — unit, integration, E2E, stress, fuzz, contract, performance, chaos
10. **Production monitoring ready** — Prometheus metrics, Grafana dashboards, health endpoints
