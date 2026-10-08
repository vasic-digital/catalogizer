# gen_matrix.py and derive_applicability.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:57:44Z |
| Status | new in the working tree (T195-T197), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed; review round 1 (WF11, 2026-10-06 UTC): fixes B1, I1, I2, I7, I11, m1, m2, m3, m11 applied; independent re-review owed (constitution 11.4.142) |
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

## Review round 1 (WF11, 2026-10-06 UTC)
- **B1**: `gen_matrix.py --gate` without `--ledger` is now a usage refusal (exit 2, `requires --ledger`) and writes nothing: absence of evidence blocks exactly as a FAIL does (11.4.135). Under `--gate` a ledger record that names no component/type of the map is refused (exit 3 `ledger_record_unmatched`), and a map with no applicable cell is refused (exit 3 `gate_vacuous`). The class closed: every gate entry point that could pass over nothing (a missing ledger, a stray record, an empty scope).
- **m3**: the 11.4.44 header carries `Created` forward and raises `Revision` only when the body (everything but the three header fields) changed.
- **I1**: `derive_applicability.py` refuses an uninitialised or empty submodule (exit 3 `submodule_uninitialised`, every empty module named, nothing written): before, 51 applicable cells became n/a in the reviewer's probe.
- **I2**: markers are matched as path TOKENS (`e2e` is not `e2ee`, `load` is not `loader`, `fault` is not `defaults`, `perf` is not `perfetto`, `inject` is not `injector`); the only directories skipped are vendored third-party trees (`node_modules`, `vendor`, `opensource`), so first-party `scripts/build`, `scripts/coverage` and `pkg/coverage` are counted.
- **m1 / provenance**: the files are enumerated with `git ls-files` per root (build output and untracked scratch are not counted); the YAML header carries `enumeration` and a `files_fingerprint`, so a re-derivation is checkable. **m2**: `MANIFEST.yaml` is not a bank. **m11**: the Website DDoS cell is re-measured on every run (a hosting configuration file flips it to `A`).
- **I11**: the A9 build `unit` cell is re-read from `tests/test_build_system.sh` (it sources the Build/lib libraries from a temp copy of `Build/`): `~`, not docs/05's `A`. `integration` and `full_automation` stay `A` with the re-read named.
- Measured effect on the committed map: 2 of 810 cells change state (A9 unit `A` to `~`; A10:assets chaos `~` to `A`); the totals P 46, ~ 137, A 414, n/a 213 are unchanged.
- **Tests**: `test_gen_matrix.sh` now has legs for the verdict, class and blocked rules and the gate inputs (the reviewer's RM1-RM4 mutations are adopted verbatim); the new `test_derive_applicability.sh` runs the deriver over synthetic repositories (determinism, empty submodule, token markers, A9, banks, Website, the unit `P` threshold = the reviewer's RM5) with a sandbox control for its mutations.

## Review round 5 (WP-23 fix round, 2026-10-07 UTC): the gate judges the REAL ledger
- **Ledger**: the evidence ledger as `tools/evidence/evrec` writes it. Its chain is walked first (`evcore.chain_walk`); a deleted, reordered, forged or truncated line refuses the whole ledger (exit 3 `ledger_chain_invalid`). The status of a cell is DERIVED by `evverdict.derive` (RED failed, three identical GREENs, the cycle after the last cutting REOPEN), plus a caught MUTATION entry and an evidence class at or above the type's need (`source < artifact < runtime < user_visible`). The earlier home-made ledger shape (`runs`, `identical_runs`, `mutation_caught`) is gone.
- **`--cell-items FILE`**: JSON `{"<component>|<type>": "<register item id>"}`. A key naming no component/type is `cell_items_unmatched`, a value that is not a register item id is `cell_items_invalid` (both exit 3).
- **Gate**: `--gate` requires `--ledger` and `--cell-items` (exit 2 without: no evidence blocks like a FAIL) and either `--repo DIR` (the map is re-derived from that repository and must equal it cell for cell: `map_not_bound`, exit 3) or an explicit `--map-unbound` (printed and recorded as `map_bound: false`). A map with no applicable cell is `gate_vacuous` (exit 3). `--candidate-fingerprint SHA` makes the GREEN entries answer for that build only.
- **Inputs recorded**: the json carries the sha256 of the map, the ledger and the cell-items file, `map_bound` and the candidate fingerprint.
- **Exit codes**: 0 ok; 1 the gate failed; 2 usage; 3 an input cannot be processed (a top-level handler turns any unexpected exception into 3: exit 1 always means the gate failed); 4 a mint failed. The YAML loader refuses duplicate and non-string keys; `--timestamp` must be `YYYY-MM-DDTHH:MM:SSZ`.
- **Minting**: `CMD <component> <type> <A|~> <idempotency-key>` (`mint:<component>|<type>`) must print a register item id (`CAT-nnn`, `FND-nnnn`, `RUN-n`, `AUD-x`); the same id for two cells and any output that is not an id are refused (exit 4). The mint ledger (default `mint-ledger.json` NEXT TO THE MAP, never under `--out`) is locked exclusively for the whole run; an intent row (`pending`) is written before the register is called and retried with the same key; a `minted` cell is never minted again. A malformed ledger refuses the run (exit 4).
- **Tests**: `test_gen_matrix.sh` builds its ledgers with the real recorder (hermetic scratch ledger, RED + 3 GREEN + a caught MUTATION per item). Mutants run in a symlink farm of `tools/evidence` so their imports are the real modules, with a sandbox control.
- **Not done in this round**: regenerating `matrix/applicability.yaml` and the committed matrix from the commit's tree (needs the `derive_applicability.py` rewrite below).
