#!/bin/bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/../../scripts/lib/cgo_flags.sh" || { echo "FATAL: ../../scripts/lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh

# Build script for Catalog API

set -e

echo "Building Catalog API..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | cut -d' ' -f3 | sed 's/go//')
REQUIRED_VERSION="1.19"

if ! printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V -C; then
    echo "Error: Go version $GO_VERSION is too old. Minimum required version is $REQUIRED_VERSION"
    exit 1
fi

echo "Go version: $GO_VERSION ✓"

# Navigate to project directory
cd "$(dirname "$0")/.."

# Download dependencies
echo "Downloading dependencies..."
go mod tidy
go mod download

# Verify dependencies
echo "Verifying dependencies..."
go mod verify

# Run tests
echo "Running tests..."
go test ./... -v

# Build for current platform
echo "Building for current platform..."
go build -o bin/catalog-api -ldflags="-s -w" .

# Build for common platforms
echo "Building for multiple platforms..."

# Linux amd64
GOOS=linux GOARCH=amd64 go build -o bin/catalog-api-linux-amd64 -ldflags="-s -w" .

# Linux arm64
GOOS=linux GOARCH=arm64 go build -o bin/catalog-api-linux-arm64 -ldflags="-s -w" .

# Windows amd64
GOOS=windows GOARCH=amd64 go build -o bin/catalog-api-windows-amd64.exe -ldflags="-s -w" .

# macOS amd64
GOOS=darwin GOARCH=amd64 go build -o bin/catalog-api-darwin-amd64 -ldflags="-s -w" .

# macOS arm64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o bin/catalog-api-darwin-arm64 -ldflags="-s -w" .

echo "Build completed successfully!"
echo "Binaries available in bin/ directory:"
ls -la bin/

echo ""
echo "To run the API server:"
echo "  ./bin/catalog-api"
echo ""
echo "To run with custom config:"
echo "  CATALOG_CONFIG_PATH=/path/to/config.json ./bin/catalog-api"