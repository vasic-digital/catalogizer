#!/usr/bin/env python3
"""test_frontmatter_yaml.py - T164 (WP-20): the front-matter reader loads every docs/issues/*.md file of the frozen snapshot with no error,
reproduces the per-status and per-category counts and the enhancement-suggestion count of an independent recount (grep -c -H over the status:
and category: lines, not the parser), handles every front-matter FORM a YAML reader handles, AND states how many blocks a strict YAML parser
refuses.

OD-17 (owner decision pending, UNCONFIRMED who answers): the plan premise "a YAML parser loads every docs/issues/*.md file" is FALSE. Measured
on the 1,778-ticket HEAD corpus (2026-10-08): yaml.safe_load refuses 399 front-matter blocks (every one `ScannerError: mapping values are not
allowed here`: an unquoted colon inside a plain value such as `resolution: a: b`). The enumerator therefore ships a TOLERANT line reader
(enumerate_sources.parse_frontmatter: it reads what YAML reads for every form that occurs, and a colon inside a value is data) AND a strict
checker (enumerate_sources.strict_frontmatter) whose refusal count is reported here and in enumeration-stats.json as a fact. This test asserts
both honestly: tolerant = 0 parse errors and recount equality; strict = the count is printed and, with --expect-strict-invalid N, equals N.
It does NOT decide OD-17 (parse the blocks with a recorded pre-normalisation of unquoted colons, or accept the line reader).

Env: ENUMERATE_SOURCES (enumerator whose parse_frontmatter / strict_frontmatter is under test; mutation and RED runs point it at a copy).
Usage: test_frontmatter_yaml.py --freeze-json <freeze.json> [--parser tolerant|strict-yaml|naive-split] [--expect-strict-invalid N]
       test_frontmatter_yaml.py --selftest            (fixture scenarios incl. moved-snapshot, needle, RED parsers)
The file set is the docs/issues/*.md entries of the frozen listing (NUL separated, key `listing` of the freeze json; without that key the
snapshot directory is walked and the listing absence is printed as UNCONFIRMED). The snapshot is re-hashed by
scripts/register/check_freeze_snapshot.sh BEFORE any file of it is read; its refusal stops the test with `freeze_snapshot_moved` naming the file
and prints no count. Never `git ls-files` (it would read the live index).
Parsers (broken-body and drop-category are RED test doubles used by --selftest only): tolerant = enumerate_sources.parse_frontmatter (the reader of the Stage 0 enumerator, T165); strict-yaml = yaml.safe_load of the block
(RED: a colon inside a value is a YAML error); naive-split = the 22-line scanner of doc04 section 14.4 (split on the first colon; RED on
trailing comments, block scalars, quotes and continuation lines).
Exit: 0 PASS, 1 FAIL (RED), 20 refusal (freeze_snapshot_moved, freeze_json_missing).
"""
import collections, importlib.util, json, os, re, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)
sys.path.insert(0, HERE)
EXPECT = {"files": 1778, "status": {"open": 1, "fixed": 492, "resolved": 704, "closed": 299, "wontfix": 282}, "enhancement": 225}


def load_enum():
    spec = importlib.util.spec_from_file_location("enumerate_sources", os.environ.get("ENUMERATE_SOURCES", os.path.join(REG, "enumerate_sources.py")))   # env: mutation / RED runs
    m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m); return m


_EN = None


def en():
    global _EN
    if _EN is None:
        _EN = load_enum()
    return _EN


def parse(mode, text):
    """-> (frontmatter dict, body text, error or None)"""
    if mode == "tolerant":
        fm, body, errs = en().parse_frontmatter(text)
        return fm, body, (",".join(errs) or None)
    if mode in ("broken-body", "drop-category"):      # RED test doubles: a reader that loses the body / the category (prove the recount comparisons are load-bearing)
        fm, body, errs = en().parse_frontmatter(text)
        if mode == "drop-category": fm.pop("category", None)
        return fm, ("" if mode == "broken-body" else body), (",".join(errs) or None)
    m = re.match(r"---\n(.*?)\n---\n(.*)", text.replace("\r\n", "\n"), re.S)
    body = m.group(2) if m else text
    if mode == "strict-yaml":
        d, err = en().strict_frontmatter(text)
        return (d or {}), body, err
    if mode == "naive-split":
        fm = dict(l.split(":", 1) for l in (m.group(1).splitlines() if m else []) if ":" in l)
        return {k.strip(): v.strip() for k, v in fm.items()}, body, (None if m else "no_front_matter")
    raise SystemExit("unknown parser " + mode)


