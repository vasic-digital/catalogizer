#!/usr/bin/env python3
"""crawl_links.py - read-only documentation link crawler (POC for spec 001, FR-013 / SC-006).

Starting at README.md it follows relative Markdown links transitively inside the repository and
prints ONE JSON document with: the reachable set, the orphans (all in-scope Markdown, and the
subset under docs/**), broken links, and broken anchors.

Rules (each one is exercised by the self-test):
  R1  Scope = every *.md under --root except directories named submodules, Upstreams, node_modules,
      .git, vendor (same exclusion as the plan's first prototype, so numbers are comparable).
  R2  Fenced code blocks (``` or ~~~, any length >= 3, closed by the same char with >= length)
      and inline code spans (`...`) are removed BEFORE link extraction. An unclosed fence swallows the
      rest of the file (CommonMark behaviour) - reported as fence_unclosed.
  R3  Link forms parsed: inline [t](url "title"), images ![a](url), reference definitions
      [id]: url, autolinks <relative/path.md>, and HTML href="..." / src="...".
  R4  Targets: skip scheme URLs (http:, https:, mailto:, tel:, data:, ftp:, javascript:); strip ?query;
      split #fragment; percent-decode the path; a leading "/" means repository root (GitHub semantics),
      otherwise relative to the source file's directory; ".." is normalised and a path that leaves the
      repository is broken ("escapes_root").
  R5  A directory link resolves to README.md, then index.md inside it; a directory with neither is
      "dir_without_index" (broken for crawling purposes, exists on disk).
  R6  Only in-scope *.md targets are graph nodes. Other existing files (pdf, png, source files) are
      checked for existence only. Existing Markdown outside scope (e.g. inside submodules/) is counted
      as out_of_scope_ok and is not followed.
  R7  #fragment on a Markdown target is checked against that file's GitHub-style heading slugs
      (lowercase, punctuation removed, spaces -> "-", duplicates get -1, -2, ...) and explicit
      id=/name= attributes. A missing fragment is a broken_anchor (the file itself is still reachable).
  R8  Same-file anchors ("#x") are checked against the source file's own headings.
  R10 Optional --site-root DIR (repeatable): files under DIR are treated as a static-site tree
      (VitePress): "/x" resolves against DIR and extensionless links try x.md and x/index.md. Off by
      default (without it the 42 Website/ links of that style are reported broken).
  R9  Case: a broken target that exists with different letter case is flagged case_hint (Linux FS is
      case sensitive; case-insensitive hosts would hide it).

Limits (documented false positives/negatives): setext headings are not slugged; headings produced by
HTML are not seen; Markdown inside HTML blocks other than href/src attributes is not parsed; links
built by a static-site generator (VitePress base, router rewrites) are not modelled; code fences
inside list items indented > 3 spaces are treated as fences by this scanner.

Usage: crawl_links.py [--root DIR] [--start README.md] [--docs-prefix docs/] [--no-lists] [--self-test]
Exit: 0 report produced (always, so the report can be consumed); with --fail-on-orphans-or-broken
      exit 1 when any docs/** orphan, broken link or broken anchor exists.
"""
import argparse
import json
import os
import re
import sys
import tempfile
import unicodedata
import urllib.parse
from collections import Counter, deque

SKIP_DIRS = {"submodules", "Upstreams", "node_modules", ".git", "vendor"}
SCHEMES = ("http:", "https:", "mailto:", "tel:", "data:", "ftp:", "javascript:", "ssh:", "git:")

INLINE = re.compile(r'!?\[(?:[^\[\]]|\[[^\]]*\])*\]\(\s*(<[^>]*>|[^)\s]*(?:\([^)\s]*\)[^)\s]*)*)(?:\s+(?:"[^"]*"|\'[^\']*\'))?\s*\)')
REFDEF = re.compile(r'^[ ]{0,3}\[[^\]]+\]:\s*(<[^>]*>|\S+)', re.M)
AUTOLINK = re.compile(r'<([^<>\s:]+\.[A-Za-z0-9]+(?:#[^<>\s]*)?)>')
HTMLREF = re.compile(r'''\b(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)')''', re.I)
HEADING = re.compile(r'^[ ]{0,3}(#{1,6})[ \t]+(.+?)[ \t]*#*[ \t]*$')
HTMLID = re.compile(r'''\b(?:id|name)\s*=\s*(?:"([^"]+)"|'([^']+)')''', re.I)
FENCE_OPEN = re.compile(r'^[ ]{0,3}(`{3,}|~{3,})')


