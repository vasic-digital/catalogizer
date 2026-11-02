#!/usr/bin/env bash
# shellcheck source=/dev/null
. "$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)/../../scripts/lib/cgo_flags.sh"   # 2026-11-02 decision (invokes go via $GO, so the gate sweep cannot see it)
# wrap-go.sh - runner wrapper for Go tests (tasks.md T051). Usage and exit statuses: lib/wrap_common.sh; guide docs/scripts/evidence-wrappers.md.
#   wrap-go.sh --run-token TOKEN [opts] -- GO_TEST_ARGS...      runs  go test -json -count=1 GO_TEST_ARGS...  and judges its test2json events
# `-count=1` always: a cached result is not a run (docs/06 s16). WRAP_GO_BIN overrides the `go` binary (tests only).
set -u
. "$(dirname "$(readlink -f "$0")")/lib/wrap_common.sh"
wrap_parse "$@"
if [ -n "$W_FROM" ]; then wrap_report go "$W_FROM"; exit $?; fi
GO=${WRAP_GO_BIN:-go}; wrap_need "$GO"; wrap_tmp
env EVREC_RUN_TOKEN="$W_TOKEN" "$GO" test -json -count=1 "${W_REST[@]}" >"$W_TMP/report" 2>"$W_TMP/err"; W_RC=$?
cat "$W_TMP/err" >>"$W_TMP/report"                        # non-JSON stderr (go: ... messages) stays in the report as text
wrap_report go "$W_TMP/report"; exit $?
