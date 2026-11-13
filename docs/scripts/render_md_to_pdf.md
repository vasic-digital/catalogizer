# render_md_to_pdf.sh and check_pdf_coverage.py

| Field | Value |
|---|---|
| Scripts | `scripts/docs/render_md_to_pdf.sh`, `scripts/docs/check_pdf_coverage.py`, `scripts/docs/print.css`, `scripts/docs/print_landscape.css` |
| Purpose | Render the `.pdf` twin of a Markdown doc without clipping wide tables or long tokens, and prove it by comparing the words of the Markdown with the text extracted from the PDF (constitution 11.4.65, 11.4.201) |
| Why | PDFs rendered from the pandoc HTML with default print rules were cut off at the right page edge (up to 29% of a doc's words missing); the file opened fine, so nothing noticed |
| Render | `render_md_to_pdf.sh FILE.md ...` writes `FILE.pdf` (pandoc gfm to HTML5, then WeasyPrint with `print.css`) |
| Orientation rule | A doc whose Markdown has a table separator row with 5 or more columns is rendered A4 landscape (`print_landscape.css`), all others A4 portrait |
| Print rules | Auto table layout at 100% width, 7.5pt table text, cell words break only when over-long (`overflow-wrap: break-word`), rows never split across pages, `pre` and `code` wrap |
| `--check` | `render_md_to_pdf.sh --check FILE.md` writes nothing and runs the coverage check on the existing PDF |
| Coverage check | `check_pdf_coverage.py [--root DIR] [--allow FILE] [--min-len N] [--quiet] FILE.md ...` |
| Method | Distinct tokens of 6 or more characters of the Markdown (link targets, HTML comments and fence markers removed, NFKC) against `pdftotext` output (soft hyphens removed, whitespace and hyphens deleted on both sides, two reading orders: default and `-raw`) |
| Allow-list | `scripts/docs/pdf_coverage_allow.tsv` (`path`, `token`, `reason`; the reason is required). Default: no file, nothing allowed |
| Control | Before the scan a token taken from the PDF must be found and a fabricated token must not be; otherwise BLIND |
| Exit | 0 no token missing; 1 tokens missing, PDF absent, empty or unreadable; 2 BLIND, usage or tool missing |
| Tests | `scripts/docs/tests/test_check_pdf_coverage.py` (mutations: no table CSS, truncated PDF, broken needle, deleted twin) |

Limit: this proves the words are present in the extracted text, not that the page looks right; a token split by a page break between two table cells can read as missing and is a reason to fix the layout, not to allow-list it. It measures 6+ character tokens only.

# check_twin_presence.py

| Field | Value |
|---|---|
| Script | `scripts/docs/check_twin_presence.py` |
| Purpose | Gate: every Markdown doc added under `docs/` since a ref has non-empty `.html` and `.pdf` twins, and the html matches `render_md_to_html.sh --check` (constitution 11.4.65) |
| Usage | `check_twin_presence.py [--root DIR] [--since REF] [--no-html-drift] [--quiet]`; `--since` defaults to the newest tag |
| Scope | Files added (`git diff --diff-filter=A REF..HEAD`, plus staged additions) under `docs/` ending in `.md` |
| Control | A doc with both twins must read complete and one missing its pdf incomplete; an empty scope is BLIND |
| Exit | 0 all present, 1 one or more `MISSING` rows, 2 BLIND or usage |
| Fix | `render_md_to_html.sh FILE.md` and `render_md_to_pdf.sh FILE.md`, never hand-edit generated twins |
