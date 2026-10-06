#!/usr/bin/env bash
# test_gates.sh - T210 (+T215/T217 gates share the pattern). Paired mutations for the bank ratchet gates CM-QA-BANK-NO-PLACEHOLDER and CM-QA-CASE-ASSERTS:
# the gate must PASS on the real banks against the staged baseline and FAIL on each mutation (add a placeholder; remove an assertion), observed before trust.
# Usage: bash scripts/qa/tests/test_gates.sh [--baseline FILE]   (through `TIC tooling unit`; default baseline: the staged copy under the WP-24 evidence)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
G="$ROOT/scripts/qa/gates/qa_bank_ratchet.sh"
BASE="$ROOT/specs/001-full-project-audit-remediation/evidence/wp24/staged/qa_banks.tsv"
[ "${1:-}" = "--baseline" ] && BASE="$2"
fail=0; ok() { echo "ok   $*"; }; bad() { echo "FAIL $*"; fail=1; }
SCR="$(mktemp -d)"; trap 'rm -rf "$SCR"' EXIT

expect() { # <label> <want_rc> <output-regex> -- cmd...
  local label="$1" want="$2" re="$3"; shift 4
  local out rc; out=$("$@" 2>&1); rc=$?
  { [ "$rc" = "$want" ] && echo "$out" | grep -qE "$re"; } && ok "$label" || { bad "$label (rc=$rc want $want)"; echo "$out" | head -5; }
}

for g in CM-QA-BANK-NO-PLACEHOLDER CM-QA-CASE-ASSERTS; do
  expect "REAL $g passes against the baseline" 0 "^PASS $g" -- bash "$G" $g --baseline "$BASE"
done

# mutation 1: add a placeholder marker to one step of a scratch copy of the real banks -> the placeholder gate FAILs
mkdir -p "$SCR/m1"; cp "$ROOT"/challenges/helixqa-banks/*.yaml "$SCR/m1/"
f=$(ls "$SCR/m1"/*.yaml | grep -v MANIFEST | head -1)
python3 - "$f" <<'P'
import sys,yaml
p=sys.argv[1]; d=yaml.safe_load(open(p)); st=d['test_cases'][0]['steps'][0]
st['action']='http: GET /x TODO placeholder'
yaml.safe_dump(d,open(p,'w'))
P
expect "MUTATION add a placeholder -> CM-QA-BANK-NO-PLACEHOLDER FAILs" 1 "^FAIL CM-QA-BANK-NO-PLACEHOLDER" -- bash "$G" CM-QA-BANK-NO-PLACEHOLDER --banks "$SCR/m1" --baseline "$BASE"

# negative control: REMOVING a placeholder (a lower count) must still PASS (the ratchet only refuses a rise)
mkdir -p "$SCR/c1"; cp "$ROOT"/challenges/helixqa-banks/*.yaml "$SCR/c1/"
f=$(ls "$SCR/c1"/*.yaml | grep -v MANIFEST | head -1)
python3 - "$f" <<'P'
import sys,yaml
p=sys.argv[1]; d=yaml.safe_load(open(p))
for s in d['test_cases'][0]['steps']:
    s['action']='http: GET /x'
yaml.safe_dump(d,open(p,'w'))
P
expect "CONTROL fewer placeholders still PASS" 0 "^PASS CM-QA-BANK-NO-PLACEHOLDER" -- bash "$G" CM-QA-BANK-NO-PLACEHOLDER --banks "$SCR/c1" --baseline "$BASE"

# mutation 2: remove an assertion from a bank that holds assertions (a scratch bank built from the golden-good fixture, baseline taken from it)
mkdir -p "$SCR/m2"; cp "$HERE/fixtures/banks/good.yaml" "$SCR/m2/good.yaml"
python3 -I "$ROOT/scripts/qa/validate_banks.py" --banks "$SCR/m2" --counts-tsv "$SCR/m2.tsv" >/dev/null; : >>"$SCR/m2.tsv"
expect "CONTROL golden-good scratch bank passes with its own baseline" 0 "^PASS CM-QA-CASE-ASSERTS" -- bash "$G" CM-QA-CASE-ASSERTS --banks "$SCR/m2" --baseline "$SCR/m2.tsv"
python3 - "$SCR/m2/good.yaml" <<'P'
import sys,yaml
p=sys.argv[1]; d=yaml.safe_load(open(p)); s=d['test_cases'][0]['steps'][0]
del s['expect_status']; del s['expect_json_path']
yaml.safe_dump(d,open(p,'w'))
P
expect "MUTATION remove an assertion -> CM-QA-CASE-ASSERTS FAILs" 1 "^FAIL CM-QA-CASE-ASSERTS" -- bash "$G" CM-QA-CASE-ASSERTS --banks "$SCR/m2" --baseline "$SCR/m2.tsv"

expect "REFUSED baseline absent is not PASS" 2 "baseline_absent" -- bash "$G" CM-QA-CASE-ASSERTS --baseline "$SCR/nope.tsv"
expect "REFUSED unknown gate id" 2 "usage" -- bash "$G" CM-NOPE
mkdir -p "$SCR/empty"
expect "REFUSED validator blind (dir without banks)" 2 "validator_failed" -- bash "$G" CM-QA-CASE-ASSERTS --banks "$SCR/empty" --baseline "$BASE"

[ "$fail" = 0 ] && echo "test_gates: PASS" || echo "test_gates: FAIL"
exit $fail
