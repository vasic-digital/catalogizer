#!/usr/bin/env python3
"""test_recount.py - T166 (WP-20): the independent recount (scripts/register/recount_sources.sh).
Real files, real SQLite (the scratch register DB is built by apply_ext.sh and filled by the real enumerator's SQL), real grep/xargs/awk;
never the live register. Env: RECOUNT_SH (tool under test; mutation runs point it at a mutated copy), ENUMERATE_SOURCES, SCRATCH_BASE (default TMPDIR).
Checks:
  (a) the recount of an enumerated scratch tree matches EVERY source (class-exhaustive: all sources the fixture enumerates are recounted, none unrecounted)
  (b) fixture 1 (T166): a root file named `-n` holding one TODO line (its source class S-23) is counted: the recount passes; a recount that passes the path bare
      to grep (no `--`) reads `-n` as an option and FAILS (paired mutation)
  (c) fixture 2 (T166): the snapshot with one file changed after its manifest -> refused `freeze_snapshot_moved`, no recount.json written
  (d) the listing with one appended path -> refused `freeze_listing_moved` before any count, no recount.json written
  (e) control needle, one plant per source class: enumerate the BASE tree, recount a tree with ONE planted entry of the class -> exactly that class's source differs,
      by exactly one, and the differing locator is itemised; every class reads > 0 on the base tree (a blind class is a failure)
  (f) an explanation turns a difference into `explained` (exit 0); an explanation for another source does not; a source with no recount method fails
  (g) scanned_entry_count that disagrees with the rows is reported; the register database bytes are unchanged by a recount; two runs give identical recount.json
  (h) a path holding a newline is counted (grep -Z, not line splitting)
  (i) doc03 section 7 comparison: equal / delta / doc03_value_not_found
Exit 0 only when every check passed.
"""
import hashlib
import json
import os
import shutil
import sqlite3
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(REG))
sys.path.insert(0, HERE)
import w2_fixture as F   # noqa: E402
import wp20_fixture as W   # noqa: E402

TOOL = os.environ.get("RECOUNT_SH", os.path.join(REG, "recount_sources.sh"))
ENUM = os.environ.get("ENUMERATE_SOURCES", os.path.join(REG, "enumerate_sources.py"))
FAILS = PASSES = 0


def check(label, cond, info=""):
    global FAILS, PASSES
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:700]))
    if cond:
        PASSES += 1
    else:
        FAILS += 1


def sh(cmd, **kw):
    return subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kw)


def sha(path):
    return hashlib.sha256(open(path, "rb").read()).hexdigest()


def ident():
    print("# identity: task=T166 utc=%s host=%s uid=%d" % (__import__("time").strftime("%Y-%m-%dT%H:%M:%SZ", __import__("time").gmtime()), os.uname().nodename.split(".")[0], os.getuid()))
    print("# git_head=%s" % sh(["git", "-C", ROOT, "rev-parse", "HEAD"]).stdout.decode().strip())
    inc = os.path.exists("/run/.containerenv") or os.environ.get("container") == "podman"
    print("# container: %s" % ("in-container run (container=%s) image=IMG-TESTUTIL" % os.environ.get("container", "unset") if inc else "host-side run (no container marker); a container leg is UNCONFIRMED for this transcript"))
    print("# tool=%s sha256=%s engine(recount_sources.py) sha256=%s enumerator sha256=%s" % (TOOL, sha(TOOL)[:16] if os.path.isfile(TOOL) else "ABSENT",
      sha(os.path.join(os.path.dirname(TOOL), "recount_sources.py"))[:16] if os.path.isfile(os.path.join(os.path.dirname(TOOL), "recount_sources.py")) else "ABSENT", sha(ENUM)[:16]))


ident()
if not os.path.isfile(TOOL):
    check("recount tool present: %s (RED: T166 not implemented)" % TOOL, False)
    print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
    sys.exit(1)

SCR = tempfile.mkdtemp(prefix="recount.", dir=os.environ.get("SCRATCH_BASE") or os.environ.get("TMPDIR"))


def build_db(tree, name):
    """freeze2 + enumerate + a scratch register DB filled with the enumerator's SQL -> (freeze json path, db path)"""
    fj = os.path.join(SCR, name + ".freeze.json")
    F.freeze2(tree, fj)
    out = os.path.join(SCR, name + ".enum")
    r = sh([sys.executable, ENUM, "--freeze-json", fj, "--out", out])
    check("enumerate %s exits 0" % name, r.returncode == 0, r.stdout + r.stderr)
    db = os.path.join(SCR, name + ".db")
    for p in (db, db + "-wal", db + "-shm"):
        if os.path.exists(p):
            os.remove(p)
    r = sh([os.path.join(REG, "apply_ext.sh"), "--db", db])
    assert r.returncode == 0, r.stdout + r.stderr
    r = sh(["sqlite3", db, ".read " + os.path.join(out, "source_entries.sql")])
    assert r.returncode == 0 and not r.stdout + r.stderr, r.stderr
    return fj, db


