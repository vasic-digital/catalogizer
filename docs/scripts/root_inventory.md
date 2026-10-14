# root_inventory.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-12 |
| Last modified | 2026-10-12T13:00:00Z |
| Status | tracked, new (WP-20, task T174); written by the worker, the independent review (constitution 11.4.142 / 11.4.209) is owed; nothing here writes the real register `docs/workable_items.db` |
| Source | `scripts/register/root_inventory.py`, `scripts/register/root_dispositions.yaml`, `scripts/register/check_freeze_listing.sh`; tests `scripts/register/tests/test_root_inventory.py`, mutations `mutate_w2.sh root_inventory` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The root-item inventory with dispositions (docs/21 section 12.4, the WP-20 output row, docs/01 section 3.8). It lists **every root entry of the frozen commit** and gives each
exactly one disposition from the closed set `problem_source` (with its source class), `component` (an application or module id), `documentation` (class assigned by WP-37,
T282) or `excluded` (with a reason), into `root-items.json`, one row per entry with its docs/01 inventory row or `absent`.

The entries come from the freeze only: the first path component of every NUL-terminated entry of the files-only listing named by `freeze.json`, the first component of every
`gitlinks` path, and the present untracked root entries recorded at freeze time (`untracked_root_entries`, the one explicit live-tree carve-out of T161). The tool runs no
`git` and no `ls`, reads no snapshot file, and never lists the live tree; its one freeze check is `check_freeze_listing.sh` (`freeze_listing_moved` stops it). A root entry
created after the freeze is therefore absent (needle). `.git` is not a root entry.

## Usage

```bash
scripts/register/root_inventory.py --freeze-json <freeze.json> --out <dir> [--dispositions root_dispositions.yaml] [--inventory <docs/01 md>]
scripts/register/root_inventory.py --check <root-items.json> [--freeze-json <freeze.json>]
```

The decisions are the rules of `root_dispositions.yaml` (first matching rule wins). An entry no rule decides is written with `disposition: null` and the run exits 1
("no row without a disposition", docs/21 WP-20). Exit: 0 PASS, 1 a row without a disposition or an invalid row, 2 usage, 20 refusal (nothing written). Nothing is moved,
removed or edited (11.4.122, 11.4.124).

## Tests and mutations

`test_root_inventory.py` (49 checks, including a probe of the rules against the root entries of git HEAD, `tests/head_root_entries.tsv`, names only, and a control that the rules without the review-fix block fail on exactly the eight entries the review named): closed sets, one row per entry, the freeze-only needle, `.git` refused, governance files are not a source class, untracked rows, the rule
table rejected when it names a disposition outside the closed set or omits the field a disposition needs, determinism, the docs/01 lookup. The mutation set (26 entries,
`mutate_w2.sh root_inventory`) covers every refusal.

## Honest boundary

The dispositions are decisions of the rule table, reviewed by an independent reviewer; the real-listing leg (`REAL_FREEZE_JSON`) waits for the T161 freeze (UNCONFIRMED until then).
