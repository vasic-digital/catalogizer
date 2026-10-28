#!/usr/bin/env bash
# orchestrator.sh - Generic build orchestration for the Build framework
# Provides CLI parsing, component iteration, container launching, and build reporting
#
# Usage: source this file after all other Build framework files, then call
#   build_main "$@"
#
# Projects must define:
#   BUILD_COMPONENTS - array of component names
#   BUILD_COMPONENT_PATTERNS - associative array for hash computation
#   build_single_component() - function to build a single component
#
# Optional project settings:
#   BUILD_BUILDER_IMAGE - container image for builds (default: localhost/catalogizer-builder:latest)
#   BUILD_COMPOSE_FILE - path to docker-compose build file
#   BUILD_CONTAINER_VOLUMES - extra volume mounts for container builds

set -euo pipefail

# Source framework if not already loaded
if ! declare -f log_info &>/dev/null; then
    source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
fi
if ! declare -f get_version &>/dev/null; then
    source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/version.sh"
fi
if ! declare -f compute_source_hash &>/dev/null; then
    source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/hash.sh"
fi

# ─── Build Report Tracking ───────────────────────────────────────────
# Arrays to collect per-component build data for the final report

declare -a _REPORT_COMPONENTS=()
declare -a _REPORT_STATUSES=()
declare -a _REPORT_DURATIONS=()
declare -a _REPORT_ARTIFACT_SIZES=()
declare -a _REPORT_ARTIFACT_PATHS=()
declare -a _REPORT_PLATFORMS=()
declare -a _REPORT_HASHES=()

# Record build result for a component
_report_add() {
    local component="$1"
    local status="$2"
    local duration="$3"
    local artifact_path="$4"
    local platform="$5"
    local source_hash="$6"

    _REPORT_COMPONENTS+=("$component")
    _REPORT_STATUSES+=("$status")
    _REPORT_DURATIONS+=("$duration")
    _REPORT_ARTIFACT_PATHS+=("$artifact_path")
    _REPORT_PLATFORMS+=("$platform")
    _REPORT_HASHES+=("$source_hash")

    # Calculate total artifact size
    local total_size=0
    if [[ -d "$artifact_path" ]]; then
        total_size=$(du -sb "$artifact_path" 2>/dev/null | cut -f1 || echo 0)
    fi
    _REPORT_ARTIFACT_SIZES+=("$total_size")
}

# Format bytes to human-readable
_format_size() {
    local bytes="$1"
    if [[ "$bytes" -ge 1073741824 ]]; then
        echo "$(awk "BEGIN{printf \"%.2f GB\", $bytes/1073741824}")"
    elif [[ "$bytes" -ge 1048576 ]]; then
        echo "$(awk "BEGIN{printf \"%.2f MB\", $bytes/1048576}")"
    elif [[ "$bytes" -ge 1024 ]]; then
        echo "$(awk "BEGIN{printf \"%.1f KB\", $bytes/1024}")"
    else
        echo "${bytes} B"
    fi
}

# Format seconds to human-readable duration
_format_duration() {
    local secs="$1"
    if [[ "$secs" -ge 3600 ]]; then
        printf "%dh %dm %ds" $((secs/3600)) $((secs%3600/60)) $((secs%60))
    elif [[ "$secs" -ge 60 ]]; then
        printf "%dm %ds" $((secs/60)) $((secs%60))
    else
        printf "%ds" "$secs"
    fi
}

