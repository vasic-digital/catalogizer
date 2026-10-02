# CLAUDE.md - Catalogizer API Client

## Overview

`@catalogizer/api-client` is a cross-platform TypeScript client library for the Catalogizer REST API. It wraps HTTP (via Axios) and WebSocket communication, providing typed service classes for authentication, media operations, and SMB management. Used by catalog-web, catalogizer-desktop, and installer-wizard.

**Package**: `@catalogizer/api-client` (TypeScript, Node.js)

## Build & Test

> Development-run and test commands. Release artifacts (production builds, APKs, installers, packages) MUST be built only inside a rootless Podman build container (constitution §11.4.173); never use sudo.

```bash
npm install
npm run build        # tsc (outputs to dist/)
npm run dev          # tsc --watch
npm run test         # vitest run
npm run lint         # eslint src --ext .ts
```

## Code Style

- TypeScript strict mode, target ES2020, CommonJS output
- PascalCase classes, camelCase functions/variables
- Imports grouped: Node.js builtins, third-party, internal
- Tests: Vitest with Node environment, co-located in `__tests__/` directories

## Package Structure

| Path | Purpose |
|------|---------|
| `src/index.ts` | `CatalogizerClient` main class and re-exports |
| `src/services/AuthService.ts` | Login, logout, register, token refresh, profile |
| `src/services/MediaService.ts` | Media CRUD, search, stats, playback progress |
| `src/services/SMBService.ts` | SMB config management, status, scanning |
| `src/utils/http.ts` | `HttpClient` -- Axios wrapper with auth token, retry, refresh |
| `src/utils/websocket.ts` | `WebSocketClient` -- reconnecting WebSocket with typed events |
| `src/types/index.ts` | All type/interface definitions and custom error classes |

## Key Exports

- `CatalogizerClient` -- Main entry point; extends `EventEmitter` with typed events. Owns `auth`, `media`, `smb` service instances. Manages HTTP + WebSocket lifecycle via `connect()`/`disconnect()`
- `HttpClient` -- Axios wrapper with token injection, automatic refresh, configurable retry
- `WebSocketClient` -- Auto-reconnecting WebSocket emitting `download:progress`, `scan:progress` events
- `AuthService` / `MediaService` / `SMBService` -- Domain service classes
- Error hierarchy: `CatalogizerError` > `AuthenticationError`, `NetworkError`, `ValidationError`
- Types: `ClientConfig`, `MediaItem`, `User`, `LoginRequest`, `SMBConfig`, `ClientEvents`, etc.

## Dependencies

- **Runtime**: `axios ^1.4.0`, `ws ^8.13.0`
- **Dev**: `typescript ^5.0`, `vitest`, `eslint`

## Commit Style

Conventional Commits: `feat(api-client): add collection endpoints`


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


