#!/usr/bin/env bash
# Guard (CAT-TAURI-INSTALLER-1): every Tauri plugin command the installer-wizard frontend calls must be granted by a
# capability file, otherwise Tauri 2 answers "Command plugin:<name>|<cmd> not allowed by ACL" at runtime (measured live
# 2026-11-06: Save Configuration failed with "Command plugin:dialog|save not allowed by ACL").
# Scope: plugin-dialog open/save, the only plugin the wizard frontend imports.
# Pass rule: ONE capability JSON (parsed, not grepped) must target window "main" (windows = list of strings, globs allowed), grant EVERY
# needed permission (dialog:default counts), be local (local true, no remote), carry no "platforms" list excluding linux, and NO
# capability may deny a needed permission (dialog:deny-*). Raw invoke('plugin:dialog|open|save') strings count as usage. Import detection handles single/double quotes, multi-line and aliased named imports;
# `import * as` and dynamic `import()` are treated as needing both open and save (conservative).
# Exit codes: 0 PASS / 1 FAIL / 2 BLIND (no plugin-dialog use found, the scan cannot see anything).
# Paired mutation test: scripts/tauri/test_wizard_capabilities_mutation.sh
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
W="$ROOT/installer-wizard"
exec python3 -I - "$W" <<'PY'
import fnmatch, json, os, re, sys
W = sys.argv[1]
SRC = os.path.join(W, "src")
CAPS = os.path.join(W, "src-tauri", "capabilities")
PKG = r"""['"]@tauri-apps/plugin-dialog['"]"""
named = re.compile(r"import\s+(?:type\s+)?\{([^}]*)\}\s*from\s*" + PKG)
star = re.compile(r"import\s+\*\s+as\s+\w+\s+from\s*" + PKG)
dyn = re.compile(r"import\s*\(\s*" + PKG + r"\s*\)")
raw = re.compile(r"plugin:dialog\|(open|save)\b")
need, seen = set(), False
for dp, dns, fns in os.walk(SRC):
    dns[:] = [d for d in dns if d != "test"]
    for fn in fns:
        if not fn.endswith((".ts", ".tsx")):
            continue
        t = open(os.path.join(dp, fn), encoding="utf-8", errors="replace").read()
        for m in named.finditer(t):
            seen = True
            for item in m.group(1).split(","):
                name = item.strip().split(" as ")[0].replace("type ", "").strip()
                if name in ("open", "save"):
                    need.add("dialog:allow-" + name)
        for m in raw.finditer(t):
            seen = True
            need.add("dialog:allow-" + m.group(1))
        if star.search(t) or dyn.search(t):
            seen = True
            need.update(("dialog:allow-open", "dialog:allow-save"))
if not seen or not need:
    print("BLIND: no plugin-dialog open/save import found under %s" % SRC, file=sys.stderr)
    sys.exit(2)
ok, problems = [], []
EXPAND = {"dialog:default": {"dialog:allow-open", "dialog:allow-save"}}
def strlist(v):
    return isinstance(v, list) and all(isinstance(x, str) for x in v)
if not os.path.isdir(CAPS):
    problems.append("capabilities dir missing: %s" % CAPS)
else:
    caps = []
    for fn in sorted(os.listdir(CAPS)):
        if not fn.endswith(".json"):
            continue
        try:
            d = json.load(open(os.path.join(CAPS, fn), encoding="utf-8"))
        except Exception as e:
            problems.append("%s: invalid JSON (%s)" % (fn, e))
            continue
        caps += [(fn, c) for c in (d if isinstance(d, list) else [d]) if isinstance(c, dict)]
    def ids(c):
        return [p if isinstance(p, str) else p.get("identifier") for p in c.get("permissions", []) if isinstance(p, (str, dict))]
    denied = {"dialog:deny-" + n.split("-", 1)[1] for n in need}
    for fn, c in caps:
        hit = denied & set(ids(c))
        if hit:
            problems.append("%s: denies %s (deny wins over allow)" % (fn, sorted(hit)))
    if not any("denies" in p for p in problems):
        for fn, c in caps:
            perms = set(ids(c))
            for p in list(perms):
                perms |= EXPAND.get(p, set())
            wins, plats = c.get("windows"), c.get("platforms")
            if not strlist(wins):
                problems.append("%s: windows must be a list of strings" % fn)
            elif not any(fnmatch.fnmatchcase("main", w) for w in wins):
                problems.append("%s: capability does not target window main" % fn)
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
    print("PASS: %s granted to window main by %s" % (" ".join(sorted(need)), ", ".join(ok)))
    sys.exit(0)
print("FAIL: no single capability targets main on linux and grants %s" % sorted(need), file=sys.stderr)
for p in problems:
    print("  - " + p, file=sys.stderr)
sys.exit(1)
PY
