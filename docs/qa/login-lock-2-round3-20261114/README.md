# CAT-LOGIN-LOCK-2 round 3 evidence (PROVISIONAL, author-produced)

Round-2 review findings F8-F13. `pg_both_collations.txt`: real PostgreSQL 15 (postgres:15-alpine and postgres:15), nothing
pulled or built (images already local); free disk before/after in `df_before_pg.txt` / `df_after_pg.txt` (3.7 GB then 3.9 GB; the
disk gate is NOT bypassed or approved, see the round-2 README: it refuses at this free space, and these runs are the author's
scope judgement). `mutants.txt`: mutants of the cap lock, the Fail hand-off and the reviewer's RA/RB/RD equivalents.

Final-code runs: `pg_both_collations_final.txt` (+ `df_*_final.txt`), `suites.txt`, `vet.txt`. `mutants_run1.txt` ran before Fail's redundant slots
lock was removed (N1-N3, N5, N6, R4, F8a unaffected in kind); `mutants.txt` is the rerun on the final structure (N4, RA, RB, RD, F8a-PG, F8c).
