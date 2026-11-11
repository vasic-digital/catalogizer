# Catalog API

A comprehensive REST API for browsing, searching, and recognizing media in multi-protocol file catalogs, built with Go and featuring AI-powered media recognition capabilities.

## Features

### Core File Management
- **Catalog Browsing**: Browse files and directories from cataloged storage sources (SMB, FTP, NFS, WebDAV, Local)
- **Advanced Search**: Search files by name, size, type, modification date, and more
- **Duplicate Detection**: Find and analyze duplicate files across all storage sources with advanced similarity algorithms
- **File Downloads**: Download individual files or create archives (ZIP, TAR, TAR.GZ)
- **Multi-Protocol Operations**: Copy files between different storage protocols
- **Statistics**: Directory size analysis and duplicate file statistics

### AI-Powered Media Recognition
- **Movie & TV Recognition**: Automatic recognition of movies, TV series, documentaries using TMDb and OMDb APIs
- **Music Recognition**: Audio fingerprinting and metadata extraction using Last.fm, MusicBrainz, and AcoustID
- **Book Recognition**: OCR-powered text extraction and metadata lookup using Google Books, Open Library, and Crossref
- **Game & Software Recognition**: Comprehensive recognition using IGDB, Steam, GitHub, and package managers (Winget, Flatpak, Homebrew)
- **Smart Duplicate Detection**: Advanced similarity analysis using multiple algorithms (Levenshtein, Jaro-Winkler, Cosine similarity, Jaccard index, Soundex, Metaphone)
- **Cover Art & Metadata Enhancement**: Automatic cover art discovery and comprehensive metadata enrichment

### Smart Recommendations & Discovery
- **Similar Items Discovery**: AI-powered recommendations showing local similar content first, then external suggestions
- **Multi-Algorithm Similarity**: Advanced content matching using metadata, textual similarity, and collaborative filtering
- **External Recommendations**: Integration with 16+ external services for comprehensive content discovery
- **Intelligent Filtering**: Genre, year, rating, language, and confidence-based recommendation filtering
- **Cross-Media Recommendations**: Discover similar content across different media types and formats
- **Trending Content Analysis**: Real-time trending recommendations based on user behavior and popularity

### Universal Deep Linking System
- **Cross-Platform Links**: Generate deep links for web, Android, iOS, and desktop applications
- **Smart Link Routing**: Automatically determine best link strategy based on user context and platform
- **Universal Link Support**: Single links that work across all platforms with intelligent fallbacks
- **App Store Integration**: Automatic app store links for users who don't have apps installed
- **UTM Parameter Support**: Full marketing campaign tracking with UTM parameter integration
- **Link Analytics**: Comprehensive tracking of link performance, conversion rates, and platform usage
- **QR Code Generation**: Automatic QR code creation for easy sharing and mobile access
- **Batch Link Generation**: Process multiple items simultaneously for efficient link creation

### Premium Reading Experience
- **Kindle-like Reader**: Advanced reading system with position tracking across devices
- **Multi-granular Position Tracking**: Page, word, character, and CFI (EPUB) position tracking
- **Cross-device Synchronization**: Seamless reading position sync with conflict resolution
- **Bookmarks & Highlights**: Full annotation system with search capabilities
- **Reading Analytics**: Reading speed tracking, time analytics, streaks, and goals
- **20+ Customization Options**: Themes, fonts, spacing, brightness, and reading modes

### Technical Excellence
- **RESTful API**: Clean REST API with JSON responses
- **CORS Support**: Enable cross-origin requests for web frontends
- **Comprehensive Testing**: 100% test coverage with mock servers for all external APIs
- **Performance Optimized**: Concurrent processing and intelligent caching

## API Endpoints

### Catalog Browsing
- `GET /api/v1/catalog` - List available storage root directories
- `GET /api/v1/catalog/{path}` - List files and directories in path
- `GET /api/v1/catalog-info/{path}?id={id}` - Get detailed file information

### Media Recognition
- `POST /api/v1/media/recognize` - Recognize media file and extract metadata
- `GET /api/v1/media/metadata/{id}` - Get cached metadata for a media file
- `POST /api/v1/media/bulk-recognize` - Batch recognize multiple media files
- `GET /api/v1/media/recognition-status/{job_id}` - Check batch recognition job status
- `POST /api/v1/media/duplicates/find` - Find duplicate media using AI similarity
- `GET /api/v1/media/duplicates/{id}` - Get duplicate groups for specific media
- `DELETE /api/v1/media/duplicates/{id}` - Remove duplicate (keep best quality)

