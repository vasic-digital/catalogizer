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
WI = os.environ.get("WI", os.path.join(ROOT, "constitution/scripts/workable-items/bin/workable-items-linux"))
PLAN_SRC = "plan-document seeds (docs/21 section 9.1)"
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
print("# identity: task=T170 utc=%s uid=%d git_head=%s" % (time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), os.getuid(), sh(["git", "-C", ROOT, "rev-parse", "HEAD"]).stdout.decode().strip()))
print("# container: %s" % ("in-container run (container=%s) image=IMG-TESTUTIL" % os.environ.get("container", "unset") if inc else "host-side run (no container marker); a container leg is UNCONFIRMED for this transcript"))
print("# importer=%s sha256=%s enumerator sha256=%s engine sha256=%s" % (IMP, sha(IMP)[:16] if os.path.isfile(IMP) else "ABSENT", sha(ENUM)[:16] if os.path.isfile(ENUM) else "ABSENT", sha(WI)[:16]))
if not os.path.isfile(IMP):
    check("importer present: %s (RED: T170 not implemented)" % IMP, False)
    print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
    sys.exit(1)

SCR = tempfile.mkdtemp(prefix="seedimp.", dir=os.environ.get("SCRATCH_BASE") or os.environ.get("TMPDIR"))
DOC21 = F.DOCS + "/21-master-plan-phases-risks-and-traceability.md"


def decoys(t):
    """two decoy lines in the doc07 fixture: an id mention in an EARLIER section than the cited one (a locator that ignores the cited section finds it first) and a row
    whose id merely STARTS with the seed id (`C10` before `C1`: a locator without the id-boundary rule matches it)"""
    p_ = os.path.join(t, F.DOCS, "07-backend-catalog-api-audit-plan.md")
    s_ = open(p_).read()
    a_ = "# doc07\n\n## 9. Findings\n\n| ID | Finding | Severity |\n|---|---|---|\n"
    assert a_ in s_
    open(p_, "w").write(s_.replace(a_, "# doc07\n\n## 1. Overview\n\n| ID | Finding | Severity |\n|---|---|---|\n| C1 | Decoy of the overview section, not the cited one | Low |\n\n## 9. Findings\n\n"
                                    "| ID | Finding | Severity |\n|---|---|---|\n| C10 | Decoy whose id only starts with the seed id | Low |\n", 1))


def mktree(name, edit=None):
    t = os.path.join(SCR, name)
    W.build(t)
    F.plan_docs(t)
    decoys(t)
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
        ("a (document, id) pair repeated in another row (counts consistent)", lambda t: _sub(t, "| **Total itemised** | | | **27** | |", "| doc15 §13 again | S-01 | security | 1 | unrated |\n| **Total itemised** | | | **28** | |"), "seed_duplicate"),
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
check("(F7) the reconciliation record carries the freeze identity (listing hash and manifest hash of THIS freeze) and `fixture` true (the head is not a full git sha)",
      rec.get("fixture") is True and (rec.get("freeze") or {}).get("listing_sha256") == json.load(open(fj0)).get("listing_sha256")
      and (rec.get("freeze") or {}).get("manifest_sha256") == __import__("hashlib").sha256(open(json.load(open(fj0))["manifest"], "rb").read()).hexdigest(), rec.get("freeze"))
# (F7) negative control: the `fixture` marker is DERIVED from the freeze head, so a freeze whose head is a full 40-hex git sha reads as NOT a fixture (a marker that is always true is no marker)
_d = json.load(open(fj0))
_d["head"] = "a" * 40
fjh = fj0 + ".head40.json"
json.dump(_d, open(fjh, "w"))
dbh = load_sql(fresh_db("head40"), out0)
rh = imp(dbh, fjh, "seeds", "--dry-run")
rech = json.load(open(dbh + ".rec.json")) if os.path.exists(dbh + ".rec.json") else {}
check("(F7) control: a freeze with a 40-hex head gives `fixture` false and records that head", rh.returncode == 0 and rech.get("fixture") is False and (rech.get("freeze") or {}).get("head") == "a" * 40, (rh.returncode, rh.stderr[-200:], rech.get("fixture")))
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

# ---- (b3) the extra seed of a row keeps a slug id, and each entry carries its rated / unrated status
stale = q(db1, "SELECT e.locator, e.legacy_id, e.raw_status FROM reg_source_entries e WHERE e.locator LIKE '%#seed:doc12:%stale%'")
check("(b3) the seed `plus the stale constitution index hash` has the slug id doc12:stale-constitution-index-hash (the leading `the` dropped)",
      stale == [("%s#seed:doc12:stale-constitution-index-hash" % DOC21, "doc12:stale-constitution-index-hash", "unrated")], stale)
check("(b3) every entry carries its status: rated seeds `rated`, unrated seeds `unrated` (the severity distribution cell of the row)",
      q(db1, "SELECT e.raw_status, count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator=? GROUP BY 1 ORDER BY 1", PLAN_SRC) == [("rated", 7), ("unrated", 20)])
