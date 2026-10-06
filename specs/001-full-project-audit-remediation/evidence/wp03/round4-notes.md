# WP-02 / WP-03 round 4 notes (WF3 review fixes)

| Field | Value |
|---|---|
| Date | 2026-10-06 |
| Host | anton, git 2.53.0, bash 5.3.9, gawk 5.3.2 (host awk), mawk 1.3.4 20260129 present, jq 1.8.1, python 3.14.4, jsonschema 4.19.2 |
| Container | IMG-TESTUTIL via scripts/containers/run_pinned.sh --network=none (mawk 1.3.4 20200120 as awk, git 2.39.5, jq 1.6, jsonschema 4.10.3, no column) |
| Repo HEAD | a27d72a55c99da68a36a3359d6334f84e62a16a2 (work tree changes uncommitted, nothing staged) |
| sha256 (first 16) | verify_repos.sh 9111298dba4a4be0; test_verify_repos.sh 22437528050ad5c4; test_verify_schema.sh b4cc181312b4b62a; scope_to_lumen_json.py 6a41f619bcb71251; test_scope_to_lumen_json.sh 3413b6158ce32622; mutate_wp02_wp03.sh 033cce96e2367e83 |
| Reviewed by | not reviewed: this is the author's fix round; the independent re-review required by the WF3 review (section 7 item 7) is owed |

## Results (final, this session)

| Suite | Host | IMG-TESTUTIL container |
|---|---|---|
| test_verify_repos.sh | 375 ok / 0 FAIL | 375 ok / 0 FAIL |
| test_verify_schema.sh | 14 / 0 | 14 / 0 (was 6 / 8 before the explicit-base change) |
| test_derive_scope.sh | 9 / 0 | 9 / 0 |
| test_scope_to_lumen_json.sh | 39 / 0 (35 + 4 new RED-first tests) | 39 / 0 |

Container before the round: test_verify_repos 169 ok / 167 FAIL (evidence/wp04/verify-repos-container-rootcause.md). Files: round4-green-host-*.txt, round4-green-container-*.txt, round4-red-*.txt (the section-38 RED was run with a harness that executes only the new section against the ORIGINAL verifier c5468681..., not the whole suite).

## Findings fixed (each RED first)

