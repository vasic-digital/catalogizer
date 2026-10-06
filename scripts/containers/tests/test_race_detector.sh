#!/usr/bin/env bash
# test_race_detector.sh - T124 (RED first, test-first part). The Go race detector must run INSIDE the pinned IMG-GO container, through run_go.sh,
# and a seeded data race must make that run FAIL while the fixed copy passes.
# Legs:
#   L1 (real containers, the part that needs no change to the existing script): a fixture package with a seeded data race and its fixed copy are
#      WRITTEN BY THIS TEST into a temporary directory at run time (a deliberate-violation fixture for the anti-bluff ratchet, never committed). The race
#      command of docs/16 6.2 / T124 - `GOMAXPROCS=3 go test -race -p 2 -parallel 2 -json`, output `/out/go-test.jsonl` - is run through
#      scripts/containers/run_go.sh in the pinned IMG-GO. Oracles: (a) the exit code, (b) the machine output /out/go-test.jsonl read with jq
#      (an independent oracle: a `DATA RACE` output event and a failed test event for the racy copy, a passed package and no race event for the fixed
#      copy), (c) facts printed BY THE FIXTURE from inside the container (race detector build tag, GOMAXPROCS) so that "failed for another reason" and
#      "the limit was not applied" are both distinguishable. Negative control: the racy copy run WITHOUT -race exits 0 and reports no race (the
#      failure is the detector's, not the fixture's).
#   L2 (shims): the command the lane runs reaches run_pinned.sh through run_go.sh with the GOMAXPROCS=3 / local-toolchain / CGO prefix and the
#      envelope limits (a run_pinned.sh shim records it).
#   L3 (RED against the CURRENT script): scripts/run-race-detector.sh must go through the container, not the bare host. A `go` shim on PATH records a
#      bare-host `go test`, a `podman` shim records any container run: the current script calls the bare-host go and starts no container, so L3 FAILS
#      today. L3 is the T124 migration contract; it turns GREEN only when scripts/run-race-detector.sh is moved to run_go.sh (and, as a remote lane, through
#      scripts/build/dispatch.sh, T121a / T005b).
#   BLOCKED (reported as SKIP with its reason, never a pass): the race report through tools/evidence/wrap-go.sh needs T051, which is not implemented.
# Usage: test_race_detector.sh      RACE_TEST_SKIP_L3=1 ...  runs L1 and L2 only (the part that is GREEN-able without touching the existing script)
#        RACE_SUT=<script> ...      the race-detector script L3 tests (default scripts/run-race-detector.sh)
# Env:   RACE_RESULT_JSON  file receiving a machine-readable summary of the run
# Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
RUNGO="${RACE_RUN_GO:-$REPO/scripts/containers/run_go.sh}"
SUT="${RACE_SUT:-$REPO/scripts/run-race-detector.sh}"
FAILS=0; PASSES=0; SKIPS=0
# count_tok <dir> <token>: how many files in <dir> carry <token> in their name (a glob, never ls | grep)
count_tok() { local n=0 f; for f in "$1"/*"$2"*; do [ -e "$f" ] && n=$((n+1)); done; echo "$n"; }
ok()   { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad()  { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
skip() { SKIPS=$((SKIPS+1)); echo "SKIP: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required by this test"; exit 2; }; done
command -v podman >/dev/null 2>&1 || { echo "FAIL: podman is required by this test"; exit 2; }
[ -f "$RUNGO" ] || { echo "FAIL: run_go.sh not found at $RUNGO"; echo "RESULT pass=$PASSES fail=1 skip=$SKIPS"; exit 1; }
REAL_PODMAN="$(command -v podman)"
T="$(mktemp -d "${TMPDIR:-/tmp}/race-test.XXXXXX")"
trap 'rm -rf "$T"' EXIT

# ---- the fixture packages (written at run time, never committed) ----
mkfix() { # <dir> <racy|fixed>
  local d=$1 kind=$2
  mkdir -p "$d"
  printf 'module racefix\n\ngo 1.25\n' >"$d/go.mod"
  printf '//go:build race\n\npackage racefix\n\nconst raceMode = "enabled"\n' >"$d/race_on.go"
  printf '//go:build !race\n\npackage racefix\n\nconst raceMode = "disabled"\n' >"$d/race_off.go"
  if [ "$kind" = racy ]; then
    cat >"$d/counter_test.go" <<'EOF'
package racefix

import (
	"runtime"
	"sync"
	"testing"
)

// SEEDED DATA RACE: two goroutines increment n with no synchronisation.
func TestSharedCounter(t *testing.T) {
	t.Logf("FACT race=%s gomaxprocs=%d", raceMode, runtime.GOMAXPROCS(0))
	var n int
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				n++
			}
		}()
	}
	wg.Wait()
	t.Logf("n=%d", n)
}
EOF
  else
    cat >"$d/counter_test.go" <<'EOF'
package racefix

import (
	"runtime"
	"sync"
	"testing"
)

// The fixed copy: the same two goroutines, the increment guarded by a mutex.
func TestSharedCounter(t *testing.T) {
	t.Logf("FACT race=%s gomaxprocs=%d", raceMode, runtime.GOMAXPROCS(0))
	var n int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				mu.Lock()
				n++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if n != 2000 {
		t.Fatalf("n=%d, want 2000", n)
	}
	t.Logf("n=%d", n)
}
EOF
  fi
}
mkfix "$T/racy" racy; mkfix "$T/fixed" fixed

# the lane command: the one of docs/16 6.2 / T124, with the machine output in /out
race_cmd() { # <extra go test flags>
  printf '%s' "go test $1 -p 2 -parallel 2 -count=1 -json ./... > /out/go-test.jsonl"
}
printf '#!/usr/bin/env bash\nexit 0\n' >"$T/sweep-shim.sh"
# an isolated long-op registry: the test never writes the real .audit/
mkdir -p "$T/reg/repo/.audit"
export LONGOPS_REPO="$T/reg/repo" LONGOPS_DIR="$T/reg/repo/.audit/longops" LONGOPS_AUDIT="$T/reg/repo/.audit" LONGOPS_ALLOW_TMPFS=1 ENVELOPE_TEST_MODE=1
# the disk-headroom records of the real runs go to scratch; the ones of run_go legs carry a token so the real evidence/disk can be checked untouched
export DISK_HEADROOM_OUT_DIR="$T/disk"; mkdir -p "$DISK_HEADROOM_OUT_DIR"
TOK="rd$$x$RANDOM"; RN=0
# run_go <fixture-dir> <out-dir> <go test flags>: the fixture dir is the working directory, hence /src of the container; sets RC, RUNLOG
rungo() {
  local dir=$1 out=$2 flags=$3
  RN=$((RN+1))
  ( cd "$dir" && env RUNNER_TEST_MODE=1 RUNNER_SWEEP="$T/sweep-shim.sh" bash "$RUNGO" --out "$out" --op-id "$TOK-$RN" -- sh -c "$(race_cmd "$flags")" ) >"$T/stdout" 2>"$T/stderr"; RC=$?
}
out_events() { jq -c 'select(.Action=="output") | .Output' "$1/go-test.jsonl" 2>/dev/null; }

# ============== instrument check (control needle): the jq reader sees a race line and does not see one that is not there ==============
printf '%s\n' '{"Action":"output","Test":"T","Output":"WARNING: DATA RACE\n"}' >"$T/needle.jsonl"
printf '%s\n' '{"Action":"output","Test":"T","Output":"ok\n"}' >"$T/needle-neg.jsonl"
mkdir -p "$T/n1" "$T/n2"; cp "$T/needle.jsonl" "$T/n1/go-test.jsonl"; cp "$T/needle-neg.jsonl" "$T/n2/go-test.jsonl"
out_events "$T/n1" | grep -q 'DATA RACE' && ok "instrument: the jsonl reader sees a DATA RACE output event (needle)" || bad "instrument blind: cannot see a seeded race line"
out_events "$T/n2" | grep -q 'DATA RACE' && bad "instrument: reports a race that is not there" || ok "instrument: no false race on a clean line (negative control)"

# ============== L1: the real containers, through run_go.sh ==============
START=$(date +%s)
rungo "$T/racy" "$T/out-racy" "-race"
echo "  (racy -race run: rc=$RC, $(( $(date +%s) - START )) s)"
RACY_RC=$RC
if grep -q 'REFUSED' "$T/stderr"; then bad "L1 racy: run_go.sh refused the run: $(grep REFUSED "$T/stderr" | head -2)"; fi
[ "$RACY_RC" -ne 0 ] && ok "L1 racy: the containerized -race run exits non-zero on the seeded race (rc $RACY_RC)" || bad "L1 racy: exit 0 on a seeded data race (a bluff)"
[ -s "$T/out-racy/go-test.jsonl" ] && ok "L1 racy: /out/go-test.jsonl was written by the run" || bad "L1 racy: no go-test.jsonl"
out_events "$T/out-racy" | grep -q 'DATA RACE' && ok "L1 racy: the machine output carries a 'DATA RACE' event" || bad "L1 racy: no DATA RACE in go-test.jsonl"
check "L1 racy: the test event is a failure" "$(jq -r 'select(.Action=="fail" and .Test=="TestSharedCounter") | .Action' "$T/out-racy/go-test.jsonl" 2>/dev/null | head -1)" fail
out_events "$T/out-racy" | grep -q 'FACT race=enabled' && ok "L1 racy: the fixture printed race=enabled from inside the container (the detector was on)" || bad "L1 racy: the detector was not enabled"
out_events "$T/out-racy" | grep -q 'gomaxprocs=3' && ok "L1 racy: GOMAXPROCS=3 inside the container (the docs/16 limit was applied)" || bad "L1 racy: GOMAXPROCS is not 3: $(out_events "$T/out-racy" | grep FACT | head -1)"
check "L1 racy: the toolchain record says the digest matched" "$(jq -r .digest_match "$T/out-racy/toolchain.json" 2>/dev/null)" true
OPR="$(ls "$LONGOPS_DIR"/ops/*.json 2>/dev/null | head -1)"
[ -n "$OPR" ] && check "L1 racy: the registered long op ended failed with verdict rc=$RACY_RC" "$(jq -r '.state + "/" + .verdict' "$OPR")" "failed/rc=$RACY_RC" || bad "L1 racy: no registered op"

rm -rf "$LONGOPS_DIR"; mkdir -p "$LONGOPS_DIR"
START=$(date +%s)
rungo "$T/fixed" "$T/out-fixed" "-race"
echo "  (fixed -race run: rc=$RC, $(( $(date +%s) - START )) s)"
FIXED_RC=$RC
check "L1 fixed: the containerized -race run exits 0 on the fixed copy" "$FIXED_RC" 0
out_events "$T/out-fixed" | grep -q 'DATA RACE' && bad "L1 fixed: a race is reported on the fixed copy" || ok "L1 fixed: no DATA RACE event on the fixed copy"
check "L1 fixed: the package passed" "$(jq -r 'select(.Action=="pass" and .Test==null) | .Action' "$T/out-fixed/go-test.jsonl" 2>/dev/null | head -1)" pass
out_events "$T/out-fixed" | grep -q 'FACT race=enabled' && ok "L1 fixed: race=enabled inside the container" || bad "L1 fixed: the detector was not enabled"
OPF="$(ls "$LONGOPS_DIR"/ops/*.json 2>/dev/null | head -1)"
[ -n "$OPF" ] && check "L1 fixed: the registered long op ended complete" "$(jq -r .state "$OPF")" complete || bad "L1 fixed: no registered op"
# the same racy copy WITHOUT -race: exit 0 and no race event - the failure above is the detector's
rm -rf "$LONGOPS_DIR"; mkdir -p "$LONGOPS_DIR"
rungo "$T/racy" "$T/out-norace" ""
check "L1 negative control: the racy copy without -race exits 0 (the detector is what fails it)" "$RC" 0
out_events "$T/out-norace" | grep -q 'DATA RACE' && bad "L1 negative control: a race reported without -race" || ok "L1 negative control: no race event without -race"
out_events "$T/out-norace" | grep -q 'FACT race=disabled' && ok "L1 negative control: race=disabled inside the container" || bad "L1 negative control: the build tag fact is missing"

# ============== L2: the lane command reaches run_pinned.sh through run_go.sh, with the limits (shims) ==============
SH="$T/shims"; mkdir -p "$SH"
cat >"$SH/run_pinned.sh" <<'SH2'
#!/usr/bin/env bash
prev=""; for a in "$@"; do [ "$prev" = --out ] && mkdir -p -- "$a"; prev=$a; done
{ echo "---CALL---"; printf 'ARG:%s\n' "$@"; printf 'ENV:RUNP_MEMORY=%s RUNP_CPUS=%s RUNP_PIDS=%s\n' "${RUNP_MEMORY-}" "${RUNP_CPUS-}" "${RUNP_PIDS-}"; } >>"${SHIM_LOG:?}"
case " $* " in *cpa-probe*) printf 'version=go version go1.25\nout_writable=yes\ncache_writable=yes\nsrc_readonly=yes\n';; esac
exit 0
SH2
chmod +x "$SH/run_pinned.sh"
rm -rf "$LONGOPS_DIR"; mkdir -p "$LONGOPS_DIR"; : >"$T/shim.log"
mkdir -p "$T/ck"
( cd "$T/ck" && env SHIM_LOG="$T/shim.log" PATH="$SH:$PATH" RUNNER_TEST_MODE=1 RUNNER_RUNP="$SH/run_pinned.sh" RUNNER_SWEEP="$T/sweep-shim.sh" bash "$RUNGO" --out "$T/o-shim" -- sh -c "$(race_cmd "-race")" ) >"$T/stdout" 2>"$T/stderr"; RC=$?
check "L2 shim: the lane command runs through run_go.sh (exit 0)" "$RC" 0
tr '\n' ' ' <"$T/shim.log" | grep -q 'ARG:env ARG:GOMAXPROCS=3 ARG:GOTOOLCHAIN=local ARG:CGO_ENABLED=1 ARG:sh ARG:-c ARG:go test -race -p 2 -parallel 2 -count=1 -json \./\.\.\. > /out/go-test\.jsonl' && ok "L2 shim: GOMAXPROCS=3 + the docs/16 flags -race -p 2 -parallel 2 -json reach run_pinned.sh in the IMG-GO container call" || bad "L2 shim: $(tr '\n' ' ' <"$T/shim.log")"
grep -qxF 'ARG:IMG-GO' "$T/shim.log" && ok "L2 shim: the image is IMG-GO" || bad "L2 shim: image missing"
grep -q 'ENV:RUNP_MEMORY=[0-9][0-9]* RUNP_CPUS=[0-9][0-9]* RUNP_PIDS=[0-9][0-9]*' "$T/shim.log" && ok "L2 shim: the envelope limits (--memory, --cpus, --pids-limit) are handed to run_pinned.sh as container limits" || bad "L2 shim: limits missing"

# ============== wrap-go.sh: BLOCKED ==============
if [ -f "$REPO/tools/evidence/wrap-go.sh" ]; then
  skip "tools/evidence/wrap-go.sh exists, but its integration (the T051 / T124 GREEN step) is not asserted by this test: counted as a skip, never as a pass (review M3)"
else
  skip "BLOCKED-ON-T051: tools/evidence/wrap-go.sh does not exist, so the race report through it is not asserted; the machine output is read with jq above"
fi

# ============== L3: RED against the CURRENT scripts/run-race-detector.sh ==============
if [ "${RACE_TEST_SKIP_L3:-0}" = 1 ]; then
  skip "L3 skipped by RACE_TEST_SKIP_L3=1"
elif [ ! -f "$SUT" ]; then
  bad "L3: $SUT not found"
else
  L3="$T/l3"; mkdir -p "$L3/shims"
  cat >"$L3/shims/go" <<EOF
#!/usr/bin/env bash
echo "BARE_HOST_GO \$*" >>"$L3/bare.log"
exit 0
EOF
  cat >"$L3/shims/podman" <<EOF
#!/usr/bin/env bash
case "\$1" in
  run) { echo "PODMAN_RUN"; printf 'A:%s\n' "\$@"; } >>"$L3/podman.log"; exit 0;;
esac
exec "$REAL_PODMAN" "\$@"
EOF
  chmod +x "$L3/shims/go" "$L3/shims/podman"; : >"$L3/bare.log"; : >"$L3/podman.log"
  rm -rf "$LONGOPS_DIR"; mkdir -p "$LONGOPS_DIR"
  ( cd "$REPO" && env PATH="$L3/shims:$PATH" RUNNER_TEST_MODE=1 RUNNER_SWEEP="$T/sweep-shim.sh" bash "$SUT" --api-only ) >"$L3/stdout" 2>"$L3/stderr"; L3RC=$?
  echo "  (script run: rc=$L3RC, bare-host go calls=$(grep -c '^BARE_HOST_GO' "$L3/bare.log"), container runs=$(grep -c '^PODMAN_RUN' "$L3/podman.log"))"
  [ "$(grep -c '^BARE_HOST_GO' "$L3/bare.log")" = 0 ] && ok "L3: the race-detector script runs NO bare-host go" || bad "L3 RED: scripts/run-race-detector.sh runs go on the bare host ($(head -1 "$L3/bare.log"))"
  grep -q '^PODMAN_RUN' "$L3/podman.log" && grep -q 'go test' "$L3/podman.log" && ok "L3: a container run executes go test" || bad "L3 RED: the script starts no container (no podman run executing go test)"
  grep -q "sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437" "$L3/podman.log" && ok "L3: the container is the IMG-GO digest of the lock" || bad "L3 RED: the IMG-GO digest is not in any container run"
  grep -qx 'A:--memory' "$L3/podman.log" && ok "L3: the container run carries --memory" || bad "L3 RED: no --memory on any container run"
fi

if [ -n "${RACE_RESULT_JSON:-}" ]; then
  jq -nc --argjson p "$PASSES" --argjson f "$FAILS" --argjson s "$SKIPS" --arg racy "$RACY_RC" --arg fixed "$FIXED_RC" '{schema:"race-test/1", pass:$p, fail:$f, skip:$s, racy_rc:($racy|tonumber), fixed_rc:($fixed|tonumber)}' >"$RACE_RESULT_JSON"
fi
EVD="$REPO/specs/001-full-project-audit-remediation/evidence/disk"
check "no disk-headroom record carrying this run's token ($TOK) was written into the real evidence/disk" "$(count_tok "$EVD" "$TOK")" 0
echo "RESULT pass=$PASSES fail=$FAILS skip=$SKIPS"
[ "$FAILS" = 0 ]
