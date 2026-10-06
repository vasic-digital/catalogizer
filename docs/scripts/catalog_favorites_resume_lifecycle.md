# catalog_favorites_resume_lifecycle.sh

**Revision:** 1
**Last modified:** 2026-06-25T13:00:00Z

Companion guide for `scripts/testing/full_automation/catalog_favorites_resume_lifecycle.sh`
(per Helix Constitution §11.4.18 — every script ships an in-source doc block AND
this external user guide).

## Overview

Full-automation REST API **lifecycle** coverage suite for the Catalogizer
`catalog-api` (Go/Gin) backend. Where `catalog_functional_matrix.sh` touches
favorites and playback once, this suite drives both end-to-end with
**state-delta assertions**:

- A favorite that is **added** MUST then be observably **PRESENT** in the
  listing, and observably **ABSENT** after removal.
- A playback position written to `N` MUST be observably **read back as exactly
  `N`** — the "remember where you left off" feature.

This is the strongest anti-bluff posture for these flows: the favorites test
FAILs if the add does not show up in the list or the remove leaves it behind;
the resume test FAILs if the read-back position is not exactly the value that
was written (§11.4 / §11.4.69). Each step writes a captured-evidence JSON file
and an unreachable API yields honest `SKIP`-with-reason (§11.4.3), never a
fabricated PASS.

### Coverage

| Test | Endpoint | Real assertion (observable delta) |
|---|---|---|
| F1 fav add | `POST /api/v1/favorites` | 200/409 write path exercised (404 → honest SKIP) |
| F2 fav list present | `GET /api/v1/favorites` | listing **contains** the added `entity_id` |
| F3 fav remove | `DELETE /api/v1/favorites/:type/:id` | 200/204 deletion |
| F4 fav list absent | `GET /api/v1/favorites` | added `entity_id` **no longer present** |
| R1 resume start | `POST /api/v1/playback/sessions/start` | 200 + `session_id` |
| R2 resume progress | `POST /api/v1/playback/sessions/progress` | 200, position advanced to `N` |
| R3 resume end | `POST /api/v1/playback/sessions/end` | 200, ended at `N`, `completed:false` |
| R4 resume read | `GET /api/v1/entities/:id/progress` | `last_position` read-back **== `N`** |

## Prerequisites

- `bash` and `curl` on `PATH`.
- `jq` is **optional** but recommended — with `jq` the R4 read-back asserts the
  exact `progress.last_position` value; without `jq` it falls back to a positive
  `"last_position": N` / `"position": N` match in the captured body (never a bare
  number grep that could match unrelated data).
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
| `CATALOGIZER_RESULTS_DIR` | `qa-results/favorites_resume_lifecycle/<ts>` | evidence output directory |
| `CATALOGIZER_FAV_ENTITY_ID` | `1` | `entity_id` used by the favorites cycle |
| `CATALOGIZER_FAV_ENTITY_TYPE` | `movie` | `entity_type` used by the favorites cycle |
| `CATALOGIZER_MEDIA_ID` | `1` | `media_item_id` used by the resume cycle |
| `CATALOGIZER_RESUME_POS` | `137` | resume position (seconds) written and read back |

Credentials are read from environment only — never hardcoded (§11.4.10).

## Outputs

- One captured-evidence JSON file per HTTP request under the results dir
  (e.g. `f1_fav_add.json`, `f2_fav_list_present.json`, `f4_fav_list_absent.json`,
  `r4_resume_read.json`).
- `summary.txt` (human-readable PASS/FAIL/SKIP rows) and `summary.json`
  (machine-readable counts).
- `PASS`/`FAIL`/`SKIP` lines on stdout, each citing its evidence path.
- Exit code `0` iff every non-SKIP test PASSed; `1` if any test FAILed.

## Usage examples

```bash
# Default target (127.0.0.1:18080), default admin/admin
./scripts/testing/full_automation/catalog_favorites_resume_lifecycle.sh

# Explicit target + credentials + results dir
CATALOGIZER_BASE_URL=http://127.0.0.1:18080 \
CATALOGIZER_USER=admin CATALOGIZER_PASS="$YOUR_QA_PASSWORD" \
CATALOGIZER_RESULTS_DIR=qa-results/favorites_resume_lifecycle/my_run \
./scripts/testing/full_automation/catalog_favorites_resume_lifecycle.sh

# Drive a specific favorite entity and a specific resume media + position
CATALOGIZER_FAV_ENTITY_ID=5 CATALOGIZER_FAV_ENTITY_TYPE=tv_show \
CATALOGIZER_MEDIA_ID=5 CATALOGIZER_RESUME_POS=420 \
./scripts/testing/full_automation/catalog_favorites_resume_lifecycle.sh
```

### Seeding a media row for the resume cycle

The `media_item_id` referenced by R1 has a foreign key to `media_items`. On an
empty DB, seed one row so the resume lifecycle runs for real instead of SKIPping:

```bash
sqlite3 "$WORK/catalog.db" \
  "INSERT INTO media_items (media_type_id, title, year, status)
   VALUES (1, 'QA Resume Fixture Movie', 2026, 'detected');"
# then run with CATALOGIZER_MEDIA_ID=<new id>
```

## Edge cases & behaviour

- **API unreachable** → pre-flight `GET /health` probe detects the transport
  failure; every test is `SKIP`-with-reason (§11.4.3); exit `0`, never a PASS.
- **No token** → all tests SKIP.
- **F1 favorites add returns 404** → honest data-precondition SKIP (the
  referenced `entity_id` does not exist); the whole favorites lifecycle (F1–F4)
  SKIPs as `depends_on`. A re-run returning `409` (already-in-favorites) still
  counts the write path as exercised.
- **R1 playback start returns 404/500** → SKIP with a `media_precondition`
  reason; the resume lifecycle (R1–R4) SKIPs as `depends_on`.
- The favorites cycle is **self-cleaning** (it removes what it adds, §11.4.14),
  and F4 verifies the removal actually took effect.
- The resume cycle is **non-destructive** (it records a resume position only).

## Internal behaviour

- `set -u`; `sh -n` and `bash -n` clean (§11.4.67).
- `http_request` writes `<name>.json` (body), `<name>.status` (HTTP code), and
  `<name>.curlerr` (stderr) for each call; a transport failure records `000`.
- `ab_pass_with_evidence` refuses to declare PASS unless the evidence file
  exists and is non-empty.
- `fav_list_contains_id` matches `"entity_id": <id>` precisely (word-boundary),
  so F2/F4 cannot be fooled by an unrelated number elsewhere in the body.
- The login response field is `session_token` (NOT `token`/`access_token`).

## Related scripts / artefacts

- `submodules/helix_qa/banks/catalog_browse_favorites_resume.yaml` — the HelixQA
  challenge bank mirroring this suite (`BFR-API-*` challenges; scores PASS only
  on positive captured evidence).
- `scripts/testing/full_automation/catalog_browse_filter_search.sh` — sibling
  browse/filter/search suite.
- `scripts/testing/full_automation/catalog_functional_matrix.sh` — the broad
  functional matrix this suite complements.
- `catalog-api/main.go` — route registration (source of truth for endpoints).

## Last verified

2026-06-25 — `bash -n` and `sh -n` clean. Designed to run against a live
`catalog-api` on `127.0.0.1:18080` with a seeded `media_items` row; against an
unreachable instance it correctly reports all-SKIP with reason, never a
fabricated PASS.

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
