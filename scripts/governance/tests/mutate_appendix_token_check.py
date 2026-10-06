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
    ("exit-always-0", "sys.exit(1 if (gaps or nodigest or stale or weak or lost or stale_floor or pgaps or stale_phr) else 0)", "sys.exit(0)"),
    ("no-digest-not-failing", "or nodigest or stale or weak", "or stale or weak"),
    ("stale-exemption-not-failing", "or nodigest or stale or weak", "or nodigest or weak"),
    ("allow-reason-optional", "or not parts[2].strip():", ":"),
    ("allow-exempts-everything-of-the-anchor", "if (k, t) in allow:\n                used.add((k, t))\n                exempt += 1\n            else:\n                gaps.append((k, t))",
     "if any(a == k for a, _ in allow):\n                used.add((k, t))\n                exempt += 1\n            else:\n                gaps.append((k, t))"),
    ("blind-canon-allowed", 'if not cb:\n        usage("BLIND', 'if False:\n        usage("BLIND'),
    ("blind-appendix-allowed", 'if not ab:\n        usage("BLIND', 'if False:\n        usage("BLIND'),
    ("duplicate-canon-allowed", "if cdups:", "if False:"),
    ("digest-token-set-ignored", "at = set(TOK.findall(joined(ab[k])))", "at = set(ct)"),
    # WF9 G3: the reviewer's code mutants T1-T9 that survived the first 15 tests (T7 and T9 were already killed), plus the wording floor / phrases
    ("T1-id-scope-narrowed-to-11.4", "ID = r'(\\d+(?:\\.[A-Za-z0-9]+)*)'", "ID = r'(11\\.4(?:\\.[A-Za-z0-9]+)*)'"),
    ("T2-dotted-subids-dropped", "ID = r'(\\d+(?:\\.[A-Za-z0-9]+)*)'", "ID = r'(\\d+(?:\\.\\d+)*)'"),
    ("T3-duplicate-appendix-digest-allowed", "if adups:", "if False:"),
    ("T4-tokens-checked-always-zero", "checked += len(ct)", "checked += 0"),
    ("T5-json-canon-sha-is-appendix-sha", '{"canon_sha256": hashlib.sha256(craw).hexdigest()', '{"canon_sha256": hashlib.sha256(araw).hexdigest()'),
    ("T6-canon-block-ends-only-at-h1", "HEAD = re.compile(r'^#{1,2} ')", "HEAD = re.compile(r'^# ')"),
    ("T7-gap-lines-not-printed", 'print("GAP §%s: digest lacks %s (present in canon block)" % (k, t))', "pass"),
    ("T8-stale-star-exemption-never-stale", "stale.append((k, t, why))", "stale.append((k, t, why)) if t != '*' else None"),
    ("floor-counts-not-compared", "if have[w] < cnt.get(w, 0):", "if False:"),
    ("floor-NEVER-not-a-counted-word", 'WORDS = ("MUST", "NEVER", "FORBIDDEN")', 'WORDS = ("MUST", "FORBIDDEN")'),
    ("compose-loss-not-recorded", "lost.append((k, r))", "pass"),
    ("stale-floor-row-not-recorded", "stale_floor.append(k)", "pass"),
    ("phrase-loss-not-recorded", "pgaps.append((k, phrase))", "pass"),
    ("stale-phrase-not-recorded", "stale_phr.append((k, phrase))", "pass"),
    ("phrase-reason-optional", "not parts[1].strip() or not parts[2].strip():", "not parts[1].strip():"),
    ("phrase-markup-not-ignored", 're.sub(r"[*`]", "", joined(text))', "joined(text)"),
    ("write-floor-omits-composes", '",".join(comp) if comp else "-"', '"-"'),
    ("exit-ignores-weakened", "or weak or lost", "or lost"),
    ("exit-ignores-phrase-loss", "or pgaps or stale_phr", "or stale_phr"),
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
        totals = [l for l in r.stdout.splitlines() if l.startswith("TOTAL")]
        print(("KILLED   " if killed else "SURVIVED ") + name + "  (" + (totals[-1] if totals else "no TOTAL line, test run ended rc=%d" % r.returncode) + ")")
        if not killed:
            bad.append(name)
print("MUTANTS %d SURVIVED_OR_FAILED %d" % (len(MUTANTS), len(bad)))
sys.exit(1 if bad else 0)
