#!/usr/bin/env bash
# Start PostgreSQL container for Catalogizer development.
#
# Usage: ./scripts/postgres-up.sh
#
# Environment variables (optional overrides):
#   POSTGRES_USER     (default: catalogizer)
#   POSTGRES_PASSWORD (default: catalogizer_dev)
#   POSTGRES_DB       (default: catalogizer)
#   POSTGRES_PORT     (default: 5433)

set -euo pipefail

CONTAINER_NAME="catalogizer-postgres"
PG_USER="${POSTGRES_USER:-catalogizer}"
PG_PASS="${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD (see .env.example)}"
PG_DB="${POSTGRES_DB:-catalogizer}"
PG_PORT="${POSTGRES_PORT:-5433}"

# Check if already running
if podman ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "PostgreSQL container '${CONTAINER_NAME}' is already running."
    exit 0
fi

# Remove stopped container if it exists
if podman ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Removing stopped container '${CONTAINER_NAME}'..."
    podman rm "${CONTAINER_NAME}" >/dev/null
fi

echo "Starting PostgreSQL container on port ${PG_PORT}..."
podman run -d \
    --name "${CONTAINER_NAME}" \
    -p "${PG_PORT}:5432" \
    -e POSTGRES_USER="${PG_USER}" \
    -e POSTGRES_PASSWORD="${PG_PASS}" \
    -e POSTGRES_DB="${PG_DB}" \
    -v catalogizer-pgdata:/var/lib/postgresql/data \
    docker.io/library/postgres:15-alpine

echo "Waiting for PostgreSQL to be ready..."
for i in $(seq 1 30); do
    if podman exec "${CONTAINER_NAME}" pg_isready -U "${PG_USER}" -d "${PG_DB}" >/dev/null 2>&1; then
        echo "PostgreSQL is ready."
        echo ""
        echo "Connection details:"
        echo "  Host:     localhost"
        echo "  Port:     ${PG_PORT}"
        echo "  Database: ${PG_DB}"
        echo "  User:     ${PG_USER}"
        echo "  Password: ${PG_PASS}"
        exit 0
    fi
    sleep 1
done

echo "ERROR: PostgreSQL did not become ready in 30 seconds."
exit 1