def recount(fj, db, out, *extra):
    if os.path.exists(out):
        shutil.rmtree(out)
    r = sh([TOOL, "--freeze-json", fj, "--db", db, "--out", out] + list(extra))
    rec = None
    p = os.path.join(out, "recount.json")
    if os.path.exists(p):
        rec = json.load(open(p))
    return r, rec


def status_of(rec):
    return {s["locator"]: s["status"] for s in rec["sources"]}


# ---- (a) the base tree: every enumerated source is recounted and matches
BASE = os.path.join(SCR, "base")
W.build(BASE)
F.recount_extras(BASE)
fj0, db0 = build_db(BASE, "base")
r, rec = recount(fj0, db0, os.path.join(SCR, "o0"))
check("(a) base tree: recount exits 0", r.returncode == 0, r.stdout + r.stderr)
st = status_of(rec) if rec else {}
nsrc = sqlite3.connect(db0).execute("SELECT count(*) FROM reg_sources").fetchone()[0]
check("(a) every source row of the register is recounted (%d sources, none without a method, none differing)" % nsrc,
      rec and len(rec["sources"]) == nsrc and all(v == "match" for v in st.values()), {k: v for k, v in st.items() if v != "match"})
check("(a) recount.json carries the freeze identity and the verdict", rec and rec["verdict"] == "PASS" and rec["freeze"]["listing_sha256"] == json.load(open(fj0))["listing_sha256"], rec and rec["verdict"])
zero = [s["locator"] for s in rec["sources"] if s["recount"] == 0 and not s["locator"].startswith("submodules/")] if rec else ["no record"]
check("(a) control: no source of the fixture reads zero (a blind instrument reads zero everywhere)", not zero, zero)

# ---- (g) determinism, read-only, count consistency
r2, rec2 = recount(fj0, db0, os.path.join(SCR, "o0b"))
check("(g) two recounts give a byte-identical recount.json", sha(os.path.join(SCR, "o0", "recount.json")) == sha(os.path.join(SCR, "o0b", "recount.json")))
h_before = sha(db0)
recount(fj0, db0, os.path.join(SCR, "o0c"))
check("(g) the recount leaves the register database bytes unchanged", sha(db0) == h_before)
db_bad = os.path.join(SCR, "bad_count.db")
shutil.copy(db0, db_bad)
sh(["sqlite3", db_bad, "UPDATE reg_sources SET scanned_entry_count = scanned_entry_count + 1 WHERE locator='docs/status/*.md'"])
r, rec = recount(fj0, db_bad, os.path.join(SCR, "o_bad"))
check("(g) a scanned_entry_count that disagrees with the rows is a difference (exit 1, the source itemised under `<scanned_entry_count vs rows>`)",
      r.returncode == 1 and rec and status_of(rec).get("docs/status/*.md") == "diff"
      and any(d["key"] == "<scanned_entry_count vs rows>" for s in rec["sources"] if s["locator"] == "docs/status/*.md" for d in s["differences"]), (r.returncode, r.stdout[-300:]))

# ---- (b) fixture 1: a root file named -n holding one marker line
T1 = os.path.join(SCR, "dashn")
shutil.copytree(BASE, T1, symlinks=True)
W.w(T1, "-n", "// TODO the file name is an option for a bare grep\n")
fj1, db1 = build_db(T1, "dashn")
n_rows = sqlite3.connect(db1).execute("SELECT count(*) FROM reg_source_entries WHERE locator LIKE '-n:L1:%'").fetchone()[0]
check("(b) the enumerator records the marker row of the file `-n` (the fixture is real)", n_rows == 1, n_rows)
r, rec = recount(fj1, db1, os.path.join(SCR, "o1"))
check("(b) fixture 1: the recount counts the line of `-n` and passes (paths are passed after `--`)", r.returncode == 0 and rec and rec["verdict"] == "PASS", (r.returncode, r.stdout[-400:] + r.stderr[-300:]))

# ---- (c) fixture 2: snapshot changed after the manifest
T2 = os.path.join(SCR, "moved")
shutil.copytree(BASE, T2, symlinks=True)
fj2 = os.path.join(SCR, "moved.freeze.json")
F.freeze2(T2, fj2)
with open(os.path.join(T2, "MASTER_EXECUTION_CHECKLIST.md"), "a") as f:
    f.write("- [ ] sneaked in\n")
out2 = os.path.join(SCR, "o2")
r, rec = recount(fj2, db0, out2)
check("(c) fixture 2: a snapshot file changed after its manifest -> exit 20 `freeze_snapshot_moved` naming the file", r.returncode == 20 and b"freeze_snapshot_moved" in r.stderr and b"MASTER_EXECUTION_CHECKLIST.md" in r.stderr, (r.returncode, r.stderr[-300:]))
check("(c) fixture 2: no recount.json written", rec is None and not os.path.exists(os.path.join(out2, "recount.json")))

