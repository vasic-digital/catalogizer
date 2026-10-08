# WP-20 evidence: T168 importer and T169 intake-match calibration (index)

| Field | Value |
|---|---|
| Created | 2026-10-08 |
| Scope | `scripts/register/import.sh` (+ `import_tickets.py`, `category_map.yaml`) and `scripts/register/tests/{test_import_classes.py,test_intake_match.sh,intake_match_fixture.json,mutate_import.py,mutate_import.sh}`; the existing `test_import.sh` (T163) is the acceptance test |
| Identity | repo HEAD `ab5a9e39` plus the uncommitted working tree (the scripts are untracked: their hashes are in `import-SHA256SUMS-scripts.txt`, which verifies from the repository root); every run was `scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL` (lock digest `sha256:56b9be70...`, python 3.11.2, sqlite 3.40.1) with `RUNP_MEMORY=1610612736` (the host had 4.7 GiB available, so the default budget refused: ledger seq 37 is that refusal recorded as a RED with `memory_budget_unavailable`; it is NOT a valid RED and is superseded by seq 38) |
| Ledger | `../ledger.jsonl`, items `AUD-T168` and `AUD-T169`; `tools/evidence/verify`: chain OK (90 entries when this file was written) |
| Verify | `cd specs/001-full-project-audit-remediation/evidence/wp20 && sha256sum -c import-SHA256SUMS` and, from the repository root, `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp20/import-SHA256SUMS-scripts.txt` |

The runs use a scratch SUT directory `.audit/scratch/t168imp/sut` (ignored, a byte copy of the tools at the end of the pass; directory fingerprint `9cc3b7cb...` before the CUT leg, see the ledger for each run) and `sut0` (the same without the importer files: the RED target).

## RED / GREEN / MUTATION

| What | Test | RED (ledger seq) | GREEN x3 (ledger seq) | Transcripts |
|---|---|---|---|---|
| T168 acceptance legs (a)(b)(c)(d) + engine identity | `test_import.sh` | 38 (and 44 on `sut0`): `importer absent`, pass=3 fail=1 | 45, 48, 51: pass=8, fail=0 | `import-RED-t163-importer-absent.txt`, `import-GREEN-t163-run1..3.txt` |
| T168 failure classes F1-F10 + CUT (112 assertions) | `test_import_classes.py` | 43 on `sut0`: `importer present` FAIL | 46, 49, 52: pass=112, fail=0 | `import-RED-t168-classes-importer-absent.txt`, `import-GREEN-t168-classes-run1..3.txt` |
| T169 calibration | `test_intake_match.sh` | 40: `calibration fixture absent` | 47, 50, 53: pass=2 and 13 `ok` lines, the record identical in all three runs (sha256 `69780643...`) | `import-RED-t169-fixture-absent.txt`, `import-GREEN-t169-run1..3.txt`, `import-intake-match-calibration.json` |
| Mutation | `mutate_import.sh --ledger` | | N00 (unmutated) passes (seq 55); C00 (comment only) survives (seq 56); M01..M34 all KILLED (seq 57..90) | `import-mutation-run.txt`, `import-mutation-result.tsv` |
| Sample import | fixture snapshot, 5 tickets | | | `import-sample-run.txt`, `import-sample-category-raw.tsv` |

Honest notes:

* The first GREEN runs of `test_import.sh` (seq 39, 42) and of T169 (seq 41) were taken before the final fix pass and are kept in the ledger; the x3 above are the ones taken on the final tools.
* `test_import_classes.py` was written AFTER the importer (the T163 legs were the RED-first part); its RED is the absent importer, and its strength is shown by the mutation run: the first (non-ledger) mutation pass left M06 (cut inside a multi-byte character), M28 (non-canonical locator) and M33 (final validate gate) alive; the CUT leg and the non-canonical-locator leg were added and those three are killed in the recorded run.
* The ledger run of the mutation set had a first aborted attempt (only the N00 control, seq 54): evrec refused the mutation author `worker` and the runner then counted the refusal as a kill. That is fixed (a kill needs a recorded `verdict=fail`); the recorded run is the repeat.
* The T169 ledger entries name the importer SUT directory as `target_ref`; the matcher under test is the engine binary (sha256 `d789d625...`, unchanged and recorded in the calibration JSON).
* Everything ran on scratch databases and fixture snapshots. `docs/workable_items.db` was never opened.
