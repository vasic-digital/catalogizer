# Client route drift: Android / Android TV Retrofit vs catalog-api (2026-11-17)

Status: PROVISIONAL. Method: static analysis of `main` at `ec3af18f4` (reproduce-first §11.4.292: result `reproduced-host-fixture`, surface host-only, by route-table scan; no device run, device behaviour UNCONFIRMED).
Lead: `specs/001-full-project-audit-remediation/poc/route_drift/results/run1.json` (counts: android 22, androidtv 1 client calls without a route). Re-verified against current code: my own scan gives 22 and 1, so the lead still holds.

Method: server routes taken from every registration site (not only main.go): `catalog-api/main.go` (groups `api := /api/v1`, per-group prefixes), `main_chpw.go` (auth group), `main_router.go` (`/api/v1/health`, cover, assets, image-proxy), `main_wsroute.go` (`/api/v1/ws/ticket`), `handlers/alias_routes.go`, `handlers/storage_routes.go` (`/scans`), `handlers/subtitle_routes.go` (`/subtitles`). Client side: every `@GET/@POST/@PUT/@DELETE` in both `CatalogizerApi.kt`. Android base URL is `<server>/api/v1/` (DependencyContainer.kt:93-96); Android TV base is `<server>/` with paths carrying `api/v1/`. Control: positive (known routes such as `auth/login`, `PUT media/{id}/favorite` matched) and negative (the doubled-prefix paths did not match). Caveat: callers traced by grep over `src/main`; Compose/ViewModel reflection or DI lambdas are not modelled. The Android MediaRepository/Offline/Sync findings were additionally read at source.

## Full inventory (Android 41 endpoints, Android TV 38 endpoints; 23 without a server route)

