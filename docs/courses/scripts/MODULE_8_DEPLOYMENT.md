# Module 8: Deployment and Production - Video Scripts

---

## Lesson 8.1: Container Builds and Podman

**Duration**: 15 minutes

### Narration

Welcome to Module 8, the second advanced module covering deployment and production operations. In this lesson, we are going to build Catalogizer for production using the containerized build pipeline.

Catalogizer uses Podman exclusively as its container runtime. Not Docker, not containerd -- Podman. This is a project-wide decision that applies everywhere. Podman is rootless by default, daemonless, and fully OCI-compatible. Every command you see in documentation and scripts uses podman or podman-compose, never docker or docker-compose.

The containerized build pipeline ensures reproducible builds across all 7 components. The builder image is based on Ubuntu 22.04 and contains Go 1.24.1, Node.js 18, Rust toolchain, JDK 17, Android SDK 34, and Playwright. It is approximately 4.82 GB. This image guarantees that everyone builds with the same toolchain versions regardless of their host environment.

There are several critical requirements when building containers with Podman. First, always use --network host for builds. The default container networking has SSL issues that cause downloads from dl.google.com, crates.io, and other registries to fail. Second, set GOTOOLCHAIN=local inside the container to prevent Go from auto-downloading newer toolchain versions. Third, use fully qualified image names like docker.io/library/golang:1.24 instead of just golang:1.24 -- short names fail without a TTY. Fourth, set APPIMAGE_EXTRACT_AND_RUN=1 for Tauri AppImage bundling because containers do not have FUSE support.

Let me walk through the build process. The release build script at scripts/release-build.sh orchestrates all 7 components. It uses the Build/ submodule framework which provides logging, container runtime detection, semantic versioning via versions.json, SHA256 change detection to skip unchanged components, and a CLI orchestrator.

Each component has its own build script in scripts/lib/: build-api.sh for the Go backend, build-web.sh for the React frontend, build-desktop.sh and build-wizard.sh for the Tauri apps, build-android.sh and build-androidtv.sh for the mobile apps, and build-api-client.sh for the TypeScript library.

To run a full release build: ./scripts/release-build.sh --container --force --skip-tests. The --container flag uses the builder container. The --force flag bypasses change detection and builds everything. The --skip-tests flag skips test execution during the build, useful when you have already verified tests separately. A full build takes approximately 17 minutes.

For building individual components without the full pipeline, you can use Podman directly. For the backend:

```bash
podman run --network host --rm \
  -v $(pwd)/catalog-api:/app:Z \
  -w /app \
  -e GOTOOLCHAIN=local \
  docker.io/library/golang:1.24 \
  go build -o catalog-api
```

For the frontend:

```bash
podman run --network host --rm \
  -v $(pwd)/catalog-web:/app:Z \
  -w /app \
  docker.io/library/node:18 \
  sh -c "npm install && npm run build"
```

The docker-compose.build.yml file defines the complete build pipeline as a Compose service. It mounts the source code, sets environment variables, and runs the build scripts inside the container.

### On-Screen Actions

- [00:00] Show title: "Container Builds and Podman"
- [00:30] Run `podman --version` and `podman-compose --version`
- [01:00] Explain why Podman over Docker: rootless, daemonless, OCI-compatible
- [01:30] Open the Dockerfile or builder image definition
- [02:00] Show the builder image contents: Go, Node, Rust, JDK, Android SDK
- [02:30] Explain --network host requirement with SSL issue example
- [03:00] Show GOTOOLCHAIN=local preventing toolchain auto-download
- [03:30] Show fully qualified image names requirement
- [04:00] Open scripts/release-build.sh -- show the orchestration logic
- [04:30] Open Build/lib/common.sh -- show logging and runtime detection
- [05:00] Open Build/lib/version.sh -- show semantic versioning from versions.json
- [05:30] Open Build/lib/hash.sh -- show SHA256 change detection
- [06:00] Open Build/lib/orchestrator.sh -- show CLI parsing and build loop
- [06:30] Open scripts/lib/build-api.sh -- show Go backend build script
- [07:00] Open scripts/lib/build-web.sh -- show React frontend build script
- [07:30] Show the remaining build scripts for desktop, mobile, and API client
- [08:00] Run the release build: `./scripts/release-build.sh --container --force --skip-tests`
- [09:00] Show build progress for each component
- [10:00] Show build artifacts in the output directory
- [10:30] Demonstrate building the backend individually with podman run
- [11:00] Demonstrate building the frontend individually with podman run
- [11:30] Open docker-compose.build.yml -- show the build pipeline definition
- [12:00] Explain resource limits: --cpus and --memory flags
- [12:30] Show `podman stats --no-stream` during a build
- [13:00] Show the builder container CPU at max 3 CPUs and 8 GB RAM
- [13:30] Explain the APPIMAGE_EXTRACT_AND_RUN=1 requirement for Tauri
- [14:00] Recap the build pipeline and critical requirements