def strip_code(text):
    """Return (text_without_fenced_blocks_and_inline_code, headings_text, unclosed_fence_flag)."""
    out, heads = [], []
    fence = None  # (char, length)
    unclosed = False
    for line in text.split("\n"):
        if fence:
            m = re.match(r'^[ ]{0,3}(%s{%d,})[ \t]*$' % (re.escape(fence[0]), fence[1]), line)
            if m:
                fence = None
            out.append("")  # keep line numbers stable
            continue
        m = FENCE_OPEN.match(line)
        if m:
            marker = m.group(1)
            # a backtick fence's info string must not contain a backtick (else it's inline code)
            if not (marker[0] == "`" and "`" in line[m.end():]):
                fence = (marker[0], len(marker))
                out.append("")
                continue
        out.append(line)
        heads.append(line)
    if fence:
        unclosed = True
    body = "\n".join(out)
    body = re.sub(r'(`+)(?:(?!\1).)+?\1', lambda m: " " * 0, body, flags=re.S)  # inline code spans
    return body, "\n".join(heads), unclosed


def slugify(h):
    h = re.sub(r'!?\[([^\]]*)\]\([^)]*\)', r'\1', h)       # links -> text
    h = re.sub(r'<[^>]+>', '', h)                           # html tags
    h = re.sub(r'[`*_~]', lambda m: m.group(0) if m.group(0) == "_" else "", h)
    h = unicodedata.normalize("NFKC", h).strip().lower()
    h = re.sub(r'[^\w\- ]', '', h, flags=re.U)
    return h.replace(" ", "-")


def anchors_of(heads_text, raw_text):
    seen, out = Counter(), set()
    for line in heads_text.split("\n"):
        m = HEADING.match(line)
        if m:
            s = slugify(m.group(2))
            n = seen[s]
            seen[s] += 1
            out.add(s if n == 0 else "%s-%d" % (s, n))
    for m in HTMLID.finditer(raw_text):
        out.add(m.group(1) or m.group(2))
    return out


def extract_targets(body):
    for m in INLINE.finditer(body):
        yield m.group(1).strip("<>")
    for m in REFDEF.finditer(body):
        yield m.group(1).strip("<>")
    for m in AUTOLINK.finditer(body):
        yield m.group(1)
    for m in HTMLREF.finditer(body):
        yield (m.group(1) if m.group(1) is not None else m.group(2))


