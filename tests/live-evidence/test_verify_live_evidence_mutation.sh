#!/usr/bin/env bash
# Paired mutation test (1.1) of verify_live_evidence.sh: with the identity check removed the verifier suite MUST fail; the unmutated copy MUST pass (control).
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
M=$(mktemp -d "${TMPDIR:-/tmp}/lev-mut.XXXXXX"); trap 'rm -rf "${M:?}"' EXIT
mkdir -p "$M/scripts/test-infra" "$M/docs/qa"
cp "$ROOT/docs/qa/live-evidence.doubles.txt" "$M/docs/qa/"
src=$ROOT/scripts/test-infra/verify_live_evidence.sh
cp "$src" "$M/scripts/test-infra/control.sh"
# mutant 1: identity_reasons always empty
sed '/^def identity_reasons:/,/# END-IDENTITY/c\def identity_reasons: [];' "$src" > "$M/scripts/test-infra/mut1.sh"
# mutant 2: double register ignored
sed 's/^reg=\$(grep.*$/reg=""/' "$src" > "$M/scripts/test-infra/mut2.sh"
# mutants 3..: every identity/invalid reason removed one at a time (marker comments "# R:<name>" in the jq program)
RS="fp_missing endpoint_missing runs_below_3 resolve_error answering_missing double cmdline_missing expected_missing exe_mismatch image_mismatch script_missing script_unbound fp_mismatch schema timestamp class verdict skip_reason port runs_missing runs_ident"
for r in $RS; do
  sed "/# R:$r\$/c\\      empty," "$src" > "$M/scripts/test-infra/r_$r.sh"
  cmp -s "$M/scripts/test-infra/r_$r.sh" "$src" && { echo "FAIL: reason mutant $r identical (marker lost)"; exit 1; }
done
# multi-value guard removed
sed 's/^  if \[ "\$nvals" != 1 \]; then.*$/  :/' "$src" > "$M/scripts/test-infra/r_multi.sh"
cmp -s "$M/scripts/test-infra/r_multi.sh" "$src" && { echo "FAIL: multi mutant identical"; exit 1; }
# --require-live SKIP rule removed
sed 's/^    SKIP) \[ "\$REQUIRE_LIVE".*$/    SKIP) :;;/' "$src" > "$M/scripts/test-infra/r_skip.sh"
cmp -s "$M/scripts/test-infra/r_skip.sh" "$src" && { echo "FAIL: skip mutant identical"; exit 1; }
chmod +x "$M"/scripts/test-infra/*.sh
cmp -s "$M/scripts/test-infra/mut1.sh" "$src" && { echo "FAIL: mutant 1 identical to original (marker lost)"; exit 1; }
cmp -s "$M/scripts/test-infra/mut2.sh" "$src" && { echo "FAIL: mutant 2 identical to original"; exit 1; }
rc=0
VERIFIER=$M/scripts/test-infra/control.sh "$HERE/test_verify_live_evidence.sh" >/dev/null 2>&1 || { echo "FAIL: control (unmutated copy) must pass"; rc=1; }
for m in mut1 mut2 r_multi r_skip $(for r in $RS; do echo r_$r; done); do
  if VERIFIER=$M/scripts/test-infra/$m.sh "$HERE/test_verify_live_evidence.sh" >/dev/null 2>&1; then echo "FAIL: $m survived (suite did not catch it)"; rc=1; else echo "ok: $m killed"; fi
done
exit "$rc"
