#!/usr/bin/env bash
# D6: the catalog-web nginx upstream is configurable through API_UPSTREAM.
#
# Real container pair on a throwaway rootless podman network:
#   backend : nginx answering /api/ping with a unique marker (stands in ONLY as
#             the thing being proxied TO; the unit under test is the web image's
#             config rendering + proxying)
#   web     : the catalog-web nginx config (rendered by the nginx entrypoint's
#             envsubst exactly as in catalog-web/Dockerfile) OR a full image.
#
# Usage: test_nginx_upstream.sh [WEB_IMAGE]   (default: nginx:alpine + mounted template)
# With WEB_IMAGE set, that image is run as-is (its own ENV defaults apply).
# Exit 0 = all checks passed, 1 = a check failed, 77 = SKIP (podman/image missing).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${HERE}/../nginx.conf.template"
WEB_IMAGE="${1:-}"
NGINX_IMG="docker.io/library/nginx:alpine"
command -v podman >/dev/null || { echo "SKIP hardware_not_present: podman"; exit 77; }
podman image exists "$NGINX_IMG" || { echo "SKIP hardware_not_present: $NGINX_IMG"; exit 77; }
[ -z "$WEB_IMAGE" ] || podman image exists "$WEB_IMAGE" || { echo "SKIP hardware_not_present: $WEB_IMAGE"; exit 77; }

T="$(mktemp -d)"; : "${T:?}"
SUF="$$-$(date +%s)"
NET="d6net-$SUF"; BE="d6be-$SUF"
FAILS=0
cleanup() { podman rm -f "$BE" "d6web-a-$SUF" "d6web-b-$SUF" >/dev/null 2>&1; podman network rm -f "$NET" >/dev/null 2>&1; rm -rf -- "${T:?}"; }
trap cleanup EXIT
check() { # name, expected, actual
  if [ "$2" = "$3" ]; then echo "PASS $1 (got: $3)"; else echo "FAIL $1 (expected: $2, got: $3)"; FAILS=$((FAILS+1)); fi
}
MARK="D6-BACKEND-MARKER-$SUF"
cat >"$T/be.conf" <<CONF
server { listen 80; location /api/ping { default_type text/plain; return 200 "$MARK"; } }
CONF
podman network create "$NET" >/dev/null || { echo "FAIL network create"; exit 1; }
podman run -d --name "$BE" --network "$NET" -v "$T/be.conf:/etc/nginx/conf.d/default.conf:ro" "$NGINX_IMG" >/dev/null || { echo "FAIL backend start"; exit 1; }

run_web() { # name, env-args...
  local name="$1"; shift
  if [ -n "$WEB_IMAGE" ]; then
    podman run -d --name "$name" --network "$NET" -p 127.0.0.1::3000 "$@" "$WEB_IMAGE" >/dev/null
  else
    podman run -d --name "$name" --network "$NET" -p 127.0.0.1::3000 \
      -v "$TEMPLATE:/etc/nginx/templates/default.conf.template:ro" \
      -e NGINX_ENVSUBST_FILTER='^API_' "$@" "$NGINX_IMG" >/dev/null
  fi
}
fetch() { # container path
  local port; port="$(podman port "$1" 3000/tcp | head -1 | sed 's/.*://')"
  for _ in $(seq 1 30); do
    out="$(curl -s --max-time 3 "http://127.0.0.1:${port}$2" 2>/dev/null)" && [ -n "$out" ] && { echo "$out"; return; }
    sleep 0.5
  done
  echo "<no-response>"
}

# 1. API_UPSTREAM points the proxy at the backend container.
run_web "d6web-a-$SUF" -e "API_UPSTREAM=${BE}:80" || { echo "FAIL web A start"; exit 1; }
check "proxy to API_UPSTREAM" "$MARK" "$(fetch "d6web-a-$SUF" /api/ping)"
# rendered config has no unsubstituted placeholder and nginx variables survive
RENDERED="$(podman exec "d6web-a-$SUF" cat /etc/nginx/conf.d/default.conf)"
check "no placeholder left in directives" "0" "$(printf '%s\n' "$RENDERED" | grep -v '^[[:space:]]*#' | grep -c 'API_UPSTREAM')"
check "nginx vars intact (\$host kept in /api and /ws)" "2" "$(printf '%s\n' "$RENDERED" | grep -c 'proxy_set_header Host \$host;')"
check "SPA location kept (try_files)" "1" "$(printf '%s' "$RENDERED" | grep -c 'try_files')"

# 2. Safe default when API_UPSTREAM is unset (full image carries the ENV default;
#    the mounted-template mode has no Dockerfile ENV, so the default check applies to the image mode only).
if [ -n "$WEB_IMAGE" ]; then
  run_web "d6web-b-$SUF" || true
  sleep 2
  DEF="$(podman exec "d6web-b-$SUF" cat /etc/nginx/conf.d/default.conf 2>/dev/null | grep -c 'proxy_pass http://host.containers.internal:8080;')"
  check "default upstream rendered (2 locations)" "2" "$DEF"
fi
echo "RESULT fails=$FAILS"
[ "$FAILS" -eq 0 ]
