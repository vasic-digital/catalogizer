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

fails = [n for n, ok in results if not ok]
print("TOTAL %d PASS %d FAIL %d" % (len(results), len(results) - len(fails), len(fails)))
sys.exit(1 if fails else 0)
