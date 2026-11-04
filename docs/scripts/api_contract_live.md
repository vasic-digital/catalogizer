# api_contract_live.sh

Live (constitution 11.4.289: no mocks, stubs or fixture servers) contract sweep of every operation in `docs/api/openapi.yaml` against a real `catalog-api` binary built from the tree.

## What it does
- Builds the binary, starts it on an ephemeral loopback port with a real SQLCipher database created by the production migrations and random secrets generated at run time (never printed).
- For each of the spec's operations it sends: unauthenticated (expect 401), non-admin on admin-gated routes (expect 403, only where the spec's 403 text says the caller must be an administrator), unknown id (expect 404 where documented), non-numeric id (expect 400/404), malformed JSON (400), and a happy path generated from the schema. Response bodies are checked against the OpenAPI schema. Any 5xx is a violation (501/503 only when documented).
- Repeats the whole sweep `API_CONTRACT_RUNS` times (default 3) and requires the same signature (sha256 over every operation's probe statuses, schema verdict, happy-path flag and violations) in every run.
- Skipped happy paths (recorded with a reason): `/ws`, challenge run endpoints, admin backup restore.

## Usage
    TMPDIR=/home/<user>/.cache/claude-tmp API_CONTRACT_MAX_LOAD=60 API_CONTRACT_RUNS=3 scripts/test-infra/api_contract_live.sh
    scripts/test-infra/api_contract_live.sh --classify <go-test-log> <go-test-exit-code>

Environment: `API_CONTRACT_ARTIFACT_DIR` (default `docs/qa/api-contract-live-<date>`), `API_CONTRACT_RUNS`, `API_CONTRACT_MEMMAX` (default 5G, run under `systemd-run --user --scope` with swap 0), `API_CONTRACT_MAX_LOAD` (SKIP when the host load average is higher; default 14), `API_CONTRACT_TRACE=1` (per-request trace-runN.log), `API_CONTRACT_SPEC`.

## Exit codes
0 PASS, 1 FAIL (violations or differing signatures), 77 SKIP (load gate or missing tool), 78 PARTIAL.

## Evidence
`summary.json` (counts, binary sha256, endpoint, pid, signature), `operations.json` (per operation: method, path, observed status codes, schema verdict, violations, advisories), `go-test.log`, optional trace logs.

## Honest limits
See `docs/FOLLOW_UPS_NEXT_PHASE.md`, section "Live API contract sweep" (AC1-AC5).
