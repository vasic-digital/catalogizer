# CAT-ALTSVC-1 fix evidence (PROVISIONAL)
- RED.txt: pre-fix main_router.go/main.go + new tests: routed 200 over TLS has Alt-Svc "" (expected h3=":28443"; ma=86400); the two golden-FALSE tests pass.
- green_v.txt: `go test -run 'AltSvc|AuthSurface|OpenAPI|Contract'` root package, -v: 113 PASS, 0 FAIL, 3 SKIP (opt-in runtime-mutant tests).
- mutants_runtime_v.txt: CHPW_RUNTIME_MUTANTS=1 runtime mutants AS1 (abort), AS2 (header dropped), AS4 (header on plain HTTP), AS5 (ma changed), AS6 (advertised while h3 disabled), AS7 (middleware removed from buildRouter), HK1: all killed (7/7).
- Source-AST mutant AS3 (Use removed from buildRouter) and re-anchored F1b2/F1b3/F1b5 run inside TestAuthSurfaceGuard_Mutants (green_v.txt).
- Host-fixture only: real buildRouter, httptest, r.TLS set by hand; no real HTTP/3 client.
