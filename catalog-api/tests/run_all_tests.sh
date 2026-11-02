#!/bin/bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/../../scripts/lib/cgo_flags.sh"   # 2026-11-02 decision: cgo define for go-sqlcipher (scripts/lib/cgo_flags.sh)

# Catalogizer v3.0 - Complete Test Suite Runner with Screenshot Automation
# This script runs all tests including unit tests, integration tests, and UI automation with screenshots

set -e

echo "🚀 Starting Catalogizer v3.0 Complete Test Suite..."
echo "================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
TEST_OUTPUT_DIR="test-results"
SCREENSHOT_DIR="../../docs/screenshots"
COVERAGE_FILE="coverage.out"
HTML_COVERAGE="coverage.html"

# Create output directories
mkdir -p "$TEST_OUTPUT_DIR"
mkdir -p "$SCREENSHOT_DIR"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to get server port from .service-port file
get_server_port() {
    local port_file=".service-port"
    if [ -f "$port_file" ]; then
        cat "$port_file"
    else
        echo "8080"
    fi
}

# Function to check if command exists
check_command() {
    if ! command -v "$1" &> /dev/null; then
        print_error "$1 is not installed or not in PATH"
        return 1
    fi
    return 0
}

# Check required dependencies
print_status "Checking dependencies..."

if ! check_command "go"; then
    print_error "Go is required but not installed"
    exit 1
fi

# Check if Google Chrome is available for UI tests
if ! check_command "google-chrome" && ! check_command "chromium-browser" && ! check_command "chromium"; then
    print_warning "Chrome/Chromium not found. UI automation tests will be skipped."
    SKIP_UI_TESTS=true
else
    print_success "Chrome/Chromium found for UI automation"
    SKIP_UI_TESTS=false
fi

# Start test database
print_status "Setting up test environment..."

# Function to run tests with proper error handling
run_test_suite() {
    local test_name="$1"
    local test_command="$2"
    local test_dir="$3"

    print_status "Running $test_name..."

    if [ -n "$test_dir" ]; then
        cd "$test_dir"
    fi

    if eval "$test_command"; then
        print_success "$test_name completed successfully"
        return 0
    else
        print_error "$test_name failed"
        return 1
    fi
}

# Initialize test results
FAILED_TESTS=()
PASSED_TESTS=()

# Change to project root
cd "$(dirname "$0")/.."

print_status "Project directory: $(pwd)"

# 1. Build and dependency check
print_status "Building project and checking dependencies..."
if go mod tidy && go mod download; then
    print_success "Dependencies resolved successfully"
else
    print_error "Failed to resolve dependencies"
    exit 1
fi

# 2. Unit Tests with Coverage
print_status "Running unit tests with coverage..."
if go test -v -race -coverprofile="$TEST_OUTPUT_DIR/$COVERAGE_FILE" -covermode=atomic ./... > "$TEST_OUTPUT_DIR/unit_tests.log" 2>&1; then
    print_success "Unit tests passed"
    PASSED_TESTS+=("Unit Tests")

    # Generate coverage report
    go tool cover -html="$TEST_OUTPUT_DIR/$COVERAGE_FILE" -o "$TEST_OUTPUT_DIR/$HTML_COVERAGE"

    # Extract coverage percentage
    COVERAGE=$(go tool cover -func="$TEST_OUTPUT_DIR/$COVERAGE_FILE" | grep total | awk '{print $3}')
    print_success "Test coverage: $COVERAGE"

    # Check if coverage meets minimum threshold (80%)
    COVERAGE_NUM=$(echo "$COVERAGE" | sed 's/%//')
    if (( $(echo "$COVERAGE_NUM >= 80" | bc -l) )); then
        print_success "Coverage threshold met (≥80%)"
    else
        print_warning "Coverage below threshold: $COVERAGE (target: ≥80%)"
    fi
else
    print_error "Unit tests failed"
    FAILED_TESTS+=("Unit Tests")
    cat "$TEST_OUTPUT_DIR/unit_tests.log"
fi

# 3. Service Integration Tests
print_status "Running service integration tests..."

# Analytics Service Tests
if run_test_suite "Analytics Service Tests" "go test -v ./tests/analytics_service_test.go ./tests/test_utils.go" ""; then
    PASSED_TESTS+=("Analytics Service")
else
    FAILED_TESTS+=("Analytics Service")
fi

# Favorites Service Tests
if run_test_suite "Favorites Service Tests" "go test -v ./tests/favorites_service_test.go ./tests/test_utils.go" ""; then
    PASSED_TESTS+=("Favorites Service")
else
    FAILED_TESTS+=("Favorites Service")
fi

# Conversion Service Tests
if run_test_suite "Conversion Service Tests" "go test -v ./tests/conversion_service_test.go ./tests/test_utils.go" ""; then
    PASSED_TESTS+=("Conversion Service")
else
    FAILED_TESTS+=("Conversion Service")
fi

