#!/usr/bin/env python3
"""check_api_route_vs_spec.py - local gate: the Gin router in catalog-api/main.go and docs/api/openapi.yaml must describe the same operations.

Usage: scripts/check_api_route_vs_spec.py [--main FILE] [--spec FILE] [--exclude-prefix P]... [--list]

Needs the Go toolchain (stdlib go/ast extractor in scripts/lib/gin_routes_ast.go).
Exit: 0 in sync | 1 drift (undocumented route, phantom spec operation, equivalent templated paths) | 2 usage / unreadable / invalid YAML (duplicate keys)
      3 blind instrument (zero routes/operations extracted, or a route registration with a non-literal path/unknown receiver)
See docs/scripts/check_api_route_vs_spec.md.
"""
import os
import re
import shutil
import subprocess
import sys
import tempfile

try:
    import yaml
except ImportError:  # pragma: no cover
    print("check_api_route_vs_spec: PyYAML is required", file=sys.stderr)
    sys.exit(2)

METHODS = ("GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS")
DEFAULT_EXCLUDES = ("/debug/pprof",)
AST_SRC = os.path.join(os.path.dirname(os.path.abspath(__file__)), "lib", "gin_routes_ast.go")


def norm(path):
    p = re.sub(r"[:*]\w+", "{}", path)
    p = re.sub(r"\{[^}/]*\}", "{}", p)
    return p.rstrip("/") or "/"


class Unresolved(Exception):
    pass


def code_routes(path):
    """Extract (METHOD, normalised path) from main() of a Go file with go/ast (scripts/lib/gin_routes_ast.go)."""
    go = shutil.which("go")
    if not go:
        raise OSError("the Go toolchain (go) is required for go/ast route extraction")
    env = dict(os.environ, GOTOOLCHAIN="local", GOPROXY="off", GOFLAGS="", CGO_ENABLED="0")
    tmp = tempfile.mkdtemp(prefix="carvs_ast.")
    try:
        exe = os.path.join(tmp, "ast")
        b = subprocess.run([go, "build", "-o", exe, AST_SRC], env=env, capture_output=True, text=True, cwd=tmp)
        if b.returncode != 0:
            raise OSError("cannot build the AST extractor: %s" % b.stderr.strip())
        r = subprocess.run([exe, path], env=env, capture_output=True, text=True)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    if r.returncode != 0:
        raise OSError("AST extraction failed: %s" % r.stderr.strip())
    routes, bad = set(), []
    for line in r.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        if line.startswith("UNRESOLVED"):
            bad.append(line)
            continue
        m, p = line.split(" ", 1)
        routes.add((m, norm(p)))
    if bad:
        raise Unresolved("\n".join(bad))
    return routes


class DupKeyLoader(yaml.SafeLoader):
    pass


def _construct(loader, node, deep=False):
    seen = set()
    for k, _ in node.value:
        key = loader.construct_object(k, deep=deep)
        if key in seen:
            raise yaml.constructor.ConstructorError(None, None, "duplicate mapping key %r" % (key,), k.start_mark)
        seen.add(key)
    return yaml.SafeLoader.construct_mapping(loader, node, deep=deep)


DupKeyLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _construct)


def spec_ops(path):
    doc = yaml.load(open(path, encoding="utf-8"), Loader=DupKeyLoader)
    ops, templated = set(), {}
    for p, item in ((doc or {}).get("paths") or {}).items():
        n = norm(p)
        templated.setdefault(n, []).append(p)
        for k in (item or {}):
            if k.upper() in METHODS:
                ops.add((k.upper(), n))
    clashes = {n: ps for n, ps in templated.items() if len(ps) > 1}
    return ops, clashes


def main(argv):
    root = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    main_go = os.path.join(root, "catalog-api", "main.go")
    spec = os.path.join(root, "docs", "api", "openapi.yaml")
    excludes, listing = list(DEFAULT_EXCLUDES), False
    custom_ex = False
    i = 0
    while i < len(argv):
        a = argv[i]
        if a == "--main" and i + 1 < len(argv): main_go = argv[i + 1]; i += 1
        elif a == "--spec" and i + 1 < len(argv): spec = argv[i + 1]; i += 1
        elif a == "--exclude-prefix" and i + 1 < len(argv):
            if not custom_ex: excludes, custom_ex = [], True
            excludes.append(argv[i + 1]); i += 1
        elif a == "--list": listing = True
        else:
            print(__doc__, file=sys.stderr); return 2
        i += 1
    try:
        code = code_routes(main_go)
        ops, clashes = spec_ops(spec)
    except Unresolved as e:
        print("BLIND: route registrations that cannot be resolved statically (refusing to report in-sync):\n%s" % e); return 3
    except (OSError, yaml.YAMLError) as e:
        print("check_api_route_vs_spec: cannot read input: %s" % e, file=sys.stderr); return 2
    keep = lambda r: not any(r[1] == x or r[1].startswith(x + "/") for x in excludes)
    code, ops = {r for r in code if keep(r)}, {r for r in ops if keep(r)}
    if listing:
        print("\n".join("%s %s" % r for r in sorted(code, key=lambda r: (r[1], r[0]))))
        return 0
    if not code or not ops:
        print("BLIND: extracted %d router routes from %s and %d spec operations from %s; refusing to report a zero as in-sync" % (len(code), main_go, len(ops), spec))
        return 3
    undoc, phantom = sorted(code - ops, key=lambda r: (r[1], r[0])), sorted(ops - code, key=lambda r: (r[1], r[0]))
    for n, ps in sorted(clashes.items()):
        print("EQUIVALENT TEMPLATED PATHS in spec (invalid per OpenAPI 3.0.3): %s" % ", ".join(ps))
    if undoc:
        print("IN CODE, NOT IN SPEC (%d):" % len(undoc)); [print("  %s %s" % r) for r in undoc]
    if phantom:
        print("IN SPEC, NOT IN CODE (%d):" % len(phantom)); [print("  %s %s" % r) for r in phantom]
    if undoc or phantom or clashes:
        return 1
    print("OK: %d router operations == %d spec operations (excluded prefixes: %s)" % (len(code), len(ops), ", ".join(excludes) or "none"))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
