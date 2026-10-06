#!/usr/bin/env bash
# holder.sh - read-only: who holds a purpose, judged on /proc, never on a `pgrep` match (T088/T089; CENTRAL C2).
#
# Usage   holder.sh <purpose>
# Output  one JSON object while the holder lives (`status` `live`) or its resume window has run out (`status` `expired`): kind,
#         run_id, pid, start_time (ticks, /proc/<pid>/stat field 22), state, plus builds/callback_state for a `suspended-run`;
#         the word `none` otherwise (no claim, a dead holder, a pid that now belongs to another process).
# Kinds   process      live while pid and start time still match /proc and the process is not a zombie.
#         suspended-run  (T121b: the CPA process exited 16 while its builds live) live while any member build has no terminal/ directory
#                      under .audit/builds, or the group callback is claimed|running, or the run is ready_to_resume within resume_ttl;
#                      ready_to_resume past resume_ttl is reported `expired`, never stale and never released here (acquire.sh --expire).
# CENTRAL C2   for purpose commit_push, CPA_APPROVED_DIR must be set (checked first) and resume_ttl is read lazily from
#         $CPA_APPROVED_DIR/scripts/repo/commit_push.conf, only when a ready_to_resume holder exists; unset gives exit 20 helper_not_approved.
# Exits   0 (JSON or none); 2 usage; 20 helper_not_approved / conf unreadable. Writes nothing.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
[ $# -eq 1 ] && lo_safe_name "$1" || lo_die usage_error "usage: holder.sh <purpose>"
p=$1; lo_require_approved "$p"
s=$(lo_holder_status "$p"); rc=$?
[ "$rc" -eq "$RC_REFUSE" ] && exit "$RC_REFUSE"      # the lazy conf read failed loudly (message already printed)
[ "$rc" -eq 0 ] || { echo none; exit 0; }
case "$s" in
  live|expired) jq -c --arg s "$s" '. + {status:$s}' "$(lo_holder_file "$p")" ;;
  *) echo none ;;
esac
