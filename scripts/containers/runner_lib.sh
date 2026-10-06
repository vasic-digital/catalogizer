#!/usr/bin/env bash
# runner_lib.sh - T120. The shared body of the build and test wrappers run_go.sh, run_node.sh, run_docs.sh, run_scan.sh, run_playwright.sh and
# run_testutil.sh. Sourced by a wrapper that has set the RUNNER_* variables below, never run on its own. Every container starts THROUGH
# scripts/containers/run_pinned.sh (RUNP, reused, never modified); this library adds what a wrapper owes on top of it (docs/16 sections 6.3, 8, 13):
#   1. the image must be allowed for the wrapper, present in the lock and pinned (sha256 digest) - else REFUSED, never a fallback to another image;
#   2. the image id the lock names must be the image podman has (`podman image inspect` digest equals the lock digest);
#   3. the anti-mess sweep (scripts/anti-mess/sweep.sh, runtime plane AM-P1,AM-P2,AM-P3, stage cadence) runs first and a drift or a blind
#      detector refuses the run (11.4.233 C, E: a gated transition never proceeds on un-reconciled drift);
#   4. the dynamic envelope (scripts/containers/envelope.sh, docs/16 8.2) fixes memory, cpus and pids, handed to RUNP as RUNP_MEMORY / RUNP_CPUS /
#      RUNP_PIDS; a caller may ask for LESS (--memory, --cpus); the nominal 2% / 1 cpu allowance above the live reading is capped at the head-room
#      under the 0.60 ceiling of the live operations AND at MemAvailable minus the reserve (so the sum of the live budgets never exceeds the 0.60
#      ceilings of 12.6 and the reserve is never entered);
#   5. the envelope is read, the limits checked and the run REGISTERED as a long operation (scripts/longops/register.sh: single owner per purpose,
#      11.4.232 A, B) under ONE flock (`<registry>/.envelope-budget.lock`), so concurrent starts cannot both receive the same head-room; the op is
#      registered BEFORE the first container starts, heartbeats while it runs (11.4.232 C: the progress offset is the byte count of the run's output
#      plus the bytes of /out, elapsed time is reported so --wall-s is enforced: the run is terminated by exact identity, exit 124) and ends in a
#      terminal state with the exit code as its verdict; the heartbeat loop ends with its wrapper (a killed wrapper's container client is reaped by
#      identity and the op released `failed` / `wrapper_died`);
#   6. a toolchain record `toolchain.json` (docs/16 6.3) is written into the out directory from a probe container run first: image id, lock and
#      inspected digests, the tool version, a write to the out dir and to the cache dir that must succeed and a write to the READ-ONLY source mount
#      that must FAIL (the control needle of 11.4.201: a probe that cannot detect a failure is itself detected, refused as probe_blind).
# Wrapper variables:  RUNNER_NAME  RUNNER_IMAGES (space separated, the first is the default; more than one enables --image)  RUNNER_TOOLCHAIN
#   RUNNER_PROBE_MODE (sh | version)  RUNNER_PROBE_VERSION (the version command)  RUNNER_CMD_PREFIX (words put before the user command)  RUNNER_BLOCKED_NOTE
#   optional function runner_probe_for_image <IMG-ID> printing the version command for one image (run_scan).
# Usage of a wrapper:
#   run_X.sh [--image IMG-ID] [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY]
#            [--op-id ID] [--no-progress-s N] [--wall-s N] -- <command word>...
# Exits:  the container's exit code on a run; 124 --wall-s exceeded; 130 TERM/INT/HUP (installed before anything is registered); 1 REFUSED (`<wrapper>: REFUSED reason=<code>` on stderr; note a container may also exit 1 - the reason line
#   tells them apart); 2 usage.
# Environment: RUNP_LOCK (lock file, as RUNP), RUNNER_HEARTBEAT_S (seconds between heartbeats, 1..60, default 5), RUNNER_LOG_DIR (default $PWD/.audit/runner-logs),
#   LONGOPS_* (registry, scripts/longops/lib.sh; relocating it needs ENVELOPE_TEST_MODE=1, see envelope.sh). Inherited RUNP_* controls the wrapper does
#   not own (RUNP_PRINT_ARGV, RUNP_TEST_MODE, RUNP_MEMINFO, RUNP_ULIMIT_U, RUNP_MEMORY, RUNP_CPUS, RUNP_PIDS ...) are UNSET with a note on stderr; only
#   RUNP_LOCK and RUNP_USER pass through. Test hooks (replace a real component, so honoured ONLY with RUNNER_TEST_MODE=1, else REFUSED
#   test_hook_outside_test_mode): RUNNER_RUNP (run_pinned.sh replacement), RUNNER_SWEEP (sweep replacement).
# The standard output and standard error of the run are collected in two files and replayed to the caller's stdout and stderr when the run ends
# (so the byte counts can be the heartbeat's progress proof); the run's own files (/out) are live.

