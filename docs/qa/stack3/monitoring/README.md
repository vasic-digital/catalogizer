# Stack 3 monitoring add-on (QA)

Prometheus + Grafana + pgAdmin + nginx reverse proxy for the running QA stack 3 (`catqa3`), as one rootless podman pod
`catqa3mon` on `pasta --map-gw`. It only READS from stack 3 (HTTP scrape / proxy / TCP to Postgres); no `catqa3-*` container is touched.

| Service | Bind (host) | Container port | Notes |
|---|---|---|---|
| nginx reverse proxy: web + `/api` + `/ws` + `/health` | 0.0.0.0:33080 | 8088 | `/edge-health` answers locally |
| nginx reverse proxy: marketing website | 0.0.0.0:33081 | 8089 | |
| Grafana | 0.0.0.0:33030 | 3000 | user `GF_SECURITY_ADMIN_USER` in `mon.env`; datasource + dashboard "Catalogizer API (QA stack 3)" provisioned |
| pgAdmin | 0.0.0.0:33050 | 5050 | login `PGADMIN_DEFAULT_EMAIL`; server `catqa3-postgres` preconfigured |
| Prometheus | **127.0.0.1**:39090 | 9090 | no auth -> loopback by default; LAN opt-in: `PROM_BIND=0.0.0.0 ./up.sh` |

Pinned images (`lib.sh`): prom/prometheus v2.55.1, grafana/grafana 11.3.1, dpage/pgadmin4 8.13, nginx 1.27-alpine, curlimages/curl 8.10.1
(digests recorded in `evidence/image_digests.txt`). Memory limits sum to 1.5 GiB.

## Use
```
docs/qa/stack3/monitoring/up.sh          # idempotent; replaces only pod catqa3mon; secrets generated on first run
docs/qa/stack3/monitoring/verify.py      # live checks, one PASS/FAIL line each (exit 0 = all PASS)
docs/qa/stack3/monitoring/down.sh [--purge]
```
Secrets: `$QA_DIR/mon.env` (0600, outside the repo; Grafana + pgAdmin passwords, generated). Postgres/admin credentials come from the existing
`$QA_DIR/qa.env`. Everything is passed to containers by variable NAME (`-e NAME`); the Postgres password reaches pgAdmin as a podman secret
(`catqa3mon-pgpass`, mounted 0600 at `/pgpass`, `PGPASSFILE`).

## Design notes (all measured 2026-11-05)
* **`/metrics` auth.** The API accepts an admin bearer token or `METRICS_SCRAPE_TOKEN` (`catalog-api/main_metricsauth.go`). The running stack-3 API
  has no `METRICS_SCRAPE_TOKEN`, and it must not be restarted from here, so `token_refresher.sh` (sidecar) logs in as `qa_admin` every 6 h and writes the
  24 h session token to a shared volume; Prometheus reads it via `authorization.credentials_file`. Preferred long-term: set `METRICS_SCRAPE_TOKEN`
  (>= 32 chars) in stack 3's `up.sh` and scrape with it (tracked as a follow-up; changes stack 3, so out of scope here).
* **Networking.** A pasta container cannot reach the host's own LAN IP (`http://192.168.1.115:38082` -> 000). With `--map-gw`,
  `host.containers.internal` (169.254.1.2) reaches the host's non-loopback listeners (API/web/website), while the *default-gateway address*
  (`MON_PG_HOST`, computed from the host route) is mapped to the host loopback and so reaches Postgres on `127.0.0.1:35434`. Side effect: the
  real LAN gateway address is unreachable from inside this pod (irrelevant for these services).
* **pgAdmin.** `servers.json` is imported on first start of a fresh volume (`down.sh --purge` then `up.sh` re-imports). In server mode pgAdmin ignores
  the `PassFile` key, so `PGPASSFILE` is exported instead. Verify uses an unknown user for the wrong-password control: a wrong password on
  the real account trips pgAdmin's / Grafana's login lockout.
* **nginx.** Security headers (nosniff, X-Frame-Options, Referrer-Policy, Permissions-Policy, COOP), `server_tokens off`, gzip, request-rate limit,
  WebSocket upgrade for `/ws`. Plain HTTP only (no certificate on this QA host, so no HSTS). **Brotli is not available**: the stock nginx image has no
  brotli module; not faked. `X-Content-Type-Options` etc. come from nginx; the API's own headers pass through unchanged.

## Limits
Prometheus data lives in a named volume (retention 7 d); pgAdmin `servers.json` shows the single Postgres server; the dashboard's latency/error panels
are empty when no matching series exists (verify allows that only for those two panels); no SonarQube (out of scope); no TLS; `verify.py`
needs the repo's `qa.env`/`mon.env` and runs on the QA host.
