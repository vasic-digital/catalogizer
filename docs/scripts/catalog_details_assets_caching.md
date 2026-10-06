# catalog_details_assets_caching.sh — User Guide

**Revision:** 1
**Last modified:** 2026-06-25T00:00:00Z

Companion guide (Helix Constitution §11.4.18) for
`scripts/testing/full_automation/catalog_details_assets_caching.sh`.

## Overview

A standing regression guard (§11.4.135) and full-automation API test
(§11.4.27 / §11.4.98) for three catalog-api end-user surfaces:

- **DETAILS** — `GET /api/v1/entities/:id` (the `mediaEntityHandler.GetEntity`
  handler) returns a JSON entity detail body. The guard asserts the body an end
  user actually sees: a NON-EMPTY `title`, a numeric `id`, and a `media_type`
  field. A 404 or an empty-title detail is a real defect, not a PASS.
- **COVER / ASSET** — `GET /api/v1/cover/:id` (the `coverHandler.ServeCover`
  handler) streams the cover **image bytes** with an `image/*` content-type. Per
  §11.4.38 (installable-asset evidence) the guard opens the response and asserts
  the asset is **non-degenerate** — image content-type AND a byte size above a
  floor. A present-but-degenerate cover (0 bytes / 1×1 / non-image body) is a
  **FAIL**, never a PASS. The companion `GET /api/v1/cover/url/:id` (JSON
  `cover_url`) and `GET /api/v1/cover/placeholder/:type` (always-available
  `image/svg+xml` fallback) are also asserted.
- **CACHING** — the entities group is wrapped in `CacheHeaders(300)`
  (`middleware/cache_headers.go`), so a detail GET carries an `ETag` +
  `Cache-Control: public, max-age=300`. The guard asserts the ETag is **stable**
  across two reads (§11.4.50 determinism) and that a second request sending
  `If-None-Match:<ETag>` is **served from cache as 304 Not Modified** (the body
  is not re-sent). This is the directly-observable HTTP caching behaviour backed
  by the `media_metadata_cache` layer.

This suite drives the LIVE catalog-api over real HTTP and captures the real HTTP
response (JSON body, or — for binary cover/asset bytes — a `.meta` sidecar
describing the observed size + content-type) as evidence for every assertion
(anti-bluff §11.4 / §11.4.38 / §11.4.69 — never a tautology). It is an
extend-to-all-cases pass (§11.4.146): the detail fields, the asset byte/
content-type census, the cover-url JSON, the ETag stability + 304 short-circuit,
and the placeholder fallback.

## Prerequisites

- A **running** catalog-api on `http://127.0.0.1:8080` (override with
  `CATALOGIZER_BASE_URL`), backed by a populated DB containing at least one
  media entity.
- `bash`, `curl`, `python3` on `PATH`. `python3` is the JSON oracle; if it is
  absent the suite SKIPs-with-reason rather than emit a tautological PASS.
- Admin credentials **for the DETAILS + CACHING assertions only**
  (`/api/v1/entities/*` requires `Authorization: Bearer <session_token>`). The
  `/api/v1/cover/*` endpoints are **public** (no auth), so DAC-B / DAC-C / DAC-F
  run without a token. The suite sources an `.api-env` file (`ADMIN_USERNAME` /
  `ADMIN_PASSWORD`, optionally `QA_TOKEN`) and **never echoes any secret**
  (§11.4.10). There is **no hardcoded password fallback** — if no
  `ADMIN_PASSWORD`/`CATALOGIZER_PASS`/token is available, login fails and the
  auth-dependent assertions SKIP honestly. By default it picks the most recent
  `qa-results/catalogizer-qa-*/.api-env`; override with `CATALOGIZER_ENV_FILE`.
  A `CATALOGIZER_TOKEN` env var (or `QA_TOKEN` from the env file)
  short-circuits the login.

## Usage examples

```bash
# Default: discover the newest .api-env, hit http://127.0.0.1:8080
./scripts/testing/full_automation/catalog_details_assets_caching.sh

# Explicit base URL + env file
CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
CATALOGIZER_ENV_FILE=qa-results/catalogizer-qa-20260625T102312Z/.api-env \
  ./scripts/testing/full_automation/catalog_details_assets_caching.sh

# Pre-acquired token (skips login)
CATALOGIZER_TOKEN="$(cat /tmp/.qa_admin_tok)" \
  ./scripts/testing/full_automation/catalog_details_assets_caching.sh

# Drive the detail/cover surfaces from a tv_show subject instead of a movie
CATALOGIZER_ENTITY_TYPE=tv_show \
  ./scripts/testing/full_automation/catalog_details_assets_caching.sh
```

## Inputs (env vars, all optional)

| Var | Default | Meaning |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:8080` | API base URL |
| `CATALOGIZER_ENV_FILE` | newest `qa-results/catalogizer-qa-*/.api-env` | credential single-source (§11.4.10) |
| `CATALOGIZER_USER` | `$ADMIN_USERNAME` or `admin` | login username |
| `CATALOGIZER_PASS` | `$ADMIN_PASSWORD` (NO hardcoded default — §11.4.10) | login password |
| `CATALOGIZER_TOKEN` | `$QA_TOKEN` (from env file) else real login | pre-acquired session token |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/details_assets_caching/<ts>` | evidence output dir |
| `CATALOGIZER_ENTITY_TYPE` | `movie` (falls back to `tv_show`, then list-any) | entity type to browse for a detail subject |
| `CATALOGIZER_COVER_FLOOR` | `64` | non-degenerate cover-byte floor (§11.4.38) |

## Outputs

