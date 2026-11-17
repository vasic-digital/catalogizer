# CAT-PG-TX-SWEEP-1 (PROVISIONAL)

Root cause: `DB.Begin/BeginTx` were promoted from the embedded `*sql.DB` and returned a raw `*sql.Tx`; statements run on it bypassed the
wrapper's dialect rewrite (placeholders `?` -> `$n`, INSERT OR IGNORE/REPLACE, boolean literals) so PostgreSQL received raw `?`.

Fix: `database.Tx` (catalog-api/database/tx.go) embeds `*sql.Tx` and shadows Exec/ExecContext/Query/QueryContext/QueryRow/QueryRowContext/
Prepare/PrepareContext with the rewrite; `DB.Begin/BeginTx` return it. `Transaction` (TxContext) keeps its own rewriting and now wraps `sqlTx.SQLTx()`.
`TxInsertReturningID` takes a small interface (accepts `*Tx` or raw `*sql.Tx`). Callers' `*sql.Tx` parameters became `*database.Tx`
(rename trackers, playlist_service, user_repository, login_throttle; `txQ` no longer rewrites itself). Existing explicit `Rewrite` calls on tx sites
are left (redundant, proven idempotent by TestRewriteQueryIsIdempotentOnPostgres).

Evidence (real PostgreSQL 15, rootless podman via internal/testpg):
- red_before_fix.txt: `pq: syntax error at or near ","` for `Exec` with `?` on a tx (both Begin and BeginTx).
- green_tx_pg.txt: same test GREEN (Exec, ExecContext, Query, QueryContext, QueryRow, QueryRowContext, Prepare, PrepareContext, INSERT OR IGNORE, integer boolean literal).
- mutants.txt: 7 mutants killed (3 statement methods without rewrite, wrapper without db, blind guard walk, scanner ignoring the type, planted raw `*sql.Tx` param).
- green_db_repo.txt: `database` ok (172.9s); `repository` as a whole hit the 10 m go test timeout under host load 47-53 (no failure line; the test in flight at the timeout had run 10 s); `internal/media/database` ok.
- green_repository_pg.txt: the repository PG/user/login/playback subset, run with -timeout 40m: ok (229s), 168 PASS, 2 honest SKIP (need CATALOGIZER_PG_TEST_* env, unrelated). Includes the PostgreSQL PGLAX1 user-repository and LoginThrottle tests, whose tx sites now go through Tx.
- green_services.txt: internal/services Rename|Playlist|Scanner subset ok (SQLite).
- `go vet ./...` over all of catalog-api (tests included) clean.

Guard: database/tx_site_guard_test.go fails if non-test code outside `database/` (allowlist: internal/media/database, the SQLite/SQLCipher-only media store) names `sql.Tx`, calls `SQLTx()`, or calls `Begin/BeginTx` on an unwrapped `.DB`; it has a control needle (>200 files walked) and a planted-site scanner test.

Sweep table: SWEEP_TABLE.md (70 sites: 39 B bypass before the fix, 18 R already rewriting, 9 S sqlite-only or portable inside a dialect guard, 4 N/A media store). Scanner source: scan/main.go.txt; raw rows sites_pre_fix.tsv / sites_post_fix.tsv.
sqlite-only SQL in tx statements: only `PRAGMA foreign_keys` and `INSERT OR IGNORE` in universal_scanner, both inside `if db.Dialect().IsSQLite()`. No datetime()/AUTOINCREMENT/GROUP_CONCAT/INSERT OR REPLACE found in tx statements.

Limits (UNCONFIRMED where stated):
- PlaylistService (21 of the sites) names columns/tables the migrations do not create (playlist_items.media_item_id/added_by/custom_title, queue_items; v13 has entity_id/entity_type). That is schema drift on every dialect, independent of this fix; whether the service is wired to a route is UNCONFIRMED. Not driven live on PostgreSQL.
- Rename trackers and universal_scanner tx paths were not driven live on PostgreSQL (the scanner tx path is SQLite-only by construction; rename trackers are portable SQL now rewritten).
- Only catalog-api was swept; submodules and other modules were not.
- The scanner is name/shape based (no go/types); a tx obtained by a pattern other than Begin/BeginTx/typed param would be missed, mitigated by the grep cross-check (75 = 70 + 5 accounted).
- Full `repository` package run exceeded the default timeout under load; subsets passed.
- Not independently reviewed (§11.4.142 owed); status PROVISIONAL, verification job: full repository + services suites on a quiet host.

Addendum (PGFU1, 2026-11-17): the line "`Transaction` (TxContext) keeps its own rewriting" overstated: it embedded the raw `*sql.Tx` and rewrote only Exec/Query/QueryRow,
so ExecContext/QueryContext/QueryRowContext/Prepare/PrepareContext reached PostgreSQL with raw `?` (reproduced RED on real PostgreSQL, docs/qa/pgfu1-20261117/red_f2_f3.txt).
`Transaction` now embeds `*Tx`, so every statement method is the rewriting one and runs exactly once; explicit `Rewrite` calls at tx sites were removed
("left (redundant, proven idempotent)" is superseded: the proof holds only for a single INSERT OR IGNORE per statement).
