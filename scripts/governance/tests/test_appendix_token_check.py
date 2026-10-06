#!/usr/bin/env python3
"""test_appendix_token_check.py - written BEFORE scripts/governance/appendix_token_check.py (WF8 finding F1/F3).

Executing tests through the tool's real command line on hand-written fixtures (the fixtures are the oracle, built here, independent of the
implementation). Control needles: every "clean" case has a paired case in which exactly one token is removed and the tool must FAIL naming it,
so a blind tool (always 0) cannot pass.
Run: scripts/containers/run_pinned.sh IMG-TESTUTIL -- python3 scripts/governance/tests/test_appendix_token_check.py [--script <path>]
Exit: 0 all cases passed, 1 at least one failed.
"""
import os, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(os.path.dirname(HERE), "appendix_token_check.py")
if len(sys.argv) == 3 and sys.argv[1] == "--script":
    SCRIPT = sys.argv[2]
results = []

CANON = """# Canon

## §11. Covenant

### §11.4.1 — First anchor
Rule text. Gates: CM-FIRST-GATE and CM-COVENANT-114-1-PROPAGATION. No escape hatch — no `--skip-first`, `--second-flag` flag.

**§11.4.2 — Second anchor in bold-block form**
Text with CM-SECOND-GATE and --bold-only-flag. Not an a--b carrier.

### §11.4.3 — Third anchor
Gate CM-THIRD-GATE.

## §12. Host
"""
APP_OK = """# Appendix

## Part 1

#### §11.4.1 — First digest
Gates: CM-FIRST-GATE, CM-COVENANT-114-1-PROPAGATION; flags `--skip-first`, `--second-flag`.

#### §11.4.2 — Second digest
CM-SECOND-GATE, --bold-only-flag.

#### §11.4.3 — Third digest
CM-THIRD-GATE.
"""


def run(tmp, canon, app, allow=None, extra=()):
    c = os.path.join(tmp, "c.md"); a = os.path.join(tmp, "a.md")
    open(c, "w", encoding="utf-8").write(canon); open(a, "w", encoding="utf-8").write(app)
    cmd = [sys.executable, "-I", SCRIPT, "--canon", c, "--appendix", a]
    if allow is not None:
        p = os.path.join(tmp, "allow.tsv"); open(p, "w", encoding="utf-8").write(allow); cmd += ["--allow", p]
    return subprocess.run(cmd + list(extra), capture_output=True, text=True, timeout=60)


def case(name, ok, detail=""):
    results.append((name, ok))
    print(("PASS " if ok else "FAIL ") + name + ("" if ok else "  :: " + detail))


