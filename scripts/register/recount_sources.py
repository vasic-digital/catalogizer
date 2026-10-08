#!/usr/bin/env python3
"""recount_sources.py - the engine of scripts/register/recount_sources.sh (T166, WP-20; doc04 section 13.3 item 2, doc03 section 15 item 1).

Independent recount of every Stage 0 source: a SECOND TOOL counts the entries of each source in the frozen snapshot with `grep -c -H` (and, where a
count is not a line count, `grep -o`, `awk` or a plain re-read of the file) and compares the result with `reg_sources.scanned_entry_count` of the
register database. It is not the enumerator (it shares no code with enumerate_sources.py), it reads the listing of the freeze (never `git`, never the live
tree) and the frozen snapshot only. The per-source totals are then compared with the doc03 section 7 table and every delta is itemised.

Usage: recount_sources.py --freeze-json F --db DB --out DIR [--explanations FILE] [--doc03 FILE]
  --freeze-json F    freeze json {snapshot, manifest, listing, listing_sha256, remotes, ...}; REQUIRED, no default snapshot, never the live tree
  --db DB            register database holding the Stage 0 rows (reg_sources, reg_source_entries); opened read-only
  --out DIR          receives recount.json (written only after both freeze checks and the whole recount; a refusal writes nothing)
  --explanations F   JSON {"<source locator>": "<why the two counts differ>"}; a differing source that is explained is `explained`, not a failure
  --doc03 FILE       doc03 for the section 7 comparison (default: the copy in the frozen snapshot)
Order of work: scripts/register/check_freeze_listing.sh (`freeze_listing_moved`), then scripts/register/check_freeze_snapshot.sh
(`freeze_snapshot_moved`); either refusal stops the recount before any count is made (round-31 review I1).
Exit: 0 every source matches or is explained; 1 a source differs without an explanation (the differing locators are itemised in recount.json and on
stdout), 2 usage, 20 refusal (`recount: REFUSED reason=<code> ...`, nothing written).
Every command that receives listing paths as arguments is `xargs -0 grep ... -- <paths>` run inside the snapshot directory (T161 round-30 m5: a path
beginning with `-` is otherwise an option; grep prints `path NUL count` with -Z, so a path holding a newline is not mis-split).
Counting units (the SAME units as the enumerator, restated here in grep/awk terms; a difference is a finding, never silently dropped):
  file entry          one per listing path of the source (a symlink counts, it is hashed as its target string)
  checkbox line       `^[[:space:]]*[-*+][[:space:]]+\\[( |x|X)\\][[:space:]]+`  (ERE, C locale)
  mark line           a line holding one of the cross glyphs or the warning glyph (fixed strings)
  Recount methods marked approximate (`exact: false`) restate a rule that grep cannot express exactly (bank case nesting, conflict sub-items,
  scan records); on the real corpus any difference of those is itemised and must be explained in --explanations.
"""
import argparse
import json
import os
import re
import sqlite3
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ENV = dict(os.environ, LC_ALL="C")
CHECKBOX = r"^[[:space:]]*[-*+][[:space:]]+\[( |x|X)\][[:space:]]+"
CROSS = ("\u274c", "\u2717", "\u2718")          # the three cross glyphs
WARN = "\u26a0"
MAX_SCAN_BYTES = 2 * 1024 * 1024
GOVERNANCE_ROOT = ("AGENTS.md", "CLAUDE.md", "CONSTITUTION.md", "GEMINI.md", "GETTING_STARTED.md", "MEMORY.md", "QUICK_REFERENCE.md", "README.md")
S12_EXTRA = ("docs/COMPREHENSIVE_PACKAGE_SUMMARY.md", "docs/README_IMPLEMENTATION_PACKAGE.md")
EARLIER_ROOT = ("TASK_TRACKER.md", "MASTER_EXECUTION_CHECKLIST.md", "COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md", "COMPREHENSIVE_UNFINISHED_WORK_REPORT.md",
                "UNFINISHED_WORK_ANALYSIS.md", "UNFINISHED_WORK_AND_ISSUES.md", "FINAL_UNFINISHED_WORK_REPORT.md",
                "SECURITY_KEY_ROTATION_REQUIRED.md", "SECURITY_AUDIT_REPORT.md")
ID_FAMILIES = (r"CATAPI-DEFECT-[0-9]+", r"(FIX|DEFER)-QA-[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]{3}", r"DEFER-[0-9]{3}", r"FIX-OC[0-9]+-[0-9]+", r"FINDING-[0-9]+",
               r"HQA-DOCS-[0-9]+", r"HQA-[0-9]{4}", r"HQA-PHASE[0-9]+-[A-Z]+-[0-9]+", r"FIX-(OBS|BROWSER)-[0-9]+", r"FIX-[0-9]{3}", r"BUG-[0-9]{3}",
               r"FIX-CONCURRENCY-[0-9]{4}-[0-9]{2}-[0-9]{2}", r"FIX-CATAPI-[0-9]{4}-[0-9]{2}-[0-9]{2}-[A-Z]+")
