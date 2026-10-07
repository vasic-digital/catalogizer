# backup_db.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T03:00:00Z |
| Status | tracked; revision 3: WF13 review fix round (11.4.276 round 3, structural: defect classes C1-C6 of `scripts/register/tests/test_fix_r2.sh`); the independent re-review of this revision is owed (constitution 11.4.142) |
| Source | `scripts/register/backup_db.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The one pre-op backup of `docs/workable_items.db` (docs/04 section 12.2). A hardlinked copy is not a backup: SQLite writes its pages in place, so `cp -al` shares the inode and the "backup" follows every later write. The helper takes the SQLite online backup (`.backup`) with the image's `sqlite3`; `cp -al` stays reserved for `.git` (T429).

## Usage

```bash
scripts/register/backup_db.sh --record <file>
```

## Behaviour

1. One `scripts/register/locked.sh` call runs one `sqlite3` script inside IMG-TESTUTIL: `PRAGMA wal_checkpoint(TRUNCATE);`, `.backup docs/workable_items.db.bak-<UTC>-<pid>-<ns>`, a canonical `.dump` of the source into the op's `/out`, and the sha256 of the source (`/out/source.sha256`), all under the one lock hold. The backup file is created with O_EXCL before the call (WF10 F7: two backups started in the same second never share a file, and a failing run removes only its own file); `source_sha256` is the hash taken under the lock (WF10 F14), never one read after the lock was released.
2. Read-only through the `immutable=1` URI (no `-shm`/`-wal` beside the backup): `PRAGMA integrity_check` must print `ok`, and a restore probe (the backup restored into a scratch database under `/out`) must dump to the same bytes as the source dump of step 1.
3. The record `--record <file>`: `backup_path`, `utc`, both sha256 values, both row counts (INSERT rows of the canonical dumps), `integrity`, `restore_probe`, `image_digest`, the op ids.

The result row of `PRAGMA wal_checkpoint(TRUNCATE)` must be `0|...` (WF13 N7, as `dump.sh` F10): a reader outside the lock blocks the TRUNCATE, and `source_sha256` (the main-file hash) would then name a state that lacks the WAL pages the backup holds, so the backup is refused `checkpoint_incomplete`. Every failure after the backup file was created (`die` as well as `fail`) removes that file (WF13 N6). Any failed check exits 1, removes the backup file and writes no record: a backup that fails a check is not a backup and the bulk step does not start. Backup files are `docs/workable_items.db.bak-<UTC>-<pid>-<ns>` (ignored through `docs/*.bak-*`, T004). T069, T071, T168, T184, T223, T224 and `locked.sh import-sql` (register imports only) take every register backup through it.

## Exit codes

0 ok; 1 a check failed or the wrapper failed; 2 usage.

## Tests and evidence

`scripts/register/tests/test_backup_db.sh`: after the helper ran, one more row is written and checkpointed in the source, and the backup's sha256 and item count stay as recorded; the control, a `cp -al` copy in the same test, changes. Fault hook `BACKUP_FAULT=truncate|dumpdiff` (test mode only). The `LOCKED`, `LOCKED_RUNP`, `LOCKED_ROOT` and `BACKUP_FAULT` environment overrides are ignored outside `LOCKED_TEST_MODE=1` (WF10 F8). `scripts/register/tests/test_fix_r1.sh` sections F7, F14, F8 and M6 pin the revision-2 behaviour (M6: a real index-damage fixture, the reviewer's control needle, makes the integrity check refuse). Paired mutation: a copy that replaces the online backup with `cp -al` fails the unchanged-after-write assertion (`mutate_register_ops.sh backup`, `$EV/wp06/backup-mutation.txt`). Evidence `$EV/wp06/backup-*`.

## Honest limits

`PRAGMA integrity_check` and the restore probe are each redundant with the other for every fault the harness can inject (a truncated file fails both); `no_checkpoint` and `record_without_rows_check` stay recorded as reviewed-equivalent. Revision 1 also listed `integrity_unchecked` as equivalent: that was wrong for two reasons (WF10 M6). The host-side line alone is redundant in effect (the in-container line skips the restore probe when the integrity result is not `ok`, so the host then fails on the missing restore dump) but the revision-2 run KILLS it, through the message of the M6 fixture (`restore probe produced no dump`); and the integrity check itself IS load-bearing for index-page damage that the canonical `.dump` cannot see: the two-layer mutant `integrity_gate_both_layers_removed` passes a corrupt backup and is killed by `test_fix_r1.sh` M6. Its entry was removed from `equivalent_ops_mutants.tsv`.