with tempfile.TemporaryDirectory() as tmp:
    r = run(tmp, CANON, APP_OK)
    case("clean fixture exits 0", r.returncode == 0, r.stdout + r.stderr)
    case("clean fixture reports counted tokens", "tokens_checked=" in r.stdout and "gaps=0" in r.stdout, r.stdout)

    for tok in ("--second-flag", "CM-FIRST-GATE", "CM-COVENANT-114-1-PROPAGATION"):
        r = run(tmp, CANON, APP_OK.replace(tok, "REMOVED", 1))
        case("missing %s -> exit 1 naming anchor and token" % tok, r.returncode == 1 and "11.4.1" in r.stdout and tok in r.stdout, r.stdout + r.stderr)

    # bold-form block: its tokens belong to 11.4.2, not to the preceding ### block
    r = run(tmp, CANON, APP_OK.replace("--bold-only-flag", "REMOVED"))
    case("bold-block token missing is charged to 11.4.2, not 11.4.1", r.returncode == 1 and "11.4.2" in r.stdout and "--bold-only-flag" in r.stdout and "§11.4.1:" not in r.stdout, r.stdout)
    # a token that lives only in the NEXT block must not be demanded of the previous digest
    r = run(tmp, CANON, APP_OK.replace("--bold-only-flag", "").replace("CM-SECOND-GATE", "CM-SECOND-GATE --bold-only-flag"))
    case("moving a token between digests of correct anchor passes", r.returncode == 0, r.stdout)
    # prose lines that merely start with a citation are not block-starts (measured in the real canon: 60+ false duplicates)
    cit = CANON.replace("### §11.4.3 — Third anchor", "§11.4.1 REFINES the first anchor and cites CM-NOT-A-BLOCK-TOKEN here.\n**§11.4.2 precedence (stated) --prose-only-flag.**\n\n### §11.4.3 — Third anchor")
    r = run(tmp, cit, APP_OK)
    case("prose citation lines do not start blocks nor create duplicates", r.returncode == 1 and "CM-NOT-A-BLOCK-TOKEN" in r.stdout and "duplicate" not in (r.stdout + r.stderr).lower(), r.stdout + r.stderr)
    # a token wrapped at a hyphen across a line break is ONE token (measured in the real canon: CM-AF-...-\n  VIDEO-COVERAGE, --no-issue-from-\nfirebase)
    wrapped = CANON.replace("Gate CM-THIRD-GATE.", "Gate CM-THIRD-\n  GATE and --wrap-flag-\nname.")
    r = run(tmp, wrapped, APP_OK.replace("CM-THIRD-GATE.", "CM-THIRD-GATE and --wrap-flag-name."))
    case("hyphen-wrapped tokens are joined (no truncated-prefix false gap)", r.returncode == 0, r.stdout)
    r = run(tmp, wrapped, APP_OK)
    case("hyphen-wrapped token missing from digest is still a gap, under its FULL name", r.returncode == 1 and "--wrap-flag-name" in r.stdout, r.stdout)
    # a lettered sub-clause digest heading `#### §<id>(X)` continues its parent digest (canon keeps the clause inside the parent block)
    sub = APP_OK.replace(", `--second-flag`.", ".").replace("#### §11.4.2 — Second digest", "#### §11.4.1(I) — sub-clause digest\nflags `--second-flag`.\n\n#### §11.4.2 — Second digest")
    r = run(tmp, CANON, sub)
    case("sub-clause heading continues the parent digest", r.returncode == 0, r.stdout)
    # a--b must not be a flag token
    case("a--b carrier is not a token", "a--b" not in run(tmp, CANON, APP_OK).stdout)

    # canon anchor with no digest
    r = run(tmp, CANON, APP_OK.replace("#### §11.4.3 — Third digest\nCM-THIRD-GATE.\n", ""))
    case("canon anchor without digest -> exit 1 (no-digest)", r.returncode == 1 and "11.4.3" in r.stdout and "no digest" in r.stdout, r.stdout)

    # appendix-only digest is informational, not a failure
    r = run(tmp, CANON, APP_OK + "\n#### §11.4.99 — Retired\nCM-GONE.\n")
    case("appendix-only digest is informational (exit 0, listed)", r.returncode == 0 and "11.4.99" in r.stdout, r.stdout)

    # blind: zero blocks
    r = run(tmp, "# nothing here\n", APP_OK)
    case("zero canon blocks -> BLIND exit 2", r.returncode == 2 and "BLIND" in (r.stdout + r.stderr), r.stdout + r.stderr)
    r = run(tmp, CANON, "# empty appendix\n")
    case("zero appendix digests -> BLIND exit 2", r.returncode == 2 and "BLIND" in (r.stdout + r.stderr), r.stdout + r.stderr)

    # duplicate canon block-start
    r = run(tmp, CANON + "\n### §11.4.1 — Dup\nx\n", APP_OK)
    case("duplicate canon block-start -> exit 2", r.returncode == 2 and "duplicate" in (r.stdout + r.stderr).lower(), r.stdout + r.stderr)

    # allow file
    allow = "11.4.1\t--second-flag\tcarrier placeholder named in prose only\n"
    r = run(tmp, CANON, APP_OK.replace("`--second-flag`", ""), allow)
    case("allow entry with reason exempts exactly that token", r.returncode == 0 and "exempt=1" in r.stdout, r.stdout + r.stderr)
    r = run(tmp, CANON, APP_OK.replace("`--second-flag`", "").replace("CM-FIRST-GATE", "X"), allow)
    case("allow entry does not hide a different missing token", r.returncode == 1 and "CM-FIRST-GATE" in r.stdout, r.stdout)
    r = run(tmp, CANON, APP_OK, "11.4.1\t--second-flag\t\n")
    case("allow entry without reason -> exit 2", r.returncode == 2, r.stdout + r.stderr)
    r = run(tmp, CANON, APP_OK, allow)
    case("stale allow entry (token present in digest) -> exit 1", r.returncode == 1 and "stale" in r.stdout.lower(), r.stdout)
    r = run(tmp, CANON, APP_OK, "11.4.1\t--not-in-canon\treason\n")
    case("stale allow entry (token not in canon block) -> exit 1", r.returncode == 1 and "stale" in r.stdout.lower(), r.stdout)
    r = run(tmp, CANON, APP_OK.replace("#### §11.4.3 — Third digest\nCM-THIRD-GATE.\n", ""), "11.4.3\t*\tretired anchor, no digest by design\n")
    case("allow whole-anchor no-digest exemption with reason -> exit 0", r.returncode == 0, r.stdout + r.stderr)

    # usage
    r = subprocess.run([sys.executable, "-I", SCRIPT], capture_output=True, text=True)
    case("no arguments -> exit 2 usage", r.returncode == 2)
    r = subprocess.run([sys.executable, "-I", SCRIPT, "--canon", "/nonexistent", "--appendix", "/nonexistent"], capture_output=True, text=True)
    case("unreadable input -> exit 2", r.returncode == 2)

    # json output carries identity hashes
    jp = os.path.join(tmp, "o.json")
    r = run(tmp, CANON, APP_OK, None, ["--json", jp])
    ok = r.returncode == 0 and os.path.exists(jp) and "canon_sha256" in open(jp).read()
    case("--json writes identity hashes", ok, r.stdout + r.stderr)

