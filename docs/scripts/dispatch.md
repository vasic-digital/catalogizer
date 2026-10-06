# dispatch.sh, remote/emit.sh, lib/evwait.py: User Guide (T005b slice)

**Revision:** 2
**Last modified:** 2026-10-06T20:30:00Z
**Status:** revision 2 (round c) is uncommitted work on top of the committed revision 1; independent review (constitution 11.4.142) owed for both. `$EV` is the evidence root, default `specs/001-full-project-audit-remediation/evidence`.

Companion guide (Helix Constitution section 11.4.18) for `scripts/build/dispatch.sh`, `scripts/build/remote/emit.sh`, `scripts/build/lib/evwait.py` and
`scripts/build/callbacks.tsv`. It is a SLICE of T005b (owner decision C1, plan KC-P2): the event-driven dispatcher on top of the already tracked consume core
`scripts/build/event_core.sh` (see `event_core.md`), which it REUSES and never duplicates: verification, replay guard, ordering, the exactly-once terminal claim
by rename of a prepared directory and the keyed-effect callback are the core's. Round c added (each has its own guide): the event hub `event_hub.sh` (`event_hub.md`), the callback runner `run_callback.sh` (`run_callback.md`), the git-plumbing snapshot, the input closure,
the build-host tree cache and the keyring cache (`build_snapshot.md`: `lib/snapshot.py`, `lib/treecache.py`, `lib/keyring.sh`), build groups, the emitter restart, host-key pinning, `peak_rss_bytes`, the
`--progress` option and the binding of every build to the long-op registry (this page). What is still owed is listed at the end.

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
- `--wallclock-cap` (default 3600 s) and `--no-progress-budget` (default 60 s) bound the build; `--heartbeat` is the emitter's period (default 2 s). A quiet build is no longer
  `build_progress_flat` by default: see `--progress` (round c) above.
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

## Round c: snapshot, groups, registry binding, pairing (options and commands added to `submit`)
- **Git source** (`--src` holds a `.git`): `--component <path>` is REQUIRED (the input closure is the unit that ships; refused `usage` without it). The source-snapshot digest of the purpose key is then
  the manifest digest of `lib/snapshot.py` (see `build_snapshot.md`): the sha256 over `(path, tree id)` of the root and every initialised submodule at every depth, each tree written by git plumbing from that
  repository's own temporary index (the real index is only read; no `git add`, no filter, no attribute; `$EV` and `$AUD` excluded from every compile-class snapshot). `--containerfile F` (repeatable) and
  `--context D` declare an image build's Containerfile and context so its `COPY`/`ADD` sources join the closure; `--snapshot-mode tic|cpa` and `--declare PATH` (repeatable) select the CPA form (seeded from HEAD,
  exactly the declared paths loaded; an undeclared dirty file inside the component's closure is refused `undeclared_dirty_input`). Refusals (20, before anything is created): `submodule_uninitialised`,
  `closure_incomplete`, `undeclared_dirty_input`. Only the closure ships, as objects the build host's cache lacks (`transfer.objects` and `transfer.bytes` in the submit record: an unchanged second submit sends 0, one
  changed source file sends its blob and the trees above it); the build host recomputes every shipped tree; a mismatch ends the build `infra_failed` with detail `snapshot_mismatch` (never `blocked-unavailable`),
  callback once, no container started.
- **Budgets**: `--no-progress-budget` and `--wallclock-cap` default to the row of `scripts/longops/purposes.tsv` matching `build:<component>:<lane>` (a `UNKNOWN` value falls back to 60 s and 3600 s; `budget_source` in the
  submit record names the source: `cli`, `purposes.tsv:<class>`, `default:UNKNOWN`, `default`). `--progress log|log+cpu` (default `log+cpu`): `log` reads progress from the build log only, so a quiet build ends
  `build_progress_flat`; `log+cpu` adds the build container's real cgroup CPU time, so a quiet build that works is not hung (a wedged one still is: its CPU time is flat).
- **Purpose key and identity**: `build:<component>:<lane>:<snapshot digest>:<argv digest>:<variant>[:<iteration>]` with component, lane and iteration at most 16 characters; the registered key is rebuilt from its parts, so
  every part decides identity: two lanes, two iterations, the two variants never share a build. A submit whose FULL key equals a running build's attaches to it; a purpose held in the registry by anything else is
  refused `purpose_conflict`. `dispatch.sh pair <build_id>` prints the partner of a build for the double build (T443, T447a, T566): every field but the variant matches, the iteration is ignored, and a `repro-cold` build
  pairs with the `primary` record whose build completed and succeeded (the deliverable), not a failed iteration.
