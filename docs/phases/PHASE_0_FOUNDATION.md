# PHASE 0: FOUNDATION & INFRASTRUCTURE
## Implementation Guide - Week 1-2

---

## OBJECTIVE

Establish the foundational infrastructure needed for the comprehensive improvement of the Catalogizer project. This phase focuses on security tooling, local CI/CD, test infrastructure, and development environment optimization.

**Success Criteria:**
- All security tools installed and configured
- Local CI/CD pipeline operational
- Test infrastructure ready
- Development environment optimized

---

## WEEK 1: SECURITY INFRASTRUCTURE

### Day 1-2: Security Tool Installation

#### Task 1.1: Install Trivy (Container & Filesystem Scanner)

```bash
# Install Trivy
sudo apt-get install -y wget apt-transport-https gnupg lsb-release
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add -
echo "deb https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main" | sudo tee -a /etc/apt/sources.list.d/trivy.list
sudo apt-get update
sudo apt-get install -y trivy

# Verify installation
trivy --version

# Test scan
trivy filesystem --scanners vuln,secret,config ./catalog-api
trivy image catalogizer-api:latest
```

**Create Configuration:**
```bash
mkdir -p /run/media/milosvasic/DATA4TB/Projects/Catalogizer/config/trivy
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/config/trivy/trivy.yaml << 'EOF'
severity:
  - HIGH
  - CRITICAL

scan:
  scanners:
    - vuln
    - misconfig
    - secret
  skip-dirs:
    - vendor
    - node_modules
    - .git
    - releases
    - reports
    - cache

report:
  formats:
    - table
    - json
    - sarif
  output: reports/security/trivy-report
EOF
```

#### Task 1.2: Install Gosec (Go Security Checker)

```bash
# Install Gosec
cd /tmp
curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b /usr/local/bin

# Verify installation
gosec --version

# Create configuration
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/config/gosec-config.json << 'EOF'
{
  "G101": {
    "pattern": "(?i)passwd|pass|password|pwd|secret|key|token",
    "ignore_entropy": false,
    "entropy_threshold": "80.0",
    "per_char_threshold": "3.0",
    "truncate": "32"
  },
  "severity": "medium",
  "confidence": "medium",
  "cwe": true,
  "nosec": true
}
EOF
```

**Create Gosec Scan Script:**
```bash
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/gosec-scan.sh << 'EOFSCRIPT'
#!/bin/bash
set -e

echo "=== Running Gosec Security Scan ==="

cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer/catalog-api

# Run Gosec with comprehensive rules
gosec -fmt sarif -out ../reports/security/gosec-results.sarif \
  -stdout -verbose \
  -exclude-dir=vendor \
  -exclude-dir=internal/tests \
  -exclude-dir=mocks \
  ./...

# Also generate JSON for processing
gosec -fmt json -out ../reports/security/gosec-results.json \
  -exclude-dir=vendor \
  -exclude-dir=internal/tests \
  -exclude-dir=mocks \
  ./...

echo "Gosec scan complete. Reports saved to reports/security/"
EOFSCRIPT
chmod +x /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/gosec-scan.sh
```

#### Task 1.3: Install Nancy (Go Dependency Vulnerability Scanner)

```bash
# Install Nancy
cd /tmp
curl -L -o nancy https://github.com/sonatype-nexus-community/nancy/releases/latest/download/nancy-linux.amd64
chmod +x nancy
sudo mv nancy /usr/local/bin/

# Verify installation
nancy --version

# Create scan script
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/nancy-scan.sh << 'EOFSCRIPT'
#!/bin/bash
set -e

echo "=== Running Nancy Dependency Scan ==="

cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer/catalog-api

# Generate dependency list and scan
go list -json -m all | nancy sleuth \
  --output json > ../reports/security/nancy-results.json 2>&1 || true

echo "Nancy scan complete. Report saved to reports/security/"
EOFSCRIPT
chmod +x /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/nancy-scan.sh
```

#### Task 1.4: Install Syft (SBOM Generator)

```bash
# Install Syft
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sudo sh -s -- -b /usr/local/bin

# Verify installation
syft --version

# Create SBOM generation script
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/generate-sbom.sh << 'EOFSCRIPT'
#!/bin/bash
set -e

echo "=== Generating Software Bill of Materials (SBOM) ==="

mkdir -p reports/sbom

# Generate SBOM for Go backend
cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer/catalog-api
syft packages dir:. -o spdx-json=../reports/sbom/catalog-api-sbom.json
syft packages dir:. -o cyclonedx-json=../reports/sbom/catalog-api-cyclonedx.json

# Generate SBOM for Web frontend
cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer/catalog-web
syft packages dir:. -o spdx-json=../reports/sbom/catalog-web-sbom.json
syft packages dir:. -o cyclonedx-json=../reports/sbom/catalog-web-cyclonedx.json

# Generate SBOM for Desktop app
cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer/catalogizer-desktop
syft packages dir:. -o spdx-json=../reports/sbom/catalogizer-desktop-sbom.json

# Generate SBOM for API Client
cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer/catalogizer-api-client
syft packages dir:. -o spdx-json=../reports/sbom/catalogizer-api-client-sbom.json

echo "SBOM generation complete. Reports saved to reports/sbom/"
EOFSCRIPT
chmod +x /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/generate-sbom.sh
```

