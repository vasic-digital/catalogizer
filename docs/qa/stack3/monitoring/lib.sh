#!/usr/bin/env bash
# Shared settings for the stack-3 monitoring add-on (sourced by up.sh / down.sh). Secrets live ONLY in $QA_DIR/mon.env (0600, outside repo).
STACK="${STACK:-catqa3}"; MON_POD="${MON_POD:-${STACK}mon}"
QA_DIR="${QA_DIR:-$HOME/.cache/claude-tmp/${STACK}}"
# The pod reaches ONLY these four host loopback/published ports (pasta `-T`: pod-loopback port N -> host-loopback port N), so MON_HOST and the
# Postgres host are the pod's own 127.0.0.1. No other host-loopback service (other stacks' DB/Redis, 11434, ...) is reachable from the pod.
MON_HOST="${MON_HOST:-127.0.0.1}"; MON_PG_HOST="${MON_PG_HOST:-127.0.0.1}"
API_PORT="${API_PORT:-38082}"; WEB_PORT="${WEB_PORT:-33020}"; WEBSITE_PORT="${WEBSITE_PORT:-33021}"; PG_PORT="${PG_PORT:-35434}"
PROM_PORT="${PROM_PORT:-39090}"; GRAFANA_PORT="${GRAFANA_PORT:-33030}"; PGADMIN_PORT="${PGADMIN_PORT:-33050}"
NGINX_PORT="${NGINX_PORT:-33080}"; NGINX_SITE_PORT="${NGINX_SITE_PORT:-33081}"
# Bind policy: nginx is the LAN entry (0.0.0.0, like stack 3). Prometheus (no auth), Grafana and pgAdmin (authenticated admin UIs that can
# be pointed at other targets) are LOOPBACK by default. LAN opt-in for manual QA: LAN_BIND=0.0.0.0 ./up.sh (Grafana+pgAdmin) and/or PROM_BIND=0.0.0.0.
NGINX_BIND="${NGINX_BIND:-0.0.0.0}"; PROM_BIND="${PROM_BIND:-127.0.0.1}"
GRAFANA_BIND="${GRAFANA_BIND:-${LAN_BIND:-127.0.0.1}}"; PGADMIN_BIND="${PGADMIN_BIND:-${LAN_BIND:-127.0.0.1}}"
PROM_IMAGE="${PROM_IMAGE:-docker.io/prom/prometheus:v2.55.1}"
GRAFANA_IMAGE="${GRAFANA_IMAGE:-docker.io/grafana/grafana:11.3.1}"
PGADMIN_IMAGE="${PGADMIN_IMAGE:-docker.io/dpage/pgadmin4:8.13}"
NGINX_IMAGE="${NGINX_IMAGE:-docker.io/library/nginx:1.27-alpine}"
CURL_IMAGE="${CURL_IMAGE:-docker.io/curlimages/curl:8.10.1}"
# F11: names and ports are interpolated into podman/sed commands - refuse anything unexpected.
_re_name='^[a-z][a-z0-9_-]{0,30}$'; _re_host='^[A-Za-z0-9.:-]{1,64}$'; _re_port='^[0-9]{2,5}$'
[[ "$STACK" =~ $_re_name && "$MON_POD" =~ $_re_name ]] || { echo "invalid STACK/MON_POD" >&2; return 2 2>/dev/null || exit 2; }
[[ "$MON_HOST" =~ $_re_host && "$MON_PG_HOST" =~ $_re_host ]] || { echo "invalid MON_HOST/MON_PG_HOST" >&2; return 2 2>/dev/null || exit 2; }
for _p in "$API_PORT" "$WEB_PORT" "$WEBSITE_PORT" "$PG_PORT" "$PROM_PORT" "$GRAFANA_PORT" "$PGADMIN_PORT" "$NGINX_PORT" "$NGINX_SITE_PORT"; do
  [[ "$_p" =~ $_re_port ]] || { echo "invalid port '$_p'" >&2; return 2 2>/dev/null || exit 2; }; done
