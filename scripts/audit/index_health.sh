#!/usr/bin/env bash
# index_health.sh - T021 (WP-02). Read-only index health gate for proofs P1..P8 (docs/02 section 4.1, 4.2).
#
# Purpose   Judge CAPTURED outputs of the two code indexes (CodeGraph status/files, Lumen index_status text) and write
#           `index-health/1` JSON. It never runs an index writer and never opens an index database: every input is a file the
#           caller captured beforehand (docs/scripts/index_health.md).
# Usage     index_health.sh --out FILE --scope FILE --cg-status FILE --cg-files FILE --tracked FILE --needle-pos PATH --needle-neg PATH
#                           [--lumen-status FILE | --lumen-bin FILE] [--lumen-files FILE] [--parity-tolerance-pct N]
#                           [--lumen-unindexable-ext CSV] [--third-party-roots FILE] [--own-org-roots FILE]
#                           [--captured-not-before ISO] [--lumenignore FILE --lumenignore-fresh FILE] [--drift-report FILE]
#                           [--live-proofs FILE --live-policy FILE --evidence-ledger FILE] [--repo DIR] [--lumen-json FILE] [--baseline FILE]
#           --captured-not-before ISO is REQUIRED whenever --lumen-status or --live-proofs is given (round 9, review N7).
#           Round 10 (structural, reviews WP-02-T028-r3): every live proof's verdict is DERIVED from its evidence file (design: specs/001-full-project-audit-remediation/
#           evidence/wp02/fix-r8-design.md): --live-proofs needs --live-policy (closed absolute program allow-list, freshness window, thresholds) and
#           --evidence-ledger (a TRACKED SHA256SUMS file of --repo that lists the evidence); --drift-report is bound to the real HEAD, scope, lumen json
#           (--lumen-json) and .lumenignore; --baseline (default: the constitution scope_baseline.txt of --repo) is applied to P3 directly.
# Exit      0 = every proof ran and none FAILs (verdict PASS), 1 = at least one proof FAIL (verdict FAIL),
#           3 = no FAIL but at least one proof SKIPped (verdict INCOMPLETE, blocked-unavailable with the reasons listed;
#           never a pass, 11.4.135), 2 = usage error.
# Side effects  writes only FILE (atomic rename in its directory). Needs python3.
# Contract  scripts/audit/tests/test_index_health.sh (13 checks). A required field that is absent is a FAIL, never read as zero.
set -u
command -v python3 >/dev/null 2>&1 || { echo "index_health: python3 required" >&2; exit 2; }
exec python3 - "$@" <<'PY'
import datetime, fnmatch, hashlib, json, math, os, re, subprocess, sys, tempfile, unicodedata

def usage(msg):
    sys.stderr.write("index_health: usage: %s\n" % msg)
    sys.exit(2)

VALUE_OPTS = ("--out", "--cg-status", "--cg-files", "--tracked", "--needle-pos", "--needle-neg", "--lumen-status",
              "--lumen-bin", "--lumen-files", "--parity-tolerance-pct", "--lumen-unindexable-ext",
              "--third-party-roots", "--own-org-roots", "--scope", "--captured-not-before", "--lumenignore",
              "--lumenignore-fresh", "--drift-report", "--live-proofs", "--repo", "--live-policy", "--evidence-ledger", "--lumen-json", "--baseline")
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
for req in ("--out", "--scope", "--cg-status", "--cg-files", "--tracked", "--needle-pos", "--needle-neg"):
    if req not in opts:  # MUT:require-out
        usage("%s is required" % req)
try:
    tol = float(opts.get("--parity-tolerance-pct", "0"))
    if tol < 0 or tol != tol:
        raise ValueError
except ValueError:
    usage("--parity-tolerance-pct must be a number >= 0")
cnb = None
if "--captured-not-before" in opts:
    try:
        cnb = datetime.datetime.fromisoformat(opts["--captured-not-before"].replace("Z", "+00:00"))
        if cnb.tzinfo is None:
            raise ValueError
    except ValueError:
        usage("--captured-not-before must be an ISO time with a zone")
if ("--lumen-status" in opts or "--live-proofs" in opts) and cnb is None:  # MUT:cnb-required
    usage("--captured-not-before is required when --lumen-status or --live-proofs is given (a capture without an age bound can be of any date)")
if "--live-proofs" in opts and "--live-policy" not in opts:  # MUT:policy-required
    usage("--live-policy is required with --live-proofs (the closed program allow-list, freshness window and thresholds are DATA, not defaults)")
if ("--live-proofs" in opts or "--drift-report" in opts) and "--evidence-ledger" not in opts:  # MUT:ledger-required
    usage("--evidence-ledger is required with --live-proofs or --drift-report (evidence must be bound to a recorded, tracked artifact)")
NOW = datetime.datetime.now(datetime.timezone.utc)
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

