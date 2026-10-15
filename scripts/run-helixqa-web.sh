#!/usr/bin/env bash
# HelixQA Autonomous QA — Web (Playwright)
# Runs full 4-phase autonomous QA with video recording against the web frontend.
#
# Usage:
#   ./scripts/run-helixqa-web.sh [--api-url URL] [--web-url URL] [--timeout MINUTES]
#
# Prerequisites:
#   - catalog-api running on port 8080
#   - catalog-web running on port 3000
#   - npx playwright, chromium installed
#
# Output: qa-results/helixqa-web-<timestamp>/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
OUTPUT_DIR="$PROJECT_ROOT/qa-results/helixqa-web-${TIMESTAMP}"
HELIXQA="$PROJECT_ROOT/submodules/helix_qa/helixqa"
FFMPEG="${HELIX_RECORDING_FFMPEG_PATH:-/home/milosvasic/bin/ffmpeg}"

API_URL="${1:-http://localhost:8080}"
WEB_URL="${2:-http://localhost:3000}"
TIMEOUT_MINUTES=30
DISPLAY="${DISPLAY:-:0}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --api-url)   API_URL="$2"; shift 2 ;;
        --web-url)   WEB_URL="$2"; shift 2 ;;
        --timeout)   TIMEOUT_MINUTES="$2"; shift 2 ;;
        *)           shift ;;
    esac
done

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log()  { echo -e "${BLUE}[HelixQA-Web]${NC} $*"; }
ok()   { echo -e "${GREEN}[    OK     ]${NC} $*"; }
warn() { echo -e "${YELLOW}[   WARN   ]${NC} $*"; }
fail() { echo -e "${RED}[   FAIL   ]${NC} $*"; }

mkdir -p "$OUTPUT_DIR"/{videos,screenshots,evidence,logs}

# ═══ PHASE 1: SETUP ═════════════════════════════════════════════════════
log "═══ PHASE 1: SETUP ═══"

if ! curl -sf "$API_URL/health" > /dev/null 2>&1; then
    fail "catalog-api not running at $API_URL"; exit 1
fi
ok "catalog-api healthy"

if ! curl -sf -o /dev/null "$WEB_URL" 2>&1; then
    fail "catalog-web not running at $WEB_URL"; exit 1
fi
ok "catalog-web running"

# Get auth token
TOKEN=$(curl -sf -X POST "$API_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null | \
    python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_token', d.get('token','')))" 2>/dev/null || echo "")
if [[ -n "$TOKEN" ]]; then
    ok "Authenticated"
else
    warn "Could not obtain auth token"
fi

# Start screen recording with ffmpeg
if command -v "$FFMPEG" &>/dev/null && [[ -n "${DISPLAY:-}" ]]; then
    "$FFMPEG" -y -f x11grab -framerate 10 -video_size 1920x1080 -i "$DISPLAY" \
        -c:v libx264 -preset ultrafast -crf 28 \
        "$OUTPUT_DIR/videos/web-session.mp4" 2>/dev/null &
    FFMPEG_PID=$!
    ok "Screen recording started (ffmpeg PID=$FFMPEG_PID)"
else
    FFMPEG_PID=""
    warn "No DISPLAY or ffmpeg — skipping screen recording"
fi

# ═══ PHASE 2: DOC-DRIVEN VERIFICATION ═══════════════════════════════════
log "═══ PHASE 2: DOC-DRIVEN VERIFICATION ═══"

PAGES=(
    "/"
    "/login"
    "/dashboard"
    "/media"
    "/browse"
    "/collections"
    "/favorites"
    "/playlists"
    "/analytics"
    "/subtitles"
    "/conversion"
    "/admin"
    "/settings"
)

STEP=1
for page in "${PAGES[@]}"; do
    PAGE_URL="${WEB_URL}${page}"
    log "  Testing: $page"

    # Navigate and screenshot with Playwright
    npx playwright screenshot --browser chromium \
        --viewport-size "1920,1080" \
        "$PAGE_URL" \
        "$OUTPUT_DIR/screenshots/$(printf '%03d' $STEP)-${page//\//_}.png" 2>/dev/null || \
        warn "  Failed to screenshot $page"

    ((STEP++))
done
ok "Phase 2: ${#PAGES[@]} pages verified"

# ═══ PHASE 3: CURIOSITY-DRIVEN EXPLORATION ══════════════════════════════
log "═══ PHASE 3: CURIOSITY-DRIVEN EXPLORATION ═══"

