#!/usr/bin/env bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/lib/cgo_flags.sh"   # 2026-11-02 decision: cgo define for go-sqlcipher (scripts/lib/cgo_flags.sh)
# run-all-platform-tests.sh — Constitution Article V compliance runner
#
# Runs every test category across every platform sequentially.
# Results are logged to docs/reports/qa-sessions/qa-session-$(date)/
#
# Usage:
#   ./scripts/run-all-platform-tests.sh              # all platforms
#   ./scripts/run-all-platform-tests.sh --api-only    # backend only
#   ./scripts/run-all-platform-tests.sh --emulators   # include Android emulators
#
# Resource limits: respects the 30-40% host budget per CLAUDE.md.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DATE=$(date +%Y-%m-%d)
REPORT_DIR="$ROOT/docs/reports/qa-sessions/qa-session-$DATE"
mkdir -p "$REPORT_DIR/logs"

PASS=0
FAIL=0
SKIP=0
RESULTS=""

log() { echo "[$(date +%H:%M:%S)] $*"; }
record() {
    local platform=$1 category=$2 status=$3 detail=$4
    RESULTS="${RESULTS}| $platform | $category | $status | $detail |\n"
    if [ "$status" = "PASS" ]; then PASS=$((PASS + 1));
    elif [ "$status" = "FAIL" ]; then FAIL=$((FAIL + 1));
    else SKIP=$((SKIP + 1)); fi
}

run_tests() {
    local platform=$1 category=$2 cmd=$3 logfile="$REPORT_DIR/logs/${1}-${2}.log"
    log "[$platform] $category..."
    if eval "$cmd" > "$logfile" 2>&1; then
        record "$platform" "$category" "PASS" "$(tail -1 "$logfile")"
    else
        record "$platform" "$category" "FAIL" "$(tail -1 "$logfile")"
    fi
}

# ─── catalog-api ───────────────────────────────────────────────────
log "=== catalog-api ==="
run_tests "catalog-api" "unit+integration" \
    "cd '$ROOT/catalog-api' && GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -count=1 -timeout 15m"
run_tests "catalog-api" "go-vet" \
    "cd '$ROOT/catalog-api' && go vet ./..."
run_tests "catalog-api" "govulncheck" \
    "cd '$ROOT/catalog-api' && govulncheck ./..."

# ─── catalog-web ───────────────────────────────────────────────────
log "=== catalog-web ==="
run_tests "catalog-web" "tsc" \
    "cd '$ROOT/catalog-web' && npx tsc --noEmit"
run_tests "catalog-web" "vitest" \
    "cd '$ROOT/catalog-web' && npx vitest run"
run_tests "catalog-web" "npm-audit" \
    "cd '$ROOT/catalog-web' && npm audit --omit=dev"

# ─── catalogizer-desktop ──────────────────────────────────────────
log "=== catalogizer-desktop ==="
run_tests "desktop" "tsc" \
    "cd '$ROOT/catalogizer-desktop' && npx tsc --noEmit"
run_tests "desktop" "vitest" \
    "cd '$ROOT/catalogizer-desktop' && npx vitest run"

# ─── installer-wizard ─────────────────────────────────────────────
log "=== installer-wizard ==="
run_tests "wizard" "tsc" \
    "cd '$ROOT/installer-wizard' && npx tsc --noEmit"
run_tests "wizard" "vitest" \
    "cd '$ROOT/installer-wizard' && npx vitest run"

# ─── catalogizer-android ──────────────────────────────────────────
log "=== catalogizer-android ==="
JAVA_HOME=${JAVA_HOME:-/usr/lib/jvm/java-21-openjdk-21.0.10.0.7-alt1.x86_64}
export JAVA_HOME
run_tests "android" "unit" \
    "cd '$ROOT/catalogizer-android' && ./gradlew :app:testDebugUnitTest --console=plain"

# ─── catalogizer-androidtv ────────────────────────────────────────
log "=== catalogizer-androidtv ==="
run_tests "androidtv" "unit" \
    "cd '$ROOT/catalogizer-androidtv' && ./gradlew :app:testDebugUnitTest --console=plain"

# ─── catalogizer-api-client ───────────────────────────────────────
log "=== catalogizer-api-client ==="
run_tests "api-client" "vitest" \
    "cd '$ROOT/Catalogizer-API-Client-TS' && npm test"

# ─── Go submodules ────────────────────────────────────────────────
log "=== Go submodules ==="
GO_SUBS="Database Concurrency Config Filesystem Auth Cache Entities EventBus Discovery Lazy Media Memory Middleware Observability RateLimiter Recovery Security Storage Streaming Watcher Challenges Containers"
for sub in $GO_SUBS; do
    run_tests "go-$sub" "unit" \
        "cd '$ROOT/$sub' && go test ./... -count=1 -timeout 2m"
