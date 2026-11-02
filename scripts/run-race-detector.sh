#!/usr/bin/env bash
# shellcheck source=/dev/null
. "$(dirname "$(readlink -f "${BASH_SOURCE[0]:-$0}")")/lib/cgo_flags.sh" || { echo "FATAL: lib/cgo_flags.sh missing (cgo define for go-sqlcipher)" >&2; exit 1; }   # 2026-11-02 decision; scripts/lib/cgo_flags.sh
# run-race-detector.sh — Run Go race detector across catalog-api and every
# Go submodule. Exits non-zero on any race or test failure.
#
# REQUIRES: no sudo. User-level Go toolchain only.
# Usage: ./scripts/run-race-detector.sh [--fast|--submodules-only|--api-only]
#
# The script honors CLAUDE.md resource limits:
#   GOMAXPROCS=3 go test ./... -p 2 -parallel 2

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

MODE="${1:---all}"
FAIL_COUNT=0
PASS_COUNT=0
SKIP_COUNT=0
FAILED_MODULES=()

# ANSI colors for the terminal. Strip when not a TTY (e.g., CI logs).
if [[ -t 1 ]]; then
  RED=$'\033[31m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; DIM=$'\033[2m'; RESET=$'\033[0m'
else
  RED=""; GREEN=""; YELLOW=""; DIM=""; RESET=""
fi

log_info() { echo "${DIM}[info]${RESET} $*"; }
log_ok()   { echo "${GREEN}[ ok ]${RESET} $*"; }
log_fail() { echo "${RED}[fail]${RESET} $*"; }
log_skip() { echo "${YELLOW}[skip]${RESET} $*"; }

# Go submodules wired into catalog-api via `replace` directives in go.mod.
# Keeping this list here (rather than parsing go.mod) makes the script
# intentional — adding a new submodule requires updating the script, which
# forces the author to decide whether the new module should be race-tested.
GO_SUBMODULES=(
  "Assets"
  "Auth"
  "Cache"
  "Challenges"
  "Concurrency"
  "Config"
  "Containers"
  "Database"
  "Discovery"
  "Entities"
  "EventBus"
  "Filesystem"
  "Lazy"
  "Media"
  "Memory"
  "Middleware"
  "Observability"
  "RateLimiter"
  "Recovery"
  "Security"
  "Storage"
  "Streaming"
  "Watcher"
)

run_module() {
  local path="$1"
  local label="$2"

  if [[ ! -d "$path" ]]; then
    log_skip "$label (directory not found)"
    SKIP_COUNT=$((SKIP_COUNT + 1))
    return
  fi

  if [[ ! -f "$path/go.mod" ]]; then
    log_skip "$label (no go.mod)"
    SKIP_COUNT=$((SKIP_COUNT + 1))
    return
  fi

  log_info "Running race detector on $label"
  local logfile
  logfile="$(mktemp -t race_${label//\//_}.XXXXXX.log)"

  if (cd "$path" && GOMAXPROCS=3 GOTOOLCHAIN=local \
       go test -race ./... -p 2 -parallel 2 -count=1 -timeout 300s) \
       >"$logfile" 2>&1; then
    log_ok "$label"
    PASS_COUNT=$((PASS_COUNT + 1))
    rm -f "$logfile"
  else
    log_fail "$label (see $logfile for details)"
    tail -30 "$logfile" | sed "s/^/    /"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    FAILED_MODULES+=("$label")
  fi
}

# --- Dispatch ---

case "$MODE" in
  --fast)
    run_module "catalog-api" "catalog-api"
    ;;
  --api-only)
    run_module "catalog-api" "catalog-api"
    ;;
  --submodules-only)
    for m in "${GO_SUBMODULES[@]}"; do
      run_module "$m" "$m"
    done
    ;;
  --all|*)
    run_module "catalog-api" "catalog-api"
    for m in "${GO_SUBMODULES[@]}"; do
      run_module "$m" "$m"
    done
    ;;
esac

# --- Summary ---

echo
echo "${DIM}────────────────────────────────────────${RESET}"
echo "Race detector summary"
echo "  ${GREEN}passed:${RESET}  $PASS_COUNT"
echo "  ${RED}failed:${RESET}  $FAIL_COUNT"
echo "  ${YELLOW}skipped:${RESET} $SKIP_COUNT"

if [[ $FAIL_COUNT -gt 0 ]]; then
  echo
  echo "${RED}Failed modules:${RESET}"
  for m in "${FAILED_MODULES[@]}"; do
    echo "  - $m"
  done
  exit 1
fi

echo
echo "${GREEN}All modules clean under -race.${RESET}"
exit 0
