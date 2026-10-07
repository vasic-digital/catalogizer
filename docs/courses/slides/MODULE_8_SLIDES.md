# Module 8: Deployment and Production - Slide Outlines

---

## Slide 8.0.1: Title Slide

**Title**: Deployment and Production

**Subtitle**: Container Builds, Production Configuration, Monitoring, and Maintenance

**Speaker Notes**: This advanced module covers everything needed to deploy and operate Catalogizer in production. Students should have completed Modules 5 and 6 before starting. By the end, students will be able to deploy, monitor, back up, and maintain a production Catalogizer instance.

---

## Slide 8.1.1: Podman as Container Runtime

**Title**: Why Podman, Not Docker

**Bullet Points**:
- **Rootless by default**: no daemon, no root privilege escalation
- **Daemonless**: no background process, each container is a child process
- **OCI-compatible**: uses the same image format and registry protocol
- **Pod support**: group related containers like Kubernetes pods
- **Project mandate**: every command uses `podman` / `podman-compose`, never Docker

**Speaker Notes**: The choice of Podman is deliberate and project-wide. It is not interchangeable with Docker in this project. Scripts, documentation, and CI all reference Podman explicitly. The rootless default is a security advantage for production deployments.

---

## Slide 8.1.2: Builder Image and Critical Requirements

**Title**: Containerized Build Pipeline

**Bullet Points**:
- Builder image: ~4.82 GB (Ubuntu 22.04)
  - Go 1.24.1, Node.js 18, Rust, JDK 17, Android SDK 34, Playwright
- **Critical**: `podman build --network host` (default networking has SSL issues)
- **Critical**: `GOTOOLCHAIN=local` (prevents Go auto-downloading newer toolchains)
- **Critical**: Fully qualified image names (`docker.io/library/...`) -- short names fail without TTY
- **Critical**: `APPIMAGE_EXTRACT_AND_RUN=1` (Tauri AppImage, no FUSE in containers)

**Speaker Notes**: These four critical requirements cause the most common build failures. If a build fails, check these first. The SSL issue with default networking is particularly subtle -- downloads appear to start but fail partway through.

---

## Slide 8.1.3: Build Framework Architecture

**Title**: Build/ Submodule Structure

**Visual**: Framework diagram:

```
scripts/release-build.sh
    |
    +-- Build/lib/common.sh      (logging, container runtime detection)
    +-- Build/lib/version.sh     (semantic versioning via versions.json)
    +-- Build/lib/hash.sh        (SHA256 change detection)
    +-- Build/lib/orchestrator.sh (CLI parsing, build loop)
    |
    +-- scripts/lib/build-api.sh       (Go backend)
    +-- scripts/lib/build-web.sh       (React frontend)
    +-- scripts/lib/build-desktop.sh   (Tauri desktop)
    +-- scripts/lib/build-wizard.sh    (Installer wizard)
    +-- scripts/lib/build-android.sh   (Android mobile)
    +-- scripts/lib/build-androidtv.sh (Android TV)
    +-- scripts/lib/build-api-client.sh (TypeScript client)
```

**Speaker Notes**: The build framework is generic and reusable. The Build/ submodule has no Catalogizer-specific code. Projects define BUILD_COMPONENTS, BUILD_COMPONENT_PATTERNS, and build_single_component(). The SHA256 change detection skips unchanged components, reducing build time significantly.

---

## Slide 8.1.4: Build Commands

**Title**: Building Catalogizer

| Scope | Command | Duration |
|-------|---------|----------|
| Full release | `./scripts/release-build.sh --container --force --skip-tests` | ~17 min |
| Backend only | `podman run --network host ... go build -o catalog-api` | ~2 min |
| Frontend only | `podman run --network host ... npm run build` | ~3 min |
| Build pipeline | `./scripts/container-build.sh` | varies |

**Bullet Points**:
- `--container`: use builder container for reproducible builds
- `--force`: bypass SHA256 change detection, build everything
- `--skip-tests`: skip test execution during build
- Resource limit: builder container max 3 CPUs, 8 GB RAM

**Speaker Notes**: The full release build produces artifacts for all 7 components. In development, omit --force to leverage change detection. The --skip-tests flag is safe when you have already run tests separately.

