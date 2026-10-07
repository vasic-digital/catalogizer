# reconcile.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T03:00:00Z |
| Status | tracked; revision 2: WF13 review fix round (11.4.276 round 3, structural: defect classes C1-C6 of `scripts/register/tests/test_fix_r2.sh`); the independent re-review of this revision is owed (constitution 11.4.142) |
| Source | `scripts/register/reconcile.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The reconciliation, re-verification, reopen, tracker and findings reports of the register (docs/04 section 11.2, formerly "to be written"), generated from the database as CSV and Markdown.

## Usage

```bash
scripts/register/reconcile.sh --db <path> --out-dir <dir>      # runs inside IMG-TESTUTIL, called by export.sh
```

Outputs: `reconciliation.csv`, `unmapped_entries.csv`, `legacy_id_collisions.csv`, `reverify_queue.csv`, `reopen_counts.csv`, `stale_tracker_sync.csv`, `findings.csv` and `Reconciliation.md` (one section per view as a Markdown table, headed by the revision header table; class `generated`, T040b). Every output is a pure function of the database content (no clock, host name or counter); `Last modified` is the largest `items.last_modified` or `none`. Every query has an explicit `ORDER BY`.

## Exit codes

0 ok; 2 usage; 3 database unreadable; 4 query failed.

## Empty views (WF13 N9)

sqlite prints no header line for an empty result, so the column list of every view is held in `HDR`. For a non-empty view the first CSV line must equal `HDR`; for an EMPTY view the column list of a TEMP view over the same query (`PRAGMA table_info`) must equal `HDR`, else exit 4 (`the header of view <n> (empty) is [..], the HDR table says [..]`): a hand-kept header can no longer ship stale until the view has rows.

## Tests and evidence

Through `scripts/register/tests/test_export.sh` (golden collision and unmapped-entry rows). Evidence `$EV/wp06/export-*`.