# ---- WF9 findings G2/G3 (written before the tool change): scope, exact counters, structure, wording floor, phrases ----
SCOPE_CANON = """# Canon

## §7. Evidence

### §7.1 NO BLUFF — positive-evidence-only validation
Gate CM-SEVEN-GATE.

## §9. Safety

### §9.2 Force-push requires explicit user authorization every time
Never use `--force-with-lease` here.

### §11.4 End-user quality guarantee — forensic anchor
Gate CM-COVENANT-PROPAGATION.

## §12. Host

### §12.6 Memory-Budget Ceiling — 60% MAXIMUM
Gate CM-TWELVE-GATE and --twelve-flag.
"""
SCOPE_APP = """# Appendix

#### §7.1 — NO BLUFF
CM-SEVEN-GATE.

#### §9.2 — Force-push
`--force-with-lease`.

#### §11.4 — End-user quality guarantee
CM-COVENANT-PROPAGATION.

#### §12.6 — Memory-Budget Ceiling
CM-TWELVE-GATE, --twelve-flag.
"""
DOT_CANON = """# Canon

### §11.4.10 — Credentials
Gate CM-TEN-GATE.

### §11.4.10.A — Pre-store audit
Gate CM-TEN-A-GATE.
"""
DOT_APP = """# Appendix

#### §11.4.10 — Credentials
CM-TEN-GATE.

#### §11.4.10.A — Pre-store audit
CM-TEN-A-GATE.
"""
FL_CANON = """# Canon

### §11.4.7 — Wording anchor
The reviewer MUST run. It MUST NOT be skipped. NEVER guess. Gate CM-WORD-GATE. Composes §11.4.5 / §11.4.6.
The rule: Fable is NOT permitted in any role.
"""
FL_APP = """# Appendix

#### §11.4.7 — Wording digest
- **Rules:** The reviewer MUST run; it MUST NOT be skipped; NEVER guess. Fable is NOT permitted in any role.
- **Gates:** CM-WORD-GATE
- **Composes:** §11.4.5, §11.4.6
"""

