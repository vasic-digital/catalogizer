#!/usr/bin/env bash
# Guard (CAT-TAURI-INSTALLER-1): every Tauri plugin command the installer-wizard frontend calls must be granted by a
# capability file, otherwise Tauri 2 answers "Command plugin:<name>|<cmd> not allowed by ACL" at runtime (measured live
# 2026-11-06: Save Configuration failed with "Command plugin:dialog|save not allowed by ACL").
# Scope: plugin-dialog open/save/ask/confirm/message, the only plugin the wizard frontend imports.
# Pass rule: ONE enabled capability (parsed, not grepped; .json, .json5 [comments/trailing commas only; other json5 syntax fails closed],
# .toml, or an inline object in tauri.conf.json app.security.capabilities) must target the app's dialog window (label "main", else the first
# window label from tauri.conf.json app.windows; globs allowed), grant EVERY needed permission (dialog:default counts), be local (local true,
# no remote), carry no "platforms" list excluding linux, and NO capability may deny a needed permission (dialog:deny-*). If tauri.conf.json
# sets app.security.capabilities only those identifiers are enabled, otherwise every file in capabilities/ is. Raw invoke('plugin:dialog|<cmd>')
# strings count as usage wherever they appear in code or in string/template literals (any quoting, aliased invoke, constants, arrays).
# NO lexing/masking of source text at all (review rounds 1-2 found every comment/string masker hides real usage somewhere). Usage detection scans
# the RAW text of EVERY file under src/ (.ts .tsx .js .jsx .mjs .cjs .mts .cts, every directory incl. test/ and __tests__/, symlinked directories
# followed with a realpath loop guard; an unreadable directory is BLIND, exit 2). The text is scanned as written AND with \xNN / \uNNNN / \u{N}
# escapes decoded. Precise forms: `import|export {..} from '<pkg>'` (single/double/backtick quotes, multi-line, aliased) demand the named commands;
# invoke('plugin:dialog|<cmd>') demands <cmd>. Structural fallback (review round 4): ANY other occurrence of "plugin-dialog" or "plugin:dialog"
# (dynamic import(), require(), `import * as`, re-exports, constants, 'plugin:dialog|'+x, comments, strings) and ANY "__TAURI__" text, and
# app.withGlobalTauri true in tauri.conf.json, demand ALL five dialog permissions. Only vi.mock/jest.mock/doMock/unmock of the package are
# explained away (test-only). *.html anywhere under the wizard root (index.html, public/; not node_modules/dist/target/.git) is scanned the
# same way; extensions match case-insensitively. A non-empty src-tauri/permissions/ (app-defined permission sets) is BLIND (exit 2).
# Capability "webviews" fields are ignored (conservative); a remote capability with urls [] FAILs (conservative). Trade-off: a mention in a comment or string demands extra permissions (false positive accepted), a false negative is not.
# Parse problems in ANY capability file or tauri.conf.json are fatal (exit 2, BLIND), never ignored. Import detection handles single/double quotes, multi-line and aliased named imports; `import * as` and dynamic `import()` need all five
# commands (conservative); `export {..} from` / `export * from` re-exports count like imports.
# Capability files: Tauri CapabilityFile = Capability | List | NamedList {"capabilities":[..]}; all three are parsed per capability. Type errors in
# tauri.conf.json / capability data and an unterminated json5 block comment are BLIND (exit 2), never a traceback or a pass.
# Capability file with a "capabilities" key is a NamedList even next to a top-level "identifier" (Tauri tries NamedList first).
# dialog:default is counted as allow-open/allow-save/allow-message only (plugin-version dependent; crate unpinned); ask/confirm additionally
# demand allow-message (upstream v2 head may route them through plugin:dialog|message).
# Known limits (not detected): a plugin name split across string pieces ('plugin:' + 'dialog'), imports via a differently named re-export module,
# non-JS/TS source (.vue/.svelte), capabilities/ subdirectories, tauri.<platform>.conf.json / Tauri.toml, multi-window apps.
# Exit codes: 0 PASS / 1 FAIL / 2 BLIND (no plugin-dialog use found, or a config parse problem; NOT a pass).
# Paired mutation test: scripts/tauri/test_wizard_capabilities_mutation.sh
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
W="$ROOT/installer-wizard"
exec python3 -I - "$W" <<'PY'
import fnmatch, json, os, re, sys
try:
    import tomllib
