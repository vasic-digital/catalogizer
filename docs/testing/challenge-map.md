# Challenge Map

Complete listing of all registered challenges organized by category, including dependency chains.

## Original Challenges (CH-001 to CH-060)

| Range | Category | Count |
|-------|----------|-------|
| CH-001 to CH-010 | SMB Connectivity & Scanning | ~10 (per-endpoint) |
| CH-011 to CH-015 | Browsing, Assets, Database | 7 |
| CH-016 to CH-020 | Entity System | 5 |
| CH-021 to CH-025 | Module Integration | 5 |
| CH-026 to CH-035 | Extended Validation | 10 |
| CH-036 to CH-040 | Security Validation | 5 |
| CH-041 to CH-044 | Performance Regression | 4 |
| CH-045 to CH-047 | Documentation Completeness | 3 |
| CH-048 to CH-050 | System Resilience | 3 |
| CH-051 to CH-060 | Extended API Validation | 10 |

---

## Feature & Security Challenges (CH-061 to CH-088)

### Feature Validation (CH-061 to CH-068)

| ID | Name | Category | Validates |
|----|------|----------|-----------|
| CH-061 | Search API Basic Query | feature | GET /api/v1/search?q=test |
| CH-062 | Search API Duplicate Detection | feature | GET /api/v1/entities/duplicates |
| CH-063 | Search API Advanced Filters | feature | Filtered search with type+year |
| CH-064 | Browse API Storage Roots | feature | GET /api/v1/storage-roots |
| CH-065 | Browse API Directory Listing | feature | GET /api/v1/files?path=/ |
| CH-066 | Sync API Endpoint Creation | feature | POST /api/v1/sync/endpoints |
| CH-067 | Sync API Cloud Providers | feature | GET /api/v1/sync/providers |
| CH-068 | Sync API User Endpoints | feature | GET /api/v1/sync/endpoints |

### Security Validation (CH-069 to CH-075)

| ID | Name | Category | Validates |
|----|------|----------|-----------|
| CH-069 | Security Headers All Responses | security | X-Content-Type-Options, X-Frame-Options |
| CH-070 | CORS Rejects Unauthorized Origins | security | Bad Origin header rejected |
| CH-071 | Input Validation Rejects Injection | security | SQL injection payloads blocked |
| CH-072 | Rate Limit Auth Endpoints | security | 429 after rapid auth attempts |
| CH-073 | JWT Token Lifecycle | security | Login, use, invalid token = 401 |
| CH-074 | File Upload Magic Bytes | security | Wrong extension rejection |
| CH-075 | Conversion Rejects Path Traversal | security | ../../etc/passwd blocked |

### Performance & Resilience (CH-076 to CH-083)

| ID | Name | Category | Validates |
|----|------|----------|-----------|
| CH-076 | API Response Latency | performance | avg < 100ms for health |
| CH-077 | API Concurrent Requests | performance | 50 concurrent GET /health |
| CH-078 | Graceful Degradation | resilience | No 500s under load |
| CH-079 | Memory Stable During Load | resilience | Health OK before and after load |
| CH-080 | DB Pool Recovery | resilience | Recovery after heavy DB usage |
| CH-081 | WebSocket Reconnection | resilience | WS connect/disconnect/reconnect |
| CH-082 | Lazy Init On First Request | performance | First request triggers init |
| CH-083 | Semaphore Prevents Overload | performance | Rate limiting under concurrency |

### Monitoring (CH-084 to CH-088)

| ID | Name | Category | Validates |
|----|------|----------|-----------|
| CH-084 | Prometheus Metrics Endpoint | monitoring | GET /metrics has go_ metrics |
| CH-085 | HTTP Request Metrics Increment | monitoring | http_requests counter |
| CH-086 | Runtime Metrics Current | monitoring | go_goroutines > 0 |
| CH-087 | DB Query Duration Tracked | monitoring | db_query_duration metric |
| CH-088 | Grafana Dashboard Renders | monitoring | Dashboard config exists |

---

## Module Verification (MOD-001 to MOD-021)

