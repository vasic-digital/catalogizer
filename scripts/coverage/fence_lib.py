"""fence_lib.py - shared helpers of the coverage exclusion fence (T200; WP-23 review rounds 1 and 2: WF11 I5, I6, round-2 K1..K5). Imported by check_exclusions.py,
bashcov.py and gocov_merge.py.

  FenceError                       a pattern, a config or an input the fence cannot evaluate: the caller REFUSES (never `stale` + PASS)
  glob_re(pattern, mode)           ONE glob dialect, the picomatch subset the measuring tools use: `**` crosses `/`, `*` and `?` do not, `[abc]` classes, `{a,b}` braces, a
                                   trailing `/` or `/**` names a subtree; extglob (`?(`, `*(`, `+(`, `@(`, `!(`), a leading `!`, regex syntax and brace ranges raise FenceError.
                                   mode `fence`: a bare directory name also names its subtree (the checked-in list reads like a .gitignore); mode `tool`: strict, as the
                                   tools' own matcher (`src` names only `src`)
  matches(pattern, rel, mode)      True when the relative path is named by the pattern
  list_files(root)                 (files, info): the TRACKED files of an application root, from `git ls-files --cached` with a clean GIT_* environment when the root is inside
                                   a work tree; a root that is not inside any work tree is walked (info.enumeration says which). A git failure INSIDE a work tree, or a dangling
                                   gitfile, raises FenceError: it never falls back to a walk (an untracked scratch file or an ignored build tree would be counted)
  load_yaml_strict(text)           PyYAML safe_load that REFUSES duplicate keys and non-string keys (a duplicate would silently drop an entry, `no` would become False)
  extract_applied(kind, cfg, ...)  what a measuring tool's OWN config applies, parsed as a JavaScript/Kotlin literal structure and FAIL-CLOSED: anything that is not a literal
                                   (a variable, a spread, a call, a template with substitutions, a function-style config) is `unverifiable`, never an empty list
  class_evidence(cls, rel, ctx)    does THIS file look like the class its entry claims? generated-code: an ANCHORED header marker, or a tracked file under a directory that
                                   a tracked project config declares as generator output; vendored-third-party: a provenance record, not the directory name; fixtures: a test
                                   SOURCE SET location, and for non-test source in a fixture directory an import search that finds no importer
"""
import fnmatch, hashlib, json, os, re, subprocess

