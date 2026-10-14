# recount_sources.sh (recount_sources.py) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-12 |
| Last modified | 2026-10-12T13:00:00Z |
| Status | tracked, new (WP-20, task T166); written by the worker, the independent review (constitution 11.4.142 / 11.4.209) is owed; nothing here writes the real register `docs/workable_items.db` |
| Source | `scripts/register/recount_sources.sh`, `scripts/register/recount_sources.py`; tests `scripts/register/tests/test_recount.py`, mutations `mutate_w2.sh recount` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The independent recount of the Stage 0 source entries (doc04 section 13.3 item 2; doc03 section 15 item 1). A second tool, sharing no code with `enumerate_sources.py`,
counts every source of the register in the **frozen snapshot** with `xargs -0 grep -c -H -Z ... --` (and `grep -o`, `awk` or a plain re-read where a count is not a line
count) and compares the result with `reg_sources.scanned_entry_count`. An exact match per source is PASS; any difference is itemised with the differing locators and the tool
exits non-zero until each is explained in the record. The per-source totals are compared with the doc03 section 7 table and every delta is itemised.

The sources it recounts include the plan-document seeds (S-26, awk over the docs/21 section 9.1 table, keyed by the docs/21 path) and the innovation entries (S-27, awk over
the section 9.5 table).

## Usage

```bash
scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- scripts/register/recount_sources.sh \
  --freeze-json <freeze.json> --db <register db> --out <dir> [--explanations <json>] [--doc03 <file>]
```

Order of work: `check_freeze_listing.sh` (`freeze_listing_moved`), then `check_freeze_snapshot.sh` (`freeze_snapshot_moved`); either refusal stops the recount before any
count and writes nothing. Every command that receives listing paths as arguments gets `--` before them (a root file named `-n` is a path, not an option).

| Exit | Meaning |
|---|---|
| 0 | every source matches or is explained |
| 1 | a source differs without an explanation (itemised in `recount.json` and on stdout), or has no recount method (`no_recount_method`) |
| 2 | usage |
| 20 | refusal (`recount: REFUSED reason=<code>`), nothing written |

## Tests and mutations

`test_recount.py`: (a) the base tree recounts every source and matches, no source reads zero (a blind instrument reads zero everywhere); (b) the `-n` fixture with and without
`--`; (c) a snapshot changed after its manifest; (d) a listing with an appended path; (e) one control needle per source class S-01..S-27 and XID (the recount of a tree with one
planted entry differs from the register at exactly that source by +1 and itemises it); (f) explanations; (g) determinism and read-only; (h) a path holding a newline; (i) the
doc03 section 7 comparison. The mutation set (`tests/mutate_w2.py recount`, 28 entries) runs through `tests/mutate_w2.sh recount`.

## Honest boundary

Methods marked approximate (`exact: false`) restate a rule grep cannot express exactly (bank case nesting, conflict sub-items, scan records): on the real corpus a difference of
those is itemised and must be explained, never silently accepted. The real-snapshot run waits for the T161 freeze (UNCONFIRMED until then).


Review fix round 2: `recount.json` carries `fixture` (true unless the freeze head is a full 40-hex git sha; derived, never typed); `test_recount.py` has 54 checks, the mutation set 27 mutants.