### Documentation Checks (MOD-001 to MOD-015)

| ID | Module | Checks |
|----|--------|--------|
| MOD-001 | Database | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-002 | Concurrency | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-003 | Observability | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-004 | Security | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-005 | Middleware | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-006 | Media | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-007 | Discovery | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-008 | Streaming | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-009 | Lazy | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-010 | Memory | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-011 | Recovery | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-012 | Storage | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-013 | Cache | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-014 | Watcher | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |
| MOD-015 | RateLimiter | ARCHITECTURE.md, CLAUDE.md, tests, go.mod |

### Functional Checks (MOD-016 to MOD-021)

| ID | Module | Checks |
|----|--------|--------|
| MOD-016 | Lazy | Value[T] with Get, MustGet, Reset + sync.Mutex |
| MOD-017 | Lazy | Service[T] with Get, Initialized + sync.Once |
| MOD-018 | Recovery | CircuitBreaker state transitions (closed/open) |
| MOD-019 | Recovery | HealthChecker verification |
| MOD-020 | Memory | LeakDetector tracking |
| MOD-021 | Memory | KnowledgeGraph BFS |

---

## User Flow Challenges (174 + 2 environment)

Complete listing of all 174 user flow challenges (plus 2 environment bookends) organized by platform and category, including dependency chains.

## Environment (2 challenges)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ENV-SETUP | Environment Setup | (none -- root) |
| UF-ENV-TEARDOWN | Environment Teardown | (all others) |

---

## API Platform (49 challenges)

### API Health (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-HEALTH | API Health Check | UF-ENV-SETUP |
| UF-API-HEALTH-VERSION | API Version Info | UF-API-HEALTH |
| UF-API-HEALTH-METRICS | API Metrics Endpoint (unauthenticated scrape must be rejected: 401) | UF-API-HEALTH |

### API Auth (5)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-AUTH-LOGIN | API Auth Login | UF-API-HEALTH |
| UF-API-AUTH-REGISTER | API Auth Register | UF-API-AUTH-LOGIN |
| UF-API-AUTH-INVALID | API Auth Invalid Credentials | UF-API-HEALTH |
| UF-API-AUTH-TOKEN-REFRESH | API Auth Token Refresh | UF-API-AUTH-LOGIN |
| UF-API-AUTH-LOGOUT | API Auth Logout | UF-API-AUTH-LOGIN |

### API Media (10)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-MEDIA-LIST | API Media List | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-GET | API Media Get By ID | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-SEARCH | API Media Search | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-TYPES | API Media Types | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-ENTITY | API Media Entity Details | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-HIERARCHY | API Media Entity Hierarchy | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-COVER | API Media Cover Art | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-METADATA | API Media External Metadata | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-RECENT | API Media Recently Added | UF-API-AUTH-LOGIN |
| UF-API-MEDIA-STATS | API Media Statistics | UF-API-AUTH-LOGIN |

### API Collections (5)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-COLL-LIST | API Collections List | UF-API-AUTH-LOGIN |
| UF-API-COLL-CREATE | API Collection Create | UF-API-COLL-LIST |
| UF-API-COLL-ADD | API Collection Add Item | UF-API-COLL-CREATE |
| UF-API-COLL-REMOVE | API Collection Remove Item | UF-API-COLL-ADD |
| UF-API-COLL-DELETE | API Collection Delete | UF-API-COLL-CREATE |

### API Storage (5)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-STORAGE-LIST | API Storage List Roots | UF-API-AUTH-LOGIN |
| UF-API-STORAGE-ADD | API Storage Add Root | UF-API-STORAGE-LIST |
| UF-API-STORAGE-SCAN | API Storage Trigger Scan | UF-API-STORAGE-LIST |
| UF-API-STORAGE-STATUS | API Storage Scan Status | UF-API-STORAGE-LIST |
| UF-API-STORAGE-FILES | API Storage List Files | UF-API-STORAGE-LIST |

