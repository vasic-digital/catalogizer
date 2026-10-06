#!/usr/bin/env python3
"""mutate_regen_speckit_catalogue.py - T076 paired mutations for scripts/governance/regen_speckit_catalogue.py (constitution 1.1).

Each mutation is a COPY of the script with one deliberate defect; the T075 test is run against the copy (inside IMG-TESTUTIL through
scripts/containers/run_pinned.sh, `--script` argument) and MUST FAIL. A mutation that leaves the test green is a surviving mutant and
fails this runner. The first mutation is the one T076 names: a drift check that ignores edits (always reports in sync) must make the
hand-edited-catalogue case FAIL; the runner checks that exact case by name.

Usage (host, from the repository root): python3 scripts/governance/tests/mutate_regen_speckit_catalogue.py [--only <name>]
Copies live in .audit/scratch/wp07/mut/ (ignored), which the container sees under /src. Exit 0: every mutant killed; 1: a survivor or a
mutation that did not apply; 2: usage / environment error.
Golden-good control: the unmutated script is run first and MUST pass all cases (a runner that fails everything proves nothing).
"""
import os
import re
import subprocess
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
SRC = os.path.join(ROOT, "scripts", "governance", "regen_speckit_catalogue.py")
TEST = "scripts/governance/tests/test_regen_speckit_catalogue.py"
MUT_DIR = os.path.join(ROOT, ".audit", "scratch", "wp07", "mut")
RUNP = os.path.join(ROOT, "scripts", "containers", "run_pinned.sh")

# name, old text (must occur exactly once), new text, case names that MUST fail (empty = any failure)
MUTATIONS = [
    ("drift-check-ignores-edits", "if same:  # MUT:drift-check", "if True:  # MUT:drift-check",
     ["hand-edited-catalogue-drift", "check-fresh-input-reports-drift", "hand-edited-pin-line-drift", "hand-edited-appendix-drift"]),
    ("drift-exit-code-zero", "return 1 if drift else 0", "return 0", ["hand-edited-catalogue-drift"]),
    ("title-limit-300", "TITLE_LIMIT = 200", "TITLE_LIMIT = 300", ["write-reproduces-golden-constitution"]),
    ("keeps-leading-em-dash", 'if t.startswith("— "):', "if False:", ["write-reproduces-golden-constitution"]),
    ("no-trailing-whitespace-strip", "lines = [l.rstrip() for l in text.split(\"\\n\")]", "lines = [l for l in text.split(\"\\n\")]",
     ["trailing-whitespace-and-single-final-newline"]),
    ("no-final-newline-normalisation", "    while lines and lines[-1] == \"\":\n        lines.pop()\n    return \"\\n\".join(lines) + \"\\n\"",
     "    return \"\\n\".join(lines) + \"\\n\"", ["trailing-whitespace-and-single-final-newline"]),
    ("lag-never-reported-in-constitution", "    if index_sha == canon_sha:\n        return head", "    if True:\n        return head",
     ["lagging-index-golden-constitution"]),
    ("lag-never-reported-in-appendix", "    if index_sha != canon_sha:\n        row +=", "    if False:\n        row +=",
     ["lagging-index-golden-appendix"]),
    ("lag-ids-dropped", "if index_sha != canon_sha else []", "if False else []", ["lagging-index-golden-constitution"]),
    ("appendix-pin-not-updated", "lines[idx[0]] = pin_row(commit, canon_sha, index_sha)", "pass", ["write-reproduces-golden-appendix"]),
    ("range-run-threshold", ">= 3:", ">= 5:", ["write-reproduces-golden-constitution"]),
    ("acronyms-not-uppercased", 'GROUP_ACRONYMS = {"ui": "UI", "tdd": "TDD"}', "GROUP_ACRONYMS = {}",
     ["write-reproduces-golden-constitution"]),
    ("section-heading-counted-as-anchor", "(\\d+(?:\\.\\d+)+(?:", "(\\d+(?:\\.\\d+)*(?:", ["lagging-index-golden-constitution"]),
    ("missing-marker-tolerated", "raise InputError(\"constitution file has no '## Anchor Catalogue' heading\")",
     "cstart = len(lines)", ["missing-catalogue-marker-exit-2-untouched"]),
    # WF8 F4 (2026-10-06): the four reviewer-authored mutants that survived the first 14 are now named and killed
    ("notes-silenced", "    for line in notes(old[\"constitution\"], len(index[\"anchors\"])):\n        print(line)",
     "    for line in notes(old[\"constitution\"], len(index[\"anchors\"])):\n        pass", ["stale-anchor-count-prints-NOTE-line"]),
    ("lag-ids-reversed", "key=key)", "key=key, reverse=True)", ["lag-ids-numeric-order-in-stdout-and-both-files"]),
    ("lag-ids-string-sorted", "return sorted(heads - set(index_ids), key=key)", "return sorted(heads - set(index_ids))",
     ["lag-ids-numeric-order-in-stdout-and-both-files"]),
    ("index-lag-banner-silenced", "        print(\"INDEX-LAG index source_sha256 %s != canon sha256 %s; anchors in canon but not in the index: %s\"\n              % (index_sha, canon_sha, \", \".join(lag_ids) if lag_ids else \"none\"))",
     "        pass", ["lagging-index-stdout-banner"]),
]


