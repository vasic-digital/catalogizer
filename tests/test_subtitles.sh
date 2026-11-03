#!/bin/bash

# Test script for subtitle endpoints
# Usage: BASE_URL=http://127.0.0.1:8080 ADMIN_PASSWORD=... ./tests/test_subtitles.sh
#
# Registration policy: under the default REGISTRATION_MODE=approval a self-registered account is
# created inactive and cannot log in (HTTP 403) until an admin approves it. This script
# registers its test user if needed and, when the account comes back pending, approves it
# through the admin API using ADMIN_PASSWORD (ADMIN_USERNAME defaults to admin). Without
# ADMIN_PASSWORD it fails with an explicit message instead of continuing unauthenticated.

BASE_URL="${BASE_URL:-http://localhost:8080}"
USERNAME="${QA_TEST_USERNAME:-testuser}"
# The server requires >= 8 characters (the former "testpass" was rejected with 400).
PASSWORD="${QA_TEST_PASSWORD:-TestPass123!x}"

# shellcheck source=../scripts/lib/qa_registration.sh
. "$(cd "$(dirname "$0")" && pwd)/../scripts/lib/qa_registration.sh"

echo "Testing subtitle endpoints..."

# First, get a JWT token by logging in
echo "1. Logging in to get JWT token..."
LOGIN_REQUEST='{
  "username": "'$USERNAME'",
  "password": "'$PASSWORD'"
}'
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "$LOGIN_REQUEST")

echo "Login response: $LOGIN_RESPONSE"

# Extract token from response
TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.session_token // .token // empty')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "Failed to get authentication token. Registering the test user (and approving it if pending)..."
  if [ -z "${ADMIN_PASSWORD:-}" ]; then
    echo "ERROR: ADMIN_PASSWORD is not set, so a pending self-registered account cannot be approved." >&2
    echo "       Set ADMIN_PASSWORD, or start the API with REGISTRATION_MODE=open." >&2
    exit 1
  fi
  ADMIN_CODE=$(qa_curl_code -X POST -H "Content-Type: application/json" \
    -d "{\"username\":\"${ADMIN_USERNAME:-admin}\",\"password\":\"$ADMIN_PASSWORD\"}" "$BASE_URL/api/v1/auth/login")
  ADMIN_TOKEN=$(jq -r '.session_token // empty' "$QA_BODY_FILE")
  if [ "$ADMIN_CODE" != "200" ] || [ -z "$ADMIN_TOKEN" ]; then
    echo "ERROR: admin login failed (HTTP $ADMIN_CODE)" >&2
    exit 1
  fi
  if ! qa_register_and_activate "$BASE_URL" "$ADMIN_TOKEN" "$USERNAME" "${USERNAME}@example.invalid" "$PASSWORD" >/dev/null; then
    echo "ERROR: could not register + activate $USERNAME (it may already exist inactive; approve it or use another QA_TEST_USERNAME)" >&2
    exit 1
  fi

  LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\": \"$USERNAME\", \"password\": \"$PASSWORD\"}")
  echo "Second login response: $LOGIN_RESPONSE"
  TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.session_token // .token // empty')
fi

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "ERROR: Could not get authentication token"
  exit 1
fi

echo "Authentication successful. Token: ${TOKEN:0:50}..."

# Set up authorization header
AUTH_HEADER="Authorization: Bearer $TOKEN"

# Test subtitle endpoints
echo ""
echo "2. Testing GET /api/v1/subtitles/languages"
RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/subtitles/languages" -H "$AUTH_HEADER")
echo "Response: $RESPONSE"
echo ""

echo "3. Testing GET /api/v1/subtitles/providers"
RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/subtitles/providers" -H "$AUTH_HEADER")
echo "Response: $RESPONSE"
echo ""

echo "4. Testing GET /api/v1/subtitles/search?media_path=/test/movie.mp4&languages=en,es"
RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/subtitles/search?media_path=/test/movie.mp4&languages=en,es" -H "$AUTH_HEADER")
echo "Response: $RESPONSE"
echo ""

echo "5. Testing GET /api/v1/subtitles/media/123"
RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/subtitles/media/123" -H "$AUTH_HEADER")
echo "Response: $RESPONSE"
echo ""

echo "6. Testing POST /api/v1/subtitles/download"
DOWNLOAD_REQUEST='{
  "media_item_id": 123,
  "result_id": "os_1",
  "language": "en",
  "verify_sync": true
}'
RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/subtitles/download" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d "$DOWNLOAD_REQUEST")
echo "Response: $RESPONSE"
echo ""

echo "7. Testing POST /api/v1/subtitles/translate"
TRANSLATE_REQUEST='{
  "subtitle_id": "sub_1234567890",
  "source_language": "en",
  "target_language": "es",
  "use_cache": true
}'
RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/subtitles/translate" \
  -H "Content-Type: application/json" \
  -H "$AUTH_HEADER" \
  -d "$TRANSLATE_REQUEST")
echo "Response: $RESPONSE"
echo ""

echo "8. Testing GET /api/v1/subtitles/sub_test_123/verify-sync/123"
RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/subtitles/sub_test_123/verify-sync/123" -H "$AUTH_HEADER")
echo "Response: $RESPONSE"
echo ""

echo "All subtitle endpoint tests completed!"