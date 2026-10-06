#!/usr/bin/env bash
# emit.sh - T005b slice: the remote emitter (runs on the build host; for the proof, on this host through the local transport).
#   emit.sh run    --dir D --build-id B --run-id R --variant V --host H --image IMG-ID --src SRCDIR --runp RUN_PINNED.sh
#                  [--heartbeat S] [--op-id ID] [--need BYTES] [--network-none] -- <build argv...>      (key: first stdin line)
#   emit.sh attach --dir D --from N      stream the journal lines after the first N (resend from the last acknowledged seq)
#   emit.sh fetch  --dir D               tar of the build's artifacts/ tree on stdout
#   emit.sh cancel --dir D               stop the build container (by label) and the daemon, reaped by exact pid
# `run` starts a detached daemon (setsid) that wraps the build container (rootless, through run_pinned.sh), appends signed
# build-event/1 events to D/journal.jsonl (one JSON object per line, seq per build from 1) and exits; it then streams the
# journal like `attach`. The per-build key is read from the first stdin line, held in a shell variable only (never written
# on this host, never logged); the signature is the canonical form of contracts/build-event.schema.json (python json.dumps
# sort_keys, compact, ensure_ascii=False) so the hub's bev_crypto.py verifies it. Event timing is the host's monotonic clock
# (/proc/uptime) and elapsed_monotonic_ms in each heartbeat; no sent_at arithmetic is ever used by the driver.
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

emit_event() { # dir keyhex build run variant host kind [extra-json-object]   appends one signed event; seq allocated under a lock
  local dir=$1 key=$2 b=$3 r=$4 v=$5 h=$6 kind=$7 extra=${8:-} seq line
  [ -n "$extra" ] || extra='{}'
  exec 7>>"$dir/.seq.lock"; flock 7
  seq=$(( $(cat "$dir/seq" 2>/dev/null || echo 0) + 1 ))
  line=$(jq -nc --arg b "$b" --arg r "$r" --arg v "$v" --arg h "$h" --arg k "$kind" --argjson s "$seq" --arg t "$(isoz)" --argjson x "$extra" \
        '{schema:"build-event/1",run_id:$r,build_id:$b,variant:$v,seq:$s,kind:$k,host:$h,sent_at:$t} + $x' | sign_event "$key") || { flock -u 7; return 1; }
  printf '%s\n' "$line" >> "$dir/journal.jsonl" && printf '%s\n' "$seq" > "$dir/seq.tmp" && mv -f "$dir/seq.tmp" "$dir/seq"
  flock -u 7; exec 7>&-
}

manifest_sha() { # artifacts dir : sha256 of the sorted "sha256  relpath" lines (the content address of the artifact tree)
  ( cd "$1" 2>/dev/null && LC_ALL=C find . -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sed 's#  \./#  #' | sha256sum | cut -d' ' -f1 ) ; }

pid_start() { awk '{print $22}' "/proc/$1/stat" 2>/dev/null; }

args_parse() { # fills DIR BID RUN VARIANT HOST IMG SRC RUNP HB OPID NEED NETNONE ARGV
  DIR=; BID=; RUN=; VARIANT=primary; HOST=local; IMG=; SRC=; RUNP=; HB=2; OPID=; NEED=0; NETNONE=0; ARGV=(); FROM=0
  while [ $# -gt 0 ]; do
    case $1 in
      --dir) DIR=$2; shift 2;; --build-id) BID=$2; shift 2;; --run-id) RUN=$2; shift 2;; --variant) VARIANT=$2; shift 2;;
      --host) HOST=$2; shift 2;; --image) IMG=$2; shift 2;; --src) SRC=$2; shift 2;; --runp) RUNP=$2; shift 2;;
      --heartbeat) HB=$2; shift 2;; --op-id) OPID=$2; shift 2;; --need) NEED=$2; shift 2;; --network-none) NETNONE=1; shift;;
      --from) FROM=$2; shift 2;; --) shift; ARGV=("$@"); break;; *) echo "emit: unknown option $1" >&2; return 2;;
    esac
  done
  [ -n "$DIR" ] || { echo "emit: --dir required" >&2; return 2; }
}

