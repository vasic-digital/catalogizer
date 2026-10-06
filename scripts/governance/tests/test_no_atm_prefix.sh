#!/usr/bin/env bash
# test_no_atm_prefix.sh (WP-01, owner decision ODG-11 2026-10-05): this project's work-item id prefix is CAT;
#   the prefix ATM must not appear in this repository's own files (generic example id: XYZ-NNN).
#   Revision 3 (round 4 after the xhigh review WF3 governance: I1 no concrete CAT id in docs/04, I4 kind constitution-quotation).
# Usage:  bash scripts/governance/tests/test_no_atm_prefix.sh
# Env:    ATM_SCAN_ROOT  repository root to scan (default: the repo root of this script). The control needles and
#                        the mutation scenarios build scratch git work trees; they never touch the real tree.
# Exit:   0 no ATM id in scope, every control needle and scenario behaved, the exemption inventory matches; 1 otherwise.
# Scope:  EVERY file of `git ls-files -co --exclude-standard` of ATM_SCAN_ROOT (tracked or untracked-not-ignored; a
#         submodule is one gitlink and is not entered; the repo root must be a git work tree, else the test FAILs as blind).
#         Text files (no NUL byte): pattern \bATM\b | \bATM- | \bATM[0-9_] (case-sensitive; 'ATMOSphere', a device
#         name, and the lowercase identifiers 'atm_id', 'new_atm_id', 'head_atm_id' are NOT matched; the owner ANSWERED yes on 2026-10-05 (batch 3) to renaming them to cat_id in this project's register design; that rename is OWED, not done, tracked in decisions/owner-request-list.md section 0 items 7 and 15 and $EV/wp01/round4-notes.md; this test does not match the lowercase forms until the rename lands and the engine-compatibility choice (option B views, docs/upstream/constitution-prefix-change-request.md) is accepted by the owner). Binary files (a NUL byte, e.g. SQLite,
#         PNG): pattern ATM-[0-9]{3} on the raw bytes (a bare 'ATM' is a random byte run in a PNG). A regular file that
#         cannot be read is a FAILURE (no silent skip). Paths are handled as bytes, so '|', '&' and spaces are safe.
# EXEMPTIONS (documented; nothing else is exempt; each exempt hit line is counted per file and the counts are asserted):
#   E1 .specify/memory/constitution.md and constitution-appendix.md (read-only here; the owner said their ATM text needs a
#      separate approved upstream change; the project deviates from 11.4.54 until then, see risks_recorded).
#   E2 $FEAT/evidence/ : captured command output (the before-inventory, RED output and mutation logs quote the prefix).
#   E3 this test file.
#   E4 FRAGMENTS: a line is exempt when, after deleting every FRAG below from it, no match remains. Each FRAG is an exact
#      quotation of the owner's relayed ODG-11 answer or of the original ODG-11 question and recommendation. A new
#      ATM mention in a decision file is therefore a hit (review N2).
#   E5 MARKERS in the file itself, with a closed kind set {captured-output, decision-history, constitution-quotation}:
#        line marker   <!-- ATM-ALLOW:<kind> --> (or '# ATM-ALLOW:<kind>')  exempts the line that carries it
#        region        <!-- ATM-ALLOW-BEGIN:<kind> --> ... <!-- ATM-ALLOW-END -->   (at most 150 lines, no nesting, must close)
#      A marker is a comment (an HTML comment opener or '#' directly before the keyword); prose that quotes a keyword is not one.
#      captured-output = quoted output of a run that really printed the old prefix (byte-true, never edited, B1); a
#      captured-output exempt line must not contain 'CAT-' (that run could not have printed it).
#      constitution-quotation = a quotation of the constitution's own ATM text, legal ONLY in a *.md file under
#      <feature>/docs/upstream/ (the upstream change request must quote what it asks to change); anywhere else it is a FAILURE.
#      Marker lines are exempt; an unknown kind, an unclosed or nested region, or a region over 150 lines is a FAILURE.
#   E7 (real repository only) docs/04 holds NO concrete CAT-NNN id (three digits): every concrete id in that document would
#      be the output of a run, and no run printed CAT (review WF3 I1: prose that reports an executed result is evidence).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export REAL_ROOT="$(cd "$HERE/../../.." && pwd)"
export SCAN_ROOT="${ATM_SCAN_ROOT:-$REAL_ROOT}"
export SELF_REL="scripts/governance/tests/test_no_atm_prefix.sh"
python3 - <<'PY'
import os, re, subprocess, sys, tempfile, shutil, stat

