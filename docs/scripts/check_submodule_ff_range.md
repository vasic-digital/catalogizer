# check_submodule_ff_range.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-30 |
| Last modified | 2026-10-30 (new) |
| Status | new; independent review owed (constitution 11.4.142); indexed by `docs/scripts/README.md` |
| Source | `scripts/repo/check_submodule_ff_range.sh`; registry row `submodule_ff` in `scripts/repo/validate_checks.tsv`; 2-line change in `scripts/repo/validate_cheap.sh`; test `scripts/repo/tests/test_check_submodule_ff_range.sh` |

## Purpose

Wires the read-only gate [`scripts/check_submodule_ff.sh`](check_submodule_ff.md) into the dedicated commit/push script `scripts/commit-push-all.sh` as a named check of stage S3 (`validate_cheap`), registry row `submodule_ff` (scope `changeset`, mode `plain`). The check fails the stage when a push would move an own-org submodule gitlink backward or sideways (constitution 11.4.113, 11.4.288(D)). It is an explicit stage of the dedicated script, not a git hook: no `core.hooksPath` hook, no `--no-verify` (11.4.234(A)(B)).

## Usage

`scripts/repo/check_submodule_ff_range.sh --root <repository>` (CPA runs it with `--root {root}`, cwd the repository). For every distinct sha of `refs/remotes/*/main` and of `@{upstream}` it runs `scripts/check_submodule_ff.sh <tip> HEAD` from the code root (the gate sits at `<code root>/scripts/check_submodule_ff.sh`, its helpers at `<code root>/scripts/audit/`). It never fetches (S1 already did), never writes, stages or pushes.

## Verdict mapping (11.4.234(D): the mechanism is never blocked by an undecidable check)

| Gate result | Wrapper exit | S3 effect | CPA outcome |
|---|---|---|---|
| PASS / FF / nothing to check (rc 0) for every tip | 0 | none | stage clean |
| NONFF own-org (rc 1) for any tip | 10 | `fail submodule_ff - <report>` | exit 10, `check_failed`; the report starts with the remediation text (repair the pin so it descends from the old pin, or merge the submodule main; never force-push), then the `NONFF own` lines |
| BLIND (rc 2), usage/list problem (rc 64), any other rc, `timeout` (rc 124), the gate missing from the code root, no remote main tip and no upstream, `HEAD` not a commit, budget exhausted | 14 | `deferred submodule_ff <reason>` (exit 0 of validate_cheap) | the existing deferral `CHECKS_DEFERRED`: run exit 14, flag in every commit of the run and in `report.json`; never a clean pass, never a block of the commit/push |
| wrapper usage error (no `--root`, unknown option, non-repository, option-like root) | 20 | `check_refused` | exit 20 |

BLIND is a recorded deferral, not a pass and not a refusal: the pin cannot be judged (typically a submodule object store that lacks the pin), so the run owes a by-hand `scripts/check_submodule_ff.sh <tip> HEAD` after `git submodule update --init`. No new deferral flag was minted: `CHECKS_DEFERRED` already carries "a registry row did not run".

`validate_cheap.sh` change: a changeset-scope script that exits 14 now prints `deferred <check> <message>` and continues, instead of the generic `fail` for every non-zero exit. Other changeset checks (`no_ci`, `detect_landmines`) never exit 14. A files-scope script exiting 14 is still a `fail`.

Gate exit 1 is always a refusal 10: when no `NONFF own` line can be parsed the report says so and quotes the first VERDICT/ERROR line (fail closed). Time bound: `SUBFF_RANGE_BUDGET` (whole seconds, default 100, values above 100 are clamped to 100) bounds the whole check and each gate call (`timeout`), below validate_cheap's own 120 s kill (which would be a refusal 20); an exceeded budget is a deferral. Gate cost is about 0.1 s per commit (see the gate guide).

## Tests

`scripts/repo/tests/test_check_submodule_ff_range.sh` (33 checks) is UNIT-CLASS (constitution 11.4.289(C)): it drives the real wrapper, the real gate and the real `validate_cheap.sh` against throwaway repositories with real local submodules (a local-path submodule URL is own-org by the gate's fail-closed rule). Cases: forward (PASS), backward (NONFF, 10, remediation first), fabricated pin (BLIND, 14), empty range, no remote and no upstream (14), gate absent (14), budget exhausted (14), two remotes sharing one tip, usage errors (20), refs/work tree/HEAD unchanged after every case, and the three outcomes through `validate_cheap.sh` (fail 10, deferred 0, clean 0). RED was observed with the wrapper absent (11 ok, 22 failed). `--mutants` runs four lightweight mutants (NONFF not failing, BLIND read as pass, missing remote read as pass, a 14 treated as fail by validate_cheap); each must make the test fail (all four killed when written).

## Limits and open points

- The range is `<remote main>..HEAD` at S3, i.e. commits that already exist. Commits S5 makes later in the same run, and pin moves of `--repo` runs recorded in `.audit/pending_pins.tsv`, are not in the range. A check after S5 and before S6 needs a new stage (not done).
- Adoption: CPA runs the APPROVED registry. Until the owner approves a manifest containing the new row, `validate_cheap.sh` (a) reports the row `check_pending_release submodule_ff added` and does not run it (deferral `CHECK_PENDING_RELEASE`, exit 14), and (b) the released copy lacks `check_submodule_ff_range.sh`, `check_submodule_ff.sh`, `scripts/audit/org_of.py` and `scripts/audit/own_orgs.txt` unless they are added to the approved manifest (cpa-host trust file). Approval is the owner's act and was not performed here.
- Remote tips come from the local remote-tracking refs written by S1; a stale tracking ref gives a larger or smaller range, never a wrong verdict for the range it names.
- Only `main` remote refs are read (11.4.288); a push of another branch name is not covered.
