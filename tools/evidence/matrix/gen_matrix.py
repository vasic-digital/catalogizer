#!/usr/bin/env python3
"""gen_matrix.py - T197. The matrix generator of docs/05 13.1 - 13.3: it re-derives every count from the applicability map (and, when given, the
evidence ledger); a count is never written by hand, and a hand edit of an output is overwritten on the next run.

Usage:
  gen_matrix.py --applicability FILE --out DIR [--ledger FILE --cell-items FILE] [--timestamp UTC] [--gate (--repo DIR | --map-unbound)] [--candidate-fingerprint SHA]
                [--mint [--mint-cmd CMD] [--mint-ledger FILE]]
  --applicability  matrix/applicability.yaml (schema applicability/1, written by derive_applicability.py, T196); read with a loader that REFUSES duplicate and non-string keys
  --out            directory receiving coverage-matrix.json and coverage-matrix.md (the 11.4.44 revision header, one final newline, no trailing
                   whitespace: an `$EV/**/*.md` file is class `source` of the commit-push table, docs/05 13.1)
  --ledger         the evidence ledger AS tools/evidence/evrec WRITES it (JSONL, hash-chained, schema ev/1). Its chain is walked FIRST (evcore.chain_walk: a deleted, forged,
                   reordered or truncated line refuses the whole ledger, exit 3 `ledger_chain_invalid`); no cell is judged from a ledger that does not verify.
  --cell-items     JSON object {"<component>|<type>": "<register item id>"}: the register export that says which evidence item stands for which applicable cell. A key that names no
                   component or type of the map is refused (exit 3 `cell_items_unmatched`: a typo would leave the cell `absent` for the wrong reason). The 13.1 status of a cell is
                   `present` only when the verdict DERIVED by tools/evidence/evverdict for its item is PASS (a RED that failed, three identical GREENs, in the cycle after the last cutting
                   REOPEN), a MUTATION entry of the item is `caught`, and the GREEN entries' evidence_class (schema vocabulary: source < artifact < runtime < user_visible) is at or above
                   the one the type requires (docs/05 2); `blocked` when the latest entry of the item is a `blocked` verdict; `partial` when entries exist and none qualifies; `absent`
                   when there is none (or no item is mapped); `n/a` when the map says so. A later failing run (a cutting REOPEN) removes `present`: the ledger is judged by its LAST cycle.
  --candidate-fingerprint  when given, the GREEN entries must carry exactly this target fingerprint (a record for another build is not evidence for this one)
  --gate           the SC-004 gate (13.3): exit 1 when any applicable cell is not `present`, listing the cell and what is missing. It REQUIRES --ledger and --cell-items (exit 2 without:
                   absence of evidence blocks exactly as a FAIL does, 11.4.135 / 11.4.201(2)), refuses a map with no applicable cell (exit 3 `gate_vacuous`), and requires the
                   map to be BOUND to the repository: either --repo DIR (the map is re-derived from that repository with derive_applicability.py and must equal it cell for cell, exit 3
                   `map_not_bound` otherwise: a truncated map, or an `A` flipped to `n/a`, would remove applicable cells from the gate) or an explicit --map-unbound, which is printed
                   and recorded in the output (`map_bound: false`).
  --mint           at baseline, mint one register item per applicable cell whose declared state is `A` (absent) or `~` (partial), through
                   --mint-cmd (`CMD <component> <type> <A|~> <idempotency-key>` prints the new item id, exit 0; a component id never begins with '-'; the register enforces the key unique, so a
                   repeated call with the same key returns the same item): the real command is the `reg_ids` creation path under scripts/register/locked.sh (T069, blocked until the register
                   go-live); without --mint-cmd the run records the honest skip `mint_cmd_absent`, never a made-up item. The mint ledger (--mint-ledger, default
                   mint-ledger.json NEXT TO THE MAP, never under --out) is locked exclusively for the whole run, an intent row (`pending`) is written BEFORE the register is
                   called, pending rows are retried with the same key, and a cell already `minted` is never minted again (11.4.214, 11.4.253).
Exit: 0 ok; 1 the gate failed; 2 usage; 3 an input is invalid (a `?` cell, a missing type, an n/a without a reason, a ledger that does not verify, ...); 4 a mint failed.
"""
import fcntl, hashlib, json, os, re, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.dirname(HERE))   # python3 -I does not put the script directory on the path
TYPES = ["unit", "integration", "e2e", "full_automation", "security", "ddos", "scaling", "chaos", "stress",
         "performance", "benchmarking", "ui", "ux", "challenges", "helixqa"]
