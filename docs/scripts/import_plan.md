# import_plan.sh (import_plan.py) - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 (review fix round 2 completed: F2 compound labels, F3 repeated family member and the stated item bound) |
| Created | 2026-10-12 |
| Last modified | 2026-10-12T13:25:06Z |
| Status | tracked, new (WP-20, tasks T170 and T171); written by the worker, the independent review (constitution 11.4.142 / 11.4.209) is owed; nothing here writes the real register `docs/workable_items.db` |
| Source | `scripts/register/import_plan.sh`, `scripts/register/import_plan.py`, the S-26 / S-27 rules of `scripts/register/enumerate_sources.py`; tests `scripts/register/tests/test_seed_import.py` (T170), `test_innovation_import.py` (T171), `mutate_w2.sh seeds_enum|seeds_import|innovation_enum|innovation_import` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

Stage 0 and Stage 1/2 of two plan-document sources of the findings register (requirement-loss guard, 11.4.197):

* **S-26 plan-document seeds** (`--class seeds`, docs/21 sections 9.1 and 9.3): one `reg_source_entries` row (kind `report_doc`) per itemised seed id of the docs/21 section 9.1
  table, locator `<docs/21 path>#seed:<docNN>:<id>`; the 30 section 9.3 families are imported as one item each, the other members linked `duplicate_of`; a member that is only
  partly folded (`(part)`) keeps its own item. Items are `Task`, `Queued`, `component_id` NULL, no `reg_findings` row; severity: the highest member severity, the source's own
  label kept in `severity_source`, an unrated seed defaulted `medium [DEFAULT - adjustable]`; the R-ADJ seeds of docs/18 name the plan document and fix work package of docs/18
  section 1.1.
* **S-27 innovation entries** (`--class innovation`, docs/21 section 9.5): one entry per `PROPOSAL` (and `PROPOSAL` part) with its docs/18 candidate row; one `Feature` item
  each, `Queued`, category `gap`, severity `low` (a proposal has no severity), the docs/18 row, the evidence label under the docs/18 section 2.1 mapping (EMERGING recorded
  PLAUSIBLE, the secondary-source marking as the SINGLE-SOURCE qualifier, `n/a` with the repository citation), the ranked game changer and experiment where docs/18 has one, a
  Sources block and the line `**Disposition (ODG-39):** pending`. A proposal is never a finding, defect or task of feature 001 and is not implemented here.

Both classes read the plan documents from the **frozen snapshot** (never the live tree) through the one parser of `enumerate_sources.py`; the entry check
`check_freeze_snapshot.sh` runs first. A table whose Count cell, Total row, Counts line, family count or duplicate pair disagrees is refused at enumeration, never imported short.

## Usage

```bash
scripts/register/import_plan.sh --class seeds|innovation --db <register db> --freeze-json <freeze.json> --engine <workable-items binary> \
  [--category-map F] [--backup-record F] [--progress-log F] [--report F] [--reconciliation F] [--dry-run]
```

The real register is written only through `scripts/register/locked.sh`, after `scripts/register/backup_db.sh --record F` (`--backup-record F`; the real database name is refused
without it, `backup_record_required`). Idempotent and resumable (marker `register-import-plan:e<entry_id>` in `reg_ids.minted_by`). Exit: 0, 2 usage, 20 refusal (nothing
written), 21 resumable write failure, 1 other. The reconciliation (`--reconciliation F`) records the counts; for S-27 it covers all docs/18 candidates (R-ADJ only through S-26,
PROPOSAL only through S-27, mixed through both) and lists any mismatch.

## Tests and mutations

`test_seed_import.py` (64 checks) and `test_innovation_import.py` (51 checks): planted-entry completeness (one added row gives exactly one new entry, removed again the sql is
byte-identical), the table controls, the import (counts, relations, types, severity, descriptions within 2,048 bytes, validate/foreign_key/integrity clean), idempotence, a crash
after each stage then a re-run, refusals with nothing written. The mutation sets (19 + 26 + 17 + 26 entries, M17, the item-bound check, is killed by a docs/21 whose stated number is below the plan) run through `mutate_w2.sh`.

## Honest boundary

The real-document legs (`REAL_FREEZE_JSON`: 254 seeds, 198 items, 30 families, 34 innovation entries) run only after the T161 freeze (UNCONFIRMED until then). The evidence label of
a docs/18 row that mixes labels is a COMPOUND label (docs/18 section 2.1 has no row for it): its `;`-separated clauses are read separately (a parenthesis is a citation; a clause that does not start with one of the four label words, for example `integrated form not found`, states no evidence level and counts as UNCONFIRMED) and the WEAKEST clause is the register value, so `PROVEN restore (7.3); UNCONFIRMED marker-row verify` is UNCONFIRMED, never PROVEN. The description says `compound label PROVEN+UNCONFIRMED: the weakest part wins`. Review F2 found nine such rows in the real docs/18 (T2-B, T3-B, T5-D, T6-C, T6-D, T6-E, T7-C, T8-B, T10-C); the first-word rule recorded all nine as PROVEN. Whether the owner wants a different rule (for example a `MIXED` value) is an owner decision (`[DEFAULT - adjustable]`).


## Family members named twice

A member named twice inside ONE family of docs/21 section 9.3 is refused `family_member_repeated` (exit 20, nothing written): it would count as folded twice, shrink the item bound and make the planner plan more items than its bound. The stated member count of docs/21 counts the raw tokens, so the count check does not catch it (review F3). The item bound has two sources: the number derived from the parsed tables (seeds minus folded members) and the number docs/21 section 9.3 itself states ("fold to at most N register items"); a plan above the smaller of the two is refused `item_bound_exceeded` (exit 20, nothing written), and a document that states two different numbers is refused `declared_bound_conflict`. The stated number is the independent oracle: a docs/21 whose stated number is below the plan kills mutant M17 (`test_seed_import.py`, `stated_low`); for every accepted layout the planned item count equals the derived bound (four layouts). The reconciliation record carries `items_bound_stated_in_docs21`. Every reconciliation record (S-26 and S-27) also carries `freeze` (head, frozen_at, listing_sha256, manifest_sha256 of the freeze it was produced from) and `fixture` (derived: true unless the freeze head is a full 40-hex git sha), so a fixture record cannot be mistaken for a real-snapshot record (review fix round 2, F7).
