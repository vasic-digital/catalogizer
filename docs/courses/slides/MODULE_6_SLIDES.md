# Module 6: Developer Guide - Slide Outlines

---

## Slide 6.0.1: Title Slide

**Title**: Developer Guide

**Subtitle**: Architecture, Development Environment, Adding Features, Submodules, and Build Pipeline

**Speaker Notes**: This module is for developers who want to understand the codebase, set up a development environment, add new features, work with the submodule system, and build for all platforms. Familiarity with Go and TypeScript is assumed.

---

## Slide 6.1.1: Request Flow

**Title**: Tracing a Request Through the Backend

**Visual**: Flowchart: HTTP Request -> Gin Router (main.go, /api/v1) -> Auth Middleware (internal/auth/middleware.go) -> Handler (internal/handlers/) -> Service (internal/services/) -> Repository -> SQLCipher Database

**Bullet Points**:
- Routes registered in `main.go` under `/api/v1`
- Auth middleware validates JWT on every protected request
- Handlers are thin: parse request, call service, format response
- Services contain business logic
- Repositories manage database access

**Speaker Notes**: Follow a concrete example. A GET /api/v1/media request hits the router, passes through auth middleware, reaches the media handler, which calls the catalog service, which queries the repository, which reads from the database. Understanding this flow lets you find any feature in the codebase.

---

## Slide 6.1.2: Two-Layer Handler/Service Architecture

**Title**: Top-Level and Internal

| Layer | Top-Level | Internal |
|-------|-----------|----------|
| Handlers | `handlers/auth_handler.go`, `media_handler.go`, `browse.go`, `search.go`, `configuration_handler.go` | `internal/handlers/catalog.go`, `smb.go`, `media_player_handlers.go`, `localization_handlers.go` |
| Services | `services/auth_service.go`, `favorites_service.go`, `conversion_service.go`, `sync_service.go` | `internal/services/catalog.go`, `playlist_service.go`, `subtitle_service.go`, `recommendation_service.go` |

**Speaker Notes**: The two layers are an organizational choice. Top-level handlers/services handle cross-cutting concerns. Internal handlers/services handle domain-specific operations. When looking for a feature, check both layers.

---

## Slide 6.1.3: Media Detection Pipeline

**Title**: From File to Catalog Entry

**Visual**: Pipeline diagram with code file references:
1. `services/universal_scanner.go` -- Discovers files on connected sources
2. `internal/media/detector/` -- Identifies media type from name, path, extension
3. `internal/media/analyzer/` -- Extracts technical metadata (resolution, codec, bitrate)
4. `internal/media/providers/providers.go` -- Fetches external metadata
5. Recognition providers: `movie_recognition_provider.go`, `music_recognition_provider.go`, `book_recognition_provider.go`, `game_software_recognition_provider.go`

**Speaker Notes**: Each stage in the pipeline is independent and extensible. The detector can recognize new media types. The analyzer can extract new metadata fields. Providers can be added for new external services. This modularity is intentional.

---

## Slide 6.1.4: Real-Time Event System

**Title**: Event Bus to WebSocket

**Visual**: Diagram: File Change Event -> Event Bus (internal/media/realtime/) -> WebSocket Server -> Connected Clients

**Bullet Points**:
- Event bus captures all changes: new files, deletions, metadata updates, scan progress
- WebSocket server pushes events to all connected clients
- Frontend: `AuthContext.tsx` -> `WebSocketContext.tsx` -> Router -> ProtectedRoute -> Pages
- React Query manages server state with automatic caching and revalidation
- Components subscribe to specific event types via WebSocketContext

**Speaker Notes**: The event system is the backbone of real-time functionality. Every change in the backend generates an event. The WebSocket infrastructure delivers it to all connected clients. React Query ensures the UI stays consistent with the server state.

---

## Slide 6.2.1: Development Environment Setup

**Title**: Setting Up Each Component

| Component | Directory | Setup Command | Port |
|-----------|-----------|---------------|------|
| Backend | `catalog-api/` | `go mod tidy && go run main.go` | 8080 |
| Frontend | `catalog-web/` | `npm install && npm run dev` | 5173 |
| Desktop | `catalogizer-desktop/` | `npm run tauri:dev` | native |
| Android | `catalogizer-android/` | `./gradlew assembleDebug` | n/a |
| Android TV | `catalogizer-androidtv/` | `./gradlew assembleDebug` | n/a |
| API Client | `catalogizer-api-client/` | `npm install && npm run build && npm run test` | n/a |
| Full Stack | project root | `podman-compose -f docker-compose.dev.yml up` | all |

