#!/usr/bin/env python3
"""derive_applicability.py - T196 (TS-00). Re-derives specs/001-full-project-audit-remediation/matrix/applicability.yaml from direct reads of the
repository, so that no cell is invented: every cell carries a state and a reason that names the measurement it came from.

Usage: derive_applicability.py [--repo DIR] [--out FILE]      (default: write the tracked applicability.yaml; `-` writes to stdout)

Sources, in order of authority:
  1. applications A1..A9 (and their per-type n/a reasons): the `4.4` table of docs/05 (`specs/.../docs/05-test-strategy-and-coverage-matrix.md`),
     read cell by cell, with the one correction the TS-00 reads below made (A8 Website DDoS, see WEBSITE_DDOS);
  2. every shared module of `.gitmodules` (A10 Go libraries, A11 TypeScript/React modules, A12 governance and QA modules): measured by walking the
     module's tree (excluding vendored `tools/opensource`, `vendor`, `node_modules`, `.git`): test files, test functions, benchmark and fuzz files,
     build tags and the file names that carry an integration / e2e / security / stress / chaos / performance / UI marker;
  3. A13 (the repository-level harness): measured over `scripts/`, `tools/`, `tests/`, `challenges/`.

HONEST LIMITS (11.4.6): a state other than `A`/`n/a` is a STRUCTURAL reading (a file with the marker exists), never a verdict that the test uses a real
system or passes; `P` is given only for `unit` (the doc05 4.4 reading rule) and every other present-looking cell is `~` until the evidence ledger
(doc 06) proves it. The ledger-derived status of docs/05 13.1 is computed by gen_matrix.py, not here. The output is a pure function of the TRACKED files of
the repository (`git ls-files` per root: build output and untracked scratch are never counted; a root that is not a work tree is walked, and the header says so),
so two runs over the same tracked tree are byte-identical (checked by tests/test_derive_applicability.sh). The header carries `enumeration` and a
`files_fingerprint` (sha256 over the counted file list) so a reader can verify a re-derivation against the same fingerprint.

Review round 1 of WP-23 (WF11 I1, I2, I11, m1, m2, m11): an uninitialised or empty submodule is REFUSED (exit 3 `submodule_uninitialised`), never read as "no tests";
markers are matched as path TOKENS (`e2e` is not `e2ee`, `load` is not `loader`, `fault` is not `defaults`, `perf` is not `perfetto`); the only directories skipped are
vendored third-party trees, so first-party `scripts/build`, `scripts/coverage` and `pkg/coverage` are counted; the A9 build cells are RE-READ from tests/ instead of copied
from docs/05; the Website DDoS cell is re-measured every run.
"""
import hashlib, json, os, re, subprocess, sys

TYPES = ["unit", "integration", "e2e", "full_automation", "security", "ddos", "scaling", "chaos", "stress",
         "performance", "benchmarking", "ui", "ux", "challenges", "helixqa"]
SKIP_DIRS = {".git", "node_modules", "vendor", "opensource"}   # vendored third-party trees only (review I2: build/ and coverage/ hold first-party code here)
WALK_ONLY_SKIP = {"dist", "target"}   # build output, skipped only when the root is not a git work tree (a tracked list never contains them)
_FPR = {}   # repo-relative root label -> (mode, file count, sha256 of the sorted list): the provenance of the counted files
_REPO = [""]

DOC05 = "specs/001-full-project-audit-remediation/docs/05-test-strategy-and-coverage-matrix.md"
# doc05 4.4 columns that are applications (A10 and A11 are aggregates, expanded per module below)
APP_COLS = {"A1": "catalog-api", "A2": "catalog-web", "A3": "catalogizer-desktop", "A4": "installer-wizard", "A5": "catalogizer-android",
            "A6": "catalogizer-androidtv", "A7": "catalogizer-api-client", "A8": "website", "A9": "build"}
TYPE_ROWS = {1: "unit", 2: "integration", 3: "e2e", 4: "full_automation", 5: "security", 6: "ddos", 7: "scaling", 8: "chaos", 9: "stress",
             10: "performance", 11: "benchmarking", 12: "ui", 13: "ux", 14: "challenges", 15: "helixqa"}

