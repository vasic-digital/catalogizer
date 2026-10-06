#!/usr/bin/env python3
"""mutate_appendix_digests.py - data mutations of the REAL Spec Kit appendix, run through the real tool with its real allow list,
wording floor and required phrases (WF9 G3: the reviewer's data mutants D1-D12 plus the wording-floor and phrase mutants).

Each mutant edits ONE digest of a temporary copy of `.specify/memory/constitution-appendix.md` (the repository file is never written),
runs `appendix_token_check.py --canon <Constitution.md> --appendix <copy> --allow --floor --phrases`, and the tool MUST exit non-zero
(KILLED). The only expected survivor is D1 (one of two occurrences of a flag removed: an EQUIVALENT mutant, the digest still states the
flag; it must exit 0 and is reported as EQUIVALENT, not a failure). The control run on the unmutated appendix MUST exit 0 first: a
tool that always fails would otherwise "kill" everything. An injection that matches nothing is an error (the mutant would be the
unmodified file).

Run: scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- python3 -I scripts/governance/tests/mutate_appendix_digests.py
Exit 0: control green, every non-equivalent mutant killed, every equivalent mutant green. Exit 1 otherwise.
"""
import os, re, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
GOV = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(GOV))
TOOL = os.path.join(GOV, "appendix_token_check.py")
CANON = os.path.join(ROOT, "submodules", "constitution", "Constitution.md")
APPX = os.path.join(ROOT, ".specify", "memory", "constitution-appendix.md")
ALLOW = os.path.join(GOV, "appendix_token_allow.tsv")
FLOOR = os.path.join(GOV, "appendix_wording_floor.tsv")
PHR = os.path.join(GOV, "appendix_required_phrases.tsv")
src = open(APPX, encoding="utf-8").read()


def span(text, aid):
    m = re.search(r"^#### §" + re.escape(aid) + r" ", text, re.M)
    if not m:
        raise SystemExit("INJECT-FAIL: no digest for " + aid)
    n = re.search(r"^#{1,4} ", text[m.end():], re.M)
    return m.start(), (m.end() + n.start() if n else len(text))


def edit(aid, old, new, count=0):
    def f(text):
        a, b = span(text, aid)
        seg = text[a:b]
        if old not in seg:
            raise SystemExit("INJECT-FAIL: %r not in digest %s" % (old[:50], aid))
        return text[:a] + (seg.replace(old, new, count) if count else seg.replace(old, new)) + text[b:]
    return f


def dup_first(aid):
    def f(text):
        a, b = span(text, aid)
        seg = text[a:b]
        return text[:a] + seg.replace("MUST", "SHOULD") + "\n" + seg + text[b:]
    return f


def drop_lines(aid, prefix):
    def f(text):
        a, b = span(text, aid)
        lines = text[a:b].split("\n")
        keep = [l for l in lines if not l.startswith(prefix)]
        if len(keep) == len(lines):
            raise SystemExit("INJECT-FAIL: no line %r in %s" % (prefix, aid))
        return text[:a] + "\n".join(keep) + text[b:]
    return f


