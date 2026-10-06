# verify_repos.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 7 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06 round 6 (WF6 review N6-1 to N6-4, MR1, MR2: tail-match refs, undecided held check, report after counts, repository-configured programs); earlier: 2026-10-06 round 5 (WF5 review R1 hooks, R2 exit-map fail-closed, R3 pin held by any remote ref); earlier: 2026-10-05 (review fixes of WF-REVIEW-wp02-wp03: B1-B3, I1-I9, m1-m4; revision 3: remote-name, announced-default-branch and --show-toplevel test gaps closed, remotes read per line; revision 4: WF2-REVIEW round 3, N-B1, N-B2, N-I1 to N-I5, m-a, m-b, m-c, m-e, m-h); revision 5 (2026-10-06): WF3 round 4, I-1 to I-5 and the container portability root causes) |
| Status | draft, untracked work product of T032 and T036 (WP-03); the scripts index `docs/scripts/README.md` and the root `README.md` link are NOT created here (they edit tracked files, deferred) |
| Source | `scripts/repo/verify_repos.sh` |

## Purpose

The single recursive, read-only repository verifier. For the main repository and every submodule at every depth it reports whether the working tree is clean (stash included), whether the pin is in order, and, for repositories owned by the own-organisation list, how each remote's live tip relates to the local commit. It is the promotion of the spec 001 POC `poc/repo_verify/verify_repo.sh` (docs/21 IC-17, IC-37); it keeps the `repo-verification-report/1` JSON shape and the constant `"tool": "verify_repo.sh"`.

It observes only, including its own control needle (revision 4: the needle makes NO commit, runs with `core.hooksPath=/dev/null` and an empty template, and every inherited repository-selecting git variable is scrubbed first). It never commits, stashes, resets, cleans, merges or writes a branch ref, a remote-tracking ref, a working tree, `FETCH_HEAD` or `.git/index` (it runs git with `GIT_OPTIONAL_LOCKS=0`, so `git status` does not refresh and rewrite the index; a test asserts the index bytes and mtime are unchanged).

It fails closed. Every git command whose answer the verdict depends on has its exit status checked (`rev-parse`, `symbolic-ref`, `status`, `stash list`, `remote`, `merge-base`, the `.gitmodules` reads, and `git submodule status --recursive` itself). A repository that cannot be examined makes the run exit 20 with `verify_repos: cannot verify <path>: <reason>` on stderr and NO report is written: it is never read as clean. The number of result rows must equal the number of listed repositories (A-1: submodule status lines + 1) and each worker writes a collision-free file (an index, not a name derived from the path).

## Usage

```bash
scripts/repo/verify_repos.sh [--root DIR] [--json FILE | --json-out FILE] [--fetch] [--no-remote] [--strict]
                             [--jobs N] [--timeout SEC] [--owned-orgs a,b,c] [--exceptions FILE] [--quiet] [--self-test]
```

| Option | Effect |
|---|---|
| `--root DIR` | repository root (default `$PWD`) |
| `--json FILE`, `--json-out FILE` | write the report to FILE (aliases, byte-identical output once `generated_utc` is removed) |
| `--fetch` | decide ancestry by fetching objects only: `git fetch --no-tags --no-write-fetch-head --refmap= <remote> <branch>` |
| `--no-remote` | contact no remote; only dirty, pointer drift and uninitialised are decided; report records `no_remote: true` |
| `--strict` | adds `behind` and `pin` to `problems`; the `summary` counts are the same in both modes |
| `--jobs`, `--timeout` | parallel workers (default 6), per-remote timeout in seconds (default 25); each must be a positive integer, else exit 20 (`--timeout 0` would disable the timeout) |
| `--owned-orgs` | comma list overriding `scripts/audit/own_orgs.txt`; matched case-insensitively |
| `--exceptions FILE` | exception list (default `scripts/repo/exceptions.tsv`), see "Exceptions" |
| `--quiet` | no table; JSON to stdout when no file is given |
| `--self-test` | runs the control needle only, wherever it stands on the command line; the full matrix is `scripts/repo/tests/test_verify_repos.sh` |

`--root` must be the repository root itself; a subdirectory of a repository is refused with exit 20 (git would resolve it to the enclosing repository).

Environment: `VERIFY_GIT` names a git binary or shim (used by the tests).

