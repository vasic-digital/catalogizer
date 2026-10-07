#!/usr/bin/env bash
# test_runners.sh - T118 (RED first) / T120 (GREEN). Oracle for the build and test wrappers scripts/containers/run_{go,node,docs,scan,playwright,testutil}.sh
# and their shared library runner_lib.sh. The wrappers are control-plane: they run on the host and start every container THROUGH run_pinned.sh.
# Two independent oracles must agree: (1) a run_pinned.sh SHIM (RUNNER_RUNP, test hook) that logs its argv and the RUNP_* limit environment it was
# given and the long-op registry state at the moment it was called, and (2) the REAL long-op registry (scripts/longops, fixture state) read back with jq.
# The anti-mess sweep is a shim too (RUNNER_SWEEP), except in the real-sweep leg that runs the real scripts/anti-mess/sweep.sh. A real leg at the end
# runs the real run_pinned.sh against the pinned images that exist on this host (IMG-TESTUTIL, IMG-GO, IMG-SHELLCHECK); images that are absent from the lock
# (a fixture lock without IMG-NODE / IMG-PW / IMG-DOCS; the real lock pins all three, T106) must be REFUSED honestly (image_not_in_lock), never faked.
# Paired mutations: copies of the containers directory whose runner_lib.sh has ONE expression changed (python str replace, exactly one occurrence); this
# body is re-run against each copy (RUNNER_SUT_DIR=<copy>, RUNNER_TEST_MUTANT=1) and every copy must make it FAIL. The `--memory` removal mutation (the
# limit is not handed to run_pinned.sh) is one of them.
# Usage: test_runners.sh           tests and mutations           RUNNER_TEST_NO_MUTATIONS=1 ...  tests only      RUNNER_TEST_NO_REAL=1 ...  skip the real container leg
# Env:   RUNNER_MUTATION_RECORD    file that receives one line per mutation (default: scratch)
# Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
SUTDIR="${RUNNER_SUT_DIR:-$HERE/..}"
FAILS=0; PASSES=0
# count_tok <dir> <token>: how many files in <dir> carry <token> in their name (a glob, never ls | grep)
count_tok() { local n=0 f; for f in "$1"/*"$2"*; do [ -e "$f" ] && n=$((n+1)); done; echo "$n"; }
ok()  { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required by this test"; exit 2; }; done
WRAPPERS="run_go run_node run_docs run_scan run_playwright run_testutil"
for w in $WRAPPERS; do [ -f "$SUTDIR/$w.sh" ] || { bad "$w.sh not found at $SUTDIR"; }; done
[ -f "$SUTDIR/runner_lib.sh" ] || bad "runner_lib.sh not found at $SUTDIR"
[ "$FAILS" = 0 ] || { echo "RESULT pass=$PASSES fail=$FAILS"; exit 1; }

T="$(mktemp -d "${TMPDIR:-/tmp}/runners-test.XXXXXX")"
trap 'rm -rf "$T"' EXIT
D1="sha256:$(printf 'a%.0s' $(seq 64))"
SHIMS="$T/shims"; mkdir -p "$SHIMS"
# --- shim for run_pinned.sh: logs argv, RUNP_* env and the registry states at call time; probe calls print the probe output ---
cat >"$SHIMS/run_pinned.sh" <<'SH'
#!/usr/bin/env bash
prev=""; for a in "$@"; do [ "$prev" = --out ] && mkdir -p -- "$a"; prev=$a; done   # RUNP creates the out dir
{
  echo "---CALL---"
  printf 'ARG:%s\n' "$@"
  printf 'ENV:RUNP_MEMORY=%s\nENV:RUNP_CPUS=%s\nENV:RUNP_PIDS=%s\n' "${RUNP_MEMORY-}" "${RUNP_CPUS-}" "${RUNP_PIDS-}"
  printf 'REG:%s\n' "$(for f in "${LONGOPS_DIR:-/nonexistent}"/ops/*.json; do [ -e "$f" ] && jq -r '.op_id + "=" + .state' "$f"; done 2>/dev/null | tr '\n' ' ')"
} >>"${SHIM_LOG:?}"
case " $* " in
  *cpa-probe*|*" --version "*) [ -z "${SHIM_PROBE_OUT:-}" ] || cat "$SHIM_PROBE_OUT"; exit "${SHIM_PROBE_RC:-0}";;
esac
[ -z "${SHIM_SLEEP:-}" ] || sleep "$SHIM_SLEEP"
echo "main-run-stdout"; echo "main-run-stderr" >&2
exit "${SHIM_RC:-0}"
SH
cat >"$SHIMS/sweep.sh" <<'SH'
#!/usr/bin/env bash
printf 'SWEEP:%s\n' "$*" >>"${SWEEP_LOG:?}"
echo "AM-P1 clean"; exit "${SHIM_SWEEP_RC:-0}"
SH
cat >"$SHIMS/podman" <<'SH'
#!/usr/bin/env bash
{ echo "---PODMAN---"; printf '%s\n' "$@"; } >>"${PODMAN_LOG:?}"
case "$*" in "image inspect"*) echo "${SHIM_INSPECT_DIGEST:-}"; exit 0;; esac
exit 0
SH
chmod +x "$SHIMS"/*

# --- fixtures: lock, probe output, meminfo, registry ---
LOCKF="$T/lock.yaml"
cat >"$LOCKF" <<EOF
schema: 1
images:
- id: IMG-GO
  reference: docker.io/example/golang
  tag_intent: "1.25"
  digest: "$D1"
- id: IMG-TESTUTIL
  reference: localhost/example-testutil
  tag_intent: abc
  digest: "$D1"
- id: IMG-SHELLCHECK
  reference: docker.io/example/shellcheck
  tag_intent: stable
  digest: "$D1"
  entrypoint_override: /bin/shellcheck
- id: IMG-SCAN-TRIVY
  reference: docker.io/example/trivy
  tag_intent: "0.75.0"
  digest: "$D1"
- id: IMG-UNPINNED
  reference: docker.io/example/unpinned
  tag_intent: latest
- id: IMG-BADDIGEST
  reference: docker.io/example/baddigest
  digest: "sha256:zz"
EOF
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
PROBE_OK="$T/probe_ok.txt"; printf 'version=go version go1.25.14 linux/amd64\nout_writable=yes\ncache_writable=yes\nsrc_readonly=yes\n' >"$PROBE_OK"
newreg() { rm -rf "$T/reg"; mkdir -p "$T/reg/repo/.audit"; export LONGOPS_REPO="$T/reg/repo" LONGOPS_DIR="$T/reg/repo/.audit/longops" LONGOPS_AUDIT="$T/reg/repo/.audit" LONGOPS_ALLOW_TMPFS=1; }
newreg
CK="$T/checkout"; mkdir -p "$CK"
export RUNNER_TEST_MODE=1 RUNNER_RUNP="$SHIMS/run_pinned.sh" RUNNER_SWEEP="$SHIMS/sweep.sh" RUNP_LOCK="$LOCKF" RUNNER_HEARTBEAT_S=1
export ENVELOPE_TEST_MODE=1 ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000
export SHIM_LOG="$T/run.log" SWEEP_LOG="$T/sweep.log" PODMAN_LOG="$T/podman.log" SHIM_PROBE_OUT="$PROBE_OK" SHIM_INSPECT_DIGEST="$D1"
export DISK_HEADROOM_OUT_DIR="$T/disk"; mkdir -p "$DISK_HEADROOM_OUT_DIR"   # the real legs' disk-headroom records go to scratch, never into the real evidence/disk
TOK="rt$$x$RANDOM"; RN=0
ORIG_PATH="$PATH"; export PATH="$SHIMS:$PATH"
# the expected envelope for the fixture host (hand computed, section 8.2): memory 19660800000 (ceiling 0.60*32768000000), cpus min(14, 9) = 9, pids 2048
EXP_MEM=19660800000; EXP_CPUS=9; EXP_PIDS=2048
resetlogs() { : >"$SHIM_LOG"; : >"$SWEEP_LOG"; : >"$PODMAN_LOG"; unset SHIM_RC SHIM_SLEEP SHIM_SWEEP_RC SHIM_PROBE_RC; export SHIM_PROBE_OUT="$PROBE_OK" SHIM_INSPECT_DIGEST="$D1"; }
# wr <wrapper> <args...>: run a wrapper in the checkout, stdout -> $T/stdout, stderr -> $T/stderr, sets RC
wr() { local w=$1; shift; ( cd "$CK" && bash "$SUTDIR/$w.sh" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
ncalls() { grep -c '^---CALL---$' "$SHIM_LOG"; }
# call <n>: the lines of the nth shim call (1 = the toolchain probe, 2 = the run)
call() { awk -v n="$1" '/^---CALL---$/{c++; next} c==n{print}' "$SHIM_LOG"; }
callarg() { call "$1" | grep -qxF -- "ARG:$2"; }
callenv() { call "$1" | sed -n "s/^ENV:$2=//p"; }
refused() { grep -q "REFUSED reason=$1" "$T/stderr"; }
opfiles() { ls "$LONGOPS_DIR"/ops/*.json 2>/dev/null; }
nops() { opfiles | wc -l | tr -d ' '; }
oneop() { opfiles | head -1; }

# ============== usage and argument handling (every wrapper) ==============
for w in $WRAPPERS; do
  resetlogs; newreg
  wr "$w"; check "$w: no arguments is a usage error" "$RC" 2
  wr "$w" --out "$T/o" ; check "$w: missing -- command is a usage error" "$RC" 2
  wr "$w" --bogus -- true; check "$w: unknown option is a usage error" "$RC" 2
  wr "$w" --memory abc -- true; check "$w: --memory must be a byte count" "$RC" 2
  wr "$w" --cpus 0 -- true; check "$w: --cpus must be a positive integer" "$RC" 2
  check "$w: usage errors start no container" "$(ncalls)" 0
  check "$w: usage errors register nothing" "$(nops)" 0
done

# ============== run_go / run_testutil / run_scan: the wrapper contract on the shims ==============
for w in run_go run_testutil; do
  case "$w" in run_go) IMG="IMG-GO" ;; run_testutil) IMG="IMG-TESTUTIL" ;; esac
  resetlogs; newreg; OUT="$T/out-$w"; rm -rf "$OUT"
  wr "$w" --out "$OUT" -- echo hello world
  check "$w: success exit 0" "$RC" 0
  check "$w: two containers go through run_pinned.sh (the toolchain probe, then the run)" "$(ncalls)" 2
  callarg 1 "$IMG" && callarg 2 "$IMG" && ok "$w: both calls name the image $IMG" || bad "$w: image id missing from the shim calls"
  callarg 2 "--out" && ok "$w: the run gets an explicit --out" || bad "$w: --out missing"
  check "$w: RUNP_MEMORY is the envelope memory (the limit is handed to run_pinned.sh)" "$(callenv 2 RUNP_MEMORY)" "$EXP_MEM"
  check "$w: RUNP_CPUS is the envelope cpus" "$(callenv 2 RUNP_CPUS)" "$EXP_CPUS"
  check "$w: RUNP_PIDS is the envelope pids" "$(callenv 2 RUNP_PIDS)" "$EXP_PIDS"
  check "$w: the probe carries the same limits" "$(callenv 1 RUNP_MEMORY)" "$EXP_MEM"
  call 2 | grep -qxF 'ARG:echo' && call 2 | grep -qxF 'ARG:world' && ok "$w: the user command is passed after --" || bad "$w: user command missing: $(call 2 | tr '\n' ' ')"
  # same op id (label) for the probe and the run, and the op id is a registry row
  OPA="$(call 1 | grep -A1 '^ARG:--op-id$' | tail -1 | sed 's/^ARG://')"; OPB="$(call 2 | grep -A1 '^ARG:--op-id$' | tail -1 | sed 's/^ARG://')"
  [ -n "$OPA" ] && [ "$OPA" = "$OPB" ] && ok "$w: probe and run carry one --op-id ($OPA)" || bad "$w: op ids differ ('$OPA' vs '$OPB')"
  check "$w: exactly one long-op is registered" "$(nops)" 1
  OPF="$(oneop)"
  check "$w: the registry row is the op id of the container label" "$(jq -r .op_id "$OPF")" "$OPA"
  check "$w: the op reached the terminal state complete" "$(jq -r .state "$OPF")" complete
  check "$w: the op budget records the envelope memory" "$(jq -r .budget.memory_bytes "$OPF")" "$EXP_MEM"
  check "$w: the op budget records the envelope cpus" "$(jq -r .budget.cpus "$OPF")" "$EXP_CPUS"
  check "$w: the op records its out dir as a declared write path" "$(jq -r '.write_paths[0]' "$OPF")" "$OUT"
  call 1 | grep -q "^REG:$OPA=registered" && ok "$w: the op was registered BEFORE the first container started" || bad "$w: op not registered at probe time: $(call 1 | grep '^REG:')"
  check "$w: the sweep ran once, before the run" "$(grep -c '^SWEEP:' "$SWEEP_LOG")" 1
  grep -q -- '--stage cadence' "$SWEEP_LOG" && grep -q -- '--only AM-P1,AM-P2,AM-P3' "$SWEEP_LOG" && ok "$w: the sweep reads the runtime plane (AM-P1,AM-P2,AM-P3) at the cadence stage" || bad "$w: sweep args: $(cat "$SWEEP_LOG")"
  # the toolchain record
  TR="$OUT/toolchain.json"
  [ -s "$TR" ] && jq -e . "$TR" >/dev/null 2>&1 && ok "$w: toolchain.json is written and valid JSON" || bad "$w: toolchain.json missing or invalid"
  check "$w: record schema" "$(jq -r .schema "$TR" 2>/dev/null)" "toolchain-record/1"
  check "$w: record wrapper" "$(jq -r .wrapper "$TR" 2>/dev/null)" "$w"
  check "$w: record image id" "$(jq -r .image_id "$TR" 2>/dev/null)" "$IMG"
  check "$w: record lock digest" "$(jq -r .lock_digest "$TR" 2>/dev/null)" "$D1"
  check "$w: record inspected digest equals the lock digest" "$(jq -r .inspected_digest "$TR" 2>/dev/null)" "$D1"
  check "$w: record digest_match" "$(jq -r .digest_match "$TR" 2>/dev/null)" true
  check "$w: record carries the version line from the probe" "$(jq -r .version "$TR" 2>/dev/null)" "go version go1.25.14 linux/amd64"
  check "$w: record control needle (read-only mount write must fail)" "$(jq -r .probes.src_readonly "$TR" 2>/dev/null)" yes
  check "$w: record out writable" "$(jq -r .probes.out_writable "$TR" 2>/dev/null)" yes
  check "$w: record op id" "$(jq -r .op_id "$TR" 2>/dev/null)" "$OPA"
  check "$w: record envelope jobs (per_job UNKNOWN: 1)" "$(jq -r .envelope.jobs "$TR" 2>/dev/null)" 1
  check "$w: record envelope memory" "$(jq -r .envelope.memory_bytes "$TR" 2>/dev/null)" "$EXP_MEM"
  grep -q 'main-run-stdout' "$T/stdout" && ok "$w: run stdout reaches the caller" || bad "$w: stdout lost"
  grep -q 'main-run-stderr' "$T/stderr" && ok "$w: run stderr reaches the caller's stderr" || bad "$w: stderr lost"
  # exit code passthrough, op failed
  resetlogs; newreg; export SHIM_RC=7
  wr "$w" --out "$T/o2" -- false
  check "$w: the container exit code is passed through" "$RC" 7
  check "$w: a failed run ends the op in state failed" "$(jq -r .state "$(oneop)")" failed
  check "$w: the verdict names the exit code" "$(jq -r .verdict "$(oneop)")" "rc=7"
  unset SHIM_RC
done
# run_go prefixes the user command with the Go limits of docs/16 6.2 (GOMAXPROCS=3, local toolchain, CGO for the race detector)
resetlogs; newreg
wr run_go --out "$T/o3" -- go test -race ./...
call 2 | tr '\n' ' ' | grep -q 'ARG:env ARG:GOMAXPROCS=3 ARG:GOTOOLCHAIN=local ARG:CGO_ENABLED=1 ARG:go ARG:test ARG:-race ARG:\./\.\.\.' && ok "run_go: the command is prefixed env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1" || bad "run_go prefix: $(call 2 | tr '\n' ' ')"
# run_testutil adds no prefix
resetlogs; newreg
wr run_testutil --out "$T/o3b" -- python3 -c 'print(1)'
call 2 | tr '\n' ' ' | grep -q -- '-- ARG:IMG-TESTUTIL ARG:--' || true
call 2 | grep -qxF 'ARG:env' && bad "run_testutil: unexpected env prefix" || ok "run_testutil: no command prefix"

# ============== explicit limits: lower is honoured, above the envelope is refused ==============
resetlogs; newreg
wr run_go --out "$T/o4" --memory 5000000000 --cpus 3 -- true
check "explicit lower limits: exit 0" "$RC" 0
check "explicit lower memory reaches run_pinned.sh" "$(callenv 2 RUNP_MEMORY)" 5000000000
check "explicit lower cpus reaches run_pinned.sh" "$(callenv 2 RUNP_CPUS)" 3
check "the op budget records the explicit memory" "$(jq -r .budget.memory_bytes "$(oneop)")" 5000000000
resetlogs; newreg
wr run_go --out "$T/o5" --memory $((EXP_MEM + 1)) -- true
check "memory above the envelope: refused (exit 1)" "$RC" 1
refused limit_exceeds_envelope && ok "memory above the envelope: reason limit_exceeds_envelope" || bad "reason: $(cat "$T/stderr")"
check "memory above the envelope: no container, no op" "$(ncalls)/$(nops)" "0/0"
resetlogs; newreg
wr run_go --out "$T/o6" --cpus $((EXP_CPUS + 1)) -- true
check "cpus above the envelope: refused" "$RC" 1
refused limit_exceeds_envelope && ok "cpus above the envelope: reason limit_exceeds_envelope" || bad "reason: $(cat "$T/stderr")"
# there is no allowance above the live reading any more (review round 2 I1; it was arithmetically 0 after the F2 fix): host where MemAvailable binds (20000000 kB: budget 15564800000, the reserve starts above it)
printf 'MemTotal:       32000000 kB\nMemAvailable:   20000000 kB\n' >"$T/meminfo"
resetlogs; newreg
wr run_go --out "$T/o6b" --memory 15564800001 -- true
check "memory 1 byte above the live envelope would enter the MemAvailable reserve: refused (the limits are the one reading)" "$RC" 1
refused limit_exceeds_envelope && ok "reserve entry: reason limit_exceeds_envelope" || bad "reason: $(cat "$T/stderr")"
resetlogs; newreg
wr run_go --out "$T/o6b2" --memory 15564800000 -- true
check "memory exactly the live envelope (MemAvailable binds): accepted" "$RC" 0
check "the accepted value reaches run_pinned.sh" "$(callenv 2 RUNP_MEMORY)" 15564800000
resetlogs; newreg
wr run_go --out "$T/o6c" --memory 15876096001 -- true
check "memory 2% above the live envelope: refused (no allowance above the reading)" "$RC" 1
refused limit_exceeds_envelope && ok "above the slack: reason limit_exceeds_envelope" || bad "reason: $(cat "$T/stderr")"
resetlogs; newreg
wr run_go --out "$T/o6d" --cpus 10 -- true
check "cpus 1 above the live envelope: refused" "$RC" 1
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
# registered long operations shrink the envelope the wrapper hands out (docs/16 8.1): one live op with 2e9 memory and 9 cpus
resetlogs; newreg
sleep 600 >/dev/null 2>&1 & HOLD=$!
bash "$REPO/scripts/longops/register.sh" --purpose other:held --owner t120 --pid "$HOLD" --memory-bytes 2000000000 --cpus 9 --no-progress-s 600 >/dev/null 2>&1
wr run_go --out "$T/o7" -- true
check "a registered live op shrinks the memory limit" "$(callenv 2 RUNP_MEMORY)" 17660800000
check "a registered live op shrinks the cpus limit (cpu_budget 5)" "$(callenv 2 RUNP_CPUS)" 5
# the same purpose is single-owner (11.4.232 B): a second run with a live holder of the purpose is refused before any container
resetlogs
bash "$REPO/scripts/longops/register.sh" --purpose t120:same --owner t120 --pid "$HOLD" >/dev/null 2>&1
wr run_go --out "$T/o8" --purpose t120:same -- true
check "purpose held by a live op: refused" "$RC" 1
refused purpose_conflict && ok "purpose held: reason purpose_conflict" || bad "reason: $(cat "$T/stderr")"
check "purpose held: no container started" "$(ncalls)" 0
[[ "$HOLD" =~ ^[0-9]+$ && "$HOLD" -gt 1 ]] && kill "$HOLD" 2>/dev/null

# ============== refusals: unpinned image, image not in the lock, foreign image ==============
resetlogs; newreg
sed -i 's/IMG-GO$/IMG-GO/' "$LOCKF"
cp "$LOCKF" "$T/lock.keep"
# an image whose lock entry has no digest (UNPINNED) or a malformed digest is refused before anything starts
python3 -I - "$LOCKF" <<'PY'
import sys, re
p = sys.argv[1]
s = open(p).read()
# make IMG-GO unpinned: drop its digest line
s = re.sub(r'(- id: IMG-GO\n  reference: [^\n]*\n  tag_intent: [^\n]*\n)  digest: "[^"]*"\n', r'\1', s)
open(p, "w").write(s)
PY
wr run_go --out "$T/o9" -- true
check "unpinned image (no digest): refused" "$RC" 1
refused image_unpinned && ok "unpinned image: reason image_unpinned" || bad "reason: $(cat "$T/stderr")"
check "unpinned image: no container, no op, no sweep" "$(ncalls)/$(nops)/$(grep -c '^SWEEP:' "$SWEEP_LOG")" "0/0/0"
cp "$T/lock.keep" "$LOCKF"
python3 -I - "$LOCKF" "$D1" <<'PY'
import sys
p, d = sys.argv[1], sys.argv[2]
s = open(p).read().replace('digest: "%s"\n- id: IMG-TESTUTIL' % d, 'digest: "sha256:zz"\n- id: IMG-TESTUTIL', 1)
open(p, "w").write(s)
PY
resetlogs; newreg
wr run_go --out "$T/o10" -- true
check "malformed digest: refused" "$RC" 1
refused image_unpinned && ok "malformed digest: reason image_unpinned" || bad "reason: $(cat "$T/stderr")"
cp "$T/lock.keep" "$LOCKF"
# the three wrappers whose image is absent from the FIXTURE lock: honest blocked, never a faked run
for pair in run_node:IMG-NODE:T106 run_playwright:IMG-PW:T106 run_docs:IMG-DOCS:T106; do
  w="${pair%%:*}"; rest="${pair#*:}"; IMG="${rest%%:*}"
  resetlogs; newreg
  wr "$w" --out "$T/o11" -- true
  check "$w: image $IMG absent from the lock is refused (exit 1)" "$RC" 1
  refused image_not_in_lock && ok "$w: reason image_not_in_lock" || bad "$w: reason: $(cat "$T/stderr")"
  grep -q "BLOCKED" "$T/stderr" && grep -q "$IMG" "$T/stderr" && ok "$w: the refusal says BLOCKED and names $IMG" || bad "$w: blocked message: $(cat "$T/stderr")"
  check "$w: blocked starts no container, registers no op, runs no sweep" "$(ncalls)/$(nops)/$(grep -c '^SWEEP:' "$SWEEP_LOG")" "0/0/0"
done
# a lock entry that is present makes the same wrapper runnable (the refusal is the lock's state, not a hard-coded block)
cat >>"$LOCKF" <<EOF
- id: IMG-NODE
  reference: docker.io/example/node
  tag_intent: "20"
  digest: "$D1"
EOF
resetlogs; newreg; printf 'version=v20.0.0\nout_writable=yes\ncache_writable=yes\nsrc_readonly=yes\n' >"$T/probe_node.txt"; export SHIM_PROBE_OUT="$T/probe_node.txt"
wr run_node --out "$T/o12" -- node --version
check "run_node with IMG-NODE in the lock: runs" "$RC" 0
callarg 2 IMG-NODE && ok "run_node: the run names IMG-NODE" || bad "run_node: image missing"
cp "$T/lock.keep" "$LOCKF"
# run_scan: default IMG-SHELLCHECK (entrypoint override: the first command word names the entrypoint), --image restricted to scanner images
resetlogs; newreg; printf 'ShellCheck - shell script analysis tool\nversion: 0.10.0\n' >"$T/probe_sc.txt"; export SHIM_PROBE_OUT="$T/probe_sc.txt"
wr run_scan --out "$T/o13" -- shellcheck -x /src/scripts/containers/run_pinned.sh
check "run_scan: default image IMG-SHELLCHECK runs" "$RC" 0
callarg 2 IMG-SHELLCHECK && ok "run_scan: names IMG-SHELLCHECK" || bad "run_scan: image missing"
check "run_scan: the record states no shell probe is possible (entrypoint-only image)" "$(jq -r .probes.src_readonly "$T/o13/toolchain.json" 2>/dev/null)" "n/a:no_shell"
check "run_scan: the record carries the version line, not the banner (review round 2 m5: shellcheck prints its banner first)" "$(jq -r .version "$T/o13/toolchain.json" 2>/dev/null)" "version: 0.10.0"
resetlogs; newreg; printf 'ShellCheck - shell script analysis tool\n' >"$T/probe_sc0.txt"; export SHIM_PROBE_OUT="$T/probe_sc0.txt"
wr run_scan --out "$T/o13c" -- shellcheck -x /src/scripts/containers/run_pinned.sh
check "run_scan: a probe output with no line carrying a version number is refused (the banner is not a version)" "$RC" 1
refused probe_blind && ok "run_scan: reason probe_blind for a banner-only probe" || bad "run_scan: banner-only reason: $(cat "$T/stderr")"
resetlogs; newreg; export SHIM_PROBE_OUT="$T/probe_sc.txt"
resetlogs; newreg
wr run_scan --out "$T/o13b" --image IMG-SCAN-TRIVY -- trivy --version
check "run_scan: an allowed scanner image with no defined version command is refused (blocked, never guessed)" "$RC" 1
refused probe_not_defined && ok "run_scan: reason probe_not_defined" || bad "reason: $(cat "$T/stderr")"
check "run_scan: probe_not_defined starts no container and the op ended failed" "$(ncalls)/$(jq -r .state "$(oneop)")" "0/failed"
resetlogs; newreg
wr run_scan --out "$T/o14" --image IMG-GO -- true
check "run_scan: a non-scanner image is refused (exit 1)" "$RC" 1
refused image_not_allowed && ok "run_scan: reason image_not_allowed" || bad "reason: $(cat "$T/stderr")"
resetlogs; newreg
wr run_go --out "$T/o14b" --image IMG-TESTUTIL -- true
check "run_go: --image is not an option of a single-image wrapper (usage)" "$RC" 2

# ============== the anti-mess sweep gate ==============
resetlogs; newreg; export SHIM_SWEEP_RC=10
wr run_go --out "$T/o15" -- true
check "sweep drift (rc 10): the run is refused" "$RC" 1
refused anti_mess_drift && ok "sweep drift: reason anti_mess_drift" || bad "reason: $(cat "$T/stderr")"
check "sweep drift: no container, no op" "$(ncalls)/$(nops)" "0/0"
resetlogs; newreg; export SHIM_SWEEP_RC=20
wr run_go --out "$T/o16" -- true
check "sweep refusal/blind (rc 20): refused" "$RC" 1
refused anti_mess_blind && ok "sweep rc 20: reason anti_mess_blind" || bad "reason: $(cat "$T/stderr")"
check "sweep rc 20: no container, no op" "$(ncalls)/$(nops)" "0/0"
resetlogs; newreg; export SHIM_SWEEP_RC=99
wr run_go --out "$T/o16b" -- true
check "sweep unexpected rc (99): refused (fail closed)" "$RC" 1
refused anti_mess_blind && ok "sweep unexpected rc: reason anti_mess_blind" || bad "reason: $(cat "$T/stderr")"
resetlogs; newreg
( cd "$CK" && RUNNER_SWEEP="$T/no-such-sweep.sh" bash "$SUTDIR/run_go.sh" --out "$T/o17" -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
check "sweep script missing: refused (fail closed)" "$RC" 1
refused anti_mess_sweep_missing && ok "sweep missing: reason anti_mess_sweep_missing" || bad "reason: $(cat "$T/stderr")"
# the test hooks are honoured only in a declared test run
resetlogs; newreg
( cd "$CK" && env -u RUNNER_TEST_MODE bash "$SUTDIR/run_go.sh" --out "$T/o18" -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
check "RUNNER_RUNP / RUNNER_SWEEP without RUNNER_TEST_MODE: refused" "$RC" 1
refused test_hook_outside_test_mode && ok "hook reason test_hook_outside_test_mode" || bad "reason: $(cat "$T/stderr")"
check "hooks outside test mode: nothing started" "$(ncalls)" 0
for hv in RUNNER_SWEEP RUNNER_RUNP; do
  resetlogs; newreg
  if [ "$hv" = RUNNER_SWEEP ]; then ( cd "$CK" && env -u RUNNER_TEST_MODE -u RUNNER_RUNP bash "$SUTDIR/run_go.sh" --out "$T/o18$hv" -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
  else ( cd "$CK" && env -u RUNNER_TEST_MODE -u RUNNER_SWEEP bash "$SUTDIR/run_go.sh" --out "$T/o18$hv" -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?; fi
  check "the $hv hook alone without RUNNER_TEST_MODE: refused (exit 1)" "$RC" 1
  refused test_hook_outside_test_mode && ok "the $hv hook alone: reason test_hook_outside_test_mode" || bad "the $hv hook alone reason: $(cat "$T/stderr")"
done
# an empty inspected digest never passes (a podman that printed nothing reads as the lock digest only when the lock has an empty platform digest)
resetlogs; newreg; export SHIM_INSPECT_DIGEST=""
wr run_go --out "$T/o18e" -- true
check "an empty inspected digest: refused (exit 1)" "$RC" 1
refused image_digest_mismatch && ok "empty inspected digest: reason image_digest_mismatch" || bad "empty digest reason: $(cat "$T/stderr")"
export SHIM_INSPECT_DIGEST="$D1"

# ============== the toolchain probe: a probe that cannot detect a failure is itself detected (11.4.201) ==============
resetlogs; newreg; printf 'version=x\nout_writable=yes\ncache_writable=yes\nsrc_readonly=no\n' >"$T/probe_blind.txt"; export SHIM_PROBE_OUT="$T/probe_blind.txt"
wr run_go --out "$T/o19" -- true
check "control needle failed (write to the read-only mount succeeded): refused" "$RC" 1
refused probe_blind && ok "reason probe_blind" || bad "reason: $(cat "$T/stderr")"
check "probe_blind: the run did not start (only the probe call)" "$(ncalls)" 1
check "probe_blind: the op ended failed" "$(jq -r .state "$(oneop)")" failed
resetlogs; newreg; printf 'version=x\nout_writable=no\ncache_writable=yes\nsrc_readonly=yes\n' >"$T/probe_ro.txt"; export SHIM_PROBE_OUT="$T/probe_ro.txt"
wr run_go --out "$T/o20" -- true
check "/out not writable by the mapped user: refused" "$RC" 1
refused out_not_writable && ok "reason out_not_writable" || bad "reason: $(cat "$T/stderr")"
resetlogs; newreg; printf 'version=x\nout_writable=yes\ncache_writable=no\nsrc_readonly=yes\n' >"$T/probe_nc.txt"; export SHIM_PROBE_OUT="$T/probe_nc.txt"
wr run_go --out "$T/o20b" -- true
check "cache dir not writable: refused" "$RC" 1
refused cache_not_writable && ok "reason cache_not_writable" || bad "reason: $(cat "$T/stderr")"
resetlogs; newreg; : >"$T/probe_empty.txt"; export SHIM_PROBE_OUT="$T/probe_empty.txt"
wr run_go --out "$T/o21" -- true
check "an empty probe output (the instrument saw nothing): refused" "$RC" 1
refused probe_blind && ok "empty probe: reason probe_blind" || bad "reason: $(cat "$T/stderr")"
resetlogs; newreg; export SHIM_PROBE_RC=3
wr run_go --out "$T/o21b" -- true
check "the probe container failing: refused" "$RC" 1
refused probe_failed && ok "reason probe_failed" || bad "reason: $(cat "$T/stderr")"
resetlogs; newreg
SHIM_INSPECT_DIGEST="sha256:$(printf 'c%.0s' $(seq 64))"; export SHIM_INSPECT_DIGEST
wr run_go --out "$T/o22" -- true
check "inspected image digest differs from the lock digest: refused" "$RC" 1
refused image_digest_mismatch && ok "reason image_digest_mismatch" || bad "reason: $(cat "$T/stderr")"
check "digest mismatch: the run did not start" "$(ncalls)" 0

# ============== --network=none, --rw and --need are handed through ==============
resetlogs; newreg
wr run_go --out "$T/o23" --network=none --rw docs --need 123 -- true
check "passthrough: exit 0" "$RC" 0
callarg 2 --network=none && callarg 2 --rw && callarg 2 docs && callarg 2 --need && callarg 2 123 && ok "passthrough: --network=none --rw docs --need 123 reach run_pinned.sh" || bad "passthrough: $(call 2 | tr '\n' ' ')"

# ============== liveness: the heartbeat advances while the run is alive ==============
resetlogs; newreg; export SHIM_SLEEP=3
wr run_go --out "$T/o24" -- true
[ "$(jq -r .heartbeat_seq "$(oneop)")" -ge 1 ] && ok "a 3 s run records at least one heartbeat (seq $(jq -r .heartbeat_seq "$(oneop)"))" || bad "no heartbeat recorded: $(jq -c . "$(oneop)")"
unset SHIM_SLEEP

# ============== the real-sweep leg: the wrapper follows the real scripts/anti-mess/sweep.sh verdict ==============
resetlogs; newreg
( cd "$REPO" && bash scripts/anti-mess/sweep.sh --stage cadence --only AM-P1,AM-P2,AM-P3 ) >"$T/realsweep.txt" 2>&1; SWRC=$?
( cd "$CK" && RUNNER_SWEEP="$REPO/scripts/anti-mess/sweep.sh" ANTIMESS_ROOT="$REPO" bash "$SUTDIR/run_go.sh" --out "$T/o25" -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
case "$SWRC" in
  0)  check "real sweep clean (rc 0): the wrapper runs" "$RC" 0;;
  10) check "real sweep reports drift (rc 10): the wrapper refuses" "$RC" 1; refused anti_mess_drift && ok "real sweep drift: reason anti_mess_drift" || bad "reason: $(cat "$T/stderr")";;
  *)  check "real sweep refusal (rc $SWRC): the wrapper refuses" "$RC" 1;;
esac
echo "  (real sweep verdict for this run: rc=$SWRC)"

# ============== the real container leg: real run_pinned.sh, real pinned images present on this host ==============
if [ "${RUNNER_TEST_NO_REAL:-0}" != 1 ]; then
  # the sweep stays the shim (its container census would see other agents' labelled containers that have no row in THIS fixture registry); the
  # real-sweep leg above proves the wiring to the real sweep. Everything else is real: PATH without the shims, the real lock, the real /proc (ENVELOPE_TEST_MODE stays: it only
  # permits the scratch long-op registry; the three reading hooks are removed).
  realrun() { local w=$1 o=$2; shift 2; RN=$((RN+1)); ( cd "$REPO" && env -u RUNNER_RUNP -u RUNP_LOCK -u ENVELOPE_MEMINFO -u ENVELOPE_NPROC -u ENVELOPE_ULIMIT_U PATH="$ORIG_PATH" bash "$SUTDIR/$w.sh" --out "$o" --op-id "$TOK-$RN" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
  resetlogs; newreg
  REALOUT="$T/real-testutil"
  realrun run_testutil "$REALOUT" -- python3 -c 'import yaml, jsonschema, pytest; print("real-testutil-ok")'
  check "real: run_testutil in the pinned IMG-TESTUTIL exits 0" "$RC" 0
  grep -q 'real-testutil-ok' "$T/stdout" && ok "real: the python3 import ran inside the container" || bad "real: output: $(cat "$T/stdout" "$T/stderr" | head -5)"
  check "real: toolchain.json digest_match" "$(jq -r '.digest_match' "$REALOUT/toolchain.json" 2>/dev/null)" true
  check "real: control needle yes in the real container" "$(jq -r '.probes.src_readonly' "$REALOUT/toolchain.json" 2>/dev/null)" yes
  check "real: the real op ended complete" "$(jq -r .state "$(oneop)")" complete
  REALOUT2="$T/real-go"
  realrun run_go "$REALOUT2" -- go version
  check "real: run_go in the pinned IMG-GO exits 0" "$RC" 0
  grep -q 'go version go1' "$T/stdout" && ok "real: go version printed by the toolchain inside IMG-GO" || bad "real: $(cat "$T/stdout" "$T/stderr" | head -5)"
  check "real: IMG-GO toolchain.json digest_match" "$(jq -r '.digest_match' "$REALOUT2/toolchain.json" 2>/dev/null)" true
  check "real: IMG-GO the source mount is read-only (needle)" "$(jq -r '.probes.src_readonly' "$REALOUT2/toolchain.json" 2>/dev/null)" yes
  REALOUT3="$T/real-scan"
  realrun run_scan "$REALOUT3" --network=none -- shellcheck --version
  check "real: run_scan in the pinned IMG-SHELLCHECK exits 0" "$RC" 0
  grep -qi 'version:' "$T/stdout" && ok "real: shellcheck --version printed" || bad "real: $(cat "$T/stdout" "$T/stderr" | head -5)"
  # the images of run_node / run_playwright / run_docs: refused honestly while the real lock has no entry (another task pins them), and run for real,
  # one version smoke each, once the lock has the entry; the leg reads the lock, so it stays true as the lock grows
  lock_has() { python3 -I - "$REPO/build/containers/images.lock.yaml" "$1" <<'PY2'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
sys.exit(0 if any(isinstance(i, dict) and i.get("id") == sys.argv[2] for i in d.get("images", [])) else 1)
PY2
  }
  for tri in "run_node:IMG-NODE:node --version:v" "run_playwright:IMG-PW:npx playwright --version:Version" "run_docs:IMG-DOCS:python3 --version:Python"; do
    w="${tri%%:*}"; rest="${tri#*:}"; IMG="${rest%%:*}"; rest="${rest#*:}"; vcmd="${rest%%:*}"; pat="${rest#*:}"
    if lock_has "$IMG"; then
      # shellcheck disable=SC2086
      realrun "$w" "$T/real-$w" -- $vcmd
      check "real lock has $IMG: $w runs '$vcmd' in the pinned image (exit 0)" "$RC" 0
      grep -q "$pat" "$T/stdout" && ok "real: $w printed the toolchain version ($(head -1 "$T/stdout"))" || bad "real: $w output: $(head -3 "$T/stdout" "$T/stderr")"
      check "real: $w toolchain record digest_match" "$(jq -r '.digest_match' "$T/real-$w/toolchain.json" 2>/dev/null)" true
    else
      realrun "$w" "$T/real-$w" -- true
      check "real lock has no $IMG: $w is refused (exit 1)" "$RC" 1
      refused image_not_in_lock && ok "real lock: $w reason image_not_in_lock (BLOCKED, nothing faked)" || bad "real lock: $w: $(cat "$T/stderr")"
    fi
  done
fi

# ============== the real evidence tree is untouched by the real legs ==============
if [ "${RUNNER_TEST_NO_REAL:-0}" != 1 ]; then
  EVD="$REPO/specs/001-full-project-audit-remediation/evidence/disk"
  check "no disk-headroom record carrying this run's token ($TOK) was written into the real evidence/disk" "$(count_tok "$EVD" "$TOK")" 0
  [ "$(count_tok "$DISK_HEADROOM_OUT_DIR" "$TOK")" -ge 1 ] && ok "the real legs' records went to DISK_HEADROOM_OUT_DIR in scratch ($(count_tok "$DISK_HEADROOM_OUT_DIR" "$TOK"))" || bad "no record in the scratch DISK_HEADROOM_OUT_DIR"
fi

echo "RESULT pass=$PASSES fail=$FAILS"
[ "$FAILS" = 0 ] || EXIT=1
EXIT="${EXIT:-0}"

# ============== paired mutations ==============
if [ "${RUNNER_TEST_MUTANT:-0}" = 1 ] || [ "${RUNNER_TEST_NO_MUTATIONS:-0}" = 1 ]; then exit "$EXIT"; fi
MREC="${RUNNER_MUTATION_RECORD:-$T/mutation.txt}"; : >"$MREC"
CAUGHT=0; SURV=0; TOTAL=0
mut_case() { # <id> <old> <new>
  local id=$1 old=$2 new=$3 d="$T/mut-$1"
  TOTAL=$((TOTAL+1))
  rm -rf "$d"; mkdir -p "$d/scripts" "$d/tools"
  cp -r "$SUTDIR" "$d/scripts/containers" 2>/dev/null; rm -rf "$d/scripts/containers/tests"
  for l in longops anti-mess; do ln -s "$REPO/scripts/$l" "$d/scripts/$l"; done
  ln -s "$REPO/build" "$d/build"
  python3 -I - "$d/scripts/containers/runner_lib.sh" "$old" "$new" <<'PY' || { echo "INVALID $id: pattern not found exactly once" | tee -a "$MREC"; SURV=$((SURV+1)); return; }
import sys
s = open(sys.argv[1]).read()
if sys.argv[2] and s.count(sys.argv[2]) != 1: sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]) if sys.argv[2] else s)
PY
  ( RUNNER_SUT_DIR="$d/scripts/containers" RUNNER_TEST_MUTANT=1 RUNNER_TEST_NO_REAL=1 QUIET=1 bash "${BASH_SOURCE[0]}" ) >"$T/mut-$id.log" 2>&1; local rc=$?
  if [ "$id" = CONTROL ]; then   # the negative control (review round 2 m4): an UNMUTATED copy placed like a mutant must pass the whole body
    if [ "$rc" = 0 ]; then echo "CONTROL  an unmutated copy placed like a mutant passes the body ($(grep '^RESULT' "$T/mut-$id.log"))" | tee -a "$MREC"; TOTAL=$((TOTAL-1))
    else echo "CONTROL FAILED: the unmutated copy fails the body ($(grep -c '^FAIL' "$T/mut-$id.log") checks): the mutation harness is blind" | tee -a "$MREC"; SURV=$((SURV+1)); fi
    return
  fi
  if [ "$rc" -ne 0 ]; then CAUGHT=$((CAUGHT+1)); echo "CAUGHT   $id ($(grep -c '^FAIL' "$T/mut-$id.log") failing checks)" | tee -a "$MREC"
  else SURV=$((SURV+1)); echo "SURVIVED $id (test stayed green on the mutant)" | tee -a "$MREC"; fi
}
mut_case CONTROL '' ''
mut_case no-memory-limit 'RUNP_MEMORY="$LIM_MEM"' 'RUNP_MEMORY_DROPPED="$LIM_MEM"'
mut_case no-cpus-limit 'RUNP_CPUS="$LIM_CPUS"' 'RUNP_CPUS_DROPPED="$LIM_CPUS"'
mut_case no-pids-limit 'RUNP_PIDS="$LIM_PIDS"' 'RUNP_PIDS_DROPPED="$LIM_PIDS"'
mut_case limit-above-envelope '[ "$LIM_MEM" -le "$ENV_MEM" ]' 'true'
mut_case cpus-above-envelope '[ "$LIM_CPUS" -le "$ENV_CPUS" ]' 'true'
mut_case sweep-drift-accepted 'if [ "$SW_RC" = 10 ]' 'if [ "$SW_RC" = 9999 ]'
mut_case sweep-blind-accepted '[ "$SW_RC" = 0 ] || rl_refuse anti_mess_blind' 'true'
mut_case sweep-missing-accepted '[ -f "$SWEEP" ] || rl_refuse anti_mess_sweep_missing' 'true'
mut_case sweep-skipped 'rl_sweep_gate   # MUT:sweep-gate' ':'
mut_case unpinned-accepted 'rl_refuse image_unpinned "id=$IMG has no sha256 digest in the lock"' 'true'
mut_case not-in-lock-accepted 'rl_refuse image_not_in_lock "BLOCKED: $IMG is not in the lock' 'true "BLOCKED: $IMG is not in the lock'
mut_case image-not-allowed-accepted 'rl_refuse image_not_allowed' 'true'
mut_case register-skipped 'bash "$ROOT_DIR/scripts/longops/register.sh"' 'true'
mut_case purpose-conflict-ignored 'rl_refuse purpose_conflict "$REG_ERR"   # MUT:purpose-conflict' 'true'
mut_case needle-unchecked '[ "$P_SRC" = yes ] || rl_fail_op probe_blind' 'true'
mut_case out-writable-unchecked '[ "$P_OUT" = yes ] || rl_fail_op out_not_writable' 'true'
mut_case cache-writable-unchecked '[ "$P_CACHE" = yes ] || rl_fail_op cache_not_writable' 'true'
mut_case probe-not-defined-ignored '[ -n "$PV_CMD" ] || rl_fail_op probe_not_defined' 'true'
mut_case probe-rc-ignored '[ "$PROBE_RC" = 0 ] || rl_fail_op probe_failed' 'true'
mut_case digest-unchecked '[ "$INSPECTED" = "$DIGEST" ] || rl_refuse image_digest_mismatch' 'true'
mut_case rc-masked 'rl_release_op "$RUN_RC"' 'rl_release_op 0'
mut_case exit-masked 'exit "$RUN_RC"   # MUT:exit' 'exit 0'
mut_case no-heartbeat 'bash "$ROOT_DIR/scripts/longops/heartbeat.sh" --op-id "$OP_ID" --progress-offset "$(rl_progress)" --elapsed-ms "$el" >/dev/null 2>&1; rc=$?' 'true; rc=$?'
mut_case test-hooks '[ "${RUNNER_TEST_MODE:-}" != 1 ]; then' '[ "${RUNNER_TEST_MODE:-}" != 1 ] && false; then'
mut_case R9-empty-digest-accepted '[ -n "$PDIGEST" ] && [ "$INSPECTED" = "$PDIGEST" ]' '[ "$INSPECTED" = "$PDIGEST" ]'
mut_case R10-sweep-hook-gate ' || [ -n "${RUNNER_SWEEP+x}" ]' ''
mut_case version-banner-recorded 'grep -m1 -E '"'"'[0-9]+\.[0-9]+'"'"' || true' 'head -n 1'
mut_case prefix-dropped '${RUNNER_CMD_PREFIX:-}' '${RUNNER_CMD_PREFIX_DROPPED:-}'
echo "MUTATION RESULT caught=$CAUGHT survived=$SURV total=$TOTAL" | tee -a "$MREC"
[ "$SURV" = 0 ] || EXIT=1
exit "$EXIT"
