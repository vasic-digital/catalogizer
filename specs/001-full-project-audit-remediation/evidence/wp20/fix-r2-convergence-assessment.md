# Convergence assessment (constitution 11.4.276(E)) - WF23 review of WP-20 / WP-21, fix pass 2

| Field | Value |
|---|---|
| Created | 2026-10-08 |
| Author | the single fixer (Sonnet); the review round number of WF23 is UNKNOWN to the fixer (the review file does not state one) |
| Review | `WF23-REVIEW-wp20-21.md` (independent, Opus 5.5, verdict NO-GO WP-20 / GO-with-fixes WP-21 gate code): 31 findings, 19 `source-defect`, 5 `test-instrumentation`, 7 `process-doc` |
| This pass | ONE pass, every finding addressed except D1 (OD-17, owner), D6 (WP-21 test file out of the edit scope), the `locked.sh`/T161/docs-index sides of E4, E6, F1, F2, F6 (requests, `fix-r2-open-requests.md`) |

## Classes of defect, not instances (11.4.276(C))

The review's 31 findings are four classes. Each class was closed as a class, with an inventory produced by an instrument that found its control needle first (`fix-r2-class-inventories.md`).

| Class | Members (findings) | Root cause | Class-complete remedy |
|---|---|---|---|
| K1 oracle shaped like the implementation | A1, A3, A4, A5, C1, D2, D3, D4 | every test oracle was built from a fixture the author shaped like the code, so tool and test agreed while the real corpus did not (S-03 statuses all NULL on 269/269 real rows, 21 prose-only ids absent, 181 inflected lead lines missed, 65 checkbox lines absent) | (1) absolute assertions on the fields of every entry class; (2) `test_real_snapshot.py`: the tools run on a REAL snapshot and every count is compared with an INDEPENDENT instrument (grep -E, grep -Iw, grep -i word-boundary) after that instrument found a control needle; (3) the id rule became ONE cross-source index whose census residue is printed; the marker scan lost its extension allow-list; the lead vocabulary lists its inflections |
| K2 silent loss at a boundary | A2, A6, A7, B1, B2, C2, C4, C7, F3, F4 | a boundary dropped, merged or kept stale without a signal (external sources not enumerated, id-less bank cases, a second skip hit refusing the whole run, INSERT OR IGNORE keeping a stale row, a duplicate label with no covering row, a malformed HC-2 read as `[]`, a FIFO blocking forever) | each boundary now either carries the item (a row, a listed exclusion, a counted skip) or refuses by name; the re-import is gated (all or nothing); each has a test and a mutant |
| K3 control that cannot fail | C3, D5, E5 (header) | a needle derived from the scan's own output; a boundary check on a str that can never fail; a header line printing a stale claim | fixed needles located by plain text search, the vocabulary control run on the same functions, the cut check on stored bytes, the container line derived from the run |
| K4 evidence on the wrong target | E1, E2, E3 | ledger entries fingerprinted the TEST, not the gate; the RED was said impossible once the implementation existed (it is reproducible through the test's `GATE` hook); WP-20 had no ledger entries | `record_wp20_evidence.py`: RED on the pre-fix tool copies, GREEN x3, MUTATION, target = the tool under test (directory pair for RED/GREEN) |

Plus two single-instance items that are not a class: B3 (`--check-script` removed), B4 (`pyyaml_missing` refusal), D1 (owner decision).

## Structural decisions (replace, not layer)

- The per-source id rules of S-14 and S-15 were REPLACED by one id index; the class of an id is where its first occurrence lies.
- The lead scan's file-level duplicate test was REPLACED by a per-line cover computed from the structured sources' own rules.
- The SQL writer is ONE function (`render_block`) shared by both tools and carries the gate; INSERT OR IGNORE stays but is no longer trusted alone.

## Why this pass is expected to converge

Every defect class has (a) a test that fails on the pre-fix tool (RED recorded), (b) an independent real-corpus instrument, (c) mutants of the fix itself (63+ in `mutate_wf23.py`) that the tests kill. The honest open residue is listed in `fix-r2-open-requests.md`; none of it is a defect class left half-closed.
