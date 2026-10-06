# event_hub.sh - User Guide (T005b)

**Revision:** 1
**Last modified:** 2026-10-06T20:30:00Z
**Status:** new, uncommitted; independent review (constitution 11.4.142) owed. `$EV` is the evidence root, default `specs/001-full-project-audit-remediation/evidence`. NOT yet listed in `docs/scripts/README.md` (that index is being edited by another stream): the row is owed, see the end.

Companion guide (Helix Constitution section 11.4.18) for `scripts/build/event_hub.sh` and the event source `scripts/build/lib/evwait.py serve`. Part of T005b (owner decision C1, plan KC-P2); read `dispatch.md` first.

## What it is
ONE process per driver checkout (`.audit/builds/`), the supervisor and reconciler above the per-build pumps of `dispatch.sh`. The pumps own the event streams of their builds (one blocking reader each); the hub is woken only
by events and keeps the whole set consistent:
- restarts the pump of an open build whose pump died (`dispatch.sh resume`: the emitter streams from the last acknowledged seq, nothing is resubmitted);
- starts a queued build when a slot frees (`dispatch.sh drain`);
- runs every callback found `claimed` or `running` from its durable state (`dispatch.sh resume` on a terminal build), and every script callback not yet run (`run_callback.sh`, detached);
- evaluates build groups (`dispatch.sh group-sweep`), including the budget deadline of an unsealed group (the hub's only timer: a single `read -t` to the earliest deadline, never a poll);
- stamps `CPA_EXEC_SHA256` as `hub_sha256` into every terminal record (`terminal/state.json`) and consumed mark (third token `hub_sha256=<hex>`) it sees; a hub started without that variable writes no stamp;
- refills the kernel keyring cache of the driver secret from its file at its start (`lib/keyring.sh`).

## Commands
`event_hub.sh run` (foreground), `start` (detached unless one lives; idempotent; what `dispatch.sh submit` and `resume` call), `status` (the live record or `none`, exit 1).

## Single instance, identity, stop
- An `flock` on `builds/hub.lock` held for the hub's life (children never inherit it: every child runs with fd 9 closed); a second `run` in the same checkout waits 3 s (a hub that is exiting releases it) and is then refused:
  exit 20 `REFUSED reason=hub_already_running holder=pid=<N>`, the holder named through `scripts/longops/holder.sh` (registry purpose `build-hub:<first 16 hex of the checkout id>`, op `hub-<id12>-<pid>`).
- `builds/hub.pid` (`<pid> <start-time>`, temp-then-rename, removed at exit) and `builds/hub.json` (`{"pid":..,"start_time":..}`, the record the anti-mess sweep reads). A hub killed outright leaves them stale (they
  name a dead process); the next `start` removes them and the next hub reaps the dead holder's registry claim (`reap.sh --purpose`) before taking its own.
- TERM/INT/HUP stop it cleanly (the event source is stopped by its exact pid, the registry op is released `complete`, the files removed). It also ends when the builds root disappears.

## The event source (`evwait.py serve <builds_root>`)
One process, one blocking `select`, inotify watches armed ONCE and kept armed between events, so an event that happens while the hub reconciles is queued in the pipe and never lost (the earlier design re-armed per wait and
missed such events: found by test c10). Watched: the builds root and each `b-*/` directory (create, rename-in, delete of entries; NOT plain modification, so heartbeats appended to a journal wake nobody), each `terminal/`
and `group/<id>/` directory (any change), and a pidfd per live pump (identity proven by `/proc/<pid>/stat` start time and cmdline `_pump`): a pump that exits is an event. `pump.pid` is written temp-then-rename so the
rename is the event after which its content is complete. Idle cost: nothing starts (test c12 counts process starts during an idle window with `strace`, with a control needle).

## Callbacks table
The hub copies `callbacks.tsv` ONCE at its start to `builds/hub.callbacks.tsv` (its own export) and runs callbacks from that copy only (`RUNCB_CALLBACKS_TSV`): a row added later is refused at callback time
(`terminal/script.state` = `failed`, reason `callback_not_registered`, the script never runs) until the hub restarts (test c40). Limit: the core's inline keyed-effect callback of a consumed `completed` event runs inside
`event_core.sh consume` and cannot be governed by this export (owed request O2).

## Environment
`DISPATCH_BUILDS_ROOT`, `DISPATCH_CALLBACKS_TSV`, `DISPATCH_EVWAIT`, `DISPATCH_STATE_DIR`, `DISPATCH_LONGOPS_DIR` (as `dispatch.md`); `HUB_DISPATCH` and `HUB_RUNCB` override the dispatcher and runner paths (tests, mutation runs);
`LONGOPS_SCRIPTS` the registry scripts.

## Owed (not claimed)
- The start through `cpa-host --exec-approved` (CENTRAL C5, C10): `cpa-host` does not exist yet.
- The `systemd --user` unit: the reviewed template `scripts/build/systemd/catalogizer-build-hub@.service` (instance = the checkout id, `Restart=on-failure`, install steps in its header) exists and passes
  `systemd-analyze --user verify`, but is NOT installed or enabled, `loginctl enable-linger` was not run, and its `ExecStart` is `bash event_hub.sh run`, not `cpa-host --exec-approved` (UNCONFIRMED on a real session).
  Until it is installed the supervisor is the next `dispatch.sh submit`/`resume` call, so a killed hub with open builds and no further call is reported by the sweep as `open_builds_without_hub`, which starts nothing.
- One SSH connection per build host held by the hub (the pumps hold one each); `hub_sha256` written atomically inside the terminal record (owed request O3 on the core).

## Tests
`scripts/build/tests/test_dispatch_c.sh` cases c8 to c13 and c40; paired mutations in `mutate_dispatch_c.sh` (`m_hub_*`). Evidence `$EV/wp09/dispatch-c-*`.

## Index row owed (docs/scripts/README.md)
`| [event_hub.md](event_hub.md) | scripts/build/event_hub.sh, lib/evwait.py serve | T005b |`
