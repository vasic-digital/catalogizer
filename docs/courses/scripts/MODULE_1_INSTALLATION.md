# Module 1: Introduction & Installation - Video Scripts

---

## Lesson 1.1: What is Catalogizer?

**Duration**: 8 minutes

### Narration

Welcome to the Catalogizer video course. In this first lesson, we are going to explore what Catalogizer is and why it exists.

Catalogizer is an advanced multi-protocol media collection management system. It automatically detects, categorizes, and organizes your media files across multiple storage protocols. Whether your media lives on SMB network shares, FTP servers, NFS mounts, WebDAV endpoints, or local disks, Catalogizer brings it all together into a single, searchable catalog.

The system identifies over 50 media types, including movies, TV shows, music, games, software, documentaries, and more. It does not just list files -- it enriches them with metadata from external providers like TMDB, IMDB, TVDB, MusicBrainz, Spotify, and Steam.

Let us take a look at the main components of Catalogizer:

First, the **catalog-api**. This is the Go backend built with the Gin framework. It handles authentication, media detection, file system monitoring, and exposes a REST API under /api/v1.

Second, **catalog-web**. This is the React TypeScript frontend with Tailwind CSS. It provides a modern, responsive interface with real-time updates via WebSocket.

Third, the **desktop applications**. These are built with Tauri, combining a React frontend with a Rust backend. There is also an installer wizard for guided setup.

Fourth, the **mobile applications**. There are separate apps for Android phones and Android TV, both built with Kotlin and Jetpack Compose using MVVM architecture.

Finally, the **API client library**. This is a TypeScript library that lets you integrate Catalogizer into your own applications and automations.

Catalogizer also uses a modular architecture with reusable submodules for common functionality like authentication, caching, database operations, filesystem access, and more. These are independent git submodules maintained under the vasic-digital organization.

### On-Screen Actions

- [00:00] Show title card: "What is Catalogizer?"
- [00:15] Display the architecture diagram showing React Web App, Go REST API, SQLite DB, WebSocket, Media Detection Engine, External APIs, and Multi-Protocol File System Clients
- [01:30] Highlight each protocol: SMB, FTP, NFS, WebDAV, Local
- [02:30] Show the component list with logos: catalog-api (Go gopher), catalog-web (React logo), Tauri logo, Kotlin logo, TypeScript logo
- [04:00] Demonstrate a quick walkthrough of the web UI -- dashboard with media stats
- [05:30] Show the desktop app running alongside the web version
- [06:30] Brief clip of the Android app browsing media
- [07:00] Show the submodule directory listing: Auth/, Cache/, Database/, Filesystem/, etc.
- [07:15] Show the API client code in an editor, making a simple call

### Key Points

- Catalogizer unifies media scattered across multiple storage protocols into one catalog
- Automatic detection of 50+ media types with external metadata enrichment
- Six main components: backend API, web frontend, desktop app, installer wizard, Android apps, API client
- Five supported protocols: SMB/CIFS, FTP/FTPS, NFS, WebDAV, local filesystem
- Real-time monitoring with WebSocket-based live updates
- Modular architecture with reusable submodules for Auth, Cache, Database, Filesystem, and more

### Tips

> **Tip**: You do not need to use all components. The catalog-api and catalog-web are the core -- everything else extends the experience to additional platforms.

### Quiz Questions

1. **Q**: How many storage protocols does Catalogizer support?
   **A**: Five: SMB/CIFS, FTP/FTPS, NFS, WebDAV, and local filesystem.

2. **Q**: What external providers does Catalogizer use for metadata enrichment?
   **A**: TMDB, IMDB, TVDB, MusicBrainz, Spotify, and Steam.

3. **Q**: What technology is the backend API built with?
   **A**: Go with the Gin framework.

---

## Lesson 1.2: System Requirements & Architecture Overview

**Duration**: 10 minutes

### Narration

Before we install Catalogizer, let us understand what you need and how the system is structured.

For the backend, you need Go version 1.21 or later. For the frontend, Node.js 18 or later with npm. For container deployment, either Podman or Docker with their respective compose tools. For the desktop apps, you need the Rust toolchain. For Android apps, Android Studio with the Android SDK. The web frontend supports Chrome 90+, Firefox 88+, Safari 14+, and Edge 90+.

