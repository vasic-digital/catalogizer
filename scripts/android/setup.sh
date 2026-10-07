#!/bin/bash

# Catalogizer Android Development Environment Setup Script
# Sets up local JDK with jmods and Android SDK within the project directory
# Downloaded components are stored in tools/ (ignored by git)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$PROJECT_ROOT")"  # scripts/android -> scripts -> root

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_header() { echo -e "${PURPLE}[ANDROID SETUP]${NC} $1"; }

# Default paths
TOOLS_DIR="$PROJECT_ROOT/tools"
JDK_DIR="$TOOLS_DIR/jdk"
ANDROID_SDK_DIR="$TOOLS_DIR/android-sdk"
DOWNLOADS_DIR="$TOOLS_DIR/downloads"

# JDK configuration
JDK_VERSION="21.0.10+7"
JDK_URL="https://github.com/adoptium/temurin21-binaries/releases/download/jdk-${JDK_VERSION//+/%2B}/OpenJDK21U-jdk_x64_linux_hotspot_${JDK_VERSION//+/_}.tar.gz"
JDK_ARCHIVE_NAME="openjdk-21.tar.gz"
JDK_EXTRACT_DIR="$JDK_DIR/temurin-${JDK_VERSION}"

# Android SDK configuration
ANDROID_CMDLINE_TOOLS_URL="https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip"
ANDROID_CMDLINE_TOOLS_ARCHIVE="commandlinetools-linux.zip"
ANDROID_CMDLINE_TOOLS_DIR="$ANDROID_SDK_DIR/cmdline-tools/latest"

# Required SDK packages (matching docker/Dockerfile.android2)
ANDROID_SDK_PACKAGES=(
    "platform-tools"
    "build-tools;34.0.0"
    "platforms;android-33"
    "sources;android-33"
    "platforms;android-34"
    "sources;android-34"
    "extras;android;m2repository"
    "extras;google;m2repository"
)

# Gradle properties files
ROOT_GRADLE_PROPERTIES="$PROJECT_ROOT/gradle.properties"
ANDROID_GRADLE_PROPERTIES="$PROJECT_ROOT/catalogizer-android/gradle.properties"

# Ensure required commands exist
check_commands() {
    local commands=("curl" "unzip" "tar")
    local missing=()
    for cmd in "${commands[@]}"; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required commands: ${missing[*]}"
        log_info "Please install them with your package manager, e.g.:"
        log_info "  sudo apt-get install curl unzip tar"
        exit 1
    fi
}

# Create directory structure
create_dirs() {
    log_info "Creating directory structure..."
    mkdir -p "$TOOLS_DIR"
    mkdir -p "$JDK_DIR"
    mkdir -p "$ANDROID_SDK_DIR"
    mkdir -p "$DOWNLOADS_DIR"
    log_success "Directories created"
}

# Download file with retries
download_with_retry() {
    local url="$1"
    local output="$2"
    local max_retries=3
    local retry_count=0
    local wait_time=10
    
    while [[ $retry_count -lt $max_retries ]]; do
        if curl --retry 5 --retry-delay 5 --retry-all-errors -fL "$url" -o "$output"; then
            log_success "Downloaded: $(basename "$output")"
            return 0
        else
            retry_count=$((retry_count + 1))
            log_warning "Download attempt $retry_count failed, retrying in ${wait_time}s..."
            sleep "$wait_time"
        fi
    done
    log_error "Failed to download $(basename "$output") after $max_retries attempts"
    return 1
}

# Install JDK with jmods
install_jdk() {
    log_header "Installing JDK ${JDK_VERSION} with jmods..."
    
    # Check if already installed
    if [[ -d "$JDK_EXTRACT_DIR" && -d "$JDK_EXTRACT_DIR/jmods" ]]; then
        log_success "JDK already installed at $JDK_EXTRACT_DIR"
        return 0
    fi
    
    # Download JDK
    log_info "Downloading Temurin JDK ${JDK_VERSION}..."
    local jdk_archive="$DOWNLOADS_DIR/$JDK_ARCHIVE_NAME"
    download_with_retry "$JDK_URL" "$jdk_archive"
    
    # Extract JDK
    log_info "Extracting JDK..."
    mkdir -p "$JDK_EXTRACT_DIR"
    tar -xzf "$jdk_archive" -C "$JDK_EXTRACT_DIR" --strip-components=1
    rm -f "$jdk_archive"
    
    # Verify jmods directory exists
    if [[ ! -d "$JDK_EXTRACT_DIR/jmods" ]]; then
        log_error "JDK installation missing jmods directory - may cause Android Gradle Plugin issues"
        log_info "Checking for jmods in subdirectories..."
        find "$JDK_EXTRACT_DIR" -name "jmods" -type d | head -5
        return 1
    fi
    
    # Create symlink
    ln -sfn "$JDK_EXTRACT_DIR" "$JDK_DIR/current"
    
    log_success "JDK installed to $JDK_EXTRACT_DIR"
    log_info "Java version:"
    "$JDK_DIR/current/bin/java" -version 2>&1 | head -3
}

