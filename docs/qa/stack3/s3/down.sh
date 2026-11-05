#!/usr/bin/env bash
# down.sh - stop the live S3 server and remove ITS container + network only. Pass --purge to also delete the state dir.
# S3_PURGE_ROOT is TEST-ONLY (the sandboxed test points it into a mktemp dir); operators must not set it.
# --purge is guarded (§11.4.287): the state dir is canonicalised with readlink -m and must be strictly below the fixed root
# $S3_PURGE_ROOT (default $HOME/.cache/claude-tmp) with a basename starting "catqa-s3live"; empty values and '..' segments are refused.
set -euo pipefail
S3_STATE="${S3_STATE-$HOME/.cache/claude-tmp/catqa-s3live}"
ROOT="${S3_PURGE_ROOT:-$HOME/.cache/claude-tmp}"
if [ "${1:-}" != "--purge" ]; then
  podman rm -f catqa-s3live-garage >/dev/null 2>&1 || true
  podman network rm catqa-s3live >/dev/null 2>&1 || true
  echo "garage down"; exit 0
fi
purge_ok() {
  local s="$1" r="$2" c rr
  [ -n "$s" ] && [ -n "$r" ] || return 1
  case "/$s/" in */../*|*/./*) return 1 ;; esac
  c="$(readlink -m -- "$s")"; rr="$(readlink -m -- "$r")"
  [ -n "$c" ] && [ -n "$rr" ] && [ "$rr" != "/" ] || return 1
  [ "$(dirname -- "$c")" = "$rr" ] || return 1
  case "$(basename -- "$c")" in catqa-s3live*) return 0 ;; *) return 1 ;; esac
}
if ! purge_ok "$S3_STATE" "$ROOT"; then echo "refusing to purge '$S3_STATE'" >&2; exit 2; fi
podman rm -f catqa-s3live-garage >/dev/null 2>&1 || true
podman network rm catqa-s3live >/dev/null 2>&1 || true
rm -rf -- "${S3_STATE:?}"
echo "garage down, state purged"
