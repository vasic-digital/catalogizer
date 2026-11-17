# CAT-LEGACY-MEDIAHANDLER-1, part 2: the whole legacy media system

Date: 2026-11-17. HEAD `81cccc88f` (main). Read-only investigation; nothing in the checkout was edited or deleted.
Extends `docs/qa/CAT-LEGACY-MEDIAHANDLER-1-memo-20261117.md` (memo 1, handler only). Evidence is static (source, `git log`, `grep`);
nothing was run against a live server, so runtime claims are marked UNCONFIRMED where they matter.

## 0. Short answer to the operator

- **Never wired, not "died".** The whole `internal/media` subsystem (manager, detector, analyzer, realtime watchers, encrypted
  `media_catalog.db`, and the legacy `internal/handlers.MediaHandler`) arrived in one bulk commit `16843b133` (2025-10-04, message
  "Implementations", 203 files, +42,174 lines) together with `MEDIA_SYSTEM_README.md`, whose "Add to your main.go" snippet
  (`catalog-api/MEDIA_SYSTEM_README.md:293-305`) was never applied. `main.go` in that same commit (146 lines) contains no
  media reference (`git show 16843b133:catalog-api/main.go | grep -i media` returns nothing). No later commit ever constructed
  `NewMediaManager` / `NewMediaAnalyzer` / `NewDetectionEngine` / `NewSMBChangeWatcher` / `NewEnhancedChangeWatcher` /
  `NewMediaDatabase` / `NewProviderManager` outside `internal/media` itself (grep below, section 2).
