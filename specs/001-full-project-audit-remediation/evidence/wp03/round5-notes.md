# WP-02 / WP-03 round 5 notes (fix of WF5-REVIEW-wp02-wp03)

| Field | Value |
|---|---|
| Date / host | 2026-10-06, host `anton`, git 2.53.0 (container: git 2.39.5, mawk, jq 1.6), jq 1.8.1, HEAD a27d72a5 (unchanged), nothing staged or committed |
| Scope touched | scripts/repo/verify_repos.sh, scripts/repo/tests/test_verify_repos.sh, scripts/audit/index_health.sh, scripts/audit/scope_to_lumen_json.py, scripts/audit/tests/{test_index_health,test_index_health_extra,test_scope_to_lumen_json,mutate_index_health}.sh, docs/scripts/{verify_repos,index_health,scope_to_lumen_json}.md, evidence/{wp02,wp03}/round5-* |
| Identity | every round5-* file carries a `# sha256` header of the files it judged (ev.sh helper) |

## Fixed

| Id | Fix | Tests (RED first, then GREEN x3) |
|---|---|---|
| R1 | `core.hooksPath=/dev/null` is part of `$GIT` (every git command of the verifier) and explicit on the fetch. Header and docs now state it. | section 39 `q5r1`: live `reference-transaction` hook + post-checkout/post-merge/post-rewrite/pre-commit markers; control that a plain fetch DOES run the hook. RED: hook ran (`prepared committed`). On git 2.39.5 (container) a ref-less fetch runs no such hook, so the control degrades to an explicit SKIP-with-reason and the two absence lines are labelled VACUOUS there (the host run on git 2.53 is the live proof). |
| R2 | `cnt()` checks jq status and that the answer is a non-negative integer; every use is `|| die20`. | `q5r2*`: jq shim failing ONLY the exit-map filter (rc 5, and rc 0 with `null`): no-remote 14 -> was 0, dirty 13 -> was 0/14; now exit 20 with a message. |
| R3 | An attached owned submodule's pin is HELD (not reported) when any remote branch or tag tip (`ls-remote refs/heads/* refs/tags/*`, live) equals it or descends from it; otherwise the class stays. Classification decision: pin reachable from some remote ref = held; a pin no remote ref holds keeps DIVERGED 12 / REMOTE-BEHIND 11 / UNKNOWN-DIFFERENT 14. | `q5a` (other branch tip, plain/--strict 15/--fetch), `q5b` (ancestor of another branch tip), `q5c` (tag only, branch deleted), with a real fresh-clone oracle (rc 0); M5-1 fixtures `q5d` (diverged, held by none: 12, fresh clone really fails `not our ref`) and `q5e` (pin object unknown locally, held by none: 14). |
| R4 | `index_health.sh` P3: a third-party root line that is not canonical (control char incl. CR, surrounding whitespace, leading `/` or `./`, empty/`.`/`..` component) FAILs P3 with `third_party_root_not_canonical`; one trailing `/` and blank lines still accepted. Decision: FAIL row, not exit 2 (same shape as the own-org leg). | `test_index_health_extra.sh` R4 x9 variants + 3 controls. |
| D1 | `scope_to_lumen_json.py --check`: a non-string element inside a `classes` list is rc 3, no traceback (was TypeError rc 1). | `test_scope_to_lumen_json.sh` round5 D1 x3. |
| D3 | SC2054 false positive in test_index_health.sh carries a stated `shellcheck disable`; shellcheck -S warning on all area files is clean (IMG-SHELLCHECK, round5-shellcheck.txt). | |
| M5-1, M5-2, M5-3 | Fixtures added (see `q5d`, `q5e`, X-section M5-2, M5-3). | `round5-mutation.txt`, `wp02/round5-t021-mutation.txt` (31 mutants, 0 survived; m26-m31 are new). |
| D2 | All round-4 legs re-recorded on the FINAL files: host x3, container, mutation, shellcheck (round5-*). | |

## Mutation results (round5-mutation.txt, wp02/round5-t021-mutation.txt)

All caught except one stated equivalent: **R1b** (remove only the global `core.hooksPath` from `$GIT`, keeping the explicit one on the fetch) SURVIVES. It is equivalent for every command the verifier runs today (only `fetch` can run a hook; status/diff-index/ls-files/ls-remote/rev-parse do not), the global flag is defence in depth; a test cannot kill it without a verifier command that runs a hook. A jq-status-only mutant of `cnt()` (numeric check kept) is equivalent too (an empty answer fails the numeric check); not listed.

## Owed (not done, recorded)

- `scripts/audit/tests/mutate_wp02_wp03.sh` (the author runner) was NOT extended with the round-5 mutants; they ran as a separate set (round5-mutation.txt). Its DRYRUN pattern check was run on the final verifier: 81 caught / 0 survived / 0 broken (round5-mutate-runner-dryrun.txt).
- Mutation-adequacy for the new fixtures on the container git (2.39.5) was not run (host only).
- kcov/PS4 line coverage was NOT re-measured on the final test file (round4-coverage.txt is for the round-4 file). UNCONFIRMED whether the 85.6 percent figure still holds (the new section adds executed lines; the new verifier lines are all exercised by section 39 by the mutation results above).
- `wp02/t021-*.txt` (round-4 era) still have broken identity headers (empty sha256); superseded by round5-t021-mutation.txt and round5-green-*; the old files were not edited.
- O1 (T021 against its task text: `--out` required, P4-P6 SKIP, `complete:false`, `--lumen-bin` FAILs instead of probing): owner item, unchanged, disclosed in docs/scripts/index_health.md.
- docs/scripts/README index and root README link remain deferred (tracked files).
- Round-4 owed items (a), (b), m-d, m-i, m-k..m-n, m-10..m-12: unchanged.

## UNCONFIRMED

- (resolved) Real tree, plain, read-only: rc 13, summary identical to baseline-verify.json (98 repos, SAME 191, UNKNOWN-DIFFERENT 8, 1 unproven, 2 dirty / 1 excepted); the attached-pin held check adds no row (round5-real-tree.*).
- Behaviour against remotes that advertise a huge number of branches/tags (the held check reads all heads and tags once per owned attached submodule whose pin is not on the compared branch; bounded by `--timeout`, no cap on the tip count).