# ---- repository, policy, ledger: the trust boundary of the derived proofs (design fix-r8-design.md sections 2 and 3) -------------------------
def git(*a):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_") or k in ("GIT_CONFIG_NOSYSTEM",)}
    return subprocess.run(["git", "-C", REPO] + list(a), capture_output=True, env=env)

def resolve_repo():
    if "--repo" in opts:
        return os.path.abspath(opts["--repo"])
    r = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True)
    return r.stdout.decode("utf-8", "replace").strip() if r.returncode == 0 and r.stdout.strip() else os.getcwd()
REPO = resolve_repo()
SKEW = 300

def load_policy():
    """--live-policy FILE (live-policy/1): the closed absolute program allow-list per proof, the freshness window and the thresholds are DATA of
    the project, hash-recorded in `inputs`. Returns (policy, None) or (None, reason)."""
    if "--live-policy" not in opts:
        return None, "no_live_policy"
    try:
        import yaml
    except ImportError:
        usage("PyYAML is required to read --live-policy")
    raw = read_raw("live_policy", opts["--live-policy"])
    if raw is None:
        return None, "live_policy_unusable"
    try:
        d = yaml.safe_load(raw.decode("utf-8"))
    except (yaml.YAMLError, UnicodeDecodeError):
        return None, "live_policy_unusable"
    try:
        ok = (isinstance(d, dict) and d.get("schema") == "live-policy/1" and isinstance(d.get("programs"), dict)
              and all(isinstance(d["programs"].get(k), list) and d["programs"][k] and all(isinstance(x, str) and x for x in d["programs"][k]) for k in ("P4", "P5", "P6"))
              and isinstance(d.get("fallback_programs"), list) and all(isinstance(x, str) and os.path.isabs(x) for x in d["fallback_programs"])
              and isinstance(d.get("thresholds"), dict))
        th = d["thresholds"]
        pol = {"max_age": datetime.timedelta(hours=float(d.get("max_age_hours", 72))), "skew": float(d.get("clock_skew_seconds", SKEW)),
               "programs": d["programs"], "fallback": d["fallback_programs"],
               "min_queries": int(th["p4_min_queries"]), "min_recall": float(th["p4_min_recall"]), "dims": int(th["p6_expected_dims"])}
        if not ok or pol["max_age"] <= datetime.timedelta(0) or pol["skew"] < 0:
            return None, "live_policy_unusable"
    except (KeyError, TypeError, ValueError):
        return None, "live_policy_unusable"
    return pol, None
POLICY, POLICY_ERR = load_policy()
if POLICY is not None and cnb is not None:
    if cnb < NOW - POLICY["max_age"]:  # MUT:cnb-age-bound
        usage("--captured-not-before is older than the policy max_age_hours: a caller may not widen the freshness window")
    if cnb > NOW + datetime.timedelta(seconds=POLICY["skew"]):  # MUT:cnb-future
        usage("--captured-not-before lies in the future")
SKEW_TD = datetime.timedelta(seconds=POLICY["skew"] if POLICY else SKEW)

def parse_time(v):
    try:
        t = datetime.datetime.fromisoformat(v.replace("Z", "+00:00"))
    except (ValueError, AttributeError):
        return None
    return t if t.tzinfo is not None else None

def is_zero_digest(h):
    return isinstance(h, str) and set(h) == {"0"}

def tracked_clean(path):
    """True when `path` (absolute, inside REPO) is tracked and its working-tree bytes equal HEAD: a recorded artifact, not a file somebody typed."""
    rel = os.path.relpath(os.path.abspath(path), REPO)
    if rel.startswith("..") or os.path.isabs(rel):
        return False
    if git("ls-files", "--error-unmatch", "--", rel).returncode != 0:
        return False
    if git("cat-file", "-e", "HEAD:" + rel).returncode != 0:
        return False
    return git("diff", "--quiet", "HEAD", "--", rel).returncode == 0

LEDGER = None   # (dir, {relpath: set(sha)}) or a reason string
def ledger():
    global LEDGER
    if LEDGER is not None:
        return LEDGER
    lp = os.path.abspath(opts["--evidence-ledger"]) if "--evidence-ledger" in opts else None
    raw = read_raw("evidence_ledger", lp) if lp else None
    if lp is None or raw is None:
        LEDGER = "ledger_not_tracked"
    elif not tracked_clean(lp):  # MUT:ledger-tracked
        LEDGER = "ledger_not_tracked"
    else:
        m = {}
        for line in raw.decode("utf-8", "replace").split("\n"):
            mm = re.fullmatch(r"([0-9a-f]{64}) [ *](.+)", line)
            if mm:
                m.setdefault(os.path.normpath(mm.group(2)), set()).add(mm.group(1))
        LEDGER = (os.path.dirname(lp), m)
    return LEDGER

