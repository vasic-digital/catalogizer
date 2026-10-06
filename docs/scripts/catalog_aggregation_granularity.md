# catalog_aggregation_granularity.sh — User Guide

**Revision:** 1
**Last modified:** 2026-06-25T15:18:00Z

Companion guide (Helix Constitution §11.4.18) for
`scripts/testing/full_automation/catalog_aggregation_granularity.sh`.

## Overview

A standing regression guard (§11.4.135) and full-automation API test
(§11.4.27 / §11.4.98) for **DEFECT-E** — title-granular SMB-scan aggregation.

**DEFECT-E (FACT):** the SMB scan produced only 3 directory-level entities
("movies" / "music" / "software") instead of one entity *per title*.
**Fix:** title-granular leaf walk in
`catalog-api/internal/services/aggregation_service.go` (commit `e748bba5`).
After the fix the populated DB exposes **15 per-title entities** through the
catalog-api.

This suite drives the LIVE catalog-api over real HTTP and asserts the
per-title aggregation is genuinely present, capturing the real HTTP response
body as evidence for every assertion (anti-bluff §11.4 / §11.4.69 — never a
tautology). It also runs the **negative** assertion that the bug's
folder-level signature is absent.

It is an extend-to-all-cases pass (§11.4.146): every movie title, every level
of the Breaking Bad tv tree (show → season → episode), the music album, and a
full negative sweep of all entities.

## Prerequisites

- A **running** catalog-api on `http://127.0.0.1:8080` (override with
  `CATALOGIZER_BASE_URL`), backed by the populated DB
  (`/Volumes/T7/Projects/catalogizer/data/catalogizer.db`).
- `bash`, `curl`, `python3` on `PATH`. `python3` is the JSON oracle; if it is
  absent the suite SKIPs-with-reason rather than emit a tautological PASS.
- Admin credentials. The suite sources an `.api-env` file
  (`ADMIN_USERNAME` / `ADMIN_PASSWORD`, optionally `QA_TOKEN`) and **never
  echoes any secret** (§11.4.10). By default it picks the most recent
  `qa-results/catalogizer-qa-*/.api-env`; override with
  `CATALOGIZER_ENV_FILE`. A `CATALOGIZER_TOKEN` env var (or `QA_TOKEN` from the
  env file) short-circuits the login.

## Usage examples

```bash
# Default: discover the newest .api-env, hit http://127.0.0.1:8080
./scripts/testing/full_automation/catalog_aggregation_granularity.sh

# Explicit base URL + env file
CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
CATALOGIZER_ENV_FILE=qa-results/catalogizer-qa-20260625T102312Z/.api-env \
  ./scripts/testing/full_automation/catalog_aggregation_granularity.sh

# Pre-acquired token (skips login)
CATALOGIZER_TOKEN="$(cat /tmp/.qa_admin_tok)" \
  ./scripts/testing/full_automation/catalog_aggregation_granularity.sh
```

## Inputs (env vars, all optional)

| Var | Default | Meaning |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:8080` | API base URL |
| `CATALOGIZER_ENV_FILE` | newest `qa-results/catalogizer-qa-*/.api-env` | credential single-source (§11.4.10) |
| `CATALOGIZER_USER` | `$ADMIN_USERNAME` or `admin` | login username |
| `CATALOGIZER_PASS` | `$ADMIN_PASSWORD` | login password |
| `CATALOGIZER_TOKEN` | `$QA_TOKEN` (from env file) else real login | pre-acquired session token |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/aggregation_granularity/<ts>` | evidence output dir |

## Outputs

- Per-assertion captured-evidence JSON under the results dir (the actual HTTP
  response body — e.g. `agg_a_browse_movie.json`, `agg_b2_seasons.json`).
- `summary.txt` (PASS/FAIL/SKIP rows + counts) and `summary.json`
  (machine-readable).
- PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
- **Exit code 0** iff every non-SKIP assertion PASSed; **1** otherwise.

## Assertions

| ID | Endpoint(s) | Asserts |
|---|---|---|
| AGG-A | `GET /api/v1/entities/browse/movie` | distinct per-title entities **Inception**, **Interstellar**, **The Matrix** (not a collapsed "movies" folder) |
| AGG-A2 | `GET /api/v1/entities/{id}` | each of those titles resolves to detail `media_type == "movie"` |
| AGG-B | `GET /api/v1/entities/browse/tv_show` | a browsable tv_show **Breaking Bad** |
| AGG-B2 | `GET /api/v1/entities/{bb}/children` | children **Season 1** + **Season 2**, `media_type == "tv_season"` |
| AGG-B3 | `GET /api/v1/entities/{season}/children` | tv_episode descendants (`media_type == "tv_episode"`) |
| AGG-C | `GET /api/v1/entities/browse/music_album` + detail | a music_album **The Dark Side of the Moon** |
| AGG-D | `GET /api/v1/entities?limit=500` | **NEGATIVE** — no entity titled `movies`/`music`/`series`/`software`/`tv`/`shows` (the bug's signature) |

## Edge cases

- **API unreachable** → every assertion SKIPs-with-reason (§11.4.3); never a
  fabricated PASS.
- **No token** (login failed and none supplied) → auth-dependent assertions
  SKIP-with-reason.
- **`python3` absent** → all assertions SKIP (no tautology oracle).
- The suite is **read-only** (GETs only): it creates no server-side state and
  is safe to re-run any number of times (§11.4.50, §11.4.98 re-runnable).

## Internal behaviour

`http_get` performs each request, writing the body to `<name>.json` and the
HTTP status to `<name>.status`. JSON parsing is done by inline `python3`
(`json_titles`, `json_field`, `id_for_title`). A PASS is only emitted via
`ab_pass_with_evidence`, which refuses to pass on a missing/empty evidence
file. The list endpoints expose `media_type_id` only; the per-entity detail
endpoint (`/api/v1/entities/{id}`) resolves the `media_type` string asserted by
AGG-A2/B2/B3/C.

## Related scripts

- `scripts/testing/full_automation/catalog_functional_matrix.sh` — the
  functional-matrix pattern this guard is modelled on.
- `submodules/helix_qa/banks/catalog_aggregation_granularity.yaml` — the
  HelixQA challenge bank that scores these assertions.

## Cross-references

- The code under guard: `catalog-api/internal/services/aggregation_service.go`
  (title-granular leaf walk, commit `e748bba5`).
- Constitution: §11.4.18 (script docs), §11.4.27 (real system), §11.4.69
  (captured evidence), §11.4.135 (standing regression guard), §11.4.146
  (extend-to-all-cases), §11.4.10 (credentials), §11.4.3 (topology SKIP),
  §11.4.67 (target-shell parseability).

**Last verified:** 2026-06-25 (live run against `http://127.0.0.1:8080`,
PASS=7 FAIL=0 SKIP=0).

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
