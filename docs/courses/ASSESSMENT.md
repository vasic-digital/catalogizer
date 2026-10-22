# Catalogizer Video Course - Certification Assessment

This assessment covers all modules of the Catalogizer video course. Questions are organized by certification level. Each question has four options (A-D) with one correct answer.

**Passing Score**: 80% at each certification level

**Scoring**:
- Catalogizer User (Modules 1-4): 30 questions, need 24/30 to pass
- Catalogizer Administrator (Modules 1-5): 45 questions total, need 36/45 to pass
- Catalogizer Developer (All 6 core): 60 questions total, need 48/60 to pass
- Catalogizer Expert (All 8 modules): 70 questions total, need 56/70 to pass

---

## Section 1: Catalogizer User (Modules 1-4)

### Module 1: Introduction and Installation

**Q1.** What programming language is the catalog-api backend built with?

A) Python
B) Rust
C) Go
D) Java

---

**Q2.** Which of the following is NOT a storage protocol supported by Catalogizer?

A) SMB/CIFS
B) FTP
C) SFTP
D) WebDAV

---

**Q3.** What web framework does the catalog-api backend use?

A) Echo
B) Fiber
C) Gin
D) Chi

---

**Q4.** Which database is used for development by default?

A) PostgreSQL
B) MySQL
C) SQLite
D) MongoDB

---

**Q5.** What container runtime does the Catalogizer project use exclusively?

A) Docker
B) containerd
C) Podman
D) CRI-O

---

**Q6.** What file does the backend write on startup to communicate its port to the frontend?

A) .port
B) .service-port
C) config.json
D) .env

---

**Q7.** What command initializes git submodules after cloning the repository?

A) `git clone --depth 1`
B) `scripts/safe_submodule_init.sh`
C) `git modules install`
D) `git pull --submodules`

---

### Module 2: Getting Started with Media Management

**Q8.** How many storage protocols does Catalogizer support?

A) Three
B) Four
C) Five
D) Six

---

**Q9.** Which interface abstracts all storage protocol implementations in the backend?

A) StorageClient
B) FileSystemAdapter
C) UnifiedClient
D) ProtocolHandler

---

**Q10.** What technology delivers real-time updates from the backend to the web frontend?

A) Server-Sent Events
B) Long polling
C) WebSocket
D) gRPC streaming

---

**Q11.** Which external provider is used for movie metadata enrichment?

A) Rotten Tomatoes
B) TMDB
C) Letterboxd
D) Metacritic

---

**Q12.** How many media types does Catalogizer recognize in its entity system?

A) 5
B) 8
C) 11
D) 15

---

**Q13.** What client-side state management library does catalog-web use for server state?

A) Redux
B) MobX
C) React Query
D) SWR

---

**Q14.** What client-side state management library does catalog-web use for client state?

A) Redux
B) Zustand
C) Jotai
D) Recoil

---

**Q15.** What CSS framework does catalog-web use for styling?

A) Bootstrap
B) Material UI
C) Tailwind CSS
D) Chakra UI

---

### Module 3: Advanced Media Features

**Q16.** What are the three types of collections that can be created in Catalogizer?

A) Public, Private, and Shared
B) Manual, Smart, and Dynamic
C) Basic, Advanced, and Custom
D) Personal, Group, and System

---

**Q17.** What does the playback position service (playback_position_service.go) provide?

A) Volume normalization across tracks
B) Resume playback from a saved position
C) Automatic subtitle synchronization
D) Playlist shuffle algorithm

---

**Q18.** Which service enables sharing specific moments in media files?

A) bookmark_service.go
B) share_service.go
C) deep_linking_service.go
D) timestamp_service.go

---

**Q19.** What formats can favorites be exported to?

A) XML and YAML
B) JSON and CSV
C) PDF and HTML
D) SQL and Parquet

---

**Q20.** What hook provides drag-and-drop reordering for playlist items?

A) useDragDrop
B) usePlaylistReorder
C) useSortable
D) useReorderItems