daemon() { # runs detached; key = first stdin line
  local KEY bpid t0 po lines out ec manifest img_digest logsha
  read -r KEY || exit 3
  [[ $KEY =~ ^[0-9a-f]{64}$ ]] || exit 3
  mkdir -p "$DIR/out/artifacts"; : > "$DIR/build.log"
  printf '%s %s\n' "$$" "$(pid_start $$)" > "$DIR/daemon.pid"
  t0=$(mono_ms)
  cancelled=0
  stop() { # TERM: stop the container by its label (exact ids), then the build subshell by exact pid > 1
    cancelled=1
    local c; for c in $(podman ps -q --filter "label=catalogizer.op_id=$OPID" 2>/dev/null); do podman kill "$c" >/dev/null 2>&1; done
    if [ -n "${bpid:-}" ] && [ "$bpid" -gt 1 ] 2>/dev/null; then kill "$bpid" 2>/dev/null; fi
  }
  trap stop TERM
  emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" accepted
  ( cd "$SRC" && DISK_HEADROOM_OUT_DIR="$DIR/disk" "$RUNP" --out "$DIR/out" --op-id "$OPID" --need "$NEED" $( [ "$NETNONE" = 1 ] && echo --network=none ) "$IMG" -- "${ARGV[@]}" ) >> "$DIR/build.log" 2>&1 &
  bpid=$!
  rc=; local sp wp wrc
  while [ -z "$rc" ]; do
    # a heartbeat every HB seconds: a producer-side timer (the consumers never poll); `wait -n` returns at once when the build ends
    sleep "$HB" & sp=$!
    wp=; wait -n -p wp "$bpid" "$sp"; wrc=$?
    if [ "${wp:-}" = "$bpid" ]; then rc=$wrc; kill "$sp" 2>/dev/null; wait "$sp" 2>/dev/null; break; fi
    [ "${wp:-}" = "$sp" ] || { kill "$sp" 2>/dev/null; wait "$sp" 2>/dev/null; continue; }   # a trapped TERM interrupted the wait: loop again
    po=$(stat -c %s "$DIR/build.log" 2>/dev/null || echo 0); lines=$(wc -l < "$DIR/build.log" 2>/dev/null || echo 0)
    emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" heartbeat \
      "$(jq -nc --argjson p "${po:-0}" --argjson s "${lines:-0}" --argjson e "$(( $(mono_ms) - t0 ))" '{progress_offset:$p,stage:$s,elapsed_monotonic_ms:$e}')"
  done
  if [ "$cancelled" = 1 ]; then ec=cancelled
  elif [ "$rc" = 0 ]; then ec=succeeded
  elif grep -q 'run_pinned: REFUSED' "$DIR/build.log" 2>/dev/null || [ "$rc" -ge 125 ]; then ec=infra_failed
  else ec=build_failed; fi
  manifest=$(manifest_sha "$DIR/out/artifacts"); img_digest="sha256:$manifest"
  logsha=$(sha256sum "$DIR/build.log" | cut -d' ' -f1)
  emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" completed \
    "$(jq -nc --arg e "$ec" --arg m "$manifest" --arg i "$img_digest" --arg l "$logsha" '{exit_class:$e,artifact_manifest_sha256:$m,image_digest:$i,remote_log_sha256:$l}')"
  exit 0
}

attach() { # stream the journal after FROM lines, blocking on inotify; ends at the completed line or when the daemon is gone
  local p="" s
  if [ -s "$DIR/daemon.pid" ]; then read -r p s < "$DIR/daemon.pid"; fi
  if [ -n "$p" ] && [ "$(pid_start "$p")" = "${s:-x}" ]; then exec python3 -I "$EVWAIT" tail "$DIR/journal.jsonl" "$FROM" "$p" "emit.sh"; fi
  touch "$DIR/journal.jsonl" 2>/dev/null
  exec python3 -I "$EVWAIT" tail "$DIR/journal.jsonl" "$FROM"
}

cmd_cancel() {
  local p="" s c
  if [ -s "$DIR/daemon.pid" ]; then read -r p s < "$DIR/daemon.pid"; fi
  # exact pid > 1, proven ours by start time and by /proc cmdline (never a pattern kill, never a process-group signal)
  if [ -n "$p" ] && [ "$p" -gt 1 ] 2>/dev/null && [ "$(pid_start "$p")" = "$s" ] && tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q 'emit.sh'; then kill -TERM "$p"; echo "cancel sent"; else echo "no live daemon"; fi
}

main() {
  local sub=${1:-}; shift || true
  case $sub in
    run)    args_parse "$@" || exit 2; [ -n "$BID" ] && [ -n "$RUN" ] && [ -n "$SRC" ] && [ -n "$RUNP" ] && [ -n "$IMG" ] || exit 2
            [ -n "$OPID" ] || OPID="emit-$BID"
            mkdir -p "$DIR" || exit 2
            local key; read -r key || exit 3
            # idempotent: when a journal or a live daemon already exists the build is NOT started again, the journal is only streamed (resume after a hub restart)
            if [ ! -s "$DIR/journal.jsonl" ] && [ ! -s "$DIR/daemon.pid" ]; then
            # the daemon inherits the key through its own stdin pipe only
            printf '%s\n' "$key" | setsid -f bash "$0" _daemon --dir "$DIR" --build-id "$BID" --run-id "$RUN" --variant "$VARIANT" --host "$HOST" --image "$IMG" --src "$SRC" \
              --runp "$RUNP" --heartbeat "$HB" --op-id "$OPID" --need "$NEED" $( [ "$NETNONE" = 1 ] && echo --network-none ) -- "${ARGV[@]}" >/dev/null 2>&1
              python3 -I "$EVWAIT" waitfile "$DIR/daemon.pid" 10 || exit 5     # the daemon announces itself (an event, not a poll) before the stream attaches to it
            fi
            attach;;
    _daemon) args_parse "$@" || exit 2; daemon;;
    attach) args_parse "$@" || exit 2; attach;;
    fetch)  args_parse "$@" || exit 2; tar -C "$DIR/out/artifacts" -cf - .;;
    cancel) args_parse "$@" || exit 2; cmd_cancel;;
    *) echo "usage: emit.sh run|attach|fetch|cancel ..." >&2; exit 2;;
  esac
}
if [ "${BASH_SOURCE[0]}" = "$0" ]; then main "$@"; fi
