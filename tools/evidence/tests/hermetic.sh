#!/usr/bin/env bash
# Sourced by the evrec test files (T050 round 3, review finding I12). Makes a test run hermetic:
#   - every environment variable the tools read is unset, so nothing leaks in from the caller;
#   - EVREC_REPO_ROOT points at a scratch repo with an empty .audit/ directory, so the real checkout's
#     .audit/commit_turn.json is never read;
#   - the working directory is an empty scratch directory (a PROBE target ref such as `x` is resolved against the cwd, so a
#     file named x in the caller's cwd changed a fingerprint: found by the round-3 capture run from another directory);
#   - CPA_HOST_ENTRY points at a scratch stub that only logs the call and exits 99, and HOME is a scratch directory,
#     so the owner's real host entry (~/.local/bin/cpa-host) can never be reached.
# hermetic_init S      S is the scratch directory of the calling test
# hermetic_repo        re-export the scratch repo root (a test that switched to its own fixture calls this to come back)
# hermetic_host_calls  number of calls that reached the hermetic stub (0 expected outside the commit-turn fixtures)
hermetic_init() {
  H=$1/hermetic
  unset EV EV_LEDGER EV_BLOBS EV_ANCHOR EV_SCHEMA EV_TEST_SOURCES EV_LEDGER_BOUND EV_BLOB_BOUND EV_FLAKE_LEDGER \
        EVREC_REDACT_VARS EVREC_FAULT EVREC_LOCK_TIMEOUT EVREC_TURN_RUN_ID EVREC_PROC_ROOT EVREC_RUN_TOKEN EVREC_OPERAND_DIR_MAX EVREC_OPERAND_DIR_BYTES_MAX \
        PYTHONOPTIMIZE PYTHONPATH PYTHONSTARTUP PYTHONDONTWRITEBYTECODE CT_REFUSE CT_LOG CT_HOST_LOG
  mkdir -p "$H/repo/.audit" "$H/home" "$H/cwd"; cd "$H/cwd" || return 1
  printf '#!/usr/bin/env bash\necho "$*" >>"%s/host_calls"\nexit 99\n' "$H" >"$H/host_entry.sh"; chmod +x "$H/host_entry.sh"
  : >"$H/host_calls"
  export HOME=$H/home CPA_HOST_ENTRY=$H/host_entry.sh EVREC_REPO_ROOT=$H/repo
}
hermetic_repo() { export EVREC_REPO_ROOT=$H/repo CPA_HOST_ENTRY=$H/host_entry.sh; unset EVREC_TURN_RUN_ID; }
hermetic_host_calls() { wc -l <"$H/host_calls" | tr -d ' '; }