---

### Module 4: Multi-Platform Experience

**Q21.** What architecture pattern do the Android apps follow?

A) MVC
B) MVP
C) MVVM
D) VIPER

---

**Q22.** What dependency injection framework do the Android apps use?

A) Dagger
B) Koin
C) Hilt
D) Kodein

---

**Q23.** What technology stack is used for the Catalogizer desktop application?

A) Electron with JavaScript
B) Qt with C++
C) Tauri with Rust and React
D) Flutter with Dart

---

**Q24.** What local database do the Android apps use for offline caching?

A) Realm
B) SQLDelight
C) Room
D) ObjectBox

---

**Q25.** What networking library do the Android apps use for API communication?

A) Volley
B) Retrofit
C) Ktor
D) Fuel

---

**Q26.** How do the Tauri desktop apps communicate between the React frontend and Rust backend?

A) REST API
B) IPC commands and events
C) Shared memory
D) Unix sockets

---

**Q27.** What command starts the desktop app in development mode?

A) `npm run dev`
B) `npm run desktop`
C) `npm run tauri:dev`
D) `cargo run`

---

**Q28.** What UI framework do the Android apps use for building interfaces?

A) XML Views
B) Jetpack Compose
C) Flutter
D) React Native

---

**Q29.** What state management mechanism do the Android ViewModels use?

A) LiveData
B) StateFlow
C) RxJava Observable
D) Channel

---

**Q30.** What is the purpose of the catalogizer-api-client library?

A) Internal testing only
B) TypeScript library for custom integrations and automations via the API
C) Mobile-only API wrapper
D) GraphQL schema generator

---

## Section 2: Catalogizer Administrator (Module 5)

**Q31.** What authentication mechanism does Catalogizer use?

A) OAuth2 only
B) API keys only
C) JWT-based authentication
D) SAML

---

**Q32.** Which environment variable sets the secret used for JWT token signing?

A) AUTH_SECRET
B) JWT_SECRET
C) TOKEN_KEY
D) SIGNING_KEY

---

**Q33.** What resilience pattern does the SMB client implement for handling connection failures?

A) Retry with jitter
B) Circuit breaker with exponential backoff
C) Bulkhead isolation
D) Rate limiting

---

**Q34.** What metrics format does Catalogizer expose for monitoring?

A) StatsD
B) OpenTelemetry
C) Prometheus
D) Datadog

---

**Q35.** What endpoint exposes application metrics?

A) /api/v1/stats
B) /health
C) /metrics
D) /api/v1/monitoring

---

**Q36.** What security scanning tool checks Go dependencies for known vulnerabilities?

A) gosec
B) govulncheck
C) go vet
D) staticcheck

---

**Q37.** What database is recommended for production deployments?

A) MySQL
B) SQLite
C) PostgreSQL
D) CockroachDB

---

**Q38.** What environment variable controls the log verbosity level?

A) VERBOSITY
B) LOG_LEVEL
C) DEBUG_MODE
D) LOG_VERBOSE

---

**Q39.** What does the offline cache provide in the SMB client?

A) Faster read access for frequently used files
B) Continued access to previously cached data when network storage is unavailable
C) Automatic file compression
D) Peer-to-peer file sharing

---

**Q40.** What is the purpose of the recovery mechanisms in internal/recovery/?

A) Database defragmentation
B) Crash recovery after unexpected service termination
C) Automatic backup scheduling
D) User password reset

---

**Q41.** Where are Grafana dashboard configurations stored?

A) config/dashboards/
B) monitoring/grafana/ and config/grafana-dashboards/
C) grafana/
D) dashboards/grafana/

---

**Q42.** What takes precedence in the configuration hierarchy?

A) config.json > .env > environment variables > defaults
B) defaults > config.json > .env > environment variables
C) environment variables > .env > config.json > defaults
D) .env > environment variables > config.json > defaults

---

**Q43.** What encryption feature is available for the SQLite database?

