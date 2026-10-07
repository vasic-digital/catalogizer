#!/usr/bin/env python3
"""check_exclusions.py - T200. See check_exclusions.sh. Rules (docs/05 7.3, 11.4.224 E):
  1. schema coverage-exclusions/1; `application` equals the file stem (and --application when given); an `exclusions` list is present (empty is legal);
  2. each entry has a relative, non-over-broad `path` (a glob, no absolute path, no `..`, not `*`/`**`/`**/*`/`.`), listed once;
  3. `class` is one of generated-code | vendored-third-party | non-shipping-fixtures-and-golden-assets | first-party;
  4. a non-empty justification (at least MIN_JUST characters);
  5. a first-party entry names a tracked item (an id such as ATM-123, CAT-12, F-INDEX-001, LVA-98) that stands for the plan to bring it into scope;
  6. every pattern the measuring tool uses is listed (an UNLISTED exclusion voids the measured figure). The patterns the tool uses come from the tool's OWN config
     (coverage/exclusions/tools.yaml names the config, scripts/coverage/fence_lib.py reads it); a hand-kept --used file must equal that extraction (a stale hand list FAILs),
     and a fence entry the tool does not apply while it would still measure the files the entry names is LISTED-BUT-NOT-APPLIED (FAIL: the figure would claim a scope it did not have).
Review round 1 of WP-23 (WF11 I5, I6) - with --root the rules check the REAL conditions, not proxies:
  7. each non-first-party entry's matched files are consistent with its class (generated-code: a generated marker or a generated directory; vendored-third-party: a vendored
     directory; non-shipping-fixtures-and-golden-assets: a test/fixture directory or file name); a counterexample FAILs the entry (the claim is false for that file);
  8. the exclusions together (and each entry alone) must not name more than half of the root's tracked files: an exclusion list that removes most of the tree voids the figure
     in substance even when every entry is individually justified (the 11.4.224 E silent-evasion channel); the denylist of literal globs it replaces is kept as a fast first check;
  9. a first-party entry must be VERIFIABLE: either its tracked item exists in the --items-file (a register export), or `measured_by: {app, lane}` names a row of
     scripts/containers/lanes.tsv that measures it elsewhere; a well-formed id nobody can check is `tracked_item_unverifiable`, never a pass.
The checks are collected, not short-circuited, so one run names every problem."""
import hashlib, json, os, re, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fence_lib

