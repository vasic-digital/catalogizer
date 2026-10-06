#!/usr/bin/env bash
# seed for the T051 bash parser fixtures: three assertions, one skip, a summary line, exit 0
printf 'PASS: login returns a session [evidence: /tmp/ev1.json]\n'
printf 'ok   health endpoint reports status\n'
printf 'SKIP: optional feature [reason: unreachable]\n'
printf 'Summary: PASS=2 FAIL=0 SKIP=1\n'
exit 0
