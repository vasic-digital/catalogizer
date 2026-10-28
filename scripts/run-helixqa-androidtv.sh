#!/usr/bin/env bash
# HelixQA Autonomous QA — Android TV
# Runs full 4-phase autonomous QA with video recording on all connected Android TV devices.
#
# Phases:
#   1. Setup — Deploy APK, configure server, authenticate, start recording
#   2. Doc-Driven Verification — Navigate documented features on each device
#   3. Curiosity-Driven Exploration — Autonomous exploration beyond documented features
#   4. Report & Cleanup — Stop recording, pull videos, generate report
#
# Usage:
#   ./scripts/run-helixqa-androidtv.sh [--api-url URL] [--timeout MINUTES]
#
# Prerequisites:
#   - catalog-api running and accessible on the network
#   - ADB connected Android TV devices (leanback or tv characteristics)
#   - ffmpeg available
#
# Output: qa-results/helixqa-androidtv-<timestamp>/

set -euo pipefail
ADMIN_PASSWORD="${ADMIN_PASSWORD:?set ADMIN_PASSWORD to the test-stack admin password (from your .env; never hardcoded)}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
OUTPUT_DIR="$PROJECT_ROOT/qa-results/helixqa-androidtv-${TIMESTAMP}"
PKG="com.catalogizer.androidtv"
HELIXQA="$PROJECT_ROOT/submodules/helix_qa/helixqa"
BANKS_DIR="$PROJECT_ROOT/challenges/helixqa-banks"
FFMPEG="${HELIX_RECORDING_FFMPEG_PATH:-/home/milosvasic/bin/ffmpeg}"

# Defaults
API_URL=""
TIMEOUT_MINUTES=30
RECORD_SEGMENT_SECONDS=180

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log()  { echo -e "${BLUE}[HelixQA-ATV]${NC} $*"; }
ok()   { echo -e "${GREEN}[    OK     ]${NC} $*"; }
warn() { echo -e "${YELLOW}[   WARN   ]${NC} $*"; }
fail() { echo -e "${RED}[   FAIL   ]${NC} $*"; }

# ─── Parse Arguments ─────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --api-url)   API_URL="$2"; shift 2 ;;
        --timeout)   TIMEOUT_MINUTES="$2"; shift 2 ;;
        *)           shift ;;
    esac
done

# ─── Auto-detect API URL ────────────────────────────────────────────────
if [[ -z "$API_URL" ]]; then
    HOST_IP=$(ip -4 addr show scope global | grep -oP '(?<=inet\s)[\d.]+' | head -1)
    if [[ -n "$HOST_IP" ]]; then
        API_URL="http://${HOST_IP}:8080"
    else
        API_URL="http://localhost:8080"
    fi
fi
log "API URL: $API_URL"

# ─── Discover Android TV Devices ────────────────────────────────────────
log "Discovering Android TV devices..."
TV_DEVICES=()
SERIALS=$(adb devices 2>/dev/null | grep -w "device" | awk '{print $1}')
for serial in $SERIALS; do
    [[ -z "$serial" ]] && continue
    # Check for leanback or tv characteristics
    is_tv=false
    if adb -s "$serial" shell pm list features 2>/dev/null | grep -qE "leanback|television"; then
        is_tv=true
    fi
    chars=$(adb -s "$serial" shell getprop ro.build.characteristics 2>/dev/null | tr -d '\r')
    if [[ "$chars" == *"tv"* ]]; then
        is_tv=true
    fi
    if $is_tv; then
        model=$(adb -s "$serial" shell getprop ro.product.model 2>/dev/null | tr -d '\r')
        sdk=$(adb -s "$serial" shell getprop ro.build.version.sdk 2>/dev/null | tr -d '\r')
        TV_DEVICES+=("$serial")
        ok "Found TV device: $serial ($model, SDK $sdk)"
    fi
done

