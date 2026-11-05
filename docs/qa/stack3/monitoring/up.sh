#!/usr/bin/env bash
# Bring up the QA monitoring add-on for stack 3: Prometheus + Grafana + pgAdmin + nginx reverse proxy (+ token refresher) as ONE rootless
# pod `${STACK}mon` on pasta (`--map-gw`: the pod reaches the host's published stack-3 ports and loopback Postgres via MON_HOST).
# Idempotent: replaces only its own pod; stack-3 containers are only READ from (HTTP/TCP clients), never modified.
# Exit codes: 0 all probes ok | 3 image missing / stack 3 down | 4 low memory | 5 port held by a foreign owner | 6 a service not ready.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
QENV="$QA_DIR/qa.env"; MENV="$QA_DIR/mon.env"
[ -f "$QENV" ] || { echo "$QENV missing - bring up stack 3 first" >&2; exit 3; }
if [ ! -f "$MENV" ]; then   # fresh random secrets, never printed
  ( umask 077; r() { head -c "$1" /dev/urandom | od -An -vtx1 | tr -d ' \n'; }
    { echo "GF_SECURITY_ADMIN_USER=qa_grafana"; echo "GF_SECURITY_ADMIN_PASSWORD=Gf-$(r 12)-Zz9"
      echo "PGADMIN_DEFAULT_EMAIL=qa-admin@example.com"; echo "PGADMIN_DEFAULT_PASSWORD=Pg-$(r 12)-Zz9"; } > "$MENV" )
  echo "generated $MENV (0600)"
fi
[ "$(stat -c %a "$MENV")" = 600 ] || chmod 600 "$MENV"
set -a; . "$QENV"; . "$MENV"; set +a
for i in "$PROM_IMAGE" "$GRAFANA_IMAGE" "$PGADMIN_IMAGE" "$NGINX_IMAGE" "$CURL_IMAGE"; do
  podman image exists "$i" || { echo "image $i missing - podman pull it (pinned tag)" >&2; exit 3; }; done
[ "$(curl -s -m 5 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${API_PORT}/health")" = 200 ] || { echo "stack 3 API not healthy on :${API_PORT}" >&2; exit 3; }
avail_kb=$(awk '/MemAvailable/{print $2}' /proc/meminfo)
[ "$avail_kb" -gt 4000000 ] || { echo "only $((avail_kb/1024)) MiB available (<4 GiB) - refusing" >&2; exit 4; }
own_ports=""; if podman pod exists "$MON_POD"; then
  infra="$(podman pod inspect "$MON_POD" --format '{{.InfraContainerID}}' 2>/dev/null || true)"; [ -z "$infra" ] || own_ports="$(podman port "$infra" 2>/dev/null || true)"; fi
for p in "$PROM_PORT" "$GRAFANA_PORT" "$PGADMIN_PORT" "$NGINX_PORT" "$NGINX_SITE_PORT"; do
  if ss -ltnH "sport = :$p" | grep -q . && ! printf '%s\n' "$own_ports" | grep -qE ":$p\$"; then
    echo "port $p is in use by something other than pod $MON_POD - refusing" >&2; exit 5; fi; done

# ---- render configs (placeholders -> values) into a scratch dir, then copy into a podman volume (container uids cannot traverse $QA_DIR 0700)
R="$(mktemp -d "$QA_DIR/mon-render.XXXXXX")"; trap 'rm -rf -- "${R:?}"' EXIT
mkdir -p "$R/prometheus" "$R/grafana" "$R/pgadmin" "$R/nginx"
sub() { sed -e "s|__MON_HOST__|$MON_HOST|g" -e "s|__API_PORT__|$API_PORT|g" -e "s|__WEB_PORT__|$WEB_PORT|g" \
  -e "s|__WEBSITE_PORT__|$WEBSITE_PORT|g" -e "s|__PG_HOST__|$MON_PG_HOST|g" -e "s|__PG_PORT__|$PG_PORT|g" -e "s|__PG_USER__|$POSTGRES_USER|g" "$1" > "$2"; }
sub "$HERE/prometheus/prometheus.yml" "$R/prometheus/prometheus.yml"
sub "$HERE/nginx/nginx.conf" "$R/nginx/nginx.conf"
sub "$HERE/pgadmin/servers.json" "$R/pgadmin/servers.json"
cp -r "$HERE/grafana/provisioning" "$HERE/grafana/dashboards" "$R/grafana/"
cp "$HERE/token_refresher.sh" "$R/token_refresher.sh"
( umask 077; printf '%s:%s:%s:%s:%s\n' "$MON_PG_HOST" "$PG_PORT" '*' "$POSTGRES_USER" "$POSTGRES_PASSWORD" > "$R/pgpass" )

