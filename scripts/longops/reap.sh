#!/usr/bin/env bash
# reap.sh - reap a long operation ONLY on proven staleness (11.4.232 E; 11.4.196 D; 11.4.263).
#
# Usage   reap.sh --op-id <id> [--expect dead_owner|hung] [--dry-run]
#         reap.sh --purpose <key> [--dry-run]     release the claim of a dead holder; every `dead_owner` op of the purpose is reaped FIRST (with the container proof), and a purpose with a live op is refused
# Rules   OWNER GONE IS NOT WORKLOAD GONE (11.4.276 round 5, class F). An op is recorded `reaped` and its claim freed only when (1) the owner is gone, (2) EVERY container of EVERY candidate label is listed NOT
#         running after `podman stop`, and (3) the own-run purpose holder is not live. Any of the three unproven: the record and the claim are kept, with a distinct exit (9 container_survived, 20 podman unreadable, 5 holder_live).
#         dead_owner  the recorded pid no longer matches /proc (start time, boot id): no signal at all; the containers are stopped and re-listed (unlocked), then the record becomes `reaped` and the claim is released.
#         hung        the owner lives, its identity is re-resolved from /proc/<pid>/cmdline (must equal the recorded cmdline) and the start time is re-checked; the REAP INTENT (reap_requested_utc, reap_requested_by) is written
#                     into the record under the lock; then SIGTERM goes to that ONE pid through lo_signal, which refuses pid or pgid <= 1 and never signals a process group. A bare process-name search is never used.
#         advancing   refused (exit 5): a live advancing op is never reaped.
#         identity unresolvable (cmdline differs) refused, exit 6: conservative-safe (11.4.201 4), the evidence is printed.
#         --expect C  the class the caller judged (the sweep passes dead_owner): when the class under the lock differs, nothing is written and the exit is 5 class_changed (a dead-owner row rebound to a live pid
#                     before the action must never receive a TERM from an "auto-safe, no signal" reconcile, LO-C2).
# Lock    The purpose lock is held for DECISIONS and WRITES only, never across waiting or a container runtime (WF14 R2-1, R2-8). Three steps, the middle one outside the lock:
#           A (locked)    classify, identity, re-check the owner's start time, write the reap intent, send TERM to the one pid. Nothing slow runs here.
#           wait (unlocked) stop EVERY container of the op (`timeout $LONGOPS_PODMAN_TIMEOUT_S podman stop`, default 30 s each) and wait up to max(LONGOPS_REAP_GRACE_S (default 15), budget.stop_grace_s + 10) for a signalled
#                         owner to exit; then LIST the containers again. The owner can finish its own exit path, which may itself take the lock (the dispatch pump's TERM trap runs `release.sh --state handoff`,
#                         the runner wrapper stops its container before `release.sh`).
#           B (locked)    RE-DERIVE from the record: the owner released the op itself -> report that state (exit 0, never "survived"); an owner `handoff` that answers OUR reap intent is recorded `reaped` with verdict
#                         `reaped_hung:owner_handoff` (owner_verdict kept): a build the registry judged HUNG is not re-adoptable; the owner is gone and no container runs -> `reaped`; a container still running -> exit 9, record
#                         and claim kept; still alive -> survivor.
#         `--purpose` re-reads the holder under the lock; an unreadable holder record, an unreadable op record (any record of ops/ that fails the record shape), an unreadable conf (commit_push) or an unset CPA_APPROVED_DIR
#         REFUSES (20), it is never read as a stale claim; a claim whose purpose has a non-terminal op with a LIVE owner is never released (5): the claim is the op's even when its holder record is stale (WF14 R2-3).
# Survivor The op is recorded `reaped` ONLY when the process is gone after the grace period. A process that ignores TERM keeps its record (state unchanged, `reap_survived_utc`
#         set), keeps its claim (the purpose still has ONE owner) and the script exits 8: an operator decision, never two live owners (WF11 F1).
# Containers resolved by label `op_id=<id>` AND `catalogizer.op_id=<id>` (the label scripts/containers/run_pinned.sh really sets) AND the op record's own `container_label`
#         (scripts/build/dispatch.sh records `catalogizer.op_id=dispatch-<build id>`, the op id is `<build id>`; WF14 R2-4).
# Test hooks LONGOPS_TEST_SLEEP_BEFORE_LOCK (seconds) pauses before the lock so a decision outside the lock is observable; LONGOPS_TEST_SLEEP_BEFORE_SIGNAL pauses between classification and the pre-signal re-check.
# Exits   0 reaped / released itself (or would be, with --dry-run); 1 a registry write failed; 5 live advancing (or a live op owns the purpose, or the own-run holder is live, or class_changed); 6 identity mismatch; 7 unsafe signal target;
#         8 the process survived TERM (record and claim kept); 9 a container of the op is still running (record and claim kept); 4 cas / unknown op; 2 usage; 20 refusal (unreadable record or holder or container runtime,
#         helper not approved); 70 lock wait.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
opid=""; purpose=""; dry=0; expect=""
while [ $# -gt 0 ]; do case "$1" in --op-id) lo_need "$@"; opid=$2; shift 2 ;; --purpose) lo_need "$@"; purpose=$2; shift 2 ;; --expect) lo_need "$@"; expect=$2; shift 2 ;; --dry-run) dry=1; shift ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
case "$expect" in ""|dead_owner|hung) ;; *) lo_die usage_error "--expect is dead_owner or hung" ;; esac
GRACE=${LONGOPS_REAP_GRACE_S:-15}; lo_uint "$GRACE" || lo_die usage_error "LONGOPS_REAP_GRACE_S must be a non-negative integer"
if [ -n "$purpose" ]; then
  [ -z "$expect" ] || lo_die usage_error "--expect is for --op-id"
  lo_safe_name "$purpose" || lo_die usage_error "unsafe purpose"
  lo_require_approved "$purpose"
  lo_test_pause
  # LO-G2: a stale claim is released only together with the dead-owner ops of the purpose. Releasing the claim and leaving `dead_owner` rows behind kept every container run and every commit-push refused (the
  # sweep's registry_row_dead_owner + duplicate_owner) until a manual `sweep --reconcile`. Each such op is reaped with the full container proof; one that cannot be reaped keeps the claim (its exit code is returned).
  while IFS=$'\t' read -r kind file state oid pk json; do
    [ "$kind" = ok ] || continue
    case "$state" in registered|running) ;; *) continue ;; esac
    [ "$pk" = "$purpose" ] || continue
    [ "$(lo_classify_op "$json" | head -1)" = dead_owner ] || continue
    if [ "$dry" = 1 ]; then LONGOPS_TEST_SLEEP_BEFORE_LOCK= bash "${BASH_SOURCE[0]}" --op-id "$oid" --expect dead_owner --dry-run || exit $?
    else LONGOPS_TEST_SLEEP_BEFORE_LOCK= bash "${BASH_SOURCE[0]}" --op-id "$oid" --expect dead_owner || exit $?; fi
  done < <(lo_ops_snapshot)
  _live_op_of_purpose() {   # prints `unreadable:<what>` when ANY record of ops/ cannot be judged (it takes priority: an unjudgeable record is never "another purpose's" and never "no live op"), else `live:<id>` for a
    # non-terminal op of this purpose whose owner lives (advancing or hung); return 1 when none. The shape check comes BEFORE the purpose filter: a record that fails the shape names no trustworthy purpose (LO-A4).
    local kind file state oid pk json c snap
    snap=$(lo_ops_snapshot)
    while IFS=$'\t' read -r kind file state oid pk json; do
      if [ "$kind" = bad ]; then echo "unreadable:$file"; return 0; fi
    done <<<"$snap"
    while IFS=$'\t' read -r kind file state oid pk json; do
      [ "$pk" = "$purpose" ] || continue
      case "$state" in registered|running) ;; *) continue ;; esac
      c=$(lo_classify_op "$json" | head -1)
      case "$c" in advancing|hung) echo "live:$oid"; return 0 ;; unreadable) echo "unreadable:$oid"; return 0 ;; esac
    done <<<"$snap"; return 1
  }
  _rc() {   # under the purpose lock: the holder is re-read HERE; a live, expired or unreadable holder is never released
    local s rc lo
    [ -d "$LD/claims/$purpose" ] || { echo "nothing to release: $purpose has no claim"; return 0; }
    s=$(lo_holder_status "$purpose"); rc=$?
    [ "$rc" -ne "$RC_REFUSE" ] || return "$RC_REFUSE"
    [ "$rc" -eq 0 ] || s=nohold          # a claim directory with no holder record: stale (a crash between mkdir and the record)
    case "$s" in
      live|expired) echo "refused: holder of $purpose is $s" >&2; return "$RC_LIVE" ;;
      unreadable) echo "refused: the holder record of $purpose cannot be read (holder_unreadable); never released as stale" >&2; return "$RC_REFUSE" ;;
    esac
    if lo=$(_live_op_of_purpose); then
      case "$lo" in
        live:*) echo "refused: the holder record of $purpose is $s but op ${lo#live:} of that purpose has a LIVE owner: its claim is never released (WF14 R2-3)" >&2; return "$RC_LIVE" ;;
        *) echo "refused: op ${lo#unreadable:} of purpose $purpose cannot be judged (unreadable record); its claim is never released as stale" >&2; return "$RC_REFUSE" ;;
      esac
    fi
    [ "$dry" = 1 ] && { echo "would release stale claim $purpose ($s)"; return 0; }
    rm -rf -- "$LD/claims/$purpose"; lo_fsync_dir "$LD/claims"; lo_event reaped_claim --arg purpose "$purpose" --arg was "$s"; echo "released stale claim $purpose ($s)"
  }
  lo_with_lock "$purpose" _rc; exit $?
