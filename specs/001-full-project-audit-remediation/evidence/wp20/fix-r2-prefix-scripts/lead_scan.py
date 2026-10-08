#!/usr/bin/env python3
"""lead_scan.py - Step 3/4 lead scan of the findings register (WP-20, tasks T167; doc03 section 10 steps 3 and 4).

Scans the lines of the population files (lead_scan_population.py: tracked Markdown of the frozen snapshot outside every gitlink)
for the closed lead vocabulary (unfinished, not implemented, stub, missing, broken, known issue, workaround, deprecated, disabled,
skipped, TODO, FIXME, regress*, outstanding, pending, not yet) and emits one `reg_source_entries` row per lead line, never opening a
database: <out>/lead_scan.sql (+ .sha256, bare hash), <out>/lead-scan-run.json (command line, frozen head, population_files,
extra_roots, lead_scan_entry_rows, needle result). Import goes through the single-writer contract (locked.sh import-sql, T165) or, in
tests, `sqlite3 .read` on a scratch DB.
Usage: lead_scan.py --freeze-json F --out DIR [--hc2 PATH]
Entry (before any snapshot file is read): the listing check (scripts/register/check_freeze_listing.sh when present, else the inline
path-set comparison listing == manifest of lead_scan_population.listing_matches_manifest; UNCONFIRMED vs T161) -> `freeze_listing_moved`;
then check_freeze_snapshot.sh -> `freeze_snapshot_moved`. Exit 0 ok, 2 usage, 20 refusal.
Dispositions (closed reason set): a lead line inside a shell grep/rg pattern or a code-fenced command that names a marker word is
NON-PROBLEM with reason `marker text in document or pattern`; a lead line of a file the structured sources (S-01..S-06, S-10, S-11,
S-17..S-21, S-25) already enumerate is `duplicate of structured source`; every other lead line is `lead`. The disposition is in
raw_status; the lead words are in legacy_id. Control needle: the first lead line of docs/LANDMINES.md must be found by this same scan
before a zero is believed (`lead_scan_blind` otherwise).
"""
import hashlib, importlib.util, json, os, re, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
PARSER = "lead_scan.py@1"
LEAD_RE = re.compile(r"\b(unfinished|not implemented|stubs?|missing|broken|known issue|workaround|deprecated|disabled|skipped|todo|fixme|regress\w*|outstanding|pending|not yet)\b", re.I)
PATTERN_RE = re.compile(r"(\bgrep\b|\brg\b|\bawk\b|\bsed\b).*(todo|fixme|hack|xxx)", re.I)
FENCE_RE = re.compile(r"^\s*(```|~~~)")
STRUCTURED = {"S-01", "S-02", "S-03", "S-04", "S-05", "S-06", "S-10", "S-11", "S-17", "S-18", "S-19", "S-20", "S-21", "S-25"}


def load(name):
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m); return m


def sha_text(s):
    return hashlib.sha256(s.encode("utf-8")).hexdigest()


def q(v):
    return "NULL" if v is None else "'" + str(v).replace("'", "''") + "'"


def refuse(reason, detail=""):
    sys.stderr.write("lead_scan: REFUSED reason=%s %s\n" % (reason, detail)); return 20


def listing_matches_manifest(fj):
    """inline fallback of check_freeze_listing.sh (T161): the listing's path set equals the manifest's path set minus nothing."""
    lp, mp = fj.get("listing"), fj.get("manifest")
    listed = {p.decode("utf-8", "replace") for p in open(lp, "rb").read().split(b"\0") if p}
    want = {e["path"] for e in json.load(open(mp, encoding="utf-8"))["entries"]}
    return sorted(("added " + p) for p in listed - want) + sorted(("missing " + p) for p in want - listed)