### Day 3-4: Pre-commit Hooks Setup

#### Task 2.1: Install Pre-commit Framework

```bash
# Install pre-commit
pip install pre-commit

# Verify installation
pre-commit --version
```

#### Task 2.2: Create Pre-commit Configuration

```yaml
# File: /run/media/milosvasic/DATA4TB/Projects/Catalogizer/.pre-commit-config.yaml
repos:
  # General hooks
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-json
      - id: check-added-large-files
        args: ['--maxkb=1000']
      - id: check-merge-conflict
      - id: detect-private-key
      - id: detect-aws-credentials

  # Secret detection
  - repo: https://github.com/Yelp/detect-secrets
    rev: v1.4.0
    hooks:
      - id: detect-secrets
        args: ['--baseline', '.secrets.baseline']

  # Go hooks
  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-fmt
      - id: go-vet
      - id: go-imports
      - id: golangci-lint
        args: ['--fix']

  # Security scanning
  - repo: local
    hooks:
      - id: gosec-scan
        name: Gosec Security Scan
        entry: scripts/gosec-scan.sh
        language: script
        files: '\.go$'
        pass_filenames: false
        
      - id: trivy-fs-scan
        name: Trivy Filesystem Scan
        entry: trivy filesystem --scanners vuln,secret --exit-code 0
        language: system
        pass_filenames: false
        always_run: true

  # TypeScript/Node hooks
  - repo: https://github.com/pre-commit/mirrors-eslint
    rev: v8.55.0
    hooks:
      - id: eslint
        files: '\.(ts|tsx)$'
        additional_dependencies:
          - eslint@8.55.0
          - typescript-eslint@6.15.0

  - repo: https://github.com/pre-commit/mirrors-prettier
    rev: v3.1.0
    hooks:
      - id: prettier
        files: '\.(ts|tsx|js|jsx|json|yaml|yml|md)$'
```

#### Task 2.3: Initialize Secret Detection Baseline

```bash
cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer

# Create baseline
detect-secrets scan --all-files --force-use-all-plugins > .secrets.baseline

# Review and audit
# Manually mark false positives in the baseline file
```

#### Task 2.4: Install Pre-commit Hooks

```bash
cd /run/media/milosvasic/DATA4TB/Projects/Catalogizer

# Install hooks
pre-commit install

# Run on all files (initial check)
pre-commit run --all-files
```

### Day 5-7: Security Automation Scripts

#### Task 3.1: Create Master Security Scan Script

