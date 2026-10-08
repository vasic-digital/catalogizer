#!/usr/bin/env bash
# register.sh - register a long operation BEFORE it starts and claim its purpose atomically (11.4.232 A, B; docs/16 13.1; T089).
#
# Usage   register.sh --purpose <key> --owner <name> [--op-id <id>] [--pid <pid>] [--container-label <l>] [--container-id <c>]
#                     [--write-path <path>]... [--no-progress-s <n>] [--wall-s <n>] [--stop-grace-s <n>] [--memory-bytes <n>] [--cpus <n>] [--log <file>] [--no-claim] [--grammar build]
# Effect  creates ops/<op_id>.json FIRST (exclusively, state `registered`, every declared write path recorded, the owner's pid + /proc start time + boot id + cmdline), THEN claims claims/<purpose> by `mkdir`
#         (a live holder gives exit 3 purpose_conflict, a stale one exit 4, nothing started twice), appends an event. Prints the op id. The order matters (LO-G1): a signal between the two writes can never
#         leave a claim with no op record; a lost claim rewrites OUR record `failed` with verdict `claim_refused:<rc>`, and a TERM/INT/HUP inside the window rolls back (own claim only, record `failed`/`interrupted`).
# Pid     defaults to the caller's parent (the wrapper that outlives this script); identity is its /proc start time and cmdline. A pid must be an
#         integer > 1 that exists now (0, 1, junk and a vanished pid are refused, WF11 F6); a pid that vanishes while the record is built is refused (1 pid_vanished): no record ever names an empty start time.
#         No --no-progress-s (or 0) records the default budget (LONGOPS_DEFAULT_NO_PROGRESS_S, 3600 s): an op is never "never hung" (WF11 F7). Every numeric option is a canonical non-negative integer of at most 15 digits.
#         --stop-grace-s  how long the owner needs to stop its workload after TERM (the runner wrapper: STOP_GRACE_S + 20); reap.sh waits max(LONGOPS_REAP_GRACE_S, stop_grace_s + 10) (LO-D4).
# Exits   0 registered; 1 write_failed / pid_vanished; 2 usage; 3 purpose_conflict (or op_exists); 4 stale claim; 20 refusal (tmpfs state, unreadable holder record).
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
purpose=""; owner=""; opid=""; grammar=""; pid=$PPID; clab=""; cid=""; wp=(); np=0; wall=0; sg=""; mem=0; cpus=0; log=""; claim=1
while [ $# -gt 0 ]; do
  case "$1" in
    --purpose) lo_need "$@"; purpose=$2; shift 2 ;; --owner) lo_need "$@"; owner=$2; shift 2 ;; --op-id) lo_need "$@"; opid=$2; shift 2 ;; --pid) lo_need "$@"; pid=$2; shift 2 ;;
    --container-label) lo_need "$@"; clab=$2; shift 2 ;; --container-id) lo_need "$@"; cid=$2; shift 2 ;; --write-path) lo_need "$@"; wp+=("$2"); shift 2 ;;
    --no-progress-s) lo_need "$@"; np=$2; shift 2 ;; --wall-s) lo_need "$@"; wall=$2; shift 2 ;; --stop-grace-s) lo_need "$@"; sg=$2; shift 2 ;; --memory-bytes) lo_need "$@"; mem=$2; shift 2 ;; --cpus) lo_need "$@"; cpus=$2; shift 2 ;;
    --log) lo_need "$@"; log=$2; shift 2 ;; --no-claim) claim=0; shift ;; --grammar) lo_need "$@"; grammar=$2; shift 2 ;;
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
lo_pid_ok "$pid" || lo_die usage_error "--pid must be an integer > 1 naming a process that exists now"
[ -n "$opid" ] || opid="$(date -u +%Y%m%dT%H%M%SZ)-$$-$RANDOM"
lo_safe_name "$opid" || lo_die usage_error "unsafe op id"
for n in "$np" "$wall" "$mem" "$cpus" "${sg:-0}"; do lo_uint "$n" || lo_die usage_error "numeric option expected (a canonical non-negative integer of at most 15 digits)"; done
[ "$np" -gt 0 ] || np=$LO_DEFAULT_NP
lo_init
# A record this script itself failed BEFORE the operation ever started (a lost claim `claim_refused:<rc>`, a signal `interrupted`; never heartbeated) does not hold its op id: scripts/build/dispatch.sh reg_adopt and
# scripts/build/event_hub.sh retry the SAME op id after `reap.sh --purpose` released a stale claim (exit 4), and that retry must not meet `op_exists`. Any other existing record keeps its id.
_ofile=$(lo_op_file "$opid")
if [ -e "$_ofile" ] && jq -e '.state=="failed" and ((.verdict // "")|test("^(claim_refused:[0-9]+|interrupted)$")) and ((.heartbeat_seq // 0)==0)' "$_ofile" >/dev/null 2>&1; then rm -f -- "$_ofile"; lo_fsync_dir "$LD/ops"; fi
[ ! -e "$_ofile" ] || lo_die op_exists "op id $opid is already registered" "$RC_CONFLICT"
now=$(lo_now); mono=$(lo_mono); st=$(lo_pstart "$pid"); bid=$(lo_boot_id)
{ [ -n "$st" ] && [ -n "$bid" ]; } || lo_die pid_vanished "pid $pid vanished (or the boot id is unreadable) while the record was built: a record never names an empty start time" 1
wpj=$(printf '%s\n' "${wp[@]:-}" | jq -R . | jq -sc 'map(select(length>0))')
rec=$(jq -nc --arg id "$opid" --arg p "$purpose" --arg o "$owner" --argjson pid "$pid" --arg st "$st" --arg bid "$bid" --arg cmd "$(lo_cmdline "$pid")" \
  --arg cid "$cid" --arg clab "${clab:-$opid}" --arg log "$log" --argjson now "$now" --argjson mono "$mono" --arg u "$(lo_utc "$now")" \
  --argjson np "$np" --argjson wall "$wall" --arg sg "$sg" --argjson mem "$mem" --argjson cpus "$cpus" --argjson wp "$wpj" \
  '{op_id:$id,purpose_key:$p,owner:$o,run_id:$id,pid:$pid,pgid:0,start_time:$st,boot_id:$bid,cmdline:$cmd,container_id:$cid,container_label:$clab,
    state:"registered",started_utc:$u,last_heartbeat_utc:$u,heartbeat_seq:0,progress_offset:0,last_progress_epoch:$now,last_progress_mono:$mono,log_path:$log,
    budget:({memory_bytes:$mem,cpus:$cpus,wall_clock_s:$wall,no_progress_s:$np} + (if $sg!="" then {stop_grace_s:($sg|tonumber)} else {} end)),write_paths:$wp,verdict:"",evidence_path:""}')
