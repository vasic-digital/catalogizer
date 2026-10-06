#!/usr/bin/env bash
# runner_lib.sh - T120. The shared body of the build and test wrappers run_go.sh, run_node.sh, run_docs.sh, run_scan.sh, run_playwright.sh and
# run_testutil.sh. Sourced by a wrapper that has set the RUNNER_* variables below, never run on its own. Every container starts THROUGH
# scripts/containers/run_pinned.sh (RUNP, reused, never modified); this library adds what a wrapper owes on top of it (docs/16 sections 6.3, 8, 13):
#   1. the image must be allowed for the wrapper, present in the lock and pinned (sha256 digest) - else REFUSED, never a fallback to another image;
#   2. the image id the lock names must be the image podman has (`podman image inspect` digest equals the lock digest);
#   3. the anti-mess sweep (scripts/anti-mess/sweep.sh, runtime plane AM-P1,AM-P2,AM-P3, stage cadence) runs first and a drift or a blind
#      detector refuses the run (11.4.233 C, E: a gated transition never proceeds on un-reconciled drift);
#   4. the dynamic envelope (scripts/containers/envelope.sh, docs/16 8.2) fixes memory, cpus and pids, handed to RUNP as RUNP_MEMORY / RUNP_CPUS /
#      RUNP_PIDS; a caller may ask for LESS (--memory, --cpus) and for at most 2% more memory / 1 more cpu than this live reading (the envelope
#      moves with MemAvailable and the registry and the caller read it moments earlier), never above the 0.60 ceilings of 12.6;
#   5. the run is registered as a long operation BEFORE the first container starts (scripts/longops/register.sh: single owner per purpose,
#      11.4.232 A, B), heartbeats while it runs (11.4.232 C: the progress offset is the byte count of the run's output) and ends in a terminal
#      state with the exit code as its verdict;
#   6. a toolchain record `toolchain.json` (docs/16 6.3) is written into the out directory from a probe container run first: image id, lock and
#      inspected digests, the tool version, a write to the out dir and to the cache dir that must succeed and a write to the READ-ONLY source mount
#      that must FAIL (the control needle of 11.4.201: a probe that cannot detect a failure is itself detected, refused as probe_blind).
# Wrapper variables:  RUNNER_NAME  RUNNER_IMAGES (space separated, the first is the default; more than one enables --image)  RUNNER_TOOLCHAIN
#   RUNNER_PROBE_MODE (sh | version)  RUNNER_PROBE_VERSION (the version command)  RUNNER_CMD_PREFIX (words put before the user command)  RUNNER_BLOCKED_NOTE
#   optional function runner_probe_for_image <IMG-ID> printing the version command for one image (run_scan).
# Usage of a wrapper:
#   run_X.sh [--image IMG-ID] [--memory BYTES] [--cpus N] [--out DIR] [--rw docs|.audit/scratch] [--network=none] [--need BYTES] [--purpose KEY]
#            [--op-id ID] [--no-progress-s N] [--wall-s N] -- <command word>...
# Exits:  the container's exit code on a run; 1 REFUSED (`<wrapper>: REFUSED reason=<code>` on stderr; note a container may also exit 1 - the reason line
#   tells them apart); 2 usage.
# Environment: RUNP_LOCK (lock file, as RUNP), RUNNER_HEARTBEAT_S (seconds between heartbeats, 1..60, default 5), RUNNER_LOG_DIR (default $PWD/.audit/runner-logs),
#   LONGOPS_* (registry, scripts/longops/lib.sh). Test hooks (replace a real component, so honoured ONLY with RUNNER_TEST_MODE=1, else REFUSED
#   test_hook_outside_test_mode): RUNNER_RUNP (run_pinned.sh replacement), RUNNER_SWEEP (sweep replacement).
# The standard output and standard error of the run are collected in two files and replayed to the caller's stdout and stderr when the run ends
# (so the byte counts can be the heartbeat's progress proof); the run's own files (/out) are live.

