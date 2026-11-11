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
         echo "${SHIM_AFTER_START:-running}" > "$SHIM_STATE/$n"; exit 0;;
  stop)  n="${4-$n}"; echo exited > "$SHIM_STATE/$n"; exit 0;;
  inspect) [ -n "${SHIM_INSPECT_HANG-}" ] && exec /bin/sleep "$SHIM_INSPECT_HANG"
    [ -n "${SHIM_INSPECT_DELAY-}" ] && /bin/sleep "$SHIM_INSPECT_DELAY"; [ -f "$SHIM_STATE/$n" ] || exit 125
    case "$*" in
      *Health*) [ -n "${SHIM_HEALTH_HANG-}" ] && exec /bin/sleep "$SHIM_HEALTH_HANG"
                c="$SHIM_STATE/$n.polls"; k=$(( $(cat "$c" 2>/dev/null || echo 0) + 1 )); echo $k > "$c"
                [ "$k" -le "${SHIM_HEALTH_FAIL_POLLS:-0}" ] && exit 1
                if [ -n "${SHIM_HEALTH_EMPTY-}" ]; then :; elif [ -n "${SHIM_HEALTH_OUT-}" ]; then echo "$SHIM_HEALTH_OUT"; elif [ "${SHIM_NEVER_HEALTHY:-0}" = 1 ] || [ "$k" -le "${SHIM_STARTING_POLLS:-0}" ]; then echo starting; else echo healthy; fi;;
      *) cat "$SHIM_STATE/$n";;
    esac;;
