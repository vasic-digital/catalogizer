---
title: Contributing
description: How to set up the development environment, follow coding conventions, and contribute to Catalogizer
---

# Contributing

This guide covers everything needed to contribute to Catalogizer: repository setup, development environment, coding conventions, testing requirements, and the contribution process.

---

## Repository Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Backend (catalog-api) |
| Node.js | 18+ | Frontend (catalog-web), TypeScript submodules |
| Rust | Latest stable | Desktop apps (Tauri) |
| Kotlin / JDK | JDK 21 (with jvmToolchain 17) | Android apps |
| Git | 2.x+ | Version control with submodule support |
| Podman | 5+ | Container runtime (rootless, no Docker required) |

### Clone and Initialize

All git access uses SSH. Never use HTTPS URLs.

```bash
git clone git@github.com:vasic-digital/Catalogizer.git
cd Catalogizer
scripts/safe_submodule_init.sh   # never `--recursive` through constitution/ (see docs/GOVERNANCE.md)
```

The project contains 41 git submodules. The initial clone pulls all of them recursively.

### Submodule Architecture

Go modules use `replace` directives in `catalog-api/go.mod` to reference local submodule paths:

```go
replace digital.vasic.challenges => ../Challenges
replace digital.vasic.database => ../Database
replace digital.vasic.filesystem => ../Filesystem
```

TypeScript modules use `file:../` references in `catalog-web/package.json`:

```json
{
  "@vasic-digital/websocket-client": "file:../WebSocket-Client-TS",
  "@vasic-digital/ui-components": "file:../UI-Components-React"
}
```

### Updating Submodules

```bash
# Sync all submodules to their latest tracked branches
# CAUTION: --remote moves third-party pins to upstream tips without review; inspect the gitlink diff before committing
git submodule status | awk '$2 != "constitution" {print $2}' | xargs -r git submodule update --remote --recursive --

# Commit changes within a submodule
cd Challenges
git add . && git commit -m "description"
git push origin main

# Update the parent repo's submodule reference
cd ..
git add Challenges
git commit -m "chore(submodules): update Challenges to latest"
```

---

## Development Environment

### Starting the Backend

```bash
cd catalog-api
cp .env.example .env
# Edit .env: set JWT_SECRET and ADMIN_PASSWORD
go run main.go
```

The backend creates an SQLite database automatically and writes its port to `.service-port`. No external database setup is required for development.

### Starting the Frontend

```bash
cd catalog-web
npm install
npm run dev
```

The frontend runs on port 3000 and reads `.service-port` to proxy API requests to the backend. Make sure nothing else is running on port 3000 before starting.

### Using Containers

For a full-stack environment with PostgreSQL and Redis:

```bash
cp .env.example .env
# Edit .env: set POSTGRES_PASSWORD and JWT_SECRET
podman-compose -f docker-compose.dev.yml up
```

Container resource limits are mandatory:
- PostgreSQL: `--cpus=1 --memory=2g`
- catalog-api: `--cpus=2 --memory=4g`
- catalog-web: `--cpus=1 --memory=2g`
- Total budget: max 4 CPUs, 8 GB RAM across all running containers

---

## Coding Conventions

### Go Backend

- **Constructors**: Use `NewService(deps...)` pattern with dependency injection
- **Error handling**: Wrap errors with context using `fmt.Errorf("operation: %w", err)`. Use `errors.New` for static errors. Never expose internal details to clients.
- **Naming**: PascalCase for exported types, camelCase for unexported. Single-letter receivers (`s *Service`, `h *Handler`, `r *Repository`).
- **Imports**: Three groups separated by blank lines -- stdlib, third-party, local
- **Testing**: Table-driven tests with `t.Run()`. Test files sit beside source files with `_test.go` suffix.
- **Database**: Use `?` placeholders (auto-converted to `$1, $2...` for PostgreSQL). Use `InsertReturningID()` instead of `LastInsertId()`.
- **Concurrency**: Services spawning goroutines use `sync.Once` for cleanup. Tests must `defer service.Close()`.

```go
// Constructor pattern
func NewMediaService(repo MediaRepository, logger Logger) *MediaService {
    return &MediaService{repo: repo, logger: logger}
}

// Error wrapping
func (s *MediaService) GetByID(id int64) (*Media, error) {
    media, err := s.repo.FindByID(id)
    if err != nil {
        return nil, fmt.Errorf("get media by id %d: %w", id, err)
    }
    return media, nil
}

// Table-driven test
func TestMediaService_GetByID(t *testing.T) {
    tests := []struct {
        name    string
        id      int64
        wantErr bool
    }{
        {"valid id", 1, false},
        {"not found", 999, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := svc.GetByID(tt.id)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, result)
            }
        })
    }
}
```

### TypeScript / React Frontend

