#!/bin/bash -p
# T041 helper: commit_recursive.sh - CPA stage S5 commit in EXACTLY ONE repository (the name is kept from the task; it never
# recurses: round-22 review B1). It never commits inside a submodule and never creates a parent gitlink commit of its own.
#
# Usage   commit_recursive.sh --repo <dir> --run-dir <dir> --run-id <id> --message <text> [--attribution <trailer line>]
#                             [--foreign-from <file of shas>] [--held-from <tsv path<TAB>verdict path>] [--commits-tsv <file>]
#                             [--repo-key <key>] -- <path>...
#   --repo        the repository to commit in (the main repository in main mode, the target of a --repo run otherwise)
#   --run-dir     the CPA run directory (must exist); the run's deferral flags are read from it (record_deferral.sh --list) and
#                 commits.tsv is written there unless --commits-tsv is given
#   --repo-key    first column of the commits.tsv row (default `.`, the main repository; T042 gives the path under --repo)
#   --foreign-from  shas that S1 computed as foreign commits of this repository; they are named as `Foreign-Commit: <sha>` lines on the
#                 FIRST commit this run makes in the repository (no row with --repo-key in commits.tsv yet), only the ones that are
#                 ancestors of HEAD (so of the commit being made) and that no earlier `Foreign-Commit:` line already names
#   --held-from   paths held on a review verdict: the unheld paths are committed first, then one commit per verdict (in the order the
#                 verdicts first appear among the paths), each with `Awaits-Review: <verdict path>`
# Paths   every declared path (and every --held-from row) is a LITERAL path: safe_declpath refuses `.`, a `./` prefix, a `/./` component, a
#         trailing `/`, `* ? [` and a leading `:` (`unsafe_path`); an existing directory that is no gitlink is `declared_directory`; a held
#         row that names no declared path is `held_row_unmatched`, two rows with different verdicts for one path `held_row_duplicate`; every path
#         operand of git is given as `:(literal)<path>` (WF2 review B1), NOT through GIT_LITERAL_PATHSPECS, which would leak into the repository
#         hooks the commit runs (WF3 review m4).
# Commit  explicit paths only (`git add -A -- <paths>` then `git commit --only -- <paths>`), message from a file (-F), trailers:
#         CPA-Run, Deferred-Gates (when the run directory holds flags), Awaits-Review (held), Foreign-Commit (first commit),
#         then the attribution line. Repository hooks run normally (no --no-verify).
# Row     right after each commit and before anything else, `<repo key><TAB><sha><TAB><run id>` is appended to commits.tsv (clause (a)
#         of the T042 CPA-commit predicate), so a run killed after a commit still proves it; a kill between the commit and the append
#         is the one window left (UNCONFIRMED: closing it needs a prepare-commit-msg marker, owner T042).
# Exits   0 committed; 20 refusal: unsafe value, path inside a submodule (`path_in_submodule`), path neither tracked nor present,
#         run directory absent, commit refused (hook), nothing to commit. No row and no commit is made by a refusal that precedes the
#         first commit; a later group's refusal leaves the earlier groups' commits and rows in place (the next run integrates them).
# Never   a force, a rebase, a reset, an amend, a push, a network call; any value that reaches git is validated and follows `--`.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$D/lib_safe.sh"
MF=""; GOT=""; trap '[ -z "$MF" ] || rm -f "$MF" "$MF.err" "$MF.out"' EXIT; trap 'exit 143' TERM; trap 'exit 130' INT
die() { printf 'REASON\t%s\n' "$1"; echo "commit_recursive: $1: $2" >&2; exit 20; }
REPO=""; RUND=""; RID=""; MSG=""; HAVE_MSG=0; ATTR=""; FOREIGN=""; HELD=""; TSV=""; KEY="."; PATHS=(); EXPBR=""; EXPECT=""; P0=""
while [ $# -gt 0 ]; do
  case "$1" in
    --repo) [ $# -ge 2 ] || die usage "--repo needs a value"; REPO="$2"; shift 2 ;;
    --run-dir) [ $# -ge 2 ] || die usage "--run-dir needs a value"; RUND="$2"; shift 2 ;;
    --run-id) [ $# -ge 2 ] || die usage "--run-id needs a value"; RID="$2"; shift 2 ;;
    --message) [ $# -ge 2 ] || die usage "--message needs a value"; MSG="$2"; HAVE_MSG=1; shift 2 ;;
    --attribution) [ $# -ge 2 ] || die usage "--attribution needs a value"; ATTR="$2"; shift 2 ;;
    --foreign-from) [ $# -ge 2 ] || die usage "--foreign-from needs a value"; FOREIGN="$2"; shift 2 ;;
    --held-from) [ $# -ge 2 ] || die usage "--held-from needs a value"; HELD="$2"; shift 2 ;;
    --commits-tsv) [ $# -ge 2 ] || die usage "--commits-tsv needs a value"; TSV="$2"; shift 2 ;;
    --repo-key) [ $# -ge 2 ] || die usage "--repo-key needs a value"; KEY="$2"; shift 2 ;;
    --branch) [ $# -ge 2 ] || die usage "--branch needs a value"; EXPBR="$2"; shift 2 ;;
    --expect-index) [ $# -ge 2 ] || die usage "--expect-index needs a value"; EXPECT="$2"; shift 2 ;;
    --paths-from0) [ $# -ge 2 ] || die usage "--paths-from0 needs a value"; P0="$2"; shift 2 ;;
    --) shift; while [ $# -gt 0 ]; do PATHS+=("$1"); shift; done ;;
    *) die usage "unknown argument $(printf '%q' "$1")" ;;
  esac
