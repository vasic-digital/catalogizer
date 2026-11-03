#!/usr/bin/env bash
# scaling_live.sh - one-shot runner for the LIVE scaling + stress + thin-chaos suite of catalog-api.
# Guide: docs/TESTING_GUIDE.md section "Scaling and chaos (live)".
#
# Builds the REAL catalog-api binary and runs it against a REAL SQLCipher SQLite file on loopback
# (ephemeral port). Excluded from the default `go test ./...` by the `live` build tag.
#
# Exit codes: 0 PASS | 1 FAIL | 77 SKIP (real dependency absent: no go toolchain / cannot build /
# no systemd-run / host load too high) - SKIP is never PASS.
#
# Env: SCALING_LIVE_ARTIFACT_DIR (default: <repo>/.scaling-live-artifacts/<utc-ts>)
#      SCALING_LIVE_RUN (go test -run regex, default TestLive)
#      SCALING_P99_MS / SCALING_P99_FACTOR / SCALING_P99_FLOOR_MS / SCALING_RSS_GROWTH_KB (see guide)
#      SCALING_LIVE_MAX_LOAD (default 14), SCALING_LIVE_MEMMAX (default 4G)
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
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
set -e
echo "artifacts: $SCALING_LIVE_ARTIFACT_DIR"
npass=$(grep -c -- '^--- PASS: TestLive' "$LOG" || true)
nskip=$(grep -c -- '^--- SKIP: TestLive' "$LOG" || true)
nfail=$(grep -c -- '^--- FAIL: TestLive' "$LOG" || true)
echo "live tests: pass=$npass skip=$nskip fail=$nfail rc=$rc"
if [ "$rc" -ne 0 ] || [ "$nfail" -gt 0 ]; then exit 1; fi
if [ "$npass" -eq 0 ]; then [ "$nskip" -gt 0 ] && exit 77; exit 1; fi
if [ "$nskip" -gt 0 ]; then echo "partial: some live tests SKIPped (not counted as PASS)" >&2; fi
exit 0
