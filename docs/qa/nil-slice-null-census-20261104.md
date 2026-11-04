# Nil slice / map serialised as JSON `null` - class census (2026-11-04)

Defect class (§11.4.276(C)): a Go nil slice or map is written by `encoding/json` as `null`, where the API documents an array
(clients iterate it and crash). Reproduced first (§11.4.292) on main `9f97d6de` by the live contract sweep extended with `nullScan`
(RED: 18 violations over 17 operations, evidence `api-contract-live-20261103/nil-slice-red/`), then fixed at the source and re-run GREEN.

## Instrument and control needle (§11.4.273)

* Live: `catalog-api/api_contract_live_test.go` `nullScan` runs on every happy 2xx JSON body: whole-body `null`, `$.data` `null`, and `null`
  at any key named in `nilListKeys` where the spec gives no type. Nil pointers (`*string`, `*time.Time`, ...) are legitimately `null`
  and indistinguishable on the wire from a nil slice, so scalar members are not judged. Positive control: the RED run flagged the known
  members before the fix; the GREEN run reports 0.
* Structural: `catalog-api/internal/nilslice_guard` parses the code (AST) and fails on a `var x []T` / `var x map[K]V` without
  initialiser that is returned unchanged: every function in `repository/`, exported functions in `services/`, `handlers/`,
  `internal/services/`. Positive control `TestDetectorSeesKnownOffender` (a seeded offender must be flagged; an initialised and a
  non-returned variable must not). RED on main: 55 findings in `repository/`; GREEN after: 0.
* Unit: `repository/nil_slice_empty_test.go` (real migrated SQLite, empty result -> `[]`; nil `Permissions` -> `[]`; RED shown by
  reverting the sources), plus the strengthened `TestAnalyticsRepository_GetGeographicData_Real` (it asserted the old `nil`).

## Inventory and dispositions

### Live members found by the reviewer / sweep (all FIXED)

| Operation | Member | Root cause | Fix point |
|---|---|---|---|
| GET /entities/duplicates | `groups` | `DuplicateEntityGroup` list `var` | repository |
| GET /files | `data.files` | `file_repository` list `var` | repository |
| GET /stats/duplicates/groups | `data` | `stats_repository.GetTopDuplicateGroups` | repository |
| GET /stats/scans | `data.scans` | `stats_repository.GetScanHistory` | repository |
| GET /configuration, /admin/config | `sections` | `ConfigurationServiceAdapter.GetConfigurationSchema` returned an empty struct (stub) | adapter delegates to the real service (nil inner: `Sections: []`) |
| GET /recommendations/similar/{id} | `local_items` | `findLocalSimilarItems` nil | normalised in `GetSimilarItems` |
| GET /logs/collections/{id}/analyze | `insights` | `generateInsights` `var` | initialised |
| GET,PUT /roles/{id}, GET,PUT /users/{id} | `permissions`, `role.permissions` | `models.Permissions` nil / stored `null` | `Permissions.MarshalJSON` renders nil as `[]` (also fixes every other Role/User response and the stored JSON) |
| POST /logs/collections/{id}/export | whole body | `GetLogEntries` nil then `json.MarshalIndent(nil)` | repository + defensive guard in `ExportLogs` |
| GET /analytics/system | `geographic_distribution.locations` | `GetGeographicData` `var locations` | initialised (map element type, hand edit) |

### Repository list functions (FIXED, `var out []T` -> `out := []T{}`)

65 declarations across 22 files (61 generic rewrites, 1 hand edit, 3 `Tags` sites; per-file count):

| File | Sites |
|---|---|
| `repository/analytics_repository.go` | 6 |
| `repository/asset_repository.go` | 2 |
| `repository/configuration_repository.go` | 4 |
| `repository/conversion_repository.go` | 1 |
| `repository/crash_reporting_repository.go` | 4 |
| `repository/directory_analysis_repository.go` | 1 |
| `repository/duplicate_entity_repository.go` | 2 |
| `repository/error_reporting_repository.go` | 3 |
| `repository/external_metadata_repository.go` | 1 |
| `repository/favorites_repository.go` | 5 |
| `repository/file_repository.go` | 8 |
| `repository/image_quality_repository.go` | 1 |
| `repository/log_management_repository.go` | 4 |
| `repository/media_collection_repository.go` | 2 |
| `repository/media_file_repository.go` | 4 |
| `repository/media_item_repository.go` | 3 |
| `repository/playlist_repository.go` | 2 |
| `repository/share_identity_binding_repository.go` | 1 |
| `repository/stats_repository.go` | 4 |
| `repository/sync_repository.go` | 3 |
| `repository/user_metadata_repository.go` | 1 |
| `repository/user_repository.go` | 3 |

Favorites `Tags` (`*[]string`): a stored JSON `null` now yields `[]`.

### Service layer exported list functions (FIXED, 31)

`GetAvailableTemplates`, `GetRecommendedFavorites`, `BulkAddFavorites`, `CollectLogs`, `GetAllActive`, `ListPath`, `SearchFiles`,
`GetDirectoriesBySize`, `GetDuplicateGroups`, `GetSMBRoots`, `SearchCoverArt`, `GenerateVideoThumbnails`, `ScanLocalCoverArt`,
`DetectDuplicates`, `GetSupportedLanguages`, `SearchLyrics`, `GetConcertLyrics`, `GetContinueWatching`, `GetBookmarks` (x2),
`GetUserPlaylists`, `GetPlaylistItems`, `GetHighlights`, `GetRenameEvents`, `GetHosts`, `ListDirectory`, `BrowseShare`,
`LoadSMBIdentitiesFromEnv`, `SearchSubtitles`, `GetSubtitles`, `GetWatchHistory`.

### Scoped out (not fixed, with reason)

* Nil pointer members (`include_patterns`, `exclude_patterns`, `language`, `time_zone`, `md5`, `sha1`, `sha256`, `blake3`, `deleted_at`,
  `parent_id`, `duplicate_group_id`, `last_login_at`, `description`, ...): legitimately nullable optionals, not collections. Follow-up
  (spec only): mark them `nullable: true`.
* Unexported helpers in `services/` and `internal/services/` that return nil as "nothing found" and are consumed by Go callers (e.g.
  `filterLogsByDate`, `parseSRT`, `extractKeywords`, `getTopLevelDirectories`, `pdfBaseEnv`, `buildRuleCondition` args): 54 findings
  when the guard is run without the exported-only filter. Their results reach HTTP only through an exported function (fixed above or
  covered by the live sweep). Follow-up: extend the AST guard to them only with a per-caller nil-check audit.
* Locally scoped `var args []interface{}`, `var updates []fileUpdate`, `var found []legacy`: query-argument and internal accumulators, never
  serialised.
* Handlers building `gin.H{"files": files}` already use `make([]T, 0)` (stub_handler, scan_handler) or repository results fixed above.
* Members that only appear when seeded data is absent for operations that are happy-path exceptions in
  `api_contract_happy_exceptions.tsv`: covered structurally by the AST guard at the repository layer, not by the live sweep.