MEASURABLE = {"vitest": (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".vue"), "jest": (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"),
              "jacoco": (".kt", ".java")}


class FenceError(Exception):
    pass


# --------------------------------------------------------------------------- glob dialect
_EXTGLOB = re.compile(r"[?*+@!]\(")


def expand_braces(p):
    """`a{b,c}d` -> [abd, acd]; nested braces; a numeric/alpha range `{1..3}` or an unbalanced brace raises FenceError."""
    out = [p]
    while True:
        nxt = []; changed = False
        for s in out:
            depth = 0; start = None
            for i, c in enumerate(s):
                if c == "\\":
                    continue
                if c == "{":
                    if depth == 0:
                        start = i
                    depth += 1
                elif c == "}":
                    depth -= 1
                    if depth < 0:
                        raise FenceError("unbalanced } in %r" % p)
                    if depth == 0:
                        body = s[start + 1:i]
                        parts = []; d = 0; cur = ""
                        for ch in body:
                            if ch == "{": d += 1
                            if ch == "}": d -= 1
                            if ch == "," and d == 0:
                                parts.append(cur); cur = ""
                            else:
                                cur += ch
                        parts.append(cur)
                        if ".." in body and len(parts) == 1:
                            raise FenceError("brace range in %r is not supported" % p)
                        if len(parts) == 1:
                            nxt.append(s[:start] + "\0" + body + "\1" + s[i + 1:])   # a one-element brace is literal text
                        else:
                            for part in parts:
                                nxt.append(s[:start] + part + s[i + 1:])
                        changed = True
                        break
            else:
                if depth != 0:
                    raise FenceError("unbalanced { in %r" % p)
                nxt.append(s)
        out = nxt
        if not changed:
            break
    return [s.replace("\0", "{").replace("\1", "}") for s in out]


def _one_re(p, mode):
    if p.startswith("./"):
        p = p[2:]
    dir_pat = p.endswith("/") or p.endswith("/**")
    if p.endswith("/**"):
        p = p[:-3]
    p = p.rstrip("/")
    out = []; i = 0; segs = 0
    while i < len(p):
        c = p[i]
        if c == "*":
            if p[i:i + 3] == "**/" and (i == 0 or p[i - 1] == "/"):
                out.append("(?:.*/)?"); i += 3; continue
            if p[i:i + 2] == "**":
                whole = (i == 0 or p[i - 1] == "/") and (i + 2 >= len(p) or p[i + 2] == "/")
                out.append(".*" if whole else "[^/]*")
                while i < len(p) and p[i] == "*":
                    i += 1
                continue
            out.append("[^/]*")
        elif c == "?":
            out.append("[^/]")
        elif c == "[":
            j = i + 1
            if j < len(p) and p[j] in "!^":
                j += 1
            if j < len(p) and p[j] == "]":
                j += 1
            while j < len(p) and p[j] != "]":
                j += 1
            if j >= len(p):
                raise FenceError("unterminated [ in %r" % p)
            body = p[i + 1:j]
            neg = body[:1] in ("!", "^")
            if neg:
                body = body[1:]
            out.append("[" + ("^/" if neg else "") + body.replace("\\", "\\\\") + "]")
            i = j
        elif c == "\\" and i + 1 < len(p):
            out.append(re.escape(p[i + 1])); i += 1
        else:
            out.append(re.escape(c))
        i += 1
    body = "".join(out)
    tail = "(?:/.*)?" if (dir_pat or mode == "fence") else ""
    return "^" + body + tail + "$"


def glob_re(pattern, mode="fence"):
    p = pattern.strip()
    if not p:
        raise FenceError("empty pattern")
    if p.startswith("!"):
        raise FenceError("negated pattern %r is not supported" % pattern)
    if _EXTGLOB.search(p):
        raise FenceError("extglob syntax in %r is not supported" % pattern)
    alts = [_one_re(x, mode) for x in expand_braces(p)]
    return re.compile("|".join("(?:%s)" % a for a in alts))


_RE_CACHE = {}


def matches(pattern, rel, mode="fence"):
    key = (pattern, mode)
    r = _RE_CACHE.get(key)
    if r is None:
        r = _RE_CACHE[key] = glob_re(pattern, mode)
    return bool(r.match(rel))


# --------------------------------------------------------------------------- enumeration
def clean_git_env():
    return {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}


def _git(root, args):
    return subprocess.run(["git", "-C", root] + args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=clean_git_env())


def list_files(root):
    """Returns (sorted relative POSIX paths, info). See the module docstring: tracked only inside a work tree, a walk only outside every work tree."""
    root = os.path.abspath(root)
    if not os.path.isdir(root):
        raise FenceError("root_missing: %s is not a directory" % root)
    inside = _git(root, ["rev-parse", "--is-inside-work-tree"])
    has_dotgit = os.path.exists(os.path.join(root, ".git"))
    if inside.returncode == 0 and inside.stdout.strip() == b"true":
        p = _git(root, ["ls-files", "-z", "--cached", "--", "."])
        if p.returncode != 0:
            raise FenceError("enumeration_failed: git ls-files failed in %s: %s" % (root, p.stderr.decode("utf-8", "replace").strip()[:200]))
        names = sorted(n for n in p.stdout.decode("utf-8", "replace").split("\0") if n)
        return names, {"enumeration": "git-tracked", "count": len(names), "fingerprint": hashlib.sha256("\n".join(names).encode()).hexdigest()}
    if has_dotgit:
        raise FenceError("enumeration_failed: %s has a .git entry but git cannot read it (a dangling gitfile or a broken repository): refusing to walk" % root)
    out = []
    for dp, dns, fns in os.walk(root):
        dns[:] = [d for d in dns if d != ".git"]
        for f in fns:
            out.append(os.path.relpath(os.path.join(dp, f), root).replace(os.sep, "/"))
    out.sort()
    return out, {"enumeration": "walk", "count": len(out), "fingerprint": hashlib.sha256("\n".join(out).encode()).hexdigest()}


# --------------------------------------------------------------------------- strict YAML
def load_yaml_strict(text):
    import yaml

    class Strict(yaml.SafeLoader):
        pass

    def construct_mapping(loader, node, deep=False):
        seen = set()
        for k_node, _v in node.value:
            k = loader.construct_object(k_node, deep=True)
            if not isinstance(k, str):
                raise yaml.YAMLError("non-string key %r (a bare yes/no/on/off/null key is a boolean in YAML 1.1: quote it)" % (k,))
            if k in seen:
                raise yaml.YAMLError("duplicate key %r" % k)
            seen.add(k)
        return yaml.SafeLoader.construct_mapping(loader, node, deep)
    Strict.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, construct_mapping)
    return yaml.load(text, Loader=Strict)


