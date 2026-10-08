#!/usr/bin/env bash
# mutate_import.sh - T168 (WP-20): runs the importer mutation set (mutate_import.py) against test_import.sh (T163) and test_import_classes.py,
# each mutant in its own rootless IMG-TESTUTIL container through scripts/containers/run_pinned.sh (never the host). A mutant is KILLED when
# either suite exits non-zero. Expected: N00 (unmutated) PASSES, C00 (comment only) SURVIVES, every other mutant is KILLED.
# Usage: mutate_import.sh [--ledger] [ID...]      env: MUT_DIR (default .audit/scratch/t168imp/mut), RESULT (tsv path, default $MUT_DIR/result.tsv)
#   --ledger  record each run as a MUTATION entry (item AUD-T168) through tools/evidence/evrec (the result field is the observed outcome)
# Output: one line per mutant `ID<TAB>KILLED|SURVIVED|PASSED|ERROR<TAB>suite exit<TAB>what`; exit 0 when the expectations above hold, 1 otherwise.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; ROOT="$(cd "$HERE/../../.." && pwd)"; cd "$ROOT" || exit 2
MUT_DIR=${MUT_DIR:-.audit/scratch/t168imp/mut}; RESULT=${RESULT:-$MUT_DIR/result.tsv}
LEDGER=0; [ "${1:-}" = --ledger ] && { LEDGER=1; shift; }
mkdir -p "$MUT_DIR"; python3 "$HERE/mutate_import.py" "$MUT_DIR" "$@" > "$MUT_DIR/ids.txt" || exit 2
: > "$RESULT"; bad=0
while read -r id; do
  what=$(cat "$MUT_DIR/$id/WHAT")
  cmd=(scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- env "IMPORT_SH=/src/$MUT_DIR/$id/import.sh" bash -c 'bash scripts/register/tests/test_import.sh && python3 -I scripts/register/tests/test_import_classes.py')
  if [ $LEDGER = 1 ]; then
    res=caught; [ "$id" = C00 ] && res=survived; [ "$id" = N00 ] && res=none
    mj=$(python3 -c 'import json,sys;print(json.dumps({"author":"test_author","location":"scripts/register/import_tickets.py","operator":sys.argv[1],"result":sys.argv[2]}))' "$id: $what" "$res")
    if [ "$id" = N00 ]; then pol=GREEN; xa=(); else pol=MUTATION; xa=(--mutation-json "$mj"); fi
    tools/evidence/evrec run AUD-T168 "$pol" 1 shell_script "$MUT_DIR/$id" --evidence-class runtime --oracle specified --oracle-independent \
      --test-source scripts/register/tests/test_import.sh --test-source scripts/register/tests/test_import_classes.py --test-source scripts/register/tests/wp20_fixture.py \
      "${xa[@]}" -- "${cmd[@]}" >"$MUT_DIR/$id.log" 2>&1; rc=$?
    # a refusal to record (evrec: refused ...) is NOT a kill: only a line "recorded seq=..." counts, and the kill/pass is the recorded verdict
    if ! grep -q '^recorded seq=' "$MUT_DIR/$id.log"; then rc=999; else grep -q 'verdict=fail' "$MUT_DIR/$id.log" && rc=1 || rc=0; fi
  else
    "${cmd[@]}" >"$MUT_DIR/$id.log" 2>&1; rc=$?
  fi
  case "$id" in N00) o=$([ $rc -eq 0 ] && echo PASSED || echo ERROR);; C00) o=$([ $rc -eq 0 ] && echo SURVIVED || echo ERROR);; *) o=$([ $rc -eq 999 ] && echo ERROR || { [ $rc -ne 0 ] && echo KILLED || echo SURVIVED; });; esac
  case "$id:$o" in N00:PASSED|C00:SURVIVED|M*:KILLED) ;; *) bad=1;; esac
  printf '%s\t%s\t%s\t%s\n' "$id" "$o" "$rc" "$what" | tee -a "$RESULT"
done < "$MUT_DIR/ids.txt"
echo "# mutation run: $(grep -c KILLED "$RESULT") killed, $(grep -c SURVIVED "$RESULT") survived (C00 expected), expectations $([ $bad = 0 ] && echo MET || echo NOT MET)"
exit $bad
