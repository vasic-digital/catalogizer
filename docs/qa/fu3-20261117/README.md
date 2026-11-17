# FU3 evidence (PROVISIONAL, host-fixture + real PostgreSQL 15 via internal/testpg rootless podman)

Clone of main d6a7e8058, tests-only plus a godoc line and a comment (no production behaviour change).

## Commands (producing the evidence; `GOMAXPROCS=2 nice -n 10 go test -p 1 -timeout 60m -count=1`, cwd catalog-api)
- WPROG3 mutants: `docs/qa/fu3-20261117/mutant_run.sh before|after` (mutate_wprog3.py is the reviewer's mutant table). Run filter `-run '^TestWatchProgress|^TestRV2' -skip 'GuardedUpdatePostgresRewrite|PG_'`: the text-shape test and the PG test are excluded on purpose, so only behaviour tests can kill a mutant.
- SECQ-1 mutants: `docs/qa/fu3-20261117/secq1_mutants.sh before|after`, `-run TestSECQ1_Live` (request path only).
- Green run: `go test ./handlers -run '^(TestWatchProgress|TestRV2|TestSECQ1)' -v` -> `green_run_v.txt` (49 PASS, 0 FAIL, 0 SKIP; real Postgres executed, `PG executed:` lines present).

## Results
WPROG3 (F1): R1 null-guard-dropped, R2 unit-guard-dropped, R3b value-guard `<=`: BEFORE all three SURVIVE (`ok`); AFTER killed by TestWatchProgress_RV3_N1 + N5 (R1), N2 (R2), N10 (R3b).
SECQ-1 (m1..m3): A create-time netguard check disabled, B redirect check removed, C stored approved_class ignored: BEFORE all three SURVIVE; D plain http.Client killed before and after. AFTER A killed by LoopbackRootRefusedByDefault (now asserts the body), B by RedirectToForbiddenScheme, C by PublicApprovedRootRefusesAllowListedLoopback.
Honest limits: mutants run on one build each, not a full mutation sweep; the redirect target is a forbidden scheme (the dial check also covers address classes); loopback stands in for LAN/public hosts; N4c (F2 nit) not adopted; full handlers suite not run here.