## Round 3 (WF2-REVIEW) behaviour

- **Inherited git environment (N-B1).** The first thing the script does is `unset` every name of `git rev-parse --local-env-vars` plus `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`, `GIT_COMMON_DIR`, `GIT_NAMESPACE`, `GIT_PREFIX`, `GIT_CONFIG*` repository selectors and the numbered `GIT_CONFIG_KEY_n/VALUE_n`. A git hook exports these into everything it runs (in a linked worktree `GIT_DIR` is an absolute path), so without the scrub every `git -C <dir>` would act on the hook's repository. The old needle also committed into that repository and, inside a pre-commit hook, recursed (the reviewer measured 721 nested runs). Tests: sentinel repository byte-identical under `GIT_DIR`/`GIT_INDEX_FILE`/`GIT_WORK_TREE`/`GIT_OBJECT_DIRECTORY`/`GIT_COMMON_DIR`, a global `core.hooksPath` hook never runs, and a pre-commit hook that runs the verifier succeeds exactly once in the main worktree and in a linked worktree.
- **Staged gitlinks (N-B2).** `git status --ignore-submodules=all` hides a staged submodule pointer change and `git submodule status` compares the submodule HEAD with the INDEX, so `git add <sub>` read clean. The worker now also reads `git diff-index --cached --raw -z --ignore-submodules=none HEAD` and counts every entry with a 160000 mode on either side (changed, added, removed) as a tracked change (`dirty`, exit 13, never excepted).
- **Untracked files (N-I1).** `--untracked-files=all` is part of the one status invocation shared by workers and needle; a repository-level `status.showUntrackedFiles=no` is overridden and noted on stderr.
- **skip-worktree / assume-unchanged (N-I2).** `git ls-files -v` flags are listed; a flagged file present on disk is compared with its index blob (`git hash-object`), a difference is a tracked change. An ABSENT `S` (skip-worktree) file is a sparse checkout, not dirt; an absent assume-unchanged (lower-case tag) file is a deletion and is dirt. Never excepted.
- **Worse class wins (N-I3).** All 20 combinations of pin kind (SAME, LOCAL-BEHIND, REMOTE-BEHIND, DIVERGED, absent object) and HEAD kind (SAME, LOCAL-BEHIND, REMOTE-BEHIND, DIVERGED) are asserted against an independently written rank order.
- **Needle legs (N-I4, m-e).** One non-blind shim per leg: a git that hides only untracked entries, and a git that reports dirt on a clean tree; both end in exit 20.
- **Minors.** m-a the pinned-gitlink `ls-tree` exit is checked (exit 20); m-b exception rows are scoped by repository path and never apply to a modified tracked symlink (tests); m-c a remote URL the shared parser cannot classify is reported unproven (exit 14, `unproven: [NO-REMOTE-BRANCH]`, named on stderr) instead of silently third-party; m-h the real-tree agreement check with derive_scope uses a full outer join.

## Inputs

`scripts/audit/own_orgs.txt` (own organisations, one per line, case-insensitive; `helixdevelopment1` and `milos85vasic` are BLOCKED-ON ODG-15 and counted third-party, noted on stderr), `scripts/repo/exceptions.tsv`, the repository itself and, unless `--no-remote`, its remotes through `git ls-remote` (BatchMode ssh, no prompts). Needs git, jq, python3 and sha256sum.

The organisation of a remote URL comes from `scripts/audit/org_of.py`, the same parser `scripts/audit/derive_scope.sh` imports: the second-last path segment, lower-cased, tolerant of a trailing slash and a trailing `.git`; an unclassifiable URL (a relative path, no organisation segment) yields no organisation. A repository is OWNED when ANY of its remotes has an organisation in the list (derive_scope classifies by the `.gitmodules` URL and the nesting rule instead; a test asserts the two agree on every real submodule row, 97 today).

## Exceptions

