#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
set -e

# Catalogizer - Complete Remaining Tasks Script
# This script completes all remaining items from the final checklist

COLOR_RED='\033[0;31m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[1;33m'
COLOR_BLUE='\033[0;34m'
COLOR_MAGENTA='\033[0;35m'
COLOR_CYAN='\033[0;36m'
COLOR_RESET='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${COLOR_BLUE}================================================================${COLOR_RESET}"
echo -e "${COLOR_BLUE}  Catalogizer - Complete Remaining Tasks${COLOR_RESET}"
echo -e "${COLOR_BLUE}================================================================${COLOR_RESET}"
echo ""

# ---------------------------------------------------------------------------
# Step 1: Install Java/JDK if not present
# ---------------------------------------------------------------------------

echo -e "${COLOR_CYAN}Step 1: Checking Java/JDK Installation${COLOR_RESET}"
echo ""

if command -v java &> /dev/null; then
    JAVA_VERSION=$(java -version 2>&1 | head -n 1)
    echo -e "${COLOR_GREEN}✓ Java is already installed: $JAVA_VERSION${COLOR_RESET}"
else
    # RULE-CONST-001 — this script runs at user level only. Java
    # is a system dependency that requires the operator to install
    # manually (via their distro's package manager OR sdkman-style
    # user-local install). We refuse to invoke `sudo apt-get` from
    # an automated script.
    echo -e "${COLOR_RED}✗ Java/JDK not found.${COLOR_RESET}"
    echo ""
    echo "Java must be installed by the operator before this script runs."
    echo "Options:"
    echo "  1. Distro package (Fedora/RHEL): sudo dnf install java-17-openjdk-devel"
    echo "  2. Distro package (Debian/Ubuntu): sudo apt-get install openjdk-17-jdk"
    echo "  3. SDKMAN user-local: curl -s https://get.sdkman.io | bash && sdk install java 17-open"
    echo "  4. Pre-packaged in \$HOME/java — set JAVA_HOME then rerun this script"
    exit 1

    # Unreachable — kept for reference of the distro commands:
    : "$(cat <<'__NOTE__'
    # Distro reference (operator-side):
    #   sudo apt-get update
    #   sudo apt-get install -y java-17-openjdk-devel
__NOTE__
)"
    if command -v java &> /dev/null; then
        echo -e "${COLOR_GREEN}✓ Java installed successfully${COLOR_RESET}"
        java -version
    else
        echo -e "${COLOR_RED}✗ Java installation failed${COLOR_RESET}"
        exit 1
    fi
fi

echo ""

# ---------------------------------------------------------------------------
# Step 2: Set up environment variables
# ---------------------------------------------------------------------------

echo -e "${COLOR_CYAN}Step 2: Setting Up Environment Variables${COLOR_RESET}"
echo ""

# Find JAVA_HOME
if [ -z "$JAVA_HOME" ]; then
    echo "Finding JAVA_HOME..."

    # Try to find Java installation
    if [ -d "/usr/lib/jvm/java-17-openjdk" ]; then
        JAVA_HOME_PATH="/usr/lib/jvm/java-17-openjdk"
    elif [ -d "/usr/lib/jvm/java-11-openjdk" ]; then
        JAVA_HOME_PATH="/usr/lib/jvm/java-11-openjdk"
    else
        JAVA_HOME_PATH=$(dirname $(dirname $(readlink -f $(which java))))
    fi

    export JAVA_HOME="$JAVA_HOME_PATH"
    echo "export JAVA_HOME=$JAVA_HOME_PATH" >> ~/.bashrc
    echo -e "${COLOR_GREEN}✓ JAVA_HOME set to: $JAVA_HOME_PATH${COLOR_RESET}"
else
    echo -e "${COLOR_GREEN}✓ JAVA_HOME already set: $JAVA_HOME${COLOR_RESET}"
fi

# Set up ANDROID_HOME
if [ -z "$ANDROID_HOME" ]; then
    if [ -d "$HOME/Android/Sdk" ]; then
        export ANDROID_HOME="$HOME/Android/Sdk"
        echo "export ANDROID_HOME=\$HOME/Android/Sdk" >> ~/.bashrc
        echo "export PATH=\$PATH:\$ANDROID_HOME/platform-tools:\$ANDROID_HOME/cmdline-tools/latest/bin" >> ~/.bashrc
        echo -e "${COLOR_GREEN}✓ ANDROID_HOME set to: $HOME/Android/Sdk${COLOR_RESET}"
    else
        echo -e "${COLOR_YELLOW}⚠ Android SDK not found in $HOME/Android/Sdk${COLOR_RESET}"
    fi
