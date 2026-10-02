# Module 6: Developer Guide - Video Scripts

---

## Lesson 6.1: Architecture Deep Dive

**Duration**: 15 minutes

### Narration

Welcome to the developer module. In this lesson, we are going to trace through the entire Catalogizer architecture, following data from a user request through every layer.

Let us start with the backend. When a request arrives at the catalog-api, it first hits the Gin router configured in main.go. Routes are organized under /api/v1. The authentication middleware in internal/auth/middleware.go intercepts the request and validates the JWT token. If valid, the request proceeds; otherwise, it is rejected with a 401 response.

The request reaches a handler function in internal/handlers/. There are handlers for different domains: auth.go for authentication, catalog.go for catalog operations, media.go for media queries, copy.go for file operations, download.go for file downloads, smb.go and smb_discovery.go for SMB-specific operations, media_player_handlers.go for playback, and localization_handlers.go for internationalization.

Handlers are thin -- they parse the request, call the appropriate service, and format the response. The real business logic lives in internal/services/. Here you find catalog.go for the core catalog operations, subtitle_service.go for subtitles, playlist_service.go for playlists, media_player_service.go, video_player_service.go, and music_player_service.go for playback services. There are also specialized services like recommendation_service.go, duplicate_detection_service.go, cover_art_service.go, lyrics_service.go, deep_linking_service.go, and many more.

Services interact with the database through repositories, and the database itself is SQLite encrypted with SQLCipher.

Now the media detection pipeline. When a new file is discovered by the universal scanner (services/universal_scanner.go), it enters the detector (internal/media/detector/). The detector identifies the media type from the file name, path, and extension. The analyzer (internal/media/analyzer/) processes the file further -- extracting technical metadata like resolution, codec, and bitrate.

External providers (internal/media/providers/providers.go) then enrich the item with metadata from TMDB, IMDB, TVDB, MusicBrainz, Spotify, and Steam. There are specialized recognition providers: movie_recognition_provider.go, music_recognition_provider.go, book_recognition_provider.go, and game_software_recognition_provider.go.

The real-time event system in internal/media/realtime/ captures all these changes. The event bus distributes events, and the WebSocket server pushes them to connected clients.

On the frontend, AuthContext.tsx wraps the application with authentication state. WebSocketContext.tsx establishes the real-time connection. React Query manages server state with automatic caching and revalidation. The Router uses ProtectedRoute components to gate authenticated pages.

### On-Screen Actions

- [00:00] Open the catalog-api project at a high level
- [00:30] Open main.go -- show route registration under /api/v1
- [01:30] Open internal/auth/middleware.go -- show JWT validation flow
- [02:30] Open internal/handlers/ -- list all handler files
- [03:00] Open catalog.go handler -- trace a catalog list request
- [03:30] Show the handler calling into the service layer
- [04:00] Open internal/services/ -- list all service files
- [04:30] Open services/catalog.go -- show business logic
- [05:30] Open the database layer -- show repository pattern
- [06:00] Trace the media detection pipeline
- [06:30] Open services/universal_scanner.go -- show scanning logic
- [07:00] Open internal/media/detector/ -- show type detection
- [07:30] Open internal/media/analyzer/ -- show metadata extraction
- [08:00] Open internal/media/providers/providers.go -- show provider interface
- [08:30] Open movie_recognition_provider.go and music_recognition_provider.go
- [09:00] Open book_recognition_provider.go and game_software_recognition_provider.go
- [09:30] Open internal/media/realtime/ -- show event bus and WebSocket
- [10:00] Switch to the frontend: open catalog-web/src/
- [10:30] Open AuthContext.tsx -- show auth state management
- [11:00] Open WebSocketContext.tsx -- show event subscription
- [11:30] Open App.tsx -- show the provider chain wrapping the Router
- [12:00] Open a page component and show React Query usage
- [12:30] Show the ProtectedRoute component
- [13:00] Open the hooks directory -- show custom hooks
- [13:30] Diagram the complete request flow on screen
- [14:00] Recap the full architecture

### Key Points

