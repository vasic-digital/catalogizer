# replay.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 4 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T08:00:00Z |
| Status | tracked; revision 4: WF15 review fix round (11.4.276 round 4: `scripts/register/tests/test_fix_r4.sh`, `$EV/wp06/fix-r4-convergence-assessment.md`); the independent re-review of this revision is owed (constitution 11.4.142) |
| Source | `scripts/register/replay.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The register replay for a store conflict (docs/04 section 12.2 "Single writer across clones"; docs/16 section 12.2.7). A merge conflict in the database or its derived files means a second writer broke the single-writer rule; the plan owner is asked first (constitution 11.4.66), then ST-REG re-records its local rows on top of the remote side in a scratch clone.

## Usage

```bash
scripts/register/replay.sh --onto <remote side database> --since <sha256> --out <absolute dir> [--journal <file>]
```

Run in a scratch clone checked out at the remote tip: `--onto` must be `docs/workable_items.db` of that clone; `--since` is the sha256 of the database at the merge base (the `db_sha_after` of a journal row); `--journal` defaults to `.audit/register/journal.jsonl` (inputs in the sibling `inputs/`).

## Behaviour

1. The remote side's database is copied into `<dir>/replay.db` with `backup_db.sh` (the verified online backup); `<dir>` must not exist or be empty.
2. Every journal row after the base row is replayed in order through `locked.sh --out <dir>` against the copy, the database argument `/src/docs/workable_items.db` rewritten to `/out/replay.db`, each recorded input re-bound to `/out/inputs/<sha256>`.
3. `scripts/register/gate.sh` must print `GATE OK` on the copy; the copy must hold every `reg_ids` id of both sides; `dump.sh` and `export.sh` write `register.sql` and `export/` into `<dir>`; `replay-report.json` lists the rows replayed, the rows skipped and why, the ids and the sha256 of every output.

Skipped and listed: `not_a_register_write` (`--out` and scratch rows), `regenerated_by_replay` (the dump and export rows: EXACTLY `bash -c <script>` whose script STARTS with `# register-regenerate:` and that minted no id, WF15 M6; a marker anywhere else, a marker in the arguments of another program, or a row that minted ids is a real write and is replayed or refused like any other), `install_replay_owed_T175a` (`export.sh --install` rows that changed NOTHING: `export.sh` has no `--install` option and git history never had one, the T175a installer does not exist yet, UNCONFIRMED), `no_database_change` (reads, backups) and `command_failed` (a failed row that changed NOTHING: hash equal, no ids minted, no `-wal` bytes). A failed or interrupted row that DID change the register (hash, minted ids or `-wal` bytes) is never skipped: it is refused (below).

Refused (20), `<dir>` removed: `replay_install_row_changed_register` (WF15 M6: an `export.sh --install` row that CHANGED the register cannot be skipped with an OK verdict), `replay_failed_row_changed_register` (WF13 N1: a row whose command exited non-zero but whose database changed, e.g. an UPDATE that committed before a later statement failed, or a writer interrupted after partial work; skipping it would lose its committed part and replaying it would repeat the failure; the plan owner decides, nothing is written), `replay_id_collision` (a local mint whose id the remote side holds for another item; the id is named, nothing is replayed, the plan owner decides, never a renumbering), `since_not_found`, `since_malformed`, `input_missing`, `out_dir_not_empty` (left untouched), `onto_not_register_database`, `replay_row_failed`, `replay_id_lost`, `replay_gate_failed`.

## Safety rules (revision 2)

- **Base row** (WF10 F2): only `register` rows on `docs/workable_items.db` are base candidates, and a row is the base only when its `db_sha_after` equals `--since` AND the `-wal` file was empty after it (`wal_bytes_after` 0): the main-file hash alone does not identify a state while committed pages sit in the `-wal` file, and a `scratch` or `--out` row whose database file happens to hash the same is never the base. Rows carrying `seq` must be strictly increasing (`journal_order_invalid`); a corrupt line is `journal_corrupt`; a pending marker next to the journal (a write that never reached it) is `replay_pending_ops`.
- **Ids** (WF10 F1): a planned register row whose `ids_snapshot` is not `ok` (`unavailable` under a pending `-wal`, or `failed`) is refused `replay_ids_unknown`: its minted ids are unknown, so replaying it blind could give its item the next free id of the remote side. The earlier text "never a renumbering" was false for that case (reproduced end to end with real containers: the local item silently became `CAT-004` with verdict OK). Recomputing the ids safely from the pre and post snapshots is not implemented (owed): the tool refuses and the plan owner decides.

## Tests and evidence

`scripts/register/tests/test_fix_r1.sh` (sections F1, F2: synthetic journals with stub tools, and an end-to-end run with real containers and a pending `-wal`); `scripts/register/tests/test_replay.sh`: two scratch clones diverged from one base; the replay of the local mint and file-input edit onto the remote side passes the gate and holds the ids of both sides; the collision, unknown base, missing input and non-empty directory refusals; paired mutation: a replay that skips the mint rows loses the local ids and fails the both-sides fixture (`$EV/wp06/replay-mutation.txt`). Evidence `$EV/wp06/replay-*`.

## Honest limits

- Closures need a recorded custody chain in the database; the fixtures replay a mint and an edit, the closure leg runs in the T093 re-run with the real tools (UNCONFIRMED here).
- A replayed command that writes `/src/docs/...` other than the register database fails (docs is read-only in out mode) and aborts with `replay_row_failed`.
