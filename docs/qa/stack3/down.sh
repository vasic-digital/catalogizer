#!/usr/bin/env bash
# Stop and remove ONLY this stack's pod. Data volumes are kept unless --purge. Never touches other projects.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
podman pod rm -f "$POD" >/dev/null 2>&1 && echo "pod $POD removed" || echo "pod $POD not present"
if [ "${1:-}" = "--purge" ]; then
  for v in pg rd api_data api_logs api_keys; do podman volume rm -f "${STACK}-$v" >/dev/null 2>&1 || true; done
  echo "volumes purged (qa.env kept in $QA_DIR)"
fi