- Request flow: Gin Router -> Auth Middleware -> Handler -> Service -> Repository -> SQLCipher DB
- Handlers in internal/handlers/: thin request parsing and response formatting
- Services in internal/services/: business logic for catalog, playlists, subtitles, player, recommendations, etc.
- Detection pipeline: universal_scanner -> detector -> analyzer -> providers (TMDB, IMDB, Spotify, Steam, etc.)
- Specialized recognition: movie, music, book, game/software providers
- Real-time: event bus in internal/media/realtime/ pushes events via WebSocket
- Frontend: AuthContext -> WebSocketContext -> Router with ProtectedRoute, React Query for server state

### Tips

> **Tip**: When exploring the codebase, start with main.go to see all routes, then follow a single request through handler -> service -> database. This gives you a mental model for where to find any feature.

---

## Lesson 6.2: Setting Up the Development Environment

**Duration**: 12 minutes

### Narration

Let us get your development environment running so you can contribute to Catalogizer.

This is a monorepo with multiple components, each with its own technology stack. You do not need to set up everything at once -- focus on the component you want to work on.

For the backend, navigate to catalog-api. Run go mod tidy to install dependencies. Create a .env file with development settings -- you can use shorter JWT expiry, debug log level, and a local database path. Start the server with go run main.go. It runs on port 8080 by default.

For the frontend, navigate to catalog-web. Run npm install, then npm run dev. Vite starts on port 5173 with hot module replacement. Changes to React components appear instantly in the browser.

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

For the full stack with dependencies, use containers. Run podman compose -f docker-compose.dev.yml up from the project root. This starts PostgreSQL, Redis, and all services with development-friendly settings.

For the desktop apps, you need the Rust toolchain in addition to Node.js. In the catalogizer-desktop or installer-wizard directory, run npm run tauri:dev. This starts both the frontend dev server and the Rust backend with hot reloading for the frontend.

For Android development, open catalogizer-android or catalogizer-androidtv in Android Studio. The Gradle wrapper handles dependencies. Run ./gradlew assembleDebug to build, or use Android Studio's run configuration to deploy to a connected device or emulator.

For the API client library, navigate to catalogizer-api-client. Run npm install, then npm run build to compile, and npm run test to verify everything works.

The project structure follows clear conventions. Go files use *_test.go beside the source file. TypeScript tests are in __tests__ directories. Kotlin tests follow Android conventions in the test/ source set.

### On-Screen Actions

- [00:00] Show the monorepo root with all component directories
- [00:30] Backend setup: `cd catalog-api && go mod tidy`
- [01:00] Create a development .env file
- [01:30] `go run main.go` -- show server starting on port 8080
- [02:00] Open another terminal: `cd catalog-web && npm install`
- [02:30] `npm run dev` -- show Vite starting on port 5173
- [03:00] Open the browser and show the frontend connecting to the backend
- [03:30] Make a small React change -- show hot reload
- [04:00] Stop individual services and start Docker: `podman compose -f docker-compose.dev.yml up`
- [04:30] Show all containers starting
- [05:00] Desktop setup: show Rust toolchain check (`rustc --version`)
- [05:30] `cd catalogizer-desktop && npm run tauri:dev`
- [06:00] Show the desktop app window opening
- [06:30] Android setup: open catalogizer-android in Android Studio
- [07:00] Show Gradle sync and dependency resolution
- [07:30] `./gradlew assembleDebug` -- show build
- [08:00] Deploy to emulator and show app running
- [08:30] API client: `cd catalogizer-api-client && npm install && npm run build && npm run test`
- [09:00] Show tests passing
- [09:30] Show the project structure conventions: test files beside source
- [10:00] Show the scripts directory: install.sh, run-all-tests.sh, setup-implementation.sh
- [10:30] Run scripts/run-all-tests.sh to verify entire system
- [11:00] Final overview of dev environment options

### Key Points

