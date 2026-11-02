#!/bin/bash
set -e
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD (no default; CAT-021)}"
LOGIN_BODY=$(jq -n --arg u admin --arg p "$ADMIN_PASSWORD" '{username:$u,password:$p}')

# Read challenge IDs
if [ ! -f challenge_ids.txt ]; then
    echo "challenge_ids.txt not found"
    exit 1
fi

# Skip first N challenges (already passed)
SKIP=6
COUNT=0

while read id; do
    COUNT=$((COUNT + 1))
    if [ $COUNT -le $SKIP ]; then
        echo "Skipping challenge $COUNT: $id"
        continue
    fi
    echo "=== Running challenge $COUNT: $id ==="
    # Get fresh token for each challenge
    TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "$LOGIN_BODY" | jq -r '.session_token')
    if [ -z "$TOKEN" ]; then
        echo "  FAILED to get token"
        exit 1
    fi
    # Run challenge with timeout (20 minutes)
    timeout 1200 curl -s -H "Authorization: Bearer $TOKEN" -X POST "http://localhost:8080/api/v1/challenges/$id/run" -o "result_$id.json"
    if [ $? -eq 124 ]; then
        echo "  TIMEOUT after 10 minutes"
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
    sleep 3
done < challenge_ids.txt

echo "All challenges passed!"