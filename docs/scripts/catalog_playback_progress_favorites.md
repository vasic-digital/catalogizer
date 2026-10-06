# catalog_playback_progress_favorites.sh — User Guide

**Revision:** 1
**Last modified:** 2026-06-25T00:00:00Z

Companion guide (Helix Constitution §11.4.18) for
`scripts/testing/full_automation/catalog_playback_progress_favorites.sh`.

## Overview

A standing regression guard (§11.4.135) and full-automation API test
(§11.4.27 / §11.4.98) proving the catalog-api **playback-progress /
resume-where-left-off** and **favorites** endpoints really work end-to-end for
an end user — the §11.4 / §11.4.69 anti-bluff pattern: set a real value, read it
back, assert it is observably equal; add a thing, see it appear; remove it, see
it gone.

- **Resume-where-left-off:** the playback-session lifecycle
  (`POST /playback/sessions/start` → `/progress` → `/end`) upserts a
  `media_progress` row whose `last_position` is exactly where the user stopped.
  A client reads `GET /entities/{id}/progress` and resumes there. This suite
  writes a real session and asserts the read-back `progress.last_position`
  equals the `end_position` it wrote.
- **Favorites:** `POST /favorites {entity_id, entity_type}` adds a favorite;
  `GET /favorites` lists them; `GET /favorites/check/{type}/{id}` reports
  membership; `DELETE /favorites/{type}/{id}` removes one. This suite adds a
  favorite, asserts it appears (list + check), removes it, and asserts it is
  gone (list + check).

This suite drives the LIVE catalog-api over real HTTP and captures the real HTTP
response body as evidence for every assertion (anti-bluff §11.4 / §11.4.69 —
never a tautology). It is an extend-to-all-cases pass (§11.4.146) over the
set/get/list/check/remove surfaces, with a determinism re-read (§11.4.50).

It is **self-cleaning** (§11.4.14): every WRITE it makes it UNDOES on exit — the
favorite it adds it removes; the playback progress it writes it resets to the
pristine "not started" baseline (`last_position = 0`) by ending one final
session at position 0 (the API exposes no DELETE for `media_progress`, so a
reset-to-baseline is the honest quiescent state — documented, not faked). The
conductor OWNS the live DB (§11.4.119); this suite NEVER triggers scans, clears,
or aggregation — it touches ONLY its own per-user
`playback_sessions` / `media_progress` / `favorites` rows for one entity.

## Prerequisites

- A **running** catalog-api on `http://127.0.0.1:8080` (override with
  `CATALOGIZER_BASE_URL`), backed by the populated DB containing at least one
  browsable entity of `CATALOGIZER_BROWSE_TYPE` (default `movie`).
- `bash`, `curl`, `python3` on `PATH`. `python3` is the JSON oracle; if it is
  absent the suite SKIPs-with-reason rather than emit a tautological PASS.
- Admin credentials. The suite sources an `.api-env` file
  (`ADMIN_USERNAME` / `ADMIN_PASSWORD`, optionally `QA_TOKEN`) and **never
  echoes any secret** (§11.4.10). There is **no hardcoded password fallback**
  — if no `ADMIN_PASSWORD`/`CATALOGIZER_PASS`/token is available, login fails
  and the suite SKIPs honestly. By default it picks the most recent
  `qa-results/catalogizer-qa-*/.api-env`; override with `CATALOGIZER_ENV_FILE`.
  A `CATALOGIZER_TOKEN` env var (or `QA_TOKEN` from the env file)
  short-circuits the login.

## Usage examples

```bash
# Default: discover the newest .api-env, hit http://127.0.0.1:8080
./scripts/testing/full_automation/catalog_playback_progress_favorites.sh

# Explicit base URL + env file
CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
CATALOGIZER_ENV_FILE=qa-results/catalogizer-qa-20260625T102312Z/.api-env \
  ./scripts/testing/full_automation/catalog_playback_progress_favorites.sh

# Pre-acquired token (skips login)
CATALOGIZER_TOKEN="$(cat /tmp/.qa_admin_tok)" \
  ./scripts/testing/full_automation/catalog_playback_progress_favorites.sh

# Pin a specific fixture entity + resume position
CATALOGIZER_ENTITY_ID=19 CATALOGIZER_BROWSE_TYPE=movie CATALOGIZER_RESUME_POS=900 \
  ./scripts/testing/full_automation/catalog_playback_progress_favorites.sh
```

## Inputs (env vars, all optional)

| Var | Default | Meaning |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:8080` | API base URL |
| `CATALOGIZER_ENV_FILE` | newest `qa-results/catalogizer-qa-*/.api-env` | credential single-source (§11.4.10) |
| `CATALOGIZER_USER` | `$ADMIN_USERNAME` or `admin` | login username |
| `CATALOGIZER_PASS` | `$ADMIN_PASSWORD` (NO hardcoded default — §11.4.10) | login password |
| `CATALOGIZER_TOKEN` | `$QA_TOKEN` (from env file) else real login | pre-acquired session token |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/playback_progress_favorites/<ts>` | evidence output dir |
| `CATALOGIZER_BROWSE_TYPE` | `movie` | entity type to source a fixture from (favorites closed set) |
| `CATALOGIZER_ENTITY_ID` | (discovered via browse) | explicit fixture entity id |
| `CATALOGIZER_RESUME_POS` | `873` | `end_position` (seconds) to write + read back |

## Outputs

