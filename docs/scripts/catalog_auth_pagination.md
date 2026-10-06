# catalog_auth_pagination.sh

**Revision:** 1
**Last modified:** 2026-06-25T00:00:00Z

Companion guide for `scripts/testing/full_automation/catalog_auth_pagination.sh`
(per Helix Constitution §11.4.18 — every script ships an in-source doc block AND
this external user guide).

## Overview

Full-automation REST API coverage suite for the Catalogizer `catalog-api`
(Go/Gin) **AUTH matrix**, **entities pagination semantics**, and the
**unauthenticated health probe** — the slices the broader
`catalog_functional_matrix.sh` suite touches but does not deeply exercise. It
drives the **live** HTTP API with real `curl` requests and asserts, for every
test, BOTH the HTTP status AND a real observable body field/shape — never a
tautology. Each test writes a captured-evidence JSON file (the actual HTTP
response) and prints `PASS`/`FAIL`/`SKIP` with the evidence path, in the
anti-bluff `ab_pass_with_evidence` style of §11.4.69. An unreachable API yields
honest `SKIP`-with-reason (§11.4.3), never a fabricated PASS; a dataset too
small to prove page distinctness yields an honest `SKIP`, never a PASS-by-default.

### Coverage

| Test | Endpoint(s) | Real assertion |
|---|---|---|
| H1 health unauth | `GET /health` (no token) | 200 |
| A1 login success | `POST /api/v1/auth/login` (valid) | 200 + non-empty `session_token` |
| A2 login wrong password | `POST /api/v1/auth/login` (bad pw) | 401 + `error` body |
| A3 login missing field | `POST /api/v1/auth/login` `{}` | rejected (`400`/`401`/`422`); a `200` is a **credential-bypass FAIL** |
| A3b login malformed JSON | `POST /api/v1/auth/login` (`not-json{{{`) | 400 at the bind layer (401/422 also accepted as genuine rejection) |
| A4 protected unauth | `GET /api/v1/catalog` (no token) | 401 |
| P1 limit clamp | `GET /api/v1/entities?limit=1` | `items` array length `<= 1` |
| P2 total numeric | (P1 response) | `total` is a non-negative number |
| P3 distinct pages | `?limit=2&offset=0` vs `?limit=2&offset=2` | pages **differ** when `total>2`; else honest SKIP citing `total` |

## Honest backend note (§11.4.6 — captured FACT, not a guess)

`catalog-api/handlers/auth_handler.go` binds the login body with
`c.ShouldBindJSON(&req)` into `models.LoginRequest`, whose fields carry
go-playground `validate:"required"` tags — **not** gin `binding:"required"` tags
(`catalog-api/models/user.go`). Gin's `ShouldBindJSON` only enforces `binding:`
tags, so a **missing** `username`/`password` does **not** trigger a `400` at the
bind step: it binds to empty strings and is rejected downstream by the auth
service (typically `401`). Only **malformed JSON** deterministically yields `400`
at the bind layer.

Consequently:

- **A3** (missing field) accepts the rejection set `{400, 401, 422}` — a
  missing-credential login MUST be rejected by *some* layer; the test FAILs only
  if the login is *accepted* with `200` (a real credential-bypass defect) or
  rejected with an unexpected code.
- **A3b** (malformed JSON) asserts the deterministic `400`, and also accepts
  `401`/`422` as a genuine rejection if a deployment rejects it downstream.

This split keeps the assertions honest while still catching the security-critical
failure mode (an empty-credential login being accepted).

## Prerequisites

- `bash` and `curl` on `PATH`.
- `jq` is **recommended** for array-length and numeric-field extraction. When
  `jq` is absent the script falls back to `python3`, then to crude `grep`/`sed`
  parsing. If neither `jq` nor `python3` is present, the pagination
  length/number assertions FAIL honestly (length undeterminable) rather than
  guessing.
- A reachable `catalog-api` instance.

## Inputs (environment variables)

| Variable | Default | Meaning |
|---|---|---|
| `CATALOGIZER_BASE_URL` | `http://127.0.0.1:8080` | API base URL |
| `CATALOGIZER_USER` | `admin` | login username |
| `CATALOGIZER_PASS` | _(required — §11.4.10)_ | login password |
| `CATALOGIZER_TOKEN` | _(unset)_ | pre-acquired token; if set, A1 login acquisition is skipped and the token is reused for the pagination tests |
| `CATALOGIZER_RESULTS_DIR` | `qa-results/auth_pagination/<ts>` | evidence output directory |

No credentials are hardcoded in the script source; they come from the
environment (with documented defaults), per §11.4.10.

