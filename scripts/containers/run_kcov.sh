#!/usr/bin/env bash
# run_kcov.sh - T200a. Bash coverage and interpreter-class wrapper (IMG-KCOV: kcov 43, bash, python3, jq; FROM IMG-TESTUTIL): the PS4 line-trace harness
# scripts/bash-coverage.sh, kcov cross-checks and the `build-scripts unit` lane. Class `interpreter`: it only reads the source (a read-only mount) and runs
# interpreters, nothing is compiled, so it runs LOCALLY (lane row site `local`, T121a).
# Runs ONE command in the pinned image `IMG-KCOV` through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at
# start, a registered long operation and a toolchain record. The full contract (usage, refusals, exit codes, environment) is in runner_lib.sh and
# docs/scripts/run_kcov.md.
# Usage: run_kcov.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
RUNNER_NAME=run_kcov
RUNNER_IMAGES="IMG-KCOV"
RUNNER_TOOLCHAIN=kcov
RUNNER_PROBE_MODE=sh
RUNNER_PROBE_VERSION='kcov --version'
RUNNER_CMD_PREFIX=''
RUNNER_BLOCKED_NOTE='IMG-KCOV is built locally by T006; this message is only reachable if its lock entry is removed'
. "$(dirname "${BASH_SOURCE[0]}")/runner_lib.sh"
runner_main "$@"