# --------------------------------------------------------------------------- JavaScript / Kotlin literal parsing (fail closed)
class Expr:
    def __init__(self, raw):
        self.raw = raw.strip()

    def __repr__(self):
        return "Expr(%r)" % self.raw


class Spread(Expr):
    pass


def js_strip_comments(src):
    out = []; i = 0; n = len(src); q = None
    while i < n:
        c = src[i]
        if q:
            out.append(c)
            if c == "\\" and i + 1 < n:
                out.append(src[i + 1]); i += 2; continue
            if c == q:
                q = None
            i += 1; continue
        if c in "'\"`":
            q = c; out.append(c); i += 1; continue
        if c == "/" and src[i + 1:i + 2] == "/":
            while i < n and src[i] != "\n":
                i += 1
            continue
        if c == "/" and src[i + 1:i + 2] == "*":
            j = src.find("*/", i + 2)
            i = n if j < 0 else j + 2
            out.append(" "); continue
        out.append(c); i += 1
    return "".join(out)


class JsParse:
    def __init__(self, text):
        self.t = js_strip_comments(text); self.i = 0

    def ws(self):
        while self.i < len(self.t) and self.t[self.i].isspace():
            self.i += 1

    def peek(self):
        self.ws()
        return self.t[self.i] if self.i < len(self.t) else ""

    def skip_expr(self):
        """Consumes a balanced expression up to a top-level `,` or closing bracket and returns its raw text."""
        start = self.i; depth = 0
        while self.i < len(self.t):
            c = self.t[self.i]
            if c in "'\"`":
                q = c; self.i += 1
                while self.i < len(self.t) and self.t[self.i] != q:
                    if self.t[self.i] == "\\":
                        self.i += 1
                    self.i += 1
                self.i += 1; continue
            if c in "([{":
                depth += 1
            elif c in ")]}":
                if depth == 0:
                    break
                depth -= 1
            elif c == "," and depth == 0:
                break
            self.i += 1
        return self.t[start:self.i]

    def string(self):
        q = self.t[self.i]; self.i += 1; buf = []
        while self.i < len(self.t) and self.t[self.i] != q:
            c = self.t[self.i]
            if c == "\\" and self.i + 1 < len(self.t):
                n = self.t[self.i + 1]
                buf.append({"n": "\n", "t": "\t", "r": "\r"}.get(n, n)); self.i += 2; continue
            buf.append(c); self.i += 1
        self.i += 1
        return "".join(buf)

    def value(self):
        self.ws()
        start = self.i
        c = self.peek()
        try:
            if c == "{":
                v = self.obj()
            elif c == "[":
                v = self.arr()
            elif c in ("'", '"'):
                v = self.string()
            elif c == "`":
                s = self.i; v = self.string()
                if "${" in self.t[s:self.i]:
                    raise ValueError("template with substitution")
            else:
                m = re.compile(r"-?\d+(?:\.\d+)?(?![\w.])").match(self.t, self.i)
                if m:
                    self.i = m.end(); v = float(m.group(0)) if "." in m.group(0) else int(m.group(0))
                else:
                    m = re.compile(r"(true|false|null)(?![\w$])").match(self.t, self.i)
                    if m:
                        self.i = m.end(); v = {"true": True, "false": False, "null": None}[m.group(1)]
                    else:
                        raise ValueError("expression")
            n = self.peek()
            if n not in (",", "]", "}", ")", ""):
                raise ValueError("continues as an expression")
            return v
        except ValueError:
            self.i = start
            return Expr(self.skip_expr())

    def key(self):
        c = self.peek()
        if c in ("'", '"'):
            return self.string()
        m = re.compile(r"[A-Za-z_$][\w$]*|\d+").match(self.t, self.i)
        if not m:
            return None
        self.i = m.end()
        return m.group(0)

    def obj(self):
        self.i += 1; d = {}; spreads = []
        while True:
            c = self.peek()
            if c == "}":
                self.i += 1; break
            if c == "":
                raise ValueError("unterminated object")
            if self.t.startswith("...", self.i):
                self.i += 3; spreads.append(Spread(self.skip_expr()))
            elif c == "[":
                self.i += 1
                d[("computed", len(d))] = Expr(self.skip_expr())
                self.i += 1
                if self.peek() == ":":
                    self.i += 1; self.value()
            else:
                k = self.key()
                if k is None:
                    d[("odd", len(d))] = Expr(self.skip_expr())
                elif self.peek() == ":":
                    self.i += 1; d[k] = self.value()
                elif self.peek() in (",", "}"):
                    d[k] = Expr(k)
                else:   # a method: name(...) { ... }
                    self.skip_expr(); d[k] = Expr("method")
            if self.peek() == ",":
                self.i += 1
        if spreads:
            d["__spreads__"] = spreads
        return d

    def arr(self):
        self.i += 1; a = []
        while True:
            c = self.peek()
            if c == "]":
                self.i += 1; break
            if c == "":
                raise ValueError("unterminated array")
            if self.t.startswith("...", self.i):
                self.i += 3; a.append(Spread(self.skip_expr()))
            else:
                a.append(self.value())
            if self.peek() == ",":
                self.i += 1
        return a


