#!/usr/bin/env bash
# Remove only this add-on's pod + secret (volumes kept; --purge removes them too). Touches no stack-3 container.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
podman pod rm -f "$MON_POD" >/dev/null 2>&1 || true
for s in pgpass qaadmin gfpw pgapw; do podman secret rm "${MON_POD}-$s" >/dev/null 2>&1 || true; done
if [ "${1:-}" = "--purge" ]; then for v in cfg tok prom graf pga; do podman volume rm -f "${MON_POD}-$v" >/dev/null 2>&1 || true; done; fi
echo "removed pod $MON_POD"
