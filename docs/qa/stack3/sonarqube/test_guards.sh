#!/usr/bin/env bash
# Sandbox tests for the safety guards of up.sh/down.sh/scan.sh/lib.sh. No real podman, no real stack is touched:
# a podman/df/nproc shim on PATH answers from files in a mktemp sandbox. Usage: test_guards.sh [SCRIPT_DIR]
# Exit 0 = all guards behave; non-zero = at least one test failed (used by mutation_guards.sh, which must see failures).
set -uo pipefail
SRC="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
SB="$(mktemp -d /home/milosvasic/.cache/claude-tmp/sqtest.XXXXXX)"; : "${SB:?}"
trap 'rm -rf "${SB:?}"' EXIT
S="$SB/state"; BIN="$SB/bin"; mkdir -p "$S" "$BIN" "$SB/sq" "$SB/out"
cat > "$BIN/podman" <<'SH'
#!/usr/bin/env bash
echo "$*" >> "$SHIM_STATE/calls"
if [ "$1" = pull ]; then [ -f "$SHIM_STATE/pullmsg" ] && { cat "$SHIM_STATE/pullmsg" >&2; exit 1; }; exit 0; fi
case "$1 $2" in
 "image exists") [ ! -f "$SHIM_STATE/noimg" ];;
 "pod exists") [ -f "$SHIM_STATE/pod" ];;
 "pod inspect") cat "$SHIM_STATE/pod_label" 2>/dev/null; exit 0;;
 "volume exists") [ -f "$SHIM_STATE/vol_$3" ];;
 "volume inspect") cat "$SHIM_STATE/vol_$3" 2>/dev/null; exit 0;;
 "inspect "*) case "$*" in *"--format"*Health*) echo healthy;; *"--format"*) echo running;; esac; exit 0;;  # qa_stack_wait_healthy (--format); up.sh itself uses -f
 *) exit 0;;
