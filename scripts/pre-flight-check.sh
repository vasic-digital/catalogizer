#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh

# Pre-Flight Check Script
# Runs all quality checks before committing code

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

check() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $2"
        PASS=$((PASS + 1))
    else
        echo -e "${RED}✗${NC} $2"
        FAIL=$((FAIL + 1))
    fi
}

echo "=========================================="
echo "Pre-Flight Quality Checks"
echo "=========================================="
echo ""

# Backend checks
echo "Backend (Go)..."
cd catalog-api

go fmt ./... > /dev/null 2>&1
check $? "Go formatting"

go vet ./... > /dev/null 2>&1
check $? "Go vet"

go build -o /tmp/catalogizer-test . > /dev/null 2>&1
check $? "Go build"

# Run short tests only
go test -short ./internal/logging/... > /dev/null 2>&1
check $? "Go tests (short)"

cd ..

# Frontend checks
echo ""
echo "Frontend (TypeScript/React)..."
cd catalog-web

npm run type-check > /dev/null 2>&1
check $? "TypeScript type check"

npm run lint > /dev/null 2>&1
check $? "ESLint"

npm run build > /dev/null 2>&1
check $? "Production build"

cd ..

# Docs HTML (constitution 11.4.190, 11.4.12): every tracked docs/*.html declares a viewport, and the generated twins are current
echo ""
echo "Docs HTML..."
rc=0; python3 scripts/docs/check_html_viewport.py > /dev/null 2>&1 || rc=$?
check $rc "docs/*.html declare <meta name=viewport> (scripts/docs/check_html_viewport.py)"
rc=0; python3 scripts/docs/check_md_links.py > /dev/null 2>&1 || rc=$?
check $rc "tracked docs markdown relative links and #anchors resolve (scripts/docs/check_md_links.py)"
rc=0; bash scripts/docs/render_md_to_html.sh --check \
    docs/decisions/llms_verifier_reconciliation.md \
    docs/design/identity_share_discovery/RECON_RESULTS.md \
    docs/qa/findings_20260626/discovered_findings.md \
    docs/qa/findings_20260626/ui_critical_resolution.md > /dev/null 2>&1 || rc=$?
check $rc "generated docs HTML equals render_md_to_html.sh output (--check)"

# Security checks
echo ""
echo "Security..."

# Check for secrets
grep -r "password.*=.*\"" --include="*.go" catalog-api/ | grep -v "_test.go" | grep -v "example" > /tmp/secrets-check.txt 2>&1 || true
if [ ! -s /tmp/secrets-check.txt ]; then
    check 0 "No hardcoded passwords detected"
else
    check 1 "Potential hardcoded secrets found"
fi

# Summary
echo ""
echo "=========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "=========================================="

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}All checks passed! Ready to commit.${NC}"
    exit 0
else
    echo -e "${RED}Some checks failed. Please fix before committing.${NC}"
    exit 1
fi