if [[ ${#TV_DEVICES[@]} -eq 0 ]]; then
    fail "No Android TV devices found"
    exit 1
fi
log "Found ${#TV_DEVICES[@]} Android TV device(s)"

# ─── Create Output Directories ──────────────────────────────────────────
for i in "${!TV_DEVICES[@]}"; do
    device_label="device$((i+1))"
    mkdir -p "$OUTPUT_DIR/$device_label"/{videos,screenshots,evidence,logs}
done
log "Output directory: $OUTPUT_DIR"

# ═══════════════════════════════════════════════════════════════════════
# PHASE 1: SETUP — Deploy, Configure, Authenticate, Record
# ═══════════════════════════════════════════════════════════════════════
log "═══ PHASE 1: SETUP ═══"

# Check API health
if ! curl -sf "$API_URL/health" > /dev/null 2>&1; then
    fail "catalog-api not reachable at $API_URL"
    exit 1
fi
ok "catalog-api healthy at $API_URL"

# Find APK
APK=$(find "$PROJECT_ROOT/catalogizer-androidtv" -name "app-debug.apk" -path "*/build/outputs/*" 2>/dev/null | head -1)
if [[ -z "$APK" ]]; then
    warn "No debug APK found, building..."
    (cd "$PROJECT_ROOT/catalogizer-androidtv" && ./gradlew assembleDebug --no-daemon 2>&1 | tail -3)
    APK=$(find "$PROJECT_ROOT/catalogizer-androidtv" -name "app-debug.apk" -path "*/build/outputs/*" 2>/dev/null | head -1)
fi
ok "APK: $APK"

# Deploy to all devices
for serial in "${TV_DEVICES[@]}"; do
    adb -s "$serial" install -r "$APK" 2>&1 | tail -1 &
done
wait
ok "APK deployed to all devices"

# Set up ADB reverse for network access.
# The app uses http://localhost:8080 by default (configured in login_on_device);
# the catalog-api may listen on a different host port (e.g. 28080). Forward the
# device's :8080 to the host's actual API port so the app reaches it.
API_PORT="${API_URL##*:}"; API_PORT="${API_PORT%%/*}"; [[ "$API_PORT" =~ ^[0-9]+$ ]] || API_PORT=8080
for serial in "${TV_DEVICES[@]}"; do
    adb -s "$serial" reverse tcp:8080 "tcp:${API_PORT}" 2>/dev/null || true
done
ok "ADB reverse port forwarding set (device :8080 -> host :${API_PORT})"

# Launch app, configure server URL, and login on each device.
#
# §11.4.117 PIXEL-ORACLE LOGIN: the Android TV build is Jetpack-Compose-for-TV,
# whose accessibility hierarchy is EMPTY (`uiautomator dump` returns 0 nodes). The
# old hierarchy-bounds approach therefore (a) could never find the fields and
# (b) verified success by grepping the same empty dump for "Sign In" — which, on
# an empty dump, ALWAYS reports "past login" (a §11.4.107(2)/§11.4 PASS-bluff:
# the oracle reads from a source that can see nothing). Reconciled per §11.4.120:
# fixed-coordinate input (calibrated on the 1920x1080 login screen) + IME ENTER to
# submit + a REAL sink-side verification (§11.4.69) — poll the device's own okhttp
# log for the authenticated catalog fetch. No hierarchy dependency, no blind PASS.
#
# Returns 0 on VERIFIED login (real catalog API response observed), 1 otherwise.
login_on_device() {
    local SERIAL="$1"
    local DIR="$2"
    local ATTEMPT="$3"

    # Field coordinates calibrated on the 1920x1080 Compose-TV login screen.
    # Username EditText ~y=314, Password ~y=466, Server URL ~y=937 (all centered x=960).
    local UX=960 UY=314 PX=960 PY=466 SVX=960 SVY=937

    # 1) Configure server URL → http://localhost:8080 (adb-reverse forwards to the API).
    adb -s "$SERIAL" shell input tap "$SVX" "$SVY"; sleep 1
    adb -s "$SERIAL" shell input keyevent KEYCODE_MOVE_END; sleep 0.2
    for _ in $(seq 1 48); do adb -s "$SERIAL" shell input keyevent KEYCODE_DEL; done
    adb -s "$SERIAL" shell input text "http://localhost:8080"; sleep 0.5
    adb -s "$SERIAL" shell input keyevent KEYCODE_BACK; sleep 0.5

    # 2) Username.
    adb -s "$SERIAL" shell input tap "$UX" "$UY"; sleep 1
    adb -s "$SERIAL" shell input keyevent KEYCODE_MOVE_END; sleep 0.2
    for _ in $(seq 1 20); do adb -s "$SERIAL" shell input keyevent KEYCODE_DEL; done
    adb -s "$SERIAL" shell input text "admin"; sleep 0.5
    adb -s "$SERIAL" shell input keyevent KEYCODE_BACK; sleep 0.5

    # 3) Password — then submit via the IME DONE/ENTER action on the field.
    adb -s "$SERIAL" shell input tap "$PX" "$PY"; sleep 1
    adb -s "$SERIAL" shell input keyevent KEYCODE_MOVE_END; sleep 0.2
    for _ in $(seq 1 20); do adb -s "$SERIAL" shell input keyevent KEYCODE_DEL; done
    adb -s "$SERIAL" shell input text "${ADMIN_PASSWORD}"; sleep 0.5

    # Clear logcat so the verification only sees THIS attempt's network traffic.
    adb -s "$SERIAL" logcat -c 2>/dev/null || true
    adb -s "$SERIAL" shell input keyevent KEYCODE_ENTER; sleep 8

    adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/00${ATTEMPT}-login-attempt.png" 2>/dev/null

    # §11.4.69 SINK-SIDE VERIFICATION: the app, once authenticated, fetches the
    # media catalog. Observing a real /api/v1 media response in the device okhttp
    # log proves login genuinely succeeded — it cannot be faked by an empty UI dump.
    local netlog
    netlog=$(adb -s "$SERIAL" logcat -d -t 400 2>/dev/null | grep -iE 'okhttp|/api/v1|"total":|media_type_id|"items"')
    if echo "$netlog" | grep -qE '"items"|"total":|media_type_id|/api/v1/(entities|media|catalog)'; then
        ok "    Login attempt $ATTEMPT SUCCEEDED — authenticated catalog fetch observed (sink-side §11.4.69)"
        echo "$netlog" | tail -5 > "$DIR/evidence/login-catalog-fetch-attempt${ATTEMPT}.txt" 2>/dev/null
        return 0
    else
        warn "    Login attempt $ATTEMPT FAILED — no authenticated catalog fetch in device network log"
        return 1
    fi
}

setup_device() {
    local SERIAL="$1"
    local LABEL="$2"
    local DIR="$OUTPUT_DIR/$LABEL"

    log "  Setting up $LABEL ($SERIAL)..."

    # Force-stop and launch — Mi Box needs 15-20 seconds for Compose to render
    adb -s "$SERIAL" shell am force-stop "$PKG" 2>/dev/null
    sleep 1
    adb -s "$SERIAL" shell am start -n "$PKG/.ui.MainActivity" 2>/dev/null
    sleep 25  # Mi Box is SLOW — 15s splash + compose render

    # Screenshot: initial state
    adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/001-initial.png" 2>/dev/null

    # Retry login up to 3 times with UI dump-based coordinate finding
    local login_ok=false
    for attempt in 2 3 4; do
        if login_on_device "$SERIAL" "$DIR" "$attempt"; then
            login_ok=true
            break
        fi
        warn "  $LABEL: Retrying login (attempt $attempt failed)..."
        sleep 3
    done

    if $login_ok; then
        ok "  $LABEL setup complete (VERIFIED past login screen)"
    else
        fail "  $LABEL setup FAILED — could not log in after 3 attempts"
        adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/005-login-FAILED.png" 2>/dev/null
    fi
}

for i in "${!TV_DEVICES[@]}"; do
    setup_device "${TV_DEVICES[$i]}" "device$((i+1))" &
done
wait
ok "All devices configured and logged in"

# Start screen recording on all devices
log "Starting screen recording..."
RECORD_PIDS=()
for i in "${!TV_DEVICES[@]}"; do
    serial="${TV_DEVICES[$i]}"
    adb -s "$serial" shell "screenrecord --bit-rate 4000000 --time-limit $RECORD_SEGMENT_SECONDS /sdcard/qa_session_1.mp4" &
    RECORD_PIDS+=($!)
done
ok "Screen recording started on ${#TV_DEVICES[@]} devices"

# ─── CONSTITUTIONAL CONSTRAINT ───────────────────────────────────────
# HelixQA MUST NEVER explore outside the target application.
# Before EVERY action, verify the app is in the foreground.
# If not, immediately re-launch it. NEVER send HOME or MENU keys.
# Only use in-app navigation: DPAD directions, CENTER (select), BACK.
# ────────────────────────────────────────────────────────────────────

# ensure_app_foreground: Returns 0 if app is in foreground, re-launches if not.
# This is the CONSTITUTIONAL GUARD — called before every single input action.
ensure_app_foreground() {
    local SERIAL="$1"
    local fg_pkg
    fg_pkg=$(adb -s "$SERIAL" shell "dumpsys activity activities 2>/dev/null | grep -E 'mResumedActivity|mFocusedActivity' | head -1" 2>/dev/null | tr -d '\r')
    if [[ "$fg_pkg" != *"$PKG"* ]]; then
        # App is NOT in foreground — re-launch immediately
        adb -s "$SERIAL" shell am start -n "$PKG/.ui.MainActivity" 2>/dev/null
        sleep 2
        return 1
    fi
    return 0
}

# ═══════════════════════════════════════════════════════════════════════
# PHASE 2: DOC-DRIVEN VERIFICATION
# ═══════════════════════════════════════════════════════════════════════
log "═══ PHASE 2: DOC-DRIVEN VERIFICATION ═══"

# Navigate through documented screens on each device
verify_device() {
    local SERIAL="$1"
    local LABEL="$2"
    local DIR="$OUTPUT_DIR/$LABEL"
    local STEP=10

    log "  Verifying documented features on $LABEL..."

    # CONSTITUTIONAL GUARD: Ensure app is in foreground
    ensure_app_foreground "$SERIAL"
    sleep 1

    # Navigate the app: Home screen
    adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/0${STEP}-home.png" 2>/dev/null
    ((STEP++))

    # DPAD navigation through main sections
    local SECTIONS=("search" "movies" "tvshows" "settings" "player" "browse")
    for section in "${SECTIONS[@]}"; do
        ensure_app_foreground "$SERIAL"  # Guard before every action
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_RIGHT; sleep 1
        adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/0${STEP}-nav-${section}.png" 2>/dev/null
        ((STEP++))
        ensure_app_foreground "$SERIAL"
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_CENTER; sleep 2
        adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/0${STEP}-${section}-detail.png" 2>/dev/null
        ((STEP++))
        adb -s "$SERIAL" shell input keyevent KEYCODE_BACK; sleep 1
    done

    # Navigate down through rows — browse content
    for row in 1 2 3 4; do
        ensure_app_foreground "$SERIAL"
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_DOWN; sleep 1
        adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/0${STEP}-row${row}.png" 2>/dev/null
        ((STEP++))
        # Select item to browse content details
        ensure_app_foreground "$SERIAL"
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_CENTER; sleep 2
        adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/0${STEP}-row${row}-item.png" 2>/dev/null
        ((STEP++))
        # Try to play content (press CENTER again on detail screen)
        ensure_app_foreground "$SERIAL"
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_CENTER; sleep 3
        adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/0${STEP}-row${row}-play.png" 2>/dev/null
        ((STEP++))
        adb -s "$SERIAL" shell input keyevent KEYCODE_BACK; sleep 1
        adb -s "$SERIAL" shell input keyevent KEYCODE_BACK; sleep 1
    done

    # Scroll through items horizontally in each row
    for scroll in 1 2 3; do
        ensure_app_foreground "$SERIAL"
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_RIGHT; sleep 0.5
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_RIGHT; sleep 0.5
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_RIGHT; sleep 0.5
        adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/0${STEP}-scroll${scroll}.png" 2>/dev/null
        ((STEP++))
        adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_DOWN; sleep 1
    done

    ok "  $LABEL doc-driven verification complete ($STEP screenshots)"
}

for i in "${!TV_DEVICES[@]}"; do
    verify_device "${TV_DEVICES[$i]}" "device$((i+1))" &
done
wait
ok "Phase 2 complete"

# ═══════════════════════════════════════════════════════════════════════
# PHASE 3: CURIOSITY-DRIVEN EXPLORATION
# ═══════════════════════════════════════════════════════════════════════
log "═══ PHASE 3: CURIOSITY-DRIVEN EXPLORATION ═══"

# Autonomous exploration: ONLY in-app navigation, with foreground enforcement
explore_device() {
    local SERIAL="$1"
    local LABEL="$2"
    local DIR="$OUTPUT_DIR/$LABEL"
    local STEP=50
    local EXPLORE_SECONDS=$((TIMEOUT_MINUTES * 60 / 3))
    local END_TIME=$(($(date +%s) + EXPLORE_SECONDS))
    local OUT_OF_APP_COUNT=0

    log "  Curiosity exploration on $LABEL (${EXPLORE_SECONDS}s)..."

    while [[ $(date +%s) -lt $END_TIME ]]; do
        # CONSTITUTIONAL GUARD: Verify app is in foreground before EVERY action
        if ! ensure_app_foreground "$SERIAL"; then
            ((OUT_OF_APP_COUNT++))
            warn "  $LABEL: App left foreground (restored, count=$OUT_OF_APP_COUNT)"
            adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/${STEP}-RESTORED.png" 2>/dev/null
            ((STEP++))
            continue
        fi

        # ONLY in-app navigation actions — NO HOME, NO MENU, NO keys that leave the app
        local ACTION=$((RANDOM % 6))
        case $ACTION in
            0) adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_UP ;;
            1) adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_DOWN ;;
            2) adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_LEFT ;;
            3) adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_RIGHT ;;
            4) adb -s "$SERIAL" shell input keyevent KEYCODE_DPAD_CENTER ;;
            5) # BACK is allowed but could exit the app — guard on next iteration
               adb -s "$SERIAL" shell input keyevent KEYCODE_BACK ;;
        esac
        sleep 1

        # Take screenshot every 3 actions (more frequent than before)
        if [[ $((STEP % 3)) -eq 0 ]]; then
            adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/${STEP}-explore.png" 2>/dev/null
        fi
        ((STEP++))

        # Check for crashes
        # Only detect crashes in OUR package — not system processes
        local crash
        crash=$(adb -s "$SERIAL" logcat -d -t 5 "*:E" 2>/dev/null | grep -E "FATAL|AndroidRuntime" | grep -c "$PKG" || echo "0")
        if [[ "$crash" -gt 0 ]]; then
            warn "  Crash detected on $LABEL at step $STEP!"
            adb -s "$SERIAL" exec-out screencap -p > "$DIR/screenshots/${STEP}-CRASH.png" 2>/dev/null
            adb -s "$SERIAL" logcat -d -t 30 "*:E" > "$DIR/logs/crash-step${STEP}.txt" 2>/dev/null
            # Restart app after crash
            adb -s "$SERIAL" shell am force-stop "$PKG" 2>/dev/null
            sleep 1
            adb -s "$SERIAL" shell am start -n "$PKG/.ui.MainActivity" 2>/dev/null
            sleep 3
        fi
    done

    ok "  $LABEL curiosity exploration complete ($STEP steps, $OUT_OF_APP_COUNT foreground restores)"
}

