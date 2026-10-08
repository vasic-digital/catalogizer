#!/usr/bin/env bash
# mutation_safety.sh - STRUCTURAL containment of mutants for scripts/longops and scripts/anti-mess (WF11 F15; constitution 11.4.263, 11.4.201, 1.1).
# Sourced by mutate_registry.sh and mutate_sweep.sh, exercised by test_mutation_safety.sh. The earlier guard was a two-string grep on one file: it ran a
# mutant whose kill target was -1 or a process group (hypothetical mutants H1 to H3 of the WF11 review all evaluated to "run"). Three independent layers now stand
# between a mutant and a real signal, and the layers are themselves tested by running hypothetical bad mutants (test_mutation_safety.sh):
#   1. ms_scan <pristine-dir> <mutant-dir>   BEFORE a mutant runs: every regular file of the mutant tree is compared with its pristine twin; a line that carries a
#      signal or host-power token (kill, pkill, killall, killpg, os.kill, command/builtin/exec kill, /bin/kill, xargs kill, systemctl, loginctl, reboot, poweroff,
#      shutdown, halt, suspend, hibernate) and is NOT byte-identical to a pristine line aborts the run (exit 1, the offending line is printed). A mutant may remove or
#      replace a kill line by something without a token (mutant M04 does); it can add or edit none, with ONE honest exception (round 5, CONS-11): a BYTE-IDENTICAL COPY of a pristine signal
#      line is admitted ANYWHERE in the tree, so layer 1 does not bind a pristine line to its function (mutant C03 moved the `podman stop` line into another step and ran; it is safe only
#      because layer 2 refuses that call). Round 5 adds a second scan class: a NEW (non-pristine) line that carries a `/bin` or `/sbin` path fragment, or a word split by an empty quote
#      pair (`"$b/k""ill"`), is refused too, because it can spell a signal binary without writing its name (WF17 F7).
#   2. ms_run <cmd...>   the mutant runs with BASH_ENV=<shim>: the shim (a) disables the `kill` BUILTIN (so `builtin kill` fails and `command kill` finds only PATH),
#      (b) defines `kill` as a guard function that refuses any target that is not a bare integer > 1 whose process group is also > 1 (pid 0, 1, -1, -pgid, junk,
#      job specs and kernel threads are all refused, one line BLOCKED is logged), (c) defines pkill/killall as refusals, and PATH begins with stub `kill`, `pkill`,
#      `killall`, `skill` executables that refuse. The real signal goes out only through MS_REAL_KILL (default /usr/bin/kill).
#   3. the runner refuses to start when it cannot build the containment (no external kill binary, root).
# Honest limits (11.4.6; WF14 R2-D1 corrected the earlier "structurally impossible" claim): this is CONTAINMENT IN LAYERS, not a kernel boundary. Layer 1 is textual: it catches the
# shell and interpreter spellings listed above, including python os.kill/os.killpg/psutil, a glob-built absolute path (/usr/bin/k?ll) and any line that names or redefines the
# containment itself (ms_*, MS_*, BASH_ENV, enable, unset -f); it does NOT catch a command name computed at run time inside an interpreter (getattr(os, "ki"+"ll")) nor a language that is not
# listed. Layer 2 is a shell function, a set of PATH stubs (kill, pkill, killall, skill and, since WF14, podman) and read-only function definitions: it covers the shell forms that resolve a
# command NAME (function, PATH stub), NOT every shell form: an absolute path to a binary bypasses PATH and is caught by layer 1 only, and NOTHING that does not pass through bash is covered (a
# python process never sees the shim). The scan of a computed path or a quote-split word is a heuristic over NEW lines, not a proof (round 5 honest limit). A STRUCTURAL boundary
# would be a rootless container with a private PID namespace (11.4.161/11.4.173; `unshare --user --pid` is refused on this host): until it exists, no mutant that this scan cannot read is safe,
# and the mutants of the two runners are reviewed (11.4.142) before they are added.

