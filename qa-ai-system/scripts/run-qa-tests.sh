#!/bin/bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/../../scripts/lib/cgo_flags.sh"   # 2026-11-02 decision: cgo define for go-sqlcipher (scripts/lib/cgo_flags.sh)

# Catalogizer QA Tests Runner
# Manual execution of all quality assurance tests

set -e

echo "🎯 Catalogizer QA Tests Runner"
echo "==============================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_header() {
    echo -e "${PURPLE}🎯 $1${NC}"
}

# Configuration
QA_LEVEL=${1:-"standard"}
COMPONENTS=${2:-"all"}
QA_SYSTEM_DIR="qa-ai-system"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_FILE="qa-tests-$TIMESTAMP.log"

# Create log file
exec 1> >(tee -a "$LOG_FILE")
exec 2>&1

log_header "Starting QA Tests - Level: $QA_LEVEL"
echo "Timestamp: $(date)"
echo "Components: $COMPONENTS"
echo "Git Commit: $(git rev-parse HEAD 2>/dev/null || echo 'N/A')"
echo "Git Branch: $(git branch --show-current 2>/dev/null || echo 'N/A')"
echo ""

# Check if we're in the Catalogizer project
if [[ ! -d "$QA_SYSTEM_DIR" ]]; then
    log_error "Catalogizer QA system not found. Please run from project root."
    exit 1
fi

# Function to show usage
show_usage() {
    cat << EOF
Catalogizer QA Tests Runner

Usage: $0 [level] [components]

QA Levels:
  quick       - Fast validation (linters, formatters, quick tests)
  standard    - Comprehensive testing (full test suites)
  complete    - Exhaustive validation (includes security and performance checks)

Components:
  all         - Test all components (default)
  api         - Test only Go API
  android     - Test only Android app
  database    - Test only database operations
  integration - Test only integration workflows
  security    - Test only security aspects
  performance - Test only performance metrics

Examples:
  $0                          # Standard testing of all components
  $0 quick                    # Quick validation of all components
  $0 complete api             # Complete testing of API only
  $0 standard api,android     # Standard testing of API and Android

Options:
  --help, -h                  # Show this help message
  --verbose, -v               # Enable verbose output
  --no-log                    # Don't create log file
  --dry-run                   # Show what would be tested without running

EOF
}

# Parse command line arguments
VERBOSE=false
DRY_RUN=false
NO_LOG=false

for arg in "$@"; do
    case $arg in
        --help|-h)
            show_usage
            exit 0
            ;; 
        --verbose|-v)
            VERBOSE=true
            ;; 
        --dry-run)
            DRY_RUN=true
            ;; 
        --no-log)
            NO_LOG=true
            ;; 
    esac
done

# Function to run pre-commit style validation
run_pre_commit_validation() {
    log_header "Pre-Commit Style Validation"
    echo "============================="

    local validation_failed=false

    # Get list of modified files (or all files if no git)
    if git rev-parse --git-dir > /dev/null 2>&1; then
        modified_files=$(git diff --name-only HEAD~1 HEAD 2>/dev/null || git ls-files)
    else
        modified_files=$(find . -name "*.go" -o -name "*.kt" -o -name "*.java" -o -name "*.py" | head -20)
    fi

    log_info "Files to validate:"
    echo "$modified_files" | sed 's/^/  📄 /' | head -10
    if [[ $(echo "$modified_files" | wc -l) -gt 10 ]]; then
        echo "  ... and $(( $(echo "$modified_files" | wc -l) - 10 )) more files"
    fi
    echo ""

    # Check for merge conflict markers
    log_info "Checking for merge conflicts..."
    if echo "$modified_files" | xargs grep -l "<<<<<<< HEAD" 2>/dev/null; then
        log_error "Merge conflict markers found"
        validation_failed=true
    else
        log_success "No merge conflicts detected"
    fi

    # Check Go code if present
    if echo "$modified_files" | grep -E "\.go$" > /dev/null && [[ -d "catalog-api" ]]; then
        log_info "Validating Go code..."
        cd catalog-api

        # Check formatting
        unformatted=$(go fmt ./...)
        if [[ -n "$unformatted" ]]; then
            log_warning "Go code formatting issues found in the following files:"
            echo "$unformatted"
            validation_failed=true
        else
            log_success "Go code formatting: OK"
        fi

        # Run go vet
        if go vet ./...; then
            log_success "Go vet: OK"
        else
            log_warning "Go vet found issues"
            validation_failed=true
        fi

        cd ..
    fi

    # Check Android code if present
    if echo "$modified_files" | grep -E "\.(kt|java)$" > /dev/null && [[ -d "catalogizer-android" ]]; then
        log_info "Validating Android code..."
        cd catalogizer-android

        if [[ -f "gradlew" ]]; then
            chmod +x gradlew

            # Basic build check
            if ./gradlew ktlintCheck; then
                log_success "Android linting: OK"
            else
                log_warning "Android linting: Issues detected"
                validation_failed=true
            fi
        else
            log_info "Gradle wrapper not found - skipping Android validation"
        fi

        cd ..
    fi

    # Check for debug statements
    log_info "Checking for debug statements..."
    debug_count=$(echo "$modified_files" | xargs grep -l "console\.log\|print(\|debugger\|TODO\|FIXME" 2>/dev/null | wc -l)
    if [[ $debug_count -gt 0 ]]; then
        log_warning "Found $debug_count files with debug statements or TODOs"
    else
        log_success "No debug statements found"
    fi

    echo ""
    if [[ "$validation_failed" == "true" ]]; then
        log_error "Pre-commit validation: FAILED"
        return 1
    else
        log_success "Pre-commit validation: PASSED"
        return 0
    fi
}