podman pod rm -f "$MON_POD" >/dev/null 2>&1 || true
podman secret rm "${MON_POD}-pgpass" >/dev/null 2>&1 || true
podman secret create "${MON_POD}-pgpass" "$R/pgpass" >/dev/null
rm -f -- "${R:?}/pgpass"
podman volume rm -f "${MON_POD}-cfg" >/dev/null 2>&1 || true
podman volume create "${MON_POD}-cfg" >/dev/null; podman volume create "${MON_POD}-tok" >/dev/null 2>&1 || true
podman run --rm --user 0 -v "$R:/src:ro" -v "${MON_POD}-cfg:/dst" "$CURL_IMAGE" sh -c 'cp -r /src/. /dst/ && chmod -R a+rX /dst' >/dev/null
podman run --rm --user 0 -v "${MON_POD}-tok:/tokens" "$CURL_IMAGE" sh -c 'rm -f /tokens/metrics.tok; chmod 755 /tokens' >/dev/null

podman pod create --name "$MON_POD" --network 'pasta:--map-gw' \
  -p "${PROM_BIND}:${PROM_PORT}:9090" -p "${LAN_BIND}:${GRAFANA_PORT}:3000" -p "${LAN_BIND}:${PGADMIN_PORT}:5050" \
  -p "${LAN_BIND}:${NGINX_PORT}:8088" -p "${LAN_BIND}:${NGINX_SITE_PORT}:8089" >/dev/null

export MON_HOST API_PORT
podman run -d --pod "$MON_POD" --name "${MON_POD}-tokenrefresh" --memory 32m --user 0 -e ADMIN_USERNAME -e ADMIN_PASSWORD -e MON_HOST -e API_PORT \
  -v "${MON_POD}-cfg:/cfg:ro" -v "${MON_POD}-tok:/tokens" "$CURL_IMAGE" sh /cfg/token_refresher.sh >/dev/null
for i in $(seq 1 40); do podman run --rm --user 0 -v "${MON_POD}-tok:/tokens:ro" "$CURL_IMAGE" test -s /tokens/metrics.tok 2>/dev/null && break; sleep 1; done
podman run --rm --user 0 -v "${MON_POD}-tok:/tokens:ro" "$CURL_IMAGE" test -s /tokens/metrics.tok || { echo "token refresher produced no token" >&2; podman logs "${MON_POD}-tokenrefresh" >&2; exit 6; }

podman run -d --pod "$MON_POD" --name "${MON_POD}-prometheus" --memory 512m \
  -v "${MON_POD}-cfg:/cfg:ro" -v "${MON_POD}-tok:/tokens:ro" -v "${MON_POD}-prom:/prometheus" \
  "$PROM_IMAGE" --config.file=/cfg/prometheus/prometheus.yml --storage.tsdb.path=/prometheus --storage.tsdb.retention.time=7d >/dev/null
podman run -d --pod "$MON_POD" --name "${MON_POD}-grafana" --memory 384m \
  -e GF_SECURITY_ADMIN_USER -e GF_SECURITY_ADMIN_PASSWORD -e GF_USERS_ALLOW_SIGN_UP=false -e GF_ANALYTICS_REPORTING_ENABLED=false -e GF_ANALYTICS_CHECK_FOR_UPDATES=false \
  -e GF_PATHS_PROVISIONING=/cfg/grafana/provisioning -e GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/cfg/grafana/dashboards/catalogizer-api.json \
  -v "${MON_POD}-cfg:/cfg:ro" -v "${MON_POD}-graf:/var/lib/grafana" "$GRAFANA_IMAGE" >/dev/null
podman run -d --pod "$MON_POD" --name "${MON_POD}-pgadmin" --memory 512m \
  -e PGADMIN_DEFAULT_EMAIL -e PGADMIN_DEFAULT_PASSWORD -e PGADMIN_LISTEN_PORT=5050 -e PGADMIN_CONFIG_UPGRADE_CHECK_ENABLED=False \
  -e PGADMIN_SERVER_JSON_FILE=/cfg/pgadmin/servers.json -e PGPASSFILE=/pgpass \
  --secret "${MON_POD}-pgpass,target=/pgpass,uid=5050,mode=0600" \
  -v "${MON_POD}-cfg:/cfg:ro" -v "${MON_POD}-pga:/var/lib/pgadmin" "$PGADMIN_IMAGE" >/dev/null
podman run -d --pod "$MON_POD" --name "${MON_POD}-nginx" --memory 64m -v "${MON_POD}-cfg:/cfg:ro" \
  "$NGINX_IMAGE" nginx -c /cfg/nginx/nginx.conf -g 'daemon off;' >/dev/null

rc=0; probe() { local n="$1" u="$2" c i; for i in $(seq 1 90); do c="$(curl -s -m 5 -o /dev/null -w '%{http_code}' "$u" || true)"; [ "$c" = 200 ] && break; sleep 1; done
  printf '%-12s %s -> %s\n' "$n" "$u" "$c"; [ "$c" = 200 ] || rc=6; }
probe prometheus "http://127.0.0.1:${PROM_PORT}/-/ready"; probe grafana "http://127.0.0.1:${GRAFANA_PORT}/api/health"
probe pgadmin "http://127.0.0.1:${PGADMIN_PORT}/misc/ping"; probe nginx "http://127.0.0.1:${NGINX_PORT}/edge-health"
podman ps --filter "pod=$MON_POD" --format '{{.Names}}\t{{.Status}}\t{{.Image}}'
exit "$rc"
