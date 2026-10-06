#!/usr/bin/env bash
# test_manifest.sh - T213. Acceptance of challenges/helixqa-banks/MANIFEST.yaml (doc12 6.7), through `TIC tooling unit` (IMG-TESTUTIL):
#   REAL   the manifest lists every bank file and no tracked script names a bank file (QF-04), except the reviewed legacy list;
#   NEEDLE-A a bank file added in a scratch copy and not listed is reported;  NEEDLE-B a script naming a bank file in a scratch copy is reported;
#   NEEDLE-C a manifest whose floor exceeds the bank's case count is reported (below_floor).
# Usage: bash scripts/qa/tests/test_manifest.sh      (run from anywhere; the repository root is found from this file)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
CHK="python3 -I $ROOT/scripts/qa/manifest_check.py"
LEGACY="$HERE/manifest_legacy_drivers.txt"
fail=0; ok() { echo "ok   $*"; }; bad() { echo "FAIL $*"; fail=1; }

out=$($CHK --root "$ROOT" --legacy "$LEGACY" 2>&1); rc=$?
[ "$rc" = 0 ] && ok "REAL manifest lists every bank, floors hold, no unreviewed bank-path literal" || { bad "REAL rc=$rc"; echo "$out"; }

SCR="$(mktemp -d)"; trap 'rm -rf "$SCR"' EXIT
BD=helixqa-banks   # the directory name is spelled once here; no file name below is a literal (the scan would flag this very file)
mkdir -p "$SCR/challenges/$BD" "$SCR/scripts" "$SCR/tests"
cp "$ROOT"/challenges/$BD/*.yaml "$SCR/challenges/$BD/"
mapfile -t BANKS < <(cd "$SCR/challenges/$BD" && ls *.yaml | grep -v "^MANIFEST")
B1="${BANKS[0]}"; B2="${BANKS[1]}"

# the scratch copy itself must be clean (control: the instrument is silent on a clean tree)
out=$($CHK --root "$SCR" 2>&1); rc=$?
[ "$rc" = 0 ] && ok "CONTROL scratch copy is clean" || { bad "CONTROL scratch copy rc=$rc"; echo "$out"; }

cp "$SCR/challenges/$BD/$B1" "$SCR/challenges/$BD/zz-unlisted-bank.yaml"
out=$($CHK --root "$SCR" 2>&1); rc=$?
{ [ "$rc" = 1 ] && echo "$out" | grep -q 'FINDING unlisted_bank zz-unlisted-bank.yaml'; } && ok "NEEDLE-A unlisted bank reported" || { bad "NEEDLE-A rc=$rc"; echo "$out"; }
rm "$SCR/challenges/$BD/zz-unlisted-bank.yaml"

printf '#!/bin/sh\necho %s\n' "challenges/$BD/$B2" > "$SCR/scripts/names_a_bank.sh"
out=$($CHK --root "$SCR" 2>&1); rc=$?
{ [ "$rc" = 1 ] && echo "$out" | grep -q 'FINDING bank_path_literal scripts/names_a_bank.sh'; } && ok "NEEDLE-B script naming a bank reported" || { bad "NEEDLE-B rc=$rc"; echo "$out"; }
rm "$SCR/scripts/names_a_bank.sh"

printf '#!/bin/sh\necho %s\n' "$B1" > "$SCR/tests/bare_basename.sh"
out=$($CHK --root "$SCR" 2>&1); rc=$?
{ [ "$rc" = 1 ] && echo "$out" | grep -q 'bank_path_literal tests/bare_basename.sh'; } && ok "NEEDLE-B2 bare basename reported" || { bad "NEEDLE-B2 rc=$rc"; echo "$out"; }
rm "$SCR/tests/bare_basename.sh"

sed -i '0,/case_floor: [0-9]*/s//case_floor: 99999/' "$SCR/challenges/$BD/MANIFEST.yaml"
out=$($CHK --root "$SCR" 2>&1); rc=$?
{ [ "$rc" = 1 ] && echo "$out" | grep -q 'FINDING below_floor'; } && ok "NEEDLE-C floor above the case count reported" || { bad "NEEDLE-C rc=$rc"; echo "$out"; }

[ "$fail" = 0 ] && echo "test_manifest: PASS" || echo "test_manifest: FAIL"
exit $fail
