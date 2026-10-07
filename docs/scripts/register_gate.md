# gate.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T17:00:00Z |
| Status | tracked from WP-06 (T063, register gate, step 2) and WP-21 (T179 and T180, `--completion`, step 3); independent review of the `--completion` leg owed (constitution 11.4.142), the register-gate leg was reviewed in WP-06 |
| Source | `scripts/register/gate.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose
The register gate (docs/04 section 12.3 step 2) and the feature completion gate (docs/04 section 12.3 step 3, section 13.3). Step 2 says whether the register is internally sound. Step 3 says whether the feature may be called done: every row of every view registered as `view_not_done` in `reg_gate_checks` is open work, and today that is `v_reverify_queue` (closed legacy items that must be re-verified, owner decision D-1).

## Usage
```bash
scripts/register/gate.sh --db <db> [--ext <register_ext.sql>] [--allow-url-evidence]                 # step 2
scripts/register/gate.sh --db <db> [--ext <register_ext.sql>] [--allow-url-evidence] --completion    # steps 2 and 3
```
The database under test is never modified: every read goes through a read-only copy taken with the SQLite online backup.

## --completion (T180)
After the register steps the gate counts the rows of every `view_not_done` view. The list of views comes from the trusted reference database built from `register_ext.sql` (like the `view_empty` list), never from the database under test, so a registry row deleted from the register cannot hide the queue (the seed diff FAILs that deletion too, and the queue is still counted). Each name must match `^v_[a-z0-9_]+$` and must be a VIEW of the database under test; a count that is not a number is a FAIL, never zero rows.

Output, in order: one `NOT DONE <view> rows=N` line per populated view (name order), `INFO not_done checked=K`, then the verdict.

| Verdict line | Exit | Meaning |
|---|---|---|
| `COMPLETION OK` | 0 | register gate passed and every `view_not_done` view is empty |
| `COMPLETION NOT DONE views=V rows=R` | 4 | register gate passed, V views hold R rows in total |
| `GATE FAILED` | 1 | a register step failed; the `NOT DONE` lines (when counted) are still printed before it |
| `COMPLETION PASS-PARTIAL url_skipped=N (...)` | 3 (0 with `--allow-url-evidence`) | URL-shaped tracker receipts were counted, not re-hashed; never a bare `COMPLETION OK` |
| usage / absent database | 2 / 1 | as for step 2 |

Priority: 1 (register FAIL) over 4 (NOT DONE) over 3 (PASS-PARTIAL) over 0. A reference registry with no `view_not_done` row is a FAIL (a gate that checks nothing must not complete). Without `--completion` the gate is unchanged: `view_not_done` rows are not counted and a populated `v_reverify_queue` still prints `GATE OK`.

## Tests and evidence
- `scripts/register/tests/test_gate.sh` (T063, 126 checks) and `scripts/register/tests/test_reverify_gate.sh` (T179, 30 checks); mutation runners `mutate_gate.sh` and `mutate_reverify_gate.sh` (T180: gate.sh mutants plus `register_ext.sql` registry-row mutants, two negative controls that must NOT be killed).
- Evidence under `$EV/wp21/`: `RED-test_reverify_gate.txt`, `GREEN-test_reverify_gate-run1..3.txt`, `mut1/mut.tsv`, `ledger` entries, `SHA256SUMS`.

## Stated limits (11.4.6)
- The completion gate proves the queue is empty or names its size; it does not re-verify any closure.
- The container leg identity line of the test headers still reads `container_image_digest=UNCONFIRMED` (the shared `lib.sh` header text predates RUNP); the run itself was in IMG-TESTUTIL through `scripts/containers/run_pinned.sh`.
- `docs/scripts/README.md` does not list this page yet (owned by another stream); the link is owed.
