#!/usr/bin/env python3
"""route_drift.py - read-only API drift detector (POC for spec 001, FR-016 / SC-008).

Compares three views of the HTTP API and prints ONE JSON document:
  SERVER  Go routes in catalog-api (gin: <group>.GET/POST/PUT/DELETE/PATCH/HEAD/Any("path", ...),
          Group("/x") nesting; gorilla/mux: PathPrefix(..).Subrouter() + HandleFunc("/p", h).Methods(..))
  SPEC    OpenAPI operations (docs/api/openapi.yaml, every file given with --spec)
  CLIENT  catalogizer-api-client/src (axios wrapper), catalog-web/src (axios `api` + fetch),
          Android and Android TV Retrofit interfaces (@GET/@POST/@PUT/@DELETE/@PATCH/@HTTP)

Reported sets:
  undocumented_routes        in SERVER, not in SPEC
  stale_spec_entries         in SPEC, not in SERVER (annotated when only an UNWIRED mux route would serve it)
  unwired_mux_routes         mux routes in a RegisterRoutes() that has no call site in non-test code
  client_calls_without_route per client: a call with no SERVER route (wildcard/segment aware)
  double_prefix_calls        web calls that start with /api/v1 although the axios baseURL already adds /api/v1
  unresolved                 constructs the extractor could not resolve (listed, never silently dropped)

NORMALISATION RULES (all applied before comparing; keys look like "GET /api/v1/favorites/{}"):
  N1  method upper-case; OPTIONS and HEAD are ignored on every side (CORS pre-flight / auto-generated).
      gin Any() and mux routes without .Methods() become method ANY and match every method.
  N2  every path parameter becomes "{}": gin ":id", mux "{id}" or "{id:[0-9]+}", OpenAPI "{id}",
      TypeScript "${expr}", Kotlin/Retrofit "{id}". Parameter NAMES are never compared.
  N3  gin catch-all "*path" becomes the terminal token "{}" for the SERVER/SPEC key comparison
      and ALSO sets catchall=true so a client path with more segments still matches.
  N4  group prefixes are concatenated (router.Group("/api/v1") then api.Group("/smb")); a variable that
      is re-bound later in the same file uses its latest binding (sequential reading, no block scoping).
  N5  a trailing "/" is dropped (except the root "/"); query strings and fragments are cut.
  N6  routes under --exclude-prefix (default /debug/pprof) are ignored.
  N7  client base prefixes: api-client --api-client-prefix (default /api/v1, UNCONFIRMED: it is a
      constructor argument of HttpClient), web /api/v1 (catalog-web/src/lib/api.ts baseURL), Android /api/v1
      (DependencyContainer appends it) but Android TV "/" (its DependencyContainer uses baseUrl("<url>/") and its
      interface paths already begin with api/v1; UNCONFIRMED whether users enter a URL ending in /api/v1); Retrofit paths are relative. A Retrofit path
      starting with "/" is host-absolute and is NOT prefixed. A fetch() literal beginning "/api/" is
      absolute; a leading "${BASE}" expression is treated as the host and dropped (host_expr noted).
  N8  client segment matching: a client "{}" segment matches ANY server segment (literal or {}); a literal
      client segment must equal a literal server segment or meet a server "{}".
Only calls whose first argument is a string/template literal starting with "/" (TS) are captured, from an
allow-list of receivers, which removes Map.get('k')-style noise.

FALSE-POSITIVE / FALSE-NEGATIVE CAVEATS (stated, not hidden):
  * regex extraction over source text, not the running router: routes added through helper functions,
    reflection, loops, or built path strings are not seen (listed under unresolved when detectable);
  * routes inside `if` blocks are counted as registered (a conditionally registered route may be absent at
    run time); comments are stripped, string contents are not parsed;
  * a mux RegisterRoutes() is "unwired" only if NO call to `.RegisterRoutes(` exists in non-test Go code;
    interface dispatch or generated code could still wire it (UNCONFIRMED by type);
  * dynamic client URLs (variables, concatenation) are not captured; TS generic arguments containing
    parentheses can hide a call; Kotlin routes via @Url are unresolved;
  * the spec may legitimately omit routes (health, metrics, WebSocket) - review each item, the tool does not decide.
The authoritative fix is a route dump from the running router (router.Routes()) - this tool is the cheap approximation.

Usage: route_drift.py [--root DIR] [--spec FILE ...] [--api-client-prefix /api/v1] [--self-test] [--summary]
Exit: 0 report produced; with --fail-on-drift exit 1 if undocumented/stale/client-no-route/double-prefix is non-empty.
"""
import argparse
import glob
import json
import os
import re
import sys
import tempfile

