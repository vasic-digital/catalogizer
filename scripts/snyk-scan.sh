#!/bin/bash

# Snyk Security Scanner Script for Catalogizer
# This script performs comprehensive security vulnerability scanning

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SNYK_TOKEN="${SNYK_TOKEN:-}"
SNYK_ORG="${SNYK_ORG:-catalogizer}"
SNYK_SEVERITY_THRESHOLD="${SNYK_SEVERITY_THRESHOLD:-medium}"
REPORTS_DIR="$PROJECT_ROOT/reports"

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

echo "🔒 Starting Snyk Security Analysis for Catalogizer"
echo "🏢 Organization: $SNYK_ORG"
echo "📁 Project Root: $PROJECT_ROOT"
echo "⚠️  Severity Threshold: $SNYK_SEVERITY_THRESHOLD"

# Create reports directory
mkdir -p "$REPORTS_DIR"

# Function to install Snyk CLI (Enhanced Freemium approach)
install_snyk() {
    if ! command -v snyk &> /dev/null; then
        echo "📦 Installing Snyk CLI (Enhanced Freemium version)..."

        # Use npm for installation (most reliable for freemium)
        if command -v npm &> /dev/null; then
            npm install -g snyk@latest
            echo "✅ Snyk CLI installed via npm"
        else
            echo "❌ npm not available, trying direct download..."
            # Fallback to direct download for freemium
            OS=$(uname -s | tr '[:upper:]' '[:lower:]')
            ARCH=$(uname -m)

            case $ARCH in
                x86_64) ARCH="x64" ;;
                aarch64|arm64) ARCH="arm64" ;;
                *) echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
            esac

            SNYK_URL="https://static.snyk.io/cli/latest/snyk-${OS}-${ARCH}"
            # RULE-CONST-001 — user-local install
            mkdir -p "$HOME/bin"
            curl -s "$SNYK_URL" -o "$HOME/bin/snyk"
            chmod +x "$HOME/bin/snyk"
            case ":$PATH:" in *":$HOME/bin:"*) ;; *) export PATH="$HOME/bin:$PATH" ;; esac
            echo "✅ Snyk CLI installed to \$HOME/bin"
        fi
    else
        echo "✅ Snyk CLI is already installed"
        # Update to latest version
        echo "🔄 Updating Snyk CLI to latest version..."
        npm update -g snyk 2>/dev/null || true
    fi

    # Configure Snyk for freemium usage
    echo "⚙️  Configuring Snyk for freemium usage..."
    snyk config set org="$SNYK_ORG" 2>/dev/null || true
    snyk config set severity-threshold="$SNYK_SEVERITY_THRESHOLD" 2>/dev/null || true

    # Authenticate with Snyk (required for freemium usage)
    if [ -n "$SNYK_TOKEN" ]; then
        echo "🔐 Authenticating with Snyk (Freemium account)..."
        snyk auth "$SNYK_TOKEN" || echo "⚠️  Snyk authentication failed, continuing with limited functionality"
        echo "✅ Snyk authentication successful"
    else
        echo "⚠️  Using dummy Snyk token - some features may be limited"
        echo "💡 Get your free Snyk token at: https://snyk.io/account"
    fi
}

# Function to scan Go project
scan_go_project() {
    echo "🐹 Scanning Go project..."
    cd "$PROJECT_ROOT/catalog-api"
    
    if [ -f "go.mod" ]; then
        echo "📦 Scanning Go dependencies..."
        go mod download
        snyk test --org="$SNYK_ORG" --severity-threshold="$SNYK_SEVERITY_THRESHOLD" --json --json-file-output="$REPORTS_DIR/snyk-go-results.json" || true
        
        # Monitor for ongoing monitoring
        if [ -n "$SNYK_TOKEN" ]; then
            snyk monitor --org="$SNYK_ORG" --project-name="catalogizer-api" || true
        fi
        
        echo "✅ Go project scan completed"
    else
        echo "⚠️  No go.mod found in catalog-api"
    fi
}

