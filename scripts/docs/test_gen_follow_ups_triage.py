#!/usr/bin/env python3
"""Tests for gen_follow_ups_triage.py (real invocation path via main(); tiny fixture doc + temp git repo)."""
import os, subprocess, sys, tempfile, unittest, io, contextlib

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gen_follow_ups_triage as g  # noqa: E402

DOC = """# Follow-ups
## Alpha series (GO-with-follow-ups, all minor)
- CAT-PREEXIST-1 (Bug): stale docs example.
- PX-F1 (Task, minor): thing one that is still open.
- PX-F2 (Task, nit): thing two.
- PX-F3 (Task, important, source-defect): SSRF in resolver, security.
- PX-F4 (DONE): already closed.
- PX-F5 (Task): same as PX-F1 duplicate of PX-F1.
## Beta
- CAT-OTHER-9 (Task): no severity stated here.
- F1 (Task, minor): reused generic.
- F1 (Task, minor): reused generic again.
"""


def run_git(d, *a):
    subprocess.run(['git', '-C', d] + list(a), check=True, capture_output=True)


class T(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.d = tempfile.mkdtemp(prefix='gfut.')
        run_git(cls.d, 'init', '-q', '-b', 'main')
        run_git(cls.d, 'config', 'user.name', 't')
        run_git(cls.d, 'config', 'user.email', 't@t')
        os.makedirs(cls.d + '/docs')
        open(cls.d + '/docs/FOLLOW_UPS_NEXT_PHASE.md', 'w').write(DOC)
        run_git(cls.d, 'add', 'docs')
        run_git(cls.d, 'commit', '-q', '-m', 'docs: register PX-F2 follow-up')
        run_git(cls.d, 'commit', '-q', '--allow-empty', '-m', 'test(x): PX-F2 landed here')
        run_git(cls.d, 'commit', '-q', '--allow-empty', '-m', 'docs: register PX-F1 and PX-F2 review follow-ups')
        cls.rows = {}
        for r in g.build(DOC, cls.d, 'main'):
            cls.rows.setdefault(r['id'], []).append(r)

    def test_known_present_and_fabricated_absent(self):
        self.assertIn('CAT-PREEXIST-1', self.rows)
        self.assertNotIn('ZZ-NOSUCH-999', self.rows)

    def test_severity_stated_only(self):
        self.assertEqual(self.rows['CAT-OTHER-9'][0]['sevraw'], 'UNKNOWN')
        self.assertEqual(self.rows['PX-F1'][0]['sevraw'], 'minor')
        self.assertEqual(self.rows['CAT-PREEXIST-1'][0]['sev'], 'minor(sec)')

    def test_git_landing_ignores_register_commit(self):
        r = self.rows['PX-F2'][0]
        self.assertEqual(r['status'], 'landed')
        self.assertIn('test(x)', r['evidence'])
        self.assertNotIn('register', r['evidence'])
        self.assertEqual(self.rows['PX-F1'][0]['status'], 'open')

    def test_done_and_duplicate(self):
        self.assertEqual(self.rows['PX-F4'][0]['status'], 'landed')
        self.assertEqual(self.rows['PX-F5'][0]['status'], 'duplicate-of PX-F1')

    def test_blocker_rule(self):
        self.assertEqual(self.rows['PX-F3'][0]['blocker'], 'yes')
        self.assertEqual(self.rows['PX-F1'][0]['blocker'], 'no')

    def test_reused_id_gets_two_rows_no_git(self):
        self.assertEqual(len(self.rows['F1']), 2)

    def test_main_writes_and_needle_failure_writes_nothing(self):
        out = os.path.join(self.d, 'out.md')
        with contextlib.redirect_stderr(io.StringIO()) as err:
            rc = g.main(['--repo', self.d, '--out', 'out.md'])
        self.assertEqual(rc, 0, err.getvalue())  # fixture has CAT-PREEXIST-1 and PX-F*
        self.assertTrue(os.path.getsize(out) > 0)
        # mutation: doc without the needle id must fail with rc 2 and not write
        open(self.d + '/docs/n.md', 'w').write('- ABC-1 (Task): x\n')
        out2 = os.path.join(self.d, 'out2.md')
        with contextlib.redirect_stderr(io.StringIO()):
            rc = g.main(['--repo', self.d, '--doc', 'docs/n.md', '--out', 'out2.md'])
        self.assertEqual(rc, 2)
        self.assertFalse(os.path.exists(out2))


if __name__ == '__main__':
    unittest.main()