ID_EXCLUDE = ("specs/", ".audit/", "submodules/", "scripts/register/tests/")
SKIP_PCRE = (r"\b(?:t|b|tb)\.(?:Skip|SkipNow|Skipf)\(",                                   # go (both Go regexes of the enumerator)
             r"(?<![\w.])(?:it|test|describe)\.skip\(", r"(?<![\w.$])(?:xit|xdescribe|xtest)\(",   # ts
             r"@Ignore\b", r"#\[ignore\b", r"@pytest\.mark\.skip\b|@unittest\.skip\b")             # kt, rs, py
SERVICES = (("firebase-crashlytics", (".firebaserc", "firebase.json")), ("sonarqube", ("sonar-project.properties", "sonarqube/")),
            ("snyk", (".snyk", ".snyk.json")), ("trivy", (".trivyignore", ".trivy.yaml", "config/trivy/")))
DOC21 = "specs/001-full-project-audit-remediation/docs/21-master-plan-phases-risks-and-traceability.md"


class Refusal(Exception):
    def __init__(self, reason, detail=""):
        Exception.__init__(self, reason)
        self.reason, self.detail = reason, detail


def refuse(reason, detail=""):
    raise Refusal(reason, detail)


# ---------------------------------------------------------------------------------------------- the population (listing)
class Pop:
    def __init__(self, fj):
        self.snap = fj["snapshot"]
        data = open(fj["listing"], "rb").read()
        self.paths = []
        for r in data.split(b"\0")[:-1]:
            try:
                self.paths.append(r.decode("utf-8"))
            except UnicodeDecodeError:
                refuse("freeze_path_unsafe", repr(r))
        self.paths.sort(key=lambda p: p.encode("utf-8"))
        self.links = {p for p in self.paths if os.path.islink(os.path.join(self.snap, p))}
        self.fj = fj

    def select(self, pred):
        return [p for p in self.paths if pred(p)]

    def readable(self, files):
        return [p for p in files if p not in self.links and os.path.isfile(os.path.join(self.snap, p))]


def xargs_grep(pop, files, gargs, count=True):
    """xargs -0 grep <gargs> -- <files> inside the snapshot; -> {path: n} for -c, or [(path, line, text)] for -o -n. A file grep cannot read is an error."""
    if not files:
        return {} if count else []
    cmd = ["xargs", "-0", "grep", "-a", "-H", "-Z"] + (["-c"] if count else ["-n", "-o"]) + list(gargs) + ["--"]    # MUT:dashdash
    r = subprocess.run(cmd, cwd=pop.snap, input=b"\0".join(f.encode("utf-8") for f in files) + b"\0", stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=ENV)
    if r.returncode not in (0, 123):      # xargs: 123 = some grep exited 1 (no match in a file); 2 or 125+ is an error
        refuse("recount_instrument_error", "xargs/grep exited %d: %s" % (r.returncode, r.stderr.decode("utf-8", "replace")[:300]))
    if r.stderr.strip():
        refuse("recount_instrument_error", r.stderr.decode("utf-8", "replace")[:300])
    out = r.stdout
    if count:
        res = {f: 0 for f in files}
        for m in re.finditer(rb"([^\0]*)\0(\d+)\n", out):
            res[m.group(1).decode("utf-8")] = int(m.group(2))
        return res
    return [(m.group(1).decode("utf-8"), int(m.group(2)), m.group(3).decode("utf-8", "replace")) for m in re.finditer(rb"([^\0]*)\0(\d+):([^\n]*)\n", out)]


def gcount(pop, files, *patterns, fixed=False, pcre=False):
    """lines matching ANY of the patterns, per file (grep -c -H -e P1 -e P2 ...)"""
    a = ["-F" if fixed else ("-P" if pcre else "-E")]
    for p in patterns:
        a += ["-e", p]
    return xargs_grep(pop, pop.readable(files), a)


def awk_per_file(pop, files, prog):
    """run one awk program per file inside the snapshot; the program prints one integer"""
    res = {}
    for f in pop.readable(files):
        r = subprocess.run(["awk", prog, "./" + f], cwd=pop.snap, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=ENV)
        if r.returncode != 0:
            refuse("recount_instrument_error", "awk on %s: %s" % (f, r.stderr.decode("utf-8", "replace")[:200]))
        res[f] = int(r.stdout.decode().strip() or 0)
    return res


