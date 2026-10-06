#!/usr/bin/env python3
"""anchor-blocks.py - T077 independent anchor-block integrity and lockstep census (11.4.227(B), 11.4.157), read-only.
Usage: python3 -I anchor-blocks.py <extracted-constitution-tree>
For every numbered anchor `### §11.4.<N>` heading of Constitution.md: in each of CLAUDE/AGENTS/QWEN/GEMINI count the line-anchored
block-starts (the engine's three forms `**§`, `### §`, `- §`), report MISSING (0), DUPLICATED (>1), and DIVERGENT (the exactly-one
blocks, heading line through the line before the next block-start of ANY anchor, differ in sha256 across the mirrors that carry one).
A control needle (synthetic block-start must match, a mid-body citation must not) runs first; a blind instrument exits 2 (11.4.201(7)(b)).
"""
import hashlib, re, sys, os
T = sys.argv[1]
MIRRORS = ["CLAUDE.md", "AGENTS.md", "QWEN.md", "GEMINI.md"]
GENERIC = re.compile(r"^(\*\*|### |- )§11\.4\.[0-9]")
def start_re(n): return re.compile(r"^(\*\*|### |- )§11\.4\.%s(?![0-9])" % re.escape(n))
r = start_re("232")
if not (r.match("**§11.4.232 — control-needle synthetic block-start.**") and not r.match("This composes with §11.4.232(A) as a mid-sentence citation.")
        and not start_re("27").match("**§11.4.270 — x")):
    print("BLIND: control needle failed"); sys.exit(2)
canon = open(os.path.join(T, "Constitution.md"), encoding="utf-8").read().split("\n")
ids = []
for l in canon:
    m = re.match(r"^### §11\.4\.(\d+) ", l)
    if m: ids.append(m.group(1))
ids = sorted(set(ids), key=int)
text = {m: open(os.path.join(T, m), encoding="utf-8").read().split("\n") for m in MIRRORS}
miss = {m: [] for m in MIRRORS}; dup = {m: [] for m in MIRRORS}; diverg = []; ok = 0
for n in ids:
    rx = start_re(n); hs = {}
    for m in MIRRORS:
        lines = text[m]; starts = [i for i, l in enumerate(lines) if rx.match(l)]
        if not starts: miss[m].append(n); continue
        if len(starts) > 1: dup[m].append(n); continue
        i = starts[0]; j = next((k for k in range(i + 1, len(lines)) if GENERIC.match(lines[k])), len(lines))
        hs[m] = hashlib.sha256("\n".join(lines[i:j]).encode()).hexdigest()
    if len(set(hs.values())) > 1: diverg.append((n, sorted(set(hs.values()))[:0] or {m: h[:8] for m, h in hs.items()}))
    elif hs and not any(n in miss[m] or n in dup[m] for m in MIRRORS): ok += 1
print("tree=%s canon_numbered_anchors=%d" % (T, len(ids)))
for m in MIRRORS: print("  %-10s missing=%d duplicated=%d  missing_ids=%s dup_ids=%s" % (m, len(miss[m]), len(dup[m]), ",".join(miss[m][:12]) + ("..." if len(miss[m]) > 12 else ""), ",".join(dup[m][:12])))
print("  divergent anchors (blocks differ across the mirrors that carry exactly one): %d %s" % (len(diverg), ",".join(n for n, _ in diverg[:20])))
print("  fully lockstep (one block in all four, identical): %d of %d" % (ok, len(ids)))
