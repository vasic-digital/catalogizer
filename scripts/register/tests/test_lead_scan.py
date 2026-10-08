#!/usr/bin/env python3
"""test_lead_scan.py - tests of scripts/register/lead_scan.py and lead_scan_population.py (T167, WP-20). Run through
`scripts/containers/run_pinned.sh IMG-TESTUTIL -- python3 scripts/register/tests/test_lead_scan.py` or on the host.
Env: LEAD_SCAN (script under test; mutation runs point it at a mutated copy living beside copies of its helpers).
Every scenario gets its own scratch copy, manifest, listing and freeze.json; the owner's HC-2 record lives in a scratch evidence root
given through EV (the population helper reads $EV/hc/HC-2.json, there is no option for it).
Absolute assertions (WF23 review 2026-10-08, findings C1-C7 and the author-independent mutants LM01-LM06): each vocabulary word and inflection
yields its canonical word, look-alikes yield nothing, a duplicate is decided per line and carries covered_by, the marker rule needs a shell
search command, HC-2 is read from its fixed path and a malformed record is refused, lead_scan_entry_rows is counted in the imported database.
"""
import json, os, shutil, sqlite3, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__)); REG = os.path.dirname(HERE)
sys.path.insert(0, HERE)
import wp20_fixture as F
LS = os.environ.get("LEAD_SCAN", os.path.join(REG, "lead_scan.py"))
APPLY = os.path.join(REG, "apply_ext.sh")
os.environ.setdefault("WI", os.path.join(os.path.dirname(os.path.dirname(REG)), "submodules/constitution/scripts/workable-items/bin/workable-items-linux"))
FAILS = 0
NEEDLE_LINE = "- **Fix:** add the missing test; do not skip with `t.Skip()`"      # the declared corpus needle of lead_scan.NEEDLES
SCHEMA = ("CREATE TABLE reg_sources(source_id INTEGER PRIMARY KEY, kind TEXT, locator TEXT UNIQUE, parser TEXT, content_sha256 TEXT, last_scanned_at TEXT, scanned_entry_count INT);"
          "CREATE TABLE reg_source_entries(entry_id INTEGER PRIMARY KEY, source_id INT, locator TEXT, legacy_id TEXT, title TEXT, raw_status TEXT, raw_severity TEXT, entry_sha256 TEXT, scanned_at TEXT, UNIQUE(source_id,locator));")


def check(label, cond, info=""):
    global FAILS
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:400])); FAILS += 0 if cond else 1


def mk(w, name, extra=None):
    t = os.path.join(w, name); F.build(t)
    F.w(t, "docs/LANDMINES.md", "# L\n\n| RULE-API-001 | a rule |\n" + NEEDLE_LINE + "\n")
    F.w(t, "docs/notes.md", "# Notes\n\nplain text\n")
    F.w(t, "submodules/sub1/README.md", "# Sub\n\nplain\n")
    if extra:
        extra(t)
    return t


def freeze(t, gitlinks=("submodules/sub1",)):
    fj = t + ".freeze.json"; F.freeze(t, fj); j = json.load(open(fj))
    lst = t + ".files.txt"
    open(lst, "wb").write(b"".join(os.path.relpath(os.path.join(dp, n), t).encode() + b"\0" for dp, _, fns in os.walk(t) for n in sorted(fns)))
    j["listing"] = lst; j["gitlinks"] = [{"path": g, "sha": "0" * 40} for g in gitlinks]; json.dump(j, open(fj, "w")); return fj


def scan(fj, out, ev=None, extra=()):
    env = dict(os.environ); env["EV"] = ev or os.path.join(os.path.dirname(out), "ev_none")
    r = subprocess.run([sys.executable, LS, "--freeze-json", fj, "--out", out] + list(extra), capture_output=True, text=True, env=env)
    return r.returncode, r.stdout + r.stderr


