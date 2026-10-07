#!/usr/bin/env bash
# run_rust.sh - T200a. Rust build, test and coverage wrapper (catalogizer-desktop/src-tauri, installer-wizard/src-tauri: cargo test, cargo llvm-cov).
# Runs ONE command in the pinned image `IMG-RUST` through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at
# start, a registered long operation and a toolchain record (runner_lib.sh, docs/scripts/run_rust.md).
# CLASS `compile`: Rust is compiled, so this wrapper runs ONLY on a designated build host (constitution 11.4.173: no build on the bare developer host). A start anywhere else is
# REFUSED with exit 20 `compile_class_local` BEFORE anything else happens (no podman call, no registered operation, T121b).
# WHAT DECIDES (review round 1 of WP-23, WF11 I8): a HOST FACT the caller cannot set per call - the attestation file /etc/catalogizer/build-host, owned by root, writable by no
# one else, holding the exact line `build-host <short hostname>` of THIS host (the operator writes it once on each build host). The earlier guard trusted the environment
# variable RUNNER_REMOTE_CALL=1 and claimed the emitter sets it; the emitter (scripts/build/remote/emit.sh) never did, and any local caller could export it, so it enforced by
# instruction (11.4.240 B). The variable is now IGNORED. There is no test seam in this script: the test runs a sed-patched COPY (the two constants below).
# HONEST LIMIT (11.4.6): on a build host the attestation proves the HOST, not that this call came from the emitter; a local user of that host can still start it. A per-call
# proof (a token the emitter mints and this script verifies) needs an emitter change under scripts/build and is recorded as an owed request (evidence wp23/review-r1/owed-requests.md).
# IMG-RUST is built by T143: until its lock entry exists a call is refused `image_not_in_lock` (BLOCKED), never run on another image.
# Usage: run_rust.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
BUILD_HOST_ATTEST=/etc/catalogizer/build-host   # constant: no environment variable can point this elsewhere
ATTEST_OWNER_UID=0
build_host_attested() {   # 0 only when the file exists, is owned by ATTEST_OWNER_UID, is not writable by group or others, and names THIS host exactly
  [ -f "$BUILD_HOST_ATTEST" ] && [ ! -L "$BUILD_HOST_ATTEST" ] || return 1
  local owner mode
  read -r owner mode < <(stat -L -c '%u %a' "$BUILD_HOST_ATTEST" 2>/dev/null) || return 1
  [ "$owner" = "$ATTEST_OWNER_UID" ] || return 1
  [ $(( 8#$mode & 8#022 )) -eq 0 ] || return 1   # MUT:attest-perm
  grep -qxF "build-host $(hostname -s 2>/dev/null)" "$BUILD_HOST_ATTEST"
}
if ! build_host_attested; then   # MUT:class-compile
  echo "run_rust: REFUSED reason=compile_class_local run_rust.sh is class 'compile': it runs only on a designated build host (a root-owned $BUILD_HOST_ATTEST naming this host); the environment variable RUNNER_REMOTE_CALL is not honoured (11.4.173)" >&2
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