ROOT = os.environ["SCAN_ROOT"]; REAL_ROOT = os.environ["REAL_ROOT"]; SELF_REL = os.environ["SELF_REL"]
FEATREL = "specs/001-full-project-audit-remediation"
RX_TEXT = re.compile(rb"\bATM\b|\bATM-|\bATM[0-9_]")
RX_BIN = re.compile(rb"ATM-[0-9]{3}")
KINDS = {b"captured-output", b"decision-history", b"constitution-quotation"}
QUOTE_OK = lambda rel: rel.startswith(FEATREL + "/docs/upstream/") and rel.endswith(".md")
EXEMPT_FILES = {".specify/memory/constitution.md", ".specify/memory/constitution-appendix.md", SELF_REL}
EXEMPT_PREFIX = (FEATREL + "/evidence/",)
# Exact quotations (E4). Backquote forms occur in the markdown request list and the docs/21 and research.md tables.
FRAGS = [
    "id prefix CAT (never ATM; remove ATM from this repo's files)",
    "submodule ATM text needs separate approved upstream change",
    "submodule ATM text needs a separate approved upstream change",
    "id prefix `CAT` (never `ATM`; remove `ATM` from this repository's files)",
    "constitution submodule `ATM` text needs a separate approved upstream change",
    "(the earlier plan default `ATM` is withdrawn)",
    # the original ODG-11 question, recommendation and research default as recorded before the owner answered (I2)
    "`ATM` plus `docs/workable_items.db`; `docs/tracking/`; another prefix",
    "`ATM` and `docs/workable_items.db` (the doc04 POC executed there)",
    "Register id prefix `ATM` and register path",
    "ATM / other; path",
    "ATM, `docs/workable_items.db`",
    # the recorded deviation from the constitution (I1)
    "the constitution (11.4.54, 11.4.248) mandates the ATM prefix",
    "[PROTECTED-SPEC: ATM-NNN]",
]
FRAGS_B = [f.encode() for f in FRAGS]
# E6 (owner decision 2026-10-05: this project uses CAT as an owner-approved exception to the constitution's literal, recorded, with the upstream
#   change request pending). CM-ATM-TICKET-IDS-COMPLETE is the constitution's own gate name (docs/04 names it) and must stay verbatim
#   for the ledger ratchet. It is exempt ONLY as that exact quoted token (no wildcard, no other ATM literal) and ONLY in the
#   files below: the two ledger files (1 exempt line each, see EXPECT) plus the three decision-record files that already quoted it
#   as part of the recorded deviation text. In any other file the token is a hit.
TOKEN = b"CM-ATM-TICKET-IDS-COMPLETE"
TOKEN_FILES = {
    "scripts/ledger/project_gate_ledger.tsv", "scripts/ledger/project_gate_ledger_prev_names.txt",
    FEATREL + "/progress.yml", FEATREL + "/decisions/owner-decisions.yaml", FEATREL + "/docs/04-findings-register-design.md",
}
MAX_REGION = 150
fails = []
def fail(m): fails.append(m); print("FAIL " + m)
def ok(m): print("ok   " + m)

def files(root):
    top = subprocess.run(["git", "-C", root, "rev-parse", "--show-toplevel"], capture_output=True)
    if top.returncode != 0 or os.path.realpath(top.stdout.decode().strip()) != os.path.realpath(root):
        return None
    r = subprocess.run(["git", "-C", root, "ls-files", "-z", "-c", "-o", "--exclude-standard"], capture_output=True)
    if r.returncode != 0: return None
    return sorted({p for p in r.stdout.split(b"\0") if p})

