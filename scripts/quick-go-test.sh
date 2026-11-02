#!/bin/bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/lib/cgo_flags.sh"   # 2026-11-02 decision: cgo define for go-sqlcipher (scripts/lib/cgo_flags.sh)
# quick-go-test.sh - Quick Go test for current directory

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT/catalog-api"

# Run tests for current package or specific test
if [ $# -eq 0 ]; then
    echo "🧪 Running tests in current directory..."
    go test ./...
else
    echo "🧪 Running specific test: $1"
    go test -v -run "$1" ./...
fi
