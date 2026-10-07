#!/usr/bin/env bash
# run_callback.sh - T005b: the callback runner. Runs the registered callback of a TERMINAL build (or build group) exactly once in effect, from durable state.
#   run_callback.sh <builds_root> <build_id> [--retry]       a build directory under <builds_root>, or `group/<group_id>`
# The callback id comes from callback.json and must be a row of the closed registry (scripts/build/callbacks.tsv; the file the runner reads is
# RUNCB_CALLBACKS_TSV, the hub's own export of it, else DISPATCH_CALLBACKS_TSV, else the table next to this script). An id the table lacks is refused at
# callback time: terminal/script.state = failed, terminal/script.reason = callback_not_registered, the script never runs.
# Rows (id TAB script TAB effect-key-prefix TAB long-op purpose TAB no-progress budget in seconds):
#   script `-`   the keyed-effect callback of event_core.sh (run_callback): used when a callback is found claimed or running after a restart.
#   a script     a path relative to RUNCB_SCRIPT_ROOT (default: the checkout), run as `bash <script>` with the environment CB_KIND (the terminal kind),
#                CB_EXIT_CLASS, CB_REASON (the reason of a blocked-unavailable or infra_failed terminal, else empty), CB_BUILD_DIR, CB_EFFECT_KEY (the effect key
#                the script must write under <dir>/effects/ with an atomic rename, treating an existing effect as done: the runner re-runs a callback found
#                running, so every script must be idempotent through that key), CB_ARGS_FILE (the callback arguments, JSON, as data). The script runs under
#                `timeout <budget>` (the row's budget). Exit 0 and the effect present: done. Exit 0 and no effect: failed effect_not_applied. A non-zero exit:
#                failed with reason exit_<code> (timeout: exit_124); never retried silently (--retry re-runs a failed one by explicit request).
# Durable state, each file written temp-then-rename inside terminal/: script.state (running | done | failed), script.reason, script.runner ("pid start-time"
# of the runner while it works: a runner whose pid and start time no longer match /proc left a callback found `running`, which the next call re-runs),
# callback.runner_sha256 (the value of CPA_EXEC_SHA256 when the runner was started with it; nothing is written otherwise: CENTRAL C10).
# Why a separate script state: event_core.sh consume runs its own keyed-effect callback inline for a consumed `completed` event and records callback.state
# done; the script phase here is a second, separately recorded step (owed request O2: a callback-executor hook in event_core.sh so that one state serves).
# Exit: 0 done or nothing to do, 1 failed, 20 refused (REFUSED reason=...).
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$here/../.." && pwd)
refuse() { printf 'REFUSED reason=%s %s\n' "$1" "${2:-}" >&2; exit 20; }
[ $# -ge 2 ] || refuse usage "run_callback.sh <builds_root> <build_id|group/<id>> [--retry]"
BR=$1; ID=$2; RETRY=0; [ "${3:-}" = --retry ] && RETRY=1
[[ $ID =~ ^(b-[A-Za-z0-9_-]{1,126}|group/[A-Za-z0-9_-]{1,100})$ ]] || refuse event_malformed "build id"
D="$BR/$ID"
TSV="${RUNCB_CALLBACKS_TSV:-${DISPATCH_CALLBACKS_TSV:-$here/callbacks.tsv}}"
SROOT="${RUNCB_SCRIPT_ROOT:-$ROOT}"
[ -d "$D/terminal" ] || refuse no_terminal "$ID"
[ -s "$D/callback.json" ] || refuse state_corrupt "callback.json"
command -v jq >/dev/null 2>&1 || refuse dependency_missing jq
now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
starttime() { awk '{print $22}' "/proc/$1/stat" 2>/dev/null; }
tw() { # file content : temp-then-rename inside terminal/ (fsync of the file is the python helper of the core's family: here a plain sync of the temp file)
  local f=$1 t="$1.tmp-$$"; printf '%s\n' "$2" > "$t" && sync "$t" 2>/dev/null; mv -f "$t" "$f"; }
setstate() { tw "$D/terminal/script.state" "$1"; if [ -n "${2:-}" ]; then tw "$D/terminal/script.reason" "$2"; else rm -f "$D/terminal/script.reason"; fi; }
[ -z "${CPA_EXEC_SHA256:-}" ] || tw "$D/terminal/callback.runner_sha256" "$CPA_EXEC_SHA256"

cb=$(jq -r '.callback_id // empty' "$D/callback.json"); skey=$(jq -r '.script_effect_key // .effect_key // empty' "$D/callback.json")
[ -n "$cb" ] || refuse state_corrupt "callback_id"
row=""; [ -r "$TSV" ] && row=$(awk -F'\t' -v i="$cb" '!/^#/ && $1 == i { print; exit }' "$TSV")
if [ -z "$row" ]; then
  # an id the approved table lacks: failed, never run (the build's verdict is untouched)
  [ "$(cat "$D/terminal/script.state" 2>/dev/null)" = done ] || setstate failed callback_not_registered
  echo "callback failed: callback_not_registered"; exit 1
fi
IFS=$'\t' read -r _id script _pfx _purpose budget <<<"$row"
exec 8>>"$D/.scriptlock"
flock -w 120 8 || refuse lock_unavailable "script"
if [ "$script" = - ]; then
  # the keyed-effect callback of the core, resumed from its durable state; nothing separate to record
  case "$(cat "$D/terminal/callback.state" 2>/dev/null)" in claimed|running) bash "$here/event_core.sh" resume-callback "$BR" "$ID" >/dev/null 2>&1;; esac
  exit 0