```bash
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/security-scan-full.sh << 'EOFSCRIPT'
#!/bin/bash
set -e

echo "==================================================================="
echo "CATALOGIZER COMPREHENSIVE SECURITY SCAN"
echo "==================================================================="
echo ""

# Create reports directory
mkdir -p reports/security
mkdir -p reports/sbom

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="reports/security/${TIMESTAMP}"
mkdir -p "${REPORT_DIR}"

# Track results
TOTAL_ISSUES=0
HIGH_SEVERITY=0
CRITICAL_SEVERITY=0

echo "[1/8] Running Snyk Dependency Scan..."
cd catalog-api
snyk test --json > "../${REPORT_DIR}/snyk-go.json" 2>/dev/null || true
SNYK_GO_ISSUES=$(jq '.vulnerabilities | length' "../${REPORT_DIR}/snyk-go.json" 2>/dev/null || echo "0")
echo "      Found ${SNYK_GO_ISSUES} issues in Go dependencies"
cd ..

cd catalog-web
snyk test --json > "../${REPORT_DIR}/snyk-web.json" 2>/dev/null || true
SNYK_WEB_ISSUES=$(jq '.vulnerabilities | length' "../${REPORT_DIR}/snyk-web.json" 2>/dev/null || echo "0")
echo "      Found ${SNYK_WEB_ISSUES} issues in Web dependencies"
cd ..

echo ""
echo "[2/8] Running Gosec Security Scan..."
cd catalog-api
gosec -fmt json -out "../${REPORT_DIR}/gosec.json" \
  -exclude-dir=vendor -exclude-dir=internal/tests -exclude-dir=mocks \
  ./... 2>/dev/null || true
GOSEC_ISSUES=$(jq '.Issues | length' "../${REPORT_DIR}/gosec.json" 2>/dev/null || echo "0")
echo "      Found ${GOSEC_ISSUES} security issues"
cd ..

echo ""
echo "[3/8] Running Nancy Dependency Scan..."
cd catalog-api
go list -json -m all | nancy sleuth --output json > "../${REPORT_DIR}/nancy.json" 2>/dev/null || true
NANCY_ISSUES=$(jq '.vulnerable | length' "../${REPORT_DIR}/nancy.json" 2>/dev/null || echo "0")
echo "      Found ${NANCY_ISSUES} vulnerable dependencies"
cd ..

echo ""
echo "[4/8] Running govulncheck..."
cd catalog-api
govulncheck -json ./... > "../${REPORT_DIR}/govulncheck.json" 2>/dev/null || true
VULNCHECK_ISSUES=$(jq '.Vulns | length' "../${REPORT_DIR}/govulncheck.json" 2>/dev/null || echo "0")
echo "      Found ${VULNCHECK_ISSUES} vulnerabilities"
cd ..

echo ""
echo "[5/8] Running Trivy Filesystem Scan..."
trivy filesystem --scanners vuln,secret,config \
  --format json --output "${REPORT_DIR}/trivy-fs.json" \
  . 2>/dev/null || true
TRIVY_FS_ISSUES=$(jq '.Results[0].Vulnerabilities | length' "${REPORT_DIR}/trivy-fs.json" 2>/dev/null || echo "0")
echo "      Found ${TRIVY_FS_ISSUES} filesystem issues"

echo ""
echo "[6/8] Running Container Image Scan..."
# Build images first if needed
podman build -t catalogizer-api:test ./catalog-api 2>/dev/null || true
trivy image --format json --output "${REPORT_DIR}/trivy-image-api.json" \
  catalogizer-api:test 2>/dev/null || true
TRIVY_IMAGE_ISSUES=$(jq '.Results[0].Vulnerabilities | length' "${REPORT_DIR}/trivy-image-api.json" 2>/dev/null || echo "0")
echo "      Found ${TRIVY_IMAGE_ISSUES} image vulnerabilities"

echo ""
echo "[7/8] Running npm audit..."
cd catalog-web
npm audit --json > "../${REPORT_DIR}/npm-audit.json" 2>/dev/null || true
NPM_ISSUES=$(jq '.metadata.vulnerabilities.total' "../${REPORT_DIR}/npm-audit.json" 2>/dev/null || echo "0")
echo "      Found ${NPM_ISSUES} npm vulnerabilities"
cd ..

echo ""
echo "[8/8] Generating SBOM..."
./scripts/generate-sbom.sh > "${REPORT_DIR}/sbom-generation.log" 2>&1 || true
echo "      SBOM generated"

echo ""
echo "==================================================================="
echo "SECURITY SCAN SUMMARY"
echo "==================================================================="
echo "Snyk (Go):        ${SNYK_GO_ISSUES} issues"
echo "Snyk (Web):       ${SNYK_WEB_ISSUES} issues"
echo "Gosec:            ${GOSEC_ISSUES} issues"
echo "Nancy:            ${NANCY_ISSUES} issues"
echo "govulncheck:      ${VULNCHECK_ISSUES} issues"
echo "Trivy (FS):       ${TRIVY_FS_ISSUES} issues"
echo "Trivy (Image):    ${TRIVY_IMAGE_ISSUES} issues"
echo "npm audit:        ${NPM_ISSUES} issues"
echo ""
TOTAL_ISSUES=$((SNYK_GO_ISSUES + SNYK_WEB_ISSUES + GOSEC_ISSUES + NANCY_ISSUES + VULNCHECK_ISSUES + TRIVY_FS_ISSUES + TRIVY_IMAGE_ISSUES + NPM_ISSUES))
echo "TOTAL:            ${TOTAL_ISSUES} issues found"
echo ""
echo "Detailed reports saved to: ${REPORT_DIR}"
echo "==================================================================="

# Generate HTML summary report
cat > "${REPORT_DIR}/summary.html" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Security Scan Summary - ${TIMESTAMP}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        table { border-collapse: collapse; width: 100%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #4CAF50; color: white; }
        tr:nth-child(even) { background-color: #f2f2f2; }
        .critical { color: #d32f2f; font-weight: bold; }
        .high { color: #f57c00; font-weight: bold; }
        .summary { margin-top: 30px; padding: 20px; background-color: #f5f5f5; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>Security Scan Summary</h1>
    <p>Timestamp: ${TIMESTAMP}</p>
    
    <table>
        <tr>
            <th>Scanner</th>
            <th>Issues Found</th>
            <th>Report File</th>
        </tr>
        <tr>
            <td>Snyk (Go)</td>
            <td>${SNYK_GO_ISSUES}</td>
            <td><a href="snyk-go.json">snyk-go.json</a></td>
        </tr>
        <tr>
            <td>Snyk (Web)</td>
            <td>${SNYK_WEB_ISSUES}</td>
            <td><a href="snyk-web.json">snyk-web.json</a></td>
        </tr>
        <tr>
            <td>Gosec</td>
            <td>${GOSEC_ISSUES}</td>
            <td><a href="gosec.json">gosec.json</a></td>
        </tr>
        <tr>
            <td>Nancy</td>
            <td>${NANCY_ISSUES}</td>
            <td><a href="nancy.json">nancy.json</a></td>
        </tr>
        <tr>
            <td>govulncheck</td>
            <td>${VULNCHECK_ISSUES}</td>
            <td><a href="govulncheck.json">govulncheck.json</a></td>
        </tr>
        <tr>
            <td>Trivy (Filesystem)</td>
            <td>${TRIVY_FS_ISSUES}</td>
            <td><a href="trivy-fs.json">trivy-fs.json</a></td>
        </tr>
        <tr>
            <td>Trivy (Container)</td>
            <td>${TRIVY_IMAGE_ISSUES}</td>
            <td><a href="trivy-image-api.json">trivy-image-api.json</a></td>
        </tr>
        <tr>
            <td>npm audit</td>
            <td>${NPM_ISSUES}</td>
            <td><a href="npm-audit.json">npm-audit.json</a></td>
        </tr>
    </table>
    
    <div class="summary">
        <h2>Total Issues: ${TOTAL_ISSUES}</h2>
        <p>All scan reports are available in JSON format for detailed analysis.</p>
    </div>
</body>
</html>
EOF

echo "HTML summary report generated: ${REPORT_DIR}/summary.html"
EOFSCRIPT
chmod +x /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/security-scan-full.sh
```