done
safe_dir_arg "$REPO" || die unsafe_repo "$(printf '%q' "$REPO")"
safe_dir_arg "$RUND" || die unsafe_run_dir "$(printf '%q' "$RUND")"
safe_runid "$RID" || die unsafe_run_id "$(printf '%q' "$RID")"
[ "$HAVE_MSG" = 1 ] && [ -n "$MSG" ] || die usage "--message is required"
if [ -n "$P0" ]; then   # the paths as a NUL-separated file: no argument-length limit (BOUNDS-2)
  [ -r "$P0" ] || die usage "--paths-from0 file unreadable"; [ "${#PATHS[@]}" -eq 0 ] || die usage "--paths-from0 together with path arguments"
  while IFS= read -r -d '' _p; do PATHS+=("$_p"); done < "$P0"
fi
[ "${#PATHS[@]}" -gt 0 ] || die usage "at least one path is required"
[ -z "$EXPBR" ] || safe_branch "$EXPBR" || die unsafe_branch "$(printf '%q' "$EXPBR")"
declare -A EXP=()      # --expect-index: path -> "mode blob" (or "deleted -") judged at S2/S3; the commit is made only from exactly those blobs (TOCTOU-1)
if [ -n "$EXPECT" ]; then
  [ -r "$EXPECT" ] || die usage "--expect-index file unreadable"
  while IFS=$'\t' read -r _m _b _p || [ -n "${_m:-}" ]; do [ -n "${_p:-}" ] && EXP["$_p"]="$_m $_b"; done < "$EXPECT"
fi
safe_line "$ATTR" || die unsafe_attribution "control character in the attribution trailer"
safe_line "$KEY" && [ -n "$KEY" ] || die unsafe_repo_key "$(printf '%q' "$KEY")"
[ -d "$RUND" ] || die run_dir_absent "$(printf '%q' "$RUND")"
RUND="$(cd "$RUND" && pwd)"
[ -d "$REPO" ] || die not_a_repository "$(printf '%q' "$REPO")"
top="$(git -C "$REPO" rev-parse --show-toplevel 2>/dev/null)" || die not_a_repository "$(printf '%q' "$REPO")"
[ "$(cd "$REPO" && pwd -P)" = "$(cd "$top" && pwd -P)" ] || die not_a_repository "$(printf '%q' "$REPO") is not a repository root"
REPO="$top"
[ -n "$TSV" ] || TSV="$RUND/commits.tsv"
for p in "${PATHS[@]}"; do safe_declpath "$p" || die unsafe_path "$(printf '%q' "$p")"; done
# held table: path -> verdict
declare -A HV=(); VORDER=()
if [ -n "$HELD" ]; then
  [ -r "$HELD" ] || die held_unreadable "$(printf '%q' "$HELD")"
  while IFS=$'\t' read -r hp hv _rest || [ -n "${hp:-}" ]; do
    [ -n "${hp:-}" ] || continue
    safe_declpath "$hp" && safe_declpath "${hv:-}" || die unsafe_held_row "$(printf '%q' "$hp") $(printf '%q' "${hv:-}")"
    [ -z "${HV[$hp]+x}" ] || [ "${HV[$hp]}" = "$hv" ] || die held_row_duplicate "$(printf '%q' "$hp") is held on two different verdicts"
    HV["$hp"]="$hv"
  done < "$HELD"
