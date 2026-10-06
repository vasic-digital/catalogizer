# longops (long-operation registry) - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:30:00Z |
| Status | new, uncommitted when written (WP-08 T088/T089); independent review owed (constitution 11.4.142); NOT yet listed in `docs/scripts/README.md` (that index belongs to another stream: the row is owed, see "Index row owed") |
| Source | `scripts/longops/{lib,register,acquire,release,heartbeat,holder,classify,reap,check_no_build_writing_tracked,require_verdicts}.sh`; tests `scripts/longops/tests/{lib.sh,test_registry.sh,mutate_registry.sh}` |

## Purpose

The single source of truth for what long operation is running, who owns it, its state and its PROVEN liveness (constitution 11.4.232 A to E;
docs/16 section 13; docs/12 section 17.4). Every wrapper that starts a container build, test, scan, QA run or dispatched agent registers
before it starts and ends in a terminal state. A registry that is only consulted is not enough: the sweep (`docs/scripts/anti_mess_sweep.md`) re-derives
the real state and diffs it against these rows.

## State layout

`$LONGOPS_DIR` (default `<repo>/.audit/longops`, ignored by `.gitignore`, durable, REFUSED on tmpfs unless a test fixture sets `LONGOPS_ALLOW_TMPFS=1`):

| Path | Content |
|---|---|
| `ops/<op_id>.json` | one record per operation: `op_id`, `purpose_key`, `owner`, `run_id`, `pid`, `start_time` (ticks, `/proc/<pid>/stat` field 22), `cmdline` (real, from `/proc`), `container_label`, `state`, `heartbeat_seq`, `progress_offset`, `last_progress_epoch`, `budget{no_progress_s,wall_clock_s,memory_bytes,cpus}`, `write_paths[]`, `verdict`, `evidence_path` |
| `claims/<purpose>/holder.json` | the atomic `mkdir` claim of a purpose and its holder record (`kind` `process` or `suspended-run`, `run_id`, `pid`, `start_time`, `state`, ...) |
| `<purpose>.lock` | the flock file of every compare-and-swap of that purpose |
| `events.jsonl` | append-only event log |
| `signals.log` | every signal the registry ever sent (the audit trail; a test asserts it stays empty when no signal was due) |

Every write is temp-then-rename in the same directory with an fsync. States: `registered`, `running`, `complete`, `failed`, `reaped`, `handoff`, `blocked-escape`.

## Scripts

| Script | Use |
|---|---|
| `register.sh --purpose K --owner O [--pid P] [--write-path X]... [--no-progress-s N]` | register BEFORE start and claim the purpose (`mkdir`, atomic). Prints the op id. Exit 3 `purpose_conflict` (live holder), 4 stale claim |
| `heartbeat.sh --op-id ID [--progress-offset N \| --sample-log]` | liveness. Progress advances only when the offset GROWS (log size with `--sample-log`) or when the op writes its own heartbeat; a repeated flat offset is not progress |
| `release.sh --op-id ID --state complete\|failed\|reaped\|handoff\|blocked-escape [--verdict V]` | terminal state and CAS release of the claim; `--purpose K --run-id R` releases an `acquire.sh` lock |
| `acquire.sh --purpose K --run-id R` | lock without an op record. Modes `--suspend R --builds a,b`, `--update R [--callback-state S] [--state S]`, `--adopt R`, `--expire K [--op-id X]` (below) |
| `holder.sh K` | read-only: the holder as JSON while it lives (`status` `live`) or its resume window ran out (`expired`), else `none` |
| `classify.sh [--op-id ID]` | read-only: `terminal`, `advancing`, `hung` (owner alive, offset flat past `no_progress_s`), `dead_owner` (pid/start time no longer match `/proc`) |
| `reap.sh --op-id ID [--dry-run]` / `--purpose K` | reap only on proven staleness (rules below) |
| `check_no_build_writing_tracked.sh [--except-op-id ID]` | refuse a commit window (exit 1, op named) while a live registered build writes a tracked path, or any live registered op declares a write path under `$EV` or `$AUD` |
| `require_verdicts.sh --file F [--fingerprint FP] [--max-age-s N]` | refuse (exit 1) a missing, unparsable, non-PASS, wrong-fingerprint or stale verdict file; an absent `utc` is stale, never fresh |

## Identity and safety rules

- A process is judged by `/proc/<pid>/stat` start time and `/proc/<pid>/cmdline`, never by `pgrep` (11.4.196 D; a test greps the scripts for `pgrep`, `pkill`, `killall`). `kill -0` is only a pre-filter. A recycled pid reads `dead_owner`.
- All signals go through `lo_signal`: it refuses pid or pgid <= 1 (11.4.263), sends to ONE pid and never to a group, and writes `signals.log`. `reap.sh` first re-resolves the cmdline (a mismatch is exit 6, nothing signalled) and, for a hung op, the labelled container (`podman ps --filter label=op_id=<id>`), which it stops.
- A live advancing op is never reaped (exit 5). A dead-owner op is reaped with no signal at all.
- A stale claim (dead holder) is refused by `register.sh`/`acquire.sh` (exit 4), never taken over silently; `reap.sh --purpose K` releases it.

## suspended-run, adopt, expire (T088, T121b, CENTRAL C2)