`scripts/repo/exceptions.tsv` has one row per excepted file: `path`, `kind` (`dirty`), `file` (path inside the repository), `wt_sha256` (sha256 of the working-tree file), `blob_sha256` (sha256 of the file as committed in HEAD), `reason` (non-empty). A repository is excepted (`excepted: true`, `exception_reason` set, still counted in `summary.dirty`, exit 0) only when EVERY change it shows is an unstaged modification (` M`) of a file named by a row whose two hashes both match NOW and it has no stash and no untracked file. Anything else makes it an unexcepted dirty repository (exit 13): a stale hash (the file changed again, or HEAD changed it), an extra tracked or untracked change, a stash (never excepted), a staged change, a row of another `kind`, an empty reason, malformed hashes, or a legacy 3-column row. A row for a clean repository excepts nothing (`excepted` stays false). The hashes are not repeated in the report (the schema is closed), see `specs/001-full-project-audit-remediation/audit/owed-contract-items.md`.

## Outputs

JSON report (`repo-verification-report/1`, contract `specs/001-full-project-audit-remediation/contracts/repo-verification-report.schema.json`) with the mode booleans `fetch`, `strict`, `no_remote`, a `summary` and one row per repository; a table and the summary line on stdout unless `--quiet`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | clean (excepted dirty rows allowed and listed) |
| 11 | unpushed commits (`REMOTE-BEHIND`) |
| 12 | diverged, or a `behind` row under `--strict` |
| 13 | dirty, including an unexplained stash |
| 14 | unverified remote (`UNREACHABLE`, `UNKNOWN-DIFFERENT`, `NO-REMOTE-BRANCH`), both modes |
| 15 | pointer drift or uninitialised submodule, `--strict` only |
| 20 | blind verifier (control needle failed), usage or internal error, a report count (the exit map) that jq cannot read back or answers with a non-number (round 5, R2: before, an empty count fell through to a lower code or to 0) |

Several failing classes at once: 13, then 12 (diverged), then 11, then strict 15, then strict behind (12); a failing class always precedes 14. Every pair of the six classes is asserted by the test matrix. Decisions and reasons: `evidence/wp03/exit-code-decisions.md`.

Decision (b), stated plainly: a `LOCAL-BEHIND` remote is a fast-forward, NOT a divergence (no merge decision, no conflict). It maps to 12 under `--strict` only because the closed exit set has no code that fits better; 12 is therefore ambiguous between "diverged" and "behind" for a consumer that reads only the exit code. The report is unambiguous (`problems` has `diverged` or `behind`). Consequence on today's tree: `--fetch --strict` exits 12 for `submodules/constitution` while it is merely behind upstream. UNREVIEWED by the owner.

## What is compared

- Only OWNED repositories are compared with their remotes; third-party repositories (no organisation of the list on any remote) are never compared, their rows have `remotes: []`.
- A repository on a branch: HEAD against the remote tip of that branch, on every remote.
- A DETACHED owned submodule: the pinned commit (the gitlink recorded in its parent) AND its HEAD are compared against the tip of the `.gitmodules` branch, else the remote's default branch (`ls-remote --symref HEAD`), on every remote; the worse class (DIVERGED > REMOTE-BEHIND > UNKNOWN-DIFFERENT > LOCAL-BEHIND > SAME) is reported. So unpushed work past the pin is exit 11 on a detached submodule exactly as on a branch (previously only the pin was compared and such work passed a plain run). Without `--fetch` an unreachable-from-here tip object is `UNKNOWN-DIFFERENT` (exit 14), never guessed.
- An ATTACHED owned submodule (round 4, I-4 / docs/16 R4): HEAD against its branch tip as before, AND the pin recorded in its parent is checked for reachability: a pin that is AHEAD of the remote tip (`REMOTE-BEHIND`: exit 11), DIVERGED from it (12) or unknown to the local object store (`UNKNOWN-DIFFERENT`: 14) is the `not our ref` condition of a fresh clone and the worse class wins; a pin merely BEHIND the pushed tip is reachable and is not reported as a class (plain `pin` drift stays a `--strict` exit 15 matter). Round 5 (R3): a pin is also HELD, and not reported, when ANY remote branch tip or tag tip (read live with `ls-remote refs/heads/* refs/tags/*`) equals it or descends from it: a pushed commit on another branch is fetched by a fresh clone, which the test proves with a real clone (rc 0). Only a pin no remote ref holds keeps its class (DIVERGED 12, REMOTE-BEHIND 11, or UNKNOWN-DIFFERENT 14 when the pin object is absent locally and equals no remote tip). Before round 5 the pushed-on-another-branch case was a false DIVERGED/exit 12 (test section 39 `q5a`..`q5c`; the not-held cases `q5d`, `q5e` cover mutant M5-1). Oracle in the test: a real fresh clone of the parent fails with `fatal: ... not our ref`. Whether third-party pins should be checked for reachability is an owner question (they are never compared with a remote).
- The remote tip is the line whose ref field is EXACTLY `refs/heads/<branch>` (round 4, I-2): `git ls-remote <remote> refs/heads/<b>` matches the TAIL of every advertised ref, so a ref such as `refs/archive/refs/heads/<b>` was read as the branch tip (a false SAME). The default-branch path selects the exact `HEAD` line.
- Dirt is read with `core.fsmonitor=false` and `core.untrackedCache=false` on `status`, `diff-index --cached` and `ls-files -v` (round 4, I-3): a repository `core.fsmonitor` hook (or a stale monitor daemon) that reports "nothing changed" made a modified tracked file read clean. Test: a lying protocol-v2 hook (the control asserts that plain `git status` IS blind under it and that `-c core.fsmonitor=false` sees the change).
- A flagged (skip-worktree / assume-unchanged) file's index blob is looked up with literal pathspecs and a checked exit status (round 4, m-4): a name with glob metacharacters (`x[1]`) no longer selects a sibling (`x1`).
- A repository with no remote at all cannot be shown pushed: `unproven: ["NO-REMOTE-BRANCH"]` with `remotes: []`, exit 14 (`--no-remote` contacts nothing and so does not claim it).
- An uninitialised submodule (pin `-`) has an empty directory, which git would resolve to the PARENT repository; its row is reported as `uninitialised` with no head, not owned, not dirty, no remotes, and git is not asked about it.