rated_n = q(db1, "SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator=? AND e.raw_status='rated'", PLAN_SRC)[0][0]
check("(b3) the rated count equals the seeds of the rated rows (doc07 4 + doc08 3 = 7 in the fixture)", rated_n == 7, rated_n)

# ---- (b4) doc20 `T-N` seeds: the labels name the N-th data row of the cited section's first table
def _doc20(t):
    p_ = os.path.join(t, DOC21)
    s_ = open(p_).read()
    open(p_, "w").write(s_.replace("| **Total itemised** | | | **27** | |", "| doc20 §2.2 | T-1..T-2 | security tooling | 2 | unrated |\n| **Total itemised** | | | **29** | |"))
    W.w(t, F.DOCS + "/20-research-second-pass-gaps.md", "# doc20\n\n## 2.2 Tools\n\n| Tool | Why |\n|---|---|\n| first-tool | the first data row |\n| second-tool | the second data row |\n")


T20 = mktree("doc20", _doc20)
fj20, out20, r = enumerate_tree(T20, "doc20")
db20 = load_sql(fresh_db("doc20"), out20) if r.returncode == 0 else None
t2 = q(db20, "SELECT e.title FROM reg_source_entries e WHERE e.locator LIKE '%#seed:doc20:T-2'") if db20 else []
check("(b4) the doc20 seed T-2 is located at the SECOND data row of the cited table, T-1 at the first", r.returncode == 0 and t2 and "second data row" in str(t2[0]) and "first data row" in str(q(db20, "SELECT e.title FROM reg_source_entries e WHERE e.locator LIKE '%#seed:doc20:T-1'")[0]), (r.returncode, t2, r.stderr[-200:]))

# ---- (b5) an import that leaves an entry unmapped is a failure, never a quiet success
dbu = load_sql(fresh_db("unmapped"), out0)
sh(["sqlite3", dbu, "CREATE TRIGGER t170_ignore BEFORE INSERT ON reg_source_map WHEN NEW.entry_id=(SELECT entry_id FROM reg_source_entries WHERE locator LIKE '%#seed:doc09:S-02') BEGIN SELECT RAISE(IGNORE); END;"])
r = imp(dbu, fj0)
check("(b5) a mapping that the register silently drops leaves an entry unmapped: the import exits 1 `incomplete`, it does not report success", r.returncode == 1 and b"incomplete" in r.stderr, (r.returncode, r.stderr[-250:]))

# ---- (b6) an engine whose validation fails after the import is a failure, never a quiet success
_wrap = os.path.join(SCR, "wi_validate_fails.sh")
open(_wrap, "w").write('#!/bin/sh\nif [ "$1" = validate ]; then echo "validate: planted failure" >&2; exit 3; fi\nexec "%s" "$@"\n' % WI)
os.chmod(_wrap, 0o755)
dbv = load_sql(fresh_db("validatefail"), out0)
rv = sh([IMP, "--class", "seeds", "--db", dbv, "--freeze-json", fj0, "--engine", _wrap, "--report", dbv + ".run.json"])
check("(b6) the engine validation failing after the import: exit 1 `validate_failed`", rv.returncode == 1 and b"validate_failed" in rv.stderr, (rv.returncode, rv.stderr[-250:]))

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

# ---- (b2) the description cap (doc04 / T168: at most 2,048 bytes, the Sources block included, cut on a character boundary and marked) on the importer's own assembler
import importlib.util   # noqa: E402
_spec = importlib.util.spec_from_file_location("import_plan_under_test", os.path.join(os.path.dirname(IMP), "import_plan.py"))
_mod = importlib.util.module_from_spec(_spec)
sys.dont_write_bytecode = True
_spec.loader.exec_module(_mod)
_long = ("\u00e9\u00e8 a very long explanation " * 200)
_d, _cut = _mod.assemble(_long, ["- File: docs/x.md", "- sha256: " + "a" * 64], "**Disposition:** pending")
_d2, _cut2 = _mod.assemble("short body", ["- File: docs/x.md"], "")
check("(b2) assemble: a body of %d bytes is cut to a description within 2,048 bytes on a character boundary, marked as cut, the Sources block and the fixed tail kept" % len(_long.encode()),
      _cut is True and len(_d.encode("utf-8")) <= 2048 and "is cut at" in _d and "**Sources**" in _d and "**Disposition:** pending" in _d and _d.encode("utf-8").decode("utf-8") == _d, (_cut, len(_d.encode())))
check("(b2) assemble: a short body is kept whole and not marked cut", _cut2 is False and "is cut at" not in _d2 and _d2.startswith("short body"), _d2)

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

# ---- (F3) a member named twice INSIDE one family is refused (review fix round 2): it would count as folded twice, shrink the bound and make the planner plan more items than its bound
def _dup_in_family(t):
    _sub(t, "| Stubs | doc01 O-01; doc07 C3 |", "| Stubs | doc01 O-01; doc07 C3; doc07 C3 |")
    _sub(t, "These 5 families list 15 member ids", "These 5 families list 16 member ids")      # the stated count is the raw token count, so the count check passes


