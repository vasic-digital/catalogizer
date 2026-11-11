#!/usr/bin/env bash
# Recreate ONLY catqa3-api, using the exact `podman run` recipe of docs/qa/stack3/up.sh (extracted by line range, not retyped).
# usage: api_recreate.sh on|off     on = HELIX_PPROF_ENABLED=true ; off = flag unset (original config)
set -euo pipefail
MODE="${1:?on|off}"
REPO=/home/milosvasic/Projects/catalogizer
UP="$REPO/docs/qa/stack3/up.sh"
. "$REPO/docs/qa/stack3/lib.sh"
set -a; . "$QA_DIR/qa.env"; set +a
export DATABASE_PASSWORD="$POSTGRES_PASSWORD"
TAG="main-$(cut -c1-8 "$QA_DIR/BUILT_COMMIT")"
if [ "$MODE" = on ]; then export HELIX_PPROF_ENABLED=true; else unset HELIX_PPROF_ENABLED; fi
s=$(grep -n 'podman run -d --pod "$POD" --name "${STACK}-api"' "$UP" | cut -d: -f1); e=$(grep -n 'wait_healthy "${STACK}-api" 120' "$UP" | cut -d: -f1)
RUNF="$(mktemp)"; sed -n "${s},$((e-1))p" "$UP" > "$RUNF"
wait_healthy() { local c="$1" t="$2" i; for i in $(seq 1 "$t"); do
  [ "$(podman inspect "$c" --format '{{.State.Health.Status}}' 2>/dev/null)" = healthy ] && { echo "$c healthy after ${i}s"; return 0; }; sleep 1; done; echo "$c not healthy" >&2; return 1; }
podman inspect "${STACK}-api" --format '{{.Id}}' >/dev/null   # must exist
podman stop -t 20 "${STACK}-api" >/dev/null; podman container rm "${STACK}-api" >/dev/null
. "$RUNF"
wait_healthy "${STACK}-api" 120