NA_REASON = {  # per (application, type): the reason of an n/a cell of doc05 4.4, from docs/05 13.2 and the TS-00 reads
    ("catalog-api", "ui"): "no user interface (a Gin HTTP API; its handlers render no UI)",
    ("catalog-api", "ux"): "no user interface (a Gin HTTP API); accessibility and locale concerns are those of its clients",
    ("catalog-web", "ddos"): "client application, not a service; the flood tests target the API (A1)",
    ("catalog-web", "scaling"): "client application, no replicas to add or remove",
    ("catalogizer-desktop", "ddos"): "client application, not a service; the flood tests target the API (A1)",
    ("catalogizer-desktop", "scaling"): "client application, no replicas to add or remove",
    ("installer-wizard", "ddos"): "client-side installer, not a service",
    ("installer-wizard", "scaling"): "client-side installer, no replicas",
    ("catalogizer-android", "ddos"): "client application, not a service; the flood tests target the API (A1)",
    ("catalogizer-android", "scaling"): "client application, no replicas",
    ("catalogizer-androidtv", "ddos"): "client application, not a service; the flood tests target the API (A1)",
    ("catalogizer-androidtv", "scaling"): "client application, no replicas",
    ("catalogizer-api-client", "ddos"): "a TypeScript library, not a service",
    ("catalogizer-api-client", "scaling"): "a TypeScript library, no replicas",
    ("catalogizer-api-client", "ui"): "a library with no user interface (docs/05 3: A7 is a TypeScript library)",
    ("catalogizer-api-client", "ux"): "a library with no user interface",
    ("website", "scaling"): "static VitePress site (Website/package.json has dev/build/preview only); hosting platform concern",
    ("website", "chaos"): "static VitePress site: no process, database or network dependency of its own to disturb",
    ("website", "stress"): "static VitePress site: no server code in the repository to load",
    ("website", "benchmarking"): "static VitePress site: no benchmarkable server logic (frontend speed is the performance type)",
    ("build", "e2e"): "the build framework has no end-user flow; its runs are the integration and full-automation cells",
    ("build", "ddos"): "a bash build framework, not a service",
    ("build", "scaling"): "a bash build framework, not a service",
    ("build", "stress"): "a bash build framework, not a service: no load to apply",
    ("build", "ui"): "no user interface (bash build framework)",
    ("build", "ux"): "no user interface (bash build framework)",
}
# TS-00 correction of docs/05 4.4: it records A8 DDoS as `A`; docs/05 13.2 proposes n/a. The read decides (website_ddos, re-measured on every run).

APP_READING = "docs/05 4.4 (measured 2026-10-03)"
HOSTING_RE = re.compile(r"(^|/)(nginx[^/]*|dockerfile[^/]*|docker-compose[^/]*|netlify\.toml|vercel\.json|wrangler\.toml|firebase\.json|\.htaccess|_redirects|_headers|app\.yaml)$", re.I)


def website_ddos(repo):
    """Review m11: the Website DDoS cell is re-measured, not a frozen sentence. n/a only while Website/ holds no server, container or hosting configuration."""
    files = [os.path.relpath(p, repo) for p in walk(repo, "Website")]
    hits = sorted(r for r in files if HOSTING_RE.search(r))
    if hits:
        return ("A", "TS-00 re-read: Website/ holds hosting or server configuration (%s), so there is code of ours to flood; applicable and unverified" % ", ".join(hits[:5]))
    if not files:
        return ("A", "TS-00 re-read: Website/ holds no tracked file, so the DDoS applicability could not be measured; counted as an absent cell, never n/a")
    return ("n/a", "TS-00 re-read (%d tracked file(s) under Website/): no server, container or hosting configuration (no nginx, Dockerfile, docker-compose, netlify, vercel, wrangler, firebase, .htaccess, _redirects, _headers or app.yaml file), so there is no code of ours to flood; the hosting platform owns it (docs/05 13.2). docs/05 4.4 recorded `A`, this measurement supersedes it" % len(files))


def build_reread(repo, cells):
    """Review I11: the A9 unit cell is read from tests/, not copied from docs/05 4.4 (which said 0 test files in Build/). tests/test_build_system.sh copies Build/ into a
    temp project and sources its libraries, so the unit cell is partial (structural reading); integration and full_automation stay as docs/05 recorded them, with the re-read named."""
    t = os.path.join(repo, "tests", "test_build_system.sh")
    txt = read(t)
    libs = sorted(set(re.findall(r"Build/lib/([A-Za-z_]+\.sh)", txt)))
    out = {}
    if libs:
        n = len(re.findall(r"^test_[A-Za-z0-9_]+\(\)", txt, re.M))
        out["unit"] = ("~", "TS-00 re-read: tests/test_build_system.sh sources %d Build/lib script(s) (%s) from a temp copy of Build/ and defines %d test function(s); structural reading, whether the lines are executed is for the bash line harness (docs/05 7.1) to measure; docs/05 4.4 recorded `A` (0 test files in Build/), this read supersedes it" % (len(libs), ", ".join(libs), n))
    for k in ("integration", "full_automation"):
        if cells[k][0] == "A":
            out[k] = ("A", "docs/05 4.4 column A9 row %s, re-read: no test runs Build/ as a whole through its entrypoint; tests/test_build_system.sh exercises the libraries separately (%s)" % (k, "present" if libs else "absent"))
    return out


