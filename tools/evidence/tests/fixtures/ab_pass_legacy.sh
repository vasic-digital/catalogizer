#!/usr/bin/env bash
# The LEGACY per-script copy of ab_pass_with_evidence, verbatim (eleven identical copies existed under scripts/testing/full_automation/
# before T052). It is the golden-bad of test_ts02.sh: it prints PASS and writes no ev/1 entry.
ab_pass_with_evidence() {
  ab_desc="$1"; ab_evidence="$2"
  if [ ! -s "${ab_evidence}" ]; then
    printf 'FAIL: %s [evidence MISSING or empty: %s]\n' "${ab_desc}" "${ab_evidence}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    SUMMARY_ROWS="${SUMMARY_ROWS}FAIL\t${ab_desc}\t${ab_evidence}\n"
    return 1
  fi
  printf 'PASS: %s [evidence: %s]\n' "${ab_desc}" "${ab_evidence}"
  PASS_COUNT=$((PASS_COUNT + 1))
  SUMMARY_ROWS="${SUMMARY_ROWS}PASS\t${ab_desc}\t${ab_evidence}\n"
  return 0
}
