#!/bin/bash
# ============================================================
# Catalogizer - Containerized Build, Test & Release Script
# Runs INSIDE the builder container (docker/Dockerfile.builder)
# Orchestrates all testing, building, and artifact collection
# ============================================================

set -euo pipefail

# BTR_PROJECT_DIR is a dedicated test override (not the ambient PROJECT_DIR): main() runs rm -rf on $RELEASES_DIR/$REPORTS_DIR
PROJECT_DIR="${BTR_PROJECT_DIR:-/project}"
RELEASES_DIR="$PROJECT_DIR/releases"
REPORTS_DIR="$PROJECT_DIR/reports"
SCRIPTS_DIR="$PROJECT_DIR/scripts"
VERSION="${BUILD_VERSION:-1.0.0}"
SKIP_EMULATOR="${SKIP_EMULATOR_TESTS:-true}"
SKIP_E2E="${SKIP_E2E_TESTS:-false}"

# Track results
PHASE_RESULTS=()
PHASE_NOTES=()
TOTAL_PHASES=0
PASSED_PHASES=0
FAILED_PHASES=0
START_TIME=$(date +%s)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_phase() { echo -e "\n${CYAN}=========================================${NC}"; echo -e "${CYAN}  PHASE: $1${NC}"; echo -e "${CYAN}=========================================${NC}"; }
log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "${BLUE}  -> $1${NC}"; }

record_phase() {
    local name="$1"
    local status="$2"
    PHASE_RESULTS+=("$status:$name")
    TOTAL_PHASES=$((TOTAL_PHASES + 1))
    if [ "$status" = "SKIP" ]; then
        log_warn "Phase '$name' SKIPPED (not counted as passed)"
    elif [ "$status" = "PASS" ]; then
        PASSED_PHASES=$((PASSED_PHASES + 1))
        log_info "Phase '$name' completed successfully"
    else
        FAILED_PHASES=$((FAILED_PHASES + 1))
        log_error "Phase '$name' FAILED"
    fi
}

# ============================================================
# Submodule initialization (run before any phase)
# ============================================================
log_step "Initializing git submodules..."
if [ -f "$PROJECT_DIR/.gitmodules" ]; then
    cd "$PROJECT_DIR"
    git submodule init 2>/dev/null || log_warn "git submodule init failed (continuing; safe_submodule_init follows)"
    bash "$PROJECT_DIR/scripts/safe_submodule_init.sh" 2>/dev/null || log_warn "Some submodules may not be available"
    log_info "Git submodules initialized"
fi

# ============================================================
# Phase 0: Generate signing keys
# ============================================================
phase_0_signing_keys() {
    log_phase "Phase 0: Generate Signing Keys"

    if [ -x "$PROJECT_DIR/docker/signing/generate-keys.sh" ]; then
        "$PROJECT_DIR/docker/signing/generate-keys.sh" && record_phase "Signing Keys" "PASS" || record_phase "Signing Keys" "FAIL"
    else
        log_warn "Signing key script not found or not executable, skipping"
        record_phase "Signing Keys" "PASS"
    fi
}

# ============================================================
# Phase 1: Infrastructure health checks
# ============================================================
phase_1_health_checks() {
    log_phase "Phase 1: Infrastructure Health Checks"

    local pg_ok=false
    local redis_ok=false

    # Check PostgreSQL
    log_step "Checking PostgreSQL..."
    for i in $(seq 1 30); do
        if pg_isready -h "${POSTGRES_HOST:-postgres}" -p "${POSTGRES_PORT:-5432}" -U "${POSTGRES_USER:-catalogizer}" >/dev/null 2>&1; then
            log_info "PostgreSQL is ready"
            pg_ok=true
            break
        fi
        sleep 2
    done

    # Check Redis
    log_step "Checking Redis..."
    for i in $(seq 1 30); do
        if redis-cli -h "${REDIS_HOST:-redis}" -p "${REDIS_PORT:-6379}" ping 2>/dev/null | grep -q PONG; then
            log_info "Redis is ready"
            redis_ok=true
            break
        fi
        sleep 2
    done

    if $pg_ok && $redis_ok; then
        record_phase "Infrastructure Health" "PASS"
    else
        $pg_ok || log_error "PostgreSQL is not reachable"
        $redis_ok || log_error "Redis is not reachable"
        record_phase "Infrastructure Health" "FAIL"
    fi
}

