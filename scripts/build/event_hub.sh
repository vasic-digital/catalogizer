#!/usr/bin/env bash
# event_hub.sh - T005b: the event hub, ONE process per driver checkout.
#   event_hub.sh run      foreground: take the single-hub lock, write .audit/builds/hub.pid and hub.json, reconcile, then block on events
#   event_hub.sh start    start a detached hub unless one lives (idempotent; what dispatch.sh submit and resume call)
#   event_hub.sh status   print the live hub record, exit 0, or `none` and exit 1
# What it does: the pumps of dispatch.sh own the event streams of their builds (one blocking reader per build); the hub is the supervisor and reconciler above
# them, driven only by events (inotify on the builds root, build directories, terminal/ and group/ directories, and a pidfd per live pump: lib/evwait.py watch;
# no sleep loop, no periodic probe, nothing starts while idle): it restarts the pump of an open build whose pump died (resume from the last acknowledged seq,
# never a resubmission), starts a queued build when a slot frees, runs every callback found claimed or running (and every script callback not yet run) through
# run_callback.sh from its durable state, evaluates build groups, and stamps CPA_EXEC_SHA256 as hub_sha256 into terminal records and consumed marks.
# Single instance: an flock on builds/hub.lock held for the hub's life (children never inherit it), a registry claim `build-hub:<checkout>` (the holder is named
# through scripts/longops/holder.sh in the refusal `hub_already_running`), hub.pid ("pid start-time", temp-then-rename, removed at exit) and hub.json
# {pid,start_time} (the record the anti-mess sweep reads).
# Callbacks table: the hub copies callbacks.tsv ONCE at its start into builds/hub.callbacks.tsv (its own export) and runs callbacks from that copy only:
# a row added later is honoured after the next hub start (CENTRAL C10). Owed: the start through `cpa-host --exec-approved` (T046/T047 not present), the
# systemd --user unit (T005b), one SSH connection per host (the pumps hold one each).
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$here/../.." && pwd)
BUILDS="${DISPATCH_BUILDS_ROOT:-$ROOT/.audit/builds}"
DSP="${HUB_DISPATCH:-$here/dispatch.sh}"; RUNCB="${HUB_RUNCB:-$here/run_callback.sh}"
EVW="${DISPATCH_EVWAIT:-$here/lib/evwait.py}"
CALLBACKS="${DISPATCH_CALLBACKS_TSV:-$here/callbacks.tsv}"
LO="${LONGOPS_SCRIPTS:-$ROOT/scripts/longops}"
CHECKOUT_ID=$(printf '%s' "$ROOT" | sha256sum | cut -d' ' -f1)
STATE="${DISPATCH_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/$CHECKOUT_ID}"
PURPOSE="build-hub:${CHECKOUT_ID:0:16}"
. "$here/lib/longops_env.sh"; bind_longops_env "$BUILDS" "$ROOT"
refuse() { printf 'REFUSED reason=%s %s\n' "$1" "${2:-}" >&2; exit 20; }
starttime() { awk '{print $22}' "/proc/$1/stat" 2>/dev/null; }
tw() { local f=$1 t="$1.tmp-$$"; printf '%s\n' "$2" > "$t" && mv -f "$t" "$f"; }
hub_alive() { local p s; [ -s "$BUILDS/hub.pid" ] || return 1; read -r p s < "$BUILDS/hub.pid"; [ "${p:-0}" -gt 1 ] 2>/dev/null && [ "$(starttime "$p")" = "${s:-x}" ] && tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q 'event_hub.sh'; }
x() { "$@" 9>&-; }          # every child runs without the hub's lock descriptor: a pump started by the hub must never keep the single-hub lock alive

stamp() { # builddir : hub_sha256 into terminal/state.json and into every consumed mark that lacks it (atomic replace, never in place)
  local d=$1 f t m v=${CPA_EXEC_SHA256:-}
  [[ $v =~ ^[0-9a-f]{64}$ ]] || return 0
  f="$d/terminal/state.json"
  if [ -s "$f" ] && [ "$(jq -r '.hub_sha256 // ""' "$f" 2>/dev/null)" != "$v" ]; then
    t="$f.tmp-$$"; jq --arg v "$v" '.hub_sha256 = $v' "$f" > "$t" 2>/dev/null && [ -s "$t" ] && mv -f "$t" "$f" || rm -f "$t"; fi
  for m in "$d"/consumed/[1-9]*; do [ -f "$m" ] || continue
    grep -q "hub_sha256=" "$m" 2>/dev/null && continue
    t="$m.tmp-$$"; printf '%s hub_sha256=%s\n' "$(head -c 4096 "$m" | tr -d '\n')" "$v" > "$t" && mv -f "$t" "$m" || rm -f "$t"; done; }
