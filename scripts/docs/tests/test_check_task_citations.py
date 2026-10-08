"""Tests of scripts/docs/check_task_citations.py (T283a). Run: python3 -m pytest -p no:cacheprovider scripts/docs/tests (IMG-DOCS).

Oracles: the exit code, the printed `FINDING <CLASS> <file>:<line>` rows and the summary line. The script under test is $TASKCITE_SUT
(default: the repository copy); the paired mutations re-run the same battery against copies of it with one `# MUT:<name>` line weakened.
"""
import os
import re
import subprocess
import sys

import pytest

HERE = os.path.dirname(os.path.abspath(__file__))
REPO_SUT = os.path.join(HERE, "..", "check_task_citations.py")
SUT = os.environ.get("TASKCITE_SUT", REPO_SUT)

TASKS = """# Tasks
- [ ] T001 [US1] Create `scripts/alpha.sh` the alpha launcher (IMG-QA)
- [ ] T002 [US1] Build the beta record `beta.json` for WP-02
- [ ] T003 [US2] Write gamma `gamma.py` and delta
- [ ] T004 [US2] Delta ratchet `delta.tsv`
- [ ] T005 [US3] Epsilon check `epsilon.sh`
- [ ] T005a [US3] Suffix task `zeta.sh`
- [ ] T006 [US3] Eta `eta.md`
"""

# file -> lines (the golden-good plan set)
DOCS = {
    "docs/one.md": [
        "# One",
        "The alpha launcher is built by T001 (see `scripts/alpha.sh`).",
        "Beta record from T002 and gamma from T003.",
        "Tasks T004 to T006 are the tail.",
        "A format example T005z never exists.",
    ],
    "README.md": ["# Readme", "Suffix T005a is `zeta.sh`."],
    "contracts/c.json": ['{', '  "description": "per T005, epsilon.sh",', '  "x": 1', '}'],
    "evidence/ev.md": ["run record: T001 T999 unbound but out of scope"],
    "audit/a.md": ["audit note: T002 T998 unbound but out of scope"],
}

# (file, line text starts with, id, kind, keyword)
GOOD_ROWS = [
    ("docs/one.md", "The alpha launcher", "T001", "citation", "scripts/alpha.sh"),
    ("docs/one.md", "Beta record", "T002", "citation", "beta.json"),
    ("docs/one.md", "Beta record", "T003", "citation", "gamma.py"),
    ("docs/one.md", "Tasks T004 to", "T004", "range_bound", ""),
    ("docs/one.md", "Tasks T004 to", "T006", "range_bound", ""),
    ("docs/one.md", "A format example", "T005z", "format_example", ""),
    ("README.md", "Suffix T005a", "T005a", "citation", "zeta.sh"),
    ("contracts/c.json", '  "description"', "T005", "citation", "epsilon.sh"),
]


def build(tmp, docs=None, rows=None, tasks=TASKS):
    docs = DOCS if docs is None else docs
    rows = GOOD_ROWS if rows is None else rows
    feat = os.path.join(str(tmp), "feat")
    os.makedirs(feat, exist_ok=True)
    with open(os.path.join(feat, "tasks.md"), "w") as fh:
        fh.write(tasks)
    for p, lines in docs.items():
        full = os.path.join(feat, p)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w") as fh:
            fh.write("\n".join(lines) + "\n")
    with open(os.path.join(feat, "task_citations.tsv"), "w") as fh:
        fh.write("# list\n")
        for (f, pre, tid, kind, kw) in rows:
            fh.write("\t".join([f, pre, tid, kind, kw]) + "\n")
    return feat


def run(feat, *extra, sut=None):
    r = subprocess.run([sys.executable, "-I", sut or SUT, "--feature-dir", feat] + list(extra), capture_output=True, text=True, timeout=120)
    finds = re.findall(r"^FINDING (\S+) (\S+):(\d+): ", r.stdout, re.M)
    return r.returncode, finds, r.stdout, r.stderr


def classes(finds):
    return sorted(c for c, _, _ in finds)


# ---------------------------------------------------------------- battery (used directly and against mutants)
def case_good(tmp):
    return build(tmp), 0, []


def case_dangling_no_row(tmp):
    d = {k: list(v) for k, v in DOCS.items()}
    d["docs/one.md"][1] += " Also T050."
    return build(tmp, d), 1, ["DANGLING"]


def case_dangling_with_row(tmp):
    """a row for a citation whose id tasks.md does not define still gives DANGLING (a row silences only UNBOUND)"""
    d = {k: list(v) for k, v in DOCS.items()}
    d["docs/one.md"][1] = "The alpha launcher is built by T001 and T050 (see `scripts/alpha.sh`)."
    rows = [r for r in GOOD_ROWS if r[2] != "T001"] + [
        ("docs/one.md", "The alpha launcher", "T001", "citation", "scripts/alpha.sh"),
        ("docs/one.md", "The alpha launcher", "T050", "citation", "anything")]
    return build(tmp, d, rows), 1, ["DANGLING"]


