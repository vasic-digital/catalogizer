#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh

# Pre-Flight Check Script
# Runs all quality checks before committing code

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

check() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $2"
        PASS=$((PASS + 1))
    else
        echo -e "${RED}✗${NC} $2"
        FAIL=$((FAIL + 1))
    fi
}

echo "=========================================="
echo "Pre-Flight Quality Checks"
echo "=========================================="
echo ""

# Backend checks
echo "Backend (Go)..."
cd catalog-api

go fmt ./... > /dev/null 2>&1
check $? "Go formatting"

go vet ./... > /dev/null 2>&1
check $? "Go vet"

go build -o /tmp/catalogizer-test . > /dev/null 2>&1
check $? "Go build"

# Run short tests only
go test -short ./internal/logging/... > /dev/null 2>&1
check $? "Go tests (short)"

cd ..

# Frontend checks
echo ""
echo "Frontend (TypeScript/React)..."
cd catalog-web

npm run type-check > /dev/null 2>&1
check $? "TypeScript type check"

npm run lint > /dev/null 2>&1
check $? "ESLint"

npm run build > /dev/null 2>&1
check $? "Production build"

cd ..

# Security checks
echo ""
echo "Security..."

# Check for secrets
grep -r "password.*=.*\"" --include="*.go" catalog-api/ | grep -v "_test.go" | grep -v "example" > /tmp/secrets-check.txt 2>&1 || true
if [ ! -s /tmp/secrets-check.txt ]; then
    check 0 "No hardcoded passwords detected"
else
    check 1 "Potential hardcoded secrets found"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "=========================================="

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}All checks passed! Ready to commit.${NC}"
    exit 0
else
    echo -e "${RED}Some checks failed. Please fix before committing.${NC}"
    exit 1
fi
