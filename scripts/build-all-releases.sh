#!/bin/bash
# Automated build script for all Catalogizer releases
# Usage: ./scripts/build-all-releases.sh [version]

set -e  # Exit on error

VERSION=${1:-"1.0.0"}
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASES_DIR="$ROOT_DIR/releases"

echo "========================================="
echo "Building Catalogizer Releases v$VERSION"
echo "========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
log_info "Checking prerequisites..."

command -v go >/dev/null 2>&1 || { log_error "Go is not installed"; exit 1; }
command -v node >/dev/null 2>&1 || { log_error "Node.js is not installed"; exit 1; }
command -v npm >/dev/null 2>&1 || { log_error "npm is not installed"; exit 1; }

log_info "All prerequisites met"

# Clean previous releases
log_info "Cleaning previous releases..."
rm -rf "$RELEASES_DIR"
mkdir -p "$RELEASES_DIR"/{linux,windows,macos,android}/{catalog-api,catalog-web,catalogizer-desktop,installer-wizard}
mkdir -p "$RELEASES_DIR/android"/{catalogizer-android,catalogizer-androidtv}

# Build Backend API
log_info "Building catalog-api..."
cd "$ROOT_DIR/catalog-api"

log_info "  - Linux AMD64"
CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$RELEASES_DIR/linux/catalog-api/catalog-api-v$VERSION-linux-amd64" main.go

log_info "  - Windows AMD64"
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$RELEASES_DIR/windows/catalog-api/catalog-api-v$VERSION-windows-amd64.exe" main.go

log_info "  - macOS AMD64"
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$RELEASES_DIR/macos/catalog-api/catalog-api-v$VERSION-macos-amd64" main.go

log_info "  - macOS ARM64 (Apple Silicon)"
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$RELEASES_DIR/macos/catalog-api/catalog-api-v$VERSION-macos-arm64" main.go

# Build Frontend
log_info "Building catalog-web..."
cd "$ROOT_DIR/catalog-web"

