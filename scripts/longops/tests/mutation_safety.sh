#!/usr/bin/env bash
# mutation_safety.sh - STRUCTURAL containment of mutants for scripts/longops and scripts/anti-mess (WF11 F15; constitution 11.4.263, 11.4.201, 1.1).
# Sourced by mutate_registry.sh and mutate_sweep.sh, exercised by test_mutation_safety.sh. The earlier guard was a two-string grep on one file: it ran a
# mutant whose kill target was -1 or a process group (hypothetical mutants H1 to H3 of the WF11 review all evaluated to "run"). Three independent layers now stand
# between a mutant and a real signal, and the layers are themselves tested by running hypothetical bad mutants (test_mutation_safety.sh):
#   1. ms_scan <pristine-dir> <mutant-dir>   BEFORE a mutant runs: every regular file of the mutant tree is compared with its pristine twin; a line that carries a
#      signal or host-power token (kill, pkill, killall, killpg, os.kill, command/builtin/exec kill, /bin/kill, xargs kill, systemctl, loginctl, reboot, poweroff,
#      shutdown, halt, suspend, hibernate) and is NOT byte-identical to a pristine line aborts the run (exit 1, the offending line is printed). A mutant may remove or
#      replace a kill line by something without a token (mutant M04 does), it can never add or edit one.
#   2. ms_run <cmd...>   the mutant runs with BASH_ENV=<shim>: the shim (a) disables the `kill` BUILTIN (so `builtin kill` fails and `command kill` finds only PATH),
#      (b) defines `kill` as a guard function that refuses any target that is not a bare integer > 1 whose process group is also > 1 (pid 0, 1, -1, -pgid, junk,
#      job specs and kernel threads are all refused, one line BLOCKED is logged), (c) defines pkill/killall as refusals, and PATH begins with stub `kill`, `pkill`,
#      `killall`, `skill` executables that refuse. The real signal goes out only through MS_REAL_KILL (default /usr/bin/kill).
#   3. the runner refuses to start when it cannot build the containment (no external kill binary, root).
# Honest limits (11.4.6): an absolute path to a kill binary (/bin/kill) or a language-level kill (python os.kill) is caught by layer 1 only; a mutant that builds a
# command name at run time ("k=ki; ${k}ll") is caught by layer 2 only for the shell forms (the function and the PATH stubs), not for an exotic interpreter.
# Layer 2 is a function, not a kernel boundary: a rootless container with a private PID namespace would be (11.4.161/11.4.173; `unshare` is refused on this host).

MS_TOKEN_RE='(^|[^A-Za-z0-9_.-])(kill|killall|pkill|skill|killpg|os\.kill|signal\.|xargs[[:space:]]+kill|/bin/kill|/usr/bin/kill|systemctl|loginctl|reboot|poweroff|shutdown|halt|suspend|hibernate)([^A-Za-z0-9_-]|$)'

# ms_scan <pristine-dir> <mutant-dir>: 0 safe; 1 a new/edited signal line (printed) in a regular file of the mutant tree.
ms_scan() {
  local pr=$1 mu=$2 f rel line bad=0
  [ -d "$pr" ] && [ -d "$mu" ] || { echo "ms_scan: usage: pristine and mutant directories" >&2; return 2; }
  while IFS= read -r -d '' f; do
    rel=${f#"$mu"/}
    while IFS= read -r line || [ -n "$line" ]; do
      case "${line#"${line%%[![:space:]]*}"}" in '#'*) continue ;; esac     # a comment line cannot execute
      [[ "$line" =~ $MS_TOKEN_RE ]] || continue
      if [ -f "$pr/$rel" ] && grep -qxF -- "$line" "$pr/$rel"; then continue; fi
      echo "SAFETY: $rel carries a signal/host-power line that is not byte-identical to the pristine tree: $line"; bad=1
    done <"$f"
  done < <(find "$mu" -type f -print0 2>/dev/null)
  return $bad
}

# ms_make_shim <dir>: writes <dir>/bashenv.sh and <dir>/bin/{kill,pkill,killall,skill}; prints nothing. Sets MS_SHIM, MS_BIN, MS_LOG.
ms_make_shim() {
  local d=$1; MS_BIN=$d/bin; MS_SHIM=$d/bashenv.sh; MS_LOG=$d/blocked.log
  mkdir -p "$MS_BIN"; : >"$MS_LOG"
  local n; for n in kill pkill killall skill; do
    printf '#!/bin/sh\necho "BLOCKED stub %s $*" >>"%s"\nexit 1\n' "$n" "$MS_LOG" >"$MS_BIN/$n"; chmod +x "$MS_BIN/$n"
  done
  cat >"$MS_SHIM" <<'SHIM'
# bashenv.sh - sourced by every non-interactive bash of a contained mutant run (BASH_ENV). See mutation_safety.sh.
enable -n kill 2>/dev/null
ms_blocked() { printf '%s BLOCKED %s\n' "$(date -u +%H:%M:%S)" "$*" >>"${MS_LOG:-/dev/null}"; return 1; }
ms_kill_guard() {
  local a sawdd=0 first=1 skip=0 tgt=0 t pg
  for a in "$@"; do
    if [ "$skip" = 1 ]; then skip=0; first=0; continue; fi
    if [ "$sawdd" = 0 ]; then
      case "$a" in
        --) sawdd=1; first=0; continue ;;
        -s|-n|--signal) skip=1; continue ;;
        -l|-L|--list|--table|-h|--help|-V|--version) first=0; continue ;;
        -[A-Za-z]*) first=0; continue ;;
        -[0-9]*) if [ "$first" = 1 ]; then first=0; continue; fi ;;
      esac
    fi
    first=0; tgt=$((tgt+1)); t=$a
    [[ "$t" =~ ^[0-9]{1,15}$ ]] && [ "$t" -gt 1 ] || { ms_blocked "kill $* (target '$t' is not an integer > 1)"; return 1; }
    pg=$(sed 's/^.*) //' "/proc/$t/stat" 2>/dev/null | cut -d' ' -f3)
    [[ "$pg" =~ ^[0-9]+$ ]] && [ "$pg" -gt 1 ] || { ms_blocked "kill $* (process group of $t is '$pg')"; return 1; }
  done
  [ "$tgt" -gt 0 ] || { ms_blocked "kill $* (no target)"; return 1; }
  "${MS_REAL_KILL:-/usr/bin/kill}" "$@"
}
kill() { ms_kill_guard "$@"; }
pkill() { ms_blocked "pkill $*"; }
killall() { ms_blocked "killall $*"; }
SHIM
}

# ms_prepare <scratch-dir>: builds the containment; returns 2 (and says why) when it cannot.
ms_prepare() {
  [ "$(id -u)" != 0 ] || { echo "ms_prepare: refusing to contain mutants as root" >&2; return 2; }
  [ -x "${MS_REAL_KILL:-/usr/bin/kill}" ] || { echo "ms_prepare: no external kill binary (${MS_REAL_KILL:-/usr/bin/kill}); containment cannot be built, mutants are not run" >&2; return 2; }
  ms_make_shim "$1"
}

# ms_run <cmd...>: runs a command inside the containment (layer 2).
ms_run() { BASH_ENV=$MS_SHIM PATH=$MS_BIN:$PATH MS_LOG=$MS_LOG MS_REAL_KILL=${MS_REAL_KILL:-/usr/bin/kill} "$@"; }
