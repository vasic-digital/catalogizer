# check_md_links.py

| Field | Value |
|---|---|
| Script | `scripts/docs/check_md_links.py` |
| Purpose | Gate: every relative link and `#anchor` in the tracked markdown docs resolves (file exists; anchor matches a heading) |
| Scope | git-tracked `*.md` under `docs/`, plus `README.md`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` and `catalog-*/README*.md` |
| Usage | `check_md_links.py [--root DIR] [--alt-root DIR] [--list-scope]` |
| Parsing | Fenced code and inline code spans are ignored (a carrier such as `` `[x](y)` `` is not a link); external schemes (`http:`, `mailto:`) are skipped; anchors use GitHub slugs, explicit `<a id/name>` targets and pandoc `{#id}` heading ids also count |
| Controls | Before the scan a planted broken link in a temp fixture must be found and a good link must not (else BLIND, exit 2); an empty scope is BLIND |
| Output | `BROKEN<TAB>file<TAB>line<TAB>target<TAB>reason` per problem and a summary line |
| Exit | 0 no broken link, 1 one or more, 2 BLIND or usage |
| `--alt-root` | A live checkout used only for links into submodule paths that are empty in a plain clone |
| Fix | Repoint the link to the real document/heading; when the target was never written, unlink it ("not written yet") and list it in `docs/FOLLOW_UPS_NEXT_PHASE.md`. Headings that start with an emoji need an explicit `<a id="slug"></a>` line above them because GitHub drops the emoji and leaves a leading dash |
| Tests | `scripts/docs/tests/test_check_md_links.py` (includes a mutation: `CHECK_MD_LINKS_NO_EXISTENCE=1` must make the broken-link test see no breakage) |

## Wiring
Run by `scripts/pre-flight-check.sh` next to the docs HTML check (local validation, no CI/CD). Run it in a checkout with submodules initialised, or pass `--alt-root`.