try:
    import yaml
except ImportError:  # minimal fallback: only used for the paths block
    yaml = None

METHODS = ("GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS")


# ---------------------------------------------------------------------------- helpers
def strip_go_comments(src):
    out, i, n, mode = [], 0, len(src), None
    while i < n:
        c = src[i]
        nxt = src[i + 1] if i + 1 < n else ""
        if mode is None:
            if c == "/" and nxt == "/":
                while i < n and src[i] != "\n":
                    i += 1
                continue
            if c == "/" and nxt == "*":
                i += 2
                while i < n - 1 and not (src[i] == "*" and src[i + 1] == "/"):
                    out.append("\n" if src[i] == "\n" else " ")
                    i += 1
                i += 2
                continue
            if c in "\"`":
                mode = c
            elif c == "'":
                mode = "'"
            out.append(c)
        else:
            out.append(c)
            if c == "\\" and mode != "`":
                out.append(nxt); i += 2; continue
            if c == mode:
                mode = None
        i += 1
    return "".join(out)


def norm_path(p):
    p = re.sub(r"\?.*$", "", p)
    p = re.sub(r"#.*$", "", p)
    p = re.sub(r"\{[^}]*\}", "{}", p)           # N2 (mux, openapi, kotlin, normalised ts)
    p = re.sub(r":[A-Za-z_][A-Za-z0-9_]*", "{}", p)  # gin :id
    p = re.sub(r"/\*[A-Za-z_]*", "/{}", p)      # N3 catch-all key form
    p = re.sub(r"/+", "/", p)
    if len(p) > 1 and p.endswith("/"):
        p = p[:-1]
    return p or "/"


def join(prefix, path):
    return (prefix.rstrip("/") + "/" + path.lstrip("/")) if path not in ("", "/") else (prefix or "/")


def line_of(text, pos):
    return text.count("\n", 0, pos) + 1


# ---------------------------------------------------------------------------- SERVER
def go_files(root):
    base = os.path.join(root, "catalog-api")
    for d, ds, fs in os.walk(base):
        ds[:] = [x for x in ds if x not in ("node_modules", "vendor", ".git", "tests", "testdata")]
        for f in sorted(fs):
            if f.endswith(".go") and not f.endswith("_test.go"):
                p = os.path.join(d, f)
                rel = os.path.relpath(p, root).replace(os.sep, "/")
                if "/internal/tests/" in "/" + rel:
                    continue
                yield rel, p


GIN_GROUP = re.compile(r"(\w+)\s*:?=\s*(\w+)\.Group\(\s*\"([^\"]*)\"")
GIN_ROOT = re.compile(r"(\w+)\s*:?=\s*gin\.(?:Default|New)\(")
GIN_ROUTE = re.compile(r"(\w+)\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|Any)\(\s*\"([^\"]*)\"")
MUX_SUB = re.compile(r"(\w+)\s*:?=\s*(\w+)\.PathPrefix\(\s*\"([^\"]*)\"\s*\)\.Subrouter\(\)")
MUX_ROUTE = re.compile(r"(\w+)\.HandleFunc\(\s*\"([^\"]*)\"[^\n]*?\)\s*(?:\.Methods\(([^)]*)\))?")
FUNC = re.compile(r"^func\s*(?:\(\s*\w+\s+\*?(\w+)\s*\)\s*)?(\w+)\s*\(", re.M)


