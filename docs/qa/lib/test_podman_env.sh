#!/usr/bin/env bash
# GATE-LOGIC UNIT TEST (constitution 11.4.289(C)(2)): proves podman_env.sh decides/normalises correctly against a FAKE podman shim on PATH.
# It is NOT evidence that real containers start; the live proof is docs/qa/stack3/evidence-lifecycle-20261111/.
# The shim reproduces the measured failure signature: any `podman start` fails (rc 126, "OCI permission denied") when
# DBUS_SESSION_BUS_ADDRESS contains ",guid=". usage: [HELPER=path] test_podman_env.sh ; exit 0 = all pass. HOME never reassigned.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPER="${HELPER:-$HERE/podman_env.sh}"
ROOT="$(mktemp -d "${TMPDIR:-/tmp}/podman-env-test.XXXXXX")"; : "${ROOT:?}"
trap 'rm -rf -- "${ROOT:?}"' EXIT
BIN="$ROOT/bin"; mkdir -p "$BIN" "$ROOT/state"
pass=0; fail=0
ok() { if [ "$1" = 0 ]; then pass=$((pass+1)); echo "PASS $2"; else fail=$((fail+1)); echo "FAIL $2"; fi; }
cat > "$BIN/podman" <<'S'
#!/usr/bin/env bash
echo "podman $*" >> "$SHIM_LOG"
n="${2-}"
case "$1" in
  start) case "${DBUS_SESSION_BUS_ADDRESS-}" in *,guid=*) echo "crun: sd-bus call: Operation not permitted: Permission denied: OCI permission denied" >&2; exit 126;; esac
         echo running > "$SHIM_STATE/$n"; exit 0;;
  stop)  n="${4-$n}"; echo exited > "$SHIM_STATE/$n"; exit 0;;
  inspect) [ -f "$SHIM_STATE/$n" ] || exit 125
    case "$*" in
      *Health*) c="$SHIM_STATE/$n.polls"; k=$(( $(cat "$c" 2>/dev/null || echo 0) + 1 )); echo $k > "$c"
                if [ "${SHIM_NEVER_HEALTHY:-0}" = 1 ] || [ "$k" -le "${SHIM_STARTING_POLLS:-0}" ]; then echo starting; else echo healthy; fi;;
      *) cat "$SHIM_STATE/$n";;
    esac;;
esac
exit 0
S
printf '#!/usr/bin/env bash\nexit 0\n' > "$BIN/sleep"; chmod +x "$BIN"/*
export SHIM_LOG="$ROOT/log" SHIM_STATE="$ROOT/state" QA_STACK_BACKEND=podman STACK=catqa3 PATH="$BIN:$PATH"
G='unix:path=/run/user/1000/bus,guid=0123abcd'
fresh() { : > "$SHIM_LOG"; rm -f "$SHIM_STATE"/*; echo exited > "$SHIM_STATE/catqa3-api"; unset SHIM_NEVER_HEALTHY SHIM_STARTING_POLLS; }

# 1. RED signature (shim sanity): raw podman start with the guid address fails with the measured message; without guid it works.
fresh; out="$(DBUS_SESSION_BUS_ADDRESS="$G" podman start catqa3-api 2>&1)"; rc=$?
[ "$rc" = 126 ] && [[ "$out" == *"OCI permission denied"* ]]; ok $? "RED: raw podman start with ,guid= address fails (rc=$rc)"
fresh; DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/1000/bus" podman start catqa3-api >/dev/null 2>&1; ok $? "shim: guid-less address starts"

# 2. normalisation
. "$HELPER" || { echo "cannot source $HELPER"; exit 2; }
DBUS_SESSION_BUS_ADDRESS="$G"; qa_podman_env_normalize; [ "$DBUS_SESSION_BUS_ADDRESS" = "unix:path=/run/user/1000/bus" ]; ok $? "strips ,guid=: '$DBUS_SESSION_BUS_ADDRESS'"
DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/1000/bus,guid=ab,foo=bar"; qa_podman_env_normalize; [ "$DBUS_SESSION_BUS_ADDRESS" = "unix:path=/run/user/1000/bus" ]; ok $? "strips ALL ,key=val params: '$DBUS_SESSION_BUS_ADDRESS'"
DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/1000/bus"; qa_podman_env_normalize; [ "$DBUS_SESSION_BUS_ADDRESS" = "unix:path=/run/user/1000/bus" ]; ok $? "clean address unchanged (idempotent)"
unset DBUS_SESSION_BUS_ADDRESS XDG_RUNTIME_DIR; qa_podman_env_normalize; [ -z "${DBUS_SESSION_BUS_ADDRESS+x}" ] && [ "$XDG_RUNTIME_DIR" = "/run/user/$(id -u)" ]; ok $? "unset DBUS stays unset; XDG_RUNTIME_DIR defaulted"

# 3. GREEN: wrapper start works with the guid-bearing address exported, and waits for health
fresh; export DBUS_SESSION_BUS_ADDRESS="$G" SHIM_STARTING_POLLS=3
qa_stack_start catqa3-api 2>"$ROOT/err"; rc=$?
[ "$rc" = 0 ] && [ "$(cat "$SHIM_STATE/catqa3-api")" = running ]; ok $? "GREEN: qa_stack_start succeeds with guid address (rc=$rc)"
[ "$(grep -c 'Health' "$SHIM_LOG")" -ge 4 ]; ok $? "start waited for health (>=4 health polls, got $(grep -c 'Health' "$SHIM_LOG"))"
# 4. health never arrives -> bounded failure
fresh; export SHIM_NEVER_HEALTHY=1; QA_HEALTH_TIMEOUT=3 qa_stack_start catqa3-api 2>/dev/null; rc=$?
[ "$rc" = 1 ]; ok $? "never healthy -> rc 1 after bounded wait (rc=$rc)"
# 5. other stacks / bad names are refused and podman is never called
fresh; bad=0
for n in catqa1-api catqa2-api cattsrv-x catqa3api 'catqa3-a;b' '' '../catqa3-api'; do
  qa_stack_start "$n" 2>/dev/null; r1=$?; qa_stack_stop "$n" 2>/dev/null; r2=$?
  { [ "$r1" = 64 ] && [ "$r2" = 64 ]; } || { echo "  '$n' not refused ($r1,$r2)"; bad=1; }
done
[ ! -s "$SHIM_LOG" ] || { echo "  podman called for refused names"; bad=1; }; ok $bad "other stacks and malformed names refused, podman never invoked"
# 6. stop; and only the target container is addressed
fresh; echo running > "$SHIM_STATE/catqa3-web"; qa_stack_stop catqa3-api 2>/dev/null; rc=$?
[ "$rc" = 0 ] && [ "$(cat "$SHIM_STATE/catqa3-api")" = exited ] && [ "$(cat "$SHIM_STATE/catqa3-web")" = running ] && ! grep -qE 'prune|pod |rm ' "$SHIM_LOG"; ok $? "qa_stack_stop stops only the target, no prune/pod/rm"
# 7. custom prefix (probe container)
fresh; echo exited > "$SHIM_STATE/catqa-lifecycle-probe"; QA_STACK_PREFIX=catqa-lifecycle- qa_stack_start catqa-lifecycle-probe 2>/dev/null; ok $? "QA_STACK_PREFIX allows the probe container"
echo "passed=$pass failed=$fail"; [ "$fail" = 0 ]
