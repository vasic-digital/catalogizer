#!/usr/bin/env bash
# check_no_build_writing_tracked.sh - refuse a commit window while a registered live operation writes a tracked path (T089;
# 11.4.121 no-commit-while-build-writes-tracked-artifacts; 11.4.84 quiescence; docs/16 13.2).
#
# Usage   check_no_build_writing_tracked.sh [--except-op-id <id>]...
# Refuses (exit 1, one line `blocked TAB <op_id> TAB <purpose> TAB <path> TAB <reason>` per finding, naming the op) while
#         - ANY registered live op (whatever its purpose: a `build:*` op, a `container:*` runner op, a stream of tests ...) declares a write path that git tracks (a tracked file, or any tracked file under a
#           declared directory) (LO-E2: the rule no longer depends on the purpose prefix; the producers that write a tracked path declare it: the runner declares its `--rw` mount); or
#         - ANY registered live op declares a write path under $EV or $AUD: another stream's evidence is committed only at a commit window.
#         Live = a non-terminal op: its owner still matches /proc (pid, start time, boot id), OR the owner is gone and the row is not yet REAPED. A `dead_owner` row keeps blocking until `reap.sh` has proven its
#         workload gone and recorded it `reaped` (LO-D3: owner gone is not workload gone; the runner's guard loop may still be stopping a container that writes).
#         An op record that fails the record shape (empty, unparsable, mistyped, an unknown state) blocks too (its write paths are unknown): a corrupt record is never read as "no writer", a `write_paths`
#         that is not an array of strings or a `purpose_key` that is not a string included (LO-A2).
# Exits   0 no live writer; 1 blocked; 2 usage. Writes nothing.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
ex=()
while [ $# -gt 0 ]; do case "$1" in --except-op-id) lo_need "$@"; ex+=("$2"); shift 2 ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
rc=0
EVR=$(realpath -m -- "$EV"); AUDR=$(realpath -m -- "$AUD")
while IFS=$'\t' read -r kind file state id purpose j; do
  if [ "$kind" = bad ]; then printf 'blocked\t%s\t-\t-\tthe op record fails the record shape (empty, unparsable, mistyped or an unknown state): its write paths are unknown (fail closed, WF11 class 2)\n' "$file"; rc=1; continue; fi
  for e in "${ex[@]:-}"; do [ "$e" = "$id" ] && continue 2; done
  case "$state" in registered|running) cls=$(lo_classify_op "$j" | head -1) ;; *) continue ;; esac
  if [ "$cls" = unreadable ]; then printf 'blocked\t%s\t-\t-\tthe op record is non-numeric: its write paths are unknown (fail closed, WF11 class 2)\n' "$file"; rc=1; continue; fi
  case "$cls" in terminal) continue ;; esac
  while IFS= read -r wpth; do
    [ -n "$wpth" ] || continue
    case "$wpth" in /*) abs=$wpth ;; *) abs=$ROOT/$wpth ;; esac
    abs=$(realpath -m -- "$abs")
    case "$abs/" in
      "$EVR"/*|"$AUDR"/*) printf 'blocked\t%s\t%s\t%s\tlive op declares a write path under $EV/$AUD (another stream'"'"'s evidence is committed only at a commit window)\n' "$id" "$purpose" "$wpth"; rc=1; continue ;;
    esac
    rel=${abs#"$ROOT"/}
    if [ -n "$(git -C "$ROOT" ls-files -- "$rel" 2>/dev/null | head -1)" ]; then
      printf 'blocked\t%s\t%s\t%s\tlive op writes a tracked path\n' "$id" "$purpose" "$wpth"; rc=1
    fi
  done < <(jq -r '.write_paths[]' <<<"$j")
done < <(lo_ops_snapshot)
exit $rc
