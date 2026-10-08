#!/usr/bin/env python3
"""check_task_citations.py - T283a. Task-id citation check for the plan set (docs/21 section 12.7).

Why: task ids cited outside tasks.md usually still EXIST after a renumbering, they just name another task, so a check that only asks
"does the id exist" stays green on stale citations. A tracked citation list binds each cited id, at its line, to a keyword that the
cited task's text must still contain.

Usage:
  check_task_citations.py [--feature-dir DIR] [--tasks FILE] [--list FILE] [--extract] [--self-test-blind]

  --feature-dir DIR  the feature folder ($FEAT); default: specs/001-full-project-audit-remediation next to the repository root
  --tasks FILE       tasks.md (default DIR/tasks.md)
  --list FILE        citation list (default DIR/task_citations.tsv)
  --extract          print one candidate row per (file, line, id) of a FRESH extraction with a PROPOSED keyword (seeding aid; a person reviews
                     every row, nothing is written); exit 0
  --self-test-blind  TEST ONLY: makes the engine omit the ORPHAN-ROW and UNBOUND classes, so the seeded control cannot be reported (exit 20)

Plan set: every Markdown file under DIR except tasks.md and except the top-level folders evidence/ and audit/ (run records, not plan
documents), plus the contract schemas DIR/contracts/*.json.
Token: T + three digits + optional lower-case letter, no letter or digit directly before or after. A token written as an end of a range
("T001 to T595", "T001-T595", "T001..T595", "T001 through T595") is a range-form occurrence.
Citation list (TAB separated, '#' lines ignored): file, prefix, id, kind, keyword. `prefix` begins exactly one line of `file`; kind is one of
citation | range_bound | format_example. A row can silence only UNBOUND: a DANGLING id is judged against tasks.md and a STALE row against the
cited task's text.
Findings (printed `FINDING <CLASS> <file>:<line>: <detail>`): DANGLING, UNBOUND, STALE, ORPHAN-ROW, EXAMPLE-RESOLVES.
Seeded control, every run: in memory, one bound citation is replaced by the id of the task two places before it; the engine must report
that line. A run whose control is not reported is BLIND and exits 20 (its silence says nothing).
Exit: 0 no finding, 1 at least one finding, 2 usage error or unreadable input, 20 blind.
"""
import os
import re
import sys

KINDS = ("citation", "range_bound", "format_example")
TOK = re.compile(r"(?<![A-Za-z0-9])T\d{3}[a-z]?(?![A-Za-z0-9])")
RANGE = re.compile(r"(?<![A-Za-z0-9])(T\d{3}[a-z]?)(\s*(?:to|through|-|–|\.\.)\s*)(T\d{3}[a-z]?)(?![A-Za-z0-9])")
TASKLINE = re.compile(r"^- \[[ xX]\] (T\d{3}[a-z]?)\b")
SKIP_TOP = ("evidence", "audit")


class InputError(Exception):
    pass


def read_text(path):
    try:
        with open(path, encoding="utf-8") as fh:
            return fh.read()
    except (OSError, UnicodeDecodeError) as exc:
        raise InputError("cannot read %s: %s" % (path, exc))


def plan_set(fdir):
    out = []
    for dp, dn, fn in os.walk(fdir):
        rel = os.path.relpath(dp, fdir)
        top = rel.split(os.sep)[0]
        if rel != "." and top in SKIP_TOP:
            dn[:] = []
            continue
        if rel == ".":
            dn[:] = sorted(dn)
        for f in sorted(fn):
            p = f if rel == "." else os.path.join(rel, f)
            if p == "tasks.md":
                continue
            if f.endswith(".md") or (top == "contracts" and rel != "." and f.endswith(".json")):
                out.append(p)
    return sorted(out)


def load_tasks(path):
    order, texts = [], {}
    for ln in read_text(path).split("\n"):
        m = TASKLINE.match(ln)
        if m:
            order.append(m.group(1))
            texts[m.group(1)] = ln
    if not order:
        raise InputError("no task lines in %s" % path)
    return order, texts


def occurrences(lines):
    """{(lineno0, id): set of 'plain'/'range'}"""
    occ = {}
    for i, ln in enumerate(lines):
        rng = set()
        for m in RANGE.finditer(ln):
            rng.add(m.start(1))
            rng.add(m.start(3))
        for m in TOK.finditer(ln):
            kind = "range" if m.start() in rng else "plain"
            occ.setdefault((i, m.group(0)), set()).add(kind)
    return occ