def crawl(root, start="README.md", docs_prefix="docs/", site_roots=()):
    root = os.path.abspath(root)
    files = []
    for d, ds, fs in os.walk(root):
        ds[:] = sorted(x for x in ds if x not in SKIP_DIRS)
        for f in sorted(fs):
            if f.endswith(".md"):
                files.append(os.path.relpath(os.path.join(d, f), root).replace(os.sep, "/"))
    fset = set(files)
    lower_index = {}
    for f in files:
        lower_index.setdefault(f.lower(), f)
    edges, broken, broken_anchors, notes = {}, [], [], {"fence_unclosed": [], "out_of_scope_ok": 0,
                                                       "non_md_ok": 0, "external_skipped": 0}
    anchor_cache = {}
    texts = {}

    def read(f):
        if f not in texts:
            with open(os.path.join(root, f), encoding="utf-8", errors="replace") as fh:
                raw = fh.read()
            body, heads, unclosed = strip_code(raw)
            texts[f] = (raw, body, heads, unclosed)
        return texts[f]

    def anchors(f):
        if f not in anchor_cache:
            raw, body, heads, _ = read(f)
            anchor_cache[f] = anchors_of(heads, raw)
        return anchor_cache[f]

    for f in files:
        raw, body, heads, unclosed = read(f)
        if unclosed:
            notes["fence_unclosed"].append(f)
        outs = set()
        for tgt in extract_targets(body):
            tgt = tgt.strip()
            if not tgt:
                continue
            if tgt.lower().startswith(SCHEMES) or re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*:', tgt) and not re.match(r'^[a-zA-Z]:[\\/]', tgt):
                notes["external_skipped"] += 1
                continue
            path_part, _, frag = tgt.partition("#")
            path_part = path_part.split("?")[0]
            frag = urllib.parse.unquote(frag)
            if path_part == "":
                # same-file anchor
                if frag and frag.lower() not in {a.lower() for a in anchors(f)} and frag != "top":
                    broken_anchors.append({"from": f, "target": "#" + frag})
                continue
            path_part = urllib.parse.unquote(path_part)
            site = next((sr for sr in site_roots if f == sr or f.startswith(sr.rstrip("/") + "/")), None)
            if path_part.startswith("/"):
                p = os.path.normpath(((site.rstrip("/") + "/") if site else "") + path_part.lstrip("/"))
            else:
                p = os.path.normpath(os.path.join(os.path.dirname(f), path_part))
            p = p.replace(os.sep, "/")
            if site and not os.path.exists(os.path.join(root, p)):
                # static-site clean URLs (VitePress): /guide/x -> guide/x.md or guide/x/index.md
                for cand in (p + ".md", p + "/index.md"):
                    if cand in fset:
                        p = cand
                        break
            if p.startswith(".."):
                broken.append({"from": f, "target": tgt, "reason": "escapes_root"})
                continue
            full = os.path.join(root, p)
            if os.path.isdir(full):
                for idx in ("README.md", "index.md"):
                    if os.path.exists(os.path.join(full, idx)):
                        p = (p + "/" + idx) if p != "." else idx
                        break
                else:
                    broken.append({"from": f, "target": tgt, "reason": "dir_without_index"})
                    continue
            if p.endswith(".md") and p in fset:
                outs.add(p)
                if frag and frag.lower() not in {a.lower() for a in anchors(p)}:
                    broken_anchors.append({"from": f, "target": tgt, "file": p, "anchor": frag})
            elif os.path.exists(os.path.join(root, p)):
                top = p.split("/")[0]
                if p.endswith(".md") and top in SKIP_DIRS:
                    notes["out_of_scope_ok"] += 1
                else:
                    notes["non_md_ok"] += 1
            else:
                rec = {"from": f, "target": tgt, "reason": "missing"}
                hit = lower_index.get(p.lower())
                if hit:
                    rec["case_hint"] = hit
                broken.append(rec)
        edges[f] = sorted(outs)

    seen, depth, q = set(), {}, deque()
    if start in fset:
        seen.add(start); depth[start] = 0; q.append(start)
    while q:
        n = q.popleft()
        for m in edges.get(n, []):
            if m not in seen:
                seen.add(m); depth[m] = depth[n] + 1; q.append(m)
    orphans = sorted(fset - seen)
    docs_orphans = [o for o in orphans if o.startswith(docs_prefix)]
    docs_all = [f for f in files if f.startswith(docs_prefix)]
    groups = Counter("/".join(o.split("/")[:2]) if o.count("/") >= 2 else (o.split("/")[0] if "/" in o else "(root)") for o in orphans)
    bgroups = Counter(b["from"].split("/")[0] if "/" in b["from"] else b["from"] for b in broken)
    return {
        "start": start, "start_exists": start in fset,
        "in_scope": len(files), "reachable": len(seen), "orphans": len(orphans),
        "docs_prefix": docs_prefix, "docs_in_scope": len(docs_all),
        "docs_reachable": len([f for f in seen if f.startswith(docs_prefix)]),
        "docs_orphans": len(docs_orphans),
        "broken_links": len(broken), "broken_anchors": len(broken_anchors),
        "max_depth": max(depth.values()) if depth else 0,
        "notes": notes,
        "reachable_set": sorted(seen),
        "orphans_among_docs": docs_orphans,
        "orphans_all": orphans,
        "orphans_by_group": sorted(groups.items(), key=lambda kv: (-kv[1], kv[0]))[:30],
        "broken_by_source_group": sorted(bgroups.items(), key=lambda kv: (-kv[1], kv[0]))[:30],
        "broken_list": broken, "broken_anchor_list": broken_anchors,
        "depth": depth,
    }


