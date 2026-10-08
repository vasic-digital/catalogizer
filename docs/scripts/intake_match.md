# test_intake_match.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T09:00:00Z |
| Status | tracked, new (WP-20, task T169); written by the worker, the independent review (constitution 11.4.142 / 11.4.209) is owed; measured on the eight `HELIX-001-*` tickets only, NOT on the whole 1,778-ticket corpus |
| Source | `scripts/register/tests/test_intake_match.sh`, `scripts/register/tests/intake_match_fixture.json` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

Calibration of the engine matcher `workable-items intake-match --config <cfg> --report <r> --out <o>` (doc04 sections 8 and 13.2, constitution 11.4.214(4)) before it is used to cluster the legacy corpus (T172) and before `match_basis='import_1to1'` may be dropped. The matcher is trusted only when both controls pass, three times in a row on freshly built databases.

The test uses a scratch register (`apply_ext.sh`), the real engine binary, items minted the register way (`reg_ids` then `add --id`) and **dry-run decisions only** (no `--apply`).

## What it checks

| Leg | Check |
|---|---|
| (a0) | the matcher refuses a call without `--config` (exit 1, no decision file) |
| (G) golden | three re-reports of a ticket (same text with one added sentence, or re-titled with the same text) resolve `SAME_DEFECT` to the right item |
| (S) scope | the same text under another scope, or an empty scope, is `DISTINCT` (the scope is part of the key, 11.4.186) |
| (N) negative | control needle (an identical report of a registered ticket is seen); the primary negative control (two of the eight `HELIX-001-*` tickets, one id, different problems, same scope) is `DISTINCT`; leave-one-out over all eight never yields `SAME_DEFECT` and equals the pinned verdict vector of the fixture |
| (R) | three repetitions give identical result vectors |
| (P) | `intake-match --apply` on a register database is refused by the identity trigger and writes nothing |

## Measured result (engine sha256 `d789d625...`, 2026-10-08, in IMG-TESTUTIL)

`$EV/wp20/import-intake-match-calibration.json`: golden 3 of 3 `SAME_DEFECT`; scope controls 2 of 2 `DISTINCT`; leave-one-out 6 `DISTINCT`, 2 `UNDECIDED`, **0 false merges**. The two `UNDECIDED` are `Insecure Password Input` and `Password Input Field Does Not Display Password Strength Indicator` against each other (they share most of a sentence): mint-with-link, never a merge (11.4.214 clause 3).

## Findings for the owners of T172 and of the engine

1. **Re-worded duplicates are not merged.** A duplicate whose text is paraphrased scores below the engine's 50-point threshold (a measured re-worded report of the API ticket scored `UNDECIDED`). The matcher merges re-reports of the same text and re-titled reports with the same text; T172's "family" merges for the 294 `QA infrastructure screenshot timing` closures must expect that and treat `UNDECIDED` as mint-with-link.
2. **`intake-match --apply` cannot mint on a register database.** Its mint path calls `add` without `--id`; the register identity trigger (`trg_items_require_mint`) refuses (`atm_id has no reg_ids row`). Use the dry-run verdict, then mint through `reg_ids` + `add --id` (doc04 12.3). Test leg (P) pins this; if the engine gains an `--id` path the leg must change on purpose.
3. **The scope key is the marker `**Affected scope / file-scope manifest:**` in the item description**, exact string equality. The importer (`import.sh`, T168) does not write it, because the scope string for a legacy ticket (platform, screen) is T172's decision; items without the marker never match (an absent scope is not a wildcard).
4. **The corpus is not measured.** Thresholds (50 / 20) are the engine author's, derived on two fixtures; this record is a check on eight real tickets. The "trusted on the corpus" condition of T169 is therefore met only for this fixture; a full-corpus measurement (e.g. leave-one-out over a sample of the 1,778 tickets) is an open item for T172.

## Run

```bash
RUNP_MEMORY=<bytes> scripts/containers/run_pinned.sh --network=none --out <abs dir> IMG-TESTUTIL -- \
  env CALIBRATION_OUT=/out/intake-match-calibration.json bash scripts/register/tests/test_intake_match.sh
```

Env: `WI` (engine), `INTAKE_FIXTURE`, `CALIBRATION_OUT`, `EV`. RED (fixture absent): the first run before `intake_match_fixture.json` existed failed with `calibration fixture absent`.
