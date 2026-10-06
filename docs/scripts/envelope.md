# envelope.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | committed in a7cfc6d3 (T119, T120); revision 2 is the fix round r1 for the independent review (uncommitted until the owner commits it): fail-closed registry accounting, measured CPU count; its row in `docs/scripts/README.md` is still owed |
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
`test_hook_outside_test_mode`, and the registry refusals of the fix round r1 (review F3; the accounting FAILS CLOSED, an input that cannot be read is never read as `used = 0`): `registry_override_outside_test_mode` (a `LONGOPS_REPO` / `LONGOPS_DIR` / `LONGOPS_AUDIT` set outside a declared test run), `registry_library_missing` (no `scripts/longops/lib.sh` in the tree the script runs from), `registry_unreadable` (the ops directory, or a parent of it, cannot be read; an ops directory that does not exist is the honest empty registry), `registry_record_unreadable`, `registry_record_malformed` (a record that is not JSON, or a live op whose budget is not a non-negative integer), `registry_classify_failed`. Usage errors exit 2.

## Test hooks

`ENVELOPE_MEMINFO`, `ENVELOPE_NPROC`, `ENVELOPE_ULIMIT_U` replace a real reading and are honoured only with `ENVELOPE_TEST_MODE=1`; set without it they
are refused, so a stray exported variable can never lift a ceiling. The registry is relocated with `LONGOPS_REPO` / `LONGOPS_DIR` / `LONGOPS_AUDIT` of `scripts/longops/lib.sh`, which now ALSO needs `ENVELOPE_TEST_MODE=1` (review F3: a stray exported variable could point the accounting at an empty registry and hide every live budget).

## The CPU count is measured

`nproc` is read with `OMP_NUM_THREADS` and `OMP_THREAD_LIMIT` removed (`env -u ... nproc`): GNU `nproc` honours both, so `OMP_NUM_THREADS=1000` used to report 1000 CPUs and lift the 0.60 CPU ceiling (review F7). `run_pinned.sh` reads `nproc` the same naive way; fixing it is an OWED request (it is another agent's file in this round).

## Test

`scripts/containers/tests/test_envelope.sh`: SPECIFIED oracle (hand-computed goldens from the formulas) and DERIVED oracle (an independent python3
implementation over a seeded random sweep of 60 hosts), a registry fixture filled through the real `register.sh`, refusals, formats, a negative control
(two hosts give two envelopes) and a real-host leg, then 29 paired mutations. Each copy is placed in a working tree layout (`<dir>/scripts/containers/envelope.sh` beside a link to `scripts/longops`: a copy dropped in `/tmp` could not read the registry and every mutant failed the same registry checks, review M1), an UNMUTATED copy placed the same way is the negative control (it must pass the whole body), and a mutant counts as caught only when a failing check NAMES its cause. The set contains the reviewer's mutants R1-R4 (the JSON `memory_bytes`, the `ENVELOPE_ULIMIT_U` hook gate, the `pids` floor, the sum over live ops). `ENVELOPE_MUTATION_RECORD` names the record file.

## Honest boundary (11.4.6)

The envelope is arithmetic on readings taken at one instant; it does not measure `per_job_mem` (that is the measured profile of T115) and it does not
prove a build fits. Whether `user.slice` scoping on this host matches the 12.11 assumption is `UNKNOWN:` (the docs/16 8.5 probe).
