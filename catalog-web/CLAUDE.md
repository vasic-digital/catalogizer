# CLAUDE.md — catalog-web

## Overview

React 18 / TypeScript / Vite frontend for Catalogizer. Provides media browsing, search, collection management, dashboards, and a media player. Runs on port 3000 and proxies `/api` requests to the catalog-api backend.

## Commands

> Development-run and test commands. Release artifacts (production builds, APKs, installers, packages) MUST be built only inside a rootless Podman build container (constitution §11.4.173); never use sudo.

```bash
npm run dev                 # dev server on :3000 (proxies /api to catalog-api)
npm run build               # production build (tsc + vite)
npm run preview             # preview production build
npm run lint                # ESLint (zero warnings enforced)
npm run lint:fix            # ESLint with auto-fix
npm run type-check          # TypeScript type checking (tsc --noEmit)
npm run test                # unit tests (vitest, single run)
npm run test:watch          # unit tests (watch mode)
npm run test:coverage       # unit tests with coverage
npm run test:e2e            # Playwright E2E tests
npm run test:e2e:headed     # Playwright in headed browser
npm run format              # Prettier formatting
```

## Architecture

**AuthProvider -> WebSocketProvider -> Router -> Pages**

Auth-gated routes via `ProtectedRoute`. The app wraps the component tree in context providers for authentication and real-time WebSocket communication before rendering the router.

### Key Tech Stack

| Library | Purpose |
|---|---|
| React 18 | UI framework |
| TypeScript | Type safety |
| Vite 6 | Build tool and dev server |
| React Query (`@tanstack/react-query`) | Server state management |
| Zustand | Client state management |
| Tailwind CSS | Utility-first styling |
| React Hook Form + Zod | Form handling and validation |
| framer-motion | Animations |
| React Router DOM v6 | Routing |
| Recharts | Charts and analytics |
| axios | HTTP client |
| Vitest | Unit testing |
| Playwright | E2E testing |

### Local Submodule Dependencies

Linked via `file:../` in `package.json`:

- `@vasic-digital/auth-context` — Auth context provider
- `@vasic-digital/websocket-client` — WebSocket client with reconnection + React hooks
- `@vasic-digital/ui-components` — Shared UI component library
- `@vasic-digital/media-types` — Shared media type definitions
- `@vasic-digital/catalogizer-api-client` — TypeScript API client
- `@vasic-digital/media-browser` — Media browsing components
- `@vasic-digital/media-player` — Media playback components
- `@vasic-digital/collection-manager` — Collection management UI
- `@vasic-digital/dashboard-analytics` — Dashboard and analytics

### Path Aliases

Configured in `vite.config.ts` and `tsconfig.json`:

- `@/components`, `@/hooks`, `@/lib`, `@/types`
- `@/services`, `@/store`, `@/pages`, `@/assets`

### Source Directory Structure

| Directory | Purpose |
|---|---|
| `src/components/` | Reusable UI components |
| `src/pages/` | Route-level page components |
| `src/hooks/` | Custom React hooks |
| `src/lib/` | Utility functions and helpers |
| `src/types/` | TypeScript type definitions |
| `src/contexts/` | React context providers |
| `src/assets/` | Static assets (images, etc.) |
| `src/__tests__/` | Test files |
| `e2e/` | Playwright E2E test specs |

### API Proxy

The Vite dev server reads `../catalog-api/.service-port` at startup to discover the backend port, falling back to 8080. Both `/api` and `/ws` requests are proxied to the backend.

### Build Output

Production build splits into vendor chunks: `vendor` (react), `router`, `ui`, `charts`, `utils`. Output goes to `dist/` with sourcemaps enabled.

## Testing

- **Unit tests**: Vitest with jsdom environment. Setup file at `src/test-setup.ts`. Test files use `.test.ts`/`.test.tsx` extension.
- **E2E tests**: Playwright. Spec files in `e2e/` directory (excluded from Vitest via config). Run with `npm run test:e2e`.
- **Coverage**: `npm run test:coverage` generates coverage via `@vitest/coverage-v8`.

## Conventions

- **Components**: PascalCase filenames and exports (e.g., `MediaBrowser.tsx`).
- **Functions/hooks**: camelCase (e.g., `useMediaQuery`, `formatFileSize`).
- **Validation**: Zod schemas for runtime validation, React Hook Form for form state.
- **PostCSS config**: `postcss.config.js` must use `module.exports` (CommonJS) for Node 18 compatibility.
- **API URLs**: Must be relative (empty base) in dev mode so the Vite proxy works correctly.

## Constraints

- **Zero-warning policy**: ESLint runs with `--max-warnings 0`. No console errors or failed network requests allowed.
- **Port 3000 conflict**: Kill any process on port 3000 before starting (`ss -tlnp | grep :3000`).
- **Container builds**: Use Podman. `postcss.config.js` must be CommonJS.
- **API keys**: Never commit `.env` files with real secrets.


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


