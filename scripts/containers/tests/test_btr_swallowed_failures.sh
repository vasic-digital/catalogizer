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
printf '#!/usr/bin/env bash\n[ -n "${SHIM_NPM_FAIL_ON:-}" ] && { [ "${2:-}" = "$SHIM_NPM_FAIL_ON" ] || [ "${1:-}" = "$SHIM_NPM_FAIL_ON" ]; } && exit 1\nexit 0\n' >"$SH/npm"
printf '#!/usr/bin/env bash\nexit 0\n' >"$SH/npx"
printf '#!/usr/bin/env bash\n[ "${SHIM_SHA_FAIL:-0}" = 1 ] && exit 1\ncase "$*" in *"${SHIM_SHA_FAIL_MATCH:-@@none@@}"*) exit 1;; esac\nexec "$SHIM_REAL_SHA" "$@"\n' >"$SH/sha256sum"
printf '#!/usr/bin/env bash\n# fails when SHIM_GO_FAIL_ON equals the go subcommand (test/build/mod)\n[ -n "${SHIM_GO_FAIL_ON:-}" ] && [ "${1:-}" = "$SHIM_GO_FAIL_ON" ] && exit 1\nexit 0\n' >"$SH/go"
printf '#!/usr/bin/env bash\n[ "${SHIM_ADB_CONNECT_FAIL:-0}" = 1 ] && [ "${1:-}" = connect ] && exit 1\ncase "$*" in *dumpsys*) [ "${SHIM_ADB_NOLAUNCH:-0}" = 1 ] || echo catalogizer;; esac\nexit 0\n' >"$SH/adb"
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

