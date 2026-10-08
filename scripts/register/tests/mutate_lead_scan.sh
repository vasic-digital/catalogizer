#!/usr/bin/env bash
# mutate_lead_scan.sh - paired mutations of lead_scan.py / lead_scan_population.py (T167, WP-20). Each mutant is a sed edit of a scratch
# copy of the tool set; test_lead_scan.py run against it (LEAD_SCAN) MUST FAIL. Control first: the unmutated tool passes.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REG="$(dirname "$HERE")"
W=$(mktemp -d "${TMPDIR:-/tmp}/mutlead.XXXXXX"); trap 'rm -rf "$W"' EXIT
fresh() { rm -f "$W"/*.py "$W"/*.sh; cp "$REG/lead_scan.py" "$REG/lead_scan_population.py" "$REG/enumerate_sources.py" "$REG/snapshot_manifest.py" "$REG/check_freeze_snapshot.sh" "$W/"; }
run() { LEAD_SCAN="$W/lead_scan.py" python3 "$HERE/test_lead_scan.py" >"$1" 2>&1; }
fresh; run "$W/control.txt"; c=$?; echo "CONTROL unmutated: rc=$c $(tail -1 "$W/control.txt")"; [ $c -eq 0 ] || exit 1
surv=0
mutant() { local name=$1 file=$2 expr=$3 want=$4; fresh; sed -i -e "$expr" "$W/$file"
  if cmp -s "$W/$file" "$REG/$file"; then echo "MUTANT $name: NOT APPLIED - survivor"; surv=$((surv+1)); return; fi
  run "$W/o_$name.txt"; rc=$?
  if [ $rc -ne 0 ] && grep -Eq "$want" "$W/o_$name.txt"; then echo "MUTANT $name: KILLED ($(grep -c '^FAIL' "$W/o_$name.txt") failing checks; saw /$want/)"; else echo "MUTANT $name: SURVIVED rc=$rc"; surv=$((surv+1)); fi; }
mutant scan_every_md       lead_scan_population.py 's/if any(inside(p, g) for g in gls) and not any(inside(p, e) for e in extra):   # MUT:population/if False:/' 'the same line in a submodule'
mutant ignore_extra_roots  lead_scan_population.py 's/ and not any(inside(p, e) for e in extra):   # MUT:population/ and True:/' 'HC-2 extra root naming'
mutant skip_snapshot_check lead_scan.py '/MUT:snapshot-check/{n;s/if r.returncode != 0:/if False:/}' 'snapshot changed after its manifest'
mutant skip_listing_check  lead_scan.py 's/^    if bad:$/    if False:/' 'NUL path appended'
mutant pattern_rule_off    lead_scan.py 's/if disposition_marker(l, words, infence and not fence_line):/if False:/' 'C6 a marker word as the argument'
mutant structured_off      lead_scan.py 's/            elif covered:/            elif False:/' 'C2 a lead line that a structured entry sits on'
mutant blind_check_off     lead_scan.py 's/        if r is None or word not in (r.legacy_id or "").split(",") or r.status != disp:   # MUT:needle-check/        if False:/' 'refused lead_scan_blind'
echo "MUTANTS: $surv survived"; [ $surv -eq 0 ]
