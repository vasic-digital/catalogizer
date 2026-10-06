# replay.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T17:30:00Z |
| Status | tracked from the WP-06 slice T067a; independent review of this revision owed (constitution 11.4.142) |
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

Skipped and listed: `command_failed`, `not_a_register_write` (`--out` and scratch rows), `no_database_change` (reads, backups), `regenerated_by_replay` (the dump and export rows), `install_replay_owed_T175a` (`export.sh --install` rows: the T175a installer does not exist yet, UNCONFIRMED).

Refused (20), `<dir>` removed: `replay_id_collision` (a local mint whose id the remote side holds for another item; the id is named, nothing is replayed, the plan owner decides, never a renumbering), `since_not_found`, `since_malformed`, `input_missing`, `out_dir_not_empty` (left untouched), `onto_not_register_database`, `replay_row_failed`, `replay_id_lost`, `replay_gate_failed`.

## Tests and evidence

`scripts/register/tests/test_replay.sh`: two scratch clones diverged from one base; the replay of the local mint and file-input edit onto the remote side passes the gate and holds the ids of both sides; the collision, unknown base, missing input and non-empty directory refusals; paired mutation: a replay that skips the mint rows loses the local ids and fails the both-sides fixture (`$EV/wp06/replay-mutation.txt`). Evidence `$EV/wp06/replay-*`.

## Honest limits

- Closures need a recorded custody chain in the database; the fixtures replay a mint and an edit, the closure leg runs in the T093 re-run with the real tools (UNCONFIRMED here).
- A replayed command that writes `/src/docs/...` other than the register database fails (docs is read-only in out mode) and aborts with `replay_row_failed`.
