# envelope.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T18:00:00Z |
| Status | new in the working tree (T119), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/containers/envelope.sh`; test `scripts/containers/tests/test_envelope.sh` |

## Purpose

Computes the dynamic resource envelope of docs/16 section 8.2 (12.6, 12.11, 12.12) from the live host, read-only, so that no limit of a
containerized run is hard-coded (11.4.6). The wrappers (`run_go.sh` and the others, through `runner_lib.sh`) hand its `memory_bytes`, `cpus`
and `pids` to `run_pinned.sh`, and `scripts/test-in-container.sh` composes `--memory` and `--cpus` from it.

## Usage

```bash
scripts/containers/envelope.sh [--toolchain NAME] [--profile FILE] [--format kv|json]
```

`--toolchain` selects the per-toolchain measurement in the profile (`[a-z0-9_-]+`); `--profile` defaults to `build/containers/profile.json`
(`{"toolchains": {"<name>": {"per_job_mem_bytes": <positive integer>}}}`, written by the measured runs of 11.4.24); `--format kv` (default) prints one
`key=value` per line, `json` one object, schema `envelope/1`.

## Formulas (integer arithmetic)

```
reserve_cpu = max(2, ceil(0.125 * nproc))      reserve_mem = max(4 GiB, 0.15 * MemTotal)     ceiling_mem = floor(0.60 * MemTotal)
mem_budget  = min(ceiling_mem - used_mem, MemAvailable - reserve_mem)
cpu_budget  = nproc - reserve_cpu - used_cpu   jobs = max(1, min(cpu_budget, floor(mem_budget / per_job_mem)))
memory_bytes = mem_budget    cpus = clamp(cpu_budget, 1, floor(0.60 * nproc))    pids = min(2048, min(8192, ulimit -u / 2))
```

`used_mem` / `used_cpu` are the budgets of the registered long operations (`scripts/longops`) that are alive (classification `advancing` or
`hung`); terminal operations and dead owners are not subtracted (a dead owner holds nothing: it is the anti-mess sweep's finding). `per_job_mem` is
measured or `UNKNOWN`: the status then reads `UNKNOWN:<reason>` (`no_toolchain`, `profile_absent`, `profile_unreadable`, `toolchain_unmeasured`,
`per_job_invalid`) and `jobs = 1`. A value that is not a positive integer is invalid, never a division by zero.

## Refusals and exits

`envelope: REFUSED reason=<code>` on stderr and exit 1: `meminfo_unreadable`, `cpu_budget_unavailable`, `pids_budget_unavailable` (an unreadable, zero
or unparsable `ulimit -u` is a refusal, never the permissive 8192), `memory_budget_unavailable` (budget below 512 MiB), `dependency_missing`,
`test_hook_outside_test_mode`. Usage errors exit 2.

## Test hooks

`ENVELOPE_MEMINFO`, `ENVELOPE_NPROC`, `ENVELOPE_ULIMIT_U` replace a real reading and are honoured only with `ENVELOPE_TEST_MODE=1`; set without it they
are refused, so a stray exported variable can never lift a ceiling. The registry is relocated with the `LONGOPS_*` variables of `scripts/longops/lib.sh`.

## Test

`scripts/containers/tests/test_envelope.sh`: SPECIFIED oracle (hand-computed goldens from the formulas) and DERIVED oracle (an independent python3
implementation over a seeded random sweep of 60 hosts), a registry fixture filled through the real `register.sh`, refusals, formats, a negative control
(two hosts give two envelopes) and a real-host leg, then 18 paired mutations (one expression changed per copy, python str replace, exactly one
occurrence; every copy must make the test FAIL; `ENVELOPE_MUTATION_RECORD` names the record file).

## Honest boundary (11.4.6)

The envelope is arithmetic on readings taken at one instant; it does not measure `per_job_mem` (that is the measured profile of T115) and it does not
prove a build fits. Whether `user.slice` scoping on this host matches the 12.11 assumption is `UNKNOWN:` (the docs/16 8.5 probe).