def run_test(script_container_path):
    cmd = [RUNP, "IMG-TESTUTIL", "--", "python3", TEST, "--script", script_container_path]
    p = subprocess.run(cmd, capture_output=True, text=True, timeout=600, cwd=ROOT)
    return p.returncode, p.stdout + p.stderr


def main(argv):
    only = argv[1] if len(argv) == 2 and argv[0] == "--only" else None
    if argv and only is None:
        print(__doc__)
        return 2
    os.makedirs(MUT_DIR, exist_ok=True)
    with open(SRC, encoding="utf-8") as f:
        text = f.read()
    print("# golden-good control: unmutated script")
    rc, out = run_test("/src/scripts/governance/regen_speckit_catalogue.py")
    total = re.findall(r"^TOTAL .*$", out, re.M)
    print("  control rc=%d %s" % (rc, total[-1] if total else "no TOTAL line"))
    if rc != 0:
        print("CONTROL FAILED: the test does not pass on the unmutated script; mutation results would prove nothing")
        return 1
    survivors, killed, bad = [], 0, []
    for name, old, new, must_fail in MUTATIONS:
        if only and name != only:
            continue
        if text.count(old) != 1:
            print("MUTATION-NOT-APPLIED %s (old text occurs %d times)" % (name, text.count(old)))
            bad.append(name)
            continue
        path = os.path.join(MUT_DIR, name + ".py")
        with open(path, "w", encoding="utf-8") as f:
            f.write(text.replace(old, new))
        rc, out = run_test("/src/.audit/scratch/wp07/mut/" + name + ".py")
        failed = re.findall(r"^FAIL (\S+)", out, re.M)
        tot = re.findall(r"^TOTAL .*$", out, re.M)
        missing = [c for c in must_fail if c not in failed]
        if rc != 0 and not missing:
            killed += 1
            print("KILLED   %-38s rc=%d failing=%d  %s  (named cases failed: %s)" % (name, rc, len(failed), tot[-1] if tot else "", ",".join(must_fail)))
        elif rc != 0:
            survivors.append(name)
            print("WEAK     %-38s rc=%d but the named case(s) %s did not fail; failing=%s" % (name, rc, missing, failed))
        else:
            survivors.append(name)
            print("SURVIVED %-38s test stayed green (%s)" % (name, tot[-1] if tot else ""))
    print("MUTATIONS killed=%d survivors=%d not_applied=%d (control rc=0)" % (killed, len(survivors), len(bad)))
    return 0 if not survivors and not bad else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
