#!/usr/bin/env python3
"""wp20_fixture.py - builds a small scratch tree with ONE or more real entries of each of the 25 source
classes S-01..S-25 (doc03 section 4), plants one extra entry of a named class, and builds the scratch
freeze.json + manifest for it (WP-20, T162). Pure fixture code: it writes only below the directory it is given.

  wp20_fixture.py build <dir>                  base tree
  wp20_fixture.py plant <dir> <S-nn|3way>      plant ONE entry of the class (or the 3-way T162 scenario)
  wp20_fixture.py decoy <dir>                  add carriers that must NOT produce a row
  wp20_fixture.py freeze <dir> <freeze.json>   write <dir>.manifest.json next to freeze.json + freeze.json
"""
import json, os, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)


def w(root, rel, text, mode="w"):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p) or root, exist_ok=True)
    with open(p, mode, encoding="utf-8", newline="\n") as f:
        f.write(text)


def ticket(i, status="fixed"):
    return "---\nid: HELIX-%03d\nseverity: high\ncategory: functional\nstatus: %s\nfound_date: 2026-03-29\nresolution: a: b, with a colon\n---\n\n# Ticket %d\n\nbody\n" % (i, status, i)


CASE_YAML = "version: 1\nname: bank\ntest_cases:\n  - id: case-one\n    name: One\n  - id: case-two\n    name: Two\n"


# (label, front-matter text, the mapping a YAML-aware reader returns; the colon-in-value form is what a strict YAML reader refuses, OD-17)
FRONTMATTER_FORMS = [("key space colon", "---\nid: H-1\nstatus : fixed\nseverity: high\n---\n# T\n", {"id": "H-1", "status": "fixed", "severity": "high"}),
("trailing comment", "---\nid: H-2\nstatus: fixed # closed by run 5\n---\n# T\n", {"id": "H-2", "status": "fixed"}),
("folded block scalar", "---\nid: H-3\nseverity: >\n  high\n  and more\nstatus: open\n---\n# T\n", {"id": "H-3", "severity": "high and more", "status": "open"}),
("literal block scalar", "---\nid: H-4\nresolution: |\n  line one\n  line two\nstatus: closed\n---\n# T\n", {"id": "H-4", "resolution": "line one\nline two", "status": "closed"}),
("double quoted", '---\nid: H-5\nstatus: "fixed"\n---\n# T\n', {"id": "H-5", "status": "fixed"}),
("single quoted with escaped quote", "---\nid: H-6\ntitle: 'it''s'\n---\n# T\n", {"id": "H-6", "title": "it's"}),
("colon inside a value (YAML refuses this)", "---\nid: H-7\nresolution: a: b, with a colon\nstatus: fixed\n---\n# T\n", {"id": "H-7", "resolution": "a: b, with a colon", "status": "fixed"}),
("continuation line", "---\nid: H-8\nresolution: first\n  second\nstatus: fixed\n---\n# T\n", {"id": "H-8", "resolution": "first second", "status": "fixed"}),
("CRLF", "---\r\nid: H-9\r\nstatus: fixed\r\n---\r\n# T\r\n", {"id": "H-9", "status": "fixed"}),
("hash inside a quoted value", '---\nid: H-10\nstatus: "fixed # not a comment"\n---\n# T\n', {"id": "H-10", "status": "fixed # not a comment"})]


