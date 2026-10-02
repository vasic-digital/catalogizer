# AGENTS.md — catalog-api Multi-Agent Coordination Guide

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working on the `catalog-api` Go backend. It defines responsibilities, package boundaries, and coordination protocols to prevent conflicts when multiple agents operate concurrently on this module.

## Module Identity

- **Module path**: `catalogizer`
- **Language**: Go 1.25
- **Framework**: Gin (HTTP), sqlcipher/sqlite + PostgreSQL (dual dialect), QUIC/HTTP3 via quic-go
- **Depends on**: `digital.vasic.*` submodules via `replace` directives in `go.mod`
- **Entrypoint**: `main.go`

## Package Ownership Boundaries

### Top-level domain packages

| Package | Scope | Boundary |
|---|---|---|
| `handlers/` | Domain HTTP handlers (catalog, media entities, search, downloads) | Do NOT mix auth/metrics/cache logic here — those live in `internal/`. |
| `services/` | Domain business logic | Services own the repository layer they mutate — do not reach into another service's repo. |
| `repository/` | Data access (CRUD for files, media items, collections) | Goes through `database.DB` wrapper (never raw `*sql.DB`). |
| `middleware/` | Domain middleware (CORS, logging, rate limiting) | Factories must register cleanup goroutines with `middleware.StopAll()` — never leak at process exit. |
| `models/` | Shared data structures | Keep dependency-free; no imports from `handlers/` or `services/`. |
| `database/` | Dialect abstraction, connection management, migrations | Migration dispatch in `migrations.go`; per-dialect functions in `migrations_postgres.go` / `migrations_sqlite.go`. |
| `filesystem/` | Unified multi-protocol client | Add new protocols by implementing `UnifiedClient`. |
| `challenges/` | Challenge bank definitions, registration | Register new groups in `RegisterAll()` in `register.go`. |

### Internal infrastructure packages

| Package | Scope | Boundary |
|---|---|---|
| `internal/auth/` | JWT authentication, role-based access | Do not duplicate JWT logic in top-level middleware. |
| `internal/services/` | Aggregation, title parsing, scanning, media detection pipeline | The media pipeline: `internal/media/detector/` → `analyzer/` → `providers/`. |
| `internal/media/providers/` | External metadata (TMDB, OMDB, MusicBrainz, OpenLibrary, IGDB, …) | Providers MUST degrade gracefully — missing API keys never block the pipeline. New providers register in `NewProviderManager()`. |
| `internal/metrics/` | Prometheus metrics exposed at `/metrics` | HTTP instrumentation is `GinMiddleware()`; runtime stats via `StartRuntimeCollector()`. Snapshot helpers in `snapshot.go` feed `services/reporting_service.go`. |
| `internal/lifecycle/` | `LazyServiceRegistry` for deferred service init | Use this for any service that depends on another service's eventual construction. |
| `internal/concurrency/` | Semaphore-based bounded parallelism | Use semaphores here; do not hand-roll buffered channels for concurrency limits. |
| `internal/smb/` | Circuit breaker, offline cache, exponential-backoff retry | Uses `net.DialTimeout` with a bounded 5 s connect — never raw `net.Dial`. |
| `internal/httpclient/` | Pooled HTTP client | Always use this instead of `http.DefaultClient` (which has no timeout). |

## Dependency Graph

```
main.go
 ├── handlers/                  (domain)
 ├── services/                  (domain)
 ├── repository/                (domain)
 ├── middleware/                (domain)
 ├── models/                    (pure data)
 ├── database/                  (dual-dialect wrapper)
 ├── filesystem/                (protocol abstraction)
 ├── challenges/                (challenge registration)
 └── internal/
      ├── auth/
      ├── services/             (infra services)
      ├── media/
      │    ├── detector/
      │    ├── analyzer/
      │    └── providers/
      ├── metrics/
      ├── middleware/           (infra middleware)
      ├── handlers/             (infra handlers)
      ├── lifecycle/
      ├── concurrency/
      ├── smb/
      ├── httpclient/
      └── …
```

No package inside `internal/` may import from the top-level domain packages except for `models/`. Top-level packages freely import from `internal/`.

## Agent Coordination Rules

### 1. Database mutations

If you modify migrations:
- Add a new Version + Name entry to `migrations.go` RunMigrations().
- Add both Postgres and SQLite implementations in `migrations_postgres.go` and `migrations_sqlite.go`.
- Wire the dispatch function (`if db.dialect.IsPostgres() { ... } else { ... }`) in `migrations.go`.
- Add a reference `.up.sql` and `.sqlite.up.sql` pair in `database/migrations/` for CLI-tool compatibility.
- Run `TestRunMigrationsSQLite` in `database/migrations_parity_test.go` — it must stay green.

If you modify schemas:
- Never modify an existing migration after it's shipped. Create a new version.
- Use `INSERT OR IGNORE` → auto-rewritten to `ON CONFLICT DO NOTHING`.
- Use `?` placeholders → auto-rewritten to `$1, $2, …` on Postgres.
- Use `InsertReturningID()` instead of `LastInsertId()`.

### 2. Handler additions

