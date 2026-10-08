#!/bin/bash -p
# T040 helper: check_no_ci.sh - the single home of the section 11.4.156 condition (no CI pipeline definition anchored at a
# repository root). Anti-mess invariant AM-G2 (T090) calls this helper instead of re-implementing the condition.
#
# Usage   check_no_ci.sh --root <repository> [--root <repository> ...]
# Judges  per root, at that root only (a pipeline file at a deeper path is never read by a forge):
#           .github/workflows/*.yml  .github/workflows/*.yaml  .gitlab-ci.yml  .gitea/workflows/*  .woodpecker.yml  .drone.yml
#           .circleci/ (any file)
#         over the tracked files and the untracked files that git does not ignore (what a commit could carry); an ignored file
#         can reach no remote and is not judged. `.github/workflows/README.md` is no pipeline definition.
# Scope   the main repository and the own-organisation repositories of a run; third-party repositories are outside 11.4.156 and
#         are simply not given as a root by the caller.
# Output  one line per violation on stdout: `ci_pipeline<TAB><root><TAB><relative path>`
# Exits   0 clean; 10 a pipeline definition exists; 20 usage, a root that is no repository, an unsafe root value.
# Never   writes, stages or changes anything; every path operand follows `--`.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$D/lib_safe.sh"
die() { echo "check_no_ci: $1: $2" >&2; exit 20; }
ROOTS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --root) [ $# -ge 2 ] || die usage "--root needs a value"; safe_dir_arg "$2" || die unsafe_root "$(printf '%q' "$2")"; ROOTS+=("$2"); shift 2 ;;
    *) die usage "unknown argument $(printf '%q' "$1")" ;;
  esac
done
[ "${#ROOTS[@]}" -gt 0 ] || die usage "at least one --root is required"
RC=0; lf=""
trap 'rm -f "$lf"' EXIT; trap 'exit 143' TERM; trap 'exit 130' INT   # the listing temp file never outlives the run, whatever ends it (WF6 W6-5)
for r in "${ROOTS[@]}"; do
  [ -d "$r" ] || die not_a_repository "$(printf '%q' "$r")"
  top="$(git -C "$r" rev-parse --show-toplevel 2>/dev/null)" || die not_a_repository "$(printf '%q' "$r")"
  [ "$(cd "$r" && pwd -P)" = "$(cd "$top" && pwd -P)" ] || die not_a_repository "$(printf '%q' "$r") is not a repository root"
  lf="$(mktemp "${TMPDIR:-/tmp}/check_no_ci.XXXXXX")" || die internal mktemp
  # the listing goes to a file first: a git that fails (an unreadable index, a corrupt repository) must never read as a clean root (WF5 review F3)
  git -C "$r" ls-files -z --cached --others --exclude-standard >"$lf" 2>/dev/null || { rm -f "$lf"; die git_listing_failed "$(printf '%q' "$r")"; }
  while IFS= read -r -d '' f; do
    case "$f" in
      .github/workflows/*.yml|.github/workflows/*.yaml|.gitlab-ci.yml|.gitea/workflows/*|.woodpecker.yml|.drone.yml|.circleci/*)
        # a path under .github/workflows or .gitea/workflows must be directly inside it for the *.yml forms only when it is a file name
        # without a further directory: the forge reads only the files of that directory
        case "$f" in
          .github/workflows/*/*) continue ;;
        esac
        printf 'ci_pipeline\t%s\t%s\n' "$r" "$f"; RC=10 ;;
    esac
  done < "$lf"
  rm -f "$lf"
done
exit "$RC"