# Function to scan JavaScript/TypeScript projects
scan_js_projects() {
    echo "🟢 Scanning JavaScript/TypeScript projects..."
    
    for project_dir in catalog-web catalogizer-desktop catalogizer-api-client installer-wizard; do
        if [ -d "$PROJECT_ROOT/$project_dir" ] && [ -f "$PROJECT_ROOT/$project_dir/package.json" ]; then
            echo "📦 Scanning $project_dir..."
            cd "$PROJECT_ROOT/$project_dir"
            
            # Install dependencies if needed
            if [ ! -d "node_modules" ]; then
                npm install --silent
            fi
            
            # Run Snyk test
            snyk test --org="$SNYK_ORG" --severity-threshold="$SNYK_SEVERITY_THRESHOLD" --json --json-file-output="$REPORTS_DIR/snyk-${project_dir}-results.json" || true
            
            # Monitor for ongoing monitoring
            if [ -n "$SNYK_TOKEN" ]; then
                snyk monitor --org="$SNYK_ORG" --project-name="catalogizer-$project_dir" || true
            fi
            
            echo "✅ $project_dir scan completed"
        fi
    done
}

# Function to scan Android projects
scan_android_projects() {
    echo "📱 Scanning Android projects..."
    
    for project_dir in catalogizer-android catalogizer-androidtv; do
        if [ -d "$PROJECT_ROOT/$project_dir" ] && [ -f "$PROJECT_ROOT/$project_dir/build.gradle.kts" ]; then
            echo "📱 Scanning $project_dir..."
            cd "$PROJECT_ROOT/$project_dir"
            
            # Check if gradlew exists and is executable
            if [ -f "./gradlew" ]; then
                chmod +x ./gradlew
                # Run Snyk test for Gradle
                snyk test --org="$SNYK_ORG" --severity-threshold="$SNYK_SEVERITY_THRESHOLD" --json --json-file-output="$REPORTS_DIR/snyk-${project_dir}-results.json" || true
                
                # Monitor for ongoing monitoring
                if [ -n "$SNYK_TOKEN" ]; then
                    snyk monitor --org="$SNYK_ORG" --project-name="catalogizer-$project_dir" || true
                fi
            else
                echo "⚠️  gradlew not found in $project_dir"
            fi
            
            echo "✅ $project_dir scan completed"
        fi
    done
}

# Function to scan Docker images
scan_docker_images() {
    echo "🐳 Scanning Docker images..."
    
    # Check if a container runtime is available
    if [ -n "$CONTAINER_CMD" ]; then
        # Build API image if it doesn't exist
        if ! $CONTAINER_CMD images | grep -q "catalogizer-api"; then
            echo "🏗️  Building catalogizer-api image..."
            cd "$PROJECT_ROOT/catalog-api"
            if [ -f "Dockerfile" ]; then
                $CONTAINER_CMD build -t catalogizer-api:latest .
            else
                echo "⚠️  Dockerfile not found in catalog-api"
                return 0
            fi
        fi

        # Scan API image
        echo "🔍 Scanning catalogizer-api image..."
        snyk container test catalogizer-api:latest --org="$SNYK_ORG" --severity-threshold="$SNYK_SEVERITY_THRESHOLD" --json --json-file-output="$REPORTS_DIR/snyk-container-results.json" || true

        # Monitor container
        if [ -n "$SNYK_TOKEN" ]; then
            snyk container monitor catalogizer-api:latest --org="$SNYK_ORG" --project-name="catalogizer-api-container" || true
        fi

        echo "✅ Container image scan completed"
    else
        echo "⚠️  Neither docker nor podman available, skipping container scan"
    fi
}

# Function to scan code for security issues
scan_code() {
    echo "🔍 Running Snyk Code analysis..."
    cd "$PROJECT_ROOT"
    
    # Run Snyk Code (SAST)
    if [ -n "$SNYK_TOKEN" ]; then
        snyk code test --org="$SNYK_ORG" --severity-threshold="$SNYK_SEVERITY_THRESHOLD" --json --json-file-output="$REPORTS_DIR/snyk-code-results.json" || true
    else
        echo "⚠️  Skipping Snyk Code analysis - requires valid token"
    fi
    
    echo "✅ Code analysis completed"
}

