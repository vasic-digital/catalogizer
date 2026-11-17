# CAT-LEGACY-MEDIAHANDLER-1 — decision memo (read-only investigation)

Date: 2026-11-17. HEAD: `baceee7e0` (main). Investigator: subagent (Opus), read-only on the main checkout.
Scratch removal trial: `/home/milosvasic/.cache/claude-tmp/legacymh/tree/` (a `git archive HEAD catalog-api` copy, `submodules` symlinked read-only). Nothing in the main checkout was edited or deleted.

## 0. Scope correction

The order says "the legacy MediaHandler in catalog-api (root-package handlers)". The type is NOT in the root package `catalog-api/handlers`; it is
`catalog-api/internal/handlers/media.go:19` (`type MediaHandler struct`), constructor `NewMediaHandler` at `:26`. The root package only has
`AndroidTVMediaHandler` (`handlers/media_handler.go:17`), `MediaEntityHandler` (`handlers/media_entity_handler.go:141`), `MediaBrowseHandler`
(`handlers/media_browse_handler.go:19`) and `MediaQueryHandler` (`handlers/stub_handler.go:20`), all routed. The tracker entry agrees with this
reading (`docs/FOLLOW_UPS_NEXT_PHASE.md:3329`, `docs/WORKABLE_ITEMS_REGISTER.md:177`). File is now 676 lines (`wc -l`), not the 662 the item says
(the HAUTHZ-O1 change `7d0271c66` added lines).

## 1. What it is

Public methods and the routes they *declare* in swag comments (no route actually registers them, see section 3):

| Method | Line | Declared route | Data source |
|---|---|---|---|
| `GetMediaTypes` | 41 | `GET /api/v1/media/types` (`:40`) | `media_types` |
| `SearchMedia` | 106 | `GET /api/v1/media/search` (`:105`) | `media_items` + filters year/year_from/year_to/genre/min_rating/has_externals/quality/smb_roots/watched_status |
| `GetMediaItem` | 326 | `GET /api/v1/media/{id}` (`:325`) | item + external_metadata + media_files + caller's user_metadata |
| `AnalyzeDirectory` | 367 | `POST /api/v1/media/analyze` (`:366`) | queues `analyzer.MediaAnalyzer.AnalyzeDirectory(dir, smb_root, priority)` |
| `GetMediaStats` | 407 | `GET /api/v1/media/stats` (`:406`) | db stats, type distribution, quality distribution, 24 h activity |

All queries go through `h.mediaDB.GetDB()`, i.e. `internal/media/database.MediaDatabase`: a SEPARATE SQLCipher SQLite file
(`internal/media/database/database.go`: default path `media_catalog.db`, password required, DSN `file:%s?_pragma_key=...`).
Its schema includes `directory_analysis` (`internal/media/database/database.go:343`, `schema.sql:53`), a table the main catalog DB migrations do not create
(`git grep directory_analysis -- database/` returns nothing; `detection_patterns` exists in both, `database/migrations_sqlite.go:661`).

## 2. How it was wired and how it died (§11.4.124)

`git log --follow --format='%h %ad %s' -- catalog-api/internal/handlers/media.go`:
```
7d0271c66 2026-11-16 CAT-HAUTHZ-O1: caller-scoped user-metadata read, routed (PROVISIONAL)
4cfcac6b5 2026-11-02 fix(media): quality distribution counts real media_files.quality_info instead of hardcoded zeros (CAT-001)
851beda59 2026-03-24 Fix 8 bugs across 4 components, expand HelixQA test banks to 517 cases
24c9f129a 2026-02-25 fix: sqlite/postgresql compatibility and improve test coverage
7f812fcd9 / ac61fcea0 2025-10-14 Auto-commit;  202acc4fa 2025-10-10 Testing
16843b133 2025-10-04 Implementations   <- created
```

Finding: **it was never wired. It did not "die"; it was born as an optional subsystem whose integration step was documented but never executed.**

