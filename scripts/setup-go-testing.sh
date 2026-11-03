#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
# setup-go-testing.sh - Comprehensive Go backend test infrastructure setup
# Sets up test utilities, mock helpers, and example tests for Catalogizer API

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CATALOG_API_DIR="$PROJECT_ROOT/catalog-api"

echo "🔧 Setting up Go backend test infrastructure..."

# Create test utilities directory
mkdir -p "$CATALOG_API_DIR/internal/tests/testutils"
mkdir -p "$CATALOG_API_DIR/internal/tests/examples"

echo "✅ Created test utilities directories"

# Install test dependencies if needed
echo "📦 Checking test dependencies..."
cd "$CATALOG_API_DIR"
if ! go list github.com/DATA-DOG/go-sqlmock 2>/dev/null | grep -q go-sqlmock; then
    echo "Installing go-sqlmock..."
    go get github.com/DATA-DOG/go-sqlmock
fi

if ! go list github.com/stretchr/testify 2>/dev/null | grep -q testify; then
    echo "Installing testify..."
    go get github.com/stretchr/testify
fi

echo "✅ Test dependencies installed"

# Create test coverage script
cat > "$PROJECT_ROOT/scripts/run-go-tests.sh" << 'EOF'
#!/bin/bash
# run-go-tests.sh - Run Go backend tests with coverage analysis

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CATALOG_API_DIR="$PROJECT_ROOT/catalog-api"
REPORTS_DIR="$PROJECT_ROOT/reports"
COVERAGE_DIR="$REPORTS_DIR/coverage"

echo "🧪 Running Go backend tests..."

# Create reports directory
mkdir -p "$COVERAGE_DIR"

# Change to catalog-api directory
cd "$CATALOG_API_DIR"

# Run tests with resource limits (respecting 30-40% host resource limits)
echo "📊 Running tests with resource limits..."
GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -coverprofile="$COVERAGE_DIR/coverage.out"

# Generate coverage report
echo "📈 Generating coverage report..."
go tool cover -func="$COVERAGE_DIR/coverage.out" -o "$COVERAGE_DIR/coverage.txt"

# Show summary
echo "📋 Coverage Summary:"
cat "$COVERAGE_DIR/coverage.txt" | tail -5

# Generate HTML coverage report
go tool cover -html="$COVERAGE_DIR/coverage.out" -o "$COVERAGE_DIR/coverage.html"

echo "✅ Tests completed. Coverage report: $COVERAGE_DIR/coverage.html"
echo "📊 Full coverage: $COVERAGE_DIR/coverage.txt"
EOF

chmod +x "$PROJECT_ROOT/scripts/run-go-tests.sh"

echo "✅ Created test runner script: scripts/run-go-tests.sh"

# Create quick test script
cat > "$PROJECT_ROOT/scripts/quick-go-test.sh" << 'EOF'
#!/bin/bash
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
EOF

chmod +x "$PROJECT_ROOT/scripts/quick-go-test.sh"

echo "✅ Created quick test script: scripts/quick-go-test.sh"

echo ""
echo "🎉 Go test infrastructure setup complete!"
echo ""
echo "Available commands:"
echo "  ./scripts/run-go-tests.sh    - Run all tests with coverage"
echo "  ./scripts/quick-go-test.sh   - Quick test for current directory"
echo "  ./scripts/setup-go-testing.sh - Re-run this setup"
echo ""
echo "Test utilities available in:"
echo "  catalog-api/internal/tests/testutils/"
echo "  catalog-api/internal/tests/examples/"
echo ""
echo "Next steps:"
echo "  1. Review example tests in catalog-api/internal/tests/examples/"
echo "  2. Add tests for repository layer (currently 0% coverage)"
echo "  3. Add tests for service layer (currently 0% coverage)"
echo "  4. Run ./scripts/run-go-tests.sh to establish baseline coverage"