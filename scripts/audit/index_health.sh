#!/usr/bin/env bash
# index_health.sh - T021 (WP-02). Read-only index health gate for proofs P1..P8 (docs/02 section 4.1, 4.2).
#
# Purpose   Judge CAPTURED outputs of the two code indexes (CodeGraph status/files, Lumen index_status text) and write
#           `index-health/1` JSON. It never runs an index writer and never opens an index database: every input is a file the
#           caller captured beforehand (docs/scripts/index_health.md).
# Usage     index_health.sh --out FILE --cg-status FILE --cg-files FILE --tracked FILE --needle-pos PATH --needle-neg PATH
#                           [--lumen-status FILE | --lumen-bin FILE] [--lumen-files FILE] [--parity-tolerance-pct N]
#                           [--lumen-unindexable-ext CSV] [--third-party-roots FILE] [--own-org-roots FILE]
# Exit      0 = no proof FAIL (verdict PASS), 1 = at least one proof FAIL (verdict FAIL), 2 = usage error.
# Side effects  writes only FILE (atomic rename in its directory). Needs python3.
# Contract  scripts/audit/tests/test_index_health.sh (13 checks). A required field that is absent is a FAIL, never read as zero.
set -u
command -v python3 >/dev/null 2>&1 || { echo "index_health: python3 required" >&2; exit 2; }
exec python3 - "$@" <<'PY'
import datetime, hashlib, json, os, re, sys, tempfile, unicodedata

def usage(msg):
    sys.stderr.write("index_health: usage: %s\n" % msg)
    sys.exit(2)

VALUE_OPTS = ("--out", "--cg-status", "--cg-files", "--tracked", "--needle-pos", "--needle-neg", "--lumen-status",
              "--lumen-bin", "--lumen-files", "--parity-tolerance-pct", "--lumen-unindexable-ext",
              "--third-party-roots", "--own-org-roots")
args = sys.argv[1:]
opts = {}
i = 0
while i < len(args):
    a = args[i]
    if a == "--":
        i += 1
        break
    if a not in VALUE_OPTS:
        usage("unknown option %r" % a)
    if i + 1 >= len(args):
        usage("%s requires a value" % a)
    if a in opts:
        usage("%s given twice" % a)
    v = args[i + 1]
    if v == "" or v.startswith("-") or any(ord(c) < 32 or ord(c) == 127 for c in v):
        usage("%s value is empty, option-like or has a control character" % a)
    opts[a] = v
    i += 2
if i < len(args):
    usage("unexpected operand")
for req in ("--out", "--cg-status", "--cg-files", "--tracked", "--needle-pos", "--needle-neg"):
    if req not in opts:  # MUT:require-out
        usage("%s is required" % req)
try:
    tol = float(opts.get("--parity-tolerance-pct", "0"))
    if tol < 0 or tol != tol:
        raise ValueError
except ValueError:
    usage("--parity-tolerance-pct must be a number >= 0")
unind = set(e.strip().lstrip(".").lower() for e in opts.get("--lumen-unindexable-ext", "").split(",") if e.strip())

inputs = {}
def read_raw(key, path):
    """Return bytes or None; record the sha256 identity of what was judged."""
    try:
        with open(path, "rb") as fh:
            data = fh.read()
    except OSError:
        inputs[key] = {"path": path, "sha256": None, "readable": False}
        return None
    inputs[key] = {"path": path, "sha256": hashlib.sha256(data).hexdigest(), "readable": True}
    return data

def row(pid, verdict, **kw):
    r = {"id": pid, "verdict": verdict}
    r.update(kw)
    return r

def lookup(d, *keys):
    for k in keys:
        if not isinstance(d, dict) or k not in d:
            return (False, None)
        d = d[k]
    return (True, d)

def is_count(v):
    return isinstance(v, int) and not isinstance(v, bool) and v >= 0