def bind_artifact(path, key):
    """An artifact the verdict depends on (live evidence, drift report) must be tracked and unmodified in REPO and listed, with its own sha256, in the
    tracked evidence ledger. Returns (bytes, None) or (None, reason)."""
    L = ledger()
    if isinstance(L, str):
        return None, L
    ap = path if os.path.isabs(path) else os.path.join(REPO, path)
    data = read_raw(key, ap)
    if data is None:
        return None, "evidence_file_unreadable"
    if not data.strip():
        return None, "evidence_empty"
    if not tracked_clean(ap):  # MUT:evidence-tracked
        return None, "evidence_not_tracked"
    rel = os.path.normpath(os.path.relpath(ap, L[0]))
    if rel.startswith("..") or rel not in L[1]:  # MUT:evidence-in-ledger
        return None, "evidence_not_in_ledger"
    if hashlib.sha256(data).hexdigest() not in L[1][rel] or len(L[1][rel]) != 1:  # MUT:ledger-hash
        return None, "ledger_hash_mismatch"
    return data, None

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

def root_hits(r):
    """True when the root names an indexed path: the path equals it or lies below it (exact spelling)."""
    q = r.rstrip("/")
    return bool(indexed) and any(p == q or p.startswith(q + "/") for p in indexed)

def root_case_mismatch(r):
    """A root that matches NO indexed path exactly but matches one when case is ignored: a case variant of a real root (`Third` for `third`) reads
    as clean in exactly the way an encoding mismatch does (WF7 M-6a). A root that matches nothing in any case is legitimate (the tree is not indexed)."""
    q = r.rstrip("/")
    if root_hits(r) or not indexed:
        return False
    ql = q.casefold()
    return any(p.casefold() == ql or p.casefold().startswith(ql + "/") for p in indexed)

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
    if unicodedata.normalize("NFC", r) != r and not root_hits(r):
        # a decomposed spelling never equals the composed one git normally records; but a root that names a path git REALLY indexed in that spelling
        # (a macOS-made commit) is a normal hit, not a refusal (WF7 M-6b)
        return True
    comps = r.rstrip("/").split("/")
    return r.startswith("/") or any(c in ("", ".", "..") for c in comps)

PROBE_DEPTHS = ("", "a/", "a/b/")   # MUT:probe-depths  directory prefixes of depth 0, 1 and 2
def baseline_probes(pat):
    """Members a scope baseline pattern excludes, built from the pattern text, at EVERY depth an unanchored pattern applies to (review R3-3: a class that
    misses root-level members must be refused by the tool itself, not only by a test of one depth): a leading '/' anchors it at the root (depth 0 only),
    '**/' and a trailing '/' are removed, '*' becomes the word probe, a directory pattern gets a file below it."""
    anchored = pat.startswith("/")
    p = pat.lstrip("/")
    while p.startswith("**/"):  # MUT:strip-doublestar
        p = p[3:]
    is_dir = p.endswith("/")
    p = p.rstrip("/").replace("*", "probe")
    tail = p + ("/probe.py" if is_dir else "")
    return [tail] if anchored else [d + tail for d in PROBE_DEPTHS]

SCOPE_NEGATIONS = set()
def load_forbidden_classes():
    """The forbidden classes come from scope.yaml `forbidden_classes` (ONE class source with the renderer, 11.4.275(B)); each carries a positive and a
    negative fixture, and the class is self-tested here before it judges anything (a class that misses its own positive, or hits its own negative, is a
    blind or over-broad instrument). Round 9 (review N1, 11.4.273(f)): every non-negated pattern of the scope's OWN baseline_excludes (all classes:
    build outputs, caches, secrets, QA corpora) must be covered by at least one class, probed with a path built from the pattern: a pattern the scope
    excludes but P3 cannot judge is a second class source that disagrees, reported as `baseline_pattern_uncovered:<pattern>`. Round 9 (review N4):
    classes are compiled case-insensitively because the matcher that applies codegraph.json `exclude` (the runner's `ignore` library 7.0.5, default
    ignorecase true) is case-insensitive (measured, test_index_health_fix_r7.sh); the Lumen matcher is UNCONFIRMED (P3 judges the CodeGraph file list).
    Returns (classes, problems)."""
    try:
        import yaml
    except ImportError:
        usage("PyYAML is required to read --scope")
    raw = read_raw("scope", opts["--scope"])
    if raw is None:
        return [], ["scope_unreadable"]
    try:
        data = yaml.safe_load(raw.decode("utf-8"))
    except (yaml.YAMLError, UnicodeDecodeError):
        return [], ["scope_not_yaml"]
    fc = data.get("forbidden_classes") if isinstance(data, dict) else None
    if not isinstance(fc, list) or not fc:  # MUT:no-classes-blind
        return [], ["no_forbidden_classes_blind"]
    out, probs = [], []
    for c in fc:
        if not (isinstance(c, dict) and all(isinstance(c.get(k), str) and c.get(k) for k in ("name", "regex", "positive", "negative"))):
            probs.append("forbidden_class_malformed")
            continue
        try:
            rx = re.compile(c["regex"], re.IGNORECASE)
        except re.error:
            probs.append("forbidden_class_regex_invalid:" + c["name"])
            continue
        if not rx.search(c["positive"]) or rx.search(c["negative"]):  # MUT:class-selftest
            probs.append("forbidden_class_selftest_failed:" + c["name"])
            continue
        out.append((c["name"], rx))
    be = data.get("baseline_excludes")
    global SCOPE_NEGATIONS
    SCOPE_NEGATIONS = set(x[1:] for v in (be.values() if isinstance(be, dict) else []) if isinstance(v, list) for x in v if isinstance(x, str) and x.startswith("!"))
    if be is not None and not isinstance(be, dict):
        probs.append("baseline_excludes_malformed")
    elif be is not None:
        for cname, pats in sorted(be.items(), key=lambda kv: str(kv[0])):
            for pat in pats if isinstance(pats, list) else []:
                if not isinstance(pat, str) or not pat.strip() or pat.startswith("!"):
                    continue
                if not all(any(rx.search(pr) for _, rx in out) for pr in baseline_probes(pat.strip())):  # MUT:baseline-coverage
                    probs.append("baseline_pattern_uncovered:" + pat)
    return out, probs

