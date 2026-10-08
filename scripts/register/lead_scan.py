#!/usr/bin/env python3
"""lead_scan.py - Step 3/4 lead scan of the findings register (WP-20, tasks T167; doc03 section 10 steps 3 and 4).

Scans the lines of the population files (lead_scan_population.py: tracked Markdown of the frozen snapshot outside every gitlink)
for the closed lead vocabulary (unfinished, not implemented, stub, missing, broken, known issue, workaround, deprecated, disabled,
skipped, TODO, FIXME, regress*, outstanding, pending, not yet - with their inflections: stubs/stubbed/stubbing, known issues,
workarounds, todos, fixmes, not-implemented) and emits one `reg_source_entries` row per lead line, never opening a database:
<out>/lead_scan.sql (+ lead_scan.sql.sha256 and lead_scan.sha256, the same bare hash, the second being the sibling locked.sh import-sql
reads), <out>/lead-scan-run.json.
Usage: lead_scan.py --freeze-json F --out DIR
       lead_scan.py --finalize RUN_JSON --db DB --rows-before N     (after the import: counts the rows in the imported database,
                                                                   read-only, and records lead_scan_entry_rows = after - before)
There is no option that widens the population: the owner's HC-2 answer is read from the fixed path $EV/hc/HC-2.json (see
lead_scan_population.py). Entry (before any snapshot file is read): the listing check (scripts/register/check_freeze_listing.sh when
present, else the inline path-set comparison; UNCONFIRMED vs T161) -> `freeze_listing_moved`; then check_freeze_snapshot.sh ->
`freeze_snapshot_moved`. Exit 0 ok, 2 usage, 20 refusal.
Dispositions (closed reason set), decided per LINE:
  NON-PROBLEM:marker text in document or pattern  - the line names a marker word (todo, fixme) only as the argument of a shell search
                                                    command (grep/rg/awk/sed anywhere on the line, or a code-fenced command line) and holds
                                                    no other lead word
  duplicate of structured source                  - a structured source (S-01..S-06, S-10, S-11, S-17..S-21, S-25) holds an entry that covers
                                                    THIS line (a whole-file entry, an entry on this line, or the item block of an S-21 item);
                                                    raw_severity carries `covered_by=<source locator> :: <entry locator>`, the row the
                                                    importer folds this line into
  lead                                            - every other lead line (a line of a structured file that no structured entry covers is a lead)
The disposition is in raw_status; the canonical lead words are in legacy_id.
Control needles (round-23 review C3): (1) a built-in line set, one positive per vocabulary word and inflection and a set of negatives, is
scanned by the SAME function before the real scan; (2) each declared corpus needle (NEEDLES: file + text of a line that holds a lead word)
whose file is in the population must be found as a row at that line (`lead_scan_blind` otherwise) and its text must exist
(`lead_scan_needle_missing` otherwise).
"""
import hashlib, importlib.util, json, os, re, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
PARSER = "lead_scan.py@2"
# (regex fragment, canonical word); inflections are explicit, never a bare stem
VOCAB = (
    (r"unfinished", "unfinished"), (r"not[ -]implemented", "not implemented"), (r"stub(?:s|bed|bing)?", "stub"),
    (r"missing", "missing"), (r"broken", "broken"), (r"known[ -]issues?", "known issue"), (r"workarounds?", "workaround"),
    (r"deprecated", "deprecated"), (r"disabled", "disabled"), (r"skipped", "skipped"), (r"todos?", "todo"), (r"fixmes?", "fixme"),
    (r"regress\w*", "regress"), (r"outstanding", "outstanding"), (r"pending", "pending"), (r"not[ -]yet", "not yet"))