def case_unbound(tmp):
    return build(tmp, rows=[r for r in GOOD_ROWS if r[2] != "T002"]), 1, ["UNBOUND"]


def case_stale(tmp):
    rows = [r if r[2] != "T003" else (r[0], r[1], r[2], r[3], "delta.tsv") for r in GOOD_ROWS]
    return build(tmp, rows=rows), 1, ["STALE"]


def case_orphan_missing_prefix(tmp):
    rows = GOOD_ROWS + [("docs/one.md", "This line does not exist", "T001", "citation", "scripts/alpha.sh")]
    return build(tmp, rows=rows), 1, ["ORPHAN-ROW"]


def case_orphan_id_gone(tmp):
    rows = GOOD_ROWS + [("README.md", "Suffix T005a", "T001", "citation", "scripts/alpha.sh")]
    return build(tmp, rows=rows), 1, ["ORPHAN-ROW"]


def case_example_resolves(tmp):
    rows = [r for r in GOOD_ROWS if r[2] != "T002"] + [("docs/one.md", "Beta record", "T002", "format_example", "")]
    return build(tmp, rows=rows), 1, ["EXAMPLE-RESOLVES"]


def case_range_only_row_does_not_bind_plain(tmp):
    d = {k: list(v) for k, v in DOCS.items()}
    d["docs/one.md"][3] = "Tasks T004 to T006 are the tail; T006 itself is `eta.md`."
    return build(tmp, d), 1, ["UNBOUND"]


BATTERY = [case_good, case_dangling_no_row, case_dangling_with_row, case_unbound, case_stale, case_orphan_missing_prefix,
           case_orphan_id_gone, case_example_resolves, case_range_only_row_does_not_bind_plain]


def battery(tmp_root, sut):
    """names of the battery expectations that fail against `sut` (the script) - empty for the real script"""
    failed = []
    for n, fn in enumerate(BATTERY):
        sub = os.path.join(str(tmp_root), "b%d" % n)
        os.makedirs(sub)
        feat, rc, want = fn(sub)
        got_rc, finds, _, err = run(feat, sut=sut)
        if got_rc != rc or classes(finds) != sorted(want):
            failed.append("%s (rc %s, classes %s, want rc %s %s)" % (fn.__name__, got_rc, classes(finds), rc, sorted(want)))
    # blindness: the test flag must give 20, and a list with no citation row cannot carry the control
    sub = os.path.join(str(tmp_root), "blind1"); os.makedirs(sub)
    if run(build(sub), "--self-test-blind", sut=sut)[0] != 20:
        failed.append("self-test-blind does not exit 20")
    sub = os.path.join(str(tmp_root), "blind2"); os.makedirs(sub)
    if run(build(sub, rows=[r for r in GOOD_ROWS if r[3] != "citation"]), sut=sut)[0] != 20:
        failed.append("a list without citation rows does not exit 20")
    return failed


# ---------------------------------------------------------------- tests
def test_battery_passes_against_the_real_script(tmp_path):
    assert battery(tmp_path, SUT) == []


def test_golden_good_exit_0_and_control_reported(tmp_path):
    rc, finds, out, err = run(build(tmp_path))
    assert (rc, finds) == (0, []), out + err
    assert "0 findings in 3 plan-set files" in out
    assert "seeded control reported" in out


@pytest.mark.parametrize("fn,cls", [(case_dangling_no_row, "DANGLING"), (case_unbound, "UNBOUND"), (case_stale, "STALE"),
                                    (case_orphan_missing_prefix, "ORPHAN-ROW"), (case_example_resolves, "EXAMPLE-RESOLVES")])
def test_each_finding_class_fails_with_file_and_line(tmp_path, fn, cls):
    feat, rc, want = fn(str(tmp_path))
    got_rc, finds, out, _ = run(feat)
    assert got_rc == 1
    assert cls in classes(finds)
    for c, f, ln in finds:
        assert int(ln) >= 0 and f  # printed with file and line
        assert os.path.exists(os.path.join(feat, f)) or ln == "0"


def test_dangling_is_not_silenced_by_a_row(tmp_path):
    feat, rc, want = case_dangling_with_row(str(tmp_path))
    got_rc, finds, out, _ = run(feat)
    assert got_rc == 1 and ("DANGLING", "docs/one.md", "2") in finds, out


def test_stale_is_judged_against_the_task_text_not_silenced_by_a_row(tmp_path):
    feat, _, _ = case_stale(str(tmp_path))
    rc, finds, out, _ = run(feat)
    assert ("STALE", "docs/one.md", "3") in finds, out


