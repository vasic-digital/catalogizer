#!/usr/bin/env bash
# lib/ab_pass_with_evidence.sh - the ONE definition of ab_pass_with_evidence (tasks.md T052, docs/05 TS-02). Sourced by every full-automation
# suite under scripts/testing/full_automation/ (it replaces the eleven identical per-script copies); never run.
#
#   ab_pass_with_evidence <description> <evidence_path>
#
# Anti-bluff PASS helper (constitution 11.4.69, 11.4.115(F), 11.4.262): a PASS is printed only when (1) the evidence path exists and is
# non-empty, as before, AND (2) the check was recorded as an ev/1 entry by tools/evidence/evrec: `evrec run ITEM PROBE N remote_service
# <evidence_path> -- test -s <evidence_path>`, so the ledger holds the argv, the exit status and the sha256 of the evidence bytes at the time of
# the PASS. A recorder that is absent, refuses (ledger locked, commit-turn freeze, unwritable store) or exits non-zero is a FAIL, never a PASS:
# a PASS with no entry is exactly the bluff the legacy copies could print.
#
# Environment (all optional):
#   EVREC_ITEM   the register item, finding or run id the entries are filed under (CAT-nnn, FND-nnnn, RUN-n, AUD-x); default RUN-<pid of the suite>
#   EVREC_BIN    the recorder (default tools/evidence/evrec beside this file); EV_LEDGER / EV and the other evrec variables are the recorder's own
# Variables kept from the legacy copies: PASS_COUNT, FAIL_COUNT, SUMMARY_ROWS (tab-separated rows), the PASS:/FAIL: line formats.
# Limits (11.4.6): the entry proves the evidence file existed with these bytes and was non-empty when checked; whether the suite's assertion about
# the evidence is true is the suite's own (its oracle), as before.

AB_EVREC="${EVREC_BIN:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/evrec}"
AB_ITER="${AB_ITER:-0}"

ab_pass_with_evidence() {
  ab_desc="$1"; ab_evidence="$2"
  if [ ! -s "${ab_evidence}" ]; then
    printf 'FAIL: %s [evidence MISSING or empty: %s]\n' "${ab_desc}" "${ab_evidence}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    SUMMARY_ROWS="${SUMMARY_ROWS}FAIL\t${ab_desc}\t${ab_evidence}\n"
    return 1
  fi
  AB_ITER=$((AB_ITER + 1))
  ab_item="${EVREC_ITEM:-RUN-$$}"
  if ab_rec_out=$("${AB_EVREC}" run "${ab_item}" PROBE "${AB_ITER}" remote_service "${ab_evidence}" -- test -s "${ab_evidence}" 2>&1); then
    printf 'PASS: %s [evidence: %s]\n' "${ab_desc}" "${ab_evidence}"
    PASS_COUNT=$((PASS_COUNT + 1))
    SUMMARY_ROWS="${SUMMARY_ROWS}PASS\t${ab_desc}\t${ab_evidence}\n"
    return 0
  fi
  ab_why=$(printf '%s' "${ab_rec_out}" | tail -n 1 | cut -c 1-160)
  [ -n "${ab_why}" ] || ab_why="recorder ${AB_EVREC} unavailable"
  printf 'FAIL: %s [evidence recorder refused: %s; evidence: %s]\n' "${ab_desc}" "${ab_why}" "${ab_evidence}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
  SUMMARY_ROWS="${SUMMARY_ROWS}FAIL\t${ab_desc}\t${ab_evidence}\n"
  return 1
}
