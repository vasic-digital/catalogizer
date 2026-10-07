#!/usr/bin/env bash
# gen_env.sh - T129. Per-run credentials and random host ports for ONE compose project of the test-infrastructure stack.
# Writes two files and prints their paths:
#   <state>/<project>/env         every TI_* variable the compose files reference: generated credentials (high-entropy tokens from /dev/urandom),
#                                 the random host ports, the data directory, the project name, the op id. Mode 0600, inside the gitignored `.audit/`
#                                 tree, never under version control; it holds secrets and is never copied into the evidence.
#   <state>/<project>/ports.env   the non-secret subset (project, op id, ports): safe to copy into the evidence (`--ports-out`).
# Usage:  gen_env.sh --build-id <id> [--op-id <id>] [--env-out FILE] [--ports-out FILE]
# Exit:   0 written; 2 usage / invalid build id; 1 failure. The project is `catalogizer-test-<build_id>` (docs/16 section 10.4).
# The credentials are random per run: no literal credential exists in any tracked file, and the values are never printed or logged by this script.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
BID=""; OPID=""; ENVF=""; PORTF=""
while [ $# -gt 0 ]; do
  case "$1" in
    --build-id) BID=${2:-}; shift 2;; --op-id) OPID=${2:-}; shift 2;; --env-out) ENVF=${2:-}; shift 2;; --ports-out) PORTF=${2:-}; shift 2;;
    *) ti_die "unknown argument '$1'" 2;;
  esac
done
ti_valid_id "$BID" || ti_die "--build-id must match ^[a-z0-9][a-z0-9-]{0,30}\$" 2
ti_need python3 head base64 tr
P="$(ti_project "$BID")"; S="$(ti_state "$BID")"
[ -n "$OPID" ] || OPID="$P-up"
[ -n "$ENVF" ] || ENVF="$S/env"
[ -n "$PORTF" ] || PORTF="$S/ports.env"
tok() { head -c 64 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c "${1:-24}"; }
# six distinct free TCP ports (postgres, redis, ftp, smb, webdav, nfs): bind(0) one at a time and keep each until all are chosen (so they cannot repeat)
PORTS="$(python3 -I - <<'PY'
import socket
s = [socket.socket() for _ in range(6)]
for x in s: x.bind(("127.0.0.1", 0))
print(" ".join(str(x.getsockname()[1]) for x in s))
for x in s: x.close()
PY
)" || ti_die "cannot choose free ports"
set -- $PORTS
umask 077
mkdir -p "$(dirname "$ENVF")" "$(dirname "$PORTF")" || ti_die "cannot create the output directories"
TMP="$(mktemp "$(dirname "$ENVF")/.env.XXXXXX")" || ti_die "cannot create a temp file"
{
  echo "TI_PROJECT=$P"
  echo "TI_OP_ID=$OPID"
  echo "TI_DATA_DIR=$S/data"
  echo "TI_PORT_POSTGRES=$1"; echo "TI_PORT_REDIS=$2"; echo "TI_PORT_FTP=$3"; echo "TI_PORT_SMB=$4"; echo "TI_PORT_WEBDAV=$5"; echo "TI_PORT_NFS=$6"
  echo "TI_POSTGRES_DB=db$(tok 8)"; echo "TI_POSTGRES_USER=u$(tok 8)"; echo "TI_POSTGRES_PASSWORD=$(tok 28)"
  echo "TI_REDIS_PASSWORD=$(tok 28)"
  echo "TI_FTP_USER=u$(tok 8)"; echo "TI_FTP_PASSWORD=$(tok 28)"
  echo "TI_SMB_USER=u$(tok 8)"; echo "TI_SMB_PASSWORD=$(tok 28)"
  echo "TI_WEBDAV_USER=u$(tok 8)"; echo "TI_WEBDAV_PASSWORD=$(tok 28)"
  echo "TI_MINIO_ROOT_USER=u$(tok 8)"; echo "TI_MINIO_ROOT_PASSWORD=$(tok 28)"
} >"$TMP" || { rm -f "$TMP"; ti_die "cannot write the env file"; }
chmod 600 "$TMP" && mv -f "$TMP" "$ENVF" || { rm -f "$TMP"; ti_die "cannot place the env file"; }
{ echo "# non-secret subset of the per-run environment (docs/16 section 10.4)"; grep -E '^TI_(PROJECT|OP_ID|PORT_[A-Z]+)=' "$ENVF"; } >"$PORTF"
chmod 644 "$PORTF"
echo "env=$ENVF"; echo "ports=$PORTF"
