#!/usr/bin/env bash
# api_contract_live.sh - one-shot runner for the LIVE OpenAPI contract sweep of catalog-api.
# Test: catalog-api/api_contract_live_test.go (build tag `live`). Guide: docs/TESTING_GUIDE.md ("API contract sweep (live)").
#
# Builds the REAL catalog-api binary, runs it against a REAL SQLCipher SQLite file on loopback (ephemeral port, random
# runtime secrets that are never printed) and drives EVERY operation of docs/api/openapi.yaml: unauthenticated -> 401,
# non-admin on admin routes -> 403, unknown id -> 404, malformed body / missing required query -> 400, happy path with
# response-schema validation, no unexpected 5xx. The sweep runs API_CONTRACT_RUNS (default 3) times on fresh servers and
# fresh databases; the status/verdict signature must be identical in all runs (N=3, deterministic).
#
# Exit codes (distinct; none is ever read as another):
#   0   PASS     the test passed and did not skip
#   1   FAIL     a contract violation, a non-deterministic signature, a build failure, or no test ran
#   77  SKIP     a real dependency is absent: no go toolchain / no systemd-run / host load too high
#   78  PARTIAL  the test ran but SKIPped (nothing proven)
# `api_contract_live.sh --classify <go-test-log> <go-test-exit-code>` prints the verdict for an existing log.
#
# Env: API_CONTRACT_ARTIFACT_DIR (evidence dir, default <repo>/docs/qa/api-contract-live-<utc-date>/run-<ts>), API_CONTRACT_RUNS (3),
#      API_CONTRACT_SPEC, API_CONTRACT_MEMMAX (5G), API_CONTRACT_MAX_LOAD (14).
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
classify() { # $1 = log, $2 = go test exit code
  local log="$1" rc="$2" npass nskip nfail
  npass=$(grep -c -- '^--- PASS: TestLiveAPIContract' "$log" || true)
  nskip=$(grep -c -- '^--- SKIP: TestLiveAPIContract' "$log" || true)
  nfail=$(grep -c -- '^--- FAIL: TestLiveAPIContract' "$log" || true)
  echo "live contract test: pass=$npass skip=$nskip fail=$nfail rc=$rc"
  if [ "$rc" -ne 0 ] || [ "$nfail" -gt 0 ]; then return 1; fi
  if [ "$npass" -eq 0 ]; then [ "$nskip" -gt 0 ] && return 77; return 1; fi
  if [ "$nskip" -gt 0 ]; then return 78; fi
  return 0
}
if [ "${1:-}" = "--classify" ]; then
  [ $# -eq 3 ] || { echo "usage: $0 --classify <go-test-log> <go-test-exit-code>" >&2; exit 64; }
  classify "$2" "$3"; exit $?
fi
skip() { echo "SKIP: $*" >&2; exit 77; }
command -v go >/dev/null 2>&1 || skip "go toolchain not found"
command -v systemd-run >/dev/null 2>&1 || skip "systemd-run not found (memory-capped scope required)"
MAXLOAD="${API_CONTRACT_MAX_LOAD:-14}"
load="$(cut -d' ' -f1 /proc/loadavg)"
awk -v l="$load" -v m="$MAXLOAD" 'BEGIN{exit !(l<=m)}' || skip "host load $load above $MAXLOAD"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE="${API_CONTRACT_ARTIFACT_DIR:-$ROOT/docs/qa/api-contract-live-$(date -u +%Y%m%d)/run-$TS}"
# server run dirs (database, key file, server.log) and the built binary stay in a scratch dir; only the JSON evidence and the
# go test log are copied to $EVIDENCE (never a database, key or server log).
SCRATCH="$(mktemp -d "${TMPDIR:-/tmp}/api-contract-live.XXXXXX")" || exit 1
export SCALING_LIVE_ARTIFACT_DIR="${SCRATCH:?}"
mkdir -p "${EVIDENCE:?}"
LOG="$SCRATCH/go-test.log"
cd "$ROOT/catalog-api" || exit 1
set +e
systemd-run --user --scope -q -p MemoryMax="${API_CONTRACT_MEMMAX:-5G}" -p MemorySwapMax=0 \
  go test -tags live -count=1 -timeout 40m -v -run '^TestLiveAPIContract$' . 2>&1 | tee "$LOG"
rc=${PIPESTATUS[0]}
for f in summary.json operations.json go-test.log trace-run*.log; do [ -f "$SCRATCH/$f" ] && cp "$SCRATCH/$f" "$EVIDENCE/$f"; done
echo "artifacts: $EVIDENCE"
classify "$LOG" "$rc"
v=$?
rm -rf "${SCRATCH:?}"
exit $v
