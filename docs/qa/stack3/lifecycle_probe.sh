#!/usr/bin/env bash
# LIVE probe (constitution 11.4.289): real rootless podman, real image, guid-bearing DBUS address exported on purpose.
# Touches ONLY the throwaway pod-less container catqa-lifecycle-probe. usage: lifecycle_probe.sh   (evidence on stdout)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
C=catqa-lifecycle-probe; IMG="${PROBE_IMAGE:-docker.io/library/redis:7-alpine}"
export DBUS_SESSION_BUS_ADDRESS="${PROBE_DBUS:-unix:path=/run/user/$(id -u)/bus,guid=0123456789abcdef0123456789abcdef}"
echo "date: $(date -Is)"; echo "podman: $(podman --version)"; echo "DBUS before: $DBUS_SESSION_BUS_ADDRESS"
. "$HERE/../lib/podman_env.sh"
export QA_STACK_PREFIX=catqa-lifecycle-
echo "backend info: $(qa_stack_backend_info 2>/dev/null)"
podman rm -f "$C" >/dev/null 2>&1 || true
podman create --name "$C" "$IMG" >/dev/null || { echo "create failed"; exit 2; }
echo "--- control: raw podman start with the guid address (expected to FAIL in an agent shell; informational)"
podman start "$C" 2>&1 | sed 's/^/raw: /'; echo "raw rc=${PIPESTATUS[0]}"; podman stop -t 2 "$C" >/dev/null 2>&1 || true
echo "--- wrapper start"
qa_stack_start "$C"; rc=$?; echo "qa_stack_start rc=$rc  DBUS after: $DBUS_SESSION_BUS_ADDRESS"
echo "state: $(podman inspect "$C" --format '{{.State.Status}}')"
[ "$(podman inspect "$C" --format '{{.State.Status}}')" = running ] || { echo "ASSERT FAIL: not running"; podman rm -f "$C" >/dev/null 2>&1; exit 1; }
echo "--- wrapper stop"; qa_stack_stop "$C"; echo "qa_stack_stop rc=$?  state: $(podman inspect "$C" --format '{{.State.Status}}')"
echo "--- refusal of another stack"; qa_stack_stop catqa1-api; echo "rc=$? (64 expected)"
podman rm -f "$C" >/dev/null; podman container exists "$C" && echo "LEFTOVER" || echo "cleanup ok: $C removed"
