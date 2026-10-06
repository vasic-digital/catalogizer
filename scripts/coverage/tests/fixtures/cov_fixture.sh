#!/usr/bin/env bash
# fixture of the T199 bash line-trace harness: one covered function, one never-called function, one branch never taken.
set -u
covered_fn() {
  local a=1
  local b=2
  echo $((a+b))
}
uncovered_fn() {
  local c=3
  echo "never $c"
}
covered_fn
if [ "${1:-}" = never ]; then
  echo "branch not taken"
fi
echo done