def extract_server(root, exclude_prefix):
    routes, unresolved, unwired, call_sites = [], [], [], []
    mux_by_type = {}
    sources = list(go_files(root))
    for rel, p in sources:
        src = strip_go_comments(open(p, encoding="utf-8", errors="replace").read())
        for m in re.finditer(r"\.RegisterRoutes\(", src):
            call_sites.append("%s:%d" % (rel, line_of(src, m.start())))
    # function boundaries for mux receiver typing
    for rel, p in sources:
        src = strip_go_comments(open(p, encoding="utf-8", errors="replace").read())
        groups = {"router": ""}
        for m in GIN_ROOT.finditer(src):
            groups[m.group(1)] = ""
        events = []
        for m in GIN_GROUP.finditer(src):
            events.append((m.start(), "group", m))
        for m in GIN_ROUTE.finditer(src):
            events.append((m.start(), "route", m))
        for m in MUX_SUB.finditer(src):
            events.append((m.start(), "muxsub", m))
        for m in MUX_ROUTE.finditer(src):
            events.append((m.start(), "mux", m))
        funcs = [(m.start(), m.group(1), m.group(2)) for m in FUNC.finditer(src)]
        events.sort(key=lambda e: e[0])
        for pos, kind, m in events:
            ln = line_of(src, pos)
            if kind == "group":
                parent = m.group(2)
                if parent in groups:
                    groups[m.group(1)] = join(groups[parent], m.group(3))
                else:
                    unresolved.append({"file": rel, "line": ln, "what": "Group on unknown receiver '%s'" % parent})
            elif kind == "muxsub":
                groups[m.group(1)] = join(groups.get(m.group(2), ""), m.group(3))
            elif kind == "route":
                recv, meth, path = m.group(1), m.group(2).upper(), m.group(3)
                if recv not in groups:
                    if path.startswith("/"):   # zap.Any("event", ...) style calls are not routes
                        unresolved.append({"file": rel, "line": ln, "what": "route %s %s on unknown receiver '%s'" % (meth, path, recv)})
                    continue
                full = join(groups[recv], path)
                if full.startswith(exclude_prefix) or meth in ("HEAD", "OPTIONS"):
                    continue
                routes.append({"method": "ANY" if meth == "ANY" else meth, "raw": full, "path": norm_path(full),
                               "catchall": bool(re.search(r"/\*[A-Za-z_]*$", full)), "file": rel, "line": ln, "source": "gin"})
            elif kind == "mux":
                recv, path, ms = m.group(1), m.group(2), m.group(3)
                if recv not in groups:
                    unresolved.append({"file": rel, "line": ln, "what": "mux HandleFunc on unknown receiver '%s'" % recv})
                    continue
                full = join(groups[recv], path)
                meths = [x.strip().strip('"').upper() for x in ms.split(",")] if ms else ["ANY"]
                owner = None
                for fp, ft, fn in funcs:
                    if fp <= pos:
                        owner = (ft, fn)
                for me in meths:
                    if me in ("OPTIONS", "HEAD"):
                        continue
                    r = {"method": me, "raw": full, "path": norm_path(full), "catchall": False, "file": rel,
                         "line": ln, "source": "mux", "owner": "%s.%s" % owner if owner and owner[0] else (owner[1] if owner else "?")}
                    mux_by_type.setdefault(owner[0] if owner else "?", []).append(r)
    # wiring decision for mux routes: a RegisterRoutes() of type T is WIRED when a call can be linked to T
    # (var := ...NewT(...) / &T{...} followed by var.RegisterRoutes(, or NewT(...).RegisterRoutes( inline).
    allsrc = "\n".join(strip_go_comments(open(p, encoding="utf-8", errors="replace").read()) for _, p in sources)
    reg_calls = len(call_sites)
    for typ, rs in mux_by_type.items():
        linked = False
        if typ != "?":
            for m in re.finditer(r"(\w+)\s*:?=\s*(?:&?%s\{|[\w.]*New%s\()" % (typ, typ), allsrc):
                if re.search(r"\b%s\.RegisterRoutes\(" % re.escape(m.group(1)), allsrc):
                    linked = True
            if re.search(r"New%s\([^;]*\)\.RegisterRoutes\(" % typ, allsrc):
                linked = True
        own_fn = rs[0].get("owner", "").split(".")[-1]
        if own_fn != "RegisterRoutes" or linked:
            routes.extend(rs)          # not a registration entry point, or a call was linked
        else:
            for r in rs:
                r["confidence"] = "no_call_sites" if reg_calls == 0 else "calls_exist_but_none_linked_to_%s" % typ
            unwired.extend(rs)
    return routes, unwired, unresolved, {"register_routes_call_sites": call_sites, "go_files_scanned": len(sources)}


