#!/usr/bin/env bash
# test_bank_id_floor.sh - T214 (RED first: before challenges/helixqa-banks/.bank-id-floor.txt exists this exits non-zero). Acceptance of the bank-id floor
# (doc12 7.7): the floor file exists beside the banks, is not header-only, holds every case id the banks hold (an independent grep count of `- id:` case
# lines agrees with the id count the checker derives), and the directory scan (the loader's checkBankIDFloor rule, implemented here as
# scripts/qa/bank_id_floor_check.py so a host or CI-less check can run it; helixqa itself stays the authority that regenerates the floor) FAILS when a
# case is deleted in a scratch copy: the paired mutation.
# Usage: bash scripts/qa/tests/test_bank_id_floor.sh      (through `TIC tooling unit`)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
BD="$ROOT/challenges/helixqa-banks"
FLOOR="$BD/.bank-id-floor.txt"
CHK="$ROOT/scripts/qa/bank_id_floor_check.py"
fail=0; ok() { echo "ok   $*"; }; bad() { echo "FAIL $*"; fail=1; }

[ -f "$FLOOR" ] && ok "floor file exists: $FLOOR" || { bad "floor file absent: $FLOOR"; echo "test_bank_id_floor: FAIL"; exit 1; }
[ -f "$CHK" ] || { bad "checker absent: $CHK"; echo "test_bank_id_floor: FAIL"; exit 1; }
ids=$(grep -vE '^[[:space:]]*(#|$)' "$FLOOR" | wc -l | tr -d ' ')
[ "$ids" -gt 0 ] && ok "floor holds $ids case ids (not header-only)" || bad "floor holds no ids"
sorted_unique=$(grep -vE '^[[:space:]]*(#|$)' "$FLOOR" | LC_ALL=C sort -u | wc -l | tr -d ' ')
[ "$ids" = "$sorted_unique" ] && ok "floor ids are unique" || bad "floor has duplicate ids ($ids vs $sorted_unique unique)"
# independent oracle: grep of the case-id lines of the banks (`- id:` at indent 0 to 2), unique
grepn=$(cat "$BD"/*.yaml | grep -E '^ {0,2}- id: ' | sed 's/^- id: *//; s/^["'\'']//; s/["'\'']$//' | LC_ALL=C sort -u | wc -l | tr -d ' ')
[ "$grepn" = "$ids" ] && ok "independent grep count of unique case ids ($grepn) equals the floor ($ids)" || bad "grep count $grepn != floor $ids"
out=$(python3 -I "$CHK" --banks "$BD" 2>&1); rc=$?
[ "$rc" = 0 ] && ok "directory scan passes on the real banks" || { bad "directory scan rc=$rc"; echo "$out" | head -3; }

SCR="$(mktemp -d)"; trap 'rm -rf "$SCR"' EXIT
cp "$BD"/*.yaml "$BD/.bank-id-floor.txt" "$SCR/"
python3 -I "$CHK" --banks "$SCR" >/dev/null 2>&1 && ok "CONTROL scratch copy passes" || bad "CONTROL scratch copy fails"
# mutation: delete one case from one bank in the scratch copy
VB=$(ls "$SCR"/*.yaml | grep -v MANIFEST | tail -1)   # the bank is chosen from the listing: this file names no bank (test_manifest.sh scans it)
victim=$(grep -E '^ {0,2}- id: ' "$VB" | head -1 | sed 's/^ *- id: *//')
python3 - "$VB" "$victim" <<'P'
import sys, yaml
p, vid = sys.argv[1:3]
d = yaml.safe_load(open(p)); d["test_cases"] = [c for c in d["test_cases"] if c["id"] != vid]
yaml.safe_dump(d, open(p, "w"))
P
out=$(python3 -I "$CHK" --banks "$SCR" 2>&1); rc=$?
{ [ "$rc" = 1 ] && echo "$out" | grep -q "$victim"; } && ok "MUTATION deleting case $victim: the scan FAILS and names it" || { bad "MUTATION rc=$rc"; echo "$out" | head -3; }
# negative control: ADDING a case never trips the floor
cp "$BD"/*.yaml "$SCR/"; python3 - "$VB" <<'P'
import sys, yaml, copy
p = sys.argv[1]; d = yaml.safe_load(open(p)); c = copy.deepcopy(d["test_cases"][0]); c["id"] = "zz-added-case"; d["test_cases"].append(c)
yaml.safe_dump(d, open(p, "w"))
P
python3 -I "$CHK" --banks "$SCR" >/dev/null 2>&1 && ok "CONTROL adding a case does not trip the floor" || bad "adding a case tripped the floor"
# floor absent in a directory: not enforced (the loader's rule), reported as such by the checker with exit 0 and a NOTE
rm "$SCR/.bank-id-floor.txt"; out=$(python3 -I "$CHK" --banks "$SCR" 2>&1); [ "$?" = 0 ] && echo "$out" | grep -q 'not enforced' && ok "no floor file: reported as not enforced" || bad "absent floor: $out"
[ "$fail" = 0 ] && echo "test_bank_id_floor: PASS" || echo "test_bank_id_floor: FAIL"
exit $fail
