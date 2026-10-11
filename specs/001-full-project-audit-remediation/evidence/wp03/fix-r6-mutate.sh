#!/usr/bin/env bash
# Paired mutations for WP-03 round 11 (structural round). Every mutant is built in a SCRATCH copy (mktemp) of the verifier + scripts/audit inputs;
# the tracked tree is never written. Suites r8, r9, r10 (and, for the two T034 mutants, the full T031 matrix test_verify_repos.sh) must FAIL (caught)
# unless a mutant is listed EQUIVALENT with its reason. Usage: fix-r6-mutate.sh [JOBS]   Output: one block per mutant.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
V=scripts/repo/verify_repos.sh; D="$(mktemp -d)"; trap 'rm -rf "$D"' EXIT; JOBS="${1:-4}"
echo "verifier sha256 $(sha256sum $V | cut -c1-64)"; echo "tracked-tree status before: $(git status --porcelain scripts/repo | sha256sum | cut -c1-16)"
mk() { # name old new   -> scratch copy with ONE replacement
  mkdir -p "$D/$1/scripts/repo" "$D/$1/scripts/audit"; cp scripts/audit/*.py scripts/audit/*.txt "$D/$1/scripts/audit/" 2>/dev/null
  python3 - "$V" "$D/$1/scripts/repo/verify_repos.sh" "$2" "$3" <<'P' || { echo "pattern absent for $1"; return 1; }
import sys
s=open(sys.argv[1]).read(); assert sys.argv[3] in s, "pattern absent: "+sys.argv[3]
open(sys.argv[2],'w').write(s.replace(sys.argv[3],sys.argv[4],1))
P
  chmod +x "$D/$1/scripts/repo/verify_repos.sh"; }
SPEC=()
ONLY="${ONLY:-.}"
add() { [[ "$1" =~ $ONLY ]] || return 0; mk "$1" "$2" "$3" && SPEC+=("$1:${4:-r8,r9,r10}"); }
add M0_control_unmutated 'ISOLATION (WP-03 round 11' 'ISOLATION (WP-03 round 11'
# --- ancestry (P01-P10)
add MR1_replace_objects_dropped ' --no-replace-objects"' '"'
add MR2_graft_file_dropped 'export GIT_GRAFT_FILE=/nonexistent/verify_repos_no_grafts' ':'
add MR3_commitgraph_dropped ' -c core.commitGraph=false --no-replace-objects"' ' --no-replace-objects"'
# EQUIVALENT (probe P12/P12b): core.commitGraph=false already stops fetch writing a commit-graph, so dropping fetch.writeCommitGraph=false is not observable
add MR4_writecommitgraph_dropped ' -c fetch.writeCommitGraph=false' ''
# --- excludes (P20-P28)
add MX1_local_only_scope '--show-scope --type=path --get core.excludesFile' '--local --type=path --get core.excludesFile'
add MX2_worktree_scope_dropped 'local|worktree) locx=' 'local) locx='
add MX3_global_scope_counted 'local|worktree) locx=' 'local|worktree|global) locx='
add MX4_infoexclude_ignored '[ -f "$lx" ] && lxa+=' 'false && lxa+='
add MX5_global_counted '[ -n "${shown_l[$ulf]+x}" ] && continue' ':'
add MX6_rename_skip_dropped 'case "${ent:0:2}" in R*|C*|?R|?C) IFS= read -r -d '"''"' ent || true ;; esac
  done < "${outdir:?}/${idx:?}.status"
  $GIT "${GITQ[@]}" -C "$abs" ls-files -z --others' '
  done < "${outdir:?}/${idx:?}.status"
  $GIT "${GITQ[@]}" -C "$abs" ls-files -z --others'
# --- environment isolation (P28, P40-P48)
add MI1_isolation_skipped 'if ! _vr_env_clean || case' 'if false || case'
add MI2_bash_env_allowed 'PATH|HOME|LANG|LANGUAGE|LC_*|TZ|' 'PATH|HOME|BASH_ENV|ENV|LANG|LANGUAGE|LC_*|TZ|'
add MI3_rebuild_keeps_all 'if _vr_keep "$_vr_n" && [ "$_vr_n" != VERIFY_ISOLATED ]; then' 'if [ "$_vr_n" != VERIFY_ISOLATED ]; then'
add MI4_passenv_readmits_dangerous 'case "$1" in BASH*|ENV|SHELLOPTS|CDPATH|GLOBIGNORE|PS4|IFS|GIT_*) return 1 ;; esac' ':'
add MI5_exec_without_p '/bin/bash -p "${BASH_SOURCE[0]}" "$@"' '/bin/bash "${BASH_SOURCE[0]}" "$@"'
add MI6_git_family_passable 'BASH*|ENV|SHELLOPTS|CDPATH|GLOBIGNORE|PS4|IFS|GIT_*) return 1' 'BASH*|ENV|SHELLOPTS|CDPATH|GLOBIGNORE|PS4|IFS) return 1'
# (probe P47b: a bash -p process still passes an exported function on to its children, so detecting it matters; caught by r10 P47)
add MI7_functions_not_detected '< <("$_vr_envbin" -0)' '< <(compgen -e | tr "\n" "\0")'
add MI8_dirty_env_loop_unguarded '[ "${VERIFY_ISOLATED-}" = 1 ] && {' 'false && {'
# --- worker dispatch: EQUIVALENT once the environment is clean (a plain bash -c with no BASH_ENV in its environment sources nothing)
add EQ1_worker_without_p 'xargs -0 -n 3 -P "$JOBS" bash -p -c' 'xargs -0 -n 3 -P "$JOBS" bash -c'
# --- T034: the dirty / divergence checks against the full T031 matrix
add T034_no_dirty_check '[ "$n_dirty" -gt 0 ] && exit 13' ':' main
add T034_no_divergence_check '[ "$n_div" -gt 0 ] && exit 12' ':' main
runone() { # name:suites
  local name="${1%%:*}" suites="${1#*:}" t rc o res=""
  for t in $(echo "$suites" | tr ',' ' '); do
    case "$t" in main) f=test_verify_repos.sh ;; *) f=test_verify_repos_$t.sh ;; esac
    o="$(VERIFY_PASS_ENV=FAILCMD,MBCOUNT,TL_PARENT,JQFAIL,JQFAIL_MODE VR="$D/$name/scripts/repo/verify_repos.sh" bash scripts/repo/tests/$f 2>&1)"; rc=$?
    res="$res
  $t rc=$rc $(echo "$o" | grep '^SUMMARY')$(echo "$o" | grep '^FAIL' | head -3 | sed 's/^/\n    /' | tr -d '\r')"
  done
  printf '=== mutant %s%s\n' "$name" "$res" > "$D/$name.res"
}
export -f runone; export D
printf '%s\n' "${SPEC[@]}" | xargs -P "$JOBS" -I{} bash -c 'runone "{}"'
for s in "${SPEC[@]}"; do cat "$D/${s%%:*}.res"; done
echo "tracked-tree status after: $(git status --porcelain scripts/repo | sha256sum | cut -c1-16)"
