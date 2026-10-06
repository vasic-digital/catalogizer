# gen_matrix.py and derive_applicability.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | new in the working tree (T195-T197), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed |
| Source | `tools/evidence/matrix/gen_matrix.py`, `tools/evidence/matrix/derive_applicability.py`; tests `tools/evidence/matrix/tests/test_gen_matrix.sh`; data `specs/001-full-project-audit-remediation/matrix/applicability.yaml` |

## Purpose
`derive_applicability.py` (T196, TS-00) re-derives `applicability.yaml` from direct reads of the repository: applications A1 to A9 from the 4.4 table of docs/05, every shared module of `.gitmodules` (A10 Go libraries, A11 TypeScript and React modules, A12 governance and QA modules) from a walk of its tree (test files, `Test` functions, benchmark and fuzz files, build tags, marker file names), A13 from `scripts/`, `tools/`, `tests/`, `challenges/`. Every cell carries its state and the reading it came from; there is no `?` cell. `gen_matrix.py` (T197) reads the map (and, optionally, the evidence ledger) and writes `coverage-matrix.json` and `coverage-matrix.md`: counts are derived, never typed, and a hand edit of an output is overwritten on the next run.

## Usage
`derive_applicability.py [--repo DIR] [--out FILE|-]`. `gen_matrix.py --applicability FILE --out DIR [--ledger FILE] [--timestamp UTC] [--gate] [--mint [--mint-cmd CMD] [--mint-ledger FILE]]`. Run through `scripts/test-in-container.sh tooling unit -- python3 -I tools/evidence/matrix/gen_matrix.py ...` (IMG-TESTUTIL carries python3 and PyYAML).

## Exit statuses
`gen_matrix.py`: 0 ok; 1 `--gate` failed (SC-004: an applicable cell is not `present`, each named with what is missing); 2 usage; 3 the map is invalid (a `?` cell, a missing type, an n/a without a reason, a wrong schema), every problem listed and NO output written; 4 a mint failed.

## The 13.1 status
With `--ledger` a cell is `present` only with a PASS record of three identical runs, a caught mutation and an evidence class at or above the type's requirement (unit `source`, ui/ux/helixqa `user-visible`, the rest `runtime`); `partial` when records exist and none qualifies; `blocked` when a record says so; `absent` when none; `n/a` from the map.

## Minting (spec US3 acceptance 4, docs/05 13.3 rev 19)
`--mint` mints one register item per applicable cell declared `A` or `~`, once: a cell already in the mint ledger is skipped (11.4.214). The command is `CMD <component> <type> <A|~>` printing the item id. Without `--mint-cmd` the run records the honest skip `mint_cmd_absent` (the `reg_ids` creation path under `locked.sh` is T069, not yet live); nothing is fabricated.

## Stated limits (11.4.6)
`P`, `~` in the map are structural readings (a marker file exists), never a verdict that a test uses a real system or passes. A name-based discovery can miss or over-count a marker; the ledger decides. The derivation changed one cell of docs/05 4.4: A8 Website DDoS is `n/a` (no server or hosting configuration in `Website/`), where 4.4 recorded `A`.

## Tests
`test_gen_matrix.sh`: hand-counted golden counts, the doc05 4.4 table tallied from the document text (9 `?` cells today, refused by the generator), the one-`?` fixture, n/a without reason, missing type, hand-edit overwrite, gate legs, minting legs, and eight paired mutations.
