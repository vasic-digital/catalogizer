#!/usr/bin/env bash
# mutate_registry.sh - paired mutations (G-GATE, 11.4.115 F / 1.1) of scripts/longops: each mutation breaks ONE load-bearing line in a COPY of
# the scripts; test_registry.sh run against that copy must FAIL (exit non-zero). A mutation that leaves the test green is a surviving mutant
# and fails this runner. The real scripts are never edited. SAFETY (WF11 F15, 11.4.263): a mutant can no longer run a real kill of pid <= 1 or -1, structurally:
# (1) ms_scan aborts a mutant that carries ANY signal/host-power line not byte-identical to the pristine tree; (2) every mutant runs inside the containment of
# mutation_safety.sh (BASH_ENV shim: the kill builtin is disabled, `kill` is a guard function that refuses pid 0, 1, -1, -pgid, junk and any pgid <= 1, pkill/killall refused,
# PATH stubs); the containment is itself tested with hypothetical bad mutants by test_mutation_safety.sh. The old two-string grep is gone.
# Usage  mutate_registry.sh [--only M05,M09]       Output: one line per mutation, then `MUTATION RESULT caught=N survived=M total=T`
. "$(dirname "$0")/lib.sh"   # TROOT, S (the real scripts), FX scratch
. "$(dirname "$0")/mutation_safety.sh"
[ "$(id -u)" != 0 ] || { echo "refusing to run mutations as root"; exit 2; }
ms_prepare "$FX/shim" || { echo "SAFETY: the mutant containment cannot be built; no mutant is run"; exit 2; }
ONLY=""; [ "${1:-}" = --only ] && ONLY=,${2:-},
caught=0; surv=0; tot=0
M() {  # M <id> <file> <description> <python-old> <python-new> [<python-old2> <python-new2>]  (the optional second pair is applied too, in the same mutant)
  local id=$1 file=$2 desc=$3 old=$4 new=$5 old2=${6:-} new2=${7:-}
  [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); local d=$FX/mut-$id; rm -rf "$d"; mkdir -p "$d"; cp "$S"/*.sh "$d/"
  python3 - "$d/$file" "$old" "$new" "$old2" "$new2" <<'PY' || { echo "INVALID $id $desc: pattern not found in $file"; surv=$((surv+1)); return; }
import sys
p,old,new,old2,new2=sys.argv[1:6]; s=open(p).read()
if old not in s: sys.exit(1)
s=s.replace(old,new,1)
if old2:
    if old2 not in s: sys.exit(1)
    s=s.replace(old2,new2,1)
open(p,'w').write(s)
PY
  # SAFETY layer 1: the mutant tree may not add or edit a signal / host-power line (layer 2, the containment, wraps the run below)
  if ! ms_scan "$S" "$d" >"$FX/scan-$id.out" 2>&1; then echo "SAFETY-ABORT $id: $(head -1 "$FX/scan-$id.out" | cut -c1-200); not run"; surv=$((surv+1)); return; fi
  LONGOPS_SCRIPTS=$d ms_run bash "$(dirname "$0")/test_registry.sh" >"$FX/mut-$id.out" 2>&1; local rc=$?
  if [ $rc -ne 0 ]; then caught=$((caught+1)); echo "CAUGHT   $id $desc :: $(grep -m2 '^FAIL' "$FX/mut-$id.out" | cut -c1-110 | tr '\n' '|')"
  else surv=$((surv+1)); echo "SURVIVED $id $desc"; fi
}
M M01 lib.sh "drop the flock: every CAS runs unlocked (adopt vs expire gives two winners)" '( flock -w "$LO_LOCK_WAIT" 9 || exit 70; "$@" ) 9>' '( "$@" ) 9>'
M M02 check_no_build_writing_tracked.sh "check_no_build_writing_tracked made always-pass" 'set -u
' 'set -u
exit 0
'
M M03 require_verdicts.sh "require_verdicts accepts a missing verdict" "printf 'refused\\t%s\\tmissing\\n' \"\$f\"; rc=1; continue;" "printf 'ok\\t%s\\n' \"\$f\"; continue;"
M M04 lib.sh "signal guards on pid and pgid removed (the real kill is replaced by a no-op IN THE SAME MUTANT: a guard-less kill of 0 or -1 would signal the whole session)" '[[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return "$RC_UNSAFE"
  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt 1 ]] || return "$RC_UNSAFE"' 'pg=$(lo_ppgrp "$pid")' 'kill -s "$sig" -- "$pid" 2>/dev/null' ':'
M M05 lib.sh "classification never reports hung (flat offset never detected)" '[ "$np" -gt 0 ] && [ $((now - lp)) -gt "$np" ]' 'false'
M M06 reap.sh "a live advancing op may be reaped" '  advancing) echo' '  advancing_x) echo'
M M07 lib.sh "process identity ignores the /proc start time (a recycled pid reads live)" '[ "$(lo_pstart "$pid")" = "$st" ] || return 1' 'true'
M M08 lib.sh "record write is not atomic (remove, pause, rename)" 'mv -f "$tmp" "$f"' 'rm -f "$f"; sleep 0.05; mv -f "$tmp" "$f"'
M M09 lib.sh "the purpose claim is not exclusive (no existing-claim check, mkdir -p)" '  if [ -d "$LD/claims/$p" ]; then' '  if false; then'
M M10 lib.sh "tmpfs state accepted" 'lo_die tmpfs_state' ': tmpfs_state'
M M11 reap.sh "reap does not resolve the real identity from /proc" 'if [ "$want" != "$have" ]; then' 'if false; then'
M M12 acquire.sh "--expire ignores the resume_ttl" '[ "$(lo_now)" -ge $((rdy + ttl2)) ] ||' 'true ||'
M M13 lib.sh "CPA_APPROVED_DIR check dropped for commit_push" '[ "$1" != commit_push ] || [ -n "${CPA_APPROVED_DIR:-}" ] || lo_die' '[ "$1" != commit_push ] || [ -n "${CPA_APPROVED_DIR:-}" ] || : lo_die'
M M14 lib.sh "a dead process holder reads live" 'if lo_alive "$pid" "$st"; then echo live; else echo dead; fi' 'echo live'
M M15 check_no_build_writing_tracked.sh "write paths under \$EV/\$AUD no longer block" '"$(realpath -m -- "$EV")"/*|"$(realpath -m -- "$AUD")"/*)' '"/nonexistent-ev"/*)'
M M16 require_verdicts.sh "fingerprint not compared" 'if [ -n "$fp" ] &&' 'if false &&'
M M17 acquire.sh "adopt does not check it adopts a suspended-run of that run (no CAS)" '[ -n "$h" ] && [ "$(jq -r .run_id <<<"$h")" = "$run" ] && [ "$(jq -r .kind <<<"$h")" = suspended-run ] || { echo "cas_mismatch: no suspended-run holder for $run" >&2; return "$RC_CAS"; }' ':'
M M18 lib.sh "stale-holder reader does not re-read the record (snapshot race)" '[ "$(cat "$hf" 2>/dev/null)" = "$s2" ] &&' 'true &&'
M M19 register.sh "the build purpose-key grammar is not enforced (T089a)" 'build) [[ "$purpose" =~' 'build) true || [[ "$purpose" =~'
M M20 lib.sh "the wall-clock cap is ignored: an advancing but over-long build never reads hung (T089a)" 'if [ "$wc" -gt 0 ] && [ "$el" -gt $((wc * 1000)) ]; then echo hung;' 'if false; then echo hung;'
M M21 heartbeat.sh "the build host's elapsed time is not recorded (T089a)" '| (if $el!="" then .elapsed_ms=($el|tonumber) else . end)' ''
M M22 lib.sh "RM1: the pgid guard of lo_signal is dropped (pid guard kept)" '  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt 1 ]] || return "$RC_UNSAFE"
  printf' '  pg=$(lo_ppgrp "$pid")
  printf'
M M23 lib.sh "RM2: the zombie check of lo_alive is dropped" '  [ "$(lo_pstate "$pid")" != Z ] || return 1
' ''
M M24 lib.sh "RM3: lo_safe_name accepts a slash after the first character" '[A-Za-z0-9._@+:=-]{0,199}$' '[A-Za-z0-9._@+:=/-]{0,199}$'
M M25 lib.sh "RM4: lo_claim swallows the conf-unreadable refusal" '[ "$rc" -eq "$RC_REFUSE" ] && return "$RC_REFUSE"; [ "$rc" -eq 0 ] || s=nohold' '[ "$rc" -eq 0 ] || s=nohold'
M M26 reap.sh "F1: a process that survived TERM is still recorded reaped" '    dead_owner) _reap_write "$j" hung' '    dead_owner|advancing|hung) _reap_write "$j" hung'
M M27 reap.sh "F2: the reap decision uses a snapshot taken before the lock (decision outside the lock)" 'j=$(cat "$f"); r=$(lo_classify_op "$j"); cls=' 'j=${SNAP:-$(cat "$f")}; r=$(lo_classify_op "$j"); cls=' 'lo_load_op "$opid"
lo_test_pause
SF=' 'lo_load_op "$opid"
SNAP=$(cat "$f"); lo_test_pause
SF='
M M28 reap.sh "F2: reap --purpose judges a holder snapshot taken before the lock" '    s=$(lo_holder_status "$purpose"); rc=$?
    [ "$rc" -ne "$RC_REFUSE" ] || return "$RC_REFUSE"' '    s=$PRE; rc=0
    [ "$rc" -ne "$RC_REFUSE" ] || return "$RC_REFUSE"' '  lo_require_approved "$purpose"
  lo_test_pause' '  lo_require_approved "$purpose"
  PRE=$(lo_holder_status "$purpose" 2>/dev/null); lo_test_pause'
M M29 lib.sh "F3: an unreadable holder record is read as dead" '<<<"$h" || { echo unreadable; return 0; }
  kind=' '<<<"$h" || { echo dead; return 0; }
  kind='
M M30 lib.sh "F5: lo_wjson writes empty or invalid JSON (no input check, no -e, no size check)" '  local f=$1 tmp; [ -n "${2:-}" ] || return 1
  tmp=$(mktemp "$(dirname "$f")/.tmp.XXXXXX") || return 1
  { printf '"'"'%s\n'"'"' "$2" | jq -ce . >"$tmp" 2>/dev/null && [ -s "$tmp" ]; } || { rm -f "$tmp"; return 1; }
  sync "$tmp" 2>/dev/null; mv -f' '  local f=$1 tmp
  tmp=$(mktemp "$(dirname "$f")/.tmp.XXXXXX") || return 1
  printf '"'"'%s\n'"'"' "$2" | jq -c . >"$tmp" 2>/dev/null
  sync "$tmp" 2>/dev/null; mv -f'
M M31 heartbeat.sh "F6: a heartbeat may rebind the op to pid 1, 0 or a dead pid" '[ -z "$pid" ] || lo_pid_ok "$pid" ||' '[ -z "$pid" ] || true ||'
M M32 register.sh "F6: register accepts pid 0, 1 and a pid that does not exist" 'lo_pid_ok "$pid" || lo_die usage_error "--pid must be an integer > 1 naming a process that exists now"' '[[ "$pid" =~ ^[0-9]+$ ]] || lo_die usage_error "--pid must be an integer"'
M M33 register.sh "F7: register records a 0 no-progress budget (never hung)" '[ "$np" -gt 0 ] || np=$LO_DEFAULT_NP' ':'
M M34 lib.sh "F7: a record with no no-progress budget is never hung" '  [ "$np" -gt 0 ] || np=$LO_DEFAULT_NP   #' '  :   #'
M M35 release.sh "F11: a terminal record can be rewritten" '      complete|failed|reaped|blocked-escape)
        if' '      complete_x)
        if'
M M36 lib.sh "class 2: an unparsable op record is classified dead_owner" '<<<"$j" || { echo unreadable; echo "op record is empty, unparsable or has no string state"; return 0; }' '<<<"$j" || { echo dead_owner; echo "op record is empty, unparsable or has no string state"; return 0; }'
M M37 lib.sh "F2 member: the op record is created with an overwriting rename (not exclusive)" 'ln -- "$tmp" "$f" 2>/dev/null; local r=$?' 'mv -f "$tmp" "$f" 2>/dev/null; local r=$?'
M M38 lib.sh "class 3: lo_uint accepts any number of digits" '^[0-9]{1,15}$ ]]; }' '^[0-9]+$ ]]; }'
M M39 check_no_build_writing_tracked.sh "class 2: an unreadable op record is read as no writer" 'if [ "$cls" = unreadable ]; then printf' 'if false; then printf'
M M40 holder.sh "class 2: an unreadable holder is reported none" '  unreadable) lo_die holder_unreadable' '  unreadable_x) lo_die holder_unreadable'
M M41 classify.sh "F14: classify --op-id of an unknown op succeeds with empty output" '[ -e "$(lo_op_file "$only")" ] || lo_die unknown_op' 'true || lo_die unknown_op'
M M42 reap.sh "label: reap does not look at the label the launcher sets (catalogizer.op_id)" 'cands=("op_id=$opid" "catalogizer.op_id=$opid")' 'cands=("op_id=$opid")'
M M43 lib.sh "RM5 (round-2 reviewer, verbatim): the last_progress_epoch is not validated: a valid-JSON record with a text epoch reads as an empty class / shell error" 'for v in "$np" "$lp" "$wc" "$el"; do' 'for v in "$np" "$wc" "$el"; do'
M M44 lib.sh "RM11 (round-2 reviewer, verbatim): a holder whose pid is not a number is judged by lo_alive instead of being unreadable" 'lo_uint "$pid" || { echo unreadable; return 0; }' ':'
M M45 reap.sh "WF14 R2-1: the grace wait runs INSIDE the purpose lock (an owner whose exit path takes the lock cannot finish: the dispatch pump shape)" 'printf '"'"'%s %s\n'"'"' "$pid" "$pst" >"$SF"; return 0 ;;' 'printf '"'"'%s %s\n'"'"' "$pid" "$pst" >"$SF"; n=0; while [ "$n" -lt $((GRACE * 5)) ]; do lo_alive "$pid" "$pst" || break; sleep 0.2; n=$((n+1)); done; return 0 ;;'
M M46 heartbeat.sh "WF14 R2-3: a rebind does not move the claim holder with the op owner" 'if [ "$(jq -r '"'"'.run_id // ""'"'"' <<<"$h" 2>/dev/null)" = "$(jq -r .run_id <<<"$j")" ]; then' 'if false; then'
M M47 reap.sh "WF14 R2-3: reap --purpose ignores a live op of the purpose" 'if lo=$(_live_op_of_purpose); then' 'if false; then'
M M48 lib.sh "WF14 R2-5: lo_pid_ok accepts a zombie" '[ "$(lo_pstate "$1")" != Z ] && ' ''
M M49 lib.sh "WF14 R2-5: lo_pid_ok accepts a kernel thread (process group 0)" ' && g=$(lo_ppgrp "$1") && [[ "$g" =~ ^[0-9]+$ && "$g" -gt 1 ]]; }' '; }'
M M50 lib.sh "WF14 R2-6: the default budget is not validated (0 or junk brings back never-hung)" 'lo_uint "$LO_DEFAULT_NP" && [ "$LO_DEFAULT_NP" -gt 0 ] ||' 'true ||'
M M51 release.sh "WF14 R2-10: release reports a claim that belongs to another run as a failed CAS after the record was written" 'lo_unclaim_own "$purpose" "$runid"' 'lo_unclaim "$purpose" "$runid"'
M M52 reap.sh "WF14 R2-10: reap of a --no-claim op fails the CAS after the record was written" 'lo_unclaim_own "$purpose" "$(jq -r .run_id <<<"$1")"' 'lo_unclaim "$purpose" "$(jq -r .run_id <<<"$1")"'
M M53 reap.sh "WF14 R2-4: the record's own container_label is not used to find the container" 'case "$clab" in "$opid"|"") ;; *=*) cands+=("$clab") ;; *) cands+=("catalogizer.op_id=$clab" "op_id=$clab") ;; esac' ':'
M M54 lib.sh "WF14 class D: a record whose pid is not a number is classified by lo_alive (dead_owner), not unreadable" '{ lo_uint "$pid" && [[ "$pst" =~ ^[0-9]*$ ]]; } ||' 'true ||'
M M55 lib.sh "WF14: lo_unclaim_own releases a claim that belongs to another run" 'if [ "$cur" = "$run" ]; then rm -rf' 'if true; then rm -rf'
M M56 lib.sh "WF14: the lock wait budget is ignored (fixed 15 s)" '( flock -w "$LO_LOCK_WAIT" 9 || exit 70; "$@" ) 9>' '( flock -w 15 9 || exit 70; "$@" ) 9>'
M M57 lib.sh "WF14 class D: a process holder whose start_time cannot identify a process is judged dead, not unreadable" '[[ "$st" =~ ^[0-9]+$ ]] || { echo unreadable; return 0; }' ':'
M M58 lib.sh "WF14 class D: a suspended-run holder with a mistyped field is judged dead, not unreadable" '((.builds // [])|type=="array" and all(.[]; type=="string")) and (.state|type=="string") and ((.callback_state // "none")|type=="string")' 'true'
echo "MUTATION RESULT caught=$caught survived=$surv total=$tot"
[ "$surv" -eq 0 ]
