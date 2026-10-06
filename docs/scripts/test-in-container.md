# test-in-container.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | committed in a7cfc6d3 (T121); revision 2 is the fix round r1 (TIC asks for 98% of its envelope reading; single-hook gate tests; uncommitted until the owner commits it); independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent). T121a (remote lanes, the `site` column) and T121b are BLOCKED on `scripts/build/dispatch.sh` (T005b) |
| Source | `scripts/test-in-container.sh` (TIC), lane table `scripts/containers/lanes.tsv`; test `scripts/containers/tests/test_test_in_container.sh` |

## Purpose

The lane dispatcher: every build and test lane is started through this one command. It looks the `(app, lane)` up in `scripts/containers/lanes.tsv`
and hands the command to the wrapper that row names, with `--memory` and `--cpus` composed from `scripts/containers/envelope.sh` (`--memory` is 98% of TIC's reading: the wrapper reads the envelope again moments later and refuses a request above its own reading, so TIC asks for LESS and a 2% fall of `MemAvailable` between the two reads is covered; review F2/F16). There is no
bare-host fallback: an unknown app, an unknown lane, an `(app, lane)` without a row and a row whose wrapper does not exist yet are each REFUSED and the
command never runs.

## Usage

```bash
scripts/test-in-container.sh [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
                             [--no-progress-s N] [--wall-s N] <app> <lane> -- <command word>...
```

The options are handed to the wrapper unchanged, after the envelope `--memory` / `--cpus`; `--memory` and `--cpus` are not options of TIC.

## The lane table (`scripts/containers/lanes.tsv`)

Three TAB separated columns, `app lane wrapper`; `#` lines and blank lines are ignored. Apps: `catalog-api`, `catalog-web`, `qa`, `docs`, `tooling`.
Lanes: `unit`, `contract`, `integration`, `e2e`, `api`, `docs`, `render`, `tooling`. The wrapper is one of the reviewed set (`run_go`, `run_node`,
`run_docs`, `run_scan`, `run_playwright`, `run_testutil`, `run_qa`, `run_rust`, `run_kcov`). The table is validated as a whole before any lookup: a
malformed row, an unreviewed wrapper name or a duplicate `(app, lane)` refuses every lane.

| App | Lane | Wrapper |
|---|---|---|
| catalog-api | unit, contract, integration, e2e | run_go |
| catalog-web | unit, contract, tooling | run_node |
| catalog-web | e2e | run_playwright |
| docs | docs, render | run_docs |
| tooling | unit | run_testutil |

`qa` is reserved with no row: until T212 adds the `run_qa.sh` rows, `TIC qa ...` is refused as `unknown_app` (never sent to another image). No row
names `run_scan.sh` yet (the task text names no app key for it), `run_rust.sh` / `run_kcov.sh` (T200a) and the Android rows of T204 are added by their own
reviewed rows. The `site` column (`remote` / `split` / `local`) is T121a's and is not implemented.

## Exits

The wrapper's exit code on a run; 1 `test-in-container: REFUSED reason=<code>`: `unknown_app`, `unknown_lane`, `no_lane_row`, `lane_table_unreadable`,
`lane_table_malformed`, `lane_table_duplicate`, `wrapper_missing` (BLOCKED until its task adds the wrapper), `envelope_refused`,
`test_hook_outside_test_mode`; 2 usage.

## Test hooks

`TIC_WRAPPER_DIR` (a directory of `run_*.sh` shims) and `TIC_LANES` (a lane table file), honoured only with `TIC_TEST_MODE=1`.

## Test

`scripts/containers/tests/test_test_in_container.sh`: the SPECIFIED oracle is a hand-written `(app, lane) -> wrapper` table that the real lane table must
equal exactly; a shim per wrapper records the argv; the control needle (`catalog-api unit` reaches the `run_go.sh` shim); every refusal asserts that the
command did not run anywhere; table validation; and a real chain TIC -> wrapper -> `run_pinned.sh` -> pinned image. 18 paired mutations (the fix round r1 adds the single-hook gates `TIC_LANES` and `TIC_WRAPPER_DIR`, review R5, and the 98% margin), among them the
one the task names: a dispatcher copy that drops `--memory` from the composed call (`drop-memory`); the record is `TIC_MUTATION_RECORD`.

## Honest boundary (11.4.6)

TIC places a command in the right wrapper with the envelope limits; it does not decide which lanes need the WP-13 stack or a remote build host
(T121a, blocked).