RL_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$RL_HERE/../.." && pwd)"
LOCK="${RUNP_LOCK:-$ROOT_DIR/build/containers/images.lock.yaml}"
OP_ID=""; OP_REGISTERED=0; TR_OUT=""; SIGNALLED=0; CH=""; CH_ST=""; HBP=""; STOP=""; LOCKFD=""; LOGDIR=""

rl_refuse() { echo "$RUNNER_NAME: REFUSED reason=$1 ${2:-}" >&2; exit 1; }
rl_usage()  { echo "$RUNNER_NAME: usage: $1" >&2; echo "$RUNNER_NAME: $RUNNER_NAME.sh [--image IMG-ID] [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY] [--op-id ID] [--no-progress-s N] [--wall-s N] -- <cmd>..." >&2; exit 2; }
rl_valid_int() { case "$1" in ''|*[!0-9]*) return 1;; 0) return 0;; 0*) return 1;; esac; [ "${#1}" -le 18 ]; }
# rl_release_op <rc> [verdict]: end the registered op (state complete on rc 0, failed otherwise); the exit code is the verdict, never the state's source
rl_release_op() {
  [ "$OP_REGISTERED" = 1 ] || return 0
  local rc=$1 st=failed v="${2:-rc=$1}"
  [ "$rc" != 0 ] || st=complete
  bash "$ROOT_DIR/scripts/longops/release.sh" --op-id "$OP_ID" --state "$st" --verdict "$v" --evidence-path "${TR_OUT:-}" >/dev/null 2>&1
  OP_REGISTERED=0
}
# process identity helpers (a pid is only ever signalled while /proc/<pid>/stat start time still equals the one recorded at spawn: never a bare pid, never <= 1)
rl_pstart() { [ -r "/proc/$1/stat" ] && sed 's/^.*) //' "/proc/$1/stat" 2>/dev/null | cut -d' ' -f20; }
rl_ms() { local t=${EPOCHREALTIME/./}; echo $(( t / 1000 )); }
rl_signal_child() { # rl_signal_child <pid> <start> <sig>: signal exactly that process if it is still the same one
  [[ "$1" =~ ^[0-9]+$ && "$1" -gt 1 && -n "$2" ]] || return 1
  [ "$(rl_pstart "$1")" = "$2" ] || return 1
  kill -s "$3" "$1" 2>/dev/null
}
rl_on_signal() { SIGNALLED=1; rl_signal_child "$CH" "$CH_ST" TERM; }
rl_stop_hb() { if [ -n "$HBP" ]; then : >"$STOP" 2>/dev/null; wait "$HBP" 2>/dev/null; HBP=""; fi; [ -z "$STOP" ] || rm -f -- "$STOP" "$STOP.wall"; }
# rl_on_exit: any exit path ends the heartbeat loop and never leaves a registered op with a live owner that is gone
rl_on_exit() { rl_stop_hb; rl_release_op 1 wrapper_exited; }
# rl_checkpoint: a TERM/INT/HUP seen since the last checkpoint ends the run here (the op is released `failed` / `interrupted` when it was registered)
rl_checkpoint() { [ "$SIGNALLED" = 1 ] || return 0; rl_stop_hb; rl_release_op 1 interrupted; exit 130; }
# rl_bg_wait: wait for the background child CH; the status is in RL_RC; a signal makes wait return at once, the handler has already signalled CH
rl_bg_wait() { wait "$CH"; RL_RC=$?; if [ "$SIGNALLED" = 1 ]; then wait "$CH" 2>/dev/null; RL_RC=$?; fi; CH=""; CH_ST=""; }
# rl_progress: the heartbeat's progress proof: bytes of the run's stdout and stderr plus the bytes under /out (a quiet run that writes only /out is progressing)
rl_progress() {
  local a b c
  a="$(stat -c %s "$LOG_OUT" 2>/dev/null || echo 0)"; b="$(stat -c %s "$LOG_ERR" 2>/dev/null || echo 0)"
  c="$(find "$OUT" -type f -printf '%s\n' 2>/dev/null | awk '{s += $1} END {print s + 0}')"
  echo $(( a + b + c ))
}
# rl_hb_loop: the heartbeat loop (a background subshell). Reports progress and elapsed ms; ends the run at --wall-s (TERM to the container client,
# by identity); ends itself when the wrapper is gone (identity of the wrapper recorded at start), reaping the container client and releasing the op.
rl_hb_loop() {
  local wp=$1 wst=$2 t0 el tick=0 per=$(( HB_S * 5 )) walled=0 rc
  t0="$(rl_ms)"
  while [ ! -e "$STOP" ]; do
    if [ "$(rl_pstart "$wp")" != "$wst" ]; then   # MUT:parent-liveness
      rl_signal_child "$CH" "$CH_ST" TERM
      for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25; do [ "$(rl_pstart "$CH")" = "$CH_ST" ] || break; sleep 0.2; done
      bash "$ROOT_DIR/scripts/longops/release.sh" --op-id "$OP_ID" --state failed --verdict wrapper_died --evidence-path "${TR_OUT:-}" >/dev/null 2>&1
      return 0
    fi
    el=$(( $(rl_ms) - t0 ))
    if [ "$WALL_S" -gt 0 ] && [ "$walled" = 0 ] && [ "$el" -gt $(( WALL_S * 1000 )) ]; then   # MUT:wall-enforce
      walled=1; : >"$STOP.wall"; rl_signal_child "$CH" "$CH_ST" TERM
    fi
    if [ $(( tick % per )) = 0 ]; then
      bash "$ROOT_DIR/scripts/longops/heartbeat.sh" --op-id "$OP_ID" --progress-offset "$(rl_progress)" --elapsed-ms "$el" >/dev/null 2>&1; rc=$?   # MUT:heartbeat
      [ "$rc" != 4 ] || return 0   # the op is terminal: nothing left to report
    fi
    tick=$(( tick + 1 )); sleep 0.2
  done
}
# rl_fail_op <reason> [detail]: the op ended failed with the refusal reason as its verdict, then the refusal
rl_fail_op() { rl_release_op 1 "$1"; rl_refuse "$1" "${2:-}"; }

