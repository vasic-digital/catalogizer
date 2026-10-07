# locked.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T03:00:00Z |
| Status | tracked; revision 3: WF13 review fix round (11.4.276 round 3, structural: defect classes C1-C6 of `scripts/register/tests/test_fix_r2.sh`); the independent re-review of this revision is owed (constitution 11.4.142) |
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

5. **Signals and the writer container** (WF10 F3, rewritten by WF13 N2/N3). The lock is kept until **no container carrying the label `catalogizer.op_id=<id>` exists**, not merely until the podman client (RUNP's last process) is gone: the container is held by conmon and keeps running when the client dies (SIGPIPE from a closed reader such as `locked.sh -- ... | head -n1`, OOM, crash). SIGTERM, SIGINT, SIGHUP, SIGUSR1, SIGUSR2, SIGALRM and SIGQUIT received while the RUNP call runs are forwarded to that one call's pid (a direct child of the wrapper, proven ours through `/proc` before every kill; never a pid <= 1, never a process group); SIGPIPE is ignored by the wrapper itself (set after the fork, so the client keeps its default). After the client is gone the wrapper polls `podman ps -a --filter label=catalogizer.op_id=<id>`; while a container exists it keeps the lock, reads the container's real exit status (`podman wait`) and uses it instead of the dead client's (the row then also records `client_exit`). The operation is journaled only after the container is gone, with `"container_drain"` = `none` (nothing to wait for), `waited`, `killed` (the container outlived `LOCKED_CONTAINER_WAIT`, default 1800 s, and was removed with `podman rm -f -t 0`; exit 137) or `unverified` (podman could not answer three times: the claim "the write finished" is not made, a warning is printed). `killed` and `unverified` keep the pending marker, so replay refuses. A signal is journaled as `"interrupted":"<SIG>"`; the wrapper exits with the writer's status (128+n when the writer itself ended 0). A SIGINT or SIGQUIT sent to a wrapper that was started as a background job of a non-interactive shell is ignored by the shell itself (POSIX) and changes nothing. The writer does not inherit fd 9 (the lock).
   **SIGKILL of the wrapper cannot be trapped** and releases the lock while the container keeps writing. It is fenced by the NEXT wrapper: after it holds the lock, a register or scratch call looks at every `.audit/register/pending/<op>` marker whose owner pid is not a live `locked.sh` and waits until no container of that op exists (refused `writer_still_running` after `LOCKED_CONTAINER_WAIT`, `writer_state_unverifiable` when podman cannot answer). The killed write itself is never journaled; its marker stays and replay refuses (`replay_pending_ops`). That residual limit (the killed writer's work is detectable but not recorded in the journal, and nothing makes its container die with the wrapper) is tracked as OWED-WP06-14.
6. **Lock claims** (WF10 F6, WF13 N4). `LOCKED_LOCK_HELD` is a proof, not a switch: its value is the pid of the ancestor `locked.sh` that holds the lock. It is honoured only when that pid is an ancestor of the calling process, fd 9 is open on this root's `docs/.register.lock`, a fresh open file description cannot take the lock (someone holds it) and `flock -n 9` succeeds (fd 9's own open file description is the holder; if another description holds it the call fails); anything else is refused `lock_claim_invalid` (20), in test mode and outside it. The production nesting (`import-sql` register -> `backup_db.sh` -> `locked.sh`) passes the proof; a variable set by hand, an fd 9 that was opened but never locked, or an fd 9 on another file does not.
7. **Journal safety** (WF10 F5). The journal must be a writable regular file (or creatable) before the writer starts: else 20 `journal_unwritable`, nothing written. A register or scratch write first creates `.audit/register/pending/<op_id>` (O_EXCL), removed only after its row is on disk (`replay.sh` refuses while one exists, `replay_pending_ops`). A journal append that fails after the write exits 21 (`REFUSED reason=journal_append_failed`) and keeps the marker, so an unjournaled write is detectable. An argument file that cannot be copied is 20 `input_capture_failed`. Rows carry `seq` (strictly increasing, assigned under the lock), `wal_bytes_before` and, only after a signal, `interrupted`.
8. **Id snapshots** (WF10 F4). The status of the snapshot call itself is used (never a pipeline's last stage). `ids_snapshot` is `ok` only when both snapshots answered, `failed` when a call failed or answered nothing (a zero-byte answer is not an empty table: sqlite always prints a row terminator), `unavailable` for a pending `-wal` file or an `--out` run. `replay.sh` refuses a planned register row whose snapshot is not `ok`.
9. **Reaper and fd 9** (WF10 F13). The commit-turn reaper runs with fd 9 closed and under `timeout -k 2` (60 s then SIGKILL after 2 s, so a reaper that ignores SIGTERM is still bounded; `LOCKED_REAPER_TIMEOUT` in test mode), so a child it leaves behind cannot hold the register lock and a hung reaper cannot hold it forever (it counts as a failing reaper: `commit_turn_held`). The snapshot calls run with fd 9 closed too.

## Exit codes

The wrapped command's status; 2 usage; 20 refused (`REFUSED reason=<code>`, including `writer_still_running` and `writer_state_unverifiable` from the fence); 21 the journal row could not be written (the write ran); RUNP's own refusals are passed through.

## Test hooks

`LOCKED_ROOT` (scratch repository root), `LOCKED_RUNP`, `LOCKED_BACKUP`, `LOCKED_REAPER_TIMEOUT`, `LOCKED_PODMAN` (the podman binary used for the container lookups), `LOCKED_CONTAINER_WAIT` (seconds), honoured only with `LOCKED_TEST_MODE=1` (else `test_hook_outside_test_mode`).

## Tests and evidence

WF13 (revision 3): `scripts/register/tests/test_fix_r2.sh` sections DRAIN, SIG, RACE, FENCE, LC, RP (stub RUNP and a stub podman: the client leaves before its container, every catchable signal, the exit status race over 120 calls, the fence for a dead predecessor, the lock-claim proof, a reaper that ignores SIGTERM) and REAL (real podman and the real `locked.sh`: SIGTERM, SIGPIPE through `| head -n1`, SIGKILL, each asserting that the lock is held until the container is gone and that the journal row is written after the container's write); transcripts `$EV/wp06/fix-r2-*.txt`.

`scripts/register/tests/test_fix_r1.sh` sections F3, F4, F5, F6, F13 (stub RUNP and fake repository trees, no container) pin the revision-2 behaviour; `scripts/register/tests/test_locked.sh` (container leg through a logging `podman` shim; see `scripts/register/tests/clib.sh`), paired mutations by `scripts/register/tests/mutate_register_ops.sh locked` (revision 2: the locked mutants are scored against `test_locked.sh`, then `test_replay.sh`, then `test_fix_r1.sh`; mutant `RMa_db_inputs_copied` survives `test_locked.sh` and is killed by `test_replay.sh` P5); evidence `$EV/wp06/locked-*`; `$EV/wp06/register-binaries.json` holds the image's sqlite3 version, the engine `validate` probe and the sha256 of both binaries.

## Honest limits

- The task text asks for a `podman` shim running the inner command inside the test container. Nested rootless podman is not available there, so the tests run on the host with a shim that logs its argv and then execs the real podman (real image, real mounts): the composed-call oracle is the same, the sqlite3 is the image's.
- The database sha256 of a row is exact only when `wal_bytes_after` is 0 (the image's last connection checkpoints on close; the row says when it did not).
- The wrapper does not make a register write atomic against a crash of the host; `backup_db.sh` is the pre-op protection.
