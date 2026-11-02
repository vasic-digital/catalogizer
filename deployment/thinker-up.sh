#!/usr/bin/env bash
# Bring up the catalogizer stack on thinker.local (rootless Podman).
#
# Reads deployment/thinker.local.env and launches postgres, redis, and
# catalog-api with the dev overlay image. Idempotent: recreates containers
# if they already exist.
#
# Run locally OR via:
#   ssh thinker.local bash < deployment/thinker-up.sh
#
# No sudo, no root. All ports bound to 127.0.0.1 for safety.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/thinker.local.env"

if [[ -f "${ENV_FILE}" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a
fi

: "${CZ_THINKER_NETWORK:=catalogizer-thinker}"
: "${CZ_THINKER_POSTGRES_PORT:=5445}"
: "${CZ_THINKER_REDIS_PORT:=6391}"
: "${CZ_THINKER_API_PORT:=8092}"
: "${CZ_THINKER_POSTGRES_NAME:=cz-postgres-thinker}"
: "${CZ_THINKER_REDIS_NAME:=cz-redis-thinker}"
: "${CZ_THINKER_API_NAME:=cz-api-thinker}"
: "${CZ_THINKER_POSTGRES_CPUS:=1}"
: "${CZ_THINKER_POSTGRES_MEMORY:=2g}"
: "${CZ_THINKER_REDIS_CPUS:=1}"
: "${CZ_THINKER_REDIS_MEMORY:=1g}"
: "${CZ_THINKER_API_CPUS:=2}"
: "${CZ_THINKER_API_MEMORY:=4g}"
: "${POSTGRES_USER:=catalogizer}"
: "${POSTGRES_PASSWORD:=catalogizer}"
: "${POSTGRES_DB:=catalogizer}"
: "${JWT_SECRET:=dev-secret-for-operational-run-needs-32-plus-chars-to-be-valid}"
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD (no default; CAT-021)}"
: "${GIN_MODE:=release}"
: "${DB_TYPE:=postgres}"

API_IMAGE="${API_IMAGE:-localhost/catalogizer-api:latest}"
API_CONFIG="${API_CONFIG:-/tmp/catalogizer-run-config.json}"

echo "==> Target host: $(hostname)"
echo "==> API image: ${API_IMAGE}"
echo "==> Network: ${CZ_THINKER_NETWORK}"

podman network inspect "${CZ_THINKER_NETWORK}" >/dev/null 2>&1 \
    || podman network create "${CZ_THINKER_NETWORK}"

for name in "${CZ_THINKER_WEB_NAME:-cz-web-thinker}" "${CZ_THINKER_API_NAME}" "${CZ_THINKER_REDIS_NAME}" "${CZ_THINKER_POSTGRES_NAME}"; do
    if podman ps -a --format '{{.Names}}' | grep -qx "${name}"; then
        echo "==> removing stale ${name}"
        podman rm -f "${name}" >/dev/null
    fi
done

: "${CZ_THINKER_WEB_PORT:=3092}"
: "${CZ_THINKER_WEB_NAME:=cz-web-thinker}"
: "${CZ_THINKER_WEB_CPUS:=1}"
: "${CZ_THINKER_WEB_MEMORY:=1g}"
: "${ADMIN_USERNAME:=admin}"
WEB_IMAGE="${WEB_IMAGE:-localhost/catalogizer-web:latest}"

# Linger keeps rootless containers alive after SSH disconnects
loginctl enable-linger 2>/dev/null || true

echo "==> starting postgres"
podman run -d \
    --name "${CZ_THINKER_POSTGRES_NAME}" \
    --network "${CZ_THINKER_NETWORK}" \
    -e POSTGRES_USER="${POSTGRES_USER}" \
    -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
    -e POSTGRES_DB="${POSTGRES_DB}" \
    -p "127.0.0.1:${CZ_THINKER_POSTGRES_PORT}:5432" \
    --cpus="${CZ_THINKER_POSTGRES_CPUS}" \
    --memory="${CZ_THINKER_POSTGRES_MEMORY}" \
    docker.io/library/postgres:16-alpine >/dev/null

echo "==> waiting for postgres to accept connections"
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    if podman exec "${CZ_THINKER_POSTGRES_NAME}" pg_isready -U "${POSTGRES_USER}" >/dev/null 2>&1; then
        echo "==> postgres ready (after ${i}s)"
        break
    fi
    sleep 1
done

echo "==> starting redis"
podman run -d \
    --name "${CZ_THINKER_REDIS_NAME}" \
    --network "${CZ_THINKER_NETWORK}" \
    -p "127.0.0.1:${CZ_THINKER_REDIS_PORT}:6379" \
    --cpus="${CZ_THINKER_REDIS_CPUS}" \
    --memory="${CZ_THINKER_REDIS_MEMORY}" \
    docker.io/library/redis:7-alpine >/dev/null

MOUNT_ARGS=()
if [[ -f "${API_CONFIG}" ]]; then
    MOUNT_ARGS+=(-v "${API_CONFIG}:/app/config.json:ro")
fi

echo "==> starting catalog-api"
podman run -d \
    --name "${CZ_THINKER_API_NAME}" \
    --network "${CZ_THINKER_NETWORK}" \
    -p "127.0.0.1:${CZ_THINKER_API_PORT}:8080" \
    --cpus="${CZ_THINKER_API_CPUS}" \
    --memory="${CZ_THINKER_API_MEMORY}" \
    "${MOUNT_ARGS[@]}" \
    -e DATABASE_HOST="${CZ_THINKER_POSTGRES_NAME}" \
    -e DATABASE_PORT=5432 \
    -e DATABASE_NAME="${POSTGRES_DB}" \
    -e DATABASE_USER="${POSTGRES_USER}" \
    -e DATABASE_PASSWORD="${POSTGRES_PASSWORD}" \
    -e REDIS_HOST="${CZ_THINKER_REDIS_NAME}" \
    -e REDIS_PORT=6379 \
    -e JWT_SECRET="${JWT_SECRET}" \
    -e ADMIN_USERNAME="${ADMIN_USERNAME}" \
    -e ADMIN_PASSWORD="${ADMIN_PASSWORD}" \
    -e SERVER_PORT=8080 \
    -e HOST=0.0.0.0 \
    -e GIN_MODE="${GIN_MODE}" \
    -e DB_TYPE="${DB_TYPE}" \
    "${API_IMAGE}" >/dev/null

echo "==> starting catalog-web"
podman run -d \
    --name "${CZ_THINKER_WEB_NAME}" \
    --network "${CZ_THINKER_NETWORK}" \
    -p "127.0.0.1:${CZ_THINKER_WEB_PORT}:3000" \
    --cpus="${CZ_THINKER_WEB_CPUS}" \
    --memory="${CZ_THINKER_WEB_MEMORY}" \
    --add-host host.containers.internal:host-gateway \
    "${WEB_IMAGE}" >/dev/null

echo "==> waiting for /health"
api_ok=0
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do
    if curl -sf "http://127.0.0.1:${CZ_THINKER_API_PORT}/health" >/dev/null 2>&1; then
        echo "==> catalog-api healthy on http://127.0.0.1:${CZ_THINKER_API_PORT} (after ${i}s)"
        api_ok=1
        break
    fi
    sleep 1
done

web_ok=0
for i in 1 2 3 4 5 6 7 8 9 10; do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${CZ_THINKER_WEB_PORT}/" 2>/dev/null || true)
    if [[ "$code" == "200" ]]; then
        echo "==> catalog-web serving on http://127.0.0.1:${CZ_THINKER_WEB_PORT} (after ${i}s)"
        web_ok=1
        break
    fi
    sleep 1
done

if [[ "$api_ok" == "1" && "$web_ok" == "1" ]]; then
    exit 0
fi
[[ "$api_ok" != "1" ]] && { echo "!! catalog-api unhealthy"; podman logs --tail 30 "${CZ_THINKER_API_NAME}" || true; }
[[ "$web_ok" != "1" ]] && { echo "!! catalog-web not serving"; podman logs --tail 30 "${CZ_THINKER_WEB_NAME}" || true; }
exit 1
