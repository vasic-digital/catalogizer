#!/usr/bin/env bash
# qa_blocked_not_pass.sh - T217. Gate CM-QA-BLOCKED-NOT-PASS: a run whose summary records a blocked or a failed verdict must have exited non-zero (blocked is not pass,
# FR-025; the wrapper exits 4 / 1). Reads the summary.json that scripts/qa/run_profile.sh writes (fields pass, fail, blocked, exit_code, lane).
# Usage: qa_blocked_not_pass.sh --summary FILE [--summary FILE ...]
# Exits: 0 PASS, 1 FAIL (a summary with blocked/failed verdicts and exit_code 0, or one whose lane is not deterministic), 2 usage / unreadable or malformed summary.
set -u
SUMS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --summary) [ $# -ge 2 ] || { echo "qa_blocked_not_pass: usage: --summary needs a value" >&2; exit 2; }; SUMS+=("$2"); shift 2;;
    *) echo "qa_blocked_not_pass: usage: unknown option '$1'" >&2; exit 2;;
  esac
done
[ "${#SUMS[@]}" -gt 0 ] || { echo "qa_blocked_not_pass: usage: at least one --summary" >&2; exit 2; }
rc=0
for f in "${SUMS[@]}"; do
  verdict=$(python3 -I - "$f" <<'P'
import json, sys
try:
    d = json.load(open(sys.argv[1]))
    b, fl, ec = int(d["blocked"]), int(d["fail"]), int(d["exit_code"])
except Exception as e:
    print("MALFORMED %s" % str(e)[:80]); sys.exit(0)
if d.get("lane") != "deterministic":
    print("FAIL lane is %r, not deterministic" % d.get("lane"))
elif (b > 0 or fl > 0) and ec == 0:
    print("FAIL %d blocked / %d failed verdict(s) but the run exited 0" % (b, fl))
elif b > 0 and ec not in (4, 5, 6, 7, 1):
    print("FAIL blocked verdicts with an unexpected exit code %d" % ec)
else:
    print("OK")
P
)
  case "$verdict" in
    OK) ;;
    MALFORMED*) echo "REFUSED CM-QA-BLOCKED-NOT-PASS $f: ${verdict#MALFORMED }" >&2; exit 2;;
    *) echo "FAIL CM-QA-BLOCKED-NOT-PASS $f: ${verdict#FAIL }"; rc=1;;
  esac
done
[ "$rc" = 0 ] && echo "PASS CM-QA-BLOCKED-NOT-PASS (${#SUMS[@]} summary file(s))"
exit $rc
