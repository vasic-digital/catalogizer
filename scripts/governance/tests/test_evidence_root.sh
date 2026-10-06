#!/usr/bin/env bash
# test_evidence_root.sh: the feature's evidence root is $EV = $FEAT/evidence (tasks.md, phase-group conventions);
#   no evidence may live in a repo-root evidence/ path, and no script may name or default to a repo-root evidence path.
#   Revision 2 (fix round 3 after the xhigh review WF2 governance, findings I6, E1 to E4, m7).
# Usage: bash scripts/governance/tests/test_evidence_root.sh
# Env:   EVROOT_SCAN_ROOT  repository root to check (default: the repo root of this script); the self-test scenarios
#                          build scratch git work trees and never touch the real tree.
# Exit:  0 all checks pass and every control needle and mutation scenario behaved; 1 otherwise.
# Checks:
#   R1 nothing named <root>/evidence exists (directory, file or symlink: -e / -L; the tools/evidence Go tree is another path).
#   R2 every text file under scripts/ and tools/ (git ls-files -co --exclude-standard; this file excluded) is scanned for an
#      'evidence/' path segment that is not rooted at $EV, $FEAT, ${FEAT} or specs/<feature>/ :
#        A  'evidence/' not directly preceded by a path character (start, space, quote, '=', '(', '>', ':-', ',' ...),
#           so 'mkdir -p evidence/wp09', '"evidence/ledger.jsonl"', '${LEDGER:-evidence/x}' and prose 'see evidence/wp01' match;
#        B  './evidence/' and '$ROOT|$root|$REPO|$REPO_ROOT|$REAL_ROOT|$PWD|$(pwd)/evidence/' (the likely real defaults).
#      '$FEAT/evidence/', 'specs/x/evidence/', 'tools/evidence/' and '$OUTPUT_DIR/evidence/' are preceded by a path
#      character and do not match. Use '$EV/wp01/...' for an evidence path in prose (review m3).
#   R3 residue: RESIDUE lists, per file of an OTHER area that this fix round may not edit, the exact number of prose or fixture
#      hit lines, with the reason. A file with more or fewer hits than listed is a FAILURE (a new hit, or a stale entry): it only shrinks.
#   R4 control needles and mutation scenarios in scratch trees (review E1 to E4): each pattern class is seen, each legal
#      path is not matched, an unreadable file is a failure, a non-git root is refused as blind.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export REAL_ROOT="$(cd "$HERE/../../.." && pwd)"
export SCAN_ROOT="${EVROOT_SCAN_ROOT:-$REAL_ROOT}"
export SELF_REL=scripts/governance/tests/test_evidence_root.sh
python3 - <<'PY'
import os, re, subprocess, sys, tempfile, shutil, stat

ROOT = os.environ["SCAN_ROOT"]; REAL_ROOT = os.environ["REAL_ROOT"]; SELF_REL = os.environ["SELF_REL"]
RX = re.compile(
    rb"(?<![A-Za-z0-9_./}$])(?<![A-Za-z0-9_]-)evidence/"
    rb"|(?<![A-Za-z0-9_$}])\./evidence/"
    rb"|\$\{?(?:ROOT|root|REPO|REPO_ROOT|REAL_ROOT|PWD|REPO_DIR)\}?/evidence/"
    rb"|\$\(pwd\)/evidence/")
SCOPE = ("scripts/", "tools/")
# (path, exact substring of the hit line, reason). Files of other areas (scripts/repo, tools/evidence); not edited in fix round 3.
RESIDUE = {   # path -> (exact number of hit lines, reason). All are prose or fixture strings naming a $EV-relative path (review m3),
              # read line by line when this list was made; none is a write or a default. Files of OTHER areas, not edited in fix round 3.
    "scripts/register/register_ext.sql": (1, "comment naming evidence/wp06/README.md (area scripts/register)"),
    "scripts/repo/verify_repos.sh": (1, "comment naming evidence/wp03/... (area scripts/repo)"),
    "scripts/repo/tests/test_verify_repos.sh": (2, "comments naming evidence/wp03/... (area scripts/repo)"),
    "scripts/repo/validate_checks.tsv": (1, "table text naming evidence/wp04/ (area scripts/repo)"),
    "tools/evidence/evcore.py": (1, "docstring naming evidence/wp05/... (area tools/evidence)"),
    "tools/evidence/tests/capture_evidence.sh": (3, "comments and header text of the capture script naming evidence/wp05/... and the rule itself (area tools/evidence)"),
    "tools/evidence/tests/test_evrec_r3.sh": (1, "comment naming evidence/wp05/... (area tools/evidence)"),
    "tools/evidence/tests/test_evidence_record_schema.sh": (2, "schema fixture strings holding a $EV-relative value, not a write (area tools/evidence)"),
}
fails = []
def bad(m): fails.append(m); print("FAIL " + m)
def ok(m): print("ok   " + m)

