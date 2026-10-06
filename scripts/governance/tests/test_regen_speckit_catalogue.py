#!/usr/bin/env python3
"""test_regen_speckit_catalogue.py - T075 (WP-G1), written BEFORE scripts/governance/regen_speckit_catalogue.py (T076).

What it proves (each case is an executing test through the script's real command line, never a parse check):
  - `write` regenerates the Anchor Catalogue and the pin lines of a fixture Spec Kit constitution from a fixture
    constitution_index.yaml and reproduces the GOLDEN files byte for byte (the goldens are hand-written, they are the oracle);
  - `write` is idempotent;
  - `check` exits 0 on in-sync files and 1 (DRIFT, naming the file) on a hand-edited catalogue, a hand-edited pin line and a
    hand-edited appendix; it exits 2 (touching nothing) on a bad index or a missing marker;
  - governance-carrier hygiene (T040b): a fixture appendix heading that ends in trailing whitespace is written without it, a body
    line ending in a tab likewise, and the appendix ends in exactly one newline (whether the input had none or several);
  - an index that lags the canon (index source_sha256 differs from the canon file hash) is stated as lagging in both files, with the
    anchors present in the canon but absent from the index named.

Run (host-neutral, the project runs it through the image mapped to python3):
  scripts/containers/run_pinned.sh IMG-TESTUTIL -- python3 scripts/governance/tests/test_regen_speckit_catalogue.py
Argument: --script <path> (or the environment variable REGEN_SCRIPT) runs the same cases against another copy of the script (used by the paired mutation, T076).
Exit: 0 all cases passed, 1 at least one failed.
"""
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
FIX = os.path.join(HERE, "fixtures", "speckit_catalogue")
SCRIPT = os.environ.get("REGEN_SCRIPT") or os.path.join(os.path.dirname(HERE), "regen_speckit_catalogue.py")
if len(sys.argv) == 3 and sys.argv[1] == "--script":  # same as REGEN_SCRIPT (RUNP does not forward the environment)
    SCRIPT = sys.argv[2]
COMMIT = "1111111111111111111111111111111111111111"

results = []


def read(p):
    with open(p, "rb") as f:
        return f.read()


def put(p, data):
    with open(p, "wb") as f:
        f.write(data)


def run(mode, work, canon="canon.md", index="index.yaml", extra=()):
    cmd = [sys.executable, "-I", SCRIPT, mode,
           "--index", os.path.join(work, index),
           "--canon-md", os.path.join(work, canon),
           "--commit", COMMIT,
           "--constitution-file", os.path.join(work, "constitution.md"),
           "--appendix-file", os.path.join(work, "appendix.md")] + list(extra)
    return subprocess.run(cmd, capture_output=True, text=True, timeout=120)


def fresh(tmp, name, constitution_in="constitution.in.md", appendix_in="appendix.in.md"):
    work = os.path.join(tmp, name)
    os.makedirs(work)
    for f in ("index.yaml", "canon.md", "canon_lag.md"):
        shutil.copy(os.path.join(FIX, f), os.path.join(work, f))
    shutil.copy(os.path.join(FIX, constitution_in), os.path.join(work, "constitution.md"))
    shutil.copy(os.path.join(FIX, appendix_in), os.path.join(work, "appendix.md"))
    return work


def case(name, ok, detail=""):
    results.append((name, ok))
    print(("PASS " if ok else "FAIL ") + name + ("" if ok else "  :: " + detail))