- Monorepo with independent components: set up only what you need
- Backend: `go mod tidy && go run main.go` (port 8080)
- Frontend: `npm install && npm run dev` (port 5173 with HMR)
- Full stack: `podman compose -f docker-compose.dev.yml up`
- Desktop: Rust toolchain + `npm run tauri:dev`
- Android: Android Studio + `./gradlew assembleDebug`
- API client: `npm install && npm run build && npm run test`
- All tests: `scripts/run-all-tests.sh`

### Tips

> **Tip**: For most development work, running just the backend and frontend without Docker is fastest. Use Docker only when you need PostgreSQL or Redis for features that depend on them.

---

## Lesson 6.3: Adding New Features

**Duration**: 15 minutes

### Narration

Let us walk through how to add new features to Catalogizer. We will cover the three most common extension points: new storage protocols, new API endpoints, and new external metadata providers.

Starting with a new storage protocol. Open filesystem/interface.go and study the UnifiedClient interface. Every method defined there must be implemented by your new client. Create a new file -- say, filesystem/s3_client.go -- and implement the interface. Your client needs methods for listing files, reading file metadata, connecting, and disconnecting.

Then update filesystem/factory.go to recognize your new protocol scheme. Add a case to the factory function that creates your client when it sees the appropriate URL prefix. Write tests in a corresponding *_test.go file.

For a new API endpoint, you work across three layers. First, define or extend models in internal/models/ if needed. Second, create or extend a service in internal/services/. Follow the constructor injection pattern -- create a NewMyService function that accepts dependencies. Third, add a handler function in internal/handlers/. Keep the handler thin: parse the request, call the service, return the response.

Register the route in main.go under /api/v1. Apply appropriate middleware -- authentication middleware for protected endpoints. Write a handler test in the corresponding *_test.go file.

For a new external metadata provider, look at internal/media/providers/providers.go. Study the existing provider interface. Then look at the recognition providers as examples. The movie_recognition_provider.go shows how to integrate with an external API to fetch metadata. Create your provider, implement the interface, and register it.

On the frontend, adding a new page follows a pattern. Create a page component in catalog-web/src/pages/. Create any supporting components in a new subdirectory under components/. Add a route in the router configuration. If the page needs to be protected, wrap it with ProtectedRoute. Use React Query hooks for data fetching from the backend.

For the Android apps, follow MVVM. Create a new Composable for the UI, a ViewModel to hold state, and extend or create Repositories for data access.

### On-Screen Actions

- [00:00] Open filesystem/interface.go -- highlight every method in UnifiedClient
- [01:00] Show filesystem/local_client.go as a simple implementation example
- [02:00] Create a skeleton for a new protocol client
- [02:30] Open filesystem/factory.go -- show where to add the new protocol
- [03:00] Show an existing test file: filesystem/smb_client_test.go
- [03:30] Discuss testing the new client
- [04:00] Switch to API endpoint creation
- [04:30] Open internal/services/catalog.go -- show the NewService pattern
- [05:00] Create a skeleton service with constructor injection
- [05:30] Open internal/handlers/catalog.go -- show a handler function
- [06:00] Create a skeleton handler
- [06:30] Open main.go -- show route registration
- [07:00] Add a new route with auth middleware
- [07:30] Show catalog_test.go for test examples
- [08:00] Switch to metadata provider creation
- [08:30] Open internal/media/providers/providers.go -- show the interface
- [09:00] Open movie_recognition_provider.go as an example
- [09:30] Show the API call pattern and metadata mapping
- [10:00] Create a skeleton provider
- [10:30] Switch to frontend
- [11:00] Open an existing page: MediaBrowser.tsx
- [11:30] Show the component structure and React Query usage
- [12:00] Create a skeleton page component
- [12:30] Add a route in the router configuration
- [13:00] Show the ProtectedRoute wrapper
- [13:30] Briefly discuss Android feature addition: Composable + ViewModel + Repository
- [14:00] Recap the feature addition patterns

### Key Points

- New protocol: implement UnifiedClient interface (filesystem/interface.go), update factory.go, add tests
- New API endpoint: model (internal/models/) -> service (internal/services/) -> handler (internal/handlers/) -> route (main.go)
- Follow constructor injection: `NewService(deps)` pattern for services
- New metadata provider: implement provider interface, register in providers.go
- New frontend page: page component (pages/) + components (components/) + route + ProtectedRoute
- Always write tests: *_test.go beside Go source, __tests__/ for TypeScript

