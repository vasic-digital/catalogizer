#!/usr/bin/env python3
"""record_wp20_evidence.py - records the RED, GREEN and MUTATION runs of the WP-20 / WP-21 round-2 fix pass into the evidence ledger through
tools/evidence/evrec (WF23 review E1-E3), one entry per run, and copies each run's stored stdout to the evidence folder as a transcript.
  RED       the new test against the PRE-FIX tool (the verbatim copy kept in evidence/wp20/fix-r2-prefix-scripts/, hash-checked here)
  GREEN     the same command after the fix, three iterations, on a DIFFERENT target fingerprint
  MUTATION  a reviewer-authored mutant (WF23) makes the same command fail
The system under test of a RED/GREEN pair is a DIRECTORY (target_ref) filled with the pre-fix set for the RED and with a byte copy of the fixed set
for the GREEN, so both entries share argv, cwd, target_ref and test fingerprint (the pairing rule of tools/evidence/evverdict.py) and differ in the
target fingerprint. WP-21: .audit/scratch/wp21-r2/gate.sh likewise (pre-T180 gate for the RED) plus GREEN entries whose target is
scripts/register/gate.sh itself (E1: the ledger fingerprints the gate, not its test).
Usage: record_wp20_evidence.py wp20-red|wp20-green|wp20-mut|wp21-red|wp21-green|wp21-mut|all
Env: EV_MODE=container (default; scripts/containers/run_pinned.sh IMG-TESTUTIL, waits for the host memory budget it requires) | host (no container;
     the entry's argv says so and the entry proves less). Never writes docs/workable_items.db; unsets EV so the real ledger is used."""
import hashlib, json, os, shutil, subprocess, sys, time

HERE = os.path.dirname(os.path.abspath(__file__)); ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
os.chdir(ROOT)
for k in ("EV", "EV_LEDGER", "EV_BLOBS", "EV_ANCHOR"):
    os.environ.pop(k, None)
EVD = "specs/001-full-project-audit-remediation/evidence"; W20 = EVD + "/wp20"; W21 = EVD + "/wp21"
PRE = W20 + "/fix-r2-prefix-scripts"; MODE = os.environ.get("EV_MODE", "container")
SUT20 = ".audit/scratch/wp20-r2/sut"; MUT20 = ".audit/scratch/wp20-r2/mut"; SUT21 = ".audit/scratch/wp21-r2"
SET = ("enumerate_sources.py", "lead_scan.py", "lead_scan_population.py", "snapshot_manifest.py", "check_freeze_snapshot.sh")
TS = ["scripts/register/tests/wp20_fixture.py", "scripts/register/tests/lib.sh"]


def sha(p):
    return hashlib.sha256(open(p, "rb").read()).hexdigest()


