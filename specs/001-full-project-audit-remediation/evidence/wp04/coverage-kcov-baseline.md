# kcov line-coverage baseline for the WP-04 bash scripts (T044 support)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | evidence note of WP-04 fix round 4; the section 11.4.44 header was added in the round-4 cleanup pass |
| Status summary | kcov line-coverage baseline of the WP-04 bash scripts (T044 support), measured in IMG-KCOV |
| Issues | none recorded in this note |
| Issues summary | see the body below |
| Fixed | section 11.4.44 header added |
| Fixed summary | header only; the body is unchanged |
| Continuation | the open items named in the body, if any |

| Field | Value |
|---|---|
| date_utc | `2026-10-06T00:12:09Z` |
| image_id | `IMG-KCOV` |
| image_digest | `sha256:34e6c26730c284da43a8384e9c5fa9a1af90dc7f9ea355f2f472f5fd230ab164` |
| kcov_version | `kcov 43` |
| repo_head | `a27d72a55c99da68a36a3359d6334f84e62a16a2` |
| runner | `scripts/containers/run_pinned.sh` |
| runner_sha256 | `8a0e6daa71cf64cf5160dd6b0eef844a3135a062be8866afb0edbd6339f48c40` |
| lock_sha256 | `d2e8fa9d23863776a53549b292d771d7b0b0de9902c53ef668f475268cf1a1c3` |
| kcov_flags | `--include-path=/src/scripts --exclude-pattern=/tests/` |
| measurement | `kcov bash line coverage (executable lines as kcov determines them), union over the listed tests, union of hits>0` |

Status: measured. No file was edited under test; no commit. Raw kcov outputs: `.audit/scratch/kcov/` (git-ignored), every `cobertura.xml` and `.out` file sha256 is in `coverage-kcov-baseline.json` (`raw_outputs_sha256`).

## Method
- Each test ran once on the host (baseline pass counts, `.audit/scratch/kcov/host/`) and once under `kcov 43` in IMG-KCOV via `scripts/containers/run_pinned.sh --rw .audit/scratch --out <dir> --network=none IMG-KCOV -- kcov --include-path=/src/scripts --exclude-pattern=/tests/ /out/cov <test script>`; `disk_headroom.sh --need 0` passed before every run (the runner calls it too). Source mount read-only, `--rw .audit/scratch` only.
- The test script is passed to kcov directly (it is executable with a bash shebang). The form written in T044 and in the test headers, `kcov /out/cov bash <script>`, was tried first and FAILED: `kcov: error: Can't start/attach to /usr/bin/bash` (kcov treated `bash` as an ELF binary and needs ptrace, "Can't set personality: Function not implemented"). So T044 and the test-file `Usage` lines name a command that does not work as written (finding F1).
- Per script: executable lines and covered lines are kcov's own cobertura numbers, union across the tests listed (hits > 0 in any run). Percent = covered / executable. No estimates.

## Result table

| Script | Test(s) | Covered / executable (kcov) | % | Uncovered line ranges | File lines | Embedded Python (not measurable) |
|---|---|---|---|---|---|---|
| `scripts/repo/check_no_ci.sh` | check_no_ci | 17 / 19 | 89.47 | 38, 42 | 44 | - |
| `scripts/repo/check_revision_headers.sh` | check_revision_headers | 33 / 39 | 84.62 **<85** | 31-33, 52-54 | 70 | - |
| `scripts/repo/scope_check.sh` | scope_check | 58 / 62 | 93.55 | 56, 59, 108, 110 | 113 | - |
| `scripts/repo/validate_cheap.sh` | validate_cheap | 3 / 3 | 100.0 | none | 360 | 318 lines (42-359) |
| `scripts/repo/commit_recursive.sh` | commit_recursive | 83 / 87 | 95.4 | 44-45, 74, 125 | 140 | - |
| `scripts/repo/push_recursive.sh` | push_recursive | 97 / 105 | 92.38 | 45, 48, 74, 118, 120, 134-135, 176 | 185 | 12 lines (92-103) |
| `scripts/repo/integrate_merge.sh` | integrate_merge | 164 / 171 | 95.91 | 122, 165, 214-215, 221, 231, 247 | 253 | - |
| `scripts/repo/integrate_ff_only.sh` | integrate_ff_only | 140 / 155 | 90.32 | 44, 59, 85, 101, 115, 151, 191-199 | 248 | - |
| `scripts/repo/record_deferral.sh` | record_deferral | 35 / 35 | 100.0 | none | 54 | - |
| `scripts/repo/check_class.sh` | check_classes | 3 / 3 | 100.0 | none | 106 | 85 lines (21-105) |
| `scripts/repo/fixture_roots.sh` | fixture_roots | 4 / 4 | 100.0 | none | 51 | 35 lines (14-48) |
| `scripts/repo/verify_repos.sh` | verify_repos | 196 / 261 | 75.1 (INVALID: failing run, see F3) | 83-84, 100, 114, 130, 143-144, 147, 173-183, 185-187, 209-213, 227-228, 231, 242, 285, 321, 324, 345-357, 359-372, 375-377, 379 | 397 | - |
| `scripts/repo/lib_safe.sh` | all 14 suites (sourced library) | 9 / 9 | 100.0 | none | 51 | - |
| `scripts/review/check_review_provenance.sh` | check_review_provenance | 70 / 82 | 85.37 | 85-95, 166 | 193 | 10 lines (85-94) |
| `scripts/ledger/project_gate_ledger_ratchet.sh` | project_gate_ledger | 62 / 67 | 92.54 | 40, 81, 83, 88, 107 | 124 | - |

