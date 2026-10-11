#!/usr/bin/env bash
# Paired mutations for the WP-03 R1-R5 fix round: each mutant is a copy of the current verifier with ONE edit; the r8 test must FAIL (caught) unless noted EQUIVALENT.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
V=scripts/repo/verify_repos.sh; D="$(mktemp -d)"; trap 'rm -rf "$D"' EXIT
echo "verifier sha256 $(sha256sum $V | cut -c1-64)"
mut() { # name  python-replace-old  new
  python3 - "$V" "$D/$1.sh" "$2" "$3" <<'P'
import sys
s=open(sys.argv[1]).read(); assert sys.argv[3] in s, "pattern absent: "+sys.argv[3]
open(sys.argv[2],'w').write(s.replace(sys.argv[3],sys.argv[4],1))
P
  chmod +x "$D/$1.sh"
  # the copy must sit beside org_of.py's relative path: keep it in the verifier's directory
  cp "$D/$1.sh" "scripts/repo/.mut_$1.sh"
  out="$(VR="scripts/repo/.mut_$1.sh" bash scripts/repo/tests/test_verify_repos_r8.sh 2>&1)"; rc=$?
  rm -f "scripts/repo/.mut_$1.sh"
  echo "=== mutant $1: rc=$rc $(echo "$out" | grep '^SUMMARY')"; echo "$out" | grep '^FAIL' | head -6
}
mut M1_no_replace_off 'export GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE=' 'export GIT_GRAFT_FILE='
mut M2_no_graft_off 'export GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE=/nonexistent/verify_repos_no_grafts' 'export GIT_NO_REPLACE_OBJECTS=1'
mut M3_commitgraph_write ' -c fetch.writeCommitGraph=false"' '"'
mut M4_hidden_not_counted 'hid=$((hid+1)); untracked=$((untracked+1))' 'hid=$((hid+1))'
mut M5_exclude_standard '--others --exclude-per-directory=.gitignore' '--others --exclude-standard'
mut M6_bash_no_p 'xargs -0 -n 3 -P "$JOBS" bash -p -c' 'xargs -0 -n 3 -P "$JOBS" bash -c'
mut M7_pin_U_dropped '$r.pin == "U" or ' ''
mut M8_untracked_cache_override ' -c core.untrackedCache=false' ''
mut M9_replace_ref_base_unscrub 'unset GIT_REPLACE_REF_BASE GIT_GRAFT_FILE GIT_SHALLOW_FILE' 'unset GIT_GRAFT_FILE'

mut M0_control_unmutated 'Object and ancestry resolution is part' 'Object and ancestry resolution is part'