#### Task 3.2: Create Security Gates Script

```bash
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/security-gates.sh << 'EOFSCRIPT'
#!/bin/bash

# Security gates - fail build if security thresholds exceeded

MAX_CRITICAL=0
MAX_HIGH=10
MAX_MEDIUM=50

echo "=== Security Gates ==="
echo "Max Critical Issues: ${MAX_CRITICAL}"
echo "Max High Issues: ${MAX_HIGH}"
echo "Max Medium Issues: ${MAX_MEDIUM}"
echo ""

# Check for critical vulnerabilities
CRITICAL_COUNT=0
HIGH_COUNT=0
MEDIUM_COUNT=0

# Parse latest scan results
LATEST_SCAN=$(ls -td reports/security/*/ 2>/dev/null | head -1)

if [ -z "${LATEST_SCAN}" ]; then
    echo "ERROR: No security scan results found"
    exit 1
fi

echo "Checking scan results in: ${LATEST_SCAN}"

# Count critical vulnerabilities from all reports
if [ -f "${LATEST_SCAN}/snyk-go.json" ]; then
    CRITICAL_COUNT=$(($(jq '[.vulnerabilities[] | select(.severity=="critical")] | length' "${LATEST_SCAN}/snyk-go.json" 2>/dev/null || echo 0) + CRITICAL_COUNT))
fi

if [ -f "${LATEST_SCAN}/trivy-fs.json" ]; then
    CRITICAL_COUNT=$(($(jq '[.Results[].Vulnerabilities[] | select(.Severity=="CRITICAL")] | length' "${LATEST_SCAN}/trivy-fs.json" 2>/dev/null || echo 0) + CRITICAL_COUNT))
fi

echo ""
echo "Results:"
echo "  Critical Issues: ${CRITICAL_COUNT} (max: ${MAX_CRITICAL})"
echo "  High Issues: ${HIGH_COUNT} (max: ${MAX_HIGH})"
echo "  Medium Issues: ${MEDIUM_COUNT} (max: ${MAX_MEDIUM})"
echo ""

# Check gates
FAILED=0

if [ ${CRITICAL_COUNT} -gt ${MAX_CRITICAL} ]; then
    echo "FAIL: Critical vulnerabilities exceed threshold"
    FAILED=1
fi

if [ ${HIGH_COUNT} -gt ${MAX_HIGH} ]; then
    echo "FAIL: High vulnerabilities exceed threshold"
    FAILED=1
fi

if [ ${FAILED} -eq 1 ]; then
    echo ""
    echo "SECURITY GATES FAILED - Build blocked"
    exit 1
else
    echo "SECURITY GATES PASSED"
    exit 0
fi
EOFSCRIPT
chmod +x /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/security-gates.sh
```