RL_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$RL_HERE/../.." && pwd)"
LOCK="${RUNP_LOCK:-$ROOT_DIR/build/containers/images.lock.yaml}"
OP_ID=""; OP_REGISTERED=0; TR_OUT=""

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

  # ---- the envelope and the limits ----
  local ENVJ ENV_MEM ENV_CPUS ENV_PIDS LIM_MEM LIM_CPUS LIM_PIDS
  ENVJ="$(bash "$RL_HERE/envelope.sh" --toolchain "$RUNNER_TOOLCHAIN" --format json 2>&1)" || rl_refuse envelope_refused "$ENVJ"
  ENV_MEM="$(jq -r .memory_bytes <<<"$ENVJ")"; ENV_CPUS="$(jq -r .cpus <<<"$ENVJ")"; ENV_PIDS="$(jq -r .pids <<<"$ENVJ")"
  local ENV_CEIL ENV_NPROC ALLOW_MEM ALLOW_CPUS CPU_CEIL
  ENV_CEIL="$(jq -r .ceiling_mem_bytes <<<"$ENVJ")"; ENV_NPROC="$(jq -r .nproc <<<"$ENVJ")"
  { rl_valid_int "$ENV_MEM" && rl_valid_int "$ENV_CPUS" && rl_valid_int "$ENV_PIDS" && rl_valid_int "$ENV_CEIL" && rl_valid_int "$ENV_NPROC"; } || rl_refuse envelope_refused "unparsable envelope: $ENVJ"
  # an explicit limit may be LOWER than the envelope (always) or exceed this live reading by at most 2% of memory / 1 cpu: the envelope follows
  # MemAvailable and the registry, and a caller (TIC) read it moments before this wrapper does; the 0.60 ceilings of 12.6 are never exceeded
  ALLOW_MEM=$(( ENV_MEM + ENV_MEM / 50 )); [ "$ALLOW_MEM" -le "$ENV_CEIL" ] || ALLOW_MEM="$ENV_CEIL"   # MUT:slack-ceiling
  CPU_CEIL=$(( ENV_NPROC * 60 / 100 )); [ "$CPU_CEIL" -ge 1 ] || CPU_CEIL=1
  ALLOW_CPUS=$(( ENV_CPUS + 1 )); [ "$ALLOW_CPUS" -le "$CPU_CEIL" ] || ALLOW_CPUS="$CPU_CEIL"
  LIM_MEM="${ARG_MEM:-$ENV_MEM}"; LIM_CPUS="${ARG_CPUS:-$ENV_CPUS}"; LIM_PIDS="$ENV_PIDS"
  [ "$LIM_MEM" -le "$ALLOW_MEM" ] || rl_refuse limit_exceeds_envelope "--memory $LIM_MEM is above the envelope memory $ENV_MEM (allowed up to $ALLOW_MEM)"   # MUT:mem-envelope
  [ "$LIM_CPUS" -le "$ALLOW_CPUS" ] || rl_refuse limit_exceeds_envelope "--cpus $LIM_CPUS is above the envelope cpus $ENV_CPUS (allowed up to $ALLOW_CPUS)"   # MUT:cpus-envelope

  # ---- identity of this run ----
  [ -n "$OP_ID" ] || OP_ID="$RUNNER_NAME-$(date -u +%Y%m%dT%H%M%SZ)-$$"
  [ -n "$OUT" ] || OUT="$PWD/.audit/out/$OP_ID"
  if [ -z "$PURPOSE" ]; then PURPOSE="container:$RUNNER_NAME:$(printf '%s\0' "$PWD" "$IMG" "${USER_CMD[@]}" | sha256sum | cut -c1-12)"; fi
  local LOGDIR="${RUNNER_LOG_DIR:-$PWD/.audit/runner-logs}" LOG_OUT LOG_ERR HB_S="${RUNNER_HEARTBEAT_S:-5}"
  { rl_valid_int "$HB_S" && [ "$HB_S" -ge 1 ] && [ "$HB_S" -le 60 ]; } || rl_usage "RUNNER_HEARTBEAT_S must be 1..60"
  mkdir -p -- "$LOGDIR" 2>/dev/null || rl_refuse log_dir_uncreatable "$LOGDIR"
  LOG_OUT="$LOGDIR/$OP_ID.out"; LOG_ERR="$LOGDIR/$OP_ID.err"; : >"$LOG_OUT"; : >"$LOG_ERR"

  # ---- register the long operation BEFORE the first container starts ----
  local REG_RC REG_ERR
  REG_ERR="$(bash "$ROOT_DIR/scripts/longops/register.sh" --purpose "$PURPOSE" --owner "$RUNNER_NAME" --op-id "$OP_ID" --pid "$$" --container-label "$OP_ID" \
    --write-path "$OUT" --no-progress-s "$NP_S" --wall-s "$WALL_S" --memory-bytes "$LIM_MEM" --cpus "$LIM_CPUS" --log "$LOG_OUT" 2>&1 >/dev/null)"; REG_RC=$?   # MUT:register
  [ "$REG_RC" != 3 ] || rl_refuse purpose_conflict "$REG_ERR"   # MUT:purpose-conflict
  [ "$REG_RC" = 0 ] || rl_refuse register_failed "register.sh exit $REG_RC: $REG_ERR"
  OP_REGISTERED=1; TR_OUT="$OUT/toolchain.json"

  # ---- run_pinned.sh with the envelope limits (the one place a container is started) ----
  rl_runp() { # rl_runp <command word>...: the common flags, then the command
    local -a A=()
    [ -z "$RW" ] || A+=(--rw "$RW")
    [ -z "$NEED" ] || A+=(--need "$NEED")
    [ "$NET" != none ] || A+=(--network=none)
    RUNP_MEMORY="$LIM_MEM" RUNP_CPUS="$LIM_CPUS" RUNP_PIDS="$LIM_PIDS" bash "$RUNP" "${A[@]}" --out "$OUT" --op-id "$OP_ID" "$IMG" -- "$@"
  }

  # ---- the toolchain probe (docs/16 6.3) ----
  local PROBE_OUT PROBE_RC P_VERSION P_OUT P_CACHE P_SRC PV_CMD="$RUNNER_PROBE_VERSION"
  if declare -F runner_probe_for_image >/dev/null 2>&1; then PV_CMD="$(runner_probe_for_image "$IMG")"; [ -n "$PV_CMD" ] || rl_fail_op probe_not_defined "no version command is defined for $IMG"; fi
  local -a PVW; read -r -a PVW <<<"$PV_CMD"
  if [ "$RUNNER_PROBE_MODE" = version ]; then
    PROBE_OUT="$(rl_runp "${PVW[@]}" 2>"$LOG_ERR" </dev/null)"; PROBE_RC=$?
    P_VERSION="$(printf '%s\n' "$PROBE_OUT" | head -n 1)"; P_OUT="n/a:no_shell"; P_CACHE="n/a:no_shell"; P_SRC="n/a:no_shell"
  else
    PROBE_OUT="$(rl_runp sh -c 'v="$('"$PV_CMD"' 2>&1 | head -n 1)"; echo "version=$v"