# ---------------- F1: every phase records FAIL (not PASS) on a failing step; absent tools are SKIP ----------------
# The phases are called exactly as main() calls them (`fn || ...`), which disables set -e inside the function.
for d in catalogizer-api-client catalog-api catalogizer-desktop installer-wizard catalogizer-android catalogizer-androidtv; do mkdir -p "$P/$d"; done
phasex() { # $1 = fn; rest = env. sleep is stubbed so the emulator phase is fast
  local fn=$1; shift
  ( cd "$T" && env PATH="$SH:$PATH" BTR_PROJECT_DIR="$P" SKIP_E2E_TESTS=true SHIM_REAL_SHA="$REAL_SHA" "$@" "$BASH_BIN" -c '
      source "$1/scripts/build-test-release.sh" >/dev/null 2>&1
      sleep() { :; }
      rc=0; '"$fn"' >"$2/phase.out" 2>&1 || rc=$?
      echo "RC=$rc"; printf "RES=%s\n" "${PHASE_RESULTS[@]:-}"; printf "NOTE=%s\n" "${PHASE_NOTES[@]:-}"' _ "$REPO" "$T" ) 2>"$T/phase.err"
}
expect() { # $1 label, $2 output, $3 want RES, $4 want rc: 0 = zero, 1 = non-zero
  local rc_ok=0
  if [ "$4" = 0 ]; then grep -q '^RC=0' <<<"$2" && rc_ok=1; else grep -q '^RC=0' <<<"$2" || rc_ok=1; fi
  if grep -qxF "RES=$3" <<<"$2" && [ "$rc_ok" = 1 ]; then ok "$1"; else bad "$1: $(head -3 <<<"$2") $(tail -3 "$T/phase.err")"; fi
}
# phase 2
expect "F1 phase 2: failing npm test -> FAIL:API Client" "$(phasex phase_2_api_client SHIM_NPM_FAIL_ON=test)" "FAIL:API Client" 1
expect "F1 phase 2: failing npm run build -> FAIL" "$(phasex phase_2_api_client SHIM_NPM_FAIL_ON=run)" "FAIL:API Client" 1
expect "F1 phase 2: golden-good -> PASS" "$(phasex phase_2_api_client)" "PASS:API Client" 0
# phase 3
expect "F1 phase 3: failing go test -> FAIL:Backend API" "$(phasex phase_3_backend_api SHIM_GO_FAIL_ON=test)" "FAIL:Backend API" 1
expect "F1 phase 3: failing go build -> FAIL" "$(phasex phase_3_backend_api SHIM_GO_FAIL_ON=build)" "FAIL:Backend API" 1
expect "F1 phase 3: golden-good -> PASS" "$(phasex phase_3_backend_api)" "PASS:Backend API" 0
# phase 5 (a successful tauri build leaves a bundle dir; the shim cannot, so the sandbox provides one)
for d in catalogizer-desktop installer-wizard; do mkdir -p "$P/$d/src-tauri/target/release/bundle"; : >"$P/$d/src-tauri/target/release/bundle/$d.deb"; done
expect "F1 phase 5: failing desktop npm test -> FAIL:Desktop Apps" "$(phasex phase_5_desktop_apps SHIM_NPM_FAIL_ON=run)" "FAIL:Desktop Apps" 1
expect "F1 phase 5: only the unit-test step failing -> FAIL (not masked by the build step)" "$(phasex phase_5_desktop_apps SHIM_NPM_FAIL_ON=test)" "FAIL:Desktop Apps" 1
expect "F1 phase 5: failing npm install -> FAIL" "$(phasex phase_5_desktop_apps SHIM_NPM_FAIL_ON=install)" "FAIL:Desktop Apps" 1
expect "F1 phase 5: golden-good -> PASS" "$(phasex phase_5_desktop_apps)" "PASS:Desktop Apps" 0
# phase 6: gradlew shim in both android projects
GW='#!/usr/bin/env bash\n[ -n "${SHIM_GRADLE_FAIL_ON:-}" ] && [ "${1:-}" = "$SHIM_GRADLE_FAIL_ON" ] && exit 1\nexit 0\n'
expect "F1 phase 6: no gradlew anywhere -> SKIP (not PASS) with a note" "$(phasex phase_6_android_apps)" "SKIP:Android Apps" 0
grep -q 'NOTE=Android Apps: catalogizer-android SKIPPED reason=tool_absent' <<<"$(phasex phase_6_android_apps)" && ok "F1 phase 6: absent gradlew leaves a SKIPPED reason note" || bad "F1 phase 6: no skip note"
for d in catalogizer-android catalogizer-androidtv; do printf "$GW" >"$P/$d/gradlew"; chmod +x "$P/$d/gradlew"; mkdir -p "$P/$d/app/build/outputs/apk"; : >"$P/$d/app/build/outputs/apk/app.apk"; done
expect "F1 phase 6: failing testDebugUnitTest -> FAIL:Android Apps" "$(phasex phase_6_android_apps SHIM_GRADLE_FAIL_ON=testDebugUnitTest)" "FAIL:Android Apps" 1
expect "F1 phase 6: failing assembleRelease -> FAIL" "$(phasex phase_6_android_apps SHIM_GRADLE_FAIL_ON=assembleRelease)" "FAIL:Android Apps" 1
rm -rf -- "${P:?}/releases"; mkdir -p "$P/releases"; : >"$P/releases/android"   # a FILE where the destination dir should be: mkdir and cp -t fail -> FAIL (kills the \; mutant)
expect "F1 phase 6: APK copy failure (destination missing) -> FAIL" "$(phasex phase_6_android_apps)" "FAIL:Android Apps" 1
rm -f -- "${P:?}/releases/android"; mkdir -p "$P/releases/android/catalogizer-android" "$P/releases/android/catalogizer-androidtv"
expect "F1 phase 6: golden-good -> PASS" "$(phasex phase_6_android_apps)" "PASS:Android Apps" 0
[ -f "$P/releases/android/catalogizer-android/app.apk" ] && ok "F1 phase 6: APK actually copied" || bad "F1 phase 6: APK not copied"
# phase 7
expect "F1 phase 7: SKIP_EMULATOR_TESTS=true -> SKIP (not PASS)" "$(phasex phase_7_emulator_tests SKIP_EMULATOR_TESTS=true)" "SKIP:Emulator Tests" 0
expect "F1 phase 7: connect failure -> FAIL" "$(phasex phase_7_emulator_tests SKIP_EMULATOR_TESTS=false SHIM_ADB_CONNECT_FAIL=1)" "FAIL:Emulator Tests" 1
APK="$P/releases/android/catalogizer-android/app.apk"; : >"$APK"
expect "F1 phase 7: app not launched -> FAIL" "$(phasex phase_7_emulator_tests SKIP_EMULATOR_TESTS=false SHIM_ADB_NOLAUNCH=1)" "FAIL:Emulator Tests" 1
rm -f -- "${APK:?}"
expect "F1 phase 7: no APK to install -> SKIP, not PASS" "$(phasex phase_7_emulator_tests SKIP_EMULATOR_TESTS=false)" "SKIP:Emulator Tests" 0
: >"$P/releases/android/catalogizer-android/app.apk"
expect "F1 phase 7: golden-good -> PASS" "$(phasex phase_7_emulator_tests SKIP_EMULATOR_TESTS=false)" "PASS:Emulator Tests" 0
# phase 8
mkdir -p "$P/scripts"
printf '#!/usr/bin/env bash\nexit 1\n' >"$P/scripts/validate-coverage.sh"; chmod +x "$P/scripts/validate-coverage.sh"
expect "F1 phase 8: validator non-zero -> FAIL" "$(phasex phase_8_coverage_validation)" "FAIL:Coverage Validation" 1
printf '#!/usr/bin/env bash\nexit 0\n' >"$P/scripts/validate-coverage.sh"
expect "F1 phase 8: golden-good -> PASS" "$(phasex phase_8_coverage_validation)" "PASS:Coverage Validation" 0
rm -f "$P/scripts/validate-coverage.sh"
expect "F1 phase 8: validator absent -> SKIP (not PASS)" "$(phasex phase_8_coverage_validation)" "SKIP:Coverage Validation" 0

# ---------------- F7/F8: partial or empty SHA256SUMS ----------------
rm -rf -- "${P:?}/releases"; mkdir -p "$P/releases"; : >"$P/releases/catalog-api-test"
phasex phase_9_collect_artifacts SHIM_SHA_FAIL=1 >/dev/null
[ ! -e "$P/releases/SHA256SUMS.txt" ] && [ ! -e "$P/releases/.SHA256SUMS.tmp" ] && ok "F7: failed checksum run leaves no partial SHA256SUMS.txt or temp" || bad "F7: partial checksum file left"
rm -rf -- "${P:?}/releases"; mkdir -p "$P/releases"
expect "F8: zero artifacts -> FAIL, not PASS" "$(phasex phase_9_collect_artifacts)" "FAIL:Artifact Collection" 1
[ ! -e "$P/releases/SHA256SUMS.txt" ] && ok "F8: no empty SHA256SUMS.txt published" || bad "F8: empty SHA256SUMS.txt published"

# one of two artifacts fails to checksum: the phase must FAIL (find -exec \; would hide it) and record FAIL exactly once
rm -rf -- "${P:?}/releases"; mkdir -p "$P/releases"; : >"$P/releases/catalog-api-good"; : >"$P/releases/catalog-api-bad"
O="$(phasex phase_9_collect_artifacts SHIM_SHA_FAIL_MATCH=bad)"
expect "F8: one artifact failing to checksum -> FAIL" "$O" "FAIL:Artifact Collection" 1
[ "$(grep -c '^RES=' <<<"$O")" = 1 ] && ok "F8: FAIL recorded exactly once (no fall-through to a second record)" || bad "F8: phase recorded more than once: $O"
[ ! -e "$P/releases/SHA256SUMS.txt" ] && ok "F7: partial checksum list not published" || bad "F7: partial SHA256SUMS.txt published"

# ---------------- F2/F9: main() still reports after a phase-9 failure ----------------
mainx() { # $@ = env; runs main() with phases 0-8 stubbed, phase 9 and 10 real
  rm -rf -- "${P:?}/releases" "${P:?}/reports"
  ( cd "$T" && env PATH="$SH:$PATH" BTR_PROJECT_DIR="$P" SHIM_REAL_SHA="$REAL_SHA" "$@" "$BASH_BIN" -c '
      source "$1/scripts/build-test-release.sh" >/dev/null 2>&1
      for f in phase_0_signing_keys phase_1_health_checks phase_2_api_client phase_3_backend_api phase_4_web_frontend phase_5_desktop_apps phase_6_android_apps phase_7_emulator_tests phase_8_coverage_validation; do eval "$f(){ record_phase $f PASS; }"; done
      phase_7_emulator_tests(){ PHASE_NOTES+=("Emulator Tests: SKIPPED reason=x"); record_phase Emulator SKIP; }
      main' _ "$REPO" 2>&1; echo "RC=$?" ) | sed 's/\x1b\[[0-9;]*m//g'
}
mkdir -p "$P/releases"; 
O="$(mainx SHIM_SHA_FAIL=1)"
grep -q 'Build Pipeline Summary' <<<"$O" && grep -q 'Phase 10\|Generate Reports' <<<"$O" && [ -s "$P/reports/build-report.json" ] && grep -q '^RC=1' <<<"$O" && grep -q '\[FAIL\] Artifact Collection' <<<"$O" && ok "F2: phase-9 failure still produces report + summary and exits non-zero" || bad "F2: main aborted early: $(tail -12 <<<"$O")"
grep -q '\[NOTE\] Emulator Tests: SKIPPED' <<<"$O" && grep -q '\[SKIP\] Emulator' <<<"$O" && ok "F9: notes print under [NOTE], skipped phases under [SKIP]" || bad "F9: labels wrong: $(tail -12 <<<"$O")"
echo "failures: $FAILS"; [ "$FAILS" -eq 0 ]
