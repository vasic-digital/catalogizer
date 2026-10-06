#!/usr/bin/env bash
# T040 helper: scope_check.sh - CPA stage S2 (docs/16 section 12.2) scope check of a declared change set.
#
# Usage   scope_check.sh --root <repo> --paths-from <list> [--ev <relative evidence dir>] [--exceptions <tsv>]
#           --paths-from  the declared change set, one path per line, relative to the main root
#           --ev          evidence directory relative to the root (default specs/001-full-project-audit-remediation/evidence)
#           --exceptions  reviewed exceptions (default <root>/scripts/repo/exceptions.tsv when it exists; none otherwise);
#                         columns path, kind, file, wt_sha256, blob_sha256, reason (verify_repos.sh, T032)
# Refuses a declared change set that holds (reason named per path):
#   build_output     a path that the committed .gitignore ignores (`git check-ignore --no-index -q`, rc 0), never a name match on
#                    `build`: the planned tracked paths build/containers/go/Containerfile, build/components.json and
#                    scripts/build/verify_artifact.sh pass because the T004 rules re-include them
#   database         any `*.db` file but the tracked register database docs/workable_items.db
#   env_file         `.env` or `.env.*` (but .env.example, .env.sample, .env.template)
#   append_only      a change that alters or removes an earlier byte of $EV/ledger.jsonl, anchors.jsonl, deferrals.jsonl or
#                    flake_ledger.jsonl: the HEAD content must be a byte prefix of the working-tree content (a pure append passes)
#   blob_name        a file $EV/blobs/<name> whose name is not the sha256 of its content (or is no sha256 at all)
#   dirty_submodule  a submodule, at any depth, with a modified or untracked file that has no reviewed exceptions row (path, `dirty`,
#                    file) whose wt_sha256 is the file's current sha256 (judged for the whole repository, not only declared paths)
# NOT done in this slice: the S2 secret fold (detect-secrets absent on the host, T040a/T006), the private-key fold.
# Paths   every declared path is a LITERAL path (safe_declpath: no `.`, `./` prefix, `/./`, trailing `/`, `* ? [`, leading `:`) and is no
#         directory unless it is a gitlink; the gitlink test runs with GIT_LITERAL_PATHSPECS=1 (WF2 review B1): `./ev/ledger.jsonl` can no longer
#         dodge append_only. (`git check-ignore` runs without it: under literal pathspecs it stops matching .gitignore rules.)
# Exits   0 clean; 13 refused (S2 refusal code, UNCONFIRMED until the T042 owner accepts it for every reason above);
#         20 usage, unsafe path (absolute, `..`, leading dash, control character, glob character, non-canonical spelling), a declared
#         directory (`declared_directory`), root that is no repository, and a git read that failed (never read as clean): `git_listing_failed` (a
#         gitlink listing, at any depth), `git_status_failed` (a submodule's status), `submodule_git_unreadable` (a submodule with a `.git` entry that
#         git cannot read: only an ABSENT `.git` is an uninitialised submodule; WF6 W6-6), `git_tree_unreadable` (HEAD's tree cannot be listed, or HEAD does not resolve in a repository that has history (a zero-length or garbage branch ref,
#         an emptied packed-refs; WF7 M-2), so "is the store in HEAD?" cannot be answered; presence is decided only from a successful `git ls-tree`, a repository with no commit yet holds
#         no store; WF6 W6-7).
# Links   a declared store or blob path is judged by what git will COMMIT: a symlink (the path itself or any parent directory of it) or a path whose
#         HEAD entry is a symlink (a type change rewrites every byte) is refused append_only / blob_name; `-f`, `stat`, `cmp` and `sha256sum` follow a
#         link and would judge the link's target instead (WF3 review B-1).
# Never   writes, stages or changes anything. Each repository is judged by its own work tree; every path follows `--` or `HEAD:`.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$D/lib_safe.sh"
# NOT exported globally: `git check-ignore` reads a .gitignore match through the pathspec machinery and would stop matching under
# GIT_LITERAL_PATHSPECS=1 (build_output would pass); the declared paths are already validated literal, and the one pathspec call
# that takes a declared path other than check-ignore (the gitlink test) sets the variable inline.
die() { echo "scope_check: $1: $2" >&2; exit 20; }
trap 'exit 143' TERM; trap 'exit 130' INT
ROOT=""; LIST=""; EV="specs/001-full-project-audit-remediation/evidence"; EXC=""; HAVE_EXC=0
while [ $# -gt 0 ]; do
  case "$1" in
    --root) [ $# -ge 2 ] || die usage "--root needs a value"; safe_dir_arg "$2" || die unsafe_root "$(printf '%q' "$2")"; ROOT="$2"; shift 2 ;;
    --paths-from) [ $# -ge 2 ] || die usage "--paths-from needs a value"; LIST="$2"; shift 2 ;;
    --ev) [ $# -ge 2 ] || die usage "--ev needs a value"; safe_declpath "${2%/}" || die unsafe_path "--ev $(printf '%q' "$2")"; EV="${2%/}"; shift 2 ;;
    --exceptions) [ $# -ge 2 ] || die usage "--exceptions needs a value"; EXC="$2"; HAVE_EXC=1; shift 2 ;;
    *) die usage "unknown argument $(printf '%q' "$1")" ;;
  esac
