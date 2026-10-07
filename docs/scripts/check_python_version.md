# check_python_version.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T17:00:00Z |
| Status | applied in the working tree (not committed); the independent review is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp11/owner-0107-*` |
| Source | `scripts/check_python_version.sh`, declaration `scripts/python-min-version`; test `scripts/tests/test_check_python_version.sh` |

## Purpose

Owner decision 2026-10-07 (2): the project tooling (the Python helpers under `scripts/`, the register, coverage, governance and review tools) requires **Python >= 3.11**.
The single declaration is `scripts/python-min-version` (one line, `X.Y`); this page and the check read it, nothing else repeats the number.

## Usage

```bash
scripts/check_python_version.sh [--python CMD] [--min X.Y]
```

`--python` defaults to `$PYTHON`, else `python3`. The interpreter is asked for `sys.version_info` through `python -I` (a module planted in the working directory is never
imported); `--version` text is never parsed.

| Exit | Meaning |
|---|---|
| 0 | interpreter >= the declared minimum |
| 1 | interpreter older than the minimum |
| 2 | usage or malformed declaration / `--min` (must be `X.Y`) |
| 3 | interpreter missing, not runnable, or its output unparseable (a blind probe never passes) |

## Where it applies

Run it before any tooling that needs the interpreter, on the host or inside the pinned tooling image (`scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- bash scripts/check_python_version.sh`).
The pinned image is the authoritative interpreter; the host `python3` is only a convenience.

## Test

`scripts/tests/test_check_python_version.sh`: fake interpreters (3.9, 3.10, 3.11, 3.12, 4.0), a dead interpreter, an unparseable one, a missing one, bad `--min`, the real
interpreter of the environment, and a paired mutation (inverted comparison).
