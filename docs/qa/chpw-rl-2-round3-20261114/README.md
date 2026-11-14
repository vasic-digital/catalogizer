# CAT-CHPW-RL-2 round 3 (structural round) - evidence, 2026-11-14

Scope: close the class "anything that can answer an /api/v1/auth path without the
intended limiter chain" (round-2 review findings F1a, F1b, F2, F3, F4). Unit-class
and source-mutant evidence; no live deployment claimed. Status: PROVISIONAL.

## Design (class-wide, not per instance)
- `patternReachesAuthPaths` (main_router.go): a property of the route PATTERN alone.
  Pattern is cleaned like gin cleans it, segments are aligned with api / v1 / auth
  (case-insensitive); literal, `prefix:param` (gin params run to the next `/`) and
  `*catchall` segments are handled; every pattern whose first three segments are
  compatible, or that has an earlier catch-all, counts. Method-independent.
  `verifyAuthRouteTable` requires that set to equal `authRouteTable`.
- `verifyAuthRouteTable` additionally requires (a) the path-matching flags pinned to the values gin.New() sets (round 4: this now includes UseEscapedPath; see chpw-rl-2-round4-20261114), (b) the global handler chain to equal the pinned `wantGlobalChain`
  (optionally followed by `altSvcMiddleware`), (c) NoRoute/NoMethod chains equal to it.
  main() calls it twice: after the routes and again after its one late `Use`.
- main()'s late `Use` is now the named `altSvcMiddleware`; the AST scan allows in
  main() exactly one engine Use whose argument is that identifier.
- `handlerKey` no longer needs a "/catalog-api/" directory: the module root is
  derived from the file of `buildRouter`; dependencies are cut at `<module>@<version>`.
- Independent oracle in the test: `inAuthSurface` instantiates a pattern with witness
  values and asks a real gin engine whether it answers the candidate auth path.

## RED before the fix (observed, this session)
Verifier-level mutants that SURVIVED (verifyAuthRouteTable returned nil): F1a1-F1a6,
F1b, M1, M2, M5, M8, M9-M14 (17 of 28 mutants; M3/M4/M6/M7 were already refused by
gin or the old rules). AST mutants that SURVIVED the scan: F1a1-F1a6, F1a1b, F1b,
M1-M5, M8 (14), plus the helper-scan shapes partial_param_first/partial_param_then/
any_capture. Logs of the RED runs were kept in the scratch dir (red1.log, red2.log).

## GREEN after the fix
- `go test -run 'TestVerifyAuthRouteTable|TestAuthSurfaceGuard|TestRouteHelpers|
  TestMainGoMountsAdmin|TestBuildRouterAuthSurface|TestMainPackageAuthSurfaceClosed|
  TestAltSvc|TestHandlerKey'`: ok, 119 PASS lines (-v).
- Opt-in `CHPW_RUNTIME_MUTANTS=1 go test -run 'RuntimeMutants|Trimpath'`: 32 source
  mutants compiled through `go test -overlay`, all KILLED (incl. 12 new class mutants
  F1a1-6, F1b, M1-M5), plus `TestBuildRouterAuthSurface_Trimpath` PASS (pins hold under
  -trimpath), plus AS1/AS2/HK1 (altSvc abort, altSvc header dropped, handlerKey
  depending on the directory name under -trimpath) all KILLED. One earlier mutant (AS1
  "drop c.Next()") SURVIVED because Next is redundant in gin's dispatch loop; it was
  replaced by behaviour-changing mutants.
- Full `go test -p 1 -v .` of catalog-api: 143 PASS, 4 SKIP, 2 FAIL (see below).

## Failures in the full suite (not caused by this change)
- TestSSRF_RealBinary_SyncEndpointURL: FAILS on the round-2 tip too (verified with the
  change stashed) and on base 4be1f0570 per the review; unrelated. Track separately.
- TestD4_FullTextSearchOverExtractedDocuments_RealBinary: failed once in the full run
  ("PDF text must be searchable") under host load ~45, PASSED 2/2 when re-run alone
  with this change. Cause UNCONFIRMED (load-sensitive PDF extraction suspected).

## Not claimed
No live /api/v1/auth request was driven. Gin matching was exercised only through the
test oracle (real gin engines) and the startup verifier over the real production engine
(every *_RealBinary test starts the real binary, which now runs the stricter verifier).
Pre-existing, unchanged: main() installs the Alt-Svc Use while the HTTPS goroutine
already serves (a data race on the handler chain); not touched here.