SYMS = ["P", "~", "A", "n/a"]
CLASS_RANK = {"source": 1, "artifact": 2, "runtime": 3, "user_visible": 4}   # the schema vocabulary (contracts/evidence-record.schema.json evidence_class)
NEED_CLASS = {t: ("runtime" if t not in ("unit", "ui", "ux", "helixqa") else "source" if t == "unit" else "user_visible") for t in TYPES}
ITEM_RE = re.compile(r"^(CAT-[0-9]{3,}|FND-[0-9]{4,}|RUN-[0-9]+|AUD-[0-9A-Za-z-]+)$")   # the register item ids the evidence schema accepts
TS_RE = re.compile(r"^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$")
USAGE = ("usage: gen_matrix.py --applicability FILE --out DIR [--ledger FILE --cell-items FILE] [--timestamp UTC] [--gate (--repo DIR | --map-unbound)] "
         "[--candidate-fingerprint SHA] [--mint [--mint-cmd CMD] [--mint-ledger FILE]]")


def die(code, msg):
    sys.stderr.write("gen_matrix: %s\n" % msg)
    sys.exit(code)


def load_yaml(path):
    try:
        import yaml
    except ImportError:
        die(2, "PyYAML is required (run through `scripts/test-in-container.sh tooling unit`: IMG-TESTUTIL carries it)")

    class Strict(yaml.SafeLoader):
        pass

    def construct_mapping(loader, node, deep=False):
        seen = set()
        for k_node, _v in node.value:
            k = loader.construct_object(k_node, deep=True)
            if not isinstance(k, str):
                raise yaml.YAMLError("non-string key %r (a bare yes/no/on/off key is a boolean in YAML 1.1: quote it)" % (k,))
            if k in seen:
                raise yaml.YAMLError("duplicate key %r" % k)
            seen.add(k)
        return yaml.SafeLoader.construct_mapping(loader, node, deep)
    Strict.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, construct_mapping)   # MUT:strict_yaml
    try:
        with open(path, encoding="utf-8") as fh:
            return yaml.load(fh, Loader=Strict)
    except (OSError, yaml.YAMLError, UnicodeDecodeError) as e:
        die(3, "cannot read %s: %s" % (path, e))


def validate(amap):
    """Returns the list of components [(id, {type: (state, reason)})]; exits 3 listing EVERY problem."""
    problems = []
    if not isinstance(amap, dict) or amap.get("schema") != "applicability/1":
        die(3, "schema must be applicability/1 (got %r)" % (amap.get("schema") if isinstance(amap, dict) else None))
    if not isinstance(amap.get("types"), list) or list(amap.get("types")) != TYPES:   # MUT:types_check
        die(3, "types must be exactly the fifteen of docs/05 2, in order: %s" % ", ".join(TYPES))
    comps = amap.get("components")
    if not isinstance(comps, dict) or not comps:
        die(3, "no components")
    out = []
    for cid in sorted(comps):
        if not isinstance(comps[cid], dict):
            problems.append("%s: the component is not a mapping (%r)" % (cid, comps[cid]))
            continue
        if cid.startswith("-"):
            problems.append("%s: a component id must not begin with '-' (it would reach the mint command as an option)" % cid)
        cells = comps[cid].get("cells")
        if not isinstance(cells, dict):
            problems.append("%s: no cells" % cid)
            continue
        norm = {}
        for t in TYPES:
            if t not in cells:
                problems.append("%s/%s: missing type" % (cid, t))
                continue
            cell = cells[t]
            if not isinstance(cell, dict):
                problems.append("%s/%s: the cell is not a mapping {state, reason} (%r)" % (cid, t, cell))
                continue
            st = cell.get("state")
            st = "n/a" if st in ("na", "n/a") else st
            reason = (cell.get("reason") or "").strip() if isinstance(cell.get("reason"), str) else ""
            if st == "?":
                problems.append("%s/%s: the cell is '?' (could not classify: read the component, docs/05 4.4 legend)" % (cid, t))
            elif st not in SYMS:
                problems.append("%s/%s: unknown state %r" % (cid, t, st))
            elif st == "n/a" and not reason:
                problems.append("%s/%s: n/a without a reason (an n/a that removes an applicable type is the SC-004 bluff)" % (cid, t))
            elif st != "n/a" and not reason:   # MUT:reason_needed
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


