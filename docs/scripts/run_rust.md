# run_rust.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T00:58:05Z |
| Status | new in the working tree (T200a), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed; review round 1 (WF11, 2026-10-06 UTC): fix I8 applied; independent re-review owed (constitution 11.4.142); a per-call emitter proof is OWED (an emitter change under scripts/build) |
| Source | `scripts/containers/run_rust.sh` (a few lines on top of `runner_lib.sh`); test `scripts/containers/tests/test_run_rust_kcov.sh` |

## Purpose
Rust build, test and coverage wrapper (IMG-RUST; `cargo test`, `cargo llvm-cov` for `src-tauri` of the desktop app and the installer). Class `compile`: it starts only on a designated build host (see the review round 1 section below: a root-owned attestation file naming this host; an earlier revision of this guide claimed the emitter sets `RUNNER_REMOTE_CALL=1`, which was false and is no longer honoured). A start anywhere else is refused with exit 20 `compile_class_local` before anything else happens (no podman call, no registered operation, constitution 11.4.173).

## Contract
Otherwise the T120 runner contract of `runner_lib.sh` (toolchain record, long-op registration, anti-mess sweep at start, envelope limits, lock-pinned image). IMG-RUST is built by T143: until its lock entry exists a remote call is refused `image_not_in_lock` (BLOCKED), never run on another image. Lane rows: `catalogizer-desktop rust`, `installer-wizard rust` (3-column form until the T121a `site` column exists).

## Review round 1 (WF11 I8, 2026-10-06 UTC): the class guard is a host fact, not a variable
The guard no longer trusts `RUNNER_REMOTE_CALL=1` (the paragraph above, written in round 7, said the emitter sets it; `git grep RUNNER_REMOTE_CALL -- scripts/build` finds nothing, so that claim was false, and any local caller could export it). The variable is ignored. The wrapper now starts only on a designated build host: `/etc/catalogizer/build-host` (a constant in the script, no environment variable can change it) must be a regular file (not a symlink), owned by uid 0, not writable by group or others, holding the exact line `build-host <short hostname>` of this host. Anything else is refused with exit 20 `compile_class_local` before any container call or registered operation. The test runs a sed-patched COPY of the script (the two constants) inside a mirror of the containers directory.

Honest limit (11.4.6): the attestation proves the host, not that a call came from the emitter; a local user on a build host can still start it. A per-call proof (a token the emitter mints and this script verifies) needs an emitter change under `scripts/build` and is recorded as an owed request: `$EV/wp23/review-r1/owed-requests.md`. The operator action to enable a build host: `printf 'build-host %s\n' "$(hostname -s)" | sudo tee /etc/catalogizer/build-host; sudo chmod 644 /etc/catalogizer/build-host` (host change, outside any agent's remit).
