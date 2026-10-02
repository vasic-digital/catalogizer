# AGENTS.md — catalog-web Multi-Agent Coordination Guide

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working on the `catalog-web` React frontend. It defines responsibilities, directory boundaries, and coordination protocols to prevent conflicts when multiple agents operate concurrently on this module.

## Module Identity

- **Package**: `catalog-web`
- **Language**: TypeScript (strict mode)
- **Framework**: React 18, Vite 6, React Router DOM v6
- **State**: React Query (server), Zustand (client)
- **Styling**: Tailwind CSS
- **Testing**: Vitest (unit), Playwright (E2E)
- **Depends on**: 9 `@vasic-digital/*` submodule packages via `file:../` links

## Directory Ownership Boundaries

| Directory | Scope | Boundary |
|---|---|---|
| `src/pages/` | Route-level page components | One component per route. Pages compose components from `src/components/` — do not inline complex UI here. |
| `src/components/` | Reusable UI components | Shared across pages. Do not import from `src/pages/`. |
| `src/hooks/` | Custom React hooks | Must not contain UI rendering logic. Do not import components. |
| `src/lib/` | Utility functions and helpers | Pure functions with no React dependencies except `cn()` for Tailwind class merging. |
| `src/types/` | TypeScript type definitions | Keep dependency-free — no runtime imports. |
| `src/contexts/` | React context providers | Auth and WebSocket contexts. Changes here affect the entire component tree. |
| `src/store/` | Zustand stores | Client-side state only. Server state belongs in React Query hooks. |
| `src/services/` | API communication layer | Uses `@vasic-digital/catalogizer-api-client`. Do not duplicate HTTP logic. |
| `src/assets/` | Static assets (images, icons) | No code files. |
| `src/__tests__/` | Unit test files | Mirror the source structure. |
| `e2e/` | Playwright E2E test specs | Excluded from Vitest via config. |

## Dependency Graph

```
App.tsx
 ├── src/contexts/          (AuthProvider, WebSocketProvider)
 ├── src/pages/             (route-level components)
 │    └── src/components/   (shared UI)
 │         └── src/hooks/   (custom hooks)
 │              ├── src/services/  (API layer)
 │              └── src/store/     (Zustand client state)
 ├── src/lib/               (pure utilities)
 └── src/types/             (type definitions)
```

No file in `src/types/` or `src/lib/` may import from `src/components/`, `src/pages/`, or `src/contexts/`.

## Local Submodule Dependencies

These are linked via `file:../` in `package.json`. Changes in any submodule may require `npm install` to refresh the link.

| Package | Role |
|---|---|
| `@vasic-digital/auth-context` | Auth context provider (login, logout, token refresh) |
| `@vasic-digital/websocket-client` | WebSocket client with reconnection + React hooks |
| `@vasic-digital/ui-components` | Shared UI component library (Button, Card, Input, etc.) |
| `@vasic-digital/media-types` | Shared media type definitions |
| `@vasic-digital/catalogizer-api-client` | TypeScript API client for catalog-api REST endpoints |
| `@vasic-digital/media-browser` | Media browsing components |
| `@vasic-digital/media-player` | Media playback components |
| `@vasic-digital/collection-manager` | Collection management UI |
| `@vasic-digital/dashboard-analytics` | Dashboard and analytics components |

## Agent Coordination Rules

### 1. State management

- **Server state**: Use React Query (`@tanstack/react-query`) for all data fetched from the API. Do not cache API responses in Zustand.
- **Client state**: Use Zustand for UI-only state (theme, sidebar open, modal state). Do not put server data in Zustand stores.
- **Auth state**: Managed exclusively by `AuthProvider` in `src/contexts/`. Do not create parallel auth mechanisms.

### 2. Adding a new page/route

1. Create the page component in `src/pages/`.
2. Register the route in the router configuration (React Router DOM v6).
3. Wrap with `ProtectedRoute` if the page requires authentication.
4. Add Vitest unit tests in `src/__tests__/`.
5. Add Playwright E2E tests in `e2e/` if the page has user flows.