def add(*dicts):
    out = {}
    for d in dicts:
        for k, v in d.items():
            out[k] = out.get(k, 0) + v
    return out


def one_each(pop, files):
    return {f: 1 for f in files}


def mul_glob(pat):
    return re.compile("^" + pat + "$")


# ---------------------------------------------------------------------------------------------- the methods, one per source locator
def m_files(sel):
    return lambda pop, ctx: (one_each(pop, pop.select(sel)), True, "one entry per listing path")


def m_lines(sel, *pats, fixed=False, with_files=False):
    def f(pop, ctx):
        files = pop.select(sel)
        d = gcount(pop, files, *pats, fixed=fixed)
        if with_files:
            d = add(d, one_each(pop, files))
        return (d, True, "grep -c -H lines" + (" plus one file entry per path" if with_files else ""))
    return f


def m_task_rows(pop, ctx):
    return gcount(pop, ["TASK_TRACKER.md"] if "TASK_TRACKER.md" in pop.paths else [], r"^\| *[0-9]+\.[0-9]+"), True, "grep -c table rows `| N.N`"


def m_landmines(pop, ctx):
    files = pop.select(lambda p: p == "docs/LANDMINES.md")
    ids = {}
    for f in pop.readable(files):
        o = subprocess.run(["grep", "-a", "-o", "-E", r"RULE-[A-Za-z0-9]+-[0-9]+", "--", f], cwd=pop.snap, stdout=subprocess.PIPE, env=ENV).stdout.decode("utf-8", "replace")
        ids[f] = len(set(o.split()))
    return ids, True, "distinct RULE ids (grep -o)"


def s12_files(pop):
    earlier = set(EARLIER_ROOT)
    return pop.select(lambda p: (("/" not in p and p.endswith(".md") and p not in GOVERNANCE_ROOT) or p in S12_EXTRA) and p not in earlier)


def m_s12(pop, ctx):
    files = s12_files(pop)
    return add(one_each(pop, files), gcount(pop, files, CHECKBOX)), True, "root status reports + 2 docs files: one entry per file plus checkbox lines"


def audit_files(pop):
    return pop.select(lambda p: re.match(r"^docs/audits/[^/]*\.md$", p) or re.match(r"^docs/[^/]*AUDIT[^/]*\.md$", p))


def qa_files(pop):
    return pop.select(lambda p: p.startswith("docs/qa/") or p.startswith("docs/reports/qa-sessions/"))


def m_s14_audits(sel_re):
    def f(pop, ctx):
        files = pop.select(lambda p: re.match(sel_re, p))
        return add(one_each(pop, files), gcount(pop, files, CHECKBOX)), True, "one entry per file plus checkbox lines"
    return f


def m_s22(pop, ctx):
    files = pop.select(lambda p: os.path.basename(p) in ("CLAUDE.md", "AGENTS.md") and "/" in p and not p.startswith(("submodules/", ".audit/")))
    return one_each(pop, files), True, "module CLAUDE.md and AGENTS.md outside submodules"


def m_providers(name):
    prog = ('/^\\|[[:space:]]*Provider[[:space:]]*\\|/{h=1;next} h&&/^\\|/{ if ($0 !~ /^\\|[[:space:]:|-]+\\|?[[:space:]]*$/) n++; next } h&&!/^\\|/{h=0} END{print n+0}')

    def f(pop, ctx):
        files = pop.select(lambda p: p == name)
        if os.path.basename(name) == "SECURITY_KEY_ROTATION_REQUIRED.md":
            return awk_per_file(pop, files, prog), True, "awk: table rows after the Provider header"
        return one_each(pop, files), True, "one file entry"
    return f


def m_bank_yaml(pop, ctx):
    files = pop.select(lambda p: re.match(r"^challenges/helixqa-banks/[^/]*\.yaml$", p))
    # cases: list items `- id:` at the shallowest indentation that holds one (nested step lists with ids are not cases)
    prog = ('/^[[:space:]]*-[[:space:]]+id:/{ m=match($0,/[^ \\t]/); if (min==0 || m<min) min=m; c[m]++ } END{print c[min]+0}')
    return add(one_each(pop, files), awk_per_file(pop, files, prog)), False, "file entries + `- id:` items at the shallowest indentation (approximate)"


def m_bank_json(pop, ctx):
    files = pop.select(lambda p: p == "challenges/data/challenges_bank.json")
    ids = {}
    for f, _n, _t in xargs_grep(pop, pop.readable(files), ["-E", "-o", "-e", r'"id"[[:space:]]*:'], count=False):
        ids[f] = ids.get(f, 0) + 1
    return add(one_each(pop, files), ids), False, "file entry + occurrences of an \"id\" key (approximate: nested ids count)"


