#!/usr/bin/env bash
# test_btr_swallowed_failures.sh - register CAT-014: build-test-release.sh must not swallow lint/type-check/checksum failures.
# Drives the REAL script (sourced via its main-guard) in a mktemp sandbox (BTR_PROJECT_DIR). Substitutes are toolchain shims
# (npm, npx, sha256sum) that exit with a chosen status: a test of the script's failure propagation (gate logic, 11.4.289(C)(2)).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
BASH_BIN="$(command -v bash)"
T="$(mktemp -d "${TMPDIR:-/tmp}/btr-swallow.XXXXXX")"; : "${T:?}"
trap 'rm -rf -- "${T:?}"' EXIT
FAILS=0
ok()  { echo "PASS: $1"; }
bad() { echo "FAIL: $1"; FAILS=$((FAILS+1)); }

SH="$T/shims"; mkdir -p "$SH"
printf '#!/usr/bin/env bash\n[ -n "${SHIM_NPM_FAIL_ON:-}" ] && [ "${2:-}" = "$SHIM_NPM_FAIL_ON" ] && exit 1\nexit 0\n' >"$SH/npm"
printf '#!/usr/bin/env bash\nexit 0\n' >"$SH/npx"
printf '#!/usr/bin/env bash\n[ "${SHIM_SHA_FAIL:-0}" = 1 ] && exit 1\nexec "$SHIM_REAL_SHA" "$@"\n' >"$SH/sha256sum"
chmod +x "$SH"/*
REAL_SHA="$(command -v sha256sum)"

P="$T/project"; mkdir -p "$P/catalog-web/dist" "$P/catalog-api" "$P/reports" "$P/releases"; : >"$P/catalog-web/dist/index.html"
phase() { # $1 = phase fn; rest = env. prints RC, RES lines, NOTE lines
  local fn=$1; shift
  ( cd "$T" && env PATH="$SH:$PATH" BTR_PROJECT_DIR="$P" SKIP_E2E_TESTS=true SHIM_REAL_SHA="$REAL_SHA" "$@" "$BASH_BIN" -c '
      source "$1/scripts/build-test-release.sh" >/dev/null 2>&1
      rc=0; '"$fn"' >"$2/phase.out" 2>&1 || rc=$?
      echo "RC=$rc"; printf "RES=%s\n" "${PHASE_RESULTS[@]:-}"; printf "NOTE=%s\n" "${PHASE_NOTES[@]:-}"' _ "$REPO" "$T" ) 2>"$T/phase.err"
}
# type-check
O="$(phase phase_4_web_frontend SHIM_NPM_FAIL_ON=type-check)"
grep -q '^RES=FAIL:Web Frontend' <<<"$O" && ! grep -q '^RC=0' <<<"$O" && ok "failing type-check fails the Web Frontend phase" || bad "type-check failure swallowed: $O $(cat "$T/phase.err")"
# lint: surfaced as note, phase not failed
O="$(phase phase_4_web_frontend SHIM_NPM_FAIL_ON=lint)"
grep -q '^NOTE=Web Frontend: lint FAILED' <<<"$O" && ok "failing lint is surfaced as a phase note" || bad "lint failure silent: $O"
grep -q '^RES=PASS:Web Frontend' <<<"$O" && ok "lint findings alone do not fail the phase (run-all parity)" || bad "lint made phase fail: $O"
# golden good
O="$(phase phase_4_web_frontend)"
grep -q '^RES=PASS:Web Frontend' <<<"$O" && grep -q '^RC=0' <<<"$O" && ! grep -q '^NOTE=Web Frontend: lint' <<<"$O" && ok "golden-good: clean lint/type-check -> PASS, no note" || bad "golden-good wrong: $O $(cat "$T/phase.err")"
# phase 9 checksum
: >"$P/releases/catalog-api-test"
O="$(phase phase_9_collect_artifacts SHIM_SHA_FAIL=1)"; cp "$T/phase.out" "$T/p9.out"
grep -q '^RES=FAIL:Artifact Collection' <<<"$O" && ! grep -q '^RC=0' <<<"$O" && ok "failing sha256sum fails Artifact Collection" || bad "checksum failure swallowed: $O $(tail -5 "$T/phase.out") $(cat "$T/phase.err")"
O="$(phase phase_9_collect_artifacts SHIM_SHA_FAIL=0)"
grep -q '^RES=PASS:Artifact Collection' <<<"$O" && [ -s "$P/releases/SHA256SUMS.txt" ] && grep -q 'catalog-api-test' "$P/releases/SHA256SUMS.txt" && ok "golden-good: checksums generated, phase PASS" || bad "golden-good phase 9: $O $(cat "$T/phase.err")"

echo "failures: $FAILS"; [ "$FAILS" -eq 0 ]
