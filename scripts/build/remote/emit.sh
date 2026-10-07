#!/usr/bin/env bash
# emit.sh - T005b: the remote emitter (runs on the build host; for the proof, on this host through the local transport).
#   emit.sh run    --dir D --build-id B --run-id R --variant V --host H --image IMG-ID --src SRCDIR --runp RUN_PINNED.sh
#                  [--heartbeat S] [--op-id ID] [--need BYTES] [--network-none] [--progress log|log+cpu] -- <build argv...>      (key: first stdin line)
#   emit.sh attach --dir D --from N      stream the journal lines after the first N, signed (resend from the last acknowledged seq)   (key: first stdin line)
#   emit.sh fetch  --dir D               tar of the build's artifacts/ tree on stdout
#   emit.sh cancel --dir D               stop the build container (by label) and the daemon/build, reaped by exact pid
# `run` starts a detached daemon (setsid) that wraps the build container (rootless, through run_pinned.sh), appends build-event/1 events to D/journal.jsonl
# (one JSON object per line, seq per build from 1) and streams the journal like `attach`. THE JOURNAL HOLDS NO SIGNATURE AT REST: an event is signed when it
# is STREAMED (HMAC-SHA256 over the canonical form of contracts/build-event.schema.json: python json.dumps sort_keys, compact, ensure_ascii=False), with the
# per-build key the driver sends on this process's stdin (first line) at every `run` and `attach`; the key is held in a shell variable only, never written on
# this host, never logged. A line that already carries an hmac passes through unchanged. Without a valid key the emitter REFUSES to send (exit 3, nothing on
# stdout): the driver then sees no progress and the build ends through the liveness rule (T005a (c), (i2)).
# Restart (T005a (i2)): the build runs as a background group that records `build.pid` (pid + start time) and, when it ends, `build.rc`. When `run` finds a
# journal with no `completed` line and no live daemon (the emitter died, the build host rebooted its emitter), it starts an ADOPT daemon with the re-sent key:
# it waits for `build.rc` (an inotify wait on the file, one heartbeat per period), emits the remaining heartbeats and the `completed` event, never starts the
# build again; a build that is gone with no `build.rc` ends `infra_failed`.
# Progress (T005a (c)): progress_offset = the build log's byte length (+ the build container's CPU time in ms with `--progress log+cpu`, the default, so a quiet
# build that works is not read as flat; kept monotone through $DIR/po.max); stage = the log's line count. The container's cgroup is the one of the container
# labelled catalogizer.op_id=<op id> (podman ps/inspect, EMIT_CGROUP_ROOT default /sys/fs/cgroup); an unreadable cgroup contributes nothing, never a guess.
# `completed` carries peak_rss_bytes = the container cgroup's memory.peak as last sampled at a heartbeat (a lower bound at heartbeat granularity: the cgroup is
# gone with the container), omitted when never read. Event timing is the host's monotonic clock (/proc/uptime) and elapsed_monotonic_ms in each heartbeat; no
# sent_at arithmetic is ever used by the driver.
# Sourcing this file defines the functions only (the tests' fake emitters reuse sign/emit).
set -u
EMIT_HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
EVWAIT="${EVWAIT:-$EMIT_HERE/../lib/evwait.py}"

