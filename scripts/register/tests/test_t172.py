#!/usr/bin/env python3
"""test_t172.py - T172 (WP-20): doc12 WP-Q5 ticket specifics inside the import: QA-instrument cause items (docs/12 section 11.3, QF-10), platform inference and
duplicate families (docs/12 section 11.4, doc04 section 8) by scripts/register/import_t172.sh. Real SQLite (apply_ext.sh), the real engine binary, scratch databases
and scratch snapshots only; the real register is never written.
Env: IMPORT_T172 (tool under test; mutation runs point it at a mutated copy that sits beside copies of import_tickets.py, import_plan.py and enumerate_sources.py),
IMPORT_TICKETS (T168 importer, default the one beside the tool), ENUMERATE_SOURCES, WI (engine), SCRATCH_BASE.
Checks (each is a failure class of the task text, T172):
  (a) the entry-set invariant (the RED of T172): the sorted triple export of reg_source_entries is byte-identical before and after each stage; an import that inserts one
      row, and one that inserts one row and deletes another (the count unchanged), are each refused `t172_entry_set_changed` naming every differing triple
  (b) stage qa: the closed-class tickets that say the QA tool was wrong are grouped by the stated cause into ONE Bug item per cause (component helixqa / qa-scripts, Queued,
      category bug), every ticket linked in reg_source_map (first locator primary, the others duplicate_of, one severity_governs row), no new entry, no per-ticket item after import.sh
  (c) classification limits: an open ticket with the QA statement, a closed ticket without it, and an unlisted cause are not mis-grouped; refusals write nothing
  (d) stage families: platform inference (declared wins, one evidence class inferred and marked [DEFAULT - adjustable], two classes or none UNKNOWN, never guessed),
      duplicate families by (normalised title slug, platform class, screen) with UNDECIDED mint-with-link rows, distinct same-slug groups never linked
  (e) idempotent and resumable (a crash after each stage, then a re-run, gives the state of a clean run); a second run writes nothing
  (f) refusals with nothing written: snapshot changed after its manifest, entries older than the snapshot, families before the import, QA entries already imported one by one,
      a cause whose component is not a reg_components row, the real register database name without a backup record
  (g) HELIX-NNN is a non-unique legacy label: entries are keyed by path (v_legacy_id_collisions lists the shared ids, every locator is distinct)
Exit 0 only when every check passed.
"""
import hashlib
import json
import os
import re
import shutil
import sqlite3
import subprocess
import sys
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(REG))
sys.path.insert(0, HERE)
import w2_fixture as F   # noqa: E402
import wp20_fixture as W   # noqa: E402

IMP = os.environ.get("IMPORT_T172", os.path.join(REG, "import_t172.sh"))
IMPT = os.environ.get("IMPORT_TICKETS", os.path.join(os.path.dirname(IMP), "import.sh"))
ENUM = os.environ.get("ENUMERATE_SOURCES", os.path.join(os.path.dirname(IMP), "enumerate_sources.py"))
WI = os.environ.get("WI", os.path.join(ROOT, "constitution/scripts/workable-items/bin/workable-items-linux"))
PLAN_SRC = "plan-document seeds (docs/21 section 9.1)"
INNO_SRC = "innovation entries (docs/21 section 9.5)"
FAILS = PASSES = 0


def check(label, cond, info=""):
    global FAILS, PASSES
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:900]))
    if cond:
        PASSES += 1
    else:
        FAILS += 1
        if os.environ.get("W2_FAILFAST") == "1":     # mutation runs: the first FAIL kills the mutant, the rest of the suite is not needed
            print("RESULT: pass=%d fail=%d (stopped at the first failure: W2_FAILFAST=1)" % (PASSES, FAILS))
            sys.exit(1)


def sh(cmd, **kw):
    return subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kw)


def sha(p):
    return hashlib.sha256(open(p, "rb").read()).hexdigest()


inc = os.path.exists("/run/.containerenv") or os.environ.get("container") == "podman"
print("# identity: task=T172 utc=%s uid=%d git_head=%s" % (time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), os.getuid(), sh(["git", "-C", ROOT, "rev-parse", "HEAD"]).stdout.decode().strip()))
print("# container: %s" % ("in-container run (container=%s) image=IMG-TESTUTIL" % os.environ.get("container", "unset") if inc else "host-side run (no container marker); a container leg is UNCONFIRMED for this transcript"))
print("# importer=%s sha256=%s enumerator sha256=%s engine sha256=%s" % (IMP, sha(IMP)[:16] if os.path.isfile(IMP) else "ABSENT", sha(ENUM)[:16] if os.path.isfile(ENUM) else "ABSENT", sha(WI)[:16]))
if not os.path.isfile(IMP):
    check("importer present: %s (RED: T172 not implemented)" % IMP, False)
    print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
    sys.exit(1)

SCR = tempfile.mkdtemp(prefix="seedimp.", dir=os.environ.get("SCRATCH_BASE") or os.environ.get("TMPDIR"))
DOC21 = F.DOCS + "/21-master-plan-phases-risks-and-traceability.md"


def mktree(name, edit=None):
    t = os.path.join(SCR, name)
    W.build(t)
    F.plan_docs(t)
    if edit:
        edit(t)
    return t


def enumerate_tree(tree, name):
    fj = os.path.join(SCR, name + ".freeze.json")
    F.freeze2(tree, fj)
    out = os.path.join(SCR, name + ".enum")
    r = sh([sys.executable, "-I", ENUM, "--freeze-json", fj, "--out", out])
    return fj, out, r


