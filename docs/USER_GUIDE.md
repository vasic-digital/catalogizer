# Catalogizer -- User Guide

## Table of Contents

1. [Introduction](#introduction)
2. [Getting Started](#getting-started)
3. [Web Application](#web-application)
4. [Desktop Application](#desktop-application)
5. [Android Mobile App](#android-mobile-app)
6. [Android TV App](#android-tv-app)
7. [Media Management](#media-management)
8. [Search and Discovery](#search-and-discovery)
9. [Collections](#collections)
10. [Favorites](#favorites)
11. [Playlists](#playlists)
12. [Subtitle Management](#subtitle-management)
13. [Media Playback](#media-playback)
14. [Format Conversion](#format-conversion)
15. [Analytics and Insights](#analytics-and-insights)
16. [AI Dashboard](#ai-dashboard)
17. [Administration Panel](#administration-panel)
18. [Database Encryption and Key Rotation](#database-encryption-and-key-rotation)
19. [Troubleshooting](#troubleshooting)

---

## Introduction

Catalogizer is a multi-platform media collection manager that detects, categorizes, and organizes digital media across network storage protocols (SMB, FTP, NFS, WebDAV) and local filesystems. It consists of multiple client applications that connect to a central Go/Gin backend API:

- **catalog-web** -- React/TypeScript web frontend
- **catalogizer-desktop** -- Tauri (Rust + React) desktop app for Windows, macOS, and Linux
- **catalogizer-android** -- Kotlin/Jetpack Compose mobile app for Android phones and tablets
- **catalogizer-androidtv** -- Kotlin/Compose app optimized for Android TV

All clients communicate with **catalog-api**, the backend server that handles media scanning, metadata enrichment (via TMDB, IMDB, and other providers), user authentication, subtitle management, format conversion, and real-time event streaming via WebSocket.

### Key Capabilities

- Automatic detection and categorization of movies, TV shows, music, documentaries, anime, and other media types
- Multi-protocol storage scanning: SMB/CIFS, FTP, NFS, WebDAV, and local paths
- Metadata enrichment from TMDB, IMDB, and other external providers
- Subtitle search, download, upload, and sync verification across multiple providers
- Media format conversion with job queue management
- Smart collections with rule-based automatic population
- Playlist creation and management with shuffle and sequential playback
- Favorites with import/export support
- AI-powered insights, categorization, and natural language search
- Real-time library updates via WebSocket
- Offline mode with automatic sync (Android)
- JWT-based authentication with role-based access control

### System Requirements

| Client | Requirements |
|--------|-------------|
| Web Browser | Chrome 90+, Firefox 88+, Safari 14+, Edge 90+ |
| Desktop (Windows) | Windows 10 or later |
| Desktop (macOS) | macOS 11.0 or later |
| Desktop (Linux) | AppImage or .deb package compatible distribution |
| Android | Android 8.0 (API 26) or higher, 2 GB RAM minimum |
| Android TV | Android TV 8.0 or higher |

---

## Getting Started

### Quick Start (5 Minutes)

1. **Start the backend server.** If your administrator has already deployed the server, skip to step 3. Otherwise:

   ```bash
   cd catalog-api
   export JWT_SECRET="your-secret-key-at-least-32-chars"
   export ADMIN_USERNAME="admin"
   export ADMIN_PASSWORD="your-admin-password"
   go run main.go
   ```

2. **Start the web frontend** (for web access):

   ```bash
   cd catalog-web
   npm install
   npm run dev
   ```

3. **Open the web application** at `http://localhost:5173` (development) or the URL provided by your administrator.

4. **Log in** with your username and password. If this is a fresh installation, use the admin credentials configured during server setup.

5. **Configure storage sources** from the Admin panel. Add paths to your media directories (local paths, SMB shares, FTP servers, etc.).

6. **Trigger a library scan** from the Dashboard using the "Scan Library" quick action. Catalogizer will discover and catalog all media files from your configured storage sources.

7. **Browse your media** in the Media Browser. Items are automatically categorized by type (movies, TV shows, music, etc.) with metadata fetched from external providers.

### Account Setup

After your first login:

1. Navigate to your profile settings (top-right corner of the web app).
2. Change the default password if you are using one provided by an administrator.
3. Review your notification and display preferences.
4. Set your preferred theme (Light, Dark, or System).

---

## Web Application

The web application (`catalog-web`) is the most feature-rich Catalogizer client. It runs in any modern browser and provides access to all platform capabilities.

### Navigation

The main navigation bar at the top of the screen contains:

| Navigation Item | Description |
|----------------|-------------|
| Dashboard | Overview, statistics, system health, and quick actions |
| Media Browser | Browse, search, filter, and manage all media files |
| Collections | Create and manage themed groups of media items |
| Favorites | View and manage bookmarked media |
| Playlists | Create ordered playback sequences |
| Subtitles | Search, download, upload, and manage subtitles |
| Conversion | Convert media between formats |
| Analytics | Charts, statistics, and growth trends |
| AI Dashboard | AI-powered insights and tools |
| Admin | System administration (admin users only) |

### Dashboard

The Dashboard is the landing page after login. It displays:

**Statistics Cards:**
- Total Media Items in your library
- Active Users currently online
- Sessions Today (login count)
- Average Session Duration

**Media Distribution Chart:**
A pie chart showing the breakdown of your library by media type (movies, TV shows, music, documents, etc.).

**System Status Panel:**
Real-time indicators for CPU usage, memory usage, disk usage, network status, and server uptime. Color-coded bars (green/yellow/red) indicate health levels.

**Activity Feed:**
A chronological list of recent library events (new media detected, collections updated, files shared).

**Quick Actions:**
- Upload Media -- opens the upload interface
- Scan Library -- triggers a re-scan of all configured storage sources
- Search -- jumps to Media Browser with search focused
- Settings -- opens system settings

### Login and Session Management

1. Navigate to your Catalogizer URL.
2. Enter your username and password on the login screen.
3. Your session is maintained via a JWT token stored in the browser. Sessions persist across page refreshes.
4. The token automatically refreshes before expiration if you remain active.
5. Click "Logout" in the profile menu to end your session.

---

## Desktop Application

The desktop application (`catalogizer-desktop`) provides a native experience using Tauri (Rust backend + React frontend). It is available for Windows, macOS, and Linux.

### Installation

**Windows:** Download the `.msi` installer from the releases page and run it. Catalogizer is added to your Start Menu.

**macOS:** Download the `.dmg` file, open it, and drag Catalogizer to your Applications folder. On first launch, allow the app in System Preferences > Security & Privacy.

**Linux:** Download the `.AppImage` or `.deb` package. For AppImage, make it executable with `chmod +x Catalogizer.AppImage` and run it. For Debian/Ubuntu, install with `sudo dpkg -i catalogizer_*.deb`.

**Building from source:**
```bash
cd catalogizer-desktop
npm install
npm run tauri:build
```

### Initial Configuration

On first launch, you are redirected to the Settings page:

1. Enter your **Server URL** (e.g., `http://localhost:8080` or `https://catalogizer.example.com`).
2. Click **Test** to verify the connection.
3. Click **Save Settings**.
4. Log in with your credentials.

The desktop app stores its configuration (server URL, auth token, theme, auto-start preference) securely in the OS-specific application data directory using Tauri's secure storage.

### Features

- **Home Page** -- welcome screen with library statistics, recently added media, and navigation links
- **Library** -- full media browsing with search, type filter, sort options, and grid/list view toggle
- **Search** -- dedicated full-text search interface
- **Media Detail** -- comprehensive view of a single media item with metadata from external providers
- **Settings** -- server URL, theme (Light/Dark/System), storage source management, and auto-start toggle

### Storage Source Management

From Settings > Storage Configuration:

1. View all configured storage sources with their paths and dates added.
2. Click **Add Storage Source** to configure a new path.
3. Enter the storage path (e.g., `//nas-server/media` for SMB, or `/mnt/media` for local).
4. Optionally enter credentials for authenticated protocols.
5. Click **Add Source** to save.

---

## Android Mobile App

The Android app (`catalogizer-android`) is built with Kotlin and Jetpack Compose, following MVVM architecture. It features robust offline support.

### Installation

**Google Play Store:** Search for "Catalogizer" and install.

**APK Sideload:** Download the APK, enable "Install from unknown sources" in your device settings, and install the APK file.

**Requirements:** Android 8.0+ (API 26), 2 GB RAM minimum, network connectivity for initial setup.

### Login

1. Enter your Catalogizer server URL (e.g., `http://192.168.1.100:8080`).
2. Enter your username and password.
3. Tap **Sign In**.
4. Your session token is stored securely on the device for automatic re-authentication.

### Home Screen

The Home screen displays:

- **Recently Added** -- horizontally scrollable row of media cards showing new library additions
- **Favorites** -- horizontally scrollable row of your favorited items (shown if you have favorites)
- Search and Settings icons in the top bar

### Offline Mode

The Android app provides full offline support via a local Room database:

**When online:**
- Browsed media items are cached locally
- Search results are cached for offline access
- Favorites, ratings, and watch progress are stored locally

**When offline:**
- Browse all previously cached content
- Search returns results from local cache
- Actions (favorites, ratings) are queued as sync operations
- When connectivity returns, queued operations sync automatically

**Offline Settings:**

| Setting | Default | Description |
|---------|---------|-------------|
| Offline Mode | Off | Enables explicit offline mode with periodic sync |
| Auto Download | Off | Automatically download metadata for offline use |
| Download Quality | 1080p | Quality level for downloaded content |
| Wi-Fi Only | On | Only sync when connected to Wi-Fi |
| Storage Limit | 5 GB | Maximum local storage for cached content |

### Data Export and Import

For backup purposes, export your offline data (cached media, sync operations, search queries) to a JSON file. Import it later on the same or a different device.

---

## Android TV App

The Android TV app (`catalogizer-androidtv`) is optimized for the TV form factor with a lean-back interface designed for remote control navigation.

### Features

- Large card-based media browsing optimized for TV screens
- Remote-control-friendly navigation
- Media playback integration
- Favorites and watch progress tracking
- Same backend connectivity as the mobile app

---

## Media Management

### Browsing Media

The Media Browser is the primary interface for exploring your catalog.

**Stats Cards** at the top show:
- Total Items -- number of items in the library
- Media Types -- count of distinct categories
- Total Size -- combined storage size (GB)
- Recent Additions -- items added recently

**Search:** Type in the search bar to filter by title, description, or metadata. Search is debounced (300ms delay) for responsive results.

**Filters:** Click the Filters button to open the filter panel:
- Media type (movie, TV show, music, documentary, anime, etc.)
- Quality level
- Year range
- File size range
- Sort order (title, date updated, date added, year, rating, file size)
- Sort direction (ascending/descending)

Click **Reset** to clear all active filters.

**View Modes:**
- **Grid View** -- visual cards in a responsive grid layout
- **List View** -- compact rows with additional detail per item

**Pagination:** When your library exceeds the page limit (24 items by default), Previous/Next buttons and a page indicator appear.

### Uploading Media

1. In the Media Browser, click the **Upload** button (arrow-up icon).
2. The Upload Manager panel expands.
3. Drag and drop files or click to select from your filesystem.
4. Upload progress is shown for each file.
5. The media list refreshes automatically on completion.

### Viewing Media Details

Click any media item to open the **Media Detail Modal**:
- Title, year, and quality information
- Media type and file size
- Description and metadata from external providers (TMDB, IMDB)
- Action buttons: Play, Download, Close

### Downloading Media

Click the **Download** button on a media card or in the detail modal. A notification confirms when the download starts and completes.

---

## Search and Discovery

### Basic Search

All clients provide a search bar for full-text search across your media library. Results match against titles, descriptions, and metadata.

### Advanced Filtering (Web)

The web application provides advanced filtering:

- **Media Type** -- filter by movie, TV show, music, documentary, anime, etc.
- **Quality** -- filter by quality level (720p, 1080p, 4K, etc.)
- **Year Range** -- specify a range of release years
- **File Size** -- filter by file size range
- **Sort** -- order results by title, date, year, rating, or file size
- **Direction** -- ascending or descending

### Recommendations

The API provides recommendation endpoints:

- **Similar Items** -- find media similar to a specific item
- **Trending** -- see what is popular in the library
- **Personalized** -- recommendations based on your viewing history and preferences

### Duplicate Detection

The system automatically detects duplicate files across your storage sources. View duplicate statistics and groups from the Stats endpoints.

---

## Collections

Collections let you organize media items into themed groups.

### Collection Types

- **Manual Collections** -- manually add and remove items
- **Smart Collections** -- automatically populated based on rules you define (e.g., "all movies from 2024 with rating > 7")
- **Favorites** -- a built-in collection of your favorite items

### Creating a Smart Collection

1. Navigate to Collections in the web app.
2. Click the **Smart Collection** button.
3. Enter a name and description.
4. Define rules (conditions) that determine which media items are automatically included.
5. Click **Save**.

### Browsing Collections

- Use the tab bar to filter: All, Smart, Manual, Favorites, Templates, Automation, Integrations, AI Features
- Search collections by name
- Filter by media type (All, Music, Video, Images, Documents)
- Sort by name, date created, date updated, or item count
- Toggle between grid and list views

### Collection Actions

For each collection:
- **Preview** -- see collection contents
- **Share** -- share with other users (set view, comment, download permissions)
- **Duplicate** -- create a copy
- **Export** -- export as JSON, CSV, or M3U format
- **Settings** -- edit name, description, and rules
- **Analytics** -- view collection statistics
- **Real-Time Collaboration** -- enable live collaboration
- **Delete** -- permanently remove

### Bulk Operations

1. Select multiple collections using checkboxes.
2. Or use "Select all" to select all visible collections.
3. Click **Bulk Actions** for batch operations: delete, share, export, or duplicate.

---

## Favorites

The Favorites page provides a dedicated view for managing bookmarked media.

### Tabs

- **Favorites** -- current favorites in a filterable grid
- **Recently Added** -- favorites sorted by when you added them
- **Statistics** -- insights about your favorites (total count, breakdown by type, most common type, recent activity)

### Managing Favorites

- Click the heart/star icon on any media item to toggle favorite status
- Use bulk actions mode to select multiple favorites for batch operations
- **Import** -- load favorites from a previously exported file
- **Export** -- save your favorites list to a file for backup or sharing

### Favorites on Android

On the Android app, favorites work offline. When you favorite an item while offline, the change is queued and synced automatically when connectivity returns.

---

## Playlists

Playlists allow you to create ordered sequences of media items for playback.

### Creating a Playlist

1. Navigate to Playlists in the web app.
2. Click **Create Playlist**.
3. Enter a name and description.
4. Add media items by searching or browsing.

### Smart Playlists

Use the Smart Playlist Builder to define rules that automatically populate playlists based on criteria.

### Playback

- Play through playlist items in sequence
- Enable shuffle mode for randomized playback
- Drag and drop to reorder items within a playlist

---

## Subtitle Management

The Subtitle Manager lets you search, download, upload, and manage subtitles for your media files.

### Selecting a Media Item

1. Click **Select Media** to open the media search interface.
2. Type a title to search your library.
3. Click on a result to select it.

### Searching for Subtitles

1. With a media item selected, type a search query or use the pre-filled title.
2. Click **Filters** to refine:
   - **Language** -- select from supported languages
   - **Providers** -- toggle subtitle providers (e.g., OpenSubtitles, Subscene)
3. Click **Search** to fetch results.

### Search Results

Each result shows:
- Subtitle title
- Language
- Provider source
- Rating (if available)
- Release group
- Tags: HI (hearing impaired), Foreign Parts Only, Machine Translated

Click **Download** to download the subtitle and associate it with your selected media.

### Managing Existing Subtitles

When a media item is selected, the bottom section shows all associated subtitles:
- Language name
- Provider and format (SRT, SUB, etc.)
- Encoding information
- Sync offset (in milliseconds)
- Verification status

Actions per subtitle:
- **Verify Sync** -- opens the Subtitle Sync Modal to check and adjust timing
- **Delete** -- remove the subtitle

### Uploading Subtitles

Click **Upload Subtitle** to upload a subtitle file from your computer and associate it with the selected media.

### Subtitle Translation

The API supports subtitle translation. Submit a subtitle for translation to a target language.

### Supported Languages and Providers

Use the API endpoints to retrieve the full list of supported subtitle languages and providers:
- `GET /api/v1/subtitles/languages`
- `GET /api/v1/subtitles/providers`

---

## Media Playback

### Web Player

Click the **Play** button on a media card or from the detail modal to open the fullscreen Media Player. The player supports video and audio playback with standard controls:
- Play/pause
- Seek bar
- Volume control
- Fullscreen toggle
- Click **Close** to exit the player

### Watch Progress

The API tracks watch progress per user per media item. On the Android app, progress is stored locally and synced when online. The endpoint `PUT /api/v1/media/:id/progress` updates your position.

---

## Format Conversion

The Format Converter converts media files between different formats.

### Supported Formats

- **Video:** MP4, MKV, AVI, MOV, WebM
- **Audio:** MP3, WAV, FLAC

### Starting a Conversion

1. Navigate to Conversion in the web app.
2. Select a source file from your media library.
3. Choose the target output format.
4. Configure quality settings.
5. Click **Start Conversion**.

### Managing Conversion Jobs

The page displays all conversion jobs with:
- Job status: pending, in progress, completed, failed, cancelled
- Progress percentage for active jobs
- Source and target format information

Actions per job:
- **Cancel** -- stop an in-progress conversion
- **Retry** -- restart a failed conversion
- **Download** -- download the converted file when complete

The job list auto-refreshes every 30 seconds.

### API Endpoints

```
POST   /api/v1/conversion/jobs          -- create a new conversion job
GET    /api/v1/conversion/jobs          -- list all conversion jobs
GET    /api/v1/conversion/jobs/:id      -- get a specific job
POST   /api/v1/conversion/jobs/:id/cancel -- cancel a job
GET    /api/v1/conversion/formats       -- list supported formats
```

---

## Analytics and Insights

The Analytics page provides visual insights through interactive charts.

### Key Metrics

Four stat cards with mini trend charts:
- **Total Media Items** -- with month-over-month growth percentage
- **Storage Used** -- total disk usage (GB) with weekly change
- **Recent Additions** -- count of newly detected items
- **Media Types** -- number of distinct media categories

### Charts

- **Media Types Distribution** -- pie chart showing percentage breakdown by type
- **Quality Distribution** -- bar chart showing counts by quality level (720p, 1080p, 4K)
- **Collection Growth** -- area chart showing library growth over time
- **Weekly Activity** -- bar chart of daily additions for the current week

### Recently Added

A scrollable list of the 10 most recently added items, showing media type icon, title, year, type badge, quality, file size, and date added.

---

## AI Dashboard

The AI Dashboard provides AI-powered insights and tools:

- **AI Collection Suggestions** -- automated recommendations for new collections
- **Natural Language Search** -- search using conversational queries (e.g., "show me action movies from the 90s")
- **Content Quality Analysis** -- analyze media quality across your library
- **Content Categorizer** -- AI-powered categorization of media items
- **Behavior Analytics** -- understand usage patterns
- **Smart Organization** -- suggestions for reorganizing collections
- **Metadata Extraction** -- extract metadata from content using AI
- **Automation Rules** -- create AI-driven automation rules
- **Predictive Analytics** -- forecast library trends

---

## Administration Panel

The Admin panel is available to users with administrator privileges. Access it from the "Admin" link in the navigation bar.

### System Information

View server health metrics, version information, and uptime statistics.

### User Management

- View all registered users
- Create new user accounts
- Edit user roles and permissions
- Reset user passwords
- Lock and unlock accounts
- Delete user accounts

### Role Management

- Create custom roles with specific permissions
- List all available roles
- Edit role permissions
- Delete roles
- View the full list of available permissions

### Storage Management

- View configured storage sources and their status
- Add new storage sources (SMB, FTP, NFS, WebDAV, Local)
- Test storage connections
- Remove storage sources

### Configuration Management

- View current system configuration
- Test configuration changes before applying
- Use the configuration wizard for guided setup
- View system status

### Error and Crash Reporting

- View error reports with filtering and status management
- View crash reports
- Update report statuses (open, investigating, resolved)
- View error and crash statistics
- Monitor system health

### Log Management

- Create log collections from specified time ranges
- View and filter log entries
- Export logs for external analysis
- Analyze log patterns
- Share logs with other users via secure tokens
- Stream logs in real-time
- View log statistics

### Backup Management

- View available backups
- Create new backups
- Restore from a backup

---

## Database Encryption and Key Rotation

The main SQLite database is encrypted at rest (SQLCipher). The key is generated
on first run and stored in a mode-0600 file outside the repository
(`DATABASE_ENCRYPTION_KEY_FILE`, default `~/.config/catalogizer/db.key`).
Rotate it with `catalog-api db-rekey` (server stopped; a backup is taken first).
Back the key file up separately: without it the database and its backups cannot
be opened. Full details and FAQ: [DATABASE_ENCRYPTION.md](DATABASE_ENCRYPTION.md).

## Troubleshooting

### Login Issues

**Problem:** "Invalid username or password"
- Verify your username spelling (case-sensitive).
- Check that Caps Lock is not enabled.
- Use the password reset feature if available.
- Clear browser cache and cookies, then try again.
- Try an incognito/private browsing window.

**Problem:** Session expired unexpectedly
- JWT tokens expire after the configured duration (default 24 hours).
- Log in again to obtain a new token.
- If this happens frequently, contact your administrator about session duration settings.

### Connection Issues

**Problem:** Cannot connect to the server
- Verify the server URL is correct (include `http://` or `https://`).
- Check that the server is running: `curl http://your-server:8080/health` should return `{"status":"healthy"}`.
- Verify firewall rules allow connections on the server port (default 8080).
- If using HTTPS, ensure the SSL certificate is valid and trusted.

**Problem:** WebSocket connection fails
- WebSocket requires the server to support the `Upgrade` header.
- If behind a reverse proxy (nginx), ensure WebSocket proxying is configured.
- Check that the browser is not blocking WebSocket connections.

### Media Not Appearing

**Problem:** Library scan does not find media files
- Verify storage sources are correctly configured in the Admin panel.
- For SMB shares, ensure credentials are correct and the share is accessible.
- Check file permissions on local paths.
- Verify supported file types are present (media files with common extensions).
- Trigger a manual re-scan from the Dashboard.

**Problem:** Metadata is missing or incorrect
- Metadata is fetched from external providers (TMDB, IMDB). Ensure API keys are configured if required.
- Some media files may not match any external database entry.
- File naming conventions affect detection accuracy. Consistent naming improves recognition.

### Upload Problems

**Problem:** Upload fails
- Check file size against the server's maximum (default: 5 GB archive, 1 MB chunk size).
- Verify the file format is supported.
- Test network connectivity.
- Clear browser cache and retry.
- Try uploading from a different browser.

**Problem:** Slow upload speeds
- Use a wired connection when possible.
- Upload during off-peak hours.
- Close other bandwidth-intensive applications.
- Upload files in smaller batches.

### Desktop App Issues

**Problem:** Blank white screen
- Check for JavaScript errors in the developer console (if accessible).
- Ensure the app is up to date.
- Clear app data and reconfigure.

**Problem:** Auto-start not working
- Verify the toggle is enabled in Settings.
- Linux: check `~/.config/autostart/` for the desktop entry.
- macOS: check System Preferences > Users & Groups > Login Items.
- Windows: check Task Manager > Startup tab.

### Android App Issues

**Problem:** App crashes on launch
- Ensure the device runs Android 8.0 or higher.
- Clear app data: Settings > Apps > Catalogizer > Storage > Clear Data.
- Update to the latest app version.
- If the issue persists, uninstall and reinstall.

**Problem:** Sync failures
- Check offline stats for failed sync operations.
- Verify the server is reachable.
- Force a manual sync when connectivity is stable.
- Log out and back in to refresh the authentication token.

**Problem:** Slow performance
- Clear app cache: Settings > Apps > Catalogizer > Storage > Clear Cache.
- Trigger cache cleanup if the local database has grown large.
- Ensure sufficient free storage on the device.

### Conversion Issues

**Problem:** Conversion job fails
- Check the job status for error details.
- Verify the source file is accessible and not corrupted.
- Ensure the target format is supported.
- Check server resources (CPU, memory, disk space).
- Retry the conversion.

### Subtitle Issues

**Problem:** No subtitle results found
- Try different search terms (exact movie title, year).
- Check that the selected language is correct.
- Try enabling additional subtitle providers.
- Verify network connectivity to external subtitle services.

**Problem:** Subtitle sync is off
- Use the Verify Sync feature to check timing.
- Adjust the sync offset in milliseconds.
- Try a different subtitle file from search results.

### Browser Compatibility

| Browser | Minimum Version | Recommended |
|---------|----------------|-------------|
| Chrome | 90+ | Latest |
| Firefox | 88+ | Latest |
| Safari | 14+ | Latest |
| Edge | 90+ | Latest |

### Getting Help

If you encounter issues not covered here:

1. Check the server logs: `journalctl -u catalogizer` (Linux systemd) or the log file at the configured path.
2. Use the built-in diagnostics at `/admin/diagnostics` (web app, admin users).
3. Check the health endpoint: `GET /health`.
4. Review the error reporting system in the Admin panel.
5. Consult the [Admin Guide](ADMIN_GUIDE.md) for server-side troubleshooting.
6. Consult the [Developer Guide](DEVELOPER_GUIDE.md) for debugging information.

---

*This guide covers Catalogizer across all platforms. For platform-specific details, see also: [Web App Guide](guides/WEB_APP_GUIDE.md), [Desktop Guide](guides/DESKTOP_GUIDE.md), and [Android Guide](guides/ANDROID_GUIDE.md). For server administration, see the [Admin Guide](ADMIN_GUIDE.md).*