# Install Android SDK
install_android_sdk() {
    log_header "Installing Android SDK..."
    
    # Check if cmdline-tools already installed
    if [[ -d "$ANDROID_CMDLINE_TOOLS_DIR" && -f "$ANDROID_CMDLINE_TOOLS_DIR/bin/sdkmanager" ]]; then
        log_success "Android SDK command-line tools already installed"
    else
        # Download command-line tools
        log_info "Downloading Android command-line tools..."
        local tools_archive="$DOWNLOADS_DIR/$ANDROID_CMDLINE_TOOLS_ARCHIVE"
        download_with_retry "$ANDROID_CMDLINE_TOOLS_URL" "$tools_archive"
        
        # Extract command-line tools
        log_info "Extracting command-line tools..."
        mkdir -p "$ANDROID_SDK_DIR/cmdline-tools"
        unzip -q "$tools_archive" -d "$ANDROID_SDK_DIR/cmdline-tools"
        mv "$ANDROID_SDK_DIR/cmdline-tools/cmdline-tools" "$ANDROID_CMDLINE_TOOLS_DIR"
        rm -f "$tools_archive"
        log_success "Android command-line tools installed"
    fi
    
    # Add Android SDK tools to PATH for this script
    export PATH="$ANDROID_CMDLINE_TOOLS_DIR/bin:$PATH"
    export ANDROID_HOME="$ANDROID_SDK_DIR"
    export ANDROID_SDK_ROOT="$ANDROID_SDK_DIR"
    
    # Accept licenses
    log_info "Accepting Android SDK licenses..."
    # No `|| true` (owner decision 2026-10-07): the pipeline status is sdkmanager's (no pipefail here), so a licence that cannot be accepted stops the setup.
    if ! yes | sdkmanager --licenses >/dev/null; then
        log_error "Could not accept the Android SDK licenses (sdkmanager --licenses failed)"
        return 1
    fi
    
    # Install SDK packages with retries
    log_info "Installing Android SDK packages..."
    for package in "${ANDROID_SDK_PACKAGES[@]}"; do
        log_info "  Installing $package..."
        local attempt=1
        local max_attempts=3
        while [[ $attempt -le $max_attempts ]]; do
            if sdkmanager "$package" --sdk_root="$ANDROID_SDK_DIR" >/dev/null 2>&1; then
                log_success "    Installed $package"
                break
            else
                log_warning "    Attempt $attempt failed for $package"
                attempt=$((attempt + 1))
                sleep 5
                if [[ $attempt -gt $max_attempts ]]; then
                    log_error "    Failed to install $package after $max_attempts attempts"
                fi
            fi
        done
    done
    
    log_success "Android SDK installation complete"
}