# ---- (d) listing moved
T3 = os.path.join(SCR, "lmoved")
shutil.copytree(BASE, T3, symlinks=True)
fj3 = os.path.join(SCR, "lmoved.freeze.json")
F.freeze2(T3, fj3)
with open(json.load(open(fj3))["listing"], "ab") as f:
    f.write(b"sneaked/in.md\0")
out3 = os.path.join(SCR, "o3")
r, rec = recount(fj3, db0, out3)
check("(d) listing with an appended path -> exit 20 `freeze_listing_moved`, no recount.json", r.returncode == 20 and b"freeze_listing_moved" in r.stderr and rec is None, (r.returncode, r.stderr[-300:]))

# ---- (e) control needle, one plant per class
base_counts = {s["locator"]: s["recount"] for s in json.load(open(os.path.join(SCR, "o0", "recount.json")))["sources"]}
CLASS_LOC = {  # class -> the source locator that must differ
    "S-01": "docs/issues/*.md", "S-02": "issues/*.md", "S-03": "TASK_TRACKER.md", "S-04": "MASTER_EXECUTION_CHECKLIST.md", "S-05": "docs/MASTER_EXECUTION_CHECKLIST.md",
    "S-06": "COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md", "S-07": "COMPREHENSIVE_UNFINISHED_WORK_REPORT.md", "S-08": "docs/UNFINISHED_WORK_COMPREHENSIVE_REPORT.md",
    "S-09": "UNFINISHED_WORK_ANALYSIS.md+UNFINISHED_WORK_AND_ISSUES.md+FINAL_UNFINISHED_WORK_REPORT.md", "S-10": "docs/OPEN_POINTS_CLOSURE.md", "S-11": "docs/LANDMINES.md",
    "S-12": "/*.md (other root status reports)+docs/COMPREHENSIVE_PACKAGE_SUMMARY.md+docs/README_IMPLEMENTATION_PACKAGE.md", "S-13": "docs/status/*.md",
    "S-14": "legacy-ids:defect ids first seen in the S-14 files", "S-15": "legacy-ids:defect ids first seen in the S-15 files", "S-16": "docs/security/**",
    "S-17": "challenges/helixqa-banks/*.yaml", "S-18": "challenges/data/challenges_bank.json", "S-19": "submodules/helix_qa/banks/**",
    "S-20": "submodules/helix_qa/challenges/baselines/bluff-baseline.txt+submodules/helix_qa/docs/behavior-anchors.md", "S-21": ".specify/memory/constitution.md",
    "S-22": "module CLAUDE.md and AGENTS.md (outside submodules)", "S-23": "marker and skipped-test scan of every text file of the snapshot", "S-24": ".implementation/**",
    "S-25": "S-25:SECURITY_KEY_ROTATION_REQUIRED.md", "XID": "legacy-ids:defect ids referenced only outside the S-14 and S-15 files"}
for cls, loc in CLASS_LOC.items():
    if base_counts.get(loc, 0) <= 0:
        check("(e) base tree: the source of %s reads > 0 (%s)" % (cls, loc), False, base_counts.get(loc))
        continue
    T = os.path.join(SCR, "pl_" + cls)
    shutil.copytree(BASE, T, symlinks=True)
    W.plant(T, cls)
    fjp = os.path.join(SCR, "pl_%s.freeze.json" % cls)
    F.freeze2(T, fjp)
    r, rec = recount(fjp, db0, os.path.join(SCR, "o_pl_" + cls))
    differing = [s for s in (rec["sources"] if rec else []) if s["status"] == "diff"]
    # S-14 / S-15 / XID plants add ids that live in text files that other classes also enumerate: the class's own source must differ by +1
    ok = r.returncode == 1 and any(s["locator"] == loc and s["recount"] - s["scanned_entry_count"] == 1 and s["differences"] for s in differing)
    check("(e) control needle %s: the recount of a tree with one planted entry differs from the base register at %s by exactly +1 and itemises it" % (cls, loc), ok,
          [(s["locator"], s["scanned_entry_count"], s["recount"]) for s in differing] or (r.returncode, r.stderr[-200:]))