### Tips

> **Tip**: Study an existing implementation before writing new code. The patterns are consistent throughout the codebase, and matching them makes code review smoother and integration cleaner.

> **Tip**: When adding a new API endpoint, write the handler test first. This clarifies the request/response contract before you implement the service logic.

---

## Lesson 6.4: Testing Strategy

**Duration**: 12 minutes

### Narration

Testing is a first-class concern in Catalogizer. Each component has its own testing approach, and there is also a system-level test runner.

In the Go backend, tests follow the standard Go convention: test files are named *_test.go and placed beside the source file they test. For example, catalog.go is tested by catalog_test.go, and smb.go by smb_test.go.

Go tests in this project use table-driven test patterns. You define a slice of test cases, each with input and expected output, and loop through them. This makes it easy to add new test cases and keeps tests readable. Let me show an example from catalog_test.go.

Run all backend tests with go test ./... from the catalog-api directory. For a specific test, use go test -v -run TestName ./pkg/. The -v flag gives verbose output.

The frontend uses a test framework configured in catalog-web. Tests live in __tests__ directories alongside their components. Run npm run test to execute. Before committing, always run npm run lint for ESLint checks and npm run type-check for TypeScript compilation verification.

The API client library (catalogizer-api-client) has its own test suite in src/__tests__/. Run npm run build && npm run test to verify.

Android tests follow Android conventions. Unit tests are in the test/ source set. Run ./gradlew test in either the Android or Android TV project directory.

For integration and system-level testing, the scripts/run-all-tests.sh script executes tests across all components. It runs Go tests, frontend tests, security tests, and more in sequence. This is the script that should pass before merging any changes.

Security testing has its own scripts: scripts/security-test.sh for security-focused tests, scripts/snyk-scan.sh for dependency vulnerability scanning, and scripts/sonarqube-scan.sh for static analysis. The docker-compose.security.yml provides a dedicated environment for security testing.

Benchmark tests exist in the providers package: providers_bench_test.go measures the performance of metadata provider calls.

### On-Screen Actions

- [00:00] Open catalog-api and show *_test.go files beside source files
- [00:30] Open catalog_test.go -- show table-driven test pattern
- [01:30] Run `go test ./...` -- show all tests passing
- [02:30] Run a single test: `go test -v -run TestCatalog ./internal/services/`
- [03:00] Show test coverage output
- [03:30] Open services/smb_test.go -- show another test example
- [04:00] Open services/cache_service_test.go
- [04:30] Open services/rename_tracker_test.go
- [05:00] Switch to frontend testing
- [05:30] Open a __tests__ directory under components or hooks
- [06:00] Show a React component test
- [06:30] Run `npm run test` in catalog-web
- [07:00] Run `npm run lint` -- show ESLint output
- [07:30] Run `npm run type-check` -- show TypeScript verification
- [08:00] Open catalogizer-api-client tests
- [08:30] Run `npm run build && npm run test`
- [09:00] Show Android test execution: `./gradlew test`
- [09:30] Open scripts/run-all-tests.sh -- show what it runs
- [10:00] Execute the full test suite
- [10:30] Open providers_bench_test.go -- show benchmark tests
- [11:00] Show security test scripts
- [11:30] Final overview of the testing strategy

### Key Points

- Go: table-driven *_test.go files beside source; run with `go test ./...`
- Frontend: __tests__ directories; run with `npm run test`, lint with `npm run lint`, type-check with `npm run type-check`
- API client: `npm run build && npm run test`
- Android: `./gradlew test` in both Android and Android TV directories
- Full system: scripts/run-all-tests.sh runs everything
- Security: scripts/security-test.sh, snyk-scan.sh, sonarqube-scan.sh
- Benchmarks: providers_bench_test.go for performance testing

### Tips

> **Tip**: Run scripts/run-all-tests.sh before submitting any pull request. It catches issues across components that might not be obvious when testing a single component in isolation.