### Key Points

- Podman exclusively: rootless, daemonless, OCI-compatible -- never Docker
- Builder image: 4.82 GB with Go 1.24.1, Node 18, Rust, JDK 17, Android SDK 34
- Critical: `podman build --network host` to avoid SSL issues
- Critical: `GOTOOLCHAIN=local` to prevent Go auto-downloading toolchains
- Critical: fully qualified image names (docker.io/library/...) for Podman
- Critical: `APPIMAGE_EXTRACT_AND_RUN=1` for Tauri AppImage in containers
- Build framework: Build/ submodule with versioning, change detection, orchestration
- Per-component builders in scripts/lib/build-*.sh
- Full build: `./scripts/release-build.sh --container --force --skip-tests` (~17 min)
- Resource limits: builder container max 3 CPUs, 8 GB RAM

### Tips

> **Tip**: Use the SHA256 change detection to skip unchanged components in development. Remove the --force flag and the build system will only rebuild what changed, significantly reducing build time.

> **Tip**: If a build fails with SSL errors, the first thing to check is whether you used --network host. This is the most common build failure cause.

---

## Lesson 8.2: Production Configuration

**Duration**: 14 minutes

### Narration

Now that we can build Catalogizer, let us configure it for production. Production deployment involves several infrastructure services beyond the application itself: PostgreSQL for the database, Redis for caching, and Nginx as a reverse proxy.

The production Docker Compose file is docker-compose.yml at the project root. Let me walk through each service.

PostgreSQL 15 Alpine is the production database. The configuration includes health checks using pg_isready that run every 10 seconds. Resource limits cap it at 1 CPU and 2 GB RAM. The data volume is persistent via a named volume. The container maps port 5432 internally to 5433 externally to avoid conflicts with any host PostgreSQL installation. Required environment variables: POSTGRES_PASSWORD, POSTGRES_DB, POSTGRES_USER.

Redis 7 Alpine provides caching. It uses config/redis.conf for configuration, has a health check using redis-cli ping, and is limited to appropriate resource constraints. Redis is optional -- the application functions without it but performs better with it.

Nginx serves as the reverse proxy. It uses config/nginx.conf and config/nginx/catalogizer.prod.conf for its configuration. Nginx handles TLS termination, static file serving, and request routing to the backend API. It also handles HTTP/3 (QUIC) protocol support and Brotli compression.

The catalog-api service is configured with several important environment variables. DB_TYPE must be set to postgres for production. DB_HOST, DB_PORT, DB_NAME, DB_USER, and DB_PASSWORD connect it to PostgreSQL. JWT_SECRET must be a strong, unique secret for token signing. GIN_MODE should be set to release for production. The API container needs --add-host=nas.local:192.168.0.241 if accessing NAS storage.

For HTTP/3 support, the backend uses quic-go/http3 with self-signed TLS certificates generated at startup. Brotli compression middleware using andybalholm/brotli compresses responses. The fallback chain is HTTP/3 with Brotli, then HTTP/2 with gzip. HTTP/1.1 is never acceptable in production.

Version injection happens at build time via Go ldflags. The Version, BuildNumber, and BuildDate variables are set with -ldflags "-X main.Version=... -X main.BuildNumber=... -X main.BuildDate=...".

For bare-metal deployment without containers, the systemd service file at config/systemd/catalogizer-api.service defines the service unit. It configures automatic restart, resource limits, environment file loading, and proper process management.

The .env file in catalog-api/ holds all configuration. Remember the precedence: environment variables override .env, which overrides config.json, which overrides defaults. In production, use a proper secrets manager rather than .env files for sensitive values like JWT_SECRET and database passwords.

### On-Screen Actions

