#!/usr/bin/env bash
# nfs_fallback_state.sh - T134a. Chooses the terminal state of the owner-host NFS fallback leg and writes its record (docs/16 DR-16-2, finding D-10, ODG-08).
# Usage:  nfs_fallback_state.sh --attempt-json <nfs-attempt.json> --state not_needed|blocked [--out <nfs-fallback.json>]
#   --state   the state the caller claims; the check verifies the claim against the `state` field of the T134 record and refuses a claim the record does not support.
# States (exactly one):
#   not_needed  the T134 record has `state` == `pass` AND `proves_kernel_mount_path` == true: the user-space pass alone NEVER makes the fallback unnecessary (WF12 F7): the
#               unprivileged NFSv3 round trip (unfs3 + libnfs) does not exercise the application's own path (syscall.Mount, a kernel NFS client), and the owner-host leg exists to
#               confirm exactly that path. REFUSED `nfs_attempt_not_pass` (exit 1) when the state is not `pass`, REFUSED `nfs_pass_does_not_cover_kernel_mount` when it is a pass
#               that does not prove the kernel path (the only record this repository's T134 check writes).
#   blocked     the kernel mount path is UNCONFIRMED (the T134 record is a protocol-only pass, a structural-impossibility record or a `blocked` nfs_client_unverified record): the
#               fallback round trip (x3, on the remote build host or the owner NFS host that ODG-08 names) is owed; while ODG-08 is unanswered the record is `blocked` with reason
#               odg08_unanswered, the leg is owed to the ODG-08 register item and listed in T159. A protocol-only pass adds `unconfirmed`: the path that stays open.
#               (The `pass` of a run on that host is written by that run, not by this check.) blocked_external.sh's `nfs_owner_host` leg is DERIVED from this record (blocked_external.sh).
# Refusals (exit 1): attempt_record_missing, attempt_record_malformed, nfs_attempt_not_pass, nfs_pass_does_not_cover_kernel_mount, fallback_not_owed (`blocked` claimed while the
#   T134 record proves the kernel path). Needs bash, jq, sha256sum; runs through `TIC tooling unit`.
set -u
ATT=""; OUT=""; CLAIM=""
usage() { echo "nfs_fallback_state: $1" >&2; exit 2; }
refuse() { echo "nfs_fallback_state: REFUSED reason=$1 ${2:-}" >&2; exit 1; }
while [ $# -gt 0 ]; do case "$1" in --attempt-json) ATT=${2:-}; shift 2;; --state) CLAIM=${2:-}; shift 2;; --out) OUT=${2:-}; shift 2;; *) usage "unknown argument '$1'";; esac; done
[ -n "$ATT" ] || usage "--attempt-json is required"
case "$CLAIM" in not_needed|blocked) ;; *) usage "--state must be not_needed or blocked";; esac
command -v jq >/dev/null 2>&1 && command -v sha256sum >/dev/null 2>&1 || usage "jq and sha256sum are required"
[ -r "$ATT" ] || refuse attempt_record_missing "$ATT"
STATE="$(jq -r '.state // empty' "$ATT" 2>/dev/null)"   # STATE-READ
KERNEL="$(jq -r '.proves_kernel_mount_path // false' "$ATT" 2>/dev/null)"   # KERNEL-READ
[ -n "$STATE" ] || refuse attempt_record_malformed "$ATT has no state field"
SHA="$(sha256sum "$ATT" | cut -d' ' -f1)"
emit() { local rec; rec="$(jq -n --arg state "$1" --arg reason "$2" --arg st "$STATE" --arg sha "$SHA" --argjson extra "$3" '{schema:"nfs-fallback/1", finding:"D-10", task:"T134a", state:$state, reason:$reason, attempt_record:{file:"wp10/nfs-attempt.json", state:$st, sha256:$sha}} + $extra')" || refuse record_write_failed
  if [ -n "$OUT" ]; then printf '%s\n' "$rec" >"$OUT" || refuse record_write_failed "$OUT"; else printf '%s\n' "$rec"; fi; exit 0; }
case "$CLAIM" in
  not_needed) [ "$STATE" = pass ] || refuse nfs_attempt_not_pass "the T134 record has state '$STATE'"
    [ "$KERNEL" = true ] || refuse nfs_pass_does_not_cover_kernel_mount "the T134 pass is a user-space protocol round trip; the application's kernel mount path (syscall.Mount) is UNCONFIRMED and the owner-host leg stays owed"
    emit not_needed "" '{"fallback_run":false}';;
  blocked) { [ "$STATE" != pass ] || [ "$KERNEL" != true ]; } || refuse fallback_not_owed "the T134 record proves the kernel mount path"
    if [ "$STATE" = pass ]; then emit blocked odg08_unanswered '{"fallback_run":false,"owed_to":["ODG-08"],"listed_in":"T159","unconfirmed":"the application kernel NFS mount path (syscall.Mount): the T134 pass is a user-space protocol round trip only"}'
    else emit blocked odg08_unanswered '{"fallback_run":false,"owed_to":["ODG-08"],"listed_in":"T159"}'; fi;;
esac
