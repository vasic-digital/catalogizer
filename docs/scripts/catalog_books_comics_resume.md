# catalog_books_comics_resume.sh

**Revision:** 1
**Last modified:** 2026-06-25T00:00:00Z

Companion user guide (§11.4.18) for
`scripts/testing/full_automation/catalog_books_comics_resume.sh`.

## Overview

A §11.4 full-automation API guard that proves the **catalog-api BOOK + COMIC
reading surfaces** actually work for an end user, against the LIVE catalog-api
over real HTTP. It validates three things:

1. **Entity resolution** — a book and/or comic entity resolves via the
   type-browse endpoint.
2. **Chapter/page listing** — that entity's chapters/pages list via the
   container-children endpoint (≥0, honest if none seeded).
3. **Resume-where-left-off** — reading progress works through the **same**
   playback-session mechanism episodes and movies use: a reading position is
   set and then read back equal.

It is modelled on `catalog_episode_titles_dedup.sh` and is a §11.4.135 standing
regression guard plus a §11.4.146 extend-to-all-cases pass (both book AND comic
types, the children sweep, the progress round-trip, and a determinism re-read).

## Why reading progress is the same mechanism

The catalog-api playback handler explicitly records sessions for "video, audio,
**book, comic**, game" (see `catalog-api/handlers/playback_handler.go`). The
per-entity rollup at `GET /api/v1/entities/{id}/progress` carries
`progress.last_position` and `progress.position_unit`. For books/comics the
position dimension is **`pages`** (vs `seconds` for audio/video). There is no
separate "reading-position" subsystem — resume is the playback rollup with
`position_unit = "pages"`, and this suite exercises exactly that path.

## Real endpoints exercised (discovered from `catalog-api/main.go`)

| Purpose | Method + path |
|---|---|
| Login (acquire `session_token`) | `POST /api/v1/auth/login` `{username,password}` |
| Browse a media type | `GET /api/v1/entities/browse/{book\|comic}` |
| Container children (chapters/pages) | `GET /api/v1/entities/{id}/children` |
| Per-entity reading progress (read) | `GET /api/v1/entities/{id}/progress` |
| Start a reading session (write, gated) | `POST /api/v1/playback/sessions/start` |
| Advance reading position (write, gated) | `POST /api/v1/playback/sessions/progress` |
| Health pre-flight | `GET /health` |

Auth is `Authorization: Bearer <session_token>`.

## Assertions

| ID | Name | Type | What it proves |
|---|---|---|---|
| BCR-A | book-or-comic-entity-resolves | POSITIVE | ≥1 entity resolves from `browse/book` or `browse/comic` with a numeric id + media_type in `{book, comic, comic_book}`. |
| BCR-B | chapters-pages-via-children | POSITIVE / honest-zero | `GET /entities/{id}/children` returns 200 and a JSON list (≥0 chapters/pages). A non-200 or non-array is a FAIL; a genuine empty list PASSes (single-file book/comic). |
| BCR-C | reading-position-resume-roundtrip | POSITIVE | The progress endpoint returns a well-formed `{"progress": object\|null}` envelope; with the write-leg enabled, a page is written via a real session and read back **equal** (`progress.last_position` + `progress.position_unit == "pages"`). |
| BCR-D | children-count-determinism | POSITIVE (§11.4.50) | The children count is stable across two independent reads. |

## Read-only vs. write (the resume round-trip)

Per §11.4.119 the conductor **owns** the live API + DB, so by default this suite
only issues **read-only GET** probes. The BCR-C resume *write* round-trip
(`start` + `progress`, which append a per-user playback session row — never a
scan/clear/aggregate) is gated behind `CATALOGIZER_ALLOW_PLAYBACK_WRITE=1`.

- **Read-only (default):** BCR-C PASSes the read half (the progress endpoint
  returns a well-formed envelope) and SKIPs the write round-trip with a clear
  reason.
- **Write enabled (conductor-gated):** BCR-C writes page `${CATALOGIZER_READ_POSITION}`
  (default 7) and asserts it reads back equal — the full resume proof.

## Prerequisites

- The catalog-api running and reachable at `CATALOGIZER_BASE_URL`
  (default `http://127.0.0.1:8080`).
- `bash`, `curl`, `python3` (python3 is the JSON oracle — absent ⇒ honest SKIP).
- Credentials available via a `.api-env` file or the documented env vars
  (§11.4.10 — never hardcoded, never echoed).

## Usage

```bash
# Read-only (default) — safe against a conductor-owned live API
./scripts/testing/full_automation/catalog_books_comics_resume.sh

# Full resume round-trip (conductor-gated write-leg)
CATALOGIZER_ALLOW_PLAYBACK_WRITE=1 \
CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
CATALOGIZER_ENV_FILE=qa-results/catalogizer-qa-XXXX/.api-env \
./scripts/testing/full_automation/catalog_books_comics_resume.sh
```

## Inputs (env vars, all optional)

| Var | Default | Meaning |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:8080` | API base URL |
| `CATALOGIZER_ENV_FILE` | most-recent `qa-results/catalogizer-qa-*/.api-env` | credentials source (sourced, never echoed) |
| `CATALOGIZER_USER` | `$ADMIN_USERNAME` or `admin` | login username |
| `CATALOGIZER_PASS` | `$ADMIN_PASSWORD` (no hardcoded secret) | login password |
| `CATALOGIZER_TOKEN` | `$QA_TOKEN` from env file, else login | pre-acquired session token |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/books_comics_resume/<ts>` | evidence output dir |
| `CATALOGIZER_ALLOW_PLAYBACK_WRITE` | unset | `1` enables the resume write round-trip |
| `CATALOGIZER_READ_POSITION` | `7` | page number written + read back in BCR-C |

## Outputs

- Per-assertion captured-evidence JSON (the real HTTP response bodies) under the
  results dir.
- `summary.txt` (human) and `summary.json` (machine) with PASS/FAIL/SKIP counts.
- Exit `0` iff every non-SKIP assertion PASSed; `1` otherwise.

## Edge cases & honest SKIPs (anti-bluff §11.4 / §11.4.3 / §11.4.6)

- **No book/comic seeded** — if neither `browse/book` nor `browse/comic`
  resolves an entity (or the type name isn't provisioned in `media_types`, which
  returns HTTP 400), every assertion SKIPs with reason. **Never a fake PASS,
  never a FAIL** — the reading surface can only be proven when content exists.
- **API unreachable** — every assertion SKIPs (`network_unreachable`).
- **No auth token** — every assertion SKIPs (`no_token`).
- **python3 absent** — every assertion SKIPs (no tautology fallback).
- **Single-file book/comic** — zero child entities is a legitimate honest-zero
  PASS for BCR-B, with the captured (empty-list) body as evidence.

## Internal behaviour

The suite resolves the first `{book, comic, comic_book}` entity from the
type-browse responses, walks its children once (cached for BCR-B + BCR-D),
reads the progress envelope, and — only when the write-leg is enabled —
performs the `start → progress → read-back` round-trip. Every PASS asserts a
**real observed value** (entity id, media_type, children count, or
`last_position`) from a captured response body and cites its evidence file.

## Related scripts

- `scripts/testing/full_automation/catalog_episode_titles_dedup.sh` — the model
  this suite is built on (TV-episode aggregation guard).
- `submodules/helix_qa/banks/catalog_books_comics_resume.yaml` — HelixQA bank
  driving this suite's assertions (ids `BCR-API-001..004`).

**Last verified:** 2026-06-25 (authored; `sh -n` + `bash -n` clean).

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
