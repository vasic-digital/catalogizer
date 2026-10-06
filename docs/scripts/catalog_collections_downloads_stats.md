# catalog_collections_downloads_stats.sh

**Revision:** 1
**Last modified:** 2026-06-25T13:35:00Z

Companion guide for
`scripts/testing/full_automation/catalog_collections_downloads_stats.sh`
(per Helix Constitution §11.4.18 — every script ships an in-source doc block AND
this external user guide).

## Overview

Full-automation REST API test suite for the Catalogizer `catalog-api` (Go/Gin)
backend, covering the **collections / downloads / recommendations / statistics**
surfaces that the existing `catalog_*.sh` suites
(`catalog_functional_matrix`, `catalog_auth_pagination`,
`catalog_browse_filter_search`, `catalog_favorites_resume_lifecycle`) do **not**
touch. It drives the **live** HTTP API with real `curl` requests and asserts, for
every test, BOTH the HTTP status AND a real observable body field — never a
tautology. Each test writes a captured-evidence JSON file (the actual HTTP
response) plus a `.status` sidecar, and prints `PASS`/`FAIL` with the evidence
path, in the anti-bluff `ab_pass_with_evidence` style of §11.4.69. An unreachable
API yields honest `SKIP`-with-reason (§11.4.3), never a fabricated PASS.

Every route below was **verified present in `catalog-api/main.go`** before a test
was written for it (line refs are to that file). Routes that do **not** exist are
deliberately **not** tested — there are no phantom-endpoint tests (§11.4.6).

### Coverage matrix

| Test | Endpoint (main.go ref) | Real assertion |
|---|---|---|
| C1 collections list | `GET /api/v1/collections` (L1410) | 200 + `items[]` + `total` |
| C2 collections unauth | `GET /api/v1/collections` (no token; group gate L1107) | 401 access-control |
| C3 collections CRUD | `POST` / `GET /:id` / `DELETE /:id` (L1411/1412/1414) | create → get-by-id **contains** created name → delete (self-cleaning) |
| D1 download archive | `POST /api/v1/download/archive` (L1125) | 400 + `error` for empty paths (validation proven) — or 200 reachable |
| R1 recommendations root | `GET /api/v1/recommendations` (L1587) | 200 + `items[]` |
| R2 recommendations trending | `GET /api/v1/recommendations/trending` (L1223) | 200 + `items[]` + `time_range` |
| R3 recommendations by-type | `GET /api/v1/recommendations/by-type` (L1591) | 200 + `recommendations` object |
| S1 stats overall | `GET /api/v1/stats/overall` (L1253) | 200 + `success` + `data` |
| S2 stats scans | `GET /api/v1/stats/scans` (L1261) | 200 + `success` + `data` |
| S3 stats scan-history | `GET /api/v1/stats/scan-history` (L1672) | 200 + `history[]` + `count` |
| M1 media popular | `GET /api/v1/media/popular` (L1139) | 200 + `items[]` + `total` |
| M2 media recent clamp | `GET /api/v1/media/recent?limit=99999` (L1138) | 200 + `items[]` + `total` (oversized limit clamped, no 500) |

### Routes deliberately NOT tested (do not exist in main.go — §11.4.6)

- `GET /api/v1/downloads` — there is **no** downloads list route; only the
  per-resource `/api/v1/download/file/:id`, `/download/directory/*path`, and
  `/download/archive` (POST) exist.
- `GET /api/v1/media/recommendations` — recommendations live under
  `/api/v1/recommendations/*`, not under `/media/*`.

## Prerequisites

- `bash` and `curl` on `PATH`.
- `jq` is **optional** — the script falls back to `grep`/`sed` field extraction
  when `jq` is absent.
- A reachable `catalog-api` instance. By default the suite targets
  `http://127.0.0.1:8080`.

## Usage

