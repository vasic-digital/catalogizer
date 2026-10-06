#!/usr/bin/env bash
# run_rust.sh - T200a. Rust build, test and coverage wrapper (catalogizer-desktop/src-tauri, installer-wizard/src-tauri: cargo test, cargo llvm-cov).
# Runs ONE command in the pinned image `IMG-RUST` through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at
# start, a registered long operation and a toolchain record (runner_lib.sh, docs/scripts/run_rust.md).
# CLASS `compile`: Rust is compiled, so every call of this wrapper is a REMOTE command composed by the T005b emitter (scripts/build/remote/emit.sh),
# which sets RUNNER_REMOTE_CALL=1 on the build host; a start on the local host is REFUSED with exit 20 `compile_class_local` BEFORE anything else happens
# (no podman call, no registered operation, T121b). Only the literal value 1 counts: the variable is a statement by the emitter, never by a caller.
# IMG-RUST is built by T143: until its lock entry exists a remote call is refused `image_not_in_lock` (BLOCKED), never run on another image.
# Usage: run_rust.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
if [ "${RUNNER_REMOTE_CALL:-}" != 1 ]; then   # MUT:class-compile
  echo "run_rust: REFUSED reason=compile_class_local run_rust.sh is class 'compile': it runs only as a remote command composed by the build emitter (scripts/build/remote/emit.sh, RUNNER_REMOTE_CALL=1), never on the local host" >&2
  exit 20
fi
RUNNER_NAME=run_rust
RUNNER_IMAGES="IMG-RUST"
RUNNER_TOOLCHAIN=rust
RUNNER_PROBE_MODE=sh
RUNNER_PROBE_VERSION='cargo --version'
RUNNER_CMD_PREFIX=''
RUNNER_BLOCKED_NOTE='IMG-RUST is built by T143 (ODG-07); until then a Rust lane is blocked-unavailable with reason image_not_built, never run on another image'
. "$(dirname "${BASH_SOURCE[0]}")/runner_lib.sh"
runner_main "$@"
