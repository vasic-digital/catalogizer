#!/usr/bin/env python3
"""check_doc_coverage.py - documentation-vs-reality coverage matrix (docs sync C4).

Checks (each prints GAP lines and a per-check count; exit 1 when any gap remains):
  routes   every HTTP route registered by catalog-api (main.go AND the Register*Routes helpers it calls)
           is an operation in docs/api/openapi.yaml, and every spec operation is a registered route
  env      every environment variable read by non-test catalog-api Go code appears in
           docs/deployment/ENVIRONMENT_VARIABLES.md or docs/ENV_VARIABLES.md
  scripts  every top-level scripts/<file> has docs/scripts/<name>.md or an entry in the exemption file
  readme   README.md links the documentation index docs/README.md
  claims   curated behaviour statements: the doc says it AND the code still does it
Usage: check_doc_coverage.py [--root DIR] [--check NAME]... [--exempt FILE] [--json]
Exit: 0 no gap | 1 gaps | 2 usage / unreadable input | 3 BLIND (a check extracted zero items)
See docs/scripts/check_doc_coverage.md.
"""
import argparse, json, os, re, subprocess, sys

try:
    import yaml
except ImportError:
    print("check_doc_coverage: PyYAML is required", file=sys.stderr)
    sys.exit(2)

METHODS = ("GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS")
ROUTE_EXCLUDE_PREFIX = ("/debug/pprof",)


class Blind(Exception):
    pass


def read(root, rel):
    with open(os.path.join(root, rel), encoding="utf-8", errors="replace") as f:
        return f.read()


def tracked(root, pattern):
    out = subprocess.run(["git", "-C", root, "ls-files", pattern], capture_output=True, text=True)
    if out.returncode != 0:
        raise OSError("git ls-files failed: " + out.stderr.strip())
    return [p for p in out.stdout.split("\n") if p]


def norm(path):
    p = re.sub(r"[:*]\w+", "{}", path)
    p = re.sub(r"\{[^}/]*\}", "{}", p)
    return p.rstrip("/") or "/"


# ---------------------------------------------------------------- routes
GROUP = re.compile(r'(\w+)\s*:?=\s*(\w+)\.Group\(\s*"([^"]*)"')
ROUTE = re.compile(r'\b(\w+)\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\(\s*"([^"]*)"')
HELPER_DEF = re.compile(r'func\s+(?:\(\s*\w+\s+\*?\w+\s*\)\s*)?(Register\w+)\(\s*(\w+)\s+\*gin\.RouterGroup')
HELPER_CALL = re.compile(r'\b(Register\w+)\(\s*(\w+)')


def scan_lines(lines, env, helpers, out, base_var=None):
    """env: var -> prefix. Collect (METHOD, full path) into out; expand helper calls."""
    for line in lines:
        m = GROUP.search(line)
        if m and m.group(2) in env:
            env[m.group(1)] = env[m.group(2)] + m.group(3)
        for r in ROUTE.finditer(line):
            if r.group(1) in env:
                out.add((r.group(2), env[r.group(1)] + r.group(3)))
        for c in HELPER_CALL.finditer(line):
            name, arg = c.group(1), c.group(2)
            if name in helpers and arg in env:
                pvar, body = helpers[name]
                scan_lines(body, {pvar: env[arg]}, helpers, out)


def code_routes(root):
    main = read(root, "catalog-api/main.go").split("\n")
    helpers = {}
    for f in tracked(root, "catalog-api/handlers/*.go"):
        if f.endswith("_test.go"):
            continue
        lines = read(root, f).split("\n")
        for i, l in enumerate(lines):
            d = HELPER_DEF.search(l)
            if d:
                j = i + 1
                while j < len(lines) and not lines[j].startswith("}"):
                    j += 1
                helpers[d.group(1)] = (d.group(2), lines[i + 1:j])
    out = set()
    scan_lines(main, {"router": ""}, helpers, out)
    # helper call receivers are not tracked by name (e.g. `x.RegisterAliasRoutes(api)`): HELPER_CALL covers it
    res = {(m, norm(p)) for m, p in out if not any(p.startswith(x) for x in ROUTE_EXCLUDE_PREFIX)}
    if not res:
        raise Blind("routes: zero routes extracted from catalog-api/main.go")
    return res


