#!/usr/bin/env bash
# Start the on-demand SonarQube QA service (rootless podman pod, capped, LAN-bound). Idempotent.
# Pure script: no LLM/model calls. Run ./scan.sh next, then ./down.sh when the scan window is over.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
command -v podman jq curl >/dev/null || die "need podman, jq, curl"
# NOTE: no `cmd | grep -q` under pipefail (grep -q exits early -> SIGPIPE -> random failures). Capture first, then match.
ADDRS="$(ip -4 addr show)" || die "cannot list local addresses"
[[ "$ADDRS" == *"inet ${SONAR_BIND}/"* ]] || die "SONAR_BIND ${SONAR_BIND} is not a local address"
LISTEN="$(ss -ltn | awk '{print $4}')" || die "cannot list listening ports (ss failed) - refusing to start (fail closed)"
if grep -q "[:.]${SONAR_PORT}\$" <<<"$LISTEN" && ! podman pod exists "$POD"; then die "port ${SONAR_PORT} already in use by something else"; fi
require_owned_resources                          # F1: refuse to reuse/replace anything not labelled as ours
ensure_secrets; load_secrets

# Elasticsearch prerequisites (no sudo available/used): vm.max_map_count must be >= 262144, nofile >= 65535.
MMC="$(cat /proc/sys/vm/max_map_count)"; ES_EXTRA=""
if [ "$MMC" -lt 262144 ]; then
  ES_EXTRA="-Dnode.store.allow_mmap=false"
  echo "NOTE: vm.max_map_count=$MMC < 262144; falling back to $ES_EXTRA (slower, documented in README)"
fi
# Host disk: Elasticsearch's flood-stage watermark (95% used) makes every index read-only and SonarQube then aborts at boot
# ("Background initialization failed"). MEASURED 2026-11-05 on this host: / at 98% used. When the volume store is >= 90% used we
# disable ES's disk threshold via the documented sonar.search.javaAdditionalOpts hook (README: risk = ES can fill the disk).
VOLROOT="$(podman info --format '{{.Store.VolumePath}}' 2>/dev/null || true)"; [ -d "${VOLROOT:-/nonexistent}" ] || VOLROOT="$HOME"
check_disk_floor "$VOLROOT"                      # F4: hard floor (>=10 GB free, <=98% used)
USED="$DISK_USED_PCT"
if [ "${USED:-0}" -ge 90 ]; then
  ES_EXTRA="${ES_EXTRA:+$ES_EXTRA }-Dcluster.routing.allocation.disk.threshold_enabled=false"
  echo "NOTE: volume store ${USED}% used (>=90%); ES disk threshold disabled for this run (see README)"
fi
NOFILE="$(ulimit -Hn)"; [ "$NOFILE" -ge 65535 ] || die "hard nofile limit $NOFILE < 65535"

if podman pod exists "$POD" && [ "$(podman inspect -f '{{.State.Running}}' "${STACK}-sonar" 2>/dev/null || echo false)" = true ]; then
  echo "pod $POD already running"
else
  ensure_images "$SONAR_IMAGE" "$PG_IMAGE"       # before anything is created: a failed pull leaves no half-started pod
  wait_for_load "${SQ_LOAD_TIMEOUT:-900}"
  podman pod exists "$POD" && owned_pod && podman pod rm -f "$POD" >/dev/null 2>&1
  for v in pg data extensions logs; do podman volume exists "${STACK}-$v" || podman volume create --label "$LABEL_KEY=$LABEL_VAL" "${STACK}-$v" >/dev/null; done
  # Own pod = own network namespace (no shared network / DNS with other stacks); UI published on one LAN address only.
  podman pod create --name "$POD" --label "$LABEL_KEY=$LABEL_VAL" -p "${SONAR_BIND}:${SONAR_PORT}:9000" >/dev/null
  ( umask 077
    printf 'POSTGRES_USER=sonar\nPOSTGRES_DB=sonar\nPOSTGRES_PASSWORD=%s\n' "$SONAR_DB_PASSWORD" > "$SQ_DIR/pg.env"
    printf 'SONAR_JDBC_URL=jdbc:postgresql://127.0.0.1:5432/sonar\nSONAR_JDBC_USERNAME=sonar\nSONAR_JDBC_PASSWORD=%s\n' "$SONAR_DB_PASSWORD" > "$SQ_DIR/sonar-container.env" )
  podman run -d --pod "$POD" --name "${STACK}-db" --label "$LABEL_KEY=$LABEL_VAL" --memory "$PG_MEM" --memory-swap "$PG_MEM" --cpus "$PG_CPUS" \
    --env-file "$SQ_DIR/pg.env" -e PGDATA=/var/lib/postgresql/data/pgdata -v "${STACK}-pg:/var/lib/postgresql/data" \
    --health-cmd 'pg_isready -U sonar -d sonar' --health-interval 5s --health-timeout 5s --health-retries 12 "$PG_IMAGE" >/dev/null
  for i in $(seq 1 60); do [ "$(podman inspect -f '{{.State.Health.Status}}' "${STACK}-db")" = healthy ] && break; sleep 2; done
  [ "$(podman inspect -f '{{.State.Health.Status}}' "${STACK}-db")" = healthy ] || die "postgres not healthy"
  # 4g total: web 768m + CE 1g + ES 512m heaps leave headroom for off-heap/metaspace.
  podman run -d --pod "$POD" --name "${STACK}-sonar" --label "$LABEL_KEY=$LABEL_VAL" --memory "$SONAR_MEM" --memory-swap "$SONAR_MEM" --cpus "$SONAR_CPUS" \
    --ulimit nofile=131072:131072 --env-file "$SQ_DIR/sonar-container.env" \
    -e SONAR_WEB_JAVAOPTS="-Xmx768m -Xms256m" -e SONAR_CE_JAVAOPTS="-Xmx1g -Xms256m" \
    -e SONAR_SEARCH_JAVAOPTS="-Xmx512m -Xms512m" ${ES_EXTRA:+-e SONAR_SEARCH_JAVAADDITIONALOPTS="$ES_EXTRA"} \
    -e SONAR_TELEMETRY_ENABLE=false \
    -v "${STACK}-data:/opt/sonarqube/data" -v "${STACK}-extensions:/opt/sonarqube/extensions" -v "${STACK}-logs:/opt/sonarqube/logs" \
    "$SONAR_IMAGE" >/dev/null
