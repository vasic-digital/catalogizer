#!/usr/bin/env python3
"""inventory_qa_classes.py - member inventory of the QA-instrument classifier over a ticket directory (review fix round 2, F4).

Runs the REAL classifier (import_tickets.qa_cause_of, the one T168 and T172 share) over every ticket of a docs/issues directory (default: the LIVE docs/issues; the
live tree is NOT the frozen snapshot, so every count is UNCONFIRMED for the T161 freeze) and prints, per status, the number of tickets, how many the classifier groups
(per cause), and the CANDIDATES it does not group (a terminal-status ticket whose resolution text mentions `infrastructure`, `false positive` or `screenshot`) by
resolution beginning, so a class the classifier misses is visible. Control needles (11.4.201(7)(b)): a planted ticket per class MUST classify, planted non-members MUST NOT.
Usage: inventory_qa_classes.py [ISSUES_DIR] [--expect-needles-only]       Exit 0; 1 when a needle fails (the instrument is blind or too broad).
"""
import collections, glob, importlib.util, os, re, sys, tempfile

REG = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ROOT = os.path.dirname(os.path.dirname(REG))


def load(name, fn):
    sp = importlib.util.spec_from_file_location(name, os.path.join(REG, fn))
    m = importlib.util.module_from_spec(sp)
    sp.loader.exec_module(m)
    return m


IT, EN = load("import_tickets", "import_tickets.py"), load("enumerate_sources", "enumerate_sources.py")


def classify(text):
    fm, body, _ = EN.parse_frontmatter(text)
    return (fm.get("status") or "").strip().lower(), IT.qa_cause_of(fm.get("status"), fm, EN_strip(body)), fm, body


def EN_strip(body):
    return IT.Importer.strip_heading(body)


NEEDLES = (   # (name, ticket text, expected cause)
    ("closed + fm QA infrastructure", "---\nstatus: closed\nresolution: QA infrastructure failure: screenshot timing.\n---\n# t\n", "screenshot_timing"),
    ("wontfix + fm QA infrastructure", "---\nstatus: wontfix\nresolution: QA infrastructure failure: wrong screen captured.\n---\n# t\n", "wrong_screen"),
    ("resolved + body Resolution FP statement", "---\nstatus: resolved\n---\n# t\n\n## Resolution\n\nFalse positive: screenshot captured during screen transition or app loading.\n", "screenshot_timing"),
    ("fixed + fm test infrastructure (other)", "---\nstatus: fixed\nresolution: test infrastructure was flaky.\n---\n# t\n", "other_qa_instrument"),
    ("open + QA infrastructure (NOT a member)", "---\nstatus: open\nresolution: QA infrastructure failure.\n---\n# t\n", None),
    ("closed, product reason (NOT a member)", "---\nstatus: closed\nresolution: Fixed in the product.\n---\n# t\n", None),
    ("fixed + QA phrase only in Notes (NOT a member)", "---\nstatus: fixed\n---\n# t\n\n## Notes\n\nQA infrastructure was suspected.\n\n## Resolution\n\nFixed.\n", None),
)


def main(a):
    needles_only = "--expect-needles-only" in a
    a = [x for x in a if not x.startswith("--")]
    bad = 0
    for name, text, want in NEEDLES:
        got = classify(text)[1]
        print("needle %-52s want=%-22s got=%-22s %s" % (name, want, got, "ok" if got == want else "FAIL"))
        bad += got != want
    if needles_only or bad:
        return 1 if bad else 0
    d = a[0] if a else os.path.join(ROOT, "docs/issues")
    files = sorted(glob.glob(os.path.join(d, "*.md")))
    by_status, grouped, cands = collections.Counter(), collections.Counter(), collections.Counter()
    sample = {}
    for p in files:
        st, cause, fm, body = classify(open(p, encoding="utf-8").read())
        by_status[st] += 1
        if cause:
            grouped[(st, cause)] += 1
        elif st in IT.QA_TERMINAL:
            res = IT.resolution_text(fm, EN_strip(body))
            if re.search(r"infrastructure|false positive|screenshot", res, re.I):
                k = re.sub(r"\s+", " ", res)[:70]
                cands[k] += 1
    print("\ntickets read: %d (%s)" % (len(files), d))
    print("by status: %s" % dict(sorted(by_status.items())))
    print("grouped (status, cause): count")
    for k, v in sorted(grouped.items()):
        print("  %-10s %-22s %d" % (k[0], k[1], v))
    print("grouped total: %d (terminal statuses %s)" % (sum(grouped.values()), IT.QA_TERMINAL))
    print("by status of the grouped: %s" % dict(sorted(collections.Counter(k[0] for k, v in grouped.elements()).items())) if False else
          "by status of the grouped: %s" % dict(sorted(collections.Counter(st for (st, _c), v in grouped.items() for _ in range(v)).items())))
    print("UNGROUPED candidates (terminal status, resolution mentions infrastructure / false positive / screenshot): %d distinct, %d tickets" % (len(cands), sum(cands.values())))
    for k, v in cands.most_common(15):
        print("  %4d  %s" % (v, k))
    return 0


sys.exit(main(sys.argv[1:]))
