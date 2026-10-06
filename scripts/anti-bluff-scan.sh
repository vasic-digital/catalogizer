#!/usr/bin/env bash
# anti-bluff-scan.sh - thin wrapper (owner decision 2026-10-05): the path the project CLAUDE.md and the constitution name.
#
# Purpose   Calls scripts/audit/anti-bluff-scan.sh (the static scanner) and adds a read-only --files-from mode that scans ONLY the
#           listed files. Exit 0 = zero findings, 1 = any finding, 2 = usage or unsafe input (nothing was scanned).
# Usage     scripts/anti-bluff-scan.sh [--root DIR] [--files-from FILE] [--] [DIR]
#             no --files-from  the whole tree under DIR (or --root, or the git toplevel): the scanner's own behaviour.
#             --files-from F   F lists one path per line, relative to the root. Each path must be a LITERAL relative path
#                              (safe_declpath of scripts/repo/lib_safe.sh), a regular file, with no symlink on any component.
#                              The scan runs on a private COPY of exactly those files, so a violation in an unlisted file is
#                              never seen and the real tree is never written.
# Output    findings as the scanner prints them (TSV: file, line, kind, excerpt) on stdout; diagnostics on stderr.
# Side effects  a temp directory (removed on exit). Needs bash 4+.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INNER="$HERE/audit/anti-bluff-scan.sh"
# shellcheck source=repo/lib_safe.sh
. "$HERE/repo/lib_safe.sh"

die() { echo "anti-bluff-scan: REFUSED reason=$1 ${2:-}" >&2; exit 2; }
ROOT=""; LIST=""; POS=""
while [ $# -gt 0 ]; do
  case "$1" in
    --) shift; break ;;
    --root)         [ $# -ge 2 ] || die missing_value "--root"; ROOT="$2"; shift 2 ;;
    --root=*)       ROOT="${1#--root=}"; shift ;;
    --files-from)   [ $# -ge 2 ] || die missing_value "--files-from"; LIST="$2"; shift 2 ;;
    --files-from=*) LIST="${1#--files-from=}"; shift ;;
    -*)             die unknown_option "$(printf '%q' "$1")" ;;
    *)              [ -z "$POS" ] || die extra_operand "$(printf '%q' "$1")"; POS="$1"; shift ;;
  esac
done
if [ $# -gt 0 ]; then [ -z "$POS" ] && [ $# -eq 1 ] || die extra_operand; POS="$1"; fi
[ -z "$ROOT" ] || [ -z "$POS" ] || die root_given_twice
ROOT="${ROOT:-$POS}"
if [ -z "$ROOT" ]; then ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || die no_root "(not in a git tree; pass --root)"; fi
case "$ROOT" in -*) die option_like_root "$(printf '%q' "$ROOT")" ;; esac
safe_dir_arg "$ROOT" || die unsafe_root "$(printf '%q' "$ROOT")"
[ -d "$ROOT" ] || die root_not_a_directory "$(printf '%q' "$ROOT")"
ROOT="$(cd "$ROOT" && pwd -P)"
[ -r "$INNER" ] || die inner_scanner_missing "$INNER"

T="$(mktemp -d "${TMPDIR:-/tmp}/abs_wrap.XXXXXX")" || die no_tempdir
trap 'rm -rf "$T"' EXIT
RES="$T/findings.tsv"; : > "$RES"

if [ -z "$LIST" ]; then
  # whole-tree mode: the scanner's own contract; findings are re-checked against its exit code (a finding with rc 0 is still a failure)
  SCAN_ROOT="$ROOT"
else
  case "$LIST" in -*) die option_like_list "$(printf '%q' "$LIST")" ;; esac  # MUT:list-optionlike
  safe_dir_arg "$LIST" || die unsafe_list_path "$(printf '%q' "$LIST")"
  [ -f "$LIST" ] && [ -r "$LIST" ] || die list_unreadable "$(printf '%q' "$LIST")"
  SCAN_ROOT="$T/tree"; mkdir -p "$SCAN_ROOT"
  n=0
  while IFS= read -r line || [ -n "$line" ]; do
    safe_declpath "$line" || die unsafe_path "$(printf '%q' "$line")"  # MUT:declpath
    # no symlink on any component (never follow a link out of the tree), and the target must be a regular file
    acc="$ROOT"; rest="$line"
    while [ -n "$rest" ]; do
      comp="${rest%%/*}"; case "$rest" in */*) rest="${rest#*/}" ;; *) rest="" ;; esac
      acc="$acc/$comp"
      if [ -L "$acc" ]; then die symlink_component "$(printf '%q' "$line")"; fi  # MUT:symlink
    done
    [ -f "$acc" ] || die not_a_regular_file "$(printf '%q' "$line")"
    mkdir -p -- "$SCAN_ROOT/$(dirname -- "$line")" || die copy_failed "$(printf '%q' "$line")"
    cp -- "$acc" "$SCAN_ROOT/$line" || die copy_failed "$(printf '%q' "$line")"
    n=$((n+1))
  done < "$LIST"
  [ "$n" -gt 0 ] || die empty_list "(an empty list scans nothing and must not read as clean)"  # MUT:empty
fi

bash "$INNER" "$SCAN_ROOT" > "$RES" 2>"$T/inner.err"; rc=$?
cat "$RES"
[ ! -s "$T/inner.err" ] || cat "$T/inner.err" >&2
if [ "$rc" -ge 2 ]; then echo "anti-bluff-scan: inner scanner failed rc=$rc" >&2; exit 2; fi
if [ "$rc" -ne 0 ] || [ -s "$RES" ]; then exit 1; fi  # MUT:exit
exit 0
