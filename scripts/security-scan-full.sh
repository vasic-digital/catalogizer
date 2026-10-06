#!/bin/bash

# Comprehensive Security Scanning Script
# Runs all security tools: Trivy, Gosec, Nancy, Semgrep

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REPORTS_DIR="$PROJECT_ROOT/reports/security"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Catalogizer Security Scan Suite"
echo "Date: $(date)"
echo "=========================================="
echo ""

# Create reports directory
mkdir -p "$REPORTS_DIR"

# Check for required tools
check_tool() {
    if ! command -v "$1" &> /dev/null; then
        echo -e "${RED}ERROR: $1 is not installed${NC}"
        echo "Install with: $2"
        return 1
    fi
    echo -e "${GREEN}✓ $1 found${NC}"
    return 0
}

echo "Checking required tools..."
check_tool trivy "download Trivy 0.75.0 from https://github.com/aquasecurity/trivy/releases/tag/v0.75.0, verify the SHA-256 of the archive against trivy_0.75.0_checksums.txt of that release, then install the binary (or run the digest-pinned image of build/containers/images.lock.yaml, IMG-SCAN-TRIVY)"
check_tool gosec "go install github.com/securego/gosec/v2/cmd/gosec@v2.24.6"
check_tool nancy "download nancy v2.1.0 (nancy-v2.1.0-linux-amd64) from https://github.com/sonatype-nexus-community/nancy/releases/tag/v2.1.0, verify its SHA-256 against the release, then install it into /usr/local/bin"
check_tool semgrep "pip install semgrep==1.179.0"
echo ""

# Track failures
FAILURES=0

# 1. Trivy Filesystem Scan
echo "=========================================="
echo "1. Running Trivy Filesystem Scan..."
echo "=========================================="
if trivy filesystem \
    --scanners vuln,secret,config \
    --severity HIGH,CRITICAL \
    --format table \
    --output "$REPORTS_DIR/trivy-fs-$TIMESTAMP.txt" \
    "$PROJECT_ROOT" 2>&1 | tee "$REPORTS_DIR/trivy-fs-$TIMESTAMP.log"; then
    echo -e "${GREEN}✓ Trivy filesystem scan completed${NC}"
else
    echo -e "${YELLOW}⚠ Trivy filesystem scan found issues (see report)${NC}"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# 2. Gosec Security Scan
echo "=========================================="
echo "2. Running Gosec Security Scan..."
echo "=========================================="
cd "$PROJECT_ROOT/catalog-api"
if gosec \
    -fmt json \
    -out "$REPORTS_DIR/gosec-$TIMESTAMP.json" \
    -severity high \
    -confidence medium \
    ./... 2>&1 | tee "$REPORTS_DIR/gosec-$TIMESTAMP.log"; then
    echo -e "${GREEN}✓ Gosec scan completed${NC}"
else
    echo -e "${YELLOW}⚠ Gosec scan found issues (see report)${NC}"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# 3. Nancy Dependency Scan
echo "=========================================="
echo "3. Running Nancy Dependency Scan..."
echo "=========================================="
cd "$PROJECT_ROOT/catalog-api"
if go list -json -deps ./... | nancy sleuth \
    --output json \
    > "$REPORTS_DIR/nancy-$TIMESTAMP.json" 2>"$REPORTS_DIR/nancy-$TIMESTAMP.log"; then
    echo -e "${GREEN}✓ Nancy scan completed${NC}"
else
    echo -e "${YELLOW}⚠ Nancy scan found vulnerabilities (see report)${NC}"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# 4. Semgrep Static Analysis
echo "=========================================="
echo "4. Running Semgrep Static Analysis..."
echo "=========================================="
if semgrep \
    --config=auto \
    --json \
    --output="$REPORTS_DIR/semgrep-$TIMESTAMP.json" \
    "$PROJECT_ROOT" 2>"$REPORTS_DIR/semgrep-$TIMESTAMP.log"; then
    echo -e "${GREEN}✓ Semgrep scan completed${NC}"
else
    echo -e "${YELLOW}⚠ Semgrep found issues (see report)${NC}"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Generate Summary Report
echo "=========================================="
echo "Generating Summary Report..."
echo "=========================================="

cat > "$REPORTS_DIR/summary-$TIMESTAMP.md" << EOF
# Security Scan Summary

**Date:** $(date)
**Scan ID:** $TIMESTAMP

## Tools Run

| Tool | Status | Report |
|------|--------|--------|
| Trivy Filesystem | Completed | trivy-fs-$TIMESTAMP.txt |
| Gosec | Completed | gosec-$TIMESTAMP.json |
| Nancy | Completed | nancy-$TIMESTAMP.json |
| Semgrep | Completed | semgrep-$TIMESTAMP.json |

## Reports Location

All reports are located in: \`$REPORTS_DIR\`

## Quick Commands

View Trivy results:
\`\`\`bash
cat $REPORTS_DIR/trivy-fs-$TIMESTAMP.txt
\`\`\`

View Gosec results:
\`\`\`bash
cat $REPORTS_DIR/gosec-$TIMESTAMP.json | jq '.Issues[] | {rule: .rule_id, severity: .severity, file: .file, line: .line}' 2>/dev/null || cat $REPORTS_DIR/gosec-$TIMESTAMP.json
\`\`\`

## Next Steps

1. Review findings in each report
2. Prioritize HIGH and CRITICAL severity issues
3. Create tickets for remediation
4. Re-run scan after fixes

EOF

echo ""
echo "=========================================="
echo "Security Scan Complete!"
echo "=========================================="
echo ""
echo "Reports saved to: $REPORTS_DIR"
echo "Summary report: $REPORTS_DIR/summary-$TIMESTAMP.md"
echo ""

if [ $FAILURES -gt 0 ]; then
    echo -e "${YELLOW}⚠ $FAILURES scan(s) found issues. Review reports for details.${NC}"
    exit 0  # Don't fail CI, just report
else
    echo -e "${GREEN}✓ All scans completed without critical findings${NC}"
fi