# MS_TOKEN_RE: a signal, host-power, containment-tampering or container side-effect token. The left boundary admits `.` (WF14 RB5: `os.killpg(` and `.kill(` were invisible while `.` was excluded).
MS_TOKEN_RE='(^|[^A-Za-z0-9_-])(kill|killall|pkill|skill|killpg|tgkill|sigqueue|pidfd_send_signal|send_signal|terminate|psutil|os\.kill|signal\.|xargs[[:space:]]+kill|/bin/kill|/usr/bin/kill|systemctl|loginctl|reboot|poweroff|shutdown|halt|suspend|hibernate|enable|BASH_ENV|MS_[A-Z_]+|ms_[a-z_]+|unset[[:space:]]+-f|(/usr)?/s?bin/podman|podman"?[[:space:]]+(stop|kill|rm|rmi|restart|pause|unpause|prune|system|volume|network|pod|container|exec|run|create|start)|PODMAN"?[[:space:]]+(stop|kill|rm|rmi|restart|pause|unpause|prune|system|volume|network|pod|container|exec|run|create|start))([^A-Za-z0-9_-]|$)'
# MS_GLOB_RE: an absolute path into a bin directory that contains a glob character: it can name a kill binary without spelling it (WF14 RB7: /usr/bin/k?ll).
MS_GLOB_RE='/(usr/)?s?bin/[^[:space:]]*[?*[]'

# MS_PATH_RE: a NEW line naming a bin directory, MS_SPLIT_RE: a NEW line with a word split by an empty quote pair (`"$b/k""ill"`, `k''ill`). Either can build a signal binary name at run time (WF17 F7).
MS_PATH_RE='(^|[^A-Za-z0-9_])/(usr/)?(local/)?s?bin([/"'"'"'[:space:]]|$)'
MS_SPLIT_RE='[A-Za-z0-9_/.$}-](""|'"''"')[A-Za-z0-9_/.$"-]'

# ms_scan <pristine-dir> <mutant-dir>: 0 safe; 1 a new/edited signal line (printed) in a regular file of the mutant tree.
ms_scan() {
  local pr=$1 mu=$2 f rel line bad=0
  [ -d "$pr" ] && [ -d "$mu" ] || { echo "ms_scan: usage: pristine and mutant directories" >&2; return 2; }
  while IFS= read -r -d '' f; do
    rel=${f#"$mu"/}
    while IFS= read -r line || [ -n "$line" ]; do
      case "${line#"${line%%[![:space:]]*}"}" in '#'*) continue ;; esac     # a comment line cannot execute
      [[ "$line" =~ $MS_TOKEN_RE ]] || [[ "$line" =~ $MS_GLOB_RE ]] || [[ "$line" =~ $MS_PATH_RE ]] || [[ "$line" =~ $MS_SPLIT_RE ]] || continue
      if [ -f "$pr/$rel" ] && grep -qxF -- "$line" "$pr/$rel"; then continue; fi
      echo "SAFETY: $rel carries a signal/host-power/computed-path line that is not byte-identical to the pristine tree: $line"; bad=1
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
  # WF14 R2-T2: podman inside the containment is a stub: `ps` lists nothing (exit 0), every other subcommand is refused and logged. The host podman is never reached through PATH.
  printf '#!/bin/sh\ncase "$1" in ps|ls|list|inspect|version|info) exit 0 ;; esac\necho "BLOCKED stub podman $*" >>"%s"\nexit 1\n' "$MS_LOG" >"$MS_BIN/podman"; chmod +x "$MS_BIN/podman"
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
    pg=$({ IFS= read -r -d '' pst; } 2>/dev/null <"/proc/$t/stat"; pst=${pst##*) }; set -- $pst; echo "${3:-}")   # pure bash: a line-oriented sed was fooled by a comm that holds a newline (round 5)
    [[ "$pg" =~ ^[0-9]+$ ]] && [ "$pg" -gt 1 ] || { ms_blocked "kill $* (process group of $t is '$pg')"; return 1; }
  done
  [ "$tgt" -gt 0 ] || { ms_blocked "kill $* (no target)"; return 1; }
  "${MS_REAL_KILL:-/usr/bin/kill}" "$@"
}
kill() { ms_kill_guard "$@"; }
pkill() { ms_blocked "pkill $*"; }
killall() { ms_blocked "killall $*"; }
podman() { case "${1:-}" in ps|ls|list|inspect|version|info) return 0 ;; esac; ms_blocked "podman $*"; }
# WF14 RB8: the containment cannot be redefined or removed from inside a contained run (a redefinition fails, the original stays).
readonly -f kill pkill killall podman ms_blocked ms_kill_guard 2>/dev/null
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
