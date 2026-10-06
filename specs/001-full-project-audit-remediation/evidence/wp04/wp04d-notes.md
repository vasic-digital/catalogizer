# WP-04 fix round 4: notes

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | evidence note of the fix round for WF3-REVIEW-wp04-helpers (nothing staged or committed; tasks.md not edited) |

## What was fixed (finding -> change -> killing case)

| Finding | Change | Cases in `scripts/repo/tests/test_wp04d.sh` (mutants in `run_wp04d_mutations.sh`) |
|---|---|---|
| B-1 symlink type change defeats S2 | `scope_check.sh`: a declared store / blob path that is a symlink, lies below a symlinked directory, or whose HEAD entry is a symlink is refused 13 `append_only` / `blob_name`. `validate_cheap.sh`: a declared symlink in class `evidence` / `evidence-ledger`, and a regular file whose HEAD entry is a link there, is `fail symlink <path>` (10); any other class reports `symlink_not_judged <path>`. `check_revision_headers.sh` reports `symlink_not_judged revision_header <path>`. | 7 S2 cases, 5 S3 cases, 2 revision-header cases; D1-D8 |
| I-1 unreachable remote read as "holds nothing" | S1 (`integrate_ff_only.sh`): no reachable main remote -> 11 `remote_unreachable`; an unreachable remote contributes its last fetched tip (`refs/remotes/<r>/<branch>`, `known_tip` in `lib_safe.sh`); with no known tip and a local-only non-CPA commit -> 11 (cannot compute); otherwise unchanged (20 names only commits no known tip covers). S6 (`push_recursive.sh`): same rule, NOPUSH `remote_unreachable` (11) instead of REFUSED. | 6 S1 cases, 3 S6 cases; E1-E6 |
| I-2 helper tables trusted from the working tree | `validate_cheap.sh` `helper_table`: `--trusted-tables <dir>`; else the code root's committed HEAD (untracked or edited -> exit 20 `class_table_unreviewed`); else a snapshot that is no repository; `--adopt-working-tables` is the explicit owner-approved adoption form. `docs/scripts/check_classes.md` line 72 corrected. | 7 cases; T1-T4 (T3 equivalent, see below) |
| I-3 surviving reviewer mutants | cases for X1, X2, X3 (S1, S6, merge), X6, X7, X8 (3 tables + CPA golden-false), X9 | X1 X2 X6 X7 X8 X9 all CAUGHT (X3 is the shared predicate, killed by X2's mutant) |
| m1 directory whose first entry is a gitlink | S2 and S3 compare the entry's path with the declared path | N1, N2 |
| m2 owned-submodule guard | judges `refs/heads/<run branch>`, falling back to the checked-out branch only when the run branch is absent | N3 |
| m3 two CPA-Run parsers | one shared `cpa_runid` / `cpa_row` in `lib_safe.sh` for S1, S6 and the merge path | N4, X2 |
| m4 `GIT_LITERAL_PATHSPECS` leaked into hooks | `commit_recursive.sh` uses `:(literal)<path>` operands; `integrate_ff_only.sh` sets the variable only on its `g` calls; `push_recursive.sh` no longer exports it | N5, N6, N7 |
| m5 missing explicit input read as empty | `--changeset-from`, `--path-gates` (ff_only and merge) missing -> 20 `changeset_unreadable` / `path_gates_unreadable` | N8, N9, N10 |
| m6 no timeout in `push_recursive.sh` | `--timeout <s>` (default 60) on `ls-remote` and `push` | N11 |
| m7 temp leaks | `validate_cheap.sh` removes `vc_tbl.*` / `vc_reg.*`; `commit_recursive.sh` removes its message files on exit (EXIT trap; before only on the happy path); TERM/INT traps in all helpers | N14, N15, N20 |
| m8 wrong files in the `fail` line | names the files the check output mentions | N16 |
| m9 two notes without a header | `evidence/wp04/README.md` and `T038-findings.md` got the section 11.4.44 table | `m9` case (runs the real reader over the two files) |
| round-2 minors: duplicate / unmatched held rows | `held_row_duplicate`, `held_row_unmatched` (validate_cheap, commit_recursive) | N17, N18, N19 |
| round-2 minor: `test_wp04c.sh` mode 644 | `chmod 755` on `test_wp04c.sh` and `test_wp04d.sh` (working tree mode; nothing staged) | - |
| container portability | the three awk programs that read and print NUL (RS/ORS `"\0"`) are replaced by a pure-bash `gitlinks` function in `lib_safe.sh`; `lib_wp04b.sh` exports `safe.directory=*` through `GIT_CONFIG_*`; no awk interval expression `{N}` is used in the helpers | mawk run and container run (below) |

## Evidence files (all sha256-headed)

- `wp04d-red.txt`: `test_wp04d.sh` against the unfixed helpers: 55 ok, 67 FAIL (exit 1). The pre-fix helper hashes were not recorded (UNCONFIRMED); the sha256 lines are the files as they are now.
- `wp04d-green.txt`: 3 host runs of all 13 WP-04 helper tests (12 earlier tests unchanged in count: 131, 30, 25, 55, 30, 80, 117, 92, 26, 50, 106, 230, plus 124 new; 0 FAIL in all three), one run with mawk 1.3.4 as `awk`, one run of each of the 13 tests inside the pinned container `IMG-TESTUTIL` through `scripts/containers/run_pinned.sh` (uid mapped): 13/13 exit 0.
- `wp04d-mutation.txt`: 44 mutants, 41 CAUGHT, 3 EQUIVALENT, 0 SURVIVED; control run exit 0.
- `wp04d-real-s3.txt`: the shipped S3 and S2 over 77 WP-04 files of the real repository.

## Changes to earlier tests (no assertion weakened, none removed)

- `test_validate_cheap.sh` `run()` and `test_wp04c.sh` `vcrun` / the I2 direct call now pass `--adopt-working-tables`: they run the helper in place from the real repository, whose class tables are still untracked, which the new I-2 rule refuses without the explicit form. Every assertion is unchanged.
- `lib_wp04b.sh`: `GIT_CONFIG_COUNT` 1 -> 2 (adds `safe.directory=*`).
- `run_wp04b_mutations.sh`: 11 stale anchors updated (7 by this round: V29, C13, P2, P3, P13, P16, P16b; 4 stale since round 3: S13, V18, V27, C12); P2 now targets `lib_safe.sh`. `run_wp04c_mutations.sh`: 5 anchors updated (EQ-C6, S4, V3, V4, V5). A scripted check shows every anchor of both drivers now occurs exactly once. The two drivers were NOT re-run this round (they take hours): UNCONFIRMED that every mutant still dies.

## Equivalent mutants (stated, not hidden)

- EQ-T3: removing the "table not committed" refusal is covered by the second check (the working copy differs from the empty committed copy). Residual: an EMPTY untracked table would pass; an empty class table is refused later by the loader.
- EQ-N12 / EQ-N13: on this bash (5.3.9) the EXIT trap already runs when SIGTERM ends the script, so the explicit TERM/INT traps are belt and braces that make the cleanup independent of the shell version. The round-3 reviewer's claim that the EXIT trap of `push_recursive.sh` does not run on SIGTERM did NOT reproduce here (the case passes before and after); UNCONFIRMED on other bash versions.

## Owed (tracked, not fixed here)

1. `check_class.sh` and `check_revision_headers.sh` run standalone still read the tables next to the script (working tree) when used in place. The I-2 rule is applied in `validate_cheap.sh`, the S3 decision seam, which always hands explicit tables to `check_class.sh`. A standalone `check_class.sh` trust rule would change its documented default; owner decision.
2. `commit_recursive.sh` does not itself refuse a declared symlink (S2 / S3 refuse it before S5 in the stage order; a defence in depth at S5 needs the evidence directory, which S5 does not know).
3. `validate_cheap.sh` and `scope_check.sh` callers (T042) must pass `--trusted-tables <approved copy>` or, for the adoption run, `--adopt-working-tables`, until the three tables are committed.
4. Git identity: `commit_recursive.sh` and `integrate_merge.sh` create commits and need `user.name` / `user.email`. The tests supply them through `GIT_AUTHOR_*` / `GIT_COMMITTER_*`; the container run of T042 must supply them too (UNCONFIRMED how `cpa-host` will).
5. The real S3 over the WP-04 files fails `revision_header` only for two notes written by other agents: `evidence/wp04/coverage-kcov-baseline.md` and `evidence/wp04/verify-repos-container-rootcause.md` (no section 11.4.44 header). Not edited here (not my files, in flight).
6. The secret fold (T040a), ratchets, `--measure` / `--check` modes and `hook-filters.json` remain unbuilt, as stated in `wp04c-notes.md`.
7. Rounds 2-3 minors still open (declared in `wp04c-notes.md`): 4, 6, 9, 10, 11 (the `CPA_HELPER_PID` export part), 12, 13, 14.

## UNCONFIRMED

- Whether `known_tip` (the last fetched tip of an unreachable remote) is an acceptable stand-in for what the remote holds is a design decision of this round; it is conservative (a commit pushed from another clone since the last fetch is named as unrecorded) and the cannot-compute case is 11, not 20. The owner may prefer 11 for every unreachable remote; the existing I6 test (hand commit in an owned submodule behind an unreachable scp-form URL, expects 20) then needs a reachable fixture.
- No `pgrep -f` or `pkill` is used by any helper or test of this round.