A) AES-256 file encryption
B) SQLCipher
C) TDE (Transparent Data Encryption)
D) GPG encryption

---

**Q44.** What HTTP protocol version does Catalogizer mandate for production communication?

A) HTTP/1.1
B) HTTP/2
C) HTTP/3 (QUIC)
D) HTTP/1.1 with keep-alive

---

**Q45.** What compression algorithm is preferred for HTTP responses?

A) gzip
B) deflate
C) Brotli
D) zstd

---

## Section 3: Catalogizer Developer (Module 6)

**Q46.** What is the correct order of the backend request flow?

A) Service -> Handler -> Repository -> Database
B) Handler -> Repository -> Service -> Database
C) Router -> Auth Middleware -> Handler -> Service -> Repository -> Database
D) Router -> Handler -> Auth Middleware -> Service -> Database

---

**Q47.** What pattern do Go services use for dependency management?

A) Global singletons
B) Constructor injection via NewService functions
C) Service locator
D) Dependency injection container

---

**Q48.** Where does a new storage protocol implementation need to be registered?

A) main.go
B) filesystem/factory.go
C) config/protocols.go
D) internal/registry/protocols.go

---

**Q49.** What testing convention does the Go backend follow?

A) Tests in a separate tests/ directory
B) Tests in *_test.go files beside the source file
C) Tests in test/ subdirectories
D) Tests in a parallel test module

---

**Q50.** What test pattern is the standard convention in the Go codebase?

A) BDD-style tests
B) Property-based tests
C) Table-driven tests
D) Snapshot tests

---

**Q51.** How do you run a single Go test by name?

A) `go test --filter TestName`
B) `go test -v -run TestName ./path/to/pkg/`
C) `go test -only TestName`
D) `go test -test TestName`

---

**Q52.** What file defines the UnifiedClient interface for storage protocols?

A) filesystem/client.go
B) filesystem/interface.go
C) internal/storage/unified.go
D) storage/client_interface.go

---

**Q53.** What is the media detection pipeline order?

A) Analyzer -> Detector -> Providers -> Scanner
B) Scanner -> Providers -> Detector -> Analyzer
C) Scanner -> Detector -> Analyzer -> Providers
D) Detector -> Scanner -> Analyzer -> Providers

---

**Q54.** What React component gates authenticated routes in the frontend?

A) AuthGuard
B) PrivateRoute
C) ProtectedRoute
D) SecureRoute

---

**Q55.** What build tool does the frontend use?

A) webpack
B) Parcel
C) Vite
D) esbuild

---

**Q56.** What convention must PostCSS configuration files follow for Node 18 compatibility?

A) ESM with export default
B) CommonJS with module.exports
C) JSON format
D) TypeScript with ts-node

---

**Q57.** What resource limits must be applied when running Go tests?

A) No limits required
B) `GOMAXPROCS=3 go test ./... -p 2 -parallel 2`
C) `GOMAXPROCS=8 go test ./...`
D) `go test -cpu 2 ./...`

---

**Q58.** What validation library does the frontend use for form data?

A) Yup
B) Joi
C) Zod
D) class-validator

---

**Q59.** How are Go submodules referenced in catalog-api?

A) go.sum entries
B) replace directives in go.mod
C) vendor directory
D) go workspace file

---

**Q60.** What command pushes to all 6 configured git remotes?

A) `git push --all`
B) `GIT_SSH_COMMAND="ssh -o BatchMode=yes" git push origin main`
C) `git push --mirror`
D) `git push upstream main`

---

## Section 4: Catalogizer Expert (Modules 7-8)

**Q61.** How many total challenges exist in the Catalogizer challenge system?

A) 35
B) 174
C) 209
D) 250

---

**Q62.** What test framework does catalog-web use for unit tests?

A) Jest
B) Mocha
C) Vitest
D) Jasmine

---

**Q63.** What tool is used for end-to-end testing of the web frontend?