- [00:00] Show title: "Production Configuration"
- [00:30] Open docker-compose.yml -- show the complete production stack
- [01:00] Walk through the PostgreSQL service definition
- [01:30] Show health check: pg_isready every 10 seconds
- [02:00] Show resource limits: cpus and memory
- [02:30] Show the persistent data volume
- [03:00] Walk through the Redis service definition
- [03:30] Open config/redis.conf -- show Redis configuration
- [04:00] Walk through the Nginx service definition
- [04:30] Open config/nginx.conf -- show reverse proxy configuration
- [05:00] Open config/nginx/catalogizer.prod.conf -- show production-specific config
- [05:30] Show TLS termination configuration
- [06:00] Walk through the catalog-api service definition
- [06:30] Show all required environment variables
- [07:00] Show the DB_TYPE=postgres configuration
- [07:30] Explain JWT_SECRET requirements for production
- [08:00] Show HTTP/3 and Brotli compression setup
- [08:30] Show version injection with ldflags
- [09:00] Open config/systemd/catalogizer-api.service -- show systemd unit
- [09:30] Show automatic restart and resource limit configuration
- [10:00] Show environment file loading in the systemd unit
- [10:30] Explain the configuration precedence: env vars > .env > config.json > defaults
- [11:00] Show a production .env file template (without real secrets)
- [11:30] Discuss secrets management best practices
- [12:00] Deploy: `podman-compose up -d`
- [12:30] Verify: `podman ps` and check health status
- [13:00] Recap production configuration requirements

### Key Points

- Production database: PostgreSQL 15 Alpine with health checks and persistent volumes
- Port mapping: internal 5432 to external 5433 to avoid host conflicts
- Redis: optional caching layer with config/redis.conf
- Nginx: reverse proxy with TLS termination, config/nginx.conf and config/nginx/catalogizer.prod.conf
- Required env vars: POSTGRES_PASSWORD, JWT_SECRET, DB_TYPE=postgres, GIN_MODE=release
- HTTP/3 (QUIC) mandatory in production with Brotli compression; fallback HTTP/2 + gzip
- Version injection via ldflags at build time
- Bare-metal: config/systemd/catalogizer-api.service for systemd deployment
- Config precedence: environment variables > .env > config.json > defaults
- Use a secrets manager in production, not plain .env files

### Tips

> **Tip**: Always test your production configuration locally first. Run podman-compose -f docker-compose.yml config --quiet to validate the Compose file without starting any containers.

> **Tip**: The config.json write_timeout must be set to 900 (not the default 30) if you plan to run long-running challenge suites in production. A 30-second write timeout will kill challenge RunAll operations.

---

## Lesson 8.3: Monitoring and Alerting

**Duration**: 14 minutes

### Narration

A production deployment is only as reliable as your ability to observe it. In this lesson, we set up comprehensive monitoring for Catalogizer using Prometheus and Grafana.

The backend exposes Prometheus-format metrics at the /metrics endpoint. The metrics system is implemented in internal/metrics/metrics.go and internal/metrics/middleware.go. The middleware automatically records HTTP request duration, request count by status code and method, and response sizes. Custom metrics track media detection pipeline throughput, scanner performance, WebSocket connection counts, and database query latency.

Prometheus configuration is at monitoring/prometheus.yml. This file defines the scrape targets -- at minimum, the catalog-api service. The scrape interval determines how often Prometheus collects metrics. For production, a 15-second interval balances detail with storage requirements.

To deploy Prometheus with Podman:

```bash
podman run -d --name prometheus \
  --network host \
  --cpus=1 --memory=2g \
  -v $(pwd)/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro \
  docker.io/prom/prometheus:latest
```

Grafana provides the visualization layer. Dashboard definitions are stored in monitoring/grafana/ and config/grafana-dashboards/. These pre-built dashboards show request rate, error rate, latency percentiles, resource usage, and application-specific metrics.

To deploy Grafana:

```bash
podman run -d --name grafana \
  --network host \
  --cpus=1 --memory=2g \
  -v $(pwd)/monitoring/grafana:/etc/grafana/provisioning:ro \
  docker.io/grafana/grafana:latest
```

For alerting, configure Prometheus alerting rules for critical conditions: service down (target health), high error rate (5xx responses exceeding a threshold), high latency (p99 response time exceeding a threshold), disk space running low, and high memory usage.

SMB connection monitoring deserves special attention. The circuit breaker in internal/smb/ tracks connection states. When an SMB share becomes unreachable, the circuit breaker opens after repeated failures, and the offline cache serves previously cached data. Monitor these transitions: circuit breaker state changes, offline cache hit rates, retry attempts, and exponential backoff delays. These metrics tell you when a storage source is having problems before users report it.

