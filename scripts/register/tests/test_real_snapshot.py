#!/usr/bin/env python3
"""test_real_snapshot.py - T162/T165/T167 (WP-20): the enumerator and the lead scan against a REAL frozen snapshot, every number checked against
an INDEPENDENT instrument (grep -E / grep -w over the same snapshot, not the code under test) after the instrument itself found its control needle.
Reusable for T166 (independent recount) and T176 (the real sources). Written after the WF23 review found that every fixture-built oracle agreed with
the implementation while the real corpus did not (S-03 statuses all NULL, 21 prose-only ids missing, 181 inflected lead lines missed).

Env: REAL_FREEZE_JSON (required): a freeze json {snapshot, manifest, listing, gitlinks, remotes, head, frozen_at}; EV (scratch evidence root, optional);
     WI (engine binary for the scratch register DB); EXPECT_STRICT_INVALID (optional integer: the OD-17 fact, 399 on the 2026-10-08 HEAD corpus).
Writes only below TMPDIR. Exit 0 only when every check passed.
"""
import collections, json, os, re, sqlite3, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__)); REG = os.path.dirname(HERE); ROOT = os.path.dirname(os.path.dirname(REG))
FAILS = 0
os.environ.setdefault("WI", os.path.join(ROOT, "submodules/constitution/scripts/workable-items/bin/workable-items-linux"))


def check(label, cond, info=""):
    global FAILS
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:600])); FAILS += 0 if cond else 1


def sh(cmd, cwd=None, inp=None):
    return subprocess.run(cmd, cwd=cwd, input=inp, capture_output=True)


fjp = os.environ.get("REAL_FREEZE_JSON")
if not fjp:
    print("REFUSED: REAL_FREEZE_JSON is required (this test reads a real frozen snapshot only)"); sys.exit(20)
