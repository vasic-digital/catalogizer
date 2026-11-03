#!/usr/bin/env python3
"""Classify web test files as REAL / MOCKED / MIXED (constitution 11.4.289).

A test is MOCKED when a test double is in its path: fixtures/api-mocks helpers,
page/context.route interception, route.fulfill, msw, init-script fetch/XHR/Tauri
bridge overrides, or a file-level helper that does one of those. Mocked = unit-class
and can never be cited as live evidence. A test with none of those is REAL
(candidate: the absence of a double is necessary, not sufficient; a live run with
machine fields per 11.4.289(D) is still required).

File class: all tests MOCKED -> MOCKED; all REAL -> REAL; both -> MIXED.
Non-test drivers: SELFTEST (setContent only, no stack), HELPER (no browser driving).

Usage:
  classify_web_tests.py --root REPO [--out-json F] [--out-md F]
  classify_web_tests.py --root REPO --files a.spec.ts b.spec.ts --json-only
Exit 0 on success; 2 when nothing was classified (BLIND, never a clean zero).
Documentation: docs/qa/e2e-triage-20261103/README.md
"""
import argparse, json, os, re, sys, pathlib

SKIP_DIRS = {"node_modules", ".git", "submodules", "constitution", "dist", "build", "playwright-report", "test-results", "testdata"}
SCOPES = [  # (root-relative dir, glob suffixes)
    ("catalog-web/e2e", (".spec.ts", ".spec.tsx")),
    ("catalogizer-desktop/e2e", (".spec.ts",)),
    ("Website", (".spec.ts",)),
    ("installer-wizard", (".spec.ts",)),
    ("tests/responsive-audit", (".mjs",)),
]
DOMAINS = {
    "playback": r"playback|\bplayer\b|<video|<audio|\bvideo\b|\baudio\b|media-playback|seek|currentTime",
    "login": r"\blogin\b|sign.?in|/auth/login",
    "registration": r"regist|sign.?up|/auth/register",
    "collections": r"collection",
    "admin": r"\badmin\b|settings-admin|backup|/admin",
}
MOCK_HELPER_CALL = re.compile(r"\bmock[A-Z]\w*\s*\(")
ROUTE = re.compile(r"\b(?:page|context|ctx|browserContext|\w*[cC]ontext)\.route(?:FromHAR|WebSocket)?\s*\(")
FULFILL = re.compile(r"\broute\.fulfill\s*\(|\.fulfill\s*\(")
MSW = re.compile(r"\bsetupServer\b|\bsetupWorker\b|from\s+['\"]msw")
FAKE_SERVER = re.compile(r"\bhttp\.createServer\s*\(|\bnew\s+WebSocketServer\b")
INIT_DOUBLE = re.compile(r"addInitScript[\s\S]{0,700}?(?:__TAURI_INTERNALS__|window\.fetch\s*=|globalThis\.fetch\s*=|XMLHttpRequest\.prototype|window\.WebSocket\s*=)")
JEST_MOCK = re.compile(r"\b(?:vi|jest)\.mock\s*\(")
REQUEST_API = re.compile(r"\brequest\.(?:get|post|put|patch|delete|fetch|head)\s*\(")
GOTO = re.compile(r"\.goto\s*\(")
SETCONTENT = re.compile(r"\.setContent\s*\(")
PW_IMPORT = re.compile(r"@playwright/test|playwright-core|from\s+['\"]playwright['\"]")
MOCK_IMPORT = re.compile(r"""from\s+['"][^'"]*(?:api-mocks|fixtures/auth)['"]""")
STATUS_ERR = re.compile(r"status\s*:\s*([45]\d\d)")
ROUTE_URL = re.compile(r"""\.route\s*\(\s*(['"`])(.*?)\1""")