# ---- P1 state -------------------------------------------------------------------------------------------------------
def p1():
    raw = read_raw("cg_status", opts["--cg-status"])
    cmd = "codegraph status --json (captured file)"
    if raw is None:
        return row("P1", "FAIL", cmd=cmd, reason="status_file_unreadable"), None
    try:
        st = json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        return row("P1", "FAIL", cmd=cmd, reason="status_not_json"), None
    required = (("index", "state"), ("index", "pendingRefs"), ("index", "reindexRecommended"),
                ("pendingChanges", "added"), ("pendingChanges", "modified"), ("pendingChanges", "removed"),
                ("worktreeMismatch",))
    absent = [".".join(p) for p in required if not lookup(st, *p)[0]]
    if absent:
        return row("P1", "FAIL", cmd=cmd, reason="required_field_absent", absent=absent), st
    why = []
    if lookup(st, "index", "state")[1] != "complete":
        why.append("index.state_not_complete")
    pr = lookup(st, "index", "pendingRefs")[1]
    if not is_count(pr):
        why.append("index.pendingRefs_not_a_count")
    elif pr != 0:  # MUT:p1-pendingrefs
        why.append("index.pendingRefs_nonzero")
    for k in ("added", "modified", "removed"):
        v = lookup(st, "pendingChanges", k)[1]
        if not is_count(v):
            why.append("pendingChanges.%s_not_a_count" % k)
        elif v != 0:
            why.append("pendingChanges.%s_nonzero" % k)
    if lookup(st, "worktreeMismatch")[1] is not None:  # MUT:p1-worktree
        why.append("worktreeMismatch_set")
    if lookup(st, "index", "reindexRecommended")[1] is not False:
        why.append("reindexRecommended_not_false")
    return row("P1", "FAIL" if why else "PASS", cmd=cmd, reasons=why), st

# ---- P2 completeness vs tracked, P3 scope ---------------------------------------------------------------------------
def load_lines(key, path):
    raw = read_raw(key, path)
    if raw is None:
        return None
    return [l for l in raw.decode("utf-8", "replace").split("\n") if l.strip() != ""]

indexed = None
def p2():
    global indexed
    cmd = "codegraph files --json (captured file) vs tracked list"
    raw = read_raw("cg_files", opts["--cg-files"])
    tracked_l = load_lines("tracked", opts["--tracked"])
    needles = {"needle_pos": {"path": opts["--needle-pos"], "found": None},
               "needle_neg": {"path": opts["--needle-neg"], "found": None}}
    if raw is None or tracked_l is None:
        return row("P2", "FAIL", cmd=cmd, reason="input_unreadable", **needles)
    try:
        files = json.loads(raw.decode("utf-8"))
        if not isinstance(files, list):
            raise ValueError
        indexed = set()
        for f in files:
            if not isinstance(f, dict) or not isinstance(f.get("path"), str):
                raise ValueError
            indexed.add(f["path"])
    except (ValueError, UnicodeDecodeError):
        indexed = None
        return row("P2", "FAIL", cmd=cmd, reason="files_not_a_list_of_path_objects", **needles)
    tracked = set(tracked_l)
    missing = sorted(tracked - indexed)
    needles["needle_pos"]["found"] = opts["--needle-pos"] in indexed
    needles["needle_neg"]["found"] = opts["--needle-neg"] in indexed
    why = []
    if not tracked:
        why.append("tracked_list_empty_blind")
    if missing:
        why.append("tracked_files_not_indexed")
    if not needles["needle_pos"]["found"]:  # MUT:p2-pos
        why.append("positive_needle_missing")
    if needles["needle_neg"]["found"]:  # MUT:p2-neg
        why.append("negative_needle_found")
    return row("P2", "FAIL" if why else "PASS", cmd=cmd, reasons=why, tracked_in_scope=len(tracked),
               indexed=len(indexed), missing_count=len(missing), missing=missing[:50], **needles)