def build(root):
    w(root, "docs/issues/HELIX-001-a.md", ticket(1))
    w(root, "docs/issues/HELIX-001-b.md", ticket(1, "open"))
    w(root, "issues/ANR-2026-04-08-X.md", "# ANR\n\nID: ANR-2026-04-08-001\nStatus: RESOLVED\n")
    w(root, "TASK_TRACKER.md", "# T\n\n**Status:**\n- \u2b1c **Not Started**\n- \U0001f7e8 **In Progress**\n- \u2705 **Complete**\n- \u23f8\ufe0f **Blocked**\n\n| ID | Task | Pri | Status |\n|---|---|---|---|\n| 0.1 | first | high | \u2b1c |\n| 0.2 | second | high | \u2705 |\n| 0.3 | third | low | \U0001f7e8 |\n| 0.4 | fourth | low | \u23f8\ufe0f |\n| 0.5 | fifth | low | Blocked |\n")
    w(root, "MASTER_EXECUTION_CHECKLIST.md", "# C\n\n- [ ] one\n- [ ] two\n")
    w(root, "docs/MASTER_EXECUTION_CHECKLIST.md", "# C\n\n- [ ] one\n")
    w(root, "COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md", "# P\n\n- [ ] one\n")
    w(root, "COMPREHENSIVE_UNFINISHED_WORK_REPORT.md", "# U\n\n- [ ] one\n❌ bad thing\n")
    w(root, "docs/UNFINISHED_WORK_COMPREHENSIVE_REPORT.md", "# S\n\n❌ one\n⚠ two\n")
    for n in ("UNFINISHED_WORK_ANALYSIS.md", "UNFINISHED_WORK_AND_ISSUES.md", "FINAL_UNFINISHED_WORK_REPORT.md"):
        w(root, n, "# R\n\n❌ one\n")
    w(root, "docs/OPEN_POINTS_CLOSURE.md", "# O\n\n- [ ] open\n- [x] done\n")
    w(root, "docs/LANDMINES.md", "# L\n\n| RULE-API-001 | a |\n| RULE-WEB-002 | b |\n")
    for n in ("AGENTS.md", "CLAUDE.md", "CONSTITUTION.md", "GEMINI.md", "GETTING_STARTED.md", "MEMORY.md", "QUICK_REFERENCE.md", "README.md"):
        w(root, n, "# governance %s\n" % n)
    w(root, "ALL_ISSUES_FIXED.md", "# Report\n\n- [ ] leftover\n")
    w(root, "docs/status/s1.md", "# Status 1\n")
    w(root, "docs/audits/a1.md", "# Audit\n\nCATAPI-DEFECT-001 broken\n")
    w(root, "docs/FRONTEND_AUDIT.md", "# FA\n")
    w(root, "docs/qa/q1.md", "# QA\n\nFIX-QA-2026-04-21-001 fixed\n")
    w(root, "docs/reports/qa-sessions/s1/notes.md", "# Session\n")
    w(root, "docs/security/gosec-20260101_000000.json", json.dumps({"Issues": [{"rule_id": "G101", "file": "a.go", "line": "3"}]}))
    w(root, "docs/security/gosec-20260201_000000.json", json.dumps({"Issues": [{"rule_id": "G102", "file": "b.go", "line": "4"}, {"rule_id": "G103", "file": "b.go", "line": "9"}]}))
    w(root, "docs/security/snyk-go-20260201_000000.json", json.dumps({"ok": False, "error": "not authenticated"}))
    w(root, "docs/security/note.md", "# Note\n")
    w(root, "docs/security/firebase-api-key-exposure-20260629.md", "# Firebase\n")
    w(root, "SECURITY_AUDIT_REPORT.md", "# SAR\n")
    w(root, "SECURITY_KEY_ROTATION_REQUIRED.md", "# Rot\n\n## Affected Providers\n\n| Provider | Variable | URL |\n|---|---|---|\n| Astica | `ASTICA_API_KEY` | https://a.example |\n| Cerebras | `CEREBRAS_API_KEY` | https://c.example |\n")
    w(root, "challenges/helixqa-banks/b1.yaml", CASE_YAML)
    w(root, "challenges/data/challenges_bank.json", json.dumps({"version": "1", "categories": {"x": "desc"}, "challenges": [{"id": "c1", "name": "C1"}, {"id": "c2", "name": "C2"}]}))
    w(root, "submodules/helix_qa/banks/hq1.yaml", CASE_YAML)
    w(root, "submodules/helix_qa/challenges/baselines/bluff-baseline.txt", "# comment\npkg/a.go:BLUFF-1\npkg/b.go:0.5:10\n")
    w(root, "submodules/helix_qa/docs/behavior-anchors.md", "# A\n\n| ID | x |\n|---|---|\n| CAP-001 | one |\n| CAP-002 | two |\n")
    w(root, ".specify/memory/constitution.md", "# C\n\ntext\n\n## Known Conflicts and Open Decisions\n\nPreface: 3 DECIDED (operator), 4 NOTE.\n\n1. **One.** DECIDED here.\n2. **Two.** OPEN here.\n   - sub a\n3. **Three.** No status word on this line.\n4. **Four.**\n   Second line without a word.\n5. **Five.** First part.\n    **FIXED:** the first part is done.\n    **OPEN:** the second part is not.\n")
    w(root, "catalog-web/CLAUDE.md", "# module\n")
    w(root, ".implementation/r1.md", "# Impl\n")
    w(root, "catalog-api/main.go", "package main\n\n// HACK base marker\nfunc main() {}\n")
    w(root, "docs/COMPREHENSIVE_PACKAGE_SUMMARY.md", "# Pkg\n\n- [ ] pkg one\n- [x] pkg two\n")
    w(root, "docs/README_IMPLEMENTATION_PACKAGE.md", "# Impl\n\n- [ ] impl one\n")
    w(root, "docs/MASTER_AUDIT_AND_IMPLEMENTATION_PLAN.md", "# MA\n\n- [ ] audit one\n- [ ] audit two\n")
    w(root, "docs/procedures/howto.md", "# How\n\n- [ ] step one\n- [ ] step two\n- [x] step three\n")
    w(root, "docs/OTHER_REF.md", "# Other\n\nDEFER-001 and FINDING-7 are named here only; FIX-OC2-001 too.\n")
    w(root, ".firebaserc", "{}\n")
    w(root, "sonar-project.properties", "sonar.projectKey=x\n")