## Values that reach git (argument injection) and the toplevel guard

- A branch (from `.gitmodules` or from the remote's `HEAD` symref) and a remote NAME are passed to git only after `valid_branch` / `valid_remote`: not empty, no leading `-`, no whitespace, accepted by `git check-ref-format`. A refused value is never sent to git; the row's class is `NO-REMOTE-BRANCH` (exit 14, unproven, never clean) and stderr names it (`refused the remote name ...`, `refused the default branch name ... announced by remote ...`). Every call also puts `--` before the remote and refspec.
- Remote names are read ONE PER LINE from `git remote` (revision 3). Before, the names were word-split, so a name with whitespace became two unrelated names and the run ended in exit 20 on the first `git remote get-url` failure; the whitespace refusal in `valid_remote` was unreachable. Now the name stays whole and is refused as above. A name containing a newline cannot be kept whole by any line-oriented read; it still ends in exit 20 (never clean, nothing executed).
- `--show-toplevel` guard: the path a worker examines must BE the repository git resolves it to (compared with `pwd -P` on both sides, so a symlinked `--root` is fine). A directory inside a repository, or a submodule path that git resolves to its parent, makes the run exit 20 with `git resolves this path to another repository (...)` and no report.
- Tests (section 30 of `test_verify_repos.sh`): `rn upload|uploadsp|dash|space` (hostile remote names), `symref upload|dash` (hostile announced default branch), `tl` (a `--root` inside a repository, a shim answering the parent for a submodule row, and a symlink control). Mutants MX3, MX3b, MX4, MX5 of `mutate_wp02_wp03.sh` remove each validation and each is caught.
- Not covered by a fixture: a remote HEAD announcing a branch NAME WITH WHITESPACE. Git refuses such a ref locally so a real bare repository cannot be built to announce it; the worker's symref pattern also excludes whitespace, so such an announcement yields no branch at all. UNCONFIRMED against a hostile server that speaks the protocol directly.
- The three shim tests (git `stash list`, `merge-base`, `rev-parse HEAD` failing) were RED-checked against a RECONSTRUCTED pre-fix copy (`scripts/audit/tests/red_prefix_wp03.sh`), not the original pre-fix file, which no longer exists; UNCONFIRMED that the reconstruction is byte-equivalent to it.

## Limits

- IC-30 "emit both classifications" for the ODG-15 pending accounts is NOT carried by the report (schema-closed): a stderr note names the accounts; the owed contract item is `specs/001-full-project-audit-remediation/audit/owed-contract-items.md`. `derive_scope.sh` does carry both readings.
- shellcheck (round 4, I-5): run from the pinned `IMG-SHELLCHECK` (0.11.0) at `-S warning` over the verifier, its tests and the mutation runner: clean after round 4 (three findings fixed or justified: SC2034 `remotes`, SC2194 in the exit-set check with a stated directive, SC2034 `VR` in the runner). Info/style-level notes remain (SC2086 on the intentional word-split `$GIT`, SC2015 `A && B || C` in the test idiom, SC2016): not warnings, recorded in `evidence/wp03/round4-shellcheck.txt`.
- The suites now run in `IMG-TESTUTIL` (and the host): see `evidence/wp03/round4-notes.md` for the results; a coverage figure is reported there with its method and its limits.

## Portability (round 4: the pinned container images)

The images ship `mawk` as `awk` and no `column` command; the verifier now needs neither gawk nor `column`: the worker feed is written with the bash `printf '%s\0'` (mawk ends an awk `printf` format at the first `\0`, which truncated every record: root cause RC1, about 150 of 167 failing assertions in the container), the exception-row hash test is `length()` plus a character-class test (mawk has no `{64}` interval expression: RC2), and the table is aligned by awk (RC5). `git` needs an identity only in the test fixtures (RC4: per-call `-c user.name -c user.email` on the two `commit-tree` calls); the dubious-ownership cause (RC3) is closed in `scripts/containers/run_pinned.sh` (`--user` mapped uid), not here. Tests: section 38 runs the verifier with `mawk` symlinked as `awk` and with a failing `column` shim (it must not be called); mutants R4i, R4j, R4k.

## Round 3: stated residuals (not hidden)

- N-I6 (round 3 statement, superseded in round 4): the images exist since 2026-10-05; the suites, shellcheck and the coverage run are recorded in `specs/001-full-project-audit-remediation/evidence/wp03/round4-notes.md`.
- m-d: three organisation classifiers exist (`org_of.py`, `scope_render.py`, `derive_scope.sh` reuses the first); they agree on every real URL today (97/97 github.com) and can differ on a non-github host. Owed to the owner.
- m-f: a checked-out submodule whose working tree was deleted is reported `uninitialised` (`--ignore-submodules=all` hides the deletion).
- m-11 (round 3 review, owed): no per-repository timeout covers `git status`, `ls-files` or `hash-object`; only the remote calls carry `--timeout` (a hung filesystem hangs the run).
- m-k: a local branch other than HEAD's with unpushed commits is not examined (outside R2's literal scope).
- m-l: `.gitmodules` `branch = .` is refused and reported unproven; no fixture for the OD-32 "master counts as main" rule.
- The needle proves the instrument sees the cases its legs exercise; it cannot prove it sees every hiding mechanism (a per-repository `.git/info` exclude of a file, `git update-index --really-refresh` states and `.gitattributes` filters are not covered): UNCONFIRMED.