def js_config_object(text):
    """The literal config object of a vite/vitest/jest config: `defineConfig({`, `export default {` or `module.exports = {`. None when the config is not literal
    (a function style `defineConfig(({ mode }) => ...)`, a variable): the caller treats that as unverifiable."""
    clean = js_strip_comments(text)
    best = None
    for rx in (r"defineConfig\s*\(\s*\{", r"export\s+default\s*\{", r"module\.exports\s*=\s*\{"):
        m = re.search(rx, clean)
        if m and (best is None or m.start() < best.start()):
            best = m
    if not best:
        return None
    p = JsParse(clean); p.i = best.end() - 1
    try:
        return p.obj()
    except ValueError:
        return None


def _lit_str_list(v, what):
    """A literal array of literal strings, or raises FenceError."""
    if not isinstance(v, list):
        raise FenceError("applied_exclusions_unverifiable: %s is not a literal array (%r)" % (what, v))
    out = []
    for e in v:
        if not isinstance(e, str):
            raise FenceError("applied_exclusions_unverifiable: %s holds a non-literal element %r" % (what, e))
        out.append(e)
    return out


def _version_major(root, pkg):
    try:
        d = json.load(open(os.path.join(root, "package.json"), encoding="utf-8"))
    except (OSError, ValueError):
        return None, None
    for sec in ("devDependencies", "dependencies"):
        v = (d.get(sec) or {}).get(pkg)
        if v:
            m = re.search(r"(\d+)\.(\d+)", v)
            return (int(m.group(1)) if m else None), v
    return None, None


def package_scripts(root):
    try:
        return json.load(open(os.path.join(root, "package.json"), encoding="utf-8")).get("scripts") or {}
    except (OSError, ValueError):
        return {}


def tool_in_script(script):
    """Which of jest / vitest a package.json script runs: 'vitest', 'jest', 'both' or None."""
    s = script or ""
    v = bool(re.search(r"(?<![\w-])vitest(?![\w-])", s)); j = bool(re.search(r"(?<![\w-])jest(?![\w-])", s))
    return "both" if v and j else "vitest" if v else "jest" if j else None