Evidence:
- `git log -S NewMediaHandler` over the whole repo (16 commits) — every hunk that adds/removes `NewMediaHandler` is in `media.go`, its tests, coverage
  reports, docs, or `internal/tests/integration_test.go`. No production file ever constructed it.
  - `6193c4355` (2025-10-09) added `mediaHandler := handlers.NewMediaHandler(recognitionService, duplicationService)` in
    `catalog-api/internal/tests/integration_test.go` — a DIFFERENT 2-arg signature that never matched `media.go`'s 3-arg constructor; removed by
    `100d40877` (2025-11-27 "Fix Android TV build and resolve test issues"). Test code only.
  - `16843b133` added `catalog-api/MEDIA_SYSTEM_README.md` with an "Integrate with Existing API — Add to your main.go" snippet
    (`MEDIA_SYSTEM_README.md:293-305`: `media.NewMediaManager`, `handlers.NewMediaHandler(...)`, four `api.*` routes). That instruction was never applied.
- `git log -G "mediaHandler\b|MediaHandler\(|NewMediaManager|mediaManager" -- catalog-api/main.go` → only `273b70bf7` (2025-12-09), whose matching lines are
  `androidTVMediaHandler := root_handlers.NewAndroidTVMediaHandler(...)` and its 3 routes, not this type.
- The whole backing stack is also unwired: `NewMediaManager` (`internal/media/manager.go:53`) has no non-test caller
  (`grep -rn 'NewMediaManager(' --include=*.go . | grep -v _test` → only its definition); `main.go` imports only `catalogizer/internal/media/providers`
  from `internal/media` (`main.go:18`). Nothing in production constructs `MediaDatabase` or `MediaAnalyzer` (the only non-test constructors are inside
  `manager.go:64,81`). So the encrypted `media_catalog.db` the handler reads is never created or written by the server
  (consistent with `specs/001-full-project-audit-remediation/docs/01-system-architecture-map.md:163,665`).

How the same paths got other owners (`git log --reverse -S <token> -- catalog-api/main.go`):
- `GET /media/:id` → `androidTVMediaHandler.GetMediaByID`: `273b70bf7` 2025-12-09.
- `GET /media/search`, `GET /media/stats` → `mediaBrowseHandler`; `/entities/*` incl. `/entities/types` → `mediaEntityHandler`: `3851b594a` 2026-02-18.
- `POST /media/analyze` → `stubHandler.AnalyzeMedia` `919e24e0f` 2026-03-29, then `mediaQueryHandler.AnalyzeMedia` `f590bf0b2` 2026-03-30.
- caller-scoped user metadata `GET /entities/:id/user-metadata` → `mediaEntityHandler.GetUserMetadata`: `7d0271c66` 2026-11-16.

Later edits (`24c9f129a`, `851beda59`, `4cfcac6b5`, `7d0271c66`) were all correctness/coverage fixes to keep the unrouted file consistent; none wired it.

## 3. Every reference

Production code (main checkout, `git grep -n "NewMediaHandler\|\bMediaHandler\b"`, excluding `internal/handlers/media*.go`):
- **No route, no constructor call, no reflection, no route table.** `main.go` route list for media: `main.go:1053-1065` (all other handlers), `main.go:1280-1310` (entities).
- No build tags in `media.go` (file starts `package handlers`, `media.go:1`). The package `internal/handlers` IS compiled into the server (main.go imports it, `main.go:15`, for Catalog/Download/Stream/Copy/SMB handlers, `main.go:701-707`), so `media.go` is compiled but has zero references; the linker drops it.
- No swag/OpenAPI generation: `grep swag catalog-api/go.mod` → nothing; no `docs/docs.go`/`swagger.*` tracked. The `@Router` comments are inert.
- `docs/api/openapi.yaml:3283` (`/media/search`), `:3339` (`/media/stats`), `:3354` (`/media/{id}`), `:3614` (`/media/analyze`) describe the LIVE handlers (e.g. `/media/analyze` documents `path`/`id` body, `:3614-3630`), not this one. `/api/v1/media/types` appears in no OpenAPI or API doc (`grep -rln media/types --include=*.md --include=*.yaml --include=*.json` → nothing).

Tests that reference it (the only callers):
- `internal/handlers/media_test.go` (432 lines, 20 tests, `:51-425`)
- `internal/handlers/media_quality_cat001_test.go` (42 lines, 2 tests, `:20,:36`)
- `internal/handlers/media_usermeta_authz_test.go` (54 lines, 1 test, `:15`)
- Run at HEAD: `go test -count=1 -v -run 'TestNewMediaHandler|TestMediaHandler_|TestCAT001_MediaHandler' ./internal/handlers/` → 23 `--- PASS`, 0 FAIL/SKIP.

