# backup_db.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T17:30:00Z |
| Status | tracked from the WP-06 slice T064a; independent review of this revision owed (constitution 11.4.142) |
| Source | `scripts/register/backup_db.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The one pre-op backup of `docs/workable_items.db` (docs/04 section 12.2). A hardlinked copy is not a backup: SQLite writes its pages in place, so `cp -al` shares the inode and the "backup" follows every later write. The helper takes the SQLite online backup (`.backup`) with the image's `sqlite3`; `cp -al` stays reserved for `.git` (T429).

## Usage

```bash
scripts/register/backup_db.sh --record <file>
```

## Behaviour

1. One `scripts/register/locked.sh` call runs one `sqlite3` script inside IMG-TESTUTIL: `PRAGMA wal_checkpoint(TRUNCATE);`, `.backup docs/workable_items.db.bak-<UTC>`, and a canonical `.dump` of the source into the op's `/out`.
2. Read-only through the `immutable=1` URI (no `-shm`/`-wal` beside the backup): `PRAGMA integrity_check` must print `ok`, and a restore probe (the backup restored into a scratch database under `/out`) must dump to the same bytes as the source dump of step 1.
3. The record `--record <file>`: `backup_path`, `utc`, both sha256 values, both row counts (INSERT rows of the canonical dumps), `integrity`, `restore_probe`, `image_digest`, the op ids.

Any failed check exits 1, removes the backup file and writes no record: a backup that fails a check is not a backup and the bulk step does not start. Backup files are `docs/workable_items.db.bak-<UTC>` (ignored through `docs/*.bak-*`, T004). T069, T071, T168, T184, T223, T224 and `locked.sh import-sql` (register imports only) take every register backup through it.

## Exit codes

0 ok; 1 a check failed or the wrapper failed; 2 usage.

## Tests and evidence

`scripts/register/tests/test_backup_db.sh`: after the helper ran, one more row is written and checkpointed in the source, and the backup's sha256 and item count stay as recorded; the control, a `cp -al` copy in the same test, changes. Fault hook `BACKUP_FAULT=truncate|dumpdiff` (test mode only). Paired mutation: a copy that replaces the online backup with `cp -al` fails the unchanged-after-write assertion (`mutate_register_ops.sh backup`, `$EV/wp06/backup-mutation.txt`). Evidence `$EV/wp06/backup-*`.

## Honest limits

`PRAGMA integrity_check` and the restore probe are each redundant with the other for every fault the harness can inject (a truncated file fails both); the two mutants that remove one of them are recorded as reviewed-equivalent, not claimed killed.
