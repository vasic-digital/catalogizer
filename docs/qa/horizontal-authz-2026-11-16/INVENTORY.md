# Horizontal-authorization inventory (CAT-HAUTHZ-1)

Date 2026-11-16, base `c7fa65dea`. Sources: `docs/ROUTE_AUTHZ_TABLE.md` (261 rows), router registrations in `catalog-api/main.go` (line numbers below), `catalog-api/main_router.go`, `catalog-api/handlers/{storage_routes,subtitle_routes}.go`, and a read of every handler named. Method: every authenticated route was classified by whether it takes or returns a per-user resource (an id in path, query or body that names something owned by one user, or a list that is scoped to the caller). Result is PROVISIONAL until the sweep is re-run on the merge target.

Sweep test: `catalog-api/horizontal_authz_binary_test.go` (`TestHorizontalAuthz_RealBinary`, real server binary, real SQLCipher DB, real register/login, real scan of real files; users A and B are self-registered role-2 users and the test asserts both get 403 on `/admin/users`).

## 1. Per-user resource routes (horizontal surface): 61 routes

"Cov" = case name in the sweep (`HZ|<name>` log lines). Class: OWNER = id names a resource owned by one user; SELF = no foreign id, scoped by the token (list-leak and body-spoof checks only); PATHUSER = foreign user id in the path.

