# Environment Variables Reference

Complete reference of all environment variables used across Catalogizer components. This document was compiled from source code analysis of all Go, TypeScript, Kotlin, and configuration files.

## Table of Contents

1. [catalog-api (Go Backend)](#catalog-api-go-backend)
2. [catalog-web (React Frontend)](#catalog-web-react-frontend)
3. [Docker Compose (Infrastructure)](#docker-compose-infrastructure)
4. [Android / Android TV (Kotlin)](#android--android-tv-kotlin)
5. [Tauri Desktop / Installer Wizard](#tauri-desktop--installer-wizard)
6. [Monitoring Stack](#monitoring-stack)
7. [Security Scanning](#security-scanning)
8. [Integration Testing](#integration-testing)
9. [Quick Reference by Component](#quick-reference-by-component)

---

## catalog-api (Go Backend)

These variables are read by the Go backend via `os.Getenv()` calls in the source code.

### Core Application

| Variable | Description | Default | Required | Source File |
|----------|-------------|---------|----------|-------------|
| `PORT` | HTTP server listen port | `8080` | No | `main.go` |
| `GIN_MODE` | Gin framework mode (`debug`, `release`, `test`) | `debug` | No | `main.go` |
| `APP_ENV` | Application environment (`development`, `staging`, `production`) | `production` | No | `docker-compose.yml` |
| `API_PORT` | API port (Docker Compose alias for PORT) | `8080` | No | `docker-compose.yml` |
| `LOG_LEVEL` | Logging level (`debug`, `info`, `warn`, `error`) | `info` | No | `docker-compose.yml` |

### Authentication and Security

| Variable | Description | Default | Required | Source File |
|----------|-------------|---------|----------|-------------|
| `JWT_SECRET` | JWT signing secret (minimum 32 characters) | (none) | **Yes** | `main.go`, `config/config.go` |
| `ADMIN_USERNAME` | Default admin account username | (none) | **Yes** (if auth enabled) | `main.go`, `config/config.go` |
| `ADMIN_PASSWORD` | Default admin account password | (none) | **Yes** (if auth enabled) | `main.go`, `config/config.go` |
| `CORS_ALLOWED_ORIGINS` | Comma-separated list of allowed CORS origins | `*` | No | `middleware/request.go`, `internal/middleware/middleware.go` |
| `CORS_ENABLED` | Enable/disable CORS | `false` | No | `docker-compose.yml` |
| `CORS_ORIGINS` | Allowed CORS origins (Docker Compose) | (empty) | No | `docker-compose.yml` |

### Database Configuration

| Variable | Description | Default | Required | Source File |
|----------|-------------|---------|----------|-------------|
| `DATABASE_TYPE` | Database type (`postgres`, `sqlite`) | `postgres` | No | `docker-compose.yml` |
| `DATABASE_HOST` | PostgreSQL host | `postgres` | No | `docker-compose.yml` |
| `DATABASE_PORT` | PostgreSQL port | `5432` | No | `docker-compose.yml` |
| `DATABASE_USER` | PostgreSQL username | `catalogizer` | No | `docker-compose.yml` |
| `DATABASE_PASSWORD` | PostgreSQL password | (none) | **Yes** (for Postgres) | `docker-compose.yml` |
| `DATABASE_NAME` | PostgreSQL database name | `catalogizer` | No | `docker-compose.yml` |
| `MEDIA_DB_PASSWORD` | Media database password (media manager) | (none) | No | `internal/media/manager.go` |

### Redis Configuration

| Variable | Description | Default | Required | Source File |
|----------|-------------|---------|----------|-------------|
| `REDIS_ADDR` | Redis server address (`host:port`) | (empty) | No | `main.go` |
| `REDIS_HOST` | Redis hostname (Docker Compose) | `redis` | No | `docker-compose.yml` |
| `REDIS_PORT` | Redis port (Docker Compose) | `6379` | No | `docker-compose.yml` |
| `REDIS_PASSWORD` | Redis authentication password | (empty) | No | `main.go`, `docker-compose.yml` |

### Configuration Paths

| Variable | Description | Default | Required | Source File |
|----------|-------------|---------|----------|-------------|
| `CATALOG_CONFIG_PATH` | Path to `config.json` file | `config.json` | No | `internal/config/config.go` |
| `CONFIG_PATH` | Configuration file path (wizard service) | (empty) | No | `services/configuration_wizard_service.go` |

### Path confinement and outbound destinations

| Variable | Description | Default | Required | Source File |
|----------|-------------|---------|----------|-------------|
| `CATALOGIZER_READ_ROOTS` | Colon-separated absolute directories the server may read from request-named paths (sync upload sources, conversion input; local storage roots are governed by `CATALOGIZER_LOCAL_ROOT_BASES`). Replaces the default when set | `MEDIA_ROOT_PATH` + `./data` | No | `internal/pathguard/roots.go` |
| `CATALOGIZER_LOCAL_ROOT_BASES` | Colon-separated absolute directories under which a `local` storage root may live (symlinks resolved, re-checked at every use; the database and key files are always refused). Unset = no local root allowed | unset (deny) | No | `filesystem/local_policy.go` |
| `CATALOGIZER_SYNC_ROOTS` | Directories sync downloads / destinations may write to | `./data` | No | `internal/pathguard/roots.go` |
| `CATALOGIZER_CONVERSION_ROOTS` | Directories conversion output may be written to | `./data` | No | `internal/pathguard/roots.go` |
| `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` | Comma-separated networks (e.g. `192.168.1.0/24`) the server may connect to even though they are private or loopback, for sync endpoints on a LAN NAS. Link-local / metadata addresses are never admitted. Invalid entries allow nothing | (empty: private ranges refused) | No | `internal/netguard/netguard.go` |

See `docs/security/PATH_CONFINEMENT.md`.

### SMB and Media

| Variable | Description | Default | Required | Source File |
|----------|-------------|---------|----------|-------------|
| `SMB_ENABLED` | Enable SMB filesystem support | `true` | No | `docker-compose.yml` |
| `MEDIA_ROOT_PATH` | Root path for media files | `/media` | No | `docker-compose.yml` |

---

## catalog-web (React Frontend)

These variables are accessed via `import.meta.env` (Vite environment variables). They must be prefixed with `VITE_` to be exposed to the browser.

| Variable | Description | Default | Required | Source File |
|----------|-------------|---------|----------|-------------|
| `VITE_API_BASE_URL` | Backend API base URL | `http://localhost:8080` | No | `src/lib/api.ts` |
| `VITE_WS_URL` | WebSocket server URL | `ws://localhost:8080/ws` | No | `src/lib/websocket.ts` |
| `VITE_ANALYTICS_URL` | Analytics endpoint URL | (none) | No | `src/lib/webVitals.ts` |

### Build-Time Variables

These are available during build but not at runtime:

| Variable | Description | Usage |
|----------|-------------|-------|
| `NODE_ENV` | Node environment (`development`, `production`) | Build mode detection |

---

## Docker Compose (Infrastructure)

These variables are used in `docker-compose.yml`, `docker-compose.dev.yml`, and `deployment/docker-compose.yml` for configuring container services.

### PostgreSQL

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `POSTGRES_USER` | PostgreSQL superuser name | `catalogizer` | No |
| `POSTGRES_PASSWORD` | PostgreSQL superuser password | (none) | **Yes** |
| `POSTGRES_DB` | Default database name | `catalogizer` | No |
| `POSTGRES_PORT` | Host port for PostgreSQL | `5432` | No |

### Redis

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `REDIS_PORT` | Host port for Redis | `6379` | No |
| `REDIS_PASSWORD` | Redis authentication password | (empty) | No |

### Nginx

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `HTTP_PORT` | Host port for HTTP | `80` | No |
| `HTTPS_PORT` | Host port for HTTPS | `443` | No |

### Docker Build

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `GO_VERSION` | Go version for building API image | `1.21` | No |

### Deployment-Specific (deployment/docker-compose.yml)

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `CATALOGIZER_VERSION` | Docker image tag | `latest` | No |
| `RESTART_POLICY` | Docker restart policy | `unless-stopped` | No |
| `CATALOGIZER_HOST` | Server bind host | `0.0.0.0` | No |
| `CATALOGIZER_PORT` | Server bind port | `8080` | No |
| `CATALOGIZER_WS_PORT` | WebSocket listen port | `8081` | No |
| `CATALOGIZER_ADMIN_PORT` | Admin interface port | `9090` | No |
| `CATALOGIZER_ENV` | Environment name | `production` | No |
| `MEDIA_DIRECTORIES` | Media mount directories | `/media` | No |
| `TRANSCODE_ENABLED` | Enable media transcoding | `true` | No |
| `TRANSCODE_QUALITY` | Transcoding quality (`low`, `medium`, `high`) | `medium` | No |
| `THUMBNAIL_GENERATION` | Enable thumbnail generation | `true` | No |
| `MAX_TRANSCODE_JOBS` | Maximum concurrent transcoding jobs | `2` | No |
| `TRANSCODER_CPU_LIMIT` | CPU limit for transcoder container | `2.0` | No |
| `TRANSCODER_MEMORY_LIMIT` | Memory limit for transcoder container | `2G` | No |
| `SSL_ENABLED` | Enable SSL/TLS | `false` | No |
| `SSL_CERT_PATH` | Path to SSL certificate | `/app/ssl/cert.pem` | No |
| `SSL_KEY_PATH` | Path to SSL private key | `/app/ssl/key.pem` | No |
| `DATA_DIR` | Persistent data directory | `/var/lib/catalogizer` | No |
| `WEB_PORT` | Web interface HTTP port | `80` | No |
| `WEB_SSL_PORT` | Web interface HTTPS port | `443` | No |
| `DOCKER_NETWORK` | Docker network name | `catalogizer-network` | No |
| `DOCKER_DATA_VOLUME` | Database volume name | `catalogizer-database-data` | No |
| `DOCKER_CONFIG_VOLUME` | Config volume name | `catalogizer-server-config` | No |

### Backup Service (deployment/docker-compose.yml, backup profile)

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `BACKUP_SCHEDULE` | Cron schedule for automated backups | `0 2 * * *` | No |
| `BACKUP_RETENTION_DAYS` | Days to retain local backups | `30` | No |
| `S3_BACKUP_ENABLED` | Enable S3 remote backup upload | `false` | No |
| `S3_BACKUP_BUCKET` | S3 bucket for backups | (empty) | No |
| `S3_ACCESS_KEY` | AWS access key for S3 backups | (empty) | No |
| `S3_SECRET_KEY` | AWS secret key for S3 backups | (empty) | No |

---

## Android / Android TV (Kotlin)

These are compile-time build configuration fields set in `build.gradle.kts`, accessed via `BuildConfig` in Kotlin code.

### catalogizer-android

| Field | Description | Debug Default | Release Default | Source File |
|-------|-------------|---------------|-----------------|-------------|
| `BuildConfig.API_BASE_URL` | Backend API base URL | `http://10.0.2.2:8080` | `https://your-catalogizer-api.com` | `app/build.gradle.kts` |

### catalogizer-androidtv

| Field | Description | Debug Default | Release Default | Source File |
|-------|-------------|---------------|-----------------|-------------|
| `BuildConfig.API_BASE_URL` | Backend API base URL | `http://10.0.2.2:8080` | `https://your-catalogizer-api.com` | `app/build.gradle.kts` |

Note: `10.0.2.2` is the Android emulator's alias for the host machine's localhost.

---

## Tauri Desktop / Installer Wizard

These are used during the Tauri build process via Vite.

| Variable | Description | Usage | Source File |
|----------|-------------|-------|-------------|
| `TAURI_PLATFORM` | Target platform (`windows`, `linux`, `macos`) | Build target detection | `installer-wizard/vite.config.ts` |
| `TAURI_DEBUG` | Debug mode flag | Enables source maps, disables minification | `installer-wizard/vite.config.ts` |

---

## Monitoring Stack

### Prometheus

| Variable | Description | Default | Source |
|----------|-------------|---------|-------|
| `PROMETHEUS_PORT` | Host port for Prometheus UI | `9090` | `docker-compose.yml` |

### Grafana

| Variable | Description | Default | Source |
|----------|-------------|---------|-------|
| `GRAFANA_PORT` | Host port for Grafana UI | `3001` (main), `3000` (deployment) | `docker-compose.yml` |
| `GRAFANA_USER` | Grafana admin username | `admin` | `docker-compose.yml` |
| `GRAFANA_PASSWORD` | Grafana admin password | `admin` | `docker-compose.yml` |

---

## Security Scanning

Used in `docker-compose.security.yml` for security testing tools.

### Snyk

| Variable | Description | Default | Source |
|----------|-------------|---------|-------|
| `SNYK_TOKEN` | Snyk authentication token | `dummy-token` | `docker-compose.security.yml` |
| `SNYK_ORG` | Snyk organization name | `catalogizer` | `docker-compose.security.yml` |
| `SNYK_SEVERITY_THRESHOLD` | Minimum severity to report (`low`, `medium`, `high`, `critical`) | `medium` | `docker-compose.security.yml` |

---

## Integration Testing

These are used only in integration and automation tests.

| Variable | Description | Default | Source File |
|----------|-------------|---------|-------------|
| `API_BASE_URL` | API base URL for automation tests | (none) | `tests/automation/storage_operations_test.go` |
| `SMB_TEST_SERVER` | SMB test server flag (set to enable SMB tests) | (empty) | `tests/integration/protocol_rename_tests.go` |
| `SMB_TEST_HOST` | SMB test server hostname | (none) | `tests/integration/protocol_rename_tests.go` |
| `SMB_TEST_SHARE` | SMB test share name | (none) | `tests/integration/protocol_rename_tests.go` |
| `SMB_TEST_USER` | SMB test username | (none) | `tests/integration/protocol_rename_tests.go` |
| `SMB_TEST_PASS` | SMB test password | (none) | `tests/integration/protocol_rename_tests.go` |
| `FTP_TEST_SERVER` | FTP test server flag (set to enable FTP tests) | (empty) | `tests/integration/protocol_rename_tests.go` |
| `NFS_TEST_SERVER` | NFS test server flag (set to enable NFS tests) | (empty) | `tests/integration/protocol_rename_tests.go` |
| `WEBDAV_TEST_SERVER` | WebDAV test server flag (set to enable WebDAV tests) | (empty) | `tests/integration/protocol_rename_tests.go` |

---

## Quick Reference by Component

### Minimum Production .env File

```bash
# === REQUIRED ===
POSTGRES_PASSWORD=<strong-password-here>
JWT_SECRET=<minimum-32-character-secret-here>

# === RECOMMENDED ===
APP_ENV=production
LOG_LEVEL=info
ADMIN_USERNAME=admin
ADMIN_PASSWORD=<strong-admin-password>
CORS_ENABLED=false
REDIS_PASSWORD=<redis-password>
GRAFANA_PASSWORD=<grafana-password>
```

### Full Production .env Template

```bash
# =============================================
# Catalogizer Production Environment Variables
# =============================================

# --- Application ---
APP_ENV=production
LOG_LEVEL=info
API_PORT=8080

# --- PostgreSQL Database ---
DATABASE_TYPE=postgres
POSTGRES_USER=catalogizer
POSTGRES_PASSWORD=CHANGE_ME_STRONG_PASSWORD
POSTGRES_DB=catalogizer
POSTGRES_PORT=5432

# --- Redis ---
REDIS_PORT=6379
REDIS_PASSWORD=CHANGE_ME_REDIS_PASSWORD

# --- Security ---
JWT_SECRET=CHANGE_ME_MINIMUM_32_CHARACTERS_LONG_SECRET
ADMIN_USERNAME=admin
ADMIN_PASSWORD=CHANGE_ME_ADMIN_PASSWORD
CORS_ENABLED=false
CORS_ORIGINS=https://your-domain.com

# --- Media / SMB ---
SMB_ENABLED=true
MEDIA_ROOT_PATH=/media

# --- Docker Build ---
GO_VERSION=1.21

# --- Nginx ---
HTTP_PORT=80
HTTPS_PORT=443

# --- Monitoring (optional) ---
PROMETHEUS_PORT=9090
GRAFANA_PORT=3001
GRAFANA_USER=admin
GRAFANA_PASSWORD=CHANGE_ME_GRAFANA_PASSWORD

# --- Backup (optional, deployment profile) ---
BACKUP_SCHEDULE=0 2 * * *
BACKUP_RETENTION_DAYS=30
S3_BACKUP_ENABLED=false
# S3_BACKUP_BUCKET=
# S3_ACCESS_KEY=
# S3_SECRET_KEY=
```

### Frontend .env Template

For `catalog-web/.env` or `catalog-web/.env.local`:

```bash
VITE_API_BASE_URL=https://your-domain.com
VITE_WS_URL=wss://your-domain.com/ws
# VITE_ANALYTICS_URL=https://analytics.your-domain.com
```

### catalog-api .env Template

For `catalog-api/.env` (standalone mode without Docker):

```bash
PORT=8080
HOST=localhost
GIN_MODE=release
DB_PATH=./data/catalogizer.db
JWT_SECRET=CHANGE_ME_MINIMUM_32_CHARACTERS_LONG_SECRET
ADMIN_USERNAME=admin
ADMIN_PASSWORD=CHANGE_ME_ADMIN_PASSWORD
ENABLE_AUTH=true
ENABLE_HTTPS=false
LOG_LEVEL=info
LOG_FORMAT=json
TEMP_DIR=./temp
MAX_CONCURRENT_SCANS=3
ENABLE_CACHE=true
CACHE_TTL_MINUTES=15
CORS_ALLOWED_ORIGINS=https://your-domain.com
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
```
