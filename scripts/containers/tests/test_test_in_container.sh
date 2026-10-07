#!/usr/bin/env bash
# test_test_in_container.sh - T121 (RED first). Oracle for scripts/test-in-container.sh (TIC), the lane dispatcher, and its lane table
# scripts/containers/lanes.tsv. Oracle strategy (11.4.245): SPECIFIED - the (app, lane) -> wrapper table below is written by hand from the T121 task
# text, independent of lanes.tsv, and the real lane table must equal it exactly; the dispatch is observed through a SHIM per run_*.sh wrapper
# (TIC_WRAPPER_DIR, test hook) that logs the argv it receives. Control needle: the `catalog-api unit` row must reach the run_go.sh shim.
# Paired mutations: copies of test-in-container.sh with ONE expression changed (python str replace, exactly one occurrence), run against this body
# (TIC_SUT=<copy>, TIC_TEST_MUTANT=1); every copy must make it FAIL. The one the task names - a dispatcher that drops --memory from the composed call -
# is `drop-memory`; its result lines are the record the task asks for.
# Usage: test_test_in_container.sh      TIC_TEST_NO_MUTATIONS=1 ...  tests only      TIC_TEST_NO_REAL=1 ...  skip the real container leg
# Env:   TIC_MUTATION_RECORD  file that receives one line per mutation (default: scratch)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
SUT="${TIC_SUT:-$REPO/scripts/test-in-container.sh}"
FAILS=0; PASSES=0
# count_tok <dir> <token>: how many files in <dir> carry <token> in their name (a glob, never ls | grep)
count_tok() { local n=0 f; for f in "$1"/*"$2"*; do [ -e "$f" ] && n=$((n+1)); done; echo "$n"; }
ok()  { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required by this test"; exit 2; }; done
[ -f "$SUT" ] || { echo "FAIL: test-in-container.sh not found at $SUT"; echo "RESULT pass=$PASSES fail=1"; exit 1; }
[ -f "$REPO/scripts/containers/lanes.tsv" ] || { echo "FAIL: scripts/containers/lanes.tsv not found"; echo "RESULT pass=$PASSES fail=1"; exit 1; }
# the SUT locates its siblings (containers/envelope.sh, lanes.tsv) relative to itself: a mutant copy lives in a scratch tree that links them
T="$(mktemp -d "${TMPDIR:-/tmp}/tic-test.XXXXXX")"
trap 'rm -rf "$T"' EXIT
SUTROOT="$(dirname "$SUT")"
if [ "$SUTROOT" != "$REPO/scripts" ]; then :; fi

# --- shims: one per wrapper, logging argv ---
SH="$T/wrappers"; mkdir -p "$SH"
for w in run_go run_node run_docs run_scan run_playwright run_testutil run_kcov run_rust run_qa; do
  cat >"$SH/$w.sh" <<EOF
#!/usr/bin/env bash
{ echo "---CALL---"; echo "WRAPPER:$w"; printf 'ARG:%s\n' "\$@"; } >>"\${SHIM_LOG:?}"
echo "shim-$w-stdout"
exit "\${SHIM_RC:-0}"
EOF
  chmod +x "$SH/$w.sh"
done
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
CK="$T/checkout"; mkdir -p "$CK"
export TIC_TEST_MODE=1 TIC_WRAPPER_DIR="$SH" SHIM_LOG="$T/shim.log"
export ENVELOPE_TEST_MODE=1 ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000
export LONGOPS_REPO="$T/reg/repo" LONGOPS_DIR="$T/reg/repo/.audit/longops" LONGOPS_AUDIT="$T/reg/repo/.audit" LONGOPS_ALLOW_TMPFS=1; mkdir -p "$T/reg/repo/.audit"
export DISK_HEADROOM_OUT_DIR="$T/disk"; mkdir -p "$DISK_HEADROOM_OUT_DIR"   # the real leg's disk-headroom records go to scratch, never into the real evidence/disk
TOK="tc$$x$RANDOM"; RN=0
# review round 2 I1: TIC reads no envelope and passes NO --memory and NO --cpus: the wrapper's own locked reading is the only source of the limits
tic() { : >"$SHIM_LOG"; unset SHIM_RC; ( cd "$CK" && bash "$SUT" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
ncalls() { grep -c '^---CALL---$' "$SHIM_LOG"; }
wrapper_of() { sed -n 's/^WRAPPER://p' "$SHIM_LOG" | head -1; }
refused() { grep -q "REFUSED reason=$1" "$T/stderr"; }

# ============== the lane table, as written by hand from the task text (the specified oracle) ==============
# T121's eleven rows, then the three rows T200a added (build-scripts unit run_kcov, catalogizer-desktop rust run_rust, installer-wizard rust run_rust; review round 2 I3: this
# oracle was not updated when they landed and the suite was red on main with its own negative control declaring the mutation harness blind)
EXPECT=$'catalog-api unit run_go\ncatalog-api contract run_go\ncatalog-api integration run_go\ncatalog-api e2e run_go\ncatalog-web unit run_node\ncatalog-web contract run_node\ncatalog-web tooling run_node\ncatalog-web e2e run_playwright\ndocs docs run_docs\ndocs render run_docs\ntooling unit run_testutil\nbuild-scripts unit run_kcov\ncatalogizer-desktop rust run_rust\ninstaller-wizard rust run_rust'
ACTUAL="$(grep -v '^#' "$REPO/scripts/containers/lanes.tsv" | grep -v '^[[:space:]]*$' | tr '\t' ' ')"
check "the real lane table equals the hand-written table (14 reviewed rows, no extra row)" "$ACTUAL" "$EXPECT"
[ "$(grep -c '^qa	' "$REPO/scripts/containers/lanes.tsv")" = 0 ] && ok "the reserved qa key has no row" || bad "qa has a row"

# ============== every row dispatches to its wrapper, with the envelope limits composed in ==============
while read -r app lane want; do
  tic "$app" "$lane" -- echo "go $app $lane"
  check "$app $lane: dispatched (exit 0)" "$RC" 0
  check "$app $lane: reached the $want shim" "$(wrapper_of)" "$want"
  check "$app $lane: exactly one wrapper call" "$(ncalls)" 1
  grep -qxF -- "ARG:--memory" "$SHIM_LOG" && bad "$app $lane: TIC passed --memory (the wrapper's locked reading is the only source of the limit): $(tr '\n' ' ' <"$SHIM_LOG")" || ok "$app $lane: TIC passes no --memory"
  grep -qxF -- "ARG:--cpus" "$SHIM_LOG" && bad "$app $lane: TIC passed --cpus" || ok "$app $lane: TIC passes no --cpus"
done <<<"$EXPECT"
# control needle (11.4.201): the catalog-api unit row must reach the run_go.sh shim, and the shim must see the command after --
tic catalog-api unit -- go test ./...
check "control needle: catalog-api unit reaches run_go" "$(wrapper_of)" run_go
tr '\n' ' ' <"$SHIM_LOG" | grep -q -- 'ARG:-- ARG:go ARG:test ARG:\./\.\.\. ' && ok "the command words follow -- in the composed call" || bad "command: $(tr '\n' ' ' <"$SHIM_LOG")"
# the option order: the passthrough options exactly as given, then --
tic --out /tmp/x --network=none --purpose p:1 --op-id oid-1 --need 5 --rw docs --no-progress-s 9 --wall-s 100 tooling unit -- pytest -q
tr '\n' ' ' <"$SHIM_LOG" | grep -q -- "ARG:--out ARG:/tmp/x ARG:--network=none ARG:--purpose ARG:p:1 ARG:--op-id ARG:oid-1 ARG:--need ARG:5 ARG:--rw ARG:docs ARG:--no-progress-s ARG:9 ARG:--wall-s ARG:100 ARG:-- ARG:pytest ARG:-q" && ok "wrapper options are handed through unchanged, in order, with no limit added" || bad "composed: $(tr '\n' ' ' <"$SHIM_LOG")"
# a command with spaces and shell metacharacters stays one argv word each
tic tooling unit -- sh -c 'echo "a b"; touch x' 'arg with  spaces'
grep -qxF 'ARG:echo "a b"; touch x' "$SHIM_LOG" && grep -qxF 'ARG:arg with  spaces' "$SHIM_LOG" && ok "words with spaces and metacharacters are passed as single argv words" || bad "words: $(tr '\n' '|' <"$SHIM_LOG")"
# stdout and the exit code of the wrapper are the caller's
tic tooling unit -- true; grep -q 'shim-run_testutil-stdout' "$T/stdout" && ok "the wrapper's stdout reaches the caller" || bad "stdout lost"
: >"$SHIM_LOG"; ( cd "$CK" && SHIM_RC=42 bash "$SUT" tooling unit -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
check "the wrapper exit code is passed through" "$RC" 42

# ============== refusals: never a bare-host fallback ==============
MARK="$T/marker-ran"
for case_ in "bogus unit:unknown_app" "qa unit:unknown_app" "qa api:unknown_app" "catalog-api bogus:unknown_lane" "catalog-api docs:no_lane_row" "docs unit:no_lane_row" "tooling e2e:no_lane_row" "catalog-web api:no_lane_row"; do
  pair="${case_%%:*}"; reason="${case_##*:}"; app="${pair% *}"; lane="${pair#* }"
  rm -f "$MARK"; tic "$app" "$lane" -- touch "$MARK"
  check "$app $lane: refused with a non-zero exit" "$([ "$RC" -ne 0 ] && echo nonzero || echo "rc=$RC")" nonzero
  check "$app $lane: exit code is 1 (REFUSED)" "$RC" 1
  refused "$reason" && ok "$app $lane: reason $reason" || bad "$app $lane: reason: $(cat "$T/stderr")"
  [ ! -e "$MARK" ] && [ "$(ncalls)" = 0 ] && ok "$app $lane: the command did not run anywhere (no wrapper call, no bare-host run)" || bad "$app $lane: the command ran or a wrapper was called"
done
grep -q "never sent to another image" "$T/stderr" 2>/dev/null; tic qa unit -- true; grep -q 'never sent to another image' "$T/stderr" && ok "qa: the refusal explains the reservation until T212" || bad "qa message: $(cat "$T/stderr")"
tic; check "usage: no arguments" "$RC" 2
tic catalog-api; check "usage: lane missing" "$RC" 2
tic catalog-api unit; check "usage: missing --" "$RC" 2
tic catalog-api unit --; check "usage: missing command after --" "$RC" 2
tic --bogus catalog-api unit -- true; check "usage: unknown option" "$RC" 2
tic --memory 1 catalog-api unit -- true; check "usage: --memory is not a TIC option (the wrapper reads the envelope itself)" "$RC" 2
tic --cpus 1 catalog-api unit -- true; check "usage: --cpus is not a TIC option" "$RC" 2
tic a b c -- true; check "usage: a third positional word" "$RC" 2
check "usage errors call no wrapper" "$(ncalls)" 0
# a wrapper that does not exist yet (a shim directory without run_playwright.sh): BLOCKED, refused, nothing run
rm -f "$SH/run_playwright.sh"; rm -f "$MARK"
tic catalog-web e2e -- touch "$MARK"
check "missing wrapper: refused" "$RC" 1
refused wrapper_missing && ok "missing wrapper: reason wrapper_missing" || bad "reason: $(cat "$T/stderr")"
[ ! -e "$MARK" ] && ok "missing wrapper: the command did not run" || bad "ran on the host"
cat >"$SH/run_playwright.sh" <<'EOF'
#!/usr/bin/env bash
{ echo "---CALL---"; echo "WRAPPER:run_playwright"; printf 'ARG:%s\n' "$@"; } >>"${SHIM_LOG:?}"
exit 0
EOF
chmod +x "$SH/run_playwright.sh"
# TIC does not read the envelope (review round 2 I1): a host below the memory reserve still reaches the wrapper, which is the one that reads the envelope and refuses it
# (`<wrapper>: REFUSED reason=envelope_refused`, covered with the real wrapper in test_runners.sh); a dispatcher that read the envelope again would stop here
printf 'MemTotal:       32000000 kB\nMemAvailable:   4000000 kB\n' >"$T/meminfo"
tic catalog-api unit -- true
check "TIC leaves the envelope to the wrapper: a host below the reserve still reaches the wrapper" "$RC" 0
check "TIC leaves the envelope to the wrapper: exactly one wrapper call" "$(ncalls)" 1
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"

# ============== the lane table is validated as a whole (TIC_LANES hook) ==============
LT="$T/lanes.tsv"
lt() { printf '%b' "$1" >"$LT"; tic_lt() { : >"$SHIM_LOG"; ( cd "$CK" && TIC_LANES="$LT" bash "$SUT" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }; }
lt 'catalog-api\tunit\trun_go\n'; tic_lt catalog-api unit -- true; check "a one-row table works" "$RC" 0
lt 'catalog-api\tunit\trun_go\ncatalog-api\tunit\trun_node\n'; tic_lt catalog-api unit -- true
check "duplicate (app, lane) rows: refused" "$RC" 1; refused lane_table_duplicate && ok "duplicate: reason lane_table_duplicate" || bad "reason: $(cat "$T/stderr")"
check "duplicate rows: no wrapper call" "$(ncalls)" 0
lt 'catalog-api\tunit\trun_go\textra\n'; tic_lt catalog-api unit -- true
check "a four-column row: refused" "$RC" 1; refused lane_table_malformed && ok "four columns: reason lane_table_malformed" || bad "reason: $(cat "$T/stderr")"
lt 'catalog-api\tunit\n'; tic_lt catalog-api unit -- true; check "a two-column row: refused" "$RC" 1
lt 'catalog-api\tunit\trun_evil\n'; tic_lt catalog-api unit -- true
check "a row naming an unreviewed wrapper: refused" "$RC" 1; refused lane_table_malformed && ok "unreviewed wrapper: reason lane_table_malformed" || bad "a row naming an unreviewed wrapper: the reason is not lane_table_malformed: $(cat "$T/stderr")"
lt 'bogusapp\tunit\trun_go\n'; tic_lt catalog-api unit -- true; check "a row with an unknown app: refused" "$RC" 1
lt 'catalog-api\tbogus\trun_go\n'; tic_lt catalog-api unit -- true; check "a row with an unknown lane: refused" "$RC" 1
# a bad row anywhere refuses EVERY lane (the table is reviewed as a whole): a good lane next to the bad row is refused too
lt 'tooling\tunit\trun_testutil\ncatalog-api\tunit\trun_evil\n'; tic_lt tooling unit -- true; check "a bad row refuses an unrelated good lane too" "$RC" 1
lt '# comment\n\ntooling\tunit\trun_testutil\n'; tic_lt tooling unit -- true; check "comments and blank lines are ignored" "$RC" 0
tic_lt() { :; }
( cd "$CK" && TIC_LANES="$T/no-such-table.tsv" bash "$SUT" tooling unit -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
check "an unreadable lane table: refused" "$RC" 1; refused lane_table_unreadable && ok "unreadable table: reason lane_table_unreadable" || bad "an unreadable lane table: the reason is not lane_table_unreadable: $(cat "$T/stderr")"
# the test hooks are honoured only in a declared test run
: >"$SHIM_LOG"; ( cd "$CK" && env -u TIC_TEST_MODE bash "$SUT" tooling unit -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
check "TIC_WRAPPER_DIR without TIC_TEST_MODE: refused" "$RC" 1; refused test_hook_outside_test_mode && ok "hook reason test_hook_outside_test_mode" || bad "reason: $(cat "$T/stderr")"
check "hooks outside test mode: no shim wrapper called" "$(ncalls)" 0

# a single hook alone (review R5) is refused too: each hook is gated on its own
for hv in TIC_LANES TIC_WRAPPER_DIR; do
  : >"$SHIM_LOG"
  if [ "$hv" = TIC_LANES ]; then ( cd "$CK" && env -u TIC_TEST_MODE -u TIC_WRAPPER_DIR TIC_LANES="$REPO/scripts/containers/lanes.tsv" bash "$SUT" tooling unit -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
  else ( cd "$CK" && env -u TIC_TEST_MODE -u TIC_LANES TIC_WRAPPER_DIR="$SH" bash "$SUT" tooling unit -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?; fi
  check "the $hv hook alone without TIC_TEST_MODE: refused (exit 1)" "$RC" 1
  refused test_hook_outside_test_mode && ok "the $hv hook alone: reason test_hook_outside_test_mode" || bad "the $hv hook alone reason: $(cat "$T/stderr")"
done

# ============== the real chain: TIC -> the real wrapper -> the real run_pinned.sh -> the pinned image ==============
if [ "${TIC_TEST_NO_REAL:-0}" != 1 ]; then
  realtic() { local o=$1; shift; RN=$((RN+1)); ( cd "$REPO" && env -u TIC_WRAPPER_DIR -u TIC_LANES -u ENVELOPE_MEMINFO -u ENVELOPE_NPROC -u ENVELOPE_ULIMIT_U -u RUNP_LOCK RUNNER_TEST_MODE=1 RUNNER_SWEEP="$T/sweep-shim.sh" bash "$SUT" --out "$o" --op-id "$TOK-$RN" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
  printf '#!/usr/bin/env bash\nexit 0\n' >"$T/sweep-shim.sh"
  # an isolated registry for the real leg (the real container census is covered by test_runners.sh)
  realtic "$T/real-1" tooling unit -- python3 -c 'import yaml, jsonschema, pytest; print("tic-real-ok")'
  check "real: TIC tooling unit runs in the pinned IMG-TESTUTIL (exit 0)" "$RC" 0
  grep -q 'tic-real-ok' "$T/stdout" && ok "real: the command's output came out of the container" || bad "real: $(cat "$T/stdout" "$T/stderr" | head -5)"
  [ -s "$T/real-1/toolchain.json" ] && ok "real: the wrapper's toolchain record exists" || bad "real: no toolchain.json"
  check "real: the record shows the envelope memory the wrapper was given (<= 60% of MemTotal)" "$([ "$(jq -r .envelope.memory_bytes "$T/real-1/toolchain.json")" -le $(( $(awk '$1=="MemTotal:"{print $2*1024}' /proc/meminfo) * 60 / 100 )) ] && echo within || echo above)" within
  realtic "$T/real-2" catalog-api unit -- go version
  check "real: TIC catalog-api unit runs in the pinned IMG-GO (exit 0)" "$RC" 0
  grep -q 'go version go1' "$T/stdout" && ok "real: go version printed inside IMG-GO" || bad "real: $(cat "$T/stdout" "$T/stderr" | head -5)"
  if python3 -I -c 'import sys, yaml; d = yaml.safe_load(open(sys.argv[1])); sys.exit(0 if any(isinstance(i, dict) and i.get("id") == "IMG-NODE" for i in d["images"]) else 1)' "$REPO/build/containers/images.lock.yaml"; then
    realtic "$T/real-3" catalog-web unit -- node --version
    check "real: TIC catalog-web unit runs in the pinned IMG-NODE (the lock has it): exit 0" "$RC" 0
    grep -q '^v[0-9]' "$T/stdout" && ok "real: node --version printed inside IMG-NODE ($(head -1 "$T/stdout"))" || bad "real: $(cat "$T/stdout" "$T/stderr" | head -5)"
  else
    realtic "$T/real-3" catalog-web unit -- true
    check "real: TIC catalog-web unit is refused (the lock has no IMG-NODE)" "$RC" 1
    grep -q 'REFUSED reason=image_not_in_lock' "$T/stderr" && ok "real: honest refusal image_not_in_lock from the wrapper" || bad "real: $(cat "$T/stderr")"
  fi
  realtic "$T/real-4" qa api -- true
  check "real: TIC qa api is refused as an unknown app" "$RC" 1
fi

if [ "${TIC_TEST_NO_REAL:-0}" != 1 ]; then
  EVD="$REPO/specs/001-full-project-audit-remediation/evidence/disk"
  check "no disk-headroom record carrying this run's token ($TOK) was written into the real evidence/disk" "$(count_tok "$EVD" "$TOK")" 0
fi
echo "RESULT pass=$PASSES fail=$FAILS"
[ "$FAILS" = 0 ] || EXIT=1
EXIT="${EXIT:-0}"

# ============== paired mutations ==============
if [ "${TIC_TEST_MUTANT:-0}" = 1 ] || [ "${TIC_TEST_NO_MUTATIONS:-0}" = 1 ]; then exit "$EXIT"; fi
MREC="${TIC_MUTATION_RECORD:-$T/mutation.txt}"; : >"$MREC"
CAUGHT=0; SURV=0; TOTAL=0
# A mutant copy is placed in a WORKING tree layout: <d>/scripts/test-in-container.sh beside links to <repo>/scripts/containers and <repo>/scripts/longops. The envelope
# fails CLOSED when it cannot read the registry (review F3), so a layout without scripts/longops would make EVERY mutant fail for that one reason whatever its mutation
# (the blindness of review M1 in the TIC harness). An UNMUTATED copy placed the same way is the negative control: it must pass the whole body.
place() { # <dir> <python-old> <python-new>: rc 1 when the pattern is not found exactly once
  local d=$1
  rm -rf "$d"; mkdir -p "$d/scripts"
  ln -s "$REPO/scripts/containers" "$d/scripts/containers"; ln -s "$REPO/scripts/longops" "$d/scripts/longops"
  python3 -I - "$SUT" "$d/scripts/test-in-container.sh" "${2-}" "${3-}" <<'PY'
import sys
s = open(sys.argv[1]).read()
old, new = sys.argv[3], sys.argv[4]
if old:
    if s.count(old) != 1: sys.exit(1)
    s = s.replace(old, new)
open(sys.argv[2], "w").write(s)
PY
}
place "$T/mut-control" "" ""
( TIC_SUT="$T/mut-control/scripts/test-in-container.sh" TIC_TEST_MUTANT=1 TIC_TEST_NO_REAL=1 QUIET=1 bash "${BASH_SOURCE[0]}" ) >"$T/mut-control.log" 2>&1; CRC=$?
if [ "$CRC" = 0 ]; then echo "CONTROL  an unmutated copy placed like a mutant passes the body ($(grep '^RESULT' "$T/mut-control.log"))" | tee -a "$MREC"
else echo "CONTROL FAILED: the unmutated copy fails the body ($(grep -c '^FAIL' "$T/mut-control.log") checks): the mutation harness is blind" | tee -a "$MREC"; EXIT=1; fi
mut_case() { # <id> <expected failing-check substring> <old> <new>: caught only by a check whose NAME contains the substring
  local id=$1 expect=$2 old=$3 new=$4
  TOTAL=$((TOTAL+1))
  place "$T/mut-$id" "$old" "$new" || { echo "INVALID $id: pattern not found exactly once" | tee -a "$MREC"; SURV=$((SURV+1)); return; }
  ( TIC_SUT="$T/mut-$id/scripts/test-in-container.sh" TIC_TEST_MUTANT=1 TIC_TEST_NO_REAL=1 QUIET=1 bash "${BASH_SOURCE[0]}" ) >"$T/mut-$id.log" 2>&1; local rc=$?
  if [ "$rc" -ne 0 ] && grep -q "^FAIL: .*$expect" "$T/mut-$id.log"; then CAUGHT=$((CAUGHT+1)); echo "CAUGHT   $id by a check naming '$expect' ($(grep -c '^FAIL' "$T/mut-$id.log") failing checks)" | tee -a "$MREC"
  elif [ "$rc" -ne 0 ]; then SURV=$((SURV+1)); echo "SURVIVED $id (failed, but no failing check names '$expect': $(grep '^FAIL' "$T/mut-$id.log" | head -2 | cut -c1-110 | tr '\n' '|'))" | tee -a "$MREC"
  else SURV=$((SURV+1)); echo "SURVIVED $id (test stayed green on the mutant)" | tee -a "$MREC"; fi
}
mut_case lookup-ignores-lane 'catalog-api docs' '[ "$ra" = "$APP" ] && [ "$rl" = "$LANE" ]' '[ "$ra" = "$APP" ]'
mut_case bare-host-fallback 'refused with a non-zero exit' 'refuse no_lane_row "($APP $LANE) has no row in $TABLE; a lane exists only by a reviewed row, and there is no bare-host fallback"' '{ "${CMD[@]}"; exit $?; } #'
mut_case wrapper-missing-ignored 'missing wrapper' '[ -f "$WDIR/$WRAPPER.sh" ] || refuse wrapper_missing' 'true'
mut_case unknown-app-ignored 'bogus unit' 'refuse unknown_app "'"'"'$APP'"'"' is not one of: $APPS"' 'true'
mut_case unknown-lane-ignored 'catalog-api bogus' 'refuse unknown_lane "'"'"'$LANE'"'"' is not one of: $LANES"' 'true'
mut_case qa-not-reserved 'qa unit: reason' 'if [ "$APP" = qa ]; then' 'if false; then'
mut_case duplicate-ignored 'duplicate' 'refuse lane_table_duplicate' 'true'
mut_case columns-unchecked 'four-column' '[ -z "${extra:-}" ]' 'true'
mut_case wrapper-name-unchecked 'unreviewed wrapper' 'refuse lane_table_malformed "row ($a $l) names' 'true "row ($a $l) names'
mut_case exit-masked 'the wrapper exit code is passed through' 'exit $?   # MUT:exit' 'exit 0'
mut_case test-hooks 'TIC_WRAPPER_DIR without TIC_TEST_MODE' '[ "${TIC_TEST_MODE:-}" != 1 ]; then' '[ "${TIC_TEST_MODE:-}" != 1 ] && false; then'
mut_case R5-lanes-hook-gate 'TIC_LANES hook alone' ' || [ -n "${TIC_LANES+x}" ]' ''
mut_case R5b-wrapper-dir-hook-gate 'TIC_WRAPPER_DIR hook alone' '{ [ -n "${TIC_WRAPPER_DIR+x}" ] || ' '{ '
mut_case table-unreadable-ignored 'unreadable lane table' '[ -r "$TABLE" ] || refuse lane_table_unreadable "$TABLE"' 'true'
mut_case passes-memory-again 'TIC passed --memory' 'bash "$WDIR/$WRAPPER.sh" "${PASS[@]}" -- "${CMD[@]}"   # MUT:no-limits' 'bash "$WDIR/$WRAPPER.sh" --memory 19660800000 "${PASS[@]}" -- "${CMD[@]}"'
mut_case passes-cpus-again 'TIC passed --cpus' 'bash "$WDIR/$WRAPPER.sh" "${PASS[@]}" -- "${CMD[@]}"   # MUT:no-limits' 'bash "$WDIR/$WRAPPER.sh" --cpus 9 "${PASS[@]}" -- "${CMD[@]}"'
mut_case reads-the-envelope-again 'leaves the envelope to the wrapper' 'bash "$WDIR/$WRAPPER.sh" "${PASS[@]}" -- "${CMD[@]}"   # MUT:no-limits' 'bash "$CDIR/envelope.sh" --toolchain "${WRAPPER#run_}" --format json >/dev/null 2>&1 || refuse envelope_refused "the envelope"
bash "$WDIR/$WRAPPER.sh" "${PASS[@]}" -- "${CMD[@]}"'
echo "MUTATION RESULT caught=$CAUGHT survived=$SURV total=$TOTAL" | tee -a "$MREC"
[ "$SURV" = 0 ] || EXIT=1
exit "$EXIT"