mono_ms() { local up _; read -r up _ < /proc/uptime; local i=${up%.*} f=${up#*.}; f=${f}00; echo $(( i * 1000 + 10#${f:0:3} )); }
isoz() { date -u +%Y-%m-%dT%H:%M:%SZ; }
sign_event() { # keyhex : json on stdin -> signed canonical json on stdout. The key goes through stdin, never argv or the environment.
  { printf '%s\n' "$1"; cat; } | python3 -I -c 'import sys, json, hmac, hashlib
k = bytes.fromhex(sys.stdin.readline().strip())
ev = json.loads(sys.stdin.read()); ev.pop("hmac", None)
c = json.dumps(ev, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
ev["hmac"] = hmac.new(k, c, hashlib.sha256).hexdigest()
sys.stdout.write(json.dumps(ev, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n")'; }
stream_signed() { # keyhex : lines on stdin -> stdout. A line without an hmac is signed (re-signed from the journal) with the key, one that carries one passes through.
  local line
  while IFS= read -r line; do
    case $line in *'"hmac":'*) printf '%s\n' "$line";; *) printf '%s\n' "$line" | sign_event "$1" || return 1;; esac
  done; }

emit_event() { # dir keyhex build run variant host kind [extra-json-object]   appends one UNSIGNED event; seq allocated under a lock (the key argument is kept for callers)
  local dir=$1 b=$3 r=$4 v=$5 h=$6 kind=$7 extra=${8:-} seq line
  [ -n "$extra" ] || extra='{}'
  exec 7>>"$dir/.seq.lock"; flock 7
  seq=$(( $(cat "$dir/seq" 2>/dev/null || echo 0) + 1 ))
  line=$(jq -nc --arg b "$b" --arg r "$r" --arg v "$v" --arg h "$h" --arg k "$kind" --argjson s "$seq" --arg t "$(isoz)" --argjson x "$extra" \
        '{schema:"build-event/1",run_id:$r,build_id:$b,variant:$v,seq:$s,kind:$k,host:$h,sent_at:$t} + $x') || { flock -u 7; return 1; }
  printf '%s\n' "$line" >> "$dir/journal.jsonl" && printf '%s\n' "$seq" > "$dir/seq.tmp" && mv -f "$dir/seq.tmp" "$dir/seq"
  flock -u 7; exec 7>&-
}

manifest_sha() { # artifacts dir : sha256 of the sorted "sha256  relpath" lines (the content address of the artifact tree)
  ( cd "$1" 2>/dev/null && LC_ALL=C find . -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sed 's#  \./#  #' | sha256sum | cut -d' ' -f1 ) ; }

pid_start() { awk '{print $22}' "/proc/$1/stat" 2>/dev/null; }
valid_key() { [[ ${1:-} =~ ^[0-9a-f]{64}$ ]]; }

args_parse() { # fills DIR BID RUN VARIANT HOST IMG SRC RUNP HB OPID NEED NETNONE PROGRESS ARGV
  DIR=; BID=; RUN=; VARIANT=primary; HOST=local; IMG=; SRC=; RUNP=; HB=2; OPID=; NEED=0; NETNONE=0; PROGRESS=log+cpu; ARGV=(); FROM=0
  while [ $# -gt 0 ]; do
    case $1 in
      --dir) DIR=$2; shift 2;; --build-id) BID=$2; shift 2;; --run-id) RUN=$2; shift 2;; --variant) VARIANT=$2; shift 2;;
      --host) HOST=$2; shift 2;; --image) IMG=$2; shift 2;; --src) SRC=$2; shift 2;; --runp) RUNP=$2; shift 2;;
      --heartbeat) HB=$2; shift 2;; --op-id) OPID=$2; shift 2;; --need) NEED=$2; shift 2;; --network-none) NETNONE=1; shift;;
      --progress) PROGRESS=$2; shift 2;;
      --from) FROM=$2; shift 2;; --) shift; ARGV=("$@"); break;; *) echo "emit: unknown option $1" >&2; return 2;;
    esac
  done
  [ -n "$DIR" ] || { echo "emit: --dir required" >&2; return 2; }
  case $PROGRESS in log|log+cpu) ;; *) echo "emit: --progress is log or log+cpu" >&2; return 2;; esac
}