def fresh_db(name):
    db = os.path.join(SCR, name + ".db")
    for p in (db, db + "-wal", db + "-shm"):
        if os.path.exists(p):
            os.remove(p)
    r = sh([os.path.join(REG, "apply_ext.sh"), "--db", db])
    assert r.returncode == 0, r.stdout + r.stderr
    return db


def load_sql(db, out):
    r = sh(["sqlite3", db, ".read " + os.path.join(out, "source_entries.sql")])
    assert r.returncode == 0 and not (r.stdout + r.stderr), r.stderr
    return db


def q(db, sql, *a):
    c = sqlite3.connect(db)
    try:
        return c.execute(sql, a).fetchall()
    finally:
        c.close()


def imp(db, fj, stage="qa", out=None, *extra, env=None):
    e = dict(os.environ)
    e.update(env or {})
    out = out or (db + "." + stage + ".out")
    return sh([IMP, "--stage", stage, "--db", db, "--freeze-json", fj, "--engine", WI, "--out-dir", out, "--report", db + "." + stage + ".run.json"] + list(extra), env=e)


def imp168(db, fj, *extra, env=None):
    e = dict(os.environ)
    e.update(env or {})
    return sh([IMPT, "--db", db, "--freeze-json", fj, "--engine", WI] + list(extra), env=e)


def state(db):
    """a canonical dump of everything the stages wrote (ids are minted in order, so a resumed and a clean run must agree)"""
    c = sqlite3.connect(db)
    try:
        return json.dumps({
            "items": c.execute("SELECT atm_id, type, status, severity, title, description FROM items ORDER BY atm_id").fetchall(),
            "ext": c.execute("SELECT atm_id, category, defect_layer, severity, severity_source, custody_basis, reverify_required, component_id FROM reg_item_ext ORDER BY atm_id").fetchall(),
            "map": c.execute("SELECT e.locator, m.atm_id, m.relation, m.match_basis, m.severity_governs, m.mapped_by FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) ORDER BY e.locator").fetchall(),
            "links": c.execute("SELECT head_atm_id, reported_entry_id, intake_path, verdict, match_basis, new_atm_id, reopened, decided_by FROM reg_recurrence_links ORDER BY link_id").fetchall(),
            "ids": c.execute("SELECT atm_id, minted_by, mint_basis FROM reg_ids ORDER BY seq").fetchall()}, sort_keys=True)
    finally:
        c.close()




COMPS = ("helixqa", "qa-scripts")


def mktree(name, edit=None):
    t = os.path.join(SCR, name)
    W.build(t)
    F.tickets_qa(t)
    # same title and screen, different declared platform class: DISTINCT (the platform class is part of the family key)
    W.w(t, *F.tk(540, "overlap-a", "Overlapping labels on the settings page", status="fixed", platform="androidtv", screen="shared-001.png"))
    W.w(t, *F.tk(541, "overlap-b", "Overlapping labels on the settings page", status="fixed", platform="desktop", screen="shared-001.png"))
    if edit:
        edit(t)
    return t


def setup(name, edit=None, components=COMPS):
    t = mktree(name, edit)
    fj, out, r = enumerate_tree(t, name)
    assert r.returncode == 0, r.stdout + r.stderr
    db = load_sql(fresh_db(name), out)
    c = sqlite3.connect(db)
    for comp in components:
        c.execute("INSERT INTO reg_components(component_id,kind,path_root,own_repo) VALUES (?,'tooling',?,0)", (comp, "tools/" + comp))
    c.commit()
    c.close()
    return t, fj, db


def read_tsv(p):
    return open(p, encoding="utf-8").read()


def loc(n, slug):
    return "docs/issues/HELIX-%03d-%s.md" % (n, slug)


QA = {501: "tv-focus-step-1", 502: "tv-focus-step-2", 503: "playback-wrong-screen", 504: "home-key", 505: "black", 506: "emulator-restart"}
CAUSE_OF = {501: "screenshot_timing", 502: "screenshot_timing", 503: "wrong_screen", 504: "wrong_adb", 505: "black_frame", 506: "other_qa_instrument"}
COMP_OF = {"screenshot_timing": "helixqa", "wrong_screen": "helixqa", "black_frame": "helixqa", "wrong_adb": "qa-scripts", "other_qa_instrument": "helixqa"}

# ---- (b) stage qa on the fixture (RED first: the tool is absent in the RED run, then each check below)
def _sev502(t):
    pth = os.path.join(t, loc(502, QA[502]))
    s_ = open(pth).read()
    assert "severity: high" in s_
    open(pth, "w").write(s_.replace("severity: high", "severity: critical", 1))


T1, fj1, db1 = setup("main", _sev502)
n_entries0 = q(db1, "SELECT count(*) FROM reg_source_entries")[0][0]
n_tick = q(db1, "SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.kind='issue_file'")[0][0]
r = imp(db1, fj1, "qa", os.path.join(SCR, "main.qa"))
check("(b) stage qa exits 0", r.returncode == 0, r.stdout + r.stderr)
rec = json.load(open(os.path.join(SCR, "main.qa", "qa_causes.json"))) if os.path.exists(os.path.join(SCR, "main.qa", "qa_causes.json")) else {}
items = q(db1, "SELECT i.atm_id, i.type, i.status, x.component_id, x.category, x.custody_basis, x.reverify_required, i.title, i.description FROM items i JOIN reg_item_ext x USING(atm_id) ORDER BY i.atm_id")
check("(b) exactly 5 items: one Bug per stated cause (screenshot timing, wrong screen, black frame, wrong ADB, other QA statement), all Queued, category bug, machine_evidence",
      len(items) == 5 and {x[1] for x in items} == {"Bug"} and {x[2] for x in items} == {"Queued"} and {x[4] for x in items} == {"bug"} and {x[5] for x in items} == {"machine_evidence"}
      and {x[6] for x in items} == {0}, [(x[0], x[1], x[2]) for x in items])