def extract_applied(kind, cfg_path, root=None):
    """Returns a dict: patterns (list of strings, tool dialect) or None, regexes (jest coveragePathIgnorePatterns), include (list or None), implicit_scope (None or a
    description of a scope the fence cannot enumerate), tool_defaults (list), note, unverifiable (None or the reason)."""
    res = {"patterns": [], "regexes": [], "include": None, "implicit_scope": None, "tool_defaults": [], "note": "", "unverifiable": None, "class_patterns": False}
    if kind == "none":
        res["note"] = "the tool applies no exclusion"; return res
    try:
        text = open(cfg_path, encoding="utf-8").read()
    except OSError as e:
        res["patterns"] = None; res["unverifiable"] = "cannot read %s: %s" % (cfg_path, e); res["note"] = res["unverifiable"]; return res
    base = os.path.basename(cfg_path)
    try:
        if kind == "vitest":
            major, ver = _version_major(root or os.path.dirname(cfg_path), "vitest")
            cfg = js_config_object(text)
            if cfg is None:
                raise FenceError("applied_exclusions_unverifiable: %s has no literal config object (defineConfig({...}), export default {...}); a function-style config needs the tool's resolved config" % base)
            if "__spreads__" in cfg:
                raise FenceError("applied_exclusions_unverifiable: a spread in the top-level config of %s" % base)
            test = cfg.get("test")
            cov = None
            if test is None:
                pass
            elif isinstance(test, dict):
                cov = test.get("coverage")
            else:
                raise FenceError("applied_exclusions_unverifiable: `test` in %s is not a literal object (%r)" % (base, test))
            cov_key = [k for k in (test or {}) if isinstance(k, str) and k.strip("'\"") == "coverage"] if isinstance(test, dict) else []
            if isinstance(test, dict) and "coverage" in test and not isinstance(cov, dict):
                raise FenceError("applied_exclusions_unverifiable: `coverage` in %s is not a literal object (%r)" % (base, cov))
            if cov is None:
                res["tool_defaults"].append("vitest default coverage.exclude applies (no coverage block): its list is version-specific and is NOT enumerated here")
                res["implicit_scope"] = "no coverage block: vitest %s measures its default include scope" % (ver or "(unknown version)")
                res["unverifiable"] = "applied_exclusions_unverifiable: %s has no coverage block, so vitest applies its DEFAULT coverage.exclude and its default include scope; neither list can be read from the config. Run the tool's resolved config (`vitest --coverage` with a config printing coverage.exclude) in the pinned image to enumerate them" % base
                res["patterns"] = None; res["note"] = res["unverifiable"]; return res
            if "__spreads__" in cov:
                raise FenceError("applied_exclusions_unverifiable: a spread in the coverage block of %s" % base)
            if "exclude" in cov:
                res["patterns"] = _lit_str_list(cov["exclude"], "coverage.exclude of %s" % base)
            else:
                res["patterns"] = None
                res["tool_defaults"].append("vitest default coverage.exclude applies (the coverage block sets no exclude)")
                res["unverifiable"] = "applied_exclusions_unverifiable: the coverage block of %s sets no `exclude`, so the vitest DEFAULT list applies and is not readable from the config" % base
                res["note"] = res["unverifiable"]; return res
            if "include" in cov:
                res["include"] = _lit_str_list(cov["include"], "coverage.include of %s" % base)
            allv = cov.get("all")
            if allv is not None and not isinstance(allv, bool):
                raise FenceError("applied_exclusions_unverifiable: coverage.all of %s is not a literal boolean" % base)
            if res["include"] is None:
                if major is None:
                    res["implicit_scope"] = "vitest version unknown: the default coverage scope cannot be derived"
                elif major >= 4:
                    res["implicit_scope"] = "vitest %s measures ONLY the files its tests load unless coverage.include is set (vitest 4 removed coverage.all); %s sets no coverage.include" % (ver, base)
                elif allv is True or (allv is None and major >= 1):
                    res["implicit_scope"] = None
                else:
                    res["implicit_scope"] = "vitest %s: coverage.all is %s, so only the files its tests load are measured (UNCONFIRMED: taken from the vitest release notes, the tool was not run here)" % (ver, "false/unset (default false before 1.0)")
            res["note"] = "coverage.exclude of %s (vitest %s)" % (base, ver or "?")
            return res
        if kind == "jest":
            cfg = js_config_object(text)
            if cfg is None:
                raise FenceError("applied_exclusions_unverifiable: %s has no literal config object (module.exports = {...})" % base)
            if "__spreads__" in cfg or "projects" in cfg:
                raise FenceError("applied_exclusions_unverifiable: a spread or `projects` in %s" % base)
            if "collectCoverageFrom" in cfg:
                arr = _lit_str_list(cfg["collectCoverageFrom"], "collectCoverageFrom of %s" % base)
                res["patterns"] = [a[1:] for a in arr if a.startswith("!")]
                res["include"] = [a for a in arr if not a.startswith("!")]
            else:
                res["patterns"] = []
                res["implicit_scope"] = "jest measures only the files its tests load unless collectCoverageFrom is set; %s sets none" % base
            if "coveragePathIgnorePatterns" in cfg:
                res["regexes"] = _lit_str_list(cfg["coveragePathIgnorePatterns"], "coveragePathIgnorePatterns of %s" % base)
            else:
                res["regexes"] = ["/node_modules/"]
                res["tool_defaults"].append("jest default coveragePathIgnorePatterns ['/node_modules/']")
            res["note"] = "collectCoverageFrom negations and coveragePathIgnorePatterns of %s" % base
            return res
        if kind == "jacoco":
            clean = js_strip_comments(text)
            if not re.search(r"\bjacoco", clean, re.I):
                raise FenceError("applied_exclusions_unverifiable: no jacoco configuration in %s" % base)
            lists = {}
            for m in re.finditer(r"\bval\s+([A-Za-z_]\w*)\s*=\s*listOf\s*\(", clean):
                j = m.end(); depth = 1
                while j < len(clean) and depth:
                    depth += (clean[j] == "(") - (clean[j] == ")"); j += 1
                inner = clean[m.end():j - 1]
                strs = re.findall(r'"((?:[^"\\]|\\.)*)"', inner)
                rest = re.sub(r'"(?:[^"\\]|\\.)*"', "", inner)
                if re.sub(r"[\s,]", "", rest):
                    lists[m.group(1)] = None   # not a literal list of strings
                else:
                    lists[m.group(1)] = [s.replace("\\$", "$") for s in strs]
            pats = []
            for m in re.finditer(r"\b(?:exclude|excludes|setExcludes)\s*\(", clean):
                j = m.end(); depth = 1
                while j < len(clean) and depth:
                    depth += (clean[j] == "(") - (clean[j] == ")"); j += 1
                args = clean[m.end():j - 1]
                for a in [x.strip() for x in re.split(r",(?![^(]*\))", args) if x.strip()]:
                    sm = re.fullmatch(r'"((?:[^"\\]|\\.)*)"', a)
                    if sm:
                        pats.append(sm.group(1).replace("\\$", "$"))
                    elif re.fullmatch(r"[A-Za-z_]\w*", a) and a in lists:
                        if lists[a] is None:
                            raise FenceError("applied_exclusions_unverifiable: %s in %s is not a literal list of strings" % (a, base))
                        pats.extend(lists[a])
                    else:
                        raise FenceError("applied_exclusions_unverifiable: exclude(%s) in %s is not a literal string or a literal list" % (a, base))
            res["patterns"] = pats; res["class_patterns"] = True
            res["tool_defaults"].append("jacoco classDirectories is limited to the debug class tree (tmp/kotlin-classes/debug); exclusions apply to compiled class files")
            res["note"] = "jacoco exclude() calls of %s" % base
            return res
    except FenceError as e:
        msg = str(e)
        res["patterns"] = None; res["unverifiable"] = msg; res["note"] = msg
        return res
    res["patterns"] = None; res["unverifiable"] = "applied_exclusions_unverifiable: unknown tool kind %r" % kind; res["note"] = res["unverifiable"]
    return res