def ledger_context(path, cell_items, comps):
    """Walks the evrec ledger's chain (refusing the whole ledger on any break) and returns (entries, blobs_dir)."""
    try:
        import evcore
    except ImportError as e:
        die(3, "ledger_unreadable: cannot import tools/evidence/evcore.py: %s" % e)
    try:
        entries, _head = evcore.chain_walk(path)
    except evcore.Refuse as r:
        die(3, "ledger_chain_invalid: %s: %s (no cell is judged from a ledger that does not verify)" % (r.reason, r.detail))
    return entries, os.path.join(os.path.dirname(os.path.abspath(path)), "blobs")


def cell_status(cid, t, declared, ctx):
    """docs/05 13.1: the ledger-derived status of one cell, and the missing evidence."""
    if declared == "n/a":
        return "n/a", []
    item = ctx["cell_items"].get("%s|%s" % (cid, t))
    if not item:
        return "absent", ["no register item is mapped to this cell (--cell-items)"]
    mine = [e for e in ctx["entries"] if e.get("item") == item]
    if not mine:
        return "absent", ["no ledger entry for %s" % item]
    import evverdict
    d = evverdict.derive(ctx["entries"], item, bdir=ctx["blobs"])
    final = [e for e in mine if e["seq"] > d["cycle_after_seq"] and e.get("polarity") != "REOPEN"]
    last = max(mine, key=lambda e: e["seq"])
    need = CLASS_RANK[NEED_CLASS[t]]
    why = []
    if d["verdict"] != "PASS":
        why.append("verdict derived from the ledger is %s (%s)" % (d["verdict"], ", ".join(k for k in evverdict.CHECKS if not d.get(k))))
    greens = [e for e in final if e.get("polarity") == "GREEN"]
    if not any(e.get("mutation", {}).get("result") == "caught" for e in final if e.get("polarity") == "MUTATION"):
        why.append("no caught mutation")
    low = [e for e in greens if CLASS_RANK.get(e.get("evidence_class"), 0) < need]
    if low:   # MUT:class_check
        why.append("evidence class %s is below %s" % (low[0].get("evidence_class"), NEED_CLASS[t]))
    if ctx.get("candidate") and any(e.get("target_fingerprint") != ctx["candidate"] for e in greens):
        why.append("the GREEN entries are for another artifact than the candidate fingerprint")
    if not why:
        return "present", []
    if last.get("verdict") == "blocked":   # MUT:blocked_branch
        return "blocked", ["the latest entry says blocked (%s)" % last.get("blocked_reason")]
    return "partial", why


def atomic_write(path, text):
    tmp = path + ".tmp.%d" % os.getpid()
    try:
        with open(tmp, "w", encoding="utf-8") as fh:
            fh.write(text)
            fh.flush()
            os.fsync(fh.fileno())
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def _header_state(path):
    """(revision, created, body) of an existing generated md, or None: the 11.4.44 header is carried forward, not rewritten (review m3)."""
    try:
        txt = open(path, encoding="utf-8", errors="replace").read()
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