- Per-assertion captured-evidence JSON under the results dir (the actual HTTP
  response body — e.g. `ppf_browse.json`, `ppf_entity_detail.json`,
  `ppf_a_start.json`, `ppf_a_end.json`, `ppf_a_progress_read.json`,
  `ppf_b_add.json`, `ppf_b_list.json`, `ppf_b_check.json`, `ppf_c_remove.json`,
  `ppf_c_list.json`, `ppf_c_check.json`, `ppf_d_read1.json`, `ppf_d_read2.json`)
  plus a `_cleanup.log` recording the undo of every write.
- `summary.txt` (PASS/FAIL/SKIP rows + counts) and `summary.json`
  (machine-readable, including the resolved `entity_id` / `entity_type` /
  `resume_pos`).
- PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
- **Exit code 0** iff every non-SKIP assertion PASSed; **1** otherwise.

## Assertions

| ID | Endpoint(s) | Asserts |
|---|---|---|
| PPF-A | `POST /playback/sessions/start` → `/progress` → `/end`, then `GET /entities/{id}/progress` | **resume-where-left-off** / POSITIVE — the rolled-up `progress.last_position` equals the `end_position` written, so a client can resume exactly where the user stopped |
| PPF-B | `POST /favorites` then `GET /favorites` + `GET /favorites/check/{type}/{id}` | **favorite add→present** / POSITIVE — the just-added `(entity_type, entity_id)` pair appears in the list AND `is_favorite` is `true` |
| PPF-C | `DELETE /favorites/{type}/{id}` then `GET /favorites` + `GET /favorites/check/{type}/{id}` | **favorite remove→gone** / NEGATIVE — the pair is absent from the list AND `is_favorite` is `false` |
| PPF-D | second + third `GET /entities/{id}/progress` | determinism (§11.4.50) — the observed `last_position` is identical across two independent reads |

## Edge cases

- **API unreachable** → every assertion SKIPs-with-reason (§11.4.3); never a
  fabricated PASS.
- **No token** (login failed and none supplied, including the no-hardcoded-
  password case) → all auth-dependent assertions SKIP-with-reason.
- **`python3` absent** → all assertions SKIP (no tautology oracle).
- **No fixture entity** (browse returns no item of `CATALOGIZER_BROWSE_TYPE`,
  and no `CATALOGIZER_ENTITY_ID` supplied) → all assertions SKIP-with-reason.
- **Favorite already present from a prior interrupted run** — `POST /favorites`
  returns `409`; the suite treats both `200` and `409` as "now present", owns
  the row, and removes it in PPF-C / the cleanup trap.
- The suite **undoes every write on exit** (favorite removed; progress reset to
  baseline `0`), so it is safe to re-run any number of times (§11.4.50,
  §11.4.98 re-runnable). The progress reset is to baseline rather than a true
  delete because the API exposes no `media_progress` DELETE — disclosed
  honestly, never faked.

## Internal behaviour

`http_get` performs each GET, writing the body to `<name>.json` and the HTTP
status to `<name>.status`; `http_body` does the same for the authenticated
`POST`/`DELETE` writes. JSON parsing is done by inline `python3` (`json_field`,
`first_browse_id`, `fav_pair_present`). A real fixture entity is resolved once
(`GET /entities/browse/{type}` → first `id`; `GET /entities/{id}` → `media_type`
for the favorite `entity_type`); the entity `id` is the key for BOTH the
favorite `entity_id` and the playback `media_item_id`. PPF-A writes a real
start→progress→end at `CATALOGIZER_RESUME_POS` and reads the rolled-up
`progress.last_position` back. PPF-B/PPF-C add and remove a favorite and assert
presence/absence in BOTH the list and the check endpoint. PPF-D re-reads the
progress twice and compares. A PASS is only emitted via
`ab_pass_with_evidence`, which refuses to pass on a missing/empty evidence file.
The EXIT/INT/TERM trap removes the favorite and resets the progress.

## Related scripts

- `scripts/testing/full_automation/catalog_episode_titles_dedup.sh` — the
  DEFECT-H/DEFECT-I episode-titles guard this suite is modelled on (same
  helpers, same `.api-env` credential source, same captured-evidence layout).
- `submodules/helix_qa/banks/catalog_playback_progress_favorites.yaml` — the
  HelixQA challenge bank that scores these assertions.

## Cross-references

- Constitution: §11.4.18 (script docs), §11.4.27 (real system), §11.4.69
  (captured evidence), §11.4.135 (standing regression guard), §11.4.146
  (extend-to-all-cases), §11.4.50 (determinism), §11.4.10 (credentials),
  §11.4.3 (topology SKIP), §11.4.14 (cleanup / undo every write), §11.4.119
  (single-resource-owner — read-mostly, only own rows touched), §11.4.67
  (target-shell parseability).
- Endpoints under guard: `catalog-api/handlers/playback_handler.go`
  (`StartSession` / `ProgressSession` / `EndSession` / `GetProgressForEntity`)
  and `catalog-api/handlers/service_handlers.go`
  (`AddFavorite` / `ListFavorites` / `RemoveFavorite` / `CheckFavorite`),
  registered under `/api/v1` in `catalog-api/main.go`.

**Last verified:** 2026-06-25 (authored; `sh -n` + `bash -n` clean; live
read-only probe confirmed every endpoint's shape — `GET /entities/19` →
`{id,title,media_type:"movie"}`, `GET /entities/19/progress` →
`{"progress":null}` on first load, `GET /favorites/check/movie/19` →
`{"is_favorite":false}`. The conductor runs the full write/read/undo cycle live
under §11.4.119 single-resource ownership.)

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