def tickets(root):
    """import fixtures: one ticket longer than 2048 bytes (two-byte characters, so the cap can fall inside one) and one short one"""
    long_body = ("\u00e9" * 1500) + "\n"
    w(root, "docs/issues/HELIX-950-long.md", "---\nid: HELIX-950\nseverity: high\nstatus: fixed\nfound_date: 2026-03-29\n---\n\n# Long ticket\n\n" + long_body)
    w(root, "docs/issues/HELIX-951-short.md", "---\nid: HELIX-951\nseverity: low\nstatus: wontfix\nfound_date: 2026-03-29\n---\n\n# Short ticket\n\nShort body text that stays whole.\n")


def plant(root, cls):
    ap = lambda rel, text: w(root, rel, text, "a")
    if cls == "S-01": w(root, "docs/issues/HELIX-900-planted.md", ticket(900))
    elif cls == "S-02": w(root, "issues/ANR-2026-01-01-planted.md", "# P\n\nANR-2026-01-01-009\n")
    elif cls == "S-03": ap("TASK_TRACKER.md", "| 9.9 | planted | Blocked |\n")
    elif cls == "S-04": ap("MASTER_EXECUTION_CHECKLIST.md", "- [ ] planted\n")
    elif cls == "S-05": ap("docs/MASTER_EXECUTION_CHECKLIST.md", "- [ ] planted\n")
    elif cls == "S-06": ap("COMPREHENSIVE_PROJECT_STATUS_AND_PLAN.md", "- [ ] planted\n")
    elif cls == "S-07": ap("COMPREHENSIVE_UNFINISHED_WORK_REPORT.md", "- [ ] planted\n")
    elif cls == "S-08": ap("docs/UNFINISHED_WORK_COMPREHENSIVE_REPORT.md", "❌ planted\n")
    elif cls == "S-09": ap("FINAL_UNFINISHED_WORK_REPORT.md", "❌ planted\n")
    elif cls == "S-10": ap("docs/OPEN_POINTS_CLOSURE.md", "- [x] planted\n")
    elif cls == "S-11": ap("docs/LANDMINES.md", "| RULE-X-999 | planted |\n")
    elif cls == "S-12": w(root, "PLANTED_REPORT.md", "# Planted\n")
    elif cls == "S-13": w(root, "docs/status/planted.md", "# P\n")
    elif cls == "S-14": ap("docs/audits/a1.md", "CATAPI-DEFECT-900 planted\n")
    elif cls == "S-15": ap("docs/qa/q1.md", "FIX-QA-2026-01-01-900 planted\n")
    elif cls == "S-16": w(root, "docs/security/planted.md", "# P\n")
    elif cls == "S-17": ap("challenges/helixqa-banks/b1.yaml", "  - id: planted-case\n    name: P\n")
    elif cls == "S-18":
        p = os.path.join(root, "challenges/data/challenges_bank.json"); d = json.load(open(p))
        d["challenges"].append({"id": "planted", "name": "P"}); json.dump(d, open(p, "w"))
    elif cls == "S-19": ap("submodules/helix_qa/banks/hq1.yaml", "  - id: planted-case\n    name: P\n")
    elif cls == "S-20": ap("submodules/helix_qa/docs/behavior-anchors.md", "| CAP-999 | planted |\n")
    elif cls == "S-21": ap(".specify/memory/constitution.md", "3. **Planted.** OPEN here.\n")
    elif cls == "S-22": w(root, "installer-wizard/AGENTS.md", "# module\n")
    elif cls == "S-23": ap("catalog-api/main.go", "// TODO planted\n")
    elif cls == "S-24": w(root, ".implementation/planted.md", "# P\n")
    elif cls == "S-25": ap("SECURITY_KEY_ROTATION_REQUIRED.md", "| Planted | `PLANTED_API_KEY` | https://p.example |\n")
    elif cls == "XID": ap("docs/OTHER_REF.md", "A new prose-only id: DEFER-777.\n")
    elif cls == "3wayreal":   # the same scenario on a real snapshot: the bank case goes into the existing JSON bank
        ap("MASTER_EXECUTION_CHECKLIST.md", "- [ ] planted checkbox\n")
        w(root, "docs/issues/HELIX-901-planted.md", ticket(901))
        plant(root, "S-18")
    elif cls == "3way":   # T162: one checkbox line, one docs/issues ticket, one bank case
        ap("MASTER_EXECUTION_CHECKLIST.md", "- [ ] planted checkbox\n")
        w(root, "docs/issues/HELIX-901-planted.md", ticket(901))
        ap("challenges/helixqa-banks/b1.yaml", "  - id: planted-case\n    name: P\n")
    else:
        sys.exit("unknown class " + cls)