fi
[ -n "$opid" ] && lo_safe_name "$opid" || lo_die usage_error "--op-id or --purpose required"
lo_load_op "$opid"
lo_test_pause
SF=$(mktemp "${TMPDIR:-/tmp}/reap-state.XXXXXX") || lo_die internal "cannot create a scratch file" 1
trap 'rm -f "$SF"' EXIT

# the candidate container labels of the op, and its containers, resolved OUTSIDE the lock (a wedged container runtime must not hold the purpose lock, WF14 R2-8).
clab=$(jq -r '.container_label' "$f"); cands=("op_id=$opid" "catalogizer.op_id=$opid")
case "$clab" in "$opid"|"") ;; *=*) cands+=("$clab") ;; *) cands+=("catalogizer.op_id=$clab" "op_id=$clab") ;; esac
CIDS=()
_clist() {   # fills CIDS with EVERY running container of EVERY candidate label (no `head -1`: a test-infra stack labels each of its services with the op id); return 1 when the runtime cannot be read
  CIDS=(); local c ids i
  for c in "${cands[@]}"; do
    ids=$(timeout "$LO_PODMAN_TIMEOUT" "$PODMAN" ps --filter "label=$c" --format '{{.ID}}' 2>/dev/null) || return 1
    while IFS= read -r i; do
      [ -n "$i" ] || continue
      lo_safe_name "$i" || return 1
      case " ${CIDS[*]:-} " in *" $i "*) ;; *) CIDS+=("$i") ;; esac
    done <<<"$ids"
  done
  return 0
}
_cstop() { local i; for i in "${CIDS[@]:-}"; do [ -n "$i" ] || continue; timeout "$LO_PODMAN_TIMEOUT" "$PODMAN" stop -t 5 "$i" >/dev/null 2>&1; done; }