### API Admin (5)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-ADMIN-USERS | API Admin List Users | UF-API-AUTH-LOGIN |
| UF-API-ADMIN-CONFIG | API Admin Configuration | UF-API-AUTH-LOGIN |
| UF-API-ADMIN-LOGS | API Admin Log Collections | UF-API-AUTH-LOGIN |
| UF-API-ADMIN-STATS | API Admin System Statistics | UF-API-AUTH-LOGIN |
| UF-API-ADMIN-SESSIONS | API Admin Active Sessions | UF-API-AUTH-LOGIN |

### API Downloads (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-DL-REQUEST | API Download Request | UF-API-AUTH-LOGIN |
| UF-API-DL-STATUS | API Download Status | UF-API-AUTH-LOGIN |
| UF-API-DL-STREAM | API Media Stream | UF-API-AUTH-LOGIN |

### API Favorites (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-FAV-ADD | API Favorites Add | UF-API-AUTH-LOGIN |
| UF-API-FAV-LIST | API Favorites List | UF-API-FAV-ADD |
| UF-API-FAV-REMOVE | API Favorites Remove | UF-API-FAV-ADD |

### API WebSocket (2)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-WS-CONNECT | API WebSocket Connect | UF-API-AUTH-LOGIN |
| UF-API-WS-EVENTS | API WebSocket Events | UF-API-WS-CONNECT |

### API Error Handling (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-ERR-404 | API Error 404 Not Found | UF-API-AUTH-LOGIN |
| UF-API-ERR-401 | API Error 401 Unauthorized | UF-API-HEALTH |
| UF-API-ERR-400 | API Error 400 Bad Request | UF-API-AUTH-LOGIN |

### API Security (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-API-SEC-CORS | API Security CORS Headers | UF-API-AUTH-LOGIN |
| UF-API-SEC-RATE | API Security Rate Limiting | UF-API-AUTH-LOGIN |
| UF-API-SEC-HEADERS | API Security Headers | UF-API-AUTH-LOGIN |

---

## Web Platform (59 challenges)

