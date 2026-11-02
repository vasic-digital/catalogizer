#!/bin/bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/lib/cgo_flags.sh"   # 2026-11-02 decision: cgo define for go-sqlcipher (scripts/lib/cgo_flags.sh)
#
# Wire Go Submodules Script
# This script wires the remaining Go submodules into catalog-api
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CATALOG_API_DIR="${PROJECT_ROOT}/catalog-api"

echo "=== Wiring Go Submodules into Catalogizer ==="
echo "Project Root: ${PROJECT_ROOT}"
echo "Catalog API: ${CATALOG_API_DIR}"
echo ""

# Define submodules to wire
declare -A SUBMODULES=(
    ["Database"]="digital.vasic.database"
    ["Discovery"]="digital.vasic.discovery"
    ["Media"]="digital.vasic.media"
    ["Middleware"]="digital.vasic.middleware"
    ["Observability"]="digital.vasic.observability"
    ["RateLimiter"]="digital.vasic.ratelimiter"
    ["Security"]="digital.vasic.security"
    ["Storage"]="digital.vasic.storage"
    ["Streaming"]="digital.vasic.streaming"
    ["Watcher"]="digital.vasic.watcher"
    ["Panoptic"]="digital.vasic.panoptic"
)

cd "${CATALOG_API_DIR}"

# Backup go.mod
cp go.mod go.mod.backup.$(date +%Y%m%d_%H%M%S)

echo "Adding replace directives to go.mod..."

for submodule in "${!SUBMODULES[@]}"; do
    module_path="${SUBMODULES[$submodule]}"
    local_path="../${submodule}"
    
    # Check if submodule exists
    if [ -d "${PROJECT_ROOT}/${submodule}" ]; then
        echo "Wiring ${submodule} -> ${module_path}"
        
        # Check if already in go.mod
        if ! grep -q "replace ${module_path}" go.mod; then
            echo "replace ${module_path} => ${local_path}" >> go.mod
            echo "  ✓ Added replace directive"
        else
            echo "  ℹ Already wired"
        fi
    else
        echo "  ✗ Submodule ${submodule} not found, skipping"
    fi
done

echo ""
echo "Tidying go modules..."
go mod tidy

echo ""
echo "Verifying dependencies..."
go mod verify

echo ""
echo "Building to verify..."
GOMAXPROCS=3 go build -o /tmp/catalogizer-test ./... 2>&1 | head -20

echo ""
echo "=== Submodules Wired Successfully ==="
echo ""
echo "Next steps:"
echo "1. Import modules in your code"
echo "2. Run tests: go test ./..."
echo "3. Commit changes: git add go.mod go.sum"
