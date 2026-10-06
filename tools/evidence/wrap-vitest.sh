#!/usr/bin/env bash
# wrap-vitest.sh - runner wrapper for vitest (tasks.md T053). Usage and exit statuses: lib/wrap_common.sh.
#   wrap-vitest.sh --run-token TOKEN [opts] -- VITEST_ARGS...   runs  vitest run --reporter=json --outputFile=<tmp> VITEST_ARGS...
# WRAP_VITEST_BIN overrides the command (default `npx vitest`; words are split). The live leg runs in IMG-NODE after WP-11.
set -u
. "$(dirname "$(readlink -f "$0")")/lib/wrap_common.sh"
wrap_parse "$@"
if [ -n "$W_FROM" ]; then wrap_report vitest "$W_FROM"; exit $?; fi
read -r -a VT <<<"${WRAP_VITEST_BIN:-npx vitest}"; wrap_need "${VT[0]}"; wrap_tmp
env EVREC_RUN_TOKEN="$W_TOKEN" "${VT[@]}" run --reporter=json --outputFile="$W_TMP/report.json" "${W_REST[@]}" >"$W_TMP/out" 2>"$W_TMP/err" </dev/null; W_RC=$?
if [ -s "$W_TMP/report.json" ]; then wrap_report vitest "$W_TMP/report.json"; else cat "$W_TMP/out" "$W_TMP/err" >"$W_TMP/report.json"; wrap_report vitest "$W_TMP/report.json"; fi
exit $?