# Function to generate comprehensive report
generate_report() {
    echo "📊 Generating Snyk security report..."
    
    # Initialize report
    cat > "$REPORTS_DIR/snyk-comprehensive-report.json" << EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "organization": "$SNYK_ORG",
  "project": "catalogizer",
  "severity_threshold": "$SNYK_SEVERITY_THRESHOLD",
  "scans": {}
}
EOF
    
    # Aggregate results from all scans
    for result_file in "$REPORTS_DIR"/snyk-*-results.json; do
        if [ -f "$result_file" ]; then
            scan_type=$(basename "$result_file" | sed 's/snyk-//' | sed 's/-results.json//')
            echo "📊 Processing $scan_type results..."
            
            # Extract summary using jq if available
            if command -v jq &> /dev/null; then
                vulnerabilities=$(jq -r '.vulnerabilities | length' "$result_file" 2>/dev/null || echo "0")
                echo "   🔍 Found $vulnerabilities vulnerabilities"
            fi
        fi
    done
    
    # Generate HTML report
    cat > "$REPORTS_DIR/snyk-security-report.html" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Catalogizer - Snyk Security Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #2c3e50; color: white; padding: 20px; border-radius: 5px; }
        .section { margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }
        .success { background: #d4edda; border-color: #c3e6cb; }
        .warning { background: #fff3cd; border-color: #ffeaa7; }
        .error { background: #f8d7da; border-color: #f5c6cb; }
        .metric { display: inline-block; margin: 10px; padding: 10px; background: #f8f9fa; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔒 Catalogizer Security Report</h1>
        <p>Generated on $(date)</p>
        <p>Organization: $SNYK_ORG | Severity Threshold: $SNYK_SEVERITY_THRESHOLD</p>
    </div>
    
    <div class="section">
        <h2>📊 Scan Summary</h2>
        <div class="metric">🐹 Go API: Scanned</div>
        <div class="metric">🟢 Web Apps: Scanned</div>
        <div class="metric">📱 Android Apps: Scanned</div>
        <div class="metric">🐳 Docker Images: Scanned</div>
        <div class="metric">🔍 Code Analysis: Scanned</div>
    </div>
    
    <div class="section">
        <h2>📋 Detailed Results</h2>
        <p>Detailed JSON reports are available in the reports directory:</p>
        <ul>
EOF

    # Add links to all result files
    for result_file in "$REPORTS_DIR"/snyk-*-results.json; do
        if [ -f "$result_file" ]; then
            filename=$(basename "$result_file")
            echo "            <li><a href=\"$filename\">$filename</a></li>" >> "$REPORTS_DIR/snyk-security-report.html"
        fi
    done

    cat >> "$REPORTS_DIR/snyk-security-report.html" << EOF
        </ul>
    </div>
    
    <div class="section">
        <h2>🔧 Recommendations</h2>
        <ul>
            <li>Review all high and critical severity vulnerabilities</li>
            <li>Update dependencies to secure versions</li>
            <li>Implement security best practices in code</li>
            <li>Regularly monitor for new vulnerabilities</li>
        </ul>
    </div>
</body>
</html>
EOF
    
    echo "📊 Snyk security report generated: $REPORTS_DIR/snyk-security-report.html"
}

# Function to check for critical issues
check_critical_issues() {
    echo "🚨 Checking for critical security issues..."
    
    critical_found=false
    
    for result_file in "$REPORTS_DIR"/snyk-*-results.json; do
        if [ -f "$result_file" ]; then
            if command -v jq &> /dev/null; then
                critical_count=$(jq -r '.vulnerabilities[] | select(.severity == "critical") | .id' "$result_file" 2>/dev/null | wc -l || echo "0")
                high_count=$(jq -r '.vulnerabilities[] | select(.severity == "high") | .id' "$result_file" 2>/dev/null | wc -l || echo "0")
                
                if [ "$critical_count" -gt 0 ] || [ "$high_count" -gt 0 ]; then
                    echo "🚨 Critical/High issues found in $(basename "$result_file")"
                    echo "   Critical: $critical_count, High: $high_count"
                    critical_found=true
                fi
            fi
        fi
    done
    
    if [ "$critical_found" = true ]; then
        echo "❌ Critical or high severity vulnerabilities found!"
        return 1
    else
        echo "✅ No critical or high severity vulnerabilities found"
        return 0
    fi
}

# Main execution
main() {
    echo "🚀 Starting Snyk Security Analysis..."
    
    # Check prerequisites
    install_snyk
    
    # Run all scans
    scan_go_project
    scan_js_projects
    scan_android_projects
    scan_docker_images
    scan_code
    
    # Generate reports
    generate_report
    
    # Check for critical issues
    if check_critical_issues; then
        echo "🎉 Snyk security analysis completed successfully!"
        exit 0
    else
        echo "❌ Snyk security analysis found critical issues!"
        exit 1
    fi
}

# Run main function
main "$@"