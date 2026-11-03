#!/usr/bin/env bash
# ============================================================================
# Catalogizer - Local CI/CD Runner
# ============================================================================
# Runs all checks, tests, builds, and security scans locally.
# Designed to replace GitHub Actions (which are permanently disabled).
#
# Usage:
#   ./scripts/ci-local.sh              # Run all phases
#   ./scripts/ci-local.sh --quick      # Skip slow tests (build only)
#   ./scripts/ci-local.sh --go-only    # Go backend only
#   ./scripts/ci-local.sh --web-only   # Web frontend only
#   ./scripts/ci-local.sh --race       # Enable Go race detector
#   ./scripts/ci-local.sh --all        # All phases (default)
#   ./scripts/ci-local.sh --help       # Show help
#
# Exit codes:
#   0 = All phases passed
#   1 = One or more phases failed
#   2 = Pre-flight check failed (missing tools)
# ============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Constants and configuration
# ---------------------------------------------------------------------------
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
readonly REPORTS_DIR="$PROJECT_ROOT/reports"
readonly TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
readonly LOG_FILE="$REPORTS_DIR/ci-local-${TIMESTAMP}.log"

# Colors (disabled automatically when not a terminal)
if [[ -t 1 ]]; then
    readonly RED='\033[0;31m'
    readonly GREEN='\033[0;32m'
    readonly YELLOW='\033[1;33m'
    readonly BLUE='\033[0;34m'
    readonly CYAN='\033[0;36m'
    readonly BOLD='\033[1m'
    readonly DIM='\033[2m'
    readonly NC='\033[0m'
else
    readonly RED=''
    readonly GREEN=''
    readonly YELLOW=''
    readonly BLUE=''
    readonly CYAN=''
    readonly BOLD=''
    readonly DIM=''
    readonly NC=''
fi

# Phase tracking
declare -a PHASE_NAMES=()
declare -a PHASE_STATUSES=()
declare -a PHASE_DURATIONS=()
TOTAL_PHASES=0
PASSED_PHASES=0
FAILED_PHASES=0
SKIPPED_PHASES=0

# Flags (defaults)
FLAG_QUICK=false
FLAG_GO_ONLY=false
FLAG_WEB_ONLY=false
FLAG_RACE=false
FLAG_ALL=true
FLAG_VERBOSE=false

# Global timer
PIPELINE_START=0

# ---------------------------------------------------------------------------
# Logging and output
# ---------------------------------------------------------------------------
_log_raw() {
    local msg="$1"
    # Strip ANSI codes for the log file
    local clean
    clean="$(printf '%b' "$msg" | sed 's/\x1b\[[0-9;]*m//g')"
    printf '%s\n' "$clean" >> "$LOG_FILE"
}

log_info() {
    local msg="[INFO] $1"
    echo -e "${GREEN}${msg}${NC}"
    _log_raw "$msg"
}

log_warn() {
    local msg="[WARN] $1"
    echo -e "${YELLOW}${msg}${NC}"
    _log_raw "$msg"
}

log_error() {
    local msg="[ERROR] $1"
    echo -e "${RED}${msg}${NC}"
    _log_raw "$msg"
}

log_step() {
    local msg="  -> $1"
    echo -e "${BLUE}${msg}${NC}"
    _log_raw "$msg"
}