def main():
    if not os.path.isfile(SCRIPT):
        # RED state before T076: report every case as failing for the one real reason, never as a parse-time crash.
        print("script absent: " + SCRIPT)
        for n in ("write-reproduces-golden-constitution", "write-reproduces-golden-appendix", "write-idempotent",
                  "check-in-sync-exits-0", "check-fresh-input-reports-drift", "hand-edited-catalogue-drift",
                  "hand-edited-pin-line-drift", "hand-edited-appendix-drift", "write-repairs-hand-edit",
                  "lagging-index-golden-constitution", "lagging-index-golden-appendix",
                  "trailing-whitespace-and-single-final-newline", "no-final-newline-gets-one",
                  "bad-index-exit-2-untouched", "missing-pin-marker-exit-2-untouched",
                  "missing-catalogue-marker-exit-2-untouched"):
            case(n, False, "regen_speckit_catalogue.py does not exist")
        return finish()

    tmp = tempfile.mkdtemp(prefix="regen-test-")
    try:
        gold_c, gold_a = read(os.path.join(FIX, "constitution.golden.md")), read(os.path.join(FIX, "appendix.golden.md"))

        # 1-3: write reproduces the goldens and is idempotent
        w = fresh(tmp, "a")
        r = run("write", w)
        case("write-reproduces-golden-constitution", r.returncode == 0 and read(os.path.join(w, "constitution.md")) == gold_c,
             "rc=%s stderr=%s" % (r.returncode, r.stderr[:300]))
        case("write-reproduces-golden-appendix", read(os.path.join(w, "appendix.md")) == gold_a, "appendix differs from golden")
        before = (read(os.path.join(w, "constitution.md")), read(os.path.join(w, "appendix.md")))
        r2 = run("write", w)
        case("write-idempotent", r2.returncode == 0 and before == (read(os.path.join(w, "constitution.md")), read(os.path.join(w, "appendix.md"))),
             "second write changed a file or failed rc=%s" % r2.returncode)

        # 4: check on the in-sync goldens
        r = run("check", w)
        case("check-in-sync-exits-0", r.returncode == 0, "rc=%s out=%s" % (r.returncode, (r.stdout + r.stderr)[:300]))

        # 5: check on the stale fresh input must report drift, naming both files
        w2 = fresh(tmp, "b")
        r = run("check", w2)
        out = r.stdout + r.stderr
        case("check-fresh-input-reports-drift", r.returncode == 1 and "DRIFT" in out and "constitution.md" in out and "appendix.md" in out,
             "rc=%s out=%s" % (r.returncode, out[:300]))
        case("check-does-not-write", read(os.path.join(w2, "constitution.md")) == read(os.path.join(FIX, "constitution.in.md")),
             "check modified the file")

        # 6: hand-edited catalogue line in an otherwise in-sync file
        w3 = fresh(tmp, "c")
        run("write", w3)
        txt = read(os.path.join(w3, "constitution.md")).decode("utf-8")
        edited = txt.replace("Per-environment-topology test dispatch", "Per-environment-topology test dispatch (hand edit)")
        edited = edited if edited != txt else txt + 'x'
        put(os.path.join(w3, "constitution.md"), edited.encode("utf-8"))
        r = run("check", w3)
        case("hand-edited-catalogue-drift", r.returncode == 1 and "DRIFT" in (r.stdout + r.stderr) and "constitution.md" in (r.stdout + r.stderr),
             "rc=%s out=%s" % (r.returncode, (r.stdout + r.stderr)[:300]))

        # 7: hand-edited pin line
        w4 = fresh(tmp, "d")
        run("write", w4)
        txt = read(os.path.join(w4, "constitution.md")).decode("utf-8")
        edited = txt.replace(COMMIT, "2" * 40, 1)
        put(os.path.join(w4, "constitution.md"), edited.encode("utf-8"))
        r = run("check", w4)
        case("hand-edited-pin-line-drift", r.returncode == 1 and "DRIFT" in (r.stdout + r.stderr), "rc=%s" % r.returncode)

        # 8: hand-edited appendix (a stray trailing space)
        w5 = fresh(tmp, "e")
        run("write", w5)
        a = read(os.path.join(w5, "appendix.md"))
        put(os.path.join(w5, "appendix.md"), a.replace(b"Last line of text\n", b"Last line of text \n"))
        r = run("check", w5)
        case("hand-edited-appendix-drift", r.returncode == 1 and "appendix.md" in (r.stdout + r.stderr), "rc=%s" % r.returncode)

        # 9: write repairs the hand edit of case 6 and check then passes
        r = run("write", w3)
        r2 = run("check", w3)
        case("write-repairs-hand-edit", r.returncode == 0 and r2.returncode == 0 and read(os.path.join(w3, "constitution.md")) == gold_c,
             "write rc=%s check rc=%s" % (r.returncode, r2.returncode))

        # 10-11: lagging index
        w6 = fresh(tmp, "f")
        r = run("write", w6, canon="canon_lag.md")
        case("lagging-index-golden-constitution",
             r.returncode == 0 and read(os.path.join(w6, "constitution.md")) == read(os.path.join(FIX, "constitution.lag.golden.md")),
             "rc=%s stderr=%s" % (r.returncode, r.stderr[:300]))
        case("lagging-index-golden-appendix", read(os.path.join(w6, "appendix.md")) == read(os.path.join(FIX, "appendix.lag.golden.md")),
             "appendix differs from lag golden")

        # 10b: diagnostics on stdout (WF8 F4: the reviewer's mutants R5 INDEX-LAG banner silenced, R1 NOTE lines silenced survived)
        so = r.stdout
        case("lagging-index-stdout-banner", "INDEX-LAG" in so and "11.4.99" in so and so.splitlines()[0].startswith("INDEX-LAG"),
             "stdout=%r" % so[:300])
        wn = fresh(tmp, "n")
        put(os.path.join(wn, "constitution.md"), read(os.path.join(wn, "constitution.md")) + b"\nStale prose that still mentions 99 anchors.\n")
        rn = run("write", wn)
        case("stale-anchor-count-prints-NOTE-line", rn.returncode == 0 and "NOTE manual:" in rn.stdout and "mentions 99 anchors" in rn.stdout,
             "stdout=%r" % rn.stdout[:300])
        rn2 = run("check", wn)
        case("NOTE-never-changes-exit-code", rn2.returncode in (0, 1) and "NOTE manual:" in rn2.stdout, "rc=%s" % rn2.returncode)

        # 10c: several canon-only ids are listed in NUMERIC order whatever their order in the canon (a string sort or a reversal is wrong)
        wm = fresh(tmp, "m")
        multi = read(os.path.join(wm, "canon_lag.md")) + b"\n### \xc2\xa711.4.101 \xe2\x80\x94 third\n\nBody.\n\n### \xc2\xa711.4.100 \xe2\x80\x94 second\n\nBody.\n"
        put(os.path.join(wm, "canon_lag.md"), multi)
        rm = run("write", wm, canon="canon_lag.md")
        want = "11.4.99, 11.4.100, 11.4.101"
        case("lag-ids-numeric-order-in-stdout-and-both-files",
             rm.returncode == 0 and want in rm.stdout and want in read(os.path.join(wm, "constitution.md")).decode("utf-8"),
             "stdout=%r" % rm.stdout[:300])

        # 12: governance-carrier hygiene, asserted on the output itself (independent of the golden)
        out = read(os.path.join(w, "appendix.md"))
        lines = out.decode("utf-8").split("\n")
        bad = [i + 1 for i, l in enumerate(lines) if l != l.rstrip()]
        case("trailing-whitespace-and-single-final-newline",
             not bad and out.endswith(b"\n") and not out.endswith(b"\n\n") and b"Heading with trailing spaces\n" in out,
             "lines with trailing whitespace: %s; endswith nl=%s" % (bad, out.endswith(b"\n")))

        # 13: input without any final newline gets exactly one
        w7 = fresh(tmp, "g")
        put(os.path.join(w7, "appendix.md"), read(os.path.join(FIX, "appendix.in.md")).rstrip(b"\n"))
        run("write", w7)
        out = read(os.path.join(w7, "appendix.md"))
        case("no-final-newline-gets-one", out.endswith(b"\n") and not out.endswith(b"\n\n") and out == gold_a, "tail=%r" % out[-30:])

        # 14: bad index (no anchors key) -> exit 2, files untouched
        w8 = fresh(tmp, "h")
        put(os.path.join(w8, "index.yaml"), b"schema_version: 1\ngroups: []\n")
        r = run("write", w8)
        case("bad-index-exit-2-untouched",
             r.returncode == 2 and read(os.path.join(w8, "constitution.md")) == read(os.path.join(FIX, "constitution.in.md"))
             and read(os.path.join(w8, "appendix.md")) == read(os.path.join(FIX, "appendix.in.md")),
             "rc=%s" % r.returncode)

        # 15-16: a missing marker in the constitution file -> exit 2, appendix untouched too (all-or-nothing)
        for label, body in (("pin", b"# no pinned-sources paragraph here\n\n## Anchor Catalogue\n\nx\n"),
                            ("catalogue", b"**Pinned sources x\n\n# no catalogue heading here\n")):
            w9 = fresh(tmp, "i" + label)
            put(os.path.join(w9, "constitution.md"), body)
            r = run("write", w9)
            case("missing-%s-marker-exit-2-untouched" % label,
                 r.returncode == 2 and read(os.path.join(w9, "constitution.md")) == body
                 and read(os.path.join(w9, "appendix.md")) == read(os.path.join(FIX, "appendix.in.md")),
                 "rc=%s" % r.returncode)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
    return finish()


def finish():
    failed = [n for n, ok in results if not ok]
    print("TOTAL %d PASS %d FAIL %d" % (len(results), len(results) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
