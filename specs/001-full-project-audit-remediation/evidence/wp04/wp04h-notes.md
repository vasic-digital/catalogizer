# WP-04 round 8 (wp04h) notes: fixes for the final finding list of the WF7 review `WF7-REVIEW-wp04-verifier-r6` (round 8 brief)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | round 8 fix evidence; independent review of this change is owed (the author is not the reviewer, constitution 11.4.142). Review effort level is not self-reportable from this session. |
| Source | the round 8 brief (authoritative finding list: I-3, M-6, M-7, M-8, MA-1..MA-3, E-1..E-4, INFO-1, the remaining `git remote` sites), `wp04g-notes.md`, `$EV/wp03/round7-notes.md` |

`$EV` = `specs/001-full-project-audit-remediation/evidence`. Records (sha256 identity headers inside): `wp04h-red.txt`, `wp04h-green.txt`, `wp04h-mutation.txt`, `$EV/wp03/round8-*`.

## Fixed (test first: RED against the pre-fix helpers, then GREEN three times)

| Id | Change | Test |
|---|---|---|
| I-3 (important; the round 7 notes recorded it as owner decision 1, which was WRONG: docs/16 S6 rule (b) already requires it) | `push_recursive.sh`: before a push of target T to a remote, every gitlink of T's tree that the remote's tip does not already carry (all of them for a remote without the branch) must be held by EVERY remote of that submodule: read live with `git ls-remote` after the deeper pushes, held = a branch or tag tip equals the pin or descends from it. Otherwise `NOPUSH <repo> <remote> submodule_commit_not_held <path> <sha> (<why>)`, exit 11, nothing pushed for the parent. Conservative cases (all "not held"): submodule not initialised, no remote, unsafe or unreachable remote, a remote tip whose objects this clone does not hold. A failing gitlink listing of the target or of the remote tip is `REFUSED git_listing_failed` 20. | `test_wp04h.sh` I-3a..I-3k: the p7b repro (REFUSED hand commit: main NOT pushed), a HELD sub commit (exit 11), sub remote unreachable, no `--recursive`, no remote, uninitialised; golden-false: every sub commit pushed in the run (parent pushed, fresh `clone --recurse-submodules` works), a pin the sub remote already holds, a gitlink the remote tip already carries (unrelated parent change pushed), a pin that is an ancestor of the remote tip, a pin held by a tag only |
| M-6 | `verify_repos.sh`: once the arguments are parsed, `die20` also removes the `--json FILE` of an earlier run (regular file or symlink itself; never a device, never the target of a link); a bad argument exits before any run and removes nothing | `test_wp04h.sh` M-6 (stale file, dangling and file symlinks, /dev/null, unknown option, bad `--jobs`, golden-false good run overwrites) |
| M-7 | `scope_to_lumen_json.py`: a `baseline_excludes` class named `third_party_nested` is exit 3 (it was overwritten; `--check` passed on the re-derived loss) | `test_scope_to_lumen_json.sh` M-7 (derive, `--check`, golden-false) |
| M-8 | `--check`: a duplicated or reordered `classes` list entry (same members, different list) is exit 1 `duplicate or misplaced entries`, like the list keys | `test_scope_to_lumen_json.sh` M-8 |
| remaining `git remote` sites | `integrate_ff_only.sh`: `vet_remotes`, the submodule fetch loop, the R4 pin loop, `unrec` (in-band `%git_listing_failed` marker, it runs in a command substitution) and the owned-submodule loop each read the remote list with its status; a failing call is exit 20 `git_listing_failed` | `test_wp04h.sh` M-3: a counter shim fails the k-th `git ... remote` call, k = 1..8, each must give 20 / `git_listing_failed` (RED: 12 of 20 assertions failed against the pre-fix copy) |
| INFO-1 (confirmed by the repro) | `integrate_merge.sh` with every remote unreachable (`ls-remote` or the object fetch failed for all) was `nothing_to_merge`, exit 0: now exit 11 `remote_unreachable`, as `integrate_ff_only.sh`. One remote that answered is enough to proceed (stated; a partial outage still prints `fetch_failed:<r>` on stderr and is not an error). | `test_wp04h.sh` INFO-1 (ls-remote failing, fetch failing via shim, one reachable, no remote at all) |
| MA-1..MA-3 | The fixtures already existed from round 7 (`test_verify_repos_r7.sh`: not-held tip of a REMOTE-BEHIND pin, `filter.<n>.process`, pin held only by a tag). Round 8 maps them: `run_wp04f_mutations.sh` VR group now runs the r6 AND the r7 file, new mutants V16 (RM2), V17 (RM3), V18 (RM4), each CAUGHT by the r7 file | `wp04h-mutation.txt` |
| E-1 | `wp04f-notes.md` pointed at `$EV/wp03/round6-notes.md`, which never existed: repointed honestly to the guides and records | edit |
| E-2 | The round-6 mutation records (`wp04f-mutation.txt`, `wp03/round6-mutation.txt`, `round6-red-verify-repos.txt`) ran against `test_verify_repos_r6.sh` sha `53b76cb3`, the committed file is `d934db91` (modified after the capture). Not re-captured: the WF7 reviewer re-ran V1..V15 against `d934db91` (all caught) and this round re-ran the whole f driver against the current r6+r7 files (`wp04h-mutation.txt`, 47 listed). | stated |
| E-3, E-4 | `docs/scripts/verify_repos.md`: r6 41 cases and r7 22 cases (both re-measured this round, `wp04h-green.txt`), the required-filter exit-20 sentence added, revision 10; `scope_to_lumen_json.md` revision 7 | docs |