def sha(path):
    try:
        return hashlib.sha256(open(path, "rb").read()).hexdigest()
    except OSError:
        return None


def parse_args(argv):
    a = {"mint": False, "gate": False, "map_unbound": False}
    valued = {"--applicability": "applicability", "--out": "out", "--ledger": "ledger", "--cell-items": "cell_items", "--timestamp": "timestamp", "--mint-cmd": "mint_cmd",
              "--mint-ledger": "mint_ledger", "--repo": "repo", "--candidate-fingerprint": "candidate"}
    i = 0
    while i < len(argv):
        k = argv[i]
        if k in ("--mint", "--gate", "--map-unbound"):
            a[k[2:].replace("-", "_")] = True; i += 1
        elif k in valued:
            if i + 1 >= len(argv) or argv[i + 1] == "":
                die(2, "%s needs a non-empty value\n%s" % (k, USAGE))
            a[valued[k]] = argv[i + 1]; i += 2
        else:
            die(2, USAGE)
    return a


def mint_locked(a, comps, led_path):
    todo = [(cid, t, cells[t][0]) for cid, cells in comps for t in TYPES if cells[t][0] in ("A", "~")]
    if not a.get("mint_cmd"):
        msg = "mint: skipped reason=mint_cmd_absent (%d cell(s) owe an item; the register creation path of T069 is not wired, so none was minted)" % len(todo)
        print(msg); sys.stderr.write("gen_matrix: " + msg + "\n")
        return
    lock = led_path + ".lock"
    os.makedirs(os.path.dirname(os.path.abspath(led_path)), exist_ok=True)
    with open(lock, "w") as lf:
        fcntl.flock(lf, fcntl.LOCK_EX)   # MUT:mint_lock  (held for the WHOLE run: a second run reads the ledger only after the first has written it)
        ledger = {"schema": "matrix-mint-ledger/2", "items": {}}
        if os.path.isfile(led_path):
            try:
                with open(led_path, encoding="utf-8") as fh:
                    ledger = json.load(fh)
                assert isinstance(ledger, dict) and isinstance(ledger.get("items"), dict)
                for k, v in ledger["items"].items():
                    assert isinstance(v, dict) and v.get("state") in ("pending", "minted") and (v["state"] == "pending" or ITEM_RE.match(str(v.get("item", ""))))
            except (OSError, ValueError, AssertionError):
                die(4, "the mint ledger %s is unreadable or malformed; refusing to mint (a duplicate item would break 11.4.214)" % led_path)   # MUT:mint_corrupt
        known_ids = {v["item"] for v in ledger["items"].values() if v.get("state") == "minted"}
        minted = 0; retried = 0
        for cid, t, st in todo:
            key = "%s|%s" % (cid, t)
            cur = ledger["items"].get(key)
            if cur and cur.get("state") == "minted":
                continue
            if cur and cur.get("state") == "pending":
                retried += 1
            ledger["items"][key] = {"state": "pending", "declared": st, "component": cid, "type": t}   # MUT:mint_intent
            atomic_write(led_path, json.dumps(ledger, indent=2, sort_keys=True) + "\n")
            p = subprocess.run([a["mint_cmd"], cid, t, st, "mint:" + key], stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True, stdin=subprocess.DEVNULL)
            item = p.stdout.strip().splitlines()[0].strip() if p.stdout.strip() else ""
            if p.returncode != 0:
                die(4, "the mint command failed for %s (exit %d): %s" % (key, p.returncode, p.stderr.strip()[:200]))
            if not item:
                die(4, "the mint command printed no item id for %s" % key)
            if not ITEM_RE.match(item):
                die(4, "the mint command printed %r for %s, which is not a register item id" % (item[:60], key))
            if item in known_ids:
                die(4, "the mint command returned the item %s for %s, which already stands for another cell" % (item, key))   # MUT:mint_distinct
            known_ids.add(item)
            ledger["items"][key] = {"state": "minted", "item": item, "declared": st, "component": cid, "type": t}
            atomic_write(led_path, json.dumps(ledger, indent=2, sort_keys=True) + "\n")
            minted += 1
        print("mint: %d item(s) minted, %d cell(s) already had one (%d pending row(s) retried)" % (minted, len(todo) - minted, retried))