done
[ -n "$ROOT" ] || die usage "--root is required"
[ -n "$LIST" ] || die usage "--paths-from is required"
[ -r "$LIST" ] || die list_unreadable "$(printf '%q' "$LIST")"
[ -d "$ROOT" ] || die not_a_repository "$(printf '%q' "$ROOT")"
ROOT="$(cd "$ROOT" && git rev-parse --show-toplevel 2>/dev/null)" || die not_a_repository "root"
if [ "$HAVE_EXC" = 0 ]; then EXC="$ROOT/scripts/repo/exceptions.tsv"; [ -f "$EXC" ] || EXC=/dev/null; fi
[ -r "$EXC" ] || die exceptions_unreadable "$(printf '%q' "$EXC")"
PATHS=()
while IFS= read -r l || [ -n "$l" ]; do
  [ -n "$l" ] || continue
  safe_declpath "$l" || die unsafe_path "$(printf '%q' "$l")"
  # a declared directory is no declared path; only a gitlink (a pin move) is one: the index entry whose path EQUALS the declared path decides, never the
  # first entry below it (WF3 review m1)
  if [ -d "$ROOT/$l" ] && [ ! -L "$ROOT/$l" ] && [ "$(GIT_LITERAL_PATHSPECS=1 git -C "$ROOT" ls-files -s -z -- "$l" 2>/dev/null | tr '\0' '\n' | awk -F'\t' -v p="$l" '{ split($1, a, " "); if ($2 == p && a[1] == "160000") f = 1 } END { print f + 0 }')" != 1 ]; then
    die declared_directory "$(printf '%q' "$l") is a directory: declare its files"
  fi
  PATHS+=("$l")
done < "$LIST"
RC=0
refuse() { printf 'refused\t%s\t%s\n' "$1" "$2"; RC=13; }
W="$(mktemp -d "${TMPDIR:-/tmp}/scope_check.XXXXXX")" || die internal "mktemp"; trap 'rm -rf "$W"' EXIT
STORES="ledger.jsonl anchors.jsonl deferrals.jsonl flake_ledger.jsonl"
has_link() { # has_link <p>: the path or one of its parent directories is a symlink in the work tree
  local acc="$ROOT" part IFS=/
  for part in $1; do acc="$acc/$part"; [ -L "$acc" ] && return 0; done
  return 1; }
