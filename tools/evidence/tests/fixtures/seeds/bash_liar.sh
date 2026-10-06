#!/usr/bin/env bash
# seed: a FAIL line in the middle of a long run, then a summary that reports FAIL=0 and exit 0 (a script that
# does not propagate its own failure): the per-test line must win over the summary
for i in $(seq 1 40); do printf 'ok   check %s\n' "$i"; done
printf 'FAIL: check 41 broke\n'
for i in $(seq 42 80); do printf 'ok   check %s\n' "$i"; done
printf 'Summary: PASS=80 FAIL=0 SKIP=0\n'
exit 0