# ---------------------------------------------------------------------------- SPEC
def extract_spec(root, spec_files):
    ops, notes = [], []
    for sf in spec_files:
        p = os.path.join(root, sf)
        if not os.path.exists(p):
            notes.append("spec file missing: " + sf); continue
        text = open(p, encoding="utf-8", errors="replace").read()
        paths = None
        if yaml:
            try:
                paths = (yaml.safe_load(text) or {}).get("paths") or {}
            except Exception as e:  # noqa
                notes.append("yaml parse failed for %s: %s (falling back to line scan)" % (sf, e))
        if paths is not None:
            for pth, item in paths.items():
                if not isinstance(item, dict):
                    continue
                for k in item:
                    if k.upper() in METHODS and k.upper() not in ("HEAD", "OPTIONS"):
                        ops.append({"method": k.upper(), "raw": pth, "path": norm_path(pth), "file": sf})
        else:
            cur, inpaths = None, False
            for line in text.split("\n"):
                if re.match(r"^paths:\s*$", line): inpaths = True; continue
                if inpaths and re.match(r"^\S", line): inpaths = False
                if not inpaths: continue
                m = re.match(r"^  (/[^\s:]*):\s*$", line)
                if m: cur = m.group(1); continue
                m = re.match(r"^    (get|post|put|delete|patch):\s*$", line)
                if m and cur: ops.append({"method": m.group(1).upper(), "raw": cur, "path": norm_path(cur), "file": sf})
    return ops, notes


# ---------------------------------------------------------------------------- CLIENTS
TS_CALL = re.compile(r"(?P<recv>[\w.]+)\.(?P<m>get|post|put|delete|patch)\s*(?:<(?:[^()<>]|<[^()<>]*>)*>)?\s*\(\s*(?P<q>[`'\"])(?P<lit>(?:\\.|(?!(?P=q)).)*)(?P=q)", re.S)
TS_FETCH = re.compile(r"\bfetch\(\s*(?P<q>[`'\"])(?P<lit>(?:\\.|(?!(?P=q)).)*)(?P=q)\s*(?:,\s*\{(?P<opts>[^}]{0,300}))?", re.S)
RECV_OK = re.compile(r"^(?:this\.)?(?:http|client|api|apiClient|axios|instance|httpClient|authApi|request)$")


def ts_files(root, sub):
    base = os.path.join(root, sub)
    for d, ds, fs in os.walk(base):
        ds[:] = [x for x in ds if x not in ("node_modules", "__tests__", "dist", "build", ".git", "coverage")]
        for f in sorted(fs):
            if f.endswith((".ts", ".tsx")) and not re.search(r"\.(test|spec)\.|\.d\.ts$", f):
                p = os.path.join(d, f)
                yield os.path.relpath(p, root).replace(os.sep, "/"), p


def clean_ts_path(lit):
    host = False
    lit = re.sub(r"^\$\{[^}]*\}(?=/)", "", lit) if lit.startswith("${") and re.match(r"^\$\{[^}]*\}/", lit) else lit
    if lit != lit:  # pragma: no cover
        host = True
    lit = re.sub(r"\$\{[^}]*\}", "{}", lit)
    return lit


def extract_ts_client(root, sub, base_prefix, kind, unresolved, receivers_seen):
    calls, double = [], []
    for rel, p in ts_files(root, sub):
        src = open(p, encoding="utf-8", errors="replace").read()
        for m in TS_CALL.finditer(src):
            recv = m.group("recv")
            lit = m.group("lit")
            receivers_seen[recv] = receivers_seen.get(recv, 0) + 1
            if not RECV_OK.match(recv):
                continue
            if lit.startswith("${") and not re.match(r"^\$\{[^}]*\}/", lit):
                unresolved.append({"file": rel, "line": line_of(src, m.start()), "what": "path begins with an expression: " + lit[:60]})
                continue
            lit2 = re.sub(r"^\$\{[^}]*\}(?=/)", "", lit)
            lit2 = re.sub(r"\$\{[^}]*\}", "{}", lit2)
            if not lit2.startswith("/"):
                continue  # Map.get('key') style noise
            if lit2.startswith("/{}") :
                unresolved.append({"file": rel, "line": line_of(src, m.start()), "what": "first path segment is dynamic (base path built elsewhere): " + lit[:60]})
                continue
            raw = lit2
            dbl = kind == "web" and re.match(r"^/api/v\d+(/|$)", raw) is not None
            full = raw if (raw.startswith("/api/") and kind != "web") else join(base_prefix, raw)
            rec = {"method": m.group("m").upper(), "raw": raw, "path": norm_path(full), "file": rel,
                   "line": line_of(src, m.start()), "client": kind}
            calls.append(rec)
            if dbl:
                double.append(dict(rec, note="starts with /api/v1 but the axios instance baseURL already ends with /api/v1"))
        if kind == "web":
            for m in TS_FETCH.finditer(src):
                lit = m.group("lit")
                lit2 = re.sub(r"^\$\{[^}]*\}(?=/)", "", lit)
                lit2 = re.sub(r"\$\{[^}]*\}", "{}", lit2)
                if not lit2.startswith("/api/"):
                    if lit2.startswith("/") or lit.startswith("${"):
                        unresolved.append({"file": rel, "line": line_of(src, m.start()), "what": "fetch target not under /api/: " + lit[:60]})
                    continue
                opts = m.group("opts") or ""
                mm = re.search(r"method\s*:\s*['\"](\w+)['\"]", opts)
                calls.append({"method": (mm.group(1).upper() if mm else "GET"), "raw": lit2, "path": norm_path(lit2),
                              "file": rel, "line": line_of(src, m.start()), "client": "web-fetch",
                              "note": "method inferred from options within 300 chars, GET if absent"})
    return calls, double