def load_rows(path):
    rows = []
    for n, ln in enumerate(read_text(path).split("\n"), 1):
        if not ln.strip() or ln.startswith("#"):
            continue
        f = ln.split("\t", 4)
        if len(f) != 5:
            raise InputError("%s:%d: expected 5 TAB separated columns, got %d" % (path, n, len(f)))
        if f[3] not in KINDS:
            raise InputError("%s:%d: kind %r is not one of %s" % (path, n, f[3], "|".join(KINDS)))
        if not f[1]:
            raise InputError("%s:%d: empty prefix" % (path, n))
        if not TOK.fullmatch(f[2]):
            raise InputError("%s:%d: %r is not a task id" % (path, n, f[2]))
        rows.append((n,) + tuple(f))
    return rows


def engine(files, order, texts, rows, blind=False):
    """files {path: text}. Returns sorted findings [(path, line1, CLASS, detail)]."""
    out = []
    lines = {p: t.split("\n") for p, t in files.items()}
    occ = {p: occurrences(lines[p]) for p in lines}
    bound = {}
    for (rn, rfile, prefix, rid, rkind, kw) in rows:
        if rfile not in lines:
            if not blind:
                out.append((rfile, 0, "ORPHAN-ROW", "list row %d: file is not in the plan set" % rn))  # MUT:orphan
            continue
        hits = [i for i, ln in enumerate(lines[rfile]) if ln.startswith(prefix)]
        if len(hits) != 1:
            if not blind:
                out.append((rfile, hits[0] + 1 if hits else 0, "ORPHAN-ROW", "list row %d: prefix %r begins %d lines, need exactly 1" % (rn, prefix[:40], len(hits))))  # MUT:orphan
            continue
        key = (rfile, hits[0], rid)
        if (hits[0], rid) not in occ[rfile]:
            if not blind:
                out.append((rfile, hits[0] + 1, "ORPHAN-ROW", "list row %d: %s no longer occurs on this line" % (rn, rid)))  # MUT:orphan
            continue
        if key in bound:
            if not blind:
                out.append((rfile, hits[0] + 1, "ORPHAN-ROW", "list row %d: duplicate of row %d for %s" % (rn, bound[key][0], rid)))  # MUT:orphan
            continue
        bound[key] = (rn, rkind, kw)
    for p in sorted(lines):
        for (i, tid), kinds in sorted(occ[p].items()):
            row = bound.get((p, i, tid))
            exists = tid in texts
            if not exists:
                skip = False
                skip = row is not None and row[1] == "citation" and False  # MUT:dangling-bound
                if row is not None and row[1] == "format_example":
                    continue
                if not skip:
                    out.append((p, i + 1, "DANGLING", "%s is cited but tasks.md defines no such task" % tid))  # MUT:dangling
                continue
            if row is None:
                if not blind:
                    out.append((p, i + 1, "UNBOUND", "%s has no row in the citation list" % tid))  # MUT:unbound
                continue
            rn, rkind, kw = row
            if rkind == "format_example":
                out.append((p, i + 1, "EXAMPLE-RESOLVES", "%s is listed as a format example but is a real task (row %d)" % (tid, rn)))  # MUT:example
            elif rkind == "citation":
                if not kw or kw not in texts[tid]:
                    out.append((p, i + 1, "STALE", "%s: keyword %r (row %d) is not in the text of the cited task" % (tid, kw, rn)))  # MUT:stale
            elif rkind == "range_bound" and "plain" in kinds:
                if not blind:
                    out.append((p, i + 1, "UNBOUND", "%s is also cited outside a range here; a range_bound row does not bind it (row %d)" % (tid, rn)))  # MUT:unbound
    return sorted(out)


def control(files, order, texts, rows, blind):
    """Plant the seeded control; return (reported, description)."""
    lines = {p: t.split("\n") for p, t in files.items()}
    cands = []
    for (rn, rfile, prefix, rid, rkind, kw) in rows:
        if rkind != "citation" or rfile not in lines or rid not in order:
            continue
        hits = [i for i, ln in enumerate(lines[rfile]) if ln.startswith(prefix)]
        if len(hits) != 1:
            continue
        ln = lines[rfile][hits[0]]
        toks = [m for m in TOK.finditer(ln) if m.group(0) == rid]
        idx = order.index(rid)
        new = order[idx - 2] if idx >= 2 else order[idx + 2] if idx + 2 < len(order) else None
        if len(toks) == 1 and new and new not in TOK.findall(ln):
            cands.append((rfile, hits[0], toks[0].start(), rid, new))
    if not cands:
        return False, "no bound citation row can carry the control"
    rfile, i, pos, rid, new = sorted(cands)[0]
    mod = dict(files)
    ls = lines[rfile][:]
    ls[i] = ls[i][:pos] + new + ls[i][pos + len(rid):]
    mod[rfile] = "\n".join(ls)
    found = [f for f in engine(mod, order, texts, rows, blind) if f[0] == rfile and f[1] == i + 1]  # MUT:control
    return bool(found), "%s:%d %s -> %s" % (rfile, i + 1, rid, new)


