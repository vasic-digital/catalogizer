# ALTSVC5 evidence: shared tls.Config between http.Server and http3.Server (PROVISIONAL)

Base: origin main 88c0bc3ed. Host runs: GOMAXPROCS=2, nice, go test -race, one build at a time.

## Reproduction (RED, base)
- `go test -race -c` of package main, then `-test.run TestAltSvc -test.count=1` six times: DATA RACE in runs 1,3,4,5,6 (5 of 6; run 2 clean). Files red-base-run1..6.txt.
- Conflicting accesses (run1): WRITE net/http.http2ConfigureServer (h2_bundle.go:4275) via (*Server).onceSetNextProtoDefaults from the ServeTLS goroutine (main_h3.go:50); READ crypto/tls.(*Config).Clone via http3.ConfigureTLSConfig (quic-go v0.59.0 server.go:51) from (*http3.Server).Serve in the h3 goroutine (main_h3.go:66). Both servers held the same tlsConfig pointer.
- Production path: main.go:1501 calls startH3Stack (the only caller), so main() startup goes through the identical shared config. Production impact (torn NextProtos/ALPN vs only a race report) is UNCONFIRMED.

## Regression tests (main_altsvc5_test.go)
- TestAltSvc5_NoSharedTLSConfig (deterministic, no race detector needed): NotSame of the two servers' TLSConfig, cert count, ALPN [h3 h2 http/1.1].
- TestAltSvc5_ConcurrentStartRaceFree: 20 start/stop cycles; fails under -race on base.
- On base (fix stashed): 3 of 3 runs FAIL both tests, 7/2/3 race reports (red-newtests-base-run*.txt).

## Fix (main_h3.go)
httpsTLS := tlsConfig.Clone() and h3TLS := tlsConfig.Clone(), taken before either server starts; each server gets its own. Certificates and NextProtos identical.

## Green
- go vet .: rc 0 (vet.txt).
- -race -run 'AltSvc|H3|AuthSurface' x3: rc 0, 0 DATA RACE (green-fix-race-run1..3.txt); includes the real-socket HTTP/2 and the real HTTP/3 request tests.
- Without -race: -run 'REDFIX1|S1G|ADMD1F1|PreGate': ok (guards-norace.txt). No guard pins main_h3.go text affected by the fix; no guard was edited.

## Mutants (mutants.sh, mutants-summary.txt; each 3 runs, -race, TestAltSvc5)
- M1 revert to shared config: KILLED 3/3.
- M5 both servers share ONE clone: KILLED 3/3.
- M4 h3 clone taken late (after the https goroutine started, https keeps the original): KILLED 3/3.
- M2 clone only https / M3 clone only h3: SURVIVE, as expected: the other server then holds the sole reference to the original, nothing is shared and no race exists (equivalent mutants, not a gap).
- M4's first build failed on an unused variable; re-done by hand with `_ = h3TLS` (see summary tail).

## Other packages
Repo grep (excluding submodules): the same pattern (one tlsCfg to http.Server and http3.Server) exists in constitution/scripts/scheduled-work-engine/cmd/scheduled-work/main.go lines 120-160 (vendored constitution, NOT modified here; report upstream). Nothing else.

## Limits
The race is timing-dependent; the loop test is probabilistic under -race (3/3 on base here), the pointer test is deterministic. Whole-package -race was not run, only the targeted set. Status PROVISIONAL.
