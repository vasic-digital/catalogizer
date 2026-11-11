#!/usr/bin/env bash
# TAURI-R2-FU-4 test: scripts/pre-flight-check.sh must chdir to the repo root regardless of the caller's cwd.
# Uses PRE_FLIGHT_CHDIR_ONLY=1 (prints the resulting cwd and exits before any check). Exit 0 PASS / 1 FAIL.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
D="$(mktemp -d)"; trap 'rm -rf "${D:?}"' EXIT
got="$(cd "$D" && PRE_FLIGHT_CHDIR_ONLY=1 bash "$ROOT/scripts/pre-flight-check.sh")"
[ "$got" = "$ROOT" ] && { echo "PASS: chdir to $got from foreign cwd"; exit 0; }
echo "FAIL: want $ROOT got '$got'" >&2; exit 1
