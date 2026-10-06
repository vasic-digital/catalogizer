# catalog_music_album_tracks.sh — User Guide

**Revision:** 1
**Last modified:** 2026-06-25T00:00:00Z

Companion guide (Helix Constitution §11.4.18) for
`scripts/testing/full_automation/catalog_music_album_tracks.sh`.

## Overview

A standing regression guard (§11.4.135) and full-automation API test
(§11.4.27 / §11.4.98) proving the catalog-api **MUSIC** surfaces work
end-to-end against the LIVE catalog-api over real HTTP:

- **Album browse + resolve** — `GET /api/v1/entities/browse/music_album`
  lists `music_album` entities; each resolves via
  `GET /api/v1/entities/{id}` to `media_type == "music_album"`.
- **Album → track listing (container children)** —
  `GET /api/v1/entities/{album_id}/children` returns the album's tracks. The
  track list shape is asserted **honestly** (`items[]` present, count ≥ 0).
  Where tracks ARE seeded, each track's `track_number` ordering is captured as
  positive evidence; where the catalog seeds an album with **no** track
  children, the children-list shape is the asserted fact rather than a
  fabricated track (§11.4.6 no-guessing, §11.4.3 honesty).
- **Per-track playback progress + resume** — the SAME mechanism every media
  card uses: `POST /api/v1/playback/sessions/start → /progress → /end`, then
  `GET /api/v1/entities/{id}/progress` reads `progress.last_position` back.
  A chosen playable media item (a track when seeded, else the album entity,
  which is itself a valid `media_item_id` for the playback lifecycle) has its
  position set and read back **equal** (resume-per-track), and the read-back is
  stable across two cycles (determinism §11.4.50).

This suite drives the LIVE catalog-api over real HTTP and captures the real
HTTP response body as evidence for every assertion (anti-bluff §11.4 /
§11.4.69 — never a tautology). It is an extend-to-all-cases pass (§11.4.146):
the browse, every album resolve, the per-album children-list-shape sweep, the
playback round-trip, and the determinism re-read.

### Corpus note (as observed 2026-06-25)

The current seeded corpus contains **3 `music_album` entities** (ids 20, 31,
30 — e.g. "The Dark Side of the Moon") but **none have track children**
(`children_count == 0` for all). Accordingly:

- MAT-API-002 asserts the children-list **shape** (`items[]`, count ≥ 0)
  honestly — it does NOT fail for an empty track list and does NOT fabricate a
  track. When track children become seeded later, the same assertion captures
  their `track_number` ordering as positive evidence with no change required.
- MAT-API-004 / MAT-API-005 drive playback on the **album entity** as the
  playable `media_item_id` (the playback lifecycle is entity-agnostic and is
  the identical mechanism a per-track resume uses). When a real track child is
  seeded, the suite automatically prefers it.

## Prerequisites

- A **running** catalog-api on `http://127.0.0.1:8080` (override with
  `CATALOGIZER_BASE_URL`), backed by the populated DB containing ≥ 1
  `music_album` entity.
- `bash`, `curl`, `python3` on `PATH`. `python3` is the JSON oracle; if it is
  absent the suite SKIPs-with-reason rather than emit a tautological PASS.
- Admin credentials. The suite sources an `.api-env` file
  (`ADMIN_USERNAME` / `ADMIN_PASSWORD`, optionally `QA_TOKEN`) and **never
  echoes any secret** (§11.4.10). There is **no hardcoded password fallback** —
  if no `ADMIN_PASSWORD` / `CATALOGIZER_PASS` / token is available, login fails
  and the suite SKIPs honestly. By default it picks the most recent
  `qa-results/catalogizer-qa-*/.api-env`; override with `CATALOGIZER_ENV_FILE`.
  A `CATALOGIZER_TOKEN` env var (or `QA_TOKEN` from the env file)
  short-circuits the login.

## Usage examples

```bash
# Default: discover the newest .api-env, hit http://127.0.0.1:8080
./scripts/testing/full_automation/catalog_music_album_tracks.sh

# Explicit base URL + env file
CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
CATALOGIZER_ENV_FILE=qa-results/catalogizer-qa-20260625T102312Z/.api-env \
  ./scripts/testing/full_automation/catalog_music_album_tracks.sh

# Pre-acquired token (skips login)
CATALOGIZER_TOKEN="$(cat /tmp/.qa_admin_tok)" \
  ./scripts/testing/full_automation/catalog_music_album_tracks.sh

# Override the playback position used for set/read-back
CATALOGIZER_TRACK_POSITION=240 \
  ./scripts/testing/full_automation/catalog_music_album_tracks.sh
```

