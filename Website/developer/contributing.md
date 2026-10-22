---
title: Contributing Guide
description: How to set up the development environment, follow coding standards, and contribute to Catalogizer
---

# Contributing Guide

This guide covers how to set up a development environment, follow the project's coding standards, and submit contributions to Catalogizer.

---

## Development Setup

### Prerequisites

- **Go 1.25+** for the backend
- **Node.js 18+** and npm for the frontend
- **Git** with submodule support
- **Podman** (or Docker) for containers

### Clone and Initialize

```bash
git clone <repository-url>
cd Catalogizer
scripts/safe_submodule_init.sh   # never `--recursive` through constitution/ (see docs/GOVERNANCE.md)
```

### Start the Backend

```bash
cd catalog-api
cp .env.example .env
# Edit .env: set JWT_SECRET and ADMIN_PASSWORD
go run main.go
```

The backend creates an SQLite database automatically and writes its port to `.service-port`.

### Start the Frontend

```bash
cd catalog-web
npm install
npm run dev
```

The frontend runs on port 3000 and reads `.service-port` to proxy API requests to the backend.

---

## Project Structure

```
Catalogizer/
  catalog-api/          # Go backend
    handlers/           # Domain HTTP handlers
    services/           # Domain business logic
    repository/         # Data access layer
    middleware/         # Domain middleware
    internal/           # Infrastructure (auth, media, smb, metrics)
    database/           # DB connection, dialect, migrations
    filesystem/         # Storage protocol clients
    challenges/         # Challenge definitions
  catalog-web/          # React frontend
    src/
      components/       # UI components
      pages/            # Route pages
      hooks/            # Custom React hooks
      services/         # API service layer
      store/            # Zustand stores
  Challenges/           # Challenge framework submodule
  Build/                # Build framework submodule
  scripts/              # Shell scripts
  config/               # Infrastructure config (nginx, redis)
  monitoring/           # Prometheus and Grafana configs
  docs/                 # Documentation
```

---

## Coding Standards

### Go

- **Constructors**: Use `NewService(deps)` pattern with dependency injection
- **Error handling**: Wrap errors with context using `fmt.Errorf("operation: %w", err)`
- **Tests**: Table-driven tests in `*_test.go` files beside the source
- **Naming**: Exported types use PascalCase; unexported use camelCase
- **Formatting**: `gofmt` (enforced automatically)

```go
// Constructor pattern
func NewMediaService(repo MediaRepository, logger Logger) *MediaService {
    return &MediaService{repo: repo, logger: logger}
}
```

### TypeScript / React

- **Components**: PascalCase for component files and names
- **Functions**: camelCase for utility functions and hooks
- **Validation**: Zod schemas for runtime validation
- **Forms**: React Hook Form with Zod resolvers
- **State**: React Query for server state, Zustand for client state
- **Styling**: Tailwind CSS utility classes
- **Path aliases**: Use `@/components`, `@/hooks`, `@/lib`, `@/types`, `@/services`, `@/store`, `@/pages`

### Kotlin (Android)

- **Architecture**: MVVM with Compose UI
- **Async**: Kotlin coroutines and StateFlow
- **DI**: Hilt for dependency injection
- **Error handling**: Result sealed classes
- **Database**: Room for local persistence

---

## Submodule Architecture

Catalogizer uses independent git submodules for shared libraries.

### Go Submodules

Go modules reference local submodule paths via `replace` directives in `go.mod`:

```go
replace digital.vasic.challenges => ../Challenges
replace digital.vasic.containers => ../Containers
```

### TypeScript Submodules

TypeScript packages use `file:../` references in `package.json`:

```json
{
  "@vasic-digital/websocket-client": "file:../WebSocket-Client-TS",
  "@vasic-digital/ui-components": "file:../UI-Components-React"
}
```

### Working with Submodules

```bash
# Update all submodules to latest
# CAUTION: --remote moves third-party pins to upstream tips without review; inspect the gitlink diff before committing
git submodule status | awk '$2 != "constitution" {print $2}' | xargs -r git submodule update --remote --recursive --

# Commit changes in a submodule
cd Challenges
git add . && git commit -m "description"
git push origin main

# Then update the parent repo's submodule reference
cd ..
git add Challenges
git commit -m "chore(submodules): update Challenges to latest"
```

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

## Running Tests Before Submitting

Run the full test suite before submitting a contribution:

```bash
# Backend
cd catalog-api
GOMAXPROCS=3 go test ./... -p 2 -parallel 2

# Frontend
cd catalog-web
npm run test
npm run lint
npm run type-check

# All at once
./scripts/run-all-tests.sh
```

---

## Zero Warning Policy

Catalogizer enforces a zero warning, zero error policy across all components:

- No browser console errors or warnings
- No failed network requests from the frontend
- Every API endpoint the frontend calls must exist and return valid responses
- No framework deprecation warnings
- If a feature is not yet implemented, provide a stub endpoint returning a valid empty response

---

## Container Runtime

Catalogizer supports dynamic container runtime detection, automatically selecting the available runtime (Podman preferred, Docker as fallback). Key notes:

- Use `podman build --network host` or `docker build --network host` (default networking has SSL issues)
- Use fully qualified image names (`docker.io/library/...`) -- required for Podman
- Set `GOTOOLCHAIN=local` to prevent Go auto-downloading toolchains
- Enforce resource limits on containers (max 4 CPUs, 8 GB RAM total)
- Set `APPIMAGE_EXTRACT_AND_RUN=1` in containers for Tauri AppImage bundling (no FUSE)

---

## Git Conventions

The project pushes to six remotes (GitHub x2, GitLab x2, GitFlic, GitVerse). When pushing:

```bash
GIT_SSH_COMMAND="ssh -o BatchMode=yes" git push origin main
```

GitHub Actions are permanently disabled. All CI/CD runs locally.
