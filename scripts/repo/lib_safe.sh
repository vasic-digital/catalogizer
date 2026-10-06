#!/usr/bin/env bash
# lib_safe.sh - argument validators shared by the WP-04 CPA helpers (check_no_ci, check_revision_headers, scope_check,
# validate_cheap, commit_recursive, push_recursive). Sourced, never run.
#
# Why     a value that comes from .gitmodules, a remote name or URL, a declared path, a branch or a run id can reach a git or
#         shell argument. Each helper validates it with one of these functions BEFORE use and still places `--` before every
#         path or ref operand, so a value that slips past one layer is still a plain operand (an automated review flagged
#         argument injection in a sibling script).
# Rule    every function returns 0 (safe) or 1 (unsafe) and prints nothing; the caller refuses with exit 20 and names the value
#         with `%q`-style quoting. Control characters (including tab and newline, which would break the TSV stores and the
#         trailers) are unsafe everywhere.
# Never   none of these runs git or touches the file system, except safe_branch (git check-ref-format, no repository needed).
# Paths   a path a caller DECLARES (a change set, a commit path, a held row, a --repo operand) is a LITERAL path: safe_declpath refuses every
#         spelling that a path-matching layer could read differently from a literal (`.`, a `./` prefix, a `/./` or `/.` component, a trailing
#         `/`, the glob characters `* ? [`, a leading `:` pathspec magic). Callers also run git with GIT_LITERAL_PATHSPECS=1, so a value that
#         slips past this layer is still never a pattern. safe_relpath alone is NOT enough for a declared path (WF2 review B1).
LC_ALL=C

_no_ctl() { case "$1" in *[[:cntrl:]]*) return 1 ;; esac; return 0; }

# safe_relpath <p>: relative path, no leading dash, no `..` component, no control character, no backslash.
safe_relpath() {
  local p="$1"
  [ -n "$p" ] || return 1
  _no_ctl "$p" || return 1
  case "$p" in /*|-*|*\\*|..|../*|*/..|*/../*|./..|*//*) return 1 ;; esac
  return 0
}
# safe_declpath <p>: a declared path: safe_relpath plus a canonical literal spelling (see the Paths note above).
safe_declpath() {
  safe_relpath "$1" || return 1
  case "$1" in .|./*|*/./*|*/.|*/|:*|*'*'*|*'?'*|*'['*) return 1 ;; esac
  return 0
}
# lr_exact <branch>: stdin is `git ls-remote` output; prints the object name of the ref refs/heads/<branch> EXACTLY (a tail-matching decoy
# such as refs/decoy/refs/heads/<branch> is never taken for it; WF2 review I3), nothing when the remote has no such ref.
lr_exact() { awk -v ref="refs/heads/$1" '$2==ref {print $1; exit}'; }
# safe_runid <id>: the run id of a CPA run (also a path component).
safe_runid() { [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]; }
# cpa_runid <repo dir> <sha>: the run id named by the last line `CPA-Run: <id>` of the commit message (ONE parser for S1, S6 and the merge path;
# a non-trailer attribution line after it changes nothing), nothing and status 1 when the line is absent or the id is no safe run id (a value
# such as `../elsewhere` can never name a path outside the audit directory; WF3 review m3, X2, X3).
cpa_runid() { local id; id="$(git -C "$1" log -1 --format=%B "$2" 2>/dev/null | awk 'index($0,"CPA-Run: ")==1 {v=substr($0,10)} END{print v}')"; safe_runid "$id" || return 1; printf '%s' "$id"; }
# cpa_row <commits.tsv> <sha>: the run's commits.tsv holds a row whose second column is the sha (clause (a) of the CPA-commit predicate).
cpa_row() { [ -f "$1" ] && awk -F'\t' -v s="$2" '$2==s {f=1} END{exit !f}' "$1"; }
# gitlinks <repo dir>: the gitlink paths of the index, each NUL-terminated. Pure bash on purpose: an awk that reads or prints NUL (RS/ORS "\0", printf "\0")
# is not portable to the mawk of the pinned container images (WF3 review, container portability).
gitlinks() { local e; while IFS= read -r -d '' e; do case "$e" in 160000\ *) printf '%s\0' "${e#*$'\t'}" ;; esac; done < <(git -C "$1" ls-files -s -z 2>/dev/null); }
# gitlinks_ok <repo dir>: the index listing that gitlinks reads succeeds. gitlinks runs in a process substitution, so its own status is lost;
# every caller checks this FIRST and refuses 20 git_listing_failed, so a failing listing is never read as "no submodules" (WF5 review F3).
gitlinks_ok() { git -C "$1" ls-files -s -z >/dev/null 2>&1; }
# known_tip <repo dir> <remote> <branch>: the last fetched tip of refs/remotes/<remote>/<branch> when it is a commit held locally: what an UNREACHABLE
# remote last held as far as this clone knows. Prints nothing when there is none (the remote is then unknown, never "empty"; WF3 review I-1).
known_tip() { local t; t="$(git -C "$1" rev-parse -q --verify "refs/remotes/$2/$3^{commit}" 2>/dev/null)" || return 0; safe_sha "$t" && printf '%s' "$t"; return 0; }
# safe_remote <name>: a git remote name used as an operand of ls-remote / push.
safe_remote() { [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] && [ "${1: -5}" != ".lock" ]; }
# safe_url <u>: a remote URL; never an option-like value, never an `ext::` transport (it runs a command).
safe_url() { [ -n "$1" ] && _no_ctl "$1" && case "$1" in -*|ext::*|*"::"*) return 1 ;; esac; return 0; }
# safe_sha <s>: a full object name.
safe_sha() { [[ "$1" =~ ^[0-9a-f]{40}$ || "$1" =~ ^[0-9a-f]{64}$ ]]; }
# safe_branch <b>: a branch name that git itself accepts and that is no option.
safe_branch() { [ -n "$1" ] && case "$1" in -*) return 1 ;; esac; _no_ctl "$1" && git check-ref-format --branch "$1" >/dev/null 2>&1; }
# safe_line <s>: a single-line value for a trailer or a TSV field.
safe_line() { _no_ctl "$1"; }
# safe_dir_arg <d>: a directory operand given on a command line (repository, root, run dir): not option-like, no control character.
safe_dir_arg() { [ -n "$1" ] && _no_ctl "$1" && case "$1" in -*) return 1 ;; esac; return 0; }
