#!/usr/bin/env bash
# envelope.sh - T119. The dynamic resource envelope of docs/16 section 8.2 (12.6, 12.11, 12.12). Read-only: it reads the host and prints numbers.
#
# Usage:  envelope.sh [--toolchain NAME] [--profile FILE] [--format kv|json]
#   --toolchain NAME   key of the per-toolchain measurement in the profile (go, node, docs, scan, playwright, testutil ...); [a-z0-9_-]+
#   --profile FILE     measured profile (default build/containers/profile.json); {"toolchains": {"<name>": {"per_job_mem_bytes": <positive integer>}}}
#   --format kv|json   one `key=value` per line (default) or one JSON object (schema envelope/1)
# Formulas (docs/16 8.2, integer arithmetic, floors as written there):
#   reserve_cpu = max(2, ceil(0.125 * nproc))        reserve_mem = max(4 GiB, 0.15 * MemTotal)       ceiling_mem = floor(0.60 * MemTotal)
#   mem_budget  = min(ceiling_mem - used_mem, MemAvailable - reserve_mem)
#   cpu_budget  = nproc - reserve_cpu - used_cpu      jobs = max(1, min(cpu_budget, floor(mem_budget / per_job_mem)))
#   used_mem / used_cpu = the budgets of the registered long operations that are alive (scripts/longops registry: states advancing or hung,
#   never terminal, never a dead owner: a dead owner holds nothing, it is the anti-mess sweep's finding, not a budget).
#   per_job_mem is measured, never guessed (11.4.6): without a valid measured value the status is UNKNOWN:<reason> and jobs = 1.
# Container limits derived for the wrappers: memory_bytes = mem_budget; cpus = clamp(cpu_budget, 1, floor(0.60 * nproc)) (the ceiling run_pinned.sh
#   enforces); pids = min(2048, min(8192, ulimit -u / 2)) (12.12).
# Output keys: schema toolchain nproc mem_total_bytes mem_available_bytes reserve_cpu reserve_mem_bytes ceiling_mem_bytes used_mem_bytes used_cpu
#   mem_budget_bytes cpu_budget per_job_mem_bytes per_job_status jobs memory_bytes cpus pids
# Test hooks (replace a real reading, so honoured ONLY with ENVELOPE_TEST_MODE=1, else REFUSED test_hook_outside_test_mode):
#   ENVELOPE_MEMINFO (file in /proc/meminfo format), ENVELOPE_NPROC, ENVELOPE_ULIMIT_U. The long-op registry is relocated with LONGOPS_REPO / LONGOPS_DIR / LONGOPS_AUDIT of scripts/longops/lib.sh, which ALSO needs
#   ENVELOPE_TEST_MODE=1 (else REFUSED registry_override_outside_test_mode: a stray variable can no longer hide the live budgets). The CPU count is measured
#   with OMP_NUM_THREADS and OMP_THREAD_LIMIT removed (GNU nproc would honour them). Registry accounting fails closed: an unreadable ops directory, a record
#   that is not JSON or has a non-integer budget, or a missing scripts/longops refuses with a named reason, never "used = 0" (an absent ops directory is the
#   honest empty registry).
# Refusals exit 1 with `envelope: REFUSED reason=<code>` (meminfo_unreadable, cpu_budget_unavailable, pids_budget_unavailable, memory_budget_unavailable,
#   dependency_missing, test_hook_outside_test_mode, registry_override_outside_test_mode, registry_library_missing, registry_unreadable,
#   registry_record_unreadable, registry_record_malformed, registry_classify_failed); usage errors exit 2. Never contacts the network, never writes.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$HERE/../.." && pwd)"
refuse() { echo "envelope: REFUSED reason=$1 ${2:-}" >&2; exit 1; }
usage()  { echo "envelope: usage: $1" >&2; echo "envelope: envelope.sh [--toolchain NAME] [--profile FILE] [--format kv|json]" >&2; exit 2; }
valid_int() { case "$1" in ''|*[!0-9]*) return 1;; 0) return 0;; 0*) return 1;; esac; [ "${#1}" -le 18 ]; }

TC=""; PROFILE="$ROOT_DIR/build/containers/profile.json"; FMT=kv
while [ $# -gt 0 ]; do
  case "$1" in
    --toolchain) [ $# -ge 2 ] || usage "--toolchain requires a value"; TC="$2"; shift 2;;
    --profile)   [ $# -ge 2 ] || usage "--profile requires a value"; PROFILE="$2"; shift 2;;
    --format)    [ $# -ge 2 ] || usage "--format requires a value"; FMT="$2"; shift 2;;
    *) usage "unknown argument '$1'";;
  esac