def strip_code(src):
    """Return (code_no_comments_keep_strings, code_blank_strings), both line/offset preserving."""
    n = len(src)
    keep = list(src)
    blank = list(src)
    i = 0
    state = None  # None, ', ", `, //, /*
    while i < n:
        c = src[i]
        nx = src[i + 1] if i + 1 < n else ""
        if state is None:
            if c == "/" and nx == "/":
                state = "//"; keep[i] = keep[i + 1] = blank[i] = blank[i + 1] = " "; i += 2; continue
            if c == "/" and nx == "*":
                state = "/*"; keep[i] = keep[i + 1] = blank[i] = blank[i + 1] = " "; i += 2; continue
            if c in "'\"`":
                state = c; i += 1; continue
            i += 1
        elif state == "//":
            if c == "\n":
                state = None
            else:
                keep[i] = blank[i] = " "
            i += 1
        elif state == "/*":
            if c == "*" and nx == "/":
                keep[i] = keep[i + 1] = blank[i] = blank[i + 1] = " "; i += 2; state = None; continue
            if c != "\n":
                keep[i] = blank[i] = " "
            i += 1
        else:  # inside string
            if c == "\\":
                if c != "\n": blank[i] = " "
                if i + 1 < n and src[i + 1] != "\n": blank[i + 1] = " "
                i += 2; continue
            if c == state:
                state = None; i += 1; continue
            if c != "\n":
                blank[i] = " "
            i += 1
    return "".join(keep), "".join(blank)


def match_brace(blank, open_idx):
    depth = 0
    for j in range(open_idx, len(blank)):
        ch = blank[j]
        if ch == "{": depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0: return j
    return None


def body_span(blank, m_end):
    """Span of the callback body `{...}` following a call opening at m_end."""
    arrow = blank.find("=>", m_end)
    func = blank.find("function", m_end)
    cands = [x for x in (arrow, func) if x != -1]
    if not cands: return None
    start = min(cands)
    ob = blank.find("{", start)
    if ob == -1: return None
    cb = match_brace(blank, ob)
    return (ob, cb) if cb else None


def line_of(src, idx):
    return src.count("\n", 0, idx) + 1


def parse_nodes(src, keep, blank):
    nodes = []  # dict(kind, title, start, end)
    pats = [
        ("test", re.compile(r"\b(?:test|it)(?:\.(?:only|skip|fixme|slow|fail))?\s*\(\s*(['\"`])")),
        ("describe", re.compile(r"\b(?:test\.)?describe(?:\.(?:serial|parallel|only|skip|fixme))?\s*\(\s*(['\"`])")),
        ("hook", re.compile(r"\b(?:test\.)?(?:beforeEach|beforeAll)\s*\(")),
    ]
    for kind, rx in pats:
        for m in rx.finditer(blank):
            # `test.describe(` / `test.beforeEach(` must not be read as a test
            if kind == "test":
                pre = blank[max(0, m.start() - 0):m.end()]
                if re.match(r"test\.(describe|beforeEach|beforeAll|afterEach|afterAll|use|setTimeout|step|skip\(\s*(?!['\"`]))", pre): continue
                if re.match(r"test\.(describe|beforeEach|beforeAll|afterEach|afterAll|use|setTimeout|step)\b", pre): continue
            sp = body_span(blank, m.end())
            if not sp: continue
            title = ""
            if kind != "hook":
                q = m.group(1)
                tstart = m.end()
                tend = src.find(q, tstart)
                title = src[tstart:tend] if tend != -1 else ""
            nodes.append({"kind": kind, "title": title, "start": sp[0], "end": sp[1], "kw": m.start()})
    return nodes


def file_helpers(blank, nodes):
    """File-level helper functions: name -> (start, end)."""
    out = {}
    for m in re.finditer(r"(?:async\s+)?function\s+(\w+)\s*\(", blank):
        ob = blank.find("{", m.end())
        cb = match_brace(blank, ob) if ob != -1 else None
        if cb and not any(n["start"] <= m.start() <= n["end"] for n in nodes):
            out[m.group(1)] = (ob, cb)
    for m in re.finditer(r"\bconst\s+(\w+)\s*=\s*(?:async\s*)?(?:\([^)]*\)|\w+)\s*=>", blank):
        ob = blank.find("{", m.end())
        if ob != -1 and blank[m.end():ob].strip() == "":
            cb = match_brace(blank, ob)
            if cb and not any(n["start"] <= m.start() <= n["end"] for n in nodes):
                out[m.group(1)] = (ob, cb)
    return out


