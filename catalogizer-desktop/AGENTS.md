# AGENTS.md — catalogizer-desktop Multi-Agent Coordination Guide

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working on the `catalogizer-desktop` Tauri application. It defines responsibilities, layer boundaries, and coordination protocols to prevent conflicts when multiple agents operate concurrently on this module.

## Module Identity

- **Identifier**: `com.catalogizer.desktop`
- **Languages**: TypeScript (frontend), Rust (backend)
- **Framework**: Tauri 2, React 18, Vite
- **State**: React Query (server), Zustand (client)
- **Styling**: Tailwind CSS
- **Testing**: Vitest + React Testing Library (frontend), `#[cfg(test)]` modules (Rust)

## Layer Ownership Boundaries

### Frontend (TypeScript/React)

| Directory | Scope | Boundary |
|---|---|---|
| `src/pages/` | Route pages (Home, Library, Login, MediaDetail, Search, Settings) | One component per route. Compose from `src/components/`. |
| `src/components/` | Shared UI components (Layout, LoadingScreen) | Reusable across pages. Do not import from `src/pages/`. |
| `src/services/apiService.ts` | HTTP client for catalog-api communication | Single source of truth for API calls. Uses Tauri's `make_http_request` IPC for proxied requests. |
| `src/stores/` | Zustand stores (`authStore`, `configStore`) | Client-side state only. |
| `src/types/index.ts` | Media, auth, playback, config type definitions | Keep dependency-free. |
| `src/utils/cn.ts` | Tailwind class merge utility | Pure function. |
| `src/test-utils/` | Test helpers, mock data, custom render | Test infrastructure only. |

### Backend (Rust/Tauri)

| File | Scope | Boundary |
|---|---|---|
| `src-tauri/src/main.rs` | IPC commands, config state, HTTP proxy | All Tauri command handlers live here. |
| `src-tauri/tauri.conf.json` | Window config (1200x800), CSP, bundle settings | App identity and permissions. |
| `src-tauri/Cargo.toml` | Rust dependencies (tauri 2, reqwest, tokio, serde) | Dependency management. |

## Agent Coordination Rules

### 1. IPC command changes

When adding or modifying a Tauri IPC command:

1. Define the Rust handler function in `src-tauri/src/main.rs` with `#[tauri::command]`.
2. Register it in the `.invoke_handler(tauri::generate_handler![...])` call.
3. Update the TypeScript bridge in `src/services/` to call the new command via `@tauri-apps/api/core`.
4. Add Rust unit tests in a `#[cfg(test)]` module.
5. Add TypeScript tests for the bridge function.

Current IPC commands:
- `get_config` / `update_config` — Read/write full `AppConfig`
- `set_server_url` / `set_auth_token` / `clear_auth_token` — Granular config mutations
- `make_http_request` — Proxied HTTP with SSRF validation (URL must match configured server)
- `get_app_version` / `get_platform` / `get_arch` — System info

### 2. HTTP proxy and SSRF protection

All HTTP requests from the frontend to catalog-api go through the `make_http_request` Rust command. This command validates that the target URL matches the configured server URL to prevent SSRF attacks. Do not bypass this by making direct HTTP requests from the frontend.

### 3. Configuration persistence

- `AppConfig` (server URL, auth token, theme, auto-start) is managed by the Rust backend.
- Frontend reads/writes config exclusively through `get_config` / `update_config` IPC commands.
- Do not persist config on the frontend side (no localStorage for auth tokens).

### 4. Adding a new page

1. Create the page component in `src/pages/`.
2. Add the route in the React Router configuration.
3. Wire authentication guards if needed.
4. Add Vitest + React Testing Library tests.

### 5. Testing standards

- **Frontend**: Vitest with jsdom. React Testing Library for component tests. Mock Tauri IPC calls in tests.
- **Rust**: `#[cfg(test)]` modules with `#[test]` and `#[tokio::test]` for async commands.
- **Coverage**: `npm run test:coverage` for frontend.

## File Ownership

| File | Primary Concern | Cross-Module Impact |
|------|----------------|---------------------|
| `src-tauri/src/main.rs` | All Rust IPC handlers, config state | Frontend service layer depends on these commands |
| `src-tauri/tauri.conf.json` | Window size, CSP, bundle config | Build output and runtime behavior |
| `src/services/apiService.ts` | HTTP client bridging to Rust proxy | All pages that fetch data |
| `src/stores/authStore.ts` | Auth state (token, user) | All authenticated pages |
| `src/stores/configStore.ts` | App config (server URL, theme) | Settings page, API service |

## Build & Validation Commands

```bash
# Frontend only
npm install
npm run build                  # tsc + vite build
npm run test                   # vitest run
npm run test:coverage          # vitest --coverage

# Full desktop app
npm run tauri:dev              # dev mode with hot reload
npm run tauri:build            # production build (AppImage/dmg/msi)

# Rust backend only
cd src-tauri
cargo test                     # unit tests
cargo build                    # debug build
```

## Commit Conventions

Conventional Commits:
- `feat(desktop): add media detail view`
- `fix(desktop): correct SSRF validation for IPv6`
- `test(desktop): add config IPC round-trip tests`

Every commit must:
- Pass `npm run test` (frontend).
- Pass `cargo test` (Rust backend).
- End with the Co-Authored-By trailer when authored with an AI assistant.

## Constraints

- **SSRF protection**: The `make_http_request` command validates URLs against the configured server. Never bypass this.
- **Container builds**: Use Podman. Set `APPIMAGE_EXTRACT_AND_RUN=1` in containers (no FUSE available).
- **API keys**: Never commit `.env` files or hardcode secrets.
- **Tauri CSP**: Content Security Policy in `tauri.conf.json` restricts network access. Update if adding new external endpoints.

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

## MANDATORY: Zero Unfinished Work

No TODOs, FIXMEs, empty implementations, silent error swallows, fake data, or panic-prone `unwrap()` may be committed. The local anti-bluff scan and the dedicated commit/push validation script report them; no CI/CD pipeline is used (§11.4.156, §11.4.234). When an issue is found, fix all instances — not just the reported one.
