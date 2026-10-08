---
title: Configuration Reference
description: Complete reference for Catalogizer configuration - environment variables, config.json, and protocol settings
---

# Configuration Reference

Catalogizer is configured through environment variables, a `.env` file, and a `config.json` file. Environment variables always take the highest precedence.

**Precedence order**: Environment variables > `.env` file > `config.json` > Built-in defaults

---

## Environment Variables

Create a `.env` file in the `catalog-api/` directory. All variables listed below can also be set as system environment variables.

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port (0 = dynamic port) |
| `GIN_MODE` | `debug` | Gin framework mode (`debug`, `release`, `test`) |
| `LOG_LEVEL` | `info` | Log verbosity (`debug`, `info`, `warn`, `error`) |
| `WRITE_TIMEOUT` | `30` | HTTP write timeout in seconds (set to `900` for long scans) |
| `READ_TIMEOUT` | `30` | HTTP read timeout in seconds |

### Database

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_TYPE` | `sqlite` | Database engine (`sqlite` or `postgres`) |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `catalogizer` | PostgreSQL database name |
| `DB_USER` | `catalogizer` | PostgreSQL username |
| `DB_PASSWORD` | | PostgreSQL password |
| `DB_ENCRYPTION_KEY` | | SQLCipher encryption key (exactly 32 characters) |

### Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | | Secret key for signing JWT tokens (required) |
| `JWT_EXPIRY` | `24h` | Access token expiry duration |
| `JWT_REFRESH_EXPIRY` | `168h` | Refresh token expiry duration (default 7 days) |
| `ADMIN_USERNAME` | `admin` | Default admin account username |
| `ADMIN_PASSWORD` | `admin123` | Default admin account password |

### External Providers

| Variable | Default | Description |
|----------|---------|-------------|
| `TMDB_API_KEY` | | The Movie Database API key (free) |
| `OMDB_API_KEY` | | Open Movie Database API key |

### Redis (Optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `localhost` | Redis server host |
| `REDIS_PORT` | `6379` | Redis server port |
| `REDIS_PASSWORD` | | Redis password |
| `REDIS_DB` | `0` | Redis database number |

### SMB Resilience

| Variable | Default | Description |
|----------|---------|-------------|
| `SMB_RETRY_ATTEMPTS` | `3` | Number of retry attempts for failed SMB operations |
| `SMB_RETRY_DELAY_SECONDS` | `5` | Initial delay between retries (doubles with backoff) |
| `SMB_HEALTH_CHECK_INTERVAL` | `60` | Seconds between SMB health checks |

---

## config.json

The `config.json` file in `catalog-api/` provides structured configuration. Values here are overridden by environment variables when both are set.

```json
{
  "server": {
    "port": 8080,
    "gin_mode": "release",
    "read_timeout": 30,
    "write_timeout": 900,
    "log_level": "info"
  },
  "database": {
    "type": "sqlite",
    "host": "localhost",
    "port": 5432,
    "name": "catalogizer",
    "user": "catalogizer",
    "password": ""
  },
  "auth": {
    "jwt_secret": "your-secret-key",
    "jwt_expiry": "24h",
    "admin_username": "admin",
    "admin_password": "admin123"
  },
  "providers": {
    "tmdb_api_key": "",
    "omdb_api_key": ""
  },
  "redis": {
    "host": "localhost",
    "port": 6379,
    "password": "",
    "db": 0
  }
}
```

---

## Protocol Configuration

Storage sources are configured through the web interface or API. Each protocol requires specific parameters.

### Local Filesystem

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Absolute path to the media directory |

```json
{
  "protocol": "local",
  "path": "/media/movies"
}
```

### SMB/CIFS

| Parameter | Required | Description |
|-----------|----------|-------------|
| `host` | Yes | Server hostname or IP address |
| `share` | Yes | Share name |
| `username` | Yes | Authentication username |
| `password` | Yes | Authentication password |
| `domain` | No | Windows domain |
| `port` | No | Port (default: 445) |

```json
{
  "protocol": "smb",
  "host": "nas.local",
  "share": "Media",
  "username": "user",
  "password": "pass",
  "domain": "WORKGROUP"
}
```

### FTP/FTPS

| Parameter | Required | Description |
|-----------|----------|-------------|
| `host` | Yes | Server hostname or IP address |
| `port` | No | Port (default: 21) |
| `username` | Yes | Authentication username |
| `password` | Yes | Authentication password |
| `path` | No | Base directory path (absolute, or relative to the login directory) |

> Plain FTP only. FTPS (FTP over TLS) is not available yet: the settings contract rejects a `tls` key instead of ignoring it. Any key a protocol does not use is rejected with the name of the key (never its value).

```json
{
  "protocol": "ftp",
  "host": "ftp.example.com",
  "port": 21,
  "username": "user",
  "password": "pass",
  "path": "/media"
}
```

### NFS

| Parameter | Required | Description |
|-----------|----------|-------------|
| `host` | Yes | Server hostname or IP address |
| `path` | Yes | NFS export path (the retired spelling `export_path` is rejected with a hint) |
| `mount_point` | No | Local directory the export is mounted on |
| `options` | No | Mount options, for example `vers=3,ro` (default `vers=3`) |

```json
{
  "protocol": "nfs",
  "host": "nas.local",
  "path": "/volume1/media",
  "mount_point": "/mnt/nas-media"
}
```

### WebDAV

| Parameter | Required | Description |
|-----------|----------|-------------|
| `url` | Yes | Full WebDAV URL |
| `username` | Yes | Authentication username |
| `password` | Yes | Authentication password |
| `path` | No | Root collection UNDER the URL's own path (url `https://host/dav/alice` with path `/Movies` addresses `/dav/alice/Movies`) |

```json
{
  "protocol": "webdav",
  "url": "https://cloud.example.com/remote.php/webdav/media",
  "username": "user",
  "password": "pass"
}
```

### Every storage root

| Parameter | Required | Description |
|-----------|----------|-------------|
| `max_depth` | No | Deepest directory level a scan lists (default 10, hard cap 64). A tree deeper than the bound makes the scan FAIL with `limit_exceeded`; it is never truncated silently |
| `allow_empty` | No | `true` when the share is expected to be empty. By default a scan that finds nothing FAILS (`empty_root`), because an empty answer is also what an unreachable or mis-addressed source looks like |

---

## Container Configuration

When running in containers, pass environment variables through the Compose file or container run flags. Catalogizer automatically detects the available container runtime (Podman or Docker).

```yaml
# docker-compose.yml excerpt
services:
  catalog-api:
    environment:
      - DB_TYPE=postgres
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_NAME=catalogizer
      - DB_USER=catalogizer
      - DB_PASSWORD=${POSTGRES_PASSWORD}
      - JWT_SECRET=${JWT_SECRET}
      - GIN_MODE=release
```

### Resource Limits

Production containers should enforce resource limits:

| Container | CPU | Memory |
|-----------|-----|--------|
| PostgreSQL | 1 | 2 GB |
| catalog-api | 2 | 4 GB |
| catalog-web | 1 | 2 GB |

---

## Dynamic Port Binding

When `PORT` is set to `0`, the server binds to a random available port and writes the chosen port to a `.service-port` file in the `catalog-api/` directory. The frontend dev server reads this file to configure its API proxy automatically.