KT_ANN = re.compile(r"@(GET|POST|PUT|DELETE|PATCH)\(\s*\"([^\"]*)\"\s*\)")
KT_HTTP = re.compile(r"@HTTP\(\s*method\s*=\s*\"(\w+)\"\s*,\s*path\s*=\s*\"([^\"]*)\"")


def extract_kotlin(root, sub, kind, unresolved, base_prefix="/api/v1"):
    calls = []
    double = []
    for d, ds, fs in os.walk(os.path.join(root, sub)):
        ds[:] = [x for x in ds if x not in ("build", ".gradle", "test", "androidTest", ".git")]
        for f in sorted(fs):
            if not f.endswith(".kt"): continue
            p = os.path.join(d, f)
            rel = os.path.relpath(p, root).replace(os.sep, "/")
            src = open(p, encoding="utf-8", errors="replace").read()
            src = re.sub(r"/\*.*?\*/", lambda m: "\n" * m.group(0).count("\n"), src, flags=re.S)
            src = re.sub(r"(?m)//.*$", "", src)
            for rx, mi, pi in ((KT_ANN, 1, 2), (KT_HTTP, 1, 2)):
                for m in rx.finditer(src):
                    path = m.group(pi)
                    meth = m.group(mi).upper()
                    if path.startswith("/"):
                        full = path           # host-absolute in Retrofit
                    else:
                        full = join(base_prefix, path)
                    rec = {"method": meth, "raw": path, "path": norm_path(full), "file": rel,
                           "line": line_of(src, m.start()), "client": kind}
                    calls.append(rec)
                    if base_prefix.rstrip("/") and re.match(r"^/?api/v\d+(/|$)", path) and base_prefix.rstrip("/") == "/api/v1":
                        double.append(dict(rec, note="relative path starts with api/v1 but this client's base URL already ends with /api/v1"))
            for m in re.finditer(r"@(GET|POST|PUT|DELETE|PATCH)\s*(?:\n|\s)+(?=suspend|fun)|@Url\b", src):
                unresolved.append({"file": rel, "line": line_of(src, m.start()), "what": "Retrofit @Url or bare annotation"})
    return calls, double


# ---------------------------------------------------------------------------- matching
def seg_match(client_segs, server, ):
    ss = [s for s in server["path"].split("/") if s != ""]
    cs = [s for s in client_segs if s != ""]
    if server.get("catchall"):
        if len(cs) < len(ss) - 1:
            return False
        ss_head, cs_head = ss[:-1], cs[:len(ss) - 1]
    else:
        if len(cs) != len(ss):
            return False
        ss_head, cs_head = ss, cs
    for a, b in zip(cs_head, ss_head):
        if a == "{}" or b == "{}":
            continue
        if a != b:
            return False
    return True


def route_for(call, servers):
    cs = [s for s in call["path"].split("/") if s != ""]
    for s in servers:
        if s["method"] not in ("ANY", call["method"]):
            continue
        if seg_match(cs, s):
            return s
    return None


def key(o):
    return "%s %s" % (o["method"], o["path"])


