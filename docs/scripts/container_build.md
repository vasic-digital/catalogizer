# container-build.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/container-build.sh`; test `tests/infra/test_consumers.sh` |

## Purpose

The containerized build entry point. `docker-compose.build.yml` requires the per-run credentials and random host ports of its postgres/redis services (`${TI_*:?}`), so a bare compose call is refused. The script now generates them with `scripts/test-infra/gen_env.sh` into a mode 0600 file under the gitignored `.audit/test-infra/catalogizer-test-build-*`, passes it with `--env-file` to every compose call (validate, up, down) and removes it on exit.

## Usage

```bash
./scripts/container-build.sh [version] [--skip-emulator] [--skip-e2e] [--with-emulator] [--validate-only]
```

`--validate-only` generates the env, validates the compose file with it, prints `Compose file is valid` and stops (before the signing keys and the build): it drives the real validation path in tests. UNCONFIRMED: a full build with the new env was not run (a containerized build of the 4.8 GB builder image is outside this fix).

## WF17 fix round 5 (revision 3)

- `container-build.sh` is a registered long operation (class A/C): a lease on `catalogizer-test-build-<...>`, a keeper that heartbeats while a labelled container runs, the op and test-root labels on every container (`x-labels` in `docker-compose.build.yml`), compose run as `-p <project> --in-pod false` under a scrubbed environment, and ONE EXIT trap that on any exit path (compose failure, INT, TERM, HUP) runs `down --volumes` for exactly that project, removes the project's resources, closes the operation (`complete` only after a successful build) and removes the per-run state. The final summary prints the real exit code.
- The build stack's Redis now requires a password (`--requirepass`, `REDIS_PASSWORD`, `REDISCLI_AUTH`) generated per run like every other credential. Documented direct use of `gen_env.sh` plus a bare compose call is withdrawn (it leaked a credential file and pinned the project live): use this script or `up.sh`.