def forms_check(mode):
    """the YAML forms the reader must handle (shared with test_enumerate_absolute.py): -> list of failure lines"""
    import wp20_fixture as F
    bad = []
    for label, text, want in F.FRONTMATTER_FORMS:
        fm, _b, err = parse(mode, text)
        if err or {k: str(v) for k, v in fm.items()} != want:
            bad.append("form %s: got %s (error %s), want %s" % (label, fm, err, want))
    return bad


def listing(fj, snap):
    lp = fj.get("listing")
    if lp and os.path.isfile(lp):
        chk = os.path.join(REG, "check_freeze_listing.sh")
        if os.path.exists(chk):
            r = subprocess.run([chk, lp] + ([fj["manifest"]] if "manifest" in fj else []), capture_output=True)
            if r.returncode != 0:
                print("REFUSED reason=freeze_listing_moved " + r.stderr.decode("utf-8", "replace").strip()); sys.exit(20)
        else:
            print("# check_freeze_listing.sh absent (T161): listing check UNCONFIRMED")
        return [p for p in open(lp, "rb").read().split(b"\0") if p]
    print("# freeze json has no `listing`: snapshot directory walked instead (UNCONFIRMED vs T161 listing)")
    out = []
    for dp, dns, fns in os.walk(os.fsencode(snap)):
        for n in fns: out.append(os.path.relpath(os.path.join(dp, n), os.fsencode(snap)))
    return sorted(out)


def run(fj_path, mode, expect_strict=None):
    try: fj = json.load(open(fj_path, encoding="utf-8")); snap, manifest = fj["snapshot"], fj["manifest"]
    except (OSError, ValueError, KeyError): print("REFUSED reason=freeze_json_missing %s" % fj_path); return 20
    chk = os.path.join(REG, "check_freeze_snapshot.sh")
    r = subprocess.run([chk, snap, manifest], capture_output=True)                       # MUT:snapshot-check
    if r.returncode != 0:
        print("REFUSED reason=freeze_snapshot_moved\n" + r.stderr.decode("utf-8", "replace").strip()); return 20
    files = sorted(p for p in listing(fj, snap) if re.fullmatch(rb"docs/issues/[^/]+\.md", p))
    fails = []
    parsed_s, parsed_c, errors = collections.Counter(), collections.Counter(), []
    enh_parser, strict_bad = 0, []
    for p in files:
        text = open(os.path.join(os.fsencode(snap), p), "rb").read().decode("utf-8", "replace")
        fm, body, err = parse(mode, text)
        if err or "status" not in fm: errors.append((p.decode("utf-8", "replace"), err or "no status key"))
        parsed_s[str(fm.get("status"))] += 1
        if "category" in fm: parsed_c[str(fm["category"])] += 1
        if re.search(r"enhancement suggestion", body + "\n" + "\n".join(str(v) for v in fm.values()), re.I): enh_parser += 1   # from the PARSED parts
        _d, serr = en().strict_frontmatter(text)
        if serr: strict_bad.append((p.decode("utf-8", "replace"), serr))
    # independent recount: grep over the snapshot (cwd = snapshot, NUL-safe -Z, `--` before the paths)
    def grep(pattern, extra=()):
        res = subprocess.run(["xargs", "-0", "grep", "-H", "-Z", "-E"] + list(extra) + ["-e", pattern, "--"], input=b"\0".join(files) + b"\0",
                             cwd=snap, capture_output=True)
        return res.stdout
    status_c, cat_c = collections.Counter(), collections.Counter()
    per_file = collections.Counter()
    for chunk in grep(r"^status:").split(b"\n"):
        if b"\0" in chunk:
            path, line = chunk.split(b"\0", 1); per_file[path] += 1
            status_c[re.sub(r"^status:\s*", "", line.decode("utf-8", "replace")).strip()] += 1
    for chunk in grep(r"^category:").split(b"\n"):
        if b"\0" in chunk:
            cat_c[re.sub(r"^category:\s*", "", chunk.split(b"\0", 1)[1].decode("utf-8", "replace")).strip()] += 1
    enh_grep = len([x for x in grep(r"enhancement suggestion", ("-i", "-l")).split(b"\0") if x.strip()])   # -l -Z: NUL-terminated names
    multi = [p for p, n in per_file.items() if n != 1]
    print("files=%d parser=%s parse_errors=%d" % (len(files), mode, len(errors)))
    print("recount status:    %s" % dict(sorted(status_c.items())))
    print("parser  status:    %s" % dict(sorted(parsed_s.items())))
    print("recount category:  %s" % dict(sorted(cat_c.items())))
    print("parser  category:  %s" % dict(sorted(parsed_c.items())))
    print("enhancement suggestion files: recount=%d parser=%d" % (enh_grep, enh_parser))
    print("EXPECT (plan time, doc03): files=%d %s enhancement=%d" % (EXPECT["files"], EXPECT["status"], EXPECT["enhancement"]))
    print("FACT (OD-17): strict YAML (yaml.safe_load) refuses %d of %d front-matter blocks%s" % (
        len(strict_bad), len(files), (" (" + ", ".join("%s x%d" % kv for kv in sorted(collections.Counter(e for _p, e in strict_bad).items())) + ")") if strict_bad else ""))
    diffs = []
    if len(files) != EXPECT["files"]: diffs.append("files: recount %d vs plan %d" % (len(files), EXPECT["files"]))
    for k in sorted(set(EXPECT["status"]) | set(status_c)):
        if status_c.get(k, 0) != EXPECT["status"].get(k, 0): diffs.append("status %s: recount %d vs plan %d" % (k, status_c.get(k, 0), EXPECT["status"].get(k, 0)))
    if enh_grep != EXPECT["enhancement"]: diffs.append("enhancement: recount %d vs plan %d" % (enh_grep, EXPECT["enhancement"]))
    print("DIFFERENCES from the plan-time numbers (itemised, informational, never a failure on their own): %s" % (diffs or "none"))
    for p, e in errors[:10]: fails.append("parse error %s: %s" % (p, e))
    if len(errors) > 10: fails.append("... %d parse errors in total" % len(errors))
    if multi: fails.append("files with != 1 status: line: %d" % len(multi))
    if dict(parsed_s) != dict(status_c): fails.append("parser status counts differ from the independent recount")
    if dict(parsed_c) != dict(cat_c): fails.append("parser category counts differ from the independent recount")
    if enh_parser != enh_grep: fails.append("parser enhancement count %d != recount %d" % (enh_parser, enh_grep))
    if expect_strict is not None and len(strict_bad) != expect_strict: fails.append("strict-yaml refusals %d != expected %d" % (len(strict_bad), expect_strict))
    for f in forms_check(mode): fails.append(f)
    for f in fails: print("FAIL " + f)
    print("RESULT: %s" % ("PASS" if not fails else "FAIL"))
    return 0 if not fails else 1