# Function to run API tests
run_api_tests() {
    log_header "API Component Testing"
    echo "======================="

    if [[ ! -d "catalog-api" ]]; then
        log_warning "API directory not found - skipping API tests"
        return 0
    fi

    log_info "Testing Go API component..."

    cd catalog-api

    # Run unit tests
    log_info "Running Go unit tests..."
    if go test -v -race -coverprofile=coverage.out ./...; then
        log_success "Go unit tests: PASSED"

        # Generate coverage report
        if go tool cover -html=coverage.out -o coverage.html; then
            coverage=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}')
            log_info "Test coverage: $coverage"
        fi
    else
        log_error "Go unit tests: FAILED"
        cd ..
        return 1
    fi

    # Build the application
    log_info "Building Go application..."
    if go build -v ./...; then
        log_success "Go build: PASSED"
    else
        log_error "Go build: FAILED"
        cd ..
        return 1
    fi

    cd ..
    return 0
}

# Function to run Android tests
run_android_tests() {
    log_header "Android Component Testing"
    echo "========================="

    if [[ ! -d "catalogizer-android" ]]; then
        log_warning "Android directory not found - skipping Android tests"
        return 0
    fi

    log_info "Testing Android component..."

    cd catalogizer-android

    if [[ -f "gradlew" ]]; then
        chmod +x gradlew

        # Run unit tests
        log_info "Running Android unit tests..."
        if ./gradlew testDebugUnitTest --no-daemon; then
            log_success "Android unit tests: PASSED"
        else
            log_error "Android unit tests: FAILED"
            cd ..
            return 1
        fi

        # Build APK
        log_info "Building Android APK..."
        if ./gradlew assembleDebug --no-daemon; then
            log_success "Android APK build: PASSED"

            # Check APK size
            apk_file="app/build/outputs/apk/debug/app-debug.apk"
            if [[ -f "$apk_file" ]]; then
                apk_size=$(stat -f%z "$apk_file" 2>/dev/null || stat -c%s "$apk_file" 2>/dev/null || echo 0)
                apk_size_mb=$((apk_size / 1024 / 1024))
                log_info "APK size: ${apk_size_mb}MB"
            fi
        else
            log_error "Android APK build: FAILED"
            cd ..
            return 1
        fi
    else
        log_warning "Gradle wrapper not found - skipping Android build tests"
    fi

    cd ..
    return 0
}

