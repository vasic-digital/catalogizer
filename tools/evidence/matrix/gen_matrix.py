#!/usr/bin/env python3
"""gen_matrix.py - T197. The matrix generator of docs/05 13.1 - 13.3: it re-derives every count from the applicability map (and, when given, the
evidence ledger); a count is never written by hand, and a hand edit of an output is overwritten on the next run.

Usage:
  gen_matrix.py --applicability FILE --out DIR [--ledger FILE] [--timestamp UTC] [--gate]
                [--mint [--mint-cmd CMD] [--mint-ledger FILE]]
  --applicability  matrix/applicability.yaml (schema applicability/1, written by derive_applicability.py, T196)
  --out            directory receiving coverage-matrix.json and coverage-matrix.md (the 11.4.44 revision header, one final newline, no trailing
                   whitespace: an `$EV/**/*.md` file is class `source` of the commit-push table, docs/05 13.1)
  --ledger         JSON list of test records {component, type, verdict, runs, identical_runs, mutation_caught, evidence_class[, blocked]}; the
                   13.1 status of a cell is `present` only with a PASS record of three identical runs, a caught mutation and an evidence class
                   at or above the one the type requires (docs/05 2); `partial` when records exist and none qualifies; `blocked` when a record
                   says so; `absent` when there is none; `n/a` when the map says so
  --gate           the SC-004 gate (13.3): exit 1 when any applicable cell is not `present`, listing the cell and what is missing. It REQUIRES --ledger
                   (exit 2 without one: absence of evidence blocks exactly as a FAIL does, 11.4.135 / 11.4.201(2)); it refuses a map with no applicable
                   cell (exit 3, gate_vacuous) and a ledger record that names no component/type of the map (exit 3, ledger_record_unmatched: a typo
                   would otherwise leave the cell `absent` for the wrong reason or hide the record)
  --mint           at baseline, mint one register item per applicable cell whose declared state is `A` (absent) or `~` (partial), through
                   --mint-cmd (`CMD <component> <type> <A|~>` prints the new item id, exit 0): the real command is the `reg_ids` creation path under
                   scripts/register/locked.sh (T069, blocked until the register go-live); without --mint-cmd the run records the honest skip
                   `mint_cmd_absent`, never a made-up item. A cell already in the mint ledger is never minted again (11.4.214).
Exit: 0 ok; 1 the gate failed; 2 usage; 3 the applicability map is invalid (a `?` cell, a missing type, an n/a without a reason, ...); 4 a mint failed.
"""
import json, os, re, subprocess, sys

TYPES = ["unit", "integration", "e2e", "full_automation", "security", "ddos", "scaling", "chaos", "stress",
         "performance", "benchmarking", "ui", "ux", "challenges", "helixqa"]
SYMS = ["P", "~", "A", "n/a"]
CLASS_RANK = {"source": 1, "artifact": 2, "runtime": 3, "user-visible": 4}
NEED_CLASS = {t: ("runtime" if t not in ("unit", "ui", "ux", "helixqa") else "source" if t == "unit" else "user-visible") for t in TYPES}


def die(code, msg):
    sys.stderr.write("gen_matrix: %s\n" % msg)
    sys.exit(code)


def load_yaml(path):
    try:
        import yaml
    except ImportError:
        die(2, "PyYAML is required (run through `scripts/test-in-container.sh tooling unit`: IMG-TESTUTIL carries it)")
    try:
        with open(path, encoding="utf-8") as fh:
            return yaml.safe_load(fh)
    except (OSError, yaml.YAMLError) as e:
        die(3, "cannot read %s: %s" % (path, e))