Now let us walk through the architecture. The system follows a layered pattern. In the backend, requests flow through Handlers, then to Services, then to Repositories, and finally to the SQLite database.

The backend has two layers of handlers and services. The top-level handlers/ directory contains handlers like auth_handler.go, media_handler.go, browse.go, search.go, and configuration_handler.go. The top-level services/ directory has services like auth_service.go, favorites_service.go, conversion_service.go, and sync_service.go. Then there is the internal layer with internal/handlers/ for domain-specific operations like SMB, downloads, and media playback, and internal/services/ for the core catalog service, media player services, subtitle service, and detection providers.

The media detection pipeline is a key component. When a file is discovered, it goes through the detector, which identifies the media type. Then the analyzer processes it further. Finally, providers like TMDB and IMDB fetch enriched metadata. There are specialized recognition providers for movies, music, books, and games.

For real-time updates, an event bus captures changes and pushes them through WebSocket connections to all connected clients. This means when a new file appears on an SMB share, your web browser updates automatically.

The filesystem layer is particularly elegant. There is a UnifiedClient interface defined in filesystem/interface.go. Each protocol -- SMB, FTP, NFS, WebDAV, local -- has its own client implementation. The factory in filesystem/factory.go creates the right client based on the protocol. If you ever want to add a new protocol, you just implement this interface.

The project also uses a submodule architecture. Reusable components are extracted into independent git repositories: Auth for authentication, Cache for caching, Database for data access, Filesystem for multi-protocol file access, EventBus for pub/sub messaging, and many more. These are maintained independently and shared across projects.

### On-Screen Actions

- [00:00] Show system requirements checklist on screen
- [01:00] Draw the layered architecture: Handler -> Service -> Repository -> SQLite
- [01:30] Show both handler layers: handlers/ (top-level) and internal/handlers/
- [02:00] Show both service layers: services/ (top-level) and internal/services/
- [02:30] Animate the media detection pipeline: file -> detector -> analyzer -> providers (show TMDB, IMDB logos)
- [04:00] Show the event bus -> WebSocket -> client browser diagram
- [05:00] Open filesystem/interface.go and highlight the UnifiedClient interface
- [06:00] Open filesystem/factory.go and show client creation logic
- [07:00] Show the frontend context chain: AuthProvider -> WebSocketProvider -> Router
- [08:00] Show the submodule directory listing and explain the architecture
- [09:00] Recap with full system diagram

### Key Points

- Backend: Go 1.21+; Frontend: Node.js 18+; Desktop: Rust; Mobile: Android SDK
- Container deployment with Podman (preferred) or Docker
- Two-layer backend: top-level handlers/services and internal/ handlers/services
- Layered backend: Handler -> Service -> Repository -> SQLite
- Media pipeline: detector -> analyzer -> providers (TMDB, IMDB, TVDB, MusicBrainz, Spotify, Steam)
- Specialized recognition providers for movies, music, books, and games/software
- Real-time: event bus -> WebSocket pushes updates to all clients
- UnifiedClient interface abstracts all storage protocols behind a common API
- Submodule architecture for reusable components (Auth, Cache, Database, Filesystem, etc.)

### Tips

> **Tip**: The architecture is designed for extension. Adding a new storage protocol means implementing a single interface. Adding a new metadata provider follows the same pattern in the providers package.

### Quiz Questions

1. **Q**: What pattern does the backend follow for request processing?
   **A**: Handler -> Service -> Repository -> SQLite (layered architecture).

2. **Q**: What is the purpose of the UnifiedClient interface?
   **A**: It abstracts all storage protocols behind a common API so the rest of the application does not need to know which protocol is being used.

3. **Q**: How are real-time updates delivered to connected clients?
   **A**: Through an event bus that pushes changes via WebSocket connections.

---

## Lesson 1.3: Container Installation (Podman/Docker)

**Duration**: 12 minutes

### Narration

Containers are the fastest way to get Catalogizer running. The project supports both Podman and Docker as container runtimes, with Podman preferred on systems where Docker is unavailable.

Let us start by checking which container runtime you have available. Run "which podman" or "which docker" to see what is installed. The project includes production, development, build, and security Docker Compose configurations, all compatible with both runtimes.

