#!/usr/bin/env bash
# run_scan.sh - T120. Scanner wrapper (read-only source in, results in /out). Default image IMG-SHELLCHECK; --image selects another scanner image once the lock has it.
# Runs ONE command in the pinned image(s) `IMG-SHELLCHECK` or a scanner image (IMG-SCAN-TRIVY, -GITLEAKS, -TRUFFLEHOG, -SEMGREP, -GOSEC, -HADOLINT, -SYFT, -SONAR-SCANNER, -DEPCHECK) through scripts/containers/run_pinned.sh, with the dynamic envelope limits, the anti-mess sweep at
# start, a registered long operation and a toolchain record. The full contract (usage, refusals, exit codes, environment) is in runner_lib.sh and
# docs/scripts/run_scan.md.
# Usage: run_scan.sh [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID]
#        [--no-progress-s N] [--wall-s N] -- <command word>...
#   --image IMG-ID    IMG-SHELLCHECK (default) or one of the file-reader / analyser scanner images of the lock; the DAST and service images (IMG-SCAN-ZAP, -TESTSSL, -SONARQUBE) are not run here.
#                     An allowed image whose version command is not defined below is refused (probe_not_defined) until a reviewed row defines it.
# Entrypoint-only images (IMG-SHELLCHECK has no shell): the first command word names the entrypoint (run_pinned.sh enforces it) and the toolchain
# record carries the version line only, its three write probes are recorded `n/a:no_shell`.
# shellcheck disable=SC2034,SC2209  # the RUNNER_* variables are consumed by the sourced runner_lib.sh
set -u
RUNNER_NAME=run_scan
RUNNER_IMAGES="IMG-SHELLCHECK IMG-SCAN-TRIVY IMG-SCAN-GITLEAKS IMG-SCAN-TRUFFLEHOG IMG-SCAN-SEMGREP IMG-SCAN-GOSEC IMG-SCAN-HADOLINT IMG-SCAN-SYFT IMG-SCAN-SONAR-SCANNER IMG-SCAN-DEPCHECK"
RUNNER_TOOLCHAIN=scan
RUNNER_PROBE_MODE=version
RUNNER_PROBE_VERSION='shellcheck --version'
RUNNER_CMD_PREFIX=''
RUNNER_BLOCKED_NOTE='the scanner image has no lock entry yet (IMG-SHELLCHECK is pinned by T006; the other scanners by T149/T150); until it is pinned no run is possible and none is faked'
# the version command of each scanner image; an image with no entry here has no defined probe and is refused (probe_not_defined), never guessed
runner_probe_for_image() {
  case "$1" in
    IMG-SHELLCHECK) echo 'shellcheck --version';;
    *) echo '';;
  esac
}
. "$(dirname "${BASH_SOURCE[0]}")/runner_lib.sh"
runner_main "$@"