def test_a_row_for_a_real_task_cannot_hide_the_wrong_task(tmp_path):
    """the renumbering case: the cited id exists but names another task; only the keyword sees it"""
    d = {k: list(v) for k, v in DOCS.items()}
    tasks = TASKS.replace("`scripts/alpha.sh` the alpha launcher", "`scripts/other.sh` something else")
    rc, finds, out, _ = run(build(tmp_path, d, tasks=tasks))
    assert rc == 1 and ("STALE", "docs/one.md", "2") in finds, out


def test_out_of_scope_folders_and_tasks_md_are_not_scanned(tmp_path):
    rc, finds, out, _ = run(build(tmp_path))
    assert not [f for f in finds if f[1].startswith(("evidence", "audit")) or f[1] == "tasks.md"]


def test_contract_json_is_in_scope(tmp_path):
    rc, finds, out, _ = run(build(tmp_path, rows=[r for r in GOOD_ROWS if r[0] != "contracts/c.json"]))
    assert ("UNBOUND", "contracts/c.json", "2") in finds, out


@pytest.mark.parametrize("text", ["T004 to T006", "T004-T006", "T004..T006", "T004 through T006", "T004 – T006"])
def test_range_forms_need_range_bound_rows_only(tmp_path, text):
    d = {k: list(v) for k, v in DOCS.items()}
    d["docs/one.md"][3] = "Tasks %s are the tail." % text
    rows = [r for r in GOOD_ROWS if not (r[3] == "range_bound")] + [
        ("docs/one.md", "Tasks %s" % text[:4], "T004", "range_bound", ""), ("docs/one.md", "Tasks %s" % text[:4], "T006", "range_bound", "")]
    rc, finds, out, _ = run(build(tmp_path, d, rows))
    assert (rc, finds) == (0, []), out
    sub = tmp_path / "x"; sub.mkdir()
    rc, finds, out, _ = run(build(sub, d, [r for r in GOOD_ROWS if r[3] != "range_bound"]))
    assert classes(finds) == ["UNBOUND", "UNBOUND"], out  # both ends are tokens and need a row


def test_range_bound_must_exist(tmp_path):
    d = {k: list(v) for k, v in DOCS.items()}
    d["docs/one.md"][3] = "Tasks T004 to T099 are the tail."
    rows = [r for r in GOOD_ROWS if r[3] != "range_bound"] + [
        ("docs/one.md", "Tasks T004", "T004", "range_bound", ""), ("docs/one.md", "Tasks T004", "T099", "range_bound", "")]
    rc, finds, out, _ = run(build(tmp_path, d, rows))
    assert classes(finds) == ["DANGLING"], out


@pytest.mark.parametrize("text,count", [
    ("see T001", 1), ("see (T001)", 1), ("see `T001`", 1), ("see T001.", 1), ("see T001a", 1), ("T001,T002", 2),
    ("xT001", 0), ("T0011", 0), ("T001x", 1), ("t001", 0), ("T01", 0), ("1T001", 0), ("T001_", 0)])
def test_token_boundaries(tmp_path, text, count):
    """a token is T + 3 digits + optional lower-case letter with no letter or digit directly around it ('_' is not a letter)"""
    d = {"docs/p.md": ["probe " + text]}
    rows = []
    rc, finds, out, _ = run(build(tmp_path, d, rows=GOOD_ROWS), "--extract")
    got = [ln for ln in out.splitlines() if ln.split("\t")[0] == "docs/p.md"]
    # T001a is not a task in the fixture (only T005a is): extract reports it as a nonexistent example, still one token
    # T001_ : the underscore is neither a letter nor a digit, so it IS a token
    expected = 1 if text == "T001_" else count
    assert len(got) == expected, out


def test_extract_proposes_keywords_and_never_writes(tmp_path):
    feat = build(tmp_path)
    before = open(os.path.join(feat, "task_citations.tsv")).read()
    rc, finds, out, _ = run(feat, "--extract")
    assert rc == 0
    row = [r.split("\t") for r in out.splitlines() if r.startswith("docs/one.md\tThe alpha")]
    assert row and row[0][2] == "T001" and row[0][3] == "citation" and row[0][4] == "scripts/alpha.sh" and row[0][5] == "auto"
    assert open(os.path.join(feat, "task_citations.tsv")).read() == before


def test_blind_when_the_control_cannot_be_reported(tmp_path):
    rc, finds, out, err = run(build(tmp_path), "--self-test-blind")
    assert rc == 20 and "BLIND" in err and "FINDING" not in out


def test_blind_when_the_list_has_no_citation_row(tmp_path):
    rc, finds, out, err = run(build(tmp_path, rows=[r for r in GOOD_ROWS if r[3] != "citation"]))
    assert rc == 20 and "BLIND" in err