- **Components**: PascalCase for component files and names
- **Functions**: camelCase for utility functions and hooks
- **Constants**: SCREAMING_SNAKE_CASE
- **Imports**: Three groups -- React, third-party, local path aliases (`@/components`, `@/hooks`, `@/lib`, etc.)
- **State**: React Query for server state, Zustand for client state
- **Forms**: React Hook Form with Zod resolvers for validation
- **Styling**: Tailwind CSS utility classes via `cn()` from `@/lib/utils`
- **Linting**: ESLint with `@typescript-eslint`, enforced with `--max-warnings 0`
- **Testing**: Vitest + React Testing Library for unit tests, Playwright for E2E

### Kotlin / Android

- **Architecture**: MVVM with Compose UI, ViewModel (StateFlow), Repository, Room + Retrofit
- **DI**: Hilt for dependency injection
- **Async**: `suspend` functions, `Flow` / `StateFlow`, Paging 3
- **Error handling**: Sealed `Result` classes for operation outcomes
- **Build**: JDK 21 with `jvmToolchain(17)` and `--add-opens` JVM args for kapt compatibility

---

## Testing Requirements

Catalogizer requires 100% test coverage across ten testing categories. Every contribution must include appropriate tests.

### The Ten Categories

| Category | What It Covers |
|----------|----------------|
| **1. Unit** | Pure logic, individual functions and classes |
| **2. Integration** | Cross-module, database, cache, queues, filesystems |
| **3. E2E** | Full user journeys through the live system |
| **4. Full Automation** | Unattended, reproducible, CI-runnable E2E |
| **5. Stress** | Saturation, concurrency, large payloads, long sessions |
| **6. Security** | AuthN/Z, injection, SSRF, secrets, CVE scans |
| **7. DDoS / Rate-Limit** | Floods, bursts, slowloris, connection exhaustion |
| **8. Benchmarking** | Latency/throughput/memory baselines with regression detection |
| **9. Challenges** | Registered `digital.vasic.challenges` entry per feature |
| **10. HelixQA** | Autonomous bank + session entry per screen and flow |

### Running Tests Before Submitting

```bash
# Backend
cd catalog-api
go fmt ./... && go vet ./...
GOMAXPROCS=3 go test ./... -p 2 -parallel 2

# Frontend
cd catalog-web
npm run lint
npm run type-check
npm run test

# All components
./scripts/run-all-tests.sh
```

### Test Database

Use `database.WrapDB()` for in-memory SQLite test databases:

```go
sqlDB, err := sql.Open("sqlite3", ":memory:")
require.NoError(t, err)
db := database.WrapDB(sqlDB, database.DialectSQLite)
// Run migrations, then test
```

---

## Zero Warning Policy

Catalogizer enforces zero warnings and zero errors across all components:

- No browser console errors or warnings in any environment
- No failed network requests from the frontend
- Every API endpoint the frontend calls must exist and return valid responses
- No framework deprecation warnings
- If a feature is not yet implemented, provide a stub endpoint returning a valid empty response

---

## Database Changes

### Adding Migrations

Migrations live in `catalog-api/database/migrations/`. Each migration has separate SQLite and PostgreSQL variants.

1. Add migration functions in `migrations_sqlite.go` and `migrations_postgres.go`
2. Register them in the migration list with the next version number
3. Test with both SQLite (unit tests) and PostgreSQL (integration)

### Dialect Considerations

Write SQL using SQLite syntax. The dialect layer rewrites it for PostgreSQL automatically:

- Use `?` placeholders (rewritten to `$1, $2, ...`)
- Use `INSERT OR IGNORE` (rewritten to `ON CONFLICT DO NOTHING`)
- Use `0/1` for booleans (rewritten to `FALSE/TRUE` for known boolean columns)
- Use `InsertReturningID()` instead of `LastInsertId()`

---

## Container Runtime

Catalogizer uses Podman (rootless) as the container runtime, with dynamic detection supporting Docker or nerdctl as alternatives.

- Use `podman build --network host` -- default container networking has SSL issues
- Use fully qualified image names (`docker.io/library/...`) -- required for Podman
- Set `GOTOOLCHAIN=local` to prevent Go from auto-downloading newer toolchains
- Set `APPIMAGE_EXTRACT_AND_RUN=1` in containers for Tauri AppImage bundling (no FUSE)

---

## Git Conventions

- All git access uses SSH (`git@github.com:...`), never HTTPS
- The project pushes to six remotes (GitHub x2, GitLab x2, GitFlic, GitVerse)
- GitHub Actions are permanently disabled; all CI/CD runs locally
- Commit messages follow Conventional Commits: `feat(web): add collection sharing`

```bash
# Push to all remotes
GIT_SSH_COMMAND="ssh -o BatchMode=yes" git push origin main
```

---

## Key Constraints

- **No elevated privileges**: All operations run at local-user level. Never use `sudo` or `root`.
- **No unfinished work**: No TODOs, FIXMEs, empty implementations, silent error swallows, or `unwrap()` in committed code.
- **API keys and secrets**: Never commit `.env` files with real keys. Use `.env.example` with placeholder values.
- **Resource limits**: Host workloads must stay under 30-40% of total resources. Go tests use `GOMAXPROCS=3 -p 2 -parallel 2`.
