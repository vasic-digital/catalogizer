#!/bin/bash
# Live check of SMB sub-directory storage roots (CAT-SMB-SUBDIR-1) against the real host Samba share Public, subdir Testing.
# Usage: run.sh <path-to-0600-env-file defining ANTON_HOST ANTON_USER ANTON_PASS>   (credentials are read by variable name, never printed)
# Not executed in CI. Target corpus: docs/qa/anton-smb-20261106/manifest.sha256.
set -eu
# Resolve to an absolute path FIRST: the mode check and the `.` below both use it, so neither can search PATH for a bare name.
ENVF="$(realpath -- "${1:?env file}")"
[ -f "$ENVF" ] || { echo "refusing: $ENVF is not a regular file" >&2; exit 2; }
[ "$(stat -c %a -- "$ENVF")" = "600" ] || { echo "refusing: $ENVF is not mode 0600" >&2; exit 2; }
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
(
  set -a; . "$ENVF"; set +a
  export SMB_LIVE_HOST="${ANTON_HOST:?}" SMB_LIVE_USER="${ANTON_USER:?}" SMB_LIVE_PASS="${ANTON_PASS:?}"
  cd "$ROOT/catalog-api"
  GOMAXPROCS=2 systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 \
    nice -n 10 go test -count=1 -tags live_smb -run 'TestLiveSMBSubdir|TestLiveSMBServiceSubdir' -v ./filesystem ./internal/services
)