# ---------------------------------------------------------------------------- driver
def analyse(root, spec_files, api_client_prefix, exclude_prefix):
    servers, unwired, unresolved, sinfo = extract_server(root, exclude_prefix)
    spec, snotes = extract_spec(root, spec_files)
    ures = list(unresolved)
    recv_seen = {}
    c_api, _ = extract_ts_client(root, "catalogizer-api-client/src", api_client_prefix, "api-client", ures, recv_seen)
    c_web, dbl = extract_ts_client(root, "catalog-web/src", "/api/v1", "web", ures, recv_seen)
    c_and, dbl_and = extract_kotlin(root, "catalogizer-android/app/src/main", "android", ures, "/api/v1")
    c_tv, _ = extract_kotlin(root, "catalogizer-androidtv/app/src/main", "androidtv", ures, "/")
    dbl = dbl + dbl_and
    web_all = c_web

    skeys = {}
    for s in servers:
        skeys.setdefault(key(s), s)
    specmap = {}
    for o in spec:
        specmap.setdefault(key(o), o)
    # ANY routes expand: compare per method against spec methods
    def any_covered(k, collection):
        meth, path = k.split(" ", 1)
        return any(("%s %s" % (mm, path)) in collection for mm in ("GET", "POST", "PUT", "DELETE", "PATCH"))
    undocumented = []
    for k, s in sorted(skeys.items()):
        meth = s["method"]
        if k in specmap or (meth == "ANY" and any_covered(k, specmap)):
            continue
        undocumented.append({"key": k, "file": s["file"], "line": s["line"], "source": s["source"]})
    unwired_keys = {}
    for u in unwired:
        unwired_keys.setdefault(key(u), u)
    stale = []
    for k, o in sorted(specmap.items()):
        meth, path = k.split(" ", 1)
        if k in skeys or ("ANY " + path) in skeys:
            continue
        # tolerate gin catch-all vs spec single param (N3): server "{}" terminal catchall already in key
        item = {"key": k, "spec_file": o["file"]}
        if k in unwired_keys or ("ANY " + path) in unwired_keys:
            u = unwired_keys.get(k) or unwired_keys.get("ANY " + path)
            item["note"] = "served only by an UNWIRED mux route (%s:%d); no call to RegisterRoutes found" % (u["file"], u["line"])
        stale.append(item)
    def no_route(calls):
        out = []
        for c in calls:
            if route_for(c, [dict(s) for s in servers]) is None:
                hit = [u for u in unwired if u["method"] in ("ANY", c["method"]) and seg_match([x for x in c["path"].split("/") if x], u)]
                item = {"key": key(c), "file": c["file"], "line": c["line"]}
                if hit:
                    item["note"] = "matches only an UNWIRED mux route (%s:%d)" % (hit[0]["file"], hit[0]["line"])
                out.append(item)
        return out
    cwr = {"api_client": no_route(c_api), "web": no_route(web_all), "android": no_route(c_and), "androidtv": no_route(c_tv)}
    counts = {
        "server_routes_gin_and_wired_mux": len(servers),
        "server_unique_keys": len(skeys),
        "unwired_mux_routes": len(unwired),
        "spec_operations": len(spec), "spec_unique_keys": len(specmap),
        "undocumented_routes": len(undocumented), "stale_spec_entries": len(stale),
        "client_calls": {"api_client": len(c_api), "web": len(web_all), "android": len(c_and), "androidtv": len(c_tv)},
        "client_calls_without_route": {k: len(v) for k, v in cwr.items()},
        "double_prefix_calls": len(dbl), "unresolved": len(ures),
    }
    return {
        "tool": "route_drift.py", "root": os.path.abspath(root), "spec_files": spec_files,
        "api_client_prefix_assumed": api_client_prefix, "exclude_prefix": exclude_prefix,
        "counts": counts, "server_info": sinfo, "spec_notes": snotes,
        "undocumented_routes": undocumented, "stale_spec_entries": stale,
        "unwired_mux_routes": [{"key": key(u), "file": u["file"], "line": u["line"], "owner": u.get("owner"), "confidence": u.get("confidence")} for u in unwired],
        "client_calls_without_route": cwr, "double_prefix_calls": dbl, "unresolved": ures[:200],
        "receivers_seen_top": sorted(recv_seen.items(), key=lambda kv: -kv[1])[:15],
        "caveat": "heuristic regex extraction; see module docstring. Treat every item as a lead to confirm, not a verdict.",
    }


# ---------------------------------------------------------------------------- self-test
def _w(root, rel, text):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w", encoding="utf-8").write(text)


