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
# strings count as usage. Comments and string/template literals are masked before import detection (an import inside them is NOT usage);
# import detection handles single/double quotes, multi-line and aliased named imports; `import * as` and dynamic `import()` need all five
# commands (conservative). Limitation: regex literals containing quotes are not lexed.
# Exit codes: 0 PASS / 1 FAIL / 2 BLIND (no plugin-dialog use found, the scan cannot see anything; NOT a pass).
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
W = sys.argv[1]
SRC = os.path.join(W, "src")
CAPS = os.path.join(W, "src-tauri", "capabilities")
CONF = os.path.join(W, "src-tauri", "tauri.conf.json")
CMDS = ("open", "save", "ask", "confirm", "message")
PKG = r"""['"]@tauri-apps/plugin-dialog['"]"""
named = re.compile(r"import\s+(?:type\s+)?\{([^}]*)\}\s*from\s*" + PKG)
star = re.compile(r"import\s+\*\s+as\s+\w+\s+from\s*" + PKG)
dyn = re.compile(r"import\s*\(\s*" + PKG + r"\s*\)")
raw = re.compile(r"plugin:dialog\|(" + "|".join(CMDS) + r")\b")
KEEP = re.compile(r"(?:\bfrom|\bimport|\bimport\s*\(|\binvoke\s*\(|\binvoke\s*<[^>]*>\s*\()\s*$")
def mask(t):
    """Blank comments and string/template bodies, except module specifiers / invoke command strings."""
    out, i, n = [], 0, len(t)
    while i < n:
        c = t[i]
        if t.startswith("//", i):
            j = t.find("\n", i); j = n if j < 0 else j
            out.append(" " * (j - i)); i = j
        elif t.startswith("/*", i):
            j = t.find("*/", i + 2); j = n if j < 0 else j + 2
            out.append(re.sub(r"[^\n]", " ", t[i:j])); i = j
        elif c in "'\"`":
            j = i + 1
            while j < n and t[j] != c:
                j += 2 if t[j] == "\\" else 1
            j = min(j + 1, n)
            lit = t[i:j]
            keep = c != "`" and KEEP.search("".join(out)[-40:]) is not None
            out.append(lit if keep else c + re.sub(r"[^\n]", " ", lit[1:-1]) + (c if lit.endswith(c) and len(lit) > 1 else ""))
            i = j
        else:
            out.append(c); i += 1
    return "".join(out)
need, seen = set(), False
for dp, dns, fns in os.walk(SRC):
    dns[:] = [d for d in dns if d != "test"]
    for fn in fns:
        if not fn.endswith((".ts", ".tsx")):
            continue
        t = mask(open(os.path.join(dp, fn), encoding="utf-8", errors="replace").read())
        for m in named.finditer(t):
            seen = True
            for item in m.group(1).split(","):
                name = item.strip().split(" as ")[0].replace("type ", "").strip()
                if name in CMDS:
                    need.add("dialog:allow-" + name)
        for m in raw.finditer(t):
            seen = True
            need.add("dialog:allow-" + m.group(1))
        if star.search(t) or dyn.search(t):
            seen = True
            need.update("dialog:allow-" + c for c in CMDS)
if not seen or not need:
    print("BLIND: no plugin-dialog import found under %s" % SRC, file=sys.stderr)
    sys.exit(2)
ok, problems = [], []
EXPAND = {"dialog:default": {"dialog:allow-" + c for c in CMDS}}
def strlist(v):
    return isinstance(v, list) and all(isinstance(x, str) for x in v)
def json5_load(txt):
    txt = mask_json5(txt)
    return json.loads(re.sub(r",(\s*[}\]])", r"\1", txt))
def mask_json5(t):
    out, i, n = [], 0, len(t)
    while i < n:
        c = t[i]
        if t.startswith("//", i):
            j = t.find("\n", i); i = n if j < 0 else j
        elif t.startswith("/*", i):
            j = t.find("*/", i + 2); i = n if j < 0 else j + 2
        elif c == '"':
            j = i + 1
            while j < n and t[j] != '"':
                j += 2 if t[j] == "\\" else 1
            out.append(t[i:j + 1]); i = j + 1
        else:
            out.append(c); i += 1
    return "".join(out)
# tauri.conf.json: window labels + enabled capabilities
labels, enabled, inline = ["main"], None, []
if os.path.exists(CONF):
    try:
        conf = json.load(open(CONF, encoding="utf-8"))
        app = conf.get("app", {}) if isinstance(conf, dict) else {}
        wl = [w.get("label", "main") for w in app.get("windows", []) if isinstance(w, dict)]
        if wl:
            labels = wl
        cl = (app.get("security") or {}).get("capabilities")
        if isinstance(cl, list):
            enabled = {x for x in cl if isinstance(x, str)}
            inline = [("tauri.conf.json", x) for x in cl if isinstance(x, dict)]
    except Exception as e:
        problems.append("tauri.conf.json: invalid (%s)" % e)
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
        except Exception as e:
            problems.append("%s: unparsable (%s)" % (fn, e))
            continue
        for c in (d if isinstance(d, list) else [d]):
            if isinstance(c, dict) and (enabled is None or c.get("identifier") in enabled):
                caps.append((fn, c))
    caps += inline
    def ids(c):
        return [p if isinstance(p, str) else p.get("identifier") for p in c.get("permissions", []) if isinstance(p, (str, dict))]
    denied = {"dialog:deny-" + n.split("-", 1)[1] for n in need}
    for fn, c in caps:
        hit = denied & set(ids(c))
        if hit:
            problems.append("%s: denies %s (deny wins over allow)" % (fn, sorted(hit)))
    if target is not None and not any("denies" in p for p in problems):
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
if ok:
    print("PASS: %s granted to window %s by %s" % (" ".join(sorted(need)), target, ", ".join(ok)))
    sys.exit(0)
print("FAIL: no single enabled capability targets window %s on linux and grants %s" % (target, sorted(need)), file=sys.stderr)
for p in problems:
    print("  - " + p, file=sys.stderr)
sys.exit(1)
PY