with tempfile.TemporaryDirectory() as tmp:
    # G2: the six anchors outside the old id scope are compared like every other one
    r = run(tmp, SCOPE_CANON, SCOPE_APP)
    case("scope: 7.1, 9.2, 11.4, 12.6 blocks all compared (clean, 4 blocks, 5 tokens)", r.returncode == 0 and "canon_blocks=4 " in r.stdout and "tokens_checked=5 " in r.stdout, r.stdout + r.stderr)
    for tok, anchor in (("CM-SEVEN-GATE", "7.1"), ("--force-with-lease", "9.2"), ("CM-COVENANT-PROPAGATION", "11.4"), ("--twelve-flag", "12.6")):
        r = run(tmp, SCOPE_CANON, SCOPE_APP.replace(tok, "REMOVED", 1))
        case("scope: %s missing from the §%s digest -> exit 1 naming it" % (tok, anchor), r.returncode == 1 and ("§" + anchor + ":") in r.stdout and tok in r.stdout, r.stdout + r.stderr)
    # a duplicate digest in the appendix is a hard error (a weakened first copy must not hide behind the real one)
    r = run(tmp, CANON, APP_OK + "\n#### §11.4.1 — Weakened duplicate\nnothing here\n")
    case("duplicate appendix digest -> exit 2", r.returncode == 2 and "duplicate appendix digest" in r.stderr, r.stdout + r.stderr)
    # exact counters (the appendix cites tokens_checked and the JSON hashes as evidence)
    r = run(tmp, CANON, APP_OK)
    case("exact counters on the clean fixture", "canon_blocks=3 appendix_digests=3 tokens_checked=7 gaps=0" in r.stdout, r.stdout)
    jp = os.path.join(tmp, "o2.json")
    import hashlib, json
    run(tmp, CANON, APP_OK, None, ["--json", jp])
    j = json.load(open(jp))
    want_c = hashlib.sha256(CANON.encode("utf-8")).hexdigest(); want_a = hashlib.sha256(APP_OK.encode("utf-8")).hexdigest()
    case("--json canon_sha256 is the canon file hash and appendix_sha256 the appendix hash", j["canon_sha256"] == want_c and j["appendix_sha256"] == want_a and want_c != want_a and j["tokens_checked"] == 7, str(j))
    # a stale whole-anchor (*) exemption (the digest exists) fails
    r = run(tmp, CANON, APP_OK, "11.4.1\t*\tclaims no digest by design\n")
    case("stale * exemption (the anchor has a digest) -> exit 1", r.returncode == 1 and "stale" in r.stdout.lower() and "11.4.1" in r.stdout, r.stdout)
    # dotted sub-ids never prefix-match their parent
    r = run(tmp, DOT_CANON, DOT_APP)
    case("dotted ids: 11.4.10 and 11.4.10.A are two blocks, two digests", r.returncode == 0 and "canon_blocks=2 appendix_digests=2 " in r.stdout, r.stdout + r.stderr)
    fp2 = os.path.join(tmp, "dotfloor.tsv")
    r = run(tmp, DOT_CANON, DOT_APP, None, ["--write-floor", fp2])
    rows = [l.split("\t")[0] for l in open(fp2, encoding="utf-8").read().splitlines() if l and not l.startswith("#")] if os.path.exists(fp2) else []
    case("--write-floor with a parent id and a dotted sub-id sorts numerically (11.4.10 before 11.4.10.A)", r.returncode == 0 and rows == ["11.4.10", "11.4.10.A"], r.stdout + r.stderr + str(rows))
    r = run(tmp, DOT_CANON, DOT_APP.replace("CM-TEN-A-GATE", "REMOVED"))
    case("dotted ids: a token missing from the .A digest is charged to 11.4.10.A, not 11.4.10", r.returncode == 1 and "§11.4.10.A:" in r.stdout and "§11.4.10:" not in r.stdout, r.stdout)
    # a canon block ends at the next ## heading: prose after it belongs to no anchor
    tail = CANON + "\n## Appendix A — Notes\nMentions CM-OUTSIDE-GATE and --outside-flag in a later section.\n"
    r = run(tmp, tail, APP_OK)
    case("text after a ## heading is not charged to the preceding anchor", r.returncode == 0, r.stdout + r.stderr)

    # G3: wording floor + required phrases (the tool proves names; this proves a digest has not lost its MUSTs, Composes entries, key sentences)
    fp = os.path.join(tmp, "floor.tsv")
    run(tmp, FL_CANON, FL_APP)
    r = subprocess.run([sys.executable, "-I", SCRIPT, "--canon", os.path.join(tmp, "c.md"), "--appendix", os.path.join(tmp, "a.md"), "--write-floor", fp], capture_output=True, text=True)
    row = open(fp, encoding="utf-8").read() if os.path.exists(fp) else ""
    case("--write-floor records MUST/NEVER/FORBIDDEN counts and Composes ids per digest", r.returncode == 0 and "11.4.7\tMUST=2,NEVER=1,FORBIDDEN=0\t11.4.5,11.4.6" in row, r.stdout + r.stderr + row)
    r = run(tmp, FL_CANON, FL_APP, None, ["--floor", fp])
    case("floor satisfied by the digest it was written from -> exit 0", r.returncode == 0 and "weakened=0" in r.stdout and "compose_lost=0" in r.stdout, r.stdout + r.stderr)
    r = run(tmp, FL_CANON, FL_APP.replace("MUST", "SHOULD"), None, ["--floor", fp])
    case("every MUST softened to SHOULD -> WEAKENED, exit 1 naming the anchor", r.returncode == 1 and "WEAKENED §11.4.7" in r.stdout and "MUST" in r.stdout, r.stdout)
    r = run(tmp, FL_CANON, FL_APP.replace("it MUST NOT be skipped", "it is not skipped"), None, ["--floor", fp])
    case("ONE MUST dropped (count below the floor) -> WEAKENED, exit 1", r.returncode == 1 and "WEAKENED §11.4.7" in r.stdout, r.stdout)
    r = run(tmp, FL_CANON, FL_APP.replace("NEVER guess", "do not guess"), None, ["--floor", fp])
    case("NEVER dropped -> WEAKENED, exit 1", r.returncode == 1 and "NEVER" in r.stdout, r.stdout)
    r = run(tmp, FL_CANON, FL_APP.replace(", §11.4.6", ""), None, ["--floor", fp])
    case("a Composes entry deleted -> COMPOSE-LOST, exit 1 naming the id", r.returncode == 1 and "COMPOSE-LOST §11.4.7" in r.stdout and "11.4.6" in r.stdout, r.stdout)
    r = run(tmp, FL_CANON, FL_APP.replace("MUST run", "MUST run MUST").replace("§11.4.6", "§11.4.6, §11.4.8"), None, ["--floor", fp])
    case("a digest that GAINS MUSTs and Composes entries is fine (a floor, not an equality)", r.returncode == 0, r.stdout)
    open(os.path.join(tmp, "floor2.tsv"), "w").write("11.4.77\tMUST=1,NEVER=0,FORBIDDEN=0\t-\n")
    r = run(tmp, FL_CANON, FL_APP, None, ["--floor", os.path.join(tmp, "floor2.tsv")])
    case("floor row for an anchor that has no digest -> STALE floor, exit 1", r.returncode == 1 and "STALE floor" in r.stdout, r.stdout)
    r = run(tmp, FL_CANON, FL_APP, None, ["--floor", os.path.join(tmp, "nonexistent.tsv")])
    case("unreadable floor file -> exit 2", r.returncode == 2, r.stdout + r.stderr)
    ph = os.path.join(tmp, "phr.tsv")
    open(ph, "w", encoding="utf-8").write("11.4.7\tFable is NOT permitted in any role\tthe canon states the Fable ban in these words\n")
    r = run(tmp, FL_CANON, FL_APP, None, ["--phrases", ph])
    case("required phrase present in digest and canon -> exit 0", r.returncode == 0 and "phrase_gaps=0" in r.stdout, r.stdout + r.stderr)
    r = run(tmp, FL_CANON, FL_APP.replace("Fable is NOT permitted in any role", "Fable as escalation tier"), None, ["--phrases", ph])
    case("required phrase replaced by its opposite -> PHRASE-LOST, exit 1", r.returncode == 1 and "PHRASE-LOST §11.4.7" in r.stdout, r.stdout)
    r = run(tmp, FL_CANON.replace("Fable is NOT permitted in any role", "Fable is allowed"), FL_APP, None, ["--phrases", ph])
    case("required phrase no longer in the canon block -> STALE phrase, exit 1", r.returncode == 1 and "STALE phrase" in r.stdout, r.stdout)
    open(ph, "w", encoding="utf-8").write("11.4.7\tFable is NOT permitted in any **role**\tmarkup in the quoted phrase is ignored\n")
    r = run(tmp, FL_CANON.replace("any role", "any `role`"), FL_APP, None, ["--phrases", ph])
    case("phrase comparison ignores markdown emphasis and code ticks on both sides", r.returncode == 0, r.stdout + r.stderr)
    open(ph, "w", encoding="utf-8").write("11.4.7\tFable is NOT permitted in any role\t\n")
    r = run(tmp, FL_CANON, FL_APP, None, ["--phrases", ph])
    case("phrase row without a reason -> exit 2", r.returncode == 2, r.stdout + r.stderr)

fails = [n for n, ok in results if not ok]
print("TOTAL %d PASS %d FAIL %d" % (len(results), len(results) - len(fails), len(fails)))
sys.exit(1 if fails else 0)
