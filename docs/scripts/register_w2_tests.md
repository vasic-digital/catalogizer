# W2 register tests (T166, T170, T171, T172, T174) - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 (review fix round 2, constitution 11.4.276) |
| Created | 2026-10-12 |
| Last modified | 2026-10-12T13:25:06Z |
| Status | tracked, new (WP-20 bundle W2); written by the worker, the independent review (constitution 11.4.142 / 11.4.209) is owed; the real-snapshot legs wait for the T161 freeze (conductor-owned) |
| Source | `scripts/register/tests/{test_recount.py,test_root_inventory.py,test_seed_import.py,test_innovation_import.py,test_t172.py,test_freeze_listing.sh,w2_fixture.py,mutate_w2.py,mutate_w2_sets.py,mutate_w2.sh,record_w2_evidence.py,equivalent_w2_mutants.tsv}` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## What each suite proves

| Task | Tool (guide) | Suite | Checks | Mutation tool |
|---|---|---|---|---|
| T166 | `recount_sources.sh` ([guide](recount_sources.md)) | `test_recount.py` | 54 | `recount` |
| T170 | `import_plan.sh --class seeds` ([guide](import_plan.md)) | `test_seed_import.py` | 64 | `seeds_enum`, `seeds_import` |
| T171 | `import_plan.sh --class innovation` ([guide](import_plan.md)) | `test_innovation_import.py` | 51 | `innovation_enum`, `innovation_import` |
| T172 | `import_t172.sh` ([guide](import_t172.md)) | `test_t172.py` | 60 | `t172`, `t172_classifier` |
| T174 | `root_inventory.py` ([guide](root_inventory.md)) | `test_root_inventory.py` | 49 | `root_inventory` |
| T161 stand-in | `check_freeze_listing.sh` ([guide](check_freeze_listing.md)) | `test_freeze_listing.sh` | 24 | `freeze_listing` |

All suites use real files, real SQLite (`apply_ext.sh`), the real engine binary and scratch databases and snapshots only; the real register is never written. Each prints an
identity header (task, UTC time, uid, git head, whether the run was inside a container, the sha256 of the tool under test).

## Running

```bash
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1
scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- python3 -I /src/scripts/register/tests/test_t172.py     # one container at a time
bash scripts/register/tests/mutate_w2.sh <tool> [--ledger] [ID...]          # one rootless container per mutant; MUT_HOST=1 only for development
python3 scripts/register/tests/record_w2_evidence.py red|green|all [recount seeds innovation t172 root listing]   # RED and GREEN x3 into the evidence ledger
```

`W2_FAILFAST=1` stops a suite at its first failing check (the mutation runs use it: the first FAIL kills the mutant). Environment variables point a suite at the tool copy under test:
`RECOUNT_SH`, `IMPORT_PLAN`, `IMPORT_T172` (and `IMPORT_TICKETS`), `ROOT_INVENTORY`, `CHECK_FREEZE_LISTING`, `ENUMERATE_SOURCES`, `WI`, `SCRATCH_BASE`.

## Test-first record

RED = the suite against a SUT directory without the tool under test (T166: with the pre-fix `recount_sources.py` of git HEAD, whose S-26 / S-27 rows were keyed `total` instead of
by file); GREEN = the same command after the directory is filled with the tool set, three iterations on a different target fingerprint. The runs are ledger entries
(`tools/evidence/evrec`, items `AUD-T166`, `AUD-T170`, `AUD-T171`, `AUD-T172`, `AUD-T174`, `AUD-W2-LISTING`) with transcripts in `$EV/register/w2/`.

## Mutations

Each mutation set holds a negative control `N00` (the unmutated tree must pass), a comment-only control `C00` (must survive: the harness does not call every change a kill) and
author-written mutants (>= 15 per tool: recount 27, root_inventory 25, freeze_listing 17, seeds_import 24, innovation_import 24, t172 33, t172_classifier 14; the t172_classifier set targets `import_tickets.py` and is below the floor by itself, the T172 tool set (t172 + t172_classifier) is 47; the reviewer's own mutants A and B are t172 M02 and M03, verbatim); a survivor is listed with its argued reason in `equivalent_w2_mutants.tsv`, never silently skipped. The results are in `$EV/register/w2/`.

## Findings recorded by these tests

- The recount of S-26 and S-27 was keyed `total` (a whole-source key) while the register keys those entries by the docs/21 path: the recount differed on the first plan-document
  tree. Fixed in `recount_sources.py` (T166); the test fixture now carries the plan documents and a control needle per class, S-26 and S-27 included.
- `reg_source_map` is keyed by `entry_id`: one entry maps to exactly one item, a mapping is corrected by UPDATE and never deleted. T172 therefore groups the QA-instrument tickets
  BEFORE `import.sh` (an entry already mapped is skipped by T168) and refuses `qa_entry_individually_imported` afterwards; this order is a consequence of the schema and is
  recorded in the T172 guide.
- Title-only duplicates with an empty screen are never linked (an empty scope stays DISTINCT, as the calibrated matcher of T169 treats it).

## Review fix round 2 (independent review of 2026-10-12, NO-GO, findings F1-F9)

The conductor-visible changes of the fix pass (one pass, constitution 11.4.276; assessment in `$EV/register/w2/fix-r2-convergence.md`):
`import.sh` refuses `qa_grouping_not_run` before any mint (F1); the compound docs/18 evidence label takes its weakest part (F2); a member named twice inside one family is refused
(F3; the item bound is checked against the number docs/21 itself states, so M17 is killed and is not an argued equivalent); the QA-instrument classifier covers `wontfix` and the `## Resolution` body statement, with a member inventory of the live tree (F4,
`inventory_qa_classes.py`); the crash-then-import and mapped-elsewhere scenarios are in `test_t172.py` (F5); `root_dispositions.yaml` decides `constitution`, `.opencode`,
`.kimi-code` and the live untracked root entries, probed against the root entries of git HEAD in `tests/head_root_entries.tsv` (F6); every T172 record, `recount.json` and
`root-items.json` carry the freeze identity and a derived `fixture` marker (F7); snake_case platform names and a declared `Android TV` (F8); counts and mutant floors above (F9).
