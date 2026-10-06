# commit-push-all.sh and cpa-host - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T19:00:00Z |
| Status | work product of T039, T042, T042a, T043, T044, T045, T045a (WP-04, slice 9); NOT yet reviewed (independent review owed, constitution 11.4.142, 11.4.209) and NOT adopted: the owner install checkpoint T046a and the adoption T047 are owner steps that this slice does not perform; indexing in `docs/scripts/README.md` is owed (that file carries another stream's uncommitted edit) |
| Status summary | `scripts/commit-push-all.sh` (CPA) with stages S0 to S8, the exit-code matrix 0, 10 to 15, 20, `--repo`, held commits, `--commit-before-integrate`, and the host entry point `scripts/repo/host_entry/cpa-host` in a reduced form; the parts of the T042 design that are not built are listed under "Built and not built" |
| Source | `scripts/commit-push-all.sh`, `scripts/repo/host_entry/cpa-host`, `scripts/repo/host_entry/INSTALL.md`; helpers `scripts/repo/{lib_safe,integrate_ff_only,integrate_merge,scope_check,validate_cheap,commit_recursive,push_recursive,record_deferral,verify_repos}.sh`, `scripts/longops/*`, `scripts/anti-mess/sweep.sh` |
| Issues | the owed items under "Built and not built" and "Owed requests to other tasks" |
| Fixed | first revision |

> `$EV` means the evidence root `specs/001-full-project-audit-remediation/evidence`; `$CPA_RUN` the run directory `.audit/commit-push/<run id>/` of one run.

## Purpose

Constitution 11.4.234: the commit and push mechanism is a dedicated script that runs the hook validations as explicit stages and is never blocked
by a hook. CPA is that one forward path. It never writes into the tracked tree and never commits its own outputs; its durable record is git itself
(the trailers below) plus the ignored run directory. It never forces: no `--force`, no `--force-with-lease`, no `+refspec`, no `--no-verify`, no
`rebase`, no `reset` (11.4.113), every merge is a plain `git merge --no-ff` made by `integrate_merge.sh`; and no git hook runs in any call of a
run (`core.hooksPath` is set to the empty `$CPA_RUN/no-hooks/` for every git process of the run, CENTRAL C4).

## Usage (always through cpa-host)

```bash
cpa-host [--run-id ID] [--paths-from FILE] [--awaits-review VERDICT] [--local-only] [--repo PATH] [--resolve-merge DIR] [--commit-before-integrate] ["message"]
cpa-host --show
```

- `--paths-from FILE`: the declared change set, one path per line relative to the main-repository root, optionally a TAB and the review verdict path.
  A path with a verdict is HELD: it is committed after the unheld paths, in its own commit with `Awaits-Review: <verdict>`, and pushed only after the
  verdict is GO. `--awaits-review V` gives V to every path without one. A run without `--paths-from` commits nothing and only integrates, pushes and verifies.
- `--local-only`: S0 to S5, no S6, S7 with `--no-remote`; exit 14, `LOCAL_ONLY` in the `Deferred-Gates:` line.
- `--repo PATH`: one owned repository at any depth: commit, integrate and push only there; a third-party path is refused (`repo_not_owned`); the parent's HEAD
  and gitlink never move; the move is recorded in `.audit/pending_pins.tsv` (columns repository path, new HEAD sha, run id; one row per repository).
- `--resolve-merge DIR`: only `.audit/merge-resolution/<run id>/` (else 20 `resolution_dir_invalid`); see "Built and not built" for its end.
- `--commit-before-integrate`: S1a fetches objects and writes the 9.2 backup under `$CPA_RUN/backup/s1a/` (the work-tree copy with its sha256 list, and a
  `git bundle` that `git bundle verify` accepts for a non-empty local range; for an empty range `backup.json` says `bundle: null`, reason `local_range_empty`),
  S2 to S5 commit the declared change set, then S1b integrates on top of the run's commits (the merge makes its own backup under `backup/`).
  Without `--paths-from`, or together with `--resolve-merge`, it is a 20 `usage_error`.
- `SKIP_LONG=<reason>` (environment): recorded at S0 as a deferral; carried by every commit of the run; exit 14.
- Any argument after the message, an unknown option, or `--paths-from` without a message is 20 `usage_error`: an option is never silently ignored.

## Stages

| Stage | Action | Refusals |
|---|---|---|
| S0 | self-test of the copied script (below), option parsing, root and branch (`main` or `master`), the merge-in-progress check (`MERGE_HEAD`; a live holder named by `merge.json`, process id and start time read against `/proc`, is `lock_held`), at least one reachable remote (`no_remote_reachable`, `--local-only` included), the purpose lock (`scripts/longops/acquire.sh`), `check_no_build_writing_tracked.sh` (`evidence_writer_active`), `SKIP_LONG`, the sweep (`scripts/anti-mess/sweep.sh --stage S0 --paths-from`; absent from the approved copy: `SWEEP_ABSENT`) | all 20 |
| S1 / S1a / S1b | `integrate_ff_only.sh` (objects-only fetch, fast-forward of the repository only; submodules are reported behind, never moved); a diverged repository goes to `integrate_merge.sh` (`git merge --no-ff` of the live remote tip, local tip first parent, `CPA-Run:` and `Foreign-Commit:` lines, a backup bundle under `backup/` that `git bundle verify` accepts) | 11, 12, 20 |
| S2 | `path_in_submodule` (a declared path below a gitlink), then `scope_check.sh` (build outputs, `*.db`, `.env`, append-only stores, blob names, a dirty submodule) | 13, 20 |
| S3 | `validate_cheap.sh` against the approved copy of the registry; `check_pending_release` rows give a 14 | 10, 20 |
| S4 | long gates: skipped under `SKIP_LONG`; else `require_verdicts.sh` over the files named in the approved `scripts/repo/long_gates.txt` when it exists | 10, 20 |
| S5 | `verdict_already_go`, then `commit_recursive.sh` (unheld paths first, then one commit per verdict) | 20 |
| S6 | `push_recursive.sh` for the repository and its owned submodules that carry the run branch, deepest first; a prefix only to a remote whose live tip is its ancestor, nothing below an unreleased merge | 11, 14 held, 20 |
| S7 | `verify_repos.sh --fetch` (`--no-remote` under `--local-only`), pointer drift against `.audit/pending_pins.tsv`, the sweep at S7 | 11, 12, 13, 15, 20 |
| S8 | `report.json`, written by the EXIT trap on every path, the lock released | |

## Exit codes (docs/16 section 12.3)

| Code | Meaning in this implementation |
|---|---|
| 0 | all stages passed, nothing owed, nothing held |
| 10 | an S3 check failed, a long-gate verdict is missing, a resolved file holds a conflict marker |
| 11 | a push was rejected or a remote unreachable; a remote moved after S1 (`remote_moved_since_s1`: no push call for it); a parent withheld because a remote of its submodule lacks the pin; S7 could not prove a remote |
| 12 | `merge_conflict` (the conflicting files with markers under `$CPA_RUN/conflicts/`, HEAD and every tracked file unchanged), `remotes_diverged`, `ff_blocked_by_local_changes`, `merge_target_moved`, a behind repository the merge helper cannot take (see owed requests) |
| 13 | scope refused, or the tracked tree is dirty at S7 (an undeclared change while the sweep is absent) |
| 14 | a recorded deferral (`SKIP_LONG`, `SWEEP_ABSENT`, `LOCAL_ONLY`), a pending check, or a push held for review (`review_pending`); never produced by passing a verifier 14 through |
| 15 | pointer drift that matches no pending pin move, an uninitialised submodule, a fast-forward that moved gitlinks which no settle helper settled (`nested_settle_unavailable`) |
| 16 | `awaiting_remote_checks`: NOT built |
| 20 | refusal or internal error: any helper exit outside its documented set, a missing or unreadable verifier report, `usage_error`, `lock_held`, `merge_in_progress`, `unrecorded_local_commit`, `pin_not_on_remote`, `backup_failed`, `resolution_dir_invalid`, `merge_resolver_not_pinned`, `verdict_already_go`, `path_in_submodule`, `no_remote_reachable`, `repo_not_found`, `repo_not_owned`, `wrong_branch`, `not_via_host_entry`, `snapshot_not_trusted`, ... |

S7 mapping of the verifier (docs/16 section 12.3): a report with no numeric `summary` counts is 20 before anything else; verifier 0 gives 0, or 14 when a
deferral is owed or a push is held; 11 (`summary.ahead`) gives 11, or no failure when the run holds commits for review (`review_pending`); 14 (a remote
could not be proven) gives 11, never 14; 12 and 13 pass through; 15 and any drift row gives 15 unless every drift row matches a row of `.audit/pending_pins.tsv`
(same path, `head` equal to the row's sha, state drifted), reported in `$CPA_RUN/pins.json` as pending; 20 gives 20.

## Lines every commit carries

`CPA-Run: <run id>` (always), `Deferred-Gates: <flags>` (when the run holds deferrals: `SKIP_LONG`, `SWEEP_ABSENT`, `LOCAL_ONLY`), `Awaits-Review: <verdict path>` (a
held commit), `Foreign-Commit: <sha>` (the first commit the run makes in a repository, one line per commit brought in from another clone that is not a CPA
commit, and in a merge commit). A CPA commit is one with the trailer AND a row in some run's `commits.tsv` (written right after the commit).

## The review verdict and the release rule

A held commit is pushed only when the verdict file is committed in the main repository's HEAD (never read from the working tree or a submodule's own tree),
validates against `specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json`, holds `verdict: GO` with `blocking_findings` 0, and lists the
commit's `CPA-Run` id in `covers_runs`. A NO-GO, a GO without the run, an uncommitted GO and an invalid file all keep the hold (14). A GO file is final: a hold on a
verdict that already holds GO in HEAD, a change to such a file, and a hold together with that verdict declared GO in the same change set are refused 20
(`verdict_already_go`); later work waits on a new review iteration `-r<n>`. A merge CPA made into a held range is itself held on
`$EV/reviews/CPA-merge-<run id>.json`; nothing below an unreleased merge is pushed to any remote.

NO-GO path (P0-P1 conventions): `git revert --no-commit` of the held commit, then `git revert --quit` and `git restore --staged`, then a later run commits the
reverted paths held on the same verdict file; for a held CPA merge, a corrective held commit for a rejected resolution, or `git revert -m 1 --no-commit` limited to
the rejected paths for rejected foreign content. The check that a GO's `covers_runs` names every run whose commits the abandoned change spans
(`verdict_covers_incomplete`) is NOT built here (see below), so the revert path is a convention of the reviewer until it is.

## The report and the run directory

`$CPA_RUN/report.json` (schema 1) is written on every exit path: `exit`, `failed_stage`, `reason`, `detail`, `mode`, `deferred_gates`, `held`, `held_commits`,
`commits` (from `commits.tsv`), `stages`, `interrupted_runs` (run directories of other runs without a report, left byte-unchanged), `trust_file` (the path the
self-test read) and `files` (the sha256 of every other file of the run directory). A run killed before S8 leaves no report; the next run lists it. After every
run every file it wrote lies under its own `$CPA_RUN`, apart from `.audit/pending_pins.tsv` and the lock records under `.audit/longops/`; no file under `$EV`
or any tracked path was written, and after a run that exits 0 or 14 `git status --porcelain --ignore-submodules=all` is empty.

## cpa-host

A run starts only in `cpa-host` (installed by the owner outside every repository, `scripts/repo/host_entry/INSTALL.md`), which: resolves the main repository root
(`git rev-parse --show-superproject-working-tree` chain, the innermost repository holding `scripts/repo/host_entry/cpa-host`; none is `project_not_trusted`); reads
`$CPA_HOST_STATE/trust.json` (`cpa-host-trust/1`: per project, keyed by the root commit of HEAD's first-parent chain, the `adoption_commit`, the approval history and the
approved manifest) and refuses with 20 `project_not_trusted` when the file or the entry is absent, the manifest lacks `scripts/commit-push-all.sh`, or the adoption commit is
not on the first-parent chain; fixes the run id (`--run-id` must match `^[A-Za-z0-9_-]+$` and be unused: `run_id_malformed`, `run_id_in_use`); creates the run directory with
`mkdir` without `-p`, `no-hooks/`, and copies every manifest path from the content-addressed store into `released/`, re-hashing each (`approved_copy_invalid`; the run
directory is removed again); writes `released/trust-snapshot.json`; and executes `released/scripts/commit-push-all.sh`. The copied script's FIRST step is its self-test:
its resolved own path must equal the released copy of its run, its sha256 the snapshot entry, the snapshot's canonical sha256 (`jq -cS` of the entries sorted by path) its
`manifest_sha256` and a manifest that an approval row of the project's history records and no `revoke` row names; else 20 `not_via_host_entry` or `snapshot_not_trusted`
with nothing written. So the tracked file, a held or foreign copy, or a fabricated run directory does nothing. A same-uid process that also forges a state directory and
points `CPA_HOST_STATE` at it passes: only the conductor rule (no agent installs, edits or approves in the owner's state) forbids that.

## Built and not built (UNCONFIRMED where stated)

Built and tested on throwaway clones with local bare remotes (`scripts/repo/tests/test_commit_push_all.sh`): the matrix of every exit code; usage errors; the lock; deferrals;
held commits and the release rule; the CPA merge (clean, held range, conflict, conflicting remote tips, a remote moved after S1); `--commit-before-integrate` with its S1a
backup; `--repo` and the pending pin rows; the S7 mapping; the report on every path; no hooks; no force/lease/`+ref`/`--no-verify`/`rebase`/`reset` (git shim log plus a
static scan of both scripts); `cpa-host` run mode, `--show` and its refusals.

NOT built in this slice, each specified in tasks.md T042 and still owed (nothing here is claimed done for them):

- `cpa-host --owner-trust approve|retire|revoke|reanchor`, `--resume`, `--check-provenance`, `--exec-approved` (20 `owner_op_unimplemented` / `mode_unimplemented`): the owner
  checkpoint T046a, T121b, T094d. The test harness writes its own throwaway trust state directly; CENTRAL C3's `script(1)`-driven `approve` is therefore UNCONFIRMED.
- the path gates (`path_gate_unheld`, `scripts/repo/path_gates.tsv`, the `release_seam` class and its refusals, `conf_code_split`), G-PIN and `accepted_pins`
  (`verdict_gate_mismatch`, `verdict_pins_incomplete`, `outgoing_gitlink_not_accepted`, `accepted_pins_*`), `verdict_covers_incomplete`, the verdict provenance check
  (`review_provenance_invalid`), the "GO committed in HEAD lists it" clause of the CPA-commit predicate, `Self-`/foreign-CPA-code routing of the first-parent chain beyond what
  `integrate_ff_only.sh` does, `held baseline` and `baseline_drift` (the ratchet baselines), the S2 secret fold and private-key fold (`detect-secrets` absent, T040a/T006).
- remote checks and exit 16 (`awaiting_remote_checks`), `remote_check_unavailable`, `--resume` and its refusals, the container S3 checks through `run_pinned.sh` (S3 runs the host builtins
  and scripts of the registry), `CHECK_PENDING_RELEASE` in the `Deferred-Gates:` trailer (see below: it is an exit-14 report line only).
- `--resolve-merge` end to end: `integrate_merge.sh` ends a resolved merge with 20 `secret_fold_unavailable` until the S2 secret fold exists; CPA passes that through. The two refusals before it
  (`resolution_dir_invalid`, `merge_resolver_not_pinned`) are tested.
- a fast-forward that moves gitlinks calls `scripts/repo/record_pending_pin.sh --settle-nested` only when that helper is in the approved copy (T435a); its output contract is UNCONFIRMED,
  and without it S7 gives 15 for exactly the changed paths (`nested_settle_unavailable`, tested).
- S6 in main mode pushes only the owned submodules that carry a LOCAL branch named like the run branch (`push_recursive.sh` pushes `refs/heads/<branch>`); a detached submodule is left out
  and logged, never pushed. Whether the real repository's owned submodules satisfy this is UNCONFIRMED (never run against the real repository).

## Owed requests to other tasks (no helper was changed by this slice)

1. `record_deferral.sh`: its closed flag set lacks `CHECK_PENDING_RELEASE` (T042 C1 (c)); CPA reports a pending check as exit 14 and in `report.json` (`check_pending_release`) but the commit trailer cannot carry it.
2. `integrate_ff_only.sh` has no `--no-move` and `integrate_merge.sh` no `--backup-only` (the docs/16 skeleton assumes both): CPA writes the S1a backup itself; the unrecorded-commit guard of S1a is a small copy of the helper's predicate.
3. `integrate_merge.sh` skips a tip that descends from HEAD (it leaves it to the fast-forward), while `integrate_ff_only.sh` routes `remotes_diverged` and `merge_path_required` to the merge path even for a behind repository: for such a window
   nothing is integrated, and CPA exits 12 with the ff_only reason (conservative, nothing moved) instead of the specified in-turn merge of two non-conflicting tips for a BEHIND repository (the same case with a local commit works and is tested).
4. `validate_cheap.sh` has no `--run-declared` use here: a declared registration of a new check is therefore never run by CPA (it needs the path gates).
5. `docs/scripts/README.md` must index this document and `scripts/repo/host_entry/INSTALL.md` (the file carries another stream's uncommitted edit, so this slice left it alone).

## Naming (constitution 11.4.29) and decoupling (11.4.177)

The canon-named `scripts/commit-push-all.sh` (11.4.234 names that path) and `cpa-host` are kebab-case exceptions to 11.4.29, and so are the other new kebab-case names that the
design documents fix, each listed with the task that creates it: `tools/evidence/wrap-go.sh` and `tools/evidence/wrap-bash.sh` (T051), `wrap-vitest.sh`, `wrap-gradle.sh` and
`wrap-cargo.sh` (T053), `scripts/test-in-container.sh` (T121), `scripts/audit/android-container.sh` (T145), `scripts/bash-coverage.sh` (T199) and `scripts/audit/adb-container.sh` (T420).
None is renamed by this task; every other new script name of this feature is snake_case. Whether `cpa-host` keeps its name on `PATH` or is renamed is an owner item (pending owner
confirmation, never changed by this task).

`cpa-host`, installed on the owner's shared `PATH`, is project-agnostic: it operates on the invocation directory, keeps per-project trust state (CENTRAL C3, C5, keyed by the
root commit) and carries no literal of this project. Gate `CM-TOOLING-PROJECT-DECOUPLED` is `scripts/repo/tests/test_tooling_decoupled.sh`: over `cpa-host` and every file it
sources, no hit of the closed list (the project name, this repository's remote URLs, the absolute path of the checkout), each hit reported by file and line, with a control
needle and a paired mutation.

## Portability: the WP-09 image runs jq 1.6

`IMG-KCOV` (and its base) carries `jq-1.6`, `git 2.39.5`, `bash 5.2.15`; the host has jq 1.8.1, git 2.53, bash 5.3. Two jq 1.6 behaviours that a host-only run hides, both found by running
the matrix in the image and both fixed in `commit-push-all.sh`: `jq -e` exits 0 on EMPTY input (so `git show HEAD:<missing path> | jq -e ...` read a path absent from HEAD as a GO verdict: emptiness is now
checked before jq, `json_is` and `head_json_is`), and `$def` is a reserved word (the report builder variable is `$deferred`). The whole matrix passes in the image (`$EV/wp04/cpa-image-matrix.txt`).

## Tests, mutations and coverage (evidence under `$EV/wp04/cpa-*`)

- `scripts/repo/tests/test_commit_push_all.sh`: the matrix, 336 assertions in 13 sections (`CPA_ONLY=<section name>` runs one; `CPA_KEEP=1` keeps the scratch tree; `CPA_SRC=<dir>` reads the scripts under test from a mutated copy). Every case builds a fresh clone, two bare remotes and its own
  trust state under `$TMPDIR`; no real remote, hosted URL or credential is reachable. A `git` shim on `PATH` logs every call: any `--force`, `--force-with-lease`, `+refspec`, `--no-verify`, `rebase` or `reset` fails the run, and a static scan of both scripts does the same.
  RED first: `cpa-red.txt` (the scripts absent, 180 of 227 assertions failing; the other 47 were vacuous absence assertions, hardened afterwards); GREEN x3 on the host: `cpa-green.txt` (336 ok, 0 failed, three runs); the same matrix in the image under kcov: `cpa-kcov.txt` (336 ok, 0 failed).
- `scripts/repo/tests/run_wp04i_mutations.sh` with `scripts/repo/tests/cpa_mutants.py` (T043): 48 mutants of CPA and `cpa-host`, each made to FAIL by the section meant to catch it while the unmutated control passes every section used: 46 caught, 0 survived, 2 equivalent (a second layer enforces the same property:
  `path_in_submodule` is also refused by `commit_recursive.sh`, the `--resolve-merge` directory also by `integrate_merge.sh`); `cpa-mutation.txt` states the two earlier runs (a reporting bug of the runner, and two mutants that survived because their test passed for the wrong reason) and their fixes.
- `scripts/repo/tests/test_tooling_decoupled.sh` (T045a, gate `CM-TOOLING-PROJECT-DECOUPLED`): RED on the control needle, GREEN x3 (`cpa-decoupled-red.txt`, `cpa-decoupled-green.txt`).
- T044: `scripts/containers/run_pinned.sh ... IMG-KCOV -- kcov --include-pattern=/released/scripts/commit-push-all.sh,/bin/cpa-host /out/cov scripts/repo/tests/test_commit_push_all.sh` (the T044 text `kcov /out/cov bash <script>` does not work: kcov treats `bash` as an ELF binary, so the script is given to kcov directly): kcov line coverage, union over the released copies,
  `scripts/commit-push-all.sh` 288 of 320 executable lines = 90.0 %, `scripts/repo/host_entry/cpa-host` 61 of 61 = 100.0 % (`cpa-kcov.txt`, `cpa-kcov.json`; line coverage, never branch coverage; a number is a floor and proves no assertion, constitution 11.4.224 (C)); lint through IMG-SHELLCHECK: both scripts clean with `-x`
  (`cpa-shellcheck.txt`, which also lists the findings of the committed helpers and of the test files, none of them touched here).
