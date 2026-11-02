# Catalogizer -- Admin User Guide

## Table of Contents

1. [Accessing the Admin Panel](#accessing-the-admin-panel)
2. [System Information Overview](#system-information-overview)
3. [User Management](#user-management)
4. [Storage Configuration and Scanning](#storage-configuration-and-scanning)
5. [Backup and Restore](#backup-and-restore)
6. [Log Collection and Analysis](#log-collection-and-analysis)
7. [Health Monitoring](#health-monitoring)
8. [Cache Management](#cache-management)
9. [Challenge System](#challenge-system)
10. [Troubleshooting](#troubleshooting)

---

## Accessing the Admin Panel

The admin panel is available exclusively to users with the **admin** role. Access it from the web application at `/admin` or via the gear icon in the top navigation bar (visible only to admin users).

### Default Admin Account

On first startup, Catalogizer creates a default admin account:

- **Username**: `admin`
- **Password**: Set via the `ADMIN_PASSWORD` environment variable (defaults to `<ADMIN_PASSWORD>` in development)

Change the default password immediately after first login. Navigate to the admin panel, select your user account, and update the password.

### Authentication

Admin panel access requires a valid JWT token with the admin role claim. The token is issued during login and stored in the browser. If your session expires, you will be redirected to the login page. The JWT secret is configured via the `JWT_SECRET` environment variable.

### Role-Based Access Control

Catalogizer uses two roles:

| Role | Capabilities |
|------|-------------|
| `user` | Browse entities, search, manage personal favorites, collections, playlists, view media, download subtitles |
| `admin` | All user capabilities plus: user management, storage configuration, scanning, backup/restore, log access, system configuration, cache management, challenge execution |

---

## System Information Overview

The admin dashboard (landing page of the admin panel) displays a real-time overview of the system:

### System Statistics

- **Total entities**: Count of all media items in the database, broken down by type
- **Total files**: Count of all scanned files across all storage roots
- **Total storage roots**: Number of configured storage sources
- **Database size**: Current size of the SQLite database file or PostgreSQL database
- **Uptime**: Time since the catalog-api process started

### Service Status

A status panel shows the health of connected services:

- **Database**: Connection status and dialect (SQLite or PostgreSQL)
- **Redis**: Connection status (if configured for caching)
- **WebSocket**: Active connection count
- **Metadata providers**: Availability of TMDB, OMDB, OpenLibrary, MusicBrainz

### Recent Activity

A feed of recent system events:

- Storage root scans (start, completion, errors)
- User logins and account changes
- Entity aggregation results
- Metadata enrichment completions
- System errors and warnings

---

## User Management

### Listing Users

Navigate to **Admin > Users** to see all registered accounts. The user list shows:

- Username
- Role (user or admin)
- Created date
- Last login date
- Account status (active or disabled)

### Creating a New User

1. Click **Create User** in the users panel.
2. Enter a username (must be unique, alphanumeric with underscores, 3-50 characters).
3. Enter a password (minimum 8 characters).
4. Select a role: `user` or `admin`.
5. Click **Save**.

The new account is active immediately. Share the credentials with the user and advise them to change their password on first login.

### Updating a User

Click a user row in the list to open the edit form. You can update:

- **Password**: Enter a new password. Leave blank to keep the current password.
- **Role**: Change between `user` and `admin`. Role changes take effect on the user's next request (existing JWT tokens carry the old role until they expire).

Username changes are not supported. To change a username, delete the account and create a new one.

### Deleting a User

Click the delete icon on a user row. A confirmation dialog appears. Deleting a user:

- Removes the account and invalidates all active sessions
- Preserves the user's metadata contributions (ratings, tags, notes) in the database
- Does not affect media files, entities, or collections

The default admin account cannot be deleted.

---

## Storage Configuration and Scanning

### Adding a Storage Root

Storage roots define where Catalogizer looks for media files. Navigate to **Admin > Storage** and click **Add Storage Root**.

Configure the following fields:

| Field | Description | Example |
|-------|-------------|---------|
| **Name** | A human-readable label | "NAS Movies" |
| **Protocol** | Connection protocol | SMB, FTP, NFS, WebDAV, Local |
| **Path** | Root path to scan | `/volume1/media/movies` |
| **Host** | Server hostname or IP (network protocols only) | `synology.local` |
| **Port** | Server port (network protocols only) | `445` (SMB default) |
| **Username** | Authentication username (if required) | `media_user` |
| **Password** | Authentication password (if required) | (stored encrypted) |
| **Enabled** | Whether this root is included in scans | `true` |

Click **Save** to add the storage root. The root is not scanned automatically on creation -- you must trigger a scan manually or wait for the next scheduled scan.

### Triggering a Scan

From the storage management page, click **Scan** on a storage root to start scanning. You can also scan all roots simultaneously by clicking **Scan All**.

During a scan, the interface shows:

- Files discovered count (incrementing in real-time via WebSocket)
- Current file being processed
- Scan duration
- Error count (files that could not be read or parsed)

Scans are rate-limited by the semaphore-based concurrency control. The default limit is 8 concurrent file operations per storage root (configurable in `config.json` under `concurrency.file_scan`).

### Scan Behavior

- **Incremental scanning**: Only new or modified files are processed. Previously scanned files are skipped unless their modification timestamp has changed.
- **Post-scan aggregation**: After the scan completes, the aggregation pipeline runs automatically to create or update entities, build hierarchies, and detect duplicates.
- **Real-time updates**: Scan progress is broadcast to all connected clients via WebSocket. The entity browser updates in real-time as new entities are created.

### Removing a Storage Root

Click the delete icon on a storage root. A confirmation dialog explains the consequences:

- The storage root configuration is removed
- Files from this root are removed from the database
- Entities that were only associated with files from this root are removed
- Entities that have files from other roots remain unaffected

---

## Backup and Restore

### Creating a Backup

Navigate to **Admin > Backup** and click **Create Backup**. The system creates a snapshot containing:

- Complete database dump (all tables: entities, files, users, metadata, collections, preferences)
- Configuration file (`config.json`)
- Storage root definitions

The backup is saved as a timestamped archive in the `backups/` directory. The file name follows the pattern `catalogizer-backup-YYYY-MM-DD-HHMMSS.tar.gz`.

Backups do not include media files themselves -- only the database and configuration. Media files remain on their original storage roots.

### Restoring from Backup

1. Navigate to **Admin > Backup**.
2. Select a backup file from the list or upload one.
3. Click **Restore**.
4. Confirm the restoration in the dialog. This operation replaces the current database.

After restoration, the application restarts automatically. All users are logged out and must re-authenticate.

### Backup Best Practices

- Create a backup before major operations (bulk scans, version upgrades, storage root changes).
- Store backup archives on a separate physical drive from the database.
- Test restore periodically to verify backup integrity.
- For PostgreSQL deployments, consider using `pg_dump` for additional backup redundancy.

---

## Log Collection and Analysis

### Viewing Logs

Navigate to **Admin > Logs** to access the log viewer. The log viewer displays application logs in reverse chronological order with the following columns:

- **Timestamp** -- ISO 8601 format with millisecond precision
- **Level** -- DEBUG, INFO, WARN, ERROR
- **Component** -- Source module (scanner, aggregation, auth, api, websocket, etc.)
- **Message** -- Log message text

### Filtering Logs

Use the filter controls above the log table:

- **Level filter**: Show only logs at or above a selected severity (e.g., WARN and ERROR only)
- **Component filter**: Show logs from a specific component
- **Time range**: Restrict to a time window (last hour, last 24 hours, custom range)
- **Search**: Free-text search within log messages

### Log Collection

The **Collect Logs** button triggers the `LogManagementService` to gather logs from all system components into a consolidated archive. This is useful for sharing with support or for offline analysis. The collected archive includes:

- Application logs (catalog-api)
- Scan logs (per storage root)
- Error traces with stack traces
- Aggregation pipeline logs

### Log Rotation

Application logs are rotated automatically. The default configuration keeps:

- Last 7 days of logs in the active log file
- Compressed archives of older logs (retained for 30 days)
- Maximum log file size of 100MB before rotation

Log rotation settings are configurable in `config.json` under the `logging` section.

---

## Health Monitoring

### Health Endpoint

The API exposes a health check endpoint at `/api/v1/health` that returns the status of all system components. The admin panel displays this information in the **System Health** widget on the dashboard.

### Prometheus Metrics

Catalogizer exposes Prometheus-compatible metrics at the `/metrics` endpoint. Available metrics include:

- **HTTP request duration** (histogram, by route and status code)
- **Active WebSocket connections** (gauge)
- **Database query duration** (histogram)
- **Scan files processed** (counter, by storage root)
- **Metadata provider requests** (counter, by provider and status)
- **Cache hit/miss ratio** (counter)
- **Semaphore utilization** (gauge, by subsystem)

### Resource Monitoring

The admin dashboard includes a resource monitoring panel showing:

- **CPU usage**: Current process CPU utilization
- **Memory usage**: Heap allocation and system memory
- **Goroutine count**: Number of active goroutines
- **Database connections**: Active and idle connection pool counts
- **Open file descriptors**: Current count vs. system limit

These metrics help ensure the application stays within the mandatory 30-40% host resource limit.

---

## Cache Management

If Redis is configured, the admin panel provides cache management controls under **Admin > Cache**:

- **View cache statistics**: Hit rate, miss rate, total keys, memory usage
- **Clear all caches**: Flush all cached data (metadata, search results, thumbnails)
- **Clear by prefix**: Flush cache entries matching a specific prefix (e.g., `metadata:tmdb:` to clear only TMDB metadata cache)
- **Cache TTL configuration**: View and adjust time-to-live settings for different cache categories

Cache clearing does not delete any permanent data. Cleared entries are re-populated on the next request.

---

## Challenge System

Catalogizer includes an integrated challenge system for end-to-end validation. Challenges are accessible under **Admin > Challenges**.

### Running Challenges

- **Run All**: Executes all registered challenges sequentially. This is a blocking operation -- no other challenges can run until it completes.
- **Run Single**: Execute a specific challenge by ID (e.g., CH-001).
- **View Results**: See pass/fail status, duration, and evidence for each challenge.

### Important Constraints

- Challenge execution is synchronous and blocking. Only one challenge (or RunAll sequence) can execute at a time.
- The liveness detector kills challenges that show no progress for 5 minutes.
- Challenges must be executed by system deliverables (the running catalog-api service), never by external scripts or curl commands.
- Challenge results are stored in `challenges/` and are viewable from the admin panel.

---

## Troubleshooting

### Cannot Access Admin Panel

Verify your account has the admin role. Check with another admin user or inspect the database directly. If locked out, restart the application -- the default admin account is recreated if no admin accounts exist.

### Scan Fails or Stalls

- Check the storage root connectivity (can the server reach the host?).
- Review scan logs in **Admin > Logs** filtered to the scanner component.
- Verify credentials for network storage protocols (SMB, FTP, NFS, WebDAV).
- Check the semaphore concurrency limits -- very low limits on slow storage can cause apparent stalls.
- For SMB connections, the circuit breaker may have tripped due to repeated failures. It resets automatically after the backoff period.

### High Resource Usage

- Check `podman stats --no-stream` for container resource consumption.
- Review the concurrency configuration in `config.json` and reduce limits if needed.
- Ensure the database connection pool settings are appropriate for your deployment (MaxOpen=25 may be too high for constrained environments).

### Database Issues

- **SQLite lock errors**: Ensure WAL mode is active (`PRAGMA journal_mode` should return `wal`). Only one process should access the SQLite database file.
- **PostgreSQL connection refused**: Verify `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD` environment variables. Check that the PostgreSQL container is running and healthy.

### WebSocket Disconnections

WebSocket connections may drop due to reverse proxy timeouts. Ensure your nginx or reverse proxy configuration allows long-lived WebSocket connections. The client reconnects automatically with exponential backoff.