| Client | Verb | Resolved path | Retrofit fn (line) | Client callers | Server route? |
|---|---|---|---|---|---|
| android | POST | `/api/v1/auth/login` | login (14) | not traced (route exists) | YES main_chpw.go:59 |
| android | POST | `/api/v1/auth/register` | register (17) | not traced (route exists) | YES main_chpw.go:60 |
| android | POST | `/api/v1/auth/logout` | logout (20) | not traced (route exists) | YES main_chpw.go:64 |
| android | GET | `/api/v1/auth/profile` | getProfile (23) | not traced (route exists) | YES main_chpw.go:68 |
| android | PUT | `/api/v1/auth/profile` | updateProfile (26) | AuthRepository.kt:174 (no ViewModel/screen caller found) | **NO** |
| android | POST | `/api/v1/auth/change-password` | changePassword (29) | not traced (route exists) | YES main_chpw.go:72 |
| android | POST | `/api/v1/ws/ticket` | mintWsTicket (33) | not traced (route exists) | YES main_wsroute.go:20 |
| android | GET | `/api/v1/auth/status` | getAuthStatus (36) | not traced (route exists) | YES main_chpw.go:66 |
| android | GET | `/api/v1/auth/permissions` | getPermissions (39) | not traced (route exists) | YES main_chpw.go:67 |
| android | GET | `/api/v1/media/search` | searchMedia (43) | not traced (route exists) | YES main.go:1053 |
| android | GET | `/api/v1/media/{id}` | getMediaById (57) | not traced (route exists) | YES main.go:1061 |
| android | GET | `/api/v1/media/{id}/metadata` | getExternalMetadata (60) | MediaRepository.kt:94 (no UI caller) | **NO** |
| android | POST | `/api/v1/media/{id}/refresh` | refreshMetadata (63) | not traced (route exists) | YES main.go:1064 |
| android | GET | `/api/v1/media/{id}/quality` | getQualityInfo (66) | not traced (route exists) | YES main.go:1065 |
| android | GET | `/api/v1/media/stats` | getMediaStats (69) | not traced (route exists) | YES main.go:1054 |
| android | GET | `/api/v1/media/recent` | getRecentMedia (72) | not traced (route exists) | YES main.go:1055 |
| android | GET | `/api/v1/media/popular` | getPopularMedia (75) | not traced (route exists) | YES main.go:1056 |
| android | DELETE | `/api/v1/media/{id}` | deleteMedia (78) | SyncManager.kt:309 (queue never fed) | **NO** |
| android | PUT | `/api/v1/media/{id}` | updateMedia (81) | SyncManager.kt:295 (queue never fed) | **NO** |
| android | PUT | `/api/v1/media/{id}/progress` | updateWatchProgress (85) | not traced (route exists) | YES main.go:1062 |
| android | PUT | `/api/v1/media/{id}/favorite` | setFavoriteStatus (88) | MediaRepository.kt:141, SyncManager.kt:263 (no UI caller) | YES main.go:1063 |
| android | PUT | `/api/v1/media/{id}/rating` | rateMedia (91) | SyncManager.kt:274 (queue never fed) | **NO** |
| android | GET | `/api/v1/user/preferences` | getUserPreferences (94) | SyncManager.kt:333 (syncUserPreferences; sync not started) | **NO** |
| android | GET | `/api/v1/media/updated` | getUpdatedMedia (97) | SyncManager.kt:209 (sync not started) | **NO** |
| android | GET | `/api/v1/smb/sources/status` | getSMBStatus (101) | none | **NO** |
| android | POST | `/api/v1/smb/sources/{id}/reconnect` | reconnectSMBSource (104) | none | **NO** |
| android | GET | `/api/v1/smb/health` | getSMBHealth (107) | none | **NO** |
| android | GET | `/api/v1/analytics/dashboard` | getDashboardData (111) | none | **NO** |
| android | GET | `/api/v1/analytics/charts` | getChartsData (114) | none | **NO** |
| android | GET | `/api/v1/health` | getSystemHealth (118) | not traced (route exists) | YES main_router.go:199 |
| android | GET | `/api/v1/status` | getSystemStatus (121) | none | **NO** |
| android | GET | `/api/v1/stream/{mediaId}` | getStreamUrl (125) | not traced (route exists) | YES main.go:1035 |
| android | GET | `/api/v1/download/{mediaId}` | getDownloadUrl (128) | MediaRepository.kt:215 (no UI caller) | **NO** |
| android | GET | `/api/v1/user/favorites` | getFavorites (132) | none | **NO** |
| android | GET | `/api/v1/user/watchlist` | getWatchlist (138) | none | **NO** |
| android | POST | `/api/v1/user/watchlist/{mediaId}` | addToWatchlist (144) | none | **NO** |
| android | DELETE | `/api/v1/user/watchlist/{mediaId}` | removeFromWatchlist (147) | none | **NO** |
| android | PUT | `/api/v1/user/progress/{mediaId}` | updateUserWatchProgress (150) | MediaRepository.kt:172, SyncManager.kt:247 (no UI caller) | **NO** |
| android | GET | `/api/v1/user/continue-watching` | getContinueWatching (156) | MediaRepository.kt:235 (no UI caller) | **NO** |
| android | GET | `/api/v1/api/v1/entities/{id}/progress` | getEntityProgress (162) | PlaybackRepository.kt:23 <- HistoryDialog <- HomeScreen.kt:155/168/206 (LONG-PRESS, USER-REACHABLE) | **NO** |
| android | GET | `/api/v1/api/v1/entities/{id}/history` | getEntityHistory (167) | PlaybackRepository.kt:50 <- HistoryDialog (USER-REACHABLE) | **NO** |
| androidtv | POST | `/api/v1/auth/login` | login (17) | not traced (route exists) | YES main_chpw.go:59 |
| androidtv | POST | `/api/v1/auth/refresh` | refreshToken (20) | not traced (route exists) | YES main_chpw.go:63 |
| androidtv | POST | `/api/v1/auth/logout` | logout (23) | not traced (route exists) | YES main_chpw.go:64 |
| androidtv | GET | `/api/v1/catalog` | getCatalog (29) | not traced (route exists) | YES main.go:1018 |
| androidtv | GET | `/api/v1/media/search` | searchMedia (32) | not traced (route exists) | YES main.go:1053 |
| androidtv | GET | `/api/v1/entities` | searchEntities (35) | not traced (route exists) | YES main.go:1284 |
| androidtv | GET | `/api/v1/entities/browse/{type}` | browseEntities (38) | not traced (route exists) | YES main.go:1289 |
| androidtv | GET | `/api/v1/entities/{id}` | getEntityById (41) | not traced (route exists) | YES main.go:1290 |
| androidtv | GET | `/api/v1/entities/{id}/children` | getEntityChildren (44) | not traced (route exists) | YES main.go:1291 |
| androidtv | GET | `/api/v1/entities/{id}/stream` | getEntityStream (47) | not traced (route exists) | YES main.go:1295 |
| androidtv | GET | `/api/v1/entities/{id}/pages` | getComicPages (56) | not traced (route exists) | YES main.go:1300 |
| androidtv | GET | `/api/v1/entities/{id}/pdf-pages` | getPdfPages (67) | not traced (route exists) | YES main.go:1305 |
| androidtv | POST | `/api/v1/playback/sessions/start` | startPlaybackSession (74) | not traced (route exists) | YES main.go:1324 |
| androidtv | POST | `/api/v1/playback/sessions/progress` | progressPlaybackSession (79) | not traced (route exists) | YES main.go:1325 |
| androidtv | POST | `/api/v1/playback/sessions/end` | endPlaybackSession (84) | not traced (route exists) | YES main.go:1326 |
| androidtv | GET | `/api/v1/entities/{id}/progress` | getEntityProgress (89) | PlaybackRepository.kt:23 <- HistoryDialog <- HomeScreen.kt:155/168/206 (LONG-PRESS, USER-REACHABLE) | YES main.go:1317 |
| androidtv | GET | `/api/v1/entities/{id}/history` | getEntityHistory (94) | PlaybackRepository.kt:50 <- HistoryDialog (USER-REACHABLE) | YES main.go:1318 |
| androidtv | GET | `/api/v1/media/{id}` | getMediaById (100) | not traced (route exists) | YES main.go:1061 |
| androidtv | GET | `/api/v1/catalog-info/{path}` | getMediaInfo (103) | not traced (route exists) | YES main.go:1020 |
| androidtv | POST | `/api/v1/media/recognize` | recognizeMedia (106) | none | **NO** |
| androidtv | PUT | `/api/v1/media/{id}/progress` | updateWatchProgress (110) | not traced (route exists) | YES main.go:1062 |
| androidtv | POST | `/api/v1/favorites` | addFavorite (114) | not traced (route exists) | YES main.go:1351 |
| androidtv | DELETE | `/api/v1/favorites/{entity_type}/{entity_id}` | removeFavorite (117) | not traced (route exists) | YES main.go:1352 |
| androidtv | GET | `/api/v1/favorites/check/{entity_type}/{entity_id}` | checkFavorite (123) | not traced (route exists) | YES main.go:1353 |
| androidtv | GET | `/api/v1/favorites` | listFavorites (129) | not traced (route exists) | YES main.go:1350 |
| androidtv | GET | `/api/v1/entities/stats` | getEntityStats (133) | not traced (route exists) | YES main.go:1287 |
| androidtv | GET | `/api/v1/collections` | getCollections (137) | not traced (route exists) | YES main.go:1265 |
| androidtv | GET | `/api/v1/collections/{id}` | getCollection (140) | not traced (route exists) | YES main.go:1267 |
| androidtv | GET | `/api/v1/recommendations/similar/{mediaId}` | getSimilarMedia (144) | not traced (route exists) | YES main.go:1087 |
| androidtv | GET | `/api/v1/recommendations/trending` | getTrendingMedia (147) | not traced (route exists) | YES main.go:1088 |
| androidtv | GET | `/api/v1/scans` | getScans (151) | not traced (route exists) | YES storage_routes.go:27 |
| androidtv | GET | `/api/v1/playlists` | getPlaylists (155) | not traced (route exists) | YES main.go:1359 |
| androidtv | POST | `/api/v1/playlists` | createPlaylist (158) | not traced (route exists) | YES main.go:1360 |
| androidtv | POST | `/api/v1/playlists/{id}/items` | addPlaylistItem (161) | not traced (route exists) | YES main.go:1364 |
| androidtv | GET | `/api/v1/stats/overall` | getOverallStats (165) | not traced (route exists) | YES main.go:1104 |
| androidtv | GET | `/api/v1/subtitles/media/{mediaId}` | getSubtitles (169) | not traced (route exists) | YES subtitle_routes.go |
| androidtv | GET | `/api/v1/subtitles/languages` | getSubtitleLanguages (172) | not traced (route exists) | YES subtitle_routes.go |

