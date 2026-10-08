#!/usr/bin/env bash
# check_evidence_binding.sh - WF17 fix round 5, TI-H1. Evidence must bind to the committed files: a capture's `# sha256 <path> <hash>` identity header is what the file under test hashed to WHEN the capture ran, and
# a reader on a clean checkout can only verify it against the COMMITTED content. Checked per evidence directory (recursively):
#   H  every `# sha256 <path> <hash>` header line of every tracked text file equals the sha256 of that path's content in the reference tree (`ABSENT` requires the path to be absent there)
#   M  every SHA256SUMS* manifest: each entry names a TRACKED file (never an untracked one: a clean checkout would not have it), reads, and has the listed hash; and every tracked file that sits next to
#      or below the manifest (not under a deeper directory that has a manifest of its own) is listed in it (a file added after the manifest was generated is a finding, not an omission to ignore)
# Reference tree: `head` (default: the committed content, `git show HEAD:<path>`; what a clean checkout holds) | `worktree` (the working tree, for the stage before the commit: tracked = `git ls-files -c -o --exclude-standard`).
# Manifests and tracked sets come from git, never from `find`: an ignored or untracked file can neither satisfy nor pollute them.
# --manifests-only skips (H): for HISTORICAL evidence whose headers name the revision they ran against (superseded by a later round), while its manifests must still verify.
# Usage:  check_evidence_binding.sh [--against head|worktree] [--manifests-only] [--root <repo>] <dir>...      Exit: 0 clean; 1 findings (printed one per line); 2 usage; 3 not a git repository
set -u
AGAINST=head; ROOT=""; DIRS=(); HDRS=1
while [ $# -gt 0 ]; do
  case "$1" in
    --against) [ $# -ge 2 ] || { echo "check_evidence_binding: --against needs a value" >&2; exit 2; }; AGAINST=$2; shift 2;;
    --manifests-only) HDRS=0; shift;;
    --root) [ $# -ge 2 ] || { echo "check_evidence_binding: --root needs a value" >&2; exit 2; }; ROOT=$2; shift 2;;
    -*) echo "check_evidence_binding: unknown option $1" >&2; exit 2;;
    *) DIRS+=("$1"); shift;;
  esac
done
case "$AGAINST" in head|worktree) ;; *) echo "check_evidence_binding: --against must be head or worktree" >&2; exit 2;; esac
[ "${#DIRS[@]}" -gt 0 ] || { echo "usage: check_evidence_binding.sh [--against head|worktree] [--root <repo>] <dir>..." >&2; exit 2; }
[ -n "$ROOT" ] || ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1 || { echo "check_evidence_binding: $ROOT is not a git repository" >&2; exit 3; }
exec python3 -I - "$ROOT" "$AGAINST" "$HDRS" "${DIRS[@]}" <<'PY'
import hashlib, os, re, subprocess, sys
root, against, hdrs, dirs = sys.argv[1], sys.argv[2], sys.argv[3] == "1", sys.argv[4:]
def git(*a, text=True):
    r = subprocess.run(["git", "-C", root] + list(a), capture_output=True, text=text)
    return r
def tracked():
    if against == "head":
        r = git("ls-tree", "-r", "--name-only", "-z", "HEAD", text=False)
        names = r.stdout.decode("utf-8", "surrogateescape").split("\0") if r.returncode == 0 else []
    else:
        r = git("ls-files", "-c", "-o", "--exclude-standard", "-z", text=False)
        names = r.stdout.decode("utf-8", "surrogateescape").split("\0")
    return {n for n in names if n}
def content(path):
    if against == "head":
        r = subprocess.run(["git", "-C", root, "show", "HEAD:" + path], capture_output=True)
        return r.stdout if r.returncode == 0 else None
    p = os.path.join(root, path)
    try:
        with open(p, "rb") as f: return f.read()
    except OSError: return None
