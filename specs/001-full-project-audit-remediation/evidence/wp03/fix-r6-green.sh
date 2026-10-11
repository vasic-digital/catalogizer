#!/usr/bin/env bash
# three GREEN runs of the seven verifier suites (host), identity line inside each file
cd "$(git rev-parse --show-toplevel)" || exit 2
EV=specs/001-full-project-audit-remediation/evidence/wp03
for n in 1 2 3; do
  for s in test_verify_repos test_verify_repos_r6 test_verify_repos_r7 test_verify_repos_r8 test_verify_repos_r9 test_verify_repos_r10 test_verify_schema; do
    ( bash scripts/repo/tests/$s.sh > "$EV/fix-r6-green-$n-$s.txt" 2>&1; echo "$n $s rc=$?" >> "$EV/fix-r6-green-rcs.txt" ) &
  done; wait
done
