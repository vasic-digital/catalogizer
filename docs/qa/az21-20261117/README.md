# AZ2-1: provider clients through one guarded constructor (PROVISIONAL-DONE)

Reproduction record (§11.4.292): result `reproduced-live`, surface host-only (real loopback listeners, real clients).
Command: `GOMAXPROCS=2 nice -n 10 go test -p 1 ./internal/services -run TestAZ21 -count=1 -v` at baseline `71e0b7c60`
(the test file and `internal/netguard/netguardtest` were the only additions).
Topology: "front" on 127.0.0.1 answers `/ok` 200 and `/to-unspecified` 302 to `http://0.0.0.0:<sinkport>/secret`; "sink" is a second
real listener that counts hits. 0.0.0.0 is a class netguard never admits (even with an allow-list) and Linux connects it to the host.
Control needles in the test: the client reaches front `/ok`, a direct request reaches the sink and moves the counter to exactly 1.
RED: `red_services_baseline.log` (10 members FAIL "client followed a redirect to 0.0.0.0 and reached the sink").
GREEN: the same test after the fix; `green_summary.txt` and `green_full_packages_b.log` (package runs; the FULL handlers suite hit the 10 min timeout under load, so it is UNCONFIRMED, only targeted handlers and root runs are green).
Class enumeration: `CLASS.md`, `census_after.txt`. Mutants: `mutants.log` (9, all killed).

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
