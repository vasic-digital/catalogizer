# Frequently Asked Questions

---

## General

### What is Catalogizer?

Catalogizer is a multi-protocol media collection management system. It connects to your media stored across SMB/CIFS, FTP/FTPS, NFS, WebDAV, and local filesystems, automatically detects and categorizes over 50 media types, and enriches items with metadata from external providers like TMDB, IMDB, TVDB, MusicBrainz, Spotify, and Steam. You access your unified catalog from a web browser, desktop app, Android phone, or Android TV.

### Is Catalogizer free?

Catalogizer is a self-hosted application. You run it on your own hardware or server. There are no subscription fees, cloud dependencies, or usage limits.

### What platforms does Catalogizer run on?

The backend (catalog-api) runs on any platform that supports Go: Linux, macOS, and Windows. The web frontend runs in any modern browser (Chrome 90+, Firefox 88+, Safari 14+, Edge 90+). Native apps are available for Windows, macOS, Linux (desktop), Android 8.0+ (phone), and Android TV.

### Does Catalogizer store my media files?

No. Catalogizer stores only metadata about your media files (titles, descriptions, quality information, organizational data). Your actual media files remain on their original storage locations. Catalogizer reads from those locations but does not copy or move files unless you explicitly use conversion or copy features.

---

## Installation

### What are the minimum system requirements?

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Storage | 20 GB | 50 GB+ |
| Network | 100 Mbps | 1 Gbps |

### Should I use Docker/Podman or manual installation?

Container installation (Docker or Podman) is recommended for most users. It handles all dependencies automatically and provides consistent behavior. Manual installation is better for development or when you need fine-grained control over each component. If Docker is not available, Podman is fully supported as an alternative container runtime.

### Do I need PostgreSQL?

For development and small deployments, SQLite is the default database and requires no additional setup. For production deployments with multiple concurrent users, PostgreSQL 15+ is recommended. The Docker Compose configuration includes PostgreSQL automatically.

### Can I run Catalogizer behind a reverse proxy?

Yes. The project includes Nginx configuration files in `config/nginx.conf` and `config/nginx/catalogizer.prod.conf`. These handle TLS termination, request routing, and static file serving. You can adapt the configuration for other reverse proxies like Caddy or Traefik.

### How do I update Catalogizer?

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

Pull the latest code, rebuild the components, and restart. For container deployments: pull the latest images and run `podman compose up -d`. Database migrations run automatically on startup. Always back up your database before updating.

---

## Storage and Protocols

### Which storage protocols are supported?

Catalogizer supports five protocols through its UnifiedClient interface:
- **SMB/CIFS**: Windows and Samba file shares
- **FTP/FTPS**: Standard and secure FTP servers
- **NFS**: Network File System (Linux and macOS)
- **WebDAV**: HTTP-based file access
- **Local Filesystem**: Directly attached storage

### Can I add multiple storage sources?

Yes. You can connect as many storage sources as you need, using any combination of protocols. All sources are unified into a single catalog with a consistent browsing and search experience.

### What happens when a network storage source goes offline?

Catalogizer uses a resilience system for network storage, particularly SMB. A circuit breaker prevents repeated connection attempts to a downed server. An offline cache serves previously loaded metadata so users can continue browsing. Exponential backoff retry gradually attempts reconnection. When the source comes back online, the system automatically recovers and resumes normal operation.

### Can Catalogizer discover shares on my network?

Yes. The SMB discovery feature can auto-detect available SMB shares on your local network. This is useful for finding shares you did not know existed or for simplifying initial setup.

### Does Catalogizer modify files on my storage sources?

Catalogizer operates in read-only mode by default. It reads files to detect types and extract metadata but does not modify, move, or delete files on storage sources unless you explicitly initiate a file operation (such as copy, rename, or conversion).

---

## Media Detection and Metadata

### How many media types does Catalogizer detect?

Catalogizer detects over 50 media types, including movies, TV shows, music albums, individual tracks, games, software, documentaries, audiobooks, podcasts, ebooks, and more. The detection pipeline uses filename analysis, path structure, and file extension matching.

### Where does the metadata come from?

Metadata is fetched from external providers:
- **TMDB** (The Movie Database): Movies and TV shows
- **IMDB**: Movies and TV shows
- **TVDB**: TV series information
- **MusicBrainz**: Music albums and tracks
- **Spotify**: Music metadata and album art
- **Steam**: Games and software