A) Cypress
B) Puppeteer
C) Playwright
D) Selenium

---

**Q64.** What is the stale threshold for the challenge runner's liveness detection?

A) 1 minute
B) 5 minutes
C) 10 minutes
D) 30 minutes

---

**Q65.** What is the critical constraint about RunAll in the challenge system?

A) It runs all challenges in parallel
B) It is synchronous and blocking -- no other challenge can run until it finishes
C) It only runs the first 20 challenges
D) It requires admin approval for each challenge

---

**Q66.** What flag must be set for Podman container builds to avoid SSL issues?

A) `--tls-verify=false`
B) `--network host`
C) `--insecure`
D) `--no-ssl`

---

**Q67.** What environment variable prevents Go from auto-downloading newer toolchain versions in containers?

A) GONOSDK=1
B) GOTOOLCHAIN=local
C) GOVERSION=fixed
D) GO_NO_UPDATE=true

---

**Q68.** What database migration strategy does Catalogizer use?

A) ORM auto-migration
B) Versioned migrations with separate SQLite and PostgreSQL variants
C) Schema-less design
D) Single migration file

---

**Q69.** What is the maximum CPU and RAM budget for all running containers combined?

A) 2 CPUs, 4 GB RAM
B) 4 CPUs, 8 GB RAM
C) 8 CPUs, 16 GB RAM
D) Unlimited

---

**Q70.** What WAL mode pragma issue exists with go-sqlcipher?

A) WAL mode is not supported
B) go-sqlcipher ignores connection string pragmas, requiring explicit PRAGMA after connection
C) WAL mode causes data corruption
D) WAL mode requires PostgreSQL

---

## Answer Key

### Section 1: Catalogizer User (Q1-Q30)

| Question | Answer | Explanation |
|----------|--------|-------------|
| Q1  | C | catalog-api is built with Go using the Gin framework |
| Q2  | C | Catalogizer supports SMB, FTP, NFS, WebDAV, and local -- not SFTP |
| Q3  | C | The Gin web framework is used for the REST API |
| Q4  | C | SQLite is the default development database; PostgreSQL is for production |
| Q5  | C | The project uses Podman exclusively, never Docker |
| Q6  | B | The backend writes `.service-port` so the frontend knows which port to proxy to |
| Q7  | B | `scripts/safe_submodule_init.sh` initializes all submodules |
| Q8  | C | Five protocols: SMB/CIFS, FTP/FTPS, NFS, WebDAV, local filesystem |
| Q9  | C | UnifiedClient in filesystem/interface.go abstracts all protocol implementations |
| Q10 | C | WebSocket provides real-time event delivery to connected clients |
| Q11 | B | TMDB (The Movie Database) is used for movie metadata |
| Q12 | C | 11 media types: movie, tv_show, tv_season, tv_episode, music_artist, music_album, song, game, software, book, comic |
| Q13 | C | React Query (@tanstack/react-query) manages server state |
| Q14 | B | Zustand is used for client-side state management |
| Q15 | C | Tailwind CSS is the styling framework |
| Q16 | B | Manual, Smart, and Dynamic are the three collection types |
| Q17 | B | The playback position service saves and restores playback positions |
| Q18 | C | deep_linking_service.go enables sharing specific moments via deep links |
| Q19 | B | Favorites can be exported to JSON and CSV formats |
| Q20 | B | usePlaylistReorder is the hook for playlist drag-and-drop reordering |
| Q21 | C | MVVM (Model-View-ViewModel) with Compose UI, ViewModel, Repository |
| Q22 | C | Hilt is the dependency injection framework for Android |
| Q23 | C | Tauri combines a Rust backend with a React frontend |
| Q24 | C | Room database provides local caching for offline mode |
| Q25 | B | Retrofit handles API communication in the Android apps |
| Q26 | B | Tauri uses IPC (Inter-Process Communication) commands and events |
| Q27 | C | `npm run tauri:dev` starts the desktop app in development mode |
| Q28 | B | Jetpack Compose is the UI framework for Android apps |
| Q29 | B | StateFlow is used for reactive state management in ViewModels |
| Q30 | B | The API client is a TypeScript library for building custom integrations |

