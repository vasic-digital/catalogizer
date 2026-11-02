# API Reference

The Catalogizer REST API is built with Go and Gin. All endpoints use JSON for requests and responses. Authenticated endpoints require a JWT token in the `Authorization: Bearer <token>` header.

---

## Base URL

```
http://localhost:8080/api/v1
```

In production, use your configured domain with HTTPS (HTTP/3 with Brotli compression).

---

## Authentication

Obtain a JWT token by sending credentials to the login endpoint:

```
POST /api/v1/auth/login
Body: { "username": "admin", "password": "<your-admin-password>" }
Response: { "access_token": "...", "refresh_token": "...", "expires_in": 86400 }
```

Include the token in subsequent requests:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

---

## Endpoint Groups

### Health and Metrics

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Server health check (no auth required) |
| GET | `/metrics` | Prometheus metrics (no auth required) |

### Auth

| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/login` | Authenticate and obtain tokens |
| POST | `/auth/refresh` | Refresh an expired access token |
| POST | `/auth/register` | Register a new user account |

### Catalog and Browse

| Method | Path | Description |
|--------|------|-------------|
| GET | `/catalog` | List cataloged files with filters |
| GET | `/catalog/:id` | Get file details |
| GET | `/browse` | Browse storage root directory tree |

### Search

| Method | Path | Description |
|--------|------|-------------|
| GET | `/search` | Full-text search across catalog |
| GET | `/search/advanced` | Search with type, quality, and date filters |

### Download and Copy

| Method | Path | Description |
|--------|------|-------------|
| GET | `/download/:id` | Download a file by ID |
| POST | `/copy` | Copy files between storage roots |

### Media Entities

| Method | Path | Description |
|--------|------|-------------|
| GET | `/entities` | List media entities with pagination and filters |
| GET | `/entities/:id` | Get entity details with metadata |
| GET | `/entities/:id/children` | Get child entities (seasons, episodes, tracks) |

### Collections

| Method | Path | Description |
|--------|------|-------------|
| GET | `/collections` | List user collections |
| POST | `/collections` | Create a new collection |
| POST | `/collections/:id/items` | Add items to a collection |

### Assets

| Method | Path | Description |
|--------|------|-------------|
| GET | `/assets/:id` | Serve an asset by ID |
| GET | `/assets/by-entity/:type/:id` | Get cover art for a media entity |

### Storage Roots

| Method | Path | Description |
|--------|------|-------------|
| GET | `/storage-roots` | List all configured storage sources |
| POST | `/storage-roots` | Add a new storage source (SMB, FTP, NFS, WebDAV, local) |
| POST | `/storage-roots/:id/test` | Test connection to a storage source |

### Scans

| Method | Path | Description |
|--------|------|-------------|
| POST | `/scans/start` | Start scanning a storage root |
| GET | `/scans/status` | Get current scan progress |
| POST | `/scans/stop` | Stop a running scan |

### Statistics

| Method | Path | Description |
|--------|------|-------------|
| GET | `/stats` | Catalog summary statistics |
| GET | `/stats/quality` | Quality distribution breakdown |
| GET | `/analytics/dashboard` | Dashboard analytics data |

### SMB Discovery

| Method | Path | Description |
|--------|------|-------------|
| GET | `/smb/discover` | Discover SMB shares on the local network |
| GET | `/smb/shares` | List shares on a specific SMB server |

### Sync

| Method | Path | Description |
|--------|------|-------------|
| POST | `/sync/start` | Start syncing between storage roots |
| GET | `/sync/status` | Get sync operation status |

### Recommendations

| Method | Path | Description |
|--------|------|-------------|
| GET | `/recommendations` | Get personalized media recommendations |
| GET | `/recommendations/similar/:id` | Get items similar to a given entity |

### Subtitles

| Method | Path | Description |
|--------|------|-------------|
| GET | `/subtitles/search` | Search subtitles for a media item |
| POST | `/subtitles/download` | Download a subtitle file |
| POST | `/subtitles/translate` | Translate a subtitle to another language |

### Format Conversion

| Method | Path | Description |
|--------|------|-------------|
| POST | `/conversion/start` | Start a media format conversion |
| GET | `/conversion/status/:id` | Get conversion progress |
| GET | `/conversion/queue` | List queued conversions |

### Users and Roles (Admin)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/users` | List all users |
| POST | `/users` | Create a new user |
| GET | `/roles` | List available roles |

### Configuration (Admin)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/configuration` | Get current server configuration |
| PUT | `/configuration` | Update server configuration |

### Challenges

| Method | Path | Description |
|--------|------|-------------|
| GET | `/challenges` | List all registered challenges |
| POST | `/challenges/run/:id` | Run a specific challenge |
| GET | `/challenges/results` | Get challenge execution results |

### WebSocket

Connect to `/ws?token=<jwt_token>` for real-time events including scan progress, new media detection, and storage source status changes. See the [Monitoring Guide](/docs/developer-guide/monitoring) for event types.

---

## Common Response Format

Successful responses return data directly or wrapped in a data field:

```json
{
  "data": [...],
  "total": 150,
  "page": 1,
  "per_page": 20,
  "total_pages": 8
}
```

Error responses use a consistent structure:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Media item not found",
    "details": "No entity with ID 42 exists"
  }
}
```

---

## Pagination

List endpoints accept pagination query parameters:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | 1 | Page number (1-based) |
| `per_page` | 20 | Items per page (max 100) |
| `sort` | varies | Sort field (e.g., `title`, `created_at`) |
| `order` | `asc` | Sort direction (`asc` or `desc`) |

---

## OpenAPI Specification

The full machine-readable API specification is available at [`docs/api/openapi.yaml`](https://github.com/vasic-digital/Catalogizer/blob/main/docs/api/openapi.yaml). Use it with Swagger UI, Postman, or any OpenAPI-compatible tool to explore and test endpoints interactively.