# head_mode <p>: sets HM to the mode of the HEAD entry of <p> (empty when HEAD has no such entry or the repository has no commit yet). Presence is decided ONLY
# from a successful `git ls-tree`: `cat-file -e HEAD:<p>` also fails (rc 128) when HEAD's tree cannot be read, which read a rewritten store as a new one
# (WF6 W6-7). An unreadable HEAD tree is refused 20 git_tree_unreadable. Runs in this shell (no subshell around it), so the refusal is effective.
HM=""
head_mode() {
  local o; HM=""
  if ! git -C "$ROOT" rev-parse -q --verify HEAD >/dev/null 2>&1; then
    # "no commit yet" is decided from the repository, not from a failing rev-parse: a zero-length or garbage branch ref, a detached HEAD that does not
    # resolve, or an emptied packed-refs fails the same call in a repository that HAS commits, and a rewritten store would read as a new one (WF7 M-2).
    # An unborn branch has no ref file and an empty HEAD reflog.
    local hb rp lg
    hb="$(git -C "$ROOT" symbolic-ref -q HEAD 2>/dev/null)" || die git_tree_unreadable "HEAD does not resolve and is not an unborn branch"
    rp="$(git -C "$ROOT" rev-parse --git-path "$hb" 2>/dev/null)"; lg="$(git -C "$ROOT" rev-parse --git-path logs/HEAD 2>/dev/null)"
    case "$rp" in /*) ;; *) rp="$ROOT/$rp" ;; esac; case "$lg" in /*) ;; *) lg="$ROOT/$lg" ;; esac
    if [ -e "$rp" ] || [ -s "$lg" ]; then die git_tree_unreadable "HEAD does not resolve but the repository has history ($hb): its ref is unreadable"; fi
    return 0
  fi
  o="$(git -C "$ROOT" ls-tree HEAD -- "$1" 2>/dev/null)" || die git_tree_unreadable "$(printf '%q' "$1"): HEAD's tree cannot be listed"
  HM="${o%% *}"
}
for p in "${PATHS[@]+"${PATHS[@]}"}"; do
  base="${p##*/}"
  # databases
  case "$base" in *.db) [ "$p" = docs/workable_items.db ] || { refuse "$p" database; continue; } ;; esac
  # env files
  case "$base" in .env.example|.env.sample|.env.template) ;; .env|.env.*) refuse "$p" env_file; continue ;; esac
  # append-only stores
  if [ "${p%/*}" = "$EV" ]; then
    for s in $STORES; do
      [ "$base" = "$s" ] || continue
      head_mode "$p"
      if has_link "$p" || [ "$HM" = 120000 ]; then refuse "$p" append_only; continue 2; fi
      if [ -n "$HM" ]; then
        if [ ! -f "$ROOT/$p" ]; then refuse "$p" append_only; continue 2; fi
        git -C "$ROOT" cat-file blob "HEAD:$p" > "$W/head.blob" 2>/dev/null || { refuse "$p" append_only; continue 2; }
        sz="$(stat -c %s "$W/head.blob")"
        if [ "$(stat -c %s "$ROOT/$p")" -lt "$sz" ] || ! cmp -s -n "$sz" "$W/head.blob" "$ROOT/$p"; then refuse "$p" append_only; continue 2; fi
      fi
    done
  fi
  # content-addressed blobs
  if [ "${p%/*}" = "$EV/blobs" ]; then
    head_mode "$p"
    if has_link "$p" || [ "$HM" = 120000 ]; then refuse "$p" blob_name; continue; fi
    if [ -f "$ROOT/$p" ]; then
      h="$(sha256sum -- "$ROOT/$p" | cut -d' ' -f1)"
      [[ "$base" =~ ^[0-9a-f]{64}$ ]] && [ "$base" = "$h" ] || { refuse "$p" blob_name; continue; }
    fi
  fi
  # build output (the committed .gitignore decides, never a name match)
  if git -C "$ROOT" check-ignore --no-index -q -- "$p" 2>/dev/null; then
    [ "$p" = docs/workable_items.db ] || { refuse "$p" build_output; continue; }
  fi
done
# dirty submodules, at every depth (each repository judged by its own work tree)
walk() { # walk <repo dir> <path prefix from the main root>
  local dir="$1" pre="$2" sp sub entry code f rest wt row stf
  gitlinks_ok "$dir" || die git_listing_failed "$(printf '%q' "${pre:-.}")"
  while IFS= read -r -d '' sp; do
    safe_relpath "$sp" || die unsafe_path "gitlink $(printf '%q' "$sp")"
    sub="$dir/$sp"; [ -d "$sub" ] || continue
    # a `.git` entry (file or directory) that git cannot read is a BROKEN submodule, never an uninitialised one: only an absent `.git` is uninitialised (WF6 W6-6)
    if [ -e "$sub/.git" ] || [ -L "$sub/.git" ]; then
      git -C "$sub" rev-parse --git-dir >/dev/null 2>&1 || die submodule_git_unreadable "$(printf '%q' "$pre$sp") has a .git entry that git cannot read"
    else continue; fi
    # git walks UP to the parent repository when the .git entry is a directory that is no valid git dir (a truncated HEAD) or a dangling symlink, so
    # `rev-parse --git-dir` above succeeds for them: the work tree git resolves must be the submodule's own, else the submodule is broken (WF7 I-3)
    [ "$(cd "$sub" && git rev-parse --show-toplevel 2>/dev/null)" = "$(cd "$sub" && pwd -P)" ] || die submodule_git_unreadable "$(printf '%q' "$pre$sp") has a .git entry that is no repository of its own (git resolves it to another work tree)"
    stf="$W/st.$$.$RANDOM"
    git -C "$sub" status --porcelain=v1 -z --ignore-submodules=all >"$stf" 2>/dev/null || { rm -f "$stf"; die git_status_failed "$(printf '%q' "$pre$sp")"; }
    while IFS= read -r -d '' entry; do
      code="${entry:0:2}"; f="${entry:3}"
      case "$code" in R*|C*) IFS= read -r -d '' _orig ;; esac
      [ -n "$f" ] || continue
      if [ -e "$sub/$f" ] && [ -f "$sub/$f" ]; then wt="$(sha256sum -- "$sub/$f" | cut -d' ' -f1)"; else wt=deleted; fi
      row="$(awk -F'\t' -v p="$pre$sp" -v f="$f" -v w="$wt" '$1==p && $2=="dirty" && $3==f && $4==w {print "y"; exit}' "$EXC")"
      [ "$row" = y ] || refuse "$pre$sp" "dirty_submodule:$f"
    done < "$stf"; rm -f "$stf"
    walk "$sub" "$pre$sp/"
  done < <(gitlinks "$dir")
}
walk "$ROOT" ""
exit "$RC"