def propose(line, ttext, texts_all):
    """Proposed keyword (seeding aid): a token of the cited task's text that the citing line also holds."""
    cands = set(re.findall(r"`([^`\n]{3,60})`", ttext))
    cands |= set(re.findall(r"(?<![A-Za-z0-9])([A-Z]{2,}[A-Z0-9]*-[A-Z0-9]{1,}[a-z]?)(?![A-Za-z0-9])", ttext))
    best = [c for c in cands if c in line and not TOK.fullmatch(c) and not re.fullmatch(r"T\d{3}[a-z]?", c)]
    if best:
        return max(best, key=lambda c: (len(c), c)), "auto"
    words = set(re.findall(r"[A-Za-z_][A-Za-z0-9_]{6,}", line)) & set(re.findall(r"[A-Za-z_][A-Za-z0-9_]{6,}", ttext))
    if words:
        freq = lambda w: texts_all.count(w)
        w = sorted(words, key=lambda w: (freq(w), -len(w), w))[0]
        return w, "word"
    return title_anchor(ttext), "title"


def title_anchor(ttext):
    """Weak fallback: the first words of the cited task's description (after its id and tags)."""
    t = re.sub(r"^- \[[ xX]\] T\d{3}[a-z]? ", "", ttext)
    t = re.sub(r"^(\[[^\]]*\]\s*)+", "", t)
    m = re.match(r"[^:,(;`.]{6,60}", t)
    return (m.group(0) if m else t[:30]).strip()


def unique_prefix(lines, i):
    ln = lines[i]
    for n in range(min(40, len(ln)), len(ln) + 1):
        pre = ln[:n]
        if "\t" in pre or "\n" in pre:
            return None
        if sum(1 for x in lines if x.startswith(pre)) == 1:
            return pre
    return None


def extract(files, order, texts):
    alltext = "\n".join(texts.values())
    out = []
    for p in sorted(files):
        lines = files[p].split("\n")
        occ = occurrences(lines)
        for (i, tid), kinds in sorted(occ.items()):
            pre = unique_prefix(lines, i)
            if tid not in texts:
                kind, kw, how = "format_example", "", "nonexistent"
            elif "plain" not in kinds:
                kind, kw, how = "range_bound", "", "range"
            else:
                kind = "citation"
                kw, how = propose("\n".join(lines[max(0, i - 2):i + 3]), texts[tid], alltext)
            out.append((p, pre, tid, kind, kw, how, i + 1))
    return out


def main(argv):
    fdir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "specs", "001-full-project-audit-remediation")
    tasks = lst = None
    do_extract = blind = False
    a = list(argv)
    try:
        while a:
            x = a.pop(0)
            if x in ("--feature-dir", "--tasks", "--list"):
                if not a:
                    raise ValueError("%s needs a value" % x)
                v = a.pop(0)
                if x == "--feature-dir":
                    fdir = v
                elif x == "--tasks":
                    tasks = v
                else:
                    lst = v
            elif x == "--extract":
                do_extract = True
            elif x == "--self-test-blind":
                blind = True
            else:
                raise ValueError("unknown argument %r" % x)
    except ValueError as exc:
        print("check_task_citations: usage: %s" % exc, file=sys.stderr)
        print("check_task_citations: usage: check_task_citations.py [--feature-dir DIR] [--tasks FILE] [--list FILE] [--extract] [--self-test-blind]", file=sys.stderr)
        return 2
    fdir = os.path.normpath(fdir)
    tasks = tasks or os.path.join(fdir, "tasks.md")
    lst = lst or os.path.join(fdir, "task_citations.tsv")
    try:
        order, texts = load_tasks(tasks)
        names = plan_set(fdir)
        files = {p: read_text(os.path.join(fdir, p)) for p in names}
        if do_extract:
            for (p, pre, tid, kind, kw, how, ln) in extract(files, order, texts):
                print("\t".join([p, pre if pre is not None else "<NO-UNIQUE-PREFIX>", tid, kind, kw, how, str(ln)]))
            return 0
        rows = load_rows(lst)
    except InputError as exc:
        print("check_task_citations: %s" % exc, file=sys.stderr)
        return 2
    findings = engine(files, order, texts, rows, blind)
    ok, desc = control(files, order, texts, rows, blind)
    if not ok:
        print("check_task_citations: BLIND: the seeded control (%s) was not reported; this run's result says nothing" % desc, file=sys.stderr)
        return 20
    for (p, ln, cls, det) in findings:
        print("FINDING %s %s:%d: %s" % (cls, p, ln, det))
    counts = {}
    for f in findings:
        counts[f[2]] = counts.get(f[2], 0) + 1
    print("check_task_citations: %d findings in %d plan-set files (%s); %d list rows; seeded control reported (%s)" % (
        len(findings), len(files), ", ".join("%s=%d" % kv for kv in sorted(counts.items())) or "none", len(rows), desc))
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
