#!/bin/bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/../../scripts/lib/cgo_flags.sh"   # 2026-11-02 decision: cgo define for go-sqlcipher (scripts/lib/cgo_flags.sh)

# Catalogizer Quick QA - Fast Development Validation
# Provides rapid feedback for developers during active development

set -e

echo "⚡ Catalogizer Quick QA"
echo "======================"
echo "🔍 Fast development validation for immediate feedback"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
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

# Quick validation function
quick_validate() {
    local issues_found=0

    log_info "Running quick validation checks..."
    echo ""

    # 1. Check for obvious syntax issues
    log_info "1. Checking for syntax issues..."

    # Go syntax check
    if [[ -d "catalog-api" ]]; then
        cd catalog-api
        if go fmt ./... > /dev/null 2>&1; then
            log_success "Go syntax: OK"
        else
            log_error "Go syntax: Issues found"
            ((issues_found++))
        fi
        cd ..
    fi

    # 2. Check for merge conflicts
    log_info "2. Checking for merge conflicts..."
    if git status --porcelain | grep -E "^(AA|DD|UU)" > /dev/null 2>&1; then
        log_error "Merge conflicts detected"
        ((issues_found++))
    else
        log_success "No merge conflicts"
    fi

    # 3. Check for debug statements
    log_info "3. Checking for debug statements..."
    debug_files=$(find . -name "*.go" -o -name "*.kt" -o -name "*.java" | \
                  xargs grep -l "fmt\.Print\|console\.log\|debugger\|TODO\|FIXME" 2>/dev/null | wc -l)
    if [[ $debug_files -gt 0 ]]; then
        log_warning "Found $debug_files files with debug statements"
    else
        log_success "No debug statements found"
    fi

    # 4. Check working directory status
    log_info "4. Checking working directory..."
    if git status --porcelain | grep -q .; then
        modified_count=$(git status --porcelain | wc -l)
        log_info "Modified files: $modified_count"
    else
        log_success "Working directory clean"
    fi

    # 5. Quick build test (if possible)
    log_info "5. Quick build validation..."

    # Go build test
    if [[ -d "catalog-api" ]]; then
        cd catalog-api
        if timeout 30 go build -v ./... > /dev/null 2>&1; then
            log_success "Go build: OK"
        else
            log_warning "Go build: Issues detected"
        fi
        cd ..
    fi

    # Android build test (quick)
    if [[ -d "catalogizer-android" && -f "catalogizer-android/gradlew" ]]; then
        cd catalogizer-android
        if timeout 60 ./gradlew compileDebugSources --no-daemon --quiet > /dev/null 2>&1; then
            log_success "Android compile: OK"
        else
            log_warning "Android compile: Issues detected"
        fi
        cd ..
    fi

    echo ""
    echo "=============================="

    if [[ $issues_found -eq 0 ]]; then
        log_success "✨ Quick QA: ALL CHECKS PASSED"
        echo ""
        echo "🚀 Your code looks good for development!"
        echo "💡 For comprehensive validation, run:"
        echo "   ./qa-ai-system/scripts/run-qa-tests.sh"
        echo ""
        return 0
    else
        log_error "⚠️  Quick QA: $issues_found ISSUES FOUND"
        echo ""
        echo "🔍 Please address the issues above before proceeding"
        echo "💡 For detailed analysis, run:"
        echo "   ./qa-ai-system/scripts/run-qa-tests.sh standard"
        echo ""
        return 1
    fi
}

# Show usage if help requested
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    cat << EOF
Catalogizer Quick QA - Fast Development Validation

Usage: $0 [--help]

This script performs rapid quality checks for immediate feedback:
  • Syntax validation (Go, Android)
  • Merge conflict detection
  • Debug statement scanning
  • Quick build validation
  • Working directory status

Typical execution time: 10-30 seconds

For comprehensive testing, use:
  ./qa-ai-system/scripts/run-qa-tests.sh

Examples:
  $0                    # Run quick validation
  $0 --help            # Show this help

EOF
    exit 0
fi

# Check if we're in the right directory
if [[ ! -d "qa-ai-system" ]]; then
    log_error "Please run from Catalogizer project root directory"
    exit 1
fi

# Show git status if available
if git rev-parse --git-dir > /dev/null 2>&1; then
    current_branch=$(git branch --show-current 2>/dev/null || echo "unknown")
    log_info "Current branch: $current_branch"

    # Show recent commits
    log_info "Recent commits:"
    git log --oneline -3 | sed 's/^/  📝 /'
    echo ""
fi

# Run the quick validation
start_time=$(date +%s)
quick_validate
end_time=$(date +%s)
duration=$((end_time - start_time))

echo "⏱️  Validation completed in ${duration} seconds"

exit $?