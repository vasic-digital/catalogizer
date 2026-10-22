#!/usr/bin/env bash
# safe_submodule_init.sh - initialise submodules WITHOUT descending through constitution/.
# Why: constitution/ hub engines (llmctl, claude_toolkit) vendor constitutions; a recursive
# init through constitution/ cycles (docs/GOVERNANCE.md section 5). Safe sequence:
#   1. every non-constitution submodule: recursive is fine
#   2. constitution itself: one level only
#   3. optional hub engines (args): one level each, never --recursive
#   4. leftover scan (warn only, exit status unchanged, nothing moved or deleted): a directory
#      under <repo>/submodules/* or <repo>/constitution that holds a .git but is NOT a gitlink in
#      <repo>'s index (what git leaves behind when an engine drops a gitlink). <repo> = the root,
#      constitution, and every initialised constitution/submodules/<engine>.
# Usage: scripts/safe_submodule_init.sh [engine-name ...]   (run from or below the repo root)
set -uo pipefail
ROOT="$(git rev-parse --show-toplevel)" || { echo "not in a git repo" >&2; exit 2; }
cd "$ROOT" || exit 2
[ -f .gitmodules ] || exit 0
rc=0
git submodule status | awk '$2 != "constitution" {print $2}' \
  | xargs -r git submodule update --init --recursive -- || rc=1
if git submodule status | awk '{print $2}' | grep -qx constitution; then
  git submodule update --init constitution || rc=1
  for e in "$@"; do
    git -C constitution submodule update --init "submodules/$e" || rc=1
  done
fi
is_gitlink() { git -C "$1" ls-files -s -- "$2" 2>/dev/null | awk '$1=="160000"{f=1} END{exit !f}'; }
scan_leftovers() {  # $1 = repo dir relative to ROOT ("." for the root)
  local repo="$1" d rel
  [ -e "$repo/.git" ] || return 0
  for d in "$repo"/submodules/*/ "$repo"/constitution/; do
    [ -d "$d" ] || continue
    d="${d%/}"; [ -e "$d/.git" ] || continue
    rel="${d#"$repo"/}"
    [ "$repo" = "." ] && [ "$rel" = constitution ] && continue
    is_gitlink "$repo" "$rel" && continue
    echo "WARN leftover: ${d#./} holds a .git but is not a gitlink in ${repo#./}'s index (remove manually after checking it has no unpushed work)" >&2
  done
}
scan_leftovers .
if [ -e constitution/.git ]; then
  scan_leftovers constitution
  for e in constitution/submodules/*/; do scan_leftovers "${e%/}"; done
fi
exit $rc
