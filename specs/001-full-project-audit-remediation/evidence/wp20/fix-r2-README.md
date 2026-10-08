# WP-20 evidence, round 2 (WF23 fix pass) - index

| Field | Value |
|---|---|
| Created | 2026-10-08 |
| Scope | the single fix pass for `WF23-REVIEW-wp20-21.md` (31 findings); WP-20 files here, WP-21 files in `../wp21/` |
| Identity | repo HEAD `f72d8757` plus the uncommitted working tree (the tools and tests below are untracked/modified: their hashes are in `SHA256SUMS-scripts.txt`, which verifies from the repository root); container runs: `scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL` (lock digest `sha256:56b9be70...`, python 3.11.2, PyYAML 6.0, sqlite3 3.40.1); host runs are labelled host-side |
| Round-1 files | `GREEN-t162-*`, `RED-t162-*`, `GREEN-t164-*`, `RED-t164-*`, `GREEN-t167-*`, `RED-t167-*`, `RED-t163-import.txt`, `enumerate-mutation*.txt`, `lead-scan-mutation.txt`, `enumeration-stats-HEAD-tree-no-submodules.json` are KEPT as history of the round-1 tools. They describe code that no longer exists (their container header line was stale, finding E5, and is superseded); `SHA256SUMS` still verifies them byte for byte |

## The convergence record and the open items

| File | What |
|---|---|
| `fix-r2-convergence-assessment.md` | 11.4.276(E): classes K1-K4, structural decisions |
| `fix-r2-class-inventories.md` | the class inventories with their independent instruments and control needles (ids, statuses, checkboxes, file types, vocabulary) |
| `fix-r2-open-requests.md` | the OD-17 question (verbatim), the requests R1-R8 to other owners, the findings NOT fixed and why |
| `lead-scan-scope.md`, `non-problem-mapping.md` | the T167 decision records (owner approval UNCONFIRMED) |

## RED / GREEN / MUTATION (ledger `../ledger.jsonl`, `tools/evidence/verify`: chain OK)

RED = the NEW test against the PRE-FIX tools (verbatim copies in `fix-r2-prefix-scripts/`, `fix-r2-prefix-scripts.sha256`); GREEN = the same command after the fix, x3, the target directory a byte copy of the fixed tools (directory fingerprint `577a5c9f40...` = the fingerprint of `scripts/register/{enumerate_sources.py,lead_scan.py,lead_scan_population.py,snapshot_manifest.py,check_freeze_snapshot.sh}` at the end of the pass, checked); RED target fingerprint `a8010cf89c...`. Transcripts `fix-r2-ledger-<suite>-<polarity><n>.txt` hold the stored stdout of each entry with the ledger header.

| Item | Test | RED (seq) | GREEN x3 (seq) | MUTATION (seq) |
|---|---|---|---|---|
| AUD-T165 | `test_enumerate_absolute.py` | 5: fail=5 (S-01 strict count, S-03 glyphs, S-21 statuses and sub-items, A3, ... then a traceback at the absent XID class) | 9, 10, 11: 56 ok, fail=0 | 21: EM06 (status/severity swapped) fail=3 |
| AUD-T162 | `test_enumerate_planted.sh` | 6: pass=71 fail=4 (27 classes, XID/EXT, `.sha256` sibling) | 12, 13, 14: pass=76 fail=0 | 22: N26 (non-deterministic order) fail=2 |
| AUD-T164 | `test_frontmatter_yaml.py --selftest` | 7: 4 FAIL (the pre-fix reader has no strict checker and fails 5 of 10 forms: `fix-r2-red-t164-forms-old-parser.txt`) | 15, 16, 17: SELFTEST PASS (12 ok) | 23: FM02 (tolerant = doc04 naive splitter) |
| AUD-T167 | `test_lead_scan.py` | 8: fail (control needle disposition, ...) | 18, 19, 20: 41 ok, fail=0 | 24 (a crash-kill, superseded) and 33: LM05 killed by the control needle |
| AUD-T179 / T180 | `../wp21/` | 25 | 26-28, 29-31 | 32 |
| probes | real snapshot (host-side, scratch snapshot) | | | 34 enumerator, 35 lead scan, 36 `--finalize` (rows_before 0, rows_after 6786 = emitted 6786) |

## Mutation runs (full transcripts)

- `fix-r2-mutation-wf23.txt`: `mutate_wf23.py` - the 26 reviewer mutants (EM01-11, LM01-06, FM01-04, GM01-05) adapted, plus the mutants of the fixes (F3, F4, N01-N26, M07-M16): 63 + 4 + 5 mutants, controls first (every unmutated suite passes; a comment-only mutant SURVIVES; a deliberately wrong one is KILLED): 62 of 63 killed in section 1 (the survivor is the comment-only control), 4/4 snap, 3 of 5 gate killed, GM01 and GM05 survive and are documented (R7; equivalent by exit code). Host-side.
- `fix-r2-mutation-enumerate.txt` (10/10 killed), `fix-r2-mutation-lead.txt` (7/7), `fix-r2-frontmatter-snapshot-mutation.txt` (1/1): the round-1 sed mutation scripts re-aimed at the revised code. Host-side.

## Real-snapshot evidence (host-side, scratch `git archive HEAD` snapshot, 8,163 files; UNCONFIRMED for the real T161 freeze)

- `fix-r2-real-snapshot-run.txt`: `test_real_snapshot.py`, 24 ok, 0 FAIL: every count equals its INDEPENDENT instrument (grep) after the instrument's control needle.
- `fix-r2-planted-real-snapshot.txt`: `test_enumerate_planted.sh` with `REAL_SNAPSHOT=`: the 3-way plant on a copy of the real sources adds exactly 3 entries; pass=78.
- `fix-r2-enumeration-stats-HEAD-archive.json`, `source-class-kinds.json` (27 classes), `fix-r2-lead-scan-run-HEAD-archive.json` (command, `emitted_rows` 6786 = `lead_scan_entry_rows` counted in the imported DB, dispositions 12 / 2,718 / 4,056, needle `docs/LANDMINES.md:L45`).
- FACT (OD-17): strict YAML refuses 399 of 1,778 front-matter blocks (`ScannerError x399`).

## UNCONFIRMED

The dispatch effort of the reviewer and of this fixer is not reported by the harness; the real T161 freeze, the real-tree numbers, the `locked.sh import-sql` journal run (R1), the owner's approval of the scope and mapping records, and whether a different serving model changes any of this are not claimed.
