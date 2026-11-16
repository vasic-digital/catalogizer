# Performance Tuning Guide

**Document Version:** 2.0
**Last Updated:** April 14, 2026
**Applies to:** Catalogizer v2.3.0+ (catalog-api)

---

## Table of Contents

1. [Overview](#1-overview)
2. [Database Connection Pool Tuning](#2-database-connection-pool-tuning)
3. [SQLite-Specific Tuning](#3-sqlite-specific-tuning)
4. [PostgreSQL-Specific Tuning](#4-postgresql-specific-tuning)
5. [Cache Configuration](#5-cache-configuration)
6. [HTTP/3 and Compression Settings](#6-http3-and-compression-settings)
7. [Rate Limiting Configuration](#7-rate-limiting-configuration)
8. [Resource Limits](#8-resource-limits)
9. [Monitoring Setup](#9-monitoring-setup)
10. [Performance Benchmarking](#10-performance-benchmarking)
11. [Recommended Configurations](#11-recommended-configurations)

---

## 1. Overview

### 1.1 Performance Targets

| Metric | Target | Critical Threshold |
|--------|--------|-------------------|
| API Response (P95) | < 200ms | > 500ms |
| Database Query (P95) | < 50ms | > 200ms |
| File Scan Speed | > 1000 files/min | < 500 files/min |
| Memory Usage | < 4GB | > 8GB |
| CPU Usage | < 70% | > 90% |
| Concurrent Users | 1000+ | < 500 |

### 1.2 Configuration Precedence

All configuration follows this precedence (highest to lowest):

1. Environment variables (e.g., `DATABASE_TYPE`, `JWT_SECRET`)
2. `.env` file in `catalog-api/`
3. `config.json` file
4. Compiled defaults in `config/config.go`

### 1.3 Key Configuration Struct

The main configuration is defined in `catalog-api/config/config.go` as the `Config` struct with these sections:

| Section | Struct | Controls |
|---------|--------|----------|
| `server` | `ServerConfig` | Host, port, timeouts, CORS, HTTPS |
| `database` | `DatabaseConfig` | Connection parameters, pool settings |
| `auth` | `AuthConfig` | JWT secret, expiration, admin credentials |
| `catalog` | `CatalogConfig` | Page sizes, cache, scanner concurrency |
| `storage` | `StorageConfig` | Storage root definitions |
| `logging` | `LoggingConfig` | Log level, format, rotation |

---

## 2. Database Connection Pool Tuning

The connection pool is configured in `database/connection.go`. The `MaxOpenConns` setting acts as the query concurrency semaphore -- the `sql.DB` pool blocks callers when all connections are in use, so no additional application-level semaphore is needed.

### 2.1 Pool Parameters

| Parameter | Config Key | Default | Description |
|-----------|-----------|---------|-------------|
| MaxOpenConns | `max_open_connections` | 25 | Maximum number of open connections to the database |
| MaxIdleConns | `max_idle_connections` | 10 (config default: 5) | Maximum number of idle connections in the pool |
| ConnMaxLifetime | `conn_max_lifetime` | 300 seconds (5 min) | Maximum time a connection can be reused |
| ConnMaxIdleTime | `conn_max_idle_time` | 180 seconds (3 min, config default: 60s) | Maximum time a connection can sit idle |

Note: The code in `connection.go` uses fallback defaults of 25/10/5min/3min when config values are <= 0. The `config.go` defaults differ slightly (5 idle, 60s idle time) but the connection code overrides these if they are too low.

### 2.2 How Pool Settings Work

```
                     +--------------------------+
                     |   sql.DB Connection Pool |
                     |                          |
   Request ------>   |  MaxOpen = 25            |  <-- Blocks if all 25 are busy
                     |  MaxIdle = 10            |  <-- Keeps 10 warm connections
                     |  MaxLifetime = 5 min     |  <-- Recycles after 5 min
                     |  MaxIdleTime = 3 min     |  <-- Closes if idle > 3 min
                     +--------------------------+
```

### 2.3 Tuning Guidance

**For low-traffic development:**

```json
{
  "database": {
    "max_open_connections": 10,
    "max_idle_connections": 5,
    "conn_max_lifetime": 300,
    "conn_max_idle_time": 180
  }
}
```

**For high-traffic production (PostgreSQL):**

```json
{
  "database": {
    "max_open_connections": 50,
    "max_idle_connections": 20,
    "conn_max_lifetime": 600,
    "conn_max_idle_time": 300
  }
}
```

**Key considerations:**

- `MaxOpenConns` should not exceed PostgreSQL's `max_connections` setting (default: 100). If running multiple catalog-api instances, divide accordingly.
- `MaxIdleConns` should be 40-50% of `MaxOpenConns` to avoid excessive connection churn.
- `ConnMaxLifetime` prevents stale connections. Keep it shorter than PostgreSQL's `idle_in_transaction_session_timeout`.
- For SQLite, `MaxOpenConns` has less impact since SQLite serializes writes, but it still controls read concurrency.

### 2.4 Default Query Timeout

The `defaultQueryTimeout` constant in `connection.go` is set to **30 seconds**. This is used by the non-context `Exec()` method as a safety timeout to prevent indefinite hangs from lock contention or network issues. Context-aware methods (`ExecContext`, `QueryContext`, `QueryRowContext`) allow callers to set their own timeouts.

### 2.5 Monitoring Pool Health

Check connection pool statistics via `db.GetStats()` which returns `sql.DBStats`:

```go
stats := db.GetStats()
// stats.OpenConnections -- current open connections
// stats.InUse           -- connections currently in use
// stats.Idle            -- idle connections
// stats.WaitCount       -- total number of waits for a connection
// stats.WaitDuration    -- total time waiting for connections
```

These metrics are exposed via Prometheus at `/metrics`.

---

## 3. SQLite-Specific Tuning

SQLite is the default development database. Configuration is in `DatabaseConfig` and applied in `connection.go`.

### 3.1 WAL Mode

WAL (Write-Ahead Logging) mode is enabled in two places for reliability:

1. **Connection string pragma:** `?_journal_mode=WAL` in the DSN
2. **Explicit PRAGMA after connection:** `PRAGMA journal_mode=WAL` -- required because go-sqlcipher ignores connection-string pragmas

This dual approach ensures WAL mode is always active. WAL mode allows concurrent reads during writes and significantly improves performance under concurrent access.

### 3.2 Connection String Parameters

The SQLite connection string is constructed in `connection.go`:

```
<path>?_busy_timeout=30000&_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=1
```

| Parameter | Value | Description |
|-----------|-------|-------------|
| `_busy_timeout` | 30000 (30s) | Time to wait when the database is locked before returning SQLITE_BUSY |
| `_journal_mode` | WAL | Write-Ahead Logging for concurrent read/write |
| `_synchronous` | NORMAL | Balance between safety and speed (FULL is safest but slowest) |
| `_foreign_keys` | 1 | Enforce foreign key constraints |
| `_wal_autocheckpoint` | 1000 | WAL auto-checkpoint threshold (pages), added when `EnableWAL` is true |
| `_cache_size` | configurable | Page cache size (negative = KB, positive = pages), default -2000 (2MB) |

### 3.3 Config Options

```json
{
  "database": {
    "type": "sqlite",
    "path": "./catalog.db",
    "enable_wal": true,
    "cache_size": -2000,
    "busy_timeout": 5000
  }
}
```

| Key | Default | Description |
|-----|---------|-------------|
| `path` | `./catalog.db` | Database file path |
| `enable_wal` | true | Add WAL autocheckpoint to connection string |
| `cache_size` | -2000 | Page cache in KB (negative) or pages (positive). -2000 = 2MB cache |
| `busy_timeout` | 5000 | Used by `createContext()` for health checks (5 seconds) |

### 3.4 SQLite Performance Tips

- **Increase cache_size** for large databases: `-8000` (8MB) or `-16000` (16MB) reduces disk I/O
- **Batch inserts in transactions** -- SQLite commits per-statement by default, which is slow for bulk operations
- **Use NORMAL synchronous mode** (already set) -- FULL flushes to disk on every commit
- **WAL checkpoint manually** during maintenance windows: `PRAGMA wal_checkpoint(TRUNCATE)`
- **VACUUM periodically** to reclaim space: `VACUUM` (locks the database during operation)

---

## 4. PostgreSQL-Specific Tuning

PostgreSQL is the production database. Container port mapping: 5432 (internal) -> 5433 (host).

### 4.1 Connection Parameters

```json
{
  "database": {
    "type": "postgres",
    "host": "localhost",
    "port": 5433,
    "name": "catalogizer",
    "user": "catalogizer",
    "password": "catalogizer_dev",
    "ssl_mode": "disable"
  }
}
```

Environment variable overrides:

| Env Variable | Overrides |
|-------------|-----------|
| `DATABASE_TYPE` | `database.type` |
| `DATABASE_HOST` | `database.host` |
| `DATABASE_PORT` | `database.port` |
| `DATABASE_NAME` | `database.name` |
| `DATABASE_USER` | `database.user` |
| `DATABASE_PASSWORD` | `database.password` |
| `DATABASE_SSL_MODE` | `database.ssl_mode` |

### 4.2 Index Strategy

The migration system creates a comprehensive set of indexes across 15 migration versions. Key performance indexes (from migrations v9 and v14):

**Files table** (most heavily queried):

| Index | Columns | Purpose |
|-------|---------|---------|
| idx_files_storage_root_path | (storage_root_id, path) | UNIQUE, primary lookup |
| idx_files_file_type | file_type | File type filtering |
| idx_files_extension | extension | Extension filtering |
| idx_files_name | name | Name search |
| idx_files_is_directory | is_directory | Directory listing |
| idx_files_created_at | created_at | Time-range queries |
| idx_files_modified_at | modified_at | Recent changes |
| idx_files_size | size | Size-based queries |
| idx_files_storage_root_created | (storage_root_id, created_at) | Compound time query |

**Media items** (entity browsing):

| Index | Columns | Purpose |
|-------|---------|---------|
| idx_media_items_title_type | (title, media_type_id) | GetByTitle + GetDuplicates |
| idx_media_items_status | status | Status filtering |
| idx_media_items_year | year | Year filtering |
| idx_media_files_item_file | (media_item_id, file_id) | UNIQUE, prevents duplicate links |

**Analytics** (time-series queries):

| Index | Columns | Purpose |
|-------|---------|---------|
| idx_analytics_events_time | timestamp | Time-range queries |
| idx_analytics_events_user | (user_id, timestamp) | Per-user timeline |
| idx_analytics_events_type | (event_type, timestamp) | Event type analysis |

### 4.3 PostgreSQL Server Tuning

Recommended `postgresql.conf` settings for Catalogizer:

```ini
# Memory (adjust to container memory limit)
shared_buffers = 512MB          # 25% of available memory
effective_cache_size = 1536MB   # 75% of available memory
work_mem = 16MB                 # Per-query sort/hash memory
maintenance_work_mem = 128MB    # VACUUM, CREATE INDEX

# WAL
wal_buffers = 16MB
checkpoint_completion_target = 0.9
max_wal_size = 1GB

# Connections
max_connections = 100           # Must be >= sum of all MaxOpenConns
idle_in_transaction_session_timeout = '5min'

# Query planner
random_page_cost = 1.1          # SSD storage
effective_io_concurrency = 200  # SSD storage
```

### 4.4 Slow Query Analysis

```sql
-- Enable pg_stat_statements extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Find slowest queries by mean time
SELECT query, calls, mean_exec_time, max_exec_time, total_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Find tables missing indexes (sequential scans on large tables)
SELECT schemaname, relname, seq_scan, seq_tup_read, idx_scan, idx_tup_fetch
FROM pg_stat_user_tables
WHERE seq_scan > 0
ORDER BY seq_tup_read DESC
LIMIT 10;
```

---

## 5. Cache Configuration

### 5.1 In-Memory Cache (CacheService)

The `CacheService` in `internal/services/cache_service.go` provides database-backed caching with automatic cleanup.

**TTL Constants:**

| Cache Type | TTL | Constant |
|-----------|-----|----------|
| Default | 24 hours | `DefaultCacheTTL` |
| Metadata (TMDB, OMDB, etc.) | 7 days | `MetadataCacheTTL` |
| Thumbnails | 30 days | `ThumbnailCacheTTL` |
| API responses | 1 hour | `APICacheTTL` |
| Translations | 30 days | `TranslationCacheTTL` |
| Subtitles | 7 days | `SubtitleCacheTTL` |
| Lyrics | 14 days | `LyricsCacheTTL` |
| Cover art | 30 days | `CoverArtCacheTTL` |

**Size Limits:**

| Limit | Value | Constant |
|-------|-------|----------|
| Max total cache entries | 100,000 | `MaxCacheEntriesTotal` |
| Max entries per table | 25,000 | `MaxCacheEntriesPerTable` |
| Size check interval | Every 100 operations | `CacheSizeCheckInterval` |

**Cleanup:**

- Automatic cleanup runs every **1 hour** (`CacheCleanupInterval`)
- Cleanup goroutine starts in `NewCacheService()` constructor
- Uses `sync.Once` for safe double-close
- Tests must call `defer service.Close()` to avoid goroutine leaks
- Cleanup context has a 5-minute timeout and is cancelled on shutdown

### 5.2 Database Cache Tables

Three cache tables are created in migration v11:

| Table | Purpose | Key Field |
|-------|---------|-----------|
| `cache_entries` | General-purpose cache | `cache_key` (UNIQUE) |
| `api_cache` | API response cache | `cache_key` (UNIQUE) |
| `media_metadata_cache` | Metadata provider cache | `cache_key` (UNIQUE) |

All cache tables include `expires_at` columns with indexes for efficient cleanup.

### 5.3 Redis Cache (Optional)

Redis is used for distributed rate limiting when available. Connection is configured via environment variables:

```bash
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
```

The application falls back to in-memory rate limiting if Redis is unavailable. The Redis connection is tested at startup with `Ping()`.

### 5.4 Catalog-Level Cache Config

```json
{
  "catalog": {
    "enable_cache": true,
    "cache_ttl_minutes": 15,
    "default_page_size": 100,
    "max_page_size": 1000
  }
}
```

| Key | Default | Description |
|-----|---------|-------------|
| `enable_cache` | true | Enable response caching |
| `cache_ttl_minutes` | 15 | Default cache TTL for catalog responses |
| `default_page_size` | 100 | Default items per page |
| `max_page_size` | 1000 | Maximum items per page |

---

## 6. HTTP/3 and Compression Settings

### 6.1 HTTP/3 (QUIC)

Catalogizer runs an HTTP/3 server alongside the standard HTTP/1.1+HTTP/2 server using `quic-go/http3`.

**Architecture:**

| Protocol | Port | Transport | Library |
|----------|------|-----------|---------|
| HTTP/1.1 + HTTP/2 | 8080 (configurable) | TCP | Go net/http |
| HTTPS + HTTP/2 | 8443 | TCP + TLS | Go net/http |
| HTTP/3 | 8443 | UDP + QUIC | quic-go/http3 |

**TLS Configuration:**

- Self-signed TLS certificates are generated at startup and cached across restarts
- `NextProtos`: `["h3", "h2", "http/1.1"]`
- `Alt-Svc` header advertises HTTP/3 support: `h3=":8443"; ma=86400`

**Server Timeouts (from config defaults):**

| Timeout | Default | Config Key |
|---------|---------|-----------|
| Read timeout | 900 seconds | `server.read_timeout` |
| Write timeout | 900 seconds | `server.write_timeout` |
| Idle timeout | 120 seconds | `server.idle_timeout` |

The write timeout must be set to 900 (not 30) to accommodate long-running challenge `RunAll` operations.

### 6.2 Compression Middleware

Response compression is handled by `internal/middleware/compression.go` using both Brotli and Gzip with pooled writers.

**Default Configuration (`DefaultCompressionConfig()`):**

| Setting | Value | Description |
|---------|-------|-------------|
| MinSize | 1024 bytes (1KB) | Responses smaller than this are not compressed |
| BrotliLevel | Default | `brotli.DefaultCompression` |
| GzipLevel | Default | `gzip.DefaultCompression` |
| ExcludedContentTypes | `image/`, `video/`, `audio/`, `application/octet-stream` | Already compressed binary formats |
| ExcludedPaths | `/metrics` | Prometheus endpoint excluded |

**Compression Priority:**

1. **Brotli** (preferred) -- if client sends `Accept-Encoding: br`
2. **Gzip** (fallback) -- if client sends `Accept-Encoding: gzip`
3. **None** -- if neither is supported or response is below MinSize

**Writer Pooling:**

Both Brotli and Gzip writers are pooled using `sync.Pool` to reduce garbage collection pressure under high load.

---

## 7. Rate Limiting Configuration

### 7.1 Three-Tier Rate Limiting

Rate limiting is configured in `main.go` using the internal auth middleware. Three tiers protect different endpoint categories:

| Tier | Rate Limit | Endpoints | Purpose |
|------|-----------|-----------|---------|
| Login | 30 rpm per IP | `/auth/login`, `/auth/register` | Brute-force protection |
| Auth | 600 rpm per IP | `/auth/refresh`, `/auth/logout`, `/auth/me`, `/auth/status`, `/auth/permissions`, `/auth/profile` | Token operations |
| Default | 2000 rpm per IP | All `/api/v1/*` routes | General API traffic |

### 7.2 Rate Limiter Implementation

```go
// From main.go
loginRateLimiter := authMiddleware.RateLimitByUser(30, "1m")
authRateLimiter := authMiddleware.RateLimitByUser(600, "1m")
defaultRateLimiter := authMiddleware.RateLimitByUser(2000, "1m")
```

The `RateLimitByUser()` function creates per-IP rate limiters using a sliding window algorithm. Rate limiters are keyed by client IP address.

### 7.3 Redis-Based Distributed Rate Limiting

When Redis is available, distributed rate limiting can be enabled for multi-instance deployments. The code in `main.go` includes commented-out configuration for Redis-based limiters:

```go
// Fixed window:
authRateLimiter = root_middleware.RedisRateLimit(
    root_middleware.AuthRedisRateLimiterConfig(redisClient))

// Sliding window (more accurate under burst traffic):
authRateLimiter = root_middleware.SlidingWindowRedisRateLimit(
    root_middleware.AuthRedisRateLimiterConfig(redisClient))
```

### 7.4 Rate Limit Tuning

- The login rate limit (30 rpm) allows the challenge runner to re-authenticate at ~1 rps during a full bank run while still blocking brute-force attacks
- The auth rate limit (600 rpm) accommodates the challenge runner which hits token endpoints heavily
- The default rate limit (2000 rpm) provides generous headroom for normal API usage
- All rate limiter cleanup goroutines are stopped during graceful shutdown via `root_middleware.StopAll()`

---

## 8. Resource Limits

### 8.1 GOMAXPROCS

The host runs other mission-critical processes, so resource consumption must stay under 30-40% of total resources.

**For tests:**

```bash
GOMAXPROCS=3 go test ./... -p 2 -parallel 2
```

**For production:** Set `GOMAXPROCS` based on container CPU allocation. If running with `--cpus=2`, set `GOMAXPROCS=2`.

### 8.2 Container Resource Limits

Mandatory `podman run` flags for all containers:

| Component | CPU Limit | Memory Limit |
|-----------|----------|--------------|
| PostgreSQL | `--cpus=1` | `--memory=2g` |
| catalog-api | `--cpus=2` | `--memory=4g` |
| catalog-web | `--cpus=1` | `--memory=2g` |
| Builder | `--cpus=3` | `--memory=8g` |

**Total budget:** max 4 CPUs, 8 GB RAM across all running containers.

### 8.3 Scanner Concurrency

```json
{
  "catalog": {
    "max_concurrent_scans": 3,
    "scanner_concurrency": 4,
    "download_chunk_size": 1048576,
    "max_archive_size": 5368709120
  }
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `max_concurrent_scans` | 3 | Maximum simultaneous storage root scans |
| `scanner_concurrency` | 4 | Parallel file processing within a scan |
| `download_chunk_size` | 1MB | Streaming download chunk size |
| `max_archive_size` | 5GB | Maximum archive download size |

### 8.4 Monitoring Resource Usage

```bash
# Container resource usage
podman stats --no-stream

# System load average
cat /proc/loadavg

# Memory pressure
free -h
```

---

## 9. Monitoring Setup

### 9.1 Prometheus Metrics

Catalogizer exposes Prometheus metrics at `/metrics`. The Prometheus server scrapes this endpoint.

**Prometheus Configuration** (`monitoring/prometheus/prometheus.yml`):

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'catalog-api'
    static_configs:
      - targets: ['catalog-api:9090']
    metrics_path: '/metrics'

  - job_name: 'node-exporter'
    static_configs:
      - targets: ['node-exporter:9100']
```

Runtime metrics are collected every 15 seconds (goroutines, memory) via `metrics.StartRuntimeCollector(15 * time.Second)` in `main.go`.

### 9.2 Alert Rules

Alert rules are defined in `monitoring/prometheus/alerts.yml`:

| Alert | Condition | Severity | Duration |
|-------|-----------|----------|----------|
| HighErrorRate | 5xx rate > 5% of total | critical | 5 min |
| HighLatency | P95 latency > 1 second | warning | 5 min |
| LowCacheHitRatio | Cache hit ratio < 50% | warning | 10 min |
| HighMemoryUsage | Memory > 4GB | warning | 5 min |
| TooManyGoroutines | Goroutine count > 1000 | warning | 5 min |
| DatabaseErrors | DB error rate > 0.1/sec | critical | 2 min |

### 9.3 Grafana Dashboards

Two pre-built dashboards in `monitoring/grafana/dashboards/`:

**catalogizer-overview.json** -- Main operational dashboard:
- HTTP Request Rate (by method and path)
- HTTP Request Duration (p50, p95, p99)
- Active Connections
- Error rates

**catalogizer-runtime.json** -- Go runtime dashboard:
- Goroutine count
- Memory allocation
- GC pause times

### 9.4 Alertmanager

Alertmanager is configured at `monitoring/alertmanager/alertmanager.yml` and receives alerts from Prometheus on port 9093.

### 9.5 Running the Monitoring Stack

```bash
# Start monitoring services
podman-compose -f docker-compose.dev.yml up prometheus grafana alertmanager node-exporter

# Access:
# Prometheus: http://localhost:9090
# Grafana:    http://localhost:3001
# Alertmanager: http://localhost:9093
```

---

## 10. Performance Benchmarking

### 10.1 k6 Load Test Scripts

15 k6 test scripts are available in `tests/k6/`:

| Script | Purpose | Key Parameters |
|--------|---------|---------------|
| `load_test.js` | Standard load test | Ramp to 50 users, verify p95 < 500ms |
| `stress_test.js` | Find breaking point | Ramp to 300 users |
| `soak_test.js` | Memory leak detection | 20 users for 30 minutes |
| `spike_test.js` | Sudden traffic spikes | Quick ramp to high load |
| `breakpoint_test.js` | Find absolute limits | Incremental load increase |
| `endurance_test.js` | Long-running stability | Extended duration test |
| `auth_load_test.js` | Authentication endpoints | Login/register under load |
| `entity_browse_load_test.js` | Media browsing | Browse/search operations |
| `media_scan_stress_test.js` | Scan operations | Concurrent scan triggers |
| `database_stress_test.js` | Database operations | Direct DB query patterns |
| `concurrent_writers_test.js` | Write contention | Concurrent write operations |
| `websocket_stress_test.js` | WebSocket connections | Many concurrent WS clients |
| `mixed_workload_test.js` | Realistic traffic mix | Combined read/write/search |
| `ddos_ratelimit_test.js` | Rate limit verification | Floods, bursts, slowloris |
| `monitoring_test.js` | Metrics endpoint | Metrics under load |

### 10.2 Running Load Tests

```bash
# Run a specific test via Podman (resource-limited)
podman run --rm --network host \
  -v $(pwd)/tests/k6:/scripts \
  docker.io/grafana/k6:latest run /scripts/load_test.js

# Run stress test
podman run --rm --network host \
  -v $(pwd)/tests/k6:/scripts \
  docker.io/grafana/k6:latest run /scripts/stress_test.js

# Run soak test (30 minutes)
podman run --rm --network host \
  -v $(pwd)/tests/k6:/scripts \
  docker.io/grafana/k6:latest run /scripts/soak_test.js
```

### 10.3 Establishing Baselines

1. Run `load_test.js` against a clean database with known seed data
2. Record p50, p95, p99 latency for each endpoint category
3. Record throughput (requests/second) at target concurrency
4. Record memory usage before and after the test
5. Store results as the baseline for regression detection

### 10.4 Detecting Regressions

Compare new test results against baselines:

- **P95 latency increase > 20%** -- investigate query plans, new middleware, or changed data volume
- **Throughput decrease > 15%** -- check for added synchronization, new allocations, or lock contention
- **Memory growth during soak test** -- indicates goroutine or resource leak (use `pprof` to diagnose)
- **Error rate increase** -- check logs for database errors, timeout changes, or rate limit misconfigurations

### 10.5 Profiling Tools

```bash
# CPU profile (30 seconds)
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine dump
curl http://localhost:8080/debug/pprof/goroutine?debug=1

# Block profile (contention)
go tool pprof http://localhost:8080/debug/pprof/block

# Generate PDF flame graph
go tool pprof -pdf http://localhost:8080/debug/pprof/heap > heap.pdf
```

---

## 11. Recommended Configurations

### 11.1 Development

Optimized for fast iteration with minimal resource usage.

```json
{
  "server": {
    "host": "localhost",
    "port": 8080,
    "read_timeout": 900,
    "write_timeout": 900,
    "idle_timeout": 120,
    "enable_cors": true,
    "enable_https": true
  },
  "database": {
    "type": "sqlite",
    "path": "./catalog.db",
    "enable_wal": true,
    "cache_size": -2000,
    "busy_timeout": 5000,
    "max_open_connections": 10,
    "max_idle_connections": 5,
    "conn_max_lifetime": 300,
    "conn_max_idle_time": 60
  },
  "catalog": {
    "enable_cache": true,
    "cache_ttl_minutes": 5,
    "max_concurrent_scans": 2,
    "scanner_concurrency": 2,
    "default_page_size": 50,
    "max_page_size": 500
  },
  "logging": {
    "level": "debug",
    "format": "json",
    "output": "stdout"
  }
}
```

```bash
# Run with resource limits
GOMAXPROCS=3 go run main.go
```

### 11.2 Staging

Mirrors production settings with lower resource limits for testing.

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": 900,
    "write_timeout": 900,
    "idle_timeout": 120,
    "enable_cors": true,
    "enable_https": true
  },
  "database": {
    "type": "postgres",
    "host": "localhost",
    "port": 5433,
    "name": "catalogizer",
    "user": "catalogizer",
    "ssl_mode": "disable",
    "max_open_connections": 25,
    "max_idle_connections": 10,
    "conn_max_lifetime": 300,
    "conn_max_idle_time": 180
  },
  "catalog": {
    "enable_cache": true,
    "cache_ttl_minutes": 15,
    "max_concurrent_scans": 3,
    "scanner_concurrency": 4,
    "default_page_size": 100,
    "max_page_size": 1000
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout"
  }
}
```

Container limits:

```bash
podman run --cpus=2 --memory=4g catalog-api
podman run --cpus=1 --memory=2g postgres
```

### 11.3 Production

Full resource allocation with all security features enabled.

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": 900,
    "write_timeout": 900,
    "idle_timeout": 120,
    "enable_cors": true,
    "enable_https": true
  },
  "database": {
    "type": "postgres",
    "host": "db.internal",
    "port": 5432,
    "name": "catalogizer",
    "user": "catalogizer",
    "ssl_mode": "require",
    "max_open_connections": 50,
    "max_idle_connections": 20,
    "conn_max_lifetime": 600,
    "conn_max_idle_time": 300
  },
  "auth": {
    "enable_auth": true,
    "jwt_expiration_hours": 24
  },
  "catalog": {
    "enable_cache": true,
    "cache_ttl_minutes": 30,
    "max_concurrent_scans": 3,
    "scanner_concurrency": 4,
    "default_page_size": 100,
    "max_page_size": 1000
  },
  "logging": {
    "level": "warn",
    "format": "json",
    "output": "stdout",
    "max_size": 100,
    "max_backups": 3,
    "max_age": 28,
    "compress": true
  }
}
```

Environment variables (never in config file):

```bash
JWT_SECRET=<at-least-32-character-secret>
ADMIN_USERNAME=<admin-user>
ADMIN_PASSWORD=<admin-password>
DATABASE_PASSWORD=<production-password>
REDIS_ADDR=redis.internal:6379
```

Container limits:

```bash
podman run --cpus=2 --memory=4g --network host \
  --add-host=nas.local:192.168.0.241 \
  catalog-api

podman run --cpus=1 --memory=2g \
  -p 5433:5432 \
  postgres:16
```

### 11.4 Configuration Checklist

- [ ] Database connection pool sized for expected concurrency
- [ ] SQLite WAL mode verified (check logs for "WAL mode enabled")
- [ ] PostgreSQL indexes verified (`\di` in psql)
- [ ] Compression middleware active (Brotli + Gzip)
- [ ] Rate limiting configured for all tiers (login/auth/default)
- [ ] HTTP/3 server running on port 8443
- [ ] Container CPU and memory limits set
- [ ] GOMAXPROCS set for test runs
- [ ] Prometheus scraping metrics endpoint
- [ ] Grafana dashboards imported
- [ ] Alert rules configured and tested
- [ ] k6 baseline tests run and results stored
- [ ] Cache TTLs appropriate for data freshness requirements
- [ ] `write_timeout` set to 900 for challenge RunAll support
