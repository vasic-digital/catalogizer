#!/usr/bin/env bash
# test_run_qa.sh - T212 (RED first: scripts/containers/run_qa.sh is absent). Oracle for the QA wrapper scripts/containers/run_qa.sh (IMG-QA, doc12 18.3).
# Same two-oracle method as test_runners.sh: a run_pinned.sh SHIM (RUNNER_RUNP, test hook) that logs argv, the RUNP_* limits and the long-op registry state at call time,
# and the REAL long-op registry read back with jq; the anti-mess sweep is a shim (RUNNER_SWEEP). Legs:
#   usage errors; a normal run (image IMG-QA, envelope limits, one registered op, toolchain.json, user command passed through, exit code passthrough);
#   REFUSED image_unpinned (lock entry without digest), REFUSED image_digest_mismatch (the altered-digest case: podman reports another digest),
#   REFUSED image_not_in_lock (and, on the REAL lock, honest BLOCKED-until-T211 status read from the lock itself);
#   the anti-mess drift refusal; a paired mutation: a copy of run_qa.sh whose image is IMG-GO must make this body FAIL (the wrapper may only ever start IMG-QA).
# Usage: bash scripts/containers/tests/test_run_qa.sh [RUNNER_TEST_NO_MUTATIONS=1 to skip the mutation leg]. Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
SUTDIR="${RUNNER_SUT_DIR:-$HERE/..}"
FAILS=0; PASSES=0
ok()  { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required by this test"; exit 2; }; done
[ -f "$SUTDIR/run_qa.sh" ] || { echo "FAIL: run_qa.sh not found at $SUTDIR (T212 not implemented)"; echo "RESULT pass=0 fail=1"; exit 1; }
[ -f "$SUTDIR/runner_lib.sh" ] || { echo "FAIL: runner_lib.sh not found at $SUTDIR"; exit 1; }

T="$(mktemp -d "${TMPDIR:-/tmp}/run-qa-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
D1="sha256:$(printf 'b%.0s' $(seq 64))"; D2="sha256:$(printf 'c%.0s' $(seq 64))"
SHIMS="$T/shims"; mkdir -p "$SHIMS"
cat >"$SHIMS/run_pinned.sh" <<'SH'
#!/usr/bin/env bash
prev=""; for a in "$@"; do [ "$prev" = --out ] && mkdir -p -- "$a"; prev=$a; done
{ echo "---CALL---"; printf 'ARG:%s\n' "$@"; printf 'ENV:RUNP_MEMORY=%s\nENV:RUNP_CPUS=%s\nENV:RUNP_PIDS=%s\n' "${RUNP_MEMORY-}" "${RUNP_CPUS-}" "${RUNP_PIDS-}"
  printf 'REG:%s\n' "$(for f in "${LONGOPS_DIR:-/nonexistent}"/ops/*.json; do [ -e "$f" ] && jq -r '.op_id + "=" + .state' "$f"; done 2>/dev/null | tr '\n' ' ')"; } >>"${SHIM_LOG:?}"
case " $* " in *cpa-probe*|*" --version "*) [ -z "${SHIM_PROBE_OUT:-}" ] || cat "$SHIM_PROBE_OUT"; exit "${SHIM_PROBE_RC:-0}";; esac
echo "main-run-stdout"; echo "main-run-stderr" >&2; exit "${SHIM_RC:-0}"
SH
cat >"$SHIMS/sweep.sh" <<'SH'
#!/usr/bin/env bash
printf 'SWEEP:%s\n' "$*" >>"${SWEEP_LOG:?}"; echo "AM-P1 clean"; exit "${SHIM_SWEEP_RC:-0}"
SH
cat >"$SHIMS/podman" <<'SH'
#!/usr/bin/env bash
case "$*" in "image inspect"*) echo "${SHIM_INSPECT_DIGEST:-}"; exit 0;; esac
exit 0
SH
chmod +x "$SHIMS"/*
LOCKF="$T/lock.yaml"
cat >"$LOCKF" <<EOF
schema: 1
images:
- id: IMG-QA
  reference: localhost/example-qa
  tag_intent: qa1
  digest: "$D1"
- id: IMG-GO
  reference: docker.io/example/golang
  tag_intent: "1.25"
  digest: "$D1"
EOF
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
PROBE_OK="$T/probe_ok.txt"; printf 'version=helixqa 0.0.0-fixture\nout_writable=yes\ncache_writable=yes\nsrc_readonly=yes\n' >"$PROBE_OK"
newreg() { rm -rf "$T/reg"; mkdir -p "$T/reg/repo/.audit"; export LONGOPS_REPO="$T/reg/repo" LONGOPS_DIR="$T/reg/repo/.audit/longops" LONGOPS_AUDIT="$T/reg/repo/.audit" LONGOPS_ALLOW_TMPFS=1; }
newreg; CK="$T/checkout"; mkdir -p "$CK"
export RUNNER_TEST_MODE=1 RUNNER_RUNP="$SHIMS/run_pinned.sh" RUNNER_SWEEP="$SHIMS/sweep.sh" RUNP_LOCK="$LOCKF" RUNNER_HEARTBEAT_S=1
export ENVELOPE_TEST_MODE=1 ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000
export SHIM_LOG="$T/run.log" SWEEP_LOG="$T/sweep.log" SHIM_PROBE_OUT="$PROBE_OK" SHIM_INSPECT_DIGEST="$D1"
export PATH="$SHIMS:$PATH"
EXP_MEM=19660800000; EXP_CPUS=9; EXP_PIDS=2048
resetlogs() { : >"$SHIM_LOG"; : >"$SWEEP_LOG"; unset SHIM_RC SHIM_SWEEP_RC; export SHIM_PROBE_OUT="$PROBE_OK" SHIM_INSPECT_DIGEST="$D1"; }
wr() { ( cd "$CK" && bash "$SUTDIR/run_qa.sh" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
ncalls() { grep -c '^---CALL---$' "$SHIM_LOG"; }
call() { awk -v n="$1" '/^---CALL---$/{c++; next} c==n{print}' "$SHIM_LOG"; }
callenv() { call "$1" | sed -n "s/^ENV:$2=//p"; }
refused() { grep -q "REFUSED reason=$1" "$T/stderr"; }
nops() { ls "$LONGOPS_DIR"/ops/*.json 2>/dev/null | wc -l | tr -d ' '; }
oneop() { ls "$LONGOPS_DIR"/ops/*.json 2>/dev/null | head -1; }

# ---- usage ----
resetlogs; newreg
wr; check "no arguments is a usage error" "$RC" 2
wr --out "$T/o"; check "missing -- command is a usage error" "$RC" 2
wr --bogus -- true; check "unknown option is a usage error" "$RC" 2
wr --image IMG-GO -- true; check "--image is not an option (the wrapper has exactly one image)" "$RC" 2
check "usage errors start no container and register nothing" "$(ncalls)/$(nops)" "0/0"

# ---- a normal run ----
resetlogs; newreg; OUT="$T/out"
wr --out "$OUT" -- helixqa --version
check "run: exit 0" "$RC" 0
check "run: probe then run (two containers through run_pinned.sh)" "$(ncalls)" 2
call 1 | grep -qxF 'ARG:IMG-QA' && call 2 | grep -qxF 'ARG:IMG-QA' && ok "run: both calls name IMG-QA" || bad "run: image id: $(call 2 | tr '\n' ' ')"
call 2 | grep -qxF 'ARG:IMG-GO' && bad "run: IMG-GO appears in the call" || ok "run: no other image appears"
check "run: RUNP_MEMORY is the envelope memory" "$(callenv 2 RUNP_MEMORY)" "$EXP_MEM"
check "run: RUNP_CPUS is the envelope cpus" "$(callenv 2 RUNP_CPUS)" "$EXP_CPUS"
check "run: RUNP_PIDS is the envelope pids" "$(callenv 2 RUNP_PIDS)" "$EXP_PIDS"
call 2 | grep -qxF 'ARG:helixqa' && call 2 | grep -qxF 'ARG:--version' && ok "run: the user command is passed through" || bad "run: command missing"
check "run: exactly one long-op registered, complete" "$(nops)/$(jq -r .state "$(oneop)")" "1/complete"
call 1 | grep -q '^REG:.*=registered' && ok "run: the op was registered BEFORE the first container started" || bad "run: registry at probe time: $(call 1 | grep '^REG:')"
check "run: the sweep ran once at the cadence stage" "$(grep -c -- '--stage cadence' "$SWEEP_LOG")" 1
TR="$OUT/toolchain.json"
check "run: toolchain.json image id" "$(jq -r .image_id "$TR" 2>/dev/null)" IMG-QA
check "run: toolchain.json wrapper" "$(jq -r .wrapper "$TR" 2>/dev/null)" run_qa
check "run: toolchain.json digest_match" "$(jq -r .digest_match "$TR" 2>/dev/null)" true
check "run: toolchain.json read-only source control needle" "$(jq -r .probes.src_readonly "$TR" 2>/dev/null)" yes
resetlogs; newreg; export SHIM_RC=7
wr --out "$T/o2" -- false
check "run: container exit code passes through" "$RC" 7
check "run: a failed run ends the op failed" "$(jq -r .state "$(oneop)")" failed
unset SHIM_RC

# ---- refusals: the wrapper refuses an unpinned or altered IMG-QA, never another image ----
cp "$LOCKF" "$T/lock.keep"
python3 -I - "$LOCKF" <<'PY'
import re, sys
p = sys.argv[1]; s = open(p).read()
s = re.sub(r'(- id: IMG-QA\n  reference: [^\n]*\n  tag_intent: [^\n]*\n)  digest: "[^"]*"\n', r'\1', s, count=1)
open(p, "w").write(s)
PY
resetlogs; newreg; wr --out "$T/o3" -- true
check "unpinned IMG-QA: refused (exit 1)" "$RC" 1
refused image_unpinned && ok "unpinned IMG-QA: reason image_unpinned" || bad "reason: $(cat "$T/stderr")"
check "unpinned IMG-QA: no container, no op" "$(ncalls)/$(nops)" "0/0"
cp "$T/lock.keep" "$LOCKF"
resetlogs; newreg; export SHIM_INSPECT_DIGEST="$D2"
wr --out "$T/o4" -- true
check "altered IMG-QA (podman reports another digest): refused" "$RC" 1
refused image_digest_mismatch && ok "altered IMG-QA: reason image_digest_mismatch" || bad "reason: $(cat "$T/stderr")"
check "altered IMG-QA: no run container started" "$(ncalls)" 0
resetlogs; newreg
python3 -I - "$LOCKF" <<'PY'
import re, sys
p = sys.argv[1]; s = open(p).read()
s = re.sub(r'- id: IMG-QA\n(?:  [^\n]*\n)+', '', s, count=1)
open(p, "w").write(s)
PY
wr --out "$T/o5" -- true
check "IMG-QA absent from the lock: refused (never another image)" "$RC" 1
refused image_not_in_lock && ok "IMG-QA absent: reason image_not_in_lock" || bad "reason: $(cat "$T/stderr")"
cp "$T/lock.keep" "$LOCKF"
resetlogs; newreg; export SHIM_SWEEP_RC=10
wr --out "$T/o6" -- true
check "anti-mess drift refuses the run" "$RC" 1
refused anti_mess_drift && ok "drift: reason anti_mess_drift" || bad "reason: $(cat "$T/stderr")"
unset SHIM_SWEEP_RC

# ---- the real lock: what T211 has or has not delivered, read from the lock itself ----
REALLOCK="$REPO/build/containers/images.lock.yaml"
if grep -q '^- id: IMG-QA$' "$REALLOCK" 2>/dev/null; then ok "real lock: IMG-QA entry present"; else echo "NOTE: real lock has no IMG-QA entry (T211 BLOCKED); run_qa.sh against the real lock is refused:"; (cd "$CK" && env -u RUNNER_TEST_MODE -u RUNNER_RUNP -u RUNNER_SWEEP RUNP_LOCK="$REALLOCK" bash "$SUTDIR/run_qa.sh" -- true) 2>&1 | head -2; fi

echo "RESULT pass=$PASSES fail=$FAILS"
if [ "${RUNNER_TEST_MUTANT:-0}" != 1 ] && [ "${RUNNER_TEST_NO_MUTATIONS:-0}" != 1 ]; then
  # paired mutation: a copy of the wrapper whose image is IMG-GO must make this body FAIL
  M="$T/mut"; mkdir -p "$M"; cp "$SUTDIR"/*.sh "$M"/
  python3 -I - "$M/run_qa.sh" <<'PY'
import sys
p = sys.argv[1]; s = open(p).read()
assert s.count('RUNNER_IMAGES="IMG-QA"') == 1, "mutation anchor not found exactly once"
open(p, "w").write(s.replace('RUNNER_IMAGES="IMG-QA"', 'RUNNER_IMAGES="IMG-GO"'))
PY
  RUNNER_SUT_DIR="$M" RUNNER_TEST_MUTANT=1 bash "$HERE/test_run_qa.sh" >"$T/mut.out" 2>&1; mrc=$?
  if [ "$mrc" != 0 ]; then echo "MUTATION image IMG-QA -> IMG-GO: test FAILED as required (rc=$mrc)"; else echo "FAIL: mutation image -> IMG-GO went undetected"; FAILS=$((FAILS+1)); fi
fi
[ "$FAILS" = 0 ] && exit 0 || exit 1