# ---- the build container's cgroup (rootless podman, cgroup v2): CPU time and memory peak
cg_sample() { # sets CPU_MS (the container's CPU time in ms, 0 when unreadable); raises $DIR/peak_rss to memory.peak
  CPU_MS=0
  local c rel p v cur
  c=$(podman ps -q --filter "label=catalogizer.op_id=$OPID" 2>/dev/null | head -1); [ -n "$c" ] || return 0
  rel=$(podman inspect --format '{{.State.CgroupPath}}' "$c" 2>/dev/null); [ -n "$rel" ] || return 0
  p="${EMIT_CGROUP_ROOT:-/sys/fs/cgroup}$rel"; [ -d "$p" ] || return 0
  v=$(awk '/^usage_usec/ {print $2}' "$p/cpu.stat" 2>/dev/null); [[ $v =~ ^[0-9]+$ ]] && CPU_MS=$(( v / 1000 ))
  v=$(cat "$p/memory.peak" 2>/dev/null); cur=$(cat "$DIR/peak_rss" 2>/dev/null || echo 0)
  if [[ $v =~ ^[0-9]+$ ]] && [ "$v" -gt "$cur" ]; then printf '%s\n' "$v" > "$DIR/peak_rss.tmp" && mv -f "$DIR/peak_rss.tmp" "$DIR/peak_rss"; fi
  return 0
}
hb_event() { # emits one heartbeat: progress_offset, stage, elapsed monotonic time since the build started on this host
  local po lines prev t0
  po=$(stat -c %s "$DIR/build.log" 2>/dev/null || echo 0); lines=$(wc -l < "$DIR/build.log" 2>/dev/null || echo 0)
  if [ "$PROGRESS" = log+cpu ]; then cg_sample; po=$(( po + CPU_MS )); fi
  prev=$(cat "$DIR/po.max" 2>/dev/null || echo 0); [ "$po" -ge "$prev" ] || po=$prev      # progress never goes backwards (the core refuses a heartbeat that does)
  printf '%s\n' "$po" > "$DIR/po.max.tmp" && mv -f "$DIR/po.max.tmp" "$DIR/po.max"
  t0=$(cat "$DIR/t0" 2>/dev/null || echo 0); local el=$(( $(mono_ms) - t0 )); [ "$el" -ge 0 ] || el=0
  emit_event "$DIR" "" "$BID" "$RUN" "$VARIANT" "$HOST" heartbeat \
    "$(jq -nc --argjson p "${po:-0}" --argjson s "${lines:-0}" --argjson e "$el" '{progress_offset:$p,stage:$s,elapsed_monotonic_ms:$e}')"
}
stop_build() { # the container by its label (exact ids), then the build group by exact pid (identity proven by start time), never a pattern kill
  local c bp bs
  for c in $(podman ps -q --filter "label=catalogizer.op_id=$OPID" 2>/dev/null); do podman kill "$c" >/dev/null 2>&1; done
  if [ -s "$DIR/build.pid" ]; then read -r bp bs < "$DIR/build.pid"; if [ "${bp:-0}" -gt 1 ] 2>/dev/null && [ "$(pid_start "$bp")" = "${bs:-x}" ]; then kill "$bp" 2>/dev/null; fi; fi
}
finish_build() { # rc : classify, content-address the artifacts, emit completed
  local rc=$1 ec manifest img_digest logsha peak ex
  if [ "$cancelled" = 1 ]; then ec=cancelled
  elif [ "$rc" = 0 ]; then ec=succeeded
  elif grep -q 'run_pinned: REFUSED' "$DIR/build.log" 2>/dev/null || [ "$rc" -ge 125 ]; then ec=infra_failed
  else ec=build_failed; fi
  manifest=$(manifest_sha "$DIR/out/artifacts"); img_digest="sha256:$manifest"
  logsha=$(sha256sum "$DIR/build.log" | cut -d' ' -f1)
  peak=$(cat "$DIR/peak_rss" 2>/dev/null || true); ex='{}'
  [[ ${peak:-} =~ ^[1-9][0-9]*$ ]] && ex=$(jq -nc --argjson p "$peak" '{peak_rss_bytes:$p}')
  emit_event "$DIR" "" "$BID" "$RUN" "$VARIANT" "$HOST" completed \
    "$(jq -nc --arg e "$ec" --arg m "$manifest" --arg i "$img_digest" --arg l "$logsha" --argjson x "$ex" '{exit_class:$e,artifact_manifest_sha256:$m,image_digest:$i,remote_log_sha256:$l} + $x')"
}

