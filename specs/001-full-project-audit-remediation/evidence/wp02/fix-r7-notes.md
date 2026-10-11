# WP-02 fix round 7 (review WP-02-T028-r2) - notes

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08 |
| Status | evidence note of the uncommitted fix-r7 working tree; independent re-review owed (11.4.142); the changes are committed only after GO |
| Answers | `../reviews/WP-02-T028-r2.json` N1 to N9 and the F7 process note |

## Finding map

| Finding | Fix | RED (old tool, new test) | Test | Mutants |
|---|---|---|---|---|
| N1 | scope.yaml classes `credentials_json`, `build_output`, extended `cache_dir`; P3 refuses a baseline pattern no class covers (`baseline_pattern_uncovered`) | `fix-r7-red-index-health.txt` | `test_index_health_fix_r7.sh` N1 (every baseline pattern probed, review fixtures, near-miss controls, subset check) | m54, scope removal case |
| N2 | live proofs verified (evidence file, sha256, command text, closed allow-list, freshness, kind); fallback = SKIP; drift report is `scope-drift/1` bound to scope and lumenignore sha256 | same | N2 block (reviewer forgeries: zero hash, `echo trust me`, year 2000, hand-written report) | m45..m52, m57..m60 |
| N3 | `lumenignore_skeleton.py`; `scope_drift_check.sh` renders from the tracked tree | `fix-r7-red-scope-drift.txt` | `test_scope_drift_check.sh` D11 (scratch repo, stray dirs), D13 (real clone) | s06, k01..k04 |
| N4 | classes case-insensitive; measured: runner `ignore` 7.0.5 is case-insensitive, git on Linux is not; Lumen matcher UNCONFIRMED | same | N4 block incl. live re-measurement with control needle | m56 |
| N5 | fixtures: `.example`/`.sample` suffix elsewhere FAILs | same | N5 block | m55 (reviewer R5) |
| N6 | the RED runs use the old tools with the new tests; no case fails on argv (rc 2), see the RED files | all `fix-r7-red-*` | - | - |
| N7 | `--captured-not-before` required with `--lumen-status`/`--live-proofs` | same | N7 block | m53 |
| N8 | real-runner leg in `test_scope_negation_reports.sh`; SKIP exits 77 in the three tests that skip | `fix-r7-red-scope-to-lumen.txt` | the tests | n01..n03 still kill; runner leg is extra |
| N9 | `scope_to_lumen_json.py --out` exits 3 without allow roots; `--no-allow-roots` opt-out | `fix-r7-red-scope-to-lumen.txt` | N9 block | j04..j06 |

## F7 (process note, answers review F7)

The next verdict for T028 MUST list `covers_runs` including `f04c555d` and `4c8b07b7` (the two commits whose WP-02 work the earlier reviews judged) in addition to the working-tree run, so no committed WP-02 change is left outside a named review. T022 and T026 stay open and are named in the review's `blocks_final_T028_GO`; nothing here claims them closed.

## Convergence assessment (11.4.276(E), round 2 of the T028 review)

The three important findings share one class: a value the gate accepted without checking it against its source (a class list vs the scope's own patterns, an evidence hash vs a file, a rendered file vs the tracked tree). Each fix closes the CLASS for its seam (coverage check over the whole pattern set, verification of every live-proof field, tracked-tree-only render), not the reviewer's instance. Ground truth was measured before the fix (matcher library behaviour, which directories gen_lumenignore.py lists, what a clone holds), not assumed.

## Not done / residual (stated)

- Lumen's own matcher case behaviour is UNCONFIRMED (not probed: a probe writes the Lumen index).
- An untracked, non-ignored directory on a host is not denied to Lumen by `.lumenignore` any more (documented in `docs/scripts/scope_drift_check.md`).
- Live proofs stay instance-tier controls on a single-uid host (`docs/scripts/index_health.md`).
- `scope.yaml` changed, so `codegraph.json` was re-rendered by `scope_render.py --write` (its `scope_sha256` is embedded); no index writer, build or podman ran.

## RED honesty (N6)

Across the three RED files only one case fails on argv: `N9 opt-out` (the old tool has no `--no-allow-roots` option, so that control cannot run on it; the N9 defect case itself fails on behaviour: rc 0 and a file written). `N7 want rc=2, got rc=3` is a behaviour failure (the old tool accepts a capture with no age bound). Mutation: `fix-r7-mutation-index-health.txt` = full run 60 mutants, 1 survivor (m37, because the new baseline check also refused the test's bad-class scope), fixed by isolating that fixture and making an absent `baseline_excludes` legal; m37 and the new m61 re-run: both caught (appended). `fix-r7-mutation-wp02.txt` 23 mutants, 0 survived.
