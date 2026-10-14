#!/usr/bin/env python3
"""record_w2_evidence.py - records the RED and GREEN runs of the W2 register tasks (T166 recount, T170 plan seeds, T171 innovation entries, T172 ticket specifics,
T174 root inventory, the freeze-listing check) into the evidence ledger through tools/evidence/evrec, one entry per run, and copies each run's stored stdout to
$EV/register/w2/ as a transcript with an identity header.
  RED     the task's suite against a SUT directory WITHOUT the tool under test (T166: with the pre-fix recount_sources.py of git HEAD, whose source-class S-26 / S-27
          rows were keyed `total` instead of by file and so differ on the first plan-document tree): the suite exits non-zero
  GREEN   the same command (same argv, cwd, target_ref, test fingerprint) after the SUT directory is filled with byte copies of the tool set, three iterations
Usage: record_w2_evidence.py red|green|all [TASK ...]   TASK in recount seeds innovation t172 root listing
Env: W2_PREFIX_DIR=<dir with one subdirectory per task holding the pre-fix tool set> (RED uses it instead of omitting the tool), W2_LABEL_PREFIX=fix-r2- (transcript names), EV_MODE=container (default; scripts/containers/run_pinned.sh IMG-TESTUTIL, waits for the host memory budget it requires) | host. Never writes docs/workable_items.db."""
import hashlib, json, os, re, shutil, subprocess, sys, time

HERE = os.path.dirname(os.path.abspath(__file__)); ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
os.chdir(ROOT)
for k in ("EV", "EV_LEDGER", "EV_BLOBS", "EV_ANCHOR"):
    os.environ.pop(k, None)
os.environ.setdefault("TMPDIR", "/dev/shm"); os.environ["DISK_HEADROOM_REPO_ROOT"] = ROOT; os.environ["LONGOPS_ALLOW_TMPFS"] = "1"
EVD = "specs/001-full-project-audit-remediation/evidence"; OUT = EVD + "/register/w2"; SCR = ".audit/scratch/w2ev"
MODE = os.environ.get("EV_MODE", "container")
COMMON = ["check_freeze_snapshot.sh", "check_freeze_listing.sh", "snapshot_manifest.py", "enumerate_sources.py", "category_map.yaml"]
TS = ["scripts/register/tests/w2_fixture.py", "scripts/register/tests/wp20_fixture.py"]
# task -> (item, test file, interpreter words, env var, SUT file, tool file set)
TASKS = {
    "recount": ("AUD-T166", "scripts/register/tests/test_recount.py", ["python3", "-I"], "RECOUNT_SH", "recount_sources.sh", ["recount_sources.sh", "recount_sources.py"] + COMMON),
    "seeds": ("AUD-T170", "scripts/register/tests/test_seed_import.py", ["python3", "-I"], "IMPORT_PLAN", "import_plan.sh", ["import_plan.sh", "import_plan.py", "import_tickets.py"] + COMMON),
    "innovation": ("AUD-T171", "scripts/register/tests/test_innovation_import.py", ["python3", "-I"], "IMPORT_PLAN", "import_plan.sh", ["import_plan.sh", "import_plan.py", "import_tickets.py"] + COMMON),
    "t172": ("AUD-T172", "scripts/register/tests/test_t172.py", ["python3", "-I"], "IMPORT_T172", "import_t172.sh",
             ["import_t172.sh", "import_t172.py", "import.sh", "import_plan.py", "import_tickets.py"] + COMMON),
    "root": ("AUD-T174", "scripts/register/tests/test_root_inventory.py", ["python3", "-I"], "ROOT_INVENTORY", "root_inventory.py", ["root_inventory.py", "root_dispositions.yaml", "check_freeze_listing.sh"]),
    "listing": ("AUD-W2-LISTING", "scripts/register/tests/test_freeze_listing.sh", ["bash"], "CHECK_FREEZE_LISTING", "check_freeze_listing.sh", ["check_freeze_listing.sh"]),
}


