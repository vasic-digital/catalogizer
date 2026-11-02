#!/bin/bash
#
# CI/CD Pipeline Script
# Runs all checks and builds for continuous integration
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=== Catalogizer CI/CD Pipeline ==="
echo "Starting at: $(date)"
echo ""

FAILED=0

# Step 1: Lint
echo "Step 1: Running linters..."
cd "${PROJECT_ROOT}/catalog-api"
if gofmt -d . | grep -q '^'; then
    echo "  ✗ gofmt issues found"
    FAILED=1
else
    echo "  ✓ gofmt passed"
fi

if ! go vet ./... 2>&1 | grep -v "# github.com/mutecomm/go-sqlcipher" | head -5; then
    echo "  ✓ go vet passed"
else
    echo "  ✗ go vet issues found"
    FAILED=1
fi

echo ""

# Step 2: Tests
echo "Step 2: Running tests..."
if CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOMAXPROCS=3 go test -short ./... 2>&1 | tail -3; then
    echo "  ✓ Tests passed"
else
    echo "  ✗ Tests failed"
    FAILED=1
fi

echo ""

# Step 3: Coverage
echo "Step 3: Checking coverage..."
coverage=$(CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOMAXPROCS=3 go test -cover -short ./services/... 2>&1 | grep -o 'coverage: [0-9.]*%' | head -1 | grep -o '[0-9.]*')
echo "  Services coverage: ${coverage}%"

if (( $(echo "$coverage < 70" | bc -l) )); then
    echo "  ✗ Coverage below 70%"
    FAILED=1
else
    echo "  ✓ Coverage acceptable"
fi

echo ""

# Step 4: Build
echo "Step 4: Building..."
if CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOMAXPROCS=3 go build -o /tmp/catalogizer-test ./... 2>&1 | grep -v "sqlite3.c"; then
    echo "  ✗ Build failed"
    FAILED=1
else
    echo "  ✓ Build successful"
fi

echo ""

# Step 5: Security scan
echo "Step 5: Running security scans..."
if command -v gosec &> /dev/null; then
    if gosec -severity high ./... 2>&1 | grep -q "No issues found"; then
        echo "  ✓ gosec passed"
    else
        echo "  ⚠ gosec warnings found"
    fi
else
    echo "  ℹ gosec not installed"
fi

echo ""

# Summary
echo "=== Pipeline Summary ==="
if [ $FAILED -eq 0 ]; then
    echo "✓ All checks passed!"
    exit 0
else
    echo "✗ Some checks failed"
    exit 1
fi
