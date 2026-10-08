# check_task_citations.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T14:00:00Z |
| Status | new, untracked at writing (task T283a); independent review owed (constitution 11.4.142; T287 documentation, T288a G-GATE); NOT yet registered in `scripts/repo/validate_checks.tsv` or `scripts/repo/cpa_code.txt`; its row in `docs/scripts/README.md` is applied by the conductor |
| Source | `scripts/docs/check_task_citations.py`; tests `scripts/docs/tests/test_check_task_citations.py`; decision data `specs/001-full-project-audit-remediation/task_citations.tsv` |
| Contract | `specs/001-full-project-audit-remediation/docs/21-master-plan-phases-risks-and-traceability.md` section 12.7; `tasks.md` T283a |
| Evidence | `specs/001-full-project-audit-remediation/evidence/docs/` (`task-citations-red.txt`, `task-citations-green-run{1,2,3}.txt`, `task-citations-mutation.txt`, `task-citations-review-queue.tsv`) |

## Purpose

A task id cited outside `tasks.md` usually still exists after a renumbering; it just names another task, so a check that only asks "does the id
exist" stays green. The check binds every cited id, at its line, to a keyword that the cited task's text must still contain.

## Usage

```bash
python3 scripts/docs/check_task_citations.py [--feature-dir DIR] [--tasks FILE] [--list FILE] [--extract] [--self-test-blind]
```

| Option | Meaning |
|---|---|
| `--feature-dir DIR` | the feature folder (default `specs/001-full-project-audit-remediation`) |
| `--tasks FILE` | tasks file (default `DIR/tasks.md`); task lines are `- [ ] Tnnn ...` |
| `--list FILE` | citation list (default `DIR/task_citations.tsv`) |
| `--extract` | seeding aid: one TAB row per (file, line, id) of a fresh extraction: `file, prefix, id, kind, PROPOSED keyword, how, line`; writes nothing |
| `--self-test-blind` | TEST ONLY: the engine omits `ORPHAN-ROW` and `UNBOUND`, so the seeded control cannot be reported (exit 20) |

Plan set: every Markdown file under `DIR` except `tasks.md` and the top-level folders `evidence/` and `audit/` (run records, so an ordinary
note never fails as `UNBOUND`), plus `DIR/contracts/*.json`. Token: `T`, three digits, optional lower-case letter, no letter or digit directly
before or after; both ends of `Tnnn to Tnnn`, `Tnnn-Tnnn`, `Tnnn..Tnnn`, `Tnnn through Tnnn` are range-form occurrences.

## Citation list

TAB separated, `#` lines ignored: `file`, `prefix`, `id`, `kind`, `keyword`. `prefix` must begin exactly one line of `file`; `kind` is
`citation` (keyword required, must occur in the cited task's text), `range_bound` (only has to exist) or `format_example` (must NOT exist).
A row can silence only `UNBOUND`. The list is decision data, not a G-GATE path; every edit needs a [REVIEW] verdict that checks each added or
changed row against its file line and the task text.

## Findings and exit codes

`FINDING <CLASS> <file>:<line>: <detail>` for `DANGLING` (id cited, not a task), `UNBOUND` (no row), `STALE` (keyword missing from the task text),
`ORPHAN-ROW` (the row's prefix does not begin exactly one line, or the id is not on it, or a duplicate row), `EXAMPLE-RESOLVES` (a format example
that is a real task). Last line: `check_task_citations: N findings in M plan-set files (...); R list rows; seeded control reported (...)`.

| Code | Meaning |
|---|---|
| 0 | no finding |
| 1 | at least one finding |
| 2 | usage error or unreadable input (missing file, non-UTF-8 file, malformed list row) |
| 20 | blind: the seeded control was not reported |

## Seeded control (every run)

In memory, the first bound `citation` row (sorted by file and line, id occurring once on its line) has its id replaced by the id of the task two
places before it in `tasks.md` order (two after when there are fewer than two before). The engine must report a finding on that line, else the run
prints `BLIND` and exits 20: its silence would say nothing (11.4.201(7)(b), 11.4.273).

## Side effects

None: reads files only; `--extract` prints.

## Registration (owed)

Registry row in `scripts/repo/validate_checks.tsv` (CPA S3 plain check, scope `changeset`, skips itself, recorded as `skipped: no plan-set path
declared`, when the run declares no plan-set file) and the command file in `scripts/repo/cpa_code.txt` (class G-GATE) belong to the G-GATE
change set T288/T288a and are NOT done here. Until the owner approves the check's GO in `cpa-host` it stays `check_pending_release`.

## Seeding honesty

The first list was produced from `--extract` with PROPOSED keywords (backticked token or marker id shared by the citing paragraph and the cited
task: "auto"; else a shared rare word: "word"; else the task's title words: "title"). It has NOT been reviewed row by row; the "word" and "title"
rows are in `evidence/docs/task-citations-review-queue.tsv`. A keyword drawn from the task's own text cannot be stale on the day it is written; it
catches later rewording or renumbering only.

## Tests

`python3 -m pytest -p no:cacheprovider scripts/docs/tests` inside IMG-DOCS (`scripts/containers/run_pinned.sh IMG-DOCS -- ...`). Fixtures: golden good,
one per finding class, range forms, token boundaries, blind runs, usage errors, and 8 paired mutations (`# MUT:<name>` lines of the script weakened;
the same battery must fail against each copy, including the "only checks that the id exists" copy).