The media detection pipeline metrics show how many files are being processed, how long detection takes per file, and which providers are being called. Track provider latency separately -- if TMDB starts responding slowly, you want to know before it affects user experience.

Log aggregation should also be configured. Set LOG_LEVEL appropriately: debug for troubleshooting, info for normal operation, warn or error for production. Configure log rotation to prevent disk exhaustion. Consider shipping logs to a centralized system for search and analysis.

Resource monitoring is critical given the 30-40% host resource limit. Use podman stats --no-stream regularly. Set up alerts if total container CPU exceeds 4 CPUs or memory exceeds 8 GB. Monitor with cat /proc/loadavg for overall system load.

### On-Screen Actions

- [00:00] Show title: "Monitoring and Alerting"
- [00:30] Open a browser and navigate to the /metrics endpoint
- [01:00] Show the Prometheus metric format: HELP, TYPE, metric lines
- [01:30] Open internal/metrics/metrics.go -- show metric definitions
- [02:00] Open internal/metrics/middleware.go -- show the recording middleware
- [02:30] Show custom metrics: detection pipeline, scanner, WebSocket
- [03:00] Open monitoring/prometheus.yml -- show scrape configuration
- [03:30] Deploy Prometheus with podman run
- [04:00] Open Prometheus UI at localhost:9090
- [04:30] Run a query: `rate(http_request_duration_seconds_count[5m])`
- [05:00] Run another query: `histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))`
- [05:30] Deploy Grafana with podman run
- [06:00] Open Grafana at localhost:3000 (Grafana default port)
- [06:30] Add Prometheus as a data source
- [07:00] Open a pre-built dashboard from config/grafana-dashboards/
- [07:30] Show request rate, error rate, and latency panels
- [08:00] Show resource usage panels
- [08:30] Configure an alert: notify when error rate exceeds 1%
- [09:00] Show SMB circuit breaker metrics
- [09:30] Show offline cache hit rate metrics
- [10:00] Show media detection pipeline throughput metrics
- [10:30] Run `podman stats --no-stream` -- show container resource usage
- [11:00] Show total resource usage against the 4 CPU / 8 GB limit
- [11:30] Run `cat /proc/loadavg` -- show system load
- [12:00] Discuss log levels and rotation
- [12:30] Discuss centralized log aggregation options
- [13:00] Recap the complete monitoring stack

### Key Points

- Metrics endpoint: /metrics exposes Prometheus-format metrics from internal/metrics/
- Middleware auto-records: request duration, count by status/method, response size
- Prometheus: monitoring/prometheus.yml defines scrape targets and intervals
- Grafana: monitoring/grafana/ and config/grafana-dashboards/ provide pre-built dashboards
- Alerting: service health, error rate, latency, disk space, memory usage
- SMB monitoring: circuit breaker states, offline cache hits, retry metrics
- Detection pipeline: throughput, per-file duration, provider latency
- Resource limits: monitor against max 4 CPUs, 8 GB RAM across all containers
- Tools: `podman stats --no-stream` and `cat /proc/loadavg` for resource monitoring
- Logging: LOG_LEVEL configuration, log rotation, optional centralized aggregation

### Tips

> **Tip**: Set up the monitoring stack before you need it, not during an incident. Even a basic Prometheus and Grafana setup gives you invaluable insight into system behavior over time.

> **Tip**: The SMB circuit breaker metrics are your early warning system for storage issues. A rising count of circuit breaker openings means a storage source is becoming unreliable, even if users have not noticed yet.

---

## Lesson 8.4: Backup, Recovery, and Maintenance

**Duration**: 12 minutes

### Narration

In this final lesson, we cover the operational procedures that keep a production Catalogizer instance reliable: backups, disaster recovery, upgrades, and capacity planning.

The backup strategy has three tiers. Daily database backups capture all catalog data, user accounts, collections, favorites, and media metadata. For PostgreSQL, use pg_dump:

```bash
podman exec catalogizer-postgres \
  pg_dump -U catalogizer catalogizer_db > backup_$(date +%Y%m%d).sql
```

For SQLite, copy the database file while the application is running -- SQLite's WAL mode allows safe copies during operation. However, for maximum consistency, use the SQLite backup API or stop writes briefly.