def self_test():
    ok = bad = 0
    def check(name, cond, detail=""):
        nonlocal ok, bad
        if cond: ok += 1; print("  PASS  " + name)
        else: bad += 1; print("  FAIL  " + name + " " + detail)
    with tempfile.TemporaryDirectory() as t:
        _w(t, "catalog-api/main.go", '''package main
func main() {
	router := gin.Default()
	router.GET("/health", h)
	// router.GET("/commented/out", h)
	s := "router.GET(\\"/in/string\\")"
	api := router.Group("/api/v1")
	api.GET("/catalog/*path", h)
	api.GET("/items/:id", h)
	api.POST("/items", h)
	api.Any("/anything", h)
	api.OPTIONS("/cors", h)
	smb := api.Group("/smb")
	smb.GET("/discover", h)
	{
		smb := api.Group("/other")
		smb.GET("/rebound", h)
	}
	pp := router.Group("/debug/pprof")
	pp.GET("/heap", h)
	ghost.GET("/unknown_receiver", h)
	w := handlers.NewWired(db)
	w.RegisterRoutes(r)
}
''')
        _w(t, "catalog-api/main_test.go", 'package main\nfunc T(){ router.GET("/only/in/test", h) }\n')
        _w(t, "catalog-api/internal/handlers/wired.go", '''package handlers
func (h *Wired) RegisterRoutes(router *mux.Router) {
	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/wired/x/{id}", h.X).Methods("GET", "OPTIONS")
}
''')
        _w(t, "catalog-api/internal/handlers/unwired.go", '''package handlers
func (h *Unwired) RegisterRoutes(router *mux.Router) {
	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/music/play", h.Play).Methods("POST", "OPTIONS")
}
''')
        _w(t, "docs/api/openapi.yaml", '''openapi: 3.0.3
paths:
  /health:
    get: {}
  /api/v1/catalog/{p}:
    get: {}
  /api/v1/items/{itemId}:
    get: {}
  /api/v1/items:
    post: {}
    delete: {}
  /api/v1/stale/only:
    get: {}
  /api/v1/music/play:
    post: {}
''')
        _w(t, "catalogizer-api-client/src/services/A.ts", '''
export class A { constructor(private http: HttpClient) {}
 a(){ return this.http.get<X>(`/items/${id}?limit=${n}`); }
 b(){ return this.http.post<void>('/items', {}); }
 c(){ return this.http.get('/nope/missing'); }
 d(){ return params.get('/not-a-call-because-receiver'); }
 e(){ return map.get('key'); }
}''')
        _w(t, "catalog-web/src/lib/w.ts", '''
export const w = {
  a: () => api.get(`/catalog/a/b/c`),
  b: () => api.get(`/api/v1/favorites?${q}`),
  c: () => fetch(`/api/v1/items/${id}`, { method: 'POST' }),
  d: () => api.post('/smb/discover'),
};''')
        _w(t, "catalogizer-android/app/src/main/java/x/Api.kt", '''
interface Api {
  @GET("items/{id}") suspend fun a(): R
  @POST("smb/connect") suspend fun b(): R
  // @GET("commented/out")
  @DELETE("/abs/path") suspend fun c(): R
  @HTTP(method = "DELETE", path = "items/{id}", hasBody = true) suspend fun d(): R
}''')
        _w(t, "catalogizer-androidtv/app/src/main/java/x/TvApi.kt", '''
interface TvApi {
  @GET("api/v1/items/{id}") suspend fun a(): R
  @GET("api/v1/tv/missing") suspend fun b(): R
}''')
        _w(t, "catalogizer-android/app/src/main/java/x/Api2.kt", '''
interface Api2 { @GET("api/v1/items/{id}") suspend fun dbl(): R }''')
        _w(t, "catalog-web/src/lib/dyn.ts", "export const d = () => api.get(`/${base}/x`)\n")
        _w(t, "catalog-api/internal/services/log.go", 'package s\nfunc f(){ zap.Any("event", e) }\n')
        r = analyse(t, ["docs/api/openapi.yaml"], "/api/v1", "/debug/pprof")
        und = {u["key"] for u in r["undocumented_routes"]}
        stale = {s["key"] for s in r["stale_spec_entries"]}
        srv = {"%s %s" % (x["method"], x["path"]) for x in []}
        check("golden-good: documented routes absent from undocumented (health, items/:id, catch-all vs {p})",
              not (und & {"GET /health", "GET /api/v1/items/{}", "GET /api/v1/catalog/{}", "POST /api/v1/items"}), str(und))
        check("golden-bad: undocumented gin routes detected", {"GET /api/v1/smb/discover", "GET /api/v1/other/rebound"} <= und, str(und))
        check("golden-bad: ANY route without spec detected", "ANY /api/v1/anything" in und, str(und))
        check("golden-bad: wired mux route undocumented detected", "GET /api/v1/wired/x/{}" in und, str(und))
        check("golden-bad: stale spec entry detected", "GET /api/v1/stale/only" in stale and "DELETE /api/v1/items" in stale, str(stale))
        check("unwired mux route reported and stale entry annotated",
              any(u["key"] == "POST /api/v1/music/play" for u in r["unwired_mux_routes"]) and
              any(s["key"] == "POST /api/v1/music/play" and "UNWIRED" in s.get("note", "") for s in r["stale_spec_entries"]), str(r["stale_spec_entries"]))
        check("control: variable re-binding uses latest prefix (smb -> /other)", "GET /api/v1/other/rebound" in und and "GET /api/v1/smb/rebound" not in und)
        check("control: commented-out route ignored", not any("commented" in k for k in und))
        check("control: route inside a string literal ignored", not any("in/string" in k for k in und))
        check("control: *_test.go route ignored", not any("only/in/test" in k for k in und))
        check("control: OPTIONS route ignored", not any("cors" in k for k in und))
        check("control: pprof excluded (N6)", not any("pprof" in k for k in und))
        check("control: unknown receiver goes to unresolved, not silently dropped", any("ghost" in u["what"] for u in r["unresolved"]))
        cw = r["client_calls_without_route"]
        check("api-client: dynamic template + query resolved, matches /items/{id}", "GET /api/v1/items/{}" not in {c["key"] for c in cw["api_client"]}, str(cw["api_client"]))
        check("api-client: unrouted call reported", {c["key"] for c in cw["api_client"]} == {"GET /api/v1/nope/missing"}, str(cw["api_client"]))
        check("control: params.get/map.get noise not captured", r["counts"]["client_calls"]["api_client"] == 3, str(r["counts"]["client_calls"]))
        check("web: catch-all matches multi-segment client path (N3/N8)", not any("catalog" in c["key"] for c in cw["web"]), str(cw["web"]))
        check("web: double prefix flagged and has no route", sum(1 for d in r["double_prefix_calls"] if d["client"] == "web") == 1 and any("favorites" in c["key"] for c in cw["web"]), str(r["double_prefix_calls"]))
        check("web: fetch with method option recognised (POST /items/{} has no route)", any(c["key"] == "POST /api/v1/items/{}" for c in cw["web"]), str(cw["web"]))
        check("android: relative paths prefixed; host-absolute not prefixed", {c["key"] for c in cw["android"]} == {"POST /api/v1/smb/connect", "DELETE /abs/path", "DELETE /api/v1/items/{}", "GET /api/v1/api/v1/items/{}"}, str(cw["android"]))
        check("androidtv: base '/' (paths already carry api/v1) matches; unrouted reported", {c["key"] for c in cw["androidtv"]} == {"GET /api/v1/tv/missing"}, str(cw["androidtv"]))
        check("android: relative api/v1 path flagged as double prefix", any(d["client"] == "android" for d in r["double_prefix_calls"]), str(r["double_prefix_calls"]))
        check("control: dynamic first segment goes to unresolved, not no-route", any("dynamic" in u["what"] for u in r["unresolved"]) and not any(c["key"] == "GET /api/v1/{}/x" for c in cw["web"]))
        check("control: zap.Any(\"event\") is not an unresolved route", not any("zap" in u["what"] for u in r["unresolved"]))
        check("control: commented Kotlin annotation ignored", not any("commented" in c["key"] for c in cw["android"]))
        r2 = analyse(t, ["docs/api/openapi.yaml"], "/api/v1", "/debug/pprof")
        check("deterministic: identical output on re-run", json.dumps(r, sort_keys=True) == json.dumps(r2, sort_keys=True))
    print("SELF-TEST: pass=%d fail=%d" % (ok, bad))
    return bad == 0


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--root", default=".")
    ap.add_argument("--spec", action="append", default=[])
    ap.add_argument("--api-client-prefix", default="/api/v1")
    ap.add_argument("--exclude-prefix", default="/debug/pprof")
    ap.add_argument("--summary", action="store_true", help="print only counts")
    ap.add_argument("--fail-on-drift", action="store_true")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    if a.self_test:
        sys.exit(0 if self_test() else 1)
    specs = a.spec or ["docs/api/openapi.yaml"]
    r = analyse(a.root, specs, a.api_client_prefix, a.exclude_prefix)
    out = {"counts": r["counts"]} if a.summary else r
    json.dump(out, sys.stdout, indent=1)
    sys.stdout.write("\n")
    c = r["counts"]
    if a.fail_on_drift and (c["undocumented_routes"] or c["stale_spec_entries"] or c["double_prefix_calls"] or any(c["client_calls_without_route"].values())):
        sys.exit(1)


if __name__ == "__main__":
    main()