### Section 2: Catalogizer Administrator (Q31-Q45)

| Question | Answer | Explanation |
|----------|--------|-------------|
| Q31 | C | JWT (JSON Web Token) based authentication with role-based access control |
| Q32 | B | JWT_SECRET is the environment variable for the signing secret |
| Q33 | B | Circuit breaker with exponential backoff retry in internal/smb/ |
| Q34 | C | Prometheus format metrics exposed via internal/metrics/ |
| Q35 | C | The /metrics endpoint exposes Prometheus-format metrics |
| Q36 | B | govulncheck scans Go dependencies for known vulnerabilities |
| Q37 | C | PostgreSQL is the recommended production database |
| Q38 | B | LOG_LEVEL controls logging verbosity |
| Q39 | B | Offline cache provides continued access when network storage is unavailable |
| Q40 | B | Recovery mechanisms handle crash recovery after unexpected termination |
| Q41 | B | Grafana dashboards are in monitoring/grafana/ and config/grafana-dashboards/ |
| Q42 | C | Environment variables > .env > config.json > defaults |
| Q43 | B | SQLCipher provides database encryption for SQLite |
| Q44 | C | HTTP/3 (QUIC) is mandatory for production; fallback is HTTP/2 + gzip |
| Q45 | C | Brotli is the preferred compression; gzip is the fallback |

### Section 3: Catalogizer Developer (Q46-Q60)

| Question | Answer | Explanation |
|----------|--------|-------------|
| Q46 | C | Router -> Auth Middleware -> Handler -> Service -> Repository -> Database |
| Q47 | B | Constructor injection via NewService(deps) functions |
| Q48 | B | filesystem/factory.go contains the protocol factory that must be updated |
| Q49 | B | Go convention: *_test.go files beside the source file they test |
| Q50 | C | Table-driven tests are the standard convention |
| Q51 | B | `go test -v -run TestName ./path/to/pkg/` runs a single named test |
| Q52 | B | filesystem/interface.go defines the UnifiedClient interface |
| Q53 | C | Scanner -> Detector -> Analyzer -> Providers is the pipeline order |
| Q54 | C | ProtectedRoute gates authenticated routes in the frontend |
| Q55 | C | Vite is the build tool for the frontend |
| Q56 | B | postcss.config.js must use module.exports (CommonJS) for Node 18 compatibility |
| Q57 | B | GOMAXPROCS=3, -p 2, -parallel 2 to limit resource usage to 30-40% |
| Q58 | C | Zod is used for data validation |
| Q59 | B | replace directives in go.mod point to local submodule paths |
| Q60 | B | GIT_SSH_COMMAND with BatchMode pushes to all 6 remotes configured on origin |

### Section 4: Catalogizer Expert (Q61-Q70)

| Question | Answer | Explanation |
|----------|--------|-------------|
| Q61 | C | 209 total: 35 original (CH-001 to CH-035) + 174 userflow challenges |
| Q62 | C | Vitest is the unit testing framework for catalog-web |
| Q63 | C | Playwright is used for end-to-end testing |
| Q64 | B | 5-minute stale threshold kills stuck challenges |
| Q65 | B | RunAll is synchronous/blocking -- no other challenge can run concurrently |
| Q66 | B | `--network host` avoids SSL issues with default container networking |
| Q67 | B | GOTOOLCHAIN=local prevents auto-downloading newer Go toolchains |
| Q68 | B | Versioned migrations with separate SQLite and PostgreSQL variants per version |
| Q69 | B | Maximum 4 CPUs, 8 GB RAM across all containers (30-40% host limit) |
| Q70 | B | go-sqlcipher ignores connection string pragmas; explicit PRAGMA journal_mode=WAL is required after connecting |
