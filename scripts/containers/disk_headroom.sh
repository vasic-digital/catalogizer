#!/usr/bin/env bash
# disk_headroom.sh - T001. Free-space headroom gate (constitution section 12.9) for image pulls and builds.
# Usage: disk_headroom.sh --need <bytes> [--op-id <id>]
#   --need   size of the image the caller pulls or builds (from the lock entry, or from the registry manifest)
# Checks two filesystems: the rootless podman graphroot (`podman info`) and the repository filesystem; each must keep
# at least min_free_bytes (disk_headroom.conf, absolute bytes) free AFTER the need. Writes before/after values to
# $EV/disk/<op_id>.json, or to $DISK_HEADROOM_OUT_DIR when that is set (CPA sets it to $CPA_RUN/disk/).
# Exit: 0 pass; 1 refused (disk_below_headroom | disk_free_unreadable | commit_turn_held); 2 usage/configuration error
#       (config_percentage_refused | config_missing_min_free_bytes | config_not_integer | config_duplicate_key | op_id_too_long | usage).
# Commit-turn freeze (T580e consumer): only when DISK_HEADROOM_OUT_DIR is unset, so only before a write to
# $EV/disk/<op_id>.json, .audit/commit_turn.json is read; while it names a run_id other than EVREC_TURN_RUN_ID the
# reaper is called once through the host entry point, the grant re-read, and a still-held, unreadable or non-JSON
# grant refuses with commit_turn_held and writes nothing. "No grant" means a PROVEN absence only (the .audit directory absent, or searchable and without the
# file); a grant that cannot be reached (untraversable .audit, dangling symlink, .audit being a file) counts as unreadable: held. A set-but-empty
# DISK_HEADROOM_OUT_DIR counts as unset for the freeze and for the record path alike (one decision, OUT_DIR_IN_USE).
# DISK_HEADROOM_REAPER_TIMEOUT (seconds, default 60) bounds the reaper call; a timeout counts as a failing reaper.
# DISK_HEADROOM_CMD_TIMEOUT (seconds, default 30) bounds `podman info` and `df`; a timeout counts as an unreadable value
# (disk_free_unreadable). Both timeouts must be positive base-10 integers: GNU `timeout 0` means NO timeout, so 0 is refused
# (exit 2, reason config_reaper_timeout | config_cmd_timeout).
# Signals: SIGTERM/SIGINT/SIGHUP while a bounded probe (`podman info`, `df`) or the reaper runs stops that probe's whole chain (exit
# 128+n, nothing written, temp output removed). Only processes this shell started are signalled, each by an exact pid > 1 after /proc
# proves it is ours (11.4.263 / 11.4.196(D)); no pgrep/killall, no process-group signal. A child inside execve reads an EMPTY /proc cmdline: it
# is still ours when its parent link and start time say so, and it is KILLed like a pre-exec child. The bounded probes write into one private
# mktemp -d directory (mode 0700), removed as soon as they are done and by the signal handler.
# Test overrides: DISK_HEADROOM_CONF, DISK_HEADROOM_REPO_ROOT, EV, CPA_HOST_ENTRY.
set -u
command -v jq >/dev/null 2>&1 || { echo "disk_headroom: jq is required" >&2; exit 2; }
# Every external tool is checked up front, so a missing one is refused under its own name (rc 2) and never surfaces as a misleading
# disk_free_unreadable / config_duplicate_key refusal further down.
for _dep in timeout grep dirname cut tr date mktemp mkdir chmod mv rm sleep cat; do command -v "$_dep" >/dev/null 2>&1 || { echo "disk_headroom: REFUSED reason=dependency_missing $_dep is required" >&2; exit 2; }; done   # MUT:preflight
ROOT_DIR="${DISK_HEADROOM_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
CONF="${DISK_HEADROOM_CONF:-$(dirname "${BASH_SOURCE[0]}")/disk_headroom.conf}"
EV="${EV:-$ROOT_DIR/specs/001-full-project-audit-remediation/evidence}"
GRANT="$ROOT_DIR/.audit/commit_turn.json"
# ONE decision, used for the freeze AND for the record path (round 4, R3): a set-but-empty DISK_HEADROOM_OUT_DIR is "not in use" for both, so the two can
# never diverge into a write below $EV/disk during a held turn.
OUT_DIR_IN_USE=0; [ -n "${DISK_HEADROOM_OUT_DIR:-}" ] && OUT_DIR_IN_USE=1   # MUT:outdir-decision

