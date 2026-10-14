# sweep_leaks.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 4 |
| Created | 2026-10-07 |
| Last modified | 2026-10-12T13:55:13Z |
| Status | WF18 fix round 6 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/test-infra/sweep_leaks.sh`; test `tests/infra/test_sweep_leaks.sh` |

## Purpose

Removes what earlier runs leaked and no lifecycle script finds again: empty unlabelled podman-compose pods `pod_catalogizer-test-<id>` and the `.audit/out/catalogizer-test-<id>-client` / `-seed` directories. Each is removed ONLY after its ownership is proven; whatever fails a proof is KEPT and the failed proof is printed. Never a bare name glob, never a signal.

Pods: (1) the creation command is exactly podman-compose's (`podman pod create --name=<pod> --infra=false --share=`); (2) state Created, no container; (3) the project is not live (no lease claim, no per-run state directory, no labelled container). Directories: a real directory (never a symlink), and no project it could belong to is live. `-logs` is never touched.

## Usage

```bash
scripts/test-infra/sweep_leaks.sh [--dry-run]
```

Output: `REMOVED|WOULD_REMOVE|KEPT <kind> <name> ...` per candidate, then `SWEEP pods_removed=<n> pods_kept=<n> dirs_removed=<n> dirs_kept=<n>`. Exit 0; 1 a proven leak could not be removed; 2 usage.

## Tests

`tests/infra/test_sweep_leaks.sh`: a victim and a survivor per proof (other create command, holds a container, live project, name outside the namespace, symlink, `-logs`, live-project directory), dry run, idempotence, a control needle, and 7 paired mutations (each proof removed, patterns widened).

## WF17 fix round 5 (revision 3)

- Two new kinds: `state` (a per-run state directory of a project that is not live, removed when its `<state>/op_id` operation is terminal in THIS registry or its owner is proven dead; a `.keep-state` marker keeps it) and `nasdir` (`$XDG_RUNTIME_DIR/catalogizer-nas-ro.<pid>` and `<repo>/.audit/out/nas-ro-<pid>` of a NAS leg whose pid is proven gone: no such process, a zombie, or a recycled pid whose command line does not name `nas_readonly_leg`). The SWEEP summary line gained `states_*` and `nasdirs_*` counters.
- A podman query that FAILS is `failed=podman_query_failed` (exit 1), never "nothing there". The registry path is the one `scripts/longops/lib.sh` derives. `tests/infra/test_sweep_leaks.sh` gained survivor fixtures for every kept case and the reviewer mutants SW1-SW3 plus an identity mutant (SW4 cannot be constructed: no fixture differs from SW3 observably, UNCONFIRMED as a distinct mutant).

## WF18 fix round 6 (revision 4)

- `nasdir` now also scans `/tmp/catalogizer-nas-ro.<pid>`: the NAS leg's run directory moved below /tmp because run_pinned (round 8, R-1) accepts the `--out` of a scratch-view client only there. /tmp is shared, so a
  directory is removed only when it is a real directory (not a symlink) AND owned by this user (`KEPT nasdir <name> failed=not_owned_by_this_user` otherwise), besides the existing pid proofs. The legacy `$XDG_RUNTIME_DIR` and
  `<repo>/.audit/out/nas-ro-<pid>` locations are still swept.
