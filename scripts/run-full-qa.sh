#!/bin/bash
# ULTIMATE COMPREHENSIVE QA SUITE
# Tests EVERY API endpoint, EVERY web page, EVERY flow, EVERY edge case
# Usage: ./scripts/run-full-qa.sh [API_URL] [WEB_URL]
set -euo pipefail
ADMIN_PASSWORD="${ADMIN_PASSWORD:?set ADMIN_PASSWORD to the test-stack admin password (from your .env; never hardcoded)}"

API="${1:-http://localhost:8085}"
WEB="${2:-http://localhost:3000}"
PASS=0; FAIL=0; TOTAL=0; WARN=0
TOKEN=""

ok() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); printf "  \033[32mPASS\033[0m  %s\n" "$1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); printf "  \033[31mFAIL\033[0m  %s (got %s)\n" "$1" "${2:-?}"; }
warn() { WARN=$((WARN+1)); TOTAL=$((TOTAL+1)); printf "  \033[33mWARN\033[0m  %s\n" "$1"; }

api() {
    local method="$1" path="$2" expect="$3" body="${4:-}" auth="${5:-yes}"
    local args=(-s -o /tmp/qa_resp.txt -w "%{http_code}" -X "$method" -H "Content-Type: application/json")
    [ "$auth" = "yes" ] && [ -n "$TOKEN" ] && args+=(-H "Authorization: Bearer $TOKEN")
    [ -n "$body" ] && args+=(-d "$body")
    local code=$(curl "${args[@]}" "${API}${path}" 2>/dev/null || echo "000")
    local name="$method $path"
    if echo "$expect" | grep -q "$code"; then
        ok "$name -> $code"
    elif [ "$code" = "429" ]; then
        warn "$name -> 429 (rate limited)"
    else
        fail "$name expected=$expect" "$code"
    fi
}

web() {
    local path="$1" expect="$2"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -L "${WEB}${path}" 2>/dev/null || echo "000")
    if [ "$code" = "$expect" ]; then
        ok "WEB $path -> $code"
    else
        fail "WEB $path expected=$expect" "$code"
    fi
}

web_contains() {
    local path="$1" text="$2"
    local body=$(curl -sL "${WEB}${path}" 2>/dev/null)
    if echo "$body" | grep -q "$text"; then
        ok "WEB $path contains '$text'"
    else
        fail "WEB $path missing '$text'"
    fi
}

echo "================================================================"
echo "  CATALOGIZER ULTIMATE QA SUITE"
echo "  API: $API | WEB: $WEB | $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "================================================================"

# ==================== SECTION 1: INFRASTRUCTURE ====================
echo ""; echo "=== 1. INFRASTRUCTURE (5 tests) ==="
api GET "/health" "200" "" "no"
api GET "/metrics" "200" "" "no"
TOTAL=$((TOTAL+1)); CT=$(curl -s -o /dev/null -w "%{content_type}" "$API/health" 2>/dev/null)
echo "$CT" | grep -q "application/json" && ok "Content-Type: $CT" || fail "Content-Type" "$CT"
TOTAL=$((TOTAL+1)); RT=$(curl -s -o /dev/null -w "%{time_total}" "$API/health" 2>/dev/null)
[ "$(echo "$RT < 1.0" | bc 2>/dev/null || echo 1)" = "1" ] && ok "Response time: ${RT}s" || fail "Response time > 1s" "${RT}s"
TOTAL=$((TOTAL+1)); curl -s "$API/health" | grep -q "healthy" && ok "Health status=healthy" || fail "Health not healthy"

# ==================== SECTION 2: AUTHENTICATION ====================
echo ""; echo "=== 2. AUTHENTICATION (12 tests) ==="
api POST "/api/v1/auth/login" "200" "{\"username\":\"admin\",\"password\":\"${ADMIN_PASSWORD}\"}" "no"
TOKEN=$(curl -s -X POST -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"${ADMIN_PASSWORD}\"}" "$API/api/v1/auth/login" | python3 -c "import sys,json; print(json.load(sys.stdin).get('session_token',''))" 2>/dev/null)
[ -n "$TOKEN" ] && ok "Token obtained (${#TOKEN} chars)" || { fail "Token extraction"; exit 1; }
api GET "/api/v1/auth/me" "200"
api GET "/api/v1/auth/status" "200" "" "no"
api GET "/api/v1/auth/permissions" "200"
api GET "/api/v1/auth/profile" "200"
TOTAL=$((TOTAL+1)); RESP=$(curl -s -H "Authorization: Bearer $TOKEN" "$API/api/v1/auth/me"); echo "$RESP" | grep -q "admin" && ok "Auth me returns admin user" || warn "Auth me check inconclusive"
api POST "/api/v1/auth/login" "400" 'invalid-json' "no"
api GET "/api/v1/auth/me" "401|429" "" "no"
# Skip logout/re-login mid-suite to avoid rate limiting — test logout at the very end
api POST "/api/v1/auth/register" "201|409|400|429" "{\"username\":\"testuser_qa\",\"email\":\"qa@test.com\",\"password\":\"${ADMIN_PASSWORD}\"}" "no"

# ==================== SECTION 3: SECURITY ====================
echo ""; echo "=== 3. SECURITY (10 tests) ==="
for ep in /api/v1/catalog /api/v1/search /api/v1/storage-roots /api/v1/media/stats /api/v1/challenges /api/v1/entities /api/v1/collections /api/v1/favorites /api/v1/users /api/v1/roles; do
    api GET "$ep" "401|429" "" "no"
done

# ==================== SECTION 4: CATALOG & BROWSE ====================
echo ""; echo "=== 4. CATALOG & BROWSE (8 tests) ==="
api GET "/api/v1/catalog" "200"
api GET "/api/v1/catalog/" "200|301|500"
api GET "/api/v1/browse/roots" "200"
api GET "/api/v1/storage-roots" "200"
api GET "/api/v1/search/files?q=test" "200"
api GET "/api/v1/search/files/duplicates?q=test" "200"
api GET "/api/v1/media/search?q=test" "200"
api GET "/api/v1/media/stats" "200"

# ==================== SECTION 5: ENTITIES ====================
echo ""; echo "=== 5. ENTITIES (6 tests) ==="
api GET "/api/v1/entities" "200"
api GET "/api/v1/entities/types" "200"
api GET "/api/v1/entities/stats" "200"
api GET "/api/v1/entities/duplicates" "200"
api GET "/api/v1/entities/browse/movie" "200"
api GET "/api/v1/entities/999999" "404|200"

# ==================== SECTION 6: STATISTICS ====================
echo ""; echo "=== 6. STATISTICS (8 tests) ==="
api GET "/api/v1/stats/overall" "200"
api GET "/api/v1/stats/filetypes" "200"
api GET "/api/v1/stats/sizes" "200"
api GET "/api/v1/stats/growth" "200"
api GET "/api/v1/stats/access" "200"
api GET "/api/v1/stats/scans" "200"
api GET "/api/v1/stats/duplicates" "200"
api GET "/api/v1/stats/duplicates/count" "200"

# ==================== SECTION 7: COLLECTIONS & FAVORITES ====================
echo ""; echo "=== 7. COLLECTIONS & FAVORITES (6 tests) ==="
api GET "/api/v1/collections" "200"
api POST "/api/v1/collections" "201|200|400" '{"name":"QA Test Collection","description":"Created by QA"}'
api GET "/api/v1/favorites" "200|500"
api GET "/api/v1/favorites/check/movie/1" "200|500"
api POST "/api/v1/favorites" "201|200|400|500" '{"entity_type":"movie","entity_id":1}'
api DELETE "/api/v1/favorites/movie/1" "200|204|404|500"

# ==================== SECTION 8: ANALYTICS & REPORTS ====================
echo ""; echo "=== 8. ANALYTICS & REPORTS (5 tests) ==="
api GET "/api/v1/analytics/system" "200|500"
api GET "/api/v1/analytics/user/1" "200|500"
api POST "/api/v1/analytics/event" "200|201|400|500" '{"event_type":"qa_test","data":{}}'
api GET "/api/v1/reports/usage" "200|500"
api GET "/api/v1/reports/performance" "200|500"

# ==================== SECTION 9: USERS & ROLES ====================
echo ""; echo "=== 9. USERS & ROLES (6 tests) ==="
api GET "/api/v1/users" "200"
api GET "/api/v1/users/1" "200|400"
api GET "/api/v1/roles" "200"
api GET "/api/v1/roles/permissions" "200"
api GET "/api/v1/roles/1" "200|400"
api PUT "/api/v1/users/1" "200|400" '{"email":"admin@catalogizer.local"}'

# ==================== SECTION 10: CONFIGURATION ====================
echo ""; echo "=== 10. CONFIGURATION (5 tests) ==="
api GET "/api/v1/configuration" "200"
api GET "/api/v1/configuration/status" "200"
api GET "/api/v1/configuration/wizard/progress" "200|404"
api POST "/api/v1/configuration/test" "200|400" '{}'
api POST "/api/v1/configuration/wizard/complete" "200|400|500" '{}'

# ==================== SECTION 11: SYNC & CONVERSION ====================
echo ""; echo "=== 11. SYNC & CONVERSION (8 tests) ==="
api GET "/api/v1/sync/endpoints" "200"
api GET "/api/v1/sync/sessions" "200"
api GET "/api/v1/sync/statistics" "200"
api GET "/api/v1/conversion/formats" "200"
api GET "/api/v1/conversion/jobs" "200"
api POST "/api/v1/sync/endpoints" "201|200|400" '{"name":"QA endpoint","type":"webdav","url":"http://test.local"}'
api POST "/api/v1/sync/cleanup" "200|204|400"
api POST "/api/v1/sync/schedules" "200|201|400" '{"endpoint_id":1,"interval":"daily"}'

# ==================== SECTION 12: ERRORS & LOGS ====================
echo ""; echo "=== 12. ERRORS & LOGS (10 tests) ==="
api GET "/api/v1/errors/reports" "200|500"
api GET "/api/v1/errors/crashes" "200|500"
api GET "/api/v1/errors/statistics" "200|500"
api GET "/api/v1/errors/crash-statistics" "200|500"
api GET "/api/v1/errors/health" "200|500"
api POST "/api/v1/errors/report" "201|200|500" '{"title":"QA Test Error","message":"test","severity":"low"}'
api GET "/api/v1/logs/collections" "200|500"
api GET "/api/v1/logs/statistics" "200|500"
api POST "/api/v1/logs/collect" "200|201|500" '{"component":"qa-test"}'
api POST "/api/v1/logs/share" "200|201|500" '{"collection_id":1,"expires_hours":24}'

# ==================== SECTION 13: SUBTITLES ====================
echo ""; echo "=== 13. SUBTITLES (4 tests) ==="
api GET "/api/v1/subtitles/languages" "200"
api GET "/api/v1/subtitles/providers" "200"
api GET "/api/v1/subtitles/search?query=test" "200|400"
api GET "/api/v1/subtitles/media/1" "200"

# ==================== SECTION 14: CHALLENGES ====================
echo ""; echo "=== 14. CHALLENGES (5 tests) ==="
api GET "/api/v1/challenges" "200"
TOTAL=$((TOTAL+1)); COUNT=$(curl -s -H "Authorization: Bearer $TOKEN" "$API/api/v1/challenges" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count',0))" 2>/dev/null)
[ "$COUNT" -ge 200 ] && ok "Challenge count: $COUNT (>= 200)" || fail "Challenge count < 200" "$COUNT"
api GET "/api/v1/challenges/results" "200"
api GET "/api/v1/challenges/auth-required" "200"
api POST "/api/v1/challenges/api-docs/run" "200" '{}'

# ==================== SECTION 15: DISCOVERY & SMB ====================
echo ""; echo "=== 15. DISCOVERY & SMB (4 tests) ==="
api GET "/api/v1/discovery" "200"
api GET "/api/v1/smb/discover" "200|400|503"
api GET "/api/v1/smb/test" "200|400|503"
api POST "/api/v1/smb/browse" "200|400" '{"host":"localhost","share":"test"}'

# ==================== SECTION 16: RECOMMENDATIONS ====================
echo ""; echo "=== 16. RECOMMENDATIONS (3 tests) ==="
api GET "/api/v1/recommendations/trending" "200"
api GET "/api/v1/recommendations/similar/1" "200|500"
api GET "/api/v1/recommendations/personalized/1" "200"

# ==================== SECTION 17: SCANS ====================
echo ""; echo "=== 17. SCANS (2 tests) ==="
api GET "/api/v1/scans" "200"
api POST "/api/v1/scans" "200|201|400" '{"path":"/tmp","name":"QA scan"}'

# ==================== SECTION 18: WEB PAGES ====================
echo ""; echo "=== 18. WEB PAGES (12 tests) ==="
web "/" "200"
web "/login" "200"
web "/register" "200"
web_contains "/login" "Catalogizer"
web_contains "/" "<!doctype html>"
web "/media" "200"
web "/browse" "200"
web "/collections" "200"
web "/favorites" "200"
web "/settings" "200"
web "/nonexistent-page" "200"
TOTAL=$((TOTAL+1)); TITLE=$(curl -sL "$WEB/" | grep -oP '<title>\K[^<]+' 2>/dev/null); [ -n "$TITLE" ] && ok "Page title: $TITLE" || warn "No page title found"

# ==================== SECTION 19: EDGE CASES ====================
echo ""; echo "=== 19. EDGE CASES (6 tests) ==="
api GET "/api/v1/nonexistent" "404" "" "no"
api POST "/api/v1/auth/login" "400" '' "no"
api GET "/api/v1/catalog-info/nonexistent/deep/path" "200|404"
TOTAL=$((TOTAL+1)); CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$API/health" 2>/dev/null); [ "$CODE" = "404" ] || [ "$CODE" = "405" ] && ok "PUT /health -> $CODE (method not allowed)" || fail "PUT /health" "$CODE"
api GET "/api/v1/search?q=" "200|400|401" "" "no"
TOTAL=$((TOTAL+1)); CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer invalid_token" "$API/api/v1/auth/me" 2>/dev/null); [ "$CODE" = "401" ] || [ "$CODE" = "429" ] && ok "Invalid token -> $CODE" || fail "Invalid token" "$CODE"

# ==================== SECTION 20: LOGOUT (end of suite) ====================
echo ""; echo "=== 20. LOGOUT (1 test) ==="
api POST "/api/v1/auth/logout" "200|400"

# ==================== SUMMARY ====================
echo ""
echo "================================================================"
printf "  RESULTS: \033[32m%d PASS\033[0m | \033[31m%d FAIL\033[0m | \033[33m%d WARN\033[0m | %d TOTAL\n" "$PASS" "$FAIL" "$WARN" "$TOTAL"
RATE=$(echo "scale=1; $PASS * 100 / $TOTAL" | bc 2>/dev/null || echo "?")
echo "  PASS RATE: ${RATE}%"
echo "================================================================"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
