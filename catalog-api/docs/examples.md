# API Examples

This document provides practical examples of using the Catalog API.

## Authentication

### Generate JWT Token (if authentication is enabled)

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "<your-admin-password>"}'
```

Response:
```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2024-01-02T15:04:05Z"
  }
}
```

### Using JWT Token

```bash
# Set the token as environment variable
export JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# Use in requests
curl -H "Authorization: Bearer $JWT_TOKEN" \
  http://localhost:8080/api/browse/roots
```

## Browse Examples

### Get All SMB Roots

```bash
curl http://localhost:8080/api/browse/roots
```

Response:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "name": "office-server",
      "host": "192.168.1.100",
      "port": 445,
      "share": "shared",
      "username": "user",
      "enabled": true,
      "last_scan_at": "2024-01-01T12:00:00Z"
    }
  ]
}
```

### Browse Directory Contents

```bash
# Browse root directory
curl "http://localhost:8080/api/browse/office-server?path=/&page=1&limit=50&sort_by=name&sort_order=asc"

# Browse specific directory
curl "http://localhost:8080/api/browse/office-server?path=/Documents&page=1&limit=100"
```

Response:
```json
{
  "success": true,
  "data": {
    "files": [
      {
        "id": 1001,
        "path": "/Documents/report.pdf",
        "name": "report.pdf",
        "size": 2048576,
        "is_directory": false,
        "extension": "pdf",
        "mime_type": "application/pdf",
        "modified_at": "2024-01-01T10:30:00Z",
        "metadata": []
      }
    ],
    "total_count": 150,
    "page": 1,
    "limit": 100,
    "total_pages": 2
  }
}
```

### Get File Information

```bash
curl http://localhost:8080/api/browse/file/1001
```

### Get Directories Sorted by Size

```bash
curl "http://localhost:8080/api/browse/office-server/sizes?page=1&limit=20&ascending=false"
```

Response:
```json
{
  "success": true,
  "data": [
    {
      "path": "/Videos",
      "name": "Videos",
      "smb_root_name": "office-server",
      "file_count": 25,
      "directory_count": 3,
      "total_size": 5368709120,
      "duplicate_count": 2,
      "modified_at": "2024-01-01T15:20:00Z"
    }
  ]
}
```

## Search Examples

### Basic Text Search

```bash
# Search for files containing "report" in name or path
curl "http://localhost:8080/api/search?q=report&page=1&limit=50"

# Search in specific SMB root
curl "http://localhost:8080/api/search?q=presentation&smb_roots=office-server"
```

### Advanced Search with Filters

```bash
# Search PDF files larger than 1MB modified in the last month
curl "http://localhost:8080/api/search?extension=pdf&min_size=1048576&modified_after=2023-12-01T00:00:00Z"

# Search for duplicate image files
curl "http://localhost:8080/api/search?file_type=image&only_duplicates=true"

# Search excluding directories
curl "http://localhost:8080/api/search?q=document&include_directories=false"
```

### Search Duplicates Only

```bash
curl "http://localhost:8080/api/search/duplicates?smb_roots=office-server&min_size=1048576"
```

### Advanced Search with POST

```bash
curl -X POST http://localhost:8080/api/search/advanced \
  -H "Content-Type: application/json" \
  -d '{
    "filter": {
      "query": "financial",
      "file_type": "document",
      "smb_roots": ["office-server", "backup-server"],
      "modified_after": "2023-01-01T00:00:00Z",
      "exclude_duplicates": true
    },
    "page": 1,
    "limit": 100,
    "sort_by": "modified_at",
    "sort_order": "desc"
  }'
```

## Download Examples

### Download Single File

```bash
# Download file as attachment
curl -O -J http://localhost:8080/api/download/file/1001

# Download file for inline viewing
curl "http://localhost:8080/api/download/file/1001?inline=true" -o preview.pdf
```

### Download Directory as ZIP

```bash
# Download entire directory
curl "http://localhost:8080/api/download/directory/office-server?path=/Documents&recursive=true" -o documents.zip

# Download with depth limit
curl "http://localhost:8080/api/download/directory/office-server?path=/Projects&recursive=true&max_depth=2" -o projects.zip
```

### Get Download Information

```bash
curl http://localhost:8080/api/download/info/1001
```

Response:
```json
{
  "success": true,
  "data": {
    "file_id": 1001,
    "name": "report.pdf",
    "path": "/Documents/report.pdf",
    "size": 2048576,
    "is_directory": false,
    "mime_type": "application/pdf",
    "extension": "pdf",
    "modified_at": "2024-01-01T10:30:00Z",
    "deleted": false
  }
}
```

## Copy Examples

### Copy to Local Computer

