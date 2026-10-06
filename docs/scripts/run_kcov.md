# run_kcov.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | new in the working tree (T200a), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/containers/run_kcov.sh`; test `scripts/containers/tests/test_run_rust_kcov.sh` |

## Purpose
Interpreter-class wrapper over IMG-KCOV (kcov 43, bash, python3, jq; FROM IMG-TESTUTIL): the `build-scripts unit` lane, the bash line-trace harness `scripts/bash-coverage.sh` and its kcov cross-check. It only reads the source (read-only mount) and runs interpreters, so it runs locally. The full contract is `runner_lib.sh` (see `run_testutil.md`): one pinned image, envelope limits, anti-mess sweep, registered long operation, toolchain record.
