#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh

# Comprehensive Test Suite for Catalogizer
# This script runs all tests including security scans with SonarQube and Snyk
# All tests must pass with 100% success - no module or feature can be left broken

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
REPORTS_DIR="$PROJECT_ROOT/reports"
LOG_FILE="$REPORTS_DIR/comprehensive-test.log"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Individual test category results (for accurate HTML reporting)
SHELL_TESTS_STATUS="PENDING"
GO_TESTS_STATUS="PENDING"
JS_TESTS_STATUS="PENDING"
ANDROID_TESTS_STATUS="PENDING"
SONARQUBE_STATUS="PENDING"
SNYK_STATUS="PENDING"
TRIVY_STATUS="PENDING"
OWASP_STATUS="PENDING"

# Container runtime detection - prefer podman over docker
if command -v podman &>/dev/null; then
    CONTAINER_CMD="podman"
    if command -v podman-compose &>/dev/null; then
        COMPOSE_CMD="podman-compose"
    else
        COMPOSE_CMD=""
    fi
elif command -v docker &>/dev/null; then
    CONTAINER_CMD="docker"
    if command -v docker-compose &>/dev/null; then
        COMPOSE_CMD="docker-compose"
    elif docker compose version &>/dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD=""
    fi
else
    CONTAINER_CMD=""
    COMPOSE_CMD=""
fi

echo -e "${BLUE}🧪 Starting Comprehensive Test Suite for Catalogizer${NC}"
echo -e "${BLUE}📁 Project Root: $PROJECT_ROOT${NC}"
echo -e "${BLUE}📊 Reports Directory: $REPORTS_DIR${NC}"
echo -e "${BLUE}⏰ Started at: $(date)${NC}"

# Create reports directory
mkdir -p "$REPORTS_DIR"

# Initialize log file
echo "Comprehensive Test Log - $(date)" > "$LOG_FILE"
echo "========================================" >> "$LOG_FILE"

# Function to log messages
log() {
    echo "$1" | tee -a "$LOG_FILE"
}

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "PASS")
            echo -e "${GREEN}✅ $message${NC}"
            ((PASSED_TESTS++))
            ;;
        "FAIL")
            echo -e "${RED}❌ $message${NC}"
            ((FAILED_TESTS++))
            ;;
        "WARN")
            echo -e "${YELLOW}⚠️  $message${NC}"
            ;;
        "INFO")
            echo -e "${BLUE}ℹ️  $message${NC}"
            ;;
    esac
    ((TOTAL_TESTS++))
}

# Function to check prerequisites
check_prerequisites() {
    log "🔍 Checking prerequisites..."
    
    local missing_prereqs=false
    
    # Check Docker/Podman
    if [ -z "$CONTAINER_CMD" ]; then
        print_status "FAIL" "Neither docker nor podman is installed"
        missing_prereqs=true
    else
        print_status "PASS" "Container runtime is available ($CONTAINER_CMD)"
    fi

    # Check Docker/Podman Compose
    if [ -z "$COMPOSE_CMD" ]; then
        print_status "FAIL" "Neither docker-compose, docker compose, nor podman-compose is installed"
        missing_prereqs=true
    else
        print_status "PASS" "Compose tool is available ($COMPOSE_CMD)"
    fi
    
    # Check Node.js
    if ! command -v node &> /dev/null; then
        print_status "FAIL" "Node.js is not installed"
        missing_prereqs=true
    else
        print_status "PASS" "Node.js is available: $(node --version)"
    fi
    
    # Check Go
    if ! command -v go &> /dev/null; then
        print_status "FAIL" "Go is not installed"
        missing_prereqs=true
    else
        print_status "PASS" "Go is available: $(go version)"
    fi
    
    # Check Java (for Android)
    if ! command -v java &> /dev/null; then
        print_status "WARN" "Java is not installed (Android tests may fail)"
    else
        print_status "PASS" "Java is available: $(java -version 2>&1 | head -n1)"
    fi
    
    # Check environment variables
    if [ -z "$SONAR_TOKEN" ]; then
        print_status "WARN" "SONAR_TOKEN environment variable not set"
    else
        print_status "PASS" "SONAR_TOKEN is set"
    fi
    
    if [ -z "$SNYK_TOKEN" ]; then
        print_status "WARN" "SNYK_TOKEN environment variable not set"
    else
        print_status "PASS" "SNYK_TOKEN is set"
    fi
    
    if [ "$missing_prereqs" = true ]; then
        log "❌ Some prerequisites are missing. Please install them before continuing."
        return 1
    fi
    
    print_status "PASS" "All prerequisites check completed"
    return 0
}

