#!/usr/bin/env python3
"""test_seed_import.py - T170 (WP-20): plan-document seeds, source class S-26 (docs/21 sections 9.1 and 9.3): the Stage 0 rule of enumerate_sources.py and the
importer scripts/register/import_plan.sh --class seeds. Real SQLite (apply_ext.sh), the real engine binary, scratch databases and scratch snapshots only.
Env: IMPORT_PLAN (importer under test; mutation runs point it at a mutated copy that sits beside copies of import_tickets.py and enumerate_sources.py),
ENUMERATE_SOURCES (default: the one beside IMPORT_PLAN), WI (engine), REAL_FREEZE_JSON (optional real-doc leg: a freeze json of a tree holding the real plan documents),
SCRATCH_BASE.
Checks (each is a failure class of the task text, T170):
  (a) planted-seed completeness (the RED of T170): ONE seed row added to a scratch copy of docs/21 yields exactly ONE new reg_source_entries row, removing it restores the
      baseline sql byte for byte; a docs/21 whose Count cell disagrees with its ids, or whose Total row disagrees with the rows, is refused (never imported short)
  (b) the import: one item per seed except the section 9.3 families (one item per family, the other members `duplicate_of` the head), the partly-folded member stays
      separate, items <= 254 - fully folded + families, every seed id mapped (v_unmapped_entries empty for the source), Type Task / status Queued, component_id NULL,
      no reg_findings row, descriptions <= 2,048 bytes with a Sources block, exactly one severity_governs per item
  (c) severity: the source's own label normalised (High if confirmed -> high, Low-Medium -> medium, rated word kept in severity_source), an unrated seed defaulted medium
  (d) a seed already imported by ANOTHER source (same legacy id) is linked, never minted twice (11.4.214); a family with such a member links as a whole
  (e) idempotent, resumable (a crash after each stage then a re-run gives the state of a clean run), a second run writes nothing
  (f) refusals with nothing written: snapshot changed after the manifest (`freeze_snapshot_moved`), entries older than the snapshot (`entries_out_of_date`), a family
      member that is not a seed, the real register database name without a backup record, a missing plan document
  (g) the R-ADJ seeds of docs/18 name the plan document and fix work package their routing gives
  (h) real documents (REAL_FREEZE_JSON): 254 entries, 198 items (the docs/21 bound), 30 families, 86 fully folded members, doc15:S-17 kept separate, rated 135 / unrated 119,
      the severity labels agree with the docs/21 section 9.2 control row counts
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

IMP = os.environ.get("IMPORT_PLAN", os.path.join(REG, "import_plan.sh"))
ENUM = os.environ.get("ENUMERATE_SOURCES", os.path.join(os.path.dirname(IMP), "enumerate_sources.py"))
WI = os.environ.get("WI", os.path.join(ROOT, "submodules/constitution/scripts/workable-items/bin/workable-items-linux"))
PLAN_SRC = "plan-document seeds (docs/21 section 9.1)"
FAILS = PASSES = 0


def check(label, cond, info=""):
    global FAILS, PASSES
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:900]))
    if cond:
        PASSES += 1
    else:
        FAILS += 1


def sh(cmd, **kw):
    return subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kw)


def sha(p):
    return hashlib.sha256(open(p, "rb").read()).hexdigest()


inc = os.path.exists("/run/.containerenv") or os.environ.get("container") == "podman"
print("# identity: task=T170 utc=%s uid=%d git_head=%s" % (time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), os.getuid(), sh(["git", "-C", ROOT, "rev-parse", "HEAD"]).stdout.decode().strip()))
print("# container: %s" % ("in-container run (container=%s) image=IMG-TESTUTIL" % os.environ.get("container", "unset") if inc else "host-side run (no container marker); a container leg is UNCONFIRMED for this transcript"))
print("# importer=%s sha256=%s enumerator sha256=%s engine sha256=%s" % (IMP, sha(IMP)[:16] if os.path.isfile(IMP) else "ABSENT", sha(ENUM)[:16] if os.path.isfile(ENUM) else "ABSENT", sha(WI)[:16]))
if not os.path.isfile(IMP):
    check("importer present: %s (RED: T170 not implemented)" % IMP, False)
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


def imp(db, fj, klass="seeds", *extra, env=None):
    e = dict(os.environ)
    e.update(env or {})
    return sh([IMP, "--class", klass, "--db", db, "--freeze-json", fj, "--engine", WI, "--report", db + ".run.json", "--reconciliation", db + ".rec.json"] + list(extra), env=e)


def state(db):
    """a canonical dump of everything the import wrote (ids are minted in order, so a resumed and a clean run must agree)"""
    c = sqlite3.connect(db)
    try:
        return json.dumps({
            "items": c.execute("SELECT atm_id, type, status, severity, title, description FROM items ORDER BY atm_id").fetchall(),
            "ext": c.execute("SELECT atm_id, category, defect_layer, severity, severity_source, custody_basis, reverify_required, component_id FROM reg_item_ext ORDER BY atm_id").fetchall(),
            "map": c.execute("SELECT e.locator, m.atm_id, m.relation, m.match_basis, m.severity_governs, m.mapped_by FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) ORDER BY e.locator").fetchall(),
            "ids": c.execute("SELECT atm_id, minted_by, mint_basis FROM reg_ids ORDER BY seq").fetchall()}, sort_keys=True)
    finally:
        c.close()


def n_entries(out):
    return json.load(open(os.path.join(out, "enumeration-stats.json")))["classes"]["S-26"]["entries"]


# ---- (a) planted-seed completeness + the table controls
BASE = mktree("base")
fj0, out0, r = enumerate_tree(BASE, "base")
check("(a) baseline enumeration exits 0", r.returncode == 0, r.stdout + r.stderr)
N0 = n_entries(out0)
check("(a) S-26 reads 27 entries on the fixture (a class that reads zero is a blind instrument)", N0 == 27, N0)
sql0 = open(os.path.join(out0, "source_entries.sql")).read()


def plant_row(t):
    p = os.path.join(t, DOC21)
    s = open(p).read()
    s = s.replace("| **Total itemised** | | | **27** | |", "| doc20 §2.2 | T-1 | security tooling | 1 | unrated |\n| **Total itemised** | | | **28** | |")
    open(p, "w").write(s)


T = mktree("plant", plant_row)
fjp, outp, r = enumerate_tree(T, "plant")
check("(a) one seed row added to a scratch copy of docs/21 -> exactly ONE new S-26 entry", r.returncode == 0 and n_entries(outp) == N0 + 1, (r.returncode, r.stderr[-200:]))
dbb = load_sql(fresh_db("plant_base"), out0)
dbp = fresh_db("plant_planted")
load_sql(dbp, outp)
check("(a) the register gains exactly ONE new reg_source_entries row (by natural key) and it is the planted seed",
      q(dbp, "SELECT count(*) FROM reg_source_entries")[0][0] - q(dbb, "SELECT count(*) FROM reg_source_entries")[0][0] == 1
      and q(dbp, "SELECT count(*) FROM reg_source_entries WHERE locator LIKE '%#seed:doc20:T-1'")[0][0] == 1)
T = mktree("plant_removed", plant_row)
p = os.path.join(T, DOC21)
_s = open(p).read()
open(p, "w").write(_s.replace("| doc20 §2.2 | T-1 | security tooling | 1 | unrated |\n", "").replace("**28**", "**27**"))
fjr, outr, r = enumerate_tree(T, "plant_removed")
check("(a) the planted row removed from the planted copy: the sql equals the baseline byte for byte", sha(os.path.join(outr, "source_entries.sql")) == sha(os.path.join(out0, "source_entries.sql")))
def _sub(t, a, b):
    pth = os.path.join(t, DOC21)
    s = open(pth).read()
    assert a in s, a
    open(pth, "w").write(s.replace(a, b))


for label, edit, reason in (
        ("a Count cell that disagrees with the ids", lambda t: _sub(t, "| doc01 §15 | O-01..O-03 | cross-system | 3 |", "| doc01 §15 | O-01..O-03 | cross-system | 4 |"), "seed_count_mismatch"),
        ("a Total row that disagrees with the rows", lambda t: _sub(t, "**27**", "**26**"), "seed_count_mismatch"),
        ("a duplicate seed pair", lambda t: _sub(t, "| doc15 §13 | S-01..S-03 |", "| doc15 §13 | S-01..S-03, S-03 |"), "seed_count_mismatch"),
        ("a family member that is not a seed", lambda t: _sub(t, "doc07 C1; doc15 S-01 (linked", "doc07 C1; doc15 S-09 (linked"), None),
        ("a family count that disagrees with the table", lambda t: _sub(t, "These 5 families list 15 member ids", "These 6 families list 15 member ids"), None)):
    tt = mktree("bad_" + re.sub(r"\W", "", label)[:12], edit)
    fjx, outx, r = enumerate_tree(tt, "bad_" + re.sub(r"\W", "", label)[:12])
    if reason:
        check("(a) %s: the enumerator refuses `%s` (exit 20), nothing written" % (label, reason), r.returncode == 20 and reason.encode() in r.stderr and not os.path.exists(os.path.join(outx, "source_entries.sql")), (r.returncode, r.stderr[-200:]))
    else:
        # the family tables are read by the importer (the enumerator reads 9.1 and 9.5 only)
        db = load_sql(fresh_db("bad_fam"), outx) if r.returncode == 0 else None
        if db:
            r2 = imp(db, fjx)
            check("(a) %s: the importer refuses (exit 20), nothing written" % label, r2.returncode == 20 and q(db, "SELECT count(*) FROM items")[0][0] == 0, (r2.returncode, r2.stderr[-250:]))
        else:
            check("(a) %s: refused already at enumeration (exit 20)" % label, r.returncode == 20, r.returncode)

# ---- (b) the import
db1 = load_sql(fresh_db("main"), out0)
r = imp(db1, fj0)
check("(b) the import exits 0", r.returncode == 0, r.stdout + r.stderr)
rec = json.load(open(db1 + ".rec.json")) if os.path.exists(db1 + ".rec.json") else {}
items = q(db1, "SELECT count(DISTINCT m.atm_id) FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator=?", PLAN_SRC)[0][0]
check("(b) 27 seeds, 5 families, 14 fully folded members -> 18 items (= 27 - 14 + 5, the docs/21 bound formula)", items == 18 and rec.get("items_upper_bound") == 18 and rec.get("families") == 5 and rec.get("fully_folded_members") == 14, (items, rec))
check("(b) the partly folded member doc15:S-03 keeps its own item", rec.get("partly_folded_members_kept_separate") == ["doc15:S-03"] and
      q(db1, "SELECT m.relation FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE e.locator LIKE '%#seed:doc15:S-03'")[0][0] == "primary")
check("(b) every seed id is mapped (v_unmapped_entries empty for the source)", q(db1, "SELECT count(*) FROM v_unmapped_entries WHERE source=?", PLAN_SRC)[0][0] == 0)
check("(b) relations: 18 primary + 9 duplicate_of = 27 map rows (head primary, the other members duplicate_of the head)",
      dict(q(db1, "SELECT m.relation, count(*) FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator=? GROUP BY 1", PLAN_SRC)) == {"primary": 18, "duplicate_of": 9})
check("(b) folded members use match_basis manual_operator (the folding is the plan's own table), single seeds import_1to1",
      {r_[0]: r_[1] for r_ in q(db1, "SELECT relation, group_concat(DISTINCT match_basis) FROM reg_source_map GROUP BY 1 ORDER BY 1")}.get("duplicate_of") == "manual_operator"
      and set((q(db1, "SELECT group_concat(DISTINCT match_basis) FROM reg_source_map WHERE relation='primary'")[0][0] or "").split(",")) == {"manual_operator", "import_1to1"})
check("(b) all items are Task / Queued; reg_item_ext custody machine_evidence, defect_layer source, component_id NULL; no reg_findings row",
      q(db1, "SELECT DISTINCT type||'/'||status FROM items") == [("Task/Queued",)] and q(db1, "SELECT DISTINCT custody_basis||'/'||defect_layer||'/'||reverify_required FROM reg_item_ext") == [("machine_evidence/source/0",)]
      and q(db1, "SELECT count(*) FROM reg_item_ext WHERE component_id IS NOT NULL")[0][0] == 0 and q(db1, "SELECT count(*) FROM reg_findings")[0][0] == 0)
check("(b) every description is within 2,048 bytes and carries a Sources block with the entry locator and its sha256",
      all(len(d.encode("utf-8")) <= 2048 and "**Sources**" in d and "#seed:" in d and re.search(r"sha256: [0-9a-f]{64}", d) for (d,) in q(db1, "SELECT description FROM items")))
check("(b) exactly one severity_governs=1 row per item", q(db1, "SELECT count(*) FROM (SELECT atm_id, sum(severity_governs) s FROM reg_source_map GROUP BY atm_id) WHERE s <> 1")[0][0] == 0)
check("(b) the engine validate, foreign_key_check and integrity_check are clean", sh([WI, "validate", "--db", db1]).returncode == 0 and q(db1, "PRAGMA foreign_key_check") == [] and q(db1, "PRAGMA integrity_check") == [("ok",)])
check("(b) no register id is minted twice: reg_ids has exactly 18 import ids for 27 entries", q(db1, "SELECT count(*) FROM reg_ids WHERE mint_basis='import'")[0][0] == 18)

# ---- (c) severity
sev = dict(q(db1, "SELECT e.locator, x.severity FROM reg_source_entries e JOIN reg_source_map m USING(entry_id) JOIN reg_item_ext x USING(atm_id) WHERE m.relation='primary'"))
src = dict(q(db1, "SELECT e.locator, x.severity_source FROM reg_source_entries e JOIN reg_source_map m USING(entry_id) JOIN reg_item_ext x USING(atm_id) WHERE m.relation='primary'"))
loc = lambda d, i: "%s#seed:%s:%s" % (DOC21, d, i)
fam1_head = q(db1, "SELECT m.atm_id FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE e.locator=?", loc("doc01", "O-02"))[0][0]
check("(c) the family 'Unauthenticated /ws' (doc01 O-02, doc07 C2 'High if confirmed', doc08 WEB-F02 'High', doc15 S-02, doc18 T5-D) takes the HIGHEST member severity",
      q(db1, "SELECT severity FROM reg_item_ext WHERE atm_id=?", fam1_head)[0][0] == "high", q(db1, "SELECT severity, severity_source FROM reg_item_ext WHERE atm_id=?", fam1_head))
check("(c) 'Low-Medium' (doc07 C4) normalises to the higher word, medium, and keeps the raw label in severity_source", sev.get(loc("doc07", "C4")) == "medium" and "Low-Medium" in src.get(loc("doc07", "C4"), ""), (sev.get(loc("doc07", "C4")), src.get(loc("doc07", "C4"))))
check("(c) an unrated seed (doc01 O-03) is defaulted medium and says so [DEFAULT - adjustable]", sev.get(loc("doc01", "O-03")) == "medium" and "DEFAULT - adjustable" in src.get(loc("doc01", "O-03"), ""), src.get(loc("doc01", "O-03")))
check("(c) a 'High' seed (doc07 C1, head of its family) is high", sev.get(loc("doc07", "C1")) == "high", sev.get(loc("doc07", "C1")))
gov = q(db1, "SELECT e.locator FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE m.atm_id=? AND m.severity_governs=1", fam1_head)
check("(c) the member that gives the family severity governs (one severity_governs row; the 'High' WEB-F02, not the 'High if confirmed' C2)", len(gov) == 1 and gov[0][0] in (loc("doc08", "WEB-F02"),), gov)

# ---- (g) the R-ADJ seeds name their plan document and fix work package
d = q(db1, "SELECT i.description FROM items i JOIN reg_source_map m ON m.atm_id=i.atm_id JOIN reg_source_entries e USING(entry_id) WHERE e.locator=?", loc("doc18", "T1-A"))[0][0]
check("(g) the R-ADJ seed doc18 T1-A names the plan document and fix work package of docs/18 section 1.1 (doc 07, WP-30, WP-51)", "Routing (docs/18 section 1.1)" in d and "WP-30" in d and "WP-51" in d and "doc 07" in d, d[:600])

# ---- (e) idempotent + resumable
s1 = state(db1)
r = imp(db1, fj0)
check("(e) a second run exits 0 and writes nothing (state identical, no new id)", r.returncode == 0 and state(db1) == s1)
for stage in ("after_mint", "after_ext", "after_add", "before_map"):
    for nth in (1, 7):
        db = load_sql(fresh_db("fault"), out0)
        r1 = imp(db, fj0, env={"IMPORT_TEST_MODE": "1", "IMPORT_FAULT": "%s:%d" % (stage, nth)})
        partial = state(db)
        r2 = imp(db, fj0)
        check("(e) crash %s at group %d (exit 137), then a re-run completes with the state of a clean run" % (stage, nth), r1.returncode == 137 and partial != s1 and r2.returncode == 0 and state(db) == s1, (r1.returncode, r2.returncode, r2.stderr[-200:]))

# ---- (d) a seed already imported by another source is linked, never minted twice
db = load_sql(fresh_db("link"), out0)
con = sqlite3.connect(db)
con.execute("PRAGMA foreign_keys=ON")
con.execute("INSERT INTO reg_sources(kind,locator,parser) VALUES ('report_doc','test:other-source','test')")
con.execute("INSERT INTO reg_source_entries(source_id,locator,legacy_id,title,entry_sha256) SELECT source_id,'other.md:L1','doc07:C2','C2 in another source','%s' FROM reg_sources WHERE locator='test:other-source'" % ("a" * 64))
con.execute("INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('test','import')")
atm = con.execute("SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1").fetchone()[0]
con.commit()
con.close()
r = sh([WI, "add", "Bug", "high", "--db", db, "--id", atm, "--prefix", "CAT", "--title", "pre-existing item", "--description", "an item another source imported before the seeds"])
con = sqlite3.connect(db)
con.execute("INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,reverify_required) VALUES (?,?,?,?,?,?)", (atm, "bug", "source", "high", "machine_evidence", 0)) if not con.execute("SELECT 1 FROM reg_item_ext WHERE atm_id=?", (atm,)).fetchone() else None
con.execute("INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by) SELECT entry_id,?, 'primary','import_1to1',1,'test' FROM reg_source_entries WHERE locator='other.md:L1'", (atm,))
con.commit()
con.close()
r = imp(db, fj0)
check("(d) the import with a pre-existing item for doc07:C2 exits 0", r.returncode == 0, r.stdout + r.stderr)
links = q(db, "SELECT e.locator, m.relation, m.match_basis FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) WHERE m.atm_id=? AND e.locator LIKE '%#seed:%' ORDER BY e.locator", atm)
check("(d) the whole family of doc07:C2 (5 seeds) is linked duplicate_of the existing item and no item is minted for it", len(links) == 5 and all(x[1] == "duplicate_of" and x[2] == "ticket" for x in links), links)
check("(d) 17 items are minted for the seeds (18 - the linked family), the existing item is not re-minted", q(db, "SELECT count(*) FROM reg_ids WHERE mint_basis='import' AND minted_by LIKE 'register-import-plan:%'")[0][0] == 17)

# ---- (f) refusals, nothing written
dbf = load_sql(fresh_db("moved"), out0)
T = mktree("movedtree")
fjm = os.path.join(SCR, "movedtree.freeze.json")
F.freeze2(T, fjm)
with open(os.path.join(T, DOC21), "a") as f:
    f.write("\nsneaked in\n")
r = imp(dbf, fjm)
check("(f) a snapshot changed after its manifest: refused `freeze_snapshot_moved` (exit 20), nothing written", r.returncode == 20 and b"freeze_snapshot_moved" in r.stderr and q(dbf, "SELECT count(*) FROM items")[0][0] == 0 and q(dbf, "SELECT count(*) FROM reg_ids")[0][0] == 0, (r.returncode, r.stderr[-200:]))
def _edit_sev(t):
    p = os.path.join(t, F.DOCS, "07-backend-catalog-api-audit-plan.md")
    _s = open(p).read()
    open(p, "w").write(_s.replace("| C3 | Scanner stubs return nil | Medium |", "| C3 | Scanner stubs return nil | High |"))


T = mktree("newer", _edit_sev)
fjn = os.path.join(SCR, "newer.freeze.json")
F.freeze2(T, fjn)
dbn = load_sql(fresh_db("newer"), out0)
r = imp(dbn, fjn)
check("(f) entries older than the snapshot (a source document changed after enumeration): refused `entries_out_of_date` naming the seed, nothing written",
      r.returncode == 20 and b"entries_out_of_date" in r.stderr and b"doc07:C3" in r.stderr and q(dbn, "SELECT count(*) FROM items")[0][0] == 0, (r.returncode, r.stderr[-300:]))
real = os.path.join(SCR, "workable_items.db")
shutil.copy(fresh_db("realname"), real)
load_sql(real, out0)
r = imp(real, fj0)
check("(f) the real register database name without a backup record: refused `backup_record_required`, nothing written", r.returncode == 20 and b"backup_record_required" in r.stderr and q(real, "SELECT count(*) FROM items")[0][0] == 0, (r.returncode, r.stderr[-200:]))
T = mktree("nodoc21", lambda t: os.remove(os.path.join(t, DOC21)))
fjd, outd, r = enumerate_tree(T, "nodoc21")
dbd = load_sql(fresh_db("nodoc21"), outd)
r = imp(dbd, fjd)
check("(f) no docs/21 in the snapshot: S-26 reads nothing and the importer refuses `plan_document_missing`", n_entries(outd) == 0 and r.returncode == 20 and b"plan_document_missing" in r.stderr, (r.returncode, r.stderr[-200:]))
r = sh([IMP, "--class", "bogus", "--db", db1, "--freeze-json", fj0, "--engine", WI])
check("(f) an unknown --class is refused (exit 20), a missing option is usage (exit 2)", r.returncode == 20 and sh([IMP]).returncode == 2)
dbk = load_sql(fresh_db("dry"), out0)
r = imp(dbk, fj0, "seeds", "--dry-run")
check("(f) --dry-run writes nothing", r.returncode == 0 and q(dbk, "SELECT count(*) FROM items")[0][0] == 0 and q(dbk, "SELECT count(*) FROM reg_ids")[0][0] == 0, r.stderr[-200:])

# ---- (h) the real documents
fjr = os.environ.get("REAL_FREEZE_JSON")
if fjr:
    outR = os.path.join(SCR, "real.enum")
    r = sh([sys.executable, "-I", ENUM, "--freeze-json", fjr, "--out", outR])
    check("(h) real documents: the enumerator exits 0", r.returncode == 0, r.stderr[-300:])
    dbR = load_sql(fresh_db("real"), outR)
    r = imp(dbR, fjr)
    recR = json.load(open(dbR + ".rec.json")) if os.path.exists(dbR + ".rec.json") else {}
    check("(h) real documents: the import exits 0", r.returncode == 0, r.stdout + r.stderr)
    check("(h) 254 entries (135 rated, 119 unrated), 30 families, 87 members of which 86 fully folded, doc15:S-17 kept separate", (recR.get("entries"), recR.get("rated"), recR.get("unrated"), recR.get("families"), recR.get("family_members"), recR.get("fully_folded_members"), recR.get("partly_folded_members_kept_separate"))
          == (254, 135, 119, 30, 87, 86, ["doc15:S-17"]), recR)
    check("(h) 198 items = 254 - 86 + 30, the docs/21 bound, and the register agrees (%s)" % recR.get("items_in_register"), recR.get("items_planned") == 198 and recR.get("items_in_register") == 198 and recR.get("unmapped_entries") == 0, recR)
    check("(h) real documents: no reg_findings row, component_id NULL, every description <= 2,048 bytes",
          q(dbR, "SELECT count(*) FROM reg_findings")[0][0] == 0 and q(dbR, "SELECT count(*) FROM reg_item_ext WHERE component_id IS NOT NULL")[0][0] == 0 and q(dbR, "SELECT max(length(CAST(description AS BLOB))) FROM items")[0][0] <= 2048)
    # the control: the labels of the rated seeds, counted by normalised word, equal the docs/21 section 9.2 TOTAL row (independent: parsed from the table, not from the importer)
    l21 = open(os.path.join(json.load(open(fjr))["snapshot"], DOC21), encoding="utf-8").read()
    m = re.search(r"\| \*\*Total rated\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \| \*\*(\d+)\*\* \|", l21)
    if m:
        ctl = [int(x) for x in m.groups()]       # Critical, Critical if confirmed, High, High if confirmed, Medium-High, Medium, Low-Medium, Low, Info, Unknown, Total
        raw = [r_[0] for r_ in q(dbR, "SELECT e.raw_severity FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator=? AND e.raw_status='rated'", PLAN_SRC)]
        def cls(x):
            x = (x or "").lower()
            if x.startswith("unknown") or not x:
                return 9
            ic = "if confirmed" in x
            w = re.match(r"^(critical|high|medium|med|low|info)", x).group(1)
            if "-" in x.split("(")[0] and w in ("medium", "med"):
                return 4
            if re.match(r"^(low)-(medium|med)", x):
                return 6
            return {"critical": 1 if ic else 0, "high": 3 if ic else 2, "medium": 5, "med": 5, "low": 7, "info": 8}[w]
        got = [0] * 10
        for x in raw:
            got[cls(x)] += 1
        check("(h) the section 9.2 TOTAL row (rated %d) is reproduced by the seed labels, column by column (%s vs %s)" % (ctl[10], got, ctl[:10]), len(raw) == ctl[10] and got == ctl[:10], (got, ctl))
    else:
        print("# (h) the section 9.2 Total rated row was not found in the real docs/21: control skipped (UNCONFIRMED)")
else:
    print("# (h) real documents leg SKIPPED: REAL_FREEZE_JSON not set")

shutil.rmtree(SCR, ignore_errors=True)
print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
sys.exit(1 if FAILS else 0)
