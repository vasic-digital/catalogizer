---
title: Architecture Overview
description: System architecture, component design, and technology stack for Catalogizer
---

# Architecture Overview

Catalogizer is a multi-platform media collection manager composed of seven components that share a common API backend. This page describes the system architecture, data flow, and technology choices.

---

## System Components

```
+-------------------+     +-------------------+     +-------------------+
|   catalog-web     |     | catalogizer-      |     | catalogizer-      |
|   (React/TS)      |     | desktop (Tauri)   |     | android (Kotlin)  |
|   Port 3000       |     |                   |     |                   |
+--------+----------+     +--------+----------+     +--------+----------+
         |                         |                          |
         |    HTTP/3 + Brotli      |    HTTP/3 + Brotli       |
         +------------+------------+------------+-------------+
                      |                         |
              +-------v-------------------------v-------+
              |            catalog-api                   |
              |         (Go 1.25 / Gin)                  |
              |           Port 8080                      |
              +----+--------+--------+---------+--------+
                   |        |        |         |
            +------+  +-----+  +----+---+ +---+------+
            |SQLite|  |Postgres| | Redis  | |Prometheus|
            +------+  +--------+ +--------+ +---------+
```

### Component Summary

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **catalog-api** | Go 1.25, Gin | REST API, media detection, storage protocols |
| **catalog-web** | React 18, TypeScript, Vite | Web frontend |
| **catalogizer-desktop** | Tauri (Rust + React) | Desktop app for Windows, macOS, Linux |
| **installer-wizard** | Tauri (Rust + React) | First-time setup wizard |
| **catalogizer-android** | Kotlin, Jetpack Compose | Android phone/tablet app |
| **catalogizer-androidtv** | Kotlin, Leanback | Android TV app |
| **catalogizer-api-client** | TypeScript | Reusable API client library |

---

## Backend Architecture (catalog-api)

The backend follows a layered architecture with dependency injection.

```
HTTP Request
    |
    v
Middleware (CORS, Auth, Rate Limit, Compression, Metrics, Input Validation)
    |
    v
Handler (request parsing, response formatting)
    |
    v
Service (business logic, orchestration)
    |
    v
Repository (data access, SQL queries)
    |
    v
Database (SQLite or PostgreSQL)
```

### Dual Package Layout

- **Top-level packages** (`handlers/`, `services/`, `repository/`, `middleware/`): Domain logic for media, collections, favorites, playback
- **Internal packages** (`internal/handlers/`, `internal/services/`, `internal/middleware/`): Infrastructure concerns such as auth, metrics, SMB resilience, media detection

### Key Subsystems

**Storage Protocol Abstraction**

`filesystem/interface.go` defines `UnifiedClient`, the common interface for all storage protocols. `filesystem/factory.go` creates the appropriate client based on protocol type. Application code interacts only with the interface, never with protocol-specific implementations.

**Media Detection Pipeline**

```
Scanner -> Detector -> Analyzer -> Providers -> Aggregation
```

- **Detector**: Identifies media type from filename, path, and extension
- **Analyzer**: Extracts quality metadata (resolution, codec, bitrate)
- **Providers**: Fetches external metadata from TMDB, IMDB, MusicBrainz, etc.
- **Aggregation**: Creates structured media entities with hierarchy (shows > seasons > episodes)

**Real-Time Events**

`internal/media/realtime/` implements an event bus that publishes scan progress, new media, and status changes over WebSocket connections to all connected clients.

**SMB Resilience**

The former `internal/smb/` resilience layer (circuit breaker, offline cache, backoff retry) was removed in SMB-FU-1/2/15 because nothing referenced it; live SMB access goes through `internal/services/smb.go` and the `filesystem` package.

---

## Database Layer

A dual-dialect abstraction supports SQLite (development) and PostgreSQL (production).

- `database/dialect.go` rewrites SQL at runtime: `?` to `$1, $2, ...` for PostgreSQL, `INSERT OR IGNORE` to `ON CONFLICT DO NOTHING`, boolean literals `0/1` to `FALSE/TRUE`
- `database.DB` wraps `*sql.DB` with shadowed `Exec()`, `Query()`, `QueryRow()` methods that apply dialect rewrites transparently
- `InsertReturningID()` handles the difference between `LastInsertId()` (SQLite) and `RETURNING id` (PostgreSQL)
- Migrations in `database/migrations/` with separate SQLite and PostgreSQL variants
- SQLCipher support for encrypted SQLite

---

## Frontend Architecture (catalog-web)

```
AuthProvider -> WebSocketProvider -> Router -> Pages -> Components
```

- **State management**: React Query for server state, Zustand for client state
- **Styling**: Tailwind CSS
- **Forms**: React Hook Form with Zod validation
- **Animations**: Framer Motion
- **Testing**: Vitest (unit), Playwright (E2E)
- **API proxy**: Dev server reads `../catalog-api/.service-port` to resolve backend port

---

## Mobile Architecture (Android)

Both Android apps follow MVVM:

```
Compose UI -> ViewModel (StateFlow) -> Repository -> Room + Retrofit
```

- **Dependency injection**: Hilt
- **Local database**: Room for offline caching
- **Network**: Retrofit with OkHttp, HTTP/3 via Cronet, Brotli encoding
- **Android TV**: Leanback library for 10-foot UI

---

## Desktop Architecture (Tauri)

```
React Frontend <-> IPC Bridge <-> Rust Backend
```

The React frontend communicates with the Rust backend through Tauri's IPC command/event system. The Rust side handles system-level operations (file access, keychain, system tray) while the React side manages the UI.

---

## Submodule Architecture

The project uses 41 independent git submodules under the vasic-digital organization. Go modules (22 total) use `replace` directives in `go.mod` to reference local submodule paths. TypeScript modules (9 total) use `file:../` references in `package.json`. Each submodule is an independent repository with its own tests, documentation, ARCHITECTURE.md, and Upstreams for multi-remote push.

---

## Networking

- **Protocol**: HTTP/3 (QUIC) with TLS 1.3, falling back to HTTP/2
- **Compression**: Brotli (`Accept-Encoding: br`), fallback to gzip
- **WebSocket**: Real-time event streaming at `/ws`
- **Self-signed TLS**: Generated at startup for development; use proper certificates in production
