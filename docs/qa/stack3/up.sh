#!/usr/bin/env bash
# Bring up Catalogizer QA stack 3 (idempotent: replaces its own pod, keeps data volumes).
# usage: up.sh [image-tag]     default tag = main-<short8 of $QA_DIR/BUILT_COMMIT>, written by build.sh
#
# Design (why not podman-compose):
#  * ONE POD per stack on the `pasta` network: all containers share a loopback, so postgres/redis/api talk over
#    127.0.0.1 - NO service-name DNS, NO bridge, NO aardvark-dns, NO shared rootless-netns. A broken/split rootless
#    netns or a dead aardvark-dns (see README) cannot affect it. Ports are published on the pod via pasta.
#  * START ORDER IS HEALTH-GATED: postgres + redis (with healthchecks) -> wait healthy -> api (healthcheck) -> wait
#    healthy -> web + website. This is depends_on:condition=service_healthy done explicitly, because podman-compose
#    ignores that condition (the API used to lose the race with Postgres and exit 1).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
mkdir -p "$QA_DIR"; chmod 700 "$QA_DIR"
ENVF="$QA_DIR/qa.env"
if [ ! -f "$ENVF" ]; then   # fresh random secrets, never printed
  ( umask 077; r() { head -c "$1" /dev/urandom | od -An -vtx1 | tr -d ' \n'; }
    { echo "ADMIN_USERNAME=qa_admin"; echo "ADMIN_PASSWORD=Qa-$(r 12)-Zz9"; echo "JWT_SECRET=$(r 32)"
      echo "POSTGRES_USER=catalogizer"; echo "POSTGRES_PASSWORD=$(r 16)"; echo "POSTGRES_DB=catalogizer"
      echo "REGISTRATION_MODE=approval"; } > "$ENVF" )
  echo "generated new $ENVF (0600)"
fi
[ "$(stat -c %a "$ENVF")" = 600 ] || chmod 600 "$ENVF"
set -a; . "$ENVF"; set +a
TAG="${1:-main-$(cut -c1-8 "$QA_DIR/BUILT_COMMIT" 2>/dev/null || echo unknown)}"
for i in "localhost/${STACK}-api:$TAG" "localhost/${STACK}-web:$TAG"; do
  podman image exists "$i" || { echo "image $i missing - run build.sh first" >&2; exit 3; }
done
[ -d "$MEDIA_DIR" ] || { echo "media corpus $MEDIA_DIR missing - run: scripts/test-infra/gen_media_fixtures.sh --out $MEDIA_DIR" >&2; exit 3; }
avail_kb=$(awk '/MemAvailable/{print $2}' /proc/meminfo)
[ "$avail_kb" -gt 6000000 ] || { echo "only $((avail_kb/1024)) MiB available (<6 GiB) - refusing (host safety)" >&2; exit 4; }
for p in "$API_PORT" "$WEB_PORT" "$WEBSITE_PORT" "$PG_PORT" "$REDIS_PORT"; do   # ports must be free unless our own pod holds them
  if ss -ltnH "sport = :$p" | grep -q . && ! podman pod exists "$POD"; then echo "port $p already in use" >&2; exit 5; fi
done
podman pod rm -f "$POD" >/dev/null 2>&1 || true
podman pod create --name "$POD" --network pasta \
  -p "0.0.0.0:${API_PORT}:8080" -p "0.0.0.0:${WEB_PORT}:3000" -p "0.0.0.0:${WEBSITE_PORT}:80" \
  -p "127.0.0.1:${PG_PORT}:5432" -p "127.0.0.1:${REDIS_PORT}:6379" >/dev/null

wait_healthy() {  # <container> <timeout-s>
  local c="$1" t="$2" i
  for i in $(seq 1 "$t"); do
    [ "$(podman inspect "$c" --format '{{.State.Health.Status}}' 2>/dev/null)" = healthy ] && { echo "$c healthy after ${i}s"; return 0; }
    [ "$(podman inspect "$c" --format '{{.State.Status}}' 2>/dev/null)" = running ] || { echo "$c is not running" >&2; podman logs --tail 20 "$c" >&2 || true; return 1; }
    sleep 1; done
  echo "$c not healthy after ${t}s" >&2; podman logs --tail 20 "$c" >&2 || true; return 1; }