```bash
curl -X POST http://localhost:8080/api/copy/local \
  -H "Content-Type: application/json" \
  -d '{
    "source_file_id": 1001,
    "destination_path": "/home/user/Downloads/report.pdf",
    "overwrite_existing": true
  }'
```

### Upload from Local Computer

```bash
curl -X POST http://localhost:8080/api/copy/upload \
  -F "destination_smb_root=office-server" \
  -F "destination_path=/Uploads" \
  -F "overwrite_existing=false" \
  -F "file=@/path/to/local/file.pdf"
```

## Statistics Examples

### Overall Statistics

```bash
curl http://localhost:8080/api/stats/overall
```

Response:
```json
{
  "success": true,
  "data": {
    "total_files": 125000,
    "total_directories": 8500,
    "total_size": 536870912000,
    "total_duplicates": 2500,
    "duplicate_groups": 850,
    "smb_roots_count": 3,
    "active_smb_roots": 2,
    "last_scan_time": 1704110400
  }
}
```

### SMB Root Statistics

```bash
curl http://localhost:8080/api/stats/smb/office-server
```

### File Type Statistics

```bash
# All file types
curl http://localhost:8080/api/stats/filetypes

# For specific SMB root
curl "http://localhost:8080/api/stats/filetypes?smb_root=office-server&limit=20"
```

Response:
```json
{
  "success": true,
  "data": [
    {
      "file_type": "document",
      "extension": "pdf",
      "count": 15000,
      "total_size": 10737418240,
      "average_size": 715827
    },
    {
      "file_type": "image",
      "extension": "jpg",
      "count": 25000,
      "total_size": 5368709120,
      "average_size": 214748
    }
  ]
}
```

### Size Distribution

```bash
curl http://localhost:8080/api/stats/sizes
```

Response:
```json
{
  "success": true,
  "data": {
    "tiny": 50000,
    "small": 30000,
    "medium": 15000,
    "large": 5000,
    "huge": 1000,
    "massive": 200
  }
}
```

### Duplicate Statistics

```bash
curl http://localhost:8080/api/stats/duplicates
```

### Top Duplicate Groups

```bash
# Sort by file count
curl "http://localhost:8080/api/stats/duplicates/groups?sort_by=count&limit=10"

# Sort by total size
curl "http://localhost:8080/api/stats/duplicates/groups?sort_by=size&limit=10"
```

### Growth Trends

```bash
# Last 12 months
curl "http://localhost:8080/api/stats/growth?months=12"

# For specific SMB root
curl "http://localhost:8080/api/stats/growth?smb_root=office-server&months=6"
```

### Scan History

```bash
# Recent scans
curl "http://localhost:8080/api/stats/scans?limit=20"

# For specific SMB root
curl "http://localhost:8080/api/stats/scans?smb_root=office-server&limit=50&offset=0"
```

## Media Recognition Examples

### Recognize a Movie File

```bash
curl -X POST http://localhost:8080/api/v1/media/recognize \
  -H "Content-Type: application/json" \
  -d '{
    "file_path": "/movies/The.Matrix.1999.1080p.BluRay.x264.mkv",
    "media_type": "video",
    "confidence_threshold": 0.7
  }'
```

Response:
```json
{
  "success": true,
  "data": {
    "id": 12345,
    "title": "The Matrix",
    "year": 1999,
    "genre": "Science Fiction",
    "director": "The Wachowskis",
    "rating": 8.7,
    "confidence": 0.95,
    "external_ids": {
      "tmdb": "603",
      "imdb": "tt0133093"
    },
    "cover_art_url": "https://image.tmdb.org/t/p/w500/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg"
  }
}
```

### Recognize Music with Audio Fingerprinting

```bash
curl -X POST http://localhost:8080/api/v1/media/recognize \
  -H "Content-Type: application/json" \
  -d '{
    "file_path": "/music/Queen - Bohemian Rhapsody.mp3",
    "media_type": "audio",
    "enable_fingerprinting": true
  }'
```

### Batch Media Recognition

```bash
curl -X POST http://localhost:8080/api/v1/media/bulk-recognize \
  -H "Content-Type: application/json" \
  -d '{
    "files": [
      {"file_path": "/movies/Matrix1.mkv", "media_type": "video"},
      {"file_path": "/movies/Matrix2.avi", "media_type": "video"},
      {"file_path": "/music/Queen - We Will Rock You.mp3", "media_type": "audio"}
    ],
    "confidence_threshold": 0.6,
    "enable_fingerprinting": true
  }'
```

## Recommendations Examples

### Get Similar Items for a Media File

```bash
curl -X GET "http://localhost:8080/api/v1/media/123/similar?max_local=10&max_external=5&threshold=0.3" \
  -H "User-Platform: web" \
  -H "User-Context: desktop" \
  -H "User-Language: en"
```