T = mktree("dupfam", _dup_in_family)
fjd_, outd_, rd_ = enumerate_tree(T, "dupfam")
dbd_ = load_sql(fresh_db("dupfam"), outd_) if rd_.returncode == 0 else None
r2 = imp(dbd_, fjd_) if dbd_ else None
check("(F3) a member named twice inside one family (the stated member count adjusted, the enumerator passes) is refused `family_member_repeated` (exit 20) naming the family and the member, nothing written",
      r2 is not None and r2.returncode == 20 and b"family_member_repeated" in r2.stderr and b"doc07 C3" in r2.stderr and b"Stubs" in r2.stderr and q(dbd_, "SELECT count(*) FROM items")[0][0] == 0,
      (rd_.returncode, r2 and r2.returncode, r2 and r2.stderr[-300:]))
check("(F3) the refusal is the NEW one: the same input does not reach the item-bound check (`item_bound_exceeded` is what the unfixed planner said)", r2 is not None and b"item_bound_exceeded" not in r2.stderr)

# ---- the item bound has two sources: the one derived from the tables and the number docs/21 STATES ("fold to at most N register items"); a plan above either is refused,
# and for every accepted layout the planned item count EQUALS the derived bound (tested on the baseline and on three different accepted family layouts, the stated number
# kept in step with each layout, as the real docs/21 keeps it)
STATED = "The 27 itemised seeds fold to at most 18 register items before any further deduplication (27 minus 14 plus 5)."


def _layout(variant):
    def edit(t):
        if variant == "drop_family":
            _sub(t, "| Stubs | doc01 O-01; doc07 C3 |\n", "")
            _sub(t, "These 5 families list 15 member ids", "These 4 families list 13 member ids")
            _sub(t, STATED, "The 27 itemised seeds fold to at most 19 register items before any further deduplication (27 minus 13 plus 4).")
        elif variant == "add_family":
            _sub(t, "| Stubs | doc01 O-01; doc07 C3 |", "| Stubs | doc01 O-01; doc07 C3 |\n| Web pair | doc08 WEB-F01; doc09 S-01 |")
            _sub(t, "These 5 families list 15 member ids", "These 6 families list 17 member ids")
            _sub(t, STATED, "The 27 itemised seeds fold to at most 17 register items before any further deduplication (27 minus 15 plus 6).")
        elif variant == "grow_family":
            _sub(t, "| Stubs | doc01 O-01; doc07 C3 |", "| Stubs | doc01 O-01; doc07 C3; doc08 WEB-F01 |")
            _sub(t, "These 5 families list 15 member ids", "These 5 families list 16 member ids")
            _sub(t, STATED, "The 27 itemised seeds fold to at most 17 register items before any further deduplication (27 minus 15 plus 5).")
    return edit


for variant in ("baseline", "drop_family", "add_family", "grow_family"):
    T = mktree("lay_" + variant, _layout(variant) if variant != "baseline" else None)
    fjl_, outl_, rl_ = enumerate_tree(T, "lay_" + variant)
    dbl_ = load_sql(fresh_db("lay_" + variant), outl_)
    r2 = imp(dbl_, fjl_)
    recl = json.load(open(dbl_ + ".rec.json")) if os.path.exists(dbl_ + ".rec.json") else {}
    check("(F3) layout %s: accepted, and the planned item count EQUALS the bound (%s items, bound %s)" % (variant, recl.get("items_planned"), recl.get("items_upper_bound")),
          r2.returncode == 0 and recl.get("items_planned") == recl.get("items_upper_bound") and recl.get("items_planned") is not None, (r2.returncode, r2.stderr[-200:], recl.get("items_planned"), recl.get("items_upper_bound")))

# ---- (F3) the number docs/21 states is an independent oracle: the mutant that removes the bound check is killed by a document whose stated number is below the plan
def _stated_low(t):
    _sub(t, STATED, "The 27 itemised seeds fold to at most 17 register items before any further deduplication (27 minus 14 plus 5).")


def _stated_conflict(t):
    _sub(t, STATED, STATED + "\nThe register holds at most 19 register items after folding to at most 19 register items.")


for nm, fn, reason in (("stated_low", _stated_low, b"item_bound_exceeded"), ("stated_conflict", _stated_conflict, b"declared_bound_conflict")):
    T = mktree(nm, fn)
    fjs_, outs_, rs_ = enumerate_tree(T, nm)
    dbs_ = load_sql(fresh_db(nm), outs_) if rs_.returncode == 0 else None
    r3 = imp(dbs_, fjs_) if dbs_ else None
    check("(F3) %s: the plan (18 items, derived bound 18) is refused `%s` (exit 20), nothing written" % (nm, reason.decode()),
          r3 is not None and r3.returncode == 20 and reason in r3.stderr and q(dbs_, "SELECT count(*) FROM items")[0][0] == 0, (rs_.returncode, r3 and r3.returncode, r3 and r3.stderr[-300:]))

shutil.rmtree(SCR, ignore_errors=True)
print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
sys.exit(1 if FAILS else 0)