# Numbers are validated base-10 within bounds: no leading zeros (bash would read octal), at most 19 digits and at most
# INT64_MAX (bash arithmetic wraps beyond it). Anything else is refused, never computed.
valid_int() { # $1 value; 0 when ^(0|[1-9][0-9]*)$ and <= 9223372036854775807
  case "$1" in ''|*[!0-9]*) return 1;; esac
  case "$1" in 0) return 0;; 0*) return 1;; esac
  [ "${#1}" -le 19 ] || return 1
  [ "${#1}" -lt 19 ] && return 0
  [[ ! "$1" > 9223372036854775807 ]]
}
valid_pos_int() { valid_int "$1" && [ "$1" != 0 ]; }   # MUT:pos-int
REAPER_TO="${DISK_HEADROOM_REAPER_TIMEOUT:-60}"; CMD_TO="${DISK_HEADROOM_CMD_TIMEOUT:-30}"
valid_pos_int "$REAPER_TO" || { echo "disk_headroom: REFUSED reason=config_reaper_timeout DISK_HEADROOM_REAPER_TIMEOUT '$REAPER_TO' must be a positive base-10 integer (0 would disable the bound)" >&2; exit 2; }
valid_pos_int "$CMD_TO" || { echo "disk_headroom: REFUSED reason=config_cmd_timeout DISK_HEADROOM_CMD_TIMEOUT '$CMD_TO' must be a positive base-10 integer (0 would disable the bound)" >&2; exit 2; }
# op-id: ^[A-Za-z0-9._-]{1,128}$, no leading '.', so it is one plain file name below the record dir: no '/', no '..', no hidden or temp-shaped name.
valid_op_id() {
  case "$1" in ''|*[!A-Za-z0-9._-]*) return 1;; esac   # MUT:opid-charset
  case "$1" in .*) return 1;; esac   # MUT:opid-dot
  return 0
}
NEED=""; OP_ID=""; SEEN_NEED=0; SEEN_OP=0
while [ $# -gt 0 ]; do
  case "$1" in
    --need|--op-id)
      [ $# -ge 2 ] || { echo "disk_headroom: REFUSED reason=usage $1 requires a value" >&2; exit 2; }
      if [ "$1" = --need ]; then [ "$SEEN_NEED" = 0 ] || { echo "disk_headroom: REFUSED reason=usage --need given twice" >&2; exit 2; }; SEEN_NEED=1; NEED="$2"
      else [ "$SEEN_OP" = 0 ] || { echo "disk_headroom: REFUSED reason=usage --op-id given twice" >&2; exit 2; }; SEEN_OP=1; OP_ID="$2"; fi
      shift 2;;
    *) echo "disk_headroom: REFUSED reason=usage unknown argument '$1'" >&2; exit 2;;
  esac
done
valid_int "$NEED" || { echo "disk_headroom: REFUSED reason=usage --need <bytes> must be a base-10 integer without leading zeros, at most 9223372036854775807" >&2; exit 2; }
[ "$SEEN_OP" = 1 ] || OP_ID="op-$(date -u +%Y%m%dT%H%M%SZ)-$$"   # MUT:opid-empty
valid_op_id "$OP_ID" || { echo "disk_headroom: REFUSED reason=usage --op-id must match [A-Za-z0-9_-][A-Za-z0-9._-]* (one plain file name: no '/', no leading '.', not empty)" >&2; exit 2; }
[ "${#OP_ID}" -le 128 ] || { echo "disk_headroom: REFUSED reason=op_id_too_long --op-id is ${#OP_ID} characters, at most 128 allowed" >&2; exit 2; }   # MUT:opid-cap