esac
SH
cat > "$BIN/df" <<'SH'
#!/usr/bin/env bash
printf 'Filesystem 1024-blocks Used Available Capacity Mounted\n/dev/x 1000000000 1 %s %s%% /\n' "$((SHIM_FREE_GB*1048576))" "$SHIM_USED_PCT"
SH
cat > "$BIN/nproc" <<'SH'
#!/usr/bin/env bash
echo "${SHIM_NPROC:-16}"
SH
printf '#!/usr/bin/env bash\nexit 0\n' > "$BIN/sleep"   # no real waiting in tests
cat > "$BIN/ss" <<'SH'
#!/usr/bin/env bash
[ -n "${SHIM_SS_FAIL:-}" ] && exit 1
printf 'State Recv-Q Send-Q Local Peer\n'
[ -n "${SHIM_LISTEN:-}" ] && printf 'LISTEN 0 4096 127.0.0.1:%s 0.0.0.0:*\n' "$SHIM_LISTEN"
exit 0
SH
REAL_IP="$(command -v ip)"; export REAL_IP
cat > "$BIN/ip" <<'SH'
#!/usr/bin/env bash
[ -n "${SHIM_IP:-}" ] && { printf '%s\n' "$SHIM_IP"; exit 0; }
exec "$REAL_IP" "$@"
SH
chmod +x "$BIN"/*
export SHIM_STATE="$S" PATH="$BIN:$PATH" SQ_DIR="$SB/sq" SHIM_FREE_GB=500 SHIM_USED_PCT=40 SONAR_BIND=127.0.0.1 SONAR_PORT=19877
export SQ_UP_TRIES=1 SQ_LOAD_TIMEOUT=0 LOADAVG_FILE="$SB/load_lo"; echo "3.00 3.00 3.00 1/1 1" > "$SB/load_lo"
FAIL=0; ok() { echo "PASS $1"; }; bad() { echo "FAIL $1"; FAIL=1; }
reset() { rm -f "$S"/* ; : > "$S/calls"; }
removed() { grep -qE '^(pod rm|volume rm|rm )' "$S/calls"; }
created() { grep -qE '^(pod create|volume create|run |create |container create )' "$S/calls"; }   # FU-10: ctr_up uses `podman create` for containers

# T1 F1: stack name outside ^catsonar[a-z0-9-]*$ is refused by down.sh --purge, nothing removed.
reset; touch "$S/pod" "$S/vol_catqa3-pg"; echo sonarqube > "$S/pod_label"
SQ_STACK=catqa3 "$SRC/down.sh" --purge >/dev/null 2>&1; rc=$?
{ [ $rc -ne 0 ] && ! removed; } && ok "T1 foreign stack name refused" || bad "T1 foreign stack name refused (rc=$rc)"
reset; SQ_STACK='catsonar;x' "$SRC/down.sh" >/dev/null 2>&1; [ $? -ne 0 ] && ok "T1b metachar stack name refused" || bad "T1b"

# T2 F1: pod without our label -> down.sh and up.sh refuse and change nothing.
reset; touch "$S/pod"
"$SRC/down.sh" >/dev/null 2>&1; rc=$?; { [ $rc -ne 0 ] && ! removed; } && ok "T2 down.sh refuses unlabelled pod" || bad "T2 down.sh unlabelled pod (rc=$rc)"
reset; touch "$S/pod"
"$SRC/up.sh" >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'without label' "$SB/o" && ! removed && ! created; } && ok "T2b up.sh refuses unlabelled pod" || bad "T2b up.sh unlabelled pod (rc=$rc)"

# T2c FU-9/FU-10: ctr_up refuses a container name outside the stack prefix BEFORE `podman create`; nothing is created or started.
reset; ( . "$SRC/lib.sh" >/dev/null 2>&1; ctr_up "otherstack-x" img >/dev/null 2>&1 ); rc=$?
{ [ $rc -ne 0 ] && ! created && ! grep -q '^start ' "$S/calls"; } && ok "T2c ctr_up refuses a foreign container name before create" || bad "T2c ctr_up foreign name (rc=$rc)"
reset; ( . "$SRC/lib.sh" >/dev/null 2>&1; ctr_up "${STACK}-x" img >/dev/null 2>&1 ); rc=$?
{ [ $rc -eq 0 ] && created; } && ok "T2d ctr_up on an in-prefix name creates the container (created() matches container create)" || bad "T2d (rc=$rc)"

# T3 F1: --purge with one unlabelled volume refuses BEFORE removing anything (even the labelled pod/volumes).
reset; touch "$S/pod" "$S/vol_catsonar-pg" "$S/vol_catsonar-data"; echo sonarqube > "$S/pod_label"; echo sonarqube > "$S/vol_catsonar-pg"; : > "$S/vol_catsonar-data"
"$SRC/down.sh" --purge >/dev/null 2>&1; rc=$?; { [ $rc -ne 0 ] && ! removed; } && ok "T3 purge refuses when a volume is unlabelled, removes nothing" || bad "T3 (rc=$rc)"
reset; touch "$S/pod" "$S/vol_catsonar-pg"; echo sonarqube > "$S/pod_label"; echo sonarqube > "$S/vol_catsonar-pg"
"$SRC/down.sh" --purge >/dev/null 2>&1; rc=$?; { [ $rc -eq 0 ] && grep -q '^pod rm' "$S/calls" && grep -q '^volume rm.*catsonar-pg' "$S/calls"; } && ok "T3b labelled resources are removed" || bad "T3b (rc=$rc)"
reset; touch "$S/vol_catsonar-pg"; echo sonarqube > "$S/vol_catsonar-pg"; touch "$S/vol_catsonar-logs"
"$SRC/up.sh" >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'without label' "$SB/o" && ! created; } && ok "T3c up.sh refuses unlabelled existing volume" || bad "T3c (rc=$rc)"

# T4 F2: load gate. High load must block (timeout 0), low load must pass; default LOAD_MAX = 0.75*nproc.
echo "50.00 40.00 30.00 1/1 1" > "$SB/load_hi"; echo "3.00 3.00 3.00 1/1 1" > "$SB/load_lo"
( . "$SRC/lib.sh"; LOADAVG_FILE="$SB/load_hi"; LOAD_MAX=12; wait_for_load 0 ) >/dev/null 2>&1; [ $? -ne 0 ] && ok "T4 high load blocks" || bad "T4 high load blocks"
( . "$SRC/lib.sh"; LOADAVG_FILE="$SB/load_lo"; LOAD_MAX=12; wait_for_load 0 ) >/dev/null 2>&1; [ $? -eq 0 ] && ok "T4b low load passes" || bad "T4b low load passes"
( . "$SRC/lib.sh"; [ "$LOAD_MAX" = 12 ] ) && ok "T4c LOAD_MAX default 0.75*16 = 12" || bad "T4c LOAD_MAX default"
( export SHIM_NPROC=8; . "$SRC/lib.sh"; [ "$LOAD_MAX" = 6 ] ) && ok "T4d LOAD_MAX follows nproc (8 -> 6)" || bad "T4d"
reset; echo "50.00 1 1 1/1 1" > "$SB/load_hi"
LOADAVG_FILE="$SB/load_hi" LOAD_MAX=12 SQ_LOAD_TIMEOUT=0 "$SRC/up.sh" >"$SB/o" 2>&1; [ $? -ne 0 ] && grep -q 'stayed above' "$SB/o" && ! created && ok "T4e up.sh does not start under high load" || bad "T4e"

# T5 F4: disk floor.
( . "$SRC/lib.sh"; SHIM_FREE_GB=5 SHIM_USED_PCT=60 check_disk_floor / ) >/dev/null 2>&1; [ $? -ne 0 ] && ok "T5 <10 GB free refused" || bad "T5"
( . "$SRC/lib.sh"; SHIM_FREE_GB=50 SHIM_USED_PCT=99 check_disk_floor / ) >/dev/null 2>&1; [ $? -ne 0 ] && ok "T5b >98% used refused" || bad "T5b"
( . "$SRC/lib.sh"; SHIM_FREE_GB=37 SHIM_USED_PCT=98 check_disk_floor / ) >/dev/null 2>&1; [ $? -eq 0 ] && ok "T5c 37 GB / 98% accepted" || bad "T5c"
reset; SHIM_FREE_GB=5 "$SRC/up.sh" >"$SB/o" 2>&1; [ $? -ne 0 ] && grep -q 'refusing to start' "$SB/o" && ! created && ok "T5d up.sh refuses on low disk" || bad "T5d"

( . "$SRC/lib.sh"; SHIM_FREE_GB=10 SHIM_USED_PCT=50 check_disk_floor / ) >/dev/null 2>&1; [ $? -eq 0 ] && ok "T5e exactly 10 GB free accepted" || bad "T5e"
( . "$SRC/lib.sh"; SHIM_FREE_GB=9 SHIM_USED_PCT=50 check_disk_floor / ) >/dev/null 2>&1; [ $? -ne 0 ] && ok "T5f 9 GB free refused" || bad "T5f"

# T10 N1: positive control - in a clean, labelled scenario up.sh must get PAST the address/port/ownership/disk/load/image checks and create
# the pod, so the "refused / nothing created" tests above cannot pass because of an early random failure. Repeated 5x (was flaky 4/8).
okc=0; for n in 1 2 3 4 5; do reset; "$SRC/up.sh" >/dev/null 2>&1; grep -q '^pod create' "$S/calls" && okc=$((okc+1)); done
[ "$okc" = 5 ] && ok "T10 clean scenario reaches pod create 5/5" || bad "T10 reached pod create only $okc/5"

# T11 N2: labels that exist but differ (io.catalogizer.service=other), pod AND volume, and empty label output with exit 0 (real podman).
reset; touch "$S/pod"; echo other > "$S/pod_label"
"$SRC/down.sh" >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'without label' "$SB/o" && ! removed; } && ok "T11 pod labelled 'other' refused (down)" || bad "T11 (rc=$rc)"
reset; touch "$S/pod"; echo other > "$S/pod_label"
"$SRC/up.sh" >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'without label' "$SB/o" && ! removed && ! created; } && ok "T11b pod labelled 'other' refused (up)" || bad "T11b (rc=$rc)"
reset; touch "$S/vol_catsonar-pg"; echo other > "$S/vol_catsonar-pg"
"$SRC/down.sh" --purge >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'without label' "$SB/o" && ! removed; } && ok "T11c volume labelled 'other' refused (purge)" || bad "T11c (rc=$rc)"
reset; touch "$S/vol_catsonar-pg"; echo other > "$S/vol_catsonar-pg"
"$SRC/up.sh" >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'without label' "$SB/o" && ! created; } && ok "T11d volume labelled 'other' refused (up)" || bad "T11d (rc=$rc)"

# T12 N1: address / port / ss-failure checks (each must refuse for its own reason; nothing created).
reset; SONAR_BIND=203.0.113.9 "$SRC/up.sh" >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'not a local address' "$SB/o" && ! created; } && ok "T12 non-local SONAR_BIND refused" || bad "T12 (rc=$rc)"
reset; SHIM_LISTEN=19877 "$SRC/up.sh" >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'already in use' "$SB/o" && ! created; } && ok "T12b busy port refused" || bad "T12b (rc=$rc)"
reset; SHIM_SS_FAIL=1 "$SRC/up.sh" >"$SB/o" 2>&1; rc=$?; { [ $rc -ne 0 ] && grep -q 'fail closed' "$SB/o" && ! created; } && ok "T12c ss failure fails closed" || bad "T12c (rc=$rc)"

# T13 FU-1: plain down.sh (no --purge) removes the pod but NEVER a volume.
reset; touch "$S/pod" "$S/vol_catsonar-pg" "$S/vol_catsonar-data"; echo sonarqube > "$S/pod_label"; echo sonarqube > "$S/vol_catsonar-pg"; echo sonarqube > "$S/vol_catsonar-data"
"$SRC/down.sh" >/dev/null 2>&1; rc=$?; { [ $rc -eq 0 ] && grep -q '^pod rm' "$S/calls" && ! grep -q '^volume rm' "$S/calls"; } && ok "T13 plain down.sh never removes volumes" || bad "T13 (rc=$rc)"
# T14 FU-2: SONAR_BIND=127.0.0.1 must not be satisfied by 127.0.0.10 (no prefix matching).
reset; SHIM_IP='    inet 127.0.0.10/8 scope host lo' SONAR_BIND=127.0.0.1 "$SRC/up.sh" >"$SB/o" 2>&1; rc=$?
{ [ $rc -ne 0 ] && grep -q 'not a local address' "$SB/o" && ! created; } && ok "T14 127.0.0.1 not satisfied by 127.0.0.10" || bad "T14 (rc=$rc)"
# T15 FU-3: port 119877 listening must not count as 19877 in use (up.sh proceeds to pod create).
reset; SHIM_LISTEN=119877 "$SRC/up.sh" >"$SB/o" 2>&1; grep -q '^pod create' "$S/calls" && ! grep -q 'already in use' "$SB/o" && ok "T15 port 119877 is not 19877" || bad "T15"

# T6 F6: no_secret_in must REPORT a planted leak, and pass a clean file; scrub_file removes it.
printf 'SONAR_ADMIN_PASSWORD=Aa1-planted-secret-value-xyz\nSONAR_DB_PASSWORD=dbplantedsecretvalue123456789\n' > "$SB/sq/sonar.env"; chmod 600 "$SB/sq/sonar.env"
printf 'ok line\nleak Aa1-planted-secret-value-xyz here\n' > "$SB/leak.txt"; printf 'nothing here\n' > "$SB/clean.txt"
( . "$SRC/lib.sh"; load_secrets; no_secret_in "$SB/leak.txt" ) ; [ $? -ne 0 ] && ok "T6 planted leak reported" || bad "T6 planted leak reported"
( . "$SRC/lib.sh"; load_secrets; no_secret_in "$SB/clean.txt" ) ; [ $? -eq 0 ] && ok "T6b clean file passes" || bad "T6b"
( . "$SRC/lib.sh"; load_secrets; scrub_file "$SB/leak.txt"; no_secret_in "$SB/leak.txt" ) ; [ $? -eq 0 ] && ok "T6c scrub removes leak" || bad "T6c"

# T7 F7: scan.sh refuses a non-empty RUN_ID dir (and leaves it untouched).
mkdir -p "$SB/out/run1"; echo keep > "$SB/out/run1/x"; reset
printf 'SONAR_ADMIN_PASSWORD=Aa1-planted-secret-value-xyz\nSONAR_DB_PASSWORD=dbplantedsecretvalue123456789\nSONAR_TOKEN=squ_plantedtoken\n' > "$SB/sq/sonar.env"
REPO="$SB" OUT="$SB/out" RUN_ID=run1 "$SRC/scan.sh" > "$SB/scan.out" 2>&1; rc=$?
{ grep -q 'is not empty' "$SB/scan.out" && [ $rc -ne 0 ] && [ "$(cat "$SB/out/run1/x")" = keep ] && [ "$(ls "$SB/out/run1" | wc -l)" = 1 ] && ! created; } && ok "T7 non-empty RUN_ID dir refused" || bad "T7 (rc=$rc)"

# T9: missing image + Docker Hub rate limit -> clear message, nothing created; without the limit message a pull failure is still fatal.
reset; touch "$S/noimg"; echo 'Error: ... toomanyrequests: You have reached your unauthenticated pull rate limit.' > "$S/pullmsg"
"$SRC/up.sh" > "$SB/up.out" 2>&1; rc=$?
{ [ $rc -ne 0 ] && grep -q 'Docker Hub rate limit' "$SB/up.out" && grep -q 'podman login' "$SB/up.out" && ! created; } && ok "T9 rate-limited pull -> clear message, nothing created" || bad "T9 (rc=$rc)"
reset; touch "$S/noimg"; "$SRC/up.sh" > "$SB/up.out" 2>&1; ! grep -q 'rate limit' "$SB/up.out" && grep -q 'pulling' "$SB/up.out" && ok "T9b pull attempted for missing image (no false rate-limit message)" || bad "T9b"

# T8 F7: scan.sh resets gate_status.txt instead of appending (static check of the truncating redirect before the loop).
grep -q ': > "\$R/gate_status.txt"' "$SRC/scan.sh" && ok "T8 gate_status.txt truncated at start" || bad "T8"
exit $FAIL
