#!/usr/bin/env python3
"""fix-r3 mutation harness (WF24 round 3, constitution 11.4.276(D)); derived from fix-r2-mut.py, itself derived from the reviewer harness (rv_mut.py), which is NOT the author mutate.py.

Derived from the independent reviewer's harness (rv_mut.py), which is NOT the
author's mutate.py. Runs INSIDE the pinned Go container. For each mutant: copy
go.mod, go.sum, pkg/ of the tree under test to /out/mut/<id>, apply ONE
exact-string replacement that must match exactly once, run

    go test -count=1 -timeout 150s ./pkg/decorators/... ./pkg/fabric/

with the WHOLE suite (including the reviewer's adopted probes) as the judge and
classify: CAUGHT (test failure / panic / timeout), SURVIVED (rc 0),
INVALID (does not compile), NOT_APPLIED (old text not found exactly once).
Controls: the baseline must PASS first, every PC* must be CAUGHT, every NC*
must SURVIVE; HARNESS_CONTROLS_OK=True only if all hold.

usage: fix-r3-mut.py MUTANTS.json TREE [ID ...]
Env RACE=1 adds -race; the round-3 mutation run uses RACE=1 for EVERY mutant (the round-2 review showed that a race-only mutant is invisible without it).
"""
import json, os, shutil, subprocess, sys

MUTS = json.load(open(sys.argv[1]))
SRC = sys.argv[2]
ONLY = set(sys.argv[3:])
ENV = dict(os.environ, GOTOOLCHAIN="local", GOFLAGS="-mod=mod", HOME="/out", GOCACHE="/out/gocache",
           GOMODCACHE="/out/gomod", GOMAXPROCS="2", CGO_ENABLED="1")
CMD = ["go", "test", "-count=1", "-timeout", "150s", "./pkg/decorators/...", "./pkg/fabric/"]
if os.environ.get("RACE") == "1":
    CMD.insert(2, "-race")


def fresh(name):
    d = os.path.join("/out/mut", name)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    for f in ("go.mod", "go.sum"):
        shutil.copy2(os.path.join(SRC, f), d)
    shutil.copytree(os.path.join(SRC, "pkg"), os.path.join(d, "pkg"))
    return d


def run(d):
    try:
        p = subprocess.run(CMD, cwd=d, env=ENV, capture_output=True, text=True, timeout=500)
        return p.returncode, p.stdout + p.stderr
    except subprocess.TimeoutExpired:
        return 124, "HARNESS_TIMEOUT"


def failing(out):
    names = sorted({l.split()[2] for l in out.splitlines() if l.startswith("--- FAIL")})
    if "[build failed]" in out or "[setup failed]" in out:
        names.append("BUILD_FAILED")
    if "panic: test timed out" in out or "HARNESS_TIMEOUT" in out:
        names.append("TIMEOUT")
    if "panic:" in out and not names:
        names.append("PANIC")
    return names


d = fresh("baseline")
rc, out = run(d)
print(f"BASELINE rc={rc} {'PASS' if rc == 0 else 'FAIL'}", flush=True)
if rc != 0:
    print(out[-3000:])
    sys.exit(2)
shutil.rmtree(d)
bad = 0
counts = {"CAUGHT": 0, "SURVIVED": 0, "INVALID": 0, "NOT_APPLIED": 0}
for m in MUTS:
    if ONLY and m["id"] not in ONLY:
        continue
    d = fresh(m["id"])
    f = os.path.join(d, m["file"])
    s = open(f).read()
    n = s.count(m["old"])
    if n != 1:
        print(f'{m["id"]} NOT_APPLIED (old matched {n}x) -- {m["why"]}', flush=True)
        counts["NOT_APPLIED"] += 1
        bad += 1
        shutil.rmtree(d)
        continue
    open(f, "w").write(s.replace(m["old"], m["new"]))
    rc, out = run(d)
    names = failing(out)
    if names == ["BUILD_FAILED"]:
        verdict, detail = "INVALID(BUILD_FAILED)", [l for l in out.splitlines() if ".go:" in l][:2] or ["RAW:" + " | ".join(out.strip().splitlines()[-3:])[:300]]
        counts["INVALID"] += 1
    elif rc != 0:
        verdict, detail = "CAUGHT", names
        counts["CAUGHT"] += 1
    else:
        verdict, detail = "SURVIVED", []
        counts["SURVIVED"] += 1
    if m["id"].startswith("PC") and verdict != "CAUGHT":
        bad += 1
        verdict += " (CONTROL FAILED: harness blind)"
    if m["id"].startswith("NC") and verdict != "SURVIVED":
        bad += 1
        verdict += " (CONTROL FAILED)"
    print(f'{m["id"]} {verdict} rc={rc} by={",".join(detail)[:240]} -- {m["why"]}', flush=True)
    shutil.rmtree(d)
print(f"SUMMARY {counts}")
print(f"HARNESS_CONTROLS_OK={bad == 0}")