# Print the final build report
print_build_report() {
    local version_string="$1"
    local build_number="$2"
    local total_duration="$3"
    local total_components=${#_REPORT_COMPONENTS[@]}

    if [[ $total_components -eq 0 ]]; then
        return
    fi

    echo ""
    echo -e "${BUILD_CYAN}╔══════════════════════════════════════════════════════════════════╗${BUILD_NC}"
    echo -e "${BUILD_CYAN}║                      BUILD REPORT                              ║${BUILD_NC}"
    echo -e "${BUILD_CYAN}╠══════════════════════════════════════════════════════════════════╣${BUILD_NC}"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Version:      ${BUILD_GREEN}$version_string${BUILD_NC}"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Build Number: ${BUILD_GREEN}$build_number${BUILD_NC}"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Git:          $(git_branch) @ $(git_short_commit)$(git_is_dirty && echo ' (dirty)' || true)"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Date:         $(build_timestamp)"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Duration:     $(_format_duration "$total_duration")"
    if is_container; then
        echo -e "${BUILD_CYAN}║${BUILD_NC}  Environment:  Container"
    else
        echo -e "${BUILD_CYAN}║${BUILD_NC}  Environment:  Host"
    fi
    echo -e "${BUILD_CYAN}╠══════════════════════════════════════════════════════════════════╣${BUILD_NC}"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  ${BUILD_YELLOW}Component Results${BUILD_NC}"
    echo -e "${BUILD_CYAN}╠══════════════════════════════════════════════════════════════════╣${BUILD_NC}"

    local success_count=0
    local fail_count=0
    local total_size=0

    for i in $(seq 0 $((total_components - 1))); do
        local comp="${_REPORT_COMPONENTS[$i]}"
        local status="${_REPORT_STATUSES[$i]}"
        local dur="${_REPORT_DURATIONS[$i]}"
        local size="${_REPORT_ARTIFACT_SIZES[$i]}"
        local path="${_REPORT_ARTIFACT_PATHS[$i]}"
        local platform="${_REPORT_PLATFORMS[$i]}"
        local hash="${_REPORT_HASHES[$i]}"

        local status_icon status_color
        if [[ "$status" == "SUCCESS" ]]; then
            status_icon="+"
            status_color="$BUILD_GREEN"
            success_count=$((success_count + 1))
            total_size=$((total_size + size))
        else
            status_icon="x"
            status_color="$BUILD_RED"
            fail_count=$((fail_count + 1))
        fi

        echo -e "${BUILD_CYAN}║${BUILD_NC}"
        echo -e "${BUILD_CYAN}║${BUILD_NC}  ${status_color}[${status_icon}] ${comp}${BUILD_NC}"
        echo -e "${BUILD_CYAN}║${BUILD_NC}      Status:    ${status_color}${status}${BUILD_NC}"
        echo -e "${BUILD_CYAN}║${BUILD_NC}      Platform:  ${platform}"
        echo -e "${BUILD_CYAN}║${BUILD_NC}      Duration:  $(_format_duration "$dur")"
        if [[ "$status" == "SUCCESS" ]]; then
            echo -e "${BUILD_CYAN}║${BUILD_NC}      Size:      $(_format_size "$size")"
            echo -e "${BUILD_CYAN}║${BUILD_NC}      Artifacts: ${path/$BUILD_PROJECT_ROOT\//}"
            echo -e "${BUILD_CYAN}║${BUILD_NC}      Hash:      ${hash:0:16}..."
        fi
    done

    echo -e "${BUILD_CYAN}╠══════════════════════════════════════════════════════════════════╣${BUILD_NC}"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  ${BUILD_YELLOW}Summary${BUILD_NC}"
    echo -e "${BUILD_CYAN}╠══════════════════════════════════════════════════════════════════╣${BUILD_NC}"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Total:        $total_components component(s)"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Succeeded:    ${BUILD_GREEN}$success_count${BUILD_NC}"
    if [[ $fail_count -gt 0 ]]; then
        echo -e "${BUILD_CYAN}║${BUILD_NC}  Failed:       ${BUILD_RED}$fail_count${BUILD_NC}"
    fi
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Total Size:   $(_format_size "$total_size")"
    echo -e "${BUILD_CYAN}║${BUILD_NC}  Total Time:   $(_format_duration "$total_duration")"
    echo -e "${BUILD_CYAN}╚══════════════════════════════════════════════════════════════════╝${BUILD_NC}"
    echo ""

    # Also generate a JSON report in the releases directory
    _generate_json_report "$version_string" "$build_number" "$total_duration"
}

# Generate machine-readable JSON build report
_generate_json_report() {
    local version_string="$1"
    local build_number="$2"
    local total_duration="$3"
    local report_file="$BUILD_PROJECT_ROOT/releases/BUILD_REPORT.json"
    local total_components=${#_REPORT_COMPONENTS[@]}

    mkdir -p "$BUILD_PROJECT_ROOT/releases"

    local components_json=""
    for i in $(seq 0 $((total_components - 1))); do
        [[ -n "$components_json" ]] && components_json+=","
        components_json+="$(cat <<COMPEOF
    {
      "component": "${_REPORT_COMPONENTS[$i]}",
      "status": "${_REPORT_STATUSES[$i]}",
      "duration_seconds": ${_REPORT_DURATIONS[$i]},
      "artifact_size_bytes": ${_REPORT_ARTIFACT_SIZES[$i]},
      "artifact_path": "${_REPORT_ARTIFACT_PATHS[$i]}",
      "platform": "${_REPORT_PLATFORMS[$i]}",
      "source_hash": "${_REPORT_HASHES[$i]}"
    }
COMPEOF
)"
    done

    cat > "$report_file" <<EOF
{
  "version_string": "$version_string",
  "build_number": $build_number,
  "git_commit": "$(git_short_commit)",
  "git_branch": "$(git_branch)",
  "build_date": "$(build_timestamp)",
  "total_duration_seconds": $total_duration,
  "environment": "$(is_container && echo "container" || echo "host")",
  "components": [
$components_json
  ]
}
EOF
    log_info "Build report saved to: releases/BUILD_REPORT.json"
}

# ─── Container Build ─────────────────────────────────────────────────

# Launch the build inside a container, forwarding all CLI args
launch_container_build() {
    local runtime
    runtime="$(detect_runtime)"

    local builder_image="${BUILD_BUILDER_IMAGE:-localhost/catalogizer-builder:latest}"

    # Check if image exists
    if ! $runtime image exists "$builder_image" 2>/dev/null; then
        log_warn "Builder image not found: $builder_image"
        log_step "Building builder image (this may take a while on first run)..."
        local dockerfile="${BUILD_DOCKERFILE:-docker/Dockerfile.builder}"
        if [[ ! -f "$BUILD_PROJECT_ROOT/$dockerfile" ]]; then
            log_error "Dockerfile not found: $dockerfile"
            return 1
        fi
        if ! $runtime build --network host \
            -t "$builder_image" \
            -f "$BUILD_PROJECT_ROOT/$dockerfile" \
            "$BUILD_PROJECT_ROOT"; then
            log_error "Failed to build builder image"
            return 1
        fi
    fi

    log_info "Launching containerized build..."
    log_info "Runtime: $runtime"
    log_info "Image: $builder_image"

    # Build args to forward (replace --container with --local)
    local forward_args=()
    local skip_next=false
    for arg in "$@"; do
        if [[ "$skip_next" == "true" ]]; then
            forward_args+=("$arg")
            skip_next=false
            continue
        fi
        case "$arg" in
            --container) forward_args+=("--local") ;;
            --component|--bump)
                forward_args+=("$arg")
                skip_next=true
                ;;
            *) forward_args+=("$arg") ;;
        esac
    done

    # Construct volume mounts
    local vol_args=(
        -v "$BUILD_PROJECT_ROOT:/project:Z"
    )

    # Add cache volumes for faster rebuilds
    local cache_volumes="${BUILD_CONTAINER_VOLUMES:-go-cache:/home/builder/go npm-cache:/home/builder/.npm gradle-cache:/home/builder/.gradle cargo-cache:/opt/cargo/registry}"
    for vol in $cache_volumes; do
        vol_args+=(-v "$vol")
    done

    # Construct environment variables
    local env_args=(
        -e "GOTOOLCHAIN=local"
        -e "ANDROID_HOME=/opt/android-sdk"
        -e "ANDROID_SDK_ROOT=/opt/android-sdk"
        -e "CI=true"
        -e "BUILD_SKIP_TESTS=${BUILD_SKIP_TESTS:-false}"
    )

    # Run the build
    # The builder image runs as uid 1000 (user `builder`, docker/Dockerfile.builder). Under rootless podman the bind-mounted
    # repo is owned by the host user, which is root inside the default user namespace; keep-id maps the host user to uid 1000
    # so /project is writable. (podman-only; the project does not support docker.)
    $runtime run --rm \
        --userns=keep-id:uid=1000,gid=1000 \
        --network host \
        --entrypoint bash \
        "${vol_args[@]}" \
        "${env_args[@]}" \
        -w /project \
        "$builder_image" \
        -c "/project/scripts/release-build.sh ${forward_args[*]}"

    return $?
}