# ---- (f) explanations; a source without a method
loc = CLASS_LOC["S-13"]
T = os.path.join(SCR, "pl_S-13")
fjp = os.path.join(SCR, "pl_S-13.freeze.json")
ex = os.path.join(SCR, "ex.json")
json.dump({loc: "a status document was added after the register was filled (test)"}, open(ex, "w"))
r, rec = recount(fjp, db0, os.path.join(SCR, "o_ex"), "--explanations", ex)
check("(f) a differing source that is explained is `explained` and the recount exits 0", r.returncode == 0 and rec and status_of(rec)[loc] == "explained" and rec["verdict"] == "PASS", (r.returncode, r.stdout[-300:]))
json.dump({"docs/qa/**+docs/reports/qa-sessions/**": "an explanation for a source that did not differ"}, open(ex, "w"))
r, rec = recount(fjp, db0, os.path.join(SCR, "o_ex2"), "--explanations", ex)
check("(f) an explanation of ANOTHER source does not excuse the difference (exit 1)", r.returncode == 1 and status_of(rec)[loc] == "diff", r.returncode)
db_x = os.path.join(SCR, "extra_src.db")
shutil.copy(db0, db_x)
sh(["sqlite3", db_x, "INSERT INTO reg_sources(kind,locator,parser) VALUES ('report_doc','mystery:source','test')"])
r, rec = recount(fj0, db_x, os.path.join(SCR, "o_x"))
check("(f) a register source with no recount method fails (`no_recount_method`), never passes unrecounted", r.returncode == 1 and status_of(rec)["mystery:source"] == "no_recount_method", r.returncode)

# ---- (h) a path with a newline
T4 = os.path.join(SCR, "nlpath")
shutil.copytree(BASE, T4, symlinks=True)
W.w(T4, "a\nb.txt", "// TODO inside a file whose name holds a newline\n")
fj4, db4 = build_db(T4, "nlpath")
r, rec = recount(fj4, db4, os.path.join(SCR, "o4"))
check("(h) a path holding a newline is counted (exit 0, markers equal)", r.returncode == 0 and rec and rec["verdict"] == "PASS", (r.returncode, r.stdout[-300:] + r.stderr[-300:]))

# ---- (i) doc03 section 7 comparison
d3 = os.path.join(SCR, "doc03.md")
open(d3, "w").write("# doc03\n\n## 7. Table of totals\n\n| Group | Source entries | x | Notes |\n|---|---|---|---|\n| HelixQA ticket files (S-01) | 1,778 | 1,778 | n |\n| ANR ticket (S-02) | 1 | 1 | n |\n| Landmine rules (S-11) | 63 | 63 | n |\n| Credential-rotation rows (S-25) | 37 | 37 | n |\n| Constitution Known Conflicts (S-21) | 16 items | 16 | n |\n\n## 8. Next\n")
r, rec = recount(fj0, db0, os.path.join(SCR, "o_d3"), "--doc03", d3)
d = {x["group"]: x for x in rec["doc03_section7"]} if rec else {}
check("(i) doc03 comparison: the fixture has 2 tickets, not 1778 -> S-01 is a delta of the right sign, itemised",
      d.get("HelixQA ticket files (S-01)", {}).get("status") == "delta" and d["HelixQA ticket files (S-01)"]["measured"] == 2, d.get("HelixQA ticket files (S-01)"))
check("(i) doc03 comparison: the fixture holds 2 rotation rows against doc03's 37 -> a delta of -35 itemised",
      d.get("Credential-rotation rows (S-25)", {}).get("status") == "delta" and d["Credential-rotation rows (S-25)"]["delta"] == -35, d.get("Credential-rotation rows (S-25)"))
open(d3, "w").write("# doc03\n\n## 7. Table of totals\n\n| Group | Source entries | x | Notes |\n|---|---|---|---|\n| ANR ticket (S-02) | 1 | 1 | n |\n\n## 8. Next\n")
r, rec = recount(fj0, db0, os.path.join(SCR, "o_d3b"), "--doc03", d3)
d = {x["group"]: x for x in rec["doc03_section7"]} if rec else {}
check("(i) doc03 comparison: S-02 equal (1 = 1); a group missing from doc03 -> `doc03_value_not_found`",
      d.get("ANR ticket (S-02)", {}).get("status") == "equal" and d.get("HelixQA ticket files (S-01)", {}).get("status") == "doc03_value_not_found", {k: v.get("status") for k, v in d.items()})
r, rec = recount(fj0, db0, os.path.join(SCR, "o_d3c"), "--doc03", os.path.join(SCR, "nope.md"))
check("(i) an unreadable doc03 is recorded `doc03_unreadable`, not a crash", r.returncode == 0 and rec and rec["doc03_section7"][0]["status"] == "doc03_unreadable", r.returncode)

# ---- usage and refusals
r = sh([TOOL])
check("usage: no arguments exits 2", r.returncode == 2, r.returncode)
r, rec = recount(os.path.join(SCR, "nofreeze.json"), db0, os.path.join(SCR, "o_nf"))
check("a missing freeze json is refused (exit 20, nothing written)", r.returncode == 20 and rec is None, r.returncode)

shutil.rmtree(SCR, ignore_errors=True)
print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
sys.exit(1 if FAILS else 0)