- **There IS another mechanism, and it is the real product.** Commit `3851b594a` (2026-02-18, "Add Media Entity Catalog System —
  structured media from flat files") rebuilt the same idea in the MAIN database: migration v8 with `media_types`, `media_items`,
  `media_files`, `external_metadata`, `user_metadata`, `directory_analyses`, `detection_rules`; six repositories; the
  `AggregationService` post-scan hook on the `UniversalScanner`; a title parser; 14 `/api/v1/entities` endpoints; web pages.
  It reuses only the TYPES of the old system (`internal/media/models`). Its commit message does not mention the old system and no
  document records the supersession decision (searched: commit message, `docs/`, `specs/`, `Website/`).
- **A third generation exists too**: the submodule `digital.vasic.media` (`submodules/media`, first commit `428e91b` 2026-02-16,
  packages `analyzer detector manager metadata models provider quality`), an extraction of the same design. The server imports only
  `pkg/quality` (image quality gate, live) and constructs `pkg/detector` into a registry field nobody reads (section 1, row 9).
- **So the feature is partly implemented, by a different, smaller pipeline.** Entity aggregation, TMDB enrichment, LLM fallback,
  cover-art resolvers, file-level duplicates and an encrypted main DB are live. Several capabilities that README, Website and
  `MEDIA_SYSTEM_README.md` advertise exist only in the dead code or nowhere at all (section 4).

## 1. Subsystem table

Wired = constructed on a production path from `main.go`. Commands behind every "no": section 2.

| # | Subsystem (file) | Designed to | Wired in production? | Live replacement today | Gap (capability with no live implementation) | Claimed in docs as working? |
|---|---|---|---|---|---|---|
| 1 | `MediaManager` `internal/media/manager.go:53` | own the media DB, detector, providers, analyzer, watcher; `Start()` watches hard-coded `nas1`/`nas2` paths (`manager.go:119-122`); needs `MEDIA_DB_PASSWORD` (`:56`) | **No** (no non-test caller of `NewMediaManager`) | `main.go:646-684`: repositories on the main DB + `UniversalScanner` + `AggregationService` + event bus | none by itself (orchestrator) | `MEDIA_SYSTEM_README.md:39-55` architecture box; `docs/architecture/MEDIA_RECOGNITION_PIPELINE.md:68-73` |
| 2 | Encrypted media DB `internal/media/database/database.go:30`, `schema.sql` (55 seeded media types, `schema.sql:180`) | separate SQLCipher file `media_catalog.db` | **No** (only `manager.go:64` constructs it; legacy handler reads it) | main DB migration v8, `database/migrations_sqlite.go:657-687`, 11 media types; main-DB encryption via `config/config.go:54-58` (`encryption_key_file`) | the 55-type taxonomy and `change_log`; nothing writes `media_catalog.db` | `docs/architecture/DATABASE_SCHEMA.md:981` describes it as a live separate encrypted DB. Note: an EMPTY, git-ignored `internal/media/database/media_catalog.db` (0 bytes, 2026-10-30, `.gitignore:29 *.db`) sits in the source tree, a test or run artefact (origin UNCONFIRMED) |
| 3 | Detection engine `internal/media/detector/engine.go:23` | pattern/structure/content detection with confidence, rules loaded from DB | **No** (only `manager.go:70`) | `AggregationService.classifyLeafMediaType` (`internal/services/aggregation_service.go:243-258`: extension → video/audio/software/comic/book) + title parser using `digital.vasic.entities` (`internal/services/title_parser.go`) | confidence scoring, content inspection, DB-driven detection rules, the 55 categories (anime, podcast, sports, courses ...) | Yes: Website `docs/developer-guide/media-pipeline.md:21,91` ("The detector in `internal/media/detector/` identifies ..."), `catalog-api/AGENTS.md:33`, `MEDIA_SYSTEM_README.md:58-61` |
| 4 | Analyzer `internal/media/analyzer/analyzer.go:62` | queued directory analysis, quality detection, version aggregation | **No** (only `manager.go:81`) | aggregation walk (title-granular, `aggregation_service.go:70`); `POST /media/analyze` → `MediaQueryHandler.AnalyzeMedia` (lookup, not analysis) | **quality detection** (resolution/codec/bitrate/HDR): `media_files.quality_info` is written ONLY by `LinkFileToItem`, and both production callers pass `nil` (`aggregation_service.go:614,970`); `GET /media/:id/quality` returns only path/size/mime (`handlers/stub_handler.go:362-405`). So `quality_info` is always NULL (UNCONFIRMED at runtime; no other writer found by grep for `quality_info`/`UPDATE media_files`) | Yes: README `:16,:20`, Website `features.md:91`, `changelog.md:280`, `media-pipeline.md:28,125`, `MEDIA_SYSTEM_README.md:18-22` |
| 5 | Realtime watchers `internal/media/realtime/watcher.go:53`, `enhanced_watcher.go:82` (added `3c002a621` 2025-10-09 "Changes monitoring") | fsnotify/SMB change watching, debounce, auto re-analysis, change log | **No** (only `manager.go:84`; enhanced watcher has no caller at all) | nothing watches storage. Live real-time = scan-progress events via `internal/eventbus` (`main.go:682-684`) to WebSocket. `UniversalRenameTracker`/`RenameTracker` (same commit) also have no production caller; module registry "watcher" entry only reads `watcher.DefaultConfig()` and marks itself `Initialized: true` (`internal/modules/registry.go:292-302`; its comment cites a nonexistent `internal/smb`) | **automatic change detection / re-catalog on file change** — user must trigger scans | Yes: README `:5,:14`, Website `features.md:93`, `MEDIA_SYSTEM_README.md:24-28`. Website `developer/architecture.md:96`, `docs/developer-guide/websockets.md:135,147`, `docs/architecture/CONCURRENCY_PATTERNS.md:764` place the event bus in `internal/media/realtime/event_bus.go`, a file that does not exist |
| 6 | `ProviderManager` + 15 providers `internal/media/providers/providers.go:75,111-144` | multi-provider search/aggregation (TMDB, IMDB, TVDB, MusicBrainz, Spotify, LastFM, IGDB, Steam, Goodreads, OpenLibrary, AniDB, MyAnimeList, YouTube, GitHub) | **No** (`NewProviderManager*` called only by `manager.go:78`). The PACKAGE is live only for `LLMProvider` (`main.go:852`) and `ProxyConfiger` | metadata: TMDB inline in `AggregationService` (`aggregation_service.go:728-822`) and `MediaEntityHandler.fetchTMDBMetadata` (`handlers/media_entity_handler.go`, refresh route `main.go:1308`, admin) with LLM fallback (`:864`); cover images: resolver chain `main.go:749-755` (cache, external_metadata, local_scan, Fanart.tv, Cover Art Archive, IGDB, LLM image search) | even inside the dead manager, 10 of 14 non-LLM providers are stubs returning `nil, nil` with "requires API key configuration" (`providers.go:1028-1032, 1118-1146, 1218-1240` and GetDetails `:846-940`): TVDB, Spotify, LastFM, IGDB, Steam, Goodreads, AniDB, MyAnimeList, YouTube, GitHub. Real code only in TMDB, IMDB, MusicBrainz, OpenLibrary, all unreachable. No live producer of `musicbrainz_release_id` found, so the Cover Art Archive resolver (`cover_art_archive_resolver.go:62`) may never fire (UNCONFIRMED) | Yes: README `:19` ("TMDB, IMDB, TVDB, MusicBrainz, Spotify, Steam"), Website `features.md:92` ("13 providers"), `faq.md:14,107-110`, `changelog.md:261`, `architecture.md:172`, `media-pipeline.md:159,166`; code comment `internal/services/media_recognition_service.go:1288-1299` calls `internal/media/providers` "the primary metadata pipeline" — it is never constructed |
| 7 | Models `internal/media/models` | shared types | **Yes** (repositories, `AggregationService`, `MediaEntityHandler`, `collection_handler.go`) | itself | `QualityDistribution(withDVD=false)` and `MediaSearchRequest` used only by the dead handler (memo 1 §6) | — |
| 8 | Legacy `internal/handlers.MediaHandler` | REST for the above | **No** (memo 1) | `/media/*`, `/entities/*` handlers (memo 1 §4) | faceted search, `/media/types` (memo 1 §7) | `MEDIA_SYSTEM_README.md:150-197`, `docs/architecture/ARCHITECTURE.md:1044` |
| 9 | `digital.vasic.media` submodule (`submodules/media/pkg/*`) | standalone extraction of the same design (detector, analyzer with resolution/codec from path, provider registry, manager) | **Partly**: `pkg/quality` live in the image quality gate (`internal/services/quality_gate.go:17`); `pkg/detector` constructed into `reg.MediaDetector` (`internal/modules/registry.go:158`) which nothing reads (`grep MediaDetector` → only the field and the assignment); `pkg/analyzer`, `pkg/provider`, `pkg/manager` not imported | — | its path analyzer could supply the missing quality/codec fields, but is not called | registry comment `registry.go:157` says it "augments the existing internal/media/detector pipeline", which is unwired |
| 10 | Adjacent second recognition stack `internal/services/{movie,music,book,game_software}_recognition_provider.go` and `CoverArtService` providers | per-type recognition via external APIs | **No** for the four `New*RecognitionProvider` (no production caller); `MediaRecognitionService` is built with all API URLs empty (`main.go:537`) so its fetchers return `ErrMetadataProviderNotConfigured`; `CoverArtService.searchProvider` MusicBrainz/LastFM/iTunes return `ErrCoverArtProviderNotImplemented` (`cover_art_service.go:477-503`); Spotify/Discogs fall to "unsupported provider" | resolver chain (row 6) | music cover search via Last.fm/Spotify/iTunes/Discogs | Website `features.md:175` (MusicBrainz, Last.fm, Spotify, iTunes, Discogs), `media-pipeline.md:166` |

Live and not a gap (for balance): file-hash duplicates (`main.go:1024,1026,1098,1108`); per-title entity hierarchy;
TMDB enrichment (needs `TMDB_API_KEY`); LLM fallback (needs a configured LLM); main-DB encryption (key file, `config/config.go:54-58`;
runtime not exercised here, UNCONFIRMED in this memo).

## 2. Commands behind "not wired"

```
# constructors, non-test, whole catalog-api
grep -rn --include=*.go "NewProviderManager\|NewMediaAnalyzer\|NewDetectionEngine\|NewSMBChangeWatcher\|NewEnhancedChangeWatcher\|NewMediaDatabase" . | grep -v _test.go
  -> only definitions + internal/media/manager.go:64,70,78,81,84 (+ providers.go:71 internal delegation)
grep -rn --include=*.go "NewMediaManager(" . | grep -v _test   -> definition only (memo 1)
# importers outside the package, non-test
internal/media/{analyzer,database}: manager.go, realtime/*, internal/handlers/media.go (all dead)
internal/media/{detector,realtime}: manager.go / analyzer.go only
internal/media/providers: main.go (LLMProvider only), handlers/media_entity_handler.go (LLMProvider, ProxyConfiger)
# second stack
grep NewMovieRecognitionProvider|NewMusicRecognitionProvider|NewBookRecognitionProvider|NewGameSoftwareRecognitionProvider (non-test, outside internal/tests) -> none
grep "NewUniversalRenameTracker(\|NewRenameTracker(" non-test -> definitions only
grep "LinkFileToItem(" non-test -> aggregation_service.go:614,970, both qualityInfo=nil
grep "MediaDetector" non-test -> registry.go:97,158 only
```
Control needles: the same grep finds `NewLLMProvider` at `main.go:852` and `NewCoverArtService` at `main.go:826`, so the
instrument sees live constructors; `grep -c "INSERT OR IGNORE INTO media_types" database/migrations_sqlite.go` = 11 and the
legacy `schema.sql` VALUES block = 55 rows (awk count), matching the 3851b594a commit message ("11 seeded media types").

## 3. History: why abandoned / superseded

| Date | Commit | Event |
|---|---|---|
| 2025-10-04 | `16843b133` "Implementations" | whole legacy system + README with an unexecuted wiring snippet, in a 42k-line bulk commit |
| 2025-10-09 | `3c002a621` "Changes monitoring" | `enhanced_watcher.go`, `rename_tracker.go`, `protocol_handlers.go`, universal scanner; watchers never wired |
| 2026-02-16 | submodule `428e91b` | `digital.vasic.media` extraction of the same design |
| 2026-02-18 | `3851b594a` | Media Entity Catalog on the main DB: the de-facto replacement; legacy not mentioned |
| 2026-03-25..04-18 | `d94036077`, `223035c2a`, `097555141`, `99c759e8c` | TMDB/OpenLibrary/MusicBrainz work, proxy, SSRF guard added INSIDE `internal/media/providers` (still unreached except LLM) while the live TMDB path was written inline in the handler and the aggregation service |
| 2026-11-02..16 | `4cfcac6b5`, `7d0271c66` | correctness fixes to the dead handler |
| spec 001 | `specs/001-.../docs/01-system-architecture-map.md:163,665` (O-10), `tasks.md:899` (T374) | already identified as unwired; T374 BLOCKED-ON ODG-23 (history first, decide per item, §11.4.122 owner confirmation) |

No recorded decision to abandon. Reading of the evidence (inference, UNCONFIRMED as intent): the old system was generated as a
design with a separate encrypted DB, the integration was skipped, and when entities were needed in February 2026 the work was
redone on the main DB rather than wiring the old stack; later provider work kept landing in the old package out of habit.

## 4. Unimplemented-but-advertised features (§11.4.266 claim-vs-reality)

| Claim (where) | Reality |
|---|---|
| "Real-time Monitoring: continuously monitors storage sources for changes" — `README.md:5,14`; `Website/features.md:93`; `MEDIA_SYSTEM_README.md:24-28` | no watcher runs; scans are triggered (row 5) |
| "External metadata from TMDB, IMDB, TVDB, MusicBrainz, Spotify, Steam" / "13 providers" — `README.md:19`; `Website/features.md:92`; `faq.md:14,107-110`; `changelog.md:261` | live: TMDB only (+ LLM, Fanart.tv, IGDB for images, Cover Art Archive UNCONFIRMED); IMDB/TVDB/MusicBrainz/Spotify/Steam metadata: none. `README.md:345-347,620-621` ask for `SPOTIFY_CLIENT_ID/SECRET` and `STEAM_API_KEY`, which no Go code reads at all (`grep -rn "SPOTIFY_CLIENT\|STEAM_API_KEY" --include=*.go catalog-api` excluding tests → 0 hits) |
| "50+ media types detected" — `Website/features.md:90`; `faq.md:14`; `changelog.md:260`; `MEDIA_SYSTEM_README.md:8` | 11 entity types, 5 extension classes (row 3); the 55 exist only in the unused `schema.sql` |
| "Quality Analysis: automatic quality detection and version tracking" / "resolution, codec, bitrate" — `README.md:16,20`; `Website/features.md:91`; `changelog.md:280` | `quality_info` never written; quality endpoint lists files only (row 4) |
| "Media detection pipeline: detector → analyzer → providers" — `Website/features.md:94`; `docs/developer-guide/media-pipeline.md`; `catalog-api/AGENTS.md:33`; `catalog-api/ARCHITECTURE.md:55`; `docs/architecture/MEDIA_RECOGNITION_PIPELINE.md:68-73` | describes dead packages and files that do not exist (`realtime/event_bus.go`, `detector/detector.go`, `analyzer/worker_pool.go`) |
| Music cover art from Last.fm, Spotify, iTunes, Discogs — `Website/features.md:175` | `ErrCoverArtProviderNotImplemented` / unsupported (row 10) |
| SQLCipher encrypted DB — README, Website | backed by the MAIN DB (live), not the legacy one; Website still documents `DB_ENCRYPTION_KEY` env while config uses a key file (stale doc, UNCONFIRMED which one the binary honours today) |
| Challenges CH-094..CH-097 (`challenges/ch094_098_provider_verification.go`) "Provider Manager media type routing", OpenLibrary/MusicBrainz provider | PASS by `os.ReadFile` + `strings.Contains` on `providers.go` source (30 such calls in the file) of a `ProviderManager` that production never constructs: a §11.4 grep-only PASS on dead code |

Client-side gaps already listed in memo 1 §3 (ignored search filters, unrouted `/media/{id}/metadata`, legacy `analyzeDirectory` body) still stand.

## 5. Decision memo

All removals need §11.4.122 operator confirmation and a §11.4.124 record (this memo + memo 1 provide the history); T374 / ODG-23 is the tracking item.

**A. Remove (dead, live replacement exists, nothing reaches it):**
1. `internal/media/manager.go` (+ tests), `internal/media/realtime/` (both watchers), `internal/media/detector/`, `internal/media/analyzer/`,
   `internal/media/database/` (+ `schema.sql`), `internal/handlers/media.go` (memo 1 option A, incl. `nilslice_guard/guard_test.go:224-225`).
   Order: handler first (memo 1), then manager+realtime (only importer of the rest), then analyzer/detector/database. Each step: `go build ./... && go vet` + affected package tests in a scratch copy, like memo 1 §5.
2. In `internal/media/providers`: the `ProviderManager`, `BaseProvider` and the 14 provider types (keep `LLMProvider`, `ProxyConfiger`, cooldown helpers if used by it — UNCONFIRMED, check importers of `cooldown.go`/`lazy_provider.go` before cutting). Alternative: move TMDB/OpenLibrary/MusicBrainz real code into option B instead of deleting.
3. Dead `internal/modules/registry.go` theatre: `reg.MediaDetector` assignment (`:158`) and the watcher "Initialized: true" entry (`:292-302`) — or make them real (B).
4. The four unconstructed `*_recognition_provider.go` services and the `UniversalRenameTracker`/`RenameTracker`: separate §11.4.124 pass (they are outside `internal/media`; not investigated in depth here).
5. Retire or rewrite challenges CH-094..CH-097 so they exercise a live path (§11.4 grep-only PASS).

**B. Finish wiring (real gaps that docs promise; file as new Feature items, built on the LIVE pipeline, not by reviving the legacy stack):**
1. Quality detection: populate `media_files.quality_info` in `AggregationService` (e.g. via `digital.vasic.media/pkg/analyzer` from file names, or ffprobe) so `/media/:id/quality`, AndroidTV and smart rules get data.
2. Real-time change detection: a watcher for local roots (fsnotify via `digital.vasic.watcher`) triggering incremental rescans; remote protocols would need polling. Only if the operator wants the feature; otherwise drop the claim (C).
3. Additional metadata providers (TVDB/MusicBrainz/IGDB metadata, not only images) as resolvers/enrichers next to the live TMDB path; and consolidate the two inline TMDB clients (`aggregation_service.go:728-822`, `media_entity_handler.go`) into one.
4. Faceted search and client filter names (memo 1 §9).

**C. Update claims now (cheap, independent of A/B, closes the §11.4.266 gaps):**
`README.md:5,14,16,19,20,345-347,620-621`; `Website/features.md:90-94,175`; `Website/faq.md:14,107-110`; `Website/changelog.md:260-261,280` (historical — annotate, don't rewrite);
`Website/docs/developer-guide/media-pipeline.md`, `websockets.md:135,147`; `Website/developer/architecture.md:96`; `catalog-api/AGENTS.md:33`; `catalog-api/ARCHITECTURE.md:55`;
`docs/architecture/{MEDIA_RECOGNITION_PIPELINE.md:68-73,DATABASE_SCHEMA.md:981,CONCURRENCY_PATTERNS.md:129,752,764,ARCHITECTURE.md:1044}`; `docs/api/WEBSOCKET_EVENTS.md:360-376`;
`catalog-api/MEDIA_SYSTEM_README.md` (mark as historical design, or delete with A); code comments `internal/services/media_recognition_service.go:1288-1299`, `internal/modules/registry.go:157,294`.

Recommended order: C immediately (doc-only), A1 with memo 1's removal (one reviewed change per step), B as tracked Feature items with operator priority; B2 needs an operator yes/no.

## 6. Open / UNCONFIRMED
- Runtime state of `quality_info` in a real catalog DB (static: no writer).
- Whether any live path supplies `musicbrainz_release_id` to the Cover Art Archive resolver.
- Whether `ProviderCooldown` (`internal/media/providers/cooldown.go`) has a live user (no `providers.` caller found outside the package; the live TMDB cooldown `isTMDBOnCooldown` is a separate handler-side helper).
- Field parity of live vs legacy JSON (memo 1 open items).
- Intent behind the supersession (no recorded decision; section 3 is inference).
