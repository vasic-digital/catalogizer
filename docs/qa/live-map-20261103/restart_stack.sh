#!/usr/bin/env bash
# usage: restart_stack.sh <qa_stack_dir> <project> <api_port>
# Staged start (fixes API-before-Postgres race AND the broken aardvark-dns in the re-created rootless netns:
# service names are pinned with extra_hosts from the live container IPs).
set -uo pipefail
D="${1:?dir}"; P="${2:?project}"; PORT="${3:?port}"
set -a; . "$D/qa.env"; set +a; cd "$D"
podman pod rm -f "pod_$P" >/dev/null 2>&1 || true
podman rm -f "$P-postgres" "$P-redis" "$P-api" "$P-web" "$P-website" >/dev/null 2>&1 || true
OV=$(mktemp "${TMPDIR:-/home/milosvasic/.cache/claude-tmp}/ov.XXXXXX.yml")
dc() { timeout 100 podman-compose -f "$D/qa-compose.yml" -f "$OV" up -d --no-build "$@" 2>&1 | tail -n 4; }
ip() { podman inspect "$1" --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'; }
echo 'services: {}' > "$OV"
dc postgres redis
for i in $(seq 1 40); do
  a=$(podman exec "$P-postgres" pg_isready -U "$POSTGRES_USER" >/dev/null 2>&1 && echo ok); b=$(podman exec "$P-redis" redis-cli ping 2>/dev/null)
  [ "$a" = ok ] && [ "$b" = PONG ] && { echo "db+redis ready after $((i*3))s"; break; }; sleep 3; done
PG=$(ip $P-postgres); RD=$(ip $P-redis); echo "postgres=$PG redis=$RD"
printf 'services:\n  api:\n    extra_hosts: ["postgres:%s","redis:%s"]\n' "$PG" "$RD" > "$OV"
dc --no-deps api
podman start "$P-api" >/dev/null 2>&1 || true   # compose double-start leaves it Created (rootlessport bind race)
for i in $(seq 1 60); do c=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/health"); [ "$c" = 200 ] && { echo "API healthy 200 after $((i*2))s"; break; }; sleep 2; done
AP=$(ip $P-api); echo "api=$AP"
printf 'services:\n  api:\n    extra_hosts: ["postgres:%s","redis:%s"]\n  web:\n    extra_hosts: ["api:%s"]\n' "$PG" "$RD" "$AP" > "$OV"
dc --no-deps web website
podman start "$P-web" "$P-website" >/dev/null 2>&1 || true
sleep 3; podman ps --filter "name=$P-" --format '{{.Names}}|{{.Status}}|{{.Ports}}'
rm -f "$OV"
