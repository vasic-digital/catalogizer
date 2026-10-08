# anti-mess sweep and invariant catalogue - Companion Guide

| Field | Value |
|---|---|
| Revision | 4 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T19:00:00Z |
| Status | WP-08 T090/T091; round 5 (constitution 11.4.276, R_max 5, STRUCTURAL): the round-4 reviews are remediated by one record shape and one lineage reader shared with `scripts/longops/lib.sh`, a driver that proves it produced one row per invariant, actions bound to the catalogue and escaped findings; see "Round 5". NOT yet listed in `docs/scripts/README.md` (row owed, see `docs/scripts/longops.md`) |
| Source | `scripts/anti-mess/sweep.sh`, `scripts/anti-mess/catalogue.yaml`; tests `scripts/anti-mess/tests/{test_sweep.sh,test_sweep_r5.sh,sweep_lib.sh,mutate_sweep.sh}` (containment: `scripts/longops/tests/mutation_safety.sh`); baseline `$EV/wp08/sweep-baseline.json` |

## Purpose

The level-triggered control plane of constitution 11.4.233 at project scale (docs/12 section 17, docs/16 section 13.3): it RE-DERIVES the actual persistent state and diffs it
against the declared invariant catalogue, before gated transitions (CPA S0 and S7, build start, tag) and on a cadence. It detects; it reconciles only on `--reconcile` and only the
catalogued auto-safe classes; it never builds, commits, merges or pushes (11.4.233 F). It contacts a remote ONLY in INV-9 (`git ls-remote` to prove a held commit is on every reachable remote tip of main, before a finished commit-push run directory is reported removable or removed); every other invariant reads the local state only.

## Usage

```bash
scripts/anti-mess/sweep.sh [--stage S0|S7|cadence] [--paths-from FILE] [--repo PATH] [--reconcile] [--json OUT] [--only AM-R1,AM-R2,...]
```

