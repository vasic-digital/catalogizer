#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
# Stress Test Runner - runs all stress tests without -short flag
set -e
echo "Running stress tests (this may take several minutes)..."
cd catalog-api
GOMAXPROCS=3 go test ./tests/stress/... -v -count=1 -timeout 30m -p 1 -parallel 2
echo "Stress tests complete."
