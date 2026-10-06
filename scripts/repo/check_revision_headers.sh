#!/usr/bin/env bash
# T040 helper: check_revision_headers.sh - section 11.4.44 revision header check on Markdown files.
#
# Usage   check_revision_headers.sh [--root <repo>] [--files-from <list>] [--measure] [<path> ...]
#           --root        repository the paths are relative to (default: the current directory's work tree)
#           --files-from  one relative path per line (the declared files of a change set)
#           --measure     baseline measurement: no refusal; prints one row per tracked `*.md` file of the repository whose class
#                         applies the check and that lacks the header: `revision_header<TAB><path><TAB>missing<TAB>1`
#                         (the key `(revision_header, path, missing)` of validate_baselines/revision_header.tsv, T040)
#         The class tables are the ones next to this script unless --exemptions/--classes/--fixture-roots are given (they are
#         handed to check_class.sh unchanged).
# Rule    a file passes when its first 40 lines hold BOTH a `Revision` and a `Last modified` field, each either as a Markdown
#         table row whose first cell is that name (`| Revision | 3 |`) or as a bold line in either form (`**Revision:** 3`,
#         `**Revision**: 3`); anything else, including a plain `Revision: 3` line, fails. Only `*.md` files are judged. A file
#         whose check class does not apply `revision_header` (generated, legacy-collection, evidence, patches, fixtures root
#         rows) is skipped; legacy_row_not_dropped (a legacy root report edited while its row is kept) is validate_cheap's.
#         --assume-class-applied: the caller (validate_cheap.sh) already applied the class of its own table set; no class lookup.
#         A declared path that does not exist (a deletion) is not judged. A declared symlink is classified by its link type, never followed: where the
#         class applies the check it is reported `symlink_not_judged<TAB>revision_header<TAB><path>` (stdout, status unchanged), never skipped silently.
# Exits   0 all judged files pass (always with --measure); 10 at least one judged file fails; 20 usage, unsafe path, unreadable list.
# Output  per failing file `fail<TAB>revision_header<TAB><path>`; per skipped file nothing (--verbose adds `skip<TAB>..`).
# Never   writes anything. check_class.sh takes no `--`, so the path is validated by safe_relpath (no leading dash) before it is given.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$D/lib_safe.sh"
die() { echo "check_revision_headers: $1: $2" >&2; exit 20; }
ROOT=""; LIST=""; MEASURE=0; PATHS=(); CCOPT=(); ASSUME=0
while [ $# -gt 0 ]; do
  case "$1" in
    --root) [ $# -ge 2 ] || die usage "--root needs a value"; safe_dir_arg "$2" || die unsafe_root "$(printf '%q' "$2")"; ROOT="$2"; shift 2 ;;
    --files-from) [ $# -ge 2 ] || die usage "--files-from needs a value"; LIST="$2"; shift 2 ;;
    --measure) MEASURE=1; shift ;;
    --assume-class-applied) ASSUME=1; shift ;;
    --exemptions|--classes|--fixture-roots) [ $# -ge 2 ] || die usage "$1 needs a value"; CCOPT+=("$1" "$2"); shift 2 ;;
    --) shift; while [ $# -gt 0 ]; do PATHS+=("$1"); shift; done ;;
    -*) die usage "unknown option $(printf '%q' "$1")" ;;
    *) PATHS+=("$1"); shift ;;
  esac
done
if [ -n "$LIST" ]; then
  [ -r "$LIST" ] || die list_unreadable "$(printf '%q' "$LIST")"
  while IFS= read -r l || [ -n "$l" ]; do [ -n "$l" ] && PATHS+=("$l"); done < "$LIST"
fi
if [ -z "$ROOT" ]; then ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || die not_a_repository "no --root and not inside a work tree"; fi
[ -d "$ROOT" ] || die not_a_repository "$(printf '%q' "$ROOT")"
ROOT="$(cd "$ROOT" && pwd)" || die not_a_repository "$ROOT"
for p in "${PATHS[@]+"${PATHS[@]}"}"; do safe_relpath "$p" || die unsafe_path "$(printf '%q' "$p")"; done
if [ "$MEASURE" = 1 ] && [ "${#PATHS[@]}" -eq 0 ]; then
  git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1 || die not_a_repository "--measure needs a repository"
  lf="$(mktemp "${TMPDIR:-/tmp}/check_rh.XXXXXX")" || die internal mktemp
  trap 'rm -f "$lf"' EXIT; trap 'exit 143' TERM; trap 'exit 130' INT   # the listing temp file never outlives the run, whatever ends it (WF6 W6-5)
  git -C "$ROOT" ls-files -z -- '*.md' >"$lf" 2>/dev/null || { rm -f "$lf"; die git_listing_failed "$(printf '%q' "$ROOT")"; }   # never read a failing listing as "no files" (WF5 review F3)
  while IFS= read -r -d '' f; do PATHS+=("$f"); done < "$lf"; rm -f "$lf"
fi
has_header() { # has_header <file>: first 40 lines hold both fields
  head -n 40 -- "$1" 2>/dev/null | LC_ALL=C awk '
    /^\|[ \t]*Revision[ \t]*\|/ || /^\*\*Revision:\*\*/ || /^\*\*Revision\*\*:/ { r=1 }
    /^\|[ \t]*Last modified[ \t]*\|/ || /^\*\*Last modified:\*\*/ || /^\*\*Last modified\*\*:/ { m=1 }
    END { exit !(r && m) }'
}
RC=0
for p in "${PATHS[@]+"${PATHS[@]}"}"; do
  case "$p" in *.md) ;; *) continue ;; esac
  f="$ROOT/$p"
  if [ -L "$f" ]; then lnk=1; else lnk=0; [ -f "$f" ] || continue; fi
  if [ "$ASSUME" = 1 ]; then c=apply; else
  c="$(cd "$ROOT" && "$D/check_class.sh" "${CCOPT[@]+"${CCOPT[@]}"}" --check revision_header "$p" 2>&1)" || die class_loader_failed "$p: $c"
  fi
  [ "$c" = apply ] || continue
  if [ "$lnk" = 1 ]; then [ "$MEASURE" = 1 ] || printf 'symlink_not_judged\trevision_header\t%s\n' "$p"; continue; fi
  if ! has_header "$f"; then
    if [ "$MEASURE" = 1 ]; then printf 'revision_header\t%s\tmissing\t1\n' "$p"
    else printf 'fail\trevision_header\t%s\n' "$p"; RC=10; fi
  fi
done
exit "$RC"