# Update Gradle properties with local JDK path
update_gradle_properties() {
    log_header "Updating Gradle properties..."
    
    local java_home="$JDK_DIR/current"
    local java_home_abs="$(cd "$java_home" && pwd)"
    
    # Update root gradle.properties
    if [[ -f "$ROOT_GRADLE_PROPERTIES" ]]; then
        log_info "Updating $ROOT_GRADLE_PROPERTIES"
        # Remove existing java.home and jdkHome lines
        grep -v "org.gradle.java.home" "$ROOT_GRADLE_PROPERTIES" | \
        grep -v "android.jdkHome" > "${ROOT_GRADLE_PROPERTIES}.tmp" || true
        mv "${ROOT_GRADLE_PROPERTIES}.tmp" "$ROOT_GRADLE_PROPERTIES"
        
        # Add new lines
        echo "org.gradle.java.home=$java_home_abs" >> "$ROOT_GRADLE_PROPERTIES"
        echo "android.jdkHome=$java_home_abs" >> "$ROOT_GRADLE_PROPERTIES"
        log_success "Root Gradle properties updated"
    else
        log_warning "Root gradle.properties not found at $ROOT_GRADLE_PROPERTIES"
    fi
    
    # Update Android module gradle.properties
    if [[ -f "$ANDROID_GRADLE_PROPERTIES" ]]; then
        log_info "Updating $ANDROID_GRADLE_PROPERTIES"
        # Remove existing java.home and jdkHome lines
        grep -v "org.gradle.java.home" "$ANDROID_GRADLE_PROPERTIES" | \
        grep -v "android.jdkHome" > "${ANDROID_GRADLE_PROPERTIES}.tmp" || true
        mv "${ANDROID_GRADLE_PROPERTIES}.tmp" "$ANDROID_GRADLE_PROPERTIES"
        
        # Add new lines
        echo "org.gradle.java.home=$java_home_abs" >> "$ANDROID_GRADLE_PROPERTIES"
        echo "android.jdkHome=$java_home_abs" >> "$ANDROID_GRADLE_PROPERTIES"
        log_success "Android Gradle properties updated"
    else
        log_warning "Android gradle.properties not found at $ANDROID_GRADLE_PROPERTIES"
    fi
    
    # Create local.properties with SDK directory
    local local_props="$PROJECT_ROOT/catalogizer-android/local.properties"
    echo "sdk.dir=$ANDROID_SDK_DIR" > "$local_props"
    echo "ndk.dir=" >> "$local_props"
    log_success "Local properties created"
}

# Create environment setup script
create_env_script() {
    log_header "Creating environment setup script..."
    
    local env_script="$TOOLS_DIR/android-env.sh"
    cat > "$env_script" << EOF
#!/bin/bash
# Android development environment variables for Catalogizer
# Source this file to set up your environment: source tools/android-env.sh

export JAVA_HOME="$JDK_DIR/current"
export ANDROID_HOME="$ANDROID_SDK_DIR"
export ANDROID_SDK_ROOT="$ANDROID_SDK_DIR"
export PATH="\$JAVA_HOME/bin:\$ANDROID_HOME/cmdline-tools/latest/bin:\$ANDROID_HOME/platform-tools:\$PATH"

echo "Android development environment configured:"
echo "  JAVA_HOME=\$JAVA_HOME"
echo "  ANDROID_HOME=\$ANDROID_HOME"
EOF
    
    chmod +x "$env_script"
    log_success "Environment script created at $env_script"
    log_info "To activate the environment, run: source $env_script"
}

# Verify installation
verify_installation() {
    log_header "Verifying installation..."
    
    # Check JDK
    if [[ -f "$JDK_DIR/current/bin/java" ]]; then
        log_success "JDK verified: $("$JDK_DIR/current/bin/java" -version 2>&1 | head -1)"
    else
        log_error "JDK verification failed"
        return 1
    fi
    
    # Check Android SDK
    if [[ -f "$ANDROID_CMDLINE_TOOLS_DIR/bin/sdkmanager" ]]; then
        log_success "Android SDK verified"
    else
        log_error "Android SDK verification failed"
        return 1
    fi
    
    # Check Gradle properties
    if grep -q "org.gradle.java.home" "$ROOT_GRADLE_PROPERTIES" 2>/dev/null || \
       grep -q "android.jdkHome" "$ROOT_GRADLE_PROPERTIES" 2>/dev/null; then
        log_success "Gradle properties updated"
    else
        log_warning "Gradle properties may not be correctly updated"
    fi
    
    log_success "Installation verification complete"
}

# Main execution
main() {
    log_header "Starting Android development environment setup"
    log_info "Project root: $PROJECT_ROOT"
    log_info "Tools directory: $TOOLS_DIR"
    log_info "JDK version: $JDK_VERSION"
    log_info "Android SDK packages: ${ANDROID_SDK_PACKAGES[*]}"
    echo ""
    
    check_commands
    create_dirs
    install_jdk
    install_android_sdk
    update_gradle_properties
    create_env_script
    verify_installation
    
    log_header "Setup complete!"
    echo ""
    log_info "Next steps:"
    log_info "1. Source the environment script: source tools/android-env.sh"
    log_info "2. Build the Android project: cd catalogizer-android && ./gradlew assembleDebug"
    log_info "3. Run tests: cd catalogizer-android && ./gradlew test"
    echo ""
    log_info "Note: Downloaded components are in tools/ (ignored by git)"
    log_info "To clean up, delete the tools/ directory"
}

# Run main if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi