#!/bin/bash

# Freemium Token Setup Script for Catalogizer Security Testing
# This script helps users set up their freemium accounts and tokens

set -e

echo "🔐 Catalogizer Freemium Security Testing Setup"
echo "=============================================="
echo ""

# Container runtime detection - prefer podman over docker
if command -v podman &>/dev/null; then
    CONTAINER_CMD="podman"
    if command -v podman-compose &>/dev/null; then
        COMPOSE_CMD="podman-compose"
    else
        COMPOSE_CMD=""
    fi
elif command -v docker &>/dev/null; then
    CONTAINER_CMD="docker"
    if command -v docker-compose &>/dev/null; then
        COMPOSE_CMD="docker-compose"
    elif docker compose version &>/dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
    else
        COMPOSE_CMD=""
    fi
else
    CONTAINER_CMD=""
    COMPOSE_CMD=""
fi

# Function to setup SonarQube token
setup_sonarqube() {
    echo "🔍 SonarQube Community Edition (Free)"
    echo "-------------------------------------"
    echo "SonarQube provides code quality analysis with:"
    echo "• Unlimited private projects"
    echo "• All major language support"
    echo "• Quality gates and rules"
    echo ""
    echo "📋 Setup Steps:"
    echo "1. Go to: https://sonarcloud.io"
    echo "2. Sign up for free account"
    echo "3. Create a new organization or use personal account"
    echo "4. Go to Account → Security → Generate Token"
    echo "5. Copy the token and set it:"
    echo ""
    echo "   export SONAR_TOKEN=<your token>"
    echo ""
    read -p "Do you have a SONAR_TOKEN? (y/n): " has_sonar
    if [[ $has_sonar =~ ^[Yy]$ ]]; then
        read -p "Enter your SONAR_TOKEN: " sonar_token
        export SONAR_TOKEN="$sonar_token"
        echo "✅ SONAR_TOKEN set successfully"
    else
        echo "⚠️  Skipping SonarQube setup"
    fi
    echo ""
}

# Function to setup Snyk token
setup_snyk() {
    echo "🔒 Snyk Security Scanning (Free)"
    echo "---------------------------------"
    echo "Snyk provides vulnerability scanning with:"
    echo "• Unlimited private repositories"
    echo "• Unlimited developers"
    echo "• 200 tests/month for public repos"
    echo "• Basic remediation guidance"
    echo ""
    echo "📋 Setup Steps:"
    echo "1. Go to: https://snyk.io"
    echo "2. Sign up for free account"
    echo "3. Verify your email"
    echo "4. Go to Account → General → API Token"
    echo "5. Copy the token and set it:"
    echo ""
    echo "   export SNYK_TOKEN=<your token>"
    echo ""
    read -p "Do you have a SNYK_TOKEN? (y/n): " has_snyk
    if [[ $has_snyk =~ ^[Yy]$ ]]; then
        read -p "Enter your SNYK_TOKEN: " snyk_token
        export SNYK_TOKEN="$snyk_token"
        echo "✅ SNYK_TOKEN set successfully"
    else
        echo "⚠️  Skipping Snyk setup"
    fi
    echo ""
}

# Function to test Docker setup
test_docker() {
    echo "🐳 Testing Container Runtime Setup"
    echo "-----------------------------------"
    if [ -n "$CONTAINER_CMD" ]; then
        echo "✅ Container runtime is installed ($CONTAINER_CMD)"
        if [ -n "$COMPOSE_CMD" ]; then
            echo "✅ Compose tool is installed ($COMPOSE_CMD)"
            echo "🚀 Container setup is ready for security testing"
        else
            echo "❌ No compose tool is installed (docker-compose/podman-compose)"
            echo "📦 Install a compose tool to run full security tests"
        fi
    else
        echo "❌ Neither docker nor podman is installed"
        echo "📦 Install docker or podman to run full security tests"
        echo "💡 You can still run basic tests without a container runtime"
    fi
    echo ""
}

# Function to create .env file
create_env_file() {
    echo "📝 Creating Environment Configuration"
    echo "------------------------------------"
    ENV_FILE=".env.security"
    cat > "$ENV_FILE" << EOF
# Catalogizer Security Testing Environment Variables
# Copy this to your .env file or export these variables

# SonarQube (Free Community Edition)
# SONAR_TOKEN=$SONAR_TOKEN
# SONAR_HOST_URL=http://localhost:9000

# Snyk (Free Tier)
# SNYK_TOKEN=$SNYK_TOKEN
# SNYK_ORG=catalogizer
# SNYK_SEVERITY_THRESHOLD=medium

# Docker Settings (if using Docker)
# COMPOSE_PROJECT_NAME=catalogizer-security
EOF

    echo "✅ Created $ENV_FILE with your configuration"
    echo "💡 Copy the relevant variables to your .env file"
    echo ""
}

# Function to run test scan
run_test_scan() {
    echo "🧪 Running Test Security Scan"
    echo "------------------------------"
    if [ -n "$SNYK_TOKEN" ]; then
        echo "🔍 Testing Snyk connection..."
        if command -v snyk &> /dev/null; then
            if snyk test --help &> /dev/null; then
                echo "✅ Snyk CLI is working"
            else
                echo "⚠️  Snyk CLI needs authentication"
                snyk auth "$SNYK_TOKEN" 2>/dev/null && echo "✅ Snyk authenticated"
            fi
        else
            echo "📦 Installing Snyk CLI..."
            npm install -g snyk 2>/dev/null && echo "✅ Snyk CLI installed"
        fi
    else
        echo "⚠️  Skipping Snyk test (no token)"
    fi

    if [ -n "$SONAR_TOKEN" ]; then
        echo "🔍 Testing SonarQube connection..."
        if curl -f -s "https://sonarcloud.io/api/system/status" &> /dev/null; then
            echo "✅ SonarCloud is accessible"
        else
            echo "⚠️  SonarCloud connection failed"
        fi
    else
        echo "⚠️  Skipping SonarQube test (no token)"
    fi
    echo ""
}

# Main execution
main() {
    echo "Welcome to Catalogizer Freemium Security Testing Setup!"
    echo ""
    echo "This setup uses FREE versions of industry-standard security tools:"
    echo "• SonarQube Community Edition (Code Quality)"
    echo "• Snyk Free Tier (Vulnerability Scanning)"
    echo "• OWASP Dependency Check (Open Source)"
    echo "• Trivy (Open Source Vulnerability Scanner)"
    echo ""

    setup_sonarqube
    setup_snyk
    test_docker
    create_env_file
    run_test_scan

    echo "🎉 Setup Complete!"
    echo "=================="
    echo ""
    echo "Next steps:"
    echo "1. Set your environment variables:"
    echo "   source .env.security"
    echo ""
    echo "2. Run security tests:"
    echo "   ./scripts/security-test.sh"
    echo ""
    echo "3. Or run individual scans:"
    echo "   ./scripts/sonarqube-scan.sh"
    echo "   ./scripts/snyk-scan.sh"
    echo ""
    echo "📖 For more information, see docs/TESTING_GUIDE.md"
    echo ""
    echo "🔒 Happy secure coding!"
}

# Run main function
main "$@"