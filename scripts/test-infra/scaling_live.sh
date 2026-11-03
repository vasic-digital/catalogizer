#!/usr/bin/env bash
# scaling_live.sh - one-shot runner for the LIVE scaling + stress + thin-chaos suite of catalog-api.
# Guide: docs/TESTING_GUIDE.md section "Scaling and chaos (live)".
#
# Builds the REAL catalog-api binary and runs it against a REAL SQLCipher SQLite file on loopback
# (ephemeral port). Excluded from the default `go test ./...` by the `live` build tag.
#
# Exit codes (all distinct, none is ever read as another):
#   0   PASS     every selected live test passed, none skipped
#   1   FAIL     a test failed, `go test` exited non-zero, no test ran, or the tree under test does
#               not build (a failed `go build` is a FAIL of the tree, never a SKIP)
#   77  SKIP     a real dependency is absent: no go toolchain / no systemd-run / host load too high,
#               or every live test skipped (e.g. no ImageMagick converter) - SKIP is never PASS
#   78  PARTIAL  at least one live test PASSed and at least one SKIPped, none failed: the run is NOT
#               a full PASS (the skipped legs are unproven) and callers must not treat it as one
# `scaling_live.sh --classify <go-test-log> <go-test-exit-code>` prints the verdict for an existing
# log and exits with the code above (used by scripts/test-infra/test_scaling_live_classify.sh).
#
# Env: SCALING_LIVE_ARTIFACT_DIR (default: <repo>/.scaling-live-artifacts/<utc-ts>)
#      SCALING_LIVE_RUN (go test -run regex, default TestLive)
#      SCALING_P99_MS / SCALING_P99_FACTOR / SCALING_P99_FLOOR_MS / SCALING_RSS_GROWTH_KB (see guide)
#      SCALING_LIVE_MAX_LOAD (default 14), SCALING_LIVE_MEMMAX (default 4G)
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
classify() { # $1 = log file, $2 = go test exit code; prints the summary line, returns the verdict code
  local log="$1" rc="$2" npass nskip nfail
  npass=$(grep -c -- '^--- PASS: TestLive' "$log" || true)
  nskip=$(grep -c -- '^--- SKIP: TestLive' "$log" || true)
  nfail=$(grep -c -- '^--- FAIL: TestLive' "$log" || true)
  echo "live tests: pass=$npass skip=$nskip fail=$nfail rc=$rc"
  if [ "$rc" -ne 0 ] || [ "$nfail" -gt 0 ]; then return 1; fi
  if [ "$npass" -eq 0 ]; then [ "$nskip" -gt 0 ] && return 77; return 1; fi
  if [ "$nskip" -gt 0 ]; then echo "PARTIAL: some live tests SKIPped (not counted as PASS)" >&2; return 78; fi
  return 0
}
if [ "${1:-}" = "--classify" ]; then
  [ $# -eq 3 ] || { echo "usage: $0 --classify <go-test-log> <go-test-exit-code>" >&2; exit 64; }
  classify "$2" "$3"; exit $?
fi
skip() { echo "SKIP: $*" >&2; exit 77; }
command -v go >/dev/null 2>&1 || skip "go toolchain not found"
command -v systemd-run >/dev/null 2>&1 || skip "systemd-run not found (memory-capped scope required)"
MAXLOAD="${SCALING_LIVE_MAX_LOAD:-14}"
for _ in $(seq 1 30); do
  load="$(cut -d' ' -f1 /proc/loadavg)"
  if awk -v l="$load" -v m="$MAXLOAD" 'BEGIN{exit !(l<=m)}'; then break; fi
  echo "host load $load > $MAXLOAD, waiting 10s" >&2; sleep 10
done
load="$(cut -d' ' -f1 /proc/loadavg)"
awk -v l="$load" -v m="$MAXLOAD" 'BEGIN{exit !(l<=m)}' || skip "host load $load stays above $MAXLOAD"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
export SCALING_LIVE_ARTIFACT_DIR="${SCALING_LIVE_ARTIFACT_DIR:-$ROOT/.scaling-live-artifacts/$TS}"
mkdir -p "$SCALING_LIVE_ARTIFACT_DIR"
LOG="$SCALING_LIVE_ARTIFACT_DIR/go-test.log"
RUN="${SCALING_LIVE_RUN:-TestLive}"
cd "$ROOT/catalog-api" || exit 1
set +e
systemd-run --user --scope -q -p MemoryMax="${SCALING_LIVE_MEMMAX:-4G}" -p MemorySwapMax=0 \
  go test -tags live -count=1 -timeout 25m -v -run "$RUN" . 2>&1 | tee "$LOG"
rc=${PIPESTATUS[0]}
echo "artifacts: $SCALING_LIVE_ARTIFACT_DIR"
classify "$LOG" "$rc"
exit $?