LEAD_RE = re.compile(r"(?<!\w)(" + "|".join("(?:%s)" % f for f, _ in VOCAB) + r")(?!\w)", re.I)
MARKER_WORDS = {"todo", "fixme"}
CMD_RE = re.compile(r"(?<![A-Za-z0-9_])(grep|rg|awk|sed)(?![A-Za-z0-9_])")
FENCE_CMD_RE = re.compile(r"^\s*(?:[$>]\s*)?(?:sudo\s+)?(grep|rg|egrep|fgrep|awk|sed|ag|ack|find|git|xargs|cat|echo|printf|jq|ls|sh|bash)\b")
FENCE_RE = re.compile(r"^\s*(```|~~~)")
STRUCTURED = {"S-01", "S-02", "S-03", "S-04", "S-05", "S-06", "S-10", "S-11", "S-17", "S-18", "S-19", "S-20", "S-21", "S-25"}
NONPROBLEM = "NON-PROBLEM:marker text in document or pattern"
DUPLICATE = "duplicate of structured source"
# declared corpus needles: (population file, text of a line that holds the lead word, that word, the disposition the line must get).
# The line is located by a plain text search of the source file, never from the scan's own rows.
NEEDLES = (("docs/LANDMINES.md", "add the missing test; do not skip with", "missing", "lead"),)
# built-in vocabulary control: every line must yield exactly the listed canonical words
CONTROL_POSITIVE = (
    ("this part is unfinished", {"unfinished"}), ("it is not implemented", {"not implemented"}), ("it is not-implemented", {"not implemented"}),
    ("a stub only", {"stub"}), ("two stubs here", {"stub"}), ("was stubbed out", {"stub"}), ("stubbing the call", {"stub"}),
    ("the file is missing", {"missing"}), ("a broken link", {"broken"}), ("a known issue", {"known issue"}), ("## 10. Known Issues", {"known issue"}),
    ("use a workaround", {"workaround"}), ("two workarounds", {"workaround"}), ("a deprecated API", {"deprecated"}),
    ("a disabled test", {"disabled"}), ("a skipped test", {"skipped"}), ("TODO write it", {"todo"}), ("many todos left", {"todo"}),
    ("FIXME later", {"fixme"}), ("the fixmes", {"fixme"}), ("a regression", {"regress"}), ("it regressed", {"regress"}),
    ("an outstanding item", {"outstanding"}), ("pending review", {"pending"}), ("not yet done", {"not yet"}), ("not-yet done", {"not yet"}))
CONTROL_NEGATIVE = ("depending on the host", "a stubborn problem", "unstubbed", "the todolist app", "missingno", "suspending", "prepending", "")


def load(name):
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m); return m


def sha_text(s):
    return hashlib.sha256(s.encode("utf-8")).hexdigest()


def refuse(reason, detail=""):
    sys.stderr.write("lead_scan: REFUSED reason=%s %s\n" % (reason, detail)); return 20


def canon(token):
    for frag, word in VOCAB:
        if re.fullmatch(frag, token, re.I):
            return word
    return token.lower()


def lead_words(line):
    return sorted({canon(m.group(1)) for m in LEAD_RE.finditer(line)})


def disposition_marker(line, words, infence):
    """True when every lead word of the line is a marker word (todo, fixme) and the line is a shell search command line, or a command
    line inside a code fence."""
    if not words or not set(words) <= MARKER_WORDS:
        return False
    if CMD_RE.search(line):
        first = min(m.start() for m in LEAD_RE.finditer(line))
        return CMD_RE.search(line).start() < first
    return infence and FENCE_CMD_RE.match(line) is not None


def listing_matches_manifest(fj):
    """inline fallback of check_freeze_listing.sh (T161): the listing's path set equals the manifest's path set."""
    lp, mp = fj.get("listing"), fj.get("manifest")
    listed = {p.decode("utf-8", "replace") for p in open(lp, "rb").read().split(b"\0") if p}
    want = {e["path"] for e in json.load(open(mp, encoding="utf-8"))["entries"]}
    return sorted(("added " + p) for p in listed - want) + sorted(("missing " + p) for p in want - listed)


def vocabulary_control():
    """-> list of failures of the built-in control set (empty = the instrument sees every word and nothing else)."""
    bad = []
    for line, want in CONTROL_POSITIVE:
        got = set(lead_words(line))
        if got != want:
            bad.append("positive %r: got %s want %s" % (line, sorted(got), sorted(want)))
    for line in CONTROL_NEGATIVE:
        if lead_words(line):
            bad.append("negative %r: matched %s" % (line, lead_words(line)))
    return bad


