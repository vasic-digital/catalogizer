#!/usr/bin/env bash
# WP-02 fix round 8 test (review R3-5, TDD): a mutant run whose test exits 77 (a skipped leg) is NOT a kill.
#
# Purpose   H1-H4 unit-test mut_verdict.sh; H5-H7 drive scripts/audit/tests/mutate_index_health.sh at runtime with stub tests (env MUT_TESTS) that
#           exit 0 for the unmutated script, and 77 / 1 / 0 for a mutant, and assert the harness reports SKIPPED-NOT-KILLED / CAUGHT / SURVIVED and
#           its exit status (only CAUGHT is success).
# Usage     bash scripts/audit/tests/test_mutation_harness_skip77.sh      Env: HARNESS=<harness under test> (default mutate_index_health.sh)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
HARNESS="${HARNESS:-scripts/audit/tests/mutate_index_health.sh}"
T="$(mktemp -d "${TMPDIR:-/tmp}/mut77.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
if [ -f scripts/audit/tests/mut_verdict.sh ]; then . scripts/audit/tests/mut_verdict.sh; else echo "NOTE: mut_verdict.sh absent (RED state)"; mut_combine() { echo 1; }; mut_class() { echo CAUGHT; }; fi
[ "$(mut_combine 0 0 0)" = 0 ] && ok "H1 all zero -> 0" || bad "H1"
[ "$(mut_combine 0 77 0)" = 77 ] && ok "H2 a 77 among zeros -> 77 (skipped, not a kill)" || bad "H2 got $(mut_combine 0 77 0)"
[ "$(mut_combine 77 1 0)" = 1 ] && ok "H3 a real failure beside a 77 -> 1 (killed)" || bad "H3 got $(mut_combine 77 1 0)"
[ "$(mut_class 77)" = SKIPPED-NOT-KILLED ] && [ "$(mut_class 1)" = CAUGHT ] && [ "$(mut_class 0)" = SURVIVED ] && ok "H4 classes: 77 SKIPPED-NOT-KILLED, 1 CAUGHT, 0 SURVIVED" || bad "H4"
# stub test: exit by mode when IH is set (a mutant run), 0 otherwise (the baseline)
cat > "$T/stub.sh" <<'S'
#!/usr/bin/env bash
[ -n "${IH:-}" ] || exit 0
exit "${STUB_RC:-0}"
S
run() { STUB_RC="$1" MUT_TESTS="$T/stub.sh" ONLY='^m01-' bash "$HARNESS" >"$T/out" 2>&1; RC=$?; }
run 77; { grep -q '^SKIPPED-NOT-KILLED m01' "$T/out" && ! grep -q '^CAUGHT' "$T/out" && [ "$RC" -ne 0 ]; } && ok "H5 stub exits 77 under the mutant -> SKIPPED-NOT-KILLED, harness not successful" || bad "H5 rc=$RC $(grep -E '^(CAUGHT|SKIPPED|SURVIVED|MUTATIONS)' "$T/out" | head -3 | tr '\n' ' ')"
run 1;  { grep -q '^CAUGHT   m01' "$T/out" && [ "$RC" -eq 0 ]; } && ok "H6 stub exits 1 under the mutant -> CAUGHT, harness success" || bad "H6 rc=$RC $(grep -E '^(CAUGHT|SKIPPED|SURVIVED|MUTATIONS)' "$T/out" | head -3 | tr '\n' ' ')"
run 0;  { grep -q '^SURVIVED m01' "$T/out" && [ "$RC" -ne 0 ]; } && ok "H7 stub exits 0 under the mutant -> SURVIVED, harness not successful" || bad "H7 rc=$RC $(grep -E '^(CAUGHT|SKIPPED|SURVIVED|MUTATIONS)' "$T/out" | head -3 | tr '\n' ' ')"
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
