# run_callback.sh - User Guide (T005b)

**Revision:** 1
**Last modified:** 2026-10-06T20:30:00Z
**Status:** new, uncommitted; independent review (constitution 11.4.142) owed. NOT yet listed in `docs/scripts/README.md` (owed row below).

Companion guide (Helix Constitution section 11.4.18) for `scripts/build/run_callback.sh`. Part of T005b; read `dispatch.md` first.

## What it does
`run_callback.sh <builds_root> <build_id|group/<group_id>> [--retry]` runs the registered callback of a TERMINAL build or build group, exactly once in effect, from durable state.
The callback id comes from `callback.json`; the row comes from the closed registry `scripts/build/callbacks.tsv` (id, script, effect-key prefix, long-op purpose, no-progress budget). The table the runner reads is
`RUNCB_CALLBACKS_TSV` (the hub's own export), else `DISPATCH_CALLBACKS_TSV`, else the table next to the script. An id the table lacks is refused at callback time: `terminal/script.state` = `failed`, `script.reason` =
`callback_not_registered`, the script never runs.
- Script `-`: the keyed-effect callback of `event_core.sh`; found `claimed` or `running` it is resumed through `event_core.sh resume-callback`.
- A script (a path relative to `RUNCB_SCRIPT_ROOT`, default the checkout; absolute paths and `..` refused): run as `bash <script>` under `timeout <budget>s` with `CB_KIND` (the terminal kind), `CB_EXIT_CLASS`, `CB_REASON`
  (the reason of a `blocked-unavailable` or `infra_failed` terminal), `CB_BUILD_DIR`, `CB_EFFECT_KEY` and `CB_ARGS_FILE` (the callback arguments, JSON, as data; never a shell line). The script must write its effect under
  `<dir>/effects/<CB_EFFECT_KEY>` by atomic rename and treat an existing effect as done: the runner re-runs a callback found `running` (the runner crashed), so every script is idempotent through that key. Exit 0 with the
  effect present: `done`; exit 0 without it: `failed effect_not_applied`; a non-zero exit (or the timeout, 124): `failed exit_<code>`, never retried silently (`--retry` re-runs a failed one on explicit request).

## Durable state (inside `terminal/`, each file temp-then-rename)
`script.state` (`running`, `done`, `failed`), `script.reason`, `script.runner` (`<pid> <start-time>` while it works: a runner whose pid and start time no longer match `/proc` left a callback found `running`, which the next call
re-runs), `callback.runner_sha256` (the value of `CPA_EXEC_SHA256` when the runner was started with it, nothing otherwise: CENTRAL C10). A per-directory lock (`.scriptlock`) serialises runners.

## Why a second state next to `callback.state`
`event_core.sh consume` runs its own keyed-effect callback inline for a consumed `completed` event and records `callback.state` `done`; the script phase is a second, separately recorded step. For a script row the core's
bookkeeping key is `cbcore-<cb>-<id>` and the script's key `cb-<cb>-<id>` (`script_effect_key` in `callback.json`); `dispatch.sh wait` waits for both. Owed request O2: a callback-executor hook in the core so that one state serves.

## Who starts it
`dispatch.sh` after every terminal path (completed, cancel, HUNG, host loss, secret loss, bring-back mismatch, resume), detached, unless a live hub exists, in which case the hub runs it from its own table export.

## Tests
`test_dispatch_c.sh` cases c5 to c7 and c40; paired mutations `m_run_*`. 

## Index row owed (docs/scripts/README.md)
`| [run_callback.md](run_callback.md) | scripts/build/run_callback.sh | T005b |`