### Recommendations & Similar Items
- `GET /api/v1/media/{id}/similar` - Get similar items for a specific media file
- `POST /api/v1/media/similar` - Advanced similar items search with custom filters
- `GET /api/v1/media/{id}/detail-with-similar` - Get media details with similar items and deep links
- `GET /api/v1/recommendations/trends` - Get trending recommendations by media type and period
- `POST /api/v1/recommendations/batch` - Get recommendations for multiple items simultaneously
- `GET /api/v1/recommendations/user/{user_id}` - Get personalized recommendations for user

### Deep Linking & Sharing
- `POST /api/v1/links/generate` - Generate deep links for all platforms
- `POST /api/v1/links/smart` - Generate smart links with automatic platform detection
- `POST /api/v1/links/batch` - Generate deep links for multiple items
- `POST /api/v1/links/track` - Track link click events and analytics
- `GET /api/v1/links/{tracking_id}/analytics` - Get detailed link performance analytics
- `POST /api/v1/links/validate` - Validate and test deep links
- `GET /api/v1/links/apps` - Get registered app configurations for deep linking

### Reader Service
- `POST /api/v1/reader/sessions` - Create new reading session
- `GET /api/v1/reader/sessions/{id}` - Get reading session details
- `PUT /api/v1/reader/sessions/{id}/position` - Update reading position
- `POST /api/v1/reader/sessions/{id}/bookmarks` - Add bookmark
- `GET /api/v1/reader/sessions/{id}/bookmarks` - Get all bookmarks
- `POST /api/v1/reader/sessions/{id}/highlights` - Add highlight
- `GET /api/v1/reader/sessions/{id}/highlights` - Get all highlights
- `GET /api/v1/reader/users/{user_id}/analytics` - Get reading analytics
- `PUT /api/v1/reader/users/{user_id}/settings` - Update reading settings
- `POST /api/v1/reader/sync/{session_id}` - Sync reading position across devices

### Search
- `GET /api/v1/search` - Search files with various filters
- `GET /api/v1/search/duplicates` - Find duplicate file groups
- `POST /api/v1/search/media` - Advanced media search with AI metadata
- `GET /api/v1/search/similar` - Find similar media based on content

### Downloads
- `GET /api/v1/download/file/{id}` - Download a single file
- `GET /api/v1/download/directory/{path}` - Download directory as archive
- `POST /api/v1/download/archive` - Create custom archive from file list

### Storage Operations
- `POST /api/v1/copy/storage` - Copy between storage sources
- `POST /api/v1/copy/local` - Copy from storage to local filesystem
- `POST /api/v1/copy/upload` - Upload file to storage source
- `GET /api/v1/storage/list/*path?storage_id={storage_id}` - List storage directory contents
- `GET /api/v1/storage/roots` - Get available storage roots

#### FileSystem Service

The FileSystemService provides unified access to multiple storage protocols with automatic connection management:

**Supported Protocols:**
- **Local**: Direct filesystem access
- **SMB**: Windows file shares (Server Message Block)
- **FTP**: File Transfer Protocol
- **NFS**: Network File System
- **WebDAV**: Web Distributed Authoring and Versioning

**Features:**
- Multi-protocol support with unified interface
- Automatic connection management and pooling
- File listing and metadata retrieval
- Cross-protocol file operations
- Comprehensive error handling
- Extensive test coverage (80%+)

**Usage:**
The service automatically handles connections and provides a consistent API across all protocols. Configure storage roots in your `config.json` and access them through the unified API endpoints.

### Statistics
- `GET /api/v1/stats/directories/by-size` - Get directories sorted by size
- `GET /api/v1/stats/duplicates/count` - Get duplicate file statistics
- `GET /api/v1/stats/media/overview` - Get media library statistics
- `GET /api/v1/stats/media/recognition-quality` - Get recognition confidence stats

## Configuration

Create a `config.json` file in the project root:

