# Production Dockerfile for Catalogizer API
# Multi-stage build for optimized image size
#
# Build from repo root:
#   podman build --network host -f catalog-api/Dockerfile -t catalogizer-api .

# Stage 1: Build (Debian-based for glibc — required by go-fitz/MuPDF)
# Go floor: go.mod `go 1.25.7` / images.lock.yaml IMG-GO (1.25); override with --build-arg GO_IMAGE=docker.io/library/golang:1.26 (also verified)
ARG GO_IMAGE=docker.io/library/golang:1.25
FROM ${GO_IMAGE} AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git make gcc libc6-dev && rm -rf /var/lib/apt/lists/*

# Prevent Go from auto-downloading newer toolchain
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=1

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

# Stage 2: Runtime (must match builder's glibc version — both use trixie)
FROM docker.io/library/debian:trixie-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata wget && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/catalog-api/catalogizer-api /app/catalogizer-api

# Copy runtime configuration files
# Note: config.json is NOT copied — the app auto-creates defaults and reads
# auth settings (JWT_SECRET, ADMIN_USERNAME, ADMIN_PASSWORD) from env vars.
COPY --from=builder /build/catalog-api/migrations /app/migrations
COPY --from=builder /build/catalog-api/challenges/config /app/challenges/config

# Create a dedicated non-root user
RUN groupadd --system --gid 1001 appgroup && \
    useradd --system --uid 1001 --gid appgroup --no-create-home appuser

# Create necessary directories with proper permissions
RUN mkdir -p /app/data /app/logs /app/temp /app/keys && \
    chmod 700 /app/keys && \
    chown -R appuser:appgroup /app

# Bind address/port for containers: the app defaults to localhost:28080 (unreachable from outside the container, and not the
# EXPOSEd/health-checked 8080); main.go honours HOST and SERVER_PORT as overrides.
ENV HOST=0.0.0.0 SERVER_PORT=8080

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
