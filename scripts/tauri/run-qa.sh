#!/usr/bin/env bash
# Run a built Tauri app (desktop|wizard) under Xvfb in a rootless container and expose it via noVNC for manual QA.
# Usage: scripts/tauri/run-qa.sh desktop|wizard [OUT_DIR]     (OUT_DIR from scripts/tauri/build.sh)
# Env:   QA_BIND (default 127.0.0.1; set to the LAN IP to expose on the LAN - read the warning below)
#        QA_VNC_PORT (default 6080 desktop / 6081 wizard)
# The noVNC/VNC password lives in $OUT/vnc.pass.txt (mode 0600, generated on first run, never printed).
# WARNING: with QA_BIND=<LAN IP> anyone on the LAN who knows the password controls the app session.
set -euo pipefail
APP="${1:?usage: run-qa.sh desktop|wizard [OUT_DIR]}"
OUT="${2:-$HOME/.cache/claude-tmp/tauri-out}"
IMAGE="${TAURI_IMAGE:-catalogizer-tauri:qa}"
BIND="${QA_BIND:-127.0.0.1}"
case "$APP" in
  desktop) BIN=catalogizer-desktop; PORT="${QA_VNC_PORT:-6080}";;
  wizard)  BIN=$(grep -m1 '^name' "$(cd "$(dirname "$0")/../.." && pwd)/installer-wizard/src-tauri/Cargo.toml" | cut -d'"' -f2); PORT="${QA_VNC_PORT:-6081}";;
  *) echo "app must be desktop|wizard" >&2; exit 2;;
esac
[ -x "$OUT/artifacts/$APP/$BIN" ] || { echo "missing $OUT/artifacts/$APP/$BIN (run scripts/tauri/build.sh)" >&2; exit 1; }
if [ ! -f "$OUT/vnc.pass.txt" ]; then
  umask 077; head -c 12 /dev/urandom | base64 | tr -d '/+=' | head -c 12 > "$OUT/vnc.pass.txt"
fi
chmod 600 "$OUT/vnc.pass.txt"
NAME="catalogizer-tauri-qa-$APP"
podman rm -f "$NAME" >/dev/null 2>&1 || true
exec systemd-run --user --scope -p MemoryMax=2G -p MemorySwapMax=0 \
  podman run -d --name "$NAME" -p "$BIND:$PORT:6080" \
    -v "$OUT/artifacts/$APP:/app:ro,Z" -v "$OUT/vnc.pass.txt:/run/vnc.pass:ro,Z" "$IMAGE" bash -c '
      export DISPLAY=:99 WEBKIT_DISABLE_COMPOSITING_MODE=1 WEBKIT_DISABLE_DMABUF_RENDERER=1 HOME=/tmp/h; mkdir -p $HOME
      Xvfb :99 -screen 0 1280x800x24 & sleep 1
      eval "$(dbus-launch --sh-syntax)"; openbox & sleep 1
      x11vnc -storepasswd "$(cat /run/vnc.pass)" /tmp/rfb >/dev/null 2>&1
      x11vnc -display :99 -rfbauth /tmp/rfb -forever -shared -localhost -rfbport 5900 -quiet &
      sleep 1; websockify --web /usr/share/novnc 6080 localhost:5900 &
      exec /app/'"$BIN"'
    '