# ------------------------------------------------------------------ self-test
def _w(root, rel, text):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as fh:
        fh.write(text)


def self_test():
    ok = bad = 0
    def check(name, cond, detail=""):
        nonlocal ok, bad
        if cond:
            ok += 1; print("  PASS  " + name)
        else:
            bad += 1; print("  FAIL  " + name + " " + detail)
    with tempfile.TemporaryDirectory() as t:
        _w(t, "README.md", """# Root
See [a](docs/a.md#second-section), [spaced](docs/my%20doc.md), [dir](docs/sub/), [self](#root), [bad anchor](docs/a.md#nope),
[gone](docs/nope.md), [pdf](docs/file.pdf), [ext](https://example.com/x.md), [up](../../outside.md), [case](docs/A.MD).
Inline code `[ghost](docs/ghost_inline.md)` must be ignored. <a href="docs/html_link.md">h</a>
```md
[fenced](docs/fenced_missing.md) and [fenced_exist](docs/only_in_fence.md)
```
~~~~
[tilde](docs/tilde_missing.md)
~~~
still inside (shorter closer does not close)
[tilde2](docs/tilde2_missing.md)
~~~~
[after](docs/after_fence.md)
[ref]: docs/refdef.md
""")
        _w(t, "docs/a.md", "# A\n\n## Second Section\n\n## Second Section\n[dup](#second-section-1)\n")
        _w(t, "docs/my doc.md", "# spaced\n")
        _w(t, "docs/sub/README.md", "# sub\n[to c](../c.md)\n")
        _w(t, "docs/c.md", "# c deep\n")
        _w(t, "docs/file.pdf", "x")
        _w(t, "docs/html_link.md", "# h\n")
        _w(t, "docs/after_fence.md", "# after\n")
        _w(t, "docs/refdef.md", "# ref\n")
        _w(t, "docs/orphan.md", "# orphan\n[to c](c.md)\n")
        _w(t, "docs/only_in_fence.md", "# linked only from a fence\n")
        _w(t, "docs/ghost_inline.md", "# linked only from inline code\n")
        _w(t, "docs/tilde2_missing.md", "# exists but only linked inside a tilde fence\n")
        _w(t, "docs/a2.md", "# unclosed\n```\n[x](docs/swallowed_missing.md)\n")
        _w(t, "submodules/x/README.md", "# not in scope\n")
        _w(t, "docs/A.md", "# case twin placeholder\n")
        os.chmod(os.path.join(t, "docs/file.pdf"), 0o644)
        r = crawl(t)
        reach = set(r["reachable_set"])
        # golden-good: everything legitimately linked is reachable
        for f in ("README.md", "docs/a.md", "docs/my doc.md", "docs/sub/README.md", "docs/c.md",
                  "docs/html_link.md", "docs/after_fence.md", "docs/refdef.md"):
            check("golden-good reachable " + f, f in reach)
        check("depth of docs/c.md is 2 (README > docs/sub/README > docs/c)", r["depth"].get("docs/c.md") == 2, str(r["depth"].get("docs/c.md")))
        # golden-bad: orphan detected
        check("golden-bad orphan.md listed as orphan", "docs/orphan.md" in r["orphans_among_docs"])
        check("golden-bad orphan inherits no reachability from its own outlinks", "docs/c.md" in reach and "docs/orphan.md" not in reach)
        # negative controls: code-fenced / inline-code links do NOT create edges or errors
        check("control: link only in ``` fence does not make target reachable", "docs/only_in_fence.md" in r["orphans_among_docs"])
        check("control: link only in inline code does not make target reachable", "docs/ghost_inline.md" in r["orphans_among_docs"])
        check("control: ~~~~ fence with shorter ~~~ line stays open", "docs/tilde2_missing.md" in r["orphans_among_docs"])
        check("control: link after a fence is parsed again", "docs/after_fence.md" in reach)
        targets = {b["target"] for b in r["broken_list"]}
        check("control: fenced missing link is NOT reported broken", not any("fenced_missing" in x or "tilde_missing" in x or "ghost_inline" in x or "swallowed" in x for x in targets), str(targets))
        check("broken: missing file reported", "docs/nope.md" in targets)
        check("broken: path escaping root reported", any(b["reason"] == "escapes_root" for b in r["broken_list"]))
        check("broken: case mismatch gets case_hint", any(b["target"] == "docs/A.MD" and b.get("case_hint") for b in r["broken_list"]), str(r["broken_list"]))
        check("ok: URL-encoded path resolved", "docs/my doc.md" in reach and "docs/my%20doc.md" not in targets)
        check("ok: existing non-md target not broken", "docs/file.pdf" not in targets)
        check("ok: external url skipped", not any(x.startswith("http") for x in targets) and r["notes"]["external_skipped"] >= 1)
        anch = {(b["from"], b["target"]) for b in r["broken_anchor_list"]}
        check("anchor: bad anchor reported", ("README.md", "docs/a.md#nope") in anch, str(anch))
        check("anchor: good cross-file anchor accepted", ("README.md", "docs/a.md#second-section") not in anch)
        check("anchor: duplicate-heading suffix -1 accepted (same-file)", ("docs/a.md", "#second-section-1") not in anch)
        check("anchor: same-file #root accepted", ("README.md", "#root") not in anch)
        check("unclosed fence recorded", "docs/a2.md" in r["notes"]["fence_unclosed"])
        check("submodules excluded from scope", not any(f.startswith("submodules/") for f in r["orphans_all"]))
        # site-root rule (VitePress style) on/off
        _w(t, "README.md", open(os.path.join(t, "README.md")).read() + "[site](Website/index.md)\n")
        _w(t, "Website/index.md", "# site\n[g](/guide/x#a) [h](other)\n")
        _w(t, "Website/guide/x.md", "# x\n## a\n")
        _w(t, "Website/other.md", "# other\n")
        off = crawl(t); on = crawl(t, site_roots=("Website",))
        check("site-root off: '/guide/x#a' reported broken", any(b["from"] == "Website/index.md" for b in off["broken_list"]))
        check("site-root on: '/guide/x#a' + extensionless 'other' reachable, anchor ok", "Website/guide/x.md" in on["reachable_set"] and "Website/other.md" in on["reachable_set"] and not any(b["from"] == "Website/index.md" for b in on["broken_list"] + on["broken_anchor_list"]), str(on["broken_list"]))
        # idempotent / deterministic
        r = crawl(t); r2 = crawl(t)
        r["depth"], r2["depth"] = sorted(r["depth"].items()), sorted(r2["depth"].items())
        check("deterministic: two runs identical", json.dumps(r, sort_keys=True) == json.dumps(r2, sort_keys=True))
    print("SELF-TEST: pass=%d fail=%d" % (ok, bad))
    return bad == 0


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--root", default=".")
    ap.add_argument("--start", default="README.md")
    ap.add_argument("--docs-prefix", default="docs/")
    ap.add_argument("--site-root", action="append", default=[], help="static-site root dir (repeatable), e.g. Website")
    ap.add_argument("--no-lists", action="store_true", help="omit reachable_set/orphans_all/depth (smaller output)")
    ap.add_argument("--fail-on-orphans-or-broken", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        sys.exit(0 if self_test() else 1)
    r = crawl(a.root, a.start, a.docs_prefix, tuple(a.site_root))
    if a.no_lists:
        for k in ("reachable_set", "orphans_all", "depth"):
            r.pop(k, None)
    else:
        r["depth"] = dict(sorted(r["depth"].items()))
    json.dump(r, sys.stdout, indent=1, ensure_ascii=False)
    sys.stdout.write("\n")
    if a.fail_on_orphans_or_broken and (r["docs_orphans"] or r["broken_links"] or r["broken_anchors"]):
        sys.exit(1)


if __name__ == "__main__":
    main()
