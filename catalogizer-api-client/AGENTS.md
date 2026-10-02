# AGENTS.md — catalogizer-api-client Multi-Agent Coordination Guide

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working on the `catalogizer-api-client` TypeScript library. It defines responsibilities, module boundaries, and coordination protocols to prevent conflicts when multiple agents operate concurrently.

## Module Identity

- **Package**: `@catalogizer/api-client`
- **Language**: TypeScript (strict mode), target ES2020, CommonJS output
- **Runtime Dependencies**: `axios ^1.4.0`, `ws ^8.13.0`
- **Consumers**: `catalog-web`, `catalogizer-desktop`, `installer-wizard`

## Module Ownership Boundaries

| File | Scope | Boundary |
|---|---|---|
| `src/index.ts` | `CatalogizerClient` facade class + re-exports | Single entry point. All public API surfaces here. |
| `src/utils/http.ts` | `HttpClient` — Axios wrapper with auth, retry, refresh | Owns all HTTP transport. Do not create parallel Axios instances. |
| `src/utils/websocket.ts` | `WebSocketClient` — reconnecting WebSocket with typed events | Owns WebSocket lifecycle. |
| `src/services/AuthService.ts` | Login, logout, register, token refresh, profile | `/auth/*` endpoints |
| `src/services/MediaService.ts` | Media CRUD, search, stats, playback progress | `/api/v1/media/*` endpoints |
| `src/services/SMBService.ts` | SMB config management, status, scanning | `/api/v1/smb/*` endpoints |
| `src/types/index.ts` | All type/interface definitions and error classes | Keep dependency-free. |

## Agent Coordination Rules

### 1. Adding a new API endpoint

1. Add the method to the appropriate service class in `src/services/`.
2. If no suitable service exists, create a new one following the existing pattern (constructor takes `HttpClient`).
3. Wire the new service in `CatalogizerClient` (`src/index.ts`) as a property.
4. Add type definitions in `src/types/index.ts` for request/response shapes.
5. Add unit tests in `__tests__/`.

### 2. Error handling

The library uses a typed error hierarchy:
- `CatalogizerError` — base class
- `AuthenticationError` — 401 responses
- `NetworkError` — connection failures
- `ValidationError` — 400 responses

HTTP status codes are mapped to error subclasses in `HttpClient`. Do not throw raw `Error` or Axios errors — always map to the typed hierarchy.

### 3. Auth token lifecycle

- `HttpClient` injects the bearer token via a request interceptor.
- A response interceptor handles 401 by attempting a token refresh, then retrying the original request.
- If refresh fails, the client emits `auth:expired` via `EventEmitter` for UI-level session handling.
- Do not duplicate token management logic in consumers.

### 4. Downstream coordination

All React UI modules and Tauri apps that fetch data depend on this library:

| Consumer | Services Used |
|---|---|
| `catalog-web` | All services (Auth, Media, SMB) |
| `catalogizer-desktop` | Auth, Media |
| `installer-wizard` | Auth, SMB |

Breaking changes to method signatures, error classes, or event names require coordinated updates in all consumers.

### 5. Testing standards

- Vitest with Node environment.
- Tests co-located in `__tests__/` directories next to source.
- Mock Axios for HTTP tests — never make real network calls in unit tests.
- Test error mapping (each HTTP status code maps to the correct error subclass).

## Build & Validation Commands

```bash
npm install
npm run build        # tsc (outputs to dist/)
npm run dev          # tsc --watch
npm run test         # vitest run
npm run lint         # eslint src --ext .ts
```

## Commit Conventions

Conventional Commits:
- `feat(api-client): add collection endpoints`
- `fix(api-client): handle token refresh race condition`
- `test(api-client): add WebSocket reconnection coverage`

Every commit must:
- Pass `npm run build`.
- Pass `npm run test`.
- Pass `npm run lint`.
- End with the Co-Authored-By trailer when authored with an AI assistant.

## Constraints

- **API contract alignment**: Service methods must match the catalog-api REST endpoints exactly. When the backend adds or changes an endpoint, this library must be updated in sync.
- **No DOM access**: This is a pure HTTP/WebSocket client library. No React, no browser APIs. Must work in Node.js and browser environments.
- **No direct consumers**: Consumers import from `@catalogizer/api-client` — never from individual service files.

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

No TODOs, FIXMEs, empty implementations, silent error swallows, fake data, or empty catch blocks may be committed. The local anti-bluff scan and the dedicated commit/push validation script report them; no CI/CD pipeline is used (§11.4.156, §11.4.234). When an issue is found, fix all instances — not just the reported one.
