#!/usr/bin/env bash
# run_docs.sh - T120. Documentation wrapper (link crawl, header and fingerprint checks, exports, diagram render). Runs while the lock has IMG-DOCS (it does: pinned by T106); without its lock entry the run is refused image_not_in_lock with a BLOCKED note, never faked.
# Runs ONE command in the pinned image(s) `IMG-DOCS` through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at
# start, a registered long operation and a toolchain record. The full contract (usage, refusals, exit codes, environment) is in runner_lib.sh and
# docs/scripts/run_docs.md.
# Usage: run_docs.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
RUNNER_NAME=run_docs
RUNNER_IMAGES="IMG-DOCS"
RUNNER_TOOLCHAIN=docs
RUNNER_PROBE_MODE=sh
RUNNER_PROBE_VERSION='python3 --version'
RUNNER_CMD_PREFIX=''
RUNNER_BLOCKED_NOTE='IMG-DOCS is pinned by T106; this message is only reachable if its lock entry is removed'
. "$(dirname "${BASH_SOURCE[0]}")/runner_lib.sh"
runner_main "$@"
