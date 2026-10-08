# commit-push-all, fix round 5 (11.4.276 round 4 of the item): convergence assessment

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08T04:41:48Z |
| Status | the assessment part was written BEFORE the fixes (11.4.276(E)); the section "Result" was added at the end with the measured results |
| Status summary | the fixer's assessment of the consolidated fix list `FIXLIST-cpa.md` (19 classes, 114 findings, 9 BLOCKING) for commit-push-all at `4b4607971029aa2c6a56b7102e41d9b01645d09a` |
| Source | `FIXLIST-cpa.md` (consolidator, independent of the producer), section 0.4 and 1 |
| Issues | the cross-area members listed under "Not fixed here" |
| Fixed | the 19 classes of the list in the scripts and the matrix of this revision; the not-fixed members are named under "Result" |

## Assessment

Round count for this item: the review budget `R_max` is at most 7; this fix answers review round 4 (WF11, WF14, WF17, and the three-lens consolidation). Two classes recurred AFTER a claimed class
closure, which is the (E)(3) trigger for a STRUCTURAL round:

1. the caller-environment class: WF11 F1/F11 (named GIT_* and a few families), WF14 N1/N3 (a larger denylist), WF17 W1-W6, now lenses A, B, C (exported functions, SHELLOPTS, BASHOPTS, BASH_ENV, TMOUT,
   PYTHONPATH, LD_PRELOAD, a relative state path, empty PATH elements). Root cause, derived on the real producer (bash, git, python, the dynamic loader): a denylist of NAMES executed as a script line
   cannot cover what the interpreter, the loader and the language runtimes read BEFORE or OUTSIDE it. The fix is a REPLACEMENT, not another layer: `#!/bin/bash -p`, an ALLOWLIST with one `env -i`
   re-exec, value checks, `python3 -I` at every call site.
2. the signal class: WF14 N2 (a trap list by name), WF17 99.7, then A F-04 / C 98.5 / 98.6. Root cause measured on the real producer (`kill -l` + 64 `$(kill -l n)` subshells = 69 to 267 ms with no
   handler; the finish handler left the old `exit` handler on numbers it had not yet swallowed). The fix replaces the 64-fork loop by one table read installed FIRST, and the handler IGNORES every
   number before it exits; `finish` ignores instead of handling.

The other classes are one model per fact that the producer and the consumer describe twice (RES, MODEL: run liveness, merge.json), a trust boundary that judged bytes other than the committed ones (TOCTOU),
and repository state that was executed or obeyed (REPO). Each is closed by replacing the second description with the first, not by adding a check.

## Class inventories (members enumerated before the fix, each decided)

Taken from the consolidated list, which already enumerates each class with a control needle; this round adds members found by the fixer while reproducing:

