#!/usr/bin/env bash
# wrap-gradle.sh - runner wrapper for Gradle test tasks (tasks.md T053). Usage and exit statuses: lib/wrap_common.sh.
#   wrap-gradle.sh --run-token TOKEN [--results-dir DIR] [opts] -- GRADLE_ARGS...   runs ./gradlew GRADLE_ARGS... and judges the
#   JUnit XML files (**/build/test-results/**/TEST-*.xml under DIR, default .) written since the run started.
# WRAP_GRADLE_BIN overrides ./gradlew. --from-report takes one XML file or a directory of them. The live leg runs in IMG-ANDROID after WP-14.
set -u
. "$(dirname "$(readlink -f "$0")")/lib/wrap_common.sh"
wrap_parse "$@"
if [ -n "$W_FROM" ]; then wrap_report gradle "$W_FROM"; exit $?; fi
GR=${WRAP_GRADLE_BIN:-./gradlew}; wrap_need "$GR"; wrap_tmp
since=$(date +%s)
env EVREC_RUN_TOKEN="$W_TOKEN" "$GR" "${W_REST[@]}" >"$W_TMP/out" 2>&1 </dev/null; W_RC=$?
wrap_report gradle "$W_RESULTS" --since "$since"; exit $?