def is_test_source(rel):
    """A test file by NAME or by SOURCE-SET directory: the denominator of the "more than half" rules is the production code the tool can instrument, not its tests."""
    parts = rel.split("/"); name = parts[-1]
    if GO_TEST_RE.search(name) or PY_TEST_RE.match(name) or JS_TEST_RE.search(name):
        return True
    low = [p.lower() for p in parts[:-1]]
    if any(p in ("__tests__", "__mocks__", "__snapshots__", "e2e", "testdata", "tests", "test", "fixtures", "__fixtures__") for p in low):
        return True
    if re.match(r"^(test_.*|.*_test|mutate_.*)\.sh$", name):
        return True
    return any(("/" + rel).find("/" + s_) >= 0 for s_ in ("src/test/", "src/androidTest/", "src/testFixtures/"))


def class_to_sources(pattern):
    """A jacoco class-file pattern -> the source patterns it names (`X.class` -> `X.kt`, `X.java`); None when the pattern is not a class pattern."""
    if pattern.endswith(".class"):
        stem = pattern[:-len(".class")]
        return [stem + ".kt", stem + ".java"]
    return None


# --------------------------------------------------------------------------- class evidence
GO_GENERATED = re.compile(rb"(?m)^// Code generated .* DO NOT EDIT\.$")
AT_GENERATED = re.compile(rb"(?m)^\s*(?://|#|/\*+|\*)\s*.*@generated\b")
TOOL_HEADER = re.compile(rb"(?im)^\s*(?://|#|/\*+|\*)\s*.*\b(?:auto-?generated|generated by|code generated)\b.*\b(?:do not edit|do not modify)\b")
FIXTURE_DIRS = {"testdata", "fixtures", "fixture", "golden", "__tests__", "__mocks__", "__snapshots__", "androidtest", "testfixtures"}
TEST_SOURCE_SETS = ("src/test/", "src/androidTest/", "src/testFixtures/", "src/test-utils/")
JS_TEST_RE = re.compile(r"(\.test\.|\.spec\.)[A-Za-z]+$")
GO_TEST_RE = re.compile(r"_test\.go$")
PY_TEST_RE = re.compile(r"(^test_.*\.py$|_test\.py$)")


