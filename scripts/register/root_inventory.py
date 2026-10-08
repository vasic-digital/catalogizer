#!/usr/bin/env python3
"""root_inventory.py - T174 (WP-20; docs/21 section 12.4 and the WP-20 output row; docs/01 section 3.8): the root-item inventory with dispositions.

Enumerates EVERY root entry of the frozen commit and gives each exactly one disposition from the closed set
  problem_source (with its source class) | component (an application or module id) | documentation (class assigned by WP-37, T282) | excluded (with a reason)
into root-items.json: one row per entry with its docs/01 inventory row or `absent`.

The entries come from the freeze ONLY: the first path component of every NUL-terminated entry of the files-only listing named by freeze.json (`listing`), the
first component of every `gitlinks` path of freeze.json, and the present untracked root entries recorded at freeze time (`untracked_root_entries`, the one explicit
live-tree carve-out of T161, recorded by `ls -A` at freeze time and NEVER re-listed here). This tool runs no `git`, no `ls`, reads no file of the snapshot and never
lists the live tree: its one freeze check is scripts/register/check_freeze_listing.sh (`freeze_listing_moved` stops it). A root entry created in the live tree after the
freeze is therefore absent from root-items.json (needle). `.git` is not a root entry: it needs no disposition and a `.git` row fails this check.
The decisions are the rules of scripts/register/root_dispositions.yaml (first matching rule wins); an entry no rule decides is written with `disposition: null` and the
run FAILS (exit 1): "no row without a disposition" (docs/21 WP-20). The docs/01 row of an entry is looked up independently of the rules in docs/01 section 3
(table rows by their first backticked path, headings by their words).
Nothing is moved, removed or edited (11.4.122, 11.4.124).

Usage: root_inventory.py --freeze-json F --out DIR [--dispositions Y] [--inventory MD]
       root_inventory.py --check FILE [--freeze-json F]       validate an existing root-items.json (closed sets, `.git` absent, one row per entry; with --freeze-json also
                                                              that the rows equal the entries the freeze yields)
Exit: 0 PASS | 1 a row without a disposition / an invalid row (root-items.json is still written by the generating run, for review) | 2 usage |
      20 refusal (`root_inventory: REFUSED reason=<code> ...`, nothing written).
"""
import argparse
import hashlib
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
DISPOSITIONS = ("problem_source", "component", "documentation", "excluded")
DEFAULT_INVENTORY = os.path.join(ROOT, "specs/001-full-project-audit-remediation/docs/01-system-architecture-map.md")
DEFAULT_RULES = os.path.join(HERE, "root_dispositions.yaml")
GOVERNANCE_ROOT = ("AGENTS.md", "CLAUDE.md", "CONSTITUTION.md", "GEMINI.md", "GETTING_STARTED.md", "MEMORY.md", "QUICK_REFERENCE.md", "README.md")


class Refusal(Exception):
    def __init__(self, reason, detail=""):
        Exception.__init__(self, reason)
        self.reason, self.detail = reason, detail


def refuse(reason, detail=""):
    raise Refusal(reason, detail)


def sha_file(p):
    h = hashlib.sha256()
    with open(p, "rb") as f:
        for c in iter(lambda: f.read(1 << 20), b""):
            h.update(c)
    return h.hexdigest()


# ---------------------------------------------------------------------------------------------- the freeze (listing + freeze.json only)
def read_freeze(fjp):
    chk = os.path.join(HERE, "check_freeze_listing.sh")
    r = subprocess.run(["bash", chk, fjp], stdout=subprocess.PIPE, stderr=subprocess.PIPE)       # MUT:listing-check
    if r.returncode != 0:
        err = r.stderr.decode("utf-8", "replace")
        sys.stderr.write(err)
        m = re.search(r"reason=(\w+)", err)
        refuse(m.group(1) if m else "freeze_listing_moved", "check_freeze_listing.sh exited %d" % r.returncode)
    fj = json.load(open(fjp, encoding="utf-8"))
    recs = open(fj["listing"], "rb").read().split(b"\0")[:-1]
    paths = []
    for rec in recs:
        try:
            paths.append(rec.decode("utf-8"))
        except UnicodeDecodeError:
            refuse("freeze_path_unsafe", repr(rec))
    gl = fj.get("gitlinks") or []
    ut = fj.get("untracked_root_entries")
    if not isinstance(gl, list) or not isinstance(ut, list):
        refuse("freeze_json_invalid", "`gitlinks` and `untracked_root_entries` must be lists (T161 records both)")
    return fj, paths, [g["path"] for g in gl if isinstance(g, dict) and isinstance(g.get("path"), str)], [str(x) for x in ut]