if touch /out/.cpa-probe-w 2>/dev/null; then rm -f /out/.cpa-probe-w; echo out_writable=yes; else echo out_writable=no; fi
mkdir -p "$XDG_CACHE_HOME" 2>/dev/null
if touch "$XDG_CACHE_HOME/.cpa-probe-c" 2>/dev/null; then echo cache_writable=yes; else echo cache_writable=no; fi
if touch /src/.cpa-probe-needle 2>/dev/null; then rm -f /src/.cpa-probe-needle; echo src_readonly=no; else echo src_readonly=yes; fi' 2>"$LOG_ERR" </dev/null)"; PROBE_RC=$?
    P_VERSION="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^version=//p' | head -n 1)"
    P_OUT="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^out_writable=//p' | head -n 1)"
    P_CACHE="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^cache_writable=//p' | head -n 1)"
    P_SRC="$(printf '%s\n' "$PROBE_OUT" | sed -n 's/^src_readonly=//p' | head -n 1)"
  fi
  [ "$PROBE_RC" = 0 ] || rl_fail_op probe_failed "the toolchain probe container exited $PROBE_RC: $(tr '\n' ' ' <"$LOG_ERR" | cut -c1-400)"   # MUT:probe-rc
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
  local STOP="$LOGDIR/$OP_ID.stop" HBP="" CH="" RUN_RC SIGNALLED=0
  local -a PFX=() CMD=()
  read -r -a PFX <<<"${RUNNER_CMD_PREFIX:-}"   # MUT:prefix
  CMD=("${PFX[@]}" "${USER_CMD[@]}")
  rm -f -- "$STOP"
  ( while [ ! -e "$STOP" ]; do
      bash "$ROOT_DIR/scripts/longops/heartbeat.sh" --op-id "$OP_ID" --progress-offset "$(( $(stat -c %s "$LOG_OUT" 2>/dev/null || echo 0) + $(stat -c %s "$LOG_ERR" 2>/dev/null || echo 0) ))" >/dev/null 2>&1   # MUT:heartbeat
      for _ in $(seq $(( HB_S * 5 ))); do [ -e "$STOP" ] && break; sleep 0.2; done
    done ) >/dev/null 2>&1 &
  HBP=$!
  rl_on_signal() { SIGNALLED=1; if [[ "$CH" =~ ^[0-9]+$ && "$CH" -gt 1 ]]; then kill "$CH" 2>/dev/null; fi; }
  trap rl_on_signal TERM INT HUP
  rl_runp "${CMD[@]}" >"$LOG_OUT" 2>"$LOG_ERR" </dev/null &
  CH=$!
  wait "$CH"; RUN_RC=$?
  [ "$SIGNALLED" = 0 ] || wait "$CH" 2>/dev/null
  trap - TERM INT HUP
  : >"$STOP"; wait "$HBP" 2>/dev/null; rm -f -- "$STOP"
  cat "$LOG_OUT"; cat "$LOG_ERR" >&2
  if [ "$SIGNALLED" = 1 ]; then rl_release_op 1 interrupted; exit 130; fi
  rl_release_op "$RUN_RC"
  exit "$RUN_RC"   # MUT:exit
}
