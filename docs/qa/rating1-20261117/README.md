# CAT-RATING-1 evidence (per-user 0..10 media rating), PROVISIONAL

Scratch clone of main baceee7e0, branch rating1-20261117. Commands: `GOMAXPROCS=2 nice -n 10 go test -p 1`.

- 01_red_tests_before_impl.txt: RED (tests written first; handlers/database packages do not build, the route methods and migration do not exist).
- 02_green_final.txt: new tests GREEN (-v), handlers + database.
- mutant_M1..M6: handler mutants, each run against the new tests: M1 GET unscoped, M2 DELETE unscoped, M3 PUT writes another user's row, M4 range unchecked, M5 fractions accepted, M6 auth skipped. All FAIL (killed). (An earlier M2/M3 attempt in this directory's history was invalid: pattern missed / compile error; the files here are the valid reruns.)
- 03_main_guards.txt: package main `Gate|Route|OpenAPI|Contract|Startup|Authz` WITH the new routes: 3 failures (TestADMD1F1S1R5_RealRouteTable..., R7_PreGate..., S1G6F3_RealChain...), all about `main.altSvcMiddleware`.
- 04_baseline_guards_without_rating_route.txt: the same 3 tests with the routes and table/openapi edits stashed: the SAME 3 fail identically, so they are pre-existing on main baceee7e0 (altsvc2 PROVISIONAL), not caused by this change. The route-table/openapi/code-scan guards that compare the new routes with docs/ROUTE_AUTHZ_TABLE.md passed.
- 05_database_pkg.txt: full database package ok (migration v31, upgrade from v30; v30 upgrade test adjusted to drop v31).
- 06_handlers_media.txt: handlers `Favorite|Media|Rating|AndroidTV` ok.

Not run (UNCONFIRMED): live API contract sweep (build tag live), real-binary route probe, PostgreSQL integration (`migrations_postgres_integration_test.go` column map extended but needs a PostgreSQL), devices/clients.