def scan(root):
    """returns (violations, exempt) ; exempt: {relpath: [(lineno, kind, text)]}"""
    viol, exempt = [], {}
    fl = files(root)
    if fl is None:
        return ["%s is not a git work tree root: the scope cannot be enumerated (instrument blind)" % root], exempt
    if not fl: return ["git ls-files listed 0 files (instrument blind)"], exempt
    for pb in fl:
        rel = pb.decode("utf-8", "replace")
        full = os.path.join(root.encode(), pb)
        if rel in EXEMPT_FILES or any(rel.startswith(x) for x in EXEMPT_PREFIX) or rel.startswith("submodules/"): continue
        try: st = os.lstat(full)
        except OSError as e: viol.append("%s: UNREADABLE (%s)" % (rel, e)); continue
        if stat.S_ISLNK(st.st_mode) or stat.S_ISDIR(st.st_mode): continue   # a link is scanned at its target's own path
        try:
            with open(full, "rb") as fh: data = fh.read()
        except OSError as e:
            viol.append("%s: UNREADABLE (%s)" % (rel, e)); continue
        if b"\0" in data:
            for m in RX_BIN.finditer(data):
                viol.append("%s: binary match %r at byte %d" % (rel, m.group(0).decode(), m.start()))
            continue
        if b"ATM" not in data: continue
        lines = data.split(b"\n")
        region = None; region_start = 0
        for i, ln in enumerate(lines, 1):
            # a marker is a COMMENT: `<!--` or `#` immediately before the keyword, so prose that quotes a keyword is not one
            mb = re.search(rb"(?:<!--|#)\s*ATM-ALLOW-BEGIN:([A-Za-z-]+)", ln); me = re.search(rb"(?:<!--|#)\s*ATM-ALLOW-END", ln)
            ml = re.search(rb"(?:<!--|#)\s*ATM-ALLOW:([A-Za-z-]+)", ln)
            if mb:
                if mb.group(1) not in KINDS: viol.append("%s:%d: unknown ATM-ALLOW-BEGIN kind %r" % (rel, i, mb.group(1).decode())); continue
                if mb.group(1) == b"constitution-quotation" and not QUOTE_OK(rel): viol.append("%s:%d: kind constitution-quotation is legal only in a *.md file under %s/docs/upstream/" % (rel, i, FEATREL)); continue
                if region is not None: viol.append("%s:%d: nested ATM-ALLOW-BEGIN (opened at line %d)" % (rel, i, region_start)); continue
                region, region_start = mb.group(1), i; continue
            if me:
                if region is None: viol.append("%s:%d: ATM-ALLOW-END without BEGIN" % (rel, i))
                else:
                    if i - region_start > MAX_REGION: viol.append("%s:%d: region from line %d is %d lines (limit %d)" % (rel, i, region_start, i - region_start, MAX_REGION))
                    region = None
                continue
            if ml:
                if ml.group(1) not in KINDS: viol.append("%s:%d: unknown ATM-ALLOW kind %r" % (rel, i, ml.group(1).decode())); continue
                if ml.group(1) == b"constitution-quotation" and not QUOTE_OK(rel): viol.append("%s:%d: kind constitution-quotation is legal only in a *.md file under %s/docs/upstream/" % (rel, i, FEATREL)); continue
            if not RX_TEXT.search(ln): continue
            kind = region if region is not None else (ml.group(1) if ml else None)
            if kind is not None:
                if kind == b"captured-output" and re.search(rb"\bCAT-", ln):
                    viol.append("%s:%d: captured-output line carries CAT- (a run that printed the old prefix cannot print the new one): %s" % (rel, i, ln.decode("utf-8", "replace")[:120])); continue
                exempt.setdefault(rel, []).append((i, kind.decode(), ln.decode("utf-8", "replace")))
                continue
            rest = ln
            for f in FRAGS_B: rest = rest.replace(f, b"")
            if rel in TOKEN_FILES: rest = rest.replace(TOKEN, b"")
            if not RX_TEXT.search(rest):
                exempt.setdefault(rel, []).append((i, "fragment", ln.decode("utf-8", "replace"))); continue
            viol.append("%s:%d: %s" % (rel, i, ln.decode("utf-8", "replace")[:200]))
        if region is not None: viol.append("%s: ATM-ALLOW-BEGIN at line %d never closed" % (rel, region_start))
    return viol, exempt

def mk(files_spec, chmod0=()):
    d = tempfile.mkdtemp(prefix="atmscan.")
    subprocess.run(["git", "-C", d, "init", "-q"], check=True)
    for rel, content in files_spec.items():
        p = os.path.join(d, rel); os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "wb") as fh: fh.write(content)
    for rel in chmod0: os.chmod(os.path.join(d, rel), 0)
    return d

# ---- 1. control needles and mutation scenarios in scratch git work trees ---------------------------------------
def want_hits(name, spec, expect_paths, chmod0=(), expect_exempt=None):
    d = mk(spec, chmod0)
    try:
        v, ex = scan(d)
    finally:
        for rel in chmod0: os.chmod(os.path.join(d, rel), 0o644)
        shutil.rmtree(d, ignore_errors=True)
    got = sorted({x.split(":")[0] for x in v})
    if got != sorted(expect_paths): fail("%s: wanted violations in %s, got %s (%s)" % (name, sorted(expect_paths), got, "; ".join(x[:90] for x in v)))
    elif expect_exempt is not None and sorted(ex) != sorted(expect_exempt): fail("%s: wanted exempt files %s, got %s" % (name, sorted(expect_exempt), sorted(ex)))
    else: ok(name)