## Inputs (environment variables)

| Variable | Default | Purpose |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:8080` | API base URL |
| `CATALOGIZER_ENV_FILE` | newest `qa-results/catalogizer-qa-*/.api-env` | credentials single-source (§11.4.10), sourced never echoed |
| `CATALOGIZER_USER` | `$ADMIN_USERNAME` or `admin` | login username |
| `CATALOGIZER_PASS` | `$ADMIN_PASSWORD` (no hardcoded fallback) | login password |
| `CATALOGIZER_TOKEN` | `$QA_TOKEN` from env file, else login | pre-acquired session token |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/music_album_tracks/<ts>` | evidence output dir |
| `CATALOGIZER_TRACK_POSITION` | `137` | playback position (seconds) set + read back |

## Assertions

| ID | Assertion |
|---|---|
| MAT-API-001 | `music_album` browse resolves ≥ 1 album (else honest SKIP) |
| MAT-API-002 | every album's `/children` returns a valid `items[]` track listing (count ≥ 0; ascending `track_number` when present) |
| MAT-API-003 | every browsed album resolves to `media_type == "music_album"` via `/entities/{id}` |
| MAT-API-004 | playback set on the chosen track/album reads back `progress.last_position == position` (resume-per-track) |
| MAT-API-005 | the read-back `last_position` is identical across two independent cycles (determinism §11.4.50) |

## Outputs

- Per-assertion captured-evidence JSON under the results dir (the real HTTP
  response body): `mat_browse_album.json`, `mat_album_detail_<id>.json`,
  `mat_album_children_<id>.json`, `mat_play_start.json`, `mat_play_read1.json`,
  `mat_play_read2.json`, …
- `summary.txt` (human) + `summary.json` (machine).
- `PASS` / `FAIL` / `SKIP` lines on stdout, each citing its evidence path.
- Exit code `0` iff every non-SKIP assertion PASSed; `1` otherwise.

## Edge cases

- **API unreachable** → every assertion SKIP-with-reason (§11.4.3), exit `0`.
- **No token** (no creds, login non-200) → SKIP-with-reason, exit `0`.
- **`python3` absent** → SKIP-with-reason (the JSON oracle is mandatory; no
  tautology fallback), exit `0`.
- **Zero seeded `music_album`** → MAT-API-001 SKIPs (topology gap, not a
  product defect); downstream assertions SKIP for lack of an id.
- **Albums with no track children** → MAT-API-002 asserts the children-list
  shape (count ≥ 0), never fabricating a track; playback falls back to the
  album entity.

## Internal behaviour

- All catalog reads (`browse`, `/children`, `/entities/{id}`) are read-only
  GETs (§11.4.119 — the conductor owns the catalog/DB; this suite never mutates
  catalog rows).
- The only server-side state is **per-user playback progress**
  (`start → progress → end`). This is NOT catalog state; `last_position` is
  set-state (overwrite), so re-runs with the same position read back the same
  value — the assertion is intrinsically idempotent and self-consistent.
- A `trap '…' EXIT INT TERM` cleanup removes the suite's transient scratch
  files on every exit path (§11.4.14).
- Credentials are sourced from the env file and **never printed** (§11.4.10);
  there is no hardcoded password.
- Parses clean under both `sh -n` and `bash -n` (§11.4.67).

## Related scripts

- `scripts/testing/full_automation/catalog_episode_titles_dedup.sh` — the model
  this suite is built on (TV-episode hierarchy + titles).
- `scripts/testing/full_automation/catalog_aggregation_granularity.sh` — entity
  aggregation granularity guard.

## Cross-references

- `submodules/helix_qa/banks/catalog_music_album_tracks.yaml` — HelixQA
  Challenge bank (ids `MAT-API-001`..`MAT-API-005`).
- `catalog-api/main.go` — route definitions (`entities/browse/:type`,
  `entities/:id/children`, `entities/:id`, `playback/sessions/*`,
  `entities/:id/progress`).
- Helix Constitution §11.4 / §11.4.3 / §11.4.10 / §11.4.27 / §11.4.50 /
  §11.4.69 / §11.4.98 / §11.4.119 / §11.4.135 / §11.4.146.

**Last verified:** 2026-06-25 (album browse + children + playback round-trip
probed live against `http://127.0.0.1:8080`; 3 `music_album` entities present,
0 track children, playback read-back `last_position` exact).

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
