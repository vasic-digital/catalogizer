# catalog_episode_titles_dedup.sh — User Guide

**Revision:** 1
**Last modified:** 2026-06-25T00:00:00Z

Companion guide (Helix Constitution §11.4.18) for
`scripts/testing/full_automation/catalog_episode_titles_dedup.sh`.

## Overview

A standing regression guard (§11.4.135) and full-automation API test
(§11.4.27 / §11.4.98) for **DEFECT-H** and **DEFECT-I** in the TV-episode
aggregation path.

- **DEFECT-H (no duplicate episodes):** the scan emitted two (or more) episode
  entities sharing the SAME `episode_number` under the SAME season. The fix
  de-duplicates by `(season, episode_number)` so each
  `(season_id, episode_number)` pair is unique.
- **DEFECT-I (real human episode titles):** episodes whose source filename
  carries a human title (e.g. `Breaking Bad S01E01.Pilot`) were titled
  generically (`"Episode 1"`) instead of `"Pilot"`. The fix carries the human
  title from the filename into the entity `title`.

This suite drives the LIVE catalog-api over real HTTP and asserts the fixed
behaviour, capturing the real HTTP response body as evidence for every
assertion (anti-bluff §11.4 / §11.4.69 — never a tautology). It is an
extend-to-all-cases pass (§11.4.146): every season, every episode, the
per-episode title check, the duplicate-pair sweep, and the determinism
re-read.

## Prerequisites

- A **running** catalog-api on `http://127.0.0.1:8080` (override with
  `CATALOGIZER_BASE_URL`), backed by the populated DB containing the
  `Breaking Bad` fixture (S01E01 "Pilot" → human title `"Pilot"`).
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
./scripts/testing/full_automation/catalog_episode_titles_dedup.sh

# Explicit base URL + env file
CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
CATALOGIZER_ENV_FILE=qa-results/catalogizer-qa-20260625T102312Z/.api-env \
  ./scripts/testing/full_automation/catalog_episode_titles_dedup.sh

# Pre-acquired token (skips login)
CATALOGIZER_TOKEN="$(cat /tmp/.qa_admin_tok)" \
  ./scripts/testing/full_automation/catalog_episode_titles_dedup.sh

# Different show under guard
CATALOGIZER_SHOW_TITLE="Breaking Bad" \
  ./scripts/testing/full_automation/catalog_episode_titles_dedup.sh
```

## Inputs (env vars, all optional)

| Var | Default | Meaning |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:8080` | API base URL |
| `CATALOGIZER_ENV_FILE` | newest `qa-results/catalogizer-qa-*/.api-env` | credential single-source (§11.4.10) |
| `CATALOGIZER_USER` | `$ADMIN_USERNAME` or `admin` | login username |
| `CATALOGIZER_PASS` | `$ADMIN_PASSWORD` (NO hardcoded default — §11.4.10) | login password |
| `CATALOGIZER_TOKEN` | `$QA_TOKEN` (from env file) else real login | pre-acquired session token |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/episode_titles_dedup/<ts>` | evidence output dir |
| `CATALOGIZER_SHOW_TITLE` | `Breaking Bad` | TV show under guard |

## Outputs

- Per-assertion captured-evidence JSON under the results dir (the actual HTTP
  response body — e.g. `etd_browse_tv.json`, `etd_seasons.json`,
  `etd_eps_season_<id>.json`, `etd_ep_detail_<n>.json`) plus the derived
  fact-table `etd_episode_facts.tsv` (`season_id`, `episode_id`,
  `episode_number`, `title`).
- `summary.txt` (PASS/FAIL/SKIP rows + counts) and `summary.json`
  (machine-readable).
- PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
- **Exit code 0** iff every non-SKIP assertion PASSed; **1** otherwise.

## Assertions

| ID | Endpoint(s) | Asserts |
|---|---|---|
| ETD-A | `GET /entities/browse/tv_show` → `/entities/{id}/children` (seasons) → `/entities/{season}/children` (episodes) → detail | **DEFECT-H** / NEGATIVE — NO two episodes under the same season share the same `episode_number` (every `(season_id, episode_number)` pair unique) |
| ETD-B | per-episode `GET /entities/{id}` detail | **DEFECT-I** / POSITIVE — at least one episode with a human filename title has an entity `title` that is the human title (NOT matching `^Episode [0-9]+$`); the `Pilot` fixture |
| ETD-C | the same per-episode detail set | fallback honesty — `"Episode N"` is an **accepted** fallback for genuinely-untitled episodes; the guard does not false-FAIL them (only an empty/malformed title is reportable) |
| ETD-D | second independent `GET /entities/{show}/children` walk | count sanity + determinism (§11.4.50) — episodes present and the de-duplicated `tv_episode` count is stable across two reads |

## Edge cases

- **API unreachable** → every assertion SKIPs-with-reason (§11.4.3); never a
  fabricated PASS.
- **No token** (login failed and none supplied, including the no-hardcoded-
  password case) → auth-dependent assertions SKIP-with-reason.
- **`python3` absent** → all assertions SKIP (no tautology oracle).
- **Genuinely-untitled episodes** (`"Episode N"`) → ETD-C explicitly ACCEPTS
  the pattern; the guard PASSes whether or not such an episode exists.
- The suite is **read-only** (GETs only): it creates no server-side state and
  is safe to re-run any number of times (§11.4.50, §11.4.98 re-runnable).

## Internal behaviour

`http_get` performs each request, writing the body to `<name>.json` and the
HTTP status to `<name>.status`. JSON parsing is done by inline `python3`
(`json_titles`, `json_field`, `id_for_title`, `json_child_ids`). The
show → seasons → episodes hierarchy is walked once into the
`etd_episode_facts.tsv` fact-table; only entities whose detail
`media_type == "tv_episode"` are considered. ETD-A reduces the fact-table to
`(season_id, episode_number)` pairs and asserts uniqueness. ETD-B/ETD-C
classify each episode `title` against the generic `^Episode [0-9]+$` regex.
ETD-D re-walks the hierarchy a second time and compares the `tv_episode`
count. A PASS is only emitted via `ab_pass_with_evidence`, which refuses to
pass on a missing/empty evidence file.

## Related scripts

- `scripts/testing/full_automation/catalog_aggregation_granularity.sh` — the
  DEFECT-E aggregation-granularity guard this suite is modelled on (same
  helpers, same `.api-env` credential source, same captured-evidence layout).
- `submodules/helix_qa/banks/catalog_episode_titles_dedup.yaml` — the HelixQA
  challenge bank that scores these assertions.

## Cross-references

- Constitution: §11.4.18 (script docs), §11.4.27 (real system), §11.4.69
  (captured evidence), §11.4.135 (standing regression guard), §11.4.146
  (extend-to-all-cases), §11.4.115 (RED-baseline reproduce-first), §11.4.50
  (determinism), §11.4.10 (credentials), §11.4.3 (topology SKIP), §11.4.14
  (cleanup), §11.4.67 (target-shell parseability).

**Last verified:** 2026-06-25 (authored; `sh -n` + `bash -n` clean. NOT yet
run against a live API — the conductor runs it GREEN post-rebuild; on the
current pre-rebuild API it RED-baselines per §11.4.115: the dup-episode +
"Episode 1" defects are still present).

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