# Run HelixQA autonomous if binary available
# HelixQA exit-code contract: 0 pass, 1 FAILED (app finding),
# 2 BLOCKED / undetermined (never a pass, never an app failure).
HQA_RC=2
if [[ -x "$HELIXQA" ]]; then
    log "  Running HelixQA autonomous engine..."
    set +e
    "$HELIXQA" autonomous \
        --project "$PROJECT_ROOT" \
        --platforms web \
        --timeout "${TIMEOUT_MINUTES}m" \
        --output "$OUTPUT_DIR/helixqa-autonomous" \
        --curiosity \
        --curiosity-timeout "10m" \
        --coverage-target 0.9 \
        --verbose 2>&1 | tee "$OUTPUT_DIR/logs/helixqa-autonomous.log"
    HQA_RC=${PIPESTATUS[0]}
    set -e
    case "$HQA_RC" in
        0) ok "HelixQA autonomous: PASSED (exit 0)" ;;
        1) warn "HelixQA autonomous: FAILED (exit 1: app finding)" ;;
        *) warn "HelixQA autonomous: BLOCKED / could not determine (exit $HQA_RC) — NOT a pass, NOT an app failure" ;;
    esac
else
    warn "HelixQA binary not found, running manual exploration..."

    # API endpoint exploration
    ENDPOINTS=(
        "/api/v1/stats/overall" "/api/v1/stats/filetypes" "/api/v1/stats/sizes"
        "/api/v1/entities" "/api/v1/entities/types" "/api/v1/entities/stats"
        "/api/v1/collections" "/api/v1/favorites" "/api/v1/challenges"
        "/api/v1/configuration" "/api/v1/roles" "/api/v1/users"
        "/api/v1/sync/sessions" "/api/v1/scans" "/api/v1/recommendations/trending"
    )

    for endpoint in "${ENDPOINTS[@]}"; do
        status=$(curl -sf -o /dev/null -w "%{http_code}" \
            -H "Authorization: Bearer $TOKEN" \
            "$API_URL$endpoint" 2>/dev/null || echo "000")
        echo "  $endpoint -> $status"
    done > "$OUTPUT_DIR/evidence/api-exploration.txt"
    ok "API exploration: ${#ENDPOINTS[@]} endpoints tested"
fi

# ═══ PHASE 4: REPORT & CLEANUP ══════════════════════════════════════════
log "═══ PHASE 4: REPORT & CLEANUP ═══"

if [[ -n "${FFMPEG_PID:-}" ]]; then
    kill "$FFMPEG_PID" 2>/dev/null || true
    wait "$FFMPEG_PID" 2>/dev/null || true
    ok "Screen recording stopped"
fi

REPORT="$OUTPUT_DIR/qa-report.md"
cat > "$REPORT" << EOF
# HelixQA Web Autonomous QA Report

**Generated:** $(date -Iseconds)
**Web URL:** $WEB_URL
**API URL:** $API_URL
**Duration:** ${TIMEOUT_MINUTES} minutes

## Pages Tested

| # | Page | Screenshot |
|---|------|-----------|
EOF

STEP=1
for page in "${PAGES[@]}"; do
    fname="$(printf '%03d' $STEP)-${page//\//_}.png"
    if [[ -f "$OUTPUT_DIR/screenshots/$fname" ]]; then
        echo "| $STEP | $page | $fname |" >> "$REPORT"
    else
        echo "| $STEP | $page | MISSING |" >> "$REPORT"
    fi
    ((STEP++))
done

SCREENSHOTS=$(find "$OUTPUT_DIR/screenshots" -name "*.png" 2>/dev/null | wc -l)
VIDEOS=$(find "$OUTPUT_DIR/videos" -name "*.mp4" 2>/dev/null | wc -l)

cat >> "$REPORT" << EOF

## Summary
- **Screenshots:** $SCREENSHOTS
- **Videos:** $VIDEOS
- **Pages tested:** ${#PAGES[@]}
- **HelixQA exit code:** $HQA_RC (0 pass · 1 failed · 2 blocked/undetermined; manual fallback = 2)

---
*Generated by HelixQA*
EOF

ok "Report: $REPORT"
log "═══ SESSION COMPLETE ═══"
log "Results: $OUTPUT_DIR"

# Propagate the contract (a manual fallback run is never a HelixQA pass).
case "$HQA_RC" in
    0) exit 0 ;;
    1) exit 1 ;;
    *) exit 2 ;;
esac
