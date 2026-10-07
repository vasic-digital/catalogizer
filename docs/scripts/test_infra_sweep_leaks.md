# sweep_leaks.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T05:00:00Z |
| Status | new (WF12 F2 fix), independent review owed (constitution 11.4.142) |
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
