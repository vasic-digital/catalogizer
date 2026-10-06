# catalog_functional_matrix.sh

**Revision:** 1
**Last modified:** 2026-06-25T12:30:00Z

Companion guide for `scripts/testing/full_automation/catalog_functional_matrix.sh`
(per Helix Constitution §11.4.18 — every script ships an in-source doc block AND
this external user guide).

## Overview

Full-automation REST API functional-matrix test suite for the Catalogizer
`catalog-api` (Go/Gin) backend. It drives the **live** HTTP API with real `curl`
requests and asserts, for every test, BOTH the HTTP status AND a real observable
body field — never a tautology. Each test writes a captured-evidence JSON file
(the actual HTTP response) and prints `PASS`/`FAIL` with the evidence path, in
the anti-bluff `ab_pass_with_evidence` style of §11.4.69. An unreachable API
yields honest `SKIP`-with-reason (§11.4.3), never a fabricated PASS.

### Functional matrix covered

| Test | Endpoint(s) | Real assertion |
|---|---|---|
| T1 health | `GET /health` | 200 + body `status` == `healthy`/`ok` |
| T2 login | `POST /api/v1/auth/login` | 200 + non-empty `session_token` |
| T2b login negative | `POST /api/v1/auth/login` (wrong pw) | 401 + `error` body |
| T3 catalog list | `GET /api/v1/catalog` (auth) | 200 + `roots` key |
| T3b catalog unauth | `GET /api/v1/catalog` (no token) | 401 |
| T4 entities browse | `GET /api/v1/entities` (auth) | 200 + `items[]` + `total` |
| T5 media recent | `GET /api/v1/media/recent` (auth) | 200 + list-shaped body |
| T6 favorites cycle | `POST` / `GET` / `DELETE /api/v1/favorites` | add → listing **contains** entity_id → remove |
| T7 playback lifecycle | `POST /api/v1/playback/sessions/{start,progress,end}` + `GET /api/v1/entities/:id/progress` | resume position persists (**"remember where you left off"**) |
| T8 search | `GET /api/v1/search?q=` | 200 + `results` array |

## Prerequisites

- `bash` and `curl` on `PATH`.
- `jq` is **optional** — the script falls back to `grep`/`sed` field extraction
  when `jq` is absent.
- A reachable `catalog-api` instance. By default the suite targets
  `http://127.0.0.1:18080` so it does NOT collide with a shared instance on
  `:8080`.

## Inputs (environment variables)

| Variable | Default | Meaning |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:18080` | API base URL |
| `CATALOGIZER_USER` | `admin` | login username |
| `CATALOGIZER_PASS` | `admin` | login password |
| `CATALOGIZER_TOKEN` | _(unset)_ | pre-acquired JWT; if set, T2 login acquisition is skipped and the token is reused |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/functional_matrix/<ts>` | evidence output directory |
| `CATALOGIZER_MEDIA_ID` | `1` | `media_item_id` used by the T7 playback lifecycle |
| `CATALOGIZER_FAV_ENTITY_ID` | `1` | `entity_id` used by the T6 favorites cycle |

## Outputs

- One captured-evidence JSON file per HTTP request under the results dir
  (e.g. `t1_health.json`, `t2_login.json`, `t7d_pb_resume.json`).
- `summary.txt` (human-readable PASS/FAIL/SKIP rows) and `summary.json`
  (machine-readable counts).
- `PASS`/`FAIL`/`SKIP` lines on stdout, each citing its evidence path.
- Exit code `0` iff every non-SKIP test PASSed; `1` if any test FAILed.

## Usage examples

```bash
# Default target (127.0.0.1:18080), default admin/admin
./scripts/testing/full_automation/catalog_functional_matrix.sh

# Explicit target + credentials + results dir
CATALOGIZER_BASE_URL=http://127.0.0.1:18080 \
CATALOGIZER_USER=admin CATALOGIZER_PASS="$YOUR_QA_PASSWORD" \
CATALOGIZER_RESULTS_DIR=qa-results/functional_matrix/my_run \
./scripts/testing/full_automation/catalog_functional_matrix.sh

# Reuse a pre-acquired token (skips the login acquisition step)
CATALOGIZER_TOKEN="<jwt>" \
./scripts/testing/full_automation/catalog_functional_matrix.sh
```

### Booting a throwaway instance for validation

To validate the suite WITHOUT touching a shared instance on `:8080`, boot your
own `catalog-api` on `:18080` from an isolated working directory (so it writes
its own `.service-port` and sqlite DB there):

```bash
WORK=$(mktemp -d)
cd "$WORK" && mkdir -p data
HOST=127.0.0.1 SERVER_PORT=18080 DATABASE_TYPE=sqlite \
  JWT_SECRET=test-only-secret ADMIN_USERNAME=admin ADMIN_PASSWORD=secret \
  /path/to/catalog-api-bin > server.log 2>&1 &
# ... wait for GET /health to return 200, run the suite, then kill the PID.
```

The `media_item_id` referenced by T7 has a foreign key to `media_items`. On an
empty DB, seed one row so the playback lifecycle runs for real instead of
SKIPping:

```bash
sqlite3 "$WORK/catalog.db" \
  "INSERT INTO media_items (media_type_id, title, year, status)
   VALUES (1, 'QA Resume Fixture Movie', 2026, 'detected');"
# then run with CATALOGIZER_MEDIA_ID=<new id>
```

## Edge cases & behaviour

- **API unreachable** → the pre-flight `GET /health` probe detects a curl
  transport failure and the suite marks every test `SKIP`-with-reason
  (§11.4.3). It exits `0` (honest SKIPs are not failures), never a PASS.
- **No token** (login failed / not supplied) → all auth-required tests SKIP.
- **T6 favorites add returns 404** → treated as an honest data-precondition SKIP
  (the referenced `entity_id` does not exist in this DB), not a PASS. A re-run
  returning 409 (already-in-favorites) still counts as the write-path exercised.
- **T7 playback start returns 404/500** → SKIP with a `media_precondition`
  reason (the `media_item_id` has no `media_items` row). Seed one and set
  `CATALOGIZER_MEDIA_ID` to run it for real.
- The favorites cycle is **self-cleaning** (it removes what it adds, §11.4.14).

## Internal behaviour

- `set -u`; `sh -n` and `bash -n` clean (§11.4.67).
- `http_request` writes `<name>.json` (body), `<name>.status` (HTTP code), and
  `<name>.curlerr` (stderr) for each call; a transport failure records `000`.
- `ab_pass_with_evidence` refuses to declare PASS unless the evidence file
  exists and is non-empty.
- The auth-required tests are gated on a real `session_token` extracted from the
  T2 login response (field name `session_token`, NOT `token`/`access_token`).

## Related scripts / artefacts

- `submodules/helix_qa/banks/catalog_functional_matrix.yaml` — the HelixQA
  challenge bank mirroring this matrix (scores PASS only on positive captured
  evidence).
- `submodules/helix_qa/banks/full-qa-api.yaml` — the broader 280-case API bank.
- `catalog-api/main.go` — route registration (source of truth for endpoints).

## Last verified

2026-06-25 — ran against a throwaway `catalog-api` on `127.0.0.1:18080`
(temp sqlite DB, one seeded `media_items` row): **15 PASS, 0 FAIL, 0 SKIP**.
The T7 resume-position evidence positively showed `last_position: 137`,
proving the "remember where you left off" feature end-to-end.

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
