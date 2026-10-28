#!/usr/bin/env bash
# Test: internal/pathsafe builds and vets for every shipped GOOS (the Windows
# catalog-api release is built by scripts/build-all-releases.sh).
# SKIPs (exit 0, loudly) only when no Go toolchain is installed.
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command -v go >/dev/null 2>&1 || { echo "SKIP - no go toolchain (not a PASS)"; exit 0; }
fail=0
for os in linux windows darwin; do
    if out="$(cd "$ROOT/catalog-api" && GOFLAGS=-mod=mod GOOS=$os go vet ./internal/pathsafe 2>&1)"; then
        echo "ok   - GOOS=$os vet internal/pathsafe"
    else
        echo "FAIL - GOOS=$os vet internal/pathsafe"; echo "$out" | head -5; fail=1
    fi
done
[ "$fail" -eq 0 ] && echo PASS || echo FAILED
exit $fail
