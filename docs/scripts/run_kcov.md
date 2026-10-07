# run_kcov.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T00:58:05Z |
| Status | new in the working tree (T200a), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed; review round 1 (WF11, 2026-10-06 UTC): review round 1 touched only the sandbox control of the shared test; independent re-review owed (constitution 11.4.142) |
| Source | `scripts/containers/run_kcov.sh`; test `scripts/containers/tests/test_run_rust_kcov.sh` |

## Purpose
Interpreter-class wrapper over IMG-KCOV (kcov 43, bash, python3, jq; FROM IMG-TESTUTIL): the `build-scripts unit` lane, the bash line-trace harness `scripts/bash-coverage.sh` and its kcov cross-check. It only reads the source (read-only mount) and runs interpreters, so it runs locally. The full contract is `runner_lib.sh` (see `run_testutil.md`): one pinned image, envelope limits, anti-mess sweep, registered long operation, toolchain record.

## Review round 1 (WF11, 2026-10-06 UTC)
No behaviour change. The shared test `test_run_rust_kcov.sh` gained a mutation-sandbox control (an unmutated mirror must pass the body, so a CAUGHT mutation is the mutation and not a broken sandbox). The harness it wraps changed: see `bash-coverage.md` (path-aware attribution, absolute `--out`).