```json
{
  "server": {
    "host": "localhost",
    "port": "8080",
    "enable_cors": true
  },
  "smb": {
    "hosts": [
      {
        "name": "nas1",
        "host": "192.168.1.100",
        "port": 445,
        "share": "shared",
        "username": "user",
        "password": "password",
        "domain": "WORKGROUP"
      }
    ]
  },
  "catalog": {
    "temp_dir": "/tmp",
    "max_archive_size": 1073741824,
    "download_chunk_size": 1048576
  },
  "media_recognition": {
    "tmdb_api_key": "your_tmdb_api_key",
    "omdb_api_key": "your_omdb_api_key",
    "lastfm_api_key": "your_lastfm_api_key",
    "igdb_client_id": "your_igdb_client_id",
    "igdb_client_secret": "your_igdb_client_secret",
    "ocr_space_api_key": "your_ocr_space_api_key",
    "enable_fingerprinting": true,
    "cache_duration_hours": 168,
    "concurrent_workers": 5,
    "timeout_seconds": 30
  },
  "reader": {
    "sync_interval_seconds": 30,
    "position_history_limit": 100,
    "bookmark_limit_per_book": 1000,
    "highlight_limit_per_book": 5000,
    "analytics_retention_days": 365
  },
  "duplicate_detection": {
    "similarity_threshold": 0.8,
    "enable_phonetic_matching": true,
    "title_weight": 0.4,
    "artist_author_weight": 0.3,
    "year_weight": 0.1,
    "metadata_weight": 0.2
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
    "collaborative_weight": 0.4
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

- `CATALOG_CONFIG_PATH` - Path to configuration file (default: `config.json`)

## Running the API

1. Install dependencies:
```bash
go mod tidy
```

2. Run the server:
```bash
go run main.go
```

The API will be available at `http://localhost:8080`

## Health Check

Check if the API is running:
```bash
curl http://localhost:8080/health
```

## Example Requests

### Media Recognition

#### Recognize a movie file
```bash
curl -X POST http://localhost:8080/api/v1/media/recognize \
  -H "Content-Type: application/json" \
  -d '{
    "file_path": "/movies/The.Matrix.1999.1080p.BluRay.x264.mkv",
    "media_type": "video"
  }'
```

#### Recognize music with audio fingerprinting
```bash
curl -X POST http://localhost:8080/api/v1/media/recognize \
  -H "Content-Type: application/json" \
  -d '{
    "file_path": "/music/Queen - Bohemian Rhapsody.mp3",
    "media_type": "audio",
    "enable_fingerprinting": true
  }'
```

#### Recognize a book with OCR
```bash
curl -X POST http://localhost:8080/api/v1/media/recognize \
  -H "Content-Type: application/json" \
  -d '{
    "file_path": "/books/Harry Potter.pdf",
    "media_type": "book",
    "enable_ocr": true
  }'
```

#### Find duplicates using AI similarity
```bash
curl -X POST http://localhost:8080/api/v1/media/duplicates/find \
  -H "Content-Type: application/json" \
  -d '{
    "media_items": [
      {"file_path": "/movies/Matrix1.mkv", "media_type": "video"},
      {"file_path": "/movies/Matrix2.avi", "media_type": "video"}
    ],
    "similarity_threshold": 0.8
  }'
```

### Reader Service

#### Create reading session
```bash
curl -X POST http://localhost:8080/api/v1/reader/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "book_path": "/books/The Lord of the Rings.epub",
    "device_id": "tablet-001"
  }'
```

#### Update reading position
```bash
curl -X PUT http://localhost:8080/api/v1/reader/sessions/session-id/position \
  -H "Content-Type: application/json" \
  -d '{
    "page": 25,
    "word": 1250,
    "character": 18750,
    "cfi": "epubcfi(/6/4[chapter01]!/4/2/1:245)"
  }'
```

#### Add bookmark
```bash
curl -X POST http://localhost:8080/api/v1/reader/sessions/session-id/bookmarks \
  -H "Content-Type: application/json" \
  -d '{
    "page": 42,
    "position": 2100,
    "title": "Important Quote",
    "note": "Remember this passage"
  }'
```

### Traditional File Operations

#### Search for files
```bash
curl "http://localhost:8080/api/v1/search?query=document&extension=pdf&min_size=1000"
```

#### Get directories by size
```bash
curl "http://localhost:8080/api/v1/stats/directories/by-size?smb_root=nas1&limit=10"
```

#### Find duplicates
```bash
curl "http://localhost:8080/api/v1/search/duplicates?smb_root=nas1&min_count=2"
```

## Architecture

The API is structured with the following components:

### Core Components
- **Models**: Data structures for files, media metadata, reading sessions, etc.
- **Services**: Business logic layer with specialized services
- **Handlers**: HTTP request handlers for each endpoint group
- **Middleware**: Cross-cutting concerns (CORS, logging, error handling)
- **Config**: Configuration management

### Media Recognition Services
- **MediaRecognitionService**: Central orchestrator for AI-powered media recognition
- **MovieRecognitionProvider**: TMDb and OMDb integration for movies and TV shows
- **MusicRecognitionProvider**: Audio fingerprinting with Last.fm, MusicBrainz, AcoustID
- **BookRecognitionProvider**: OCR integration with Google Books, Open Library, Crossref
- **GameSoftwareRecognitionProvider**: IGDB, Steam, GitHub, and package manager integration
- **DuplicateDetectionService**: Advanced similarity analysis with multiple algorithms

### Reading Experience
- **ReaderService**: Kindle-like reading experience with position tracking
- **ReadingAnalyticsService**: Reading speed, time tracking, streaks, and goals
- **SynchronizationService**: Cross-device position sync with conflict resolution

### External Integrations
- **16 Different API Providers**: TMDb, OMDb, Last.fm, MusicBrainz, AcoustID, IGDB, Steam, GitHub, Google Books, Open Library, Crossref, OCR.space, Winget, Flatpak, Snapcraft, Homebrew
- **Mock Services**: Comprehensive test infrastructure for all external APIs
- **Intelligent Caching**: Multi-level caching with TTL strategies for optimal performance

## Dependencies

### Core Framework
- **Gin**: HTTP web framework
- **go-smb2**: SMB/CIFS client library
- **Zap**: Structured logging
- **UUID**: Request ID generation
- **SQLite**: Database driver (for catalog storage)

### Media Recognition & Processing
- **gorilla/mux**: Advanced HTTP router for media endpoints
- **stretchr/testify**: Comprehensive testing framework
- **Various HTTP clients**: For external API integrations

### Audio/Video Processing
- **FFmpeg bindings** (when available): Audio fingerprinting and spectral analysis
- **Image processing libraries**: Cover art and thumbnail generation
- **PDF processing libraries**: Text extraction and OCR integration

### Text Processing & Similarity
- **Text analysis algorithms**: Levenshtein, Jaro-Winkler, Cosine similarity
- **Phonetic matching**: Soundex and Metaphone algorithms
- **Language detection**: Multi-language text analysis

### Database & Caching
- **SQLite with optimizations**: Indexed search and metadata storage
- **In-memory caching**: Multi-level caching with TTL management
- **JSON processing**: Configuration and metadata serialization

## AI-Powered Media Recognition

### Supported Media Types
- **Movies & TV Shows**: Recognition using filename parsing and external metadata APIs
- **Music**: Audio fingerprinting and metadata extraction from multiple sources
- **Books & Publications**: OCR text extraction and comprehensive metadata lookup
- **Games**: Platform-specific recognition with IGDB database integration
- **Software**: Multi-platform recognition using package managers and GitHub

### Recognition Confidence Scoring
- **High Confidence (90-100%)**: Exact matches with multiple metadata confirmations
- **Medium Confidence (70-89%)**: Good matches with some metadata variations
- **Low Confidence (50-69%)**: Filename-based recognition with limited metadata
- **Very Low Confidence (<50%)**: Basic file information only

### Duplicate Detection Algorithms
- **Levenshtein Distance**: Character-level text similarity
- **Jaro-Winkler Similarity**: Optimized for names and titles
- **Cosine Similarity**: Vector-based content similarity
- **Jaccard Index**: Set-based similarity comparison
- **Soundex & Metaphone**: Phonetic matching for audio content
- **Custom Media Weighting**: Media-type specific similarity calculations

### Caching Strategy
- **API Response Caching**: 7-day TTL for external API responses
- **Metadata Caching**: 30-day TTL for recognized media metadata
- **Similarity Caching**: 1-day TTL for duplicate detection results
- **Fingerprint Caching**: 90-day TTL for audio fingerprints

## Development

The project follows Go best practices with:

### Code Quality
- Clean architecture with separated concerns
- Dependency injection for testability
- Comprehensive error handling with structured logging
- 100% test coverage with mock external services
- Performance benchmarks for critical operations