def files(root):
    top = subprocess.run(["git", "-C", root, "rev-parse", "--show-toplevel"], capture_output=True)
    if top.returncode != 0 or os.path.realpath(top.stdout.decode().strip()) != os.path.realpath(root): return None
    r = subprocess.run(["git", "-C", root, "ls-files", "-z", "-c", "-o", "--exclude-standard"], capture_output=True)
    return sorted({p for p in r.stdout.split(b"\0") if p}) if r.returncode == 0 else None

def scan(root):
    """returns (hits, errors): hits = [(rel, lineno, text)]"""
    hits, errs = [], []
    fl = files(root)
    if fl is None: return [], ["%s is not a git work tree root: the scope cannot be enumerated (instrument blind)" % root]
    n_scanned = 0
    for pb in fl:
        rel = pb.decode("utf-8", "replace")
        if not rel.startswith(SCOPE) or rel == SELF_REL: continue
        full = os.path.join(root.encode(), pb)
        try: st = os.lstat(full)
        except OSError as e: errs.append("%s: UNREADABLE (%s)" % (rel, e)); continue
        if stat.S_ISLNK(st.st_mode) or stat.S_ISDIR(st.st_mode): continue
        try:
            with open(full, "rb") as fh: data = fh.read()
        except OSError as e: errs.append("%s: UNREADABLE (%s)" % (rel, e)); continue
        n_scanned += 1
        if b"\0" in data: continue
        for i, ln in enumerate(data.split(b"\n"), 1):
            if RX.search(ln): hits.append((rel, i, ln.decode("utf-8", "replace")))
    if n_scanned == 0: errs.append("0 files scanned under %s (instrument blind)" % (SCOPE,))
    return hits, errs

def r1(root): return [n for n in ("evidence",) if os.path.lexists(os.path.join(root, n))]

def mk(spec, chmod0=()):
    d = tempfile.mkdtemp(prefix="evroot.")
    subprocess.run(["git", "-C", d, "init", "-q"], check=True)
    for rel, content in spec.items():
        p = os.path.join(d, rel); os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "wb") as fh: fh.write(content.encode())
    for rel in chmod0: os.chmod(os.path.join(d, rel), 0)
    return d

def scenario(name, spec, want_hit, chmod0=(), want_err=False):
    d = mk(spec, chmod0)
    try: hits, errs = scan(d)
    finally:
        for rel in chmod0: os.chmod(os.path.join(d, rel), 0o644)
        shutil.rmtree(d, ignore_errors=True)
    if want_err: (ok if errs else (lambda m: bad(m + ": no error reported")))(name)
    elif bool(hits) == want_hit and not errs: ok(name)
    else: bad("%s: wanted hit=%s, got hits=%s errors=%s" % (name, want_hit, [(h[0], h[1]) for h in hits], errs))