def m_bank_sub(pop, ctx):
    files = pop.select(lambda p: p.startswith("submodules/helix_qa/banks/"))
    prog = ('/^[[:space:]]*-?[[:space:]]*"?id"?:/{ n++ } END{print n+0}')
    return add(one_each(pop, files), awk_per_file(pop, [f for f in files if f.endswith((".yaml", ".yml", ".json"))], prog)), False, "file entries + id lines (approximate)"


def m_bluff(pop, ctx):
    a = pop.select(lambda p: p == "submodules/helix_qa/challenges/baselines/bluff-baseline.txt")
    b = pop.select(lambda p: p == "submodules/helix_qa/docs/behavior-anchors.md")
    res = {}
    for f in pop.readable(a):
        res[f] = len(subprocess.run(["grep", "-a", "-v", "-E", "-e", r"^[[:space:]]*$", "-e", r"^[[:space:]]*#", "--", f], cwd=pop.snap, stdout=subprocess.PIPE, env=ENV).stdout.splitlines())
    for f in pop.readable(b):
        o = subprocess.run(["grep", "-a", "-o", "-E", r"^\|[[:space:]]*CAP-[0-9]+[[:space:]]*\|", "--", f], cwd=pop.snap, stdout=subprocess.PIPE, env=ENV).stdout.decode("utf-8", "replace")
        res[f] = len({re.search(r"CAP-[0-9]+", x).group(0) for x in o.splitlines()})
    return res, True, "bluff baseline lines (not blank, not #) + distinct CAP ids"


def m_conflicts(pop, ctx):
    files = pop.select(lambda p: p == ".specify/memory/constitution.md")
    prog = r'''
function bold(s,   n,t){ n=0; t=s; while (match(t, /\*\*(FIXED|DECIDED|OPEN|NOTE|UNCONFIRMED)[^*]*\*\*/)) { n++; t=substr(t, RSTART+RLENGTH) } return n }
/^##[[:space:]]+Known Conflicts/ { on=1; next }
on && /^##[[:space:]]/ { on=0 }
on && /^[0-9]+\.[[:space:]]/ { items++; if (blockmarks>=2) total+=blockmarks; blockmarks=0; inblk=1; blockmarks+=bold($0); next }
on && inblk { if ($0 ~ /^[[:space:]][[:space:]]+([-*]|[0-9]+[.)]|\([a-z0-9]+\)|[a-z]\))[[:space:]]+/) subs++; blockmarks+=bold($0) }
END { if (blockmarks>=2) total+=blockmarks; print items+subs+total }'''
    return awk_per_file(pop, files, prog), False, "awk: numbered items + indented bullets + bold-marker segments of multi-marker items (approximate)"


def m_security(pop, ctx):
    files = pop.select(lambda p: p.startswith("docs/security/") and p != "docs/security/firebase-api-key-exposure-20260629.md")   # that file is claimed earlier by S-25
    res = one_each(pop, files)
    latest = {}
    for p in files:
        m = re.match(r"^(.+)-(\d{8}_\d{6})(\.[a-z]+)$", os.path.basename(p))
        if m and m.group(3) in (".json", ".txt"):
            k = (m.group(1), m.group(3))
            if k not in latest or m.group(2) > latest[k][0]:
                latest[k] = (m.group(2), p)
    for k in sorted(latest):
        p, tool = latest[k][1], k[0]
        if p in pop.links:
            continue
        full = os.path.join(pop.snap, p)
        if p.endswith(".txt"):
            res[p] = res.get(p, 0) + gcount(pop, [p], r"^Vulnerability #[0-9]+:[[:space:]]*[^[:space:]]+").get(p, 0)
            continue
        try:
            data = json.load(open(full, encoding="utf-8"))
        except (OSError, ValueError):
            res[p] = res.get(p, 0) + 1      # an unparsable scan file is one scan-failed row
            continue
        n = 0
        if isinstance(data, dict):
            if tool.startswith("gosec"):
                n = len(xargs_grep(pop, [p], ["-E", "-o", "-e", r'"rule_id"'], count=False))      # occurrences: a one-line JSON file holds many on one line
            elif tool.startswith("npm-audit"):
                v = data.get("vulnerabilities") or data.get("advisories") or {}
                n = len(v) if isinstance(v, (dict, list)) else 0
            elif tool.startswith("nancy"):
                n = len(data.get("vulnerable") or [])
            elif tool.startswith("snyk"):
                if data.get("error") or (data.get("ok") is False and not data.get("vulnerabilities")):
                    n += 1
                n += len(data.get("vulnerabilities") or [])
        res[p] = res.get(p, 0) + n
    return res, False, "files + records of the latest scan per tool (gosec rule_id occurrences, other tools re-read as JSON)"


