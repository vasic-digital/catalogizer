#!/usr/bin/env bash
# run_go.sh - T120. Go build and test wrapper (catalog-api, Go submodules, race, bench, fuzz). The command is prefixed `env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1` (docs/16 6.2: the GOMAXPROCS=3 floor of scripts/run-race-detector.sh, no toolchain download, CGO for the race detector).
# Runs ONE command in the pinned image(s) `IMG-GO` through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at
# start, a registered long operation and a toolchain record. The full contract (usage, refusals, exit codes, environment) is in runner_lib.sh and
# docs/scripts/run_go.md.
# Usage: run_go.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
RUNNER_NAME=run_go
RUNNER_IMAGES="IMG-GO"
RUNNER_TOOLCHAIN=go
RUNNER_PROBE_MODE=sh
RUNNER_PROBE_VERSION='go version'
RUNNER_CMD_PREFIX='env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1'
RUNNER_BLOCKED_NOTE='IMG-GO is pinned by T006; this message is only reachable if its lock entry is removed'
. "$(dirname "${BASH_SOURCE[0]}")/runner_lib.sh"
runner_main "$@"