T = tracked(); findings = []; nfiles = 0; nhdr = 0; nman = 0
HDR = re.compile(r"^# sha256 (\S+) ([0-9a-f]{64}|ABSENT)\s*$")
ENT = re.compile(r"^([0-9a-f]{64})  (\./)?(.+)$")
for d in dirs:
    rel = os.path.relpath(os.path.join(root, d) if not os.path.isabs(d) else d, root).rstrip("/")
    under = sorted(n for n in T if n == rel or n.startswith(rel + "/"))
    if not under: findings.append("%s: no tracked file under this directory (%s): nothing to bind" % (rel, against)); continue
    manifests = [n for n in under if os.path.basename(n).startswith("SHA256SUMS")]
    for n in under:
        b = os.path.basename(n)
        if b.startswith("SHA256SUMS"): continue
        c = content(n)
        if c is None: findings.append("%s: tracked but unreadable in the %s tree" % (n, against)); continue
        nfiles += 1
        if not hdrs: continue   # --manifests-only: historical captures keep the headers of the revision they ran against
        try: txt = c.decode("utf-8")
        except UnicodeDecodeError: continue
        for ln, line in enumerate(txt.split("\n")[:2000], 1):
            if not line.startswith("#"): break   # the identity block is the leading run of comment lines (capture.sh writes it first)
            m = HDR.match(line.rstrip("\r"))
            if not m: continue
            nhdr += 1; p, h = m.group(1), m.group(2)
            ref = content(p)
            if h == "ABSENT":
                if ref is not None: findings.append("%s:%d: header says %s was ABSENT but it exists in the %s tree" % (n, ln, p, against))
            elif ref is None: findings.append("%s:%d: header names %s which is not in the %s tree" % (n, ln, p, against))
            elif hashlib.sha256(ref).hexdigest() != h: findings.append("%s:%d: header hash of %s (%s) is not the %s content (%s)" % (n, ln, p, h[:12], against, hashlib.sha256(ref).hexdigest()[:12]))
    bydir = {}
    for mf in manifests:
        nman += 1; mdir = os.path.dirname(mf); listed = bydir.setdefault(mdir, set())
        c = content(mf)
        if c is None: findings.append("%s: manifest unreadable" % mf); continue
        for ln, line in enumerate(c.decode("utf-8", "replace").split("\n"), 1):
            line = line.rstrip("\r")
            if not line.strip() or line.startswith("#"): continue
            m = ENT.match(line)
            if not m: findings.append("%s:%d: not a `<sha256>  <path>` entry: %r" % (mf, ln, line[:60])); continue
            h, p = m.group(1), os.path.normpath(os.path.join(mdir, m.group(3)))
            listed.add(p)
            if p not in T: findings.append("%s:%d: %s is not a tracked file (a clean checkout would not have it)" % (mf, ln, p)); continue
            f = content(p)
            if f is None: findings.append("%s:%d: %s cannot be read" % (mf, ln, p))
            elif hashlib.sha256(f).hexdigest() != h: findings.append("%s:%d: %s hashes to %s, the manifest says %s" % (mf, ln, p, hashlib.sha256(f).hexdigest()[:12], h[:12]))
    # completeness: every tracked file at or below a manifest directory (and not below a deeper manifest directory) is listed in the union of that directory's manifests
    for mdir, listed in bydir.items():
        deeper = [dd for dd in bydir if dd != mdir and dd.startswith(mdir + "/")]
        for n in under:
            if os.path.basename(n).startswith("SHA256SUMS") or not n.startswith(mdir + "/"): continue
            if any(n.startswith(dd + "/") for dd in deeper): continue
            if n not in listed: findings.append("%s: tracked file %s is not listed in any manifest of %s" % (mdir, n, mdir))
for f in findings: print("FAIL " + f)
print("check_evidence_binding: against=%s dirs=%d files=%d headers=%d manifests=%d findings=%d" % (against, len(dirs), nfiles, nhdr, nman, len(findings)))
sys.exit(1 if findings else 0)
PY