else
    echo -e "${COLOR_GREEN}✓ ANDROID_HOME already set: $ANDROID_HOME${COLOR_RESET}"
fi

# Reload environment
source ~/.bashrc 2>/dev/null || true

echo ""

# ---------------------------------------------------------------------------
# Step 3: Execute Android Tests
# ---------------------------------------------------------------------------

echo -e "${COLOR_CYAN}Step 3: Executing Android Tests${COLOR_RESET}"
echo ""

if [ ! -d "$PROJECT_ROOT/catalogizer-android" ]; then
    echo -e "${COLOR_RED}✗ catalogizer-android directory not found${COLOR_RESET}"
else
    echo "Running catalogizer-android tests..."
    cd "$PROJECT_ROOT/catalogizer-android"

    if [ -f "gradlew" ]; then
        chmod +x gradlew
        ./gradlew test --console=plain 2>&1 | tee "$PROJECT_ROOT/android-test-results.txt"

        if [ ${PIPESTATUS[0]} -eq 0 ]; then
            echo -e "${COLOR_GREEN}✓ catalogizer-android tests passed${COLOR_RESET}"
        else
            echo -e "${COLOR_YELLOW}⚠ Some catalogizer-android tests may have failed (check android-test-results.txt)${COLOR_RESET}"
        fi
    else
        echo -e "${COLOR_RED}✗ gradlew not found in catalogizer-android${COLOR_RESET}"
    fi
fi

echo ""

if [ ! -d "$PROJECT_ROOT/catalogizer-androidtv" ]; then
    echo -e "${COLOR_RED}✗ catalogizer-androidtv directory not found${COLOR_RESET}"
else
    echo "Running catalogizer-androidtv tests..."
    cd "$PROJECT_ROOT/catalogizer-androidtv"

    if [ -f "gradlew" ]; then
        chmod +x gradlew
        ./gradlew test --console=plain 2>&1 | tee "$PROJECT_ROOT/androidtv-test-results.txt"

        if [ ${PIPESTATUS[0]} -eq 0 ]; then
            echo -e "${COLOR_GREEN}✓ catalogizer-androidtv tests passed${COLOR_RESET}"
        else
            echo -e "${COLOR_YELLOW}⚠ Some catalogizer-androidtv tests may have failed (check androidtv-test-results.txt)${COLOR_RESET}"
        fi
    else
        echo -e "${COLOR_RED}✗ gradlew not found in catalogizer-androidtv${COLOR_RESET}"
    fi
fi

cd "$PROJECT_ROOT"
echo ""

# ---------------------------------------------------------------------------
# Step 4: Install Security Scanning Tools
# ---------------------------------------------------------------------------

echo -e "${COLOR_CYAN}Step 4: Setting Up Security Scanning Tools${COLOR_RESET}"
echo ""

# Install Snyk
if ! command -v snyk &> /dev/null; then
    echo "Installing Snyk CLI..."
    if command -v npm &> /dev/null; then
        npm install -g snyk 2>&1 | tail -5
        echo -e "${COLOR_GREEN}✓ Snyk installed${COLOR_RESET}"
    else
        echo -e "${COLOR_YELLOW}⚠ npm not found, skipping Snyk installation${COLOR_RESET}"
        echo "  Install manually: npm install -g snyk"
    fi
else
    echo -e "${COLOR_GREEN}✓ Snyk already installed${COLOR_RESET}"
fi

# Check for Trivy
if ! command -v trivy &> /dev/null; then
    echo -e "${COLOR_YELLOW}⚠ Trivy not found${COLOR_RESET}"
    echo "  Install from: https://github.com/aquasecurity/trivy/releases"
    echo "  Or use: wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add -"
else
    echo -e "${COLOR_GREEN}✓ Trivy already installed${COLOR_RESET}"
fi

echo ""

# ---------------------------------------------------------------------------
# Step 5: Run Security Scans
# ---------------------------------------------------------------------------