B = lambda s: s.encode()
OWN = FEATREL + "/decisions/owner-decisions.yaml"
want_hits("needle: ATM id, bare ATM, ATM_ID in specs/docs/tools", {"specs/x/n.md": B("item ATM-123 here\n"), "docs/n2.md": B("prefix ATM alone\n"), "tools/n3.sh": B("ATM_ID underscore\n")}, ["specs/x/n.md", "docs/n2.md", "tools/n3.sh"])
want_hits("N1 whole repo scope: .helix/reporting.yaml and a root file are scanned", {".helix/reporting.yaml": B("id_prefix: ATM\n"), "README.md": B("see ATM-007\n")}, [".helix/reporting.yaml", "README.md"])
want_hits("N3 binary file (SQLite-like, NUL byte) with a register id is a hit", {"docs/workable_items.db": B("SQLite format 3\0\0ATM-001\0ATM-002\0")}, ["docs/workable_items.db"])
want_hits("binary file without an id-shaped run is not a hit (PNG-like bare ATM)", {"x/img.png": B("\x89PNG\0\0ATM\0ATM_\0")}, [])
want_hits("N4 a nested directory named submodules is scanned; only the top-level submodules/ is skipped", {"docs/submodules/notes.md": B("ATM-777\n"), "submodules/top/x.md": B("ATM-888\n")}, ["docs/submodules/notes.md"])
want_hits("N6 path containing '|' and '&' and a space keeps its hit", {"docs/a|b.md": B("ATM-321\n"), "docs/c&d.md": B("ATM-322\n"), "docs/e f.md": B("ATM-323\n")}, ["docs/a|b.md", "docs/c&d.md", "docs/e f.md"])
want_hits("not matched: ATMOSphere, lowercase atm_id, ATMs", {"docs/ok.md": B("ATMOSphere device and atm_id column and new_atm_id\n")}, [])
want_hits("E1 constitution files and E3 self are exempt", {".specify/memory/constitution.md": B("ATM-001\n"), ".specify/memory/other.md": B("ATM-002\n"), SELF_REL: B("ATM-003\n")}, [".specify/memory/other.md"])
want_hits("E2 only the feature-folder evidence dir is exempt: docs/evidence is scanned", {FEATREL + "/evidence/log.txt": B("ATM-001\n"), "docs/evidence/log.txt": B("ATM-001\n")}, ["docs/evidence/log.txt"])
want_hits("E4 N2 a quoted fragment is exempt, a new ATM mention in the same file is a hit",
          {OWN: B("  text: id prefix CAT (never ATM; remove ATM from this repo's files); generic\n  note: first register item will be ATM-005\n")}, [OWN], expect_exempt=[OWN])
want_hits("E4 fragment plus a second ATM on the same line is a hit", {OWN: B("id prefix CAT (never ATM; remove ATM from this repo's files) and ATM-9\n")}, [OWN])
want_hits("E5 line marker with a known kind exempts; unknown kind is a violation",
          {"docs/a.md": B("quoted old output ATM-001 <!-- ATM-ALLOW:decision-history -->\n"), "docs/b.md": B("x ATM-001 <!-- ATM-ALLOW:whatever -->\n")}, ["docs/b.md"], expect_exempt=["docs/a.md"])
