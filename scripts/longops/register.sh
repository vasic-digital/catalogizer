#!/usr/bin/env bash
# register.sh - register a long operation BEFORE it starts and claim its purpose atomically (11.4.232 A, B; docs/16 13.1; T089).
#
# Usage   register.sh --purpose <key> --owner <name> [--op-id <id>] [--pid <pid>] [--container-label <l>] [--container-id <c>]
#                     [--write-path <path>]... [--no-progress-s <n>] [--wall-s <n>] [--memory-bytes <n>] [--cpus <n>] [--log <file>] [--no-claim] [--grammar build]
# Effect  writes ops/<op_id>.json (state `registered`, every declared write path recorded), claims claims/<purpose> by `mkdir`
#         (a live holder gives exit 3 purpose_conflict, a stale one exit 4, nothing started twice), appends an event. Prints the op id.
# Pid     defaults to the caller's parent (the wrapper that outlives this script); identity is its /proc start time and cmdline.
# Exits   0 registered; 2 usage; 3 purpose_conflict; 4 stale claim; 20 refusal (tmpfs state).
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
purpose=""; owner=""; opid=""; grammar=""; pid=$PPID; clab=""; cid=""; wp=(); np=0; wall=0; mem=0; cpus=0; log=""; claim=1
while [ $# -gt 0 ]; do
  case "$1" in
    --purpose) purpose=${2:-}; shift 2 ;; --owner) owner=${2:-}; shift 2 ;; --op-id) opid=${2:-}; shift 2 ;; --pid) pid=${2:-}; shift 2 ;;
    --container-label) clab=${2:-}; shift 2 ;; --container-id) cid=${2:-}; shift 2 ;; --write-path) wp+=("${2:-}"); shift 2 ;;
    --no-progress-s) np=${2:-}; shift 2 ;; --wall-s) wall=${2:-}; shift 2 ;; --memory-bytes) mem=${2:-}; shift 2 ;; --cpus) cpus=${2:-}; shift 2 ;;
    --log) log=${2:-}; shift 2 ;; --no-claim) claim=0; shift ;; --grammar) grammar=${2:-}; shift 2 ;;
    *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;;
  esac
done
[ -n "$purpose" ] && [ -n "$owner" ] || lo_die usage_error "--purpose and --owner are required"
lo_safe_name "$purpose" || lo_die usage_error "unsafe purpose"
# T089a: the purpose key of a dispatched build has ONE grammar (T005b): build:<component>:<lane-or-target>:<source-snapshot-digest>:<argv-digest>:<variant>[:<iteration>]
# (digests: 64 lowercase hex; variant primary or repro-cold; component, lane and iteration at most 16 characters so that the key fits the 200-character safe name).
# Enforced with `--grammar build` (what scripts/build/dispatch.sh passes); without it a purpose keeps the older rule (any safe name).
case "$grammar" in "") ;; build) [[ "$purpose" =~ ^build:[A-Za-z0-9_.-]{1,16}:[A-Za-z0-9_.-]{1,16}:[0-9a-f]{64}:[0-9a-f]{64}:(primary|repro-cold)(:[A-Za-z0-9_-]{1,16})?$ ]] \
  || lo_die purpose_key_malformed "a build purpose key is build:<component>:<lane>:<snapshot-digest 64 hex>:<argv-digest 64 hex>:<primary|repro-cold>[:<iteration>]" ;; *) lo_die usage_error "--grammar is build" ;; esac
[[ "$pid" =~ ^[0-9]+$ ]] || lo_die usage_error "--pid must be an integer"
[ -n "$opid" ] || opid="$(date -u +%Y%m%dT%H%M%SZ)-$$-$RANDOM"
lo_safe_name "$opid" || lo_die usage_error "unsafe op id"
for n in "$np" "$wall" "$mem" "$cpus"; do [[ "$n" =~ ^[0-9]+$ ]] || lo_die usage_error "numeric option expected"; done
lo_init
[ ! -e "$(lo_op_file "$opid")" ] || lo_die op_exists "op id $opid is already registered" "$RC_CONFLICT"
now=$(lo_now); st=$(lo_pstart "$pid")
wpj=$(printf '%s\n' "${wp[@]:-}" | jq -R . | jq -sc 'map(select(length>0))')
rec=$(jq -nc --arg id "$opid" --arg p "$purpose" --arg o "$owner" --argjson pid "$pid" --arg st "$st" --arg cmd "$(lo_cmdline "$pid")" \
  --arg cid "$cid" --arg clab "${clab:-$opid}" --arg log "$log" --argjson now "$now" --arg u "$(lo_utc "$now")" \
  --argjson np "$np" --argjson wall "$wall" --argjson mem "$mem" --argjson cpus "$cpus" --argjson wp "$wpj" \
  '{op_id:$id,purpose_key:$p,owner:$o,run_id:$id,pid:$pid,pgid:0,start_time:$st,cmdline:$cmd,container_id:$cid,container_label:$clab,
    state:"registered",started_utc:$u,last_heartbeat_utc:$u,heartbeat_seq:0,progress_offset:0,last_progress_epoch:$now,log_path:$log,
    budget:{memory_bytes:$mem,cpus:$cpus,wall_clock_s:$wall,no_progress_s:$np},write_paths:$wp,verdict:"",evidence_path:""}')
if [ "$claim" = 1 ]; then
  lo_with_lock "$purpose" lo_claim "$purpose" "$(lo_holder_json "$purpose" "$opid" "$pid")" || exit $?
fi
lo_wjson "$(lo_op_file "$opid")" "$rec" || lo_die write_failed "cannot write the op record" 1
lo_event registered --arg op "$opid" --arg purpose "$purpose"
echo "$opid"
