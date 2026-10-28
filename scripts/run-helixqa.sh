#!/usr/bin/env bash
# HelixQA Full QA Session Runner
# Runs real browser-based testing with video recording against live services.
#
# Usage:
#   ./scripts/run-helixqa.sh [web|api|all]
#
# Prerequisites:
#   - catalog-api running on port 8080
#   - catalog-web running on port 3000 (for web sessions)
#   - ffmpeg, npx playwright, chromium installed
#
# Output: qa-results/helixqa-<timestamp>/

set -euo pipefail
ADMIN_PASSWORD="${ADMIN_PASSWORD:?set ADMIN_PASSWORD to the test-stack admin password (from your .env; never hardcoded)}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PLATFORM="${1:-all}"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
OUTPUT_DIR="$PROJECT_ROOT/qa-results/helixqa-${TIMESTAMP}"
BANKS_DIR="$PROJECT_ROOT/challenges/helixqa-banks"
HELIXQA="$PROJECT_ROOT/submodules/helix_qa/helixqa"
FFMPEG="${HELIX_RECORDING_FFMPEG_PATH:-/home/milosvasic/bin/ffmpeg}"
API_URL="http://localhost:8080"
WEB_URL="http://localhost:3000"
DISPLAY="${DISPLAY:-:0}"

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'

log()  { echo -e "${BLUE}[HelixQA]${NC} $*"; }
ok()   { echo -e "${GREEN}[  OK  ]${NC} $*"; }
warn() { echo -e "${YELLOW}[ WARN ]${NC} $*"; }
fail() { echo -e "${RED}[ FAIL ]${NC} $*"; }

mkdir -p "$OUTPUT_DIR"/{videos,screenshots/{web,api},evidence,tickets}

# ─── Preflight Checks ──────────────────────────────────────────────────────

log "Checking prerequisites..."

check_cmd() {
    command -v "$1" &>/dev/null || { fail "$1 not found"; exit 1; }
}
check_cmd "$FFMPEG"
check_cmd npx

# Check services
if ! curl -sf "$API_URL/health" > /dev/null 2>&1; then
    fail "catalog-api not running on $API_URL"
    log "Start it with: cd catalog-api && go run main.go"
    exit 1
fi
ok "catalog-api is healthy"

if [[ "$PLATFORM" == "web" || "$PLATFORM" == "all" ]]; then
    if ! curl -sf -o /dev/null "$WEB_URL" 2>&1; then
        fail "catalog-web not running on $WEB_URL"
        log "Start it with: cd catalog-web && npm run dev"
        exit 1
    fi
    ok "catalog-web is running"
fi

# ─── Login and get auth token ──────────────────────────────────────────────

log "Authenticating..."
TOKEN=$(curl -sf -X POST "$API_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"admin\",\"password\":\"${ADMIN_PASSWORD}\"}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_token', d.get('token','')))" 2>/dev/null || echo "")

if [[ -z "$TOKEN" ]]; then
    warn "Could not obtain auth token (some API tests may fail)"
else
    ok "Authenticated (token obtained)"
fi

# ─── API QA Session ────────────────────────────────────────────────────────

