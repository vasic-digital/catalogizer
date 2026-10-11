# scope_drift_check.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08 (round 9, review WP-02-T028-r2 N2/N3: tracked-tree render via `lumenignore_skeleton.py`, `scope-drift/1` report; independent review owed) |
| Status | new, uncommitted; independent review owed (constitution 11.4.142); indexed by `docs/scripts/README.md`; origin: review WP-02-T028 F4/F5 |
| Source | `scripts/audit/scope_drift_check.sh`; test `scripts/audit/tests/test_scope_drift_check.sh` |

## Purpose

One command that proves the rendered index scope has not drifted from its DATA: `scope_render_check` (`scope_render.py --check`), `scope_to_lumen_json_check` (`lumen_scope.json` re-derived from committed inputs by `scope_to_lumen_json.py --check`) and `lumenignore_fresh` (the committed `.lumenignore` equals a fresh `gen_lumenignore.py` render, so a new top-level directory or an un-rendered scope change is found). It writes `[{"check","rc"}]` JSON for `index_health.sh --drift-report`.

## Usage

`scripts/audit/scope_drift_check.sh --out REPORT.json [--fresh-out FILE] [--regen] [--repo DIR] [--scope FILE] [--lumen-json FILE] [--submodules-tsv FILE] [--lumenignore FILE]`. `--fresh-out` keeps the fresh render (the `--lumenignore-fresh` input of `index_health.sh`); `--regen` rewrites `.lumenignore` from `lumen_scope.json` (the only sanctioned way to change it). Env `SCOPE_RENDER`, `STJ`, `GEN_LUMENIGNORE` override the tools (tests).

Exit: 0 all checks rc 0; 1 a check differs (report still written); 2 usage; 3 a tool is absent (recorded as rc 127, never a pass). It never runs an index writer.

## Tests

`test_scope_drift_check.sh`: real inputs drift-free, each check turns exit 1 with only its own rc non-zero (perturbed copies, a stub renderer), absent generator exit 3, `--regen` restores the fresh render, usage. Honest limit: `scope_render_check` on a drifted `.gitignore` block is tested through a stub tool only; the real renderer needs the repository state.

## Round 9 (review WP-02-T028-r2 N2/N3; revision 2)

- **N3, host independence.** `lumenignore_fresh` no longer renders from the live directory. `lumenignore_skeleton.py` (own guide: `lumenignore_skeleton.md`) writes a skeleton of `git ls-files -z -s --recurse-submodules` plus the fixed host-local names `.git .claude .remember`, and `gen_lumenignore.py` renders from that skeleton, so untracked local directories (`prev3`, a stray test directory) do not change the result and a fresh clone with the constitution submodule checked out renders the same file (tested by `D11` with a scratch repository and `D13` with a real clone). `--regen` uses the same path. Consequence stated openly: the committed `.lumenignore` no longer carries `/prev3/`, `/catalogizer-test-zzsw10647std/` and `/reports/` (the reviewer's three host-local entries; `reports/` is also in `.gitignore`, which Lumen honours). An untracked, non-ignored directory on a host is therefore NOT denied to Lumen by this file; that is the price of determinism, and the remedy is to track or `.gitignore` it. Absent skeleton tool: exit 3, check rc 127.
- **N2, the report.** `--out` is now the object `{schema: "scope-drift/1", head, scope_sha256, lumen_scope_sha256, lumenignore_sha256, checks: [{check, rc}]}`. `index_health.sh --drift-report` refuses a report whose `scope_sha256` or `lumenignore_sha256` is not that of the files it judges. An unreadable file is bound as 64 zeros.
- Env: `LUMEN_SKELETON` overrides the skeleton tool. Tests: `test_scope_drift_check.sh` (21 checks); mutants s06..s10 and k01..k04 in `mutate_wp02_fix_r6.sh`.