fi
st=$(cat "$D/terminal/script.state" 2>/dev/null)
case $st in
  done) exit 0;;
  failed) [ $RETRY = 1 ] || exit 1;;
  running)
    if [ -s "$D/terminal/script.runner" ]; then
      read -r rp rs < "$D/terminal/script.runner"
      [ "${rp:-0}" -gt 1 ] 2>/dev/null && [ "$(starttime "$rp")" = "${rs:-x}" ] && [ "$rp" != "$$" ] && { echo "callback is running (runner $rp)"; exit 0; }
    fi;;
esac
# the script path stays inside the script root: no absolute path, no parent reference, no symlink out
case $script in /*|*..*) setstate failed script_path_refused; exit 1;; esac
sp="$SROOT/$script"
[ -f "$sp" ] || { setstate failed script_missing; exit 1; }
isint() { [[ ${1:-} =~ ^[1-9][0-9]{0,6}$ ]]; }; isint "$budget" || budget=60
kind=$(jq -r .kind "$D/terminal/state.json"); ec=$(jq -r '.exit_class // ""' "$D/terminal/state.json")
reason=""; case $kind:$ec in blocked-unavailable:*|*:infra_failed|infra_failed:*) reason=$(jq -r '.digest // ""' "$D/terminal/state.json");; esac
printf '%s %s\n' "$$" "$(starttime $$)" > "$D/terminal/script.runner.tmp-$$" && mv -f "$D/terminal/script.runner.tmp-$$" "$D/terminal/script.runner"
setstate running
# run the script as a child (timeout signals that one child only); a runner killed meanwhile leaves `running`, which the next call re-runs
if [ -e "$D/effects/$skey" ]; then rc=0
else
  CB_KIND=$kind CB_EXIT_CLASS=$ec CB_REASON=$reason CB_BUILD_DIR=$D CB_EFFECT_KEY=$skey CB_ARGS_FILE="$D/callback.json" timeout "${budget}s" bash "$sp" 8>&- < /dev/null >> "$D/script.log" 2>&1; rc=$?
fi
if [ $rc -eq 0 ]; then
  if [ -e "$D/effects/$skey" ]; then setstate done; echo "callback done"; exit 0; fi
  setstate failed effect_not_applied; echo "callback failed: effect_not_applied"; exit 1
fi
setstate failed "exit_$rc"; echo "callback failed: exit_$rc"; exit 1
