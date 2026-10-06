#!/usr/bin/env bash
# classify.sh - read-only liveness classification of registered operations (11.4.232 C; docs/16 13.2 rule C).
#
# Usage   classify.sh [--op-id <id>]
# Output  `<op_id> TAB <class> TAB <evidence>` per op. Classes: terminal | advancing | hung | dead_owner.
#         hung      the owner lives but the progress offset (or heartbeat) has not advanced for longer than the op's no_progress_s.
#         dead_owner the recorded pid/start time no longer names a running process (resolved from /proc; a recycled pid is dead_owner).
#         kill -0 is only a pre-filter: an alive owner with a flat offset is hung, never advancing.
# Exits   0
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
only=""; [ "${1:-}" = --op-id ] && only=${2:-}
for f in "$LD"/ops/*.json; do
  [ -e "$f" ] || continue; j=$(cat "$f") || continue; id=$(jq -r .op_id <<<"$j")
  [ -z "$only" ] || [ "$id" = "$only" ] || continue
  r=$(lo_classify_op "$j"); printf '%s\t%s\t%s\n' "$id" "$(sed -n 1p <<<"$r")" "$(sed -n 2p <<<"$r")"
done
exit 0