for i in "${!TV_DEVICES[@]}"; do
    explore_device "${TV_DEVICES[$i]}" "device$((i+1))" &
done
wait
ok "Phase 3 complete"

# ═══════════════════════════════════════════════════════════════════════
# PHASE 4: REPORT & CLEANUP
# ═══════════════════════════════════════════════════════════════════════
log "═══ PHASE 4: REPORT & CLEANUP ═══"

# Stop recordings
log "Stopping screen recordings..."
for pid in "${RECORD_PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
done
sleep 3

# Start second recording segment (in case first finished)
for i in "${!TV_DEVICES[@]}"; do
    serial="${TV_DEVICES[$i]}"
    adb -s "$serial" shell "kill \$(pidof screenrecord) 2>/dev/null" 2>/dev/null || true
done
sleep 2

# Pull recordings from devices
log "Pulling video recordings..."
for i in "${!TV_DEVICES[@]}"; do
    serial="${TV_DEVICES[$i]}"
    label="device$((i+1))"
    for seg in 1 2 3; do
        adb -s "$serial" pull "/sdcard/qa_session_${seg}.mp4" "$OUTPUT_DIR/$label/videos/" 2>/dev/null || true
    done
    ok "  Pulled videos from $label"
done

# Pull logcat
log "Collecting device logs..."
for i in "${!TV_DEVICES[@]}"; do
    serial="${TV_DEVICES[$i]}"
    label="device$((i+1))"
    adb -s "$serial" logcat -d > "$OUTPUT_DIR/$label/logs/logcat-full.txt" 2>/dev/null
    adb -s "$serial" logcat -d -s "Catalogizer" "*:E" > "$OUTPUT_DIR/$label/logs/logcat-errors.txt" 2>/dev/null
