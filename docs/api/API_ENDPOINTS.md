# API Endpoint Catalog

> Renamed from `CHANGELOG.md`; despite that old name this is an endpoint catalog, not a changelog (the "Version History" at the end is the only history it keeps). This is a condensed method/path catalog (172 table rows at the time of writing) and may lag the router; the authoritative sources are [`openapi.yaml`](openapi.yaml) and [`API_DOCUMENTATION.md`](API_DOCUMENTATION.md) (247 operations, kept in sync with `catalog-api/main.go` by `scripts/check_api_route_vs_spec.py`).

This document catalogs REST API endpoints exposed by the Catalogizer backend (`catalog-api`). Endpoints are grouped by domain and listed with their HTTP method, path, and description. All authenticated endpoints require a valid JWT token in the `Authorization: Bearer <token>` header.

---

## Table of Contents

1. [Infrastructure](#infrastructure)
2. [Authentication](#authentication)
3. [Catalog Browsing](#catalog-browsing)
4. [Search](#search)
5. [Download](#download)
6. [File Operations](#file-operations)
7. [Media](#media)
8. [Recommendations](#recommendations)
9. [Subtitles](#subtitles)
10. [Storage](#storage)
11. [Statistics](#statistics)
12. [SMB Discovery](#smb-discovery)
13. [Scans](#scans)
14. [Conversion](#conversion)
15. [User Management](#user-management)
16. [Role Management](#role-management)
17. [Configuration](#configuration)
18. [Error Reporting](#error-reporting)
19. [Log Management](#log-management)
20. [Collections](#collections)
21. [Assets](#assets)
22. [Media Entities](#media-entities)
23. [Analytics](#analytics)
24. [Reporting](#reporting)
25. [Favorites](#favorites)
26. [Browse](#browse)
27. [Sync](#sync)
28. [Challenges](#challenges)

---

## Infrastructure

These endpoints do not require authentication.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check with version, build number, and build date |
| GET | `/metrics` | Prometheus metrics endpoint (via promhttp) |
| GET | `/ws` | WebSocket connection for real-time updates (auth via query parameter) |

---

## Authentication

Rate limited per client IP: login and register 30 requests per minute, other session endpoints 600 per minute (5 per minute only when the optional Redis limiter is on, see `API_DOCUMENTATION.md`). No JWT required for login/register/refresh/logout.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/login` | No | Authenticate with username and password; returns JWT tokens |
| POST | `/api/v1/auth/register` | No | Self-register (pending admin approval by default; see `REGISTRATION_MODE`) |
| POST | `/api/v1/auth/refresh` | No | Refresh an expired access token using a refresh token |
| POST | `/api/v1/auth/logout` | No | Invalidate the current session |
| GET | `/api/v1/auth/me` | Yes | Get the current authenticated user profile |
| GET | `/api/v1/auth/status` | No | Check authentication system status |
| GET | `/api/v1/auth/permissions` | Yes | Get permissions for the current user |
| GET | `/api/v1/auth/profile` | Yes | Alias for `/auth/me` |

---

## Catalog Browsing

All endpoints below require JWT authentication and are rate limited (2000 requests per minute per user; login/register 30 per minute; see `API_DOCUMENTATION.md`).

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/catalog` | List root-level catalog entries |
| GET | `/api/v1/catalog/*path` | List catalog entries at the specified path |
| GET | `/api/v1/catalog-info/*path` | Get detailed file information for a path |

---

## Search

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/search` | Search catalog entries by query string |
| GET | `/api/v1/search/duplicates` | Find duplicate files in the catalog |
| GET | `/api/v1/search/files` | Search files with filters (extension, type, size, date) |
| GET | `/api/v1/search/files/duplicates` | Find file-level duplicates |
| POST | `/api/v1/search/advanced` | Advanced search with JSON body (complex filters, sorting) |

---

## Download

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/download/file/:id` | Download a single file by ID |
| GET | `/api/v1/download/directory/*path` | Download a directory as an archive |
| POST | `/api/v1/download/archive` | Create and download an archive of selected files |

---

## File Operations

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/copy/storage` | Copy a file to another storage location |
| POST | `/api/v1/copy/local` | Copy a file to local filesystem |
| POST | `/api/v1/copy/upload` | Upload a file from local filesystem to storage |

---

## Media

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/media/search` | Search media items with filters and pagination |
| GET | `/api/v1/media/stats` | Get aggregate media statistics |
| GET | `/api/v1/media/:id` | Get a media item by ID (Android TV compatible) |
| PUT | `/api/v1/media/:id/progress` | Update playback watch progress for a media item |
| PUT | `/api/v1/media/:id/favorite` | Toggle favorite status for a media item |

---

## Recommendations

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/recommendations/similar/:media_id` | Get items similar to the specified media |
| GET | `/api/v1/recommendations/trending` | Get currently trending media items |
| GET | `/api/v1/recommendations/personalized/:user_id` | Get personalized recommendations for a user |

---

## Subtitles

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/subtitles/search` | Search subtitles from external providers |
| POST | `/api/v1/subtitles/download` | Download a subtitle file from an external provider |
| GET | `/api/v1/subtitles/media/:media_id` | Get all subtitles for a media item |
| GET | `/api/v1/subtitles/:subtitle_id/verify-sync/:media_id` | Verify subtitle timing sync against the media's real audio (status measured / undetermined / invalid) |
| POST | `/api/v1/subtitles/translate` | Translate a subtitle file to another language |
| POST | `/api/v1/subtitles/upload` | Upload a custom subtitle file |
| DELETE | `/api/v1/subtitles/:subtitle_id` | Delete a subtitle (uploader or admin) |
| GET | `/api/v1/subtitles/languages` | List supported subtitle languages |
| GET | `/api/v1/subtitles/providers` | List available subtitle providers |

---

## Storage

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/storage/list/*path` | List files in a storage path |
| GET | `/api/v1/storage/roots` | List all configured storage roots |
| POST | `/api/v1/storage/roots` | Create a new storage root |
| GET | `/api/v1/storage-roots` | List all storage roots (alias) |
| GET | `/api/v1/storage-roots/:id/status` | Get scan and connection status of a storage root |

---

## Statistics

Responses are cached for 60 seconds.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/stats/directories/by-size` | Get directories sorted by total size |
| GET | `/api/v1/stats/duplicates/count` | Get duplicate file count |
| GET | `/api/v1/stats/overall` | Get overall catalog statistics |
| GET | `/api/v1/stats/smb/:smb_root` | Get statistics for a specific storage root |
| GET | `/api/v1/stats/filetypes` | Get file type distribution |
| GET | `/api/v1/stats/sizes` | Get file size distribution |
| GET | `/api/v1/stats/duplicates` | Get duplicate statistics |
| GET | `/api/v1/stats/duplicates/groups` | Get top duplicate groups |
| GET | `/api/v1/stats/access` | Get file access patterns |
| GET | `/api/v1/stats/growth` | Get storage growth trends |
| GET | `/api/v1/stats/scans` | Get scan history |

---

## SMB Discovery

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/smb/discover` | Discover SMB shares on the network |
| GET | `/api/v1/smb/discover` | Discover SMB shares (GET variant) |
| POST | `/api/v1/smb/test` | Test SMB connection parameters |
| GET | `/api/v1/smb/test` | Test SMB connection (GET variant) |
| POST | `/api/v1/smb/browse` | Browse contents of an SMB share |

---

## Scans

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/scans` | Queue a new scan for a storage root |
| GET | `/api/v1/scans` | List all scan jobs |
| GET | `/api/v1/scans/:job_id` | Get status of a specific scan job |

---

## Conversion

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/conversion/jobs` | Create a new format conversion job |
| GET | `/api/v1/conversion/jobs` | List conversion jobs |
| GET | `/api/v1/conversion/jobs/:id` | Get a specific conversion job |
| POST | `/api/v1/conversion/jobs/:id/cancel` | Cancel a running conversion job |
| GET | `/api/v1/conversion/formats` | List supported conversion formats |

---

## User Management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/users` | Create a new user (admin only) |
| GET | `/api/v1/users` | List all users |
| GET | `/api/v1/users/:id` | Get a user by ID |
| PUT | `/api/v1/users/:id` | Update a user |
| DELETE | `/api/v1/users/:id` | Delete a user |
| POST | `/api/v1/users/:id/reset-password` | Reset a user's password |
| POST | `/api/v1/users/:id/lock` | Lock a user account |
| POST | `/api/v1/users/:id/unlock` | Unlock a user account |

---

## Role Management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/roles` | Create a new role |
| GET | `/api/v1/roles` | List all roles |
| GET | `/api/v1/roles/:id` | Get a role by ID |
| PUT | `/api/v1/roles/:id` | Update a role |
| DELETE | `/api/v1/roles/:id` | Delete a role |
| GET | `/api/v1/roles/permissions` | List all available permissions |

---

## Configuration

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/configuration` | Get current system configuration |
| POST | `/api/v1/configuration/test` | Test configuration changes without applying |
| GET | `/api/v1/configuration/status` | Get system status overview |
| GET | `/api/v1/configuration/wizard/step/:step_id` | Get a setup wizard step |
| POST | `/api/v1/configuration/wizard/step/:step_id/validate` | Validate a wizard step |
| POST | `/api/v1/configuration/wizard/step/:step_id/save` | Save wizard step progress |
| GET | `/api/v1/configuration/wizard/progress` | Get overall wizard progress |
| POST | `/api/v1/configuration/wizard/complete` | Complete the setup wizard |

---

## Error Reporting

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/errors/report` | Report an application error |
| POST | `/api/v1/errors/crash` | Report an application crash |
| GET | `/api/v1/errors/reports` | List error reports |
| GET | `/api/v1/errors/reports/:id` | Get a specific error report |
| PUT | `/api/v1/errors/reports/:id/status` | Update error report status |
| GET | `/api/v1/errors/crashes` | List crash reports |
| GET | `/api/v1/errors/crashes/:id` | Get a specific crash report |
| PUT | `/api/v1/errors/crashes/:id/status` | Update crash report status |
| GET | `/api/v1/errors/statistics` | Get error statistics |
| GET | `/api/v1/errors/crash-statistics` | Get crash statistics |
| GET | `/api/v1/errors/health` | Get system health from error perspective |

---

## Log Management

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/logs/collect` | Create a new log collection |
| GET | `/api/v1/logs/collections` | List log collections |
| GET | `/api/v1/logs/collections/:id` | Get a log collection |
| GET | `/api/v1/logs/collections/:id/entries` | Get entries within a log collection |
| POST | `/api/v1/logs/collections/:id/export` | Export a log collection |
| GET | `/api/v1/logs/collections/:id/analyze` | Analyze a log collection for patterns |
| POST | `/api/v1/logs/share` | Create a shareable link for a log collection |
| GET | `/api/v1/logs/share/:token` | Access a shared log collection |
| DELETE | `/api/v1/logs/share/:id` | Revoke a shared log link |
| GET | `/api/v1/logs/stream` | Stream logs in real-time |
| GET | `/api/v1/logs/statistics` | Get log statistics |

---

## Collections

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/collections` | List media collections |
| POST | `/api/v1/collections` | Create a new collection |
| GET | `/api/v1/collections/:id` | Get a collection by ID |
| PUT | `/api/v1/collections/:id` | Update a collection |
| DELETE | `/api/v1/collections/:id` | Delete a collection |

---

## Assets

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/assets/:id` | Serve an asset file (public, no auth, static cache headers) |
| POST | `/api/v1/assets/request` | Request resolution of an asset (e.g., cover art) |
| GET | `/api/v1/assets/by-entity/:type/:id` | Get assets associated with an entity |

---

## Media Entities

Entity browsing endpoints for structured media. Responses are cached for 5 minutes.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/entities` | List all media entities with pagination |
| GET | `/api/v1/entities/types` | Get available entity types |
| GET | `/api/v1/entities/stats` | Get entity statistics by type |
| GET | `/api/v1/entities/duplicates` | List duplicate entity groups |
| GET | `/api/v1/entities/browse/:type` | Browse entities by media type |
| GET | `/api/v1/entities/:id` | Get a specific entity |
| GET | `/api/v1/entities/:id/children` | Get child entities (e.g., seasons of a show) |
| GET | `/api/v1/entities/:id/files` | Get files associated with an entity |
| GET | `/api/v1/entities/:id/metadata` | Get external metadata for an entity |
| GET | `/api/v1/entities/:id/duplicates` | Get duplicate entries for an entity |
| GET | `/api/v1/entities/:id/stream` | Stream media entity content |
| GET | `/api/v1/entities/:id/download` | Download media entity content |
| GET | `/api/v1/entities/:id/install-info` | Get installation information (games/software) |
| POST | `/api/v1/entities/:id/metadata/refresh` | Refresh external metadata from providers |
| PUT | `/api/v1/entities/:id/user-metadata` | Update user-specific metadata (rating, notes, tags) |
| POST | `/api/v1/entities/:id/user-metadata` | Update user-specific metadata (POST variant) |

---

## Analytics

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/analytics/access` | Log a media access event |
| POST | `/api/v1/analytics/event` | Log a custom analytics event |
| GET | `/api/v1/analytics/user/:user_id` | Get analytics for a specific user |
| GET | `/api/v1/analytics/system` | Get system-wide analytics |
| GET | `/api/v1/analytics/media/:media_id` | Get analytics for a specific media item |
| POST | `/api/v1/analytics/reports` | Create an analytics report |

---

## Reporting

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/reports/usage` | Get usage report |
| GET | `/api/v1/reports/performance` | Get performance report |

---

## Favorites

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/favorites` | List user's favorite items |
| POST | `/api/v1/favorites` | Add an item to favorites |
| DELETE | `/api/v1/favorites/:entity_type/:entity_id` | Remove an item from favorites |
| GET | `/api/v1/favorites/check/:entity_type/:entity_id` | Check if an item is favorited |

---

## Browse

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/browse/roots` | Get all storage roots for browsing |
| GET | `/api/v1/browse/directory/*path` | Browse a directory |
| GET | `/api/v1/browse/file-info/*path` | Get file information |
| GET | `/api/v1/browse/directory-sizes/*path` | Get subdirectory sizes |
| GET | `/api/v1/browse/duplicates/*path` | Get duplicates within a directory tree |

---

## Sync

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/sync/endpoints` | Create a sync endpoint |
| GET | `/api/v1/sync/endpoints` | List user's sync endpoints |
| GET | `/api/v1/sync/endpoints/:id` | Get a sync endpoint |
| PUT | `/api/v1/sync/endpoints/:id` | Update a sync endpoint |
| DELETE | `/api/v1/sync/endpoints/:id` | Delete a sync endpoint |
| POST | `/api/v1/sync/endpoints/:id/sync` | Start a sync operation |
| GET | `/api/v1/sync/sessions` | List user's sync sessions |
| GET | `/api/v1/sync/sessions/:id` | Get a sync session |
| POST | `/api/v1/sync/schedules` | Schedule a recurring sync |
| GET | `/api/v1/sync/statistics` | Get sync statistics |
| POST | `/api/v1/sync/cleanup` | Clean up old sync sessions |

---

## Challenges

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/challenges` | List all registered challenges |
| GET | `/api/v1/challenges/:id` | Get a specific challenge |
| POST | `/api/v1/challenges/:id/run` | Run a single challenge |
| POST | `/api/v1/challenges/run` | Run all challenges (synchronous, blocking) |
| POST | `/api/v1/challenges/run/category/:category` | Run all challenges in a category |
| GET | `/api/v1/challenges/results` | Get challenge execution results |

---

## Middleware Stack

All requests pass through the following middleware in order:

1. **SecurityHeaders** -- Sets security-related HTTP headers (X-Frame-Options, CSP, etc.)
2. **ConcurrencyLimiter(100)** -- Limits concurrent requests to 100
3. **RequestTimeout(60s)** -- Enforces a 60-second request timeout
4. **CORS** -- Configurable cross-origin resource sharing
5. **GinMiddleware (Prometheus)** -- Records HTTP request metrics
6. **Logger (Zap)** -- Structured request logging
7. **ErrorHandler** -- Standardized error responses
8. **RequestID** -- Generates unique request IDs
9. **InputValidation** -- Validates and sanitizes input
10. **CompressionMiddleware (Brotli/gzip)** -- Response compression with Brotli preferred

Additional per-group middleware:
- **RequireAuth (JWT)** -- Applied to all `/api/v1` routes
- **RateLimitByUser(30/min)** -- login and register
- **RateLimitByUser(600/min)** -- other `/api/v1/auth` session routes
- **RateLimitByUser(2000/min)** -- all other authenticated `/api/v1` routes
- **CacheHeaders(60s)** -- Applied to statistics endpoints
- **CacheHeaders(300s)** -- Applied to entity browsing endpoints
- **StaticCacheHeaders** -- Applied to asset serving

---

## Version History

### v2.0 (March 2026 -- Current)

- Added media entity system (`/api/v1/entities/*`)
- Added cloud sync endpoints (`/api/v1/sync/*`)
- Added browse endpoints (`/api/v1/browse/*`)
- Added favorites endpoints (`/api/v1/favorites/*`)
- Added analytics and reporting endpoints
- Added log management endpoints
- Added error and crash reporting endpoints
- Added configuration wizard endpoints
- Added asset management endpoints
- Added challenge system endpoints
- Added HTTP/3 (QUIC) support with Brotli compression
- Added rate limiting per user (initial release values 5/min and 100/min; current values: login/register 30/min, other auth 600/min, general 2000/min)
- Added request ID middleware
- Added input validation middleware

### v1.0 (February 2026)

- Initial API release
- Authentication (JWT) with login, register, refresh, logout
- Catalog browsing and search
- File download and copy operations
- Media operations with watch progress
- Subtitle management (search, download, translate, upload)
- Storage root management and scan operations
- Statistics endpoints
- SMB discovery
- Conversion job queue
- User and role management
- WebSocket real-time updates
- Prometheus metrics
- Recommendation engine