- Per-assertion captured-evidence files under the results dir: JSON response
  bodies (`dac_browse_<type>.json`, `dac_detail.json`, `dac_cover_url.json`),
  binary-asset `.meta` sidecars (`dac_cover.meta`, `dac_placeholder.meta` —
  observed `http_code`, `size_bytes`, `content_type`; the raw bytes are saved
  to `dac_cover.bin` / `dac_placeholder.bin` but the assertion cites the small
  `.meta`), and caching header dumps (`dac_etag_read1.hdr`,
  `dac_conditional_304.hdr`).
- `summary.txt` (PASS/FAIL/SKIP rows + counts) and `summary.json`
  (machine-readable).
- PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
- **Exit code 0** iff every non-SKIP assertion PASSed; **1** otherwise.

## Assertions

| ID | Endpoint(s) | Surface | Asserts |
|---|---|---|---|
| DAC-A | `GET /entities/browse/<type>` → `GET /entities/:id` | DETAILS | POSITIVE — detail body has a NON-EMPTY `title`, a numeric `id`, and a `media_type` field (the populated detail an end user sees) |
| DAC-B | `GET /cover/:id` | COVER/ASSET | POSITIVE + §11.4.38 — response is `image/*` content-type with byte size **above** the non-degenerate floor; a 0-byte / 1×1 / non-image body FAILs |
| DAC-C | `GET /cover/url/:id` | COVER/ASSET | POSITIVE — JSON carries a `cover_url` field for the same `media_item_id` |
| DAC-D | `GET /entities/:id` (×2) | CACHING | POSITIVE + §11.4.50 — `ETag` + `Cache-Control` headers present, and the ETag is **identical** across two independent reads of the unchanged entity |
| DAC-E | `GET /entities/:id` with `If-None-Match:<ETag>` | CACHING | NEGATIVE-of-200 — a conditional request matching the ETag returns **304 Not Modified** (served from cache, body not re-sent) |
| DAC-F | `GET /cover/placeholder/<type>` | COVER/ASSET | POSITIVE + §11.4.38 — the always-available placeholder is `image/svg+xml` with byte size above the floor |

## Edge cases

- **API unreachable** → every assertion SKIPs-with-reason (§11.4.3); never a
  fabricated PASS.
- **No token** (login failed and none supplied, including the no-hardcoded-
  password case) → the auth-dependent DETAILS + CACHING assertions
  (DAC-A / DAC-D / DAC-E) SKIP-with-reason; the public COVER assertions
  (DAC-B / DAC-C / DAC-F) still run.
- **No entity resolvable** from any browse/list endpoint → DAC-A..DAC-E
  SKIP-with-reason (no subject to drive); DAC-F (placeholder, subject-free) still
  runs.
- **`python3` absent** → all assertions SKIP (no tautology oracle).
- **Cover not yet generated** for the subject → `ServeCover` falls back to the
  placeholder SVG; this is still a non-degenerate `image/svg+xml` asset, so
  DAC-B PASSes on the fallback. A truly degenerate (0-byte / non-image) body
  would FAIL.
- The suite is **read-only** (GETs only): it creates no server-side state, runs
  no scans/clears/aggregations, and is safe to re-run any number of times
  (§11.4.50, §11.4.98 re-runnable). The conductor owns the live API + DB
  (§11.4.119); this suite never mutates it.

## Internal behaviour

`http_get` performs a JSON request, writing the body to `<name>.json` and the
HTTP status to `<name>.status`. `http_get_binary` performs an asset request,
capturing `%{http_code}|%{size_download}|%{content_type}` and writing a
`<name>.meta` sidecar (the captured-evidence record for §11.4.38). `http_get_header`
dumps response headers to `<name>.hdr` and extracts a named header (and can send
an `If-None-Match` for the conditional 304 probe). JSON parsing is done by inline
`python3` (`json_titles`, `first_entity_id`, `json_field`, `json_has_key`). A
detail subject is resolved by browsing the requested type (falling back to
`tv_show`, then the unfiltered entities list) and taking the first item with a
non-empty title. A PASS is only emitted via `ab_pass_with_evidence`, which
refuses to pass on a missing/empty evidence file.

## Related scripts

- `scripts/testing/full_automation/catalog_episode_titles_dedup.sh` — the
  DEFECT-H + DEFECT-I episode-titles/dedup guard this suite is modelled on (same
  helpers, same `.api-env` credential source, same captured-evidence layout).
- `submodules/helix_qa/banks/catalog_details_assets_caching.yaml` — the HelixQA
  challenge bank that scores these assertions.

## Cross-references

- Constitution: §11.4.18 (script docs), §11.4.27 (real system), §11.4.38
  (installable-asset non-degenerate evidence), §11.4.69 (captured evidence),
  §11.4.135 (standing regression guard), §11.4.146 (extend-to-all-cases),
  §11.4.50 (determinism — ETag stability), §11.4.10 (credentials), §11.4.3
  (topology SKIP), §11.4.14 (cleanup), §11.4.67 (target-shell parseability),
  §11.4.119 (conductor owns the live resource — read-only probes only).

**Last verified:** 2026-06-25 (authored; `sh -n` + `bash -n` clean. Read-only
live probes against `http://127.0.0.1:8080` confirmed: `/health` → 200;
`/api/v1/cover/placeholder/movie` → 200 `image/svg+xml` 1398 B (DAC-F live-PASS,
> 64 B floor); `/api/v1/cover/1` → 200 `image/svg+xml` 1398 B (DAC-B live-PASS on
placeholder fallback); `/api/v1/cover/url/1` → 200 `application/json` (DAC-C
live-PASS); `/api/v1/entities/browse/movie` without auth → 401 (DAC-A/D/E need
the conductor's `.api-env` token + a real entity, which the conductor supplies
when it runs the suite live).)

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
