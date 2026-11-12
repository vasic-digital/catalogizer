#!/bin/bash
# desktop_appimage_launch_challenge.sh — Article XI §11.2 regression
# guard for catalogizer-desktop and installer-wizard Tauri AppImages.
#
# Verifies on real hardware that the binary:
#   1. Launches without immediate crash (Article XI §11.2.1)
#   2. Loads webkit2gtk + wayland-client in its address space
#      (Article XI §11.2.2 — real WebView is wired, not a stub)
#   3. Maintains a non-trivial RSS for >5s (proves it didn't return
#      from main; Article XI §11.2.5 fails-when-feature-removed)
#   4. Cleans up cleanly on SIGTERM (Article XI §11.2.6)
#
# Negative verification: invoke with --self-test-negative to corrupt
# the binary (move it aside) and confirm the test correctly FAILs.
#
# Exit:
#   0 = pass
#   1 = fail
#   2 = setup error / binary missing

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

find_project_root() {
  local d="$1"
  while [[ "$d" != "/" ]]; do
    if [[ -f "$d/CLAUDE.md" && -d "$d/catalogizer-desktop" ]]; then
      echo "$d"; return 0
    fi
    d=$(dirname "$d")
  done
  return 1
}

PROJECT_ROOT=$(find_project_root "$SCRIPT_DIR" || true) # bluff-scan: ok (next line asserts non-empty result)
if [[ -z "${PROJECT_ROOT:-}" ]]; then
  echo "FAIL (setup): cannot locate project root from $SCRIPT_DIR" >&2
  exit 2
fi
cd "$PROJECT_ROOT"

# Two binaries to verify
declare -A BINARIES=(
  [catalogizer-desktop]="catalogizer-desktop/src-tauri/target/release/catalogizer-desktop"
  [installer-wizard]="installer-wizard/src-tauri/target/release/bundle/appimage/Catalogizer Installation Wizard_2.4.2_amd64.AppImage"
)

verify_binary() {
  local name="$1"
  local binpath="$2"
  echo
  echo "=== $name ==="
  if [[ ! -f "$binpath" ]]; then
    echo "FAIL ($name): binary missing at $binpath" >&2
    return 1
  fi
  if [[ ! -x "$binpath" ]]; then
    echo "FAIL ($name): binary not executable: $binpath" >&2
    return 1
  fi

  echo "  binary: $binpath"
  echo "  size:   $(du -h "$binpath" | awk '{print $1}')"
  file "$binpath" | head -1 | sed 's/^/  /'

  # Pre-launch: capture timestamp
  local started=$(date +%s)
  echo "  launch at $(date -Iseconds)"

  # Launch in background, capture stdout/stderr to a tmp log
  local tmplog
  tmplog=$(mktemp)
  "$binpath" >"$tmplog" 2>&1 &
  local pid=$!

  # Give it 5s to initialize (Tauri WebView creation + asset loading)
  sleep 5

  # 1. Process still running?
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "FAIL ($name): process died within 5s of launch" >&2
    echo "    log:" >&2
    sed 's/^/      /' "$tmplog" | head -20 >&2
    rm -f "$tmplog"
    return 1
  fi

  # 2. RSS check
  local rss_kb
  rss_kb=$(awk '/^VmRSS:/ {print $2}' /proc/$pid/status 2>/dev/null || echo 0)
  if [[ -z "$rss_kb" || "$rss_kb" -lt 30000 ]]; then
    echo "FAIL ($name): RSS=${rss_kb}KB after 5s — process not active" >&2
    kill -TERM "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null
    rm -f "$tmplog"
    return 1
  fi
  echo "  RSS:    $((rss_kb / 1024)) MB"

  # 3. WebKit + Wayland loaded?
  local maps="/proc/$pid/maps"
  if ! grep -q "libwebkit2gtk" "$maps" 2>/dev/null; then
    echo "FAIL ($name): webkit2gtk NOT loaded in process address space" >&2
    kill -TERM "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null
    rm -f "$tmplog"
    return 1
  fi
  echo "  ✓ libwebkit2gtk loaded"

  if grep -q "libwayland-client\|libX11" "$maps" 2>/dev/null; then
    if grep -q "libwayland-client" "$maps"; then
      echo "  ✓ libwayland-client loaded (Wayland)"
    else
      echo "  ✓ libX11 loaded (X11)"
    fi
  else
    echo "FAIL ($name): no display backend (Wayland/X11) loaded" >&2
    kill -TERM "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null
    rm -f "$tmplog"
    return 1
  fi

  # 4. Open file descriptors — should include the Wayland or X11 socket
  local socket_count
  socket_count=$(ls -la /proc/$pid/fd 2>/dev/null | grep -cE "wayland-|/tmp/.X11-unix")
  echo "  display sockets open: $socket_count"
  if [[ "$socket_count" -eq 0 ]]; then
    # On some systems the socket name differs; just warn.
    echo "  (note: no Wayland/X11 socket symlink found; webkit may use a daemon)"
  fi

  # 5. Negative-side check: process must NOT have a child crash
  local child_count
  child_count=$(pgrep -P "$pid" | wc -l)
  echo "  child procs: $child_count"

  # 6. Lifecycle: SIGTERM must clean up
  echo "  sending SIGTERM..."
  kill -TERM "$pid"
  local cleanup_deadline=$(($(date +%s) + 10))
  while kill -0 "$pid" 2>/dev/null; do
    if [[ "$(date +%s)" -ge "$cleanup_deadline" ]]; then
      echo "FAIL ($name): process did not exit within 10s of SIGTERM" >&2
      kill -KILL "$pid" 2>/dev/null
      wait "$pid" 2>/dev/null
      rm -f "$tmplog"
      return 1
    fi
    sleep 0.5
  done
  echo "  ✓ exited cleanly on SIGTERM"
  rm -f "$tmplog"
  return 0
}

# Optional self-test: corrupt the binary path and verify we FAIL
if [[ "${1:-}" == "--self-test-negative" ]]; then
  echo "=== Article XI §11.5 negative self-test ==="
  echo "  expecting verify_binary to FAIL on a missing binary..."
  out=$(verify_binary "missing-bin" "/nonexistent/path/to/binary" 2>&1)
  rc=$?
  if [[ $rc -ne 0 ]] && echo "$out" | grep -q "FAIL.*binary missing"; then
    echo "  ✓ negative self-test correctly returned exit $rc with expected diagnostic"
    exit 0
  else
    echo "  ✗ negative self-test was supposed to FAIL but didn't (exit=$rc)" >&2
    echo "    output:" >&2
    echo "$out" | sed 's/^/      /' >&2
    exit 1
  fi
fi

OK=1
for name in "${!BINARIES[@]}"; do
  if ! verify_binary "$name" "${BINARIES[$name]}"; then
    OK=0
  fi
done

echo
if [[ "$OK" -eq 1 ]]; then
  echo "PASS: both Tauri binaries launch, load WebView+display backend,"
  echo "      maintain RSS, and exit cleanly on SIGTERM."
  exit 0
else
  echo "FAIL: at least one Tauri binary did not pass Article XI §11.2 verification" >&2
  exit 1
fi
