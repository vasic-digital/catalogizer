#!/usr/bin/env bash
# check_no_build_writing_tracked.sh - refuse a commit window while a registered live operation writes a tracked path (T089;
# 11.4.121 no-commit-while-build-writes-tracked-artifacts; 11.4.84 quiescence; docs/16 13.2).
#
# Usage   check_no_build_writing_tracked.sh [--except-op-id <id>]...
# Refuses (exit 1, one line `blocked TAB <op_id> TAB <purpose> TAB <path> TAB <reason>` per finding, naming the op) while
#         - a registered LIVE build (purpose `build:*`) declares a write path that git tracks (a tracked file, or any tracked file under a declared directory); or
#         - ANY registered live op declares a write path under $EV or $AUD: another stream's evidence is committed only at a commit window.
#         Live = a non-terminal op whose owner still matches /proc (pid and start time), or a `registered` op that has not started.
#         A dead-owner row is not a writer (it is reported by the sweep as AM-P1), so a crashed op never wedges the window.
# Exits   0 no live writer; 1 blocked; 2 usage. Writes nothing.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
ex=()
while [ $# -gt 0 ]; do case "$1" in --except-op-id) ex+=("${2:-}"); shift 2 ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
rc=0
for f in "$LD"/ops/*.json; do
  [ -e "$f" ] || continue; j=$(cat "$f") || continue; id=$(jq -r .op_id <<<"$j")
  for e in "${ex[@]:-}"; do [ "$e" = "$id" ] && continue 2; done
  cls=$(lo_classify_op "$j" | head -1)
  case "$cls" in terminal|dead_owner) continue ;; esac
  purpose=$(jq -r .purpose_key <<<"$j")
  while IFS= read -r wpth; do
    [ -n "$wpth" ] || continue
    case "$wpth" in /*) abs=$wpth ;; *) abs=$ROOT/$wpth ;; esac
    abs=$(realpath -m -- "$abs")
    case "$abs/" in
      "$(realpath -m -- "$EV")"/*|"$(realpath -m -- "$AUD")"/*) printf 'blocked\t%s\t%s\t%s\tlive op declares a write path under $EV/$AUD (another stream'"'"'s evidence is committed only at a commit window)\n' "$id" "$purpose" "$wpth"; rc=1; continue ;;
    esac
    case "$purpose" in build:*)
      rel=${abs#"$ROOT"/}
      if [ -n "$(git -C "$ROOT" ls-files -- "$rel" 2>/dev/null | head -1)" ]; then
        printf 'blocked\t%s\t%s\t%s\tlive build writes a tracked path\n' "$id" "$purpose" "$wpth"; rc=1
      fi ;;
    esac
  done < <(jq -r '.write_paths[]?' <<<"$j")
done
exit $rc
