# Catalog API Documentation

## Overview

The Catalog API is a REST service for browsing, searching, and managing SMB file catalogs. It provides comprehensive functionality for file operations, duplicate detection, statistics, and analytics.

## Features

- **Browse**: Navigate through SMB file systems with pagination and sorting
- **Search**: Advanced file search with multiple filters and full-text search
- **Download**: Stream individual files or compressed directory archives
- **Copy**: Copy files between SMB locations or to/from local computer
- **Statistics**: Comprehensive analytics and reporting
- **Authentication**: JWT-based security (optional)
- **Media Recognition**: AI-powered recognition for movies, music, books, games, and software
- **Recommendations**: Similar items discovery with local and external recommendations
- **Deep Linking**: Universal cross-platform linking with UTM tracking and analytics

## API Endpoints

### Browse Endpoints

- `GET /api/browse/roots` - Get all configured SMB roots
- `GET /api/browse/{smb_root}` - Browse directory contents
- `GET /api/browse/file/{id}` - Get detailed file information
- `GET /api/browse/{smb_root}/sizes` - Get directories sorted by size
- `GET /api/browse/{smb_root}/duplicates` - Get directories sorted by duplicate count

### Search Endpoints

- `GET /api/search` - Advanced file search with query parameters
- `GET /api/search/duplicates` - Search for duplicate files
- `POST /api/search/advanced` - Advanced search with JSON body

### Download Endpoints

- `GET /api/download/file/{id}` - Download a specific file
- `GET /api/download/directory/{smb_root}` - Download directory as ZIP
- `GET /api/download/info/{id}` - Get download information

### Copy Endpoints

- `POST /api/copy/local` - Copy files from SMB to local computer
- `POST /api/copy/upload` - Upload files from local computer to SMB

### Statistics Endpoints

- `GET /api/stats/overall` - Overall catalog statistics
- `GET /api/stats/smb/{smb_root}` - SMB root specific statistics
- `GET /api/stats/filetypes` - File type distribution
- `GET /api/stats/sizes` - File size distribution
- `GET /api/stats/duplicates` - Duplicate file statistics
- `GET /api/stats/duplicates/groups` - Top duplicate groups
- `GET /api/stats/access` - File access patterns
- `GET /api/stats/growth` - Storage growth trends
- `GET /api/stats/scans` - Scan operation history

### Media Recognition Endpoints

- `POST /api/v1/media/recognize` - Recognize media file and extract metadata
- `GET /api/v1/media/metadata/{id}` - Get cached metadata for a media file
- `POST /api/v1/media/bulk-recognize` - Batch recognize multiple media files
- `GET /api/v1/media/recognition-status/{job_id}` - Check batch recognition job status

### Recommendations Endpoints

- `GET /api/v1/media/{id}/similar` - Get similar items for a specific media file
- `POST /api/v1/media/similar` - Advanced similar items search with custom filters
- `GET /api/v1/media/{id}/detail-with-similar` - Get media details with similar items and deep links
- `GET /api/v1/recommendations/trends` - Get trending recommendations by media type and period
- `POST /api/v1/recommendations/batch` - Get recommendations for multiple items simultaneously

### Deep Linking Endpoints

- `POST /api/v1/links/generate` - Generate deep links for all platforms
- `POST /api/v1/links/smart` - Generate smart links with automatic platform detection
- `POST /api/v1/links/batch` - Generate deep links for multiple items
- `POST /api/v1/links/track` - Track link click events and analytics
- `GET /api/v1/links/{tracking_id}/analytics` - Get detailed link performance analytics

### Health Check

- `GET /health` - Service health status

## Configuration

Create a `config.json` file with the following structure:

```json
{
  "server": {
    "host": "localhost",
    "port": 8080,
    "read_timeout": 30,
    "write_timeout": 30,
    "idle_timeout": 120,
    "enable_cors": true,
    "enable_https": false
  },
  "database": {
    "path": "./catalog.db",
    "max_open_connections": 25,
    "max_idle_connections": 5,
    "conn_max_lifetime": 300,
    "conn_max_idle_time": 60,
    "enable_wal": true,
    "cache_size": -2000,
    "busy_timeout": 5000
  },
  "auth": {
    "jwt_secret": "your-secret-key",
    "jwt_expiration_hours": 24,
    "enable_auth": false,
    "admin_username": "admin",
    "admin_password": "<your-admin-password>"
  },
  "catalog": {
    "default_page_size": 100,
    "max_page_size": 1000,
    "enable_cache": true,
    "cache_ttl_minutes": 15,
    "max_concurrent_scans": 3,
    "download_chunk_size": 1048576,
    "max_archive_size": 5368709120,
    "allowed_download_types": ["*"],
    "temp_dir": "/tmp/catalog-api"
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout",
    "max_size": 100,
    "max_backups": 3,
    "max_age": 28,
    "compress": true
  },
  "media_recognition": {
    "tmdb_api_key": "your_tmdb_api_key",
    "omdb_api_key": "your_omdb_api_key",
    "lastfm_api_key": "your_lastfm_api_key",
    "musicbrainz_user_agent": "CatalogizerAPI/1.0",
    "igdb_client_id": "your_igdb_client_id",
    "igdb_client_secret": "your_igdb_client_secret",
    "google_books_api_key": "your_google_books_api_key",
    "ocr_space_api_key": "your_ocr_space_api_key",
    "enable_fingerprinting": true,
    "cache_duration_hours": 168,
    "concurrent_workers": 5,
    "timeout_seconds": 30
  },
  "recommendations": {
    "max_local_items": 10,
    "max_external_items": 5,
    "default_similarity_threshold": 0.3,
    "enable_external_recommendations": true,
    "cache_duration_hours": 24,
    "trending_analysis_enabled": true,
    "trending_update_interval_hours": 6,
    "collaborative_filtering_enabled": true,
    "content_based_weight": 0.6,
    "collaborative_weight": 0.4,
    "external_apis": {
      "tmdb": {"enabled": true, "weight": 0.3},
      "lastfm": {"enabled": true, "weight": 0.2},
      "google_books": {"enabled": true, "weight": 0.2},
      "igdb": {"enabled": true, "weight": 0.3}
    }
  },
  "deep_linking": {
    "base_url": "https://catalogizer.app",
    "enable_universal_links": true,
    "enable_qr_codes": true,
    "link_expiration_hours": 24,
    "track_analytics": true,
    "analytics_retention_days": 90,
    "supported_platforms": ["web", "android", "ios", "desktop"],
    "app_configurations": {
      "android": {
        "package_name": "com.catalogizer.app",
        "scheme": "catalogizer",
        "store_url": "https://play.google.com/store/apps/details?id=com.catalogizer.app"
      },
      "ios": {
        "bundle_id": "com.catalogizer.app",
        "scheme": "catalogizer",
        "store_url": "https://apps.apple.com/app/id123456789"
      }
    }
  }
}
```

