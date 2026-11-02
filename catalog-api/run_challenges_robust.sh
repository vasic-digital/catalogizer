#!/bin/bash
set -e

# Read challenge IDs
if [ ! -f challenge_ids.txt ]; then
    echo "challenge_ids.txt not found"
    exit 1
fi

# Run each challenge sequentially
while read id; do
    echo "=== Running challenge: $id ==="
    # Get fresh token for each challenge
    TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"${ADMIN_PASSWORD:?set ADMIN_PASSWORD}\"}" | grep -o '"session_token":"[^"]*"' | cut -d'"' -f4)
    if [ -z "$TOKEN" ]; then
        echo "  FAILED to get token"
        exit 1
    fi
    # Run challenge with timeout (5 minutes)
    timeout 600 curl -s -H "Authorization: Bearer $TOKEN" -X POST "http://localhost:8080/api/v1/challenges/$id/run" -o "result_$id.json"
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
    sleep 3
done < challenge_ids.txt

echo "All challenges passed!"
