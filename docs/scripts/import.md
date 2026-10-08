# import.sh (import_tickets.py) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T09:00:00Z |
| Status | tracked, new (WP-20, task T168, with the test-first legs of T163); written by the worker, the independent review (constitution 11.4.142 / 11.4.209) is owed; the real register `docs/workable_items.db` does not exist yet and has never been written by this importer (register go-live is T069); the importer is UNCONFIRMED against the real T161 freeze (stand-in snapshot tools only) and against the 1,778-ticket corpus |
| Source | `scripts/register/import.sh`, `scripts/register/import_tickets.py`, `scripts/register/category_map.yaml`; tests `scripts/register/tests/{test_import.sh,test_import_classes.py,mutate_import.py,mutate_import.sh}` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

Stage 1/2 of the findings-register import (doc04 section 9.3, the `register-import` actor; tasks.md T168). Stage 0 (`enumerate_sources.py`, T165) puts every source entry into `reg_source_entries`; this importer turns every `issue_file` entry (the `docs/issues` tickets and the `issues/ANR-*` files) into a register item:

1. **mint** a `reg_ids` row (`mint_basis='import'`, `minted_by='register-import:e<entry_id>'`, so a crash is resumed from the same id and never mints a second one);
2. **write `reg_item_ext` BEFORE the item exists** (owner answer): `custody_basis='legacy_import'`, `reverify_required=1` and the raw `legacy_status` for a closed-class ticket (`resolved`, `fixed`, `closed`); `machine_evidence`/0 for `wontfix`, `open` and anything else (FR-008: `wontfix` is not a permitted closure, it imports `Queued` with `legacy_status='wontfix'`); `defect_layer='source'` (the lowest evidence floor, it can only be raised); `component_id` NULL; `severity` by the IC-01 scale (S1..S5 or the five words; an unmapped or absent value defaults to `medium` and says so in `severity_source`);
3. **add** the item with the real engine, `workable-items add <Type> <severity> --id CAT-NNN --prefix CAT` (id prefix `CAT`, never `ATM`);
4. **close** a closed-class ticket on the legacy path with the engine's per-Type closure vocabulary (Bug `fixed`, Task `completed`, Feature `implemented`; evidence = the ticket file of the snapshot), which lands it terminal-but-unverified in `v_reverify_queue`;
5. **map** the entry (`reg_source_map`: `relation='primary'`, `match_basis='import_1to1'` until the intake-match calibration allows more, `severity_governs=1`, `mapped_by='register-import'`).

Entries are processed in `entry_id` order, so the CAT ids ascend in the legacy order. A second listing of the **same ticket path** (another source) is linked `duplicate_of` the head's item (severity_governs 0) and never mints a second item (11.4.214).

The ticket text and its sha256 come from the **frozen snapshot** named by `--freeze-json` (never the live tree); the entry check `scripts/register/check_freeze_snapshot.sh` runs first and its refusal stops the import with nothing written (`freeze_snapshot_moved`). The description is the ticket body (front matter and first heading removed) plus a **Sources** block (file, sha256, legacy id with the raw status/severity/category, the category proposed by `category_map.yaml` and the engine type with its basis), **at most 2,048 bytes in all**, cut on a UTF-8 character boundary and marked as cut when the text is longer; the full text stays in the source file the block cites.

### Type, category and what is NOT decided here

* **Type**: from `category_map.yaml` `types` (functional/functionality -> Bug; content/brand/documentation -> Task); every other raw category imports as `Task` and the Sources block says `[DEFAULT - adjustable]`. `Feature` is never assigned (doc04 9.2: only when the text says so).
* **Category** (BLOCKED-ON ODG-17 / OD-16, no `research.md` default): the importer writes ONE constant interim value (`interim_category` of the map, `shortcoming`) into `reg_item_ext.category`, because the column is `NOT NULL` and closed; it records the raw category and the **proposed** value (`categories` of the map) in the item's Sources block and, with `--category-out F`, in a TSV (`locator, legacy_id, raw_category, proposed_category`). It never writes the proposed value. T223 applies the map after HC-2 (T222). UNCONFIRMED: that a constant placeholder is the owner's wish for the interim; the alternative (apply the proposal now) was rejected because the task says "without applying".

## Usage

