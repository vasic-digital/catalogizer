#!/usr/bin/env bash
# Shared settings for the stack-3 monitoring add-on (sourced by up.sh / down.sh). Secrets live ONLY in $QA_DIR/mon.env (0600, outside repo).
STACK="${STACK:-catqa3}"; MON_POD="${MON_POD:-${STACK}mon}"
QA_DIR="${QA_DIR:-$HOME/.cache/claude-tmp/${STACK}}"
# Host address as seen from inside a pasta pod created with `--map-gw` (gateway IP -> the host incl. its loopback). The LAN IP of the
# host itself is NOT reachable from a pasta container (measured 2026-11-05: curl http://192.168.1.115:38082 -> 000, via gateway -> 200).
MON_HOST="${MON_HOST:-host.containers.internal}"
# Postgres is published on the host LOOPBACK only. From a pasta `--map-gw` pod the *gateway* address is mapped to the host's loopback
# (measured 2026-11-05: psql via 192.168.1.1 -> reaches catqa3-postgres; via host.containers.internal (169.254.1.2) -> refused).
MON_PG_HOST="${MON_PG_HOST:-$(ip -4 route show default | awk '{print $3; exit}')}"
API_PORT="${API_PORT:-38082}"; WEB_PORT="${WEB_PORT:-33020}"; WEBSITE_PORT="${WEBSITE_PORT:-33021}"; PG_PORT="${PG_PORT:-35434}"
PROM_PORT="${PROM_PORT:-39090}"; GRAFANA_PORT="${GRAFANA_PORT:-33030}"; PGADMIN_PORT="${PGADMIN_PORT:-33050}"
NGINX_PORT="${NGINX_PORT:-33080}"; NGINX_SITE_PORT="${NGINX_SITE_PORT:-33081}"
# Bind: Grafana/pgAdmin/nginx are authenticated or proxy-only -> 0.0.0.0 like stack 3. Prometheus has NO auth -> loopback unless opted in.
LAN_BIND="${LAN_BIND:-0.0.0.0}"; PROM_BIND="${PROM_BIND:-127.0.0.1}"   # opt in to LAN Prometheus: PROM_BIND=0.0.0.0
PROM_IMAGE="${PROM_IMAGE:-docker.io/prom/prometheus:v2.55.1}"
GRAFANA_IMAGE="${GRAFANA_IMAGE:-docker.io/grafana/grafana:11.3.1}"
PGADMIN_IMAGE="${PGADMIN_IMAGE:-docker.io/dpage/pgadmin4:8.13}"
NGINX_IMAGE="${NGINX_IMAGE:-docker.io/library/nginx:1.27-alpine}"
CURL_IMAGE="${CURL_IMAGE:-docker.io/curlimages/curl:8.10.1}"