# ─── CLI ─────────────────────────────────────────────────────────────

# Print usage - can be overridden by projects
build_usage() {
    local script_name="${BUILD_SCRIPT_NAME:-release-build.sh}"
    cat <<EOF
Usage: $script_name [OPTIONS]

Build system with automatic versioning and change detection.

Options:
  --component NAME    Build a single component
  --force             Force rebuild of all components (ignore change detection)
  --dry-run           Show what would be built without building
  --bump TYPE         Bump version before building (major|minor|patch)
  --skip-tests        Skip test phase during build
  --container         Force containerized build (requires builder image)
  --local             Force local build (no container)
  --distributed       Distribute builds across remote hosts (requires Go)
  --env-file FILE     Environment file for distributed builds (default: .env.distributed)
  --status            Show component change detection status and exit
  --version           Show current version and exit
  --help              Show this help message

Components: ${BUILD_COMPONENTS[*]:-none defined}

Examples:
  $script_name                           # Build all changed components
  $script_name --force                   # Rebuild everything
  $script_name --component my-api        # Build single component
  $script_name --dry-run                 # Preview what would build
  $script_name --bump patch              # Increment patch version first
  $script_name --skip-tests              # Skip tests during build
  $script_name --container --force       # Rebuild all in container
EOF
}

# Parse CLI arguments into BUILD_* variables
parse_build_args() {
    BUILD_FORCE=false
    BUILD_DRY_RUN=false
    BUILD_SKIP_TESTS=false
    BUILD_SINGLE_COMPONENT=""
    BUILD_BUMP=""
    BUILD_USE_CONTAINER=""
    BUILD_SHOW_STATUS=false
    BUILD_SHOW_VERSION=false
    BUILD_DISTRIBUTED=false
    BUILD_ENV_FILE=".env.distributed"

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --component)
                BUILD_SINGLE_COMPONENT="$2"
                shift 2
                ;;
            --force)
                BUILD_FORCE=true
                shift
                ;;
            --dry-run)
                BUILD_DRY_RUN=true
                shift
                ;;
            --bump)
                BUILD_BUMP="$2"
                shift 2
                ;;
            --skip-tests)
                BUILD_SKIP_TESTS=true
                shift
                ;;
            --container)
                BUILD_USE_CONTAINER=true
                shift
                ;;
            --local)
                BUILD_USE_CONTAINER=false
                shift
                ;;
            --distributed)
                BUILD_DISTRIBUTED=true
                shift
                ;;
            --env-file)
                BUILD_ENV_FILE="$2"
                shift 2
                ;;
            --status)
                BUILD_SHOW_STATUS=true
                shift
                ;;
            --version)
                BUILD_SHOW_VERSION=true
                shift
                ;;
            --help|-h)
                build_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                build_usage
                exit 1
                ;;
        esac
    done

    # Validate component if specified
    if [[ -n "$BUILD_SINGLE_COMPONENT" ]]; then
        validate_component "$BUILD_SINGLE_COMPONENT"
    fi

    # Validate bump type if specified
    if [[ -n "$BUILD_BUMP" ]]; then
        case "$BUILD_BUMP" in
            major|minor|patch) ;;
            *)
                log_error "Invalid bump type: $BUILD_BUMP"
                exit 1
                ;;
        esac
    fi
}

