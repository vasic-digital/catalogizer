#!/usr/bin/env bash
# Start the Catalogizer API service in a Podman container.
# Builds the image if it doesn't exist, then runs with --network host.
#
# Usage: ./scripts/api-up.sh [--rebuild]

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONTAINER_NAME="catalogizer-api"
IMAGE_NAME="catalogizer-api"

# Default environment variables (override via export before running)
JWT_SECRET="${JWT_SECRET:?set JWT_SECRET (see .env.example; scripts/install.sh generates one)}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:?set ADMIN_PASSWORD (see .env.example)}"
GIN_MODE="${GIN_MODE:-release}"

# Database configuration (PostgreSQL by default)
DATABASE_TYPE="${DATABASE_TYPE:-postgres}"
DATABASE_HOST="${DATABASE_HOST:-localhost}"
DATABASE_PORT="${DATABASE_PORT:-5433}"
DATABASE_NAME="${DATABASE_NAME:-catalogizer}"
DATABASE_USER="${DATABASE_USER:-catalogizer}"
DATABASE_PASSWORD="${DATABASE_PASSWORD:?set DATABASE_PASSWORD (see .env.example)}"
DATABASE_SSL_MODE="${DATABASE_SSL_MODE:-disable}"

REBUILD=false
if [[ "${1:-}" == "--rebuild" ]]; then
    REBUILD=true
fi

# Check if already running
if podman ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "[api] Container '${CONTAINER_NAME}' is already running."
    podman ps --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    exit 0
fi

# Remove stopped container if exists
if podman ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "[api] Removing stopped container '${CONTAINER_NAME}'..."
    podman rm "${CONTAINER_NAME}" >/dev/null
fi

# Build image if it doesn't exist or --rebuild was passed
if [[ "$REBUILD" == "true" ]] || ! podman image exists "${IMAGE_NAME}"; then
    echo "[api] Building image '${IMAGE_NAME}'..."
    podman build --network host \
        -f "${PROJECT_ROOT}/catalog-api/Dockerfile" \
        -t "${IMAGE_NAME}" \
        "${PROJECT_ROOT}" 2>&1 | tail -5
    echo "[api] Image built successfully."
fi

# Start container
echo "[api] Starting '${CONTAINER_NAME}' on port 8080..."
podman run -d \
    --name "${CONTAINER_NAME}" \
    --network host \
    --add-host synology.local:192.168.0.241 \
    -e "JWT_SECRET=${JWT_SECRET}" \
    -e "ADMIN_USERNAME=${ADMIN_USERNAME}" \
    -e "ADMIN_PASSWORD=${ADMIN_PASSWORD}" \
    -e "GIN_MODE=${GIN_MODE}" \
    -e "DATABASE_TYPE=${DATABASE_TYPE}" \
    -e "DATABASE_HOST=${DATABASE_HOST}" \
    -e "DATABASE_PORT=${DATABASE_PORT}" \
    -e "DATABASE_NAME=${DATABASE_NAME}" \
    -e "DATABASE_USER=${DATABASE_USER}" \
    -e "DATABASE_PASSWORD=${DATABASE_PASSWORD}" \
    -e "DATABASE_SSL_MODE=${DATABASE_SSL_MODE}" \
    "${IMAGE_NAME}" >/dev/null

# Wait for health
echo -n "[api] Waiting for health check"
for i in $(seq 1 15); do
    if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
        echo " OK"
        echo "[api] Catalogizer API is running at http://localhost:8080"
        exit 0
    fi
    echo -n "."
    sleep 1
done

echo " TIMEOUT"
echo "[api] Warning: Health check timed out. Checking logs..."
podman logs --tail 10 "${CONTAINER_NAME}" 2>&1
exit 1