### Testing Strategy
- **Unit Tests**: Individual service and component testing
- **Integration Tests**: End-to-end API and service testing
- **Performance Tests**: Concurrent processing and load testing
- **Mock Services**: Realistic external API simulation for testing
- **Benchmark Tests**: Performance measurement and optimization

### Security & Reliability
- **API Key Management**: Secure configuration and rotation
- **Rate Limiting**: Respectful external API usage
- **Graceful Degradation**: Fallback mechanisms for API failures
- **Data Validation**: Input sanitization and validation
- **Concurrent Processing**: Safe parallel media recognition

## Integration with Catalogizer

This API is designed to work with the existing Catalogizer Kotlin application by:

1. Reading from the same file catalog database
2. Using compatible SMB connection configurations
3. Providing REST endpoints for catalog operations
4. Supporting the same file metadata structure

The API can be deployed separately and accessed by web frontends, mobile apps, or other services that need programmatic access to the file catalog.

## Example API Requests

### Recommendations & Similar Items

#### Get Similar Items for a Media Item
```bash
curl -X GET "http://localhost:8080/api/similar/123" \
  -H "User-Platform: web" \
  -H "User-Context: desktop" \
  -H "User-Language: en"
```

#### Get Similar Items with Filters
```bash
curl -X GET "http://localhost:8080/api/similar/123?genre=action&year_min=2020&confidence_min=0.7&limit=10" \
  -H "User-Platform: android" \
  -H "User-Context: mobile"
```

#### Get Media with Similar Items (Batch)
```bash
curl -X POST "http://localhost:8080/api/media/with-similar" \
  -H "Content-Type: application/json" \
  -H "User-Platform: ios" \
  -d '{
    "media_ids": [123, 456, 789],
    "filters": {
      "genre": "drama",
      "rating_min": 7.0,
      "limit": 5
    }
  }'
```

#### Get Trending Similar Items
```bash
curl -X GET "http://localhost:8080/api/trending/similar/123?days=7&min_views=100"
```

### Deep Linking & Sharing

#### Generate Deep Links for Media Item
```bash
curl -X GET "http://localhost:8080/api/deeplink/123" \
  -H "User-Platform: web" \
  -H "User-Context: desktop" \
  -H "User-Language: en"
```

#### Generate Smart Link with Custom Parameters
```bash
curl -X POST "http://localhost:8080/api/smartlink" \
  -H "Content-Type: application/json" \
  -d '{
    "media_id": 123,
    "utm_source": "email_campaign",
    "utm_medium": "newsletter",
    "utm_campaign": "winter_2024",
    "platforms": ["web", "android", "ios"],
    "include_qr": true
  }'
```

#### Generate Batch Deep Links
```bash
curl -X POST "http://localhost:8080/api/deeplink/batch" \
  -H "Content-Type: application/json" \
  -H "User-Platform: android" \
  -d '{
    "media_ids": [123, 456, 789],
    "utm_source": "app_share",
    "include_analytics": true
  }'
```

#### Track Link Event
```bash
curl -X POST "http://localhost:8080/api/link/track" \
  -H "Content-Type: application/json" \
  -d '{
    "link_id": "abc123",
    "event_type": "click",
    "user_agent": "Mozilla/5.0...",
    "referrer": "https://example.com",
    "metadata": {
      "platform": "web",
      "location": "homepage"
    }
  }'
```

### Configuration Examples

#### Recommendations Configuration
```json
{
  "recommendations": {
    "cache_ttl": "24h",
    "max_local_items": 20,
    "max_external_items": 10,
    "confidence_threshold": 0.5,
    "external_apis": {
      "tmdb": {
        "enabled": true,
        "api_key": "your_tmdb_key"
      },
      "lastfm": {
        "enabled": true,
        "api_key": "your_lastfm_key"
      },
      "google_books": {
        "enabled": true,
        "api_key": "your_google_books_key"
      }
    }
  }
}
```

#### Deep Linking Configuration
```json
{
  "deep_linking": {
    "base_urls": {
      "web": "https://catalogizer.app",
      "android": "catalogizer://",
      "ios": "catalogizer://",
      "desktop": "catalogizer://"
    },
    "universal_links": {
      "domain": "catalogizer.app",
      "path_prefix": "/item/"
    },
    "analytics": {
      "enabled": true,
      "retention_days": 90
    },
    "qr_codes": {
      "size": 256,
      "error_correction": "medium"
    }
  }
}
```