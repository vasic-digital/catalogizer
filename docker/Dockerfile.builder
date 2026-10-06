# Catalogizer Multi-Toolchain Builder Image
# Provides Go, Node.js, Rust, JDK 17, and Android SDK for building all components
# Usage: Built by docker-compose.build.yml

# Go toolchain stage (docs/16 D-04, docs/21 IC-23): IMG-GO of build/containers/images.lock.yaml (golang 1.25-bookworm by index digest; go.mod requires 1.25.7).
FROM docker.io/library/golang:1.25-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437 AS gotoolchain

# Node.js stage: IMG-NODE of build/containers/images.lock.yaml (node 20-bookworm by index digest). Replaces the NodeSource `curl | bash` of Node 18 (end of life; catalog-web builds on Node 20, docs/16 D-05).
FROM docker.io/library/node:20-bookworm@sha256:8f693eaa7e0a8e71560c9a82b55fd54c2ae920a2ba5d2cde28bac7d1c01c9ba5 AS nodetoolchain

# Base: IMG-UBUNTU-JAMMY of build/containers/images.lock.yaml (ubuntu 22.04 by index digest).
FROM docker.io/library/ubuntu:22.04@sha256:2edbbc5dc405e9612ba3584ce95480277e3eb374407b5505fe26f17df77c7dbc

LABEL maintainer="Catalogizer Team"
LABEL description="Multi-toolchain builder for Catalogizer project"

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC

# ============================================================
# Layer 1: System dependencies
# ============================================================
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    git \
    curl \
    wget \
    pkg-config \
    libssl-dev \
    ca-certificates \
    gnupg \
    unzip \
    zip \
    webkit2gtk-4.1-dev \
    libayatana-appindicator3-dev \
    librsvg2-dev \
    libgtk-3-dev \
    patchelf \
    xvfb \
    postgresql-client \
    redis-tools \
    jq \
    file \
    bc \
    && rm -rf /var/lib/apt/lists/*

# ============================================================
# Layer 1b: VLC Media Player dependencies (for Desktop app)
# ============================================================
RUN apt-get update && apt-get install -y --no-install-recommends \
    libvlc-dev \
    libvlccore-dev \
    vlc \
    && rm -rf /var/lib/apt/lists/*

# ============================================================
# Layer 2: Go toolchain copied from the digest-pinned golang stage above (go.mod minimum 1.25.7)
# ============================================================
COPY --from=gotoolchain /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
ENV GOPATH="/root/go"
ENV CGO_ENABLED=1

RUN go version

# ============================================================
# Layer 3: Node.js 20 LTS copied from the digest-pinned node stage above
# ============================================================
COPY --from=nodetoolchain /usr/local/bin/node /usr/local/bin/node
COPY --from=nodetoolchain /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

RUN node --version && npm --version

# ============================================================
# Layer 4: Rust 1.99.0 (rustup-init 1.29.0, SHA-256 verified) — installed under /opt so the
# toolchain is world-readable. Required when the image is run with
# rootless podman's `--userns=keep-id` flag (the host user's UID is
# mapped through unchanged, so /root/.cargo would be inaccessible).
# ============================================================
ENV RUSTUP_HOME="/opt/rustup"
ENV CARGO_HOME="/opt/cargo"
RUN curl --retry 5 --retry-delay 10 --proto '=https' --tlsv1.2 -fsSL -o /tmp/rustup-init https://static.rust-lang.org/rustup/archive/1.29.0/x86_64-unknown-linux-gnu/rustup-init \
    && echo "4acc9acc76d5079515b46346a485974457b5a79893cfb01112423c89aeb5aa10  /tmp/rustup-init" | sha256sum -c - \
    && chmod +x /tmp/rustup-init \
    && /tmp/rustup-init -y --no-modify-path --default-toolchain 1.99.0 \
    && rm /tmp/rustup-init
ENV PATH="/opt/cargo/bin:${PATH}"

RUN cargo install --locked --version 2.12.1 tauri-cli

RUN chmod -R a+rX /opt/rustup /opt/cargo

RUN rustc --version && cargo --version

# ============================================================
# Layer 5: JDK 21 + Android SDK
# ============================================================
RUN apt-get update && apt-get install -y --no-install-recommends openjdk-21-jdk && rm -rf /var/lib/apt/lists/*
ENV JAVA_HOME="/usr/lib/jvm/java-21-openjdk-amd64"

ENV ANDROID_HOME="/opt/android-sdk"
ENV ANDROID_SDK_ROOT="${ANDROID_HOME}"
ENV PATH="${ANDROID_HOME}/cmdline-tools/latest/bin:${ANDROID_HOME}/platform-tools:${PATH}"

RUN mkdir -p "${ANDROID_HOME}/cmdline-tools" \
    && cd "${ANDROID_HOME}/cmdline-tools" \
    && for i in 1 2 3 4 5; do \
        echo "Attempt $i: Downloading Android cmdline-tools..." && \
        curl --retry 5 --retry-delay 10 --retry-all-errors -fSL \
            "https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip" -o cmdline-tools.zip && \
        break || (echo "Attempt $i failed, waiting 15s..." && sleep 15); \
    done \
    && echo "2d2d50857e4eb553af5a6dc3ad507a17adf43d115264b1afc116f95c92e5e258  cmdline-tools.zip" | sha256sum -c - \
    && unzip -q cmdline-tools.zip \
    && mv cmdline-tools latest \
    && rm cmdline-tools.zip

# The exit status of the pipeline is sdkmanager's: a failure to accept the licenses fails the build (no `|| true`).
RUN yes | sdkmanager --licenses >/dev/null
RUN sdkmanager \
    "platform-tools" \
    "build-tools;34.0.0" \
    "build-tools;35.0.0" \
    "platforms;android-34" \
    "platforms;android-35"

# ============================================================
# Layer 6: Playwright browsers
# ============================================================
RUN npx --yes playwright@1.57.0 install --with-deps chromium

# ============================================================
# Working directory
# ============================================================
WORKDIR /project

ENTRYPOINT ["/project/scripts/build-test-release.sh"]
