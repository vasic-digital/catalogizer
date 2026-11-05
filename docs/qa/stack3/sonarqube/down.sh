#!/usr/bin/env bash
# Stop and remove ONLY this service's pod (end of the scan window). Volumes (analysis history) kept unless --purge.
# Refuses (removes nothing) unless the pod / every existing volume carries our ownership label.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
require_owned_resources
if podman pod exists "$POD" 2>/dev/null; then podman pod rm -f "$POD" >/dev/null && echo "pod $POD removed"; else echo "pod $POD not present"; fi
if [ "${1:-}" = "--purge" ]; then
  for v in pg data extensions logs; do
    if podman volume exists "${STACK}-$v" 2>/dev/null; then owned_vol "${STACK}-$v" && podman volume rm -f "${STACK}-$v" >/dev/null; fi
  done
  echo "volumes purged (secrets in ${SQ_DIR:?} kept)"
fi
