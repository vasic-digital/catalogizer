#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
# Nancy Security Scanner Script for Catalogizer
# Scans Go dependencies for vulnerabilities

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
REPORTS_DIR="$PROJECT_ROOT/reports"
CATALOG_API_DIR="$PROJECT_ROOT/catalog-api"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
OUTPUT_FORMAT="json"
OUTPUT_FILE="$REPORTS_DIR/nancy-results.json"
EXIT_CODE=0
VERBOSE=false
BUILD_DEPENDENCIES=false

# Function to print usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -f, --format FORMAT   Output format: json, text, csv (default: json)"
    echo "  -o, --output FILE     Output file path"
    echo "  -e, --exit-code CODE  Exit code on vulnerabilities (default: 0)"
    echo "  -b, --build           Build dependencies before scanning"
    echo "  -v, --verbose         Enable verbose output"
    echo "  -h, --help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                    # Scan with defaults"
    echo "  $0 -b                 # Build and scan"
    echo "  $0 -f csv -o results.csv    # CSV output"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--format)
            OUTPUT_FORMAT="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -e|--exit-code)
            EXIT_CODE="$2"
            shift 2
            ;;
        -b|--build)
            BUILD_DEPENDENCIES=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Create reports directory
mkdir -p "$REPORTS_DIR"

# Check if Nancy is installed
if ! command -v nancy &> /dev/null; then
    echo -e "${YELLOW}Nancy not found. Installing...${NC}"
    go install github.com/sonatypecommunity/nancy@latest
fi

# Verify Nancy installation
if ! command -v nancy &> /dev/null; then
    echo -e "${RED}Error: Nancy installation failed. Please install manually:${NC}"
    echo "  go install github.com/sonatypecommunity/nancy@latest"
    exit 1
fi

NANCY_VERSION=$(nancy --version 2>&1 | head -1)
echo -e "${GREEN}Using Nancy: $NANCY_VERSION${NC}"

# Create Nancy ignore file if it doesn't exist
NANCY_IGNORE="$PROJECT_ROOT/.nancy-ignore"
if [ ! -f "$NANCY_IGNORE" ]; then
    echo "Creating default Nancy ignore file..."
    cat > "$NANCY_IGNORE" << 'EOF'
# Nancy Ignore File
# Format: <vulnerability ID> # <reason>
# Example: CVE-2020-1234 # False positive, not used in our code

# Add accepted/known vulnerabilities below with justification
EOF
    echo -e "${GREEN}Created $NANCY_IGNORE${NC}"
fi

# Change to catalog-api directory
cd "$CATALOG_API_DIR"

# Build dependencies if requested
if [ "$BUILD_DEPENDENCIES" = true ]; then
    echo -e "${BLUE}Building dependencies...${NC}"
    go build ./... || {
        echo -e "${RED}Error: Build failed${NC}"
        exit 1
    }
fi

# Check if go.sum exists
if [ ! -f "go.sum" ]; then
    echo -e "${YELLOW}go.sum not found. Generating...${NC}"
    go mod tidy
fi

echo -e "${GREEN}Starting Nancy dependency vulnerability scan...${NC}"
echo ""

# Run Nancy based on output format
case $OUTPUT_FORMAT in
    json)
        echo "Scanning in JSON mode..."
        if [ "$VERBOSE" = true ]; then
            go list -json -deps ./... | nancy sleuth --output json > "$OUTPUT_FILE" 2>&1 || NANCY_EXIT=$?
        else
            go list -json -deps ./... | nancy sleuth --output json > "$OUTPUT_FILE" 2>&1 || NANCY_EXIT=$?
        fi
        ;;
    
    csv)
        echo "Scanning in CSV mode..."
        go list -json -deps ./... | nancy sleuth --output csv > "$OUTPUT_FILE" 2>&1 || NANCY_EXIT=$?
        ;;
    
    text|*)
        echo "Scanning in text mode..."
        go list -json -deps ./... | nancy sleuth > "$OUTPUT_FILE" 2>&1 || NANCY_EXIT=$?
        ;;
esac

# Nancy returns exit code 1 if vulnerabilities found
NANCY_EXIT=${NANCY_EXIT:-0}

# Check results
if [ -f "$OUTPUT_FILE" ]; then
    echo -e "${GREEN}Scan complete. Results saved to: $OUTPUT_FILE${NC}"
    
    # Parse results based on format
    if [ "$OUTPUT_FORMAT" = "json" ] && command -v jq &> /dev/null; then
        # Count vulnerabilities by severity
        VULN_COUNT=$(jq 'length' "$OUTPUT_FILE" 2>/dev/null || echo "0")
        
        if [ "$VULN_COUNT" -gt 0 ]; then
            CRITICAL=$(jq '[.[] | select(.Vulnerability.Severity == "Critical")] | length' "$OUTPUT_FILE" 2>/dev/null || echo "0")
            HIGH=$(jq '[.[] | select(.Vulnerability.Severity == "High")] | length' "$OUTPUT_FILE" 2>/dev/null || echo "0")
            MEDIUM=$(jq '[.[] | select(.Vulnerability.Severity == "Medium")] | length' "$OUTPUT_FILE" 2>/dev/null || echo "0")
            LOW=$(jq '[.[] | select(.Vulnerability.Severity == "Low")] | length' "$OUTPUT_FILE" 2>/dev/null || echo "0")
            
            echo ""
            echo "========================================"
            echo "Vulnerability Summary:"
            echo "========================================"
            echo -e "${RED}Critical: $CRITICAL${NC}"
            echo -e "${RED}High: $HIGH${NC}"
            echo -e "${YELLOW}Medium: $MEDIUM${NC}"
            echo -e "${GREEN}Low: $LOW${NC}"
            echo "----------------------------------------"
            echo "Total: $VULN_COUNT"
            echo "========================================"
            
            # Show affected packages
            echo ""
            echo "Affected packages:"
            jq -r '.[] | "  - \(.Coordinates) (\(.Vulnerability.Severity)): \(.Vulnerability.Title)"' "$OUTPUT_FILE" 2>/dev/null | sort -u | head -20 || true
            
            # Exit with error if vulnerabilities found
            if [ "$EXIT_CODE" -ne 0 ]; then
                echo ""
                echo -e "${RED}Found $VULN_COUNT vulnerabilities${NC}"
                echo "Review $OUTPUT_FILE for details"
                echo "Add accepted vulnerabilities to $NANCY_IGNORE with justification"
                exit "$EXIT_CODE"
            fi
        else
            echo -e "${GREEN}No vulnerabilities found!${NC}"
        fi
    else
        # Text format summary
        VULN_COUNT=$(grep -c "Vulnerability" "$OUTPUT_FILE" 2>/dev/null || echo "0")
        if [ "$VULN_COUNT" -gt 0 ]; then
            echo "Found potential vulnerabilities. Review $OUTPUT_FILE"
            if [ "$EXIT_CODE" -ne 0 ]; then
                exit "$EXIT_CODE"
            fi
        fi
    fi
else
    echo -e "${RED}Error: Output file not created${NC}"
    exit 1
fi

echo -e "${GREEN}Nancy scan completed successfully${NC}"
exit 0