bycause = {k: v["atm_id"] for k, v in rec.get("causes", {}).items()}
check("(b) the record names the five causes and the component of each (helixqa, qa-scripts for the wrong ADB command)",
      set(bycause) == set(COMP_OF) and all(rec["causes"][k]["component"] == COMP_OF[k] for k in COMP_OF)
      and all(q(db1, "SELECT component_id FROM reg_item_ext WHERE atm_id=?", bycause[k])[0][0] == COMP_OF[k] for k in COMP_OF), rec.get("causes"))
mp = {r_[0]: (r_[1], r_[2], r_[3], r_[4]) for r_ in q(db1, "SELECT e.locator, m.atm_id, m.relation, m.match_basis, m.severity_governs FROM reg_source_map m JOIN reg_source_entries e USING(entry_id)")}
check("(b) every QA ticket is a source entry of ITS cause item in reg_source_map (6 tickets, 6 rows, the right item each)",
      len(mp) == 6 and all(mp[loc(n, s)][0] == bycause[CAUSE_OF[n]] for n, s in QA.items()), mp)
check("(b) the two screenshot-timing tickets: the first locator is primary, the second duplicate_of; all manual_operator (the plan's own mapping)",
      mp[loc(501, QA[501])][1] == "primary" and mp[loc(502, QA[502])][1] == "duplicate_of" and {v[2] for v in mp.values()} == {"manual_operator"}, mp)
gov = q(db1, "SELECT e.locator FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE m.atm_id=? AND m.severity_governs=1", bycause["screenshot_timing"])
check("(b) the cause item takes the HIGHEST ticket severity (HELIX-502 critical beside HELIX-501 high) and that ticket governs, while the first locator stays primary",
      q(db1, "SELECT severity FROM reg_item_ext WHERE atm_id=?", bycause["screenshot_timing"])[0][0] == "critical" and gov == [(loc(502, QA[502]),)]
      and mp[loc(501, QA[501])][1] == "primary", (gov, q(db1, "SELECT severity FROM reg_item_ext WHERE atm_id=?", bycause["screenshot_timing"])))
check("(b) exactly one severity_governs=1 row per cause item", q(db1, "SELECT count(*) FROM (SELECT atm_id, sum(severity_governs) s FROM reg_source_map GROUP BY atm_id) WHERE s <> 1")[0][0] == 0)
check("(b) no reg_source_entries row was inserted: the entry count is unchanged and the before/after triple exports are byte-identical",
      q(db1, "SELECT count(*) FROM reg_source_entries")[0][0] == n_entries0 and read_tsv(os.path.join(SCR, "main.qa", "entryset-before.tsv")) == read_tsv(os.path.join(SCR, "main.qa", "entryset-after.tsv"))
      and json.load(open(os.path.join(SCR, "main.qa", "entryset.json")))["identical"] is True and n_entries0 > 6)
check("(b) every description is within 2,048 bytes and carries the stated cause, the component, the fix with a RED case and a Sources block",
      all(len(x[8].encode("utf-8")) <= 2048 and "**Sources**" in x[8] and "deterministic capture wait" in x[8] and "RED case" in x[8] and "Cause: " in x[8] and "Component: " in x[8] for x in items))
check("(b) the engine validate, foreign_key_check and integrity_check are clean", sh([WI, "validate", "--db", db1]).returncode == 0 and q(db1, "PRAGMA foreign_key_check") == [] and q(db1, "PRAGMA integrity_check") == [("ok",)])
# the T168 import then skips the six entries (an entry already in reg_source_map is skipped)
r = imp168(db1, fj1)
check("(b) import.sh (T168) after stage qa exits 0 and mints no per-ticket item for the six QA tickets",
      r.returncode == 0 and q(db1, "SELECT count(*) FROM items")[0][0] == 5 + (n_tick - 6) and all(q(db1, "SELECT count(*) FROM reg_source_map WHERE entry_id=(SELECT entry_id FROM reg_source_entries WHERE locator=?) AND atm_id=?", loc(n, s), bycause[CAUSE_OF[n]])[0][0] == 1 for n, s in QA.items()),
      (r.returncode, r.stderr[-300:], q(db1, "SELECT count(*) FROM items")[0][0], n_tick))
check("(b) after both importers every entry is mapped (v_unmapped_entries is empty) and no QA ticket has a second item", q(db1, "SELECT count(*) FROM v_unmapped_entries WHERE locator LIKE 'docs/issues/%'")[0][0] == 0)

# ---- (g) HELIX-NNN is a non-unique legacy label, entries are keyed by path
coll = q(db1, "SELECT legacy_id, entries FROM v_legacy_id_collisions")
check("(g) HELIX-001 (two files in the fixture) is listed by v_legacy_id_collisions, and every entry locator is distinct (keyed by path)",
      ("HELIX-001", 2) in coll and q(db1, "SELECT count(*) FROM (SELECT locator FROM reg_source_entries GROUP BY source_id, locator HAVING count(*) > 1)")[0][0] == 0, coll)

