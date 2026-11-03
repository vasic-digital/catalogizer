#!/usr/bin/env python3
"""Tests for classify_web_tests.py: golden-good/golden-bad fixtures, control needles,
and a cross-check on the real tree with an independent test counter (11.4.201(7)(b), 11.4.273)."""
import json, os, re, subprocess, sys, tempfile, unittest, pathlib
HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent.parent
TD = HERE / "testdata" / "classify_web"
CLS = HERE / "classify_web_tests.py"

def run(*args):
    p = subprocess.run([sys.executable, "-I", str(CLS), *args], capture_output=True, text=True)
    return p

def classify(path):
    p = run("--files", str(path), "--root", str(ROOT), "--json-only")
    assert p.returncode == 0, p.stderr
    return json.loads(p.stdout)["files"][0]

class T(unittest.TestCase):
    def test_golden_bad_mocked_flagged(self):
        f = classify(TD / "golden_mocked.spec.ts"); self.assertEqual(f["class"], "MOCKED"); self.assertEqual(f["tests_total"], 2); self.assertEqual(f["tests_mocked"], 2)
    def test_golden_good_real_passes(self):
        f = classify(TD / "golden_real.spec.ts"); self.assertEqual(f["class"], "REAL"); self.assertEqual(f["tests_real"], 2)
    def test_mixed_per_test(self):
        f = classify(TD / "golden_mixed.spec.ts"); self.assertEqual(f["class"], "MIXED"); self.assertEqual((f["tests_real"], f["tests_mocked"]), (1, 1))
    def test_negative_control_comment_and_string_carriers(self):
        f = classify(TD / "golden_carrier.spec.ts"); self.assertEqual(f["class"], "REAL")
    def test_helper_indirection(self):
        f = classify(TD / "golden_helper.spec.ts"); self.assertEqual(f["class"], "MOCKED"); self.assertEqual(f["tests_mocked"], 2)
    def test_msw_module_level_server_marks_all_tests_mocked(self):
        f = classify(TD / "golden_msw.spec.ts"); self.assertEqual((f["class"], f["tests_total"], f["tests_mocked"]), ("MOCKED", 2, 2))
    def test_init_script_fetch_replacement(self):
        f = classify(TD / "golden_init_script.spec.ts"); self.assertEqual((f["class"], f["tests_real"], f["tests_mocked"]), ("MIXED", 1, 1))
        self.assertEqual(f["tests"][0]["class"], "MOCKED"); self.assertEqual(f["tests"][1]["class"], "REAL")
    def test_fake_server(self):
        f = classify(TD / "golden_fake_server.spec.ts"); self.assertEqual((f["class"], f["tests_mocked"]), ("MOCKED", 1))
    def test_hook_scoped_to_its_describe(self):
        f = classify(TD / "golden_scoped_hook.spec.ts"); self.assertEqual((f["class"], f["tests_real"], f["tests_mocked"]), ("MIXED", 1, 1))
        self.assertEqual([t["class"] for t in f["tests"]], ["MOCKED", "REAL"])
    def test_output_root_is_not_an_absolute_host_path(self):
        p = run("--files", str(TD / "golden_real.spec.ts"), "--root", str(ROOT), "--json-only")
        self.assertEqual(json.loads(p.stdout)["root"], ".")
        self.assertNotIn(str(ROOT), p.stdout)
        committed = json.loads((ROOT / "docs/qa/e2e-triage-20261103/classification.json").read_text())
        self.assertEqual(committed["root"], ".")
    def test_notests_with_test_call_is_blind_error(self):
        p = run("--files", str(TD / "golden_blind.spec.ts"), "--root", str(ROOT), "--json-only")
        self.assertEqual(p.returncode, 2, p.stdout + p.stderr)
        self.assertEqual(json.loads(p.stdout)["files"][0]["class"], "BLIND")
    def test_notests_without_test_call_stays_notests(self):
        with tempfile.TemporaryDirectory() as d:
            h = pathlib.Path(d) / "helper.spec.ts"; h.write_text("export const x = 1; // test( mentioned only in a comment\n")
            p = run("--files", str(h), "--root", d, "--json-only"); self.assertEqual(p.returncode, 0, p.stderr)
            self.assertEqual(json.loads(p.stdout)["files"][0]["class"], "NOTESTS")
    def test_pinned_class_counts_over_goldens(self):
        goldens = sorted(str(p) for p in TD.glob("golden_*.spec.ts") if p.name != "golden_blind.spec.ts")
        p = run("--files", *goldens, "--root", str(ROOT), "--json-only"); self.assertEqual(p.returncode, 0, p.stderr)
        c = json.loads(p.stdout)["counts"]
        self.assertEqual((c["files"], c["REAL"], c["MOCKED"], c["MIXED"], c["other"]), (9, 2, 4, 3, 0), c)
        self.assertEqual((c["tests_total"], c["tests_real"], c["tests_mocked"]), (16, 6, 10), c)
    def test_real_tree_known_files_and_independent_count(self):
        tmpd = tempfile.TemporaryDirectory(); out = pathlib.Path(tmpd.name) / "_selftest.json"
        p = run("--root", str(ROOT), "--out-json", str(out), "--out-md", os.devnull)
        self.assertEqual(p.returncode, 0, p.stderr)
        d = json.loads(out.read_text()); by = {f["path"]: f for f in d["files"]}
        self.assertEqual(by["catalog-web/e2e/tests/auth.spec.ts"]["class"], "MOCKED")      # known mocked
        self.assertEqual(by["catalog-web/e2e/tests/browsing-challenge.spec.ts"]["class"], "REAL")  # known real
        self.assertEqual(by["catalog-web/e2e/real-backend-pages.spec.ts"]["class"], "MIXED")      # known hybrid
        # independent counter: line-start test( calls
        for path, f in by.items():
            if not path.endswith(".spec.ts"): continue
            txt = (ROOT / path).read_text()
            n = len(re.findall(r"^[ \t]*test(?:\.(?:only|skip|fixme|slow))?\(\s*['\"`]", txt, re.M))
            self.assertEqual(f["tests_total"], n, path)
        tmpd.cleanup()

if __name__ == "__main__":
    unittest.main(verbosity=2)