You need API keys for some providers (TMDB is free). Metadata is cached locally to minimize external API calls.

### What if metadata is wrong for a file?

The detection pipeline uses heuristics and may occasionally misidentify a file. You can manually correct metadata through the web interface. Corrected metadata is stored locally and takes precedence over auto-detected information.

### Does Catalogizer detect video quality?

Yes. The analyzer component of the detection pipeline extracts technical metadata including resolution (720p, 1080p, 4K/UHD), codec (H.264, H.265, VP9), bitrate, and container format. Quality profiles allow comparison and ranking across versions of the same content.

---

## Media Player and Playback

### Can I play media directly in Catalogizer?

Yes. Catalogizer includes a built-in media player that handles video and audio playback in the browser. The player streams content from storage sources regardless of protocol. It supports playback controls, subtitle selection, and fullscreen mode.

### Does playback position sync across devices?

Yes. The playback position service saves your position whenever you pause or close the player. When you return to the same item from any device (web, desktop, or Android), playback resumes from where you left off.

### What subtitle formats are supported?

Catalogizer supports SRT, ASS, SSA, and VTT subtitle formats. Subtitles are automatically matched with videos based on naming conventions. You can also manually associate subtitle files and upload new ones through the Subtitle Manager page.

### Can I share a specific moment in a video?

Yes. The deep linking service generates links that include both the media item and an exact playback position. Recipients who open the link jump directly to the specified moment.

---

## Organization

### What is the difference between collections and playlists?

Collections organize media thematically. They can be Manual (hand-picked items), Smart (auto-populated based on rules), or Dynamic (adaptive criteria). Playlists are ordered sequences designed for sequential playback, with auto-advancement when one item finishes.

### How do Smart collections work?

You define rules such as "all movies from 2024" or "all 4K videos" when creating a Smart collection. Catalogizer automatically adds matching media to the collection. When new media matching the rules is detected in the future, it is added automatically without any manual intervention.

### Can I export my favorites?

Yes. Favorites can be exported to JSON or CSV format with full metadata. You can import favorites from these files on another Catalogizer instance. Matching uses metadata rather than file paths, so imports work even when files are on different storage sources.

---

## Security

### How is authentication handled?

Catalogizer uses JWT (JSON Web Token) authentication. Users log in with credentials and receive access and refresh tokens. The access token is included with every API request and validated by the auth middleware. Tokens have configurable expiry times.

### Is the database encrypted?

Yes. SQLCipher provides AES-256 encryption for the SQLite database at rest. The encryption key is configured via the DB_ENCRYPTION_KEY environment variable (exactly 32 characters). Without this key, the database file is unreadable.

### Does Catalogizer support two-factor authentication?

Yes. Administrators can enable 2FA for user accounts. Users scan a QR code with an authenticator app and provide a verification code during login in addition to their password.

### How do I run security scans?

Catalogizer includes scripts for automated security testing:
- `scripts/security-test.sh` for security-focused tests
- `scripts/snyk-scan.sh` for dependency vulnerability scanning
- `scripts/sonarqube-scan.sh` for static code analysis

---

## Multi-Platform

### Do mobile apps work offline?

Yes. The Android app uses Room database for local caching. Previously loaded metadata is available offline. When connectivity returns, the app syncs with the server automatically.

### Can I build the desktop app for my platform?

Yes. The desktop app uses Tauri and produces native installers:
- Windows: MSI installer
- macOS: DMG image
- Linux: AppImage or .deb package

Build with `npm run tauri:build` in the `catalogizer-desktop/` directory.

### What is the API client library for?

The TypeScript API client library (`catalogizer-api-client`) provides programmatic access to the Catalogizer API. Use it to build custom dashboards, automation scripts, batch processing tools, or entirely new client applications.

---

## Administration

### How do I back up Catalogizer?

Back up three things:
1. The SQLCipher database file (most critical -- contains all metadata and user data)
2. The `.env` configuration files (server settings and API keys)
3. The `config/` directory (nginx.conf, redis.conf)

Store the DB_ENCRYPTION_KEY separately from the database backup for security. Test your restore procedure monthly.

### How do I monitor Catalogizer in production?

Catalogizer exposes Prometheus-compatible metrics. The project includes a pre-configured Prometheus configuration and Grafana dashboards in the `monitoring/` directory. Dashboards cover API performance, media detection throughput, and storage source health.

