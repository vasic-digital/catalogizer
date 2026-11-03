# Configuration Reference

Complete reference for all configuration options across Catalogizer components.

## Table of Contents

- [Configuration Loading Priority](#configuration-loading-priority)
- [Backend (catalog-api)](#backend-catalog-api)
- [Frontend (catalog-web)](#frontend-catalog-web)
- [Desktop Applications](#desktop-applications)
- [Android Applications](#android-applications)
- [Advanced Configuration](#advanced-configuration)
- [Environment-Specific Configs](#environment-specific-configs)
- [Security Best Practices](#security-best-practices)

---

## Configuration Loading Priority

Configuration is loaded in the following priority (highest to lowest):

1. **Environment variables** (highest priority)
2. **`.env` file** in application directory
3. **`config.json`** file (if present)
4. **Default values** (lowest priority)

Example:
```bash
# .env file sets DB_TYPE=sqlite
DB_TYPE=sqlite

# Environment variable overrides it
export DB_TYPE=postgres

# Result: postgres is used
```

---

## Backend (catalog-api)

### Server Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `PORT` | int | `8080` | HTTP server port |
| `HOST` | string | `0.0.0.0` | Server bind address (0.0.0.0 = all interfaces) |
| `GIN_MODE` | string | `debug` | Gin framework mode: `debug`, `release`, `test` |
| `CORS_ORIGINS` | string | `*` | Allowed CORS origins (comma-separated) |
| `CORS_METHODS` | string | `GET,POST,PUT,DELETE,OPTIONS` | Allowed HTTP methods |
| `CORS_HEADERS` | string | `*` | Allowed headers |
| `READ_TIMEOUT` | duration | `30s` | HTTP read timeout |
| `WRITE_TIMEOUT` | duration | `30s` | HTTP write timeout |
| `IDLE_TIMEOUT` | duration | `120s` | HTTP idle timeout |
| `MAX_HEADER_BYTES` | int | `1048576` | Max HTTP header size (1MB) |

**Example `.env`:**
```env
PORT=8080
GIN_MODE=release
CORS_ORIGINS=https://app.example.com,https://admin.example.com
READ_TIMEOUT=60s
WRITE_TIMEOUT=60s
```

### Database Configuration

**SQLite (Development):**

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DB_TYPE` | string | `sqlite` | Database type: `sqlite` or `postgres` |
| `DB_PATH` | string | `catalogizer.db` | SQLite database file path |
| `DB_PRAGMA` | string | `journal_mode=WAL` | SQLite PRAGMA settings (semicolon-separated) |

**PostgreSQL (Production):**

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DB_TYPE` | string | `sqlite` | Set to `postgres` for PostgreSQL |
| `DB_HOST` | string | `localhost` | PostgreSQL server hostname |
| `DB_PORT` | int | `5432` | PostgreSQL server port |
| `DB_NAME` | string | `catalogizer` | Database name |
| `DB_USER` | string | `catalogizer` | Database username |
| `DB_PASSWORD` | string | *(required)* | Database password |
| `DB_SSL_MODE` | string | `disable` | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |
| `DB_MAX_OPEN_CONNS` | int | `25` | Maximum open connections |
| `DB_MAX_IDLE_CONNS` | int | `5` | Maximum idle connections |
| `DB_CONN_MAX_LIFETIME` | duration | `5m` | Connection max lifetime |
| `DB_CONN_MAX_IDLE_TIME` | duration | `10m` | Connection max idle time |

**Example PostgreSQL `.env`:**
```env
DB_TYPE=postgres
DB_HOST=db.example.com
DB_PORT=5432
DB_NAME=catalogizer_prod
DB_USER=catalogizer_user
DB_PASSWORD=super_secure_password_here
DB_SSL_MODE=require
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
```

### Authentication & Security

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `JWT_SECRET` | string | *(required)* | JWT signing secret (min 32 chars) |
| `JWT_EXPIRATION` | duration | `24h` | JWT token expiration time |
| `JWT_REFRESH_EXPIRATION` | duration | `168h` | Refresh token expiration (7 days) |
| `JWT_ISSUER` | string | `catalogizer` | JWT issuer claim |
| `ADMIN_USERNAME` | string | `admin` | Default admin username |
| `ADMIN_PASSWORD` | string | *(required)* | Default admin password |
| `ADMIN_EMAIL` | string | `admin@localhost` | Default admin email |
| `SESSION_TIMEOUT` | duration | `30m` | Session inactivity timeout |
| `MAX_LOGIN_ATTEMPTS` | int | `5` | Max failed login attempts before lockout |
| `LOCKOUT_DURATION` | duration | `15m` | Account lockout duration |
| `PASSWORD_MIN_LENGTH` | int | `8` | Minimum password length |
| `PASSWORD_REQUIRE_SPECIAL` | bool | `true` | Require special characters in password |
| `PASSWORD_REQUIRE_NUMBER` | bool | `true` | Require numbers in password |
| `PASSWORD_REQUIRE_UPPERCASE` | bool | `true` | Require uppercase letters in password |

**Example `.env`:**
```env
JWT_SECRET=change_this_to_a_random_32_character_string_minimum
JWT_EXPIRATION=24h
ADMIN_USERNAME=admin
ADMIN_PASSWORD=<your-admin-password>
SESSION_TIMEOUT=30m
MAX_LOGIN_ATTEMPTS=5
PASSWORD_MIN_LENGTH=10
```

**Security Warning:** Never commit `.env` files with production secrets to version control!

### External API Keys

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `TMDB_API_KEY` | string | *(optional)* | TheMovieDB API key for movie/TV metadata |
| `OMDB_API_KEY` | string | *(optional)* | OMDB API key for additional metadata |
| `IMDB_API_KEY` | string | *(optional)* | IMDb API key (if available) |
| `TVDB_API_KEY` | string | *(optional)* | TheTVDB API key for TV show metadata |
| `FANART_API_KEY` | string | *(optional)* | Fanart.tv API key for artwork |

**Obtain API keys:**
- TMDB: https://www.themoviedb.org/settings/api
- OMDB: https://www.omdbapi.com/apikey.aspx

**Example `.env`:**
```env
TMDB_API_KEY=your_tmdb_api_key_here
OMDB_API_KEY=your_omdb_api_key_here
```

### Redis Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `REDIS_ENABLED` | bool | `false` | Enable Redis for rate limiting and caching |
| `REDIS_HOST` | string | `localhost` | Redis server hostname |
| `REDIS_PORT` | int | `6379` | Redis server port |
| `REDIS_PASSWORD` | string | *(empty)* | Redis password (if required) |
| `REDIS_DB` | int | `0` | Redis database number (0-15) |
| `REDIS_MAX_RETRIES` | int | `3` | Max connection retry attempts |
| `REDIS_POOL_SIZE` | int | `10` | Connection pool size |
| `REDIS_MIN_IDLE_CONNS` | int | `2` | Minimum idle connections |
| `REDIS_DIAL_TIMEOUT` | duration | `5s` | Connection dial timeout |
| `REDIS_READ_TIMEOUT` | duration | `3s` | Read operation timeout |
| `REDIS_WRITE_TIMEOUT` | duration | `3s` | Write operation timeout |
| `REDIS_POOL_TIMEOUT` | duration | `4s` | Pool timeout |

**Example `.env`:**
```env
REDIS_ENABLED=true
REDIS_HOST=redis.example.com
REDIS_PORT=6379
REDIS_PASSWORD=redis_password_here
REDIS_DB=0
REDIS_POOL_SIZE=20
```

### File Watcher Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `WATCHER_ENABLED` | bool | `true` | Enable real-time file watching |
| `WATCHER_DEBOUNCE_MS` | int | `500` | Debounce delay (milliseconds) |
| `WATCHER_BUFFER_SIZE` | int | `1000` | Event buffer size |
| `WATCHER_MAX_FILES` | int | `100000` | Maximum files to watch |
| `WATCHER_IGNORE_HIDDEN` | bool | `true` | Ignore hidden files (starting with .) |
| `WATCHER_IGNORE_PATTERNS` | string | `.DS_Store,Thumbs.db` | Ignore patterns (comma-separated) |
| `WATCHER_POLL_INTERVAL` | duration | `1s` | Polling interval for non-inotify systems |

**Example `.env`:**
```env
WATCHER_ENABLED=true
WATCHER_DEBOUNCE_MS=1000
WATCHER_BUFFER_SIZE=2000
WATCHER_IGNORE_PATTERNS=.DS_Store,Thumbs.db,*.tmp
```

### Media Analysis Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `MEDIA_WORKERS` | int | `4` | Number of concurrent media analysis workers |
| `ANALYSIS_TIMEOUT` | duration | `30s` | Timeout for single file analysis |
| `ANALYSIS_QUEUE_SIZE` | int | `1000` | Analysis queue buffer size |
| `ENABLE_VIDEO_ANALYSIS` | bool | `true` | Enable video file analysis |
| `ENABLE_AUDIO_ANALYSIS` | bool | `true` | Enable audio file analysis |
| `ENABLE_IMAGE_ANALYSIS` | bool | `true` | Enable image file analysis |
| `MIN_FILE_SIZE` | int64 | `1048576` | Minimum file size to analyze (1MB) |
| `MAX_FILE_SIZE` | int64 | `10737418240` | Maximum file size to analyze (10GB) |
| `SUPPORTED_VIDEO_EXTS` | string | `.mp4,.mkv,.avi,.mov,.wmv,.flv,.webm` | Video extensions |
| `SUPPORTED_AUDIO_EXTS` | string | `.mp3,.flac,.aac,.wav,.ogg,.m4a` | Audio extensions |
| `SUPPORTED_IMAGE_EXTS` | string | `.jpg,.jpeg,.png,.gif,.bmp,.webp` | Image extensions |

**Example `.env`:**
```env
MEDIA_WORKERS=8
ANALYSIS_TIMEOUT=60s
ENABLE_VIDEO_ANALYSIS=true
ENABLE_AUDIO_ANALYSIS=true
MIN_FILE_SIZE=524288
MAX_FILE_SIZE=21474836480
```

### WebSocket Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `WS_ENABLED` | bool | `true` | Enable WebSocket server |
| `WS_PATH` | string | `/ws` | WebSocket endpoint path |
| `WS_READ_BUFFER_SIZE` | int | `1024` | WebSocket read buffer size (bytes) |
| `WS_WRITE_BUFFER_SIZE` | int | `1024` | WebSocket write buffer size (bytes) |
| `WS_PING_INTERVAL` | duration | `30s` | Ping interval for connection keep-alive |
| `WS_PONG_TIMEOUT` | duration | `10s` | Pong wait timeout |
| `WS_WRITE_TIMEOUT` | duration | `10s` | Write operation timeout |
| `WS_MAX_CONNECTIONS` | int | `1000` | Maximum concurrent WebSocket connections |

**Example `.env`:**
```env
WS_ENABLED=true
WS_PATH=/ws
WS_PING_INTERVAL=30s
WS_MAX_CONNECTIONS=5000
```

### Logging Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `LOG_LEVEL` | string | `info` | Log level: `debug`, `info`, `warn`, `error`, `fatal` |
| `LOG_FORMAT` | string | `json` | Log format: `json`, `text`, `console` |
| `LOG_OUTPUT` | string | `stdout` | Log output: `stdout`, `stderr`, `file` |
| `LOG_FILE_PATH` | string | `logs/catalog-api.log` | Log file path (if LOG_OUTPUT=file) |
| `LOG_MAX_SIZE` | int | `100` | Max log file size (MB) before rotation |
| `LOG_MAX_BACKUPS` | int | `5` | Max number of old log files to keep |
| `LOG_MAX_AGE` | int | `30` | Max days to retain old log files |
| `LOG_COMPRESS` | bool | `true` | Compress rotated log files |
| `LOG_CALLER` | bool | `true` | Include caller information in logs |
| `LOG_STACKTRACE_LEVEL` | string | `error` | Log level to include stack traces |

**Example `.env`:**
```env
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUT=file
LOG_FILE_PATH=/var/log/catalogizer/api.log
LOG_MAX_SIZE=200
LOG_MAX_BACKUPS=10
```

### Rate Limiting Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `RATE_LIMIT_ENABLED` | bool | `true` | Enable rate limiting |
| `RATE_LIMIT_REQUESTS` | int | `100` | Max requests per window |
| `RATE_LIMIT_WINDOW` | duration | `1m` | Rate limit time window |
| `RATE_LIMIT_BY` | string | `ip` | Rate limit by: `ip`, `user`, `api_key` |
| `RATE_LIMIT_SKIP_AUTH` | bool | `false` | Skip rate limiting for authenticated users |

**Example `.env`:**
```env
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS=1000
RATE_LIMIT_WINDOW=1m
RATE_LIMIT_BY=user
```

### Performance & Optimization

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ENABLE_GZIP` | bool | `true` | Enable gzip compression for responses |
| `GZIP_LEVEL` | int | `5` | Gzip compression level (1-9) |
| `ENABLE_CACHE` | bool | `true` | Enable in-memory caching |
| `CACHE_TTL` | duration | `5m` | Default cache TTL |
| `CACHE_MAX_SIZE` | int | `10000` | Max cache entries |
| `ENABLE_PROFILING` | bool | `false` | Enable pprof profiling endpoints |
| `PROFILING_PORT` | int | `6060` | pprof server port |

**Example `.env`:**
```env
ENABLE_GZIP=true
GZIP_LEVEL=6
ENABLE_CACHE=true
CACHE_TTL=10m
CACHE_MAX_SIZE=50000
```

---

## Frontend (catalog-web)

### API Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `VITE_API_BASE_URL` | string | `http://localhost:8080/api/v1` | Backend API base URL |
| `VITE_WS_URL` | string | `ws://localhost:8080/ws` | WebSocket server URL |
| `VITE_API_TIMEOUT` | number | `30000` | API request timeout (milliseconds) |
| `VITE_MAX_RETRIES` | number | `3` | Max retry attempts for failed requests |
| `VITE_RETRY_DELAY` | number | `1000` | Retry delay (milliseconds) |

**Example `.env`:**
```env
VITE_API_BASE_URL=https://api.catalogizer.com/api/v1
VITE_WS_URL=wss://api.catalogizer.com/ws
VITE_API_TIMEOUT=60000
```

### Feature Flags

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `VITE_ENABLE_ANALYTICS` | boolean | `false` | Enable Google Analytics or similar |
| `VITE_ENABLE_DEBUG` | boolean | `false` | Enable debug mode (verbose logging) |
| `VITE_ENABLE_SERVICE_WORKER` | boolean | `true` | Enable PWA service worker |
| `VITE_ENABLE_OFFLINE_MODE` | boolean | `true` | Enable offline functionality |
| `VITE_ENABLE_AUTO_UPDATE` | boolean | `true` | Enable automatic updates check |

**Example `.env`:**
```env
VITE_ENABLE_ANALYTICS=true
VITE_ENABLE_DEBUG=false
VITE_ENABLE_OFFLINE_MODE=true
```

### External Services

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `VITE_SENTRY_DSN` | string | *(optional)* | Sentry error tracking DSN |
| `VITE_GA_TRACKING_ID` | string | *(optional)* | Google Analytics tracking ID |
| `VITE_POSTHOG_KEY` | string | *(optional)* | PostHog analytics key |

**Example `.env`:**
```env
VITE_SENTRY_DSN=https://your-sentry-dsn@sentry.io/project
VITE_GA_TRACKING_ID=G-XXXXXXXXXX
```

### UI Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `VITE_DEFAULT_THEME` | string | `dark` | Default theme: `light`, `dark`, `system` |
| `VITE_DEFAULT_LANGUAGE` | string | `en` | Default language code |
| `VITE_ITEMS_PER_PAGE` | number | `50` | Default pagination size |
| `VITE_MAX_UPLOAD_SIZE` | number | `104857600` | Max file upload size (100MB) |

**Example `.env`:**
```env
VITE_DEFAULT_THEME=dark
VITE_DEFAULT_LANGUAGE=en
VITE_ITEMS_PER_PAGE=100
```

---

## Desktop Applications

### catalogizer-desktop & installer-wizard

**Tauri Configuration (`src-tauri/tauri.conf.json`):**

```json
{
  "build": {
    "beforeDevCommand": "npm run dev",
    "beforeBuildCommand": "npm run build",
    "devPath": "http://localhost:5173",
    "distDir": "../dist"
  },
  "package": {
    "productName": "Catalogizer",
    "version": "1.0.0"
  },
  "tauri": {
    "allowlist": {
      "all": false,
      "fs": {
        "scope": ["$APPDATA/**", "$HOME/**"]
      },
      "dialog": {
        "open": true,
        "save": true
      },
      "shell": {
        "open": true
      }
    },
    "windows": [
      {
        "title": "Catalogizer",
        "width": 1200,
        "height": 800,
        "resizable": true,
        "fullscreen": false
      }
    ]
  }
}
```

**Environment Variables:**

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `TAURI_CONFIG` | string | *(optional)* | Path to custom tauri.conf.json |
| `TAURI_SKIP_CODESIGN` | boolean | `false` | Skip code signing (development only) |

---

## Android Applications

### Build Configuration (`app/build.gradle.kts`)

```kotlin
android {
    compileSdk = 34

    defaultConfig {
        applicationId = "com.catalogizer.android"
        minSdk = 28
        targetSdk = 34
        versionCode = 1
        versionName = "1.0.0"

        buildConfigField("String", "API_BASE_URL", "\"https://api.catalogizer.com\"")
        buildConfigField("String", "WS_URL", "\"wss://api.catalogizer.com/ws\"")
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
        debug {
            applicationIdSuffix = ".debug"
            buildConfigField("String", "API_BASE_URL", "\"http://10.0.2.2:8080\"")
        }
    }
}
```

**Environment-Specific Configuration:**

Create `local.properties`:
```properties
# Development
api.base.url=http://10.0.2.2:8080/api/v1
ws.url=ws://10.0.2.2:8080/ws

# API Keys
tmdb.api.key=your_tmdb_key
```

---

## Advanced Configuration

### Protocol-Specific Settings

**FTP:**
```env
FTP_TIMEOUT=30
FTP_PASSIVE_MODE=true
FTP_TLS_ENABLED=false
FTP_MAX_CONNECTIONS=5
```

**NFS:**
```env
NFS_VERSION=4
NFS_MOUNT_OPTIONS=soft,timeo=30,retrans=3
NFS_AUTO_MOUNT=true
```

**WebDAV:**
```env
WEBDAV_TIMEOUT=60
WEBDAV_CHUNK_SIZE=8388608
WEBDAV_MAX_RETRIES=3
```

**SMB:**
```env
SMB_VERSION=3
SMB_TIMEOUT=30
SMB_MAX_CONNECTIONS=10
SMB_CIRCUIT_BREAKER_THRESHOLD=5
SMB_CIRCUIT_BREAKER_TIMEOUT=60
SMB_OFFLINE_CACHE_ENABLED=true
```

### Health Check Configuration

```env
HEALTH_CHECK_ENABLED=true
HEALTH_CHECK_PATH=/health
HEALTH_CHECK_TIMEOUT=5s
HEALTH_CHECK_INTERVAL=30s
```

### Metrics & Monitoring

```env
METRICS_ENABLED=true
METRICS_PATH=/metrics
PROMETHEUS_ENABLED=true
PROMETHEUS_PORT=9090
TRACING_ENABLED=false
JAEGER_ENDPOINT=http://localhost:14268/api/traces
```

---

## Environment-Specific Configs

### Development (`.env.development`)

```env
GIN_MODE=debug
LOG_LEVEL=debug
DB_TYPE=sqlite
DB_PATH=catalogizer_dev.db
CORS_ORIGINS=http://localhost:5173
ENABLE_PROFILING=true
```

### Staging (`.env.staging`)

```env
GIN_MODE=release
LOG_LEVEL=info
DB_TYPE=postgres
DB_HOST=staging-db.example.com
CORS_ORIGINS=https://staging.catalogizer.com
RATE_LIMIT_REQUESTS=1000
```

### Production (`.env.production`)

```env
GIN_MODE=release
LOG_LEVEL=warn
DB_TYPE=postgres
DB_HOST=prod-db.example.com
DB_SSL_MODE=require
CORS_ORIGINS=https://catalogizer.com,https://app.catalogizer.com
RATE_LIMIT_REQUESTS=10000
ENABLE_GZIP=true
METRICS_ENABLED=true
```

**Load environment-specific config:**
```bash
# Development
cp .env.development .env

# Production
cp .env.production .env
```

---

## Security Best Practices

### Secrets Management

**Never commit secrets:**
```bash
# Add to .gitignore
.env
.env.*
!.env.example
config/secrets.json
```

**Use environment variables in production:**
```bash
# Set in systemd service file
Environment="JWT_SECRET=your_secret_here"
Environment="DB_PASSWORD=db_password_here"

# Or use secrets manager
export JWT_SECRET=$(vault kv get -field=jwt_secret secret/catalogizer)
```

**Create `.env.example` template:**
```env
# Server
PORT=8080
GIN_MODE=debug

# Database
DB_TYPE=sqlite
DB_PATH=catalogizer.db

# Authentication (CHANGE IN PRODUCTION!)
JWT_SECRET=change_this_secret
ADMIN_PASSWORD=change_this_password

# External APIs (optional)
TMDB_API_KEY=your_key_here
```

### Production Checklist

- [ ] Set `GIN_MODE=release`
- [ ] Set `LOG_LEVEL=warn` or `error`
- [ ] Use PostgreSQL instead of SQLite
- [ ] Enable `DB_SSL_MODE=require`
- [ ] Set strong `JWT_SECRET` (32+ characters)
- [ ] Set strong `ADMIN_PASSWORD`
- [ ] Configure specific `CORS_ORIGINS` (not `*`)
- [ ] Enable rate limiting
- [ ] Enable HTTPS/TLS
- [ ] Set up log rotation
- [ ] Enable metrics collection
- [ ] Configure health checks
- [ ] Set appropriate timeouts
- [ ] Limit max connections

---

## Configuration Validation

**Validate configuration on startup:**

```go
func validateConfig() error {
    if os.Getenv("JWT_SECRET") == "" {
        return errors.New("JWT_SECRET is required")
    }
    if len(os.Getenv("JWT_SECRET")) < 32 {
        return errors.New("JWT_SECRET must be at least 32 characters")
    }
    // Add more validations...
    return nil
}
```

**Check configuration:**
```bash
# catalog-api includes built-in validation
go run main.go
# Will error if required variables are missing
```

---

## Additional Resources

- [Development Setup Guide](DEVELOPMENT_SETUP.md)
- [Protocol Implementation Guide](PROTOCOL_IMPLEMENTATION_GUIDE.md)
- [API Documentation](../api/API_DOCUMENTATION.md)
- [Security Guide](../security/README.md)
