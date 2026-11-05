# Stack 3 monitoring add-on (QA)

Prometheus + Grafana + pgAdmin + nginx reverse proxy for the running QA stack 3 (`catqa3`), as one rootless podman pod
`catqa3mon` on `pasta` with four `-T` port forwards. It only READS from stack 3 (HTTP scrape / proxy / TCP to Postgres); no `catqa3-*` container is touched.

| Service | Bind (host) | Container port | Notes |
|---|---|---|---|
| nginx reverse proxy: web + `/api` + `/ws` + `/health` | 0.0.0.0:33080 | 8088 | the LAN entry; `/edge-health` answers locally |
| nginx reverse proxy: marketing website | 0.0.0.0:33081 | 8089 | |
| Grafana | **127.0.0.1**:33030 | 3000 | login in `mon.env`; datasource + dashboard "Catalogizer API (QA stack 3)" provisioned |
| pgAdmin | **127.0.0.1**:33050 | 5050 | login in `mon.env`; server `catqa3-postgres` preconfigured |
| Prometheus | **127.0.0.1**:39090 | 9090 | no auth |

**LAN access to Grafana / pgAdmin / Prometheus is an explicit opt-in**: `LAN_BIND=0.0.0.0 ./up.sh` (Grafana + pgAdmin) and/or `PROM_BIND=0.0.0.0 ./up.sh`.
Use it for manual QA only; both UIs are admin consoles that can be pointed at other targets (see Residual risk). Otherwise reach them with an
SSH tunnel (`ssh -L 33030:127.0.0.1:33030 host`).

Pinned images (`lib.sh`): prom/prometheus v2.55.1, grafana/grafana 11.3.1, dpage/pgadmin4 8.13, nginx 1.27-alpine, curlimages/curl 8.10.1
(digests recorded in `evidence/image_digests.txt`). Memory limits sum to 1.5 GiB. `up.sh` refuses to start if a pulled image's digest differs from `evidence/image_digests.txt` OR the image has no recorded pin (fail closed; explicit override `ALLOW_UNPINNED_IMAGES=1` prints a loud warning).

