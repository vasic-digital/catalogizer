#!/usr/bin/env bash
# Paired mutations for the WP-03 round-10 fix (N1-N4). Every mutant is built in a SCRATCH copy of the repo/verifier + audit tree under mktemp (WP-03 re-review N5:
# the tracked tree is never written). Tests r8 and r9 must FAIL (caught) unless a mutant is noted EQUIVALENT.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
V=scripts/repo/verify_repos.sh; D="$(mktemp -d)"; trap 'rm -rf "$D"' EXIT
echo "verifier sha256 $(sha256sum $V | cut -c1-64)"; echo "tracked-tree status before: $(git status --porcelain scripts/repo | sha256sum | cut -c1-16)"
mut() { # name old new
  mkdir -p "$D/$1/scripts/repo" "$D/$1/scripts/audit"; cp scripts/audit/*.py scripts/audit/*.txt "$D/$1/scripts/audit/" 2>/dev/null
  python3 - "$V" "$D/$1/scripts/repo/verify_repos.sh" "$2" "$3" <<'P'
import sys
s=open(sys.argv[1]).read(); assert sys.argv[3] in s, "pattern absent: "+sys.argv[3]
open(sys.argv[2],'w').write(s.replace(sys.argv[3],sys.argv[4],1))
P
  chmod +x "$D/$1/scripts/repo/verify_repos.sh"; local rc o
  for t in r8 r9; do
    o="$(VR="$D/$1/scripts/repo/verify_repos.sh" bash scripts/repo/tests/test_verify_repos_$t.sh 2>&1)"; rc=$?
    echo "=== mutant $1 / $t: rc=$rc $(echo "$o" | grep '^SUMMARY')"; echo "$o" | grep '^FAIL' | head -4
  done
}
mut M0_control_unmutated 'N3: `bash verify_repos.sh` ignores' 'N3: `bash verify_repos.sh` ignores'
mut MX3_rename_skip_dropped 'case "${ent:0:2}" in R*|C*|?R|?C) IFS= read -r -d '"''"' ent || true ;; esac
  done < "${outdir:?}/${idx:?}.status"
  $GIT "${GITQ[@]}" -C "$abs" ls-files -z --others' '
  done < "${outdir:?}/${idx:?}.status"
  $GIT "${GITQ[@]}" -C "$abs" ls-files -z --others'
mut N2_commitgraph_dropped ' -c core.commitGraph=false"' '"'
mut N3_reexec_dropped 'case "$-" in *p*) ;; *) unset BASH_ENV ENV; exec /bin/bash -p "${BASH_SOURCE[0]}" "$@" ;; esac' ':'
mut N1_global_counted '[ -n "${shown_l[$ulf]+x}" ] && continue' ':'
mut N1_local_excludesfile_ignored '[ -f "$locx" ] && lxa+=' 'false && lxa+='
mut N1_infoexclude_ignored '[ -f "$lx" ] && lxa+=' 'false && lxa+='
mut N1_names_dropped 'hidnames="$hidnames $(printf '"'%q'"' "$ulf")"' ':'
mut M9_replace_ref_base_unscrub 'unset GIT_REPLACE_REF_BASE GIT_GRAFT_FILE GIT_SHALLOW_FILE' 'unset GIT_GRAFT_FILE'
echo "tracked-tree status after: $(git status --porcelain scripts/repo | sha256sum | cut -c1-16)"
