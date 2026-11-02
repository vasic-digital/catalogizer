#!/bin/bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/../../scripts/lib/cgo_flags.sh"   # 2026-11-02 decision: cgo define for go-sqlcipher (scripts/lib/cgo_flags.sh)

# Comprehensive Protocol Testing Script
# Tests all supported protocols with mock servers and validates 100% success rate

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
TEST_TIMEOUT=300  # 5 minutes timeout
COVERAGE_THRESHOLD=80  # Minimum coverage percentage
TEST_RETRIES=3

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "INFO")
            echo -e "${BLUE}[INFO]${NC} $message"
            ;;
        "SUCCESS")
            echo -e "${GREEN}[SUCCESS]${NC} $message"
            ;;
        "WARNING")
            echo -e "${YELLOW}[WARNING]${NC} $message"
            ;;
        "ERROR")
            echo -e "${RED}[ERROR]${NC} $message"
            ;;
    esac
}

# Function to check if required tools are available
check_dependencies() {
    print_status "INFO" "Checking dependencies..."

    local missing_deps=()

    if ! command -v go >/dev/null 2>&1; then
        missing_deps+=("go")
    fi

    if ! command -v golangci-lint >/dev/null 2>&1; then
        print_status "WARNING" "golangci-lint not found, installing..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_status "ERROR" "Missing dependencies: ${missing_deps[*]}"
        print_status "INFO" "Please install the missing dependencies and try again."
        exit 1
    fi

    print_status "SUCCESS" "All dependencies are available"
}

# Function to run linting
run_linting() {
    print_status "INFO" "Running Go linting..."

    if golangci-lint run --timeout=5m ./...; then
        print_status "SUCCESS" "Linting passed"
    else
        print_status "ERROR" "Linting failed"
        return 1
    fi
}

# Function to run unit tests
run_unit_tests() {
    print_status "INFO" "Running unit tests..."

    local test_output
    if test_output=$(go test -v -timeout=${TEST_TIMEOUT}s ./internal/... 2>&1); then
        print_status "SUCCESS" "Unit tests passed"
        echo "$test_output" | grep -E "(PASS|FAIL)"
        return 0
    else
        print_status "ERROR" "Unit tests failed"
        echo "$test_output"
        return 1
    fi
}

# Function to run integration tests
run_integration_tests() {
    print_status "INFO" "Running integration tests with mock servers..."

    local test_output
    local attempt=1

    while [ $attempt -le $TEST_RETRIES ]; do
        print_status "INFO" "Integration test attempt $attempt of $TEST_RETRIES"

        if test_output=$(go test -v -timeout=${TEST_TIMEOUT}s ./tests/integration/... 2>&1); then
            print_status "SUCCESS" "Integration tests passed on attempt $attempt"
            echo "$test_output" | grep -E "(PASS|FAIL)"
            return 0
        else
            print_status "WARNING" "Integration tests failed on attempt $attempt"
            if [ $attempt -eq $TEST_RETRIES ]; then
                print_status "ERROR" "Integration tests failed after $TEST_RETRIES attempts"
                echo "$test_output"
                return 1
            fi
            attempt=$((attempt + 1))
            sleep 2
        fi
    done
}