```bash
# a scratch run (tests, rehearsals): directly, inside IMG-TESTUTIL, on a scratch DB made by apply_ext.sh + the enumerator's SQL
scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- scripts/register/import.sh \
  --db <scratch db> --freeze-json <freeze.json> --engine <workable-items binary> [--category-out F] [--report F] [--progress-log F]

# the real register (after T069): the ONE command of locked.sh (single writer), after the pre-op backup, in the background
scripts/register/backup_db.sh --record $EV/register/backup-import.json
scripts/register/locked.sh -- scripts/register/import.sh --db docs/workable_items.db --freeze-json <freeze.json> \
  --engine submodules/constitution/scripts/workable-items/bin/workable-items --backup-record $EV/register/backup-import.json \
  --progress-log $EV/register/import-progress.log --report $EV/register/import-run.json --category-out $EV/register/category-raw.tsv
```

Options: `--kinds` (default and only supported value `issue_file`), `--category-map F`, `--busy-timeout-ms N` (default 5000), `--dry-run` (reads and checks, writes nothing), `--backup-record F` (see below).

Exit: **0** ok; **2** usage; **20** refusal (`import: REFUSED reason=<code>`, nothing written); **21** a write failed and the run is resumable (`db_locked`, `disk_full`, `io_error`; every finished entry is kept, a re-run continues); **1** engine or invariant failure.

## Refusals and failure classes (every one has a test leg in `test_import_classes.py`)

| Class | Reason (exit) | Notes |
|---|---|---|
| missing / invalid freeze, missing snapshot dir or manifest | `freeze_missing`, `freeze_invalid` (20) | |
| malformed manifest; sha tampered; file changed, added or removed after the manifest | `freeze_manifest_invalid`, `freeze_snapshot_moved` (20) | from `check_freeze_snapshot.sh` |
| changed entry at the same locator (snapshot consistent, register hash old) | `entry_changed` (20) | names each locator with both hashes |
| ticket file missing, not UTF-8, unsafe locator (`..`, absolute, outside the snapshot) | `ticket_missing`, `ticket_not_utf8`, `locator_unsafe` (20) | the whole selection is read first: no partial import for a bad file |
| duplicate locator (two sources, one ticket) | linked `duplicate_of` (0) | never minted twice |
| crash between the steps of one entry | resumed (0 on re-run) | five stages tested for a closed-class and an open entry; the state equals a clean run |
| concurrent importers | `import_already_running` (20) | advisory `flock` on the database file (no stray lock file in the tracked tree) |
| entries of other kinds | stay in `v_unmapped_entries`; `--kinds` for them: `kind_unsupported` (20) | T170-T172 and later tasks own them |
| database locked by another writer | `db_locked` (21) | `--busy-timeout-ms` bounds the wait |
| disk full | `disk_full` / `io_error` (21), or `disk_headroom` (20) before writing | free-space precondition 32 MiB |
| real register name without a valid backup | `backup_record_required`, `backup_record_invalid`, `backup_stale` (20) | the record of `backup_db.sh` must name an existing backup and the sha256 of the unchanged database |
| register DDL absent | `register_ddl_absent` (20) | |

Idempotency: an entry already in `reg_source_map` is skipped; a finished import writes nothing on the next run. At the end the importer runs the engine's `validate`, `PRAGMA foreign_key_check` and `PRAGMA integrity_check` and reports `v_unmapped_entries` for the selected kinds (must be 0).

Test hooks (refused unless `IMPORT_TEST_MODE=1`): `IMPORT_FAULT=<stage>:<n>`, `IMPORT_MIN_FREE_BYTES`, `IMPORT_PAUSE=<n>:<file>`.

## Tests and evidence

`test_import.sh` (T163: engine identity, idempotency, completeness, legacy order, description cap), `test_import_classes.py` (the failure classes above and the content mapping), `mutate_import.py` / `mutate_import.sh` (34 mutants incl. a comment-only control that must survive and the unmutated tree that must pass). Evidence: `$EV/wp20/import-*` with `SHA256SUMS`.

## UNCONFIRMED

* The importer has run only on fixture snapshots (tens of tickets) in IMG-TESTUTIL, never on the 1,778 real tickets and never on the real register; the measured runtime for 1,778 items is unknown (the engine is one process per command; doc04 estimates minutes).
* `freeze.json` shape: the enumerator's reading (`snapshot`, `manifest`) is assumed to be T161's.
* `entry_sha256` of a Stage 0 `issue_file` entry is the sha256 of the file bytes (true for the stand-in enumerator rev 2).
* The instructions of the `--backup-record` check rely on the record fields of `backup_db.sh` (`backup_path`, `source_sha256`, `integrity`, `restore_probe`); the combination with a WAL-mode database whose `-wal` is not empty is untested (the record's hash is the main file's).
* The scope convention that `workable-items intake-match` needs (`**Affected scope / file-scope manifest:**` in the description) is NOT written by this importer; T172 must decide the scope string (see `docs/scripts/intake_match.md`).
