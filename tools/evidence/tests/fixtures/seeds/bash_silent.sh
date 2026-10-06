#!/usr/bin/env bash
# seed: dies under set -e with no FAIL line and no summary (a truncated run): the exit status is the only trace
set -e
printf 'ok   first\n'
printf 'ok   second\n'
false
printf 'ok   never printed\n'
printf 'Summary: PASS=3 FAIL=0 SKIP=0\n'
