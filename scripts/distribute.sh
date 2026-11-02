#!/usr/bin/env bash
# distribute.sh - Distribute catalog-api + catalogizer-androidtv builds via Firebase
# App Distribution. Reads FIREBASE_TESTER_EMAILS (comma-separated) from .env,
# builds both artifacts with production settings, adds testers, and uploads.
#
# §11.4.18 — companion doc at docs/scripts/distribute.md

set -euo pipefail

# ── Config ──────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Source project config for shared paths
if [[ -f "$SCRIPT_DIR/lib/project-config.sh" ]]; then
    # shellcheck source=scripts/lib/project-config.sh
    source "$SCRIPT_DIR/lib/project-config.sh"
fi

# Default fallback if project-config didn't export this
BUILD_PROJECT_ROOT="${BUILD_PROJECT_ROOT:-$PROJECT_ROOT}"

# Load .env — prefer project root .env
ENV_FILE="$PROJECT_ROOT/.env"
if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck source=/dev/null
    source "$ENV_FILE"
    set +a
fi

FIREBASE_TESTER_EMAILS="${FIREBASE_TESTER_EMAILS:-}"
BUILD_MODE="${BUILD_MODE:-release}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_DISTRIBUTE="${SKIP_DISTRIBUTE:-false}"
VERBOSE="${VERBOSE:-false}"

# ── Helpers ─────────────────────────────────────────────────────────────
_log()   { echo "[distribute] $*"; }
_err()   { echo "[distribute] ERROR: $*" >&2; }
_warn()  { echo "[distribute] WARN: $*" >&2; }
_info()  { echo "[distribute] INFO: $*"; }
_debug() { [[ "$VERBOSE" == "true" ]] && echo "[distribute] DEBUG: $*"; }

_require_command() {
    if ! command -v "$1" &>/dev/null; then
        _err "Required command '$1' not found on PATH."
        _err "Install: $2"
        exit 1
    fi
}

# ── Pre-flight checks ──────────────────────────────────────────────────
_preflight() {
    _log "Running pre-flight checks..."

    # Firebase CLI
    _require_command "firebase" "npm install -g firebase-tools || curl -sL firebase.tools | bash"

    # Go (for API build)
    if [[ "$SKIP_BUILD" != "true" ]]; then
        _require_command "go" "https://go.dev/dl/"
        _require_command "git" "brew install git || apt install git"
    fi

    # Java / Gradle (for Android TV build) — check when androidtv dir exists
    if [[ "$SKIP_BUILD" != "true" && -d "$BUILD_PROJECT_ROOT/catalogizer-androidtv" ]]; then
        if ! command -v java &>/dev/null; then
            _err "Java not found. Required for Android TV Gradle build."
            _err "Install: brew install openjdk@17 || apt install openjdk-17-jdk"
            exit 1
        fi
        if [[ ! -f "$BUILD_PROJECT_ROOT/catalogizer-androidtv/gradlew" ]]; then
            _err "Gradle wrapper not found at catalogizer-androidtv/gradlew"
            exit 1
        fi
    fi

    # Firebase login check
    if ! firebase projects:list --json 2>/dev/null | grep -q '"status"'; then
        _err "Firebase CLI not logged in. Run: firebase login"
        exit 1
    fi

    # Firefox project check (heuristic — tries to read .firestorerc or fallback)
    local fb_project
    fb_project="$(firebase projects:list --json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['projects'][0]['projectId'] if d.get('projects') else '')" 2>/dev/null || true)"
    if [[ -z "$fb_project" ]]; then
        _err "No Firebase project found. Create one at https://console.firebase.google.com"
        _err "Then run: firebase use --add <project-id>"
        exit 1
    fi
    _info "Firebase project detected: $fb_project"

    # Tester emails
    if [[ -z "$FIREBASE_TESTER_EMAILS" ]]; then
        _err "FIREBASE_TESTER_EMAILS is not set in .env"
        _err "Add: FIREBASE_TESTER_EMAILS=user1@example.com,user2@example.com"
        _err "Or run: scripts/firebase_setup_env.sh"
        exit 1
    fi

    _log "Pre-flight checks passed."
}

# ── Build catalog-api binary ────────────────────────────────────────────
_build_api() {
    local api_dir="$BUILD_PROJECT_ROOT/catalog-api"

    if [[ ! -d "$api_dir" ]]; then
        _info "catalog-api directory not found at $api_dir — skipping API build."
        return 0
    fi

    _log "Building catalog-api (GOMAXPROCS=3)..."

    local output_dir="$BUILD_PROJECT_ROOT/build/catalog-api"
    mkdir -p "$output_dir"
    local binary="catalog-api"
    local output_path="$output_dir/$binary"

    pushd "$api_dir" >/dev/null

    GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1 CGO_CPPFLAGS=-DHAVE_USLEEP=1 \
        go build -ldflags="-s -w" -o "$output_path" . 2>&1 | while IFS= read -r line; do
        _debug "go: $line"
    done

    local go_exit="${PIPESTATUS[0]}"
    popd >/dev/null

    if [[ "$go_exit" -ne 0 ]]; then
        _err "catalog-api build failed (exit $go_exit)"
        return 1
    fi

    # Verify the binary
    if [[ ! -f "$output_path" ]]; then
        _err "Binary not found at $output_path"
        return 1
    fi

    local bin_size
    bin_size="$(stat -f%z "$output_path" 2>/dev/null || stat -c%s "$output_path" 2>/dev/null || echo "unknown")"
    _log "catalog-api built: $output_path ($bin_size bytes)"
}

