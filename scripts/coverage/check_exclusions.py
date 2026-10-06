#!/usr/bin/env python3
"""check_exclusions.py - T200. See check_exclusions.sh. Rules (docs/05 7.3, 11.4.224 E):
  1. schema coverage-exclusions/1; `application` equals the file stem (and --application when given); an `exclusions` list is present (empty is legal);
  2. each entry has a relative, non-over-broad `path` (a glob, no absolute path, no `..`, not `*`/`**`/`**/*`/`.`), listed once;
  3. `class` is one of generated-code | vendored-third-party | non-shipping-fixtures-and-golden-assets | first-party;
  4. a non-empty justification (at least MIN_JUST characters);
  5. a first-party entry names a tracked item (an id such as ATM-123, CAT-12, F-INDEX-001, LVA-98) that stands for the plan to bring it into scope;
  6. every pattern the measuring tool uses (--used) is listed (an UNLISTED exclusion voids the measured figure).
The checks are collected, not short-circuited, so one run names every problem."""
import fnmatch, hashlib, json, os, re, sys

CLASSES = ("generated-code", "vendored-third-party", "non-shipping-fixtures-and-golden-assets", "first-party")
MIN_JUST = 12
ITEM_RE = re.compile(r"^[A-Z][A-Z0-9]{1,7}(?:-[A-Z0-9]+)*-[0-9]+$")
BROAD = {"*", "**", "**/*", ".", "./", "./**", "*/**", "**/**"}


def norm(p):
    """The comparison form of a pattern: `vendor/`, `vendor/**` and `vendor` name the same directory."""
    p = p.strip()
    while p.endswith("/"):
        p = p[:-1]
    if p.endswith("/**"):
        p = p[:-3]
    return p


def overbroad(p):
    return p.strip() in BROAD


def main(argv):
    path = None; app = None; used = None; root = None; out = None
    i = 0
    while i < len(argv):
        k = argv[i]
        if k in ("--application", "--used", "--root", "--json") and i + 1 < len(argv):
            v = argv[i + 1]
            if k == "--application": app = v
            elif k == "--used": used = v
            elif k == "--root": root = v
            else: out = v
            i += 2
        elif not k.startswith("--") and path is None:
            path = k; i += 1
        else:
            sys.stderr.write("check_exclusions: usage: check_exclusions.sh FILE [--application NAME] [--used FILE] [--root DIR] [--json OUT]\n"); sys.exit(2)
    if path is None:
        sys.stderr.write("check_exclusions: usage: check_exclusions.sh FILE [--application NAME] [--used FILE] [--root DIR] [--json OUT]\n"); sys.exit(2)
    problems = []
    doc = None; digest = ""
    try:
        raw = open(path, "rb").read(); digest = hashlib.sha256(raw).hexdigest()
        import yaml
        doc = yaml.safe_load(raw.decode("utf-8"))
    except Exception as e:
        problems.append("cannot read %s: %s" % (path, e))
    entries_out = []; first_party = []
    exclusions = []
    if doc is not None:
        if not isinstance(doc, dict) or doc.get("schema") != "coverage-exclusions/1":
            problems.append("schema must be coverage-exclusions/1")
        stem = os.path.splitext(os.path.basename(path))[0]
        if not isinstance(doc, dict) or doc.get("application") != (app or stem):
            problems.append("application %r must equal %r (the file stem or --application)" % (doc.get("application") if isinstance(doc, dict) else None, app or stem))
        exclusions = doc.get("exclusions") if isinstance(doc, dict) else None
        if not isinstance(exclusions, list):
            problems.append("the `exclusions` list is missing (write `exclusions: []` to state there are none)")
            exclusions = []
    seen = set(); listed = []
    for n, e in enumerate(exclusions):
        e = e if isinstance(e, dict) else {}
        p = e.get("path"); cls = e.get("class"); just = (e.get("justification") or "").strip() if isinstance(e.get("justification"), str) else ""
        item = e.get("tracked_item")
        label = "entry %d (%s)" % (n + 1, p)
        if not isinstance(p, str) or not p.strip():
            problems.append("%s: no path" % label); continue
        listed.append(p)
        if p.startswith("/") or ".." in p.split("/"):
            problems.append("%s: the path must be relative to the application root and must not contain .." % label)
        if overbroad(p):
            problems.append("%s: over-broad pattern: it would exclude the whole tree" % label)
        if p in seen:
            problems.append("%s: duplicate path" % label)
        seen.add(p)
        if cls not in CLASSES:
            problems.append("%s: class %r is not one of %s" % (label, cls, ", ".join(CLASSES)))
        if len(just) < MIN_JUST:
            problems.append("%s: unjustified: a justification of at least %d characters is required" % (label, MIN_JUST))
        item_ok = isinstance(item, str) and bool(ITEM_RE.match(item))
        if cls == "first-party":
            first_party.append({"path": p, "tracked_item": item})
        if cls == "first-party" and not item_ok:
            problems.append("%s: a first-party exclusion needs a tracked item (an id such as ATM-123) naming the plan to bring it into scope" % label)
        rec = {"path": p, "class": cls, "tracked_item": item, "justification_chars": len(just)}
        if root:
            matched = 0
            for dp, dns, fns in os.walk(root):
                dns[:] = [d for d in dns if d != ".git"]
                for f in fns:
                    rel = os.path.relpath(os.path.join(dp, f), root)
                    if fnmatch.fnmatch(rel, p) or fnmatch.fnmatch(rel, p.replace("/**", "/*")) or rel == p or rel.startswith(p.rstrip("*").rstrip("/") + "/") and p.endswith("**"):
                        matched += 1
            rec["matched_files"] = matched; rec["stale"] = matched == 0
        entries_out.append(rec)
    used_unlisted = []
    if used:
        try:
            for line in open(used, encoding="utf-8"):
                u = line.strip()
                if u and not u.startswith("#") and norm(u) not in [norm(x) for x in listed]:
                    used_unlisted.append(u)
        except OSError as e:
            problems.append("cannot read --used %s: %s" % (used, e))
    if used_unlisted:
        for u in used_unlisted:
            problems.append("unlisted exclusion: the measuring tool excludes %r but the checked-in list does not carry it" % u)
    verdict = {"schema": "exclusions-check/1", "file": path, "sha256": digest, "application": (doc.get("application") if isinstance(doc, dict) else None),
               "verdict": "FAIL" if problems else "PASS", "problems": problems, "entries": entries_out, "first_party_entries": first_party,
               "unlisted": used_unlisted}
    if out:
        os.makedirs(os.path.dirname(os.path.abspath(out)), exist_ok=True)
        with open(out, "w", encoding="utf-8") as fh:
            fh.write(json.dumps(verdict, indent=2, sort_keys=True) + "\n")
    for p_ in problems:
        sys.stderr.write("check_exclusions: FAIL: %s\n" % p_)
    print("check_exclusions: %s %s (%d entr%s, %d first-party)" % (verdict["verdict"], path, len(entries_out), "y" if len(entries_out) == 1 else "ies", len(first_party)))
    sys.exit(1 if problems else 0)


if __name__ == "__main__":
    main(sys.argv[1:])
