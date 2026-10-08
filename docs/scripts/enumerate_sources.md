# enumerate_sources.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08T05:00:00Z |
| Status | tracked, new (WP-20, tasks T162 to T165); revision 2 is the single fix pass for the independent review WF23 of 2026-10-08 (findings A1-A7, B1-B4, F3, F4; D1 is open as OD-17); the independent re-review of revision 2 is owed (constitution 11.4.142); the freeze.json shape it reads is UNCONFIRMED against T161 (not written yet) |
| Source | `scripts/register/enumerate_sources.py` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

Stage 0 of the findings-register reconciliation (doc03 section 10 step 2, tasks T165): a mechanical, no-judgement enumerator that walks the 25 source classes S-01..S-25 of the **frozen snapshot**, the cross-source defect-id index (class `XID`) and the external sources (class `EXT`), and emits one `reg_source_entries` row per structured entry. It never opens a database: it only writes files, and the host imports them under the single-writer contract.

## Usage

```bash
python3 scripts/register/enumerate_sources.py --freeze-json <freeze.json> --out <dir>
python3 scripts/register/enumerate_sources.py --list-classes     # class -> reg_sources.kind mapping, rules
```

Run through `scripts/containers/run_pinned.sh IMG-TESTUTIL -- python3 scripts/register/enumerate_sources.py ...` (IMG-TESTUTIL: python3 with PyYAML). Exit 0 ok, 2 usage, 20 refusal.

There is **no option that replaces the entry check** (`--check-script` was removed: it let any executable stand in for the mandatory re-hash). PyYAML is required: without it the run is refused `pyyaml_missing` (the bank walk and the strict front-matter check would differ between environments).

`freeze.json` (UNCONFIRMED vs T161): `{"snapshot": "<abs dir>", "manifest": "<abs manifest path>", "remotes": [<name> | {"name","url"}, ...], "frozen_at": "<UTC, optional>", "head": "<sha, optional>"}`. `remotes` is **required** (`[]` is a stated fact, an absent key is refused `freeze_remotes_missing`): T165 asks for "one row per remote and named service" and the T161 text records no remote list (a request to the T161 owner, `$EV/wp20/fix-r2-open-requests.md` R3). A remote url is stored without its userinfo. The manifest is the output of `scripts/register/snapshot_manifest.py <dir>`.

## Behaviour

1. **Entry check.** `scripts/register/check_freeze_snapshot.sh <snapshot> <manifest>` runs before any snapshot file is read. A non-zero exit refuses (exit 20), naming each path, and writes nothing: `freeze_snapshot_moved` for an added, missing or changed path, `freeze_manifest_invalid` for an unreadable or malformed manifest, `freeze_special_file` for a FIFO, socket or device in the tree, `freeze_path_unsafe` for a path that is not UTF-8.
2. Walks the snapshot (files and symlinks, a symlink hashed over its `readlink` string and never followed), assigns each file to the first source that selects it (so a file is enumerated once, under its own class), and applies the per-class rule. Class table (full rules in `--list-classes`):

