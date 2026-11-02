#!/bin/bash
set -e
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD (no default; CAT-021)}"
LOGIN_BODY=$(jq -n --arg u admin --arg p "$ADMIN_PASSWORD" '{username:$u,password:$p}')
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "$LOGIN_BODY" | jq -r '.session_token')
echo "Token: $TOKEN"
echo "=== Scans ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/scans | jq .
echo "=== Stats ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/stats/overall | jq .
echo "=== Storage roots ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/storage/roots | jq .