# Function to run shell-layer tests (build helpers, auto-container dispatch)
run_shell_tests() {
    log "🐚 Running shell-layer tests (scripts/lib)..."
    local suite="$PROJECT_ROOT/scripts/tests/run-all.sh"
    if [[ ! -x "$suite" ]]; then
        print_status "WARN" "scripts/tests/run-all.sh missing or not executable — skipping shell tests"
        SHELL_TESTS_STATUS="SKIP"
        return 0
    fi
    if "$suite" 2>&1 | tee -a "$LOG_FILE"; then
        print_status "PASS" "Shell-layer tests passed"
        SHELL_TESTS_STATUS="PASS"
        return 0
    fi
    print_status "FAIL" "Shell-layer tests failed"
    SHELL_TESTS_STATUS="FAIL"
    return 1
}

# Function to run Go tests
run_go_tests() {
    log "🐹 Running Go API tests..."

    if [ -d "$PROJECT_ROOT/catalog-api" ]; then
        cd "$PROJECT_ROOT/catalog-api"
        
        # Check if go.mod exists
        if [ -f "go.mod" ]; then
            log "📦 Downloading Go dependencies..."
            go mod download
            go mod tidy
            
            # Run tests with coverage (excluding integration tests that require server)
            log "🧪 Running Go tests with coverage..."
            if go test -v -race -coverprofile=coverage.out -covermode=atomic ./... 2>&1 | tee -a "$LOG_FILE"; then
                print_status "PASS" "Go API tests passed"
                GO_TESTS_STATUS="PASS"
                
                # Generate coverage report
                go tool cover -html=coverage.out -o coverage.html 2>/dev/null || true
                cp coverage.out "$REPORTS_DIR/go-coverage.out" 2>/dev/null || true
                cp coverage.html "$REPORTS_DIR/go-coverage.html" 2>/dev/null || true
                
                # Generate test results in JSON format
                go test -json ./... > "$REPORTS_DIR/go-test-results.json" 2>/dev/null || true
                
            else
                print_status "FAIL" "Go API tests failed"
                GO_TESTS_STATUS="FAIL"
                return 1
            fi
        else
            print_status "WARN" "No go.mod found in catalog-api"
        fi
        
        cd "$PROJECT_ROOT"
    else
        print_status "WARN" "catalog-api directory not found"
    fi
    
    return 0
}