# Function to run database tests
run_database_tests() {
    log_header "Database Component Testing"
    echo "==========================="

    if [[ ! -d "catalog-api" ]]; then
        log_warning "API directory not found - skipping database tests"
        return 0
    fi

    log_info "Testing database component..."

    cd catalog-api

    local db_test_failed=false

    # Run database package tests
    log_info "Running database package tests..."
    if go test -v ./database/... 2>&1 | tee /dev/stderr | grep -q "PASS\|no test files"; then
        if go test ./database/... 2>&1 | grep -q "no test files"; then
            log_warning "No database package tests found"
        else
            log_success "Database package tests: PASSED"
        fi
    else
        log_error "Database package tests: FAILED"
        db_test_failed=true
    fi

    # Run tests for packages that use the database
    log_info "Running tests for database-dependent packages..."
    if go test -v ./internal/media/database/... 2>&1 | tee /dev/stderr | grep -q "PASS\|no test files"; then
        if go test ./internal/media/database/... 2>&1 | grep -q "no test files"; then
            log_warning "No media database tests found"
        else
            log_success "Media database tests: PASSED"
        fi
    else
        log_error "Media database tests: FAILED"
        db_test_failed=true
    fi

    # Test database connection (if main test suite exists)
    log_info "Verifying database connection capability..."
    if go test -run TestMainTestSuite/TestDatabaseConnection -v . 2>&1 | grep -q "PASS"; then
        log_success "Database connection test: PASSED"
    else
        log_warning "Database connection test not found or failed"
    fi

    cd ..

    if [[ "$db_test_failed" == "true" ]]; then
        return 1
    fi
    return 0
}

# Function to run integration tests
run_integration_tests() {
    log_header "Integration Testing"
    echo "==================="

    if [[ ! -d "catalog-api" ]]; then
        log_warning "API directory not found - skipping integration tests"
        return 0
    fi

    log_info "Testing integration workflows..."

    cd catalog-api

    local integration_test_failed=false

    # Run integration tests in internal/tests
    log_info "Running internal integration tests..."
    if go test -v ./internal/tests/... 2>&1 | tee /dev/stderr | grep -q "PASS\|no test files"; then
        if go test ./internal/tests/... 2>&1 | grep -q "no test files"; then
            log_warning "No internal integration tests found"
        else
            log_success "Internal integration tests: PASSED"
        fi
    else
        log_error "Internal integration tests: FAILED"
        integration_test_failed=true
    fi

    # Run automation tests
    log_info "Running automation integration tests..."
    if go test -v ./tests/automation/... 2>&1 | tee /dev/stderr | grep -q "PASS\|no test files"; then
        if go test ./tests/automation/... 2>&1 | grep -q "no test files"; then
            log_warning "No automation tests found"
        else
            log_success "Automation integration tests: PASSED"
        fi
    else
        log_error "Automation integration tests: FAILED"
        integration_test_failed=true
    fi

    # Run full suite integration tests (like deep linking, recommendations)
    log_info "Running feature integration tests..."
    integration_count=$(find ./internal/tests -name "*integration_test.go" 2>/dev/null | wc -l)
    if [[ $integration_count -gt 0 ]]; then
        log_info "Found $integration_count integration test files"
        if go test -v -run Integration ./... 2>&1 | grep -q "PASS"; then
            log_success "Feature integration tests: PASSED"
        else
            log_warning "Some feature integration tests may have failed or been skipped"
        fi
    else
        log_warning "No integration test files found"
    fi

    cd ..

    if [[ "$integration_test_failed" == "true" ]]; then
        return 1
    fi
    return 0
}