# ---- (d) stage families
rf = imp(db1, fj1, "families", os.path.join(SCR, "main.fam"))
check("(d) stage families exits 0 after import.sh", rf.returncode == 0, rf.stdout + rf.stderr)
fam = json.load(open(os.path.join(SCR, "main.fam", "families.json"))) if os.path.exists(os.path.join(SCR, "main.fam", "families.json")) else {}
pi = json.load(open(os.path.join(SCR, "main.fam", "platform_inference.json"))) if os.path.exists(os.path.join(SCR, "main.fam", "platform_inference.json")) else {}
prow = {r_["locator"]: r_ for r_ in pi.get("rows", [])}
check("(d) a declared platform wins over its evidence (HELIX-525: declared desktop, screen androidtv-001)", prow[loc(525, "declared")]["platform"] == "desktop" and prow[loc(525, "declared")]["basis"] == "declared")
check("(d) one evidence class is inferred and marked [DEFAULT - adjustable]: HELIX-520 web (screen web-curiosity), HELIX-521 androidtv, HELIX-522 api (URL in the body)",
      [(prow[loc(n, s)]["platform"], prow[loc(n, s)]["basis"], prow[loc(n, s)]["marker"]) for n, s in ((520, "contrast-web"), (521, "contrast-tv"), (522, "api-timeout"))]
      == [("web", "inferred", "[DEFAULT - adjustable]"), ("androidtv", "inferred", "[DEFAULT - adjustable]"), ("api", "inferred", "[DEFAULT - adjustable]")], [prow[loc(n, s)] for n, s in ((520, "contrast-web"), (521, "contrast-tv"), (522, "api-timeout"))])
check("(d) two evidence classes (HELIX-523: androidtv screen, web in the body) -> UNKNOWN conflict, no evidence (HELIX-524) -> UNKNOWN, never guessed",
      prow[loc(523, "mixed-evidence")]["platform"] == "UNKNOWN" and prow[loc(523, "mixed-evidence")]["basis"] == "UNKNOWN_conflict"
      and prow[loc(524, "no-evidence")]["platform"] == "UNKNOWN" and prow[loc(524, "no-evidence")]["basis"] == "UNKNOWN_no_evidence", (prow[loc(523, "mixed-evidence")], prow[loc(524, "no-evidence")]))
check("(d) the QA tickets are not in the families stage (they live in their cause items)", not any(loc(n, s) in prow for n, s in QA.items()))
d520 = q(db1, "SELECT i.description FROM items i JOIN reg_source_map m USING(atm_id) JOIN reg_source_entries e USING(entry_id) WHERE e.locator=?", loc(520, "contrast-web"))[0][0]
d523 = q(db1, "SELECT i.description FROM items i JOIN reg_source_map m USING(atm_id) JOIN reg_source_entries e USING(entry_id) WHERE e.locator=?", loc(523, "mixed-evidence"))[0][0]
check("(d) the inferred platform is in the item description with the marker and its evidence; an UNKNOWN platform adds no line; descriptions stay within 2,048 bytes",
      "Platform (inferred" in d520 and "[DEFAULT - adjustable]" in d520 and "web-curiosity-006.png" in d520 and "Platform (inferred" not in d523
      and all(len(x[0].encode("utf-8")) <= 2048 for x in q(db1, "SELECT description FROM items")), d520[-400:])
fm = {(f_["slug"], f_["platform_class"], f_["screen"]): f_ for f_ in fam.get("families", [])}
k_login = ("excessive-white-space-on-login-page", "androidtv", "androidtv-001-loginform.png")
check("(d) exactly one family: HELIX-510 and HELIX-511 (same normalised title slug, platform class and screen), head = the lowest locator", list(fm) == [k_login]
      and fm[k_login]["head"] == loc(510, "login-spacing") and [m_["locator"] for m_ in fm[k_login]["members"]] == [loc(511, "login-spacing-again")], list(fm))
lk = q(db1, "SELECT l.head_atm_id, l.verdict, l.match_basis, l.intake_path, l.new_atm_id, l.reopened, e.locator FROM reg_recurrence_links l JOIN reg_source_entries e ON e.entry_id=l.reported_entry_id")
head_atm = q(db1, "SELECT m.atm_id FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE e.locator=?", loc(510, "login-spacing"))[0][0]
mem_atm = q(db1, "SELECT m.atm_id FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE e.locator=?", loc(511, "login-spacing-again"))[0][0]
check("(d) one reg_recurrence_links row: UNDECIDED (mint-with-link, the doc04 section 8 autonomous default; never SAME_DEFECT), intake_path import, not reopened, head and new item as expected",
      lk == [(head_atm, "UNDECIDED", "normalised(subject,scope)", "import", mem_atm, 0, loc(511, "login-spacing-again"))], lk)
check("(d) the same slug with a different screen (HELIX-512) and the same slug with a different platform class (HELIX-520 web, HELIX-521 androidtv; HELIX-540 androidtv, HELIX-541 desktop on one screen) are DISTINCT and never linked",
      {d_["slug"] for d_ in fam.get("distinct_same_slug", [])} == {"excessive-white-space-on-login-page", "lack-of-text-contrast", "overlapping-labels-on-the-settings-page"}
      and q(db1, "SELECT count(*) FROM reg_recurrence_links")[0][0] == 1, fam.get("distinct_same_slug"))
check("(d) v_recurrence_violations is empty and the entry-set export is byte-identical for the families stage too",
      q(db1, "SELECT count(*) FROM v_recurrence_violations")[0][0] == 0 and read_tsv(os.path.join(SCR, "main.fam", "entryset-before.tsv")) == read_tsv(os.path.join(SCR, "main.fam", "entryset-after.tsv")))
