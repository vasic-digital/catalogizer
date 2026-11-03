#!/usr/bin/env python3
"""check_html_viewport.py - gate: every tracked docs HTML page declares a viewport (constitution 11.4.190, 11.4.12).

Purpose   A doc page without <meta name="viewport"> is rendered at desktop width on phones. Pandoc body fragments (no <head>)
          were committed that way; scripts/docs/render_md_to_html.sh is the generator that must be used instead.
Scope     git-tracked *.html under docs/ (a directory walk of docs/ when ROOT is not a git checkout).
Usage     check_html_viewport.py [--root DIR]
Controls  Before the scan, through the same matcher: a synthetic page WITH the meta must match (positive), one without must not
          (negative), and the scope must be non-empty. Any control failure = BLIND (exit 2).
Exit      0 all pages declare a viewport, 1 one or more do not (printed as MISSING <path>), 2 BLIND / usage.
"""
import os
import re
import subprocess
import sys

META = re.compile(r'<meta\s[^>]*name\s*=\s*["\']viewport["\'][^>]*>', re.I)


def has_viewport(text):
    return bool(META.search(text))


def tracked(root):
    try:
        out = subprocess.run(["git", "-C", root, "ls-files", "-z", "--", "docs"], capture_output=True, check=True).stdout
        return [p for p in out.decode().split("\0") if p.endswith(".html")]
    except (OSError, subprocess.CalledProcessError):
        found = []
        for d, _, fs in os.walk(os.path.join(root, "docs")):
            found += [os.path.relpath(os.path.join(d, f), root) for f in fs if f.endswith(".html")]
        return sorted(found)


def main(argv):
    root = "."
    if argv[:1] == ["--root"] and len(argv) == 2:
        root = argv[1]
    elif argv:
        print(__doc__, file=sys.stderr)
        return 2
    if not has_viewport('<head><meta name="viewport" content="width=device-width"></head>') or has_viewport("<head></head><p>x</p>"):
        print("BLIND: matcher controls failed", file=sys.stderr)
        return 2
    files = tracked(root)
    if not files:
        print("BLIND: no docs/*.html in scope", file=sys.stderr)
        return 2
    bad = [p for p in files if not has_viewport(open(os.path.join(root, p), errors="ignore").read())]
    for p in bad:
        print("MISSING", p)
    print("check_html_viewport: %d pages, %d without viewport" % (len(files), len(bad)))
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
