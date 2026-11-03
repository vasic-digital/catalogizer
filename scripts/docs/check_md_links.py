#!/usr/bin/env python3
"""check_md_links.py - gate: every relative link and #anchor in the tracked markdown docs resolves.

Purpose   Broken relative links (renamed/never-written documents, stale heading anchors) make the docs a dead end for readers.
Scope     git-tracked *.md under docs/ plus README.md, CLAUDE.md, AGENTS.md, GEMINI.md and catalog-*/README*.md (a directory
          walk of the same names when ROOT is not a git checkout).
Usage     check_md_links.py [--root DIR] [--alt-root DIR] [--list-scope]
          --alt-root  a second checkout used ONLY for link targets under a submodule path (submodules/, constitution/) that
                      is an empty directory in ROOT (a plain clone does not initialise submodules).
Parsing   Fenced code (``` and ~~~) and inline code spans are removed first, so a link carrier such as `[x](y)` is ignored.
          Inline links/images and reference definitions are checked; external schemes (http:, mailto:, ...) and absolute
          URLs are skipped. #anchors use GitHub slugs (lowercase, punctuation dropped, spaces to '-', duplicates -1, -2),
          plus explicit <a name=/id=> targets and pandoc `{#id}` heading ids. Anchors into non-markdown targets are not checked.
Controls  Before the scan, through the same code: a planted broken link in a temp fixture MUST be found and a planted good
          link MUST NOT (else BLIND, exit 2); an empty scope is BLIND too. A zero is never trusted without them.
Mutation  CHECK_MD_LINKS_NO_EXISTENCE=1 disables the existence check (used only by the test to prove the check is load-bearing).
Output    BROKEN<TAB>file<TAB>line<TAB>target<TAB>reason  per problem, and a summary line.
Exit      0 no broken link, 1 one or more, 2 BLIND / usage.
"""
import os
import re
import subprocess
import sys
import tempfile
from urllib.parse import unquote

SCOPE_NAMES = {"README.md", "CLAUDE.md", "AGENTS.md", "GEMINI.md"}
FENCE = re.compile(r"^\s{0,3}(`{3,}|~{3,})")
CODE_SPAN = re.compile(r"(`+)(.+?)\1", re.S)
INLINE = re.compile(r"!?\[(?:[^\[\]]|\[[^\]]*\])*\]\(\s*(<[^>]*>|[^()\s]*(?:\([^()\s]*\)[^()\s]*)*)(?:\s+(?:\"[^\"]*\"|'[^']*'|\([^)]*\)))?\s*\)")
REFDEF = re.compile(r"^\s{0,3}\[[^\]]+\]:\s*(<[^>]*>|\S+)")
SCHEME = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*:")
HEADING_ATX = re.compile(r"^\s{0,3}#{1,6}\s+(.*?)\s*#*\s*$")
HTML_ANCHOR = re.compile(r"""<a\s[^>]*?(?:name|id)\s*=\s*["']([^"']+)["']""", re.I)
PANDOC_ID = re.compile(r"\s*\{#([^\s}]+)[^}]*\}\s*$")
HTML_ID = re.compile(r"""\sid\s*=\s*["']([^"']+)["']""", re.I)


def in_scope(path):
    if not path.endswith(".md"):
        return False
    base = os.path.basename(path)
    if path.startswith("docs/"):
        return True
    if "/" not in path and base in SCOPE_NAMES:
        return True
    top = path.split("/", 1)[0]
    return top.startswith("catalog-") and base.startswith("README")


def scope_files(root):
    try:
        out = subprocess.run(["git", "-C", root, "ls-files", "-z"], capture_output=True, check=True).stdout.decode("utf-8", "replace")
        files = [f for f in out.split("\0") if f]
    except (subprocess.CalledProcessError, FileNotFoundError, OSError):
        files = []
        for d, dn, fn in os.walk(root):
            dn[:] = [x for x in dn if x != ".git"]
            for f in fn:
                files.append(os.path.relpath(os.path.join(d, f), root))
    return sorted(f for f in files if in_scope(f) and os.path.isfile(os.path.join(root, f)))


def strip_code(text):
    """Return list of (lineno, line) with fenced blocks blanked; inline code spans blanked in the lines kept."""
    out, fence = [], None
    for n, line in enumerate(text.split("\n"), 1):
        m = FENCE.match(line)
        if fence:
            if m and m.group(1)[0] == fence[0] and len(m.group(1)) >= len(fence) and not line.strip().strip(fence[0]):
                fence = None
            out.append((n, ""))
            continue
        if m:
            fence = m.group(1)
            out.append((n, ""))
            continue
        out.append((n, line))
    # inline spans can cross lines inside a paragraph; blank per paragraph block
    res, buf = [], []
    def flush():
        if not buf:
            return
        joined = "\n".join(l for _, l in buf)
        joined = CODE_SPAN.sub(lambda mm: re.sub(r"[^\n]", " ", mm.group(0)), joined)
        for (n, _), l in zip(buf, joined.split("\n")):
            res.append((n, l))
        buf.clear()
    for n, l in out:
        if l.strip() == "":
            flush()
            res.append((n, l))
        else:
            buf.append((n, l))
    flush()
    return sorted(res)


def slug(h):
    h = re.sub(r"!?\[([^\]]*)\]\([^)]*\)", r"\1", h)   # links -> text
    h = re.sub(r"<[^>]+>", "", h)
    h = h.replace("`", "").strip().lower()
    h = re.sub(r"[^\w\- ]", "", h, flags=re.U)
    return h.replace(" ", "-")