daemon() { # runs detached; key = first stdin line
  local KEY bpid rc sp wp wrc
  read -r KEY || exit 3
  valid_key "$KEY" || exit 3
  mkdir -p "$DIR/out/artifacts"; : > "$DIR/build.log"
  printf '%s %s\n' "$$" "$(pid_start $$)" > "$DIR/daemon.pid"
  printf '%s\n' "$(mono_ms)" > "$DIR/t0"
  cancelled=0
  trap 'cancelled=1; stop_build' TERM
  emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" accepted
  # the build runs as a background group that records its pid and, when it ends, its exit status: a restarted emitter adopts it instead of starting it again
  { ( cd "$SRC" && DISK_HEADROOM_OUT_DIR="$DIR/disk" "$RUNP" --out "$DIR/out" --op-id "$OPID" --need "$NEED" $( [ "$NETNONE" = 1 ] && echo --network=none ) "$IMG" -- "${ARGV[@]}" ) >> "$DIR/build.log" 2>&1
    printf '%s\n' "$?" > "$DIR/build.rc.tmp" && mv -f "$DIR/build.rc.tmp" "$DIR/build.rc"; } &
  bpid=$!; printf '%s %s\n' "$bpid" "$(pid_start "$bpid")" > "$DIR/build.pid"
  rc=
  while [ -z "$rc" ]; do
    # a heartbeat every HB seconds: a producer-side timer (the consumers never poll); `wait -n` returns at once when the build ends
    sleep "$HB" & sp=$!
    wp=; wait -n -p wp "$bpid" "$sp"; wrc=$?
    if [ "${wp:-}" = "$bpid" ]; then rc=$(cat "$DIR/build.rc" 2>/dev/null || echo 255); kill "$sp" 2>/dev/null; wait "$sp" 2>/dev/null; break; fi
    [ "${wp:-}" = "$sp" ] || { kill "$sp" 2>/dev/null; wait "$sp" 2>/dev/null; continue; }   # a trapped TERM interrupted the wait: loop again
    hb_event
  done
  finish_build "$rc"
  exit 0
}

adopt() { # a restarted emitter: the build was started by an emitter that is gone; key = first stdin line (re-sent by the driver)
  local KEY bp bs rc wp
  read -r KEY || exit 3
  valid_key "$KEY" || exit 3
  printf '%s %s\n' "$$" "$(pid_start $$)" > "$DIR/daemon.pid.tmp" && mv -f "$DIR/daemon.pid.tmp" "$DIR/daemon.pid"
  cancelled=0; trap 'cancelled=1; stop_build' TERM
  read -r bp bs < "$DIR/build.pid" 2>/dev/null || { bp=0; bs=x; }
  while [ ! -s "$DIR/build.rc" ]; do
    [ "${bp:-0}" -gt 1 ] 2>/dev/null && [ "$(pid_start "$bp")" = "${bs:-x}" ] || break    # the build is gone and left no exit status
    python3 -I "$EVWAIT" waitfile "$DIR/build.rc" "$HB" & wp=$!; wait "$wp"
    [ -s "$DIR/build.rc" ] || hb_event
  done
  rc=$(cat "$DIR/build.rc" 2>/dev/null || echo 255)
  finish_build "$rc"
  exit 0
}

daemon_alive() { local p="" s; [ -s "$DIR/daemon.pid" ] && read -r p s < "$DIR/daemon.pid"; [ -n "$p" ] && [ "$p" -gt 1 ] 2>/dev/null && [ "$(pid_start "$p")" = "${s:-x}" ] && tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q 'emit.sh'; }

attach() { # keyhex : stream the journal after FROM lines, signed, blocking on inotify; ends at the completed line or when the daemon is gone
  local key=$1 p="" s
  valid_key "$key" || exit 3                                       # no key, no sending
  if [ -s "$DIR/daemon.pid" ]; then read -r p s < "$DIR/daemon.pid"; fi
  if [ -n "$p" ] && [ "$(pid_start "$p")" = "${s:-x}" ]; then python3 -I "$EVWAIT" tail "$DIR/journal.jsonl" "$FROM" "$p" "emit.sh" | stream_signed "$key"; return; fi
  touch "$DIR/journal.jsonl" 2>/dev/null
  python3 -I "$EVWAIT" tail "$DIR/journal.jsonl" "$FROM" | stream_signed "$key"
}