def root_entries(paths, gitlink_paths, untracked):
    """-> {entry: {"kind", "tracked", "tracked_files"}}; also the list of forbidden entries seen (.git)"""
    ent, forbidden = {}, []
    for p in paths:
        first, sep, _rest = p.partition("/")
        e = ent.setdefault(first, {"kind": "directory" if sep else "file", "tracked": True, "tracked_files": 0})
        e["tracked_files"] += 1
        if sep:
            e["kind"] = "directory"
    for g in gitlink_paths:
        first, sep, _rest = g.partition("/")
        e = ent.setdefault(first, {"kind": "gitlink" if not sep else "directory", "tracked": True, "tracked_files": 0})
        if not sep:
            e["kind"] = "gitlink"
        e["gitlinks"] = e.get("gitlinks", 0) + 1
    for u in untracked:
        if u not in ent:
            ent[u] = {"kind": "untracked", "tracked": False, "tracked_files": 0}
    for name in list(ent):
        if name == ".git":
            forbidden.append(name)
            del ent[name]
    return ent, forbidden


# ---------------------------------------------------------------------------------------------- docs/01 lookup (independent of the rules)
def load_inventory(path):
    """-> {first path component: [(section, kind, text)]} from the tables and headings of docs/01 section 3"""
    try:
        lines = open(path, encoding="utf-8").read().split("\n")
    except OSError:
        refuse("inventory_unreadable", path)
    idx, sec, in3 = {}, None, False
    for l in lines:
        m = re.match(r"^## (\d+)\.", l)
        if m:
            in3 = m.group(1) == "3"
            sec = None
            continue
        if not in3:
            continue
        h = re.match(r"^### (3\.\d+)\s+(.*)$", l)
        if h:
            sec = h.group(1)
            title = h.group(2)
            for w in re.findall(r"[A-Za-z0-9_.][A-Za-z0-9_.\-]*", title.split("(")[0]):
                idx.setdefault(w, []).append((sec, "heading", l[4:].strip()[:200]))
            continue
        # the tables of 3.7 and 3.8 name ROOT paths; the tables of 3.1-3.6 name paths inside one application, so a token there is not a root entry. A token with a
        # sub-path (`catalog-api/tests/`) is a row of that sub-path, not of the root entry.
        if sec in ("3.7", "3.8") and l.startswith("|") and not re.match(r"^\|[\s:|-]+\|?\s*$", l):
            cells = [c.strip() for c in l.strip().strip("|").split("|")]
            if not cells:
                continue
            for tok in re.findall(r"`([^`]+)`", cells[0]):
                name = re.sub(r"^\./", "", tok.strip()).rstrip("/")
                if name and "/" not in name:
                    idx.setdefault(name, []).append((sec, "table-row", re.sub(r"\s+", " ", l.strip())[:200]))
                elif name and not name.startswith("/"):
                    idx.setdefault(name.split("/")[0], []).append((sec, "table-row-subpath", re.sub(r"\s+", " ", l.strip())[:200]))
    return idx


def docs01_of(entry, idx):
    # a table row beats a heading word (a heading word such as `Build` may be an ordinary English word)
    rows = idx.get(entry, [])
    rows = [r for r in rows if r[1] == "table-row"] or [r for r in rows if r[1] == "heading"] or [r for r in rows if r[1] == "table-row-subpath"]
    if not rows:
        return "absent"
    s, k, t = rows[0]
    return {"section": s, "kind": k, "text": t}