# ── Build catalogizer-androidtv APK ────────────────────────────────────
_build_androidtv() {
    local androidtv_dir="$BUILD_PROJECT_ROOT/catalogizer-androidtv"

    if [[ ! -d "$androidtv_dir" ]]; then
        _info "catalogizer-androidtv directory not found — skipping TV build."
        return 0
    fi

    _log "Building catalogizer-androidtv APK (assembleRelease)..."

    pushd "$androidtv_dir" >/dev/null

    OUTPUT_DIR="$androidtv_dir/build/outputs/apk/release/"
    if ./gradlew assembleRelease 2>&1 | while IFS= read -r line; do
        _debug "gradle: $line"
    done; then
        _log "APK build succeeded."
    else
        _err "APK build failed."
        popd >/dev/null
        return 1
    fi

    popd >/dev/null

    local apk
    apk="$(find "$androidtv_dir" -path '*/release/*.apk' -type f 2>/dev/null | head -1 || true)"
    if [[ -n "$apk" ]]; then
        local apk_size
        apk_size="$(stat -f%z "$apk" 2>/dev/null || stat -c%s "$apk" 2>/dev/null || echo "unknown")"
        _log "APK: $apk ($apk_size bytes)"
    else
        _info "No APK found under release/ output — check gradle build."
    fi
}

# ── Add testers via Firebase App Distribution ───────────────────────────
_add_testers() {
    local emails=()
    IFS=',' read -ra emails <<< "$FIREBASE_TESTER_EMAILS"

    _log "Adding ${#emails[@]} tester(s) to Firebase App Distribution..."

    for email in "${emails[@]}"; do
        email="$(echo "$email" | tr -d '[:space:]')"
        if [[ -z "$email" ]]; then
            continue
        fi
        _log "  Adding tester: $email"
        if firebase appdistribution:testers:add "$email" 2>&1 | while IFS= read -r line; do
            _debug "firebase: $line"
        done; then
            _info "  Tester added: $email"
        else
            _warn "  Could not add tester $email (may already exist)"
        fi
    done
}

# ── Distribute builds to Firebase App Distribution ──────────────────────
_distribute() {
    _log "Starting Firebase App Distribution..."

    local release_notes="Build $(date -u '+%Y-%m-%dT%H:%M:%SZ')"

    # Distribute API binary
    local api_binary="$BUILD_PROJECT_ROOT/build/catalog-api/catalog-api"
    if [[ -f "$api_binary" ]]; then
        _log "Distributing catalog-api binary..."
        if firebase appdistribution:distribute "$api_binary" \
            --app "$FIREBASE_API_APP_ID" \
            --release-notes "$release_notes" \
            --testers-file /dev/stdin <<< "$FIREBASE_TESTER_EMAILS" 2>&1; then
            _log "catalog-api distributed."
        else
            _err "Failed to distribute catalog-api"
            return 1
        fi
    else
        _info "No catalog-api binary found at $api_binary — skipping API distribution."
    fi

    # Distribute Android TV APK
    local apk
    apk="$(find "$BUILD_PROJECT_ROOT/catalogizer-androidtv" -path '*/release/*.apk' -type f 2>/dev/null | head -1 || true)"
    if [[ -n "$apk" ]]; then
        _log "Distributing catalogizer-androidtv APK..."
        if firebase appdistribution:distribute "$apk" \
            --app "$FIREBASE_ANDROIDTV_APP_ID" \
            --release-notes "$release_notes" \
            --testers-file /dev/stdin <<< "$FIREBASE_TESTER_EMAILS" 2>&1; then
            _log "APK distributed."
        else
            _err "Failed to distribute APK"
            return 1
        fi
    else
        _info "No APK found — skipping APK distribution."
    fi

    _log "Firebase App Distribution complete."
}

# ── Main ────────────────────────────────────────────────────────────────
main() {
    _log "=== Catalogizer Firebase Distribution ==="
    _log "Mode: $BUILD_MODE"
    _log "Skip build: $SKIP_BUILD"
    _log "Skip distribute: $SKIP_DISTRIBUTE"
    _log ""

    _preflight

    if [[ "$SKIP_BUILD" != "true" ]]; then
        _build_api
        _build_androidtv
    else
        _info "Build skipped (SKIP_BUILD=true). Using pre-existing artifacts."
    fi

    # Add testers
    _add_testers

    if [[ "$SKIP_DISTRIBUTE" != "true" ]]; then
        _distribute
    else
        _info "Distribution skipped (SKIP_DISTRIBUTE=true)."
    fi

    _log ""
    _log "=== Distribution complete ==="
}

main "$@"
