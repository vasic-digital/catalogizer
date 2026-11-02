#!/bin/bash
#
# Deployment Automation Script
# Builds and deploys Catalogizer to production
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/build"
DEPLOY_DIR="${PROJECT_ROOT}/deployment"
VERSION=$(date +%Y%m%d-%H%M%S)

# Source container runtime detection library
source "${SCRIPT_DIR}/lib/container-runtime.sh"

echo "=== Catalogizer Deployment Script ==="
echo "Version: ${VERSION}"
echo "Build Dir: ${BUILD_DIR}"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Pre-deployment checks
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Go version
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    log_info "Go version: ${GO_VERSION}"
    
    # Check Node.js
    if ! command -v node &> /dev/null; then
        log_error "Node.js is not installed"
        exit 1
    fi
    
    NODE_VERSION=$(node --version)
    log_info "Node.js version: ${NODE_VERSION}"
    
    # Check container runtime
    if container_available; then
        detect_container_runtime
    else
        log_warn "No container runtime found, skipping container builds"
    fi
    
    log_info "Prerequisites check passed"
}

# Run tests
run_tests() {
    log_info "Running tests..."
    
    cd "${PROJECT_ROOT}/catalog-api"
    CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOMAXPROCS=3 go test -short ./... 2>&1 | tail -5
    
    if [ ${PIPESTATUS[0]} -ne 0 ]; then
        log_error "Tests failed"
        exit 1
    fi
    
    log_info "Tests passed"
}

# Build backend
build_backend() {
    log_info "Building backend..."
    
    cd "${PROJECT_ROOT}/catalog-api"
    
    # Build for multiple platforms
    platforms=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64")
    
    for platform in "${platforms[@]}"; do
        IFS='/' read -r GOOS GOARCH <<< "$platform"
        output="${BUILD_DIR}/catalog-api-${VERSION}-${GOOS}-${GOARCH}"
        
        log_info "Building for ${GOOS}/${GOARCH}..."
        CGO_CPPFLAGS=-DHAVE_USLEEP=1 GOOS=${GOOS} GOARCH=${GOARCH} GOMAXPROCS=3 go build \
            -ldflags "-X main.Version=${VERSION} -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
            -o "${output}" \
            . 2>&1 | head -5
    done
    
    log_info "Backend built successfully"
}

# Build frontend
build_frontend() {
    log_info "Building frontend..."
    
    cd "${PROJECT_ROOT}/catalog-web"
    
    npm ci
    npm run build
    
    # Copy build to deployment directory
    mkdir -p "${BUILD_DIR}/web"
    cp -r dist/* "${BUILD_DIR}/web/"
    
    log_info "Frontend built successfully"
}

# Build container images
build_containers() {
    if [[ -z "${CONTAINER_CMD:-}" ]]; then
        log_warn "No container runtime available, skipping container builds"
        return
    fi

    log_info "Building container images (${CONTAINER_RUNTIME})..."

    cd "${PROJECT_ROOT}"

    # Build API image
    container_build -f Dockerfile.api -t catalogizer/api:${VERSION} .
    $CONTAINER_CMD tag catalogizer/api:${VERSION} catalogizer/api:latest

    # Build Web image
    container_build -f Dockerfile.web -t catalogizer/web:${VERSION} .
    $CONTAINER_CMD tag catalogizer/web:${VERSION} catalogizer/web:latest

    log_info "Container images built successfully"
}

# Package release
package_release() {
    log_info "Packaging release..."
    
    mkdir -p "${BUILD_DIR}/releases"
    
    # Create release archive
    tar -czf "${BUILD_DIR}/releases/catalogizer-${VERSION}.tar.gz" \
        -C "${BUILD_DIR}" \
        catalog-api-* \
        web/
    
    # Create checksums
    cd "${BUILD_DIR}/releases"
    sha256sum catalogizer-${VERSION}.tar.gz > catalogizer-${VERSION}.sha256
    
    log_info "Release packaged: ${BUILD_DIR}/releases/catalogizer-${VERSION}.tar.gz"
}

# Deploy to production
deploy() {
    log_info "Deploying to production..."
    
    # This is a placeholder - actual deployment would depend on your infrastructure
    # Options: Kubernetes, Docker Compose, systemd, etc.
    
    log_warn "Deployment step is a placeholder"
    log_info "To deploy:"
    log_info "  1. Copy ${BUILD_DIR}/releases/catalogizer-${VERSION}.tar.gz to server"
    log_info "  2. Extract and run ./install.sh"
    log_info "  3. Or use compose: ${COMPOSE_CMD:-podman-compose} up -d"
}

# Main execution
main() {
    log_info "Starting deployment process..."
    
    check_prerequisites
    run_tests
    build_backend
    build_frontend
    build_containers
    package_release
    deploy
    
    log_info "=== Deployment Complete ==="
    log_info "Version: ${VERSION}"
    log_info "Artifacts: ${BUILD_DIR}/releases/"
}

# Run main function
main "$@"