esac
exit 0
S
printf '#!/usr/bin/env bash\nexit 0\n' > "$BIN/sleep"; chmod +x "$BIN"/*
export SHIM_LOG="$ROOT/log" SHIM_STATE="$ROOT/state" QA_STACK_BACKEND=podman STACK=catqa3 PATH="$BIN:$PATH"
G='unix:path=/run/user/1000/bus,guid=0123abcd'
fresh() { : > "$SHIM_LOG"; rm -f "$SHIM_STATE"/*; echo exited > "$SHIM_STATE/catqa3-api"; unset SHIM_INSPECT_HANG SHIM_HEALTH_HANG SHIM_CTRCTL_FAIL SHIM_NEVER_HEALTHY SHIM_STARTING_POLLS SHIM_HEALTH_OUT SHIM_AFTER_START SHIM_HEALTH_FAIL_POLLS SHIM_HEALTH_EMPTY; }

# 1. RED signature (shim sanity): raw podman start with the guid address fails with the measured message; without guid it works.
fresh; out="$(DBUS_SESSION_BUS_ADDRESS="$G" podman start catqa3-api 2>&1)"; rc=$?
[ "$rc" = 126 ] && [[ "$out" == *"OCI permission denied"* ]]; ok $? "RED: raw podman start with ,guid= address fails (rc=$rc)"
fresh; DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/1000/bus" podman start catqa3-api >/dev/null 2>&1; ok $? "shim: guid-less address starts"

# 2. normalisation
. "$HELPER" || { echo "cannot source $HELPER"; exit 2; }
DBUS_SESSION_BUS_ADDRESS="$G"; qa_podman_env_normalize; [ "$DBUS_SESSION_BUS_ADDRESS" = "unix:path=/run/user/1000/bus" ]; ok $? "strips ,guid=: '$DBUS_SESSION_BUS_ADDRESS'"
for pair in 'unix:guid=abc,path=/x|unix:path=/x' 'tcp:host=1.2.3.4,port=55,guid=ab|tcp:host=1.2.3.4,port=55' 'unix:path=/a,guid=1;unix:path=/b,guid=2|unix:path=/a;unix:path=/b' 'unix:path=/run/user/1000/bus,guid=ab,foo=bar|unix:path=/run/user/1000/bus,foo=bar'; do
  DBUS_SESSION_BUS_ADDRESS="${pair%%|*}"; qa_podman_env_normalize; [ "$DBUS_SESSION_BUS_ADDRESS" = "${pair##*|}" ]; ok $? "removes only guid= (keeps other params): '${pair%%|*}' -> '$DBUS_SESSION_BUS_ADDRESS'"
done
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

# 8. health 'none' (no healthcheck) counts as ok; exited/stopped/dead after start fail FAST (no full wait)
fresh; export SHIM_HEALTH_OUT=none; qa_stack_start catqa3-api 2>/dev/null; ok $? "health 'none' -> start ok"
for st in exited stopped dead; do
  fresh; export SHIM_AFTER_START=$st; QA_HEALTH_TIMEOUT=50 qa_stack_start catqa3-api 2>/dev/null; rc=$?
  [ "$rc" = 1 ] && [ "$(grep -c Health "$SHIM_LOG")" -le 1 ]; ok $? "state '$st' after start -> fail fast rc 1 (health polls=$(grep -c Health "$SHIM_LOG"))"
done
# 8b. wall-clock bound: slow inspect calls must not stretch the wait beyond ~QA_HEALTH_TIMEOUT (+ one poll)
fresh; export SHIM_NEVER_HEALTHY=1 SHIM_INSPECT_DELAY=2; t0=$SECONDS; QA_HEALTH_TIMEOUT=3 qa_stack_start catqa3-api 2>/dev/null; rc=$?; el=$((SECONDS-t0)); unset SHIM_INSPECT_DELAY
[ "$rc" = 1 ] && [ "$el" -le 8 ]; ok $? "wall-clock bound holds with slow inspect (rc=$rc elapsed=${el}s <= 8)"
# 9. backend selection against a FAKE repo root + fake `go` (no real build)
FR="$ROOT/fakerepo"; mkdir -p "$FR/docs/qa/lib/ctrctl" "$FR/submodules/containers/pkg/runtime"; : > "$FR/submodules/containers/pkg/runtime/podman.go"
echo 'package main // v1' > "$FR/docs/qa/lib/ctrctl/main.go"; echo 'module x' > "$FR/docs/qa/lib/ctrctl/go.mod"
cat > "$BIN/go" <<'S'
#!/usr/bin/env bash
# fake go build -o OUT: refuses when main.go says BROKEN, else writes a "binary" embedding the content hash of main.go+go.mod
echo "go $*" >> "$SHIM_LOG"; out=""; while [ $# -gt 0 ]; do [ "$1" = -o ] && out="$2"; shift; done
grep -q BROKEN main.go && { [ -n "${SHIM_GO_PARTIAL-}" ] && echo partial > "$out"; echo "build error" >&2; exit 2; }
echo "GOOUT $out" >> "$SHIM_LOG"
case "$out" in *.tmp.*) ;; *) echo "fake go: refusing direct write to final path $out" >&2; exit 3;; esac
h="$(cat main.go go.mod | sha256sum | cut -c1-16)"
printf '#!/usr/bin/env bash\n[ -n "${SHIM_CTRCTL_FAIL-}" ] && exit "${SHIM_CTRCTL_FAIL}"\necho "ctrctl-bin %s $*" >> "$SHIM_LOG"\nprintf "%s" > "$SHIM_STATE/$2"\n' "$h" "${SHIM_AFTER_START:-running}" > "$out"; chmod +x "$out"
echo "TMPINODE $(stat -c %i "$out")" >> "$SHIM_LOG"
S
chmod +x "$BIN/go"
export QA_REPO_ROOT="$FR" QA_CTRCTL_CACHE="$ROOT/cache" QA_STACK_BACKEND=auto
fresh; qa_stack_start catqa3-api 2>"$ROOT/err"; rc=$?
b1="$(grep -o 'ctrctl-bin [0-9a-f]*' "$SHIM_LOG" | head -1)"
[ "$rc" = 0 ] && [ -n "$b1" ] && grep -q 'backend=containers ctrctl_sha256=' "$ROOT/err"; ok $? "auto backend builds+uses ctrctl and logs its sha256 (rc=$rc)"
echo 'package main // v2 changed' > "$FR/docs/qa/lib/ctrctl/main.go"
fresh; qa_stack_start catqa3-api 2>/dev/null; b2="$(grep -o 'ctrctl-bin [0-9a-f]*' "$SHIM_LOG" | head -1)"
[ -n "$b2" ] && [ "$b1" != "$b2" ]; ok $? "changed main.go -> NEW binary used, not the stale cache ($b1 vs $b2)"
echo 'module x // changed' > "$FR/docs/qa/lib/ctrctl/go.mod"; fresh; qa_stack_start catqa3-api 2>/dev/null; b3="$(grep -o 'ctrctl-bin [0-9a-f]*' "$SHIM_LOG" | head -1)"
[ "$b3" != "$b2" ]; ok $? "changed go.mod -> new binary"
ls "$ROOT/cache" >/dev/null 2>&1 && [ "$(qa_stack_backend_info)" != "backend=podman" ]; ok $? "qa_stack_backend_info names backend + sha256: $(qa_stack_backend_info | cut -c1-60)"
# forced containers + broken build -> none, rc 69, NO podman fallback
echo 'package main // BROKEN' > "$FR/docs/qa/lib/ctrctl/main.go"
fresh; QA_STACK_BACKEND=containers qa_stack_start catqa3-api 2>"$ROOT/err"; rc=$?
[ "$rc" = 69 ] && ! grep -q '^podman start' "$SHIM_LOG" && grep -q 'no fallback to podman' "$ROOT/err"; ok $? "forced containers + broken build -> rc 69, no podman fallback (rc=$rc)"
fresh; QA_STACK_BACKEND=containers qa_stack_stop catqa3-api 2>/dev/null; rc=$?; [ "$rc" = 69 ] && ! grep -q '^podman stop' "$SHIM_LOG"; ok $? "forced containers stop + broken build -> rc 69, no fallback"
fresh; qa_stack_start catqa3-api 2>"$ROOT/err"; rc=$?; [ "$rc" = 0 ] && grep -q 'DEVIATION' "$ROOT/err"; ok $? "auto + broken build -> logged podman fallback (rc=$rc)"
# ---- LIFE-FU-1: atomic ctrctl build (tmp in cache dir + mv -f), concurrent builds, TMPDIR irrelevant
fresh; rm -rf "${ROOT:?}/cache"; echo 'package main // v3' > "$FR/docs/qa/lib/ctrctl/main.go"
export QA_STACK_BACKEND=auto
qa_stack_start catqa3-api 2>/dev/null; rc=$?
o="$(grep -m1 '^GOOUT ' "$SHIM_LOG" | cut -d' ' -f2)"
[ "$rc" = 0 ] && [ "$(dirname "$o")" = "$ROOT/cache" ] && [ "$o" != "$ROOT/cache/ctrctl" ] && [ -x "$ROOT/cache/ctrctl" ] && [ -z "$(ls "$ROOT/cache" | grep '\.tmp\.')" ]; ok $? "build goes to tmp file in cache dir then mv -f; no tmp left (out=$(basename "$o"))"
fresh; rm -rf "${ROOT:?}/cache"; mkdir -p "$ROOT/othertmp"
( TMPDIR="$ROOT/othertmp"; for i in 1 2 3 4 5 6; do ( _qa_stack_backend > "$ROOT/conc.$i" 2>/dev/null ) & done; wait )
good=0; for i in 1 2 3 4 5 6; do grep -q "^containers $ROOT/cache/ctrctl$" "$ROOT/conc.$i" && good=$((good+1)); done
tot="$(grep -c '^GOOUT ' "$SHIM_LOG")"; uq="$(grep '^GOOUT ' "$SHIM_LOG" | sort -u | wc -l)"
[ "$good" = 6 ] && [ "$tot" = 6 ] && [ "$uq" = 6 ] && [ -x "$ROOT/cache/ctrctl" ] && [ -z "$(ls "$ROOT/cache" | grep '\.tmp\.')" ]; ok $? "6 concurrent first builds (TMPDIR elsewhere): all ok, 6 distinct tmp outputs, no tmp left (good=$good tot=$tot uniq=$uq)"
# NOTE: a true cross-filesystem text-file-busy reproduction needs a real ELF binary being executed; the fake go writes scripts, so that
# exact kernel behaviour is a documented skip. The contract tested here is the cause: the final path is only ever written by atomic rename.

# ---- LIFE-FU-2a: inspect bound is enforced (QA_INSPECT_TIMEOUT, default 10)
fresh; rm -rf "${ROOT:?}/cache"; export QA_STACK_BACKEND=podman SHIM_INSPECT_HANG=6
t0=$SECONDS; QA_INSPECT_TIMEOUT=1 QA_HEALTH_TIMEOUT=2 qa_stack_start catqa3-api 2>/dev/null; rc=$?; el=$((SECONDS-t0)); unset SHIM_INSPECT_HANG
[ "$rc" = 1 ] && [ "$el" -le 4 ]; ok $? "hung inspect is cut off by the timeout bound (rc=$rc elapsed=${el}s <= 4)"

# ---- LIFE-FU-2b: per-checkout cache keying without QA_CTRCTL_CACHE
FR2="$ROOT/fakerepo2"; mkdir -p "$FR2/docs/qa/lib/ctrctl" "$FR2/submodules/containers/pkg/runtime"; : > "$FR2/submodules/containers/pkg/runtime/podman.go"
echo 'package main // other checkout' > "$FR2/docs/qa/lib/ctrctl/main.go"; echo 'module y' > "$FR2/docs/qa/lib/ctrctl/go.mod"
fresh; export QA_STACK_BACKEND=auto
ba="$(unset QA_CTRCTL_CACHE; XDG_CACHE_HOME="$ROOT/xdg" QA_REPO_ROOT="$FR" _qa_stack_backend 2>/dev/null)"
bb="$(unset QA_CTRCTL_CACHE; XDG_CACHE_HOME="$ROOT/xdg" QA_REPO_ROOT="$FR2" _qa_stack_backend 2>/dev/null)"
da="$(dirname "${ba#containers }")"; db="$(dirname "${bb#containers }")"
[ "$da" != "$db" ] && [ "$(dirname "$da")" = "$ROOT/xdg/catalogizer-qa" ] && [ "$(dirname "$db")" = "$ROOT/xdg/catalogizer-qa" ] && [ -x "${ba#containers }" ] && [ -x "${bb#containers }" ] && ! cmp -s "${ba#containers }" "${bb#containers }"; ok $? "two checkouts get distinct cache dirs under XDG_CACHE_HOME and distinct binaries"

# ---- LIFE-FU-2c: ctrctl non-zero exit propagates
fresh; export QA_STACK_BACKEND=auto QA_CTRCTL_CACHE="$ROOT/cache"; rm -rf "${ROOT:?}/cache"; echo 'package main // v4' > "$FR/docs/qa/lib/ctrctl/main.go"
export SHIM_CTRCTL_FAIL=1
qa_stack_stop catqa3-api 2>/dev/null; r=$?; [ "$r" = 1 ]; ok $? "failing ctrctl stop -> qa_stack_stop returns 1 (rc=$r)"
echo running > "$SHIM_STATE/catqa3-api"   # container looks healthy: only the ctrctl exit code can fail the start
qa_stack_start catqa3-api 2>/dev/null; r=$?; [ "$r" = 1 ]; ok $? "failing ctrctl start -> qa_stack_start returns 1 (rc=$r)"
unset SHIM_CTRCTL_FAIL
# rc must not leak as a global
unset rc; fresh; qa_stack_stop catqa3-api 2>/dev/null; qa_stack_start catqa3-api 2>/dev/null; [ -z "${rc+x}" ]; ok $? "qa_stack_start/stop keep rc local (no global leak)"

# ---- LIFE-FU-4: glob safety in _qa_strip_guid, DEVIATION wording, missing timeout message
GD="$ROOT/globdir"; mkdir -p "$GD"; : > "$GD/abc"
out="$(cd "$GD" && _qa_strip_guid 'unix:path=/x,a*,guid=1')"
[ "$out" = 'unix:path=/x,a*' ]; ok $? "glob-looking param is not expanded against cwd files: '$out'"
( cd "$GD" && _qa_strip_guid 'unix:path=/x,a*,guid=1' >/dev/null; case $- in *f*) exit 1;; esac ); ok $? "set -f does not leak into the caller shell"
fresh; export QA_STACK_BACKEND=podman; qa_stack_stop catqa3-api 2>"$ROOT/err"; grep -q 'FORCED' "$ROOT/err" && ! grep -q 'unavailable' "$ROOT/err"; ok $? "forced podman logs FORCED wording, not 'unavailable'"
fresh; export QA_STACK_BACKEND=auto QA_REPO_ROOT="$ROOT/norepo"; qa_stack_stop catqa3-api 2>"$ROOT/err"; grep -q 'unavailable' "$ROOT/err" && ! grep -q 'FORCED' "$ROOT/err"; ok $? "auto fallback logs 'unavailable' wording"
export QA_REPO_ROOT="$FR"
NB="$ROOT/nobin"; mkdir -p "$NB"; ln -s "$BIN/podman" "$NB/podman"; ln -s "$BIN/sleep" "$NB/sleep"
fresh; out="$( PATH="$NB"; qa_stack_wait_healthy catqa3-api 2 2>&1 )"; rc=$?
[ "$rc" = 69 ] && [[ "$out" == *"'timeout' command not found"* ]]; ok $? "missing timeout -> clear error, rc 69 (rc=$rc)"

# ---- LIFE-FU-5: the final path is renamed into place (same inode as the tmp file), not copied
fresh; rm -rf "${ROOT:?}/cache"; export QA_STACK_BACKEND=auto QA_CTRCTL_CACHE="$ROOT/cache" QA_REPO_ROOT="$FR"; echo 'package main // v5' > "$FR/docs/qa/lib/ctrctl/main.go"
qa_stack_start catqa3-api 2>/dev/null; ti="$(grep -m1 '^TMPINODE ' "$SHIM_LOG" | cut -d' ' -f2)"; fi_="$(stat -c %i "$ROOT/cache/ctrctl" 2>/dev/null)"
[ -n "$ti" ] && [ "$ti" = "$fi_" ]; ok $? "final ctrctl has the tmp file's inode (atomic rename, not copy+rm): tmp=$ti final=$fi_"

# ---- LIFE-FU-6: only the Health inspect hangs; it must be bounded and must NOT be read as healthy
fresh; rm -rf "${ROOT:?}/cache"; export QA_STACK_BACKEND=podman SHIM_HEALTH_HANG=30
t0=$SECONDS; QA_INSPECT_TIMEOUT=1 QA_HEALTH_TIMEOUT=3 qa_stack_start catqa3-api 2>/dev/null; rc=$?; el=$((SECONDS-t0)); unset SHIM_HEALTH_HANG
[ "$rc" = 1 ] && [ "$el" -le 6 ]; ok $? "hung Health inspect is cut off and not treated as healthy (rc=$rc elapsed=${el}s <= 6, hang 30s)"

# ---- LIFE-FU-7: set -f already on in the caller stays on
out="$(set -f; _qa_strip_guid 'unix:path=/x,a*,guid=1' >/dev/null; case $- in *f*) echo on;; *) echo off;; esac)"
[ "$out" = on ]; ok $? "caller's pre-existing set -f is preserved by _qa_strip_guid ($out)"

# ---- LIFE-FU-8: missing timeout is pre-flighted in qa_stack_start: rc 69 and the container is NOT started
fresh; export QA_STACK_BACKEND=podman
out="$( PATH="$NB"; qa_stack_start catqa3-api 2>&1 )"; rc=$?
[ "$rc" = 69 ] && [[ "$out" == *"'timeout' command not found"* ]] && ! grep -q '^podman start' "$SHIM_LOG" && [ "$(cat "$SHIM_STATE/catqa3-api")" = exited ]; ok $? "qa_stack_start without timeout -> rc 69 before any start (rc=$rc, state=$(cat "$SHIM_STATE/catqa3-api"))"

# ---- LIFE-FU-9a: a partial tmp left by a failed build is removed
fresh; rm -rf "${ROOT:?}/cache"; export QA_STACK_BACKEND=auto SHIM_GO_PARTIAL=1; echo 'package main // BROKEN' > "$FR/docs/qa/lib/ctrctl/main.go"
qa_stack_start catqa3-api 2>/dev/null; unset SHIM_GO_PARTIAL
[ -z "$(ls "$ROOT/cache" 2>/dev/null | grep '\.tmp\.')" ] && grep -q '^GOOUT\|^go ' "$SHIM_LOG"; ok $? "failed build leaves no partial tmp file in the cache dir"
# ---- LIFE-FU-9b: a real ctrctl exit 69 must not read as "backend unavailable" (69)
fresh; rm -rf "${ROOT:?}/cache"; echo 'package main // v6' > "$FR/docs/qa/lib/ctrctl/main.go"; export SHIM_CTRCTL_FAIL=69
qa_stack_stop catqa3-api 2>/dev/null; r1=$?; echo running > "$SHIM_STATE/catqa3-api"; qa_stack_start catqa3-api 2>/dev/null; r2=$?; unset SHIM_CTRCTL_FAIL
{ [ "$r1" = 1 ] && [ "$r2" = 1 ]; }; ok $? "ctrctl's own exit 69 maps to 1, 69 stays reserved for backend unavailable (stop=$r1 start=$r2)"

# ---- LIFE-FU-10: bare call under errexit yields 1, never ctrctl's raw 69
fresh; rm -rf "${ROOT:?}/cache"; export QA_STACK_BACKEND=auto; echo 'package main // v7' > "$FR/docs/qa/lib/ctrctl/main.go"; export SHIM_CTRCTL_FAIL=69
( set -e; qa_stack_stop catqa3-api 2>"$ROOT/err1" ); r1=$?
echo running > "$SHIM_STATE/catqa3-api"; ( set -e; qa_stack_start catqa3-api 2>"$ROOT/err2" ); r2=$?; unset SHIM_CTRCTL_FAIL
{ [ "$r1" = 1 ] && [ "$r2" = 1 ] && grep -q 'stop catqa3-api failed' "$ROOT/err1" && grep -q 'start catqa3-api failed' "$ROOT/err2"; }; ok $? "bare qa_stack_stop/start under set -e: ctrctl 69 -> 1 (stop=$r1 start=$r2)"
export QA_STACK_BACKEND=podman
# ---- LIFE-FU-11: transient failing Health inspect (unknown) recovers to healthy
fresh; export SHIM_HEALTH_FAIL_POLLS=3; QA_HEALTH_TIMEOUT=20 qa_stack_start catqa3-api 2>"$ROOT/err"; rc=$?; unset SHIM_HEALTH_FAIL_POLLS
[ "$rc" = 0 ] && [ "$(grep -c Health "$SHIM_LOG")" -ge 4 ]; ok $? "Health inspect failing first 3 polls then healthy -> rc 0 (rc=$rc)"
# ---- LIFE-FU-12: Health inspect with empty output and exit 0 is unknown, not 'none'
fresh; export SHIM_HEALTH_EMPTY=1; QA_HEALTH_TIMEOUT=2 qa_stack_start catqa3-api 2>"$ROOT/err"; rc=$?; unset SHIM_HEALTH_EMPTY
[ "$rc" = 1 ] && ! grep -q 'running none' "$ROOT/err"; ok $? "empty Health output (exit 0) is not read as none (rc=$rc)"
# ---- LIFE-FU-13: _qa_strip_guid leaves an unset IFS unset
( unset IFS; _qa_strip_guid 'unix:path=/x,guid=1' >/dev/null; [ -z "${IFS+x}" ] ); ok $? "_qa_strip_guid does not leave IFS set-but-empty after unset IFS"
( IFS=:; _qa_strip_guid 'unix:path=/x,guid=1' >/dev/null; [ "$IFS" = : ] ); ok $? "_qa_strip_guid preserves a custom IFS"
# ---- LIFE-FU-14: timeout message carries the last observed state
fresh; export SHIM_NEVER_HEALTHY=1; QA_HEALTH_TIMEOUT=2 qa_stack_start catqa3-api 2>"$ROOT/err"
grep -q 'not healthy after 2s (last state: running starting)' "$ROOT/err"; ok $? "timeout message logs last state: $(grep 'not healthy' "$ROOT/err")"
export QA_STACK_BACKEND=podman
echo "passed=$pass failed=$fail"; [ "$fail" = 0 ]