Gate / guard data that names the file:
- `internal/nilslice_guard/guard_test.go:224-225` — two accepted-finding entries ("unrouted: internal/handlers.MediaHandler is never constructed outside tests").
- `docs/qa/horizontal-authz-2026-11-16/o1/mut.sh:31-32` — HAUTHZ-O1 mutants M4/M5 mutate `internal/handlers/media.go` (historical evidence script).
- SonarQube run5: one CRITICAL code smell `go:S3776` at `internal/handlers/media.go:106` (SearchMedia, cognitive complexity 35/15) (`docs/qa/sonarqube-20261111/run5/catalog-api.issues.json`; also runs 1-4).
- Old reports: `catalog-api/test_report_20251127_085749.md:381-393` (coverage listing).

Docs:
- `catalog-api/MEDIA_SYSTEM_README.md:150-197` (endpoint list, many never existed anywhere, e.g. `/media/{id}/versions`, `/media/stats/quality`) and `:293-305` (wiring snippet).
- `docs/architecture/ARCHITECTURE.md:1044` — `handlers.NewMediaHandler(registry)` "main.go (simplified)" example with a signature that does not exist (stale/illustrative).
- Tracking: `docs/FOLLOW_UPS_NEXT_PHASE.md:3225,3329,3338,3570`, `docs/WORKABLE_ITEMS_REGISTER.md:177`, `docs/CONTINUATION.md:16` / `.html:215`, `docs/qa/horizontal-authz-2026-11-16/INVENTORY.md:114,116`, `specs/001-.../docs/01-system-architecture-map.md:163,665,734`, `specs/001-.../tasks.md:899` (T374).

Clients (Android, Android TV, web, desktop, api-client, installer-wizard):
- `/media/types`: no client calls it (`git grep media/types` over `*.ts *.tsx *.kt *.rs *.js` → only `media_test.go:151,177`).
- Clients call `/media/search`, `/media/{id}`, `/media/stats`, `/media/analyze`, but those are served by the live handlers. Request shapes that match ONLY the legacy contract:
  - web `catalog-web/src/lib/mediaApi.ts:39-40` `analyzeDirectory` posts `{ directory_path }` — the legacy body (`media.go:369`); the live `AnalyzeMedia` wants `path`/`id` (`handlers/stub_handler.go:411-430`) and would answer 400 "either path or id must be provided". Added in the same commit `16843b133` (`git log -L`). It has NO production caller in `catalog-web/src` (only `__tests__`; `Dashboard.test.tsx:382` asserts it is not called). UNCONFIRMED at runtime (not executed live).
  - web `mediaApi.ts:42-43` and Android `CatalogizerApi.kt:61-62` / `MediaRepository.kt:93-94` call `GET /media/{id}/metadata`, which no route serves (only `/entities/:id/metadata`, `main.go:1293`) — and the legacy handler never served it either. No UI caller found for either (`git grep getExternalMetadata` outside the API/repository files → nothing). UNCONFIRMED at runtime.
  - Android `searchMedia` sends `year_min`, `year_max`, `rating_min`, `quality` (`CatalogizerApi.kt:44-56`); web type `MediaSearchRequest` has the same names (`catalog-web/src/types/modules.d.ts:146-160`). The legacy handler used DIFFERENT names (`year_from`, `year_to`, `min_rating`, `media.go` SearchMedia), and the live `MediaBrowseHandler.SearchMedia` reads only `query, media_type, sort_by, sort_order, limit, offset` (`handlers/media_browse_handler.go:56-61`). So those client filters are silently ignored today, and reviving the legacy handler would NOT fix that (name mismatch).

## 4. Replacements per function

