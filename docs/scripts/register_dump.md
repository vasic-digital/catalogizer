# dump.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T23:30:00Z |
| Status | tracked from the WP-06 slice T066; independent review of this revision owed (constitution 11.4.142); revision 2: WF10 review fix round 1 (F1-F14); the independent re-review of this revision is owed (constitution 11.4.142) |
| Source | `scripts/register/dump.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The deterministic text dump `docs/register/register.sql` of the register (docs/04 section 12.1 R-6): a reviewable derivative of the binary database, never edited by hand and never authoritative.

## Usage

```bash
scripts/register/dump.sh [--db docs/<name>.db] [--out docs/<dir>/<name>.sql]      # register mode, defaults docs/workable_items.db, docs/register/register.sql
scripts/register/dump.sh --out-dir <absolute dir> [--db-file <name>] [--out <name>] # out mode (replay.sh)
```

One `locked.sh` call: `PRAGMA wal_checkpoint(TRUNCATE)` first (R-2), then `sqlite3 'file:<db>?immutable=1' .dump` with the `PRAGMA` lines removed, written to a temporary file and renamed. The immutable read is exact because it runs right after the checkpoint under the register lock. Paths outside `docs/` or with `..`, spaces or quotes are refused (`path_invalid`, 20).

## Checkpoint and output checks (revision 2)

WF10 F10 (docs/04 section 12.2): the result row of `PRAGMA wal_checkpoint(TRUNCATE)` is checked (busy must be 0) and the `-wal` file must be empty afterwards, else the call is REFUSED `checkpoint_incomplete` (exit 20) and no dump is written: any other open connection (a reader outside the lock) blocks the TRUNCATE, and the immutable read would then miss committed rows still held in the `-wal` file (reproduced with a host reader holding an old snapshot: the committed `CAT-002` was missing from a dump reported OK). WF10 F8: after the wrapper exits 0 the dump must exist on the host, be newer than the start of the run and end with `COMMIT;`, else exit 1 (`FAILED reason=output_missing` / `output_incomplete`); the `LOCKED` override is honoured only with `LOCKED_TEST_MODE=1`. Pinned by `scripts/register/tests/test_fix_r1.sh` sections F8 and F10.

## Commit procedure (written down; run in T069 and by `scripts/commit-push-all.sh`)

1. `scripts/register/export.sh` (the checkpoint comes first; engine export, `reconcile.sh`, `reg_export_*` rows, engine diff "in sync").
2. `scripts/register/dump.sh` (checkpoint first). It runs after the export because the export adds its own `reg_export_*` rows to the database, and the dump must equal the database that is committed.
3. The database, the dump and the regenerated documents are committed together in one commit through `scripts/commit-push-all.sh`, never by `git commit` by hand (R-3): a database without its dump and exports, or exports without their database, is the drift this procedure prevents.

## Re-recording after a single-writer breach (docs/16 section 12.2.7 item 3, V-17 (k))

In a scratch clone checked out at the remote tip, `scripts/register/replay.sh` replays the local journal through `locked.sh` onto the remote side's database, and the dump and exports are regenerated there by this script and `export.sh` in out mode; the resolver copies the database, dump and exports into the refused run's `.audit/merge-resolution/<run_id>/` with their sha256 in `resolution.json`, never into the shared working tree. `locked.sh` keeps writing only `docs/` of the checkout it runs in, so no target-path mode exists. The procedure is exercised once in the T093 re-run with the real tools.

## Tests and evidence

`scripts/register/tests/test_dump.sh`: two and three consecutive dumps hash identically, no `PRAGMA` line, the dump follows a database change, dump -> restore -> dump is byte-identical, out mode equals register mode, refusals. Mutations `mutate_register_ops.sh dump`. Evidence `$EV/wp06/dump-*` (the recorded hash is in the GREEN transcripts).

## Honest limits

- The "checkpoint first" step is proven by a fixture that leaves a committed row in the `-wal` file (a second connection holds the database open while the row is committed, the container is then gone): the dump must hold the row, which the immutable read alone would miss.
- The dump is derived; the database stays authoritative (docs/04 R-6). Stability across runs is a measured fact of this fixture (the recorded hash is in the GREEN transcripts), not a proof for every database.
