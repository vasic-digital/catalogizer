#!/usr/bin/env bash
#
# container-local.sh - build and test Catalogizer INSIDE rootless podman on the LOCAL host.
#
# Purpose:  §11.4.173 build/test runner for the local build host (replaces the obsolete
#           thinker.local remote path of scripts/build_in_container.sh). Every compile,
#           npm install and test runs in a container; nothing builds on the bare host.
# Usage:    scripts/container-local.sh [--dry-run] <api-build|api-test|web-build|web-test>
# Env:      GO_IMAGE   (default docker.io/library/golang:1.26; go.mod needs >= 1.25.7)
#           NODE_IMAGE (default docker.io/library/node:22)
#           CAT_CACHE  (default ~/.cache/catalogizer-build: go module/build + npm caches)
#           CAT_OUT    (default <repo>/deploy/artifacts: api binary / web dist)
#           CAT_MEM    (default 8g)  CAT_CPUS (default 4)  -> host-safety ceilings (§12.6)
# Outputs:  api-build -> $CAT_OUT/catalog-api ; api-test -> $CAT_OUT/api-test.json (go test -json)
#           web-build -> $CAT_OUT/catalog-web-dist ; web-test -> $CAT_OUT/web-test.log
# Notes:    - Containers run non-root (--userns=keep-id), never rootful, never docker/sudo.
#           - Dependency download/install steps get network; test execution gets --network none
#             (hermetic: a test that needs the internet is a defect, not an environment issue).
#           - Submodules listed in catalog-api/go.mod (replace) and catalog-web/package.json (file:)
#             must be initialised first.
set -euo pipefail

DRY=0
if [ "${1:-}" = "--dry-run" ]; then DRY=1; shift; fi
MODE="${1:-}"
PROJ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_IMAGE="${GO_IMAGE:-docker.io/library/golang:1.26}"
NODE_IMAGE="${NODE_IMAGE:-docker.io/library/node:22}"
CACHE="${CAT_CACHE:-$HOME/.cache/catalogizer-build}"
OUT="${CAT_OUT:-$PROJ/deploy/artifacts}"
MEM="${CAT_MEM:-8g}"; CPUS="${CAT_CPUS:-4}"

run() { # run <network> <workdir> <image> <shell-snippet> [extra podman args...]
  local net="$1" wd="$2" img="$3" sh="$4"; shift 4
  local cmd=(podman run --rm --userns=keep-id --network "$net" --memory "$MEM" --cpus "$CPUS"
    -v "$PROJ:/src:Z" -v "$OUT:/out:Z" -v "$CACHE/gomod:/gocache/mod:Z" -v "$CACHE/gobuild:/gocache/build:Z"
    -v "$CACHE/npm:/npmcache:Z" -w "$wd"
    -e HOME=/tmp/home -e GOTOOLCHAIN=local -e CGO_ENABLED=1 -e GOFLAGS=-buildvcs=false
    -e GOMODCACHE=/gocache/mod -e GOCACHE=/gocache/build -e GOMAXPROCS="$CPUS"
    -e npm_config_cache=/npmcache -e CI=1 "$@" "$img" sh -c "$sh")
  if [ "$DRY" = 1 ]; then printf '%q ' "${cmd[@]}"; echo; else mkdir -p "$OUT" "$CACHE"/{gomod,gobuild,npm}; "${cmd[@]}"; fi
}

WEB_DEPS='set -e; mkdir -p /tmp/home
for d in media_types_ts websocket_client_ts catalogizer_api_client_ts auth_context_react ui_components_react media_browser_react media_player_react collection_manager_react dashboard_analytics_react; do
  (cd /src/submodules/$d && npm ci --legacy-peer-deps --no-audit --no-fund && npm run build)
done
cd /src/catalog-web && npm ci --legacy-peer-deps --no-audit --no-fund'

case "$MODE" in
  api-build)
    run host /src/catalog-api "$GO_IMAGE" 'set -e; go mod download; go build -p "$GOMAXPROCS" -o /out/catalog-api . && echo BUILD_OK'
    ;;
  api-test)
    run host /src/catalog-api "$GO_IMAGE" 'set -e; go mod download'
    run none /src/catalog-api "$GO_IMAGE" 'go test -p 2 -timeout 20m -json ./... > /out/api-test.json; rc=$?; echo go-test-rc=$rc; exit $rc'
    ;;
  web-build)
    run host /src/catalog-web "$NODE_IMAGE" "$WEB_DEPS"'; npm run build && rm -rf /out/catalog-web-dist && cp -r dist /out/catalog-web-dist && echo BUILD_OK'
    ;;
  web-test)
    run host /src/catalog-web "$NODE_IMAGE" "$WEB_DEPS"
    run none /src/catalog-web "$NODE_IMAGE" 'set -e; mkdir -p /tmp/home; npm test > /out/web-test.log 2>&1; rc=$?; tail -n 40 /out/web-test.log; exit $rc'
    ;;
  *) echo "usage: $0 [--dry-run] <api-build|api-test|web-build|web-test>" >&2; exit 2 ;;
esac
