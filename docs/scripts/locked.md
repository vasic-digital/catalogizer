# locked.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T17:30:00Z |
| Status | tracked from the WP-06 slice T064; the scripts/register tests run in the container leg; independent review of this revision owed (constitution 11.4.142) |
| Source | `scripts/register/locked.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The single-writer wrapper of the register (docs/04 section 12.2). Every write to `docs/workable_items.db` goes through it: it takes the host lock `docs/.register.lock` (never tracked, T004) around exactly one `scripts/containers/run_pinned.sh` (RUNP) call and runs the command inside `IMG-TESTUTIL`, so the register is written by the image's `sqlite3` and the engine binary, never by a host `sqlite3`. It is a host control-plane launcher like RUNP.

## Usage

```bash
scripts/register/locked.sh [--out <dir>] [--op-id <id>] -- <command word>...
scripts/register/locked.sh [--op-id <id>] import-sql <file.sql under .audit/out/>     # T165; LOCKED_SCRATCH_DB selects a scratch import
```

| Form | Composed call |
|---|---|
| default | `RUNP --rw docs --op-id <id> IMG-TESTUTIL -- <cmd>` (docs writable at `/src/docs`, output in `.audit/out/<op_id>/` as `/out`) |
| `--out <dir>` | `RUNP --out <dir> IMG-TESTUTIL -- <cmd>`: no `--rw docs`, the command can write only `/out` |
| scratch `import-sql` | `RUNP --rw .audit/scratch ...` and never `--rw docs` |

## Behaviour

1. **Commit-turn freeze** (register writes only). `.audit/commit_turn.json` naming a `run_id` other than `EVREC_TURN_RUN_ID` is first reaped once through `"${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}" --exec-approved scripts/release/commit_turn_check.sh --reap`, re-read, and if still held (or unreadable, or not JSON) the call is refused `commit_turn_held` (20): no podman call, no journal row. Before the owner approves the T580e GO the reaper is refused `release_seam_unreleased`, which is harmless.
2. **One RUNP call** under `flock`. The exit status of the wrapped command is passed through. RUNP passes no stdin: stdin redirection into `locked.sh` is unsupported (the usage text says so; a fixture pipes SQL and sees no row written).
3. **Journal** `.audit/register/journal.jsonl` (ignored): one JSON row per command with `time`, `op_id`, `mode` (`register`, `out`, `scratch`), `argv`, `exit`, `db`, `db_sha_before`, `db_sha_after`, `wal_bytes_after`, `ids_snapshot`, `ids_minted` (the `reg_ids` difference, read in the image through the immutable URI; skipped for `--out` runs, which cannot write docs) and `inputs` / `input_args`: every argument that names an existing file is copied to `.audit/register/inputs/<sha256>` (a path relative to the repository root, `/src/<p>`, or `.read <path>`; databases are never copied). This journal is the input of `scripts/register/replay.sh` (T067a).
4. **import-sql** checks that the sha256 of the file equals the bare hash in its sibling `<stem>.sha256` (`import_sha256_mismatch` / `import_sha256_malformed`), takes the pre-op backup first through `scripts/register/backup_db.sh --record` for a register import (the lock is inherited through `LOCKED_LOCK_HELD`) and none for a scratch import, and validates `LOCKED_SCRATCH_DB` (`scratch_db_path_invalid`, `scratch_db_subcommand_invalid`).

## Exit codes

The wrapped command's status; 2 usage; 20 refused (`REFUSED reason=<code>`); RUNP's own refusals are passed through.

## Test hooks

`LOCKED_ROOT` (scratch repository root), `LOCKED_RUNP`, `LOCKED_BACKUP`, honoured only with `LOCKED_TEST_MODE=1` (else `test_hook_outside_test_mode`).

## Tests and evidence

`scripts/register/tests/test_locked.sh` (container leg through a logging `podman` shim; see `scripts/register/tests/clib.sh`), paired mutations by `scripts/register/tests/mutate_register_ops.sh locked`; evidence `$EV/wp06/locked-*`; `$EV/wp06/register-binaries.json` holds the image's sqlite3 version, the engine `validate` probe and the sha256 of both binaries.

## Honest limits

- The task text asks for a `podman` shim running the inner command inside the test container. Nested rootless podman is not available there, so the tests run on the host with a shim that logs its argv and then execs the real podman (real image, real mounts): the composed-call oracle is the same, the sqlite3 is the image's.
- The database sha256 of a row is exact only when `wal_bytes_after` is 0 (the image's last connection checkpoints on close; the row says when it did not).
- The wrapper does not make a register write atomic against a crash of the host; `backup_db.sh` is the pre-op protection.
