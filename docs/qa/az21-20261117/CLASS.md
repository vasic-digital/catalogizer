# AZ2-1 class enumeration (outbound HTTP clients in catalog-api, non-test code)

Instrument: `census_after.txt` (grep, control needle 17 `NewProviderClient` sites, negative control 0) plus the AST guard
`TestAZ21_NoPlainHTTPClientOutsideNetguard` (control needle and polarity: `TestAZ21_GuardInstrumentSeesPlainClients`).

| Member | Before | Disposition |
|---|---|---|
| IGDB API (`igdb_resolver.go` `http`) | plain Client | CLOSED: NewProviderClient strict |
| Cover Art Archive resolver (`cover_art_archive_resolver.go` `http`) | plain Client | CLOSED strict (the gocaa library's own client: SCOPED OUT, AZ2-1a) |
| game / music / movie / book recognition providers | plain Client | CLOSED strict (4 clients) |
| lyrics service, translation service | plain Client | CLOSED strict |
| recommendation service | plain Client | CLOSED strict |
| aggregation service (TMDB enrich loop) | plain Client | CLOSED strict |
| LLM image search API client (`llm_image_resolver.go` `client`) | plain Client | CLOSED non-strict (operator endpoint, allow-list honoured) |
| TMDB (`handlers/media_entity_handler.go`) | plain / proxy Client | CLOSED strict, operator proxy via `ProxyFromConfig` |
| provider manager shared client (`providers.go`, also LLM provider) | plain / HTTP proxy Client | CLOSED non-strict, HTTP proxy kept |
| LLM provider client built in `main.go` (`buildProxyHTTPClient`) | plain / proxy Client | CLOSED non-strict, SOCKS5 / HTTP proxy via `ProxyFromConfig` |
| error reporting webhooks (`services/error_reporting_service.go`) | plain Client | CLOSED non-strict (operator webhook URLs) |
| Firebase default client (`internal/firebase/firebase.go`) | plain Client | CLOSED strict (an injected `cfg.HTTPClient` is the caller's) |
| fanart.tv, IGDB download, cover-art default, subtitle download, asset resolvers, LLM image download | `netguard.StrictClient` (earlier SSRF rounds) | ALREADY GUARDED, unchanged |
| fanarttv library's own client (`fanart_resolver.go` `fanart`) | library-owned | SCOPED OUT (AZ2-1a) |
| cover-art download with an operator proxy (`cover_art_service.go`) | Client + `guardedRedirect` | SCOPED OUT, reason: proxy resolves the destination, hops re-checked by `guardedRedirect` (AZ2-1b) |
| image proxy (`main.go` `buildImageProxyClient`) | netguard dial + host allow-list (AZ2-4) | SCOPED OUT, already class-guarded |
| DNS-over-HTTPS fallback (`main.go` `dnsDoHClient`) | plain Client | SCOPED OUT: it exists because local DNS may answer loopback; a dial-time check would defeat it; constant https URL, parsed as DNS JSON |
| operator storage clients (`filesystem/*`, `services/webdav_client.go`) | netguard class transports | SCOPED OUT, already guarded (operator classes) |
| `internal/httpclient/pool.go` | generic primitive | SCOPED OUT: no caller outside its package (grep) |
| `challenges/*` | harness clients of the local API under test | SCOPED OUT: not provider clients |
