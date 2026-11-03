# Catalogizer Sequence Diagrams

Mermaid sequence diagrams for the key operational flows in the Catalogizer system.

## Rendered SVG Diagrams

| Diagram | SVG |
|---------|-----|
| Login Flow | ![Login](images/sequence-diagrams-1.svg) |
| Authenticated Request | ![Auth Request](images/sequence-diagrams-2.svg) |
| Token Refresh | ![Token Refresh](images/sequence-diagrams-3.svg) |
| Full Scan | ![Full Scan](images/sequence-diagrams-4.svg) |
| Media Detection | ![Media Detection](images/sequence-diagrams-5.svg) |
| WebSocket Events | ![WebSocket](images/sequence-diagrams-6.svg) |
| SSE Events | ![SSE](images/sequence-diagrams-7.svg) |
| Subtitle Search/Download | ![Subtitle Search](images/sequence-diagrams-8.svg) |
| Subtitle Sync | ![Subtitle Sync](images/sequence-diagrams-9.svg) |
| Subtitle Lifecycle | ![Subtitle Lifecycle](images/sequence-diagrams-10.svg) |
| Admin System Info | *not generated yet: Admin System Info* |
| Lazy Provider Search | *not generated yet: Lazy Provider Search* |
| Non-Blocking Health Check | *not generated yet: Non-Blocking Health Check* |

## Table of Contents

