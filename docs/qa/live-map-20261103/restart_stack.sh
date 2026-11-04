#!/usr/bin/env bash
# usage: restart_stack.sh <qa_stack_dir> <project> <api_port>
# Staged start (fixes API-before-Postgres race AND the broken aardvark-dns in a re-created rootless netns:
# service names are pinned with extra_hosts from the live container IPs).
# Exit codes: 0 all healthy; non-zero on any failed step or readiness timeout (message on stderr).
set -euo pipefail
D="${1:?dir}"; P="${2:?project}"; PORT="${3:?port}"
die() { echo "restart_stack: $*" >&2; exit 1; }
cd "$D" || die "cannot cd to $D"
set -a; . "$D/qa.env"; set +a
podman pod rm -f "pod_$P" >/dev/null 2>&1 || true
podman rm -f "$P-postgres" "$P-redis" "$P-api" "$P-web" "$P-website" >/dev/null 2>&1 || true
OV=$(mktemp "${TMPDIR:-/tmp}/ov.XXXXXX.yml") || die "mktemp failed"
trap 'rm -f "${OV:?}"' EXIT
compose_up() { # keep compose's exit status (no pipe to tail)
  local log rc=0
  log=$(timeout 100 podman-compose -f "$D/qa-compose.yml" -f "$OV" up -d --no-build "$@" 2>&1) || rc=$?
  printf '%s\n' "$log" | tail -n 4
  [ $rc -eq 0 ] || die "podman-compose up $* failed (rc=$rc)"
}
container_ip() { podman inspect "$1" --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'; }
echo 'services: {}' > "$OV"
compose_up postgres redis
ready=0
for i in $(seq 1 40); do
  a=""; b=""
  podman exec "$P-postgres" pg_isready -U "$POSTGRES_USER" >/dev/null 2>&1 && a=ok
  b=$(podman exec "$P-redis" redis-cli ping 2>/dev/null || true)
  if [ "$a" = ok ] && [ "$b" = PONG ]; then echo "db+redis ready after $((i*3))s"; ready=1; break; fi
  sleep 3
done
[ $ready -eq 1 ] || die "postgres/redis not ready after 120s"
PG=$(container_ip "$P-postgres"); RD=$(container_ip "$P-redis"); echo "postgres=$PG redis=$RD"
printf 'services:\n  api:\n    extra_hosts: ["postgres:%s","redis:%s"]\n' "$PG" "$RD" > "$OV"
compose_up --no-deps api
podman start "$P-api" >/dev/null 2>&1 || true   # compose double-start can leave it Created (rootlessport bind race)
ok=0; c=""
for i in $(seq 1 60); do
  c=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/health" || true)
  if [ "$c" = 200 ]; then echo "API healthy 200 after $((i*2))s"; ok=1; break; fi
  sleep 2
done
[ $ok -eq 1 ] || die "API on :$PORT not healthy (last http=$c) after 120s"
AP=$(container_ip "$P-api"); echo "api=$AP"
printf 'services:\n  api:\n    extra_hosts: ["postgres:%s","redis:%s"]\n  web:\n    extra_hosts: ["api:%s"]\n' "$PG" "$RD" "$AP" > "$OV"
compose_up --no-deps web website
podman start "$P-web" "$P-website" >/dev/null 2>&1 || true
sleep 3
podman ps --filter "name=$P-" --format '{{.Names}}|{{.Status}}|{{.Ports}}'