def validate(amap):
    """Returns the list of components [(id, {type: (state, reason)})]; exits 3 listing EVERY problem."""
    problems = []
    if not isinstance(amap, dict) or amap.get("schema") != "applicability/1":
        die(3, "schema must be applicability/1 (got %r)" % (amap.get("schema") if isinstance(amap, dict) else None))
    if list(amap.get("types") or []) != TYPES:
        die(3, "types must be exactly the fifteen of docs/05 2, in order: %s" % ", ".join(TYPES))
    comps = amap.get("components")
    if not isinstance(comps, dict) or not comps:
        die(3, "no components")
    out = []
    for cid in sorted(comps):
        cells = (comps[cid] or {}).get("cells")
        if not isinstance(cells, dict):
            problems.append("%s: no cells" % cid)
            continue
        norm = {}
        for t in TYPES:
            if t not in cells:
                problems.append("%s/%s: missing type" % (cid, t))
                continue
            cell = cells[t] or {}
            st = cell.get("state")
            st = "n/a" if st in ("na", "n/a") else st
            reason = (cell.get("reason") or "").strip() if isinstance(cell.get("reason"), str) else ""
            if st == "?":
                problems.append("%s/%s: the cell is '?' (could not classify: read the component, docs/05 4.4 legend)" % (cid, t))
            elif st not in SYMS:
                problems.append("%s/%s: unknown state %r" % (cid, t, st))
            elif st == "n/a" and not reason:
                problems.append("%s/%s: n/a without a reason (an n/a that removes an applicable type is the SC-004 bluff)" % (cid, t))
            elif st != "n/a" and not reason:
                problems.append("%s/%s: a cell needs the reading it came from as its reason" % (cid, t))
            norm[t] = (st, reason)
        extra = sorted(set(cells) - set(TYPES))
        for t in extra:
            problems.append("%s/%s: not one of the fifteen types" % (cid, t))
        out.append((cid, norm))
    if problems:
        sys.stderr.write("gen_matrix: the applicability map is invalid (%d problem(s)):\n" % len(problems))
        for p in problems:
            sys.stderr.write("  - %s\n" % p)
        sys.exit(3)
    return out


def cell_status(cid, t, declared, records):
    """docs/05 13.1: the ledger-derived status of one cell, and the missing evidence."""
    if declared == "n/a":
        return "n/a", []
    mine = [r for r in records if r.get("component") == cid and r.get("type") == t]
    if not mine:
        return "absent", ["no ledger record"]
    need = CLASS_RANK[NEED_CLASS[t]]
    missing = None
    for rec in mine:
        why = []
        if rec.get("verdict") != "PASS": why.append("verdict %s is not PASS" % rec.get("verdict"))
        if not (rec.get("identical_runs") == rec.get("runs") == 3): why.append("not three identical runs")
        if not (rec.get("mutation_caught") is True): why.append("no caught mutation")
        if CLASS_RANK.get(rec.get("evidence_class"), 0) < need: why.append("evidence class %s is below %s" % (rec.get("evidence_class"), NEED_CLASS[t]))
        if not why:
            return "present", []
        missing = why if missing is None else missing
    if any(r.get("blocked") or r.get("verdict") == "blocked" for r in mine):
        return "blocked", ["a record says blocked"]
    return "partial", missing or []


def atomic_write(path, text):
    tmp = path + ".tmp.%d" % os.getpid()
    with open(tmp, "w", encoding="utf-8") as fh:
        fh.write(text)
        fh.flush()
        os.fsync(fh.fileno())
    os.replace(tmp, path)


def _header_state(path):
    """(revision, created, body) of an existing generated md, or None: the 11.4.44 header is carried forward, not rewritten (review m3)."""
    try:
        txt = open(path, encoding="utf-8").read()
    except OSError:
        return None
    mr = re.search(r"^\| Revision \| (\d+) \|$", txt, re.M)
    mc = re.search(r"^\| Created \| (\S+) \|$", txt, re.M)
    if not (mr and mc):
        return None
    body = "\n".join(l for l in txt.split("\n") if not re.match(r"^\| (Revision|Created|Last modified) \|", l))
    return int(mr.group(1)), mc.group(1), body


