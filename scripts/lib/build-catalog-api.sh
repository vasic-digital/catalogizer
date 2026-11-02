#!/usr/bin/env bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/cgo_flags.sh" || { echo "FATAL: cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
# build-catalog-api.sh - Builder for catalog-api (Go backend)
# Produces binaries for the native platform (CGO required for sqlcipher)
# and cross-compiled CGO_ENABLED=0 builds where possible

set -euo pipefail

build_catalog_api() {
    local version="$1"
    local build_number="$2"
    local version_string="$3"
    local source_hash="$4"
    local skip_tests="${BUILD_SKIP_TESTS:-false}"

    local comp_dir="$BUILD_PROJECT_ROOT/catalog-api"
    local build_date
    build_date="$(build_timestamp)"

    # Run tests first (unless skipped)
    if [[ "$skip_tests" != "true" ]]; then
        log_step "Running catalog-api tests..."
        if (cd "$comp_dir" && GOTOOLCHAIN=local go test ./... 2>&1); then
            log_success "Tests passed"
        else
            log_error "Tests failed for catalog-api"
            return 1
        fi
    fi

    # ldflags for version injection
    local ldflags="-X main.Version=$version -X main.BuildNumber=$build_number -X main.BuildDate=$build_date -s -w"

    # Detect native platform
    local native_os native_arch native_platform
    native_os="$(go env GOOS)"
    native_arch="$(go env GOARCH)"
    native_platform="${native_os}-${native_arch}"

    # Build for each platform
    for platform_spec in "${CATALOG_API_PLATFORMS[@]}"; do
        IFS=':' read -r platform goos goarch <<< "$platform_spec"
        log_step "Building catalog-api for $platform..."

        local release_dir
        release_dir="$(create_release_dir "catalog-api" "$platform" "$version_string")"

        local binary_name="catalog-api"
        if [[ "$goos" == "windows" ]]; then
            binary_name="catalog-api.exe"
        fi

        local cgo_enabled="0"
        if [[ "$goos-$goarch" == "$native_os-$native_arch" ]]; then
            # Native platform: enable CGO for sqlcipher support
            cgo_enabled="1"
        fi

        if (cd "$comp_dir" && GOTOOLCHAIN=local GOOS="$goos" GOARCH="$goarch" CGO_ENABLED="$cgo_enabled" \
            go build -ldflags "$ldflags" -o "$release_dir/$binary_name" .); then
            generate_checksums "$release_dir"
            generate_build_info "$release_dir" "catalog-api" "$platform" \
                "$version" "$build_number" "$version_string" "$source_hash"
            log_success "catalog-api ($platform) -> $release_dir"
        else
            if [[ "$cgo_enabled" == "0" ]]; then
                log_warn "Cross-compilation failed for $platform (CGO dependency). Skipping."
            else
                log_error "Failed to build catalog-api for $platform"
                return 1
            fi
        fi
    done
}
