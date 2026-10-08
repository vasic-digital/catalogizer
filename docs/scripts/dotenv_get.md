# dotenv_get.py (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T16:40:48Z |
| Status | new in WF17 fix round 5, in the working tree (not committed); independent review owed (constitution 11.4.142 / 11.4.209) |
| Source | `scripts/test-infra/dotenv_get.py`; tests `tests/infra/test_blocked_external.sh`, `tests/infra/test_nas_readonly_leg.sh` |

## Purpose

The ONE dotenv reader of the test infrastructure (WF17 TI-D11). `nas_readonly_leg.sh` and `blocked_external.sh` used different `sed` pipelines and disagreed about the same file; both now call this script.

## Usage

```bash
python3 -I scripts/test-infra/dotenv_get.py <env file> <VARIABLE>
```

Prints the value of the LAST assignment of `<VARIABLE>` and exits 0; prints nothing and exits 1 when the variable is unset, 2 on a usage error or an unreadable file. Dialect: CRLF line ends, an optional `export` prefix, spaces around `=`, single and double quotes, and inline ` #` comments after an unquoted value; comment lines and blank lines are skipped. The value is written to stdout only, never to a log, and the script never sources the file. It was compared with python-dotenv on a mixed file (same values for every supported form).
