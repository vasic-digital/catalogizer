#!/usr/bin/env python3
"""test_enumerate_absolute.py - T162/T165 (WP-20) ABSOLUTE-content tests of the Stage 0 enumerator (scripts/register/enumerate_sources.py).

test_enumerate_planted.sh asserts deltas (+1 per planted entry); this file asserts what the entries ARE: fields, statuses, locators, refusals,
the credential rule, the extension coverage, the cross-source id index, the external rows and the re-import gate. Every case came from the
independent review of 2026-10-08 (WF23, findings A1-A7, B1-B4, D4 and the author-independent mutants EM01-EM11) and is run verbatim by
mutate_wf23_reviewer.sh against the reviewer's mutants.
Env: ENUMERATE_SOURCES (script under test; a mutation run points it at a mutated copy beside copies of its helpers), WI (engine binary for the
scratch register DB leg, default the committed binary), TMPDIR.
Exit 0 only when every check passed.
"""
import json, os, re, shutil, sqlite3, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__)); REG = os.path.dirname(HERE); ROOT = os.path.dirname(os.path.dirname(REG))
sys.path.insert(0, HERE)
import wp20_fixture as F
ENUM = os.environ.get("ENUMERATE_SOURCES", os.path.join(REG, "enumerate_sources.py"))
APPLY = os.path.join(REG, "apply_ext.sh")
os.environ.setdefault("WI", os.path.join(ROOT, "submodules/constitution/scripts/workable-items/bin/workable-items-linux"))
FAILS = 0
SCHEMA = ("CREATE TABLE reg_sources(source_id INTEGER PRIMARY KEY, kind TEXT, locator TEXT UNIQUE, parser TEXT, content_sha256 TEXT, last_scanned_at TEXT, scanned_entry_count INT);"
          "CREATE TABLE reg_source_entries(entry_id INTEGER PRIMARY KEY, source_id INT, locator TEXT, legacy_id TEXT, title TEXT, raw_status TEXT, raw_severity TEXT, entry_sha256 TEXT, scanned_at TEXT, UNIQUE(source_id,locator));")


def check(label, cond, info=""):
    global FAILS
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:400])); FAILS += 0 if cond else 1


def run_enum(tree, out, fj=None, extra=(), env=None):
    fj = fj or tree + ".freeze.json"
    if not os.path.exists(fj):
        F.freeze(tree, fj)
    e = dict(os.environ); e.update(env or {})
    r = subprocess.run([sys.executable, ENUM, "--freeze-json", fj, "--out", out] + list(extra), capture_output=True, text=True, env=e)
    return r.returncode, r.stdout + r.stderr


def load_rows(out):
    """-> ({(source locator, entry locator): (legacy_id, title, raw_status, raw_severity)}, {source locator: scanned_entry_count})"""
    db = sqlite3.connect(":memory:"); db.executescript(SCHEMA)
    sql = "\n".join(l for l in open(os.path.join(out, "source_entries.sql"), encoding="utf-8").read().split("\n") if not l.startswith("."))
    db.executescript(sql.replace("PRAGMA foreign_keys=ON;", ""))
    rows = {(r[0], r[1]): r[2:] for r in db.execute("SELECT s.locator, e.locator, e.legacy_id, e.title, e.raw_status, e.raw_severity FROM reg_source_entries e JOIN reg_sources s USING(source_id)")}
    return rows, {r[0]: r[1] for r in db.execute("SELECT locator, scanned_entry_count FROM reg_sources")}


def by_loc(rows):
    return {k[1]: v for k, v in rows.items()}


def stats(out):
    return json.load(open(os.path.join(out, "enumeration-stats.json")))


def case(w, name, mutate=None, remotes=None):
    t = os.path.join(w, name); F.build(t)
    if mutate:
        mutate(t)
    fj = t + ".freeze.json"; F.freeze(t, fj)
    if remotes is not None:
        j = json.load(open(fj)); j["remotes"] = remotes; json.dump(j, open(fj, "w"))
    o = os.path.join(w, "o_" + name)
    rc, msg = run_enum(t, o, fj)
    return t, o, rc, msg


