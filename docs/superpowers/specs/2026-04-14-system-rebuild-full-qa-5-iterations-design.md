# System-Wide Rebuild & 5-Iteration Full QA Campaign Design

## Metadata
- **Date:** April 14, 2026  
- **Author:** AI Agent (via brainstorming skill)  
- **Status:** Approved for implementation  
- **Target:** Complete Catalogizer ecosystem rebuild and validation  

## 1. Executive Summary

### Objective
Perform a clean rebuild of the entire Catalogizer ecosystem with latest submodule integrations, distribute all components to target hosts, and execute 5 iterative QA sessions with mandatory video/log analysis and root-cause fixes between iterations.

### Success Criteria
1. **100% submodule integration:** All 41 submodules updated and properly wired
2. **Clean debug builds:** All 7 main applications built successfully
3. **Complete distribution:** Container images deployed, APKs installed on all target devices
4. **5 QA iterations:** Full HelixQA sessions across all platforms with Android TV priority
5. **Zero defects:** All discovered issues fixed between iterations, no outstanding issues
6. **100% test coverage:** Maintained across all 10 test categories (Constitution Article V)

## 2. System Architecture

### Components to Rebuild & Test

| Component | Technology | Build Type | Distribution Target |
|-----------|------------|------------|---------------------|
| **41 Submodules** | Go (23) / TS (9) / AI (9) | Source integration | Linked into main apps |
| **catalog-api** | Go 1.25 / Gin / HTTP3 | Debug binary + container | Container hosts |
| **catalog-web** | React 18 / Vite | Debug build + container | Container hosts |
| **catalogizer-desktop** | Tauri (Rust + React) | Debug app | Local testing |
| **installer-wizard** | Tauri (Rust + React) | Debug app | Local testing |
| **catalogizer-android** | Kotlin / Jetpack Compose | Debug APK | ADB devices |
| **catalogizer-androidtv** | Kotlin / Jetpack Compose | Debug APK | Android TV devices (2) |
| **catalogizer-api-client** | TypeScript | Library | Linked into web/desktop |

### Build Environment
- **Containers-based builder:** `docker-compose.build.yml` with `catalogizer-builder` container
- **Build type:** Debug builds for all components (enables comprehensive logging)
- **Runtime:** Podman exclusively (no Docker)
- **Resource limits:** 50-60% host CPU/memory maximum

### QA Environment
- **HelixQA iterations:** 5 complete rounds with self-learning capability
- **Platform priority:** Android TV → Android → Web → Desktop → API
- **Video recording:** Mandatory 16Mbps, 1920x1080, frame-by-frame analysis
- **Log monitoring:** Real-time `adb logcat`, browser console, service logs
- **Analysis:** Post-session review of all recorded materials (assets)

## 3. Pipeline Design (Approach 1: Comprehensive Sequential)

### Phase 1: Submodule Analysis & Integration
1. **Fetch latest submodules:** `git submodule update --remote --recursive` (historical; do not run it through `constitution/`, use `scripts/safe_submodule_init.sh`; `--remote` moves third-party pins unreviewed)
2. **Change analysis:** Diff each of 41 submodules for breaking changes
3. **Integration verification:**
   - Go modules: Update `catalog-api/go.mod` replace directives
   - TypeScript modules: Update `catalog-web/package.json` file links
   - Wire new APIs/services into main applications
4. **Build verification:** Quick compile check on each submodule

### Phase 2: Container-Based Build Process
Using `scripts/release-build.sh --container --force --skip-tests`:
1. **Build environment:** Start builder container (`catalogizer-builder`)
2. **Parallel component builds:**
   - Go backend: `catalog-api` binary + container image
   - React frontend: `catalog-web` production build + container
   - Android apps: APKs via builder container (debug builds)
   - Desktop apps: Tauri debug builds
3. **Artifact collection:** All binaries/images saved to `build/` directory

