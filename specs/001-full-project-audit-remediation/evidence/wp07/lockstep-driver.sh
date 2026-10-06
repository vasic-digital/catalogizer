#!/usr/bin/env bash
# lockstep-driver.sh - T077 driver, run INSIDE IMG-TESTUTIL (read-only on the extracted trees).
# Usage: lockstep-driver.sh <extracted-constitution-tree>
# Runs every CM-COVENANT-114-<N>-PROPAGATION wrapper named in scripts/gates/covenant_propagation_anchors.tsv (the §11.4.227(B)
# block-start + exactly-once + byte-identical-lockstep check across CLAUDE/AGENTS/QWEN/GEMINI, §11.4.157) with --root <tree>,
# then the gate-ledger ratchet. Prints one line per gate and a total. Exit 0 only if every gate exited 0.
T=${1:?tree}; cd "$T" || exit 2
pass=0; fail=0; failed=""
while IFS=$'\t' read -r name anchor; do
  case "$name" in \#*|"") continue;; esac
  n=${name#CM-COVENANT-114-}; n=${n%-PROPAGATION}
  w="scripts/gates/cm_covenant_114_${n}_propagation.sh"
  if [ ! -f "$w" ]; then echo "NO-WRAPPER $name"; fail=$((fail+1)); failed="$failed $name(no-wrapper)"; continue; fi
  out=$(timeout 120 bash "$w" --root "$T" --quiet 2>&1); rc=$?
  last=$(printf '%s\n' "$out" | tail -1)
  if [ $rc -eq 0 ]; then pass=$((pass+1)); echo "PASS rc=0 $name anchor=$anchor"; else fail=$((fail+1)); failed="$failed $name(rc=$rc)"; echo "FAIL rc=$rc $name anchor=$anchor :: $last"; fi
done < scripts/gates/covenant_propagation_anchors.tsv
echo "PROPAGATION-FAMILY pass=$pass fail=$fail failed:${failed:- none}"
echo "--- cm_gate_ledger_ratchet.sh"
timeout 300 bash scripts/gates/cm_gate_ledger_ratchet.sh 2>&1 | tail -12; echo "ratchet rc=${PIPESTATUS[0]}"
[ "$fail" -eq 0 ]
