#!/bin/bash
#
# Test Environment Setup Script
# Starts REAL test infrastructure (PostgreSQL, Redis, FTP, SMB, WebDAV; `nfs` on request) for integration tests through the lifecycle script
# scripts/test-infra/up.sh: one compose project per run, per-run credentials and random loopback ports (no fixed port, no literal credential),
# the per-project lease, and a bounded wait until every service answers its PROTOCOL (not merely an open port). docs/testing/real-service-stack.md.
#
# Usage: scripts/setup-test-env.sh [--build-id <id>] [--services postgres,redis,ftp,smb,webdav,nfs]
#   --build-id   1..31 lowercase letters, digits, dash (ASCII only; default env-<UTC time>-<checkout hash>-<pid>: unique per second AND per checkout, so two checkouts never share the default project)
#   --services   subset to start (default: postgres,redis,ftp,smb,webdav)
# Exit: 0 every started service answered; non-zero when up.sh failed (its diagnostics are printed: a failed start is never reported as "not available").
#       2 usage. The failure message states what is LEFT: up.sh exit 3 = another owner's stack holds the project and is still running; 4 = a stale lease (down.sh decides);
#       1 = this start's partial project was torn down.
# An interrupt (INT/TERM/HUP) tears this start down through up.sh's own EXIT trap; the operation id is persisted at registration in <state>/op_id.
# Earlier versions generated a stub docker-compose.test-infra.yml with the fixed ports 1445/2121 and the literal credentials testuser/testpass when the file was
# missing, and swallowed every start failure; both are gone (T129, WF12 F1): the compose file is a tracked, digest-pinned part of the repository.
#

set -e
export LC_ALL=C

cd "$(dirname "${BASH_SOURCE[0]}")/.."
ROOT_HASH="$(printf '%s' "$(realpath -e -- "$PWD")" | sha256sum | cut -c1-6)"
BUILD_ID="env-$(date -u +%m%d%H%M%S)-$ROOT_HASH-$$"
SERVICES=""
while [ $# -gt 0 ]; do
    case "$1" in
        --build-id|--services)
            if [ $# -lt 2 ] || [ -z "$2" ]; then echo "Error: $1 needs a non-empty value" >&2; exit 2; fi
            if [ "$1" = --build-id ]; then BUILD_ID="$2"; else SERVICES="$2"; fi
            shift 2 ;;
        -h|--help) sed -n 2,18p "$0"; exit 0 ;;
        *) echo "Error: unknown argument '$1'" >&2; exit 2 ;;
    esac
done

echo "=== Setting up Test Environment ==="
if [ ! -f "docker-compose.test-infra.yml" ]; then
    echo "Error: docker-compose.test-infra.yml not found (it is a tracked file of the repository: restore it with git)" >&2
    exit 1
fi

ARGS=(--build-id "$BUILD_ID")
[ -z "$SERVICES" ] || ARGS+=(--services "$SERVICES")
echo "Starting test infrastructure project catalogizer-test-$BUILD_ID ..."
set +e
OUT="$(bash scripts/test-infra/up.sh "${ARGS[@]}")"; URC=$?
set -e
if [ "$URC" -ne 0 ]; then
    case "$URC" in
        3) echo "Error: scripts/test-infra/up.sh refused (exit 3): the project is held by another owner or holds resources that are not this checkout's; THAT stack (if any) is still running and was not touched" >&2;;
        4) echo "Error: scripts/test-infra/up.sh refused (exit 4): a stale lease; scripts/test-infra/down.sh --build-id $BUILD_ID decides" >&2;;
        2) echo "Error: scripts/test-infra/up.sh rejected its arguments (exit 2)" >&2;;
        *) echo "Error: scripts/test-infra/up.sh failed (exit $URC, see its message above); the partial project of this start was torn down" >&2;;
    esac
    exit 1
fi
echo "$OUT"
OPID="$(printf '%s\n' "$OUT" | sed -n 's/^op_id=//p')"

echo ""
echo "Test environment setup complete!"
echo "  per-run environment file (mode 0600, never commit it): .audit/test-infra/catalogizer-test-$BUILD_ID/env"
echo "  host ports: the TI_PORT_* lines above (127.0.0.1 only)"
echo ""
echo "To stop test environment:"
echo "  bash scripts/test-infra/down.sh --build-id $BUILD_ID --op-id $OPID"
