# Catalogizer -- Developer Guide

## Table of Contents

1. [Development Environment Setup](#development-environment-setup)
2. [Architecture Overview](#architecture-overview)
3. [Project Structure](#project-structure)
4. [Backend Development (catalog-api)](#backend-development-catalog-api)
5. [Web Frontend Development (catalog-web)](#web-frontend-development-catalog-web)
6. [Desktop App Development (catalogizer-desktop)](#desktop-app-development-catalogizer-desktop)
7. [Android Development](#android-development)
8. [API Client Library (catalogizer-api-client)](#api-client-library-catalogizer-api-client)
9. [Submodule Architecture](#submodule-architecture)
10. [API Reference](#api-reference)
11. [Database Schema and Migrations](#database-schema-and-migrations)
12. [Testing](#testing)
13. [Debugging](#debugging)
14. [Extension Points](#extension-points)
15. [Contributing Guidelines](#contributing-guidelines)
16. [Code Conventions](#code-conventions)

---

## Development Environment Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.21+ | Backend API |
| Node.js | 18+ | Web frontend, desktop app, installer |
| npm | Bundled with Node.js | Package management |
| Rust + Cargo | Latest stable | Tauri desktop/installer builds |
| Android Studio | Latest | Android app development |
| Kotlin | Bundled with Android Studio | Android apps |
| SQLite3 | System default | Development database |
| Git | 2.x+ | Version control and submodules |

Optional:
- **PostgreSQL** 13+ (for production-style development)
- **Redis** 6+ (for distributed rate limiting testing)
- **Podman** or **Docker** (for containerized development)
- **FFmpeg** (for testing media conversion)

### Initial Setup

```bash
# Clone the repository, then initialise submodules WITHOUT a recursive update through `constitution/`
# (llmctl and claude_toolkit vendor a constitution of their own; see docs/GOVERNANCE.md)
git clone https://github.com/your-org/Catalogizer.git
cd Catalogizer
git submodule status | awk '$2 != "constitution" {print $2}' | xargs -r git submodule update --init --recursive --
git submodule update --init constitution
git -C constitution submodule update --init submodules/llmctl submodules/claude_toolkit submodules/open_design   # only those you need
```

### Running the Full Stack Locally

**Terminal 1 -- Backend:**
```bash
cd catalog-api
export JWT_SECRET="dev-secret-key-at-least-32-characters"
export ADMIN_USERNAME="admin"
export ADMIN_PASSWORD="admin123"
export GIN_MODE=debug
go run main.go
```

**Terminal 2 -- Frontend:**
```bash
cd catalog-web
npm install
npm run dev
```

**Access:**
- Frontend: `http://localhost:5173`
- API: `http://localhost:8080`
- Health: `http://localhost:8080/health`
- Metrics: `http://localhost:8080/metrics`

### Environment Variables for Development

Create a `.env` file in `catalog-api/`:

```env
# Server
PORT=8080
GIN_MODE=debug

# Database (SQLite by default, auto-created)
# DB_TYPE=sqlite

# Authentication
JWT_SECRET=dev-secret-key-at-least-32-characters-long
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin123

# External APIs (optional)
TMDB_API_KEY=your_tmdb_key
OMDB_API_KEY=your_omdb_key

# Redis (optional)
# REDIS_ADDR=localhost:6379
# REDIS_PASSWORD=
```

---

## Architecture Overview

### System Architecture

```
+-------------------+     +-------------------+     +-------------------+
|   catalog-web     |     | catalogizer-      |     | catalogizer-      |
|   (React/TS)      |     | desktop (Tauri)   |     | android (Kotlin)  |
+--------+----------+     +--------+----------+     +--------+----------+
         |                         |                          |
         |      HTTP/WebSocket     |    HTTP/REST             |
         +------------+------------+------------+-------------+
                      |                         |
              +-------v-------------------------v-------+
              |            catalog-api                   |
              |         (Go / Gin Framework)             |
              +-------+--------+--------+-------+-------+
                      |        |        |       |
              +-------v-+  +---v---+  +-v-----+ +v----------+
              |  SQLite  |  | Redis |  | TMDB  | | Storage   |
              |  / PgSQL |  | (opt) |  | IMDB  | | Protocols |
              +----------+  +-------+  +-------+ +-----------+
                                                  | SMB | FTP |
                                                  | NFS | DAV |
                                                  | Local     |
                                                  +-----------+
```

### Backend Architecture (catalog-api)

The backend follows a layered architecture:

```
HTTP Request
    |
    v
[Middleware] --> CORS, Logger, ErrorHandler, RequestID, InputValidation, Metrics
    |
    v
[Router] --> Gin routes (/api/v1/*)
    |
    v
[Handler] --> Request parsing, validation, response formatting
    |
    v
[Service] --> Business logic, orchestration
    |
    v
[Repository] --> Data access, SQL queries
    |
    v
[Database] --> SQLite / PostgreSQL
```

**Key architectural patterns:**

- **Constructor Injection** -- services are created with `NewService(dependencies...)` and passed to handlers
- **Service Adapters** -- bridge interface differences between packages (see `handlers/service_adapters.go`)
- **Repository Pattern** -- all database access goes through repository interfaces
- **Circuit Breaker** -- SMB connections use circuit breaker for fault tolerance
- **Event Bus** -- real-time updates are published via an internal event bus to WebSocket clients

### Frontend Architecture (catalog-web)

```
[AuthProvider] --> [WebSocketProvider] --> [Router]
                                            |
                                 [ProtectedRoute]
                                            |
                              [Page Components]
                                            |
                               [React Query] <--> [API]
```

- **React Query** -- server state management with caching and automatic refetching
- **AuthProvider** -- wraps the app with authentication context
- **WebSocketProvider** -- provides real-time event streaming
- **ProtectedRoute** -- guards routes requiring authentication

### Android Architecture

```
[Compose UI] --> [ViewModel (StateFlow)] --> [Repository] --> [Room DB + Retrofit]
                                                                     |
                                                              [Hilt DI Container]
```

- **MVVM** -- Model-View-ViewModel with unidirectional data flow
- **Jetpack Compose** -- declarative UI
- **Room** -- local database for offline caching
- **Retrofit** -- HTTP client for API communication
- **Hilt** -- dependency injection
- **StateFlow** -- reactive state management

---

## Project Structure

```
Catalogizer/
|-- catalog-api/              # Go backend API
|   |-- config/               # Configuration loading and validation
|   |-- database/             # Database connection and migrations
|   |-- handlers/             # HTTP request handlers (root-level)
|   |-- internal/
|   |   |-- auth/             # JWT authentication service
|   |   |-- config/           # Internal config types
|   |   |-- handlers/         # Internal handlers (catalog, download, copy, SMB)
|   |   |-- media/
|   |   |   |-- detector/     # Media type detection
|   |   |   |-- analyzer/     # Media content analysis
|   |   |   |-- providers/    # External metadata (TMDB, IMDB)
|   |   |   |-- realtime/     # Event bus to WebSocket
|   |   |-- metrics/          # Prometheus metrics
|   |   |-- middleware/       # Logger, error handler
|   |   |-- services/         # Internal services
|   |   |-- smb/              # SMB with circuit breaker + retry
|   |   |-- tests/            # Test helpers
|   |-- middleware/            # Root-level middleware (CORS, JWT, RequestID, InputValidation)
|   |-- repository/           # Data access layer
|   |-- services/             # Business logic services
|   |-- main.go               # Entry point, route registration
|
|-- catalog-web/              # React/TypeScript web frontend
|   |-- src/
|   |   |-- components/       # Reusable UI components
|   |   |   |-- admin/        # Admin panel components
|   |   |   |-- ai/           # AI dashboard components
|   |   |   |-- collections/  # Collection management
|   |   |   |-- conversion/   # Format conversion
|   |   |   |-- dashboard/    # Dashboard components
|   |   |   |-- favorites/    # Favorites management
|   |   |   |-- playlists/    # Playlist components
|   |   |   |-- ui/           # Base UI components (Button, Card, Input, etc.)
|   |   |-- lib/              # API client functions
|   |   |-- pages/            # Page-level components
|   |   |-- App.tsx           # Root component with routing
|
|-- catalogizer-desktop/      # Tauri desktop app
|   |-- src/                  # React frontend
|   |-- src-tauri/
|   |   |-- src/
|   |   |   |-- main.rs       # Rust IPC commands (get_config, save_config, test_connection)
|
|-- catalogizer-android/      # Android mobile app
|   |-- app/src/main/java/com/catalogizer/android/
|   |   |-- data/             # Data layer (Room, Retrofit, repositories)
|   |   |-- ui/               # Compose UI screens
|   |   |-- viewmodel/        # ViewModels
|
|-- catalogizer-androidtv/    # Android TV app
|   |-- (similar structure to android)
|
|-- catalogizer-api-client/   # TypeScript API client library
|   |-- src/services/         # Service classes for API endpoints
|
|-- installer-wizard/         # Tauri installer/setup wizard
|   |-- src/                  # React frontend
|   |-- src-tauri/src/        # Rust backend (network, SMB)
|
|-- config/                   # Infrastructure config (nginx, redis)
|-- scripts/                  # Shell scripts (install, setup, CI, testing)
|-- tests/                    # Standalone integration tests
|-- docs/                     # All documentation
|-- Assets/                   # Static assets
```

### Submodule Directories

The following directories are git submodules with independent repositories:

| Directory | Module | Language |
|-----------|--------|----------|
| `Auth/` | `digital.vasic.auth` | Go |
| `Cache/` | `digital.vasic.cache` | Go |
| `Database/` | `digital.vasic.database` | Go |
| `Concurrency/` | `digital.vasic.concurrency` | Go |
| `Storage/` | `digital.vasic.storage` | Go |
| `EventBus/` | `digital.vasic.eventbus` | Go |
| `Streaming/` | `digital.vasic.streaming` | Go |
| `Security/` | `digital.vasic.security` | Go |
| `Observability/` | `digital.vasic.observability` | Go |
| `Formatters/` | `digital.vasic.formatters` | Go |
| `Plugins/` | `digital.vasic.plugins` | Go |
| `Challenges/` | `digital.vasic.challenges` | Go |
| `Filesystem/` | `digital.vasic.filesystem` | Go |
| `RateLimiter/` | `digital.vasic.ratelimiter` | Go |
| `Config/` | `digital.vasic.config` | Go |
| `Discovery/` | `digital.vasic.discovery` | Go |
| `Media/` | `digital.vasic.media` | Go |
| `Middleware/` | `digital.vasic.middleware` | Go |
| `Watcher/` | `digital.vasic.watcher` | Go |
| `WebSocket-Client-TS/` | `@vasic-digital/websocket-client` | TypeScript |
| `UI-Components-React/` | `@vasic-digital/ui-components` | React/TS |
| `Android-Toolkit/` | Android-Toolkit | Kotlin |

---

## Backend Development (catalog-api)

### Adding a New Handler

1. Create a handler file in `catalog-api/handlers/` (for root-level) or `catalog-api/internal/handlers/` (for internal):

```go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type MyHandler struct {
    service MyServiceInterface
}

func NewMyHandler(service MyServiceInterface) *MyHandler {
    return &MyHandler{service: service}
}

func (h *MyHandler) GetItems(c *gin.Context) {
    items, err := h.service.GetItems(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, items)
}
```

2. Register the route in `main.go`:

```go
myHandler := handlers.NewMyHandler(myService)
api.GET("/my-resource", myHandler.GetItems)
```

### Adding a New Service

Create a service in `catalog-api/services/`:

```go
package services

type MyService struct {
    repo MyRepository
}

func NewMyService(repo MyRepository) *MyService {
    return &MyService{repo: repo}
}

func (s *MyService) GetItems(ctx context.Context) ([]Item, error) {
    return s.repo.FindAll(ctx)
}
```

### Adding a New Repository

Create a repository in `catalog-api/repository/`:

```go
package repository

import "database/sql"

type MyRepository struct {
    db *sql.DB
}

func NewMyRepository(db *sql.DB) *MyRepository {
    return &MyRepository{db: db}
}

func (r *MyRepository) FindAll(ctx context.Context) ([]Item, error) {
    rows, err := r.db.QueryContext(ctx, "SELECT id, name FROM items")
    if err != nil {
        return nil, fmt.Errorf("failed to query items: %w", err)
    }
    defer rows.Close()

    var items []Item
    for rows.Next() {
        var item Item
        if err := rows.Scan(&item.ID, &item.Name); err != nil {
            return nil, fmt.Errorf("failed to scan item: %w", err)
        }
        items = append(items, item)
    }
    return items, nil
}
```

### Adding a New Storage Protocol

Implement the `UnifiedClient` interface defined in `filesystem/interface.go`:

```go
type MyProtocolClient struct {
    // protocol-specific fields
}

func NewMyProtocolClient(config map[string]interface{}) (*MyProtocolClient, error) {
    // Initialize from config
}

// Implement all UnifiedClient interface methods
func (c *MyProtocolClient) List(path string) ([]FileInfo, error) { ... }
func (c *MyProtocolClient) Read(path string) (io.ReadCloser, error) { ... }
// ... etc
```

Register the protocol in `filesystem/factory.go`.

### Service Adapters

When services from different packages have incompatible interfaces, use adapter structs (see `handlers/service_adapters.go`):

```go
type AuthServiceAdapter struct {
    Inner *services.AuthService
}

// Implement the handler's expected interface by delegating to Inner
func (a *AuthServiceAdapter) ValidateToken(token string) (*User, error) {
    return a.Inner.ValidateToken(token)
}
```

---

## Web Frontend Development (catalog-web)

### Development Server

```bash
cd catalog-web
npm install
npm run dev    # Starts Vite dev server on :5173
```

### Project Commands

```bash
npm run dev          # Development server with hot reload
npm run build        # Production build
npm run test         # Run tests (Vitest, watch mode)
npm run test -- --run  # Run tests (single run)
npm run lint         # ESLint
npm run type-check   # TypeScript type checking
```

### Adding a New Page

1. Create a page component in `src/pages/`:

```tsx
// src/pages/MyPage.tsx
import React from 'react';

export default function MyPage() {
  return (
    <div className="container mx-auto p-6">
      <h1 className="text-2xl font-bold">My Page</h1>
      {/* Page content */}
    </div>
  );
}
```

2. Add the route in `src/App.tsx` inside the `ProtectedRoute`:

```tsx
<Route path="/my-page" element={<MyPage />} />
```

3. Add a navigation link in the nav bar component.

### Adding an API Call

Create or update an API function in `src/lib/api.ts`:

```typescript
export async function getMyResource(): Promise<MyResource[]> {
  const response = await fetch('/api/v1/my-resource', {
    headers: {
      'Authorization': `Bearer ${getToken()}`,
    },
  });
  if (!response.ok) throw new Error('Failed to fetch');
  return response.json();
}
```

Use React Query for data fetching in components:

```tsx
import { useQuery } from '@tanstack/react-query';
import { getMyResource } from '../lib/api';

function MyComponent() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['my-resource'],
    queryFn: getMyResource,
  });

  if (isLoading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;
  return <div>{/* render data */}</div>;
}
```

### UI Components

Base UI components are in `src/components/ui/`. These include Button, Card, Input, Badge, Progress, Select, Switch, Tabs, and Textarea. Prefer using these over raw HTML elements for consistent styling.

---

## Desktop App Development (catalogizer-desktop)

### Development

```bash
cd catalogizer-desktop
npm install
npm run tauri:dev    # Starts Tauri in development mode
```

### Build

```bash
npm run tauri:build
# Output: src-tauri/target/release/
```

### IPC Commands

The Rust backend in `src-tauri/src/main.rs` exposes IPC commands that the React frontend invokes:

| Command | Description |
|---------|-------------|
| `get_config` | Retrieves stored configuration from OS secure storage |
| `save_config` | Persists configuration to OS secure storage |
| `test_connection` | Tests connectivity to the Catalogizer server |

**Adding a new IPC command:**

1. Define the command in `src-tauri/src/main.rs`:

```rust
#[tauri::command]
fn my_command(param: String) -> Result<String, String> {
    // Implementation
    Ok("result".to_string())
}
```

2. Register it in the Tauri builder:

```rust
.invoke_handler(tauri::generate_handler![get_config, save_config, test_connection, my_command])
```

3. Call it from React:

```typescript
import { invoke } from '@tauri-apps/api/core';

const result = await invoke<string>('my_command', { param: 'value' });
```

---

## Android Development

### Setup

1. Open `catalogizer-android/` (or `catalogizer-androidtv/`) in Android Studio.
2. Let Gradle sync dependencies.
3. Connect a device or start an emulator.
4. Run the app.

### Building

```bash
cd catalogizer-android
./gradlew assembleDebug      # Debug APK
./gradlew assembleRelease    # Release APK
./gradlew test               # Unit tests
```

### Architecture

The Android app follows MVVM:

- **UI Layer** (`ui/`) -- Jetpack Compose screens observe ViewModel state via `StateFlow`
- **ViewModel Layer** (`viewmodel/`) -- exposes state and handles user actions
- **Data Layer** (`data/`) -- Repository pattern with Room (local) and Retrofit (remote)
- **DI Layer** -- Hilt modules provide dependencies

### Adding a New Screen

1. Create a Composable in `ui/`:

```kotlin
@Composable
fun MyScreen(viewModel: MyViewModel = hiltViewModel()) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
        Text("My Screen", style = MaterialTheme.typography.headlineMedium)
        // UI content
    }
}
```

2. Create a ViewModel:

```kotlin
@HiltViewModel
class MyViewModel @Inject constructor(
    private val repository: MyRepository
) : ViewModel() {
    private val _state = MutableStateFlow(MyState())
    val state: StateFlow<MyState> = _state.asStateFlow()

    init { loadData() }

    private fun loadData() {
        viewModelScope.launch {
            repository.getData().collect { result ->
                _state.update { it.copy(data = result) }
            }
        }
    }
}
```

3. Add navigation to the screen in the navigation graph.

### Offline Support

The sync mechanism uses `SyncOperation` entities stored in Room:

```kotlin
@Entity(tableName = "sync_operations")
data class SyncOperation(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val type: String,          // "favorite", "rating", "progress"
    val mediaId: String,
    val payload: String,       // JSON payload
    val createdAt: Long,
    val status: String = "pending"  // pending, synced, failed
)
```

When connectivity returns, the sync worker processes pending operations.

---

## API Client Library (catalogizer-api-client)

### Building

```bash
cd catalogizer-api-client
npm install
npm run build
npm run test
```

### Usage

The API client provides typed service classes for all API endpoints:

```typescript
import { CatalogizerClient } from '@catalogizer/api-client';

const client = new CatalogizerClient({
  baseURL: 'http://localhost:8080',
  token: 'your-jwt-token',
});

// Search media
const results = await client.search.query('matrix');

// Get media details
const media = await client.media.getById('abc123');

// List collections
const collections = await client.collections.list();
```

---

## Submodule Architecture

### Working with Submodules

```bash
# Initialize submodules after cloning (never `--recursive` through `constitution/`; see docs/GOVERNANCE.md, section 5)
git submodule status | awk '$2 != "constitution" {print $2}' | xargs -r git submodule update --init --recursive --
git submodule update --init constitution

# Add a new submodule
./scripts/setup-submodule.sh ModuleName [--create-repos] [--go|--ts|--kotlin]

# Push a submodule to all upstream remotes
cd SubmoduleName && commit "message"

# Install upstream remotes for a submodule
cd SubmoduleName && install_upstreams
```

### Submodule Development Workflow

1. Navigate into the submodule directory
2. Create a branch, make changes, and commit
3. Push to the submodule's remote repositories
4. Return to the parent repo and commit the submodule reference update

```bash
cd Filesystem/
git checkout -b feature/new-protocol
# make changes
git add . && git commit -m "Add new protocol support"
git push origin feature/new-protocol

cd ..
git add Filesystem
git commit -m "Update Filesystem submodule with new protocol"
```

---

## API Reference

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/login` | Login, returns JWT token |
| POST | `/api/v1/auth/register` | Register a new user |
| POST | `/api/v1/auth/refresh` | Refresh an expired token |
| POST | `/api/v1/auth/logout` | Invalidate current token |
| GET | `/api/v1/auth/me` | Get current user info (auth required) |

### Catalog Browsing

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/catalog` | List root catalog entries |
| GET | `/api/v1/catalog/*path` | List entries at a specific path |
| GET | `/api/v1/catalog-info/*path` | Get file info for a path |

### Search

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/search` | Search across media library |
| GET | `/api/v1/search/duplicates` | Find duplicate files |

### Downloads

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/download/file/:id` | Download a single file |
| GET | `/api/v1/download/directory/*path` | Download a directory |
| POST | `/api/v1/download/archive` | Download as archive |

### File Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/copy/storage` | Copy file to storage |
| POST | `/api/v1/copy/local` | Copy file to local path |
| POST | `/api/v1/copy/upload` | Upload file from local |
| GET | `/api/v1/storage/list/*path` | List storage path contents |
| GET | `/api/v1/storage/roots` | Get configured storage roots |

### Media

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/media/:id` | Get media item by ID |
| PUT | `/api/v1/media/:id/progress` | Update watch progress |
| PUT | `/api/v1/media/:id/favorite` | Toggle favorite status |

### Recommendations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/recommendations/similar/:media_id` | Get similar items |
| GET | `/api/v1/recommendations/trending` | Get trending items |
| GET | `/api/v1/recommendations/personalized/:user_id` | Get personalized recs |

### Subtitles

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/subtitles/search` | Search for subtitles |
| POST | `/api/v1/subtitles/download` | Download a subtitle |
| GET | `/api/v1/subtitles/media/:media_id` | Get subtitles for media |
| GET | `/api/v1/subtitles/:id/verify-sync/:media_id` | Verify subtitle sync |
| POST | `/api/v1/subtitles/translate` | Translate a subtitle |
| POST | `/api/v1/subtitles/upload` | Upload a subtitle |
| GET | `/api/v1/subtitles/languages` | List supported languages |
| GET | `/api/v1/subtitles/providers` | List supported providers |

### Statistics

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/stats/overall` | Overall library stats |
| GET | `/api/v1/stats/smb/:smb_root` | Per-SMB-root stats |
| GET | `/api/v1/stats/filetypes` | File type distribution |
| GET | `/api/v1/stats/sizes` | Size distribution |
| GET | `/api/v1/stats/duplicates` | Duplicate statistics |
| GET | `/api/v1/stats/duplicates/groups` | Top duplicate groups |
| GET | `/api/v1/stats/access` | Access patterns |
| GET | `/api/v1/stats/growth` | Growth trends |
| GET | `/api/v1/stats/scans` | Scan history |
| GET | `/api/v1/stats/directories/by-size` | Directories by size |
| GET | `/api/v1/stats/duplicates/count` | Duplicate count |

### SMB Discovery

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/smb/discover` | Discover SMB shares |
| GET | `/api/v1/smb/discover` | Discover shares (GET) |
| POST | `/api/v1/smb/test` | Test SMB connection |
| GET | `/api/v1/smb/test` | Test connection (GET) |
| POST | `/api/v1/smb/browse` | Browse an SMB share |

### Conversion

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/conversion/jobs` | Create conversion job |
| GET | `/api/v1/conversion/jobs` | List conversion jobs |
| GET | `/api/v1/conversion/jobs/:id` | Get specific job |
| POST | `/api/v1/conversion/jobs/:id/cancel` | Cancel a job |
| GET | `/api/v1/conversion/formats` | Supported formats |

### User Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/users` | Create user |
| GET | `/api/v1/users` | List users |
| GET | `/api/v1/users/:id` | Get user |
| PUT | `/api/v1/users/:id` | Update user |
| DELETE | `/api/v1/users/:id` | Delete user |
| POST | `/api/v1/users/:id/reset-password` | Reset password |
| POST | `/api/v1/users/:id/lock` | Lock account |
| POST | `/api/v1/users/:id/unlock` | Unlock account |

### Roles

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/roles` | Create role |
| GET | `/api/v1/roles` | List roles |
| GET | `/api/v1/roles/:id` | Get role |
| PUT | `/api/v1/roles/:id` | Update role |
| DELETE | `/api/v1/roles/:id` | Delete role |
| GET | `/api/v1/roles/permissions` | List permissions |

### Configuration

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/configuration` | Get configuration |
| POST | `/api/v1/configuration/test` | Test configuration |
| GET | `/api/v1/configuration/status` | System status |
| GET | `/api/v1/configuration/wizard/step/:id` | Get wizard step |
| POST | `/api/v1/configuration/wizard/step/:id/validate` | Validate step |
| POST | `/api/v1/configuration/wizard/step/:id/save` | Save step |
| GET | `/api/v1/configuration/wizard/progress` | Wizard progress |
| POST | `/api/v1/configuration/wizard/complete` | Complete wizard |

### Error Reporting

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/errors/report` | Report an error |
| POST | `/api/v1/errors/crash` | Report a crash |
| GET | `/api/v1/errors/reports` | List error reports |
| GET | `/api/v1/errors/reports/:id` | Get error report |
| PUT | `/api/v1/errors/reports/:id/status` | Update status |
| GET | `/api/v1/errors/crashes` | List crash reports |
| GET | `/api/v1/errors/crashes/:id` | Get crash report |
| PUT | `/api/v1/errors/crashes/:id/status` | Update crash status |
| GET | `/api/v1/errors/statistics` | Error statistics |
| GET | `/api/v1/errors/crash-statistics` | Crash statistics |
| GET | `/api/v1/errors/health` | System health |

### Log Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/logs/collect` | Create log collection |
| GET | `/api/v1/logs/collections` | List collections |
| GET | `/api/v1/logs/collections/:id` | Get collection |
| GET | `/api/v1/logs/collections/:id/entries` | Get entries |
| POST | `/api/v1/logs/collections/:id/export` | Export logs |
| GET | `/api/v1/logs/collections/:id/analyze` | Analyze logs |
| POST | `/api/v1/logs/share` | Create shared link |
| GET | `/api/v1/logs/share/:token` | Access shared logs |
| DELETE | `/api/v1/logs/share/:id` | Revoke shared link |
| GET | `/api/v1/logs/stream` | Stream logs |
| GET | `/api/v1/logs/statistics` | Log statistics |

### Public Endpoints (No Auth)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

---

## Database Schema and Migrations

Migrations run automatically on server startup via `databaseDB.RunMigrations(ctx)`. The migration system creates all necessary tables for:

- Users and authentication
- Files and media metadata
- Collections and favorites
- Conversion jobs
- Error and crash reports
- Log management
- Analytics data
- Sync operations
- Configuration

To check the current schema:

```bash
sqlite3 /path/to/catalogizer.db ".schema"
```

Test helpers for setting up in-memory SQLite databases for testing are in `catalog-api/internal/tests/test_helper.go`.

---

## Testing

### Backend Tests (Go)

```bash
cd catalog-api

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -v -run TestFunctionName ./path/to/package/

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Test conventions:**
- Test files: `*_test.go` alongside source files
- Table-driven tests preferred
- Use `internal/tests/test_helper.go` for database test setup
- Constructor injection makes mocking straightforward

### Frontend Tests (React/TypeScript)

```bash
cd catalog-web

# Run tests in watch mode
npm run test

# Run tests once
npm run test -- --run

# Run a specific test file
npm run test -- --run src/components/ui/__tests__/Button.test.tsx
```

**Test conventions:**
- Test files: `__tests__/*.test.tsx` or `*.test.ts`
- Use Vitest as the test runner
- React Testing Library for component tests

### Desktop App Tests

```bash
cd catalogizer-desktop
npm run test -- --run
```

### Installer Wizard Tests

```bash
cd installer-wizard
npm run test -- --run
```

### Android Tests

```bash
cd catalogizer-android
./gradlew test           # Unit tests
./gradlew connectedTest  # Instrumented tests (requires device/emulator)
```

### API Client Tests

```bash
cd catalogizer-api-client
npm run build && npm run test
```

### Running All Tests

```bash
./scripts/run-all-tests.sh
```

---

## Debugging

### Backend Debugging

**Enable debug logging:**
```bash
export GIN_MODE=debug
```

The server uses structured logging via the `internal/logging` package (built on Zap). In debug mode, all requests and responses are logged with details.

See [Structured Logging Guide](guides/STRUCTURED_LOGGING.md) for complete documentation on logging best practices and API usage.

**View server logs:**
```bash
# If running directly
# Logs go to stdout in JSON format

# If running as systemd service
journalctl -u catalogizer -f
```

**Test mode:**
```bash
./catalog-api --test-mode
```

Enables additional logging for diagnostic purposes.

**Database queries:**
```bash
sqlite3 /path/to/catalogizer.db
sqlite> .headers on
sqlite> .mode column
sqlite> SELECT * FROM files LIMIT 10;
```

### Frontend Debugging

- Open browser DevTools (F12)
- React Query DevTools are available in development mode
- Network tab shows API requests and responses
- Console shows WebSocket connection events

### Desktop App Debugging

- Tauri development mode includes DevTools
- Rust backend logs are in the terminal
- IPC calls can be traced in the browser console

### Android Debugging

- Use Android Studio Logcat for runtime logs
- Room database can be inspected via Database Inspector
- Network calls can be monitored via Android Studio Network Profiler

---

## Extension Points

### Adding a New Storage Protocol

1. Implement the `UnifiedClient` interface in `filesystem/`
2. Register it in `filesystem/factory.go`
3. Add protocol-specific settings to `StorageRootConfig`

### Adding a New Media Provider

1. Create a provider in `internal/media/providers/`
2. Implement the provider interface
3. Register it in the media detection pipeline

### Adding a New Subtitle Provider

1. Implement the subtitle provider interface in the subtitle service
2. Register it in the provider list

### Adding Custom Middleware

1. Create middleware function matching `gin.HandlerFunc`
2. Register it in `main.go` with `router.Use()`

### Adding New Metrics

1. Define Prometheus metrics in `internal/metrics/`
2. Instrument the relevant code paths
3. Metrics automatically appear at `/metrics`

---

## Contributing Guidelines

### Branch Naming

- `feature/short-description` -- new features
- `fix/short-description` -- bug fixes
- `refactor/short-description` -- code refactoring
- `docs/short-description` -- documentation changes
- `test/short-description` -- test additions or modifications

### Commit Messages

Use conventional commits:

```
feat: add subtitle translation endpoint
fix: resolve SQLite busy timeout during concurrent scans
refactor: extract media detection into separate service
docs: update API reference with new endpoints
test: add integration tests for SMB discovery
```

### Code Review Checklist

- All tests pass
- New code has appropriate test coverage
- No sensitive data (passwords, keys) in committed code
- Error handling follows existing patterns (error wrapping)
- API changes are backward compatible or version-bumped
- Documentation updated for user-facing changes

### Important Restrictions

- **GitHub Actions are disabled.** Do not create workflow files in `.github/workflows/`. All CI/CD runs locally.
- **Use Podman** when Docker is unavailable.
- **Do not commit** `.env` files, credentials, or release artifacts.

---

## Code Conventions

### Go

- `NewService(deps)` constructor injection pattern
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Table-driven tests
- `*_test.go` files alongside source
- `context.Context` as first parameter for I/O operations
- Structured logging with `zap`

### TypeScript/React

- PascalCase for components (`MediaBrowser.tsx`)
- camelCase for functions and variables
- Zod for runtime validation
- React Hook Form for form handling
- React Query for server state
- Vitest for testing

### Kotlin/Android

- MVVM architecture
- `Result` sealed classes for error handling
- Room for local persistence
- `StateFlow` for reactive state
- Hilt for dependency injection
- Compose for UI

### Configuration

- Environment variables override config file values
- Config file > defaults for non-sensitive settings
- Sensitive values (secrets, passwords) should use environment variables exclusively

---

*For server administration, see the [Admin Guide](ADMIN_GUIDE.md). For end-user documentation, see the [User Guide](USER_GUIDE.md). For quick reference, see the [Quick Reference](QUICK_REFERENCE.md).*
