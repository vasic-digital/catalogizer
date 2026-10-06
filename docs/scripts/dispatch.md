# dispatch.sh, remote/emit.sh, lib/evwait.py: User Guide (T005b slice)

**Revision:** 1
**Last modified:** 2026-10-06T17:30:00Z
**Status:** new, untracked until the owner commits; independent review (constitution 11.4.142) owed. `$EV` is the evidence root, default `specs/001-full-project-audit-remediation/evidence`.

Companion guide (Helix Constitution section 11.4.18) for `scripts/build/dispatch.sh`, `scripts/build/remote/emit.sh`, `scripts/build/lib/evwait.py` and
`scripts/build/callbacks.tsv`. It is a SLICE of T005b (owner decision C1, plan KC-P2): the event-driven dispatcher on top of the already tracked consume core
`scripts/build/event_core.sh` (see `event_core.md`), which it REUSES and never duplicates: verification, replay guard, ordering, the exactly-once terminal claim
by rename of a prepared directory and the keyed-effect callback are the core's. The event hub (`event_hub.sh`, one process per checkout), the callback runner
(`run_callback.sh`), the git-plumbing source snapshot and input closure, build groups, the systemd unit and the CPA/`cpa-host` adoption are OWED (list below).

## Design in one paragraph
`submit` validates, takes the disk gate, picks a qualified build host, ships the emitter and the source tree, writes the submit record, starts a detached per-build
**pump** and returns the `build_id` at once. The pump starts the **emitter** on the build host (over `ssh`, or locally under the declared test/proof exception) and
reads its event stream with a blocking `read -t` (no polling). The emitter wraps the build in a rootless, digest-pinned container through
`scripts/containers/run_pinned.sh`, appends signed `build-event/1` events (`accepted`, `heartbeat`, `completed`) to a journal on the build host and streams them.
Each event goes to `event_core.sh consume`; a `completed` event whose `exit_class` is `succeeded` is first authenticated, then the artifact tree is brought back and
its content address verified, and only then consumed. Every terminal path (completed, cancel, liveness, host loss, secret loss, bring-back mismatch) resolves through
the core's single rename, so the callback runs exactly once. Waiting is event-driven: `evwait.py` blocks on inotify (and on `pidfd` for process death).

## Commands
`dispatch.sh snapshot <dir>` prints the content digest of a source tree (sha256 over the sorted `sha256  path` lines of every regular file, `.git` excluded).
`dispatch.sh argv-digest <argv...>` prints the sha256 over the NUL-joined argv (the environment list is empty in this slice).

`dispatch.sh submit --purpose KEY --callback ID --image IMG-ID --src DIR [--variant primary|repro-cold] [--need BYTES] [--wallclock-cap S] [--no-progress-budget S]
[--heartbeat S] [--network-none] [--callback-args JSON] -- <build argv...>` prints the `build_id` and returns without waiting.
- `KEY` = `build:<component>:<lane>:<snapshot-digest>:<argv-digest>:<variant>[:<iteration>]`, digests 64 lowercase hex. The snapshot and argv digests are RECOMPUTED and must
  equal the key's (`purpose_digest_mismatch`), so two different trees or argvs never share a key. `iteration` is a fresh id for each run of a repeated measurement.
- build id = `b-` + the first 24 hex of sha256(KEY). **Single owner per purpose (11.4.232(B))**: an open build of the same key is attached to; a succeeded build whose brought-back
  artifact still matches its event is REUSED (build once, nothing is started); a terminal build that did not succeed is refused (`purpose_already_terminal`, submit a fresh iteration).
- `--wallclock-cap` (default 3600 s) and `--no-progress-budget` (default 60 s) bound the build; `--heartbeat` is the emitter's period (default 2 s). A build that prints nothing for longer
  than its no-progress budget is `build_progress_flat` by design: a quiet build (a bare `go build`) must be given a larger budget or a verbose flag (`go build -v`).
- Refusals exit 20 with `REFUSED reason=`: `purpose_malformed`, `purpose_digest_mismatch`, `callback_not_registered`, `local_transport_forbidden`, `no_qualified_host`, `dependency_missing`,
  `usage`, `secret_unusable`, `state_write_failed`, `purpose_already_terminal`, and the disk gate's reason (`disk_below_headroom`, ...). Nothing is left behind by a refusal (no build directory, no pump).
