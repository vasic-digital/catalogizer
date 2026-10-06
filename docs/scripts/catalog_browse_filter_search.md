# catalog_browse_filter_search.sh

**Revision:** 1
**Last modified:** 2026-06-25T13:00:00Z

Companion guide for `scripts/testing/full_automation/catalog_browse_filter_search.sh`
(per Helix Constitution §11.4.18 — every script ships an in-source doc block AND
this external user guide).

## Overview

Full-automation REST API **browse / filter / search** coverage suite for the
Catalogizer `catalog-api` (Go/Gin) backend. It drives the **live** HTTP API with
real `curl` requests and asserts, for every test, BOTH the HTTP status AND a real
observable body field — never a tautology. Each test writes a captured-evidence
JSON file (the actual HTTP response) and prints `PASS`/`FAIL` with the evidence
path, in the anti-bluff `ab_pass_with_evidence` style of §11.4.69. An unreachable
API yields honest `SKIP`-with-reason (§11.4.3), never a fabricated PASS.

This suite **deepens** the browse/discovery surface that
`catalog_functional_matrix.sh` only touches lightly: entity filtering by query
params, search WITH results AND the empty-query edge case, the media-item DETAIL
fetch, and the `media/popular` discovery feed.

### Coverage

| Test | Endpoint | Real assertion |
|---|---|---|
| B1 entities default | `GET /api/v1/entities` (auth) | 200 + `items[]` + `total` |
| B2 entities limit | `GET /api/v1/entities?limit=1` (auth) | 200 + `limit=1` honoured (≤1 item with `jq`) |
| B3 entities type filter | `GET /api/v1/entities?media_type=…` (auth) | 200 + `items[]` + `total` |
| S1 search results | `GET /api/v1/search?q=test` (auth) | 200 + `results` array |
| S2 search empty query | `GET /api/v1/search?q=` (auth) | empty `q` handled (200+`results` OR 400) — never 5xx/000 |
| D1 media detail | `GET /api/v1/media/:id` (auth) | 200 + detail body (`id`/`title`/…) OR honest 404 SKIP |
| P1 media popular | `GET /api/v1/media/popular` (auth) | 200 + list-shaped body |

## Prerequisites

- `bash` and `curl` on `PATH`.
- `jq` is **optional** — the script falls back to `grep`/`sed` field extraction
  when `jq` is absent. Note: the B2 limit-length assertion and the strictest S2
  checks are richer when `jq` is present.
- A reachable `catalog-api` instance. By default the suite targets
  `http://127.0.0.1:18080` so it does NOT collide with a shared instance on
  `:8080`.

## Inputs (environment variables)

| Variable | Default | Meaning |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:18080` | API base URL |
| `CATALOGIZER_USER` | `admin` | login username |
| `CATALOGIZER_PASS` | `admin` | login password |
| `CATALOGIZER_TOKEN` | _(unset)_ | pre-acquired token; if set, login acquisition is skipped |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/browse_filter_search/<ts>` | evidence output directory |
| `CATALOGIZER_MEDIA_ID` | `1` | `media id` used by the D1 detail fetch |
| `CATALOGIZER_MEDIA_TYPE` | `movie` | `media_type` filter used by B3 |
| `CATALOGIZER_SEARCH_QUERY` | `test` | search term used by S1 |

Credentials are read from environment only — never hardcoded (§11.4.10).

## Outputs

- One captured-evidence JSON file per HTTP request under the results dir
  (e.g. `b1_entities_default.json`, `s2_search_empty.json`, `p1_media_popular.json`).
- `summary.txt` (human-readable PASS/FAIL/SKIP rows) and `summary.json`
  (machine-readable counts).
- `PASS`/`FAIL`/`SKIP` lines on stdout, each citing its evidence path.
- Exit code `0` iff every non-SKIP test PASSed; `1` if any test FAILed.

## Usage examples

```bash
# Default target (127.0.0.1:18080), default admin/admin
./scripts/testing/full_automation/catalog_browse_filter_search.sh

# Explicit target + credentials + results dir
CATALOGIZER_BASE_URL=http://127.0.0.1:18080 \
CATALOGIZER_USER=admin CATALOGIZER_PASS="$YOUR_QA_PASSWORD" \
CATALOGIZER_RESULTS_DIR=qa-results/browse_filter_search/my_run \
./scripts/testing/full_automation/catalog_browse_filter_search.sh

# Reuse a pre-acquired token (skips login acquisition)
CATALOGIZER_TOKEN="<token>" \
./scripts/testing/full_automation/catalog_browse_filter_search.sh

# Filter entities by a different media_type and search a different term
CATALOGIZER_MEDIA_TYPE=tv_show CATALOGIZER_SEARCH_QUERY=matrix \
./scripts/testing/full_automation/catalog_browse_filter_search.sh
```

## Edge cases & behaviour

- **API unreachable** → the pre-flight `GET /health` probe detects a curl
  transport failure and the suite marks every test `SKIP`-with-reason
  (§11.4.3). It exits `0` (honest SKIPs are not failures), never a PASS.
- **No token** (login failed / not supplied) → all tests SKIP.
- **B2 limit honoured** → with `jq`, the suite asserts the returned `items`
  array length is ≤ 1; without `jq` it falls back to an honest weaker check
  (presence of `items` + `total`) rather than a fabricated PASS.
- **S2 empty query** → an empty `q` is a real edge case. The API MUST respond
  deterministically: `200` with a (possibly empty) `results` array, or `400`.
  A `5xx` or transport failure (`000`) is a genuine defect and FAILs — it is
  never silently treated as a pass.
- **D1 media detail 404** → treated as an honest data-precondition SKIP (the
  referenced media id does not exist in this DB), not a PASS. Seed a row and set
  `CATALOGIZER_MEDIA_ID` to run it for real.
- This suite is **read-only** — it creates and mutates no state.

## Internal behaviour

- `set -u`; `sh -n` and `bash -n` clean (§11.4.67).
- `http_request` writes `<name>.json` (body), `<name>.status` (HTTP code), and
  `<name>.curlerr` (stderr) for each call; a transport failure records `000`.
- `ab_pass_with_evidence` refuses to declare PASS unless the evidence file
  exists and is non-empty.
- The login response field is `session_token` (NOT `token`/`access_token`).

## Related scripts / artefacts

- `submodules/helix_qa/banks/catalog_browse_favorites_resume.yaml` — the HelixQA
  challenge bank mirroring this suite (`BFR-API-*` challenges; scores PASS only
  on positive captured evidence).
- `scripts/testing/full_automation/catalog_favorites_resume_lifecycle.sh` —
  sibling lifecycle suite (favorites + resume state deltas).
- `scripts/testing/full_automation/catalog_functional_matrix.sh` — the broad
  functional matrix this suite complements.
- `catalog-api/main.go` — route registration (source of truth for endpoints).

## Last verified

2026-06-25 — `bash -n` and `sh -n` clean. Designed to run against a live
`catalog-api` on `127.0.0.1:18080`; against an unreachable instance it correctly
reports all-SKIP with reason, never a fabricated PASS.

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
