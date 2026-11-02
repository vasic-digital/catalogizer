#!/usr/bin/env bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo "=== Starting SonarQube Infrastructure ==="
podman-compose -f "$PROJECT_ROOT/docker-compose.security.yml" up -d sonarqube sonarqube-db

echo "=== Waiting for SonarQube to be healthy (up to 5 min) ==="
for i in $(seq 1 60); do
    if curl -sf http://localhost:9000/api/system/status 2>/dev/null | grep -q '"status":"UP"'; then
        log_info "SonarQube is ready"
        break
    fi
    if [ "$i" -eq 60 ]; then
        log_error "SonarQube failed to start within 5 minutes"
        exit 1
    fi
    echo "Waiting... ($i/60)"
    sleep 5
done

echo "=== Generating Go coverage report ==="
cd "$PROJECT_ROOT/catalog-api"
GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -coverprofile=coverage.out -count=1 -short 2>&1 || true
go test ./... -json > test-results.json 2>/dev/null || true
cd "$PROJECT_ROOT"

echo "=== Generating Frontend coverage report ==="
cd "$PROJECT_ROOT/catalog-web"
npm run test:coverage 2>&1 || true
cd "$PROJECT_ROOT"

echo "=== Running SonarQube Scanner ==="
podman run --rm --network host \
    -v "$PROJECT_ROOT:/usr/src:Z" \
    -w /usr/src \
    docker.io/sonarsource/sonar-scanner-cli:latest \
    -Dsonar.host.url=http://localhost:9000 \
    -Dsonar.login=admin \
    -Dsonar.password=admin

echo "=== SonarQube scan complete ==="
echo "View results at: http://localhost:9000/dashboard?id=catalogizer"
echo ""
echo "To stop SonarQube:"
echo "  podman-compose -f docker-compose.security.yml down"
