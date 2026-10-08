# lead_scan.py and lead_scan_population.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08T05:00:00Z |
| Status | tracked, new (WP-20, task T167); revision 2 is the single fix pass for the independent review WF23 of 2026-10-08 (findings C1-C7); the independent re-review of revision 2 is owed (constitution 11.4.142); the real-tree run, the owner's approval of the scope record and the `locked.sh import-sql` journal run are owed |
| Source | `scripts/register/lead_scan.py`, `scripts/register/lead_scan_population.py` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

Step 4 of the register reconciliation (doc03 section 10): a scripted pass over the tracked Markdown of the frozen snapshot, outside every gitlink, that lists the lines matching the closed lead vocabulary as one `reg_source_entries` row per lead line. It never opens a database for the scan; it writes `lead_scan.sql` (+ `lead_scan.sql.sha256` and `lead_scan.sha256`, the same bare hash, the second being the sibling `locked.sh import-sql` reads) and `lead-scan-run.json` to `--out`, and the host imports the SQL under the single-writer contract.

## Usage

```bash
python3 scripts/register/lead_scan.py --freeze-json <freeze.json> --out <dir>
python3 scripts/register/lead_scan.py --finalize <dir>/lead-scan-run.json --db <imported db> --rows-before <N>   # after the import
python3 scripts/register/lead_scan_population.py --freeze-json <freeze.json> [--count]
```

Freeze json (UNCONFIRMED vs T161): `snapshot`, `manifest`, `listing` (NUL-separated raw paths), `gitlinks` (`[{"path","sha"}]`), `frozen_at`, `head`. **There is no option that widens the population and no `--hc2`**: the owner's HC-2 answer is read from the fixed path `$EV/hc/HC-2.json` (EV = the evidence root; the default is the repository's own). An absent record is `[]`; an unreadable record, one that is not a JSON object, or a `lead_scan_extra_roots` that is not a list of strings is refused `hc2_malformed` (an owner answer is never lost silently).

## Behaviour

1. Entry checks, before any snapshot file is read: the listing against the manifest (`check_freeze_listing.sh` when T161 has written it, else an inline path-set comparison; `freeze_listing_moved`), then `check_freeze_snapshot.sh` (`freeze_snapshot_moved`, `freeze_manifest_invalid`, `freeze_special_file`, `freeze_path_unsafe`).
2. **Vocabulary (finding C1).** unfinished, not implemented / not-implemented, stub / stubs / stubbed / stubbing, missing, broken, known issue / known issues, workaround / workarounds, deprecated, disabled, skipped, TODO / todos, FIXME / fixmes, regress*, outstanding, pending, not yet / not-yet, case-insensitive, with word boundaries on both sides (so `depending`, `stubborn`, `todolist` are not leads); the inflections are listed, not a bare stem. The matched words are stored in `legacy_id` as canonical words.
3. Population (the one helper): listing entries ending in `.md` outside every gitlink path; the only widening is `lead_scan_extra_roots` of HC-2, each of which must be a gitlink path (`lead_scan_extra_root_invalid` otherwise). The roots applied, `hc2_path` and `population_files` go to `lead-scan-run.json`; a symlinked population file is skipped and listed under `symlink_population_files_skipped` (finding C7).
4. **Dispositions, decided per LINE** (in `raw_status`): `NON-PROBLEM:marker text in document or pattern` when every lead word of the line is a marker word (todo, fixme) and the line is a shell search command line (`grep`/`rg`/`awk`/`sed` before the marker word, or a command line such as `$ git grep TODO` inside a code fence); a line with another lead word, or a bare `- TODO: ...`, is not suppressed (finding C6); `duplicate of structured source` when a structured source (S-01..S-06, S-10, S-11, S-17..S-21, S-25) holds an entry that covers THIS line (a whole-file entry, an entry on this line, or the block of an S-21 item), with `raw_severity = covered_by=<source locator> :: <entry locator>` naming that entry, the row an importer folds the line into (finding C2); `lead` otherwise, including a line of a structured file that no structured entry covers.
5. **Control needles (finding C3).** (a) A built-in line set (one positive per word and inflection, eight look-alike negatives) goes through the SAME scan functions before the real scan; a vocabulary that lost a word is refused `lead_scan_blind`. (b) Each declared corpus needle (`NEEDLES`: file, text of a line that holds a lead word, that word, the disposition the line must get; now the `docs/LANDMINES.md` line "add the missing test; do not skip with") is located by a plain text search of the source file, never from the scan's rows: absent text is `lead_scan_needle_missing`, and a scan whose row for that line lacks the word or the disposition is `lead_scan_blind`.
6. **The count (finding C5).** `lead-scan-run.json` carries the exact command (interpreter, absolute script path, arguments), `emitted_rows`, the disposition counts and, after the import, `lead_scan_entry_rows`: `--finalize` opens the imported database read-only, counts the rows of the lead-scan source and records `rows_before`, `rows_after` and `after - before` (an idempotent second import records 0; an unreadable database is refused `finalize_failed`, never a recorded 0).
7. The SQL carries the same re-import gate as the enumerator's (`docs/scripts/enumerate_sources.md`): a changed line at an existing locator refuses the whole import.

## Tests

`scripts/register/tests/test_lead_scan.py` (absolute assertions: the vocabulary table, per-line duplicates with `covered_by`, the marker rule, HC-2 fixed path and refusals, the count in the imported database, symlinks, needles, the re-import gate), `test_real_snapshot.py` (the rows equal an independent `grep -i` word-boundary set over the real population) and the reviewer mutants in `mutate_wf23.py`; `mutate_lead_scan.sh` keeps the original seven sed mutations. See `docs/scripts/register_wp20_tests.md`.

## Honest boundaries

The false-positive mapping decision (docs/21 IC-39) and the scope are recorded as owner decisions awaiting approval in `$EV/wp20/non-problem-mapping.md` and `$EV/wp20/lead-scan-scope.md` (UNCONFIRMED approval). The run on the real frozen snapshot has not been made (T161 is not done); the numbers measured on a `git archive HEAD` snapshot are in `$EV/wp20/fix-r2-*`. `lead-scan-run.json` is written by the tool, and the ledger holds the command through the recorder (`fix-r2-ledger-*` transcripts); a recorder-written copy of the file itself is not claimed.