def spec_routes(root):
    doc = yaml.safe_load(read(root, "docs/api/openapi.yaml"))
    res = set()
    for path, item in (doc.get("paths") or {}).items():
        for k in item:
            if k.upper() in METHODS:
                res.add((k.upper(), norm(path)))
    if not res:
        raise Blind("routes: zero operations in docs/api/openapi.yaml")
    return res


def check_routes(root, _ex):
    code, spec = code_routes(root), spec_routes(root)
    gaps = ["UNDOCUMENTED\t%s %s" % x for x in sorted(code - spec)]
    gaps += ["PHANTOM\t%s %s" % x for x in sorted(spec - code)]
    return gaps, len(code), len(spec)


# ---------------------------------------------------------------- env
ENV_CALL = re.compile(r'\b(?:\w+\.)*(\w*[Ee]nv\w*)\(([^()]*)\)')


def env_reads(root):
    names = {}
    for f in tracked(root, "catalog-api/*.go"):
        if f.endswith("_test.go"):
            continue
        for i, l in enumerate(read(root, f).split("\n"), 1):
            for m in ENV_CALL.finditer(l):
                if re.search(r"Setenv|Unsetenv|Expand", m.group(1)):
                    continue
                for n in re.findall(r'"([A-Z][A-Z0-9_]{2,})"', m.group(2)):
                    names.setdefault(n, "%s:%d" % (f, i))
    if not names:
        raise Blind("env: zero environment reads extracted")
    return names


def check_env(root, ex):
    names = env_reads(root)
    docs = read(root, "docs/deployment/ENVIRONMENT_VARIABLES.md") + "\n" + read(root, "docs/ENV_VARIABLES.md")
    gaps = []
    for n, src in sorted(names.items()):
        if ("env", n) in ex:
            continue
        if not re.search(r"(?<![A-Z0-9_])%s(?![A-Z0-9_])" % re.escape(n), docs):
            gaps.append("UNDOCUMENTED_ENV\t%s\t%s" % (n, src))
    return gaps, len(names), None


# ---------------------------------------------------------------- scripts
def check_scripts(root, ex):
    files = [f for f in tracked(root, "scripts/*") if re.match(r"^scripts/[^/]+$", f) and not f.endswith(".md")]
    if not files:
        raise Blind("scripts: zero top-level scripts")
    guides = {g.lower() for g in os.listdir(os.path.join(root, "docs/scripts"))}
    gaps = []
    for f in files:
        b = os.path.basename(f)
        if ("script", b) in ex:
            continue
        stem = b.rsplit(".", 1)[0] if "." in b else b
        cands = {stem + ".md", stem.replace("-", "_") + ".md", stem.replace("_", "-") + ".md", b + ".md"}
        if not any(c.lower() in guides for c in cands):
            gaps.append("NO_SCRIPT_GUIDE\t%s" % f)
    return gaps, len(files), None


# ---------------------------------------------------------------- readme
def check_readme(root, _ex):
    ok = re.search(r"\]\(\.?/?docs/README\.md\)", read(root, "README.md"))
    return ([] if ok else ["README_NO_DOCS_INDEX_LINK\tREADME.md"]), 1, None


