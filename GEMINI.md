# GEMINI.md - Catalogizer

> ## INHERITED FROM submodules/constitution/GEMINI.md
> Base agent rules live in `submodules/constitution/GEMINI.md` — READ IT
> FIRST. The base file (and `submodules/constitution/Constitution.md`) is
> authoritative for any topic not covered here. Project-specific rules
> below extend them; they never weaken them.

## Project Overview

Catalogizer is a comprehensive media collection management system that automatically detects, categorizes, and organizes your media files across multiple storage protocols including SMB, FTP, NFS, WebDAV, and local filesystem. It provides real-time monitoring, advanced analytics, and a modern web interface for managing your entire media library.

The project is composed of several components:

*   **`catalog-api`**: A Go-based REST API server that handles the core logic of the system, including media detection, metadata fetching, and real-time updates.
*   **`catalog-web`**: A React-based web application that provides a modern and responsive user interface for managing the media library.
*   **Client Applications**: The project also includes several client applications for different platforms, including Android, Android TV, and desktop.
*   **`installer-wizard`**: A graphical installation wizard for easy SMB configuration.

## Building and Running

> **Development-run only.** These commands start the apps for local development. Per §11.4.173 every
> build that produces a release artifact MUST run inside a rootless Podman build container on the
> designated build host (via the `containers` submodule), never on the bare host. Never use `sudo`.

### Backend (`catalog-api`)

1.  **Navigate to the `catalog-api` directory:**
    ```bash
    cd catalog-api
    ```
2.  **Install Go dependencies:**
    ```bash
    go mod tidy
    ```
3.  **Run the API server:**
    ```bash
    go run main.go
    ```

### Frontend (`catalog-web`)

1.  **Navigate to the `catalog-web` directory:**
    ```bash
    cd catalog-web
    ```
2.  **Install dependencies:**
    ```bash
    npm install
    ```
3.  **Start the development server:**
    ```bash
    npm run dev
    ```

### Docker

The project also includes a Docker-based deployment option.

1.  **Navigate to the `deployment` directory:**
    ```bash
    cd deployment
    ```
2.  **Start the services:**
    ```bash
    podman compose up -d
    ```

## Development Conventions

*   **Go**: The Go code follows standard Go conventions and is formatted with `gofmt`.
*   **TypeScript/React**: The frontend code follows standard React and TypeScript conventions and is linted with ESLint and formatted with Prettier.
*   **Testing**: The project includes a comprehensive test suite for both the backend and the frontend.
    *   **Backend tests**: Run with `go test ./...` in the `catalog-api` directory.
    *   **Frontend tests**: Run with `npm test` in the `catalog-web` directory.
*   **Git**: The project uses Git for version control. Commit messages should follow the conventional commits specification.

**§11.4.173 — Containerized + distributed build mandate (User mandate, 2026-06-29).** EVERY build of EVERY component (source compile, artifact/package/installer/container-image production, codegen, asset render — for ANY language/platform: Go, Android/Gradle, desktop, web, native, firmware) MUST run INSIDE a specialized build container provisioned via the `digital.vasic.containers` submodule (§11.4.76) — NEVER on the bare host. The build containers MUST be DISTRIBUTED to the designated remote build host(s) (e.g. `thinker.local`) via the SAME containers-submodule distribution mechanism the infra uses (§11.4.76 Distributor / remote compose over SSH, §11.4.161 rootless), so the build EXECUTES on the remote build host (offloading the developer/main host); once the build completes the produced artifacts MUST be brought BACK to the originating main host (scp/rsync/volume copy) for use/flashing/distribution. Building outside a container, or on the bare host, is FORBIDDEN — a release blocker (the "works on my machine" / unreproducible-build class §11.4.76 exists to prevent). The build-host target + build-container definitions are config-injected (§11.4.28, never hardcoded in the submodule); a missing build-container capability is added by EXTENDING the containers submodule upstream (§11.4.74), never by an ad-hoc host build. Honest boundary (§11.4.6): the containerized+distributed build guarantees reproducibility + host-isolation + capacity offload — it does NOT replace the §11.4.40 full-suite retest, §11.4.108 four-layer artifact→runtime verification, or §11.4.38 installable-asset evidence (those still run against the brought-back artifact). Classification: universal (§11.4.17). Composes §11.4.76 (containers submodule — sole orchestration layer), §11.4.161 (rootless runtime), §11.4.74 (extend-don't-reimplement), §11.4.28 (config injection / decoupling), §11.4.24 (build-resource stats), §11.4.82 (iteration speedup — persistent caches in the container), §11.4.121 (no-commit-while-build-writes-artifacts), §11.4.38 (installable-asset evidence on the brought-back artifact), §11.4.108 (artifact→runtime verification), §12.6 (host memory ceiling — offloading the build preserves the main host). Propagation gate `CM-COVENANT-114-173-PROPAGATION` (literal `11.4.173`) + recommended gate `CM-CONTAINERIZED-DISTRIBUTED-BUILD` (every build runs via the containers submodule on the remote build host; a bare-host build is detected + FAILs) + paired §1.1 mutation. **Canonical authority:** constitution submodule [`Constitution.md`](Constitution.md) §11.4.173. Non-compliance is a release blocker. No escape hatch — no `--build-on-host`, `--skip-container-build`, `--local-build-ok`, `--no-distributed-build`, `--bare-host-build` flag.
