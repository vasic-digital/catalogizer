#!/usr/bin/env bash
# seed: one passing assertion and one SEEDED FAILURE, a summary that says so, exit 1
printf 'PASS: first assertion [evidence: /tmp/ev1.json]\n'
printf 'FAIL: seeded failure: 2-3 is not 5 [evidence MISSING or empty: /tmp/nothing]\n'
printf 'Summary: PASS=1 FAIL=1 SKIP=0\n'
exit 1