# ---------------------------------------------------------------------------------------------- rules
def load_rules(path):
    import yaml
    try:
        d = yaml.safe_load(open(path, encoding="utf-8"))
        rules = d["rules"]
        assert d.get("schema") == "root-dispositions/1" and isinstance(rules, list)
    except (OSError, ValueError, KeyError, TypeError, AssertionError):
        refuse("rules_invalid", path)
    ids = set()
    for r in rules:
        if not isinstance(r, dict) or r.get("id") in ids or r.get("disposition") not in DISPOSITIONS or not (r.get("names") or r.get("regex")):
            refuse("rules_invalid", "rule %r: needs a unique id, a disposition in %s and `names` or `regex`" % (r.get("id") if isinstance(r, dict) else r, DISPOSITIONS))
        ids.add(r["id"])
        need = {"problem_source": "source_class", "component": "component", "excluded": "reason"}.get(r["disposition"])
        if need and not r.get(need):
            refuse("rules_invalid", "rule %s: disposition %s needs `%s`" % (r["id"], r["disposition"], need))
        if r["disposition"] == "documentation" and not r.get("doc_class_by"):
            refuse("rules_invalid", "rule %s: documentation needs `doc_class_by`" % r["id"])
    return rules


def match_rule(rules, name):
    for r in rules:
        if name in (r.get("names") or []) or (r.get("regex") and re.match(r["regex"], name)):
            return r
    return None


def row_for(name, info, rules, idx, listing_set):
    row = {"entry": name, "kind": info["kind"], "tracked": info["tracked"], "tracked_files": info["tracked_files"], "docs01": docs01_of(name, idx)}
    r = match_rule(rules, name)
    if r is None:
        row.update(disposition=None, rule=None)
        return row
    row.update(disposition=r["disposition"], rule=r["id"])
    for k in ("source_class", "component", "reason", "doc_class_by", "also_source_classes", "note"):
        if r.get(k) is not None:
            row[k] = r[k]
    if r["disposition"] == "documentation":
        row["doc_class"] = None            # assigned by WP-37 (T282), never here
    if r.get("excluded_files"):
        row["excluded_files"] = [{"path": f["path"], "reason": f["reason"], "in_frozen_listing": f["path"] in listing_set} for f in r["excluded_files"]]
    if r.get("proposed"):
        row["proposed"] = True             # [DEFAULT - adjustable]
    return row


def validate_rows(rows):
    """-> list of problems of a finished rows list (used by the generator and by --check)"""
    bad, seen = [], set()
    for i, r in enumerate(rows):
        e = r.get("entry")
        if not isinstance(e, str) or not e or "/" in e:
            bad.append("row %d: entry %r is not a root entry name" % (i, e))
            continue
        if e == ".git":
            bad.append(".git is not a root entry (a `.git` row fails this check, T161 round-30 review I2)")
        if e in seen:
            bad.append("duplicate row for %s" % e)
        seen.add(e)
        d = r.get("disposition")
        if d not in DISPOSITIONS:
            bad.append("%s: no disposition from the closed set (got %r)" % (e, d))
            continue
        if d == "problem_source" and not r.get("source_class"):
            bad.append("%s: problem_source without a source class" % e)
        if d == "component" and not r.get("component"):
            bad.append("%s: component without an application or module id" % e)
        if d == "excluded" and not r.get("reason"):
            bad.append("%s: excluded without a reason" % e)
        if d == "documentation" and "doc_class" not in r:
            bad.append("%s: documentation row without the doc_class field (null until WP-37 assigns it)" % e)
        if r.get("docs01") != "absent" and not (isinstance(r.get("docs01"), dict) and r["docs01"].get("section")):
            bad.append("%s: docs01 must be `absent` or a row with its section" % e)
    if [r.get("entry") for r in rows] != sorted((r.get("entry") for r in rows), key=lambda x: str(x).encode("utf-8")):
        bad.append("rows are not sorted by entry (byte order)")
    return bad


