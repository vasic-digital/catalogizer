#!/usr/bin/env bash
# End-to-end statistical scan of the exec-window orphan (WF3-REVIEW R1): the gate signals ITSELF right after launching its bounded probe
# (the window_suite technique), the child's exec time is shifted by padding PATH with nonexistent directories, and 0.6 s after the gate exits the
# test checks through /proc whether the hanging podman stub is still alive. Usage: e2e_scan.sh <script> <trials per padding> <pad...>
# Only the stub pid recorded by the stub itself is ever signalled (cleanup).
set -u
SCRIPT="$1"; TRIALS="$2"; shift 2
T="$(mktemp -d "${TMPDIR:-/tmp}/e2escan.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
mkdir -p "$T/shims" "$T/root"
cat >"$T/shims/podman" <<'STUB'
#!/usr/bin/env bash
echo $$ >"$SIG_DIR/stub.pid"
exec sleep 30
STUB
cat >"$T/shims/df" <<'STUB'
#!/usr/bin/env bash
echo Avail; echo 5000
STUB
chmod +x "$T/shims/"*
printf 'min_free_bytes=1\n' >"$T/conf"
P="$T/probe.sh"
sed -E '/# MUT:pid-assign$/i\  [ "${1:-}" = "${PROBE_CMD:-}" ] \&\& kill -TERM $$' "$SCRIPT" >"$P"
cmp -s "$P" "$SCRIPT" && { echo "probe copy changed nothing"; exit 2; }
alive() { [ -n "$1" ] && [ -r "/proc/$1/stat" ] && [ "$(sed -E 's/^[0-9]+ \(.*\) ([A-Za-z]) .*/\1/' "/proc/$1/stat" 2>/dev/null)" != Z ]; }
for npad in "$@"; do
  pad=""; i=0; while [ "$i" -lt "$npad" ]; do pad="$pad/nonexistent/d$i:"; i=$((i+1)); done
  orphans=0; n=0
  while [ "$n" -lt "$TRIALS" ]; do
    n=$((n+1)); rm -f "$T/stub.pid"
    env -u EVREC_TURN_RUN_ID PATH="${pad}$T/shims:$PATH" SIG_DIR="$T" PROBE_CMD=podman SHIM_GRAPHROOT=/ DISK_HEADROOM_REPO_ROOT="$T/root" EV="$T/ev" \
      DISK_HEADROOM_CONF="$T/conf" python3 -c 'import os,signal,sys
for s in (signal.SIGINT,signal.SIGTERM,signal.SIGHUP): signal.signal(s,signal.SIG_DFL)
os.execvp("bash",["bash"]+sys.argv[1:])' "$P" --need 1 --op-id e2e >"$T/out" 2>"$T/err" &
    GP=$!; wait "$GP" 2>/dev/null
    sleep 0.6
    sp="$(cat "$T/stub.pid" 2>/dev/null)"
    if [ -n "$sp" ] && alive "$sp"; then orphans=$((orphans+1)); kill -KILL "$sp" 2>/dev/null; fi
  done
  echo "script=$SCRIPT npad=$npad trials=$TRIALS orphans=$orphans"
done