Weekly configuration backups capture the .env file, config.json, nginx configuration, Redis configuration, Grafana dashboards, and Prometheus rules. Archive these together:

```bash
tar czf config_backup_$(date +%Y%m%d).tar.gz \
  catalog-api/.env catalog-api/config.json \
  config/ monitoring/
```

Monthly verification involves restoring a backup to a test environment and confirming the application starts, data is intact, and all challenges pass. This catches backup corruption before it matters.

For disaster recovery, there are three scenarios to plan for. Database corruption: restore from the most recent pg_dump. The migration system in database/migrations/ is versioned, with separate SQLite and PostgreSQL migration files per version. After restoring, the application checks migration versions and applies any needed updates.

Configuration loss: restore from the weekly config backup. The critical files are the .env (contains JWT_SECRET, database credentials), config.json (application settings), and the nginx and Redis configs. Without JWT_SECRET, all existing user sessions become invalid.

Media source failure: when a storage source goes offline, the circuit breaker opens and the offline cache serves cached data. Once the source is restored, the circuit breaker closes automatically and a rescan discovers any changes. No manual intervention is typically needed.

For upgrades, the container-based deployment supports rolling updates. Build the new version with the release pipeline. Stop the old container, start the new one. The database migration system handles schema changes automatically on startup. For zero-downtime upgrades with multiple API instances behind Nginx, update one instance at a time.

The PostgreSQL data volume (catalogizer-pgdata) is preserved across container recreations. Never delete this volume unless you intend to lose all data.

Capacity planning uses the analytics data. Monitor database size growth, storage usage trends, and request rates over time. Plan for additional storage before you run out. The media entity system tracks 11 types across the media_items table -- monitor row counts and query performance as the catalog grows.

Establish a runbook for common operational procedures: service restart, log investigation, backup verification, scaling, and incident response. Document the procedures and keep the runbook updated as the system evolves.

### On-Screen Actions

- [00:00] Show title: "Backup, Recovery, and Maintenance"
- [00:30] Show the three-tier backup strategy diagram
- [01:00] Run a PostgreSQL backup with pg_dump via podman exec
- [01:30] Show the backup file and its size
- [02:00] Show SQLite backup via file copy with WAL mode
- [02:30] Run a configuration backup with tar
- [03:00] Show the backup archive contents
- [03:30] Discuss monthly verification procedure
- [04:00] Demonstrate restoring a PostgreSQL backup:
- [04:30] `podman exec -i catalogizer-postgres psql -U catalogizer catalogizer_db < backup.sql`
- [05:00] Open database/migrations/ -- show versioned migration files
- [05:30] Show separate SQLite and PostgreSQL migration variants
- [06:00] Explain automatic migration on application startup
- [06:30] Discuss configuration loss recovery
- [07:00] Explain the critical role of JWT_SECRET
- [07:30] Show SMB circuit breaker recovery after source restoration
- [08:00] Demonstrate a container upgrade:
- [08:30] Stop old container, start new container, verify health
- [09:00] Show the PostgreSQL data volume persisting across recreations
- [09:30] Show database row counts for capacity planning
- [10:00] Show analytics data for growth trend monitoring
- [10:30] Discuss the operational runbook concept
- [11:00] Recap the backup, recovery, and maintenance procedures

### Key Points

- Three-tier backup: daily database, weekly configuration, monthly verification
- PostgreSQL backup: pg_dump via podman exec; SQLite: file copy with WAL mode safety
- Configuration backup: .env, config.json, config/, monitoring/ directories
- Monthly verification: restore to test environment, run challenges to confirm integrity
- Recovery scenarios: database corruption (pg_dump restore), config loss (.env restore), media source failure (circuit breaker auto-recovery)
- Database migrations: versioned, separate SQLite/PostgreSQL variants, auto-applied on startup
- Upgrades: build new version, stop old container, start new, migrations run automatically
- PostgreSQL data volume (catalogizer-pgdata): preserved across container recreations, never delete
- Capacity planning: monitor database size, storage usage, request rates via analytics
- Runbook: document restart, log investigation, backup verification, scaling, incident response

### Tips

> **Tip**: Test your restore procedure regularly. A backup you have never restored is a backup you cannot trust. The monthly verification catches corruption and procedural gaps.

> **Tip**: Keep the JWT_SECRET backed up separately and securely. If you lose it, every user must re-authenticate. If it is compromised, every token ever issued with it is potentially compromised.
