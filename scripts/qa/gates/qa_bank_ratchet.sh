#!/usr/bin/env bash
# qa_bank_ratchet.sh - T210. The ratchet stage of the bank gates CM-QA-BANK-NO-PLACEHOLDER (rule R-2) and CM-QA-CASE-ASSERTS (rules R-1 and R-8).
# Usage: qa_bank_ratchet.sh <CM-QA-BANK-NO-PLACEHOLDER|CM-QA-CASE-ASSERTS> [--banks DIR] [--baseline FILE]
#   --banks DIR      the bank directory (default challenges/helixqa-banks of this checkout)
#   --baseline FILE  `bank<TAB>rule<TAB>count` lines (default scripts/repo/validate_baselines/qa_banks.tsv, written by T210 from its baseline run and
#                    held on its verdict; # lines are the header naming the measured commit and the holding verdict)
# The gate runs scripts/qa/validate_banks.py over DIR and FAILS (exit 1) when a (bank, rule) count of its rules rises above the baseline or when a
# (bank, rule) key appears that the baseline does not hold: a change fails only on a rise while WP-60 converts the banks, never on the existing debt.
# Exits: 0 PASS, 1 FAIL (the offending keys are printed), 2 REFUSED (usage, baseline absent or unreadable, validator crashed: a blind gate is never PASS).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
GATE="${1:-}"; [ $# -ge 1 ] && shift
BANKS="$ROOT/challenges/helixqa-banks"; BASE="$ROOT/scripts/repo/validate_baselines/qa_banks.tsv"
while [ $# -gt 0 ]; do
  case "$1" in
    --banks) BANKS="${2:-}"; shift 2;;
    --baseline) BASE="${2:-}"; shift 2;;
    *) echo "qa_bank_ratchet: REFUSED reason=usage unknown option '$1'" >&2; exit 2;;
  esac
done
case "$GATE" in
  CM-QA-BANK-NO-PLACEHOLDER) RULES="R-2";;
  CM-QA-CASE-ASSERTS) RULES="R-1 R-8";;
  *) echo "qa_bank_ratchet: REFUSED reason=usage gate id must be CM-QA-BANK-NO-PLACEHOLDER or CM-QA-CASE-ASSERTS" >&2; exit 2;;
esac
[ -r "$BASE" ] || { echo "qa_bank_ratchet: REFUSED reason=baseline_absent $BASE (written by T210 and held on its verdict)" >&2; exit 2; }
TMP="$(mktemp)"; trap 'rm -f "$TMP"' EXIT
python3 -I "$ROOT/scripts/qa/validate_banks.py" --banks "$BANKS" --counts-tsv "$TMP" >/dev/null 2>"$TMP.err"; vrc=$?
# 0 = clean, 1 = violations (the normal state while the banks are converted); anything else means the validator did not run
if [ "$vrc" != 0 ] && [ "$vrc" != 1 ]; then
  echo "qa_bank_ratchet: REFUSED reason=validator_failed rc=$vrc $(head -c 300 "$TMP.err")" >&2; rm -f "$TMP.err"; exit 2
fi
rm -f "$TMP.err"
rc=0
for r in $RULES; do
  while IFS=$'\t' read -r bank rule cnt; do
    [ "$rule" = "$r" ] || continue
    base=$(awk -F'\t' -v b="$bank" -v r="$r" '!/^#/ && $1==b && $2==r {print $3}' "$BASE")
    if [ -z "$base" ]; then echo "FAIL $GATE new key ($bank, $r) count $cnt is not in the baseline"; rc=1
    elif [ "$cnt" -gt "$base" ]; then echo "FAIL $GATE ($bank, $r) rose $base -> $cnt"; rc=1; fi
  done <"$TMP"
done
[ "$rc" = 0 ] && echo "PASS $GATE (no (bank, rule) count rose above the baseline)"
exit $rc
