# ALTSVC4 (PROVISIONAL, 2026-11-17): follow-ups m1..m5, n1..n3 of the ALTSVC3 review

Changes: `catalog-api/main_h3.go` (seam `h3Serve`, UDP socket closed when Serve returns; n1), new `catalog-api/main_altsvc4_test.go`.
main.go, main_router.go, the pre-gate allow-list/pin files and the existing auth-surface guards are untouched (full `AltSvc|H3|AuthSurface` run: ok, `final_run.txt`).

Mutation matrix (`mutants.sh`, outputs `mutants_out_round1.txt` then `mutants_out.txt`, raw in `raw/`). "old" = pre-existing tests, "new" = ALTSVC4 tests.
- M1 (Store(false) removed): survives old, killed by new (ServeFailureClearsFlag).
- M5 (TCP bind failure aborts the stack): survived old AND survived my first new test (round 1: the test skipped INCONCLUSIVE when no h3 server, masking the defect); fixed by skipping only if the UDP port is really occupied; round 2 killed.
- N1 (UDP socket not closed): survives old, killed by new (UDPSocketReleasedAfterClose).
- M6b, M7, M2 as I formed them (handler swapped for NotFoundHandler; Store(true) moved before ListenPacket; startH3Stack result discarded) were already caught by the OLD suite, incidentally (auth-surface text guards, UDPBindFailure test), so they are not "survivors" in my reconstruction; the reviewer's exact mutants were not available (UNCONFIRMED they are identical). They are also killed by new tests: real HTTP/3 request (M6b), AST pin (M7, M2); pin self-test `TestAltSvcPinsKillMutants` has 14 textual mutants (all reported).
- Not run as a separate RED commit: tests were written after the seam was added, RED = mutant survival above (reproduce-first via mutation).

Limits / n3 note: the real `main()` startup is NOT run (binds fixed 28443); the stack tests call the same `startH3Stack` helper on ephemeral ports. The HTTP/3 request test SKIPs INCONCLUSIVE on QUIC idle/timeout (UDP unavailable) and FAILS on any other error or wrong answer. A port taken between probe and bind => retry then SKIP INCONCLUSIVE (n2). Host fixture only, no live PostgreSQL/device relevance.