def mock_signal_positions(keep, blank):
    sigs = []  # (pos, label)
    for rx, lab in ((MOCK_HELPER_CALL, "mock-helper"), (ROUTE, "route-intercept"), (FULFILL, "route.fulfill"),
                    (MSW, "msw"), (FAKE_SERVER, "fake-server"), (JEST_MOCK, "vi/jest.mock")):
        for m in rx.finditer(blank):
            sigs.append((m.start(), lab))
    for m in INIT_DOUBLE.finditer(keep):
        # exclude matches whose "fetch" is only within a comment-free but unrelated area: accept
        sigs.append((m.start(), "init-script-double"))
    return sigs


def classify_spec(path, rel):
    src = pathlib.Path(path).read_text(errors="replace")
    keep, blank = strip_code(src)
    nodes = parse_nodes(src, keep, blank)
    tests = [n for n in nodes if n["kind"] == "test"]
    helpers = file_helpers(blank, nodes)
    sigs = mock_signal_positions(keep, blank)
    mocked_helpers = {name for name, (s, e) in helpers.items() if any(s <= p <= e for p, _ in sigs)}
    # helper call sites become signals
    for name in mocked_helpers:
        for m in re.finditer(r"\b" + re.escape(name) + r"\s*\(", blank):
            hs, he = helpers[name]
            if hs <= m.start() <= he: continue
            sigs.append((m.start(), "mock-helper:" + name))
    imports_mocks = bool(MOCK_IMPORT.search(keep))
    results = []
    for t in tests:
        reasons = []
        for p, lab in sigs:
            inside = t["start"] <= p <= t["end"]
            inherited = False
            if not inside:
                for h in nodes:
                    if h["kind"] == "hook" and h["start"] <= p <= h["end"]:
                        # hook applies to tests sharing the hook's enclosing scope
                        parents = [d for d in nodes if d["kind"] == "describe" and d["start"] < h["start"] < d["end"]]
                        scope = min(parents, key=lambda d: d["end"] - d["start"]) if parents else None
                        if scope is None or (scope["start"] < t["start"] < scope["end"]):
                            inherited = True
                            lab = lab + "(hook)"
                            break
            if inside or inherited:
                reasons.append((line_of(src, p), lab))
        body = blank[t["start"]:t["end"]]
        keep_body = keep[t["start"]:t["end"]]
        real_hits = []
        if REQUEST_API.search(body): real_hits.append("direct API request (request.*)")
        if GOTO.search(body): real_hits.append("page.goto")
        results.append({
            "title": t["title"], "line": line_of(src, t["kw"]),
            "class": "MOCKED" if reasons else "REAL",
            "mock_evidence": sorted(set(f"L{l}:{lab}" for l, lab in reasons))[:6],
            "partial_real": bool(reasons) and bool(REQUEST_API.search(body)),
            "io": real_hits,
        })
    return src, keep, blank, results, imports_mocks, sigs


def domains_for(rel, titles, keep):
    tags = {}
    hay_file = rel.lower()
    for d, rx in DOMAINS.items():
        r = re.compile(rx, re.I)
        title_hits = [t["title"] for t in titles if r.search(t["title"])]
        file_hit = bool(r.search(hay_file))
        body_hits = len(r.findall(keep))
        if title_hits or file_hit:
            tags[d] = {"file_name_match": file_hit, "test_titles_matching": len(title_hits), "examples": title_hits[:3]}
    return tags