BASELINE_KINDS = ("secret", "exclude"); BASELINE_MATCHERS = ("exact", "seg", "segglob", "dir", "topdir")
def load_baseline():
    """The constitution scope_baseline.txt (--baseline, default <repo>/constitution/scripts/codegraph/scope_baseline.txt when it exists) is
    applied DIRECTLY to the indexed paths: one class source for the constitution list (review R3-4). Returns (rows, problems, not_checked)."""
    explicit = "--baseline" in opts
    path = opts["--baseline"] if explicit else os.path.join(REPO, "submodules", "constitution", "scripts", "codegraph", "scope_baseline.txt")
    if not explicit and not os.path.isfile(path):
        return [], [], ["baseline_not_supplied_and_default_absent"]
    raw = read_raw("baseline", path)
    if raw is None:
        return [], ["baseline_unreadable"], []
    rows, probs = [], []
    for line in raw.decode("utf-8", "replace").split("\n"):
        t = line.strip()
        if not t or t.startswith("#"):
            continue
        parts = t.split()
        if len(parts) != 3 or parts[1] not in BASELINE_KINDS or ":" not in parts[2] or parts[2].split(":", 1)[0] not in BASELINE_MATCHERS or not parts[2].split(":", 1)[1]:
            probs.append("baseline_malformed")
            continue
        m, v = parts[2].split(":", 1)
        rows.append((parts[0], m, v))
    if not rows and not probs:
        probs.append("baseline_empty_blind")
    return rows, sorted(set(probs)), []

def baseline_hit(row_, p):
    cls, m, v = row_
    pl, vl = p.lower(), v.lower()  # MUT:baseline-case
    comps = pl.split("/")
    if m == "exact":
        return pl == vl  # MUT:baseline-exact
    if m == "seg":
        return vl in comps  # MUT:baseline-seg
    if m == "segglob":
        # a glob the scope itself re-includes for directories (`!<glob>/` in baseline_excludes) is judged on the file name only (documented narrowing)
        pool = comps[-1:] if (v + "/") in SCOPE_NEGATIONS else comps  # MUT:baseline-narrowing
        return any(fnmatch.fnmatchcase(c, vl) for c in pool)
    if m == "dir":
        return vl in comps[:-1]  # MUT:baseline-dir
    if m == "topdir":
        return len(comps) > 1 and comps[0] == vl  # MUT:baseline-topdir
    return False

