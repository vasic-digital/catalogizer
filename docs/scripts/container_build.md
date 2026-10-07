# container-build.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T05:00:00Z |
| Status | WF12 F1 fix (consumer of the T129 compose contract), independent review owed (constitution 11.4.142) |
| Source | `scripts/container-build.sh`; test `tests/infra/test_consumers.sh` |

## Purpose

The containerized build entry point. `docker-compose.build.yml` requires the per-run credentials and random host ports of its postgres/redis services (`${TI_*:?}`), so a bare compose call is refused. The script now generates them with `scripts/test-infra/gen_env.sh` into a mode 0600 file under the gitignored `.audit/test-infra/catalogizer-test-build-*`, passes it with `--env-file` to every compose call (validate, up, down) and removes it on exit.

## Usage

```bash
./scripts/container-build.sh [version] [--skip-emulator] [--skip-e2e] [--with-emulator] [--validate-only]
```

`--validate-only` generates the env, validates the compose file with it, prints `Compose file is valid` and stops (before the signing keys and the build): it drives the real validation path in tests. UNCONFIRMED: a full build with the new env was not run (a containerized build of the 4.8 GB builder image is outside this fix).
