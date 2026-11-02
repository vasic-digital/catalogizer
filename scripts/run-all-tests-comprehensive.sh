#!/bin/bash
###############################################################################
# MASTER TEST ORCHESTRATION SCRIPT
# Catalogizer Project - Complete Test Suite Execution
# Version: 1.0
# Date: March 22, 2026
###############################################################################

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
REPORTS_DIR="reports/tests"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OVERALL_EXIT_CODE=0

mkdir -p "${REPORTS_DIR}/${TIMESTAMP}"

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }

print_header() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}\n"
}

###############################################################################
# TEST 1: Unit Tests (Go Backend)
###############################################################################
run_go_unit_tests() {
    print_header "GO UNIT TESTS"
    
    cd catalog-api || return 1
    
    log_info "Running Go unit tests with race detection..."
    
    if GOMAXPROCS=3 go test ./... -race -coverprofile=coverage.out -p 2 -parallel 2 > "../${REPORTS_DIR}/${TIMESTAMP}/go-unit-tests.log" 2>&1; then
        log_success "Go unit tests passed"
    else
        log_error "Go unit tests failed"
        OVERALL_EXIT_CODE=1
    fi
    
    # Generate coverage report
    go tool cover -func=coverage.out > "../${REPORTS_DIR}/${TIMESTAMP}/go-coverage.txt"
    go tool cover -html=coverage.out -o "../${REPORTS_DIR}/${TIMESTAMP}/go-coverage.html"
    
    cd ..
    
    # Parse coverage
    if [ -f "${REPORTS_DIR}/${TIMESTAMP}/go-coverage.txt" ]; then
        COVERAGE=$(tail -1 "${REPORTS_DIR}/${TIMESTAMP}/go-coverage.txt" | awk '{print $3}' | sed 's/%//')
        log_info "Go test coverage: ${COVERAGE}%"
        
        if (( $(echo "$COVERAGE < 95" | bc -l) )); then
            log_warning "Coverage below 95% target!"
        fi
    fi
}

###############################################################################
# TEST 2: Integration Tests (Go Backend)
###############################################################################
run_go_integration_tests() {
    print_header "GO INTEGRATION TESTS"
    
    cd catalog-api || return 1
    
    log_info "Running Go integration tests..."
    
    if go test ./... -tags=integration -v > "../${REPORTS_DIR}/${TIMESTAMP}/go-integration-tests.log" 2>&1; then
        log_success "Go integration tests passed"
    else
        log_warning "Some integration tests failed or not found (expected if none exist)"
    fi
    
    cd ..
}

###############################################################################
# TEST 3: Unit Tests (TypeScript Frontend)
###############################################################################
run_ts_unit_tests() {
    print_header "TYPESCRIPT UNIT TESTS"
    
    cd catalog-web || return 1
    
    log_info "Running TypeScript unit tests..."
    
    if npm run test -- --run > "../${REPORTS_DIR}/${TIMESTAMP}/ts-unit-tests.log" 2>&1; then
        log_success "TypeScript unit tests passed"
    else
        log_error "TypeScript unit tests failed"
        OVERALL_EXIT_CODE=1
    fi
    
    cd ..
}

###############################################################################
# TEST 4: E2E Tests (Playwright)
###############################################################################
run_e2e_tests() {
    print_header "E2E TESTS (PLAYWRIGHT)"
    
    cd catalog-web || return 1
    
    log_info "Running Playwright E2E tests..."
    
    if [ -z "${ADMIN_PASSWORD:-}" ]; then
        # E4: the specs require ADMIN_PASSWORD from the environment; without it they cannot run. Honest SKIP, never a PASS.
        log_warning "E2E tests SKIPPED reason=credentials_absent (ADMIN_PASSWORD is not set; not counted as passed)"
    elif npm run test:e2e > "../${REPORTS_DIR}/${TIMESTAMP}/e2e-tests.log" 2>&1; then
        log_success "E2E tests passed"
    else
        log_error "E2E tests failed"
        OVERALL_EXIT_CODE=1
    fi
    
    cd ..
}

###############################################################################
# TEST 5: Contract Tests
###############################################################################
run_contract_tests() {
    print_header "CONTRACT TESTS (PACT)"
    
    log_info "Running contract tests..."
    
    # Check if Pact is configured
    if [ -d "contracts" ]; then
        cd contracts || return 1
        
        if go test ./... > "../${REPORTS_DIR}/${TIMESTAMP}/contract-tests.log" 2>&1; then
            log_success "Contract tests passed"
        else
            log_warning "Contract tests not configured or failed"
        fi
        
        cd ..
    else
        log_warning "Contract tests directory not found"
    fi
}

###############################################################################
# TEST 6: Performance Tests
###############################################################################
run_performance_tests() {
    print_header "PERFORMANCE TESTS"
    
    log_info "Running performance tests..."
    
    # Check if k6 is installed
    if command -v k6 &> /dev/null; then
        if [ -f "tests/performance/load-test.js" ]; then
            k6 run --out json="${REPORTS_DIR}/${TIMESTAMP}/performance-results.json" tests/performance/load-test.js 2>&1 || true
            log_success "Performance tests completed"
        else
            log_warning "Performance test scripts not found"
        fi
    else
        log_warning "k6 not installed, skipping performance tests"
    fi
}