### 3. Adding a new component

1. Create in `src/components/` with a PascalCase filename.
2. Accept `className` prop and merge with `cn()` from `@/lib/utils`.
3. Export TypeScript props interface alongside the component.
4. Add unit tests using Vitest + React Testing Library.

### 4. API communication

- Use `@vasic-digital/catalogizer-api-client` for all REST API calls. Do not use raw `fetch` or `axios` directly.
- Wrap API calls in React Query hooks for caching, refetching, and error handling.
- API URLs must be relative (empty base) in dev mode so the Vite proxy works correctly.
- The Vite dev server reads `../catalog-api/.service-port` at startup for the proxy target (falls back to 8080).

### 5. Path aliases

All imports should use the configured path aliases from `vite.config.ts` and `tsconfig.json`:

- `@/components`, `@/hooks`, `@/lib`, `@/types`
- `@/services`, `@/store`, `@/pages`, `@/assets`

Do not use relative paths like `../../components/` — use `@/components/` instead.

### 6. Forms and validation

- Use React Hook Form for form state management.
- Use Zod schemas for runtime validation.
- Do not hand-roll form state with `useState` for non-trivial forms.

### 7. Testing standards

- **Unit tests**: Vitest with jsdom environment. Setup file at `src/test-setup.ts`. Use React Testing Library (`@testing-library/react`). Prefer `screen.getByRole` over `getByTestId`.
- **E2E tests**: Playwright. Spec files in `e2e/`. Run with `npm run test:e2e`.
- **Coverage**: `npm run test:coverage` via `@vitest/coverage-v8`. Package-level coverage must not drop below the current baseline.

## File Ownership

| File | Primary Concern | Cross-Module Impact |
|------|----------------|---------------------|
| `src/App.tsx` | Root component, provider wrapping, router | ALL components — changes affect the entire app |
| `vite.config.ts` | Path aliases, API proxy, build chunks | Build output and dev server behavior |
| `tsconfig.json` | TypeScript compiler config, path aliases | All TypeScript files |
| `src/contexts/` | Auth and WebSocket providers | All authenticated components and real-time features |
| `tailwind.config.js` | Tailwind theme and plugins | All styled components |
| `postcss.config.js` | PostCSS pipeline | Must use `module.exports` (CommonJS) for Node 18 compat |

## Build & Validation Commands

```bash
# Lint + type-check + build + test
npm run lint                    # ESLint --max-warnings 0
npm run type-check              # tsc --noEmit
npm run build                   # tsc + vite production build
npm run test                    # vitest (single run)

# Single test file
npx vitest run path/to/file.test.ts

# E2E
npm run test:e2e                # Playwright headless
npm run test:e2e:headed         # Playwright with visible browser

# Coverage
npm run test:coverage
```

## Commit Conventions

Conventional Commits:
- `feat(web): add entity detail page`
- `fix(web): correct proxy target resolution`
- `test(web): add media browser coverage`
- `refactor(web): extract search filter hook`

Every commit must:
- Pass `npm run lint` (zero warnings).
- Pass `npm run type-check`.
- Pass affected unit tests.
- End with the Co-Authored-By trailer when authored with an AI assistant.

## Constraints

- **Zero-warning policy**: ESLint runs with `--max-warnings 0`. Zero console errors, zero failed network requests in the browser.
- **Port 3000 conflict**: Kill any process on port 3000 before starting (`ss -tlnp | grep :3000`).
- **PostCSS**: `postcss.config.js` must use `module.exports` (CommonJS), not `export default` (ESM), for Node 18 compatibility.
- **API URLs**: Must be relative (empty base) in dev mode for the Vite proxy to work.
- **Container builds**: Use Podman exclusively.
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

## MANDATORY: Zero Unfinished Work

No TODOs, FIXMEs, empty implementations, silent error swallows, fake data, or empty catch blocks may be committed. The local anti-bluff scan and the dedicated commit/push validation script report them; no CI/CD pipeline is used (§11.4.156, §11.4.234). When an issue is found, fix all instances — not just the reported one.