### How do I troubleshoot connection issues?

Set `LOG_LEVEL=debug` in your `.env` file and restart the backend. Check the logs for circuit breaker state transitions, retry attempts, and error details. The Troubleshooting Guide in the documentation provides step-by-step diagnosis procedures for common issues.

---

## Security Scanning

### What security scanning tools does Catalogizer use?

Catalogizer integrates six security scanning tools for defense in depth:

1. **govulncheck**: Go's official vulnerability scanner. Unlike generic dependency scanners, it performs call graph analysis and only reports vulnerabilities in functions your code actually calls.
2. **Semgrep**: Static analysis that matches code patterns for SQL injection, XSS, path traversal, hardcoded secrets, and insecure cryptography.
3. **SonarQube**: Deep static analysis with security hotspot detection, code quality metrics, and technical debt estimation.
4. **Snyk**: Scans both dependencies and container images for known vulnerabilities.
5. **Trivy**: Scans container images for OS package and application dependency vulnerabilities.
6. **npm audit**: Scans frontend Node.js dependencies for known vulnerabilities.

Run all scans via `scripts/security-test.sh` or individually via the tool-specific scripts in `scripts/`.

### How does Catalogizer prevent SQL injection?

All database queries use parameterized placeholders (`?`). The dual-dialect abstraction layer in `database/dialect.go` automatically rewrites `?` to `$1, $2, ...` for PostgreSQL. No raw string concatenation is used in SQL construction. The input validation middleware sanitizes all incoming request data, and Semgrep rules verify that no new code introduces concatenation-based queries.

### How often should I run security scans?

Run `govulncheck` and `npm audit` before every deployment. Run Semgrep and SonarQube weekly or after significant code changes. Run Trivy after building new container images. Snyk can be configured for continuous monitoring. The release build pipeline includes security checks as a mandatory step.

---

## Load Testing

### How do I load test Catalogizer?

Catalogizer includes k6 load test scripts in `tests/k6/`. Install k6, then run:

```bash
k6 run tests/k6/load-test.js
```

The load test authenticates via JWT, exercises key API endpoints (storage roots, media search, entities, statistics), and reports latency, error rate, and throughput metrics.

### What load test scenarios are available?

Four scenarios are provided:

- **Load test**: Gradual ramp to normal capacity (10-20 virtual users over 16 minutes). Validates production-level performance.
- **Stress test**: Pushes beyond capacity (up to 150 virtual users) to find the breaking point and verify recovery.
- **Soak test**: Sustained moderate load (20 virtual users for 4+ hours) to detect memory leaks and resource exhaustion.
- **Spike test**: Sudden load increase to test how the system handles traffic bursts.

### What are the performance targets?

| Metric | Target |
|--------|--------|
| p95 latency | < 500ms |
| p99 latency | < 1 second |
| Error rate | < 1% |
| Throughput | > 100 requests/second |

The `ConcurrencyLimiter(100)` middleware protects the backend from overload by capping in-flight requests. The connection pool (MaxOpen=25, MaxIdle=10) prevents database saturation.

---

## Monitoring

### How do I monitor Catalogizer in production?

Catalogizer exposes Prometheus-compatible metrics at `/metrics`. The recommended monitoring stack is:

1. **Prometheus**: Scrapes metrics every 10-15 seconds
2. **Grafana**: Visualizes metrics with pre-built dashboards
3. **Alertmanager**: Routes alerts to email, Slack, or webhooks

Start the monitoring stack with `podman-compose -f docker-compose.dev.yml up prometheus grafana`. Import the dashboard from `monitoring/grafana/dashboards/`.

### What metrics are available?

Three categories of metrics are exported:

- **HTTP metrics**: Request count, duration histogram, request/response sizes -- per method and path
- **Go runtime metrics**: Goroutine count, heap allocation, GC pause duration, thread count -- sampled every 15 seconds
- **Application metrics**: Scan operations, files processed, media entity counts, WebSocket connections

### What alerts should I configure?

The recommended alert rules (included in `monitoring/alerts.yml`):

| Alert | Condition | Severity |
|-------|-----------|----------|
| HighErrorRate | Server error rate > 5% for 5 min | Critical |
| HighLatency | p95 latency > 1s for 5 min | Warning |
| GoroutineLeak | Goroutine count > 500 for 10 min | Warning |
| HighMemoryUsage | Heap > 1 GB for 15 min | Warning |
| ServiceDown | Health check failing for 1 min | Critical |