done
case "$FMT" in kv|json) ;; *) usage "--format must be kv or json";; esac
if [ -n "$TC" ]; then case "$TC" in *[!a-z0-9_-]*) usage "--toolchain '$TC' must match [a-z0-9_-]+";; esac; fi
command -v jq >/dev/null 2>&1 || refuse dependency_missing "jq is required"
# test hooks replace a real reading: only a declared test run may use them, so a stray exported variable can never lift a ceiling
if { [ -n "${ENVELOPE_MEMINFO+x}" ] || [ -n "${ENVELOPE_NPROC+x}" ] || [ -n "${ENVELOPE_ULIMIT_U+x}" ]; } && [ "${ENVELOPE_TEST_MODE:-}" != 1 ]; then   # MUT:test-hooks
  refuse test_hook_outside_test_mode "ENVELOPE_MEMINFO / ENVELOPE_NPROC / ENVELOPE_ULIMIT_U are test hooks and need ENVELOPE_TEST_MODE=1"
fi

# ---- inputs ----
mem_kb() { awk -v k="$1" '$1==k":" {print $2}' "${ENVELOPE_MEMINFO:-/proc/meminfo}" 2>/dev/null; }
MT_KB="$(mem_kb MemTotal)"; MA_KB="$(mem_kb MemAvailable)"
{ valid_int "$MT_KB" && valid_int "$MA_KB" && [ "$MT_KB" -gt 0 ]; } || refuse meminfo_unreadable "MemTotal/MemAvailable"
MT=$(( MT_KB * 1024 )); MA=$(( MA_KB * 1024 ))
# the CPU count is MEASURED: GNU nproc honours OMP_NUM_THREADS / OMP_THREAD_LIMIT (OMP_NUM_THREADS=1000 prints 1000), which would lift the 0.60 CPU ceiling
NPROC="${ENVELOPE_NPROC-$(env -u OMP_NUM_THREADS -u OMP_THREAD_LIMIT nproc 2>/dev/null)}"   # MUT:nproc-measured
{ valid_int "$NPROC" && [ "$NPROC" -ge 1 ]; } || refuse cpu_budget_unavailable "nproc reads '$NPROC'"
UL="${ENVELOPE_ULIMIT_U-$(ulimit -u 2>/dev/null)}"
{ [ "$UL" = unlimited ] || { valid_int "$UL" && [ "$UL" -gt 0 ]; }; } || refuse pids_budget_unavailable "ulimit -u reads '$UL': neither 'unlimited' nor a positive integer"   # MUT:ulimit-closed

# used by registered long operations (F3: FAIL CLOSED - an input that cannot be read is a refusal with a named reason, never "used = 0").
# Relocating the registry (LONGOPS_REPO / LONGOPS_DIR / LONGOPS_AUDIT) replaces a real reading, so it needs the declared test run like the other hooks.
if { [ -n "${LONGOPS_REPO+x}" ] || [ -n "${LONGOPS_DIR+x}" ] || [ -n "${LONGOPS_AUDIT+x}" ]; } && [ "${ENVELOPE_TEST_MODE:-}" != 1 ]; then   # MUT:registry-hook
  refuse registry_override_outside_test_mode "LONGOPS_REPO / LONGOPS_DIR / LONGOPS_AUDIT relocate the registry and need ENVELOPE_TEST_MODE=1"
