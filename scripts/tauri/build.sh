#!/usr/bin/env bash
# Build catalogizer-desktop and installer-wizard (Tauri 2) inside a rootless podman container.
# Usage: scripts/tauri/build.sh [OUT_DIR]   (OUT_DIR default: $HOME/.cache/claude-tmp/tauri-out, not committed)
# Env: TAURI_BUILD_MEM (default 5G), TAURI_BUILD_JOBS (default 3), TAURI_IMAGE (default catalogizer-tauri:qa)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${1:-$HOME/.cache/claude-tmp/tauri-out}"
IMAGE="${TAURI_IMAGE:-catalogizer-tauri:qa}"
MEM="${TAURI_BUILD_MEM:-5G}"
JOBS="${TAURI_BUILD_JOBS:-3}"
mkdir -p "${OUT:?}" "${OUT}/cargo-home" "${OUT}/target-desktop" "${OUT}/target-wizard"

if ! podman image exists "$IMAGE"; then
  systemd-run --user --scope -p MemoryMax="$MEM" -p MemorySwapMax=0 \
    podman build -t "$IMAGE" -f "$ROOT/docker/Dockerfile.tauri" "$ROOT/docker"
fi

# Source is mounted read-only; node_modules/dist are built in a container-local copy.
systemd-run --user --scope -p MemoryMax="$MEM" -p MemorySwapMax=0 \
  podman run --rm --userns=keep-id -e HOME=/tmp/h -e CARGO_BUILD_JOBS="$JOBS" \
    -v "$ROOT/catalogizer-desktop:/src/desktop:ro,Z" \
    -v "$ROOT/installer-wizard:/src/wizard:ro,Z" \
    -v "$OUT:/out:Z" "$IMAGE" bash -euxo pipefail -c '
      mkdir -p /tmp/h
      export CARGO_HOME=/out/cargo-home
      rustc --version; cargo --version; node --version; npm --version
      for app in desktop wizard; do
        rm -rf "/tmp/b-$app" && mkdir "/tmp/b-$app"
        (cd "/src/$app" && tar --exclude=node_modules --exclude=dist --exclude=target -cf - .) | tar -xf - -C "/tmp/b-$app"
        cd "/tmp/b-$app"
        npm ci --no-audit --no-fund
        CARGO_TARGET_DIR=/out/target-$app npx tauri build --no-bundle
        bin=$(grep -m1 "^name" src-tauri/Cargo.toml | cut -d\" -f2)
        mkdir -p /out/artifacts/$app
        cp "/out/target-$app/release/$bin" /out/artifacts/$app/
        cp src-tauri/Cargo.lock /out/artifacts/$app/Cargo.lock 2>/dev/null || true
        cp -a dist /out/artifacts/$app/dist
        cd /
      done
      (cd /out/artifacts && find . -type f ! -path "*/dist/*" -exec sha256sum {} \; > SHA256SUMS)
    '
echo "artifacts: $OUT/artifacts"; cat "$OUT/artifacts/SHA256SUMS"