def populations_for_ids(pop):
    cand = pop.select(lambda p: not p.startswith(ID_EXCLUDE))
    cand = [p for p in cand if p not in pop.links and os.path.getsize(os.path.join(pop.snap, p)) <= MAX_SCAN_BYTES]
    return cand


def ids_by_home(pop):
    """distinct defect id -> home class (S-14 / S-15 / XID): first occurrence inside the S-14 files, else the S-15 files, else anywhere"""
    cand = populations_for_ids(pop)
    pat = r"(^|[^A-Za-z0-9-])(" + "|".join(ID_FAMILIES) + ")"
    hits = xargs_grep(pop, cand, ["-I", "-E", "-e", pat], count=False)   # -o prints the match only; the boundary char is stripped below
    occ = {}
    for p, n, txt in hits:
        t = re.sub(r"^[^A-Z]", "", txt)
        t = re.sub(r"[^A-Za-z0-9]+$", "", t)
        # the match text may still hold a trailing context char of the alternation; keep only the leading id-shaped token
        mm = re.match(r"(?:" + "|".join(ID_FAMILIES) + ")", t)
        if mm:
            occ.setdefault(mm.group(0), []).append((p.encode("utf-8"), n, p))
    s14 = set(audit_files(pop)); s15 = set(qa_files(pop))
    home = {}
    for i, lst in occ.items():
        lst.sort()
        c = "XID"
        for cls, fs in (("S-14", s14), ("S-15", s15)):
            if any(p in fs for _b, _n, p in lst):
                c = cls
                break
        home[i] = c
    return home


def m_ids(cls):
    def f(pop, ctx):
        if "ids" not in ctx:
            ctx["ids"] = ids_by_home(pop)
        return {i: 1 for i, c in ctx["ids"].items() if c == cls}, True, "grep -o census of the defect-id families, home class by first occurrence"
    return f


def m_markers(pop, ctx):
    cand = [p for p in pop.paths if p not in pop.links and os.path.isfile(os.path.join(pop.snap, p)) and os.path.getsize(os.path.join(pop.snap, p)) <= MAX_SCAN_BYTES]
    res = {}
    for p, n, txt in xargs_grep(pop, cand, ["-I", "-w", "-E", "-e", "TODO|FIXME|HACK|XXX"], count=False):
        res[p] = res.get(p, 0) + 1
    for pc in SKIP_PCRE:
        for p, n, txt in xargs_grep(pop, cand, ["-I", "-P", "-e", pc], count=False):
            res[p] = res.get(p, 0) + 1
    return res, True, "grep -o -I occurrences of TODO/FIXME/HACK/XXX (word) and of the skip patterns"


def m_remotes(pop, ctx):
    return {"remotes": len(pop.fj.get("remotes") or [])}, True, "len(freeze.json remotes)"


def m_services(pop, ctx):
    return {"services": len(SERVICES)}, True, "the four named services of doc03 5.20 (one row each, found or absent)"


def awk_doc21(prog):
    def f(pop, ctx):
        files = pop.select(lambda p: p == DOC21)
        return awk_per_file(pop, files, prog), True, "awk over the docs/21 table"
    return f


S26_PROG = r'''/^### 9\.1[[:space:]]/ {on=1; next} on && /^### / {on=0} on && /^\|/ && $0 !~ /^\|[ :|-]+\|?[ ]*$/ && $0 !~ /^\| *Source *\|/ && $0 !~ /Total itemised/ { n=split($0, c, "|"); gsub(/[ ,]/, "", c[5]); if (c[5] ~ /^[0-9]+$/) s+=c[5] } END{print s+0}'''
S27_PROG = r'''/^### 9\.5[[:space:]]/ {on=1; next} on && /^### |^---$/ {on=0} on && /^\| T[0-9]+-[A-F] \|/ {n++} END{print n+0}'''