SONNET = "--leave-review-blocked-while-sonnet-reachable"
MUTANTS = [
    ("D1 one of two occurrences of the 209 Sonnet flag removed (EQUIVALENT)", edit("11.4.209", SONNET, "", 1), "equivalent"),
    ("D1b every occurrence of the 209 Sonnet flag removed", edit("11.4.209", SONNET, ""), "killed"),
    ("D2 276 flag --unbounded-review-rounds removed", edit("11.4.276", "--unbounded-review-rounds", ""), "killed"),
    ("D3 209 every MUST softened to SHOULD", edit("11.4.209", "MUST", "SHOULD"), "killed"),
    ("D3b 209 ONE MUST softened to SHOULD", edit("11.4.209", "MUST", "SHOULD", 1), "killed"),
    ("D4 211 Composes entry 11.4.209 deleted", edit("11.4.211", "- **Composes:** §11.4.209, ", "- **Composes:** "), "killed"),
    ("D5 211 flag --merge-conflict-any-model misspelled", edit("11.4.211", "--merge-conflict-any-model", "--merge-conflict-any-modelx"), "killed"),
    ("D6 209 'no Fable' replaced by 'Fable as escalation tier'", edit("11.4.209", "no Fable", "Fable as escalation tier"), "killed"),
    ("D7 9.2 digest loses --force-with-lease", edit("9.2", "--force-with-lease", ""), "killed"),
    ("D8 276 gate CM-FIX-GROUND-TRUTH-AND-CLASS-INVENTORY removed", edit("11.4.276", "CM-FIX-GROUND-TRUTH-AND-CLASS-INVENTORY", ""), "killed"),
    ("D9 235 gate CM-VERSION-INCREMENT-ON-DEPLOY removed", edit("11.4.235", "CM-VERSION-INCREMENT-ON-DEPLOY", ""), "killed"),
    ("D10 weakened duplicate 230 digest inserted before the real one", dup_first("11.4.230"), "killed"),
    ("D11 230 heading typo (§11.4.23O)", lambda t: t.replace("#### §11.4.230 —", "#### §11.4.23O —", 1), "killed"),
    ("D12 231 whole Gates line deleted", drop_lines("11.4.231", "- **Gates:**"), "killed"),
    ("D13 230 Composes entry §11.4.24 deleted", edit("11.4.230", "§11.4.24, ", ""), "killed"),
    ("D14 230 (D.2) MUST softened", edit("11.4.230", "MUST leave the deliverable correct on every path a user or QA tester can reach", "should leave the deliverable correct on every path a user or QA tester can reach"), "killed"),
    ("D15 209 last sentence of the quoted operator mandate dropped", edit("11.4.209", " Work MUST BE performed with all means we have at particular moment!", ""), "killed"),
    ("D16 209 'NEVER left undone / blocked / deferred' cut to 'NEVER left undone'", edit("11.4.209", "NEVER left undone / blocked / deferred", "NEVER left undone"), "killed"),
    ("D17 230 'four invariants' back to 'three invariants'", edit("11.4.230", "adds the four invariants they did not each state on their own", "adds three invariants"), "killed"),
]


def run_tool(text, tmp):
    p = os.path.join(tmp, "appendix.md")
    open(p, "w", encoding="utf-8").write(text)
    r = subprocess.run([sys.executable, "-I", TOOL, "--canon", CANON, "--appendix", p, "--allow", ALLOW, "--floor", FLOOR, "--phrases", PHR],
                       capture_output=True, text=True, timeout=300)
    return r.returncode, (r.stdout + r.stderr).strip().splitlines()


bad = []
with tempfile.TemporaryDirectory() as tmp:
    rc, out = run_tool(src, tmp)
    print("CONTROL unmutated appendix rc=%d  %s" % (rc, out[0] if out else ""))
    if rc != 0:
        print("CONTROL FAILED: the tool refuses the unmutated appendix; no mutant result means anything")
        sys.exit(1)
    for name, fn, expect in MUTANTS:
        new = fn(src)
        if new == src:
            print("INJECT-FAIL %s: no change" % name)
            bad.append(name)
            continue
        rc, out = run_tool(new, tmp)
        first = [l for l in out if l.split(" ")[0] in ("GAP", "WEAKENED", "COMPOSE-LOST", "PHRASE-LOST", "NO-DIGEST", "STALE") or "duplicate" in l][:1]
        if expect == "killed":
            ok = rc != 0
            print(("KILLED    " if ok else "SURVIVED  ") + name + "  rc=%d %s" % (rc, first[0][:110] if first else (out[0][:110] if out else "")))
        else:
            ok = rc == 0
            print(("EQUIVALENT " if ok else "NOT-EQUIVALENT ") + name + "  rc=%d" % rc)
        if not ok:
            bad.append(name)
print("DATA-MUTANTS %d BAD %d" % (len(MUTANTS), len(bad)))
sys.exit(1 if bad else 0)
