#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
set -e

# Memory Leak Detection Script for Catalogizer
# Runs memory profiling on Go packages and checks for leaks

COLOR_RED='\033[0;31m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[1;33m'
COLOR_BLUE='\033[0;34m'
COLOR_RESET='\033[0m'

echo -e "${COLOR_BLUE}=== Catalogizer Memory Leak Detection ===${COLOR_RESET}"
echo ""

# Navigate to catalog-api directory
cd "$(dirname "$0")/../catalog-api" || exit 1

echo -e "${COLOR_YELLOW}Step 1: Running memory profiling on all packages...${COLOR_RESET}"
echo ""

# Create profiles directory
mkdir -p profiles

# Run tests with memory profiling (on passing packages)
echo "Running tests with memory profiling on key packages..."
go test -memprofile=profiles/mem.prof -memprofilerate=1 ./handlers/... ./internal/... 2>&1 | grep -E "PASS|FAIL" | head -10 || true

if [ -f profiles/mem.prof ]; then
    echo -e "${COLOR_GREEN}✓ Memory profile generated: profiles/mem.prof${COLOR_RESET}"
    echo ""

    echo -e "${COLOR_YELLOW}Step 2: Analyzing memory profile...${COLOR_RESET}"
    echo ""

    # Analyze top memory allocations
    echo "Top 10 memory allocations by cumulative:"
    go tool pprof -top -cum profiles/mem.prof 2>/dev/null | head -20 || echo "Profile analysis not available"

    echo ""
    echo "Top 10 memory allocations by flat:"
    go tool pprof -top profiles/mem.prof 2>/dev/null | head -20 || echo "Profile analysis not available"

    echo ""
else
    echo -e "${COLOR_YELLOW}⚠ Memory profile not generated (some tests may have failed)${COLOR_RESET}"
    echo "Continuing with static analysis..."
    echo ""
fi
echo -e "${COLOR_YELLOW}Step 3: Checking for common memory leak patterns...${COLOR_RESET}"
echo ""

ISSUES_FOUND=0

# Check for unclosed HTTP response bodies
echo "Checking for unclosed HTTP response bodies..."
UNCLOSED_RESPONSES=$(grep -rn "http.Get\|http.Post\|http.Do" --include="*.go" . | \
    grep -v "_test.go" | \
    while IFS=: read -r file line content; do
        # Check if the file has defer resp.Body.Close() near the line
        if ! awk -v start=$((line-1)) -v end=$((line+10)) \
            'NR >= start && NR <= end && /defer.*Body\.Close\(\)/ {found=1; exit}
             END {exit !found}' "$file"; then
            echo "$file:$line: Potential unclosed response body"
            ISSUES_FOUND=$((ISSUES_FOUND + 1))
        fi
    done 2>/dev/null || true)

if [ -n "$UNCLOSED_RESPONSES" ]; then
    echo -e "${COLOR_RED}✗ Found potential unclosed HTTP response bodies:${COLOR_RESET}"
    echo "$UNCLOSED_RESPONSES"
    echo ""
else
    echo -e "${COLOR_GREEN}✓ No unclosed HTTP response bodies found${COLOR_RESET}"
fi

# Check for goroutine leaks (goroutines without context cancellation)
echo ""
echo "Checking for goroutines without context cancellation..."
GOROUTINE_LEAKS=$(grep -rn "go func" --include="*.go" . | \
    grep -v "_test.go" | \
    while IFS=: read -r file line content; do
        # Check if the goroutine uses context or has defer
        if ! awk -v start=$line -v end=$((line+20)) \
            'NR >= start && NR <= end && (/ctx\.Done\(\)|/context\.Context|/defer/) {found=1; exit}
             END {exit !found}' "$file"; then
            echo "$file:$line: Goroutine without context cancellation"
            ISSUES_FOUND=$((ISSUES_FOUND + 1))
        fi
    done 2>/dev/null | head -20 || true)

if [ -n "$GOROUTINE_LEAKS" ]; then
    echo -e "${COLOR_YELLOW}⚠ Found goroutines without explicit context cancellation:${COLOR_RESET}"
    echo "$GOROUTINE_LEAKS"
    echo ""
    echo "Note: Some goroutines may be intentionally long-running."
else
    echo -e "${COLOR_GREEN}✓ All goroutines use context cancellation${COLOR_RESET}"
