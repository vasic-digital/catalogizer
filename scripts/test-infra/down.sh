#!/usr/bin/env bash
# down.sh - T131. Tear down ONE test-infrastructure project by its label: containers, network and volumes that carry
# `catalogizer.test_project=<project>` (and project=catalogizer), nothing else. A container without that label survives even when its name
# starts with the project name (the carrier case); another project's resources survive. The unlabelled podman-compose pod `pod_<project>` is removed too when it is empty
# (WF12 F2), and so are this project's `<project>-client` and `<project>-seed` output directories under <repo>/.audit/out (`<project>-logs` is the --keep-logs result and stays).
# Then the lease is released (the registered operation of up.sh gets the terminal state `complete`, the holder keeper exits by itself) and the per-run state directory is removed.
# Ownership (WF12 F6, 11.4.119 / 11.4.232 E): a project whose lease is held by a LIVE holder is torn down only by the caller that names that holder's operation (`--op-id`, the
# `op_id=` line up.sh printed); any other caller is REFUSED (`reason=not_lease_owner`, exit 5) and nothing is touched. A holder that is PROVEN dead (scripts/longops/reap.sh
# --purpose --dry-run: the recorded pid no longer matches /proc) is reaped by any caller: its operation becomes `reaped` (reap.sh --op-id, no signal at all). No claim: no owner to check.
# Usage:  down.sh --build-id <id> [--op-id <op id>] [--keep-state] [--keep-logs] [--outcome complete|failed] [--reason <code>]
#   --op-id        the operation id of the start this caller owns (up.sh prints `op_id=<id>`); required while the lease holder is alive
#   --keep-state   leave <state>/<project> in place (credentials, data); default removes it with `podman unshare rm -rf` (the data files are owned by
#                  the sub-uids of the container user namespace and a plain rm cannot delete them)
#   --outcome      how the operation is recorded: `complete` (default, an owner teardown) or `failed` (a failed start, up.sh passes it with --reason <code>, [a-z0-9_]{1,40}); a PROVEN-dead holder is always `reaped`
#   --keep-logs    with the state removed, first move its log-*.txt and *.log files to <repo>/.audit/out/<project>-logs/
# Idempotent: a project that is not up is a no-op (exit 0). Never signals a process, never uses a name pattern, never `podman system prune`.
# Exit:   0; 2 usage; 1 a resource of this project could not be removed, or a podman query failed (the state is UNKNOWN: nothing is released or deleted); 5 REFUSED (not_lease_owner,
#         holder_record_unreadable, start_in_progress, foreign_owner = a resource of the project is not this checkout's: nothing is removed, project_lock_busy).
# Contract (WF17 round 5): the per-user project lock is held for the WHOLE run; the lease is released LAST, after resources, output directories, logs, the state directory and the holder's own keeper file (a
#   new owner can therefore never be destroyed by a teardown that already released); the ownership predicate is lib.sh ti_scan (project + test_project + test_root + op in THIS registry); the registry path
#   is the one scripts/longops/lib.sh derives; the refusal never prints the owner's operation id (it names the 0600 file <state>/op_id).
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
BID=""; KEEPS=0; KEEPL=0; CALLER_OP=""; OUTCOME=complete; REASON=down
while [ $# -gt 0 ]; do
  case "$1" in
    --build-id) ti_optval "$1" $# "${2:-}"; BID=$2; shift 2;; --op-id) ti_optval "$1" $# "${2:-}"; CALLER_OP=$2; shift 2;; --outcome) ti_optval "$1" $# "${2:-}"; OUTCOME=$2; shift 2;;
    --reason) ti_optval "$1" $# "${2:-}"; REASON=$2; shift 2;; --keep-state) KEEPS=1; shift;; --keep-logs) KEEPL=1; shift;; *) ti_die "unknown argument '$1'" 2;;
  esac