- [Authentication Flow](#authentication-flow)
- [File Scanning Flow](#file-scanning-flow)
- [Media Detection Pipeline](#media-detection-pipeline)
- [WebSocket Real-Time Updates](#websocket-real-time-updates)
- [Subtitle Management Flow](#subtitle-management-flow)
- [Admin System Info Flow](#admin-system-info-flow)
- [Lazy Provider Search Flow](#lazy-provider-search-flow)
- [Non-Blocking Health Check Flow](#non-blocking-health-check-flow)

---

## Authentication Flow

### Login with JWT Token

```mermaid
sequenceDiagram
    actor User
    participant Client as Client App
    participant Router as Gin Router
    participant AuthMW as Auth Middleware
    participant AuthH as Auth Handler
    participant AuthS as Auth Service
    participant UserR as User Repository
    participant DB as Database
    participant AuditLog as Auth Audit Log

    User->>Client: Enter credentials
    Client->>Router: POST /api/v1/auth/login
    Router->>AuthH: Handle login request
    AuthH->>AuthH: Validate request body

    AuthH->>AuthS: Authenticate(username, password)
    AuthS->>UserR: FindByUsername(username)
    UserR->>DB: SELECT * FROM users WHERE username = ?
    DB-->>UserR: User record
    UserR-->>AuthS: User object

    alt User not found or inactive
        AuthS->>AuditLog: Log failed_login event
        AuthS-->>AuthH: Error: invalid credentials
        AuthH-->>Client: 401 Unauthorized
    else User found
        AuthS->>AuthS: bcrypt.Compare(password, hash)
        alt Password mismatch
            AuthS->>DB: UPDATE users SET failed_login_attempts = +1
            AuthS->>AuditLog: Log failed_login event
            alt Max attempts exceeded
                AuthS->>DB: UPDATE users SET is_locked = 1, locked_until = ?
            end
            AuthS-->>AuthH: Error: invalid credentials
            AuthH-->>Client: 401 Unauthorized
        else Password matches
            AuthS->>AuthS: Generate JWT token (24h TTL)
            AuthS->>AuthS: Generate refresh token
            AuthS->>DB: INSERT INTO user_sessions (user_id, session_token, ...)
            AuthS->>DB: UPDATE users SET last_login_at = NOW(), failed_login_attempts = 0
            AuthS->>AuditLog: Log login_success event
            AuthS-->>AuthH: Token + User info
            AuthH-->>Client: 200 OK {token, refresh_token, user}
            Client->>Client: Store token in localStorage
        end
    end
```

### Authenticated Request Flow

```mermaid
sequenceDiagram
    participant Client as Client App
    participant Router as Gin Router
    participant AuthMW as Auth Middleware
    participant RateLimit as Rate Limiter
    participant Handler as Handler
    participant Service as Service
    participant DB as Database

    Client->>Router: GET /api/v1/catalog/files<br/>Authorization: Bearer <token>
    Router->>AuthMW: Extract and validate JWT

    alt No token provided
        AuthMW-->>Client: 401 Unauthorized
    else Token expired
        AuthMW-->>Client: 401 Token expired
    else Token valid
        AuthMW->>AuthMW: Parse claims (user_id, role, permissions)
        AuthMW->>RateLimit: Check rate limit for user

        alt Rate limit exceeded
            RateLimit-->>Client: 429 Too Many Requests
        else Within limit
            RateLimit->>Handler: Forward request with user context
            Handler->>Handler: Check required permissions
            alt Insufficient permissions
                Handler-->>Client: 403 Forbidden
            else Authorized
                Handler->>Service: Execute business logic
                Service->>DB: Query data
                DB-->>Service: Result
                Service-->>Handler: Response data
                Handler-->>Client: 200 OK {data}
            end
        end
    end
```

### Token Refresh Flow

```mermaid
sequenceDiagram
    participant Client as Client App
    participant Router as Gin Router
    participant AuthH as Auth Handler
    participant AuthS as Auth Service
    participant DB as Database

    Client->>Router: POST /api/v1/auth/refresh<br/>{refresh_token}
    Router->>AuthH: Handle refresh request

    AuthH->>AuthS: RefreshToken(refresh_token)
    AuthS->>DB: SELECT * FROM user_sessions WHERE refresh_token = ?
    DB-->>AuthS: Session record

    alt Session not found or expired
        AuthS-->>AuthH: Error: invalid refresh token
        AuthH-->>Client: 401 Unauthorized
    else Session valid
        AuthS->>AuthS: Generate new JWT token
        AuthS->>AuthS: Generate new refresh token
        AuthS->>DB: UPDATE user_sessions SET session_token = ?, refresh_token = ?
        AuthS-->>AuthH: New token pair
        AuthH-->>Client: 200 OK {token, refresh_token}
    end
```

---

## File Scanning Flow

### Full Scan Operation

```mermaid
sequenceDiagram
    participant Admin as Admin User
    participant API as Catalog API
    participant Scanner as Scanner Service
    participant FSClient as Filesystem Client
    participant Factory as Client Factory
    participant Protocol as Protocol Client<br/>(SMB/FTP/NFS/WebDAV/Local)
    participant DB as Database
    participant EventBus as Event Bus

    Admin->>API: POST /api/v1/catalog/scan {storage_root_id}
    API->>Scanner: StartScan(storage_root_id, "full")

    Scanner->>DB: SELECT * FROM storage_roots WHERE id = ?
    DB-->>Scanner: Storage root config

    Scanner->>DB: INSERT INTO scan_history (storage_root_id, scan_type, status, start_time)
    DB-->>Scanner: scan_id

    Scanner->>Factory: CreateClient(protocol, config)
    Factory-->>Scanner: Protocol-specific client

    Scanner->>Protocol: Connect()
    alt Connection failed
        Scanner->>DB: UPDATE scan_history SET status = 'failed', error_message = ?
        Scanner->>EventBus: Publish(scan_failed event)
        Scanner-->>API: Error: connection failed
        API-->>Admin: 500 Scan failed
    else Connected
        Scanner-->>API: 202 Accepted {scan_id}
        API-->>Admin: Scan started

        loop For each directory (BFS up to max_depth)
            Scanner->>Protocol: ListDirectory(path)
            Protocol-->>Scanner: File entries

            loop For each file entry
                Scanner->>DB: SELECT id FROM files WHERE storage_root_id = ? AND path = ?

                alt File exists in DB
                    Scanner->>Scanner: Compare modification time and size
                    alt Changed
                        Scanner->>DB: UPDATE files SET modified_at = ?, size = ?
                        Scanner->>Scanner: Increment files_updated counter
                    else Unchanged
                        Scanner->>DB: UPDATE files SET last_scan_at = NOW()
                    end
                else New file
                    Scanner->>Scanner: Detect MIME type and file_type
                    Scanner->>DB: INSERT INTO files (storage_root_id, path, name, ...)
                    Scanner->>Scanner: Increment files_added counter

                    opt enable_metadata_extraction
                        Scanner->>Protocol: ReadFileMetadata(path)
                        Protocol-->>Scanner: Metadata
                        Scanner->>DB: INSERT INTO file_metadata (file_id, key, value)
                    end

                    opt enable_duplicate_detection
                        Scanner->>Protocol: ReadPartialFile(path, first_64KB)
                        Protocol-->>Scanner: File bytes
                        Scanner->>Scanner: Compute quick_hash
                        Scanner->>DB: SELECT id FROM files WHERE quick_hash = ?
                        alt Duplicate found
                            Scanner->>DB: UPDATE files SET is_duplicate = 1, duplicate_group_id = ?
                        end
                    end
                end

                Scanner->>EventBus: Publish(file_discovered event)
            end
        end

        Scanner->>Scanner: Detect deleted files (not seen this scan)
        Scanner->>DB: UPDATE files SET deleted = 1, deleted_at = NOW() WHERE last_scan_at < scan_start

        Scanner->>DB: UPDATE scan_history SET status = 'completed', end_time = NOW(), files_processed = ?, ...
        Scanner->>DB: UPDATE storage_roots SET last_scan_at = NOW()
        Scanner->>EventBus: Publish(scan_completed event)
    end
```

---

## Media Detection Pipeline

### Content Detection and Metadata Enrichment

```mermaid
sequenceDiagram
    participant Scanner as File Scanner
    participant Detector as Media Detector
    participant Analyzer as Content Analyzer
    participant RuleEngine as Detection Rules
    participant DirAnalysis as Directory Analyzer
    participant DB as Media DB (SQLCipher)
    participant TMDB as TMDB API
    participant IMDB as OMDB API
    participant MBrainz as MusicBrainz API

    Scanner->>Detector: DetectMedia(file_path, file_info)

    Detector->>Detector: Extract filename components<br/>(title, year, quality, codec)

    Detector->>RuleEngine: MatchRules(filename, extension, directory_structure)
    RuleEngine->>DB: SELECT * FROM detection_rules WHERE enabled = 1 ORDER BY priority DESC
    DB-->>RuleEngine: Active rules

    loop For each detection rule
        RuleEngine->>RuleEngine: Apply pattern matching
        RuleEngine->>RuleEngine: Calculate confidence * weight
    end
    RuleEngine-->>Detector: Scored matches [{media_type, confidence}]

    Detector->>DirAnalysis: AnalyzeDirectory(parent_directory)
    DirAnalysis->>DirAnalysis: Count files by type
    DirAnalysis->>DirAnalysis: Analyze directory naming patterns
    DirAnalysis->>DirAnalysis: Check for NFO/metadata files
    DirAnalysis-->>Detector: Directory analysis result

    Detector->>Detector: Combine scores (hybrid method)
    Detector->>Detector: Select best media_type match

    alt confidence >= threshold (0.6)
        Detector->>Analyzer: EnrichMetadata(title, year, media_type)

        alt media_type is movie or tv_show
            Analyzer->>TMDB: Search(title, year)
            TMDB-->>Analyzer: Search results
            Analyzer->>TMDB: GetDetails(tmdb_id)
            TMDB-->>Analyzer: Full metadata

            Analyzer->>IMDB: Search(title, year)
            IMDB-->>Analyzer: IMDB data
        else media_type is music
            Analyzer->>MBrainz: Search(title, artist)
            MBrainz-->>Analyzer: Release data
        end

        Analyzer-->>Detector: Enriched metadata

        Detector->>DB: INSERT INTO media_items (media_type_id, title, year, ...)
        DB-->>Detector: media_item_id

        Detector->>DB: INSERT INTO external_metadata (media_item_id, provider, data, ...)

        Detector->>DB: INSERT INTO media_files (media_item_id, file_path, ...)

        Detector->>DB: INSERT OR REPLACE INTO directory_analysis (directory_path, media_item_id, confidence_score, ...)

    else confidence < threshold
        Detector->>DB: INSERT INTO directory_analysis (directory_path, confidence_score, detection_method)<br/>media_item_id = NULL
    end

    Detector-->>Scanner: Detection result
```

---

## WebSocket Real-Time Updates

### Real-Time Event Broadcasting

```mermaid
sequenceDiagram
    participant Client as Web Client
    participant WSClient as WebSocket Client<br/>(@vasic-digital/websocket-client)
    participant WSServer as WebSocket Server
    participant EventBus as Event Bus
    participant Watcher as Filesystem Watcher
    participant Scanner as Scanner Service
    participant DB as Database

    Note over Client,WSClient: Connection Establishment

    Client->>WSClient: useWebSocket(url, options)
    WSClient->>WSServer: WS Upgrade Request<br/>Authorization: Bearer <token>
    WSServer->>WSServer: Validate JWT token

    alt Token invalid
        WSServer-->>WSClient: 401 Unauthorized
        WSClient-->>Client: onError(auth_failed)
    else Token valid
        WSServer-->>WSClient: 101 Switching Protocols
        WSClient-->>Client: onConnect()
        WSClient->>WSServer: Subscribe(["scan.*", "file.*", "media.*"])
    end

    Note over Watcher,DB: File System Change Detected

    Watcher->>Watcher: Detect file change (debounced)
    Watcher->>EventBus: Publish({type: "file.created", path: "/videos/new.mp4"})
    Watcher->>DB: INSERT INTO change_log (entity_type, entity_id, change_type)

    EventBus->>WSServer: Broadcast to subscribers
    WSServer->>WSClient: {type: "file.created", data: {path, name, size}}
    WSClient->>Client: onMessage(event)
    Client->>Client: Update UI (React Query invalidation)

    Note over Scanner,DB: Scan Progress Updates

    Scanner->>EventBus: Publish({type: "scan.progress", progress: 45, files_processed: 1200})
    EventBus->>WSServer: Broadcast to subscribers
    WSServer->>WSClient: {type: "scan.progress", data: {progress: 45}}
    WSClient->>Client: onMessage(event)
    Client->>Client: Update progress bar

    Scanner->>EventBus: Publish({type: "scan.completed", total_files: 2650})
    EventBus->>WSServer: Broadcast to subscribers
    WSServer->>WSClient: {type: "scan.completed", data: {total: 2650}}
    WSClient->>Client: onMessage(event)
    Client->>Client: Refresh file list

    Note over WSClient,WSServer: Connection Recovery

    WSServer--xWSClient: Connection lost
    WSClient->>WSClient: Detect disconnect
    WSClient-->>Client: onDisconnect()

    loop Reconnection with exponential backoff
        WSClient->>WSClient: Wait (1s, 2s, 4s, 8s, ...)
        WSClient->>WSServer: WS Upgrade Request
        alt Server unavailable
            WSServer--xWSClient: Connection refused
        else Server available
            WSServer-->>WSClient: 101 Switching Protocols
            WSClient->>WSServer: Re-subscribe to topics
            WSClient-->>Client: onReconnect()
        end
    end
```

### SSE (Server-Sent Events) Alternative

```mermaid
sequenceDiagram
    participant Client as Client App
    participant API as Catalog API
    participant EventBus as Event Bus
    participant Stream as SSE Stream

    Client->>API: GET /api/v1/events/stream<br/>Accept: text/event-stream<br/>Authorization: Bearer <token>

    API->>API: Validate token
    API->>Stream: Create SSE connection

    loop Event streaming (keep-alive)
        EventBus->>Stream: New event available
        Stream->>Client: data: {"type": "file.created", "path": "/new.mp4"}\n\n

        Note over Stream,Client: Heartbeat every 30s
        Stream->>Client: : ping\n\n
    end

    Client->>Client: Close connection
    Stream->>Stream: Cleanup resources
```

---

## Subtitle Management Flow

### Search and Download Subtitles

```mermaid
sequenceDiagram
    participant User
    participant Client as Client App
    participant API as Catalog API
    participant SubH as Subtitle Handler
    participant SubS as Subtitle Service
    participant CacheS as Cache Service
    participant DB as Database
    participant OpenSub as OpenSubtitles API

    User->>Client: Search subtitles for movie
    Client->>API: GET /api/v1/subtitles/search?file_id=123&language=en

    API->>SubH: Handle search request
    SubH->>SubS: SearchSubtitles(file_id, language)

    SubS->>DB: SELECT * FROM files WHERE id = 123
    DB-->>SubS: File info (name, path, size)

    SubS->>SubS: Generate cache key from file + language

    SubS->>CacheS: Get(cache_key)
    CacheS->>DB: SELECT * FROM subtitle_cache WHERE cache_key = ? AND expires_at > NOW()

    alt Cache hit
        DB-->>CacheS: Cached results
        CacheS-->>SubS: Cached subtitle results
    else Cache miss
        SubS->>SubS: Extract title, year, season, episode from filename

        SubS->>OpenSub: Search(title, year, language, file_hash)
        OpenSub-->>SubS: Search results [{id, title, language, download_url, rating}]

        SubS->>SubS: Score and rank results by match_score

        SubS->>DB: INSERT INTO subtitle_cache (cache_key, result_id, provider, ...)
    end

    SubS-->>SubH: Ranked subtitle results
    SubH-->>Client: 200 OK [{id, title, language, rating, match_score}]
    Client-->>User: Display subtitle options

    Note over User,Client: User selects a subtitle

    User->>Client: Download subtitle #42
    Client->>API: POST /api/v1/subtitles/download {file_id: 123, result_id: "42"}

    API->>SubH: Handle download request
    SubH->>SubS: DownloadSubtitle(file_id, result_id)

    SubS->>DB: INSERT INTO subtitle_sync_status (media_item_id, subtitle_id, operation, status)<br/>VALUES (123, '42', 'download', 'in_progress')

    SubS->>DB: SELECT download_url FROM subtitle_cache WHERE result_id = '42'
    DB-->>SubS: Download URL

    SubS->>OpenSub: Download(download_url)
    OpenSub-->>SubS: Subtitle file content (.srt)

    SubS->>SubS: Detect encoding, validate format
    SubS->>SubS: Save to local filesystem

    SubS->>DB: INSERT INTO subtitle_tracks (media_item_id, language, language_code, format, path, ...)
    DB-->>SubS: subtitle_track_id

    SubS->>DB: INSERT INTO subtitle_downloads (media_item_id, result_id, subtitle_id, provider, ...)
    SubS->>DB: INSERT INTO media_subtitles (media_item_id, subtitle_track_id, is_active)

    SubS->>DB: UPDATE subtitle_sync_status SET status = 'completed', progress = 100

    SubS-->>SubH: Download complete
    SubH-->>Client: 200 OK {subtitle_track_id, path, language}
    Client-->>User: Subtitle downloaded and ready
```

### Subtitle Sync Verification

```mermaid
sequenceDiagram
    participant User
    participant Client as Client App
    participant API as Catalog API
    participant SubS as Subtitle Service
    participant DB as Database

    User->>Client: Verify subtitle sync for track #7
    Client->>API: POST /api/v1/subtitles/verify {subtitle_track_id: 7}

    API->>SubS: VerifySync(subtitle_track_id)

    SubS->>DB: SELECT * FROM subtitle_tracks WHERE id = 7
    DB-->>SubS: Subtitle track info

    SubS->>DB: INSERT INTO subtitle_sync_status<br/>(media_item_id, subtitle_id, operation, status)<br/>VALUES (?, '7', 'verify', 'in_progress')

    SubS->>SubS: Load subtitle file
    SubS->>SubS: Analyze timing patterns
    SubS->>SubS: Compare with video duration
    SubS->>SubS: Detect sync offset

    alt Sync is accurate
        SubS->>DB: UPDATE subtitle_tracks SET verified_sync = 1, sync_offset = 0.0
        SubS->>DB: UPDATE subtitle_sync_status SET status = 'completed'
        SubS-->>API: Sync verified, offset = 0.0
    else Sync offset detected
        SubS->>DB: UPDATE subtitle_tracks SET sync_offset = -2.5
        SubS->>DB: UPDATE subtitle_sync_status SET status = 'completed'
        SubS-->>API: Sync offset = -2.5s
    end

    API-->>Client: {verified: true, sync_offset: -2.5}
    Client-->>User: Subtitle sync: offset -2.5s applied

    opt User applies offset
        User->>Client: Apply sync offset
        Client->>API: PATCH /api/v1/subtitles/tracks/7 {sync_offset: -2.5}
        API->>SubS: UpdateSyncOffset(7, -2.5)
        SubS->>DB: UPDATE subtitle_tracks SET sync_offset = -2.5, verified_sync = 1
        SubS-->>API: Updated
        API-->>Client: 200 OK
    end
```

### Subtitle Lifecycle

```mermaid
sequenceDiagram
    participant Scanner as File Scanner
    participant Detector as Media Detector
    participant SubS as Subtitle Service
    participant DB as Database
    participant OpenSub as OpenSubtitles
    participant User as User

    Note over Scanner,SubS: Phase 1: Auto-detection during scan

    Scanner->>Detector: New video file detected
    Detector->>Detector: Check for embedded subtitles
    Detector->>Detector: Check for sidecar .srt/.ass files

    alt Embedded subtitles found
        Detector->>DB: INSERT INTO subtitle_tracks<br/>(source = 'embedded')
    end

    alt Sidecar subtitle files found
        loop For each .srt/.ass/.vtt file
            Detector->>DB: INSERT INTO subtitle_tracks<br/>(source = 'local', path = ?)
            Detector->>DB: INSERT INTO media_subtitles
        end
    end

    Note over SubS,OpenSub: Phase 2: User-initiated search

    User->>SubS: Search subtitles
    SubS->>OpenSub: Query available subtitles
    OpenSub-->>SubS: Results
    SubS->>DB: Cache results in subtitle_cache

    User->>SubS: Download selected subtitle
    SubS->>OpenSub: Download subtitle file
    OpenSub-->>SubS: Subtitle content
    SubS->>DB: INSERT INTO subtitle_tracks (source = 'downloaded')
    SubS->>DB: INSERT INTO subtitle_downloads
    SubS->>DB: INSERT INTO media_subtitles

    Note over SubS,DB: Phase 3: Cache maintenance

    SubS->>DB: DELETE FROM subtitle_cache WHERE expires_at < NOW()
```

---

## Admin System Info Flow

### Admin System Information Request

```mermaid
sequenceDiagram
    participant Client as Client App
    participant Router as Gin Router
    participant AuthMW as Auth Middleware
    participant AdminH as Admin Handler
    participant Runtime as Go Runtime

    Client->>Router: GET /api/v1/admin/system-info<br/>Authorization: Bearer <token>
    Router->>AuthMW: Validate JWT
    AuthMW->>AdminH: Forward with user context

    AdminH->>AdminH: Check admin permission
    alt Not admin role
        AdminH-->>Client: 403 Forbidden
    else Admin authorized
        AdminH->>+Runtime: Collect metrics
        Runtime-->>-AdminH: version, uptime, memory, goroutines
        AdminH->>AdminH: Build system info response
        AdminH-->>Client: 200 OK (system info JSON)
    end
```

---

## Lazy Provider Search Flow

### Concurrent Provider Search with BoundedSemaphore

```mermaid
sequenceDiagram
    participant Client as Client App
    participant PM as ProviderManager
    participant SEM as BoundedSemaphore
    participant TMDB as TMDB Provider
    participant OMDB as OMDB Provider
    participant MB as MusicBrainz Provider

    Client->>+PM: SearchAll(query)
    PM->>+SEM: Acquire (max 3)
    SEM-->>-PM: slot granted

    par Concurrent Provider Searches
        PM->>TMDB: Search(query)
        TMDB-->>PM: TMDB results
    and
        PM->>OMDB: Search(query)
        OMDB-->>PM: OMDB results
    and
        PM->>MB: Search(query)
        MB-->>PM: MusicBrainz results
    end

    PM->>PM: Merge and deduplicate results
    PM->>+SEM: Release
    SEM-->>-PM: slot freed
    PM-->>-Client: merged results
```

---

## Non-Blocking Health Check Flow

### Deep Health Check with Timeout

```mermaid
sequenceDiagram
    participant Client as Client App
    participant HH as Health Handler
    participant G as Goroutine
    participant DB as Database
    participant Redis as Redis
    participant FS as Filesystem

    Client->>+HH: GET /health/deep
    HH->>+G: RunChecks()

    par Health Check Probes
        G->>DB: Ping()
        DB-->>G: OK
    and
        G->>Redis: Ping()
        Redis-->>G: OK
    and
        G->>FS: Stat(root)
        FS-->>G: OK
    end

    alt Completes within 100ms
        G-->>HH: checks complete (all healthy)
        HH-->>Client: 200 OK {status: "healthy", checks: [...]}
    else Exceeds 100ms
        HH-->>Client: 200 OK {status: "degraded", message: "health check timeout"}
        G-->>-HH: (late result, discarded)
    end

    deactivate HH
```