def noncanonical_root(r):
    """A roots-file line must be a plain repo-relative path: no control character (CRLF included), no surrounding whitespace, no leading
    './' or '/', no empty, '.' or '..' component. A line that is not is refused, never normalised silently and never matched as written
    (a non-canonical line matches no indexed path, so a hit under that root would read as clean).
    Round 6 (N6-5) closes the ENCODING class as well: a root that can never equal an indexed path for an encoding reason reads as clean in
    exactly the same way, so it is refused too: any character of a Unicode category C* (control, format: a byte order mark U+FEFF or a zero-width
    space U+200B, unassigned, private use, surrogate) or Z* other than an ordinary space (no-break space, line and paragraph separators), the
    replacement character U+FFFD (an invalid UTF-8 byte decoded with errors=replace), a backslash (not a path separator on any platform git
    indexes for), and a spelling that is not NFC (a decomposed spelling never equals the composed one git normally records)."""
    if r != r.strip() or any(ord(c) < 32 or ord(c) == 127 for c in r):
        return True
    for c in r:
        cat = unicodedata.category(c)
        if c == "\\" or c == "\ufffd" or cat[0] == "C" or (cat[0] == "Z" and c != " "):
            return True
    if unicodedata.normalize("NFC", r) != r:
        return True
    comps = r.rstrip("/").split("/")
    return r.startswith("/") or any(c in ("", ".", "..") for c in comps)

SECRET = re.compile(r"(^\.env($|\.))|(\.(jks|keystore|p12|pfx)$)|secret", re.I)
def p3():
    cmd = "scope of codegraph files --json (captured file)"
    if indexed is None:
        return row("P3", "FAIL", cmd=cmd, reason="no_index_file_list")
    why, skipped = [], []
    secret = sorted(p for p in indexed
                    if SECRET.search(os.path.basename(p)) and os.path.basename(p) not in (".env.example", ".env.sample"))
    if secret:
        why.append("secret_class_files_indexed")
    third, third_bad = [], []
    if "--third-party-roots" in opts:
        roots = load_lines("third_party_roots", opts["--third-party-roots"])
        if roots is None:
            why.append("third_party_roots_unreadable")
        else:
            bad_roots = [r for r in roots if noncanonical_root(r)]
            if bad_roots:
                why.append("third_party_root_not_canonical")
                third_bad = [r for r in bad_roots][:50]
            else:
                third = sorted(p for p in indexed if any(p == r.rstrip("/") or p.startswith(r.rstrip("/") + "/") for r in roots))
                if third:
                    why.append("third_party_files_indexed")
    else:
        skipped.append("third_party_roots_not_supplied")
    own_missing = []
    if "--own-org-roots" in opts:
        roots = load_lines("own_org_roots", opts["--own-org-roots"])
        if roots is None:
            why.append("own_org_roots_unreadable")
        else:
            own_missing = sorted(r for r in roots
                                 if not any(p.startswith(r.rstrip("/") + "/") for p in indexed))
            if own_missing:
                why.append("own_org_root_without_indexed_file")
    else:
        skipped.append("own_org_roots_not_supplied")
    return row("P3", "FAIL" if why else "PASS", cmd=cmd, reasons=why, not_checked=skipped, secret_hits=secret[:50],
               third_party_hits=third[:50], third_party_roots_refused=third_bad, own_org_roots_empty=own_missing)

# ---- Lumen: P-Lumen (evidence source), P7 (complete + fresh) ---------------------------------------------------------
lumen_rows = []
def lumen():
    lumen_status = None
    if "--lumen-status" in opts:
        raw = read_raw("lumen_status", opts["--lumen-status"])
        if raw is None:
            lumen_rows.append(row("P-Lumen", "FAIL", reason="status_file_unreadable"))
        else:
            lumen_rows.append(row("P-Lumen", "PASS", source="index_status_capture"))
            lumen_status = raw.decode("utf-8", "replace")
    elif "--lumen-bin" in opts:
        b = opts["--lumen-bin"]
        if not (os.path.isfile(b) and os.access(b, os.X_OK)):
            lumen_rows.append(row("P-Lumen", "FAIL", reason="lumen_bin_not_executable_or_absent", path=b))
        else:
            # a `lumen search` probe runs EnsureFresh and so WRITES the Lumen index (docs/02 section 3): refused here.
            lumen_rows.append(row("P-Lumen", "FAIL", reason="probe_not_run_read_only_gate_needs_status_capture", path=b))
    else:
        lumen_rows.append(row("P-Lumen", "FAIL", reason="no_status_capture_and_no_probe"))  # MUT:plumen-nostatus
    lumen_rows.append(row("P6", "SKIP", reason="embed_call_needs_a_live_embedder_not_an_input_of_this_read_only_gate"))
    if lumen_status is None:
        lumen_rows.append(row("P7", "SKIP", reason="no_status_capture"))
        return
    cmd = "index_status text (captured file)"
    m = {}
    for key, rx in (("files", r"Files:\s*(\d+)"), ("indexed", r"Indexed:\s*(\d+)"), ("chunks", r"Chunks:\s*(\d+)"),
                    ("stale", r"Stale:\s*(yes|no)\b"), ("captured", r"Captured:\s*(\S+)")):
        mm = re.search(rx, lumen_status, re.I)
        m[key] = mm.group(1) if mm else None
    absent = sorted(k for k, v in m.items() if v is None)
    if absent:
        lumen_rows.append(row("P7", "FAIL", cmd=cmd, reason="status_field_absent", absent=absent))
        return
    why = []
    if m["files"] != m["indexed"]:
        why.append("files_ne_indexed")
    if int(m["chunks"]) <= 0:
        why.append("chunks_zero")
    if m["stale"].lower() != "no":  # MUT:p7-stale
        why.append("stale")
    try:
        datetime.datetime.fromisoformat(m["captured"].replace("Z", "+00:00"))
    except ValueError:
        why.append("captured_time_unparseable")
    lumen_rows.append(row("P7", "FAIL" if why else "PASS", cmd=cmd, reasons=why, files=int(m["files"]),
                          indexed=int(m["indexed"]), chunks=int(m["chunks"]), stale=m["stale"].lower(),
                          captured=m["captured"]))

