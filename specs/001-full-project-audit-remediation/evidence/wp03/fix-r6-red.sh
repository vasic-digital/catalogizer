#!/usr/bin/env bash
# RED for WP-03 round 11: the r10 test against a RECONSTRUCTED pre-fix verifier (scratch copy of the current one with the three round-11 / round-10 mechanisms
# switched back: isolation skipped, core.excludesFile decided with `--local`, core.commitGraph=false dropped). The original reviewed file (sha 4e3b321f) no longer
# exists as a file; its own RED run (before one assertion of the test was corrected) is fix-r6-red-r10-initial.txt. Reconstruction, not the original: UNCONFIRMED byte-equivalence.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D="$(mktemp -d)"; trap 'rm -rf "$D"' EXIT
mkdir -p "$D/scripts/repo" "$D/scripts/audit"; cp scripts/audit/*.py scripts/audit/*.txt "$D/scripts/audit/"
python3 - scripts/repo/verify_repos.sh "$D/scripts/repo/verify_repos.sh" <<'P'
import sys
s=open(sys.argv[1]).read()
for old,new in (('if ! _vr_env_clean || case','if false || case'),('--show-scope --type=path --get core.excludesFile','--local --type=path --get core.excludesFile'),(' -c core.commitGraph=false --no-replace-objects"',' --no-replace-objects"')):
    assert old in s, old; s=s.replace(old,new,1)
open(sys.argv[2],'w').write(s)
P
chmod +x "$D/scripts/repo/verify_repos.sh"
VR="$D/scripts/repo/verify_repos.sh" bash scripts/repo/tests/test_verify_repos_r10.sh