done

# ─── TS submodules ────────────────────────────────────────────────
log "=== TS submodules ==="
TS_SUBS="WebSocket-Client-TS UI-Components-React Auth-Context-React Media-Browser-React Media-Player-React Collection-Manager-React Dashboard-Analytics-React Media-Types-TS"
for sub in $TS_SUBS; do
    run_tests "ts-$sub" "vitest" \
        "cd '$ROOT/$sub' && npx vitest run 2>/dev/null || npm test"
done

# ─── k6 load tests (requires running API) ─────────────────────────
if curl -s -m 3 http://localhost:8080/health | grep -q healthy 2>/dev/null; then
    log "=== k6 tests ==="
    if command -v k6 &>/dev/null; then
        run_tests "k6" "ddos-ratelimit" \
            "k6 run '$ROOT/tests/k6/ddos_ratelimit_test.js'"
    elif command -v podman &>/dev/null; then
        run_tests "k6" "ddos-ratelimit" \
            "podman run --rm --network host -v '$ROOT/tests/k6:/scripts:Z' docker.io/grafana/k6:latest run /scripts/ddos_ratelimit_test.js"
    else
        record "k6" "ddos-ratelimit" "SKIP" "No k6 or podman"
    fi
else
    record "k6" "ddos-ratelimit" "SKIP" "API not running"
fi

# ─── Connected ADB devices ────────────────────────────────────────
if command -v adb &>/dev/null; then
    log "=== ADB devices ==="
    APK_PHONE="$ROOT/catalogizer-android/app/build/outputs/apk/debug/app-debug.apk"
    APK_TV="$ROOT/catalogizer-androidtv/app/build/outputs/apk/debug/app-debug.apk"
    for dev in $(adb devices | grep -E "device$" | awk '{print $1}'); do
        MODEL=$(adb -s "$dev" shell getprop ro.product.model 2>/dev/null | tr -d '\r')
        SDK=$(adb -s "$dev" shell getprop ro.build.version.sdk 2>/dev/null | tr -d '\r')
        # Check .devignore
        if grep -qi "$MODEL" "$ROOT/.devignore" 2>/dev/null; then
            record "adb-$MODEL" "install" "SKIP" "In .devignore"
            continue
        fi
        log "  Device: $MODEL (API $SDK) @ $dev"
        # Determine which APK to install
        IS_TV=$(adb -s "$dev" shell pm list features 2>/dev/null | grep -c "android.software.leanback")
        if [ "$IS_TV" -gt 0 ] && [ -f "$APK_TV" ]; then
            APK="$APK_TV"; PKG="com.catalogizer.androidtv"
        elif [ -f "$APK_PHONE" ]; then
            APK="$APK_PHONE"; PKG="com.catalogizer.android"
        else
            record "adb-$MODEL" "install" "SKIP" "No APK built"
            continue
        fi
        # Install
        if adb -s "$dev" install -r "$APK" 2>&1 | grep -q "Success"; then
            # Launch and check
            adb -s "$dev" logcat -c 2>/dev/null
            adb -s "$dev" shell am start -n "$PKG/.ui.MainActivity" 2>/dev/null
            sleep 5
            PID=$(adb -s "$dev" shell pidof "$PKG" 2>/dev/null | tr -d '\r')
            if [ -n "$PID" ]; then
                record "adb-$MODEL" "launch-api$SDK" "PASS" "PID=$PID"
                # Screenshot
                adb -s "$dev" shell screencap -p "/sdcard/test_$DATE.png" 2>/dev/null
                adb -s "$dev" pull "/sdcard/test_$DATE.png" "$REPORT_DIR/logs/$MODEL-api$SDK.png" 2>/dev/null
            else
                CRASH=$(adb -s "$dev" logcat -d -s AndroidRuntime:E 2>&1 | grep "$PKG" -A 3 | head -5)
                record "adb-$MODEL" "launch-api$SDK" "FAIL" "$CRASH"
            fi
        else
            record "adb-$MODEL" "install" "FAIL" "APK install failed"
        fi
    done
fi

# ─── Report ───────────────────────────────────────────────────────
REPORT="$REPORT_DIR/TEST-RESULTS.md"
cat > "$REPORT" <<HEREDOC
# Test Results — $DATE

| Platform | Category | Status | Detail |
|----------|----------|--------|--------|
$(echo -e "$RESULTS")

## Summary

- **PASS:** $PASS
- **FAIL:** $FAIL
- **SKIP:** $SKIP
- **Total:** $((PASS + FAIL + SKIP))

Generated by \`scripts/run-all-platform-tests.sh\`
HEREDOC

log "=== DONE ==="
log "PASS=$PASS FAIL=$FAIL SKIP=$SKIP"
log "Report: $REPORT"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
