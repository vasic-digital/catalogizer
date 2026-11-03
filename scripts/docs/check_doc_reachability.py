#!/usr/bin/env python3
"""check_doc_reachability.py - deterministic local gate: every in-scope Markdown doc is reachable from README.md.

Purpose   Constitution 11.4.212: the root README.md is the entry point of all project documentation. This gate walks Markdown
          links starting at README.md and reports which in-scope .md files are not reached.
Scope     tracked *.md under docs/ and catalogizer-androidtv/docs/ (git ls-files; a plain directory walk when ROOT has no .git).
Usage     check_doc_reachability.py [--root DIR] [--baseline FILE] [--baseline-ref REF] [--tracker-root REL]... [--list]
                                    [--strict] [--update-baseline]
Links     Counted: inline [t](target), reference-style [t][ref] / [ref][] / [ref] with a matching "[ref]: target" definition,
          and <a href="target">. NOT counted: anything inside HTML comments, inline code spans, fenced blocks (``` or ~~~) and
          indented code blocks (4 spaces or a tab after a blank line, outside lists). http(s)/mailto/tel/#anchor are ignored.
          A directory link resolves to its README.md; a link to a --tracker-root directory (default docs/issues) marks EVERY
          .md below it reachable. The summary line prints how many in-scope files are reachable only through a tracker root.
Ratchet   The baseline file lists tolerated orphan paths, one per line ('#' lines are comments). An orphan not in the baseline
          FAILs. The baseline may only SHRINK: when ROOT is a git checkout the working baseline is compared with the baseline
          at --baseline-ref (default HEAD) and any added entry FAILs (a file new at that ref is accepted as the initial
          snapshot). --update-baseline rewrites the baseline (keeping its comment header) only when no current orphan is
          missing from it; it never grows it. --strict ignores the baseline (any orphan fails).
Entry     If README.md does not reach docs/README.md the gate reports ENTRY-LINK-MISSING on stderr and treats docs/README.md as
          an extra root. --strict makes that a failure.
Controls  Run before the real scan, through the same parser and walk: (positive) a synthetic README that links a synthetic doc
          must reach it, and the parser must find a real link and ignore links hidden in a comment, code span, fence and
          indented block; (negative) an unlinked synthetic doc must NOT be reached. Plus: scope non-empty, README.md present and
          yielding >=1 link. Any failure = BLIND (exit 2).
Exit      0 pass, 1 orphan(s) beyond baseline / baseline grown / strict, 2 BLIND / usage.
"""
import os, re, subprocess, sys

SCOPE_PREFIXES = ("docs/", "catalogizer-androidtv/docs/")
INLINE = re.compile(r'(?<!!)\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)')
REFUSE = re.compile(r'(?<!!)\[([^\]]+)\]\[([^\]]*)\]')
REFSHORT = re.compile(r'(?<![\]!])\[([^\]]+)\](?![\[(:])')
REFDEF = re.compile(r'^ {0,3}\[([^\]]+)\]:\s*<?(\S+?)>?(?:\s+.*)?$')
AHREF = re.compile(r'<a\s[^>]*?href\s*=\s*["\']([^"\']+)["\']', re.I)
LISTITEM = re.compile(r'^\s*([-*+]|\d+[.)])\s')
EXTERNAL = re.compile(r'(https?:|mailto:|tel:|#|data:|javascript:)', re.I)


def die(msg, code=2):
    print("check_doc_reachability: " + msg, file=sys.stderr)
    sys.exit(code)


