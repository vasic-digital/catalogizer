#!/usr/bin/env bash
# down.sh - T131. Tear down ONE test-infrastructure project by its label: containers, network and volumes that carry
# `catalogizer.test_project=<project>` (and project=catalogizer), nothing else. A container without that label survives even when its name
# starts with the project name (the carrier case); another project's resources survive. Then the lease is released (the registered operation
# registered by up.sh (TI_OP_ID of the env file) gets the terminal state `complete`, the holder keeper exits by itself) and the per-run state directory is removed.
# Usage:  down.sh --build-id <id> [--keep-state] [--keep-logs]
#   --keep-state   leave <state>/<project> in place (credentials, data); default removes it with `podman unshare rm -rf` (the data files are owned by
#                  the sub-uids of the container user namespace and a plain rm cannot delete them)
#   --keep-logs    with the state removed, first move its log-*.txt and *.log files to <repo>/.audit/out/<project>-logs/
# Idempotent: a project that is not up is a no-op (exit 0). Never signals a process, never uses a name pattern, never `podman system prune`.
# Exit:   0; 2 usage; 1 a resource of this project could not be removed.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
BID=""; KEEPS=0; KEEPL=0
while [ $# -gt 0 ]; do
  case "$1" in --build-id) BID=${2:-}; shift 2;; --keep-state) KEEPS=1; shift;; --keep-logs) KEEPL=1; shift;; *) ti_die "unknown argument '$1'" 2;; esac
done
ti_valid_id "$BID" || ti_die "--build-id must match ^[a-z0-9][a-z0-9-]{0,30}\$" 2
ti_need podman jq
P="$(ti_project "$BID")"; S="$(ti_state "$BID")"; NET="$(ti_network "$BID")"
OPID="$(ti_env_get "$S/env" TI_OP_ID 2>/dev/null)"   # the operation this start registered (the env file is the record of it)
rc=0; nc=0; nn=0; nv=0
for c in $(podman ps -a -q --filter "label=catalogizer.test_project=$P" --filter "label=project=catalogizer" 2>/dev/null); do
  lab="$(podman inspect --format '{{index .Config.Labels "catalogizer.test_project"}}' "$c" 2>/dev/null)"
  [ "$lab" = "$P" ] || { echo "test-infra: skipping $c: label '$lab' is not $P" >&2; continue; }
  if podman rm -f -v "$c" >/dev/null 2>&1; then nc=$((nc+1)); else echo "test-infra: cannot remove container $c" >&2; rc=1; fi
done
for v in $(podman volume ls -q --filter "label=catalogizer.test_project=$P" 2>/dev/null); do
  if podman volume rm "$v" >/dev/null 2>&1; then nv=$((nv+1)); else rc=1; fi
done
if podman network exists "$NET" 2>/dev/null; then
  lab="$(podman network inspect "$NET" --format '{{index .Labels "catalogizer.test_project"}}' 2>/dev/null)"
  if [ "$lab" = "$P" ]; then if podman network rm "$NET" >/dev/null 2>&1; then nn=$((nn+1)); else echo "test-infra: cannot remove network $NET" >&2; rc=1; fi
  else echo "test-infra: network $NET is not labelled for $P, left alone" >&2; fi
fi
# lease: terminal state for the operation (a missing record is fine), then the keeper leaves by itself when its file goes
if [ -z "$OPID" ]; then   # state already gone: find this project's non-terminal operation in the registry (purpose key = project)
  LD="${LONGOPS_DIR:-$TI_ROOT/.audit/longops}"
  OPID="$(for f in "$LD"/ops/"$P"-up-*.json; do [ -e "$f" ] || continue; jq -r 'select(.purpose_key=="'"$P"'" and (.state|IN("complete","failed","reaped","handoff","blocked-escape")|not))|.op_id' "$f"; done | head -1)"
fi
[ -z "$OPID" ] || ti_lo release --op-id "$OPID" --state complete --verdict down >/dev/null 2>&1 || true
rm -f "$S"/lease.keep.* 2>/dev/null
if [ "$KEEPS" = 0 ] && [ -d "$S" ]; then
  case "$S" in "$TI_STATE_DIR"/catalogizer-test-*) ;; *) ti_die "refusing to remove the unexpected state path $S" 1;; esac
  if [ "$KEEPL" = 1 ]; then mkdir -p "$TI_ROOT/.audit/out/$P-logs" && cp "$S"/log-*.txt "$S"/*.log "$TI_ROOT/.audit/out/$P-logs/" 2>/dev/null; fi
  podman unshare rm -rf -- "${S:?}" 2>/dev/null || rm -rf -- "$S" 2>/dev/null || { echo "test-infra: cannot remove $S" >&2; rc=1; }
fi
echo "down: project=$P removed containers=$nc networks=$nn volumes=$nv"
exit "$rc"