| # | Route | Registration | Class | Cov |
|---|---|---|---|---|
| 1 | GET /api/v1/collections | main.go:1265 | SELF list | collection list |
| 2 | POST /api/v1/collections | main.go:1266 | SELF create | (setup of B's collection) |
| 3 | GET /api/v1/collections/:id | main.go:1267 | OWNER | collection read |
| 4 | GET /api/v1/collections/:id/items | main.go:1268 | OWNER | collection items |
| 5 | PUT /api/v1/collections/:id | main.go:1269 | OWNER | collection update, public update |
| 6 | DELETE /api/v1/collections/:id | main.go:1270 | OWNER | collection delete, public delete |
| 7 | GET /api/v1/playlists | main.go:1358 | SELF list | playlist list |
| 8 | POST /api/v1/playlists | main.go:1359 | SELF create | (setup) |
| 9 | GET /api/v1/playlists/:id | main.go:1360 | OWNER | playlist read |
| 10 | PUT /api/v1/playlists/:id | main.go:1361 | OWNER | playlist update |
| 11 | DELETE /api/v1/playlists/:id | main.go:1362 | OWNER | playlist delete |
| 12 | POST /api/v1/playlists/:id/items | main.go:1363 | OWNER | playlist add item |
| 13 | DELETE /api/v1/playlists/:id/items/:item_id | main.go:1364 | OWNER | playlist remove item |
| 14 | GET /api/v1/favorites | main.go:1349 | SELF list | favorite list |
| 15 | POST /api/v1/favorites | main.go:1350 | SELF create | (setup) |
| 16 | DELETE /api/v1/favorites/:entity_type/:entity_id | main.go:1351 | SELF (row keyed by caller) | favorite remove |
| 17 | GET /api/v1/favorites/check/:entity_type/:entity_id | main.go:1352 | SELF | favorite check |
| 18 | PUT /api/v1/media/:id/favorite | main.go:1063 | per-item GLOBAL flag | **FINDING HAUTHZ-F2** (gated test) |
| 19 | PUT /api/v1/media/:id/progress | main.go:1062 | validated no-op, nothing persisted (handlers/media_handler.go UpdateWatchProgress) | not exercisable: no state to leak |
| 20 | GET /api/v1/media/:id | main.go:1061 | shared catalogue, but returns the global `is_favorite` | via F2 |
| 21 | PUT /api/v1/entities/:id/user-metadata | main.go:1309 | SELF write (user id from token, handlers/media_entity_handler.go:1192) | Write path only; the read path is row 22a (HAUTHZ-O1 added the first read route, so the binary sweep now has a discriminating leg). Covered at store level by `handlers/horizontal_authz_unit_test.go` `TestHorizontalAuthzUnit_UserMetadataOwnerFromTokenNotBody` (real handler, real SQLite, read back through `UserMetadataRepository`); the binary sweep keeps only the owner-write positive controls |
| 22 | POST /api/v1/entities/:id/user-metadata | main.go:1310 | SELF write | same handler as 21 (same store-level test) |
| 22a | GET /api/v1/entities/:id/user-metadata | main.go (added by HAUTHZ-O1) | SELF read (row looked up by entity id AND token user id; no row for the caller, foreign or missing entity => one 404) | usermeta read B own, usermeta read A own, usermeta read foreign (A reads B's m1); unit `TestHorizontalAuthzUnit_UserMetadataReadCallerScoped`. Note: the row count/total in this inventory was NOT renumbered; 22a is an addition (PROVISIONAL) |
| 23 | GET /api/v1/entities/:id/progress | main.go:1316 | SELF | playback progress |
| 24 | GET /api/v1/entities/:id/history | main.go:1317 | SELF | playback history |
| 25 | POST /api/v1/playback/sessions/start | main.go:1323 | SELF create | (setup) |
| 26 | POST /api/v1/playback/sessions/progress | main.go:1324 | OWNER (session_id in body) | playback progress foreign session — **FINDING HAUTHZ-F1, fixed** |
| 27 | POST /api/v1/playback/sessions/end | main.go:1325 | OWNER (session_id in body) | playback end foreign session — **HAUTHZ-F1, fixed** |
| 28 | POST /api/v1/analytics/access | main.go:1331 | SELF write, body `user_id` | analytics write as B (access) |
| 29 | POST /api/v1/analytics/event | main.go:1332 | SELF write, body `user_id` | analytics write as B (event) |
| 30 | GET /api/v1/analytics/user/:user_id | main.go:1333 (RequireSelfOrAdmin) | PATHUSER | analytics user/B |
| 31 | GET /api/v1/recommendations/personalized/:user_id | main.go:1089 (RequireSelfOrAdmin) | PATHUSER | recommendations personalized/B |
| 32 | POST /api/v1/subtitles/upload | handlers/subtitle_routes.go:16 | SELF create (uploaded_by) | (setup) |
| 33 | DELETE /api/v1/subtitles/:subtitle_id | handlers/subtitle_routes.go:17 | OWNER (uploader) | subtitle delete by A, subtitle survives |
| 34 | POST /api/v1/conversion/jobs | main.go:1138 | SELF create | (setup) |
| 35 | GET /api/v1/conversion/jobs | main.go:1139 | SELF list | conversion list |
| 36 | GET /api/v1/conversion/jobs/:id | main.go:1140 | OWNER | conversion read |
| 37 | POST /api/v1/conversion/jobs/:id/cancel | main.go:1141 | OWNER | conversion cancel |
| 38 | DELETE /api/v1/conversion/jobs/:id | main.go:1142 | OWNER | conversion delete |
| 39 | POST /api/v1/conversion/jobs/:id/retry | main.go:1143 | OWNER | conversion retry |
| 40 | GET /api/v1/conversion/jobs/:id/download | main.go:1144 | OWNER | conversion download |
| 41 | GET /api/v1/users/:id | main.go:1196 | PATHUSER (permission `user.view`) | user read |
| 42 | PUT /api/v1/users/:id | main.go:1197 | PATHUSER (permission) | user update |
| 43 | DELETE /api/v1/users/:id | main.go:1198 | PATHUSER (permission) | user delete |
| 44 | POST /api/v1/users/:id/reset-password | main.go:1199 | PATHUSER (permission) | user reset-password |
| 45 | POST /api/v1/users/:id/lock | main.go:1200 | PATHUSER (permission) | user lock |
| 46 | POST /api/v1/users/:id/unlock | main.go:1201 | PATHUSER (permission) | user unlock |
| 47 | PUT /api/v1/admin/users/:id | main.go:1172 (RequireAdmin group + in-handler check) | PATHUSER | admin user update |
| 48 | GET /api/v1/users/me | main.go:1431 | SELF | self view |
| 49 | GET /api/v1/auth/me | main_router.go:290 | SELF | self view |
| 50 | GET /api/v1/auth/profile | main_router.go:292 | SELF | self view |
| 51 | GET /api/v1/auth/permissions | main_router.go:291 | SELF | self view |
| 52 | POST /api/v1/auth/change-password | main_router.go:294 | SELF (current password required) | not exercised: acts on the caller only, no foreign id; would revoke the token under test |
| 53 | POST /api/v1/auth/logout | main_router.go:296 | SELF | not exercised (revokes the token under test); acts on the caller's session only |
| 54 | GET /api/v1/configuration/wizard/progress | main.go:1226 | SELF (configUserID) | NOT COVERED by a live test. Code fact: `ConfigurationRepository.GetWizardProgress` queries `WHERE user_id = ?` with the caller's id (`repository/configuration_repository.go:94`), so the read is owner-scoped; writes need `system.configure`, which only Admin holds by default |
| 55 | POST /api/v1/configuration/wizard/step/:step_id/save | main.go:1225 (configGate) | SELF write | NOT COVERED by a live test (same code fact) |
| 56 | POST /api/v1/configuration/wizard/complete | main.go:1227 (configGate) | SELF write | NOT COVERED by a live test (same code fact; also mutates server configuration) |
| 57 | POST /api/v1/stream/:id/ticket | main.go:1038 | capability minted for caller+session+media id | NOT COVERED horizontally: shared catalogue; the ticket is a 60 s bearer capability for ONE media id (covered by `stream_ticket_binary_test.go`) |
| 58 | GET /api/v1/stream/:id (`?ticket=`) | main.go:1035 | shared catalogue | same |
| 59 | POST /api/v1/ws/ticket | main_wsroute.go:20 | capability, bound to user and session | NOT COVERED horizontally (covered by `main_wsticket_test.go`) |
| 60 | GET /api/v1/subtitles/media/:media_id | handlers/subtitle_routes.go:13 | shared catalogue (every user sees uploaded subtitles of an item by design) | used as the state check in case 33 |
| 61 | GET /api/v1/scans, GET /api/v1/scans/:job_id, GET /api/v1/storage-roots/:id/status | handlers/scan_handler.go (RegisterScanRoutes), handlers/storage_routes.go:20 | scan jobs and storage roots are installation-wide, not per-user (operator decision SSE-OPQ-1: non-admin sees class text only) | NOT per-user; covered by `route_perms_sse_opq1_binary_test.go` |

Rows 60 and 61 are counted once each although row 61 holds three routes; row 2/8/15/25/32/34 are creates used as setup, and they are in the 61.

## 2. Permission-gated installation-wide diagnostics (not horizontal by design)

`/api/v1/errors/*` (main.go:1233-1243; reclassified per-user by the CAT-HAUTHZ-1 follow-up 2 below), `/api/v1/logs/*` (main.go:1249-1259) incl. log shares (`logs/share`, `logs/share/:token`), `/api/v1/roles/*`, `/api/v1/configuration` (non-wizard). Each handler requires `system.admin`, `report.view`/`report.create`, `user.manage` or `system.configure` before touching a record (`handlers/log_management_handler.go:142` etc.; `handlers/error_reporting_handler.go:58`). Correction (CAT-HAUTHZ-1 follow-up): error and crash reports are NOT installation-wide for a holder: `services/error_reporting_service.go` `GetErrorReport`/`GetCrashReport` (lines 182-205) refuse when `report.UserID != userID`, so they are owner-filtered. Log collections and diagnostics are installation-wide for a permission holder by design (the granted permission, not a defect). `GET /logs/share/:token` is a bearer-token capability: possession of the token is the grant (checked against the share's `read` permission and expiry), classified with the ws/stream tickets (rows 58-59), not as a per-user resource. Error and crash reports are swept (section 2a); log collections and shares are NOT swept horizontally (installation-wide by design); plain-user 403 is pinned by the route-permission tests (`route_perms_*_binary_test.go`).

### 2a. Error and crash reports: caller-scoped, now swept (CAT-HAUTHZ-1 follow-up 2, F2)

Code facts: every `/api/v1/errors/*` handler (`handlers/error_reporting_handler.go`) takes the user id from the request context and requires `report.view` (reads, lists, statistics) or `report.create` (create, status update). The default `User` role holds neither (`database/migrations_sqlite.go:284`: `media.view`, `media.download`), so a plain default user answers 403; only a permission holder reaches the data, and a holder is served only its OWN rows: `services/error_reporting_service.go` `GetErrorReport`/`GetCrashReport`/`UpdateErrorStatus`/`UpdateCrashStatus` refuse `report.UserID != caller`, and `ErrorReportingRepository.GetErrorReportsByUser`, `GetErrorStatistics` and the crash equivalents filter `WHERE user_id = caller`. These are per-user resources, not installation-wide, and are swept in `horizontal_authz_binary_test.go` subtest `error_reports` (real binary; users A and B hold `report.view` + `report.create` through a role; B files an error report and a crash report, A files one error report): B reads its own (positive controls: read, list, statistics, crash read/list/statistics, owner resolve); A is refused 404 on B's report and crash read, sees none of B's markers in the list, statistics (`errors_by_component`, `crashes_by_signal`) calls, and its status update on B's reports writes nothing (state check; owner resolve control proves the route is live).

Unrouted handler methods (no `main.go` route; `grep` of `main.go`): `ExportReports` (filters by caller id), `UpdateConfiguration`, `GetConfiguration` and `CleanupOldReports` (installation-wide service operations); none is reachable, so none is swept (tracked: unrouted). Route count: 11 `/api/v1/errors/*` routes (`main.go:1233-1243`), 13 with the admin-group `admin/errors` and `admin/health` routes.

Finding HAUTHZ-F3 (FIXED, PROVISIONAL): `PUT /errors/reports/:id/status` and `PUT /errors/crashes/:id/status` used to answer 500 for a foreign AND a missing id (the handler mapped every service error to 500). The service now returns the typed `services.ErrReportNotFound` for a missing row (`sql.ErrNoRows`) and for a foreign owner alike, and the handler maps it to the same 404 and the same body as the GET routes (real internal errors stay 500); the sweep pins 404 for foreign == missing == GET and an identical body.

Still not swept horizontally: `GET /errors/health` and `admin/errors|health` (system.admin), log collections/shares (installation-wide by design, as above).

## 3. Admin-only routes (cannot be horizontal; a non-admin never reaches the handler)

`/metrics`, `/debug/pprof/*`, `/api/v1/copy/*`, `/api/v1/smb/*`, `/api/v1/admin/*` (except `PUT /admin/users/:id`, row 47), `/api/v1/storage/*`, `POST|GET /api/v1/storage-roots`, `POST /api/v1/scans`, `/api/v1/sync/*` (main.go:1380-1390, 1447), `/api/v1/challenges/*`, `/api/v1/analytics/{system,media,reports}`, `/api/v1/reports/*`, `POST /entities/enrich`, `POST /entities/:id/metadata/refresh`, `POST /assets/request`, `GET|POST /subtitles/{search,download,translate}`. Source: `docs/ROUTE_AUTHZ_TABLE.md` (RequireAdmin rows) and `main.go` `adminGate` uses.

## 4. Shared-catalogue and public routes (no per-user resource)

`/api/v1/catalog*`, `/search*`, `/download/*`, `/media/*` reads, `/entities/*` reads, `/stats/*`, `/browse*`, `/assets/:id`, `/cover/*`, `/image-proxy`, `/discovery`, `/health*`, `/auth/{login,register,refresh,status,init-status}`. The catalogue is shared by design.

## 5. Count

61 rows (row 61 groups three installation-wide routes, so 63 routes). Disposition of the 61 rows:

- 48 rows exercised by the sweep (rows 1-17 and 21-51): a negative case as the other user plus a positive control (the creates 2, 8, 15, 25, 32, 34 and row 22 serve as setup or share a handler with row 21).
- 2 rows (18, 20) carry open finding HAUTHZ-F2, reproduced by the gated test `TestHorizontalAuthz_OpenFinding_MediaFavoriteFlag`.
- 1 row (19) is a validated no-op that persists nothing, so there is no state to leak.
- 2 rows (52, 53) act on the caller only and revoke the token under test: not exercised.
- 3 rows (54-56) NOT COVERED (reason in the table).
- 4 rows (57-59, 61) are capability or installation-wide routes covered by other tests; row 60 is the shared subtitle list used as a state check.


## HAUTHZ-O1 (2026-11-16)

Legacy `internal/handlers/media.go` `getUserMetadata` (from 16843b133) was unrouted and read `user_metadata` by `media_item_id` alone. Fixed: it now takes the caller id and filters `user_id`, and a new routed `GET /api/v1/entities/:id/user-metadata` (live `MediaEntityHandler.GetUserMetadata`, auth group, no admin marker) is caller-scoped. Evidence: `o1/` (red, green x3, mutants.txt, SHA256SUMS). PROVISIONAL.