Response:
```json
{
  "success": true,
  "data": {
    "local_items": [
      {
        "id": 124,
        "title": "The Matrix Reloaded",
        "genre": "Science Fiction",
        "year": 2003,
        "rating": 7.2,
        "similarity_score": 0.89,
        "similarity_reasons": ["same_franchise", "same_genre", "same_director"]
      }
    ],
    "external_items": [
      {
        "id": "ext_456",
        "title": "Blade Runner 2049",
        "provider": "TMDB",
        "url": "https://www.themoviedb.org/movie/335984",
        "score": 0.75,
        "description": "Similar cyberpunk themes and visual style"
      }
    ],
    "total_found": 15,
    "processing_time": "125ms"
  }
}
```

### Advanced Similar Items Search with Filters

```bash
curl -X POST http://localhost:8080/api/v1/media/similar \
  -H "Content-Type: application/json" \
  -H "User-Platform: android" \
  -d '{
    "media_id": 123,
    "filters": {
      "genre": "Science Fiction",
      "year_min": 1990,
      "year_max": 2010,
      "rating_min": 7.0,
      "confidence_min": 0.6
    },
    "max_local_items": 15,
    "max_external_items": 8,
    "include_trending": true
  }'
```

### Get Media with Similar Items (Batch)

```bash
curl -X GET "http://localhost:8080/api/v1/media/123/detail-with-similar" \
  -H "User-Platform: ios" \
  -H "User-Context: mobile"
```

Response:
```json
{
  "success": true,
  "data": {
    "media": {
      "id": 123,
      "title": "The Matrix",
      "description": "A computer programmer discovers reality is a simulation",
      "genre": "Science Fiction",
      "year": 1999
    },
    "similar_items": {
      "local_items": [...],
      "external_items": [...]
    },
    "deep_links": {
      "web_url": "https://catalogizer.app/item/123",
      "android_url": "catalogizer://item/123",
      "ios_url": "catalogizer://item/123",
      "smart_url": "https://catalogizer.app/smart/123"
    }
  }
}
```

### Get Trending Recommendations

```bash
curl -X GET "http://localhost:8080/api/v1/recommendations/trends?media_type=movie&period=7d&limit=20"
```

### Batch Recommendations

```bash
curl -X POST http://localhost:8080/api/v1/recommendations/batch \
  -H "Content-Type: application/json" \
  -d '{
    "media_ids": [123, 456, 789],
    "max_items_per_media": 5,
    "include_external": true,
    "filters": {
      "confidence_min": 0.5
    }
  }'
```

## Deep Linking Examples

### Generate Deep Links for All Platforms

```bash
curl -X POST http://localhost:8080/api/v1/links/generate \
  -H "Content-Type: application/json" \
  -H "User-Platform: web" \
  -d '{
    "media_id": 123,
    "context": "detail_screen",
    "utm_source": "app_share",
    "utm_medium": "social",
    "utm_campaign": "winter_2024",
    "custom_data": {
      "include_qr": "true"
    }
  }'
```

Response:
```json
{
  "success": true,
  "data": {
    "web_url": "https://catalogizer.app/item/123?utm_source=app_share&utm_medium=social&utm_campaign=winter_2024",
    "android_url": "catalogizer://item/123?utm_source=app_share&utm_medium=social&utm_campaign=winter_2024",
    "ios_url": "catalogizer://item/123?utm_source=app_share&utm_medium=social&utm_campaign=winter_2024",
    "desktop_url": "catalogizer://item/123?utm_source=app_share&utm_medium=social&utm_campaign=winter_2024",
    "smart_url": "https://catalogizer.app/smart/123",
    "qr_code_url": "https://catalogizer.app/qr/123.png",
    "analytics": {
      "utm_source": "app_share",
      "utm_medium": "social",
      "utm_campaign": "winter_2024"
    }
  }
}
```

### Generate Smart Links with Platform Detection

```bash
curl -X POST http://localhost:8080/api/v1/links/smart \
  -H "Content-Type: application/json" \
  -H "User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X)" \
  -d '{
    "media_id": 123,
    "fallback_url": "https://catalogizer.app/item/123",
    "enable_analytics": true
  }'
```

### Batch Deep Link Generation

```bash
curl -X POST http://localhost:8080/api/v1/links/batch \
  -H "Content-Type: application/json" \
  -d '{
    "media_ids": [123, 456, 789],
    "platforms": ["web", "android", "ios"],
    "utm_source": "email_campaign",
    "utm_medium": "newsletter",
    "include_qr_codes": true
  }'
```

### Track Link Events

