# Sync API Reference

Base path: `/api/v1/sync`

All endpoints require JWT authentication via `Authorization: Bearer <token>` header. Endpoints return JSON with `{ "success": true, "data": ... }` or `{ "success": false, "error": "...", "details": "..." }`.

## Endpoint Management

### POST /api/v1/sync/endpoints

Create a new sync endpoint. The service validates configuration and tests the connection.

**Request body:**

```json
{
  "name": "Backup NAS",
  "type": "webdav",
  "url": "https://nas.example.com/remote.php/webdav",
  "username": "admin",
  "password": "secret",
  "sync_direction": "push",
  "local_path": "/media/movies",
  "remote_path": "/backup/movies",
  "sync_settings": "{\"delete_orphans\": false}"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name for the endpoint |
| `type` | string | Protocol type (webdav, s3, gcs, local) |
| `url` | string | Remote endpoint URL |
| `username` | string | Authentication username |
| `password` | string | Authentication password (never returned in responses) |
| `sync_direction` | string | `push`, `pull`, or `bidirectional` |
| `local_path` | string | Local filesystem path |
| `remote_path` | string | Remote filesystem path |
| `sync_settings` | string | Optional JSON settings |

**Response 201:** Created endpoint object.

**Path confinement:** `local_path` (and `source_directory` / `destination_directory` in `sync_settings`) must lie inside the configured roots - read roots for paths that are only read (`upload`), sync write roots for paths that are written (`download`, `bidirectional`, destinations). Otherwise 400. Existing endpoints are re-checked when a sync runs. See [PATH_CONFINEMENT](../security/PATH_CONFINEMENT.md).

**Errors:** 400 (invalid config), 502 (connection test failed).

### GET /api/v1/sync/endpoints

List all sync endpoints for the authenticated user.

**Response 200:** Array of `SyncEndpoint` objects (password field excluded from JSON).

### GET /api/v1/sync/endpoints/:id

Get a specific endpoint by ID. Returns 403 if the endpoint belongs to another user.

### PUT /api/v1/sync/endpoints/:id

Update an existing endpoint. Only provided fields are updated.

**Request body:** Partial `UpdateSyncEndpointRequest` -- all fields optional.

```json
{
  "name": "Updated Name",
  "sync_direction": "bidirectional",
  "is_active": true
}
```

**Errors:** 400 (invalid), 403 (unauthorized), 404 (not found), 502 (connection test failed).

### DELETE /api/v1/sync/endpoints/:id

Delete a sync endpoint. Returns 403 if the endpoint belongs to another user.

**Response 200:** `{ "success": true, "data": { "message": "Sync endpoint deleted" } }`

## Sync Operations

### POST /api/v1/sync/endpoints/:id/sync

Start a sync operation on the specified endpoint. Creates a new sync session.

**Response 200:** Created `SyncSession` object.

**Errors:** 403 (unauthorized), 404 (not found), 409 (endpoint not active).

### GET /api/v1/sync/sessions

List sync sessions for the authenticated user.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 50 | Max items (1-200) |
| `offset` | int | 0 | Offset for pagination |

**Response 200:** Array of `SyncSession` objects.

### GET /api/v1/sync/sessions/:id

Get a specific sync session. Includes file counts and error messages.

**SyncSession fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | Session ID |
| `endpoint_id` | int | Associated endpoint |
| `status` | string | `running`, `completed`, `failed` |
| `sync_type` | string | Sync type |
| `started_at` | datetime | Start time |
| `completed_at` | datetime | Completion time (null if running) |
| `total_files` | int | Total files to sync |
| `synced_files` | int | Successfully synced |
| `failed_files` | int | Failed to sync |
| `skipped_files` | int | Skipped (unchanged) |
| `error_message` | string | Error details (null if successful) |

## Scheduling

### POST /api/v1/sync/schedules

Schedule recurring sync operations.

**Request body:**

```json
{
  "endpoint_id": 1,
  "frequency": "daily"
}
```

**Response 201:** Created `SyncSchedule` object with `next_run` computed.

## Statistics and Maintenance

### GET /api/v1/sync/statistics

Get sync statistics for the authenticated user over a date range.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `start_date` | string | 30 days ago | Start date (RFC 3339 or YYYY-MM-DD) |
| `end_date` | string | now | End date (RFC 3339 or YYYY-MM-DD) |

### POST /api/v1/sync/cleanup

Remove old completed sync sessions.

**Request body:**

```json
{ "older_than_days": 30 }
```

Defaults to 30 days if not provided or <= 0.

## Source

- Handler: `catalog-api/handlers/sync_handler.go`
- Route registration: `catalog-api/main.go` (lines 831-843)
- Models: `catalog-api/models/user.go` (SyncEndpoint, SyncSession, SyncSchedule, SyncStatistics)
