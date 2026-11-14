# Route authorization table (authz round 2)

Generated 2026-11-03 on branch `fix-authz-round2` from (a) a static extraction of every `GET|POST|PUT|DELETE|PATCH` registration in `catalog-api/main.go` plus the routes mounted from `handlers/storage_routes.go` and `handlers/alias_routes.go` (261 rows; `POST /api/v1/stream/:id/ticket` added by hand, not yet re-probed), and (b) a live probe of the REAL binary (`route_enumeration_probe_test.go`, env-gated by `ROUTE_PROBE_FILE`/`ROUTE_PROBE_OUT`): each route is requested with no token, with a self-registered plain user's token and, for GET routes, with the administrator's token. Cells are HTTP status codes. `404`/`400` for a user means the request passed the gate and the handler rejected the made-up id or empty body. `skip` = not requested as admin (mutating or streaming). `ERR` = the 4 s probe client timed out (`pprof/profile` samples for 30 s).

Instrument checks (control needles): the static extractor found 239 `main.go` registrations, equal to an independent `grep -cE` count of the same file region (239); the known routes `/metrics` and `/api/v1/smb/probe-and-ingest` are present and a fabricated route is absent; the probe asserts before and after the run that the plain token still gets `403` (not `401`) on `POST /api/v1/smb/probe` (the first probe run was invalid because `POST /auth/logout` revoked the plain token; that route and `change-password` are now skipped).

Gate legend: `public` = no authentication; `auth` = `RequireAuth` on the `/api/v1` group only; `RequireAdmin (route)` = per-route `RequireAdmin` (live-DB admin decision, a stale claim never grants); `RequireAdmin (group)` = the whole group is mounted behind `RequireAdmin`; `RequireAuth+RequireAdmin` = both, applied explicitly on the route (`/metrics`, `/debug/pprof/*`); `RequireSelfOrAdmin` = the caller must be the user named in the path or an admin; `configGate` = the route is mounted behind the configuration-permission gate; `auth + in-handler permission` = `RequireAuth` only at the router, the handler itself checks a permission (users, roles, configuration, errors, logs; recognised by plain user = 403 in the live probe); `not probed` = deliberately not requested (it would revoke the probe session or change the probe user password).

`/metrics` additionally accepts the dedicated scrape credential `METRICS_SCRAPE_TOKEN` as `Authorization: Bearer` (this route only; see `docs/deployment/MONITORING_GUIDE.md`); the live probe does not send it.

This round also added the merged-main route `GET /api/v1/collections/:id/items` (auth; ownership enforced in the handler: plain user 404 for an id they do not own).

Operator decisions encoded here: `/metrics` and `/debug/pprof/*` admin-only; outbound enrichment, asset-resolution and subtitle search/download/translate triggers admin-only and SSRF-guarded through `internal/netguard`; `GET /scans*` auth-only; `POST /admin/image-quality/revalidate` and `POST /admin/covers/generate-thumbnails` keep a per-route `RequireAdmin`.

Changed in this round (before -> after for a self-registered user): `/metrics` 200 -> 403 (anonymous 200 -> 401); every `/debug/pprof/*` 200 -> 403; `POST /entities/enrich`, `POST /entities/:id/metadata/refresh`, `POST /assets/request`, `GET /subtitles/search`, `POST /subtitles/download`, `POST /subtitles/translate` reachable -> 403.