def decoy(root):
    """carriers: none of these may produce a row (control needle of the negative side)."""
    w(root, "docs/issues/notes.txt", "not a ticket\n")
    w(root, "docs/status/notes.txt", "not markdown\n")
    w(root, "MASTER_EXECUTION_CHECKLIST.md", "# C\n\n- [ ] one\n- [ ] two\n\n[ ] not a list item\n- [?] odd box\n", "w")
    w(root, "catalog-api/other.go", "package main\n// TODOS and XXXL are other words\nfunc f() { s.Skip(1) }\n")
    w(root, "desktop/main.rs", "fn main() { std::process::exit(1) }\n")


# the credential-looking userinfo of the second remote is a decoy: no output may carry it (11.4.10)
REMOTES = [{"name": "origin", "url": "git@example.invalid:owner/repo.git"},
           {"name": "github", "url": "https://fixtureuser:FIXTURETOKEN123@github.example.invalid/owner/repo.git"}]


def freeze(root, fj):
    out = os.path.abspath(fj)
    mf = out + ".manifest.json"
    r = subprocess.run([sys.executable, os.path.join(REG, "snapshot_manifest.py"), root], stdout=open(mf, "w"))
    if r.returncode != 0:
        sys.exit("snapshot_manifest failed")
    json.dump({"snapshot": os.path.abspath(root), "manifest": mf, "frozen_at": "2026-10-07T00:00:00Z", "head": "scratch", "remotes": REMOTES}, open(out, "w"))


if __name__ == "__main__":
    a = sys.argv[1:]
    if a[:1] == ["build"]: build(a[1])
    elif a[:1] == ["plant"]: plant(a[1], a[2])
    elif a[:1] == ["tickets"]: tickets(a[1])
    elif a[:1] == ["decoy"]: decoy(a[1])
    elif a[:1] == ["freeze"]: freeze(a[1], a[2])
    else: sys.exit(__doc__)
