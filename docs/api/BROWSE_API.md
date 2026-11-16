# Browse API Reference

Base path: `/api/v1`

All browse endpoints return JSON with `{ "success": true, "data": ... }` envelope.

## Endpoints

### GET /api/v1/storage-roots

List all configured storage roots. Used by the scan handler for root management.

**Response 200:**

```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "Media",
      "protocol": "smb",
      "host": "nas.local",
      "port": 445,
      "path": "/media",
      "enabled": true,
      "max_depth": 10,
      "enable_duplicate_detection": true
    }
  ]
}
```

Storage root protocols: `smb`, `ftp`, `nfs`, `webdav`, `local`.

### GET /api/v1/storage-roots/:id/status

Get scan status for a specific storage root.

### GET /api/v1/browse/roots

List all storage roots (browse handler variant). Returns the same data as `/storage-roots`.

### GET /api/v1/browse/directory/*path

Browse directory contents within a storage root.

The `*path` segment encodes `/:storage_root/optional/subpath`. The first path component is the storage root name.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `path` | query string | `/` | Directory path within the storage root |
| `page` | int | 1 | Page number |
| `limit` | int | 100 | Items per page (1-1000) |
| `sort_by` | string | name | `name`, `size`, `modified_at`, `created_at`, `path`, `extension` |
| `sort_order` | string | asc | `asc` or `desc` |

**Response 200:** Paginated `SearchResult` with files and directories.

```json
{
  "success": true,
  "data": {
    "files": [
      {
        "id": 42,
        "name": "Movies",
        "path": "/Movies",
        "is_directory": true,
        "size": 0
      },
      {
        "id": 43,
        "name": "photo.jpg",
        "path": "/photo.jpg",
        "is_directory": false,
        "size": 4194304
      }
    ],
    "total_count": 2,
    "page": 1,
    "limit": 100,
    "total_pages": 1
  }
}
```

**Errors:** 400 (missing storage root), 500 (browse failure).

### GET /api/v1/browse/file-info/*path

Get detailed metadata for a specific file by its path.

**Response 200:**

```json
{
  "success": true,
  "data": {
    "id": 43,
    "name": "photo.jpg",
    "path": "/photo.jpg",
    "size": 4194304,
    "extension": "jpg",
    "mime_type": "image/jpeg",
    "modified_at": "2026-01-15T10:30:00Z"
  }
}
```

**Errors:** 400 (invalid ID), 404 (not found), 500 (failure).

### GET /api/v1/browse/directory-sizes/*path

Get directories sorted by their total file size within a storage root.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `limit` | int | 50 | Items per page (1-500) |
| `ascending` | bool | false | Sort ascending instead of descending |

**Response 200:** Array of `DirectoryInfo` objects with `path`, `total_size`, and `file_count`.

### GET /api/v1/browse/duplicates/*path

Get directories sorted by their number of duplicate files.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `limit` | int | 50 | Items per page (1-500) |
| `ascending` | bool | false | Sort ascending instead of descending |

**Response 200:** Array of `DirectoryInfo` objects.

### GET /api/v1/entities/browse/:type

Browse media entities by type. Returns aggregated media entities (not raw files).

Types: `movie`, `tv_show`, `tv_season`, `tv_episode`, `music_artist`, `music_album`, `song`, `game`, `software`, `book`, `comic`.

## Pagination

Browse endpoints use the same pagination model as search:

- `page` starts at 1, `limit` defaults vary (100 for directories, 50 for sizes/duplicates)
- Size/duplicate limits cap at 500 instead of 1000

## Source

- Handler: `catalog-api/handlers/browse.go`
- Route registration: `catalog-api/main.go` (lines 820-827)
- Model: `catalog-api/models/file.go` (StorageRoot, SearchResult, FileWithMetadata)