## Side effects

`--fetch` writes objects into the object store only. A temporary directory, removed on exit. Nothing else: not `.git/index`, not a ref (in the repository OR in any submodule), not `FETCH_HEAD`, not a stash, no `git gc` / `git maintenance` and so no repository hook (`pre-auto-gc`) and no detached job. Round 5 (R1): `core.hooksPath=/dev/null` is now part of every git command the verifier runs (and is explicit on the fetch), because git runs the `reference-transaction` hook, twice, even for a fetch that writes no ref; the earlier claim "no repository hook" was true only of gc/maintenance. Test section 39 `q5r1`: a live `reference-transaction` hook (a control proves a plain fetch DOES run it) plus post-checkout/post-merge/post-rewrite/pre-commit marker hooks, none may run, in plain mode or under `--fetch`. Round 4 (I-1): the fetch is `git -c fetch.recurseSubmodules=false -c maintenance.auto=false -c gc.auto=0 fetch --no-recurse-submodules --no-auto-maintenance --no-tags --no-write-fetch-head --refmap= ...`. Before, git's default `fetch.recurseSubmodules=on-demand` ran a CHILD fetch inside a submodule whose pin the fetched commits bumped (writing that submodule's tracking refs) and `git maintenance run --auto --detach` could run gc and execute the repository's hooks; the previous text of this section and the "objects only" test (one repository, no submodule, default gc thresholds) were therefore wrong for a superproject. Tests: section 38 (`f1`: submodule refs and FETCH_HEAD byte-identical; `f1g`: `gc.auto=1` tripped by 1500 loose objects, a `pre-auto-gc` marker hook must not run and the pack count must not change); mutant R4a.