def make_live_plan(rel, f, keep, sigs_labels, src):
    """Mechanical to-do list to make a MOCKED/MIXED spec LIVE."""
    plan = []
    labs = " ".join(sigs_labels)
    if "mock-helper" in labs:
        plan.append("Remove mockAuthEndpoints/mockAllEndpoints/mockDashboard|Media|CollectionEndpoints calls (e2e/fixtures/api-mocks.ts, e2e/fixtures/auth.ts); drive the real backend instead.")
    if "route" in labs:
        urls = sorted(set(m.group(2) for m in ROUTE_URL.finditer(keep)))[:6]
        plan.append("Delete page.route/route.fulfill interception" + (f" (urls: {', '.join(urls)})" if urls else "") + "; assert on real responses.")
    if "init-script-double" in labs or "fake-server" in labs or "msw" in labs:
        plan.append("Replace the init-script/fake-server/msw double with the real service.")
    errs = sorted(set(STATUS_ERR.findall(keep)))
    if errs:
        plan.append(f"Mocked error statuses {errs}: live equivalent needs real fault injection (bad credentials, stop a fixture, expired token) - L11 - or the case stays legitimately unit-class.")
    if re.search(r"mockMedia|Test Movie|Test Song|test-movie", keep, re.I) or "mockMedia" in src:
        plan.append("Assertions depend on mock fixture rows (mockMedia); re-point them at the seeded L1 corpus manifest (counts and titles from scripts/test-infra/seed_corpus.sh).")
    if re.search(r"toHaveScreenshot|screenshot", keep):
        plan.append("Visual snapshots were taken against mock data; regenerate baselines against the seeded live corpus (deterministic seed required) and record the artifact fingerprint.")
    if re.search(r"waitForTimeout|performance|load ?time|Core Web", keep, re.I):
        plan.append("Timing thresholds were measured against instant mocks; re-baseline against the live API on the target host (report p50/p95, N>=3).")
    if re.search(r"WebSocket|\bws\b|realtime", keep):
        plan.append("WebSocket behaviour: connect to the real /ws endpoint and trigger a real server event (scan/convert job) instead of a scripted frame.")
    plan.append("Use real login with requireEnv('ADMIN_PASSWORD') (pattern: e2e/tests/browsing-challenge.spec.ts loginAsAdmin) and seed any needed data through the real API first (pattern: real-backend-pages.spec.ts request.post /collections), cleaning up after.")
    plan.append("Add the file to testMatch in catalog-web/playwright.real.config.ts (currently only real-backend-pages.spec.ts) and run with PLAYWRIGHT_BASE_URL=<stack>; record target fingerprint + endpoint identity per 11.4.289(D).")
    return plan


def classify_driver(path, rel):
    src = pathlib.Path(path).read_text(errors="replace")
    keep, blank = strip_code(src)
    sigs = mock_signal_positions(keep, blank)
    has_pw = bool(PW_IMPORT.search(keep))
    if not has_pw:
        return {"class": "HELPER", "why": "no browser automation; infrastructure helper (serve.mjs-style static server/reverse proxy to the real API)" if "createServer" in keep else "no browser automation", "signals": [], "src": src, "keep": keep}
    if SETCONTENT.search(keep) and not GOTO.search(keep):
        return {"class": "SELFTEST", "why": "page.setContent on synthetic HTML; oracle self-test (golden-good/bad), not evidence about the product", "signals": [], "src": src, "keep": keep}
    labs = [f"L{line_of(src, p)}:{lab}" for p, lab in sigs]
    if labs:
        return {"class": "MIXED", "why": "drives the real app and real API but replaces part of the system with a double (see signals); the double is unit-class", "signals": labs, "src": src, "keep": keep}
    return {"class": "REAL", "why": "drives the real built app against the real API (token from a real login)", "signals": [], "src": src, "keep": keep}


def collect(root):
    files = []
    for rel_dir, suffixes in SCOPES:
        base = pathlib.Path(root) / rel_dir
        if not base.exists(): continue
        for dp, dns, fns in os.walk(base):
            dns[:] = [d for d in dns if d not in SKIP_DIRS]
            for fn in fns:
                if fn.endswith(suffixes):
                    files.append(str(pathlib.Path(dp) / fn))
    return sorted(files)


