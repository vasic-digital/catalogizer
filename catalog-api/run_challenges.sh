#!/bin/bash
set -e
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD (no default; CAT-021)}"
LOGIN_BODY=$(jq -n --arg u admin --arg p "$ADMIN_PASSWORD" '{username:$u,password:$p}')

# Get server port from .service-port file, default 8080
get_server_port() {
    if [ -f ".service-port" ]; then
        cat ".service-port"
    else
        echo "8080"
    fi
}
API_PORT=$(get_server_port)
API_BASE="http://localhost:$API_PORT"

# Get JWT token using jq (more robust)
RESP=$(curl -s -X POST $API_BASE/api/v1/auth/login -H "Content-Type: application/json" -d @- <<<"$LOGIN_BODY")
TOKEN=$(echo "$RESP" | jq -r '.session_token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "Failed to extract token from response: $RESP"
    exit 1
fi
echo "Token obtained"
sleep 2

# Read challenge IDs
if [ ! -f challenge_ids.txt ]; then
    echo "challenge_ids.txt not found"
    exit 1
fi

# Run each challenge sequentially
while read id; do
    echo "=== Running challenge: $id ==="
    # Run challenge with timeout (10 minutes)
    timeout 600 curl -s -H "Authorization: Bearer $TOKEN" -X POST "$API_BASE/api/v1/challenges/$id/run" -o "result_$id.json"
    if [ $? -eq 124 ]; then
        echo "  TIMEOUT after 5 minutes"
        exit 1
    fi
    # Check if result file exists
    if [ ! -f "result_$id.json" ]; then
        echo "  No result file"
        exit 1
    fi
    # Parse status
    status=$(jq -r '.data.status' "result_$id.json")
    echo "  Status: $status"
    if [ "$status" != "passed" ]; then
        echo "  FAILED - see result_$id.json"
        exit 1
    fi
    echo "  PASSED"
    # Small delay between challenges
    sleep 10
done < challenge_ids.txt

echo "All challenges passed!"