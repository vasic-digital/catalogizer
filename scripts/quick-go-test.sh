#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
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
