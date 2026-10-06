#!/usr/bin/env bash
# no_skip_in_deterministic_lane.sh - T215. Gate CM-QA-NO-SKIP-IN-DETERMINISTIC-LANE: a conduit stream of a deterministic-lane run must hold no SKIP verdict
# (doc12 8.2, R-4). Runs scripts/qa/conduit_to_ledger.py --check-only in the deterministic lane over each stream and FAILS (exit 1) when the adapter calls the run invalid.
# Usage: no_skip_in_deterministic_lane.sh --stream FILE [--stream FILE ...] [--target-fingerprint V]
# Exits: 0 PASS (every stream is a valid deterministic run), 1 FAIL (the first invalid stream and the adapter's reason are printed), 2 usage / unreadable stream.
set -u
QA="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STREAMS=(); FP="gate-check-fingerprint"
while [ $# -gt 0 ]; do
  case "$1" in
    --stream) [ $# -ge 2 ] || { echo "no_skip_in_deterministic_lane: usage: --stream needs a value" >&2; exit 2; }; STREAMS+=("$2"); shift 2;;
    --target-fingerprint) FP="$2"; shift 2;;
    *) echo "no_skip_in_deterministic_lane: usage: unknown option '$1'" >&2; exit 2;;
  esac
done
[ "${#STREAMS[@]}" -gt 0 ] || { echo "no_skip_in_deterministic_lane: usage: at least one --stream" >&2; exit 2; }
rc=0
for s in "${STREAMS[@]}"; do
  out=$(python3 -I "$QA/conduit_to_ledger.py" --stream "$s" --ledger /dev/null --run gate --lane deterministic --target-fingerprint "$FP" --check-only 2>&1); arc=$?   # MUT:gate-lane
  case "$arc" in
    0) ;;
    3) echo "FAIL CM-QA-NO-SKIP-IN-DETERMINISTIC-LANE $s: $out"; rc=1;;
    *) echo "REFUSED CM-QA-NO-SKIP-IN-DETERMINISTIC-LANE $s: adapter rc=$arc $out" >&2; exit 2;;
  esac
done
[ "$rc" = 0 ] && echo "PASS CM-QA-NO-SKIP-IN-DETERMINISTIC-LANE (${#STREAMS[@]} stream(s))"
exit $rc
