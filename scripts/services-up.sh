#!/usr/bin/env bash
# Start all Catalogizer services (API + Web).
# Services are started in dependency order: API first, then Web.
#
# Usage: ./scripts/services-up.sh [--rebuild]
#
# Options:
#   --rebuild   Force rebuild of container images before starting

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REBUILD_FLAG=""

if [[ "${1:-}" == "--rebuild" ]]; then
    REBUILD_FLAG="--rebuild"
fi

echo "=== Starting Catalogizer Services ==="
echo ""

# Start PostgreSQL (database must be ready before API)
"${SCRIPT_DIR}/postgres-up.sh"
echo ""

# Start API (web app depends on it)
"${SCRIPT_DIR}/api-up.sh" ${REBUILD_FLAG}
echo ""

# Start Web frontend
"${SCRIPT_DIR}/web-up.sh"
echo ""

echo "=== All Services Running ==="
echo "  API: http://localhost:8080"
echo "  Web: http://localhost:3000"
echo ""
echo "Login with: admin / (ADMIN_PASSWORD from your .env)"
