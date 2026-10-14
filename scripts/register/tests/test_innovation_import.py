#!/usr/bin/env python3
"""test_innovation_import.py - T171 (WP-20): innovation entries, source class S-27 (docs/21 section 9.5; docs/18 candidates): the Stage 0 rule of enumerate_sources.py and the
importer scripts/register/import_plan.sh --class innovation. Real SQLite (apply_ext.sh), the real engine binary, scratch databases and scratch snapshots only.
Env: IMPORT_PLAN (importer under test; mutation runs point it at a mutated copy beside copies of import_tickets.py and enumerate_sources.py), ENUMERATE_SOURCES, WI, SCRATCH_BASE.
Checks (each is a failure class of the task text, T171):
  (a) planted-candidate completeness (the RED of T171): ONE entry added to docs/21 section 9.5 (with its docs/18 candidate row) of a scratch snapshot yields exactly ONE new
      reg_source_entries row, removing it restores the sql byte for byte; a docs/18 candidate row WITHOUT a 9.5 entry adds no entry and is reported by the reconciliation as
      a coverage mismatch (a requirement is never lost silently); a Counts line that disagrees, a duplicate entry, an unknown kind and a missing section are refused
  (b) the import: one Feature item per entry, status Queued, category gap, mint_basis import, component_id from nothing (NULL), relation primary / import_1to1, no reg_findings row,
      descriptions <= 2,048 bytes with a Sources block, the docs/18 row, the evidence-label line and the Disposition (ODG-39) pending line, ranked game changer + experiment where docs/18 has one
  (c) reconciliation: every docs/18 candidate covered (R-ADJ only through S-26, PROPOSAL only through S-27, mixed through both), per-class counts, no mismatch
  (d) idempotent, resumable (a crash after each stage, then a re-run, gives the state of a clean run)
  (e) refusals with nothing written: snapshot changed after the manifest, entries older than the snapshot, docs/18 absent from the snapshot, a candidate row missing for an entry
  (f) the register is never written outside the 4 entries: the existing S-26 items are unchanged by the innovation import
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
print("# identity: task=T171 utc=%s uid=%d git_head=%s" % (time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), os.getuid(), sh(["git", "-C", ROOT, "rev-parse", "HEAD"]).stdout.decode().strip()))
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


def imp(db, fj, klass="innovation", *extra, env=None):
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
    return json.load(open(os.path.join(out, "enumeration-stats.json")))["classes"]["S-27"]["entries"]


DOC18 = F.DOCS + "/18-research-product-innovation-and-game-changers.md"
FEAT_Q = "SELECT i.atm_id, i.type, i.status, i.description FROM items i JOIN reg_source_map m ON m.atm_id = i.atm_id JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator = ? ORDER BY e.locator"


def _sub(t, rel, a, b):
    pth = os.path.join(t, rel)
    s = open(pth).read()
    assert a in s, a
    open(pth, "w").write(s.replace(a, b, 1))


def plant_both(t):
    _sub(t, DOC21, "| T7-C | `PROPOSAL` part |", "| T1-C | `PROPOSAL` | planted candidate | - |\n| T7-C | `PROPOSAL` part |")
    _sub(t, DOC21, "Counts: 4 entries (2 `PROPOSAL`", "Counts: 5 entries (3 `PROPOSAL`")
    _sub(t, DOC18, "| T1-B | A manual Identify", "| T1-C | A planted candidate row. | PROPOSAL | catalog-api | S | Low | Low | PROVEN (9.9) |\n| T1-B | A manual Identify")


# ---- (a) planted-candidate completeness + the table controls
BASE = mktree("base")
fj0, out0, r = enumerate_tree(BASE, "base")
check("(a) baseline enumeration exits 0", r.returncode == 0, r.stdout + r.stderr)
N0 = n_entries(out0)
check("(a) S-27 reads 4 entries on the fixture (a class that reads zero is a blind instrument)", N0 == 4, N0)
T = mktree("plant", plant_both)
fjp, outp, r = enumerate_tree(T, "plant")
check("(a) one candidate added to docs/21 section 9.5 and docs/18 of a scratch copy -> exactly ONE new S-27 entry", r.returncode == 0 and n_entries(outp) == N0 + 1, (r.returncode, r.stderr[-200:]))
dbb = load_sql(fresh_db("plant_base"), out0)
dbp = fresh_db("plant_planted")
load_sql(dbp, outp)
check("(a) the register gains exactly ONE new reg_source_entries row (by natural key) and it is the planted candidate",
      q(dbp, "SELECT count(*) FROM reg_source_entries")[0][0] - q(dbb, "SELECT count(*) FROM reg_source_entries")[0][0] == 1
      and q(dbp, "SELECT count(*) FROM reg_source_entries WHERE locator LIKE '%#innovation:T1-C'")[0][0] == 1)


def unplant(t):
    plant_both(t)
    _sub(t, DOC21, "| T1-C | `PROPOSAL` | planted candidate | - |\n", "")
    _sub(t, DOC21, "Counts: 5 entries (3 `PROPOSAL`", "Counts: 4 entries (2 `PROPOSAL`")
    _sub(t, DOC18, "| T1-C | A planted candidate row. | PROPOSAL | catalog-api | S | Low | Low | PROVEN (9.9) |\n", "")


T = mktree("plant_removed", unplant)
fjr, outr, r = enumerate_tree(T, "plant_removed")
check("(a) the planted candidate removed again: the sql equals the baseline byte for byte", r.returncode == 0 and sha(os.path.join(outr, "source_entries.sql")) == sha(os.path.join(out0, "source_entries.sql")), r.stderr[-200:])


def plant_doc18_only(t):
    _sub(t, DOC18, "| T1-B | A manual Identify", "| T1-C | A planted candidate row. | PROPOSAL | catalog-api | S | Low | Low | PROVEN (9.9) |\n| T1-B | A manual Identify")


T = mktree("plant18", plant_doc18_only)
fj18, out18, r = enumerate_tree(T, "plant18")
check("(a) a docs/18 candidate with NO docs/21 section 9.5 entry adds no S-27 entry (the entry set is docs/21's)", r.returncode == 0 and n_entries(out18) == N0, (r.returncode, r.stderr[-200:]))
db18 = load_sql(fresh_db("plant18"), out18)
imp(db18, fj18, "seeds")
r = imp(db18, fj18)
rec18 = json.load(open(db18 + ".rec.json")) if os.path.exists(db18 + ".rec.json") else {}
check("(a) ... and the reconciliation reports it as a coverage mismatch (candidate T1-C in neither S-26 nor S-27: a requirement is never lost silently)",
      r.returncode == 0 and any(m_["id"] == "T1-C" and not m_["in_S27"] for m_ in rec18.get("coverage_mismatches", [])) and "T1-C" in rec18.get("coverage", {}).get("neither", []), (r.returncode, rec18.get("coverage_mismatches")))
for label, edit, reason in (
        ("a Counts line that disagrees with the table", lambda t: _sub(t, DOC21, "Counts: 4 entries (2 `PROPOSAL`", "Counts: 5 entries (3 `PROPOSAL`"), "innovation_count_mismatch"),
        ("a duplicate entry id", lambda t: _sub(t, DOC21, "| T2-B | `PROPOSAL` |", "| T1-B | `PROPOSAL` |"), "innovation_duplicate"),
        ("an unknown kind", lambda t: _sub(t, DOC21, "| T1-B | `PROPOSAL` |", "| T1-B | `IDEA` |"), "innovation_kind_unknown"),
        ("a missing section", lambda t: _sub(t, DOC21, "### 9.5 Innovation candidates", "### 9.6 Innovation candidates"), "innovation_section_missing")):
    nm = "bad_" + re.sub(r"\W", "", label)[:12]
    tt = mktree(nm, edit)
    fjx, outx, r = enumerate_tree(tt, nm)
    check("(a) %s: the enumerator refuses `%s` (exit 20), nothing written" % (label, reason), r.returncode == 20 and reason.encode() in r.stderr and not os.path.exists(os.path.join(outx, "source_entries.sql")), (r.returncode, r.stderr[-200:]))

# ---- (b) the import (the S-26 seeds first, as in the real order T170 -> T171)
db1 = load_sql(fresh_db("main"), out0)
rs = imp(db1, fj0, "seeds")
check("(b) the S-26 import that T171 depends on exits 0", rs.returncode == 0, rs.stdout + rs.stderr)
seed_state = q(db1, "SELECT atm_id, type, status, severity, title, description FROM items ORDER BY atm_id")
seed_map = q(db1, "SELECT e.locator, m.atm_id, m.relation FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) ORDER BY e.locator")
r = imp(db1, fj0)
check("(b) the innovation import exits 0", r.returncode == 0, r.stdout + r.stderr)
rec = json.load(open(db1 + ".rec.json")) if os.path.exists(db1 + ".rec.json") else {}
check("(F7) the reconciliation record carries the freeze identity (listing hash and manifest hash of THIS freeze) and `fixture` true (the head is not a full git sha)",
      rec.get("fixture") is True and (rec.get("freeze") or {}).get("listing_sha256") == json.load(open(fj0)).get("listing_sha256")
      and (rec.get("freeze") or {}).get("manifest_sha256") == __import__("hashlib").sha256(open(json.load(open(fj0))["manifest"], "rb").read()).hexdigest(), rec.get("freeze"))
rows = q(db1, FEAT_Q, INNO_SRC)
check("(b) 4 Feature items (28-style PROPOSAL entries and PROPOSAL parts each their own item), all Queued", len(rows) == 4 and {(x[1], x[2]) for x in rows} == {("Feature", "Queued")}, [(x[0], x[1], x[2]) for x in rows])
check("(b) every entry of the source is mapped (v_unmapped_entries empty for S-27)", q(db1, "SELECT count(*) FROM v_unmapped_entries WHERE source=?", INNO_SRC)[0][0] == 0)
check("(b) relation primary, match_basis import_1to1, one severity_governs row per item",
      q(db1, "SELECT m.relation, m.match_basis, count(*) FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.locator=? GROUP BY 1,2", INNO_SRC) == [("primary", "import_1to1", 4)]
      and q(db1, "SELECT count(*) FROM (SELECT atm_id, sum(severity_governs) s FROM reg_source_map GROUP BY atm_id) WHERE s <> 1")[0][0] == 0)
check("(b) category gap, defect_layer source, custody machine_evidence, component_id NULL, no reg_findings row, mint_basis import",
      q(db1, "SELECT DISTINCT x.category||'/'||x.defect_layer||'/'||x.custody_basis||'/'||x.reverify_required FROM reg_item_ext x JOIN items i USING(atm_id) WHERE i.type='Feature'") == [("gap/source/machine_evidence/0",)]
      and q(db1, "SELECT count(*) FROM reg_item_ext x JOIN items i USING(atm_id) WHERE i.type='Feature' AND x.component_id IS NOT NULL")[0][0] == 0
      and q(db1, "SELECT count(*) FROM reg_findings")[0][0] == 0
      and q(db1, "SELECT count(*) FROM reg_ids r JOIN items i USING(atm_id) WHERE i.type='Feature' AND r.mint_basis='import'")[0][0] == 4)
check("(b) every description is within 2,048 bytes, carries the Sources block with the S-27 locator, its sha256, the docs/18 row line and the Disposition (ODG-39) pending line",
      all(len(d.encode("utf-8")) <= 2048 and "**Sources**" in d and "#innovation:" in d and re.search(r"sha256: [0-9a-f]{64}", d) and "- docs/18 row: " in d and ":L" in d
          and d.rstrip().endswith("**Disposition (ODG-39):** pending") and "not a finding, defect or task of feature 001" in d for (_a, _t, _s, d) in rows))
d_t2b = [x[3] for x in rows if "#innovation:T2-B" in x[3]][0]
check("(b) T2-B (rank 1) records its ranked game changer and the falsifiable experiment E1 of docs/18 section 13.1", "Ranked game changer (docs/18 section 13, rank 1)" in d_t2b and "E1" in d_t2b and "precision of auto-accepted" in d_t2b, d_t2b[:900])
d_t1b = [x[3] for x in rows if "#innovation:T1-B" in x[3]][0]
check("(b) T1-B (no ranked game changer) records none and says its evidence label maps to a register value (docs/18 section 2.1)", "Ranked game changer" not in d_t1b and "Evidence label: PROVEN (1.3) -> register value PROVEN" in d_t1b, d_t1b[:900])
check("(b) the PROPOSAL parts T5-D and T7-C are marked as parts in the locator and each has its own item",
      sum(1 for x in rows if ":PROPOSAL-part" in x[3]) >= 2 and len({x[0] for x in rows}) == 4)
check("(b) the engine validate, foreign_key_check and integrity_check are clean", sh([WI, "validate", "--db", db1]).returncode == 0 and q(db1, "PRAGMA foreign_key_check") == [] and q(db1, "PRAGMA integrity_check") == [("ok",)])

# ---- (c) reconciliation
check("(c) reconciliation: 6 candidates read, tag classes 2 PROPOSAL / 2 R-ADJ / 2 mixed, coverage 2 S-26 only / 2 S-27 only / 2 both / 0 neither, no mismatch, no reg_findings row",
      rec.get("candidates_read") == 6 and rec.get("tag_classes") == {"PROPOSAL": 2, "R-ADJ": 2, "mixed": 2, "none": 0}
      and rec.get("coverage_counts") == {"S-26 only": 2, "S-27 only": 2, "both": 2, "neither": 0} and rec.get("coverage_mismatches") == [] and rec.get("no_reg_findings_row") is True
      and rec.get("by_kind") == {"PROPOSAL": 2, "PROPOSAL part": 2}, rec)

# ---- (f) the S-26 items are untouched by the innovation import
check("(f) the S-26 items and map rows written before are byte-identical after the innovation import",
      [x for x in q(db1, "SELECT atm_id, type, status, severity, title, description FROM items ORDER BY atm_id") if x[1] != "Feature"] == [x for x in seed_state if x[1] != "Feature"]
      and [x for x in q(db1, "SELECT e.locator, m.atm_id, m.relation FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) ORDER BY e.locator") if "#innovation:" not in x[0]] == seed_map)

# ---- (d) idempotent + resumable
s1 = state(db1)
r = imp(db1, fj0)
check("(d) a second run exits 0 and writes nothing (state identical, no new id)", r.returncode == 0 and state(db1) == s1)
for stage in ("after_mint", "after_ext", "after_add", "before_map"):
    for nth in (1, 3):
        db = load_sql(fresh_db("fault"), out0)
        imp(db, fj0, "seeds")
        base_state = state(db)
        r1 = imp(db, fj0, env={"IMPORT_TEST_MODE": "1", "IMPORT_FAULT": "%s:%d" % (stage, nth)})
        partial = state(db)
        r2 = imp(db, fj0)
        check("(d) crash %s at group %d (exit 137), then a re-run completes with the state of a clean run" % (stage, nth), r1.returncode == 137 and partial != s1 and r2.returncode == 0 and state(db) == s1, (r1.returncode, r2.returncode, r2.stderr[-200:]))

# ---- (e) refusals, nothing written
dbf = load_sql(fresh_db("moved"), out0)
T = mktree("movedtree")
fjm = os.path.join(SCR, "movedtree.freeze.json")
F.freeze2(T, fjm)
with open(os.path.join(T, DOC18), "a") as f:
    f.write("\nsneaked in\n")
r = imp(dbf, fjm)
check("(e) a snapshot changed after its manifest: refused `freeze_snapshot_moved` (exit 20), nothing written", r.returncode == 20 and b"freeze_snapshot_moved" in r.stderr and q(dbf, "SELECT count(*) FROM items")[0][0] == 0, (r.returncode, r.stderr[-200:]))
T = mktree("newer", lambda t: _sub(t, DOC18, "| T1-B | A manual Identify / re-match action with provider-id input on every client.", "| T1-B | A manual Identify / re-match action, reworded after the enumeration."))
fjn = os.path.join(SCR, "newer.freeze.json")
F.freeze2(T, fjn)
dbn = load_sql(fresh_db("newer"), out0)
r = imp(dbn, fjn)
check("(e) entries older than the snapshot (docs/18 row changed after enumeration): refused `entries_out_of_date` naming the entry, nothing written",
      r.returncode == 20 and b"entries_out_of_date" in r.stderr and b"T1-B" in r.stderr and q(dbn, "SELECT count(*) FROM items")[0][0] == 0, (r.returncode, r.stderr[-300:]))


def drop18(t):
    os.remove(os.path.join(t, DOC18))


T = mktree("no18", drop18)
fj_n18 = os.path.join(SCR, "no18.freeze.json")
F.freeze2(T, fj_n18)
dbm = load_sql(fresh_db("no18"), out0)
r = imp(dbm, fj_n18)
check("(e) docs/18 absent from the snapshot: refused (exit 20), nothing written", r.returncode == 20 and q(dbm, "SELECT count(*) FROM items")[0][0] == 0, (r.returncode, r.stderr[-300:]))

# ---- (b2) the evidence-label mapping of docs/18 section 2.1 and the Feature item severity
def labels(t):
    _sub(t, DOC18, "| catalog-api, catalog-web, android | M | High | Low | PROVEN (1.3) |", "| catalog-api, catalog-web, android | M | High | Low | EMERGING (secondary source: vendor blog) |")
    _sub(t, DOC18, "| catalog-api DB, API | M | High | Low | PROVEN components (1.4, 1.7); integrated form not found |", "| catalog-api DB, API | M | High | Low | n/a (defect measured in this repository) |")


T = mktree("labels", labels)
fjl, outl, r = enumerate_tree(T, "labels")
dbl = load_sql(fresh_db("labels"), outl)
imp(dbl, fjl, "seeds")
r = imp(dbl, fjl)
rl = json.load(open(dbl + ".rec.json")) if os.path.exists(dbl + ".rec.json") else {}
dl = {x[3].split("#innovation:")[1].split("\n")[0]: x[3] for x in q(dbl, FEAT_Q, INNO_SRC)}
check("(b2) EMERGING (secondary source) is recorded PLAUSIBLE with the single-source qualifier (docs/18 section 2.1)",
      r.returncode == 0 and "register value PLAUSIBLE + single_source" in dl.get("T1-B", "") and rl.get("single_source") == 1 and rl.get("by_evidence_label", {}).get("PLAUSIBLE") == 1, (r.returncode, rl.get("by_evidence_label"), dl.get("T1-B", "")[-500:]))
check("(b2) n/a is recorded n/a and the repository citation is stated", "register value n/a" in dl.get("T2-B", "") and "repository citation" in dl.get("T2-B", ""), dl.get("T2-B", "")[-500:])
check("(b2) Feature items carry severity low with a note that a proposal has no severity, and the Type/category stay Feature / gap",
      q(db1, "SELECT DISTINCT x.severity FROM reg_item_ext x JOIN items i USING(atm_id) WHERE i.type='Feature'") == [("low",)]
      and all("proposal, not a defect" in (v[0] or "") for v in q(db1, "SELECT x.severity_source FROM reg_item_ext x JOIN items i USING(atm_id) WHERE i.type='Feature'")))

# ---- (e2) candidate rows
T = mktree("dupcand", lambda t: _sub(t, DOC18, "| T1-B | A manual Identify", "| T1-B | A duplicate row. | PROPOSAL | catalog-api | S | Low | Low | PROVEN (9.9) |\n| T1-B | A manual Identify"))
fjd, outd, r = enumerate_tree(T, "dupcand")
check("(e2) a docs/18 candidate id that occurs twice: the enumerator refuses `candidate_duplicate` (exit 20), nothing written",
      r.returncode == 20 and b"candidate_duplicate" in r.stderr and not os.path.exists(os.path.join(outd, "source_entries.sql")), (r.returncode, r.stderr[-200:]))


def drop_cand(t):
    pth = os.path.join(t, DOC18)
    s_ = open(pth).read()
    out_ = "\n".join(l for l in s_.split("\n") if not l.startswith("| T1-B | A manual Identify"))
    assert out_ != s_
    open(pth, "w").write(out_)


T = mktree("nocand", drop_cand)
fjc, outc, r = enumerate_tree(T, "nocand")
dbc = load_sql(fresh_db("nocand"), outc) if r.returncode == 0 else None
r2 = imp(dbc, fjc) if dbc else r
check("(e2) an entry whose docs/18 candidate row is missing: refused `candidate_row_missing` naming T1-B (exit 20), nothing written",
      r2.returncode == 20 and b"candidate_row_missing" in r2.stderr and b"T1-B" in r2.stderr and (not dbc or q(dbc, "SELECT count(*) FROM items")[0][0] == 0), (r2.returncode, r2.stderr[-250:]))

# ---- (e3) a candidate row that is not a complete row of its table is no candidate row
def short_row(t):
    _sub(t, DOC18, "| T1-B | A manual Identify / re-match action with provider-id input on every client. | PROPOSAL | catalog-api, catalog-web, android | M | High | Low | PROVEN (1.3) |",
         "| T1-B | A manual Identify / re-match action with provider-id input on every client. | PROPOSAL | catalog-api, catalog-web, android | M | High | PROVEN (1.3) |")


T = mktree("shortrow", short_row)
fjs, outs, r = enumerate_tree(T, "shortrow")
dbs = load_sql(fresh_db("shortrow"), outs) if r.returncode == 0 else None
r2 = imp(dbs, fjs) if dbs else r
check("(e3) a docs/18 row with a missing cell is no candidate row: the entry is refused `candidate_row_missing` naming T1-B (exit 20), nothing written",
      r2.returncode == 20 and b"candidate_row_missing" in r2.stderr and b"T1-B" in r2.stderr and (not dbs or q(dbs, "SELECT count(*) FROM items")[0][0] == 0), (r2.returncode, r2.stderr[-250:]))

# ---- (a2) the entry status and the enumeration statistics
check("(a2) each S-27 entry carries its kind as its status (PROPOSAL / PROPOSAL part) and the legacy id doc18:<id>",
      q(db1, "SELECT e.legacy_id, e.raw_status FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator=? ORDER BY 1", INNO_SRC)
      == [("doc18:T1-B", "PROPOSAL"), ("doc18:T2-B", "PROPOSAL"), ("doc18:T5-D", "PROPOSAL part"), ("doc18:T7-C", "PROPOSAL part")])
stats = json.load(open(os.path.join(out0, "enumeration-stats.json")))
check("(a2) the enumeration statistics record the 6 docs/18 candidates read and no entry without a docs/18 row", stats["classes"]["S-27"]["innovation"].get("doc18_candidates_read") == 6
      and stats["classes"]["S-27"]["innovation"].get("entry_without_doc18_row") == [] and stats["classes"]["S-27"]["innovation"].get("proposal") == 2
      and stats["classes"]["S-27"]["innovation"].get("proposal_part") == 2, stats["classes"]["S-27"])

# ---- (F2) a COMPOUND docs/18 label (docs/18 section 2.1 has no row for it) takes its WEAKEST part: it never states more evidence than its weakest stated part
def compound(t):
    _sub(t, DOC18, "| PROPOSAL | catalog-api, catalog-web, android | M | High | Low | PROVEN (1.3) |", "| PROPOSAL | catalog-api, catalog-web, android | M | High | Low | PROVEN backup step; SPECULATIVE guard and rollback |")
    _sub(t, DOC18, "| Medium | Medium | PROVEN resume pattern (5.4) |", "| Medium | Medium | PROVEN resume pattern (5.4); prerequisite is a repository defect |")


T = mktree("compound", compound)
fjc, outc, r = enumerate_tree(T, "compound")
dbc = load_sql(fresh_db("compound"), outc)
imp(dbc, fjc, "seeds")
r = imp(dbc, fjc)
rc_ = json.load(open(dbc + ".rec.json")) if os.path.exists(dbc + ".rec.json") else {}
dc = {x[3].split("#innovation:")[1].split("\n")[0]: x[3] for x in q(dbc, FEAT_Q, INNO_SRC)}
dcp = lambda k: next((v for kk, v in dc.items() if kk.startswith(k)), "")      # the R-ADJ part entries are keyed `T5-D:PROPOSAL-part`
check("(F2) `PROVEN backup step; SPECULATIVE guard and rollback` -> SPECULATIVE (not PROVEN), and the description says it is a compound label whose weakest part wins",
      r.returncode == 0 and "register value SPECULATIVE (compound label PROVEN+SPECULATIVE: the weakest part wins)" in dc.get("T1-B", ""), dc.get("T1-B", "")[-600:])
check("(F2) `PROVEN components (1.4, 1.7); integrated form not found` (a clause that states no evidence level) -> UNCONFIRMED",
      "register value UNCONFIRMED (compound label PROVEN+UNCONFIRMED: the weakest part wins)" in dc.get("T2-B", ""), dc.get("T2-B", "")[-600:])
check("(F2) `PROVEN restore (7.3); UNCONFIRMED marker-row verify` -> UNCONFIRMED, and `PROVEN resume pattern (5.4); prerequisite is a repository defect` -> UNCONFIRMED (the second clause names no evidence level)",
      "register value UNCONFIRMED (compound" in dcp("T7-C") and "register value UNCONFIRMED (compound" in dcp("T5-D"), [l for l in (dcp("T7-C") + dcp("T5-D")).split("\n") if "Evidence label" in l])
check("(F2) the reconciliation counts them by the weakest label: SPECULATIVE 1, UNCONFIRMED 3, PROVEN 0", rc_.get("by_evidence_label") == {"SPECULATIVE": 1, "UNCONFIRMED": 3}, rc_.get("by_evidence_label"))

# the nine rows of the REAL docs/18 whose labels were compound (the review's list), through the real mapping function of the tool under test
import importlib.util as _iu
_sp = _iu.spec_from_file_location("import_plan_ut", os.path.join(os.path.dirname(IMP), "import_plan.py"))
_ip = _iu.module_from_spec(_sp)
_sp.loader.exec_module(_ip)
NINE = {"T2-B": ("PROVEN components; integrated form not found", "UNCONFIRMED"), "T3-B": ("PROVEN protocol; UNCONFIRMED Go client", "UNCONFIRMED"),
        "T5-D": ("PROVEN resume pattern; prerequisite is a repository defect", "UNCONFIRMED"), "T6-C": ("PROVEN standard; UNCONFIRMED client support", "UNCONFIRMED"),
        "T6-D": ("PROVEN mechanism; UNCONFIRMED version defaults", "UNCONFIRMED"), "T6-E": ("PROVEN building blocks; SPECULATIVE product", "SPECULATIVE"),
        "T7-C": ("PROVEN restore; UNCONFIRMED marker-row verify", "UNCONFIRMED"), "T8-B": ("PROVEN practice; UNCONFIRMED spec text", "UNCONFIRMED"),
        "T10-C": ("PROVEN backup step; SPECULATIVE guard and rollback", "SPECULATIVE")}
got9 = {k: _ip.evidence_label(v[0])[0] for k, v in NINE.items()}
check("(F2) all nine compound labels of the real docs/18 map to their weakest part (none PROVEN): %s" % got9, got9 == {k: v[1] for k, v in NINE.items()}, got9)
check("(F2) the single-part mappings are unchanged: PROVEN (1.3), EMERGING out-of-process (8.1) -> PLAUSIBLE, SPECULATIVE (the need is PROVEN as a complaint, 7.4) -> SPECULATIVE (a parenthesis is a citation, not a part), n/a (...) -> n/a",
      [_ip.evidence_label(x)[0] for x in ("PROVEN (1.3)", "EMERGING out-of-process (8.1)", "SPECULATIVE (the need is PROVEN as a complaint, 7.4)", "n/a (defect measured in this repository)")] == ["PROVEN", "PLAUSIBLE", "SPECULATIVE", "n/a"])

print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
sys.exit(1 if FAILS else 0)
