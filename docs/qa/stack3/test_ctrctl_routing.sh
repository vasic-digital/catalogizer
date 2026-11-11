#!/usr/bin/env bash
# CAT-QA-STACK-LIFECYCLE-1: stack3 up.sh / sonarqube/up.sh must START containers through qa_stack_start (ctrctl over the Containers submodule), never `podman run -d`.
# (1) static guard: no raw `podman run -d` / `podman start` in the migrated scripts. (2) behaviour: a shim records calls; every container gets `podman create` then
# a start via the helper (shim ctrctl via QA_STACK_BACKEND=containers is NOT used: no go build here; the podman fallback is forced and the helper's name guard is exercised).
# usage: test_ctrctl_routing.sh [--root DIR]   (--root runs against a mutated copy of docs/qa/stack3; used by test_ctrctl_routing_mutations.sh)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ "${1:-}" = --root ] && HERE="${2:?}"
pass=0; fail=0
ok() { if [ "$1" = 0 ]; then pass=$((pass+1)); echo "PASS $2"; else fail=$((fail+1)); echo "FAIL $2"; fi; }
for f in up.sh sonarqube/up.sh; do
  grep -nE '^[^#]*\bpodman (run -d|run --detach|start)\b' "$HERE/$f" >/dev/null; [ $? -ne 0 ]; ok $? "static: $f has no raw podman run -d / podman start"
  grep -q 'ctr_up ' "$HERE/$f"; ok $? "static: $f starts containers via ctr_up"
done
for f in lib.sh sonarqube/lib.sh; do
  grep -q 'qa_stack_start "\$n"' "$HERE/$f"; ok $? "static: $f ctr_up delegates the start to qa_stack_start"
  grep -nE '^[^#]*\bpodman (run|start)\b' "$HERE/$f" >/dev/null; [ $? -ne 0 ]; ok $? "static: $f ctr_up has no raw podman run/start"
done
# behaviour: source lib.sh with a podman shim, call ctr_up, assert create + (helper-routed) start, and that a start failure propagates.
ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ctrroute.XXXXXX")"; : "${ROOT:?}"; trap 'rm -rf -- "${ROOT:?}"' EXIT
mkdir -p "$ROOT/bin"
cat > "$ROOT/bin/podman" <<'S'
#!/usr/bin/env bash
echo "podman $*" >> "$SHIM_LOG"
case "$1" in
  create) exit "${SHIM_CREATE_RC:-0}";;
  start) exit "${SHIM_START_RC:-0}";;
  inspect) case "$*" in *Health*) echo healthy;; *) echo running;; esac;;
esac
exit 0
S
chmod +x "$ROOT/bin/podman"
for lib in lib.sh sonarqube/lib.sh; do
  case "$lib" in lib.sh) pfx=catqa3;; *) pfx=catsonar;; esac
  for mode in ok startfail; do
    export SHIM_LOG="$ROOT/log.$pfx.$mode"; : > "$SHIM_LOG"; unset SHIM_START_RC; [ "$mode" = startfail ] && export SHIM_START_RC=1
    ( export PATH="$ROOT/bin:$PATH" QA_STACK_BACKEND=podman QA_DIR="$ROOT/qa" SQ_DIR="$ROOT/sq" HOME="$HOME"; . "$HERE/$lib" 2>/dev/null; ctr_up "$pfx-api" --memory 1g img >/dev/null 2>&1 ); rc=$?
    if [ "$mode" = ok ]; then
      [ "$rc" = 0 ] && grep -q "^podman create --pod $pfx --name $pfx-api --memory 1g img" "$SHIM_LOG" && grep -q "^podman start $pfx-api" "$SHIM_LOG" && ! grep -qE '^podman run' "$SHIM_LOG"; ok $? "behaviour[$lib]: create then helper start, no podman run"
    else
      [ "$rc" != 0 ] && grep -q "^podman logs --tail 20 $pfx-api" "$SHIM_LOG"; ok $? "behaviour[$lib]: start failure propagates (rc=$rc) and logs are shown"
    fi
  done
  export SHIM_LOG="$ROOT/log.$pfx.foreign"; : > "$SHIM_LOG"; unset SHIM_START_RC
  ( export PATH="$ROOT/bin:$PATH" QA_STACK_BACKEND=podman QA_DIR="$ROOT/qa" SQ_DIR="$ROOT/sq"; . "$HERE/$lib" 2>/dev/null; ctr_up "otherstack-api" img >/dev/null 2>&1 ); rc=$?
  [ "$rc" != 0 ] && ! grep -q '^podman start' "$SHIM_LOG"; ok $? "behaviour[$lib]: name outside the stack prefix is refused by the helper (nothing started)"
done
echo "pass=$pass fail=$fail"; [ "$fail" = 0 ]