cmd_cancel() {
  local p="" s
  if [ -s "$DIR/daemon.pid" ]; then read -r p s < "$DIR/daemon.pid"; fi
  # exact pid > 1, proven ours by start time and by /proc cmdline (never a pattern kill, never a process-group signal)
  if [ -n "$p" ] && [ "$p" -gt 1 ] 2>/dev/null && [ "$(pid_start "$p")" = "$s" ] && tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q 'emit.sh'; then kill -TERM "$p"; echo "cancel sent"
  else
    # no live emitter (it died and none has been restarted): stop the container and the build group directly
    [ -n "$OPID" ] || OPID="emit-$(basename "$(dirname "$DIR")")"
    stop_build; echo "no live daemon"
  fi
}

main() {
  local sub=${1:-}; shift || true
  case $sub in
    run)    args_parse "$@" || exit 2; [ -n "$BID" ] && [ -n "$RUN" ] && [ -n "$SRC" ] && [ -n "$RUNP" ] && [ -n "$IMG" ] || exit 2
            [ -n "$OPID" ] || OPID="emit-$BID"
            mkdir -p "$DIR" || exit 2
            local key; read -r key || exit 3
            valid_key "$key" || exit 3                                # without the key the emitter sends nothing and starts nothing
            ( flock 9
              if [ ! -s "$DIR/journal.jsonl" ] && [ ! -s "$DIR/daemon.pid" ]; then
                # a new build: the daemon inherits the key through its own stdin pipe only
                printf '%s\n' "$key" | setsid -f bash "$0" _daemon --dir "$DIR" --build-id "$BID" --run-id "$RUN" --variant "$VARIANT" --host "$HOST" --image "$IMG" --src "$SRC" \
                  --runp "$RUNP" --heartbeat "$HB" --op-id "$OPID" --need "$NEED" --progress "$PROGRESS" $( [ "$NETNONE" = 1 ] && echo --network-none ) -- "${ARGV[@]}" >/dev/null 2>&1 9>&-
                python3 -I "$EVWAIT" waitfile "$DIR/daemon.pid" 10 || exit 5     # the daemon announces itself (an event, not a poll) before the stream attaches to it
              elif ! daemon_alive && ! grep -q '"kind":"completed"' "$DIR/journal.jsonl" 2>/dev/null && [ -s "$DIR/build.pid" ]; then
                # the emitter died with the build unfinished: an adopt daemon with the re-sent key finishes the job (T005a (i2)); the build is never started again
                rm -f "$DIR/daemon.pid"
                printf '%s\n' "$key" | setsid -f bash "$0" _adopt --dir "$DIR" --build-id "$BID" --run-id "$RUN" --variant "$VARIANT" --host "$HOST" --op-id "$OPID" --heartbeat "$HB" \
                  --progress "$PROGRESS" >/dev/null 2>&1 9>&-
                python3 -I "$EVWAIT" waitfile "$DIR/daemon.pid" 10 || exit 5
              fi ) 9>>"$DIR/.run.lock" || exit $?
            attach "$key";;
    _daemon) args_parse "$@" || exit 2; daemon;;
    _adopt)  args_parse "$@" || exit 2; adopt;;
    attach) args_parse "$@" || exit 2; local akey; read -r akey || exit 3; attach "$akey";;
    fetch)  args_parse "$@" || exit 2; tar -C "$DIR/out/artifacts" -cf - .;;
    cancel) args_parse "$@" || exit 2; cmd_cancel;;
    *) echo "usage: emit.sh run|attach|fetch|cancel ..." >&2; exit 2;;
  esac
}
if [ "${BASH_SOURCE[0]}" = "$0" ]; then main "$@"; fi