def parse_links(text, base_dir):
    """Return the set of normalised repo-relative link targets visible in a Markdown text."""
    text = re.sub(r"<!--.*?-->", "", text, flags=re.S)
    visible = []
    fence = None
    prev_blank = True
    in_list = False
    in_indent = False
    for line in text.split("\n"):
        s = line.strip()
        m = re.match(r"^ {0,3}(`{3,}|~{3,})", line)
        if fence:
            if m and m.group(1)[0] == fence[0] and len(m.group(1)) >= len(fence) and s.strip(fence[0]) == "":
                fence = None
            continue
        if m:
            fence = m.group(1)
            prev_blank = False
            continue
        if not s:
            prev_blank = True
            in_indent = in_indent  # an indented block may continue across a blank line
            continue
        indented = line.startswith("\t") or line.startswith("    ")
        if indented and not in_list and (prev_blank or in_indent):
            in_indent = True
            prev_blank = False
            continue
        in_indent = False
        if LISTITEM.match(line):
            in_list = True
        elif not indented:
            in_list = False
        prev_blank = False
        visible.append(line)
    body = "\n".join(visible)
    body = re.sub(r"(`+)(?:(?!\1).)+?\1", "", body)  # inline code spans (single line)
    defs = {}
    kept = []
    for line in body.split("\n"):
        d = REFDEF.match(line)
        if d:
            defs[d.group(1).strip().lower()] = d.group(2)
        else:
            kept.append(line)
    body = "\n".join(kept)
    raw = set(INLINE.findall(body)) | set(AHREF.findall(body))
    for t, r in REFUSE.findall(body):
        k = (r or t).strip().lower()
        if k in defs:
            raw.add(defs[k])
    for t in REFSHORT.findall(body):
        k = t.strip().lower()
        if k in defs:
            raw.add(defs[k])
    res = set()
    for u in raw:
        if EXTERNAL.match(u):
            continue
        u = u.split("#")[0].split("?")[0]
        if not u:
            continue
        u = u.replace("%20", " ")
        res.add(os.path.normpath(os.path.join(base_dir, u)) if not u.startswith("/") else u.lstrip("/"))
    return res


def walk(roots, outs, fileset, dirs, scope, trackers):
    scope_set = set(scope)
    seen = set(roots); q = list(roots); via_tracker = set()
    while q:
        c = q.pop()
        for p in outs(c):
            tgt = []
            if p in dirs:
                if any(p == t or p.startswith(t + "/") for t in trackers):
                    for f in scope:
                        if f.startswith(p + "/") and f not in seen:
                            seen.add(f); via_tracker.add(f)
                    continue
                r = p + "/README.md"
                if r in fileset: tgt.append(r)
            elif p in fileset and p.endswith(".md"):
                tgt.append(p)
            for t in tgt:
                if t not in seen:
                    seen.add(t); q.append(t)
    return seen, via_tracker & scope_set


def controls():
    """Non-tautological controls: exercise the real parser and the real walk on synthetic input."""
    doc = ("# T\n[real](docs/real.md)\n<!-- [hidden](docs/c1.md) -->\n`[code](docs/c2.md)`\n```\n[f](docs/c3.md)\n```\n"
           "~~~\n[f](docs/c4.md)\n~~~\n\n    [ind](docs/c5.md)\n\n[ref use][r1]\n<a href=\"docs/anchor.md\">x</a>\n\n[r1]: docs/refdef.md\n")
    got = parse_links(doc, "")
    want = {"docs/real.md", "docs/refdef.md", "docs/anchor.md"}
    if got != want:
        die("BLIND parser control failed: got %s want %s" % (sorted(got), sorted(want)))
    fs = {"README.md", "docs/linked.md", "docs/unlinked.md"}
    graph = {"README.md": {"docs/linked.md"}, "docs/linked.md": set(), "docs/unlinked.md": set()}
    seen, _ = walk(["README.md"], lambda f: graph.get(f, set()), fs, {"docs"}, ["docs/linked.md", "docs/unlinked.md"], ["docs/issues"])
    if "docs/linked.md" not in seen:
        die("BLIND positive walk control failed: linked synthetic doc not reached")
    if "docs/unlinked.md" in seen:
        die("BLIND negative walk control failed: unlinked synthetic doc reached")


def list_files(root):
    if os.path.exists(os.path.join(root, ".git")):
        out = subprocess.run(["git", "-C", root, "ls-files"], capture_output=True, text=True)
        if out.returncode != 0:
            die("BLIND git ls-files failed")
        return [f for f in out.stdout.split("\n") if f], True
    res = []
    for d, _, fs in os.walk(root):
        for f in fs:
            res.append(os.path.relpath(os.path.join(d, f), root))
    return sorted(res), False


def read_entries(text):
    return set(l.strip() for l in text.split("\n") if l.strip() and not l.startswith("#"))