---

## WEEK 2: LOCAL CI/CD & TEST INFRASTRUCTURE

### Day 8-10: Local CI/CD Pipeline

#### Task 4.1: Install Drone CI (Lightweight Option)

```bash
# Install Drone CLI
curl -L https://github.com/harness/drone-cli/releases/latest/download/drone_linux_amd64.tar.gz | tar zx
sudo install -t /usr/local/bin drone

# Create Drone configuration directory
mkdir -p /run/media/milosvasic/DATA4TB/Projects/Catalogizer/.drone
```

#### Task 4.2: Create Drone CI Configuration

```yaml
# File: /run/media/milosvasic/DATA4TB/Projects/Catalogizer/.drone.yml
kind: pipeline
type: exec
name: default

platform:
  os: linux
  arch: amd64

steps:
  # ==========================================
  # PHASE 1: Setup & Dependencies
  # ==========================================
  - name: setup
    commands:
      - echo "Setting up environment..."
      - mkdir -p reports/{security,coverage,tests}
      - git submodule update --init --recursive

  # ==========================================
  # PHASE 2: Go Backend Validation
  # ==========================================
  - name: go-fmt
    commands:
      - cd catalog-api
      - test -z "$(gofmt -l .)"

  - name: go-vet
    commands:
      - cd catalog-api
      - go vet ./...

  - name: go-test
    commands:
      - cd catalog-api
      - GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -v -coverprofile=../reports/coverage/go-coverage.out

  - name: go-coverage
    commands:
      - cd catalog-api
      - go tool cover -html=../reports/coverage/go-coverage.out -o ../reports/coverage/go-coverage.html
      - go tool cover -func=../reports/coverage/go-coverage.out | grep total | awk '{print "Total coverage: " $3}'

  # ==========================================
  # PHASE 3: TypeScript Frontend Validation
  # ==========================================
  - name: web-lint
    commands:
      - cd catalog-web
      - npm ci
      - npm run lint

  - name: web-typecheck
    commands:
      - cd catalog-web
      - npm run type-check

  - name: web-test
    commands:
      - cd catalog-web
      - npm run test:coverage

  # ==========================================
  # PHASE 4: Security Scanning
  # ==========================================
  - name: security-scan
    commands:
      - ./scripts/security-scan-full.sh
    failure: ignore  # Don't fail build, just report

  - name: security-gates
    commands:
      - ./scripts/security-gates.sh

  # ==========================================
  # PHASE 5: Integration Tests
  # ==========================================
  - name: integration-tests
    commands:
      - scripts/test-infra/up.sh --build-id ci   # waits until every service answers its protocol
      - sleep 0
      - cd catalog-api
      - go test -v ./tests/integration/... -tags=integration
      - scripts/test-infra/down.sh --build-id ci --op-id "$(sed -n 's/^TI_OP_ID=//p' .audit/test-infra/catalogizer-test-ci/env)"
    failure: ignore

  # ==========================================
  # PHASE 6: Build Validation
  # ==========================================
  - name: build-api
    commands:
      - cd catalog-api
      - go build -o ../build/catalog-api

  - name: build-web
    commands:
      - cd catalog-web
      - npm run build

trigger:
  branch:
    - main
    - develop
  event:
    - push
    - pull_request
```

#### Task 4.3: Create Local CI Runner Script

