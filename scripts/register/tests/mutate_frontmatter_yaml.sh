#!/usr/bin/env bash
# mutate_frontmatter_yaml.sh - paired mutation of test_frontmatter_yaml.py (T164, WP-20): a test copy that skips the
# snapshot re-hash call counts a file changed after the manifest was written, so its --selftest MUST FAIL (the moved-snapshot
# check). Control: the unmutated selftest passes. Exit 0 only when the control passes and every mutant is killed.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REG="$(dirname "$HERE")"
W=$(mktemp -d "${TMPDIR:-/tmp}/mutfm.XXXXXX"); trap 'rm -rf "$W"' EXIT
mkdir -p "$W/tests"; cp "$REG/enumerate_sources.py" "$REG/check_freeze_snapshot.sh" "$REG/snapshot_manifest.py" "$W/"; cp "$HERE/wp20_fixture.py" "$W/tests/"
cp "$HERE/test_frontmatter_yaml.py" "$W/tests/orig.py"; python3 "$W/tests/orig.py" --selftest >"$W/control.txt" 2>&1; c=$?
echo "CONTROL unmutated selftest: rc=$c $(tail -1 "$W/control.txt")"; [ $c -eq 0 ] || exit 1
surv=0
mut() { local name=$1 expr=$2 want=$3; cp "$HERE/test_frontmatter_yaml.py" "$W/tests/m.py"; sed -i -e "$expr" "$W/tests/m.py"
  if cmp -s "$W/tests/m.py" "$HERE/test_frontmatter_yaml.py"; then echo "MUTANT $name: NOT APPLIED - survivor"; surv=$((surv+1)); return; fi
  python3 "$W/tests/m.py" --selftest >"$W/o_$name.txt" 2>&1; rc=$?
  if [ $rc -ne 0 ] && grep -Eq "$want" "$W/o_$name.txt"; then echo "MUTANT $name: KILLED (saw /$want/)"; else echo "MUTANT $name: SURVIVED rc=$rc"; surv=$((surv+1)); fi; }
mut skip_snapshot_check 's/^    if r.returncode != 0:/    if False:/' 'FAIL moved snapshot'
echo "MUTANTS: $surv survived"; [ $surv -eq 0 ]
