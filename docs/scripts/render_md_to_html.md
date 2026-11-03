# render_md_to_html.sh

| Field | Value |
|---|---|
| Script | `scripts/docs/render_md_to_html.sh` |
| Purpose | Regenerate the standalone `.html` twin of Markdown docs as a complete responsive page (doctype, `<head>`, charset, `<meta name="viewport">`) |
| Why | A bare `pandoc` body fragment has no viewport meta; phones render it at 980px (constitution 11.4.190, 11.4.12) |
| Usage | `render_md_to_html.sh [--check] FILE.md [FILE.md ...]` |
| `--check` | Writes nothing; exit 1 (`DRIFT <file>`) if an existing `.html` differs from what would be generated |
| Needs | `pandoc` 3 or newer |
| Exit | 0 ok, 1 drift or a generated page without viewport meta, 2 usage or missing tool/input |
| Tests | `scripts/docs/tests/test_check_html_viewport.py` |

The page title is the first `# ` heading of the Markdown file. Companion gate: `scripts/docs/check_html_viewport.py` (see `check_html_viewport.md`).
