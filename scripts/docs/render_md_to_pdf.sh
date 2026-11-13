#!/usr/bin/env bash
# render_md_to_pdf.sh - regenerate the .pdf sibling of Markdown docs without clipping (constitution 11.4.12, 11.4.65, 11.4.77).
#
# Purpose  Wide tables and long unbroken tokens were clipped at the right page edge when a PDF was rendered from the pandoc HTML with
#          default print rules (pandoc's body max-width and auto table layout). This is the one canonical PDF renderer: it renders the
#          canonical HTML (the same pandoc call as render_md_to_html.sh) with WeasyPrint and the print stylesheets below.
# Usage    render_md_to_pdf.sh [--check] FILE.md [FILE.md ...]
#          default  writes FILE.pdf next to each FILE.md (overwrites)
#          --check  writes nothing; runs scripts/docs/check_pdf_coverage.py on the existing FILE.pdf (exit 1 on missing tokens)
# Needs    pandoc >= 3, weasyprint, pdftotext (for --check)
# Rule     Orientation: a doc whose Markdown has a table header row with >= 5 columns is rendered A4 LANDSCAPE
#          (scripts/docs/print_landscape.css), all others A4 portrait. Both use scripts/docs/print.css
#          (auto table layout at 100% width, 7.5pt table text, word-breaking of cells, pre-wrap code).
# Currency The PDF carries the sha256 of its Markdown in its Keywords field (src-sha256:<hex>); check_twin_presence.py compares it.
# Exit     0 ok, 1 coverage failure (--check) or render failure, 2 usage / missing tool or input.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
check=0
if [ "${1:-}" = "--check" ]; then check=1; shift; fi
[ "$#" -ge 1 ] || { echo "usage: render_md_to_pdf.sh [--check] FILE.md ..." >&2; exit 2; }
for t in pandoc weasyprint pdftotext; do command -v "$t" >/dev/null 2>&1 || { echo "render_md_to_pdf: $t not found" >&2; exit 2; }; done
rc=0
for md in "$@"; do
  [ -f "$md" ] || { echo "render_md_to_pdf: no such file: $md" >&2; exit 2; }
  pdf="${md%.md}.pdf"
  if [ "$check" = 1 ]; then
    python3 "$here/check_pdf_coverage.py" --quiet "$md" || rc=1
    continue
  fi
  title="$(sed -n 's/^# \(.*\)$/\1/p' "$md" | head -n1)"
  [ -n "$title" ] || title="$(basename "$md" .md)"
  tmpd="$(mktemp -d "${TMPDIR:-/tmp}/render-md-pdf.XXXXXX")"
  trap 'rm -rf "${tmpd:?}"' EXIT
  html="$tmpd/page.html"
  pandoc -f gfm -t html5 -s --metadata "pagetitle=$title" --metadata "lang=en" --metadata document-css=false --metadata "keywords=src-sha256:$(sha256sum "$md" | cut -d" " -f1)" "$md" -o "$html"  # document-css=false: pandoc's own <style> (body max-width 36em, padding) outranks -s sheets in WeasyPrint
  css=(-s "$here/print.css"); chars=135
  # wide-table rule: any table separator row (|---|---|...) with >= 5 columns
  if grep -Eq '^\|?[ :-]*-[ :-]*(\|[ :-]*-[ :-]*){4,}\|?[ ]*$' "$md"; then css+=(-s "$here/print_landscape.css"); chars=200; fi
  python3 "$here/pdf_table_cols.py" "$html" "$chars"
  weasyprint -q "${css[@]}" "$html" "$tmpd/out.pdf"
  chmod 644 "$tmpd/out.pdf"
  mv -f "$tmpd/out.pdf" "$pdf"; echo "wrote $pdf"
  rm -rf "${tmpd:?}"; trap - EXIT
done
exit $rc
