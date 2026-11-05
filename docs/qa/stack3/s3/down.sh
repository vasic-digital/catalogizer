#!/usr/bin/env bash
# down.sh - stop the live S3 server and remove ITS container + network only. Pass --purge to also delete the state dir.
set -euo pipefail
S3_STATE="${S3_STATE:-$HOME/.cache/claude-tmp/catqa-s3live}"
: "${S3_STATE:?}"
podman rm -f catqa-s3live-garage >/dev/null 2>&1 || true
podman network rm catqa-s3live >/dev/null 2>&1 || true
if [ "${1:-}" = "--purge" ]; then
  case "$S3_STATE" in "$HOME"/.cache/claude-tmp/catqa-s3live*) rm -rf -- "${S3_STATE:?}" ;; *) echo "refusing to purge $S3_STATE" >&2; exit 2 ;; esac
fi
echo "garage down"
