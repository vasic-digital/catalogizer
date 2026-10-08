#!/usr/bin/env bash
# nfs_attempt.sh - T134. Runs the unprivileged NFS attempt of docs/16 DR-16-2 end to end and records its ONE terminal state:
#   1. nfs_build.sh builds the user-space NFS server image (unfs3 + rpcbind) rootless on this host;
#   2. up.sh --services nfs starts it as a compose project on a private network (no privileged mode, no kernel mount, no host port);
#   3. roundtrip.sh --protocol nfs runs the libnfs round trip three times, each through tools/evidence/evrec (one ev/1 record per run, ledger under <ev-dir>/ledger-nfs);
#   4. down.sh removes the project by label;
#   5. nfs_terminal_state.sh (through `TIC tooling unit`) reads the client verdict FIRST (<client-json>, evidence wp11/nfs-client.json) and writes <ev-dir>/nfs-attempt.json.
# Usage:  nfs_attempt.sh --ev-dir <dir> --client-json <file> [--build-id <id>]
#   --ev-dir and --client-json are made absolute against the caller's cwd and must lie inside this checkout (the terminal-state check reads them through the /src mount of a TIC container).
# Exit:   0 a terminal state was recorded (the state itself is in the record); 1 the state could not be chosen or the stack could not be started; 2 usage (including a path outside the checkout).
# Every run starts from an EMPTY record set (WF17 TI-E1): the previous `nfs-attempt-observed.json`, `.rts` and ledger are removed before anything is observed, so a stale observed file can never be
# turned into a `pass` without a round trip. The observed file is written ONLY from this run (schema `nfs-attempt-observed/1`); each round trip cites the ledger sequence the recorder REPORTED for it
# (`recorded seq=<n>`), or `"record": null, "stored": false` when evrec stored nothing (a failed GREEN run exits 65 and stores no record: a pointer is never fabricated, WF17 TI-E2).
# The default build id carries the checkout hash, so two checkouts never share it.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
EVD=""; CLIENT=""; BID=""
while [ $# -gt 0 ]; do
  case "$1" in
    --ev-dir) ti_optval "$1" $# "${2:-}"; EVD=$2; shift 2;; --client-json) ti_optval "$1" $# "${2:-}"; CLIENT=$2; shift 2;; --build-id) ti_optval "$1" $# "${2:-}"; BID=$2; shift 2;;
    *) ti_die "unknown argument '$1'" 2;;
  esac
