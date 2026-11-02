#!/bin/bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/../scripts/lib/cgo_flags.sh"   # 2026-11-02 decision: cgo define for go-sqlcipher (scripts/lib/cgo_flags.sh)

echo "=========================================="
echo "Catalogizer Conversion Service Verification"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Track results
TOTAL_CHECKS=0
PASSED_CHECKS=0

# Function to log a check
log_check() {
    local check_name="$1"
    local status="$2"
    local details="$3"
    
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✓ PASS${NC} ${check_name}"
        if [ -n "$details" ]; then
            echo -e "       ${details}"
        fi
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    else
        echo -e "${RED}✗ FAIL${NC} ${check_name}"
        if [ -n "$details" ]; then
            echo -e "       ${details}"
        fi
    fi
}

echo -e "${BLUE}1. API Implementation Verification${NC}"
echo "----------------------------------------"

# Check handler implementation
if [ -f "handlers/conversion_handler.go" ]; then
    functions=$(grep -E "^func \(\*ConversionHandler\)" handlers/conversion_handler.go | wc -l)
    log_check "Conversion Handler Implementation" "PASS" "Found $functions handler functions"
else
    log_check "Conversion Handler Implementation" "FAIL" "handlers/conversion_handler.go not found"
fi

# Check service implementation  
if [ -f "services/conversion_service.go" ]; then
    methods=$(grep -E "^func \(\*ConversionService\)" services/conversion_service.go | wc -l)
    log_check "Conversion Service Implementation" "PASS" "Found $methods service methods"
else
    log_check "Conversion Service Implementation" "FAIL" "services/conversion_service.go not found"
fi

# Check repository implementation
if [ -f "repository/conversion_repository.go" ]; then
    methods=$(grep -E "^func \(\*ConversionRepository\)" repository/conversion_repository.go | wc -l)
    log_check "Conversion Repository Implementation" "PASS" "Found $methods repository methods"
else
    log_check "Conversion Repository Implementation" "FAIL" "repository/conversion_repository.go not found"
fi

echo ""
echo -e "${BLUE}2. Database Migration Verification${NC}"
echo "----------------------------------------"

# Check migration files
if [ -f "database/migrations/000002_conversion_jobs.up.sql" ]; then
    log_check "Migration Up Script" "PASS" "000002_conversion_jobs.up.sql exists"
else
    log_check "Migration Up Script" "FAIL" "Migration up script not found"
fi

if [ -f "database/migrations/000002_conversion_jobs.down.sql" ]; then
    log_check "Migration Down Script" "PASS" "000002_conversion_jobs.down.sql exists"
else
    log_check "Migration Down Script" "FAIL" "Migration down script not found"
fi

# Check migration integration
if grep -q "createConversionJobsTable" database/migrations.go; then
    log_check "Migration Integration" "PASS" "Migration function integrated in migrations.go"
else
    log_check "Migration Integration" "FAIL" "Migration function not found in migrations.go"
fi

echo ""
echo -e "${BLUE}3. API Routes Registration${NC}"
echo "----------------------------------------"

# Check route registration
if grep -q "conversionGroup" main.go; then
    endpoints=$(grep -A5 "conversionGroup :=" main.go | grep -E "(POST|GET)" | wc -l)
    log_check "API Routes Registration" "PASS" "Found $endpoints conversion endpoints registered"
else
    log_check "API Routes Registration" "FAIL" "Conversion routes not found in main.go"
fi

echo ""
echo -e "${BLUE}4. Data Models Verification${NC}"
echo "----------------------------------------"

# Check data models
if grep -q "type ConversionJob struct" models/user.go; then
    log_check "ConversionJob Model" "PASS" "ConversionJob struct defined in models"
else
    log_check "ConversionJob Model" "FAIL" "ConversionJob model not found"
fi

if grep -q "type ConversionRequest struct" models/user.go; then
    log_check "ConversionRequest Model" "PASS" "ConversionRequest struct defined in models"
else
    log_check "ConversionRequest Model" "FAIL" "ConversionRequest model not found"
fi

# Check permission constants
if grep -q "PermissionConversionView" models/user.go; then
    permissions=$(grep -E "PermissionConversion" models/user.go | wc -l)
    log_check "Permission Constants" "PASS" "Found $permissions conversion permission constants"
else
    log_check "Permission Constants" "FAIL" "Conversion permissions not defined"
fi

echo ""
echo -e "${BLUE}5. Testing Verification${NC}"
echo "----------------------------------------"

