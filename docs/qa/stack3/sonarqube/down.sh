#!/usr/bin/env bash
# Stop and remove ONLY this service's pod (end of the scan window). Volumes (analysis history) kept unless --purge.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
podman pod rm -f "$POD" >/dev/null 2>&1 && echo "pod $POD removed" || echo "pod $POD not present"
if [ "${1:-}" = "--purge" ]; then
  for v in pg data extensions logs; do podman volume rm -f "${STACK}-$v" >/dev/null 2>&1 || true; done
  echo "volumes purged (secrets in ${SQ_DIR:?} kept)"
fi
