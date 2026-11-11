# Cover-art fetch live checks (CAT-COVERART-LIVE-1)

Closes the tooling gap in `docs/PROVISIONAL_VERIFICATION_PLAN.md` section 2.3: no script drove the
cover-art SSRF items F2 (32 MiB cap) and F4 (redirect / private-address blocking) through the
product code path. Status of the items themselves stays PROVISIONAL until a result in this
directory is cited in the changelog.

## What runs
`./run.sh [--out DIR]` runs `go test -tags live_coverart -run TestLiveCoverArt` in `catalog-api`
(file `internal/services/coverart_live_ssrf_test.go`, build-tagged so normal test runs skip it).
The test calls the real `CoverArtService.downloadImage`: string guard `GuardProviderURL`,
the `netguard` HTTP client with its dial-time and redirect checks, and the production
32 MiB cap (the test asserts `coverArtMaxBytes == 32<<20` and never lowers it).

Fixture-input class (11.4.289 clause C(3)): the servers are plain Go HTTP servers on 127.0.0.1 (and
127.0.0.2) started and stopped by the test. They are inputs the real code talks to over real TCP
sockets, not stand-ins for the code under test.

Which client each leg uses (stated plainly):
- Cap legs (F2) and redirect legs (F4) use the **allow-list-aware `netguard.Client`** (set as `s.httpClient`, allow-list `127.0.0.1/32`), NOT the production strict client, because the production client refuses loopback fixtures by design. They exercise the real `downloadImage` body, string guard, cap and netguard dial/redirect layers, but not the `StrictClient` construction.
- The two last rows use the **untouched production client** from `NewCoverArtService` (`StrictClient`).

| Case | Client | Expectation |
|---|---|---|
| body exactly 32 MiB | netguard.Client | accepted, 33554432 bytes |
| body 32 MiB + 1 | netguard.Client | rejected, `cover_art: download exceeds 33554432 bytes`, no body |
| normal PNG | netguard.Client | accepted, byte-identical |
| redirect to an allow-listed host (control needle) | netguard.Client | followed, target contacted |
| redirect to 127.0.0.2 (not allow-listed, counted listener), 169.254.169.254, 10.0.0.5, [::1] (counted listener when IPv6 loopback exists, else closed port) | netguard.Client | each refused with `errors.Is(err, netguard.ErrBlocked)`; counted servers see 0 hits |
| redirect chain: 3 hops (control) followed, 5 hops not | netguard.Client | 5-hop end never reached |
| loopback origin, string guard on, allow-list admitting loopback | production (Strict) | refused with `ErrSSRFBlocked`, 0 hits |
| loopback origin, string guard relaxed, allow-list admitting loopback | production (Strict) | refused by the dial layer with `ErrBlocked`, 0 hits |

The verdict (`verdict.txt`) is PASS only when all 10 named tests show `--- PASS` and no FAIL line exists;
empty output is FAIL (instrument-blind), never PASS.

## Tests of the script
- `./test_run.sh` : CLI contract and parser (golden-good, golden-bad, missing-test, empty-file negative control). Fast, no Go build.
- `./mutation.sh [OUTDIR]` : baseline must PASS; each mutant must FAIL. Mutants are applied with `go test -overlay`
  on a scratch copy of one file (the working tree is never edited):
  `cap` (32 MiB becomes 1 TiB), `strict` (production client made allow-list-aware AND string guard removed),
  `netguard` (block predicate returns false), `m1` (StrictClient replaced by Client), `m2` (link-local check removed),
  `m5` (redirect limit removed), `m7` (IPv6 never blocked), `m8` (IsPrivate removed).
  A mutant counts as KILLED only if the tests ran and an assertion FAILed; a compile failure (BLIND, exit 2) does not.

## Evidence
`results/final/` : per run `go_test.txt` (-v), `go_test.json.txt`, `ENV.txt` (includes git head, tree hash, test-file and run.sh sha256, home path scrubbed to `~`), `verdict.txt`, `SHA256SUMS.txt`;
mutants add `mutation.diff.txt`. Runtime parameters: `GOMAXPROCS=2`, `nice -n 10`, systemd scope `MemoryMax=4G`,
`MemorySwapMax=0` when a user bus exists.

## Scope and limits (honest)
- Requires initialised submodules (the `replace` targets of `catalog-api/go.mod`) and a Go toolchain on the host.
  Constitution 11.4.173 wants builds in a container: this runs `go test` on the host, a recorded deviation, like the SMB runner.
- Real: the product download function, guards and client. Not real: the origin servers (fixtures), and
  the process is a test binary, not the deployed `catalog-api` container behind its HTTP routes.
- NOT a bare-internet SSRF proof: loopback is reachable only through the test hook `AllowPrivateSSRFForTests`
  plus `CATALOGIZER_OUTBOUND_ALLOW_CIDRS=127.0.0.1/32` (the same seam the unit tests use); production code is unchanged.
  The string guard and the dial guard overlap by design, so the `strict` mutant removes both layers.
- Not covered: the HTTP proxy path of `downloadImage`, ffprobe/ffmpeg paths (F3/F5), a crafted m3u8 through `generate-thumbnails`,
  DNS-rebinding hostnames, and the deployed stack. N is 1 run per invocation; run it 3 times for an N>=3 claim.
