# Container Setup for User Flow Testing

## Overview

User flow testing runs in a containerized environment using Podman (not Docker). The test container stack is defined in `docker-compose.test.yml` at the project root and provides three core services: the Go API server, the React dev server, and a Playwright browser container.

## Prerequisites

- Podman 5.x or later
- podman-compose 1.5.0 or later
- At least 4 CPU cores and 8 GB RAM available for containers

Verify your installation:

```bash
podman --version          # >= 5.0
podman-compose --version  # >= 1.5.0
```

## Starting the Test Stack

```bash
# Validate the compose file
podman-compose -f docker-compose.test.yml config --quiet

# Start all services
podman-compose -f docker-compose.test.yml up -d

# Verify services are running
podman-compose -f docker-compose.test.yml ps
```

## Service Details

### catalog-api

The Go API server built from `./catalog-api`.

| Property | Value |
|----------|-------|
| Build context | `./catalog-api` |
| Port | 8080 |
| CPU limit | 2 |
| Memory limit | 4 GB |
| Network | host |
| Health check | `GET /health` |

Environment variables:

```
GIN_MODE=release
DB_TYPE=sqlite
JWT_SECRET=test-secret-key
ADMIN_PASSWORD=<generate-your-own, e.g. openssl rand -base64 24>
GOTOOLCHAIN=local
```

### catalog-web

The React dev server built from `./catalog-web`.

| Property | Value |
|----------|-------|
| Build context | `./catalog-web` |
| Port | 3000 |
| CPU limit | 1 |
| Memory limit | 2 GB |
| Network | host |
| Depends on | catalog-api |

The dev server reads `../catalog-api/.service-port` to discover the API port for proxy configuration.

### playwright

The Playwright browser container for web automation.

| Property | Value |
|----------|-------|
| Image | `mcr.microsoft.com/playwright:v1.40.0-jammy` |
| Port | 9222 (CDP) |
| CPU limit | 1 |
| Memory limit | 2 GB |
| Network | host |

The web challenges connect to the Playwright container via Chrome DevTools Protocol (CDP) at `ws://localhost:9222`.

## Resource Limits

The total resource budget is 4 CPUs and 8 GB RAM, respecting the project's 30-40% host resource policy.

| Service | CPUs | Memory |
|---------|------|--------|
| catalog-api | 2 | 4 GB |
| catalog-web | 1 | 2 GB |
| playwright | 1 | 2 GB |
| **Total** | **4** | **8 GB** |

## Platform Groups

Not all services need to run simultaneously. The test runner starts and stops containers per platform group:

### API group

Only `catalog-api` is needed.

```bash
podman-compose -f docker-compose.test.yml up -d catalog-api
curl -X POST http://localhost:8080/api/v1/challenges/run/category/userflow-api
podman-compose -f docker-compose.test.yml stop catalog-api
```

### Web group

Requires `catalog-api`, `catalog-web`, and `playwright`.

```bash
podman-compose -f docker-compose.test.yml up -d
curl -X POST http://localhost:8080/api/v1/challenges/run/category/userflow-web
podman-compose -f docker-compose.test.yml down
```

### Desktop group

Requires `catalog-api` and the desktop binary built locally. Desktop challenges use Tauri WebDriver, which runs the application binary directly (not in a container).

```bash
podman-compose -f docker-compose.test.yml up -d catalog-api
cd catalogizer-desktop && npm run tauri:build
DESKTOP_BINARY_PATH=./src-tauri/target/debug/catalogizer-desktop \
  curl -X POST http://localhost:8080/api/v1/challenges/run/category/userflow-desktop
```

### Wizard group

Similar to desktop -- requires only the wizard binary.

```bash
cd installer-wizard && npm run tauri:build
WIZARD_BINARY_PATH=./src-tauri/target/debug/installer-wizard \
  curl -X POST http://localhost:8080/api/v1/challenges/run/category/userflow-wizard
```

### Android and Android TV groups

Require `catalog-api` and an Android emulator or physical device connected via ADB.

```bash
podman-compose -f docker-compose.test.yml up -d catalog-api

# Ensure emulator is running and ADB is connected
adb devices

# Android phone challenges
ANDROID_DEVICE_SERIAL=emulator-5554 \
  curl -X POST http://localhost:8080/api/v1/challenges/run/category/userflow-android

# Android TV challenges
ANDROIDTV_DEVICE_SERIAL=emulator-5556 \
  curl -X POST http://localhost:8080/api/v1/challenges/run/category/userflow-androidtv
```

## Network Configuration

All services use `network_mode: host` to avoid container networking issues. This means:

- Services bind directly to the host's network interfaces
- No port mapping is needed (ports are used directly)
- The Playwright container can reach the web server at `localhost:3000`
- The API server can be reached at `localhost:8080`

This matches the project convention -- `podman run --network host` is required for reliable SSL and inter-service communication.

## Build Notes

When building container images:

```bash
# Must use --network host for reliable builds
podman build --network host -t catalogizer-api-test ./catalog-api
podman build --network host -t catalogizer-web-test ./catalog-web
```

Critical container build requirements:

- Set `GOTOOLCHAIN=local` to prevent Go from auto-downloading newer toolchain versions
- Use fully qualified image names (e.g., `docker.io/library/node:18`) since Podman short names fail without TTY
- Set `APPIMAGE_EXTRACT_AND_RUN=1` for Tauri AppImage bundling in containers

## Stopping and Cleaning Up

```bash
# Stop all services
podman-compose -f docker-compose.test.yml down

# Stop and remove volumes
podman-compose -f docker-compose.test.yml down -v

# Clean up dangling images
podman image prune -f
```

## Monitoring

```bash
# Container resource usage
podman stats --no-stream

# Host load average
cat /proc/loadavg

# Container logs
podman-compose -f docker-compose.test.yml logs -f catalog-api
podman-compose -f docker-compose.test.yml logs -f catalog-web
podman-compose -f docker-compose.test.yml logs -f playwright
```
