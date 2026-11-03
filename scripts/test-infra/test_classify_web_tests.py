#!/usr/bin/env python3
"""Tests for classify_web_tests.py: golden-good/golden-bad fixtures, control needles,
and a cross-check on the real tree with an independent test counter (11.4.201(7)(b), 11.4.273)."""
import json, os, re, subprocess, sys, unittest, pathlib
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
    def test_real_tree_known_files_and_independent_count(self):
        out = ROOT / "docs/qa/e2e-triage-20261103/_selftest.json"
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
        out.unlink()

if __name__ == "__main__":
    unittest.main(verbosity=2)