# locator -> method (the locators are the enumerator's source locators: the source identity in reg_sources)
METHODS = {
    "S-25:SECURITY_KEY_ROTATION_REQUIRED.md": m_providers("SECURITY_KEY_ROTATION_REQUIRED.md"),
    "S-25:SECURITY_AUDIT_REPORT.md": m_providers("SECURITY_AUDIT_REPORT.md"),
    "docs/security/firebase-api-key-exposure-20260629.md": m_providers("docs/security/firebase-api-key-exposure-20260629.md"),
    "TASK_TRACKER.md": m_task_rows,
    "MASTER_EXECUTION_CHECKLIST.md": m_lines(lambda p: p == "MASTER_EXECUTION_CHECKLIST.md", CHECKBOX),
    "docs/MASTER_EXECUTION_CHECKLIST.md": m_lines(lambda p: p == "docs/MASTER_EXECUTION_CHECKLIST.md", CHECKBOX),
    "COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md": m_lines(lambda p: p == "COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md", CHECKBOX),
    "COMPREHENSIVE_UNFINISHED_WORK_REPORT.md": m_lines(lambda p: p == "COMPREHENSIVE_UNFINISHED_WORK_REPORT.md", CHECKBOX, *CROSS),
    "docs/UNFINISHED_WORK_COMPREHENSIVE_REPORT.md": m_lines(lambda p: p == "docs/UNFINISHED_WORK_COMPREHENSIVE_REPORT.md", *(CROSS + (WARN,)), fixed=False),
    "UNFINISHED_WORK_ANALYSIS.md+UNFINISHED_WORK_AND_ISSUES.md+FINAL_UNFINISHED_WORK_REPORT.md":
        m_lines(lambda p: p in ("UNFINISHED_WORK_ANALYSIS.md", "UNFINISHED_WORK_AND_ISSUES.md", "FINAL_UNFINISHED_WORK_REPORT.md"), *CROSS),
    "docs/OPEN_POINTS_CLOSURE.md": m_lines(lambda p: p == "docs/OPEN_POINTS_CLOSURE.md", CHECKBOX),
    "docs/LANDMINES.md": m_landmines,
    "/*.md (other root status reports)+docs/COMPREHENSIVE_PACKAGE_SUMMARY.md+docs/README_IMPLEMENTATION_PACKAGE.md": m_s12,
    "docs/status/*.md": m_files(lambda p: re.match(r"^docs/status/[^/]*\.md$", p)),
    "docs/audits/*.md": m_s14_audits(r"^docs/audits/[^/]*\.md$"),
    "docs/*AUDIT*.md": m_s14_audits(r"^docs/[^/]*AUDIT[^/]*\.md$"),
    "docs/issues/*.md": m_files(lambda p: re.match(r"^docs/issues/[^/]*\.md$", p)),
    "issues/*.md": m_files(lambda p: re.match(r"^issues/[^/]*\.md$", p)),
    "challenges/helixqa-banks/*.yaml": m_bank_yaml,
    "challenges/data/challenges_bank.json": m_bank_json,
    "submodules/helix_qa/banks/**": m_bank_sub,
    "submodules/helix_qa/challenges/baselines/bluff-baseline.txt+submodules/helix_qa/docs/behavior-anchors.md": m_bluff,
    ".specify/memory/constitution.md": m_conflicts,
    "module CLAUDE.md and AGENTS.md (outside submodules)": m_s22,
    ".implementation/**": m_files(lambda p: p.startswith(".implementation/")),
    "docs/qa/**+docs/reports/qa-sessions/**": lambda pop, ctx: (one_each(pop, qa_files(pop)), True, "one entry per file"),
    "legacy-ids:defect ids first seen in the S-14 files": m_ids("S-14"),
    "legacy-ids:defect ids first seen in the S-15 files": m_ids("S-15"),
    "legacy-ids:defect ids referenced only outside the S-14 and S-15 files": m_ids("XID"),
    "docs/security/**": m_security,
    "marker and skipped-test scan of every text file of the snapshot": m_markers,
    "external:git remotes recorded in the freeze json": m_remotes,
    "external:named services (firebase-crashlytics, sonarqube, snyk, trivy)": m_services,
    "plan-document seeds (docs/21 section 9.1)": awk_doc21(S26_PROG),
    "innovation entries (docs/21 section 9.5)": awk_doc21(S27_PROG),
}
WHOLE_KEY = ("plan-document seeds (docs/21 section 9.1)", "innovation entries (docs/21 section 9.5)", "external:git remotes recorded in the freeze json",
             "external:named services (firebase-crashlytics, sonarqube, snyk, trivy)")
ID_KEY = ("legacy-ids:defect ids first seen in the S-14 files", "legacy-ids:defect ids first seen in the S-15 files",
          "legacy-ids:defect ids referenced only outside the S-14 and S-15 files")


def entry_key(locator, paths_set, loc_of_source):
    if loc_of_source in ID_KEY:
        return locator.rsplit("#", 1)[-1]
    if loc_of_source in WHOLE_KEY:
        return {"external:git remotes recorded in the freeze json": "remotes", "external:named services (firebase-crashlytics, sonarqube, snyk, trivy)": "services"}.get(loc_of_source, "total")
    # longest listing path that is a prefix of the locator followed by end, `:L<n>` or `#`
    best = None
    for i in range(len(locator), 0, -1):
        if (i == len(locator) or locator[i] in ":#") and locator[:i] in paths_set:
            best = locator[:i]
            break
    return best if best is not None else locator