def _read(p, n=4096):
    try:
        with open(p, "rb") as fh:
            return fh.read(n)
    except OSError:
        return None


def generated_output_dirs(ctx):
    """Directories (relative to the root) that a TRACKED project config declares as generator output: tsconfig outDir, a vite build.outDir, the Tauri gen directory."""
    if "gen_dirs" in ctx:
        return ctx["gen_dirs"]
    root = ctx["root_abs"]; dirs = {}
    for r in ctx["files"]:
        base = os.path.basename(r)
        if re.match(r"^tsconfig[\w.-]*\.json$", base):
            try:
                d = json.loads(js_strip_comments(open(os.path.join(root, r), encoding="utf-8").read()))
                od = (d.get("compilerOptions") or {}).get("outDir")
                if isinstance(od, str):
                    dirs[os.path.normpath(os.path.join(os.path.dirname(r), od)).replace(os.sep, "/")] = "tsconfig outDir of %s" % r
            except (OSError, ValueError):
                pass
        elif re.match(r"^v(?:ite|itest)\.config\.[cm]?[jt]s$", base):
            try:
                m = re.search(r"\bbuild\s*:\s*\{[^{}]*?\boutDir\s*:\s*['\"]([^'\"]+)['\"]", js_strip_comments(open(os.path.join(root, r), encoding="utf-8").read()), re.S)
                if m:
                    dirs[os.path.normpath(os.path.join(os.path.dirname(r), m.group(1))).replace(os.sep, "/")] = "vite build.outDir of %s" % r
            except OSError:
                pass
        elif base == "Cargo.toml":
            t = _read(os.path.join(root, r), 100000) or b""
            if re.search(rb"(?m)^\s*tauri-build\s*=", t):
                dirs[os.path.normpath(os.path.join(os.path.dirname(r), "gen")).replace(os.sep, "/")] = "tauri-build is a build-dependency of %s: its build script writes gen/" % r
    ctx["gen_dirs"] = dirs
    return dirs


def vendored_provenance(ctx, rel):
    """A provenance record for the vendored directory that holds `rel`, or None. A directory NAME alone is not provenance."""
    parts = rel.split("/")
    root = ctx["root_abs"]; files = ctx["files_set"]
    for k in range(1, len(parts)):
        d = "/".join(parts[:k])
        if parts[k - 1] == "vendor" and (d + "/modules.txt") in files:
            return "go vendor/modules.txt at %s" % d
        if parts[k - 1] == "node_modules" and any(f in files for f in ("package-lock.json", "pnpm-lock.yaml", "yarn.lock", "npm-shrinkwrap.json")):
            return "npm lockfile declares node_modules"
        lic = [f for f in files if f.startswith(d + "/") and f.count("/") == d.count("/") + 1 and re.match(r"(?i)(licen[sc]e|copying)", os.path.basename(f))]
        up = [f for f in files if f.startswith(d + "/") and f.count("/") == d.count("/") + 1 and re.match(r"(?i)(upstream|vendored|vendor_info|origin)(\..*)?$", os.path.basename(f))]
        if lic and up:
            t = _read(os.path.join(root, up[0]), 4096) or b""
            if re.search(rb"https?://\S+", t):
                return "%s + %s name the upstream" % (lic[0], up[0])
    for g in ctx.get("gitmodules", []):
        if rel == g or rel.startswith(g + "/"):
            return ".gitmodules entry %s" % g
    return None


def _go_module(ctx):
    if "gomod" in ctx:
        return ctx["gomod"]
    ctx["gomod"] = None
    for r in ctx["files"]:
        if r == "go.mod":
            m = re.search(rb"(?m)^module\s+(\S+)", _read(os.path.join(ctx["root_abs"], r), 20000) or b"")
            if m:
                ctx["gomod"] = m.group(1).decode().strip('"')
    return ctx["gomod"]


