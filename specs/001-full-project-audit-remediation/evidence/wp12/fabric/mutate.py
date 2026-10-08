#!/usr/bin/env python3
"""Mutation harness for PA-02/PA-07 (runs INSIDE the pinned Go container).

For every mutant in mutants.json: copy the submodule tree to /tmp, apply ONE exact-string replacement (it must match
exactly once, else the mutant is reported NOT_APPLIED and the run fails), run the two packages' tests, and require a
test failure (or build failure) for every mutant except the negative control (id starting NC), which must survive.
A baseline (unmutated copy) must pass first. Output: one line per mutant + a summary; exit 0 only if every mutant is
CAUGHT, the negative control SURVIVED and the baseline PASSED.
"""
import json, os, shutil, subprocess, sys, tempfile

SRC = "/src/submodules/filesystem"
MUTS = json.load(open(sys.argv[1]))
ONLY = set(sys.argv[2:])
TIMEOUT = 240
ENV = dict(os.environ, GOTOOLCHAIN="local", GOFLAGS="-mod=mod", HOME="/out", GOCACHE="/out/gocache",
           GOMODCACHE="/out/gomod", GOMAXPROCS="3", CGO_ENABLED="1")

def run(tree):
    try:
        p = subprocess.run(["go", "test", "-race", "-count=1", "-timeout", "90s", "./pkg/decorators/", "./pkg/fabric/"], cwd=tree, env=ENV,
                           capture_output=True, text=True, timeout=TIMEOUT)
        out = p.stdout + p.stderr
        return p.returncode, out
    except subprocess.TimeoutExpired as e:
        return 124, "TIMEOUT (a hang counts as caught)\n" + ((e.stdout or b"").decode(errors="replace") if isinstance(e.stdout, bytes) else (e.stdout or ""))

def fresh():
    d = tempfile.mkdtemp(prefix="mut-", dir="/tmp")
    shutil.copytree(SRC, os.path.join(d, "t"), ignore=shutil.ignore_patterns(".git", ".codegraph"))
    return d, os.path.join(d, "t")

def failing(out):
    names = sorted({l.split()[2] for l in out.splitlines() if l.startswith("--- FAIL")})
    if "[build failed]" in out or "build failed" in out:
        names.append("BUILD_FAILED")
    if "TIMEOUT" in out or "panic: test timed out" in out:
        names.append("TIMEOUT")
    return names

bad = 0
d, t = fresh()
rc, out = run(t)
print(f"BASELINE rc={rc} {'PASS' if rc == 0 else 'FAIL'}")
if rc != 0:
    print(out[-2000:]); sys.exit(2)
shutil.rmtree(d)
caught = survived_nc = 0
total = 0
for m in MUTS:
    if ONLY and m["id"] not in ONLY:
        continue
    total += 1
    d, t = fresh()
    f = os.path.join(t, m["file"])
    s = open(f).read()
    n = s.count(m["old"])
    if n != 1:
        print(f'{m["id"]} NOT_APPLIED (old matched {n} times) {m["file"]}'); bad += 1; shutil.rmtree(d); continue
    open(f, "w").write(s.replace(m["old"], m["new"]))
    rc, out = run(t)
    nc = m["id"].startswith("NC")
    if nc:
        ok = rc == 0
        print(f'{m["id"]} {"SURVIVED(expected)" if ok else "UNEXPECTED_FAIL"} rc={rc} -- {m["why"]}')
        survived_nc += ok; bad += (not ok)
    else:
        names = failing(out)
        invalid = names == ["BUILD_FAILED"]  # a mutant that does not compile proves nothing
        ok = rc != 0 and not invalid
        if invalid:
            print(f'{m["id"]} INVALID_MUTANT(BUILD_FAILED) -- {m["why"]}'); bad += 1; shutil.rmtree(d); continue
        print(f'{m["id"]} {"CAUGHT" if ok else "SURVIVED(BAD)"} rc={rc} by={",".join(failing(out))[:160]} -- {m["why"]}')
        caught += ok; bad += (not ok)
    shutil.rmtree(d)
print(f"SUMMARY mutants={total - sum(1 for m in MUTS if m['id'].startswith('NC') and (not ONLY or m['id'] in ONLY))} caught={caught} negative_controls_survived={survived_nc} bad={bad}")
sys.exit(1 if bad else 0)
