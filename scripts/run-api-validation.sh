#!/bin/bash
# Comprehensive API Validation Runner
# Executes real HTTP requests against the running catalog-api and validates responses
# Usage: ./scripts/run-api-validation.sh [API_URL]

set -euo pipefail
ADMIN_PASSWORD="${ADMIN_PASSWORD:?set ADMIN_PASSWORD to the test-stack admin password (from your .env; never hardcoded)}"

API_URL="${1:-http://localhost:8085}"
PASS=0; FAIL=0; TOTAL=0
FAILURES=""

check() {
    local name="$1" method="$2" path="$3" expected_code="$4" body="${5:-}" token="${6:-}"
    TOTAL=$((TOTAL+1))

    local args=(-s -o /tmp/api_response.txt -w "%{http_code}" -X "$method")
    [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
    args+=(-H "Content-Type: application/json")
    [ -n "$body" ] && args+=(-d "$body")
    args+=("${API_URL}${path}")

    local code
    code=$(curl "${args[@]}" 2>/dev/null || echo "000")

    if [ "$code" = "$expected_code" ]; then
        PASS=$((PASS+1))
        printf "  \033[32mPASS\033[0m  %-45s %s -> %s\n" "$name" "$method $path" "$code"
    else
        FAIL=$((FAIL+1))
        FAILURES="$FAILURES\n  FAIL: $name ($method $path) expected=$expected_code got=$code"
        printf "  \033[31mFAIL\033[0m  %-45s %s -> %s (expected %s)\n" "$name" "$method $path" "$code" "$expected_code"
    fi
}

check_contains() {
    local name="$1" method="$2" path="$3" substring="$4" token="${5:-}"
    TOTAL=$((TOTAL+1))

    local args=(-s -X "$method")
    [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
    args+=(-H "Content-Type: application/json")
    args+=("${API_URL}${path}")

    local response
    response=$(curl "${args[@]}" 2>/dev/null || echo "")

    if echo "$response" | grep -q "$substring"; then
        PASS=$((PASS+1))
        printf "  \033[32mPASS\033[0m  %-45s contains '%s'\n" "$name" "$substring"
    else
        FAIL=$((FAIL+1))
        FAILURES="$FAILURES\n  FAIL: $name - response does not contain '$substring'"
        printf "  \033[31mFAIL\033[0m  %-45s missing '%s'\n" "$name" "$substring"
    fi
}

echo "============================================"
echo " Catalogizer API Validation Suite"
echo " Target: $API_URL"
echo " Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "============================================"
echo ""

# --- Phase 1: Health & Metrics ---
echo "--- Phase 1: Health & Metrics ---"
check "Health endpoint" GET "/health" "200"
check_contains "Health returns healthy" GET "/health" "healthy"
check_contains "Metrics endpoint" GET "/metrics" "go_goroutines"
check_contains "Health has version" GET "/health" "version"

# --- Phase 2: Authentication ---
echo ""
echo "--- Phase 2: Authentication ---"
check "Login with valid creds" POST "/api/v1/auth/login" "200" "{\"username\":\"admin\",\"password\":\"${ADMIN_PASSWORD}\"}"
TOKEN=$(curl -s -X POST -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"${ADMIN_PASSWORD}\"}" "${API_URL}/api/v1/auth/login" | python3 -c "import sys,json; print(json.load(sys.stdin).get('session_token',''))" 2>/dev/null || echo "")

if [ -z "$TOKEN" ]; then
    echo "  FATAL: Could not obtain auth token, aborting"
    exit 1
fi

# Test with token first (before rate limit kicks in from failed logins)
check "Auth me (with token)" GET "/api/v1/auth/me" "200" "" "$TOKEN"
check "Auth permissions" GET "/api/v1/auth/permissions" "200" "" "$TOKEN"
check "Auth status (public)" GET "/api/v1/auth/status" "200"
# These may return 429 if rate-limited from prior test runs; both 401 and 429 are acceptable
TOTAL=$((TOTAL+1)); code=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/api/v1/auth/me"); if [ "$code" = "401" ] || [ "$code" = "429" ]; then PASS=$((PASS+1)); printf "  \033[32mPASS\033[0m  %-45s %s\n" "Auth me (no token)" "$code"; else FAIL=$((FAIL+1)); printf "  \033[31mFAIL\033[0m  %-45s %s\n" "Auth me (no token)" "$code"; fi
TOTAL=$((TOTAL+1)); code=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" -d '{}' "${API_URL}/api/v1/auth/login"); if [ "$code" = "401" ] || [ "$code" = "429" ]; then PASS=$((PASS+1)); printf "  \033[32mPASS\033[0m  %-45s %s\n" "Login with empty body" "$code"; else FAIL=$((FAIL+1)); printf "  \033[31mFAIL\033[0m  %-45s %s\n" "Login with empty body" "$code"; fi
check "Login with malformed JSON" POST "/api/v1/auth/login" "400" 'not-json'

# --- Phase 3: Security ---
echo ""
echo "--- Phase 3: Security ---"
check "Protected: catalog (no auth)" GET "/api/v1/catalog" "401"
check "Protected: search (no auth)" GET "/api/v1/search" "401"
check "Protected: storage-roots (no auth)" GET "/api/v1/storage-roots" "401"
check "Protected: media/stats (no auth)" GET "/api/v1/media/stats" "401"
check "Protected: challenges (no auth)" GET "/api/v1/challenges" "401"

# --- Phase 4: Core API Endpoints ---
echo ""
echo "--- Phase 4: Core API Endpoints ---"
check "Catalog root" GET "/api/v1/catalog" "200" "" "$TOKEN"
check "Search files" GET "/api/v1/search/files?q=test" "200" "" "$TOKEN"
check "Search duplicates" GET "/api/v1/search/files/duplicates?q=test" "200" "" "$TOKEN"
check "Storage roots" GET "/api/v1/storage-roots" "200" "" "$TOKEN"
check "Media search" GET "/api/v1/media/search?q=test" "200" "" "$TOKEN"
check "Media stats" GET "/api/v1/media/stats" "200" "" "$TOKEN"
check "Challenges list" GET "/api/v1/challenges" "200" "" "$TOKEN"
check_contains "283 challenges" GET "/api/v1/challenges" "283" "$TOKEN"
check "Discovery" GET "/api/v1/discovery" "200" "" "$TOKEN"
check "Configuration" GET "/api/v1/configuration" "200" "" "$TOKEN" || true

# --- Phase 5: Content Types & Headers ---
echo ""
echo "--- Phase 5: Response Quality ---"
# Content-Type check via headers
TOTAL=$((TOTAL+1))
CT=$(curl -s -o /dev/null -w "%{content_type}" http://localhost:8085/health 2>/dev/null)
if echo "$CT" | grep -q "application/json"; then
    PASS=$((PASS+1))
    printf "  \033[32mPASS\033[0m  %-45s Content-Type: %s\n" "JSON content type" "$CT"
else
    FAIL=$((FAIL+1))
    printf "  \033[31mFAIL\033[0m  %-45s Content-Type: %s\n" "JSON content type" "$CT"
fi
check_contains "Health has build_date" GET "/health" "build_date"
check_contains "Health has build_number" GET "/health" "build_number"

# --- Phase 6: Error Handling ---
echo ""
echo "--- Phase 6: Error Handling ---"
check "404 for unknown route" GET "/api/v1/nonexistent" "404"
check "Invalid challenge ID" POST "/api/v1/challenges/nonexistent-id/run" "500" '{}' "$TOKEN"

# --- Summary ---
echo ""
echo "============================================"
printf " Results: \033[32m%d passed\033[0m, \033[31m%d failed\033[0m / %d total\n" "$PASS" "$FAIL" "$TOTAL"
if [ "$FAIL" -gt 0 ]; then
    echo ""
    echo " Failures:"
    echo -e "$FAILURES"
fi
echo "============================================"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