fi
# a subshell sources the registry library, the parent's variables stay untouched; it answers one line: `OK <mem> <cpu>` or `ERR <reason> <detail>`
USED="$( ( LONGOPS_REPO="${LONGOPS_REPO:-$ROOT_DIR}"; . "$ROOT_DIR/scripts/longops/lib.sh" >/dev/null 2>&1 || { echo "ERR registry_library_missing could not source $ROOT_DIR/scripts/longops/lib.sh (a copy of envelope.sh outside the repository layout cannot read the registry)"; exit 0; }   # MUT:registry-lib

    ops="$LD/ops"
    if ! st="$(stat -c %F -- "$ops" 2>&1)"; then
      case "$st" in *"No such file"*) echo "OK 0 0"; exit 0;; *) echo "ERR registry_unreadable cannot stat $ops: $st"; exit 0;; esac   # MUT:registry-stat
    fi
    { [ "$st" = directory ] && [ -r "$ops" ] && [ -x "$ops" ]; } || { echo "ERR registry_unreadable $ops is not a readable directory ($st)"; exit 0; }   # MUT:registry-dir
    um=0; uc=0
    for f in "$ops"/*.json; do
      [ -e "$f" ] || continue
      j="$(cat -- "$f" 2>&1)" || { echo "ERR registry_record_unreadable $f: $j"; exit 0; }
      # ONE jq call validates the record as JSON (an empty record is caught by the next line: jq accepts empty input) and reads its state: terminal records (the bulk of a long-lived registry: op records are never deleted) hold no
      # budget and are skipped here without the classification and budget calls (review round 2 m7: the cost per record was paid inside the budget lock)
      st="$(jq -r 'if type == "object" and (.state | type == "string") then .state else "-" end' 2>/dev/null <<<"$j")" || { echo "ERR registry_record_malformed $f is not a JSON record"; exit 0; }   # MUT:registry-malformed
      [ -n "$st" ] || { echo "ERR registry_record_malformed $f is not a JSON record (empty: jq prints nothing and exits 0 on empty input, review round 3 n1)"; exit 0; }   # MUT:registry-empty
      case "$st" in complete|failed|reaped|handoff|blocked-escape) continue;; esac   # MUT:terminal-skip
      cl="$(lo_classify_op "$j" 2>/dev/null | head -1)"
      case "$cl" in
        advancing|hung)   # MUT:used-classes
          mb="$(jq -r '(.budget.memory_bytes // 0) | if type == "number" and . == floor and . >= 0 then tostring else "INVALID" end' <<<"$j" 2>/dev/null)"
          cb="$(jq -r '(.budget.cpus // 0) | if type == "number" and . == floor and . >= 0 then tostring else "INVALID" end' <<<"$j" 2>/dev/null)"
          { valid_int "$mb" && valid_int "$cb"; } || { echo "ERR registry_record_malformed $f has a budget that is not a non-negative integer (memory '$mb', cpus '$cb')"; exit 0; }   # MUT:registry-budget
          um=$(( um + mb )); uc=$(( uc + cb )) ;;
        terminal|dead_owner) ;;
        *) echo "ERR registry_classify_failed $f classified as '$cl'"; exit 0;;   # MUT:registry-classify
      esac
    done
    echo "OK $um $uc" ) 2>/dev/null )"
case "$USED" in
  "OK "*) read -r _ USED_MEM USED_CPU <<<"$USED"
          { valid_int "${USED_MEM:-}" && valid_int "${USED_CPU:-}"; } || refuse registry_unreadable "the registry accounting answered '$USED'";;
  "ERR "*) read -r _ _reason _detail <<<"$USED"; refuse "${_reason:-registry_unreadable}" "${_detail:-}";;
  *) refuse registry_unreadable "the registry accounting gave no answer";;
esac

# per_job_mem: measured value or UNKNOWN:<reason>
PER_JOB=""; PJ_STATUS="UNKNOWN:no_toolchain"
if [ -n "$TC" ]; then
  if [ ! -r "$PROFILE" ]; then PJ_STATUS="UNKNOWN:profile_absent"
  elif ! jq -e . "$PROFILE" >/dev/null 2>&1; then PJ_STATUS="UNKNOWN:profile_unreadable"
  else
    _raw="$(jq -r --arg t "$TC" 'if (.toolchains | type) == "object" and (.toolchains | has($t)) then (.toolchains[$t].per_job_mem_bytes | if type == "number" and . == floor and . > 0 then tostring else "INVALID" end) else "ABSENT" end' "$PROFILE" 2>/dev/null)"
    case "$_raw" in
      ABSENT) PJ_STATUS="UNKNOWN:toolchain_unmeasured";;
      INVALID|'') PJ_STATUS="UNKNOWN:per_job_invalid";;
      *) if valid_int "$_raw" && [ "$_raw" -gt 0 ]; then PER_JOB="$_raw"; PJ_STATUS=measured; else PJ_STATUS="UNKNOWN:per_job_invalid"; fi;;
    esac
  fi
fi

# ---- formulas (docs/16 8.2) ----
RES_CPU=$(( (NPROC + 7) / 8 )); [ "$RES_CPU" -ge 2 ] || RES_CPU=2   # MUT:reserve-cpu
RES_MEM=$(( MT * 15 / 100 )); [ "$RES_MEM" -ge 4294967296 ] || RES_MEM=4294967296   # MUT:reserve-mem
CEIL=$(( MT * 60 / 100 ))   # MUT:ceiling
A=$(( CEIL - USED_MEM )); B=$(( MA - RES_MEM ))   # MUT:used-mem
MEM_BUDGET=$(( A < B ? A : B ))   # MUT:min-term
[ "$MEM_BUDGET" -ge 536870912 ] || refuse memory_budget_unavailable "budget $MEM_BUDGET bytes is below 512 MiB (MemAvailable ${MA_KB} kB, reserve $RES_MEM, ceiling $CEIL, used by registered operations $USED_MEM)"   # MUT:refusal
CPU_BUDGET=$(( NPROC - RES_CPU - USED_CPU ))   # MUT:cpu-budget
if [ -n "$PER_JOB" ]; then
  J=$(( MEM_BUDGET / PER_JOB )); [ "$J" -le "$CPU_BUDGET" ] || J=$CPU_BUDGET   # MUT:jobs-min
  JOBS=$J
else JOBS=1; fi
[ "$JOBS" -ge 1 ] || JOBS=1   # MUT:jobs-floor
CPUS=$CPU_BUDGET; CPU_CEIL=$(( NPROC * 60 / 100 )); [ "$CPU_CEIL" -ge 1 ] || CPU_CEIL=1
[ "$CPUS" -le "$CPU_CEIL" ] || CPUS=$CPU_CEIL   # MUT:cpu-ceiling
[ "$CPUS" -ge 1 ] || CPUS=1
PIDS=2048; PIDS_CEIL=8192
if [ "$UL" != unlimited ] && [ $(( UL / 2 )) -lt "$PIDS_CEIL" ]; then PIDS_CEIL=$(( UL / 2 )); fi
[ "$PIDS_CEIL" -ge 1 ] || PIDS_CEIL=1
[ "$PIDS" -le "$PIDS_CEIL" ] || PIDS=$PIDS_CEIL   # MUT:pids

# ---- output ----
if [ "$FMT" = json ]; then
  jq -nc --arg tc "$TC" --arg pjs "$PJ_STATUS" --arg pj "$PER_JOB" \
    --argjson nproc "$NPROC" --argjson mt "$MT" --argjson ma "$MA" --argjson rc "$RES_CPU" --argjson rm "$RES_MEM" --argjson ce "$CEIL" \
    --argjson um "$USED_MEM" --argjson uc "$USED_CPU" --argjson mb "$MEM_BUDGET" --argjson cb "$CPU_BUDGET" --argjson jobs "$JOBS" \
    --argjson cpus "$CPUS" --argjson pids "$PIDS" \
    '{schema:"envelope/1", toolchain:$tc, nproc:$nproc, mem_total_bytes:$mt, mem_available_bytes:$ma, reserve_cpu:$rc, reserve_mem_bytes:$rm,
      ceiling_mem_bytes:$ce, used_mem_bytes:$um, used_cpu:$uc, mem_budget_bytes:$mb, cpu_budget:$cb,
      per_job_mem_bytes:(if $pj == "" then null else ($pj|tonumber) end), per_job_status:$pjs, jobs:$jobs, memory_bytes:$mb, cpus:$cpus, pids:$pids}'
else
  printf 'schema=envelope/1\ntoolchain=%s\nnproc=%s\nmem_total_bytes=%s\nmem_available_bytes=%s\nreserve_cpu=%s\nreserve_mem_bytes=%s\nceiling_mem_bytes=%s\n' \
    "$TC" "$NPROC" "$MT" "$MA" "$RES_CPU" "$RES_MEM" "$CEIL"
  printf 'used_mem_bytes=%s\nused_cpu=%s\nmem_budget_bytes=%s\ncpu_budget=%s\nper_job_mem_bytes=%s\nper_job_status=%s\njobs=%s\nmemory_bytes=%s\ncpus=%s\npids=%s\n' \
    "$USED_MEM" "$USED_CPU" "$MEM_BUDGET" "$CPU_BUDGET" "${PER_JOB:-UNKNOWN}" "$PJ_STATUS" "$JOBS" "$MEM_BUDGET" "$CPUS" "$PIDS"
fi
