# anti-mess sweep and invariant catalogue - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T16:30:00Z |
| Status | new, uncommitted when written (WP-08 T090/T091); independent review owed (constitution 11.4.142); NOT yet listed in `docs/scripts/README.md` (row owed, see `docs/scripts/longops.md`) |
| Source | `scripts/anti-mess/sweep.sh`, `scripts/anti-mess/catalogue.yaml`; tests `scripts/anti-mess/tests/{test_sweep.sh,mutate_sweep.sh}`; baseline `$EV/wp08/sweep-baseline.json` |

## Purpose

The level-triggered control plane of constitution 11.4.233 at project scale (docs/12 section 17, docs/16 section 13.3): it RE-DERIVES the actual persistent state and diffs it
against the declared invariant catalogue, before gated transitions (CPA S0 and S7, build start, tag) and on a cadence. It detects; it reconciles only on `--reconcile` and only the
catalogued auto-safe classes; it never builds, commits, merges or pushes (11.4.233 F) and contacts no remote.

## Usage

```bash
scripts/anti-mess/sweep.sh [--stage S0|S7|cadence] [--paths-from FILE] [--repo PATH] [--reconcile] [--json OUT] [--only AM-R1,AM-R2,...]
```

| Option | Effect |
|---|---|
| `--stage` | `S0`: AM-R1 excludes the declared change set; a blocking `core.hooksPath` is refused with 20. `S7`: nothing excluded. `cadence` (default): every invariant. An invariant outside the stage is `skipped_stage` |
| `--paths-from FILE` | the declared change set (S0 only): one path per line relative to the main repository root, optionally TAB and a verdict path |
| `--repo PATH` | AM-R1 scoped to that repository (and below) |
| `--reconcile` | repair catalogued `auto-safe` classes only (stale git lock, dead-owner registry row, orphan or terminal-op container, leftover build `.tmp-<pid>-<start>` directory, uninitialised submodule, finished CPA run beyond both retention bounds with every held commit on every reachable remote tip of main). Every action re-verifies its precondition; operator-gated classes are never touched |
| `--json OUT` | report `anti-mess-sweep/1` (written by temp-then-rename) |

Exits: 0 no drift; 10 drift reported; 20 refusal (usage, a BLIND detector, a blocking `core.hooksPath` at S0). Env: `ANTIMESS_ROOT`, `LONGOPS_*`, `ANTIMESS_OWNED_ORGS`
(fixtures), `ANTIMESS_LOCK_MIN_AGE` (60 s), `ANTIMESS_ORPHAN_AGE_S` (300 s), `CPA_APPROVED_DIR`.

## Guards, not scripts that print "clean" (11.4.201)

Before a detector is trusted on the real state it runs on a seeded fixture (the control needle: the seeded drift MUST be reported) and on a golden-false-with-carrier fixture
(the carrier MUST NOT be reported). A detector that fails either is `blind`: the sweep exits 20 instead of printing a clean report. A catalogued invariant without a detector is
`not_evaluated` with its reason, never `clean`. A source that cannot be read (`verify_repos.sh` gave no report, podman failed) is `unread` / an informational finding, never clean.

## Catalogue (`catalogue.yaml`, schema `anti-mess-catalogue/1`)

| Id (alias) | Detector | Reconcile | Notes |
|---|---|---|---|
| AM-R1 (INV-2) | `det_AM_R1` | operator-gated | `scripts/repo/verify_repos.sh --no-remote`: dirty minus `dirty_excepted` (`exceptions.tsv`); S0 change set; a drift equal to `.audit/pending_pins.tsv` is a pending pin move |
| AM-R2 (INV-1) | `det_AM_R2` | auto-safe | `*.lock` under the git dir (submodule gitdirs included): stale only when no process holds it open (`/proc/*/fd`), no git process runs inside the repository and it is older than the minimum age |
| AM-R5 | `det_AM_R5` | auto-safe | `git submodule status --recursive` leading `-` |
| AM-G2 | `det_AM_G2` | operator-gated | workflow half: `scripts/repo/check_no_ci.sh` (cadence); `core.hooksPath` half: unset or no executable client hook in the directory (S0, S7, cadence) |
| AM-P1 (INV-3) | `det_AM_P1` | auto-safe | registry rows: `hung_op`, `registry_row_dead_owner`; containers `label=project=catalogizer`: `orphan_container`, `container_of_terminal_op`, `container_without_op_label` |
| AM-P2 (INV-4) | `det_AM_P2` | operator-gated | two non-terminal ops with one `purpose_key` unless attached or superseded |
| AM-P3 | `det_AM_P3` | auto-safe | leftover `.tmp-<pid>-<start>` dirs; `open_builds_without_hub`; `build_without_registry_row` (UNCONFIRMED: hub record `hub.json` {pid,start_time} and the op_id/build_id mapping are guesses until T005b exists) |
| INV-9 | `det_INV_9` | auto-safe | `.audit/commit-push/<run>/`: `interrupted_run`, `interrupted_merge` (remediation of T042 S0, only when the process of `merge.json` is gone), `live_merge_holder`, `suspended_run`, `ready_to_resume`, `ready_to_resume_expired` (reported, never released), `removable_finished_run`, `kept_held_commit`. The `commit_push` holder is read through `holder.sh` with the run's `CPA_APPROVED_DIR`, else `cpa-host --exec-approved`, else reported unread (`project_not_trusted` before T046a) |
| AM-R3, AM-R4 (INV-5), AM-G1, AM-S1, INV-6, INV-7, INV-8 | none | | `not_evaluated`, each with its reason (network, WP-G1, WP-06 wiring, T006, T005b); tracked gaps, not claimed covered |

UNCONFIRMED: the retention keys (`retain_runs`, `retain_days`) and `merge.json` fields (`repo`, `pid`, `start_time`) follow the T042 text; the files do not exist yet. The run-directory age is the
mtime of `report.json`.

## Tests and mutations

`test_sweep.sh`: real git repositories, processes and `/proc`; stand-ins only for `podman` and `cpa-host` (unit level). `mutate_sweep.sh`: sixteen one-line mutants of the sweep, each must fail the test.
HOST-SIDE run (RUNP/IMG-TESTUTIL absent, container leg UNCONFIRMED).