- Order: validation, callback registry, digests, build-once check, **disk gate** (`scripts/containers/disk_headroom.sh`, before anything is created), host selection, secret, directory, submit record, pump.
  The 60% memory ceiling and the process headroom are enforced per container by `run_pinned.sh` (section 12.6, 12.12).
- Concurrency: `DISPATCH_JOBS` (default 1) builds run at once; an over-budget submit is QUEUED (file `queued`, status `queued`), never started, and the pump of a finishing build starts the oldest queued one.

`dispatch.sh status <build_id>` prints JSON: `state` (`open|queued|terminal`), `terminal` (`kind`, `exit_class`, `reason`), `callback_state`, `consumed` seqs, `pump_alive`.
Terminal kinds: `completed` (with an `exit_class`: `succeeded`, `build_failed`, `test_failed`, `infra_failed`, `cancelled`), `cancelled`, `infra_failed` (reason detail `artifact_mismatch`),
`blocked-unavailable` with one reason of the closed set `host_unreachable`, `no_qualified_host`, `build_liveness_lost`, `build_progress_flat`, `build_wallclock_exceeded`, `driver_secret_lost`,
`artifact_unavailable` (and `signing_key_not_provisioned`, T446, owed). The reason of a non-`completed` terminal is stored in `terminal/state.json` field `digest` (the core has no reason field: owed request O1).

`dispatch.sh wait <build_id> [S]` blocks on inotify until the callback is `done` (exit 0) or `failed` (exit 1); exit 3 and the line `timeout` after `S` seconds; unknown build 20.
`dispatch.sh resume <build_id>` restarts the pump of an open build (the emitter streams from the last consumed seq, nothing is resubmitted), or, for a terminal build, re-runs a callback found
`claimed|running` and drains the journal (a late `completed` is recorded `late_ignored`, the verdict never changes).
`dispatch.sh cancel <build_id>` sends the remote cancel (the container is stopped by its label, the daemon by exact pid), claims `cancelled` through the core, and stops the pump (exact pid, proven ours
by start time and `/proc/<pid>/cmdline`, never a process-group signal).