# the op record is created FIRST and EXCLUSIVELY (a hard link never overwrites): two registrations of one op id under different purposes cannot both win (WF11 F2 member), and a signal between the two
# writes can never leave a claim with no op record (LO-G1)
if ! lo_wjson_new "$(lo_op_file "$opid")" "$rec"; then
  if [ -e "$(lo_op_file "$opid")" ]; then lo_die op_exists "op id $opid is already registered" "$RC_CONFLICT"; fi
  lo_die write_failed "cannot write the op record" 1
fi
# _reg_fail <verdict>: terminal `failed` for OUR record (the run id is the op id) and release OUR claim only (another op's claim is never touched)
_reg_fail() {
  local j v=$1
  j=$(jq -c --arg v "$v" --arg u "$(lo_utc "$(lo_now)")" '.state="failed"|.verdict=$v|.last_heartbeat_utc=$u' "$(lo_op_file "$opid")" 2>/dev/null) && lo_wjson "$(lo_op_file "$opid")" "$j"
  [ "$claim" != 1 ] || lo_with_lock "$purpose" lo_unclaim_own "$purpose" "$opid" >/dev/null 2>&1
  lo_event register_failed --arg op "$opid" --arg verdict "$v"
}
trap '_reg_fail interrupted; exit 143' TERM INT HUP
lo_cs_pause   # test hook: widens the window between the record and the claim so a signal there is observable
if [ "$claim" = 1 ]; then
  lo_with_lock "$purpose" lo_claim "$purpose" "$(lo_holder_json "$purpose" "$opid" "$pid")"; crc=$?
  if [ "$crc" -ne 0 ]; then trap - TERM INT HUP; _reg_fail "claim_refused:$crc"; exit "$crc"; fi
fi
lo_cs_pause   # test hook: widens the window after the claim so a signal there is observable (the rollback releases OUR claim)
trap - TERM INT HUP
lo_event registered --arg op "$opid" --arg purpose "$purpose"
echo "$opid"
