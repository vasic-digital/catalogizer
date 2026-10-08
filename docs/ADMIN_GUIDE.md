# Catalogizer -- Administrator Guide

## Table of Contents

1. [Overview](#overview)
2. [Server Setup](#server-setup)
3. [Configuration](#configuration)
4. [Database Administration](#database-administration)
5. [Storage Configuration](#storage-configuration)
6. [User Management](#user-management)
7. [Role and Permission Management](#role-and-permission-management)
8. [Authentication and Security](#authentication-and-security)
9. [Monitoring and Health Checks](#monitoring-and-health-checks)
10. [Log Management](#log-management)
11. [Error and Crash Reporting](#error-and-crash-reporting)
12. [Backup and Recovery](#backup-and-recovery)
13. [Performance Tuning](#performance-tuning)
14. [Reverse Proxy Setup](#reverse-proxy-setup)
15. [Containerized Deployment](#containerized-deployment)
16. [Systemd Service Management](#systemd-service-management)
17. [Upgrading](#upgrading)
18. [Troubleshooting](#troubleshooting)

---

## Overview

This guide is intended for system administrators responsible for deploying, configuring, and maintaining a Catalogizer instance. It covers the backend server (`catalog-api`), database management, storage protocol configuration, user administration, monitoring, and operational procedures.

Catalogizer's backend is a Go application built with the Gin framework, using SQLite (default) or PostgreSQL as its database. It supports multi-protocol filesystem access (SMB, FTP, NFS, WebDAV, local), JWT-based authentication, real-time event streaming via WebSocket, and optional Redis for distributed rate limiting.

---

## Server Setup

### Prerequisites

| Component | Requirement |
|-----------|------------|
| Go | 1.21 or later |
| Node.js | 18 or later (for web frontend) |
| Rust + Cargo | Latest stable (for Tauri desktop/installer builds) |
| SQLite3 | Bundled (via go-sqlcipher) |
| PostgreSQL | 13+ (optional, for production) |
| Redis | 6+ (optional, for distributed rate limiting) |
| FFmpeg | Latest (optional, for video conversion) |

### Building the Backend

```bash
cd catalog-api
go build -o catalog-api
```

### Running the Server

```bash
# Required environment variables
export JWT_SECRET="your-secret-key-minimum-32-characters-long"
export ADMIN_USERNAME="admin"
export ADMIN_PASSWORD="your-secure-admin-password"

# Optional environment variables
export PORT=8080
export GIN_MODE=release          # Use "release" for production
export REDIS_ADDR="localhost:6379"
export REDIS_PASSWORD=""

# Start the server
cd catalog-api
./catalog-api
```

Alternatively, use a configuration file (`config.json`):

```bash
./catalog-api --config /etc/catalogizer/config.json
```

### Building the Web Frontend

```bash
cd catalog-web
npm install
npm run build    # Production build output in dist/
```

The built frontend can be served by nginx, the Go server, or any static file server.

### Health Check

Verify the server is running:

```bash
curl http://localhost:8080/health
# Expected response: {"status":"healthy","time":"2026-02-16T10:00:00Z"}
```

---

## Configuration

### Configuration File

The server loads configuration from `config.json` in the working directory. If the file does not exist, a default configuration is generated automatically.

**File location by convention:**
- Development: `./config.json` (in the catalog-api directory)
- Production: `/etc/catalogizer/config.json`

### Configuration Structure

```json
{
  "server": {
    "host": "localhost",
    "port": 8080,
    "read_timeout": 30,
    "write_timeout": 30,
    "idle_timeout": 120,
    "enable_cors": true,
    "enable_https": false,
    "cert_file": "",
    "key_file": ""
  },
  "database": {
    "path": "./data/catalogizer.db",
    "max_open_connections": 25,
    "max_idle_connections": 5,
    "conn_max_lifetime": 300,
    "conn_max_idle_time": 60,
    "enable_wal": true,
    "cache_size": -2000,
    "busy_timeout": 5000
  },
  "auth": {
    "jwt_secret": "",
    "jwt_expiration_hours": 24,
    "enable_auth": true,
    "admin_username": "",
    "admin_password": ""
  },
  "catalog": {
    "default_page_size": 100,
    "max_page_size": 1000,
    "enable_cache": true,
    "cache_ttl_minutes": 15,
    "max_concurrent_scans": 3,
    "download_chunk_size": 1048576,
    "max_archive_size": 5368709120,
    "allowed_download_types": ["*"],
    "temp_dir": "/tmp/catalog-api"
  },
  "storage": {
    "roots": [
      {
        "id": "local-media",
        "name": "Local Media",
        "protocol": "local",
        "enabled": true,
        "max_depth": 10,
        "enable_duplicate_detection": true,
        "enable_metadata_extraction": true,
        "settings": {
          "base_path": "/mnt/media"
        }
      }
    ]
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout",
    "max_size": 100,
    "max_backups": 3,
    "max_age": 28,
    "compress": true
  }
}
```

### Environment Variable Overrides

Environment variables take precedence over config file values. These are the primary overrides:

| Variable | Description | Example |
|----------|-------------|---------|
| `JWT_SECRET` | JWT signing key (minimum 32 characters) | `your-secret-key-here` |
| `ADMIN_USERNAME` | Initial admin username | `admin` |
| `ADMIN_PASSWORD` | Initial admin password | `secure-password` |
| `PORT` | Server port | `8080` |
| `GIN_MODE` | Gin framework mode | `release` or `debug` |
| `REDIS_ADDR` | Redis server address | `localhost:6379` |
| `REDIS_PASSWORD` | Redis password | (empty for no auth) |

### Configuration Validation

The server validates configuration on startup. Common validation rules:
- Server port must be between 1 and 65535
- Database path cannot be empty
- JWT secret must be at least 32 characters when auth is enabled
- Admin credentials must be set when auth is enabled
- Default page size must be positive
- Max page size must be greater than or equal to default page size

If validation fails, the server logs the error and exits.

---

## Database Administration

### SQLite (Default)

SQLite is the default database, requiring no external setup. The database file is created automatically at the configured `database.path` (default: `./data/catalogizer.db`).

**Recommended SQLite settings for production:**

```json
{
  "database": {
    "path": "/var/lib/catalogizer/catalogizer.db",
    "enable_wal": true,
    "cache_size": -2000,
    "busy_timeout": 5000,
    "max_open_connections": 25,
    "max_idle_connections": 5
  }
}
```

The server applies these PRAGMA settings:
- `_journal_mode=WAL` -- Write-Ahead Logging for concurrent reads
- `_synchronous=NORMAL` -- balanced durability and performance
- `_foreign_keys=1` -- enforce referential integrity
- `_busy_timeout=5000` -- wait up to 5 seconds for locked database

### PostgreSQL (Production)

For production deployments with higher concurrency requirements:

```bash
# Set environment variables
export DB_TYPE=postgres
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=catalogizer
export DB_USER=catalogizer
export DB_PASSWORD=your_password
```

**Create the database:**

```sql
CREATE USER catalogizer WITH PASSWORD 'your_password';
CREATE DATABASE catalogizer OWNER catalogizer;
GRANT ALL PRIVILEGES ON DATABASE catalogizer TO catalogizer;
```

### Database Migrations

Migrations run automatically on server startup:

```
Running database migrations...
Database migrations completed successfully
```

If migrations fail, the server logs the error and exits. Check the migration error message to identify the issue.

### Manual Database Operations

**SQLite backup:**
```bash
sqlite3 /var/lib/catalogizer/catalogizer.db ".backup '/backup/catalogizer-$(date +%Y%m%d).db'"
```

**SQLite integrity check:**
```bash
sqlite3 /var/lib/catalogizer/catalogizer.db "PRAGMA integrity_check;"
```

**PostgreSQL backup:**
```bash
pg_dump -U catalogizer catalogizer > /backup/catalogizer-$(date +%Y%m%d).sql
```

---

## Storage Configuration

Catalogizer supports multiple storage protocols for scanning media files. Each storage root is configured in the `storage.roots` array.

### Local Filesystem

```json
{
  "id": "local-media",
  "name": "Local Media Library",
  "protocol": "local",
  "enabled": true,
  "max_depth": 10,
  "enable_duplicate_detection": true,
  "enable_metadata_extraction": true,
  "include_patterns": ["*.mp4", "*.mkv", "*.avi", "*.mp3", "*.flac"],
  "exclude_patterns": ["*.tmp", "*.partial"],
  "settings": {
    "base_path": "/mnt/media"
  }
}
```

### SMB/CIFS

```json
{
  "id": "nas-share",
  "name": "NAS Media Share",
  "protocol": "smb",
  "enabled": true,
  "max_depth": 8,
  "enable_duplicate_detection": true,
  "enable_metadata_extraction": true,
  "settings": {
    "host": "192.168.1.100",
    "share": "media",
    "username": "media_user",
    "password": "media_password",
    "domain": "WORKGROUP",
    "port": 445
  }
}
```

The SMB client includes:
- Circuit breaker pattern for fault tolerance
- Offline cache for resilience during network outages
- Exponential backoff retry for transient failures

### FTP

```json
{
  "id": "ftp-archive",
  "name": "FTP Archive",
  "protocol": "ftp",
  "enabled": true,
  "max_depth": 5,
  "settings": {
    "host": "ftp.example.com",
    "port": 21,
    "username": "archive_user",
    "password": "archive_password",
    "path": "/archive"
  }
}
```

### NFS

```json
{
  "id": "nfs-storage",
  "name": "NFS Storage",
  "protocol": "nfs",
  "enabled": true,
  "max_depth": 10,
  "settings": {
    "host": "nfs-server.local",
    "path": "/exports/media",
    "mount_point": "/mnt/nfs-storage",
    "options": "vers=4"
  }
}
```

### WebDAV

```json
{
  "id": "webdav-cloud",
  "name": "WebDAV Cloud Storage",
  "protocol": "webdav",
  "enabled": true,
  "max_depth": 6,
  "settings": {
    "url": "https://webdav.example.com/media/",
    "username": "webdav_user",
    "password": "webdav_password",
    "timeout": 30
  }
}
```

### SMB Discovery

The API provides endpoints for discovering SMB shares on the network:

```bash
# Discover shares on a host
curl -X POST http://localhost:8080/api/v1/smb/discover \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"host": "192.168.1.100"}'

# Test SMB connection
curl -X POST http://localhost:8080/api/v1/smb/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "host": "192.168.1.100",
    "share": "media",
    "username": "user",
    "password": "pass"
  }'

# Browse an SMB share
curl -X POST http://localhost:8080/api/v1/smb/browse \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "host": "192.168.1.100",
    "share": "media",
    "path": "/movies",
    "username": "user",
    "password": "pass"
  }'
```

### Storage Root Testing

Test a configured storage root's connectivity from the Configuration API:

```bash
curl -X POST http://localhost:8080/api/v1/configuration/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"storage_id": "nas-share"}'
```

---

## User Management

### Creating Users

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "password": "secure-password",
    "email": "newuser@example.com",
    "role": "user"
  }'
```

### Listing Users

```bash
curl -X GET http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Getting a Specific User

```bash
curl -X GET http://localhost:8080/api/v1/users/{user_id} \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Updating a User

```bash
curl -X PUT http://localhost:8080/api/v1/users/{user_id} \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "updated@example.com",
    "role": "admin"
  }'
```

### Resetting a User Password

```bash
curl -X POST http://localhost:8080/api/v1/users/{user_id}/reset-password \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"new_password": "new-secure-password"}'
```

### Locking and Unlocking Accounts

```bash
# Lock an account (prevent login)
curl -X POST http://localhost:8080/api/v1/users/{user_id}/lock \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Unlock an account
curl -X POST http://localhost:8080/api/v1/users/{user_id}/unlock \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Deleting a User

```bash
curl -X DELETE http://localhost:8080/api/v1/users/{user_id} \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Role and Permission Management

### Creating Roles

```bash
curl -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "media_manager",
    "description": "Can manage media and collections",
    "permissions": ["media.read", "media.write", "collections.manage"]
  }'
```

### Listing Roles

```bash
curl -X GET http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Viewing Available Permissions

```bash
curl -X GET http://localhost:8080/api/v1/roles/permissions \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Updating a Role

```bash
curl -X PUT http://localhost:8080/api/v1/roles/{role_id} \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "permissions": ["media.read", "media.write", "collections.manage", "subtitles.manage"]
  }'
```

### Deleting a Role

```bash
curl -X DELETE http://localhost:8080/api/v1/roles/{role_id} \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Authentication and Security

### JWT Configuration

JWT tokens are signed with the `JWT_SECRET` environment variable or the `auth.jwt_secret` config value. The secret must be at least 32 characters long.

**Token lifecycle:**
- Tokens expire after `jwt_expiration_hours` (default: 24 hours)
- Clients can refresh tokens via `POST /api/v1/auth/refresh`
- Tokens can be invalidated via `POST /api/v1/auth/logout`

**If no JWT secret is configured**, the server generates a random ephemeral secret at startup. This means all sessions are invalidated on server restart. Always configure a persistent secret in production.

### Rate Limiting

The server applies rate limiting at two levels:

| Endpoint Category | Rate Limit |
|-------------------|-----------|
| Authentication (`/api/v1/auth/*`) | 5 requests per minute per user |
| General API (`/api/v1/*`) | 100 requests per minute per user |

When Redis is available (configured via `REDIS_ADDR`), rate limiting is distributed across multiple server instances. Without Redis, rate limiting is in-memory per server instance.

### CORS Configuration

CORS is enabled by default. The middleware allows all origins in development. For production, restrict allowed origins in the nginx reverse proxy configuration.

### Middleware Stack

The server applies the following middleware in order:

1. **CORS** -- Cross-Origin Resource Sharing headers
2. **Metrics** -- Prometheus metrics collection
3. **Logger** -- Structured request logging
4. **Error Handler** -- Centralized error handling
5. **Request ID** -- Unique request identifier for tracing
6. **Input Validation** -- Request input sanitization

### HTTPS Configuration

For production deployments, configure HTTPS either at the application level or (recommended) at the reverse proxy level.

**Application-level HTTPS:**
```json
{
  "server": {
    "enable_https": true,
    "cert_file": "/etc/ssl/certs/catalogizer.crt",
    "key_file": "/etc/ssl/private/catalogizer.key"
  }
}
```

---

## Monitoring and Health Checks

### Health Endpoint

```bash
curl http://localhost:8080/health
# Response: {"status":"healthy","time":"2026-02-16T10:00:00Z"}
```

Use this endpoint for load balancer health checks and uptime monitoring.

### Prometheus Metrics

The server exposes Prometheus metrics at `/metrics`:

```bash
curl http://localhost:8080/metrics
```

Metrics include:
- HTTP request counts, durations, and status codes
- Go runtime metrics (goroutines, memory allocation)
- Custom application metrics

**Prometheus scrape configuration:**

```yaml
scrape_configs:
  - job_name: 'catalogizer'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

Runtime metrics are collected every 15 seconds automatically.

### System Status

```bash
curl -X GET http://localhost:8080/api/v1/configuration/status \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Returns system health information including database connectivity, storage accessibility, and service status.

### Statistics Endpoints

Comprehensive library statistics are available:

```bash
# Overall library statistics
curl http://localhost:8080/api/v1/stats/overall -H "Authorization: Bearer $TOKEN"

# File type distribution
curl http://localhost:8080/api/v1/stats/filetypes -H "Authorization: Bearer $TOKEN"

# Size distribution
curl http://localhost:8080/api/v1/stats/sizes -H "Authorization: Bearer $TOKEN"

# Duplicate statistics
curl http://localhost:8080/api/v1/stats/duplicates -H "Authorization: Bearer $TOKEN"

# Top duplicate groups
curl http://localhost:8080/api/v1/stats/duplicates/groups -H "Authorization: Bearer $TOKEN"

# Access patterns
curl http://localhost:8080/api/v1/stats/access -H "Authorization: Bearer $TOKEN"

# Growth trends
curl http://localhost:8080/api/v1/stats/growth -H "Authorization: Bearer $TOKEN"

# Scan history
curl http://localhost:8080/api/v1/stats/scans -H "Authorization: Bearer $TOKEN"

# Per-SMB-root statistics
curl http://localhost:8080/api/v1/stats/smb/{smb_root} -H "Authorization: Bearer $TOKEN"
```

---

## Log Management

### Log Collection

Create a log collection to capture logs from a specific time range:

```bash
curl -X POST http://localhost:8080/api/v1/logs/collect \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "debug-session-2026-02-16",
    "start_time": "2026-02-16T00:00:00Z",
    "end_time": "2026-02-16T23:59:59Z",
    "level": "debug"
  }'
```

### Viewing Log Collections

```bash
# List all collections
curl http://localhost:8080/api/v1/logs/collections \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Get a specific collection
curl http://localhost:8080/api/v1/logs/collections/{id} \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Get log entries for a collection
curl http://localhost:8080/api/v1/logs/collections/{id}/entries \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Log Analysis

```bash
curl http://localhost:8080/api/v1/logs/collections/{id}/analyze \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Returns pattern analysis of log entries, identifying common error patterns and anomalies.

### Log Export

```bash
curl -X POST http://localhost:8080/api/v1/logs/collections/{id}/export \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"format": "json"}'
```

### Log Sharing

Share logs securely with other team members via tokens:

```bash
# Create a shared log link
curl -X POST http://localhost:8080/api/v1/logs/share \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "collection_id": "abc123",
    "expires_in": "72h"
  }'

# Access shared logs (no auth required, uses token)
curl http://localhost:8080/api/v1/logs/share/{token}

# Revoke a shared log link
curl -X DELETE http://localhost:8080/api/v1/logs/share/{id} \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Real-time Log Streaming

```bash
curl http://localhost:8080/api/v1/logs/stream \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Returns a streaming response of log entries as they occur.

### Log Statistics

```bash
curl http://localhost:8080/api/v1/logs/statistics \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Application-Level Logging Configuration

Configure logging in `config.json`:

```json
{
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout",
    "max_size": 100,
    "max_backups": 3,
    "max_age": 28,
    "compress": true
  }
}
```

| Field | Description | Values |
|-------|-------------|--------|
| `level` | Minimum log level | `debug`, `info`, `warn`, `error` |
| `format` | Log output format | `json`, `text` |
| `output` | Output destination | `stdout`, `file` |
| `max_size` | Max log file size (MB) before rotation | Integer |
| `max_backups` | Number of rotated files to keep | Integer |
| `max_age` | Days to retain old log files | Integer |
| `compress` | Compress rotated files | `true`, `false` |

---

## Error and Crash Reporting

### Reporting Errors

```bash
# Report an error
curl -X POST http://localhost:8080/api/v1/errors/report \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "error_type": "storage_connection",
    "message": "Failed to connect to SMB share",
    "stack_trace": "...",
    "severity": "high",
    "component": "filesystem",
    "metadata": {"host": "192.168.1.100", "share": "media"}
  }'

# Report a crash
curl -X POST http://localhost:8080/api/v1/errors/crash \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "crash_type": "panic",
    "message": "nil pointer dereference",
    "stack_trace": "...",
    "component": "media_scanner"
  }'
```

### Viewing Reports

```bash
# List error reports
curl http://localhost:8080/api/v1/errors/reports \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Get a specific error report
curl http://localhost:8080/api/v1/errors/reports/{id} \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Update error status (investigating, resolved, etc.)
curl -X PUT http://localhost:8080/api/v1/errors/reports/{id}/status \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'

# List crash reports
curl http://localhost:8080/api/v1/errors/crashes \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Get a specific crash report
curl http://localhost:8080/api/v1/errors/crashes/{id} \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Update crash status
curl -X PUT http://localhost:8080/api/v1/errors/crashes/{id}/status \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "investigating"}'
```

### Statistics

```bash
# Error statistics
curl http://localhost:8080/api/v1/errors/statistics \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Crash statistics
curl http://localhost:8080/api/v1/errors/crash-statistics \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# System health overview (errors + crashes combined)
curl http://localhost:8080/api/v1/errors/health \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Backup and Recovery

### Database Backup

**SQLite backup (recommended method):**
```bash
# Stop the server or use SQLite online backup
sqlite3 /var/lib/catalogizer/catalogizer.db ".backup '/backup/catalogizer-$(date +%Y%m%d).db'"
```

**PostgreSQL backup:**
```bash
pg_dump -U catalogizer -h localhost catalogizer > /backup/catalogizer-$(date +%Y%m%d).sql
```

### Configuration Backup

```bash
cp /etc/catalogizer/config.json /backup/config-$(date +%Y%m%d).json
```

### Full System Backup

A complete backup includes:
1. Database file or dump
2. Configuration file
3. Media storage (if managed locally)
4. SSL certificates (if applicable)

```bash
#!/bin/bash
BACKUP_DIR="/backup/catalogizer-$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

# Database
sqlite3 /var/lib/catalogizer/catalogizer.db ".backup '$BACKUP_DIR/database.db'"

# Configuration
cp /etc/catalogizer/config.json "$BACKUP_DIR/config.json"

# Compress
tar -czf "$BACKUP_DIR.tar.gz" -C /backup "$(basename $BACKUP_DIR)"
rm -rf "$BACKUP_DIR"

echo "Backup created: $BACKUP_DIR.tar.gz"
```

### Automated Backup Schedule

Using cron:

```bash
# Edit crontab
crontab -e

# Daily database backup at 2:00 AM
0 2 * * * sqlite3 /var/lib/catalogizer/catalogizer.db ".backup '/backup/catalogizer-daily-$(date +\%Y\%m\%d).db'"

# Weekly full backup on Sundays at 3:00 AM
0 3 * * 0 /scripts/full-backup.sh

# Cleanup backups older than 30 days
0 4 * * * find /backup -name "catalogizer-daily-*.db" -mtime +30 -delete
```

### Recovery

**Restore SQLite database:**
```bash
# Stop the server
systemctl stop catalogizer

# Replace the database file
cp /backup/catalogizer-20260216.db /var/lib/catalogizer/catalogizer.db

# Fix permissions
chown catalogizer:catalogizer /var/lib/catalogizer/catalogizer.db

# Start the server
systemctl start catalogizer
```

**Restore PostgreSQL database:**
```bash
# Stop the server
systemctl stop catalogizer

# Drop and recreate the database
sudo -u postgres psql -c "DROP DATABASE catalogizer;"
sudo -u postgres psql -c "CREATE DATABASE catalogizer OWNER catalogizer;"

# Restore
psql -U catalogizer catalogizer < /backup/catalogizer-20260216.sql

# Start the server
systemctl start catalogizer
```

---

## Performance Tuning

### Database Tuning (SQLite)

For large libraries (100,000+ files):

```json
{
  "database": {
    "enable_wal": true,
    "cache_size": -4000,
    "busy_timeout": 10000,
    "max_open_connections": 50,
    "max_idle_connections": 10,
    "conn_max_lifetime": 600
  }
}
```

Key parameters:
- `enable_wal` -- Write-Ahead Logging enables concurrent readers with a single writer
- `cache_size` -- negative value sets size in KB (e.g., -4000 = 4 MB page cache)
- `busy_timeout` -- milliseconds to wait for a locked database
- `max_open_connections` -- limit concurrent database connections

### Catalog Tuning

```json
{
  "catalog": {
    "default_page_size": 100,
    "max_page_size": 500,
    "enable_cache": true,
    "cache_ttl_minutes": 30,
    "max_concurrent_scans": 2,
    "download_chunk_size": 2097152
  }
}
```

Key parameters:
- `max_concurrent_scans` -- limit parallel storage scanning (reduce for constrained systems)
- `cache_ttl_minutes` -- increase for stable libraries, decrease for frequently changing ones
- `download_chunk_size` -- larger chunks improve throughput for large files

### Redis for Distributed Rate Limiting

When running multiple server instances behind a load balancer, configure Redis:

```bash
export REDIS_ADDR="redis.example.com:6379"
export REDIS_PASSWORD="your-redis-password"
```

If Redis is unavailable, the server falls back to in-memory rate limiting per instance with a warning logged.

### Server Timeouts

Adjust timeouts for your network conditions:

```json
{
  "server": {
    "read_timeout": 60,
    "write_timeout": 60,
    "idle_timeout": 300
  }
}
```

For large file uploads/downloads, increase `read_timeout` and `write_timeout`.

---

## Reverse Proxy Setup

### Nginx Configuration

Create `/etc/nginx/sites-available/catalogizer`:

```nginx
server {
    listen 80;
    server_name catalogizer.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name catalogizer.example.com;

    ssl_certificate /etc/letsencrypt/live/catalogizer.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/catalogizer.example.com/privkey.pem;

    # Security headers
    add_header X-Frame-Options SAMEORIGIN;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # API proxy
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts for large file operations
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }

    # Health and metrics
    location /health {
        proxy_pass http://localhost:8080;
    }

    location /metrics {
        proxy_pass http://localhost:8080;
        # Restrict metrics to internal networks
        allow 10.0.0.0/8;
        allow 172.16.0.0/12;
        allow 192.168.0.0/16;
        deny all;
    }

    # Web frontend (static files)
    location / {
        root /var/www/catalogizer;
        try_files $uri $uri/ /index.html;
        expires 1d;
        add_header Cache-Control "public, immutable";
    }

    # File upload size limit
    client_max_body_size 5G;
}
```

Enable the site:

```bash
sudo ln -s /etc/nginx/sites-available/catalogizer /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

---

## Containerized Deployment

### Using Podman (Preferred)

This project uses Podman as the container runtime. Docker commands are compatible.

```bash
# Validate compose file
podman-compose -f docker-compose.dev.yml config --quiet

# Start development environment
podman-compose -f docker-compose.dev.yml up

# Start production environment
podman-compose -f docker-compose.yml up -d
```

**Build requirements:**
- Use `podman build --network host` to avoid SSL issues in containers
- Set `GOTOOLCHAIN=local` to prevent Go auto-downloading toolchains
- Use fully qualified image names (e.g., `docker.io/library/postgres:15`)

### Container Networking

When building containers, use `--network host` to resolve SSL certificate issues with package mirrors:

```bash
podman build --network host -t catalogizer:latest .
```

For compose files, set `network_mode: host` for the builder container.

---

## Systemd Service Management

### Service File

Create `/etc/systemd/system/catalogizer.service`:

```ini
[Unit]
Description=Catalogizer Media Management Server
After=network.target
Requires=network.target

[Service]
Type=simple
User=catalogizer
Group=catalogizer
WorkingDirectory=/var/lib/catalogizer
ExecStart=/usr/local/bin/catalog-api
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Environment
Environment=GIN_MODE=release
Environment=JWT_SECRET=your-production-secret-here
Environment=ADMIN_USERNAME=admin
Environment=ADMIN_PASSWORD=your-admin-password
EnvironmentFile=-/etc/catalogizer/env

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/catalogizer /var/log/catalogizer /tmp/catalog-api

[Install]
WantedBy=multi-user.target
```

### Service Commands

```bash
# Enable on boot
sudo systemctl enable catalogizer

# Start
sudo systemctl start catalogizer

# Stop
sudo systemctl stop catalogizer

# Restart
sudo systemctl restart catalogizer

# View status
sudo systemctl status catalogizer

# View logs
sudo journalctl -u catalogizer -f

# View logs from last hour
sudo journalctl -u catalogizer --since "1 hour ago"
```

---

## Upgrading

### Pre-Upgrade Checklist

1. Back up the database
2. Back up the configuration file
3. Note the current version
4. Review the release notes for breaking changes
5. Test the upgrade in a non-production environment first

### Upgrade Process

```bash
# Stop the server
sudo systemctl stop catalogizer

# Back up database
sqlite3 /var/lib/catalogizer/catalogizer.db ".backup '/backup/pre-upgrade-$(date +%Y%m%d).db'"

# Back up configuration
cp /etc/catalogizer/config.json /backup/config-pre-upgrade.json

# Replace the binary
sudo cp catalog-api-new /usr/local/bin/catalog-api
sudo chmod +x /usr/local/bin/catalog-api

# Start the server (migrations run automatically)
sudo systemctl start catalogizer

# Verify
curl http://localhost:8080/health
sudo journalctl -u catalogizer --since "5 minutes ago"
```

### Rollback

If the upgrade causes issues:

```bash
# Stop the new version
sudo systemctl stop catalogizer

# Restore the old binary
sudo cp /usr/local/bin/catalog-api.backup /usr/local/bin/catalog-api

# Restore the database
cp /backup/pre-upgrade-20260216.db /var/lib/catalogizer/catalogizer.db
chown catalogizer:catalogizer /var/lib/catalogizer/catalogizer.db

# Restore configuration
cp /backup/config-pre-upgrade.json /etc/catalogizer/config.json

# Start
sudo systemctl start catalogizer
```

---

## Troubleshooting

### Server Will Not Start

**Check the logs:**
```bash
sudo journalctl -u catalogizer --since "10 minutes ago" --no-pager
```

**Common causes:**
- Port already in use: `lsof -i :8080`
- Invalid configuration: check the error message for validation details
- Missing JWT secret: set `JWT_SECRET` environment variable
- Missing admin credentials: set `ADMIN_USERNAME` and `ADMIN_PASSWORD`
- Database file permissions: `ls -la /var/lib/catalogizer/`
- Database migration failure: check the migration error message

### Database Issues

**"database is locked" errors:**
- Ensure WAL mode is enabled: `enable_wal: true`
- Increase `busy_timeout` to 10000 or higher
- Reduce `max_open_connections` if too many concurrent accesses
- Check for long-running queries or scans

**Database corruption:**
```bash
sqlite3 /var/lib/catalogizer/catalogizer.db "PRAGMA integrity_check;"
```

If corruption is detected, restore from backup.

### Storage Connection Failures

**SMB shares not accessible:**
- Verify network connectivity: `ping 192.168.1.100`
- Test SMB access: `smbclient //192.168.1.100/share -U username`
- Check credentials in storage root settings
- Verify the share exists and permissions are correct
- Check firewall rules for port 445

**FTP connection failures:**
- Test with command-line FTP client
- Check passive mode setting
- Verify firewall allows data channel ports

### Rate Limiting Issues

If users report being rate limited:
- Check the rate limit configuration (5/min for auth, 100/min for API)
- If Redis is configured, verify Redis connectivity
- If Redis is down, the server uses in-memory rate limiting per instance

### Memory Issues

If the server consumes excessive memory:
- Reduce `cache_size` in database config
- Reduce `max_concurrent_scans`
- Monitor with Prometheus metrics at `/metrics`
- Check for goroutine leaks in the runtime metrics

### Graceful Shutdown

The server handles SIGINT and SIGTERM signals for graceful shutdown:
- Stops accepting new connections
- Waits up to 30 seconds for in-flight requests to complete
- Closes Redis connection (if configured)
- Closes database connection
- Logs "Server exited cleanly"

If the server does not shut down gracefully, check for hung requests or long-running operations.

---

*For developer-oriented information, see the [Developer Guide](DEVELOPER_GUIDE.md). For end-user documentation, see the [User Guide](USER_GUIDE.md). For quick reference on API endpoints and configuration, see the [Quick Reference](QUICK_REFERENCE.md).*