## Liveness (11.4.232(C)): proven, never inferred from a living process
- Silence on the stream past the no-progress budget: `build_liveness_lost`. Heartbeats whose `progress_offset` (the remote log's byte length) and `stage` (its line count) do not advance for the
  budget: `build_progress_flat`. `elapsed_monotonic_ms` past the cap: `build_wallclock_exceeded`. All three cancel the remote build and give a verdict that is never a pass.
- The progress view is updated only from events the core CONSUMED (an unauthenticated event moves nothing). The elapsed time is the emitter's own monotonic clock (`/proc/uptime`), never the
  difference between the driver's clock and `sent_at`.
- After a pump restart the view is rebuilt from the resent events only; the pump's own downtime is never counted.

## Environment (all optional)
`DISPATCH_BUILDS_ROOT` (default `<checkout>/.audit/builds`), `DISPATCH_STATE_DIR` (default `${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/<sha256 of the checkout path>`; the driver secret lives
here, outside the checkout, and `secret-init` refuses a path inside it), `DISPATCH_HOSTS_FILE` (default `build/hosts.env`, format `build/hosts.env.example`, T137a), `DISPATCH_CALLBACKS_TSV`
(default `scripts/build/callbacks.tsv`), `DISPATCH_TRANSPORT` (`ssh` default | `local`), `DISPATCH_ALLOW_LOCAL=1` (the declared test/proof exception without which `local` is refused:
owner decision C1 removed the local-build option), `DISPATCH_SSH` (the ssh command, a shim in tests), `DISPATCH_JOBS`, `DISPATCH_EMIT_DIR`, `DISPATCH_EVWAIT`, `DISPATCH_REMOTE_RUNP` (path of
`run_pinned.sh` on the build host), `DISPATCH_RECONNECTS` (default 3) and `DISPATCH_RECONNECT_DELAY` (default 1 s) for a stream that ends without a `completed`, `DISPATCH_DISK_OUT`.

## Hosts and shipping (ssh transport)
Hosts come only from `build/hosts.env` (`BUILD_HOST_<n>=user@addr`, tried in order, at submit time only). A host is qualified when `ssh <host> true` succeeds and
`podman info --format {{.Host.Security.Rootless}}` is `true` (`runtime_not_rootless` otherwise); every attempt is recorded in `submit.json` `host_attempts`. None left: the build ends
`blocked-unavailable` `no_qualified_host`, never a local build. The emitter (`remote/emit.sh` with `lib/evwait.py`) is shipped to `~/.cache/catalogizer/emit/<sha256>/` and its sha256 recorded in
the submit record; the source tree goes to the content-addressed cache `~/.cache/catalogizer/trees/<snapshot digest>/` and is not sent again when present. SSH host-key pinning from
`docs/infrastructure/build-hosts.md` (T100) is OWED (the real ssh path uses `BatchMode` and the `BUILD_SSH_IDENTITY` key file; `StrictHostKeyChecking` pinning arrives with T100).

## The emitter
`emit.sh run|attach|fetch|cancel` (see its header). `run` is idempotent: with a journal or a live daemon present it only streams. The per-build key K = HKDF-SHA256(driver secret, info =
len4be(build_id) || build_id || len4be(run_id) || run_id) is derived by the driver (`event_core.sh derive-key`), sent on the emitter's STDIN only, held in a shell variable by the daemon, never
written, never on a command line. Events are signed in the canonical form of `contracts/build-event.schema.json` by an inline python signer independent of `bev_crypto.py`
(cross-implementation: the core verifies them). The artifact tree of a build is `out/artifacts/`; its content address is the sha256 of the sorted `sha256  path` lines, carried as
`artifact_manifest_sha256` and `image_digest` (`sha256:<hex>`). `peak_rss_bytes` is owed (T115).

## Files written (per build, under `$DISPATCH_BUILDS_ROOT/<build_id>/`)
`submit.json` (written before the remote start; `started` flips to true as the pump begins), `callback.json`, the core's `events.jsonl`, `consumed/<seq>`, `terminal/`, `effects/`, `pump.pid`, `pump.log`
(one line per consumed or refused event, no secret), `artifacts/` (brought back and verified), `queued`. On the build host (local transport: `<build_id>/remote/`): `journal.jsonl`, `seq`, `daemon.pid`,
`build.log`, `out/artifacts/`.

## Security
The driver secret, the derived keys and the HMAC values of refused events are in no file, log or evidence (test s21 scans every file the run wrote, with a planted needle proving the scan sees). A `completed`
event is authenticated before any bring-back is attempted, so a forged one can neither start a transfer nor end the build (test s14b). The tail never ends on a `completed` line while the daemon lives.

## Tests and evidence
`scripts/build/tests/test_dispatch.sh` (fake emitter + ssh shim; `ONLY="s6 s9"` runs a subset), `test_dispatch_e2e.sh` (a real Go build in IMG-GO through `run_pinned.sh`, a failing build, a real container cancelled by label),
`mutate_dispatch.sh` (paired mutants, `list`, `all`). The core's cases stay in `test_dispatch_events.sh`. Evidence: `$EV/wp09/dispatch-b-*`.

## Owed (not claimed)
`event_hub.sh` (one hub per checkout, `hub.pid`, supervisor unit) and `run_callback.sh` (a script-running callback; the slice runs the core's keyed-effect callback, `callbacks.tsv` script column `-`);
the git-plumbing snapshot with temporary index, input closure, `submodule_uninitialised`, `closure_incomplete`, `snapshot_mismatch` and the remote tree recompute (the slice digests the given directory);
build groups and `--cancel-on-fail`; the `cpa-host --exec-approved` start path and the approved callbacks table; emitter restart re-signing (i2); `longops/heartbeat.sh` as the HUNG owner; keyring refill;
host-key pinning; `peak_rss_bytes`; a `reason` field in the core's terminal record (O1); heartbeat-less hub downtime handling beyond the per-build pump; transfer budget measurement (w).
