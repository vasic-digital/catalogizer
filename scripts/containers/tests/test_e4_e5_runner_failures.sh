#!/usr/bin/env bash
# test_e4_e5_runner_failures.sh - follow-ups E4/E5 (docs/FOLLOW_UPS_NEXT_PHASE.md).
# Drives the REAL scripts (sourced via their own main-guard, or executed) in a mktemp sandbox. The only substitutes are
# toolchain command shims (npm/npx/xvfb-run/podman) that exit with a chosen status: this is a test of the RUNNER's
# failure propagation (gate logic, constitution 11.4.289(C)(2)); it is NOT evidence that the web E2E suite passes.
# ADMIN_PASSWORD is used by NAME only; the value below is a random per-run sentinel, never printed except in the leak check's verdict.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
T="$(mktemp -d "${TMPDIR:-/tmp}/e4e5-test.XXXXXX")"; : "${T:?}"
trap 'rm -rf -- "${T:?}"' EXIT
FAILS=0
ok()  { echo "PASS: $1"; }
bad() { echo "FAIL: $1"; FAILS=$((FAILS+1)); }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
SENT="sentinel-$RANDOM$RANDOM$RANDOM"

SH="$T/shims"; mkdir -p "$SH"
# npm: succeeds for everything. npx: fails only for `playwright test` (the E2E step) when SHIM_E2E_FAIL=1.
printf '#!/usr/bin/env bash\n# fails only for the npm subcommand named in SHIM_NPM_FAIL_ON (e.g. test, build); SHIM_NPM_RC fails every call\n[ -n "${SHIM_NPM_FAIL_ON:-}" ] && [ "${2:-}" = "$SHIM_NPM_FAIL_ON" ] && exit 1\nexit "${SHIM_NPM_RC:-0}"\n' >"$SH/npm"
printf '#!/usr/bin/env bash\n[ "${SHIM_E2E_FAIL:-0}" = 1 ] && { echo "e2e: 1 failed"; exit 1; }\nexit 0\n' >"$SH/npx"
printf '#!/usr/bin/env bash\nexec "$@"\n' >"$SH/xvfb-run"
chmod +x "$SH"/*

# ---- E4a: build-test-release.sh phase_4 ----
P="$T/project"; mkdir -p "$P/catalog-web/dist" "$P/catalog-api" "$P/reports" "$P/releases"; : >"$P/catalog-web/dist/index.html"
printf '#!/usr/bin/env bash\nsleep 30\n' >"$P/catalog-api/catalog-api"; chmod +x "$P/catalog-api/catalog-api"
phase4() { # $@ = env assignments; prints the PHASE_RESULTS array and exit code
  ( cd "$T" && env PATH="$SH:$PATH" BTR_PROJECT_DIR="$P" "$@" bash -c '
      source "$1/scripts/build-test-release.sh" >/dev/null 2>&1
      sleep() { :; }
      rc=0; phase_4_web_frontend >"$2/phase4.out" 2>&1 || rc=$?
      echo "RC=$rc"; printf "RES=%s\n" "${PHASE_RESULTS[@]:-}"; printf "NOTE=%s\n" "${PHASE_NOTES[@]:-}"' _ "$REPO" "$T" ) 2>"$T/p4.err"
}
OUT="$(phase4 SKIP_E2E_TESTS=false ADMIN_PASSWORD="$SENT" SHIM_E2E_FAIL=1)"
echo "$OUT" | grep -q '^RES=FAIL:Web Frontend' && ok "E4a: failing E2E step records Web Frontend FAIL" || bad "E4a: failing E2E not recorded FAIL: $OUT $(cat "$T/p4.err")"
echo "$OUT" | grep -q 'PASS:Web Frontend' && bad "E4a: PASS recorded despite failing E2E" || ok "E4a: no PASS recorded for failing E2E"
echo "$OUT" | grep -q '^RC=0' && bad "E4a: phase returned 0 on failing E2E" || ok "E4a: phase returns non-zero on failing E2E"
OUT="$(phase4 SKIP_E2E_TESTS=false ADMIN_PASSWORD="$SENT" SHIM_E2E_FAIL=0)"
echo "$OUT" | grep -q '^RES=PASS:Web Frontend' && ok "E4a: passing steps still record PASS (no false FAIL)" || bad "E4a: golden-good not PASS: $OUT $(cat "$T/p4.err")"
OUT="$(phase4 SKIP_E2E_TESTS=false SHIM_E2E_FAIL=1 ADMIN_PASSWORD=)"
grep -q 'credentials_absent' "$T/phase4.out" && ok "E4a: unset ADMIN_PASSWORD is SKIP credentials_absent" || bad "E4a: no credentials_absent SKIP: $(cat "$T/phase4.out")"
echo "$OUT" | grep -q 'PASS:Web Frontend' && ok "E4a: skipped E2E does not fail the other steps" || bad "E4a: skip case: $OUT"
OUT="$(phase4 SKIP_E2E_TESTS=false ADMIN_PASSWORD="$SENT" SHIM_NPM_FAIL_ON=test)"
echo "$OUT" | grep -q '^RES=FAIL:Web Frontend' && ok "E4a: failing unit-test step records FAIL" || bad "E4a: unit failure not FAIL: $OUT"

grep -q '^NOTE=Web Frontend: E2E SKIPPED reason=credentials_absent' <<<"$(phase4 SKIP_E2E_TESTS=false ADMIN_PASSWORD=)" && ok "E4a: skipped E2E leaves a SKIP note" || bad "E4a: no SKIP note"
# ambient PROJECT_DIR must not steer the script's rm -rf: main() with PROJECT_DIR=$T/victim must not delete victim/releases
mkdir -p "$T/victim/releases"; : >"$T/victim/releases/keep"
( cd "$T" && PROJECT_DIR="$T/victim" PATH="$SH:$PATH" bash "$REPO/scripts/build-test-release.sh" >/dev/null 2>&1 ) || true
[ -e "$T/victim/releases/keep" ] && ok "E4a: ambient PROJECT_DIR is not honoured (no rm -rf of it)" || bad "E4a: ambient PROJECT_DIR steered rm -rf"

# ---- E4b: run-all-tests-comprehensive.sh run_e2e_tests ----
C="$T/cwd"; mkdir -p "$C/catalog-web"
e2e() { ( cd "$C" && env PATH="$SH:$PATH" "$@" bash -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; run_e2e_tests >"$2/e2e.out" 2>&1; echo "OVERALL=$OVERALL_EXIT_CODE"' _ "$REPO" "$T" ); }
check "E4b: failing E2E sets OVERALL_EXIT_CODE=1" "$(e2e ADMIN_PASSWORD="$SENT" SHIM_NPM_FAIL_ON=test:e2e)" "OVERALL=1"
check "E4b: passing E2E leaves OVERALL_EXIT_CODE=0" "$(e2e ADMIN_PASSWORD="$SENT" SHIM_NPM_RC=0)" "OVERALL=0"
e2e SHIM_NPM_FAIL_ON=test:e2e ADMIN_PASSWORD= >/dev/null
grep -q 'credentials_absent' "$T/e2e.out" && ok "E4b: unset ADMIN_PASSWORD is SKIP credentials_absent" || bad "E4b: no SKIP: $(cat "$T/e2e.out")"
grep -q 'E2E tests passed' "$T/e2e.out" && bad "E4b: printed passed on skip" || ok "E4b: skip does not print 'E2E tests passed'"

# summary honesty: E2E skipped => TEST_SUMMARY.md must not claim a plain PASSED / checked E2E
e2esum() { ( cd "$C" && env PATH="$SH:$PATH" "$@" bash -c 'source "$1/scripts/run-all-tests-comprehensive.sh" >/dev/null 2>&1; run_e2e_tests >/dev/null 2>&1; generate_test_summary >/dev/null 2>&1; cat "${REPORTS_DIR}/${TIMESTAMP}/TEST_SUMMARY.md"' _ "$REPO" ); }
SUM="$(e2esum ADMIN_PASSWORD=)"
grep -q 'SKIPPED' <<<"$SUM" && grep -q 'credentials_absent' <<<"$SUM" && ok "E4b: summary shows E2E SKIPPED with reason" || bad "E4b: summary lacks SKIP+reason"
grep -qE '^\*\*Status:\*\* ✅ PASSED$' <<<"$SUM" && bad "E4b: summary claims plain PASSED with E2E skipped" || ok "E4b: summary does not claim plain PASSED"
grep -q '✅ E2E Tests' <<<"$SUM" && bad "E4b: summary ticks E2E when skipped" || ok "E4b: summary does not tick E2E when skipped"
SUM="$(e2esum ADMIN_PASSWORD="$SENT" SHIM_NPM_FAIL_ON=test:e2e)"
grep -q 'FAILED' <<<"$SUM" && ok "E4b: summary shows FAILED for failing E2E" || bad "E4b: summary does not show FAILED"
SUM="$(e2esum ADMIN_PASSWORD="$SENT")"
grep -qE '^\*\*Status:\*\* ✅ PASSED$' <<<"$SUM" && grep -q '✅ E2E Tests' <<<"$SUM" && ok "E4b: golden-good summary is PASSED with E2E ticked" || bad "E4b: golden-good summary wrong"

# ---- E5: run_pinned.sh forwards ADMIN_PASSWORD by NAME ----
D1="sha256:$(printf 'a%.0s' $(seq 64))"; export SHIM_INSPECT_DIGEST="$D1"
cat >"$SH/podman" <<'SHX'
#!/usr/bin/env bash
case "$*" in *GraphRoot*) echo "${SHIM_GRAPHROOT:-/nonexistent}";; "image inspect"*) echo "${SHIM_INSPECT_DIGEST:-}"; esac; exit 0
SHX
printf '#!/usr/bin/env bash\necho Avail; echo 900000000000\n' >"$SH/df"; chmod +x "$SH/podman" "$SH/df"
mkdir -p "$T/home" "$T/graph" "$T/disk" "$T/ck/docs" "$T/ck/.audit/scratch"
printf 'schema: 1\nimages:\n- id: IMG-TOOL\n  reference: docker.io/example/tool\n  tag_intent: "1"\n  digest: "%s"\n  size_bytes: 1000\n' "$D1" >"$T/lock.yaml"
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
runp() { ( cd "$T/ck" && env -u ADMIN_PASSWORD -u RUNP_PASS_ENV "$@" HOME="$T/home" PATH="$SH:$PATH" SHIM_GRAPHROOT="$T/graph" DISK_HEADROOM_OUT_DIR="$T/disk" \
   RUNP_TEST_MODE=1 RUNP_MEMINFO="$T/meminfo" RUNP_LOCK="$T/lock.yaml" RUNP_PRINT_ARGV=1 bash "$REPO/scripts/containers/run_pinned.sh" --op-id e5 IMG-TOOL -- true ) >"$T/argv" 2>"$T/argv.err"; RC=$?; }
has_e() { local i; i="$(grep -nxF ADMIN_PASSWORD "$T/argv" | head -1 | cut -d: -f1)"; [ -n "$i" ] && [ "$(sed -n "$((i-1))p" "$T/argv")" = "-e" ]; }
runp ADMIN_PASSWORD="$SENT" RUNP_PASS_ENV=ADMIN_PASSWORD
has_e && ok "E5: allow-listed RUNP_PASS_ENV=ADMIN_PASSWORD forwards '-e ADMIN_PASSWORD' (name only)" || bad "E5: not forwarded when allow-listed (rc $RC, $(head -c 200 "$T/argv.err"))"
grep -qF -- "$SENT" "$T/argv" "$T/argv.err" && bad "E5: password VALUE leaked into argv/stderr" || ok "E5: password value absent from argv and stderr"
runp ADMIN_PASSWORD="$SENT"
has_e && bad "E5: least privilege: forwarded although RUNP_PASS_ENV not set" || ok "E5: least privilege: ambient ADMIN_PASSWORD is NOT forwarded without RUNP_PASS_ENV"
runp RUNP_PASS_ENV=ADMIN_PASSWORD
has_e && bad "E5: forwarded although unset" || ok "E5: allow-listed but unset: nothing forwarded (no default)"
runp ADMIN_PASSWORD="$SENT" RUNP_PASS_ENV=HOME_SECRET
check "E5: non-allow-listed name is refused" "$RC" 1
grep -q 'reason=pass_env_not_allowed' "$T/argv.err" && ok "E5: refusal reason pass_env_not_allowed" || bad "E5: refusal reason: $(head -c 200 "$T/argv.err")"

# ---- E5 wrapper plumbing: only run_playwright hands ADMIN_PASSWORD to run_pinned; other wrappers/ambient get nothing ----
cat >"$SH/rp_shim.sh" <<'SHX'
#!/usr/bin/env bash
printf 'PASSENV=[%s]\n' "${RUNP_PASS_ENV-}" >>"${SHIM_LOG:?}"
prev=""; for a in "$@"; do [ "$prev" = --out ] && mkdir -p -- "$a"; prev=$a; done
case " $* " in *" --version "*|*cpa-probe*) printf 'version=x\nout_writable=yes\ncache_writable=yes\nsrc_readonly=yes\n';; esac
exit 0
SHX
printf '#!/usr/bin/env bash\necho "AM-P1 clean"\n' >"$SH/sweep.sh"; chmod +x "$SH/rp_shim.sh" "$SH/sweep.sh"
printf 'schema: 1\nimages:\n- id: IMG-GO\n  reference: docker.io/example/go\n  tag_intent: "1"\n  digest: "%s"\n- id: IMG-PW\n  reference: docker.io/example/pw\n  tag_intent: "1"\n  digest: "%s"\n' "$D1" "$D1" >"$T/lock2.yaml"
mkdir -p "$T/wck" "$T/lo/repo/.audit"
wrap() { w=$1; shift; : >"$T/wlog"; ( cd "$T/wck" && env "$@" RUNNER_TEST_MODE=1 RUNNER_RUNP="$SH/rp_shim.sh" RUNNER_SWEEP="$SH/sweep.sh" RUNP_LOCK="$T/lock2.yaml" SHIM_LOG="$T/wlog" \
  ENVELOPE_TEST_MODE=1 ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000 LONGOPS_REPO="$T/lo/repo" LONGOPS_DIR="$T/lo/repo/.audit/longops" LONGOPS_AUDIT="$T/lo/repo/.audit" LONGOPS_ALLOW_TMPFS=1 \
  DISK_HEADROOM_OUT_DIR="$T/disk" PATH="$SH:$PATH" bash "$REPO/scripts/containers/$w.sh" --out "$T/wout-$w" -- true ) >"$T/wout.txt" 2>&1; }
wrap run_playwright ADMIN_PASSWORD="$SENT"
grep -qx 'PASSENV=\[ADMIN_PASSWORD\]' "$T/wlog" && ok "E5: run_playwright hands RUNP_PASS_ENV=ADMIN_PASSWORD to run_pinned" || bad "E5: run_playwright plumbing: $(cat "$T/wlog") $(tail -3 "$T/wout.txt")"
wrap run_go ADMIN_PASSWORD="$SENT" RUNNER_PASS_ENV=ADMIN_PASSWORD RUNP_PASS_ENV=ADMIN_PASSWORD
if [ -s "$T/wlog" ] && ! grep -v '^PASSENV=\[\]$' "$T/wlog" | grep -q .; then ok "E5: run_go (other tool) gets nothing, even with ambient RUNNER_PASS_ENV/RUNP_PASS_ENV"; else bad "E5: run_go leaked or did not run: $(cat "$T/wlog") $(tail -3 "$T/wout.txt")"; fi

echo "failures: $FAILS"; [ "$FAILS" -eq 0 ]