### How do I detect goroutine leaks?

Monitor the `go_goroutines` Prometheus metric. Under stable load, this value should remain roughly constant. A steadily increasing count that does not decrease after load drops indicates a goroutine leak. Normal values for Catalogizer under moderate load are 50-200 goroutines. If the count exceeds 500, investigate with `go tool pprof http://localhost:8080/debug/pprof/goroutine`.

---

## Performance Tuning

### How do I tune database performance?

For SQLite (development):
- WAL mode is enabled automatically (`PRAGMA journal_mode=WAL`)
- Busy timeout is set to 30 seconds to handle write contention
- Cache size is configurable via `config.json`

For PostgreSQL (production):
- Connection pool: MaxOpen=25, MaxIdle=10, ConnMaxLifetime=5min (configurable)
- Run `VACUUM ANALYZE` periodically to update statistics
- Monitor slow queries via `pg_stat_statements`
- The performance indexes added in migration v9 target the most common query patterns

### How do I tune the API for high throughput?

Key configuration settings:

| Setting | Default | Description |
|---------|---------|-------------|
| `ConcurrencyLimiter` | 100 | Maximum in-flight HTTP requests |
| `RequestTimeout` | 60s | Maximum request processing time |
| `write_timeout` | 900s | Must be 900 (not 30) for RunAll challenges |
| `MaxOpenConnections` | 25 | Database connection pool size |
| `ScannerConcurrency` | 4 | Concurrent scan workers per storage root |
| `AssetWorkers` | 4 | Concurrent asset resolution workers |

For containerized deployments, enforce resource limits: API container at `--cpus=2 --memory=4g`, PostgreSQL at `--cpus=1 --memory=2g`. Total container budget: max 4 CPUs, 8 GB RAM.

### How do I optimize media entity queries?

Migration v9 adds performance indexes tailored to the most common query patterns:

- `idx_media_items_title_type` (compound): Accelerates `GetByTitle` and `GetDuplicates` which always filter by title and type together
- `idx_media_items_status` and `idx_media_items_year`: Accelerates search and duplicate detection filters
- `idx_user_metadata_user_watched` (compound): Accelerates watched media queries
- `idx_media_files_item_file` (unique): Prevents duplicate file-entity links and accelerates junction lookups

---

## Database Migration

### How do I migrate from SQLite to PostgreSQL?

The [Migration Guide](../MIGRATION_GUIDE.md) provides detailed steps:

1. Create the PostgreSQL database and user
2. Set `DATABASE_TYPE=postgres` and connection parameters in `.env`
3. Start Catalogizer to create the schema (migrations run automatically)
4. Export data from SQLite using `sqlite3 -csv`
5. Import into PostgreSQL using `\COPY` with CSV
6. Reset PostgreSQL sequences to match imported data
7. Verify row counts and run the challenge suite

The dialect abstraction layer handles SQL differences (placeholders, boolean literals, upsert syntax) transparently, so application code works unchanged with either database.

### Do I need to migrate my database when upgrading?

No manual migration is needed. Catalogizer runs migrations automatically on startup. The `migrations` table tracks which versions have been applied. When you update the binary, new migrations run the next time the service starts. Always back up your database before upgrading as a precaution.

### Can I downgrade after upgrading?

There is no built-in rollback mechanism. If you need to downgrade, restore the database from a pre-upgrade backup. This is why backing up before every upgrade is important. See the [Disaster Recovery Guide](../DISASTER_RECOVERY.md) for backup and restore procedures.

---

## Development

### How do I contribute?

See the Contributing Guide (`docs/CONTRIBUTING.md`) for detailed instructions. The basic workflow is: set up the development environment, make changes following existing code patterns, write tests, and submit a pull request. Run `scripts/run-all-tests.sh` before submitting.

### How do I add support for a new storage protocol?

Implement the UnifiedClient interface defined in `filesystem/interface.go`. Create your client file (e.g., `filesystem/s3_client.go`), update the factory in `filesystem/factory.go` to recognize your protocol, and write tests. No other application code needs to change.

### How do I add a new metadata provider?

Study the provider interface in `internal/media/providers/providers.go` and use existing providers like `movie_recognition_provider.go` as a reference. Create your provider, implement the interface, and register it in the provider system. Handle API authentication, rate limiting, and error cases.