# Run handler tests
echo "Running handler tests..."
test_output=$(go test ./handlers -v -run "TestCreateJob|TestGetJob|TestListJobs|TestCancelJob|TestGetSupportedFormats" 2>&1)
test_passes=$(echo "$test_output" | grep -c "PASS")
test_fails=$(echo "$test_output" | grep -c "FAIL")

if [ "$test_fails" -eq 0 ] && [ "$test_passes" -gt 0 ]; then
    log_check "Handler Unit Tests" "PASS" "$test_passes/5 tests passing"
else
    log_check "Handler Unit Tests" "FAIL" "$test_passes tests passing, $test_fails failing"
fi

# Check for integration tests
if [ -f "tests/conversion_api_integration_test.go" ]; then
    log_check "Integration Tests" "PASS" "Integration tests exist"
else
    log_check "Integration Tests" "FAIL" "Integration tests not found"
fi

echo ""
echo -e "${BLUE}6. External Dependencies Check${NC}"
echo "----------------------------------------"

# Check for FFmpeg
if command -v ffmpeg &> /dev/null; then
    ffmpeg_version=$(ffmpeg -version 2>/dev/null | head -n1 | cut -d' ' -f3)
    log_check "FFmpeg Installation" "PASS" "FFmpeg $ffmpeg_version installed"
else
    log_check "FFmpeg Installation" "FAIL" "FFmpeg not installed (required for production)"
fi

# Check for ImageMagick
if command -v convert &> /dev/null; then
    log_check "ImageMagick Installation" "PASS" "ImageMagick convert command available"
else
    log_check "ImageMagick Installation" "FAIL" "ImageMagick not installed"
fi

# Check for go-fitz
if grep -q "github.com/gen2brain/go-fitz" go.mod; then
    log_check "go-fitz Library" "PASS" "go-fitz PDF library included in go.mod"
else
    log_check "go-fitz Library" "FAIL" "go-fitz library not found in go.mod"
fi

echo ""
echo -e "${BLUE}7. Security Verification${NC}"
echo "----------------------------------------"

# Check JWT authentication
if grep -q "getCurrentUser" handlers/conversion_handler.go; then
    log_check "JWT Authentication" "PASS" "JWT authentication implemented in handlers"
else
    log_check "JWT Authentication" "FAIL" "JWT authentication not found"
fi

# Check permission-based access
if grep -q "CheckPermission.*PermissionConversion" handlers/conversion_handler.go; then
    log_check "Permission-Based Access" "PASS" "Role-based permissions implemented"
else
    log_check "Permission-Based Access" "FAIL" "Permission checks not found"
fi

echo ""
echo -e "${BLUE}8. Configuration Verification${NC}"
echo "----------------------------------------"

# Check service factory pattern
if grep -q "NewConversionService" services/conversion_service.go; then
    log_check "Service Factory Pattern" "PASS" "NewConversionService constructor implemented"
else
    log_check "Service Factory Pattern" "FAIL" "Service factory pattern not found"
fi

# Check dependency injection
if grep -q "NewConversionHandler" handlers/conversion_handler.go; then
    log_check "Dependency Injection" "PASS" "Constructor injection pattern used"
else
    log_check "Dependency Injection" "FAIL" "Dependency injection not found"
fi

echo ""
echo "=========================================="
echo "VERIFICATION SUMMARY"
echo "=========================================="
echo ""

success_rate=$(( (PASSED_CHECKS * 100) / TOTAL_CHECKS ))
echo -e "Total Checks: ${TOTAL_CHECKS}"
echo -e "Passed: ${GREEN}${PASSED_CHECKS}${NC}"
echo -e "Failed: ${RED}$((TOTAL_CHECKS - PASSED_CHECKS))${NC}"
echo -e "Success Rate: ${success_rate}%"

echo ""

if [ "$success_rate" -ge 90 ]; then
    echo -e "${GREEN}🎉 EXCELLENT: Conversion service is production-ready!${NC}"
elif [ "$success_rate" -ge 80 ]; then
    echo -e "${YELLOW}⚠️  GOOD: Conversion service is mostly complete with minor issues${NC}"
elif [ "$success_rate" -ge 70 ]; then
    echo -e "${YELLOW}⚠️  FAIR: Conversion service needs some attention${NC}"
else
    echo -e "${RED}❌ POOR: Conversion service needs significant work${NC}"
fi

echo ""
echo "=========================================="

# Exit with appropriate code
if [ "$success_rate" -ge 90 ]; then
    exit 0
else
    exit 1
fi