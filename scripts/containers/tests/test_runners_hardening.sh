#!/usr/bin/env bash
# test_runners_hardening.sh - fix round r1 (RED first, then GREEN x3) for the independent review of the resource envelope and the runner wrappers
# (specs/001-full-project-audit-remediation, WF10 p1-envelope-wrappers). Oracle for scripts/containers/runner_lib.sh and the run_*.sh wrappers; every block
# names the review finding it reproduces:
#   F1 envelope read + registration atomic (N concurrent starts never sum above the 0.60 ceiling)      F2 the 2% / 1 cpu allowance is capped by head-room
#   F4 inherited RUNP_* controls are scrubbed (RUNP_PRINT_ARGV=1 can no longer fake a version probe)     F5 a reused --op-id never truncates the other run's logs
#   F6 TERM/INT/HUP during the toolchain probe ends the run 130 with the op released                    F8 --wall-s is enforced (exit 124, op failed)
#   F9 a run that writes only /out is progressing, a run that writes nothing is hung (control needle)   F10 a killed wrapper's heartbeat loop and container client end
#   F11 a failing version command is refused version_probe_failed (real IMG-TESTUTIL)                    TREE the test runs leave the real evidence/disk untouched
#   Round 2 (WF13): B1 every stop path ends the CONTAINER (the shim's client ignores TERM like a container's PID 1; the real legs use IMG-TESTUTIL `sleep`)
#   I1 the dispatcher passes no limit, the wrapper's locked reading is the only one    I2 a refused duplicate --op-id touches none of the running run's files
#   m1 a wrapper killed during its probe        m2 TERM while waiting for the budget lock        m4 a negative control for the mutation harness
# F3 and F7 (the envelope itself) are in test_envelope.sh. Oracle strategy (11.4.245): SPECIFIED (hand-computed limits of docs/16 8.2 on a fixture host) and a
# second independent observer for every behaviour: a run_pinned.sh SHIM that records what it was given, the REAL long-op registry read back with jq, and /proc.
# Paired mutations: copies of the containers directory whose runner_lib.sh has ONE expression changed; each mutant re-runs only the blocks that cover it
# (R1_ONLY) and must be caught by a check whose NAME contains the expected cause (so "caught" never means "failed for an unrelated reason").
# Usage: test_runners_hardening.sh      tests and mutations      R1_TEST_NO_MUTATIONS=1 tests only      R1_TEST_NO_REAL=1 skip the real container block
# Env:   R1_SUT_DIR (containers directory under test; RED runs point it at an archive of the committed scripts)  R1_ONLY (space separated block names)
#        R1_MUTATION_RECORD (file receiving one line per mutation)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUTDIR="${R1_SUT_DIR:-$HERE/..}"
SUTDIR="$(cd "$SUTDIR" && pwd)"
ROOT="$(cd "$SUTDIR/../.." && pwd)"
FAILS=0; PASSES=0
# count_tok <dir> <token>: how many files in <dir> carry <token> in their name (a glob, never ls | grep)
count_tok() { local n=0 f; for f in "$1"/*"$2"*; do [ -e "$f" ] && n=$((n+1)); done; echo "$n"; }
ok()  { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
want() { [ -z "${R1_ONLY:-}" ] && return 0; case " $R1_ONLY " in *" $1 "*) return 0;; *) return 1;; esac; }
for d in jq python3 flock; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required by this test"; exit 2; }; done
[ -f "$SUTDIR/runner_lib.sh" ] || { echo "FAIL: runner_lib.sh not found at $SUTDIR"; exit 1; }

T="$(mktemp -d "${TMPDIR:-/tmp}/runners-hardening.XXXXXX")"
SHIMS="$T/shims"; mkdir -p "$SHIMS"
HOLDERS=()
cleanup() {
  local p cl
  for p in "${HOLDERS[@]:-}"; do [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && [ "$({ tr '\0' ' ' <"/proc/$p/cmdline"; } 2>/dev/null)" = "sleep 600 " ] && kill "$p" 2>/dev/null; done
  # shim processes of this run (identity: the cmdline names OUR shim script; never a bare pgrep)
  if [ -s "$T/allpids" ]; then while read -r p; do
    [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] || continue
    cl="$({ tr '\0' ' ' <"/proc/$p/cmdline"; } 2>/dev/null)"; case "$cl" in *"$SHIMS/run_pinned.sh"*) kill -KILL "$p" 2>/dev/null;; esac
  done <"$T/allpids"; fi
  rm -rf "$T"
}
trap cleanup EXIT
D1="sha256:$(printf 'a%.0s' $(seq 64))"
cat >"$SHIMS/run_pinned.sh" <<'SH'
#!/usr/bin/env bash
prev=""; OUTD=""; OPID=""; for a in "$@"; do [ "$prev" = --out ] && { mkdir -p -- "$a"; OUTD=$a; }; [ "$prev" = --op-id ] && OPID=$a; prev=$a; done
echo $$ >>"${SHIM_ALLPIDS:?}"
# the shim stands for `podman run` + its container: PID 1 of a container ignores a TERM it has no handler for, and the client proxies TERM to it, so the shim
# ignores TERM too; it ends only when the fake podman (below) "stops the container" with KILL (review round 2 B1: a shim that dies on TERM hid the defect)
trap '' TERM
mkdir -p "${SHIM_CDIR:?}"; printf '%s\n%s\n' "$$" "$OUTD" >"$SHIM_CDIR/$OPID.c"
# the unstoppable case: the "container" is a separate process (a client that is killed leaves it running, like a real container whose client died)
case " $* " in *cpa-probe*|*" --version "*) ;; *) if [ -n "${SHIM_STUBBORN:-}" ]; then sleep 600 & printf '%s\n%s\n' "$!" "$OUTD" >"$SHIM_CDIR/$OPID.c"; wait; exit 0; fi;; esac
{
  echo "---CALL---"
  printf 'ARG:%s\n' "$@"
  printf 'ENV:RUNP_MEMORY=%s\nENV:RUNP_CPUS=%s\nENV:RUNP_PIDS=%s\n' "${RUNP_MEMORY-}" "${RUNP_CPUS-}" "${RUNP_PIDS-}"
  for v in RUNP_PRINT_ARGV RUNP_TEST_MODE RUNP_MEMINFO RUNP_ULIMIT_U RUNP_USER; do
    if [ -n "$(eval "echo \${$v+x}")" ]; then printf 'ENV:%s=%s\n' "$v" "$(eval "echo \${$v}")"; else printf 'ENV:%s=<unset>\n' "$v"; fi
  done
} >>"${SHIM_LOG:?}"
case " $* " in
  *cpa-probe*|*" --version "*) [ -z "${SHIM_PROBE_SLEEP:-}" ] || sleep "$SHIM_PROBE_SLEEP"; [ -z "${SHIM_PROBE_OUT:-}" ] || cat "$SHIM_PROBE_OUT"; exit "${SHIM_PROBE_RC:-0}";;
esac
[ -z "${SHIM_PIDFILE:-}" ] || echo $$ >"$SHIM_PIDFILE"
[ -z "${SHIM_EARLY:-}" ] || echo "early-line"
if [ -n "${SHIM_OUTLINES:-}" ]; then for i in $(seq "$SHIM_OUTLINES"); do echo "line $i" >>"$OUTD/go-test.jsonl"; sleep 1; done; fi
if [ -n "${SHIM_GATE:-}" ]; then while [ ! -e "$SHIM_GATE" ]; do sleep 0.2; done; fi
[ -z "${SHIM_SLEEP:-}" ] || sleep "$SHIM_SLEEP"
echo "main-run-stdout"; echo "main-run-stderr" >&2
exit "${SHIM_RC:-0}"
SH
cat >"$SHIMS/sweep.sh" <<'SH'
#!/usr/bin/env bash
echo "AM-P1 clean"; exit 0
SH
cat >"$SHIMS/podman" <<'SH'
#!/usr/bin/env bash
# a fake podman that knows the containers of the fake run_pinned.sh (records $SHIM_CDIR/<op id>.c: pid, out dir); `stop` and `kill` end the shim with KILL
case "$*" in "image inspect"*) echo "${SHIM_INSPECT_DIGEST:-}"; exit 0;; esac
cpid() { head -1 "$SHIM_CDIR/$1.c" 2>/dev/null; }
calive() { local p; p="$(cpid "$1")"; [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && kill -0 "$p" 2>/dev/null && [ "$(sed 's/^.*) //' "/proc/$p/stat" 2>/dev/null | cut -d' ' -f1)" != Z ]; }
case "${1:-}" in
  ps) opid=""; for a in "$@"; do case "$a" in label=catalogizer.op_id=*) opid=${a#label=catalogizer.op_id=};; esac; done
      [ -z "$opid" ] || ! calive "$opid" || echo "shimcid-$opid"; exit 0;;
  inspect) for a in "$@"; do case "$a" in shimcid-*) id=$a;; esac; done; sed -n 2p "$SHIM_CDIR/${id#shimcid-}.c" 2>/dev/null; exit 0;;
  stop|kill) echo "$*" >>"${SHIM_PODLOG:?}"
      [ -z "${SHIM_STUBBORN:-}" ] || exit 0   # a container that survives stop and kill (the unstoppable case)
      for a in "$@"; do case "$a" in shimcid-*) id=$a;; esac; done
      p="$(cpid "${id#shimcid-}")"
      if [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && case "$(tr '\0' ' ' <"/proc/$p/cmdline" 2>/dev/null)" in *"$SHIM_DIR/run_pinned.sh"*) true;; *) false;; esac; then kill -KILL "$p" 2>/dev/null; fi
      exit 0;;
esac
exit 0
SH
chmod +x "$SHIMS"/*
LOCKF="$T/lock.yaml"
cat >"$LOCKF" <<EOF
schema: 1
images:
- id: IMG-GO
  reference: docker.io/example/golang
  tag_intent: "1.25"
  digest: "$D1"
- id: IMG-SHELLCHECK
  reference: docker.io/example/shellcheck
  tag_intent: stable
  digest: "$D1"
  entrypoint_override: /bin/shellcheck
EOF
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
PROBE_OK="$T/probe_ok.txt"; printf 'version=go version go1.25.14 linux/amd64\nout_writable=yes\ncache_writable=yes\nsrc_readonly=yes\n' >"$PROBE_OK"
newreg() { rm -rf "$T/reg"; mkdir -p "$T/reg/repo/.audit"; export LONGOPS_REPO="$T/reg/repo" LONGOPS_DIR="$T/reg/repo/.audit/longops" LONGOPS_AUDIT="$T/reg/repo/.audit" LONGOPS_ALLOW_TMPFS=1; }
newreg
CK="$T/checkout"; mkdir -p "$CK"
export DISK_HEADROOM_OUT_DIR="$T/disk"; mkdir -p "$DISK_HEADROOM_OUT_DIR"
export RUNNER_TEST_MODE=1 RUNNER_RUNP="$SHIMS/run_pinned.sh" RUNNER_SWEEP="$SHIMS/sweep.sh" RUNP_LOCK="$LOCKF" RUNNER_HEARTBEAT_S=1
export ENVELOPE_TEST_MODE=1 ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000
export SHIM_LOG="$T/run.log" SHIM_ALLPIDS="$T/allpids" SHIM_PROBE_OUT="$PROBE_OK" SHIM_INSPECT_DIGEST="$D1" SHIM_CDIR="$T/cont" SHIM_PODLOG="$T/podman.log" SHIM_DIR="$SHIMS" RUNNER_STOP_GRACE_S=2
ORIG_PATH="$PATH"; export PATH="$SHIMS:$PATH"
EXP_CEIL=19660800000
: >"$SHIM_LOG"
resetlogs() { : >"$SHIM_LOG"; unset SHIM_RC SHIM_SLEEP SHIM_PROBE_RC SHIM_PROBE_SLEEP SHIM_GATE SHIM_PIDFILE SHIM_EARLY SHIM_OUTLINES; export SHIM_PROBE_OUT="$PROBE_OK" SHIM_INSPECT_DIGEST="$D1"; }
wr() { local w=$1; shift; ( cd "$CK" && bash "$SUTDIR/$w.sh" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
refused() { grep -q "REFUSED reason=$1" "$T/stderr"; }
call() { awk -v n="$1" '/^---CALL---$/{c++; next} c==n{print}' "$SHIM_LOG"; }
callenv() { call "$1" | sed -n "s/^ENV:$2=//p"; }
opf() { echo "$LONGOPS_DIR/ops/$1.json"; }
waitfor() { local n=0; while ! eval "$1"; do sleep 0.2; n=$((n+1)); [ "$n" -lt "${2:-300}" ] || return 1; done; }
# bg <tag> <wrapper> <args...>: start a wrapper in the background (SIGINT and SIGHUP at their defaults: a plain `&` ignores SIGINT and a `nohup` launch of this suite would hand SIGHUP-ignored down, review round 2 m4)
bg() {
  local tag=$1 w=$2; shift 2
  ( cd "$CK" && exec python3 -I -c 'import os, signal, sys
signal.signal(signal.SIGINT, signal.SIG_DFL)
signal.signal(signal.SIGHUP, signal.SIG_DFL)
os.execvp("bash", ["bash"] + sys.argv[1:])' "$SUTDIR/$w.sh" "$@" ) >"$T/$tag.out" 2>"$T/$tag.err" &
  BGPID=$!
}
live_stats() { local s=0 n=0 f st; for f in "$LONGOPS_DIR"/ops/*.json; do [ -e "$f" ] || continue; st="$(jq -r .state "$f")"; case "$st" in complete|failed|reaped|handoff|blocked-escape) ;; *) s=$(( s + $(jq -r '.budget.memory_bytes' "$f") )); n=$((n+1));; esac; done; echo "$n $s"; }
hold() { sleep 600 >/dev/null 2>&1 & HOLD=$!; HOLDERS+=("$HOLD"); }
# shim_up <op id>: "up" when the fake container of that op is a live non-zombie process (the observer independent of the wrapper)
shim_up() { local p; p="$(head -1 "$SHIM_CDIR/$1.c" 2>/dev/null)"; [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && kill -0 "$p" 2>/dev/null && [ "$(sed 's/^.*) //' "/proc/$p/stat" 2>/dev/null | cut -d' ' -f1)" != Z ] && echo up || echo gone; }

# ============================================================ F1: N concurrent starts
if want F1; then
  resetlogs; newreg; export SHIM_GATE="$T/gate"; rm -f "$SHIM_GATE"
  PIDS=()
  for n in 1 2 3 4 5 6; do bg "f1a$n" run_go --out "$T/o-f1a-$n" --memory 4000000000 -- true "$n"; PIDS+=("$BGPID"); done
  waitfor '[ $(( $(ls "$LONGOPS_DIR"/ops/*.json 2>/dev/null | wc -l) + $(grep -l "REFUSED" "$T"/f1a*.err 2>/dev/null | wc -l) )) -ge 6 ]' 450
  read -r N S <<<"$(live_stats)"
  : >"$SHIM_GATE"; for p in "${PIDS[@]}"; do wait "$p"; done
  check "F1 six concurrent starts, each asking 4000000000: exactly four are admitted (4 x 4e9 fits the 0.60 ceiling, a fifth does not)" "$N" 4
  [ "$S" -le "$EXP_CEIL" ] && ok "F1 sum of live budgets $S is within the 0.60 ceiling $EXP_CEIL" || bad "F1 sum of live budgets $S exceeds the 0.60 ceiling $EXP_CEIL (N=$N)"
  check "F1 two of the six are refused limit_exceeds_envelope, none silently dropped" "$(grep -l 'REFUSED reason=limit_exceeds_envelope' "$T"/f1a*.err 2>/dev/null | wc -l | tr -d ' ')" 2
  resetlogs; newreg; export SHIM_GATE="$T/gate2"; rm -f "$SHIM_GATE"
  PIDS=()
  for n in 1 2 3 4; do bg "f1b$n" run_go --out "$T/o-f1b-$n" -- true "$n"; PIDS+=("$BGPID"); done
  waitfor '[ $(( $(ls "$LONGOPS_DIR"/ops/*.json 2>/dev/null | wc -l) + $(grep -l "REFUSED" "$T"/f1b*.err 2>/dev/null | wc -l) )) -ge 4 ]' 450
  read -r N S <<<"$(live_stats)"
  : >"$SHIM_GATE"; for p in "${PIDS[@]}"; do wait "$p"; done
  [ "$S" -le "$EXP_CEIL" ] && ok "F1 four concurrent starts with no --memory: sum of live budgets $S within the ceiling" || bad "F1 four concurrent starts with no --memory: sum $S exceeds the ceiling $EXP_CEIL (N=$N, each was handed the full envelope)"
  check "F1 four concurrent starts with no --memory: only the first is admitted" "$N" 1
  check "F1 the other three are refused envelope_refused (no head-room left)" "$(grep -l 'REFUSED reason=envelope_refused' "$T"/f1b*.err 2>/dev/null | wc -l | tr -d ' ')" 3
fi

# ============================================================ F2: the allowance is capped by head-room
if want F2; then
  resetlogs; newreg; hold
  bash "$ROOT/scripts/longops/register.sh" --purpose other:held --owner r1 --pid "$HOLD" --memory-bytes 2000000000 --cpus 9 --no-progress-s 600 >/dev/null 2>&1
  wr run_go --out "$T/o-f2a" --memory 18014016000 -- true
  check "F2 --memory above (ceiling - live ops) inside the nominal 2%: refused" "$RC" 1
  refused limit_exceeds_envelope && ok "F2 reason limit_exceeds_envelope" || bad "F2 reason: $(cat "$T/stderr")"
  resetlogs
  wr run_go --out "$T/o-f2b" --memory 17660800000 -- true
  check "F2 --memory exactly the live envelope (ceiling - live ops): accepted" "$RC" 0
  resetlogs
  wr run_go --out "$T/o-f2c" --cpus 6 -- true
  check "F2 --cpus above the live cpu budget (5) by the nominal +1: refused (the slack never enters the reserve)" "$RC" 1
  resetlogs
  wr run_go --out "$T/o-f2d" --cpus 5 -- true
  check "F2 --cpus exactly the live cpu budget: accepted" "$RC" 0
  [[ "$HOLD" =~ ^[0-9]+$ && "$HOLD" -gt 1 ]] && kill "$HOLD" 2>/dev/null
  # MemAvailable binds: 20000000 kB available -> budget 15564800000; one byte above is the reserve
  printf 'MemTotal:       32000000 kB\nMemAvailable:   20000000 kB\n' >"$T/meminfo"
  resetlogs; newreg
  wr run_go --out "$T/o-f2e" --memory 15564800001 -- true
  check "F2 MemAvailable binds: 1 byte above the live envelope would enter the reserve: refused" "$RC" 1
  resetlogs
  wr run_go --out "$T/o-f2f" --memory 15564800000 -- true
  check "F2 MemAvailable binds: exactly the live envelope: accepted" "$RC" 0
  printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
fi

# ============================================================ F4: inherited RUNP_* controls are scrubbed
if want F4; then
  resetlogs; newreg
  ( cd "$CK" && RUNP_PRINT_ARGV=1 RUNP_TEST_MODE=1 RUNP_MEMINFO=/nonexistent RUNP_ULIMIT_U=99999 RUNP_USER=1234:1234 bash "$SUTDIR/run_go.sh" --out "$T/o-f4" -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?
  check "F4 the run itself succeeds" "$RC" 0
  check "F4 RUNP_PRINT_ARGV is not handed to run_pinned.sh (it would print its argv and start nothing)" "$(callenv 2 RUNP_PRINT_ARGV)" "<unset>"
  check "F4 RUNP_TEST_MODE is scrubbed (a test hook gate the wrapper does not own)" "$(callenv 2 RUNP_TEST_MODE)" "<unset>"
  check "F4 RUNP_MEMINFO is scrubbed (a test hook)" "$(callenv 2 RUNP_MEMINFO)" "<unset>"
  check "F4 RUNP_ULIMIT_U is scrubbed (a test hook)" "$(callenv 2 RUNP_ULIMIT_U)" "<unset>"
  check "F4 the probe call is scrubbed too" "$(callenv 1 RUNP_PRINT_ARGV)" "<unset>"
  check "F4 RUNP_USER (a documented input) passes through" "$(callenv 2 RUNP_USER)" "1234:1234"
  grep -q 'ignored inherited RUNP_PRINT_ARGV' "$T/stderr" && ok "F4 the scrub is announced on stderr" || bad "F4 no note on stderr: $(head -3 "$T/stderr")"
fi

# ============================================================ F5: a reused --op-id never truncates the other run's logs
if want F5; then
  resetlogs; newreg
  wr run_go --out "$T/o-f5a" --op-id dup1 -- true
  LOGA="$CK/.audit/runner-logs/dup1.out"
  SUM_A="$(sha256sum "$LOGA" | cut -c1-64)"; SZ_A="$(stat -c %s "$LOGA")"
  [ "$SZ_A" -gt 0 ] && ok "F5 run A left a log of $SZ_A bytes" || bad "F5 run A log empty"
  resetlogs
  wr run_go --out "$T/o-f5b" --op-id dup1 -- true
  check "F5 a finished op id reused: refused (exit 1)" "$RC" 1
  refused op_exists && ok "F5 the reason is op_exists (not purpose_conflict)" || bad "F5 reason: $(grep REFUSED "$T/stderr")"
  check "F5 run A's log is byte-identical after the refused reuse" "$(sha256sum "$LOGA" | cut -c1-64)" "$SUM_A"
  check "F5 run A's record is still complete" "$(jq -r .state "$(opf dup1)")" complete
  # A running, B refused, A's replayed output intact
  resetlogs; newreg; export SHIM_EARLY=1 SHIM_GATE="$T/gate5"; rm -f "$SHIM_GATE"
  bg f5a run_go --out "$T/o-f5c" --op-id dup2 -- true; PA=$BGPID
  waitfor '[ "$(jq -r .state "$LONGOPS_DIR/ops/dup2.json" 2>/dev/null)" = running ] && [ "$(stat -c %s "$CK/.audit/runner-logs/dup2.out" 2>/dev/null || echo 0)" -ge 10 ]' 300
  SZ_RUN="$(stat -c %s "$CK/.audit/runner-logs/dup2.out" 2>/dev/null || echo 0)"
  wr run_go --out "$T/o-f5d" --op-id dup2 -- true
  refused op_exists && ok "F5 reuse of a RUNNING op id: reason op_exists" || bad "F5 running reuse reason: $(grep REFUSED "$T/stderr")"
  check "F5 the running run's log was not truncated by the refused start" "$(stat -c %s "$CK/.audit/runner-logs/dup2.out")" "$SZ_RUN"
  : >"$SHIM_GATE"; wait "$PA"
  printf 'early-line\nmain-run-stdout\n' >"$T/f5-expect"
  cmp -s "$T/f5a.out" "$T/f5-expect" && ok "F5 run A's replayed stdout is exactly its own output (no NUL bytes, no lost lines)" || bad "F5 run A stdout: $(od -c "$T/f5a.out" | head -3)"
fi

# ============================================================ F6: TERM/INT/HUP during the toolchain probe
if want F6; then
  for sig in TERM INT HUP; do
    resetlogs; newreg; export SHIM_PROBE_SLEEP=20
    bg "f6$sig" run_go --out "$T/o-f6-$sig" --op-id "f6$sig" -- true; PW=$BGPID
    waitfor '[ "$(grep -c "^---CALL---$" "$SHIM_LOG")" -ge 1 ]' 300
    kill -s "$sig" "$PW" 2>/dev/null
    wait "$PW"; RCW=$?
    check "F6 $sig during the toolchain probe: the wrapper ends 130" "$RCW" 130
    check "F6 $sig during the probe: the op is released failed" "$(jq -r .state "$(opf "f6$sig")" 2>/dev/null)" failed
    check "F6 $sig during the probe: the verdict is interrupted" "$(jq -r .verdict "$(opf "f6$sig")" 2>/dev/null)" interrupted
    check "F6 $sig during the probe: no op is left with a dead owner" "$(bash "$ROOT/scripts/longops/classify.sh" | cut -f2 | grep -c dead_owner)" 0
    unset SHIM_PROBE_SLEEP
  done
  # mid-run (R7/R8): TERM while the container runs
  resetlogs; newreg; export SHIM_SLEEP=30
  bg f6mid run_go --out "$T/o-f6m" --op-id f6mid -- true; PW=$BGPID
  waitfor '[ "$(grep -c "^---CALL---$" "$SHIM_LOG")" -ge 2 ]' 300
  waitfor '[ "$(shim_up f6mid)" = up ]' 100
  S0=$(date +%s); : >"$T/podman.log"
  kill -s TERM "$PW" 2>/dev/null; wait "$PW"; RCW=$?; EL=$(( $(date +%s) - S0 ))
  check "F6 TERM during the run: the wrapper ends 130" "$RCW" 130
  check "F6 TERM during the run: the op is failed / interrupted" "$(jq -r '.state + "/" + .verdict' "$(opf f6mid)")" "failed/interrupted"
  # B1: the shim (like a container's PID 1) ignores TERM: only a STOP OF THE CONTAINER ends it
  check "F6 TERM during the run: the container (a process that ignores TERM) is gone when the op is released" "$(shim_up f6mid)" gone
  [ "$EL" -le 12 ] && ok "F6 TERM during the run: the wrapper ended ${EL}s after the TERM (the run was a 30 s sleep)" || bad "F6 TERM during the run: ${EL}s after the TERM (the container was not stopped)"
  grep -q "^stop --time 2 -- shimcid-f6mid" "$T/podman.log" && ok "F6 TERM during the run: the container was stopped by podman stop --time <grace>, by its own id" || bad "F6 no podman stop of the container: $(cat "$T/podman.log")"
fi

# ============================================================ F8: --wall-s is enforced
if want F8; then
  resetlogs; newreg; export SHIM_SLEEP=30
  S0=$(date +%s)
  wr run_go --out "$T/o-f8" --op-id f8 --wall-s 2 -- true
  EL=$(( $(date +%s) - S0 ))
  check "F8 a run past --wall-s 2 is terminated: exit 124" "$RC" 124
  [ "$EL" -le 15 ] && ok "F8 the run was cut short (${EL}s for --wall-s 2, of a 30 s sleep)" || bad "F8 the run lasted ${EL}s for --wall-s 2: the wall clock did not take effect"
  check "F8 the container (ignoring TERM like a PID 1) is gone" "$(shim_up f8)" gone
  grep -q 'main-run-stdout' "$T/stdout" && bad "F8 the run completed its work ('main-run-stdout'): it was not stopped" || ok "F8 the run did not complete its work (no 'main-run-stdout')"
  grep -q 'wall clock of 2s exceeded' "$T/stderr" && ok "F8 the wrapper says so on stderr" || bad "F8 stderr: $(cat "$T/stderr")"
  EMS="$(jq -r '.elapsed_ms // 0' "$(opf f8)" 2>/dev/null)"
  # the loop's own clock at the instant it ended the run: independent of how long stopping the container took (N3: a wall clock 3x too long reads ~6000)
  { [ "$EMS" -ge 2000 ] && [ "$EMS" -le 3500 ]; } && ok "F8 the heartbeat reported the elapsed time at the wall clock (${EMS} ms)" || bad "F8 the heartbeat reported the elapsed time $EMS ms: not about 2000-3500 ms for --wall-s 2"
  check "F8 the op is failed with verdict wall_clock_exceeded" "$(jq -r '.state + "/" + .verdict' "$(opf f8)" 2>/dev/null)" "failed/wall_clock_exceeded"
  [ "$(jq -r '.elapsed_ms // 0' "$(opf f8)" 2>/dev/null)" -gt 0 ] && ok "F8 the heartbeat reported elapsed time (the registry's wall-clock rule can see it)" || bad "F8 no elapsed_ms in the op record"
  resetlogs; newreg; export SHIM_SLEEP=2
  wr run_go --out "$T/o-f8b" --wall-s 60 -- true
  check "F8 control: a run well inside --wall-s is not terminated" "$RC" 0
fi

# ============================================================ F9: progress counts /out writes; a silent run is hung
if want F9; then
  resetlogs; newreg; export SHIM_OUTLINES=8
  bg f9a run_go --out "$T/o-f9a" --op-id f9a --no-progress-s 2 -- true; PW=$BGPID
  waitfor '[ "$(jq -r .state "$LONGOPS_DIR/ops/f9a.json" 2>/dev/null)" = running ]' 300
  sleep 3; C1="$(bash "$ROOT/scripts/longops/classify.sh" --op-id f9a | cut -f2)"
  sleep 2; C2="$(bash "$ROOT/scripts/longops/classify.sh" --op-id f9a | cut -f2)"
  check "F9 a run that writes only /out (3 s in, no-progress budget 2 s): advancing" "$C1" advancing
  check "F9 the same run 2 s later: still advancing" "$C2" advancing
  wait "$PW"
  resetlogs; newreg; export SHIM_SLEEP=9
  bg f9b run_go --out "$T/o-f9b" --op-id f9b --no-progress-s 2 -- true; PW=$BGPID
  waitfor '[ "$(jq -r .state "$LONGOPS_DIR/ops/f9b.json" 2>/dev/null)" = running ]' 300
  sleep 5; C3="$(bash "$ROOT/scripts/longops/classify.sh" --op-id f9b | cut -f2)"
  check "F9 control needle: a run that writes nothing anywhere is hung after its budget (the instrument can see a stall)" "$C3" hung
  wait "$PW"
fi

# ============================================================ F10: a killed wrapper's heartbeat loop and container client end
if want F10; then
  resetlogs; newreg; export SHIM_SLEEP=60 SHIM_PIDFILE="$T/f10.pid"; rm -f "$SHIM_PIDFILE"
  bg f10 run_go --out "$T/o-f10" --op-id f10 -- true; PW=$BGPID
  waitfor '[ -s "$T/f10.pid" ] && [ "$(jq -r .heartbeat_seq "$LONGOPS_DIR/ops/f10.json" 2>/dev/null || echo 0)" -ge 1 ]' 300
  SHIMPID="$(cat "$T/f10.pid")"; SHIMST="$(sed 's/^.*) //' "/proc/$SHIMPID/stat" 2>/dev/null | cut -d' ' -f20)"
  kill -KILL "$PW" 2>/dev/null; wait "$PW" 2>/dev/null
  waitfor '[ "$(jq -r .state "$LONGOPS_DIR/ops/f10.json" 2>/dev/null)" = failed ]' 150
  check "F10 the op is released only AFTER its container is gone (the budget is never freed while the container runs)" "$(shim_up f10)" gone
  SEQ1="$(jq -r .heartbeat_seq "$(opf f10)")"; sleep 3; SEQ2="$(jq -r .heartbeat_seq "$(opf f10)")"
  NOW_ST="$(sed 's/^.*) //' "/proc/$SHIMPID/stat" 2>/dev/null | cut -d' ' -f20)"
  check "F10 the container client of the killed wrapper is gone (identity: pid $SHIMPID start $SHIMST)" "$([ "$NOW_ST" = "$SHIMST" ] && echo alive || echo gone)" gone
  check "F10 the op of the killed wrapper is released failed" "$(jq -r .state "$(opf f10)")" failed
  check "F10 the verdict names the cause: wrapper_died" "$(jq -r .verdict "$(opf f10)")" wrapper_died
  check "F10 the heartbeat loop ended: the sequence did not advance in 3 s" "$SEQ2" "$SEQ1"
  : >"$CK/.audit/runner-logs/f10.stop"   # clean up an orphan loop of an unfixed wrapper (its stop file)
  if [ "$NOW_ST" = "$SHIMST" ]; then [[ "$SHIMPID" =~ ^[0-9]+$ && "$SHIMPID" -gt 1 ]] && kill -KILL "$SHIMPID" 2>/dev/null; fi
fi

# ============================================================ F10b (m1): a wrapper killed DURING its toolchain probe
if want F10b; then
  resetlogs; newreg; export SHIM_PROBE_SLEEP=40
  bg f10b run_go --out "$T/o-f10b" --op-id f10b -- true; PW=$BGPID
  waitfor '[ "$(grep -c "^---CALL---$" "$SHIM_LOG")" -ge 1 ] && [ "$(shim_up f10b)" = up ]' 300
  kill -KILL "$PW" 2>/dev/null; wait "$PW" 2>/dev/null
  waitfor '[ "$(jq -r .state "$LONGOPS_DIR/ops/f10b.json" 2>/dev/null)" = failed ]' 150
  check "F10b a wrapper killed during its probe: the op is released failed (not left registered with a dead owner)" "$(jq -r .state "$(opf f10b)")" failed
  check "F10b the verdict names the cause: wrapper_died" "$(jq -r .verdict "$(opf f10b)")" wrapper_died
  check "F10b the probe container is gone when the op is released" "$(shim_up f10b)" gone
  check "F10b no op is left with a dead owner" "$(bash "$ROOT/scripts/longops/classify.sh" | cut -f2 | grep -c dead_owner)" 0
fi

# ============================================================ F12 (m2): TERM while waiting for the budget lock
if want F12; then
  resetlogs; newreg; mkdir -p "$LONGOPS_DIR"
  ( exec 9>"$LONGOPS_DIR/.envelope-budget.lock"; flock 9; exec sleep 600 ) >/dev/null 2>&1 & HOLD=$!; HOLDERS+=("$HOLD")
  sleep 1
  bg f12 run_go --out "$T/o-f12" --op-id f12 -- true; PW=$BGPID
  sleep 3; S0=$(date +%s); kill -s TERM "$PW" 2>/dev/null; wait "$PW"; RCW=$?; EL=$(( $(date +%s) - S0 ))
  check "F12 TERM while waiting for the budget lock: the wrapper ends 130" "$RCW" 130
  [ "$EL" -le 4 ] && ok "F12 it ended ${EL}s after the TERM (not when the lock frees, up to 120 s later)" || bad "F12 it took ${EL}s to honour the TERM"
  [ ! -e "$(opf f12)" ] && ok "F12 nothing was registered" || bad "F12 an op record exists"
  [[ "$HOLD" =~ ^[0-9]+$ && "$HOLD" -gt 1 ]] && kill "$HOLD" 2>/dev/null
fi

# ============================================================ F13 (I2): a refused duplicate --op-id touches none of the running run's files
if want F13; then
  resetlogs; newreg; export SHIM_SLEEP=8
  bg f13a run_go --out "$T/o-f13a" --op-id f13 -- true; PA=$BGPID
  waitfor '[ "$(shim_up f13)" = up ] && [ "$(jq -r .state "$LONGOPS_DIR/ops/f13.json" 2>/dev/null)" = running ]' 300
  LD13="$CK/.audit/runner-logs"
  : >"$LD13/f13.stop.wall"   # a marker of the RUNNING run (its wall clock was reached): the refused start must not delete it
  for n in 1 2 3; do wr run_go --out "$T/o-f13b$n" --op-id f13 -- true; refused op_exists || bad "F13 duplicate start $n: not refused op_exists: $(cat "$T/stderr")"; done
  [ -e "$LD13/f13.stop.wall" ] && ok "F13 three refused duplicate starts left the running run's wall marker alone" || bad "F13 the wall marker of the running run was deleted by a refused start"
  rm -f "$LD13/f13.stop.wall"
  # a burst of duplicates across the end of the run: the run still ends, and ends clean
  for n in 1 2 3 4 5 6 7 8; do wr run_go --out "$T/o-f13c$n" --op-id f13 -- true; sleep 0.5; done
  S0=$(date +%s); wait "$PA"; RCA=$?
  check "F13 the running run ended on its own with its own exit code (0) after a burst of refused duplicates" "$RCA" 0
  check "F13 its op is complete" "$(jq -r .state "$(opf f13)")" complete
  [ $(( $(date +%s) - S0 )) -le 15 ] && ok "F13 it did not hang after the duplicates (a deleted stop file keeps the guard loop for ever)" || bad "F13 the run hung after the duplicate starts"
fi

# ============================================================ F14 (I1): the dispatcher passes no limit; the wrapper's locked reading is the only one
if want F14; then
  TICSH="$SUTDIR/../test-in-container.sh"
  if [ -f "$TICSH" ]; then
    mkdir -p "$T/tw"; printf '#!/usr/bin/env bash\nexec bash "%s/run_go.sh" "$@"\n' "$SUTDIR" >"$T/tw/run_go.sh"; chmod +x "$T/tw/run_go.sh"
    # the sweep shim changes the host in exactly the window between the dispatcher's start and the wrapper's locked reading
    cat >"$SHIMS/drift_sweep.sh" <<'SH'
#!/usr/bin/env bash
case "${DRIFT:-}" in
  mem)  printf 'MemTotal:       32000000 kB\nMemAvailable:   %s kB\n' "${DRIFT_MA:?}" >"$ENVELOPE_MEMINFO";;
  cpu6) bash "${DRIFT_ROOT:?}/scripts/longops/register.sh" --purpose drift:other --owner drift --pid "${DRIFT_PID:?}" --memory-bytes 1000000000 --cpus 6 --no-progress-s 600 >/dev/null 2>&1;;
esac
exit 0
SH
    chmod +x "$SHIMS/drift_sweep.sh"
    tic() { ( cd "$CK" && TIC_TEST_MODE=1 TIC_WRAPPER_DIR="$T/tw" RUNNER_SWEEP="$SHIMS/drift_sweep.sh" DRIFT_ROOT="$ROOT" bash "$TICSH" catalog-api unit -- true ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
    # baseline: no drift: the host reading is the budget (30000000 kB available -> the 0.60 ceiling 19660800000; 16 cpus -> 9)
    resetlogs; newreg; unset DRIFT; tic
    check "F14 baseline (no drift): the lane runs" "$RC" 0
    check "F14 baseline: run_pinned got the live envelope memory" "$(callenv 2 RUNP_MEMORY)" 19660800000
    check "F14 baseline: run_pinned got the live envelope cpus" "$(callenv 2 RUNP_CPUS)" 9
    # drift 1: MemAvailable binds and falls 3.9% in the window (20000000 kB -> 19400000 kB: budget 15564800000 -> 14950400000); a request taken from the earlier
    # reading (98% = 15253504000) is above the live reading and was refused
    resetlogs; newreg; printf 'MemTotal:       32000000 kB\nMemAvailable:   20000000 kB\n' >"$T/meminfo"
    DRIFT=mem DRIFT_MA=19400000 tic
    check "F14 the budget falls 3.9% between the dispatcher's start and the wrapper's reading: the lane still runs (no limit_exceeds_envelope)" "$RC" 0
    check "F14 the container gets the wrapper's own locked reading, not a number from before" "$(callenv 2 RUNP_MEMORY)" 14950400000
    printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
    # drift 2: another op registers 6 cpus in the window
    resetlogs; newreg; hold
    DRIFT=cpu6 DRIFT_PID="$HOLD" tic
    check "F14 another op registers 6 cpus in the window: the lane still runs" "$RC" 0
    check "F14 the container gets the cpus left after that op (14 - 6 = 8), not the 9 read before" "$(callenv 2 RUNP_CPUS)" 8
    [[ "$HOLD" =~ ^[0-9]+$ && "$HOLD" -gt 1 ]] && kill "$HOLD" 2>/dev/null
    # stress: lane starts (3 at a time) while the host reading changes every 0.1 s; none may be refused limit_exceeds_envelope
    resetlogs; newreg; unset DRIFT; export SHIM_SLEEP=1; rm -f "$T/stress.stop"
    ( i=0; while [ ! -e "$T/stress.stop" ]; do i=$((i+1)); ma=$(( 22000000 + (i % 7) * 1000000 )); printf 'MemTotal:       32000000 kB\nMemAvailable:   %s kB\n' "$ma" >"$T/meminfo.tmp" && mv -f "$T/meminfo.tmp" "$T/meminfo"; sleep 0.1; done ) &
    CHURN=$!; SPIDS=()
    for n in 1 2 3 4 5 6 7 8 9 10 11 12; do
      ( cd "$CK" && TIC_TEST_MODE=1 TIC_WRAPPER_DIR="$T/tw" bash "$TICSH" catalog-api unit -- true ) >"$T/stress.$n.out" 2>"$T/stress.$n.err"; echo $? >"$T/stress.$n.rc" &
      SPIDS+=("$!")
      [ $(( n % 3 )) != 0 ] || { for p in "${SPIDS[@]}"; do wait "$p"; done; SPIDS=(); }
    done
    for p in "${SPIDS[@]:-}"; do [ -z "$p" ] || wait "$p"; done
    : >"$T/stress.stop"; wait "$CHURN" 2>/dev/null
    check "F14 stress: 12 lane starts while MemAvailable changes every 0.1 s: none refused limit_exceeds_envelope" "$(grep -l 'REFUSED reason=limit_exceeds_envelope' "$T"/stress.*.err 2>/dev/null | wc -l | tr -d ' ')" 0
    check "F14 stress: every start either ran (exit 0) or was refused for lack of head-room (envelope_refused) and nothing else" "$(for f in "$T"/stress.*.rc; do r="$(cat "$f")"; [ "$r" = 0 ] || grep -q 'REFUSED reason=envelope_refused' "${f%.rc}.err" || echo "bad:$f"; done | wc -l | tr -d ' ')" 0
    [ "$(grep -lx 0 "$T"/stress.*.rc 2>/dev/null | wc -l)" -ge 4 ] && ok "F14 stress: at least four starts ran (the control needle: the instrument sees successes, not only refusals)" || bad "F14 stress: fewer than four starts ran: $(cat "$T"/stress.*.rc | tr '\n' ' ')"
    printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"; unset SHIM_SLEEP
  else
    echo "SKIP: the dispatcher test-in-container.sh is not beside this containers directory: the F14 block is not run (not faked)"
  fi
fi

# ============================================================ B1: the real container leg (IMG-TESTUTIL / IMG-GO): every stop path ends the CONTAINER
# PID 1 of these containers is `sleep` / `sh`: it ignores a TERM it has no handler for, which is exactly why a TERM to the `podman run` client never stopped a run
# (review round 2 B1). The observers are independent of the wrapper: `podman ps` by the op's label, the op record's own elapsed_ms, the run's own stdout.
if want B1 && [ "${R1_TEST_NO_REAL:-0}" != 1 ]; then
  B1TOK="r1b$$x$RANDOM"
  # the observers use the REAL podman (PATH carries the fake one for the shim blocks above): a "gone" read through the fake would pass for nothing
  b1_up() { PATH="$ORIG_PATH" podman ps -q --filter "label=catalogizer.op_id=$1" 2>/dev/null | wc -l | tr -d ' '; }
  # b1_main <op id>: 1 when the MAIN command (`sleep 40`) of that op is a running container (the toolchain probe container is a different command)
  b1_main() { PATH="$ORIG_PATH" podman ps --filter "label=catalogizer.op_id=$1" --format '{{.Command}}' 2>/dev/null | grep -c 'sleep 40' | tr -d ' '; }
  has_img() { python3 -I -c 'import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
sys.exit(0 if any(isinstance(i, dict) and i.get("id") == sys.argv[2] for i in d.get("images", [])) else 1)' "$ROOT/build/containers/images.lock.yaml" "$1"; }
  # the real wrapper under test, with the real run_pinned.sh and the real podman; only the sweep (the live registry's drift is not this test's subject) is a shim
  printf '#!/usr/bin/env bash\nexit 0\n' >"$T/b1-sweep.sh"
  b1env=(env -u RUNNER_RUNP -u RUNP_LOCK -u ENVELOPE_MEMINFO -u ENVELOPE_NPROC -u ENVELOPE_ULIMIT_U "PATH=$ORIG_PATH" "RUNNER_SWEEP=$T/b1-sweep.sh" RUNNER_STOP_GRACE_S=2 "RUNNER_LOG_DIR=$T/b1-logs")
  bwr() { local w=$1; shift; ( cd "$ROOT" && "${b1env[@]}" bash "$SUTDIR/$w.sh" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
  bbg() { local tag=$1 w=$2; shift 2
    ( cd "$ROOT" && exec "${b1env[@]}" python3 -I -c 'import os, signal, sys
signal.signal(signal.SIGINT, signal.SIG_DFL)
signal.signal(signal.SIGHUP, signal.SIG_DFL)
os.execvp("bash", ["bash"] + sys.argv[1:])' "$SUTDIR/$w.sh" "$@" ) >"$T/$tag.out" 2>"$T/$tag.err" &
    BGPID=$!; }
  if has_img IMG-TESTUTIL; then
    newreg
    # (a) --wall-s: `sleep 40` under --wall-s 4. Before the fix the container ran its 40 s and the wrapper exited 124 only at the end.
    S0=$(date +%s)
    ( for _i in $(seq 40); do sleep 0.2; if [ "$(b1_main "$B1TOK-wall")" = 1 ]; then echo seen >"$T/b1a.seen"; break; fi; done ) &   # control needle: the observer can see this run's container while it runs
    OBS=$!
    bwr run_testutil --wall-s 4 --op-id "$B1TOK-wall" --out "$T/b1-out-wall" -- sleep 40
    EL=$(( $(date +%s) - S0 )); wait "$OBS" 2>/dev/null
    check "B1a --wall-s 4 on a real container whose PID 1 (sleep) ignores TERM: exit 124" "$RC" 124
    [ "$EL" -le 22 ] && ok "B1a the run ended ${EL}s after its start (the sleep was 40 s; wall 4 s + the probe + a 2 s stop grace)" || bad "B1a the run lasted ${EL}s: the container was not stopped at the wall clock"
    [ -e "$T/b1a.seen" ] && ok "B1a control needle: the observer saw the container up while the run was running" || bad "B1a control needle: the observer never saw the container (the later 'gone' would mean nothing)"
    check "B1a the container is gone when the wrapper exits" "$(b1_up "$B1TOK-wall")" 0
    check "B1a the op is failed / wall_clock_exceeded" "$(jq -r '.state + "/" + .verdict' "$(opf "$B1TOK-wall")" 2>/dev/null)" "failed/wall_clock_exceeded"
    # (b) the race-lane shape (run_go prefix `env`, then `sh -c`): the work must NOT complete after the wrapper said "terminated"
    if has_img IMG-GO; then
      bwr run_go --wall-s 3 --op-id "$B1TOK-go" --out "$T/b1-out-go" -- sh -c 'sleep 25; echo slept-to-the-end'
      check "B1b run_go --wall-s 3 on 'sh -c sleep 25; echo ...': exit 124" "$RC" 124
      grep -q 'slept-to-the-end' "$T/stdout" && bad "B1b the command completed ('slept-to-the-end' on stdout) although the wrapper reported it terminated" || ok "B1b the command did not complete: nothing printed after the wall clock"
      check "B1b the container is gone" "$(b1_up "$B1TOK-go")" 0
    else echo "SKIP: IMG-GO is not in the lock on this host: B1b is not run (not faked)"; fi
    # (c) TERM to the wrapper while the container runs
    newreg
    bbg b1c run_testutil --op-id "$B1TOK-term" --out "$T/b1-out-term" -- sleep 40; PW=$BGPID
    waitfor '[ "$(b1_main "$B1TOK-term")" = 1 ]' 300 && ok "B1c control needle: the observer saw the main container up before the TERM" || bad "B1c control needle: the observer never saw the main container (a later 'gone' would mean nothing)"
    S0=$(date +%s); kill -s TERM "$PW" 2>/dev/null; wait "$PW"; RCW=$?; EL=$(( $(date +%s) - S0 ))
    check "B1c TERM to the wrapper during a real run: it ends 130" "$RCW" 130
    [ "$EL" -le 12 ] && ok "B1c it ended ${EL}s after the TERM (the sleep was 40 s)" || bad "B1c it ended ${EL}s after the TERM: the container ignored it"
    check "B1c the container is gone when the wrapper exits" "$(b1_up "$B1TOK-term")" 0
    check "B1c the op is failed / interrupted" "$(jq -r '.state + "/" + .verdict' "$(opf "$B1TOK-term")" 2>/dev/null)" "failed/interrupted"
    # (d) SIGKILL to the wrapper: the guard loop stops the container, then (and only then) releases the op
    bbg b1d run_testutil --op-id "$B1TOK-kill" --out "$T/b1-out-kill" -- sleep 40; PW=$BGPID
    waitfor '[ "$(b1_main "$B1TOK-kill")" = 1 ]' 300 && ok "B1d control needle: the observer saw the main container up before the SIGKILL" || bad "B1d control needle: the observer never saw the main container"
    kill -KILL "$PW" 2>/dev/null; wait "$PW" 2>/dev/null
    S0=$(date +%s)
    waitfor '[ "$(jq -r .state "$(opf "$B1TOK-kill")" 2>/dev/null)" = failed ]' 300 || true
    EL=$(( $(date +%s) - S0 ))
    check "B1d the op of a SIGKILLed wrapper is released failed / wrapper_died" "$(jq -r '.state + "/" + .verdict' "$(opf "$B1TOK-kill")" 2>/dev/null)" "failed/wrapper_died"
    check "B1d at the moment the op is released the container is already gone (the budget is never freed while it runs)" "$(b1_up "$B1TOK-kill")" 0
    [ "$EL" -le 25 ] && ok "B1d released ${EL}s after the SIGKILL" || bad "B1d released ${EL}s after the SIGKILL"
    check "B1d envelope: the budget of the dead run is not counted any more (used_mem_bytes 0)" "$(cd "$ROOT" && env PATH="$ORIG_PATH" ENVELOPE_TEST_MODE=1 bash "$SUTDIR/envelope.sh" --format json 2>/dev/null | jq -r .used_mem_bytes)" 0
  else
    echo "SKIP: IMG-TESTUTIL is not in the lock on this host: the B1 real legs are not run (not faked)"
  fi
fi

# ============================================================ F15 (B1, fake podman): a container that cannot be stopped keeps its op registered, exit 125
if want F15; then
  resetlogs; newreg; export SHIM_SLEEP=60 SHIM_STUBBORN=1
  S0=$(date +%s)
  wr run_go --out "$T/o-f15" --op-id f15 --wall-s 2 -- true
  EL=$(( $(date +%s) - S0 ))
  check "F15 a container that survives podman stop and kill: the wrapper exits 125 (not 124, not 0)" "$RC" 125
  [ "$EL" -le 60 ] && ok "F15 the effort was bounded (${EL}s)" || bad "F15 took ${EL}s"
  check "F15 the op is NOT released while its container is Up (state is not terminal)" "$(case "$(jq -r .state "$(opf f15)")" in complete|failed|reaped|handoff|blocked-escape) echo terminal;; *) echo live;; esac)" live
  grep -q 'could not be stopped' "$T/stderr" && ok "F15 the wrapper says the container could not be stopped" || bad "F15 stderr: $(cat "$T/stderr")"
  p="$(head -1 "$SHIM_CDIR/f15.c")"; [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && [ "$({ tr '\0' ' ' <"/proc/$p/cmdline"; } 2>/dev/null)" = "sleep 600 " ] && kill -KILL "$p" 2>/dev/null
  unset SHIM_STUBBORN
fi

# ============================================================ F11 + TREE: the real container leg (IMG-TESTUTIL), the real evidence tree stays untouched
if want F11 && [ "${R1_TEST_NO_REAL:-0}" != 1 ]; then
  EVD="$ROOT/specs/001-full-project-audit-remediation/evidence/disk"
  TOK="r1t$$x$RANDOM"
  before="$(ls "$EVD" 2>/dev/null | sort | sha256sum | cut -c1-64)"
  mkwrapper() { # <file> <version command>
    cat >"$1" <<EOF
#!/usr/bin/env bash
set -u
RUNNER_NAME=run_testutil
RUNNER_IMAGES="IMG-TESTUTIL"
RUNNER_TOOLCHAIN=testutil
RUNNER_PROBE_MODE=sh
RUNNER_PROBE_VERSION='$2'
RUNNER_CMD_PREFIX=''
RUNNER_BLOCKED_NOTE='IMG-TESTUTIL is pinned'
. "$SUTDIR/runner_lib.sh"
runner_main "\$@"
EOF
  }
  realrun() { local w=$1 o=$2; shift 2; ( cd "$ROOT" && env -u RUNNER_RUNP -u RUNP_LOCK -u ENVELOPE_MEMINFO -u ENVELOPE_NPROC -u ENVELOPE_ULIMIT_U PATH="$ORIG_PATH" RUNNER_LOG_DIR="$T/real-logs" bash "$w" --out "$o" "$@" ) >"$T/stdout" 2>"$T/stderr"; RC=$?; }
  if python3 -I - "$ROOT/build/containers/images.lock.yaml" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
sys.exit(0 if any(isinstance(i, dict) and i.get("id") == "IMG-TESTUTIL" for i in d.get("images", [])) else 1)
PY
  then
    newreg
    mkwrapper "$T/wr_bad.sh" 'r1-no-such-tool --version'
    realrun "$T/wr_bad.sh" "$T/real-bad" --op-id "$TOK-bad" -- python3 -c 'print("main-ran")'
    check "F11 a failing version command is a refusal (exit 1), the run does not start" "$RC" 1
    refused version_probe_failed && ok "F11 reason version_probe_failed" || bad "F11 reason: $(grep -h 'REFUSED\|version' "$T/stderr" | head -3) / record: $(jq -c '.version' "$T/real-bad/toolchain.json" 2>/dev/null)"
    grep -q 'main-ran' "$T/stdout" && bad "F11 the main run started after a failed version probe" || ok "F11 the main run did not start"
    mkwrapper "$T/wr_good.sh" 'python3 --version'
    realrun "$T/wr_good.sh" "$T/real-good" --op-id "$TOK-good" -- python3 -c 'print("main-ran")'
    check "F11 control: a working version command runs" "$RC" 0
    case "$(jq -r .version "$T/real-good/toolchain.json" 2>/dev/null)" in Python\ 3*) ok "F11 control: the record carries the real tool version";; *) bad "F11 control: record version '$(jq -r .version "$T/real-good/toolchain.json" 2>/dev/null)'";; esac
    check "F11 control: the control needle (write to the read-only source mount fails) is yes" "$(jq -r .probes.src_readonly "$T/real-good/toolchain.json" 2>/dev/null)" yes
    after="$(ls "$EVD" 2>/dev/null | sort | sha256sum | cut -c1-64)"
    [ "$(count_tok "$EVD" "$TOK")" = 0 ] && ok "TREE no disk-headroom record of these real runs was written into the real evidence/disk" || bad "TREE records of this test were written into $EVD: $(for _f in "$EVD"/*"$TOK"*; do echo "$_f"; done | head -3)"
    [ "$(count_tok "$DISK_HEADROOM_OUT_DIR" "$TOK")" -ge 1 ] && ok "TREE the records went to DISK_HEADROOM_OUT_DIR in scratch ($(count_tok "$DISK_HEADROOM_OUT_DIR" "$TOK") records)" || bad "TREE no record in the scratch DISK_HEADROOM_OUT_DIR"
    echo "  (evidence/disk listing hash before/after: $before / $after; other agents may add files meanwhile, only files carrying this run's token count)"
  else
    echo "SKIP: IMG-TESTUTIL is not in the lock on this host: the F11 real leg is not run (not faked)"
  fi
fi

echo "RESULT pass=$PASSES fail=$FAILS"
[ "$FAILS" = 0 ] || EXIT=1
EXIT="${EXIT:-0}"

# ============================================================ paired mutations
if [ "${R1_TEST_MUTANT:-0}" = 1 ] || [ "${R1_TEST_NO_MUTATIONS:-0}" = 1 ]; then exit "$EXIT"; fi
MREC="${R1_MUTATION_RECORD:-$T/mutation.txt}"; : >"$MREC"
CAUGHT=0; SURV=0; TOTAL=0
# mut_case <id> <blocks> <expected check-name substring> <old> <new> [file]: <file> is containers/runner_lib.sh (default) or test-in-container.sh
place_tree() { # <dir>: a working tree layout: scripts/containers (a copy), scripts/test-in-container.sh (a copy), scripts/longops + anti-mess + build (links)
  local d=$1
  rm -rf "$d"; mkdir -p "$d/scripts" "$d/tools"
  cp -r "$SUTDIR" "$d/scripts/containers" 2>/dev/null; rm -rf "$d/scripts/containers/tests"
  [ ! -f "$SUTDIR/../test-in-container.sh" ] || cp "$SUTDIR/../test-in-container.sh" "$d/scripts/test-in-container.sh"
  for l in longops anti-mess; do ln -s "$ROOT/scripts/$l" "$d/scripts/$l"; done
  ln -s "$ROOT/build" "$d/build"
}
mut_case() {
  local id=$1 blocks=$2 want_sub=$3 old=$4 new=$5 file="${6:-containers/runner_lib.sh}" d="$T/mut-$1"
  TOTAL=$((TOTAL+1))
  place_tree "$d"
  python3 -I - "$d/scripts/$file" "$old" "$new" <<'PY' || { echo "INVALID $id: pattern not found exactly once" | tee -a "$MREC"; SURV=$((SURV+1)); return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]))
PY
  ( R1_SUT_DIR="$d/scripts/containers" R1_ONLY="$blocks" R1_TEST_MUTANT=1 R1_TEST_NO_REAL=1 QUIET=1 bash "${BASH_SOURCE[0]}" ) >"$T/mut-$id.log" 2>&1; local rc=$?
  if [ "$rc" -ne 0 ] && grep -q "^FAIL: .*$want_sub" "$T/mut-$id.log"; then CAUGHT=$((CAUGHT+1)); echo "CAUGHT   $id by a check naming '$want_sub' ($(grep -c '^FAIL' "$T/mut-$id.log") failing checks)" | tee -a "$MREC"
  elif [ "$rc" -ne 0 ]; then SURV=$((SURV+1)); echo "SURVIVED $id (failed, but no failing check names '$want_sub': $(grep '^FAIL' "$T/mut-$id.log" | head -2 | cut -c1-120 | tr '\n' '|'))" | tee -a "$MREC"
  else SURV=$((SURV+1)); echo "SURVIVED $id (test stayed green on the mutant)" | tee -a "$MREC"; fi
}
# NEGATIVE CONTROL (review round 2 m4): an UNMUTATED copy placed like a mutant must pass the blocks the mutants run, else every "caught" is meaningless
place_tree "$T/mut-control"
( R1_SUT_DIR="$T/mut-control/scripts/containers" R1_ONLY="F1 F6 F8 F9 F10 F10b F12 F13 F14 F15" R1_TEST_MUTANT=1 R1_TEST_NO_REAL=1 QUIET=1 bash "${BASH_SOURCE[0]}" ) >"$T/mut-control.log" 2>&1; CRC=$?
if [ "$CRC" = 0 ]; then echo "CONTROL  an unmutated copy placed like a mutant passes the blocks ($(grep '^RESULT' "$T/mut-control.log"))" | tee -a "$MREC"
else echo "CONTROL FAILED: the unmutated copy fails the blocks ($(grep -c '^FAIL' "$T/mut-control.log") checks: $(grep '^FAIL' "$T/mut-control.log" | head -2 | cut -c1-100 | tr '\n' '|')): the mutation harness is blind" | tee -a "$MREC"; EXIT=1; fi
RLB='[ ! -e "$LDIR/ops/$OP_ID.json" ] || rl_refuse op_exists "op id $OP_ID is already registered; the other run'"'"'s logs and record are untouched"'
mut_case budget-lock-dropped "F1" "F1 " 'if flock -w 1 "$LOCKFD"; then _got=1; break; fi   # MUT:budget-lock' 'if true; then _got=1; break; fi'
mut_case lock-wait-foreground "F12" "F12 " 'if flock -w 1 "$LOCKFD"; then _got=1; break; fi   # MUT:budget-lock' 'if flock -w 120 "$LOCKFD"; then _got=1; break; fi'
mut_case N2-unlock-before-register "F1" "F1 " 'REG_ERR="$(bash "$ROOT_DIR/scripts/longops/register.sh"' 'flock -u "$LOCKFD"; REG_ERR="$(bash "$ROOT_DIR/scripts/longops/register.sh"'
mut_case mem-above-envelope-accepted "F1" "F1 two of the six" '[ "$LIM_MEM" -le "$ENV_MEM" ] || rl_refuse limit_exceeds_envelope "--memory $LIM_MEM is above the envelope memory $ENV_MEM"' 'true'
mut_case scrub-dropped "F4" "F4 " 'RUNP_*) echo "$RUNNER_NAME: note: ignored inherited $_v (the wrapper owns every RUNP_* control but RUNP_LOCK and RUNP_USER)" >&2; unset "$_v";;' 'RUNP_*) ;;'
mut_case op-exists-check-dropped "F5" "F5 " "$RLB" 'true'
mut_case I2-stop-named-before-op-exists "F13" "F13 the wall marker" "$RLB" "STOP=\"\$LOGDIR/\$OP_ID.stop\"; $RLB"
mut_case trap-not-installed "F6" "F6 " 'trap rl_on_signal TERM INT HUP' 'true'
mut_case signal-verdict-flipped "F6" "F6 TERM during the run" 'if [ "$SIGNALLED" = 1 ]; then rl_release_op 1 interrupted; exit "$(rl_end_code 130)"; fi   # MUT:signal-release' 'if [ "$SIGNALLED" = 1 ]; then rl_release_op 0 interrupted; exit "$(rl_end_code 130)"; fi'
mut_case probe-checkpoint-dropped "F6" "F6 " 'rl_bg_wait; PROBE_RC=$RL_RC; rl_checkpoint
    PROBE_OUT="$(cat "$PROBE_FILE" 2>/dev/null)"
    P_VERSION="$(printf '"'"'%s\n'"'"' "$PROBE_OUT" | sed -n' 'rl_bg_wait; PROBE_RC=$RL_RC
    PROBE_OUT="$(cat "$PROBE_FILE" 2>/dev/null)"
    P_VERSION="$(printf '"'"'%s\n'"'"' "$PROBE_OUT" | sed -n'
mut_case wall-enforce-dropped "F8" "F8 " '[ "$WALL_S" -gt 0 ] && [ "$walled" = 0 ] && [ "$el" -gt $(( WALL_S * 1000 )) ]' 'false'
mut_case N3-wall-clock-times-3 "F8" "F8 the heartbeat reported the elapsed time" '[ "$el" -gt $(( WALL_S * 1000 )) ]' '[ "$el" -gt $(( WALL_S * 3000 )) ]'
mut_case wall-does-not-stop-the-container "F8" "F8 the run completed" 'rl_terminate_run || : >"$STOP.held"   # MUT:wall-terminate' ':'
mut_case release-does-not-stop-the-container "F6" "F6 TERM during the run: the container" '[ -e "${STOP:-/nonexistent}.held" ] || ! rl_ensure_down; then' '[ -e "${STOP:-/nonexistent}.held" ] || false; then'
mut_case bg-wait-blind-to-a-signal "F6" "F6 TERM during the run" 'while [ "$SIGNALLED" = 0 ] && rl_palive "$CH" "$CH_ST"; do sleep 0.2; done' 'while rl_palive "$CH" "$CH_ST"; do sleep 0.2; done'
mut_case release-ignores-live-container "F15" "F15 the op is NOT released" 'if [ "$OP_HOLD" = 1 ] || [ -e "${STOP:-/nonexistent}.held" ] || ! rl_ensure_down; then' 'if false; then'
mut_case parent-death-does-not-stop-the-container "F10" "F10 " '      if rl_terminate_run; then
        bash' '      if true; then
        bash'
mut_case parent-liveness-dropped "F10" "F10 " 'if [ "$(rl_pstart "$wp")" != "$wst" ]; then   # MUT:parent-liveness' 'if false; then   # MUT:parent-liveness'
mut_case guard-loop-not-started-at-registration "F10b" "F10b " '  ( rl_hb_loop "$$" "$(rl_pstart "$$")" ) >/dev/null 2>&1 &   # the guard loop: heartbeats, wall clock, parent liveness; it covers the toolchain probe too
  HBP=$!' '  :'
mut_case out-bytes-not-progress "F9" "F9 a run that writes only /out" 'c="$(find "$OUT" -type f -printf '"'"'%s\n'"'"' 2>/dev/null | awk '"'"'{s += $1} END {print s + 0}'"'"')"' 'c=0'
mut_case progress-offset-zero "F9" "F9 a run that writes only /out" '--progress-offset "$(rl_progress)" --elapsed-ms "$el" >/dev/null 2>&1; rc=$?' '--progress-offset 0 --elapsed-ms "$el" >/dev/null 2>&1; rc=$?'
mut_case I1-dispatcher-passes-memory-again "F14" "F14 " 'bash "$WDIR/$WRAPPER.sh" "${PASS[@]}" -- "${CMD[@]}"   # MUT:no-limits' 'bash "$WDIR/$WRAPPER.sh" --memory "$(bash "$CDIR/envelope.sh" --toolchain "${WRAPPER#run_}" --format json | jq -r .memory_bytes)" "${PASS[@]}" -- "${CMD[@]}"' "test-in-container.sh"
mut_case I1-dispatcher-passes-cpus-again "F14" "F14 " 'bash "$WDIR/$WRAPPER.sh" "${PASS[@]}" -- "${CMD[@]}"   # MUT:no-limits' 'bash "$WDIR/$WRAPPER.sh" --cpus "$(bash "$CDIR/envelope.sh" --toolchain "${WRAPPER#run_}" --format json | jq -r .cpus)" "${PASS[@]}" -- "${CMD[@]}"' "test-in-container.sh"
echo "MUTATION RESULT caught=$CAUGHT survived=$SURV total=$TOTAL" | tee -a "$MREC"
[ "$SURV" = 0 ] || EXIT=1
exit "$EXIT"
