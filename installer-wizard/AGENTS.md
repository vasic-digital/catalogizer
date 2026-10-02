# AGENTS.md — installer-wizard Multi-Agent Coordination Guide

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working on the `installer-wizard` Tauri application. It defines responsibilities, layer boundaries, and coordination protocols to prevent conflicts when multiple agents operate concurrently on this module.

## Module Identity

- **Identifier**: `com.catalogizer.installer-wizard`
- **Languages**: TypeScript (frontend), Rust (backend)
- **Framework**: Tauri 2, React 18, Vite
- **State**: Zustand, React Hook Form + Zod
- **Styling**: Tailwind CSS
- **Testing**: Vitest + React Testing Library (frontend), `#[cfg(test)]` with `#[tokio::test]` (Rust)

## Layer Ownership Boundaries

### Frontend (TypeScript/React)

| Directory | Scope | Boundary |
|---|---|---|
| `src/components/wizard/` | Step components (Welcome, ProtocolSelection, NetworkScan, SMB/FTP/NFS/WebDAV/Local config, ConfigurationManagement, Summary) | One component per wizard step. Each step is self-contained. |
| `src/components/ui/` | Reusable UI primitives (Button, Card, Input) | Shared across wizard steps. Do not import from `wizard/`. |
| `src/components/layout/` | `WizardLayout` — step navigation chrome | Wraps all wizard steps. |
| `src/contexts/` | `WizardContext` (step state), `ConfigurationContext` (source/access state) | Wizard flow control. Changes affect all steps. |
| `src/services/tauri.ts` | Tauri IPC bridge for Rust commands | Single source of truth for IPC calls. |
| `src/types/index.ts` | Network, config, wizard, per-protocol connection types | Keep dependency-free. |

### Backend (Rust/Tauri)

| File | Scope | Boundary |
|---|---|---|
| `src-tauri/src/main.rs` | Entry point, IPC command registration, shared domain structs | All command handlers registered here. |
| `src-tauri/src/network.rs` | Network scanning (trust-dns, ipnetwork) | Host discovery on local network. |
| `src-tauri/src/smb.rs` | SMB share scanning, browsing, connection testing | SMB protocol operations. |
| `src-tauri/src/ftp.rs` | FTP connection testing | FTP protocol operations. |
| `src-tauri/src/nfs.rs` | NFS connection testing | NFS protocol operations. |
| `src-tauri/src/webdav.rs` | WebDAV connection testing | WebDAV protocol operations. |
| `src-tauri/src/local.rs` | Local filesystem validation | Local path validation. |

## Agent Coordination Rules

### 1. Wizard step flow

The wizard follows a fixed step sequence managed by `WizardContext`:
1. Welcome
2. Protocol Selection
3. Network Scan (for network protocols)
4. Protocol-specific configuration (SMB/FTP/NFS/WebDAV/Local)
5. Configuration Management (save/load)
6. Summary

When adding a new step:
1. Create the step component in `src/components/wizard/`.
2. Add the step to the `WizardStep` enum in `src/types/index.ts`.
3. Update `WizardContext` to include the new step in the navigation flow.
4. Add Vitest tests for the step component.

### 2. Adding a new protocol

1. Create the Rust connection tester in `src-tauri/src/{protocol}.rs`.
2. Add the `test_{protocol}_connection` IPC command in `main.rs`.
3. Register the command in `tauri::generate_handler!`.
4. Create the frontend config component in `src/components/wizard/{Protocol}Config.tsx`.
5. Add the connection type definition in `src/types/index.ts`.
6. Update `ProtocolSelection` to include the new option.
7. Update the TypeScript IPC bridge in `src/services/tauri.ts`.
8. Add tests for both Rust and TypeScript sides.

### 3. IPC commands

Current commands and their responsibilities:

| Command | Module | Purpose |
|---|---|---|
| `scan_network` | `network.rs` | Discover hosts on local network |
| `scan_smb_shares` | `smb.rs` | List SMB shares on a host |
| `browse_smb_share` | `smb.rs` | List files/directories in an SMB share |
| `test_smb_connection` | `smb.rs` | Validate SMB credentials and path |
| `test_ftp_connection` | `ftp.rs` | Validate FTP connection |
| `test_nfs_connection` | `nfs.rs` | Validate NFS connection |
| `test_webdav_connection` | `webdav.rs` | Validate WebDAV connection |
| `test_local_connection` | `local.rs` | Validate local filesystem path |
| `load_configuration` | `main.rs` | Read JSON config from disk |
| `save_configuration` | `main.rs` | Write JSON config to disk |
| `get_default_config_path` | `main.rs` | Returns `~/.catalogizer/config.json` |

### 4. Configuration persistence

- Configurations are saved/loaded as JSON files via Tauri IPC commands (`save_configuration` / `load_configuration`).
- Default path: `~/.catalogizer/config.json`.
- The `ConfigurationContext` manages the in-memory config state.
- Tauri plugins (`shell`, `dialog`, `fs`) provide file picker and save dialogs.

### 5. Form validation

- Use React Hook Form for form state in each protocol configuration step.
- Use Zod schemas for runtime validation of connection parameters.
- Validation errors must display inline next to the relevant field.

### 6. Testing standards

- **Frontend**: Vitest with jsdom. React Testing Library for step components. Mock Tauri IPC calls.
- **Rust**: `#[cfg(test)]` modules. `#[tokio::test]` for async commands. Test struct serialization, local connection validation, config path resolution.
- **Coverage**: `npm run test:coverage` for frontend.

## File Ownership

| File | Primary Concern | Cross-Module Impact |
|------|----------------|---------------------|
| `src-tauri/src/main.rs` | IPC registration, config I/O | All wizard steps that call IPC |
| `src/contexts/WizardContext.tsx` | Step navigation state | All step components |
| `src/contexts/ConfigurationContext.tsx` | Source/access configuration state | Protocol config steps and summary |
| `src/services/tauri.ts` | IPC bridge | All components calling Rust commands |
| `src/types/index.ts` | All type definitions | Entire frontend codebase |

## Build & Validation Commands

```bash
# Frontend only
npm install
npm run build                  # tsc + vite build
npm run test                   # vitest run
npm run test:coverage          # vitest --coverage

# Full desktop app
npm run tauri:dev              # dev mode with hot reload
npm run tauri:build            # production build

# Rust backend only
cd src-tauri
cargo test                     # unit tests
cargo build                    # debug build
```

## Commit Conventions

Conventional Commits:
- `feat(wizard): add WebDAV configuration step`
- `fix(wizard): correct network scan timeout`
- `test(wizard): add SMB connection validation tests`

Every commit must:
- Pass `npm run test` (frontend).
- Pass `cargo test` (Rust backend).
- End with the Co-Authored-By trailer when authored with an AI assistant.

## Constraints

- **Container builds**: Use Podman. Set `APPIMAGE_EXTRACT_AND_RUN=1` in containers (no FUSE available).
- **Tauri plugins**: Uses `shell` (open), `dialog` (file picker), `fs` (file access). Permissions configured in `tauri.conf.json`.
- **API keys**: Never commit `.env` files or hardcode secrets.
- **Rust error handling**: Uses `anyhow`/`thiserror` for errors. Never use `unwrap()` on fallible operations.

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
