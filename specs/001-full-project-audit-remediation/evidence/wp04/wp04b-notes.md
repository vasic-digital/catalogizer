# WP-04 second slice: stage helpers T040 / T040b / T041 that need no container and no cpa-host

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-05 |
| Last modified | 2026-10-05 |
| Status | host run (bash 5.3.9, git 2.53.0, python3 with PyYAML and jsonschema, jq 1.8.1); nothing staged, committed or pushed; every test uses throwaway repositories and local bare remotes only |
| Follows | `README.md` of this directory (first slice) |

## Files

New under `scripts/repo/`: `lib_safe.sh`, `check_no_ci.sh`, `check_revision_headers.sh`, `scope_check.sh`, `validate_cheap.sh`, `validate_checks.tsv`,
`commit_recursive.sh`, `push_recursive.sh`, `integrate_merge.sh`. New under `scripts/repo/tests/`: `lib_wp04b.sh`, `test_check_no_ci.sh`,
`test_check_revision_headers.sh`, `test_scope_check.sh`, `test_validate_cheap.sh`, `test_commit_recursive.sh`, `test_push_recursive.sh`,
`test_integrate_merge.sh`, `run_wp04b_mutations.sh`, `wp04b_evidence.sh`. New: `docs/scripts/check_classes.md`. Evidence here: `wp04b-red-first.txt`,
`wp04b-red.txt`, `wp04b-green.txt`, `wp04b-mutation.txt`, `pending-precedence-mutation.txt`, this file. Existing files were not touched.

## Argument-injection policy (applies to every helper above)

`lib_safe.sh` holds the validators. Every value that comes from a command line, a list file, a registry row, `resolution.json`, a remote name or URL, a
branch, a declared path or a gitlink is validated before it reaches git or a shell, and a path or ref operand follows `--` (or is a validated 40/64-hex
object name, or the `HEAD:` form). Refused with exit 20: a path that is absolute, holds `..`, starts with `-`, holds a control character or a backslash;
a remote name that is not `[A-Za-z0-9][A-Za-z0-9._-]*`; a remote URL that starts with `-` or uses `::` (`ext::` runs a command; `GIT_ALLOW_PROTOCOL` is also
pinned to file:ssh:git:https); a branch that `git check-ref-format --branch` refuses or that starts with `-`; a run id outside `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`.
No registry command is ever run through a shell: `builtin:<name>` or one safe relative script path plus the closed placeholders `{root}`, `{files}`, `{paths}`.
`check_class.sh` (not changed here) takes no `--`; callers give it only `safe_relpath` paths.
Two guards are redundant on purpose: a repository named `-x` is refused by the option-like-value check and, if that check is removed, again by `cd -x` failing
(the five mutants that remove only the first layer are listed `EQUIVALENT`, not claimed caught).

## Status per helper (done / partial / blocked)