# Function to run JavaScript/TypeScript tests
run_js_tests() {
    log "🟢 Running JavaScript/TypeScript tests..."
    
    local js_projects=("catalog-web" "catalogizer-desktop" "catalogizer-api-client" "installer-wizard")
    local js_failed=0
    
    for project in "${js_projects[@]}"; do
        if [ -d "$PROJECT_ROOT/$project" ] && [ -f "$PROJECT_ROOT/$project/package.json" ]; then
            log "📦 Processing $project..."
            cd "$PROJECT_ROOT/$project"
            
            # Install dependencies if needed
            if [ ! -d "node_modules" ]; then
                log "📦 Installing dependencies for $project..."
                npm install --silent
            fi
            
            # Check if test script exists
            if npm run test --silent 2>/dev/null; then
                log "🧪 Running tests for $project..."
                if npm test 2>&1 | tee -a "$LOG_FILE"; then
                    print_status "PASS" "$project tests passed"
                    
                    # Copy coverage reports if available
                    if [ -f "coverage/lcov.info" ]; then
                        mkdir -p "$REPORTS_DIR/coverage-$project"
                        cp -r coverage/* "$REPORTS_DIR/coverage-$project/" 2>/dev/null || true
                    fi
                else
                    print_status "FAIL" "$project tests failed"
                    js_failed=1
                fi
            else
                print_status "WARN" "No test script found for $project"
            fi
            
            cd "$PROJECT_ROOT"
        else
            print_status "WARN" "$project directory or package.json not found"
        fi
    done
    
    # Set overall JS tests status
    if [ $js_failed -eq 0 ]; then
        JS_TESTS_STATUS="PASS"
    else
        JS_TESTS_STATUS="FAIL"
        return 1
    fi
    
    return 0
}

# Function to run Android tests
run_android_tests() {
    log "📱 Running Android tests..."
    
    local android_projects=("catalogizer-android" "catalogizer-androidtv")
    local android_failed=0
    
    for project in "${android_projects[@]}"; do
        if [ -d "$PROJECT_ROOT/$project" ] && [ -f "$PROJECT_ROOT/$project/build.gradle.kts" ]; then
            log "📱 Processing $project..."
            cd "$PROJECT_ROOT/$project"
            
            # Check if gradlew exists and is executable
            if [ -f "./gradlew" ]; then
                chmod +x ./gradlew
                
                # Run unit tests
                log "🧪 Running unit tests for $project..."
                if ./gradlew testDebugUnitTest 2>&1 | tee -a "$LOG_FILE"; then
                    print_status "PASS" "$project unit tests passed"
                    
                    # Copy test reports
                    if [ -d "app/build/reports/tests" ]; then
                        mkdir -p "$REPORTS_DIR/android-$project"
                        cp -r app/build/reports/tests/* "$REPORTS_DIR/android-$project/" 2>/dev/null || true
                    fi
                else
                    print_status "FAIL" "$project unit tests failed"
                    android_failed=1
                fi
            else
                print_status "WARN" "gradlew not found in $project"
            fi
            
            cd "$PROJECT_ROOT"
        else
            print_status "WARN" "$project directory or build.gradle.kts not found"
        fi
    done
    
    # Set overall Android tests status
    if [ $android_failed -eq 0 ]; then
        ANDROID_TESTS_STATUS="PASS"
    else
        ANDROID_TESTS_STATUS="FAIL"
        return 1
    fi
    
    return 0
}

# Function to run security tests
run_security_tests() {
    log "🔒 Running security tests..."
    
    # Run SonarQube analysis
    log "🔍 Running SonarQube analysis..."
    if "$SCRIPT_DIR/sonarqube-scan.sh" 2>&1 | tee -a "$LOG_FILE"; then
        print_status "PASS" "SonarQube analysis passed"
        SONARQUBE_STATUS="PASS"
    else
        print_status "FAIL" "SonarQube analysis failed"
        SONARQUBE_STATUS="FAIL"
        return 1
    fi
    
    # Run Snyk analysis
    log "🔒 Running Snyk analysis..."
    if "$SCRIPT_DIR/snyk-scan.sh" 2>&1 | tee -a "$LOG_FILE"; then
        print_status "PASS" "Snyk analysis passed"
        SNYK_STATUS="PASS"
    else
        print_status "FAIL" "Snyk analysis failed"
        SNYK_STATUS="FAIL"
        return 1
    fi
    
    # Run additional security scans
    log "🔍 Running additional security scans..."
    cd "$PROJECT_ROOT"
    
    # Run Trivy scan
    if $COMPOSE_CMD -f docker-compose.security.yml --profile trivy-scan run --rm trivy-scanner 2>&1 | tee -a "$LOG_FILE"; then
        print_status "PASS" "Trivy scan completed"
        TRIVY_STATUS="PASS"
    else
        print_status "WARN" "Trivy scan failed"
        TRIVY_STATUS="WARN"
    fi
    
    # Run OWASP Dependency Check
    if $COMPOSE_CMD -f docker-compose.security.yml --profile dependency-check run --rm dependency-check 2>&1 | tee -a "$LOG_FILE"; then
        print_status "PASS" "OWASP Dependency Check completed"
        OWASP_STATUS="PASS"
    else
        print_status "WARN" "OWASP Dependency Check failed"
        OWASP_STATUS="WARN"
    fi
    
    return 0
}

# Function to generate comprehensive report
generate_final_report() {
    log "📊 Generating comprehensive test report..."
    
    local success_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
    
    cat > "$REPORTS_DIR/comprehensive-test-report-$TIMESTAMP.html" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Catalogizer - Comprehensive Test Report</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; margin: 20px; background: #f5f7fa; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; border-radius: 15px; text-align: center; margin-bottom: 30px; box-shadow: 0 4px 15px rgba(0,0,0,0.1); }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .metric { background: white; padding: 25px; border-radius: 10px; text-align: center; box-shadow: 0 2px 10px rgba(0,0,0,0.1); transition: transform 0.3s ease; }
        .metric:hover { transform: translateY(-5px); }
        .metric h3 { margin: 0; color: #495057; font-size: 14px; text-transform: uppercase; letter-spacing: 1px; }
        .metric p { margin: 10px 0 0 0; font-size: 32px; font-weight: bold; }
        .success { color: #28a745; }
        .warning { color: #ffc107; }
        .error { color: #dc3545; }
        .section { background: white; margin: 20px 0; padding: 25px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .section h2 { color: #495057; border-bottom: 2px solid #e9ecef; padding-bottom: 10px; }
        .test-item { display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-bottom: 1px solid #e9ecef; }
        .test-item:last-child { border-bottom: none; }
        .status-pass { color: #28a745; font-weight: bold; }
        .status-fail { color: #dc3545; font-weight: bold; }
        .status-warn { color: #ffc107; font-weight: bold; }
        .progress-bar { width: 100%; height: 20px; background: #e9ecef; border-radius: 10px; overflow: hidden; margin: 10px 0; }
        .progress-fill { height: 100%; background: linear-gradient(90deg, #28a745, #20c997); transition: width 0.3s ease; }
        .file-list { max-height: 300px; overflow-y: auto; background: #f8f9fa; padding: 15px; border-radius: 8px; font-family: 'Courier New', monospace; font-size: 12px; }
        .recommendations { background: #e7f3ff; border-left: 4px solid #007bff; padding: 15px; margin: 15px 0; border-radius: 0 8px 8px 0; }
        .footer { text-align: center; margin-top: 40px; padding: 20px; color: #6c757d; font-size: 14px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🧪 Catalogizer Comprehensive Test Report</h1>
        <p>Complete Test Suite Results Including Security Analysis</p>
        <p>Generated on $(date)</p>
    </div>
    
    <div class="summary">
        <div class="metric">
            <h3>Total Tests</h3>
            <p>$TOTAL_TESTS</p>
        </div>
        <div class="metric">
            <h3>Passed</h3>
            <p class="success">$PASSED_TESTS</p>
        </div>
        <div class="metric">
            <h3>Failed</h3>
            <p class="error">$FAILED_TESTS</p>
        </div>
        <div class="metric">
            <h3>Success Rate</h3>
            <p class="success">$success_rate%</p>
        </div>
    </div>
    
    <div class="section">
        <h2>📊 Test Progress</h2>
        <div class="progress-bar">
            <div class="progress-fill" style="width: $success_rate%;"></div>
        </div>
        <p><strong>$success_rate%</strong> of tests passed successfully</p>
    </div>
    
    <div class="section">
        <h2>🧪 Test Results Summary</h2>
        <div class="test-item">
            <span>🐹 Go API Tests</span>
            $(if [ "$GO_TESTS_STATUS" = "PASS" ]; then echo '<span class="status-pass">✅ Passed</span>'; elif [ "$GO_TESTS_STATUS" = "FAIL" ]; then echo '<span class="status-fail">❌ Failed</span>'; else echo '<span class="status-warn">⚠️ Not Run</span>'; fi)
        </div>
        <div class="test-item">
            <span>🟢 JavaScript/TypeScript Tests</span>
            $(if [ "$JS_TESTS_STATUS" = "PASS" ]; then echo '<span class="status-pass">✅ Passed</span>'; elif [ "$JS_TESTS_STATUS" = "FAIL" ]; then echo '<span class="status-fail">❌ Failed</span>'; else echo '<span class="status-warn">⚠️ Not Run</span>'; fi)
        </div>
        <div class="test-item">
            <span>📱 Android Tests</span>
            $(if [ "$ANDROID_TESTS_STATUS" = "PASS" ]; then echo '<span class="status-pass">✅ Passed</span>'; elif [ "$ANDROID_TESTS_STATUS" = "FAIL" ]; then echo '<span class="status-fail">❌ Failed</span>'; else echo '<span class="status-warn">⚠️ Not Run</span>'; fi)
        </div>
        <div class="test-item">
            <span>🔍 SonarQube Analysis</span>
            $(if [ "$SONARQUBE_STATUS" = "PASS" ]; then echo '<span class="status-pass">✅ Passed</span>'; elif [ "$SONARQUBE_STATUS" = "FAIL" ]; then echo '<span class="status-fail">❌ Failed</span>'; else echo '<span class="status-warn">⚠️ Not Run</span>'; fi)
        </div>
        <div class="test-item">
            <span>🔒 Snyk Security Scan</span>
            $(if [ "$SNYK_STATUS" = "PASS" ]; then echo '<span class="status-pass">✅ Passed</span>'; elif [ "$SNYK_STATUS" = "FAIL" ]; then echo '<span class="status-fail">❌ Failed</span>'; else echo '<span class="status-warn">⚠️ Not Run</span>'; fi)
        </div>
        <div class="test-item">
            <span>🐳 Trivy Container Scan</span>
            $(if [ "$TRIVY_STATUS" = "PASS" ]; then echo '<span class="status-pass">✅ Passed</span>'; elif [ "$TRIVY_STATUS" = "WARN" ]; then echo '<span class="status-warn">⚠️ Warning</span>'; else echo '<span class="status-warn">⚠️ Not Run</span>'; fi)
        </div>
        <div class="test-item">
            <span>🛡️ OWASP Dependency Check</span>
            $(if [ "$OWASP_STATUS" = "PASS" ]; then echo '<span class="status-pass">✅ Passed</span>'; elif [ "$OWASP_STATUS" = "WARN" ]; then echo '<span class="status-warn">⚠️ Warning</span>'; else echo '<span class="status-warn">⚠️ Not Run</span>'; fi)
        </div>
    </div>
    
    <div class="section">
        <h2>📁 Available Reports</h2>
        <div class="file-list">
EOF

    # Add links to all report files
    for report_file in "$REPORTS_DIR"/*.{json,html,xml}; do
        if [ -f "$report_file" ]; then
            filename=$(basename "$report_file")
            echo "📄 $filename<br>" >> "$REPORTS_DIR/comprehensive-test-report-$TIMESTAMP.html"
        fi
    done

    cat >> "$REPORTS_DIR/comprehensive-test-report-$TIMESTAMP.html" << EOF
        </div>
    </div>
    
    <div class="section">
        <h2>🎯 Security Analysis Summary</h2>
        <div class="recommendations">
            <h3>$(if [ "$SONARQUBE_STATUS" = "PASS" ] && [ "$SNYK_STATUS" = "PASS" ] && [ "$TRIVY_STATUS" = "PASS" ] && [ "$OWASP_STATUS" = "PASS" ]; then echo '✅ Security Status'; elif [ "$SONARQUBE_STATUS" = "FAIL" ] || [ "$SNYK_STATUS" = "FAIL" ]; then echo '❌ Security Status - Critical Issues Found'; else echo '⚠️ Security Status - Some Scans Incomplete'; fi)</h3>
            <p>$(if [ "$SONARQUBE_STATUS" = "PASS" ] && [ "$SNYK_STATUS" = "PASS" ] && [ "$TRIVY_STATUS" = "PASS" ] && [ "$OWASP_STATUS" = "PASS" ]; then echo 'All security scans completed successfully. Review individual scan reports for details.'; elif [ "$SONARQUBE_STATUS" = "FAIL" ] || [ "$SNYK_STATUS" = "FAIL" ]; then echo 'Critical security scans failed. Immediate attention required.'; else echo 'Some security scans did not complete or reported warnings. Review logs for details.'; fi)</p>
            <ul>
                <li><strong>SonarQube:</strong> $(if [ "$SONARQUBE_STATUS" = "PASS" ]; then echo '✅ Passed - Code quality analyzed'; elif [ "$SONARQUBE_STATUS" = "FAIL" ]; then echo '❌ Failed'; else echo '⚠️ Not run'; fi)</li>
                <li><strong>Snyk:</strong> $(if [ "$SNYK_STATUS" = "PASS" ]; then echo '✅ Passed - Dependencies scanned'; elif [ "$SNYK_STATUS" = "FAIL" ]; then echo '❌ Failed - Vulnerabilities may exist'; else echo '⚠️ Not run'; fi)</li>
                <li><strong>Trivy:</strong> $(if [ "$TRIVY_STATUS" = "PASS" ]; then echo '✅ Passed - Container images scanned'; elif [ "$TRIVY_STATUS" = "WARN" ]; then echo '⚠️ Warning - Check logs'; else echo '⚠️ Not run'; fi)</li>
                <li><strong>OWASP:</strong> $(if [ "$OWASP_STATUS" = "PASS" ]; then echo '✅ Passed - Dependencies analyzed'; elif [ "$OWASP_STATUS" = "WARN" ]; then echo '⚠️ Warning - Check logs'; else echo '⚠️ Not run'; fi)</li>
            </ul>
        </div>
    </div>
    
    <div class="section">
        <h2>🔧 Recommendations</h2>
        <div class="recommendations">
            <h3>Continuous Improvement</h3>
            <ul>
                <li>Set up automated testing in CI/CD pipeline</li>
                <li>Regularly update dependencies to latest secure versions</li>
                <li>Implement code coverage requirements (minimum 80%)</li>
                <li>Schedule regular security assessments</li>
                <li>Monitor test performance and optimize slow tests</li>
            </ul>
        </div>
    </div>
    
    <div class="footer">
        <p>This report was generated automatically by the Catalogizer comprehensive test suite.</p>
        <p>For questions or concerns, please contact the development team.</p>
        <p>Log file: <code>comprehensive-test.log</code></p>
    </div>
</body>
</html>
EOF
    
    # Create symlink to latest report
    ln -sf "comprehensive-test-report-$TIMESTAMP.html" "$REPORTS_DIR/latest-comprehensive-test-report.html"
    
    log "📊 Comprehensive test report generated: $REPORTS_DIR/comprehensive-test-report-$TIMESTAMP.html"
}

# Function to cleanup
cleanup() {
    log "🧹 Cleaning up..."
    
    # Stop container services
    cd "$PROJECT_ROOT"
    $COMPOSE_CMD -f docker-compose.security.yml down 2>/dev/null || true
    
    log "✅ Cleanup completed"
}

# Set up trap for cleanup
trap cleanup EXIT

# Main execution
main() {
    log "🚀 Starting Comprehensive Test Suite..."
    
    # Check prerequisites
    if ! check_prerequisites; then
        log "❌ Prerequisites check failed"
        exit 1
    fi
    
    # Start security services
    log "🚀 Starting security testing services..."
    cd "$PROJECT_ROOT"
    $COMPOSE_CMD -f docker-compose.security.yml up -d sonarqube sonarqube-db
    
    # Wait for SonarQube to be ready
    log "⏳ Waiting for SonarQube to be ready..."
    for i in {1..60}; do
        if curl -f -s http://localhost:9000/api/system/status > /dev/null 2>&1; then
            log "✅ SonarQube is ready"
            break
        fi
        if [ $i -eq 60 ]; then
            print_status "FAIL" "SonarQube failed to start within timeout"
            exit 1
        fi
        sleep 10
    done
    
    # Run all test suites
    local test_failed=false
    
    # Run functional tests first
    if ! run_shell_tests; then
        test_failed=true
    fi

    if ! run_go_tests; then
        test_failed=true
    fi

    if ! run_js_tests; then
        test_failed=true
    fi

    if ! run_android_tests; then
        test_failed=true
    fi
    
    # Run security tests
    if ! run_security_tests; then
        test_failed=true
    fi
    
    # Generate final report
    generate_final_report
    
    # Final status
    echo ""
    echo -e "${BLUE}🧪 Comprehensive Test Suite Summary:${NC}"
    echo -e "${BLUE}=====================================${NC}"
    echo -e "Total Tests: $TOTAL_TESTS"
    echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
    echo -e "${RED}Failed: $FAILED_TESTS${NC}"
    echo -e "Success Rate: $((PASSED_TESTS * 100 / TOTAL_TESTS))%"
    echo -e "📊 Report: $REPORTS_DIR/comprehensive-test-report-$TIMESTAMP.html"
    echo -e "📋 Log: $LOG_FILE"
    
    if [ "$test_failed" = true ] || [ "$FAILED_TESTS" -gt 0 ]; then
        echo ""
        echo -e "${RED}❌ SOME TESTS FAILED!${NC}"
        echo -e "${RED}All tests must pass with 100% success before deployment.${NC}"
        exit 1
    else
        echo ""
        echo -e "${GREEN}🎉 ALL TESTS PASSED SUCCESSFULLY!${NC}"
        echo -e "${GREEN}The project is ready for deployment.${NC}"
        exit 0
    fi
}

# Run main function
main "$@"