# ============================================================
# Phase 2: Test & Build - API Client
# ============================================================
phase_2_api_client() {
    log_phase "Phase 2: Test & Build - API Client"

    # A failing step fails the phase (same pattern as phase 4); nothing is swallowed and nothing is reported PASS.
    local phase_rc=0
    cd "$PROJECT_DIR/catalogizer-api-client" || { log_error "catalogizer-api-client directory missing"; record_phase "API Client" "FAIL"; return 1; }

    log_step "Installing dependencies..."
    npm install || { log_error "API client npm install FAILED"; phase_rc=1; }

    log_step "Running tests..."
    npm test 2>&1 | tee "$REPORTS_DIR/api-client-test.log" || { log_error "API client tests FAILED"; phase_rc=1; }

    log_step "Building..."
    npm run build || { log_error "API client build FAILED"; phase_rc=1; }

    if [ "$phase_rc" -ne 0 ]; then
        record_phase "API Client" "FAIL"
        return 1
    fi
    log_info "API Client built successfully"
    record_phase "API Client" "PASS"
}

# ============================================================
# Phase 3: Test & Build - Backend API
# ============================================================
phase_3_backend_api() {
    log_phase "Phase 3: Test & Build - Backend API"

    local phase_rc=0
    cd "$PROJECT_DIR/catalog-api" || { log_error "catalog-api directory missing"; record_phase "Backend API" "FAIL"; return 1; }

    log_step "Downloading Go dependencies..."
    go mod download || { log_error "go mod download FAILED"; phase_rc=1; }
    go mod tidy || { log_error "go mod tidy FAILED"; phase_rc=1; }

    log_step "Running Go tests with coverage..."
    CGO_CPPFLAGS=-DHAVE_USLEEP=1 go test -v -race -coverprofile="$REPORTS_DIR/go-coverage.out" -covermode=atomic ./... 2>&1 | tee "$REPORTS_DIR/go-test.log" || { log_error "Go tests FAILED"; phase_rc=1; }

    # Generate coverage HTML report
    go tool cover -html="$REPORTS_DIR/go-coverage.out" -o "$REPORTS_DIR/go-coverage.html" 2>/dev/null || log_warn "Go coverage HTML report generation failed"

    # Generate coverage summary
    go tool cover -func="$REPORTS_DIR/go-coverage.out" > "$REPORTS_DIR/go-coverage-summary.txt" 2>/dev/null || log_warn "Go coverage summary generation failed"

    log_step "Building Linux AMD64 binary..."
    CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$VERSION" \
        -o "$RELEASES_DIR/linux/catalog-api/catalog-api-v$VERSION-linux-amd64" main.go || { log_error "Go build FAILED"; phase_rc=1; }

    log_step "Cross-compiling Windows AMD64..."
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$VERSION" \
        -o "$RELEASES_DIR/windows/catalog-api/catalog-api-v$VERSION-windows-amd64.exe" main.go || { log_error "Go build FAILED"; phase_rc=1; }

    log_step "Cross-compiling macOS AMD64..."
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$VERSION" \
        -o "$RELEASES_DIR/macos/amd64/catalog-api/catalog-api-v$VERSION-macos-amd64" main.go || { log_error "Go build FAILED"; phase_rc=1; }

    log_step "Cross-compiling macOS ARM64..."
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.Version=$VERSION" \
        -o "$RELEASES_DIR/macos/arm64/catalog-api/catalog-api-v$VERSION-macos-arm64" main.go || { log_error "Go build FAILED"; phase_rc=1; }

    if [ "$phase_rc" -ne 0 ]; then
        record_phase "Backend API" "FAIL"
        return 1
    fi
    log_info "Backend API built for all platforms"
    record_phase "Backend API" "PASS"
}