fi
# every held row must name a declared path: a row that matches nothing would leave its file unheld, silently
declare -A DECL=(); for p in "${PATHS[@]}"; do DECL["$p"]=1; done
for hp in "${!HV[@]}"; do [ -n "${DECL[$hp]+x}" ] || die held_row_unmatched "$(printf '%q' "$hp") is a held row but not a declared path"; done
# foreign shas (validated up front, before any commit)
FSH=()
if [ -n "$FOREIGN" ]; then
  [ -r "$FOREIGN" ] || die foreign_unreadable "$(printf '%q' "$FOREIGN")"
  while IFS= read -r s || [ -n "$s" ]; do [ -n "$s" ] || continue; safe_sha "$s" || die unsafe_foreign "$(printf '%q' "$s")"; FSH+=("$s"); done < "$FOREIGN"
fi
# paths: never inside a submodule; tracked or present
GL=()
gitlinks_ok "$REPO" || die git_listing_failed "$(printf '%q' "$REPO")"
while IFS= read -r -d '' g; do GL+=("$g"); done < <(gitlinks "$REPO")
for p in "${PATHS[@]}"; do
  for g in "${GL[@]+"${GL[@]}"}"; do case "$p" in "$g"/*) die path_in_submodule "$(printf '%q' "$p") lies inside the submodule $(printf '%q' "$g")" ;; esac; done
  [ -e "$REPO/$p" ] || [ -L "$REPO/$p" ] || git -C "$REPO" ls-files --error-unmatch -- ":(literal)$p" >/dev/null 2>&1 || die path_unknown "$(printf '%q' "$p") is neither tracked nor present"
  if [ -d "$REPO/$p" ] && [ ! -L "$REPO/$p" ]; then   # a directory is no declared path; a gitlink (a pin move) is the one directory allowed
    isg=0; for g in "${GL[@]+"${GL[@]}"}"; do [ "$p" = "$g" ] && isg=1; done
    [ "$isg" = 1 ] || die declared_directory "$(printf '%q' "$p") is a directory: declare its files"
  fi
done
# groups: unheld first, then one per verdict in order of first appearance
UNHELD=(); declare -A GP=()
for p in "${PATHS[@]}"; do
  v="${HV[$p]-}"
  if [ -z "$v" ]; then UNHELD+=("$p"); else
    if [ -z "${GP[$v]+x}" ]; then VORDER+=("$v"); GP["$v"]="$p"; else GP["$v"]="${GP[$v]}"$'\n'"$p"; fi
  fi
done
export CPA_HELPER_PID=$$
have_row() { [ -f "$TSV" ] && awk -F'\t' -v k="$KEY" '$1==k {f=1} END{exit !f}' "$TSV"; }
make_commit() { # make_commit <verdict or ""> <paths...>
  local verdict="$1"; shift
  local mf deferred first=0 s tr="" lp=() x
  for x in "$@"; do lp+=(":(literal)$x"); done
  mf="$(mktemp "${TMPDIR:-/tmp}/cr_msg.XXXXXX")" || die internal mktemp; MF="$mf"
  have_row || first=1
  deferred="$("$D/record_deferral.sh" --run-dir "$RUND" --list 2>/dev/null)" || deferred=""
  { printf '%s\n\n' "$MSG"
    printf 'CPA-Run: %s\n' "$RID"
    [ -z "$deferred" ] || printf 'Deferred-Gates: %s\n' "$deferred"
    [ -z "$verdict" ] || printf 'Awaits-Review: %s\n' "$verdict"
    if [ "$first" = 1 ]; then
      for s in "${FSH[@]+"${FSH[@]}"}"; do
        git -C "$REPO" cat-file -e "$s^{commit}" 2>/dev/null || continue
        git -C "$REPO" merge-base --is-ancestor "$s" HEAD 2>/dev/null || continue
        git -C "$REPO" log --format='%(trailers:key=Foreign-Commit,valueonly,separator=%x0A)' HEAD 2>/dev/null | grep -qxF -- "$s" && continue
        printf 'Foreign-Commit: %s\n' "$s"
      done
    fi
    [ -z "$ATTR" ] || printf '%s\n' "$ATTR"
  } > "$mf"
  if ! git -C "$REPO" add -A -- "${lp[@]}" >/dev/null 2>"$mf.err"; then local e; e="$(head -c 300 "$mf.err")"; rm -f "$mf" "$mf.err"; die add_failed "$e"; fi
  if [ -n "$EXPECT" ]; then
    # (1) nothing is staged outside this group, so the commit below holds exactly the group; (2) every staged blob of the group IS the one judged at S3
    local sp ent mode sha ex=""; declare -A GRP=(); for x in "$@"; do GRP["$x"]=1; done
    if git -C "$REPO" rev-parse -q --verify HEAD >/dev/null 2>&1; then
      while IFS= read -r -d '' sp; do [ -n "${GRP[$sp]+x}" ] || { rm -f "$mf" "$mf.err"; die unrelated_staged_changes "$(printf '%q' "$sp") is staged and is not part of this group"; }; done \
        < <(git -C "$REPO" diff --cached --name-only -z HEAD 2>/dev/null)
    fi
    declare -A NOW=()
    while IFS= read -r -d '' ent; do
      mode="${ent%% *}"; sha="${ent#* }"; sha="${sha%% *}"; sp="${ent#*$'\t'}"; NOW["$sp"]="$mode $sha"
    done < <(git -C "$REPO" ls-files -s -z -- "${lp[@]}" 2>/dev/null)
    for x in "$@"; do
      ex="${EXP[$x]-}"; [ -n "$ex" ] || { rm -f "$mf" "$mf.err"; die changed_since_validation "$(printf '%q' "$x") was not judged"; }
      if [ "${ex%% *}" = deleted ]; then [ -z "${NOW[$x]+x}" ] || { rm -f "$mf" "$mf.err"; die changed_since_validation "$(printf '%q' "$x") reappeared after it was judged deleted"; }
      else [ "${NOW[$x]-}" = "$ex" ] || { rm -f "$mf" "$mf.err"; die changed_since_validation "$(printf '%q' "$x") holds other bytes now than the ones judged at S3"; }; fi
    done
  fi
  if [ -n "$EXPBR" ] && [ "$(git -C "$REPO" symbolic-ref -q HEAD 2>/dev/null)" != "refs/heads/$EXPBR" ]; then rm -f "$mf" "$mf.err"; die branch_moved "HEAD is no longer refs/heads/$EXPBR"; fi
  # the commit and its row are ONE window: a signal that arrives in it is held (a flag) and acted on after the row is written, never between the two (RES-3)
  GOT=""; trap 'GOT=143' TERM; trap 'GOT=130' INT; trap 'GOT=129' HUP
  if [ -n "$EXPECT" ]; then git -C "$REPO" commit -q -F "$mf" >"$mf.out" 2>"$mf.err"; else git -C "$REPO" commit -q -F "$mf" --only -- "${lp[@]}" >"$mf.out" 2>"$mf.err"; fi
  crc=$?
  if [ "$crc" != 0 ]; then
    local e; e="$(head -c 300 "$mf.err")$(head -c 300 "$mf.out")"; rm -f "$mf" "$mf.err" "$mf.out"; trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP; die commit_failed "$e"
  fi
  rm -f "$mf" "$mf.err" "$mf.out"
  s="$(git -C "$REPO" rev-parse HEAD)" || { trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP; die internal "rev-parse HEAD"; }
  if [ -n "$EXPECT" ]; then       # the committed tree entries are the judged ones: a race lost in the last milliseconds is caught here and NO row is written (the next run names the commit unrecorded)
    for x in "$@"; do
      ex="${EXP[$x]}"; ent="$(git -C "$REPO" ls-tree -z HEAD -- ":(literal)$x" 2>/dev/null | tr '\0' '\n' | head -1)"
      if [ "${ex%% *}" = deleted ]; then [ -z "$ent" ] || { trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP; die changed_since_validation "$s holds $(printf '%q' "$x"), judged deleted"; }
      else [ "${ent%% *} $(printf '%s' "$ent" | awk '{print $3}')" = "$ex" ] || { trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP; die changed_since_validation "$s holds other bytes for $(printf '%q' "$x") than the ones judged"; }; fi
    done
  fi
  printf '%s\t%s\t%s\n' "$KEY" "$s" "$RID" >> "$TSV" || { trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP; die commits_tsv_write_failed "$(printf '%q' "$TSV")"; }
  trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP
  [ -z "$GOT" ] || exit "$GOT"
  printf 'COMMIT\t%s\t%s\t%s\n' "$KEY" "$s" "${verdict:--}"
}
[ "${#UNHELD[@]}" -eq 0 ] || make_commit "" "${UNHELD[@]}"
for v in "${VORDER[@]+"${VORDER[@]}"}"; do
  mapfile -t gp <<< "${GP[$v]}"
  make_commit "$v" "${gp[@]}"
done
exit 0
