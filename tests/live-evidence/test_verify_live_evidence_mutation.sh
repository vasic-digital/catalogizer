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
RS="codeload hash_malformed fp_malformed expected_unknown exe_path kind_mismatch fp_missing endpoint_missing runs_below_3 resolve_error answering_missing double cmdline_missing expected_missing identity_mismatch script_missing script_unbound fp_mismatch schema timestamp class verdict skip_reason port runs_missing runs_ident"
for r in $RS; do
  sed "/# R:$r\$/c\\      empty," "$src" > "$M/scripts/test-infra/r_$r.sh"
  cmp -s "$M/scripts/test-infra/r_$r.sh" "$src" && { echo "FAIL: reason mutant $r identical (marker lost)"; exit 1; }
done
# register BLIND check removed
sed '/has no rules/d' "$src" > "$M/scripts/test-infra/r_blind.sh"
cmp -s "$M/scripts/test-infra/r_blind.sh" "$src" && { echo "FAIL: blind mutant identical"; exit 1; }
# name match made case-sensitive; bare-hex fingerprint binding dropped; ? glob dropped; timestamp range dropped; runtime_hint ignored
sed 's/test(\$r; "i")))$/test($r)))/' "$src" > "$M/scripts/test-infra/r_case.sh"
sed '/^def is_sha/c\def is_sha($s): (($s // "") | tostring | ascii_downcase | startswith("sha256:")) and (norm($s) | test("^[0-9a-f]{64}$"));' "$src" > "$M/scripts/test-infra/r_bare.sh"
sed 's/ | gsub("\\\\?"; ".")//' "$src" > "$M/scripts/test-infra/r_qglob.sh"
sed 's/ and (\$r.timestamp | test("\\\\A\[0-9\]{4}-(0\[1-9\].*60)(\\\\.\[0-9\]+)?(Z|\[+-\](\[01\]\[0-9\]|2\[0-3\]):\[0-5\]\[0-9\])\\\\z"))//' "$src" > "$M/scripts/test-infra/r_tsrange.sh"
sed 's/(\$a.runtime_hint | present) or //' "$src" > "$M/scripts/test-infra/r_hint.sh"
for x in case bare qglob tsrange hint; do cmp -s "$M/scripts/test-infra/r_$x.sh" "$src" && { echo "FAIL: mutant $x identical"; exit 1; }; done
# round-3 class mutants: one normaliser, all rules case-insensitive, image/script/socket binding, real control needle
sed '/^def norm/s/ | ascii_downcase//' "$src" > "$M/scripts/test-infra/r_normcase.sh"
sed '/^def norm/s/ | ltrimstr("sha256:")//' "$src" > "$M/scripts/test-infra/r_normprefix.sh"
sed '/\$cmds | any/s/; "i"//' "$src" > "$M/scripts/test-infra/r_cmdcase.sh"
sed '/\$imgs | any/s/; "i"//' "$src" > "$M/scripts/test-infra/r_imgcase.sh"
sed 's/\$imgs | any(/[] | any(/' "$src" > "$M/scripts/test-infra/r_imgignored.sh"
sed 's/, norm(\$a.script_sha256)\]/]/' "$src" > "$M/scripts/test-infra/r_scripthash.sh"
sed 's/(\$k == "process" or \$k == "socket") and interp/($k == "process") and interp/' "$src" > "$M/scripts/test-infra/r_sockinterp.sh"
sed 's/(\$k != "container") and (\$A.cmdline/($k == "process") and ($A.cmdline/' "$src" > "$M/scripts/test-infra/r_sockcmd.sh"
sed 's/ || \[ "\$(jq -r .\.missed | length. <<<"\$needle")" != 0 \]//' "$src" > "$M/scripts/test-infra/r_needle.sh"
sed 's/norm(\$a.image_digest), //' "$src" > "$M/scripts/test-infra/r_rm1.sh"
sed 's/\$r.runs.identical != true/(\$r.runs.identical | not)/' "$src" > "$M/scripts/test-infra/r_rm6.sh"
sed 's/ and (\$r.endpoint.port != null)) then empty else "endpoint_identity_missing"/) then empty else "endpoint_identity_missing"/' "$src" > "$M/scripts/test-infra/r_rm7.sh"
sed 's/ or \$a.pid < 1 or \$a.pid != (\$a.pid | floor)//' "$src" > "$M/scripts/test-infra/r_pid.sh"
sed 's/\\\\A\[0-9a-f\]{64}\\\\z/[0-9a-f]{64}/' "$src" > "$M/scripts/test-infra/r_hexanchor.sh"
sed 's/(\$img | test(\$r; "i")) or (\$d | test(\$r; "i"))/(\$img + " " + \$d) | test(\$r; "i")/' "$src" > "$M/scripts/test-infra/r_exactimg.sh"
sed 's/ + \[rules("sha256")\[\] | select(hexok(.) | not) | {malformed_sha256_rule: .}\]//' "$src" > "$M/scripts/test-infra/r_shaneedle.sh"
for x in rm1 rm6 rm7 pid hexanchor exactimg shaneedle; do cmp -s "$M/scripts/test-infra/r_$x.sh" "$src" && { echo "FAIL: mutant $x identical"; exit 1; }; done
for x in normcase normprefix cmdcase imgcase imgignored scripthash sockinterp sockcmd needle; do cmp -s "$M/scripts/test-infra/r_$x.sh" "$src" && { echo "FAIL: mutant $x identical"; exit 1; }; done
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
for m in mut1 mut2 r_blind r_rm1 r_rm6 r_rm7 r_pid r_hexanchor r_exactimg r_shaneedle r_normcase r_normprefix r_cmdcase r_imgcase r_imgignored r_scripthash r_sockinterp r_sockcmd r_needle r_case r_bare r_qglob r_tsrange r_hint r_multi r_skip $(for r in $RS; do echo r_$r; done); do
  if VERIFIER=$M/scripts/test-infra/$m.sh "$HERE/test_verify_live_evidence.sh" >/dev/null 2>&1; then echo "FAIL: $m survived (suite did not catch it)"; rc=1; else echo "ok: $m killed"; fi
done
exit "$rc"