# Determine which components need building
get_components_to_build() {
    local force="$1"
    local single="${2:-}"
    local to_build=()

    if [[ -n "$single" ]]; then
        if needs_rebuild "$single" "$force"; then
            to_build+=("$single")
        elif [[ "$force" != "true" ]]; then
            log_info "$single is up to date (no changes detected)"
        fi
    else
        for component in "${BUILD_COMPONENTS[@]}"; do
            if needs_rebuild "$component" "$force"; then
                to_build+=("$component")
            else
                log_info "Skipping $component (up to date)"
            fi
        done
    fi

    echo "${to_build[@]}"
}

# Detect platform string for a component
_detect_platform() {
    local component="$1"
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
    esac

    # Some components have specific platform semantics
    case "$component" in
        catalog-api)        echo "multi-platform" ;;
        catalog-web)        echo "web" ;;
        catalogizer-api-client) echo "npm" ;;
        catalogizer-android|catalogizer-androidtv) echo "android" ;;
        *)                  echo "${os}-${arch}" ;;
    esac
}

# Find the release directory for a component after build
_find_release_dir() {
    local component="$1"
    local version_string="$2"
    # Look for any directory under releases/$component/ that matches the version
    local base="$BUILD_PROJECT_ROOT/releases/$component"
    if [[ -d "$base" ]]; then
        local found
        found=$(find "$base" -type d -name "$version_string" 2>/dev/null | head -1)
        if [[ -n "$found" ]]; then
            echo "$found"
            return
        fi
        # Fallback: find any directory with BUILD_INFO.json
        found=$(find "$base" -name "BUILD_INFO.json" -type f 2>/dev/null | head -1)
        if [[ -n "$found" ]]; then
            dirname "$found"
            return
        fi
    fi
    echo "$base"
}

# ─── Main ────────────────────────────────────────────────────────────