---

## Slide 8.2.1: Production Stack

**Title**: docker-compose.yml Services

**Visual**: Architecture diagram:

```
                    Nginx (reverse proxy, TLS, HTTP/3)
                         |
              +----------+----------+
              |                     |
        catalog-api           catalog-web
              |                     |
    +---------+---------+           |
    |                   |           |
PostgreSQL 15       Redis 7     (static files)
```

**Bullet Points**:
- PostgreSQL 15 Alpine: persistent data, health checks every 10s, port 5432->5433
- Redis 7 Alpine: caching layer (optional), config/redis.conf
- Nginx: reverse proxy, TLS termination, config/nginx.conf
- catalog-api: Go backend with HTTP/3 and Brotli compression
- catalog-web: static files served via Nginx

**Speaker Notes**: This is the production architecture. PostgreSQL stores all data with a named volume for persistence. Redis accelerates frequent queries. Nginx handles TLS termination and routing. The port mapping (5432 to 5433) avoids conflicts with any host PostgreSQL installation.

---

## Slide 8.2.2: Required Environment Variables

**Title**: Production Configuration

| Variable | Purpose | Example |
|----------|---------|---------|
| `DB_TYPE` | Database engine | `postgres` |
| `DB_HOST` | Database hostname | `localhost` |
| `DB_PORT` | Database port | `5433` |
| `DB_NAME` | Database name | `catalogizer_db` |
| `DB_USER` | Database user | `catalogizer` |
| `DB_PASSWORD` | Database password | (secret) |
| `JWT_SECRET` | Token signing key | (secret, 32+ chars) |
| `GIN_MODE` | Server mode | `release` |
| `ADMIN_PASSWORD` | Initial admin password | (secret) |

**Bullet Points**:
- **Config precedence**: environment variables > .env > config.json > defaults
- Use a secrets manager in production (Vault, AWS Secrets Manager, etc.)
- Never commit secrets to version control
- `config.json write_timeout` must be 900 for challenge RunAll operations

**Speaker Notes**: JWT_SECRET is the most critical secret. If it changes, all existing tokens become invalid and every user must re-authenticate. If it leaks, all tokens are compromised. Use a strong, unique value and back it up securely.

---

## Slide 8.2.3: HTTP/3 and Compression

**Title**: Network Protocol Requirements

**Bullet Points**:
- **Mandatory**: HTTP/3 (QUIC) with Brotli compression in production
- **Fallback**: HTTP/2 with gzip
- **Never**: HTTP/1.1 in production
- Backend: `quic-go/http3` server + `andybalholm/brotli` middleware
- TLS certificates: self-signed, generated at startup (or provided via Nginx)
- Nginx: TLS termination with production certificates

**Speaker Notes**: HTTP/3 uses QUIC (UDP-based transport) for faster connection establishment and better performance over unreliable networks. Brotli provides better compression ratios than gzip, reducing bandwidth. The backend generates self-signed certs for direct access; in production, Nginx handles TLS with real certificates.

---

## Slide 8.2.4: Bare-Metal Deployment

**Title**: systemd Service Configuration

**Bullet Points**:
- Service file: `config/systemd/catalogizer-api.service`
- Automatic restart on failure
- Environment file loading from `.env`
- Resource limits via systemd cgroups
- Process management: `Type=simple`, `Restart=always`

**Visual**: Simplified systemd unit file:

```ini
[Unit]
Description=Catalogizer API
After=network.target postgresql.service

[Service]
Type=simple
EnvironmentFile=/opt/catalogizer/.env
ExecStart=/opt/catalogizer/catalog-api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**Speaker Notes**: For deployments without container infrastructure, systemd provides process management. The service file handles automatic restart, log capture via journald, and resource limits. Install PostgreSQL and Redis as separate systemd services or use managed database services.

---

## Slide 8.3.1: Metrics System

**Title**: Prometheus Metrics in Catalogizer

**Bullet Points**:
- Endpoint: `/metrics` exposes Prometheus-format metrics
- Implementation: `internal/metrics/metrics.go` and `internal/metrics/middleware.go`
- Auto-recorded: HTTP request duration, count by status/method, response size
- Custom metrics: detection pipeline, scanner throughput, WebSocket connections, DB latency
- Scrape config: `monitoring/prometheus.yml`

**Speaker Notes**: The metrics middleware intercepts every HTTP request automatically. Custom metrics are added for application-specific concerns. The /metrics endpoint is unauthenticated by design so Prometheus can scrape it without credentials.

---

## Slide 8.3.2: Monitoring Stack

**Title**: Prometheus + Grafana Deployment

**Visual**: Monitoring architecture diagram:

```
catalog-api (/metrics) <-- Prometheus (scrape every 15s) <-- Grafana (dashboards)
         |                        |                              |
         v                        v                              v
   Application Metrics      Time-Series DB             Visualization + Alerts
