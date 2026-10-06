# run_rust.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | new in the working tree (T200a), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/containers/run_rust.sh` (a few lines on top of `runner_lib.sh`); test `scripts/containers/tests/test_run_rust_kcov.sh` |

## Purpose
Rust build, test and coverage wrapper (IMG-RUST; `cargo test`, `cargo llvm-cov` for `src-tauri` of the desktop app and the installer). Class `compile`: every call is a remote command composed by the T005b emitter, which sets `RUNNER_REMOTE_CALL=1` on the build host. A start on the local host is refused with exit 20 `compile_class_local` before anything else happens (no podman call, no registered operation, constitution 11.4.173). Only the literal value 1 counts.

## Contract
Otherwise the T120 runner contract of `runner_lib.sh` (toolchain record, long-op registration, anti-mess sweep at start, envelope limits, lock-pinned image). IMG-RUST is built by T143: until its lock entry exists a remote call is refused `image_not_in_lock` (BLOCKED), never run on another image. Lane rows: `catalogizer-desktop rust`, `installer-wizard rust` (3-column form until the T121a `site` column exists).