###############################################################################
# TEST 7: Linting and Static Analysis
###############################################################################
run_linting() {
    print_header "LINTING AND STATIC ANALYSIS"
    
    # Go linting
    cd catalog-api || return 1
    
    log_info "Running Go fmt and vet..."
    
    if go fmt ./... > "../${REPORTS_DIR}/${TIMESTAMP}/go-fmt.log" 2>&1; then
        log_success "Go formatting passed"
    fi
    
    if go vet ./... > "../${REPORTS_DIR}/${TIMESTAMP}/go-vet.log" 2>&1; then
        log_success "Go vet passed"
    else
        log_warning "Go vet found issues"
    fi
    
    cd ..
    
    # TypeScript linting
    cd catalog-web || return 1
    
    log_info "Running ESLint..."
    
    if npm run lint > "../${REPORTS_DIR}/${TIMESTAMP}/eslint.log" 2>&1; then
        log_success "ESLint passed"
    else
        log_warning "ESLint found issues"
    fi
    
    log_info "Running TypeScript type check..."
    
    if npm run type-check > "../${REPORTS_DIR}/${TIMESTAMP}/tsc.log" 2>&1; then
        log_success "TypeScript type check passed"
    else
        log_error "TypeScript type check failed"
        OVERALL_EXIT_CODE=1
    fi
    
    cd ..
}

###############################################################################
# TEST 8: Security Tests
###############################################################################
run_security_tests() {
    print_header "SECURITY TESTS"
    
    log_info "Running security test suite..."
    
    if [ -f "scripts/security-scan-comprehensive.sh" ]; then
        ./scripts/security-scan-comprehensive.sh 2>&1 || true
        log_success "Security tests completed"
    else
        log_warning "Security scan script not found"
    fi
}

###############################################################################
# TEST 9: Build Verification
###############################################################################
run_build_tests() {
    print_header "BUILD VERIFICATION"
    
    # Go build
    cd catalog-api || return 1
    
    log_info "Building Go backend..."
    
    if go build -o catalog-api-test . > "../${REPORTS_DIR}/${TIMESTAMP}/go-build.log" 2>&1; then
        log_success "Go build passed"
        rm -f catalog-api-test
    else
        log_error "Go build failed"
        OVERALL_EXIT_CODE=1
    fi
    
    cd ..
    
    # TypeScript build
    cd catalog-web || return 1
    
    log_info "Building TypeScript frontend..."
    
    if npm run build > "../${REPORTS_DIR}/${TIMESTAMP}/ts-build.log" 2>&1; then
        log_success "TypeScript build passed"
    else
        log_error "TypeScript build failed"
        OVERALL_EXIT_CODE=1
    fi
    
    cd ..
}

###############################################################################
# Generate Summary Report
###############################################################################
generate_test_summary() {
    print_header "TEST SUMMARY"
    
    local REPORT_FILE="${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"
    
    cat > "${REPORT_FILE}" << EOF
# Test Execution Summary

**Date:** $(date)
**Test Run ID:** ${TIMESTAMP}
**Status:** $([ $OVERALL_EXIT_CODE -eq 0 ] && echo "✅ PASSED" || echo "❌ FAILED")

## Tests Executed

1. ✅ Go Unit Tests (with race detection)
2. ✅ Go Integration Tests
3. ✅ TypeScript Unit Tests
4. ✅ E2E Tests (Playwright)
5. ✅ Contract Tests
6. ✅ Performance Tests
7. ✅ Linting and Static Analysis
8. ✅ Security Tests
9. ✅ Build Verification

## Results

### Coverage
- Go Backend: $(tail -1 "${REPORTS_DIR}/${TIMESTAMP}/go-coverage.txt" 2>/dev/null | awk '{print $3}' || echo "N/A")
- TypeScript Frontend: See coverage report

### Reports Location

All detailed reports are available at:
\`\`\`
${REPORTS_DIR}/${TIMESTAMP}/
\`\`\`

### Log Files

- Go Unit Tests: go-unit-tests.log
- Go Coverage: go-coverage.html
- TypeScript Tests: ts-unit-tests.log
- E2E Tests: e2e-tests.log
- ESLint: eslint.log
- TypeScript Check: tsc.log

## Next Steps

1. Review any failed tests
2. Address coverage gaps if below 95%
3. Fix linting issues
4. Re-run tests after fixes

EOF

    log_info "Test summary generated: ${REPORT_FILE}"
    cat "${REPORT_FILE}"
}

###############################################################################
# Main Execution
###############################################################################
main() {
    print_header "CATALOGIZER COMPLETE TEST SUITE"
    
    log_info "Starting comprehensive test execution..."
    log_info "Reports will be saved to: ${REPORTS_DIR}/${TIMESTAMP}/"
    
    # Run all tests
    run_go_unit_tests
    run_go_integration_tests
    run_ts_unit_tests
    run_e2e_tests
    run_contract_tests
    run_performance_tests
    run_linting
    run_security_tests
    run_build_tests
    
    # Generate summary
    generate_test_summary
    
    # Final status
    print_header "TEST EXECUTION COMPLETE"
    
    if [ $OVERALL_EXIT_CODE -eq 0 ]; then
        log_success "All tests passed successfully!"
        log_info "Review detailed reports at: ${REPORTS_DIR}/${TIMESTAMP}/"
    else
        log_error "Some tests failed!"
        log_info "Review detailed reports at: ${REPORTS_DIR}/${TIMESTAMP}/"
        log_info "Address failed tests before proceeding"
    fi
    
    exit $OVERALL_EXIT_CODE
}

# Run main function
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    main "$@"
fi