| Helper | Status | What is built | What is NOT built (named) |
|---|---|---|---|
| `check_no_ci.sh` | done | the 7 root-anchored forms of 11.4.156, tracked + untracked-unignored files, several `--root`, nested needle (`.specify/extensions/superspec/.github/workflows/ci.yml`) stays 0 | own-organisation repository discovery (the caller names the roots; T090 AM-G2 will call it) |
| `check_revision_headers.sh` | partial | the header rule (table rows / both bold forms, first 40 lines, both fields), class gating through `check_class.sh`, `--measure` (baseline rows) | the baseline files `validate_baselines/revision_header.tsv`, the ratchet stage, `baseline_lowering_owed` / `baseline_drift`, measurement over the own-organisation repositories |
| `scope_check.sh` | partial | build output by `git check-ignore --no-index`, databases, `.env` files, append-only stores (HEAD must be a byte prefix), blob names, dirty submodules at every depth with the exceptions rows (path, kind, file, wt hash) | the S2 secret fold and private-key carriers (T040a: `detect-secrets` absent on the host, `.secrets.baseline` not built), the class-based per-path S2 checks, the held-table S2 side |
| `validate_cheap.sh` + `validate_checks.tsv` | partial | registry reading, `plain` / `deferred` modes, 7 builtin checks run check-only on the host, 4 script checks, class application per file with HEAD tables / declared tables, `class_table_unreviewed`, `table_admits_unheld_path` (20), `legacy_row_not_dropped`, `check_pending_release` and the exit precedence 20 > 10 > 14, `size_alarm`, binary files left out, loader validation (`check_unknown`) | `ratchet` mode (a ratchet row is refused 20 `ratchet_not_implemented`), baselines, `--measure`, `--check` catch-up, the pinned-container run (images are recorded, not used), `hook-filters.json` (filters are suffix/text rules, UNCONFIRMED against the hook manifests), the moved-legacy-file rule, the OD-76 blob rule |
| `anti-bluff-scan.sh --files-from` | not done | nothing | `scripts/anti-bluff-scan.sh` does not exist (the tracked scanner is `scripts/audit/anti-bluff-scan.sh`, 5 `find .` walkers, no `--files-from`); `scripts/audit` is outside this task's allowed paths, so the mode was not added; the registry row `anti_bluff` is `deferred` |
| `commit_recursive.sh` | done (one window UNCONFIRMED) | one repository, explicit paths (`add -A --` + `commit --only --`), message from a file, trailers (CPA-Run, Deferred-Gates, Awaits-Review, Foreign-Commit on the first commit, ancestors only, never twice), unheld commit first then one per verdict, row appended right after each commit, `path_in_submodule`, SIGKILL after commit 1 leaves its row | the window between `git commit` returning and the row append (a kill there loses the row; closing it needs a `prepare-commit-msg` marker, T042); a gitlink path itself is accepted as an ordinary path (pin move), UNCONFIRMED |
| `push_recursive.sh` | partial | per repository and remote the longest releasable prefix, `ls-remote` tips, ancestry check, `remote_moved_since_s1` (11), per-remote failure recorded, holds read from the main HEAD with schema validation, unreleased merge blocks the repository, `unrecorded_local_commit` (20), force-like options refused, deepest first (`--recursive` from gitlinks) | the "no GO committed in HEAD lists it" clause of the CPA predicate, `--json`, exit 14 for holds and 11 for a rejected push are my reading of T042 (UNCONFIRMED) |
| `integrate_merge.sh` | partial | targets (diverged tips, adoption rule), newest-target rule, pairwise `merge-tree` (`remotes_diverged`), working-tree blocks (`ff_blocked_by_local_changes`), 9.2 backup (bundle + verify + worktree copy with sha256), `merge.json` with pid and start time, `--no-ff --no-commit` merge, commit with trailers and row, held decision (local held commit, G-GATE content, admission table by a non-CPA commit, adoption), conflict handling (copies with markers, abort, state check, 12), all `--resolution` refusals | a completed resolved merge: after every check passes the helper exits 20 `secret_fold_unavailable` because the S2 secret fold is not built, so no resolved file is ever written; the `resolution.json` field names are my reading (UNCONFIRMED); the `Foreign-Commit` list needs `--anchor` (adoption.json is T047's) |
| held-table rules | done in `validate_cheap.sh` | `class_table_unreviewed`, `table_admits_unheld_path`, `legacy_row_not_dropped` | the S2 side |
| `docs/scripts/check_classes.md` | done | purpose, usage, resolution, class matrix, held-table rules | |

## UNCONFIRMED decisions (for the T042 owner and review)

1. S2 refusals exit 13 in `scope_check.sh` (the task names 13 for the append-only and blob refusals and for the secret fold only).
2. `.env.example`, `.env.sample`, `.env.template` are allowed; every other `.env` / `.env.*` is refused.
3. Dirty-submodule exceptions are matched per file with the file's current sha256 (`deleted` for a removed file); a stale hash or another file refuses.
4. `push_recursive.sh`: exit 11 also for an unreachable remote and a rejected push; exit 14 when only holds withheld commits; the CPA predicate needs only the sha in the run's `commits.tsv`
   (the repository column is not compared); a verdict releases a commit when its `covers_runs` holds an entry with the commit's run id as `cpa_run` (the schema's entries are objects
   `{repository, commit, cpa_run}`; matching `commit` and `repository` as well would be stricter).
