# CAT-LOGIN-LOCK-2 round 2 evidence (PROVISIONAL, author-produced)

Fixes for the round-1 NO-GO of the independent review (F1 CRITICAL: in-flight cap not enforced on PostgreSQL).

- `ll2-pg-r2.json`: `scripts/containers/disk_headroom.sh --need 600000000` REFUSED (`disk_below_headroom`: 4.49 GB free in the podman
  graphroot against a 12.17 GB floor). The gate is documented for image pulls and builds. The real-PostgreSQL run below pulls and
  builds nothing (both images were already local, `--rm` containers of about 60 MB each, removed by the test). The gate was NOT
  bypassed or edited; running local images under a refused gate is a scope judgement of the author, disclosed here for the reviewer.
  `df_before.txt` / `df_after.txt` show free space around the run.
- `pg_both_collations.txt`: `TestLoginThrottlePostgres_InflightCapHolds` against real PostgreSQL 15 (postgres:15-alpine and postgres:15,
  Debian), 12 rounds of 40 concurrent Begins each plus a serial control: every round allowed exactly 4 (cap 4). Note: both images
  report `datcollate=en_US.utf8`; the alpine (musl) image still sorts bytewise, so the collation-dependent range comparison
  failed only on the Debian image (mutant N2 below).
- `new_mutants.txt`: mutants of the new mechanism (N1 lock removed: killed on both images; N2/N3 range comparison: killed on the
  Debian image only; N4 off-by-one, N5 expired tokens counted, N6 token not inserted: killed on SQLite). `r4_mutant.txt`: reviewer R4.
- `suites.txt`, `vet.txt`: regression suites and `go vet`.