## Environment Variables

- `CONFIG_PATH` - Path to configuration file (default: `./config.json`)

## Running the Server

```bash
# Build the application
go build -o catalog-api

# Run with default configuration
./catalog-api

# Run with custom configuration
CONFIG_PATH=/path/to/config.json ./catalog-api
```

## Authentication

When authentication is enabled (`auth.enable_auth: true`), include a JWT token in the Authorization header:

```
Authorization: Bearer <jwt-token>
```

## Error Handling

All endpoints return standardized error responses:

```json
{
  "success": false,
  "error": "Error message",
  "details": "Detailed error information (optional)"
}
```

## Success Responses

All endpoints return standardized success responses:

```json
{
  "success": true,
  "data": { /* Response data */ }
}
```

## Pagination

Endpoints supporting pagination use these query parameters:

- `page` - Page number (default: 1)
- `limit` - Items per page (default: 100, max: 1000)

## Sorting

Endpoints supporting sorting use these query parameters:

- `sort_by` - Field to sort by
- `sort_order` - Sort order (`asc` or `desc`)

## Search Filters

The search endpoints support various filters:

- `q` - Text query (searches name and path)
- `path` - Path filter (partial match)
- `name` - Name filter (partial match)
- `extension` - File extension filter
- `file_type` - File type filter
- `mime_type` - MIME type filter
- `smb_roots` - SMB roots filter (comma-separated)
- `min_size` / `max_size` - Size range filters
- `modified_after` / `modified_before` - Date range filters (RFC3339 format)
- `include_deleted` - Include deleted files
- `only_duplicates` - Only show duplicates
- `exclude_duplicates` - Exclude duplicates
- `include_directories` - Include directories

## File Downloads

File downloads support:

- Streaming for large files
- Content-Type detection
- Content-Disposition headers
- Range requests (partial downloads)
- Inline display option

Directory downloads:

- ZIP compression
- Recursive directory inclusion
- Size limits
- Progress tracking

## Copy Operations

Copy operations support:

- SMB to SMB copying
- SMB to local copying
- Local to SMB uploading
- Overwrite control
- Progress tracking
- Error handling

## Statistics and Analytics

The API provides comprehensive statistics including:

- Overall catalog metrics
- Per-SMB root statistics
- File type distribution
- Size distribution
- Duplicate analysis
- Access patterns
- Growth trends
- Scan operation history

## OpenAPI/Swagger Documentation

When running in debug mode, Swagger documentation is available at:

```
http://localhost:8080/swagger/index.html
```

## Database Schema

The API connects to a SQLite database created by the Kotlin cataloging system. The database contains:

- `smb_roots` - SMB connection configurations
- `files` - File and directory records
- `file_metadata` - Extracted metadata
- `duplicate_groups` - Duplicate file groups
- `virtual_paths` - Virtual file system paths
- `scan_history` - Scan operation history

## Security Considerations

- Enable authentication in production environments
- Use HTTPS for secure communication
- Implement proper rate limiting
- Validate all user inputs
- Use strong JWT secrets
- Regularly rotate authentication keys
- Monitor access logs

## Performance Tips

- Use pagination for large result sets
- Enable database WAL mode for better concurrency
- Configure appropriate connection pool sizes
- Use caching for frequently accessed data
- Monitor database performance
- Consider read replicas for high-traffic scenarios

## Troubleshooting

Common issues and solutions:

1. **Database connection errors**: Check database path and permissions
2. **SMB connection failures**: Verify SMB credentials and network connectivity
3. **Large file download timeouts**: Increase timeout settings
4. **Memory issues with large archives**: Reduce max archive size
5. **Authentication failures**: Check JWT secret and token validity

## Development

To set up a development environment:

```bash
# Clone the repository
git clone <repository-url>
cd catalog-api

# Install dependencies
go mod tidy

# Run in development mode
go run main.go
```

## API Versioning

The current API version is v1. All endpoints are prefixed with `/api/`.

Future versions will be available at `/api/v2/`, etc., maintaining backward compatibility.