A CPA run that exits 16 while its builds live hands the lock to a holder of kind `suspended-run` (`acquire.sh --suspend R --builds ids`), live while any member
build has no `terminal/` directory under `.audit/builds`, or the group callback is `claimed`/`running`, or the run is `ready_to_resume` within `resume_ttl`
(`--update R --callback-state done --state ready_to_resume` is one swap). `--adopt R` swaps it back to a process holder (pid and start time of `--pid`, default the caller's
parent). Every transition is a compare-and-swap on the expected prior holder (run id, state) under ONE flock on `<purpose>.lock`; the record is replaced by rename, so a
reader never sees `none` across the swaps (the reader re-reads the record after judging it and retries on change). `--adopt` racing `--expire` has exactly one winner.
`holder.sh` reports `ready_to_resume` past `resume_ttl` as `expired` and releases nothing; `--expire K` releases it and writes `resume_expired.json` into `$CPA_RUN` when
`CPA_RUN_ID` and `CPA_RUN` are set, else under `.audit/out/<op_id>/` (`--op-id` required, exit 20 `usage_error` without it); never into the expired run's directory.
For purpose `commit_push` both `holder.sh` and `--expire` need `CPA_APPROVED_DIR` (exit 20 `helper_not_approved` first) and read `resume_ttl` from
`$CPA_APPROVED_DIR/scripts/repo/commit_push.conf` only when a `ready_to_resume` holder exists (lazy). UNCONFIRMED: the key syntax of `commit_push.conf` (T042 does not exist
yet); the scripts read `resume_ttl=<seconds>` (also `:`), and a missing key is exit 20 `conf_unreadable`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | done / no live writer / verdicts pass |
| 1 | `check_no_build_writing_tracked` blocked; `require_verdicts` refused |
| 2 | usage |
| 3 | `purpose_conflict` (a live or expired holder, or an op id already registered) |
| 4 | stale claim, `cas_mismatch`, unknown or terminal op, `not_expired` |
| 5 | `reap.sh` refused a live advancing op |
| 6 | `reap.sh` could not resolve the identity from `/proc` |
| 7 | unsafe signal target (pid or pgid <= 1) |
| 20 | refusal: `helper_not_approved`, `conf_unreadable`, `usage_error` under `--expire`, `tmpfs_state` |

## Tests and mutations

`scripts/longops/tests/test_registry.sh` (real processes, real flock, real `/proc`; scratch state only). `mutate_registry.sh` breaks one load-bearing line per mutant in a COPY of
the scripts and requires the test to fail; the signal-guard mutant also removes the real `kill` in the same mutant (a guard-less `kill -- -1` would hit the whole session). Test-only hooks:
`LONGOPS_NOW`, `LONGOPS_ALLOW_TMPFS`, `LONGOPS_TEST_SLEEP_IN_CS` (widens the critical section so a missing flock is observable). HOST-SIDE run: RUNP/IMG-TESTUTIL (T007/T008)
do not exist yet; container leg UNCONFIRMED.

## T089a: the build dispatcher is bound to this registry (round c)

`scripts/build/dispatch.sh` registers every dispatched build here (the pump, before the remote start; owner `dispatch`; op id = the build id, `<id>-a<N>` on a re-adoption) and feeds it with the build's events: see
`dispatch.md` "Long-op registry". Additions to the scripts of this directory for it (additive; tests `G1` to `G3b` and `W1` to `W4` in `test_registry.sh`, mutants `M19` to `M21`):
- `register.sh --grammar build`: validates the purpose-key grammar of T005b, `build:<component>:<lane-or-target>:<snapshot digest 64 hex>:<argv digest 64 hex>:<primary|repro-cold>[:<iteration>]`, component, lane and iteration
  at most 16 characters (the key must fit the 200-character safe name); a malformed key is exit 2 `purpose_key_malformed`. Without `--grammar` a purpose keeps the older rule (any safe name), so existing callers are unchanged.
- `heartbeat.sh --elapsed-ms N`: the op's own elapsed MONOTONIC time (a build reports its build host's clock); `classify.sh` marks an op `hung` once `elapsed_ms` passes `budget.wall_clock_s` (evidence `wall_clock: ...`), so an
  advancing but over-long build is hung too. A wall cap of 0 (not recorded) never fires.
- `purposes.tsv` (new, reviewed data): `class TAB no_progress_s TAB wall_clock_s TAB basis`; every row is `UNKNOWN` until T115 measures the lane (the dispatcher then uses its defaults and records `default:UNKNOWN`).
- Registry states a build ends in: `complete` (completed and succeeded), `failed`, `reaped` (HUNG: the remote container cancelled by its label), `blocked-escape` (the other blocked reasons), `handoff` (a driver stop).

## Not done here

The registry scripts do not signal a remote build host (a HUNG build's remote container is cancelled by the emitter by its label, not by `reap.sh`, which only knows local containers); the sweep's `build_without_registry_row`
mapping (op id equals the build id) holds for the first op of a build. `wall_clock_s` is enforced only for ops that report `--elapsed-ms`.

## Index row owed (docs/scripts/README.md)

`| [longops.md](longops.md) | scripts/longops/*.sh | T088/T089 registry |` and `| [anti_mess_sweep.md](anti_mess_sweep.md) | scripts/anti-mess/sweep.sh, catalogue.yaml | T090/T091 |`.
