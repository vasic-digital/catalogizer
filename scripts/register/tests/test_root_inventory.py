#!/usr/bin/env python3
"""test_root_inventory.py - T174 (WP-20): the root-item inventory (scripts/register/root_inventory.py + root_dispositions.yaml).
Real files, real freeze listing built from a scratch tree; no git, no ls, no live tree. Env: ROOT_INVENTORY (tool under test; mutation runs point it at a mutated
copy that sits beside copies of check_freeze_listing.sh and root_dispositions.yaml), REAL_FREEZE_JSON (optional: a freeze json of a real tree, e.g. a `git archive HEAD`
snapshot: the real-listing leg runs only then), EXPECT_ROOT_MD (default 32,9,8: the T174 text), SCRATCH_BASE.
  (a) the base tree: PASS, one row per root entry, byte-order sorted, no row without a disposition, `.git` absent
  (b) needle: a root entry planted into the tree before the freeze appears as a row WITHOUT a disposition and the run FAILS (exit 1)
  (c) needle: a root entry created in the live tree AFTER the freeze is absent from root-items.json, and the run passes with the snapshot directory deleted
      (the tool reads no snapshot file and never lists the live tree)
  (d) `.git` in untracked_root_entries fails the check and yields no row; a root-level gitlink is a `gitlink` row
  (e) every rule of the decision table is reachable: a root entry named by a rule gets THAT rule (no rule is shadowed), a regex rule's sample too; every disposition of the
      closed set occurs; the rule table is validated (closed set, required fields, unique ids)
  (f) --check: a good file passes; a removed disposition, an unknown disposition, a `.git` row, a duplicate, an unsorted file, a missing row and an extra row (with
      --freeze-json) each fail
  (g) listing moved -> refused `freeze_listing_moved`, nothing written; determinism; documentation rows carry doc_class null; excluded rows carry reasons
  (h) the docs/01 lookup: a root path row of sections 3.7/3.8 and a heading of 3.1-3.6 are found, an application-relative path of 3.1-3.6 is not a root row, `absent` otherwise
  (i) config names its two index files individually (T174 round-31 review I3); the untracked entries come from freeze.json, with kind `untracked`
  (j) real leg (REAL_FREEZE_JSON): all four dispositions, no row without a disposition, the root Markdown split of the T174 text
Exit 0 only when every check passed.
"""
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(REG))
sys.path.insert(0, HERE)
import w2_fixture as F   # noqa: E402
import wp20_fixture as W   # noqa: E402

TOOL = os.environ.get("ROOT_INVENTORY", os.path.join(REG, "root_inventory.py"))
RULES = os.path.join(os.path.dirname(TOOL), "root_dispositions.yaml")
FAILS = PASSES = 0


def check(label, cond, info=""):
    global FAILS, PASSES
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:700]))
    if cond:
        PASSES += 1
    else:
        FAILS += 1


def run(*args, **kw):
    return subprocess.run([sys.executable, "-I", TOOL] + list(args), stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kw)


def sha(p):
    return hashlib.sha256(open(p, "rb").read()).hexdigest()


