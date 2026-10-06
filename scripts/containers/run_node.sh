#!/usr/bin/env bash
# run_node.sh - T120. Node wrapper (catalog-web build and unit tests, TypeScript modules, catalogizer-api-client). BLOCKED until IMG-NODE is pinned in the lock (T106).
# Runs ONE command in the pinned image(s) `IMG-NODE` through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at
# start, a registered long operation and a toolchain record. The full contract (usage, refusals, exit codes, environment) is in runner_lib.sh and
# docs/scripts/run_node.md.
# Usage: run_node.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
RUNNER_NAME=run_node
RUNNER_IMAGES="IMG-NODE"
RUNNER_TOOLCHAIN=node
RUNNER_PROBE_MODE=sh
RUNNER_PROBE_VERSION='node --version'
RUNNER_CMD_PREFIX=''
RUNNER_BLOCKED_NOTE='IMG-NODE has no lock entry yet: the image is created by T106 (build/containers tree and its lock entries, built through the T005b dispatcher); until it is pinned no run is possible and none is faked'
. "$(dirname "${BASH_SOURCE[0]}")/runner_lib.sh"
runner_main "$@"
