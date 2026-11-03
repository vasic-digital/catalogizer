# Catalogizer — Ultimate Master Plan
## The Definitive, Granular, Zero-Defect Completion Strategy
### Version: 2026-04-22 | Status: EXECUTION-READY

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [The Universal Problem & The Universal Solution](#2-the-universal-problem--the-universal-solution)
3. [Project State Reconstruction](#3-project-state-reconstruction)
4. [The 15-Phase Execution Engine](#4-the-15-phase-execution-engine)
5. [Phase-by-Phase Granular Breakdown](#5-phase-by-phase-granular-breakdown)
6. [Cross-Component Integration Matrix](#6-cross-component-integration-matrix)
7. [The Verification Pyramid](#7-the-verification-pyramid)
8. [Operational Artifacts & Templates](#8-operational-artifacts--templates)
9. [Appendices](#9-appendices)

---

## 1. Executive Summary

This document is the single source of truth for completing the Catalogizer project to a **100% functional state with zero defects**. It is not a suggestion — it is a mechanical execution plan derived from the universal principles of fixing AI-assisted large-scale projects, applied specifically to the Catalogizer multi-platform media collection management system.

### 1.1 The Core Problem

Despite **1,004+ commits**, extensive documentation (50+ markdown files), high reported test coverage, and multiple "completion reports," the fundamental problem persists: **the code compiles, tests pass, but the product does not work when manually tested.** This is the classic "green tests, broken product" phenomenon documented in `Fixing_big_projects.md`.

### 1.2 The Root Cause Diagnosis

After comprehensive analysis of the repository, issues, documentation, and commit history, the root causes are:

| # | Root Cause | Evidence | Impact |
|---|-----------|----------|--------|
| 1 | **Semantic Violations** | AI-generated code breaks unwritten architectural rules | Features appear complete but fail in integration |
| 2 | **Mock-Only Testing** | High unit test coverage with mocked dependencies | Real services (SMB, FTP, NFS, WebDAV) never exercised |
| 3 | **Disabled Features Shipped** | `.go.disabled` files for conversion, recognition, recommendation | Core capabilities non-functional despite "completion" |
| 4 | **Missing Integration Layer** | Frontend ↔ Backend ↔ Mobile contracts not enforced | API changes break downstream silently |
| 5 | **No End-to-End Validation** | No full user journey testing | Login → Browse → Play → Settings flows untested |
| 6 | **Fragmented Documentation** | 50+ docs with conflicting "completion" statuses | No single source of truth for what actually works |
| 7 | **Premature Phase Closure** | Issues #2-#8 marked "completed" in comments but remain open | False sense of progress |

### 1.3 The Solution Architecture

This plan implements a **mechanical verification engine** around the AI development workflow. It replaces trust with proof at every layer. The approach is:

1. **Document the Undocumented** — Capture all institutional knowledge
2. **Test the Untested** — Integration, E2E, and real-protocol testing
3. **Enable the Disabled** — Re-enable and fix all `.disabled` features
4. **Verify the Unverified** — Full-QA Master Cycle per CONSTITUTION.md Article VII
5. **Close for Real** — No issue closes without demonstrable proof

---

## 2. The Universal Problem & The Universal Solution

### 2.1 Why AI-Generated Code Fails in Production

The document `Fixing_big_projects.md` identifies a well-documented phenomenon: LLMs excel at generating syntactically correct code that passes isolated unit tests, but fail to understand the context-dependent rules that make complex systems actually work. In the Catalogizer project specifically:

- **42 Go backend files** have critical implementation gaps despite "passing" tests
- **Conversion System** is entirely disabled (`*.go.disabled`)
- **Media Recognition** features are disabled
- **Recommendation System** is disabled
- **Deep Linking** is disabled
- **SMB Testing** — critical protocol tests are disabled
- **Video Player Subtitle Type Mismatch** at `video_player_service.go:1366` — a real bug masked by mocks
- **Authentication Rate Limiting Bypassed** at `auth/middleware.go:285` — security hole invisible to unit tests

### 2.2 The 8-Step Universal Fix (Applied to Catalogizer)

The universal approach from `Fixing_big_projects.md` is adapted specifically for Catalogizer:

#### Step 1: Institutional Knowledge Mapping
**Goal:** Capture every unwritten rule that "everyone just knows."

**Action for Catalogizer:**
- Create `docs/LANDMINES.md` with platform-specific rules
- Document the 41-submodule dependency graph with exact versioning
- Map the API contract between `catalog-api` and all clients (web, android, androidtv, desktop)
- Document the media type detection heuristics (50+ media types)
- Record the protocol-specific error handling patterns (SMB reconnection, FTP passive mode, NFS mount retry)

#### Step 2: Rebalance the Testing Pyramid
**Goal:** Move from unit-test-only to full pyramid coverage.

**Action for Catalogizer:**
| Test Category | Current State | Target State | Method |
|--------------|---------------|--------------|--------|
| Unit Tests | ~95% reported | 95% verified real | Remove mock-only tests, add real dependencies |
| Integration Tests | Minimal | 100% API coverage | Test each endpoint with real DB + real services |
| Protocol Tests | Disabled | All protocols active | Spin up SMB/FTP/NFS/WebDAV containers in CI |
| E2E Tests | None | All critical journeys | Playwright (web) + Espresso (Android) + Tauri (desktop) |
| Contract Tests | None | All API contracts | Pact or custom contract validation |
| Security Tests | Basic | Full coverage | OWASP ZAP + custom auth/rate-limit tests |
| Stress Tests | None | API + file operations | k6 load tests + concurrent file scan tests |
| Visual Regression | None | All platforms | Screenshot comparison per PR |

#### Step 3: Implement the Verification Loop
**Goal:** Every AI-generated change must pass verification before human review.

**Action for Catalogizer:**
- Create the Self-Correcting Prompt Chain for Claude Code
- Define exact verification commands per module (see Section 8.1)
- Enforce: IMPLEMENT → VERIFY → REPORT → ESCALATE workflow
- No code merges without passing the full verification gate

#### Step 4: LLM-as-Judge Layer
**Goal:** Catch semantic violations that tests cannot see.

**Action for Catalogizer:**
- Pre-merge review prompt targeting Catalogizer-specific rules
- Check against `LANDMINES.md` rules
- Verify API contract compatibility
- Check for disabled feature references in "completed" code

#### Step 5: CLAUDE.md as Operating System
**Goal:** The AI must read and follow project-specific constraints.

**Action for Catalogizer:**
- The existing `CLAUDE.md` is comprehensive — it must be enforced
- Every Claude Code session must start with: "Read CLAUDE.md, AGENTS.md, CONSTITUTION.md, and docs/LANDMINES.md before any code change"
- The Full-QA Master Cycle (Article VII) must be mechanically followed, not referenced

#### Step 6: Multi-Agent Review
**Goal:** Separate author and reviewer roles.

**Action for Catalogizer:**
- Agent 1 (Author): Implements the feature
- Agent 2 (Reviewer): Reviews against architecture and LANDMINES
- Agent 3 (Structure): Checks consistency, copy-paste errors, best practices

#### Step 7: Operational Guardrails
**Goal:** Prevent AI from breaking production environment assumptions.

**Action for Catalogizer:**
- Explicit environment definition in every prompt
- Feature flags for all AI-generated changes
- Real-device testing mandate (Article VIII of CONSTITUTION.md)

#### Step 8: Continuous Learning
**Goal:** Every bug found becomes a rule that prevents future bugs.

**Action for Catalogizer:**
- Every manual test failure → new LANDMINES rule + new test
- Monthly review of AI-generated code quality metrics
- Update verification commands as the codebase evolves

---

## 3. Project State Reconstruction

### 3.1 Component Inventory (The Truth)

| Component | Language/Framework | Status (Claimed) | Status (Actual) | Issues |
|-----------|-------------------|------------------|-----------------|--------|
| `catalog-api` | Go 1.25 / Gin | "Complete" | Core complete, disabled features exist | #6, #8 |
| `catalog-web` | React 18 / TS / Vite | "Complete" | 2,334 tests, 68.41% coverage | #8 |
| `catalogizer-android` | Kotlin / Compose | "Complete" | Scoped storage fixed, MediaPlayer/SyncService implemented | #8 |
| `catalogizer-androidtv` | Kotlin / Leanback | "Complete" | Mi Box 4 tested, focus navigation working | #3, #8 |
| `catalogizer-desktop` | Tauri / Rust + React | "Complete" | Auto-container dispatch landed | #8 |
| `catalogizer-api-client` | TypeScript | "Complete" | Library published | #8 |
| `HelixQA` | Go | "In Progress" | Autonomous QA engine (501 commits, 40+ pkg) | #3 |
| `LLMsVerifier` | Go | "Core Done" | Strategy pattern + recipes (needs integration) | #2 |
| `LLMOrchestrator` | Go | "Core Done" | Multi-provider pool (needs E2E) | #7 |
| `DocProcessor` | Go | "Complete" | ADOC/RST/MD/YAML/HTML parsing | — |
| `VisionEngine` | Go | "Complete" | CV + LLM vision (24 commits) | — |
| `ScreenDiff` | Go | "Complete" | Screenshot comparison | — |
| `VisualRegression` | Go | "Complete" | Visual regression testing | — |
| `TrainingCollector` | Go | "Complete" | Training data collection | — |
| `ReplayBuffer` | Go | "Complete" | Session replay | — |
| `OCU-CUDA-Sidecar` | Docker | "Complete" | GPU inference sidecar | — |

### 3.2 Submodule Inventory (41 Submodules)

**Go Modules (digital.vasic.*):**
Auth, Cache, Challenges, Concurrency, Config, Database, Discovery, DocProcessor, Entities, EventBus, Filesystem, Lazy, Media, Middleware, Memory, Observability, RateLimiter, Recovery, ReplayBuffer, Security, Storage, Streaming, Watcher

**TypeScript/React Packages (@vasic-digital/*):**
Auth-Context-React, Catalogizer-API-Client-TS, Collection-Manager-React, Dashboard-Analytics-React, Media-Browser-React, Media-Player-React, Media-Types-TS, UI-Components-React, WebSocket-Client-TS

**AI/QA Stack:**
HelixQA, LLMOrchestrator, LLMProvider, LLMsVerifier, ScreenDiff, TrainingCollector, VisualRegression, VisionEngine

**Infrastructure:**
Build, Containers, OCU-CUDA-Sidecar, Upstreams, Website

### 3.3 The Seven Open Issues (The Real Roadmap)

| Issue | Phase | Claimed Status | Actual Status | Estimated Real Work |
|-------|-------|---------------|---------------|-------------------|
| #2 | LLMsVerifier Strategy Pattern | "COMPLETED" | Core code done, needs integration verification | 8h |
| #7 | OpenCode Headless Integration | "COMPLETED" | Core code done, needs multi-provider E2E test | 12h |
| #3 | Enhanced Autonomous Session | "In Progress" | LLM navigator partially done, needs completion | 40h |
| #5 | Comprehensive Testing | "COMPLETED" | Unit tests done, integration/E2E gaps remain | 60h |
| #6 | Configuration & Environment | "Open" | Environment variable docs incomplete | 16h |
| #4 | Documentation | "Open" | Video course not recorded, some docs incomplete | 40h |
| #8 | Final Integration & Deployment | "Open" | Never actually performed | 80h |

### 3.4 Disabled Features Requiring Activation

These features are explicitly disabled in the codebase and MUST be re-enabled and fixed:

| Feature | Files | Why Disabled | Activation Work |
|---------|-------|-------------|-----------------|
| PDF Conversion Service | `*.go.disabled` | Implementation incomplete | Re-enable + complete implementation + tests |
| Media Recognition | Recognition features disabled | Dependency issues | Fix dependencies + re-enable + integration tests |
| Recommendation System | All recommendation code disabled | Algorithm incomplete | Complete algorithm + re-enable + E2E tests |
| Deep Linking | Deep linking disabled | Platform-specific URL handling | Implement per-platform handlers + test |
| SMB Protocol Testing | Critical SMB tests disabled | Requires SMB server | Container-based SMB server for CI |
| Content Conversion | Conversion API disabled | FFmpeg integration issues | Fix FFmpeg integration + re-enable |

### 3.5 Critical Bugs (Known but Unfixed)

| Bug | Location | Severity | Fix Required |
|-----|----------|----------|--------------|
| Video Player Subtitle Type Mismatch | `video_player_service.go:1366` | High | Fix type casting + add integration test |
| Auth Rate Limiting Bypass | `auth/middleware.go:285` | Critical | Implement proper Redis-backed rate limiting |
| Android TV HTTP/2 Failure | TV module OkHttpClient | Critical | Force HTTP/1.1 for TV endpoints |
| TV Focus Navigation | Various TV screens | High | Ensure all UI elements implement Focusable |

---

## 4. The 15-Phase Execution Engine

This plan divides the remaining work into **15 sequential phases**. Each phase has:
- **Entry Criteria**: What must be true before starting
- ** granular Tasks**: Every file, function, and test to modify
- **Verification Steps**: Exact commands that must exit 0
- **Exit Criteria**: What must be true to consider the phase complete
- **Deliverables**: Concrete outputs

### Phase Overview

```
Phase  1: Institutional Knowledge Capture (LANDMINES + Contracts)
Phase  2: Test Infrastructure Resurrection (Real Dependencies)
Phase  3: Disabled Feature Archaeology (Re-enable + Fix)
Phase  4: Critical Bug Extermination (Known Bugs)
Phase  5: HelixQA & AI Stack Completion (The Verification Engine)
Phase  6: Backend Integration Hardening (catalog-api)
Phase  7: Frontend Integration Hardening (catalog-web)
Phase  8: Android Mobile Hardening (catalogizer-android)
Phase  9: Android TV Hardening (catalogizer-androidtv)
Phase 10: Desktop Hardening (catalogizer-desktop)
Phase 11: Cross-Platform Contract Validation
Phase 12: Security Hardening & Penetration Testing
Phase 13: Performance Optimization & Stress Testing
Phase 14: Documentation Completion & Video Course
Phase 15: Final Integration, Deployment & Sign-Off
```

---

## 5. Phase-by-Phase Granular Breakdown

---

### PHASE 1: Institutional Knowledge Capture
**Duration: 3 days | Owner: Lead Architect + AI Agent**

**Goal:** Create the complete `docs/LANDMINES.md` file that captures every unwritten rule, and establish API contracts between all components.

**Entry Criteria:** None — this is the starting phase.

**1.1 Task: Landmine Rule Extraction**

Step-by-step method:
1. Read every existing `.md` file in the repository root (50+ files)
2. Extract all "must", "never", "always", "critical", "WARNING" statements
3. Interview (via document analysis) the implicit rules from commit messages
4. Map the 41-submodule dependency graph with exact `replace` directives
5. Document the API contract between `catalog-api` and each client

**The LANDMINES.md Template (to be created):**

```markdown
# Catalogizer Production Landmines & Semantic Rules

## Go Backend Rules
- **RULE-GO-001: No context.Background() in request handlers**
  - Context: All DB and HTTP calls MUST respect ctx.Done()
  - Detection: grep -r "context.Background()" internal/handler/ internal/service/
  - Fix: Use the request context, pass it through all call chains

- **RULE-GO-002: SMB Reconnection Grace Period**
  - Context: SMB disconnections are temporary; don't fail immediately
  - Detection: Check Filesystem module SMB client retry logic
  - Fix: Implement 3 retries with exponential backoff before marking offline

- **RULE-GO-003: Analytics Non-Blocking**
  - Context: Analytics calls add 200-400ms latency
  - Detection: Check for analytics.Track() inside HTTP handlers
  - Fix: Wrap in goroutine with timeout

- **RULE-GO-004: Rate Limiting Must Use Redis**
  - Context: In-memory rate limiting breaks on multi-instance deployments
  - Detection: Check auth/middleware.go rate limit implementation
  - Fix: Use Redis-backed sliding window (digital.vasic.ratelimiter)

- **RULE-GO-005: Never Disable Tests for "Convenience"**
  - Context: Disabled tests hide real bugs
  - Detection: Find all .disabled files, all t.Skip() calls without TODO
  - Fix: Re-enable and fix, or delete with explicit justification

- **RULE-GO-006: FFmpeg Integration Error Handling**
  - Context: FFmpeg can fail silently; errors must propagate
  - Detection: Check Media module FFmpeg wrapper error paths
  - Fix: Every FFmpeg call must check exit code and stderr

- **RULE-GO-007: SQLCipher Key Management**
  - Context: Database encryption key must never be logged
  - Detection: Check all fmt.Printf, log.Printf for key strings
  - Fix: Key in env only, redact in all logs

## React/Web Rules
- **RULE-WEB-001: API Response Caching**
  - Context: /api/v1/user must not be browser-cached
  - Detection: Check fetch calls for Cache-Control headers
  - Fix: Set Cache-Control: no-cache on auth endpoints

- **RULE-WEB-002: WebSocket Reconnection**
  - Context: WebSocket disconnects are normal; must auto-reconnect
  - Detection: Check WebSocket client reconnection logic
  - Fix: Exponential backoff reconnection with max retry limit

- **RULE-WEB-003: Zero Console Warnings**
  - Context: Warnings indicate potential runtime issues
  - Detection: npm run dev → check browser console
  - Fix: Resolve every warning, no exceptions

## Android Rules
- **RULE-AND-001: Scoped Storage Compliance**
  - Context: Android 13+ requires scoped storage
  - Detection: Check FileProvider usage, manifest permissions
  - Fix: Use MediaStore API for media access

- **RULE-AND-002: Foreground Service for Sync**
  - Context: Background sync is killed by Doze mode
  - Detection: Check SyncService notification channel
  - Fix: Proper foreground service with persistent notification

- **RULE-AND-003: ProGuard Rules for Every Library**
  - Context: R8/ProGuard strips code that looks unused
  - Detection: Check proguard-rules.pro completeness
  - Fix: Add -keep for all Retrofit/Gson/Reflection models

## Android TV Rules
- **RULE-TV-001: HTTP/1.1 Only**
  - Context: Android TV (API 28) fails HTTP/2 handshake on some chipsets
  - Detection: Check OkHttpClient.Builder in TV module
  - Fix: Explicitly set .protocols(listOf(Protocol.HTTP_1_1))

- **RULE-TV-002: D-Pad Navigation Focus**
  - Context: TV users navigate with D-pad, not touch
  - Detection: Every new UI element must handle focus
  - Fix: Implement Focusable, test focus traversal order

- **RULE-TV-003: Leanback Fragments**
  - Context: TV UI must use Leanback components
  - Detection: Check for standard Android UI components in TV module
  - Fix: Use BrowseSupportFragment, DetailsSupportFragment, etc.

## Desktop (Tauri) Rules
- **RULE-DESK-001: Rust Error Propagation**
  - Context: unwrap() in Rust crashes the entire application
  - Detection: grep -r "unwrap()" src-tauri/src/
  - Fix: Replace with proper Result handling

- **RULE-DESK-002: File Protocol Security**
  - Context: Tauri file access must be explicitly allowed
  - Detection: Check tauri.conf.json allowlist
  - Fix: Minimal permissions, scoped to media directories
```

**1.2 Task: API Contract Documentation**

Create `docs/API_CONTRACTS.md` with:

1. Every endpoint in `catalog-api` with:
   - Method, path, request body schema, response schema
   - Auth requirements (public / JWT / API key)
   - Rate limit tier
   - Which clients call it (web, android, androidtv, desktop)

2. WebSocket event contracts:
   - Event types pushed from server
   - Event format (JSON schema)
   - Client-side handlers

3. Real-time update contracts:
   - What triggers a WebSocket push
   - Debouncing/throttling rules

**1.3 Task: Submodule Dependency Map**

Create `docs/SUBMODULE_DEPENDENCIES.md`:

1. Graph of all 41 submodules
2. Which submodules depend on which
3. Version pinning strategy
4. Update cadence per submodule

**Verification for Phase 1:**
```bash
# Must pass: All documents created and non-empty
[ -s docs/LANDMINES.md ] && [ -s docs/API_CONTRACTS.md ] && [ -s docs/SUBMODULE_DEPENDENCIES.md ]
# Must pass: LANDMINES has at least 20 rules
grep -c "^##" docs/LANDMINES.md | xargs -I {} test {} -ge 20
# Must pass: API_CONTRACTS documents all endpoints
grep -c "^###" docs/API_CONTRACTS.md | xargs -I {} test {} -ge 30
```

**Exit Criteria:**
- [ ] `docs/LANDMINES.md` exists with ≥20 platform-specific rules
- [ ] `docs/API_CONTRACTS.md` exists with all endpoints documented
- [ ] `docs/SUBMODULE_DEPENDENCIES.md` exists with dependency graph
- [ ] All 3 documents reviewed by LLM-as-Judge

---

### PHASE 2: Test Infrastructure Resurrection
**Duration: 5 days | Owner: QA Lead + AI Agent**

**Goal:** Replace mock-heavy tests with real-dependency tests. Build the infrastructure to test against actual protocols and databases.

**Entry Criteria:** Phase 1 complete (LANDMINES documented).

**2.1 Task: Docker Test Infrastructure**

Create `docker-compose.test-infra.yml` with:

```yaml
# SMB Server for protocol testing
smb-test-server:
  image: dperson/samba
  volumes:
    - smb-test-data:/mount
  environment:
    - USERNAME=testuser;testpass
  ports:
    - "4445:445"

# FTP Server for protocol testing
ftp-test-server:
  image: stilliard/pure-ftpd
  environment:
    - FTP_USER_NAME=testuser
    - FTP_USER_PASS=testpass
    - FTP_USER_HOME=/home/testuser
  ports:
    - "2100:21"
    - "30000-30009:30000-30009"

# NFS Server for protocol testing
nfs-test-server:
  image: erichough/nfs-server
  environment:
    - NFS_EXPORT_0=/mnt/test-data 10.0.0.0/8(rw,sync,no_subtree_check)
  volumes:
    - nfs-test-data:/mnt/test-data
  ports:
    - "2049:2049"

# WebDAV Server for protocol testing
webdav-test-server:
  image byterendition/webdav
  environment:
    - WEBDAV_USERNAME=testuser
    - WEBDAV_PASSWORD=testpass
  ports:
    - "8080:80"

# Redis for rate limiting tests
redis-test:
  image: redis:7-alpine
  ports:
    - "6379:6379"
```

**2.2 Task: Go Backend Integration Test Suite**

For each module in `catalog-api`:

1. Create `internal/tests/integration/` directory
2. Write tests that:
   - Start a real SQLite database (not in-memory — file-based with SQLCipher)
   - Connect to Dockerized protocol servers
   - Exercise the full handler → service → repository → DB chain
   - Verify response format matches API contract

Example test structure:
```go
func TestMediaDetection_Integration(t *testing.T) {
    // Setup real test infrastructure
    db := setupRealTestDB(t)
    smbServer := setupSMBTestContainer(t)
    
    // Execute real detection
    result, err := media.Detect(smbServer.GetSharePath("/movies"))
    
    // Verify real results (not mocks)
    require.NoError(t, err)
    require.NotEmpty(t, result)
    require.Equal(t, "movie", result[0].MediaType)
    require.FileExists(t, result[0].ThumbnailPath) // Real thumbnail generated
}
```

**2.3 Task: Frontend Integration Test Suite**

1. Set up MSW (Mock Service Worker) with realistic API responses
2. Create integration tests for:
   - Login flow → token storage → authenticated request
   - Media browse → detail → play flow
   - Settings → save → persist → reload verification
   - Real-time updates via WebSocket

3. Create E2E tests with Playwright:
```typescript
// E2E test: Full user journey
test('complete media browsing journey', async ({ page }) => {
  // Login
  await page.goto('/login');
  await page.fill('[data-testid="username"]', 'admin');
  await page.fill('[data-testid="password"]', 'admin');
  await page.click('[data-testid="login-button"]');
  
  // Wait for dashboard
  await expect(page.locator('[data-testid="dashboard"]')).toBeVisible();
  
  // Navigate to media
  await page.click('[data-testid="media-nav"]');
  await expect(page.locator('[data-testid="media-grid"]')).toBeVisible();
  
  // Click on a media item
  await page.click('[data-testid="media-item"]:first-child');
  await expect(page.locator('[data-testid="media-detail"]')).toBeVisible();
  
  // Play
  await page.click('[data-testid="play-button"]');
  await expect(page.locator('[data-testid="video-player"]')).toBeVisible();
  
  // Verify no console errors
  const consoleErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });
  expect(consoleErrors).toHaveLength(0);
});
```

**2.4 Task: Android Instrumentation Tests**

1. Create `app/src/androidTest/java/` test suite
2. Tests that run on real device/emulator:
   - LoginActivity → MainActivity navigation
   - Media browser → detail → player flow
   - SyncService with real network calls
   - Scoped storage file operations
   - Background playback

**2.5 Task: Contract Test Suite**

Create `tests/contract/` with Pact or custom implementation:

```go
// Contract test: catalog-api → catalog-web
func TestContract_MediaListResponse(t *testing.T) {
    response := callAPIMediaList()
    
    // Verify schema matches contract
    assert.NotEmpty(t, response.Items)
    assert.NotZero(t, response.TotalCount)
    
    // Verify each item has required fields
    for _, item := range response.Items {
        assert.NotEmpty(t, item.ID)
        assert.NotEmpty(t, item.Title)
        assert.NotEmpty(t, item.MediaType)
        assert.NotNil(t, item.ThumbnailURL)
        // Verify media type is one of 50+ known types
        assert.Contains(t, validMediaTypes, item.MediaType)
    }
}
```

**Verification for Phase 2:**
```bash
# Start test infrastructure
docker-compose -f docker-compose.test-infra.yml up -d

# Backend integration tests
cd catalog-api && go test ./... -tags=integration -count=1

# Frontend E2E tests
cd catalog-web && npm run test:e2e

# Android instrumented tests
cd catalogizer-android && ./gradlew connectedAndroidTest

# Contract tests
cd tests/contract && go test ./... -count=1
```

**Exit Criteria:**
- [ ] Docker test infrastructure starts cleanly
- [ ] ≥50 integration tests passing for backend
- [ ] ≥10 E2E tests passing for web
- [ ] ≥10 instrumented tests passing for Android
- [ ] Contract tests validate all API endpoints
- [ ] CI pipeline runs all test categories

---

### PHASE 3: Disabled Feature Archaeology
**Duration: 10 days | Owner: Backend Lead + AI Agent**

**Goal:** Find every `.disabled` file, every `t.Skip()`, every `if false` block, and either re-enable + fix or explicitly delete with justification.

**Entry Criteria:** Phase 2 complete (test infrastructure ready).

**3.1 Task: Inventory All Disabled Code**

```bash
# Find all disabled files
find . -name "*.disabled" -o -name "*.go.disabled" -o -name "*.disabled.go"
# Find all skipped tests
grep -r "t.Skip" --include="*.go" .
grep -r "it.skip" --include="*.ts" --include="*.tsx" .
grep -r "@Ignore" --include="*.kt" .
# Find all "if false" blocks
grep -rn "if false" --include="*.go" --include="*.ts" --include="*.tsx" --include="*.kt" .
# Find all commented-out functionality
grep -rn "// TODO.*enable\|// FIXME.*enable\|// DISABLED\|// HACK" --include="*.go" --include="*.ts" --include="*.tsx" --include="*.kt" .
```

**3.2 Task: PDF Conversion Service**

1. Locate all `.go.disabled` files related to PDF conversion
2. Rename to `.go`
3. Fix compilation errors
4. Fix logical errors (the reason it was disabled)
5. Write integration tests
6. Write E2E test: Upload PDF → Convert → Verify output

**3.3 Task: Media Recognition**

1. Find disabled recognition code
2. Verify ML model dependencies are available
3. Re-enable recognition pipeline
4. Write tests with sample media files
5. Verify recognition accuracy against known test data

**3.4 Task: Recommendation System**

1. Find disabled recommendation code
2. Complete the recommendation algorithm
3. Integrate with user viewing history
4. Write tests: Given history X, recommend Y
5. Verify performance with large media libraries

**3.5 Task: Deep Linking**

1. Implement deep link handlers for each platform:
   - Web: URL route parsing (`/media/:id`, `/collection/:id`)
   - Android: Intent filters in manifest
   - Android TV: Leanback deep linking
   - Desktop: Custom protocol handler (`catalogizer://`)
2. Write cross-platform deep link tests
3. Verify from each entry point

**3.6 Task: SMB Protocol Testing**

1. Re-enable disabled SMB tests
2. Ensure Docker SMB container is used in CI
3. Test: Connect → Browse → Read → Disconnect → Reconnect flow
4. Test: Large directory listing performance
5. Test: Unicode filename handling

**3.7 Task: Content Conversion Pipeline**

1. Re-enable conversion API
2. Fix FFmpeg integration issues
3. Test: Video → Different format, Audio → MP3, etc.
4. Verify error handling for corrupted files

**Verification for Phase 3:**
```bash
# Zero disabled files remain (or each has explicit justification doc)
[ $(find . -name "*.disabled" | wc -l) -eq 0 ] || cat DISABLED_JUSTIFICATIONS.md

# Zero skipped tests without justification
! grep -r "t.Skip" --include="*.go" . | grep -v "t.Skip.*justified"

# All new features have integration tests
cd catalog-api && go test ./services/conversion -run TestIntegration -count=1
cd catalog-api && go test ./services/recognition -run TestIntegration -count=1
cd catalog-api && go test ./services/recommendation -run TestIntegration -count=1
```

**Exit Criteria:**
- [ ] Zero `.disabled` files (or documented justification for each)
- [ ] All previously disabled features have passing integration tests
- [ ] Each re-enabled feature has ≥1 E2E test
- [ ] No `t.Skip()` without linked issue number

---

### PHASE 4: Critical Bug Extermination
**Duration: 7 days | Owner: Backend Lead + AI Agent**

**Goal:** Fix every known critical/high bug with root-cause analysis and regression tests.

**Entry Criteria:** Phase 3 complete (disabled features re-enabled).

**4.1 Task: Video Player Subtitle Type Mismatch (video_player_service.go:1366)**

Root cause analysis:
1. Read the code at line 1366
2. Identify the type mismatch (likely string vs []byte for subtitle content)
3. Trace the data flow from subtitle file → parser → player
4. Fix the type at the source
5. Add type assertions at boundaries

Regression test:
```go
func TestVideoPlayer_SubtitleTypeConsistency(t *testing.T) {
    // Load test video with embedded subtitles
    video := loadTestVideo("test_subtitle.mkv")
    
    // Extract subtitles
    subtitles, err := video.ExtractSubtitles()
    require.NoError(t, err)
    
    // Pass to player — must not panic on type mismatch
    player := NewPlayer(video)
    err = player.LoadSubtitles(subtitles)
    require.NoError(t, err)
    
    // Verify subtitle text is renderable
    require.IsType(t, string(""), subtitles[0].Text)
}
```

**4.2 Task: Authentication Rate Limiting Bypass (auth/middleware.go:285)**

Root cause analysis:
1. Examine current rate limiting implementation
2. Identify bypass vector (likely IP spoofing or path bypass)
3. Implement Redis-backed sliding window
4. Add tests for: Normal use, Edge of limit, Over limit, IP spoofing attempt

```go
func TestAuth_RateLimiting_CannotBypass(t *testing.T) {
    redis := setupTestRedis(t)
    middleware := auth.NewRateLimitMiddleware(redis, 10, time.Minute)
    
    handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    
    // 10 requests should succeed
    for i := 0; i < 10; i++ {
        req := httptest.NewRequest("GET", "/api/v1/media", nil)
        rr := httptest.NewRecorder()
        handler.ServeHTTP(rr, req)
        assert.Equal(t, http.StatusOK, rr.Code)
    }
    
    // 11th request should be rate limited
    req := httptest.NewRequest("GET", "/api/v1/media", nil)
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    assert.Equal(t, http.StatusTooManyRequests, rr.Code)
    
    // Attempt IP spoofing — should not bypass
    req = httptest.NewRequest("GET", "/api/v1/media", nil)
    req.Header.Set("X-Forwarded-For", "1.2.3.4")
    rr = httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}
```

**4.3 Task: Android TV HTTP/2 Failure**

1. Force HTTP/1.1 for all TV module OkHttpClient instances
2. Verify with Mi Box 4 test device or emulator (API 28)
3. Test: Login → Browse → Play all use HTTP/1.1

```kotlin
// In TV module OkHttpClient builder
val client = OkHttpClient.Builder()
    .protocols(listOf(Protocol.HTTP_1_1))  // RULE-TV-001
    .build()
```

**4.4 Task: TV Focus Navigation**

1. Audit every screen in TV module for focus handling
2. Ensure all clickable elements have `android:focusable="true"`
3. Verify D-pad navigation order with `nextFocusUp/Down/Left/Right`
4. Test: Navigate entire app with D-pad only, no touch

**Verification for Phase 4:**
```bash
# All critical bugs have regression tests
cd catalog-api && go test ./services/video -run TestVideoPlayer_SubtitleTypeConsistency -count=1
cd catalog-api && go test ./auth -run TestAuth_RateLimiting -count=1

# TV tests on emulator
cd catalogizer-androidtv && ./gradlew connectedCheck

# Security audit passes
gosec ./...  # No critical or high findings
```

**Exit Criteria:**
- [ ] Video player subtitle type mismatch fixed + regression test
- [ ] Rate limiting bypass closed + comprehensive tests
- [ ] TV HTTP/2 forced to HTTP/1.1
- [ ] TV focus navigation works on emulator with D-pad only
- [ ] All fixes verified on real devices (Article VIII)

---

### PHASE 5: HelixQA & AI Stack Completion (The Verification Engine)
**Duration: 14 days | Owner: AI/QA Lead + AI Agent**

**Goal:** Complete the HelixQA autonomous QA system and all its AI stack dependencies to the point where it can reliably validate all other Catalogizer components. HelixQA is not a test tool — it is the **verification engine** that makes the Full-QA Master Cycle (CONSTITUTION.md Article VII) possible. Without it, no other component can be declared "done."

**Entry Criteria:** Phase 4 complete (critical bugs fixed, disabled features re-enabled).

**5.0 Understanding the HelixQA Architecture**

HelixQA (at `HelixDevelopment/HelixQA`, 501 commits, 40+ packages) is a complete AI-driven QA orchestration system. It is the largest and most complex submodule in the entire project.

**HelixQA Internal Package Map (40+ packages in `pkg/`):**

| Package | Purpose | Status Risk |
|---------|---------|-------------|
| `pkg/agent` | LLM agent management, VLM-guided DFS explorer | Medium |
| `pkg/analysis` | PELT change-point segmentation for performance analysis | Medium |
| `pkg/autonomous` | Autonomous QA session core (mapper, coordinator) | **High** — Issue #3 |
| `pkg/bridge` | scrcpy bridge for Android screen control | Medium |
| `pkg/bridges` | Bridge registry with sidecar probes, ToolKind | Medium |
| `pkg/capture` | Screen capture (xcbshm Linux capture) | Medium |
| `pkg/config` | Configuration loader and validation | Low |
| `pkg/controller` | Process controller, fallback models | Low |
| `pkg/detector` | Issue detection engine | Medium |
| `pkg/discovery` | Service discovery for test targets | Low |
| `pkg/distributed` | Distributed test execution | **High** — Complex |
| `pkg/evidence` | Evidence collection (screenshots, logs, video) | Medium |
| `pkg/gpu/infer` | Triton KServe v2 GPU inference client | **High** — Requires GPU |
| `pkg/gst` | GStreamer integration for video | Medium |
| `pkg/infra` | Infrastructure decoupling from Catalogizer | Low |
| `pkg/issuedetector` | Issue detection with acceptance criteria | Medium |
| `pkg/learning` | ML learning from past sessions | Medium |
| `pkg/llm` | LLM provider abstraction layer | Low |
| `pkg/maestro` | Maestro FlowRunner for YAML mobile flows | Medium |
| `pkg/memory` | Vector memory database for session state | Medium |
| `pkg/navigator` | LLM-powered navigation (Android-9 KeyPress fallback) | **High** — Recent fixes |
| `pkg/nexus` | ANSI-terminal accessibility backend (axtree/TUI) | Medium |
| `pkg/observe/frida` | Frida dynamic instrumentation bridge | **High** — Complex |
| `pkg/opensource` | OSS vendoring and license audit | Low |
| `pkg/orchestrator` | QA orchestration engine | Low |
| `pkg/performance` | Performance testing and benchmarking | Medium |
| `pkg/planning` | Test planning and coverage optimization | Medium |
| `pkg/regression` | Regression testing with HTML reporter | Low |
| `pkg/replay` | Session replay functionality | Medium |
| `pkg/reporter` | Executive summary, navigation map, LLM summary | Low |
| `pkg/reproduce` | Bug reproduction from tickets | Medium |
| `pkg/session` | Session management | Low |
| `pkg/streaming` | Video streaming for remote devices | Medium |
| `pkg/testbank` | Test bank management | Low |
| `pkg/ticket` | Ticket generation with evidence | Low |
| `pkg/training` | Training data collection | Medium |
| `pkg/types` | Core type definitions | Low |
| `pkg/validator` | Video routing and input validation | Low |
| `pkg/validators` | Additional validators | Low |
| `pkg/video` | Video recording, segmentation, safety fuses | Medium |
| `pkg/vision` | Perceptual vision (DreamSim, LPIPS on GPU infer) | **High** — M60 refactor |
| `pkg/visual` | Visual regression detection | Medium |

**AI Stack Dependency Chain:**

```
HelixQA (the orchestrator)
├── LLMsVerifier (strategy pattern + recipe builder) ← Issue #2
│   ├── pkg/strategy (VerificationStrategy interface)
│   ├── pkg/recipe (builder + validator)
│   └── pkg/helixqa (QA-specific strategies + 7 recipes)
├── LLMOrchestrator (multi-provider LLM pool) ← Issue #7
│   ├── OpenCode adapter (headless CLI mode)
│   ├── Claude Code adapter
│   ├── Gemini adapter
│   ├── Junie adapter
│   ├── Qwen Code adapter
│   └── pkg/pool/multi_pool.go (agent selection)
├── LLMProvider (unified LLM interface)
│   ├── OpenAI (GPT-4o)
│   ├── Anthropic (Claude 3.5/4 Sonnet)
│   ├── Google (Gemini 2.0/2.5 Flash)
│   ├── Groq (Llama 3.3 70B)
│   ├── DeepSeek (deepseek-chat)
│   ├── xAI (Grok)
│   ├── Qwen (qwen-max)
│   └── Local (llama.cpp RPC via OCU-CUDA-Sidecar)
├── DocProcessor (documentation parsing)
│   ├── Markdown
│   ├── YAML
│   ├── HTML
│   ├── AsciiDoc (ADOC)
│   └── reStructuredText (RST)
├── VisionEngine (computer vision + LLM vision)
│   ├── Analyzer (screen analysis, element detection)
│   ├── NavigationGraph (BFS pathfinding, DOT/JSON/Mermaid)
│   ├── LLM Vision (GPT-4o, Claude, Gemini, Qwen-VL)
│   └── OpenCV (GoCV with build-tag gating)
├── ScreenDiff (screenshot comparison)
├── TrainingCollector (training data aggregation)
├── VisualRegression (visual regression testing)
└── OCU-CUDA-Sidecar (GPU sidecar for local inference)
    ├── Triton KServe v2 server
    └── llama.cpp RPC backend
```

**5.1 Task: LLMsVerifier Completion (Issue #2)**

The LLMsVerifier submodule implements the Strategy pattern for LLM selection. It must be fully functional before HelixQA can select the right model for each QA phase.

**5.1.1 Verification Strategy Interface**

Verify `pkg/strategy/interface.go` implements:

```go
// Must be complete and tested:
type VerificationStrategy interface {
    Name() string
    Score(ctx context.Context, model LLMModel, phase QAPhase) (Score, error)
    Select(ctx context.Context, candidates []LLMModel, phase QAPhase) (LLMModel, error)
    Rank(ctx context.Context, candidates []LLMModel, phase QAPhase) ([]RankedLLM, error)
}
```

**Verification steps:**
```bash
cd LLMsVerifier && go test ./pkg/strategy -run TestInterface -count=1
cd LLMsVerifier && go test ./pkg/strategy -run TestDefaultStrategy -count=1
```

**5.1.2 Recipe Builder and Validator**

Verify `pkg/recipe/builder.go` and `pkg/recipe/validator.go`:

| Recipe | Purpose | Verification |
|--------|---------|--------------|
| `qa-comprehensive` | Full QA with all phases | Build → Validate |
| `qa-speed` | Fast smoke test | Build → Validate |
| `qa-quality` | Maximum coverage | Build → Validate |
| `qa-cost-optimized` | Cheapest LLM per phase | Build → Validate |
| `qa-vision-heavy` | Vision-focused testing | Build → Validate |
| `qa-api-only` | API contract testing | Build → Validate |
| `qa-mobile-first` | Mobile-focused testing | Build → Validate |

```bash
cd LLMsVerifier && go test ./pkg/recipe -run TestBuilder -count=1
cd LLMsVerifier && go test ./pkg/recipe -run TestValidator -count=1
cd LLMsVerifier && go test ./pkg/recipe -run TestAllRecipes -count=1
```

**5.1.3 HelixQA-Specific Strategy**

Verify `pkg/helixqa/strategy.go` implements phase-aware model selection:

| QA Phase | Preferred Model Type | Rationale |
|----------|---------------------|-----------|
| Navigation (Execute/Curiosity) | JSON-action models (fast) | Quick UI interactions |
| Analysis (Analyze) | Rich-description models | Detailed issue descriptions |
| Planning (Learn/Plan) | Reasoning models | Complex test planning |

```bash
cd LLMsVerifier && go test ./pkg/helixqa -run TestQAStrategy -count=1
cd LLMsVerifier && go test ./pkg/helixqa -run TestPhaseSelection -count=1
```

**5.1.4 Wire LLMsVerifier into HelixQA**

Verify the `replace` directive in `HelixQA/go.mod`:
```bash
cd HelixQA && grep "LLMsVerifier" go.mod
cd HelixQA && go mod tidy && go build ./...
cd HelixQA && go test ./pkg/llm -run TestVerifierIntegration -count=1
```

**5.2 Task: LLMOrchestrator Multi-Provider Pool (Issue #7)**

The LLMOrchestrator manages multiple CLI agents in headless mode. It must handle agent lifecycle, output parsing, and intelligent routing.

**5.2.1 OpenCode Headless Adapter**

Verify `pkg/adapter/opencode_headless.go`:

```bash
cd LLMOrchestrator && go test ./pkg/adapter -run TestOpenCodeHeadless -count=1
cd LLMOrchestrator && go test ./pkg/adapter -run TestOpenCodeParser -count=1
```

Tests must verify:
- Headless mode starts without interactive prompts
- stdin/stdout/stderr pipes work correctly
- Output parser handles partial/chunked responses
- Timeout handling works (120s default)
- Process cleanup on context cancellation

**5.2.2 Multi-Provider Pool**

Verify `pkg/pool/multi_pool.go`:

```bash
cd LLMOrchestrator && go test ./pkg/pool -run TestMultiPool -count=1
cd LLMOrchestrator && go test ./pkg/pool -run TestAgentSelection -count=1
cd LLMOrchestrator && go test ./pkg/pool -run TestPoolRecovery -count=1
```

Tests must verify:
- Pool initialization with N agents
- Round-robin and priority-based selection
- Agent failure detection and replacement
- Graceful degradation when agents unavailable
- Concurrent request handling without races

**5.2.3 All Agent Adapters**

| Agent | Binary | Headless Test | Status |
|-------|--------|--------------|--------|
| OpenCode | `opencode` | `TestOpenCode*` | Must pass |
| Claude Code | `claude` | `TestClaudeCode*` | Must pass |
| Gemini | `gemini` | `TestGemini*` | Must pass |
| Junie | `junie` | `TestJunie*` | Must pass |
| Qwen Code | `qwen-code` | `TestQwenCode*` | Must pass |

```bash
cd LLMOrchestrator && go test ./pkg/adapter -run 'TestOpenCode|TestClaude|TestGemini|TestJunie|TestQwen' -count=1
```

**5.2.4 Wire LLMOrchestrator into HelixQA**

```bash
cd HelixQA && grep "LLMOrchestrator" go.mod
cd HelixQA && go test ./pkg/orchestrator -run TestLLMOrchestratorIntegration -count=1
```

**5.3 Task: Enhanced Autonomous Session (Issue #3)**

This is the core of HelixQA — the autonomous testing engine. It has 16 sub-tasks across 6 functional areas.

**5.3.1 Feature-to-Test Mapper (`pkg/autonomous/mapper.go`)**

Verifications:
```bash
cd HelixQA && go test ./pkg/autonomous -run TestMapper -count=1
cd HelixQA && go test ./pkg/autonomous -run TestFeatureCache -count=1
cd HelixQA && go test ./pkg/autonomous -run TestDocProcessorIntegration -count=1
```

Must verify:
- ADOC, RST, MD, YAML, HTML formats parse correctly
- Feature extraction produces valid FeatureTestMapping
- Cache hit/miss works correctly
- LLM-generated test steps are valid JSON actions

**5.3.2 LLM-Powered Navigator (`pkg/navigator/llm_navigator.go`)**

Verifications:
```bash
cd HelixQA && go test ./pkg/navigator -run TestLLMNavigator -count=1
cd HelixQA && go test ./pkg/navigator -run TestNavigationGraph -count=1
cd HelixQA && go test ./pkg/navigator -run TestShortestPath -count=1
cd HelixQA && go test ./pkg/navigator -run TestAndroid9KeyPressFallback -count=1
```

Must verify:
- Navigation graph builds correctly from app screens
- Shortest path calculation works (BFS)
- LLM path inference works when graph is incomplete
- Screen verification after each action
- **Android 9 KeyPress fallback** (recent fix — must work on Mi Box 4)
- Stagnation detection (identical screen >10s)

**5.3.3 Issue Analyzer (`pkg/issuedetector` + LLM analyzer)**

Verifications:
```bash
cd HelixQA && go test ./pkg/issuedetector -run TestIssueDetection -count=1
cd HelixQA && go test ./pkg/issuedetector -run TestSeverityClassification -count=1
cd HelixQA && go test ./pkg/issuedetector -run TestAcceptanceCriteria -count=1
```

Must verify:
- All issue categories detected (crash, ANR, visual, functional, performance)
- Severity classification (critical/high/medium/low/info) is accurate
- LLM analysis provides actionable descriptions
- Acceptance criteria enforced in ticket generation

**5.3.4 Session Recorder (`pkg/evidence` + timeline)**

Verifications:
```bash
cd HelixQA && go test ./pkg/evidence -run TestSessionRecorder -count=1
cd HelixQA && go test ./pkg/evidence -run TestTimeline -count=1
cd HelixQA && go test ./pkg/evidence -run TestAnnotatedScreenshot -count=1
```

Must verify:
- Screenshots capture at correct resolution (1920x1080 min)
- Video recording with timestamp alignment
- Timeline events correlate screenshots + video + actions
- Annotated screenshots highlight detected issues
- Evidence saved to `qa-results/session-<timestamp>/`

**5.3.5 Ticket Generator (`pkg/ticket/generator.go`)**

Verifications:
```bash
cd HelixQA && go test ./pkg/ticket -run TestTicketGenerator -count=1
cd HelixQA && go test ./pkg/ticket -run TestTicketTemplates -count=1
```

Must verify:
- Markdown tickets include all evidence
- Severity and category correctly assigned
- Video timestamp references included
- Acceptance criteria for reproduction

**5.3.6 Session Coordinator Integration**

The coordinator wires all Phase 3 components together:

```bash
cd HelixQA && go test ./pkg/session -run TestFullAutonomousSession -count=1
cd HelixQA && go test ./pkg/session -run TestCoordinatorWiring -count=1
```

Must verify:
- Session starts with correct configuration
- All subsystems initialize without error
- Graceful shutdown saves partial results
- Timeout enforcement works

**5.4 Task: VisionEngine Integration**

VisionEngine (`HelixDevelopment/VisionEngine`, 24 commits) provides computer vision capabilities.

**5.4.1 Verify VisionEngine Standalone**

```bash
cd VisionEngine && go build ./...
cd VisionEngine && go test ./pkg/analyzer -run TestScreenAnalysis -count=1
cd VisionEngine && go test ./pkg/analyzer -run TestNavigationGraph -count=1
```

**5.4.2 Verify HelixQA → VisionEngine Integration**

```bash
cd HelixQA && go test ./pkg/vision -run TestVisionEngineIntegration -count=1
cd HelixQA && go test ./pkg/vision -run TestDreamSimLPIPS -count=1
```

**5.5 Task: DocProcessor Format Support**

Verify all 5 documentation formats parse correctly:

```bash
cd DocProcessor && go test ./... -run TestMarkdown -count=1
cd DocProcessor && go test ./... -run TestYAML -count=1
cd DocProcessor && go test ./... -run TestHTML -count=1
cd DocProcessor && go test ./... -run TestADOC -count=1
cd DocProcessor && go test ./... -run TestRST -count=1
cd HelixQA && go test ./pkg/autonomous -run TestDocProcessorFormats -count=1
```

**5.6 Task: GPU Inference Pipeline (OCU-CUDA-Sidecar)**

The OCU-CUDA-Sidecar provides local GPU inference for vision tasks.

**5.6.1 Verify Triton KServe v2 Client**

```bash
cd HelixQA && go test ./pkg/gpu/infer -run TestTritonClient -count=1
```

**5.6.2 Verify llama.cpp RPC Backend**

```bash
# If GPU available:
cd OCU-CUDA-Sidecar && docker build -t ocu-cuda .
cd OCU-CUDA-Sidecar && docker run --gpus all ocu-cuda ./healthcheck
```

**5.7 Task: ScreenDiff + VisualRegression**

```bash
cd ScreenDiff && go test ./... -run TestScreenshotComparison -count 1
cd VisualRegression && go test ./... -run TestVisualRegression -count 1
cd HelixQA && go test ./pkg/visual -run TestVisualDetection -count=1
```

**5.8 Task: TrainingCollector**

```bash
cd TrainingCollector && go test ./... -count 1
cd HelixQA && go test ./pkg/learning -run TestTrainingData -count=1
```

**5.9 Task: Frida Dynamic Instrumentation (`pkg/observe/frida`)**

The Frida bridge provides dynamic instrumentation for Android apps.

```bash
cd HelixQA && go test ./pkg/observe/frida -run TestFridaBridge -count=1
cd HelixQA && go test ./pkg/observe/frida -run TestHTTPBridge -count=1
```

Must verify:
- Frida server connection works
- Method hooking captures calls
- HTTP bridge relays data without loss
- No performance degradation on target app

**5.10 Task: HelixQA Configuration Loader**

Verify all 40+ environment variables load correctly:

```bash
cd HelixQA && go test ./pkg/config -run TestLoadFromEnv -count=1
cd HelixQA && go test ./pkg/config -run TestValidation -count=1
```

Critical env vars to verify:
- All 8 LLM provider API keys (OpenAI, Anthropic, Google, Groq, DeepSeek, xAI, Qwen)
- All 5 agent binary paths (OpenCode, Claude, Gemini, Junie, Qwen)
- Verifier strategy selection
- Autonomous session configuration
- Platform-specific settings (Android device, Desktop process, Web URL, API URL)
- Recording configuration (FFmpeg, quality, format)
- Resource limits (memory, goroutines)

**5.11 Task: Test Banks (`banks/`)**

HelixQA uses YAML test banks that define comprehensive test scenarios.

Verify all banks load and parse:

```bash
cd HelixQA && go test ./pkg/testbank -run TestBankLoad -count=1
cd HelixQA && go test ./pkg/testbank -run TestFullQA -count=1
```

Bank files to verify:
| Bank File | Coverage |
|-----------|----------|
| `banks/full-qa-api.yaml` | All API endpoints |
| `banks/full-qa-web.yaml` | All web screens + flows |
| `banks/full-qa-androidtv.yaml` | All TV screens + D-pad flows |
| `banks/full-qa-android.yaml` | All mobile screens + flows |
| `banks/full-qa-cross-platform.yaml` | Cross-platform sync |
| `banks/fixes-validation.yaml` | Regression tests for all fixed bugs |

**5.12 Task: Challenges (`challenges/`)**

The Challenges submodule provides structured test scenarios.

```bash
cd Challenges && go test ./... -count 1
cd HelixQA && go test ./challenges -run TestChallengeSuite -count=1
```

**5.13 Task: ReplayBuffer Integration**

```bash
cd ReplayBuffer && go test ./... -count 1
cd HelixQA && go test ./pkg/replay -run TestReplayIntegration -count=1
```

**5.14 Task: End-to-End HelixQA Validation**

The final test: Run HelixQA against Catalogizer itself.

```bash
# Full E2E: HelixQA validates catalog-web
cd HelixQA && go test ./tests/e2e -run TestE2E_WebQuick -count=1 -v

# Full E2E: HelixQA validates catalog-api
cd HelixQA && go test ./tests/e2e -run TestE2E_APIQuick -count=1 -v

# Full E2E: HelixQA validates Android TV (if device connected)
cd HelixQA && go test ./tests/e2e -run TestE2E_AndroidTV -count=1 -v

# Full campaign: All 10 test categories
cd HelixQA && ./scripts/helixqa-orchestrator.sh all
```

**Verification for Phase 5:**

```bash
# All AI stack submodules build
cd LLMsVerifier && go build ./...
cd LLMOrchestrator && go build ./...
cd LLMProvider && go build ./...
cd VisionEngine && go build ./...
cd DocProcessor && go build ./...
cd ScreenDiff && go build ./...
cd VisualRegression && go build ./...
cd TrainingCollector && go build ./...
cd ReplayBuffer && go build ./...

# HelixQA builds with all dependencies
cd HelixQA && go mod tidy && go build ./...

# All HelixQA unit tests pass
cd HelixQA && go test ./pkg/strategy ./pkg/recipe ./pkg/helixqa -count=1
cd HelixQA && go test ./pkg/autonomous ./pkg/navigator ./pkg/issuedetector -count=1
cd HelixQA && go test ./pkg/evidence ./pkg/ticket ./pkg/session -count=1
cd HelixQA && go test ./pkg/config ./pkg/testbank ./pkg/llm -count=1
cd HelixQA && go test ./pkg/vision ./pkg/visual ./pkg/replay -count=1
cd HelixQA && go test ./pkg/detector ./pkg/analysis ./pkg/performance -count=1
cd HelixQA && go test ./pkg/distributed ./pkg/learning ./pkg/planning -count=1
cd HelixQA && go test ./pkg/observe/frida -count=1 || echo "Frida requires device"
cd HelixQA && go test ./pkg/gpu/infer -count=1 || echo "GPU infer requires GPU"

# E2E tests pass
cd HelixQA && go test ./tests/e2e -count=1

# No race conditions
cd HelixQA && go test ./... -race -count=1 -short

# Banks are valid YAML and loadable
cd HelixQA && go test ./pkg/testbank -run TestAllBanks -count=1
```

**Exit Criteria:**
- [ ] LLMsVerifier: All strategies + 7 recipes passing tests
- [ ] LLMOrchestrator: All 5 agent adapters + pool working
- [ ] HelixQA autonomous session: Mapper, Navigator, Analyzer, Recorder, Ticket Generator all tested
- [ ] VisionEngine: Screen analysis + navigation graph working
- [ ] DocProcessor: All 5 formats (MD, YAML, HTML, ADOC, RST) parse correctly
- [ ] All 6 test banks load and are valid
- [ ] E2E tests: HelixQA can validate catalog-web and catalog-api
- [ ] Zero race conditions in HelixQA
- [ ] HelixQA can run a complete autonomous session end-to-end

---

### PHASE 6: Backend Integration Hardening (catalog-api)
**Duration: 10 days | Owner: Backend Lead + AI Agent**

**Goal:** Every endpoint in `catalog-api` works correctly with real dependencies, handles all error cases, and performs within budget.

**Entry Criteria:** Phase 4 complete (critical bugs fixed).

**5.1 Task: Endpoint-by-Endpoint Audit**

For EVERY endpoint in `catalog-api`:

1. **Happy path test** — Expected input → Expected output
2. **Error path test** — Invalid input → Proper error (not 500)
3. **Auth test** — No token → 401, Invalid token → 403
4. **Rate limit test** — Excessive requests → 429
5. **Database error test** — DB down → Graceful degradation
6. **Large payload test** — Max file size, max directory listing

**5.2 Task: Protocol Integration Matrix**

Test every CRUD operation across all protocols:

| Operation | Local | SMB | FTP | NFS | WebDAV |
|-----------|-------|-----|-----|-----|--------|
| Scan directory | Test | Test | Test | Test | Test |
| Read file | Test | Test | Test | Test | Test |
| Write file | Test | Test | Test | Test | Test |
| Delete file | Test | Test | Test | Test | Test |
| Rename file | Test | Test | Test | Test | Test |
| Large file (>1GB) | Test | Test | Test | Test | Test |
| Unicode filename | Test | Test | Test | Test | Test |
| Disconnection recovery | N/A | Test | Test | Test | Test |

**5.3 Task: Media Detection Accuracy**

1. Create test corpus with 100+ media files covering all 50+ media types
2. Run detection on each
3. Verify correct classification
4. Verify metadata extraction (title, year, quality, etc.)
5. Verify thumbnail generation

**5.4 Task: Database Migration Verification**

1. Test migration from v1.x → v2.x → current
2. Verify data integrity after migration
3. Test rollback procedure
4. Verify SQLCipher encryption on all DB files

**5.5 Task: WebSocket Real-Time Updates**

1. Connect 10 simultaneous clients
2. Trigger media scan → Verify all clients receive progress
3. Disconnect network → Verify reconnection
4. Verify no memory leaks with long-running connections

**Verification for Phase 5:**
```bash
# All backend tests pass
cd catalog-api && go test ./... -count=1 -race

# Integration tests with real dependencies
cd catalog-api && go test ./... -tags=integration -count=1

# Load test
k6 run tests/load/api-load-test.js

# Race detection
cd catalog-api && go test ./... -race -count=1
```

**Exit Criteria:**
- [ ] 100% endpoint coverage with happy + error paths
- [ ] All 5 protocols pass CRUD matrix
- [ ] Media detection ≥95% accuracy on test corpus
- [ ] WebSocket handles 10+ concurrent clients
- [ ] Zero race conditions detected

---

### PHASE 7: Frontend Integration Hardening (catalog-web)
**Duration: 8 days | Owner: Frontend Lead + AI Agent**

**Goal:** The web application works flawlessly with the real backend, handles all edge cases, and has zero console errors.

**6.1 Task: API Integration Verification**

1. Verify every API call in the frontend matches the backend contract
2. Verify error handling for every endpoint (network error, 4xx, 5xx)
3. Verify loading states for all async operations
4. Verify retry logic for failed requests

**6.2 Task: Real-Time Update Handling**

1. WebSocket connection establishes on login
2. Media scan progress updates in real-time
3. New media appears without page refresh
4. Connection drop → Reconnect → Updates resume

**6.3 Task: Cross-Browser Testing**

| Browser | Version | OS | Status |
|---------|---------|-----|--------|
| Chrome | Latest | macOS, Windows, Linux | Test |
| Firefox | Latest | macOS, Windows, Linux | Test |
| Safari | Latest | macOS, iOS | Test |
| Edge | Latest | Windows | Test |

Test on each:
1. Login → Dashboard → Media → Player → Settings
2. Verify zero console warnings/errors
3. Verify responsive layout at 1920x1080, 1366x768, 375x667

**6.4 Task: Accessibility Audit**

1. Run axe-core on every page
2. Verify keyboard navigation works throughout
3. Verify screen reader compatibility
4. Fix all WCAG 2.1 AA violations

**6.5 Task: Performance Budget Verification**

| Metric | Budget | Actual |
|--------|--------|--------|
| First Contentful Paint | < 1.5s | Measure |
| Time to Interactive | < 3.5s | Measure |
| Bundle size (gzipped) | < 500KB | Measure |
| API response time (p95) | < 200ms | Measure |

**Verification for Phase 6:**
```bash
# Build and type check
cd catalog-web && npm run build && npm run type-check

# Lint: zero warnings
cd catalog-web && npm run lint

# All tests
cd catalog-web && npm test

# E2E tests
cd catalog-web && npm run test:e2e

# Accessibility audit
npx axe http://localhost:3000

# Performance
npx lighthouse http://localhost:3000 --preset=desktop
```

**Exit Criteria:**
- [ ] Zero ESLint warnings
- [ ] Zero TypeScript errors
- [ ] All 2,334+ tests passing
- [ ] E2E tests pass on Chrome, Firefox, Safari
- [ ] Lighthouse score ≥90 on all categories
- [ ] Zero accessibility violations

---

### PHASE 8: Android Mobile Hardening (catalogizer-android)
**Duration: 8 days | Owner: Mobile Lead + AI Agent**

**7.1 Task: Device Testing Matrix**

| Device | Android Version | Screen Size | Test Status |
|--------|----------------|-------------|-------------|
| Pixel 8 | Android 15 | 6.7" | Verify |
| Pixel 6 | Android 14 | 6.4" | Verify |
| Samsung Galaxy S23 | Android 14 | 6.1" | Verify |
| Xiaomi Mi 11 | Android 13 | 6.81" | Verify |
| Emulator (small) | Android 10 | 5" | Verify |

**7.2 Task: Complete User Journey Tests**

1. Fresh install → Onboarding → Login → Dashboard
2. Add SMB source → Scan → Browse detected media → Play video
3. Add FTP source → Browse → Download for offline → Play offline
4. Search → Filter by type → Sort by date → Open detail
5. Settings → Change theme → Change language → Verify persistence
6. Background playback → Notification controls → Kill app → Resume
7. Offline mode → Cached content works → Sync when online

**7.3 Task: Permission Flow Testing**

1. Deny storage permission → App handles gracefully
2. Grant permission → Full functionality
3. Revoke permission while app running → Graceful degradation
4. Android 13+ scoped storage → Verify MediaStore usage

**7.4 Task: Background Sync Testing**

1. Enable auto-sync
2. Add new media to source
3. Verify notification appears
4. Verify new media appears in app
5. Verify sync doesn't drain battery (JobScheduler/WorkManager)

**Verification for Phase 7:**
```bash
# Build success
./gradlew :app:assembleDebug :app:assembleRelease

# Unit tests
./gradlew testDebugUnitTest

# Lint
./gradlew lintDebug

# Instrumented tests (on connected device)
./gradlew connectedDebugAndroidTest

# No crashes in logcat
adb logcat -d | grep -i "fatal\|crash\|ANR" | wc -l  # Must be 0
```

**Exit Criteria:**
- [ ] Build successful for debug and release
- [ ] All unit tests passing
- [ ] All instrumented tests passing
- [ ] Zero crashes on 5+ test devices
- [ ] All user journeys verified
- [ ] Scoped storage compliant on Android 13+

---

### PHASE 9: Android TV Hardening (catalogizer-androidtv)
**Duration: 7 days | Owner: TV Lead + AI Agent**

**8.1 Task: TV-Specific Device Matrix**

| Device | Android Version | Test Status |
|--------|----------------|-------------|
| Mi Box 4 (Real) | Android 9 | Primary test device |
| Chromecast with Google TV | Android 12 | Verify |
| NVIDIA Shield TV | Android 11 | Verify |
| Android TV Emulator | Android 14 | CI testing |

**8.2 Task: D-Pad Navigation Audit**

1. Navigate every screen using only D-pad
2. Verify focus is visible on all focusable elements
3. Verify logical focus traversal (no jumps, no traps)
4. Verify back button behavior on every screen
5. Verify fastlane/channel integration

**8.3 Task: Leanback UI Compliance**

1. Use BrowseSupportFragment for browsing
2. Use DetailsSupportFragment for media details
3. Use PlaybackSupportFragment for playback
4. Use SearchSupportFragment for search
5. Verify recommendations row on home screen

**8.4 Task: Playback Testing**

1. Play video → Pause → Resume → Seek → Stop
2. Audio track switching
3. Subtitle track switching
4. 4K/HDR content playback
5. Background audio playback (picture-in-picture if supported)

**Verification for Phase 8:**
```bash
# TV build
./gradlew :app:assembleDebug :app:assembleRelease

# TV lint
./gradlew lintTvDebug

# Tests
./gradlew testTvDebugUnitTest

# Real device: D-pad navigation video recorded
# Real device: Logcat shows zero crashes
adb logcat -d | grep -i "fatal\|crash\|ANR" | wc -l  # Must be 0
```

**Exit Criteria:**
- [ ] App navigable with D-pad only
- [ ] All Leanback fragments used correctly
- [ ] Playback works on Mi Box 4 (real device)
- [ ] Zero crashes, zero ANRs
- [ ] Video recording of complete D-pad navigation session

---

### PHASE 10: Desktop Hardening (catalogizer-desktop)
**Duration: 5 days | Owner: Desktop Lead + AI Agent**

**9.1 Task: Platform Matrix**

| OS | Architecture | Test Status |
|-----|-------------|-------------|
| macOS (Intel) | x86_64 | Verify |
| macOS (Apple Silicon) | aarch64 | Verify |
| Windows 11 | x86_64 | Verify |
| Linux (Ubuntu) | x86_64 | Verify |

**9.2 Task: Tauri-Specific Testing**

1. Auto-updater works (check update → download → install)
2. Native file dialogs work (open directory, save file)
3. System tray integration works
4. Keyboard shortcuts work (Ctrl/Cmd+ shortcuts)
5. Window state persists (size, position)

**9.3 Task: Desktop-Specific Features**

1. Drag-and-drop media files
2. Native file system browsing
3. Hardware-accelerated video playback
4. Multi-window support (if applicable)

**Verification for Phase 9:**
```bash
# Tauri build
cd catalogizer-desktop && npm run tauri build

# No unwrap() in Rust code
! grep -r "unwrap()" src-tauri/src/ | grep -v "// SAFE"

# Desktop E2E tests (if available)
npm run test:e2e:desktop
```

**Exit Criteria:**
- [ ] Builds on all 4 platform/architecture combinations
- [ ] Auto-updater functional
- [ ] Zero Rust unwrap() calls (or all documented as safe)
- [ ] Native integrations tested

---

### PHASE 11: Cross-Platform Contract Validation
**Duration: 5 days | Owner: Integration Lead + AI Agent**

**10.1 Task: API Contract Cross-Verification**

Verify that the same API request produces compatible responses across all clients:

```bash
# Test login across all clients
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'
# Verify: All clients can parse this response

# Test media list
curl http://localhost:8080/api/v1/media?page=1&limit=20 \
  -H "Authorization: Bearer $TOKEN"
# Verify: All clients render this correctly
```

**10.2 Task: Real-Time Sync Verification**

1. Open web app → Open Android app → Open TV app
2. Add media source from web
3. Verify: All clients show new source without refresh
4. Play media on one client
5. Verify: Other clients show "now playing" (if applicable)

**10.3 Task: Settings Sync Verification**

1. Change setting on web → Verify on Android
2. Change setting on Android → Verify on TV
3. Verify settings persist across app restarts

**Verification for Phase 10:**
```bash
# Run full cross-platform test suite
cd tests/cross-platform && go test ./... -count=1 -v
```

**Exit Criteria:**
- [ ] All API responses parseable by all clients
- [ ] Real-time sync works across web + mobile + TV
- [ ] Settings sync bidirectionally

---

### PHASE 12: Security Hardening & Penetration Testing
**Duration: 5 days | Owner: Security Lead + AI Agent**

**11.1 Task: Authentication Security**

1. JWT token: Verify proper expiration, refresh token rotation
2. Password policy: Enforce minimum complexity
3. Brute force protection: Account lockout after N attempts
4. Session management: Concurrent session limit
5. Logout: Token invalidation on server

**11.2 Task: API Security**

1. SQL injection: Test all endpoints with `' OR 1=1 --`
2. XSS: Test all inputs with `<script>alert(1)</script>`
3. CSRF: Verify CSRF tokens on state-changing requests
4. Path traversal: Test `../../../etc/passwd` on file endpoints
5. Content-Type validation: Reject unexpected content types

**11.3 Task: Infrastructure Security**

1. Secrets: Verify no secrets in code (gitleaks scan)
2. Dependencies: Verify no known vulnerabilities (trivy, nancy)
3. Container: Non-root user, minimal image, no secrets in layers
4. Network: TLS 1.3, HSTS headers, secure cookies

**Verification for Phase 11:**
```bash
# Security scanning
gitleaks detect --source .
trivy filesystem .
nancy go.sum
gosec ./...
semgrep --config=auto .

# Penetration test results documented
[ -s docs/security/PENTEST_REPORT.md ]
```

**Exit Criteria:**
- [ ] Zero secrets in codebase
- [ ] Zero high/critical vulnerabilities in dependencies
- [ ] All OWASP Top 10 addressed with tests
- [ ] Penetration test report with all findings fixed

---

### PHASE 13: Performance Optimization & Stress Testing
**Duration: 5 days | Owner: Performance Lead + AI Agent**

**12.1 Task: API Performance Benchmarks**

| Endpoint | Target (p50) | Target (p95) | Target (p99) |
|----------|-------------|-------------|-------------|
| GET /api/v1/media | 50ms | 100ms | 200ms |
| POST /api/v1/auth/login | 100ms | 200ms | 500ms |
| GET /api/v1/media/:id | 30ms | 50ms | 100ms |
| WebSocket events | 10ms | 20ms | 50ms |

**12.2 Task: Load Testing**

```bash
# k6 load test: 100 concurrent users for 10 minutes
k6 run --vus 100 --duration 10m tests/load/media-browse-load.js

# Verify: No 5xx errors, p95 < target, memory stable
```

**12.3 Task: Large Library Testing**

1. Import 10,000 media files
2. Verify scan completes in < 1 hour
3. Verify browsing remains responsive
4. Verify search returns results in < 500ms
5. Verify memory usage stays < 2GB

**Verification for Phase 12:**
```bash
# Performance benchmarks pass
cd catalog-api && go test ./... -bench=. -benchtime=10s

# Load test passes
k6 run tests/load/full-load-test.js

# Memory profiling shows no leaks
go tool pprof heap.prof
```

**Exit Criteria:**
- [ ] All API endpoints meet performance targets
- [ ] 100 concurrent users for 10 minutes with zero errors
- [ ] 10,000 media library scan completes in < 1 hour
- [ ] No memory leaks detected

---

### PHASE 14: Documentation Completion & Video Course
**Duration: 10 days | Owner: Documentation Lead + AI Agent**

**13.1 Task: Documentation Audit**

Verify completeness of existing documentation:

| Document | Status | Action |
|----------|--------|--------|
| README.md | Exists | Update to reflect actual state |
| GETTING_STARTED.md | Exists | Verify accuracy with fresh install |
| API documentation | Partial | Complete all endpoints |
| Developer guide | Partial | Complete architecture guide |
| User manual | Missing | Create platform-specific guides |
| Troubleshooting | Partial | Expand with real issues found |
| CONTRIBUTING.md | Exists | Update with new test requirements |

**13.2 Task: Video Course Recording**

Record 5 modules:
1. Introduction (15 min): What is Catalogizer, architecture overview
2. Configuration (25 min): Setting up protocols, environment variables
3. Running Sessions (30 min): Web, Android, TV, Desktop walkthroughs
4. Advanced Features (35 min): API integration, automation, HelixQA
5. Troubleshooting (20 min): Common issues, debugging, log analysis

**13.3 Task: Architecture Diagrams**

Create/update:
1. System architecture diagram
2. Data flow diagram
3. Sequence diagrams for critical flows
4. Deployment diagram

**Verification for Phase 13:**
```bash
# All docs build without errors
[ -s docs/README.md ]
[ -s docs/guides/getting-started.md ]
[ -s docs/guides/configuration.md ]
[ $(find docs/ -name "*.md" | wc -l) -ge 30 ]

# Video files exist
[ $(find docs/video-course/ -name "*.mp4" | wc -l) -eq 5 ]
```

**Exit Criteria:**
- [ ] All documentation accurate and verified
- [ ] 5 video modules recorded
- [ ] Architecture diagrams created
- [ ] Fresh install from docs succeeds

---

### PHASE 15: Final Integration, Deployment & Sign-Off
**Duration: 5 days | Owner: Project Lead + Full Team**

**14.1 Task: Full-QA Master Cycle (CONSTITUTION.md Article VII)**

Execute the complete cycle:

1. **Clean rebuild**: `make clean && make build-all`
2. **All tests**: Unit → Integration → E2E → Security → Stress
3. **All Challenges**: Run full challenge suite
4. **All HelixQA banks**: Run autonomous QA on all platforms
5. **Autonomous QA per platform**: Web, Android, Android TV, Desktop
6. **Video + Screenshot review**: Review all session recordings
7. **Tickets**: File tickets for any findings
8. **Root-cause fix**: Fix → unit test + fixes-validation entry + HelixQA bank entry + challenge
9. **Rebuild → Repeat**: Until clean pass

**14.2 Task: Version Bump & Release**

1. Update `versions.json` for all components
2. Create Git tags for all repositories
3. Write release notes
4. Create GitHub release with binaries
5. Update submodule references to release versions

**14.3 Task: Deployment Verification**

1. Deploy to staging environment
2. Run smoke tests
3. Deploy to production
4. Monitor for 24 hours
5. Verify all metrics (error rate, latency, user actions)

**14.4 Task: Final Sign-Off Checklist**

| # | Item | Verified By | Date |
|---|------|-------------|------|
| 1 | All 7 GitHub issues closed with proof | | |
| 2 | All disabled features re-enabled and tested | | |
| 3 | All critical bugs fixed with regression tests | | |
| 4 | 100% endpoint coverage | | |
| 5 | All 5 protocols tested with real servers | | |
| 6 | All 4 clients (web, android, tv, desktop) tested | | |
| 7 | Security audit passed | | |
| 8 | Performance benchmarks met | | |
| 9 | Documentation complete | | |
| 10 | Video course published | | |
| 11 | Full-QA Master Cycle clean pass | | |
| 12 | Deployment verified in production | | |

**Verification for Phase 14:**
```bash
# All components build
cd catalog-api && go build ./...
cd catalog-web && npm run build
cd catalogizer-android && ./gradlew assembleRelease
cd catalogizer-desktop && npm run tauri build

# All tests pass
cd catalog-api && go test ./... -count=1
cd catalog-web && npm test
cd catalogizer-android && ./gradlew test

# Version file matches
jq -r '.version' versions.json

# Zero open issues
[ $(gh issue list --state open | wc -l) -eq 0 ]
```

**Exit Criteria:**
- [ ] All 12 sign-off items verified
- [ ] Full-QA Master Cycle clean pass
- [ ] Production deployment stable for 24 hours
- [ ] All 7 GitHub issues closed

---

## 6. Cross-Component Integration Matrix

### 6.1 Dependency Graph

```
catalog-api (Go backend)
├── 21 Go submodules (digital.vasic.*)
│   ├── Auth, Cache, Config, Database, Filesystem, Media, ...
├── External services
│   ├── TMDB, IMDB, TVDB, MusicBrainz, Spotify, Steam
│   ├── Redis (caching, rate limiting)
│   └── SQLCipher (encrypted database)
│
├── Clients
│   ├── catalog-web (React/TS)
│   │   └── 9 TS submodules (@vasic-digital/*)
│   ├── catalogizer-android (Kotlin/Compose)
│   ├── catalogizer-androidtv (Kotlin/Leanback)
│   ├── catalogizer-desktop (Tauri/Rust+React)
│   └── catalogizer-api-client (TS library)
│
├── QA/AI Stack
│   ├── HelixQA (autonomous testing)
│   ├── LLMsVerifier (LLM strategy)
│   ├── LLMOrchestrator (multi-provider)
│   ├── DocProcessor (document processing)
│   └── VisionEngine (image analysis)
│
└── Infrastructure
    ├── Docker (containerization)
    ├── Monitoring (Prometheus/Grafana)
    └── Deployment (scripts, configs)
```

### 6.2 Change Impact Matrix

When modifying a component, these tests MUST run:

| Component Change | Must Test |
|-----------------|-----------|
| Any Go submodule | catalog-api build + all integration tests |
| catalog-api endpoint | Contract tests + all client E2E tests |
| catalog-web | Build + lint + unit + E2E |
| catalogizer-android | Build + unit + instrumented |
| catalogizer-androidtv | Build + TV-specific tests + D-pad nav |
| catalogizer-desktop | Tauri build + platform tests |
| Any shared TS submodule | All TS projects build + test |
| HelixQA | Run HelixQA against all platforms |
| Database schema | Migration test + all backend tests |

---

## 7. The Verification Pyramid

### 7.1 Testing Hierarchy

```
                    /\
                   /  \
                  / E2E \          <- Full user journeys (Phase 2, 10)
                 /--------\
                / Contract \      <- API contracts (Phase 2, 10)
               /------------\
              / Integration  \    <- Real dependencies (Phase 2, 5-9)
             /----------------\
            /    Property-Based \ <- Invariants always hold (Phase 5)
           /----------------------\
          /       Unit Tests        \ <- Functions in isolation (existing)
         /--------------------------\
```

### 7.2 Quality Gates

| Gate | Criteria | Enforced By |
|------|----------|-------------|
| Commit | Lint + Unit tests pass | Pre-commit hooks |
| PR | All tests + Code review | CI pipeline |
| Merge | Integration tests + LLM Judge | Protected branch rules |
| Release | E2E tests + Security scan + Performance | Release pipeline |
| Deploy | Full-QA Master Cycle clean pass | Manual + HelixQA |

### 7.3 The Verification Loop (Per CONSTITUTION.md)

```
┌─────────────┐    ┌──────────────┐    ┌───────────────┐
│  IMPLEMENT  │───>│   VERIFY     │───>│    REPORT     │
│  (AI Agent) │    │ (Run Tests)  │    │  (Results)    │
└─────────────┘    └──────────────┘    └───────┬───────┘
      ▲                                        │
      └────────────────────────────────────────┘
                   If FAIL: Fix + Retry
                   If PASS: Proceed
                   If BLOCKED: Escalate
```

---

## 8. Operational Artifacts & Templates

### 8.1 Verification Command Reference

**Backend (catalog-api):**
```bash
# Full verification suite
cd catalog-api && \
  go fmt ./... && \
  go vet ./... && \
  golangci-lint run --config .golangci.yml ./... && \
  go test -short -count=1 ./... && \
  go test -tags=integration -count=1 ./... && \
  go test -race -count=1 ./...
```

**Frontend (catalog-web):**
```bash
cd catalog-web && \
  npm run lint && \
  npm run type-check && \
  npm test && \
  npm run test:e2e && \
  npm run build
```

**Android:**
```bash
cd catalogizer-android && \
  ./gradlew ktlintCheck && \
  ./gradlew lintDebug && \
  ./gradlew testDebugUnitTest && \
  ./gradlew connectedDebugAndroidTest && \
  ./gradlew assembleRelease
```

**Android TV:**
```bash
cd catalogizer-androidtv && \
  ./gradlew lintTvDebug && \
  ./gradlew testTvDebugUnitTest && \
  ./gradlew assembleTvRelease
```

### 8.2 AI Task Assignment Template

```
[SYSTEM DIRECTIVE: VERIFICATION MODE ENABLED]

TASK: [FEATURE NAME] in [MODULE NAME].

REQUIRED READING (read before any code change):
1. docs/LANDMINES.md — Section [PLATFORM]
2. docs/API_CONTRACTS.md — Endpoint [NAME]
3. CONSTITUTION.md — Article VII (Full-QA Master Cycle)

VERIFICATION COMMANDS (must all exit 0):
[Insert commands from Section 8.1]

CONSTRAINTS:
- Do not claim task complete until ALL verification commands pass
- Do not remove or comment out error handling
- Do not add t.Skip() without linking to an open issue
- Update docs/LANDMINES.md if you discover a new rule

If verification fails:
- Step A: Read the error log
- Step B: Fix the error
- Step C: Re-run verification
- Step D: If still failing after 2 attempts, output: "VERIFICATION BLOCKED: [error]"

BEGIN TASK.
```

### 8.3 LLM-as-Judge Pre-Merge Template

```
ROLE: Senior Software Architect & QA Gatekeeper.
CONTEXT: Reviewing a pull request for the Catalogizer project.

INPUT:
- PR Diff: [PASTE GIT DIFF]
- Landmine Rules: [PASTE docs/LANDMINES.md]
- API Contracts: [PASTE relevant section of docs/API_CONTRACTS.md]

REVIEW CHECKLIST:
1. Does this change violate any LANDMINES rules? (Y/N + which)
2. Does this change modify any public API contract? (Y/N + diff)
3. Are all new functions covered by tests? (Y/N + coverage %)
4. Are there any unwrap(), t.Skip(), or .disabled additions? (Y/N)
5. Does this change handle all error cases? (Y/N + list)
6. Are there any security implications? (Y/N + details)

OUTPUT FORMAT (JSON):
{
  "veto": true/false,
  "severity": "BLOCKER" / "WARNING" / "INFO",
  "violations": [
    {
      "rule": "RULE-ID",
      "description": "What was violated",
      "fix_suggestion": "How to fix"
    }
  ],
  "risk_assessment": "Low / Medium / High - Explanation."
}

CRITICAL: If you are less than 95% confident this code works on Android TV, veto: true.
```

### 8.4 Bug Retrospective Template

When ANY bug is found in manual testing:

```markdown
## Bug Report: [DATE] — [BRIEF DESCRIPTION]

### What the user did:
1. [Step 1]
2. [Step 2]
3. [Result: What went wrong]

### What the automated tests did right:
- [Which tests passed that should have caught this]

### Why the tests failed to catch this (Root Cause):
- **Missing Constraint:** [What rule was violated]
- **Detection Gap:** [What test type was missing]
- **Coverage Gap:** [What wasn't tested]

### New Landmine Rule:
**RULE-[PLATFORM]-[NNN]: [Rule Name]**
- Context: [Why this matters]
- Detection: [How to check]
- Fix: [How to fix]

### Tests Added:
- [ ] Unit test: [description]
- [ ] Integration test: [description]
- [ ] E2E test: [description]
- [ ] Regression entry in banks/fixes-validation.yaml

### Evidence:
- Screenshot: [path]
- Video: [path]
- Log excerpt: [path]
```

---

## 9. Appendices

### Appendix A: Issue-to-Phase Mapping

| GitHub Issue | Phase(s) | Real Completion Criteria |
|-------------|----------|------------------------|
| #2: LLMsVerifier Strategy | 1, 5 | Strategy working in production QA runs |
| #7: OpenCode Headless | 1, 5 | Multi-provider E2E with real LLMs |
| #3: Enhanced Autonomous Session | 2, 5 | HelixQA runs end-to-end without human intervention |
| #5: Comprehensive Testing | 2, 4, 5, 11, 12, 13 | All 10 test categories passing |
| #6: Configuration | 1, 14 | OPEN_POINTS_CLOSURE.md shows all items ticked |
| #4: Documentation | 14 | 5 video modules published |
| #8: Final Integration | 15 | Full-QA Master Cycle clean pass + production stable 24h |

### Appendix B: Time Estimates

| Phase | Duration | Cumulative |
|-------|----------|------------|
| Phase 1: Institutional Knowledge | 3 days | 3 days |
| Phase 2: Test Infrastructure | 5 days | 8 days |
| Phase 3: Disabled Features | 10 days | 18 days |
| Phase 4: Critical Bugs | 7 days | 25 days |
| Phase 5: HelixQA & AI Stack | 14 days | 39 days |
| Phase 6: Backend Hardening | 10 days | 49 days |
| Phase 7: Frontend Hardening | 8 days | 57 days |
| Phase 8: Android Mobile | 8 days | 65 days |
| Phase 9: Android TV | 7 days | 72 days |
| Phase 10: Desktop | 5 days | 77 days |
| Phase 11: Cross-Platform | 5 days | 82 days |
| Phase 12: Security | 5 days | 87 days |
| Phase 13: Performance | 5 days | 92 days |
| Phase 14: Documentation | 10 days | 102 days |
| Phase 15: Final Integration | 5 days | **107 days (~15 weeks)** |

### Appendix C: Key Files Reference

| File | Purpose | Must Read Before |
|------|---------|-----------------|
| `CLAUDE.md` | AI operating manual | Every AI session |
| `CONSTITUTION.md` | Non-negotiable rules | Every AI session |
| `AGENTS.md` | Agent constraints | Every AI session |
| `docs/LANDMINES.md` | Production rules | Every code change |
| `docs/API_CONTRACTS.md` | API specifications | API changes |
| `versions.json` | Component versions | Releases |
| `docker-compose.test-infra.yml` | Test infrastructure | Integration tests |

### Appendix D: Definition of "Done"

For this project, "done" means:

1. **Code**: Compiles with zero warnings on all platforms
2. **Tests**: All 10 categories passing (unit, integration, E2E, automation, stress, security, DDoS/rate-limit, benchmarking, challenges, HelixQA)
3. **Features**: Zero `.disabled` files (or all justified)
4. **Bugs**: Zero open critical/high bugs
5. **Security**: All scans clean, pentest passed
6. **Performance**: All benchmarks met
7. **Docs**: All documentation accurate, video course published
8. **Integration**: Full-QA Master Cycle clean pass
9. **Deployment**: Production stable for 24+ hours
10. **Sign-off**: All 12 checklist items verified by human

---

## End of Plan

**This plan was generated on 2026-04-22 based on comprehensive analysis of:**
- The Catalogizer GitHub repository (1,004+ commits, 41 submodules, 7 open issues)
- The `Fixing_big_projects.md` methodology document
- The project's own `CONSTITUTION.md`, `CLAUDE.md`, and extensive documentation
- All open issues (#2 through #8) with their claimed vs. actual status

**Execution starts immediately upon approval.**
