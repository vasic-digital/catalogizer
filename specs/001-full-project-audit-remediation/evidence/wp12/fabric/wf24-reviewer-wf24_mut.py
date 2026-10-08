#!/usr/bin/env python3
"""WF24 independent mutation harness (reviewer-authored).
Copies go.mod, go.sum, pkg/ of the pristine scratch tree (/out/tree, == committed 83c0ac1) to /out/mut/<id>,
DELETES the reviewer probe file (pkg/fabric/zz_wf24_test.go) so ONLY the committed suite judges, applies ONE
exact-string replacement that must match exactly once, runs
  go test -race -count=1 -timeout 240s ./pkg/decorators/... ./pkg/fabric/
CAUGHT (rc!=0, tests failed), SURVIVED (rc 0), INVALID (build failed), NOT_APPLIED (anchor count != 1).
PC* must be CAUGHT, NC* must SURVIVE, and the baseline must PASS first. Optional argv[2:]: ids to run only.
With env JUDGE=probes the probe file is KEPT and only -run $RUN is executed (instrument validation)."""
import json, os, shutil, subprocess, sys

SRC = "/out/tree"
MUTS = json.load(open(sys.argv[1]))
ONLY = set(sys.argv[2:])
ENV = dict(os.environ, GOTOOLCHAIN="local", GOFLAGS="-mod=mod", HOME="/out", GOCACHE="/out/gocache",
           GOMODCACHE="/out/gomod", GOMAXPROCS="2", CGO_ENABLED="1")
JUDGE = os.environ.get("JUDGE", "suite")
CMD = ["go", "test", "-race", "-count=1", "-timeout", "240s", "./pkg/decorators/...", "./pkg/fabric/"]
if JUDGE == "probes":
    CMD = ["go", "test", "-race", "-count=1", "-timeout", "240s", "-run", os.environ["RUN"], "./pkg/fabric/"]

def fresh(name):
    d = os.path.join("/out/mut", name)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    for f in ("go.mod", "go.sum"):
        shutil.copy2(os.path.join(SRC, f), d)
    shutil.copytree(os.path.join(SRC, "pkg"), os.path.join(d, "pkg"))
    if JUDGE != "probes":
        import glob
        for fp in glob.glob(os.path.join(d, "pkg/*/zz_wf24*_test.go")) + glob.glob(os.path.join(d, "pkg/*/*/zz_wf24*_test.go")):
            os.remove(fp)
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
    if "WARNING: DATA RACE" in out:
        names.append("DATA_RACE")
    return names

d = fresh("baseline")
rc, out = run(d)
print(f"BASELINE judge={JUDGE} rc={rc} {'PASS' if rc == 0 else 'FAIL'}", flush=True)
if rc != 0 and JUDGE != "probes":
    print(out[-3000:]); sys.exit(2)
if JUDGE == "probes":
    print("BASELINE_FAILING=" + ",".join(failing(out)), flush=True)
shutil.rmtree(d)
bad = 0; tally = {}
for m in MUTS:
    if ONLY and m["id"] not in ONLY:
        continue
    d = fresh(m["id"])
    f = os.path.join(d, m["file"])
    s = open(f).read()
    n = s.count(m["old"])
    if n != 1:
        print(f'{m["id"]} NOT_APPLIED (old matched {n}x) -- {m["why"]}', flush=True)
        bad += 1; tally["NOT_APPLIED"] = tally.get("NOT_APPLIED", 0) + 1
        shutil.rmtree(d); continue
    open(f, "w").write(s.replace(m["old"], m["new"]))
    rc, out = run(d)
    names = failing(out)
    if names == ["BUILD_FAILED"]:
        verdict = "INVALID"; detail = [l for l in out.splitlines() if ".go:" in l][:2]
    elif rc != 0:
        verdict = "CAUGHT"; detail = names
    else:
        verdict = "SURVIVED"; detail = []
    tally[verdict] = tally.get(verdict, 0) + 1
    if m["id"].startswith("PC") and verdict != "CAUGHT":
        bad += 1; verdict += " (CONTROL FAILED: harness blind)"
    if m["id"].startswith("NC") and verdict != "SURVIVED":
        bad += 1; verdict += " (CONTROL FAILED)"
    print(f'{m["id"]} {verdict} rc={rc} by={",".join(detail)[:240]} -- {m["why"]}', flush=True)
    shutil.rmtree(d)
print("SUMMARY", tally)
print(f"HARNESS_CONTROLS_OK={bad == 0}")
