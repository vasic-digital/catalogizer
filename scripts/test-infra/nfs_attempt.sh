#!/usr/bin/env bash
# nfs_attempt.sh - T134. Runs the unprivileged NFS attempt of docs/16 DR-16-2 end to end and records its ONE terminal state:
#   1. nfs_build.sh builds the user-space NFS server image (unfs3 + rpcbind) rootless on this host;
#   2. up.sh --services nfs starts it as a compose project on a private network (no privileged mode, no kernel mount, no host port);
#   3. roundtrip.sh --protocol nfs runs the libnfs round trip three times, each through tools/evidence/evrec (one ev/1 record per run, ledger under <ev-dir>/ledger-nfs);
#   4. down.sh removes the project by label;
#   5. nfs_terminal_state.sh (through `TIC tooling unit`) reads the client verdict FIRST (<client-json>, evidence wp11/nfs-client.json) and writes <ev-dir>/nfs-attempt.json.
# Usage:  nfs_attempt.sh --ev-dir <dir> --client-json <file> [--build-id <id>] [--attempt-json-out <file>]
# Exit:   0 a terminal state was recorded (the state itself is in the record); 1 the state could not be chosen or the stack could not be started.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
EVD=""; CLIENT=""; BID=""
while [ $# -gt 0 ]; do case "$1" in --ev-dir) EVD=${2:-}; shift 2;; --client-json) CLIENT=${2:-}; shift 2;; --build-id) BID=${2:-}; shift 2;; *) ti_die "unknown argument '$1'" 2;; esac; done
[ -n "$EVD" ] && [ -n "$CLIENT" ] || ti_die "--ev-dir and --client-json are required" 2
[ -n "$BID" ] || BID="nfs$(date +%s | tail -c 7)"
ti_valid_id "$BID" || ti_die "invalid build id" 2
mkdir -p "$EVD"; LEDGER="$EVD/ledger-nfs"; rm -rf -- "${LEDGER:?}"; mkdir -p "$LEDGER"
ATT="$EVD/nfs-attempt-observed.json"
OPID=""   # the operation of the start this script owns (up.sh prints `op_id=`): down.sh refuses any other caller of a live lease (WF12 F6)
trap 'bash "$HERE/down.sh" --build-id "$BID" ${OPID:+--op-id "$OPID"} >/dev/null 2>&1' EXIT
VERDICT="$(jq -r '.verdict // empty' "$CLIENT" 2>/dev/null)"
if [ "$VERDICT" = VERIFIED ]; then
  bash "$HERE/up.sh" --build-id "$BID" --services nfs >"$EVD/nfs-up.txt" 2>&1 || { echo '{"round_trips":[{"iteration":1,"ok":false}],"failing_step":{"side":"server","step":"server_start"}}' >"$ATT"; echo "nfs_attempt: the server did not start (see $EVD/nfs-up.txt)" >&2; }
  OPID="$(sed -n 's/^op_id=//p' "$EVD/nfs-up.txt" | head -1)"
  if [ ! -s "$ATT" ]; then
    rts="["; sep=""
    for i in 1 2 3; do
      if bash "$HERE/roundtrip.sh" --build-id "$BID" --protocol nfs --record "$LEDGER" --iteration "$i" >"$EVD/nfs-roundtrip-run$i.txt" 2>&1; then ok=true; else ok=false; fi
      rts="$rts$sep{\"iteration\":$i,\"ok\":$ok,\"record\":\"ledger-nfs/ledger.jsonl#seq$i\",\"output\":\"nfs-roundtrip-run$i.txt\"}"; sep=","
    done
    printf '%s]' "$rts" >"$EVD/.rts"
    jq -n --slurpfile r <(cat "$EVD/.rts") '{round_trips:$r[0]}' >"$ATT"; rm -f "$EVD/.rts"
  fi
fi
bash "$HERE/down.sh" --build-id "$BID" ${OPID:+--op-id "$OPID"} >"$EVD/nfs-down.txt" 2>&1
OUTD="$TI_ROOT/.audit/out/nfs-attempt-$$"; mkdir -p "$OUTD"
ARGS=(--client-json "/src/${CLIENT#"$TI_ROOT"/}"); [ ! -s "$ATT" ] || ARGS+=(--attempt-json "/src/${ATT#"$TI_ROOT"/}")
n=0
while :; do
  bash "$TI_ROOT/scripts/test-in-container.sh" --out "$OUTD" tooling unit -- bash /src/scripts/test-infra/nfs_terminal_state.sh "${ARGS[@]}" --out /out/nfs-attempt.json >"$EVD/nfs-state.log" 2>&1; rc=$?
  if [ "$rc" -ne 0 ] && grep -q 'reason=\(anti_mess_drift\|limit_exceeds_envelope\)' "$EVD/nfs-state.log" && [ "$n" -lt "${TI_TIC_RETRIES:-400}" ]; then n=$((n+1)); sleep 5; continue; fi
  break
done
[ "$rc" = 0 ] && [ -s "$OUTD/nfs-attempt.json" ] || { echo "nfs_attempt: no terminal state recorded (rc=$rc): $(tail -2 "$EVD/nfs-state.log" | tr '\n' ' ')" >&2; rm -rf -- "${OUTD:?}"; exit 1; }
cp "$OUTD/nfs-attempt.json" "$EVD/nfs-attempt.json"; rm -rf -- "${OUTD:?}"
echo "nfs_attempt: terminal state recorded: $(jq -r '.state + " " + .reason' "$EVD/nfs-attempt.json") -> $EVD/nfs-attempt.json"
