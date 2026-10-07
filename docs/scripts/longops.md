# longops (long-operation registry) - Companion Guide

| Field | Value |
|---|---|
| Revision | 4 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T04:45:00Z |
| Status | WP-08 T088/T089/T089a; WF11 review round 1 (NO-GO) remediated in revision 3; WF14 review round 2 (NO-GO, R2-1..R2-11, R2-T1..T6, R2-D1..D3) remediated in revision 4 by a STRUCTURAL round 3 (see "WF14 round 3"); the independent RE-REVIEW of revision 4 is owed (constitution 11.4.142, 11.4.276 round 3); NOT yet listed in `docs/scripts/README.md` (that index belongs to another stream: the row is owed, see "Index row owed") |
| Source | `scripts/longops/{lib,register,acquire,release,heartbeat,holder,classify,reap,check_no_build_writing_tracked,require_verdicts}.sh`; tests `scripts/longops/tests/{lib.sh,test_registry.sh,mutate_registry.sh,mutation_safety.sh,test_mutation_safety.sh,mutate_safety.sh}` |

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
| `register.sh --purpose K --owner O [--pid P] [--write-path X]... [--no-progress-s N]` | register BEFORE start and claim the purpose (`mkdir`, atomic; the op record is created EXCLUSIVELY, a lost op-id race rolls the loser's claim back). Prints the op id. `--pid` must be an integer > 1 naming a process that exists now (exit 2 otherwise). No `--no-progress-s` (or 0) records the default budget (3600 s, `LONGOPS_DEFAULT_NO_PROGRESS_S`). Exit 3 `purpose_conflict`/`op_exists`, 4 stale claim, 20 unreadable holder |
| `heartbeat.sh --op-id ID [--progress-offset N \| --sample-log] [--pid P] [--elapsed-ms N]` | liveness. Progress advances only when the offset GROWS (log size with `--sample-log`) or when the op writes its own heartbeat; a repeated flat offset is not progress. Every numeric option is validated before any write (exit 2); `--pid` rebinds pid, start time AND cmdline together (a pid > 1 that exists) |
| `release.sh --op-id ID --state complete\|failed\|reaped\|handoff\|blocked-escape [--verdict V]` | terminal state and CAS release of the claim; `--purpose K --run-id R` releases an `acquire.sh` lock. A terminal record is IMMUTABLE: another state or verdict is exit 4 `already_terminal`, the same state and verdict is an idempotent 0; `handoff` (re-adoptable) may be resolved into any terminal state |
| `acquire.sh --purpose K --run-id R` | lock without an op record. Modes `--suspend R --builds a,b`, `--update R [--callback-state S] [--state S]`, `--adopt R`, `--expire K [--op-id X]` (below) |
| `holder.sh K` | read-only: the holder as JSON while it lives (`status` `live`) or its resume window ran out (`expired`), else `none`; an unreadable holder record is exit 20 `holder_unreadable`, never `none` |
| `classify.sh [--op-id ID]` | read-only: `terminal`, `advancing`, `hung` (owner alive, offset flat past `no_progress_s`, or elapsed time past the wall cap), `dead_owner` (pid/start time no longer match `/proc`; a zombie is dead), `unreadable` (an empty, unparsable or non-numeric record: the first column is its file name, never read as dead_owner or advancing). An unknown `--op-id` is exit 4, an unknown argument exit 2 |
| `reap.sh --op-id ID [--dry-run]` / `--purpose K` | reap only on proven staleness (rules below); every decision is made UNDER the purpose lock |
| `check_no_build_writing_tracked.sh [--except-op-id ID]` | refuse a commit window (exit 1, op named) while a live registered build writes a tracked path, or any live registered op declares a write path under `$EV` or `$AUD`; an unreadable op record blocks too (its write paths are unknown) |
| `require_verdicts.sh --file F [--fingerprint FP] [--max-age-s N]` | refuse (exit 1) a missing, unparsable, non-PASS, wrong-fingerprint or stale verdict file; an absent `utc` is stale, never fresh |

## Identity and safety rules

- A process is judged by `/proc/<pid>/stat` start time and `/proc/<pid>/cmdline`, never by `pgrep` (11.4.196 D; a test greps the scripts for `pgrep`, `pkill`, `killall`). `kill -0` is only a pre-filter. A recycled pid reads `dead_owner`.
- All signals go through `lo_signal`: it refuses pid or pgid <= 1 (11.4.263), sends to ONE pid and never to a group, and writes `signals.log`. `lo_kill_child` (a needle's own sleeper) has the same guards; with the `kill -0` of `lo_alive` these are the ONLY three `kill` lines of the scope (a test asserts the set). `reap.sh` first re-resolves the cmdline (a mismatch is exit 6, nothing signalled) and, for a hung op, the labelled container (`podman ps --filter label=op_id=<id>` and `label=catalogizer.op_id=<id>`, the label `scripts/containers/run_pinned.sh` really sets), which it stops.
- A live advancing op is never reaped (exit 5). A dead-owner op is reaped with no signal at all.
- THE LOCK (WF11 class 1, WF14 R2-1/R2-8): `reap.sh` holds the purpose lock for DECISIONS and WRITES only, never across waiting or a container runtime. Step A (locked): classify, identity, re-check the owner's start time, TERM to the one pid. Wait (UNLOCKED): `timeout 30 podman stop` of the op's container, then up to `LONGOPS_REAP_GRACE_S` (default 15) for the owner to exit, so an owner whose exit path takes the same lock (the dispatch pump's TERM trap runs `release.sh --state handoff`; the runner wrapper stops its container, then releases) can finish. Step B (locked): the record is RE-DERIVED: released by the owner -> reported (exit 0, "released the op itself"), owner gone -> `reaped`, still alive -> survivor. `reap.sh --purpose` re-reads the holder under the lock and never releases the claim of a purpose that has a non-terminal op with a LIVE owner (5) or an op that cannot be judged (20). Test hooks `LONGOPS_TEST_SLEEP_BEFORE_LOCK`, `LONGOPS_LOCK_WAIT_S` (default 15), `LONGOPS_REAP_GRACE_S`.
- SURVIVOR (F1): the op is recorded `reaped` and its claim released ONLY when the process is gone after the grace period AND the owner did not release the op itself. A process that ignores TERM keeps its record (state unchanged, `reap_survived_utc` set) and its claim (the purpose keeps exactly ONE owner) and the script exits 8: an operator decision, never two live owners.
- OWNER AND CLAIM HOLDER (WF14 R2-3, R2-5): `heartbeat.sh --pid` moves the op's owner AND the purpose claim's holder record (when the claim is the op's own, run id equal) in one critical section, so a launcher that hands over to a worker and exits never leaves a dead holder beside a live op. A pid is accepted only when it is an integer > 1 that exists now, is not a zombie and has a process group > 1 (the test `lo_alive` and `lo_signal` apply).
- A CLAIM THAT BELONGS TO ANOTHER RUN (WF14 R2-10): a terminal write by `release.sh --op-id` or `reap.sh --op-id` releases the claim only when this op holds it; a claim now held by another run (the successor that re-adopted a `handoff` purpose, another op of the purpose) is left untouched and is NOT an error (the record write succeeded). `release.sh --purpose K --run-id R` keeps its compare-and-swap (4).
- FIELDS (WF14 class D): a valid-JSON record whose `pid` or `start_time` is not a non-negative integer, or whose numeric fields are not integers of at most 15 digits, is `unreadable`, never `dead_owner`. `LONGOPS_DEFAULT_NO_PROGRESS_S` and `LONGOPS_LOCK_WAIT_S` are validated at load (a positive integer).
- CONTAINER LABELS (WF14 R2-4): the container of an op is found by `op_id=<id>`, `catalogizer.op_id=<id>` AND the record's own `container_label` (a dispatched build: label `catalogizer.op_id=dispatch-<build id>`, op id `<build id>`).
- UNREADABLE (WF11 class 2): an empty, unparsable or non-numeric op record is `unreadable` (reap, heartbeat and release exit 20, `check_no_build_writing_tracked` blocks); an unparsable, unknown-kind or non-numeric holder record is `unreadable` (`holder.sh` and `reap.sh --purpose` exit 20, `register.sh`/`acquire.sh` exit 20, never `stale_claim`); an unset `CPA_APPROVED_DIR` or an unreadable `commit_push.conf` makes `reap.sh --purpose commit_push` exit 20. Nothing unreadable is ever read as clean, stale, dead or advancing.
- INPUT (WF11 class 3): `lo_wjson` refuses empty or invalid JSON (a refused write leaves the old file intact); every numeric option and every number read from a record is a non-negative integer of at most 15 digits; `--pid` (register, heartbeat, acquire) is an integer > 1 naming an existing process; `LONGOPS_NOW` is validated at load.
- A stale claim (dead holder) is refused by `register.sh`/`acquire.sh` (exit 4), never taken over silently; `reap.sh --purpose K` releases it (also a claim directory with no holder record, a crash between `mkdir` and the record).

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
| 4 | stale claim, `cas_mismatch` (`acquire.sh`, `release.sh --purpose`), unknown op, `already_terminal`, `not_expired` |
| 5 | `reap.sh` refused a live advancing op, or a live/expired holder (`--purpose`) |
| 6 | `reap.sh` could not resolve the identity from `/proc` |
| 7 | unsafe signal target (pid or pgid <= 1) |
| 8 | `reap.sh`: the process survived TERM; record and claim kept (an operator decision) |
| 20 | refusal: `helper_not_approved`, `conf_unreadable`, `holder_unreadable`, `op_record_unreadable`, `usage_error` under `--expire`, `tmpfs_state` |

## Tests and mutations

`scripts/longops/tests/test_registry.sh` (real processes, real flock, real `/proc`; scratch state only). `mutate_registry.sh` breaks one load-bearing line per mutant (M01 to M56; M43 and M44 are the round-2 reviewer mutants RM5 and RM11 verbatim) in a COPY of
the scripts and requires the test to fail. Test-only hooks: `LONGOPS_NOW`, `LONGOPS_ALLOW_TMPFS`, `LONGOPS_TEST_SLEEP_IN_CS` (widens the critical section so a missing flock is observable; `register.sh` also pauses
between the claim and the record), `LONGOPS_TEST_SLEEP_AFTER_READ`, `LONGOPS_TEST_SLEEP_BEFORE_LOCK`. HOST-SIDE run: RUNP/IMG-TESTUTIL (T007/T008) do not exist yet; container leg UNCONFIRMED.

### Mutant containment (WF11 F15, WF14 R2-T1/T2; 11.4.263): CONTAINMENT IN LAYERS, not a kernel boundary

(Corrected in revision 4: revision 3 called a real kill of pid <= 1 "structurally impossible"; the WF14 review demonstrated four bypasses and that was an overclaim. The honest statement is below.)
`tests/mutation_safety.sh`, shared by `mutate_registry.sh` and `scripts/anti-mess/tests/mutate_sweep.sh`, puts three layers between a mutant and a real effect: (1) `ms_scan` aborts a mutant tree that carries a signal, host-power, containment-tampering or container side-effect
line that is not byte-identical to a pristine line, in EVERY regular file of the mutant tree: kill, pkill, killall, skill, killpg, tgkill, sigqueue, pidfd_send_signal, send_signal, terminate, psutil, os.kill (a `.` before the token no longer hides `os.killpg(` or `.kill(`), command/builtin/exec kill, /bin/kill,
xargs kill, systemctl, loginctl, reboot, poweroff, shutdown, halt, suspend, hibernate, an absolute path into a bin directory with a glob character (`/usr/bin/k?ll`), `podman stop|kill|rm|...`, an `/usr/bin/podman` path, and any line that names or redefines the containment (`ms_*`, `MS_*`, `BASH_ENV`, `enable`, `unset -f`); a comment is a line whose FIRST non-blank character is `#`
(a trailing comment does not hide a command: reviewer mutant RM8). (2) every mutant runs under a `BASH_ENV` shim that disables the `kill` builtin, defines `kill` as a guard that refuses pid 0, 1, -1, -pgid, junk, job specs and any process group <= 1, plus `pkill`/`killall` refusals, PATH stubs for `kill`, `pkill`, `killall`,
`skill` AND `podman` (inside the containment `ps` lists nothing and every other subcommand is refused and logged: the host podman is never reached through PATH), and makes the shim functions READ-ONLY so a mutant cannot redefine the guard; every test fixture also defaults `LONGOPS_PODMAN` to a null stub (no test ever reaches the real podman). (3) the runner exits 2 when it cannot build the containment.
`tests/test_mutation_safety.sh` tests the layers by running hypothetical bad mutants (signal 0 only, a recorder in place of the kill binary, a control needle that proves the recorder can see the one allowed call); `tests/mutate_safety.sh` (MS01 to MS08) mutates the containment library itself.
HONEST LIMITS: layer 1 is textual. It does NOT catch a command name computed at run time inside an interpreter (`getattr(os, "ki"+"ll")`) or an interpreter and API that are not listed; a python process never passes the shell shim, so only layer 1 stands between it and a signal; an absolute `/usr/bin/podman` is caught by layer 1 only.
Layer 2 is a function and a set of stubs, not a kernel boundary. A STRUCTURAL boundary would be a rootless container with a private PID namespace (11.4.161/11.4.173; `unshare --user --pid` is refused on this host); until it exists the mutants of the two runners are reviewed (11.4.142) before they are added.

## T089a: the build dispatcher is bound to this registry (round c)

`scripts/build/dispatch.sh` registers every dispatched build here (the pump, before the remote start; owner `dispatch`; op id = the build id, `<id>-a<N>` on a re-adoption) and feeds it with the build's events: see
`dispatch.md` "Long-op registry". Additions to the scripts of this directory for it (additive; tests `G1` to `G3b` and `W1` to `W4` in `test_registry.sh`, mutants `M19` to `M21`):
- `register.sh --grammar build`: validates the purpose-key grammar of T005b, `build:<component>:<lane-or-target>:<snapshot digest 64 hex>:<argv digest 64 hex>:<primary|repro-cold>[:<iteration>]`, component, lane and iteration
  at most 16 characters (the key must fit the 200-character safe name); a malformed key is exit 2 `purpose_key_malformed`. Without `--grammar` a purpose keeps the older rule (any safe name), so existing callers are unchanged.
- `heartbeat.sh --elapsed-ms N`: the op's own elapsed MONOTONIC time (a build reports its build host's clock); `classify.sh` marks an op `hung` once `elapsed_ms` passes `budget.wall_clock_s` (evidence `wall_clock: ...`), so an
  advancing but over-long build is hung too. A wall cap of 0 (not recorded) never fires.
- `purposes.tsv` (new, reviewed data): `class TAB no_progress_s TAB wall_clock_s TAB basis`; every row is `UNKNOWN` until T115 measures the lane (the dispatcher then uses its defaults and records `default:UNKNOWN`).
- Registry states a build ends in: `complete` (completed and succeeded), `failed`, `reaped` (HUNG: the remote container cancelled by its label), `blocked-escape` (the other blocked reasons), `handoff` (a driver stop).

## WF11 fix round (independent review `WF11-REVIEW-longops-antimess`, NO-GO, constitution 11.4.276 classes)

Defect CLASSES named and every member closed (the finding ids are the review's): (1) a decision made outside the lock that guards the action, with no re-check: `reap.sh --op-id` (F2), `reap.sh --purpose` (F2/F3), `register.sh` op-id check (exclusive create), the sweep's reconcile actions (see `anti_mess_sweep.md`);
(2) refusal or unreadable input collapsed into clean, stale, dead or advancing: `reap.sh --purpose` (F3), unparsable op records (`classify.sh`, `check_no_build_writing_tracked.sh`, `reap.sh`, `heartbeat.sh`, `release.sh`), unparsable holder records (`holder.sh`, `lo_claim`), `classify.sh` unknown op (F14), elapsed beyond int64 (F13);
(3) unvalidated input producing an empty write or a rebinding to init / pid <= 1: `lo_wjson` (F5), `heartbeat.sh --pid/--elapsed-ms/--progress-offset` (F5/F6), `register.sh --pid` (F6), `acquire.sh --pid/--resume-ttl/--callback-state/--state` (F5/F6), a non-numeric `resume_ttl` or `LONGOPS_NOW`;
(4) terminal-state handling: a TERM-ignoring process was recorded `reaped` and its claim released (F1), a terminal record could be rewritten (F11), a stale release could reach another owner's claim; (5) the mutation runners (F15, above). F7: no op is "never hung" (default budget). F16: reviewer mutants RM1 to RM4 are mutants `M22` to `M25` and fail the suite (tests `M1` to `M3`, `X6`).
F17: the stray `kill -0 1` is gone and a test asserts the set of kill lines. Boundary of round 3, superseded in WF14: `scripts/build/dispatch.sh` passes `--container-label catalogizer.op_id=dispatch-<id>`; the registry and the sweep now read the record's own `container_label` (R2-4).

## WF14 round 3 (STRUCTURAL; constitution 11.4.276 E): ground truth first, classes, not instances

Round 2 found four important source-defects, three introduced by round-1 fixes, with ONE shared cause: a fix verified against a SIMPLIFIED model of the real composition path. The ground truth was re-derived from the real producers before any fix: `scripts/build/dispatch.sh` (the pump's TERM trap runs `release.sh --state handoff`
under the same purpose lock; `reg_adopt` registers `<id>-aN` for the same purpose and never marks the old record; the container label is `catalogizer.op_id=dispatch-<id>`), `scripts/containers/runner_lib.sh` (stops its container for `STOP_GRACE_S`, default 10 s, before the EXIT trap releases the op) and `scripts/containers/run_pinned.sh` (label `catalogizer.op_id=<op id>`).
Classes (each inventoried with a control-needled instrument, evidence in `evidence/wp08/README.md`): (A) a lock held across waiting or a slow external call (R2-1, R2-8); (B) owner identity and claim-holder consistency (R2-3, R2-5, R2-9); (C) records and fields a reader depends on that no real producer writes (R2-2, R2-4);
(D) valid JSON with one bad field read as a definite fact (RM5, RM11, RM6, RM9, the pid/start_time branch, R2-7); (E) containment of mutant side effects (R2-T1, R2-T2); (F) a record written successfully and then reported as a failure (R2-10); (G) a destructive auto-action resting on an unproven staleness predicate (R2-11).
Tests now drive the REAL producers: the real `reg_adopt` (extracted verbatim from `dispatch.sh`) in `test_sweep.sh`, the pump-shaped cooperating owner in `test_registry.sh`. UNCONFIRMED: the R2-9 window (a pid recycled between the identity check and the signal) is closed by a start-time re-check immediately before the signal but has no deterministic test.

## Not done here

The registry scripts do not signal a remote build host (a HUNG build's remote container is cancelled by the emitter by its label, not by `reap.sh`, which only knows local containers); the sweep's `build_without_registry_row`
mapping (op id equals the build id) holds for the first op of a build. `wall_clock_s` is enforced only for ops that report `--elapsed-ms`.

## Index row owed (docs/scripts/README.md)

`| [longops.md](longops.md) | scripts/longops/*.sh | T088/T089 registry |` and `| [anti_mess_sweep.md](anti_mess_sweep.md) | scripts/anti-mess/sweep.sh, catalogue.yaml | T090/T091 |`.
