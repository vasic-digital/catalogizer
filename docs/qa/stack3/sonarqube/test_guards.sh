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
case "$1 $2" in
 "pod exists") [ -f "$SHIM_STATE/pod" ];;
 "pod inspect") cat "$SHIM_STATE/pod_label" 2>/dev/null;;
 "volume exists") [ -f "$SHIM_STATE/vol_$3" ];;
 "volume inspect") cat "$SHIM_STATE/vol_$3" 2>/dev/null;;
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
chmod +x "$BIN"/*
export SHIM_STATE="$S" PATH="$BIN:$PATH" SQ_DIR="$SB/sq" SHIM_FREE_GB=500 SHIM_USED_PCT=40 SONAR_BIND=127.0.0.1 SONAR_PORT=19877
export SQ_UP_TRIES=1 SQ_LOAD_TIMEOUT=0 LOADAVG_FILE="$SB/load_lo"; echo "3.00 3.00 3.00 1/1 1" > "$SB/load_lo"
FAIL=0; ok() { echo "PASS $1"; }; bad() { echo "FAIL $1"; FAIL=1; }
reset() { rm -f "$S"/* ; : > "$S/calls"; }
removed() { grep -qE '^(pod rm|volume rm|rm )' "$S/calls"; }
created() { grep -qE '^(pod create|volume create|run )' "$S/calls"; }

# T1 F1: stack name outside ^catsonar[a-z0-9-]*$ is refused by down.sh --purge, nothing removed.
reset; touch "$S/pod" "$S/vol_catqa3-pg"; echo sonarqube > "$S/pod_label"
SQ_STACK=catqa3 "$SRC/down.sh" --purge >/dev/null 2>&1; rc=$?
{ [ $rc -ne 0 ] && ! removed; } && ok "T1 foreign stack name refused" || bad "T1 foreign stack name refused (rc=$rc)"
reset; SQ_STACK='catsonar;x' "$SRC/down.sh" >/dev/null 2>&1; [ $? -ne 0 ] && ok "T1b metachar stack name refused" || bad "T1b"

# T2 F1: pod without our label -> down.sh and up.sh refuse and change nothing.
reset; touch "$S/pod"
"$SRC/down.sh" >/dev/null 2>&1; rc=$?; { [ $rc -ne 0 ] && ! removed; } && ok "T2 down.sh refuses unlabelled pod" || bad "T2 down.sh unlabelled pod (rc=$rc)"
reset; touch "$S/pod"
"$SRC/up.sh" >/dev/null 2>&1; rc=$?; { [ $rc -ne 0 ] && ! removed && ! created; } && ok "T2b up.sh refuses unlabelled pod" || bad "T2b up.sh unlabelled pod (rc=$rc)"

# T3 F1: --purge with one unlabelled volume refuses BEFORE removing anything (even the labelled pod/volumes).
reset; touch "$S/pod" "$S/vol_catsonar-pg" "$S/vol_catsonar-data"; echo sonarqube > "$S/pod_label"; echo sonarqube > "$S/vol_catsonar-pg"; : > "$S/vol_catsonar-data"
"$SRC/down.sh" --purge >/dev/null 2>&1; rc=$?; { [ $rc -ne 0 ] && ! removed; } && ok "T3 purge refuses when a volume is unlabelled, removes nothing" || bad "T3 (rc=$rc)"
reset; touch "$S/pod" "$S/vol_catsonar-pg"; echo sonarqube > "$S/pod_label"; echo sonarqube > "$S/vol_catsonar-pg"
"$SRC/down.sh" --purge >/dev/null 2>&1; rc=$?; { [ $rc -eq 0 ] && grep -q '^pod rm' "$S/calls" && grep -q '^volume rm.*catsonar-pg' "$S/calls"; } && ok "T3b labelled resources are removed" || bad "T3b (rc=$rc)"
reset; touch "$S/vol_catsonar-pg"; echo sonarqube > "$S/vol_catsonar-pg"; touch "$S/vol_catsonar-logs"
"$SRC/up.sh" >/dev/null 2>&1; rc=$?; { [ $rc -ne 0 ] && ! created; } && ok "T3c up.sh refuses unlabelled existing volume" || bad "T3c (rc=$rc)"

# T4 F2: load gate. High load must block (timeout 0), low load must pass; default LOAD_MAX = 0.75*nproc.
echo "50.00 40.00 30.00 1/1 1" > "$SB/load_hi"; echo "3.00 3.00 3.00 1/1 1" > "$SB/load_lo"
( . "$SRC/lib.sh"; LOADAVG_FILE="$SB/load_hi"; LOAD_MAX=12; wait_for_load 0 ) >/dev/null 2>&1; [ $? -ne 0 ] && ok "T4 high load blocks" || bad "T4 high load blocks"
( . "$SRC/lib.sh"; LOADAVG_FILE="$SB/load_lo"; LOAD_MAX=12; wait_for_load 0 ) >/dev/null 2>&1; [ $? -eq 0 ] && ok "T4b low load passes" || bad "T4b low load passes"
( . "$SRC/lib.sh"; [ "$LOAD_MAX" = 12 ] ) && ok "T4c LOAD_MAX default 0.75*16 = 12" || bad "T4c LOAD_MAX default"
( export SHIM_NPROC=8; . "$SRC/lib.sh"; [ "$LOAD_MAX" = 6 ] ) && ok "T4d LOAD_MAX follows nproc (8 -> 6)" || bad "T4d"
reset; echo "50.00 1 1 1/1 1" > "$SB/load_hi"
LOADAVG_FILE="$SB/load_hi" LOAD_MAX=12 SQ_LOAD_TIMEOUT=0 "$SRC/up.sh" >/dev/null 2>&1; [ $? -ne 0 ] && ! created && ok "T4e up.sh does not start under high load" || bad "T4e"

# T5 F4: disk floor.
( . "$SRC/lib.sh"; SHIM_FREE_GB=5 SHIM_USED_PCT=60 check_disk_floor / ) >/dev/null 2>&1; [ $? -ne 0 ] && ok "T5 <10 GB free refused" || bad "T5"
( . "$SRC/lib.sh"; SHIM_FREE_GB=50 SHIM_USED_PCT=99 check_disk_floor / ) >/dev/null 2>&1; [ $? -ne 0 ] && ok "T5b >98% used refused" || bad "T5b"
( . "$SRC/lib.sh"; SHIM_FREE_GB=37 SHIM_USED_PCT=98 check_disk_floor / ) >/dev/null 2>&1; [ $? -eq 0 ] && ok "T5c 37 GB / 98% accepted" || bad "T5c"
reset; SHIM_FREE_GB=5 "$SRC/up.sh" >/dev/null 2>&1; [ $? -ne 0 ] && ! created && ok "T5d up.sh refuses on low disk" || bad "T5d"

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

# T8 F7: scan.sh resets gate_status.txt instead of appending (static check of the truncating redirect before the loop).
grep -q ': > "\$R/gate_status.txt"' "$SRC/scan.sh" && ok "T8 gate_status.txt truncated at start" || bad "T8"
exit $FAIL