# ============================================================
# Phase 4: Test & Build - Web Frontend
# ============================================================
phase_4_web_frontend() {
    log_phase "Phase 4: Test & Build - Web Frontend"

    cd "$PROJECT_DIR/catalog-web"

    log_step "Installing dependencies..."
    npm install

    # E4: a failing step fails the phase; it is never swallowed and never reported PASS.
    local phase_rc=0

    log_step "Running lint and type-check..."
    # Lint findings are surfaced (warning + phase note), like run-all-tests-comprehensive.sh; a type error fails the phase.
    if ! npm run lint 2>&1 | tee "$REPORTS_DIR/web-lint.log"; then
        log_warn "Web lint reported issues (see web-lint.log)"
        PHASE_NOTES+=("Web Frontend: lint FAILED (see web-lint.log)")
    fi
    if ! npm run type-check 2>&1 | tee "$REPORTS_DIR/web-typecheck.log"; then
        log_error "Web type-check FAILED"
        phase_rc=1
    fi

    log_step "Running unit tests..."
    if npm run test -- --run 2>&1 | tee "$REPORTS_DIR/web-test.log"; then
        log_info "Web unit tests passed"
    else
        log_error "Web unit tests FAILED"
        phase_rc=1
    fi

    log_step "Building production bundle..."
    if ! npm run build; then
        log_error "Web production build FAILED"
        record_phase "Web Frontend" "FAIL"
        return 1
    fi

    # Copy web build to releases
    mkdir -p "$RELEASES_DIR/linux/catalog-web"
    cp -r dist/* "$RELEASES_DIR/linux/catalog-web/"

    # E2E tests (optional, requires running server and ADMIN_PASSWORD from the environment)
    if [ "$SKIP_E2E" = "true" ]; then
        log_warn "E2E tests skipped (SKIP_E2E_TESTS=true)"
    elif [ -z "${ADMIN_PASSWORD:-}" ]; then
        log_warn "E2E tests SKIPPED reason=credentials_absent (ADMIN_PASSWORD is not set; E2E not run, not counted as passed)"
        PHASE_NOTES+=("Web Frontend: E2E SKIPPED reason=credentials_absent")
    else
        log_step "Running Playwright E2E tests..."

        # Start API server in background for E2E
        cd "$PROJECT_DIR/catalog-api"
        ./catalog-api 2>/dev/null &
        local api_pid=$!

        # Start web dev server
        cd "$PROJECT_DIR/catalog-web"
        npm run dev &
        local web_pid=$!
        sleep 10

        if ! PLAYWRIGHT_BASE_URL="http://localhost:5173" xvfb-run npx playwright test --project=chromium 2>&1 | tee "$REPORTS_DIR/web-e2e.log"; then
            log_error "Playwright E2E tests FAILED"
            phase_rc=1
        fi

        kill $api_pid $web_pid 2>/dev/null || true
    fi

    if [ "$phase_rc" -ne 0 ]; then
        record_phase "Web Frontend" "FAIL"
        return 1
    fi
    log_info "Web frontend built successfully"
    record_phase "Web Frontend" "PASS"
}

# ============================================================
# Phase 5: Test & Build - Desktop Apps
# ============================================================
phase_5_desktop_apps() {
    log_phase "Phase 5: Test & Build - Desktop Apps"

    local phase_rc=0 app
    # name|npm-built binary name; both apps follow the same steps
    for app in catalogizer-desktop installer-wizard; do
        log_step "Building $app..."
        if ! cd "$PROJECT_DIR/$app"; then
            log_error "$app directory missing"
            phase_rc=1
            continue
        fi
        npm install || { log_error "$app npm install FAILED"; phase_rc=1; }

        if npm run test -- --run 2>&1 | tee "$REPORTS_DIR/${app}-test.log"; then
            log_info "$app tests passed"
        else
            log_error "$app tests FAILED"
            phase_rc=1
        fi

        log_step "Building Tauri $app..."
        if npm run tauri:build 2>&1 | tee "$REPORTS_DIR/${app}-build.log"; then
            mkdir -p "$RELEASES_DIR/linux/$app"
            find src-tauri/target/release/bundle -type f \( -name "*.AppImage" -o -name "*.deb" \) \
                -exec cp -t "$RELEASES_DIR/linux/$app/" {} + || { log_error "$app bundle artifact copy FAILED"; phase_rc=1; }
            if [ -f "src-tauri/target/release/$app" ]; then
                cp "src-tauri/target/release/$app" "$RELEASES_DIR/linux/$app/" || { log_error "$app binary copy FAILED"; phase_rc=1; }
            fi
            log_info "$app built"
        else
            log_error "$app Tauri build FAILED"
            phase_rc=1
        fi
    done

    if [ "$phase_rc" -ne 0 ]; then
        record_phase "Desktop Apps" "FAIL"
        return 1
    fi
    record_phase "Desktop Apps" "PASS"
}

# ============================================================
# Phase 6: Test & Build - Android Apps
# ============================================================
phase_6_android_apps() {
    log_phase "Phase 6: Test & Build - Android Apps"

    local android_projects=("catalogizer-android" "catalogizer-androidtv")
    local phase_rc=0 built=0 project

    for project in "${android_projects[@]}"; do
        log_step "Processing $project..."
        if ! cd "$PROJECT_DIR/$project"; then
            log_error "$project directory missing"
            phase_rc=1
            continue
        fi

        if [ ! -f "gradlew" ]; then
            log_warn "gradlew not found in $project, SKIPPED"
            PHASE_NOTES+=("Android Apps: $project SKIPPED reason=tool_absent (gradlew)")
            continue
        fi
        built=$((built + 1))
        chmod +x ./gradlew

        log_step "Running unit tests for $project..."
        if ./gradlew testDebugUnitTest 2>&1 | tee "$REPORTS_DIR/${project}-test.log"; then
            log_info "$project unit tests passed"
        else
            log_error "$project unit tests FAILED"
            phase_rc=1
        fi

        # Coverage report is derived output: failure is a warning, not a phase failure
        log_step "Generating coverage report for $project..."
        ./gradlew jacocoTestReport 2>&1 || log_warn "JaCoCo report generation failed for $project"

        mkdir -p "$REPORTS_DIR/$project"
        cp -r app/build/reports/tests/* "$REPORTS_DIR/$project/" 2>/dev/null || log_warn "No unit-test report to copy for $project"
        cp -r app/build/reports/jacoco/* "$REPORTS_DIR/$project/" 2>/dev/null || log_warn "No JaCoCo report to copy for $project"

        log_step "Building release APK for $project..."
        if ./gradlew assembleRelease 2>&1 | tee "$REPORTS_DIR/${project}-build.log"; then
            mkdir -p "$RELEASES_DIR/android/$project"
            find app/build/outputs/apk -name "*.apk" \
                -exec cp -t "$RELEASES_DIR/android/$project/" {} + || { log_error "APK copy FAILED for $project"; phase_rc=1; }
            log_info "$project APK built"
        else
            log_error "$project APK build FAILED"
            phase_rc=1
        fi
    done

    if [ "$phase_rc" -ne 0 ]; then
        record_phase "Android Apps" "FAIL"
        return 1
    fi
    if [ "$built" -eq 0 ]; then
        record_phase "Android Apps" "SKIP"
        return 0
    fi
    record_phase "Android Apps" "PASS"
}

# ============================================================
# Phase 7: Android Emulator Smoke Tests
# ============================================================
phase_7_emulator_tests() {
    log_phase "Phase 7: Android Emulator Smoke Tests"

    if [ "$SKIP_EMULATOR" = "true" ]; then
        log_warn "Emulator tests skipped (SKIP_EMULATOR_TESTS=true)"
        PHASE_NOTES+=("Emulator Tests: SKIPPED reason=feature_disabled_by_config (SKIP_EMULATOR_TESTS=true)")
        record_phase "Emulator Tests" "SKIP"
        return 0
    fi

    local phase_rc=0 apk launched=0
    log_step "Connecting to Android emulator..."
    if ! adb connect android-emulator:5555 2>/dev/null; then
        log_error "Could not connect to Android emulator"
        record_phase "Emulator Tests" "FAIL"
        return 1
    fi
    sleep 5

    log_step "Waiting for emulator to boot..."
    adb -s android-emulator:5555 wait-for-device || { log_error "wait-for-device FAILED"; phase_rc=1; }
    sleep 10

    for apk in "$RELEASES_DIR"/android/catalogizer-android/*.apk; do
        [ -f "$apk" ] || continue
        launched=1
        log_step "Installing $(basename "$apk")..."
        adb -s android-emulator:5555 install -r "$apk" 2>&1 || { log_error "APK install FAILED"; phase_rc=1; }

        log_step "Launching app..."
        adb -s android-emulator:5555 shell am start -n com.catalogizer.android/.MainActivity 2>&1 || log_warn "am start failed (the activity check below decides)"
        sleep 5

        if adb -s android-emulator:5555 shell dumpsys activity activities 2>/dev/null | grep -q "catalogizer"; then
            log_info "App launched successfully"
        else
            log_error "App did not launch"
            phase_rc=1
        fi
    done

    if [ "$phase_rc" -ne 0 ]; then
        record_phase "Emulator Tests" "FAIL"
        return 1
    fi
    if [ "$launched" -eq 0 ]; then
        PHASE_NOTES+=("Emulator Tests: SKIPPED reason=no_apk (nothing to install)")
        record_phase "Emulator Tests" "SKIP"
        return 0
    fi
    record_phase "Emulator Tests" "PASS"
}

# ============================================================
# Phase 8: Coverage Validation
# ============================================================
phase_8_coverage_validation() {
    log_phase "Phase 8: Coverage Validation & False Positive Detection"

    if [ -x "$SCRIPTS_DIR/validate-coverage.sh" ]; then
        if "$SCRIPTS_DIR/validate-coverage.sh" 2>&1 | tee "$REPORTS_DIR/validation.log"; then
            record_phase "Coverage Validation" "PASS"
        else
            log_error "Coverage validation reported issues"
            record_phase "Coverage Validation" "FAIL"
            return 1
        fi
    else
        log_warn "validate-coverage.sh not found, SKIPPED"
        PHASE_NOTES+=("Coverage Validation: SKIPPED reason=tool_absent (validate-coverage.sh)")
        record_phase "Coverage Validation" "SKIP"
    fi
}

# ============================================================
# Phase 9: Collect Artifacts
# ============================================================
phase_9_collect_artifacts() {
    log_phase "Phase 9: Collect Artifacts to releases/"

    cd "$RELEASES_DIR"

    # Build API client library release
    log_step "Collecting API client library..."
    mkdir -p "$RELEASES_DIR/lib/catalogizer-api-client"
    if [ -d "$PROJECT_DIR/catalogizer-api-client/dist" ]; then
        cp -r "$PROJECT_DIR/catalogizer-api-client/dist"/* "$RELEASES_DIR/lib/catalogizer-api-client/" || log_warn "API client dist copy failed"
        cp "$PROJECT_DIR/catalogizer-api-client/package.json" "$RELEASES_DIR/lib/catalogizer-api-client/" || log_warn "API client package.json copy failed"
    fi

    # Generate SHA256 checksums
    log_step "Generating SHA256 checksums..."
    # Write to a temp file and publish only on success: a failed run must not leave a partial SHA256SUMS.txt that looks like an integrity file.
    local sums_tmp="$RELEASES_DIR/.SHA256SUMS.tmp"
    if ! find . -type f \( -name "*.exe" -o -name "*.AppImage" -o -name "*.deb" \
        -o -name "*.apk" -o -name "catalog-api*" -o -name "*.msi" \) \
        -exec sha256sum {} + > "$sums_tmp"; then
        rm -f -- "${sums_tmp:?}"
        log_error "SHA256 checksum generation FAILED"
        record_phase "Artifact Collection" "FAIL"
        return 1
    fi
    if [ ! -s "$sums_tmp" ]; then
        rm -f -- "${sums_tmp:?}"
        log_error "No release artifacts found to checksum (zero artifacts is not a pass)"
        record_phase "Artifact Collection" "FAIL"
        return 1
    fi
    mv -- "$sums_tmp" "$RELEASES_DIR/SHA256SUMS.txt"

    # Generate MANIFEST.json
    log_step "Generating MANIFEST.json..."
    local build_date
    build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    cat > "$RELEASES_DIR/MANIFEST.json" << MANIFEST_EOF
{
  "version": "$VERSION",
  "build_date": "$build_date",
  "build_type": "containerized",
  "builder_image": "catalogizer-builder",
  "components": {
    "catalog-api": {
      "platforms": ["linux-amd64", "windows-amd64", "macos-amd64", "macos-arm64"],
      "artifacts": $(find linux/catalog-api windows/catalog-api macos/*/catalog-api -type f 2>/dev/null | jq -R -s 'split("\n") | map(select(. != ""))' 2>/dev/null || echo '[]')
    },
    "catalog-web": {
      "type": "static-files",
      "artifacts": $(find linux/catalog-web -type f 2>/dev/null | wc -l || echo 0)
    },
    "catalogizer-api-client": {
      "type": "npm-library",
      "artifacts": $(find lib/catalogizer-api-client -type f 2>/dev/null | wc -l || echo 0)
    },
    "catalogizer-desktop": {
      "platforms": ["linux"],
      "artifacts": $(find linux/catalogizer-desktop -type f 2>/dev/null | jq -R -s 'split("\n") | map(select(. != ""))' 2>/dev/null || echo '[]')
    },
    "installer-wizard": {
      "platforms": ["linux"],
      "artifacts": $(find linux/installer-wizard -type f 2>/dev/null | jq -R -s 'split("\n") | map(select(. != ""))' 2>/dev/null || echo '[]')
    },
    "catalogizer-android": {
      "artifacts": $(find android/catalogizer-android -name "*.apk" -type f 2>/dev/null | jq -R -s 'split("\n") | map(select(. != ""))' 2>/dev/null || echo '[]')
    },
    "catalogizer-androidtv": {
      "artifacts": $(find android/catalogizer-androidtv -name "*.apk" -type f 2>/dev/null | jq -R -s 'split("\n") | map(select(. != ""))' 2>/dev/null || echo '[]')
    }
  }
}
MANIFEST_EOF

    log_info "Artifacts collected to $RELEASES_DIR"
    record_phase "Artifact Collection" "PASS"
}