def render_md(comps, counts, ts, source, prev=None):
    revision, created = 1, ts
    lines = []
    lines.append("# Coverage matrix (generated)")
    lines.append("")
    lines.append("| Field | Value |")
    lines.append("|---|---|")
    lines.append("| Revision | @REV@ |")
    lines.append("| Created | @CREATED@ |")
    lines.append("| Last modified | %s |" % ts)
    lines.append("| Status | generated by tools/evidence/matrix/gen_matrix.py; never hand-edited (a hand edit is overwritten on the next run) |")
    lines.append("| Source | %s |" % source)
    lines.append("")
    d = counts["declared"]
    lines.append("Cells: %d (%d components x %d types). Declared: P %d, ~ %d, A %d, n/a %d. A `?` cell is refused by the generator." %
                 (counts["cells"], len(comps), len(TYPES), d["P"], d["~"], d["A"], d["n/a"]))
    if "ledger_status" in counts:
        s = counts["ledger_status"]
        lines.append("")
        lines.append("Ledger status (docs/05 13.1): present %d, partial %d, absent %d, blocked %d, n/a %d." %
                     (s["present"], s["partial"], s["absent"], s["blocked"], s["n/a"]))
    lines.append("")
    lines.append("| Component | " + " | ".join(TYPES) + " |")
    lines.append("|---|" + "---|" * len(TYPES))
    for cid, cells in comps:
        lines.append("| %s | " % cid + " | ".join(cells[t][0] for t in TYPES) + " |")
    lines.append("")
    lines.append("| Type | P | ~ | A | n/a |")
    lines.append("|---|---|---|---|---|")
    for t in TYPES:
        b = counts["by_type"][t]
        lines.append("| %s | %d | %d | %d | %d |" % (t, b["P"], b["~"], b["A"], b["n/a"]))
    text = "\n".join(l.rstrip() for l in lines) + "\n"
    if prev:   # carry Created forward; Revision rises only when the body (everything but the three header fields) changed
        pbody = prev[2]
        nbody = "\n".join(l for l in text.replace("@REV@", "0").replace("@CREATED@", "0").split("\n") if not re.match(r"^\| (Revision|Created|Last modified) \|", l))
        revision = prev[0] if pbody == nbody else prev[0] + 1
        created = prev[1]
    return text.replace("@REV@", str(revision)).replace("@CREATED@", created)