```bash
# Defaults: BASE_URL=http://127.0.0.1:8080, USER=admin, PASS=$YOUR_QA_PASSWORD
./scripts/testing/full_automation/catalog_collections_downloads_stats.sh

# Explicit endpoint + credentials (credentials via env ONLY, never hardcoded)
CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
CATALOGIZER_USER=admin CATALOGIZER_PASS=secret \
  ./scripts/testing/full_automation/catalog_collections_downloads_stats.sh

# Skip login by supplying a pre-acquired token
CATALOGIZER_TOKEN="<session_token>" \
  ./scripts/testing/full_automation/catalog_collections_downloads_stats.sh
```

### Inputs (env vars, all optional with defaults)

| Variable | Meaning | Default |
|---|---|---|
| `CATALOGIZER_BASE_URL` | API base URL | `http://127.0.0.1:8080` |
| `CATALOGIZER_USER` | login username | `admin` |
| `CATALOGIZER_PASS` | login password | _(required — §11.4.10)_ |
| `CATALOGIZER_TOKEN` | pre-acquired session token (skips login) | _(empty)_ |
| `CATALOGIZER_RESULTS_DIR` | evidence output directory | `qa-results/collections_downloads_stats/<ts>` |

Credentials are read from the environment only — never hardcoded, never logged
(§11.4.10).

### Outputs

- Per-test captured-evidence JSON under the results dir (one file per request)
  plus a `.status` sidecar carrying the HTTP code.
- `summary.txt` (human-readable) and `summary.json` (machine-readable).
- `PASS`/`FAIL`/`SKIP` lines on stdout, each citing its evidence path or reason.
- Exit code `0` iff every non-`SKIP` test `PASS`ed; `1` otherwise.

## Edge cases

- **API unreachable** — the pre-flight `GET /health` probe detects a curl
  transport failure (`000`) and marks every test `SKIP`-with-reason (§11.4.3).
  The script exits `0` (honest SKIPs are not failures). This tolerates a
  conductor's in-progress backend rebuild.
- **Login yields no token** — the auth-gated tests `SKIP` with `no_token`; the
  unauthenticated `C2` test still runs (it needs no token) and asserts the 401.
- **Oversized limit (`M2`)** — `media/recent` clamps `limit > 200` to its
  default; the test asserts a well-formed `items+total` body and no `500`.
- **Empty download archive (`D1`)** — posts an empty `paths` array so the
  validation branch is exercised (expected `400 + error`) without producing a
  large artefact; a `200` is also accepted as the route being genuinely live.

## Internal behaviour

- `set -u` safe; parses clean under both `bash -n` and `sh -n` (§11.4.67) — no
  bash-only construct (`<(...)`, `[[ ]]`, arrays) outside an `eval`.
- `http_request` builds the `curl` argument vector with the POSIX positional
  parameters (`set -- ...`), writes the body to `<name>.json` and the status to
  `<name>.status`, and echoes the HTTP code (`000` on transport failure).
- `ab_pass_with_evidence` refuses to PASS unless the cited evidence file exists
  and is non-empty (§11.4.69).
- The `C3` collections cycle creates a uniquely-named collection
  (`qa-cds-<ts>-<pid>`) and deletes it on the same run (self-cleaning, §11.4.14).

## Anti-bluff posture (§11.4 / §11.4.27 / §11.4.69)

Every PASS asserts a **real** body field (e.g. the created collection name
round-tripping through `GET /:id`, `success`+`data` on stats, the `401`
Authorization-required body on the unauth probe), never a tautology, and cites a
captured-evidence file. An unreachable API SKIPs with reason; a genuine product
defect surfaces as `FAIL` with its captured response — it is never masked.

## Related scripts

- `scripts/testing/full_automation/catalog_functional_matrix.sh` — the
  house-style reference this suite mirrors.
- `scripts/testing/full_automation/catalog_auth_pagination.sh`,
  `catalog_browse_filter_search.sh`,
  `catalog_favorites_resume_lifecycle.sh` — sibling suites covering the
  complementary surfaces.
- `submodules/helix_qa/banks/catalog_collections_downloads_stats.yaml` — the
  HelixQA challenge bank (`CDS-API-001..012`) that dispatches to this script.

## Last verified

- **2026-06-25** — run live against `http://127.0.0.1:8080` (catalog-api `dev`):
  **14 PASS / 0 FAIL / 0 SKIP**. `bash -n` and `sh -n` both clean.

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