# Function to run protocol-specific tests
run_protocol_tests() {
    print_status "INFO" "Running protocol-specific connectivity tests..."

    local protocols=("SMB" "FTP" "NFS" "WebDAV" "Local")
    local failed_protocols=()

    for protocol in "${protocols[@]}"; do
        print_status "INFO" "Testing $protocol protocol..."

        local test_output
        if test_output=$(go test -v -timeout=60s -run "Test.*${protocol}.*" ./tests/integration/... 2>&1); then
            print_status "SUCCESS" "$protocol protocol tests passed"
        else
            print_status "ERROR" "$protocol protocol tests failed"
            failed_protocols+=("$protocol")
            echo "$test_output" | grep -E "(FAIL|ERROR)"
        fi
    done

    if [ ${#failed_protocols[@]} -eq 0 ]; then
        print_status "SUCCESS" "All protocol tests passed"
        return 0
    else
        print_status "ERROR" "Failed protocols: ${failed_protocols[*]}"
        return 1
    fi
}

# Function to run edge case and error condition tests
run_edge_case_tests() {
    print_status "INFO" "Running edge case and error condition tests..."

    local test_output
    if test_output=$(go test -v -timeout=120s -run "TestEdgeCases" ./tests/integration/... 2>&1); then
        print_status "SUCCESS" "Edge case tests passed"
        echo "$test_output" | grep -E "(PASS|FAIL)"
        return 0
    else
        print_status "ERROR" "Edge case tests failed"
        echo "$test_output"
        return 1
    fi
}

# Function to run coverage analysis
run_coverage_analysis() {
    print_status "INFO" "Running test coverage analysis..."

    local coverage_file="coverage.out"
    local coverage_html="coverage.html"

    # Run tests with coverage
    if go test -coverprofile="$coverage_file" ./...; then
        # Generate coverage report
        local coverage_percent
        coverage_percent=$(go tool cover -func="$coverage_file" | grep "total:" | awk '{print $3}' | sed 's/%//')

        # Generate HTML coverage report
        go tool cover -html="$coverage_file" -o "$coverage_html"

        print_status "INFO" "Test coverage: ${coverage_percent}%"
        print_status "INFO" "Coverage report generated: $coverage_html"

        # Check if coverage meets threshold
        if (( $(echo "$coverage_percent >= $COVERAGE_THRESHOLD" | bc -l) )); then
            print_status "SUCCESS" "Coverage threshold met (${coverage_percent}% >= ${COVERAGE_THRESHOLD}%)"
        else
            print_status "WARNING" "Coverage below threshold (${coverage_percent}% < ${COVERAGE_THRESHOLD}%)"
        fi

        return 0
    else
        print_status "ERROR" "Coverage analysis failed"
        return 1
    fi
}

# Function to run performance benchmarks
run_benchmarks() {
    print_status "INFO" "Running performance benchmarks..."

    local benchmark_output
    if benchmark_output=$(go test -bench=. -benchmem ./internal/services/... 2>&1); then
        print_status "SUCCESS" "Benchmarks completed"
        echo "$benchmark_output" | grep -E "(Benchmark|PASS)"
        return 0
    else
        print_status "ERROR" "Benchmarks failed"
        echo "$benchmark_output"
        return 1
    fi
}

# Function to validate mock servers
validate_mock_servers() {
    print_status "INFO" "Validating mock server implementations..."

    local test_output
    if test_output=$(go test -v -timeout=60s -run "TestMock.*" ./tests/mocks/... 2>&1); then
        print_status "SUCCESS" "Mock server validation passed"
        return 0
    else
        print_status "ERROR" "Mock server validation failed"
        echo "$test_output"
        return 1
    fi
}

# Function to run stress tests
run_stress_tests() {
    print_status "INFO" "Running stress tests..."

    local test_output
    if test_output=$(go test -v -timeout=180s -run "TestConcurrent.*" ./tests/integration/... 2>&1); then
        print_status "SUCCESS" "Stress tests passed"
        return 0
    else
        print_status "ERROR" "Stress tests failed"
        echo "$test_output"
        return 1
    fi
}

# Function to generate test report
generate_test_report() {
    print_status "INFO" "Generating comprehensive test report..."

    local report_file="test_report_$(date +%Y%m%d_%H%M%S).md"

    cat > "$report_file" << EOF
# Catalogizer Protocol Testing Report

**Generated:** $(date)
**Test Suite Version:** 1.0
**Go Version:** $(go version)

## Test Results Summary

EOF

    # Add test results to report
    if [ -f "test_results.log" ]; then
        echo "### Detailed Test Results" >> "$report_file"
        echo '```' >> "$report_file"
        cat "test_results.log" >> "$report_file"
        echo '```' >> "$report_file"
    fi

    # Add coverage information
    if [ -f "coverage.out" ]; then
        echo "### Coverage Analysis" >> "$report_file"
        echo '```' >> "$report_file"
        go tool cover -func=coverage.out >> "$report_file"
        echo '```' >> "$report_file"
    fi

    print_status "SUCCESS" "Test report generated: $report_file"
}

# Main execution function
main() {
    print_status "INFO" "Starting comprehensive protocol testing suite"
    print_status "INFO" "=========================================="

    local start_time=$(date +%s)
    local failed_steps=()

    # Change to script directory
    cd "$(dirname "$0")/.."

    # Create logs directory
    mkdir -p logs

    # Redirect all output to log file
    exec > >(tee -a "logs/test_run_$(date +%Y%m%d_%H%M%S).log")
    exec 2>&1

    # Run all test phases
    local test_phases=(
        "check_dependencies"
        "run_linting"
        "validate_mock_servers"
        "run_unit_tests"
        "run_integration_tests"
        "run_protocol_tests"
        "run_edge_case_tests"
        "run_stress_tests"
        "run_coverage_analysis"
        "run_benchmarks"
    )

    local passed_phases=0
    local total_phases=${#test_phases[@]}

    for phase in "${test_phases[@]}"; do
        print_status "INFO" "Executing phase: $phase"

        if $phase; then
            print_status "SUCCESS" "Phase $phase completed successfully"
            passed_phases=$((passed_phases + 1))
        else
            print_status "ERROR" "Phase $phase failed"
            failed_steps+=("$phase")
        fi

        print_status "INFO" "Progress: $passed_phases/$total_phases phases completed"
        echo "----------------------------------------"
    done

    # Calculate success rate
    local success_rate=$(echo "scale=2; $passed_phases * 100 / $total_phases" | bc)

    # Generate final report
    generate_test_report

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    print_status "INFO" "=========================================="
    print_status "INFO" "Comprehensive testing completed"
    print_status "INFO" "Duration: ${duration} seconds"
    print_status "INFO" "Success rate: ${success_rate}% ($passed_phases/$total_phases phases)"

    if [ ${#failed_steps[@]} -eq 0 ]; then
        print_status "SUCCESS" "🎉 ALL TESTS PASSED! 100% SUCCESS RATE ACHIEVED!"
        exit 0
    else
        print_status "ERROR" "❌ Some tests failed. Failed phases: ${failed_steps[*]}"
        print_status "INFO" "Review the logs and test report for details"
        exit 1
    fi
}

# Script usage information
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -t, --timeout SECONDS   Set test timeout (default: 300)"
    echo "  -c, --coverage PERCENT  Set coverage threshold (default: 80)"
    echo "  -r, --retries COUNT     Set test retry count (default: 3)"
    echo ""
    echo "Examples:"
    echo "  $0                      # Run all tests with default settings"
    echo "  $0 -t 600 -c 90        # Run with 10min timeout and 90% coverage"
    echo "  $0 --retries 5          # Run with 5 retries for flaky tests"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -t|--timeout)
            TEST_TIMEOUT="$2"
            shift 2
            ;;
        -c|--coverage)
            COVERAGE_THRESHOLD="$2"
            shift 2
            ;;
        -r|--retries)
            TEST_RETRIES="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Run main function
main "$@"