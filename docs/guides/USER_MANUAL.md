# Catalogizer User Manual v2.3.0

A comprehensive guide to installing, configuring, and using Catalogizer -- the multi-platform media collection manager. Catalogizer detects, categorizes, and organizes media files across SMB, FTP, NFS, WebDAV, and local filesystems, serving them through a web interface, desktop applications, and mobile apps.

---

## Table of Contents

- [Chapter 1: Getting Started](#chapter-1-getting-started)
- [Chapter 2: Storage Configuration](#chapter-2-storage-configuration)
- [Chapter 3: Scanning and Detection](#chapter-3-scanning-and-detection)
- [Chapter 4: Browsing Media](#chapter-4-browsing-media)
- [Chapter 5: Collections](#chapter-5-collections)
- [Chapter 6: Playback](#chapter-6-playback)
- [Chapter 7: Android Mobile App](#chapter-7-android-mobile-app)
- [Chapter 8: Android TV App](#chapter-8-android-tv-app)
- [Chapter 9: Desktop App](#chapter-9-desktop-app)
- [Chapter 10: Administration](#chapter-10-administration)
- [Chapter 11: Monitoring](#chapter-11-monitoring)
- [Chapter 12: Troubleshooting](#chapter-12-troubleshooting)

---

## Chapter 1: Getting Started

### What Is Catalogizer

Catalogizer is a self-hosted, multi-platform media collection manager. It scans your storage systems -- NAS devices, file servers, cloud storage, and local drives -- to automatically detect, categorize, and organize media files. It recognizes 11 distinct media types (movies, TV shows, music, games, software, books, comics, and more) and enriches them with metadata from providers like TMDB, OMDB, OpenLibrary, and MusicBrainz.

Catalogizer consists of seven components:

| Component | Technology | Description |
|-----------|-----------|-------------|
| catalog-api | Go / Gin | REST API backend with SQLite or PostgreSQL |
| catalog-web | React 18 / TypeScript / Vite | Web frontend served on port 3000 |
| catalogizer-desktop | Tauri 2 / Rust + React | Desktop app for Windows, macOS, Linux |
| installer-wizard | Tauri 2 / Rust + React | Storage configuration wizard |
| catalogizer-android | Kotlin / Jetpack Compose | Android mobile app |
| catalogizer-androidtv | Kotlin / Compose for TV | Android TV app with D-pad navigation |
| catalogizer-api-client | TypeScript | Shared API client library |

### System Requirements

**Server (catalog-api):**

- Go 1.25 or later (for building from source)
- 2 CPU cores minimum, 4 recommended
- 2 GB RAM minimum, 4 GB recommended
- SQLite (development) or PostgreSQL 14+ (production)
- Optional: Redis for caching and rate limiting
- Optional: FFmpeg for media format conversion

**Web Client (catalog-web):**

- Any modern browser: Chrome 90+, Firefox 90+, Safari 15+, Edge 90+
- JavaScript enabled

**Desktop Client:**

- Windows 10+, macOS 11+, or Linux (AppImage/deb)
- 200 MB disk space

**Android Mobile:**

- Android 8.0 (API 26) or higher
- 2 GB RAM minimum
- Network connectivity to the Catalogizer server

**Android TV:**

- Android TV OS 8.0 or higher
- Remote control or gamepad
- Network connectivity (Wi-Fi or Ethernet)

### First-Time Installation and Setup

The fastest way to get Catalogizer running is with Podman (or Docker):

```bash
# Production stack with all services
podman-compose -f docker-compose.yml up -d

# Development stack
podman-compose -f docker-compose.dev.yml up -d
```

To run from source for development:

```bash
# Terminal 1: Start the backend
cd catalog-api
go run main.go
# The server writes its port to .service-port for frontend discovery

# Terminal 2: Start the frontend
cd catalog-web
npm install
npm run dev
# Opens on http://localhost:3000, proxies /api to the backend
```

The backend creates a SQLite database file (`catalogizer.db`) automatically on first run. No database setup is required for development.

### First Login

On first launch, a default admin account is created. The credentials are set via environment variables or `.env` file in the `catalog-api/` directory:

```env
ADMIN_USERNAME=admin
ADMIN_PASSWORD=<ADMIN_PASSWORD>
JWT_SECRET=your-dev-secret-key-minimum-32-characters
```

1. Open `http://localhost:3000` in your browser.
2. Enter the admin username and password on the login screen.
3. After login you are redirected to the Dashboard.
4. Change the default password immediately: navigate to your profile settings or use the change password API endpoint (`POST /api/v1/auth/change-password`).

<!-- Screenshot: Login screen with username and password fields, a "Sign In" button, and the Catalogizer logo -->

### Initial Configuration Wizard

The Installer Wizard application walks you through configuring storage sources. Download the wizard for your platform (Windows .msi, macOS .dmg, Linux .AppImage) or build from source:

```bash
cd installer-wizard
npm install
npm run tauri:build
```

The wizard guides you through six steps:

1. **Welcome** -- overview of what you need (credentials, server addresses)
2. **Protocol Selection** -- choose SMB, FTP, NFS, WebDAV, or Local
3. **Protocol Configuration** -- enter connection details and test connectivity
4. **Network Scan** (optional) -- auto-discover SMB servers on your LAN
5. **Configuration Management** -- review and manage all added sources
6. **Summary** -- save the configuration file and deploy it to your server

After completing the wizard, copy the generated configuration file to your server and restart catalog-api.

---

## Chapter 2: Storage Configuration

### Understanding Storage Roots

A storage root is a configured entry point into a filesystem that Catalogizer scans for media. Each root specifies a protocol, a path, and optional credentials. You can configure multiple roots pointing to different servers, shares, or local directories.

Storage roots are managed via:
- The web admin panel (Settings > Storage)
- The Installer Wizard desktop application
- The REST API directly

**API endpoints for storage roots:**

```
GET  /api/v1/storage-roots          -- List all storage roots
POST /api/v1/storage/roots          -- Create a new storage root
GET  /api/v1/storage-roots/:id/status -- Check status of a specific root
```

### Adding Local Filesystem Paths

Local paths are the simplest storage type. Point Catalogizer to a directory on the server machine:

1. In the web admin panel, navigate to Storage settings.
2. Click **Add Storage Source**.
3. Enter the absolute path (e.g., `/mnt/media` or `C:\Media`).
4. No credentials are required.
5. Click **Add Source**.

Via the API:

```bash
curl -X POST http://localhost:8080/api/v1/storage/roots \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Local Media",
    "protocol": "local",
    "path": "/mnt/media",
    "enabled": true
  }'
```

### Configuring SMB/CIFS Shares (Windows/NAS)

SMB is the most common protocol for NAS devices (Synology, QNAP) and Windows file shares.

**Required information:**

| Field | Example | Description |
|-------|---------|-------------|
| Host | `192.168.0.241` or `synology.local` | Server hostname or IP |
| Port | `445` | SMB port (445 is standard) |
| Share Name | `media` | Name of the shared folder |
| Username | `mediauser` | SMB authentication username |
| Password | `secret` | SMB authentication password |
| Domain | `WORKGROUP` | Windows domain (optional) |
| Path | `/movies` | Subdirectory within the share (optional) |

**Resilience features built into SMB connections:**

- **Circuit breaker**: After repeated connection failures, requests fail fast instead of hanging. The breaker transitions through Closed (normal) to Open (failing fast) to Half-Open (testing recovery).
- **Offline cache**: When an SMB source goes offline, cached metadata continues serving requests.
- **Exponential backoff retry**: Reconnection attempts are spaced exponentially to avoid flooding the server.

**SMB-specific API endpoints:**

```
POST /api/v1/smb/discover   -- Discover SMB shares on the network
POST /api/v1/smb/test        -- Test an SMB connection
POST /api/v1/smb/browse      -- Browse files on an SMB share
```

**Environment variables for SMB tuning:**

```env
SMB_VERSION=3
SMB_TIMEOUT=30
SMB_MAX_CONNECTIONS=10
SMB_CIRCUIT_BREAKER_THRESHOLD=5
SMB_CIRCUIT_BREAKER_TIMEOUT=60
SMB_OFFLINE_CACHE_ENABLED=true
```

### Configuring FTP Servers

```bash
curl -X POST http://localhost:8080/api/v1/storage/roots \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "FTP Media Server",
    "protocol": "ftp",
    "host": "ftp.example.com",
    "port": 21,
    "username": "ftpuser",
    "password": "ftppassword",
    "path": "/media",
    "enabled": true
  }'
```

**FTP configuration variables:**

```env
FTP_TIMEOUT=30
FTP_PASSIVE_MODE=true
FTP_TLS_ENABLED=false
FTP_MAX_CONNECTIONS=5
```

For servers behind NAT or firewalls, enable passive mode. If the server supports FTPS, set `FTP_TLS_ENABLED=true`.

### Configuring NFS Mounts

NFS uses host-based access control rather than username/password authentication. Ensure the NFS server exports the directory to the Catalogizer server's IP.

```env
NFS_VERSION=4
NFS_MOUNT_OPTIONS=soft,timeo=30,retrans=3
NFS_AUTO_MOUNT=true
```

Specify the export path (e.g., `/exports/media`) and optionally the NFS version and mount options.

### Configuring WebDAV Endpoints

WebDAV works over HTTP/HTTPS and is useful for cloud storage or web-based file servers:

```env
WEBDAV_TIMEOUT=60
WEBDAV_CHUNK_SIZE=8388608
WEBDAV_MAX_RETRIES=3
```

Provide the full URL (e.g., `https://cloud.example.com/webdav/media`), username, and password. Use HTTPS whenever possible.

### Testing Connections

Every protocol supports connection testing before committing a storage root:

1. In the Installer Wizard, click **Test Connection** after filling in the configuration form.
2. In the web admin panel, use the test button next to each storage source.
3. Via the API: `POST /api/v1/smb/test` (and equivalent endpoints for other protocols).

Test results indicate either success or a specific failure reason: connection refused, authentication failed, host not found, or connection timeout.

### Managing Multiple Storage Roots

You can configure as many storage roots as needed. Each root is independently scanned, and its status is tracked separately. Use the storage roots list to:

- View connection status (connected, disconnected, offline)
- Enable or disable individual roots without deleting them
- Monitor retry attempts and last-connected timestamps
- Trigger reconnection for failed roots

<!-- Screenshot: Admin panel showing a list of storage roots with status indicators (green connected, red offline), protocol icons, and action buttons -->

---

## Chapter 3: Scanning and Detection

### How Scanning Works

Catalogizer uses a UniversalScanner that traverses every configured storage root and catalogs the files it finds. The scanning process is protocol-agnostic -- the same scanner works across SMB, FTP, NFS, WebDAV, and local filesystems through the `UnifiedClient` interface defined in `filesystem/interface.go`.

**Scan API endpoints:**

```
POST /api/v1/scan            -- Queue a new scan job
GET  /api/v1/scan            -- List all scan jobs
GET  /api/v1/scan/:job_id    -- Get status of a specific scan job
```

### Initiating Manual Scans

**From the web interface:**

1. Navigate to the Dashboard.
2. Click the **Scan Library** quick action button.
3. A scan job is queued and begins processing storage roots.
4. Monitor progress from the Dashboard activity feed or via WebSocket real-time updates.

**From the admin panel:**

Navigate to Storage settings and click **Scan Storage** to trigger a full scan.

**Via the API:**

```bash
curl -X POST http://localhost:8080/api/v1/scan \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"storage_root_id": "all"}'
```

### Understanding the Detection Pipeline

After files are discovered by the scanner, they pass through a three-stage pipeline:

```
1. DETECT    (media/detector/)
   Identifies file type based on extension, MIME type, and file structure.

2. ANALYZE   (media/analyzer/)
   Extracts metadata: resolution, duration, bitrate, codec information.

3. AGGREGATE (services/aggregation_service.go)
   Groups files into media entities, builds hierarchies, detects duplicates.
```

The aggregation stage is the most important for end users. It is triggered automatically after each scan completes via a post-scan hook in `AggregationService.AggregateAfterScan()`.

### The 11 Media Types

Catalogizer recognizes and categorizes media into these types (seeded in the `media_types` database table):

| Type | Description | Example |
|------|-------------|---------|
| `movie` | Feature films | The Matrix (1999) |
| `tv_show` | Television series (parent entity) | Breaking Bad |
| `tv_season` | Season within a TV show | Breaking Bad Season 1 |
| `tv_episode` | Individual episode | S01E01 - Pilot |
| `music_artist` | Musical artist (parent entity) | Pink Floyd |
| `music_album` | Album by an artist | The Dark Side of the Moon |
| `song` | Individual audio track | Time |
| `game` | Video game | Half-Life 2 |
| `software` | Application or utility | LibreOffice 7.6 |
| `book` | E-book or audiobook | Dune |
| `comic` | Comic book or graphic novel | Watchmen |

### Title Parsing and Metadata Extraction

The title parser (`internal/services/title_parser.go`) uses regex patterns to extract structured information from filenames:

- **Movies**: `Movie.Title.2024.1080p.BluRay.x264.mkv` parses to title "Movie Title", year 2024, quality "1080p"
- **TV Episodes**: `Show.Name.S03E07.Episode.Title.720p.mkv` parses to show "Show Name", season 3, episode 7
- **Music**: `Artist - Album (2023) - 01 - Track Title.flac` parses to artist, album, year, track number, title

After title parsing, Catalogizer queries metadata providers:

- **TMDB** and **OMDB** for movie and TV metadata (poster art, synopsis, ratings, cast)
- **OpenLibrary** for book metadata (author, publisher, ISBN, cover art)
- **MusicBrainz** for music metadata (artist, album, track listing, genre)

Missing API keys or unavailable providers do not block the pipeline -- metadata enrichment degrades gracefully.

### Scan Progress Monitoring

Monitor scan progress through multiple channels:

- **Dashboard activity feed**: Shows real-time scan events
- **WebSocket**: The frontend receives push updates via `ws://server:8080/ws`
- **API polling**: Query `GET /api/v1/scan/:job_id` for status and progress percentage
- **Prometheus metrics**: `catalogizer_media_files_scanned_total` and `catalogizer_media_files_analyzed_total`

**Configuration for the analysis pipeline:**

```env
MEDIA_WORKERS=4            # Concurrent analysis workers
ANALYSIS_TIMEOUT=30s       # Timeout per file
ANALYSIS_QUEUE_SIZE=1000   # Queue buffer size
MIN_FILE_SIZE=1048576      # Minimum 1 MB
MAX_FILE_SIZE=10737418240  # Maximum 10 GB
```

<!-- Screenshot: Dashboard showing scan progress bar at 67%, activity feed listing recently detected files, and a notification toast "Scan in progress: 4,521 of 6,780 files processed" -->

---

## Chapter 4: Browsing Media

### Entity Browser Overview

The Media Browser is the primary interface for exploring your cataloged media. Access it from the main navigation bar in the web app.

**Key API endpoints used by the browser:**

```
GET /api/v1/entities              -- List media entities with pagination and filters
GET /api/v1/entities/types        -- Get available media types
GET /api/v1/entities/stats        -- Get entity statistics
GET /api/v1/entities/browse/:type -- Browse entities filtered by type
GET /api/v1/entities/:id          -- Get a single entity with full details
GET /api/v1/entities/:id/children -- Get child entities (e.g., seasons of a TV show)
GET /api/v1/entities/:id/files    -- Get files associated with an entity
GET /api/v1/entities/:id/metadata -- Get external metadata (TMDB, OpenLibrary, etc.)
```

The browser displays four summary cards at the top: Total Items, Media Types, Total Size, and Recent Additions.

### Searching Media

Type in the search bar to filter media by title, description, or metadata. Search is debounced with a 300ms delay so results update as you type without excessive API calls.

**API endpoints for search:**

```
GET  /api/v1/media/search     -- Search media items (text query, filters)
GET  /api/v1/search            -- General catalog search
GET  /api/v1/search/files      -- Search individual files
POST /api/v1/search/advanced   -- Advanced search with complex criteria
GET  /api/v1/search/duplicates -- Find duplicate files
```

Example API call:

```bash
curl "http://localhost:8080/api/v1/media/search?q=matrix&type=movie&limit=20" \
  -H "Authorization: Bearer $TOKEN"
```

### Filtering by Type, Year, Rating

Click the **Filters** button to open the filter sidebar with these options:

- **Media type**: movie, tv_show, music_album, song, game, software, book, comic, and more
- **Quality level**: SD, 720p, 1080p, 4K
- **Year range**: filter by release year
- **File size range**: filter by file size
- **Sort order**: by title, date updated, date added, year, rating, or file size
- **Sort direction**: ascending or descending

Click **Reset** to clear all active filters.

### View Modes

Toggle between two view modes using the icons next to the filter controls:

- **Grid View**: Media displayed as visual cards in a responsive grid. Each card shows a poster image (from TMDB/OMDB metadata or a placeholder), title, year, and rating. Cards have a hover zoom animation.
- **List View**: Compact rows showing thumbnail, title, year, type, rating, and a description excerpt. More items visible per screen.

### Entity Detail View

Click any media item to open the detail view, which shows:

- Full title, year, and quality information
- Media type badge and file size
- Description and synopsis from metadata providers
- External metadata: TMDB rating, OMDB data, OpenLibrary ISBN, MusicBrainz release info
- Entity hierarchy: for TV shows, navigate to seasons and episodes; for music artists, navigate to albums and songs
- Associated files list with paths, sizes, and formats

**Action buttons on the detail view:**
- **Play** -- opens the media player
- **Download** -- downloads the file to your device
- **Refresh Metadata** -- re-fetches metadata from external providers

### Media File Information

For each entity, view its associated files via `GET /api/v1/entities/:id/files`. File information includes:

- Original file path and filename
- File size and format
- Storage root and protocol
- Detection timestamp

### Cover Art and Thumbnails

Cover art is served via dedicated endpoints:

```
GET /api/v1/cover/:id                  -- Serve cover image by ID
GET /api/v1/cover/url/:id              -- Get cover image URL
GET /api/v1/cover/placeholder/:type    -- Get a placeholder for a media type
GET /api/v1/assets/by-entity/:type/:id -- Get asset by entity type and ID
```

Cover art is fetched from TMDB, OMDB, OpenLibrary, and MusicBrainz during the metadata enrichment phase.

<!-- Screenshot: Media Browser in grid view showing movie posters in a 4-column grid. The search bar reads "sci-fi" and the type filter is set to "Movies". Pagination shows "Page 1 of 8" -->

---

## Chapter 5: Collections

### Creating Collections

Collections let you organize media items into themed groups. There are two types:

- **Manual Collections**: You add and remove items by hand.
- **Smart Collections**: Automatically populated based on rules you define (e.g., "all movies from 2024 with rating > 7").

**API endpoints for collections:**

```
GET    /api/v1/collections         -- List all collections
POST   /api/v1/collections         -- Create a new collection
GET    /api/v1/collections/:id     -- Get a specific collection
PUT    /api/v1/collections/:id     -- Update a collection
DELETE /api/v1/collections/:id     -- Delete a collection
```

**To create a collection in the web app:**

1. Navigate to the Collections page.
2. Click the **Smart Collection** button to open the Smart Collection Builder.
3. Enter a name and description.
4. Define rules (conditions) that determine which media items are automatically included.
5. Click **Save**.

### Adding Items to Collections

For manual collections, browse your media library, select items, and add them to an existing collection. From the Collections page, open a collection and use the **Add Items** interface to search and select media.

### Smart Collections with Rules

Smart collections auto-update as your library changes. Define rules using criteria such as:

- Media type equals "movie"
- Year greater than 2020
- Rating greater than 7.5
- Title contains "Marvel"

Multiple rules can be combined with AND/OR logic.

### Collection Import/Export

Collections can be exported in multiple formats:

- **JSON** -- full collection data including rules and item references
- **CSV** -- tabular format for spreadsheet applications
- **M3U** -- playlist format for media players

Use the **Export** button on any collection card, or use bulk export to export multiple collections at once.

To import: click **Import** and upload a previously exported JSON file.

### Bulk Operations

1. Enable selection mode by clicking the checkboxes on collection cards.
2. Use **Select All** to select every visible collection.
3. Click **Bulk Actions** to perform batch operations: delete, share, export, or duplicate.

### Sharing Collections

Share a collection with other Catalogizer users:

1. Click **Share** on a collection.
2. Set permissions: view, comment, or download.
3. A share link is generated using the server's actual hostname (not hardcoded localhost), so it works when accessed from other devices on the network.

### Real-Time Collaboration

Enable live collaboration on a collection so multiple users can curate it simultaneously. Changes propagate in real-time via WebSocket.

<!-- Screenshot: Collections page showing a grid of collection cards. One card is labeled "Sci-Fi Classics" with 47 items, another "2024 Releases" with 12 items. The Smart Collection Builder modal is open with a rule "type = movie AND year >= 2020" -->

---

## Chapter 6: Playback

### Media Player Overview

The web app includes a built-in media player for video and audio playback. Click **Play** on any media item to open the fullscreen player with standard controls.

**Streaming API endpoint:**

```
GET /api/v1/stream/:id             -- Stream a file by ID
GET /api/v1/entities/:id/stream    -- Stream an entity's primary file
GET /api/v1/entities/:id/download  -- Download an entity's file
```

The player supports seek, volume control, and fullscreen mode. Click **Close** or press Escape to exit.

### Playlist Management

Playlists let you create ordered sequences of media items for continuous playback.

**API endpoints for playlists:**

```
GET    /api/v1/playlists               -- List all playlists
POST   /api/v1/playlists               -- Create a playlist
GET    /api/v1/playlists/:id           -- Get a playlist with items
PUT    /api/v1/playlists/:id           -- Update playlist details
DELETE /api/v1/playlists/:id           -- Delete a playlist
POST   /api/v1/playlists/:id/items     -- Add an item to a playlist
DELETE /api/v1/playlists/:id/items/:item_id -- Remove an item
```

**Web app playlist features:**

- Create, edit, and delete playlists
- Drag and drop to reorder items
- Shuffle mode for randomized playback
- Smart Playlist Builder with auto-population rules
- Playlist Player for sequential or shuffled playback

### Subtitle Management

The Subtitle Manager lets you search, download, upload, and manage subtitles.

**API endpoints for subtitles:**

```
GET  /api/v1/subtitles/search                   -- Search external subtitle providers
POST /api/v1/subtitles/download                  -- Download a subtitle
GET  /api/v1/subtitles/media/:media_id           -- Get subtitles for a media item
GET  /api/v1/subtitles/:subtitle_id/verify-sync/:media_id -- Verify subtitle timing
POST /api/v1/subtitles/translate                 -- Translate a subtitle
POST /api/v1/subtitles/upload                    -- Upload a subtitle file
GET  /api/v1/subtitles/languages                 -- List supported languages
GET  /api/v1/subtitles/providers                 -- List subtitle providers
```

**Workflow:**

1. Go to the Subtitle Manager page.
2. Click **Select Media** and choose the media item.
3. Search for subtitles by title and language.
4. Download the best match from the search results.
5. If timing is off, use the **Verify Sync** button to open the Subtitle Sync Modal and adjust the offset in milliseconds.
6. Upload your own subtitle files via the **Upload Subtitle** button.

### Format Conversion

Convert media files between formats using the built-in converter (requires FFmpeg on the server).

**Supported formats:**

- Video: MP4, MKV, AVI, MOV, WebM
- Audio: MP3, WAV, FLAC

**API endpoints for conversion:**

```
POST   /api/v1/conversion/jobs              -- Create a conversion job
GET    /api/v1/conversion/jobs              -- List all jobs
GET    /api/v1/conversion/jobs/:id          -- Get job status
POST   /api/v1/conversion/jobs/:id/cancel   -- Cancel a running job
POST   /api/v1/conversion/jobs/:id/retry    -- Retry a failed job
DELETE /api/v1/conversion/jobs/:id          -- Delete a job
GET    /api/v1/conversion/jobs/:id/download -- Download converted file
GET    /api/v1/conversion/formats           -- List supported formats
```

**Workflow:**

1. Navigate to the Format Converter page in the web app.
2. Select a source file from your library.
3. Choose the target output format and quality settings.
4. Click **Start Conversion**.
5. Monitor progress (the job list auto-refreshes every 30 seconds).
6. Download the converted file when complete.

### Streaming to Devices

Media playback is available on all client platforms:

- **Web**: Built-in HTML5 player
- **Android**: ExoPlayer (Media3) with offline caching
- **Android TV**: ExoPlayer with TV session support (play/pause from remote, background audio)
- **Desktop**: Integrated player within the Tauri app

All clients stream from the same `GET /api/v1/stream/:id` endpoint. The server supports range requests for seeking within large files.

<!-- Screenshot: Media player in fullscreen showing a movie with playback controls (play/pause, seek bar, volume, fullscreen toggle) and the title overlaid at the top -->

---

## Chapter 7: Android Mobile App

### Installation

**System Requirements:** Android 8.0+ (API 26), 2 GB RAM, network connectivity.

**From APK (sideload):**

1. Download the APK from your organization's distribution channel.
2. On your device, go to **Settings > Security** and enable **Install from unknown sources** (or grant permission to your file manager).
3. Open the downloaded APK and tap **Install**.
4. Launch Catalogizer from your app drawer.

**From source:**

```bash
cd catalogizer-android
./gradlew assembleDebug
./gradlew installDebug   # Install on a connected device
```

### Server Discovery and Connection

On first launch:

1. The app attempts to connect to `http://localhost:8080` (works with ADB reverse proxy: `adb reverse tcp:8080 tcp:8080`).
2. If that fails, the login screen presents a **Server URL** field.
3. Enter your server address: `http://192.168.1.100:8080` (use your server's LAN IP, not `localhost`).
4. For emulators, use `http://10.0.2.2:8080`.
5. The server URL is persisted via DataStore across app restarts.

### Login and Authentication

1. Enter your **Username** and **Password**.
2. Tap **Sign In**.
3. Your JWT session token is stored securely on the device.
4. You remain logged in across app restarts until you explicitly sign out or the token expires.

### Browsing and Search on Mobile

**Home Screen:**

- **Recently Added**: Horizontally scrollable row of recently cataloged media cards showing title, year, rating.
- **Favorites**: Horizontally scrollable row of your favorited items.
- **Search icon** in the top bar navigates to the full-text search screen.

**Search:**

1. Tap the search icon.
2. Type your query -- results appear from both the server (online) and local cache (offline).
3. Tap a result to view details.

### Offline Mode and Background Sync

The Android app uses Room database for local caching and WorkManager for background sync:

- **Automatic caching**: Media items you browse are cached locally.
- **Offline browsing**: When offline, browse all previously cached content.
- **Queued operations**: Favorites, ratings, and watch progress are saved locally and synced when connectivity returns.
- **Sync settings**: Configure via Settings:
  - Offline Mode: On/Off
  - Auto Download: On/Off
  - Download Quality: 720p / 1080p / 4K
  - Wi-Fi Only: On/Off (default: On)
  - Storage Limit: configurable (default: 5 GB)

**Battery optimization:** For reliable background sync, set Catalogizer to "Unrestricted" in Android battery settings: Settings > Apps > Catalogizer > Battery > Unrestricted.

### Settings and Preferences

Access Settings via the gear icon on the Home screen:

- **About**: App name and version information
- **Sign Out**: Clears token and returns to login
- **Offline Statistics**: View cached items count, pending sync operations, and storage usage

<!-- Screenshot: Android phone showing the Home screen with a "Recently Added" horizontal row of movie cards and a "Favorites" row below. The top bar shows "Catalogizer" with search and settings icons -->

---

## Chapter 8: Android TV App

### Installation on TV Devices

**Supported devices:** Xiaomi Mi Box, NVIDIA Shield, any Android TV 8.0+ device.

**Sideloading via APK:**

1. Enable **Unknown Sources**: Settings > Security & restrictions > Unknown sources.
2. Transfer the APK via USB drive, cloud storage, or a sideload utility app.
3. Use a file manager on the TV to install the APK.
4. Find Catalogizer in your apps list.

**Via ADB:**

```bash
adb connect 192.168.0.214:5555          # Connect to your TV's IP
adb install catalogizer-androidtv.apk   # Install the APK
adb reverse tcp:8080 tcp:8080           # Set up reverse proxy for local server
```

### D-pad / Remote Navigation

The entire app is designed for 10-foot UI navigation with a TV remote:

| Button | Action |
|--------|--------|
| D-Pad Up/Down/Left/Right | Navigate between items and sections |
| Select / OK / Center | Confirm selection, open item, press button |
| Back | Return to previous screen or close overlay |
| Home | Return to Android TV home screen |
| Play/Pause | Toggle media playback |
| Fast Forward / Rewind | Seek forward/backward in the player |

**Focus behavior:** The currently focused item is visually highlighted with a border or scale effect. Focus moves horizontally within rows and vertically between sections.

### Home Screen Channels

The home screen is organized into horizontal content rails:

1. **Continue Watching** -- resume media where you left off (pressing Select starts playback directly)
2. **Recently Added** -- latest additions to your library
3. **Movies** -- all movies in your catalog
4. **TV Shows** -- all TV series
5. **Music** -- music items
6. **Documents** -- document-type media

Each section only appears if it contains items. The home screen uses parallel API calls for fast loading.

The app also integrates with the Android TV **Home Screen Channels** via `tvprovider`, allowing Catalogizer recommendations to appear on the Android TV launcher.

### Voice Search

If your remote has a voice button or microphone, you can use the Android TV voice search to find content. Search results from your Catalogizer library appear in the standard TV search interface.

### Media Playback on TV

The TV player uses ExoPlayer (Media3) with TV media session support:

- **Full-screen playback** with semi-transparent overlays for title and controls
- **Remote control**: Play/Pause, Fast Forward, Rewind, and D-pad scrubbing
- **Background audio**: Media session integration allows controlling playback from the remote even when overlays are hidden
- **Auto-play**: Playback starts automatically when entering the player

**Player settings (configurable in Settings):**

| Setting | Options |
|---------|---------|
| Auto-Play | On / Off |
| Streaming Quality | Auto / 720p / 1080p / 4K |
| Subtitles | On / Off |
| Subtitle Language | English, Spanish, etc. |

### Focus Navigation Tips

- If focus appears "stuck," press Back to return to a known location and re-navigate.
- On the Login screen, use D-pad to move between fields. Press Select/OK to open the on-screen keyboard.
- A USB or Bluetooth keyboard makes text entry significantly faster.
- The on-screen keyboard requires pressing Select on the Server URL field before typing.

<!-- Screenshot: Android TV home screen showing horizontal rails of movie posters. The "Recently Added" row is focused with a highlighted card showing a movie poster, title, and year. The top bar shows "Catalogizer" with Search and Settings buttons -->

---

## Chapter 9: Desktop App

### Installation

**Windows:**

1. Download the `.msi` installer from the releases page.
2. Run the installer and follow the prompts.
3. Catalogizer is added to your Start Menu.

**macOS:**

1. Download the `.dmg` file.
2. Open the DMG and drag Catalogizer to Applications.
3. On first launch, allow the app in **System Preferences > Security & Privacy**.

**Linux:**

1. Download the `.AppImage` or `.deb` package.
2. For AppImage: `chmod +x Catalogizer.AppImage && ./Catalogizer.AppImage`
3. For Debian/Ubuntu: `sudo dpkg -i catalogizer_*.deb`

**Building from source:**

```bash
cd catalogizer-desktop
npm install
npm run tauri:build
# Binary output: src-tauri/target/release/
```

### Installer Wizard Walkthrough

The Installer Wizard is a separate Tauri application specifically for configuring storage sources. See [Chapter 2](#chapter-2-storage-configuration) for the full six-step wizard flow.

Key wizard capabilities:
- Automatic network scanning for SMB servers
- Per-protocol configuration forms with validation
- Connection testing for every protocol
- Configuration file generation (JSON) for the catalog-api server
- Load and save configuration files

### Desktop App Features

The desktop app provides these pages:

| Page | Description |
|------|-------------|
| **Home** | Library overview with statistics, recent additions, quick access |
| **Library** | Full media browser with search, filters, grid/list views |
| **Search** | Dedicated full-text search interface |
| **Media Detail** | Comprehensive view of a single media entity |
| **Settings** | Server configuration, theme, storage management, auto-start |
| **Login** | Authentication screen |

### System Tray Functionality

The desktop app supports:

- **Auto-start**: Configure the app to launch automatically when your computer boots (toggle in Settings).
- **Background operation**: The Rust backend persists configuration and auth tokens using OS-level secure storage.
- **Platform detection**: The app detects your OS and architecture for optimal behavior.

### Server Connection Management

1. If no server URL is configured, the app redirects to Settings automatically.
2. Enter your **Server URL** (e.g., `http://localhost:8080` or `https://catalogizer.example.com`).
3. Click **Test** to verify connectivity -- a success or error message explains the result.
4. Click **Save Settings**.
5. You are redirected to Login.

The desktop app uses Tauri IPC commands to communicate between the React frontend and Rust backend:

| IPC Command | Purpose |
|-------------|---------|
| `get_config` / `update_config` | Read/write app configuration |
| `set_server_url` / `set_auth_token` | Granular config mutations |
| `make_http_request` | Proxied HTTP with SSRF protection |
| `get_app_version` / `get_platform` / `get_arch` | System information |

### Storage Configuration from Desktop

In Settings, scroll to the Storage Configuration section:

1. View configured storage sources with paths and dates.
2. Click **Add Storage Source** to expand the form.
3. Enter the path (e.g., `//nas-server/media`), optionally with credentials.
4. Click **Add Source** -- the server begins scanning the new source.

### Theme Selection

Choose between **Light**, **Dark**, or **System** (follows OS preference) themes in Settings. The change takes effect immediately.

<!-- Screenshot: Desktop app showing the Library page in dark theme. A sidebar shows navigation links (Home, Library, Search, Settings). The main area shows a grid of media cards with poster images. The search bar and filter controls are visible at the top -->

---

## Chapter 10: Administration

### User Management

Administrators can manage user accounts via the web admin panel or the REST API.

**API endpoints for user management:**

```
POST   /api/v1/users                  -- Create a new user
GET    /api/v1/users                  -- List all users
GET    /api/v1/users/:id              -- Get a specific user
PUT    /api/v1/users/:id              -- Update a user
DELETE /api/v1/users/:id              -- Delete a user
POST   /api/v1/users/:id/reset-password -- Reset a user's password
POST   /api/v1/users/:id/lock         -- Lock a user account
POST   /api/v1/users/:id/unlock       -- Unlock a user account
```

**From the web admin panel:**

1. Navigate to the Admin section (visible only to admin users).
2. Click **User Management** to see all registered users.
3. Use the **Create User** form to add new accounts.
4. Edit user roles, reset passwords, or deactivate accounts.

### Role-Based Access Control

Catalogizer uses role-based access with three tiers:

| Role | Capabilities |
|------|-------------|
| **Admin** | Full access: user management, storage configuration, system settings, all media operations |
| **User** | Browse, search, play, download, manage personal collections and favorites, manage playlists |
| **Viewer** | Browse and search media, view collections (read-only) |

**Role management API:**

```
POST   /api/v1/roles               -- Create a role
GET    /api/v1/roles               -- List all roles
GET    /api/v1/roles/:id           -- Get a specific role
PUT    /api/v1/roles/:id           -- Update a role
DELETE /api/v1/roles/:id           -- Delete a role
GET    /api/v1/roles/permissions   -- List all available permissions
```

### System Settings

View system information and health via the admin panel:

```
GET /api/v1/admin/system-info  -- Server version, uptime, resource usage
GET /api/v1/admin/storage      -- Storage source status and space usage
```

**Server configuration** is controlled via environment variables or `.env` file. Key settings:

```env
PORT=8080                   # Server port
GIN_MODE=release            # Production mode
DB_TYPE=postgres            # Database type
JWT_SECRET=<32+ chars>      # JWT signing secret
JWT_EXPIRATION=24h          # Token expiration
SESSION_TIMEOUT=30m         # Session timeout
MAX_LOGIN_ATTEMPTS=5        # Lockout after 5 failed attempts
LOCKOUT_DURATION=15m        # Lockout duration
```

See the [Configuration Reference](CONFIGURATION_REFERENCE.md) for all options.

### API Key Management

External metadata providers require API keys configured in the server environment:

| Variable | Provider | Purpose |
|----------|----------|---------|
| `TMDB_API_KEY` | TheMovieDB | Movie/TV metadata and poster art |
| `OMDB_API_KEY` | OMDB | Additional movie/TV metadata |

Obtain keys from:
- TMDB: https://www.themoviedb.org/settings/api
- OMDB: https://www.omdbapi.com/apikey.aspx

These are optional -- the system works without them but with less metadata enrichment.

### Log Management

Catalogizer provides comprehensive log management through the API:

```
POST /api/v1/logs/collect                    -- Create a log collection
GET  /api/v1/logs/collections                -- List collections
GET  /api/v1/logs/collections/:id            -- Get a collection
GET  /api/v1/logs/collections/:id/entries    -- Get log entries
POST /api/v1/logs/collections/:id/export     -- Export logs
GET  /api/v1/logs/collections/:id/analyze    -- Analyze logs
POST /api/v1/logs/share                      -- Create a shareable log link
GET  /api/v1/logs/stream                     -- Stream logs in real-time
GET  /api/v1/logs/statistics                 -- Get log statistics
```

**Log configuration:**

```env
LOG_LEVEL=info             # debug, info, warn, error, fatal
LOG_FORMAT=json            # json, text, console
LOG_OUTPUT=stdout          # stdout, stderr, file
LOG_FILE_PATH=logs/catalog-api.log
LOG_MAX_SIZE=100           # Max file size in MB before rotation
LOG_MAX_BACKUPS=5          # Number of old log files to keep
LOG_MAX_AGE=30             # Days to retain old log files
LOG_COMPRESS=true          # Compress rotated files
```

### Error Reporting

The error reporting system captures and tracks application errors:

```
POST /api/v1/errors/report             -- Report an error
POST /api/v1/errors/crash              -- Report a crash
GET  /api/v1/errors/reports            -- List error reports
GET  /api/v1/errors/statistics         -- Error statistics
GET  /api/v1/errors/health             -- System health summary
```

### Backup Management

Create and restore database backups through the admin API:

```
GET  /api/v1/admin/backups            -- List available backups
POST /api/v1/admin/backups            -- Create a new backup
POST /api/v1/admin/backups/:id/restore -- Restore from a backup
```

<!-- Screenshot: Admin panel showing User Management page with a table of users. Columns: Username, Email, Role, Status (Active/Locked), Last Login. Action buttons: Edit, Reset Password, Lock/Unlock, Delete -->

---

## Chapter 11: Monitoring

### Prometheus Metrics Overview

Catalogizer exposes metrics at `GET /metrics` in Prometheus exposition format. These metrics cover every aspect of the system:

**HTTP metrics:**
- `catalogizer_http_requests_total` -- total requests by method, path, status
- `catalogizer_http_request_duration_seconds` -- request duration histogram
- `catalogizer_http_active_connections` -- current active connections

**Database metrics:**
- `catalogizer_db_queries_total` -- queries by operation and table
- `catalogizer_db_query_duration_seconds` -- query duration histogram
- `catalogizer_db_connections_active` / `_idle` -- connection pool status

**Media processing metrics:**
- `catalogizer_media_files_scanned_total` -- total files scanned
- `catalogizer_media_files_analyzed_total` -- total files analyzed
- `catalogizer_media_by_type` -- media count by type

**Cache metrics:**
- `catalogizer_cache_hits_total` / `_misses_total` -- cache performance
- `catalogizer_cache_size_bytes` -- cache memory usage

**WebSocket metrics:**
- `catalogizer_websocket_connections_active` -- active connections
- `catalogizer_websocket_messages_total` -- messages by direction

**Runtime metrics:**
- `catalogizer_runtime_goroutines` -- goroutine count
- `catalogizer_runtime_memory_alloc_bytes` -- allocated memory
- `catalogizer_uptime_seconds` -- application uptime

### Grafana Dashboard Setup

1. Start Prometheus and Grafana (using Podman):

```bash
# Create prometheus.yml
cat > prometheus.yml << 'EOF'
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: 'catalogizer'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
EOF

# Start Prometheus
podman run -d --name prometheus --network host \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  docker.io/prom/prometheus

# Start Grafana
podman run -d --name grafana -p 3001:3000 \
  docker.io/grafana/grafana
```

2. Open Grafana at `http://localhost:3001` (default credentials: admin/admin).
3. Add a Prometheus data source: Configuration > Data Sources > Add > Prometheus > URL: `http://localhost:9090`.
4. Import the Catalogizer dashboard from `config/grafana-dashboards/catalogizer-overview.json`.

The dashboard displays: Request Rate, Uptime, HTTP Requests by Method, Response Time (P50/P95), Memory Usage, Connections (goroutines, DB, WebSocket), Media by Type, and Cache Hit Rate.

### Key Metrics to Monitor

| Metric | Healthy Range | Alert Threshold |
|--------|---------------|-----------------|
| HTTP P95 latency | < 200ms | > 1s for 5 minutes |
| Error rate | < 0.1% | > 10 errors/sec for 5 minutes |
| Memory usage | < 1 GB | > 1 GB for 5 minutes |
| DB connections active | < 20 | Pool exhausted (25/25) |
| Cache hit rate | > 70% | < 50% for 10 minutes |
| Goroutine count | < 500 | > 1000 (possible leak) |

### AlertManager Configuration

Create `alerts.yml` for Prometheus alerting:

```yaml
groups:
  - name: catalogizer_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(catalogizer_errors_total[5m]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"

      - alert: HighResponseTime
        expr: histogram_quantile(0.95, rate(catalogizer_http_request_duration_seconds_bucket[5m])) > 1
        for: 5m
        labels:
          severity: warning

      - alert: DatabaseDown
        expr: up{job="catalogizer"} == 0
        for: 1m
        labels:
          severity: critical

      - alert: HighMemoryUsage
        expr: catalogizer_runtime_memory_alloc_bytes > 1e9
        for: 5m
        labels:
          severity: warning
```

### Health Check Endpoints

Catalogizer provides four health endpoints for orchestration and monitoring:

| Endpoint | Purpose | When to Use |
|----------|---------|-------------|
| `GET /health` | Detailed health with component status | Dashboard monitoring |
| `GET /health/live` | Liveness probe (is the process alive?) | Kubernetes livenessProbe |
| `GET /health/ready` | Readiness probe (can it serve traffic?) | Kubernetes readinessProbe |
| `GET /health/startup` | Startup probe (has initialization completed?) | Kubernetes startupProbe |
| `GET /health/deep` | Deep health check with full diagnostics | Debugging |

Example response from `GET /health`:

```json
{
  "status": "healthy",
  "timestamp": "2026-04-03T10:30:00Z",
  "version": "2.2.0",
  "uptime": "2h15m30s",
  "components": {
    "database": {
      "status": "healthy",
      "latency": "2.5ms"
    }
  }
}
```

<!-- Screenshot: Grafana dashboard with 8 panels arranged in a 4x2 grid. Top row: Request Rate (line chart), Uptime counter, HTTP Requests by Method (stacked bar), Response Time P50/P95 (line chart). Bottom row: Memory Usage (area chart), Connections (multi-line), Media by Type (pie chart), Cache Hit Rate (gauge) -->

---

## Chapter 12: Troubleshooting

### Common Issues and Solutions

**Cannot log in with correct credentials:**

- Verify environment variables `ADMIN_USERNAME` and `ADMIN_PASSWORD` match what you are entering.
- Check if the account is locked (exceeded `MAX_LOGIN_ATTEMPTS`). Wait for `LOCKOUT_DURATION` (default 15 minutes) or unlock via `POST /api/v1/users/:id/unlock`.
- Ensure your device clock is accurate (JWT tokens are time-sensitive).

**Media not appearing after scan:**

- Confirm storage roots are in "connected" status: `GET /api/v1/storage-roots`.
- Check that files meet minimum size: `MIN_FILE_SIZE` (default 1 MB).
- Verify file extensions are in the supported lists (`SUPPORTED_VIDEO_EXTS`, `SUPPORTED_AUDIO_EXTS`, etc.).
- Check server logs for scan errors: `grep -i error logs/catalog-api.log`.

**WebSocket disconnections (real-time updates not working):**

- If behind a reverse proxy (Nginx), ensure WebSocket upgrade headers are forwarded:

```nginx
location /ws {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 86400;
}
```

- The web app includes automatic reconnection with exponential backoff.

**SMB source goes offline repeatedly:**

- Test manually: `smbclient -L //server -U username`
- Check SMB protocol version: some servers require SMB2 or SMB3.
- Review circuit breaker state via server logs.
- Force reconnection: `POST /api/v1/smb/sources/:id/reconnect`.

### Diagnostic Commands

```bash
# Check if the API server is running
curl http://localhost:8080/health

# Check detailed health status
curl http://localhost:8080/health/deep

# Verify authentication
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<ADMIN_PASSWORD>"}'

# Check storage root status
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/storage-roots

# View Prometheus metrics
curl http://localhost:8080/metrics

# Check system info (admin only)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/admin/system-info

# Test SMB connection
curl -X POST http://localhost:8080/api/v1/smb/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"host":"192.168.0.241","port":445,"share":"media","username":"user","password":"pass"}'
```

### Log Locations and Analysis

**Server logs:**

| Log Type | Location | Description |
|----------|----------|-------------|
| Application | `stdout` (default) or `LOG_FILE_PATH` | All application events |
| Access | Via Prometheus metrics | HTTP request records |
| Error | Filtered by `LOG_LEVEL` | Application errors |

```bash
# Follow real-time logs (when LOG_OUTPUT=file)
tail -f logs/catalog-api.log

# Filter errors
grep '"level":"error"' logs/catalog-api.log

# Stream logs via API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/logs/stream
```

**Frontend logs:**

Open browser DevTools (F12) and check the Console tab. Catalogizer enforces a zero-warning, zero-error policy -- any console error indicates a defect.

### Database Troubleshooting

**SQLite "database locked" errors:**

```bash
# Check for processes holding locks
fuser catalogizer.db

# Verify WAL mode is enabled
sqlite3 catalogizer.db "PRAGMA journal_mode;"
# Should return: wal

# Check database integrity
sqlite3 catalogizer.db "PRAGMA integrity_check;"

# Rebuild indexes
sqlite3 catalogizer.db "REINDEX;"

# Reclaim space
sqlite3 catalogizer.db "VACUUM;"
```

**PostgreSQL connection issues:**

```bash
# Test PostgreSQL connectivity
psql -h localhost -p 5433 -U catalogizer -d catalogizer -c "SELECT 1;"

# Check connection pool usage via metrics
curl http://localhost:8080/metrics | grep db_connections
```

**Database pool defaults:**

| Setting | Default | Environment Variable |
|---------|---------|---------------------|
| Max open connections | 25 | `DB_MAX_OPEN_CONNS` |
| Max idle connections | 10 | `DB_MAX_IDLE_CONNS` |
| Connection max lifetime | 5m | `DB_CONN_MAX_LIFETIME` |
| Connection max idle time | 3m | `DB_CONN_MAX_IDLE_TIME` |

### Network Connectivity Issues

**Port conflicts (especially port 3000):**

```bash
# Check what is using port 3000
ss -tlnp | grep :3000

# Kill the process if needed
kill $(lsof -t -i :3000)
```

**CORS errors in browser:**

Ensure `CORS_ORIGINS` includes your frontend URL:

```env
CORS_ORIGINS=http://localhost:3000,https://app.example.com
```

**Android app cannot connect:**

- Do not use `localhost` -- this refers to the Android device itself.
- Use the server's LAN IP address (e.g., `http://192.168.1.100:8080`).
- For emulators, use `http://10.0.2.2:8080`.
- For Android TV via ADB: set up reverse proxy with `adb reverse tcp:8080 tcp:8080`.
- Ensure the server allows cleartext HTTP from the device's IP (Android blocks cleartext by default on non-local addresses).

**Self-signed HTTPS certificates on Android:**

Android rejects self-signed certificates by default. Either:
- Install the CA certificate on the device: Settings > Security > Install from storage.
- Use plain HTTP for development.

### Getting Help

**Collect debug information before reporting an issue:**

```bash
# System info
uname -a

# Server health
curl http://localhost:8080/health

# Storage status
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/storage-roots

# Recent logs (last 100 lines)
tail -100 logs/catalog-api.log

# Container status (if running in containers)
podman ps --format "table {{.Names}} {{.Status}} {{.Ports}}"
podman stats --no-stream
```

**Useful resources:**

- [Configuration Reference](CONFIGURATION_REFERENCE.md) -- all environment variables and config options
- [Troubleshooting Guide](TROUBLESHOOTING.md) -- detailed protocol-specific troubleshooting
- [Monitoring Setup Guide](MONITORING_SETUP_GUIDE.md) -- Prometheus and Grafana setup
- [Web App Guide](WEB_APP_GUIDE.md) -- detailed web UI feature guide
- [Android Guide](ANDROID_GUIDE.md) -- Android mobile app details
- [Android TV Guide](ANDROID_TV_GUIDE.md) -- Android TV app details
- [Desktop Guide](DESKTOP_GUIDE.md) -- Desktop app details
- [Installer Wizard Guide](INSTALLER_WIZARD_GUIDE.md) -- storage configuration wizard
- [Development Setup Guide](DEVELOPMENT_SETUP.md) -- setting up a development environment

**Report issues** on the project's GitHub repository with the collected debug information, including server version, OS, and relevant log excerpts.

---

*Catalogizer v2.3.0 -- Vasic Digital*