**Speaker Notes**: You do not need to set up everything. Focus on the component you are working on. For most development, just the backend and frontend are sufficient. The full Docker setup is for integration testing.

---

## Slide 6.2.2: Submodule Initialization

**Title**: Working with Git Submodules

**Bullet Points**:
- Initialize after cloning: `scripts/safe_submodule_init.sh` (never `--recursive` through `constitution/`)
- Submodules appear in root: Auth/, Cache/, Database/, Filesystem/, EventBus/, etc.
- Each submodule has its own repo, tests, and documentation
- Create new submodules: `./scripts/setup-submodule.sh ModuleName [--create-repos] [--go|--ts|--kotlin]`
- Push to all remotes: `cd SubmoduleName && commit "message"`

**Speaker Notes**: Submodules are independent git repositories nested within the main repo. They can be developed, tested, and released independently. The setup script automates creation with proper remote configuration.

---

## Slide 6.3.1: Adding a New Storage Protocol

**Title**: Implementing the UnifiedClient Interface

**Bullet Points**:
- Study `filesystem/interface.go` -- every method must be implemented
- Reference: `filesystem/local_client.go` (simplest existing implementation)
- Create: `filesystem/your_protocol_client.go`
- Update: `filesystem/factory.go` to recognize the new protocol scheme
- Write: `filesystem/your_protocol_client_test.go`

**Visual**: Code skeleton showing a new client struct implementing the interface

**Speaker Notes**: This is the cleanest extension point in the codebase. The interface is well-defined, and existing implementations serve as templates. The factory pattern means no other code needs to change -- just add the client and update the factory.

---

## Slide 6.3.2: Adding a New API Endpoint

**Title**: Handler -> Service -> Repository -> Route

**Bullet Points**:
1. Define or extend models in `internal/models/` if needed
2. Create service in `internal/services/` with `NewService(deps)` constructor injection
3. Create handler in `internal/handlers/` -- keep it thin
4. Register route in `main.go` under `/api/v1` with appropriate middleware
5. Write handler and service tests in corresponding `*_test.go` files

**Visual**: Code skeleton showing the three files and the route registration

**Speaker Notes**: Follow the existing patterns exactly. Constructor injection via NewService functions. Thin handlers that delegate to services. Table-driven tests. Matching these patterns makes code review smooth and integration predictable.

---

## Slide 6.3.3: Adding a Metadata Provider

**Title**: Extending the Detection Pipeline

**Bullet Points**:
- Study `internal/media/providers/providers.go` for the provider interface
- Reference: `movie_recognition_provider.go` for a complete implementation
- Create your provider implementing the interface
- Register in the provider registry
- Handle API authentication, rate limiting, and error cases
- Write tests with mocked API responses

**Speaker Notes**: Adding a metadata provider follows the same pattern as adding a storage protocol -- implement an interface and register. The existing providers demonstrate how to handle external API calls, map responses to internal models, and handle errors gracefully.

---

## Slide 6.3.4: Adding Frontend Features

**Title**: React Pages and Components

**Bullet Points**:
- Create page component in `catalog-web/src/pages/`
- Create supporting components in `catalog-web/src/components/your-feature/`
- Add route in router configuration; wrap with ProtectedRoute if authenticated
- Use React Query hooks for data fetching from the backend
- Custom hooks in `hooks/` for reusable state logic
- Tests in `__tests__/` directories using Vitest

**Speaker Notes**: The frontend follows React conventions strictly. PascalCase for components, camelCase for functions. React Query handles all server state. Zod validates incoming data. React Hook Form manages form state. Follow these patterns for consistency.

---

## Slide 6.4.1: Submodule Architecture

**Title**: Reusable Components Across Projects

**Bullet Points**:
- **Go Modules**: Auth, Cache, Database, Concurrency, Storage, EventBus, Streaming, Security, Observability, Formatters, Plugins, Challenges, Filesystem, RateLimiter, Config, Discovery, Media, Middleware, Watcher
- **TypeScript Modules**: WebSocket-Client-TS, UI-Components-React
- **Kotlin Module**: Android-Toolkit
- Each module: independent git repo, own test suite, own documentation
- Multi-remote push: GitHub + GitLab