- ENV: functions (git, unset, exit, [, ...), SHELLOPTS (noexec, xtrace + PS4, noclobber, noglob), BASHOPTS (nocasematch), BASH_ENV, ENV, TMOUT, PYTHONPATH/PYTHONHOME, LD_PRELOAD/LD_LIBRARY_PATH/LD_AUDIT,
  relative or empty HOME/CPA_HOST_STATE/TMPDIR, empty/relative/in-tree PATH elements, LC_ALL not exported, `OLDPWD` (bash exports it after `cd`; found by the fixer: the core would re-exec once per run),
  every name outside the allowlist. Closed by one allowlist, so a member not listed is closed too.
- SIG: start-up window of the copied script, of the host entry, finish before the swallow, a group signal, latency behind a foreground helper, signals 32/33 (uncatchable: documented residual).
- RES: SIGKILL, signals, selfrefuse after the run directory exists (tool_missing, not_via_host_entry x3, snapshot_not_trusted x3, revoked), the pre-run EXIT trap, the host EXIT trap, finish's own
  report failure, a custom run id; the leaked claim at four points; a commit without its row (2 sites).
- Cross-area (not touched, see "Not fixed here"): `scripts/anti-mess/sweep.sh` INV-9 and its merge.json reader (MODEL-2/3), `scripts/longops/*` shebangs and `lib.sh:208` text.

## Ground truth

Every behaviour above was re-derived on the real bash 5.3.9, git 2.53.0, python 3.14.4, jq 1.8.1 of this host and on the real helpers through `cpa-host`; no test re-implements guarded logic. The probes of
the three lenses are adopted as matrix cases (section 18 of `test_commit_push_all.sh`), and the matrix harness is changed so that its controls travel in files (the run's environment is an allowlist).

## Decisions where the fix list is silent or contradicted by the project (recorded, not hidden)

- `remote.*.pushurl` is ALLOWED in the repository-config table although the list names it as refused: this project's multi-upstream push (constitution 2.1) is built on it (the real checkout carries
  six push URLs). The attack of probe P3b (a push URL that names an unapproved repository) is closed by the ownership rule instead: EVERY url and push url of EVERY remote must name an own
  organisation (`repo_owned`), also for the main repository. Consequence for the real checkout, UNCONFIRMED against a live run: `scripts/audit/own_orgs.txt` lists `vasic-digital` and `HelixDevelopment`,
  while two push URLs of `origin` name `milos85vasic`; the owner must add that organisation (or drop those push URLs) before a real run passes S0. Not changed here (audit area).
- The commit is made from the verified index (`git commit -F`, no `--only`) because `git commit --only` re-reads the work tree at commit time and would commit bytes other than the verified ones. A
  consequence: anything staged outside the declared group refuses the commit (20 `unrelated_staged_changes`); the real sweep already refuses an undeclared change at S0.
- `S0` closes a dead run by writing `interrupted.json` AND a `report.json` (`closed_by`): the real sweep (another area, unchanged) reads `report.json` to decide whether a run is interrupted, so
  without the report the sweep would keep blocking every later run.

## Result (added after the fixes; every number is in the files named, nothing here is a prediction)

- Matrix: `fix-r5-green.txt` = 1044 ok / 0 failed, three runs, exit 0 each (24 sections). `fix-r5-red.txt` = the same matrix against the scripts of the reviewed revision: 550 ok / 159 failed over the 12 sections that carry the fixes.
- Mutation: `fix-r5-mutation.txt` = 150 mutants, 24 of 24 controls green; run 1 145 caught / 1 survived / 4 equivalent, the survivor re-run caught: 146 caught, 0 survived, 4 equivalent. Every non-equivalent mutant names the assertion that fails.
- Helper suites (`fix-r5-helpers.txt`): 21 suites, all green after two of them (`test_validate_cheap.sh`, `test_wp04d.sh`) were brought to the new designs (the approved registry decides the run set; a NUL file of an unlisted suffix is `not_judged`; the verdict predicate is a second file that the copies must carry).
- 11.4.276(E)(3) check at the end of this round: no class recurred inside this pass. Two points where a first fix was wrong and the proof said so, recorded because they are the reason the proof runs on the real producer: (1) the matrix signalled the process of ANOTHER parallel mutation job through an unscoped `pgrep`, which made 18.23 and its neighbours fail in the controls; scoping it to the clone's own path closed the class (two uses); (2) a refusal raised INSIDE a helper call that is written `helper_run X >file 2>&1` went into that file and the operator saw nothing; the run now keeps its own stderr on fd 8.
- Not fixed or UNCONFIRMED, with the reason: STALE-2 is an OWNER decision (recorded in the guide, 11.4.66); `scripts/audit/own_orgs.txt` lacks the organisation of two push urls of the real `origin` (audit area; the first real run stops at S0 until the owner decides); the S7 route "held remainder plus verifier 11 with unproven above 0" is tested through verifier 14 only; ENV-13 and REASON-3; the cross-area members (sweep INV-9 and the `merge.json` reader, longops and anti-mess shebangs, the `lib.sh` text) were not touched; kcov was not rerun and shellcheck ran on the host binary, not on the pinned image; the VD1 kill is UNCONFIRMED; signals 32 and 33 and the pre-scrub start of the caller's own entry process are stated residuals.