# ---- R4 scenarios first: they prove the instrument before it is trusted on the real tree -----------------------
scenario("E1 '$ROOT/evidence/wp09/...' in scripts/register is seen", {"scripts/register/e1.sh": 'OUT="$ROOT/evidence/wp09/probe.json"\n'}, True)
scenario("E1b '${REPO_ROOT}/evidence/' and './evidence/' are seen", {"scripts/a.sh": 'x=${REPO_ROOT}/evidence/y\n', "tools/b.sh": 'cp f ./evidence/y\n'}, True)
scenario("E2 'mkdir -p evidence/wp09' in an unlisted directory (scripts/containers) is seen", {"scripts/containers/e2.sh": "mkdir -p evidence/wp09\n"}, True)
scenario("E3 non-wp default 'evidence/ledger.jsonl' via ${X:-...} is seen", {"scripts/ledger/e3.sh": 'LEDGER="${LEDGER:-evidence/ledger.jsonl}"\n'}, True)
scenario("E3b quoted default and prose 'see evidence/hc/' are seen", {"scripts/q.sh": 'f="evidence/hc/HC-0.json"\n', "tools/p.md": "(see evidence/wp01/README.md)\n"}, True)
scenario("legal: $EV, $FEAT/evidence, specs/<feature>/evidence, tools/evidence, $OUTPUT_DIR/evidence are not matched",
         {"scripts/ok.sh": 'a=$EV/wp01/x\nb=$FEAT/evidence/wp01\nc=specs/001-x/evidence/wp01\nd=tools/evidence/verify\ne="$OUTPUT_DIR/evidence/y"\nf=${FEAT}/evidence/z\n'}, False)
scenario("outside the scope (docs/) is not scanned", {"docs/x.md": "evidence/wp01\n", "scripts/fine.sh": "true\n"}, False)
if os.geteuid() != 0: scenario("unreadable file in scope is an error, not a silent skip", {"scripts/locked.sh": "evidence/wp01\n"}, False, chmod0=["scripts/locked.sh"], want_err=True)
else: print("SKIP unreadable-file scenario: running as root")
d = mk({"scripts/x.sh": "echo\n"}); shutil.rmtree(os.path.join(d, ".git")); _, e = scan(d); shutil.rmtree(d, ignore_errors=True)
ok("a root that is not a git work tree is refused as blind") if e and "not a git work tree" in e[0] else bad("non-git root not refused: %s" % e)
# R1 scenarios
for nm, mkr in (("directory", lambda d: os.mkdir(os.path.join(d, "evidence"))), ("regular file (E4)", lambda d: open(os.path.join(d, "evidence"), "w").close()), ("symlink", lambda d: os.symlink("/tmp", os.path.join(d, "evidence")))):
    d = tempfile.mkdtemp(prefix="evroot.")
    try:
        mkr(d); ok("R1 a repo-root evidence %s is seen" % nm) if r1(d) else bad("R1 a repo-root evidence %s is NOT seen" % nm)
    finally: shutil.rmtree(d, ignore_errors=True)
d = tempfile.mkdtemp(prefix="evroot."); os.makedirs(os.path.join(d, "tools/evidence")); ok("R1 tools/evidence is not a repo-root evidence path") if not r1(d) else bad("R1 matched tools/evidence"); shutil.rmtree(d, ignore_errors=True)

# ---- the real tree -----------------------------------------------------------------------------------------------
r = r1(ROOT)
ok("R1 no repo-root evidence path") if not r else bad("R1 repo-root evidence path exists: %s" % r)
hits, errs = scan(ROOT)
for e in errs: bad("R2 " + e)
per = {}
for (rel, i, txt) in hits: per.setdefault(rel, []).append("%d: %s" % (i, txt.strip()[:150]))
unlisted = []
for rel, lst in sorted(per.items()):
    want = RESIDUE.get(rel, (0, ""))[0]
    if len(lst) != want: unlisted.append((rel, want, lst))
if unlisted:
    bad("R2 evidence path outside $EV: %d file(s) differ from the residue list (new hit, or a listed hit is gone: edit the list):" % len(unlisted))
    for rel, want, lst in unlisted[:40]:
        print("     %s: %d hit(s), residue allows %d" % (rel, len(lst), want))
        for l in lst[:6]: print("         " + l)
else: ok("R2 no script or tool names a repo-root evidence path, apart from the %d listed residue lines in %d files" % (sum(v[0] for v in RESIDUE.values()), len(RESIDUE)))
stale = [p for p in RESIDUE if p not in per]
if stale: bad("R3 stale RESIDUE entries (no hit left in the file: delete the entry): %s" % stale)
else: ok("R3 every residue file still has hits (the list only shrinks)")
print("PASS" if not fails else "FAILED"); sys.exit(1 if fails else 0)
PY
