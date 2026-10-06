# qa_bank_ratchet.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:45:00Z |
| Status | new in the working tree (WP-24), not yet committed; independent review owed (constitution 11.4.142, T219); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/qa/gates/qa_bank_ratchet.sh` (T210); test `scripts/qa/tests/test_gates.sh` |

## Purpose

The ratchet stage of the gates `CM-QA-BANK-NO-PLACEHOLDER` (rule R-2) and `CM-QA-CASE-ASSERTS` (rules R-1 and R-8). It runs the validator over the bank directory and FAILS when a `(bank, rule)` count of its rules rises above the baseline or a new `(bank, rule)` key appears, so a change fails only on a rise while WP-60 converts the banks.

## Usage

```bash
scripts/qa/gates/qa_bank_ratchet.sh CM-QA-BANK-NO-PLACEHOLDER|CM-QA-CASE-ASSERTS [--banks DIR] [--baseline FILE]
```

Default baseline `scripts/repo/validate_baselines/qa_banks.tsv` (a CPA table: not written by this change; the baseline is staged under the WP-24 evidence, `evidence/wp24/staged/qa_banks.tsv`). Exit 0 PASS, 1 FAIL, 2 REFUSED (usage, baseline absent, validator crashed: a blind gate is never PASS).

## Honest boundary

Registration as a ratchet stage of CPA S3 (`scripts/repo/validate_checks.tsv`, `scripts/repo/cpa_code.txt`) is owed to the CPA change set and is held on the T219 verdict. Paired mutations (add a placeholder; remove an assertion) are observed failing in `test_gates.sh`.