- **Build groups**: `submit --group G [--cancel-on-fail] [--group-budget S]` (the `--callback` then names the GROUP callback, registered at the first submit; a later submit naming another is refused
  `group_conflict`; a member's own callback is the record-only no-op); `dispatch.sh group-seal G` closes the membership (a later submit: `group_sealed`; after an early end: `group_ended`); `group-status G`; `group-sweep`
  (the hub's call); `group-next-expiry`. A group is terminal once SEALED and every member is terminal, claimed by the rename of a prepared `builds/group/<G>/terminal` (mode 0700 parent): one group callback, exactly once.
  A member ending `blocked-unavailable`, `cancelled` or `infra_failed` (with `--cancel-on-fail` also `test_failed` / `build_failed`) ends the group early once: the open members are marked `cancelled_by_group` (a file in
  their directory) and cancelled, which never triggers anything and never sets the kind. The kind is decided by precedence over the other members in member order, never by arrival: `test_failed`/`build_failed` (the first
  such member), else `infra_failed`, else `blocked-unavailable`, else `cancelled`, else `completed`; `terminal/members.json` records every member's kind (`succeeded`, `test_failed`, `build_failed`, `infra_failed`,
  `blocked-unavailable`, `cancelled`, `cancelled_by_group`), the trigger and its reason. A group not sealed within its budget (or sealed with no member) is made terminal `cancelled` (`orphaned: true`), its members
  cancelled by the group; the sweep (hub) does that on the budget's deadline.
- **Long-op registry (T089a, constitution 11.4.232)**: every build is a registered long-op. The PUMP registers it (`scripts/longops/register.sh --grammar build`, owner `dispatch`, the pump as the live owner, op id = the
  build id, `<id>-a<N>` on a re-adoption) BEFORE the remote start; each consumed heartbeat feeds `scripts/longops/heartbeat.sh` (progress = `progress_offset + stage`, `--elapsed-ms` = the build host's own monotonic time) and
  `scripts/longops/classify.sh` is the one owner of the HUNG decision (flat progress past `no_progress_s`: `build_progress_flat`; the host's elapsed time past the cap: `build_wallclock_exceeded`; silence:
  `build_liveness_lost`), after which the remote container is cancelled by its label and the op ends `reaped`. Terminal mapping: completed+succeeded `complete`; other completed, `infra_failed`, `cancelled` `failed`;
  HUNG `reaped`; the other `blocked-unavailable` reasons `blocked-escape`; the verdict is the reason. A driver stop (TERM on the pump) hands the op over (`handoff`, claim released, the remote build left running);
  `resume` (and the hub) re-adopt it as a new op owned by the new pump, nothing resubmitted; a pump killed outright leaves a dead-owner op that the re-adoption reaps (no signal, proven from `/proc`). The op ends just after the
  callback runs (the registry release follows the terminal claim and the callback; a reader waits for the state it needs). The registry sits next to the builds root (`<audit>/longops`); `DISPATCH_LONGOPS_DIR` overrides it.
- **Hub**: `submit` and `resume` start the hub (`event_hub.md`) when none runs: default on for the real builds root (`<checkout>/.audit/builds`), off for a fixture root unless `DISPATCH_HUB=1`. One pump per build is enforced
  by a lock (`.pumplock`) held for the pump's life. With a live hub the callback of a script row is run by the hub, from its own export of the callbacks table; without one, by the terminal path directly.
- **Host-key pinning**: a host is trusted ONLY by the fingerprint pinned for it in `build/hosts.env` (`BUILD_HOST_<n>_FP`): its keys are scanned (`ssh-keyscan`), the one whose fingerprint equals the pin goes into a
  known-hosts file of its own (`<state>/known_hosts/<hash>`, re-verified on every use) and ssh runs with `StrictHostKeyChecking=yes`, `UserKnownHostsFile=` that file and `GlobalKnownHostsFile=/dev/null`. A host with no pin
  (`host_key_not_pinned`) or whose scanned keys do not match (`host_key_mismatch`) is not qualified and never connected to; no ssh call ever carries `StrictHostKeyChecking=no` or `accept-new`. The test ssh shim
  (`DISPATCH_SSH`) bypasses pinning unless `DISPATCH_PIN=1` (then `DISPATCH_KEYSCAN` stands in for ssh-keyscan: the shim's key). UNCONFIRMED against a real host: T100 has recorded none.

## Liveness (11.4.232(C)): proven, never inferred from a living process
- Silence on the stream past the no-progress budget (+1 s, so the registry's integer-second clock is past it): `build_liveness_lost`. Heartbeats whose progress (`progress_offset`, the remote log's byte length plus, with
  `--progress log+cpu`, the container's CPU ms, kept monotone; and `stage`, its line count) do not advance for the budget: `build_progress_flat`. `elapsed_monotonic_ms` past the cap: `build_wallclock_exceeded`.
  The three are decided by the long-op registry (see above), cancel the remote build and give a verdict that is never a pass.
- The progress view is updated only from events the core CONSUMED (an unauthenticated event moves nothing). The elapsed time is the emitter's own monotonic clock (`/proc/uptime`), never the
  difference between the driver's clock and `sent_at`.
- After a pump restart the view is rebuilt from the resent events only; the pump's own downtime is never counted.

## Environment (all optional)
`DISPATCH_BUILDS_ROOT` (default `<checkout>/.audit/builds`), `DISPATCH_STATE_DIR` (default `${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/<sha256 of the checkout path>`; the driver secret lives
here, outside the checkout, and `secret-init` refuses a path inside it), `DISPATCH_HOSTS_FILE` (default `build/hosts.env`, format `build/hosts.env.example`, T137a), `DISPATCH_CALLBACKS_TSV`
(default `scripts/build/callbacks.tsv`), `DISPATCH_TRANSPORT` (`ssh` default | `local`), `DISPATCH_ALLOW_LOCAL=1` (the declared test/proof exception without which `local` is refused:
owner decision C1 removed the local-build option), `DISPATCH_SSH` (the ssh command, a shim in tests), `DISPATCH_JOBS`, `DISPATCH_EMIT_DIR`, `DISPATCH_EVWAIT`, `DISPATCH_REMOTE_RUNP` (path of
`run_pinned.sh` on the build host), `DISPATCH_RECONNECTS` (default 3) and `DISPATCH_RECONNECT_DELAY` (default 1 s) for a stream that ends without a `completed`, `DISPATCH_DISK_OUT`. Round c: `DISPATCH_HUB` (1/0), `DISPATCH_HUB_SCRIPT`, `DISPATCH_LONGOPS_DIR`, `DISPATCH_PURPOSES_TSV`, `DISPATCH_GROUP_BUDGET` (default of `--group-budget`, 3600 s),
`DISPATCH_SNAPSHOT_EXCLUDES` (default the evidence and audit trees), `DISPATCH_PIN`, `DISPATCH_KEYSCAN`; for the emitter `EMIT_CGROUP_ROOT` (default `/sys/fs/cgroup`).

## Hosts and shipping (ssh transport)
Hosts come only from `build/hosts.env` (`BUILD_HOST_<n>=user@addr`, tried in order, at submit time only). A host is qualified when `ssh <host> true` succeeds and
`podman info --format {{.Host.Security.Rootless}}` is `true` (`runtime_not_rootless` otherwise); every attempt is recorded in `submit.json` `host_attempts`. None left: the build ends
`blocked-unavailable` `no_qualified_host`, never a local build. The emitter (`remote/emit.sh` with `lib/evwait.py`) is shipped to `~/.cache/catalogizer/emit/<sha256>/` and its sha256 recorded in
the submit record; the source tree goes to the content-addressed cache `~/.cache/catalogizer/trees/<snapshot digest>/` and is not sent again when present. SSH host-key pinning from
`docs/infrastructure/build-hosts.md` (T100) is OWED (the real ssh path uses `BatchMode` and the `BUILD_SSH_IDENTITY` key file; `StrictHostKeyChecking` pinning arrives with T100).

## The emitter (revision 2)
The emitter now keeps NO signature at rest: the journal holds unsigned event bodies and an event is signed when it is streamed, with the key the driver sends on stdin at every `run` and `attach`; without a valid key the
emitter refuses to send (exit 3). The build runs as a background group that records `build.pid` and, when it ends, `build.rc`; when `run` finds a journal with no `completed` line and no live daemon (the emitter died), an ADOPT
daemon with the re-sent key finishes the job (heartbeats, then `completed`) without starting the build again; `cancel` stops the container and the build group directly when no daemon lives. `completed` carries
`peak_rss_bytes`: the build container's cgroup `memory.peak` as last sampled at a heartbeat (a lower bound at heartbeat granularity: the cgroup goes with the container), omitted when never read.

## The emitter (revision 1 text, still accurate where not superseded above)
`emit.sh run|attach|fetch|cancel` (see its header). `run` is idempotent: with a journal or a live daemon present it only streams. The per-build key K = HKDF-SHA256(driver secret, info =
len4be(build_id) || build_id || len4be(run_id) || run_id) is derived by the driver (`event_core.sh derive-key`), sent on the emitter's STDIN only, held in a shell variable by the daemon, never
written, never on a command line. Events are signed in the canonical form of `contracts/build-event.schema.json` by an inline python signer independent of `bev_crypto.py`
(cross-implementation: the core verifies them). The artifact tree of a build is `out/artifacts/`; its content address is the sha256 of the sorted `sha256  path` lines, carried as
`artifact_manifest_sha256` and `image_digest` (`sha256:<hex>`). `peak_rss_bytes` is owed (T115).

## Files written (per build, under `$DISPATCH_BUILDS_ROOT/<build_id>/`)
`submit.json` (written before the remote start; `started` flips to true as the pump begins; round c adds `transfer`, `component`, `closure_repos`, `budget_source`, `progress`, `group`), `op_id` (the registry op of the build), `.pumplock`, `script.log`, `callback.json` (a script row adds `script_effect_key`: the core keeps its own bookkeeping key `cbcore-...`), the core's `events.jsonl`, `consumed/<seq>`, `terminal/`, `effects/`, `pump.pid`, `pump.log`
(one line per consumed or refused event, no secret), `artifacts/` (brought back and verified), `queued`. On the build host (local transport: `<build_id>/remote/`): `journal.jsonl`, `seq`, `daemon.pid`,
`build.log`, `build.pid`, `build.rc`, `t0`, `po.max`, `peak_rss`, `out/artifacts/`.

## Security
The driver secret, the derived keys and the HMAC values of refused events are in no file, log or evidence (test s21 scans every file the run wrote, with a planted needle proving the scan sees). A `completed`
event is authenticated before any bring-back is attempted, so a forged one can neither start a transfer nor end the build (test s14b). The tail never ends on a `completed` line while the daemon lives.

## Tests and evidence
Revision 1: `scripts/build/tests/test_dispatch.sh` (fake emitter + ssh shim; `ONLY="s6 s9"` runs a subset), `test_dispatch_e2e.sh` (a real Go build in IMG-GO through `run_pinned.sh`), `mutate_dispatch.sh`.
Round c: `test_snapshot.sh` (real git repositories: root, submodules, a nested one, an uninitialised one; cases p1 to p9), `test_dispatch_c.sh` (cases c1 to c40: git shipping, callback runner, hub, groups, registry binding,
the real emitter with a stub of `run_pinned.sh` and a podman/cgroup shim, pinning, keyring), `test_dispatch_c_e2e.sh` (REAL containers: a Go module with a `replace` into a submodule built in IMG-GO from a shipped
closure; a quiet build with the real cgroup CPU counter and a real `peak_rss_bytes`), `mutate_dispatch_c.sh` (paired mutations). The core's cases stay in `test_dispatch_events.sh`.
Evidence: `$EV/wp09/dispatch-b-*` (revision 1), `$EV/wp09/dispatch-c-*` and `$EV/wp08/dispatch-registry-*` (round c).

## Owed (not claimed)
- Start of the hub, the runner and the dispatcher through `cpa-host --exec-approved` and the approved callbacks table / `cpa_code.txt` adoption (CENTRAL C5, C10; T046/T047 do not exist): the hub is started by
  `event_hub.sh start`; the `systemd --user` unit template `scripts/build/systemd/catalogizer-build-hub@.service` is written and statically verified but not installed or enabled.
- One hub SSH connection per build host (each pump holds its own); the hub is the supervisor and reconciler above the per-build pumps (restart of a dead pump, queue drain, callbacks, groups, stamps).
- `verify_artifact.sh` as a separate script and the image bring-back by `podman save` / `load` by digest (the artifact tree verification is inline in `dispatch.sh`); the `T001` disk-headroom precondition before a load.
- Owed requests on `event_core.sh` (not modified here): O1 a `reason` field in the terminal record (the reason of a non-completed terminal is still read from `digest`); O2 a callback-executor hook, so that one
  callback state serves a script row (today `terminal/script.state` is a second, separately recorded step) and the hub's callbacks table also governs the core's inline keyed-effect callback; O3 `hub_sha256` written
  atomically inside the terminal record (today the hub stamps it into `state.json` and the consumed marks after the rename).
- The anti-mess sweep reads `builds/hub.json` (written here) and skips a `groups` directory (the group directory here is `group`, per the task): align one of them.
- Heartbeat granularity of `peak_rss_bytes`; a real build host (T100, T006a BLOCKED-ON ODG-07): everything over `ssh` is shim-tested, the real ssh path is UNCONFIRMED; `signing_key_not_provisioned` (T446); transfer budget
  time measurement (the byte and object counts are measured and asserted, the time budget is not); the T121b CPA integration.
