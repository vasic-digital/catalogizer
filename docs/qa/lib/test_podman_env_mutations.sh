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
mut no_normalize 's|DBUS_SESSION_BUS_ADDRESS="$(_qa_strip_guid "$DBUS_SESSION_BUS_ADDRESS")"|:|'
mut strip_from_first_comma 's|guid=\*) ;; \*)|*) ;; *)|'
mut keep_guid 's|guid=\*) ;;|NOMATCH=*) ;;|'
mut stale_cache 's|( cd "$root/docs/qa/lib/ctrctl" \&\& GOFLAGS=-mod=mod go build -o "$tmpb" . ) >\&2 \&\& \[ -x "$tmpb" \] \&\& mv -f "$tmpb" "$bin"|[ -x "$bin" ] \|\| ( cd "$root/docs/qa/lib/ctrctl" \&\& go build -o "$bin" . ) >\&2|'
mut forced_falls_back 's|if \[ "$forced" = containers \]; then|if false; then|'
mut no_wallclock 's| \&\& \[ "$SECONDS" -lt "$end" \]||'
mut skip_health_wait 's|^  qa_stack_wait_healthy "\$1"$|  true|'
mut no_name_check_start '/^qa_stack_start() {/,/^}/ s|_qa_stack_check_name "${1-}" \|\| return \$?|:|'
mut no_name_check_stop '/^qa_stack_stop() {/,/^}/ s|_qa_stack_check_name "${1-}" \|\| return \$?|:|'
mut loose_prefix 's|case "\$c" in "\$pfx"\*)|case "$c" in *)|'
# LIFE-FU-1/2/4 mutants
mut direct_build_no_tmp 's|go build -o "$tmpb" \. ) >\&2 \&\& \[ -x "$tmpb" \] \&\& mv -f "$tmpb" "$bin"|go build -o "$bin" . ) >\&2 \&\& [ -x "$bin" ]|'
mut pid_dollar_dollar 's|\${BASHPID:-\$\$}|$$|'
mut no_inspect_timeout 's|timeout "${QA_INSPECT_TIMEOUT:-10}" ||g'
mut cache_not_per_checkout 's|/$(printf %s "$root" \| sha256sum \| cut -c1-12)}"|}"|'
mut stop_rc_ignored '/^qa_stack_stop() {/,/^}/ s|stop $1 failed"; return 1;|stop $1 failed";|'
mut start_rc_ignored '/^qa_stack_start() {/,/^}/ s|start $1 failed"; return 1;|start $1 failed";|'
mut glob_not_disabled 's|case $- in \*f\*) ;; \*) noglob=0; set -f;; esac|noglob=1|'
mut glob_flag_leaks 's|\[ "$noglob" = 0 \] \&\& set +f|:|'
mut forced_wording_lost 's|= podman \]; then _qa_stack_log "backend=podman FORCED|= NOPE ]; then _qa_stack_log "backend=podman FORCED|'
mut rc_global 's|local rc=0; _qa_stack_run|_qa_stack_run|'
mut no_timeout_check 's|^  command -v timeout .*$|  :|'
# LIFE-FU-5..9 mutants
mut copy_instead_of_rename 's|mv -f "$tmpb" "$bin"|{ cp "$tmpb" "$bin" \&\& rm -f "$tmpb"; }|'
mut health_hang_reads_none 's|2>/dev/null)" \|\| h=unknown|2>/dev/null)"|; s|${h:-unknown}|${h:-none}|'
mut noglob_restore_unconditional 's|\[ "$noglob" = 0 \] \&\& set +f|set +f|'
mut no_start_timeout_preflight '/^qa_stack_start() {/,/^}/ s|_qa_stack_need_timeout \|\| return \$?|:|'
mut no_tmp_cleanup 's|^    rm -f -- "${tmpb:?}"$|    :|'
mut ctrctl_69_unmapped 's|r=\$?; \[ "$r" = 69 \] \&\& r=1; return|r=$?; return|'
# LIFE-FU-10..15 mutants
mut errexit_run_raw 's/"\$2" || r=\$?;/"$2"; r=$?;/'
mut errexit_wrapper_stop '/^qa_stack_stop() {/,/^}/ s/ || rc=\$?;/; rc=$?;/'
mut errexit_wrapper_start '/^qa_stack_start() {/,/^}/ s/ || rc=\$?;/; rc=$?;/'
mut unknown_health_fails 's/"exited "\*|"stopped "\*/"running unknown"|"exited "*|"stopped "*/'
mut empty_health_reads_none 's/${h:-unknown}/${h:-none}/'
mut ifs_not_local 's/^  local IFS .*$/  :/'
mut podman_fail_swallowed 's/>\/dev\/null || { r=\$?;/>\/dev\/null || { return 0; r=\$?;/'
mut podman_69_unmapped 's/\[ "\$r" = 69 \] \&\& r=1; return "\$r"; };;/return "\$r"; };;/'
mut last_state_none 's/${st:-unobserved}/${st:-none}/'
mut no_last_state 's/ (last state: ${st:-unobserved})//'
exit $bad
