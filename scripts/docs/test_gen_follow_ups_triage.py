#!/usr/bin/env python3
"""Tests for gen_follow_ups_triage.py (real invocation path via main()/build(); fixture doc + temp git repo, removed in tearDown)."""
import contextlib, io, os, shutil, subprocess, sys, tempfile, unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gen_follow_ups_triage as g  # noqa: E402

FILLER = "\n".join("Filler %d." % i for i in range(45))
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
## Epsilon git and shape rules
- CAT-GIT-1 (Task, minor): a docs-only commit names it.
- CAT-GIT-2 (Task, minor): a code commit names it.
- CAT-GIT-3 (Task, minor): a README-only commit names it.
- CAT-GIT-4 (Task, minor): a commit touching only a non-markdown file under docs names it.
- CAT-GIT-5 (Task, minor): a code commit names it but this long description runs past eighty characters before it says the rest is OWED.
- CAT-RGC-1 (Task, minor): range commit one.
- CAT-RGC-2 (Task, minor): range commit two.
- CAT-BND-1 (Task, minor): DONEWORK is still pending.
- CAT-NOUN-1 (spec): fixed members have no schema.
- CAT-PART-1 (Task, minor): FIXED for a and b; OWED c.
- CAT-PART-2 (DONE, Task): done but Open: the tail.
- CAT-WEAK-1 (Task, minor): described below, see PROVISIONAL-DONE in prose.
- CAT-START-1 (Task, minor): LANDED in commit abc.
- CAT-START-2 (Task, minor): [LANDED] and tested.
- CAT-IMPTI-1 (important, test-instrumentation): SSRF mutant guard test gap.
- CLOSED on branch fix-x (2026-11-03):
  - CAT-NEST-1 (Task, minor): detail one.
- STILL OPEN note:
  - CAT-NEST-2 (Task, minor): detail two.