# ============================================================
# Phase 10: Generate Reports
# ============================================================
phase_10_generate_reports() {
    log_phase "Phase 10: Generate Reports"

    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    local minutes=$((duration / 60))
    local seconds=$((duration % 60))

    # Generate build summary report
    cat > "$REPORTS_DIR/build-report.json" << REPORT_EOF
{
  "version": "$VERSION",
  "build_date": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "duration_seconds": $duration,
  "total_phases": $TOTAL_PHASES,
  "passed_phases": $PASSED_PHASES,
  "failed_phases": $FAILED_PHASES,
  "phases": [
$(for result in "${PHASE_RESULTS[@]}"; do
    local status="${result%%:*}"
    local name="${result#*:}"
    echo "    {\"name\": \"$name\", \"status\": \"$status\"},"
done | sed '$ s/,$//')
  ]
}
REPORT_EOF

    # Generate HTML report
    cat > "$REPORTS_DIR/build-report.html" << 'HTML_HEADER'
<!DOCTYPE html>
<html>
<head>
    <title>Catalogizer Build Report</title>
    <style>
        body { font-family: 'Segoe UI', sans-serif; margin: 20px; background: #f5f7fa; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; border-radius: 15px; text-align: center; margin-bottom: 30px; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 15px; margin-bottom: 30px; }
        .metric { background: white; padding: 20px; border-radius: 10px; text-align: center; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .metric h3 { margin: 0; color: #495057; font-size: 13px; text-transform: uppercase; }
        .metric p { margin: 8px 0 0; font-size: 28px; font-weight: bold; }
        .pass { color: #28a745; }
        .fail { color: #dc3545; }
        .section { background: white; margin: 15px 0; padding: 20px; border-radius: 10px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .phase { display: flex; justify-content: space-between; padding: 10px 0; border-bottom: 1px solid #e9ecef; }
        .phase:last-child { border-bottom: none; }
        .status-pass { color: #28a745; font-weight: bold; }
        .status-fail { color: #dc3545; font-weight: bold; }
    </style>
</head>
<body>
HTML_HEADER

    cat >> "$REPORTS_DIR/build-report.html" << HTML_BODY
    <div class="header">
        <h1>Catalogizer Build Report</h1>
        <p>Version $VERSION - $(date)</p>
        <p>Duration: ${minutes}m ${seconds}s</p>
    </div>
    <div class="summary">
        <div class="metric"><h3>Total Phases</h3><p>$TOTAL_PHASES</p></div>
        <div class="metric"><h3>Passed</h3><p class="pass">$PASSED_PHASES</p></div>
        <div class="metric"><h3>Failed</h3><p class="fail">$FAILED_PHASES</p></div>
    </div>
    <div class="section">
        <h2>Phase Results</h2>
HTML_BODY

    for result in "${PHASE_RESULTS[@]}"; do
        local status="${result%%:*}"
        local name="${result#*:}"
        local css_class="status-pass"
        local symbol="PASS"
        if [ "$status" = "FAIL" ]; then
            css_class="status-fail"
            symbol="FAIL"
        fi
        echo "        <div class=\"phase\"><span>$name</span><span class=\"$css_class\">$symbol</span></div>" >> "$REPORTS_DIR/build-report.html"
    done

    cat >> "$REPORTS_DIR/build-report.html" << 'HTML_FOOTER'
    </div>
</body>
</html>
HTML_FOOTER

    log_info "Reports generated in $REPORTS_DIR"
    record_phase "Report Generation" "PASS"
}

# ============================================================
# Main execution
# ============================================================
main() {
    echo -e "${CYAN}"
    echo "============================================="
    echo "  Catalogizer Build, Test & Release Pipeline"
    echo "  Version: $VERSION"
    echo "  Started: $(date)"
    echo "============================================="
    echo -e "${NC}"

    # Prepare output directories
    : "${PROJECT_DIR:?}" "${RELEASES_DIR:?}" "${REPORTS_DIR:?}"
    rm -rf -- "${RELEASES_DIR:?}" "${REPORTS_DIR:?}"
    mkdir -p "$RELEASES_DIR"/{linux,windows,android}/{catalog-api,catalog-web,catalogizer-desktop,installer-wizard}
    mkdir -p "$RELEASES_DIR/macos"/{amd64,arm64}/catalog-api
    mkdir -p "$RELEASES_DIR/android"/{catalogizer-android,catalogizer-androidtv}
    mkdir -p "$RELEASES_DIR/lib"
    mkdir -p "$REPORTS_DIR"

    # Run all phases
    phase_0_signing_keys
    phase_1_health_checks
    phase_2_api_client      || log_error "Phase 2 failed but continuing"
    phase_3_backend_api     || log_error "Phase 3 failed but continuing"
    phase_4_web_frontend    || log_error "Phase 4 failed but continuing"
    phase_5_desktop_apps    || log_error "Phase 5 failed but continuing"
    phase_6_android_apps    || log_error "Phase 6 failed but continuing"
    phase_7_emulator_tests  || log_error "Phase 7 failed but continuing"
    phase_8_coverage_validation || log_error "Phase 8 failed but continuing"
    phase_9_collect_artifacts || log_error "Phase 9 failed but continuing"
    phase_10_generate_reports

    # Final summary
    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    local minutes=$((duration / 60))
    local seconds=$((duration % 60))

    echo ""
    echo -e "${CYAN}=============================================${NC}"
    echo -e "${CYAN}  Build Pipeline Summary${NC}"
    echo -e "${CYAN}=============================================${NC}"
    echo -e "  Version:  $VERSION"
    echo -e "  Duration: ${minutes}m ${seconds}s"
    echo -e "  Phases:   $TOTAL_PHASES total"
    echo -e "  ${GREEN}Passed:   $PASSED_PHASES${NC}"
    echo -e "  ${RED}Failed:   $FAILED_PHASES${NC}"
    echo ""

    for result in "${PHASE_RESULTS[@]}"; do
        local status="${result%%:*}"
        local name="${result#*:}"
        if [ "$status" = "PASS" ]; then
            echo -e "  ${GREEN}[PASS]${NC} $name"
        elif [ "$status" = "SKIP" ]; then
            echo -e "  ${YELLOW}[SKIP]${NC} $name"
        else
            echo -e "  ${RED}[FAIL]${NC} $name"
        fi
    done

    for note in "${PHASE_NOTES[@]:-}"; do [ -z "$note" ] || echo -e "  ${YELLOW}[NOTE]${NC} $note"; done

    echo ""
    echo -e "  Releases: $RELEASES_DIR"
    echo -e "  Reports:  $REPORTS_DIR"
    echo -e "${CYAN}=============================================${NC}"

    if [ "$FAILED_PHASES" -gt 0 ]; then
        echo -e "${RED}Build pipeline completed with $FAILED_PHASES failure(s)${NC}"
        exit 1
    else
        echo -e "${GREEN}Build pipeline completed successfully!${NC}"
        exit 0
    fi
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    main "$@"
fi
