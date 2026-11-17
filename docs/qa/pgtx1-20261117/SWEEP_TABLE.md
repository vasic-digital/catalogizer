# CAT-PG-TX-SWEEP-1 sweep table

Instrument: go/ast scanner (`scan/main.go`, kept beside this table), run over every non-test .go file under catalog-api on the pre-fix tree (HEAD 81cccc88f).
Tx value = identifier assigned from a `Begin`/`BeginTx` call, or a parameter typed `*sql.Tx`/`Tx`/`Transaction`/`TxExecQuerier`. Statement = Exec/ExecContext/Query/QueryContext/QueryRow/QueryRowContext/Prepare/PrepareContext.
Control needles (fixture): 3 planted tx sites found, 2 planted non-tx look-alikes (`db.Exec`, `other.Exec`) not found. Cross-check: independent grep for `tx.<stmt>(` = 75 = 70 scanner sites + 3 (`database.Transaction` Exec/Query/QueryRow, rewriting) + 2 (`txQ` wrapper, rewriting). Pre-fix and post-fix scans both give 70 sites.

Classes: R = rewritten correctly before the fix (explicit `Rewrite`/`rewriteQuery`); B = BYPASS before the fix (raw `?` / dialect syntax reached PostgreSQL); S = sqlite-only SQL, dialect-guarded (never reaches PostgreSQL); N/A = separate SQLite-only media store. After the fix every B and R site goes through `*database.Tx`, which rewrites.

