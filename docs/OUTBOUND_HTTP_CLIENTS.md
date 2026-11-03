# Outbound HTTP clients in catalog-api (SSRF census)

Date: 2026-11-04. Scope: non-test Go code under `catalog-api/` (module `catalogizer`).
Rule: a client that fetches a URL taken from third-party data (provider responses, model
output, subtitle/cover links) MUST be `netguard.StrictClient` (dial-time address check on
every hop, redirects re-checked, `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` ignored, environment
proxies ignored). A string-level `GuardProviderURL` check alone is NOT enough: it checks
once, while a redirect or a second DNS answer reaches hosts it never saw.

## Census method and control needle

Command (from `catalog-api/`): `grep -rlE '&http\.Client\{|http\.DefaultClient' . --include=*.go | grep -v _test.go`,
plus `http.Get(` / `http.Post(` (0 hits in non-test code), `&http.Transport{` and
`netguard.(Client|StrictClient|Transport)(`.
Control needles on the same path: positive `netguard.StrictClient(` returned 6 hits
(>= the 3 production callers known before this change plus the 3 added here), negative
fabricated token `ZZ_NO_SUCH_TOKEN_ZZ` returned 0. Result: 44 non-test files contain a
constructed client; 23 of them are under `challenges/` (QA probes against our own server,
not product code, listed once below).

## Classification

| Class | Meaning |
|---|---|
| STRICT | URL comes from third-party data; `netguard.StrictClient` |
| FIXED-VENDOR | base URL is a fixed vendor API (or config-injected vendor base); the attacker cannot choose the host |
| OPERATOR | destination chosen by the operator in configuration (LAN NAS, WebDAV, LLM endpoint, error-report sink) |
| PROXY | operator SOCKS5/HTTP proxy resolves the destination, so a dial-time check cannot apply; every redirect hop is re-checked with `GuardProviderURL` |
| SELF | test/QA harness calling this server |

| File | Client | Class | Notes |
|---|---|---|---|
| `internal/services/subtitle_service.go` | `httpClient` | STRICT | fixed earlier (authz round 2) |
| `internal/services/asset_resolvers.go` | `ExternalMetadataResolver.client` | STRICT | authz round 2 |
| `internal/services/fanart_resolver.go` | `client` | STRICT | this change (AZ2-R1); image URLs come from fanart.tv responses |
| `internal/services/igdb_resolver.go` | `dl` (downloads) | STRICT | this change; `http` (IGDB API calls) stays FIXED-VENDOR |
| `internal/services/llm_image_resolver.go` | `imgs` (downloads) | STRICT | this change; URL is model output. `client` (orchestrator endpoint) is OPERATOR |
| `internal/services/cover_art_service.go` | `httpClient` | STRICT | this change; when a proxy is configured the per-call proxy client is PROXY (`guardedRedirect`) |
| `main.go` `buildImageProxyClient` / `newImageProxyClient` | image proxy | FIXED-VENDOR + STRICT-LIKE | host allow-list (`imageProxyHosts`), https only, redirects limited to the allow-list, dial via `netguard.DialContext`/`IsBlockedIP` |
| `main.go` `buildProxyHTTPClient` | LLM provider client | OPERATOR / PROXY | operator-configured LLM endpoint |
| `main.go` `dnsDoHClient` | DNS-over-HTTPS | FIXED-VENDOR | fixed resolver endpoints |
| `handlers/media_entity_handler.go` | TMDB search | FIXED-VENDOR / PROXY | `api.themoviedb.org`, `GuardProviderURL` per request |
| `internal/services/aggregation_service.go` | TMDB enrichment | FIXED-VENDOR | |
| `internal/services/cover_art_archive_resolver.go` | (`caa` library client) | FIXED-VENDOR | `http` field unused |
| `internal/services/{movie,music,book,game_software}_recognition_provider.go` | `httpClient` | FIXED-VENDOR | `baseURLs` map (config-injected vendor bases), `GuardProviderURL` on the built target |
| `internal/services/lyrics_service.go`, `translation_service.go`, `recommendation_service.go` | `httpClient` | FIXED-VENDOR | vendor bases only; no URL taken from a response |
| `internal/media/providers/providers.go`, `llm_provider.go` | shared provider client | FIXED-VENDOR / PROXY / OPERATOR | HTTP proxy optional |
| `internal/firebase/firebase.go` | `httpClient` | FIXED-VENDOR | |
| `services/sync_service.go` | `netguard.Transport()` | OPERATOR (LAN) | NAS/sync endpoints; honours the allow-list by design |
| `filesystem/webdav_client.go`, `services/webdav_client.go` | WebDAV | OPERATOR | operator-configured server; env proxy honoured |
| `services/error_reporting_service.go` | `httpClient` | OPERATOR | operator-configured webhooks |
| `internal/httpclient/pool.go` | pooled client | SELF | used by `challenges/` against our own server |
| `challenges/*.go` (23 files) | probes | SELF | QA harness, base URL is our server |

## Residual (tracked in `docs/FOLLOW_UPS_NEXT_PHASE.md`)

* FIXED-VENDOR clients still use a plain `http.Client` (redirects followed). The host is
  fixed, so an attacker cannot choose it; a vendor-side redirect to an internal host would
  be followed. Move to `netguard.Client` when a vendor base becomes operator-writable.
* PROXY clients cannot pin the destination (the proxy resolves it); only the redirect hop
  string check applies.