# ---------------------------------------------------------------- claims
# (id, doc file, doc regex, code file, code regex)
CLAIMS = [
    ("registration-approval-default", "docs/UPGRADE_NOTES.md", r"REGISTRATION_MODE.{0,40}approval",
     "catalog-api/handlers/auth_handler.go", r"RegistrationModeApproval"),
    ("registration-mode-in-env-reference", "docs/deployment/ENVIRONMENT_VARIABLES.md", r"REGISTRATION_MODE",
     "catalog-api/handlers/auth_handler.go", r'Getenv\("REGISTRATION_MODE"\)'),
    ("smart-playlist-501", "docs/api/API_DOCUMENTATION.md", r"is_smart.{0,80}\*\*501\*\*",
     "catalog-api/handlers/playlist_handler.go", r"StatusNotImplemented.{0,200}smart playlists are not supported|smart playlists are not supported"),
    ("share-controls-disabled", "docs/UPGRADE_NOTES.md", r"(?i)collection.{0,40}shar\w+.{0,200}disabled",
     "catalog-web/src/components/collections/CollectionSharing.tsx", r"SHARE_API_AVAILABLE"),
    ("converters-in-image", "docs/deployment/DOCKER_SETUP.md", r"ffmpeg.{0,200}imagemagick|imagemagick.{0,200}ffmpeg",
     "catalog-api/Dockerfile.runtime", r"ffmpeg\s+imagemagick"),
    ("converters-probe-in-health", "docs/api/openapi.yaml", r"converters:",
     "catalog-api/internal/metrics/health.go", r'json:"converters"'),
    ("go-fitz-removed-poppler-renders", "docs/UPGRADE_NOTES.md", r"go-fitz.{0,300}(?i:AGPL).{0,300}(?i:removed)|go-fitz.{0,300}(?i:removed)",
     "catalog-api/services/pdf_page_render.go", r'"pdftoppm"'),
]


def check_claims(root, ex):
    gaps = []
    for cid, df, dre, cf, cre in CLAIMS:
        if ("claim", cid) in ex:
            continue
        try:
            d = read(root, df)
        except OSError:
            gaps.append("CLAIM_DOC_MISSING\t%s\t%s" % (cid, df))
            continue
        if not re.search(dre, d, re.S):
            gaps.append("CLAIM_NOT_IN_DOC\t%s\t%s" % (cid, df))
        try:
            c = read(root, cf)
        except OSError:
            gaps.append("CLAIM_CODE_MISSING\t%s\t%s" % (cid, cf))
            continue
        if not re.search(cre, c, re.S):
            gaps.append("CLAIM_NOT_IN_CODE\t%s\t%s" % (cid, cf))
    return gaps, len(CLAIMS), None


CHECKS = {"routes": check_routes, "env": check_env, "scripts": check_scripts, "readme": check_readme, "claims": check_claims}


def load_exempt(path):
    ex = set()
    if path and os.path.exists(path):
        for ln in open(path, encoding="utf-8"):
            ln = ln.rstrip("\n")
            if not ln or ln.startswith("#"):
                continue
            parts = ln.split("\t")
            if len(parts) < 3 or not parts[2].strip():
                raise ValueError("exemption line needs kind<TAB>name<TAB>reason: %r" % ln)
            ex.add((parts[0], parts[1]))
    return ex


def main(argv):
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=os.getcwd())
    ap.add_argument("--check", action="append", choices=sorted(CHECKS))
    ap.add_argument("--exempt", default=None)
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args(argv)
    exempt = a.exempt or os.path.join(a.root, "scripts/docs/doc_coverage_exempt.tsv")
    try:
        ex = load_exempt(exempt)
    except (OSError, ValueError) as e:
        print("check_doc_coverage: " + str(e), file=sys.stderr)
        return 2
    total, report, blind = 0, {}, False
    for name in a.check or sorted(CHECKS):
        try:
            gaps, n, m = CHECKS[name](a.root, ex)
        except Blind as e:
            print("BLIND\t" + str(e))
            blind = True
            continue
        except OSError as e:
            print("check_doc_coverage: %s: %s" % (name, e), file=sys.stderr)
            return 2
        total += len(gaps)
        report[name] = {"items": n, "gaps": len(gaps)}
        for g in gaps:
            print("GAP\t%s\t%s" % (name, g))
        print("check %s: %d items, %d gaps" % (name, n, len(gaps)))
    if a.json:
        print(json.dumps(report, sort_keys=True))
    print("total gaps: %d" % total)
    return 3 if blind else (1 if total else 0)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