```bash
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/local-ci.sh << 'EOFSCRIPT'
#!/bin/bash

# Local CI/CD runner - runs all checks locally

set -e

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="reports/ci/${TIMESTAMP}"
mkdir -p "${REPORT_DIR}"

echo "==================================================================="
echo "CATALOGIZER LOCAL CI/CD PIPELINE"
echo "Timestamp: ${TIMESTAMP}"
echo "==================================================================="
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track results
TOTAL_STEPS=0
PASSED_STEPS=0
FAILED_STEPS=0
SKIPPED_STEPS=0

run_step() {
    local name=$1
    local command=$2
    local optional=$3
    
    TOTAL_STEPS=$((TOTAL_STEPS + 1))
    
    echo ""
    echo "[${TOTAL_STEPS}] Running: ${name}"
    echo "Command: ${command}"
    echo "----------------------------------------"
    
    if eval "${command}" > "${REPORT_DIR}/${name}.log" 2>&1; then
        echo -e "${GREEN}✓ PASSED${NC}: ${name}"
        PASSED_STEPS=$((PASSED_STEPS + 1))
        return 0
    else
        if [ "${optional}" == "optional" ]; then
            echo -e "${YELLOW}⚠ SKIPPED${NC}: ${name} (optional)"
            SKIPPED_STEPS=$((SKIPPED_STEPS + 1))
        else
            echo -e "${RED}✗ FAILED${NC}: ${name}"
            echo "See: ${REPORT_DIR}/${name}.log"
            FAILED_STEPS=$((FAILED_STEPS + 1))
        fi
        return 1
    fi
}

# ==========================================
# PHASE 1: Setup & Dependencies
# ==========================================
echo ""
echo "PHASE 1: Setup & Dependencies"
echo "==================================================================="

run_step "submodule-init" "git submodule update --init --recursive"

# ==========================================
# PHASE 2: Go Backend Validation
# ==========================================
echo ""
echo "PHASE 2: Go Backend Validation"
echo "==================================================================="

run_step "go-fmt" "cd catalog-api && test -z '\$(gofmt -l .)'"
run_step "go-vet" "cd catalog-api && go vet ./..."
run_step "go-test-unit" "cd catalog-api && GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -v" optional
run_step "go-coverage" "cd catalog-api && go test -coverprofile=${REPORT_DIR}/go-coverage.out ./... && go tool cover -func=${REPORT_DIR}/go-coverage.out | grep total"

# ==========================================
# PHASE 3: TypeScript Frontend Validation
# ==========================================
echo ""
echo "PHASE 3: TypeScript Frontend Validation"
echo "==================================================================="

run_step "web-install" "cd catalog-web && npm ci"
run_step "web-lint" "cd catalog-web && npm run lint"
run_step "web-typecheck" "cd catalog-web && npm run type-check"
run_step "web-test" "cd catalog-web && npm run test" optional

# ==========================================
# PHASE 4: Security Scanning
# ==========================================
echo ""
echo "PHASE 4: Security Scanning"
echo "==================================================================="

run_step "security-scan" "./scripts/security-scan-full.sh" optional
run_step "security-gates" "./scripts/security-gates.sh" optional

# ==========================================
# PHASE 5: Build Validation
# ==========================================
echo ""
echo "PHASE 5: Build Validation"
echo "==================================================================="

run_step "build-api" "cd catalog-api && go build -o ${REPORT_DIR}/catalog-api"
run_step "build-web" "cd catalog-web && npm run build"

# ==========================================
# PHASE 6: Pre-commit Hooks
# ==========================================
echo ""
echo "PHASE 6: Pre-commit Hooks"
echo "==================================================================="

run_step "pre-commit" "pre-commit run --all-files" optional

# ==========================================
# Summary Report
# ==========================================
echo ""
echo "==================================================================="
echo "CI/CD PIPELINE SUMMARY"
echo "==================================================================="
echo "Total Steps:    ${TOTAL_STEPS}"
echo -e "${GREEN}Passed:         ${PASSED_STEPS}${NC}"
echo -e "${RED}Failed:         ${FAILED_STEPS}${NC}"
echo -e "${YELLOW}Skipped:        ${SKIPPED_STEPS}${NC}"
echo ""

# Generate summary JSON
cat > "${REPORT_DIR}/summary.json" << EOF
{
  "timestamp": "${TIMESTAMP}",
  "total_steps": ${TOTAL_STEPS},
  "passed": ${PASSED_STEPS},
  "failed": ${FAILED_STEPS},
  "skipped": ${SKIPPED_STEPS},
  "success_rate": $(echo "scale=2; ${PASSED_STEPS} * 100 / ${TOTAL_STEPS}" | bc -l 2>/dev/null || echo "0")
}
EOF

echo "Report Directory: ${REPORT_DIR}"
echo "==================================================================="

if [ ${FAILED_STEPS} -gt 0 ]; then
    echo -e "${RED}PIPELINE FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}PIPELINE PASSED${NC}"
    exit 0
fi
EOFSCRIPT
chmod +x /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/local-ci.sh
```

### Day 11-14: Test Infrastructure Setup

#### Task 5.1: Create Test Environment Provisioning

```bash
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/setup-test-env.sh << 'EOFSCRIPT'
#!/bin/bash

# Setup test environment with all required infrastructure

set -e

echo "=== Setting up Test Environment ==="

# Start test infrastructure
scripts/test-infra/up.sh --build-id setup

# Wait for services to be ready
echo "Waiting for test services to be ready..."
sleep 15

# Verify services
echo ""
echo "Checking service availability:"

# Check SMB
if timeout 5 bash -c "</dev/tcp/localhost/1445" 2>/dev/null; then
    echo "  ✓ SMB server: localhost:1445"
else
    echo "  ✗ SMB server: localhost:1445 (not ready)"
fi

# Check FTP
if timeout 5 bash -c "</dev/tcp/localhost/2121" 2>/dev/null; then
    echo "  ✓ FTP server: localhost:2121"
else
    echo "  ✗ FTP server: localhost:2121 (not ready)"
fi

# Check WebDAV
if curl -s http://localhost:8081 > /dev/null 2>&1; then
    echo "  ✓ WebDAV server: localhost:8081"
else
    echo "  ✗ WebDAV server: localhost:8081 (not ready)"
fi

# Check NFS
if timeout 5 bash -c "</dev/tcp/localhost/2049" 2>/dev/null; then
    echo "  ✓ NFS server: localhost:2049"
else
    echo "  ✗ NFS server: localhost:2049 (not ready)"
fi

echo ""
echo "Test environment setup complete!"
echo ""
echo "To stop test environment:"
echo "  scripts/test-infra/down.sh --build-id setup --op-id <op_id up.sh printed>"
EOFSCRIPT
chmod +x /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/setup-test-env.sh
```