def wait_budget():
    if MODE != "container":
        return
    for _ in range(240):
        mi = {l.split(":")[0]: int(l.split()[1]) for l in open("/proc/meminfo")}
        if mi["MemAvailable"] * 1024 >= max(4 * 2 ** 30, mi["MemTotal"] * 1024 * 15 // 100) + 832 * 2 ** 20:
            return
        time.sleep(15)
    sys.exit("record_w2_evidence: the host memory budget run_pinned.sh requires did not appear in 60 minutes")


def pfx():
    return ["scripts/containers/run_pinned.sh", "--network=none", "IMG-TESTUTIL", "--"] if MODE == "container" else []


def apath(rel):
    return ("/src/" if MODE == "container" else ROOT + "/") + rel


OMIT_IN_RED = {"seeds": ("import_plan.sh", "import_plan.py"), "innovation": ("import_plan.sh", "import_plan.py"), "t172": ("import_t172.sh", "import_t172.py"),
               "root": ("root_inventory.py", "root_dispositions.yaml"), "listing": ("check_freeze_listing.sh",), "recount": ()}


def fill(task, kind):
    sut = "%s/%s/sut" % (SCR, task)
    shutil.rmtree(sut, ignore_errors=True); os.makedirs(sut)
    files = TASKS[task][5]
    pre = os.environ.get("W2_PREFIX_DIR")          # review fix rounds: RED = the PRE-FIX tool set (a byte copy kept before the fix) with the NEW suite
    if kind == "red" and pre:
        for f in os.listdir(os.path.join(pre, task)):
            shutil.copy(os.path.join(pre, task, f), sut + "/" + f)
        return sut
    for f in files:
        if kind == "red" and f in OMIT_IN_RED[task]:
            continue
        if kind == "red" and task == "recount" and f == "recount_sources.py":       # the PRE-FIX tool: the committed HEAD version (S-26 / S-27 keyed `total`)
            open(sut + "/" + f, "wb").write(subprocess.run(["git", "show", "HEAD:scripts/register/recount_sources.py"], capture_output=True, check=True).stdout)
            continue
        shutil.copy("scripts/register/" + f, sut + "/" + f)
    return sut


def rec(task, pol, it, sut):
    wait_budget()
    item, tf, interp, envv, sutf, files = TASKS[task]
    a = ["tools/evidence/evrec", "run", item, pol, str(it), "shell_script", sut, "--oracle", "specified", "--oracle-independent", "--evidence-class", "runtime"]
    for t in [tf] + TS:
        a += ["--test-source", t]
    a += ["--"] + pfx() + ["env", "%s=%s" % (envv, apath(sut + "/" + sutf))] + interp + [apath(tf) if MODE == "container" else tf]
    r = subprocess.run(a, capture_output=True, text=True)
    out = (r.stdout + r.stderr).strip()
    label = "%s%s-%s%d" % (os.environ.get("W2_LABEL_PREFIX", ""), task, pol.lower(), it)
    print("%s: evrec rc=%d %s" % (label, r.returncode, out), flush=True)
    m = re.search(r"recorded seq=(\d+)", out)
    if not m:
        return
    for l in open(EVD + "/ledger.jsonl"):
        d = json.loads(l)
        if d["seq"] == int(m.group(1)):
            blob = os.path.join(EVD, "blobs", d["stdout_sha256"])
            hdr = "# ledger seq=%s item=%s polarity=%s iteration=%s exit_status=%s verdict=%s\n# target_ref=%s target_fingerprint=%s test_fingerprint=%s\n# argv=%s\n" % (
                d["seq"], d["item"], d["polarity"], d["iteration"], d["exit_status"], d["verdict"], d["target_ref"], d["target_fingerprint"], d.get("test_fingerprint"), json.dumps(d["argv"]))
            os.makedirs(OUT, exist_ok=True)
            open("%s/ledger-%s.txt" % (OUT, label), "wb").write(hdr.encode() + (open(blob, "rb").read() if os.path.exists(blob) else b"(stdout blob not found)\n"))


if __name__ == "__main__":
    a = sys.argv[1:]
    if not a or a[0] not in ("red", "green", "all"):
        sys.exit(__doc__)
    tasks = a[1:] or list(TASKS)
    for t in tasks:
        if a[0] in ("red", "all"):
            rec(t, "RED", 1, fill(t, "red"))
        if a[0] in ("green", "all"):
            sut = fill(t, "green")
            for i in (1, 2, 3):
                rec(t, "GREEN", i, sut)