```bash
curl -X POST http://localhost:8080/api/v1/links/track \
  -H "Content-Type: application/json" \
  -d '{
    "link_id": "abc123def456",
    "event_type": "click",
    "user_agent": "Mozilla/5.0 (Android 12; Mobile; rv:68.0) Gecko/68.0 Firefox/88.0",
    "referrer": "https://twitter.com",
    "ip_address": "192.168.1.100",
    "metadata": {
      "platform": "android",
      "location": "homepage",
      "user_id": "user789"
    }
  }'
```

### Get Link Analytics

```bash
curl -X GET "http://localhost:8080/api/v1/links/abc123def456/analytics?period=30d&group_by=platform"
```

Response:
```json
{
  "success": true,
  "data": {
    "link_id": "abc123def456",
    "total_clicks": 1250,
    "unique_clicks": 890,
    "conversion_rate": 0.712,
    "platforms": {
      "web": {"clicks": 650, "conversions": 520},
      "android": {"clicks": 400, "conversions": 280},
      "ios": {"clicks": 200, "conversions": 90}
    },
    "top_referrers": [
      {"source": "twitter.com", "clicks": 450},
      {"source": "facebook.com", "clicks": 320}
    ],
    "geographic_data": [
      {"country": "US", "clicks": 500},
      {"country": "CA", "clicks": 200}
    ]
  }
}
```

## Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "time": "2024-01-01T12:00:00Z"
}
```

## Error Examples

### File Not Found

```bash
curl http://localhost:8080/api/browse/file/999999
```

Response:
```json
{
  "success": false,
  "error": "File not found",
  "details": "file not found"
}
```

### Invalid Parameters

```bash
curl "http://localhost:8080/api/search?min_size=invalid"
```

Response:
```json
{
  "success": false,
  "error": "Invalid request parameters",
  "details": "min_size must be a valid number"
}
```

### Unauthorized Access

```bash
curl http://localhost:8080/api/browse/roots
```

Response (when auth is enabled):
```json
{
  "success": false,
  "error": "Authorization header required"
}
```

## Batch Operations Examples

### Multiple File Downloads

```bash
#!/bin/bash
# Download multiple files by ID
file_ids=(1001 1002 1003 1004)

for id in "${file_ids[@]}"; do
  echo "Downloading file ID: $id"
  curl -O -J "http://localhost:8080/api/download/file/$id"
done
```

### Bulk Search and Process

```bash
#!/bin/bash
# Search for large duplicate files and list them
curl -s "http://localhost:8080/api/search/duplicates?min_size=104857600&limit=1000" | \
  jq -r '.data.files[] | "\(.id): \(.path) (\(.size) bytes)"'
```

## Integration Examples

### Python Script

```python
import requests
import json

class CatalogAPI:
    def __init__(self, base_url, token=None):
        self.base_url = base_url
        self.headers = {}
        if token:
            self.headers['Authorization'] = f'Bearer {token}'

    def search_files(self, query, **kwargs):
        params = {'q': query, **kwargs}
        response = requests.get(
            f'{self.base_url}/api/search',
            params=params,
            headers=self.headers
        )
        return response.json()

    def download_file(self, file_id, output_path):
        response = requests.get(
            f'{self.base_url}/api/download/file/{file_id}',
            headers=self.headers,
            stream=True
        )

        with open(output_path, 'wb') as f:
            for chunk in response.iter_content(chunk_size=8192):
                f.write(chunk)

# Usage
api = CatalogAPI('http://localhost:8080')
results = api.search_files('presentation', file_type='document')
print(f"Found {results['data']['total_count']} files")
```

### JavaScript/Node.js Example

```javascript
const axios = require('axios');
const fs = require('fs');

class CatalogAPI {
  constructor(baseURL, token = null) {
    this.api = axios.create({
      baseURL,
      headers: token ? { Authorization: `Bearer ${token}` } : {}
    });
  }

  async searchFiles(query, options = {}) {
    const response = await this.api.get('/api/search', {
      params: { q: query, ...options }
    });
    return response.data;
  }

  async getStats() {
    const response = await this.api.get('/api/stats/overall');
    return response.data;
  }

  async downloadFile(fileId, outputPath) {
    const response = await this.api.get(`/api/download/file/${fileId}`, {
      responseType: 'stream'
    });

    const writer = fs.createWriteStream(outputPath);
    response.data.pipe(writer);

    return new Promise((resolve, reject) => {
      writer.on('finish', resolve);
      writer.on('error', reject);
    });
  }
}

// Usage
const api = new CatalogAPI('http://localhost:8080');
api.searchFiles('report', { file_type: 'document', limit: 10 })
  .then(results => console.log(results))
  .catch(error => console.error(error));
```

These examples demonstrate the comprehensive functionality available through the Catalog API, from basic file browsing to advanced analytics and automation scenarios.