# Module 12: Deployment and DevOps - Script

**Duration**: 45 minutes
**Module**: 12 - Deployment and DevOps

---

## Scene 1: Container Strategy (0:00 - 20:00)

**[Visual: Container architecture diagram showing all services and their resource limits]**

**Narrator**: Welcome to Module 12, the final module. Catalogizer uses Podman exclusively for containerization -- no Docker. Every build, every service, and every test runs in containers with strict resource limits. Let us explore the container strategy.

**[Visual: Show the Docker Compose files]**

**Narrator**: The project defines six Docker Compose files, each for a specific purpose:

- `docker-compose.yml` -- Production stack
- `docker-compose.dev.yml` -- Development environment
- `docker-compose.build.yml` -- Containerized build pipeline
- `docker-compose.test.yml` -- Test stack (API, web, Playwright; host network)
- `docker-compose.test-infra.yml` -- Test infrastructure services
- `docker-compose.security.yml` -- Security scanning tools

**[Visual: Show resource limits from CLAUDE.md]**

**Narrator**: Resource limits are mandatory. The host machine runs other mission-critical processes, so all container workloads are limited to 30-40% of total resources:

- PostgreSQL: `--cpus=1 --memory=2g`
- API: `--cpus=2 --memory=4g`
- Web: `--cpus=1 --memory=2g`
- Builder: `--cpus=3 --memory=8g`
- Total budget: max 4 CPUs, 8 GB RAM across all running containers

```bash
# Monitor container resource usage
podman stats --no-stream
cat /proc/loadavg
```

**[Visual: Show multi-stage build]**

**Narrator**: The builder image is a multi-stage build: Ubuntu 22.04 base with Go 1.24.1, Node 18, Rust, JDK 17, Android SDK 34, and Playwright. It is 4.82 GB and builds all 7 components. Critical build flags:

```bash
# Must use host networking for builds (SSL issues with default networking)
podman build --network host -t catalogizer-builder .

# Must use host networking for running builds
podman run --network host catalogizer-builder

# Prevent Go from auto-downloading newer toolchains
GOTOOLCHAIN=local

# Use fully qualified image names (short names fail without TTY)
FROM docker.io/library/ubuntu:22.04
```

**[Visual: Show the Build framework submodule]**

**Narrator**: The `Build/` submodule provides a reusable shell-based build framework with four libraries:

- `common.sh` -- Logging, container runtime detection, git helpers
- `version.sh` -- Semantic versioning via `versions.json`
- `hash.sh` -- SHA256 change detection to skip unchanged components
- `orchestrator.sh` -- CLI parsing and build loop

```bash
# Source the framework in build scripts
source Build/lib/common.sh
source Build/lib/version.sh
source Build/lib/hash.sh
source Build/lib/orchestrator.sh
```

**[Visual: Show the release build script]**

**Narrator**: The release build orchestrates all 7 components through per-component builders in `scripts/lib/`: `build-api.sh`, `build-web.sh`, `build-desktop.sh`, `build-wizard.sh`, `build-android.sh`, `build-androidtv.sh`, and `build-api-client.sh`.

```bash
# Full release build (containerized, all 7 components)
./scripts/release-build.sh --container --force --skip-tests
# Completes in ~17 minutes
```

**[Visual: Show change detection with SHA256 hashing]**

**Narrator**: The hash library computes SHA256 hashes of each component's source files. If a component has not changed since the last build, it is skipped. This turns a 17-minute full build into a 2-minute incremental build when only one component changes.

---

## Scene 2: Installation Wizard (20:00 - 35:00)

**[Visual: Screenshot of the installer wizard application]**

**Narrator**: The installation wizard is a separate Tauri application that guides first-time users through setup. It replaces the complexity of manual configuration with a step-by-step UI.

**[Visual: Show the wizard flow]**

**Narrator**: The wizard flow has several stages:

1. **Environment Detection** -- Scans for available storage (NAS devices, mounted drives, network shares)
2. **Protocol Configuration** -- Helps configure SMB, FTP, NFS, WebDAV, or local storage connections
3. **Backend Setup** -- Configures the catalog-api: database type, port, JWT secret
4. **Connectivity Verification** -- Tests connections to all configured storage roots
5. **Initial Scan** -- Optionally triggers the first scan to populate the catalog
6. **Completion** -- Generates `config.json` and `.env` files

**[Visual: Show the configuration wizard service]**

**Narrator**: The backend supports the wizard through `services/configuration_wizard_service.go`, which provides endpoints for environment detection, configuration validation, and progress tracking.

**[Visual: Show the configuration service]**

**Narrator**: The `services/configuration_service.go` and `repository/configuration_repository.go` manage persistent configuration. The config precedence is: environment variables, then `.env` file, then `config.json`, then compiled defaults.

```bash
# Config precedence (highest to lowest)
1. Environment variables (export JWT_SECRET=...)
2. .env file (catalog-api/.env)
3. config.json (catalog-api/config.json)
4. Compiled defaults
```

**[Visual: Show first-time user experience]**

**Narrator**: On first launch without any configuration, the web application detects the unconfigured state and redirects to the setup wizard. The wizard is accessible both as a standalone desktop application and as a web-based flow within catalog-web.

---

## Scene 3: Production Considerations (35:00 - 45:00)

**[Visual: Production architecture diagram with PostgreSQL, Redis, reverse proxy]**

**Narrator**: Moving to production requires several configuration changes. Let us walk through the key decisions.

**[Visual: Show PostgreSQL migration]**

**Narrator**: Production uses PostgreSQL instead of SQLite. The migration is configuration-driven: set `DB_TYPE=postgres` and provide connection details. The dialect abstraction rewrites all SQL automatically. No code changes needed.

```bash
# Production database configuration
export DB_TYPE=postgres
export DB_HOST=db.example.com
export DB_PORT=5432
export DB_NAME=catalogizer
export DB_USER=catalogizer
export DB_PASSWORD=secure-password
```

**[Visual: Show WAL mode note for SQLite]**

**Narrator**: If you stay with SQLite for smaller deployments, the WAL (Write-Ahead Logging) mode is already enabled. This allows concurrent reads during writes, which is essential for any multi-user scenario.

**[Visual: Show the production compose file]**

**Narrator**: The production `docker-compose.yml` defines the complete stack: PostgreSQL with persistent volume, Redis for caching and rate limiting, the catalog-api with NAS host access, the catalog-web served through an HTTP/3-capable reverse proxy, and monitoring services.

**[Visual: Show NAS access configuration]**

**Narrator**: For NAS scanning in containers, the API container needs `--add-host=synology.local:192.168.0.241` to resolve NAS hostnames. SMB, NFS, and other protocols connect directly to the NAS from within the container.

**[Visual: Show backup strategies]**

**Narrator**: Backup strategies depend on the database:

- **PostgreSQL**: `pg_dump` for logical backups, continuous archiving with WAL shipping for point-in-time recovery
- **SQLite**: File-level backup of the `.db` file (ensure WAL checkpoint first)
- **Redis**: RDB snapshots or AOF persistence (optional, since Redis is used as a cache)
- **Media files**: Not backed up by Catalogizer -- they live on the NAS with its own backup strategy

**[Visual: Show horizontal scaling considerations]**

**Narrator**: For horizontal scaling, PostgreSQL is the prerequisite (SQLite does not support multiple writers). Multiple catalog-api instances can run behind a load balancer, sharing the same PostgreSQL database and Redis cache. WebSocket connections need sticky sessions or a dedicated pub/sub broker for cross-instance broadcasting.

**[Visual: Show monitoring alerts]**

**Narrator**: Production monitoring uses the Prometheus metrics from Module 11. Key alerts:

- Database query latency exceeding 100ms
- Error rate above 1% of requests
- SMB connection state degraded for more than 5 minutes
- Disk usage above 80%
- Memory usage above 80% of container limit
- Scan job stuck (no progress for 5 minutes)

