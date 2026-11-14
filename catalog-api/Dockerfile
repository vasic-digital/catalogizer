# Production Dockerfile for Catalogizer API (slim ffmpeg: see docs/deployment/FFMPEG_MINIMAL.md)
# Multi-stage build for optimized image size
#
# Build from repo root:
#   podman build --network host -f catalog-api/Dockerfile -t catalogizer-api .

# Stage 1: Build (Debian-based for glibc; this was chosen for go-fitz/MuPDF, which was removed in D3 -
# UNCONFIRMED whether the CGO build stage is still required for other dependencies; not changed without a build proof)
# Go floor: go.mod `go 1.25.7` / images.lock.yaml IMG-GO (1.25); override with --build-arg GO_IMAGE=docker.io/library/golang:1.26 (also verified)
ARG GO_IMAGE=docker.io/library/golang:1.25
ARG FFMPEG_VERSION=7.1.5
ARG FFMPEG_SHA256=de668509caf9e35e3cd162473441fdb29538c6d96ed080292b3cf9e6fc5d558f
FROM ${GO_IMAGE} AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git make gcc libc6-dev && rm -rf /var/lib/apt/lists/*

# Prevent Go from auto-downloading newer toolchain
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=1
# 2026-11-02 decision: SQLite busy handler sleeps in ms (go-sqlcipher); CPPFLAGS keeps go's default -O2 -g CFLAGS
ENV CGO_CPPFLAGS=-DHAVE_USLEEP=1

WORKDIR /build

# Copy submodules (required by go.mod replace directives)
COPY submodules/assets/ /build/submodules/assets/
COPY submodules/auth/ /build/submodules/auth/
COPY submodules/cache/ /build/submodules/cache/
COPY submodules/challenges/ /build/submodules/challenges/
COPY submodules/concurrency/ /build/submodules/concurrency/
COPY submodules/config/ /build/submodules/config/
COPY submodules/containers/ /build/submodules/containers/
COPY submodules/database/ /build/submodules/database/
COPY submodules/discovery/ /build/submodules/discovery/
COPY submodules/entities/ /build/submodules/entities/
COPY submodules/event_bus/ /build/submodules/event_bus/
COPY submodules/filesystem/ /build/submodules/filesystem/
COPY submodules/lazy/ /build/submodules/lazy/
COPY submodules/media/ /build/submodules/media/
COPY submodules/memory/ /build/submodules/memory/
COPY submodules/middleware/ /build/submodules/middleware/
COPY submodules/observability/ /build/submodules/observability/
COPY submodules/rate_limiter/ /build/submodules/rate_limiter/
COPY submodules/recovery/ /build/submodules/recovery/
COPY submodules/security/ /build/submodules/security/
COPY submodules/storage/ /build/submodules/storage/
COPY submodules/streaming/ /build/submodules/streaming/
COPY submodules/watcher/ /build/submodules/watcher/

# Copy catalog-api source
COPY catalog-api/go.mod catalog-api/go.sum /build/catalog-api/
WORKDIR /build/catalog-api
RUN go mod download

COPY catalog-api/ /build/catalog-api/

# Build the application
RUN go build -a -ldflags="-w -s" -o catalogizer-api .

# ---------------------------------------------------------------------------------------------------
# CAT-IMG-SLIM-1: minimal ffmpeg/ffprobe, built from the pinned upstream release (the distro ffmpeg links
# libavfilter -> libplacebo/vulkan -> Mesa gallium + libLLVM + libz3 + flite voices: ~400 MB of the old image).
# Full capability list / exclusions / rebuild command: docs/deployment/FFMPEG_MINIMAL.md
# --disable-autodetect keeps GPU/X11/network/audio-device/etc. libraries out; every native decoder, demuxer,
# muxer, native encoder, parser and filter stays (the code relies on silencedetect, scale, lavfi test sources
# in tests, all demuxers of the -format_whitelist). Only the external libraries listed below are enabled.
# ---------------------------------------------------------------------------------------------------
FROM docker.io/library/debian:trixie-slim AS ffmpeg-build
ARG FFMPEG_VERSION
ARG FFMPEG_SHA256
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl xz-utils build-essential nasm pkg-config \
    libx264-dev libvpx-dev libmp3lame-dev libopus-dev libvorbis-dev libxvidcore-dev libdav1d-dev zlib1g-dev libbz2-dev liblzma-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN curl -fsSL -o ffmpeg.tar.xz "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz" \
    && echo "${FFMPEG_SHA256}  ffmpeg.tar.xz" | sha256sum -c - \
    && tar -xJf ffmpeg.tar.xz --strip-components=1 && rm ffmpeg.tar.xz
RUN ./configure --prefix=/opt/ffmpeg --enable-gpl --disable-autodetect --disable-doc --disable-debug \
        --disable-ffplay --enable-pic \
        --enable-libx264 --enable-libvpx --enable-libmp3lame --enable-libopus --enable-libvorbis \
        --enable-libxvid --enable-libdav1d \
        --enable-zlib --enable-bzlib --enable-lzma --enable-iconv \
        --enable-zlib --enable-bzlib --enable-lzma --enable-iconv \
    && make -j"$(n=$(nproc); [ "$n" -gt 8 ] && n=8; echo "$n")" && make install && strip /opt/ffmpeg/bin/ffmpeg /opt/ffmpeg/bin/ffprobe

# Stage 2: Runtime (must match builder's glibc version — both use trixie)
FROM docker.io/library/debian:trixie-slim

# Install runtime dependencies. poppler-utils (pdftotext, pdfinfo) is the primary PDF text engine
# (CAT-002); util-linux provides prlimit, which caps every PDF parser child. Without prlimit the
# service refuses to parse PDFs. ffmpeg (audio/video) and imagemagick (`convert`: images; PDF
# pages are rasterised by poppler pdftoppm, ghostscript is deliberately NOT installed, so the ImageMagick PDF fallback
# reports a specific error and /health/deep does not claim convert_pdf) are the converter tools the conversion worker execs; without them every conversion
# job fails with "converter tool ... is not installed" (GET /health/deep -> converters shows what is present).
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata wget poppler-utils util-linux imagemagick \
        libx264-164 libvpx9 libmp3lame0 libopus0 libvorbis0a libvorbisenc2 libxvidcore4 libdav1d7 \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*

COPY --from=ffmpeg-build /opt/ffmpeg/bin/ffmpeg /opt/ffmpeg/bin/ffprobe /usr/local/bin/

WORKDIR /app

# Dedicated non-root user first, so COPY --chown writes each file once with its final owner (a
# trailing `chown -R /app` would duplicate the 87 MB binary into a second layer: CAT-IMG-SLIM-1)
RUN groupadd --system --gid 1001 appgroup && \
    useradd --system --uid 1001 --gid appgroup --no-create-home appuser

# Copy binary from builder
COPY --from=builder --chown=appuser:appgroup /build/catalog-api/catalogizer-api /app/catalogizer-api

# Copy runtime configuration files
# Note: config.json is NOT copied — the app auto-creates defaults and reads
# auth settings (JWT_SECRET, ADMIN_USERNAME, ADMIN_PASSWORD) from env vars.
COPY --from=builder --chown=appuser:appgroup /build/catalog-api/migrations /app/migrations
COPY --from=builder --chown=appuser:appgroup /build/catalog-api/challenges/config /app/challenges/config

# Create necessary directories with proper permissions (small dirs only; no recursive chown of the binary)
RUN mkdir -p /app/data /app/logs /app/temp /app/keys && \
    chmod 700 /app/keys && \
    chown appuser:appgroup /app /app/data /app/logs /app/temp /app/keys /app/challenges

# Bind address/port for containers: the app defaults to localhost:28080 (unreachable from outside the container, and not the
# EXPOSEd/health-checked 8080); main.go honours HOST and SERVER_PORT as overrides.
ENV HOST=0.0.0.0 SERVER_PORT=8080
# CAT-021: release mode by default in every image built from this Dockerfile (disables
# CATALOGIZER_ALLOW_INSECURE_DEFAULTS); dev uses Dockerfile.dev.
ENV GIN_MODE=release

# Expose API port
# CAT-001: the SQLCipher key lives in its own 0700 volume, separate from the
# database volume, so it survives container recreation and is not captured by
# a plain backup of /app/data. Mount a named volume here in production.
ENV DATABASE_ENCRYPTION_KEY_FILE=/app/keys/db.key
VOLUME ["/app/keys"]

EXPOSE 8080

# Health check
# GET, not --spider: /health is GET-only (a HEAD request answers 404)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 -O /dev/null http://127.0.0.1:8080/health || exit 1

# Switch to non-root user
USER appuser

# Run the application
ENTRYPOINT ["/app/catalogizer-api"]