def p3():
    cmd = "scope of codegraph files --json (captured file)"
    if indexed is None:
        return row("P3", "FAIL", cmd=cmd, reason="no_index_file_list")
    why, skipped = [], []
    classes, probs = load_forbidden_classes()
    if probs:
        why.append("forbidden_classes_unusable")
    brows, bprobs, bskip = load_baseline()
    why += bprobs
    skipped += bskip
    forbidden, bhits = {}, {}
    for p in sorted(indexed):
        if os.path.basename(p) in (".env.example", ".env.sample"):  # MUT:exemption-exact
            continue
        for name, rx in classes:
            if rx.search(p):
                forbidden.setdefault(name, []).append(p)
        for br in brows:
            if baseline_hit(br, p):  # MUT:baseline-predicate
                bhits.setdefault(br[0], []).append(p)
    secret = sorted(set(p for v in forbidden.values() for p in v))
    if secret:
        why.append("forbidden_class_files_indexed")
    if bhits:
        why.append("baseline_class_files_indexed")
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
                case_bad = [r for r in roots if root_case_mismatch(r)]
                if case_bad:
                    why.append("third_party_root_case_mismatch")
                    third_bad = case_bad[:50]
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
                                 if not any(p.startswith(r.rstrip("/") + "/") for p in indexed))  # MUT:ownorg-boundary
            if own_missing:
                why.append("own_org_root_without_indexed_file")
    else:
        skipped.append("own_org_roots_not_supplied")
    return row("P3", "FAIL" if why else "PASS", cmd=cmd, reasons=why, not_checked=skipped, forbidden_hits=secret[:50], forbidden_by_class={k: v[:20] for k, v in sorted(forbidden.items())}, baseline_hits_by_class={k: sorted(set(v))[:20] for k, v in sorted(bhits.items())},
               forbidden_class_names=[n for n, _ in classes], forbidden_class_problems=probs,
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
    lumen_rows.append(live_or_skip("P6"))
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
        cap = datetime.datetime.fromisoformat(m["captured"].replace("Z", "+00:00"))
        if cap.tzinfo is None:
            cap = cap.replace(tzinfo=datetime.timezone.utc)
        if cnb is not None and cap < cnb:  # MUT:p7-notbefore
            why.append("captured_before_required_time")
        if cap > NOW + SKEW_TD:  # MUT:p7-future
            why.append("captured_in_future")
    except ValueError:
        why.append("captured_time_unparseable")  # MUT:p7-captured
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

# ---- P4/P5/P6 live proofs: the verdict is DERIVED from the evidence file (design sections 2, 4, 5) ---------------------------------------
SKIP_WHY = {"P4": "golden_questions_need_live_index_queries_not_an_input_of_this_read_only_gate",
            "P5": "freshness_needs_lastIndexed_and_git_log_not_an_input_of_this_read_only_gate",
            "P6": "embed_call_needs_a_live_embedder_not_an_input_of_this_read_only_gate"}
live = {}
LIVE_KINDS = ("live", "fallback")
EVIDENCE_SCHEMA = "live-evidence/1"
REPEATABLE = ("q",)

def program_regex(pat):
    pat = pat.replace("$HOME", os.path.expanduser("~")).replace("$REPO", REPO)
    return re.compile("/".join("[^/]+" if seg == "*" else re.escape(seg) for seg in pat.split("/")))

def check_program(prog, patterns):
    """The program that produced the evidence: an absolute, normalised path literally matching an entry of the closed allow-list (a `*` stands for one
    path segment), existing as an executable file. A basename is never enough (review R3-1: /tmp/evil/codegraph)."""
    if not isinstance(prog, str) or not os.path.isabs(prog):  # MUT:program-absolute
        return "program_not_absolute"
    if os.path.normpath(prog) != prog or not any(program_regex(x).fullmatch(prog) for x in patterns):  # MUT:program-allowlist
        return "program_not_allowed"
    if not (os.path.isfile(prog) and os.access(prog, os.X_OK)):  # MUT:program-exists
        return "program_not_executable"
    return None

def parse_evidence(data):
    """key: value lines ('#' comments and blank lines ignored). Returns (dict key -> str, or list for REPEATABLE keys, None) or (None, reason)."""
    d = {}
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        return None, "evidence_schema_invalid"
    for line in text.split("\n"):
        t = line.strip()
        if not t or t.startswith("#"):
            continue
        m = re.fullmatch(r"([a-z][a-z0-9_]*):\s*(.*)", t)
        if not m:
            return None, "evidence_schema_invalid"
        k, v = m.group(1), m.group(2)
        if k in REPEATABLE:
            d.setdefault(k, []).append(v)
        elif k in d:  # MUT:evidence-dup
            return None, "evidence_field_duplicate:" + k
        else:
            d[k] = v
    if d.get("schema") != EVIDENCE_SCHEMA:  # MUT:evidence-schema
        return None, "evidence_schema_invalid"
    return d, None

def need(d, *keys):
    miss = [k for k in keys if not (isinstance(d.get(k), str) and d[k] != "")]
    return ["evidence_field_missing:" + k for k in miss]

def derive_p4(d):
    why = need(d, "queries", "correct", "recall")
    rows = d.get("q") or []
    parsed = []
    for r in rows:
        kv = dict(x.split("=", 1) for x in r.split() if "=" in x)
        if not (kv.get("id") and kv.get("expect") in ("answerable", "unanswerable") and kv.get("result") in ("correct", "wrong", "refused")):  # MUT:p4-row-strict
            return ["p4_row_malformed"]
        parsed.append(kv)
    if why:
        return why
    total = len(parsed)
    answerable = [r for r in parsed if r["expect"] == "answerable"]
    correct = [r for r in answerable if r["result"] == "correct"]
    controls = [r for r in parsed if r["expect"] == "unanswerable"]
    recall = (len(correct) / len(answerable)) if answerable else 0.0
    try:
        claim = (int(d["queries"]), int(d["correct"]), float(d["recall"]))
    except ValueError:
        return ["p4_summary_mismatch"]
    if claim[0] != total or claim[1] != len(correct) or not math.isfinite(claim[2]) or abs(claim[2] - recall) > 0.0005:  # MUT:p4-summary
        why.append("p4_summary_mismatch")
    if total < POLICY["min_queries"]:  # MUT:p4-min-queries
        why.append("p4_too_few_queries")
    if not controls or any(r["result"] != "refused" for r in controls):  # MUT:p4-control
        why.append("p4_control_not_refused")
    if recall + 1e-12 < POLICY["min_recall"]:  # MUT:p4-recall
        why.append("p4_recall_below_threshold")
    return why

def derive_p5(d):
    why = need(d, "last_indexed", "last_commit")
    if why:
        return why
    li, lc = parse_time(d["last_indexed"]), parse_time(d["last_commit"])
    if li is None or lc is None:  # MUT:p5-time
        return ["p5_time_unparseable"]
    if lc > NOW + SKEW_TD:  # MUT:p5-commit-future
        why.append("p5_commit_in_future")
    if li > NOW + SKEW_TD:
        why.append("p5_time_in_future")
    if li < lc:  # MUT:p5-order
        why.append("p5_index_older_than_commit")
    return why

def derive_p6(d):
    why = need(d, "embed_dims", "embed_vectors", "embed_norm")
    if why:
        return why
    if not (re.fullmatch(r"[0-9]+", d["embed_dims"]) and re.fullmatch(r"[0-9]+", d["embed_vectors"])):
        return ["p6_field_not_number"]
    try:
        norm = float(d["embed_norm"])
    except ValueError:
        return ["p6_field_not_number"]
    if int(d["embed_dims"]) != POLICY["dims"]:  # MUT:p6-dims
        why.append("p6_dims_mismatch")
    if int(d["embed_vectors"]) < 1:  # MUT:p6-vectors
        why.append("p6_no_vectors")
    if not math.isfinite(norm) or norm <= 0:  # MUT:p6-norm
        why.append("p6_norm_not_positive")
    return why
DERIVE = {"P4": derive_p4, "P5": derive_p5, "P6": derive_p6}

def verify_live(k, e):
    """One --live-proofs entry. The entry is a POINTER to evidence, never a verdict: every field is derived from the evidence file, validated against a
    real value, or rejected (design section 5). The evidence is bound to a tracked, ledger-listed artifact; the program must be on the closed absolute
    allow-list; the time window is [max(--captured-not-before, now - max_age), now + skew]; the verdict is computed by the parser of that proof. An
    entry `verdict` is checked against the derived one, never used."""
    ok = (isinstance(e, dict) and e.get("kind") in LIVE_KINDS and e.get("verdict", "PASS") in ("PASS", "FAIL")
          and all(isinstance(e.get(f), str) and e[f] for f in ("cmd", "evidence_path", "evidence_sha256", "captured_at"))
          and re.fullmatch(r"[0-9a-f]{64}", e["evidence_sha256"]))
    if not ok:
        return row(k, "FAIL", reason="live_proof_entry_malformed")
    if POLICY is None:
        return row(k, "FAIL", reason=POLICY_ERR or "live_policy_unusable")
    if is_zero_digest(e["evidence_sha256"]):  # MUT:zero-digest
        return row(k, "FAIL", reason="evidence_hash_zero", evidence_path=e["evidence_path"])
    data, why = bind_artifact(e["evidence_path"], "live_evidence_" + k)
    if why:
        return row(k, "FAIL", reason=why, evidence_path=e["evidence_path"])
    if hashlib.sha256(data).hexdigest() != e["evidence_sha256"]:  # MUT:live-hash
        return row(k, "FAIL", reason="evidence_hash_mismatch", evidence_path=e["evidence_path"])
    d, why = parse_evidence(data)
    if why:
        return row(k, "FAIL", reason=why, evidence_path=e["evidence_path"])
    reasons = need(d, "proof", "program", "argv", "exit", "captured_at", "scope_sha256")
    if reasons:
        return row(k, "FAIL", reasons=reasons, evidence_path=e["evidence_path"])
    if d["proof"] != k:  # MUT:evidence-proof
        return row(k, "FAIL", reason="evidence_proof_mismatch", evidence_path=e["evidence_path"])
    if d["exit"] != "0":  # MUT:evidence-exit
        return row(k, "FAIL", reason="evidence_exit_nonzero", evidence_path=e["evidence_path"])
    allowed = POLICY["programs"][k] if e["kind"] == "live" else POLICY["fallback"]
    why = check_program(d["program"], allowed)
    if why:
        return row(k, "FAIL", reason=why, program=d["program"])
    if e["cmd"] != d["program"] + " " + d["argv"]:  # MUT:entry-cmd
        return row(k, "FAIL", reason="evidence_cmd_mismatch", cmd=e["cmd"])
    read_raw("scope", opts["--scope"])
    if d["scope_sha256"] != (inputs.get("scope") or {}).get("sha256"):  # MUT:evidence-scope
        return row(k, "FAIL", reason="evidence_scope_mismatch", evidence_path=e["evidence_path"])
    when = parse_time(d["captured_at"])
    if when is None:
        return row(k, "FAIL", reason="captured_at_unparseable", evidence_path=e["evidence_path"])
    if e["captured_at"] != d["captured_at"]:  # MUT:entry-captured-at
        return row(k, "FAIL", reason="captured_at_mismatch", evidence_path=e["evidence_path"])
    if when > NOW + SKEW_TD:  # MUT:live-future
        return row(k, "FAIL", reason="captured_in_future", captured_at=d["captured_at"])
    if when < cnb:  # MUT:live-fresh  (cnb >= now - max_age is enforced at startup, so the window floor is cnb)
        return row(k, "FAIL", reason="live_proof_stale", captured_at=d["captured_at"])
    common = dict(program=d["program"], evidence_path=e["evidence_path"], evidence_sha256=e["evidence_sha256"], captured_at=d["captured_at"], kind=e["kind"])
    if e["kind"] == "fallback":  # MUT:live-fallback
        return row(k, "SKIP", reason="fallback_record_is_not_a_live_proof", **common)
    reasons = DERIVE[k](d)
    verdict = "FAIL" if reasons else "PASS"
    if e.get("verdict") is not None and e["verdict"] != verdict:  # MUT:entry-verdict
        reasons = reasons + ["entry_verdict_disagrees_with_evidence"]
        verdict = "FAIL"
    return row(k, verdict, reasons=reasons, **common)

def load_live():
    """--live-proofs FILE: {"P4": {entry}, ...}. Absent proof = SKIP (blocked-unavailable); a malformed or unverifiable entry is a FAIL, never a pass."""
    if "--live-proofs" not in opts:
        return
    raw = read_raw("live_proofs", opts["--live-proofs"])
    try:
        d = json.loads(raw.decode("utf-8")) if raw is not None else None
    except (ValueError, UnicodeDecodeError):
        d = None
    if not isinstance(d, dict):
        for k in SKIP_WHY:
            live[k] = row(k, "FAIL", reason="live_proofs_file_unusable")
        return
    for k in SKIP_WHY:
        if k in d:
            live[k] = verify_live(k, d[k])

def live_or_skip(k):
    return live.get(k) or row(k, "SKIP", reason=SKIP_WHY[k])

def p_lumenignore():
    if ("--lumenignore" in opts) != ("--lumenignore-fresh" in opts):
        usage("--lumenignore and --lumenignore-fresh are given together")
    if "--lumenignore" not in opts:
        return row("P-LumenIgnore", "SKIP", reason="no_lumenignore_pair_supplied")
    a, b = read_raw("lumenignore", opts["--lumenignore"]), read_raw("lumenignore_fresh", opts["--lumenignore-fresh"])
    if a is None or b is None:
        return row("P-LumenIgnore", "FAIL", reason="input_unreadable")
    if not b.strip():
        return row("P-LumenIgnore", "FAIL", reason="fresh_render_empty_blind")
    return row("P-LumenIgnore", "PASS" if a == b else "FAIL", cmd="committed .lumenignore vs gen_lumenignore.py re-render",
               reasons=[] if a == b else ["lumenignore_stale_vs_scope"])  # MUT:lumenignore-cmp

DRIFT_REQUIRED = ("scope_render_check", "scope_to_lumen_json_check", "lumenignore_fresh")
def drift_head_ok(head):
    """The report's head must be a real commit of --repo and be HEAD, or an ancestor of HEAD from which only files under the ledger directory changed
    (the report itself is committed after it ran, so HEAD moves by exactly the evidence commit). Returns None or a reason."""
    if not isinstance(head, str) or not re.fullmatch(r"[0-9a-fA-F]{7,64}", head) or git("rev-parse", "--verify", "-q", head + "^{commit}").returncode != 0:  # MUT:drift-head-known
        return "drift_head_unknown"
    cur = git("rev-parse", "--verify", "-q", "HEAD")
    if cur.returncode != 0:
        return "drift_head_unknown"
    full = git("rev-parse", "--verify", "-q", head + "^{commit}").stdout.decode().strip()
    if full == cur.stdout.decode().strip():
        return None
    L = ledger()
    ldir = None if isinstance(L, str) else os.path.relpath(L[0], REPO)
    if git("merge-base", "--is-ancestor", full, "HEAD").returncode != 0 or ldir is None or ldir.startswith(".."):  # MUT:drift-head-ancestor
        return "drift_head_not_current"
    diff = git("diff", "--name-only", "-z", full, "HEAD")
    if diff.returncode != 0:
        return "drift_head_not_current"
    changed = [x for x in diff.stdout.decode("utf-8", "replace").split("\0") if x]
    pre = "" if ldir == "." else ldir.rstrip("/") + "/"
    if any(not c.startswith(pre) for c in changed):  # MUT:drift-head-only-ledger-dir
        return "drift_head_not_current"
    return None

def p_drift():
    """--drift-report is the object scripts/audit/scope_drift_check.sh writes (schema scope-drift/1). It is bound to RECORDED, REAL values (review R3-2):
    the report file is a tracked, ledger-listed artifact; its head is the real HEAD (or the ancestor described in drift_head_ok); scope_sha256,
    lumen_scope_sha256 and lumenignore_sha256 equal the sha256 of the files given to THIS gate (an all-zero digest is refused); and the report's
    lumenignore_fresh rc must agree with the gate's own byte comparison of the .lumenignore pair."""
    if "--drift-report" not in opts:
        return row("P-ScopeDrift", "SKIP", reason="no_drift_report_supplied")
    if "--lumen-json" not in opts:  # MUT:lumen-json-required
        return row("P-ScopeDrift", "FAIL", reason="lumen_json_not_supplied")
    raw, bad = bind_artifact(opts["--drift-report"], "drift_report")
    if bad:
        return row("P-ScopeDrift", "FAIL", reason=bad)
    try:
        d = json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        d = None
    sha = lambda v: isinstance(v, str) and re.fullmatch(r"[0-9a-f]{64}", v)
    if not (isinstance(d, dict) and d.get("schema") == "scope-drift/1" and isinstance(d.get("head"), str) and d["head"]
            and sha(d.get("scope_sha256")) and sha(d.get("lumen_scope_sha256")) and sha(d.get("lumenignore_sha256")) and isinstance(d.get("checks"), list)
            and all(isinstance(e, dict) and isinstance(e.get("check"), str) and is_count(e.get("rc")) for e in d["checks"])):
        return row("P-ScopeDrift", "FAIL", reason="drift_report_unusable")
    seen = {e["check"]: e["rc"] for e in d["checks"]}
    why = ["missing_check:" + c for c in DRIFT_REQUIRED if c not in seen]
    why += ["drift:" + c for c, rc in sorted(seen.items()) if rc != 0]  # MUT:drift-rc
    if any(is_zero_digest(d[k]) for k in ("scope_sha256", "lumen_scope_sha256", "lumenignore_sha256")):  # MUT:drift-zero
        why.append("drift_digest_zero")
    h = drift_head_ok(d["head"])
    if h:
        why.append(h)
    read_raw("scope", opts["--scope"])
    if (inputs.get("scope") or {}).get("sha256") != d["scope_sha256"]:  # MUT:drift-bind-scope
        why.append("drift_report_not_for_this_scope")
    read_raw("lumen_json", opts["--lumen-json"])
    if (inputs.get("lumen_json") or {}).get("sha256") != d["lumen_scope_sha256"]:  # MUT:drift-bind-lumen-json
        why.append("drift_report_not_for_this_lumen_scope")
    not_checked = []
    if "--lumenignore" in opts:
        read_raw("lumenignore", opts["--lumenignore"])
        if (inputs.get("lumenignore") or {}).get("sha256") != d["lumenignore_sha256"]:  # MUT:drift-bind-lumenignore
            why.append("drift_report_not_for_this_lumenignore")
        if "--lumenignore-fresh" in opts and "lumenignore_fresh" in seen:
            a_, b_ = read_raw("lumenignore", opts["--lumenignore"]), read_raw("lumenignore_fresh", opts["--lumenignore-fresh"])
            if a_ is not None and b_ is not None and (seen["lumenignore_fresh"] == 0) != (a_ == b_ and bool(b_.strip())):  # MUT:drift-rc-crosscheck
                why.append("drift_disagrees_with_lumenignore_compare")
    else:
        not_checked.append("lumenignore_binding_not_checked_no_lumenignore_supplied")
    return row("P-ScopeDrift", "FAIL" if why else "PASS", cmd="scripts/audit/scope_drift_check.sh (captured report)", reasons=why, head=d["head"], not_checked=not_checked)

load_live()
p1row, status = p1()
cg_rows = [p1row, p2(), p3(), live_or_skip("P4"), live_or_skip("P5"), p_drift(), p_lumenignore()]
lumen()
parity = p8()
all_rows = cg_rows + lumen_rows + [parity]
skips = [r for r in all_rows if r["verdict"] == "SKIP"]
if any(r["verdict"] == "FAIL" for r in all_rows):  # MUT:verdict
    verdict = "FAIL"
elif skips:  # MUT:skip-incomplete
    verdict = "INCOMPLETE"
else:
    verdict = "PASS"
doc = {"schema": "index-health/1", "verdict": verdict,
       "complete": not skips,
       "skipped_proofs": [r["id"] for r in skips],
       "blocked_unavailable": {r["id"]: r.get("reason", "") for r in skips},
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
sys.exit({"PASS": 0, "FAIL": 1, "INCOMPLETE": 3}[verdict])
PY
