#!/usr/bin/env bash
# T052 paired mutation (docs/05 TS-02): restore ONE local copy of ab_pass_with_evidence into a scratch copy of the migrated suites; test_ts02.sh
# run in that scratch tree MUST FAIL, with a FAIL line that names the cause (a per-script definition under scripts/testing/full_automation, or the
# one-shipping-definition rule). Run twice: once on the unmodified tree (control: PASS) and once on the mutated tree. Exit 0 iff control PASS and mutant FAIL.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
mk() { local d; d=$(mktemp -d); mkdir -p "$d/tools" "$d/scripts" "$d/specs/001-full-project-audit-remediation/contracts"
  cp -r "$root/tools/evidence" "$d/tools/"; cp -r "$root/scripts/testing" "$d/scripts/"; mkdir -p "$d/scripts/repo"; cp "$root/scripts/repo/check_classes.tsv" "$d/scripts/repo/"
  cp "$root/specs/001-full-project-audit-remediation/contracts/evidence-record.schema.json" "$d/specs/001-full-project-audit-remediation/contracts/"; echo "$d"; }
C=$(mk); echo "## control (unmodified migrated tree)"; (cd "$C" && bash tools/evidence/tests/test_ts02.sh 2>&1 | tail -3); bash "$C/tools/evidence/tests/test_ts02.sh" >/dev/null 2>&1; crc=$?
M=$(mk); victim=$M/scripts/testing/full_automation/catalog_browse_filter_search.sh
python3 - "$victim" "$here/fixtures/ab_pass_legacy.sh" <<'P'
import sys
v, legacy = sys.argv[1], open(sys.argv[2]).read()
fn = legacy[legacy.index("ab_pass_with_evidence() {"):]
s = open(v).read()
src = [l for l in s.split("\n") if l.startswith('. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../../../tools/evidence/lib/ab_pass_with_evidence.sh"')]
assert len(src) == 1, src
open(v, "w").write(s.replace(src[0], fn.rstrip("\n")))
P
echo "## mutant: the legacy local copy restored into catalog_browse_filter_search.sh (scratch tree)"; (cd "$M" && bash tools/evidence/tests/test_ts02.sh 2>&1 | grep -E '^FAIL|^checks')
bash "$M/tools/evidence/tests/test_ts02.sh" >/dev/null 2>&1; mrc=$?
bash "$M/tools/evidence/tests/test_ts02.sh" 2>&1 | grep -q 'FAIL repo census: 1 per-script definitions under' && named=yes || named=no
rm -rf "$C" "$M"
echo "control_rc=$crc mutant_rc=$mrc cause_named=$named"
[ "$crc" = 0 ] && [ "$mrc" != 0 ] && [ "$named" = yes ]