5. `integrate_merge.sh`: the held decision for a local range counts commits missing from at least one live remote tip; `--anchor` stands in for the adoption anchor; a range with no commit beyond a
   tip has no bundle (`backup/<key>.bundle.empty`); `--resolution` field names (`tip`, `paths`, `model`, `effort`, `files`, `method`).
6. `validate_cheap.sh`: the table change is "held" when its verdict is declared in the change set (not GO) or named by a `--held-from` row; a missing registry key runs only with `--run-declared <check>`.
7. The class loader is called per file (a process per file); a full-repository `--measure` over about 2,500 Markdown files is therefore slow (not run here).

## Reproduce

```bash
bash scripts/repo/tests/test_<helper>.sh                       # one test (RED = helper absent or H=/nonexistent/x)
bash scripts/repo/tests/wp04b_evidence.sh red|redfinal|green   # the recorder that wrote the red/green files
bash scripts/repo/tests/run_wp04b_mutations.sh [out] [id-prefix]  # control run + one edit per mutation on a COPY
```

## Results

| Item | Result |
|---|---|
| First RED (`wp04b-red-first.txt`) | the six first tests run while NO helper existed: 6 x exit 1, 284 `FAIL` lines (`integrate_merge` and several later assertions were added afterwards) |
| Final RED (`wp04b-red.txt`) | every FINAL test with `H=/nonexistent/<helper>`: 7 x exit 1, 404 `FAIL` lines; 71 assertions that only observe the absence of an effect (nothing pushed, no row written) hold vacuously, as in the first slice |
| GREEN x3 (`wp04b-green.txt`) | 21 runs (7 tests x 3, fresh throwaway trees each), 21 x exit 0, 0 `FAIL`; assertions per run: check_no_ci 30, check_revision_headers 25, scope_check 50, validate_cheap 106, commit_recursive 55, push_recursive 92, integrate_merge 117 (475) |
| Mutations (`wp04b-mutation.txt`) | 145 listed: 140 CAUGHT, 5 EQUIVALENT (documented, second guard layer), 0 unexplained survivors, 0 errors; control run: every test passes on an unmutated copy |
| Pending-row precedence (`pending-precedence-mutation.txt`) | the `max(10, 14)` exit computation is CAUGHT by the fixture that expects 10 |
| Identity | each section header of the red/green files carries the sha256 of the helper (or `ABSENT`), the test and `lib_safe.sh`, git HEAD (`a27d72a5`, working tree) and tool versions; `wp04b-mutation.txt` lists the sha256 of every final file |

Survivors found and fixed on the way (all recorded in `wp04b-mutation.txt`): the tests did not distinguish the pre-merge refusals from git's own refusal (run-dir writes now asserted), a
valid bundle always verifies (a git shim now makes `bundle verify` fail), the resolution-directory test used a non-existent directory (an existing one outside `.audit/merge-resolution` is used now),
the end-of-file mutant that rewrote the source was hidden by an earlier run (a tree checksum now brackets every single-file case), a foreign-commit test could not see "first commit only"
(a second, not yet named commit is offered to the second commit), `-n` / `a/../b` paths did not exist (real files of those names are declared now), `.env` was git-ignored in the fixture,
`merge_conflict` was not mutated completely, and `P11` (ancestry) was blind for a remote tip that is held locally but diverged (case 6b added).
Also fixed while testing (real defects of my own code, each found by its test): the release check read `covers_runs` as strings (the real schema has objects
`{repository, commit, cpa_run}`), the merge held-decision used `HEAD^1..` before the merge commit existed, `git rev-parse --git-path` is relative to `-C`, and a wrong depth sort in the
deepest-first order.