| Option | Effect |
|---|---|
| `--stage` | `S0`: AM-R1 excludes the declared change set; a blocking `core.hooksPath` is refused with 20. `S7`: nothing excluded. `cadence` (default): every invariant. An invariant outside the stage is `skipped_stage` |
| `--paths-from FILE` | the declared change set (S0 only): one path per line relative to the main repository root, optionally TAB and a verdict path |
| `--repo PATH` | AM-R1 scoped to that repository (and below) |
| `--reconcile` | repair catalogued `auto-safe` classes only (stale git lock, dead-owner registry row, container of a TERMINAL op (an orphan container is reported, never stopped), leftover build `.tmp-<pid>-<start>` directory, uninitialised submodule, finished CPA run beyond both retention bounds with every held commit on every reachable remote tip of main). Every action re-verifies its precondition AT ACTION TIME (rmlock: no holder, no git activity, still old enough; stopcontainer: the container's op class is re-derived from the current registry; rmdir `build_tmp`: the owner is still gone; rmdir `finished_run`: report.json still finished and every held commit still on a remote tip; initsub: still uninitialised; reapop: `reap.sh` re-classifies under the purpose lock); operator-gated classes are never touched |
| `--json OUT` | report `anti-mess-sweep/1` (written by temp-then-rename) |

Exits: 0 no drift; 10 drift reported; 11 a source could not be read and no drift was found (unread: never clean); 20 refusal (usage, an unknown `--only` id, a BLIND detector, a blocking `core.hooksPath` at S0). Env: `ANTIMESS_ROOT`, `LONGOPS_*`, `ANTIMESS_OWNED_ORGS`
(fixtures), `ANTIMESS_LOCK_MIN_AGE` (60 s), `ANTIMESS_ORPHAN_AGE_S` (300 s), `CPA_APPROVED_DIR`. Test hook `ANTIMESS_TEST_BEFORE_ACTION` (a script run before every reconcile action; honoured only with `ANTIMESS_TEST_MODE=1`, else exit 20) moves the state between detection and action.

## Guards, not scripts that print "clean" (11.4.201)

Before a detector is trusted on the real state it runs on a seeded fixture (the control needle: the seeded drift MUST be reported) and on a golden-false-with-carrier fixture
(the carrier MUST NOT be reported). A detector that fails either is `blind`: the sweep exits 20 instead of printing a clean report. A catalogued invariant without a detector is
`not_evaluated` with its reason, never `clean`. A source that cannot be read (`verify_repos.sh` gave no report, podman failed) is `unread` (exit 11 when nothing else drifted), never clean: AM-P1 and AM-P2 report an unparsable, empty or non-numeric op record as an `unread` finding `corrupt_op_record` and still evaluate every readable record (each file is parsed on its own); AM-P1 reports a failing `podman ps` as `unread` `containers_unread`. When an invariant has both drift and unread findings its status is `drift` and the unread findings stay in the report.

## Catalogue (`catalogue.yaml`, schema `anti-mess-catalogue/1`)

| Id (alias) | Detector | Reconcile | Notes |
|---|---|---|---|
| AM-R1 (INV-2) | `det_AM_R1` | operator-gated | `scripts/repo/verify_repos.sh --no-remote`: dirty minus `dirty_excepted` (`exceptions.tsv`); S0 change set; a drift equal to `.audit/pending_pins.tsv` is a pending pin move |
| AM-R2 (INV-1) | `det_AM_R2` | auto-safe | `*.lock` under the git dir (submodule gitdirs included): stale only when no process holds it open (`/proc/*/fd`), no git process runs inside the repository and it is older than the minimum age |
| AM-R5 | `det_AM_R5` | auto-safe | `git submodule status --recursive` leading `-` |
| AM-G2 | `det_AM_G2` | operator-gated | workflow half: `scripts/repo/check_no_ci.sh` (cadence); `core.hooksPath` half: unset or no executable client hook in the directory (S0, S7, cadence) |
| AM-P1 (INV-3) | `det_AM_P1` | auto-safe | registry rows: `hung_op`, `registry_row_dead_owner`; containers `label=project=catalogizer` matched on the label the launcher sets, `catalogizer.op_id` (scripts/containers/run_pinned.sh; the older `op_id` label is accepted too): `orphan_container`, `container_of_terminal_op`, `container_without_op_label` (neither label); the container of a `handoff` op is informational (`container_of_handoff_op`) and is never stopped (re-adoptable, 11.4.232 D) |
| AM-P2 (INV-4) | `det_AM_P2` | operator-gated | two non-terminal ops with one `purpose_key` unless attached or superseded |
| AM-P4 | `det_AM_P4` | operator-gated | purpose claims and handoffs (WF11 F9): `stale_claim` (dead holder, the purpose is blocked), `claim_without_holder` (older than the minimum age; younger is `claim_young`, a registration in progress), `claim_unreadable`, a holder that cannot be judged (`unread`), `handoff_unadopted` (a `handoff` op nothing supersedes, adopts or resolves). Reported only: release is `reap.sh --purpose` / `release.sh` |
| AM-P3 | `det_AM_P3` | auto-safe | leftover `.tmp-<pid>-<start>` dirs; `open_builds_without_hub`; `build_without_registry_row` (UNCONFIRMED: hub record `hub.json` {pid,start_time} and the op_id/build_id mapping are guesses until T005b exists) |
| INV-9 | `det_INV_9` | auto-safe | `.audit/commit-push/<run>/`: `interrupted_run`, `interrupted_merge` (remediation of T042 S0, only when the process of `merge.json` is gone), `live_merge_holder`, `suspended_run`, `ready_to_resume`, `ready_to_resume_expired` (reported, never released), `removable_finished_run`, `kept_held_commit`. The `commit_push` holder is read through `holder.sh` with the run's `CPA_APPROVED_DIR`, else `cpa-host --exec-approved`, else reported unread (`project_not_trusted` before T046a) |
| AM-R3, AM-R4 (INV-5), AM-G1, AM-S1, INV-6, INV-7, INV-8 | none | | `not_evaluated`, each with its reason (network, WP-G1, WP-06 wiring, T006, T005b); tracked gaps, not claimed covered |

UNCONFIRMED: the retention keys (`retain_runs`, `retain_days`) and `merge.json` fields (`repo`, `pid`, `start_time`) follow the T042 text; the files do not exist yet. The run-directory age is the
mtime of `report.json`.

## WF11 fix round (independent review NO-GO; constitution 11.4.276 classes)

(1) Decisions outside the guarding lock or without a re-check: every reconcile action re-verifies at action time (see the `--reconcile` row); `reapop` delegates to `reap.sh`, which decides under the purpose lock; `--only` ids are validated.
(2) Unreadable input read as clean: corrupt op records no longer blind AM-P1/AM-P2 (F4); an unreadable podman is `unread` (F8); stale claims, holderless claims, unreadable holders and un-adopted handoffs are now catalogued as AM-P4 (F9); a sweep with only unread sources exits 11 (F8); an unknown or lower-case `--only` id exits 20 (F12).
(3) Handoff (F10): a `handoff` op is re-adoptable, so its container is never stopped. (4) Label: containers are matched on `catalogizer.op_id` (the label the launcher really sets), so a `run_pinned.sh` container of a live op is no longer `container_without_op_label`, which had made TIC refuse.
F18: the `not_evaluated_reason` of INV-7 and INV-8 now states the facts (check_pins.sh and dispatch.sh exist; no detector reads them yet). Honest boundary: `rmdir` `build_tmp` re-verification (the owner is still gone) cannot be varied by a test (the pid is part of the directory name); its other conditions are tested.

## WF14 round 3 (structural; constitution 11.4.276 E)

(1) A container is stopped by `--reconcile` ONLY when its op is in THIS checkout's registry and terminal: the registry is per checkout and the podman namespace is per user, so a container whose op is absent here may belong to another checkout, track or scratch copy. `orphan_container` is still reported (drift) and never stopped
(R2-11: absence from one registry is not proof of staleness, 11.4.232 E). (2) A container is matched to its op by the op id OR the record's own `container_label` (a dispatched build: label `catalogizer.op_id=dispatch-<build id>`, op id `<build id>`; R2-4). (3) AM-P4 `handoff_unadopted`: a handoff op is adopted when a LATER op of the same
`purpose_key` exists (what `scripts/build/dispatch.sh` `reg_adopt` really writes: `<id>-aN`), or when `superseded_by`/`attached_to`/`adopted_by` is set (accepted, but no producer writes them); the old record is no longer left as permanent drift after every driver restart (R2-2). (4) [superseded in round 5 by the ONE record shape of `scripts/longops/lib.sh` (`LO_OP_SHAPE`): types, canonical integers and the closed state set are all validated] A record is readable only when `op_id`, `purpose_key` and `state` are strings;
valid JSON with a missing or mistyped field is `unread`, never skipped (RM6); an op whose state cannot be read is never `terminal` (its container is never stopped, RM9); a record whose `pid` or `start_time` is not an integer is `unreadable`, never reaped as `dead_owner`. (5) INV-9: a `commit_push` holder that could not be read makes a run awaiting its remote checks `unread`
(`suspended_run_holder_unread`), never drift `suspended_run_without_live_holder` (R2-7). (6) `podman ps` and `podman stop` run under `timeout $LO_PODMAN_TIMEOUT` (default 30 s, round 5; a wedged runtime is `unread`, exit 11, never a hang). Tests: `RA*` (the REAL `reg_adopt`), `LB*`, `FC*`, `BF*`, `HU*` in `test_sweep.sh`; mutants S33 to S40.

## Round 5 (constitution 11.4.276, STRUCTURAL; R_max 5)

Convergence assessment: the round-4 findings had four shared causes. (1) The sweep kept its own, weaker idea of a readable op record, so the registry and the sweep disagreed on the same file: the sweep now reads ops/ through `lo_ops_snapshot` and judges every record with `LO_OP_SHAPE` (a shape failure is `unread corrupt_op_record`, for AM-P1, AM-P2, AM-P4 and the lineage map alike). (2) The label-to-op match was a per-container scan; it is now ONE lineage map (`lo_lineage_build` / `lo_lineage_class`: a dispatched build `<id>`, `<id>-a2`, ... is one lineage, the NUMERICALLY latest attempt decides, a label is matched as `<id>`, `catalogizer.op_id=<id>`, `op_id=<id>` or the record's bare `container_label`, one label in two lineages or any unreadable record makes the class `unreadable`, never `terminal`). (3) The driver could finish having evaluated fewer invariants than it selected, and a detector that died mid-way looked clean: every detector now runs in a subshell with a sentinel file, an abnormal end is `blind`, the report must hold exactly ONE result row per selected invariant or the sweep exits 20 `driver_incomplete`, and the exit status and the report come from the state AFTER the reconcile (at most 3 passes, an action is never repeated; the report carries `detected` and the final status). (4) Actions were strings in the findings stream: they are now BOUND to the catalogue (`actions:` of each invariant; an action the invariant does not list is `refused_action_not_of_invariant`), every field of the stream escapes backslash, TAB, LF and CR (a lock file named with a newline cannot forge a row), a name must pass `lo_safe_name` or the plain-path check before an action is attached, and the dead-owner reap is passed `--expect dead_owner` so a row that became live between detection and action is `class_changed`, not signalled.

Also: a HUNG op (live owner) is reported with NO action and `--reconcile` never reaps it (C5); the container of a `handoff` op is never stopped; the containers of an op whose owner is dead are resolved by `reap.sh` with the full container proof; every podman call is timed. Test hooks (honoured only with `ANTIMESS_TEST_MODE=1`, else exit 20 `test_hook_outside_test_mode`): `ANTIMESS_CATALOGUE`, `ANTIMESS_TEST_DETECTORS`, `ANTIMESS_TEST_BEFORE_ACTION`. Tests `test_sweep_r5.sh` (sections B, C, A, E, HI, J; `S5_SECTIONS` selects) drive the real registry scripts, a stateful JSON podman stub with label filtering and the real `reg_adopt`; scale probe J1: the HEAD scan of 400 records spawned 7232 jq/python processes in 73.9 s, now 56 in 2.8 s.

## Tests and mutations

`test_sweep.sh`: real git repositories, processes and `/proc`; stand-ins only for `podman` and `cpa-host` (unit level). `mutate_sweep.sh` (round 5: S01 to S40 kept, thirteen PORTED (S18 retargeted to `_unread_op_record`: its former target `_emit_unread_ops` was dead after the rewrite, three callers at 350372a8 and none now, and was removed) to the new code with their intent unchanged; Y1 to Y3 and C08 to C10 are the round-4 reviewers' mutants (Y1, Y3, C10 ported to the lineage code in `lib.sh`, `ML` variants that mutate a copy of the longops scripts); NM-* is one mutant per guard the round adds; `--check` proves every pattern still applies; each suite run is bounded by `timeout ${MUT_TIMEOUT_S:-900}`, `TIMEOUT` is its own result; the containment refuses a mutant that edits a `podman stop` line, so there is no NM mutant for the stop timeout, which the J/E sections cover) -- originally forty one-line mutants of the sweep (S01 to S40; S17 is the round-1 reviewer's RMS1, S33 and S34 are the round-2 reviewer mutants RM6 and RM9), each must fail the test, run inside the containment of `scripts/longops/tests/mutation_safety.sh` (a mutant that adds or edits a signal line aborts; the kill builtin is disabled and `kill` is a guard function; see `longops.md`).
HOST-SIDE run (RUNP/IMG-TESTUTIL absent, container leg UNCONFIRMED).