# --- threshold (consumer data); a percentage is a configuration error ---
KEYLINES="$(grep -E '^[[:space:]]*min_free_bytes[[:space:]]*=' "$CONF" 2>/dev/null)"
[ "$(printf '%s\n' "$KEYLINES" | grep -c .)" -le 1 ] || { echo "disk_headroom: REFUSED reason=config_duplicate_key min_free_bytes appears more than once in $CONF" >&2; exit 2; }
RAW="$(printf '%s\n' "$KEYLINES" | cut -d= -f2-)"   # MUT:conf-trim
# Only the leading and trailing whitespace of the value is trimmed (a CR of a CRLF file included); whitespace INSIDE the value is kept, so
# valid_int refuses "100 0" and "1 000" instead of reading them as 1000 (round 4, R8).
RAW="${RAW#"${RAW%%[![:space:]]*}"}"; RAW="${RAW%"${RAW##*[![:space:]]}"}"
[ -n "$RAW" ] || { echo "disk_headroom: REFUSED reason=config_missing_min_free_bytes in $CONF" >&2; exit 2; }
case "$RAW" in *%*) echo "disk_headroom: REFUSED reason=config_percentage_refused min_free_bytes must be an absolute byte count, got '$RAW'" >&2; exit 2;; esac
valid_int "$RAW" || { echo "disk_headroom: REFUSED reason=config_not_integer min_free_bytes '$RAW' (base-10, no leading zeros, at most 9223372036854775807)" >&2; exit 2; }
MIN="$RAW"

