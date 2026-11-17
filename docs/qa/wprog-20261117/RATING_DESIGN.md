# Rating routes: design proposal only (no route added; PROVISIONAL, 2026-11-17)

Clients call `PUT /api/v1/media/{id}/rating` (Android `rateMedia`, body `RatingRequest {rating: Double}`) and
`PUT /api/v1/user/progress/{mediaId}` (Android `updateUserWatchProgress`). The server has neither route
(docs/qa/client-route-drift-20261117/REPORT.md; both only have dead or unfed callers on Android).

## user/progress/{mediaId}
Not added. The Android body is the same `WatchProgressRequest` the fixed `PUT /media/:id/progress` now accepts, so the
cheapest fix is on the CLIENT: change `updateUserWatchProgress` (CatalogizerApi.kt:148, MediaRepository.kt:179,
SyncManager.kt:250) to call `PUT media/{id}/progress`. Adding a second server alias would need an entry in the route
authz table / contract gate (owned by the gate owner). Follow-up for the gate owner if a server alias is preferred.

## media/{id}/rating
Store: the per-user table `user_metadata` already has `user_rating REAL`, `media_item_id`, `user_id` (migrations_sqlite.go
~line 764, Postgres twin ~line 636); it has no UNIQUE(user_id, media_item_id) shown in this read, UNCONFIRMED, so the
upsert must be SELECT/UPDATE/INSERT like the progress route.
Proposed contract: `PUT /api/v1/media/:id/rating {"rating": number}`; scale UNDECIDED (Android passes a Double, web
`media.rating` is 0-10 for external ratings): needs an owner decision (0-10 vs 0-5 vs 1-5). 401 before parse, 400 on
invalid id/body/out-of-range, 404 unknown media, 200 `{success, persisted:true}`; `DELETE` or `rating:null` clears.
Reads: `GET /media/:id` would expose `user_rating` for the caller only (same shape as `is_favorite`).
Not implemented because (a) the scale is an owner decision, (b) a new route must be added to `docs/ROUTE_AUTHZ_TABLE.md`
and the route-set equality check of the admin-403 gate (api_contract_gate_*, not touched by this change), and (c)
Android's rating queue is never fed (`OfflineRepository.toggleFavoriteOffline` only), so it is not user-visible today.
Follow-up for the gate owner: when the route is added, add `PUT /api/v1/media/:id/rating` as an authenticated
(non-admin) row to the route authz table.