def main(argv):
    root = None; baseline = None; bref = "HEAD"; trackers = []; lst = False; strict = False; upd = False
    i = 0
    while i < len(argv):
        a = argv[i]
        if a in ("--root", "--baseline", "--tracker-root", "--baseline-ref"):
            if i + 1 >= len(argv):
                die("usage: " + a + " needs a value")
            v = argv[i + 1]; i += 2
            if a == "--root": root = v
            elif a == "--baseline": baseline = v
            elif a == "--baseline-ref": bref = v
            else: trackers.append(v.rstrip("/"))
        elif a == "--list": lst = True; i += 1
        elif a == "--strict": strict = True; i += 1
        elif a == "--update-baseline": upd = True; i += 1
        else: die("usage: unknown argument " + a)
    if root is None:
        root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    if not trackers:
        trackers = ["docs/issues"]
    controls()
    files, is_git = list_files(root)
    fileset = set(files)
    scope = sorted(f for f in files if f.endswith(".md") and f.startswith(SCOPE_PREFIXES))
    if not scope:
        die("BLIND empty scope (no .md under docs/ or catalogizer-androidtv/docs/)")
    if "README.md" not in fileset:
        die("BLIND README.md not found at root")
    dirs = set()
    for f in files:
        p = os.path.dirname(f)
        while p:
            dirs.add(p); p = os.path.dirname(p)
    cache = {}

    def outs(f):
        if f not in cache:
            try:
                txt = open(os.path.join(root, f), errors="ignore").read()
            except OSError:
                txt = ""
            cache[f] = parse_links(txt, os.path.dirname(f))
        return cache[f]

    if not outs("README.md"):
        die("BLIND README.md yields no links (parser or file broken)")
    seen, via_tracker = walk(["README.md"], outs, fileset, dirs, scope, trackers)
    entry_ok = "docs/README.md" in seen
    if not entry_ok:
        print("check_doc_reachability: ENTRY-LINK-MISSING README.md does not reach docs/README.md (using it as an extra root)", file=sys.stderr)
        if "docs/README.md" in fileset:
            seen, via_tracker = walk(["README.md", "docs/README.md"], outs, fileset, dirs, scope, trackers)
    if len(seen) < 2:
        die("BLIND real walk reached nothing beyond README.md")
    orph = [f for f in scope if f not in seen]
    base = set(); grown = []
    bheader = ""
    if baseline:
        if os.path.exists(baseline):
            btxt = open(baseline).read()
            base = read_entries(btxt)
            bheader = "".join(l for l in btxt.splitlines(True) if l.startswith("#"))
        elif not upd:
            die("BLIND baseline file not found: " + baseline)
        rel = os.path.relpath(os.path.abspath(baseline), os.path.abspath(root))
        if is_git and not rel.startswith(".."):
            r = subprocess.run(["git", "-C", root, "show", "%s:%s" % (bref, rel)], capture_output=True, text=True)
            if r.returncode == 0:
                grown = sorted(base - read_entries(r.stdout))
    new = [f for f in orph if f not in base]
    gone = sorted(base - set(orph))
    print("scope=%d reachable=%d orphans=%d reachable_only_via_tracker_root=%d (%.0f%%) baseline=%d new_orphans=%d cleared_from_baseline=%d baseline_grown=%d entry_link=%s"
          % (len(scope), len(scope) - len(orph), len(orph), len(via_tracker), 100.0 * len(via_tracker) / len(scope),
             len(base), len(new), len(gone), len(grown), "ok" if entry_ok else "MISSING"))
    if lst:
        for f in orph: print(("NEW " if f in new else "OLD ") + f)
    if upd:
        if not baseline:
            die("usage: --update-baseline needs --baseline")
        if new and os.path.exists(baseline):
            die("refusing to update baseline: %d orphan(s) are not in it (it never grows)" % len(new), 1)
        if new and not os.path.exists(baseline):
            pass  # initial snapshot of a new baseline file
        hdr = bheader or "# doc reachability ratchet baseline: tolerated orphan paths, one per line. Entries may only be removed.\n"
        open(baseline, "w").write(hdr + "".join(f + "\n" for f in orph))
        print("baseline written: %d entries" % len(orph))
        sys.exit(0)
    bad = False
    if grown:
        for f in grown[:50]: print("BASELINE-GROWN " + f, file=sys.stderr)
        bad = True
    if strict:
        bad = bad or len(orph) > 0 or not entry_ok
    else:
        bad = bad or len(new) > 0
    if bad:
        for f in (orph if strict else new)[:50]:
            print("UNREACHABLE " + f, file=sys.stderr)
        sys.exit(1)
    sys.exit(0)


if __name__ == "__main__":
    main(sys.argv[1:])