podman run -d --pod "$POD" --name "${STACK}-postgres" --memory 1g \
  -e POSTGRES_USER -e POSTGRES_PASSWORD -e POSTGRES_DB -e PGDATA=/var/lib/postgresql/data/pgdata \
  -v "${STACK}-pg:/var/lib/postgresql/data" \
  --health-cmd 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"' --health-interval 5s --health-timeout 5s --health-retries 12 --health-start-period 5s \
  "$PG_IMAGE" >/dev/null
podman run -d --pod "$POD" --name "${STACK}-redis" --memory 256m -v "${STACK}-rd:/data" \
  --health-cmd 'redis-cli ping | grep -q PONG' --health-interval 5s --health-timeout 3s --health-retries 12 \
  "$REDIS_IMAGE" >/dev/null
wait_healthy "${STACK}-postgres" 90; wait_healthy "${STACK}-redis" 60

podman run -d --pod "$POD" --name "${STACK}-api" --memory 2g --restart on-failure:3 \
  -e DATABASE_TYPE=postgres -e DATABASE_HOST=127.0.0.1 -e DATABASE_PORT=5432 \
  -e DATABASE_USER="$POSTGRES_USER" -e DATABASE_PASSWORD="$POSTGRES_PASSWORD" -e DATABASE_NAME="$POSTGRES_DB" \
  -e DATABASE_ENCRYPTION_KEY_FILE=/app/keys/db.key -e REDIS_HOST=127.0.0.1 -e REDIS_PORT=6379 \
  -e APP_ENV=development -e LOG_LEVEL=info -e JWT_SECRET -e ADMIN_USERNAME -e ADMIN_PASSWORD -e REGISTRATION_MODE \
  -e CORS_ENABLED=true -e CORS_ORIGINS='*' -e SMB_ENABLED=true -e MEDIA_ROOT_PATH="$MEDIA_MOUNT" \
  -e CATALOGIZER_LOCAL_ROOT_BASES="$MEDIA_MOUNT" -e STORAGE_TYPE=local -e STORAGE_BUCKET=catalogizer-covers -e STORAGE_USE_SSL=false -e STORAGE_REGION=us-east-1 \
  -v "$MEDIA_DIR:$MEDIA_MOUNT:ro" -v "${STACK}-api_data:/app/data" -v "${STACK}-api_logs:/var/log/catalogizer" -v "${STACK}-api_keys:/app/keys" \
  --health-cmd 'wget -q -O /dev/null http://127.0.0.1:8080/health' --health-interval 5s --health-timeout 3s --health-retries 12 --health-start-period 10s \
  "localhost/${STACK}-api:$TAG" >/dev/null
wait_healthy "${STACK}-api" 120

podman run -d --pod "$POD" --name "${STACK}-web" --memory 256m -e API_UPSTREAM=127.0.0.1:8080 \
  "localhost/${STACK}-web:$TAG" >/dev/null
if [ -f "$QA_DIR/website_dist/index.html" ]; then
  podman run -d --pod "$POD" --name "${STACK}-website" --memory 128m -v "$QA_DIR/website_dist:/usr/share/nginx/html:ro,Z" "$NGINX_IMAGE" >/dev/null
else echo "NOTE: no $QA_DIR/website_dist - website container not started (build.sh builds it)"; fi
for i in $(seq 1 30); do [ "$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${WEB_PORT}/")" = 200 ] && break; sleep 1; done   # web: nginx, image HEALTHCHECK is not honoured by podman (OCI format)

for u in "http://127.0.0.1:${API_PORT}/health" "http://127.0.0.1:${WEB_PORT}/" "http://127.0.0.1:${WEB_PORT}/api/v1/health" "http://127.0.0.1:${WEBSITE_PORT}/"; do
  printf '%s -> %s\n' "$u" "$(curl -s -o /dev/null -w '%{http_code}' "$u")"; done
podman ps --filter "pod=$POD" --format '{{.Names}}\t{{.Status}}\t{{.Image}}'