# _own_holder: 0 nothing to protect; 5 the purpose holder is THIS op's own run and it is live/expired (the worker still runs under this claim, LO-F2); 20 the holder cannot be read.
_own_holder() {
  local hs hrc run=$1
  hs=$(lo_holder_status "$purpose"); hrc=$?
  [ "$hrc" -ne "$RC_REFUSE" ] || return "$RC_REFUSE"
  [ "$hrc" -eq 0 ] || return 0
  case "$hs" in
    unreadable) echo "refused: the holder record of $purpose cannot be read (holder_unreadable); the claim of op $opid is not released on a guess" >&2; return "$RC_REFUSE" ;;
    live|expired) [ "$(jq -r '.run_id // ""' "$(lo_holder_file "$purpose")" 2>/dev/null)" != "$run" ] || { echo "refused: holder_live: the purpose holder of $purpose is this op's own run $run and it is $hs: its worker still runs under the claim" >&2; return "$RC_LIVE"; } ;;
  esac
  return 0
}
_reap_write() {   # _reap_write <record-json> <class> <evidence>: the terminal write + the claim release (only this op's own claim, WF14 R2-10) + the event
  local k own; own=$(jq -r .run_id <<<"$1")
  _own_holder "$own" || return $?
  k=$(jq -c --arg u "$(lo_utc "$(lo_now)")" --arg cls "$2" '.state="reaped"|.verdict="reaped_"+$cls|.last_heartbeat_utc=$u' <<<"$1")
  lo_wjson "$f" "$k" && lo_unclaim_own "$purpose" "$own" || return $?
  lo_event reaped --arg op "$opid" --arg cls "$2"; echo "reaped $opid ($2: $3)"
}
_reap_a() {   # step A, under the purpose lock: decide, and for a hung op send TERM to the one pid. Writes "<mode> <pid> <start>" to $SF when the caller must continue outside the lock.
  local j r cls ev pid pst want have rc
  j=$(cat "$f"); r=$(lo_classify_op "$j"); cls=$(sed -n 1p <<<"$r"); ev=$(sed -n 2p <<<"$r")
  case "$cls" in
    terminal) echo "nothing to reap: $opid is already terminal"; return 0 ;;
    unreadable) echo "refused: the record of $opid is unreadable ($ev)" >&2; return "$RC_REFUSE" ;;
  esac
  [ -z "$expect" ] || [ "$cls" = "$expect" ] || { echo "refused: class_changed: the caller judged $expect but $opid is $cls ($ev); nothing was written" >&2; return "$RC_LIVE"; }
  pid=$(jq -r .pid <<<"$j"); pst=$(jq -r .start_time <<<"$j")
  case "$cls" in
    advancing) echo "refused: $opid is live and advancing ($ev)" >&2; return "$RC_LIVE" ;;
    hung)
      want=$(jq -r .cmdline <<<"$j"); have=$(lo_cmdline "$pid")
      if [ "$want" != "$have" ]; then echo "refused: identity of pid $pid unresolved: recorded '$want' but /proc says '$have'" >&2; return "$RC_IDENT"; fi
      [ "$dry" = 1 ] && { echo "would reap hung $opid pid=$pid cmdline='$have' container='${DRYC:-none}' ($ev)"; return 0; }
      [ -z "${LONGOPS_TEST_SLEEP_BEFORE_SIGNAL:-}" ] || sleep "$LONGOPS_TEST_SLEEP_BEFORE_SIGNAL"
      # the pid must still be the process that was judged (a recycled pid inside the window must never receive the signal, WF14 R2-9)
      [ "$(lo_pstart "$pid")" = "$pst" ] || { echo "refused: pid $pid is no longer the process that was judged (start time changed before the signal)" >&2; return "$RC_IDENT"; }
      # the REAP INTENT is written before the signal: an owner whose TERM handler hands the op off (the dispatch pump) is recognised in step B as answering OUR reap (LO-D2)
      lo_wjson "$f" "$(jq -c --arg u "$(lo_utc "$(lo_now)")" --arg by "$$ $(lo_pstart "$$")" '.reap_requested_utc=$u|.reap_requested_by=$by' <<<"$j")" || { echo "refused: the reap intent could not be written; nothing was signalled" >&2; return 1; }
      lo_signal TERM "$pid"; rc=$?
      [ "$rc" -ne "$RC_UNSAFE" ] || { echo "refused: unsafe signal target pid=$pid (rc $rc)" >&2; return "$RC_UNSAFE"; }
      printf 'hung %s %s\n' "$pid" "$pst" >"$SF"; return 0 ;;
    dead_owner)
      _own_holder "$(jq -r .run_id <<<"$j")" || return $?
      [ "$dry" = 1 ] && { echo "would reap dead $opid ($ev) container='${DRYC:-none}'"; return 0; }
      printf 'dead %s %s\n' "$pid" "$pst" >"$SF"; return 0 ;;
  esac
}
_reap_b() {   # step B, under the purpose lock: the record is re-read and re-classified; nothing from step A is trusted
  local j r cls ev mode=$1 st ov
  j=$(cat "$f"); r=$(lo_classify_op "$j"); cls=$(sed -n 1p <<<"$r"); ev=$(sed -n 2p <<<"$r")
  if [ "$CUNREAD" = 1 ] && [ "$cls" != terminal ]; then echo "refused: the containers of $opid cannot be listed (podman failed, timed out or printed an unsafe id): the workload is not proven gone; record and claim kept (container_runtime_unreadable)" >&2; return "$RC_REFUSE"; fi
  if [ "$CSURV" -gt 0 ] && [ "$cls" = dead_owner ]; then
    lo_wjson "$f" "$(jq -c --arg u "$(lo_utc "$(lo_now)")" '.reap_container_survived_utc=$u' <<<"$j")"
    lo_event reap_container_survived --arg op "$opid"
    echo "refused: $CSURV container(s) of $opid are still running after podman stop: the owner is gone but the workload is not; record and claim kept (container_survived)" >&2; return "$RC_CSURV"
  fi
  case "$cls" in
    terminal)
      st=$(jq -r .state <<<"$j")
      if [ "$mode" = hung ] && [ "$st" = handoff ] && [ -n "$(jq -r '.reap_requested_utc // empty' <<<"$j")" ]; then
        ov=$(jq -r '.verdict // ""' <<<"$j")
        lo_wjson "$f" "$(jq -c --arg u "$(lo_utc "$(lo_now)")" '.owner_verdict=.verdict|.state="reaped"|.verdict="reaped_hung:owner_handoff"|.last_heartbeat_utc=$u' <<<"$j")" || return 1
        lo_event reaped --arg op "$opid" --arg cls hung_owner_handoff
        echo "reaped $opid: the owner answered TERM with a handoff (owner verdict '$ov'); a build judged HUNG is not re-adoptable, so the record is reaped (verdict reaped_hung:owner_handoff)"; return 0
      fi
      if [ "$mode" = hung ]; then echo "reaped $opid: the owner released the op itself after TERM (state $st, verdict '$(jq -r .verdict <<<"$j")')"
      else echo "nothing to reap: $opid became terminal meanwhile (state $st, verdict '$(jq -r .verdict <<<"$j")')"; fi
      return 0 ;;
    unreadable) echo "refused: the record of $opid became unreadable after TERM ($ev)" >&2; return "$RC_REFUSE" ;;
    dead_owner) if [ "$mode" = hung ]; then _reap_write "$j" hung "TERM delivered, the owner is gone; $ev"; else _reap_write "$j" dead_owner "$ev"; fi; return $? ;;
    *) if [ "$mode" = dead ]; then echo "refused: class_changed: $opid was dead_owner and is $cls now ($ev); nothing was written" >&2; return "$RC_LIVE"; fi
       lo_wjson "$f" "$(jq -c --arg u "$(lo_utc "$(lo_now)")" '.reap_survived_utc=$u|.reap_attempts=((.reap_attempts // 0) + 1)' <<<"$j")"
       lo_event reap_survived --arg op "$opid" --argjson pid "$(jq -r .pid <<<"$j")"
       echo "refused: pid $(jq -r .pid <<<"$j") is still alive ${WAITED}s after TERM; $opid stays '$(jq -r .state <<<"$j")' and keeps its claim: an operator decision (reap_survived)" >&2; return "$RC_SURVIVED" ;;
  esac
}
DRYC=""
if [ "$dry" = 1 ]; then if _clist; then DRYC=${CIDS[*]:-}; DRYC=${DRYC:-none}; else DRYC=unreadable; fi; fi
lo_with_lock "$purpose" _reap_a; rc=$?
[ "$rc" -eq 0 ] && [ -s "$SF" ] || exit "$rc"
read -r mode pid pst <"$SF"
# unlocked: stop EVERY container (an unreadable runtime is recorded, never read as "no containers"), wait for a signalled owner, then LIST again: the owner being gone proves nothing about the workload
CUNREAD=0; CSURV=0
if _clist; then _cstop; else CUNREAD=1; fi
WAITED=$GRACE
sgv=$(jq -r '.budget.stop_grace_s // empty' "$f" 2>/dev/null)
if [ -n "$sgv" ] && lo_uint "$sgv" && [ "$sgv" -gt 0 ] && [ $((sgv + 10)) -gt "$WAITED" ]; then WAITED=$((sgv + 10)); fi
if [ "$mode" = hung ]; then
  n=0; while [ "$n" -lt $((WAITED * 5)) ]; do
    lo_alive "$pid" "$pst" || break
    case "$(jq -r '.state // ""' "$f" 2>/dev/null)" in complete|failed|reaped|handoff|blocked-escape) break ;; esac
    sleep 0.2; n=$((n+1))
  done
fi
if [ "$CUNREAD" = 0 ]; then if _clist; then CSURV=${#CIDS[@]}; else CUNREAD=1; fi; fi
lo_with_lock "$purpose" _reap_b "$mode"; exit $?
