#!/usr/bin/env bash
# Paired mutation test for podman_env.sh: every mutant MUST make test_podman_env.sh fail; the unmutated helper MUST pass.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
M="$(mktemp -d "${TMPDIR:-/tmp}/podman-env-mut.XXXXXX")"; : "${M:?}"; trap 'rm -rf -- "${M:?}"' EXIT
H="$HERE/podman_env.sh"; bad=0
run() { HELPER="$1" bash "$HERE/test_podman_env.sh" >"$M/out" 2>&1; }
run "$H" && echo "PASS baseline helper passes" || { echo "FAIL baseline"; bad=1; }
mut() { # name sed-expr
  sed "$2" "$H" > "$M/$1.sh"; cmp -s "$H" "$M/$1.sh" && { echo "FAIL $1: mutation did not change the helper"; bad=1; return; }
  if run "$M/$1.sh"; then echo "FAIL $1 SURVIVED"; bad=1; else echo "PASS $1 killed"; fi
}
mut no_normalize 's|DBUS_SESSION_BUS_ADDRESS="${DBUS_SESSION_BUS_ADDRESS%%,\*}"|:|'
mut first_param_only 's|\${DBUS_SESSION_BUS_ADDRESS%%,\*}|${DBUS_SESSION_BUS_ADDRESS%,*}|'
mut skip_health_wait 's|^  qa_stack_wait_healthy "\$1"$|  true|'
mut no_name_check_start '/^qa_stack_start() {/,/^}/ s|_qa_stack_check_name "${1-}" \|\| return \$?|:|'
mut no_name_check_stop '/^qa_stack_stop() {/,/^}/ s|_qa_stack_check_name "${1-}" \|\| return \$?|:|'
mut loose_prefix 's|case "\$c" in "\$pfx"\*)|case "$c" in *)|'
exit $bad
