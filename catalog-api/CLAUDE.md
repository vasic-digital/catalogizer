# CLAUDE.md — catalog-api

## Overview

Go 1.25 REST API built with Gin. Serves as the backend for all Catalogizer clients (web, desktop, Android, Android TV). Provides media browsing, search, recognition, recommendations, deep linking, and a reading experience across multi-protocol storage (SMB, FTP, NFS, WebDAV, local).

## Commands

```bash
# Development
go run main.go                              # starts server, writes port to .service-port
go build -o catalog-api                     # dev-only; release builds run in a rootless container (§11.4.173)

# Testing (resource-limited — host runs other critical processes)
GOMAXPROCS=3 go test ./... -p 2 -parallel 2 # all tests
go test -v -run TestName ./path/to/pkg/     # single test

# Lint / vet
go vet ./...
```

## Architecture

**Handler -> Service -> Repository -> Database (SQLite/PostgreSQL)**

Routes are registered under `/api/v1` in `main.go`. The Gin engine is the HTTP framework.

### Dual-Package Layout

- **Top-level packages** (`handlers/`, `services/`, `repository/`, `middleware/`, `models/`): Domain logic — catalog browsing, media entities, search, downloads, storage operations.
- **Internal packages** (`internal/handlers/`, `internal/services/`, `internal/middleware/`): Infrastructure concerns — auth, metrics, WebSocket, cache, media detection pipeline, SMB circuit breaker, lifecycle management.

### Key Directories

| Directory | Purpose |
|---|---|
| `handlers/` | Domain HTTP handlers (catalog, media entities, search, downloads) |
| `services/` | Domain business logic |
| `repository/` | Data access layer (CRUD for files, media items, collections) |
| `middleware/` | Domain middleware (CORS, logging) |
| `models/` | Shared data structures |
| `database/` | Connection management, dialect abstraction, migrations |
| `filesystem/` | Unified multi-protocol client (`interface.go` defines `UnifiedClient`, `factory.go` creates per-protocol clients) |
| `challenges/` | Challenge bank definitions, registration, and config |
| `internal/auth/` | JWT authentication with role-based access |
| `internal/services/` | Aggregation, title parsing, scanning, media detection pipeline |
| `internal/media/` | Detection (`detector/`) -> analysis (`analyzer/`) -> providers (TMDB, IMDB, etc.) |
| `internal/smb/` | Circuit breaker, offline cache, exponential backoff retry |
| `internal/metrics/` | Prometheus metrics (exposed at `/metrics`) |
| `internal/lifecycle/` | `LazyServiceRegistry` for deferred service init with dependency ordering |
| `internal/concurrency/` | Semaphore-based concurrency control |
| `internal/httpclient/` | Pooled HTTP client with connection reuse and retry |
| `config/` | Configuration loading (env vars > `.env` > `config.json` > defaults) |

### Database Dialect Abstraction

Dual-dialect support for SQLite (dev) and PostgreSQL (production) in `database/dialect.go`:

- `RewritePlaceholders()` — `?` -> `$1, $2, ...` for PostgreSQL
- `RewriteInsertOrIgnore()` — `INSERT OR IGNORE` -> `ON CONFLICT DO NOTHING`
- `BooleanLiterals()` — `= 0/1` -> `= FALSE/TRUE` for known boolean columns
- `database.DB` wraps `*sql.DB` with shadowed `Exec()`, `Query()`, `QueryRow()` that auto-rewrite SQL
- `InsertReturningID()` and `TxInsertReturningID()` replace `LastInsertId()` (PostgreSQL uses `RETURNING id`)

Migrations live in `database/migrations/` with separate SQLite and PostgreSQL variants.

### Dynamic Port Binding

On startup, the server writes its chosen port to `.service-port`. The frontend reads this file to configure its API proxy target.

### HTTP/3 (QUIC)

Uses `quic-go/http3` with self-signed TLS certs generated at startup. Brotli compression via `andybalholm/brotli` middleware.

## Testing Conventions

- **Table-driven tests**: Standard Go pattern with subtests via `t.Run()`.
- **Test files**: `*_test.go` beside the source file they test.
- **Test helper**: `internal/tests/test_helper.go` provides in-memory SQLite setup via `database.WrapDB(sqlDB, DialectSQLite)`.
- **Constructor injection**: Services use `NewService(deps...)` — pass mocks in tests.
- **Error wrapping**: Use `fmt.Errorf("context: %w", err)` for wrapped errors.

## Constraints

- **Resource limits**: `GOMAXPROCS=3`, `-p 2 -parallel 2` for tests. Never exceed 30-40% of host CPU/RAM.
- **Container builds**: Use `podman build --network host`. Set `GOTOOLCHAIN=local`. Use fully qualified image names.
- **Zero-warning policy**: No console errors, no failed network requests, no deprecation warnings.
- **API keys**: Never commit `.env` files or hardcode secrets. Use `.env.example` with placeholders.
- **Config precedence**: env vars > `.env` > `config.json` > defaults.
- **Concurrency safety**: `CacheService` and `WebSocketHandler` spawn goroutines — tests must call `defer service.Close()` / `handler.Stop()`.
- **SQLite WAL mode**: Explicit `PRAGMA journal_mode=WAL` after connection in `database/connection.go`.
- **Connection pool defaults**: MaxOpen=25, MaxIdle=10, MaxLifetime=5m, MaxIdleTime=3m.


## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** use `su` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Container-Based Solutions
When a build or runtime environment requires system-level dependencies, use containers instead of elevation:

- **Use the `Containers` submodule** (`https://github.com/vasic-digital/Containers`) for containerized build and runtime environments
- **Add the `Containers` submodule as a Git dependency** and configure it for local use within the project
- **Build and run inside containers** to avoid any need for privilege escalation
- **Rootless Podman/Docker** is the preferred container runtime

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo` or `su`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Use the `Containers` submodule for containerized builds
5. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**


