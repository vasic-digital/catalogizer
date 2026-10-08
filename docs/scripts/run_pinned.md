# run_pinned.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T12:00:00Z |
| Status | tracked script (T007), guide written by T009; independent review owed (constitution 11.4.142) |
| Source | `scripts/containers/run_pinned.sh`; test `scripts/containers/tests/test_run_pinned.sh` (T003); disk gate `scripts/containers/disk_headroom.sh` ([guide](disk_headroom.md)); lock file `build/containers/images.lock.yaml` |
| Evidence | `specs/001-full-project-audit-remediation/evidence/wp09/` (README, `runp-*` records, `mutation.txt`); reuse check `containers-run-primitive.md` |

## Purpose

The one launcher for a one-shot, rootless, digest-pinned container run (docs/16 sections 7 and 8; constitution 11.4.161, 11.4.173,
12.6, 12.9, 12.12). It resolves an image id from the lock file, refuses anything that is not a full `sha256:` digest present in local
storage (it never pulls), runs the disk-headroom gate, computes memory, CPU and pid limits from the host, composes the `podman run`
argv and executes it. Podman is called directly: the Go run primitive of `submodules/containers` has no typed memory, pids, label or
digest options (tracked deviation (b); `evidence/wp09/containers-run-primitive.md`).

## Usage

```bash
scripts/containers/run_pinned.sh [--rw docs|.audit/scratch] [--out DIR] [--network=none] [--need BYTES] [--op-id ID] IMG-ID -- COMMAND...
```

| Option | Meaning |
|---|---|
| `IMG-ID` | id of a lock entry (`$RUNP_LOCK`, default `build/containers/images.lock.yaml`) |
| `--rw V` | one extra writable bind, exactly `docs` or `.audit/scratch`, at most once; the source must be a real directory (no symlink component) |
| `--out DIR` | output directory mounted at `/out:rw`; absolute; default `$PWD/.audit/out/<op_id>/`; inside the tree only under the sanctioned roots |
| `--network=none` | no network in the container |
| `--need BYTES` | bytes the run needs for the disk gate (default the lock entry size) |
| `--op-id ID` | operation id (label `catalogizer.op_id`, disk record name) |

The source tree is always bound read-only at `/src` (workdir); `/tmp` is a tmpfs; `--cap-drop=ALL`, `no-new-privileges`, `--userns=keep-id`,
user = host uid:gid (override `RUNP_USER`), `--pull=never`, labels `project=catalogizer` and `catalogizer.op_id`.

### Environment

| Variable | Meaning |
|---|---|
| `RUNP_PRINT_ARGV=1` | print the composed argv, one element per line, exit 0, start nothing (oracle 1 of the test) |
| `RUNP_LOCK` | lock file to read |
| `RUNP_MEMORY` / `RUNP_CPUS` / `RUNP_PIDS` | limit overrides, bounded by 0.60 of MemTotal / nproc and by `ulimit -u` (the 12.6 ceiling cannot be lifted) |
| `RUNP_USER` | `uid:gid` of the container user |
| `RUNP_TEST_MODE=1` | enables the two test hooks `RUNP_MEMINFO` and `RUNP_ULIMIT_U`; the hooks are refused without it |
| `DISK_HEADROOM_OUT_DIR` | inherited by the disk gate |

Limits: `memory = min(0.60*MemTotal, MemAvailable - max(4 GiB, 0.15*MemTotal))`, `memory-swap = memory`, `cpus = min(2, 0.60*nproc)`,
`pids-limit = min(2048, min(8192, ulimit -u / 2))`, until a measured envelope exists.

## Exit codes and refusals

| Code | Meaning |
|---|---|
| exec | success: the script `exec`s podman, the exit status is the container's |
| 1 | refusal, printed `run_pinned: REFUSED reason=<code> ...` |
| 2 | usage error |

Reasons include: `digest_malformed`, `tag_only_reference`, `reference_missing`, `reference_malformed`, `lock_unreadable`, `lock_size_malformed`,
`image_not_present_locally`, `memory_budget_unavailable` (budget below 512 MiB: retry later, never override), `cpu_budget_unavailable`,
`pids_budget_unavailable`, `memory_override_out_of_bounds`, `cpus_override_out_of_bounds`, `pids_override_out_of_bounds`, `rw_value_not_allowed`,
`rw_given_twice`, `rw_source_not_canonical`, `out_dir_*` (`in_source`, `malformed`, `not_absolute`, `uncreatable`, `unresolvable`),
`source_path_malformed`, `secret_state_in_mount`, `state_dir_unresolvable`, `user_override_malformed`, `entrypoint_*`,
`test_hook_outside_test_mode`, `headroom_script_missing`, `dependency_missing`; a disk-gate refusal passes its own reason through
(`disk_below_headroom`, ...).

## Side effects

Creates the `--out` directory (and `$PWD/<rw>`) when absent; the disk gate writes one JSON record (see the disk guide). No image is pulled.

## Tests and mutations

`scripts/containers/tests/test_run_pinned.sh` (T003) checks two independent oracles (print-argv and a `podman` shim log) and re-runs the
body against copies of the launcher with one `# MUT:<name>` line removed or weakened; every copy must fail. T009 adds the focused mutation
"remove `--memory` only": `evidence/wp09/mutation.txt`.

## Troubleshooting

* `memory_budget_unavailable`: the host has less than about 5.2 GB available at 31 GB total; free memory, do not set `RUNP_MEMORY`.
* `image_not_present_locally`: pull by digest through the pin tooling (T005d), never through this script.
* Wrappers may refuse with `anti_mess_drift` after a leaked container; call this script directly.