## Use
```
docs/qa/stack3/monitoring/up.sh          # idempotent; replaces only pod catqa3mon; secrets generated on first run
docs/qa/stack3/monitoring/test_scripts.sh   # sandbox tests (fake curl/podman): bearer never in argv, logout retry, fail-closed digest pins
docs/qa/stack3/monitoring/verify.py      # live checks, one PASS/FAIL line each (exit 0 = all PASS)
docs/qa/stack3/monitoring/down.sh [--purge]
```
Secrets: `$QA_DIR/mon.env` (0600, outside the repo; Grafana + pgAdmin passwords, generated). Postgres/admin credentials come from the existing
`$QA_DIR/qa.env`. No password is in any container's environment or `podman inspect` output (checked: `evidence/inspect_leak_scan.txt`): the Grafana, pgAdmin and
Postgres passwords and the `qa_admin` login used by the token refresher are podman secrets mounted as 0400 files (`GF_SECURITY_ADMIN_PASSWORD__FILE`,
`PGADMIN_DEFAULT_PASSWORD_FILE`, `PGPASSFILE`, the refresher's `CRED_FILE`). Only non-secret usernames/e-mail are passed with `-e`.
(Podman secrets live in the user's podman store, mode-protected; `down.sh` removes them.)

## Design notes (all measured 2026-11-05)
* **`/metrics` auth.** The API accepts an admin bearer token or `METRICS_SCRAPE_TOKEN` (`catalog-api/main_metricsauth.go`). The running stack-3 API
  has no `METRICS_SCRAPE_TOKEN`, and it must not be restarted from here, so `token_refresher.sh` (sidecar) logs in as `qa_admin` every 6 h, writes the
  24 h session token to a shared volume (owner 65534 = Prometheus uid, mode 0400), then logs the previous session out (`POST /api/v1/auth/logout`, bearer and login body passed on curl's stdin - never in argv; up to 3 attempts, then dropped and logged; the superseded token is kept in a 0400 file on the token volume so a container restart still revokes it - if that also fails the session lives until its 24 h expiry);
  `verify.py` also logs out its own sessions and checks the token is rejected afterwards). Login JSON is escaped (`\` and `"`), failures back off
  30 s doubling to 900 s so a wrong password cannot hammer the login lockout. **`QA-MON-FU-1` (implemented 2026-11-05, `seed_scrape_token.sh`, live-verified in docs/qa/stack3-verify-20261105): when stack 3's `qa.env` carries `METRICS_SCRAPE_TOKEN` (stack-3 `up.sh` now generates it) this add-on seeds it as Prometheus' credentials file and starts NO refresher / no admin session; the refresher remains only as a fallback for a stack-3 API without the token. Original follow-up text:** set `METRICS_SCRAPE_TOKEN` (>= 32 chars) in
  stack 3's `up.sh` and scrape with it instead of an admin session (changes stack 3, so not done here). The refresher still holds admin credentials (as a secret).
* **Networking / residual risk.** The pod is on `pasta` with `-T` port forwards for exactly four host ports (Postgres 35434, API 38082, web 33020,
  website 33021): inside the pod `127.0.0.1:<port>` reaches the host's `127.0.0.1:<port>`. An earlier revision used `--map-gw`, which exposed ALL
  host-loopback services to the pod (measured before the change: 35434, 36381, 38082 and 11434 all reachable); after the change only the four are
  (`verify.py`: `pod.reaches_*` needles pass, `pod.CANNOT_reach_*` for 36381, 11434, 35433 pass). Residual: a pgAdmin or Grafana admin can still add
  servers/datasources pointing at those four ports (the pgpass entry only matches 127.0.0.1:35434; the API/web/website
  are LAN-reachable anyway) and the pod has full outbound internet via pasta. The default loopback bind makes that an operator-only capability.
* **Client IP / shared limits (F4, N3).** Stack 3's API has no `TRUSTED_PROXIES` (`catalog-api/main.go`: unset = trust no proxy header), and this
  add-on does not change stack 3. The wired routes (`LoginGin` etc.) use gin's `c.ClientIP()`, which therefore ignores `X-Forwarded-For`/`X-Real-IP`
  and returns the TCP peer. Behind the nginx proxy the API consequently sees one source address (the pod's pasta egress) for every proxied user:
  per-IP rate limits and login lockouts are shared across all proxy users and API audit logs record that address, not the real client.
  (The legacy `net/http` handler `AuthHandler.Login` / `getClientIP` in `handlers/auth_handler.go` does trust `X-Real-IP` unconditionally, but it is not
  registered in stack 3's router - only the `*Gin` handlers are.) nginx sets `X-Real-IP` to `$remote_addr` (overwriting any client value) and appends to
  `X-Forwarded-For` (`$proxy_add_x_forwarded_for`, so a client-supplied prefix is preserved there). Fix: stack-3 `up.sh` now sets `TRUSTED_PROXIES` (explicit CIDR, `lib.sh`; value measured live, see docs/qa/stack3-verify-20261105).
* **Restart window (named follow-up `QA-MON-FU-2`).** If the refresher container dies after writing a new `metrics.tok` but before it updates `previous.tok` (or between revoking and re-persisting), one admin session can be left to expire on its own (<= 24 h). Closing it needs a two-entry revoke list; not done.
* **pgAdmin.** `servers.json` is imported on first start of a fresh volume (`down.sh --purge` then `up.sh` re-imports). In server mode pgAdmin ignores
  the `PassFile` key, so `PGPASSFILE` is exported instead. Verify uses an unknown user for the wrong-password control: a wrong password on
  the real account trips pgAdmin's / Grafana's login lockout.
* **nginx.** Security headers (nosniff, X-Frame-Options, Referrer-Policy, Permissions-Policy, COOP), `server_tokens off`, gzip, request-rate limit,
  WebSocket upgrade for `/ws`. Plain HTTP only (no certificate on this QA host, so no HSTS). **Brotli is not available**: the stock nginx image has no
  brotli module; not faked. `X-Content-Type-Options` etc. come from nginx; the API's own headers pass through unchanged.

## Limits
Stack 3's API limits `/auth/login` to 30 req/min per source IP, and every rejected request counts: run `verify.py` at most once a minute (evidence: one live round on the current head, 45/45; three rounds spaced 70 s apart were run on the previous revision 0547347a), and never loop a login in a test container.
Prometheus data lives in a named volume (retention 7 d); pgAdmin `servers.json` shows the single Postgres server; panels 2 and 3 (error rate, latency) may legitimately be empty
when no matching series exists, so `verify.py` only requires data from panels 1, 4, 5, 6 (panel 7, GC pause, is provisioned and must return HTTP 200 without a query error; 7 panels in total); no SonarQube (out of scope); no TLS; `verify.py`
needs the repo's `qa.env`/`mon.env` and runs on the QA host.
