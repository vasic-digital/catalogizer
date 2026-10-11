#!/usr/bin/env bash
# mut_verdict.sh - sourced helper of the WP-02 mutation harnesses (fix round 8, review R3-5).
#
# Purpose   One definition of how a mutant run's test exit codes classify. A test exits 77 when a leg was SKIPPED (no case failed, but part of the
#           assertions did not run). A mutant run that ends in 77 has NOT been killed: counting it as CAUGHT is the false-kill twin of the 11.4.201(6)
#           false-null (a mutant whose only effect is to make a leg skip would be reported as detected).
# Usage     . scripts/audit/tests/mut_verdict.sh
#             mut_combine RC...   -> prints 1 if any rc is neither 0 nor 77 (a real failure), else 77 if any rc is 77, else 0
#             mut_class   RC      -> prints CAUGHT (rc 1..76,78..), SKIPPED-NOT-KILLED (rc 77) or SURVIVED (rc 0)
# Exit      mut_* always return 0; the classification is on stdout.
mut_combine() {
  local r out=0
  for r in "$@"; do
    if [ "$r" -ne 0 ] && [ "$r" -ne 77 ]; then echo 1; return 0; fi
    [ "$r" -ne 77 ] || out=77
  done
  echo "$out"
}
mut_class() {
  case "$1" in 0) echo SURVIVED ;; 77) echo SKIPPED-NOT-KILLED ;; *) echo CAUGHT ;; esac
}