#### Task 5.2: Create Test Data Fixtures

```go
// File: catalog-api/internal/tests/fixtures/fixtures.go
package fixtures

import (
	"catalog-api/database"
	"catalog-api/models"
	"time"
)

// TestFixtures provides reusable test data
type TestFixtures struct {
	DB *database.DB
}

// NewTestFixtures creates a new fixtures instance
func NewTestFixtures(db *database.DB) *TestFixtures {
	return &TestFixtures{DB: db}
}

// CreateTestUser creates a test user
func (f *TestFixtures) CreateTestUser(username, password string) (*models.User, error) {
	user := &models.User{
		Username:     username,
		PasswordHash: password, // Should be hashed in real code
		Email:        username + "@test.com",
		Role:         "user",
		CreatedAt:    time.Now(),
	}
	
	query := `INSERT INTO users (username, password_hash, email, role, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`
	var id int64
	err := f.DB.QueryRow(query, user.Username, user.PasswordHash, user.Email, user.Role, user.CreatedAt).Scan(&id)
	if err != nil {
		return nil, err
	}
	
	user.ID = int(id)
	return user, nil
}

// CreateTestStorageRoot creates a test storage root
func (f *TestFixtures) CreateTestStorageRoot(name, path string, userID int) (*models.StorageRoot, error) {
	root := &models.StorageRoot{
		Name:      name,
		Path:      path,
		Protocol:  "local",
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	
	query := `INSERT INTO storage_roots (name, path, protocol, user_id, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`
	var id int64
	err := f.DB.QueryRow(query, root.Name, root.Path, root.Protocol, root.UserID, root.CreatedAt).Scan(&id)
	if err != nil {
		return nil, err
	}
	
	root.ID = int(id)
	return root, nil
}

// CreateTestMediaItem creates a test media item
func (f *TestFixtures) CreateTestMediaItem(title string, mediaType string) (*models.MediaItem, error) {
	item := &models.MediaItem{
		Title:       title,
		Type:        mediaType,
		ReleaseYear: 2024,
		CreatedAt:   time.Now(),
	}
	
	query := `INSERT INTO media_items (title, type, release_year, created_at) VALUES (?, ?, ?, ?) RETURNING id`
	var id int64
	err := f.DB.QueryRow(query, item.Title, item.Type, item.ReleaseYear, item.CreatedAt).Scan(&id)
	if err != nil {
		return nil, err
	}
	
	item.ID = id
	return item, nil
}

// Cleanup removes all test data
func (f *TestFixtures) Cleanup() error {
	// Delete in correct order to respect foreign keys
	tables := []string{
		"media_files",
		"media_collection_items",
		"favorites",
		"files",
		"media_items",
		"media_collections",
		"storage_roots",
		"users",
	}
	
	for _, table := range tables {
		if _, err := f.DB.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}
	
	return nil
}
```

#### Task 5.3: Create Test Coverage Tracking

```bash
cat > /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/track-coverage.sh << 'EOFSCRIPT'
#!/bin/bash

# Track test coverage over time

set -e

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
COVERAGE_DIR="reports/coverage"
mkdir -p "${COVERAGE_DIR}"

echo "=== Tracking Test Coverage ==="
echo "Timestamp: ${TIMESTAMP}"
echo ""

# Go coverage
echo "[1/4] Collecting Go coverage..."
cd catalog-api
GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -coverprofile=../${COVERAGE_DIR}/go-coverage-${TIMESTAMP}.out
go tool cover -func=../${COVERAGE_DIR}/go-coverage-${TIMESTAMP}.out | grep total > ../${COVERAGE_DIR}/go-total-${TIMESTAMP}.txt
cd ..

GO_COVERAGE=$(cat ${COVERAGE_DIR}/go-total-${TIMESTAMP}.txt | awk '{print $3}' | sed 's/%//')
echo "      Go Coverage: ${GO_COVERAGE}%"

# Web coverage
echo "[2/4] Collecting Web coverage..."
cd catalog-web
npm run test:coverage 2>/dev/null || true
if [ -f coverage/coverage-summary.json ]; then
    WEB_COVERAGE=$(jq '.total.lines.pct' coverage/coverage-summary.json 2>/dev/null || echo "0")
    cp coverage/coverage-summary.json ../${COVERAGE_DIR}/web-coverage-${TIMESTAMP}.json
else
    WEB_COVERAGE="0"
fi
cd ..
echo "      Web Coverage: ${WEB_COVERAGE}%"

# Desktop coverage
echo "[3/4] Collecting Desktop coverage..."
cd catalogizer-desktop
npm run test:coverage 2>/dev/null || true
if [ -f coverage/coverage-summary.json ]; then
    DESKTOP_COVERAGE=$(jq '.total.lines.pct' coverage/coverage-summary.json 2>/dev/null || echo "0")
    cp coverage/coverage-summary.json ../${COVERAGE_DIR}/desktop-coverage-${TIMESTAMP}.json
else
    DESKTOP_COVERAGE="0"
fi
cd ..
echo "      Desktop Coverage: ${DESKTOP_COVERAGE}%"

# API Client coverage
echo "[4/4] Collecting API Client coverage..."
cd catalogizer-api-client
npm run test:coverage 2>/dev/null || true
if [ -f coverage/coverage-summary.json ]; then
    CLIENT_COVERAGE=$(jq '.total.lines.pct' coverage/coverage-summary.json 2>/dev/null || echo "0")
    cp coverage/coverage-summary.json ../${COVERAGE_DIR}/client-coverage-${TIMESTAMP}.json
else
    CLIENT_COVERAGE="0"
fi
cd ..
echo "      API Client Coverage: ${CLIENT_COVERAGE}%"

# Generate coverage report
echo ""
echo "=== Coverage Summary ==="
echo "Go Backend:      ${GO_COVERAGE}%"
echo "Web Frontend:    ${WEB_COVERAGE}%"
echo "Desktop App:     ${DESKTOP_COVERAGE}%"
echo "API Client:      ${CLIENT_COVERAGE}%"
echo ""

# Save to history
echo "${TIMESTAMP},${GO_COVERAGE},${WEB_COVERAGE},${DESKTOP_COVERAGE},${CLIENT_COVERAGE}" >> ${COVERAGE_DIR}/coverage-history.csv

# Generate trend chart (if gnuplot available)
if command -v gnuplot > /dev/null 2>&1; then
    cat > /tmp/coverage-plot.gnuplot << 'EOF'
set terminal png size 1200,600
set output 'reports/coverage/trend.png'
set datafile separator ","
set xlabel "Build"
set ylabel "Coverage %"
set title "Test Coverage Trend"
set grid
plot 'reports/coverage/coverage-history.csv' using 0:2 with lines title 'Go Backend', \
     '' using 0:3 with lines title 'Web Frontend', \
     '' using 0:4 with lines title 'Desktop App'
EOF
    gnuplot /tmp/coverage-plot.gnuplot 2>/dev/null || true
    echo "Trend chart generated: reports/coverage/trend.png"
fi

echo ""
echo "Coverage tracking complete!"
echo "History file: ${COVERAGE_DIR}/coverage-history.csv"
EOFSCRIPT
chmod +x /run/media/milosvasic/DATA4TB/Projects/Catalogizer/scripts/track-coverage.sh
```

---

## DELIVERABLES CHECKLIST

### Week 1 Deliverables:
- [ ] Trivy installed and configured
- [ ] Gosec installed and configured
- [ ] Nancy installed and configured
- [ ] Syft installed and configured
- [ ] Pre-commit hooks configured
- [ ] Secret detection baseline created
- [ ] Security scan scripts created
- [ ] Security gates implemented

### Week 2 Deliverables:
- [ ] Local CI/CD pipeline configured
- [ ] Drone CI configuration created
- [ ] Local CI runner script created
- [ ] Test environment provisioning scripts
- [ ] Test data fixtures created
- [ ] Coverage tracking system implemented
- [ ] All scripts tested and working

---

## VALIDATION

Run these commands to validate Phase 0 completion:

```bash
# 1. Verify security tools
trivy --version
gosec --version
nancy --version
syft --version
pre-commit --version

# 2. Test security scan
./scripts/security-scan-full.sh

# 3. Test local CI
./scripts/local-ci.sh

# 4. Test coverage tracking
./scripts/track-coverage.sh

# 5. Verify pre-commit hooks
pre-commit run --all-files
```

All commands should complete successfully with no errors.

---

**Phase 0 Complete: Foundation ready for comprehensive improvements**
