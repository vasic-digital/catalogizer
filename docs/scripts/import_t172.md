# import_t172.sh (import_t172.py) - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 (review fix round 2: F1 order guard + recovery, F4 classifier classes, F7 freeze identity, F8 platform rules) |
| Created | 2026-10-12 |
| Last modified | 2026-10-12T13:25:06Z |
| Status | tracked, new (WP-20, task T172); written by the worker, the independent review (constitution 11.4.142 / 11.4.209) is owed; nothing here writes the real register `docs/workable_items.db` |
| Source | `scripts/register/import_t172.sh`, `scripts/register/import_t172.py`, the classifier and the order guard in `scripts/register/import_tickets.py`, `scripts/register/tests/inventory_qa_classes.py`; tests `scripts/register/tests/test_t172.py`, mutations `mutate_w2.sh t172` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

doc12 WP-Q5 ticket specifics inside the import (docs/12 sections 11.3 and 11.4, QF-10; doc04 sections 8 and 9). Ticket text comes from the frozen snapshot through the reader
of `import_tickets.py`; no evidence file the ticket names is opened and the live tree is never read. Tickets are keyed by path (`HELIX-NNN` is a non-unique legacy label,
`v_legacy_id_collisions`). **No `reg_source_entries` row is inserted by either stage**: the sorted triple export (`SELECT s.locator, e.locator, e.entry_sha256 ... ORDER BY 1, 2, 3`)
is taken before and after, written to `entryset-before.tsv`, `entryset-after.tsv` and `entryset.json`, and a difference is refused `t172_entry_set_changed` naming each triple.

* `--stage qa` (BEFORE `import.sh`, T168): the terminal-status tickets (`resolved`, `fixed`, `closed` and `wontfix`; never `open`) whose RESOLUTION text says the QA tool was wrong
  ("QA infrastructure failure ... screenshot timing", wrong screen, black frame, wrong ADB command or package, "test infrastructure", "False positive: screenshot captured during
  screen transition or app loading", any other QA statement) are grouped by the stated cause into **one `Bug` item per cause**, component `helixqa` (or `qa-scripts` for the
  ADB cause), `Queued`, category `bug`; every ticket of the cause is mapped to it in `reg_source_map` (lowest locator `primary`, the others `duplicate_of`, `manual_operator`, exactly
  one `severity_governs`), so T168 skips those entries and imports no per-ticket item. A QA ticket already mapped to another item is refused (`qa_entry_individually_imported`, a
  mapping is corrected, never deleted; see Recovery); a component that is not a `reg_components` row is refused (`component_not_registered`). Output: `qa_causes.json`.
* `--stage families` (AFTER `import.sh`): platform inference for the tickets with an empty `platform:` from the path strings of the ticket text (screenshot, frame and logcat
  names, URLs): a declared platform wins, one evidence class is inferred and marked `[DEFAULT - adjustable]` (also appended to the item description), two classes or none are
  `UNKNOWN`, never guessed; duplicate families by (normalised title slug, platform class, screen) with one `reg_recurrence_links` row per non-head member (intake_path `import`,
  verdict `UNDECIDED` = mint-with-link, the doc04 section 8 autonomous default, head = the lowest locator); an empty screen is no scope and is never linked; the same slug under another
  platform class or screen is recorded DISTINCT. Output: `families.json`, `platform_inference.json`.

### The classifier (one definition, shared with `import.sh`)