done
[ -n "$EVD" ] && [ -n "$CLIENT" ] || ti_die "--ev-dir and --client-json are required" 2
EVD="$(ti_abs "$EVD")"; CLIENT="$(ti_abs "$CLIENT")"; RT="$(realpath -m -- "$TI_ROOT")"
for pth in "$EVD" "$CLIENT"; do case "$pth" in "$RT"/*) ;; *) ti_refuse path_outside_repo "$pth is outside the checkout $RT: the terminal-state check can only read it through the /src mount" 2;; esac; done
[ -n "$BID" ] || BID="nfs$(date +%s | tail -c 7)-$(ti_root_hash | cut -c1-6)"
ti_valid_id "$BID" || ti_die "invalid build id" 2
mkdir -p "$EVD"; LEDGER="$EVD/ledger-nfs"; ATT="$EVD/nfs-attempt-observed.json"
# an EMPTY record set: nothing this run did not write survives into the terminal-state decision (WF17 TI-E1)
rm -rf -- "${LEDGER:?}" "$ATT" "$EVD/.rts" "$EVD/nfs-attempt.json" "$EVD"/nfs-roundtrip-run*.txt; mkdir -p "$LEDGER"
S="$(ti_state "$BID")"
# the operation of the start this script owns is persisted by up.sh at registration (<state>/op_id): down.sh refuses any other caller of a live lease (WF12 F6), and an interrupt between the start and
# the first printed line still leaves the id readable here (WF17 TI-C1)
nfs_down() { local op; op="$(head -1 "$S/op_id" 2>/dev/null)"; bash "$HERE/down.sh" --build-id "$BID" ${op:+--op-id "$op"} "$@"; }
ti_exit_on_signals
trap 'nfs_down >/dev/null 2>&1' EXIT
VERDICT="$(jq -r '.verdict // empty' "$CLIENT" 2>/dev/null)"
if [ "$VERDICT" = VERIFIED ]; then
  FAILED_START=0
  bash "$HERE/up.sh" --build-id "$BID" --services nfs >"$EVD/nfs-up.txt" 2>&1 || { FAILED_START=1; echo "nfs_attempt: the server did not start (see $EVD/nfs-up.txt)" >&2; }
  if [ "$FAILED_START" = 1 ]; then
    jq -n '{schema:"nfs-attempt-observed/1", round_trips:[{iteration:1, ok:false, record:null, stored:false}], failing_step:{side:"server", step:"server_start"}}' >"$ATT" || ti_die "cannot write $ATT" 1
  else
    rts="[]"
    for i in 1 2 3; do
      bash "$HERE/roundtrip.sh" --build-id "$BID" --protocol nfs --record "$LEDGER" --iteration "$i" >"$EVD/nfs-roundtrip-run$i.txt" 2>&1; rrc=$?
      seq="$(sed -n 's/^.*recorded seq=\([0-9][0-9]*\).*$/\1/p' "$EVD/nfs-roundtrip-run$i.txt" | tail -1)"
      if [ "$rrc" = 0 ]; then ok=true; else ok=false; fi
      if [ -n "$seq" ]; then rts="$(jq -c --argjson i "$i" --argjson ok "$ok" --arg rec "ledger-nfs/ledger.jsonl#seq$seq" --arg out "nfs-roundtrip-run$i.txt" '. + [{iteration:$i, ok:$ok, record:$rec, stored:true, output:$out}]' <<<"$rts")"
      else rts="$(jq -c --argjson i "$i" --argjson ok "$ok" --arg out "nfs-roundtrip-run$i.txt" '. + [{iteration:$i, ok:$ok, record:null, stored:false, output:$out}]' <<<"$rts")"; fi
    done
    jq -n --argjson r "$rts" '{schema:"nfs-attempt-observed/1", round_trips:$r}' >"$ATT" || ti_die "cannot write $ATT" 1
  fi
fi
nfs_down >"$EVD/nfs-down.txt" 2>&1
OUTD="$TI_ROOT/.audit/out/nfs-attempt-$$"; mkdir -p "$OUTD"
ARGS=(--client-json "/src/${CLIENT#"$RT"/}"); [ ! -s "$ATT" ] || ARGS+=(--attempt-json "/src/${ATT#"$RT"/}")
n=0
while :; do
  bash "$TI_ROOT/scripts/test-in-container.sh" --out "$OUTD" tooling unit -- bash /src/scripts/test-infra/nfs_terminal_state.sh "${ARGS[@]}" --out /out/nfs-attempt.json >"$EVD/nfs-state.log" 2>&1; rc=$?
  if [ "$rc" -ne 0 ] && grep -q 'reason=\(anti_mess_drift\|limit_exceeds_envelope\)' "$EVD/nfs-state.log" && [ "$n" -lt "$((10#${TI_TIC_RETRIES:-400}))" ]; then n=$((n+1)); sleep 5; continue; fi
  break
done
[ "$rc" = 0 ] && [ -s "$OUTD/nfs-attempt.json" ] || { echo "nfs_attempt: no terminal state recorded (rc=$rc): $(tail -2 "$EVD/nfs-state.log" | tr '\n' ' ')" >&2; rm -rf -- "${OUTD:?}"; exit 1; }
cp "$OUTD/nfs-attempt.json" "$EVD/nfs-attempt.json" || { echo "nfs_attempt: cannot copy the terminal-state record to $EVD" >&2; rm -rf -- "${OUTD:?}"; exit 1; }
rm -rf -- "${OUTD:?}"
echo "nfs_attempt: terminal state recorded: $(jq -r '.state + " " + .reason' "$EVD/nfs-attempt.json") -> $EVD/nfs-attempt.json"
