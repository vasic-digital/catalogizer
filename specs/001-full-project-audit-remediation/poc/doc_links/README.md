# doc_links - documentation link crawler (read-only)

`crawl_links.py` starts at `README.md`, follows relative Markdown links transitively and prints JSON: reachable
set, orphans (all in-scope and the `docs/**` subset), broken links (with reason and case hint) and broken anchors.
Python 3 standard library only.

```
python3 crawl_links.py --root . > results/run1.json                  # same scope as the plan's first prototype
python3 crawl_links.py --root . --site-root Website > results/run2.json   # treat Website/ as a VitePress tree
python3 crawl_links.py --self-test                                   # 31 deterministic checks on a temp tree
```

Rules R1-R10 are in the module docstring (scope exclusions, code fences incl. `~~~` and longer closers, inline code,
reference definitions, autolinks, HTML href/src, percent-decoding, root-relative links, directory links to
README.md/index.md, GitHub-style heading anchors with `-1` suffixes, case hints, optional static-site root).

Limits: setext headings are not slugged; Markdown inside HTML blocks is not parsed; generator-specific URL rewrites
are not modelled (only the optional `--site-root` clean-URL rule); an unclosed fence swallows the rest of that file
(reported under `notes.fence_unclosed`).

Self-test covers: golden-good reachability (encoded path, directory link, reference definition, HTML href, link after a
fence), golden-bad orphan, and negative controls (link only inside ``` / `~~~~` fences or inline code creates neither an edge
nor a broken link; a shorter `~~~` line does not close a `~~~~` fence; an orphan's outlinks do not make its targets reachable).

Results: `results/run1.*` (default), `results/run2.*` (`--site-root Website`), `results/selftest.txt`.