runner_main() {
  local IMG="" ARG_MEM="" ARG_CPUS="" OUT="" RW="" NET="" NEED="" PURPOSE="" NP_S=0 WALL_S=0 multi=0
  local -a USER_CMD=()
  set -- "$@"
  case "$RUNNER_IMAGES" in *" "*) multi=1;; esac
  while [ $# -gt 0 ]; do
    case "$1" in
      --) shift; break;;
      --image)  [ "$multi" = 1 ] || rl_usage "--image is not an option of $RUNNER_NAME (its image is $RUNNER_IMAGES)"; [ $# -ge 2 ] || rl_usage "--image requires a value"; IMG="$2"; shift 2;;
      --memory) [ $# -ge 2 ] || rl_usage "--memory requires a value"; ARG_MEM="$2"; shift 2;;
      --cpus)   [ $# -ge 2 ] || rl_usage "--cpus requires a value"; ARG_CPUS="$2"; shift 2;;
      --out)    [ $# -ge 2 ] || rl_usage "--out requires a value"; OUT="$2"; shift 2;;
      --rw)     [ $# -ge 2 ] || rl_usage "--rw requires a value"; RW="$2"; shift 2;;
      --network=none) NET=none; shift;;
      --need)   [ $# -ge 2 ] || rl_usage "--need requires a value"; NEED="$2"; shift 2;;
      --purpose) [ $# -ge 2 ] || rl_usage "--purpose requires a value"; PURPOSE="$2"; shift 2;;
      --op-id)  [ $# -ge 2 ] || rl_usage "--op-id requires a value"; OP_ID="$2"; shift 2;;
      --no-progress-s) [ $# -ge 2 ] || rl_usage "--no-progress-s requires a value"; NP_S="$2"; shift 2;;
      --wall-s) [ $# -ge 2 ] || rl_usage "--wall-s requires a value"; WALL_S="$2"; shift 2;;
      -*) rl_usage "unknown option '$1'";;
      *) rl_usage "unexpected argument '$1' before --";;
    esac
  done
  [ $# -ge 1 ] || rl_usage "command missing after --"
  USER_CMD=("$@")
  [ -z "$ARG_MEM" ] || rl_valid_int "$ARG_MEM" || rl_usage "--memory '$ARG_MEM' is not a base-10 byte count"
  [ -z "$ARG_CPUS" ] || { rl_valid_int "$ARG_CPUS" && [ "$ARG_CPUS" -ge 1 ]; } || rl_usage "--cpus '$ARG_CPUS' is not a positive integer"
  [ -z "$ARG_MEM" ] || [ "$ARG_MEM" -ge 1 ] || rl_usage "--memory must be at least 1"
  [ -z "$NEED" ] || rl_valid_int "$NEED" || rl_usage "--need '$NEED' is not a base-10 byte count"
  rl_valid_int "$NP_S" || rl_usage "--no-progress-s must be a non-negative integer"
  rl_valid_int "$WALL_S" || rl_usage "--wall-s must be a non-negative integer"
  local first="${RUNNER_IMAGES%% *}"; [ -n "$IMG" ] || IMG="$first"
  case " $RUNNER_IMAGES " in *" $IMG "*) ;; *) rl_refuse image_not_allowed "$IMG is not an image of $RUNNER_NAME (allowed: $RUNNER_IMAGES)";; esac   # MUT:image-allowed
  if [ -n "$OP_ID" ]; then case "$OP_ID" in ''|*[!A-Za-z0-9._-]*|.*) rl_usage "op id '$OP_ID' must match ^[A-Za-z0-9_-][A-Za-z0-9._-]*\$";; esac; fi
  [ -z "$PURPOSE" ] || case "$PURPOSE" in *[!A-Za-z0-9._@+:=-]*|[.-]*) rl_usage "purpose '$PURPOSE' has a character outside A-Z a-z 0-9 . _ @ + : = -";; esac
  for _d in python3 jq; do command -v "$_d" >/dev/null 2>&1 || rl_refuse dependency_missing "$_d is required"; done

  # ---- inherited RUNP_* controls the wrapper does not own are scrubbed (F4): RUNP_PRINT_ARGV=1 would make run_pinned.sh print its argv and exit 0 with
  # no container, which the version-probe mode read as a version line (`podman`). RUNP_LOCK and RUNP_USER are documented inputs and pass through.
  local _v
  while IFS= read -r _v; do
    case "$_v" in RUNP_LOCK|RUNP_USER) ;; RUNP_*) echo "$RUNNER_NAME: note: ignored inherited $_v (the wrapper owns every RUNP_* control but RUNP_LOCK and RUNP_USER)" >&2; unset "$_v";; esac   # MUT:scrub-runp
  done < <(compgen -e)

  # ---- test hooks: only a declared test run may replace a real component ----
  local RUNP="$RL_HERE/run_pinned.sh" SWEEP="$ROOT_DIR/scripts/anti-mess/sweep.sh"
  if { [ -n "${RUNNER_RUNP+x}" ] || [ -n "${RUNNER_SWEEP+x}" ]; } && [ "${RUNNER_TEST_MODE:-}" != 1 ]; then   # MUT:test-hooks
    rl_refuse test_hook_outside_test_mode "RUNNER_RUNP / RUNNER_SWEEP are test hooks and need RUNNER_TEST_MODE=1"
  fi
  [ -z "${RUNNER_RUNP:-}" ] || RUNP="$RUNNER_RUNP"
  [ -z "${RUNNER_SWEEP:-}" ] || SWEEP="$RUNNER_SWEEP"

  # ---- the lock entry: present and pinned ----
  [ -r "$LOCK" ] || rl_refuse lock_unreadable "$LOCK"
  local ENTRY REF="" DIGEST="" PDIGEST="" k v
  ENTRY="$(python3 -I - "$LOCK" "$IMG" <<'PY'