fj = json.load(open(fjp)); snap = fj["snapshot"]
with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as w:
    # ---- run both tools
    o1, o2 = os.path.join(w, "enum"), os.path.join(w, "lead")
    env = dict(os.environ, EV=os.environ.get("EV", os.path.join(w, "ev_none")))
    r = subprocess.run([sys.executable, os.path.join(REG, "enumerate_sources.py"), "--freeze-json", fjp, "--out", o1], capture_output=True, text=True, env=env)
    check("enumerator exits 0 on the real snapshot", r.returncode == 0, r.stdout + r.stderr)
    r = subprocess.run([sys.executable, os.path.join(REG, "lead_scan.py"), "--freeze-json", fjp, "--out", o2], capture_output=True, text=True, env=env)
    check("lead scan exits 0 on the real snapshot", r.returncode == 0, r.stdout + r.stderr)
    if FAILS:
        print("RESULT: fail=%d" % FAILS); sys.exit(1)
    db = os.path.join(w, "reg.db"); ar = subprocess.run([os.path.join(REG, "apply_ext.sh"), "--db", db], capture_output=True, text=True)
    check("scratch register DB", ar.returncode == 0, ar.stdout + ar.stderr)
    for sqlp in (os.path.join(o1, "source_entries.sql"), os.path.join(o2, "lead_scan.sql")):
        r = subprocess.run(["sqlite3", db, ".read " + sqlp], capture_output=True, text=True)
        check("import of %s applies cleanly" % os.path.basename(sqlp), r.returncode == 0 and r.stdout + r.stderr == "", r.stderr)
    con = sqlite3.connect(db)
    Q = lambda sql, *a: con.execute(sql, a).fetchall()
    stats = json.load(open(os.path.join(o1, "enumeration-stats.json")))

    # ---- instrument controls (an instrument that cannot see a known needle certifies nothing, 11.4.201(7))
    ctl = os.path.join(w, "ctl.txt"); open(ctl, "w").write("a TODO here\n- [ ] box\nFIX-QA-2026-04-21-001 id\nthe todos are stubbed\ndepending on it\n")
    chk = lambda pat, flags=("-E",): sh(["grep", "-c"] + list(flags) + ["-e", pat, ctl]).stdout.decode().strip()
    check("control: grep sees a TODO, a checkbox, an id, an inflected word and does NOT see 'depending' as a word", chk(r"\bTODO\b") == "1" and chk(r"^\s*[-*+]\s+\[( |x|X)\]\s+") == "1" and chk(r"FIX-QA-[0-9]{4}") == "1"
          and chk(r"(^|[^[:alnum:]_])stubbed([^[:alnum:]_]|$)", ("-iE",)) == "1" and chk(r"(^|[^[:alnum:]_])pending([^[:alnum:]_]|$)", ("-iE",)) == "0")

    # ---- S-03 statuses (finding A4): the real file marks status with a glyph
    st3 = Q("SELECT raw_status, count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator='TASK_TRACKER.md' GROUP BY 1")
    tot3 = sum(n for _s, n in st3)
    check("S-03: no row has a NULL status, every status is one of the file's own legend words (%s)" % dict(st3), tot3 > 0 and all(s in ("Not Started", "In Progress", "Complete", "Blocked") for s, _n in st3), st3)
    ind = int(sh(["grep", "-cE", r"^\| *[0-9]+\.[0-9]+", os.path.join(snap, "TASK_TRACKER.md")]).stdout.decode().strip() or 0)
    check("S-03: one row per table row (independent grep count %d)" % ind, tot3 == ind, (tot3, ind))
    # ---- S-21 statuses
    nul = Q("SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.kind='constitution_conflict' AND e.locator LIKE '%#item%' AND e.locator NOT GLOB '*#item*.*' AND e.raw_status IS NULL")[0][0]
    items = Q("SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.kind='constitution_conflict' AND e.locator NOT GLOB '*#item*.*'")[0][0]
    nums = len(re.findall(r"(?m)^\d+\.\s", re.split(r"(?m)^## ", "\n" + open(os.path.join(snap, ".specify/memory/constitution.md"), encoding="utf-8").read().split("## Known Conflicts", 1)[1], maxsplit=1)[0]))
    check("S-21: every numbered item (%d, independent count %d) has a status, none NULL (%d)" % (items, nums, nul), items == nums and nul == 0, (items, nums, nul))
    # ---- ids: independent census
    fam = [r"CATAPI-DEFECT-[0-9]+", r"(FIX|DEFER)-QA-[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]{3}", r"DEFER-[0-9]{3}", r"FIX-OC[0-9]+-[0-9]+", r"FINDING-[0-9]+", r"HQA-DOCS-[0-9]+",
           r"HQA-[0-9]{4}", r"HQA-PHASE[0-9]+-[A-Z]+-[0-9]+", r"FIX-(OBS|BROWSER)-[0-9]+", r"FIX-[0-9]{3}", r"BUG-[0-9]{3}", r"FIX-CONCURRENCY-[0-9]{4}-[0-9]{2}-[0-9]{2}", r"FIX-CATAPI-[0-9]{4}-[0-9]{2}-[0-9]{2}-[A-Z]+"]
    pat = r"(^|[^A-Za-z0-9-])(" + "|".join(fam) + ")"
    g = sh(["grep", "-rIohE", "--exclude-dir=specs", "--exclude-dir=submodules", "--exclude-dir=.audit", "--exclude-dir=.git", "-e", pat, "."], cwd=snap).stdout.decode("utf-8", "replace").split("\n")
    # grep -o with the leading boundary group keeps one character of context: strip it
    ind_ids = {re.sub(r"^[^A-Z]", "", x) for x in g if x}
    ind_ids = {re.sub(r"[^A-Za-z0-9]+$", "", x) for x in ind_ids}
    # the independent instrument can not exclude scripts/register/tests/ by --exclude-dir with a path: remove carrier hits by re-grepping that directory
    gt = sh(["grep", "-rIohE", "-e", pat, "scripts/register/tests"], cwd=snap).stdout.decode("utf-8", "replace").split("\n") if os.path.isdir(os.path.join(snap, "scripts/register/tests")) else []
    carriers = {re.sub(r"[^A-Za-z0-9]+$", "", re.sub(r"^[^A-Z]", "", x)) for x in gt if x}
    elsewhere = collections.Counter()
    for x in carriers:
        r2 = sh(["grep", "-rIlE", "--exclude-dir=specs", "--exclude-dir=submodules", "--exclude-dir=.audit", "--exclude-dir=.git", "-e", re.escape(x), "."], cwd=snap).stdout.decode().split("\n")
        elsewhere[x] = len([p for p in r2 if p and not p.startswith("./scripts/register/tests/")])
    ind_ids -= {x for x in carriers if elsewhere[x] == 0}
    got = {r[0] for r in Q("SELECT legacy_id FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator LIKE 'legacy-ids:%'")}
    check("ids: the entries equal the independent grep census of the snapshot (%d distinct ids)" % len(ind_ids), got == ind_ids, "only in entries %s; only in grep %s" % (sorted(got - ind_ids), sorted(ind_ids - got)))
    check("ids: one entry per distinct id (no id in two sources)", len(got) == Q("SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator LIKE 'legacy-ids:%'")[0][0])
    # ---- S-01
    lst = [p for p in open(fj["listing"], "rb").read().split(b"\0") if re.fullmatch(rb"docs/issues/[^/]+\.md", p)] if fj.get("listing") else []
    n1 = Q("SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.kind='issue_file' AND s.locator='docs/issues/*.md'")[0][0]
    check("S-01: one entry per docs/issues/*.md file of the listing (%d)" % len(lst), n1 == len(lst), (n1, len(lst)))
    sinv = stats["classes"]["S-01"].get("frontmatter_strict_yaml_invalid_count")
    print("FACT (OD-17): strict YAML refuses %s of %d front-matter blocks" % (sinv, n1))
    if os.environ.get("EXPECT_STRICT_INVALID"):
        check("S-01: the strict-YAML refusal count equals the recorded fact", str(sinv) == os.environ["EXPECT_STRICT_INVALID"], sinv)
    # ---- checkbox lines of the doc03-counted docs/ files (finding A3), independent grep
    for f in ("docs/COMPREHENSIVE_PACKAGE_SUMMARY.md", "docs/README_IMPLEMENTATION_PACKAGE.md", "docs/MASTER_AUDIT_AND_IMPLEMENTATION_PLAN.md", "docs/DISABLED_FEATURES_AUDIT.md"):
        if not os.path.exists(os.path.join(snap, f)):
            continue
        k = int(sh(["grep", "-cE", r"^\s*[-*+]\s+\[( |x|X)\]\s+", os.path.join(snap, f)]).stdout.decode().strip() or 0)
        e = Q("SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE e.locator GLOB ? AND e.locator NOT GLOB '*#*' AND s.locator NOT LIKE 'lead-scan:%' AND s.kind <> 'code_marker'", f + ":L*")[0][0]
        check("A3: %s has %d checkbox entries (independent grep %d)" % (f, e, k), e == k, (e, k))
    ex = stats["checkbox_lines_not_enumerated"]
    print("FACT: %d Markdown files hold %d checkbox lines no checkbox source enumerates (listed in enumeration-stats.json, doc03 3.1 rule 2)" % (ex["files"], ex["lines"]))
    # ---- S-23: every non-binary file, independent grep -Iw
    gm = sh(["grep", "-rIlwE", "-e", "TODO|FIXME|HACK|XXX", "."], cwd=snap).stdout.decode("utf-8", "replace").split("\n")
    gm = {x[2:] for x in gm if x}
    big = {p for p in gm if os.path.getsize(os.path.join(snap, p)) > 2 * 1024 * 1024}
    have = {r[0].split(":L")[0] for r in Q("SELECT e.locator FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.kind='code_marker' AND e.raw_status='marker'")}
    # symlinked files grep -r does not follow are not in gm either
    only_g = sorted(gm - have - big); only_h = sorted(have - gm)
    check("S-23: the files with marker rows equal grep -Iw's file set (%d files; %d over 2 MiB skipped by design); differences grep-only=%s entries-only=%s" % (len(gm), len(big), only_g[:5], only_h[:5]),
          not only_g and not only_h, (only_g[:10], only_h[:10]))
    # ---- lead scan: the independent instrument is grep -iE with explicit word boundaries over the population
    pop = [p.decode() for p in open(fj["listing"], "rb").read().split(b"\0") if p.endswith(b".md") and not any(p.decode().startswith(g["path"] + "/") or p.decode() == g["path"] for g in fj.get("gitlinks", []))]
    words = r"unfinished|not[ -]implemented|stubs?|stubbed|stubbing|missing|broken|known[ -]issues?|workarounds?|deprecated|disabled|skipped|todos?|fixmes?|regress[[:alnum:]_]*|outstanding|pending|not[ -]yet"
    gp = sh(["xargs", "-0", "grep", "-HniE", "-e", r"(^|[^[:alnum:]_])(" + words + r")([^[:alnum:]_]|$)", "--"], cwd=snap, inp=b"\0".join(p.encode() for p in pop if os.path.isfile(os.path.join(snap, p)) and not os.path.islink(os.path.join(snap, p))) + b"\0").stdout.decode("utf-8", "replace").split("\n")
    ind_rows = {re.match(r"^(.*?):(\d+):", x).group(1) + ":L" + re.match(r"^(.*?):(\d+):", x).group(2) for x in gp if re.match(r"^(.*?):(\d+):", x)}
    rows = {r[0] for r in Q("SELECT e.locator FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator LIKE 'lead-scan:%'")}
    check("C1: the lead rows equal the independent grep -i word-boundary set over the population (%d lines); grep-only=%s rows-only=%s" % (len(ind_rows), sorted(ind_rows - rows)[:3], sorted(rows - ind_rows)[:3]), rows == ind_rows, (len(ind_rows - rows), len(rows - ind_rows)))
    disp = collections.Counter(r[0] for r in Q("SELECT raw_status FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator LIKE 'lead-scan:%'"))
    run = json.load(open(os.path.join(o2, "lead-scan-run.json")))
    check("C5: emitted_rows equals the rows in the imported database (%d)" % len(rows), run["emitted_rows"] == len(rows), (run["emitted_rows"], len(rows)))
    dup_no_cov = Q("SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator LIKE 'lead-scan:%' AND e.raw_status='duplicate of structured source' AND (e.raw_severity IS NULL OR e.raw_severity NOT LIKE 'covered_by=%')")[0][0]
    check("C2: every duplicate row names the structured entry that covers it (%s)" % dict(disp), dup_no_cov == 0, dup_no_cov)
    # the covering entry must exist as a structured row
    miss = 0
    for (cov,) in Q("SELECT DISTINCT raw_severity FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator LIKE 'lead-scan:%' AND e.raw_status='duplicate of structured source'"):
        srcl, entl = cov[len("covered_by="):].split(" :: ", 1)
        if not Q("SELECT 1 FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator=? AND e.locator=?", srcl, entl):
            miss += 1
    check("C2: every covered_by target exists as a row of the imported structured source (%d distinct targets missing)" % miss, miss == 0, miss)
    # ---- external rows
    ext = Q("SELECT e.locator, e.raw_status FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.kind='external_ticket'")
    check("A2: one external row per freeze-json remote (%d) and per named service (4)" % len(fj.get("remotes", [])), len([x for x in ext if x[0].startswith("remote:")]) == len(fj.get("remotes", [])) and len([x for x in ext if x[0].startswith("service:")]) == 4, ext)
    sqlall = open(os.path.join(o1, "source_entries.sql"), encoding="utf-8").read()
    cred = [r.get("url", "") for r in fj.get("remotes", []) if isinstance(r, dict)]
    ui = [re.match(r"^[a-z+]+://([^/@]+)@", u).group(1) for u in cred if re.match(r"^[a-z+]+://([^/@]+)@", u)]
    check("11.4.10: no remote userinfo (%d with credentials in the freeze json) appears in the sql" % len(ui), not any(x in sqlall for x in ui), ui)
print("RESULT: fail=%d" % FAILS)
sys.exit(1 if FAILS else 0)
