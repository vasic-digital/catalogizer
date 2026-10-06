# WP-04 round 6 (wp04f) notes: fixes for the WF6 review of the WP-04 helpers (round-5 fixes F1, F3)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | round 6 fix evidence; independent review of this change owed (author is not the reviewer) |
| Source | the WF6 review `WF6-REVIEW-wp04-helpers` (scratchpad copy), findings W6-1 to W6-12; part B items (N6-1 to N6-6) have NO separate round-6 note: `$EV/wp03/round6-notes.md` was never written (WF7 E-1, repointed in round 8); they are described in `docs/scripts/verify_repos.md` section "Round 6", `docs/scripts/index_health.md` and `docs/scripts/scope_to_lumen_json.md`, and recorded in `$EV/wp03/round6-*.txt` and `$EV/wp04/wp04f-*.txt` |

`$EV` = `specs/001-full-project-audit-remediation/evidence`.

## Fixed (test first: RED against the pre-fix helpers, then GREEN three times)

| Id | Change | Test (file, section) |
|---|---|---|
| W6-2 (important) | `push_recursive.sh`: the outgoing list is captured with its status (`ul="$(git rev-list ...)"`), no longer `mapfile < <(...)`: a failing `rev-list` is `REFUSED <repo> git_listing_failed`, exit 20, nothing pushed. The parents listing of a held commit gets the same treatment. | `test_push_recursive.sh` 12, 16b (shim failing `*rev-list --topo-order*` / `*rev-list --parents*`, with and without an unrecorded hand commit; control without the shim) |
| W6-12 | `push_recursive.sh`: `git remote` read with its status (a failing one was "no remotes", exit 0). `integrate_merge.sh`: `git status --porcelain` and `git diff --name-only` go to files first; failing ones are exit 20 `git_status_failed` / `git_diff_failed`. | `test_push_recursive.sh` 12b; `test_integrate_merge.sh` last section |
| W6-1 (important) | `push_recursive.sh`, conservative rule: unrecorded local commits AND any remote whose live tip is not held locally (moved since S1) give `NOPUSH <repo> <remote> remote_moved_since_s1` for each moved remote (and `remote_unreachable` for unreachable ones), exit 11, nothing pushed; exit 20 `unrecorded_local_commit` only when no remote moved and none is unknown. F2 is NOT decided. | `test_push_recursive.sh` 13 (S1-form fast-forward with a stale tracking ref, CPA commit, every remote moves again: 11, never 20 naming the published F), 13b, 13c; section 10 and `test_wp04c.sh` W4 expectations changed, see below |
| W6-3 | `discover` no longer prints a `?<path>` marker into the stream of submodule paths: it runs in the current shell and dies itself. A submodule at the path `?x` is a normal repository. | `test_push_recursive.sh` 14 |
| W6-4 | the same change makes `die unsafe_repo` effective (it was lost in a process substitution: stderr message, exit 0, repository pushed). | `test_push_recursive.sh` 15 (gitlink `-evil`) |
| W6-5 | `check_no_ci.sh` and `check_revision_headers.sh --measure`: EXIT trap removes the listing temp file, TERM and INT traps exit 143 / 130. | `test_check_no_ci.sh` 7, `test_check_revision_headers.sh` (last section): SIGTERM during a slow listing, the temp directory is empty afterwards, with a control that the temp file exists while the listing runs |
| W6-6 | `scope_check.sh` walk: a submodule with a `.git` entry that git cannot read is `submodule_git_unreadable`, exit 20; only an ABSENT `.git` is an uninitialised submodule. | `test_scope_check.sh` 8 (git dir moved away; golden-false: an empty directory is no failure) |
| W6-7 | `scope_check.sh`: "is the store in HEAD?" and "is it a link in HEAD?" come from one `head_mode` helper that decides presence only from a successful `git ls-tree` (an unreadable HEAD tree is exit 20 `git_tree_unreadable`; a repository without a commit holds no store). | `test_scope_check.sh` 9 (loose tree object removed: real failure; shim failing `*ls-tree*`; golden-false: a repository with no commit) |
| W6-8 / W6-9 | mutation adequacy: nested listing failure for `push_recursive --recursive` and `scope_check`; the moved-with-tracking-ref plus unreachable-without-tip mix | `test_push_recursive.sh` 16, 13c; `test_scope_check.sh` 10 |

## Changed expectations (not weakened, stated)

Two existing assertions encoded the round-5 rule "unrecorded commit plus a moved remote is 20". The W6-1 directive replaces that rule, so they now assert the new one, and keep the part that matters (the commit is never published, nothing is pushed):

