#!/usr/bin/env bash
# mutate_w2.sh - runs the mutation set of one W2 register tool (mutate_w2.py) against its suite, each mutant in its own rootless IMG-TESTUTIL container
# through scripts/containers/run_pinned.sh (never the host). A mutant is KILLED when the suite exits non-zero. Expected: N00 (unmutated) PASSES, C00 (comment
# only) SURVIVES, every other mutant is KILLED, except those listed with a reason in equivalent_w2_mutants.tsv (reported as EQUIVALENT, not as a failure).
# Usage: mutate_w2.sh <tool> [--ledger] [ID...]     env: MUT_DIR (default .audit/scratch/w2mut/<tool>), RESULT (tsv, default $MUT_DIR/result.tsv),
#        MUT_HOST=1 (run on the host instead; only for development, the evidence runs use the container), MUT_WAIT_S (max seconds to wait for the memory budget, default 1800)
#   --ledger  record each run as a MUTATION entry (the tool's item, e.g. AUD-T166) through tools/evidence/evrec (the result field is the observed outcome)
# Output: one line per mutant `ID<TAB>KILLED|SURVIVED|PASSED|EQUIVALENT|ERROR<TAB>suite exit<TAB>what`; exit 0 when the expectations above hold, 1 otherwise.
# A run refused by the launcher with memory_budget_unavailable is retried (never overridden) until MUT_WAIT_S has passed; a refusal is not a kill.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; ROOT="$(cd "$HERE/../../.." && pwd)"; cd "$ROOT" || exit 2
TOOL=${1:-}; [ -n "$TOOL" ] || { echo "usage: mutate_w2.sh <tool> [--ledger] [ID...]" >&2; exit 2; }; shift
MUT_DIR=${MUT_DIR:-.audit/scratch/w2mut/$TOOL}; RESULT=${RESULT:-$MUT_DIR/result.tsv}; WAIT=${MUT_WAIT_S:-1800}
LEDGER=0; [ "${1:-}" = --ledger ] && { LEDGER=1; shift; }
mkdir -p "$MUT_DIR"; python3 "$HERE/mutate_w2.py" "$TOOL" "$MUT_DIR" "$@" > "$MUT_DIR/ids.txt" || exit 2
SUITE=$(python3 "$HERE/mutate_w2.py" --field "$TOOL" suite); ITEM=$(python3 "$HERE/mutate_w2.py" --field "$TOOL" item)
mapfile -t SRCS < <(python3 "$HERE/mutate_w2.py" --field "$TOOL" sources)
: > "$RESULT"; bad=0
run_one() {  # run_one ID -> sets rc (suite exit; 999 = never ran) and log
  local id=$1 envs=() l start
  local base=/src; [ "${MUT_HOST:-0}" = 1 ] && base=$ROOT
  while IFS= read -r l; do p=${l#*=}; case $p in /*) ;; *) p=$base/$p;; esac; envs+=("${l%%=*}=$p"); done < <(python3 "$HERE/mutate_w2.py" --env "$TOOL" "$id" "$MUT_DIR/$id")
  local cmd=(scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- env "${envs[@]}" bash -c "$SUITE")
  [ "${MUT_HOST:-0}" = 1 ] && cmd=(env "${envs[@]}" bash -c "$SUITE")
  start=$(date +%s)
  while :; do
    if [ $LEDGER = 1 ]; then
      what=$(cat "$MUT_DIR/$id/WHAT"); res=caught; [ "$id" = C00 ] && res=survived; [ "$id" = N00 ] && res=none
      mj=$(python3 -c 'import json,sys;print(json.dumps({"author":"test_author","location":sys.argv[3],"operator":sys.argv[1],"result":sys.argv[2]}))' "$id: $what" "$res" "scripts/register/$(python3 "$HERE/mutate_w2.py" --field "$TOOL" target)")
      if [ "$id" = N00 ]; then pol=GREEN; xa=(); else pol=MUTATION; xa=(--mutation-json "$mj"); fi
      ts=(); for s in "${SRCS[@]}"; do ts+=(--test-source "$s"); done
      tools/evidence/evrec run "$ITEM" "$pol" 1 shell_script "$MUT_DIR/$id" --evidence-class runtime --oracle specified --oracle-independent "${ts[@]}" "${xa[@]}" -- "${cmd[@]}" >"$MUT_DIR/$id.log" 2>&1
      if ! grep -q '^recorded seq=' "$MUT_DIR/$id.log"; then rc=999; else grep -q 'verdict=fail' "$MUT_DIR/$id.log" && rc=1 || rc=0; fi
    else
      "${cmd[@]}" >"$MUT_DIR/$id.log" 2>&1; rc=$?
    fi
    if grep -q 'REFUSED reason=memory_budget_unavailable' "$MUT_DIR/$id.log" && [ $(( $(date +%s) - start )) -lt "$WAIT" ]; then sleep 20; continue; fi
    break
  done
}
while read -r id; do
  what=$(cat "$MUT_DIR/$id/WHAT"); run_one "$id"
  case "$id" in N00) o=$([ $rc -eq 0 ] && echo PASSED || echo ERROR);; C00) o=$([ $rc -eq 0 ] && echo SURVIVED || echo ERROR);;
    *) if [ $rc -eq 999 ]; then o=ERROR; elif [ $rc -ne 0 ]; then o=KILLED; else o=SURVIVED; fi;; esac
  if [ "$o" = SURVIVED ] && grep -q "^$TOOL	$id	" "$HERE/equivalent_w2_mutants.tsv" 2>/dev/null; then o=EQUIVALENT; fi
  case "$id:$o" in N00:PASSED|C00:SURVIVED|M*:KILLED|M*:EQUIVALENT) ;; *) bad=1;; esac
  printf '%s\t%s\t%s\t%s\n' "$id" "$o" "$rc" "$what" | tee -a "$RESULT"
done < "$MUT_DIR/ids.txt"
echo "# mutation run ($TOOL): $(grep -c KILLED "$RESULT") killed, $(grep -c SURVIVED "$RESULT") survived (C00 expected), $(grep -c EQUIVALENT "$RESULT") equivalent, expectations $([ $bad = 0 ] && echo MET || echo NOT MET)"
exit $bad