import sys
try:
    import yaml
    d = yaml.safe_load(open(sys.argv[1]))
except Exception:
    print("ERR\tlock_unreadable"); sys.exit(0)
imgs = d.get("images") if isinstance(d, dict) else None
if not isinstance(imgs, list):
    print("ERR\tlock_unreadable"); sys.exit(0)
m = [i for i in imgs if isinstance(i, dict) and i.get("id") == sys.argv[2]]
if not m: print("ERR\timage_not_in_lock"); sys.exit(0)
if len(m) > 1: print("ERR\tlock_duplicate_id"); sys.exit(0)
e = m[0]
for k in ("reference", "digest", "platform_digest"):
    v = e.get(k)
    print("%s\t%s" % (k, "" if v is None else str(v).replace("\n", " ").replace("\t", " ")))
PY
)" || rl_refuse lock_unreadable "python3 failed"
  case "$ENTRY" in ERR*) k="$(printf '%s' "$ENTRY" | head -1 | cut -f2)"
    [ "$k" != image_not_in_lock ] || rl_refuse image_not_in_lock "BLOCKED: $IMG is not in the lock $LOCK; ${RUNNER_BLOCKED_NOTE:-the image has to be built and pinned first}"   # MUT:not-in-lock
    rl_refuse "$k" "id=$IMG lock=$LOCK";; esac
  while IFS=$'\t' read -r k v; do case "$k" in reference) REF="$v";; digest) DIGEST="$v";; platform_digest) PDIGEST="$v";; esac; done <<<"$ENTRY"
  local _hex="${DIGEST#sha256:}"
  { [ -n "$REF" ] && case "$REF" in -*|*[[:space:]]*|*@*) false;; esac; } || rl_refuse image_unpinned "id=$IMG has no usable reference in the lock"
  { [ "${DIGEST%%:*}" = sha256 ] && [ "${#_hex}" -eq 64 ] && [ -z "${_hex//[0-9a-f]/}" ]; } || rl_refuse image_unpinned "id=$IMG has no sha256 digest in the lock"   # MUT:unpinned
  local IMAGE_REF="$REF@$DIGEST"
  # the image id the lock names is the image podman has (the digest podman reports equals the lock digest or the lock platform digest)
  local INSPECTED
  INSPECTED="$(podman image inspect --format '{{.Digest}}' -- "$IMAGE_REF" 2>/dev/null)" || rl_refuse image_not_present_locally "$IMAGE_REF (pull it through the pin procedure, never here)"
  if [ -n "$PDIGEST" ] && [ "$INSPECTED" = "$PDIGEST" ]; then INSPECTED="$DIGEST"; fi
  [ "$INSPECTED" = "$DIGEST" ] || rl_refuse image_digest_mismatch "id=$IMG lock digest $DIGEST, podman reports '$INSPECTED'"   # MUT:digest

  # ---- the anti-mess sweep gate (11.4.233) ----
  rl_sweep_gate() {
    local SW_RC
    [ -f "$SWEEP" ] || rl_refuse anti_mess_sweep_missing "$SWEEP"   # MUT:sweep-missing
    bash "$SWEEP" --stage cadence --only AM-P1,AM-P2,AM-P3 >&2; SW_RC=$?
    if [ "$SW_RC" = 10 ]; then rl_refuse anti_mess_drift "the anti-mess sweep reports drift (see its table above); reconcile it before starting a container"; fi
    [ "$SW_RC" = 0 ] || rl_refuse anti_mess_blind "the anti-mess sweep exited $SW_RC (a refusal or a blind detector, never read as clean)"
  }
  rl_sweep_gate   # MUT:sweep-gate

  # ---- identity of this run (before the lock: the op id and the log paths are needed to check for a clash inside it) ----
  [ -n "$OP_ID" ] || OP_ID="$RUNNER_NAME-$(date -u +%Y%m%dT%H%M%SZ)-$$"
  [ -n "$OUT" ] || OUT="$PWD/.audit/out/$OP_ID"
  if [ -z "$PURPOSE" ]; then PURPOSE="container:$RUNNER_NAME:$(printf '%s\0' "$PWD" "$IMG" "${USER_CMD[@]}" | sha256sum | cut -c1-12)"; fi
  local HB_S="${RUNNER_HEARTBEAT_S:-5}" LOG_OUT LOG_ERR
  { rl_valid_int "$HB_S" && [ "$HB_S" -ge 1 ] && [ "$HB_S" -le 60 ]; } || rl_usage "RUNNER_HEARTBEAT_S must be 1..60"
  LOGDIR="${RUNNER_LOG_DIR:-$PWD/.audit/runner-logs}"
  mkdir -p -- "$LOGDIR" 2>/dev/null || rl_refuse log_dir_uncreatable "$LOGDIR"
  LOG_OUT="$LOGDIR/$OP_ID.out"; LOG_ERR="$LOGDIR/$OP_ID.err"; STOP="$LOGDIR/$OP_ID.stop"
  local LDIR
  [ -r "$ROOT_DIR/scripts/longops/lib.sh" ] || rl_refuse registry_library_missing "$ROOT_DIR/scripts/longops/lib.sh"
  LDIR="$( . "$ROOT_DIR/scripts/longops/lib.sh" >/dev/null 2>&1 && printf '%s' "$LD" )"
  [ -n "$LDIR" ] || rl_refuse registry_library_missing "scripts/longops/lib.sh gave no registry directory"
  mkdir -p -- "$LDIR" 2>/dev/null || rl_refuse registry_unusable "cannot create $LDIR"

  # ---- signals: installed BEFORE anything is registered (F6); the handler only records, the checkpoints act ----
  trap rl_on_signal TERM INT HUP
  trap rl_on_exit EXIT

  # ---- ONE lock spans the envelope read, the limit checks and the registration (F1): two starts can never both be handed the same head-room ----
  if ! { exec {LOCKFD}>"$LDIR/.envelope-budget.lock"; } 2>/dev/null; then rl_refuse budget_lock_unavailable "$LDIR/.envelope-budget.lock"; fi
  flock -w 120 "$LOCKFD" || rl_refuse budget_lock_timeout "no budget lock in 120 s: $LDIR/.envelope-budget.lock"   # MUT:budget-lock
  rl_checkpoint
  # an op id already registered belongs to another run: refused BEFORE any log file is opened, so that run's logs are never truncated (F5)
  [ ! -e "$LDIR/ops/$OP_ID.json" ] || rl_refuse op_exists "op id $OP_ID is already registered; the other run's logs and record are untouched"   # MUT:op-exists

  local ENVJ ENV_MEM ENV_CPUS ENV_PIDS LIM_MEM LIM_CPUS LIM_PIDS
  ENVJ="$(bash "$RL_HERE/envelope.sh" --toolchain "$RUNNER_TOOLCHAIN" --format json 2>&1)" || rl_refuse envelope_refused "$ENVJ"
  ENV_MEM="$(jq -r .memory_bytes <<<"$ENVJ")"; ENV_CPUS="$(jq -r .cpus <<<"$ENVJ")"; ENV_PIDS="$(jq -r .pids <<<"$ENVJ")"
  local ENV_CEIL ENV_NPROC ENV_USED ENV_MA ENV_RES ENV_CPUBUD ALLOW_MEM ALLOW_CPUS CPU_CEIL
  ENV_CEIL="$(jq -r .ceiling_mem_bytes <<<"$ENVJ")"; ENV_NPROC="$(jq -r .nproc <<<"$ENVJ")"; ENV_USED="$(jq -r .used_mem_bytes <<<"$ENVJ")"
  ENV_MA="$(jq -r .mem_available_bytes <<<"$ENVJ")"; ENV_RES="$(jq -r .reserve_mem_bytes <<<"$ENVJ")"; ENV_CPUBUD="$(jq -r .cpu_budget <<<"$ENVJ")"
  { rl_valid_int "$ENV_MEM" && rl_valid_int "$ENV_CPUS" && rl_valid_int "$ENV_PIDS" && rl_valid_int "$ENV_CEIL" && rl_valid_int "$ENV_NPROC" \
      && rl_valid_int "$ENV_USED" && rl_valid_int "$ENV_MA" && rl_valid_int "$ENV_RES"; } || rl_refuse envelope_refused "unparsable envelope: $ENVJ"
  case "$ENV_CPUBUD" in -[0-9]*|[0-9]*) ;; *) rl_refuse envelope_refused "unparsable cpu_budget in the envelope: $ENVJ";; esac
  # the allowance above the live reading (2% of memory, 1 cpu) never goes above what is left under the 0.60 ceiling once the live operations are
  # counted, never into the reserve (MemAvailable - reserve), never above 0.60 * nproc: every cap is an input of the same reading
  ALLOW_MEM=$(( ENV_MEM + ENV_MEM / 50 ))
  [ "$ALLOW_MEM" -le $(( ENV_CEIL - ENV_USED )) ] || ALLOW_MEM=$(( ENV_CEIL - ENV_USED ))   # MUT:slack-ceiling
  [ "$ALLOW_MEM" -le $(( ENV_MA - ENV_RES )) ] || ALLOW_MEM=$(( ENV_MA - ENV_RES ))   # MUT:slack-reserve
  CPU_CEIL=$(( ENV_NPROC * 60 / 100 )); [ "$CPU_CEIL" -ge 1 ] || CPU_CEIL=1
  ALLOW_CPUS=$(( ENV_CPUS + 1 ))
  [ "$ALLOW_CPUS" -le "$CPU_CEIL" ] || ALLOW_CPUS="$CPU_CEIL"
  [ "$ALLOW_CPUS" -le $(( ENV_CPUBUD < 1 ? 1 : ENV_CPUBUD )) ] || ALLOW_CPUS=$(( ENV_CPUBUD < 1 ? 1 : ENV_CPUBUD ))   # MUT:cpu-slack-budget
  LIM_MEM="${ARG_MEM:-$ENV_MEM}"; LIM_CPUS="${ARG_CPUS:-$ENV_CPUS}"; LIM_PIDS="$ENV_PIDS"
  [ "$LIM_MEM" -le "$ALLOW_MEM" ] || rl_refuse limit_exceeds_envelope "--memory $LIM_MEM is above the envelope memory $ENV_MEM (allowed up to $ALLOW_MEM)"   # MUT:mem-envelope
  [ "$LIM_CPUS" -le "$ALLOW_CPUS" ] || rl_refuse limit_exceeds_envelope "--cpus $LIM_CPUS is above the envelope cpus $ENV_CPUS (allowed up to $ALLOW_CPUS)"   # MUT:cpus-envelope

  : >"$LOG_OUT"; : >"$LOG_ERR"

  # ---- register the long operation BEFORE the first container starts ----
  local REG_RC REG_ERR
  REG_ERR="$(bash "$ROOT_DIR/scripts/longops/register.sh" --purpose "$PURPOSE" --owner "$RUNNER_NAME" --op-id "$OP_ID" --pid "$$" --container-label "$OP_ID" \
    --write-path "$OUT" --no-progress-s "$NP_S" --wall-s "$WALL_S" --memory-bytes "$LIM_MEM" --cpus "$LIM_CPUS" --log "$LOG_OUT" 2>&1 >/dev/null)"; REG_RC=$?   # MUT:register
  if [ "$REG_RC" = 3 ]; then
    case "$REG_ERR" in *op_exists*) rl_refuse op_exists "$REG_ERR";; esac
    rl_refuse purpose_conflict "$REG_ERR"   # MUT:purpose-conflict
  fi
  [ "$REG_RC" = 0 ] || rl_refuse register_failed "register.sh exit $REG_RC: $REG_ERR"
  OP_REGISTERED=1; TR_OUT="$OUT/toolchain.json"
  flock -u "$LOCKFD"; exec {LOCKFD}>&-; LOCKFD=""   # the head-room is now a registered budget: the next start sees it
  rl_checkpoint

  # ---- run_pinned.sh with the envelope limits (the one place a container is started) ----
  rl_runp() { # rl_runp <command word>...: the common flags, then the command; ONLY ever called in a background subshell: it execs, so the subshell's pid IS run_pinned's, then podman's
    local -a A=()
    [ -z "$RW" ] || A+=(--rw "$RW")
    [ -z "$NEED" ] || A+=(--need "$NEED")
    [ "$NET" != none ] || A+=(--network=none)
    RUNP_MEMORY="$LIM_MEM" RUNP_CPUS="$LIM_CPUS" RUNP_PIDS="$LIM_PIDS" exec bash "$RUNP" "${A[@]}" --out "$OUT" --op-id "$OP_ID" "$IMG" -- "$@"
  }
  rl_spawn() { # rl_spawn <stdout-file> <stderr-file> <command word>...: background child CH, identity recorded; a signal that arrived meanwhile is honoured at once
    local so=$1 se=$2; shift 2
    ( rl_runp "$@" ) >"$so" 2>"$se" </dev/null &
    CH=$!; CH_ST="$(rl_pstart "$CH")"
    [ "$SIGNALLED" = 0 ] || rl_signal_child "$CH" "$CH_ST" TERM
  }

  # ---- the toolchain probe (docs/16 6.3) ----
  local PROBE_OUT PROBE_RC P_VERSION P_VFAIL="" P_OUT P_CACHE P_SRC PV_CMD="$RUNNER_PROBE_VERSION" PROBE_FILE="$LOGDIR/$OP_ID.probe"
  if declare -F runner_probe_for_image >/dev/null 2>&1; then PV_CMD="$(runner_probe_for_image "$IMG")"; [ -n "$PV_CMD" ] || rl_fail_op probe_not_defined "no version command is defined for $IMG"; fi
  local -a PVW; read -r -a PVW <<<"$PV_CMD"
  if [ "$RUNNER_PROBE_MODE" = version ]; then
    rl_spawn "$PROBE_FILE" "$LOG_ERR" "${PVW[@]}"; rl_bg_wait; PROBE_RC=$RL_RC; rl_checkpoint
    PROBE_OUT="$(cat "$PROBE_FILE" 2>/dev/null)"
    P_VERSION="$(printf '%s\n' "$PROBE_OUT" | head -n 1)"; P_OUT="n/a:no_shell"; P_CACHE="n/a:no_shell"; P_SRC="n/a:no_shell"
  else
    # F11: a failed version command is a failure, never a version (the error message of a missing tool would pass the blind-probe check as a "version")
    rl_spawn "$PROBE_FILE" "$LOG_ERR" sh -c 'if v="$('"$PV_CMD"' 2>&1)"; then echo "version=$(printf "%s\n" "$v" | head -n 1)"; else echo "version_failed=$(printf "%s\n" "$v" | head -n 1)"; fi