def structured_cover(en, sn):
    """-> {path: {"file": (source loc, entry loc) or None, "lines": {n: (source loc, entry loc)}, "blocks": [(start, end, src, loc)]}} built by
    running the structured sources' own rules over the snapshot (the same entries the enumerator emits)."""
    claimed, cover = {}, {}
    for cls, kind, loc, sel, rule in en.SOURCES:
        if en.is_noclaim(cls, loc):
            continue
        fl = en.root_other_reports(sn.files, claimed) if loc.startswith("/*.md") else sel(sn.files, claimed)
        for p in fl:
            claimed[p] = loc
        if cls not in STRUCTURED:
            continue
        sn.class_files.setdefault(cls, set()).update(fl)
        ents = rule(sn, fl, {})
        for e in ents:
            m = re.match(r"^(.*?):L(\d+)(?:#(.*))?$", e.locator)
            path = e.locator.split("#", 1)[0] if not m else m.group(1)
            c = cover.setdefault(path, {"file": None, "lines": {}, "blocks": []})
            if not m:
                if "#" not in e.locator or e.locator.startswith(path + "#"):
                    c["file"] = c["file"] or (loc, e.locator)
                continue
            n = int(m.group(2))
            c["lines"].setdefault(n, (loc, e.locator))
            if m.group(3) and re.fullmatch(r"item\d+", m.group(3)):
                c["blocks"].append([n, None, loc, e.locator])
    for path, c in cover.items():     # an S-21 item entry covers its whole block: up to the next item entry, or for the last item up to the next `## ` heading
        c["blocks"].sort()
        for i, b in enumerate(c["blocks"]):
            if i + 1 < len(c["blocks"]):
                b[1] = c["blocks"][i + 1][0] - 1
            else:
                ls = [l[:-1] if l.endswith("\r") else l for l in sn.text(path).split("\n")]
                b[1] = next((n - 1 for n in range(b[0] + 1, len(ls) + 1) if re.match(r"^##\s", ls[n - 1])), len(ls))
    return cover


def covering(cover, path, n):
    c = cover.get(path)
    if not c:
        return None
    if c["file"]:
        return c["file"]
    if n in c["lines"]:
        return c["lines"][n]
    for a, b, src, loc in c["blocks"]:
        if a <= n <= b:
            return (src, loc)
    return None


def finalize(run_json, db, before):
    import sqlite3
    try:
        before = int(before)
        run = json.load(open(run_json, encoding="utf-8"))
        con = sqlite3.connect("file:%s?mode=ro" % db, uri=True)
        after = con.execute("SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.locator=?",
                            (run["source_locator"],)).fetchone()[0]
    except (ValueError, OSError, KeyError, sqlite3.Error) as e:
        return refuse("finalize_failed", "%s: %s" % (type(e).__name__, e))
    run.update({"rows_before": before, "rows_after": after, "lead_scan_entry_rows": after - before, "lead_scan_entry_rows_basis": "counted in the imported database before and after"})
    json.dump(run, open(run_json, "w"), indent=1, sort_keys=True)
    print("lead_scan: finalized rows_before=%d rows_after=%d lead_scan_entry_rows=%d emitted=%d" % (before, after, after - before, run["emitted_rows"]))
    return 0


