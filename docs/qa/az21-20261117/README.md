# AZ2-1: provider clients through one guarded constructor (PROVISIONAL-DONE)

Reproduction record (§11.4.292): result `reproduced-live`, surface host-only (real loopback listeners, real clients).
Command: `GOMAXPROCS=2 nice -n 10 go test -p 1 ./internal/services -run TestAZ21 -count=1 -v` at baseline `71e0b7c60`
(the test file and `internal/netguard/netguardtest` were the only additions).
Topology: "front" on 127.0.0.1 answers `/ok` 200 and `/to-unspecified` 302 to `http://0.0.0.0:<sinkport>/secret`; "sink" is a second
real listener that counts hits. 0.0.0.0 is a class netguard never admits (even with an allow-list) and Linux connects it to the host.
Control needles in the test: the client reaches front `/ok`, a direct request reaches the sink and moves the counter to exactly 1.
RED: `red_services_baseline.txt` (10 members FAIL "client followed a redirect to 0.0.0.0 and reached the sink").
GREEN: the same test after the fix; `green_summary.txt` and `green_full_packages_b.txt` (package runs; the FULL handlers suite hit the 10 min timeout under load, so it is UNCONFIRMED, only targeted handlers and root runs are green).
Class enumeration: `CLASS.md`, `census_after.txt`. Mutants: `mutants.txt` (9 in round 1 plus 5 in round 2, all killed). Logs are `.txt` because `.gitignore` drops `*.log` (round-2 finding I1).

Tests added: `internal/netguard/provider_client_test.go` (redirect class, scheme, loop, hop limit, dial class check, allow-list strict vs non-strict,
test relax loopback-only, DNS-rebinding pin via the lookup seam, response cap, operator HTTP proxy hop check, SOCKS5 handshake bound),
`internal/netguard/provider_client_guard_test.go` (AST guard + instrument self-test), per-package real-client tests in internal/services (11 members),
handlers (TMDB), internal/media/providers, main, services (error reporting).

Honest limits (UNCONFIRMED unless stated):
- DNS rebinding: proven at the dial seam with replaced lookup/dial functions, not with a real rebinding DNS server; no pre-fix RED for it.
- Behind an operator proxy the destination is resolved by the proxy: hop checks cover literal IP hosts and scheme only.
- gocaa and fanarttv own their clients (not injectable here). Firebase default client and live provider hosts not exercised against the internet.
- The TMDB handler hard-codes its host, so the handler path could not be driven end to end with a loopback upstream; the real handler method's client is tested.
- Behaviour change: providers.go / handler clients no longer follow redirects to blocked classes, and responses over 64 MiB fail with ErrResponseTooLarge.
- PROVISIONAL-DONE: no live rerun on a rebuilt stack, no independent review yet (Opus review owed).

Round 2 changes (Opus review NO-GO):
- I2 resource leak: `NewProviderClient` now sets `IdleConnTimeout` 30s, `MaxIdleConns` 10, `MaxIdleConnsPerHost` 2, and `cappingTransport` forwards
  `CloseIdleConnections` (without it `http.Client.CloseIdleConnections` was a no-op). The two per-call sites (`MediaEntityHandler` TMDB search and
  `AggregationService.enrichNewEntities`) call `defer client.CloseIdleConnections()`. Class census of every `NewProviderClient` site: all others are built
  once in a constructor called from wiring (`main.go`, `media.NewManager`, service constructors); whether `NewMediaPlayerService` / `NewSubtitleService`
  are constructed more than once at runtime is UNCONFIRMED (grep shows wiring-time calls only).
- M1/M2/M3 tests and mutants added (see `mutants.txt`). M4: the DoH client is migrated to the shared client (Strict, 5s, 1 MiB cap), scope-out removed.
- M7 behaviour change (listed, not a regression to hide): the proxy-aware builders no longer read `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` from the
  environment; only the operator's configured proxy (`ProxyFromConfig` SOCKS5 or HTTP) is used, and with none the connection is direct and dial-guarded.
  Every provider client therefore ignores ambient proxy variables. An operator who relied on them must configure the proxy settings instead.
