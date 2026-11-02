#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
#
# Local CI/CD Pipeline Runner
# Runs all checks locally before committing
#

set -e

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="reports/ci/${TIMESTAMP}"
mkdir -p "${REPORT_DIR}"

echo "==================================================================="
echo "CATALOGIZER LOCAL CI/CD PIPELINE"
echo "Timestamp: ${TIMESTAMP}"
echo "==================================================================="
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TOTAL_STEPS=0
PASSED_STEPS=0
FAILED_STEPS=0
SKIPPED_STEPS=0

run_step() {
    local name=$1
    local command=$2
    local optional=$3
    
    TOTAL_STEPS=$((TOTAL_STEPS + 1))
    
    echo ""
    echo "[${TOTAL_STEPS}] Running: ${name}"
    echo "----------------------------------------"
    
    if eval "${command}" > "${REPORT_DIR}/${name}.log" 2>&1; then
        echo -e "${GREEN}✓ PASSED${NC}: ${name}"
        PASSED_STEPS=$((PASSED_STEPS + 1))
        return 0
    else
        if [ "${optional}" == "optional" ]; then
            echo -e "${YELLOW}⚠ SKIPPED${NC}: ${name} (optional)"
            SKIPPED_STEPS=$((SKIPPED_STEPS + 1))
        else
            echo -e "${RED}✗ FAILED${NC}: ${name}"
            echo "See: ${REPORT_DIR}/${name}.log"
            FAILED_STEPS=$((FAILED_STEPS + 1))
        fi
        return 1
    fi
}

# ==========================================
# PHASE 1: Go Backend Validation
# ==========================================
echo ""
echo "PHASE 1: Go Backend Validation"
echo "==================================================================="

run_step "go-fmt" "cd catalog-api && test -z '\$(gofmt -l .)'"
run_step "go-vet" "cd catalog-api && go vet ./..." "optional"
run_step "go-test" "cd catalog-api && GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -v" "optional"
run_step "go-coverage" "cd catalog-api && go test -coverprofile=${REPORT_DIR}/go-coverage.out ./... && go tool cover -func=${REPORT_DIR}/go-coverage.out | grep total" "optional"

# ==========================================
# PHASE 2: TypeScript Frontend Validation
# ==========================================
echo ""
echo "PHASE 2: TypeScript Frontend Validation"
echo "==================================================================="

run_step "web-lint" "cd catalog-web && npm run lint" "optional"
run_step "web-typecheck" "cd catalog-web && npm run type-check" "optional"
run_step "web-test" "cd catalog-web && npm run test" "optional"

# ==========================================
# PHASE 3: Build Validation
# ==========================================
echo ""
echo "PHASE 3: Build Validation"
echo "==================================================================="

run_step "build-api" "cd catalog-api && go build -o ${REPORT_DIR}/catalog-api" "optional"
run_step "build-web" "cd catalog-web && npm run build" "optional"

# ==========================================
# Summary Report
# ==========================================
echo ""
echo "==================================================================="
echo "CI/CD PIPELINE SUMMARY"
echo "==================================================================="
echo "Total Steps:    ${TOTAL_STEPS}"
echo -e "${GREEN}Passed:         ${PASSED_STEPS}${NC}"
echo -e "${RED}Failed:         ${FAILED_STEPS}${NC}"
echo -e "${YELLOW}Skipped:        ${SKIPPED_STEPS}${NC}"
echo ""

# Generate summary JSON
cat > "${REPORT_DIR}/summary.json" << EOF
{
  "timestamp": "${TIMESTAMP}",
  "total_steps": ${TOTAL_STEPS},
  "passed": ${PASSED_STEPS},
  "failed": ${FAILED_STEPS},
  "skipped": ${SKIPPED_STEPS},
  "success_rate": $(echo "scale=2; ${PASSED_STEPS} * 100 / ${TOTAL_STEPS}" | bc -l 2>/dev/null || echo "0")
}
EOF

echo "Report Directory: ${REPORT_DIR}"
echo "==================================================================="

if [ ${FAILED_STEPS} -gt 0 ]; then
    echo -e "${RED}PIPELINE FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}PIPELINE PASSED${NC}"
    exit 0
fi
