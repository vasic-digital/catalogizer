# WP-04 round 5 (wp04e) notes: fixes for the WF5 review of the WP-04 helpers

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | round 5 fix evidence; independent review of this change owed |
| Source | the WF5 review WF5-REVIEW-wp04-helpers (scratchpad copy), findings F1 to F5 |

## Fixed (test first: RED, then GREEN three times)

- **F1 (important)** `push_recursive.sh`: a live remote tip that is not held locally (the remote moved after S1; fetch is not allowed in S6) was silently left out of the known set, so `rev-list` had no exclusion and S6 named already-published commits as `unrecorded_local_commit` (exit 20). Now: the remote is marked moved; its last fetched tip stands in; with no such tip the outgoing set is not computable (UNK). Outcome: exit 11 with `NOPUSH <repo> <remote> remote_moved_since_s1` for each moved remote, never a false 20. A real hand commit still gives 20 (golden-false), naming only that commit. Cases added to `test_push_recursive.sh` section 10 (all remotes moved, one moved, hand commit plus all moved, all moved with no tracking ref).
- **F3 (minor, real defect)** a failing git listing no longer reads as clean. `check_no_ci.sh` (ls-files), `scope_check.sh` (gitlink listing and per-submodule status), `commit_recursive.sh` and `push_recursive.sh --recursive` (gitlink listing; through new `gitlinks_ok` in `lib_safe.sh`) refuse 20 with `git_listing_failed` or `git_status_failed`. Same class found while fixing and fixed: `check_revision_headers.sh --measure` (ls-files). Cases use a failing git shim (rc 128) in the five test files, each with a real-git control.

## Evidence ($EV = specs/001-full-project-audit-remediation/evidence)

- `$EV/wp04/wp04e-red.txt`: failing lines before the fixes (F1: 8 FAIL; F3: check_no_ci 3, scope_check 4, push_recursive 3, commit_recursive 3, check_revision_headers 2).
- `$EV/wp04/wp04e-green.txt`: all 13 WP-04 test files, 3 sequential runs, 39 of 39 rc 0 and 0 FAIL; counts: check_no_ci 35, check_revision_headers 27, scope_check 55, validate_cheap 106, commit_recursive 58, push_recursive 109, integrate_merge 117, integrate_ff_only 80, record_deferral 26, check_classes 131, fixture_roots 30, wp04c 230, wp04d 124. The evidence-root guard `scripts/governance/tests/test_evidence_root.sh` prints PASS. sha256 of every helper and test is at the head of the file.
- `$EV/wp04/wp04e-mutation.txt`: 9 mutants of the new code (F1 a, b, c; F3 a to f), 9 CAUGHT, each with a passing control run on the unmutated copy. The older mutation drivers were NOT re-run (they take hours): UNCONFIRMED that every older mutant still dies.

## Owed (not done here)

1. **F2 (owner decision)** the `known_tip` stand-in is stale in the normal CPA flow: S1's fetch uses `--refmap=` and writes no tracking ref. Options for the owner: S1 writes `refs/remotes/<r>/<branch>` after a successful fetch (changes the T032 "no ref written" form), or an unreachable remote whose known tip does not cover a non-CPA commit gives 11 (cannot compute) instead of 20. The `wp04d-notes.md` description of the trigger ("a push from another clone") should be corrected either way. Not touched.
2. **F4 (mutation adequacy)** Y2 (`validate_cheap --trusted-tables` falling back to the working tree for a table missing from the approved directory) and Y3 (`commit_recursive` repository-root guard removed; `--repo <repo>/<subdir>` commits the parent's files in the parent) SURVIVED the reviewer's mutants. Golden-false cases the reviewer proposed (a trusted-tables directory lacking `check_exemptions.tsv` while the working tree holds an exempting row: expect 10 or 20 `trusted_table_missing`; `commit_recursive --repo <repo>/<subdirectory>` refuses 20 with the parent HEAD unchanged) are NOT added: Y2 and Y3 are listed for the next round (the Y2 outcome, 10 or 20, is an owner choice).
3. **F5 (docs-evidence)** the `test_wp04d.sh` sha256 recorded in the earlier `wp04d-green.txt` and `wp04d-mutation.txt` (2a0247ca...) is stale; the file now hashes to 58fd484e64337b86585bdfbbef642f613b219a95b45ba19a342aafc72bc9476c (only the `m9` label wording changed per the reviewer; 124 of 124 pass in all three runs above). The older files were not edited here.
4. Owner and docs items from `wp04d-notes.md` owed items 1 to 7 are unchanged. `docs/scripts/check_classes.md` needed no change.

## UNCONFIRMED

- Whether `scope_check.sh` and `check_no_ci.sh` behave the same under a mawk-only container: not re-run in a container this round.
- No `pgrep -f` or `pkill` is used by any helper, test or driver of this round.
- No git stage or commit was made; `tasks.md` was not edited.