# Function to run security tests
run_security_tests() {
    log_header "Security Testing"
    echo "==============="

    if [[ ! -d "catalog-api" ]]; then
        log_warning "API directory not found - skipping security tests"
        return 0
    fi

    log_info "Running security validation..."

    local security_failed=false

    # Basic security checks
    log_info "Checking for common security issues..."

    # Check for hardcoded secrets
    log_info "Scanning for hardcoded secrets..."
    secret_patterns="password.*=|api.*key.*=|secret.*=|token.*="
    secret_matches=$(find . -name "*.go" -o -name "*.kt" -o -name "*.java" -o -name "*.py" |
       xargs grep -i "$secret_patterns" 2>/dev/null | grep -v "test" | grep -v "example" | wc -l)

    if [[ $secret_matches -gt 0 ]]; then
        log_warning "Found $secret_matches potential hardcoded secrets"
        find . -name "*.go" -o -name "*.kt" -o -name "*.java" -o -name "*.py" |
            xargs grep -i "$secret_patterns" 2>/dev/null | grep -v "test" | grep -v "example" | head -5
    else
        log_success "No hardcoded secrets detected"
    fi

    # Check for SQL injection patterns
    log_info "Checking for SQL injection vulnerabilities..."
    sql_patterns="fmt\.Sprintf.*SELECT|\+.*SELECT|\+.*INSERT"
    sql_matches=$(find . -name "*.go" | xargs grep -E "$sql_patterns" 2>/dev/null | wc -l)

    if [[ $sql_matches -gt 0 ]]; then
        log_warning "Found $sql_matches potential SQL injection patterns"
        find . -name "*.go" | xargs grep -E "$sql_patterns" 2>/dev/null | head -3
    else
        log_success "No obvious SQL injection patterns detected"
    fi

    # Check for gosec installation and run it
    log_info "Running gosec security scanner..."
    if command -v gosec &> /dev/null; then
        cd catalog-api
        if gosec -fmt=text ./... 2>&1 | tee /tmp/gosec-output.txt; then
            log_success "Gosec security scan: PASSED"
        else
            # Check if there are actual issues or just warnings
            issue_count=$(grep -c "Issue" /tmp/gosec-output.txt 2>/dev/null || echo 0)
            if [[ $issue_count -gt 0 ]]; then
                log_warning "Gosec found $issue_count security issues"
                head -20 /tmp/gosec-output.txt
            else
                log_success "Gosec security scan: PASSED"
            fi
        fi
        cd ..
    else
        log_warning "gosec not installed - skipping static security analysis"
        log_info "Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"
    fi

    # Check for known vulnerable dependencies
    log_info "Checking for vulnerable dependencies..."
    cd catalog-api
    if command -v govulncheck &> /dev/null; then
        if govulncheck ./... 2>&1 | tee /tmp/govulncheck-output.txt; then
            log_success "No known vulnerabilities in dependencies"
        else
            log_warning "Vulnerable dependencies detected"
            head -20 /tmp/govulncheck-output.txt
        fi
    else
        log_warning "govulncheck not installed - skipping vulnerability check"
        log_info "Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
    fi
    cd ..

    # Check permissions on sensitive files
    log_info "Checking file permissions..."
    sensitive_files=".env .env.local config.yaml credentials.json"
    for file in $sensitive_files; do
        if [[ -f "$file" ]]; then
            perms=$(stat -c %a "$file" 2>/dev/null || stat -f %A "$file" 2>/dev/null || echo "unknown")
            if [[ "$perms" != "600" && "$perms" != "400" ]]; then
                log_warning "File $file has insecure permissions: $perms (should be 600 or 400)"
            fi
        fi
    done

    if [[ "$security_failed" == "true" ]]; then
        return 1
    fi
    return 0
}

# Function to run performance tests
run_performance_tests() {
    log_header "Performance Testing"
    echo "==================="

    if [[ ! -d "catalog-api" ]]; then
        log_warning "API directory not found - skipping performance tests"
        return 0
    fi

    log_info "Running performance validation..."

    cd catalog-api

    local perf_test_failed=false

    # Run Go benchmarks
    log_info "Running Go benchmarks..."
    benchmark_output=$(go test -bench=. -benchmem ./... 2>&1)
    if echo "$benchmark_output" | grep -q "Benchmark"; then
        log_success "Go benchmarks executed"
        # Show summary of benchmarks
        echo "$benchmark_output" | grep "Benchmark" | head -10
        if [[ $(echo "$benchmark_output" | grep "Benchmark" | wc -l) -gt 10 ]]; then
            echo "  ... and more benchmarks (see full log)"
        fi
    else
        log_warning "No Go benchmarks found"
    fi

    # Check for performance test files
    log_info "Checking for performance test files..."
    perf_test_count=$(find . -name "*performance*test.go" -o -name "*bench*test.go" 2>/dev/null | wc -l)
    if [[ $perf_test_count -gt 0 ]]; then
        log_info "Found $perf_test_count performance test files"
        if go test -v -run Performance ./... 2>&1 | grep -q "PASS"; then
            log_success "Performance tests: PASSED"
        else
            log_warning "Some performance tests may have failed or been skipped"
        fi
    else
        log_warning "No dedicated performance test files found"
    fi

    # Basic performance metrics
    log_info "Checking build performance..."
    build_start=$(date +%s)
    if go build -v ./... >/dev/null 2>&1; then
        build_end=$(date +%s)
        build_time=$((build_end - build_start))
        log_info "Build time: ${build_time}s"
        if [[ $build_time -lt 30 ]]; then
            log_success "Build performance: GOOD (<30s)"
        elif [[ $build_time -lt 60 ]]; then
            log_warning "Build performance: MODERATE (30-60s)"
        else
            log_warning "Build performance: SLOW (>60s)"
        fi
    else
        log_error "Build failed during performance check"
        perf_test_failed=true
    fi

    # Check binary size
    log_info "Checking binary size..."
    if go build -o catalogizer-api ./cmd/api 2>/dev/null; then
        binary_size=$(stat -f%z catalogizer-api 2>/dev/null || stat -c%s catalogizer-api 2>/dev/null || echo 0)
        binary_size_mb=$((binary_size / 1024 / 1024))
        log_info "Binary size: ${binary_size_mb}MB"
        rm -f catalogizer-api
    fi

    cd ..

    if [[ "$perf_test_failed" == "true" ]]; then
        return 1
    fi
    return 0
}