fi

echo "waiting for SonarQube (up to 10 min)..."
for i in $(seq 1 "${SQ_UP_TRIES:-120}"); do
  st="$(curl -s --max-time 5 "$API_URL/api/system/status" | jq -r .status 2>/dev/null || true)"
  [ "$st" = UP ] && break; sleep 5
done
[ "${st:-}" = UP ] || { podman logs --tail 30 "${STACK}-sonar" >&2; die "SonarQube did not reach UP"; }

# First boot ships admin/admin: replace it with the generated password. Later boots: already changed.
write_netrc admin "$SONAR_ADMIN_PASSWORD"
if [ "$(sq -o /dev/null -w '%{http_code}' "$API_URL/api/authentication/validate")" != 200 ] || ! sq "$API_URL/api/authentication/validate" | jq -e .valid >/dev/null; then
  write_netrc admin admin
  code="$(sq -o /dev/null -w '%{http_code}' -X POST "$API_URL/api/users/change_password" \
    --data-urlencode login=admin --data-urlencode previousPassword=admin --data-urlencode "password=$SONAR_ADMIN_PASSWORD")"
  [ "$code" = 204 ] || die "could not change default admin password (HTTP $code)"
  write_netrc admin "$SONAR_ADMIN_PASSWORD"
fi
sq "$API_URL/api/authentication/validate" | jq -e .valid >/dev/null || die "admin login failed"

# Scan token: reuse if still valid, else mint (revoking the old one of the same name). Stored 0600 only.
tok_ok=0
if [ -n "${SONAR_TOKEN:-}" ]; then
  printf 'machine %s login %s password \n' "$SONAR_BIND" "$SONAR_TOKEN" > "$SQ_DIR/netrc.tok"; chmod 600 "$SQ_DIR/netrc.tok"
  curl -sS --netrc-file "$SQ_DIR/netrc.tok" "$API_URL/api/authentication/validate" | jq -e .valid >/dev/null 2>&1 && tok_ok=1
  rm -f "$SQ_DIR/netrc.tok"
fi
if [ "$tok_ok" = 0 ]; then
  sq -o /dev/null -X POST "$API_URL/api/user_tokens/revoke" --data-urlencode name=catqa-scan || true
  T="$(sq -X POST "$API_URL/api/user_tokens/generate" --data-urlencode name=catqa-scan --data-urlencode type=GLOBAL_ANALYSIS_TOKEN | jq -r .token)"
  [ -n "$T" ] && [ "$T" != null ] || die "token generation failed"
  grep -v '^SONAR_TOKEN=' "$SQ_DIR/sonar.env" > "$SQ_DIR/sonar.env.new" || true
  echo "SONAR_TOKEN=$T" >> "$SQ_DIR/sonar.env.new"; chmod 600 "$SQ_DIR/sonar.env.new"; mv "$SQ_DIR/sonar.env.new" "$SQ_DIR/sonar.env"
fi
echo "SonarQube is UP: $API_URL (login: admin; password and scan token in $SQ_DIR/sonar.env, mode 0600)"
podman ps --filter "pod=$POD" --format '{{.Names}}\t{{.Status}}\t{{.Image}}'