with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as w:
    if not os.path.exists(ENUM):
        check("enumerator exists", False, ENUM); print("RESULT: fail=%d" % FAILS); sys.exit(1)
    base, ob, rc, msg = case(w, "base")
    check("baseline enumeration exits 0", rc == 0, msg)
    if rc != 0:
        print("RESULT: fail=%d" % FAILS); sys.exit(1)
    rows, counts = load_rows(ob); L = by_loc(rows); st = stats(ob)

    # ---- S-01 fields (EM06): status and severity are not swapped, the id and the heading are the ticket's own
    check("S-01 ticket a: legacy id, heading, status fixed, severity high", L.get("docs/issues/HELIX-001-a.md") == ("HELIX-001", "Ticket 1", "fixed", "high"), L.get("docs/issues/HELIX-001-a.md"))
    check("S-01 ticket b: status open", (L.get("docs/issues/HELIX-001-b.md") or [0] * 3)[2] == "open", L.get("docs/issues/HELIX-001-b.md"))
    check("S-01 strict-yaml measurement: both fixture tickets hold a colon in a value, so a strict YAML reader refuses them (a measured fact, OD-17)",
          st["classes"]["S-01"].get("frontmatter_strict_yaml_invalid_count") == 2, st["classes"]["S-01"].get("frontmatter_strict_yaml_invalid_count"))
    # ---- S-03 status words and glyphs from the file's own legend (A4)
    st3 = {v[0]: v[2] for k, v in L.items() if k.startswith("TASK_TRACKER.md:L")}
    check("S-03 glyph and word statuses: 0.1 Not Started, 0.2 Complete, 0.3 In Progress, 0.4 Blocked (glyph), 0.5 Blocked (word)",
          st3 == {"0.1": "Not Started", "0.2": "Complete", "0.3": "In Progress", "0.4": "Blocked", "0.5": "Blocked"}, st3)
    # ---- S-21 statuses over the whole item block, preface fallback, inline status-marker sub-items (A4)
    s21 = {v[0]: v[2] for k, v in L.items() if "#item" in k and "." not in k.split("#item")[1]}
    check("S-21 item statuses: block words, preface fallback for items without a word of their own",
          s21 == {"1": "DECIDED", "2": "OPEN", "3": "DECIDED", "4": "NOTE", "5": "FIXED,OPEN"}, s21)
    subs = {v[0]: v[2] for k, v in L.items() if re.search(r"#item\d+\.\d+$", k)}
    check("S-21 sub-items: the bullet of item 2 and the two bold status-marker segments of item 5", subs == {"2.1": None, "5.1": "FIXED", "5.2": "OPEN"}, subs)
    # ---- S-25 provider rows: names only (EM07)
    prov = {v[0]: v for k, v in L.items() if k.startswith("SECURITY_KEY_ROTATION_REQUIRED.md:L")}
    check("S-25 provider rows: legacy id is the variable NAME, title is the provider name only (no table line, no url)",
          set(prov) == {"ASTICA_API_KEY", "CEREBRAS_API_KEY"} and {v[1] for v in prov.values()} == {"Astica", "Cerebras"}, prov)
    # ---- S-16: the LATEST scan only (EM04)
    sec = sorted(k.split("#", 1)[1] for k in L if k.startswith("docs/security/gosec-") and "#" in k)
    check("S-16 vulnerability rows come from the latest gosec file only", [x.split(":")[1] for x in sec] == ["G102", "G103"] and all("20260201" in k for k in L if k.startswith("docs/security/gosec-") and "#" in k), sec)
    check("S-16 the failed snyk scan is a scan-failed row", any(k.startswith("docs/security/snyk-go-") and k.endswith("#scan-failed") and v[2] == "scan-failed" for k, v in L.items()))
    # ---- A3: checkbox entries in docs/ files, and the explicit exclusion list
    cb = {k: v[2] for k, v in L.items() if re.match(r"docs/(COMPREHENSIVE_PACKAGE_SUMMARY|README_IMPLEMENTATION_PACKAGE|MASTER_AUDIT_AND_IMPLEMENTATION_PLAN)\.md:L", k)}
    check("A3 checkbox lines of the three docs/ files doc03 counts become entries (2 + 1 + 2)", len(cb) == 5 and sorted(cb.values()).count("checked") == 1, cb)
    ex = st["checkbox_lines_not_enumerated"]
    check("A3 checkbox lines no checkbox source enumerates are LISTED, not ignored (procedural checklist howto.md: 3 lines)",
          ex["list"] == [["docs/procedures/howto.md", 3]] and ex["lines"] == 3 and ex["files"] == 1, ex)
    # ---- A1 + EM03: one entry per distinct id across the whole snapshot, prose-only ids included, family set incl. DEFER-NNN, FINDING, FIX-OC, HQA-DOCS
    ids = {v[0]: (k, st_) for k, v in rows.items() for st_ in [k[0]] if k[0].startswith("legacy-ids:")}
    check("A1 prose-only ids outside S-14/S-15 get an XID entry each (DEFER-001, FINDING-7, FIX-OC2-001)",
          {"DEFER-001", "FINDING-7", "FIX-OC2-001"} <= set(ids) and all("outside the S-14 and S-15" in ids[i][1] for i in ("DEFER-001", "FINDING-7", "FIX-OC2-001")), sorted(ids))
    check("A1 ids inside the S-14 and S-15 files keep their class (CATAPI-DEFECT-001 in S-14, FIX-QA-2026-04-21-001 in S-15)",
          "S-14 files" in ids["CATAPI-DEFECT-001"][1] and "S-15 files" in ids["FIX-QA-2026-04-21-001"][1], ids.get("CATAPI-DEFECT-001"))
    check("A1 the id entry locator is the first occurrence and the title is its line", ("docs/OTHER_REF.md:L3#DEFER-001" in L) and "named here only" in L["docs/OTHER_REF.md:L3#DEFER-001"][1], [k for k in L if "DEFER-001" in k])
    # carriers: documents and tests that QUOTE ids are not sources of ids
    def m_carrier(t):
        F.w(t, "specs/plan.md", "quotes FIX-QA-2099-01-01-001\n")
        F.w(t, "scripts/register/tests/fx.py", "x = 'FIX-QA-2099-01-02-002'\n")
        F.w(t, "submodules/s/readme.md", "FIX-QA-2099-01-03-003\n")
        F.w(t, "docs/qa/q2.md", "FIX-QA-2099-01-04-004 and FIX-QA-2026-04-21-001 again\n")
    t, o, rc, msg = case(w, "carrier", m_carrier)
    if rc == 0:
        Lk = by_loc(load_rows(o)[0])
        idsk = sorted(v[0] for k, v in Lk.items() if "#" in k and re.search(r"#(FIX-QA|DEFER|FINDING|CATAPI)", k))
        check("A1 carriers (specs/, scripts/register/tests/, submodules/) are not scanned for ids; an id in two files is ONE entry",
              not any("2099-01-0" in i for i in idsk if i != "FIX-QA-2099-01-04-004") and "FIX-QA-2099-01-04-004" in idsk and idsk.count("FIX-QA-2026-04-21-001") == 1, idsk)
        check("A1 the census residue is stated: id-like tokens no family matches are listed in the stats, not dropped",
              "id_like_unmatched" in stats(o)["classes"]["XID"] and "id_scan_excluded_prefixes" in stats(o)["classes"]["XID"], stats(o)["classes"]["XID"].keys())
    else:
        check("carrier tree enumerates", False, msg)
    # ---- A2 + credentials: external rows
    ext = {k: v for k, v in L.items() if k.startswith(("remote:", "service:"))}
    check("A2 one row per freeze-json remote (origin, github), not queried", {k for k in ext if k.startswith("remote:")} == {"remote:origin", "remote:github"} and all(v[2] == "not_queried" for k, v in ext.items() if k.startswith("remote:")), ext)
    check("A2 one row per named service; marker_found / marker_absent says what the snapshot holds",
          {k: v[2] for k, v in ext.items() if k.startswith("service:")} == {"service:firebase-crashlytics": "marker_found", "service:sonarqube": "marker_found", "service:snyk": "marker_absent", "service:trivy": "marker_absent"}, ext)
    sqltxt = open(os.path.join(ob, "source_entries.sql"), encoding="utf-8").read()
    check("11.4.10: no remote credential (userinfo, token) appears anywhere in the sql, the stats or the kinds file",
          not any(x in (sqltxt + open(os.path.join(ob, "enumeration-stats.json")).read()) for x in ("FIXTURETOKEN123", "fixtureuser")))
    kinds = {k: v["kind"] for k, v in json.load(open(os.path.join(ob, "source-class-kinds.json")))["classes"].items()}
    check("A2 the external rows use the kind external_ticket", kinds.get("EXT") == "external_ticket", kinds.get("EXT"))
    # ---- E4: names the committed locked.sh import-sql reads
    h = open(os.path.join(ob, "source_entries.sha256")).read(); h2 = open(os.path.join(ob, "source_entries.sql.sha256")).read()
    import hashlib
    check("E4 source_entries.sha256 (locked.sh sibling) and source_entries.sql.sha256 (T165 name) are the same one bare lowercase hash line of the sql",
          h == h2 and re.fullmatch(r"[0-9a-f]{64}\n", h) and h[:64] == hashlib.sha256(open(os.path.join(ob, "source_entries.sql"), "rb").read()).hexdigest(), (h, h2))
    lk = open(os.path.join(REG, "locked.sh"), encoding="utf-8").read()
    check("E4 contract: the committed locked.sh import-sql derives its sibling as <stem>.sha256 and takes ONE argument (re-read each run, so a change of locked.sh is noticed)",
          'stem="${IMPORT%.sql}"; sib="$stem.sha256"' in lk and '[ $# -eq 1 ] || usage' in lk)

    # ---- A5 + EM01 + EM02: markers and skipped tests in every non-binary file, whatever its extension; B1: no locator collision
    def m_ext(t):
        for n in ("Dockerfile", "Makefile", "a.rb", "a.php", "a.xml", "a.scss", "a.dart", "a.cs", "a.properties", "noext"):
            F.w(t, "x/" + n, "# TODO: probe %s\n" % n)
        F.w(t, "x/c.go", "// TODO: control go\n")
        F.w(t, "x/bin.dat", "TODO in a binary \0 file\n")
        F.w(t, "x/skips_test.go", 'package x\nfunc f(t *T) { t.Skip("a") }\nfunc g(b *B) { b.SkipNow() }\nfunc h(t, b T) { if c { t.Skip("a") } else { b.Skip("b") } }\n')
        F.w(t, "x/a.test.js", 'it.skip("a", f);\nit.skip("a", f); xit("b", g);\ndescribe.skip("d", f); xdescribe("e", f);\n')
        F.w(t, "x/K.kt", "@Ignore\nclass K\n")
        F.w(t, "x/r.rs", "#[ignore]\nfn r() {}\n")
        F.w(t, "x/p.py", "@pytest.mark.skip\ndef t(): pass\n")
    t, o, rc, msg = case(w, "markers", m_ext)
    check("A5/B1 probe tree enumerates (rc 0; no duplicate_entry_locator from two skip hits on one line)", rc == 0, msg)
    if rc == 0:
        rowsm, _ = load_rows(o); Lm = by_loc(rowsm)
        got = {k for k in Lm if k.startswith("x/") and k.endswith(":L1:TODO")}
        check("A5 every non-binary file is scanned whatever its extension (10 probe files + c.go), the binary file is not",
              got == {"x/%s:L1:TODO" % n for n in ("Dockerfile", "Makefile", "a.rb", "a.php", "a.xml", "a.scss", "a.dart", "a.cs", "a.properties", "noext", "c.go")} and not any(k.startswith("x/bin.dat") for k in Lm), sorted(got))
        check("A5 the binary file is counted under skipped.binary", stats(o)["classes"]["S-23"]["skipped"]["binary"] >= 1, stats(o)["classes"]["S-23"]["skipped"])
        sk = {k: v[2] for k, v in Lm.items() if ":skip" in k and k.startswith("x/")}
        want = {"x/skips_test.go:L2:skip", "x/skips_test.go:L3:skip", "x/skips_test.go:L4:skip", "x/skips_test.go:L4:skip#2",
                "x/a.test.js:L1:skip-ts", "x/a.test.js:L2:skip-ts", "x/a.test.js:L2:skip-ts#2", "x/a.test.js:L3:skip-ts", "x/a.test.js:L3:skip-ts#2",
                "x/K.kt:L1:skip-kt", "x/r.rs:L1:skip-rs", "x/p.py:L1:skip-py"}
        check("EM02/B1 skipped tests of every language become rows; two hits on one line are numbered #1, #2 across the regexes of the tag", set(sk) == want and set(sk.values()) == {"skipped_test"}, sorted(set(sk) ^ want))

    # ---- A6: a bank case without an id is a case too
    def m_bank(t):
        F.w(t, "challenges/helixqa-banks/idless.yaml", "test_cases:\n  - id: has-id\n    name: A\n  - name: no-id-case\n    steps: [x]\n")
    t, o, rc, msg = case(w, "bank", m_bank)
    if rc == 0:
        Lb = by_loc(load_rows(o)[0])
        check("A6 bank case with an id is #has-id, the sibling without an id is #@1 with no legacy id and its own name as title",
              Lb.get("challenges/helixqa-banks/idless.yaml#has-id", [0])[0] == "has-id" and
              Lb.get("challenges/helixqa-banks/idless.yaml#@1") == (None, "no-id-case", None, None), {k: v for k, v in Lb.items() if "idless" in k})
    else:
        check("A6 bank tree enumerates", False, msg)

    # ---- A7 / D1: front-matter forms the tolerant reader must handle the way YAML does, and the colon-in-value form YAML refuses
    import importlib.util
    sp = importlib.util.spec_from_file_location("en_under_test", ENUM); en = importlib.util.module_from_spec(sp); sp.loader.exec_module(en)
    forms = F.FRONTMATTER_FORMS
    for label, text, want in forms:
        fm, _b, errs = en.parse_frontmatter(text)
        check("A7 tolerant reader, %s" % label, fm == want and not errs, (fm, errs))
    fm, _b, errs = en.parse_frontmatter("---\nid: x\nstatus: fixed\n")
    check("D1/EM11 an unterminated block is reported as an error, the reader does not invent a mapping", errs == ["unterminated"], (fm, errs))
    _d, serr = en.strict_frontmatter(forms[6][1])
    check("D1 strict reader (yaml.safe_load) refuses the colon-in-value form: ScannerError", serr == "ScannerError", serr)
    _d, serr = en.strict_frontmatter(forms[0][1])
    check("D1 strict reader accepts `key : value`", serr is None, serr)

    def m_fm(t):
        F.w(t, "docs/issues/HELIX-777-bad.md", "---\nid: HELIX-777\nstatus: open\n\n# no closing marker\n")
        F.w(t, "docs/issues/HELIX-778-form.md", "---\nid: HELIX-778\nstatus : fixed # note\nseverity: >\n  high\n---\n\n# Form ticket\n")
    t, o, rc, msg = case(w, "fm", m_fm)
    if rc == 0:
        Lf = by_loc(load_rows(o)[0]); stf = stats(o)
        check("EM11 a ticket whose front matter is malformed is still an entry (nothing dropped) and is listed under frontmatter_problems",
              "docs/issues/HELIX-777-bad.md" in Lf and ["docs/issues/HELIX-777-bad.md", "unterminated"] in stf["classes"]["S-01"].get("frontmatter_problems", []), Lf.get("docs/issues/HELIX-777-bad.md"))
        check("A7 the enumerator reads `status : fixed # note` as fixed and the folded severity as high", Lf.get("docs/issues/HELIX-778-form.md") == ("HELIX-778", "Form ticket", "fixed", "high"), Lf.get("docs/issues/HELIX-778-form.md"))
    else:
        check("fm tree enumerates", False, msg)

    # ---- EM08 / EM10: checkbox bullets and CRLF
    def m_cb(t):
        F.w(t, "MASTER_EXECUTION_CHECKLIST.md", "# C\n\n- [ ] dash\n* [ ] star\n+ [x] plus\n  - [X] nested\n")
        F.w(t, "docs/MASTER_EXECUTION_CHECKLIST.md", "# C\r\n\r\n- [ ] crlf item\r\n")
    t, o, rc, msg = case(w, "cb", m_cb)
    if rc == 0:
        Lc = by_loc(load_rows(o)[0])
        got = {k: (v[1], v[2]) for k, v in Lc.items() if k.startswith("MASTER_EXECUTION_CHECKLIST.md:L")}
        check("EM08 checkbox bullets - * + and a nested one are all recognised, with their checked state",
              got == {"MASTER_EXECUTION_CHECKLIST.md:L3": ("dash", "unchecked"), "MASTER_EXECUTION_CHECKLIST.md:L4": ("star", "unchecked"),
                      "MASTER_EXECUTION_CHECKLIST.md:L5": ("plus", "checked"), "MASTER_EXECUTION_CHECKLIST.md:L6": ("nested", "checked")}, got)
        check("EM10 a CRLF line has the title of its LF form (the CR is not part of the entry)", Lc["docs/MASTER_EXECUTION_CHECKLIST.md:L3"][1] == "crlf item", Lc.get("docs/MASTER_EXECUTION_CHECKLIST.md:L3"))
        shas = re.findall(r"'docs/MASTER_EXECUTION_CHECKLIST.md:L3'.*?'([0-9a-f]{64})'", open(os.path.join(o, "source_entries.sql"), encoding="utf-8").read())
        check("EM10 the entry sha256 is the sha256 of the line without its CR", shas and shas[0] == hashlib.sha256(b"- [ ] crlf item").hexdigest(), shas)
    else:
        check("cb tree enumerates", False, msg)

    # ---- EM09: duplicate locators are refused, nothing written
    def m_dup(t):     # ids c1, c1 and c1~2: the second c1 gets the suffix ~2 and collides with the explicit id c1~2
        F.w(t, "challenges/helixqa-banks/dup.yaml", "test_cases:\n  - id: c1\n  - id: c1\n  - id: c1~2\n")
    t, o, rc, msg = case(w, "dup", m_dup)
    check("EM09 bank ids c1, c1, c1~2 give two entries with one locator: refused duplicate_entry_locator (exit 20), nothing written", rc == 20 and "duplicate_entry_locator" in msg and not os.path.exists(os.path.join(o, "source_entries.sql")), (rc, msg))

    # ---- B3: the entry check cannot be replaced from the command line
    t, o, rc, msg = case(w, "chk")
    r = subprocess.run([sys.executable, ENUM, "--freeze-json", t + ".freeze.json", "--out", os.path.join(w, "o_chk2"), "--check-script", "/bin/true"], capture_output=True, text=True)
    check("B3 --check-script is not an option any more: usage, exit 2, nothing written", r.returncode == 2 and not os.path.exists(os.path.join(w, "o_chk2")), (r.returncode, r.stderr))
    # ---- B4: PyYAML absent is a refusal, not a different output
    shim = os.path.join(w, "noyaml"); os.makedirs(shim); open(os.path.join(shim, "yaml.py"), "w").write('raise ImportError("blocked for the test")\n')
    rc, msg = run_enum(t, os.path.join(w, "o_noyaml"), env={"PYTHONPATH": shim})
    check("B4 without PyYAML: refused pyyaml_missing (exit 20), nothing written", rc == 20 and "pyyaml_missing" in msg and not os.path.exists(os.path.join(w, "o_noyaml")), (rc, msg))
    # ---- A2: a freeze json without `remotes` is refused
    j = json.load(open(t + ".freeze.json")); del j["remotes"]; json.dump(j, open(os.path.join(w, "noremotes.json"), "w"))
    rc, msg = run_enum(t, os.path.join(w, "o_norem"), fj=os.path.join(w, "noremotes.json"))
    check("A2 a freeze json with no `remotes` list is refused freeze_remotes_missing (exit 20), nothing written", rc == 20 and "freeze_remotes_missing" in msg and not os.path.exists(os.path.join(w, "o_norem")), (rc, msg))
    j["remotes"] = []; json.dump(j, open(os.path.join(w, "emptyremotes.json"), "w"))
    rc, msg = run_enum(t, os.path.join(w, "o_emptyrem"), fj=os.path.join(w, "emptyremotes.json"))
    check("A2 `remotes: []` is a stated fact: accepted, no remote row, the four service rows remain", rc == 0 and not any(k.startswith("remote:") for k in by_loc(load_rows(os.path.join(w, "o_emptyrem"))[0])), (rc, msg))

    # ---- B2: a changed entry at the same locator is never kept stale silently (real register DB, real triggers)
    def m_a(t):
        F.w(t, "MASTER_EXECUTION_CHECKLIST.md", "# C\n\n- [ ] alpha\n")
    ta, oa, rc, msg = case(w, "stale_a", m_a)
    db = os.path.join(w, "stale.db"); ar = subprocess.run([APPLY, "--db", db], capture_output=True, text=True)
    check("B2 scratch register DB (apply_ext.sh)", ar.returncode == 0 and os.path.exists(db), ar.stdout + ar.stderr)
    if ar.returncode == 0 and rc == 0:
        def imp(sqlp):
            r = subprocess.run(["sqlite3", db, ".read " + sqlp], capture_output=True, text=True); return r.returncode, r.stdout + r.stderr
        r1 = imp(os.path.join(oa, "source_entries.sql"))
        check("B2 first import applies cleanly", r1 == (0, ""), r1)
        r2 = imp(os.path.join(oa, "source_entries.sql"))
        check("B2 a second import of the same sql applies cleanly and changes nothing (idempotent)", r2 == (0, ""), r2)
        n1 = subprocess.run(["sqlite3", db, "SELECT count(*) FROM reg_source_entries"], capture_output=True, text=True).stdout.strip()
        tb, ob2, rcb, msgb = case(w, "stale_b", lambda t: F.w(t, "MASTER_EXECUTION_CHECKLIST.md", "# C\n\n- [ ] beta CHANGED\n- [ ] gamma NEW\n"))
        r3 = imp(os.path.join(ob2, "source_entries.sql"))
        title = subprocess.run(["sqlite3", db, "SELECT title FROM reg_source_entries WHERE locator='MASTER_EXECUTION_CHECKLIST.md:L3'"], capture_output=True, text=True).stdout.strip()
        cnt = subprocess.run(["sqlite3", db, "SELECT scanned_entry_count FROM reg_sources WHERE locator='MASTER_EXECUTION_CHECKLIST.md'"], capture_output=True, text=True).stdout.strip()
        check("B2 re-import after the line changed: REFUSED (exit != 0, the message names stale_entries_changed_at_same_locator)", r3[0] != 0 and "stale_entries_changed_at_same_locator" in r3[1], r3)
        gamma = subprocess.run(["sqlite3", db, "SELECT count(*) FROM reg_source_entries WHERE title='gamma NEW'"], capture_output=True, text=True).stdout.strip()
        check("B2 the refused import is ALL-OR-NOTHING: the old entry (alpha) and the old count remain, the NEW line of the same file (gamma) was not imported, no new row", title == "alpha" and cnt == "1" and gamma == "0" and subprocess.run(["sqlite3", db, "SELECT count(*) FROM reg_source_entries"], capture_output=True, text=True).stdout.strip() == n1, (title, cnt, gamma))

print("RESULT: fail=%d" % FAILS)
sys.exit(1 if FAILS else 0)
