# IMG-DOCS

Documentation render and check tools (SC-007): pandoc, weasyprint (PDF engine), chromium, Mermaid CLI 12.0.0, VitePress 1.6.4, fonts, python3 with pytest, PyYAML and jsonschema, bash, git and jq. The `docs docs` check commands (link crawl, header and fingerprint checks) are interpreter-class; the renderers (pandoc, the PDF engine, the diagram renderers) are compile-class and remote (T121b rule: classed by command, the lock default is `compile`).
Honest gaps: the two npm tools are version-exact only (no committed package-lock for the global installs); the DOCX writer is pandoc's; `Website/` being the VitePress site is UNCONFIRMED (owned by document 13); Mermaid CLI needs `--no-sandbox` through a puppeteer config when run as root (not baked in).
Smoke test: `python3 -c 'import pytest, yaml, jsonschema'`, `pandoc --version`, `mmdc --version`.