- I-1 (--fetch recursion and maintenance): fixed; RED 1 FAIL line(s) of the I-1 group on the original verifier (the submodule ref snapshot changed; the gc fixture ran gc and the pre-auto-gc hook once it had 1500 loose objects), GREEN after the fix. The first version of the gc fixture did NOT go RED (too few loose objects); it was strengthened and re-observed RED before the fix.
- I-2 (ls-remote tail match): fixed; RED exit 0 (false SAME) vs want 11 and a wrong remote_tip. The default-branch (--symref HEAD) path always prints HEAD first, so the exact-HEAD match cannot be separated from a tail match by any fixture; the I-2b fixture is a control only (no mutant is claimed for it).
- I-3 (core.fsmonitor): fixed; RED exit 0 / dirty false with a lying protocol-v2 hook; the test's control asserts that plain git status is blind under the same hook and that -c core.fsmonitor=false sees the change.
- I-4 (attached owned submodule pin reachability): fixed; RED exit 0 vs want 11; oracle: a real fresh clone of the parent fails with 'not our ref' (asserted in the test). Controls: pushed pin exit 0; pin behind the pushed tip exit 0 and class SAME; --strict on that is 15 only.
- I-5: suites run in IMG-TESTUTIL (table above); shellcheck 0.11.0 (pinned image) at -S warning is clean (round4-shellcheck.txt; three warnings fixed: SC2034 remotes, SC2194 with a stated directive, SC2034 VR in the runner); a coverage figure: round4-coverage.txt. kcov could NOT run: the image run drops ptrace (kcov: error: Can't start/attach to /usr/bin/bash), so the PS4 line-trace method of 11.4.224(E) was used: 244 of 285 executable lines (85.6 percent strict), 244 of 245 (99.6 percent) after excluding 40 instrument-artifact lines; limits in the file (line not branch; an assertion-free test would count as covered).
- Container root causes: RC1 (mawk NUL printf) fixed with a bash printf loop; RC2 ({64} interval) fixed with length/class tests in the verifier and in the test; RC4 identity: per-call -c user.name/-c user.email on the three bare commit-tree calls (the 'commit -a' hook case already used the G prefix and passes in both environments); RC5 column: replaced by awk alignment and asserted by a failing column shim; RC6 jsonschema skew: the test sets an explicit absolute base on an in-memory COPY of the schema (the contract file is unchanged); RC3 dubious ownership is run_pinned.sh (another agent's change; its --user is present and was used by every container run here).
- m-1, m-2 (the code was already right, the mutants survived): tests added (FAILCMD=symbolic-ref; retargeted skip-worktree symlink), mutants R4g, R4h CAUGHT. m-4 (real defect, glob pathspec and dropped exit status): fixed, RED exit 0 vs want 13. m-3 (sibling root control): test added, SM12 CAUGHT. m-5a, m-5b, m-8: real latent defects in scope_to_lumen_json.py, fixed with tests (RED 4 FAIL), SM8-SM11 CAUGHT. m-6 (--help printed source code): fixed and tested. m-9: SM3 re-pointed; the scope part of the runner is 21 CAUGHT / 0 SURVIVED / 0 BROKEN. m-7: stale statements updated (verify_repos.md, scope_to_lumen_json.md, org_of.md, owed-contract-items.md, exit-code-decisions.md).

## Mutation

Runner additions (scripts/audit/tests/mutate_wp02_wp03.sh): R4a-R4k (verify part), SM3 re-pointed, SM8-SM12 (scope part), DRYRUN=1 (proves every pattern applies exactly once without running suites: 81 caught, 0 broken). Results: round4-mutation.txt, wp02/round4-mutation-scope.txt. R4a-R4i and R4k CAUGHT on the host. R4j (a {64} interval put back into the exception test) SURVIVED on the host ONLY because the host mawk (20260129) supports intervals; in the container (mawk 20200120) the same mutant fails 7 assertions (CAUGHT there). STATED: R4j is caught only in the container. Older patterns that mention STATUS_ARGS (RV6, RV6b), RM1 and MB2b were re-pointed to the new source text. The R4 mutation runs used the test file as it was before the last five added assertions (--help x3, unparseable submodule status x2) and before the SC2194 directive line; those changes only add assertions or a comment.

## Real tree (read-only)

plain rc 13: SAME 191, UNKNOWN-DIFFERENT 8, unproven 1 (submodules/constitution): identical to the round-3 baseline, so the attached-pin check adds no new failing or unproven row; --no-remote rc 13. File: round4-real-tree.txt. No UNREACHABLE occurred in this run.

## Owed items (not done in this round) and UNCONFIRMED

- Independent re-review of this round (owed, T011a).
- m-10: the final-verifier real-tree record is the summary in round4-real-tree.txt; a full-JSON baseline replacing review-fix3-real-tree.json was not rewritten.
- m-11: no per-repository timeout on git status / ls-files / hash-object (a hung filesystem hangs the run); low priority, recorded in verify_repos.md.
- m-12: T036 (docs/scripts/README.md is a NEW file, and the root README link) is outside this round's touch list; still owed.
- Decisions (a), (b), m-d, m-i, m-k, m-l, m-m, m-n: owner items, unchanged.
- UNCONFIRMED: the older mutants of the verify part (RM2b-d, RM4, RM8-RM11 and others) were not all re-run against the final verifier; this round ran R4a-R4k, the scope part in full, and the DRYRUN pattern proof. The attached-pin class DIVERGED is reported when the pin sits on another branch than the compared one (same semantic as the detached case); no real-tree row shows it today.
- UNCONFIRMED: kcov branch coverage (kcov cannot run in the sandboxed image).
- UNCONFIRMED: behaviour on a git older than 2.29 (no --no-auto-maintenance): the fetch command line would be rejected and the class would read UNKNOWN-DIFFERENT (fail-closed), not clean.
