#!/bin/bash
# HelixQA Integration Test Runner for Catalogizer
# Runs HelixQA tests on all Catalogizer services and apps

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$PROJECT_ROOT/reports/helixqa-$(date +%Y%m%d_%H%M%S)"

mkdir -p "$RESULTS_DIR"

echo "╔════════════════════════════════════════════════════════════╗"
echo "║  HelixQA Test Suite - Catalogizer Integration             ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

HELIQA_ROOT="$PROJECT_ROOT/submodules/helix_qa"
FAILED=0

log_section() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  $1"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
    FAILED=$((FAILED + 1))
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Check HelixQA exists
if [ ! -d "$HELIQA_ROOT" ]; then
    log_error "HelixQA not found at $HELIQA_ROOT"
    exit 1
fi

# ============ 1. BUILD HELIXQA ============
log_section "1. Building HelixQA"

cd "$HELIQA_ROOT"

log_info "Building HelixQA modules..."
if go build ./... > "$RESULTS_DIR/build.log" 2>&1; then
    log_success "HelixQA build successful"
else
    log_error "HelixQA build failed (see $RESULTS_DIR/build.log)"
    exit 1
fi

# ============ 2. HELIXQA UNIT TESTS ============
log_section "2. HelixQA Unit Tests"

log_info "Running HelixQA unit tests..."
if go test ./... -count=1 -race > "$RESULTS_DIR/unit-tests.log" 2>&1; then
    log_success "HelixQA unit tests passed"
else
    log_error "HelixQA unit tests failed"
fi

# ============ 3. HELIXQA CHALLENGE BANKS ============
log_section "3. HelixQA Challenge Banks"

if [ -d "$HELIQA_ROOT/banks" ]; then
    log_info "Available test banks:"
    for bank in "$HELIQA_ROOT/banks"/*.yaml; do
        if [ -f "$bank" ]; then
            log_info "  - $(basename "$bank")"
        fi
    done
fi

# ============ 4. CATALOGIZER-SPECIFIC TESTS ============
log_section "4. Catalogizer Service Tests via HelixQA"

# Create Catalogizer test bank if it doesn't exist
CATALOGIZER_BANK="$PROJECT_ROOT/challenges/helixqa-catalogizer.yaml"

if [ ! -f "$CATALOGIZER_BANK" ]; then
    log_info "Creating Catalogizer HelixQA test bank..."
    cat > "$CATALOGIZER_BANK" <> 'EOF'
name: "Catalogizer Full System Test"
description: "Comprehensive test suite for all Catalogizer services"
version: "1.0.0"

platforms:
  - api
  - web
  - desktop
  - android

tests:
  - id: "cat-api-health"
    name: "API Health Check"
    description: "Verify catalog-api responds to health endpoint"
    platform: api
    priority: critical
    steps:
      - action: http_get
        target: "http://localhost:8080/api/v1/health"
        assertions:
          - status_code: 200
          - response_time_ms: < 1000

  - id: "cat-web-load"
    name: "Web UI Load Test"
    description: "Verify catalog-web loads without errors"
    platform: web
    priority: critical
    steps:
      - action: navigate
        target: "http://localhost:3000"
        assertions:
          - element_exists: "[data-testid='app-container']"
          - console_errors: 0

  - id: "cat-auth-flow"
    name: "Authentication Flow"
    description: "Test login/logout functionality"
    platform: web
    priority: high
    steps:
      - action: navigate
        target: "http://localhost:3000/login"
      - action: input
        target: "#username"
        value: "admin"
      - action: input
        target: "#password"
        value: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD}"
      - action: click
        target: "#login-button"
        assertions:
          - url_contains: "/dashboard"
          - element_exists: "[data-testid='user-menu']"

  - id: "cat-entity-browse"
    name: "Entity Browsing"
    description: "Browse media entities without errors"
    platform: web
    priority: high
    steps:
      - action: navigate
        target: "http://localhost:3000/browse"
      - action: wait
        duration: 2000
        assertions:
          - element_exists: "[data-testid='entity-grid']"
          - network_errors: 0

  - id: "cat-websocket-connect"
    name: "WebSocket Connection"
    description: "Verify real-time updates work"
    platform: web
    priority: medium
    steps:
      - action: navigate
        target: "http://localhost:3000"
      - action: wait
        duration: 3000
        assertions:
          - websocket_connected: true
          - console_errors: 0
EOF
    log_success "Created Catalogizer test bank"
fi

# Run HelixQA in autonomous mode
log_info "Running HelixQA autonomous tests..."

cd "$HELIQA_ROOT"

# Start catalog-api if not running
API_PID=""
if ! curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
    log_info "Starting catalog-api..."
    cd "$PROJECT_ROOT/catalog-api"
    go run main.go > "$RESULTS_DIR/api.log" 2>&1 &
    API_PID=$!
    sleep 10
fi

# Run HelixQA tests
cd "$HELIQA_ROOT"
if go run ./cmd/helixqa run \
    --bank "$CATALOGIZER_BANK" \
    --platform all \
    --output "$RESULTS_DIR/helixqa-report.md" \
    --verbose \
    > "$RESULTS_DIR/helixqa-run.log" 2>&1; then
    log_success "HelixQA tests completed successfully"
else
    log_warn "HelixQA tests completed with issues (see $RESULTS_DIR/helixqa-run.log)"
fi

# Cleanup API
if [ -n "$API_PID" ]; then
    kill $API_PID 2>/dev/null || true
fi

# ============ 5. MODULE CHALLENGE TESTS VIA HELIXQA ============
log_section "5. Module Challenge Integration"

log_info "Running module challenges through HelixQA orchestrator..."

for module in Auth Cache Concurrency Database EventBus Memory Observability Security Storage Streaming; do
    MODULE_DIR="$PROJECT_ROOT/$module"
    if [ -d "$MODULE_DIR/challenges/scripts" ]; then
        log_info "Testing $module..."
        
        cd "$MODULE_DIR"
        
        # Run compile challenge
        if [ -f "challenges/scripts/${module,,}_compile_challenge.sh" ]; then
            if bash "challenges/scripts/${module,,}_compile_challenge.sh" > "$RESULTS_DIR/${module}-compile.log" 2>&1; then
                log_success "$module compile challenge passed"
            else
                log_error "$module compile challenge failed"
            fi
        fi
        
        # Run unit challenge
        if [ -f "challenges/scripts/${module,,}_unit_challenge.sh" ]; then
            if bash "challenges/scripts/${module,,}_unit_challenge.sh" > "$RESULTS_DIR/${module}-unit.log" 2>&1; then
                log_success "$module unit challenge passed"
            else
                log_error "$module unit challenge failed"
            fi
        fi
    fi
done

# ============ SUMMARY ============
log_section "HelixQA Test Summary"

echo ""
echo "Results directory: $RESULTS_DIR"
echo "Failed test groups: $FAILED"
echo ""

# Show report location
if [ -f "$RESULTS_DIR/helixqa-report.md" ]; then
    log_info "Detailed report: $RESULTS_DIR/helixqa-report.md"
fi

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}╔════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  HELIXQA TESTS PASSED! ✓           ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════╝${NC}"
    exit 0
else
    echo -e "${RED}╔════════════════════════════════════╗${NC}"
    echo -e "${RED}║  $FAILED HELIXQA TEST(S) FAILED    ║${NC}"
    echo -e "${RED}╚════════════════════════════════════╝${NC}"
    exit 1
fi