check("(d) the engine validate, foreign_key_check and integrity_check are clean after the families stage", sh([WI, "validate", "--db", db1]).returncode == 0 and q(db1, "PRAGMA foreign_key_check") == [] and q(db1, "PRAGMA integrity_check") == [("ok",)])
s_all = state(db1)

# ---- (e) idempotent + resumable
r1 = imp(db1, fj1, "qa", os.path.join(SCR, "main.qa2"))
r2 = imp(db1, fj1, "families", os.path.join(SCR, "main.fam2"))
check("(e) a second run of each stage exits 0 and writes nothing (state identical, no new id, no new link)", r1.returncode == 0 and r2.returncode == 0 and state(db1) == s_all, (r1.stderr[-200:], r2.stderr[-200:]))
for stage in ("after_mint", "after_ext", "after_add", "before_map"):
    for nth in (1, 3):
        t_, fj_, db = setup("fault_%s_%d" % (stage, nth), _sev502)
        r1 = imp(db, fj_, "qa", os.path.join(SCR, "f.qa"), env={"IMPORT_TEST_MODE": "1", "IMPORT_FAULT": "%s:%d" % (stage, nth)})
        partial = state(db)
        r2 = imp(db, fj_, "qa", os.path.join(SCR, "f.qa2"))
        imp168(db, fj_)
        imp(db, fj_, "families", os.path.join(SCR, "f.fam"))
        check("(e) crash %s at cause %d (exit 137), then a re-run completes with the state of a clean run" % (stage, nth), r1.returncode == 137 and partial != s_all and r2.returncode == 0 and state(db) == s_all, (r1.returncode, r2.returncode, r2.stderr[-200:]))

# ---- (a) the entry-set invariant: a stage that adds an entry, or adds one and deletes another, is refused naming every triple
t_, fj_, db = setup("es_insert")
sh(["sqlite3", db, "CREATE TRIGGER t172_plant AFTER INSERT ON reg_source_map BEGIN INSERT OR IGNORE INTO reg_source_entries(source_id,locator,legacy_id,title,entry_sha256) "
    "SELECT source_id,'docs/issues/PLANTED.md','HELIX-999','planted','%s' FROM reg_sources WHERE kind='issue_file' LIMIT 1; END;" % ("b" * 64)])
r = imp(db, fj_, "qa", os.path.join(SCR, "es1.out"))
check("(a) a stage whose writes insert ONE reg_source_entries row is refused `t172_entry_set_changed` (exit 20) naming the planted triple",
      r.returncode == 20 and b"t172_entry_set_changed" in r.stderr and b"docs/issues/PLANTED.md" in r.stderr, (r.returncode, r.stderr[-300:]))
ev = json.load(open(os.path.join(SCR, "es1.out", "entryset.json"))) if os.path.exists(os.path.join(SCR, "es1.out", "entryset.json")) else {}
check("(a) the verdict file records identical=false with the added triple", ev.get("identical") is False and len(ev.get("added", [])) == 1 and ev.get("removed") == [], ev)
t_, fj_, db = setup("es_swap")
sh(["sqlite3", db, "DROP TRIGGER reg_source_entries_no_delete"])
sh(["sqlite3", db, "CREATE TRIGGER t172_swap AFTER INSERT ON reg_source_map BEGIN INSERT OR IGNORE INTO reg_source_entries(source_id,locator,legacy_id,title,entry_sha256) "
    "SELECT source_id,'docs/issues/PLANTED.md','HELIX-999','planted','%s' FROM reg_sources WHERE kind='issue_file' LIMIT 1; "
    "DELETE FROM reg_source_entries WHERE locator='docs/issues/HELIX-001-b.md' AND NOT EXISTS (SELECT 1 FROM reg_source_map WHERE entry_id=reg_source_entries.entry_id); END;" % ("b" * 64)])
n_before = q(db, "SELECT count(*) FROM reg_source_entries")[0][0]
r = imp(db, fj_, "qa", os.path.join(SCR, "es2.out"))
ev = json.load(open(os.path.join(SCR, "es2.out", "entryset.json"))) if os.path.exists(os.path.join(SCR, "es2.out", "entryset.json")) else {}
check("(a) a stage that inserts one row and deletes another (the COUNT is unchanged) is refused `t172_entry_set_changed`, the verdict lists both triples",
      r.returncode == 20 and b"t172_entry_set_changed" in r.stderr and q(db, "SELECT count(*) FROM reg_source_entries")[0][0] == n_before and len(ev.get("added", [])) == 1 and len(ev.get("removed", [])) == 1, (r.returncode, r.stderr[-300:], ev))

# ---- (c) classification limits
def edit_limits(t):
    W.w(t, "docs/issues/HELIX-530-open-qa.md", F.tk(530, "open-qa", "Open ticket with the QA statement", status="open", platform="androidtv", resolution=F.GENERIC)[1])
    W.w(t, "docs/issues/HELIX-531-closed-product.md", F.tk(531, "closed-product", "Closed with a product reason", status="closed", platform="androidtv", resolution="Fixed in the product: the focus order was corrected in the TV launcher.")[1])


t_, fj_, db = setup("limits", edit_limits)
r = imp(db, fj_, "qa", os.path.join(SCR, "lim.out"))
mp2 = {r_[0]: r_[1] for r_ in q(db, "SELECT e.locator, m.atm_id FROM reg_source_map m JOIN reg_source_entries e USING(entry_id)")}
check("(c) an OPEN ticket that carries the QA statement and a closed ticket without it are not grouped (only the six closed QA tickets are mapped)",
      r.returncode == 0 and len(mp2) == 6 and "docs/issues/HELIX-530-open-qa.md" not in mp2 and "docs/issues/HELIX-531-closed-product.md" not in mp2, (r.returncode, sorted(mp2)))

