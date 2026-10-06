#!/usr/bin/env bash
# nfs_terminal_state.sh - T134. Chooses the ONE terminal state of the unprivileged NFS attempt (docs/16 DR-16-2, finding D-10) from two records and writes it.
# Usage:  nfs_terminal_state.sh --client-json <nfs-client.json> [--attempt-json <attempt.json>] [--out <nfs-attempt.json>]
#   --client-json   the 11.4.270 verdict of the user-space NFS client (evidence wp11/nfs-client.json). It is read FIRST: its `verdict` field decides whether the
#                   round trip may be attempted at all. An absent file is REFUSED `nfs_client_verdict_missing` (exit 1), never defaulted.
#   --attempt-json  what the attempt observed: {"round_trips":[{"iteration":1,"ok":true,"record":"<ev/1 ledger entry ref>"},...],
#                   "failing_step":{"side":"server"|"client","step":"<name>","transcript":"<path>"},"client_version_transcript":"<path>"}
#   --out           where the record is written (default: stdout). One JSON object, schema `nfs-attempt/1`, with `state`, `reason`, `finding: "D-10"`.
# States (exactly one):
#   blocked                  reason nfs_client_unverified  - the client verdict is UNVERIFIED or AMBIGUOUS: no round trip is attempted, the record cites the
#                            sha256 of the client file and its verdict, and the leg is owed to T134a and the ODG-08 register item. Never a pass, never a
#                            structural-impossibility record (the client side is not the failing server step).
#   pass                     the client verdict is VERIFIED and the attempt holds >= 3 round trips, every one ok. The record states what that proves (a protocol round trip)
#                            and what it does not (the application's kernel-mount path).
#   structural_impossibility reason rootless_cannot_provide_kernel_nfs - the client verdict is VERIFIED, the failing step is on the SERVER side, and the client's
#                            `nfs-ls --version` transcript is attached. Scope bounded to the NFS server side; FTP, SMB and WebDAV are not covered.
# Refusals (exit 1, `nfs_terminal_state: REFUSED reason=<code>`): nfs_client_verdict_missing, nfs_client_verdict_unreadable, attempt_record_missing,
#   attempt_record_malformed, client_side_failure_is_an_image_defect (a failing CLIENT step is a defect of IMG-INFRA-CLIENT fixed by a reviewed change of its
#   Containerfile, not a terminal state), client_version_transcript_missing, attempt_inconclusive (no 3 passing round trips and no failing step).
# Needs bash, jq, sha256sum. Runs through `TIC tooling unit`. Exit 0 = a state was written.
set -u
CLIENT=""; ATT=""; OUT=""
usage() { echo "nfs_terminal_state: $1" >&2; exit 2; }
refuse() { echo "nfs_terminal_state: REFUSED reason=$1 ${2:-}" >&2; exit 1; }
while [ $# -gt 0 ]; do
  case "$1" in --client-json) CLIENT=${2:-}; shift 2;; --attempt-json) ATT=${2:-}; shift 2;; --out) OUT=${2:-}; shift 2;; *) usage "unknown argument '$1'";; esac
done
[ -n "$CLIENT" ] || usage "--client-json is required"
command -v jq >/dev/null 2>&1 && command -v sha256sum >/dev/null 2>&1 || usage "jq and sha256sum are required"
[ -r "$CLIENT" ] || refuse nfs_client_verdict_missing "$CLIENT"
VERDICT="$(jq -r '.verdict // empty' "$CLIENT" 2>/dev/null)"   # VERDICT-READ
case "$VERDICT" in VERIFIED|AMBIGUOUS|UNVERIFIED) ;; *) refuse nfs_client_verdict_unreadable "verdict field is '$VERDICT'";; esac
CSHA="$(sha256sum "$CLIENT" | cut -d' ' -f1)"
emit() { # emit <state> <reason> <extra-json-object>
  local rec; rec="$(jq -n --arg state "$1" --arg reason "$2" --arg verdict "$VERDICT" --arg sha "$CSHA" --argjson extra "$3" \
    '{schema:"nfs-attempt/1", finding:"D-10", state:$state, reason:$reason, client_verdict:{file:"wp11/nfs-client.json", verdict:$verdict, sha256:$sha}} + $extra')" || refuse record_write_failed
  if [ -n "$OUT" ]; then printf '%s\n' "$rec" >"$OUT" || refuse record_write_failed "$OUT"; else printf '%s\n' "$rec"; fi
  exit 0
}
case "$VERDICT" in
  UNVERIFIED|AMBIGUOUS) emit blocked nfs_client_unverified '{"round_trip_attempted":false,"owed_to":["T134a","ODG-08"],"listed_in":"T159"}';;
esac
[ -n "$ATT" ] && [ -r "$ATT" ] || refuse attempt_record_missing "${ATT:-<none>}"
jq -e '.round_trips | type == "array"' "$ATT" >/dev/null 2>&1 || refuse attempt_record_malformed "$ATT has no round_trips array"
OKN="$(jq '[.round_trips[] | select(.ok == true)] | length' "$ATT")"; ALLN="$(jq '.round_trips | length' "$ATT")"
if [ "$ALLN" -ge 3 ] && [ "$OKN" = "$ALLN" ]; then
  emit pass "" "$(jq -c '{round_trip_attempted:true, round_trips:.round_trips, proves:"an NFSv3 round trip between a user-space server and the libnfs user-space client, rootless and unprivileged", does_not_prove:"the application NFS path (syscall.Mount, a kernel NFS client): UNCONFIRMED on this host, see evidence wp12/nfs-codepath.md"}' "$ATT")"
fi
FSIDE="$(jq -r '.failing_step.side // empty' "$ATT")"
case "$FSIDE" in
  client) refuse client_side_failure_is_an_image_defect "step $(jq -r '.failing_step.step // "?"' "$ATT"): fix IMG-INFRA-CLIENT by a reviewed change, never record it as rootless_cannot_provide_kernel_nfs";;
  server)
    TR="$(jq -r '.client_version_transcript // empty' "$ATT")"
    [ -n "$TR" ] && [ -r "$TR" ] && grep -q 'nfs-ls' "$TR" || refuse client_version_transcript_missing "the nfs-ls --version transcript of the client must be attached"
    emit structural_impossibility rootless_cannot_provide_kernel_nfs "$(jq -c --arg tr "$(sha256sum "$TR" | cut -d' ' -f1)" '{round_trip_attempted:true, scope:"the NFS server side under rootless unprivileged podman; not FTP, SMB or WebDAV", failing_step:.failing_step, client_version_transcript_sha256:$tr}' "$ATT")";;
  *) refuse attempt_inconclusive "neither 3 passing round trips nor a failing step on record";;
esac