# Print a banner separator
banner() {
    local text="$1"
    local width=60
    local pad=$(( (width - ${#text} - 2) / 2 ))
    local line
    line="$(printf '%*s' "$width" '' | tr ' ' '=')"
    echo ""
    echo -e "${CYAN}${line}${NC}"
    printf "${CYAN}%*s %s %*s${NC}\n" "$pad" "" "$text" "$pad" ""
    echo -e "${CYAN}${line}${NC}"
    echo ""
    _log_raw "$line"
    _log_raw "$text"
    _log_raw "$line"
}

# Print the opening banner
print_header() {
    echo ""
    echo -e "${CYAN}${BOLD}"
    echo "  ================================================================"
    echo "    Catalogizer - Local CI/CD Pipeline"
    echo "  ================================================================"
    echo -e "    Started:  $(date)"
    echo -e "    Project:  ${PROJECT_ROOT}"
    echo -e "    Log:      ${LOG_FILE}"
    echo "  ================================================================"
    echo -e "${NC}"
}

# ---------------------------------------------------------------------------
# Timing helpers
# ---------------------------------------------------------------------------
now_seconds() {
    date +%s
}

format_duration() {
    local total_seconds="$1"
    local minutes=$(( total_seconds / 60 ))
    local seconds=$(( total_seconds % 60 ))
    if (( minutes > 0 )); then
        printf '%dm %ds' "$minutes" "$seconds"
    else
        printf '%ds' "$seconds"
    fi
}

# ---------------------------------------------------------------------------
# Phase lifecycle
# ---------------------------------------------------------------------------
# Call at the start of a phase. Sets PHASE_START_TIME.
phase_start() {
    local name="$1"
    PHASE_START_TIME="$(now_seconds)"
    banner "Phase $((TOTAL_PHASES + 1)): $name"
}

# Record a phase result.
# Usage: phase_end "Phase Name" "PASS"|"FAIL"|"SKIP"
phase_end() {
    local name="$1"
    local status="$2"
    local end_time
    end_time="$(now_seconds)"
    local duration=$(( end_time - PHASE_START_TIME ))

    PHASE_NAMES+=("$name")
    PHASE_STATUSES+=("$status")
    PHASE_DURATIONS+=("$duration")
    (( TOTAL_PHASES++ )) || true

    case "$status" in
        PASS)
            (( PASSED_PHASES++ )) || true
            log_info "Phase '${name}' PASSED ($(format_duration "$duration"))"
            ;;
        FAIL)
            (( FAILED_PHASES++ )) || true
            log_error "Phase '${name}' FAILED ($(format_duration "$duration"))"
            ;;
        SKIP)
            (( SKIPPED_PHASES++ )) || true
            log_warn "Phase '${name}' SKIPPED"
            ;;
    esac
}

# Run a command, tee-ing output to the log file.
# Returns the command's exit code without aborting (due to set -e).
run_cmd() {
    local label="$1"
    shift
    log_step "$label"
    _log_raw "CMD: $*"
    local rc=0
    "$@" 2>&1 | tee -a "$LOG_FILE" || rc=$?
    return "$rc"
}

# Quieter version -- captures output, only shows on failure.
run_cmd_quiet() {
    local label="$1"
    shift
    log_step "$label"
    _log_raw "CMD: $*"
    local output
    local rc=0
    output="$("$@" 2>&1)" || rc=$?
    printf '%s\n' "$output" >> "$LOG_FILE"
    if (( rc != 0 )); then
        echo "$output" | tail -30
    fi
    return "$rc"
}

# ---------------------------------------------------------------------------
# Phase implementations
# ---------------------------------------------------------------------------

# Phase 1: Pre-flight checks
phase_preflight() {
    phase_start "Pre-flight Checks"
    local ok=true

    # Verify working directory
    if [[ ! -f "$PROJECT_ROOT/catalog-api/go.mod" ]]; then
        log_error "Cannot find catalog-api/go.mod -- wrong project root?"
        ok=false
    else
        log_info "Working directory verified: $PROJECT_ROOT"
    fi

    # Required tools
    local required_tools=("go" "node" "npm")
    for tool in "${required_tools[@]}"; do
        if command -v "$tool" &>/dev/null; then
            log_info "$tool found: $(command -v "$tool")"
        else
            log_error "Required tool not found: $tool"
            ok=false
        fi
    done

    # Optional tools (warn but do not fail)
    local optional_tools=("rustc" "cargo")
    for tool in "${optional_tools[@]}"; do
        if command -v "$tool" &>/dev/null; then
            log_info "$tool found: $(command -v "$tool")"
        else
            log_warn "Optional tool not found: $tool (Tauri builds will be skipped)"
        fi
    done

    # Version checks
    if command -v go &>/dev/null; then
        local go_version
        go_version="$(go version 2>&1)"
        log_info "Go version: $go_version"
    fi

    if command -v node &>/dev/null; then
        log_info "Node version: $(node --version 2>&1)"
    fi

    if command -v npm &>/dev/null; then
        log_info "npm version: $(npm --version 2>&1)"
    fi

    if command -v rustc &>/dev/null; then
        log_info "Rust version: $(rustc --version 2>&1)"
    fi

    if [[ "$ok" == true ]]; then
        phase_end "Pre-flight Checks" "PASS"
        return 0
    else
        phase_end "Pre-flight Checks" "FAIL"
        return 1
    fi
}

# Phase 2: Go Backend (catalog-api)
phase_go_backend() {
    phase_start "Go Backend (catalog-api)"

    if [[ ! -d "$PROJECT_ROOT/catalog-api" ]]; then
        log_error "catalog-api directory not found"
        phase_end "Go Backend" "FAIL"
        return 1
    fi

    local phase_ok=true
    local go_test_flags=("-v" "-count=1")
    if [[ "$FLAG_RACE" == true ]]; then
        go_test_flags+=("-race")
        log_info "Race detector enabled"
    fi

    cd "$PROJECT_ROOT/catalog-api"

    # go vet
    log_step "Running go vet..."
    if GOTOOLCHAIN=local go vet ./... >> "$LOG_FILE" 2>&1; then
        log_info "go vet: PASSED"
    else
        log_error "go vet: FAILED"
        phase_ok=false
    fi

    # go build
    log_step "Running go build..."
    if CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOTOOLCHAIN=local go build ./... >> "$LOG_FILE" 2>&1; then
        log_info "go build: PASSED"
    else
        log_error "go build: FAILED"
        phase_ok=false
    fi

    # go test
    if [[ "$FLAG_QUICK" == false ]]; then
        log_step "Running go test ${go_test_flags[*]} ./..."
        local test_output
        local test_rc=0
        test_output="$(CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOTOOLCHAIN=local go test "${go_test_flags[@]}" ./... 2>&1)" || test_rc=$?
        printf '%s\n' "$test_output" >> "$LOG_FILE"

        # Count pass/fail from output
        local go_pass go_fail go_total
        go_pass="$(echo "$test_output" | grep -c '^ok' || true)"
        go_fail="$(echo "$test_output" | grep -c '^FAIL' || true)"
        go_total=$(( go_pass + go_fail ))

        if (( test_rc == 0 )); then
            log_info "go test: PASSED ($go_pass/$go_total packages)"
        else
            log_error "go test: FAILED ($go_fail/$go_total packages failed)"
            # Show failing lines
            echo "$test_output" | grep '^FAIL' | head -20
            phase_ok=false
        fi
    else
        log_warn "go test: SKIPPED (--quick mode)"
    fi

    cd "$PROJECT_ROOT"

    if [[ "$phase_ok" == true ]]; then
        phase_end "Go Backend" "PASS"
    else
        phase_end "Go Backend" "FAIL"
    fi
}

# Phase 3: Web Frontend (catalog-web)
phase_web_frontend() {
    phase_start "Web Frontend (catalog-web)"

    if [[ ! -d "$PROJECT_ROOT/catalog-web" ]] || [[ ! -f "$PROJECT_ROOT/catalog-web/package.json" ]]; then
        log_error "catalog-web directory or package.json not found"
        phase_end "Web Frontend" "FAIL"
        return 1
    fi

    local phase_ok=true
    cd "$PROJECT_ROOT/catalog-web"

    # npm install (if node_modules missing)
    if [[ ! -d "node_modules" ]]; then
        log_step "Installing npm dependencies..."
        if npm install --no-audit --no-fund >> "$LOG_FILE" 2>&1; then
            log_info "npm install: DONE"
        else
            log_error "npm install: FAILED"
            phase_end "Web Frontend" "FAIL"
            cd "$PROJECT_ROOT"
            return 1
        fi
    else
        log_info "node_modules already present, skipping install"
    fi

    # type-check
    log_step "Running type-check..."
    if npm run type-check >> "$LOG_FILE" 2>&1; then
        log_info "type-check: PASSED"
    else
        log_error "type-check: FAILED"
        phase_ok=false
    fi

    # lint
    log_step "Running lint..."
    if npm run lint >> "$LOG_FILE" 2>&1; then
        log_info "lint: PASSED"
    else
        log_warn "lint: FAILED (non-blocking)"
    fi

    # tests
    if [[ "$FLAG_QUICK" == false ]]; then
        log_step "Running vitest..."
        if npx vitest run >> "$LOG_FILE" 2>&1; then
            log_info "vitest: PASSED"
        else
            log_error "vitest: FAILED"
            phase_ok=false
        fi
    else
        log_warn "vitest: SKIPPED (--quick mode)"
    fi

    # build
    log_step "Running production build..."
    if npm run build >> "$LOG_FILE" 2>&1; then
        log_info "build: PASSED"
    else
        log_error "build: FAILED"
        phase_ok=false
    fi

    cd "$PROJECT_ROOT"

    if [[ "$phase_ok" == true ]]; then
        phase_end "Web Frontend" "PASS"
    else
        phase_end "Web Frontend" "FAIL"
    fi
}

# Phase 4: Desktop App (catalogizer-desktop)
phase_desktop_app() {
    phase_start "Desktop App (catalogizer-desktop)"

    if [[ ! -d "$PROJECT_ROOT/catalogizer-desktop" ]] || [[ ! -f "$PROJECT_ROOT/catalogizer-desktop/package.json" ]]; then
        log_warn "catalogizer-desktop not found"
        phase_end "Desktop App" "SKIP"
        return 0
    fi

    local phase_ok=true
    cd "$PROJECT_ROOT/catalogizer-desktop"

    # npm install
    if [[ ! -d "node_modules" ]]; then
        log_step "Installing npm dependencies..."
        if npm install --no-audit --no-fund >> "$LOG_FILE" 2>&1; then
            log_info "npm install: DONE"
        else
            log_error "npm install: FAILED"
            phase_end "Desktop App" "FAIL"
            cd "$PROJECT_ROOT"
            return 1
        fi
    else
        log_info "node_modules already present, skipping install"
    fi

    # tests (if they exist)
    if [[ "$FLAG_QUICK" == false ]]; then
        # Check if there are any test files before running vitest
        local test_count
        test_count="$(find . -path ./node_modules -prune -o -name '*.test.ts' -print -o -name '*.test.tsx' -print 2>/dev/null | wc -l)"
        if (( test_count > 0 )); then
            log_step "Running vitest ($test_count test files found)..."
            if npx vitest run >> "$LOG_FILE" 2>&1; then
                log_info "vitest: PASSED"
            else
                log_error "vitest: FAILED"
                phase_ok=false
            fi
        else
            log_warn "No test files found, skipping vitest"
        fi
    else
        log_warn "vitest: SKIPPED (--quick mode)"
    fi

    cd "$PROJECT_ROOT"

    if [[ "$phase_ok" == true ]]; then
        phase_end "Desktop App" "PASS"
    else
        phase_end "Desktop App" "FAIL"
    fi
}

# Phase 5: Installer Wizard
phase_installer_wizard() {
    phase_start "Installer Wizard"

    if [[ ! -d "$PROJECT_ROOT/installer-wizard" ]] || [[ ! -f "$PROJECT_ROOT/installer-wizard/package.json" ]]; then
        log_warn "installer-wizard not found"
        phase_end "Installer Wizard" "SKIP"
        return 0
    fi

    local phase_ok=true
    cd "$PROJECT_ROOT/installer-wizard"

    # npm install
    if [[ ! -d "node_modules" ]]; then
        log_step "Installing npm dependencies..."
        if npm install --no-audit --no-fund >> "$LOG_FILE" 2>&1; then
            log_info "npm install: DONE"
        else
            log_error "npm install: FAILED"
            phase_end "Installer Wizard" "FAIL"
            cd "$PROJECT_ROOT"
            return 1
        fi
    else
        log_info "node_modules already present, skipping install"
    fi

    # tests
    if [[ "$FLAG_QUICK" == false ]]; then
        log_step "Running vitest..."
        if npx vitest run >> "$LOG_FILE" 2>&1; then
            log_info "vitest: PASSED"
        else
            log_error "vitest: FAILED"
            phase_ok=false
        fi
    else
        log_warn "vitest: SKIPPED (--quick mode)"
    fi

    cd "$PROJECT_ROOT"

    if [[ "$phase_ok" == true ]]; then
        phase_end "Installer Wizard" "PASS"
    else
        phase_end "Installer Wizard" "FAIL"
    fi
}

# Phase 6: API Client
phase_api_client() {
    phase_start "API Client (catalogizer-api-client)"

    if [[ ! -d "$PROJECT_ROOT/catalogizer-api-client" ]] || [[ ! -f "$PROJECT_ROOT/catalogizer-api-client/package.json" ]]; then
        log_warn "catalogizer-api-client not found"
        phase_end "API Client" "SKIP"
        return 0
    fi

    local phase_ok=true
    cd "$PROJECT_ROOT/catalogizer-api-client"

    # npm install
    if [[ ! -d "node_modules" ]]; then
        log_step "Installing npm dependencies..."
        if npm install --no-audit --no-fund >> "$LOG_FILE" 2>&1; then
            log_info "npm install: DONE"
        else
            log_error "npm install: FAILED"
            phase_end "API Client" "FAIL"
            cd "$PROJECT_ROOT"
            return 1
        fi
    else
        log_info "node_modules already present, skipping install"
    fi

    # build
    log_step "Running build..."
    if npm run build >> "$LOG_FILE" 2>&1; then
        log_info "build: PASSED"
    else
        log_error "build: FAILED"
        phase_ok=false
    fi

    # tests
    if [[ "$FLAG_QUICK" == false ]]; then
        local test_count
        test_count="$(find . -path ./node_modules -prune -o -name '*.test.ts' -print -o -name '*.test.tsx' -print 2>/dev/null | wc -l)"
        if (( test_count > 0 )); then
            log_step "Running tests ($test_count test files found)..."
            if npm test >> "$LOG_FILE" 2>&1; then
                log_info "tests: PASSED"
            else
                log_error "tests: FAILED"
                phase_ok=false
            fi
        else
            log_warn "No test files found, skipping tests"
        fi
    else
        log_warn "tests: SKIPPED (--quick mode)"
    fi

    cd "$PROJECT_ROOT"

    if [[ "$phase_ok" == true ]]; then
        phase_end "API Client" "PASS"
    else
        phase_end "API Client" "FAIL"
    fi
}

# Phase 7: Security Scans
phase_security() {
    phase_start "Security Scans"

    local phase_ok=true
    local js_projects=("catalog-web" "catalogizer-desktop" "installer-wizard" "catalogizer-api-client")

    # npm audit for each JS project
    for project in "${js_projects[@]}"; do
        local project_dir="$PROJECT_ROOT/$project"
        if [[ -d "$project_dir" ]] && [[ -d "$project_dir/node_modules" ]]; then
            log_step "npm audit: $project"
            local audit_output
            local audit_rc=0
            audit_output="$(cd "$project_dir" && npm audit --audit-level=critical 2>&1)" || audit_rc=$?
            printf '%s\n' "$audit_output" >> "$LOG_FILE"

            if (( audit_rc == 0 )); then
                log_info "npm audit $project: CLEAN"
            else
                # Only fail on critical vulnerabilities
                if echo "$audit_output" | grep -qi 'critical'; then
                    log_warn "npm audit $project: critical vulnerabilities found"
                else
                    log_info "npm audit $project: no critical vulnerabilities (non-critical findings ignored)"
                fi
            fi
        else
            log_warn "npm audit $project: SKIPPED (node_modules not present)"
        fi
    done

    # go vet was already run in Phase 2, so just note it
    log_info "go vet: already executed in Go Backend phase"

    # If gosec is available, run it
    if command -v gosec &>/dev/null; then
        log_step "Running gosec on catalog-api..."
        if (cd "$PROJECT_ROOT/catalog-api" && gosec -quiet ./...) >> "$LOG_FILE" 2>&1; then
            log_info "gosec: PASSED"
        else
            log_warn "gosec: findings detected (review log)"
        fi
    fi

    # Security phase is informational -- does not fail the pipeline
    phase_end "Security Scans" "PASS"
}

# ---------------------------------------------------------------------------
# Summary report
# ---------------------------------------------------------------------------
print_summary() {
    local pipeline_end
    pipeline_end="$(now_seconds)"
    local total_duration=$(( pipeline_end - PIPELINE_START ))

    echo ""
    echo -e "${CYAN}${BOLD}"
    echo "  ================================================================"
    echo "    CI/CD Pipeline Summary"
    echo "  ================================================================"
    echo -e "${NC}"

    # Table header
    printf "  ${BOLD}%-4s  %-35s  %-8s  %s${NC}\n" "#" "Phase" "Status" "Duration"
    printf "  %-4s  %-35s  %-8s  %s\n" "----" "-----------------------------------" "--------" "--------"

    # Table rows
    local i
    for (( i = 0; i < ${#PHASE_NAMES[@]}; i++ )); do
        local name="${PHASE_NAMES[$i]}"
        local status="${PHASE_STATUSES[$i]}"
        local duration="${PHASE_DURATIONS[$i]}"
        local color=""
        local status_label=""

        case "$status" in
            PASS)
                color="$GREEN"
                status_label="PASS"
                ;;
            FAIL)
                color="$RED"
                status_label="FAIL"
                ;;
            SKIP)
                color="$YELLOW"
                status_label="SKIP"
                ;;
        esac

        printf "  %-4s  %-35s  ${color}%-8s${NC}  %s\n" \
            "$((i + 1))" "$name" "$status_label" "$(format_duration "$duration")"
    done

    echo ""
    printf "  %-4s  %-35s  %-8s  %s\n" "----" "-----------------------------------" "--------" "--------"

    # Totals
    echo -e "  ${BOLD}Total phases:${NC}   $TOTAL_PHASES"
    echo -e "  ${GREEN}Passed:${NC}         $PASSED_PHASES"
    echo -e "  ${RED}Failed:${NC}         $FAILED_PHASES"
    echo -e "  ${YELLOW}Skipped:${NC}        $SKIPPED_PHASES"
    echo -e "  ${BOLD}Total time:${NC}     $(format_duration "$total_duration")"
    echo ""
    echo -e "  ${DIM}Log file:${NC}       $LOG_FILE"
    echo ""

    if (( FAILED_PHASES > 0 )); then
        echo -e "  ${RED}${BOLD}RESULT: PIPELINE FAILED${NC}"
        echo -e "  ${RED}$FAILED_PHASES phase(s) failed. Review the log for details.${NC}"
    else
        echo -e "  ${GREEN}${BOLD}RESULT: PIPELINE PASSED${NC}"
        echo -e "  ${GREEN}All phases completed successfully.${NC}"
    fi

    echo ""
    echo -e "${CYAN}  ================================================================${NC}"
    echo ""
}

# ---------------------------------------------------------------------------
# Usage / help
# ---------------------------------------------------------------------------
print_help() {
    cat <<'USAGE'
Catalogizer Local CI/CD Runner

Usage:
  ./scripts/ci-local.sh [OPTIONS]

Options:
  --all         Run all phases (default)
  --quick       Skip test execution; only run vet, build, type-check, lint
  --go-only     Run only Go backend phases (Phase 1-2)
  --web-only    Run only web frontend phases (Phase 1, 3)
  --race        Enable Go race detector during tests
  --verbose     Show full command output inline
  --help        Show this help message

Phases:
  1. Pre-flight    Check required tools, versions, working directory
  2. Go Backend    go vet, go build, go test (catalog-api)
  3. Web Frontend  type-check, lint, vitest, build (catalog-web)
  4. Desktop App   vitest (catalogizer-desktop)
  5. Wizard        vitest (installer-wizard)
  6. API Client    build, tests (catalogizer-api-client)
  7. Security      npm audit for JS projects, gosec if available
  8. Summary       Results table, exit code

Log output:
  All output is logged to reports/ci-local-TIMESTAMP.log

Exit codes:
  0  All phases passed
  1  One or more phases failed
  2  Pre-flight check failed

Examples:
  ./scripts/ci-local.sh                   # Full pipeline
  ./scripts/ci-local.sh --quick           # Fast: build + lint only
  ./scripts/ci-local.sh --go-only --race  # Go with race detector
  ./scripts/ci-local.sh --web-only        # Web frontend only
USAGE
}

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
parse_args() {
    while (( $# > 0 )); do
        case "$1" in
            --help|-h)
                print_help
                exit 0
                ;;
            --quick)
                FLAG_QUICK=true
                ;;
            --go-only)
                FLAG_GO_ONLY=true
                FLAG_ALL=false
                ;;
            --web-only)
                FLAG_WEB_ONLY=true
                FLAG_ALL=false
                ;;
            --race)
                FLAG_RACE=true
                ;;
            --all)
                FLAG_ALL=true
                FLAG_GO_ONLY=false
                FLAG_WEB_ONLY=false
                ;;
            --verbose)
                FLAG_VERBOSE=true
                ;;
            *)
                log_error "Unknown option: $1"
                echo "Use --help for usage information."
                exit 2
                ;;
        esac
        shift
    done
}

# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------
main() {
    parse_args "$@"

    # Create reports directory
    mkdir -p "$REPORTS_DIR"

    # Initialize log file
    {
        echo "Catalogizer Local CI/CD Log"
        echo "Started: $(date)"
        echo "Flags: quick=$FLAG_QUICK go_only=$FLAG_GO_ONLY web_only=$FLAG_WEB_ONLY race=$FLAG_RACE"
        echo "========================================"
    } > "$LOG_FILE"

    PIPELINE_START="$(now_seconds)"

    print_header

    # Show active flags
    local mode_desc="all phases"
    if [[ "$FLAG_GO_ONLY" == true ]]; then
        mode_desc="Go backend only"
    elif [[ "$FLAG_WEB_ONLY" == true ]]; then
        mode_desc="Web frontend only"
    fi
    if [[ "$FLAG_QUICK" == true ]]; then
        mode_desc="$mode_desc (quick mode -- tests skipped)"
    fi
    if [[ "$FLAG_RACE" == true ]]; then
        mode_desc="$mode_desc (race detector on)"
    fi
    log_info "Mode: $mode_desc"
    echo ""

    # -----------------------------------------------------------------------
    # Phase 1: Pre-flight (always runs)
    # -----------------------------------------------------------------------
    if ! phase_preflight; then
        print_summary
        exit 2
    fi

    # -----------------------------------------------------------------------
    # Phase 2: Go Backend
    # -----------------------------------------------------------------------
    if [[ "$FLAG_ALL" == true ]] || [[ "$FLAG_GO_ONLY" == true ]]; then
        phase_go_backend || true
    fi

    # -----------------------------------------------------------------------
    # Phase 3: Web Frontend
    # -----------------------------------------------------------------------
    if [[ "$FLAG_ALL" == true ]] || [[ "$FLAG_WEB_ONLY" == true ]]; then
        phase_web_frontend || true
    fi

    # -----------------------------------------------------------------------
    # Phase 4: Desktop App
    # -----------------------------------------------------------------------
    if [[ "$FLAG_ALL" == true ]]; then
        phase_desktop_app || true
    fi

    # -----------------------------------------------------------------------
    # Phase 5: Installer Wizard
    # -----------------------------------------------------------------------
    if [[ "$FLAG_ALL" == true ]]; then
        phase_installer_wizard || true
    fi

    # -----------------------------------------------------------------------
    # Phase 6: API Client
    # -----------------------------------------------------------------------
    if [[ "$FLAG_ALL" == true ]]; then
        phase_api_client || true
    fi

    # -----------------------------------------------------------------------
    # Phase 7: Security Scans
    # -----------------------------------------------------------------------
    if [[ "$FLAG_ALL" == true ]]; then
        phase_security || true
    fi

    # -----------------------------------------------------------------------
    # Phase 8: Summary
    # -----------------------------------------------------------------------
    print_summary

    # Write summary to log as well
    {
        echo ""
        echo "========================================"
        echo "SUMMARY"
        echo "Total: $TOTAL_PHASES  Passed: $PASSED_PHASES  Failed: $FAILED_PHASES  Skipped: $SKIPPED_PHASES"
        echo "Finished: $(date)"
        echo "========================================"
    } >> "$LOG_FILE"

    # Exit code
    if (( FAILED_PHASES > 0 )); then
        exit 1
    else
        exit 0
    fi
}

main "$@"