# ---- (f) refusals, nothing written
def nothing(db):
    return q(db, "SELECT count(*) FROM items")[0][0] == 0 and q(db, "SELECT count(*) FROM reg_source_map")[0][0] == 0 and q(db, "SELECT count(*) FROM reg_ids")[0][0] == 0


t_, fj_, dbn = setup("nocomp", components=("helixqa",))
r = imp(dbn, fj_, "qa", os.path.join(SCR, "nc.out"))
check("(f) a cause whose component is not a reg_components row (qa-scripts) is refused `component_not_registered`, nothing written", r.returncode == 20 and b"component_not_registered" in r.stderr and nothing(dbn), (r.returncode, r.stderr[-250:]))
def _strip_qa(t):        # the damaged-register tree: the same ticket files WITHOUT the QA statement, so import.sh imports each one per ticket (the pre-guard state)
    for n, sl in QA.items():
        pth = os.path.join(t, loc(n, sl))
        s_ = open(pth).read()
        open(pth, "w").write(re.sub(r"^resolution: .*$", "resolution: Not reproducible.", s_, flags=re.M))


t_, fj_, dbi = setup("indiv", _strip_qa)                        # enumerated and imported from the stripped tree ...
ri = imp168(dbi, fj_)
TB_ = mktree("indivB")                                          # ... then the stage runs on the real tree (the real QA statements)
fjb_ = os.path.join(SCR, "indivB.freeze.json")
F.freeze2(TB_, fjb_)
before = state(dbi)
r = imp(dbi, fjb_, "qa", os.path.join(SCR, "ind.out"))
check("(f) QA tickets that import.sh already imported one by one: refused `qa_entry_individually_imported`, nothing written", ri.returncode == 0 and r.returncode == 20 and b"qa_entry_individually_imported" in r.stderr and state(dbi) == before, (ri.returncode, r.returncode, r.stderr[-250:]))
check("(F1) the refusal states the recovery path (restore the pre-import backup, then stage qa to completion, then import.sh; the guide section Recovery), not only the stage to run",
      b"Recovery: restore the pre-import backup" in r.stderr and b"--stage qa to completion" in r.stderr and b"section Recovery" in r.stderr, r.stderr[-400:])
check("(f) the refusal states that it cannot be repaired by the tool and names the recovery (restore the pre-import backup, stage qa to completion, then import.sh)",
      b"cannot be repaired" in r.stderr and b"restore the pre-import backup" in r.stderr and b"--stage qa to completion" in r.stderr, r.stderr[-700:])
t_, fj_, dbe = setup("early")
r = imp(dbe, fj_, "families", os.path.join(SCR, "early.out"))
check("(f) the families stage before import.sh: refused `families_need_import`, nothing written", r.returncode == 20 and b"families_need_import" in r.stderr and nothing(dbe), (r.returncode, r.stderr[-250:]))
t_, fj_, dbm = setup("moved")
T_ = os.path.join(SCR, "movedtree")
W.build(T_)
F.tickets_qa(T_)
fjm = os.path.join(SCR, "movedtree.freeze.json")
F.freeze2(T_, fjm)
with open(os.path.join(T_, "docs/issues/HELIX-501-tv-focus-step-1.md"), "a") as f_:
    f_.write("\nsneaked in\n")
r = imp(dbm, fjm, "qa", os.path.join(SCR, "mv.out"))
check("(f) a snapshot changed after its manifest: refused `freeze_snapshot_moved` (exit 20), nothing written", r.returncode == 20 and b"freeze_snapshot_moved" in r.stderr and nothing(dbm), (r.returncode, r.stderr[-250:]))
T_ = mktree("newer", lambda t: open(os.path.join(t, "docs/issues/HELIX-501-tv-focus-step-1.md"), "a").write("\nedited after enumeration\n"))
fjn = os.path.join(SCR, "newer.freeze.json")
F.freeze2(T_, fjn)
t_, fj_, dbo = setup("older")
r = imp(dbo, fjn, "qa", os.path.join(SCR, "old.out"))
check("(f) entries older than the snapshot (a ticket changed after enumeration): refused `entry_changed` (exit 20) naming the ticket, nothing written",
      r.returncode == 20 and b"entry_changed" in r.stderr and b"HELIX-501" in r.stderr and nothing(dbo), (r.returncode, r.stderr[-300:]))
real = os.path.join(SCR, "workable_items.db")
t_, fj_, dbr = setup("realname")
shutil.copy(dbr, real)
r = imp(real, fj_, "qa", os.path.join(SCR, "real.out"))
check("(f) the real register database name without a backup record: refused `backup_record_required`, nothing written", r.returncode == 20 and b"backup_record_required" in r.stderr and nothing(real), (r.returncode, r.stderr[-250:]))

# ---- (F1) the ORDER guard: import.sh refuses before any mint while a QA-instrument ticket is unmapped (the recovery of a refusal that fires after the damage does not exist)
t_, fj_, dbg = setup("guard")
before = state(dbg)
rg = imp168(dbg, fj_)
rgd = imp168(dbg, fj_, "--dry-run")
check("(F1) import.sh BEFORE stage qa is refused `qa_grouping_not_run` (exit 20), names a QA ticket and the stage to run, and writes nothing (no id minted, no item, no map row)",
      rg.returncode == 20 and b"qa_grouping_not_run" in rg.stderr and b"HELIX-50" in rg.stderr and b"import_t172.sh --stage qa" in rg.stderr and state(dbg) == before and nothing(dbg), (rg.returncode, rg.stderr[-400:]))
