# Download and Install Catalogizer

Choose the installation method that best fits your environment.

---

## Containers (Recommended)

The fastest way to get Catalogizer running. Includes the API server, PostgreSQL, and Redis. Both Podman and Docker are supported as container runtimes.

### Requirements
- Podman 5+ with podman-compose, or Docker 20.10+ with Docker Compose v2+
- 4 GB RAM minimum

### Quick Install

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

```bash
git clone <repository-url>
cd Catalogizer
cp .env.example .env
# Edit .env with your POSTGRES_PASSWORD and JWT_SECRET

# Using Podman (preferred when Docker is unavailable)
podman-compose up -d

# Or using Docker
podman compose up -d
```

The API is available at http://localhost:8080.

### With Monitoring

Include Prometheus and Grafana for metrics and dashboards:

```bash
podman-compose --profile monitoring up -d
# or
podman compose --profile monitoring up -d
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 (default login: admin / admin)

### Development Environment

Use the development compose file for local development with hot reloading:

```bash
podman-compose -f docker-compose.dev.yml up
# or
podman compose -f docker-compose.dev.yml up
```

Includes pgAdmin (port 5050) and Redis Commander (port 8081) with the `tools` profile:

```bash
podman-compose -f docker-compose.dev.yml --profile tools up
```

### Podman Notes

When using Podman, keep these points in mind:
- Use fully qualified image names (e.g., `docker.io/library/postgres:15-alpine`) as short names may not resolve without a TTY
- Use `podman build --network host` to avoid SSL issues with package downloads
- Set `GOTOOLCHAIN=local` in build environments to prevent Go from auto-downloading toolchains

See the [Quick Start Tutorial](../tutorials/QUICK_START.md) for a step-by-step walkthrough.

---

## Manual Installation

Build and run each component individually.

### Requirements
- Go 1.21+ (backend)
- Node.js 18+ (frontend and TypeScript projects)
- SQLCipher (encrypted database)
- Git

### Backend (catalog-api)

```bash
cd catalog-api
go mod tidy
cp .env.example .env
# Edit .env with your configuration
go run main.go
```

Build a production binary:

```bash
CGO_ENABLED=1 go build -o catalog-api main.go
```

### Frontend (catalog-web)

```bash
cd catalog-web
npm install
npm run dev      # Development server on port 5173
npm run build    # Production build
```

See the [Installation Guide](../INSTALLATION_GUIDE.md) for complete manual setup instructions.

---

## Desktop Application

Cross-platform desktop app built with Tauri (Rust + React).

### Requirements
- Node.js 18+
- Rust toolchain (install via https://rustup.rs)
- Tauri CLI: `cargo install tauri-cli`
- Platform-specific dependencies (see Tauri prerequisites: https://tauri.app/v1/guides/getting-started/prerequisites)

### Build from Source

```bash
cd catalogizer-desktop
npm install
npm run tauri:build
```

Platform-specific builds:

```bash
# Windows
npm run tauri:build -- --target x86_64-pc-windows-msvc

# macOS
npm run tauri:build -- --target x86_64-apple-darwin

# Linux
npm run tauri:build -- --target x86_64-unknown-linux-gnu
```

Built binaries are output to `src-tauri/target/release/bundle/`.

### Installation Wizard

A guided setup tool for first-time configuration:

```bash
cd installer-wizard
npm install
npm run tauri:build
```

See the [Desktop Guide](../guides/DESKTOP_GUIDE.md) and [Installer Wizard Guide](../guides/INSTALLER_WIZARD_GUIDE.md) for usage instructions.

---

## Android

### Mobile App

Build the Android app from source using Android Studio or the command line.

#### Requirements
- Android Studio (latest stable) or Android SDK command-line tools
- JDK 17+
- Android device or emulator running Android 8.0+ (API 26)

#### Build

```bash
cd catalogizer-android
./gradlew assembleDebug
```

Install on a connected device:

```bash
adb install app/build/outputs/apk/debug/app-debug.apk
```

See the [Mobile Setup Tutorial](../tutorials/MOBILE_SETUP.md) for connection and offline configuration.

### Android TV App

Optimized for TV screens with Leanback UI and remote control navigation.

```bash
cd catalogizer-androidtv
./gradlew assembleDebug
adb install app/build/outputs/apk/debug/app-debug.apk
```

See the [Android TV Guide](../guides/ANDROID_TV_GUIDE.md) for usage instructions.

---

## API Client Library

TypeScript client for integrating Catalogizer into your own applications.

```bash
cd catalogizer-api-client
npm install
npm run build
```

Use locally via npm link:

```bash
npm link
# In your project:
npm link @catalogizer/api-client
```

See the [API Documentation](../api/API_DOCUMENTATION.md) for endpoint details.

---

## System Requirements Summary

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Storage | 20 GB | 50 GB+ |
| Network | 100 Mbps | 1 Gbps |
| OS | Linux, macOS, or Windows | Linux (for Docker deployments) |

### Network Ports

| Port | Service | Required |
|------|---------|----------|
| 8080 | Catalogizer API | Yes |
| 5432 | PostgreSQL | Yes (Docker) |
| 6379 | Redis | Yes (Docker) |
| 9090 | Prometheus | Optional (monitoring) |
| 3001 | Grafana | Optional (monitoring) |
| 80/443 | Nginx reverse proxy | Optional (production) |
| 139, 445 | SMB access | If using SMB sources |
| 21 | FTP access | If using FTP sources |