# --- own-process signalling (constitution 11.4.263 / 11.4.196(D)) ---
# The only processes this gate may signal are the ones it started itself: the `timeout` of a bounded probe or of the reaper, and
# the descendants that `timeout` started. Every target is addressed by an exact pid (> 1, never a process group) after /proc proves
# it is ours: the `timeout` has this shell as parent and a command line starting with "timeout "; a descendant was found by walking
# parent links from that verified `timeout`, and is re-checked against its start time (stat field 22) before it is signalled, so a
# recycled pid is never hit. Nothing is found by name (no pgrep/killall).
OUT_F=""; OUT_N=0; SNAP=()
proc_fields() { # $1 pid; sets PF_STATE PF_PPID PF_START from /proc/<pid>/stat; 1 when unreadable (no fork: builtin read only)
  local s; IFS= read -r s 2>/dev/null <"/proc/$1/stat" || return 1
  s="${s##*) }"; set -- $s
  PF_STATE="$1"; PF_PPID="$2"; PF_START="${20}"
  [ -n "$PF_START" ]
}
SELF_CMD="$(tr '\0' ' ' 2>/dev/null <"/proc/$$/cmdline")"   # this gate's own command line: a child forked from it shows this until its exec completes
proc_fields "$$" && SELF_START="$PF_START" || SELF_START=""   # this shell's start time (stat field 22): a child of ours never started before it
own_child() { # $1 pid; 0 only when pid > 1, it is no zombie, its parent is this shell and its cmdline starts with "timeout ", is still this gate's own (pre-exec) or is EMPTY (inside execve)
  local p="$1" cmd
  case "$p" in ''|*[!0-9]*) return 1;; esac
  [ "$p" -gt 1 ] || return 1
  proc_fields "$p" || return 1
  [ "$PF_STATE" != Z ] || return 1
  [ "$PF_PPID" = "$$" ] || return 1   # MUT:own-ppid
  cmd="$(tr '\0' ' ' 2>/dev/null <"/proc/$p/cmdline")"
  case "$cmd" in "timeout "*) return 0;; esac   # MUT:own-cmdline
  [ -n "$SELF_CMD" ] && [ "$cmd" = "$SELF_CMD" ] && return 0   # MUT:own-preexec
  # Inside execve the kernel has swapped the memory map but not yet set up the argument area: /proc/<pid>/cmdline reads EMPTY although the child is
  # alive, not a zombie and ours. An empty read is a read the instrument could not make, never evidence of "not ours" (round 4, R1): the parent link
  # (checked above) and a start time not older than this shell's own prove the identity.
  [ -z "$cmd" ] && [ -n "$SELF_START" ] && [ "$PF_START" -ge "$SELF_START" ] 2>/dev/null && return 0   # MUT:own-empty
  return 1
}
pre_exec_child() { # $1 pid already proven ours by own_child: 0 when it has not finished exec'ing (its command line is still this gate's own, or empty inside execve)
  local cmd; cmd="$(tr '\0' ' ' 2>/dev/null <"/proc/$1/cmdline")"
  [ -z "$cmd" ] && return 0   # MUT:preexec-empty
  [ -n "$SELF_CMD" ] && [ "$cmd" = "$SELF_CMD" ]
}
snapshot_descendants() { # $1 verified root pid; sets SNAP to "pid:starttime" of every non-zombie descendant found through parent links
  SNAP=()
  local -A kids=() start=() state=()
  local d p
  for d in /proc/[0-9]*; do
    p="${d#/proc/}"
    proc_fields "$p" || continue
    kids[$PF_PPID]+="$p "; start[$p]="$PF_START"; state[$p]="$PF_STATE"
  done
  local queue=("$1") q c
  while [ "${#queue[@]}" -gt 0 ]; do
    q="${queue[0]}"; queue=("${queue[@]:1}")
    for c in ${kids[$q]-}; do
      [ "$c" -gt 1 ] && [ "$c" != "$$" ] || continue
      [ "${state[$c]}" != Z ] && SNAP+=("$c:${start[$c]}")
      queue+=("$c")   # MUT:snap-descend
    done
  done
}
snap_alive() { local e p; for e in "${SNAP[@]}"; do p="${e%%:*}"; proc_fields "$p" && [ "$PF_STATE" != Z ] && [ "$PF_START" = "${e#*:}" ] && return 0; done; return 1; }
snap_signal() { # $1 signal; signals each snapshotted descendant that is still the same process (pid > 1 and unchanged start time)
  local e p
  for e in "${SNAP[@]}"; do
    p="${e%%:*}"
    [ "$p" -gt 1 ] || continue   # MUT:snap-pid-guard
    proc_fields "$p" && [ "$PF_STATE" != Z ] && [ "$PF_START" = "${e#*:}" ] && kill -"$1" "$p" 2>/dev/null   # MUT:snap-start-guard
  done
  return 0
}
stop_chain() { # $1 pid of a child we started: stop it and everything it started; TERM first, KILL for what ignores it
  local p="$1" i
  own_child "$p" || return 0
  # Fork-to-exec window: a signal that lands before the child has exec'd finds it still wearing this gate's own command line. It has not started
  # anything yet, so it has no descendants and must simply never get to exec: KILL it (an exact pid of our own child, never a group).
  if pre_exec_child "$p"; then kill -KILL "$p" 2>/dev/null; return 0; fi   # MUT:preexec-kill
  snapshot_descendants "$p"
  kill -TERM "$p" 2>/dev/null
  i=0; while [ "$i" -lt 30 ] && own_child "$p"; do sleep 0.1; i=$((i + 1)); done
  if own_child "$p"; then kill -KILL "$p" 2>/dev/null; fi   # MUT:kill-fallback
  i=0; while [ "$i" -lt 10 ] && snap_alive; do sleep 0.1; i=$((i + 1)); done
  snap_signal TERM
  i=0; while [ "$i" -lt 10 ] && snap_alive; do sleep 0.1; i=$((i + 1)); done
  snap_signal KILL   # MUT:kill-chain
}
BDIR=""
drop_bdir() { # removes the private directory of the bounded probes (only a directory this gate created with mktemp -d)
  case "$BDIR" in */disk_headroom.??????) rm -rf "$BDIR";; esac
  BDIR=""
}
on_signal() { # $1 signal name, $2 exit status (128+n): stop the child we are waiting for, remove temp output, write nothing, exit 128+n
  local bang="$!"   # bash sets $! at the fork itself, so it names the child even when the signal lands before our own variable could be set
  trap '' TERM INT HUP
  stop_chain "$bang"   # MUT:bang-fallback
  [ -z "$OUT_F" ] || rm -f "$OUT_F"
  drop_bdir
  echo "disk_headroom: interrupted by SIG$1, running probe stopped, nothing written" >&2
  exit "$2"
}
trap 'on_signal TERM 143' TERM; trap 'on_signal INT 130' INT; trap 'on_signal HUP 129' HUP   # MUT:signal-trap

