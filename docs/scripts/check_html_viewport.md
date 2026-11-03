# check_html_viewport.py

| Field | Value |
|---|---|
| Script | `scripts/docs/check_html_viewport.py` |
| Purpose | Gate: every git-tracked `docs/**/*.html` page declares `<meta name="viewport">` (constitution 11.4.190) |
| Usage | `check_html_viewport.py [--root DIR]` |
| Controls | Before the scan a synthetic page with the meta must match and one without must not; an empty scope is BLIND (exit 2), never a pass |
| Output | `MISSING <path>` per offending page and a summary line |
| Exit | 0 all pages declare a viewport, 1 one or more do not, 2 BLIND or usage |
| Fix | Regenerate the page with `scripts/docs/render_md_to_html.sh` (never hand-edit generated HTML) |
| Tests | `scripts/docs/tests/test_check_html_viewport.py` |

## Wiring
Run by `scripts/pre-flight-check.sh` (local validation, no CI/CD) together with `render_md_to_html.sh --check` for the four generated pages.
