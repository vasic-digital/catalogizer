# ALTSVC3 (ALTSVC2 review follow-ups F1..F6), PROVISIONAL

Scratch clone of main baceee7e0. Files: catalog-api/main_h3.go (new `startH3Stack`), main.go, main_router.go, main_altsvc3_test.go (new), main_router_test.go (mutants AS8..AS11), main_chpw_authsurface_test.go (pin + R4 mutants, `ce.Pos()` removed).

- green_v.txt: `go test -run 'AltSvc|AuthSurface|RouteChainsLockstep|VerifyAuthRouteTable|DumpIsolation'` -> ok (234.7s), 194 PASS lines, no FAIL; the 3 opt-in runtime-mutant tests are skipped there.
- mutants_runtime_v.txt: `CHPW_RUNTIME_MUTANTS=1 -run TestAltSvcAndHandlerKey_RuntimeMutants` -> ok, every mutant killed incl. AS8 (R5, defer after Next, killed by the real-socket test), AS9 (flag before cert load), AS10 (flag before UDP bind), AS11 (middleware after routes).
- Mutant R4 (Store in main() before the cert load) and R4b (call removed) are killed by TestAuthSurfaceGuard_Mutants (part of the green run).
- RED.txt (F4): AS11 mutant, the real bug signature: the 404 control line passes and the failure is on `routed 200 over TLS must advertise HTTP/3`. RED for F1/F2 is the AS9/AS10 mutants (flag set early = the pre-fix behaviour of setting it unconditionally).
- Ports: new tests use ephemeral ports free on TCP and UDP (never 28443); no such port after a bounded wait -> t.Skip "INCONCLUSIVE". The existing real-main test still binds 28443.
Limits: no HTTP/3 (QUIC) client request; real main() not asserted on 28443; HTTPS and HTTP/3 binds are now independent.
