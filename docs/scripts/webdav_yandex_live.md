# webdav_yandex_live.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-03 |
| Last modified | 2026-11-03 |
| Status | Implemented; live runs against the real Yandex WebDAV recorded in `docs/qa/webdav-yandex-live-20261103/` |
| Source | `scripts/test-infra/webdav_yandex_live.sh`; tests `catalog-api/filesystem/live_webdav_yandex_test.go` (build tag `live`), `catalog-api/internal/services/live_webdav_scan_test.go` (build tag `live`); unit-side decisions in `catalog-api/filesystem/live_webdav_yandex_logic_test.go` |

## Purpose

Runs the LIVE suite of Catalogizer's own `WebDAVClient` (and the WebDAV `GenericScanner`) against the REAL Yandex WebDAV service, N times (default 3), and checks that every run produced the same step results and the same content hashes. Constitution section 11.4.289: real service only; no credentials or an unreachable service is an honest SKIP (exit 3), never a stand-in.

## Hard rule: everything stays in `/Testing`

The test installs a guard `http.RoundTripper`. Every request whose URL path (or `Destination` header) is not `/Testing` or below, every non-https request and every foreign host is refused before a byte leaves the process (and counted as a guard violation in the evidence). The suite works in a unique `Testing/catalogizer-live-<epoch>` directory and deletes only that. `/Testing` itself is created only when missing and is never deleted. The decision function `yandexAllowedPath` is unit-tested (`TestYandexAllowedPath`, default `go test`).

## Credentials

`YANDEX_WEBDAV_ENV_FILE` names a file (mode 0600, else the script refuses) defining `WEBDAV_URL`, `WEBDAV_USER`, `WEBDAV_PASSWORD`, `WEBDAV_TEST_DIR` (`/Testing`). The script sources it inside its own process, maps the values to `LIVE_WEBDAV_*` environment variables and unsets the originals. No credential appears on any command line; the output is redacted by an `awk` filter that reads the secrets from `ENVIRON`. The evidence JSON holds no credential, no header and no host-specific address.

## Usage

```bash
YANDEX_WEBDAV_ENV_FILE=/path/to/0600.env scripts/test-infra/webdav_yandex_live.sh
YANDEX_RUNS=1 YANDEX_EVIDENCE_DIR=/path/to/dir YANDEX_WEBDAV_ENV_FILE=... scripts/test-infra/webdav_yandex_live.sh
# the scan leg (separate test, same environment names after the mapping above):
cd catalog-api && go test -tags live -count=1 -v -run TestLiveYandexScan ./internal/services/
```

Heavy runs are wrapped in `systemd-run --user --scope -p MemoryMax=5G -p MemorySwapMax=0` when `systemd-run` exists. Run it in the background: a run takes several minutes because the service throttles uploads (see [TESTING_GUIDE](../TESTING_GUIDE.md), section "Live Yandex WebDAV suite (real service, `/Testing` only)").

## Exit codes

0 `VERDICT=PASS` or `VERDICT=PASS_WITH_ANOMALIES` (every run passed, signatures identical, integrity fields as above; the latter when transient retries occurred); 1 `VERDICT=FAIL`; 2 `VERDICT=ERROR` (env file not mode 0600); 3 `VERDICT=SKIPPED` / `PARTIAL` (no env file, credentials absent, service unreachable).

## Evidence

Per run `runN.json` under `YANDEX_EVIDENCE_DIR`: request count by method and status, bytes uploaded, latency p50/p95/max, guard violations (exactly 2: the two deliberate out-of-sandbox probes of step 17; the script FAILs on any other value), `transient_retries` and `transient_retry_classes` (service resets/5xx/stalls, printed and summarised as ANOMALIES), `lost_acked_writes` (any value above 0 FAILs: a write the service acknowledged was absent on read-back) and `idle_cancel_errors` (any value above 0 FAILs: `context canceled` is never retried), per-step results and notes (sha256 values, mtime skew, per-operation timings), and a `signature` object that must be identical in all runs. The final comparison prints `SIGNATURES IDENTICAL`.

## Honest scope

The scan leg (`TestLiveYandexScan_RemoteTreeIsCatalogued`) uses the real remote tree and the real scanner; the database record sink is an in-memory collector (FOLLOW_UPS W8). The live tests replace the client transport with a guard transport (FOLLOW_UPS WD-F3). See `docs/FOLLOW_UPS_NEXT_PHASE.md` WD-F1 (possible lost acknowledged write, UNCONFIRMED).