### Web Auth (5)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-AUTH-LOGIN | Web Auth Login | UF-API-HEALTH |
| UF-WEB-AUTH-REGISTER | Web Auth Register | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-AUTH-INVALID | Web Auth Invalid Credentials | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-AUTH-LOGOUT | Web Auth Logout | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-AUTH-PERSIST | Web Auth Persist | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Dashboard (5)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-DASH-LOAD | Web Dashboard Load | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-DASH-STATS | Web Dashboard Stats | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-DASH-CHARTS | Web Dashboard Charts | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-DASH-ACTIVITY | Web Dashboard Activity | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-DASH-NAV | Web Dashboard Navigation | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Media Browser (8)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-BROWSE-LOAD | Web Browse Load | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-BROWSE-SEARCH | Web Browse Search | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-BROWSE-FILTER | Web Browse Filter | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-BROWSE-DETAIL | Web Browse Detail | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-BROWSE-PAGINATION | Web Browse Pagination | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-BROWSE-SORT | Web Browse Sort | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-BROWSE-GRID | Web Browse Grid View | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-BROWSE-EMPTY | Web Browse Empty State | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Collections (6)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-COLL-LIST | Web Collections List | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-COLL-CREATE | Web Collection Create | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-COLL-ADD | Web Collection Add Item | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-COLL-REMOVE | Web Collection Remove Item | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-COLL-DELETE | Web Collection Delete | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-COLL-SEARCH | Web Collection Search | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Player (4)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-PLAYER-LOAD | Web Player Load | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-PLAYER-CONTROLS | Web Player Controls | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-PLAYER-SUBTITLE | Web Player Subtitle | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-PLAYER-FULLSCREEN | Web Player Fullscreen | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Admin (5)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-ADMIN-LOAD | Web Admin Load | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-ADMIN-USERS | Web Admin Users | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-ADMIN-CONFIG | Web Admin Config | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-ADMIN-LOGS | Web Admin Logs | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-ADMIN-STATS | Web Admin Stats | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Subtitles (4)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-SUB-SEARCH | Web Subtitle Search | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-SUB-DOWNLOAD | Web Subtitle Download | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-SUB-UPLOAD | Web Subtitle Upload | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-SUB-SYNC | Web Subtitle Sync | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Conversion (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-CONV-LOAD | Web Conversion Load | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-CONV-FORMATS | Web Conversion Formats | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-CONV-CREATE | Web Conversion Create | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Analytics (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-ANALYTICS-LOAD | Web Analytics Load | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-ANALYTICS-CHARTS | Web Analytics Charts | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-ANALYTICS-FILTERS | Web Analytics Filters | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Favorites (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-FAV-ADD | Web Favorites Add | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-FAV-LIST | Web Favorites List | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-FAV-REMOVE | Web Favorites Remove | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Playlists (4)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-PLAYLIST-LIST | Web Playlist List | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-PLAYLIST-CREATE | Web Playlist Create | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-PLAYLIST-ADD | Web Playlist Add Item | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-PLAYLIST-PLAY | Web Playlist Play | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Responsive (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-RESP-MOBILE | Web Responsive Mobile | UF-API-HEALTH |
| UF-WEB-RESP-TABLET | Web Responsive Tablet | UF-API-HEALTH |
| UF-WEB-RESP-DESKTOP | Web Responsive Desktop | UF-API-HEALTH |

### Web Error Handling (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-ERR-404 | Web Error 404 | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-ERR-NETWORK | Web Error Network | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-ERR-BOUNDARY | Web Error Boundary | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

### Web Accessibility (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WEB-A11Y-KEYBOARD | Web Accessibility Keyboard | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-A11Y-ARIA | Web Accessibility ARIA | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |
| UF-WEB-A11Y-CONTRAST | Web Accessibility Contrast | UF-API-HEALTH, UF-WEB-AUTH-LOGIN |

---

## Desktop Platform (18 challenges)

### Desktop Build (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-DESKTOP-BUILD | Desktop Build | (none) |
| UF-DESKTOP-TEST | Desktop Unit Tests | UF-DESKTOP-BUILD |
| UF-DESKTOP-LINT | Desktop Lint | UF-DESKTOP-BUILD |

### Desktop Launch (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-DESKTOP-LAUNCH | Desktop Launch | UF-DESKTOP-BUILD |
| UF-DESKTOP-STABLE | Desktop Stability | UF-DESKTOP-BUILD |
| UF-DESKTOP-SCREENSHOT | Desktop Screenshot | UF-DESKTOP-LAUNCH |

### Desktop Auth (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-DESKTOP-AUTH-LOGIN | Desktop Auth Login | UF-DESKTOP-LAUNCH |
| UF-DESKTOP-AUTH-PERSIST | Desktop Auth Persist | UF-DESKTOP-AUTH-LOGIN |
| UF-DESKTOP-AUTH-LOGOUT | Desktop Auth Logout | UF-DESKTOP-AUTH-LOGIN |

### Desktop Browse (4)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-DESKTOP-BROWSE-LOAD | Desktop Browse Load | UF-DESKTOP-AUTH-LOGIN |
| UF-DESKTOP-BROWSE-SEARCH | Desktop Browse Search | UF-DESKTOP-BROWSE-LOAD |
| UF-DESKTOP-BROWSE-DETAIL | Desktop Browse Detail | UF-DESKTOP-BROWSE-LOAD |
| UF-DESKTOP-BROWSE-FILTER | Desktop Browse Filter | UF-DESKTOP-BROWSE-LOAD |

### Desktop IPC (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-DESKTOP-IPC-VERSION | Desktop IPC Version | UF-DESKTOP-LAUNCH |
| UF-DESKTOP-IPC-CONFIG | Desktop IPC Config | UF-DESKTOP-LAUNCH |
| UF-DESKTOP-IPC-SETTINGS | Desktop IPC Settings | UF-DESKTOP-LAUNCH |

### Desktop Settings (2)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-DESKTOP-SETTINGS-LOAD | Desktop Settings Load | UF-DESKTOP-AUTH-LOGIN |
| UF-DESKTOP-SETTINGS-SAVE | Desktop Settings Save | UF-DESKTOP-SETTINGS-LOAD |

---

## Wizard Platform (10 challenges)

### Wizard Build (2)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WIZARD-BUILD | Wizard Build | (none) |
| UF-WIZARD-TEST | Wizard Unit Tests | UF-WIZARD-BUILD |

### Wizard Flow (5)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WIZARD-LAUNCH | Wizard Launch | UF-WIZARD-BUILD |
| UF-WIZARD-WELCOME | Wizard Welcome Screen | UF-WIZARD-LAUNCH |
| UF-WIZARD-PROTOCOL | Wizard Protocol Selection | UF-WIZARD-WELCOME |
| UF-WIZARD-SERVER | Wizard Server Details | UF-WIZARD-PROTOCOL |
| UF-WIZARD-COMPLETE | Wizard Complete | UF-WIZARD-SERVER |

### Wizard Validation (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-WIZARD-VALIDATE-EMPTY | Wizard Validate Empty Form | UF-WIZARD-LAUNCH |
| UF-WIZARD-VALIDATE-IP | Wizard Validate Invalid IP | UF-WIZARD-LAUNCH |
| UF-WIZARD-VALIDATE-PATH | Wizard Validate Invalid Path | UF-WIZARD-LAUNCH |

---

## Android Platform (22 challenges)

### Android Build (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROID-BUILD | Android Build | (none) |
| UF-ANDROID-TEST | Android Unit Tests | UF-ANDROID-BUILD |
| UF-ANDROID-LINT | Android Lint | UF-ANDROID-BUILD |

### Android Launch (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROID-LAUNCH | Android Launch | UF-ANDROID-BUILD |
| UF-ANDROID-STABLE | Android Stability | UF-ANDROID-BUILD |
| UF-ANDROID-SCREENSHOT | Android Screenshot | UF-ANDROID-LAUNCH |

### Android Auth (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROID-AUTH-LOGIN | Android Auth Login | UF-ANDROID-LAUNCH |
| UF-ANDROID-AUTH-INVALID | Android Auth Invalid | UF-ANDROID-LAUNCH |
| UF-ANDROID-AUTH-LOGOUT | Android Auth Logout | UF-ANDROID-AUTH-LOGIN |

### Android Browse (4)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROID-BROWSE-LOAD | Android Browse Load | UF-ANDROID-AUTH-LOGIN |
| UF-ANDROID-BROWSE-SEARCH | Android Browse Search | UF-ANDROID-BROWSE-LOAD |
| UF-ANDROID-BROWSE-DETAIL | Android Browse Detail | UF-ANDROID-BROWSE-LOAD |
| UF-ANDROID-BROWSE-SCROLL | Android Browse Scroll | UF-ANDROID-BROWSE-LOAD |

### Android Playback (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROID-PLAY-START | Android Playback Start | UF-ANDROID-AUTH-LOGIN |
| UF-ANDROID-PLAY-CONTROLS | Android Playback Controls | UF-ANDROID-PLAY-START |
| UF-ANDROID-PLAY-SEEK | Android Playback Seek | UF-ANDROID-PLAY-START |

### Android Settings (2)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROID-SETTINGS-LOAD | Android Settings Load | UF-ANDROID-AUTH-LOGIN |
| UF-ANDROID-SETTINGS-SERVER | Android Settings Server URL | UF-ANDROID-SETTINGS-LOAD |

### Android Offline (2)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROID-OFFLINE-BANNER | Android Offline Banner | UF-ANDROID-AUTH-LOGIN |
| UF-ANDROID-OFFLINE-CACHE | Android Offline Cache | UF-ANDROID-OFFLINE-BANNER |

### Android Instrumented (2)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROID-INSTR-UI | Android Instrumented UI Tests | UF-ANDROID-BUILD |
| UF-ANDROID-INSTR-NAV | Android Instrumented Navigation Tests | UF-ANDROID-BUILD |

---

## Android TV Platform (16 challenges)

### Android TV Build (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROIDTV-BUILD | Android TV Build | (none) |
| UF-ANDROIDTV-TEST | Android TV Unit Tests | UF-ANDROIDTV-BUILD |
| UF-ANDROIDTV-LINT | Android TV Lint | UF-ANDROIDTV-BUILD |

### Android TV Launch (2)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROIDTV-LAUNCH | Android TV Launch | UF-ANDROIDTV-BUILD |
| UF-ANDROIDTV-STABLE | Android TV Stability | UF-ANDROIDTV-BUILD |

### Android TV Navigation (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROIDTV-NAV-DPAD | Android TV D-pad Navigation | UF-ANDROIDTV-LAUNCH |
| UF-ANDROIDTV-NAV-SELECT | Android TV D-pad Select | UF-ANDROIDTV-LAUNCH |
| UF-ANDROIDTV-NAV-BACK | Android TV Back Navigation | UF-ANDROIDTV-LAUNCH |

### Android TV Browse (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROIDTV-BROWSE-LOAD | Android TV Browse Load | UF-ANDROIDTV-LAUNCH |
| UF-ANDROIDTV-BROWSE-ROW | Android TV Browse Row Scroll | UF-ANDROIDTV-BROWSE-LOAD |
| UF-ANDROIDTV-BROWSE-DETAIL | Android TV Browse Detail | UF-ANDROIDTV-BROWSE-LOAD |

### Android TV Playback (3)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROIDTV-PLAY-START | Android TV Playback Start | UF-ANDROIDTV-LAUNCH |
| UF-ANDROIDTV-PLAY-CONTROLS | Android TV Playback Controls | UF-ANDROIDTV-PLAY-START |
| UF-ANDROIDTV-PLAY-DPAD | Android TV Playback D-pad | UF-ANDROIDTV-PLAY-START |

### Android TV Settings (2)

| ID | Name | Dependencies |
|----|------|-------------|
| UF-ANDROIDTV-SETTINGS-LOAD | Android TV Settings Load | UF-ANDROIDTV-LAUNCH |
| UF-ANDROIDTV-SETTINGS-SERVER | Android TV Settings Server URL | UF-ANDROIDTV-SETTINGS-LOAD |

---

## Dependency Chain Summary

The longest dependency chains per platform:

| Platform | Longest Chain |
|----------|---------------|
| API | ENV-SETUP -> HEALTH -> AUTH-LOGIN -> COLL-LIST -> COLL-CREATE -> COLL-ADD -> COLL-REMOVE (7 deep) |
| Web | HEALTH -> AUTH-LOGIN -> (all other web challenges, 2-3 deep) |
| Desktop | BUILD -> LAUNCH -> AUTH-LOGIN -> BROWSE-LOAD -> BROWSE-SEARCH (5 deep) |
| Wizard | BUILD -> LAUNCH -> WELCOME -> PROTOCOL -> SERVER -> COMPLETE (6 deep) |
| Android | BUILD -> LAUNCH -> AUTH-LOGIN -> BROWSE-LOAD -> BROWSE-SEARCH (5 deep) |
| Android TV | BUILD -> LAUNCH -> BROWSE-LOAD -> BROWSE-ROW (4 deep) |

---

## Challenge Totals

| Category | Count |
|----------|-------|
| Original (CH-001 to CH-060) | ~60 |
| Feature & Security (CH-061 to CH-088) | 28 |
| Module Doc Verification (MOD-001 to MOD-015) | 15 |
| Module Functional Verification (MOD-016 to MOD-021) | 6 |
| User Flow - API (UF-API-*) | 49 |
| User Flow - Web (UF-WEB-*) | 59 |
| User Flow - Desktop (UF-DESKTOP-*) | 18 |
| User Flow - Wizard (UF-WIZARD-*) | 10 |
| User Flow - Android (UF-ANDROID-*) | 22 |
| User Flow - Android TV (UF-ANDROIDTV-*) | 16 |
| User Flow - Environment | 2 |
| **Total** | **~285** |