Let us start with the development environment, which is what you will use for evaluation and testing. Open your terminal and navigate to the Catalogizer project root.

Run podman-compose with the dev configuration file. This starts all required services: the PostgreSQL database, Redis cache, the catalog-api backend, and the catalog-web frontend.

The production docker-compose.yml configures PostgreSQL 15 Alpine with a dedicated user, password, and database. It sets up health checks that run pg_isready every 10 seconds. Resource limits are configured -- 2 CPUs and 2GB RAM for the database.

Redis 7 Alpine is configured with a custom redis.conf from the config directory. It also has health checks via redis-cli ping.

Nginx acts as a reverse proxy, routing requests to the appropriate service. Its configuration lives in config/nginx.conf, with a production-specific configuration at config/nginx/catalogizer.prod.conf.

There are important environment variables you must set. At minimum, you need POSTGRES_PASSWORD -- the compose file will refuse to start without it. Other variables like POSTGRES_USER and POSTGRES_DB have sensible defaults.

There is also a docker-compose.build.yml for the containerized build pipeline. This creates a builder image with all required toolchains -- Go, Node.js, Rust, and Android SDK -- for reproducible cross-platform builds.

An important note for Podman users: always use fully qualified image names like docker.io/library/postgres:15-alpine rather than short names, as Podman without a TTY cannot resolve short names. Also, use --network host for builds to avoid SSL issues with package downloads.

### On-Screen Actions

- [00:00] Open terminal at project root
- [00:30] Check container runtime: `which podman && podman --version`
- [01:00] Show the docker-compose.dev.yml file structure
- [01:30] Run: `podman-compose -f docker-compose.dev.yml up`
- [02:30] Show containers starting in the terminal output
- [03:30] Open docker-compose.yml and walk through the PostgreSQL service configuration
- [05:00] Highlight the health check configuration and resource limits
- [06:00] Show the Redis service with config/redis.conf volume mount
- [07:00] Show the Nginx service and config/nginx.conf
- [07:30] Show config/nginx/catalogizer.prod.conf
- [08:00] Create a .env file with required variables: POSTGRES_PASSWORD, JWT_SECRET, TMDB_API_KEY
- [09:00] Show docker-compose.build.yml for the build pipeline
- [09:30] Run `podman-compose up` for production
- [10:30] Run `podman ps` to show all healthy containers
- [11:00] Open browser to http://localhost to verify the application loads
- [11:30] Show the API health endpoint

### Key Points

- Development: `podman-compose -f docker-compose.dev.yml up`
- Production: `podman-compose up` (requires .env with POSTGRES_PASSWORD)
- Build pipeline: `./scripts/container-build.sh` (generates the per-run env file docker-compose.build.yml requires)
- Services: PostgreSQL 15, Redis 7, Nginx reverse proxy, catalog-api, catalog-web
- Health checks configured for all database and cache services
- Config files for nginx and redis live in the config/ directory -- do not move them without updating volume mounts
- Podman users: use fully qualified image names and --network host for builds

### Tips

> **Tip**: For development, use the dev compose file. It has faster startup, hot reloading, and relaxed security settings. Never use the dev configuration in production.

> **Tip**: If you change config/nginx.conf or config/redis.conf, you need to restart the respective containers for changes to take effect.

> **Tip**: For Podman, set GOTOOLCHAIN=local in your build environment to prevent Go from auto-downloading newer toolchain versions inside containers.

### Quiz Questions

1. **Q**: What is the minimum required environment variable for the production Docker Compose?
   **A**: POSTGRES_PASSWORD is required; the compose file will refuse to start without it.

2. **Q**: Which container runtime does the project prefer when Docker is unavailable?
   **A**: Podman with podman-compose.

3. **Q**: Where do the Nginx and Redis configuration files live?
   **A**: In the config/ directory at the project root: config/nginx.conf and config/redis.conf.

---

## Lesson 1.4: Manual Installation (Backend & Frontend)

**Duration**: 15 minutes

### Narration

If you prefer running Catalogizer without containers, or need to set up a development environment for contributing, let us walk through the manual installation.

Start by cloning the repository. After cloning, initialize the submodules with git submodule init followed by git submodule update --recursive. This pulls in all the reusable modules: Auth, Cache, Database, Filesystem, and the rest.

