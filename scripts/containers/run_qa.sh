#!/usr/bin/env bash
# run_qa.sh - T212. QA wrapper (IMG-QA: helixqa built from the pinned submodules/helix_qa, Tesseract, ffmpeg, Playwright browsers, python3 with PyYAML and pytest):
# runs scripts/qa/run_profile.sh, the bank-id floor regeneration, the analyzer self-test and the QA tooling tests from the READ-ONLY source mount.
# Runs ONE command in the pinned image `IMG-QA` through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at start, a registered
# long operation and a toolchain record: the T120 runner contract of runner_lib.sh (docs/scripts/runner_lib.md). An IMG-QA lock entry that is missing is REFUSED
# image_not_in_lock, one without a digest image_unpinned, one whose digest podman does not report image_digest_mismatch: never another image, never the bare host.
# Usage: run_qa.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
RUNNER_NAME=run_qa
RUNNER_IMAGES="IMG-QA"
RUNNER_TOOLCHAIN=qa
RUNNER_PROBE_MODE=sh
RUNNER_PROBE_VERSION='helixqa --version'
RUNNER_CMD_PREFIX=''
RUNNER_BLOCKED_NOTE='IMG-QA is built by T211 (BLOCKED on the owner host entry point ODG-07 and on its lock entry); until its digest is in build/containers/images.lock.yaml this wrapper refuses every run'
. "$(dirname "${BASH_SOURCE[0]}")/runner_lib.sh"
runner_main "$@"