def run(argv):
    a = {"fj": None, "out": None, "hc2": None}
    i = 0
    while i < len(argv):
        if argv[i] in ("--freeze-json", "--out", "--hc2") and i + 1 < len(argv):
            a[{"--freeze-json": "fj", "--out": "out", "--hc2": "hc2"}[argv[i]]] = argv[i + 1]; i += 2
        else:
            sys.stderr.write("usage: lead_scan.py --freeze-json F --out DIR [--hc2 PATH]\n"); return 2
    if not a["fj"] or not a["out"]:
        sys.stderr.write("usage: lead_scan.py --freeze-json F --out DIR [--hc2 PATH] (--freeze-json is required)\n"); return 2
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
        sys.stderr.write(r.stderr.decode("utf-8", "replace")); return refuse("freeze_snapshot_moved", "check_freeze_snapshot.sh exited %d" % r.returncode)
    try:
        files, extra = pop.population(fj, a["hc2"])
    except pop.PopulationRefusal as e:
        return refuse(e.reason, e.detail)
    en = load("enumerate_sources")
    sn = en.Snapshot(snap)
    claimed, structured_files = {}, set()
    for cls, kind, loc, sel, rule in en.SOURCES:
        if cls == "S-23":
            continue
        fl = en.root_other_reports(sn.files, claimed) if loc.startswith("/*.md") else sel(sn.files, claimed)
        for p in fl:
            claimed[p] = loc
            if cls in STRUCTURED:
                structured_files.add(p)
    rows, needle_line = [], None
    for p in files:
        if p in sn.links:
            continue
        infence = False
        for n, l in enumerate(sn.text(p).split("\n"), 1):
            l = l[:-1] if l.endswith("\r") else l
            if FENCE_RE.match(l):
                infence = not infence
            m = LEAD_RE.findall(l)
            if not m:
                continue
            if PATTERN_RE.search(l):
                disp = "NON-PROBLEM:marker text in document or pattern"
            elif p in structured_files:
                disp = "duplicate of structured source"
            else:
                disp = "lead"
            if p == "docs/LANDMINES.md" and needle_line is None:
                needle_line = "%s:L%d" % (p, n)
            rows.append(("%s:L%d" % (p, n), sha_text(p + "\0" + l), ",".join(sorted({x.lower() for x in m})), l.strip()[:200], disp))
    if "docs/LANDMINES.md" in files and needle_line is not None and not any(r[0] == needle_line for r in rows):
        return refuse("lead_scan_blind", "control needle %s not found by the scan" % needle_line)
    if "docs/LANDMINES.md" in files and needle_line is None:
        return refuse("lead_scan_blind", "docs/LANDMINES.md is in the population but holds no lead line to serve as the control needle")
    rows.sort(key=lambda r: r[0].encode("utf-8"))
    scanned = fj.get("frozen_at") or "1970-01-01T00:00:00Z"
    loc = "lead-scan:tracked Markdown outside every gitlink"
    sql = ["PRAGMA foreign_keys=ON;", "BEGIN;", "INSERT OR IGNORE INTO reg_sources(kind,locator,parser) VALUES ('report_doc',%s,%s);" % (q(loc), q(PARSER))]
    sid = "(SELECT source_id FROM reg_sources WHERE locator=%s)" % q(loc)
    for lo, sh, ids, title, disp in rows:
        sql.append("INSERT OR IGNORE INTO reg_source_entries(source_id,locator,legacy_id,title,raw_status,raw_severity,entry_sha256,scanned_at) VALUES (%s,%s,%s,%s,%s,NULL,%s,%s);" % (sid, q(lo), q(ids), q(re.sub(r"[\x00-\x1f]", " ", title)), q(disp), q(sh), q(scanned)))
    sql.append("UPDATE reg_sources SET last_scanned_at=%s, scanned_entry_count=%d WHERE locator=%s;" % (q(scanned), len(rows), q(loc)))
    sql.append("COMMIT;")
    txt = "\n".join(sql) + "\n"
    os.makedirs(a["out"], exist_ok=True)
    sp = os.path.join(a["out"], "lead_scan.sql")
    open(sp, "w", encoding="utf-8", newline="\n").write(txt)
    open(sp + ".sha256", "w").write(sha_text(txt) + "\n")
    json.dump({"command": ["lead_scan.py"] + argv, "frozen_head": fj.get("head"), "population_files": len(files), "extra_roots": extra,
               "lead_scan_entry_rows": len(rows), "needle": needle_line, "sql_sha256": sha_text(txt)}, open(os.path.join(a["out"], "lead-scan-run.json"), "w"), indent=1, sort_keys=True)
    print("lead_scan: ok population_files=%d rows=%d" % (len(files), len(rows)))
    return 0


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:]))
