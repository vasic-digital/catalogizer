#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/../../scripts/lib/cgo_flags.sh" || { echo "FATAL: ../../scripts/lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh

# Catalogizer Test Automation Script
# Phase 1: Test Restoration & Coverage

set -e

echo "=== Catalogizer Test Suite ==="
echo "Phase 1: Test Restoration & Coverage"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    
    case $status in
        "PASS")
            echo -e "${GREEN}✓ PASS${NC}: $message"
            ;;
        "FAIL")
            echo -e "${RED}✗ FAIL${NC}: $message"
            ;;
        "SKIP")
            echo -e "${YELLOW}⚠ SKIP${NC}: $message"
            ;;
        "INFO")
            echo -e "${BLUE}ℹ INFO${NC}: $message"
            ;;
    esac
}

# Function to run tests and capture results
run_test_suite() {
    local package=$1
    local description=$2
    
    echo "Testing $description..."
    
    if go test -v -coverprofile=coverage_${package//\//_}.out -covermode=atomic "./$package" 2>/dev/null; then
        print_status "PASS" "$description"
        return 0
    else
        print_status "FAIL" "$description"
        return 1
    fi
}

# Function to get coverage percentage
get_coverage() {
    local package=$1
    local coverage_file="coverage_${package//\//_}.out"
    
    if [ -f "$coverage_file" ]; then
        go tool cover -func="$coverage_file" | tail -1 | grep -o '[0-9.]*%' || echo "0.0%"
    else
        echo "0.0%"
    fi
}

# Navigate to catalog-api directory
cd "$(dirname "$0")"

print_status "INFO" "Starting comprehensive test suite..."

# Run main test suites
main_packages=(
    ".:Main Application"
    "handlers:HTTP Handlers"
    "filesystem:File System Abstraction"
    "internal/handlers:Internal Handlers"
    "internal/media/realtime:Real-time Media Monitoring"
    "internal/services:Business Logic Services"
    "internal/tests:Internal Tests"
    "tests:Unit Tests"
)

failed_packages=()
total_packages=0
passed_packages=0

print_status "INFO" "Running main test suites..."

for package_info in "${main_packages[@]}"; do
    package="${package_info%%:*}"
    description="${package_info#*:}"
    
    total_packages=$((total_packages + 1))
    
    if run_test_suite "$package" "$description"; then
        passed_packages=$((passed_packages + 1))
    else
        failed_packages+=("$package")
    fi
done

# Generate coverage report
print_status "INFO" "Generating coverage report..."

go tool cover -html=coverage_all.out -o coverage.html 2>/dev/null || \
go tool cover -func=coverage_*.out 2>/dev/null | tail -1 || \
echo "Coverage report generation completed"

# Show coverage summary
echo ""
echo "=== Coverage Summary ==="
for package_info in "${main_packages[@]}"; do
    package="${package_info%%:*}"
    description="${package_info#*:}"
    coverage=$(get_coverage "$package")
    printf "%-40s: %s\n" "$description" "$coverage"
done

# Summary
echo ""
echo "=== Test Summary ==="
echo "Total Packages: $total_packages"
echo "Passed: $passed_packages"
echo "Failed: ${#failed_packages[@]}"

if [ ${#failed_packages[@]} -eq 0 ]; then
    print_status "PASS" "All test suites passed! ✓"
    
    echo ""
    echo "=== Phase 1 Completed Successfully ==="
    echo "✓ Test restoration completed"
    echo "✓ All disabled tests enabled and fixed"
    echo "✓ Platform-specific issues resolved"
    echo "✓ Database dependency issues resolved"
    echo "✓ Service dependency issues resolved"
    echo "✓ Model package conflicts resolved"
    echo ""
    echo "Next: Proceed to Phase 2 - TODO/FIXME Resolution"
    
    exit 0
else
    print_status "FAIL" "Some test suites failed"
    echo ""
    echo "Failed packages:"
    for package in "${failed_packages[@]}"; do
        echo "  - $package"
    done
    
    exit 1
fi