### Phase 3: Distribution & Deployment
Using `scripts/full-distribute.sh --all`:
1. **Container distribution:** Push images to configured hosts
2. **Service startup:** Start catalog-api, catalog-web, PostgreSQL, Redis
3. **APK installation:** Install debug APKs on connected devices (via ADB)
   - Check `.devignore` for excluded devices (ATMOSphere prohibited)
   - Use `.devconnect` for auto-connection (192.168.0.214:5555)
4. **Health verification:** All services respond, apps launch successfully

### Phase 4: 5-Iteration QA Loop
For iteration = 1 to 5:
1. **Pre-flight:** `.devconnect` auto-connect, check device availability
2. **Android TV QA:** `scripts/run-helixqa-androidtv.sh` with video/log capture
3. **Android mobile QA:** `scripts/run-helixqa-android.sh`
4. **Web QA:** `scripts/run-helixqa-web.sh`
5. **Desktop QA:** `scripts/run-helixqa-desktop.sh`
6. **API QA:** `scripts/run-helixqa-api.sh`
7. **Post-session analysis:** Video review, log analysis, issue triage
8. **Root cause fixes:** Fix all discovered issues before next iteration

## 4. Error Handling & Rollback Strategy

### Build Failures
- **Submodule integration fails:** Rollback to previous working commit, create ticket
- **Compilation errors:** Stop pipeline, fix code, restart from submodule analysis
- **Container build fails:** Clean builder cache, rebuild with verbose logging

### Distribution Failures
- **Host unreachable:** Skip that host, continue with others, log warning
- **APK installation fails:** Retry 3 times, then mark device as unavailable
- **Service startup fails:** Capture container logs, attempt restart with debug flags

### QA Session Failures
- **HelixQA crash:** Restart with `--debug` flag, capture stack trace
- **Device disconnection:** Auto-reconnect via `.devconnect`, resume from last step
- **ANR/crash detection:** Immediate pause, capture logs/video, fix root cause

### Rollback Capabilities
- **Build artifacts:** Version-tagged in `build/` directory
- **Container images:** Tagged with commit hash + timestamp
- **Database:** Backup before major changes
- **APKs:** Previous working version kept for emergency rollback

## 5. Testing Strategy & Coverage Validation

### Pre-QA Test Suite (run before each iteration)
1. **Unit tests:** All `*_test.go` files with resource limits (`GOMAXPROCS=3`)
2. **Integration tests:** `catalog-api/tests/integration/` with test containers
3. **E2E tests:** Playwright for web, Espresso for Android
4. **Challenge tests:** All registered challenges via API
5. **Security scans:** `govulncheck`, `npm audit`, Semgrep, Trivy

### Coverage Requirements (Constitution Article V)
- ✅ **Unit tests:** 100% coverage
- ✅ **Integration tests:** 100% coverage  
- ✅ **E2E tests:** 100% coverage
- ✅ **Stress tests:** All components
- ✅ **Security tests:** All categories
- ✅ **DDoS/rate-limit tests:** Functional
- ✅ **Benchmarking:** Baseline + regression detection
- ✅ **Challenges:** Registered per feature
- ✅ **HelixQA:** Bank entry per screen/flow

### Validation Loop
```
Test Failure → Root Cause Analysis → Fix → Regression Test → Re-run All Tests
```
- **No partial fixes** - complete implementations only
- **No TODOs** - zero tolerance for unfinished work
- **No silent error ignoring** - proper error handling mandatory

## 6. Resource Management & Constraints

### Host Resource Limits (50-60% maximum)
- **Container CPU limits:**
  - PostgreSQL: `--cpus=1`
  - API: `--cpus=2`
  - Web: `--cpus=1`
  - Builder: `--cpus=3`
- **Memory limits:** Total ≤ 8GB across all containers
- **Test concurrency:** `GOMAXPROCS=3 go test ./... -p 2 -parallel 2`