## Answers

### AFAV-1: does Android reach toggleFavorite / toggleFavoriteOffline / the sync favorite queue from production UI? NO (static)
- `MediaRepository.toggleFavorite` (`catalogizer-android/.../data/repository/MediaRepository.kt:131`): callers are only `MediaRepositoryTest.kt:183-223`. No ViewModel or screen calls it.
- `OfflineRepository.toggleFavoriteOffline` (`OfflineRepository.kt:157`): callers only `OfflineRepositoryTest.kt:113,129`. It is the only producer of `SyncManager.queueFavoriteToggle` (`OfflineRepository.kt:166`), and `queueRatingUpdate` (`:176`); `queueWatchProgressUpdate` is commented out (`:154`). `queueMetadataUpdate`/`queueMediaDeletion` have no caller at all.
- DI: `DependencyContainer.kt:136` defines `offlineRepository` lazily; nothing in `src/main` dereferences it. So `SyncManager.startPeriodicSync` (only caller `OfflineRepository.kt:68` via `setOfflineMode`, itself uncalled) never runs, and the queue drain in `SyncManager.performSyncInternal` (`:151-215`) is never started in production (`SyncService.kt` only simulates phases, `:69-110`; it is registered in the manifest, `AndroidManifest.xml:69`, and never started from code).
- Screens: `ui/screens` has only home, login, search, settings. No favorite icon exists (grep for `Icons.*Favorite` in the Android module: none). `HomeViewModel.kt:55-57` fills `favoriteMedia` from `getPopularMedia` (popular items, not favorites; the HomeScreen section at `HomeScreen.kt:161-165` presents popular items under the favorites state name).
- Consequence: the favorite fix is not user-visible on Android; the whole favorite/rating/offline path is dead code. Related: `ANDROID-RETROFIT-MAP-1` (setFavoriteStatus cannot even be invoked through real Retrofit) stays open and is not user-visible for the same reason. Device behaviour: UNCONFIRMED (needs a build with the HomeScreen long-press flow; long-press opens the history dialog only, `HomeScreen.kt:155,168`).

