#!/usr/bin/env bash
# T040a test (TDD): the fixture-root reader scripts/repo/fixture_roots.sh (deliberate-violation roots, abbreviation table).
# Usage   bash scripts/repo/tests/test_fixture_roots.sh      Env: H=<reader path> (the mutation driver points it at a copy)
# Fixtures every fake secret and private-key header is ASSEMBLED AT RUN TIME from fragments in a throwaway tree; no violating
#         file is committed (the deliberate-violation convention binding every WP-04 test).
# Scope   the root-list reader: a listed root exempts a file only from the checks its row names, a file one directory outside
#         is not exempt, a malformed row is refused (20). NOT here: the secret baseline `.secrets.baseline` and the S2
#         secret-fold fixtures (hex filter, baseline drift, private-key carrier list): they need detect-secrets
#         (IMG-TESTUTIL, T006) and scope_check.sh, absent in this slice (see the evidence note).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
H="${H:-scripts/repo/fixture_roots.sh}"; case "$H" in /*) ;; *) H="$(pwd)/$H" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/fr_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
R="$T/roots.txt"
printf '# root\texempt checks\treason\ttask\ntests/security/needles/\tsecret_fold,anti_bluff,check_pins\tseeded needles\tT150\nscripts/qa/tests/fixtures\tprivate_key\tqa fixtures\tT209\n' > "$R"
ex() { "$H" --file "$R" exempt "$1" "$2" >"$T/out" 2>"$T/err"; RC=$?; }
# fake material assembled at run time from fragments (never a literal in this committed file)
A1="AK"; A2="IA"; A3="IOSFODNN7"; A4="EXAMPLE"; SECRET="$A1$A2$A3$A4"
K1="-----BEGIN RSA PRIV"; K2="ATE KEY-----"; KEYHDR="$K1$K2"
mkdir -p "$T/t/tests/security/needles" "$T/t/tests/security/other" "$T/t/tests/security/needles_x" "$T/t/scripts/qa/tests/fixtures"
for f in tests/security/needles/n1.txt tests/security/other/n1.txt tests/security/needles_x/n1.txt; do printf 'k=%s\n' "$SECRET" > "$T/t/$f"; done
printf '%s\n' "$KEYHDR" > "$T/t/scripts/qa/tests/fixtures/k.pem"
# 1 golden-true: a listed root exempts the files under it from the checks its row names
ex tests/security/needles/n1.txt secret_fold;        eq "root row names secret_fold: exempt" "$RC" 0
eq "exempt prints its word" "$(cat "$T/out")" exempt
ex tests/security/needles/deep/dir/n.go anti_bluff;  eq "nested file under the root, anti_bluff: exempt" "$RC" 0
ex tests/security/needles/n1.txt check_pins;         eq "third named check: exempt" "$RC" 0
ex scripts/qa/tests/fixtures/k.pem private_key;      eq "second root (no trailing slash in its row): private_key exempt" "$RC" 0
# 2 golden-false: a listed root does not exempt a check its row does not name
ex tests/security/needles/n1.txt trailing_whitespace; eq "check not named on the row: not exempt" "$RC" 1
eq "not-exempt prints its word" "$(cat "$T/out")" not_exempt
ex tests/security/needles/n1.txt private_key;         eq "another root's check on this root: not exempt" "$RC" 1
ex scripts/qa/tests/fixtures/k.pem secret_fold;       eq "second root, secret_fold not named: not exempt" "$RC" 1
# 3 golden-false: the same files one directory outside the root are refused (not exempt)
ex tests/security/other/n1.txt secret_fold;           eq "one directory outside the root: not exempt" "$RC" 1
ex tests/security/needles_x/n1.txt secret_fold;       eq "sibling sharing the root name prefix: not exempt" "$RC" 1
ex tests/security/n1.txt secret_fold;                 eq "parent directory of the root: not exempt" "$RC" 1
ex ./tests/security/needles/n1.txt secret_fold;       eq "leading ./ is normalised: exempt" "$RC" 0
ex tests/security/needles/../other/n1.txt secret_fold; eq ".. segment resolves outside the root: not exempt" "$RC" 1
# 4 filter: the S3 ratchet stages drop findings under a root for the checks its row names (run-time materialised files)
( cd "$T/t" && find . -type f | sed 's|^\./||' | sort ) > "$T/all.txt"
"$H" --file "$R" filter secret_fold < "$T/all.txt" > "$T/kept.txt" 2>"$T/err"; RC=$?
eq "filter secret_fold: exit" "$RC" 0
eq "filter secret_fold keeps the outside files and the unnamed second root" "$(tr '\n' ' ' < "$T/kept.txt")" "scripts/qa/tests/fixtures/k.pem tests/security/needles_x/n1.txt tests/security/other/n1.txt "
"$H" --file "$R" filter private_key < "$T/all.txt" > "$T/kept.txt"
eq "filter private_key drops only the second root" "$(grep -c 'scripts/qa' "$T/kept.txt")" 0
eq "filter private_key keeps the needles root (its row does not name it)" "$(grep -c 'tests/security/needles/' "$T/kept.txt")" 1
"$H" --file "$R" filter trailing_whitespace < "$T/all.txt" > "$T/kept.txt"
eq "filter of a check no row names drops nothing" "$(wc -l < "$T/kept.txt" | tr -d ' ')" "$(wc -l < "$T/all.txt" | tr -d ' ')"
# 5 an empty roots file (only comments) exempts nothing
printf '# no rows\n' > "$T/empty.txt"; "$H" --file "$T/empty.txt" exempt tests/security/needles/n1.txt secret_fold >/dev/null 2>&1; eq "no rows: nothing exempt" "$?" 1
# 6 validation: malformed rows are refused with 20, never skipped
bad_row() { printf '%b' "$1" > "$T/badroots.txt"; "$H" --file "$T/badroots.txt" exempt x/y secret_fold >"$T/out" 2>"$T/err"; RC=$?; }
bad_row 'x/\tnot_a_check\treason\tT1\n';  eq "check outside the closed set: 20" "$RC" 20
bad_row 'x/\tsecret_fold\n';               eq "row with missing columns: 20" "$RC" 20
bad_row 'x/*\tsecret_fold\tr\tT1\n';       eq "glob character in a root: 20" "$RC" 20
bad_row '../x/\tsecret_fold\tr\tT1\n';     eq ".. in a root: 20" "$RC" 20
bad_row '/abs/\tsecret_fold\tr\tT1\n';     eq "absolute root: 20" "$RC" 20
bad_row 'x/\t\tr\tT1\n';                   eq "row naming no check: 20" "$RC" 20
bad_row 'x/\tsecret_fold\t\tT1\n';         eq "row without a reason: 20" "$RC" 20
"$H" --file "$T/does-not-exist" exempt a b >/dev/null 2>&1; eq "missing roots file: 20" "$?" 20
"$H" --file "$R" exempt a not_a_check >/dev/null 2>&1; eq "unknown check asked: 20" "$?" 20
# 7 the committed roots file itself parses (header only today)
"$H" --file scripts/repo/fixture_roots.txt exempt tests/security/needles/n1.txt secret_fold >/dev/null 2>&1; eq "committed fixture_roots.txt parses; no row exempts anything yet" "$?" 1
echo "---- $PASSN ok, $FAILN failed"; [ "$FAILN" = 0 ] && [ -x "$H" ]