# ---- P8 scope parity -------------------------------------------------------------------------------------------------
def p8():
    if "--lumen-files" not in opts:
        return {"id": "P8", "verdict": "SKIP", "reason": "no_lumen_files_list_supplied"}
    ll = load_lines("lumen_files", opts["--lumen-files"])
    if ll is None or indexed is None:
        return {"id": "P8", "verdict": "FAIL", "reason": "input_unreadable"}
    ext = lambda p: os.path.splitext(p)[1].lstrip(".").lower()
    lset = set(p for p in ll if ext(p) not in unind)
    cset = set(p for p in indexed if ext(p) not in unind)
    lonly, conly = sorted(lset - cset), sorted(cset - lset)
    base = max(len(cset), 1)
    pct = (len(lonly) + len(conly)) * 100.0 / base
    ok = pct <= tol  # MUT:p8-tol
    return {"id": "P8", "verdict": "PASS" if ok else "FAIL", "cmd": "set parity of captured file lists",
            "tolerance_pct": tol, "diff_pct": round(pct, 4), "codegraph_files": len(cset), "lumen_files": len(lset),
            "lumen_only_count": len(lonly), "codegraph_only_count": len(conly), "lumen_only": lonly[:50],
            "codegraph_only": conly[:50], "unindexable_ext_removed_from_both": sorted(unind)}

p1row, status = p1()
cg_rows = [p1row, p2(), p3(),
           row("P4", "SKIP", reason="golden_questions_need_live_index_queries_not_an_input_of_this_read_only_gate"),
           row("P5", "SKIP", reason="freshness_needs_lastIndexed_and_git_log_not_an_input_of_this_read_only_gate")]
lumen()
parity = p8()
all_rows = cg_rows + lumen_rows + [parity]
verdict = "FAIL" if any(r["verdict"] == "FAIL" for r in all_rows) else "PASS"  # MUT:verdict
doc = {"schema": "index-health/1", "verdict": verdict,
       "complete": not any(r["verdict"] == "SKIP" for r in all_rows),
       "skipped_proofs": [r["id"] for r in all_rows if r["verdict"] == "SKIP"],
       "inputs": inputs,
       "codegraph": {"status": status, "proofs": cg_rows}, "lumen": {"proofs": lumen_rows}, "parity": parity}
out = opts["--out"]
d = os.path.dirname(os.path.abspath(out))
try:
    os.makedirs(d, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=d, prefix=".index-health.")
    with os.fdopen(fd, "w") as fh:
        json.dump(doc, fh, indent=2, sort_keys=True)
        fh.write("\n")
    os.replace(tmp, out)
except OSError as e:
    sys.stderr.write("index_health: cannot write %s: %s\n" % (out, e))
    sys.exit(2)
sys.exit(0 if verdict == "PASS" else 1)
PY