if touch /out/.cpa-probe-w 2>/dev/null; then rm -f /out/.cpa-probe-w; echo out_writable=yes; else echo out_writable=no; fi
mkdir -p "$XDG_CACHE_HOME" 2>/dev/null
if touch "$XDG_CACHE_HOME/.cpa-probe-c" 2>/dev/null; then echo cache_writable=yes; else echo cache_writable=no; fi
if touch /src/.cpa-probe-needle 2>/dev/null; then rm -f /src/.cpa-probe-needle; echo src_readonly=no; else echo src_readonly=yes; fi'
    rl_bg_wait; PROBE_RC=$RL_RC; rl_checkpoint
    PROBE_OUT="$(cat "$PROBE_FILE" 2>/dev/null)"
    P_VERSION="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^version=//p' | head -n 1)"
    P_VFAIL="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^version_failed=//p' | head -n 1)"
    P_OUT="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^out_writable=//p' | head -n 1)"
    P_CACHE="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^cache_writable=//p' | head -n 1)"
    P_SRC="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^src_readonly=//p' | head -n 1)"
  fi
  rm -f -- "$PROBE_FILE"
  [ "$PROBE_RC" = 0 ] || rl_fail_op probe_failed "the toolchain probe container exited $PROBE_RC: $(tr '\n' ' ' <"$LOG_ERR" | cut -c1-400)"   # MUT:probe-rc
  if printf '%s\n' "$PROBE_OUT" | grep -q '^version_failed='; then rl_fail_op version_probe_failed "the version command '$PV_CMD' failed in the container: $P_VFAIL"; fi   # MUT:version-rc
  [ -n "$P_VERSION" ] || rl_fail_op probe_blind "the probe printed no version line: the instrument saw nothing"
  if [ "$RUNNER_PROBE_MODE" != version ]; then
    [ "$P_SRC" = yes ] || rl_fail_op probe_blind "the control needle failed: a write to the read-only source mount did not fail (src_readonly='$P_SRC')"   # MUT:needle
    [ "$P_OUT" = yes ] || rl_fail_op out_not_writable "/out is not writable by the mapped user (out_writable='$P_OUT')"   # MUT:out-writable
    [ "$P_CACHE" = yes ] || rl_fail_op cache_not_writable "the cache directory is not writable by the mapped user (cache_writable='$P_CACHE')"   # MUT:cache-writable
  fi
  [ -d "$OUT" ] || rl_fail_op out_not_writable "$OUT does not exist after the probe"
  jq -nc --arg w "$RUNNER_NAME" --arg img "$IMG" --arg ref "$REF" --arg ld "$DIGEST" --arg id "$INSPECTED" --arg ver "$P_VERSION" --arg op "$OP_ID" \
     --arg po "$P_OUT" --arg pc "$P_CACHE" --arg ps "$P_SRC" --arg u "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg purpose "$PURPOSE" --argjson env "$ENVJ" \
     '{schema:"toolchain-record/1", wrapper:$w, image_id:$img, reference:$ref, lock_digest:$ld, inspected_digest:$id, digest_match:($ld == $id), version:$ver,
       probes:{out_writable:$po, cache_writable:$pc, src_readonly:$ps}, op_id:$op, purpose_key:$purpose, envelope:$env, utc:$u}' >"$OUT/.toolchain.json.tmp" \
    && mv -f "$OUT/.toolchain.json.tmp" "$TR_OUT" || rl_fail_op record_unwritable "cannot write $TR_OUT"

  # ---- the run: heartbeats while it is alive, the exit code is the verdict ----
  local RUN_RC
  local -a PFX=() CMD=()
  read -r -a PFX <<<"${RUNNER_CMD_PREFIX:-}"   # MUT:prefix
  CMD=("${PFX[@]}" "${USER_CMD[@]}")
  rm -f -- "$STOP" "$STOP.wall"
  rl_spawn "$LOG_OUT" "$LOG_ERR" "${CMD[@]}"
  ( rl_hb_loop "$$" "$(rl_pstart "$$")" ) >/dev/null 2>&1 &
  HBP=$!
  rl_bg_wait; RUN_RC=$RL_RC
  local WALLED=0; [ ! -e "$STOP.wall" ] || WALLED=1
  rl_stop_hb
  cat "$LOG_OUT"; cat "$LOG_ERR" >&2
  if [ "$SIGNALLED" = 1 ]; then rl_release_op 1 interrupted; exit 130; fi   # MUT:signal-release
  if [ "$WALLED" = 1 ]; then rl_release_op 1 wall_clock_exceeded; echo "$RUNNER_NAME: wall clock of ${WALL_S}s exceeded: the run was terminated (TERM to its container client)" >&2; exit 124; fi
  rl_release_op "$RUN_RC"
  exit "$RUN_RC"   # MUT:exit
}