# ---------------------------------------------------------------------------------------------- doc03 section 7 comparison
DOC03 = (   # (group label as written in doc03 section 7, doc03 value, source locator, comparable measure)
    ("HelixQA ticket files (S-01)", 1778, "docs/issues/*.md", "entries"),
    ("ANR ticket (S-02)", 1, "issues/*.md", "entries"),
    ("Landmine rules (S-11)", 63, "docs/LANDMINES.md", "entries"),
    ("Credential-rotation rows (S-25)", 37, "S-25:SECURITY_KEY_ROTATION_REQUIRED.md", "entries"),
    ("Constitution Known Conflicts (S-21) items", 16, ".specify/memory/constitution.md", "numbered items"),
)


def doc03_compare(pop, doc03_path, per_source):
    """-> list of {group, doc03_value, measured, delta, status}; a value that doc03 no longer holds is `doc03_value_not_found`"""
    out = []
    try:
        text = open(doc03_path, encoding="utf-8").read()
    except OSError:
        return [{"group": "doc03 section 7", "status": "doc03_unreadable", "path": doc03_path}]
    sec = text.split("## 7. Table of totals", 1)[-1].split("\n## 8.", 1)[0]
    for label, val, loc, what in DOC03:
        row = next((l for l in sec.split("\n") if l.startswith("|") and label.split(" (")[0] in l), None)
        found = row is not None and re.search(r"(?<![0-9,])" + format(val, ",") .replace(",", ",?") + r"(?![0-9])", row) is not None
        if what == "numbered items":
            measured = None
            files = pop.select(lambda p: p == ".specify/memory/constitution.md")
            if files:
                measured = awk_per_file(pop, files, r'/^##[[:space:]]+Known Conflicts/{on=1;next} on&&/^##[[:space:]]/{on=0} on&&/^[0-9]+\.[[:space:]]/{n++} END{print n+0}').get(files[0], 0)
        else:
            measured = per_source.get(loc)
        rec = {"group": label, "doc03_value": val, "measured": measured, "measure": what}
        if not found:
            rec["status"] = "doc03_value_not_found"
        elif measured is None:
            rec["status"] = "source_absent"
        else:
            rec["delta"] = measured - val
            rec["status"] = "equal" if measured == val else "delta"
        out.append(rec)
    return out


# ---------------------------------------------------------------------------------------------- run
def sha256_file(path):
    import hashlib
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for c in iter(lambda: f.read(1 << 20), b""):
            h.update(c)
    return h.hexdigest()