If you add a new route:
- Register it in `main.go`'s `setupRouter()` (or the appropriate route group helper).
- Add a unit test in `handlers/` or `internal/handlers/` that hits the route via `httptest.Server`.
- If the route needs auth, wrap with the existing `authMiddleware` — do not hand-roll auth checks inside the handler.
- If the route is rate-limited, use per-route rate limiting configured in `main.go` or `middleware.RedisRateLimit(...)` for Redis-backed limiting.

### 3. Concurrency & goroutines

Services spawning goroutines must:
- Spawn with `wg.Add(1)` BEFORE `go func()`, not inside it.
- Use `sync.Once` for cleanup (`Stop()` / `Close()`).
- Expose a shutdown method callable from `main.go`'s shutdown sequence.
- Provide tests that call `defer service.Close()` / `handler.Stop()` — otherwise `go test -race` will flag the goroutine leak.

Required shutdown order in `main.go`:
1. `metrics.StopRuntimeCollector()`
2. `wsHandler.Stop()`
3. `cacheService.Close()`
4. `mediaEntityHandler.Close()`
5. `logAdapter.Close()`
6. `root_middleware.StopAll()` (drains rate-limiter registry)
7. `srv.Shutdown(shutdownCtx)` (HTTP server)

### 4. Adding a new metadata provider

1. Implement `MetadataProvider` interface in `internal/media/providers/providers.go`.
2. Add a constructor `NewXProvider(client, logger)`.
3. Register in `NewProviderManager()` via `lazyProviders["x"] = NewLazyProvider(...)`.
4. Graceful degradation: if the API key is missing, the provider must return an empty result with no error — never block the pipeline.

### 5. Adding a new challenge group

1. Create `challenges/userflow_x.go` (or similar).
2. Define challenge structs embedding `challenge.BaseChallenge`.
3. Expose a `RegisterUserFlowXChallenges(reg challenge.Registry) error` function.
4. Call it from `challenges/register.go`'s `RegisterAll()`.
5. Never write "passes as stub" results — return `challenge.StatusSkipped` with a structured reason when infrastructure is unavailable.

### 6. Testing standards

- **Race detector**: every concurrency-touching change must run `GOMAXPROCS=3 go test -race ./path/... -p 2 -parallel 2`.
- **Table-driven tests**: use `t.Run(name, func(t *testing.T) { ... })` subtests.
- **Test DB**: use `database.WrapDB(sqlDB, DialectSQLite)` for in-memory SQLite.
- **Mock HTTP**: `httptest.NewServer` for external service mocks.
- **Coverage gate**: package-level coverage must not drop below the current baseline. Check with `go test -cover ./...`.

### 7. Error handling

- Wrap with `fmt.Errorf("context: %w", err)`.
- Never `_ = err` silently — log at `Warn` or return it.
- Never `.catch(() => {})` equivalent in Go (empty error check).
- Return typed errors (`errors.New`, `errors.Is`/`errors.As`) for control flow — stringly-typed error matching is fragile.

### 8. Dependencies

- Use `internal/httpclient` — never `http.DefaultClient` (no timeout).
- Use `database.DB` wrapper — never raw `*sql.DB.Exec` (breaks dialect rewriting).
- Import submodules via `digital.vasic.*` module paths, wired via `replace` in `go.mod`.

## File Ownership

| File | Primary Concern | Cross-Package Impact |
|------|----------------|---------------------|
| `main.go` | Entry point, route registration, shutdown sequence | ALL packages — touches every service constructor |
| `database/migrations.go` | Migration dispatch | Affects every service that reads these tables |
| `database/dialect.go` | SQL rewriting rules | Affects every SQL call through `database.DB` |
| `filesystem/interface.go` | `UnifiedClient` protocol | Affects every protocol client |
| `challenges/register.go` | Challenge registration | Affects challenge runner output |
| `internal/metrics/snapshot.go` | Metrics snapshot helpers | Read by `services/reporting_service.go` |

## Build & Validation Commands

```bash
# Format + vet + build + test
go fmt ./...
go vet ./...
go build ./...
GOMAXPROCS=3 go test -race ./... -p 2 -parallel 2

# Single package
GOMAXPROCS=3 go test -race ./services/ -count=1

# Race detector across the whole tree (catalog-api + Go submodules)
../scripts/run-race-detector.sh --fast     # catalog-api only
../scripts/run-race-detector.sh --all      # + all submodules

# Coverage sweep
go test -cover ./... -count=1
```

## Commit Conventions

Conventional Commits:
- `feat(services): add sync rollup report`
- `fix(smb): bound dial timeout`
- `test(metrics): add snapshot coverage`
- `refactor(handlers): extract search filter`
- `docs(agents): clarify goroutine shutdown`

Every commit must:
- Pass `go fmt`, `go vet`, `go build`.
- Pass affected package tests under `-race`.
- End with the Co-Authored-By trailer when authored with an AI assistant.

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


## ⚠️ MANDATORY: Zero Unfinished Work

No TODOs, FIXMEs, empty implementations, silent error swallows, fake data, or `_ = err` patterns may be committed. The local anti-bluff scan and the dedicated commit/push validation script report them; no CI/CD pipeline is used (§11.4.156, §11.4.234). When an issue is found, fix all instances — not just the reported one.