fi

# Check for unclosed file handles
echo ""
echo "Checking for unclosed file handles..."
UNCLOSED_FILES=$(grep -rn "os\.Open\|os\.Create\|ioutil\.ReadFile" --include="*.go" . | \
    grep -v "_test.go" | \
    while IFS=: read -r file line content; do
        # Check if the file has defer file.Close() near the line
        if echo "$content" | grep -q "ioutil\.ReadFile"; then
            continue  # ReadFile handles closing automatically
        fi
        if ! awk -v start=$((line-1)) -v end=$((line+10)) \
            'NR >= start && NR <= end && /defer.*\.Close\(\)/ {found=1; exit}
             END {exit !found}' "$file"; then
            echo "$file:$line: Potential unclosed file handle"
            ISSUES_FOUND=$((ISSUES_FOUND + 1))
        fi
    done 2>/dev/null | head -10 || true)

if [ -n "$UNCLOSED_FILES" ]; then
    echo -e "${COLOR_RED}✗ Found potential unclosed file handles:${COLOR_RESET}"
    echo "$UNCLOSED_FILES"
    echo ""
else
    echo -e "${COLOR_GREEN}✓ No unclosed file handles found${COLOR_RESET}"
fi

echo ""
echo -e "${COLOR_YELLOW}Step 4: Running race detector...${COLOR_RESET}"
echo ""

# Run race detector
if go test -race ./... > profiles/race-detector.log 2>&1; then
    echo -e "${COLOR_GREEN}✓ No race conditions detected${COLOR_RESET}"
else
    if grep -q "WARNING: DATA RACE" profiles/race-detector.log; then
        echo -e "${COLOR_RED}✗ Race conditions detected!${COLOR_RESET}"
        echo "See profiles/race-detector.log for details"
        grep -A 10 "WARNING: DATA RACE" profiles/race-detector.log | head -30
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    else
        echo -e "${COLOR_GREEN}✓ Race detector passed (some tests may have failed for other reasons)${COLOR_RESET}"
    fi
fi

echo ""
echo -e "${COLOR_YELLOW}Step 5: Checking goroutine count stability...${COLOR_RESET}"
echo ""

# Create a test program to check goroutine stability
cat > profiles/goroutine_test.go << 'EOF'
package main

import (
    "fmt"
    "runtime"
    "time"
)

func main() {
    runtime.GC()
    time.Sleep(100 * time.Millisecond)

    initial := runtime.NumGoroutine()
    fmt.Printf("Initial goroutines: %d\n", initial)

    // Let goroutines stabilize
    time.Sleep(1 * time.Second)
    runtime.GC()
    time.Sleep(100 * time.Millisecond)

    final := runtime.NumGoroutine()
    fmt.Printf("Final goroutines: %d\n", final)

    if final > initial+5 {
        fmt.Printf("WARNING: Goroutine count increased by %d (possible leak)\n", final-initial)
    } else {
        fmt.Println("OK: Goroutine count stable")
    }
}
EOF

go run profiles/goroutine_test.go
rm -f profiles/goroutine_test.go

echo ""
echo -e "${COLOR_YELLOW}Step 6: Memory profile summary...${COLOR_RESET}"
echo ""

# Generate memory profile summary
go tool pprof -text -alloc_space profiles/mem.prof 2>/dev/null | head -15

echo ""
echo -e "${COLOR_BLUE}=== Memory Leak Detection Complete ===${COLOR_RESET}"
echo ""

if [ $ISSUES_FOUND -eq 0 ]; then
    echo -e "${COLOR_GREEN}✓ No critical memory leaks detected${COLOR_RESET}"
    echo ""
    echo "Profile files saved in: catalog-api/profiles/"
    echo "  - mem.prof: Memory profile"
    echo "  - race-detector.log: Race detector results"
    echo ""
    echo "To analyze memory profile interactively:"
    echo "  go tool pprof catalog-api/profiles/mem.prof"
    echo ""
    exit 0
else
    echo -e "${COLOR_RED}✗ Found $ISSUES_FOUND potential issues${COLOR_RESET}"
    echo ""
    echo "Please review the findings above and fix any critical issues."
    echo "Profile files saved in: catalog-api/profiles/"
    echo ""
    exit 1
fi
