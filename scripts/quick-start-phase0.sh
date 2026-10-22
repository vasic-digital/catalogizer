#!/bin/bash
#
# QUICK START GUIDE - PHASE 0: FOUNDATION & INFRASTRUCTURE
# Run this script to set up the foundation for comprehensive improvements
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     CATALOGIZER - PHASE 0 QUICK START                      ║${NC}"
echo -e "${BLUE}║     Foundation & Infrastructure Setup                      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check if running as root
if [ "$EUID" -eq 0 ]; then 
   echo -e "${RED}ERROR: Please do not run this script as root${NC}"
   exit 1
fi

# Function to check if command exists
command_exists() {
    command -v "$1" &> /dev/null
}

# Function to print status
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

# ==========================================
# STEP 1: CHECK PREREQUISITES
# ==========================================
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}STEP 1: CHECKING PREREQUISITES${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

PREREQS_MET=true

# Check Go
if command_exists go; then
    GO_VERSION=$(go version | awk '{print $3}')
    print_success "Go installed: $GO_VERSION"
else
    print_error "Go not installed. Please install Go 1.21+"
    PREREQS_MET=false
fi

# Check Node.js
if command_exists node; then
    NODE_VERSION=$(node --version)
    print_success "Node.js installed: $NODE_VERSION"
else
    print_error "Node.js not installed. Please install Node.js 18+"
    PREREQS_MET=false
fi

# Check npm
if command_exists npm; then
    NPM_VERSION=$(npm --version)
    print_success "npm installed: $NPM_VERSION"
else
    print_error "npm not installed"
    PREREQS_MET=false
fi

# Check Podman
if command_exists podman; then
    PODMAN_VERSION=$(podman --version)
    print_success "Podman installed: $PODMAN_VERSION"
else
    print_warning "Podman not installed. Container builds will not work"
fi

# Check Python (for pre-commit)
if command_exists python3; then
    PYTHON_VERSION=$(python3 --version)
    print_success "Python installed: $PYTHON_VERSION"
else
    print_warning "Python not installed. Pre-commit hooks require Python"
fi

# Check pip
if command_exists pip3; then
    print_success "pip installed"
else
    print_warning "pip not installed. Cannot install pre-commit"
fi

if [ "$PREREQS_MET" = false ]; then
    echo ""
    print_error "Some prerequisites are missing. Please install them first."
    exit 1
fi

print_success "All essential prerequisites met!"

# ==========================================
# STEP 2: INSTALL SECURITY TOOLS
# ==========================================
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}STEP 2: INSTALLING SECURITY TOOLS${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# RULE-CONST-001 — install into $HOME/bin (user-level), never
# /usr/local/bin (requires sudo). Operator must ensure $HOME/bin
# is on PATH; the script adds it to the *current* shell so
# subsequent command_exists checks succeed, and prints the
# profile-file hint at the end.
USER_BIN="$HOME/bin"
mkdir -p "$USER_BIN"
case ":$PATH:" in
    *":$USER_BIN:"*) ;;
    *) export PATH="$USER_BIN:$PATH" ;;
esac

# Create temp directory
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Install Trivy
if command_exists trivy; then
    print_success "Trivy already installed"
else
    print_status "Installing Trivy..."
    curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b "$USER_BIN"
    if command_exists trivy; then
        print_success "Trivy installed successfully"
    else
        print_error "Failed to install Trivy"
    fi
fi

# Install Gosec
if command_exists gosec; then
    print_success "Gosec already installed"
else
    print_status "Installing Gosec..."
    curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b "$USER_BIN"
    if command_exists gosec; then
        print_success "Gosec installed successfully"
    else
        print_error "Failed to install Gosec"
    fi
fi

# Install Nancy
if command_exists nancy; then
    print_success "Nancy already installed"
else
    print_status "Installing Nancy..."
    curl -sL -o "$USER_BIN/nancy" https://github.com/sonatype-nexus-community/nancy/releases/latest/download/nancy-linux.amd64
    chmod +x "$USER_BIN/nancy"
    if command_exists nancy; then
        print_success "Nancy installed successfully"
    else
        print_error "Failed to install Nancy"
    fi
fi

# Install Syft
if command_exists syft; then
    print_success "Syft already installed"
else
    print_status "Installing Syft..."
    curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b "$USER_BIN"
    if command_exists syft; then
        print_success "Syft installed successfully"
    else
        print_error "Failed to install Syft"
    fi
fi

# Install pre-commit
if command_exists pre-commit; then
    print_success "pre-commit already installed"
else
    if command_exists pip3; then
        print_status "Installing pre-commit..."
        pip3 install --user pre-commit
        if command_exists pre-commit; then
            print_success "pre-commit installed successfully"
        else
            print_warning "pre-commit installed but not in PATH. You may need to restart your shell."
        fi
    else
        print_warning "pip3 not available, skipping pre-commit installation"
    fi