want_hits("E5 balanced region exempts its lines", {"docs/r.md": B("<!-- ATM-ALLOW-BEGIN:captured-output -->\nminted|1|ATM-001\n<!-- ATM-ALLOW-END -->\n")}, [], expect_exempt=["docs/r.md"])
want_hits("E5 unclosed region is a violation", {"docs/u.md": B("<!-- ATM-ALLOW-BEGIN:captured-output -->\nminted|1|ATM-001\n")}, ["docs/u.md"])
want_hits("E5 nested region is a violation", {"docs/n.md": B("<!-- ATM-ALLOW-BEGIN:captured-output -->\n<!-- ATM-ALLOW-BEGIN:captured-output -->\nATM-001\n<!-- ATM-ALLOW-END -->\n")}, ["docs/n.md"])
want_hits("E5 END without BEGIN is a violation", {"docs/e.md": B("<!-- ATM-ALLOW-END -->\n")}, ["docs/e.md"])
want_hits("E5 region over 150 lines is a violation", {"docs/l.md": B("<!-- ATM-ALLOW-BEGIN:captured-output -->\n" + "ATM-001\n" * 160 + "<!-- ATM-ALLOW-END -->\n")}, ["docs/l.md"])
want_hits("E5 captured-output line carrying CAT- is a violation (never printed by that run)", {"docs/c.md": B("<!-- ATM-ALLOW-BEGIN:captured-output -->\nminted|1|CAT-001 ATM-001\n<!-- ATM-ALLOW-END -->\n")}, ["docs/c.md"])
UP = FEATREL + "/docs/upstream/req.md"
want_hits("E5 kind constitution-quotation exempts inside docs/upstream/*.md", {UP: B("<!-- ATM-ALLOW-BEGIN:constitution-quotation -->\nthe constitution says [ATM-NNN]\n<!-- ATM-ALLOW-END -->\n")}, [], expect_exempt=[UP])
want_hits("E5 kind constitution-quotation is a violation outside docs/upstream (region and line marker)", {"docs/q.md": B("<!-- ATM-ALLOW-BEGIN:constitution-quotation -->\nATM-001\n<!-- ATM-ALLOW-END -->\n"), "docs/q2.md": B("ATM-002 <!-- ATM-ALLOW:constitution-quotation -->\n"), FEATREL + "/docs/upstream/x.txt": B("ATM-003 <!-- ATM-ALLOW:constitution-quotation -->\n")}, ["docs/q.md", "docs/q2.md", FEATREL + "/docs/upstream/x.txt"])
want_hits("E5 an un-marked ATM line in docs/upstream is still a hit", {UP: B("<!-- ATM-ALLOW-BEGIN:constitution-quotation -->\nATM-001\n<!-- ATM-ALLOW-END -->\nATM-002 unmarked\n")}, [UP])
LEDGER = "scripts/ledger/project_gate_ledger.tsv"; LEDGER_PREV = "scripts/ledger/project_gate_ledger_prev_names.txt"
TOK = "CM-ATM-TICKET-IDS-COMPLETE"
want_hits("E6 the exact constitution gate token is exempt in the two ledger files (1 exempt line each)",
          {LEDGER: B(TOK + "\tDEFERRED\tPENDING\n"), LEDGER_PREV: B(TOK + "\n")}, [], expect_exempt=[LEDGER, LEDGER_PREV])
want_hits("E6 a second ATM literal in a ledger file is still a hit", {LEDGER: B(TOK + "\tDEFERRED\n" + "CM-ATM-OTHER-GATE\tDEFERRED\n")}, [LEDGER])
want_hits("E6 ATM-001 on the same line as the token is still a hit", {LEDGER_PREV: B(TOK + " ATM-001\n")}, [LEDGER_PREV])
want_hits("E6 the token is not exempt in a third file", {"scripts/ledger/other_ledger.tsv": B(TOK + "\tDEFERRED\n"), "docs/x.md": B("see " + TOK + "\n")}, ["scripts/ledger/other_ledger.tsv", "docs/x.md"])
if os.geteuid() != 0:
    want_hits("unreadable regular file is a failure, not a silent skip", {"docs/locked.md": B("ATM-001\n")}, ["docs/locked.md"], chmod0=["docs/locked.md"])
else: print("SKIP unreadable-file scenario: running as root (chmod 000 does not block root)")
d = mk({"docs/x.md": B("fine\n")}); shutil.rmtree(os.path.join(d, ".git"))
v, _ = scan(d); shutil.rmtree(d, ignore_errors=True)
ok("a root that is not a git work tree is refused as blind") if v and "not a git work tree" in v[0] else fail("non-git root was not refused: %s" % v)

# ---- 2. the real scan --------------------------------------------------------------------------------------------
real, exempt = scan(ROOT)
if real:
    fail("%d ATM occurrence(s) or marker violation(s) remain in %s:" % (len(real), ROOT))
    for x in real[:60]: print("     " + x[:220])
else: ok("no ATM id in scope of %s" % ROOT)

