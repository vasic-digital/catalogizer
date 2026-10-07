#!/usr/bin/env bash
# mutscore.sh - the scoring decision of the register mutation runner (mutate_register_ops.sh), kept in one sourced function so a test can drive it (WF13 T3).
# mut_score <exit status of the suite run> <first line of that run starting with FAIL, or empty> prints exactly one of
#   survived  the suite passed against the mutant
#   killed    the suite failed AND its first failing line is a real check failure (a FAIL line that names no environmental refusal)
#   env       the suite failed for an environmental reason (disk headroom, image lock, memory budget, ...) or failed without any FAIL line (crash, signal,
#             timeout): the run says NOTHING about the mutant and is never counted as a kill
# The runner retries an env run (a transient host condition) and, when the last try is still env, reports ENV and fails the group.
ENV_RE="disk_below_headroom|disk_free_unreadable|lock_unreadable|memory_budget_unavailable|image_not_present_locally|cpu_budget_unavailable"
mut_score() {
  local rc=$1 line=${2:-}
  if [ "$rc" -eq 0 ]; then echo survived
  elif [ -z "$line" ]; then echo env
  elif printf '%s' "$line" | grep -Eq "$ENV_RE"; then echo env
  else echo killed; fi
}