def _tracked(root, sub=""):
    """Tracked files of `root` (relative to root), or None when root is not a git work tree."""
    if not os.path.exists(os.path.join(root, ".git")):
        return None
    try:
        p = subprocess.run(["git", "-C", root, "ls-files", "-z"] + (["--", sub] if sub else []), stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    except OSError:
        return None
    if p.returncode != 0:
        return None
    return [n for n in p.stdout.decode("utf-8", "replace").split("\0") if n]


def _vendored(rel):
    return any(part in SKIP_DIRS for part in rel.split("/")[:-1]) or ("tools/opensource/" in rel)


def walk(root, sub=""):
    """Absolute paths of the counted files under root/sub, sorted; the list is also recorded for the provenance fingerprint."""
    tracked = _tracked(root, sub)
    if tracked is not None:
        rels = sorted(r for r in tracked if not _vendored(r))
        mode = "git-tracked"
    else:
        base = os.path.join(root, sub) if sub else root
        rels = []
        for dp, dns, fns in os.walk(base):
            dns[:] = sorted(d for d in dns if d not in SKIP_DIRS and d not in WALK_ONLY_SKIP)
            for f in sorted(fns):
                rels.append(os.path.relpath(os.path.join(dp, f), root).replace(os.sep, "/"))
        rels.sort(); mode = "walk"
    label = os.path.relpath(os.path.join(root, sub) if sub else root, _REPO[0] or root).replace(os.sep, "/")
    _FPR[label] = (mode, len(rels), hashlib.sha256("\n".join(rels).encode("utf-8")).hexdigest())
    for r in rels:
        yield os.path.join(root, r)


def read(p):
    try:
        with open(p, encoding="utf-8", errors="replace") as fh:
            return fh.read()
    except OSError:
        return ""


def _tok(words):
    """A marker is a TOKEN of the path (review I2): split on anything that is not a letter or digit and on camelCase, lower-cased; `e2e` never matches `e2ee`,
    `load` never `loader` or `LoadingSpinner`, `fault` never `defaults`, `perf` never `perfetto`. `end_to_end` is the three-token sequence."""
    class M:
        def search(self, rel):
            toks = [t for t in re.split(r"[^a-z0-9]+", re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", rel).lower()) if t]
            joined = "_" + "_".join(toks) + "_"
            return any(("_" + w + "_") in joined for w in words)
    return M()


def module_facts(root):
    f = dict(go_tests=0, go_funcs=0, bench=0, fuzz=0, ts_tests=0, ts_decl=0, sh_tests=0, py_tests=0, tags=set(),
             n_int=0, n_e2e=0, n_sec=0, n_st=0, n_ch=0, n_pf=0, n_ui=0, n_chal=0, a11y=0, has_go=False, has_pkg=False, react=False, io_files=0)
    r_int, r_e2e, r_sec, r_st, r_ch, r_pf, r_ui = (_tok(s) for s in (("integration", "integrations"), ("e2e", "end_to_end"), ("security", "inject", "injection", "fuzz", "fuzzing"),
                                                  ("stress", "load", "soak"), ("chaos", "fault", "faults"), ("perf", "performance", "latency"), ("visual", "screenshot", "screenshots")))
    for p in walk(root):
        base = os.path.basename(p)
        rel = os.path.relpath(p, root)
        if base == "go.mod" and os.sep not in rel:
            f["has_go"] = True
        if base == "package.json" and os.sep not in rel:
            f["has_pkg"] = True
            f["react"] = "\"react\"" in read(p)
        test_file = False
        if base.endswith("_test.go"):
            test_file = True
            t = read(p)
            f["go_tests"] += 1
            f["go_funcs"] += len(re.findall(r"^func Test", t, re.M))
            if re.search(r"^func Benchmark", t, re.M): f["bench"] += 1
            if re.search(r"^func Fuzz", t, re.M): f["fuzz"] += 1
            for m in re.findall(r"^//go:build (.+)$", t, re.M): f["tags"].add(m.strip())
        elif re.search(r"\.(test|spec)\.(ts|tsx|js|jsx)$", base):
            test_file = True
            t = read(p)
            f["ts_tests"] += 1
            f["ts_decl"] += len(re.findall(r"^\s*(?:it|test)\(", t, re.M))
            if "@testing-library" in t or re.search(r"(?<![A-Za-z0-9_])render\(", t): f["a11y"] += 1   # MUT:render_call  (`prerender(` is not a render call)
        elif re.match(r"(test_.*|.*_test)\.sh$", base):
            test_file = True; f["sh_tests"] += 1
        elif re.match(r"test_.*\.py$", base) or re.match(r".*_test\.py$", base):
            test_file = True; f["py_tests"] += 1
        if test_file:
            for key, r in (("n_int", r_int), ("n_e2e", r_e2e), ("n_sec", r_sec), ("n_st", r_st), ("n_ch", r_ch), ("n_pf", r_pf), ("n_ui", r_ui)):
                if r.search(rel): f[key] += 1
        if base.endswith(".go") and not base.endswith("_test.go"):
            if re.search(r'"net/http"|"net"|database/sql|os/exec', read(p)): f["io_files"] += 1
    f["tags"] = sorted(f["tags"])
    return f


def parse_doc_table(repo):
    """The 4.4 table of docs/05: {(column id, type): state}."""
    txt = read(os.path.join(repo, DOC05))
    m = re.search(r"### 4\.4 The matrix.*?\n(\| Type .*?)\n\n", txt, re.S)
    if not m:
        sys.exit("derive_applicability: the 4.4 table of docs/05 was not found")
    rows = [r for r in m.group(1).split("\n") if r.startswith("|")]
    hdr = [c.strip() for c in rows[0].strip("|").split("|")]
    cols = [re.match(r"(A\d+)", h).group(1) if re.match(r"A\d+", h) else h for h in hdr]
    out = {}
    for r in rows[2:]:
        cells = [c.strip() for c in r.strip("|").split("|")]
        n = int(cells[0].split()[0])
        for cid, v in zip(cols[1:], cells[1:]):
            out[(cid, TYPE_ROWS[n])] = v.split()[0] if v else v
    return out


def norm(s):
    return {"n/a": "n/a", "P": "P", "~": "~", "A": "A", "?": "?"}.get(s, s)


def go_lib_cells(name, f, kind):
    """Cells of a Go library (A10) or a Go governance/QA module (A12). Returns {type: (state, reason)}."""
    tf, fn = f["go_tests"], f["go_funcs"]
    c = {}
    meas = "%d test files, %d Test functions" % (tf, fn)
    c["unit"] = ("P" if tf >= 10 else "~" if tf >= 1 else "A", "TS-00 read (%s); `P` from 10 test files, doc05 4.4 reading rule" % meas)
    def named(key, what, absent_when="no file name"):
        n = f[key]
        if n:
            return ("~", "structural: %d test file(s) whose path carries a %s marker; use of a real system is UNCONFIRMED (the ledger decides)" % (n, what))
        return ("A", "TS-00 read: %s carries a %s marker among its %d test files" % (absent_when.replace("no file name", "no test file name"), what, tf))
    tagint = [t for t in f["tags"] if _tok(("integration", "integrations")).search(t)]   # MUT:tag_token  (a build tag is matched as a token too: `disintegration` is not one)
    if f["n_int"] or tagint:
        c["integration"] = ("~", "structural: %d test file(s) with an integration path marker, build tags %s; real-service use UNCONFIRMED" % (f["n_int"], f["tags"] or "none"))
    else:
        c["integration"] = ("A", "TS-00 read: no integration path marker or build tag among %d test files" % tf)
    c["e2e"] = named("n_e2e", "e2e")
    c["full_automation"] = ("A", "TS-00 read: no orchestrated per-feature full-automation suite for this module (the repository suites of A13 drive the API)")
    c["security"] = ("~", "structural: %d test file(s) with a security/inject/fuzz path marker, %d fuzz file(s); content UNCONFIRMED" % (f["n_sec"], f["fuzz"])) \
        if (f["n_sec"] or f["fuzz"]) else ("A", "TS-00 read: no security, injection or fuzz test among %d test files" % tf)
    if name in ("rate_limiter", "middleware"):
        c["ddos"] = ("A", "TS-00 read: a rate-limiting/protective module (flood behaviour is its purpose); no flood test marker among %d test files" % tf)
    else:
        c["ddos"] = ("n/a", "a library, not a network service; the flood tests target the API (A1) which composes this module")
    c["scaling"] = ("n/a", "a library, no replicas to add or remove")
    if f["io_files"] == 0 and kind == "lib":
        c["chaos"] = ("n/a", "TS-00 read: no net/http, net, database/sql or os/exec import in non-test sources, so no process or network boundary to disturb")
    else:
        c["chaos"] = named("n_ch", "chaos/fault") if f["n_ch"] else ("A", "TS-00 read: %d non-test file(s) import net, database or exec, and no chaos or fault test marker among %d test files" % (f["io_files"], tf))
    c["stress"] = named("n_st", "stress/load/soak")
    c["performance"] = ("~", "structural: %d test file(s) with a perf/latency marker; baselines UNCONFIRMED" % f["n_pf"]) if f["n_pf"] else \
        ("A", "TS-00 read: no performance/latency test marker among %d test files" % tf)
    c["benchmarking"] = ("~", "structural: %d file(s) with `func Benchmark`; drift detection UNCONFIRMED" % f["bench"]) if f["bench"] else \
        ("A", "TS-00 read: no `func Benchmark` among %d test files" % tf)
    c["ui"] = ("n/a", "a library with no user interface")
    c["ux"] = ("n/a", "a library with no user interface")
    c["challenges"] = ("A", "TS-00 read: no Challenge scripts bound to this module are known; the Challenges of A1 exercise it through catalog-api")
    c["helixqa"] = ("A", "TS-00 read: no HelixQA bank targets this module directly")
    return c


def ts_module_cells(name, f):
    tf = f["ts_tests"]
    meas = "%d test files, %d it/test declarations" % (tf, f["ts_decl"])
    c = {}
    c["unit"] = ("P" if tf >= 4 else "~" if tf >= 1 else "A", "TS-00 read (%s); `P` from 4 test files for a module of this size (doc05 4.4 `P (most)`)" % meas)
    c["integration"] = ("~", "structural: %d test file(s) with an integration path marker; UNCONFIRMED" % f["n_int"]) if f["n_int"] else \
        ("A", "TS-00 read: no integration path marker among %d test files" % tf)
    c["e2e"] = ("A", "TS-00 read: a library module; the flows run in catalog-web E2E (A2)")
    c["full_automation"] = ("A", "TS-00 read: no per-feature automation suite for this module")
    c["security"] = ("A", "TS-00 read: no security test marker among %d test files (doc05 4.4 A11 `A`)" % tf)
    c["ddos"] = ("n/a", "a client-side library, not a service")
    c["scaling"] = ("n/a", "a client-side library, no replicas")
    c["chaos"] = ("A", "TS-00 read: no chaos/fault marker among %d test files (doc05 4.4 A11 `A`)" % tf)
    c["stress"] = ("n/a", "a client-side library: no sustained-load surface (doc05 4.4 A11 `n/a`)")
    c["performance"] = ("A", "TS-00 read: no performance marker among %d test files (doc05 4.4 A11 `A`)" % tf)
    c["benchmarking"] = ("A", "TS-00 read: no benchmark among %d test files (doc05 4.4 A11 `A`)" % tf)
    if f["react"]:
        c["ui"] = ("~", "structural: %d test file(s) render a component with a testing-library or render() call; visual proof UNCONFIRMED (doc05 4.4 A11 `~`)" % f["a11y"]) if f["a11y"] else \
            ("A", "TS-00 read: a React module with no component-render test among %d test files" % tf)
        c["ux"] = ("A", "TS-00 read: a React module; the WCAG 2.2 AA checks and walkthroughs are authored in WP-61 (docs/05 13.4: automated axe checks in component tests with a real DOM)")
    else:
        c["ui"] = ("n/a", "a non-visual TypeScript module (types, API client or socket client): no user interface")
        c["ux"] = ("n/a", "a non-visual TypeScript module: no user interface")
    c["challenges"] = ("A", "TS-00 read: no Challenge script targets this module (doc05 4.4 A11 `A`)")
    c["helixqa"] = ("A", "TS-00 read: no HelixQA bank targets this module (doc05 4.4 A11 `A`)")
    return c


def governance_cells(name, f):
    if f["has_go"]:
        c = go_lib_cells(name, f, "gov")
        # a QA/governance module is a toolchain for the project, not a network service
        c["ddos"] = ("n/a", "a QA/governance Go module, not a network service")
        if name in ("vision_engine", "screen_diff", "visual_regression", "replay_buffer", "training_collector"):
            c["ui"] = ("n/a", "an image/vision library with no user interface of its own")
            c["ux"] = ("n/a", "an image/vision library with no user interface of its own")
        if name == "helix_qa":
            c["ui"] = ("A", "applicable until the WP-34 audit row decides: HelixQA tracks two operator web pages (docs/website/challenges-dashboard, docs/website/ticket-viewer); counted as an absent cell, not hidden (docs/05 13.4)")
            c["ux"] = ("A", "applicable until the WP-34 audit row decides (see ui); counted as an absent cell, not hidden")
        if name in ("challenges", "helix_qa"):
            c["challenges"] = ("~", "structural: the module is the Challenges / HelixQA framework itself; its own tests are its unit cell and its framework use is UNCONFIRMED")
            c["helixqa"] = ("~", "structural: the module is the Challenges / HelixQA framework itself; UNCONFIRMED")
        return c
    # constitution, superspec: governance text, templates and shell
    n = f["sh_tests"] + f["py_tests"] + f["go_tests"] + f["ts_tests"]
    meas = "%d test files (shell %d, python %d, go %d, ts %d)" % (n, f["sh_tests"], f["py_tests"], f["go_tests"], f["ts_tests"])
    c = {t: ("A", "TS-00 read (%s): governance text and scripts with no test of this type found" % meas) for t in TYPES}
    c["unit"] = ("P" if n >= 10 else "~" if n >= 1 else "A", "TS-00 read (%s)" % meas)
    for t, why in (("ddos", "not a service"), ("scaling", "not a service"), ("ui", "no user interface"), ("ux", "no user interface"),
                   ("stress", "not a service: no load to apply"), ("benchmarking", "no benchmarkable logic: governance text and shell")):
        c[t] = ("n/a", "a governance submodule (%s)" % why)
    return c


def build_components(repo):
    doc = parse_doc_table(repo)
    comps = []  # (id, group, name, path, cells)
    for cid, name in APP_COLS.items():
        cells = {}
        for t in TYPES:
            v = norm(doc[(cid, t)])
            if v == "n/a":
                if (name, t) == ("website", "ddos"):
                    continue
                r = NA_REASON.get((name, t))
                if not r:
                    sys.exit("derive_applicability: no n/a reason for (%s, %s)" % (name, t))
                cells[t] = ("n/a", r)
            elif v in ("P", "~", "A"):
                cells[t] = (v, "%s, column %s row %s" % (APP_READING, cid, t))
            else:
                sys.exit("derive_applicability: unexpected state %r for (%s, %s)" % (v, cid, t))
        if name == "website":
            cells["ddos"] = website_ddos(repo)
        if name == "build":
            cells.update(build_reread(repo, cells))
        comps.append((cid, "application", name, {"A1": "catalog-api/", "A2": "catalog-web/", "A3": "catalogizer-desktop/", "A4": "installer-wizard/",
                                                    "A5": "catalogizer-android/", "A6": "catalogizer-androidtv/", "A7": "catalogizer-api-client/",
                                                    "A8": "Website/", "A9": "Build/"}[cid], cells))
    # submodules
    gm = read(os.path.join(repo, ".gitmodules"))
    paths = re.findall(r"path = (\S+)", gm)
    a10 = ["assets", "auth", "cache", "concurrency", "config", "database", "discovery", "entities", "event_bus", "filesystem", "lazy", "media", "memory",
           "middleware", "observability", "rate_limiter", "recovery", "security", "storage", "streaming", "watcher"]
    a11 = ["auth_context_react", "catalogizer_api_client_ts", "collection_manager_react", "dashboard_analytics_react", "media_browser_react",
           "media_player_react", "media_types_ts", "ui_components_react", "websocket_client_ts"]
    names = [p.split("/", 1)[1] for p in paths]
    a12 = [n for n in names if n not in a10 and n not in a11]
    for n in a10 + a11 + a12:
        if n not in names:
            sys.exit("derive_applicability: %s is not in .gitmodules" % n)
    # review I1: a submodule that is not checked out holds no files, and "no files" would be read as "no tests" - 51 applicable cells became n/a that way in the
    # reviewer's probe. Fail closed (11.4.233 G): every module root must hold at least one tracked/regular file before anything is derived.
    empty = [n for n in a10 + a11 + a12 if not any(True for _ in walk(os.path.join(repo, "submodules", n)))]   # MUT:empty_submodule
    if empty:
        sys.stderr.write("derive_applicability: REFUSED reason=submodule_uninitialised %d submodule(s) hold no file (run `git submodule update --init --recursive`): %s\n" % (len(empty), ", ".join(empty)))
        sys.exit(3)
    for group, lst, gid in (("go_module", a10, "A10"), ("ts_module", a11, "A11"), ("governance_qa_module", a12, "A12")):
        for n in lst:
            f = module_facts(os.path.join(repo, "submodules", n))
            if group == "go_module": cells = go_lib_cells(n, f, "lib")
            elif group == "ts_module": cells = ts_module_cells(n, f)
            else: cells = governance_cells(n, f)
            comps.append((gid + ":" + n, group, n, "submodules/" + n + "/", cells))
    # A13: the repository-level harness, measured
    f = dict(sh=0, k6=0, banks=0, chal=0)
    for top in ("scripts", "tools", "tests"):
        for p in walk(repo, top):
            if re.match(r"(test_.*|.*_test)\.sh$", os.path.basename(p)): f["sh"] += 1
    sub = lambda rel: [p for p in walk(repo, rel)] if os.path.isdir(os.path.join(repo, rel)) else []
    f["k6"] = len([p for p in sub("tests/k6") if p.endswith(".js")])
    f["banks"] = len([p for p in sub("challenges/helixqa-banks") if p.endswith(".yaml") and os.path.basename(p) != "MANIFEST.yaml"])   # review m2: the manifest is not a bank
    f["chal"] = len(sub("challenges/scripts"))
    fa = len([p for p in sub("scripts/testing/full_automation") if p.endswith(".sh")])
    c = {}
    c["unit"] = ("~", "TS-00 read: %d `test_*.sh` / `*_test.sh` files under scripts/, tools/, tests/; whether they cover the harness code is UNCONFIRMED (docs/05 7.1: the bash line harness measures it)" % f["sh"])
    c["integration"] = ("A", "TS-00 read: the harness has no test of its own that drives its parts together")
    c["e2e"] = ("n/a", "test and harness code: it is the instrument that runs the end-to-end flows, not their subject")
    c["full_automation"] = ("~", "TS-00 read: %d bash suites under scripts/testing/full_automation; they drive the API, so they are the instrument, not a test of the harness" % fa)
    c["security"] = ("A", "TS-00 read: the harness handles credentials and runs commands; no security test of it is known")
    c["ddos"] = ("n/a", "test and harness code, not a service")
    c["scaling"] = ("n/a", "test and harness code, not a service")
    c["chaos"] = ("A", "TS-00 read: no chaos test of the harness is known")
    c["stress"] = ("~", "TS-00 read: %d k6 scripts under tests/k6 apply load to A1; they are instruments, not a stress test of the harness" % f["k6"])
    c["performance"] = ("~", "TS-00 read: %d k6 scripts under tests/k6; instruments, not subject" % f["k6"])
    c["benchmarking"] = ("A", "TS-00 read: no benchmark of the harness")
    c["ui"] = ("n/a", "no user interface (test and harness code)")
    c["ux"] = ("n/a", "no user interface (test and harness code)")
    c["challenges"] = ("~", "TS-00 read: %d files under challenges/scripts; the Challenges are instruments applied to A1..A6" % f["chal"])
    c["helixqa"] = ("~", "TS-00 read: %d bank files under challenges/helixqa-banks; doc05 4.4 records 1178 non-executable placeholder steps (finding F-1)" % f["banks"])
    comps.append(("A13", "harness", "harness", "scripts/ tools/ tests/ challenges/", c))
    return comps


def cross_cutting():
    na = lambda r: {"status": "na", "reason": r}
    tr = {
        "catalog-api": {"status": "open", "reason": "localization_handlers.go exists in catalog-api/internal/handlers; its reachability in the running server is UNCONFIRMED, WP-30 (T227) records it"},
        "catalog-web": na("no i18n library declared in catalog-web/package.json (manifest read, docs/05 13.4)"),
        "catalogizer-desktop": na("no i18n library in catalogizer-desktop/package.json"),
        "installer-wizard": na("no i18n library in installer-wizard/package.json"),
        "catalogizer-android": na("only the default res/values folder, no values-<locale> folder"),
        "catalogizer-androidtv": na("only the default res/values folder, no values-<locale> folder"),
        "website": na("no locale configuration in Website/package.json or .vitepress/config.ts as read"),
        "catalogizer-api-client": na("a library with no user-facing strings"),
        "build": na("no user-facing text"),
        "go-modules": na("an i18n seam exists in several modules with an English bundle only; no translated bundle found (docs/05 13.4 revision 4)"),
        "ts-react-modules": na("they render text supplied by catalog-web"),
        "governance-qa-modules": {"status": "open", "reason": "translated bundles exist in submodules/containers (de, fr, ja, sr, zh) and submodules/doc_processor (sr); whether they went through the 11.4.255 pipeline and the 11.4.256 / 11.4.237 reviews is UNCONFIRMED, owned by the WP-34 audit rows"},
        "harness": na("test and harness code, no user-facing text"),
    }
    acc = {
        "applies": ["catalog-web", "catalogizer-desktop", "installer-wizard", "catalogizer-android", "catalogizer-androidtv", "website", "ts-react-modules"],
        "na": {"catalog-api": "no user interface", "catalogizer-api-client": "a library", "build": "no user interface",
               "go-modules": "no user interface", "harness": "no user interface"},
        "open": {"governance-qa-modules": "HelixQA operator web pages; the 11.4.190 scope is decided on the WP-34 row"},
    }
    return {"translation_i18n": tr, "accessibility_wcag22_aa": acc}


def q(s):
    return json.dumps(s, ensure_ascii=False)


def render(repo):
    _REPO[0] = repo; _FPR.clear()
    comps = build_components(repo)
    out = []
    out.append("# applicability.yaml - T196 (TS-00), docs/05 13.2 format. GENERATED by tools/evidence/matrix/derive_applicability.py from direct reads of the")
    out.append("# repository: do not hand-edit a cell, change the derivation or the repository and re-run it. Every cell carries the state and the reason that")
    out.append("# names its measurement. States: P present, ~ partial, A absent (applicable), n/a not applicable. There is no `?` cell (docs/05 4.4 legend).")
    out.append("# States other than `A` and `n/a` are STRUCTURAL readings (11.4.6): the evidence ledger (doc 06) and gen_matrix.py decide the 13.1 status.")
    # provenance (review m1): what was counted. `enumeration` is git-tracked for a work tree, walk otherwise; the fingerprint is the sha256 of the per-root file-list hashes.
    modes = sorted(set(v[0] for v in _FPR.values())) or ["walk"]
    out.append("# enumeration: %s; files_fingerprint: %s (%d roots, %d files)" % ("+".join(modes), hashlib.sha256("\n".join("%s %s %d %s" % ((k,) + v) for k, v in sorted(_FPR.items())).encode()).hexdigest()[:32],
                                                                           len(_FPR), sum(v[1] for v in _FPR.values())))
    out.append("schema: applicability/1")
    out.append("types: [%s]" % ", ".join(TYPES))
    out.append("source: %s" % q("tools/evidence/matrix/derive_applicability.py over the tree, docs/05 4.4 for A1..A9"))
    out.append("components:")
    for cid, group, name, path, cells in comps:
        out.append("  %s:" % q(cid))
        out.append("    group: %s" % group)
        out.append("    name: %s" % q(name))
        out.append("    path: %s" % q(path))
        out.append("    cells:")
        for t in TYPES:
            st, why = cells[t]
            out.append("      %s: {state: %s, reason: %s}" % (t, q(st) if st in ("~", "n/a") else st, q(why)))
    out.append("cross_cutting:")
    cc = cross_cutting()
    out.append("  translation_i18n:")
    for k, v in cc["translation_i18n"].items():
        out.append("    %s: {status: %s, reason: %s}" % (k, v["status"], q(v["reason"])))
    out.append("  accessibility_wcag22_aa:")
    out.append("    applies: [%s]" % ", ".join(cc["accessibility_wcag22_aa"]["applies"]))
    out.append("    na:")
    for k, v in cc["accessibility_wcag22_aa"]["na"].items():
        out.append("      %s: %s" % (k, q(v)))
    out.append("    open:")
    for k, v in cc["accessibility_wcag22_aa"]["open"].items():
        out.append("      %s: %s" % (k, q(v)))
    return "\n".join(out) + "\n"


def main(argv):
    repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
    out = os.path.join("specs", "001-full-project-audit-remediation", "matrix", "applicability.yaml")
    i = 0
    while i < len(argv):
        if argv[i] == "--repo": repo = os.path.abspath(argv[i + 1]); i += 2
        elif argv[i] == "--out": out = argv[i + 1]; i += 2
        else: sys.exit("usage: derive_applicability.py [--repo DIR] [--out FILE|-]")
    text = render(repo)
    if out == "-":
        sys.stdout.write(text)
    else:
        path = out if os.path.isabs(out) else os.path.join(repo, out)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(text)
        print("derive_applicability: wrote %s (%d bytes)" % (path, len(text)))


if __name__ == "__main__":
    main(sys.argv[1:])
