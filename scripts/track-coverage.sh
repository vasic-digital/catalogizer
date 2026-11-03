#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
#
# Coverage Tracking Script
# Tracks test coverage over time
#

set -e

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
COVERAGE_DIR="reports/coverage"
mkdir -p "${COVERAGE_DIR}"

echo "=== Tracking Test Coverage ==="
echo "Timestamp: ${TIMESTAMP}"
echo ""

# Go coverage
echo "[1/4] Collecting Go coverage..."
cd catalog-api 2>/dev/null || true
if [ -f go.mod ]; then
    GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -coverprofile=../${COVERAGE_DIR}/go-coverage-${TIMESTAMP}.out 2>/dev/null || true
    if [ -f ../${COVERAGE_DIR}/go-coverage-${TIMESTAMP}.out ]; then
        go tool cover -func=../${COVERAGE_DIR}/go-coverage-${TIMESTAMP}.out | grep total | awk '{print "Total Go coverage: " $3}'
    fi
fi
cd .. 2>/dev/null || true

# Try to get coverage value
GO_COVERAGE="0"
if [ -f ${COVERAGE_DIR}/go-coverage-${TIMESTAMP}.out ]; then
    GO_COVERAGE=$(cd catalog-api && go tool cover -func=../${COVERAGE_DIR}/go-coverage-${TIMESTAMP}.out 2>/dev/null | grep total | awk '{print $3}' | sed 's/%//' || echo "0")
fi

echo "      Go Coverage: ${GO_COVERAGE}%"

# Generate coverage report
echo ""
echo "=== Coverage Summary ==="
echo "Go Backend:      ${GO_COVERAGE}%"
echo ""

# Save to history
echo "${TIMESTAMP},${GO_COVERAGE},0,0,0" >> ${COVERAGE_DIR}/coverage-history.csv

echo ""
echo "Coverage tracking complete!"
echo "History file: ${COVERAGE_DIR}/coverage-history.csv"