def summarise(rows, paths):
    md = [r for r in rows if r["kind"] == "file" and r["entry"].endswith(".md")]
    cls = {}
    for r in md:
        k = "governance" if r["entry"] in GOVERNANCE_ROOT else r.get("source_class", "none")
        cls[k] = cls.get(k, 0) + 1
    byd = {}
    for r in rows:
        byd[r["disposition"] or "none"] = byd.get(r["disposition"] or "none", 0) + 1
    return {"entries": len(rows), "by_disposition": dict(sorted(byd.items())),
            "untracked": [r["entry"] for r in rows if not r["tracked"]],
            "root_markdown": {"tracked_files": len(md), "by_class": dict(sorted(cls.items()))},
            "without_disposition": [r["entry"] for r in rows if r["disposition"] is None],
            "proposed_not_owner_decided": sorted(r["entry"] for r in rows if r.get("proposed"))}


def generate(a):
    fj, paths, glinks, untracked = read_freeze(a.freeze_json)
    rules = load_rules(a.dispositions)
    idx = load_inventory(a.inventory)
    ent, forbidden = root_entries(paths, glinks, untracked)
    lset = set(paths)
    rows = [row_for(n, ent[n], rules, idx, lset) for n in sorted(ent, key=lambda x: x.encode("utf-8"))]
    problems = validate_rows(rows)
    if forbidden:
        problems.append("the freeze lists %s as a root entry: .git is not a root entry (T161 excludes it)" % ", ".join(forbidden))
    rec = {"schema": "root-items/1",
           "freeze": {"head": fj.get("head"), "listing_sha256": fj.get("listing_sha256"), "frozen_at": fj.get("frozen_at")},
           "rules": {"file": os.path.basename(a.dispositions), "sha256": sha_file(a.dispositions)},
           "inventory_doc": {"file": os.path.basename(a.inventory), "sha256": sha_file(a.inventory)},
           "rows": rows, "summary": summarise(rows, paths), "problems": problems, "verdict": "FAIL" if problems else "PASS"}
    os.makedirs(a.out, exist_ok=True)
    with open(os.path.join(a.out, "root-items.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(rec, f, indent=1, sort_keys=True)
        f.write("\n")
    print("root_inventory: %s entries=%d %s" % (rec["verdict"], len(rows), json.dumps(rec["summary"]["by_disposition"], sort_keys=True)))
    for p in problems:
        print("root_inventory: PROBLEM " + p)
    return 1 if problems else 0


def check(a):
    try:
        rec = json.load(open(a.check, encoding="utf-8"))
        rows = rec["rows"]
        assert rec.get("schema") == "root-items/1" and isinstance(rows, list)
    except (OSError, ValueError, KeyError, AssertionError, TypeError):
        refuse("root_items_invalid", a.check)
    problems = validate_rows(rows)
    if a.freeze_json:
        fj, paths, glinks, untracked = read_freeze(a.freeze_json)
        ent, forbidden = root_entries(paths, glinks, untracked)
        have = {r.get("entry") for r in rows}
        for n in sorted(set(ent) - have):
            problems.append("%s is a root entry of the freeze with no row" % n)
        for n in sorted(have - set(ent) - {".git"}):
            problems.append("%s has a row but is not a root entry of the freeze" % n)
    for p in problems:
        print("root_inventory: PROBLEM " + p)
    print("root_inventory: check %s rows=%d" % ("FAIL" if problems else "PASS", len(rows)))
    return 1 if problems else 0


def main(argv):
    ap = argparse.ArgumentParser(prog="root_inventory.py")
    ap.add_argument("--freeze-json", dest="freeze_json")
    ap.add_argument("--out")
    ap.add_argument("--dispositions", default=DEFAULT_RULES)
    ap.add_argument("--inventory", default=DEFAULT_INVENTORY)
    ap.add_argument("--check")
    try:
        a = ap.parse_args(argv)
    except SystemExit as e:
        return 2 if e.code else 0
    try:
        if a.check:
            return check(a)
        if not a.freeze_json or not a.out:
            sys.stderr.write("root_inventory: usage: --freeze-json F --out DIR | --check FILE [--freeze-json F]\n")
            return 2
        return generate(a)
    except Refusal as r:
        sys.stderr.write("root_inventory: REFUSED reason=%s %s\n" % (r.reason, r.detail))
        return 20


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