| # | site | function | method | pre-fix class | flags / note |
|---|---|---|---|---|---|
| 1 | database/tx_helpers.go:18 | DB.TxInsertReturningID | QueryRowContext | R | TxInsertReturningID calls db.rewriteQuery itself |
| 2 | database/tx_helpers.go:26 | DB.TxInsertReturningID | ExecContext | R | TxInsertReturningID calls db.rewriteQuery itself |
| 3 | internal/media/database/database.go:110 | MediaDatabase.createSchemaFromString | Exec | N/A | raw *sql.DB on the SQLite/SQLCipher media store; no PostgreSQL mode; allowlisted in the guard |
| 4 | internal/media/database/database.go:165 | MediaDatabase.Backup | Exec | N/A | raw *sql.DB on the SQLite/SQLCipher media store; no PostgreSQL mode; allowlisted in the guard |
| 5 | internal/media/database/database.go:173 | MediaDatabase.Backup | Exec | N/A | raw *sql.DB on the SQLite/SQLCipher media store; no PostgreSQL mode; allowlisted in the guard |
| 6 | internal/media/database/database.go:179 | MediaDatabase.Backup | Exec | N/A | raw *sql.DB on the SQLite/SQLCipher media store; no PostgreSQL mode; allowlisted in the guard |
| 7 | internal/services/playlist_service.go:323 | PlaylistService.AddToPlaylist | QueryRowContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 8 | internal/services/playlist_service.go:332 | PlaylistService.AddToPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 9 | internal/services/playlist_service.go:346 | PlaylistService.AddToPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 10 | internal/services/playlist_service.go:384 | PlaylistService.RemoveFromPlaylist | QueryRowContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 11 | internal/services/playlist_service.go:391 | PlaylistService.RemoveFromPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 12 | internal/services/playlist_service.go:398 | PlaylistService.RemoveFromPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 13 | internal/services/playlist_service.go:433 | PlaylistService.ReorderPlaylist | QueryRowContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 14 | internal/services/playlist_service.go:446 | PlaylistService.ReorderPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 15 | internal/services/playlist_service.go:456 | PlaylistService.ReorderPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 16 | internal/services/playlist_service.go:463 | PlaylistService.ReorderPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 17 | internal/services/playlist_service.go:475 | PlaylistService.ReorderPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 18 | internal/services/playlist_service.go:556 | PlaylistService.RefreshSmartPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 19 | internal/services/playlist_service.go:561 | PlaylistService.RefreshSmartPlaylist | QueryContext | B | query built in a variable; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 20 | internal/services/playlist_service.go:575 | PlaylistService.RefreshSmartPlaylist | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 21 | internal/services/playlist_service.go:640 | PlaylistService.AddToQueue | QueryRowContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 22 | internal/services/playlist_service.go:648 | PlaylistService.AddToQueue | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 23 | internal/services/playlist_service.go:712 | PlaylistService.updatePlaylistStats | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 24 | internal/services/playlist_service.go:723 | PlaylistService.updatePlaylistTags | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 25 | internal/services/playlist_service.go:729 | PlaylistService.updatePlaylistTags | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 26 | internal/services/playlist_service.go:747 | PlaylistService.updatePlaylistCollaborators | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 27 | internal/services/playlist_service.go:753 | PlaylistService.updatePlaylistCollaborators | ExecContext | B | raw `?` on PostgreSQL; also: SQL names columns/tables absent from the migrations (see SUMMARY limits) |
| 28 | internal/services/rename_tracker.go:207 | RenameTracker.moveFile | QueryRow | B | raw `?` on PostgreSQL |
| 29 | internal/services/rename_tracker.go:220 | RenameTracker.moveFile | Exec | B | raw `?` on PostgreSQL |
| 30 | internal/services/rename_tracker.go:239 | RenameTracker.moveDirectory | Query | B | raw `?` on PostgreSQL |
| 31 | internal/services/rename_tracker.go:280 | RenameTracker.moveDirectory | QueryRow | B | raw `?` on PostgreSQL |
| 32 | internal/services/rename_tracker.go:292 | RenameTracker.moveDirectory | Exec | B | raw `?` on PostgreSQL |
| 33 | internal/services/rename_tracker.go:323 | RenameTracker.markRenameEventStatus | Exec | B | raw `?` on PostgreSQL |
| 34 | internal/services/universal_rename_tracker.go:312 | UniversalRenameTracker.moveFile | QueryRow | B | raw `?` on PostgreSQL |
| 35 | internal/services/universal_rename_tracker.go:324 | UniversalRenameTracker.moveFile | Exec | B | raw `?` on PostgreSQL |
| 36 | internal/services/universal_rename_tracker.go:342 | UniversalRenameTracker.moveDirectory | Query | B | raw `?` on PostgreSQL |
| 37 | internal/services/universal_rename_tracker.go:397 | UniversalRenameTracker.markRenameEventStatus | Exec | B | raw `?` on PostgreSQL |
| 38 | internal/services/universal_scanner.go:1017 | insertFileRecord | ExecContext | S | PRAGMA; inside `if db.Dialect().IsSQLite()`; PRAGMA never runs on PostgreSQL |
| 39 | internal/services/universal_scanner.go:1024 | insertFileRecord | ExecContext | S | bool-int-literal; inside `if db.Dialect().IsSQLite()` branch (PostgreSQL has its own non-tx path) |
| 40 | internal/services/universal_scanner.go:1033 | insertFileRecord | ExecContext | S | inside `if db.Dialect().IsSQLite()` branch (PostgreSQL has its own non-tx path) |
| 41 | internal/services/universal_scanner.go:1045 | insertFileRecord | ExecContext | S | PRAGMA; inside `if db.Dialect().IsSQLite()`; PRAGMA never runs on PostgreSQL |
| 42 | internal/services/universal_scanner.go:822 | ensureDirectoryPathExists | ExecContext | S | PRAGMA; inside `if db.Dialect().IsSQLite()`; PRAGMA never runs on PostgreSQL |
| 43 | internal/services/universal_scanner.go:824 | ensureDirectoryPathExists | ExecContext | S | INSERT OR IGNORE; inside `if db.Dialect().IsSQLite()` branch (PostgreSQL has its own non-tx path) |
| 44 | internal/services/universal_scanner.go:834 | ensureDirectoryPathExists | QueryRowContext | S | inside `if db.Dialect().IsSQLite()` branch (PostgreSQL has its own non-tx path) |
| 45 | internal/services/universal_scanner.go:842 | ensureDirectoryPathExists | ExecContext | S | PRAGMA; inside `if db.Dialect().IsSQLite()`; PRAGMA never runs on PostgreSQL |
| 46 | repository/login_throttle.go:379 | LoginThrottle.lockSlots | ExecContext | R | INSERT OR IGNORE; explicit db.Rewrite; now redundant but idempotent |
| 47 | repository/login_throttle.go:382 | LoginThrottle.lockSlots | ExecContext | R | explicit db.Rewrite; now redundant but idempotent |
| 48 | repository/login_throttle.go:413 | LoginThrottle.takeToken | QueryRowContext | R | explicit db.Rewrite; now redundant but idempotent |
| 49 | repository/login_throttle.go:420 | LoginThrottle.takeToken | ExecContext | R | explicit db.Rewrite; now redundant but idempotent |
| 50 | repository/playback_session_repository.go:150 | PlaybackSessionRepository.End | ExecContext | B | raw `?` on PostgreSQL |
| 51 | repository/playback_session_repository.go:164 | PlaybackSessionRepository.End | QueryRowContext | B | raw `?` on PostgreSQL |
| 52 | repository/playback_session_repository.go:180 | PlaybackSessionRepository.End | QueryRowContext | B | raw `?` on PostgreSQL |
| 53 | repository/playback_session_repository.go:198 | PlaybackSessionRepository.End | ExecContext | B | raw `?` on PostgreSQL |
| 54 | repository/playback_session_repository.go:210 | PlaybackSessionRepository.End | ExecContext | B | raw `?` on PostgreSQL |
| 55 | repository/user_repository.go:1100 | UserRepository.UpdateRole | Exec | R | argument is r.db.Rewrite(q) or a dialect-guarded literal (advisory lock, PostgreSQL only) |
| 56 | repository/user_repository.go:1103 | UserRepository.UpdateRole | Exec | B | raw `?` on PostgreSQL |
| 57 | repository/user_repository.go:1130 | UserRepository.UpdateRole | Exec | R | argument is r.db.Rewrite(q) or a dialect-guarded literal (advisory lock, PostgreSQL only) |
| 58 | repository/user_repository.go:1168 | UserRepository.DeleteRole | QueryRow | B | raw `?` on PostgreSQL |
| 59 | repository/user_repository.go:1177 | UserRepository.DeleteRole | Exec | B | bool-int-literal; raw `?` on PostgreSQL |
| 60 | repository/user_repository.go:260 | UserRepository.UpdateGuarded | Exec | R | argument is r.db.Rewrite(q) or a dialect-guarded literal (advisory lock, PostgreSQL only) |
| 61 | repository/user_repository.go:332 | UserRepository.lockAdminWrites | Exec | R | argument is r.db.Rewrite(q) or a dialect-guarded literal (advisory lock, PostgreSQL only) |
| 62 | repository/user_repository.go:335 | UserRepository.lockAdminWrites | Exec | S | else-branch of the PostgreSQL advisory-lock check: literal `id = 0` statement, no placeholder, portable; no rewrite needed |
| 63 | repository/user_repository.go:348 | UserRepository.adminRoleIDsTx | Query | R | explicit db.Rewrite; now redundant but idempotent |
| 64 | repository/user_repository.go:573 | UserRepository.LockAccount | Exec | R | explicit db.Rewrite; now redundant but idempotent |
| 65 | repository/user_repository.go:678 | UserRepository.SetSuspended | Exec | R | explicit db.Rewrite; now redundant but idempotent |
| 66 | repository/user_repository.go:722 | UserRepository.Delete | QueryRow | R | explicit db.Rewrite; now redundant but idempotent |
| 67 | repository/user_repository.go:726 | UserRepository.Delete | Exec | R | explicit db.Rewrite; now redundant but idempotent |
| 68 | repository/user_repository.go:733 | UserRepository.Delete | Exec | R | argument is r.db.Rewrite(q) or a dialect-guarded literal (advisory lock, PostgreSQL only) |
| 69 | repository/user_repository.go:744 | UserRepository.Delete | QueryRow | R | explicit db.Rewrite; now redundant but idempotent |
| 70 | repository/user_repository.go:751 | UserRepository.Delete | Exec | R | argument is r.db.Rewrite(q) or a dialect-guarded literal (advisory lock, PostgreSQL only) |
