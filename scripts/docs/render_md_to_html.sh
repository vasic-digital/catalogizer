#!/usr/bin/env bash
# render_md_to_html.sh - regenerate the standalone .html sibling of Markdown docs (constitution 11.4.12, 11.4.65, 11.4.77, 11.4.190).
#
# Purpose  The HTML twin of a doc must be a COMPLETE responsive page (doctype, <head>, charset and
#          <meta name="viewport">), never a bare pandoc body fragment: a fragment has no viewport meta, so phones render it at
#          980px and the doc fails the 11.4.190 responsiveness mandate. This is the one canonical command for that render.
# Usage    render_md_to_html.sh [--check] FILE.md [FILE.md ...]
#          default  writes FILE.html next to each FILE.md (overwrites)
#          --check  writes nothing; exit 1 if an existing FILE.html differs from what would be generated, 0 otherwise
# Needs    pandoc >= 3 (its standard HTML5 template carries the viewport meta).
# Output   FILE.html ; the title is the first '# ' heading of FILE.md (file name when there is none).
# Exit     0 ok, 1 drift (--check) or a generated page without viewport meta, 2 usage / missing tool or input.
set -euo pipefail
check=0
if [ "${1:-}" = "--check" ]; then check=1; shift; fi
[ "$#" -ge 1 ] || { echo "usage: render_md_to_html.sh [--check] FILE.md ..." >&2; exit 2; }
command -v pandoc >/dev/null 2>&1 || { echo "render_md_to_html: pandoc not found" >&2; exit 2; }
rc=0
for md in "$@"; do
  [ -f "$md" ] || { echo "render_md_to_html: no such file: $md" >&2; exit 2; }
  out="${md%.md}.html"
  title="$(sed -n 's/^# \(.*\)$/\1/p' "$md" | head -n1)"
  [ -n "$title" ] || title="$(basename "$md" .md)"
  tmp="$(mktemp "${TMPDIR:-/tmp}/render-md-html.XXXXXX")"
  pandoc -f gfm -t html5 -s --metadata "title=$title" --metadata "lang=en" "$md" -o "$tmp"
  if ! grep -q '<meta name="viewport"' "$tmp"; then
    echo "render_md_to_html: generated page for $md has no viewport meta" >&2; rm -f "${tmp:?}"; rc=1; continue
  fi
  if [ "$check" = 1 ]; then
    if ! cmp -s "$tmp" "$out"; then echo "DRIFT $out"; rc=1; fi
    rm -f "${tmp:?}"
  else
    mv -f "$tmp" "$out"; echo "wrote $out"
  fi
done
exit $rc