def go_importers(ctx, rel):
    """Non-test Go files of the root that import the package holding `rel` (an import search, 11.4.224 E fixtures class)."""
    mod = _go_module(ctx)
    if not mod:
        return None
    pkg_dir = os.path.dirname(rel)
    imp = mod + ("/" + pkg_dir if pkg_dir else "")
    cache = ctx.setdefault("go_imports", {})
    if "all" not in cache:
        allimps = []
        for r in ctx["files"]:
            if r.endswith(".go") and not GO_TEST_RE.search(r):
                t = _read(os.path.join(ctx["root_abs"], r), 1 << 20) or b""
                allimps.append((r, set(re.findall(rb'"([^"\n]+)"', t))))
        cache["all"] = allimps
    out = []
    for r, strs in cache["all"]:
        if os.path.dirname(r) == pkg_dir:
            continue
        if imp.encode() in strs:
            out.append(r)
    return out


def ts_importers(ctx, rel):
    """Non-test JS/TS source files that import a module inside the directory of `rel` (relative specifiers and the `@/` alias for src/)."""
    root = ctx["root_abs"]; target_dir = os.path.dirname(rel)
    out = []
    for r in ctx["files"]:
        if not r.endswith((".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".vue")) or JS_TEST_RE.search(r):
            continue
        if r == rel or r.startswith(target_dir + "/") or any(p in FIXTURE_DIRS or p in ("test", "tests", "test-utils", "test_utils", "__tests__") for p in r.split("/")[:-1]):
            continue
        t = (_read(os.path.join(root, r), 1 << 20) or b"").decode("utf-8", "replace")
        for sp in re.findall(r"""(?:from\s+|import\s*\(\s*|require\s*\(\s*|import\s+)['"]([^'"]+)['"]""", t):
            if sp.startswith("."):
                res = os.path.normpath(os.path.join(os.path.dirname(r), sp)).replace(os.sep, "/")
            elif sp.startswith("@/"):
                res = "src/" + sp[2:]
            else:
                continue
            if res == target_dir or res.startswith(target_dir + "/"):
                out.append(r); break
    return out


def class_evidence(cls, rel, ctx):
    """True when the file is consistent with the class its entry claims; (False, why) otherwise. first-party has no content evidence (it needs a tracked item or a lane)."""
    abs_path = os.path.join(ctx["root_abs"], rel)
    parts = rel.split("/")
    name = parts[-1]
    if cls == "generated-code":
        head = _read(abs_path, 4096)
        if head is not None:
            if GO_GENERATED.search(head):
                return True, ""
            first = b"\n".join(head.split(b"\n")[:30])
            if AT_GENERATED.search(first) or TOOL_HEADER.search(first):
                return True, ""
        for d, why in generated_output_dirs(ctx).items():
            if rel == d or rel.startswith(d.rstrip("/") + "/"):
                return True, ""
        return False, "no anchored generated-code header (Go `// Code generated ... DO NOT EDIT.`, `@generated`, a tool header with `DO NOT EDIT`) and not under a directory a tracked config declares as generator output"
    if cls == "vendored-third-party":
        why = vendored_provenance(ctx, rel)
        return (True, "") if why else (False, "no provenance record for the vendored directory (Go vendor/modules.txt, an npm lockfile, a .gitmodules entry, or LICENSE plus an upstream URL file): the directory name alone is not provenance")
    if cls == "non-shipping-fixtures-and-golden-assets":
        in_set = any(("/" + rel).find("/" + s) >= 0 or rel.startswith(s) for s in TEST_SOURCE_SETS)
        if rel.endswith((".kt", ".java")):
            return (True, "") if in_set else (False, "a Kotlin/Java file outside src/test, src/androidTest and src/testFixtures is not a test source, whatever its name")
        if GO_TEST_RE.search(name) or PY_TEST_RE.match(name) or JS_TEST_RE.search(name):
            return True, ""
        low = [p.lower() for p in parts[:-1]]
        in_fixture_dir = any(p in FIXTURE_DIRS for p in low) or in_set
        in_test_dir = any(p in ("test", "tests", "test-utils", "test_utils", "mocks", "mock", "e2e", "testing", "spec", "specs") for p in low)
        if in_fixture_dir:
            return True, ""
        if in_test_dir:
            if name.endswith(".go"):
                imp = go_importers(ctx, rel)
                if imp is None:
                    return False, "a non-test Go file in a test directory, and the Go module could not be read for the import search"
                return (True, "") if not imp else (False, "imported by non-test source %s" % ", ".join(imp[:3]))
            if name.endswith((".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".vue")):
                imp = ts_importers(ctx, rel)
                return (True, "") if not imp else (False, "imported by non-test source %s" % ", ".join(imp[:3]))
            return True, ""
        return False, "not a test source file (test file name) and not under a fixtures/test source-set directory"
    return True, ""
