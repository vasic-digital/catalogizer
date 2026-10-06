#!/usr/bin/env bash
# wrap-bash.sh - runner wrapper for bash test scripts (tasks.md T051). Usage and exit statuses: lib/wrap_common.sh.
#   wrap-bash.sh --run-token TOKEN [opts] -- COMMAND ARGS...   runs the command (a script that prints PASS:/FAIL:/SKIP:/ok/not ok lines)
# stdout and stderr are merged into the report; the exit status of the command takes part in the verdict.
set -u
. "$(dirname "$(readlink -f "$0")")/lib/wrap_common.sh"
wrap_parse "$@"
if [ -n "$W_FROM" ]; then wrap_report bash "$W_FROM"; exit $?; fi
[ "${#W_REST[@]}" -gt 0 ] || wrap_refuse usage_error "no command after --"
wrap_need "${W_REST[0]}"; wrap_tmp
env EVREC_RUN_TOKEN="$W_TOKEN" "${W_REST[@]}" >"$W_TMP/report" 2>&1 </dev/null; W_RC=$?
wrap_report bash "$W_TMP/report"; exit $?