# ---- 3. exemption inventory (real repository only): counts are asserted so growth is a visible edit ----------
EXPECT = {
    FEATREL + "/decisions/owner-decisions.yaml": 7,   # 7 since the batch3 intake: the merged ODG-11 text wraps the two quoted fragments onto two lines
    FEATREL + "/decisions/owner-request-list.md": 3,
    FEATREL + "/progress.yml": 2,
    FEATREL + "/docs/04-findings-register-design.md": 33,   # 30 + 3: 30 lines of captured/fragment/decision text and, round 4 (WF3 I1), the three prose lines that report an executed result, restored byte-true from HEAD with a captured-output marker
    FEATREL + "/docs/21-master-plan-phases-risks-and-traceability.md": 2,   # ODG-11 row as recorded + the owner-answer paragraph (I2)
    "scripts/governance/tests/test_decision_intake.sh": 1,                   # NOTE_FLAGS quotes the recorded ODG-11 note (a fragment)
    "scripts/ledger/project_gate_ledger.tsv": 1,                              # E6: the constitution gate token CM-ATM-TICKET-IDS-COMPLETE (row 5)
    "scripts/ledger/project_gate_ledger_prev_names.txt": 1,                   # E6: the same token (line 1)
    FEATREL + "/docs/upstream/constitution-prefix-change-request.md": 30,    # kind constitution-quotation (the draft quotes the constitution's text it asks to change); continuum-decision-brief.md holds none
    FEATREL + "/research.md": 2,                                              # OD-11 row as recorded + the owner-answer note (I2)
}
if os.path.realpath(ROOT) == os.path.realpath(REAL_ROOT):
    got = {k: len(v) for k, v in exempt.items()}
    for k in sorted(set(EXPECT) | set(got)):
        if EXPECT.get(k, 0) != got.get(k, 0): fail("exemption inventory: %s has %d exempt line(s), expected %d" % (k, got.get(k, 0), EXPECT.get(k, 0)))
    if not any(f.startswith("exemption inventory") for f in fails): ok("exemption inventory matches: %d file(s), %d exempt line(s)" % (len(got), sum(got.values())))
    # the upstream draft may carry ONLY the constitution-quotation kind (a region re-kinded to another kind would pass the line count)
    UPD = FEATREL + "/docs/upstream/constitution-prefix-change-request.md"
    kinds = sorted({k for (_, k, _) in exempt.get(UPD, [])})
    if kinds != ["constitution-quotation"]: fail("%s exempt kinds are %s, expected only ['constitution-quotation']" % (UPD, kinds))
    else: ok("upstream draft: every exempt line is kind constitution-quotation")
    # captured POC transcripts of docs/04 section 14 stay byte-true (B1): each anchor is the original line as it was printed
    D4 = FEATREL + "/docs/04-findings-register-design.md"
    ANCH = [
        "minted|1|ATM-001",
        "add: created ATM-001 (Task, status=Queued) in Issues",
        "rc=1;  items row still ATM-001|In testing|Issues",
        "close: moved ATM-001 Issues->Fixed",
        "accepted: github|ATM-003|SKIPPED|credentials_absent",
        "legacy status wontfix  -> ATM-005 Queued",
        "v_duplicate_item_ids=ATM-001|2|Fixed,Issues",
        "MUTATION row of ATM-012 citing ATM-011's red_run",
    ]
    cap = [t for (_, k, t) in exempt.get(D4, []) if k == "captured-output"]
    miss = [a for a in ANCH if not any(a in t for t in cap)]
    if miss: fail("docs/04 section 14: %d captured transcript line(s) missing or not inside a captured-output marker: %s" % (len(miss), miss))
    else: ok("docs/04 section 14: all %d captured transcript anchors are byte-true and marked captured-output" % len(ANCH))
    # E7 (WF3 I1): no concrete CAT-NNN id in docs/04; every concrete id there would be the output of a run and no run printed CAT
    d4 = open(os.path.join(ROOT, D4), "rb").read()
    conc = [(i, ln) for i, ln in enumerate(d4.split(b"\n"), 1) if re.search(rb"\bCAT-[0-9]{3}\b", ln)]
    if conc: fail("docs/04 carries %d line(s) with a concrete CAT-NNN id (no run printed one; prose reporting an executed result is evidence): lines %s" % (len(conc), [i for i, _ in conc]))
    else: ok("docs/04 holds no concrete CAT-NNN id (E7)")
    # control needle for E7 on the same path
    if not re.search(rb"\bCAT-[0-9]{3}\b", b"x `CAT-050` y"): fail("E7 control needle not seen (instrument blind)")
else: print("note: exemption inventory and docs/04 anchors are checked for the real repository only")
print("failures=%d" % len(fails)); sys.exit(1 if fails else 0)
PY
