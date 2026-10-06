#!/usr/bin/env bash
# wrap-cargo.sh - runner wrapper for cargo test (tasks.md T053). Usage and exit statuses: lib/wrap_common.sh.
#   wrap-cargo.sh --run-token TOKEN [opts] -- CARGO_TEST_ARGS...   runs  cargo test CARGO_TEST_ARGS...  (colour off, stdout+stderr merged)
# and judges the libtest lines (`test NAME ... ok|FAILED|ignored`, `test result:`). WRAP_CARGO_BIN overrides `cargo`.
# The live leg runs in IMG-RUST after WP-14.
set -u
. "$(dirname "$(readlink -f "$0")")/lib/wrap_common.sh"
wrap_parse "$@"
if [ -n "$W_FROM" ]; then wrap_report cargo "$W_FROM"; exit $?; fi
CG=${WRAP_CARGO_BIN:-cargo}; wrap_need "$CG"; wrap_tmp
env EVREC_RUN_TOKEN="$W_TOKEN" CARGO_TERM_COLOR=never "$CG" test "${W_REST[@]}" >"$W_TMP/report" 2>&1 </dev/null; W_RC=$?
wrap_report cargo "$W_TMP/report"; exit $?
