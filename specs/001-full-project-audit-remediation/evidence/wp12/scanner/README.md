# identity: WP-12 PA-01 (settings-key contract) and PA-03 (generic breadth-first scanner), evidence - REVISION 2 (WF22 fix round)
# head: 4c8b07b7 (main) with a dirty working tree; ALL code is UNCOMMITTED (no git add/commit/push was made by any worker of this item)
# round 1: 2026-10-07 (evidence logs 18:15 - 19:40 UTC), author: Sonnet worker. independent review WF22 (Opus, one pass): NO-GO, 17 IMPORTANT findings (14 source-defect), 19 MINOR, 21 of 37 reviewer mutants survived the author's tests
# round 2: 2026-10-08 (logs `fix-r2-*`), single fixer (Sonnet) under 11.4.276. A fresh independent review of revision 2 (11.4.142 / 11.4.209) is OWED, not performed
# verify: `cd specs/001-full-project-audit-remediation/evidence/wp12/scanner && sha256sum -c SHA256SUMS`

## Round 2 in one table (what the review found and where it is closed)

Full detail: `fix-r2-convergence.md` (classes, member inventories, structural decisions, what is NOT fixed), `fix-r2-class-inventory.txt` (the control-needled censuses), `fix-r2-reproducibility.txt` (P1).

