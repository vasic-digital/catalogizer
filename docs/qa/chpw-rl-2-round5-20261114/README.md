# CAT-CHPW-RL-2 round 5 (on top of round 4, 56c35110d) - PROVISIONAL

Status: PROVISIONAL (awaiting the independent review of round 5).
Landing note: the branch also carries the four CAT-CHPW-RL-1-FU commits
f091b2cf0..4be1f0570, which are not yet on main.

## R4-1 / R4-2: empty parameter values shift the alignment
Gin routes an EMPTY parameter value, so `/:p/api/v1/auth/me` answers
`//api/v1/auth/me` and `/api/:p/v1/auth/me` answers `/api//v1/auth/me`.
`patternReachesAuthPaths` (production) could not see this; the round-4 corpus test
had found it and the oracle was edited to hide it. That edit is reverted.
- Production: `patternReachesAuthPaths` is now a small matcher where a whole-parameter
  segment may consume zero request segments or one; catch-all returns true.
  It feeds both the verifier and the AST scan, so both are fixed.
- Oracle `inAuthSurface`: the `strings.Contains(c, "//")` skip is removed. Its
  definition of the auth surface is now stated: a request path whose CLEANED form
  (empty segments dropped, as path.Clean, RedirectFixedPath and normalising proxies
  do) is under /api/v1/auth. The oracle still asks a real gin engine.
- Measured with a probe on the real buildRouter: GET `//api/v1/auth/me` answers 404
  from the real engine, not 401 as the review stated (UNCONFIRMED how the review got
  401); the foreign route answered 299 there, and `/api//v1/auth/me` likewise for the
  middle-position form. The refusal is therefore conservative: it protects against a
  normalising front end or client that maps the doubled slash onto the auth route.
- The round-4 README claim ("oracle defect fixed in the oracle") is corrected there.

## Mutants (all killed, message-specific assertions)
Verifier (real engines, error must match one of the verifier's own messages):
X1 leading param, X2 Group(/:p).Any(.../*rest), X3 param between api/v1, X4 between
v1/auth, X5 two leading params, X6 param, api, param, X7 leading catch-all (gin
refuses at registration), X8 param then catch-all, X9 param at the auth position,
X10 param before v1. Pattern table test TestPatternReachesAuthPaths_EmptyParamShiftClass
covers the shapes gin refuses on the full engine.
AST (finding must contain the scan's own text): X1, X2, X3 (in main.go), X4, X5.
RB/RB1/RB2/RC/RC2/RD/RE now assert their specific message (R4-5).
Runtime overlay: R5a/R5b/R5c (remove the empty-value branch; killed by the corpus,
the class table and the verifier case), R5d (X1 route in buildRouter).

## Results (measured at the commit of this round)
- go vet: clean.
- Focused suites: 149 PASS lines (-v) before the overlay mutants were added.
- CHPW_RUNTIME_MUTANTS=1 TestBuildRouterAuthSurface_RuntimeMutants (32),
  TestAltSvcAndHandlerKey_RuntimeMutants (3), TestVerifier_RuntimeMutants (8): all PASS.
  Trimpath was NOT re-run this round (budget); last run in round 3.
- Full go test -p 1 -v: 331 PASS lines (subtests included), 5 SKIP, 1 FAIL
  (the FAIL is the pre-existing TestSSRF_RealBinary_SyncEndpointURL if listed above).

## Not done
F-D (trusted proxies / ForwardedByClientIP / RemoteIPHeaders / TrustedPlatform changed
after buildRouter pass the verifier) stays a disclosure; the coordinator registers it.