| Legacy | Live replacement | Parity |
|---|---|---|
| `GetMediaTypes` | `GET /entities/types` → `MediaEntityHandler.GetEntityTypes` (`main.go:1286`, `media_entity_handler.go:410`) | equivalent purpose, different JSON shape (UNCONFIRMED field-by-field) |
| `SearchMedia` | `GET /media/search` → `MediaBrowseHandler.SearchMedia` (`main.go:1053`); `GET /entities/search` (`main.go:1285`) | text + type + sort + paging only; NO year/genre/rating/quality/has_externals/smb_roots/watched_status filters (see section 6) |
| `GetMediaItem` | `GET /media/:id` (AndroidTV, `main.go:1061`); `GET /entities/:id`, `/:id/files`, `/:id/metadata`, `/:id/user-metadata` (`main.go:1291-1309`) | covered across several routes; user-metadata now caller-scoped live (operator decision HAUTHZ-O1-FU1, `FOLLOW_UPS_NEXT_PHASE.md:3338`) |
| `AnalyzeDirectory` | `POST /media/analyze` → `MediaQueryHandler.AnalyzeMedia` (per item/path lookup, not a queue); scanning via `scanHandler` / universal scanner (`main.go:864`) | the legacy one queued the `internal/media` detector/analyzer pipeline, which is itself unwired; the live product scans through a different pipeline |
| `GetMediaStats` | `GET /media/stats` (`main.go:1054`), `GET /entities/stats` (`media_entity_handler.go:1255`) | equivalent purpose, different shape |
| watch progress / favorite | (legacy never had these) live in `AndroidTVMediaHandler.UpdateWatchProgress` / `UpdateFavoriteStatus` (`main.go:1062-1063`) | n/a |

## 5. Gate impact of a removal (measured in the scratch copy, files removed: `media.go`, `media_test.go`, `media_quality_cat001_test.go`, `media_usermeta_authz_test.go`)

- `go build ./...` → rc 0. `go vet ./internal/handlers/ ./internal/media/...` → rc 0.
- `go test ./internal/handlers/` → `ok catalogizer/internal/handlers 70.693s`.
- `go test ./internal/nilslice_guard/` → **FAIL**: `guard_test.go:272: stale accepted entry "internal/handlers/media.go var mediaItems ..."` and `"... var mediaTypes ..."`. Control: the same test at HEAD in the main checkout → `ok`. So a removal commit MUST delete `guard_test.go:224-225`.
- `go test ./internal/errleak/` → ok. `go test -run '^(TestWrapHTTPHandler_NoParamsLeavesMuxVarsUsable|TestPXF23R5b_SourceSweep)$' ./handlers/` (source sweeps that walk `internal/handlers`) → ok.
- Full root `./handlers/` package in the scratch copy: INCONCLUSIVE — `FAIL catalogizer/handlers 600.218s`, i.e. the default 10-minute `go test` timeout panic (stack ends in `go-sqlcipher ... sqlite3.go:672`), not a test assertion. It cannot be caused by the removal: `go list -deps ./handlers/` and `go list -test -deps ./handlers/` both contain `catalogizer/internal/handlers` 0 times, so the removed files are not in that test binary. Not re-run at HEAD with a longer `-timeout` (UNCONFIRMED whether HEAD also exceeds 10 minutes on this host).
- Auth-surface guard (`main_chpw_authsurface_test.go`) and API-contract/route-dump gates (`api_contract_gate_*_test.go`) derive from `main.go` registrations / a route dump of the built binary; since no route ever registered the legacy handler, the route set is unchanged by construction. Not executed in the scratch copy (UNCONFIRMED by run; FACT by source: zero registrations).
- Coverage: no per-package floor references it (`coverage/exclusions/catalog-api.yaml` has no media entry). Removing ~676 tested lines changes the `internal/handlers` package percentage; direction/size not measured (UNCONFIRMED).
- SonarQube: removes the CRITICAL `go:S3776` at `media.go:106` (one fewer open issue for G5).

## 6. Tests that would be lost and whether they cover anything unique

- 20 `media_test.go` tests exercise only this handler's SQL against a media-DB fixture. No live route runs this code, so they guard no user-visible behaviour.
- `media_usermeta_authz_test.go` (HAUTHZ-O1 legacy half): the live half is covered by the routed-handler tests added in the same commit `7d0271c66` (`git show --stat`: `catalog-api/handlers/horizontal_authz_unit_test.go` +27, `catalog-api/horizontal_authz_binary_test.go`), which stay after a removal. The HAUTHZ-O1 evidence script `o1/mut.sh:31-32` (mutants M4/M5) would no longer be re-runnable; the recorded evidence files remain.
- `media_quality_cat001_test.go`: the ONLY test of `models.QualityDistribution(db, withDVD=false)` (`internal/media/models/quality_distribution.go:34`); the `true` branch stays covered by `internal/media/manager_quality_cat001_test.go:12` and `manager_test.go:1095,1114`. After removal the `withDVD=false` branch has no caller at all (`git grep QualityDistribution(` → only `media.go:638` and `manager.go:498`), so it becomes dead too — not a lost user capability.
- `models.MediaSearchRequest` (`internal/media/models/media.go:268-270`) is used only by `media.go:107`; it would become unused (follow-on dead-code candidate, keep for a separate §11.4.124 pass).

