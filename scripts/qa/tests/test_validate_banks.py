"""T209 - validator tests for scripts/qa/validate_banks.py (doc12 6.6, rules R-1..R-8).

One golden-bad fixture per rule (scripts/qa/tests/fixtures/banks/bad_rN.yaml), one golden-good, a negative control per rule family,
the --cases filter, and an independent count of the real banks (a plain regex over the bank files, not the validator's own parser).
Run through `scripts/test-in-container.sh tooling unit -- python3 -m pytest /src/scripts/qa/tests/test_validate_banks.py` (IMG-TESTUTIL).
"""
import glob
import json
import os
import re
import subprocess
import sys

import pytest

HERE = os.path.dirname(os.path.abspath(__file__))
QA = os.path.dirname(HERE)
ROOT = os.path.dirname(os.path.dirname(QA))
FIX = os.path.join(HERE, "fixtures", "banks")
VALIDATOR = os.path.join(QA, "validate_banks.py")
REAL = os.path.join(ROOT, "challenges", "helixqa-banks")


def run(*args):
    return subprocess.run([sys.executable, "-I", VALIDATOR, *args], capture_output=True, text=True)


def findings(proc):
    return json.loads(proc.stdout)["findings"]


def rules_of(proc):
    return sorted({f["rule"] for f in findings(proc)})


def test_validator_exists():
    assert os.path.isfile(VALIDATOR), "scripts/qa/validate_banks.py is absent (T210 not implemented)"


def test_good_fixture_is_clean():
    p = run("--banks", os.path.join(FIX, "good.yaml"), "--json-stdout")
    assert p.returncode == 0, p.stderr + p.stdout
    assert findings(p) == []


@pytest.mark.parametrize("rule,name", [("R-%d" % n, "bad_r%d.yaml" % n) for n in range(1, 8)])
def test_golden_bad_reports_exactly_its_rule(rule, name):
    p = run("--banks", os.path.join(FIX, name), "--json-stdout")
    assert p.returncode == 1, p.stderr + p.stdout
    assert rules_of(p) == [rule], findings(p)


def test_r8_centre_tap_without_assertion():
    p = run("--banks", os.path.join(FIX, "bad_r8.yaml"), "--json-stdout")
    assert p.returncode == 1
    assert "R-8" in rules_of(p)


def test_r8_centre_tap_with_assertion_is_still_r8_only():
    p = run("--banks", os.path.join(FIX, "bad_r8b.yaml"), "--json-stdout")
    assert p.returncode == 1
    assert rules_of(p) == ["R-8"], findings(p)


def test_r8_negative_control_good_tap_not_flagged():
    p = run("--banks", os.path.join(FIX, "good.yaml"), "--json-stdout")
    assert "R-8" not in rules_of(p)


def test_r5_negative_control_env_reference_and_negative_step_not_flagged():
    # good.yaml holds ${QA_PASS} and a deliberate wrong-password literal in a `negative: true` step
    p = run("--banks", os.path.join(FIX, "good.yaml"), "--json-stdout")
    assert "R-5" not in rules_of(p)


def test_r7_unknown_strategy(tmp_path):
    src = open(os.path.join(FIX, "good.yaml")).read().replace("strategy: specified", "strategy: vibes")
    f = tmp_path / "strategy.yaml"
    f.write_text(src)
    p = run("--banks", str(f), "--json-stdout")
    assert "R-7" in rules_of(p)


def test_directory_run_covers_all_fixtures():
    p = run("--banks", FIX, "--json-stdout")
    assert p.returncode == 1
    assert set(rules_of(p)) == {"R-%d" % n for n in range(1, 9)}


def test_cases_filter_reports_only_listed():
    lst = os.path.join(HERE, "fixtures", "cases_listed.txt")
    p = run("--banks", FIX, "--cases", lst, "--json-stdout")
    assert p.returncode == 1
    fs = findings(p)
    assert fs and {f["case"] for f in fs} == {"bad-r1"}, fs


def test_cases_filter_listed_clean_case_exits_zero(tmp_path):
    lst = tmp_path / "c.txt"
    lst.write_text("good-login\n")
    p = run("--banks", FIX, "--cases", str(lst), "--json-stdout")
    assert p.returncode == 0, p.stdout
    assert findings(p) == []


def test_cases_filter_unknown_id_refused():
    lst = os.path.join(HERE, "fixtures", "cases_unknown.txt")
    p = run("--banks", FIX, "--cases", lst, "--json-stdout")
    assert p.returncode == 2
    assert "no-such-case" in p.stderr


def test_missing_banks_refused(tmp_path):
    p = run("--banks", str(tmp_path / "nothing-here"), "--json-stdout")
    assert p.returncode == 2


def test_empty_directory_is_blind_not_clean(tmp_path):
    p = run("--banks", str(tmp_path), "--json-stdout")
    assert p.returncode == 2 and "no bank" in p.stderr


def _count(pattern, flags=0):
    n = 0
    for f in glob.glob(os.path.join(REAL, "*.yaml")):
        n += len(re.findall(pattern, open(f).read(), flags))
    return n


def test_real_banks_counts_match_independent_regex():
    """The validator over the real banks equals an independent regex count: the oracle is not the validator's own parser."""
    p = run("--banks", REAL, "--json-stdout")
    assert p.returncode == 1
    fs = findings(p)
    r8 = [f for f in fs if f["rule"] == "R-8"]
    # every coordinate tap of the real banks lacks an assertion, so R-8 equals the independent count of `tap: x,y` actions; 234 of them are 960,540
    assert len(r8) == _count(r"action:\s*['\"]?tap:\s*\d+,\d+")
    assert len([f for f in r8 if "960,540" in f["message"]]) == _count(r"action:\s*['\"]?tap:\s*960,540") == 234
    r2 = [f for f in fs if f["rule"] == "R-2"]
    # every R-2 step of the real banks is an action that starts with `#` (the "# TODO: Convert to executable" marker): independent count
    assert len(r2) == _count(r"action:\s*['\"]?#") == 1178
    assert len({(f["bank"], f["case"], f["step"]) for f in r2}) == len(r2)


def test_manifest_file_in_a_bank_directory_is_not_a_bank(tmp_path):
    (tmp_path / "MANIFEST.yaml").write_text("schema: qa-manifest/1\nbanks: []\n")
    import shutil
    shutil.copy(os.path.join(FIX, "good.yaml"), tmp_path / "good.yaml")
    p = run("--banks", str(tmp_path), "--json-stdout")
    assert p.returncode == 0, p.stdout
    assert json.loads(p.stdout)["banks"] == ["good.yaml"]
