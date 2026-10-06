#!/usr/bin/env bash
# capture.sh - run one command and store its output with an identity header (head, run_at, sha256 of the files under test, command, exit code).
# Usage: capture.sh <out-file> <title> <file-under-test,file2,...> -- <command word>...
set -u
out=$1; title=$2; files=$3; shift 4
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
trap - EXIT; rm -rf -- "${TI_SCRATCH:?}"   # the sourced lib made a scratch dir and an EXIT trap; this wrapper needs neither
IFS=, read -r -a fl <<<"$files"
mkdir -p "$(dirname "$out")"
{ ti_identity "$title" "${fl[@]}"; echo "# command: $*"; "$@" 2>&1; echo "# exit: $?"; } >"$out"
tail -n 3 "$out"