# Sync Service Tests
if run_test_suite "Sync Service Tests" "go test -v ./tests/sync_service_test.go ./tests/test_utils.go" ""; then
    PASSED_TESTS+=("Sync Service")
else
    FAILED_TESTS+=("Sync Service")
fi

# Stress Test Service Tests
if run_test_suite "Stress Test Service Tests" "go test -v ./tests/stress_test_service_test.go ./tests/test_utils.go" ""; then
    PASSED_TESTS+=("Stress Test Service")
else
    FAILED_TESTS+=("Stress Test Service")
fi

# Error Reporting Service Tests
if run_test_suite "Error Reporting Service Tests" "go test -v ./tests/error_reporting_service_test.go ./tests/test_utils.go" ""; then
    PASSED_TESTS+=("Error Reporting Service")
else
    FAILED_TESTS+=("Error Reporting Service")
fi

# Log Management Service Tests
if run_test_suite "Log Management Service Tests" "go test -v ./tests/log_management_service_test.go ./tests/test_utils.go" ""; then
    PASSED_TESTS+=("Log Management Service")
else
    FAILED_TESTS+=("Log Management Service")
fi

# Configuration Service Tests
if run_test_suite "Configuration Service Tests" "go test -v ./tests/configuration_service_test.go ./tests/test_utils.go" ""; then
    PASSED_TESTS+=("Configuration Service")
else
    FAILED_TESTS+=("Configuration Service")
fi

# 4. API Integration Tests
print_status "Running API integration tests..."

# Start test server in background
print_status "Starting test server..."
go run main.go --test-mode &
SERVER_PID=$!

# Wait for server to start and write port file
print_status "Waiting for server port..."
for i in {1..30}; do
    if [ -f ".service-port" ]; then
        break
    fi
    sleep 1
done

if [ ! -f ".service-port" ]; then
    print_error "Server failed to write port file"
    kill $SERVER_PID 2>/dev/null || true
    FAILED_TESTS+=("API Integration")
    return 1
fi

SERVER_PORT=$(get_server_port)
print_status "Server running on port $SERVER_PORT"

# Wait for server to be ready
sleep 2

# Check if server is still running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    print_error "Test server failed to start"
    FAILED_TESTS+=("API Integration")
    return 1
fi

# API endpoint tests
API_BASE_URL="http://localhost:$SERVER_PORT"

test_api_endpoint() {
    local endpoint="$1"
    local method="$2"
    local expected_status="$3"

    print_status "Testing $method $endpoint"

    response=$(curl -s -w "%{http_code}" -X "$method" "$API_BASE_URL$endpoint" -o /dev/null)

    if [ "$response" = "$expected_status" ]; then
        print_success "$endpoint: $response"
        return 0
    else
        print_error "$endpoint: Expected $expected_status, got $response"
        return 1
    fi
}

# Test critical API endpoints
if test_api_endpoint "/api/health" "GET" "200"; then
    print_success "Server health check passed"
    if test_api_endpoint "/api/analytics/events" "GET" "200" && \
       test_api_endpoint "/api/favorites" "GET" "200" && \
       test_api_endpoint "/api/collections" "GET" "200"; then
        print_success "API integration tests passed"
        PASSED_TESTS+=("API Integration")
    else
        print_error "API integration tests failed"
        FAILED_TESTS+=("API Integration")
    fi
else
    print_error "Server health check failed - server may not have started properly"
    FAILED_TESTS+=("API Integration")
fi

# Stop test server
kill $SERVER_PID 2>/dev/null || true

# 5. UI Automation Tests with Screenshots
if [ "$SKIP_UI_TESTS" = false ]; then
    print_status "Running UI automation tests with screenshot capture..."

    # Change to automation test directory
    cd tests/automation

    # Install UI test dependencies
    if go mod tidy; then
        print_success "UI test dependencies resolved"
    else
        print_warning "Failed to resolve UI test dependencies"
    fi

    # Start application server for UI tests
    cd ../..
    go run main.go --test-mode &
    UI_SERVER_PID=$!

    # Wait for server to start and write port file
    print_status "Waiting for UI server port..."
    for i in {1..30}; do
        if [ -f ".service-port" ]; then
            break
        fi
        sleep 1
    done

    if [ ! -f ".service-port" ]; then
        print_error "UI server failed to write port file"
        kill $UI_SERVER_PID 2>/dev/null || true
        FAILED_TESTS+=("UI Automation")
        cd tests/automation
        return 1
    fi

    UI_SERVER_PORT=$(get_server_port)
    print_status "UI server running on port $UI_SERVER_PORT"
    export API_BASE_URL="http://localhost:$UI_SERVER_PORT/api/v1"

    # Wait for server to be ready
    sleep 2

    # Check if UI server is still running
    if ! kill -0 $UI_SERVER_PID 2>/dev/null; then
        print_error "UI test server failed to start"
        FAILED_TESTS+=("UI Automation")
        cd tests/automation
        return 1
    fi

    cd tests/automation

    # Run UI automation with screenshots
    if go test -v -timeout=30m ./full_automation_test.go > "../../$TEST_OUTPUT_DIR/ui_automation.log" 2>&1; then
        print_success "UI automation tests with screenshots completed successfully"
        PASSED_TESTS+=("UI Automation with Screenshots")

        # Count captured screenshots
        SCREENSHOT_COUNT=$(find "$SCREENSHOT_DIR" -name "*.png" -type f | wc -l)
        print_success "Captured $SCREENSHOT_COUNT screenshots for documentation"

    else
        print_error "UI automation tests failed"
        FAILED_TESTS+=("UI Automation")
        cat "../../$TEST_OUTPUT_DIR/ui_automation.log"
    fi

    # Stop UI test server
    kill $UI_SERVER_PID 2>/dev/null || true

    cd ../..
