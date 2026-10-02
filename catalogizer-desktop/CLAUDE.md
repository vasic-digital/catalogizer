# CLAUDE.md - Catalogizer Desktop

## Overview

Tauri 2 desktop application for browsing and managing Catalogizer media collections. React 18 frontend with a Rust backend that handles configuration persistence, HTTP proxying (with SSRF protection), and platform detection via IPC commands.

**Identifier**: `com.catalogizer.desktop` (Tauri 2 / Rust + React 18 / TypeScript / Vite)

## Build & Test

```bash
# Frontend
npm install
npm run dev                # Vite dev server (:1420)
npm run build              # tsc + vite build
npm run test               # vitest run
npm run test:watch         # vitest --watch
npm run test:coverage      # vitest --coverage

# Tauri (full desktop app)
npm run tauri:dev          # dev mode with hot reload
npm run tauri:build        # production build (AppImage/dmg/msi)

# Rust backend only
cd src-tauri
cargo test                 # unit tests
cargo build                # debug build
```

## Code Style

- **TypeScript**: strict mode, PascalCase components, camelCase functions. Tailwind CSS for styling
- **Rust**: edition 2021, `snake_case` functions, `PascalCase` structs. Serde for serialization
- Imports grouped: React/framework, third-party, internal
- Tests: Vitest + React Testing Library (frontend), `#[cfg(test)]` modules (Rust)

## Directory Structure

| Path | Purpose |
|------|---------|
| `src/pages/` | Route pages: Home, Library, Login, MediaDetail, Search, Settings |
| `src/components/` | Shared UI: Layout, LoadingScreen |
| `src/services/apiService.ts` | HTTP client for catalog-api communication |
| `src/stores/` | Zustand stores: `authStore`, `configStore` |
| `src/types/index.ts` | Media, auth, playback, config type definitions |
| `src/utils/cn.ts` | Tailwind class merge utility |
| `src/test-utils/` | Test helpers, mock data, custom render |
| `src-tauri/src/main.rs` | Rust backend: IPC commands, config state, HTTP proxy |
| `src-tauri/tauri.conf.json` | Tauri app config (window 1200x800, CSP, bundle) |
| `src-tauri/Cargo.toml` | Rust dependencies (tauri 2, reqwest, tokio, serde) |

## Key IPC Commands (Rust)

- `get_config` / `update_config` -- Read/write full `AppConfig` (server_url, auth_token, theme, auto_start)
- `set_server_url` / `set_auth_token` / `clear_auth_token` -- Granular config mutations
- `make_http_request` -- Proxied HTTP with SSRF validation (URL must match configured server)
- `get_app_version` / `get_platform` / `get_arch` -- System info

## Key Frontend Types

- `MediaItem`, `MediaVersion`, `ExternalMetadata` -- Media domain
- `User`, `LoginRequest`, `LoginResponse`, `AuthStatus` -- Authentication
- `AppConfig`, `SMBConfig`, `PlaybackProgress`, `DownloadJob` -- App state
- `MediaType`, `QualityLevel`, `SortOption`, `Theme` -- Union types

## Dependencies

- **Frontend**: React 18, React Router 6, React Query 4, Zustand, Tailwind CSS, Lucide icons
- **Rust**: tauri 2, reqwest 0.11, tokio, serde, env_logger

## Commit Style

Conventional Commits: `feat(desktop): add media detail view`


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

## Optional features and E2E

- Optional Cargo feature `vlc-player` (`libvlc-sys`) with sources in `src-tauri/src/vlc/`.
- Playwright E2E: `npm run test:e2e` and `npm run test:e2e:update` (snapshot refresh).