- `test_push_recursive.sh` section 10, "every remote moved AND a real hand commit": 20 naming the hand commit became 11 naming each moved remote, no `unrecorded_local_commit`, r1 untouched.
- `test_wp04c.sh` W4, "an unrecorded commit with a remote tip unknown locally": same change (11, `remote_moved_since_s1` for r2, r1 untouched).
- Golden-false kept: hand commit with NO remote moved is still 20 naming it (test 2 and 13b).

## Evidence ($EV/wp04)

- `wp04f-red.txt`: failing lines of the round-6 cases against the PRE-FIX helpers: push_recursive 27, scope_check 6, check_no_ci 1, check_revision_headers 1, integrate_merge 4 (hashes of the pre-fix copies at the head).
- `wp04f-green.txt`: all 13 WP-04 test files, 3 sequential runs, 39 of 39 rc 0 and 0 FAIL; counts: check_no_ci 38, check_revision_headers 29, scope_check 70, validate_cheap 106, commit_recursive 58, push_recursive 153, integrate_merge 125, integrate_ff_only 80, record_deferral 26, check_classes 131, fixture_roots 30, wp04c 231, wp04d 124; the evidence-root guard prints PASS after each run. sha256 of every helper and test at the head.
- `wp04f-mutation.txt`: 44 mutants of the round-6 code (WP-04, WP-03, WP-02 parts in one driver, `scripts/repo/tests/run_wp04f_mutations.sh`), control run per test file passes, 41 CAUGHT, 3 EQUIVALENT (stated), 0 survived. A first run had 8 survivors; five got new test cases (F8, V14, V15, W5, S4), G5 is also run against `test_wp04d.sh` (it owns the type-change case), V10 was a badly built mutant and was rebuilt, H2 and V3 are equivalent.

## Owed (not done here)

1. **F2 (owner decision, open)** `known_tip` is stale after an S1 fast-forward because S1's fetch writes no tracking ref. W6-1 makes S6 conservative around it; the clean fix is still one of: S1 writes `refs/remotes/<r>/<branch>` after a successful fetch (changes the T032 "no ref written" form), or S6 takes the stand-in from S1's recorded per-remote tips (the S1 report already carries `remotes[{name,fetch,tip}]`; persist it in the run directory). **Residual of the conservative rule:** a genuinely unrecorded commit is reported 11 instead of 20 while any remote has moved, until S1 is re-run and the moved tips are fetched. `wp04e-notes.md` F1 says "never a false 20"; read it with this note (the false 20 was closed only while the tracking ref was current).
2. **W6-10 (owner, docs)** a hand commit with one reachable unmoved remote and one moved remote without a tracking ref was 20 and is now 11 (covered by 13b/13c); the header describes it. Confirm the 11.
3. **F4 (Y2, Y3)** still listed for the next round (unchanged from `wp04e-notes.md`).
4. **Other pipe-status sites, UNCONFIRMED consequence** found while grepping `git ... | ...` and not changed: `push_recursive.sh` `trailer()` (a failing `git log` reads as "no Awaits-Review", but the same `git log` already failed `cpa_runid` for that commit, so it was refused earlier); `integrate_merge.sh` lines about `rev-list ... | grep -qx`, `merge-base` helpers, `diff-tree | head`, `show | sha256sum`; `integrate_ff_only.sh` `submodule status | awk`, `rev-list | head`, `ls-files -s | awk`, `diff-tree | head`. No trigger was demonstrated; each is the F3 class and needs its own failing-git case.
5. **Check-then-use race** in `scope_check.sh` and `commit_recursive.sh`: `gitlinks_ok` and `gitlinks` are two separate listings (`push_recursive.sh` now uses one listing). Not demonstrated, not changed.
6. **W6-11 (docs-evidence)** the hashes of `verify_repos.sh` and `test_verify_repos.sh` in the older `wp04e-green.txt` header are stale; the new `wp04f-green.txt`, `$EV/wp03/round6-green-*` records the current ones. The older file was not edited.
7. The older mutation drivers (`run_wp04_mutations.sh`, `run_wp04b/c/d_mutations.sh`) were NOT re-run (hours): UNCONFIRMED that every older mutant still dies, in particular the W4 mutants of `run_wp04c_mutations.sh` after the changed expectation above.

## UNCONFIRMED

- A real-git trigger in which `rev-list` fails but the following push would succeed (the evidence is the rc-128 shim model).
- Behaviour under a mawk-only image for the changed helpers: the container leg was run for the verify and audit suites only (`$EV/wp03`, `$EV/wp02`), not for the WP-04 helper suites.
- No `pgrep -f` or `pkill` was used by any helper, test or driver of this round. No git stage or commit was made; `tasks.md` was not edited.