## Do the tests pass the same count under kcov as on the host?

| Test | host pass/fail | under kcov pass/fail | Same |
|---|---|---|---|
| test_check_no_ci | 30/0 | 30/0 | yes |
| test_check_revision_headers | 25/0 | 25/0 | yes |
| test_scope_check | 50/0 | 50/0 | yes |
| test_validate_cheap | 106/0 | 106/0 | yes |
| test_commit_recursive | 55/0 | 55/0 | yes |
| test_push_recursive | 92/0 | 92/0 | yes |
| test_integrate_merge | 117/0 | 117/0 | yes |
| test_integrate_ff_only | 80/0 | 80/0 | yes |
| test_record_deferral | 26/0 | 24/0 | **NO** |
| test_check_classes | 131/0 | 131/0 | yes |
| test_fixture_roots | 30/0 | 30/0 | yes |
| test_verify_repos | 336/0 | 169/167 | **NO** |
| test_check_review_provenance | 113/0 | 113/0 | yes |
| test_project_gate_ledger | 43/0 | 43/0 | yes |

Test `sha256` values are in the JSON file. Plain wording: 12 of 14 suites give the host count under kcov. `check_review_provenance` matches only after a git setting (F2); `record_deferral` and `verify_repos` do not match (F3, F4).

## Findings

- **F1** The T044 command form `kcov /out/cov bash <script>` does not work in IMG-KCOV (error quoted above). The working form passes the executable script path directly. Owed: correct T044 and the Usage lines of the test headers (not edited here).
- **F2** Inside the container the repo mount `/src` is "dubious ownership" for git (the first lines of every kcov run print `fatal: detected dubious ownership in repository at '/src'`). Most suites only show it as noise. `check_review_provenance` (needs a git work tree for `--bootstrap`) went 109 pass / 4 fail without a fix and 113 / 0 with `git config --global --add safe.directory "*"` run first inside the container (`bash -c` wrapper in the same podman run, HOME is tmpfs; run3 in the evidence list). The reported coverage for it (70/82) comes from the 113/0 run. The first attempt (109/4) is kept in `.audit/scratch/kcov/run/` and in `kcov_first_attempt_pass_fail`.
- **F3** `test_verify_repos.sh` is 336/0 on the host and 169 pass / 167 fail inside IMG-KCOV, with and without kcov (a plain `bash` run inside IMG-KCOV, with the safe.directory and git identity set, gives the same 169/167, so kcov is NOT the cause; the container environment is). Concrete lead from a probe in the image: for a repository whose root is `--root <dir>` the report row has `"path":""` where the host gives `"."`, and `summary.pin_drift` is 1 on a plain repository; the image also lacks the `column` command used by the table output (`column: command not found`). Root cause beyond that: UNCONFIRMED. The 75.1% for `verify_repos.sh` therefore comes from a run in which 167 assertions failed and MUST NOT be accepted as a baseline; the host-side measurement needs a kcov run that reproduces 336/0 first.
- **F4** `test_record_deferral.sh`: 26 pass on the host, 24 under kcov (the container user is root; the three unwritable-directory cases print `unwritable case skipped (root)`). The covered lines of `record_deferral.sh` (35/35) therefore come from a run that skipped 3 of the cases; the number is a lower bound only for the skipped branch tests, not a finding about the script.
- **F5** Four scripts are thin bash shims around an embedded Python program (`validate_cheap.sh` 318 Python lines, `check_class.sh` 85, `fixture_roots.sh` 35, `push_recursive.sh` 12, `check_review_provenance.sh` 10). kcov traces bash only. The percentages above cover the bash lines of those scripts; for `validate_cheap.sh` (3 of 3 bash lines), `check_class.sh` (3/3) and `fixture_roots.sh` (4/4) a 100% figure says nothing about the Python logic, which carries almost all behaviour. Their code coverage is UNCONFIRMED (needs a Python coverage instrument; none measured here).
- **F6** Honest limits of the figure (constitution 11.4.224(C)(E)): kcov gives LINE coverage, not branch coverage; lines inside `set +x` regions and traps are not accounted; subshell and child-process lines were traced where the child is `bash` started from a script path (e.g. `check_no_ci.sh`, `lib_safe.sh`, `record_deferral.sh` appear under the caller's test), but a script run from a copied fixture path outside `/src/scripts` is not counted. The 85% floor also needs RED-capable tests (T044/11.4.224 numerator rule); this baseline did NOT run the mutation sweeps, so it does not establish that the covered lines are RED-capable. It is an upper bound for the floor check, not a pass.

## Below 85% or not acceptable
- `scripts/repo/check_revision_headers.sh` 84.62% (33/39; uncovered 31-33, 52-54).
- `scripts/repo/verify_repos.sh` 75.1% from an invalid run (F3).
- `scripts/review/check_review_provenance.sh` 85.37% (70/82): at the floor by 0.37 points, 12 uncovered lines (85-95, 166), and 10 of its lines are embedded Python.
- Scripts 85-100% by kcov but with an unmeasured Python body (F5): `validate_cheap.sh`, `check_class.sh`, `fixture_roots.sh`.

## UNCONFIRMED
- Root cause of the 167 `verify_repos` failures in IMG-KCOV (F3, one lead given).
- Coverage of the embedded Python (F5).
- Branch coverage of any script (not measured).
- Whether covered lines are RED-capable (no mutation runs here by instruction).
- `scripts/ledger/project_gate_ledger_ratchet.sh` was measured through `test_project_gate_ledger.sh` only; the mutations file was not run.
