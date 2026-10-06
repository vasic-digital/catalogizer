#!/usr/bin/env bash
# run_testutil.sh - T120. Test-utility wrapper (IMG-TESTUTIL: bash, git, sqlite3, python3, jq, jsonschema, PyYAML, pytest): register DDL tests, verifier and commit-push fixture tests, schema validation, governance and tooling Python.
# Runs ONE command in the pinned image(s) `IMG-TESTUTIL` through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at
# start, a registered long operation and a toolchain record. The full contract (usage, refusals, exit codes, environment) is in runner_lib.sh and
# docs/scripts/run_testutil.md.
# Usage: run_testutil.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
RUNNER_NAME=run_testutil
RUNNER_IMAGES="IMG-TESTUTIL"
RUNNER_TOOLCHAIN=testutil
RUNNER_PROBE_MODE=sh
RUNNER_PROBE_VERSION='python3 --version'
RUNNER_CMD_PREFIX=''
RUNNER_BLOCKED_NOTE='IMG-TESTUTIL is built by T006; this message is only reachable if its lock entry is removed'
. "$(dirname "${BASH_SOURCE[0]}")/runner_lib.sh"
runner_main "$@"