## 7. Reverse hypothesis: does any user-visible capability exist ONLY in it?

No user-visible capability exists in it at all, because no route reaches it and the database it reads is never written by the server (section 2). Code-level capabilities that have NO live equivalent:
1. Faceted media search (year range, genre, min rating, has external metadata, quality, storage root, watched status) — `media.go` SearchMedia. Real product gap: clients already send year/rating/quality filters (section 3) that the live search ignores. But the legacy implementation cannot serve it (wrong param names, wrong database).
2. Queued directory analysis through the `internal/media` detector/analyzer with priority — depends on the unwired MediaManager stack.
3. A 24 h "recent activity" counter (`analyses_24h`, `new_items_24h`, `metadata_updates_24h`) — `directory_analysis` exists only in the media DB.
4. `GET /media/types` path itself (live equivalent is `/entities/types`); no client uses it.

## 8. Options

A. **Remove** (recommended, needs §11.4.122 operator approval). Commit contents:
   - delete `catalog-api/internal/handlers/media.go`, `media_test.go`, `media_quality_cat001_test.go`, `media_usermeta_authz_test.go`;
   - edit `catalog-api/internal/nilslice_guard/guard_test.go` (drop lines 224-225), else that gate FAILs (measured);
   - docs: `catalog-api/MEDIA_SYSTEM_README.md` (wiring snippet + endpoint list), `docs/architecture/ARCHITECTURE.md:1044`, close CAT-LEGACY-MEDIAHANDLER-1 in the tracker (DB + regenerated `docs/FOLLOW_UPS_NEXT_PHASE.md`/`WORKABLE_ITEMS_REGISTER.md`), note in `docs/qa/horizontal-authz-2026-11-16/INVENTORY.md` that M4/M5 targeted removed code;
   - commit message cites this memo's history evidence (§11.4.124) and `Roadmap-Item: CAT-LEGACY-MEDIAHANDLER-1`;
   - risk: none observed for users or routes; the only behavioural loss is test coverage of dead code.
   - not included (separate items): the rest of the unwired `internal/media` stack (manager, realtime, detector, analyzer, encrypted media DB; T374/O-10), `models.MediaSearchRequest`, the `withDVD=false` branch.
B. **Keep unrouted** (status quo). Cost: 676 lines + 23 tests maintained for no user, a CRITICAL Sonar smell, two guard exceptions, recurring audit noise (HAUTHZ-FU-O1 was raised against it).
C. **Revive a subset.** Only the faceted search is worth it, and it should be built in the live `MediaBrowseHandler.SearchMedia` / `SearchEntities` on the main DB using the parameter names clients already send (`year_min`, `year_max`, `rating_min`, `quality`), not by routing this handler (wrong DB, wrong names, `/media/search` already owned). That is a new feature item, independent of whether this file is removed.

## 9. Recommendation

Option A (remove, after operator approval), plus file new items for the real gaps this investigation found:
- client search filters `year_min/year_max/rating_min/quality/genre` silently ignored by `GET /media/search` (Android `CatalogizerApi.kt:44-56`, web `modules.d.ts:146-160`);
- clients reference unrouted `GET /media/{id}/metadata` (web `mediaApi.ts:42`, Android `CatalogizerApi.kt:61`);
- web `mediaApi.analyzeDirectory` sends the legacy `{directory_path}` body to the live `/media/analyze` (would be 400; no UI caller);
- the remaining unwired `internal/media` stack (already T374 / O-10).

Open (UNCONFIRMED): field-level parity of `/entities/types` and `/media/stats` vs the legacy JSON shapes; the coverage delta; live behaviour of the three client findings (static reading only, nothing run against a server).