# --- free space of both filesystems (df -B1 is the oracle; an unreadable value is refused, never enough) ---
# bounded_out <cmd...>: runs the command under `timeout` as a background child of THIS shell (never under $(...): a signal to the gate
# must reach it and a child of a command substitution could not be proven ours), with its output going to a file (never a pipe held
# by an orphaned grandchild, shell footgun I7). Sets BOUND_OUT to that output; a timeout or failure leaves it empty, which every
# caller treats as unreadable. The output files live in ONE private directory (mktemp -d, mode 0700, random name, removed as soon as the probes are
# done and by the signal handler), so no name below a shared TMPDIR is predictable and nothing planted there (a FIFO, a symlink) is ever opened
# (round 4, R4). Noclobber stays as defence in depth.
BOUND_OUT=""
bounded_out() {
  BOUND_OUT=""
  [ -n "$BDIR" ] || return 0   # no private directory: nothing is read, every caller treats that as unreadable
  OUT_N=$((OUT_N + 1)); OUT_F="$BDIR/out.$OUT_N"   # MUT:outname
  if ! { set -C; : >"$OUT_F"; } 2>/dev/null; then set +C; OUT_F=""; return 0; fi
  set +C
  timeout -k 2 "$CMD_TO" "$@" >"$OUT_F" 2>/dev/null &   # MUT:kill-after
  local pid=$!   # MUT:pid-assign
  local rc=0; wait "$pid" 2>/dev/null || rc=$?
  if [ "$rc" -eq 0 ]; then   # MUT:bound-rc
    IFS= read -r -d '' BOUND_OUT <"$OUT_F"
    while [[ "$BOUND_OUT" == *$'\n' ]]; do BOUND_OUT="${BOUND_OUT%$'\n'}"; done
  fi
  rm -f "$OUT_F"; OUT_F=""
  return 0
}
BDIR="$(mktemp -d "${TMPDIR:-/tmp}/disk_headroom.XXXXXX" 2>/dev/null)" || BDIR=""   # MUT:privdir
bounded_out podman info --format '{{.Store.GraphRoot}}'   # MUT:podman-timeout
GRAPHROOT="$BOUND_OUT"
FREE_OUT=""
free_bytes() { # sets FREE_OUT to the one trimmed token df reports as available; empty when the last line is not exactly one field
  local out; FREE_OUT=""
  bounded_out df -B1 --output=avail "$1"   # MUT:df-timeout
  out="${BOUND_OUT##*$'\n'}"; out="${out//$'\r'/}"
  out="${out#"${out%%[![:space:]]*}"}"; out="${out%"${out##*[![:space:]]}"}"
  case "$out" in *[[:space:]]*) return 0;; esac   # MUT:single-field
  FREE_OUT="$out"
}
VERDICT=pass; REASON=""; FSJ="[]"; READ_OK=0
for spec in "graphroot:$GRAPHROOT" "repository:$ROOT_DIR"; do
  role="${spec%%:*}"; path="${spec#*:}"
  free=""; [ -z "$path" ] || { free_bytes "$path"; free="$FREE_OUT"; }
  if valid_int "$free"; then isnum=1; else isnum=0; fi
  case "$isnum" in
    0)
      VERDICT=refused; REASON=disk_free_unreadable   # unreadable outranks below-headroom whatever the filesystem order
      FSJ="$(jq -c --arg r "$role" --arg p "$path" '. + [{role:$r,path:$p,free_before:null,free_after:null,readable:false}]' <<<"$FSJ")";;
    *)
      READ_OK=$((READ_OK + 1))
      after=$((free - NEED))
      if [ "$after" -lt "$MIN" ]; then VERDICT=refused; [ "$REASON" = disk_free_unreadable ] || REASON=disk_below_headroom; fi
      FSJ="$(jq -c --arg r "$role" --arg p "$path" --argjson b "$free" --argjson a "$after" '. + [{role:$r,path:$p,free_before:$b,free_after:$a,readable:true}]' <<<"$FSJ")";;
  esac
done

drop_bdir

# a pass needs exactly 2 filesystems actually read; anything less is a refusal (fail closed).
# Defence in depth only: every unreadable branch above already refuses, so no test can pin this line on its own.
if [ "$VERDICT" = pass ] && [ "$READ_OK" -ne 2 ]; then VERDICT=refused; REASON=disk_free_unreadable; fi

