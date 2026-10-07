#!/bin/bash
#
# Test Environment Setup Script
# Starts REAL test infrastructure (PostgreSQL, Redis, FTP, SMB, WebDAV; `nfs` on request) for integration tests through the lifecycle script
# scripts/test-infra/up.sh: one compose project per run, per-run credentials and random loopback ports (no fixed port, no literal credential),
# the per-project lease, and a bounded wait until every service answers its PROTOCOL (not merely an open port). docs/testing/real-service-stack.md.
#
# Usage: scripts/setup-test-env.sh [--build-id <id>] [--services postgres,redis,ftp,smb,webdav,nfs]
#   --build-id   lowercase letters, digits, dash (default env-<UTC time>)
#   --services   subset to start (default: postgres,redis,ftp,smb,webdav)
# Exit: 0 every started service answered; non-zero when up.sh failed (its diagnostics are printed: a failed start is never reported as "not available").
# Earlier versions generated a stub docker-compose.test-infra.yml with the fixed ports 1445/2121 and the literal credentials testuser/testpass when the file was
# missing, and swallowed every start failure; both are gone (T129, WF12 F1): the compose file is a tracked, digest-pinned part of the repository.
#

set -e

BUILD_ID="env-$(date -u +%m%d%H%M%S)"
SERVICES=""
while [ $# -gt 0 ]; do
    case "$1" in
        --build-id) BUILD_ID="${2:-}"; shift 2 ;;
        --services) SERVICES="${2:-}"; shift 2 ;;
        -h|--help) sed -n 2,16p "$0"; exit 0 ;;
        *) echo "Error: unknown argument '$1'" >&2; exit 2 ;;
    esac
done

cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "=== Setting up Test Environment ==="
if [ ! -f "docker-compose.test-infra.yml" ]; then
    echo "Error: docker-compose.test-infra.yml not found (it is a tracked file of the repository: restore it with git)" >&2
    exit 1
fi

ARGS=(--build-id "$BUILD_ID")
[ -z "$SERVICES" ] || ARGS+=(--services "$SERVICES")
echo "Starting test infrastructure project catalogizer-test-$BUILD_ID ..."
OUT="$(bash scripts/test-infra/up.sh "${ARGS[@]}")" || {
    echo "Error: scripts/test-infra/up.sh failed (see its message above); nothing is left running" >&2
    exit 1
}
echo "$OUT"
OPID="$(printf '%s\n' "$OUT" | sed -n 's/^op_id=//p')"

echo ""
echo "Test environment setup complete!"
echo "  per-run environment file (mode 0600, never commit it): .audit/test-infra/catalogizer-test-$BUILD_ID/env"
echo "  host ports: the TI_PORT_* lines above (127.0.0.1 only)"
echo ""
echo "To stop test environment:"
echo "  bash scripts/test-infra/down.sh --build-id $BUILD_ID --op-id $OPID"