print("# identity: task=T174 utc=%s uid=%d git_head=%s" % (__import__("time").strftime("%Y-%m-%dT%H:%M:%SZ", __import__("time").gmtime()), os.getuid(),
      subprocess.run(["git", "-C", ROOT, "rev-parse", "HEAD"], stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout.decode().strip()))
inc = os.path.exists("/run/.containerenv") or os.environ.get("container") == "podman"
print("# container: %s" % ("in-container run (container=%s) image=IMG-TESTUTIL" % os.environ.get("container", "unset") if inc else "host-side run (no container marker); a container leg is UNCONFIRMED for this transcript"))
print("# tool=%s sha256=%s rules sha256=%s" % (TOOL, sha(TOOL)[:16] if os.path.isfile(TOOL) else "ABSENT", sha(RULES)[:16] if os.path.isfile(RULES) else "ABSENT"))
if not os.path.isfile(TOOL):
    check("root_inventory.py present: %s (RED: T174 not implemented)" % TOOL, False)
    print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
    sys.exit(1)

SCR = tempfile.mkdtemp(prefix="rootinv.", dir=os.environ.get("SCRATCH_BASE") or os.environ.get("TMPDIR"))
DOC01 = os.path.join(SCR, "doc01.md")
open(DOC01, "w").write("""# doc01
## 2. Other
| `Website/` | not read: section 2 |
## 3. Applications and shared components
### 3.1 catalog-api (Go backend)
| Path | What |
|---|---|
| `database/` | application-relative path in 3.1: NOT a root row |
### 3.7 Other top-level components
| Path | What it is |
|---|---|
| `Website/` | VitePress site |
| `examples/helixcode-qa-integration/` | a nested module (a sub-path row) |
### 3.8 Root configuration files
| Path | State |
|---|---|
| `LICENSE` | tracked |
| `.claude/` | ignored |
## 4. Submodules
| `Upstreams/` | not read: section 4 |
""")


def freeze(tree, name, untracked=(".claude", ".remember"), gitlinks=()):
    fj = os.path.join(SCR, name + ".freeze.json")
    F.freeze2(tree, fj, untracked, gitlinks)
    return fj


def gen(fj, out, *extra):
    if os.path.exists(out):
        shutil.rmtree(out)
    r = run("--freeze-json", fj, "--out", out, "--inventory", DOC01, *extra)
    p = os.path.join(out, "root-items.json")
    return r, (json.load(open(p)) if os.path.exists(p) else None)


# ---- (a) base tree
BASE = os.path.join(SCR, "base")
W.build(BASE)
fj0 = freeze(BASE, "base")
r, rec = gen(fj0, os.path.join(SCR, "o0"))
rows = rec["rows"] if rec else []
names = [x["entry"] for x in rows]
check("(a) base tree: exit 0 and verdict PASS", r.returncode == 0 and rec and rec["verdict"] == "PASS", (r.returncode, r.stdout[-500:], r.stderr[-300:]))
want = sorted({p.split("/")[0] for p in (q.decode() for q in open(json.load(open(fj0))["listing"], "rb").read().split(b"\0")[:-1])} | {".claude", ".remember"}, key=lambda x: x.encode())
check("(a) one row per root entry of the freeze (%d), sorted by byte order, no duplicates" % len(want), names == want, (set(names) ^ set(want)))
check("(a) no row without a disposition and no `.git` row", rec and all(x["disposition"] in ("problem_source", "component", "documentation", "excluded") for x in rows) and ".git" not in names)
check("(a) untracked entries come from freeze.json (`.claude`, `.remember`) with kind `untracked`, tracked false", all(x["kind"] == "untracked" and x["tracked"] is False for x in rows if x["entry"] in (".claude", ".remember")) and ".claude" in names)
check("(a) documentation rows carry doc_class null and doc_class_by; excluded rows carry a reason",
      all(x["doc_class"] is None and x["doc_class_by"] == "WP-37 T282" for x in rows if x["disposition"] == "documentation") and all(x.get("reason") for x in rows if x["disposition"] == "excluded"))
check("(a) the root Markdown summary counts the 8 governance files and the Stage 0 source classes (the fixture holds S-03, S-04, S-06, S-07, S-09 x3, S-25 x2 and S-12 reports)",
      rec and rec["summary"]["root_markdown"]["by_class"].get("governance") == 8 and rec["summary"]["root_markdown"]["by_class"].get("S-09") == 3, rec and rec["summary"]["root_markdown"])
r2, rec2 = gen(fj0, os.path.join(SCR, "o0b"))
check("(g) two runs give a byte-identical root-items.json", sha(os.path.join(SCR, "o0", "root-items.json")) == sha(os.path.join(SCR, "o0b", "root-items.json")))

# ---- (b) needle: a planted root entry has no disposition and fails the run
T1 = os.path.join(SCR, "plant")
shutil.copytree(BASE, T1, symlinks=True)
W.w(T1, "zz_planted_root_dir/file.txt", "planted\n")
W.w(T1, "zz_planted_root_file.cfg", "planted\n")
r, rec = gen(freeze(T1, "plant"), os.path.join(SCR, "o1"))
byname = {x["entry"]: x for x in rec["rows"]} if rec else {}
check("(b) needle: the planted root entries appear as rows WITHOUT a disposition", all(byname.get(n, {}).get("disposition", "x") is None for n in ("zz_planted_root_dir", "zz_planted_root_file.cfg")), byname.get("zz_planted_root_dir"))
check("(b) needle: the run FAILS (exit 1) and names them; the file is still written for review", r.returncode == 1 and rec and rec["verdict"] == "FAIL"
      and set(rec["summary"]["without_disposition"]) == {"zz_planted_root_dir", "zz_planted_root_file.cfg"}, (r.returncode, rec and rec["summary"]["without_disposition"]))

# ---- (c) needle: created after the freeze -> absent; the snapshot is never read
T2 = os.path.join(SCR, "late")
shutil.copytree(BASE, T2, symlinks=True)
fj2 = freeze(T2, "late")
W.w(T2, "created_after_freeze.md", "# late\n")
os.makedirs(os.path.join(T2, "late_dir"))
r, rec = gen(fj2, os.path.join(SCR, "o2"))
check("(c) needle: a root entry created in the live tree after the freeze is absent from root-items.json",
      r.returncode == 0 and rec and "created_after_freeze.md" not in {x["entry"] for x in rec["rows"]} and "late_dir" not in {x["entry"] for x in rec["rows"]})
shutil.rmtree(T2)
r, rec = gen(fj2, os.path.join(SCR, "o2b"))
check("(c) the run passes with the snapshot directory DELETED (no snapshot file is read, only the listing and freeze.json)", r.returncode == 0 and rec and rec["verdict"] == "PASS", (r.returncode, r.stderr[-300:]))

# ---- (d) `.git` and a root-level gitlink
r, rec = gen(freeze(BASE, "gitdir", untracked=(".claude", ".git")), os.path.join(SCR, "o3"))
check("(d) `.git` in untracked_root_entries: the run FAILS and no `.git` row is written", r.returncode == 1 and rec and ".git" not in {x["entry"] for x in rec["rows"]}
      and any(".git" in p for p in rec["problems"]), (r.returncode, rec and rec["problems"]))
r, rec = gen(freeze(BASE, "gl", gitlinks=[("vendor_sub", "0" * 40)]), os.path.join(SCR, "o4"))
gl = {x["entry"]: x for x in rec["rows"]} if rec else {}
check("(d) a root-level gitlink is a `gitlink` row (tracked, no listing line)", gl.get("vendor_sub", {}).get("kind") == "gitlink" and gl["vendor_sub"]["tracked"] is True, gl.get("vendor_sub"))
r, rec = gen(freeze(BASE, "glnest", gitlinks=[("submodules/nested", "1" * 40)]), os.path.join(SCR, "o4b"))
check("(d) a nested gitlink makes no new root entry (its root is `submodules`, already a directory row)", rec and len([x for x in rec["rows"] if x["entry"] == "submodules"]) == 1 and r.returncode == 0)

# ---- (e) every rule reachable; the rule table validated
import yaml   # noqa: E402
rules = yaml.safe_load(open(RULES))["rules"]
T3 = os.path.join(SCR, "allrules")
os.makedirs(T3)
want_rule = {}
for rl in rules:
    for n in (rl.get("names") or []):
        want_rule[n] = rl["id"]
    if rl.get("regex"):
        for sample in {"R-S12": ["SOME_STATUS_REPORT.md"], "R-C-COMPOSE": ["docker-compose.zzz.yml"]}.get(rl["id"], ["UNSAMPLED_REGEX_RULE"]):
            want_rule[sample] = rl["id"]
for n in want_rule:
    W.w(T3, n + "/x" if n in (".github", ".claude") else n, "x\n")
fj3 = freeze(T3, "allrules", untracked=())
r, rec = gen(fj3, os.path.join(SCR, "o5"))
got = {x["entry"]: x.get("rule") for x in rec["rows"]} if rec else {}
shadow = {n: (got.get(n), w) for n, w in want_rule.items() if got.get(n) != w}
check("(e) every rule of the table decides its own entries (%d entries, no rule shadowed by an earlier one)" % len(want_rule), not shadow and "UNSAMPLED_REGEX_RULE" not in want_rule, shadow)
check("(e) every disposition of the closed set occurs in the table's output", rec and set(rec["summary"]["by_disposition"]) == {"problem_source", "component", "documentation", "excluded"}, rec and rec["summary"]["by_disposition"])
check("(e) the table's output has no row without a disposition", rec and not rec["summary"]["without_disposition"], rec and rec["summary"]["without_disposition"])
for label, mut, want_reason in (
        ("a disposition outside the closed set", lambda d: d["rules"][0].update(disposition="deleted"), "rules_invalid"),
        ("problem_source without a source class", lambda d: [r.pop("source_class", None) for r in d["rules"] if r["id"] == "R-S03"], "rules_invalid"),
        ("excluded without a reason", lambda d: [r.pop("reason", None) for r in d["rules"] if r["id"] == "R-X-CLAUDE"], "rules_invalid"),
        ("a component without an id", lambda d: [r.pop("component", None) for r in d["rules"] if r["id"] == "R-C-API"], "rules_invalid"),
        ("documentation without doc_class_by", lambda d: [r.pop("doc_class_by", None) for r in d["rules"] if r["id"] == "R-GOV"], "rules_invalid"),
        ("a duplicate rule id", lambda d: d["rules"][1].update(id=d["rules"][0]["id"]), "rules_invalid")):
    d = yaml.safe_load(open(RULES))
    mut(d)
    bad = os.path.join(SCR, "bad_rules.yaml")
    yaml.safe_dump(d, open(bad, "w"))
    r = run("--freeze-json", fj0, "--out", os.path.join(SCR, "o_bad"), "--inventory", DOC01, "--dispositions", bad)
    check("(e) rule table with %s: refused `%s` (exit 20)" % (label, want_reason), r.returncode == 20 and want_reason.encode() in r.stderr, (r.returncode, r.stderr[-200:]))

# ---- (f) --check
good = os.path.join(SCR, "o0", "root-items.json")
r = run("--check", good, "--freeze-json", fj0)
check("(f) --check of a generated file passes (with the freeze)", r.returncode == 0, r.stdout[-300:] + r.stderr[-200:])


def mutated(fn):
    d = json.load(open(good))
    fn(d)
    p = os.path.join(SCR, "chk.json")
    json.dump(d, open(p, "w"))
    return p


cases = (
    ("a removed disposition", lambda d: d["rows"][3].update(disposition=None), 1),
    ("an unknown disposition", lambda d: d["rows"][3].update(disposition="deleted"), 1),
    ("a `.git` row (inserted in sorted position)", lambda d: (d["rows"].append({"entry": ".git", "disposition": "excluded", "reason": "x", "docs01": "absent", "kind": "directory", "tracked": True, "tracked_files": 0}), d["rows"].sort(key=lambda r: r["entry"].encode())), 1),
    ("a duplicate row", lambda d: d["rows"].insert(1, dict(d["rows"][0])), 1),
    ("an unsorted file", lambda d: d["rows"].reverse(), 1),
    ("a problem_source row without a source class", lambda d: [r.pop("source_class") for r in d["rows"] if r["disposition"] == "problem_source"][:1], 1),
    ("an excluded row without a reason", lambda d: [r.pop("reason") for r in d["rows"] if r["disposition"] == "excluded"][:1], 1),
    ("a documentation row without doc_class", lambda d: [r.pop("doc_class") for r in d["rows"] if r["disposition"] == "documentation"][:1], 1),
    ("a missing row (the freeze has the entry)", lambda d: d["rows"].pop(2), 1),
    ("an extra row (the freeze lacks the entry; sorted position)", lambda d: (d["rows"].append({"entry": "zzz_extra", "disposition": "excluded", "reason": "x", "docs01": "absent", "kind": "file", "tracked": True, "tracked_files": 1}), d["rows"].sort(key=lambda r: r["entry"].encode())), 1),
)
for label, fn, rc in cases:
    r = run("--check", mutated(fn), "--freeze-json", fj0)
    check("(f) --check of a file with %s fails (exit 1)" % label, r.returncode == rc, (r.returncode, r.stdout[-200:]))
r = run("--check", os.path.join(SCR, "nope.json"))
check("(f) --check of an unreadable file is refused `root_items_invalid` (exit 20)", r.returncode == 20 and b"root_items_invalid" in r.stderr, r.returncode)

# ---- (g) listing moved
T4 = os.path.join(SCR, "lm")
shutil.copytree(BASE, T4, symlinks=True)
fj4 = freeze(T4, "lm")
with open(json.load(open(fj4))["listing"], "ab") as f:
    f.write(b"sneaked/in.md\0")
r, rec = gen(fj4, os.path.join(SCR, "o6"))
check("(g) listing moved after the freeze: refused `freeze_listing_moved` (exit 20), nothing written", r.returncode == 20 and b"freeze_listing_moved" in r.stderr and rec is None, (r.returncode, r.stderr[-200:]))
r = run()
check("(g) usage: no arguments exits 2", r.returncode == 2, r.returncode)

# ---- (h) docs/01 lookup, (i) config's two files
T5 = os.path.join(SCR, "lookup")
os.makedirs(T5)
for p in ("Website/x.md", "database/schema.sql", "catalog-api/main.go", "examples/helixcode-qa-integration/main.go", "LICENSE", "Upstreams/a.sh", "config/nginx.conf", "config/index/scope.yaml"):
    W.w(T5, p, "x\n")
r, rec = gen(freeze(T5, "lookup", untracked=(".claude",)), os.path.join(SCR, "o7"))
d = {x["entry"]: x for x in rec["rows"]} if rec else {}
sec = lambda n: (d[n]["docs01"]["section"], d[n]["docs01"]["kind"]) if isinstance(d.get(n, {}).get("docs01"), dict) else d.get(n, {}).get("docs01")
check("(h) a 3.7 table row is found for a root path (Website -> 3.7)", sec("Website") == ("3.7", "table-row"), sec("Website"))
check("(h) a 3.8 table row is found for a root file and a ignored directory (LICENSE, .claude)", sec("LICENSE") == ("3.8", "table-row") and sec(".claude") == ("3.8", "table-row"), (sec("LICENSE"), sec(".claude")))
check("(h) a heading of 3.1 is found (catalog-api -> 3.1 heading)", sec("catalog-api") == ("3.1", "heading"), sec("catalog-api"))
check("(h) an application-relative path of 3.1 is NOT a root row (database -> absent)", sec("database") == "absent", sec("database"))
check("(h) a sub-path row is the weakest evidence for its root (examples -> table-row-subpath)", sec("examples") == ("3.7", "table-row-subpath"), sec("examples"))
check("(h) a path named only in sections 2 and 4 is absent (Upstreams -> absent)", sec("Upstreams") == "absent", sec("Upstreams"))
cf = d.get("config", {})
check("(i) config names the two index files individually, each excluded with a reason, with their listing state",
      [(x["path"], x["in_frozen_listing"]) for x in cf.get("excluded_files", [])] == [("config/index/scope.yaml", True), ("config/index/lumen_scope.json", False)] and all(x["reason"] for x in cf.get("excluded_files", [])), cf.get("excluded_files"))

# ---- (j) the real listing leg
fjr = os.environ.get("REAL_FREEZE_JSON")
if fjr:
    rr = run("--freeze-json", fjr, "--out", os.path.join(SCR, "o_real"))
    rec = json.load(open(os.path.join(SCR, "o_real", "root-items.json"))) if os.path.exists(os.path.join(SCR, "o_real", "root-items.json")) else None
    check("(j) real listing: the run passes (no root entry without a disposition)", rr.returncode == 0 and rec and rec["verdict"] == "PASS", (rr.returncode, rec and rec["summary"]["without_disposition"], rr.stdout[-400:]))
    if rec:
        check("(j) real listing: all four dispositions occur (%s)" % rec["summary"]["by_disposition"], set(rec["summary"]["by_disposition"]) == {"problem_source", "component", "documentation", "excluded"})
        exp = os.environ.get("EXPECT_ROOT_MD", "32,9,8").split(",")
        bc = rec["summary"]["root_markdown"]["by_class"]
        other = sum(v for k, v in bc.items() if k not in ("S-12", "governance"))
        check("(j) real listing: the root Markdown split is the T174 text (%s S-12, %s other classes, %s governance = %d)" % (exp[0], exp[1], exp[2], rec["summary"]["root_markdown"]["tracked_files"]),
              (bc.get("S-12"), other, bc.get("governance")) == tuple(int(x) for x in exp), bc)
        check("(j) real listing: the entry count equals an independent count of first path components of the listing",
              rec["summary"]["entries"] == len({q.decode().split("/")[0] for q in open(json.load(open(fjr))["listing"], "rb").read().split(b"\0")[:-1]} | set(json.load(open(fjr)).get("untracked_root_entries", []))))
else:
    print("# (j) real listing leg SKIPPED: REAL_FREEZE_JSON not set")

shutil.rmtree(SCR, ignore_errors=True)
print("RESULT: pass=%d fail=%d" % (PASSES, FAILS))
sys.exit(1 if FAILS else 0)