def main(argv):
    a = parse_args(argv)
    if "applicability" not in a or "out" not in a:
        die(2, "--applicability and --out are required")
    if a["gate"] and not (a.get("ledger") and a.get("cell_items")):   # MUT:gate_needs_ledger
        die(2, "--gate requires --ledger and --cell-items: with no ledger there is no evidence, and absence of evidence blocks exactly as a FAIL does (11.4.135)")
    if a["gate"] and not (a.get("repo") or a["map_unbound"]):   # MUT:gate_needs_binding
        die(2, "--gate requires --repo DIR (the map is re-derived from it) or an explicit --map-unbound (recorded in the output)")
    if a.get("cell_items") and not a.get("ledger"):
        die(2, "--cell-items without --ledger")
    if a.get("timestamp") and not TS_RE.match(a["timestamp"]):   # MUT:timestamp_check
        die(2, "--timestamp must be a UTC instant like 2026-10-07T10:00:00Z (a space or a newline would corrupt the 11.4.44 header of the next run)")
    if os.path.exists(a["out"]) and not os.path.isdir(a["out"]):
        die(2, "--out %s exists and is not a directory" % a["out"])
    if not os.path.isfile(a["applicability"]):
        die(3, "applicability file not found: %s" % a["applicability"])
    amap = load_yaml(a["applicability"])
    comps = validate(amap)
    ts = a.get("timestamp") or __import__("datetime").datetime.now(__import__("datetime").timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    ctx = None; map_bound = None
    if a.get("ledger"):
        if not os.path.isfile(a["ledger"]):
            die(3, "cannot read the ledger %s: not a file" % a["ledger"])
        if not a.get("cell_items"):
            die(2, "--ledger needs --cell-items: the ledger says nothing about which item stands for which cell")
        try:
            with open(a["cell_items"], encoding="utf-8") as fh:
                items = json.load(fh)
            assert isinstance(items, dict) and all(isinstance(k, str) and isinstance(v, str) for k, v in items.items())
        except (OSError, ValueError, AssertionError) as e:
            die(3, "cannot read --cell-items %s: a JSON object of strings is required (%s)" % (a["cell_items"], e))
        known = {"%s|%s" % (cid, t) for cid, _c in comps for t in TYPES}
        stray = sorted(k for k in items if k not in known)
        if stray:   # MUT:cell_items_unmatched
            die(3, "cell_items_unmatched: %d key(s) of %s name no component/type of the applicability map (first: %r)" % (len(stray), a["cell_items"], stray[0][:160]))
        bad = sorted(k for k, v in items.items() if not ITEM_RE.match(v))
        if bad:
            die(3, "cell_items_invalid: %r is not a register item id (%r)" % (items[bad[0]][:60], bad[0]))
        entries, blobs = ledger_context(a["ledger"], items, comps)
        ctx = {"entries": entries, "blobs": blobs, "cell_items": items, "candidate": a.get("candidate")}
    if a["gate"]:
        if not any(cells[t][0] != "n/a" for _cid, cells in comps for t in TYPES):   # MUT:gate_vacuous
            die(3, "gate_vacuous: the map has no applicable cell, so the gate would pass over nothing")
        if a.get("repo"):
            map_bound = True
            try:
                import derive_applicability as da
                fresh = load_yaml_text(da.render(os.path.abspath(a["repo"])))
            except SystemExit as e:
                die(3, "map_not_bound: the repository %s could not be derived (%s)" % (a["repo"], e))
            fc, mc = (fresh or {}).get("components") or {}, amap.get("components") or {}
            diff = sorted(set(fc) ^ set(mc))
            for cid in sorted(set(fc) & set(mc)):
                for t in TYPES:
                    f_ = (fc[cid].get("cells") or {}).get(t) or {}; m_ = (mc[cid].get("cells") or {}).get(t) or {}
                    if f_.get("state") != m_.get("state"):
                        diff.append("%s/%s: map says %r, the repository derives %r" % (cid, t, m_.get("state"), f_.get("state")))
            if diff:   # MUT:map_binding
                die(3, "map_not_bound: the applicability map is not what the repository derives (%d difference(s), first: %s)" % (len(diff), "; ".join(diff[:3])))
        else:
            map_bound = False
            sys.stderr.write("gen_matrix: NOTE --map-unbound: the map was not re-derived from a repository; this run records map_bound=false\n")
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
            if ctx is not None:
                ls, missing = cell_status(cid, t, st, ctx)
                ledger_status[ls] += 1
                row["status"] = ls
                if ls not in ("present", "n/a"):
                    gate_fail.append((cid, t, ls, missing))
            cells_out.append(row)
    counts = {"cells": len(cells_out), "declared": declared, "by_type": by_type, "by_component": by_comp}
    if ctx is not None:
        counts["ledger_status"] = ledger_status
    source = os.path.basename(a["applicability"])
    inputs = {"applicability_sha256": sha(a["applicability"]), "ledger_sha256": sha(a["ledger"]) if a.get("ledger") else None,
              "cell_items_sha256": sha(a["cell_items"]) if a.get("cell_items") else None, "map_bound": map_bound,
              "candidate_fingerprint": a.get("candidate")}
    doc = {"schema": "coverage-matrix/1", "generated_at": ts, "source": source, "inputs": inputs, "counts": counts, "cells": cells_out}
    md_source = "%s (sha256 %s)%s" % (source, inputs["applicability_sha256"][:16], "; ledger sha256 %s" % inputs["ledger_sha256"][:16] if inputs["ledger_sha256"] else "")
    os.makedirs(a["out"], exist_ok=True)
    prev = _header_state(os.path.join(a["out"], "coverage-matrix.md"))
    for name, text in (("coverage-matrix.json", json.dumps(doc, indent=2, sort_keys=True, ensure_ascii=False) + "\n"),
                       ("coverage-matrix.md", render_md(comps, counts, ts, md_source, prev))):
        atomic_write(os.path.join(a["out"], name), text)
    print("gen_matrix: %d cells, declared P=%d ~=%d A=%d n/a=%d" % (counts["cells"], declared["P"], declared["~"], declared["A"], declared["n/a"]))
    # ---- minting: one register item per applicable absent/partial cell, once ----
    if a["mint"]:
        led_path = a.get("mint_ledger") or os.path.join(os.path.dirname(os.path.abspath(a["applicability"])), "mint-ledger.json")
        mint_locked(a, comps, led_path)
    if a["gate"] and gate_fail:
        sys.stderr.write("gen_matrix: GATE FAIL (SC-004): %d applicable cell(s) are not `present`:\n" % len(gate_fail))
        for cid, t, ls, missing in gate_fail[:200]:
            sys.stderr.write("  - %s / %s: %s (%s)\n" % (cid, t, ls, "; ".join(missing)))
        sys.exit(1)
    sys.exit(0)


def load_yaml_text(text):
    import tempfile
    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False, encoding="utf-8") as fh:
        fh.write(text); p = fh.name
    try:
        return load_yaml(p)
    finally:
        os.unlink(p)


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except SystemExit:
        raise
    except Exception as e:   # MUT:toplevel_handler  (K11.1: exit 1 means "the gate failed"; an input the generator cannot process is exit 3)
        sys.stderr.write("gen_matrix: input could not be processed: %s: %s\n" % (type(e).__name__, e))
        sys.exit(3)
