# CAT-CHPW-RL-2 round 4 (on top of round 3, 240050a90) - PROVISIONAL

Status: PROVISIONAL (round-4 review verdict NO-GO; superseded by round 5, see chpw-rl-2-round5-20261114).

Item: CAT-CHPW-RL-2. Answers the round-3 review (NO-GO, one important finding).

## F-A (important): UseEscapedPath
RED first: with `UseEscapedPath = true`, `GET /api/v1/auth%2Fevil` answers a request
whose URL.Path is `/api/v1/auth/evil`; the verifier returned nil and AST mutant RA
(`router.UseEscapedPath = true` in main.go) produced 0 findings.

Fix, class-wide:
- `TestGinEngineFieldsClassified` enumerates every field of `gin.Engine` (exported and
  unexported, gin 1.12.0) via reflect. Each is either matching-relevant (8 bool flags
  and the handler chains/trees, all pinned) or irrelevant with a written reason. A field
  gin adds later is unclassified and fails the test. For each of the 8 bool flags the
  test flips it on a real built router and requires `verifyAuthRouteTable` to refuse,
  and checks the pinned value equals gin.New()'s own default.
- `verifyAuthRouteTable` now pins `UseEscapedPath`; `bannedEngineSel` bans it.
- Mutants: verifier `FA`, `FA2`; AST `RA` (main.go), `RA2` (buildRouter); runtime
  overlay `RA3` (buildRouter sets it), `RA4` (verifier ignores it). All killed.
- The wording "flags are gin's defaults" is replaced: the flags are pinned to the
  values gin.New() sets, and a test proves that equality.

## F-B: verify-call sequence
`scanMainVerifySequence` (AST) pins in main(): exactly two top-level
`if err := verifyAuthRouteTable(router); err != nil { logger.Fatal(...) }`, no other
mention of the verifier, no router-touching statement after the first except the one
`router.Use(altSvcMiddleware)` and the second verify, first verify before every
`ListenAndServe*`, second after the Use and before `srv.ListenAndServe`.
Mutants RB, RB1, RB2 (delete), RC, RC2 (Fatal to Warn), RD (registration after first
verify), RE (condition inverted): all killed.

## F-C: patternReachesAuthPaths vs oracle
`TestPatternReachesAuthPaths_AgreesWithOracleOnCorpus` compares it with the
independent oracle `inAuthSurface` over a generated corpus (more than 5000 patterns).
CORRECTION (round 5): the round-4 text claimed the corpus exposed an oracle defect that
was fixed in the oracle. That was wrong. The disagreement was a real production defect
(an empty parameter value shifts the alignment) and the oracle had been edited to hide
it. Round 5 restores the oracle and fixes production. Verifier case `FC` and runtime
mutants RM2 and RM2b (drop ToLower) are killed.

## F-D (not implemented, suggested separate item)
SetTrustedProxies / ForwardedByClientIP / RemoteIPHeaders / TrustedPlatform changed
after buildRouter pass the verifier and let clients choose the IP seen by IP-keyed
limiters. Out of scope here.

## F-F
The pre-existing Alt-Svc `Use` data race (a Use while the HTTPS goroutine already
serves) is disclosed in round 3 and left unchanged.

## Results (measured at 56c35110d)
- Focused suites: 133 PASS lines (-v).
- Runtime mutants (CHPW_RUNTIME_MUTANTS=1): 39 killed (32 + 3 + 4); Trimpath not re-run.
- Full `go test -p 1 -v .`: 315 PASS lines (subtests included), 5 SKIP, 1 FAIL (pre-existing TestSSRF_RealBinary_SyncEndpointURL).
- Landing note: the branch also carries the four CAT-CHPW-RL-1-FU commits f091b2cf0..4be1f0570, not yet on main.
