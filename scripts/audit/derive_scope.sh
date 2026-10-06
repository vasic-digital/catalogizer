#!/usr/bin/env bash
# derive_scope.sh - classify every git submodule at every depth as own or third_party (T017, docs/02 4.3).
#
# Purpose   Walk .gitmodules recursively (the root file, then the .gitmodules of every checked-out submodule at any
#           depth) and write one TSV row per submodule: the own-org versus third-party split that the index scope, the
#           repository verifier and the coverage corpus consume. Read-only: never writes inside the repository, never
#           touches the network.
# Usage     scripts/audit/derive_scope.sh [--root DIR] [--out FILE] [--own-orgs FILE] [--pending-orgs FILE]
#             --root DIR          repository root (default: git toplevel of $PWD)
#             --out FILE          TSV output (default: specs/001-full-project-audit-remediation/audit/submodules.tsv)
#             --own-orgs FILE     one organisation per line (default: scripts/audit/own_orgs.txt beside this script)
#             --pending-orgs FILE accounts BLOCKED-ON ODG-15 (default: scripts/audit/own_orgs_pending.txt), optional
# Inputs    .gitmodules files only (read with `git config -f`), so a submodule that is declared but not checked out (no
#           .git entry in its directory: missing, or the empty directory an uninitialised submodule leaves) is listed
#           but its own nested modules cannot be seen (reported on stderr as `not-descended <path>`, exit still 0: its
#           files are not on disk either, so nothing of it can enter an index until it is initialised and this is re-run).
#           The organisation parser is scripts/audit/org_of.py, shared with scripts/repo/verify_repos.sh.
# Outputs   TSV, tab separated, sorted by path, no header, five columns: path, class (own|third_party), URL, alt_class,
#           flag. `class` counts the pending accounts as third_party (the conservative own_orgs.txt reading), `alt_class`
#           counts them as own, `flag` is ODG-15 on a row where the two differ, else "-" (docs/21 IC-30).
#           A module nested under a third_party module is third_party whatever its own URL says (11.4.79(6)).
#           Organisation match is case-insensitive; the organisation is the second-last path segment of the URL.
# Exit      0 ok; 2 usage; 3 fail closed (a URL that cannot be classified, an unreadable .gitmodules or orgs file);
#           on exit 3 nothing is written to --out.
# Side effects  writes --out (and its parent directory) only on success. Needs bash, git, python3.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="" OUT="" OWN="$HERE/own_orgs.txt" PEND="$HERE/own_orgs_pending.txt"
while [ $# -gt 0 ]; do
  case "$1" in
    --root) [ $# -ge 2 ] || { echo "--root needs a value" >&2; exit 2; }; ROOT="$2"; shift 2 ;;
    --out) [ $# -ge 2 ] || { echo "--out needs a value" >&2; exit 2; }; OUT="$2"; shift 2 ;;
    --own-orgs) [ $# -ge 2 ] || { echo "--own-orgs needs a value" >&2; exit 2; }; OWN="$2"; shift 2 ;;
    --pending-orgs) [ $# -ge 2 ] || { echo "--pending-orgs needs a value" >&2; exit 2; }; PEND="$2"; shift 2 ;;
    -h|--help) sed -n '2,26p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
[ -n "$ROOT" ] || ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "not in a git repository and no --root" >&2; exit 2; }
[ -d "$ROOT" ] || { echo "root not a directory: $ROOT" >&2; exit 2; }
[ -n "$OUT" ] || OUT="$ROOT/specs/001-full-project-audit-remediation/audit/submodules.tsv"
[ -r "$OWN" ] || { echo "own-orgs file unreadable: $OWN" >&2; exit 3; }
TMP="$(mktemp "${TMPDIR:-/tmp}/derive_scope.XXXXXX")"; trap 'rm -f "$TMP"' EXIT
python3 - "$ROOT" "$OWN" "$PEND" "$TMP" "$HERE" <<'PY'
import os, re, subprocess, sys
root, own_f, pend_f, out, here = sys.argv[1:6]
sys.path.insert(0, here)
from org_of import org_of   # the ONE organisation parser, shared with scripts/repo/verify_repos.sh
def orgs(p, required):
    if not os.path.exists(p):
        if required: print("orgs file missing: " + p, file=sys.stderr); sys.exit(3)
        return set()
    try:
        with open(p) as fh:
            return {l.strip().lower() for l in fh if l.strip() and not l.lstrip().startswith("#")}
    except OSError as e:
        print("orgs file unreadable: %s (%s)" % (p, e), file=sys.stderr); sys.exit(3)
own, pend = orgs(own_f, True), orgs(pend_f, False)
def modules(d):
    f = os.path.join(d, ".gitmodules")
    if not os.path.isfile(f): return []
    r = subprocess.run(["git", "config", "-f", f, "--get-regexp", r"^submodule\..*\.(path|url)$"], capture_output=True, text=True)
    if r.returncode not in (0, 1): print("unreadable " + f, file=sys.stderr); sys.exit(3)
    mods = {}
    for line in r.stdout.splitlines():
        k, _, v = line.partition(" ")
        name, _, field = k[len("submodule."):].rpartition(".")
        mods.setdefault(name, {})[field] = v
    return [(m.get("path"), m.get("url")) for m in mods.values()]
rows, bad = [], []
def walk(d, rel, third, alt_third):
    for path, url in modules(d):
        if not path: bad.append((rel, url)); continue
        full = (rel + "/" + path) if rel else path
        o = org_of(url)
        if o is None: bad.append((full, url)); continue
        cls = "third_party" if third else ("own" if o in own else "third_party")
        alt = "third_party" if alt_third else ("own" if (o in own or o in pend) else "third_party")
        rows.append((full, cls, url, alt, "ODG-15" if cls != alt else "-"))
        sub = os.path.join(root, full)
        if os.path.exists(os.path.join(sub, ".git")):
            walk(sub, full, cls == "third_party", alt == "third_party")
        else:   # not checked out (missing directory, or an empty directory left by an uninitialised submodule)
            print("not-descended " + full, file=sys.stderr)
walk(root, "", False, False)
if bad:
    for p, u in bad: print("unclassifiable: %s url=%r" % (p, u), file=sys.stderr)
    sys.exit(3)
with open(out, "w") as f:
    for r in sorted(rows): f.write("\t".join(r) + "\n")
PY
rc=$?
[ "$rc" -eq 0 ] || exit "$rc"
mkdir -p "$(dirname "$OUT")" && cp "$TMP" "$OUT" || exit 3
exit 0
