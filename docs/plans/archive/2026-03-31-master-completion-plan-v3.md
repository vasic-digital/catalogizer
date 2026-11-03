# Master Completion Plan v3 — Full Project Hardening & 100% Coverage

**Date**: 2026-03-31
**Version Target**: 2.1.0 (Build 14)
**Supersedes**: 2026-03-30-master-completion-plan.md (v2.0.0 Build 13)
**Scope**: Complete hardening of all remaining gaps discovered in post-v2.0.0 audit

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Full Audit Report — Remaining Gaps](#2-full-audit-report--remaining-gaps)
3. [Phase 1: Concurrency Safety & Memory Leak Hardening](#phase-1)
4. [Phase 2: Security Scanning Execution & Remediation](#phase-2)
5. [Phase 3: Go Backend Test Coverage to 95%+](#phase-3)
6. [Phase 4: Frontend Test Coverage to 90%+](#phase-4)
7. [Phase 5: TypeScript Submodule Test Coverage to 90%+](#phase-5)
8. [Phase 6: Desktop, Mobile & API Client Coverage to 80%+](#phase-6)
9. [Phase 7: Stress, Integration & Performance Test Expansion](#phase-7)
10. [Phase 8: Challenge Bank to 400+](#phase-8)
11. [Phase 9: Go Package Documentation (doc.go for all packages)](#phase-9)
12. [Phase 10: TypeScript JSDoc & Accessibility Fixes](#phase-10)
13. [Phase 11: Architecture Docs for All Submodules](#phase-11)
14. [Phase 12: Video Course Extension & Website Update](#phase-12)
15. [Phase 13: Final Verification, Scanning & Release](#phase-13)
16. [Constraints & Rules](#constraints)
17. [Risk Matrix](#risk-matrix)
18. [Deliverables Summary](#deliverables)

---

## 1. Executive Summary

### Current State (2026-03-31) — Post-v2.0.0 Audit

v2.0.0 (Build 13) completed the 11-phase master plan. However, a comprehensive re-audit reveals **remaining gaps** that prevent claiming true 100% coverage and hardening.

| Metric | v2.0.0 Actual | v2.1.0 Target |
|--------|---------------|---------------|
| Go backend test files | 331 | 400+ (all packages covered) |
| Frontend test files | 312 | 400+ (all components covered) |
| TS submodule test files | 114 | 200+ (all exports tested) |
| Desktop test files | 2 | 50+ |
| Android test files | 50 | 100+ |
| Android TV test files | 31 | 80+ |
| API Client test files | ~5 | 50+ |
| Registered challenges | 249 | 400+ |
| Go packages with doc.go | 0/41 | 41/41 |
| Components with JSDoc | ~20% | 100% |
| Accessibility violations | 12+ | 0 |
| Submodules with ARCHITECTURE.md | 5/43 | 43/43 |
| `.env` in `.gitignore` (submodules) | 15/43 | 43/43 |
| `as any` in production code | 5+ | 0 |
| DEBUG printf statements | 2 | 0 |
| Remaining memory leak patterns | 5 | 0 |
| Security scan artifacts | 0 | Full reports |
| Stress tests actually run | 0 (all skipped) | All passing |

### What This Plan Delivers

- **Zero remaining concurrency/memory issues** — all debounce maps bounded, all scans cleaned, all lock ordering documented
- **Security scans ACTUALLY executed** — Snyk, SonarQube, Semgrep, govulncheck with real reports and all findings resolved
- **95%+ Go test coverage** — every package, every handler, every service, every challenge file tested
- **90%+ frontend coverage** — all 82 untested components, 13 pages, 8 hooks, 2 contexts tested
- **90%+ TS submodule coverage** — all 9 submodules with comprehensive tests
- **80%+ desktop/mobile/API client coverage** — realistic targets for platform-specific code
- **400+ challenges** — expanding from 249 to cover all new features, stress, security, monitoring
- **100% package documentation** — doc.go for all 41 Go packages, JSDoc for all TS exports
- **Zero accessibility violations** — all 12+ missing alt texts fixed, ARIA labels added
- **Updated video course** — all modules reflecting v2.1.0 features
- **Updated website** — changelog, features, guides all current

---

## 2. Full Audit Report — Remaining Gaps

### 2.1 Concurrency & Memory Safety (5 remaining issues)

| # | File | Line | Issue | Severity |
|---|------|------|-------|----------|
| 1 | `internal/media/realtime/watcher.go` | 34-35 | Debounce map entries not cleaned when timers cancelled | MEDIUM |
| 2 | `internal/services/universal_scanner.go` | 35 | `activeScans` map retains completed scans | MEDIUM |
| 3 | `middleware/request.go` | 36 | IP bucket map has no max size cap | MEDIUM |
| 4 | `Assets/pkg/event/bus.go` | 36-48 | Event handlers stored indefinitely, no unsubscribe | MEDIUM |
| 5 | `handlers/media_browse_handler.go` | 117-126 | `rows.Err()` never checked after iteration | LOW |

### 2.2 Dead Code & Quality Issues

| # | File | Line | Issue |
|---|------|------|-------|
| 1 | `challenges/auth_token_refresh.go` | 97, 150 | `fmt.Printf` DEBUG statements |
| 2 | `handlers/media_browse_handler.go` | 139 | `by_quality` returns hardcoded empty `map[string]int{}` |
| 3 | `catalog-web/src/types/collections.ts` | 38 | `value: any` in production type |
| 4 | `catalog-web/src/components/performance/MemoCache.tsx` | 213, 218 | `console.debug()` in production |

### 2.3 Test Coverage Gaps

#### 2.3.1 Untested Frontend Components (82 files)

**Auth**: LoginForm, RegisterForm, ProtectedRoute
**Collections** (14): CollectionTemplates, CollectionSettings, AdvancedSearch, BulkOperations, CollectionAutomation, CollectionsManager, ExternalIntegrations, CollectionSharing, CollectionAnalytics, CollectionExport, CollectionPreview, SmartCollectionBuilder, CollectionRealTime, PerformanceOptimizer
**Dashboard** (3): ActivityFeed, DashboardStats, MediaDistributionChart
**Favorites** (2): FavoriteToggle, FavoritesGrid
**Layout** (3): Layout, Header, PageHeader
**Media** (5): MediaDetailModal, MediaCard, MediaFilters, MediaGrid, MediaPlayer
**Performance** (2): MemoCache, LazyComponents
**Playlists** (7): PlaylistManager, SortablePlaylistItem, SmartPlaylistBuilder, PlaylistGrid, PlaylistAnalytics, PlaylistItem, PlaylistPlayer
**Subtitles** (2): SubtitleSyncModal, SubtitleUploadModal
**UI base** (13): ConnectionStatus, Progress, Tabs, Avatar, EmptyState, LoadingSpinner, Card, Button, Input, Switch, Select, Badge, Textarea
**Entity** (4): TypeSelector, EntityCard, EntityGrid, EntityDetailView
**Upload**: UploadManager
**Error**: ErrorBoundary, PageErrorBoundary, SplashScreen
**Contexts** (2): WebSocketContext, AuthContext
**Hooks** (4): useFavorites, usePlayerState, usePlaylistReorder, usePlaylists
**Pages** (13): EntityBrowser, Favorites, SubtitleManager, MediaBrowser, ConversionTools, Playlists, Settings, EntityDetail, Analytics, Admin, Dashboard, AIDashboard, Collections

#### 2.3.2 Untested TS Submodule Exports (40+ files)

| Module | Untested Files |
|--------|---------------|
| WebSocket-Client-TS | client.ts, hooks.ts, types.ts |
| Media-Types-TS | index.ts |
| Catalogizer-API-Client-TS | http.ts, types.ts, 14 service files |
| Auth-Context-React | AuthContext.tsx |
| Media-Browser-React | 5 component files |
| Media-Player-React | 4 files |
| Collection-Manager-React | 5 files |
| Dashboard-Analytics-React | 5 files |
| UI-Components-React | 3 base components |

#### 2.3.3 Missing Test Types

| Type | Go | Frontend | Desktop | Mobile |
|------|-----|----------|---------|--------|
| Visual regression | N/A | MISSING | MISSING | N/A |
| Accessibility | N/A | 1 test only | MISSING | N/A |
| Contract | Partial | MISSING | N/A | N/A |
| Frontend performance | N/A | MISSING | N/A | N/A |
| Fuzz | 8 tests | MISSING | N/A | N/A |

### 2.4 Documentation Gaps

| Gap | Count | Details |
|-----|-------|---------|
| Go packages without doc.go | 41 | All core + internal packages |
| Components without JSDoc | 53+ | All major component categories |
| Submodules without ARCHITECTURE.md | 38 | Only 5/43 have it |
| Submodules missing .env in .gitignore | 28 | Potential secret leak risk |
| Missing alt text on images | 12+ | Accessibility violation |

### 2.5 Accessibility Violations (12 files)

| File | Issue |
|------|-------|
| `CollectionsManager.tsx:116,190` | Collection thumbnail missing alt |
| `CollectionPreview.tsx:239` | Item thumbnail missing alt |
| `MediaDetailModal.tsx:76,99` | Backdrop + poster missing alt |
| `MediaCard.tsx:117` | Cover image missing alt |
| `SortablePlaylistItem.tsx:130` | Thumbnail missing alt |
| `PlaylistItem.tsx:156` | Item thumbnail missing alt |
| `PlaylistPlayer.tsx:389` | Playlist artwork missing alt |
| `SplashScreen.tsx:44,59` | Logo images missing alt |

### 2.6 Challenge Gap Analysis

- v2.0.0 claimed 352 challenges but only 249 verified in `register.go`
- Missing categories: playlist CRUD stress, backup lifecycle, change password, media analysis depth, cross-platform consistency
- 67 challenge implementation files have no unit tests

---

## Phase 1: Concurrency Safety & Memory Leak Hardening {#phase-1}

**Duration**: 1 session | **Risk**: MEDIUM | **Dependencies**: None

### Step 1.1: Debounce Map Cleanup (watcher.go)

Add TTL-based cleanup for debounce entries in `internal/media/realtime/watcher.go`:

```go
// Add to debounce cleanup goroutine:
func (w *ChangeWatcher) cleanupStaleDebounce() {
    w.debounceMu.Lock()
    defer w.debounceMu.Unlock()
    now := time.Now()
    for key, entry := range w.debounceMap {
        if now.Sub(entry.lastSeen) > 5*time.Minute {
            entry.timer.Stop()
            delete(w.debounceMap, key)
        }
    }
}
```

Add max size cap (10,000 entries). Add `lastSeen` field to debounce entry.

### Step 1.2: Active Scans Map Cleanup (universal_scanner.go)

Add cleanup of completed scan sessions:

```go
func (s *UniversalScanner) cleanupCompletedScans() {
    s.activeScansMu.Lock()
    defer s.activeScansMu.Unlock()
    for id, session := range s.activeScans {
        if session.Status == "completed" || session.Status == "failed" {
            if time.Since(session.CompletedAt) > 30*time.Minute {
                delete(s.activeScans, id)
            }
        }
    }
}
```

Wire cleanup to the existing periodic goroutine or create a new ticker.

### Step 1.3: IP Bucket Map Max Size (request.go)

Add max bucket count (100,000) with LRU eviction in the cleanup goroutine:

```go
const maxIPBuckets = 100000
if len(rl.buckets) > maxIPBuckets {
    // Evict oldest 10% by last-seen time
}
```

### Step 1.4: Event Bus Unsubscribe (Assets/pkg/event/bus.go)

Add `Unsubscribe(eventType string, handlerID string)` method. Add max handler count per event type (1,000).

### Step 1.5: Check rows.Err() (media_browse_handler.go)

Add `rows.Err()` check after the iteration loop at line 126:

```go
if err := rows.Err(); err != nil {
    log.Printf("error iterating file type rows: %v", err)
}
```

### Step 1.6: Remove DEBUG Statements

Replace `fmt.Printf("DEBUG: ...")` in `challenges/auth_token_refresh.go:97,150` with structured logging or remove entirely.

### Step 1.7: Fix by_quality Empty Map

Replace hardcoded `map[string]int{}` in `media_browse_handler.go:139` with actual quality analysis query from file metadata.

### Step 1.8: Document Lock Ordering

Add comment block to `UniversalScanner` documenting mutex acquisition order:
1. `mu` (general state)
2. `protocolScannersMu` (scanner registry)
3. `activeScansMu` (active scans)

### Step 1.9: Tests for Phase 1

| Test File | Validates |
|-----------|-----------|
| `internal/media/realtime/watcher_cleanup_test.go` | Debounce map bounded under load |
| `internal/services/universal_scanner_cleanup_test.go` | Completed scans evicted |
| `middleware/request_bucket_test.go` | IP buckets bounded |
| `handlers/media_browse_handler_test.go` | rows.Err() checked, quality populated |

### Step 1.10: Challenges for Phase 1

| ID | Name | Validates |
|----|------|-----------|
| CH-251 | Debounce Map Bounded | Watcher debounce map stays < 10K entries |
| CH-252 | Scan Cleanup Active | Completed scans removed after 30min |
| CH-253 | IP Bucket Bounded | Rate limiter buckets stay < 100K |
| CH-254 | Event Bus Unsubscribe | Handlers can be removed |
| CH-255 | Media Quality Real Data | /api/v1/media/stats returns real quality breakdown |

---

## Phase 2: Security Scanning Execution & Remediation {#phase-2}

**Duration**: 1-2 sessions | **Risk**: LOW-MEDIUM | **Dependencies**: Phase 1

### Step 2.1: Execute govulncheck

```bash
cd catalog-api && govulncheck ./...
```

Fix any reported vulnerabilities. Document results.

### Step 2.2: Execute npm audit

```bash
cd catalog-web && npm audit --production
cd catalogizer-desktop && npm audit --production
cd installer-wizard && npm audit --production
cd catalogizer-api-client && npm audit --production
```

Fix all critical/high vulnerabilities.

### Step 2.3: Execute Semgrep with Custom Rules

```bash
podman-compose -f docker-compose.security.yml --profile semgrep-scan run --rm semgrep-scanner
```

Additionally run custom rules:
```bash
semgrep --config config/semgrep-rules.yml catalog-api/ catalog-web/src/
```

Resolve all ERROR and WARNING findings.

### Step 2.4: Execute SonarQube Scan

```bash
# Start SonarQube
podman-compose -f docker-compose.security.yml up -d sonarqube sonarqube-db

# Wait for health, then run scan
./scripts/run-sonarqube-scan.sh
```

Review quality gate. Fix all Blocker and Critical issues.

### Step 2.5: Execute Snyk Scan

```bash
podman-compose -f docker-compose.security.yml --profile snyk-scan run --rm snyk-scanner
```

Or if Snyk token not available:
```bash
cd catalog-api && snyk test --all-projects 2>/dev/null || echo "Snyk: token required"
```

### Step 2.6: Save Scan Artifacts

Store all scan results in `reports/security/`:
- `reports/security/govulncheck-2026-03-31.txt`
- `reports/security/npm-audit-2026-03-31.json`
- `reports/security/semgrep-2026-03-31.json`
- `reports/security/sonarqube-2026-03-31.json`
- `reports/security/snyk-2026-03-31.json`

### Step 2.7: Challenges for Phase 2

| ID | Name | Validates |
|----|------|-----------|
| CH-256 | govulncheck Clean | 0 vulnerabilities |
| CH-257 | npm Audit Clean | 0 production critical/high |
| CH-258 | Semgrep Clean | 0 ERROR findings |
| CH-259 | SonarQube Gate Pass | Quality gate PASSES |
| CH-260 | Custom Rules Pass | All 8 custom Semgrep rules pass |

---

## Phase 3: Go Backend Test Coverage to 95%+ {#phase-3}

**Duration**: 3-4 sessions | **Risk**: LOW | **Dependencies**: Phase 1

### Step 3.1: Package doc.go + Missing Package Tests

For each of the 41 packages lacking doc.go:
1. Create `doc.go` with package-level comment
2. Verify test file exists; create if missing
3. Ensure all exported functions have at least one test

### Step 3.2: Challenge Implementation Unit Tests

Create unit tests for all 67 challenge files in `catalog-api/challenges/`:
- Test `NewChallenge()` constructor
- Test `Execute()` with mock HTTP client
- Test assertion validation logic
- Test error handling

### Step 3.3: Handler Test Completion

Ensure full coverage for:
- `handlers/admin_handler.go` — consolidate fragmented tests
- `handlers/playlist_handler.go` — CRUD + pagination + errors
- `handlers/challenge.go` — endpoint routing
- `handlers/media_browse_handler.go` — all query paths

### Step 3.4: Repository & Service Tests

- `repository/playlist_repository.go` — all DB operations
- `services/playlist_service.go` — business logic + validation
- `database/tx_helpers.go` — transaction helpers
- Migration files — up/down verification

### Step 3.5: Fuzz Test Expansion

Expand from 8 to 20+ fuzz tests covering:
- All API input parsing (query params, JSON bodies)
- File path validation
- Search query parsing
- Media type detection

### Step 3.6: Benchmark Expansion

Expand from 33 to 50+ benchmarks covering:
- All repository CRUD operations
- Handler response times
- Serialization/deserialization
- Cache operations
- WebSocket broadcast

### Step 3.7: Challenges for Phase 3

| ID | Name | Validates |
|----|------|-----------|
| CH-261 | Go Coverage 95% | `go test -cover` reports >= 95% |
| CH-262 | All Packages Documented | Every package has doc.go |
| CH-263 | Challenge Tests Pass | All challenge unit tests green |
| CH-264 | Fuzz Tests 20+ | At least 20 fuzz targets |
| CH-265 | Benchmarks 50+ | At least 50 benchmark functions |

---

## Phase 4: Frontend Test Coverage to 90%+ {#phase-4}

**Duration**: 3-5 sessions | **Risk**: LOW | **Dependencies**: None (parallel with Phase 3)

### Step 4.1: Auth Component Tests

- `LoginForm.test.tsx` — render, submit, validation, error display
- `RegisterForm.test.tsx` — render, submit, password match, error
- `ProtectedRoute.test.tsx` — redirect unauthenticated, render authenticated

### Step 4.2: Collection Component Tests (14 files)

For each: render test, user interaction, props validation, loading/error states.

### Step 4.3: Media Component Tests (5 files)

- MediaDetailModal — open/close, data display, actions
- MediaCard — render, click, hover states
- MediaFilters — filter selection, clear, apply
- MediaGrid — grid render, responsive, empty state
- MediaPlayer — play/pause, seek, volume, fullscreen

### Step 4.4: Playlist Component Tests (7 files)

- PlaylistManager — CRUD operations, drag-drop ordering
- PlaylistGrid — grid render, selection, actions
- PlaylistPlayer — queue management, next/prev, shuffle

### Step 4.5: Entity Component Tests (4 files)

- TypeSelector — type selection, filter
- EntityCard — render, click, metadata display
- EntityGrid — grid layout, pagination, sorting
- EntityDetailView — full detail render, related items

### Step 4.6: UI Base Component Tests (13 files)

For each: render, variants, disabled state, accessibility attributes.

### Step 4.7: Layout Component Tests (3 files)

- Layout — sidebar toggle, navigation
- Header — user menu, logout, navigation
- PageHeader — title, breadcrumbs, actions

### Step 4.8: Page Tests (13 files)

For each page: route render, data loading, navigation, error states, empty states.

### Step 4.9: Hook Tests (4 files)

For each hook: initial state, state transitions, cleanup, error handling.

### Step 4.10: Context Tests (2 files)

- WebSocketContext — connection, reconnection, message handling
- AuthContext — login, logout, token refresh, role-based access

### Step 4.11: Remaining Component Tests

- SplashScreen, ErrorBoundary, PageErrorBoundary
- UploadManager, FormatConverter
- Dashboard (3), Favorites (2), Subtitles (2), Performance (2), AI (3)

### Step 4.12: Accessibility Tests

Fix all 12+ missing alt attributes. Create accessibility test suite:

```tsx
// src/__tests__/accessibility/images.test.tsx
// Verify all <img> tags have alt attributes
// Verify all interactive elements have aria labels
// Verify semantic HTML structure
```

### Step 4.13: Challenges for Phase 4

| ID | Name | Validates |
|----|------|-----------|
| CH-266 | Frontend Coverage 90% | Vitest coverage >= 90% |
| CH-267 | All Components Tested | Zero untested component files |
| CH-268 | Accessibility Clean | Zero missing alt/aria violations |
| CH-269 | Zero Console Debug | No console.debug in production |
| CH-270 | Zero `any` in Production | No bare `any` in src/ (non-test) |

---

## Phase 5: TypeScript Submodule Test Coverage to 90%+ {#phase-5}

**Duration**: 2-3 sessions | **Risk**: LOW | **Dependencies**: None (parallel with Phases 3-4)

### Step 5.1: WebSocket-Client-TS Tests

- `client.test.ts` — connect, disconnect, reconnect, message send/receive
- `hooks.test.ts` — useWebSocket hook lifecycle
- `types.test.ts` — type validation

### Step 5.2: Catalogizer-API-Client-TS Tests

- `http.test.ts` — request/response, auth headers, error handling
- Tests for all 14 service files (AuthService through ReportService)

### Step 5.3: Auth-Context-React Tests

- `AuthContext.test.tsx` — provider, useAuth hook, login/logout flow

### Step 5.4: Media-Browser-React Tests

- Tests for all 5 components: EntityBrowser, EntityCard, EntityGrid, Pagination, TypeSelector

### Step 5.5: Media-Player-React Tests

- Tests for all 4 files: MediaPlayer, PlayerControls, useMediaPlayer, index

### Step 5.6: Collection-Manager-React Tests

- Tests for all 5 files: CollectionCard, CollectionForm, CollectionList, SmartRuleBuilder, index

### Step 5.7: Dashboard-Analytics-React Tests

- Tests for all 5 files: ActivityFeed, EntityStatsGrid, MediaDistributionBar, StatsCard, index

### Step 5.8: UI-Components-React Tests

- Tests for Button, Card, Input components

### Step 5.9: Media-Types-TS Tests

- Type validation tests for all exported media types

### Step 5.10: Challenges for Phase 5

| ID | Name | Validates |
|----|------|-----------|
| CH-271 | WebSocket Client 90% | Coverage >= 90% |
| CH-272 | API Client 90% | Coverage >= 90% |
| CH-273 | Auth Context 90% | Coverage >= 90% |
| CH-274 | Media Browser 90% | Coverage >= 90% |
| CH-275 | Media Player 90% | Coverage >= 90% |
| CH-276 | Collection Manager 90% | Coverage >= 90% |
| CH-277 | Dashboard Analytics 90% | Coverage >= 90% |
| CH-278 | UI Components 90% | Coverage >= 90% |

---

## Phase 6: Desktop, Mobile & API Client Coverage to 80%+ {#phase-6}

**Duration**: 3-4 sessions | **Risk**: MEDIUM | **Dependencies**: None (parallel with Phases 3-5)

### Step 6.1: Tauri Desktop Tests (3.8% -> 80%+)

- IPC command handler tests
- State management tests
- UI component tests
- Configuration management tests
- Service discovery tests

### Step 6.2: Installer Wizard Tests (9.4% -> 80%+)

- Installation workflow state machine tests
- System requirement validation tests
- Database initialization tests
- Network setup validation tests
- Wizard step navigation tests

### Step 6.3: Android Tests (4.8% -> 80%+)

- ViewModel unit tests (StateFlow assertions)
- Repository tests with Room mocking
- Compose UI tests (semantics, interaction)
- Navigation tests
- Network client tests (Retrofit mocking)

### Step 6.4: Android TV Tests (3.2% -> 80%+)

- D-pad navigation tests
- Focus management tests
- Media playback control tests
- Leanback component tests
- Remote control input tests

### Step 6.5: API Client Tests (0.6% -> 90%+)

- HttpClient tests (fetch mocking)
- All 14 service method tests
- Error handling tests
- Authentication flow tests
- Type serialization tests

### Step 6.6: Challenges for Phase 6

| ID | Name | Validates |
|----|------|-----------|
| CH-279 | Desktop Coverage 80% | Coverage >= 80% |
| CH-280 | Wizard Coverage 80% | Coverage >= 80% |
| CH-281 | Android Coverage 80% | Coverage >= 80% |
| CH-282 | Android TV Coverage 80% | Coverage >= 80% |
| CH-283 | API Client Old Coverage 90% | Coverage >= 90% |

---

## Phase 7: Stress, Integration & Performance Test Expansion {#phase-7}

**Duration**: 2-3 sessions | **Risk**: LOW | **Dependencies**: Phases 1-3

### Step 7.1: Enable All Skipped Stress Tests

Create non-short-mode runner. Verify all 54+ stress tests pass:
```bash
cd catalog-api && GOMAXPROCS=3 go test ./tests/stress/... -v -count=1 -timeout 30m -p 1 -parallel 1
```

### Step 7.2: Enable Filesystem Integration Tests

For FTP, NFS, SMB, WebDAV — use docker-compose.test-infra.yml:
```bash
podman-compose -f docker-compose.test-infra.yml up -d
cd catalog-api && go test ./filesystem/... -v -count=1 -run Integration
```

### Step 7.3: New Integration Tests

| Test File | Validates |
|-----------|-----------|
| `tests/integration/full_lifecycle_test.go` | Register -> Login -> Root -> Scan -> Browse -> Search -> Collect -> Play |
| `tests/integration/backup_restore_test.go` | Create -> Modify -> Restore -> Verify |
| `tests/integration/entity_pipeline_test.go` | Scan -> Aggregate -> Enrich -> Search |
| `tests/integration/websocket_events_test.go` | Subscribe -> Event -> Receive |
| `tests/integration/auth_lifecycle_test.go` | Register -> Login -> Refresh -> Password -> Relogin |
| `tests/integration/concurrent_users_test.go` | 10 users simultaneously |

### Step 7.4: New k6 Load Tests

| Test | Validates |
|------|-----------|
| `tests/k6/scan_load_test.js` | Concurrent scan operations |
| `tests/k6/search_load_test.js` | 200 concurrent searches |
| `tests/k6/websocket_load_test.js` | 100 concurrent WebSocket connections |
| `tests/k6/playlist_load_test.js` | Playlist CRUD under load |

### Step 7.5: Frontend Performance Tests

| Test | Target |
|------|--------|
| Bundle size | < 500KB gzipped |
| Component render | No render > 16ms |
| Virtual list scroll | 60fps at 10K items |

### Step 7.6: Monitoring & Metrics Tests

| Test | Validates |
|------|-----------|
| `tests/monitoring/prometheus_complete_test.go` | All custom metrics emit |
| `tests/monitoring/health_complete_test.go` | /health checks all subsystems |
| `tests/monitoring/goroutine_count_test.go` | Count accurate under load |
| `tests/monitoring/memory_profile_test.go` | No unbounded growth |

### Step 7.7: Visual Regression Test Framework

Set up Playwright visual comparison for critical UI paths:
- Login screen
- Dashboard
- Media browser
- Entity detail
- Collection manager

### Step 7.8: Challenges for Phase 7

| ID | Name | Validates |
|----|------|-----------|
| CH-284 | All Stress Tests Pass | 54+ stress tests green |
| CH-285 | Filesystem Integration Pass | FTP, SMB, NFS, WebDAV tests pass |
| CH-286 | Full Lifecycle Integration | End-to-end flow works |
| CH-287 | k6 Load p95 < 500ms | Performance at 50 users |
| CH-288 | k6 Stress Stable | System stable at 200 users |
| CH-289 | k6 Soak No Leaks | 30min soak, zero memory growth |
| CH-290 | Prometheus Complete | All metrics verified |
| CH-291 | Frontend Bundle < 500KB | gzipped bundle size check |

---

## Phase 8: Challenge Bank to 400+ {#phase-8}

**Duration**: 2-3 sessions | **Risk**: LOW | **Dependencies**: Phases 1-7

### Step 8.1: New Challenge Categories

| Category | ID Range | Count | Description |
|----------|----------|-------|-------------|
| Concurrency Hardening | CH-251 to CH-255 | 5 | Phase 1 |
| Security Scanning | CH-256 to CH-260 | 5 | Phase 2 |
| Backend Coverage | CH-261 to CH-265 | 5 | Phase 3 |
| Frontend Coverage | CH-266 to CH-270 | 5 | Phase 4 |
| Submodule Coverage | CH-271 to CH-278 | 8 | Phase 5 |
| Platform Coverage | CH-279 to CH-283 | 5 | Phase 6 |
| Stress & Performance | CH-284 to CH-291 | 8 | Phase 7 |
| Playlist E2E | CH-292 to CH-301 | 10 | Playlist CRUD + ordering + sharing |
| Backup E2E | CH-302 to CH-311 | 10 | Backup lifecycle + restore + verify |
| Password Mgmt | CH-312 to CH-316 | 5 | Password change + validation |
| Media Analysis E2E | CH-317 to CH-326 | 10 | Quality + detection + enrichment |
| Cross-Platform | CH-327 to CH-341 | 15 | Platform consistency |
| Monitoring E2E | CH-342 to CH-351 | 10 | Metrics + alerts + dashboards |
| Documentation | CH-352 to CH-361 | 10 | All docs complete + accurate |
| Accessibility | CH-362 to CH-371 | 10 | WCAG compliance |
| Error Handling | CH-372 to CH-381 | 10 | Graceful degradation |
| Data Integrity | CH-382 to CH-391 | 10 | Referential integrity + constraints |
| API Contract | CH-392 to CH-401 | 10 | OpenAPI spec compliance |

**New total**: 249 existing + 152 new = **401 challenges**

### Step 8.2: Register All New Challenges

Update `catalog-api/challenges/register.go` with all new registrations.

### Step 8.3: Create Challenge Config

Add JSON definitions in `challenges/config/` for each new challenge.

### Step 8.4: Challenges for Phase 8

Run `POST /api/v1/challenges/run-all` and verify 100% pass rate.

---

## Phase 9: Go Package Documentation (doc.go for all packages) {#phase-9}

**Duration**: 1-2 sessions | **Risk**: LOW | **Dependencies**: Phase 3

### Step 9.1: Create doc.go for All 41 Packages

For each package, create `doc.go` containing:
```go
// Package <name> provides <one-line description>.
//
// <2-3 sentence expanded description>
//
// Key types: <list of main exported types>
//
// Usage:
//
//     <brief code example>
package <name>
```

Priority order:
1. Core packages: config, database, handlers, services, repository, models, middleware
2. Internal packages: auth, cache, concurrency, eventbus, httpclient, lifecycle, media/*, metrics, monitoring, recovery, smb
3. Test packages: tests, tests/stress, tests/integration, tests/security, tests/monitoring

### Step 9.2: Document All Exported Functions

For every exported function lacking a godoc comment, add:
```go
// FunctionName does X with Y, returning Z.
// It returns an error if <condition>.
```

### Step 9.3: Challenges for Phase 9

| ID | Name | Validates |
|----|------|-----------|
| CH-352 | All Packages doc.go | Every Go package has doc.go |
| CH-353 | All Exports Documented | godoc coverage 100% |

---

## Phase 10: TypeScript JSDoc & Accessibility Fixes {#phase-10}

**Duration**: 1-2 sessions | **Risk**: LOW | **Dependencies**: Phase 4

### Step 10.1: Fix All Accessibility Violations

Fix all 12+ missing alt attributes with descriptive text:
- Collection thumbnails: `alt={collection.name}`
- Media images: `alt={media.title}`
- Playlist thumbnails: `alt={item.title}`
- Logo: `alt="Catalogizer logo"`

### Step 10.2: Eliminate `any` from Production Code

Replace `value: any` in `types/collections.ts:38` with proper type:
```typescript
value: string | number | boolean | string[];
```

### Step 10.3: Remove console.debug from Production

Remove or wrap in `import.meta.env.DEV` guard:
```typescript
if (import.meta.env.DEV) {
    console.debug('...')
}
```

### Step 10.4: Add JSDoc to All Components

For all 53+ undocumented components:
```tsx
/**
 * ComponentName renders/manages X.
 *
 * @param props - Component props
 * @param props.field - Description
 * @returns Rendered component
 *
 * @example
 * ```tsx
 * <ComponentName field="value" />
 * ```
 */
```

### Step 10.5: Add JSDoc to All Hooks

```tsx
/**
 * useHookName manages X state.
 *
 * @param param - Description
 * @returns Object containing { state, actions }
 *
 * @example
 * ```tsx
 * const { data, isLoading } = useHookName(param)
 * ```
 */
```

### Step 10.6: Add JSDoc to API Client Services

For all 14 service files in Catalogizer-API-Client-TS.

### Step 10.7: Challenges for Phase 10

| ID | Name | Validates |
|----|------|-----------|
| CH-354 | Zero Alt Violations | No missing alt attributes |
| CH-355 | Zero Any Production | No bare `any` in production TS |
| CH-356 | All Components JSDoc | JSDoc on all exported components |
| CH-357 | All Hooks JSDoc | JSDoc on all custom hooks |

---

## Phase 11: Architecture Docs for All Submodules {#phase-11}

**Duration**: 2-3 sessions | **Risk**: LOW | **Dependencies**: None

### Step 11.1: Create ARCHITECTURE.md for All 38 Missing Submodules

Each ARCHITECTURE.md follows this template:
```markdown
# Architecture — <Module Name>

## Purpose
<One paragraph>

## Structure
<Directory tree>

## Key Components
<Component descriptions>

## Data Flow
<How data moves through the module>

## Dependencies
<External and internal dependencies>

## Testing Strategy
<How to test this module>
```

### Step 11.2: Add .env to .gitignore for 28 Missing Submodules

For each submodule missing `.env` in `.gitignore`, add:
```
.env
.env.local
.env.*.local
```

### Step 11.3: Create LLMsVerifier README.md

The only submodule missing a README.

### Step 11.4: Update Main CLAUDE.md and AGENTS.md

Reflect all v2.1.0 changes, new patterns, new challenge IDs.

### Step 11.5: Challenges for Phase 11

| ID | Name | Validates |
|----|------|-----------|
| CH-358 | All Submodules ARCHITECTURE.md | 43/43 have ARCHITECTURE.md |
| CH-359 | All Submodules .env Protected | 43/43 have .env in .gitignore |
| CH-360 | All Submodules README | 43/43 have README.md |
| CH-361 | CLAUDE.md Current | Main CLAUDE.md reflects v2.1.0 |

---

## Phase 12: Video Course Extension & Website Update {#phase-12}

**Duration**: 1-2 sessions | **Risk**: LOW | **Dependencies**: Phases 1-11

### Step 12.1: New Video Course Modules

| Module | Title | Content |
|--------|-------|---------|
| Module 25 | Concurrency Hardening | Debounce cleanup, scan cleanup, IP bucket bounding, lock ordering |
| Module 26 | Security Scanning in Practice | Running Snyk, SonarQube, Semgrep, govulncheck; interpreting results |
| Module 27 | Test Coverage Mastery | Achieving 95%+ Go, 90%+ frontend, table-driven tests, fuzz testing |
| Module 28 | Performance Monitoring | Prometheus metrics, Grafana dashboards, k6 load testing, alerting |
| Module 29 | Module Architecture Deep Dive | 43 submodules, wiring patterns, lazy initialization, semaphores |
| Module 30 | Cross-Platform Consistency | Android, Android TV, Desktop, Web — shared patterns, platform-specific testing |

### Step 12.2: Update Existing Course Scripts

Update all 25 existing module scripts to reference v2.1.0 features.

### Step 12.3: Update Course Exercises

Add exercises for: security scanning, stress testing, challenge creation, monitoring setup.

### Step 12.4: Update Website Pages

| Page | Updates |
|------|---------|
| `index.md` | Version 2.1.0 highlights |
| `features.md` | Concurrency hardening, security scanning, 400+ challenges |
| `changelog.md` | Full v2.1.0 changelog |
| `faq.md` | New FAQ entries for security, testing, monitoring |
| `documentation.md` | Updated doc index |
| `course.md` | 30-module course listing |
| All `guides/*.md` | Reflect v2.1.0 changes |

### Step 12.5: Update SQL Schema Documentation

- `docs/architecture/SQL_COMPLETE_SCHEMA.md` — current DDL
- `docs/architecture/SQL_MIGRATIONS.md` — all migration versions
- `docs/architecture/DATABASE_SCHEMA.md` — ER descriptions

### Step 12.6: Update Architecture Diagrams

- Regenerate SVGs in `docs/diagrams/` reflecting new module wiring
- Update sequence diagrams for new flows (backup/restore, password change)
- Update ER diagrams for any schema changes

### Step 12.7: Challenges for Phase 12

| ID | Name | Validates |
|----|------|-----------|
| CH-362 | Course 30 Modules | 30 video scripts present |
| CH-363 | Website Current | All pages reflect v2.1.0 |
| CH-364 | Diagrams Updated | SVGs reflect current architecture |

---

## Phase 13: Final Verification, Scanning & Release {#phase-13}

**Duration**: 1-2 sessions | **Risk**: LOW | **Dependencies**: ALL previous phases

### Step 13.1: Run Complete Test Suite

```bash
# Go backend (all tests with race detection)
cd catalog-api && GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -race -count=1

# Frontend
cd catalog-web && npm run test && npm run test:e2e

# Desktop + Installer
cd catalogizer-desktop && npm run test
cd installer-wizard && npm run test

# API Client
cd catalogizer-api-client && npm run test

# Android
cd catalogizer-android && ./gradlew test
cd catalogizer-androidtv && ./gradlew test
```

### Step 13.2: Run Stress Tests

```bash
cd catalog-api && GOMAXPROCS=3 go test ./tests/stress/... -v -count=1 -timeout 30m -p 1
```

### Step 13.3: Run All Security Scans

```bash
cd catalog-api && govulncheck ./...
cd catalog-web && npm audit --production
./scripts/run-sonarqube-scan.sh
podman-compose -f docker-compose.security.yml --profile semgrep-scan run --rm semgrep-scanner
```

### Step 13.4: Run All Load Tests

```bash
for test in load_test stress_test soak_test spike_test auth_load_test entity_browse_load_test mixed_workload_test; do
    podman run --rm --network host -v $(pwd)/tests/k6:/scripts docker.io/grafana/k6:latest run /scripts/${test}.js
done
```

### Step 13.5: Run All 401 Challenges

```bash
# Via catalog-api service (compiled binary only)
curl -X POST http://localhost:8080/api/v1/challenges/run-all
```

Verify 100% pass rate.

### Step 13.6: Verify Coverage Gates

| Component | Gate | Command |
|-----------|------|---------|
| catalog-api | >= 95% | `go test -cover ./...` |
| catalog-web | >= 90% | `npm run test:coverage` |
| catalogizer-desktop | >= 80% | `npm run test -- --coverage` |
| installer-wizard | >= 80% | `npm run test -- --coverage` |
| catalogizer-android | >= 80% | `./gradlew test jacocoTestReport` |
| catalogizer-androidtv | >= 80% | `./gradlew test jacocoTestReport` |
| catalogizer-api-client | >= 90% | `npm run test -- --coverage` |
| All TS submodules | >= 90% | Per-module `npm run test -- --coverage` |

### Step 13.7: Verify Zero Issues

| Check | Expected |
|-------|----------|
| Go test failures | 0 |
| Frontend test failures | 0 |
| Race conditions (`-race`) | 0 |
| Goroutine leaks | 0 |
| Memory leaks | 0 |
| Security vulns (critical) | 0 |
| SonarQube quality gate | PASS |
| Semgrep ERROR findings | 0 |
| Console errors (browser) | 0 |
| Console warnings (browser) | 0 |
| Failed network requests | 0 |
| Dead code / stubs | 0 |
| Accessibility violations | 0 |
| Missing documentation | 0 |
| Missing ARCHITECTURE.md | 0 |
| Missing .env protection | 0 |
| `any` in production TS | 0 |
| DEBUG statements | 0 |

### Step 13.8: Build Release

```bash
./scripts/release-build.sh --container --force
```

All 7 components must build successfully.

### Step 13.9: Version Bump

Update `versions.json`: `2.0.0 build 13` -> `2.1.0 build 14`

### Step 13.10: Write Final Report

Create `docs/status/FINAL_COMPLETION_REPORT_2026-03-31.md` with all metrics and verification results.

### Step 13.11: Commit & Push

```bash
GIT_SSH_COMMAND="ssh -o BatchMode=yes" git push origin main
```

---

## Constraints & Rules {#constraints}

All work MUST comply with these constraints from CLAUDE.md and AGENTS.md:

1. **Container-only builds (Podman)** — No bare metal builds/services in production/QA
2. **No GitHub Actions** — All CI/CD runs locally
3. **No API keys in git** — .env.example with placeholders only
4. **HTTP/3 (QUIC) + Brotli** — All network communication
5. **Host resource limits** — 30-40% max (GOMAXPROCS=3, container limits)
6. **SQLite WAL mode** — Explicit PRAGMA after connection
7. **Zero warning/error policy** — No console errors, no failed requests
8. **HelixQA fully autonomous** — No hardcoded flows
9. **Challenge ops by compiled binaries only** — No curl/scripts in challenges
10. **No interactive processes** — No sudo/root password prompts during execution
11. **No destructive git operations** — No force push, no reset --hard without explicit approval
12. **PostCSS CommonJS** — module.exports for Node 18 compat

---

## Risk Matrix {#risk-matrix}

| Phase | Risk | Primary Risk | Mitigation |
|-------|------|-------------|------------|
| Phase 1 (Concurrency) | MEDIUM | New deadlocks | -race flag, lock ordering docs |
| Phase 2 (Security) | LOW | False positives | Manual review of findings |
| Phase 3 (Go Tests) | LOW | Test maintenance | Table-driven tests, behavior focus |
| Phase 4 (Frontend Tests) | LOW | Mock complexity | React Testing Library best practices |
| Phase 5 (Submodule Tests) | LOW | Dependency mocking | Per-module test isolation |
| Phase 6 (Platform Tests) | MEDIUM | Platform availability | CI containers for Android/Desktop |
| Phase 7 (Stress/Perf) | LOW | Flaky tests | Deterministic design, generous timeouts |
| Phase 8 (Challenges) | LOW | Registration errors | Automated verification |
| Phase 9 (Go Docs) | LOW | Stale docs | Generate from code where possible |
| Phase 10 (TS Docs) | LOW | Incomplete JSDoc | Lint enforcement |
| Phase 11 (Architecture) | LOW | Stale architecture | Cross-reference with code |
| Phase 12 (Course/Web) | LOW | Content lag | Sequential after implementation |
| Phase 13 (Verify) | LOW | Missed issues | Comprehensive checklist |

---

## Deliverables Summary {#deliverables}

| Phase | New Files | Modified Files | New Tests | New Challenges |
|-------|-----------|---------------|-----------|----------------|
| Phase 1 | ~8 | ~6 | ~15 | 5 (CH-251-255) |
| Phase 2 | ~5 (reports) | ~5 | ~10 | 5 (CH-256-260) |
| Phase 3 | ~80 | ~40 | ~200 | 5 (CH-261-265) |
| Phase 4 | ~95 | ~15 | ~300 | 5 (CH-266-270) |
| Phase 5 | ~45 | ~10 | ~120 | 8 (CH-271-278) |
| Phase 6 | ~150 | ~20 | ~400 | 5 (CH-279-283) |
| Phase 7 | ~25 | ~15 | ~80 | 8 (CH-284-291) |
| Phase 8 | ~20 | ~5 | ~30 | 113 (CH-292-401) |
| Phase 9 | ~45 | ~60 | 0 | 2 (CH-352-353) |
| Phase 10 | ~5 | ~70 | ~10 | 4 (CH-354-357) |
| Phase 11 | ~42 | ~30 | 0 | 4 (CH-358-361) |
| Phase 12 | ~10 | ~30 | 0 | 3 (CH-362-364) |
| Phase 13 | ~3 | ~10 | 0 | 0 |
| **TOTAL** | **~533** | **~316** | **~1,165** | **152 new** |

**Final Totals**:
- **Challenges**: 249 + 152 = **401**
- **New test files**: ~1,165
- **Go coverage**: 95%+
- **Frontend coverage**: 90%+
- **All other components**: 80%+
- **Security scans**: All PASS with artifacts
- **Documentation**: 100% coverage
- **Accessibility**: Zero violations
- **Dead code / stubs**: Zero
- **Memory/concurrency issues**: Zero