Navigate to the catalog-api directory. Run go mod tidy to install all Go dependencies. This will pull in the Gin framework, database bindings, WebSocket libraries, and all other backend dependencies.

Next, set up your environment variables. Create a .env file in the catalog-api directory. The critical variables are: PORT for the API server (default 8080), JWT_SECRET for authentication tokens, and optionally DB_TYPE (defaults to sqlite). For development, SQLite requires no additional setup -- the database file is created automatically.

Start the API server with go run main.go. You should see the Gin server start on port 8080. Verify it works by accessing the API.

Now for the frontend. Navigate to the catalog-web directory and run npm install. Create a .env.local file. The key settings are VITE_API_BASE_URL pointing to your backend and VITE_WS_URL for WebSocket connections. There are also feature flags: VITE_ENABLE_ANALYTICS, VITE_ENABLE_REALTIME, VITE_ENABLE_EXTERNAL_METADATA, and VITE_ENABLE_OFFLINE_MODE.

Start the development server with npm run dev. The frontend will start on port 5173 with hot module replacement enabled. Open your browser and navigate to http://localhost:5173.

You can also build for production using npm run build, which generates optimized static files. Before deploying, run npm run lint and npm run type-check to ensure code quality.

For the first-time configuration, you will want to set your JWT_SECRET to a strong random string, configure any external API keys you want to use (TMDB_API_KEY is the most valuable for movie metadata), and set up your initial admin account.

SMB resilience configuration includes SMB_RETRY_ATTEMPTS (default 5), SMB_RETRY_DELAY_SECONDS (default 30) with exponential backoff, SMB_HEALTH_CHECK_INTERVAL (default 60), SMB_CONNECTION_TIMEOUT (default 30), and SMB_OFFLINE_CACHE_SIZE (default 1000).

### On-Screen Actions

- [00:00] Open terminal and clone the repository
- [00:30] Initialize submodules: `git submodule init && git submodule update --recursive`
- [01:00] Show the submodule directories appearing
- [01:30] `cd catalog-api && go mod tidy`
- [02:30] Create and edit the .env file with database, JWT, and API key configuration
- [04:00] `go run main.go` -- show server startup output
- [05:00] Verify the backend is running with a curl request
- [05:30] `cd ../catalog-web && npm install`
- [06:30] Show .env.example and create .env.local
- [07:30] Highlight the feature flags: analytics, realtime, external metadata, offline mode
- [08:30] `npm run dev` -- show Vite startup on port 5173
- [09:00] Open browser to http://localhost:5173 -- show the login page
- [09:30] Run `npm run lint && npm run type-check` to demonstrate quality checks
- [10:30] Run `npm run build` for production output
- [11:00] Show the build output directory
- [11:30] Configure SMB sources and resilience parameters
- [12:30] Configure external API keys: TMDB, Spotify, Steam
- [13:00] Restart the backend and watch sources being scanned
- [13:30] Open the web UI and watch media items populate
- [14:00] Quick verification: log in and see the dashboard

### Key Points

- Clone and initialize submodules: `git submodule init && git submodule update --recursive`
- Backend: `go mod tidy` -> configure .env -> `go run main.go` (port 8080)
- Frontend: `npm install` -> configure .env.local -> `npm run dev` (port 5173)
- SQLite is the default database for development -- no additional setup required
- Feature flags in .env.local control analytics, real-time updates, external metadata, and offline mode
- SMB resilience settings ensure Catalogizer handles network interruptions gracefully
- External API keys (TMDB, Spotify, Steam) enable rich metadata enrichment
- TMDB API key is free to obtain at themoviedb.org

### Tips

> **Tip**: Keep your JWT_SECRET safe and unique per environment. Use a strong random string of at least 32 characters.

> **Tip**: During development, enable all feature flags in .env.local so you can test the full functionality of the application.

> **Tip**: Start with a single storage source and verify it works before adding more. This makes troubleshooting easier if something goes wrong.

### Quiz Questions

1. **Q**: What command initializes all submodules after cloning?
   **A**: `git submodule init && git submodule update --recursive`

2. **Q**: What is the default database type for development?
   **A**: SQLite -- no additional setup required, the database file is created automatically.

3. **Q**: What are the default ports for the backend and frontend?
   **A**: Backend runs on port 8080, frontend runs on port 5173.