**[Visual: Show the git remote strategy]**

**Narrator**: Code is pushed to six remotes: two GitHub, two GitLab, GitFlic, and GitVerse. This provides redundancy across hosting providers. Each push uses `GIT_SSH_COMMAND="ssh -o BatchMode=yes"` to prevent interactive prompts.

```bash
# Push to all 6 remotes
GIT_SSH_COMMAND="ssh -o BatchMode=yes" git push origin main

# GitVerse uses port 2222
ssh-keyscan -p 2222 gitverse.ru >> ~/.ssh/known_hosts
```

**[Visual: Show the complete service lifecycle]**

**Narrator**: The full service lifecycle:

```bash
# Start all services
./scripts/services-up.sh

# Run all tests + security scans
./scripts/run-all-tests.sh

# Build all 7 components
./scripts/release-build.sh --container --force

# Stop all services
./scripts/services-down.sh
```

**[Visual: Final course title card]**

**Narrator**: That completes the Catalogizer course. You have built a production-grade, multi-platform media collection manager: a Go backend with clean architecture and dual-dialect database support, a React frontend with real-time updates, desktop apps with Tauri, mobile apps with Kotlin Compose, five protocol implementations, HTTP/3 performance, 209 passing challenges, and a containerized deployment pipeline. Every component is tested, documented, and ready for production. Thank you for joining me on this journey.

---

## Key Code Examples

### Production Deployment
```bash
# Start production stack
podman-compose -f docker-compose.yml up -d

# Verify health
curl http://localhost:8080/api/v1/health

# Monitor resources
podman stats --no-stream
```

### Environment Variables (.env)
```env
# Production configuration
PORT=8080
GIN_MODE=release
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
DB_NAME=catalogizer
DB_USER=catalogizer
DB_PASSWORD=secure-password
JWT_SECRET=256-bit-cryptographic-random-secret
ADMIN_PASSWORD=<your-admin-password>
REDIS_ADDR=localhost:6379
```

### Container Resource Limits
```yaml
# docker-compose.yml
services:
  catalog-api:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
  postgres:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 2G
```

### Multi-Remote Push
```bash
# Add all hosts to known_hosts
ssh-keyscan github.com gitlab.com gitflic.ru >> ~/.ssh/known_hosts
ssh-keyscan -p 2222 gitverse.ru >> ~/.ssh/known_hosts

# Push to all 6 remotes
GIT_SSH_COMMAND="ssh -o BatchMode=yes" git push origin main
```

---

## Quiz Questions

1. Why does Catalogizer use Podman instead of Docker?
   **Answer**: Podman runs containers without a daemon (daemonless), does not require root privileges (rootless by default), and is fully compatible with Docker CLI commands and Compose files. It avoids the single-point-of-failure of the Docker daemon. The project configuration uses `podman` and `podman-compose` exclusively.

2. What is the purpose of SHA256 change detection in the build framework?
   **Answer**: The `hash.sh` library computes SHA256 hashes of each component's source files. Before building, it compares the current hash to the last build's hash. If unchanged, the component is skipped entirely. This turns a 17-minute full build into a fast incremental build when only one or two components have changed.

3. What must be done to migrate from SQLite to PostgreSQL in production?
   **Answer**: Set the `DB_TYPE=postgres` environment variable and provide connection details (DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD). The dialect abstraction automatically rewrites all SQL: `?` becomes `$1,$2,...`, `INSERT OR IGNORE` becomes `ON CONFLICT DO NOTHING`, and boolean literals are converted. No code changes are needed. Run migrations, which have separate PostgreSQL variants.

4. Why must container builds use `--network host` on this project?
   **Answer**: Default container networking has SSL/TLS issues when downloading dependencies from dl.google.com, crates.io, and other package registries. Using `--network host` bypasses the container network namespace, allowing builds to use the host's network stack directly. This resolves SSL certificate verification failures that occur with Podman's default network bridge.