check("(F1) the dry run is refused the same way (the order is wrong whether or not it writes)", rgd.returncode == 20 and b"qa_grouping_not_run" in rgd.stderr, (rgd.returncode, rgd.stderr[-200:]))
t_, fj_, dbc = setup("guardcrash", _sev502)
r1 = imp(dbc, fj_, "qa", os.path.join(SCR, "gc.qa"), env={"IMPORT_TEST_MODE": "1", "IMPORT_FAULT": "after_mint:5"})
partial = state(dbc)
rc_ = imp168(dbc, fj_)
check("(F1) the reviewer's sequence: stage qa crashes at the last cause (wrong_screen, HELIX-503) (exit 137), then import.sh is REFUSED `qa_grouping_not_run` (it used to import HELIX-503 as its own item), nothing written",
      r1.returncode == 137 and rc_.returncode == 20 and b"qa_grouping_not_run" in rc_.stderr and b"HELIX-503" in rc_.stderr and state(dbc) == partial, (r1.returncode, rc_.returncode, rc_.stderr[-300:]))
r2 = imp(dbc, fj_, "qa", os.path.join(SCR, "gc.qa2"))
r3 = imp168(dbc, fj_)
check("(F1) stage qa is then re-run (it resumes) and import.sh succeeds with no per-ticket item for a QA ticket", r2.returncode == 0 and r3.returncode == 0 and q(dbc, "SELECT count(*) FROM v_unmapped_entries WHERE locator LIKE 'docs/issues/%'")[0][0] == 0, (r2.returncode, r3.returncode, r3.stderr[-200:]))
# ---- (F5) a cause item exists and a ticket of that cause is mapped to ANOTHER item: refused, naming the ticket (kills the narrowed refusal condition `if r and not cause_atm`)
t_, fj_, dbm5 = setup("mapped_elsewhere", _sev502)
imp(dbm5, fj_, "qa", os.path.join(SCR, "me.qa"), env={"IMPORT_TEST_MODE": "1", "IMPORT_FAULT": "after_mint:5"})
other = q(dbm5, "SELECT atm_id FROM reg_source_map LIMIT 1")[0][0]
c_ = sqlite3.connect(dbm5)
c_.execute("INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by) SELECT entry_id, ?, 'duplicate_of', 'import_1to1', 0, 'planted' FROM reg_source_entries WHERE locator=?", (other, loc(503, QA[503])))
c_.commit()
c_.close()
before = state(dbm5)
rm = imp(dbm5, fj_, "qa", os.path.join(SCR, "me.qa2"))
check("(F5) the cause item of HELIX-503 exists (minted before the crash) and HELIX-503 is mapped to another item: refused `qa_entry_individually_imported` naming HELIX-503 (exit 20, no crash), nothing written",
      rm.returncode == 20 and b"qa_entry_individually_imported" in rm.stderr and b"HELIX-503" in rm.stderr and state(dbm5) == before, (rm.returncode, rm.stderr[-300:]))

# ---- (F4) the class membership of the QA-instrument classifier, member by member (the live-tree inventory is evidence/register/w2/fix-r2-qa-class-inventory.txt)
def edit_classes(t):
    def put(i, slug, **kw):
        W.w(t, *F.tk(i, slug, "Ticket %d" % i, **kw))
    put(532, "wontfix-wrong-screen", status="wontfix", platform="androidtv", resolution=WRONGSCR_W)
    put(533, "fp-transition", status="resolved", platform="androidtv", body="Nothing visible.\n\n## Resolution\n\nFalse positive: screenshot captured during screen transition or app loading. The app renders correctly after initial load.\nResolved: 2026-03-30")
    put(534, "open-fp", status="open", platform="androidtv", body="x\n\n## Resolution\n\nFalse positive: screenshot captured during screen transition or app loading.")
    put(535, "wontfix-product", status="wontfix", platform="androidtv", resolution="Works as designed; not reproducible in the product.")
    put(536, "notes-only", status="fixed", platform="androidtv", body="x\n\n## Notes\n\nQA infrastructure failure was suspected by the reporter.\n\n## Resolution\n\nFixed the layout in the product.")
    put(537, "fp-other", status="resolved", platform="androidtv", body="x\n\n## Resolution\n\nFalse positive: the layout really is wrong, fixed in the product.")


WRONGSCR_W = "QA infrastructure failure: wrong screen captured, the harness never left the launcher. Won't fix, the harness is replaced."
t_, fj_, dbk = setup("classes", edit_classes)
rk = imp(dbk, fj_, "qa", os.path.join(SCR, "cls.out"))
mpk = {r_[0]: r_[1] for r_ in q(dbk, "SELECT e.locator, m.atm_id FROM reg_source_map m JOIN reg_source_entries e USING(entry_id)")}
reck = json.load(open(os.path.join(SCR, "cls.out", "qa_causes.json"))) if rk.returncode == 0 else {}
cb = {k: v["atm_id"] for k, v in reck.get("causes", {}).items()}
check("(F4) a WONTFIX ticket that states the QA tool was wrong joins its cause item (wrong_screen), like its closed siblings", loc(532, "wontfix-wrong-screen") in mpk and mpk[loc(532, "wontfix-wrong-screen")] == cb.get("wrong_screen"), (rk.returncode, sorted(mpk)))
check("(F4) a `False positive: screenshot captured during screen transition or app loading` closure written in the `## Resolution` section of the body joins screenshot_timing",
      loc(533, "fp-transition") in mpk and mpk[loc(533, "fp-transition")] == cb.get("screenshot_timing"), sorted(mpk))