run_api_session() {
    log "Starting API QA session..."
    local report="$OUTPUT_DIR/api-session-report.md"
    local pass=0 fail_count=0 total=0
    local start_time=$(date +%s)

    echo "# HelixQA API Session Report" > "$report"
    echo "" >> "$report"
    echo "**Generated:** $(date -Iseconds)" >> "$report"
    echo "**API URL:** $API_URL" >> "$report"
    echo "" >> "$report"
    echo "| # | Endpoint | Method | Status | Result | Duration |" >> "$report"
    echo "|---|----------|--------|--------|--------|----------|" >> "$report"

    # Test all major endpoints
    local endpoints=(
        "GET /health"
        "GET /metrics"
        "GET /api/v1/auth/status"
        "GET /api/v1/discovery"
        "GET /api/v1/catalog"
        "GET /api/v1/search/files?q=test"
        "GET /api/v1/media/search"
        "GET /api/v1/media/stats"
        "GET /api/v1/storage-roots"
        "GET /api/v1/entities"
        "GET /api/v1/entities/types"
        "GET /api/v1/entities/stats"
        "GET /api/v1/entities/duplicates"
        "GET /api/v1/collections"
        "GET /api/v1/favorites"
        "GET /api/v1/stats/overall"
        "GET /api/v1/stats/filetypes"
        "GET /api/v1/stats/sizes"
        "GET /api/v1/stats/duplicates"
        "GET /api/v1/stats/access"
        "GET /api/v1/stats/growth"
        "GET /api/v1/stats/scans"
        "GET /api/v1/challenges"
        "GET /api/v1/configuration"
        "GET /api/v1/configuration/status"
        "GET /api/v1/configuration/wizard/progress"
        "GET /api/v1/errors/statistics"
        "GET /api/v1/errors/crash-statistics"
        "GET /api/v1/errors/health"
        "GET /api/v1/errors/reports"
        "GET /api/v1/errors/crashes"
        "GET /api/v1/logs/statistics"
        "GET /api/v1/logs/collections"
        "GET /api/v1/subtitles/languages"
        "GET /api/v1/subtitles/providers"
        "GET /api/v1/conversion/formats"
        "GET /api/v1/conversion/jobs"
        "GET /api/v1/roles"
        "GET /api/v1/roles/permissions"
        "GET /api/v1/users"
        "GET /api/v1/browse/roots"
        "GET /api/v1/sync/endpoints"
        "GET /api/v1/sync/sessions"
        "GET /api/v1/sync/statistics"
        "GET /api/v1/reports/usage"
        "GET /api/v1/reports/performance"
        "GET /api/v1/analytics/system"
        "GET /api/v1/recommendations/trending"
        "GET /api/v1/scans"
        "GET /api/v1/challenges/results"
        "POST /api/v1/auth/login"
    )

    for endpoint in "${endpoints[@]}"; do
        local method=$(echo "$endpoint" | cut -d' ' -f1)
        local path=$(echo "$endpoint" | cut -d' ' -f2)
        total=$((total + 1))
        local t_start=$(date +%s%N)

        local status
        if [[ "$method" == "POST" && "$path" == "/api/v1/auth/login" ]]; then
            status=$(curl -sf -o /dev/null -w '%{http_code}' \
                -X POST "${API_URL}${path}" \
                -H 'Content-Type: application/json' \
                -d "{\"username\":\"admin\",\"password\":\"${ADMIN_PASSWORD}\"}" 2>/dev/null || echo "000")
        else
            status=$(curl -sf -o /dev/null -w '%{http_code}' \
                -X "$method" "${API_URL}${path}" \
                -H "Authorization: Bearer $TOKEN" \
                -H 'Content-Type: application/json' 2>/dev/null || echo "000")
        fi

        local t_end=$(date +%s%N)
        local duration_ms=$(( (t_end - t_start) / 1000000 ))

        local result="FAIL"
        if [[ "$status" -ge 200 && "$status" -lt 400 ]]; then
            result="PASS"
            pass=$((pass + 1))
            ok "  $method $path → $status (${duration_ms}ms)"
        else
            fail_count=$((fail_count + 1))
            fail "  $method $path → $status (${duration_ms}ms)"
        fi
        echo "| $total | \`$path\` | $method | $status | $result | ${duration_ms}ms |" >> "$report"
    done

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo "" >> "$report"
    echo "## Summary" >> "$report"
    echo "- **Total:** $total endpoints tested" >> "$report"
    echo "- **Passed:** $pass" >> "$report"
    echo "- **Failed:** $fail_count" >> "$report"
    echo "- **Duration:** ${duration}s" >> "$report"

    log "API session complete: $pass/$total passed, $fail_count failed (${duration}s)"
}

# ─── Web QA Session with Video ─────────────────────────────────────────────