## Results (verbatim numbers; the files hold the raw output)

- RED `wp04h-red.txt`: `---- 42 ok, 37 failed` against the pre-fix helpers. `round8-red-scope-to-lumen-json.txt`: `SUMMARY pass=55 fail=5`.
- GREEN x3: `test_wp04h.sh` `---- 75 ok, 0 failed` three times; `test_scope_to_lumen_json.sh` `SUMMARY pass=60 fail=0` three times; container IMG-TESTUTIL (git 2.39.5, mawk): test_wp04h 75/0, test_scope_to_lumen_json 60/0, test_push_recursive 153/0.
- All suites, one sequential run on the fixed tree, all rc 0 (`wp04h-green.txt`): check_no_ci 38, check_revision_headers 29, scope_check 70, validate_cheap 106, commit_recursive 58, push_recursive 153, integrate_merge 130, integrate_ff_only 84, record_deferral 26, check_classes 131, fixture_roots 30, wp04c 231, wp04d 124, wp04g 30, wp04h 75, verify_repos_r6 41, verify_repos_r7 22, verify_schema 14, derive_scope 9, scope_to_lumen_json 60, index_health 13, index_health_extra 54, anti_bluff_scan_wrapper 45, verify_repos (full, separate run) 404.
- Mutation: `run_wp04h_mutations.sh` 24 listed, 0 not caught in the final run (H1-H9 push_recursive, FF1-FF6 ff_only, IM1-IM3 merge, VM1-VM4 verify, ST1-ST2) plus FF7 caught afterwards. Honest history: the FIRST h run had 2 survivors, IM3 (a failed fetch not counted) and VM2 (symlink cleanup): the tests were extended (every-fetch-fails shim case, dangling symlink case) and both are caught now. `run_wp04g_mutations.sh` 20 listed: M2 survived because the new `vet_remotes` answers first (a second layer); the g driver now also runs `test_wp04h.sh` for the FF group (counter shim k=2) and M2 is CAUGHT; EQ-K4 stays EQUIVALENT (documented). `run_wp04f_mutations.sh` 47 listed: V10 survived because the M-6 cleanup removes a report written before an exit-20, so the end state is identical: re-classified `EQ-V10` with the reason (EQUIVALENT in the run); V16-V18 CAUGHT.

## Owner decisions (recorded, NOT decided)

- **O-1** filter policy: with `required=false` real git-lfs content still reads false-dirty (13); alternative: honour global/system-scope drivers and override only repository-scope ones. Security trade-off.
- **O-2** whether `--fetch` fetches the undecided candidate tips (objects only) so the pin question is decided instead of 14.
- **O-3 / W6-10** confirm exit 11 for "moved remote plus hand commit"; in that branch the reachable unmoved remotes print no line and the unrecorded commits are not named.
- (changed in this round) the round 7 owner decision 1 (main pushed past an unpublished submodule pin) is NOT an owner decision: fixed as I-3 because docs/16 S6 rule (b) already mandates it. Owner decision 3 (stale `--json` file) is fixed as M-6 (remove on exit 20; the alternatives "remove at start" and "refuse to overwrite" were not chosen).

## Owed / UNCONFIRMED (stated, not hidden)

- M-8: the review text says "duplicates rc 3 as the docstring promises"; the docstring never promised 3 for a duplicate and the list keys already give exit 1, so classes lists give exit 1 too. UNCONFIRMED against the reviewer's intent; a one-line change to 3 if the owner wants it.
- I-3 conservatism: a remote advertising a tip this clone does not hold, an uninitialised or remote-less submodule, and a pin held only by refs outside `refs/heads` and `refs/tags` (e.g. `refs/pull/*`, which a protocol-v2 server may still serve) all withhold the parent (11). A false withhold is recoverable (fetch, re-run); a false publish is not.
- `pin_held` accepts only 40-hex object names (`safe_sha`), so a SHA-256 object-format repository cannot be held (as the verifier, pre-existing, UNCONFIRMED relevance).
- Not re-run: `run_wp04_mutations.sh`, `run_wp04b..d_mutations.sh` (hours; their `old` texts: E-5 of the review says P9/D1/D2/D4/E5 no longer find their text: still owed, the ports PD1/PD4/PE5/PP9 were caught in the WF7 review).
- SIGINT legs of W6-5 void (background job ignores SIGINT): an interactive-terminal SIGINT run is owed. `derive_scope.sh` duplicate `classes.build` entry UNCONFIRMED consumer.
- The mawk behaviour of the verify suites was covered in earlier container legs; this round's container leg ran test_wp04h, test_scope_to_lumen_json and test_push_recursive.
- Other agents' uncommitted edits were present in the tree (verify_repos.sh, index_health, etc.); this round edited only the files listed in the brief. No git stage, commit or push; no `pgrep -f`; no signal sent; no credential.