> **Tip**: When writing Go tests, always use table-driven patterns. They are the project convention and make it trivial to add edge cases later.

---

## Lesson 6.5: CI/CD, Security Scanning & Deployment

**Duration**: 11 minutes

### Narration

In this final lesson, let us cover the build, security, and deployment pipeline.

Build scripts live in the build-scripts directory at the project root. These automate the compilation and packaging of each component. They handle Go compilation, frontend bundling, Tauri packaging, and Android APK generation.

Security scanning is integrated into the workflow. Snyk scans dependencies for known vulnerabilities across both Go modules and npm packages. Run it with scripts/snyk-scan.sh. The output shows any vulnerable dependencies along with severity levels and remediation advice.

SonarQube performs static code analysis. The configuration is in sonar-project.properties at the project root. Run the scan with scripts/sonarqube-scan.sh. SonarQube catches code smells, potential bugs, security hotspots, and code duplication.

The dependency-check-suppressions.xml file manages known false positives in dependency scanning. When a scanner flags a dependency that you have verified is safe, add a suppression entry rather than ignoring the entire scan.

For deployment, Docker is the recommended approach. The production docker-compose.yml sets up all services with appropriate resource limits, health checks, and restart policies.

The deployment includes three infrastructure services. PostgreSQL 15 Alpine for persistent data with a health check that runs every 10 seconds. Redis 7 Alpine for caching with its own health check. Nginx as a reverse proxy with configuration from config/nginx.conf.

When deploying, set all required environment variables. At minimum: POSTGRES_PASSWORD, JWT_SECRET, DB_ENCRYPTION_KEY, and any external API keys. Use a proper secrets management solution rather than plain text files in production.

Resource limits in docker-compose.yml prevent any single service from consuming all system resources. PostgreSQL is capped at 2 CPUs and 2GB RAM with reservations of 1 CPU and 1GB.

Monitor your deployment using the Prometheus and Grafana stack from the monitoring directory. Set up alerts for service health, disk space, and error rates.

### On-Screen Actions

- [00:00] Open the build-scripts directory -- show available scripts
- [01:00] Run a build script for the backend
- [01:30] Run a build script for the frontend
- [02:00] Open scripts/snyk-scan.sh -- show what it runs
- [02:30] Execute the Snyk scan and review output
- [03:30] Open sonar-project.properties
- [04:00] Run scripts/sonarqube-scan.sh
- [04:30] Show SonarQube dashboard with analysis results
- [05:30] Open dependency-check-suppressions.xml -- show how to manage false positives
- [06:00] Open docker-compose.yml -- walk through production configuration
- [06:30] Show PostgreSQL service with health checks and resource limits
- [07:00] Show Redis service configuration
- [07:30] Show Nginx reverse proxy configuration
- [08:00] Show environment variable requirements for production
- [08:30] Deploy: `podman compose up -d` (detached mode)
- [09:00] Verify all containers are healthy: `docker ps`
- [09:30] Open the deployed application in a browser
- [10:00] Show monitoring/prometheus.yml and monitoring/grafana/ for production monitoring
- [10:30] Final overview and course conclusion

### Key Points

- Build automation in build-scripts/ for all components
- Snyk (scripts/snyk-scan.sh): dependency vulnerability scanning for Go and npm
- SonarQube (scripts/sonarqube-scan.sh + sonar-project.properties): static code analysis
- dependency-check-suppressions.xml: manage false positives in security scans
- Production deployment: docker-compose.yml with PostgreSQL, Redis, Nginx, and application services
- Resource limits and health checks configured for production stability
- Required secrets: POSTGRES_PASSWORD, JWT_SECRET, DB_ENCRYPTION_KEY -- use a secrets manager
- Monitor with Prometheus + Grafana from monitoring/ directory

### Tips

> **Tip**: Never store production secrets in version control. Use environment variables injected at runtime from a secure secrets manager like HashiCorp Vault, AWS Secrets Manager, or Kubernetes secrets.

> **Tip**: After deploying, run the full test suite against the production environment to verify everything is working. Use a separate test user account to avoid interfering with real data.
