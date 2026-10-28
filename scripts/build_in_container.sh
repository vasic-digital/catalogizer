#!/usr/bin/env bash
#
# §11.4.173 — Containerized + distributed build.
#
# Purpose:   Build catalog-api (Go) INSIDE a container on a REMOTE build host
#            (never on the bare host), then bring the artifact BACK to this host.
# Usage:     scripts/build_in_container.sh
# Inputs:    env (all config-injected, §11.4.28; defaults shown):
#              BUILD_HOST=<remote host; UNSET => exec scripts/container-local.sh api-build on THIS host>   BUILD_USER=milosvasic
#              REMOTE_BUILD_DIR=catalogizer-build
#              GO_BUILD_IMAGE=docker.io/library/golang:1.25-alpine
# Outputs:   BUILD_HOST unset (default): deploy/artifacts/catalog-api (written by container-local.sh; no md5 compare,
#            nothing is copied between hosts).
#            BUILD_HOST set: deploy/artifacts/catalog-api-container (the brought-back binary) + a printed proof
#            block (remote md5 == local md5, file type); exits 1 on md5 mismatch.
# Side-fx:   rsync's source to the remote build host; runs a podman build there.
# Deps:      ssh, rsync, scp, podman (on the remote), a Go build image (cached remote).
# Cross-ref: §11.4.173 (containerized+distributed build), §11.4.76 (containers),
#            §11.4.161 (rootless), docs/scripts/build_in_container.md.
set -euo pipefail

# thinker.local is retired as a default build host. With BUILD_HOST unset the build runs on the LOCAL host inside
# rootless podman via scripts/container-local.sh (same §11.4.173 containerized build, no remote hop). The remote
# path below stays available ONLY when the caller names a host explicitly (BUILD_HOST=<host>).
if [ -z "${BUILD_HOST:-}" ]; then
  echo "[build] BUILD_HOST not set: delegating to scripts/container-local.sh api-build (local rootless podman)" >&2
  exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/container-local.sh" api-build
fi
BUILD_HOST="${BUILD_HOST}"
BUILD_USER="${BUILD_USER:-milosvasic}"
REMOTE_DIR="${REMOTE_BUILD_DIR:-catalogizer-build}"
# glibc-based (debian) golang image — NOT alpine/musl: a CGO dep (docx.c) references glibc
# _FORTIFY_SOURCE symbols (__snprintf_chk) that musl lacks, so an alpine build fails at link.
GO_IMAGE="${GO_BUILD_IMAGE:-docker.io/library/golang:1.25-bookworm}"
PROJ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ART_DIR="$PROJ/deploy/artifacts"
HOST="$BUILD_USER@$BUILD_HOST"
mkdir -p "$ART_DIR"

echo "[build] §11.4.173 containerized+distributed build → $HOST ($GO_IMAGE)"

# 1. Sync source (catalog-api + the replaced submodules) to the remote build host.
echo "[build] syncing source to $HOST:~/$REMOTE_DIR/ ..."
ssh "$HOST" "mkdir -p ~/$REMOTE_DIR"
rsync -az --delete \
  --exclude '.git' --exclude 'node_modules' --exclude '/catalog-api/out' \
  --exclude 'build/' --exclude '*.db' --exclude 'catalog-api-local' \
  --exclude 'catalog-api-container' \
  "$PROJ/catalog-api" "$PROJ/submodules" \
  "$HOST:~/$REMOTE_DIR/"

# 2. Build INSIDE the container ON the remote host (CGO on for go-sqlite3 → add a
#    C toolchain to the alpine image at build time).
echo "[build] building catalog-api in container on $BUILD_HOST ..."
ssh "$HOST" "cd ~/$REMOTE_DIR && podman run --rm \
  -v \$PWD:/src:Z -w /src/catalog-api \
  -e GOTOOLCHAIN=local -e CGO_ENABLED=1 \
  $GO_IMAGE sh -c '(command -v apk >/dev/null 2>&1 && apk add --no-cache build-base sqlite-dev >/dev/null 2>&1) || true; go build -o catalog-api-container . && echo BUILD_OK'"

# 3. Capture the remote artifact identity.
REMOTE_MD5=$(ssh "$HOST" "md5sum ~/$REMOTE_DIR/catalog-api/catalog-api-container | cut -d' ' -f1")
REMOTE_SIZE=$(ssh "$HOST" "stat -c %s ~/$REMOTE_DIR/catalog-api/catalog-api-container")

# 4. Bring the artifact BACK to this host.
echo "[build] copying artifact back to $ART_DIR/ ..."
scp -q "$HOST:~/$REMOTE_DIR/catalog-api/catalog-api-container" "$ART_DIR/catalog-api-container"
LOCAL_MD5=$(md5sum "$ART_DIR/catalog-api-container" | cut -d' ' -f1)

# 5. Proof block (§11.4.6 — rock-solid evidence, no bluff).
echo "=== §11.4.173 BUILD PROOF ==="
echo "  build_host:   $BUILD_HOST (in container $GO_IMAGE)"
echo "  remote_md5:   $REMOTE_MD5  (size $REMOTE_SIZE)"
echo "  local_md5:    $LOCAL_MD5"
echo "  md5_match:    $([ "$REMOTE_MD5" = "$LOCAL_MD5" ] && echo YES || echo NO)"
echo "  artifact:     $(file -b "$ART_DIR/catalog-api-container")"
[ "$REMOTE_MD5" = "$LOCAL_MD5" ] || { echo "[build] FAIL: md5 mismatch"; exit 1; }
echo "[build] OK — built on $BUILD_HOST in a container, artifact verified on this host."
