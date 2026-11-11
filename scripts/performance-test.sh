#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
set -e

COLOR_RED='\033[0;31m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[1;33m'
COLOR_BLUE='\033[0;34m'
COLOR_RESET='\033[0m'

echo -e "${COLOR_BLUE}=== Catalogizer Performance Testing Suite ===${COLOR_RESET}"
echo ""

# Navigate to project root
cd "$(dirname "$0")/.." || exit 1

# ---------------------------------------------------------------------------
# Backend Performance Tests
# ---------------------------------------------------------------------------

echo -e "${COLOR_YELLOW}Step 1: Running Go benchmark tests...${COLOR_RESET}"
echo ""

cd catalog-api || exit 1

# Run all benchmark tests
echo "Running protocol client benchmarks..."
go test -bench=. -benchmem -benchtime=3s ./tests/performance/protocol_bench_test.go 2>&1 | tee ../performance-results-protocol.txt || true

echo ""
echo "Running database benchmarks..."
go test -bench=. -benchmem -benchtime=3s ./tests/performance/database_bench_test.go 2>&1 | tee ../performance-results-database.txt || true

echo ""
echo "Running API endpoint benchmarks..."
go test -bench=. -benchmem -benchtime=3s ./tests/performance/baseline_test.go 2>&1 | tee ../performance-results-api.txt || true

echo ""
echo "Running service-specific benchmarks..."
go test -bench=. -benchmem -benchtime=2s ./services/auth_service_bench_test.go 2>&1 | tee ../performance-results-auth.txt || true
go test -bench=. -benchmem -benchtime=2s ./internal/media/detector/engine_bench_test.go 2>&1 | tee ../performance-results-detector.txt || true
go test -bench=. -benchmem -benchtime=2s ./internal/media/providers/providers_bench_test.go 2>&1 | tee ../performance-results-providers.txt || true

echo -e "${COLOR_GREEN}✓ Go benchmarks complete${COLOR_RESET}"
echo ""

cd ..

# ---------------------------------------------------------------------------
# Frontend Performance Tests
# ---------------------------------------------------------------------------

echo -e "${COLOR_YELLOW}Step 2: Analyzing frontend bundle size...${COLOR_RESET}"
echo ""

cd catalog-web || exit 1

# Build production bundle
echo "Building production bundle..."
npm run build > /dev/null 2>&1

# Analyze bundle size
echo "Analyzing bundle size..."
if command -v du &> /dev/null; then
    echo ""
    echo "Bundle size breakdown:"
    du -h dist/assets/*.js | sort -h
    echo ""
    echo "Total bundle size:"
    du -sh dist
    echo ""
fi

# Check if source-map-explorer is installed
if npm list source-map-explorer > /dev/null 2>&1; then
    echo "Generating bundle analysis with source-map-explorer..."
    npx source-map-explorer 'dist/assets/*.js' --html ../bundle-analysis.html 2>&1 | tail -10
    echo -e "${COLOR_GREEN}✓ Bundle analysis saved to bundle-analysis.html${COLOR_RESET}"
else
    echo -e "${COLOR_YELLOW}⚠ source-map-explorer not installed, skipping detailed analysis${COLOR_RESET}"
    echo "  To install: npm install --save-dev source-map-explorer"
fi

echo ""

# ---------------------------------------------------------------------------
# Lighthouse CI Performance Tests
# ---------------------------------------------------------------------------

echo -e "${COLOR_YELLOW}Step 3: Running Lighthouse CI performance tests...${COLOR_RESET}"
echo ""

# Check if lhci is installed
if command -v lhci &> /dev/null || npm list @lhci/cli > /dev/null 2>&1; then
    echo "Running Lighthouse CI..."

    # Build if not already built
    if [ ! -d "dist" ]; then
        npm run build > /dev/null 2>&1
    fi

    # Run Lighthouse CI
    npx lhci autorun 2>&1 | tee ../lighthouse-results.txt || {
        echo -e "${COLOR_YELLOW}⚠ Lighthouse CI failed (this is normal if server is not running)${COLOR_RESET}"
        echo "  To run manually:"
        echo "    1. Start preview server: npm run preview"
        echo "    2. In another terminal: npx lhci autorun"
    }
else
    echo -e "${COLOR_YELLOW}⚠ Lighthouse CI not installed, skipping${COLOR_RESET}"
    echo "  To install: npm install --save-dev @lhci/cli"
    echo "  To run manually:"
    echo "    1. npm run build"
    echo "    2. npm run preview (in one terminal)"
    echo "    3. npx lhci autorun (in another terminal)"
fi

echo ""

cd ..

# ---------------------------------------------------------------------------
# Core Web Vitals Monitoring
# ---------------------------------------------------------------------------

echo -e "${COLOR_YELLOW}Step 4: Core Web Vitals configuration...${COLOR_RESET}"
echo ""

echo "Core Web Vitals monitoring is configured in:"
echo "  - catalog-web/src/reportWebVitals.ts"
echo "  - catalog-web/src/main.tsx"
echo ""
echo "Metrics tracked:"
echo "  - LCP (Largest Contentful Paint) - Target: < 2.5s"
echo "  - FID (First Input Delay) - Target: < 100ms"
echo "  - CLS (Cumulative Layout Shift) - Target: < 0.1"
echo "  - FCP (First Contentful Paint) - Target: < 1.8s"
echo "  - TTFB (Time to First Byte) - Target: < 600ms"
echo ""
echo "To view metrics in production:"
echo "  1. Open browser DevTools"
echo "  2. Go to Console"
echo "  3. Look for 'Web Vitals:' log entries"
echo ""

# ---------------------------------------------------------------------------
# Performance Summary
# ---------------------------------------------------------------------------

echo -e "${COLOR_BLUE}=== Performance Testing Summary ===${COLOR_RESET}"
echo ""

echo "Results saved to:"
echo "  - performance-results-protocol.txt (Protocol client benchmarks)"
echo "  - performance-results-database.txt (Database benchmarks)"
echo "  - performance-results-api.txt (API endpoint benchmarks)"
echo "  - performance-results-auth.txt (Auth service benchmarks)"
echo "  - performance-results-detector.txt (Media detector benchmarks)"
echo "  - performance-results-providers.txt (Media provider benchmarks)"
echo "  - performance-results-smb.txt (SMB resilience benchmarks)"
echo "  - bundle-analysis.html (Bundle size analysis)"
echo "  - lighthouse-results.txt (Lighthouse CI results)"
echo ""

echo -e "${COLOR_GREEN}✓ Performance testing complete${COLOR_RESET}"
echo ""

# ---------------------------------------------------------------------------
# Performance Recommendations
# ---------------------------------------------------------------------------

echo -e "${COLOR_YELLOW}Performance Optimization Recommendations:${COLOR_RESET}"
echo ""

echo "Backend:"
echo "  - Review benchmark results for operations > 10ms"
echo "  - Check memory allocations (B/op column) for high-frequency operations"
echo "  - Optimize database queries with EXPLAIN ANALYZE"
echo "  - Consider caching for frequently accessed data"
echo ""

echo "Frontend:"
echo "  - Keep bundle size < 500KB (gzipped)"
echo "  - Implement code splitting for large components"
echo "  - Use React.lazy() for route-based code splitting"
echo "  - Optimize images (WebP format, lazy loading)"
echo "  - Implement virtual scrolling for large lists"
echo ""

echo "Monitoring:"
echo "  - Set up Prometheus for production metrics"
echo "  - Enable Grafana dashboards for real-time monitoring"
echo "  - Track Core Web Vitals in production"
echo "  - Set up alerts for performance degradation"
echo ""

exit 0