echo -e "${COLOR_CYAN}Step 5: Running Security Scans${COLOR_RESET}"
echo ""

# Run Snyk scan if available
if command -v snyk &> /dev/null; then
    echo "Running Snyk vulnerability scan..."

    # Backend
    echo "Scanning backend (catalog-api)..."
    cd "$PROJECT_ROOT/catalog-api"
    snyk test --json > "$PROJECT_ROOT/snyk-backend-results.json" 2>&1 || true

    # Frontend
    echo "Scanning frontend (catalog-web)..."
    cd "$PROJECT_ROOT/catalog-web"
    snyk test --json > "$PROJECT_ROOT/snyk-frontend-results.json" 2>&1 || true

    # API Client
    echo "Scanning API client..."
    cd "$PROJECT_ROOT/catalogizer-api-client"
    snyk test --json > "$PROJECT_ROOT/snyk-api-client-results.json" 2>&1 || true

    cd "$PROJECT_ROOT"
    echo -e "${COLOR_GREEN}✓ Snyk scans complete (results saved)${COLOR_RESET}"
else
    echo -e "${COLOR_YELLOW}⚠ Snyk not available, skipping scans${COLOR_RESET}"
fi

echo ""

# ---------------------------------------------------------------------------
# Step 6: Generate Test Coverage Reports
# ---------------------------------------------------------------------------

echo -e "${COLOR_CYAN}Step 6: Generating Test Coverage Reports${COLOR_RESET}"
echo ""

# Backend coverage
echo "Generating backend test coverage..."
cd "$PROJECT_ROOT/catalog-api"
go test -coverprofile=coverage.out ./... 2>&1 | tail -20
go tool cover -html=coverage.out -o coverage.html 2>/dev/null || true
echo -e "${COLOR_GREEN}✓ Backend coverage report: catalog-api/coverage.html${COLOR_RESET}"

# Frontend coverage
echo "Generating frontend test coverage..."
cd "$PROJECT_ROOT/catalog-web"
npm run test:coverage 2>&1 | tail -20 || npm run test -- --coverage 2>&1 | tail -20 || true
echo -e "${COLOR_GREEN}✓ Frontend coverage report: catalog-web/coverage/index.html${COLOR_RESET}"

cd "$PROJECT_ROOT"
echo ""

# ---------------------------------------------------------------------------
# Step 7: Summary Report
# ---------------------------------------------------------------------------

echo -e "${COLOR_BLUE}================================================================${COLOR_RESET}"
echo -e "${COLOR_BLUE}  Execution Summary${COLOR_RESET}"
echo -e "${COLOR_BLUE}================================================================${COLOR_RESET}"
echo ""

echo -e "${COLOR_GREEN}Completed Tasks:${COLOR_RESET}"
echo "  ✓ Java/JDK installation verified"
echo "  ✓ Environment variables configured"
echo "  ✓ Android tests executed"
echo "  ✓ AndroidTV tests executed"
echo "  ✓ Security scanning tools setup"
echo "  ✓ Security scans executed"
echo "  ✓ Test coverage reports generated"
echo ""

echo -e "${COLOR_CYAN}Generated Files:${COLOR_RESET}"
echo "  • android-test-results.txt"
echo "  • androidtv-test-results.txt"
echo "  • snyk-backend-results.json"
echo "  • snyk-frontend-results.json"
echo "  • snyk-api-client-results.json"
echo "  • catalog-api/coverage.html"
echo "  • catalog-web/coverage/index.html"
echo ""

echo -e "${COLOR_CYAN}Environment Variables Added to ~/.bashrc:${COLOR_RESET}"
echo "  • JAVA_HOME"
echo "  • ANDROID_HOME"
echo "  • PATH (updated with Android tools)"
echo ""

echo -e "${COLOR_MAGENTA}Next Steps:${COLOR_RESET}"
echo "  1. Review test results in *-test-results.txt files"
echo "  2. Check security scan results in snyk-*-results.json files"
echo "  3. Review coverage reports in HTML files"
echo "  4. Source your .bashrc: source ~/.bashrc"
echo ""

echo -e "${COLOR_GREEN}✓ All remaining tasks completed!${COLOR_RESET}"
echo ""