done
ti_valid_id "$BID" || ti_die "--build-id must match ^[a-z0-9][a-z0-9-]{0,30}\$" 2
[ -z "$CALLER_OP" ] || ti_safe_token --op-id "$CALLER_OP"
case "$OUTCOME" in complete|failed) ;; *) ti_die "--outcome must be complete or failed" 2;; esac
[[ "$REASON" =~ ^[a-z0-9_]{1,40}$ ]] || ti_die "--reason must match ^[a-z0-9_]{1,40}\$" 2
ti_need podman jq flock realpath
P="$(ti_project "$BID")"; S="$(ti_state "$BID")"
ti_ld_init; LD="$TI_LD_CACHE"
ti_lock "$P"   # held for the whole run (released with the process): a start of the same project waits, a down of another checkout waits
# ---- ownership: the lease holder (re-read from the registry) is the caller's, or provably dead, or there is none ----
HOLDER=""; STALE=0
if [ -d "$LD/claims/$P" ]; then
  HOLDER="$(jq -r '.run_id // empty' "$LD/claims/$P/holder.json" 2>/dev/null)"
  if [ -z "$HOLDER" ]; then ti_refuse holder_record_unreadable "the claim of $P has no readable holder record ($LD/claims/$P/holder.json); scripts/longops/reap.sh --purpose $P --dry-run says what the registry decides" 5; fi
  if [ -n "$CALLER_OP" ] && [ "$CALLER_OP" = "$HOLDER" ]; then :
  elif DRY="$(ti_lo reap --purpose "$P" --dry-run 2>&1)" && printf '%s' "$DRY" | grep -q 'would release stale claim'; then STALE=1
  else ti_refuse not_lease_owner "$P is held by another owner (not provably dead: $(printf '%s' "$DRY" | tr '\n' ' ' | cut -c1-120)); the owner's operation id is in its 0600 file $S/op_id, pass it as --op-id" 5; fi
else
  # no claim: but a start that created its keeper and has not claimed yet must not be torn down under its feet
  if KF="$(ti_live_keeper_in "$S")"; then ti_refuse start_in_progress "a start of $P is in progress (live keeper $KF); wait for it or interrupt it" 5; fi
fi
rc=0
ti_rm_resources "$P"; rr=$?
case "$rr" in
  0) ;; 1) rc=1;;
  3) ti_die "the state of $P is UNKNOWN ($TI_UNKNOWN failed): nothing was released or deleted; retry when podman answers" 1;;
  5) exit 5;;   # ti_rm_resources printed REFUSED reason=foreign_owner
esac
nc=$TI_NC; nn=$TI_NN; nv=$TI_NV; np=$TI_NP
ti_rm_out_dirs "$P" || rc=1
[ -z "${TI_TEST_SLEEP_BEFORE_RELEASE:-}" ] || sleep "$TI_TEST_SLEEP_BEFORE_RELEASE"   # test hook (TI_TEST_MODE=1): widens the window between the cleanup and the release
if [ "$KEEPS" = 0 ] && [ -d "$S" ]; then
  case "$S" in "$TI_STATE_DIR"/catalogizer-test-*) ;; *) ti_die "refusing to remove the unexpected state path $S" 1;; esac
  if [ "$KEEPL" = 1 ]; then mkdir -p "$TI_ROOT/.audit/out/$P-logs" && cp "$S"/log-*.txt "$S"/*.log "$TI_ROOT/.audit/out/$P-logs/" 2>/dev/null; fi
  podman unshare rm -rf -- "${S:?}" 2>/dev/null || rm -rf -- "$S" 2>/dev/null || { echo "test-infra: cannot remove $S" >&2; rc=1; }
fi
[ "$KEEPS" = 0 ] || { [ ! -d "$S" ] || : >"$S/.keep-state"; }   # a deliberate result: sweep_leaks.sh never removes a state directory that carries this marker
# the holder's OWN keeper file only (never a glob: another start's file stays), then the lease LAST
[ -z "$HOLDER" ] || rm -f -- "$S/lease.keep.$HOLDER" 2>/dev/null
if [ -n "$HOLDER" ] && [ "$STALE" = 1 ]; then ti_lo reap --op-id "$HOLDER" >/dev/null 2>&1 || ti_lo reap --purpose "$P" >/dev/null 2>&1 || { echo "test-infra: cannot reap the dead holder of $P" >&2; rc=1; }
elif [ -n "$HOLDER" ]; then ti_lo release --op-id "$HOLDER" --state "$OUTCOME" --verdict "$REASON" >/dev/null 2>&1 || ti_lo release --purpose "$P" --run-id "$HOLDER" >/dev/null 2>&1 || true
else   # no claim: an operation record of this project that never reached a terminal state (purpose key = project) has a dead owner by definition (the claim is gone): it is recorded `reaped`, not `complete`
  for f in "$LD"/ops/"$P"-up-*.json; do [ -e "$f" ] || continue
    o="$(jq -r 'select(.purpose_key=="'"$P"'" and (.state|IN("complete","failed","reaped","handoff","blocked-escape")|not))|.op_id' "$f" 2>/dev/null)"
    [ -z "$o" ] || ti_lo reap --op-id "$o" >/dev/null 2>&1 || echo "test-infra: the operation $o of $P is not provably dead and has no claim; left as it is (scripts/longops/reap.sh --op-id $o --dry-run says why)" >&2
  done
fi
echo "down: project=$P removed containers=$nc networks=$nn volumes=$nv pods=$np"
exit "$rc"
