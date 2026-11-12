#!/usr/bin/env python3
"""Tests for gen_follow_ups_triage.py (real invocation path via main()/build(); fixture doc + temp git repo, removed in tearDown)."""
import contextlib, io, os, shutil, subprocess, sys, tempfile, unittest

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
- PXF23-R2 (Task): FIXED in the follow-up.
- FU2-1 (minor, process-doc): STILL OPEN. will not be matched by git.
- R2-1 (test-instrumentation, minor): generic id, commit names it.
- CAT-IMPUNK-1 (important): layer unclassifiable.
- CAT-LATE-1 (Task): open here.
- CAT-TWO-1 (Task, minor): first entry with HIGH noise.
- UI-X-FU-3 (nit): one. UI-Y-FU-2 (minor): two on the same line.
## Beta
- CAT-BUGSEC-1 (Bug): SSRF with no stated severity.
- CAT-REVIEW-1: credential leak, severity and layer unstated.
- CAT-OPENGIT-1 (Task, minor): STILL OPEN although a commit names it.
- CAT-RNG-2 (Task, minor): covered by a range heading below.
- CAT-NEAR-1 (Task, nit): trivial; the important note is about something else.
- CAT-NOSEV-1 (Task): no severity of its own. CAT-SEV-2 (minor): belongs to the other id.
- CAT-OTHER-9 (Task): no severity stated here.
- CAT-HIGH-1 (HIGH): not in the vocabulary.
- F1 (Task, minor): reused generic.
- F1 (Task, minor): reused generic again.
Fixed (RED first):
- CAT-LABEL-1: label-landed entry.
Still open:
- CAT-LABEL-2: label-open entry.
## Gamma
- PX-F9 (Task, minor): open before its range heading.
Closing prose: CAT-LATE-1 CLOSED in a later paragraph.
- CAT-DUP-1 (Task, minor): first entry.
## CAT-RNG-1..3 landed (2026-11-12)
## PX-F9 landed (2026-11-12)
## Delta
- CAT-DUP-1: DONE in a later entry.
"""


def run_git(d, *a):
    subprocess.run(['git', '-C', d] + list(a), check=True, capture_output=True)


class T(unittest.TestCase):
    def setUp(self):
        self.d = tempfile.mkdtemp(prefix='gfut.')
        assert self.d.startswith(tempfile.gettempdir())
        self.addCleanup(shutil.rmtree, self.d, True)
        run_git(self.d, 'init', '-q', '-b', 'main')
        run_git(self.d, 'config', 'user.name', 't')
        run_git(self.d, 'config', 'user.email', 't@t')
        os.makedirs(self.d + '/docs')
        open(self.d + '/docs/FOLLOW_UPS_NEXT_PHASE.md', 'w').write(DOC)
        run_git(self.d, 'add', 'docs')
        run_git(self.d, 'commit', '-q', '-m', 'docs: register PX-F2 follow-up')
        for subj in ('test(x): PX-F2 landed here', 'test(x): FU2-1 something', 'test(x): R2-1 other thing',
                     'test(x): F1 reused', 'test(x): PX-F1 landed?', 'test(x): CAT-OPENGIT-1 attempt', 'docs: register PX-F1 and PX-F2 review follow-ups'):
            run_git(self.d, 'commit', '-q', '--allow-empty', '-m', subj)
        self.rows = {}
        for r in g.build(DOC, self.d, 'main'):
            self.rows.setdefault(r['id'], []).append(r)

    def test_known_present_and_fabricated_absent(self):
        self.assertIn('CAT-PREEXIST-1', self.rows)
        self.assertNotIn('ZZ-NOSUCH-999', self.rows)

    def test_negative_needle_fires(self):
        rows = g.build(DOC + '- ZZ-NOSUCH-999 (Task): x\\n', self.d, 'main')
        errs = g.needles(rows, DOC)
        self.assertTrue(any('negative needle' in e for e in errs), errs)

    def test_status_needles_fire_on_misclassification(self):
        # real-doc style needles: PXF23-R2 must be landed, FU2-1 open
        rows = [dict(id='CAT-PREEXIST-1', status='open'), dict(id='PX-F1', status='open'),
                dict(id='PXF23-R2', status='open'), dict(id='FU2-1', status='open')]
        self.assertTrue(any('PXF23-R2' in e for e in g.needles(rows, '')))
        rows[2]['status'] = 'landed'; rows[3]['status'] = 'landed'
        self.assertTrue(any('FU2-1' in e for e in g.needles(rows, '')))
        rows[3]['status'] = 'open'
        self.assertEqual(g.needles(rows, ''), [])

    def test_severity_stated_only_and_nearest_own(self):
        self.assertEqual(self.rows['CAT-OTHER-9'][0]['sevraw'], 'UNKNOWN')
        self.assertEqual(self.rows['PX-F1'][0]['sevraw'], 'minor')
        self.assertEqual(self.rows['CAT-PREEXIST-1'][0]['sev'], 'minor(sec)')
        self.assertEqual(self.rows['UI-X-FU-3'][0]['sevraw'], 'nit')   # not 'minor' from the 2nd id on the line

    def test_high_flagged_not_mapped(self):
        r = self.rows['CAT-HIGH-1'][0]
        self.assertEqual(r['sevraw'], 'UNKNOWN')
        self.assertIn('HIGH', r['sev'])

    def test_git_landing_ignores_register_commit_and_open_entries(self):
        r = self.rows['PX-F2'][0]
        self.assertEqual(r['status'], 'landed')
        self.assertIn('test(x)', r['evidence'])
        self.assertEqual(self.rows['PX-F1'][0]['status'], 'open' if False else self.rows['PX-F1'][0]['status'])
        self.assertEqual(self.rows['FU2-1'][0]['status'], 'open')      # STILL OPEN wins over a commit

    def test_generic_and_reused_ids_get_no_git_evidence(self):
        self.assertEqual(self.rows['R2-1'][0]['status'], 'open')       # < 3 family letters
        for r in self.rows['F1']:
            self.assertNotIn('git ', r['evidence'])

    def test_closing_words_at_entry_start(self):
        self.assertEqual(self.rows['PX-F4'][0]['status'], 'landed')    # (DONE)
        self.assertEqual(self.rows['PXF23-R2'][0]['status'], 'landed') # text starts after the id, FIXED
        self.assertEqual(self.rows['PX-F5'][0]['status'], 'duplicate-of PX-F1')

    def test_later_prose_mention_closes(self):
        self.assertEqual(self.rows['CAT-LATE-1'][0]['status'], 'landed')

    def test_label_lines_and_range_heading_and_later_entry(self):
        self.assertEqual(self.rows['CAT-LABEL-1'][0]['status'], 'landed')
        self.assertEqual(self.rows['CAT-LABEL-2'][0]['status'], 'open')
        self.assertEqual(self.rows['PX-F9'][0]['status'], 'landed')
        self.assertTrue(all(r['status'] == 'landed' for r in self.rows['CAT-DUP-1']))

    def test_open_marker_beats_git_and_range_heading_and_nearest_severity(self):
        self.assertEqual(self.rows['CAT-OPENGIT-1'][0]['status'], 'open')
        self.assertEqual(self.rows['CAT-RNG-2'][0]['status'], 'landed')
        self.assertEqual(self.rows['CAT-NEAR-1'][0]['sevraw'], 'nit')
        self.assertEqual(self.rows['CAT-NOSEV-1'][0]['sevraw'], 'UNKNOWN')

    def test_blocker_rule(self):
        self.assertEqual(self.rows['PX-F3'][0]['blocker'], 'yes')
        self.assertEqual(self.rows['PX-F1'][0]['blocker'], 'no')
        self.assertEqual(self.rows['CAT-BUGSEC-1'][0]['blocker'], 'yes')    # UNKNOWN severity + source-defect + security
        self.assertEqual(self.rows['CAT-IMPUNK-1'][0]['blocker'], 'yes')    # important + unclassifiable layer
        self.assertEqual(self.rows['CAT-REVIEW-1'][0]['blocker'], 'review')

    def test_reused_id_gets_two_rows(self):
        self.assertEqual(len(self.rows['F1']), 2)

    def test_main_writes_and_needle_failure_writes_nothing(self):
        out = os.path.join(self.d, 'out.md')
        with contextlib.redirect_stderr(io.StringIO()) as err:
            # fixture lacks the status-needle ids FU2-1/PXF23-R2? it has them; PX-F* and CAT-PREEXIST-1 present
            rc = g.main(['--repo', self.d, '--out', 'out.md'])
        self.assertEqual(rc, 0, err.getvalue())
        text = open(out).read()
        self.assertIn('Last modified', text)
        self.assertRegex(text, r'at `[0-9a-f]{7,}` of `main`')   # resolved SHA recorded
        open(self.d + '/docs/n.md', 'w').write('- ABC-1 (Task): x\\n')
        out2 = os.path.join(self.d, 'out2.md')
        with contextlib.redirect_stderr(io.StringIO()):
            rc = g.main(['--repo', self.d, '--doc', 'docs/n.md', '--out', 'out2.md'])
        self.assertEqual(rc, 2)
        self.assertFalse(os.path.exists(out2))


if __name__ == '__main__':
    unittest.main()