check("(F4) NOT grouped: an open ticket with that statement, a wontfix ticket with a product reason, a QA phrase in a `## Notes` section, a `False positive:` of another kind",
      not any(loc(n, s) in mpk for n, s in ((534, "open-fp"), (535, "wontfix-product"), (536, "notes-only"), (537, "fp-other"))) and len(mpk) == 8, sorted(mpk))
rk2 = imp168(dbk, fj_)
check("(F4) import.sh then imports the four non-QA tickets as their own items and no QA ticket", rk2.returncode == 0 and q(dbk, "SELECT count(*) FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE e.locator IN (?,?,?,?) AND m.relation='primary'",
      loc(534, "open-fp"), loc(535, "wontfix-product"), loc(536, "notes-only"), loc(537, "fp-other"))[0][0] == 4, (rk2.returncode, rk2.stderr[-200:]))

# ---- (F8) the platform rules treat `_` as a separator (snake_case names), a declared platform spelled as a class is that class, a bare logcat names no platform
def edit_plat(t):
    def put(i, slug, title, **kw):
        W.w(t, *F.tk(i, slug, title, status="fixed", **kw))
    put(550, "snake-screen", "Snake screen", screen="android_home.png", body="Seen on the home page.")
    put(551, "snake-logcat", "Snake logcat", body="See logcat_android.txt for the trace.")
    put(552, "bare-logcat", "Bare logcat", body="See logcat.txt for the trace.")
    put(553, "decl-a", "Same declared class", platform="Android TV", screen="atv-001.png", body="declared with a space")
    put(554, "decl-b", "Same declared class", platform="androidtv", screen="atv-001.png", body="declared without a space")
    put(555, "tv-snake", "Snake tv screen", screen="android_tv_home.png", body="x")


t_, fj_, dbp = setup("plat", edit_plat)
imp(dbp, fj_, "qa", os.path.join(SCR, "pl.qa"))
imp168(dbp, fj_)
rpf = imp(dbp, fj_, "families", os.path.join(SCR, "pl.fam"))
pip = {r_["locator"]: r_ for r_ in json.load(open(os.path.join(SCR, "pl.fam", "platform_inference.json")))["rows"]} if rpf.returncode == 0 else {}
famp = json.load(open(os.path.join(SCR, "pl.fam", "families.json"))) if rpf.returncode == 0 else {}
g = lambda n, s: (pip.get(loc(n, s), {}).get("platform"), pip.get(loc(n, s), {}).get("basis"))
check("(F8) `android_home.png` (screen) and `logcat_android.txt` (body) are android evidence (an underscore separates words), `android_tv_home.png` is androidtv",
      g(550, "snake-screen") == ("android", "inferred") and g(551, "snake-logcat") == ("android", "inferred") and g(555, "tv-snake") == ("androidtv", "inferred"), (g(550, "snake-screen"), g(551, "snake-logcat"), g(555, "tv-snake")))
check("(F8) a bare `logcat.txt` names no platform (the Android log tool serves android AND androidtv): UNKNOWN_no_evidence, never guessed", g(552, "bare-logcat") == ("UNKNOWN", "UNKNOWN_no_evidence"), g(552, "bare-logcat"))
check("(F8) a declared `Android TV` is the class androidtv, so it forms ONE family with a declared `androidtv` of the same title and screen",
      g(553, "decl-a") == ("androidtv", "declared") and any(f_["platform_class"] == "androidtv" and f_["screen"] == "atv-001.png" and {f_["head"]} | {m_["locator"] for m_ in f_["members"]} == {loc(553, "decl-a"), loc(554, "decl-b")}
                                                              for f_ in famp.get("families", [])), (g(553, "decl-a"), famp.get("families")))

# ---- (F7) every record names the freeze it was produced from, and says whether it is fixture evidence
ids_ = [json.load(open(os.path.join(SCR, d_, n_))) for d_, n_ in (("main.qa", "qa_causes.json"), ("main.fam", "families.json"), ("main.fam", "platform_inference.json"), ("main.qa", "entryset.json"))]
fjd = json.load(open(fj1))
man_sha = hashlib.sha256(open(fjd["manifest"], "rb").read()).hexdigest()
check("(F7) qa_causes.json, families.json, platform_inference.json and entryset.json each carry freeze {head, frozen_at, listing_sha256, manifest_sha256} equal to the freeze they were produced from",
      all(d_.get("freeze") == {"head": "scratch", "frozen_at": fjd["frozen_at"], "listing_sha256": fjd["listing_sha256"], "manifest_sha256": man_sha, "fixture": True} for d_ in ids_), [d_.get("freeze") for d_ in ids_])
check("(F7) the fixture freeze (head `scratch`) marks every record `fixture: true`", all(d_.get("fixture") is True for d_ in ids_))
t_, fj_, dbf7 = setup("realhead", _sev502)
fjx = json.load(open(fj_))
fjx["head"] = "a1" * 20
json.dump(fjx, open(fj_, "w"))
r7 = imp(dbf7, fj_, "qa", os.path.join(SCR, "f7.qa"))
d7 = json.load(open(os.path.join(SCR, "f7.qa", "qa_causes.json"))) if r7.returncode == 0 else {}
check("(F7) a freeze whose head is a full 40-hex git sha is NOT marked fixture (the marker is derived from the freeze, never typed)", r7.returncode == 0 and d7.get("fixture") is False and d7["freeze"]["head"] == "a1" * 20, (r7.returncode, d7.get("fixture")))

print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
sys.exit(1 if FAILS else 0)
