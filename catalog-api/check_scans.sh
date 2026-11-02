#!/bin/bash
set -e
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"admin\",\"password\":\"${ADMIN_PASSWORD:?set ADMIN_PASSWORD}\"}" | jq -r '.session_token')
echo "Token: $TOKEN"
echo "=== Scans ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/scans | jq .
echo "=== Stats ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/stats/overall | jq .
echo "=== Storage roots ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/storage/roots | jq .