`import_tickets.qa_cause_of(status, front matter, body)` decides the class; T168 and T172 call the same function. The resolution text is the front-matter `resolution:` field
plus the `## Resolution` section of the body (a QA phrase elsewhere in the body, for example under `## Notes`, does not count). Class membership, measured on the LIVE
`docs/issues` (1,778 tickets; NOT the T161 freeze, so UNCONFIRMED for the freeze) by `scripts/register/tests/inventory_qa_classes.py` (evidence `$EV/register/w2/fix-r2-qa-class-inventory.txt`):
393 tickets are grouped (closed 299, fixed 42, resolved 31, wontfix 21). Review F4 found 21 `wontfix` and 10 `resolved` ("False positive: screenshot captured during screen
transition or app loading", written in the body section only) that the earlier classifier missed. `wontfix` is INCLUDED because the statement "the QA tool, not the product, was
wrong" is the same defect whatever closure word the ticket carries (`[DEFAULT - adjustable]`, an owner decision). Doc12 section 11.3 and document 03 F-5 cite 443 tickets
"attributed to QA infrastructure"; that number is the 2026-04-17 bulk-close report's own count of a mixed batch of 460 tickets, not a ticket-level statement set, so it is NOT
reconciled with the 393 (UNCONFIRMED: the report lists no ticket ids). Six more terminal tickets mention `infrastructure`, `false positive` or `screenshot` without being a
QA-instrument statement (the inventory lists them).

## Order, and what to do when it was broken

`enumerate_sources.py` -> `import_t172.sh --stage qa` (to completion) -> `import.sh` -> `import_t172.sh --stage families`. The order is a consequence of the schema: ids are never
retracted and a `reg_source_map` row is never deleted, so a QA ticket imported as its own item cannot be folded into its cause item afterwards. Therefore **`import.sh` refuses
`qa_grouping_not_run` (exit 20, before any id is minted, dry runs included) while a QA-instrument ticket is not mapped yet**, for example because stage qa was never run or
crashed part way (re-run stage qa: it resumes; then run `import.sh`).

## Recovery

If a QA ticket was nevertheless imported as its own item (a register written before this guard existed), stage qa refuses `qa_entry_individually_imported` and writes nothing. This
tool cannot repair it. Restore the pre-import backup that `backup_db.sh` recorded (the `--backup-record` of the import), run `--stage qa` to completion, then `import.sh`. A
register that cannot be restored needs an OWNER DECISION on the orphaned per-ticket items (the minted ids stay; retire them with a recorded reason, or leave them and link them
by hand); that decision is not made here.

## Usage

```bash
scripts/register/import_t172.sh --stage qa|families --db <register db> --freeze-json <freeze.json> --engine <workable-items binary> --out-dir <dir> \
  [--backup-record F] [--progress-log F] [--report F] [--dry-run]
```

Order (enforced by `import.sh`, see above): `enumerate_sources.py`, `import_t172.sh --stage qa`, `import.sh`, `import_t172.sh --stage families`; the real register is written only through `locked.sh` after the pre-op
backup. Exit: 0, 2 usage, 20 refusal (`import_t172: REFUSED reason=<code>`), 21 resumable write failure, 1 other. Idempotent and resumable (marker `register-import-t172:qa:<cause>`).

## Tests and mutations

`test_t172.py` (60 checks): the entry-set invariant (identical exports; a trigger that inserts one row, and one that inserts one row and deletes another so the count is unchanged,
are each refused naming the triples), the cause grouping, classification limits (an open ticket and a closed non-QA ticket are not grouped), platform inference, families and
distinct groups, crash and resume after each stage, refusals; review fix round 2 adds the order guard (import.sh before stage qa, after a crashed stage qa, dry run), the
mapped-elsewhere refusal, the classifier classes member by member (a wontfix QA ticket, a `## Resolution` body statement, and the non-members), snake_case platform names, a declared
`Android TV`, and the freeze identity of every record. 33 mutants run through `mutate_w2.sh t172` (the two mutants of the independent review, A and B, verbatim, are M02 and M03)
and 14 through `mutate_w2.sh t172_classifier` (the shared classifier and the `import.sh` guard in `import_tickets.py`).

## Records

`qa_causes.json`, `families.json`, `platform_inference.json` and `entryset.json` carry `freeze` (`head`, `frozen_at`, `listing_sha256`, `manifest_sha256`) and `fixture`:
`fixture` is true unless the freeze head is a full 40-hex git sha (derived, never typed), so a fixture record cannot pass as a record of the real snapshot. Platform names: an
underscore separates words (`android_home.png` is android; `logcat.txt` alone names no platform, because the log tool serves android and androidtv); a declared platform spelled as a class
(`Android TV`, `android_tv`) is that class (`androidtv`) in the family key.

## Honest boundary

The `UNDECIDED` family verdict is the autonomous default; the calibrated `intake-match` (T169) is not invoked by this tool, so no family is decided `SAME_DEFECT` here (UNCONFIRMED
whether the owner wants matcher-driven merges on the real corpus). The real-corpus legs (294 timing closures, the 73 % empty platforms, 246 repeated slugs) wait for the T161 freeze.
