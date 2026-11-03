#!/bin/bash
# Recommendation endpoint checks against a RUNNING catalog-api (no server is started here and
# no mock data is involved).
#
# CAT-001: the API has no recommendation engine and no trending source, so these routes answer
# an HONEST 501 instead of fabricated titles. This script pins exactly that, so a regression to
# an invented 200 FAILs, and so does a 404/500. It exits non-zero if any check fails.
#
# Usage:
#   API_URL=http://127.0.0.1:8080 ADMIN_PASSWORD=... ./tests/test_recommendations.sh
#   (ADMIN_USERNAME defaults to admin; or pass a pre-acquired token in CATALOGIZER_TOKEN)
set -uo pipefail

API_URL="${API_URL:?set API_URL to the running catalog-api (e.g. http://127.0.0.1:8080)}"
USERNAME="${ADMIN_USERNAME:-admin}"
FAILS=0

# shellcheck source=../scripts/lib/qa_registration.sh
. "$(cd "$(dirname "$0")" && pwd)/../scripts/lib/qa_registration.sh"

TOKEN="${CATALOGIZER_TOKEN:-}"
if [ -z "$TOKEN" ]; then
    : "${ADMIN_PASSWORD:?set ADMIN_PASSWORD (or CATALOGIZER_TOKEN)}"
    code=$(qa_curl_code -X POST -H 'Content-Type: application/json' \
        -d "{\"username\":\"${USERNAME}\",\"password\":\"${ADMIN_PASSWORD}\"}" "${API_URL}/api/v1/auth/login")
    [ "$code" = "200" ] || { echo "login failed: HTTP $code" >&2; exit 1; }
    TOKEN=$(python3 -c 'import sys,json; print(json.load(open(sys.argv[1])).get("session_token",""))' "$QA_BODY_FILE")
fi
[ -n "$TOKEN" ] || { echo "no session token" >&2; exit 1; }
ME_ID=$(qa_curl_code -H "Authorization: Bearer ${TOKEN}" "${API_URL}/api/v1/auth/me" >/dev/null; \
        python3 -c 'import sys,json; print(json.load(open(sys.argv[1])).get("id",""))' "$QA_BODY_FILE")

check() { # name path expected-code
    local name="$1" path="$2" want="$3" got
    got=$(qa_curl_code -H "Authorization: Bearer ${TOKEN}" "${API_URL}${path}")
    if [ "$got" = "$want" ]; then
        echo "PASS: ${name}: GET ${path} -> ${got}"
    else
        echo "FAIL: ${name}: GET ${path} expected ${want}, got ${got}"
        FAILS=$((FAILS + 1))
    fi
}

echo "Testing recommendation endpoints against ${API_URL}"
check "root alias"            "/api/v1/recommendations"                    501
check "trending"              "/api/v1/recommendations/trending"           501
check "by-type"               "/api/v1/recommendations/by-type"            501
check "personalized (own id)" "/api/v1/recommendations/personalized/${ME_ID:-0}" 501
check "personalized (bad id)" "/api/v1/recommendations/personalized/not-a-number" 400

# 501 must carry an error body and no fabricated items.
qa_curl_code -H "Authorization: Bearer ${TOKEN}" "${API_URL}/api/v1/recommendations/trending" >/dev/null
if grep -q '"error"' "$QA_BODY_FILE" && ! grep -q '"items"' "$QA_BODY_FILE"; then
    echo "PASS: trending 501 body has an error and no items[]"
else
    echo "FAIL: trending 501 body must carry an error and no items[]: $(head -c 200 "$QA_BODY_FILE")"
    FAILS=$((FAILS + 1))
fi

rm -f "$QA_BODY_FILE"
echo "Recommendation endpoint tests completed: ${FAILS} failure(s)"
[ "$FAILS" -eq 0 ]