def main(argv):
    a = {"mint": False, "gate": False}
    i = 0
    while i < len(argv):
        k = argv[i]
        if k in ("--mint", "--gate"):
            a[k[2:]] = True; i += 1
        elif k in ("--applicability", "--out", "--ledger", "--timestamp", "--mint-cmd", "--mint-ledger") and i + 1 < len(argv):
            a[k[2:].replace("-", "_")] = argv[i + 1]; i += 2
        else:
            die(2, "usage: gen_matrix.py --applicability FILE --out DIR [--ledger FILE] [--timestamp UTC] [--gate] [--mint [--mint-cmd CMD] [--mint-ledger FILE]]")
    if "applicability" not in a or "out" not in a:
        die(2, "--applicability and --out are required")
    if a["gate"] and not a.get("ledger"):   # MUT:gate_needs_ledger
        die(2, "--gate requires --ledger: with no ledger there is no evidence, and absence of evidence blocks exactly as a FAIL does (11.4.135); "
               "pass an empty ledger (`[]`) to gate a baseline with nothing recorded")
    if not os.path.isfile(a["applicability"]):
        die(3, "applicability file not found: %s" % a["applicability"])
    comps = validate(load_yaml(a["applicability"]))
    ts = a.get("timestamp") or __import__("datetime").datetime.now(__import__("datetime").timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    records = None
    if a.get("ledger"):
        try:
            with open(a["ledger"], encoding="utf-8") as fh:
                records = json.load(fh)
            assert isinstance(records, list)
        except (OSError, ValueError, AssertionError) as e:
            die(3, "cannot read the ledger %s: %s" % (a["ledger"], e))
    if a["gate"]:
        known = {(cid, t) for cid, cells in comps for t in TYPES}
        stray = [r for r in records if not (isinstance(r, dict) and (r.get("component"), r.get("type")) in known)]   # MUT:gate_unmatched
        if stray:
            die(3, "ledger_record_unmatched: %d ledger record(s) name no component/type of the applicability map (first: %s)" % (len(stray), json.dumps(stray[0], sort_keys=True)[:160]))
        if not any(cells[t][0] != "n/a" for _cid, cells in comps for t in TYPES):   # MUT:gate_vacuous
            die(3, "gate_vacuous: the map has no applicable cell, so the gate would pass over nothing")
    # ---- counts, derived and never typed ----
    declared = {s: 0 for s in SYMS}
    by_type = {t: {s: 0 for s in SYMS} for t in TYPES}
    by_comp = {}
    cells_out = []
    ledger_status = {"present": 0, "partial": 0, "absent": 0, "blocked": 0, "n/a": 0}
    gate_fail = []
    for cid, cells in comps:
        by_comp[cid] = {s: 0 for s in SYMS}
        for t in TYPES:
            st, reason = cells[t]
            declared[st] += 1; by_type[t][st] += 1; by_comp[cid][st] += 1
            row = {"component": cid, "type": t, "declared": st, "reason": reason}
            if records is not None:
                ls, missing = cell_status(cid, t, st, records)
                ledger_status[ls] += 1
                row["status"] = ls
                if ls not in ("present", "n/a"):
                    gate_fail.append((cid, t, ls, missing))
            cells_out.append(row)
    counts = {"cells": len(cells_out), "declared": declared, "by_type": by_type, "by_component": by_comp}
    if records is not None:
        counts["ledger_status"] = ledger_status
    source = os.path.basename(a["applicability"])
    doc = {"schema": "coverage-matrix/1", "generated_at": ts, "source": source, "counts": counts, "cells": cells_out}
    os.makedirs(a["out"], exist_ok=True)
    prev = _header_state(os.path.join(a["out"], "coverage-matrix.md"))
    for name, text in (("coverage-matrix.json", json.dumps(doc, indent=2, sort_keys=True, ensure_ascii=False) + "\n"),
                       ("coverage-matrix.md", render_md(comps, counts, ts, source, prev))):
        atomic_write(os.path.join(a["out"], name), text)
    print("gen_matrix: %d cells, declared P=%d ~=%d A=%d n/a=%d" % (counts["cells"], declared["P"], declared["~"], declared["A"], declared["n/a"]))
    # ---- minting: one register item per applicable absent/partial cell, once ----
    if a["mint"]:
        led_path = a.get("mint_ledger") or os.path.join(a["out"], "mint-ledger.json")
        ledger = {"schema": "matrix-mint-ledger/1", "items": {}}
        if os.path.isfile(led_path):
            try:
                with open(led_path, encoding="utf-8") as fh:
                    ledger = json.load(fh)
                assert isinstance(ledger.get("items"), dict)
            except (OSError, ValueError, AssertionError):
                die(4, "the mint ledger %s is unreadable; refusing to mint (a duplicate item would break 11.4.214)" % led_path)
        todo = [(cid, t, cells[t][0]) for cid, cells in comps for t in TYPES if cells[t][0] in ("A", "~")]
        if not a.get("mint_cmd"):
            msg = "mint: skipped reason=mint_cmd_absent (%d cell(s) owe an item; the register creation path of T069 is not wired, so none was minted)" % len(todo)
            print(msg); sys.stderr.write("gen_matrix: " + msg + "\n")
        else:
            minted = 0
            for cid, t, st in todo:
                key = "%s|%s" % (cid, t)
                if key in ledger["items"]:
                    continue
                p = subprocess.run([a["mint_cmd"], cid, t, st], stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)
                rc = p.returncode
                item = p.stdout.strip().splitlines()[0].strip() if p.stdout.strip() else ""
                if rc != 0:
                    die(4, "the mint command failed for %s (exit %d): %s" % (key, rc, p.stderr.strip()[:200]))
                if not item:
                    die(4, "the mint command printed no item id for %s" % key)
                ledger["items"][key] = {"item": item, "declared": st, "component": cid, "type": t}
                atomic_write(led_path, json.dumps(ledger, indent=2, sort_keys=True) + "\n")
                minted += 1
            print("mint: %d item(s) minted, %d cell(s) already had one" % (minted, len(todo) - minted))
    if a["gate"] and gate_fail:
        sys.stderr.write("gen_matrix: GATE FAIL (SC-004): %d applicable cell(s) are not `present`:\n" % len(gate_fail))
        for cid, t, ls, missing in gate_fail[:200]:
            sys.stderr.write("  - %s / %s: %s (%s)\n" % (cid, t, ls, "; ".join(missing)))
        sys.exit(1)
    sys.exit(0)


if __name__ == "__main__":
    main(sys.argv[1:])