def rows(out):
    db = sqlite3.connect(":memory:"); db.executescript(SCHEMA)
    sql = "\n".join(l for l in open(os.path.join(out, "lead_scan.sql"), encoding="utf-8").read().split("\n") if not l.startswith("."))
    db.executescript(sql.replace("PRAGMA foreign_keys=ON;", ""))
    return {r[0]: r[1:] for r in db.execute("SELECT locator, legacy_id, raw_status, raw_severity, title FROM reg_source_entries")}


def runj(out):
    return json.load(open(os.path.join(out, "lead-scan-run.json")))


def hc2(ev, body, raw=False):
    os.makedirs(os.path.join(ev, "hc"), exist_ok=True)
    open(os.path.join(ev, "hc", "HC-2.json"), "w").write(body if raw else json.dumps(body))


with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as w:
    if not os.path.exists(LS):
        check("lead_scan.py exists (RED: T167 not implemented)", False, LS); print("RESULT: fail=%d" % FAILS); sys.exit(1)
    base = mk(w, "base"); fj0 = freeze(base); o0 = os.path.join(w, "o0")
    rc, out = scan(fj0, o0); check("baseline scan exits 0", rc == 0, out)
    R0 = rows(o0) if rc == 0 else {}
    run0 = runj(o0) if rc == 0 else {}
    check("control needle found with its word and its disposition (lead: no structured entry covers that line)",
          run0.get("needles") == ["docs/LANDMINES.md:L4"] and list(R0.get("docs/LANDMINES.md:L4", [None, None])[:2]) == ["missing", "lead"], (run0.get("needles"), R0.get("docs/LANDMINES.md:L4")))
    check("population excludes the gitlink Markdown (submodules/sub1/README.md not scanned)", not any(k.startswith("submodules/") for k in R0))
    check("the sql, its .sql.sha256 (T165 name) and its .sha256 (the sibling locked.sh import-sql reads) are written and equal",
          open(os.path.join(o0, "lead_scan.sql.sha256")).read() == open(os.path.join(o0, "lead_scan.sha256")).read() and len(open(os.path.join(o0, "lead_scan.sha256")).read()) == 65)
    # ---- C1 vocabulary: each word and inflection yields its canonical word, look-alikes yield nothing (LM01, LM06)
    POS = [("unfinished", "this part is unfinished"), ("not implemented", "it is not implemented"), ("not implemented", "it is not-implemented"), ("stub", "a stub only"),
           ("stub", "two stubs here"), ("stub", "was stubbed out"), ("stub", "stubbing the call"), ("missing", "the file is missing"), ("broken", "a broken link"),
           ("known issue", "a known issue"), ("known issue", "## 10. Known Issues"), ("workaround", "use a workaround"), ("workaround", "two workarounds"),
           ("deprecated", "a deprecated API"), ("disabled", "a disabled test"), ("skipped", "a skipped test"), ("todo", "TODO write it"), ("todo", "many todos left"),
           ("fixme", "FIXME later"), ("fixme", "the fixmes"), ("regress", "a regression"), ("regress", "it regressed"), ("outstanding", "an outstanding item"),
           ("pending", "pending review"), ("not yet", "not yet done"), ("not yet", "not-yet done")]
    NEG = ["depending on the host", "a stubborn problem", "unstubbed", "the todolist app", "missingno", "suspending", "prepending", "no lead words here"]
    t1 = mk(w, "vocab", lambda t: F.w(t, "docs/vocab.md", "# V\n\n" + "\n".join(l for _, l in POS) + "\n\n" + "\n".join(NEG) + "\n"))
    o1 = os.path.join(w, "o1"); rc, out = scan(freeze(t1), o1); R1 = rows(o1) if rc == 0 else {}
    got = {k: v[0] for k, v in R1.items() if k.startswith("docs/vocab.md:L")}
    want = {"docs/vocab.md:L%d" % (3 + i): word for i, (word, _l) in enumerate(POS)}
    check("C1 every vocabulary word and inflection is a lead with its canonical word; every look-alike is not a lead", got == want, {k: (got.get(k), want.get(k)) for k in set(got) ^ set(want) | {k for k in want if got.get(k) != want[k]}})
    # ---- planted line per main-repo md / submodule md (needle) and the row count
    t2 = mk(w, "p_main", lambda t: F.w(t, "docs/notes.md", "- this is not implemented yet\n", "a")); o2 = os.path.join(w, "o2")
    rc, out = scan(freeze(t2), o2); R2 = rows(o2)
    check("a planted lead line in a main-repository .md yields a lead row with its canonical word", list(R2.get("docs/notes.md:L4", [None, None])[:2]) == ["not implemented", "lead"], R2.get("docs/notes.md:L4"))
    t3 = mk(w, "p_sub", lambda t: F.w(t, "submodules/sub1/README.md", "- this is not implemented yet\n", "a")); o3 = os.path.join(w, "o3")
    rc, out = scan(freeze(t3), o3); R3 = rows(o3)
    check("the same line in a submodule .md yields no row", rc == 0 and not any(k.startswith("submodules/") for k in R3), R3)
    check("emitted_rows is exactly one higher for the planted copy", runj(o2)["emitted_rows"] == run0["emitted_rows"] + 1 and runj(o2)["emitted_rows"] == len(R2), (runj(o2)["emitted_rows"], run0["emitted_rows"], len(R2)))
    # ---- C2 the duplicate disposition is decided per line, and names the covering structured entry
    def m_dup(t):
        F.w(t, "MASTER_EXECUTION_CHECKLIST.md", "# C\n\n- [ ] one is not implemented\n\nProse in a structured file: this is a workaround.\n")
        F.w(t, "docs/issues/HELIX-001-a.md", F.ticket(1) + "\nThis ticket mentions a broken thing.\n")
        F.w(t, ".specify/memory/constitution.md", "# C\n\n## Known Conflicts and Open Decisions\n\n1. **One.** DECIDED here.\n   A continuation line with a stub in it.\n\n## Next\n\nA stub after the section.\n")
    t4 = mk(w, "dup", m_dup); o4 = os.path.join(w, "o4"); rc, out = scan(freeze(t4), o4); R4 = rows(o4)
    dup = "duplicate of structured source"
    check("C2 a lead line that a structured entry sits on is a duplicate and covered_by names that entry", tuple(R4.get("MASTER_EXECUTION_CHECKLIST.md:L3", [0, 0, 0, 0])[1:3]) == (dup, "covered_by=MASTER_EXECUTION_CHECKLIST.md :: MASTER_EXECUTION_CHECKLIST.md:L3"), R4.get("MASTER_EXECUTION_CHECKLIST.md:L3"))
    check("C2 a lead line of the same structured file that NO structured entry covers is a lead (not a duplicate)", tuple(R4.get("MASTER_EXECUTION_CHECKLIST.md:L5", [0, 0, 0])[1:3]) == ("lead", None), R4.get("MASTER_EXECUTION_CHECKLIST.md:L5"))
    check("C2 a lead line inside a ticket file is a duplicate of the whole-file entry", any(v[1] == dup and "HELIX-001-a.md" in (v[2] or "") for k, v in R4.items() if k.startswith("docs/issues/HELIX-001-a.md:L")), [(k, v[1:3]) for k, v in R4.items() if "HELIX-001-a" in k])
    check("C2 a continuation line inside an S-21 item block is covered by the item; a line after the section is a lead",
          any(v[1] == dup and v[2].endswith(":L5#item1") for k, v in R4.items() if k.startswith(".specify/memory/constitution.md:L6")) and
          any(v[1] == "lead" for k, v in R4.items() if k.startswith(".specify/memory/constitution.md:L10")),
          [(k, v[1:3]) for k, v in R4.items() if k.startswith(".specify")])
    # ---- C6 the marker rule needs a shell search command; mixed lines and fence handling
    def m_marker(t):
        F.w(t, "docs/notes.md", "# N\n\n```\ngrep -rn 'TODO|FIXME' src\n$ git grep TODO\nTODO plain text in a fence\n```\n"
                                 "grep -rn TODO src/ | sed 's/x/y/' # also a workaround\n"
                                 "- TODO: write the docs\n"
                                 "the TODO is found with grep\n"
                                 "sed -n '/FIXME/p' f\n")
    t5 = mk(w, "marker", m_marker); o5 = os.path.join(w, "o5"); rc, out = scan(freeze(t5), o5); R5 = rows(o5)
    np = "NON-PROBLEM:marker text in document or pattern"
    g = lambda n: R5.get("docs/notes.md:L%d" % n, [None, None, None])[1]
    check("C6 a marker word as the argument of grep / git grep / sed is NON-PROBLEM", (g(4), g(5), g(11)) == (np, np, np), (g(4), g(5), g(11)))
    check("C6 a marker word in a fence WITHOUT a command is a lead", g(6) == "lead", g(6))
    check("C6 a search command line that also holds another lead word (workaround) is a lead, not suppressed", g(8) == "lead", g(8))
    check("C6 a bare `- TODO: ...` line (no search command) is a lead (LM03: the command is required)", g(9) == "lead", g(9))
    check("C6 a command word AFTER the marker word does not suppress", g(10) == "lead", g(10))
    # ---- C4 HC-2: fixed path, no option, malformed refused
    ev = os.path.join(w, "ev"); rc, out = scan(freeze(t3), os.path.join(w, "o_noop"), ev=ev)
    check("C4 no HC-2 record: no widening (the submodule line is not scanned)", rc == 0 and not any(k.startswith("submodules/") for k in rows(os.path.join(w, "o_noop"))) and runj(os.path.join(w, "o_noop"))["extra_roots"] == [], out)
    hc2(ev, {"lead_scan_scope": "amended", "lead_scan_extra_roots": ["submodules/sub1"]})
    o6 = os.path.join(w, "o6"); rc, out = scan(freeze(t3), o6, ev=ev); R6 = rows(o6) if rc == 0 else {}
    check("C4 HC-2 extra root naming the gitlink (read from $EV/hc/HC-2.json, no option) brings the submodule line into scope", R6.get("submodules/sub1/README.md:L4", [0, 0])[1] == "lead" and runj(o6)["extra_roots"] == ["submodules/sub1"] and runj(o6)["hc2_path"].endswith("/hc/HC-2.json"), (rc, out))
    rc, out = scan(freeze(t3), os.path.join(w, "o6b"), ev=ev, extra=["--hc2", "/etc/passwd"])
    check("C4 there is no --hc2 option: usage, exit 2", rc == 2 and not os.path.exists(os.path.join(w, "o6b")), (rc, out))
    for label, body, raw in (("docs", {"lead_scan_extra_roots": ["docs"]}, False), ):
        hc2(ev, body, raw); o = os.path.join(w, "o7"); shutil.rmtree(o, ignore_errors=True); rc, out = scan(freeze(t3), o, ev=ev)
        check("C4 an extra root that is no gitlink path is refused lead_scan_extra_root_invalid naming it", rc == 20 and "lead_scan_extra_root_invalid" in out and "docs" in out and not os.path.exists(o), (rc, out))
    for label, body in (("not JSON", "{broken"), ("a JSON list", "[1,2]"), ("a string value", '{"lead_scan_extra_roots": "submodules/sub1"}'), ("a non-string element", '{"lead_scan_extra_roots": [1]}')):
        hc2(ev, body, raw=True); o = os.path.join(w, "o8"); shutil.rmtree(o, ignore_errors=True); rc, out = scan(freeze(t3), o, ev=ev)
        check("C4 a malformed HC-2 record (%s) is refused hc2_malformed, nothing written" % label, rc == 20 and "hc2_malformed" in out and not os.path.exists(o), (rc, out))
    shutil.rmtree(ev)
    # ---- C5 the record: exact command, rows counted in the imported database before and after
    check("C5 the run record holds the exact command line (interpreter, absolute script path, arguments)", run0["command"][0] == sys.executable and os.path.isabs(run0["command"][1]) and run0["command"][2:6] == ["--freeze-json", fj0, "--out", o0], run0.get("command"))
    db = os.path.join(w, "s.db"); ar = subprocess.run([APPLY, "--db", db], capture_output=True, text=True)
    check("C5 scratch register DB", ar.returncode == 0, ar.stdout + ar.stderr)
    if ar.returncode == 0:
        def cnt():
            return int(subprocess.run(["sqlite3", db, "SELECT count(*) FROM reg_source_entries"], capture_output=True, text=True).stdout.strip())
        for i in (1, 2):
            before = cnt(); r = subprocess.run(["sqlite3", db, ".read " + os.path.join(o2, "lead_scan.sql")], capture_output=True, text=True)
            check("C5 import %d of lead_scan.sql applies cleanly" % i, r.returncode == 0 and r.stdout + r.stderr == "", r.stderr)
            rj = os.path.join(w, "run%d.json" % i); shutil.copy(os.path.join(o2, "lead-scan-run.json"), rj)
            pr = subprocess.run([sys.executable, LS, "--finalize", rj, "--db", db, "--rows-before", str(before)], capture_output=True, text=True)
            fin = json.load(open(rj))
            check("C5 import %d: lead_scan_entry_rows counted in the DB = %s (emitted %d)" % (i, fin["lead_scan_entry_rows"], fin["emitted_rows"]),
                  pr.returncode == 0 and fin["lead_scan_entry_rows"] == (len(R2) if i == 1 else 0) and fin["emitted_rows"] == len(R2) and fin["rows_after"] - fin["rows_before"] == fin["lead_scan_entry_rows"], (pr.stderr, fin))
        rc = subprocess.run([sys.executable, LS, "--finalize", os.path.join(w, "run1.json"), "--db", os.path.join(w, "no_such.db"), "--rows-before", "0"], capture_output=True, text=True)
        check("C5 finalize on an unreadable database is refused finalize_failed (exit 20), never a recorded 0", rc.returncode == 20 and "finalize_failed" in rc.stderr, (rc.returncode, rc.stderr))
        # B2 for the lead scan: a changed line at the same locator is refused, the old row stays
        tc = mk(w, "p_changed", lambda t: F.w(t, "docs/notes.md", "- this is a workaround CHANGED\n- and a brand new pending line\n", "a")); oc = os.path.join(w, "oc")
        tb = mk(w, "p_changed0", lambda t: F.w(t, "docs/notes.md", "- this is a workaround\n", "a")); ob = os.path.join(w, "ob")
        scan(freeze(tb), ob); scan(freeze(tc), oc)
        d2 = os.path.join(w, "s2.db"); subprocess.run([APPLY, "--db", d2], capture_output=True)
        r0 = subprocess.run(["sqlite3", d2, ".read " + os.path.join(ob, "lead_scan.sql")], capture_output=True, text=True)
        r1 = subprocess.run(["sqlite3", d2, ".read " + os.path.join(oc, "lead_scan.sql")], capture_output=True, text=True)
        title = subprocess.run(["sqlite3", d2, "SELECT title FROM reg_source_entries WHERE locator='docs/notes.md:L4'"], capture_output=True, text=True).stdout.strip()
        newrow = subprocess.run(["sqlite3", d2, "SELECT count(*) FROM reg_source_entries WHERE locator='docs/notes.md:L5'"], capture_output=True, text=True).stdout.strip()
        check("B2 lead scan: a re-import after a line changed is refused (stale_entries_changed_at_same_locator), ALL-OR-NOTHING: the old row is kept and the new line of the same file was not imported",
              r0.returncode == 0 and r1.returncode != 0 and "stale_entries_changed_at_same_locator" in r1.stderr and title == "- this is a workaround" and newrow == "0", (r0.returncode, r1.returncode, r1.stderr, title, newrow))
    # ---- C7 symlinked population files are recorded
    def m_link(t):
        os.symlink("notes.md", os.path.join(t, "docs/link.md"))
    t9 = mk(w, "link", m_link); o9 = os.path.join(w, "o9"); rc, out = scan(freeze(t9), o9)
    check("C7 a symlinked population file is skipped AND recorded in the run record", rc == 0 and runj(o9)["symlink_population_files_skipped"] == ["docs/link.md"], (rc, out, runj(o9) if rc == 0 else None))
    # ---- needles: the declared line missing from the file is a refusal; marker/pattern lines elsewhere do not matter
    t10 = mk(w, "noneedle", lambda t: F.w(t, "docs/LANDMINES.md", "# L\n\n| RULE-API-001 | a workaround |\n")); o10 = os.path.join(w, "o10"); rc, out = scan(freeze(t10), o10)
    check("a population holding docs/LANDMINES.md without the declared needle line is refused lead_scan_needle_missing", rc == 20 and "lead_scan_needle_missing" in out and not os.path.exists(o10), (rc, out))
    t11 = mk(w, "covered_needle", lambda t: F.w(t, "docs/LANDMINES.md", "# L\n\n| RULE-API-001 | " + NEEDLE_LINE.lstrip("- ") + " |\n")); o11 = os.path.join(w, "o11"); rc, out = scan(freeze(t11), o11)
    check("a needle line that a structured entry covers (disposition duplicate, expected lead) is refused lead_scan_blind", rc == 20 and "lead_scan_blind" in out, (rc, out))
    # ---- moved snapshot / moved listing
    t6 = mk(w, "p_moved"); fj6 = freeze(t6); open(os.path.join(t6, "docs/notes.md"), "a").write("changed after the manifest\n")
    o6m = os.path.join(w, "o6m"); rc, out = scan(fj6, o6m)
    check("snapshot changed after its manifest: freeze_snapshot_moved, no /out file", rc == 20 and "freeze_snapshot_moved" in out and "docs/notes.md" in out and not os.path.exists(o6m), (rc, out))
    t7 = mk(w, "p_list"); fj7 = freeze(t7); j = json.load(open(fj7)); open(j["listing"], "ab").write(b"docs/ghost.md\0")
    o7m = os.path.join(w, "o7m"); rc, out = scan(fj7, o7m)
    check("a NUL path appended to the listing: freeze_listing_moved, no /out file", rc == 20 and "freeze_listing_moved" in out and not os.path.exists(o7m), (rc, out))
    # ---- the vocabulary control is WIRED into the run: a copy of the tool with a word missing from the vocabulary refuses lead_scan_blind
    cp = os.path.join(w, "toolcopy"); os.makedirs(cp)
    for fn in ("lead_scan.py", "lead_scan_population.py", "enumerate_sources.py", "snapshot_manifest.py", "check_freeze_snapshot.sh"):
        shutil.copy(os.path.join(os.path.dirname(LS), fn), cp)
    txt = open(os.path.join(cp, "lead_scan.py")).read()
    broken = txt.replace('(r"known[ -]issues?", "known issue"), ', "")
    check("(test setup) the copy differs from the tool by exactly the dropped vocabulary word", broken != txt)
    open(os.path.join(cp, "lead_scan.py"), "w").write(broken)
    ob = os.path.join(w, "o_vocab_broken"); rcb = subprocess.run([sys.executable, os.path.join(cp, "lead_scan.py"), "--freeze-json", fj0, "--out", ob], capture_output=True, text=True, env=dict(os.environ, EV=os.path.join(w, "ev_none")))
    check("a tool whose vocabulary lost a word is refused lead_scan_blind by its own control before the real scan (nothing written)", rcb.returncode == 20 and "lead_scan_blind" in rcb.stderr and "vocabulary control failed" in rcb.stderr and not os.path.exists(ob), (rcb.returncode, rcb.stderr))
    # ---- the built-in vocabulary control is part of the run: the module's own control set passes
    import importlib.util
    sp = importlib.util.spec_from_file_location("ls_under_test", LS); lsm = importlib.util.module_from_spec(sp); sp.loader.exec_module(lsm)
    check("the built-in vocabulary control set passes (instrument sees every word and nothing else)", lsm.vocabulary_control() == [], lsm.vocabulary_control())
print("RESULT: fail=%d" % FAILS)
sys.exit(1 if FAILS else 0)