CLASSES = ("generated-code", "vendored-third-party", "non-shipping-fixtures-and-golden-assets", "first-party")
MIN_JUST = 12
ITEM_RE = re.compile(r"^[A-Z][A-Z0-9]{1,7}(?:-[A-Z0-9]+)*-[0-9]+$")
BROAD = {"*", "**", "**/*", ".", "./", "./**", "*/**", "**/**", "?*/**", "*/*", "**/*.*", "*.*", "**/.*"}


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
    path = None; app = None; used = None; root = None; out = None; repo = None; items = None; tools = None; lanes = None
    i = 0
    while i < len(argv):
        k = argv[i]
        if k in ("--application", "--used", "--root", "--json", "--repo", "--items-file", "--tools", "--lanes") and i + 1 < len(argv):
            v = argv[i + 1]
            if k == "--application": app = v
            elif k == "--used": used = v
            elif k == "--root": root = v
            elif k == "--repo": repo = v
            elif k == "--items-file": items = v
            elif k == "--tools": tools = v
            elif k == "--lanes": lanes = v
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
    if repo is None:
        d = os.path.dirname(os.path.abspath(path))
        if os.path.basename(d) == "exclusions" and os.path.basename(os.path.dirname(d)) == "coverage":
            repo = os.path.dirname(os.path.dirname(d))
    _rf = []
    def get_root_files():   # lazy: listing a big checkout is only worth it when there is an entry to match (a fence with no entries lists nothing)
        if root and not _rf:
            _rf.append(fence_lib.list_files(root))
        return _rf[0] if _rf else None
    root_abs = os.path.abspath(root) if root else None
    known_items = None
    if items:
        try:
            known_items = set(l.strip() for l in open(items, encoding="utf-8") if l.strip() and not l.startswith("#"))
        except OSError as e:
            problems.append("cannot read --items-file %s: %s" % (items, e))
    lane_rows = None
    lanes_path = lanes or (os.path.join(repo, "scripts", "containers", "lanes.tsv") if repo else None)
    if lanes_path and os.path.isfile(lanes_path):
        lane_rows = set()
        for line in open(lanes_path, encoding="utf-8"):
            cols = line.rstrip("\n").split("\t")
            if len(cols) >= 2 and not line.startswith("#"):
                lane_rows.add((cols[0], cols[1]))
    union = set()
    for n, e in enumerate(exclusions):
        e = e if isinstance(e, dict) else {}
        p = e.get("path"); cls = e.get("class"); just = (e.get("justification") or "").strip() if isinstance(e.get("justification"), str) else ""
        item = e.get("tracked_item"); mby = e.get("measured_by")
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
        mby_ok = isinstance(mby, dict) and isinstance(mby.get("app"), str) and isinstance(mby.get("lane"), str)
        if cls == "first-party":
            first_party.append({"path": p, "tracked_item": item, "measured_by": mby})
            if not item_ok and not mby_ok:
                problems.append("%s: a first-party exclusion needs a tracked item (an id such as ATM-123) naming the plan to bring it into scope, or `measured_by: {app, lane}` naming the lane that measures it" % label)
            elif item_ok and not mby_ok:
                if known_items is None:   # MUT:item_unverifiable
                    problems.append("%s: tracked_item_unverifiable: %s is well-formed but there is no register export (--items-file) to prove it exists" % (label, item))
                elif item not in known_items:   # MUT:item_missing
                    problems.append("%s: tracked item %s does not exist in %s" % (label, item, items))
            if mby_ok:
                if lane_rows is None:
                    problems.append("%s: measured_by cannot be verified: no lanes table (scripts/containers/lanes.tsv or --lanes)" % label)
                elif (mby["app"], mby["lane"]) not in lane_rows:   # MUT:measured_by
                    problems.append("%s: measured_by %s/%s is not a row of the lanes table: nothing measures these files" % (label, mby["app"], mby["lane"]))
        rec = {"path": p, "class": cls, "tracked_item": item, "justification_chars": len(just)}
        if root:
            root_files = get_root_files()
            matched = [r for r in root_files if fence_lib.matches(p, r)]
            union.update(matched)
            rec["matched_files"] = len(matched); rec["stale"] = len(matched) == 0
            if root_files and len(matched) * 2 > len(root_files):   # MUT:fraction_entry
                problems.append("%s: over-broad: it names %d of the root's %d files (more than half): an exclusion that removes most of the tree voids the figure" % (label, len(matched), len(root_files)))
            if cls in CLASSES and cls != "first-party" and matched:   # MUT:class_evidence
                bad_ex = []
                for r in matched:
                    ok_, why = fence_lib.class_evidence(cls, r, os.path.join(root_abs, r))
                    if not ok_:
                        bad_ex.append("%s (%s)" % (r, why))
                if bad_ex:
                    problems.append("%s: class %s is not true of %d of %d matched file(s), e.g. %s" % (label, cls, len(bad_ex), len(matched), "; ".join(bad_ex[:3])))
        entries_out.append(rec)
    if root and exclusions and get_root_files():
        root_files = get_root_files()
        if len(union) * 2 > len(root_files):   # MUT:fraction_union
            problems.append("the exclusions together name %d of the root's %d files (more than half): the figure would describe a minority of the code" % (len(union), len(root_files)))
    used_unlisted = []; listed_not_applied = []; applied_checked = False; applied_note = None
    nl = [norm(x) for x in listed]
    applied = None
    if repo and root:
        tp = tools or os.path.join(repo, "coverage", "exclusions", "tools.yaml")
        tdoc = None
        if os.path.isfile(tp):
            try:
                import yaml
                tdoc = yaml.safe_load(open(tp, encoding="utf-8").read())
            except Exception as e:
                problems.append("cannot read tools file %s: %s" % (tp, e))
        rec_t = ((tdoc or {}).get("apps") or {}).get(app or (doc.get("application") if isinstance(doc, dict) else None) or os.path.splitext(os.path.basename(path))[0]) if tdoc else None
        if rec_t:
            kind = rec_t.get("kind")
            if kind == "fence":
                applied, applied_note, incl = list(listed), "the collector applies the fence file itself", None
            else:
                cfg = os.path.join(repo, rec_t.get("config", "")) if rec_t.get("config") else ""
                applied, applied_note, incl = fence_lib.extract_applied(kind, cfg)
            if applied is None:
                problems.append("applied exclusions unverifiable: %s" % applied_note)   # MUT:applied_unreadable
            else:
                applied_checked = True
                na = [norm(x) for x in applied]
                for u in applied:
                    if norm(u) not in nl:
                        used_unlisted.append(u)
                # listed but not applied: the tool would still measure files the entry names
                if kind != "fence" and listed and get_root_files():
                    root_files = get_root_files()
                    for p_ in listed:
                        if norm(p_) in na:
                            continue
                        exts = fence_lib.MEASURABLE.get(kind)   # only files the tool can instrument are a scope claim (a json schema or a snapshot is not measured)
                        still = [r for r in root_files if fence_lib.matches(p_, r) and (exts is None or r.endswith(exts))
                                 and not any(fence_lib.matches(a_, r) for a_ in applied)
                                 and (incl is None or any(fence_lib.matches(i_, r) for i_ in incl))]
                        if still:   # MUT:listed_not_applied
                            listed_not_applied.append(p_)
                            problems.append("listed but not applied: the fence lists %r but the tool (%s) does not exclude it, and would still measure %d file(s) it names (e.g. %s)" % (p_, applied_note, len(still), still[0]))
    if used:
        try:
            hand = [l.strip() for l in open(used, encoding="utf-8") if l.strip() and not l.startswith("#")]
            for line in hand:
                if norm(line) not in nl and line not in used_unlisted:
                    used_unlisted.append(line)
            if applied is not None and sorted(set(norm(x) for x in hand)) != sorted(set(norm(x) for x in applied)):   # MUT:hand_stale
                problems.append("the hand-kept used list %s differs from the exclusions the tool config applies (%s): a stale hand list is not evidence" % (used, applied_note))
        except OSError as e:
            problems.append("cannot read --used %s: %s" % (used, e))
    if used_unlisted:
        for u in used_unlisted:
            problems.append("unlisted exclusion: the measuring tool excludes %r but the checked-in list does not carry it" % u)
    verdict = {"schema": "exclusions-check/1", "file": path, "sha256": digest, "application": (doc.get("application") if isinstance(doc, dict) else None),
               "verdict": "FAIL" if problems else "PASS", "problems": problems, "entries": entries_out, "first_party_entries": first_party,
               "unlisted": used_unlisted, "listed_not_applied": listed_not_applied, "content_checked": bool(root), "applied_checked": applied_checked,
               "applied_source": applied_note}
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