| Review finding | Closed by (source) | Pinned by (test) |
|---|---|---|
| R1 cover art builds an SMB-shaped map for every protocol | `cover_art_service.go` -> `SettingsFromRoot`; `models.StorageRootConnColumns*` | `TestWF22_R1_Gate_...` (AST: every `CreateClient` caller), `TestWF22_R1_CoverArt...`, probe R1 |
| R17a/R17b path convention, `INSERT OR REPLACE` | `catalogPath` at the DB seam; UPDATE-then-INSERT in `insertFileRecord` | `TestWF22_R17_*` (real SQLite, production migrations), `TestWF22_R17b_*`, probe R17, `TestRealDB_*` (real servers) |
| T1 no DB-backed test | the above | same |
| H1 loader omits url / mount_point; H2 create cannot store them, protocol list fixed | `loadStorageRoot`, `CreateStorageRoot`, `supportedStorageProtocol` (= the factory's list) | `TestWF22_UserPath_*` (HTTP -> scanner -> real WebDAV client -> SQLite), `TestWF22_H1_*`, `TestWF22_H2_*`, probes H1/H2 |
| F5 WebDAV path replaced the URL path | `NewWebDAVClient` joins | `TestWF22_F5_*`, probe F5, `TestRealWebDAV_URLPathAndRootPathAreJoined` |
| R18 relative FTP path applied twice | `FTPClient.base` from `PWD` | `TestWF22_R18_*`, probe R18, `TestRealFTP_RelativeAndAbsoluteRootPathsScanTheSameSubtree` |
| R9 credential in `reason` | `redactURL` in every client error + `scrubSecrets` at `fail/cancel/setNote` and the event payload | `TestWF22_R9_*` (x3), probe R9, `TestWF22_UserPath_WebDAVListingFailure...`, `TestRealWebDAV_JobReasonNeverCarriesTheCredential` |
| R10 displayname as path segment; R11 `..` stripped inside names; F2 F3 W1 | `webdav_propfind.go` (href segment, direct children, streaming + bounded), `resolveURL` | `TestWF22_R10_*`, `_R11_*`, `_F3_*`, `_W1_*`, probes F2 F3 R10 R11 |
| R2 depth truncation; R4 bounds after a whole level; W1 | `handleListing` depth probe, `fetchLimit` + `WithListLimit`, wave processing | `TestWF22_A01_*`, `_A03_*`, `_A04_*`, `_R4_*`, probes R2 R4 R4b, `TestGenericScanner_MaxDepth*` |
| R3 empty share | `storage_roots.allow_empty` (migration v21), `ScanJob` root flag | `TestMigrationV21_*`, `TestWF22_R3_*`, probe R3, `TestRealEmptyDirectoryCompletesOnlyWithAllowEmpty` |
| R16 MLSD `.`/`..` as errors | scanner drops them silently; FTP client filters | probe R16 (in-process MLSD), `TestRealDB_FTP_*` (real: pre-fix `error_count` 48, now 0) |
| R12 R15 partial outcomes | `MaxSkippedRatio`, `MaxRecordFailRatio`, status note naming directories | `TestWF22_R12_*`, `_R15_*`, probes R12 R15 |
| R7 R13 P0 cancellation / timeout / panic | `FailTimeout`, `ctxKind`, `EventScanCancelled`, `listOnce` returns on `ctx.Done()` with a recover, panic path fails the visible status | `TestWF22_R7_*`, `_B03_*`, `_P0_*`, `_R13_*`, probes R7 R13 |
| R5 token not injective; R6 incremental claim | `%q` for the etag; `SupportsIncrementalScan` needs a token store | probes R5 R6, `TestWF22_A27_*` |
| R14 hostile names; S1 `#snapshot` | `validEntryName`, default skip list | `TestWF22_ValidEntryName`, `_A20_S1_*`, probe R14 |
| X1 `filepath` on remote paths | all remote-path sites use `path`; guard test | `TestWF22_X1_*`, `TestWF22_R17_SMBAndGeneric...` |
| F1 F4 C09 C02 D02 settings | `wholeNumber`, port range, empty domain, hint | `TestWF22_F1_*`, `_D02_*`, `_C09_*`, `_C02_F4_*`, probes F1 F4 |
| T2 21 surviving mutants | one test per survivor (named in the test) | `fix-r2-mutation_results.tsv` |
| T3 stale fingerprints / vague logs | every `fix-r2-*` log is stamped with its command and the tree fingerprint | `fix-r2-fingerprint.sh`, `fix-r2-fingerprint.txt` |
| T4 harness | `fix-r2-mutate.py`: private copy, fail-closed verdict, negative control first, shared-tree hash before/after | its output header |
| P1 P2 P3 P4 | reported / docs corrected / user docs / NFS recorded `blocked-unavailable` | `fix-r2-reproducibility.txt`, the docs |
| R7 (part) cancel endpoint, `scansApi.ts` `reason`, UI notification | NOT done: outside `catalog-api`, new API surface | tracked (below) |

## Round 2 evidence

| File | Content |
|---|---|
| `fix-r2-red-probes-filesystem.log`, `fix-r2-red-probes-services-handlers.log` | RED: the 24 reviewer probes, VERBATIM (`fix-r2-reviewer-probes-verbatim/`), on a snapshot of the pre-fix tree (`.audit/scratch/wf22fix/prefix`): 23 fail (the log shows `--- FAIL` per probe; R14 is informational and passes pre-fix); the filesystem log still carries the first attempt's `setup failed` lines for the other packages (the snapshot lacked symlinks for the other submodules; fixed before the second log) |
| `fix-r2-red-new-tests.log` | RED: the new `TestWF22_*` / `TestMigrationV21_*` tests on the same pre-fix snapshot plus name-only shims (`zz_red_shims.go`, no behaviour): 44 fail, 14 pass. The 14 that pass are the tests written for reviewer mutants whose behaviour was already right but untested (A03 A04 A13 A14 A15/16 A18 A27 B06/B07 C09 D02 E03 ...): their proof is the mutant they kill, not a RED |
| `fix-r2-realfs_red_prefix.log` | RED on the REAL pure-ftpd / Apache mod_dav stack, pre-fix tree: 5 of the 6 new real tests fail (DB write for FTP and WebDAV: leading-slash rows, `error_count 48`; relative FTP path; credential in the reason; `allow_empty`). Credentials of the throw-away stack are scrubbed (`REDACTED-IN-EVIDENCE`) |
| `fix-r2-green_full_x3.log` | GREEN `go test -count=3` of filesystem, internal/services, handlers, internal/handlers, database, models, eventbus, internal/tests, config, lifecycle |
| `fix-r2-green_race_x3.log` | GREEN `go test -race -count=3` of filesystem, internal/services, handlers, database, models, eventbus (the three connect-timeout tests of the filesystem package skipped: they wait 20+20+10 s on an invalid server and exercise no changed code) |
| `fix-r2-realfs_green_1.log`, `_2.log`, `_3_race.log` | GREEN on the real stack, 19 real tests each, the third with a `-race` binary |
| `fix-r2-vet_all.log`, `fix-r2-module_all.log` | `go vet ./...` and `go test ./...` of the whole module (go.mod / go.sum need no update with the working-tree submodule: the default read-only module mode built everything) |
| `fix-r2-mutation_results_pass1.tsv`, `_pass2.tsv`, `_pass3.tsv`, `fix-r2-mutation_results_realfs.tsv`, `fix-r2-realfs_mut_*.log` | mutation runs of the new harness (private copy, negative control first, fail-closed verdict). UNIT: 116 mutants (the 37 reviewer mutants plus the author's and the new-code mutants): pass 1 killed 45, 71 ERROR (host memory: `run_pinned` REFUSED or the linker was killed - no verdict); pass 2 re-ran those 71: 69 killed, 2 ERROR (E09 and H06: the mutant text did not compile, `declared and not used` - a harness defect); pass 3 re-ran the two with compiling mutants: 2 killed. TOTAL 116 killed, 0 survived, shared-tree `catalog-api` hash unchanged in all three passes. REAL stack: 11 mutants, 6 killed, 5 survived (N13 N18 N23 W03 T01: reasons in `fix-r2-convergence.md` section 6; each is killed by a unit test in the unit table) |
| `fix-r2-class-inventory.txt`, `fix-r2-reproducibility.txt`, `fix-r2-convergence.md` | class censuses with control needles; what is not committed; the convergence assessment |
| `fix-r2-fingerprint.sh`, `fix-r2-fingerprint.txt`, `fix-r2-run_realfs.sh`, `fix-r2-mutate.py`, `fix-r2-up.log` | tooling, the sha256 of every modified or untracked file the evidence was produced from, the stack start (ports only) |

Probe adaptations (each listed in the header comment of the adopted file): R3 (the default stays "an empty root fails", the probe asserts the reason names `allow_empty` and that the root flag completes it), R4 (a fake that honours the list limit; the reviewer's measure, the server-side size of `/a`, cannot be met by any scanner that lists `/a` at all), R6 (a production scanner must NOT claim incremental support; with a token store it skips), R13 (the abandoned call is drained so goleak is clean), R14 (control characters and names over 255 bytes are now rejected), R17 (the catalog convention is slashless), H2 (the probe registers `sftp` itself: the allow-list is the factory's list).

Assertions of earlier tests that changed, and why (a changed assertion is evidence only if it is not masking):
`webdav_client_test.go TestWebDAVClient_resolveURL` x3 (the old expectation encoded the "path replaces the URL path" bug, WF22 F5); `universal_scanner_test.go TestScanners_SupportsIncrementalScan` FTP and WebDAV back to `false` (the round-1 flip to `true` was the unearned claim the reviewer named, R6) and NFS `true`->`false` (the original stub claimed it while doing nothing; no production scanner has a token store); `generic_scanner_test.go` the max-depth tests now assert FAILURE (round 1 asserted the silent truncation as correct), the benign-error test uses a tree with enough readable directories (1 of 3 unreadable is a smaller catalog, 1 of 1 is an outage), the change-token and hostile-name assertions follow the new token form and the silent `.`/`..`; `scan_handler_test.go TestCreateStorageRoot_AllSupportedProtocolsAccepted` sends a `url` for webdav (a webdav root without one is refused: it can never be scanned).

## Deviations and facts (round 2)

1. The SQLite bundled with go-sqlcipher predates 3.24 (no `ON CONFLICT DO UPDATE`, no `DROP COLUMN`): measured, not assumed (the first upsert made 9 existing tests fail with `near "ON": syntax error`). The SQLite branch is UPDATE-then-INSERT in one transaction; the PostgreSQL branch is unchanged. The migration v21 PostgreSQL branch (`ADD COLUMN IF NOT EXISTS`) is not executed anywhere here (no PostgreSQL server).
2. A migration was added (v21, `storage_roots.allow_empty`) because the owner-visible per-root setting needs a column; if another worker also adds a v21 the conductor must renumber (`database/migrations.go`, the three `021_*.sql` files, `migrations_test.go` `latestVersion`).
3. go.mod / go.sum: unchanged by round 2. Their round-1 bumps follow the submodule's UNCOMMITTED go.mod; see `fix-r2-reproducibility.txt` for exactly which `catalog-api` files import which untracked submodule packages (`pkg/fabric`, `pkg/decorators`) and which requirement lines move. ORDER for the conductor: commit and push the submodule (with those packages and its go.mod), bump the pointer, then commit `catalog-api`.
4. NFS has no real-target evidence and cannot get one rootless (`syscall.Mount`): recorded `blocked-unavailable`; the NFS client's remote `FileInfo.Path` now uses `path`.
5. `docs/scripts/README.md` is not touched (instruction); the two companion guides have revision 2.

## UNCONFIRMED / not done (round 2)

- UNCONFIRMED: behaviour against the 7 NAS hosts (no NAS contact was made, as instructed). UNCONFIRMED: Synology `#snapshot` applies to those hosts (the skip is a default, `SkipDirNames` is configurable).
- UNCONFIRMED: PostgreSQL execution of the v21 migration and of `insertFileRecord`'s unchanged PostgreSQL branch with the new `parent`-path convention (no PostgreSQL server in this environment).
- Not done, tracked: a cancel endpoint (needs a job-context registry in `UniversalScanner`); `reason` and `scan.cancelled` in `catalog-web/src/lib/scansApi.ts` and the UI; a persistent change-token store (migration) so that a production scanner can truthfully be incremental; deleted-file detection.
- Abandoned listing: a client that ignores the context (the FTP library) keeps its call running after a cancel; the scan returns at once and the call's answer is dropped. The window in which the caller disconnects the client under that call is covered by a recover, not by a client change (the submodule is not this worker's).

---

# Round 1 record (2026-10-07). Superseded where Round 2 says so; kept unchanged below except two corrections marked [R2]

## What was built (repo-relative)

| Path | Content |
|---|---|
| `catalog-api/filesystem/settings.go` | PA-01: the single settings-key vocabulary, per-protocol schema, strict `ValidateSettings` (unknown key and wrong type are errors, values never echoed), `RegisterProtocol` registry for pluggable protocols, the one `SettingsFromRoot` mapping |
| `catalog-api/filesystem/factory.go` | validates before building; builds registered protocols; `SupportedProtocols` = built-ins then registered |
| `catalog-api/internal/services/universal_scanner.go` | `storageRootToSettings` delegates to `SettingsFromRoot`; the three empty scanner bodies removed; scanners registered with the DB; registered protocols resolved on demand; `ScanStatus.Reason`, `cancel`; failed/cancelled jobs carry a reason; SMB path join `filepath`->`path` |
| `catalog-api/internal/services/generic_scanner.go` | PA-03: `GenericScanner` (BFS, ReadOnly + HostBudget + Retrying, bounds, `ScanError` kinds, `ChangeToken`, `MemoryTokenStore`, pluggable-protocol scanner) |
| `catalog-api/internal/handlers/stream_handler.go`, `catalog-api/handlers/comic_pages_handler.go`, `catalog-api/handlers/scan_handler.go` | the two duplicate settings mappings now delegate; scan status JSON gains `reason` |
| `catalog-api/filesystem/webdav_propfind.go`, `webdav_client.go` | namespace-aware PROPFIND parser (the old string split listed an EMPTY root on the real Apache fixture) and the collection trailing slash |
| `catalog-api/tests/realfs/` | 13 real-service tests in round 1 (19 in round 2; build tag `realfs`, fail instead of skip) against the pure-ftpd and WebDAV containers |
| unit tests | `settings_test.go`, `webdav_propfind_test.go`, `settings_contract_test.go`, `generic_scanner_test.go` (+`_fake_`, `_job_`, `_regression_`) |
| `catalog-api/go.mod`, `go.sum` | compile-forced bumps, see Deviations |
| `docs/scripts/generic_scanner.md`, `docs/scripts/settings_contract.md` | companion guides with revision headers |

## Evidence files

| File | Content |
|---|---|
| `red_pa01_settings_contract.log` | RED on the old code: the settings contract fails for ftp (path not forwarded), nfs (`export_path` vs `path`), webdav (path not forwarded); local and smb pass |
| `red_pa03_stub_scanners.log` | RED on the old code: the FTP, NFS and WebDAV scanners find 0 of 5 entries of a populated tree |
| `realfs_run1.log` | RED on the real WebDAV fixture before the parser fix: the scan listed an empty root (`empty_root`) and a sub-path answered status 200 (the 301 replayed as GET); FTP already green. The honest empty-root rule exposed a client defect the old stub would have hidden |
| `green1_*.log`, `green2_*.log`, `green4_*.log` | first GREEN runs of the PA-01 / PA-03 / parser tests |
| `green3_race_x3.log`, `green5_race_x3.log` | `-race -count=3` of every new test plus the WebDAV ListDirectory tests (final state is `green5`) |
| `realfs_run2.log`, `realfs_run3.log`, `realfs_run4_race.log` | 11 real tests GREEN (three runs, the fourth with `-race`) before the empty-directory tests were added |
| `realfs_run5.log`, `realfs_run6.log`, `realfs_run7_race.log` | 13 real tests GREEN x3 (the third with `-race`). [R2] CORRECTION (review T3): NOT "on the final code" - `generic_scanner.go` and its test were edited after these runs (22:58 against 22:44 / 22:52 / 22:57) and the logs carry no source fingerprint; superseded by `fix-r2-realfs_green_*` |
| `pkgs_full1.log`, `pkgs_full2.log` | full `internal/services`, `internal/handlers`, `handlers` (and `filesystem` in run 2) GREEN, no regression |
| `build_vet.log` | `go build` of the main package, `go vet` of the touched packages, `./tests/...` and `-tags realfs`. [R2] The file is 9 bytes (`vet_rc=0`) with no command or package list (review T3); superseded by `fix-r2-vet_all.log` |
| `mutation_results_final.txt` (+ raw `mutation_results_*.txt`, `realfs_mut_R*.log`) | 28 mutants (22 unit, 6 on the real stack) all KILLED in the final state; negative control (no change) SURVIVES (tests pass). [R2] This holds ONLY for the author's own mutants: the reviewer's 37 independent mutants had 21 survivors (review T2), and the harness mutated the shared tree and counted a missing exit code as KILLED (T4). Superseded by `fix-r2-mutation_results*.tsv` |
| `mutate_unit.py`, `mutate_realfs.py`, `run_unit.sh`, `run_realfs.sh` | the harnesses used (apply, run, restore with a sha256 check) |
| `up.log`, `down.log` | the stack of the real run (`wp12scan`, services ftp and webdav; ports only, no credential) and its teardown |

## Numbers

- PA-01 RED: 3 of 5 protocols fail the contract (`red_pa01_settings_contract.log`); GREEN after the change; `export_path` is now rejected by the factory with a hint.
- PA-03 RED: 3 of 3 scanners find 0 of 5; GREEN: 5 of 5. Real FTP and WebDAV: exactly the 24 files of the seeded corpus manifest plus 23 directories, in 3 GREEN runs (1 under `-race`); the sub-path `/movies` reaches the client on both; the empty directory `/deep/empty-leaf` ends `failed` (`empty_root`) on both; a missing path ends `failed` (`root_unreadable`); a whole job through `QueueScan` ends `completed` with the corpus and `failed` with a reason for a missing root.
- Unit: 14 + 6 + 1 + 32 + 8 + 1 test functions (new); `-race -count=3` GREEN.
- Mutants: 22 unit + 6 real = 28 KILLED, negative control survives. Four mutants first survived or did not compile (M04, M10 build-broken; M15, M20 survived): the two weak tests were strengthened, the build-broken mutants re-made, all re-run KILLED. R5 survived the real stack until an empty-directory real test existed.

## Deviations (stated, not hidden)

1. `catalog-api/go.mod` / `go.sum` changed: the concurrently edited `submodules/filesystem/go.mod` (ftp v0.2.4, testify 1.12.1, crypto 0.54.0, pkg/sftp) made `catalog-api` unbuildable ("updates to go.mod needed"). The change is exactly what `go mod tidy` produces from the submodule's current go.mod (run with `-modfile` on a scratch copy because the source mount is read-only; the final state was re-checked tidy-clean). If the submodule's go.mod moves again, `catalog-api/go.mod` must be re-tidied by whoever lands the submodule.
2. The corpus has 24 files and 23 directories (47 entries), not the 40 files and 5 directories the task text of T361 quotes; the oracle used is the generated `/manifest.sha256`.
3. `catalog-api` does NOT import `submodules/filesystem/pkg/{sftp,nfs3,ftp}`: pluggable protocols are wired through `filesystem.RegisterProtocol` and a ProtocolSpec per protocol (documented in `docs/scripts/settings_contract.md`); importing them needs `github.com/pkg/sftp` in `catalog-api/go.mod`.
4. Existing tests changed because their expectation encoded the bug or the stub: `export_path`->`path` (2 tests), the FTP/WebDAV `SupportsIncrementalScan` (now true) and the NFS `ChecksumCalculation` (now false, the stub claimed true while doing nothing), and `TestSupportedProtocols_AllCreateable` (it passed a kitchen-sink settings map to every protocol, which the strict contract rightly rejects; now per-protocol keys).

## UNCONFIRMED / not done

- UNCONFIRMED: behaviour against the 7 NAS hosts (no NAS contact was made; scope was the container fixtures). [R2] The relative FTP `path` doubt was CONFIRMED by the review (R18) and fixed in round 2.
- Not done: a database-backed change-token store and deleted-file detection; include/exclude patterns; `TokenSource` (etag/inode) is an interface no client implements yet; FTPS has no client (PA-04); sftp/nfs3 registration is not wired in `catalog-api` (deviation 3); `docs/scripts/README.md` rows (file not touched, as instructed).
- One test-infra run was lost to an out-of-memory kill of `go build` (another worker's load); it was repeated, the failed attempt left no result.
- `tests/integration/protocol_rename_test.go` still passes `export_path` in `GetTestConfig()`; that map feeds the rename tracker, not the factory, so it was left alone.
