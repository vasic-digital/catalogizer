# W2 review fix round 2: convergence assessment (constitution 11.4.276)

Revision: 1 | Created: 2026-10-12 | Status: written by the resuming worker after a forced host power-off killed the first fixer mid-run; reviewer: Opus 5.5 at unrecorded effort (`?`), so the NO-GO verdict is provisional (11.4.209).

## Round count

This is the second independent review of W2 (round 1 produced the r1 fixes, the independent review of 2026-10-12 produced F1-F9: 0 Blocking, 6 Important, 3 Minor). The fix of F1-F9 is this single class-complete pass, round 2 of at most `R_max`; no round 3 is requested before the next independent review.

## Defect classes (11.4.276 (C)), not instances

| Class | Findings | What closes the class (not only the instance) |
|---|---|---|
| ORDER guard at the wrong seam | F1, F5 | the guard lives in the importer that would mint (`import_tickets.py`, before any mint, dry run included); `import_t172.py` shares the SAME classifier (`qa_cause_of`), so the two cannot disagree; crash-then-import and mapped-elsewhere scenarios are real-path tests; the refusal text carries the recovery path and the guide has a Recovery section |
| Evidence inflation by simplification | F2 | the compound label takes its WEAKEST part (not its first word); all nine compound rows of the real docs/18 are asserted, plus a clause with no evidence level (UNCONFIRMED, mutant M21) |
| An invariant with no independent oracle | F3 | duplicates inside a family are refused at parse time, AND the item bound is checked against the number docs/21 states itself (`fold to at most N register items`), an oracle independent of the parsed tables; M17 is therefore KILLED and the equivalence row is removed (the first fixer had re-argued M17 as an equivalent; that contradicted the brief and is reverted) |
| Classifier membership decided by example | F4 | class membership is one function with a closed terminal-status set (`resolved`, `fixed`, `closed`, `wontfix`) and a closed statement set; `tests/inventory_qa_classes.py` runs the real classifier over every ticket with planted needles (a member and a non-member per class), `fix-r2-qa-class-inventory.txt` |
| Disposition rules not probed against the real entry set | F6 | `root_dispositions.yaml` decides `constitution`, `.opencode`, `.kimi-code`, `.env` and the live untracked entries; the suite runs the rules against the root entries of git HEAD (`tests/head_root_entries.tsv`) and has a control that the pre-fix rules FAIL on exactly the review's entries |
| Output record without provenance | F7 | every T172 record, `recount.json` and `root-items.json` carry the freeze identity (head, listing hash, manifest hash) and a derived `fixture` boolean (true unless the freeze `head` is a full 40-hex sha, so a freeze.json without `head` reads as fixture, the safe direction; the field names are the stand-in shape of the enumerator, UNCONFIRMED vs the T161 owner); the suite asserts it per record |
| Pattern-boundary inconsistency | F8 | one boundary rule (`(?<![a-z0-9])...(?![a-z0-9])`) for every platform class; a declared `Android TV` is normalised to the class |
| Documentation/measurement drift | F9 | guide counts re-derived from the suite output, mutant floors met for every tool, the O(n^2) duplicate count replaced by one `Counter` pass |

## Control-needled member inventories

- QA classes: `fix-r2-qa-class-inventory.txt` (7 needles, 4 members and 3 non-members; then the 1,778 tickets of the LIVE tree, UNCONFIRMED for the T161 freeze). Grouped 393 of 1,778; ungrouped candidates 3 distinct resolutions on 6 tickets, each a product statement, not a QA-instrument one.
- doc12 cites "443 attributed to QA infrastructure": that figure is doc03 F-5's attribution of the 460-ticket bulk close of 2026-04-17 (a count over a different ticket set, 1,965 tickets in the report vs 1,778 files). It is NOT reproducible from the live tree: 383 resolutions carry the statement, 393 are grouped (383 plus 10 "False positive: screenshot captured during screen transition" resolutions). The difference of 50 to 60 is recorded as UNCONFIRMED, not reconciled; no code depends on the 443.
- Compound evidence labels: all 47 candidate rows of the real docs/18 (live tree), 9 compound.
- Root entries: the root entries of git HEAD (`tests/head_root_entries.tsv`, 144 data rows) plus the live untracked entries (`.env` decided by name, its contents never read).

## Honest boundary

Real-snapshot legs of T170/T171/T172/T174 and every count taken from the live tree stay UNCONFIRMED until the conductor-owned T161 freeze exists. The T172 matcher-merge decision is pending and not taken here. The tasks.md T172 dependency text ("depends on T168") contradicts the implemented order (stage qa before import.sh): reported as an open request, tasks.md not edited.