def test_seeded_control_replaces_a_citation_by_the_task_two_places_before(tmp_path):
    rc, finds, out, err = run(build(tmp_path))
    # first sorted candidate row: README.md line 2, T005a (order index 5) -> index 3 = T004
    assert "README.md:2 T005a -> T004" in out, out


@pytest.mark.parametrize("args", [["--bogus"], ["--list"], ["--feature-dir"]])
def test_usage_errors_exit_2(tmp_path, args):
    r = subprocess.run([sys.executable, "-I", SUT] + args, capture_output=True, text=True, timeout=60)
    assert r.returncode == 2 and "usage" in r.stderr


def test_unreadable_inputs_exit_2(tmp_path):
    feat = build(tmp_path)
    os.remove(os.path.join(feat, "task_citations.tsv"))
    assert run(feat)[0] == 2
    feat = build(tmp_path / "b")
    open(os.path.join(feat, "docs", "bin.md"), "wb").write(b"\xff\xfe\x00bad")
    assert run(feat)[0] == 2


def test_malformed_list_exit_2(tmp_path):
    for bad in ["a\tb\tc\n", "docs/one.md\tx\tT001\tnotakind\tk\n", "docs/one.md\t\tT001\tcitation\tk\n", "docs/one.md\tx\tnot-an-id\tcitation\tk\n"]:
        feat = build(tmp_path / ("m%d" % abs(hash(bad))))
        open(os.path.join(feat, "task_citations.tsv"), "w").write(bad)
        assert run(feat)[0] == 2, bad


def test_duplicate_row_and_ambiguous_prefix_are_orphan_rows(tmp_path):
    rows = GOOD_ROWS + [GOOD_ROWS[0]]
    rc, finds, out, _ = run(build(tmp_path, rows=rows))
    assert classes(finds) == ["ORPHAN-ROW"], out
    d = {k: list(v) for k, v in DOCS.items()}
    d["docs/one.md"] = d["docs/one.md"] + ["The alpha launcher again, no id here."]
    sub = tmp_path / "amb"; sub.mkdir()
    rc, finds, out, _ = run(build(sub, d))
    assert classes(finds) == ["ORPHAN-ROW", "UNBOUND"], out  # an unresolvable row binds nothing, so its citation is UNBOUND too


def test_same_id_twice_on_a_line_needs_one_row(tmp_path):
    d = {k: list(v) for k, v in DOCS.items()}
    d["docs/one.md"][1] = "The alpha launcher is built by T001 (T001 again; see `scripts/alpha.sh`)."
    rc, finds, out, _ = run(build(tmp_path, d))
    assert (rc, finds) == (0, []), out


# ---------------------------------------------------------------- paired mutations (the same battery against weakened copies)
MUTANTS = [
    ("dangling-bound", [("dangling-bound", " and False", "")]),
    ("dangling", [("dangling", None, None)]),
    ("stale", [("stale", None, None)]),
    ("unbound", [("unbound", None, None)]),
    ("orphan", [("orphan", None, None)]),
    ("example", [("example", None, None)]),
    ("control-always-reported", [("control", "found = [f for f in engine(mod, order, texts, rows, blind) if f[0] == rfile and f[1] == i + 1]", "found = [1]")]),
    # the shape the task names: a copy that only checks that a cited id exists
    ("exists-only", [("stale", None, None), ("unbound", None, None), ("orphan", None, None), ("example", None, None)]),
]


def mutate(src, steps):
    lines = src.split("\n")
    for (name, old, new) in steps:
        hit = 0
        for i, ln in enumerate(lines):
            if ln.rstrip().endswith("# MUT:" + name):
                ind = ln[: len(ln) - len(ln.lstrip())]
                if old is None:
                    lines[i] = ind + "pass  # MUT:" + name
                else:
                    assert old in ln, (name, ln)
                    lines[i] = ln.replace(old, new)
                hit += 1
        assert hit >= 1, "mutation %s found no marker line (script defect)" % name
    return "\n".join(lines)


@pytest.mark.parametrize("name,steps", MUTANTS, ids=[m[0] for m in MUTANTS])
def test_paired_mutation_is_caught(tmp_path, name, steps):
    src = open(SUT).read()
    mut = tmp_path / "mutant.py"
    mut.write_text(mutate(src, steps))
    assert mut.read_text() != src, "mutant identical to the script"
    subprocess.run([sys.executable, "-I", "-c", "import ast,sys;ast.parse(open(sys.argv[1]).read())", str(mut)], check=True)
    (tmp_path / "bat").mkdir()
    failed = battery(tmp_path / "bat", str(mut))
    assert failed, "mutation %s SURVIVED: the battery passed against the weakened script" % name
