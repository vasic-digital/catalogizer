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
fabricated token `ZZ_NO_SUCH_TOKEN_ZZ` returned 0. Result: 44 non-test files construct a client directly:
23 under `challenges/` (one row below), 21 elsewhere. Every one of the 21 appears in the table
(`netguard.go` is the implementation itself, `webdav_client.go` is two files, one per package);
the table also lists the three files that now obtain their client from `netguard.StrictClient`
and the raw `net.Dial*` sites, which are not HTTP but are outbound connections.

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
| `services/sync_service.go` | `netguard.Transport()` | STRICT + allow-list (operator-configured endpoint) | NAS/sync endpoints; `netguard.Transport()` refuses private ranges unless `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` lists them, so a LAN sync endpoint needs the allow-list (unlike a LAN storage root). Unification with the operator policy is tracked (CAT-SECQ1-A follow-up W5) |
| `filesystem/webdav_client.go`, `services/webdav_client.go` | WebDAV | OPERATOR | operator-configured server; `netguard.OperatorDialContext` (SECQ-1/W5): loopback, link-local/metadata, unspecified, multicast and their mapped/NAT64/6to4 forms refused at dial and on redirect hops; RFC 1918/ULA/public allowed; `CATALOGIZER_OPERATOR_DENY_PRIVATE=1` also refuses private; env proxy honoured fail-closed (destination host checked locally first; proxy address bypasses the policy). Address classes (CAT-SECQ1-A, `netguard/class.go`): never (loopback, link-local, metadata incl. 168.63.129.16, reserved, the server's own interface addresses), private, public. The transport is approved for `private` (default) and pins itself to the class of its FIRST connection: once its first connection reached a public address, THAT TRANSPORT cannot afterwards reach a private or never-class address, through a redirect (also inside gowebdav, which owns its CheckRedirect), a second hop or a DNS answer that changes later. Honest scope: the pin lives in the transport object, not in storage. A process restart, a new client or a re-created root starts a new pin (the S3 client keeps one transport across its own Connect calls); persistent per-endpoint approval is the later `approved_class` work (CAT-SECQ1-B/C). Concurrent first connections are serialised, so the first one decides. The server's own interface addresses are class never unless allow-listed; if the interface list cannot be read, private-range addresses are refused (fail closed). `services/webdav_client.go` is not on a live path today (its only caller replaces the transport with the strict `netguard.Transport()`; wiring is tracked) `OperatorTransportFor(class)`, `OperatorDialContextClass` and `OperatorCheckRedirectFor(class)` take an explicit approved class (stored per endpoint by a later change). A dial-time `Control` hook re-checks the address the kernel connects to. Host names must be ASCII (punycode for IDNs) and not numeric spellings, on the strict and the operator path |
| `services/error_reporting_service.go` | `httpClient` | OPERATOR | operator-configured webhooks |
| `internal/netguard/netguard.go` | `Client`, `StrictClient`, `Transport` | implementation | dial-time check, pinned IP |
| `internal/httpclient/pool.go` | pooled client | SELF | used by `challenges/` against our own server |
| `challenges/*.go` (23 files) | probes | SELF | QA harness, base URL is our server |

### Storage connectors (smb, ftp, nfs): host restrictions (SEC-DIAL-1)

An smb, ftp or nfs storage root names a HOST (not a URL). It gets the same operator-class policy as a WebDAV or S3 root
(`netguard` operator class, see the WebDAV row above): the host is refused when it is or resolves to loopback, link-local
(169.254.0.0/16, fe80::/10), unspecified, multicast, a cloud metadata address (169.254.169.254, 100.100.100.200,
fd00:ec2::254, fd20:ce::254, 168.63.129.16), a reserved range (198.18.0.0/15, 192.0.0.0/24, ...), one of the server's own
interface addresses, or an IPv4-mapped / compatible / NAT64 / 6to4 form of those. RFC 1918, ULA, CGNAT and public hosts
are allowed, so a LAN NAS works without configuration. `CATALOGIZER_OPERATOR_DENY_PRIVATE=1` also refuses private hosts and
`CATALOGIZER_OUTBOUND_ALLOW_CIDRS` re-admits loopback / private networks it names (never the metadata addresses).

Where it is enforced:

- **Root create** (`POST /api/v1/storage/roots`): `filesystem.VetStorageRoot` refuses with HTTP 400 and the generic message
  `storage host is not allowed` (no address or resolver detail is returned) and nothing is stored. `filesystem.ValidateSettings`
  (so `ValidateRoot` and the client factory) refuses a malformed host: only an IP literal or a plain DNS name
  (`[A-Za-z0-9_.-]`, not starting with `-`); a port, userinfo, bracket, slash, comma, whitespace or control character is refused.
- **Every dial**: the smb and ftp clients (ftp control AND data connections, including the CopyFile tracker) connect through
  `netguard.OperatorDialContext`, which resolves, checks and dials the vetted IP itself (pinned, never the name; the Control hook
  re-checks the address the kernel connects to), so DNS rebinding between create and dial cannot redirect a connection.
  `SMBService.getConnection` (config-file hosts) uses `VetSMBHostIPs` + `dialVetted`, the same pinned path as SMB discovery.
- **NFS mount**: the host is resolved and vetted at mount time and the kernel receives the vetted ADDRESS (`<ip>:<export>`,
  `addr=<ip>`), never the name. Export path and mount options are checked for injection: the export may not contain whitespace,
  control characters or `,` and may not start with `-`; options are a closed grammar of comma-separated `key` or `key=value`
  elements (key `[A-Za-z0-9_]`, value `[A-Za-z0-9_.:-]`, no empty element). `addr=` and `mountaddr=` must be IP literals that pass
  the same policy (a user-supplied `addr=127.0.0.1` is refused). The darwin client applies the same checks.

Honest scope: the evidence for this change is unit-class (real handler, real clients, real netguard in-process; no real
NAS or kernel mount). The create-time check resolves the name once for a clear early refusal; the dial-time check is the
enforcement. Discovery routes keep their own pre-existing `VetSMBHost` check.

### Raw dial sites (non-HTTP outbound connections), all OPERATOR class

The target is chosen by the operator (configured storage/NAS/FTP/SMB host, discovery of the
operator's own network). None takes an address from third-party data. They do not honour
`CATALOGIZER_OUTBOUND_ALLOW_CIDRS` because they are meant to reach the LAN.

| File | What it dials |
|---|---|
| `services/configuration_wizard_service.go` (`testNetworkDatabaseConnection`) | TCP reachability probe of an operator-entered database host (operator-trusted database target; loopback is legitimate; unvetted, tracked in the SEC-DIAL-1 report) |
| `internal/services/smb_discovery.go`, `smb_probe.go` | SMB hosts (445) the operator scans: `VetSMBHostIPs` + `dialVetted` |
| `internal/services/smb.go` (`getConnection`) | config-file SMB host: `VetSMBHostIPs` + `dialVetted` (SEC-DIAL-1) |
| `filesystem/ftp_client.go`, `filesystem/ftp_ops.go` (`ftp.Dial` via `dial`/`dialWith`, `connTracker`, `ftpConnGroup`) | operator-configured FTP server: `netguard.OperatorDialContext` (SEC-DIAL-1) |
| `filesystem/smb_client.go` | operator-configured SMB server: `netguard.OperatorDialContext` (SEC-DIAL-1) |
| `filesystem/nfs_client.go`, `nfs_client_darwin.go` | operator-configured NFS server: vetted address mounted (SEC-DIAL-1) |
| `database/connection.go` (`sql.Open("postgres", ...)`), `main.go` (`redis.NewClient`) | operator-configured Postgres and Redis servers (database/cache connections from operator configuration, not user data; trusted) |
| `internal/infra/provisioner.go:259` | local infrastructure readiness probe |
| `main.go:150` | UDP "connect" to 8.8.8.8 to learn the local address; sends no data |
| `challenges/*`, `tests/infra_helper.go` | QA harness |

## Residual (tracked in `docs/FOLLOW_UPS_NEXT_PHASE.md`)

* FIXED-VENDOR clients still use a plain `http.Client` (redirects followed). The host is
  fixed, so an attacker cannot choose it; a vendor-side redirect to an internal host would
  be followed. Move to `netguard.Client` when a vendor base becomes operator-writable.
* PROXY clients cannot pin the destination. The only address check is `GuardProviderURL`,
  which resolves the host in THIS process before the request; the proxy then does its own,
  separate DNS lookup, so a name that answers differently to the proxy (rebinding) is not
  caught. Redirect hops are re-checked the same way. Treat the operator proxy as trusted to
  enforce its own egress policy.

## Known residuals of the address classification (S1A3-2, S1A3-3)

`netguard.ClassOf` unwraps the IPv6 forms that carry an IPv4 address and are recognisable from the address alone:
IPv4-mapped, IPv4-compatible (`::/96`), SIIT (`::ffff:0:a.b.c.d`), the well-known NAT64 prefix `64:ff9b::/96` and 6to4
(`2002::/16`). Two forms are NOT recognisable and are classified public (hence reachable by every client class):

- NAT64 with an operator network-specific prefix (RFC 6052 section 2.2, for example your own `/96` inside global space
  followed by the IPv4 address). The server does not know the prefix. There is no configurable prefix list today; if you
  run such a gateway, stop it from forwarding to private, loopback and metadata destinations. Adding a configurable list
  of network-specific prefixes is possible later work (S1A3-2).
- ISATAP interface identifiers (`::5efe:a.b.c.d`, `::200:5efe:a.b.c.d`) under a global prefix (S1A3-3). They name a
  tunnelled host, not a literal IPv4 destination of this server.

Both are pinned by tests (`TestS1A3_2_*`, `TestS1A3_3_*` in `catalog-api/internal/netguard/s1a3_test.go`).

## Stored approval per endpoint (CAT-SECQ1-B / W3)

Migration v27 adds `approved_class` (`private` default, or `public`) and `approved_class_source` (`default`, `auto`,
`operator`) to `storage_roots` and `sync_endpoints`. `private` is the behaviour from before SECQ-1 (private and public
addresses reachable); `public` allows public addresses only. `never` is not storable. Every row that existed before the
migration is `private/default`; the migration does not resolve names or read the machine's interfaces.

- `netguard.ClassifyHost` / `ClassifyEndpoint` derive the class from a host (IP literal or DNS name, or the host of a URL):
  all addresses public gives `public`, any private address gives `private`, any never-class address (loopback, link-local,
  metadata, the server's own address unless allow-listed), an empty host, a non-ASCII or numeric-spelling host, or a name
  that does not resolve is an error and nothing is stored. A name with several answers is refused as soon as ONE answer is
  never-class, whatever the order. The three metadata singletons (`fd00:ec2::254`, `fd20:ce::254`, `100.100.100.200`) are
  never-class for EVERY client class (one shared list in `netguard.go`, checked before the allow-list, so classification
  and the dial agree; operator decision 2026-11-13); Teredo (`2001::/32`) is refused by the operator class only.
- `database.DB.GetApprovedClass` / `SetApprovedClass` / `AutoClassifyApprovedClass` read, write and classify the stored
  value; an `operator` approval is never overwritten by automatic classification (the automatic write is conditional on the stored
  source not being `operator`, so a decision made while a slow DNS lookup runs survives).
- Not wired yet: the create/update handlers (W4, CAT-SECQ1-C) that call the classifier, ask for confirmation of `private`
  and require step-up to widen, and the clients (`OperatorTransportFor`) that consume the stored class (W4/W5).
  Until then the stored value is not enforced anywhere; the first-connection pin remains the only per-root protection.
- The PostgreSQL branch of the migration is exercised by `TestRunMigrationsPostgresRealContainerZeroErrors`, which starts
  its own throwaway container and honestly SKIPs without podman or the image.

## Dial cap trade-off (SEC-DIAL-1-FU F6)

`netguard.OperatorDialContext` tries at most `maxVettedDials` (4) vetted addresses of one name, in resolver order, so a name with many answers cannot multiply the connect cost (the same bound discovery uses). Trade-off: a name whose first four vetted answers are all unreachable is reported as failed even if a fifth or later answer is live; that address is never tried. Operators with such a name should list fewer answers (or an IP literal) for the storage host. Unit-class tests: `internal/netguard/sec_dial_fu_test.go`.