fi

# Cleanup
rm -rf "$TEMP_DIR"

# ==========================================
# STEP 3: SETUP PROJECT
# ==========================================
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}STEP 3: SETTING UP PROJECT${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Change to project directory
cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer

# Create necessary directories
print_status "Creating project directories..."
mkdir -p reports/{security,coverage,tests,ci,sbom}
mkdir -p build
mkdir -p temp
print_success "Directories created"

# Update git submodules
print_status "Updating git submodules..."
# Safe sequence: never descend through constitution/ (docs/GOVERNANCE.md section 5)
bash "$(dirname "${BASH_SOURCE[0]}")/safe_submodule_init.sh"
print_success "Submodules updated"

# ==========================================
# STEP 4: CONFIGURE PRE-COMMIT HOOKS
# ==========================================
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}STEP 4: CONFIGURING PRE-COMMIT HOOKS${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

if command_exists pre-commit; then
    print_status "Installing pre-commit hooks..."
    pre-commit install
    print_success "Pre-commit hooks installed"
    
    print_status "Creating secret detection baseline..."
    if [ ! -f .secrets.baseline ]; then
        detect-secrets scan --all-files > .secrets.baseline 2>/dev/null || print_warning "Could not create secrets baseline (detect-secrets not installed)"
    fi
else
    print_warning "pre-commit not available, skipping hook installation"
fi

# ==========================================
# STEP 5: VALIDATE INSTALLATION
# ==========================================
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}STEP 5: VALIDATING INSTALLATION${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

print_status "Checking installed tools..."

TOOLS_OK=true

for tool in trivy gosec nancy syft; do
    if command_exists $tool; then
        VERSION=$($tool --version 2>&1 | head -1)
        print_success "$tool: $VERSION"
    else
        print_error "$tool: NOT FOUND"
        TOOLS_OK=false
    fi
done

if [ "$TOOLS_OK" = true ]; then
    print_success "All security tools installed successfully"
else
    print_warning "Some tools failed to install. You may need to install them manually."
fi

# ==========================================
# STEP 6: RUN INITIAL TESTS
# ==========================================
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}STEP 6: RUNNING INITIAL TESTS${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Test Go build
print_status "Testing Go backend build..."
cd catalog-api
if go build -o ../build/catalog-api-test 2>/dev/null; then
    print_success "Go backend builds successfully"
    rm ../build/catalog-api-test
else
    print_error "Go backend build failed"
fi
cd ..

# Test npm install
print_status "Testing Web frontend setup..."
cd catalog-web
if npm ci 2>/dev/null; then
    print_success "Web frontend dependencies installed"
else
    print_error "Web frontend setup failed"
fi
cd ..

# ==========================================
# STEP 7: GENERATE REPORTS
# ==========================================
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}STEP 7: GENERATING INITIAL REPORTS${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Generate coverage baseline
print_status "Generating coverage baseline..."
cd catalog-api
GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -coverprofile=../reports/coverage/baseline.out 2>/dev/null || print_warning "Some tests failed, but coverage baseline created"
go tool cover -func=../reports/coverage/baseline.out | grep total | awk '{print "Total Go coverage: " $3}'
cd ..

# Try to run security scan
if command_exists trivy; then
    print_status "Running initial security scan (this may take a few minutes)..."
    trivy filesystem --scanners vuln,secret,config --format json --output reports/security/baseline-scan.json . 2>/dev/null || print_warning "Security scan completed with some issues (see report)"
    print_success "Initial security scan complete"
fi

# ==========================================
# COMPLETION
# ==========================================
echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║     PHASE 0 SETUP COMPLETE!                                ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Next Steps:${NC}"
echo ""
echo "1. Review the comprehensive plan:"
echo "   cat COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md"
echo ""
echo "2. Review Phase 0 details:"
echo "   cat docs/phases/PHASE_0_FOUNDATION.md"
echo ""
echo "3. Run local CI pipeline:"
echo "   ./scripts/local-ci.sh"
echo ""
echo "4. Run security scan:"
echo "   ./scripts/security-scan-full.sh"
echo ""
echo "5. Track coverage:"
echo "   ./scripts/track-coverage.sh"
echo ""
echo "6. Start implementing Phase 1 (Test Coverage):"
echo "   cat docs/phases/PHASE_1_TEST_COVERAGE.md"
echo ""
echo -e "${BLUE}Useful Commands:${NC}"
echo "  ./scripts/local-ci.sh          - Run local CI pipeline"
echo "  ./scripts/security-scan-full.sh - Run security scans"
echo "  ./scripts/track-coverage.sh    - Track test coverage"
echo "  pre-commit run --all-files     - Run pre-commit hooks"
echo ""
echo -e "${YELLOW}Note:${NC} Make sure to set up Snyk and SonarQube tokens in .env.security"
echo ""