# --- commit-turn freeze (only before a write to $EV/disk) ---
grant_proven_absent() { # 0 only when "no grant" is PROVEN: the grant's directory is absent (and no link) or a directory this user can search
  local d="${GRANT%/*}"
  if [ ! -e "$d" ] && [ ! -L "$d" ]; then return 0; fi
  [ -d "$d" ] && [ -x "$d" ]
}
read_grant_run() { # sets GRUN; returns 0 grant read, 1 unreadable/not JSON/not provably absent, 2 no grant file
  GRUN=""
  # `test -e` is false for ENOENT but ALSO for a dangling symlink and for a stat that fails (an untraversable .audit, .audit being a file): only a
  # proven absence is "no grant"; every other case is an unreadable grant and the turn stays held (round 4, R2).
  if [ ! -e "$GRANT" ] && [ ! -L "$GRANT" ]; then grant_proven_absent && return 2; return 1; fi   # MUT:enoent
  [ -r "$GRANT" ] || return 1   # MUT:unreadable-as-none
  GRUN="$(jq -er '.run_id | select(type=="string" and length>0)' "$GRANT" 2>/dev/null)" || return 1
  return 0
}
turn_freeze_check() {
  [ "$OUT_DIR_IN_USE" = 1 ] && return 0
  local st
  read_grant_run; st=$?
  [ "$st" -eq 2 ] && return 0
  [ "$st" -eq 0 ] && [ "$GRUN" = "${EVREC_TURN_RUN_ID:-}" ] && return 0
  if [ "$st" -eq 0 ]; then
    # held by another run: one reap through the host entry point (an absent or failing reaper leaves the refusal in place)
    # The reaper runs as OUR background child (exec makes the child pid the `timeout` itself) so that a signal sent to the gate
    # while it waits reaches it: bash defers a trap past a foreground child, and an untrapped SIGTERM would orphan the reaper.
    (cd "$ROOT_DIR" && exec timeout -k 2 "$REAPER_TO" "${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}" --exec-approved scripts/release/commit_turn_check.sh --reap) >/dev/null 2>&1 &   # MUT:direct-reap
    local rpid=$!   # MUT:pid-assign
    wait "$rpid" 2>/dev/null || true   # a failing or timed-out reaper leaves the refusal in place
    read_grant_run; st=$?   # MUT:no-reread
    [ "$st" -eq 2 ] && return 0
    [ "$st" -eq 0 ] && [ "$GRUN" = "${EVREC_TURN_RUN_ID:-}" ] && return 0
  fi
  echo "disk_headroom: REFUSED reason=commit_turn_held grant $GRANT held by run_id '${GRUN:-unreadable}', nothing written" >&2
  return 1
}
turn_freeze_check || exit 1   # MUT:ignore-grant
# nothing left to stop. From here on a signal removes the temp record before the gate exits 128+n (the default action would leave the temp file in
# the evidence directory); the final rename is atomic, so the record is either complete or absent.
TMP=""
trap 'rm -f "$TMP"; exit 143' TERM; trap 'rm -f "$TMP"; exit 130' INT; trap 'rm -f "$TMP"; exit 129' HUP   # MUT:write-trap

# --- record ---
if [ "$OUT_DIR_IN_USE" = 1 ]; then OUTDIR="$DISK_HEADROOM_OUT_DIR"; else OUTDIR="$EV/disk"; fi
mkdir -p "$OUTDIR" || { echo "disk_headroom: cannot create $OUTDIR" >&2; exit 1; }
TMP="$(mktemp "$OUTDIR/.$OP_ID.XXXXXX")" || { echo "disk_headroom: cannot create a temporary record in $OUTDIR" >&2; exit 1; }   # MUT:tmpname
jq -n --arg op "$OP_ID" --argjson need "$NEED" --argjson min "$MIN" --argjson fs "$FSJ" --arg v "$VERDICT" --arg r "$REASON" \
  --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{schema:"disk-headroom/1",op_id:$op,at:$at,need_bytes:$need,min_free_bytes:$min,filesystems:$fs,verdict:$v,reason:(if $r=="" then null else $r end)}' \
  >"$TMP" && chmod 0644 "$TMP" && mv "$TMP" "$OUTDIR/$OP_ID.json" || { rm -f "$TMP"; exit 1; }
if [ "$VERDICT" = pass ]; then echo "disk_headroom: pass op=$OP_ID need=$NEED min_free=$MIN record=$OUTDIR/$OP_ID.json"; exit 0; fi
echo "disk_headroom: REFUSED reason=$REASON op=$OP_ID need=$NEED min_free=$MIN record=$OUTDIR/$OP_ID.json" >&2
exit 1
