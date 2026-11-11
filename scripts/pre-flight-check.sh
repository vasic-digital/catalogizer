#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh

# TAURI-R2-FU-4: always run from the repository root, whatever the caller's cwd (relative paths below depend on it)
cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/.." || { echo "FATAL: cannot cd to repo root" >&2; exit 1; }
[ -z "${PRE_FLIGHT_CHDIR_ONLY:-}" ] || { pwd; exit 3; }   # test hook only: distinct non-zero exit, never reads as a passing run

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

# Installer wizard (CAT-TAURI-INSTALLER-1, TAURI-R1-FU-3): frontend dialog commands must be granted by a Tauri capability
echo ""
echo "Installer wizard (Tauri)..."
rc=0; bash scripts/tauri/test_wizard_capabilities.sh > /dev/null 2>&1 || rc=$?
check $rc "wizard dialog open/save granted by one capability for window main (scripts/tauri/test_wizard_capabilities.sh)"
rc=0; bash scripts/tauri/test_wizard_capabilities_mutation.sh > /dev/null 2>&1 || rc=$?
check $rc "wizard capability guard kills its paired mutations (scripts/tauri/test_wizard_capabilities_mutation.sh)"
# The mutant run is slow: only when the tauri guard / wizard capability files changed (vs merge-base with main, plus worktree), else an explicit SKIP line.
tauri_changed=0
if [ "${PRE_FLIGHT_FORCE_TAURI_MUTANTS:-}" = 1 ]; then tauri_changed=1
else
  mb=$(git merge-base HEAD main 2>/dev/null || git rev-parse HEAD 2>/dev/null || true)
  if [ -n "$mb" ] && [ -n "$({ git diff --name-only "$mb" -- scripts/tauri installer-wizard/src-tauri/capabilities installer-wizard/src-tauri/tauri.conf.json installer-wizard/src-tauri/permissions installer-wizard/src installer-wizard/index.html installer-wizard/public 2>/dev/null; git ls-files --others --exclude-standard -- scripts/tauri installer-wizard/src-tauri installer-wizard/src installer-wizard/index.html installer-wizard/public 2>/dev/null; } | head -1)" ]; then tauri_changed=1; fi
  [ -n "$mb" ] || tauri_changed=1   # cannot tell: run it
fi
if [ "$tauri_changed" = 1 ]; then
  rc=0; bash scripts/tauri/test_wizard_capabilities_guard_mutants.sh > /dev/null 2>&1 || rc=$?
  check $rc "wizard capability suite kills mutants of the guard itself (scripts/tauri/test_wizard_capabilities_guard_mutants.sh)"
else
  echo "SKIPPED (no tauri guard changes): wizard capability guard mutants (set PRE_FLIGHT_FORCE_TAURI_MUTANTS=1 to force)"
fi

rc=0; bash scripts/tauri/test_preflight_cwd.sh > /dev/null 2>&1 || rc=$?
check $rc "pre-flight-check.sh chdirs to repo root from any cwd (scripts/tauri/test_preflight_cwd.sh)"

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