pump_up() { local p s; [ -s "$1/pump.pid" ] || return 1; read -r p s < "$1/pump.pid"; [ "${p:-0}" -gt 1 ] 2>/dev/null && [ "$(starttime "$p")" = "${s:-x}" ] && tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q '_pump'; }
runner_up() { local p s; [ -s "$1/terminal/script.runner" ] || return 1; read -r p s < "$1/terminal/script.runner"; [ "${p:-0}" -gt 1 ] 2>/dev/null && [ "$(starttime "$p")" = "${s:-x}" ]; }
spawn_runner() { # builddir-or-groupdir : the callback runner, detached, from the hub's own export of the callbacks table
  x env RUNCB_CALLBACKS_TSV="$BUILDS/hub.callbacks.tsv" setsid -f bash "$RUNCB" "$BUILDS" "${1#$BUILDS/}" >> "$1/script.log" 2>&1 < /dev/null; }
RESTARTS=0
reconcile() {
  local d id cbs ss resumed=0
  for d in "$BUILDS"/b-*/; do d=${d%/}; id=${d##*/}; [ -f "$d/submit.json" ] || continue
    if [ -d "$d/terminal" ]; then
      stamp "$d"
      cbs=$(cat "$d/terminal/callback.state" 2>/dev/null); ss=$(cat "$d/terminal/script.state" 2>/dev/null)
      if grep -q '"script_effect_key"' "$d/callback.json" 2>/dev/null; then       # a cheap test: this runs for every terminal build at every event
        case $ss in done|failed) ;; running) runner_up "$d" || spawn_runner "$d";; *) spawn_runner "$d";; esac
      fi
      case $cbs in claimed|running) x env RUNCB_CALLBACKS_TSV="$BUILDS/hub.callbacks.tsv" bash "$DSP" resume "$id" >> "$d/pump.log" 2>&1;; esac
    elif [ -e "$d/queued" ]; then :
    elif pump_up "$d"; then :
    else
      x bash "$DSP" resume "$id" >> "$d/pump.log" 2>&1; resumed=1
    fi
  done
  # terminal build groups: their script callback (the core's keyed-effect callback is resumed by group-sweep below)
  local g
  for g in "$BUILDS"/group/*/; do g=${g%/}; [ -d "$g/terminal" ] || continue
    stamp "$g"
    if grep -q '"script_effect_key"' "$g/callback.json" 2>/dev/null; then
      ss=$(cat "$g/terminal/script.state" 2>/dev/null); case $ss in done|failed) ;; running) runner_up "$g" || spawn_runner "$g";; *) spawn_runner "$g";; esac
    fi
  done
  if [ $resumed = 1 ]; then RESTARTS=$((RESTARTS+1)); else RESTARTS=0; fi
  x bash "$DSP" drain >/dev/null 2>&1
  x bash "$DSP" group-sweep >/dev/null 2>&1
}
cmd_run() {
  command -v jq >/dev/null 2>&1 && command -v flock >/dev/null 2>&1 || refuse dependency_missing "jq flock"
  mkdir -p "$BUILDS" || refuse state_write_failed "$BUILDS"
  exec 9>>"$BUILDS/hub.lock"
  if ! flock -w 3 9; then      # a hub that is exiting releases the lock within moments; a live one holds it
    local h; h=$(cat "$BUILDS/hub.pid" 2>/dev/null | tr '\n' ' ')
    local rh; rh=$(bash "$LO/holder.sh" "$PURPOSE" 2>/dev/null | jq -r 'select(type=="object") | "pid=\(.pid)"' 2>/dev/null)
    refuse hub_already_running "holder=${rh:-${h:-unknown}} (hub.pid: ${h:-absent})"
  fi
  local me=$$ st; st=$(starttime $$)
  # the registry claim (a dead holder's stale claim is reaped, never taken over silently)
  local rc
  bash "$LO/register.sh" --purpose "$PURPOSE" --owner event_hub --op-id "hub-${CHECKOUT_ID:0:12}-$me" --pid $$ --no-progress-s 0 >/dev/null 2>&1; rc=$?
  if [ $rc -eq 4 ]; then bash "$LO/reap.sh" --purpose "$PURPOSE" >/dev/null 2>&1; bash "$LO/register.sh" --purpose "$PURPOSE" --owner event_hub --op-id "hub-${CHECKOUT_ID:0:12}-$me" --pid $$ --no-progress-s 0 >/dev/null 2>&1; rc=$?; fi
  # the kernel keyring entry is only a cache of the secret file: refilled from it at every start (after a reboot or logout it is empty); a lost or altered file is
  # logged here and every open build then ends driver_secret_lost through its pump (never silently resubmitted)
  bash "$here/lib/keyring.sh" refill "$STATE" "$ROOT" >> "$BUILDS/hub.log" 2>&1 9>&-
  tw "$BUILDS/hub.pid" "$me $st"; tw "$BUILDS/hub.json" "$(jq -nc --argjson p "$me" --arg s "$st" '{pid:$p,start_time:$s}')"
  trap 'hub_exit' EXIT; trap 'exit 0' TERM INT HUP
  [ -r "$CALLBACKS" ] && cp -f "$CALLBACKS" "$BUILDS/hub.callbacks.tsv.tmp-$$" && mv -f "$BUILDS/hub.callbacks.tsv.tmp-$$" "$BUILDS/hub.callbacks.tsv"
  [ -f "$BUILDS/hub.callbacks.tsv" ] || : > "$BUILDS/hub.callbacks.tsv"
  # the event source stays armed for the hub's whole life (an event that arrives while a reconcile runs is queued in the pipe, never lost)
  exec {efd}< <(python3 -I "$EVW" serve "$BUILDS" 9>&-)
  WP=$!
  reconcile
  local exp rc
  while :; do
    [ -d "$BUILDS" ] || exit 0         # the builds root is gone (a removed fixture): nothing left to supervise
    # the only timer: the budget of the earliest unsealed build group (a deadline, not a poll); a build whose pump keeps dying at once backs off the same way
    exp=$(x bash "$DSP" group-next-expiry 2>/dev/null)
    [ "$RESTARTS" -le 20 ] || { exp=10; RESTARTS=0; }
    if [ -n "$exp" ]; then read -r -t "$exp" -u "$efd" _ev; rc=$?; else read -r -u "$efd" _ev; rc=$?; fi
    [ $rc -eq 0 ] || [ $rc -gt 128 ] || exit 0       # EOF: the event source ended
    reconcile
  done
}
WP=0
hub_exit() {
  local p s
  [ "${WP:-0}" -gt 1 ] 2>/dev/null && kill "$WP" 2>/dev/null
  if [ -s "$BUILDS/hub.pid" ]; then read -r p s < "$BUILDS/hub.pid"; if [ "$p" = "$$" ]; then rm -f "$BUILDS/hub.pid" "$BUILDS/hub.json"; fi; fi
  bash "$LO/release.sh" --op-id "hub-${CHECKOUT_ID:0:12}-$$" --state complete --verdict stopped >/dev/null 2>&1
}
cmd_start() {
  mkdir -p "$BUILDS" 2>/dev/null
  if hub_alive; then echo "hub running"; return 0; fi
  rm -f "$BUILDS/hub.pid" "$BUILDS/hub.json"          # a stale record of a dead hub
  setsid -f bash "$0" run >> "$BUILDS/hub.log" 2>&1 < /dev/null 9>&-
  python3 -I "$EVW" waitfile "$BUILDS/hub.pid" 10 || { echo "hub did not start" >&2; return 1; }
  echo "hub started"
}
case "${1:-}" in
  run) cmd_run;;
  start) cmd_start;;
  status) if hub_alive; then cat "$BUILDS/hub.pid"; else echo none; exit 1; fi;;
  *) echo "usage: $0 run|start|status" >&2; exit 2;;
esac
