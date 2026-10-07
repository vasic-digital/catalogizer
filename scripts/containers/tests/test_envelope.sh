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
( unset ENVELOPE_TEST_MODE LONGOPS_DIR LONGOPS_REPO LONGOPS_AUDIT; ENVELOPE_MEMINFO="$T/meminfo" bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
check "test hook without ENVELOPE_TEST_MODE: refused" "$RC" 1
grep -q 'reason=test_hook_outside_test_mode' "$T/err" && ok "hook reason test_hook_outside_test_mode" || bad "test hook without ENVELOPE_TEST_MODE: reason is not test_hook_outside_test_mode: $(cat "$T/err")"
( unset ENVELOPE_TEST_MODE LONGOPS_DIR LONGOPS_REPO LONGOPS_AUDIT; ENVELOPE_NPROC=4 bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
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
LONGOPS_NOW=$(( $(date +%s) + 700 )) env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "registry: a hung (flat-progress, owner alive) op is still subtracted" "$(kv used_mem_bytes)" 2000000000
check "registry: the op in that fixture really IS classified hung (the class is exercised, not assumed)" "$(LONGOPS_NOW=$(( $(date +%s) + 700 )) bash "$REPO/scripts/longops/classify.sh" --op-id "$OPA" | cut -f2)" hung
# a registered op whose owner is dead holds no resources: it is not subtracted (it is the anti-mess sweep's finding, not a budget)
kill "$SL1" 2>/dev/null; sleep 0.3
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "registry: a dead-owner op is not subtracted" "$(kv used_mem_bytes)" 0
# two live operations are SUMMED (one live op cannot tell a sum from a copy)
sleep 600 >/dev/null 2>&1 & SL3=$!; SLEEPERS+=("$SL3")
sleep 600 >/dev/null 2>&1 & SL4=$!; SLEEPERS+=("$SL4")
bash "$REPO/scripts/longops/register.sh" --purpose t118:live3 --owner t118 --pid "$SL3" --memory-bytes 2000000000 --cpus 2 --no-progress-s 600 >/dev/null 2>&1
bash "$REPO/scripts/longops/register.sh" --purpose t118:live4 --owner t118 --pid "$SL4" --memory-bytes 3000000000 --cpus 4 --no-progress-s 600 >/dev/null 2>&1
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "registry: used_mem sums two live ops (2e9 + 3e9; the dead-owner and terminal records stay out)" "$(kv used_mem_bytes)" 5000000000
check "registry: used_cpu sums two live ops (2 + 4)" "$(kv used_cpu)" 6
newreg
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "registry empty: used 0" "$(kv used_mem_bytes)/$(kv used_cpu)" "0/0"

# ---------- the registry input fails CLOSED (review F3): a reading that cannot be made is a refusal with a named reason, never "used = 0" ----------
# control (golden-TRUE): a live 6e9 op is counted and the run exits 0, so the refusals below are not a blanket refusal
newreg; sleep 600 >/dev/null 2>&1 & SL5=$!; SLEEPERS+=("$SL5")
OPX="$(bash "$REPO/scripts/longops/register.sh" --purpose t118:six --owner t118 --pid "$SL5" --memory-bytes 6000000000 --cpus 1 --no-progress-s 600 2>/dev/null)"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "fail closed control: a readable registry with a live 6e9 op: exit 0, used 6e9" "$RC/$(kv used_mem_bytes)" "0/6000000000"
jq -c '.op_id="bad2" | .budget.memory_bytes=1.5' "$LONGOPS_DIR/ops/$OPX.json" >"$LONGOPS_DIR/ops/bad2.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "fail closed: a record with a float budget (1.5) refuses (exit 1), it does not zero the whole sum" "$RC" 1
grep -q 'reason=registry_record_malformed' "$T/err" && ok "float budget: reason registry_record_malformed" || bad "float budget reason: $(cat "$T/err")"
rm -f "$LONGOPS_DIR/ops/bad2.json"
printf 'this is not json' >"$LONGOPS_DIR/ops/garbage.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "fail closed: a malformed record (not JSON) refuses (exit 1)" "$RC" 1
grep -q 'reason=registry_record_malformed' "$T/err" && ok "malformed record: reason registry_record_malformed" || bad "malformed record reason: $(cat "$T/err")"
rm -f "$LONGOPS_DIR/ops/garbage.json"
# review round 3 n1: jq prints nothing and exits 0 on empty input; the reason code of an empty record is the same as the one of any non-JSON record
: >"$LONGOPS_DIR/ops/empty.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "fail closed: an EMPTY record refuses (exit 1)" "$RC" 1
grep -q 'reason=registry_record_malformed' "$T/err" && ok "empty record: reason registry_record_malformed (not a classification failure)" || bad "empty record reason: $(cat "$T/err")"
printf ' \n' >"$LONGOPS_DIR/ops/empty.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
grep -q 'reason=registry_record_malformed' "$T/err" && ok "whitespace-only record: reason registry_record_malformed" || bad "whitespace-only record reason: $(cat "$T/err")"
# a valid object followed by garbage: jq prints the state of the first value and then fails; the failure (not the empty output) is what refuses it (a mutant that ignores jq's exit status survived the plain garbage case once the empty-output line existed)
printf '{"state":"running"} trailing garbage' >"$LONGOPS_DIR/ops/empty.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "fail closed: a malformed record (valid JSON followed by garbage) refuses (exit 1)" "$RC" 1
grep -q 'reason=registry_record_malformed' "$T/err" && ok "malformed record (valid then garbage): reason registry_record_malformed (jq's failure status is honoured)" || bad "malformed record (valid then garbage) reason: $(cat "$T/err")"
rm -f "$LONGOPS_DIR/ops/empty.json"
chmod 000 "$LONGOPS_DIR/ops/$OPX.json"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "fail closed: an unreadable record refuses (exit 1)" "$RC" 1
grep -q 'reason=registry_record_unreadable' "$T/err" && ok "unreadable record: reason registry_record_unreadable" || bad "unreadable record reason: $(cat "$T/err")"
chmod 644 "$LONGOPS_DIR/ops/$OPX.json"
chmod 000 "$LONGOPS_DIR/ops"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "fail closed: an ops dir unreadable (EACCES) refuses (exit 1), it is not read as used = 0" "$RC" 1
grep -q 'reason=registry_unreadable' "$T/err" && ok "ops dir unreadable: reason registry_unreadable" || bad "ops dir unreadable reason: $(cat "$T/err")"
chmod 755 "$LONGOPS_DIR/ops"
chmod 000 "$LONGOPS_DIR"
env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
check "fail closed: a parent directory unreadable (stat gives EACCES, not ENOENT) refuses (exit 1)" "$RC" 1
grep -q 'reason=registry_unreadable' "$T/err" && ok "parent directory unreadable: reason registry_unreadable" || bad "parent directory unreadable reason: $(cat "$T/err")"
chmod 755 "$LONGOPS_DIR"
# a stray LONGOPS_* variable no longer hides the live budgets: relocating the registry needs the declared test run
mi 32000000 30000000
for v in LONGOPS_DIR LONGOPS_REPO LONGOPS_AUDIT; do
  ( unset ENVELOPE_TEST_MODE ENVELOPE_MEMINFO ENVELOPE_NPROC ENVELOPE_ULIMIT_U LONGOPS_DIR LONGOPS_REPO LONGOPS_AUDIT; export "$v=$T/stray"; bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
  check "stray $v without ENVELOPE_TEST_MODE: refused (exit 1)" "$RC" 1
  grep -q 'reason=registry_override_outside_test_mode' "$T/err" && ok "stray $v: reason registry_override_outside_test_mode" || bad "stray $v reason: $(cat "$T/err")"
done
# a copy of envelope.sh outside a tree that has scripts/longops cannot read the registry: refused, not read as empty
mkdir -p "$T/nolib/scripts/containers"; cp "$SUT" "$T/nolib/scripts/containers/envelope.sh"
( ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000 bash "$T/nolib/scripts/containers/envelope.sh" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
check "registry library missing (copy outside the repository layout): refused (exit 1)" "$RC" 1
grep -q 'reason=registry_library_missing' "$T/err" && ok "registry library missing: reason registry_library_missing" || bad "registry library missing reason: $(cat "$T/err")"
# review round 2 m3 (reviewer mutant N1): a registry library that cannot classify a record (no lo_classify_op) refuses registry_classify_failed with a live op in the registry;
# the branch used to be untested and `*) ;;` there read the live 6e9 op as used = 0 (the fail-open the F3 fix forbids). Control: with the real library the same registry reads used 6e9.
mkdir -p "$T/nocls/scripts/containers"; cp "$SUT" "$T/nocls/scripts/containers/envelope.sh"; cp -r "$REPO/scripts/longops" "$T/nocls/scripts/longops"
sed -i 's/^lo_classify_op() {/lo_classify_op_renamed() {/' "$T/nocls/scripts/longops/lib.sh"
grep -q '^lo_classify_op_renamed() {' "$T/nocls/scripts/longops/lib.sh" && ok "classify fixture: the copy of the registry library lacks lo_classify_op (control needle: the rename is in the copy)" || bad "classify fixture: the rename did not apply"
( ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000 bash "$T/nocls/scripts/containers/envelope.sh" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
check "registry classify failed (a live op, a library that cannot classify): refused (exit 1), never used = 0" "$RC" 1
grep -q 'reason=registry_classify_failed' "$T/err" && ok "registry classify failed: reason registry_classify_failed" || bad "registry classify failed reason: $(cat "$T/err")"
( ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000 bash "$SUT" --toolchain go --format json ) >"$T/out" 2>"$T/err"; RC=$?
check "registry classify control: the real library on the same registry reads the live 6e9 op" "$RC/$(jq -r .used_mem_bytes "$T/out" 2>/dev/null)" "0/6000000000"
# review round 2 m7: the cost of the registry read must not grow with the terminal records (op records are never deleted; the read is paid inside the budget lock).
# A copy whose lo_classify_op counts its calls: five terminal records and one live op -> ONE classification, the live op still counted (control: used 2e9).
newreg; sleep 600 >/dev/null 2>&1 & SL6=$!; SLEEPERS+=("$SL6")
OPL="$(bash "$REPO/scripts/longops/register.sh" --purpose t118:cnt --owner t118 --pid "$SL6" --memory-bytes 2000000000 --cpus 2 --no-progress-s 600 2>/dev/null)"
for i in 1 2 3 4 5; do jq -c --arg id "term$i" '.op_id=$id | .state="complete"' "$LONGOPS_DIR/ops/$OPL.json" >"$LONGOPS_DIR/ops/term$i.json"; done
mkdir -p "$T/cnt/scripts/containers"; cp "$SUT" "$T/cnt/scripts/containers/envelope.sh"; cp -r "$REPO/scripts/longops" "$T/cnt/scripts/longops"
sed -i 's/^lo_classify_op() {/lo_classify_op_renamed() {/' "$T/cnt/scripts/longops/lib.sh"
printf '\nlo_classify_op() { echo call >>"$CNT_FILE"; lo_classify_op_renamed "$@"; }\n' >>"$T/cnt/scripts/longops/lib.sh"
: >"$T/cnt.calls"
( CNT_FILE="$T/cnt.calls" ENVELOPE_MEMINFO="$T/meminfo" ENVELOPE_NPROC=16 ENVELOPE_ULIMIT_U=100000 bash "$T/cnt/scripts/containers/envelope.sh" --toolchain go --format json ) >"$T/out" 2>"$T/err"; RC=$?
check "registry cost control: the live op is counted next to five terminal records (used 2e9)" "$RC/$(jq -r .used_mem_bytes "$T/out" 2>/dev/null)" "0/2000000000"
check "terminal records are not classified: six records (5 complete, 1 live) cost ONE classification call" "$(wc -l <"$T/cnt.calls" | tr -d ' ')" 1
newreg

# ---------- the CPU count is measured: OMP_NUM_THREADS / OMP_THREAD_LIMIT do not lift the 0.60 ceiling (review F7) ----------
REAL_NPROC="$(env -u OMP_NUM_THREADS -u OMP_THREAD_LIMIT nproc)"
( unset ENVELOPE_MEMINFO ENVELOPE_NPROC ENVELOPE_ULIMIT_U; OMP_NUM_THREADS=1000 OMP_THREAD_LIMIT=1000 bash "$SUT" --toolchain go --format json ) >"$T/out" 2>"$T/err"; RC=$?
check "OMP_NUM_THREADS=1000 on the real host: exit 0" "$RC" 0
check "OMP_NUM_THREADS=1000: nproc is the measured count, not 1000" "$(jq -r .nproc "$T/out" 2>/dev/null)" "$REAL_NPROC"
[ "$(jq -r .cpus "$T/out" 2>/dev/null)" -le $(( REAL_NPROC * 60 / 100 )) ] && ok "OMP_NUM_THREADS=1000: container cpus within 0.60 * nproc ($(jq -r .cpus "$T/out"))" || bad "OMP_NUM_THREADS lifted the cpu ceiling: cpus $(jq -r .cpus "$T/out") nproc $REAL_NPROC"
( unset ENVELOPE_MEMINFO ENVELOPE_NPROC ENVELOPE_ULIMIT_U; OMP_THREAD_LIMIT=2 bash "$SUT" --toolchain go --format json ) >"$T/out" 2>"$T/err"
check "OMP_THREAD_LIMIT=2: nproc is the measured count, not 2" "$(jq -r .nproc "$T/out" 2>/dev/null)" "$REAL_NPROC"

# ---------- single-hook and floor cases the reviewer's mutants R1-R3 exposed ----------
for hv in ENVELOPE_MEMINFO ENVELOPE_NPROC ENVELOPE_ULIMIT_U; do
  case "$hv" in ENVELOPE_MEMINFO) hval="$T/meminfo";; ENVELOPE_NPROC) hval=4;; *) hval=100;; esac
  ( unset ENVELOPE_TEST_MODE ENVELOPE_MEMINFO ENVELOPE_NPROC ENVELOPE_ULIMIT_U LONGOPS_DIR LONGOPS_REPO LONGOPS_AUDIT; export "$hv=$hval"; bash "$SUT" --toolchain go ) >"$T/out" 2>"$T/err"; RC=$?
  check "the $hv hook alone without ENVELOPE_TEST_MODE: refused (exit 1)" "$RC" 1
  grep -q 'reason=test_hook_outside_test_mode' "$T/err" && ok "the $hv hook alone: reason test_hook_outside_test_mode" || bad "the $hv hook alone reason: $(cat "$T/err")"
done
for ulv in 1 2 3; do
  UL=$ulv env_run 32000000 30000000 16 --toolchain go --profile "$T/profile.json"
  check "pids at ulimit $ulv: floor 1, never 0 (a 0 pids-limit is no limit in some runtimes)" "$(kv pids)" 1
done
UL=100000
env_run 32000000 20000000 16 --toolchain go --profile "$T/profile.json" --format json
env_run 32000000 20000000 16 --toolchain go --profile "$T/profile.json"
check "kv memory_bytes equals mem_budget_bytes when MemAvailable binds (15564800000, not the 0.60 ceiling)" "$(kv memory_bytes)" 15564800000
env_run 32000000 20000000 16 --toolchain go --profile "$T/profile.json" --format json
check "json memory_bytes equals mem_budget_bytes when MemAvailable binds (15564800000, not the 0.60 ceiling)" "$(jq -r .memory_bytes "$T/out" 2>/dev/null)" 15564800000
check "json mem_budget_bytes when MemAvailable binds" "$(jq -r .mem_budget_bytes "$T/out" 2>/dev/null)" 15564800000

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
( unset ENVELOPE_MEMINFO ENVELOPE_NPROC ENVELOPE_ULIMIT_U; bash "$SUT" --toolchain go --format json ) >"$T/out" 2>"$T/err"; RC=$?
if [ "$RC" = 0 ]; then
  ok "real host: exit 0"
  REAL_MT="$(awk '$1=="MemTotal:"{print $2*1024}' /proc/meminfo)"
  check "real host: ceiling_mem is 0.60*MemTotal from /proc/meminfo" "$(jq -r .ceiling_mem_bytes "$T/out")" $((REAL_MT * 60 / 100))
  check "real host: nproc equals nproc(1) with OMP_NUM_THREADS removed" "$(jq -r .nproc "$T/out")" "$(env -u OMP_NUM_THREADS -u OMP_THREAD_LIMIT nproc)"
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
# A mutant copy is placed in a WORKING tree layout (<d>/scripts/containers/envelope.sh next to a link to <repo>/scripts/longops): a copy dropped in /tmp could
# not source scripts/longops and read the registry as empty, so every mutant failed the same registry checks whatever its mutation (review M1).
place() { # <dir> <python-old> <python-new>: copy the SUT into <dir>/scripts/containers, optionally with one replacement; rc 1 when the pattern is not found exactly once
  local d=$1
  rm -rf "$d"; mkdir -p "$d/scripts/containers"; ln -s "$REPO/scripts/longops" "$d/scripts/longops"
  python3 -I - "$SUT" "$d/scripts/containers/envelope.sh" "${2-}" "${3-}" <<'PY'
import sys
s = open(sys.argv[1]).read()
old, new = sys.argv[3], sys.argv[4]
if old:
    if s.count(old) != 1: sys.exit(1)
    s = s.replace(old, new)
open(sys.argv[2], "w").write(s)
PY
}
# negative control: an UNMUTATED copy placed exactly like a mutant must pass the whole body; if it does not, the harness is blind and says so
place "$T/mut-control" "" ""
( ENVELOPE_SUT="$T/mut-control/scripts/containers/envelope.sh" ENVELOPE_TEST_MUTANT=1 QUIET=1 bash "${BASH_SOURCE[0]}" ) >"$T/mut-control.log" 2>&1; CRC=$?
if [ "$CRC" = 0 ]; then echo "CONTROL  an unmutated copy placed like a mutant passes the body ($(grep '^RESULT' "$T/mut-control.log"))" | tee -a "$MREC"
else echo "CONTROL FAILED: the unmutated copy fails the body ($(grep -c '^FAIL' "$T/mut-control.log") checks): the mutation harness is blind" | tee -a "$MREC"; EXIT=1; fi
mut_case() { # <id> <expected failing-check substring> <old> <new>: caught only by a check whose NAME contains the substring
  local id=$1 expect=$2 old=$3 new=$4
  TOTAL=$((TOTAL+1))
  place "$T/mut-$id" "$old" "$new" || { echo "INVALID $id: pattern not found exactly once" | tee -a "$MREC"; SURV=$((SURV+1)); return; }
  ( ENVELOPE_SUT="$T/mut-$id/scripts/containers/envelope.sh" ENVELOPE_TEST_MUTANT=1 QUIET=1 bash "${BASH_SOURCE[0]}" ) >"$T/mut-$id.log" 2>&1; local rc=$?
  if [ "$rc" -ne 0 ] && grep -q "^FAIL: .*$expect" "$T/mut-$id.log"; then CAUGHT=$((CAUGHT+1)); echo "CAUGHT   $id by a check naming '$expect' ($(grep -c '^FAIL' "$T/mut-$id.log") failing checks)" | tee -a "$MREC"
  elif [ "$rc" -ne 0 ]; then SURV=$((SURV+1)); echo "SURVIVED $id (failed, but no failing check names '$expect': $(grep '^FAIL' "$T/mut-$id.log" | head -2 | cut -c1-110 | tr '\n' '|'))" | tee -a "$MREC"
  else SURV=$((SURV+1)); echo "SURVIVED $id (test stayed green on the mutant)" | tee -a "$MREC"; fi
}
mut_case reserve-cpu-floor 'reserve_cpu' '|| RES_CPU=2' '|| RES_CPU=1'
mut_case reserve-mem-floor 'reserve_mem' '|| RES_MEM=4294967296' '|| RES_MEM=0'
mut_case reserve-mem-pct 'reserve_mem' 'RES_MEM=$(( MT * 15 / 100 ))' 'RES_MEM=$(( MT * 10 / 100 ))'
mut_case ceiling 'ceiling' 'CEIL=$(( MT * 60 / 100 ))' 'CEIL=$(( MT * 70 / 100 ))'
mut_case used-mem 'registry: mem_budget' 'A=$(( CEIL - USED_MEM ))' 'A=$(( CEIL ))'
mut_case min-term 'mem_budget' 'MEM_BUDGET=$(( A < B ? A : B ))' 'MEM_BUDGET=$(( A > B ? A : B ))'
mut_case refusal 'MemAvailable below reserve' '[ "$MEM_BUDGET" -ge 536870912 ]' '[ "$MEM_BUDGET" -ge -9999999999999 ]'
mut_case cpu-budget 'cpu_budget' 'CPU_BUDGET=$(( NPROC - RES_CPU - USED_CPU ))' 'CPU_BUDGET=$(( NPROC - USED_CPU ))'
mut_case jobs-min 'cpu bound: jobs' '[ "$J" -le "$CPU_BUDGET" ] || J=$CPU_BUDGET' 'true'
mut_case jobs-floor 'per_job above mem_budget: jobs' '[ "$JOBS" -ge 1 ] || JOBS=1' 'true'
mut_case unknown-jobs 'unknown' 'else JOBS=1; fi' 'else JOBS=$CPU_BUDGET; fi'
mut_case cpu-ceiling 'nproc 64: container cpus' '[ "$CPUS" -le "$CPU_CEIL" ] || CPUS=$CPU_CEIL' 'true'
mut_case pids 'pids = ulimit' '[ "$PIDS" -le "$PIDS_CEIL" ] || PIDS=$PIDS_CEIL' 'true'
mut_case ulimit-closed 'ulimit -u 0' '[ "$UL" -gt 0 ]; }; }' 'true; }; }'
mut_case test-hooks 'test hook without ENVELOPE_TEST_MODE' '[ "${ENVELOPE_TEST_MODE:-}" != 1 ]; then   # MUT:test-hooks' '[ "${ENVELOPE_TEST_MODE:-}" != 1 ] && false; then   # MUT:test-hooks'
mut_case used-classes-hung 'hung' 'advancing|hung)' 'advancing)'
mut_case used-classes-dead 'dead-owner' 'advancing|hung)' 'advancing|hung|dead_owner)'
mut_case memory-output 'kv memory_bytes' '"$JOBS" "$MEM_BUDGET" "$CPUS"' '"$JOBS" "$CEIL" "$CPUS"'
# the reviewer's mutants (none of them was in the original set)
mut_case R1-json-memory-bytes 'json memory_bytes' 'memory_bytes:$mb,' 'memory_bytes:$ce,'
mut_case R2-ulimit-hook-gate 'ENVELOPE_ULIMIT_U hook alone' ' || [ -n "${ENVELOPE_ULIMIT_U+x}" ]' ''
mut_case R3-pids-floor 'pids at ulimit' '[ "$PIDS_CEIL" -ge 1 ] || PIDS_CEIL=1' 'true'
mut_case R4-used-sum 'used_mem sums two live ops' 'um=$(( um + mb ))' 'um=$(( 0 + mb ))'
# the fail-closed registry (review F3) and the measured CPU count (review F7)
mut_case registry-hook 'stray LONGOPS' '[ "${ENVELOPE_TEST_MODE:-}" != 1 ]; then   # MUT:registry-hook' '[ "${ENVELOPE_TEST_MODE:-}" != 1 ] && false; then   # MUT:registry-hook'
mut_case N1-classify-failure-silent 'registry classify failed' '*) echo "ERR registry_classify_failed $f classified as '"'"'$cl'"'"'"; exit 0;;' '*) ;;'
mut_case registry-lib 'registry library missing' '. "$ROOT_DIR/scripts/longops/lib.sh" >/dev/null 2>&1 || {' 'true || {'
mut_case registry-dir 'ops dir unreadable' '{ [ "$st" = directory ] && [ -r "$ops" ] && [ -x "$ops" ]; } ||' 'true ||'
mut_case registry-stat 'parent directory unreadable' '*) echo "ERR registry_unreadable cannot stat $ops: $st"; exit 0;; esac' '*) echo "OK 0 0"; exit 0;; esac'
mut_case registry-malformed 'malformed record' '2>/dev/null <<<"$j")" || { echo "ERR registry_record_malformed $f is not a JSON record"; exit 0; }' '2>/dev/null <<<"$j")" || true'
mut_case R4-n1-empty-record-passes-jq 'empty record' '[ -n "$st" ] || { echo "ERR registry_record_malformed $f is not a JSON record (empty: jq prints nothing and exits 0 on empty input, review round 3 n1)"; exit 0; }   # MUT:registry-empty' ':'
mut_case terminal-skip-dropped 'terminal records are not classified' 'case "$st" in complete|failed|reaped|handoff|blocked-escape) continue;; esac' 'case "$st" in NEVER) continue;; esac'
mut_case registry-budget 'float budget' '{ valid_int "$mb" && valid_int "$cb"; } ||' 'true ||'
mut_case nproc-measured 'OMP_NUM_THREADS' '$(env -u OMP_NUM_THREADS -u OMP_THREAD_LIMIT nproc 2>/dev/null)' '$(nproc 2>/dev/null)'
echo "MUTATION RESULT caught=$CAUGHT survived=$SURV total=$TOTAL" | tee -a "$MREC"
[ "$SURV" = 0 ] || EXIT=1
exit "$EXIT"
