#!/usr/bin/env bash
# common.sh - Shared functions for the Build framework
# Provides logging, runtime detection, git helpers, and artifact generation
#
# Usage: source this file from your build scripts
#   source "$(dirname "$0")/Build/lib/common.sh"
#
# Requires: BUILD_PROJECT_ROOT to be set (or auto-detected)

set -euo pipefail

# Colors for terminal output
readonly BUILD_RED='\033[0;31m'
readonly BUILD_GREEN='\033[0;32m'
readonly BUILD_YELLOW='\033[1;33m'
readonly BUILD_BLUE='\033[0;34m'
readonly BUILD_CYAN='\033[0;36m'
readonly BUILD_NC='\033[0m' # No Color

# Build framework root (this file's directory -> Build/lib -> Build)
if [[ -z "${BUILD_FRAMEWORK_ROOT:-}" ]]; then
    BUILD_FRAMEWORK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fi

# Project root must be provided or auto-detected
if [[ -z "${BUILD_PROJECT_ROOT:-}" ]]; then
    # Try to detect from framework location (assumes Build/ is in project root)
    BUILD_PROJECT_ROOT="$(cd "$BUILD_FRAMEWORK_ROOT/.." && pwd)"
fi

# ─── Logging ─────────────────────────────────────────────────────────

log_info() {
    echo -e "${BUILD_BLUE}[INFO]${BUILD_NC} $*"
}

log_success() {
    echo -e "${BUILD_GREEN}[OK]${BUILD_NC} $*"
}

log_warn() {
    echo -e "${BUILD_YELLOW}[WARN]${BUILD_NC} $*"
}

log_error() {
    echo -e "${BUILD_RED}[ERROR]${BUILD_NC} $*" >&2
}

log_step() {
    echo -e "${BUILD_CYAN}[STEP]${BUILD_NC} $*"
}

log_header() {
    echo ""
    echo -e "${BUILD_CYAN}════════════════════════════════════════════════════════════${BUILD_NC}"
    echo -e "${BUILD_CYAN}  $*${BUILD_NC}"
    echo -e "${BUILD_CYAN}════════════════════════════════════════════════════════════${BUILD_NC}"
    echo ""
}

# ─── Container Runtime Detection ─────────────────────────────────────

# Detect container runtime (podman preferred, then docker)
detect_runtime() {
    if command -v podman &>/dev/null; then
        echo "podman"
    elif command -v docker &>/dev/null; then
        # podman-only: the builder image runs as uid 1000 and relies on podman's --userns=keep-id (docker has no equivalent)
        log_error "docker found but not supported: this project builds with rootless podman only (--userns=keep-id). Install podman."
        return 1
    else
        log_error "No container runtime found. Install podman."
        return 1
    fi
}

# Detect compose command
detect_compose() {
    local runtime
    runtime="$(detect_runtime)"
    if [[ "$runtime" == "podman" ]]; then
        if command -v podman-compose &>/dev/null; then
            echo "podman-compose"
        else
            log_error "podman-compose not found. Install it with: pip install podman-compose"
            return 1
        fi
    else
        if command -v docker-compose &>/dev/null; then
            echo "docker-compose"
        elif docker compose version &>/dev/null 2>&1; then
            echo "docker compose"
        else
            log_error "docker-compose not found."
            return 1
        fi
    fi
}

# Check if running inside a container
is_container() {
    [[ -f /.dockerenv ]] || [[ -f /run/.containerenv ]] || grep -q 'docker\|lxc\|containerd' /proc/1/cgroup 2>/dev/null
}

# ─── Git Helpers ─────────────────────────────────────────────────────

git_short_commit() {
    git -C "$BUILD_PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown"
}

git_full_commit() {
    git -C "$BUILD_PROJECT_ROOT" rev-parse HEAD 2>/dev/null || echo "unknown"
}

git_branch() {
    git -C "$BUILD_PROJECT_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown"
}

git_is_dirty() {
    ! git -C "$BUILD_PROJECT_ROOT" diff --quiet HEAD 2>/dev/null
}

# ─── Timestamps ──────────────────────────────────────────────────────

build_timestamp() {
    date -u '+%Y-%m-%dT%H:%M:%SZ'
}

# ─── Component Helpers ───────────────────────────────────────────────

# Validate component name against the project's component list
# Requires BUILD_COMPONENTS array to be defined by the project
validate_component() {
    local component="$1"
    if [[ -z "${BUILD_COMPONENTS[*]:-}" ]]; then
        log_error "BUILD_COMPONENTS array not defined. Set it in your project config."
        return 1
    fi
    for c in "${BUILD_COMPONENTS[@]}"; do
        if [[ "$c" == "$component" ]]; then
            return 0
        fi
    done
    log_error "Unknown component: $component"
    log_error "Valid components: ${BUILD_COMPONENTS[*]}"
    return 1
}

# Get component directory path
component_dir() {
    local component="$1"
    echo "$BUILD_PROJECT_ROOT/$component"
}

# Check if a component directory exists
component_exists() {
    local component="$1"
    [[ -d "$(component_dir "$component")" ]]
}

# ─── Release Artifacts ───────────────────────────────────────────────

# Create versioned release directory
# Usage: create_release_dir "component" "platform" "v1.0.0-build.3"
create_release_dir() {
    local component="$1"
    local platform="$2"
    local version_string="$3"
    local dir="$BUILD_PROJECT_ROOT/releases/$component/$platform/$version_string"
    mkdir -p "$dir"
    echo "$dir"
}

# Generate SHA256SUM file for all artifacts in a directory
generate_checksums() {
    local dir="$1"
    (
        cd "$dir"
        find . -type f ! -name 'SHA256SUM' ! -name 'BUILD_INFO.json' -print0 | \
            sort -z | \
            xargs -0 sha256sum > SHA256SUM
    )
}

# Generate BUILD_INFO.json metadata file
generate_build_info() {
    local dir="$1"
    local component="$2"
    local platform="$3"
    local version="$4"
    local build_number="$5"
    local version_string="$6"
    local source_hash="$7"

    cat > "$dir/BUILD_INFO.json" <<EOF
{
  "component": "$component",
  "platform": "$platform",
  "version": "$version",
  "build_number": $build_number,
  "version_string": "$version_string",
  "git_commit": "$(git_short_commit)",
  "git_commit_full": "$(git_full_commit)",
  "git_branch": "$(git_branch)",
  "build_date": "$(build_timestamp)",
  "source_hash": "$source_hash"
}
EOF
}
