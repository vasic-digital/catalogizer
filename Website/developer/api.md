---
title: API Reference
description: REST API endpoints, authentication, and WebSocket events for Catalogizer
---

# API Reference

The Catalogizer API is a RESTful service built with Go and the Gin framework. All endpoints are under `/api/v1` unless noted otherwise. The API uses JSON for request and response bodies.

---

## Base URL

```
http://localhost:8080/api/v1
```

In production, use your configured domain with HTTPS.

---

## Authentication

Most endpoints require a valid JWT token. Obtain one by logging in.

### Login

```
POST /api/v1/auth/login
```

```json
{
  "username": "admin",
  "password": "<ADMIN_PASSWORD>"
}
```

**Response:**

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 86400
}
```

### Refresh Token

```
POST /api/v1/auth/refresh
```

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

### Using Tokens

Include the access token in the `Authorization` header:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

---

## Endpoint Groups

### Storage Sources

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/storage-roots` | List all storage sources |
| POST | `/storage-roots` | Add a new storage source |
| GET | `/storage-roots/:id` | Get storage source details |
| PUT | `/storage-roots/:id` | Update a storage source |
| DELETE | `/storage-roots/:id` | Remove a storage source |
| POST | `/storage-roots/:id/test` | Test connection |

### Scanning

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/scans/start` | Start a scan |
| GET | `/scans/status` | Get current scan status |
| POST | `/scans/stop` | Stop a running scan |

### Media Entities

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/entities` | List media entities with filters |
| GET | `/entities/:id` | Get entity details |
| GET | `/entities/:id/children` | Get child entities (seasons, episodes) |
| GET | `/entities/search` | Search entities by query |
| GET | `/entities/types` | List available media types |

### Collections

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/collections` | List collections |
| POST | `/collections` | Create a collection |
| GET | `/collections/:id` | Get collection details with items |
| PUT | `/collections/:id` | Update a collection |
| DELETE | `/collections/:id` | Delete a collection |
| POST | `/collections/:id/items` | Add items to a collection |
| DELETE | `/collections/:id/items/:itemId` | Remove item from collection |

### Favorites

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/favorites` | List user's favorites |
| POST | `/favorites` | Add a favorite |
| DELETE | `/favorites/:id` | Remove a favorite |
| GET | `/favorites/export` | Export favorites (JSON/CSV) |
| POST | `/favorites/import` | Import favorites |

### Subtitles

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/subtitles/search` | Search subtitles for a media item |
| POST | `/subtitles/download` | Download a subtitle |
| POST | `/subtitles/upload` | Upload a subtitle file |
| POST | `/subtitles/translate` | Translate a subtitle |
| GET | `/subtitles/sync-check` | Verify subtitle timing |

### Recommendations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/recommendations` | Get personalized recommendations |
| GET | `/recommendations/similar/:id` | Get similar media items |

### Statistics and Analytics

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/stats` | Catalog statistics summary |
| GET | `/stats/quality` | Quality distribution |
| GET | `/stats/growth` | Growth over time |
| GET | `/analytics/dashboard` | Dashboard analytics data |

### Format Conversion

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/conversion/start` | Start a format conversion |
| GET | `/conversion/status/:id` | Get conversion status |
| GET | `/conversion/queue` | List queued conversions |
| DELETE | `/conversion/:id` | Cancel a conversion |

### User Management (Admin)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/users` | List all users |
| POST | `/users` | Create a user |
| PUT | `/users/:id` | Update a user |
| DELETE | `/users/:id` | Delete a user |
| GET | `/roles` | List available roles |

### Search and Browse

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/search` | Full-text search with advanced filters |
| GET | `/browse/roots` | List storage roots with summary stats |
| GET | `/browse/directory` | Browse directory tree with pagination |

### Backup Management (Admin)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/admin/backup` | Create a database backup |
| GET | `/admin/backups` | List available backups |
| POST | `/admin/backup/restore` | Restore from a backup |

### Password Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/change-password` | Change current user's password |

### Media Quality and Analysis

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/media/stats` | Media statistics with quality breakdown by file extension |
| POST | `/media/:id/analyze` | Trigger quality analysis for a media item |
| POST | `/media/:id/refresh-metadata` | Refresh external metadata |
| GET | `/media/popular` | Popular media sorted by favorites count |
| GET | `/media/recent` | Recently discovered media |
| GET | `/media/search-by-path` | Search media by file path |

### Configuration (Admin)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/configuration` | Get server configuration |
| PUT | `/configuration` | Update server configuration |

### Assets

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/assets/:id` | Serve an asset by ID |
| GET | `/assets/by-entity/:type/:id` | Get cover art for an entity |

### SMB Discovery

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/smb/discover` | Discover SMB shares on the network |
| GET | `/smb/shares` | List shares on a specific server |

### Challenges

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/challenges` | List all challenges |
| POST | `/challenges/run/:id` | Run a specific challenge |
| POST | `/challenges/run-all` | Run all challenges |
| GET | `/challenges/status` | Get challenge run status |
| GET | `/challenges/results` | Get challenge results |

---

## Public Endpoints

These endpoints do not require authentication:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Server health check |
| GET | `/metrics` | Prometheus metrics |
| POST | `/api/v1/auth/login` | Authentication |
| POST | `/api/v1/auth/refresh` | Token refresh |
| POST | `/api/v1/auth/register` | User registration |

---

## WebSocket Events

Connect to `/ws` with a valid JWT token for real-time events.

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?token=<jwt_token>');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data.type, data.payload);
};
```

### Event Types

| Event | Description |
|-------|-------------|
| `scan.started` | A scan has begun |
| `scan.progress` | Scan progress update (file count, percentage) |
| `scan.completed` | A scan has finished |
| `media.new` | New media item detected |
| `media.updated` | Media item metadata updated |
| `source.connected` | Storage source connected |
| `source.disconnected` | Storage source went offline |
| `source.recovered` | Storage source reconnected |
| `conversion.progress` | Format conversion progress |
| `conversion.completed` | Format conversion finished |

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Media item not found"
  }
}
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad request (validation error) |
| 401 | Unauthorized (missing or invalid token) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Not found |
| 429 | Too many requests (rate limited) |
| 500 | Internal server error |

---

## Pagination

List endpoints support pagination with query parameters:

```
GET /api/v1/entities?page=1&per_page=20&sort=title&order=asc
```

Paginated responses include metadata:

```json
{
  "data": [...],
  "total": 1250,
  "page": 1,
  "per_page": 20,
  "total_pages": 63
}
```