def wait_budget():
    if MODE != "container":
        return
    for _ in range(240):
        mi = {l.split(":")[0]: int(l.split()[1]) for l in open("/proc/meminfo")}
        if mi["MemAvailable"] * 1024 >= max(4 * 2 ** 30, mi["MemTotal"] * 1024 * 15 // 100) + 832 * 2 ** 20:    # run_pinned.sh: MemAvailable - max(4 GiB, 15% of MemTotal) >= 512 MiB
            return
        time.sleep(15)
    sys.exit("record_wp20_evidence: the host memory budget run_pinned.sh requires did not appear in 60 minutes")


def pfx():
    return ["scripts/containers/run_pinned.sh", "--network=none", "IMG-TESTUTIL", "--"] if MODE == "container" else []


def apath(rel):
    return ("/src/" if MODE == "container" else ROOT + "/") + rel


def rec(item, pol, it, ref, label, test_srcs, cmd, mutation=None):
    wait_budget()
    a = ["tools/evidence/evrec", "run", item, pol, str(it), "shell_script", ref, "--oracle", "specified", "--oracle-independent", "--evidence-class", "runtime"]
    for t in test_srcs:
        a += ["--test-source", t]
    if mutation:
        a += ["--mutation-json", json.dumps(mutation)]
    a += ["--"] + pfx() + cmd
    r = subprocess.run(a, capture_output=True, text=True)
    out = (r.stdout + r.stderr).strip()
    print("%s: evrec rc=%d %s" % (label, r.returncode, out), flush=True)
    import re
    m = re.search(r"recorded seq=(\d+)", out)
    if not m:
        return
    for l in open(EVD + "/ledger.jsonl"):
        d = json.loads(l)
        if d["seq"] == int(m.group(1)):
            blob = os.path.join(EVD, "blobs", d["stdout_sha256"])
            hdr = "# ledger seq=%s item=%s polarity=%s iteration=%s exit_status=%s verdict=%s\n# target_ref=%s target_fingerprint=%s test_fingerprint=%s\n# argv=%s\n" % (
                d["seq"], d["item"], d["polarity"], d["iteration"], d["exit_status"], d["verdict"], d["target_ref"], d["target_fingerprint"], d.get("test_fingerprint"), json.dumps(d["argv"]))
            body = open(blob, "rb").read() if os.path.exists(blob) else b"(stdout blob not found)\n"
            open("%s/fix-r2-ledger-%s.txt" % (W20 if label.startswith("t16") else W21, label), "wb").write(hdr.encode() + body)


def install(kind):
    shutil.rmtree(SUT20, ignore_errors=True); os.makedirs(SUT20)
    for f in SET:
        shutil.copy((PRE if kind == "pre" else "scripts/register") + "/" + f, SUT20 + "/" + f)


# name -> (item, test file, env var naming the SUT file, SUT file, interpreter words)
SUITES = {"t165": ("AUD-T165", "scripts/register/tests/test_enumerate_absolute.py", "ENUMERATE_SOURCES", "enumerate_sources.py", ["python3"]),
          "t162": ("AUD-T162", "scripts/register/tests/test_enumerate_planted.sh", "ENUMERATE_SOURCES", "enumerate_sources.py", ["bash"]),
          "t164": ("AUD-T164", "scripts/register/tests/test_frontmatter_yaml.py", "ENUMERATE_SOURCES", "enumerate_sources.py", ["python3"]),
          "t167": ("AUD-T167", "scripts/register/tests/test_lead_scan.py", "LEAD_SCAN", "lead_scan.py", ["python3"])}


def run20(s, pol, it, sutdir=SUT20, mutation=None):
    item, tf, envv, sutf, interp = SUITES[s]
    tail = [tf] + (["--selftest"] if s == "t164" else [])
    rec(item, pol, it, sutdir, "%s-%s%d" % (s, pol.lower(), it), [tf] + TS, ["env", "%s=%s" % (envv, apath(sutdir + "/" + sutf))] + interp + tail, mutation)


def mutants():
    shutil.rmtree(MUT20, ignore_errors=True)
    def mk(name, f, old, new):
        d = MUT20 + "/" + name; os.makedirs(d)
        for x in SET:
            shutil.copy("scripts/register/" + x, d)
        t = open(d + "/" + f, encoding="utf-8").read(); assert t.count(old) == 1, (name, old); open(d + "/" + f, "w", encoding="utf-8").write(t.replace(old, new))
    mk("EM06", "enumerate_sources.py", 'status=fm.get("status") or None, severity=fm.get("severity") or None))', 'status=fm.get("severity") or None, severity=fm.get("status") or None))')
    mk("N26", "enumerate_sources.py", 'for e in sorted(ents, key=lambda x: x.locator.encode("utf-8")):', 'for e in sorted(ents, key=lambda x: hash((x.locator, os.getpid()))):')
    mk("FM02", "enumerate_sources.py", "    fm, last, block, parts = {}, None, None, []\n",
       "    fm, last, block, parts = {}, None, None, []\n    return {k.strip(): v.strip() for k, v in (l.split(':', 1) for l in lines[1:end] if ':' in l)}, '\\n'.join(lines[end + 1:]), []\n")
    mk("LM05", "lead_scan.py", "            covered = covering(cover, p, n)\n", "            covered = covering(cover, p, n) or ((\"STRUCTURED\", p) if p in cover else None)\n")


def main(step):
    for f in SET:                                  # the pre-fix copies are the review's verbatim files: check them against their recorded hashes
        want = [l.split()[0] for l in open(W20 + "/fix-r2-prefix-scripts.sha256") if l.split()[1] == f][0]
        assert sha(PRE + "/" + f) == want, f
    if step in ("wp20-red", "all"):
        install("pre")
        for s in ("t165", "t162", "t164", "t167"):
            run20(s, "RED", 1)
    if step in ("wp20-green", "all"):
        install("post")
        for s in ("t165", "t162", "t164", "t167"):
            for i in (1, 2, 3):
                run20(s, "GREEN", i)
    if step in ("wp20-mut", "all"):
        mutants()
        mj = lambda op, loc: {"author": "reviewer", "operator": op, "location": loc, "result": "caught"}
        run20("t165", "MUTATION", 1, MUT20 + "/EM06", mj("swap status and severity of the S-01 entries (WF23 EM06)", "scripts/register/enumerate_sources.py rule_issue_file"))
        run20("t162", "MUTATION", 1, MUT20 + "/N26", mj("nondeterministic entry order (planted-suite mutant)", "scripts/register/enumerate_sources.py enumerate_snapshot"))
        run20("t164", "MUTATION", 1, MUT20 + "/FM02", mj("the tolerant reader replaced by the doc04 naive splitter (WF23 FM02)", "scripts/register/enumerate_sources.py parse_frontmatter"))
        run20("t167", "MUTATION", 1, MUT20 + "/LM05", mj("duplicate label for any covered file (WF23 LM05)", "scripts/register/lead_scan.py disposition"))
    t21 = ["scripts/register/tests/test_reverify_gate.sh", "scripts/register/tests/lib.sh"]
    gate_cmd = lambda ref: ["env", "GATE=" + apath(ref), "bash", "scripts/register/tests/test_reverify_gate.sh"]
    if step in ("wp21-red", "all"):
        os.makedirs(SUT21, exist_ok=True); shutil.copy(W21 + "/fix-r2-gate_pre_t180.sh", SUT21 + "/gate.sh")
        rec("AUD-T179", "RED", 1, SUT21 + "/gate.sh", "t179-red1", t21, gate_cmd(SUT21 + "/gate.sh")[:0] + ["env", "GATE=" + apath(SUT21 + "/gate.sh"), "bash", "scripts/register/tests/test_reverify_gate.sh"])
    if step in ("wp21-green", "all"):
        os.makedirs(SUT21, exist_ok=True); shutil.copy("scripts/register/gate.sh", SUT21 + "/gate.sh")
        for i in (1, 2, 3):
            rec("AUD-T179", "GREEN", i, SUT21 + "/gate.sh", "t179-green%d" % i, t21, ["env", "GATE=" + apath(SUT21 + "/gate.sh"), "bash", "scripts/register/tests/test_reverify_gate.sh"])
        for i in (1, 2, 3):
            rec("AUD-T180", "GREEN", i, "scripts/register/gate.sh", "t180-green%d" % i, t21, ["bash", "scripts/register/tests/test_reverify_gate.sh"])
    if step in ("wp21-mut", "all"):
        rec("AUD-T180", "MUTATION", 1, "scripts/register/gate.sh", "t180-mut1", t21,
            ["env", "REG_EXT_SQL=" + apath(W21 + "/mutant_ext_row_removed.sql"), "bash", "scripts/register/tests/test_reverify_gate.sh"],
            {"author": "test_author", "operator": "remove-registry-row v_reverify_queue/view_not_done from register_ext.sql", "location": "scripts/register/register_ext.sql reg_gate_checks INSERT", "result": "caught"})


if __name__ == "__main__":
    if len(sys.argv) != 2 or sys.argv[1] not in ("wp20-red", "wp20-green", "wp20-mut", "wp21-red", "wp21-green", "wp21-mut", "all"):
        sys.exit(__doc__)
    main(sys.argv[1])