except ImportError:
    tomllib = None
def _blind(t, v, tb):
    print("BLIND: internal guard error (%s: %s); refusing to pass or fail" % (t.__name__, v), file=sys.stderr)
    os._exit(2)
sys.excepthook = _blind
W = sys.argv[1]
SRC = os.path.join(W, "src")
CAPS = os.path.join(W, "src-tauri", "capabilities")
CONF = os.path.join(W, "src-tauri", "tauri.conf.json")
CMDS = ("open", "save", "ask", "confirm", "message")
PKG = r"""['"`]@tauri-apps/plugin-dialog['"`]"""
named = re.compile(r"(?:import|export)\s+(?:type\s+)?\{([^}]*)\}\s*from\s*" + PKG)
raw = re.compile(r"plugin:dialog\|(" + "|".join(CMDS) + r")\b")
# test-only module mocks never run in the app: explained, no permission demanded
mock = re.compile(r"\b(?:vi|jest)\.(?:mock|doMock|unmock)\s*\(\s*" + PKG)
# ANY mention of the plugin that no precise form above explains (dynamic import, require, constants, concatenation, comments, ...)
# demands all five. Same for the global API object.
mention = re.compile(r"plugin[-:]dialog")
EXTS = (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts")
ALL5 = {"dialog:allow-" + c for c in CMDS}
ESC = re.compile(r"\\x([0-9a-fA-F]{2})|\\u([0-9a-fA-F]{4})|\\u\{([0-9a-fA-F]{1,6})\}")
def unescape(t):
    def f(m):
        try:
            return chr(int(m.group(1) or m.group(2) or m.group(3), 16))
        except ValueError:
            return m.group(0)
    return ESC.sub(f, t)
need, seen = set(), False
def walk_error(e):
    raise e
real_seen = set()
SKIPDIRS = ("node_modules", "dist", "target", ".git")
PERMS = os.path.join(W, "src-tauri", "permissions")
if os.path.isdir(PERMS) and os.listdir(PERMS):
    print("BLIND: %s holds app-defined permission sets (not parsed); the guard cannot see what they grant" % PERMS, file=sys.stderr)
    sys.exit(2)
for dp, dns, fns in os.walk(W, followlinks=True, onerror=walk_error):
    keep = []
    for d in dns:
        if d in SKIPDIRS:
            continue
        rp = os.path.realpath(os.path.join(dp, d))
        if rp not in real_seen:  # symlink-loop guard: every real directory is walked once
            real_seen.add(rp)
            keep.append(d)
    dns[:] = keep
    for fn in fns:
        lo = fn.lower()
        in_src = dp == SRC or dp.startswith(SRC + os.sep)  # lexical: symlinked dirs under src/ count as src/
        if not (lo.endswith(".html") or (in_src and lo.endswith(EXTS))):
            continue
        t0 = open(os.path.join(dp, fn), encoding="utf-8", errors="replace").read()
        t1 = unescape(t0)
        for t in ((t0,) if t1 == t0 else (t0, t1)):
            explained = []
            for m in named.finditer(t):
                seen = True
                explained.append((m.start(), m.end()))
                for w in re.findall(r"[A-Za-z_$][\w$]*", m.group(1)):
                    if w in CMDS:
                        need.add("dialog:allow-" + w)
            for m in raw.finditer(t):
                seen = True
                explained.append((m.start(), m.end()))
                need.add("dialog:allow-" + m.group(1))
            for m in mock.finditer(t):
                explained.append((m.start(), m.end()))
            for m in mention.finditer(t):
                if not any(a <= m.start() < b for a, b in explained):
                    seen = True
                    need |= ALL5
            if "__TAURI__" in t:
                seen = True
                need |= ALL5
if not seen or not need:
    print("BLIND: no plugin-dialog import found under %s" % SRC, file=sys.stderr)
    sys.exit(2)
# ask/confirm may use plugin:dialog|message internally (upstream v2 head), so demand allow-message with them
if need & {"dialog:allow-ask", "dialog:allow-confirm"}:
    need.add("dialog:allow-message")
ok, problems, fatal = [], [], []
denied_hit = False
# dialog:default expansion is plugin-version dependent (dialog-v2.6.0: all five; upstream v2 head: allow-message, allow-save, allow-open only).
# The Rust crate is "2" unpinned and no Cargo.lock is tracked, so assume the narrower set.
EXPAND = {"dialog:default": {"dialog:allow-open", "dialog:allow-save", "dialog:allow-message"}}
def strlist(v):
    return isinstance(v, list) and all(isinstance(x, str) for x in v)
def mask_json5(t):
    out, i, n = [], 0, len(t)
    while i < n:
        c = t[i]
        if t.startswith("//", i):
            j = t.find("\n", i); i = n if j < 0 else j
        elif t.startswith("/*", i):
            j = t.find("*/", i + 2)
            if j < 0:
                raise ValueError("unterminated /* comment")
            i = j + 2
        elif c == '"':
            j = i + 1
            while j < n and t[j] != '"':
                j += 2 if t[j] == "\\" else 1
            out.append(t[i:j + 1]); i = j + 1
        else:
            out.append(c); i += 1
    return "".join(out)
def json5_load(txt):
    return json.loads(re.sub(r",(\s*[}\]])", r"\1", mask_json5(txt)))
def check_cap(c, where):
    """Shape validation: anything Tauri could not load as a Capability is a BLIND problem, never a crash or a pass."""
    if not isinstance(c, dict):
        raise ValueError("%s: capability is not an object" % where)
    if not isinstance(c.get("identifier"), str):
        raise ValueError("%s: capability identifier missing or not a string" % where)
    perms = c.get("permissions", [])
    if not isinstance(perms, list):
        raise ValueError("%s: permissions is not a list" % where)
    for p in perms:
        if not (isinstance(p, str) or (isinstance(p, dict) and isinstance(p.get("identifier"), str))):
            raise ValueError("%s: permission entry %r is neither a string nor an object with a string identifier" % (where, p))
def cap_file_caps(d, where):
    """Tauri CapabilityFile = Capability | List(Vec<Capability>) | NamedList{capabilities:Vec<Capability>}."""
    if isinstance(d, dict) and "capabilities" in d:
        # Tauri deserializes NamedList BEFORE Capability (untagged enum), so this key wins even next to a top-level "identifier"
        if not isinstance(d["capabilities"], list):
            raise ValueError("%s: \"capabilities\" is not a list" % where)
        items = d["capabilities"]
    elif isinstance(d, list):
        items = d
    elif isinstance(d, dict) and "identifier" in d:
        items = [d]
    else:
        raise ValueError("%s: not a Capability, a list of them, or {\"capabilities\": [..]}" % where)
    for c in items:
        check_cap(c, where)
    return items
# tauri.conf.json: window labels + enabled capabilities
labels, enabled, inline = ["main"], None, []
if os.path.exists(CONF):
    try:
        conf = json.load(open(CONF, encoding="utf-8"))
        if not isinstance(conf, dict):
            raise ValueError("top level is not an object")
        app = conf.get("app", {})
        if not isinstance(app, dict):
            raise ValueError("app is not an object")
        wins = app.get("windows", [])
        if not isinstance(wins, list) or not all(isinstance(w, dict) for w in wins):
            raise ValueError("app.windows is not a list of objects")
        wl = []
        for w in wins:
            if not isinstance(w.get("label", "main"), str):
                raise ValueError("app.windows[].label is not a string")
            wl.append(w.get("label", "main"))
        if wl:
            labels = wl
        wg = app.get("withGlobalTauri", False)
        if not isinstance(wg, bool):
            raise ValueError("app.withGlobalTauri is not a boolean")
        if wg:
            seen = True
            need |= ALL5
        sec = app.get("security")
        if sec is None:
            sec = {}
        if not isinstance(sec, dict):
            raise ValueError("app.security is not an object")
        cl = sec.get("capabilities")
        if cl is not None:
            if not isinstance(cl, list) or not all(isinstance(x, (str, dict)) for x in cl):
                raise ValueError("app.security.capabilities is not a list of identifiers/objects")
            enabled = {x for x in cl if isinstance(x, str)}
            for x in cl:
                if isinstance(x, dict):
                    check_cap(x, "tauri.conf.json")
                    inline.append(("tauri.conf.json", x))
    except Exception as e:
        fatal.append("tauri.conf.json: invalid (%s)" % e)
        labels = None
target = None if labels is None else ("main" if "main" in labels else labels[0])
if not os.path.isdir(CAPS) and not inline:
    problems.append("capabilities dir missing: %s" % CAPS)
else:
    caps = []
    for fn in (sorted(os.listdir(CAPS)) if os.path.isdir(CAPS) else []):
        path = os.path.join(CAPS, fn)
        try:
            if fn.endswith(".json"):
                d = json.load(open(path, encoding="utf-8"))
            elif fn.endswith(".json5"):
                d = json5_load(open(path, encoding="utf-8").read())
            elif fn.endswith(".toml"):
                if tomllib is None:
                    raise RuntimeError("python tomllib unavailable (need 3.11+)")
                d = tomllib.load(open(path, "rb"))
            else:
                continue
            for c in cap_file_caps(d, fn):
                if enabled is None or c["identifier"] in enabled:
                    caps.append((fn, c))
        except Exception as e:
            fatal.append("%s: unparsable/invalid (%s)" % (fn, e))
    caps += inline
    def ids(c):
        return [p if isinstance(p, str) else p["identifier"] for p in c.get("permissions", [])]
    denied = {"dialog:deny-" + n.split("-", 1)[1] for n in need}
    for fn, c in caps:
        hit = denied & set(ids(c))
        if hit:
            denied_hit = True
            problems.append("%s: denies %s (deny wins over allow)" % (fn, sorted(hit)))
    if target is not None and not denied_hit:
        for fn, c in caps:
            perms = set(ids(c))
            for p in list(perms):
                perms |= EXPAND.get(p, set())
            wins, plats = c.get("windows"), c.get("platforms")
            if not strlist(wins):
                problems.append("%s: windows must be a list of strings" % fn)
            elif not any(fnmatch.fnmatchcase(target, w) for w in wins):
                problems.append("%s: capability does not target window %s" % (fn, target))
            elif plats is not None and (not strlist(plats) or "linux" not in plats):
                problems.append("%s: platforms %r missing linux or not a list of strings" % (fn, plats))
            elif c.get("local", True) is not True:
                problems.append("%s: local is not true (remote-only capability)" % fn)
            elif c.get("remote"):
                problems.append("%s: grants to remote origins" % fn)
            elif not need <= perms:
                problems.append("%s: missing %s" % (fn, sorted(need - perms)))
            else:
                ok.append(fn)
if fatal:
    print("BLIND: capability/config parse problem(s); the guard cannot see the real ACL, refusing to pass or fail:", file=sys.stderr)
    for p in fatal:
        print("  - " + p, file=sys.stderr)
    sys.exit(2)
if ok:
    print("PASS: %s granted to window %s by %s" % (" ".join(sorted(need)), target, ", ".join(ok)))
    sys.exit(0)
print("FAIL: no single enabled capability targets window %s on linux and grants %s" % (target, sorted(need)), file=sys.stderr)
for p in problems:
    print("  - " + p, file=sys.stderr)
sys.exit(1)
PY