### AFAV-3: Retrofit methods without a server route
23 rows marked **NO** above. Grouped:
- Dead calls (no caller anywhere, or only dead callers): `user/favorites`, `user/watchlist` (GET/POST/DELETE), `user/progress/{id}` (callers only dead repo/sync code), `user/continue-watching`, `user/preferences`, `smb/sources/status`, `smb/sources/{id}/reconnect`, `smb/health`, `analytics/dashboard`, `analytics/charts`, `status`, `download/{mediaId}`, `media/{id}/metadata`, `media/{id}` PUT and DELETE, `media/{id}/rating`, `media/updated`, `auth/profile` PUT, Android TV `media/recognize`. Server has `GET auth/profile` only (`main_chpw.go:68`), `GET /api/v1/health` but no `/api/v1/status`, `download/file/:id` rather than `download/:id` (`main.go:1030`), `/entities/:id/metadata` rather than `/media/:id/metadata` (`main.go:1293`).
- **User-reachable dead calls (2):** Android `getEntityProgress` and `getEntityHistory` (`CatalogizerApi.kt:162,167`) declare paths starting `api/v1/...` while the Retrofit base already ends in `/api/v1/` (`DependencyContainer.kt:93-96`), so the request goes to `/api/v1/api/v1/entities/{id}/progress` (no such route, the server has `/api/v1/entities/:id/progress`, `main.go:1317-1318`). Reached by long-press on a Home item -> `HistoryDialog` -> `PlaybackRepository.getProgress/listHistory` (`PlaybackRepository.kt:23,50`; `HomeScreen.kt:155,168,206`). The repository swallows non-2xx and returns null/empty, so the dialog shows an empty history without any error (silent failure). Reproduction on device: UNCONFIRMED. The Android TV client (base `<server>/`) uses the same paths correctly.
- Also reachable but harmless: `updateProfile` has an unwired repository method only (no ViewModel caller found).
- Android TV: all 37 other endpoints resolve to a server route. Note `getScans` hits `GET /api/v1/scans` (registered by `RegisterScanRoutes`, `storage_routes.go:27`), which the older drift POC may have counted as unwired.
- The server's alias routes (`alias_routes.go:59-70`) include 501 stubs; none of them is called by these two clients.

### AFAV-4: do the three clients write the same favorites store? NO, two stores
- Store A, per-user `user_favorites(user_id, media_id)` (`database/migrations_v30_user_favorites.go:26-34`): written by `PUT /api/v1/media/:id/favorite` (`main.go:1063` -> `handlers/media_handler.go:238-290`, insert `:283`, delete `:286`) and read for `is_favorite` in `GET /api/v1/media/:id` (`media_handler.go:69`). Used by: Android (`setFavoriteStatus`, dead path per AFAV-1).
- Store B, `favorites(id, user_id, entity_type, entity_id, category, notes, tags, is_public, ...)` (`database/migrations_v11_service_tables.go:25-30`): written by `POST/DELETE /api/v1/favorites...` (`main.go:1347-1353` -> `FavoritesHandler` `service_handlers.go:368,483` -> `FavoritesService` -> `repository/favorites_repository.go:21-23` `INSERT INTO favorites`). Used by: Android TV (`api/v1/favorites*`, `CatalogizerApi.kt:114-130`; UI toggle `MediaDetailScreen.kt:544-551`, `HomeViewModel.kt:335-350`) and the web client (`catalog-web/src/lib/favoritesApi.ts:47-90`, all `/favorites`). Also read by smart playlist rules (`internal/smartrules/smartrules.go:705`).
- No code bridges the stores (grep: `user_favorites` appears only in `media_handler.go`, the v30 migration and its registration). So a favorite set on TV/web never shows as `is_favorite` in `GET /media/:id`, and a PUT `media/:id/favorite` never appears in the web/TV favorites list. Today this is latent because Android has no favorite UI; it becomes a user-visible inconsistency the moment one is added. Needs an operator decision on the single store (merge, or make PUT media/:id/favorite write store B with entity_type/entity_id).

## Follow-ups filed
See `docs/FOLLOW_UPS_NEXT_PHASE.md` section "client route drift (afavinv)": AFAV-7 .. AFAV-12.

## Limits (§11.4.6)
Static only, nothing run on a device or against a live server; web client (166 calls, 69 without route per run1.json) is out of the scope of this task and NOT re-inventoried here; callers for routes that exist were not individually traced.