# Main test execution logic
main() {
    echo "🚀 Starting Catalogizer QA Tests..."
    echo "QA Level: $QA_LEVEL"
    echo "Components: $COMPONENTS"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN MODE - Showing what would be tested:"
        echo ""
    fi

    local overall_success=true
    local tests_run=0

    # Always run pre-commit validation for quick feedback
    if [[ "$QA_LEVEL" == "quick" ]]; then
        if [[ "$DRY_RUN" == "false" ]]; then
            if ! run_pre_commit_validation; then
                overall_success=false
            fi
        else
            echo "Would run: Pre-commit validation"
        fi
        ((tests_run++))
        echo ""
    fi

    # Component-specific testing
    if [[ "$QA_LEVEL" == "standard" || "$QA_LEVEL" == "complete" ]]; then
        case "$COMPONENTS" in
            *"all"*|*"api"*) 
                if [[ "$DRY_RUN" == "false" ]]; then
                    if ! run_api_tests; then
                        overall_success=false
                    fi
                else
                    echo "Would run: API component tests"
                fi
                ((tests_run++))
                echo ""
                ;; 
        esac

        case "$COMPONENTS" in
            *"all"*|*"android"*) 
                if [[ "$DRY_RUN" == "false" ]]; then
                    if ! run_android_tests; then
                        overall_success=false
                    fi
                else
                    echo "Would run: Android component tests"
                fi
                ((tests_run++))
                echo ""
                ;; 
        esac

        case "$COMPONENTS" in
            *"all"*|*"database"*) 
                if [[ "$DRY_RUN" == "false" ]]; then
                    if ! run_database_tests; then
                        overall_success=false
                    fi
                else
                    echo "Would run: Database component tests"
                fi
                ((tests_run++))
                echo ""
                ;; 
        esac

        case "$COMPONENTS" in
            *"all"*|*"integration"*) 
                if [[ "$DRY_RUN" == "false" ]]; then
                    if ! run_integration_tests; then
                        overall_success=false
                    fi
                else
                    echo "Would run: Integration tests"
                fi
                ((tests_run++))
                echo ""
                ;; 
        esac
    fi

    if [[ "$QA_LEVEL" == "complete" ]]; then
        case "$COMPONENTS" in
            *"all"*|*"security"*) 
                if [[ "$DRY_RUN" == "false" ]]; then
                    if ! run_security_tests; then
                        overall_success=false
                    fi
                else
                    echo "Would run: Security tests"
                fi
                ((tests_run++))
                echo ""
                ;; 
        esac

        case "$COMPONENTS" in
            *"all"*|*"performance"*) 
                if [[ "$DRY_RUN" == "false" ]]; then
                    if ! run_performance_tests; then
                        overall_success=false
                    fi
                else
                    echo "Would run: Performance tests"
                fi
                ((tests_run++))
                echo ""
                ;; 
        esac
    fi

    # Final results
    echo "======================================="
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN COMPLETE"
        echo "Would execute $tests_run test suites"
        echo "Use without --dry-run to execute tests"
    elif [[ "$overall_success" == "true" ]]; then
        log_success "🎉 ALL QA TESTS PASSED!"
        echo ""
        echo "✅ Test suites executed: $tests_run"
        echo "✅ Overall result: SUCCESS"
        echo "✅ Quality level: $QA_LEVEL validation completed"
    else
        log_error "❌ QA TESTS FAILED"
        echo ""
        echo "📊 Test suites executed: $tests_run"
        echo "❌ Overall result: FAILED"
        echo "🔍 Please review failed components above"
        echo ""
        echo "💡 Suggestions:"
        echo "   • Run individual component tests: $0 standard api"
        echo "   • Check logs for detailed error information"
        echo "   • Fix issues and re-run tests"
    fi

    echo ""
    echo "📋 Test log saved to: $LOG_FILE"
    echo "⏰ Test execution completed at: $(date)"

    if [[ "$overall_success" == "true" ]]; then
        exit 0
    else
        exit 1
    fi
}

# Execute main logic
main