### Device Management
- **`.devignore` compliance:** Never test on ATMOSphere devices
- **`.devconnect` auto-connect:** Script runs before each QA session
- **Video recording:** 16Mbps, 1920x1080, frame-by-frame analysis
- **Log monitoring:** Real-time `adb logcat` + service logs

### Storage Requirements
- **Build artifacts:** `build/` directory (10-15GB)
- **QA recordings:** `qa-results/` directory (2-5GB per iteration)
- **Log archives:** `docs/reports/qa-sessions/` (1-2GB)

## 7. Key Constraints & Compliance

### CRITICAL: `.devignore` Devices - NEVER USE FOR TESTING
- **ATMOSphere devices are explicitly excluded** - never install or test on them
- **Always check `.devignore` before any ADB device operation**
- **Only use devices NOT matching any pattern in `.devignore`**

### CRITICAL: HelixQA ONLY for UI/UX Automated Testing
- **ALL automated UI/UX testing MUST be performed exclusively by HelixQA**
- **NEVER write custom scripts for UI testing** (no ADB tap sequences)
- **Every UI interaction MUST be LLM vision-driven:** screenshot → LLM analysis → action decision

### CRITICAL: Real-Time Log Monitoring During ALL QA Sessions
- **Active `adb logcat` monitoring during HelixQA execution**
- **Browser console logs must be captured and streamed in real-time**
- **ANR/crash detection must pause QA session immediately**

### CRITICAL: Universal Solution Principle
- **NEVER add test-only code to the application under test**
- **ALWAYS implement fixes in the testing tool/infrastructure** (HelixQA)
- **Target applications require ZERO modifications for testing**

### CRITICAL: Device Auto-Connect via `.devconnect`
- **Ensure Android TV devices are connected BEFORE running HelixQA**
- **Run `./scripts/devconnect.sh` before every HelixQA session**
- **Validation:** Script pings devices first, only connects reachable devices

### CRITICAL: ZERO UNFINISHED WORK POLICY
- **NO unfinished work, TODOs, or known issues may remain in the codebase**
- **Fix ALL discovered issues immediately** - no deferrals
- **Complete implementations before committing**

## 8. Success Metrics & Exit Criteria

### Success Metrics
1. **Build success rate:** 100% of components built successfully
2. **Distribution success:** All hosts receive correct versions
3. **Test pass rate:** 100% across all test categories
4. **QA iteration completion:** 5 full iterations with fixes between
5. **Zero defects:** No open issues after final iteration

### Exit Criteria (all must be true)
- ✅ All 41 submodules integrated and wired
- ✅ All 7 main applications built (debug)
- ✅ Container images deployed to all hosts
- ✅ APKs installed on all target devices
- ✅ 5 HelixQA iterations completed
- ✅ All discovered issues fixed (zero outstanding)
- ✅ 100% test coverage maintained in all categories
- ✅ Zero console warnings/errors in all environments
- ✅ No ANR/crashes in final iteration
- ✅ All recorded materials analyzed and documented

## 9. Implementation Notes

### Script Usage
- **Build:** `./scripts/release-build.sh --container --force --skip-tests`
- **Distribute:** `./scripts/full-distribute.sh --all`
- **QA:** `./scripts/run-helixqa-all.sh` (or platform-specific variants)
- **Device connect:** `./scripts/devconnect.sh`

### Environment Configuration
- **Podman required** (no Docker)
- **Debug builds enabled** for comprehensive logging
- **HTTP/3 (QUIC) with Brotli compression** mandatory
- **Container resource limits** enforced

### Documentation Requirements
- **QA session reports:** `docs/reports/qa-sessions/qa-session-<date>/`
- **Issue tracking:** Create tickets for all defects
- **Fix validation:** Add regression tests to `fixes-validation` bank

## 10. Approval & Next Steps

**Design Status:** ✅ Approved by user

**Next Action:** Invoke `writing-plans` skill to create detailed implementation plan

**Git Commit:** This spec will be committed to version control before proceeding