## Tests

`scripts/repo/tests/test_verify_repos.sh` (section 38 = round 4; mode and exit matrix on local bare remotes, the all-pairs precedence matrix, the review-fix cases, a real-tree leg), `scripts/repo/tests/test_verify_schema.sh` (schema legs, including excepted, no-remote, uninitialised and detached rows), `scripts/audit/tests/mutate_wp02_wp03.sh` (the mutation runner: reviewer mutants RM1-RM11, RV1-RV6, SV1-SV9, DM1-DM7, SM1-SM4 and the mutants of the fixes, MX1-MX5, NB1a-e, NB2a-e, NI1b, NI2a-e, MC1, Ma1; run in parallel chunks with `ONLY=<regex>`), `scripts/audit/tests/red_prefix_wp03.sh` (RED of the shim tests against the reconstructed pre-fix copy). Each test prints an `IDENTITY` line (sha256 of the test and verifier, HEAD, host, tool versions, UTC time).

## Round 6 (WF6 review of the round-5 fixes; revision 7)

- **N6-1 tail match in the held check.** `ls-remote <remote> 'refs/heads/*' 'refs/tags/*'` matches the TAIL of every advertised ref, so `refs/archive/refs/heads/old` was taken for a branch. Only a ref whose name STARTS with `refs/heads/` or `refs/tags/` is a branch or tag tip now. Test (`test_verify_repos_r6.sh`): a pin advertised only under `refs/archive/refs/heads/old` is exit 12 DIVERGED (was 0 SAME); the same pin under `refs/archive/old` and a pin held by a real branch are the controls.
- **N6-2 "cannot decide" is not DIVERGED.** A remote tip whose object this clone does not hold, or a pin object this clone does not hold, makes the held question UNDECIDED: the class becomes `UNKNOWN-DIFFERENT` (exit 14) when no ref holds the pin by equality, never a definite DIVERGED. `--fetch` still fetches only the compared branch, so it does not decide this case either; fetch the other tips (or run `git fetch` in the submodule) and re-run. Test: a colleague pushes one commit to `release`: exit 14 (was 12).
- **MR1.** A pin equal to a remote branch tip is held through the equality even when its own object is absent locally (exit 0); the shortcut is covered by a fixture now.
- **N6-3 count failure writes nothing.** The six exit-map counts are read before `--json FILE`, the `--quiet` stdout copy and the table are produced, so an unreadable count (exit 20) leaves no report anywhere, as the header says. **MR2:** each of the dirty, diverged, ahead, pin, behind and unproven counts has its own failing-jq case.
- **N6-4 the false claim.** Earlier revisions said "NO git command of the verifier runs a repository hook". That was true of hooks only. Now stated exactly, in the script header and here. Disabled on every git command: `core.hooksPath=/dev/null`, `core.fsmonitor=false` (it ran 4 times through `git submodule status --recursive` and `git ls-files -s`), `credential.helper` and `core.askPass` empty, `core.gitProxy` empty, `protocol.ext.allow=never`. Per repository, every configured `filter.<name>.clean|smudge|process` is overridden with an EMPTY command through `GIT_CONFIG_COUNT/KEY/VALUE` (an empty command is no filter in git 2.53; measured: the driver ran once for a touched file under plain `git status`, never under the override), so the comparison is UNFILTERED: a file that a filter normalises can read modified (false dirty), never clean by mistake. Per remote, `--upload-pack=git-upload-pack` on every `ls-remote` and `fetch`: a `-c remote.<n>.uploadpack=...` does NOT work, git keeps the FIRST of several values and the repository's own is first (measured: "more than one uploadpack given, using the first"). NOT switched off, stated: a remote helper named by `remote.<n>.vcs` or by a `<helper>::` URL (such a URL is unclassifiable and never sent to git), `GIT_SSH`. Tests: three fixtures (fsmonitor, uploadpack in plain and `--fetch`, a clean filter), each with a control that proves the fixture is live, plus a header check.
- Tests: `scripts/repo/tests/test_verify_repos_r6.sh` (35 cases, about 12 s) and the full matrix `test_verify_repos.sh` (404 cases) both pass.