def process(path, root):
    rel = os.path.relpath(path, root) if str(path).startswith(str(root)) else os.path.basename(path)
    if path.endswith(".mjs"):
        d = classify_driver(path, rel)
        out = {"path": rel, "kind": "audit-script", "class": d["class"], "why": d["why"], "mock_signals": d["signals"],
               "tests_total": 0, "tests_real": 0, "tests_mocked": 0, "domains": domains_for(rel, [], d["keep"])}
        if d["class"] == "MIXED":
            out["make_live"] = ["Replace the emulated Tauri IPC bridge (window.__TAURI_INTERNALS__) with the real Tauri window (built desktop artifact under Xvfb) for the desktop target; web/installer targets are already live."] if "__TAURI" in d["src"] else ["Remove the listed double."]
        return out
    src, keep, blank, results, imports_mocks, sigs = classify_spec(path, rel)
    nm = sum(1 for r in results if r["class"] == "MOCKED")
    nr = len(results) - nm
    if not results: cls = "NOTESTS"
    elif nm and nr: cls = "MIXED"
    elif nm: cls = "MOCKED"
    else: cls = "REAL"
    labs = sorted({lab for _, lab in sigs})
    out = {
        "path": rel, "kind": "playwright-spec", "class": cls,
        "tests_total": len(results), "tests_real": nr, "tests_mocked": nm,
        "imports_mock_fixtures": imports_mocks,
        "mock_signal_lines": sorted({f"L{line_of(src, p)}:{lab}" for p, lab in sigs})[:12],
        "tests": results,
        "api_touched_by_real_tests": sorted({io for r in results if r["class"] == "REAL" for io in r["io"]}),
        "domains": domains_for(rel, results, keep),
    }
    if cls in ("MOCKED", "MIXED"):
        out["make_live"] = make_live_plan(rel, out, keep, labs, src)
    if cls == "REAL" and not any("page.goto" in r["io"] or "direct API request (request.*)" in r["io"] for r in results):
        out["note"] = "no mock found but also no goto/request in test bodies; verify it touches the stack"
    if cls == "REAL" and not re.search(r"ADMIN_PASSWORD|request\.|/api/|login", keep):
        out["note"] = "REAL = no test double; this spec does not touch the API at all (UI/theme only)"
    return out


def to_md(data):
    c = data["counts"]
    L = ["# Web test classification (11.4.289)", "",
         f"Files: {c['files']}  |  REAL {c['REAL']}  MOCKED {c['MOCKED']}  MIXED {c['MIXED']}  other {c['other']}",
         f"Tests (playwright specs): {c['tests_total']}  |  real {c['tests_real']}  mocked {c['tests_mocked']}", "",
         "| File | Class | Tests (real/mocked) | Domains | Evidence (file:line) |", "|---|---|---|---|---|"]
    for f in data["files"]:
        dom = ",".join(sorted(f["domains"])) or "-"
        ev = ", ".join(f.get("mock_signal_lines", f.get("mock_signals", []))[:3]) or "-"
        L.append(f"| {f['path']} | {f['class']} | {f['tests_real']}/{f['tests_mocked']} | {dom} | {ev} |")
    L += ["", "## Domain coverage (playback / login / registration / collections / admin)", ""]
    for d in DOMAINS:
        fs = [f for f in data["files"] if d in f["domains"]]
        L.append(f"- **{d}**: " + (", ".join(f"{f['path'].split('/e2e/')[-1]} [{f['class']}]" for f in fs) or "none"))
    L += ["", "## Make-live plan per MOCKED/MIXED file", ""]
    for f in data["files"]:
        if f.get("make_live"):
            L.append(f"### {f['path']} ({f['class']})")
            L += [f"- {s}" for s in f["make_live"]]
            L.append("")
    return "\n".join(L) + "\n"


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--root", required=True)
    ap.add_argument("--files", nargs="*")
    ap.add_argument("--out-json"); ap.add_argument("--out-md")
    ap.add_argument("--json-only", action="store_true", help="print JSON to stdout")
    a = ap.parse_args()
    root = os.path.abspath(a.root)
    paths = [os.path.abspath(p) for p in a.files] if a.files else collect(root)
    if not paths:
        print("BLIND: no test files found in scope", file=sys.stderr); return 2
    files = [process(p, root) for p in paths]
    other = sum(1 for f in files if f["class"] not in ("REAL", "MOCKED", "MIXED"))
    data = {"constitution": "11.4.289", "root": root, "files": files, "counts": {
        "files": len(files), "REAL": sum(f["class"] == "REAL" for f in files), "MOCKED": sum(f["class"] == "MOCKED" for f in files),
        "MIXED": sum(f["class"] == "MIXED" for f in files), "other": other,
        "tests_total": sum(f["tests_total"] for f in files), "tests_real": sum(f["tests_real"] for f in files),
        "tests_mocked": sum(f["tests_mocked"] for f in files)}}
    if a.json_only:
        print(json.dumps(data, indent=1)); return 0
    if a.out_json:
        pathlib.Path(a.out_json).write_text(json.dumps(data, indent=1) + "\n")
    if a.out_md and a.out_md != os.devnull:
        pathlib.Path(a.out_md).write_text(to_md(data))
    print(json.dumps(data["counts"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
