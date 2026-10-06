#!/usr/bin/env python3
"""mutate_appendix_token_check.py - paired mutations of scripts/governance/appendix_token_check.py (WF8 F3, mutation adequacy).

Each mutant is the tool with ONE behavioural defect injected by an exact string replacement; test_appendix_token_check.py is run against
it with --script and MUST fail. A mutant that survives (tests still 0) is a test gap. A replacement that matches nothing is itself an
error (the mutant would be the unmodified tool). Run: python3 -I scripts/governance/tests/mutate_appendix_token_check.py
Exit 0 all killed, 1 a mutant survived or an injection failed.
"""
import os, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
TOOL = os.path.join(os.path.dirname(HERE), "appendix_token_check.py")
TEST = os.path.join(HERE, "test_appendix_token_check.py")
src = open(TOOL, encoding="utf-8").read()
MUTANTS = [
    ("flags-not-tokenised", "|(?<![\\w-])--[a-z][a-z0-9-]*[a-z0-9]')", "')"),
    ("gate-names-not-tokenised", "TOK = re.compile(r'CM-[A-Z0-9][A-Z0-9-]*[A-Z0-9]|", "TOK = re.compile(r'(?!)|"),
    ("hyphen-wrap-not-joined", "return WRAP.sub(r'\\1', text)", "return text"),
    ("bold-block-start-ignored", "|\\*\\*§' + ID + r'\\s+—)')", "|(?!))')"),
    ("bare-prose-citation-is-block-start", "(?:#{3,4} +§' + ID + r'(?=\\s|—)|", "(?:#{3,4} +§|§)' + ID + r'(?=\\s|—)|"),
    ("sub-clause-continuation-off", "and not (cont_re and cont_re.match(line))", ""),
    ("exit-always-0", "sys.exit(1 if (gaps or nodigest or stale) else 0)", "sys.exit(0)"),
    ("no-digest-not-failing", "sys.exit(1 if (gaps or nodigest or stale) else 0)", "sys.exit(1 if (gaps or stale) else 0)"),
    ("stale-exemption-not-failing", "sys.exit(1 if (gaps or nodigest or stale) else 0)", "sys.exit(1 if (gaps or nodigest) else 0)"),
    ("allow-reason-optional", "or not parts[2].strip():", ":"),
    ("allow-exempts-everything-of-the-anchor", "if (k, t) in allow:\n                used.add((k, t))\n                exempt += 1\n            else:\n                gaps.append((k, t))",
     "if any(a == k for a, _ in allow):\n                used.add((k, t))\n                exempt += 1\n            else:\n                gaps.append((k, t))"),
    ("blind-canon-allowed", 'if not cb:\n        usage("BLIND', 'if False:\n        usage("BLIND'),
    ("blind-appendix-allowed", 'if not ab:\n        usage("BLIND', 'if False:\n        usage("BLIND'),
    ("duplicate-canon-allowed", "if cdups:", "if False:"),
    ("digest-token-set-ignored", "at = set(TOK.findall(joined(ab[k])))", "at = set(ct)"),
]
bad = []
with tempfile.TemporaryDirectory() as tmp:
    for name, old, new in MUTANTS:
        if src.count(old) < 1:
            print("INJECT-FAIL %s: pattern not found" % name)
            bad.append(name)
            continue
        p = os.path.join(tmp, name + ".py")
        open(p, "w", encoding="utf-8").write(src.replace(old, new, 1))
        r = subprocess.run([sys.executable, "-I", TEST, "--script", p], capture_output=True, text=True, timeout=300)
        killed = r.returncode != 0
        print(("KILLED   " if killed else "SURVIVED ") + name + "  (" + [l for l in r.stdout.splitlines() if l.startswith("TOTAL")][-1:][0] + ")" if r.stdout.strip() else name)
        if not killed:
            bad.append(name)
print("MUTANTS %d SURVIVED_OR_FAILED %d" % (len(MUTANTS), len(bad)))
sys.exit(1 if bad else 0)
