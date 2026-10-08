# Class inventories with control needles (constitution 11.4.276(C), 11.4.201(7)(b)) - WF23 fix pass 2

| Field | Value |
|---|---|
| Created | 2026-10-08 |
| Instrument context | host-side measurements over the `git archive HEAD` scratch snapshot of the review (8,163 files, `/dev/shm/wf23/snap`; it holds the helix_qa banks, baselines and anchors only, 24 `export-ignore` paths are missing); the tool runs and the real-snapshot test also ran there. The REAL freeze (T161) does not exist yet: every number is UNCONFIRMED for it |
| Reproduce | `REAL_FREEZE_JSON=<freeze.json with remotes> EXPECT_STRICT_INVALID=399 python3 scripts/register/tests/test_real_snapshot.py` (transcript `fix-r2-real-snapshot-run.txt`) |

Each inventory states the class, the instrument, the control needle the instrument found first, the members, and what changed against the pre-fix tools (`fix-r2-prefix-scripts/`).

## K1 / A1 - defect ids: the id census

- Class: every defect-id family referenced anywhere in the main-repository snapshot (doc03 section 9: ids referenced only in prose are imported).
- Instrument: `grep -rIohE` with an ERE per family over the snapshot (specs/, submodules/, .audit/, .git excluded; the test-fixture directory `scripts/register/tests/` re-grepped to remove carrier-only ids), independent of the Python `re` set of the tool. Control needle: `FIX-QA-2026-04-21-001` found in the control file by the same grep.
- Members: 60 distinct ids (`CATAPI-DEFECT` 6, `FIX-QA`/`DEFER-QA` dated, `DEFER-001/002`, `FIX-OC2-001`, `FIX-OC3-001`, `FIX-OC3-011`, `FIX-OC4-016`, `FINDING-1..3`, `HQA-DOCS-001`, `HQA-0001/0002`, `HQA-PHASE1-GOCORE-001`, `FIX-011..019`, `BUG-001/002`, `FIX-OBS-001/002`, `FIX-BROWSER-001/002`, `FIX-CONCURRENCY-2026-06-29`, `FIX-CATAPI-2026-04-29-MEDIA`). The tool's entries equal the grep set exactly (`test_real_snapshot.py`). Pre-fix: 19 distinct ids; the 21 ids named by the review (FIX-QA-2026-04-21-001..010, FIX-QA-2026-04-20-002, FIX-QA-2026-04-22-003, DEFER-QA-2026-04-21-001/002, DEFER-QA-2026-04-22-002, FIX-OC2-001, FIX-OC3-001, FIX-OC3-011, FIX-OC4-016, HQA-DOCS-001, FINDING-3) are all present now (checked by name).
- Residue stated, not dropped: `id_like_unmatched` in the stats lists the tokens no family matches (placeholders `FIX-QA-YYYY-MM-DD-NNN`, `BUG-XXX`, `FINDING-LAYER`, `FIX-DEFECT-IN-TEST`, a bare date prefix `FIX-QA-2026-04-20`, `FIX-QA-2026-04-21-COVERS`).

## K1 / A4 - statuses

- S-03: pre-fix 269 of 269 rows had a NULL status (the file uses the glyph `⬜`); now 0 NULL, all `Not Started`; the glyph map is read from the file's own legend. Instrument: independent `grep -cE '^\| *[0-9]+\.[0-9]+'` = 269 rows.
- S-21: pre-fix 17 of 18 items NULL (the words sit on continuation lines); now 0 of 18 NULL, 27 entries (18 items + 9 sub-items). Instrument: an independent count of the numbered items of the section = 18.

## K1 / A3 - checkbox lines

- Instrument: `grep -cE '^\s*[-*+]\s+\[( |x|X)\]\s+'` per file; control needle: the same grep finds the checkbox line of a control file. The four docs/ files now have entries equal to the grep count (`COMPREHENSIVE_PACKAGE_SUMMARY` 37, `README_IMPLEMENTATION_PACKAGE` 8, `MASTER_AUDIT_AND_IMPLEMENTATION_PLAN` 20, `DISABLED_FEATURES_AUDIT` 4). The checkbox lines no checkbox source enumerates are LISTED in the stats (`checkbox_lines_not_enumerated`: 82 files, 3,757 lines on this snapshot; the review's 3,144 counted a different file set).

## K1 / A5 - file types of the marker scan

- The extension allow-list is gone: every non-binary file up to 2 MiB is scanned; skipped files are counted by reason (symlink, too_large, binary, unreadable). Marker rows pre-fix 2,603 = now 2,603 on the main-repo snapshot (the review measured 0 members of other types in the main repo and 251 lines in submodule trees; those are not in this snapshot). Skipped-test rows pre-fix 200, now 310: the +110 are `t.Skip(`-style text quoted in `.tsv` evidence files under `specs/` (carriers, listed for the step-3 NON-PROBLEM disposition, not hidden).

## K1 / C1 - the lead vocabulary

- Instrument: `grep -i` with explicit non-word boundaries over the population (2,717 files); control needle: the grep finds `the todos are stubbed` and does not find `depending on it`. The rows equal the grep set exactly (6,786 lines; pre-fix 6,685: 101 new, 0 lost). The 101 new: `stubbed` 23, `known issues` 22, `todos` 21, `workarounds` 17, `not-yet` 12, `not-implemented` 2, `stubbing` 1, `known-issues` 1, 2 other.
- Decision stated: the doc03 instrument is a bare substring and matches 483 more lines on this population (`check_pending_release`, `pending_pins`, `depending`, `visual_regression`, `v_gate_missing_objects`, ...); word boundaries are kept because those are identifiers and compounds, not lead words (the review's own list for this finding is the 86 inflected words, all covered). A switch to the substring form is the owner's call; it would add the 483.

## K2 - boundaries that lost items silently (A2, A6, A7, B1, B2, C2, C4, C7, F3, F4)

Each boundary was listed, given a row/listing/refusal, and tested (`test_enumerate_absolute.py`, `test_lead_scan.py`, `test_snapshot_checks.py`): external remotes and services (12 rows on this snapshot with 8 remotes), id-less bank cases (`#@<index>`), the front-matter forms (`status : x`, trailing comments, block scalars, quotes, continuation lines; 5 of 10 forms failed the pre-fix reader), skip-hit locators (numbered per tag), the stale re-import (all-or-nothing gate), per-line duplicate cover (pre-fix 2,781 lines carried the duplicate label because their FILE was structured; now 2,718 lines carry it, each naming an existing structured row, and the other lines of those files are `lead`), HC-2 (fixed path, malformed refused), symlinked population files (recorded), the malformed manifest and the FIFO (refused by name).
