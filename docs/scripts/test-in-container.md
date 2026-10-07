# test-in-container.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 4 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T08:00:00Z |
| Status | committed in a7cfc6d3 (T121); revision 2 in 96779242 (single-hook gate tests); revision 3 in d9162b7d (TIC passes no limit); revision 4 is fix round r4 for the WF15 review (TIC EXECS the wrapper: a TERM, INT or HUP to the dispatcher reaches the wrapper, which stops the container; the dispatcher used to die alone with 143 and leave the lane running, and a retry was refused `purpose_conflict`, I1; uncommitted until the owner commits it); independent re-review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed. T121a (remote lanes, the `site` column) and T121b are BLOCKED on `scripts/build/dispatch.sh` (T005b) |
| Source | `scripts/test-in-container.sh` (TIC), lane table `scripts/containers/lanes.tsv`; test `scripts/containers/tests/test_test_in_container.sh` |

## Purpose

The lane dispatcher: every build and test lane is started through this one command. It looks the `(app, lane)` up in `scripts/containers/lanes.tsv`
and hands the command to the wrapper that row names. TIC passes NO `--memory` and NO `--cpus` and reads no envelope: the wrapper reads the envelope itself, once, under its budget lock, so there is exactly one reading of the live quantity (WF13 I1: TIC used to pass 98% of its own earlier reading, and a budget that fell by more than 2%, or another op registering, in the seconds between the two readings refused a valid lane with `limit_exceeds_envelope`, about 1 start in 5 on a loaded host; no fixed margin covers a registration in that window). There is no
bare-host fallback: an unknown app, an unknown lane, an `(app, lane)` without a row and a row whose wrapper does not exist yet are each REFUSED and the
command never runs.

## Usage

```bash
scripts/test-in-container.sh [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
                             [--no-progress-s N] [--wall-s N] <app> <lane> -- <command word>...
```

The options are handed to the wrapper unchanged, in the order given; `--memory` and `--cpus` are not options of TIC (a caller that needs less asks the wrapper directly).

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

The wrapper's exit code on a run (TIC `exec`s the wrapper: the wrapper is the very process TIC was started as, so a TERM, INT or HUP sent to TIC reaches the lane and ends it with 130, WF15 I1); 1 `test-in-container: REFUSED reason=<code>`: `unknown_app`, `unknown_lane`, `no_lane_row`, `lane_table_unreadable`,
`lane_table_malformed`, `lane_table_duplicate`, `wrapper_missing` (BLOCKED until its task adds the wrapper), `envelope_refused`,
`test_hook_outside_test_mode`; 2 usage.

## Test hooks

`TIC_WRAPPER_DIR` (a directory of `run_*.sh` shims) and `TIC_LANES` (a lane table file), honoured only with `TIC_TEST_MODE=1`.

## Test

`scripts/containers/tests/test_test_in_container.sh`: the SPECIFIED oracle is a hand-written `(app, lane) -> wrapper` table (14 rows since T200a: build-scripts unit run_kcov, catalogizer-desktop rust run_rust, installer-wizard rust run_rust; WF13 I3: the oracle was not updated when they landed and the suite was red on main with its negative control declaring the mutation harness blind) that the real lane table must
equal exactly; a shim per wrapper records the argv; the control needle (`catalog-api unit` reaches the `run_go.sh` shim); every refusal asserts that the
command did not run anywhere; table validation; and a real chain TIC -> wrapper -> `run_pinned.sh` -> pinned image. paired mutations (the fix round r1 adds the single-hook gates `TIC_LANES` and `TIC_WRAPPER_DIR`, review R5; round 2 replaces the memory-composition mutants by `passes-memory-again`, `passes-cpus-again` and `reads-the-envelope-again`: a dispatcher that passes a limit or reads the envelope again fails), with an unmutated copy as the negative control; the record is `TIC_MUTATION_RECORD`. The F14 block of `test_runners_hardening.md` runs the real TIC against the real wrapper while the host and the registry change in the window between them (a 3.9% fall of the budget, another op registering 6 cpus, a 12-start stress loop with `MemAvailable` changing every 0.1 s: none may be refused `limit_exceeds_envelope`).

## Honest boundary (11.4.6)

TIC places a command in the right wrapper with the envelope limits; it does not decide which lanes need the WP-13 stack or a remote build host
(T121a, blocked).