def selftest():
    import wp20_fixture as F
    me = os.path.abspath(__file__); bad = 0
    def check(label, cond, info=""):
        nonlocal bad
        print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:300])); bad += 0 if cond else 1
    def sub(fj, mode="tolerant", extra=()): return subprocess.run([sys.executable, me, "--freeze-json", fj, "--parser", mode] + list(extra), capture_output=True, text=True)
    def mkfj(w, name, extra_files=()):
        tree = os.path.join(w, name); F.build(tree); F.tickets(tree)
        for rel, text in extra_files: F.w(tree, rel, text)
        fj = os.path.join(w, name + ".freeze.json"); F.freeze(tree, fj); j = json.load(open(fj))
        lst = os.path.join(w, name + ".files.txt")
        open(lst, "wb").write(b"".join(os.path.relpath(os.path.join(dp, n), tree).encode() + b"\0" for dp, _, fns in os.walk(tree) for n in sorted(fns)))
        j["listing"] = lst; json.dump(j, open(fj, "w")); return tree, fj
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as w:
        tree, fj = mkfj(w, "tree")
        r = sub(fj, extra=["--expect-strict-invalid", "2"])
        check("tolerant reader loads every fixture ticket, matches the recount (status, category, enhancement), handles every form, strict count 2 (the two colon-in-value tickets)",
              r.returncode == 0 and "parse_errors=0" in r.stdout and "files=4" in r.stdout and "strict YAML (yaml.safe_load) refuses 2 of 4" in r.stdout, r.stdout[-500:])
        r = sub(fj, "strict-yaml"); check("RED: strict yaml.safe_load fails on a colon inside a value (parse error AND the colon-in-value form)", r.returncode == 1 and "parse error" in r.stdout and "form colon inside a value" in r.stdout, r.stdout[-300:])
        r = sub(fj, "naive-split")
        check("RED: the doc04 naive splitter fails the form checks (trailing comment, block scalars, quotes, continuation lines)", r.returncode == 1 and all(x in r.stdout for x in ("form trailing comment", "form folded block scalar", "form literal block scalar", "form double quoted", "form single quoted", "form continuation line")), r.stdout[-600:])
        r = sub(fj, extra=["--expect-strict-invalid", "3"]); check("the strict-refusal count is asserted: a wrong expectation (3) FAILs", r.returncode == 1 and "strict-yaml refusals 2 != expected 3" in r.stdout, r.stdout[-200:])
        # needle: ONE ticket planted into the snapshot moves exactly its own status count (the recount and the parser both read the snapshot)
        tree2, fj2 = mkfj(w, "tree2", [("docs/issues/HELIX-960-planted.md", F.ticket(960, "wontfix"))])
        a = sub(fj).stdout; b = sub(fj2).stdout
        ca = re.search(r"recount status:\s+(.*)", a).group(1); cb = re.search(r"recount status:\s+(.*)", b).group(1)
        check("needle: a ticket planted in the snapshot adds exactly one wontfix to the recount and to the parser", "'wontfix': 1" in ca and "'wontfix': 2" in cb and "files=5" in b and re.search(r"parser  status:\s+(.*)", b).group(1) == cb, (ca, cb))
        # a file present in the tree but NOT in the frozen listing is not read (the listing is the authority)
        tree3, fj3 = mkfj(w, "tree3"); F.w(tree3, "docs/issues/HELIX-961-unlisted.md", F.ticket(961))
        # the unlisted file also moves the manifest check, so refresh the manifest only: the listing stays the frozen one
        subprocess.run([sys.executable, os.path.join(REG, "snapshot_manifest.py"), tree3], stdout=open(tree3 + ".freeze.json.manifest.json", "w"))
        r = sub(fj3); check("needle: a ticket in the snapshot but absent from the frozen listing is REFUSED by the listing check `freeze_listing_moved` (W2: check_freeze_listing.sh now exists; before it, this state was merely not counted, files=4)",
                            r.returncode == 20 and "freeze_listing_moved" in r.stdout and "missing docs/issues/HELIX-961-unlisted.md" in r.stdout, r.stdout[:200])
        # the recount comparisons are load-bearing: a reader that loses the body breaks the enhancement count, one that loses the category breaks the category count
        tree6, fj6 = mkfj(w, "tree6", [("docs/issues/HELIX-963-enh.md", "---\nid: HELIX-963\nstatus: fixed\ncategory: ux\n---\n\n# E\n\nEnhancement suggestions: more\n")])
        r = sub(fj6, "broken-body"); check("RED: a reader that loses the ticket body fails the enhancement-suggestion comparison", r.returncode == 1 and "parser enhancement count" in r.stdout, r.stdout[-300:])
        r = sub(fj6, "drop-category"); check("RED: a reader that loses the category fails the category comparison", r.returncode == 1 and "parser category counts differ" in r.stdout, r.stdout[-300:])
        r = sub(fj6); check("tolerant reader on the same tree passes (the doubles fail for their own defect only)", r.returncode == 0, r.stdout[-300:])
        tree7, fj7 = mkfj(w, "tree7", [("docs/issues/HELIX-964-twice.md", "---\nid: HELIX-964\nstatus: fixed\nstatus: open\n---\n\n# Twice\n")])
        r = sub(fj7); check("a ticket with two status: lines is reported (files with != 1 status: line)", r.returncode == 1 and "files with != 1 status: line: 1" in r.stdout, r.stdout[-300:])
        # moved snapshot: refused, no count printed
        open(os.path.join(tree, "docs/issues/HELIX-001-a.md"), "a").write("tampered\n")
        r = sub(fj); check("moved snapshot: freeze_snapshot_moved naming the file, no count printed", r.returncode == 20 and "freeze_snapshot_moved" in r.stdout and "HELIX-001-a.md" in r.stdout and "files=" not in r.stdout, r.stdout[:300])
        # a ticket whose status the independent recount reads differently from the parser (`status : fixed` is not `^status:`) is a recount mismatch, not a silent pass
        tree4, fj4 = mkfj(w, "tree4", [("docs/issues/HELIX-962-spaced.md", "---\nid: HELIX-962\nstatus : fixed\nseverity: low\n---\n\n# Spaced\n")])
        r = sub(fj4); check("a status the grep recount cannot see (`status : fixed`) is a count mismatch, never silently passed", r.returncode == 1 and "parser status counts differ" in r.stdout, r.stdout[-300:])
    print("SELFTEST: %s" % ("PASS" if not bad else "FAIL")); return 0 if not bad else 1


if __name__ == "__main__":
    a = sys.argv[1:]
    if a[:1] == ["--selftest"]: sys.exit(selftest())
    mode = "tolerant"
    if "--parser" in a: mode = a[a.index("--parser") + 1]
    exp = int(a[a.index("--expect-strict-invalid") + 1]) if "--expect-strict-invalid" in a else None
    fj = a[a.index("--freeze-json") + 1] if "--freeze-json" in a else os.environ.get("FREEZE_JSON")
    if not fj: print("REFUSED reason=freeze_json_missing (no --freeze-json and no FREEZE_JSON): the test reads the frozen snapshot only"); sys.exit(20)
    sys.exit(run(fj, mode, exp))
