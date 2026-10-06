#!/usr/bin/env bash
# red_prefix_wp03.sh - a RED run of the FINAL scripts/repo/tests/test_verify_repos.sh against a RECONSTRUCTED pre-fix verifier.
#
# Purpose   The original pre-fix verifier (sha256 2ebb67e4...) was never committed and no copy survives, so the RED of the three
#           shim tests (git stash list, git merge-base, git rev-parse HEAD failing) against the FINAL test hash cannot be replayed from
#           it. This script reconstructs the pre-fix behaviour of exactly those three checks from the CURRENT verifier by textual
#           reversal (each replaced text must exist exactly once, else exit 2): the stash-list exit status unchecked, the
#           rev-parse HEAD exit status and emptiness unchecked, and an ancestry (merge-base) error read as a class instead of a
#           failure. These are the same reversals as the reviewer mutants MB1c, MB1b and Mdet of mutate_wp02_wp03.sh, applied together.
#           The RECONSTRUCTION IS NOT THE ORIGINAL FILE (UNCONFIRMED that the two are byte-equivalent); it proves only that the final
#           test cases stfail, hdfail, mbfail and mb2fail FAIL when those three checks are absent, and pass when they are present.
# Usage     bash scripts/audit/tests/red_prefix_wp03.sh [OUT_DIR]     (default OUT_DIR = a temp dir, removed on exit)
#           Writes OUT_DIR/prefix_verify_repos.sh, OUT_DIR/prefix.diff, OUT_DIR/red.txt (the full test transcript).
# Exit      0 when the three shim cases (and only failures among cases that exercise those checks) FAIL as required; 1 when a
#           required case did NOT fail; 2 when a reversal pattern is not found exactly once (the verifier changed).
# Side effects  temp files only; runs the test suite once (about two minutes). Needs git, jq, python3.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
OUT="${1:-}"; [ -n "$OUT" ] || { OUT="$(mktemp -d "${TMPDIR:-/tmp}/redpre.XXXXXX")"; trap 'rm -rf "${OUT:?}"' EXIT; }
mkdir -p "$OUT/repo" "$OUT/audit"; cp scripts/audit/org_of.py scripts/audit/own_orgs*.txt "$OUT/audit/"; cp scripts/repo/exceptions.tsv "$OUT/repo/"
python3 - scripts/repo/verify_repos.sh "$OUT/repo/verify_repos.sh" <<'PY' || exit 2
import sys
s = open(sys.argv[1]).read()
rev = [
 ('stash="$($GIT -C "$abs" stash list 2>/dev/null)" || { fail "git stash list failed"; return 1; }',
  'stash="$($GIT -C "$abs" stash list 2>/dev/null)"'),
 ('head="$($GIT -C "$abs" rev-parse HEAD 2>/dev/null)" || { fail "git rev-parse HEAD failed"; return 1; }\n  [ -n "$head" ] || { fail "empty HEAD"; return 1; }',
  'head="$($GIT -C "$abs" rev-parse HEAD 2>/dev/null)"'),
 ('case "$a" in 0) echo REMOTE-BEHIND; return 0 ;; 1) ;; *) return 1 ;; esac',
  'case "$a" in 0) echo REMOTE-BEHIND; return 0 ;; *) ;; esac'),
 ('case "$a" in 0) echo LOCAL-BEHIND; return 0 ;; 1) echo DIVERGED; return 0 ;; *) return 1 ;; esac',
  'case "$a" in 0) echo LOCAL-BEHIND; return 0 ;; *) echo DIVERGED; return 0 ;; esac'),
]
for old, new in rev:
    if s.count(old) != 1:
        print("pattern occurs %d times: %r" % (s.count(old), old[:80])); sys.exit(2)
    s = s.replace(old, new)
open(sys.argv[2], "w").write(s)
PY
chmod +x "$OUT/repo/verify_repos.sh"; diff -u scripts/repo/verify_repos.sh "$OUT/repo/verify_repos.sh" > "$OUT/prefix.diff"
echo "RECONSTRUCTED pre-fix verifier sha256=$(sha256sum "$OUT/repo/verify_repos.sh" | cut -c1-64) (current verifier sha256=$(sha256sum scripts/repo/verify_repos.sh | cut -c1-64)); diff lines: $(grep -c '^[-+][^-+]' "$OUT/prefix.diff")"
VR="$OUT/repo/verify_repos.sh" bash scripts/repo/tests/test_verify_repos.sh > "$OUT/red.txt" 2>&1; rc=$?
head -1 "$OUT/red.txt"; grep '^FAIL' "$OUT/red.txt"; tail -1 "$OUT/red.txt"; echo "suite rc=$rc"
need=0
for k in 'B1 git stash list fails' 'B1 git merge-base fails' 'B1 git rev-parse HEAD fails' 'B1 the second ancestry check'; do
  if grep -q "^FAIL $k" "$OUT/red.txt"; then echo "RED ok: $k"; else echo "NOT RED: $k"; need=1; fi
done
[ "$need" = 0 ]