def run(a):
    try:
        fj = json.load(open(a.freeze_json, encoding="utf-8"))
        snapdir, manifest = fj["snapshot"], fj["manifest"]
    except (OSError, ValueError, KeyError):
        refuse("freeze_json_invalid", a.freeze_json)
    if not os.path.isdir(snapdir) or not os.path.isfile(manifest):
        refuse("freeze_json_invalid", "snapshot or manifest path does not exist")
    lchk = os.path.join(HERE, "check_freeze_listing.sh")
    r = subprocess.run(["bash", lchk, a.freeze_json], stdout=subprocess.PIPE, stderr=subprocess.PIPE)   # MUT:listing-check
    if r.returncode != 0:
        err = r.stderr.decode("utf-8", "replace")
        sys.stderr.write(err)
        m = re.search(r"reason=(\w+)", err)
        refuse(m.group(1) if m else "freeze_listing_moved", "check_freeze_listing.sh exited %d" % r.returncode)
    schk = os.path.join(HERE, "check_freeze_snapshot.sh")
    r = subprocess.run(["bash", schk, snapdir, manifest], stdout=subprocess.PIPE, stderr=subprocess.PIPE)   # MUT:snapshot-check
    if r.returncode != 0:
        err = r.stderr.decode("utf-8", "replace")
        sys.stderr.write(err)
        mm = re.search(r"REFUSED reason=(freeze_manifest_invalid|freeze_special_file|freeze_path_unsafe)", err)
        refuse(mm.group(1) if mm else "freeze_snapshot_moved", "check_freeze_snapshot.sh exited %d" % r.returncode)
    explanations = {}
    if a.explanations:
        try:
            explanations = json.load(open(a.explanations, encoding="utf-8"))
            assert isinstance(explanations, dict)
        except (OSError, ValueError, AssertionError):
            refuse("explanations_invalid", a.explanations)
    if not os.path.isfile(a.db):
        refuse("db_missing", a.db)
    pop = Pop(fj)
    paths_set = set(pop.paths)
    con = sqlite3.connect("file:%s?mode=ro" % a.db, uri=True)
    srcs = con.execute("SELECT source_id, kind, locator, scanned_entry_count FROM reg_sources ORDER BY source_id").fetchall()
    ctx = {}
    out_src, per_source_total = [], {}
    for sid, kind, loc, sc in srcs:
        rows = [r[0] for r in con.execute("SELECT locator FROM reg_source_entries WHERE source_id=?", (sid,))]
        db_by = {}
        for l in rows:
            k = entry_key(l, paths_set, loc)
            db_by[k] = db_by.get(k, 0) + 1
        rec = {"locator": loc, "kind": kind, "scanned_entry_count": sc, "db_entry_rows": len(rows)}
        meth = METHODS.get(loc)
        if meth is None:
            rec.update(status="no_recount_method", recount=None, method="none: this source is not a Stage 0 enumerator source", differences=[])
        else:
            by, exact, note = meth(pop, ctx)
            by = {k: v for k, v in by.items()}
            total = sum(by.values())
            diffs = []
            for k in sorted(set(by) | set(db_by), key=lambda x: x.encode("utf-8")):
                if by.get(k, 0) != db_by.get(k, 0):
                    diffs.append({"key": k, "register": db_by.get(k, 0), "recount": by.get(k, 0)})
            if sc != len(rows):
                diffs.append({"key": "<scanned_entry_count vs rows>", "register": sc, "recount": len(rows)})
            rec.update(recount=total, method=note, exact=exact, differences=diffs[:200], difference_count=len(diffs))
            rec["status"] = "match" if (not diffs and total == sc) else ("explained" if loc in explanations else "diff")
            if total != sc and not diffs:
                rec["status"] = "explained" if loc in explanations else "diff"
            if loc in explanations:
                rec["explanation"] = explanations[loc]
            per_source_total[loc] = total
        out_src.append(rec)
    doc03_path = a.doc03 or os.path.join(snapdir, "specs/001-full-project-audit-remediation/docs/03-existing-issue-inventory.md")
    d3 = doc03_compare(pop, doc03_path, per_source_total)
    failing = [s for s in out_src if s["status"] in ("diff", "no_recount_method")]
    rec = {"schema": "source-recount/1", "freeze": {"head": fj.get("head"), "listing_sha256": fj.get("listing_sha256"), "manifest_sha256": sha256_file(manifest),
                                                    "frozen_at": fj.get("frozen_at")},
           "db": os.path.basename(a.db), "listing_paths": len(pop.paths),
           "sources": out_src, "doc03_section7": d3,
           "summary": {"sources": len(out_src), "match": sum(1 for s in out_src if s["status"] == "match"),
                       "explained": sum(1 for s in out_src if s["status"] == "explained"), "diff": sum(1 for s in out_src if s["status"] == "diff"),
                       "no_recount_method": sum(1 for s in out_src if s["status"] == "no_recount_method"),
                       "approximate_methods": sorted(s["locator"] for s in out_src if s.get("exact") is False),
                       "doc03_deltas": sum(1 for d in d3 if d.get("status") == "delta")},
           "verdict": "FAIL" if failing else "PASS"}
    os.makedirs(a.out, exist_ok=True)
    with open(os.path.join(a.out, "recount.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(rec, f, indent=1, sort_keys=True)
        f.write("\n")
    for s in out_src:
        print("%-8s %-70s register=%s recount=%s%s" % (s["status"], s["locator"][:70], s["scanned_entry_count"], s.get("recount"),
                                                       "" if s["status"] == "match" else "  diffs=%s" % json.dumps(s.get("differences", [])[:5])))
    print("recount: %s sources=%d match=%d explained=%d diff=%d no_method=%d" % (rec["verdict"], rec["summary"]["sources"], rec["summary"]["match"],
                                                                                rec["summary"]["explained"], rec["summary"]["diff"], rec["summary"]["no_recount_method"]))
    return 1 if failing else 0


def main(argv):
    ap = argparse.ArgumentParser(prog="recount_sources.sh")
    ap.add_argument("--freeze-json", dest="freeze_json", required=True)
    ap.add_argument("--db", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--explanations")
    ap.add_argument("--doc03")
    try:
        a = ap.parse_args(argv)
    except SystemExit as e:
        return 2 if e.code else 0
    try:
        return run(a)
    except Refusal as r:
        sys.stderr.write("recount: REFUSED reason=%s %s\n" % (r.reason, r.detail))
        return 20


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