| Method | Path | Gate (middleware) | Anonymous | Plain user | Admin |
|---|---|---|---|---|---|
| GET | `/discovery` | public | 200 | 200 | 200 |
| GET | `/metrics` | RequireAuth+RequireAdmin (or `METRICS_SCRAPE_TOKEN` bearer, this route only) | 401 | 403 | 200 |
| GET | `/debug/pprof/` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/cmdline` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/profile` | RequireAuth+RequireAdmin | 401 | 403 | ERR |
| POST | `/debug/pprof/symbol` | RequireAuth+RequireAdmin | 401 | 403 | skip |
| GET | `/debug/pprof/symbol` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/trace` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/allocs` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/block` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/goroutine` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/heap` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/mutex` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/debug/pprof/threadcreate` | RequireAuth+RequireAdmin | 401 | 403 | 200 |
| GET | `/health` | public | 200 | 200 | 200 |
| GET | `/api/v1/health` | public | 200 | 200 | 200 |
| GET | `/health/deep` | public | 200 | 200 | 200 |
| GET | `/ws` | public | 400 | 400 | 400 |
| GET | `/api/v1/image-proxy` | public | 400 | 400 | 400 |
| GET | `/api/v1/assets/:id` | public | 200 | 200 | 200 |
| GET | `/api/v1/cover/placeholder/:type` | public | 200 | 200 | 200 |
| GET | `/api/v1/cover/url/:id` | public | 200 | 200 | 200 |
| GET | `/api/v1/cover/:id` | public | 200 | 200 | 200 |
| POST | `/api/v1/auth/login` | public | 401 | 401 | skip |
| POST | `/api/v1/auth/register` | public | 400 | 400 | skip |
| POST | `/api/v1/auth/refresh` | public | 401 | 401 | skip |
| POST | `/api/v1/auth/logout` | public | not probed | not probed | not probed |
| GET | `/api/v1/auth/me` | auth | 401 | 200 | 200 |
| GET | `/api/v1/auth/status` | public | 200 | 200 | 200 |
| GET | `/api/v1/auth/permissions` | auth | 401 | 200 | 200 |
| GET | `/api/v1/auth/profile` | auth | 401 | 200 | 200 |
| GET | `/api/v1/auth/init-status` | public | 200 | 200 | 200 |
| POST | `/api/v1/auth/change-password` | auth | not probed | not probed | not probed |
| GET | `/api/v1/catalog` | auth | 401 | 200 | 200 |
| GET | `/api/v1/catalog/*path` | auth | 401 | 404 | 404 |
| GET | `/api/v1/catalog-info/*path` | auth | 401 | 404 | 404 |
| GET | `/api/v1/search` | auth | 401 | 200 | 200 |
| GET | `/api/v1/search/duplicates` | auth | 401 | 400 | 400 |
| GET | `/api/v1/search/files` | auth | 401 | 200 | 200 |
| GET | `/api/v1/search/files/duplicates` | auth | 401 | 200 | 200 |
| POST | `/api/v1/search/advanced` | auth | 401 | 200 | skip |
| GET | `/api/v1/download/file/:id` | auth | 401 | 404 | skip |
| GET | `/api/v1/download/directory/*path` | auth | 401 | 500 | skip |
| POST | `/api/v1/download/archive` | auth | 401 | 400 | skip |
| GET | `/api/v1/stream/:id` | auth, or `?ticket=` (stream ticket, this route and this id only; see note below) | 401 | 404 | skip |
| POST | `/api/v1/stream/:id/ticket` | auth (bearer; like every RequireAuth route it also still accepts the legacy `?access_token=` query; a stream ticket is NOT accepted here) | 401 | 404 | skip |
| POST | `/api/v1/copy/storage` | RequireAdmin (route) | 401 | 403 | skip |
| POST | `/api/v1/copy/local` | RequireAdmin (route) | 401 | 403 | skip |
| POST | `/api/v1/copy/upload` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/media/search` | auth | 401 | 200 | 200 |
| GET | `/api/v1/media/stats` | auth | 401 | 200 | 200 |
| GET | `/api/v1/media/recent` | auth | 401 | 200 | 200 |
| GET | `/api/v1/media/popular` | auth | 401 | 200 | 200 |
| GET | `/api/v1/media/by-path` | auth | 401 | 400 | 400 |
| POST | `/api/v1/media/analyze` | auth | 401 | 400 | skip |
| GET | `/api/v1/media/:id` | auth | 401 | 404 | 404 |
| PUT | `/api/v1/media/:id/progress` | auth | 401 | 400 | skip |
| PUT | `/api/v1/media/:id/favorite` | auth | 401 | 400 | skip |
| POST | `/api/v1/media/:id/refresh` | auth | 401 | 404 | skip |
| GET | `/api/v1/media/:id/quality` | auth | 401 | 404 | 404 |
| POST | `/api/v1/admin/image-quality/revalidate` | RequireAdmin (route) | 401 | 403 | skip |
| POST | `/api/v1/admin/covers/generate-thumbnails` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/recommendations/similar/:media_id` | auth | 401 | 404 | 404 |
| GET | `/api/v1/recommendations/trending` | auth | 401 | 501 | 501 |
| GET | `/api/v1/recommendations/personalized/:user_id` | RequireSelfOrAdmin | 401 | 403 | 501 |
| GET | `/api/v1/subtitles/search` | RequireAdmin (route) | 401 | 403 | 400 |
| POST | `/api/v1/subtitles/download` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/subtitles/media/:media_id` | auth | 401 | 200 | 200 |
| GET | `/api/v1/subtitles/:subtitle_id/verify-sync/:media_id` | auth (analysis capped: concurrent runs bounded, 429 `SYNC_BUSY` beyond the cap; results cached per unchanged media file) | 401 | 404 | 404 |
| POST | `/api/v1/subtitles/translate` | RequireAdmin (route) | 401 | 403 | skip |
| POST | `/api/v1/subtitles/upload` | auth | 401 | 400 | skip |
| DELETE | `/api/v1/subtitles/:subtitle_id` | auth + handler check (uploader or admin; ownerless rows admin-only; a non-owner non-admin gets 404, not 403, so existence is not disclosed; 403 only when the admin lookup itself fails) | 401 | 404 (non-owner) / 204 (owner) | 204 |
| GET | `/api/v1/subtitles/languages` | auth | 401 | 200 | 200 |
| GET | `/api/v1/subtitles/providers` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/directories/by-size` | auth | 401 | 400 | 400 |
| GET | `/api/v1/stats/duplicates/count` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/overall` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/smb/:smb_root` | auth | 401 | 404 | 404 |
| GET | `/api/v1/stats/filetypes` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/sizes` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/duplicates` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/duplicates/groups` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/access` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/growth` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/scans` | auth (non-admin: `error_message` replaced by a fixed class text; admin sees raw) | 401 | 200 | 200 |
| POST | `/api/v1/smb/discover` | RequireAdmin (group) | 401 | 403 | skip |
| GET | `/api/v1/smb/discover` | RequireAdmin (group) | 401 | 403 | 410 |
| POST | `/api/v1/smb/test` | RequireAdmin (group) | 401 | 403 | skip |
| GET | `/api/v1/smb/test` | RequireAdmin (group) | 401 | 403 | 410 |
| POST | `/api/v1/smb/browse` | RequireAdmin (group) | 401 | 403 | skip |
| GET | `/api/v1/smb/identities` | RequireAdmin (group) | 401 | 403 | 200 |
| POST | `/api/v1/smb/probe` | RequireAdmin (group) | 401 | 403 | skip |
| POST | `/api/v1/smb/probe-and-ingest` | RequireAdmin (group) | 401 | 403 | skip |
| POST | `/api/v1/conversion/jobs` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/conversion/jobs` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/conversion/jobs/:id` | auth | 401 | 404 | 404 |
| POST | `/api/v1/conversion/jobs/:id/cancel` | auth + in-handler permission | 401 | 403 | skip |
| DELETE | `/api/v1/conversion/jobs/:id` | auth + in-handler permission | 401 | 403 | skip |
| POST | `/api/v1/conversion/jobs/:id/retry` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/conversion/jobs/:id/download` | auth | 401 | 404 | skip |
| GET | `/api/v1/conversion/formats` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/admin/system-info` | RequireAdmin (group) | 401 | 403 | 200 |
| GET | `/api/v1/admin/users` | RequireAdmin (group) | 401 | 403 | 200 |
| PUT | `/api/v1/admin/users/:id` | RequireAdmin (group) + in-handler system.admin check; role change: closed set = roles table, no self/last-admin demotion (D9) | 401 | 403 | skip |
| GET | `/api/v1/admin/storage` | RequireAdmin (group) | 401 | 403 | 200 |
| GET | `/api/v1/admin/backups` | RequireAdmin (group) | 401 | 403 | 200 |
| POST | `/api/v1/admin/backups` | RequireAdmin (group) | 401 | 403 | skip |
| POST | `/api/v1/admin/backups/:id/restore` | RequireAdmin (group) | 401 | 403 | skip |
| POST | `/api/v1/admin/storage/scan` | RequireAdmin (group) | 401 | 403 | skip |
| GET | `/api/v1/admin/config` | RequireAdmin (group) | 401 | 403 | 200 |
| GET | `/api/v1/admin/errors` | RequireAdmin (group) | 401 | 403 | 200 |
| GET | `/api/v1/admin/health` | RequireAdmin (group) | 401 | 403 | 200 |
| GET | `/api/v1/admin/logs` | RequireAdmin (group) | 401 | 403 | 200 |
| POST | `/api/v1/users` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/users` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/users/:id` | auth + in-handler permission | 401 | 403 | 200 |
| PUT | `/api/v1/users/:id` | auth + in-handler permission | 401 | 403 | skip |
| DELETE | `/api/v1/users/:id` | auth + in-handler permission | 401 | 403 | skip |
| POST | `/api/v1/users/:id/reset-password` | auth + in-handler permission | 401 | 403 | skip |
| POST | `/api/v1/users/:id/lock` | auth + in-handler permission | 401 | 403 | skip |
| POST | `/api/v1/users/:id/unlock` | auth + in-handler permission | 401 | 403 | skip |
| POST | `/api/v1/roles` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/roles` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/roles/:id` | auth + in-handler permission | 401 | 403 | 200 |
| PUT | `/api/v1/roles/:id` | auth + in-handler permission | 401 | 403 | skip |
| DELETE | `/api/v1/roles/:id` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/roles/permissions` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/configuration` | auth + in-handler permission | 401 | 403 | 200 |
| POST | `/api/v1/configuration/test` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/configuration/status` | auth + in-handler permission | 401 | 403 | 501 |
| GET | `/api/v1/configuration/wizard/step/:step_id` | auth | 401 | 404 | 404 |
| POST | `/api/v1/configuration/wizard/step/:step_id/validate` | configGate | 401 | 403 | skip |
| POST | `/api/v1/configuration/wizard/step/:step_id/save` | configGate | 401 | 403 | skip |
| GET | `/api/v1/configuration/wizard/progress` | auth | 401 | 200 | 200 |
| POST | `/api/v1/configuration/wizard/complete` | configGate | 401 | 403 | skip |
| POST | `/api/v1/errors/report` | auth + in-handler permission | 401 | 403 | skip |
| POST | `/api/v1/errors/crash` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/errors/reports` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/errors/reports/:id` | auth + in-handler permission | 401 | 403 | 404 |
| PUT | `/api/v1/errors/reports/:id/status` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/errors/crashes` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/errors/crashes/:id` | auth + in-handler permission | 401 | 403 | 404 |
| PUT | `/api/v1/errors/crashes/:id/status` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/errors/statistics` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/errors/crash-statistics` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/errors/health` | auth + in-handler permission | 401 | 403 | 200 |
| POST | `/api/v1/logs/collect` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/logs/collections` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/logs/collections/:id` | auth + in-handler permission | 401 | 403 | 404 |
| GET | `/api/v1/logs/collections/:id/entries` | auth + in-handler permission | 401 | 403 | 500 |
| POST | `/api/v1/logs/collections/:id/export` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/logs/collections/:id/analyze` | auth + in-handler permission | 401 | 403 | 500 |
| POST | `/api/v1/logs/share` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/logs/share/:token` | auth | 401 | 404 | 404 |
| DELETE | `/api/v1/logs/share/:id` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/logs/stream` | auth + in-handler permission | 401 | 403 | skip |
| GET | `/api/v1/logs/statistics` | auth + in-handler permission | 401 | 403 | 200 |
| GET | `/api/v1/collections` | auth | 401 | 200 | 200 |
| POST | `/api/v1/collections` | auth | 401 | 400 | skip |
| GET | `/api/v1/collections/:id` | auth | 401 | 404 | 404 |
| GET | `/api/v1/collections/:id/items` | auth | 401 | 404 | 404 |
| PUT | `/api/v1/collections/:id` | auth | 401 | 404 | skip |
| DELETE | `/api/v1/collections/:id` | auth | 401 | 404 | skip |
| POST | `/api/v1/assets/request` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/assets/by-entity/:type/:id` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/search` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/types` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/stats` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/duplicates` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/browse/:type` | auth | 401 | 400 | 400 |
| GET | `/api/v1/entities/:id` | auth | 401 | 404 | 404 |
| GET | `/api/v1/entities/:id/children` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/:id/files` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/:id/metadata` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/:id/duplicates` | auth | 401 | 404 | 404 |
| GET | `/api/v1/entities/:id/stream` | auth | 401 | 404 | skip |
| GET | `/api/v1/entities/:id/download` | auth | 401 | 404 | skip |
| GET | `/api/v1/entities/:id/pages` | auth | 401 | 404 | 404 |
| GET | `/api/v1/entities/:id/pages/:n` | auth | 401 | 404 | 404 |
| GET | `/api/v1/entities/:id/pdf-pages` | auth | 401 | 404 | 404 |
| GET | `/api/v1/entities/:id/pdf-pages/:n` | auth | 401 | 404 | 404 |
| GET | `/api/v1/entities/:id/install-info` | auth | 401 | 404 | 404 |
| POST | `/api/v1/entities/:id/metadata/refresh` | RequireAdmin (route) | 401 | 403 | skip |
| PUT | `/api/v1/entities/:id/user-metadata` | auth | 401 | 500 | skip |
| POST | `/api/v1/entities/:id/user-metadata` | auth | 401 | 500 | skip |
| POST | `/api/v1/entities/enrich` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/entities/:id/progress` | auth | 401 | 200 | 200 |
| GET | `/api/v1/entities/:id/history` | auth | 401 | 200 | 200 |
| POST | `/api/v1/playback/sessions/start` | auth | 401 | 400 | skip |
| POST | `/api/v1/playback/sessions/progress` | auth | 401 | 400 | skip |
| POST | `/api/v1/playback/sessions/end` | auth | 401 | 400 | skip |
| POST | `/api/v1/analytics/access` | auth | 401 | 200 | skip |
| POST | `/api/v1/analytics/event` | auth | 401 | 200 | skip |
| GET | `/api/v1/analytics/user/:user_id` | RequireSelfOrAdmin | 401 | 403 | 200 |
| GET | `/api/v1/analytics/system` | RequireAdmin (route) | 401 | 403 | 200 |
| GET | `/api/v1/analytics/media/:media_id` | RequireAdmin (route) | 401 | 403 | 200 |
| POST | `/api/v1/analytics/reports` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/reports/usage` | RequireAdmin (route) | 401 | 403 | 200 |
| GET | `/api/v1/reports/performance` | RequireAdmin (route) | 401 | 403 | 200 |
| GET | `/api/v1/favorites` | auth | 401 | 200 | 200 |
| POST | `/api/v1/favorites` | auth | 401 | 400 | skip |
| DELETE | `/api/v1/favorites/:entity_type/:entity_id` | auth | 401 | 400 | skip |
| GET | `/api/v1/favorites/check/:entity_type/:entity_id` | auth | 401 | 400 | 400 |
| GET | `/api/v1/playlists` | auth | 401 | 200 | 200 |
| POST | `/api/v1/playlists` | auth | 401 | 400 | skip |
| GET | `/api/v1/playlists/:id` | auth | 401 | 404 | 404 |
| PUT | `/api/v1/playlists/:id` | auth | 401 | 404 | skip |
| DELETE | `/api/v1/playlists/:id` | auth | 401 | 404 | skip |
| POST | `/api/v1/playlists/:id/items` | auth | 401 | 400 | skip |
| DELETE | `/api/v1/playlists/:id/items/:item_id` | auth | 401 | 404 | skip |
| GET | `/api/v1/browse/roots` | auth | 401 | 200 | 200 |
| GET | `/api/v1/browse/directory/*path` | auth | 401 | 400 | 400 |
| GET | `/api/v1/browse/file-info/*path` | auth | 401 | 400 | 400 |
| GET | `/api/v1/browse/directory-sizes/*path` | auth | 401 | 400 | 400 |
| GET | `/api/v1/browse/duplicates/*path` | auth | 401 | 400 | 400 |
| POST | `/api/v1/sync/endpoints` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/sync/endpoints` | RequireAdmin (route) | 401 | 403 | 200 |
| GET | `/api/v1/sync/endpoints/:id` | RequireAdmin (route) | 401 | 403 | 404 |
| PUT | `/api/v1/sync/endpoints/:id` | RequireAdmin (route) | 401 | 403 | skip |
| DELETE | `/api/v1/sync/endpoints/:id` | RequireAdmin (route) | 401 | 403 | skip |
| POST | `/api/v1/sync/endpoints/:id/sync` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/sync/sessions` | RequireAdmin (route) | 401 | 403 | 200 |
| GET | `/api/v1/sync/sessions/:id` | RequireAdmin (route) | 401 | 403 | 404 |
| POST | `/api/v1/sync/schedules` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/sync/statistics` | RequireAdmin (route) | 401 | 403 | 200 |
| POST | `/api/v1/sync/cleanup` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/challenges` | RequireAdmin (group) | 401 | 403 | 200 |
| GET | `/api/v1/challenges/:id` | RequireAdmin (group) | 401 | 403 | 404 |
| POST | `/api/v1/challenges/:id/run` | RequireAdmin (group) | 401 | 403 | skip |
| POST | `/api/v1/challenges/run` | RequireAdmin (group) | 401 | 403 | skip |
| POST | `/api/v1/challenges/run/category/:category` | RequireAdmin (group) | 401 | 403 | skip |
| GET | `/api/v1/challenges/results` | RequireAdmin (group) | 401 | 403 | 200 |
| GET | `/api/v1/browse` | auth | 401 | 200 | 200 |
| GET | `/api/v1/analytics/stats` | auth | 401 | 200 | 200 |
| GET | `/api/v1/analytics/duplicates` | auth | 401 | 200 | 200 |
| GET | `/api/v1/users/me` | auth | 401 | 200 | 200 |
| GET | `/api/v1/languages` | auth | 401 | 200 | 200 |
| GET | `/api/v1/recommendations` | auth | 401 | 501 | 501 |
| GET | `/api/v1/sync/history` | RequireAdmin (route) | 401 | 403 | 200 |
| GET | `/api/v1/localization/languages` | auth | 401 | 200 | 200 |
| GET | `/api/v1/storage/list/*path` | RequireAdmin (route) | 401 | 403 | 400 |
| GET | `/api/v1/storage/roots` | auth | 401 | 200 | 200 |
| POST | `/api/v1/storage/roots` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/storage-roots` | auth | 401 | 200 | 200 |
| GET | `/api/v1/storage-roots/:id/status` | auth | 401 | 404 | 404 |
| POST | `/api/v1/scans` | RequireAdmin (route) | 401 | 403 | skip |
| GET | `/api/v1/scans` | auth | 401 | 200 | 200 |
| GET | `/api/v1/scans/:job_id` | auth | 401 | 404 | 404 |
| GET | `/api/v1/media-types` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/media-types` | auth | 401 | 200 | 200 |
| GET | `/api/v1/stats/scan-history` | auth | 401 | 200 | 200 |
| GET | `/api/v1/sync/status` | RequireAdmin (route) | 401 | 403 | 200 |
| GET | `/api/v1/sync/providers` | RequireAdmin (route) | 401 | 403 | 200 |
| GET | `/api/v1/sync/devices` | RequireAdmin (route) | 401 | 403 | 501 |
| GET | `/api/v1/sync/conflicts` | RequireAdmin (route) | 401 | 403 | 501 |
| GET | `/api/v1/files` | auth | 401 | 400 | 400 |
| GET | `/api/v1/subtitles` | auth | 401 | 200 | 200 |
| GET | `/api/v1/translations` | auth | 401 | 501 | 501 |
| GET | `/api/v1/localization/translations` | auth | 401 | 501 | 501 |
| GET | `/api/v1/localization/stats` | auth | 401 | 501 | 501 |
| GET | `/api/v1/recommendations/by-type` | auth | 401 | 501 | 501 |

## Stream tickets (added with the web player fix)

`POST /api/v1/stream/:id/ticket` mints an HMAC-SHA256 ticket (60 s) bound to the media id, the user id and the minting session. `GET /api/v1/stream/:id?ticket=...` accepts it in place of the Authorization header ONLY for that id (middleware/auth.go `streamRoute`); on every other route, method or id the `ticket` parameter is ignored and the request is unauthenticated (401). A ticket dies with its session (logout/revocation is checked at use). The catalogue has no per-user ownership model, so any authenticated user may mint a ticket for any catalogued file, exactly as they may stream it with a bearer token. The legacy `?access_token=` / `?token=` query path stays for native clients (libVLC, ffmpeg) and is a follow-up to remove; both parameters are redacted from the gin and zap access logs (`internal/logredact`). Covered by `catalog-api/stream_ticket_binary_test.go` (real binary).