**Speaker Notes**: The submodule architecture is a strategic decision. Common functionality extracted into independent modules can be reused across projects. The Auth module, for example, handles JWT, API key, OAuth2, and HTTP auth middleware -- useful in any Go web application.

---

## Slide 6.4.2: Working with Submodules

**Title**: Day-to-Day Submodule Operations

**Bullet Points**:
- **Initialize**: `scripts/safe_submodule_init.sh` (never `--recursive` through `constitution/`)
- **Create new**: `./scripts/setup-submodule.sh ModuleName [--create-repos] [--go|--ts|--kotlin]`
- **Commit changes**: `cd SubmoduleName && commit "message"`
- **Install upstreams**: `cd SubmoduleName && install_upstreams`
- **Update all**: `git submodule status | awk '$2 != "constitution" {print $2}' | xargs -r git submodule update --remote --recursive --` (caution: this moves third-party pins to upstream tips unreviewed)

**Speaker Notes**: The workflow is straightforward. Make changes in the submodule directory, commit and push from within that directory. The parent repo sees the submodule reference update and includes it in its next commit.

---

## Slide 6.5.1: Build Pipeline

**Title**: Building for All Platforms

**Bullet Points**:
- Build scripts in `scripts/` for automated builds
- Container build: `docker-compose.build.yml` with all toolchains (Go, Node.js, Rust, Android SDK)
- Backend: `go build -o catalog-api` (CGO_ENABLED=1 for SQLCipher)
- Frontend: `npm run build` (production-optimized static files)
- Desktop: `npm run tauri:build` (native installer per platform)
- Android: `./gradlew assembleDebug` or `assembleRelease`
- Full system: `scripts/run-all-tests.sh` before any release

**Speaker Notes**: The containerized build pipeline ensures reproducible builds. The builder image includes every toolchain needed to build every component. This eliminates "works on my machine" issues and guarantees consistent output.

---

## Slide 6.5.2: Testing Strategy

**Title**: Tests Across All Components

| Component | Convention | Command |
|-----------|-----------|---------|
| Go backend | `*_test.go` beside source; table-driven | `go test ./...` |
| React frontend | `__tests__/*.test.tsx` | `npm run test` |
| API client | `src/__tests__/*.test.ts` | `npm run build && npm run test` |
| Android | `test/` source set | `./gradlew test` |
| Security | Snyk + SonarQube | `scripts/snyk-scan.sh`, `scripts/sonarqube-scan.sh` |
| Full system | All of the above | `scripts/run-all-tests.sh` |

**Speaker Notes**: Run the full test suite before submitting any pull request. The individual commands are useful during development for fast feedback on the component you are changing. The full suite catches integration issues.

---

## Slide 6.5.3: Deployment Configuration

**Title**: Production Deployment Essentials

**Bullet Points**:
- `docker-compose.yml`: PostgreSQL 15, Redis 7, Nginx reverse proxy, app services
- Resource limits: 2 CPUs, 2GB RAM for database; configured per service
- Health checks: pg_isready for PostgreSQL, redis-cli ping for Redis
- Nginx: `config/nginx.conf` and `config/nginx/catalogizer.prod.conf`
- Systemd: `config/systemd/catalogizer-api.service` for bare-metal deployment
- Monitoring: `monitoring/prometheus.yml` and `monitoring/grafana/`

**Speaker Notes**: The production compose file is battle-tested. Resource limits prevent any single service from consuming all system resources. Health checks ensure services are restarted if they become unhealthy. Nginx handles TLS termination and request routing.

---

## Slide 6.5.4: Module 6 Summary

**Title**: What We Covered

**Bullet Points**:
- Request flow: Router -> Auth Middleware -> Handler -> Service -> Repository -> Database
- Media detection pipeline: Scanner -> Detector -> Analyzer -> Providers
- Development environment for all seven components
- Adding features: new protocols, API endpoints, metadata providers, frontend pages
- Submodule architecture: 19 Go + 2 TypeScript + 1 Kotlin modules
- Build pipeline with containerized reproducible builds
- Testing strategy across all components

**Course Conclusion**: Students completing all six modules have the knowledge to use, administer, and extend Catalogizer across all platforms.

**Speaker Notes**: This concludes the six-module course. Students who completed Modules 1-4 are certified Catalogizer Users. Adding Module 5 qualifies as Catalogizer Administrator. All six modules qualify as Catalogizer Developer. Refer to the assessment and exercises documents for certification requirements.
