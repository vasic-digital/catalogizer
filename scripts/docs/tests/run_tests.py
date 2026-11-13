#!/usr/bin/env python3
"""run_tests.py - stdlib-only runner for scripts/docs/tests/test_*.py (no pytest needed).

Purpose  Calls every test_* function of each test file; a test that takes a parameter gets a fresh pathlib temp directory (the pytest tmp_path convention), prints PASS/FAIL per test and
         a summary line. Usage: run_tests.py [-k SUBSTRING] [FILE ...]  (default: every test_*.py next to this runner). Exit 0 all pass, 1 any fail, 2 usage.
         Environment DOCS_SUT_DIR points the tests at another copy of scripts/docs (used by the mutation runs).
"""
import glob, importlib.util, inspect, os, pathlib, shutil, sys, tempfile, traceback


def main(argv):
    sub = None; files = []; i = 0
    while i < len(argv):
        if argv[i] == '-k' and i + 1 < len(argv): sub = argv[i + 1]; i += 2
        elif argv[i].startswith('-'): print(__doc__, file=sys.stderr); return 2
        else: files.append(argv[i]); i += 1
    files = files or sorted(glob.glob(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'test_*.py')))
    ok = bad = 0
    for f in files:
        spec = importlib.util.spec_from_file_location(os.path.basename(f)[:-3], f); m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
        for n in sorted(dir(m)):
            if not n.startswith('test_') or (sub and sub not in n): continue
            d = pathlib.Path(tempfile.mkdtemp(prefix='docs-test-'))
            try:
                fn = getattr(m, n); fn(d) if inspect.signature(fn).parameters else fn(); ok += 1; print('PASS', n)
            except Exception:
                bad += 1; print('FAIL', n); traceback.print_exc()
            finally:
                shutil.rmtree(d, ignore_errors=True)
    print('%d passed, %d failed' % (ok, bad))
    return 1 if bad else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