```

**Bullet Points**:
- Prometheus: `monitoring/prometheus.yml` with scrape targets
- Grafana: pre-built dashboards in `monitoring/grafana/` and `config/grafana-dashboards/`
- Resource limits: each monitoring container max 1 CPU, 2 GB RAM
- Total container budget: max 4 CPUs, 8 GB RAM across all services

**Speaker Notes**: Deploy both with Podman using --network host and resource limits. The pre-built Grafana dashboards give immediate visibility. Customize them for your specific deployment needs. Remember the total resource budget -- monitoring containers count toward the 4 CPU / 8 GB limit.

---

## Slide 8.3.3: Critical Alerts

**Title**: What to Monitor and Alert On

| Alert | Condition | Severity |
|-------|-----------|----------|
| Service Down | Target health = 0 | Critical |
| High Error Rate | 5xx rate > 1% | High |
| High Latency | p99 > 2s | Medium |
| Disk Space Low | < 10% free | High |
| Memory High | > 80% limit | Medium |
| Circuit Breaker Open | SMB CB state = open | Medium |
| Scan Stalled | No progress > 5 min | Low |

**Bullet Points**:
- SMB circuit breaker metrics: early warning for storage source problems
- Detection pipeline metrics: throughput, per-file duration, provider latency
- Resource monitoring: `podman stats --no-stream` and `cat /proc/loadavg`

**Speaker Notes**: These alerts cover the most important failure modes. Service down is the most critical -- if the API is unreachable, nothing works. The circuit breaker alerts are an early warning system. Track provider latency separately so you know when external services degrade before users notice.

---

## Slide 8.3.4: Log Management

**Title**: Logging Configuration

**Bullet Points**:
- `LOG_LEVEL` environment variable: debug, info, warn, error
- **Debug**: full request/response logging, SQL queries, provider calls
- **Info**: normal operations, scan progress, connection events
- **Warn**: recoverable errors, circuit breaker transitions, retry attempts
- **Error**: failures requiring attention
- Configure log rotation to prevent disk exhaustion
- Consider centralized log aggregation for search and analysis

**Speaker Notes**: In production, start with info level. Switch to debug only for active troubleshooting, and switch back when done. Debug logging generates significant volume and can affect performance. Log rotation is critical -- an unrotated log file can fill a disk surprisingly fast.

---

## Slide 8.4.1: Backup Strategy

**Title**: Three-Tier Backup Plan

| Tier | Frequency | Contents | Method |
|------|-----------|----------|--------|
| Database | Daily | All catalog data, users, collections, metadata | `pg_dump` via podman exec |
| Configuration | Weekly | .env, config.json, nginx, redis, grafana, prometheus | `tar` archive |
| Verification | Monthly | Restore to test environment, run challenges | Full restore test |

**Bullet Points**:
- PostgreSQL: `podman exec catalogizer-postgres pg_dump -U catalogizer catalogizer_db > backup.sql`
- SQLite: file copy is safe with WAL mode (explicit PRAGMA in database/connection.go)
- Never delete `catalogizer-pgdata` volume without a verified backup
- Store backups off-site -- at least one copy on separate physical storage

**Speaker Notes**: The three-tier strategy balances effort with safety. Daily database backups capture the data that changes most. Weekly config backups capture the settings that change rarely. Monthly verification catches corruption and procedural issues before they become emergencies.

---

## Slide 8.4.2: Disaster Recovery

**Title**: Recovery Scenarios

| Scenario | Recovery Procedure | Data Loss |
|----------|--------------------|-----------|
| Database corruption | Restore from latest `pg_dump`; migrations auto-apply | Since last backup |
| Configuration loss | Restore from weekly config archive; JWT_SECRET critical | Sessions invalidated |
| Media source offline | Circuit breaker auto-opens; offline cache serves data; auto-recovers | None (cached data) |
| Container crash | Restart container; named volume preserves DB | None |
| Complete host failure | New host + backup restore + config restore | Since last backup |

**Speaker Notes**: The most critical item to protect is JWT_SECRET. If lost, every user must re-authenticate. If the database is lost but configuration is preserved, you can rescan media sources to rebuild -- but user data (favorites, collections, playlists) would be lost. The circuit breaker provides automatic recovery for the most common failure mode: network storage going offline temporarily.

---

## Slide 8.4.3: Database Migrations

**Title**: Versioned Schema Management

**Bullet Points**:
- Migration files in `database/migrations/`
- Separate SQLite and PostgreSQL variants per version
- 9 migration versions: v1 (base schema) through v9 (performance indexes)
- Auto-applied on application startup
- Dialect abstraction rewrites SQL:
  - `?` placeholders -> `$1, $2, ...` for PostgreSQL
  - `INSERT OR IGNORE` -> `ON CONFLICT DO NOTHING`
  - Boolean literals `= 0/1` -> `= FALSE/TRUE`

**Speaker Notes**: Migrations run automatically when the application starts. If you restore a database from an older backup, the migration system detects the version difference and applies any missing migrations. The dialect abstraction means most SQL is written once and works on both SQLite and PostgreSQL.

---

## Slide 8.4.4: Upgrade Procedure

**Title**: Zero-Downtime Upgrades

**Bullet Points**:
1. Build new version: `./scripts/release-build.sh --container --force --skip-tests`
2. Run tests against new build: `scripts/run-all-tests.sh`
3. Back up current database: `pg_dump` before upgrading
4. Stop old container: `podman stop catalogizer-api`
5. Start new container: `podman start catalogizer-api` (or recreate with new image)
6. Migrations apply automatically on startup
7. Verify health: `curl /api/v1/health` and `podman stats --no-stream`
8. For multi-instance: upgrade one at a time behind Nginx load balancer

**Speaker Notes**: Always back up before upgrading. The migration system handles schema changes, but having a restore point is essential. For true zero-downtime, run multiple API instances behind Nginx and upgrade them one at a time. Verify each instance is healthy before upgrading the next.

---

## Slide 8.4.5: Capacity Planning

**Title**: Growth and Scaling

**Bullet Points**:
- Monitor database size: `SELECT pg_database_size('catalogizer_db');`
- Monitor table row counts: media_items, media_files, files tables
- Track storage usage trends via analytics dashboard
- Track request rates via Prometheus metrics
- **Resource budget**: max 4 CPUs, 8 GB RAM across all containers
- Plan storage expansion before you reach 80% capacity
- 11 media types across media_items table -- monitor query performance as catalog grows

**Speaker Notes**: Capacity planning is proactive, not reactive. If your catalog is growing at 1 GB per month, plan for 12 GB of additional storage per year plus overhead. Monitor query performance -- as the media_items table grows, queries may need optimization. The analytics dashboard provides the data you need for planning.

---

## Slide 8.4.6: Module 8 Summary

**Title**: What We Covered

**Bullet Points**:
- Container builds with Podman: builder image, critical requirements, release pipeline
- Production configuration: PostgreSQL, Redis, Nginx, HTTP/3, environment variables
- Monitoring: Prometheus metrics, Grafana dashboards, critical alerts, log management
- Maintenance: three-tier backup, disaster recovery, migrations, upgrades, capacity planning
- Resource limits: max 4 CPUs, 8 GB RAM across all containers (30-40% host budget)

**Course Conclusion**: Students completing all 8 modules, including these two advanced modules, are certified as **Catalogizer Expert**. This certification demonstrates mastery of usage, administration, development, testing, and production operations.

**Speaker Notes**: This concludes the complete Catalogizer course. The eight modules cover the full lifecycle from installation to production operations. The certification path -- User, Administrator, Developer, Expert -- recognizes increasing depth of knowledge. Refer to the assessment and exercises documents for certification requirements at each level.
