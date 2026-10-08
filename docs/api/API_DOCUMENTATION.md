# Catalogizer REST API Reference

Complete REST API documentation for the Catalogizer backend (`catalog-api`). All endpoints are served under the base URL `http://localhost:8080`.

**Total registered endpoints: 208**

## Table of Contents

1. [Overview](#overview)
2. [Health and Infrastructure](#health-and-infrastructure)
3. [Service Discovery](#service-discovery)
4. [WebSocket](#websocket)
5. [Image Proxy](#image-proxy)
6. [Cover Art (Public)](#cover-art-public)
7. [Assets (Public)](#assets-public)
8. [Authentication](#authentication)
9. [Catalog Browsing](#catalog-browsing)
10. [Search](#search)
11. [Download](#download)
12. [Streaming](#streaming)
13. [File Copy Operations](#file-copy-operations)
14. [Media Browsing](#media-browsing)
15. [Media Operations](#media-operations)
16. [Recommendations](#recommendations)
17. [Subtitles](#subtitles)
18. [Storage and Storage Roots](#storage-and-storage-roots)
19. [Statistics](#statistics)
20. [SMB Discovery](#smb-discovery)
21. [Scanning](#scanning)
22. [Conversion](#conversion)
23. [Admin](#admin)
24. [User Management](#user-management)
25. [Role Management](#role-management)
26. [Configuration](#configuration)
27. [Error Reporting](#error-reporting)
28. [Log Management](#log-management)
29. [Collections](#collections)
30. [Assets (Authenticated)](#assets-authenticated)
31. [Media Entities](#media-entities)
32. [Analytics](#analytics)
33. [Reports](#reports)
34. [Favorites](#favorites)
35. [Playlists](#playlists)
36. [Browse](#browse)
37. [Sync](#sync)
38. [Challenges](#challenges)
39. [Global Middleware](#global-middleware)
40. [Error Handling](#error-handling)
41. [Rate Limiting](#rate-limiting)

---

## Overview

| Property | Value |
|---|---|
| Base URL | `http://localhost:8080` |
| HTTPS URL | `https://localhost:8443` (HTTP/2 + HTTP/3 QUIC) |
| API Version | `v1` |
| Auth Method | JWT Bearer Token |
| Content Type | `application/json` |
| Compression | Brotli (primary), gzip (fallback) |
| Database | SQLite (dev) / PostgreSQL (prod) |

All API routes under `/api/v1/*` (except auth endpoints, public assets, covers, and discovery) require a valid JWT token in the `Authorization` header:

```
Authorization: Bearer <jwt_token>
```

---

## Health and Infrastructure

### GET /health

Simple health check endpoint. No authentication required. Cached for 5 seconds.

**Description:** Returns basic service health status with version info.
**Auth:** None
**Rate Limit:** None

**Response:** 200 OK

```json
{
  "status": "healthy",
  "time": "2024-01-20T12:00:00Z",
  "version": "2.1.0",
  "build_number": "16",
  "build_date": "2026-03-31"
}
```

---

### GET /health/deep

Deep health check that pings the database. Times out after 100ms to avoid blocking. Cached for 5 seconds.

**Description:** Returns component-level health including database connectivity.
**Auth:** None
**Rate Limit:** None

**Response:** 200 OK

```json
{
  "status": "healthy",
  "time": "2024-01-20T12:00:00Z",
  "version": "2.1.0",
  "components": {
    "database": {
      "status": "healthy",
      "latency_ms": 2
    }
  }
}
```

**Response:** 200 OK (degraded, timeout exceeded)

```json
{
  "status": "degraded",
  "time": "2024-01-20T12:00:00Z",
  "version": "2.1.0",
  "message": "health check exceeded 100ms timeout",
  "components": {
    "timeout": {
      "status": "degraded",
      "message": "deep health check took too long"
    }
  }
}
```

**Response:** 503 Service Unavailable (unhealthy)

---

### GET /metrics

Prometheus metrics endpoint. Returns metrics in Prometheus exposition format. No authentication required.

**Description:** Exposes Prometheus metrics (HTTP request durations, counts, response sizes, goroutines, memory).
**Auth:** None
**Rate Limit:** None

**Response:** 200 OK (text/plain; Prometheus exposition format)

---

## Service Discovery

### GET /discovery

Service discovery endpoint for LAN clients to find the API. Cached for 60 seconds.

**Description:** Returns service connection info including WebSocket URL and API base URL.
**Auth:** None
**Rate Limit:** None

**Response:** 200 OK

```json
{
  "service": "catalogizer-api",
  "name": "Catalogizer API",
  "version": "2.1.0",
  "build": "16",
  "build_date": "2026-03-31",
  "host": "192.168.0.100",
  "port": 8080,
  "protocol": "http",
  "websocket_url": "ws://192.168.0.100:8080/ws",
  "api_base_url": "http://192.168.0.100:8080/api/v1",
  "capabilities": ["catalog", "media", "streaming", "sync", "websocket", "entities"],
  "database": "sqlite",
  "instance_id": "catalogizer-1234567890",
  "uptime_seconds": 3600
}
```

---

### GET /api/v1/discovery

Alias for `/discovery` under the API path. Same response. Cached for 60 seconds.

**Description:** Service discovery under the API prefix.
**Auth:** None
**Rate Limit:** None

---

### GET /api/v1/discovery/announce

Alias for `/discovery` under the announce path. Same response. Cached for 60 seconds.

**Description:** Service announcement endpoint.
**Auth:** None
**Rate Limit:** None

---

## WebSocket

### GET /ws

WebSocket connection endpoint for real-time updates (scan progress, asset updates, live events).

**Description:** Upgrades to WebSocket. Auth is via query parameter, not header.
**Auth:** Query parameter token
**Rate Limit:** None

**Connection:**

```
ws://localhost:8080/ws?token=<jwt_token>
```

**Message Types (server-to-client):**

```json
{
  "type": "asset_update",
  "action": "asset_ready",
  "asset_id": "abc123",
  "asset_type": "cover",
  "entity_type": "movie",
  "entity_id": "42"
}
```

---

## Image Proxy

### GET /api/v1/image-proxy

Proxy external images (TMDB, OMDB, IGDB CDNs) through the API for devices that cannot reach external CDNs directly.

**Description:** Fetches and proxies images from allowed CDN domains.
**Auth:** None
**Rate Limit:** None

**Query Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `url` | string | Yes | Full URL of the image to proxy |

**Allowed Domains:** `image.tmdb.org`, `img.omdbapi.com`, `images.igdb.com`

**Response:** Image binary with original Content-Type. Cached for 24 hours via `Cache-Control: public, max-age=86400`.

**Errors:**

| Status | Condition |
|---|---|
| 400 | Missing `url` parameter |
| 403 | Domain not in allowlist |
| 502 | Failed to fetch image from origin |

---

## Cover Art (Public)

Cover image endpoints are public (no auth required).

### GET /api/v1/cover/placeholder/:type

Serve a placeholder SVG image for a media type. Cached for 24 hours.

**Description:** Returns an SVG placeholder for the given media type.
**Auth:** None
**Rate Limit:** None

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `type` | string | Media type: `movie`, `tv_show`, `music_album`, `game`, `software`, `book`, `comic`, etc. |

**Response:** 200 OK (image/svg+xml)

---

### GET /api/v1/cover/url/:id

Get the cover image URL for a media entity. Cached for 5 minutes.

**Description:** Returns the URL where the cover image can be fetched.
**Auth:** None
**Rate Limit:** None

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | int | Media entity ID |

**Response:** 200 OK

```json
{
  "url": "/api/v1/cover/42",
  "source": "tmdb"
}
```

---

### GET /api/v1/cover/:id

Serve the actual cover image for a media entity. Cached for 24 hours.

**Description:** Returns the cover image binary data.
**Auth:** None
**Rate Limit:** None

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | int | Media entity ID |

**Response:** 200 OK (image/jpeg, image/png, or image/svg+xml)

---

## Assets (Public)

### GET /api/v1/assets/:id

Serve an asset by ID. Static cache headers (immutable, 1 year).

**Description:** Serves fingerprinted/content-hashed asset files.
**Auth:** None
**Rate Limit:** None

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | string | Asset ID |

**Response:** 200 OK with `Cache-Control: public, max-age=31536000, immutable`

---

## Authentication

Authentication endpoints are under `/api/v1/auth`. Write operations (login, register) have strict rate limiting (5/min). Read operations use standard rate limiting (100/min).

### POST /api/v1/auth/login

Authenticate a user and receive JWT tokens.

**Description:** Validates credentials and returns session + refresh tokens.
**Auth:** None
**Rate Limit:** Strict (5/min)

**Request Body:**

```json
{
  "username": "admin",
  "password": "securepassword",
  "device_info": {
    "device_type": "desktop",
    "platform": "linux",
    "app_version": "3.0.0"
  },
  "remember_me": true
}
```

**Response:** 200 OK

```json
{
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@example.com",
    "role_id": 1,
    "role": {
      "id": 1,
      "name": "admin",
      "permissions": ["*"]
    },
    "is_active": true,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  },
  "session_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-02T00:00:00Z"
}
```

**Errors:** 400 (malformed JSON), 401 (invalid credentials)

---

### POST /api/v1/auth/register

Register a new user account.

**Description:** Creates a new user and returns the user object.
**Auth:** None
**Rate Limit:** Strict (5/min)

**Request Body:**

```json
{
  "username": "newuser",
  "email": "newuser@example.com",
  "password": "securepassword",
  "first_name": "John",
  "last_name": "Doe"
}
```

| Field | Type | Required | Validation |
|---|---|---|---|
| `username` | string | Yes | Unique |
| `email` | string | Yes | Valid email, unique |
| `password` | string | Yes | Min 8 characters |
| `first_name` | string | Yes | - |
| `last_name` | string | Yes | - |

**Response:** 201 Created (User object)

**Errors:** 400 (validation failure), 409 (duplicate username or email)

---

### POST /api/v1/auth/refresh

Refresh an expired access token using a refresh token.

**Description:** Issues new session and refresh tokens.
**Auth:** None
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response:** 200 OK

```json
{
  "session_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-02T00:00:00Z"
}
```

**Errors:** 400 (malformed JSON), 401 (invalid or expired refresh token)

---

### POST /api/v1/auth/logout

Invalidate the current session token.

**Description:** Logs out the current session.
**Auth:** None (token optional)
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "message": "Logged out successfully"
}
```

---

### GET /api/v1/auth/me

Get the currently authenticated user's profile.

**Description:** Returns the user object for the JWT bearer.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK (User object)

---

### GET /api/v1/auth/status

Get authentication system status (whether auth is configured, if initial setup is needed).

**Description:** Returns auth system status without requiring authentication.
**Auth:** None
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "auth_configured": true,
  "users_exist": true,
  "registration_enabled": true
}
```

---

### GET /api/v1/auth/permissions

Get the permissions for the current user.

**Description:** Returns the permission set granted to the authenticated user's role.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "permissions": ["media.view", "media.edit", "media.upload"]
}
```

---

### GET /api/v1/auth/profile

Alias for `/api/v1/auth/me`. Returns the current user profile.

**Description:** Same as GET /api/v1/auth/me.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/auth/init-status

Get initialization status (whether the system has been set up).

**Description:** Returns whether the system needs initial configuration.
**Auth:** None
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "initialized": true,
  "has_admin": true,
  "has_storage_roots": true,
  "has_scanned_files": true
}
```

---

### POST /api/v1/auth/change-password

Change the password for the currently authenticated user.

**Description:** Updates the user's password after verifying the current password.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "current_password": "oldpassword",
  "new_password": "newsecurepassword"
}
```

**Response:** 200 OK

```json
{
  "message": "Password changed successfully"
}
```

**Errors:** 400 (validation failure), 401 (current password incorrect)

---

## Catalog Browsing

Browse the file catalog across all configured storage roots (SMB, FTP, NFS, WebDAV, local). All endpoints require authentication.

### GET /api/v1/catalog

List all available storage root directories.

**Description:** Returns the list of configured storage root names.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "roots": [
    "nas-media",
    "nas-backup",
    "local-storage"
  ]
}
```

---

### GET /api/v1/catalog/{path}

List files and directories at the specified path.

**Description:** Browse files and subdirectories within a storage root.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `sort_by` | string | `name` | Sort field: `name`, `size`, `modified` |
| `sort_order` | string | `asc` | Sort order: `asc`, `desc` |
| `limit` | int | `100` | Max results to return |
| `offset` | int | `0` | Pagination offset |

**Response:** 200 OK

```json
{
  "files": [
    {
      "id": 1024,
      "storage_root_id": 1,
      "storage_root_name": "nas-media",
      "path": "movies/The Matrix (1999)",
      "name": "The Matrix (1999)",
      "size": 4294967296,
      "is_directory": true,
      "mime_type": null,
      "created_at": "2024-01-15T10:30:00Z",
      "modified_at": "2024-01-15T10:30:00Z"
    }
  ],
  "count": 1,
  "limit": 50,
  "offset": 0
}
```

---

### GET /api/v1/catalog-info/{path}

Get detailed information about a specific file or directory.

**Description:** Returns file metadata including size, type, timestamps.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK (FileInfo object)

**Errors:** 400 (empty path), 404 (path not found)

---

## Search

### GET /api/v1/search

Search for files and directories using various criteria (catalog-level search).

**Description:** Full-text search across the file catalog with filtering.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `query` | string | Yes | - | Search term (filename match) |
| `path` | string | No | - | Path filter |
| `extension` | string | No | - | File extension filter |
| `mime_type` | string | No | - | MIME type filter |
| `min_size` | int | No | - | Minimum file size (bytes) |
| `max_size` | int | No | - | Maximum file size (bytes) |
| `smb_roots` | string | No | - | Comma-separated storage root names |
| `is_directory` | bool | No | - | Filter by directory status |
| `sort_by` | string | No | `name` | Sort field |
| `sort_order` | string | No | `asc` | Sort direction |
| `limit` | int | No | `100` | Max results |
| `offset` | int | No | `0` | Pagination offset |

**Response:** 200 OK

```json
{
  "files": [...],
  "total": 15,
  "count": 15,
  "limit": 100,
  "offset": 0
}
```

**Errors:** 400 (missing query parameter)

---

### GET /api/v1/search/duplicates

Find groups of duplicate files within a storage root (catalog-level).

**Description:** Detects duplicate files based on content hashing.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `smb_root` | string | Yes | - | Storage root name to search |
| `min_count` | int | No | `2` | Minimum duplicates per group |
| `limit` | int | No | `50` | Max groups to return |

**Response:** 200 OK

```json
{
  "groups": [
    {
      "id": 42,
      "file_count": 3,
      "total_size": 12884901888,
      "created_at": "2024-01-10T08:00:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    }
  ],
  "count": 1
}
```

---

### GET /api/v1/search/files

Search files in the database (file-level search handler).

**Description:** Database-level file search with advanced filtering.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | Yes | Search term |
| `storage_root_id` | int | No | Filter by storage root |
| `extension` | string | No | File extension filter |
| `min_size` | int | No | Minimum size in bytes |
| `max_size` | int | No | Maximum size in bytes |
| `limit` | int | No | Max results |
| `offset` | int | No | Pagination offset |

**Response:** 200 OK

```json
{
  "files": [...],
  "total": 42,
  "limit": 100,
  "offset": 0
}
```

---

### GET /api/v1/search/files/duplicates

Find duplicate files via the file-level search handler.

**Description:** Detects duplicate files using the database search handler.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `storage_root_id` | int | No | Filter by storage root |
| `min_count` | int | No | Minimum duplicates per group |
| `limit` | int | No | Max groups |

**Response:** 200 OK

---

### POST /api/v1/search/advanced

Advanced multi-criteria search with structured query body.

**Description:** Accepts a JSON body with complex search criteria.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "query": "matrix",
  "filters": {
    "extensions": [".mkv", ".mp4"],
    "min_size": 1073741824,
    "max_size": 10737418240,
    "storage_root_ids": [1, 2],
    "is_directory": false,
    "date_from": "2024-01-01",
    "date_to": "2024-12-31"
  },
  "sort": {
    "field": "size",
    "order": "desc"
  },
  "limit": 50,
  "offset": 0
}
```

**Response:** 200 OK

---

## Download

### GET /api/v1/download/file/:id

Download a single file by its database ID.

**Description:** Streams the file binary data for download.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | int | File ID |

**Response Headers:**

```
Content-Disposition: attachment; filename="movie.mkv"
Content-Type: application/octet-stream
Content-Length: 4294967296
```

**Errors:** 400 (invalid ID, cannot download directory), 404 (file not found)

---

### GET /api/v1/download/directory/{path}

Download a directory as a compressed archive.

**Description:** Compresses and streams a directory for download.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `format` | string | `zip` | Archive format: `zip`, `tar`, `tar.gz` |

**Errors:** 400 (unsupported format, directory too large), 404 (not found or empty)

---

### POST /api/v1/download/archive

Create and download an archive from multiple specified file paths.

**Description:** Bundles selected files into an archive for download.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "paths": [
    "movies/The Matrix (1999)/The.Matrix.mkv",
    "movies/Inception (2010)/Inception.mkv"
  ],
  "format": "zip",
  "smb_root": "nas-media"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `paths` | string[] | Yes | Array of file paths to include |
| `format` | string | No | `zip` (default), `tar`, `tar.gz` |
| `smb_root` | string | No | Storage root name |

---

## Streaming

### GET /api/v1/stream/:id

Stream a file from any storage backend (SMB, FTP, NFS, WebDAV, local).

**Description:** Proxies file data for real-time streaming/playback. Supports range requests.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | int | File ID in the database |

**Response Headers:**

```
Content-Type: video/x-matroska
Accept-Ranges: bytes
Content-Length: 4294967296
```

**Errors:** 400 (invalid ID), 404 (file not found), 500 (storage backend error)

---

## File Copy Operations

### POST /api/v1/copy/storage

Copy a file to a storage location.

**Description:** Copies a file between storage locations.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "source_path": "/tmp/upload/document.pdf",
  "dest_path": "/documents/archive/document.pdf",
  "storage_id": "local"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `source_path` | string | Yes | Source file path |
| `dest_path` | string | Yes | Destination file path |
| `storage_id` | string | Yes | Target storage root ID |

**Response:** 200 OK

```json
{
  "message": "File copied to storage successfully",
  "source": "/tmp/upload/document.pdf",
  "destination": "/documents/archive/document.pdf",
  "storage_id": "local"
}
```

---

### POST /api/v1/copy/local

Copy a file from a remote storage (SMB) to local filesystem.

**Description:** Downloads a file from remote storage to a local path.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "source_path": "nas-media:movies/movie.mkv",
  "destination_path": "/local/downloads/movie.mkv",
  "overwrite": false
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `source_path` | string | Yes | Source in `host:path` format |
| `destination_path` | string | Yes | Local destination path |
| `overwrite` | bool | No | Overwrite existing files (default: false) |

**Errors:** 400 (invalid source format), 409 (file exists and overwrite=false)

---

### POST /api/v1/copy/upload

Upload a file from local filesystem to SMB storage.

**Description:** Uploads a file to remote storage via multipart form data.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Content-Type:** `multipart/form-data`

**Form Fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `file` | file | Yes | File to upload |
| `destination` | string | Yes | Destination in `host:path` format |
| `overwrite` | string | No | `"true"` to overwrite existing files |

**Response:** 200 OK

```json
{
  "message": "File uploaded successfully",
  "filename": "document.pdf",
  "destination": "nas-media:documents/document.pdf",
  "size": 1048576
}
```

---

## Media Browsing

Endpoints for searching and querying media at the database level.

### GET /api/v1/media/search

Search media items in the database.

**Description:** Full-text search across scanned media items.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | Yes | Search term |
| `type` | string | No | Media type filter |
| `limit` | int | No | Max results |
| `offset` | int | No | Pagination offset |

**Response:** 200 OK

```json
{
  "results": [...],
  "total": 42,
  "limit": 100,
  "offset": 0
}
```

---

### GET /api/v1/media/stats

Get aggregate media statistics.

**Description:** Returns counts and size totals grouped by media type.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "total_files": 25000,
  "total_size": 5497558138880,
  "by_type": {
    "video": {"count": 5000, "size": 2748779069440},
    "audio": {"count": 12000, "size": 549755813888}
  }
}
```

---

### GET /api/v1/media/recent

Get recently added or accessed media items.

**Description:** Returns media items sorted by recency.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | int | `20` | Max results |
| `type` | string | - | Media type filter |

**Response:** 200 OK

---

### GET /api/v1/media/popular

Get most popular/frequently accessed media items.

**Description:** Returns media items sorted by popularity/access count.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | int | `20` | Max results |
| `type` | string | - | Media type filter |

**Response:** 200 OK

---

### GET /api/v1/media/by-path

Look up a media item by its file path.

**Description:** Resolves a file path to a media entity.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `path` | string | Yes | File or directory path |

**Response:** 200 OK

---

### POST /api/v1/media/analyze

Analyze a media file for metadata extraction.

**Description:** Triggers media analysis pipeline for a specific file.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "path": "/movies/The Matrix (1999)/The.Matrix.mkv",
  "storage_root_id": 1
}
```

**Response:** 200 OK

---

## Media Operations

### GET /api/v1/media/:id

Get detailed media item information by ID.

**Description:** Returns full media metadata including external metadata, versions, and user-specific data.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | int | Media item ID |

**Response:** 200 OK

```json
{
  "id": 42,
  "title": "The Matrix (1999)",
  "media_type": "video",
  "year": 1999,
  "description": "A computer hacker learns about the true nature of reality.",
  "cover_image": "https://image.tmdb.org/t/p/w500/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg",
  "rating": 8.7,
  "quality": "1080p",
  "file_size": 4294967296,
  "duration": 8160,
  "directory_path": "/movies/The Matrix (1999)",
  "smb_path": "//nas/media/movies/The Matrix (1999)",
  "created_at": "2024-01-15 10:30:00",
  "updated_at": "2024-01-15 10:30:00",
  "external_metadata": [],
  "versions": [],
  "is_favorite": true,
  "watch_progress": 0.75,
  "last_watched": "2024-01-20 20:00:00",
  "is_downloaded": false
}
```

**Errors:** 400 (invalid ID), 404 (not found)

---

### PUT /api/v1/media/:id/progress

Update watch progress for a media item.

**Description:** Saves the playback position for continue-watching functionality.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "progress": 0.75
}
```

| Field | Type | Required | Validation |
|---|---|---|---|
| `progress` | float | Yes | 0.0 to 1.0 |

**Response:** 200 OK

```json
{
  "success": true,
  "message": "Watch progress updated successfully"
}
```

---

### PUT /api/v1/media/:id/favorite

Update the favorite status for a media item.

**Description:** Toggles the favorite flag on a media item.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "favorite": true
}
```

**Response:** 200 OK

```json
{
  "success": true,
  "message": "Favorite status updated successfully"
}
```

---

### POST /api/v1/media/:id/refresh

Refresh metadata for a specific media item from external providers.

**Description:** Re-fetches metadata from TMDB, OMDB, MusicBrainz, etc.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "message": "Metadata refresh initiated"
}
```

---

### GET /api/v1/media/:id/quality

Get quality information for a media item (resolution, codec, bitrate).

**Description:** Returns technical quality details for the media file.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "id": 42,
  "resolution": "1920x1080",
  "codec": "h264",
  "bitrate": 8000000,
  "quality_label": "1080p"
}
```

---

## Recommendations

### GET /api/v1/recommendations/similar/:media_id

Get media items similar to a given media item.

**Description:** Uses content-based and collaborative filtering to find similar items.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `media_id` | int | Media item ID |

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `max_local_items` | int | `10` | Max local results |
| `max_external_items` | int | `5` | Max external results |
| `include_external` | bool | `false` | Include external API results |
| `similarity_threshold` | float | `0.3` | Minimum similarity score |

**Response:** 200 OK

```json
{
  "media_id": "42",
  "local_items": [...],
  "external_items": [...],
  "total_local": 8,
  "total_external": 5
}
```

---

### GET /api/v1/recommendations/trending

Get trending media items based on recent activity.

**Description:** Returns media items trending in a given time window.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `media_type` | string | - | Filter by type: `video`, `audio`, etc. |
| `limit` | int | `20` | Max results |
| `time_range` | string | `week` | Time range: `day`, `week`, `month`, `year` |

**Response:** 200 OK

```json
{
  "items": [
    {
      "id": 100,
      "title": "Trending Movie 1",
      "media_type": "video",
      "rating": 8.2,
      "is_favorite": false,
      "watch_progress": 0,
      "is_downloaded": true
    }
  ],
  "media_type": "video",
  "time_range": "week",
  "generated_at": "2024-01-20T12:00:00Z"
}
```

---

### GET /api/v1/recommendations/personalized/:user_id

Get personalized recommendations based on viewing history.

**Description:** ML-based recommendations tailored to user preferences.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `user_id` | int | User ID |

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | int | `20` | Max results |

**Response:** 200 OK

```json
{
  "user_id": 1,
  "items": [...],
  "generated_at": "2024-01-20T12:00:00Z"
}
```

---

## Subtitles

### GET /api/v1/subtitles/search

Search for subtitles across multiple providers.

**Description:** Queries subtitle databases for matching subtitles.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `media_path` | string | Yes | Path to the media file |
| `title` | string | No | Media title override |
| `year` | int | No | Release year |
| `season` | int | No | TV season number |
| `episode` | int | No | TV episode number |
| `languages` | string | No | Comma-separated language codes |
| `providers` | string | No | Comma-separated provider names |

**Available Providers:** `opensubtitles`, `subdb`, `yifysubtitles`, `subscene`, `addic7ed`

**Response:** 200 OK

```json
{
  "success": true,
  "results": [
    {
      "id": "os-12345",
      "title": "The Matrix",
      "language": "English",
      "language_code": "en",
      "provider": "opensubtitles",
      "format": "srt",
      "rating": 9.2,
      "download_count": 150000
    }
  ],
  "count": 1
}
```

---

### POST /api/v1/subtitles/download

Download a specific subtitle by result ID.

**Description:** Downloads and saves a subtitle file for a media item.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "media_item_id": 42,
  "result_id": "os-12345",
  "language": "en"
}
```

**Response:** 200 OK

```json
{
  "success": true,
  "track": {
    "id": "sub-001",
    "language": "English",
    "language_code": "en",
    "format": "srt",
    "is_default": false,
    "is_forced": false
  }
}
```

---

### GET /api/v1/subtitles/media/:media_id

Get all subtitle tracks for a media item.

**Description:** Lists available subtitles associated with a media file.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "success": true,
  "subtitles": [
    {
      "id": "sub-001",
      "language": "English",
      "language_code": "en",
      "format": "srt"
    }
  ],
  "media_item_id": 42
}
```

---

### GET /api/v1/subtitles/:subtitle_id/verify-sync/:media_id

Verify if a subtitle is properly synchronized with its media.

**Description:** Checks subtitle timing alignment against the media file.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "success": true,
  "sync_result": {
    "is_synced": true,
    "offset_ms": 150,
    "confidence": 0.95
  }
}
```

---

### POST /api/v1/subtitles/translate

Translate a subtitle to another language.

**Description:** Machine-translates subtitle content between languages.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "subtitle_id": "sub-001",
  "source_language": "en",
  "target_language": "es"
}
```

**Response:** 200 OK

```json
{
  "success": true,
  "translated_track": {
    "id": "sub-002",
    "language": "Spanish",
    "language_code": "es",
    "format": "srt"
  }
}
```

---

### POST /api/v1/subtitles/upload

Upload a subtitle file for a media item.

**Description:** Uploads a custom subtitle file.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Content-Type:** `multipart/form-data`

**Form Fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `media_item_id` | int | Yes | Media item ID |
| `language` | string | Yes | Language name (e.g., "English") |
| `language_code` | string | Yes | ISO 639-1 code (e.g., "en") |
| `file` | file | Yes | Subtitle file (.srt, .vtt, .ass, .txt) |

---

### GET /api/v1/subtitles/languages

Get the list of supported subtitle languages.

**Description:** Returns all languages available for subtitle operations.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "success": true,
  "languages": [
    {"code": "en", "name": "English", "native_name": "English"},
    {"code": "es", "name": "Spanish", "native_name": "Espanol"},
    {"code": "fr", "name": "French", "native_name": "Francais"},
    {"code": "de", "name": "German", "native_name": "Deutsch"},
    {"code": "ru", "name": "Russian", "native_name": "Russkij"}
  ],
  "count": 19
}
```

---

### GET /api/v1/subtitles/providers

Get the list of supported subtitle providers.

**Description:** Returns available subtitle provider configurations.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "success": true,
  "providers": [
    {"provider": "opensubtitles", "name": "OpenSubtitles", "description": "Large subtitle database with multiple languages", "supported": true},
    {"provider": "subdb", "name": "SubDB", "description": "Hash-based subtitle matching", "supported": true},
    {"provider": "yifysubtitles", "name": "YIFY Subtitles", "description": "Subtitles for YIFY movie releases", "supported": true},
    {"provider": "subscene", "name": "Subscene", "description": "Community-driven subtitle site", "supported": true},
    {"provider": "addic7ed", "name": "Addic7ed", "description": "TV show subtitles with translations", "supported": true}
  ],
  "count": 5
}
```

---

## Storage and Storage Roots

### GET /api/v1/storage/list/{path}

List files in a storage path.

**Description:** Browse files within a specific storage backend.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `storage_id` | string | Yes | Storage root ID |

**Response:** 200 OK

```json
{
  "path": "/documents",
  "storage_id": "local",
  "files": [...]
}
```

---

### GET /api/v1/storage/roots

Get all available storage root configurations.

**Description:** Lists configured storage roots (SMB, FTP, NFS, WebDAV, local).
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "roots": [
    {
      "id": 1,
      "name": "Local Storage",
      "protocol": "local",
      "path": "/data/storage",
      "enabled": true
    }
  ]
}
```

---

### POST /api/v1/storage/roots

Create a new storage root configuration.

**Description:** Registers a new storage location for scanning.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "name": "NAS Media",
  "protocol": "smb",
  "host": "192.168.0.241",
  "port": 445,
  "path": "/media",
  "username": "user",
  "password": "password",
  "domain": "WORKGROUP",
  "enabled": true
}
```

Further fields: `url` (required for `webdav`), `mount_point` and `options` (NFS), `allow_empty` (default `false`: a scan that finds nothing fails with `empty_root`), `max_depth`. `port` must be 1-65535.
The accepted protocols are the ones the client factory builds (`local`, `smb`, `ftp`, `nfs`, `webdav` and every protocol registered at start-up); the 400 answer lists them.
`GET /api/v1/storage/roots` returns the `url` with its password redacted; `GET /api/v1/scans[/:job_id]` returns `reason`.

**Response:** 201 Created

---

### GET /api/v1/storage-roots

Alias for GET /api/v1/storage/roots. Returns same response.

**Description:** Alternative path for listing storage roots.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/storage-roots/:id/status

Get the current status of a specific storage root (online/offline, last scan time).

**Description:** Checks connectivity and returns scan status for a storage root.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | int | Storage root ID |

**Response:** 200 OK

```json
{
  "id": 1,
  "name": "NAS Media",
  "status": "online",
  "last_scan": "2024-01-15T10:00:00Z",
  "file_count": 25000,
  "total_size": 5497558138880
}
```

---

## Statistics

All statistics endpoints are cached for 60 seconds.

### GET /api/v1/stats/directories/by-size

Get directories sorted by total size.

**Description:** Returns the largest directories across storage roots.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `smb_root` | string | Yes | - | Storage root name |
| `limit` | int | No | `50` | Max results |

**Response:** 200 OK

```json
{
  "directories": [
    {
      "path": "/movies/4K",
      "name": "4K",
      "storage_root_name": "nas-media",
      "file_count": 150,
      "total_size": 1099511627776
    }
  ],
  "count": 10
}
```

---

### GET /api/v1/stats/duplicates/count

Get statistics about duplicate files.

**Description:** Returns aggregate duplicate file counts and wasted space.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `smb_root` | string | No | Storage root name |

**Response:** 200 OK

```json
{
  "duplicate_groups": 42,
  "total_duplicates": 125,
  "total_wasted_space": 53687091200,
  "smb_root": "nas-media"
}
```

---

### GET /api/v1/stats/overall

Get comprehensive catalog statistics.

**Description:** Returns total files, directories, sizes, duplicates, and storage root counts.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "success": true,
  "data": {
    "total_files": 25000,
    "total_directories": 3200,
    "total_size": 5497558138880,
    "total_duplicates": 125,
    "duplicate_groups": 42,
    "storage_roots_count": 5,
    "active_storage_roots": 4,
    "last_scan_time": 1705312200
  }
}
```

---

### GET /api/v1/stats/smb/:smb_root

Get statistics for a specific storage root.

**Description:** Returns file counts, sizes, and scan info for one storage root.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "success": true,
  "data": {
    "name": "nas-media",
    "total_files": 15000,
    "total_directories": 2000,
    "total_size": 3298534883328,
    "duplicate_files": 80,
    "duplicate_groups": 25,
    "last_scan_time": 1705312200,
    "is_online": true
  }
}
```

---

### GET /api/v1/stats/filetypes

Get file type distribution statistics.

**Description:** Returns file counts and sizes grouped by extension/type.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `smb_root` | string | - | Storage root filter |
| `limit` | int | `50` | Max results (max 1000) |

**Response:** 200 OK

```json
{
  "success": true,
  "data": [
    {"file_type": "video", "extension": ".mkv", "count": 5000, "total_size": 2748779069440, "average_size": 549755813},
    {"file_type": "audio", "extension": ".flac", "count": 12000, "total_size": 549755813888, "average_size": 45812984}
  ]
}
```

---

### GET /api/v1/stats/sizes

Get file size distribution.

**Description:** Returns file counts bucketed by size range.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `smb_root` | string | Storage root filter |

**Response:** 200 OK

```json
{
  "success": true,
  "data": {
    "tiny": 500,
    "small": 3000,
    "medium": 8000,
    "large": 10000,
    "huge": 3000,
    "massive": 500
  }
}
```

Size buckets: tiny (<1KB), small (1KB-1MB), medium (1MB-10MB), large (10MB-100MB), huge (100MB-1GB), massive (>1GB).

---

### GET /api/v1/stats/duplicates

Get duplicate file statistics.

**Description:** Returns aggregate duplicate stats across the catalog.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "success": true,
  "data": {
    "total_duplicates": 125,
    "duplicate_groups": 42,
    "wasted_space": 53687091200,
    "largest_duplicate_group": 8,
    "average_group_size": 2.97
  }
}
```

---

### GET /api/v1/stats/duplicates/groups

Get the largest duplicate groups.

**Description:** Lists duplicate file groups sorted by count or size.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `sort_by` | string | `count` | `count` or `size` |
| `limit` | int | `20` | Max results (max 100) |
| `smb_root` | string | - | Storage root filter |

---

### GET /api/v1/stats/access

Get file access pattern statistics.

**Description:** Analyzes file access patterns over a time period.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `smb_root` | string | - | Storage root filter |
| `days` | int | `30` | Analysis period (max 365) |

---

### GET /api/v1/stats/growth

Get storage growth trends over time.

**Description:** Returns monthly file and size growth data.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `smb_root` | string | - | Storage root filter |
| `months` | int | `12` | Analysis period (max 60) |

**Response:** 200 OK

```json
{
  "success": true,
  "data": {
    "monthly_growth": [
      {"month": "2024-01", "files_added": 500, "size_added": 274877906944, "total_files": 25000, "total_size": 5497558138880}
    ],
    "total_growth_rate": 12.5,
    "file_growth_rate": 8.3,
    "size_growth_rate": 15.2
  }
}
```

---

### GET /api/v1/stats/scans

Get scan operation history.

**Description:** Returns historical scan results with pagination.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `smb_root` | string | - | Storage root filter |
| `limit` | int | `50` | Max results (max 1000) |
| `offset` | int | `0` | Pagination offset |

**Response:** 200 OK

```json
{
  "success": true,
  "data": {
    "scans": [
      {
        "id": 23,
        "storage_root_id": 1,
        "scan_type": "full",
        "status": "completed",
        "start_time": "2024-01-15T10:00:00Z",
        "end_time": "2024-01-15T10:45:00Z",
        "files_processed": 25000,
        "files_added": 150,
        "files_updated": 30,
        "files_deleted": 5,
        "error_count": 0
      }
    ],
    "total_count": 100,
    "limit": 50,
    "offset": 0
  }
}
```

---

## SMB Discovery

### POST /api/v1/smb/discover

Discover available SMB shares on a host.

**Description:** Scans an SMB host for available shares.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "host": "192.168.1.100",
  "username": "user",
  "password": "password",
  "domain": "WORKGROUP"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `host` | string | Yes | SMB host address |
| `username` | string | Yes | Authentication username |
| `password` | string | Yes | Authentication password |
| `domain` | string | No | Windows domain |

**Response:** 200 OK (array of SMBShareInfo objects)

---

### GET /api/v1/smb/discover

Discover SMB shares using query parameters (convenience endpoint for testing).

**Description:** Same as POST but with query string parameters.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required |
|---|---|---|
| `host` | string | Yes |
| `username` | string | Yes |
| `password` | string | Yes |
| `domain` | string | No |

---

### POST /api/v1/smb/test

Test connectivity to an SMB share.

**Description:** Verifies that the API server can reach and authenticate to an SMB share.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "host": "192.168.1.100",
  "port": 445,
  "share": "media",
  "username": "user",
  "password": "password",
  "domain": "WORKGROUP"
}
```

**Response:** 200 OK

```json
{
  "success": true,
  "host": "192.168.1.100",
  "share": "media",
  "username": "user",
  "connection": true
}
```

---

### GET /api/v1/smb/test

Test SMB connection using query parameters.

**Description:** Same as POST but with query string parameters.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Required | Default |
|---|---|---|---|
| `host` | string | Yes | - |
| `share` | string | Yes | - |
| `username` | string | Yes | - |
| `password` | string | Yes | - |
| `domain` | string | No | - |
| `port` | int | No | `445` |

---

### POST /api/v1/smb/browse

Browse files and directories in an SMB share.

**Description:** Lists contents of a remote SMB share directory.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "host": "192.168.1.100",
  "port": 445,
  "share": "media",
  "username": "user",
  "password": "password",
  "domain": "WORKGROUP",
  "path": "movies"
}
```

**Response:** 200 OK (array of SMBFileEntry objects)

---

## Scanning

### POST /api/v1/scans

Queue a new scan job for a storage root.

**Description:** Initiates a file system scan on a configured storage root.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "storage_root_id": 1,
  "scan_type": "full"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `storage_root_id` | int | Yes | Storage root to scan |
| `scan_type` | string | No | `full` or `incremental` |

**Response:** 202 Accepted

```json
{
  "job_id": "scan-abc123",
  "storage_root_id": 1,
  "status": "queued"
}
```

---

### GET /api/v1/scans

List all scan jobs with their status.

**Description:** Returns recent and active scan operations.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "scans": [
    {
      "job_id": "scan-abc123",
      "storage_root_id": 1,
      "status": "running",
      "progress": 0.45,
      "files_processed": 11250,
      "started_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

---

### GET /api/v1/scans/:job_id

Get the status of a specific scan job.

**Description:** Returns detailed progress for a scan operation.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `job_id` | string | Scan job ID |

**Response:** 200 OK

```json
{
  "job_id": "scan-abc123",
  "storage_root_id": 1,
  "status": "completed",
  "progress": 1.0,
  "files_processed": 25000,
  "files_added": 150,
  "files_updated": 30,
  "files_deleted": 5,
  "errors": [],
  "started_at": "2024-01-15T10:00:00Z",
  "completed_at": "2024-01-15T10:45:00Z"
}
```

---

## Conversion

### POST /api/v1/conversion/jobs

Create a new media format conversion job.

**Description:** Queues a media file for format conversion.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "source_path": "/media/video.mp4",
  "target_path": "/media/video.mp3",
  "source_format": "mp4",
  "target_format": "mp3",
  "conversion_type": "audio",
  "quality": "high",
  "priority": 1,
  "settings": "{\"bitrate\": \"320k\"}",
  "scheduled_for": "2024-01-20T10:00:00Z"
}
```

**Response:** 200 OK (ConversionJob object)

---

### GET /api/v1/conversion/jobs

List conversion jobs for the current user.

**Description:** Returns conversion jobs with optional status filtering.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `status` | string | - | Filter: `pending`, `running`, `completed`, `failed`, `cancelled` |
| `limit` | int | `50` | Max results (max 100) |
| `offset` | int | `0` | Pagination offset |

**Response:** 200 OK (array of ConversionJob objects)

---

### GET /api/v1/conversion/jobs/:id

Get a specific conversion job by ID.

**Description:** Returns full details for a conversion job.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Errors:** 400 (invalid ID), 404 (not found)

---

### POST /api/v1/conversion/jobs/:id/cancel

Cancel a running conversion job.

**Description:** Cancels an in-progress or queued conversion.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "message": "Job cancelled successfully"
}
```

---

### DELETE /api/v1/conversion/jobs/:id

Delete a conversion job record.

**Description:** Removes a completed or cancelled conversion job from history.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 204 No Content

---

### POST /api/v1/conversion/jobs/:id/retry

Retry a failed conversion job.

**Description:** Re-queues a previously failed conversion job.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK (ConversionJob object with status reset)

---

### GET /api/v1/conversion/jobs/:id/download

Download the output file of a completed conversion job.

**Description:** Streams the converted file for download.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** Binary file download with appropriate Content-Type.

**Errors:** 400 (job not completed), 404 (not found)

---

### GET /api/v1/conversion/formats

Get all supported conversion formats.

**Description:** Lists input and output formats by media type.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "video": {
    "input": ["mp4", "mkv", "avi", "mov", "wmv", "flv", "webm"],
    "output": ["mp4", "mkv", "avi", "webm"]
  },
  "audio": {
    "input": ["mp3", "flac", "wav", "aac", "ogg", "wma", "m4a"],
    "output": ["mp3", "flac", "wav", "aac", "ogg"]
  },
  "document": {
    "input": ["pdf", "doc", "docx", "txt", "rtf"],
    "output": ["pdf", "txt"]
  },
  "image": {
    "input": ["jpg", "png", "gif", "bmp", "webp", "tiff"],
    "output": ["jpg", "png", "webp"]
  }
}
```

---

## Admin

Administration endpoints for system management. All require authentication and admin privileges.

### GET /api/v1/admin/system-info

Get system information (OS, CPU, memory, disk, Go runtime).

**Description:** Returns detailed system metrics and runtime info.
**Auth:** Required (admin)
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "version": "2.1.0",
  "build_number": "16",
  "os": "linux",
  "arch": "amd64",
  "go_version": "go1.25",
  "goroutines": 42,
  "memory_alloc_mb": 128,
  "cpu_count": 8,
  "uptime_seconds": 86400
}
```

---

### GET /api/v1/admin/users

List all users (admin view with additional fields).

**Description:** Returns user list with admin-level detail.
**Auth:** Required (admin)
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/admin/users/:id

Update a user via admin interface.

**Description:** Admin-level user update (can change role, active status).
**Auth:** Required (admin)
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/admin/storage

Get storage utilization info across all backends.

**Description:** Returns disk space usage for all configured storage roots.
**Auth:** Required (admin)
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/admin/backups

List available backups.

**Description:** Returns database and configuration backup history.
**Auth:** Required (admin)
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/admin/backups

Create a new backup.

**Description:** Triggers a database and/or configuration backup.
**Auth:** Required (admin)
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "type": "full",
  "include_database": true,
  "include_config": true
}
```

---

### POST /api/v1/admin/backups/:id/restore

Restore from a backup.

**Description:** Restores the system from a specified backup.
**Auth:** Required (admin)
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `id` | int | Backup ID |

---

### POST /api/v1/admin/storage/scan

Trigger a storage scan via admin interface.

**Description:** Admin-level scan trigger (can scan all roots).
**Auth:** Required (admin)
**Rate Limit:** Standard (100/min)

---

## User Management

All user management endpoints require a JWT token and appropriate permissions.

### POST /api/v1/users

Create a new user.

**Description:** Creates a user account with role assignment.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "securepassword123",
  "role_id": 2,
  "first_name": "John",
  "last_name": "Doe",
  "display_name": "John D.",
  "time_zone": "America/New_York",
  "language": "en",
  "is_active": true
}
```

**Response:** 201 Created (User object)

**Errors:** 400 (password validation), 409 (duplicate username/email)

---

### GET /api/v1/users

List all users with pagination.

**Description:** Returns paginated user list.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | int | `50` | Max results (max 100) |
| `offset` | int | `0` | Pagination offset |

**Response:** 200 OK

```json
{
  "users": [...],
  "total_count": 150,
  "limit": 50,
  "offset": 0
}
```

---

### GET /api/v1/users/:id

Get a specific user by ID.

**Description:** Returns user profile. Users can view their own profile; viewing others requires `user.view` permission.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/users/:id

Update a user's information.

**Description:** Updates user fields. Users can update own profile; changing `role_id` or `is_active` requires `user.manage`.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body (all fields optional):**

```json
{
  "username": "newusername",
  "email": "newemail@example.com",
  "first_name": "John",
  "last_name": "Doe",
  "display_name": "John D.",
  "avatar_url": "https://example.com/avatar.jpg",
  "time_zone": "America/New_York",
  "language": "en",
  "role_id": 2,
  "is_active": true,
  "settings": {"theme": "dark"}
}
```

---

### DELETE /api/v1/users/:id

Delete a user account.

**Description:** Permanently removes a user. Cannot delete own account.
**Auth:** Required (`user.delete`)
**Rate Limit:** Standard (100/min)

**Response:** 204 No Content

**Errors:** 400 (cannot delete own account)

---

### POST /api/v1/users/:id/reset-password

Reset a user's password (admin operation).

**Description:** Admin-level forced password reset.
**Auth:** Required (`user.manage`)
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "new_password": "newSecurePassword123"
}
```

---

### POST /api/v1/users/:id/lock

Lock a user account until a specified time.

**Description:** Temporarily disables a user account.
**Auth:** Required (`user.manage`)
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "lock_until": "2024-02-01T00:00:00Z"
}
```

---

### POST /api/v1/users/:id/unlock

Unlock a locked user account.

**Description:** Re-enables a previously locked account.
**Auth:** Required (`user.manage`)
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "message": "Account unlocked successfully"
}
```

---

## Role Management

All role management endpoints require `system.admin` permission.

### POST /api/v1/roles

Create a new role.

**Description:** Defines a new role with a set of permissions.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "name": "editor",
  "description": "Content editor with media management access",
  "permissions": ["media.view", "media.edit", "media.upload"]
}
```

**Response:** 201 Created (Role object)

---

### GET /api/v1/roles

List all roles.

**Description:** Returns all defined roles with their permissions.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Response:** 200 OK (array of Role objects)

---

### GET /api/v1/roles/:id

Get a specific role by ID.

**Description:** Returns role details including permissions.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/roles/:id

Update a role. System roles cannot be modified.

**Description:** Modifies role name, description, or permissions.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "name": "editor",
  "description": "Updated description",
  "permissions": ["media.view", "media.edit", "media.upload", "media.delete"]
}
```

---

### DELETE /api/v1/roles/:id

Delete a role. System roles and roles assigned to users cannot be deleted.

**Description:** Removes a custom role definition.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Response:** 204 No Content

---

### GET /api/v1/roles/permissions

Get the complete permission catalog organized by category.

**Description:** Lists all available permissions that can be assigned to roles.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "user_management": {
    "create_user": "user.create",
    "view_user": "user.view",
    "edit_user": "user.update",
    "delete_user": "user.delete",
    "manage_users": "user.manage"
  },
  "media_management": {
    "view_media": "media.view",
    "upload_media": "media.upload",
    "edit_media": "media.edit",
    "delete_media": "media.delete"
  },
  "share_management": {
    "view_shares": "share.view",
    "create_shares": "share.create",
    "edit_shares": "share.edit",
    "delete_shares": "share.delete"
  },
  "system": {
    "system_admin": "system.admin",
    "view_analytics": "analytics.view",
    "export_data": "analytics.export",
    "manage_settings": "system.configure"
  }
}
```

---

## Configuration

### GET /api/v1/configuration

Get the current system configuration schema.

**Description:** Returns the configuration schema and current values.
**Auth:** Required (`system.configure`)
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/configuration/test

Test a configuration without applying it.

**Description:** Validates a configuration object and reports errors/warnings.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Request Body:** A Configuration object.

**Response:** 200 OK

```json
{
  "is_valid": true,
  "errors": [],
  "warnings": ["SMTP not configured"]
}
```

---

### GET /api/v1/configuration/status

Get system component health status.

**Description:** Returns health status of all system components.
**Auth:** Required (`system.configure`)
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "status": "healthy",
  "version": "3.0.0",
  "uptime": "24h 30m",
  "components": {
    "database": "healthy",
    "storage": "healthy",
    "authentication": "healthy",
    "media_conversion": "healthy",
    "sync": "healthy"
  }
}
```

---

### GET /api/v1/configuration/wizard/step/:step_id

Get a specific setup wizard step definition.

**Description:** Returns the step configuration including fields, validation rules, and defaults.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/configuration/wizard/step/:step_id/validate

Validate data for a specific wizard step.

**Description:** Validates user input for a wizard step without saving.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/configuration/wizard/step/:step_id/save

Save progress for a specific wizard step.

**Description:** Persists the user's input for a wizard step.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/configuration/wizard/progress

Get the current wizard progress for the authenticated user.

**Description:** Returns which steps are completed and the current step.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/configuration/wizard/complete

Complete the setup wizard and apply the configuration.

**Description:** Finalizes wizard setup, applying all saved step data as the active configuration.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

## Error Reporting

### POST /api/v1/errors/report

Submit an error report.

**Description:** Logs an application error with stack trace and context.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "level": "error",
  "message": "Failed to process media file",
  "error_code": "MEDIA_PROCESS_ERROR",
  "component": "media_processor",
  "stack_trace": "goroutine 1 [running]:...",
  "context": {"file_id": 123, "operation": "thumbnail_generation"},
  "user_agent": "Mozilla/5.0...",
  "url": "/api/v1/media/123/thumbnail"
}
```

**Response:** 200 OK (ErrorReport object)

---

### POST /api/v1/errors/crash

Submit a crash report.

**Description:** Logs a crash event with signal and stack trace.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "signal": "SIGSEGV",
  "message": "Segmentation fault in media decoder",
  "stack_trace": "...",
  "context": {"media_id": 42}
}
```

---

### GET /api/v1/errors/reports

List error reports with filtering.

**Description:** Returns paginated error reports with optional filters.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `level` | string | Filter: `debug`, `info`, `warning`, `error`, `fatal` |
| `component` | string | Filter by component |
| `status` | string | Filter: `new`, `in_progress`, `resolved`, `ignored` |
| `start_date` | string | Start date (YYYY-MM-DD) |
| `end_date` | string | End date (YYYY-MM-DD) |
| `limit` | int | Max results |
| `offset` | int | Pagination offset |

---

### GET /api/v1/errors/reports/:id

Get a specific error report.

**Description:** Returns full details of an error report.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/errors/reports/:id/status

Update error report status.

**Description:** Changes the triage status of an error report.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "status": "resolved"
}
```

---

### GET /api/v1/errors/crashes

List crash reports with filtering.

**Description:** Returns paginated crash reports (same parameters as error reports, with `signal` instead of `level`/`component`).
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/errors/crashes/:id

Get a specific crash report.

**Description:** Returns full details of a crash report.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/errors/crashes/:id/status

Update crash report status.

**Description:** Changes the triage status of a crash report.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "status": "resolved"
}
```

---

### GET /api/v1/errors/statistics

Get error reporting statistics.

**Description:** Returns aggregate error counts by level, component, and resolution status.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "total_errors": 150,
  "errors_by_level": {"error": 100, "warning": 40, "fatal": 10},
  "errors_by_component": {"media_processor": 80, "auth": 30, "storage": 40},
  "recent_errors": 15,
  "resolved_errors": 120,
  "avg_resolution_time": 3600.5
}
```

---

### GET /api/v1/errors/crash-statistics

Get crash reporting statistics.

**Description:** Returns aggregate crash counts by signal and resolution status.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/errors/health

Get system health based on error and crash data.

**Description:** Evaluates system health by analyzing recent error and crash trends.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

---

## Log Management

All log management endpoints require `system.admin` permission.

### POST /api/v1/logs/collect

Create a new log collection.

**Description:** Initiates log collection from specified components and time range.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "name": "Debug Session 2024-01-20",
  "description": "Investigating media processing issue",
  "components": ["api", "media_processor", "storage"],
  "log_level": "debug",
  "start_time": "2024-01-20T00:00:00Z",
  "end_time": "2024-01-20T23:59:59Z",
  "filters": {"include_stack_traces": true}
}
```

---

### GET /api/v1/logs/collections

List log collections.

**Description:** Returns paginated list of log collections.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default |
|---|---|---|
| `limit` | int | `20` |
| `offset` | int | `0` |

---

### GET /api/v1/logs/collections/:id

Get a specific log collection.

**Description:** Returns metadata for a log collection.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/logs/collections/:id/entries

Get log entries for a collection.

**Description:** Returns individual log entries with filtering.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `level` | string | Filter by level |
| `component` | string | Filter by component |
| `search` | string | Full-text search |
| `start_time` | string | ISO 8601 start time |
| `end_time` | string | ISO 8601 end time |
| `limit` | int | Max results |
| `offset` | int | Pagination offset |

---

### POST /api/v1/logs/collections/:id/export

Export log collection data.

**Description:** Exports a log collection in the specified format.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Options |
|---|---|---|---|
| `format` | string | `json` | `json`, `csv`, `txt`, `zip` |

---

### GET /api/v1/logs/collections/:id/analyze

Analyze a log collection for patterns and insights.

**Description:** Runs pattern analysis on log entries and returns insights.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "collection_id": 5,
  "total_entries": 5000,
  "entries_by_level": {"error": 100, "warning": 500, "info": 3000, "debug": 1400},
  "entries_by_component": {"api": 2000, "storage": 1500, "media_processor": 1500},
  "error_patterns": {"connection_timeout": 45, "file_not_found": 30},
  "time_range": {"start": "2024-01-20T00:00:00Z", "end": "2024-01-20T23:59:59Z"},
  "insights": ["Error rate increased 40% between 14:00-16:00", "Storage component shows timeout pattern"]
}
```

---

### POST /api/v1/logs/share

Create a shareable link for a log collection.

**Description:** Generates a time-limited sharing token for a log collection.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "collection_id": 5,
  "share_type": "private",
  "expires_at": "2024-02-01T00:00:00Z",
  "permissions": ["read"],
  "recipients": ["dev@example.com"]
}
```

---

### GET /api/v1/logs/share/:token

Access a shared log collection via share token.

**Description:** Retrieves log collection data using a share token.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

---

### DELETE /api/v1/logs/share/:id

Revoke a log share.

**Description:** Invalidates a sharing token.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/logs/stream

Stream live logs via Server-Sent Events (SSE).

**Description:** Opens a persistent SSE connection for real-time log streaming.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Content-Type:** `text/event-stream`

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `level` | string | Filter by level |
| `component` | string | Filter by component |
| `search` | string | Search term |

**SSE Format:**

```
data: {"id":1,"timestamp":"2024-01-20T10:00:00Z","level":"error","component":"api","message":"Request timeout","context":{}}

data: {"id":2,"timestamp":"2024-01-20T10:00:01Z","level":"info","component":"storage","message":"File scan completed","context":{"files":500}}
```

---

### GET /api/v1/logs/statistics

Get log management statistics.

**Description:** Returns aggregate log collection and entry statistics.
**Auth:** Required (`system.admin`)
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "total_collections": 25,
  "total_entries": 150000,
  "active_shares": 3,
  "collections_by_status": {"completed": 20, "in_progress": 3, "failed": 2},
  "recent_collections": 5
}
```

---

## Collections

Media collection endpoints for organizing media items into user-defined collections.

### GET /api/v1/collections

List all media collections.

**Description:** Returns user's media collections.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "collections": [
    {
      "id": 1,
      "name": "Sci-Fi Favorites",
      "description": "Best sci-fi movies",
      "item_count": 25,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

---

### POST /api/v1/collections

Create a new media collection.

**Description:** Creates an empty collection for the user.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "name": "Sci-Fi Favorites",
  "description": "Best sci-fi movies",
  "is_public": false
}
```

---

### GET /api/v1/collections/:id

Get a specific collection with its items.

**Description:** Returns collection details and contained media items.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/collections/:id

Update a collection's name or description.

**Description:** Modifies collection metadata.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### DELETE /api/v1/collections/:id

Delete a collection.

**Description:** Removes the collection (does not delete the media items).
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

## Assets (Authenticated)

Authenticated asset management endpoints.

### POST /api/v1/assets/request

Request an asset to be resolved and cached.

**Description:** Triggers async asset resolution (cover art, thumbnails) via the asset pipeline.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "entity_type": "movie",
  "entity_id": "42",
  "asset_type": "cover"
}
```

**Response:** 202 Accepted

```json
{
  "asset_id": "abc123",
  "status": "resolving"
}
```

---

### GET /api/v1/assets/by-entity/:type/:id

Get the asset for a specific entity type and ID.

**Description:** Returns the resolved asset URL/data for an entity.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `type` | string | Entity type (e.g., `movie`, `tv_show`, `music_album`) |
| `id` | int | Entity ID |

**Response:** 200 OK (asset data or redirect to asset URL)

---

## Media Entities

Structured media entity browsing. All endpoints are cached for 5 minutes.

### GET /api/v1/entities

List media entities with filtering and pagination.

**Description:** Returns paginated media entities with optional type and search filters.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `type` | string | - | Media type filter |
| `query` | string | - | Search term |
| `parent_id` | int | - | Filter by parent entity |
| `limit` | int | `50` | Max results |
| `offset` | int | `0` | Pagination offset |

**Response:** 200 OK

```json
{
  "entities": [...],
  "total": 150,
  "limit": 50,
  "offset": 0
}
```

---

### GET /api/v1/entities/types

Get all media entity types.

**Description:** Returns the list of media types with counts.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "types": [
    {"name": "movie", "label": "Movie", "count": 500},
    {"name": "tv_show", "label": "TV Show", "count": 150},
    {"name": "tv_season", "label": "TV Season", "count": 800},
    {"name": "tv_episode", "label": "TV Episode", "count": 5000},
    {"name": "music_artist", "label": "Music Artist", "count": 200},
    {"name": "music_album", "label": "Music Album", "count": 1200},
    {"name": "song", "label": "Song", "count": 12000},
    {"name": "game", "label": "Game", "count": 100},
    {"name": "software", "label": "Software", "count": 50},
    {"name": "book", "label": "Book", "count": 300},
    {"name": "comic", "label": "Comic", "count": 75}
  ]
}
```

---

### GET /api/v1/entities/stats

Get aggregate entity statistics.

**Description:** Returns counts and totals grouped by entity type.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/entities/duplicates

List groups of duplicate media entities.

**Description:** Finds entities that may be duplicates based on title, type, and year.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/entities/browse/:type

Browse entities by media type.

**Description:** Returns entities of a specific type with type-appropriate sorting and filtering.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `type` | string | Media type: `movie`, `tv_show`, `music_artist`, `game`, `software`, `book`, `comic`, etc. |

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `sort_by` | string | `title` | Sort field |
| `sort_order` | string | `asc` | Sort direction |
| `limit` | int | `50` | Max results |
| `offset` | int | `0` | Pagination offset |

---

### GET /api/v1/entities/:id

Get a specific media entity by ID.

**Description:** Returns full entity details including metadata and hierarchy info.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "id": 42,
  "title": "The Matrix",
  "media_type": "movie",
  "year": 1999,
  "description": "A computer hacker learns about the true nature of reality.",
  "parent_id": null,
  "cover_url": "/api/v1/cover/42",
  "metadata": {},
  "file_count": 3,
  "total_size": 8589934592,
  "created_at": "2024-01-15T10:30:00Z"
}
```

---

### GET /api/v1/entities/:id/children

Get child entities of a parent entity (e.g., seasons of a TV show, albums of an artist).

**Description:** Returns the entity hierarchy children.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "parent_id": 42,
  "children": [
    {"id": 43, "title": "Season 1", "media_type": "tv_season", "year": 1999},
    {"id": 44, "title": "Season 2", "media_type": "tv_season", "year": 2000}
  ]
}
```

---

### GET /api/v1/entities/:id/files

Get files associated with a media entity.

**Description:** Returns the file records linked to this entity via the media_files junction table.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/entities/:id/metadata

Get external metadata for a media entity.

**Description:** Returns metadata from external providers (TMDB, OMDB, MusicBrainz, OpenLibrary).
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/entities/:id/duplicates

Get potential duplicates of a specific entity.

**Description:** Finds entities that may be duplicates of this one.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/entities/:id/stream

Stream the primary file of a media entity.

**Description:** Proxies the main media file for playback. Supports range requests.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/entities/:id/download

Download the primary file of a media entity.

**Description:** Downloads the main media file as an attachment.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/entities/:id/install-info

Get installation information for a software or game entity.

**Description:** Returns install instructions, requirements, and file details for software/game entities.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/entities/:id/metadata/refresh

Refresh external metadata for a specific entity.

**Description:** Re-fetches metadata from all configured providers.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/entities/:id/user-metadata

Update user-specific metadata for an entity (e.g., personal rating, notes).

**Description:** Saves user-provided metadata that augments the entity.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "rating": 9,
  "notes": "One of my all-time favorites",
  "tags": ["sci-fi", "action", "classic"]
}
```

---

### POST /api/v1/entities/:id/user-metadata

Alias for PUT /api/v1/entities/:id/user-metadata. Same behavior.

**Description:** Alternative method for updating user metadata.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/entities/enrich

Trigger bulk metadata enrichment for all entities.

**Description:** Starts background enrichment from external providers for entities missing metadata.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "message": "Enrichment started",
  "entities_queued": 150
}
```

---

## Analytics

### POST /api/v1/analytics/access

Log a media access event.

**Description:** Records that a user accessed/viewed a media item.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "media_id": 42,
  "action": "play",
  "duration_seconds": 7200
}
```

---

### POST /api/v1/analytics/event

Log a generic analytics event.

**Description:** Records a custom analytics event with arbitrary context.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "event_type": "search",
  "context": {"query": "matrix", "results_count": 15}
}
```

---

### GET /api/v1/analytics/user/:user_id

Get analytics data for a specific user.

**Description:** Returns user engagement metrics and activity history.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `user_id` | int | User ID |

---

### GET /api/v1/analytics/system

Get system-wide analytics.

**Description:** Returns aggregate system usage metrics.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/analytics/media/:media_id

Get analytics data for a specific media item.

**Description:** Returns access counts, popularity metrics, and engagement data for a media item.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `media_id` | int | Media item ID |

---

### POST /api/v1/analytics/reports

Create an analytics report.

**Description:** Generates a custom analytics report based on specified criteria.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

## Reports

### GET /api/v1/reports/usage

Get a usage report.

**Description:** Returns system usage metrics over a time period.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `start_date` | string | - | Start date (YYYY-MM-DD) |
| `end_date` | string | - | End date (YYYY-MM-DD) |
| `granularity` | string | `daily` | `hourly`, `daily`, `weekly`, `monthly` |

---

### GET /api/v1/reports/performance

Get a performance report.

**Description:** Returns API performance metrics (response times, error rates, throughput).
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

## Favorites

### GET /api/v1/favorites

List the current user's favorites.

**Description:** Returns all entities the user has marked as favorite.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "favorites": [
    {
      "entity_type": "movie",
      "entity_id": 42,
      "title": "The Matrix",
      "added_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

---

### POST /api/v1/favorites

Add an entity to favorites.

**Description:** Marks an entity as a favorite for the current user.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "entity_type": "movie",
  "entity_id": 42
}
```

---

### DELETE /api/v1/favorites/:entity_type/:entity_id

Remove an entity from favorites.

**Description:** Removes the favorite mark from an entity.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `entity_type` | string | Entity type (e.g., `movie`, `tv_show`) |
| `entity_id` | int | Entity ID |

---

### GET /api/v1/favorites/check/:entity_type/:entity_id

Check if an entity is in the user's favorites.

**Description:** Returns whether the entity is favorited by the current user.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `entity_type` | string | Entity type |
| `entity_id` | int | Entity ID |

**Response:** 200 OK

```json
{
  "is_favorite": true
}
```

---

## Playlists

### GET /api/v1/playlists

List the current user's playlists.

**Description:** Returns all playlists owned by the user.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/playlists

Create a new playlist.

**Description:** Creates an empty playlist.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "name": "Road Trip Mix",
  "description": "Music for long drives",
  "is_public": false
}
```

---

### GET /api/v1/playlists/:id

Get a specific playlist with its items.

**Description:** Returns playlist details and ordered items.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/playlists/:id

Update a playlist's metadata.

**Description:** Modifies playlist name, description, or visibility.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### DELETE /api/v1/playlists/:id

Delete a playlist.

**Description:** Removes the playlist (does not delete the media items).
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/playlists/:id/items

Add an item to a playlist.

**Description:** Appends a media entity to the playlist.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "entity_type": "song",
  "entity_id": 123,
  "position": 5
}
```

---

### DELETE /api/v1/playlists/:id/items/:item_id

Remove an item from a playlist.

**Description:** Removes a specific item from the playlist.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

## Browse

Directory browsing and file information endpoints using the database file repository.

### GET /api/v1/browse/roots

Get all storage roots available for browsing.

**Description:** Lists storage roots with browse-relevant metadata.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/browse/directory/{path}

Browse a directory and list its contents from the database.

**Description:** Returns files and subdirectories at the specified path.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/browse/file-info/{path}

Get detailed file information from the database.

**Description:** Returns metadata for a specific file path.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/browse/directory-sizes/{path}

Get subdirectory sizes for a given path.

**Description:** Returns the total size of each subdirectory.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/browse/duplicates/{path}

Get duplicate files within a directory.

**Description:** Finds duplicates under the specified directory path.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

## Sync

Remote synchronization endpoints (WebDAV, S3, GCS, local).

### POST /api/v1/sync/endpoints

Create a sync endpoint configuration.

**Description:** Registers a remote sync destination.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "name": "Cloud Backup",
  "type": "s3",
  "config": {
    "bucket": "my-backup-bucket",
    "region": "us-east-1",
    "access_key": "...",
    "secret_key": "..."
  },
  "sync_direction": "push",
  "filters": {"extensions": [".mkv", ".mp4"]}
}
```

---

### GET /api/v1/sync/endpoints

List the user's sync endpoints.

**Description:** Returns all configured sync destinations for the user.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/sync/endpoints/:id

Get a specific sync endpoint.

**Description:** Returns endpoint configuration details.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### PUT /api/v1/sync/endpoints/:id

Update a sync endpoint configuration.

**Description:** Modifies sync endpoint settings.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### DELETE /api/v1/sync/endpoints/:id

Delete a sync endpoint.

**Description:** Removes a sync destination configuration.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/sync/endpoints/:id/sync

Start a sync operation on an endpoint.

**Description:** Triggers immediate synchronization.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 202 Accepted

```json
{
  "session_id": "sync-abc123",
  "status": "started"
}
```

---

### GET /api/v1/sync/sessions

List the user's sync sessions.

**Description:** Returns sync operation history for the user.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### GET /api/v1/sync/sessions/:id

Get a specific sync session.

**Description:** Returns detailed sync session status and progress.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/sync/schedules

Create a sync schedule.

**Description:** Sets up automatic recurring synchronization.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Request Body:**

```json
{
  "endpoint_id": 1,
  "cron": "0 2 * * *",
  "enabled": true
}
```

---

### GET /api/v1/sync/statistics

Get sync statistics.

**Description:** Returns aggregate sync metrics (bytes transferred, sessions, errors).
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/sync/cleanup

Clean up old sync sessions.

**Description:** Removes completed sync session records older than a threshold.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

## Challenges

Challenge system endpoints for running validation and testing challenges.

### GET /api/v1/challenges

List all registered challenges.

**Description:** Returns the full challenge catalog with status.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "challenges": [
    {
      "id": "CH-001",
      "name": "Health Check",
      "category": "infrastructure",
      "description": "Verify API health endpoint returns 200",
      "status": "passed"
    }
  ],
  "total": 492
}
```

---

### GET /api/v1/challenges/:id

Get a specific challenge by ID.

**Description:** Returns challenge details and last run result.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

### POST /api/v1/challenges/:id/run

Run a specific challenge.

**Description:** Executes a single challenge and returns the result.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "id": "CH-001",
  "status": "passed",
  "duration_ms": 150,
  "assertions": [
    {"name": "health_endpoint_returns_200", "passed": true}
  ]
}
```

---

### POST /api/v1/challenges/run

Run all challenges (synchronous, blocking).

**Description:** Executes the entire challenge suite sequentially. WARNING: This is blocking -- no other challenge can run until it finishes. Can take 25+ minutes for NAS scans.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Response:** 200 OK

```json
{
  "total": 492,
  "passed": 490,
  "failed": 2,
  "skipped": 0,
  "duration_ms": 1500000,
  "results": [...]
}
```

---

### POST /api/v1/challenges/run/category/:category

Run all challenges in a specific category.

**Description:** Executes challenges filtered by category.
**Auth:** Required
**Rate Limit:** Standard (100/min)

**Path Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `category` | string | Challenge category (e.g., `infrastructure`, `browsing`, `media`, `security`) |

---

### GET /api/v1/challenges/results

Get the results from the most recent challenge run.

**Description:** Returns cached results from the last challenge execution.
**Auth:** Required
**Rate Limit:** Standard (100/min)

---

## Global Middleware

All requests pass through these middleware layers:

| Middleware | Description |
|---|---|
| Security Headers | HSTS, X-Content-Type-Options, X-Frame-Options, etc. |
| Concurrency Limiter | Max 100 concurrent requests |
| Request Timeout | 60-second request timeout |
| CORS | Cross-Origin Resource Sharing headers |
| Prometheus Metrics | Request duration and count tracking |
| Logger | Structured request logging (zap) |
| Error Handler | Consistent error response formatting |
| Request ID | Unique `X-Request-ID` header per request |
| Input Validation | Request body sanitization and validation |
| Compression | Brotli (primary) with gzip fallback |
| JWT Auth | Token validation on `/api/v1/*` routes (except auth/public) |
| Rate Limiting | Per-user request throttling |
| Cache Headers | Cache-Control headers on static/discovery endpoints |

---

## Error Handling

All error responses follow a consistent format:

```json
{
  "error": "Human-readable error message"
}
```

Or the structured format used by subtitle and recommendation handlers:

```json
{
  "success": false,
  "error": "Human-readable error message",
  "code": "MACHINE_READABLE_CODE"
}
```

### HTTP Status Codes

| Code | Meaning |
|---|---|
| 200 | Success |
| 201 | Created (user registration, resource creation) |
| 202 | Accepted (async operation queued) |
| 204 | No Content (successful deletion) |
| 400 | Bad Request (validation error, invalid parameters) |
| 401 | Unauthorized (missing or invalid JWT) |
| 403 | Forbidden (insufficient permissions or domain not allowed) |
| 404 | Not Found (resource does not exist) |
| 409 | Conflict (duplicate resource) |
| 429 | Too Many Requests (rate limited) |
| 500 | Internal Server Error |
| 502 | Bad Gateway (upstream fetch failed, e.g., image proxy) |
| 503 | Service Unavailable (unhealthy) |

---

## Rate Limiting

Rate limiting is applied per-user based on the JWT token. Two tiers are used:

| Endpoint Group | Limit | Description |
|---|---|---|
| `POST /api/v1/auth/login` | 5 requests/minute | Brute-force protection |
| `POST /api/v1/auth/register` | 5 requests/minute | Brute-force protection |
| All other `/api/v1/*` | 100 requests/minute | Standard rate limit |

When rate limited, the server returns `429 Too Many Requests`.

Optional Redis-based distributed rate limiting is available when a Redis instance is configured. Supports both fixed-window and sliding-window algorithms.
