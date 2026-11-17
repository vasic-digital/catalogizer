# CAT-PG-SESSION-END-1 (PROVISIONAL)

Root cause (measured on real PostgreSQL 15, rootless podman): the first failing statement is `PlaybackSessionRepository.Start`,
whose INSERT carries the integer literal `0` for the BOOLEAN `completed` column (`pq: column "completed" is of type boolean but
expression is of type integer`); the reviewer's probe hit it via Start, not End. Mutant M2 (End binding an integer 0/1) PASSES on
pq (the server infers boolean from the parameter), so the End integer bind is an equivalent mutant; End is now bound as a Go bool anyway.
Further defects fixed in the same path: End's transaction statements bypassed the DB wrapper (raw `?` on PostgreSQL, then rewritten
via DB.Rewrite; since CAT-PG-TX-SWEEP-1 the *database.Tx rewrites them itself). Get and ListHistory are plain DB reads (`db.QueryRowContext` / `db.QueryContext`), not transaction statements, and always went through the wrapper; `completed` scans into bool; End is a compare-and-set on `ended_at IS NULL` (idempotent on retry/race);
media_progress increments are atomic in SQL (no lost update on READ COMMITTED). First-ever concurrent End for a user/item can still
collide on the primary key (documented limit, no portable atomic upsert).
Dialect literal rewrite list extended with is_completed, completed, favorite, is_primary, can_read, can_write.
Files: red_before_fix.txt (RED), green_*.txt, mutants.txt (M1,M3,M4,M5,M6 killed; M2 equivalent).
Sweep: no other int-bound boolean parameter found in repository/services/handlers; literal `col = 0/1` on bool columns not in the
rewrite list were is_completed/favorite/completed (fixed). Not swept / follow-up: ~70 other `tx.Exec*` sites (playlist_service,
user_repository, universal_scanner, rename_trackers) bypass the wrapper rewrite; not audited here.
