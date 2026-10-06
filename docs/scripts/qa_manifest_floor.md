# QA manifest and bank-id floor checks - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:45:00Z |
| Status | new in the working tree (WP-24), not yet committed; independent review owed (constitution 11.4.142, T219); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/qa/manifest_check.py` (T213), `scripts/qa/bank_id_floor_check.py` (T214); tests `scripts/qa/tests/test_manifest.sh`, `scripts/qa/tests/test_bank_id_floor.sh` |

## Purpose

`manifest_check.py` verifies `challenges/helixqa-banks/MANIFEST.yaml` (doc12 section 6.7): every bank file is listed, every listed file exists, each `case_floor` is at most the bank's case count, and no file of `scripts/`, `challenges/` or `tests/` names a bank file (QF-04), except the reviewed transitional list `scripts/qa/tests/manifest_legacy_drivers.txt` (a listed file that stops offending is reported stale). `bank_id_floor_check.py` is the directory scan of `.bank-id-floor.txt` (the loader's `checkBankIDFloor` rule): a deleted case is named and fails; adding a case never trips it.

## Usage

```bash
python3 scripts/qa/manifest_check.py --root REPO_ROOT [--legacy FILE]
python3 scripts/qa/bank_id_floor_check.py --banks DIR
```

## Honest boundary

The floor file was generated without `helixqa banks regen-floor` (IMG-QA is not built): the same algorithm (the default header of `loader.go` plus the sorted unique ids of the banks, 1,269) written by a script; running the real command in IMG-QA must reproduce it byte for byte, which is owed when T211 lands. Whether the loader accepts `MANIFEST.yaml` in a bank directory (it should decline a file with no cases, not abort) is UNCONFIRMED until helixqa runs.
