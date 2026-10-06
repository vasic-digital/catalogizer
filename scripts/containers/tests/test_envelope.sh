#!/usr/bin/env bash
# test_envelope.sh - T118 (RED first) / T119 (GREEN). Oracle for scripts/containers/envelope.sh, the docs/16 section 8.2 dynamic envelope.
# Oracle strategy (11.4.245): SPECIFIED (hand-computed goldens from the section 8.2 formulas on fixture meminfo/nproc) and DERIVED (an
# independent python3 implementation of the same formulas, compared over a seeded random sweep); neither shares code with the SUT.
# Inputs are fixtures only: ENVELOPE_MEMINFO / ENVELOPE_NPROC / ENVELOPE_ULIMIT_U (test hooks, honoured only with ENVELOPE_TEST_MODE=1),
# a fixture long-op registry (LONGOPS_DIR) filled by the real scripts/longops/register.sh, and a fixture profile.json.
# Paired mutations: copies of envelope.sh with ONE load-bearing expression changed (python str replace, exactly one occurrence);
# the test body is re-run against each copy (ENVELOPE_SUT=<copy>, ENVELOPE_TEST_MUTANT=1) and every copy must make it FAIL.
# Usage: test_envelope.sh            tests and mutations
#        ENVELOPE_TEST_NO_MUTATIONS=1 ...   tests only
# Env:   ENVELOPE_MUTATION_RECORD  file that receives one line per mutation (default: scratch)
# Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
SUT="${ENVELOPE_SUT:-$HERE/../envelope.sh}"
FAILS=0; PASSES=0
ok()  { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required by this test"; exit 2; }; done
[ -f "$SUT" ] || { echo "FAIL: envelope.sh not found at $SUT"; exit 1; }

T="$(mktemp -d "${TMPDIR:-/tmp}/envelope-test.XXXXXX")"
SLEEPERS=()
cleanup() { local p; for p in "${SLEEPERS[@]:-}"; do [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && kill "$p" 2>/dev/null; done; rm -rf "$T"; }
trap cleanup EXIT
export ENVELOPE_TEST_MODE=1
# an isolated, empty long-op registry unless a case fills it
newreg() { rm -rf "$T/reg"; mkdir -p "$T/reg/repo/.audit"; export LONGOPS_REPO="$T/reg/repo" LONGOPS_DIR="$T/reg/repo/.audit/longops" LONGOPS_AUDIT="$T/reg/repo/.audit" LONGOPS_ALLOW_TMPFS=1; }
newreg
GIB=1073741824

mi() { printf 'MemTotal:       %s kB\nMemAvailable:   %s kB\n' "$1" "$2" >"$T/meminfo"; }
# env <memtotal_kb> <memavail_kb> <nproc> [sut args...]: runs the SUT on fixtures, stdout -> $T/out, stderr -> $T/err, sets RC
env_run() {
  local mt=$1 ma=$2 np=$3; shift 3
  mi "$mt" "$ma"
  ( ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC="$np" ENVELOPE_ULIMIT_U="${UL:-100000}" bash "$SUT" "$@" ) >"$T/out" 2>"$T/err"; RC=$?
}
kv() { sed -n "s/^$1=//p" "$T/out" | head -1; }
profile() { # <toolchain> <per_job_bytes-or-raw-json-value>
  printf '{"toolchains":{"%s":{"per_job_mem_bytes":%s}}}\n' "$1" "$2" >"$T/profile.json"
}

# ---------- golden 1 (hand computed from section 8.2): 32000000 kB total, 30000000 kB available, 16 CPUs, per_job 1.5 GiB ----------
# mem_total=32768000000 reserve_cpu=max(2,ceil(16/8))=2 reserve_mem=max(4294967296,4915200000)=4915200000 ceiling=19660800000
# mem_budget=min(19660800000, 30720000000-4915200000)=19660800000 cpu_budget=14 jobs=min(14, floor(19660800000/1610612736)=12)=12
profile go 1610612736
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "golden: exit 0" "$RC" 0
check "golden: schema" "$(kv schema)" "envelope/1"
check "golden: nproc" "$(kv nproc)" 16
check "golden: reserve_cpu = max(2, ceil(0.125*16)) = 2" "$(kv reserve_cpu)" 2
check "golden: reserve_mem = max(4 GiB, 0.15*MemTotal)" "$(kv reserve_mem_bytes)" 4915200000
check "golden: ceiling_mem = floor(0.60*MemTotal)" "$(kv ceiling_mem_bytes)" 19660800000
check "golden: mem_budget" "$(kv mem_budget_bytes)" 19660800000
check "golden: cpu_budget" "$(kv cpu_budget)" 14
check "golden: per_job status measured" "$(kv per_job_status)" measured
check "golden: jobs = max(1, min(14, 12))" "$(kv jobs)" 12
check "golden: container memory equals mem_budget" "$(kv memory_bytes)" 19660800000
check "golden: container cpus is the cpu budget under 0.60*nproc (9)" "$(kv cpus)" 9
check "golden: pids = min(2048, min(8192, ulimit/2))" "$(kv pids)" 2048

# ---------- per_job UNKNOWN: jobs = 1 ----------
rm -f "$T/profile.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "unknown (profile absent): exit 0" "$RC" 0
check "unknown (profile absent): per_job_mem UNKNOWN" "$(kv per_job_mem_bytes)" UNKNOWN
check "unknown (profile absent): status names the reason" "$(kv per_job_status)" UNKNOWN:profile_absent
check "unknown (profile absent): jobs = 1" "$(kv jobs)" 1
profile go 1610612736
env_run 32000000 30000000 16 --toolchain node --profile "$T/profile.json"
check "unknown (toolchain not in profile): jobs = 1" "$(kv jobs)" 1
check "unknown (toolchain not in profile): status" "$(kv per_job_status)" UNKNOWN:toolchain_unmeasured
env_run 32000000 30000000 16 --profile "$T/profile.json"
check "unknown (no --toolchain): jobs = 1" "$(kv jobs)" 1
for bad_v in 0 -5 '"x"' null 1.5 '"1610612736"'; do
  profile go "$bad_v"
  env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
  check "invalid per_job value $bad_v: jobs = 1 (never a division by zero)" "$(kv jobs)" 1
  check "invalid per_job value $bad_v: status names per_job_invalid" "$(kv per_job_status)" UNKNOWN:per_job_invalid
done
printf 'not json{' >"$T/profile.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "malformed profile: jobs = 1" "$(kv jobs)" 1
check "malformed profile: status" "$(kv per_job_status)" UNKNOWN:profile_unreadable

# ---------- jobs = 1 when per_job exceeds the budget ----------
profile go $((30*GIB))
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "per_job above mem_budget: jobs = max(1, 0) = 1" "$(kv jobs)" 1
check "per_job above mem_budget: still measured" "$(kv per_job_status)" measured
# per_job smaller than budget but cpu bound: 1 GiB per job -> floor(19660800000/1073741824)=18 -> min(14,18)=14
profile go $GIB
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "cpu bound: jobs = cpu_budget" "$(kv jobs)" 14

# ---------- reserve_cpu floor and scaling ----------
env_run 32000000 30000000 1 --toolchain go --profile "$T/profile.json"
check "nproc 1: reserve_cpu floor 2" "$(kv reserve_cpu)" 2
check "nproc 1: cpu_budget may be negative (-1)" "$(kv cpu_budget)" -1
check "nproc 1: jobs still 1" "$(kv jobs)" 1
check "nproc 1: container cpus floor 1" "$(kv cpus)" 1
env_run 32000000 30000000 64 --toolchain go --profile "$T/profile.json"
check "nproc 64: reserve_cpu = ceil(64/8) = 8" "$(kv reserve_cpu)" 8
check "nproc 64: cpu_budget 56" "$(kv cpu_budget)" 56
check "nproc 64: container cpus = floor(0.60*64) = 38" "$(kv cpus)" 38
env_run 32000000 30000000 17 --toolchain go --profile "$T/profile.json"
check "nproc 17: reserve_cpu = ceil(2.125) = 3" "$(kv reserve_cpu)" 3

# ---------- reserve_mem floor of 4 GiB on a small host ----------
# 8000000 kB total = 8192000000 B; 0.15 -> 1228800000 < 4 GiB -> reserve 4294967296; ceiling 4915200000
env_run 8000000 7000000 16 --toolchain go --profile "$T/profile.json"
check "small host: reserve_mem floor is 4 GiB" "$(kv reserve_mem_bytes)" 4294967296
check "small host: ceiling_mem" "$(kv ceiling_mem_bytes)" 4915200000
# avail 7168000000 - 4294967296 = 2873032704 < ceiling -> available is the binding term
check "small host: mem_budget = MemAvailable - reserve (the lower term)" "$(kv mem_budget_bytes)" 2873032704

# ---------- refusals ----------
env_run 32000000 4000000 16 --toolchain go --profile "$T/profile.json"
check "MemAvailable below reserve: exit 1" "$RC" 1
grep -q 'reason=memory_budget_unavailable' "$T/err" && ok "MemAvailable below reserve: reason memory_budget_unavailable" || bad "reason: $(cat "$T/err")"
printf 'garbage\n' >"$T/meminfo"
( ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
check "unparsable meminfo: exit 1" "$RC" 1
grep -q 'reason=meminfo_unreadable' "$T/err" && ok "unparsable meminfo: reason meminfo_unreadable" || bad "reason: $(cat "$T/err")"
mi 32000000 30000000
( ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=0 bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
check "nproc 0: exit 1" "$RC" 1
grep -q 'reason=cpu_budget_unavailable' "$T/err" && ok "nproc 0: reason cpu_budget_unavailable" || bad "reason: $(cat "$T/err")"
( ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=0 bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
check "ulimit -u 0: exit 1 (fail closed, never the permissive 8192)" "$RC" 1
grep -q 'reason=pids_budget_unavailable' "$T/err" && ok "ulimit 0: reason pids_budget_unavailable" || bad "reason: $(cat "$T/err")"
# the test hooks are honoured only in a declared test run (a stray exported variable must never lift a ceiling)
( unset ENVELOPE_TEST_MODE; ENVELOPE_MEMINFO="$T/meminfo" bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
check "test hook without ENVELOPE_TEST_MODE: refused" "$RC" 1
grep -q 'reason=test_hook_outside_test_mode' "$T/err" && ok "hook reason test_hook_outside_test_mode" || bad "reason: $(cat "$T/err")"
( unset ENVELOPE_TEST_MODE; ENVELOPE_NPROC=4 bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
check "ENVELOPE_NPROC without test mode: refused" "$RC" 1
env_run 32000000 30000000 16 --bogus
check "usage: unknown option" "$RC" 2
env_run 32000000 30000000 16 --toolchain 'a b'
check "usage: toolchain name must be [a-z0-9_-]+" "$RC" 2
env_run 32000000 30000000 16 --format yaml
check "usage: unknown format" "$RC" 2

# ---------- ulimit -u bounds the pids limit (12.12) ----------
UL=1000 env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "pids = ulimit/2 when that is below 2048" "$(kv pids)" 500
UL=unlimited env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "pids with ulimit unlimited = 2048" "$(kv pids)" 2048
UL=100000

# ---------- registered long operations are subtracted (docs/16 8.1, 13) ----------
# two real registered operations through scripts/longops/register.sh: one live (memory 2e9, cpus 2) and one terminal (must not count)
newreg
sleep 600 >/dev/null 2>&1 & SL1=$!; SLEEPERS+=("$SL1")
sleep 600 >/dev/null 2>&1 & SL2=$!; SLEEPERS+=("$SL2")
OPA="$(bash "$REPO/scripts/longops/register.sh" --purpose t118:live --owner t118 --pid "$SL1" --memory-bytes 2000000000 --cpus 2 --no-progress-s 600 2>/dev/null)"
OPB="$(bash "$REPO/scripts/longops/register.sh" --purpose t118:done --owner t118 --pid "$SL2" --memory-bytes 5000000000 --cpus 7 2>/dev/null)"
bash "$REPO/scripts/longops/release.sh" --op-id "$OPB" --state complete --verdict ok >/dev/null 2>&1
[ -n "$OPA" ] && [ -n "$OPB" ] && ok "fixture: two operations registered through the real register.sh" || bad "fixture registration failed"
profile go $GIB
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "registry: used_mem counts the live op only" "$(kv used_mem_bytes)" 2000000000
check "registry: used_cpu counts the live op only" "$(kv used_cpu)" 2
check "registry: mem_budget = min(ceiling - used, avail - reserve)" "$(kv mem_budget_bytes)" 17660800000
check "registry: cpu_budget = nproc - reserve_cpu - used" "$(kv cpu_budget)" 12
check "registry: jobs = min(12, floor(17660800000/1073741824)=16)" "$(kv jobs)" 12
# a hung operation (owner alive, progress flat past no_progress_s) still holds its resources: it IS subtracted (clock moved 100 s forward)
LONGOPS_NOW=$(( $(date +%s) + 100 )) env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "registry: a hung (flat-progress, owner alive) op is still subtracted" "$(kv used_mem_bytes)" 2000000000
# a registered op whose owner is dead holds no resources: it is not subtracted (it is the anti-mess sweep's finding, not a budget)
kill "$SL1" 2>/dev/null; sleep 0.3
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "registry: a dead-owner op is not subtracted" "$(kv used_mem_bytes)" 0
newreg
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "registry empty: used 0" "$(kv used_mem_bytes)/$(kv used_cpu)" "0/0"

# ---------- formats ----------
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json" --format json
check "json: valid and jobs matches" "$(jq -r '.jobs' "$T/out" 2>/dev/null)" 14
check "json: schema" "$(jq -r '.schema' "$T/out" 2>/dev/null)" "envelope/1"
check "json: per_job_mem_bytes is a number when measured" "$(jq -r '.per_job_mem_bytes|type' "$T/out" 2>/dev/null)" number
rm -f "$T/profile.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json" --format json
check "json: per_job_mem_bytes is null when UNKNOWN" "$(jq -r '.per_job_mem_bytes|type' "$T/out" 2>/dev/null)" null
check "json: per_job_status carries the reason" "$(jq -r '.per_job_status' "$T/out" 2>/dev/null)" "UNKNOWN:profile_absent"

# ---------- DERIVED oracle: independent python3 formulas over a seeded random sweep ----------
profile go 1610612736
python3 -I - "$T" "$SUT" <<'PY' >"$T/sweep.txt" 2>&1
import os, random, subprocess, sys, json
T, SUT = sys.argv[1], sys.argv[2]
rnd = random.Random(20261006)
GiB = 1 << 30
bad = 0
for n in range(60):
    mt_kb = rnd.randint(4_000_000, 520_000_000)
    ma_kb = rnd.randint(mt_kb // 8, mt_kb)
    nproc = rnd.choice([1, 2, 3, 4, 8, 12, 16, 17, 24, 32, 48, 64, 96, 128])
    per_job = rnd.choice([None, 300 * 1024 * 1024, GiB, 1610612736, 3 * GiB, 20 * GiB])
    mt, ma = mt_kb * 1024, ma_kb * 1024
    reserve_cpu = max(2, -(-nproc // 8))
    reserve_mem = max(4 * GiB, mt * 15 // 100)
    ceiling = mt * 60 // 100
    mem_budget = min(ceiling, ma - reserve_mem)
    cpu_budget = nproc - reserve_cpu
    if per_job is None:
        jobs = 1
    else:
        jobs = max(1, min(cpu_budget, mem_budget // per_job))
    open(os.path.join(T, "sw_meminfo"), "w").write("MemTotal: %d kB\nMemAvailable: %d kB\n" % (mt_kb, ma_kb))
    prof = os.path.join(T, "sw_profile.json")
    if per_job is None:
        if os.path.exists(prof): os.remove(prof)
    else:
        json.dump({"toolchains": {"go": {"per_job_mem_bytes": per_job}}}, open(prof, "w"))
    env = dict(os.environ, ENVELOPE_TEST_MODE="1", ENVELOPE_MEMINFO=os.path.join(T, "sw_meminfo"), ENVELOPE_NPROC=str(nproc), ENVELOPE_ULIMIT_U="100000")
    p = subprocess.run(["bash", SUT, "--toolchain", "go", "--profile", prof, "--format", "json"], env=env, capture_output=True, text=True)
    if mem_budget < 512 * 1024 * 1024:
        if p.returncode != 1: print("MISMATCH case %d: expected refusal, rc=%d" % (n, p.returncode)); bad += 1
        continue
    if p.returncode != 0: print("MISMATCH case %d: rc=%d %s" % (n, p.returncode, p.stderr.strip())); bad += 1; continue
    d = json.loads(p.stdout)
    want = {"jobs": jobs, "mem_budget_bytes": mem_budget, "cpu_budget": cpu_budget, "reserve_cpu": reserve_cpu, "reserve_mem_bytes": reserve_mem, "ceiling_mem_bytes": ceiling}
    for k, v in want.items():
        if d.get(k) != v: print("MISMATCH case %d (mt_kb=%d ma_kb=%d nproc=%d per_job=%s): %s got %r want %r" % (n, mt_kb, ma_kb, nproc, per_job, k, d.get(k), v)); bad += 1
    if d["memory_bytes"] > ceiling: print("CEILING VIOLATED case %d" % n); bad += 1
    if d["jobs"] < 1: print("JOBS < 1 case %d" % n); bad += 1
print("sweep done mismatches=%d" % bad)
sys.exit(1 if bad else 0)
PY
SW_RC=$?
check "derived oracle: 60 seeded random cases agree with the independent python implementation" "$SW_RC" 0
[ "$SW_RC" = 0 ] || sed 's/^/    /' "$T/sweep.txt" | head -10

# ---------- negative control: the SUT must not be a constant (two different hosts must give different budgets) ----------
profile go $GIB
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"; A="$(kv mem_budget_bytes)/$(kv jobs)"
env_run 64000000 60000000 32 --toolchain go --profile "$T/profile.json"; B="$(kv mem_budget_bytes)/$(kv jobs)"
[ "$A" != "$B" ] && ok "negative control: different hosts give different envelopes ($A vs $B)" || bad "the envelope did not respond to the host ($A = $B)"

# ---------- the real host leg (no hooks): a record of what the live /proc says, internally consistent ----------
( unset ENVELOPE_TEST_MODE ENVELOPE_MEMINFO ENVELOPE_NPROC ENVELOPE_ULIMIT_U; bash "$SUT" --toolchain go --format json ) >"$T/out" 2>"$T/err"; RC=$?
if [ "$RC" = 0 ]; then
  ok "real host: exit 0"
  REAL_MT="$(awk '$1=="MemTotal:"{print $2*1024}' /proc/meminfo)"
  check "real host: ceiling_mem is 0.60*MemTotal from /proc/meminfo" "$(jq -r .ceiling_mem_bytes "$T/out")" $((REAL_MT * 60 / 100))
  check "real host: nproc equals nproc(1)" "$(jq -r .nproc "$T/out")" "$(nproc)"
  [ "$(jq -r .memory_bytes "$T/out")" -le $((REAL_MT * 60 / 100)) ] && ok "real host: container memory within the 12.6 ceiling" || bad "real host: memory above the ceiling"
else
  bad "real host: exit $RC: $(cat "$T/err")"
fi

echo "RESULT pass=$PASSES fail=$FAILS"
[ "$FAILS" = 0 ] || EXIT=1
EXIT="${EXIT:-0}"

# ---------- paired mutations (ENVELOPE_SUT is a copy with ONE expression changed; the test body must FAIL on every copy) ----------
if [ "${ENVELOPE_TEST_MUTANT:-0}" = 1 ] || [ "${ENVELOPE_TEST_NO_MUTATIONS:-0}" = 1 ]; then exit "$EXIT"; fi
MREC="${ENVELOPE_MUTATION_RECORD:-$T/mutation.txt}"; : >"$MREC"
CAUGHT=0; SURV=0; TOTAL=0
mut_case() { # <marker> <old> <new>
  local id=$1 old=$2 new=$3 cp="$T/mut-$1.sh"
  TOTAL=$((TOTAL+1))
  python3 -I - "$SUT" "$cp" "$old" "$new" <<'PY' || { echo "INVALID $id: pattern not found exactly once" | tee -a "$MREC"; SURV=$((SURV+1)); return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[3]) != 1: sys.exit(1)
open(sys.argv[2], "w").write(s.replace(sys.argv[3], sys.argv[4]))
PY
  ( ENVELOPE_SUT="$cp" ENVELOPE_TEST_MUTANT=1 QUIET=1 bash "${BASH_SOURCE[0]}" ) >"$T/mut-$id.log" 2>&1; local rc=$?
  if [ "$rc" -ne 0 ]; then CAUGHT=$((CAUGHT+1)); echo "CAUGHT   $id ($(grep -c '^FAIL' "$T/mut-$id.log") failing checks)" | tee -a "$MREC"
  else SURV=$((SURV+1)); echo "SURVIVED $id (test stayed green on the mutant)" | tee -a "$MREC"; fi
}
mut_case reserve-cpu-floor '|| RES_CPU=2' '|| RES_CPU=1'
mut_case reserve-mem-floor '|| RES_MEM=4294967296' '|| RES_MEM=0'
mut_case reserve-mem-pct 'RES_MEM=$(( MT * 15 / 100 ))' 'RES_MEM=$(( MT * 10 / 100 ))'
mut_case ceiling 'CEIL=$(( MT * 60 / 100 ))' 'CEIL=$(( MT * 70 / 100 ))'
mut_case used-mem 'A=$(( CEIL - USED_MEM ))' 'A=$(( CEIL ))'
mut_case min-term 'MEM_BUDGET=$(( A < B ? A : B ))' 'MEM_BUDGET=$(( A > B ? A : B ))'
mut_case refusal '[ "$MEM_BUDGET" -ge 536870912 ]' '[ "$MEM_BUDGET" -ge -9999999999999 ]'
mut_case cpu-budget 'CPU_BUDGET=$(( NPROC - RES_CPU - USED_CPU ))' 'CPU_BUDGET=$(( NPROC - USED_CPU ))'
mut_case jobs-min '[ "$J" -le "$CPU_BUDGET" ] || J=$CPU_BUDGET' 'true'
mut_case jobs-floor '[ "$JOBS" -ge 1 ] || JOBS=1' 'true'
mut_case unknown-jobs 'else JOBS=1; fi' 'else JOBS=$CPU_BUDGET; fi'
mut_case cpu-ceiling '[ "$CPUS" -le "$CPU_CEIL" ] || CPUS=$CPU_CEIL' 'true'
mut_case pids '[ "$PIDS" -le "$PIDS_CEIL" ] || PIDS=$PIDS_CEIL' 'true'
mut_case ulimit-closed '[ "$UL" -gt 0 ]; }; }' 'true; }; }'
mut_case test-hooks '[ "${ENVELOPE_TEST_MODE:-}" != 1 ]; then' '[ "${ENVELOPE_TEST_MODE:-}" != 1 ] && false; then'
mut_case used-classes-hung 'advancing|hung)' 'advancing)'
mut_case used-classes-dead 'advancing|hung)' 'advancing|hung|dead_owner)'
mut_case memory-output '"$JOBS" "$MEM_BUDGET" "$CPUS"' '"$JOBS" "$CEIL" "$CPUS"'
echo "MUTATION RESULT caught=$CAUGHT survived=$SURV total=$TOTAL" | tee -a "$MREC"
[ "$SURV" = 0 ] || EXIT=1
exit "$EXIT"
