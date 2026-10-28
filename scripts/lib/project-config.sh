#!/usr/bin/env bash
# project-config.sh - Catalogizer-specific build configuration
# Defines components, source patterns, and project-specific settings
#
# This file is sourced by release-build.sh before the Build framework

set -euo pipefail

# Project root (resolved from this file's location: scripts/lib -> scripts -> project root)
BUILD_PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Build script name (for help text)
BUILD_SCRIPT_NAME="release-build.sh"

# Docker Compose build file
BUILD_COMPOSE_FILE="$BUILD_PROJECT_ROOT/docker-compose.build.yml"

# Builder container image
BUILD_BUILDER_IMAGE="localhost/catalogizer-builder:latest"
BUILD_DOCKERFILE="docker/Dockerfile.builder"

# Cache volumes for container builds (the builder image runs as user `builder`, HOME /home/builder) (space-separated "name:path" pairs)
BUILD_CONTAINER_VOLUMES="go-cache:/home/builder/go npm-cache:/home/builder/.npm gradle-cache:/home/builder/.gradle cargo-cache:/opt/cargo/registry"

# All 7 Catalogizer components
BUILD_COMPONENTS=(
    "catalog-api"
    "catalog-web"
    "catalogizer-api-client"
    "catalogizer-desktop"
    "installer-wizard"
    "catalogizer-android"
    "catalogizer-androidtv"
)

# Source file patterns per component (for change detection hashing)
declare -A BUILD_COMPONENT_PATTERNS=(
    ["catalog-api"]="*.go go.mod go.sum"
    ["catalog-web"]="*.ts *.tsx *.js *.json *.html *.css"
    ["catalogizer-api-client"]="*.ts *.js *.json"
    ["catalogizer-desktop"]="*.ts *.tsx *.js *.json *.html *.css *.rs *.toml"
    ["installer-wizard"]="*.ts *.tsx *.js *.json *.html *.css *.rs *.toml"
    ["catalogizer-android"]="*.kt *.java *.xml *.gradle.kts *.properties"
    ["catalogizer-androidtv"]="*.kt *.java *.xml *.gradle.kts *.properties"
)

# Go cross-compilation targets for catalog-api
CATALOG_API_PLATFORMS=(
    "linux-amd64:linux:amd64"
    "windows-amd64:windows:amd64"
    "darwin-amd64:darwin:amd64"
    "darwin-arm64:darwin:arm64"
)