_anchor_cache = {}


def anchors_of(path):
    if path in _anchor_cache:
        return _anchor_cache[path]
    seen, anchors = {}, set()
    try:
        text = open(path, encoding="utf-8", errors="replace").read()
    except OSError:
        text = ""
    lines = strip_code(text)
    raw = text
    for a in HTML_ANCHOR.findall(raw) + HTML_ID.findall(raw):
        anchors.add(a)
    prev = ""
    for _, l in lines:
        m = HEADING_ATX.match(l)
        h = None
        if m:
            h = m.group(1)
        elif prev.strip() and re.match(r"^\s{0,3}(=+|-{2,})\s*$", l) and not prev.lstrip().startswith(("-", "*", "|", ">")):
            h = prev.strip()
        if h:
            pm = PANDOC_ID.search(h)
            if pm:
                anchors.add(pm.group(1))
                h = PANDOC_ID.sub("", h).strip()
            s = slug(h)
            k = seen.get(s, 0)
            anchors.add(s if k == 0 else "%s-%d" % (s, k))
            seen[s] = k + 1
        prev = l
    _anchor_cache[path] = anchors
    return anchors


def extract(text):
    for n, line in strip_code(text):
        for m in INLINE.finditer(line):
            yield n, m.group(1)
        m = REFDEF.match(line)
        if m:
            yield n, m.group(1)


def submodule_paths(root):
    paths = set()
    try:
        for l in open(os.path.join(root, ".gitmodules"), encoding="utf-8", errors="replace"):
            m = re.match(r"\s*path\s*=\s*(\S+)", l)
            if m:
                paths.add(m.group(1).rstrip("/"))
    except OSError:
        pass
    return paths


def check_file(root, alt, rel, subs, existence=True):
    problems = []
    full = os.path.join(root, rel)
    text = open(full, encoding="utf-8", errors="replace").read()
    for n, raw in extract(text):
        t = raw.strip()
        if t.startswith("<") and t.endswith(">"):
            t = t[1:-1]
        if not t or SCHEME.match(t) or t.startswith("//"):
            continue
        path_part, _, frag = t.partition("#")
        path_part = path_part.split("?", 1)[0]
        path_part, frag = unquote(path_part), unquote(frag)
        if path_part == "":
            target = full
        elif path_part.startswith("/"):
            target = os.path.normpath(os.path.join(root, path_part.lstrip("/")))
        else:
            target = os.path.normpath(os.path.join(os.path.dirname(full), path_part))
        relt = os.path.relpath(target, root)
        if relt.startswith(".."):
            problems.append((rel, n, t, "escapes repository"))
            continue
        ok = os.path.exists(target)
        if not ok and alt:
            top = [s for s in subs if relt == s or relt.startswith(s + "/")]
            if top and os.path.exists(os.path.join(alt, relt)):
                ok = True
                target = os.path.join(alt, relt)
        if not ok:
            if existence:
                reason = "target does not exist"
                if any(relt == s or relt.startswith(s + "/") for s in subs) and not alt:
                    reason += " (inside a submodule path; pass --alt-root to verify against a live checkout)"
                problems.append((rel, n, t, reason))
            continue
        if frag and target.endswith(".md") and os.path.isfile(target):
            if frag not in anchors_of(target) and frag.lower() not in {a.lower() for a in anchors_of(target)}:
                if existence:
                    problems.append((rel, n, t, "anchor not found in %s" % os.path.relpath(target, root if target.startswith(root) else alt)))
    return problems


def scan(root, alt=None, files=None, existence=True):
    _anchor_cache.clear()
    files = scope_files(root) if files is None else files
    subs = submodule_paths(root)
    probs = []
    for f in files:
        probs.extend(check_file(root, alt, f, subs, existence))
    return files, probs


def controls():
    with tempfile.TemporaryDirectory() as d:
        os.makedirs(os.path.join(d, "docs"))
        open(os.path.join(d, "docs", "g.md"), "w").write("# G\n")
        open(os.path.join(d, "docs", "p.md"), "w").write("# P\n\n[good](g.md#g) [bad](planted_missing.md) [a](#nope)\n")
        files, probs = scan(d)
    got = sorted(p[2] for p in probs)
    return got == ["#nope", "planted_missing.md"]


def main(argv):
    root, alt, list_scope = ".", None, False
    i = 0
    while i < len(argv):
        a = argv[i]
        if a == "--root" and i + 1 < len(argv):
            root = argv[i + 1]; i += 2
        elif a == "--alt-root" and i + 1 < len(argv):
            alt = argv[i + 1]; i += 2
        elif a == "--list-scope":
            list_scope = True; i += 1
        else:
            print(__doc__, file=sys.stderr)
            return 2
    root = os.path.abspath(root)
    if not controls():
        print("BLIND: control fixture did not find the planted broken link", file=sys.stderr)
        return 2
    files = scope_files(root)
    if list_scope:
        print("\n".join(files))
        return 0
    if not files:
        print("BLIND: empty scope under %s" % root, file=sys.stderr)
        return 2
    existence = os.environ.get("CHECK_MD_LINKS_NO_EXISTENCE") != "1"
    files, probs = scan(root, os.path.abspath(alt) if alt else None, files, existence)
    for f, n, t, why in probs:
        print("BROKEN\t%s\t%d\t%s\t%s" % (f, n, t, why))
    print("checked %d files, %d broken links" % (len(files), len(probs)))
    return 1 if probs else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