## Outputs

- One captured-evidence JSON file per HTTP request under the results dir
  (e.g. `h1_health.json`, `a1_login.json`, `a3_login_missing.json`,
  `p1_entities_limit1.json`, `p3_page0.json`, `p3_page1.json`).
- `summary.txt` (human-readable PASS/FAIL/SKIP rows) and `summary.json`
  (machine-readable counts).
- `PASS`/`FAIL`/`SKIP` lines on stdout, each citing its evidence path / reason.
- Exit code `0` iff every non-SKIP test PASSed; `1` if any test FAILed.

## Usage examples

```bash
# Default target (127.0.0.1:8080), admin user; export CATALOGIZER_PASS (§11.4.10)
./scripts/testing/full_automation/catalog_auth_pagination.sh

# Explicit target + credentials + results dir
CATALOGIZER_BASE_URL=http://127.0.0.1:18080 \
CATALOGIZER_USER=admin CATALOGIZER_PASS=secret \
CATALOGIZER_RESULTS_DIR=qa-results/auth_pagination/my_run \
./scripts/testing/full_automation/catalog_auth_pagination.sh

# Reuse a pre-acquired token (skips the A1 login acquisition step)
CATALOGIZER_TOKEN="<token>" \
./scripts/testing/full_automation/catalog_auth_pagination.sh
```

### Booting a throwaway instance for validation

To validate the suite WITHOUT touching a shared instance, boot your own
`catalog-api` from an isolated working directory (so it writes its own sqlite DB
there), wait for `GET /health` to return `200`, run the suite, then kill the PID.
To exercise **P3** for real (instead of the honest dataset-too-small SKIP), seed
**more than two** entity rows so `total > 2` and the two pages genuinely differ.

## Edge cases & behaviour

- **API unreachable** → the pre-flight `GET /health` probe detects a curl
  transport failure and the suite marks every test `SKIP`-with-reason
  (§11.4.3). It exits `0` (honest SKIPs are not failures), never a PASS.
- **No token** (login failed / not supplied) → the three pagination tests SKIP
  with a `no_token` reason; the auth + health tests still run.
- **`total <= 2`** → **P3** SKIPs with a `dataset_too_small` reason and cites the
  captured `total`, rather than pretending the pages are distinct.
- **Empty-credential login accepted (`200`)** → **A3** FAILs and surfaces it as a
  credential-bypass defect (the security-critical failure mode).
- **`jq` and `python3` both absent** → **P1** FAILs honestly (`items` length
  undeterminable) rather than guessing a pass.

## Internal behaviour

- `set -u`; `sh -n` and `bash -n` clean (§11.4.67).
- `http_request` writes `<name>.json` (body), `<name>.status` (HTTP code), and
  `<name>.curlerr` (stderr) for each call; a transport failure records `000`.
- `ab_pass_with_evidence` refuses to declare PASS unless the evidence file
  exists and is non-empty.
- The login token is extracted from the A1 response field `session_token` (NOT
  `token`/`access_token`) and reused for the pagination tests.
- `array_len` prefers `jq`, then `python3`, then a crude fallback; `is_uint`
  guards every numeric comparison so a non-numeric value never silently passes.

## Related scripts / artefacts

- `scripts/testing/full_automation/catalog_functional_matrix.sh` — the sibling
  functional-matrix suite (health/login/catalog/entities/media/favorites/
  playback/search). This suite deepens the **auth + pagination + health** slices
  it does not fully cover.
- `submodules/helix_qa/banks/catalog_auth_pagination.yaml` — the HelixQA
  challenge bank mirroring this coverage (scores PASS only on positive captured
  evidence).
- `catalog-api/handlers/auth_handler.go` + `catalog-api/models/user.go` — the
  login handler + request model (source of truth for the binding/validation
  behaviour documented above).

## Last verified

2026-06-25 — see the run tally reported by the authoring session. The script is
`bash -n` and `sh -n` clean; when run against a live `catalog-api` it captures
one evidence JSON file per request under the results dir.

## TS-02 adoption (T052, 2026-10-07)

`ab_pass_with_evidence` is no longer defined inside this script: it is sourced from `tools/evidence/lib/ab_pass_with_evidence.sh`, the single shared definition. It keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check as an ev/1 entry through `tools/evidence/evrec` (a recorder that is absent or refuses makes the assertion a FAIL, never a PASS). Environment: `EVREC_ITEM` (register item or run id, default `RUN-<pid>`), `EVREC_BIN` (the recorder). Counters and PASS/FAIL line formats are unchanged. Guide: `docs/scripts/evidence-ts02.md`.
