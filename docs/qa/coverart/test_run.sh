#!/usr/bin/env bash
# Quick tests of run.sh itself (no Go build): CLI contract and the verdict parser,
# including golden-good / golden-bad / negative-control fixtures (11.4.273, 11.4.107(10)).
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fail=0; n=0
t() { n=$((n+1)); if "$@"; then echo "ok $n - $DESC"; else echo "NOT ok $n - $DESC"; fail=$((fail+1)); fi; }
DESC="run.sh exists and is executable"; t test -x "$D/run.sh"
DESC="--help exits 0"; t bash -c "'$D/run.sh' --help >/dev/null 2>&1"
DESC="unknown option exits 64"; t bash -c "'$D/run.sh' --bogus >/dev/null 2>&1; [ \$? -eq 64 ]"
DESC="unknown mutation exits 64"; t bash -c "'$D/run.sh' --mutate nope >/dev/null 2>&1; [ \$? -eq 64 ]"
DESC="parser: golden-good fixture -> PASS (exit 0)"; t bash -c "'$D/run.sh' --parse '$D/fixtures/good.txt' >/dev/null 2>&1"
DESC="parser: golden-bad (a FAIL line) -> FAIL (exit 1)"; t bash -c "'$D/run.sh' --parse '$D/fixtures/bad_fail.txt' >/dev/null 2>&1; [ \$? -eq 1 ]"
DESC="parser: golden-bad (required test missing) -> FAIL (exit 1)"; t bash -c "'$D/run.sh' --parse '$D/fixtures/bad_missing.txt' >/dev/null 2>&1; [ \$? -eq 1 ]"
DESC="parser: negative control (empty file) -> FAIL, never PASS"; t bash -c ": > '$D/fixtures/empty.txt'; '$D/run.sh' --parse '$D/fixtures/empty.txt' >/dev/null 2>&1; [ \$? -eq 1 ]"
echo "tests=$n failed=$fail"; [ "$fail" -eq 0 ]