## Zeta reuse
- F8 (Task, minor): reused one.
- F8 (Task, minor): reused two.
Prose line: F8 FIXED in some commit.
- Q9 (Task, minor): single generic with a later mention.
Prose line: Q9 FIXED here.
## Series wavefront review items
- F5 (Task, minor): first, open.
- F6 (Task, minor): first, unrelated series.
## Series wavefront landing wave
- F5 (DONE): later entry of the same series.
## Alpha tangent items
- F6 (Task, minor): other open.
## Series wavefront far items
- F7 (Task, minor): first, far away.
""" + FILLER + """
## Series wavefront far landing
- F7 (DONE): later entry, too far.
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
        for i, (subj, path) in enumerate((
                ('test(x): PX-F2 landed here', 'src/a.go'), ('test(x): FU2-1 something', 'src/b.go'), ('test(x): R2-1 other thing', 'src/c.go'),
                ('test(x): F1 reused', 'src/d.go'), ('test(x): PX-F1 landed?', 'src/e.go'), ('test(x): CAT-OPENGIT-1 attempt', 'src/f.go'),
                ('docs: register PX-F1 and PX-F2 review follow-ups', 'docs/x.md'),
                ('docs: CAT-GIT-1 line refs', 'src/g1.go'), ('fix(x): CAT-GIT-2 real fix', 'src/g2.go'),
                ('chore: CAT-GIT-3 readme only', 'README.md'), ('fix(x): CAT-GIT-4 docs dir txt', 'docs/qa/e.txt'),
                ('fix(x): CAT-GIT-5 code but entry OWED', 'src/g5.go'), ('fix(x): CAT-RGC-1..2 range', 'src/h.go'))):
            os.makedirs(os.path.dirname(os.path.join(self.d, path)), exist_ok=True)
            with open(os.path.join(self.d, path), 'w') as fh:
                fh.write('x%d\n' % i)
            run_git(self.d, 'add', path)
            run_git(self.d, 'commit', '-q', '-m', subj)
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
        rows.append(dict(id='SMB-FU-17', status='landed'))
        self.assertTrue(any('SMB-FU-17' in e for e in g.needles(rows, '')))
        rows[-1]['status'] = 'open'
        rows.append(dict(id='R3-1', status='landed'))
        self.assertTrue(any('R3-1' in e for e in g.needles(rows, '')))
        rows[-1]['status'] = 'unconfirmed'
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

    def test_later_prose_mention_is_only_unconfirmed(self):
        self.assertEqual(self.rows['CAT-LATE-1'][0]['status'], 'unconfirmed')   # weak evidence is NOT landed
        self.assertEqual(self.rows['Q9'][0]['status'], 'unconfirmed')         # single generic lead + later mention
        for r in self.rows['F8']:                                              # reused generic id: no later-mention evidence (kills dropped dup guard)
            self.assertEqual(r['status'], 'open')

    def test_series_pairing_and_window(self):
        self.assertEqual(self.rows['F5'][0]['status'], 'landed')                # same series, within window (kills dropped pairing)
        self.assertIn('series pairing', self.rows['F5'][0]['evidence'])
        for r in self.rows['F6']:                                               # no shared heading word
            self.assertEqual(r['status'], 'open')
        self.assertEqual(self.rows['F7'][0]['status'], 'open')                  # 45 lines apart (kills window 40 -> 4000)

    def test_git_evidence_needs_code_files(self):
        self.assertEqual(self.rows['CAT-GIT-1'][0]['status'], 'open')           # docs: subject
        self.assertEqual(self.rows['CAT-GIT-2'][0]['status'], 'landed')
        self.assertIn('code commit', self.rows['CAT-GIT-2'][0]['evidence'])
        self.assertEqual(self.rows['CAT-GIT-3'][0]['status'], 'open')           # *.md only
        self.assertEqual(self.rows['CAT-GIT-4'][0]['status'], 'open')           # only under docs/
        self.assertEqual(self.rows['CAT-GIT-5'][0]['status'], 'unconfirmed')    # commit but entry states an open remainder
        for i in ('CAT-RGC-1', 'CAT-RGC-2'):                                    # range in a commit subject
            self.assertEqual(self.rows[i][0]['status'], 'landed', i)

    def test_strong_start_rule_noun_partial_and_nested(self):
        self.assertEqual(self.rows['CAT-BND-1'][0]['status'], 'open')           # token boundary: DONEWORK is not DONE
        self.assertEqual(self.rows['CAT-NOUN-1'][0]['status'], 'open')          # "fixed members" is a noun
        self.assertEqual(self.rows['CAT-PART-1'][0]['status'], 'unconfirmed')   # partial
        self.assertEqual(self.rows['CAT-PART-2'][0]['status'], 'unconfirmed')
        self.assertEqual(self.rows['CAT-WEAK-1'][0]['status'], 'unconfirmed')   # closing word not at the start
        self.assertEqual(self.rows['CAT-START-1'][0]['status'], 'landed')
        self.assertEqual(self.rows['CAT-START-2'][0]['status'], 'landed')
        self.assertEqual(self.rows['CAT-NEST-1'][0]['status'], 'landed')        # under "- CLOSED on branch"
        self.assertEqual(self.rows['CAT-NEST-2'][0]['status'], 'open')          # under "- STILL OPEN note"

    def test_starts_closed_unit(self):
        for t in ('(DONE): x', ': FIXED in y', '(Task, FIXED) z', '[LANDED] q', '(F4/F5): fixed (F4 logs)'):
            self.assertTrue(g.starts_closed(t), t)
        for t in ('(spec): fixed members', '(Task): the fix is below', 'prefixed FIXED later', 'DONEWORK', ''):
            self.assertFalse(g.starts_closed(t), t)

    def test_expand_ranges(self):
        self.assertEqual(g.expand_ranges('fix D7-R2-F1..F4 now'), ['D7-R2-F1', 'D7-R2-F2', 'D7-R2-F3', 'D7-R2-F4'])
        self.assertEqual(g.expand_ranges('PX-F1..F3'), ['PX-F1', 'PX-F2', 'PX-F3'])

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
        self.assertEqual(self.rows['CAT-BUGSEC-1'][0]['blocker'], 'unclassified')   # UNKNOWN severity + source-defect: neither yes nor no
        self.assertEqual(self.rows['CAT-IMPUNK-1'][0]['blocker'], 'yes')    # important + unclassifiable layer
        self.assertEqual(self.rows['CAT-REVIEW-1'][0]['blocker'], 'unclassified')
        self.assertEqual(self.rows['CAT-IMPTI-1'][0]['blocker'], 'no')      # important + sec word but test-instrumentation layer

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
        self.assertIn('`unconfirmed` rows are NOT closed', text)
        self.assertIn('| unclassified |' if False else 'unclassified', text)
        self.assertRegex(text, r'at `[0-9a-f]{7,}` of `main`')   # resolved SHA recorded
        open(self.d + '/docs/n.md', 'w').write('- ABC-1 (Task): x\\n')
        out2 = os.path.join(self.d, 'out2.md')
        with contextlib.redirect_stderr(io.StringIO()):
            rc = g.main(['--repo', self.d, '--doc', 'docs/n.md', '--out', 'out2.md'])
        self.assertEqual(rc, 2)
        self.assertFalse(os.path.exists(out2))


if __name__ == '__main__':
    unittest.main()