| Class | Source | Kind | One row per |
|---|---|---|---|
| S-01 | `docs/issues/*.md` | issue_file | ticket file (front-matter: id, status, severity, title; the strict-YAML refusals are counted in the stats) |
| S-02 | `issues/*.md` | issue_file | file |
| S-03 | `TASK_TRACKER.md` | md_tracker | table row `^\| *[0-9]+\.[0-9]+`; status from the status word or, as in the real file, from the glyph of the file's own legend (`⬜ **Not Started**`) |
| S-04, S-05, S-06, S-10 | the checklists | md_tracker | `- [ ]` / `* [x]` / `+ [X]` line |
| S-07 | `COMPREHENSIVE_UNFINISHED_WORK_REPORT.md` | md_tracker | checkbox line and cross-mark line |
| S-08, S-09 | unfinished-work reports | report_doc | cross / warning mark line |
| S-11 | `docs/LANDMINES.md` | report_doc | distinct `RULE-<scope>-NNN` |
| S-12 | other root `*.md` (the 8 governance and onboarding files and the 9 own-class files excluded) plus `docs/COMPREHENSIVE_PACKAGE_SUMMARY.md` and `docs/README_IMPLEMENTATION_PACKAGE.md` | report_doc | file and checkbox line |
| S-13 | `docs/status/*.md` | report_doc | file |
| S-14 | `docs/audits/*.md`, `docs/*AUDIT*.md` | report_doc | file and checkbox line; plus the ids first seen in these files (below) |
| S-15 | `docs/qa/**`, `docs/reports/qa-sessions/**` | qa_results | file; plus the ids first seen in these files |
| XID | every other text file (cross-source id index) | report_doc | distinct id referenced only outside the S-14 and S-15 files |
| S-16 | `docs/security/**` | report_doc | file; vulnerability records of the latest scan per tool and app; `scan-failed` row for a failed scan |
| S-17, S-18, S-19 | HelixQA banks | qa_bank | bank file and bank case (a case without an `id` is `#@<index>`) |
| S-20 | `bluff-baseline.txt`, `behavior-anchors.md` | report_doc | baseline line, `CAP-NNN` row |
| S-21 | `.specify/memory/constitution.md` Known Conflicts | constitution_conflict | numbered item (status = the distinct status words of its whole block, else the preface's word) and sub-item (indented bullet, or one segment per bold status marker when an item holds two or more) |
| S-22 | per-module `CLAUDE.md` / `AGENTS.md` outside submodules | report_doc | file |
| S-23 | every non-binary file of the snapshot, any extension | code_marker | `TODO`/`FIXME`/`HACK`/`XXX` hit and skipped-test call (locators `#k` and `skip-<lang>#k` numbered per line and tag) |
| S-24 | `.implementation/**` | report_doc | file |
| S-25 | rotation list, audit report, firebase exposure doc | report_doc | provider row (names only, no line text); file |
| EXT | `remotes` of the freeze json; the named services firebase-crashlytics, sonarqube, snyk, trivy | external_ticket | remote (status `not_queried`); service (`marker_found` / `marker_absent`: whether its marker files are in the snapshot) |

3. **Defect ids (finding A1).** One entry per DISTINCT id of the whole snapshot (doc03 section 9: an id referenced only in prose is still imported), over the family set `CATAPI-DEFECT-N`, `FIX|DEFER-QA-date-N`, `DEFER-NNN`, `FIX-OCn-N`, `FINDING-N`, `HQA-DOCS-N`, `HQA-NNNN`, `HQA-PHASEn-X-N`, `FIX-OBS|BROWSER-N`, `FIX-NNN`, `BUG-NNN`, `FIX-CONCURRENCY-date`, `FIX-CATAPI-date-WORD` (the census of the HEAD snapshot, specs/ and submodules/ excluded). Locator `<path>:L<n>#<id>` of the first occurrence (inside the S-14/S-15 file sets when there is one, which keeps the class), title the line. Carriers that quote ids as examples are not scanned: `specs/`, `.audit/`, `submodules/`, `scripts/register/tests/`. The id-like tokens no family matches (placeholders such as `FIX-QA-YYYY-MM-DD-NNN`, a bare date prefix) are listed in the stats (`id_like_unmatched`), never dropped silently.
4. **Checkbox lines (finding A3).** The Markdown files with checkbox lines that no checkbox source enumerates (procedural checklists among them) are LISTED with their counts in `checkbox_lines_not_enumerated` of the stats (doc03 3.1 rule 2: "listed and excluded explicitly rather than silently ignored").
5. Writes `<out>/source_entries.sql`, `<out>/source_entries.sql.sha256` (the T165 name) and `<out>/source_entries.sha256` (the **same one bare-hash line, under the name `scripts/register/locked.sh import-sql` reads**: its sibling is `<stem>.sha256`, stem = file name without `.sql`), `<out>/source-class-kinds.json` and `<out>/enumeration-stats.json` (per-class files and entries, skipped files by reason, empty sources, parse problems, the strict-YAML refusal count, the id census residue: honest gaps, never silent).

Output is deterministic: two runs over one snapshot give a byte-identical SQL file. `scanned_at` comes from `frozen_at` of the freeze json (`1970-01-01T00:00:00Z` and a stats note otherwise).

## The SQL file and re-import safety (finding B2)

The file starts with `.bail on`, creates a temp table with every entry, then runs a **gate**: `stale_entries_changed_at_same_locator` is the count of entries that already exist in the register at the same `(source, locator)` with a DIFFERENT `entry_sha256`; a non-zero count violates a CHECK, `.bail` stops the import and the open transaction is rolled back (all or nothing). `INSERT OR IGNORE` alone would keep the stale row and update the source's hash and count to the new scan without a signal. An unchanged re-import prints nothing and changes nothing. A re-scan of a moved commit therefore needs an explicit reconciliation, not a silent merge. `locked.sh import-sql` runs `sqlite3 <DB> ".read <file>"`, so the dot-command and the gate work there unchanged (**UNCONFIRMED end to end**: the real `locked.sh import-sql` journal run was not made, see `$EV/wp20/fix-r2-open-requests.md` R1).

## Honest boundaries

- External sources: the remotes are rows, not queried (`not_queried`); the named-service list is the closed list of doc03 5.20 and a row says only whether the marker files are in the snapshot.
- The S-23 scan covers every non-binary file up to 2 MiB whatever its extension; larger, binary and symlinked files are counted under `skipped` in the stats.
- The S-21 sub-item split is mechanical (indented bullets, bold status markers): doc03 5.17's `13a`-`13d` split of one inline OPEN list needs a reading no script makes; the item's status lists FIXED and OPEN both.
- The tolerant front-matter reader is **not a YAML parser**: 399 of the 1,778 docs/issues blocks are not valid YAML (OD-17, `$EV/wp20/fix-r2-open-requests.md` D1). `strict_frontmatter` measures it; no entry field comes from it.
- The id census is of the HEAD snapshot; a family a later commit adds shows first under `id_like_unmatched`.
- Rules version `enumerate_sources.py@2` (the `parser` column of every source row); a change of a rule changes it and is a reviewed edit.

## Tests

`scripts/register/tests/test_enumerate_absolute.py` (absolute contents: fields, statuses, locators, refusals, credentials, ids, external rows, the re-import gate on a real register DB), `test_enumerate_planted.sh` (planted entry per class, decoys, DB bytes, idempotency, moved snapshot, optional real-snapshot leg), `test_real_snapshot.py` (a real snapshot against independent grep instruments), `test_snapshot_checks.py`, `mutate_wf23.py` (the 26 reviewer mutants adapted plus the mutants of this pass) and `mutate_enumerate_sources.sh`; see `docs/scripts/register_wp20_tests.md`.
