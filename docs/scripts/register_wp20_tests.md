# WP-20 register source tests - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08T05:00:00Z |
| Status | tracked, new (WP-20, tasks T162 to T167); revision 2 is the single fix pass for the independent review WF23 of 2026-10-08; the independent re-review of revision 2 is owed (constitution 11.4.142); `test_import.sh` still needs T168 |
| Source | `scripts/register/tests/test_enumerate_planted.sh`, `test_enumerate_absolute.py`, `test_real_snapshot.py`, `test_snapshot_checks.py`, `test_import.sh`, `test_frontmatter_yaml.py`, `test_lead_scan.py`, `mutate_wf23.py`, `mutate_enumerate_sources.sh`, `mutate_frontmatter_yaml.sh`, `mutate_lead_scan.sh`, `record_wp20_evidence.py`, `wp20_fixture.py`, `wp20_ident.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

Tests of the Stage 0/1 register source pipeline (tasks T162 to T167). All use real SQLite, the real `$WI` engine binary where a database is involved, and scratch trees and databases under `$TMPDIR`; none touches `docs/workable_items.db`.

Why this set looks the way it does: the review WF23 found that every oracle of revision 1 was built from fixtures shaped like the implementation, so 23 of 26 author-independent mutants survived and the real corpus disagreed with the tools in several places (S-03 statuses all NULL, 21 prose-only ids missing, 181 inflected lead lines missed). Revision 2 therefore asserts what the entries ARE (absolute contents), checks a REAL snapshot against INDEPENDENT instruments (grep) after the instruments found a control needle, and keeps every reviewer mutant as a permanent test.

| File | Task | What it proves |
|---|---|---|
| `test_enumerate_planted.sh` | T162 | class-exhaustive control needle: every class (S-01..S-25, XID, EXT) reads > 0 on the baseline; one planted entry per class grows that class and a fresh DB by exactly 1; the 3-way plant (checkbox + ticket + bank case) gives exactly 3 rows by natural key and its removal FROM THE PLANTED COPY gives the baseline SQL byte for byte; decoy carriers add no row; the enumerator run leaves a scratch DB's bytes unchanged; the import is idempotent; the `.sha256` siblings agree; a moved snapshot is refused. `REAL_SNAPSHOT=<dir>` repeats the 3-way scenario on a copy of a real snapshot (T176) |
| `test_enumerate_absolute.py` | T162/T165 | absolute contents: S-01 fields, S-03 glyph statuses, S-21 block statuses and sub-items, S-25 names only, S-16 latest scan, checkbox entries of the docs/ files and the listed exclusions, the cross-source id index (prose-only ids, carriers excluded, one entry per id), external rows and the no-credential rule, markers in every file type, skipped-test locators, id-less bank cases, the front-matter forms, malformed front matter, `*`/`+` bullets, CRLF, duplicate locators, no `--check-script`, `pyyaml_missing`, `freeze_remotes_missing`, and the re-import gate on a real register DB (all or nothing) |
| `test_real_snapshot.py` | T162/T165/T167/T166 | needs `REAL_FREEZE_JSON`: the enumerator and the lead scan on a real frozen snapshot, imported into a scratch register; S-03/S-21 have no NULL status, the id entries equal an independent `grep -E` census, S-01 equals the listing, the checkbox entries equal `grep -c`, the marker files equal `grep -Iw`, the lead rows equal an independent `grep -i` word-boundary set, every duplicate row names an existing covering row, remote credentials never reach the SQL |
| `test_snapshot_checks.py` | T161 stand-ins | manifest rule (symlinks hashed over their target string), moved snapshot naming each path, FIFO refused, malformed manifest refused, the enumerator names the reason |
| `test_import.sh` | T163 | engine binary identity (step 0), (a) idempotency, (b) completeness through `v_unmapped_entries`, (c) legacy order, (d) the 2048-byte description cap (stored bytes decoded strictly, no U+FFFD, a prefix of the body); RED until `scripts/register/import.sh` (T168) exists |
| `test_frontmatter_yaml.py` | T164 | the tolerant reader loads every `docs/issues/*.md`, equals an independent `grep` recount (status, category) and an enhancement count taken from the PARSED parts, handles every front-matter form (RED on the doc04 naive splitter and on strict YAML), and states how many blocks strict YAML refuses (a FACT, OD-17; `--expect-strict-invalid N` asserts it); `--selftest` includes RED reader doubles that lose the body or the category |
| `test_lead_scan.py` | T167 | the lead scan: the vocabulary table (every word and inflection, look-alikes), planted and submodule lines, per-line duplicates with `covered_by`, the marker rule, HC-2 fixed path and malformed-record refusals, the count in the imported database (`--finalize`), symlinked population files, the needle refusals, the vocabulary control wired into the run, the re-import gate, moved snapshot and listing |
| `mutate_wf23.py` | all | the 26 WF23 reviewer mutants (EM01-EM11, LM01-LM06, FM01-FM04; GM01-GM05 below) adapted to the revised code plus the mutants of this pass; a comment-only mutant must survive and a deliberately wrong one must die |
| `mutate_enumerate_sources.sh`, `mutate_lead_scan.sh`, `mutate_frontmatter_yaml.sh` | T162-T167 | the round-1 sed mutation scripts, kept and re-aimed at the revised code |
| `record_wp20_evidence.py` | E1-E3 | records the RED (pre-fix tools), GREEN x3 and MUTATION runs of WP-20 and WP-21 into `$EV/ledger.jsonl` through `tools/evidence/evrec`, one entry per run, with the target fingerprint of the tool under test (a directory pair for RED/GREEN) |
| `wp20_fixture.py`, `wp20_ident.sh` | T162-T167 | the scratch tree builder (25 classes, plants, decoys, remotes, the front-matter forms) and the identity header whose container line says what the run really was |

GM01-GM05 are mutants of `scripts/register/gate.sh` run against `test_reverify_gate.sh` (WP-21, not edited in this pass): GM02, GM03, GM04 are killed, GM05 is equivalent by exit code, GM01 (numeric guard anchors) survives because the fixture's error text holds no digit; that test-instrumentation finding (D6) is open, see `$EV/wp20/fix-r2-open-requests.md` R7.

## Usage

```bash
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1
scripts/containers/run_pinned.sh IMG-TESTUTIL -- python3 scripts/register/tests/test_enumerate_absolute.py
scripts/containers/run_pinned.sh IMG-TESTUTIL -- bash scripts/register/tests/test_enumerate_planted.sh
scripts/containers/run_pinned.sh IMG-TESTUTIL -- python3 scripts/register/tests/test_frontmatter_yaml.py --selftest
scripts/containers/run_pinned.sh IMG-TESTUTIL -- python3 scripts/register/tests/test_lead_scan.py
REAL_FREEZE_JSON=<freeze.json with remotes> EXPECT_STRICT_INVALID=399 python3 scripts/register/tests/test_real_snapshot.py
python3 scripts/register/tests/mutate_wf23.py [--only NAME] [--suite absolute|lead|fm|planted|snap] [--list]   # ~25 minutes for the full run
python3 scripts/register/tests/record_wp20_evidence.py all                  # EV_MODE=host when no container fits the memory budget
```

## Findings recorded by these tests

- Strict `yaml.safe_load` of the front matter fails on 399 of the 1,778 `docs/issues` files (a colon inside a plain value, `mapping values are not allowed here`, all 399). The plan premise "a YAML parser loads every file" is false; the tolerant reader and the strict checker both ship and the test asserts both (OD-17 is the owner's, `$EV/wp20/fix-r2-open-requests.md` D1).
- The committed engine binary has no `--version` subcommand (exit 1 and the usage text); `test_import.sh` step 0 falls back to `$WI validate --db <scratch>` and says so.
- `reg_source_map` rows cannot be deleted while trigger `reg_source_map_no_delete` exists; leg (b) of `test_import.sh` drops that trigger in its scratch DB first.
- `scripts/register/locked.sh import-sql` takes ONE argument and reads the sibling `<stem>.sha256`; the T165 text says two argv elements and `<file>.sql.sha256`. The tools write both hash names; the contract difference is a request to the owner of `locked.sh` (R1), and the end-to-end journal run is UNCONFIRMED.
- A planted tree cannot be imported over the baseline register any more when a planted file changes the sha256 of its file entry: the re-import gate refuses it by design; the planted tests import into a copy of a pristine extension DB.