if [ -f "package.json" ]; then
    log_info "  - Installing dependencies"
    npm install --production=false

    log_info "  - Building production bundle"
    npm run build

    log_info "  - Copying to releases"
    cp -r dist/* "$RELEASES_DIR/linux/catalog-web/"
    cp -r dist/* "$RELEASES_DIR/windows/catalog-web/"
    cp -r dist/* "$RELEASES_DIR/macos/catalog-web/"
else
    log_warn "  - catalog-web/package.json not found, skipping"
fi

# Build Desktop Application
log_info "Building catalogizer-desktop..."
cd "$ROOT_DIR/catalogizer-desktop"

if [ -f "package.json" ]; then
    log_info "  - Installing dependencies"
    npm install

    log_info "  - Building Tauri application"
    npm run tauri:build || log_warn "Desktop build failed (may need platform-specific tools)"

    # Copy builds if they exist
    if [ -d "src-tauri/target/release/bundle" ]; then
        log_info "  - Copying desktop builds"
        find src-tauri/target/release/bundle -type f \( -name "*.AppImage" -o -name "*.deb" \) \
            -exec cp {} "$RELEASES_DIR/linux/catalogizer-desktop/" \; 2>/dev/null || true
        find src-tauri/target/release/bundle -type f \( -name "*.msi" -o -name "*.exe" \) \
            -exec cp {} "$RELEASES_DIR/windows/catalogizer-desktop/" \; 2>/dev/null || true
        find src-tauri/target/release/bundle -type f \( -name "*.dmg" -o -name "*.app" \) \
            -exec cp {} "$RELEASES_DIR/macos/catalogizer-desktop/" \; 2>/dev/null || true
    fi
else
    log_warn "  - catalogizer-desktop/package.json not found, skipping"
fi

# Build Installer Wizard
log_info "Building installer-wizard..."
cd "$ROOT_DIR/installer-wizard"

if [ -f "package.json" ]; then
    log_info "  - Installing dependencies"
    npm install

    log_info "  - Building installer"
    npm run tauri:build || log_warn "Installer build failed (may need platform-specific tools)"

    # Copy builds if they exist
    if [ -d "src-tauri/target/release/bundle" ]; then
        log_info "  - Copying installer builds"
        find src-tauri/target/release/bundle -type f \( -name "*.AppImage" -o -name "*.deb" \) \
            -exec cp {} "$RELEASES_DIR/linux/installer-wizard/" \; 2>/dev/null || true
        find src-tauri/target/release/bundle -type f \( -name "*.msi" -o -name "*.exe" \) \
            -exec cp {} "$RELEASES_DIR/windows/installer-wizard/" \; 2>/dev/null || true
        find src-tauri/target/release/bundle -type f \( -name "*.dmg" -o -name "*.app" \) \
            -exec cp {} "$RELEASES_DIR/macos/installer-wizard/" \; 2>/dev/null || true
    fi
else
    log_warn "  - installer-wizard/package.json not found, skipping"
fi

# Build Android Applications
log_info "Building Android applications..."

# catalogizer-android
cd "$ROOT_DIR/catalogizer-android"
if [ -f "gradlew" ]; then
    log_info "  - Building catalogizer-android"
    ./gradlew assembleRelease || log_warn "Android build failed"

    if [ -d "app/build/outputs/apk/release" ]; then
        cp app/build/outputs/apk/release/*.apk "$RELEASES_DIR/android/catalogizer-android/" 2>/dev/null || true
    fi
else
    log_warn "  - catalogizer-android/gradlew not found, skipping"
fi

# catalogizer-androidtv
cd "$ROOT_DIR/catalogizer-androidtv"
if [ -f "gradlew" ]; then
    log_info "  - Building catalogizer-androidtv"
    ./gradlew assembleRelease || log_warn "Android TV build failed"

    if [ -d "app/build/outputs/apk/release" ]; then
        cp app/build/outputs/apk/release/*.apk "$RELEASES_DIR/android/catalogizer-androidtv/" 2>/dev/null || true
    fi
else
    log_warn "  - catalogizer-androidtv/gradlew not found, skipping"
fi

# Generate checksums
log_info "Generating SHA256 checksums..."
cd "$RELEASES_DIR"
find . -type f \( -name "*.exe" -o -name "*.AppImage" -o -name "*.dmg" -o -name "*.apk" \
    -o -name "catalog-api*" -o -name "*.deb" -o -name "*.msi" \) \
    -exec sha256sum {} \; > "$RELEASES_DIR/SHA256SUMS.txt"

# Generate release manifest
log_info "Generating release manifest..."
cat > "$RELEASES_DIR/MANIFEST.json" <<EOF
{
  "version": "$VERSION",
  "build_date": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "platforms": {
    "linux": {
      "catalog-api": "$(ls linux/catalog-api/ 2>/dev/null | head -1)",
      "catalogizer-desktop": "$(ls linux/catalogizer-desktop/ 2>/dev/null | head -1)"
    },
    "windows": {
      "catalog-api": "$(ls windows/catalog-api/ 2>/dev/null | head -1)",
      "catalogizer-desktop": "$(ls windows/catalogizer-desktop/ 2>/dev/null | head -1)"
    },
    "macos": {
      "catalog-api": "$(ls macos/catalog-api/ 2>/dev/null | head -1)",
      "catalogizer-desktop": "$(ls macos/catalogizer-desktop/ 2>/dev/null | head -1)"
    },
    "android": {
      "catalogizer-android": "$(ls android/catalogizer-android/ 2>/dev/null | head -1)",
      "catalogizer-androidtv": "$(ls android/catalogizer-androidtv/ 2>/dev/null | head -1)"
    }
  }
}
EOF

# Print summary
log_info "========================================="
log_info "Build Summary for v$VERSION"
log_info "========================================="

echo ""
echo "Linux builds:"
find "$RELEASES_DIR/linux" -type f 2>/dev/null | sed 's|^|  - |' || echo "  (none)"

echo ""
echo "Windows builds:"
find "$RELEASES_DIR/windows" -type f 2>/dev/null | sed 's|^|  - |' || echo "  (none)"

echo ""
echo "macOS builds:"
find "$RELEASES_DIR/macos" -type f 2>/dev/null | sed 's|^|  - |' || echo "  (none)"

echo ""
echo "Android builds:"
find "$RELEASES_DIR/android" -type f 2>/dev/null | sed 's|^|  - |' || echo "  (none)"

echo ""
log_info "Checksums: $RELEASES_DIR/SHA256SUMS.txt"
log_info "Manifest: $RELEASES_DIR/MANIFEST.json"
echo ""
log_info "========================================="
log_info "Release build complete!"
log_info "========================================="
