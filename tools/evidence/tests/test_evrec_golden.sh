#!/usr/bin/env bash
# T050 round 3: runs the python golden/unit tests (test_evcore_golden.py) so the mutation runner sees their FAIL lines.
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
[ -x "$root/tools/evidence/evrec" ] && [ -x "$root/tools/evidence/verify" ] || { echo "FAIL golden tests (vacuous: recorder/verifier absent)"; echo "checks=1 failures=1"; exit 1; }
out=$(python3 "$here/test_evcore_golden.py" 2>&1); r=$?; printf '%s\n' "$out"
echo "checks=$(grep -c '^\(ok\|FAIL\) ' <<<"$out") failures=$(grep -c '^FAIL ' <<<"$out")"; [ $r -eq 0 ] && ! grep -q '^FAIL ' <<<"$out"