def run(argv):
    a = {"fj": None, "out": None, "fin": None, "db": None, "before": None}
    names = {"--freeze-json": "fj", "--out": "out", "--finalize": "fin", "--db": "db", "--rows-before": "before"}
    i = 0
    while i < len(argv):
        if argv[i] in names and i + 1 < len(argv):
            a[names[argv[i]]] = argv[i + 1]; i += 2
        else:
            sys.stderr.write("usage: lead_scan.py --freeze-json F --out DIR | --finalize RUN_JSON --db DB --rows-before N\n"); return 2
    if a["fin"]:
        if not a["db"] or a["before"] is None:
            sys.stderr.write("usage: lead_scan.py --finalize RUN_JSON --db DB --rows-before N\n"); return 2
        return finalize(a["fin"], a["db"], a["before"])
    if not a["fj"] or not a["out"]:
        sys.stderr.write("usage: lead_scan.py --freeze-json F --out DIR (--freeze-json is required)\n"); return 2
    try:
        fj = json.load(open(a["fj"], encoding="utf-8")); snap, manifest = fj["snapshot"], fj["manifest"]
    except (OSError, ValueError, KeyError):
        return refuse("freeze_json_invalid", a["fj"])
    pop = load("lead_scan_population")
    lchk = os.path.join(HERE, "check_freeze_listing.sh")
    if fj.get("listing") and os.path.exists(lchk):
        r = subprocess.run([lchk, fj["listing"], manifest], capture_output=True)
        bad = [] if r.returncode == 0 else [r.stderr.decode("utf-8", "replace").strip()]
    elif fj.get("listing"):
        bad = listing_matches_manifest(fj)     # MUT:listing-check
    else:
        return refuse("freeze_listing_missing", "freeze json has no `listing`")
    if bad:
        sys.stderr.write("\n".join(bad) + "\n"); return refuse("freeze_listing_moved", "; ".join(bad)[:300])
    r = subprocess.run([os.path.join(HERE, "check_freeze_snapshot.sh"), snap, manifest], capture_output=True)   # MUT:snapshot-check
    if r.returncode != 0:
        err = r.stderr.decode("utf-8", "replace"); sys.stderr.write(err)
        mm = re.search(r"REFUSED reason=(freeze_manifest_invalid|freeze_special_file|freeze_path_unsafe)", err)
        return refuse(mm.group(1) if mm else "freeze_snapshot_moved", "check_freeze_snapshot.sh exited %d" % r.returncode)
    vc = vocabulary_control()
    if vc:
        return refuse("lead_scan_blind", "vocabulary control failed: " + "; ".join(vc)[:300])   # MUT:vocab-control
    try:
        files, extra = pop.population(fj)
    except pop.PopulationRefusal as e:
        return refuse(e.reason, e.detail)
    en = load("enumerate_sources")
    sn = en.Snapshot(snap)
    cover = structured_cover(en, sn)
    needles, needle_expect = [], {}
    for npath, ntext, nword, ndisp in NEEDLES:
        if npath not in files:
            continue
        ls = [l[:-1] if l.endswith("\r") else l for l in sn.text(npath).split("\n")]
        at = [n for n, l in enumerate(ls, 1) if ntext in l]
        if not at:
            return refuse("lead_scan_needle_missing", "%s holds no line with %r: update NEEDLES" % (npath, ntext))
        needles.append("%s:L%d" % (npath, at[0]))
        needle_expect["%s:L%d" % (npath, at[0])] = (nword, ndisp)
    rows, symlinks = [], []
    for p in files:
        if p in sn.links:
            symlinks.append(p)
            continue
        infence = False
        for n, l in enumerate(sn.text(p).split("\n"), 1):
            l = l[:-1] if l.endswith("\r") else l
            fence_line = FENCE_RE.match(l) is not None
            if fence_line:
                infence = not infence
            words = lead_words(l)
            if not words:
                continue
            covered = covering(cover, p, n)
            if disposition_marker(l, words, infence and not fence_line):
                disp, cov = NONPROBLEM, None
            elif covered:
                disp, cov = DUPLICATE, "covered_by=%s :: %s" % covered
            else:
                disp, cov = "lead", None
            rows.append(en.E("%s:L%d" % (p, n), sha_text(p + "\0" + l), legacy_id=",".join(words), title=l.strip()[:200], status=disp, severity=cov))
    have = {r.locator: r for r in rows}
    for nl in needles:
        r = have.get(nl)
        word, disp = needle_expect[nl]
        if r is None or word not in (r.legacy_id or "").split(",") or r.status != disp:   # MUT:needle-check
            return refuse("lead_scan_blind", "control needle %s: expected a row with word %r and disposition %r, got %s" %
                          (nl, word, disp, "no row" if r is None else "%r/%r" % (r.legacy_id, r.status)))
    rows.sort(key=lambda r: r.locator.encode("utf-8"))
    scanned = fj.get("frozen_at") or "1970-01-01T00:00:00Z"
    loc = "lead-scan:tracked Markdown outside every gitlink"
    txt = en.render_block([("report_doc", loc, PARSER, rows, None)], scanned, "generated by lead_scan.py@2; INSERT OR IGNORE only (idempotent)")
    h = en.write_sql_files(a["out"], "lead_scan", txt)
    json.dump({"command": [sys.executable, os.path.abspath(__file__)] + argv, "frozen_head": fj.get("head"), "population_files": len(files),
               "extra_roots": extra, "hc2_path": pop.hc2_path(), "emitted_rows": len(rows), "lead_scan_entry_rows": None,
               "lead_scan_entry_rows_basis": "not counted yet: run --finalize after the import", "source_locator": loc,
               "needles": needles, "symlink_population_files_skipped": symlinks,
               "dispositions": {d: sum(1 for r in rows if r.status == d) for d in sorted({r.status for r in rows})}, "sql_sha256": h},
              open(os.path.join(a["out"], "lead-scan-run.json"), "w"), indent=1, sort_keys=True)
    print("lead_scan: ok population_files=%d rows=%d" % (len(files), len(rows)))
    return 0


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:]))