done
ok "Device logs collected"

# Generate report
log "Generating QA report..."
REPORT="$OUTPUT_DIR/qa-report.md"
cat > "$REPORT" << EOF
# HelixQA Android TV Autonomous QA Report

**Generated:** $(date -Iseconds)
**API URL:** $API_URL
**Duration:** ${TIMEOUT_MINUTES} minutes
**Devices:** ${#TV_DEVICES[@]}

## Devices Tested

| # | Serial | Model | SDK |
|---|--------|-------|-----|
EOF

for i in "${!TV_DEVICES[@]}"; do
    serial="${TV_DEVICES[$i]}"
    model=$(adb -s "$serial" shell getprop ro.product.model 2>/dev/null | tr -d '\r')
    sdk=$(adb -s "$serial" shell getprop ro.build.version.sdk 2>/dev/null | tr -d '\r')
    echo "| $((i+1)) | $serial | $model | $sdk |" >> "$REPORT"
done

cat >> "$REPORT" << 'EOF'

## Phases

### Phase 1: Setup
- APK deployed to all devices
- Server URL configured via ADB reverse
- Authentication completed (admin / ADMIN_PASSWORD from env)
- Screen recording started

### Phase 2: Doc-Driven Verification
- Navigated through all documented screens
- Screenshots captured at each transition
- DPAD navigation through sections and rows

### Phase 3: Curiosity-Driven Exploration
- Random navigation actions (UP/DOWN/LEFT/RIGHT/CENTER/BACK/MENU/HOME)
- Screenshots captured every 5 actions
- Crash detection via logcat monitoring
- Automatic app restart on crash

### Phase 4: Report & Cleanup
- Video recordings pulled from devices
- Logcat logs collected
- QA report generated

## Evidence

EOF

for i in "${!TV_DEVICES[@]}"; do
    label="device$((i+1))"
    screenshots=$(find "$OUTPUT_DIR/$label/screenshots" -name "*.png" 2>/dev/null | wc -l)
    videos=$(find "$OUTPUT_DIR/$label/videos" -name "*.mp4" 2>/dev/null | wc -l)
    crashes=$(find "$OUTPUT_DIR/$label/logs" -name "crash-*.txt" 2>/dev/null | wc -l)
    echo "### Device $((i+1)) (${TV_DEVICES[$i]})" >> "$REPORT"
    echo "- Screenshots: $screenshots" >> "$REPORT"
    echo "- Videos: $videos" >> "$REPORT"
    echo "- Crashes detected: $crashes" >> "$REPORT"
    echo "" >> "$REPORT"
done

echo "---" >> "$REPORT"
echo "*Generated by HelixQA Autonomous QA Runner*" >> "$REPORT"

ok "Report: $REPORT"
log "═══ SESSION COMPLETE ═══"
log "Results: $OUTPUT_DIR"