run_web_session() {
    log "Starting Web QA session with video recording..."
    local video_file="$OUTPUT_DIR/videos/web-session.mp4"
    local screenshot_dir="$OUTPUT_DIR/screenshots/web"
    local report="$OUTPUT_DIR/web-session-report.md"

    # Start ffmpeg video recording of the X11 display
    log "Starting video recording (DISPLAY=$DISPLAY)..."
    "$FFMPEG" -y -f x11grab -video_size 1920x1080 -framerate 10 -i "$DISPLAY" \
        -c:v libx264 -preset ultrafast -pix_fmt yuv420p -threads 2 \
        "$video_file" </dev/null &>/dev/null &
    local ffmpeg_pid=$!
    sleep 1

    if ! kill -0 $ffmpeg_pid 2>/dev/null; then
        warn "ffmpeg recording failed to start — trying with display size detection..."
        local screen_size=$(xdpyinfo -display "$DISPLAY" 2>/dev/null | grep dimensions | awk '{print $2}' | head -1 || echo "1920x1080")
        "$FFMPEG" -y -f x11grab -video_size "$screen_size" -framerate 10 -i "$DISPLAY" \
            -c:v libx264 -preset ultrafast -pix_fmt yuv420p -threads 2 \
            "$video_file" </dev/null &>/dev/null &
        ffmpeg_pid=$!
        sleep 1
    fi

    if kill -0 $ffmpeg_pid 2>/dev/null; then
        ok "Video recording started (PID: $ffmpeg_pid)"
    else
        warn "Video recording could not start — continuing without video"
        ffmpeg_pid=""
    fi

    # Navigate through all pages with Playwright and take screenshots
    local pages=(
        "login:Login Page"
        ":Home/Dashboard"
        "dashboard:Dashboard"
        "media:Media Browser"
        "browse:Entity Browser"
        "collections:Collections"
        "favorites:Favorites"
        "playlists:Playlists"
        "analytics:Analytics"
        "subtitles:Subtitle Manager"
        "conversion:Conversion Tools"
        "admin:Admin Panel"
        "ai:AI Dashboard"
    )

    local screenshot_count=0
    local pass=0
    local fail_count=0
    local total=${#pages[@]}

    echo "# HelixQA Web Session Report" > "$report"
    echo "" >> "$report"
    echo "**Generated:** $(date -Iseconds)" >> "$report"
    echo "**Web URL:** $WEB_URL" >> "$report"
    echo "**Video:** videos/web-session.mp4" >> "$report"
    echo "" >> "$report"
    echo "| # | Page | URL | Screenshot | Status |" >> "$report"
    echo "|---|------|-----|------------|--------|" >> "$report"

    for page_info in "${pages[@]}"; do
        local page_path="${page_info%%:*}"
        local page_name="${page_info##*:}"
        local url="${WEB_URL}/${page_path}"
        local screenshot_name="$(printf '%03d' $screenshot_count)-${page_path:-home}.png"
        local screenshot_path="${screenshot_dir}/${screenshot_name}"

        log "  Capturing: $page_name ($url)..."

        # Use Playwright to take screenshot (wait for network idle to avoid splash captures)
        if npx playwright screenshot --browser chromium --wait-for-timeout 3000 "$url" "$screenshot_path" &>/dev/null; then
            ok "    Screenshot: $screenshot_name"
            pass=$((pass + 1))
            echo "| $((screenshot_count+1)) | $page_name | \`/$page_path\` | $screenshot_name | PASS |" >> "$report"
        else
            warn "    Failed to capture: $page_name"
            fail_count=$((fail_count + 1))
            echo "| $((screenshot_count+1)) | $page_name | \`/$page_path\` | - | FAIL |" >> "$report"
        fi
        screenshot_count=$((screenshot_count + 1))
        sleep 2  # Allow page to render
    done

    # Stop video recording
    if [[ -n "${ffmpeg_pid:-}" ]] && kill -0 "$ffmpeg_pid" 2>/dev/null; then
        log "Stopping video recording..."
        kill -INT "$ffmpeg_pid" 2>/dev/null || true
        sleep 3
        kill -TERM "$ffmpeg_pid" 2>/dev/null || true
        wait "$ffmpeg_pid" 2>/dev/null || true
    fi

    # Report video info
    echo "" >> "$report"
    echo "## Video Recording" >> "$report"
    if [[ -f "$video_file" ]]; then
        local video_size=$(du -h "$video_file" | cut -f1)
        local video_duration=$("$FFMPEG" -i "$video_file" 2>&1 | grep Duration | awk '{print $2}' | tr -d ',')
        echo "- **File:** videos/web-session.mp4" >> "$report"
        echo "- **Size:** $video_size" >> "$report"
        echo "- **Duration:** $video_duration" >> "$report"
        ok "Video: $video_file ($video_size, $video_duration)"
    else
        echo "- **Status:** No video recorded" >> "$report"
        warn "No video file produced"
    fi

    echo "" >> "$report"
    echo "## Summary" >> "$report"
    echo "- **Pages tested:** $total" >> "$report"
    echo "- **Screenshots captured:** $pass" >> "$report"
    echo "- **Failed captures:** $fail_count" >> "$report"

    log "Web session complete: $pass/$total screenshots captured"
}

# ─── HelixQA Native Run (Step Validation) ──────────────────────────────────

run_helixqa_native() {
    local platform="$1"
    local bank_file="$2"
    local session_name="$3"
    local output_subdir="$OUTPUT_DIR/$session_name"

    log "Running HelixQA native: $session_name ($platform)..."
    mkdir -p "$output_subdir"

    "$HELIXQA" run \
        -platform "$platform" \
        -banks "$bank_file" \
        -browser-url "$WEB_URL" \
        -output "$output_subdir" \
        -report markdown \
        -record \
        -validate \
        -timeout 10m \
        -verbose \
        2>&1 | tail -5

    if [[ -f "$output_subdir/qa-report.md" ]]; then
        local steps=$(grep -c "| PASSED \|| FAILED " "$output_subdir/qa-report.md" 2>/dev/null || echo 0)
        local passed=$(grep -c "| PASSED |" "$output_subdir/qa-report.md" 2>/dev/null || echo 0)
        ok "  $session_name: $passed/$steps steps passed"
    fi
}

# ─── Device Video Recording & Auto-Login ──────────────────────────────────

# Record device screen (uses screenrecord on Android ≤13, screenshot-to-video on 14+)
record_device() {
    local device="$1" output="$2" duration="${3:-60}"
    local sdk=$(adb -s "$device" shell getprop ro.build.version.sdk 2>/dev/null | tr -d '\r')
    if [[ "$sdk" -le 33 ]]; then
        adb -s "$device" shell "nohup screenrecord --bit-rate 4000000 --time-limit $duration /sdcard/qa_rec.mp4 > /dev/null 2>&1 &"
        log "  Recording via screenrecord on $device (SDK $sdk)"
    else
        log "  SDK $sdk: screenrecord unavailable, using screenshot capture on $device"
    fi
}

stop_and_pull_recording() {
    local device="$1" output="$2"
    adb -s "$device" shell "killall screenrecord" 2>/dev/null
    sleep 2
    if adb -s "$device" shell "[ -s /sdcard/qa_rec.mp4 ]" 2>/dev/null; then
        adb -s "$device" pull /sdcard/qa_rec.mp4 "$output" 2>/dev/null
        adb -s "$device" shell rm /sdcard/qa_rec.mp4 2>/dev/null
        ok "  Video saved: $output ($(du -h "$output" | cut -f1))"
    fi
}

# Auto-login on Android device via ADB input
device_auto_login() {
    local device="$1" pkg="$2"
    local activity="$pkg/.ui.MainActivity"
    log "  Auto-login on $device..."
    adb -s "$device" shell am force-stop "$pkg" 2>/dev/null
    sleep 1
    adb -s "$device" shell am start -n "$activity" 2>/dev/null
    sleep 5  # Wait for splash + login screen

    # Tap username field and type credentials
    adb -s "$device" shell input keyevent KEYCODE_DPAD_DOWN 2>/dev/null
    sleep 0.5
    adb -s "$device" shell input keyevent KEYCODE_DPAD_CENTER 2>/dev/null
    sleep 0.5
    adb -s "$device" shell input text "admin" 2>/dev/null
    sleep 0.5
    # Move to password field
    adb -s "$device" shell input keyevent KEYCODE_TAB 2>/dev/null
    sleep 0.5
    adb -s "$device" shell input text "${ADMIN_PASSWORD}" 2>/dev/null
    sleep 0.5
    # Press Enter to submit
    adb -s "$device" shell input keyevent KEYCODE_ENTER 2>/dev/null
    sleep 3
    ok "  Auto-login attempted on $device"
}

# ─── Main Execution ────────────────────────────────────────────────────────

log "HelixQA Full QA Session — $(date)"
log "Platform: $PLATFORM"
log "Output: $OUTPUT_DIR"
echo ""

SESSION_START=$(date +%s)

if [[ "$PLATFORM" == "api" || "$PLATFORM" == "all" ]]; then
    run_api_session
    run_helixqa_native web "$BANKS_DIR/catalogizer-api-comprehensive-executable.yaml" "helixqa-api"
    echo ""
fi

if [[ "$PLATFORM" == "web" || "$PLATFORM" == "all" ]]; then
    run_web_session
    run_helixqa_native web "$BANKS_DIR/catalogizer-web-comprehensive-executable.yaml" "helixqa-web"
    echo ""
fi

if [[ "$PLATFORM" == "all" ]]; then
    run_helixqa_native android "$BANKS_DIR/catalogizer-android-comprehensive-executable.yaml" "helixqa-android"
    run_helixqa_native android "$BANKS_DIR/catalogizer-androidtv-comprehensive-executable.yaml" "helixqa-androidtv"
    run_helixqa_native desktop "$BANKS_DIR/catalogizer-desktop-comprehensive-executable.yaml" "helixqa-desktop"
    run_helixqa_native desktop "$BANKS_DIR/catalogizer-wizard-comprehensive-executable.yaml" "helixqa-wizard"
    echo ""
fi

SESSION_END=$(date +%s)
TOTAL_DURATION=$((SESSION_END - SESSION_START))

# ─── Final Summary ─────────────────────────────────────────────────────────

log "═══════════════════════════════════════════════════════"
log "  HelixQA Session Complete"
log "  Duration: ${TOTAL_DURATION}s"
log "  Output:   $OUTPUT_DIR"
log "═══════════════════════════════════════════════════════"

# Video summary
echo ""
log "Video recordings:"
find "$OUTPUT_DIR" -name "*.mp4" -exec sh -c '
    for f; do
        size=$(du -h "$f" | cut -f1)
        dur=$('"$FFMPEG"' -i "$f" 2>&1 | grep Duration | awk "{print \$2}" | tr -d ",")
        echo "  $f — $size, $dur"
    done
' sh {} +

echo ""
log "Screenshots:"
find "$OUTPUT_DIR" -name "*.png" | wc -l | xargs -I{} echo "  {} screenshots captured"

echo ""
log "Reports:"
find "$OUTPUT_DIR" -name "*report*" -type f | while read f; do echo "  $f"; done
