#!/usr/bin/env bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
#
# Catalogizer Distributed Boot Script
# Uses the Containers module for automatic distribution to remote hosts
#
# Usage:
#   ./scripts/distributed-boot.sh [--local] [--dry-run] [--env PATH]
#
# Options:
#   --local      Run only locally (no remote distribution)
#   --dry-run    Show distribution plan without deploying
#   --env PATH   Path to .env file (default: ../submodules/containers/.env)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Default options
LOCAL_ONLY=""
DRY_RUN=""
ENV_FILE="${PROJECT_ROOT}/submodules/containers/.env"
COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.dev.yml"
TIMEOUT="5m"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --local)
            LOCAL_ONLY="--local"
            shift
            ;;
        --dry-run)
            DRY_RUN="--dry-run"
            shift
            ;;
        --env)
            ENV_FILE="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [--local] [--dry-run] [--env PATH] [--timeout DURATION]"
            echo ""
            echo "Options:"
            echo "  --local         Run only locally (no remote distribution)"
            echo "  --dry-run       Show distribution plan without deploying"
            echo "  --env PATH      Path to .env file (default: submodules/containers/.env)"
            echo "  --timeout DUR   Boot timeout (default: 5m)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "╔════════════════════════════════════════════════════════════╗"
echo "║       CATALOGIZER DISTRIBUTED BOOT SYSTEM                  ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "Configuration:"
echo "  Project root:  ${PROJECT_ROOT}"
echo "  Env file:      ${ENV_FILE}"
echo "  Compose file:  ${COMPOSE_FILE}"
echo "  Local only:    ${LOCAL_ONLY:-no}"
echo "  Dry run:       ${DRY_RUN:-no}"
echo ""

# Check if .env file exists
if [[ ! -f "${ENV_FILE}" ]]; then
    echo "Warning: .env file not found at ${ENV_FILE}"
    echo "Creating default configuration for local development..."
    mkdir -p "$(dirname "${ENV_FILE}")"
    cat > "${ENV_FILE}" << 'EOF'
# Catalogizer Container Distribution Configuration
# Copy this file and modify for your environment

# Enable/disable remote distribution
CONTAINERS_REMOTE_ENABLED=false

# Scheduler strategy: resource_aware, round_robin, affinity, spread, bin_pack
CONTAINERS_REMOTE_SCHEDULER=resource_aware

# Example remote host configuration (uncomment and modify):
# CONTAINERS_REMOTE_HOST_1_NAME=thinker
# CONTAINERS_REMOTE_HOST_1_ADDRESS=thinker.local
# CONTAINERS_REMOTE_HOST_1_PORT=22
# CONTAINERS_REMOTE_HOST_1_USER=milosvasic
# CONTAINERS_REMOTE_HOST_1_RUNTIME=podman
# CONTAINERS_REMOTE_HOST_1_LABELS=storage=fast,memory=high
EOF
    echo "Created default .env file at ${ENV_FILE}"
fi

# Build the boot command if needed
if [[ ! -f "${PROJECT_ROOT}/catalog-api/cmd/boot/boot" ]]; then
    echo "Building boot command..."
    cd "${PROJECT_ROOT}/catalog-api"
    go build -o cmd/boot/boot ./cmd/boot/
fi

# Run the boot command
cd "${PROJECT_ROOT}/catalog-api"
exec ./cmd/boot/boot \
    ${LOCAL_ONLY} \
    ${DRY_RUN} \
    --env "${ENV_FILE}" \
    --compose "${COMPOSE_FILE}" \
    --timeout "${TIMEOUT}"
