#!/usr/bin/env bash
# classify.sh - read-only liveness classification of registered operations (11.4.232 C; docs/16 13.2 rule C).
#
# Usage   classify.sh [--op-id <id>]
# Output  `<op_id> TAB <class> TAB <evidence>` per op. Classes: terminal | advancing | hung | dead_owner | unreadable (an empty, unparsable or non-numeric
#         record: the first column is the file name; never read as dead_owner or advancing).
#         hung      the owner lives but the progress offset (or heartbeat) has not advanced for longer than the op's no_progress_s.
#         dead_owner the recorded pid/start time no longer names a running process (resolved from /proc; a recycled pid is dead_owner).
#         kill -0 is only a pre-filter: an alive owner with a flat offset is hung, never advancing.
# Exits   0; 2 usage (unknown argument); 4 --op-id names no record (unknown_op: never an empty successful answer)
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
only=""; have_only=0
while [ $# -gt 0 ]; do case "$1" in --op-id) [ $# -ge 2 ] || lo_die usage_error "--op-id needs a value"; only=$2; have_only=1; shift 2 ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
if [ "$have_only" = 1 ]; then lo_safe_name "$only" || lo_die usage_error "unsafe op id"; [ -e "$(lo_op_file "$only")" ] || lo_die unknown_op "$only" "$RC_CAS"; fi
for f in "$LD"/ops/*.json; do
  [ -e "$f" ] || continue
  if [ "$have_only" = 1 ] && [ "$f" != "$(lo_op_file "$only")" ]; then continue; fi
  j=$(cat "$f") || { printf '%s\tunreadable\tcannot read the record\n' "$f"; continue; }
  r=$(lo_classify_op "$j"); c=$(sed -n 1p <<<"$r")
  if [ "$c" = unreadable ]; then id=$f; else id=$(jq -r .op_id <<<"$j"); fi
  printf '%s\t%s\t%s\n' "$id" "$c" "$(sed -n 2p <<<"$r")"
done
exit 0
