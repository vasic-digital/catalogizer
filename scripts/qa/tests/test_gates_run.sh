#!/usr/bin/env bash
# test_gates_run.sh - T215/T217. Paired mutations of the gates CM-QA-NO-SKIP-IN-DETERMINISTIC-LANE and CM-QA-BLOCKED-NOT-PASS:
#   no_skip: a valid stream PASSes, an injected-SKIP stream FAILs, an exploratory-only stream is not this gate's business only when run in that lane (the gate
#   itself always uses the deterministic lane); mutation: a copy of the gate whose lane is switched to exploratory must be caught (the injected SKIP then passes).
#   blocked_not_pass: summaries of an all-pass run PASS; blocked/failed with exit 0 FAIL; mutation: a copy that treats blocked as pass is caught.
# Usage: bash scripts/qa/tests/test_gates_run.sh    (through `TIC tooling unit`)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QA="$(cd "$HERE/.." && pwd)"
fail=0; ok() { echo "ok   $*"; }; bad() { echo "FAIL $*"; fail=1; }
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
ev() { printf '{"seq":%s,"type":"%s","session":"s"%s}\n' "$1" "$2" "$3"; }
{ ev 1 challenge_verdict ',"challenge":"c1","verdict":"PASS"'; ev 2 challenge_verdict ',"challenge":"c2","verdict":"FAIL"'; } >"$T/good.jsonl"
{ ev 1 challenge_verdict ',"challenge":"c1","verdict":"PASS"'; ev 2 challenge_verdict ',"challenge":"c2","verdict":"SKIP","reason":"x"'; } >"$T/skip.jsonl"
G="$QA/gates/no_skip_in_deterministic_lane.sh"
expect() { local label="$1" want="$2" re="$3"; shift 3; local out rc; out=$("$@" 2>&1); rc=$?; { [ "$rc" = "$want" ] && echo "$out" | grep -qE "$re"; } && ok "$label" || { bad "$label (rc=$rc want $want)"; echo "$out" | head -3; }; }
expect "valid stream: PASS" 0 '^PASS CM-QA-NO-SKIP' bash "$G" --stream "$T/good.jsonl"
expect "injected SKIP: FAIL" 1 '^FAIL CM-QA-NO-SKIP.*SKIP is not accepted' bash "$G" --stream "$T/skip.jsonl"
expect "two streams, one with a SKIP: FAIL" 1 '^FAIL' bash "$G" --stream "$T/good.jsonl" --stream "$T/skip.jsonl"
expect "unreadable stream: REFUSED (never PASS)" 2 'REFUSED' bash "$G" --stream "$T/nope.jsonl"
expect "no arguments: usage" 2 'usage' bash "$G"
mkdir -p "$T/mut/gates"; cp "$QA/conduit_to_ledger.py" "$T/mut/"
python3 - "$G" "$T/mut/gates/no_skip_in_deterministic_lane.sh" <<'P'
import sys
s = open(sys.argv[1]).read(); assert s.count("--lane deterministic") == 1
open(sys.argv[2], "w").write(s.replace("--lane deterministic", "--lane exploratory"))
P
out=$(bash "$T/mut/gates/no_skip_in_deterministic_lane.sh" --stream "$T/skip.jsonl" 2>&1); rc=$?
[ "$rc" = 0 ] && ok "MUTATION lane exploratory: the mutant lets the injected SKIP through (rc=0)" || bad "mutant behaviour unexpected rc=$rc"
# the property test itself FAILs against the mutant: the injected SKIP must FAIL, the mutant says PASS
[ "$rc" != 1 ] && ok "MUTATION is detected: the real gate FAILs the same injected SKIP (rc=1), the mutant does not" || bad "mutation went undetected"

B="$QA/gates/qa_blocked_not_pass.sh"
w() { printf '{"schema":"qa-run-summary/1","pass":%s,"fail":%s,"blocked":%s,"exit_code":%s,"lane":"%s"}\n' "$2" "$3" "$4" "$5" "${6:-deterministic}" >"$T/$1.json"; }
w allpass 5 0 0 0; w blocked_exit0 4 0 1 0; w blocked_exit4 4 0 1 4; w fail_exit0 4 1 0 0; w fail_exit1 4 1 0 1; w exploratory 5 0 0 0 exploratory
expect "all pass, exit 0: PASS" 0 '^PASS CM-QA-BLOCKED' bash "$B" --summary "$T/allpass.json"
expect "blocked verdict with exit 0: FAIL" 1 '^FAIL CM-QA-BLOCKED.*blocked' bash "$B" --summary "$T/blocked_exit0.json"
expect "blocked verdict with exit 4: PASS (blocked is correctly non-zero)" 0 '^PASS' bash "$B" --summary "$T/blocked_exit4.json"
expect "failed verdict with exit 0: FAIL" 1 '^FAIL' bash "$B" --summary "$T/fail_exit0.json"
expect "failed verdict with exit 1: PASS" 0 '^PASS' bash "$B" --summary "$T/fail_exit1.json"
expect "non-deterministic lane summary: FAIL" 1 '^FAIL.*lane' bash "$B" --summary "$T/exploratory.json"
echo '{bad' >"$T/bad.json"; expect "malformed summary: REFUSED" 2 'REFUSED' bash "$B" --summary "$T/bad.json"
python3 - "$B" "$T/mut_b.sh" <<'P'
import sys
s = open(sys.argv[1]).read()
for old in ("elif (b > 0 or fl > 0) and ec == 0:", "elif b > 0 and ec not in (4, 5, 6, 7, 1):"):
    assert s.count(old) == 1
    s = s.replace(old, "elif fl > 0 and ec == 0:" if old.startswith("elif (b") else "elif False:")   # blocked treated as pass
open(sys.argv[2], "w").write(s)
P
out=$(bash "$T/mut_b.sh" --summary "$T/blocked_exit0.json" 2>&1); [ "$?" = 0 ] && ok "MUTATION blocked-as-pass: the mutant PASSes the blocked-with-exit-0 summary that the real gate FAILs (detected)" || bad "blocked-as-pass mutation undetected"
[ "$fail" = 0 ] && echo "test_gates_run: PASS" || echo "test_gates_run: FAIL"; exit $fail