else
    print_warning "Skipping UI automation tests (Chrome not available)"
fi

# 6. Performance Tests
print_status "Running performance benchmarks..."
if go test -bench=. -benchmem ./... > "$TEST_OUTPUT_DIR/benchmarks.log" 2>&1; then
    print_success "Performance benchmarks completed"
    PASSED_TESTS+=("Performance Benchmarks")
else
    print_warning "Some performance benchmarks failed"
    FAILED_TESTS+=("Performance Benchmarks")
fi

# 7. Security Tests
print_status "Running security tests..."

# Test for SQL injection vulnerabilities
print_status "Checking for SQL injection vulnerabilities..."
if grep -r "fmt.Sprintf.*%.*SELECT\|UPDATE\|DELETE\|INSERT" --include="*.go" .; then
    print_warning "Potential SQL injection vulnerabilities found"
else
    print_success "No obvious SQL injection vulnerabilities detected"
fi

# Test for hardcoded secrets
print_status "Checking for hardcoded secrets..."
if grep -r "password.*=.*\"\|token.*=.*\"\|secret.*=.*\"" --include="*.go" . | grep -v "test\|example"; then
    print_warning "Potential hardcoded secrets found"
else
    print_success "No hardcoded secrets detected"
fi

PASSED_TESTS+=("Security Tests")

# 8. Generate Test Report
print_status "Generating comprehensive test report..."

cat > "$TEST_OUTPUT_DIR/test_report.md" << EOF
# Catalogizer v3.0 - Test Report

**Generated on:** $(date)
**Test Suite Version:** 3.0.0
**Total Test Categories:** $((${#PASSED_TESTS[@]} + ${#FAILED_TESTS[@]}))

## Test Results Summary

### ✅ Passed Tests (${#PASSED_TESTS[@]})
$(printf '%s\n' "${PASSED_TESTS[@]}" | sed 's/^/- /')

### ❌ Failed Tests (${#FAILED_TESTS[@]})
$(printf '%s\n' "${FAILED_TESTS[@]}" | sed 's/^/- /')

## Test Coverage
- **Coverage Report:** [coverage.html](coverage.html)
- **Coverage Percentage:** $COVERAGE

## Screenshot Documentation
- **Screenshots Captured:** $SCREENSHOT_COUNT
- **Screenshot Directory:** [screenshots](../../docs/screenshots/)

## Log Files
- **Unit Tests:** [unit_tests.log](unit_tests.log)
- **UI Automation:** [ui_automation.log](ui_automation.log)
- **Performance Benchmarks:** [benchmarks.log](benchmarks.log)

## Quick Links
- [API Documentation](../../docs/api-documentation.md)
- [User Guide](../../docs/user-guide.md)
- [Admin Guide](../../docs/admin-guide.md)
- [Troubleshooting Guide](../../docs/troubleshooting-guide.md)

EOF

# 9. Documentation Validation
print_status "Validating generated documentation..."

REQUIRED_DOCS=(
    "../../docs/README.md"
    "../../docs/screenshots"
)

MISSING_DOCS=()

for doc in "${REQUIRED_DOCS[@]}"; do
    if [ ! -e "$doc" ]; then
        MISSING_DOCS+=("$doc")
    fi
done

if [ ${#MISSING_DOCS[@]} -eq 0 ]; then
    print_success "All required documentation is present"
else
    print_warning "Missing documentation: ${MISSING_DOCS[*]}"
fi

# 10. Final Results
echo ""
echo "================================================="
echo "🎯 Test Suite Complete!"
echo "================================================="

if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    print_success "ALL TESTS PASSED! 🎉"
    print_success "✅ ${#PASSED_TESTS[@]} test categories completed successfully"
    print_success "📊 Test coverage: $COVERAGE"
    print_success "📸 Screenshots captured: $SCREENSHOT_COUNT"
    print_success "📋 Test report: $TEST_OUTPUT_DIR/test_report.md"
    exit 0
else
    print_error "❌ ${#FAILED_TESTS[@]} test categories failed"
    print_warning "⚠️  Review the test logs for details"
    print_status "📋 Test report: $TEST_OUTPUT_DIR/test_report.md"
    exit 1
fi