# PGFU1 (PROVISIONAL): PGTX1 + PGEND1 review follow-ups

- F1 tx_site_guard: AST scanner now follows aliased `database/sql` imports, local unwrap (`raw := db.DB; raw.BeginTx`), names declared `*sql.DB` (params, vars, fields), `.Tx.Tx` embedded unwrap. Still an AST walk, NOT go/types. RED: red_f1_old_scanner.txt (old scanner misses 8 planted cases).
- F2 `database.Transaction` embeds `*Tx` (all statement methods rewrite, once). RED on real PostgreSQL 15: red_f2_f3.txt (`ExecContext` with `?` -> `pq: syntax error`).
- F3 explicit `Rewrite` at tx sites removed (user_repository, login_throttle, playback_session_repository; refusedByGuard/currentlyAdmin lost their `rewrite` flag). RewriteInsertOrIgnore left first-occurrence-only (statement-level suffix; not safe to extend), documented + pinned (TestRewriteInsertOrIgnoreIsSingleStatementOnly).
- F4 SafeRollback/SafeCommit take `TxFinisher` (typed nil treated as nil).
- N1 stale comments fixed (connection.go Rewrite, tx_helpers.go, playback_session_repository.go, INTEGER-affinity). N2 (sql.Conn in encryption.go) not touched.
- PGEND1-R-m1: two-user/two-item test (SQLite + PG); m4: End on a PUT-created media_progress row (SQLite + PG; the PUT SQL is replicated, the handler is not called, repository cannot import handlers); n2: pgend1 SUMMARY wording fixed.
- Mutants: mutants.txt (R6, R6b, G1-G4, F2, F2b, F4: 9 killed, 0 survivors). Green runs: green_*.txt, vet.txt.
- Not done / UNCONFIRMED: no independent review; full `repository` and `handlers` suites not run; client-visible behaviour unchanged.