# Main build orchestration loop
build_main() {
    local all_args=("$@")
    parse_build_args "$@"

    # Handle container mode: re-launch inside container
    if [[ "$BUILD_USE_CONTAINER" == "true" ]] && ! is_container; then
        launch_container_build "${all_args[@]}"
        return $?
    fi

    if [[ "$BUILD_DISTRIBUTED" == "true" ]]; then
        log_info "Distributed build mode enabled"
        local dist_dir="$BUILD_PROJECT_ROOT/Containers"
        local dist_cmd="./cmd/distributed-build"
        if ! command -v go &>/dev/null; then
            log_error "Go is required for distributed builds"
            return 1
        fi
        local env_path="$BUILD_PROJECT_ROOT/$BUILD_ENV_FILE"
        (cd "$dist_dir" && go run "$dist_cmd" \
            --project "$BUILD_PROJECT_ROOT" \
            --env "$env_path" \
            ${BUILD_SKIP_TESTS:+--skip-tests} \
            ${BUILD_SINGLE_COMPONENT:+--component "$BUILD_SINGLE_COMPONENT"} \
            ${BUILD_DRY_RUN:+--dry-run})
        return $?
    fi

    # Initialize versions file
    init_versions

    # Handle --version
    if [[ "$BUILD_SHOW_VERSION" == "true" ]]; then
        echo "$(get_version_string)"
        exit 0
    fi

    # Handle --status
    if [[ "$BUILD_SHOW_STATUS" == "true" ]]; then
        show_hash_status "$BUILD_FORCE"
        exit 0
    fi

    log_header "Build System"
    log_info "Version: $(get_version_string)"
    log_info "Git: $(git_branch) @ $(git_short_commit)"

    # Handle version bump
    if [[ -n "$BUILD_BUMP" ]]; then
        bump_version "$BUILD_BUMP"
    fi

    # Determine what to build
    local components_str
    components_str="$(get_components_to_build "$BUILD_FORCE" "$BUILD_SINGLE_COMPONENT")"

    if [[ -z "$components_str" ]]; then
        log_success "All components are up to date. Nothing to build."
        if [[ "$BUILD_FORCE" != "true" ]]; then
            log_info "Use --force to rebuild anyway."
        fi
        exit 0
    fi

    # Convert to array
    local components_to_build
    read -ra components_to_build <<< "$components_str"

    log_info "Components to build: ${components_to_build[*]}"

    # Dry run - just show what would happen
    if [[ "$BUILD_DRY_RUN" == "true" ]]; then
        log_header "Dry Run - Would Build"
        for component in "${components_to_build[@]}"; do
            local hash
            hash="$(compute_source_hash "$component")"
            echo "  - $component (hash: ${hash:0:12})"
        done
        echo ""
        log_info "Version would be: $(get_version_string)"
        log_info "Use without --dry-run to actually build."
        exit 0
    fi

    # Increment build number
    local build_number
    build_number="$(increment_build_number)"
    local version version_string
    version="$(get_version)"
    version_string="$(get_version_string)"

    log_info "Build number: $build_number"
    log_info "Version string: $version_string"

    # Track overall build time
    local build_start_time=$SECONDS

    # Build each component
    local build_success=true
    local built_count=0
    local failed_count=0

    for component in "${components_to_build[@]}"; do
        log_header "Building: $component"

        local source_hash
        source_hash="$(compute_source_hash "$component")"
        local platform
        platform="$(_detect_platform "$component")"

        # Track per-component time
        local comp_start=$SECONDS

        if declare -f build_single_component &>/dev/null; then
            if build_single_component "$component" "$version" "$build_number" "$version_string" "$source_hash"; then
                local comp_duration=$((SECONDS - comp_start))
                update_component_state "$component" "$build_number" "$source_hash"

                local release_dir
                release_dir="$(_find_release_dir "$component" "$version_string")"

                _report_add "$component" "SUCCESS" "$comp_duration" "$release_dir" "$platform" "$source_hash"
                log_success "$component built successfully ($(_format_duration "$comp_duration"))"
                built_count=$((built_count + 1))
            else
                local comp_duration=$((SECONDS - comp_start))
                _report_add "$component" "FAILED" "$comp_duration" "" "$platform" "$source_hash"
                log_error "$component build FAILED ($(_format_duration "$comp_duration"))"
                build_success=false
                failed_count=$((failed_count + 1))
            fi
        else
            log_error "build_single_component() not defined. Projects must implement this function."
            exit 1
        fi
    done

    local total_duration=$((SECONDS - build_start_time))

    # Print detailed build report
    print_build_report "$version_string" "$build_number" "$total_duration"

    if [[ "$build_success" == "true" ]]; then
        log_success "All builds completed successfully."
    else
        log_error "Some builds failed."
        exit 1
    fi
}
