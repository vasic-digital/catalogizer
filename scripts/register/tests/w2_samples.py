#!/usr/bin/env python3
"""w2_samples.py - runs the real W2 tools end to end on the FIXTURE tree of the tests (never the real register, never the real snapshot) and copies the records they
write to the directory given as argument: qa_causes.json, families.json, platform_inference.json, entryset-before.tsv, entryset-after.tsv, entryset.json (T172),
root-items.json (T174), recount.json (T166), the S-26 and S-27 reconciliations (T170, T171). These are SAMPLES of the record shapes on a fixture; the real records wait for
the T161 freeze. Usage: w2_samples.py <out dir>   (inside IMG-TESTUTIL: scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- python3 -I /src/scripts/register/tests/w2_samples.py /out/<dir>)"""
import json, os, shutil, subprocess, sys, tempfile
HERE = os.path.dirname(os.path.abspath(__file__)); REG = os.path.dirname(HERE); ROOT = os.path.dirname(os.path.dirname(REG))
sys.path.insert(0, HERE)
import w2_fixture as F, wp20_fixture as W   # noqa: E402
WI = os.environ.get("WI", os.path.join(ROOT, "constitution/scripts/workable-items/bin/workable-items-linux"))
out = sys.argv[1]
os.makedirs(out, exist_ok=True)
scr = tempfile.mkdtemp(prefix="w2samples.", dir=os.environ.get("SCRATCH_BASE") or os.environ.get("TMPDIR"))


def run(cmd, **kw):
    r = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, **kw)
    print("rc=%d %s" % (r.returncode, " ".join(cmd[:3])[:120]))
    if r.returncode not in (0,):
        print(r.stdout.decode()[-800:])
    return r


T = os.path.join(scr, "tree")
W.build(T); F.tickets_qa(T); F.plan_docs(T)
fj = os.path.join(scr, "f.json"); F.freeze2(T, fj)
run([sys.executable, "-I", REG + "/enumerate_sources.py", "--freeze-json", fj, "--out", scr + "/enum"])
db = scr + "/r.db"
run([REG + "/apply_ext.sh", "--db", db])
run(["sqlite3", db, ".read " + scr + "/enum/source_entries.sql"])
subprocess.run(["sqlite3", db, "INSERT INTO reg_components(component_id,kind,path_root,own_repo) VALUES ('helixqa','tooling','submodules/helix_qa',0),('qa-scripts','tooling','scripts',0)"], check=True)
run([REG + "/import_t172.sh", "--stage", "qa", "--db", db, "--freeze-json", fj, "--engine", WI, "--out-dir", out + "/t172-qa"])
run([REG + "/import.sh", "--db", db, "--freeze-json", fj, "--engine", WI])
run([REG + "/import_t172.sh", "--stage", "families", "--db", db, "--freeze-json", fj, "--engine", WI, "--out-dir", out + "/t172-families"])
run([REG + "/import_plan.sh", "--class", "seeds", "--db", db, "--freeze-json", fj, "--engine", WI, "--reconciliation", out + "/seeds-reconciliation.json"])
run([REG + "/import_plan.sh", "--class", "innovation", "--db", db, "--freeze-json", fj, "--engine", WI, "--reconciliation", out + "/innovation-reconciliation.json"])
run([REG + "/recount_sources.sh", "--freeze-json", fj, "--db", db, "--out", out + "/recount"])
run([sys.executable, "-I", REG + "/root_inventory.py", "--freeze-json", fj, "--out", out + "/root-items"])
shutil.rmtree(scr, ignore_errors=True)
