#!/usr/bin/env python3
"""org_of.py - the single owner-organisation parser shared by derive_scope.sh and verify_repos.sh (WF-REVIEW I1, m9).

Purpose  Extract the organisation of a git remote URL the same way everywhere: the second-last path segment, lower-cased,
         tolerating a trailing slash and a trailing .git. A URL that cannot be classified (no organisation segment, a
         relative path starting ./ or ../, no slash at all) yields no organisation, never a guess.
Usage    org_of.py URL [URL ...]      prints one line per URL: the lower-cased organisation, or an empty line
         import org_of; org_of.org_of(url)   -> str or None
Exit     0 always for well-formed arguments; 2 when called with no URL.
Side effects  none (pure function, no file or network access).
"""
import re
import sys


def org_of(url):
    m = re.search(r"[:/]([^/:]+)/[^/]+?(?:\.git)?/?$", url or "")
    if not m or (url or "").startswith(("./", "../")) or "/" not in (url or "").replace("://", ""):
        return None
    return m.group(1).lower()


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("usage: org_of.py URL [URL ...]", file=sys.stderr)
        sys.exit(2)
    for u in sys.argv[1:]:
        print(org_of(u) or "")
