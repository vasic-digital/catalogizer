#!/usr/bin/env bash
# mutate_registry.sh - paired mutations (G-GATE, 11.4.115 F / 1.1) of scripts/longops: each mutation breaks ONE load-bearing line in a COPY of
# the scripts; the registry suites run against that copy must FAIL (exit non-zero). A mutation that leaves every suite green is a surviving mutant and fails this runner. The real scripts are never edited.
# SAFETY (WF11 F15, 11.4.263): a mutant can no longer run a real kill of pid <= 1 or -1, structurally:
# (1) ms_scan aborts a mutant that carries ANY signal/host-power line not byte-identical to the pristine tree; (2) every mutant runs inside the containment of
# mutation_safety.sh (BASH_ENV shim: the kill builtin is disabled, `kill` is a guard function that refuses pid 0, 1, -1, -pgid, junk and any pgid <= 1, pkill/killall refused,
# PATH stubs); the containment is itself tested with hypothetical bad mutants by test_mutation_safety.sh. The old two-string grep is gone.
# SUITES (11.4.276 round 5): `M` runs test_registry.sh first and, only if the mutant survived it, test_registry_r5.sh; `MR <sections>` runs the named sections of test_registry_r5.sh first (R5_SECTIONS) and then
# test_registry.sh. A mutant is CAUGHT when ANY suite fails. TIME (LO-T4, 11.4.232 C): every suite run is bounded by `timeout ${MUT_TIMEOUT_S:-900}`; a timeout is its own result `TIMEOUT <id>`, fails the runner and is
# never counted as caught. An EQUIVALENT mutant (`EQ`) is recorded with its reason and counted apart; it is never silently dropped.
# Usage  mutate_registry.sh [--only M05,M09] [--check]       Output: one line per mutation, then `MUTATION RESULT caught=N survived=M timeout=T equivalent=E total=T`
#        --check (or MUT_CHECK_ONLY=1) applies every mutation and the safety scan but runs no suite: `APPLIES <id>` / `INVALID <id>` (a pattern that is not found is never counted as caught)
. "$(dirname "$0")/lib.sh"   # TROOT, S (the real scripts), FX scratch
. "$(dirname "$0")/mutation_safety.sh"
[ "$(id -u)" != 0 ] || { echo "refusing to run mutations as root"; exit 2; }
ms_prepare "$FX/shim" || { echo "SAFETY: the mutant containment cannot be built; no mutant is run"; exit 2; }
ONLY=""; CHECK=${MUT_CHECK_ONLY:-0}
while [ $# -gt 0 ]; do case "$1" in --only) ONLY=,${2:-},; shift 2 ;; --check) CHECK=1; shift ;; *) echo "unknown argument $1"; exit 2 ;; esac; done
TMO=${MUT_TIMEOUT_S:-900}
caught=0; surv=0; tot=0; tmo=0; eqv=0
_mk() {  # _mk <id> <file> <desc> <python-old> <python-new> [<old2> <new2>]: build the mutant tree $MD; 0 ok; 1 invalid (counted as survived); 2 safety abort (counted as survived)
  local id=$1 file=$2 desc=$3 old=$4 new=$5 old2=${6:-} new2=${7:-}
  MD=$FX/mut-$id; rm -rf "$MD"; mkdir -p "$MD"; cp "$S"/*.sh "$MD/"
  python3 - "$MD/$file" "$old" "$new" "$old2" "$new2" <<'PY' || { echo "INVALID $id $desc: pattern not found in $file"; surv=$((surv+1)); return 1; }
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
  if ! ms_scan "$S" "$MD" >"$FX/scan-$id.out" 2>&1; then echo "SAFETY-ABORT $id: $(head -1 "$FX/scan-$id.out" | cut -c1-200); not run"; surv=$((surv+1)); return 2; fi
  return 0
}
_suite() {  # _suite <id> <test-file> [ENV=VAL...]: run one suite against $MD inside the containment, bounded in time; sets SUITE_RC (124 = timeout)
  local id=$1 tf=$2; shift 2
  env "$@" MUT_FAILFAST=1 LONGOPS_SCRIPTS=$MD BASH_ENV=$MS_SHIM PATH=$MS_BIN:$PATH MS_LOG=$MS_LOG MS_REAL_KILL=${MS_REAL_KILL:-/usr/bin/kill} timeout "$TMO" bash "$(dirname "$0")/$tf" >"$FX/mut-$id.$tf.out" 2>&1; SUITE_RC=$?
}
_verdict() {  # _verdict <id> <desc> <rc-of-the-last-suite> <suite-file>
  local id=$1 desc=$2 rc=$3 sn=$4
  if [ "$rc" -eq 124 ]; then tmo=$((tmo+1)); echo "TIMEOUT  $id $desc (suite $sn exceeded ${TMO}s: never counted as caught)"
  elif [ "$rc" -ne 0 ]; then caught=$((caught+1)); echo "CAUGHT   $id $desc [$sn] :: $(grep -m2 '^FAIL' "$FX/mut-$id.$sn.out" | cut -c1-110 | tr '\n' '|')"
  else surv=$((surv+1)); echo "SURVIVED $id $desc"; fi
}
M() {  # M <id> <file> <description> <old> <new> [<old2> <new2>]: test_registry.sh first, then test_registry_r5.sh (all sections)
  local id=$1; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); _mk "$@" || return 0
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return 0; }
  _suite "$id" test_registry.sh; if [ "$SUITE_RC" -eq 0 ]; then _suite "$id" test_registry_r5.sh; _verdict "$id" "$3" "$SUITE_RC" test_registry_r5.sh; else _verdict "$id" "$3" "$SUITE_RC" test_registry.sh; fi
}
MR() {  # MR <sections> <id> <file> <description> <old> <new> [<old2> <new2>]: the named sections of test_registry_r5.sh first (R5_SECTIONS), then test_registry.sh
  local sec=$1; shift; local id=$1; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); _mk "$@" || return 0
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return 0; }
  _suite "$id" test_registry_r5.sh R5_SECTIONS="$sec"; if [ "$SUITE_RC" -eq 0 ]; then _suite "$id" test_registry.sh; _verdict "$id" "$3" "$SUITE_RC" test_registry.sh; else _verdict "$id" "$3" "$SUITE_RC" test_registry_r5.sh; fi
}
NC() {  # NC <id> <file> <description> <text>: the NEGATIVE CONTROL. An UNMUTATED copy (the text is replaced by itself) goes through the identical pipeline (copy, scan, containment, both suites, fail-fast env) and MUST PASS;
  # a control that fails means the pipeline itself fails every tree and every CAUGHT above proves nothing (11.4.201 (7)(b))
  local id=$1 file=$2 desc=$3 text=$4; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); _mk "$id" "$file" "$desc" "$text" "$text" || return 0
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return 0; }
  _suite "$id" test_registry.sh; local r1=$SUITE_RC r2=0
  if [ "$r1" -eq 0 ]; then _suite "$id" test_registry_r5.sh; r2=$SUITE_RC; fi
  if [ "$r1" -eq 0 ] && [ "$r2" -eq 0 ]; then echo "CONTROL-OK $id $desc (both suites PASSED on the unmutated copy: a CAUGHT elsewhere is a real kill)"; else surv=$((surv+1)); echo "CONTROL-BROKEN $id $desc (suite rc $r1/$r2): the pipeline fails an UNMUTATED tree, every CAUGHT is void"; fi
}
EQ() {  # EQ <id> <reason> <file> <description>: a mutant proven EQUIVALENT is recorded with its reason, never run as a survivor and never silently dropped
  local id=$1 reason=$2; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); eqv=$((eqv+1)); echo "EQUIVALENT $id $4 :: $reason"
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
M M15 check_no_build_writing_tracked.sh "write paths under \$EV/\$AUD no longer block (ported: the gate now compares canonical \$EVR/\$AUDR)" '"$EVR"/*|"$AUDR"/*)' '"/nonexistent-ev"/*)'
M M16 require_verdicts.sh "fingerprint not compared (ported: the check is keyed on have_fp)" 'if [ "$have_fp" = 1 ] &&' 'if false &&'
M M17 acquire.sh "adopt does not check it adopts a suspended-run of that run (no CAS) (ported to the _hget holder variable)" '[ "$(jq -r .run_id <<<"$H")" = "$run" ] && [ "$(jq -r .kind <<<"$H")" = suspended-run ] || { echo "cas_mismatch: no suspended-run holder for $run" >&2; return "$RC_CAS"; }' ':'
M M18 lib.sh "stale-holder reader does not re-read the record (snapshot race) (ported: the re-read is s3)" '[ "$s3" = "$s2" ] &&' 'true &&'
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
M M26 reap.sh "F1 (ported): a process that survived TERM is still recorded reaped" '    dead_owner) if [ "$mode" = hung ]; then _reap_write "$j" hung' '    dead_owner|advancing|hung) if [ "$mode" = hung ]; then _reap_write "$j" hung'
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
M M36 lib.sh "class 2 (ported): a record that fails the shape is classified dead_owner" '[ -n "$fl" ] || { echo unreadable;' '[ -n "$fl" ] || { echo dead_owner;'
M M37 lib.sh "F2 member: the op record is created with an overwriting rename (not exclusive)" 'ln -- "$tmp" "$f" 2>/dev/null; local r=$?' 'mv -f "$tmp" "$f" 2>/dev/null; local r=$?'
M M38 lib.sh "class 3 (ported): lo_uint accepts any number of digits and leading zeros" '^(0|[1-9][0-9]{0,14})$ ]]; }' '^[0-9]+$ ]]; }'
EQ M39 "round 5 (found by the negative control, the first run caught it only at H1b): check_no_build_writing_tracked reads records through lo_ops_snapshot, whose \`bad\` rows are exactly the records the shape rejects (that guard is M39b), and lo_classify_op applies the SAME shape, so cls=unreadable cannot occur for an \`ok\` row; the branch is defence in depth. Re-validated: with the branch disabled the whole registry suites stay green" check_no_build_writing_tracked.sh "class 2: an unreadable op record is read as no writer (the cls=unreadable defence-in-depth branch)"
M M39b check_no_build_writing_tracked.sh "class 2 (the live guard): a record that fails the shape (kind=bad) is read as no writer" 'if [ "$kind" = bad ]; then printf '"'"'blocked' 'if false; then printf '"'"'blocked'
M M40 holder.sh "class 2: an unreadable holder is reported none" '  unreadable) lo_die holder_unreadable' '  unreadable_x) lo_die holder_unreadable'
M M41 classify.sh "F14: classify --op-id of an unknown op succeeds with empty output" '[ -e "$(lo_op_file "$only")" ] || lo_die unknown_op' 'true || lo_die unknown_op'
M M42 reap.sh "label: reap does not look at the label the launcher sets (catalogizer.op_id)" 'cands=("op_id=$opid" "catalogizer.op_id=$opid")' 'cands=("op_id=$opid")'
M M43 lib.sh "RM5 (ported): the last_progress_epoch is not validated by the record shape: a valid-JSON record with a text epoch reads as readable" 'and (.last_progress_epoch|u) and (.last_progress_mono|u)' 'and (.last_progress_mono|u)'
EQ M44 "round 5 (found by the negative control): LO_HOLDER_SHAPE already requires a process holder's pid to be a number > 1 and < 1e15, so after the shape check lo_uint cannot fail; the line is defence in depth (the live guards are the shape twin and this line together: M44c)" lib.sh "RM11 (round-2 reviewer, verbatim): a holder whose pid is not a number is judged by lo_alive instead of being unreadable (the lo_uint defence-in-depth line)"
M M44c lib.sh "RM11 (both guards, ported): neither the holder shape nor the lo_uint line requires a process holder's pid to be a number: a holder whose pid is not a number is judged dead by lo_alive instead of being unreadable (M44 and its shape twin are each redundant; removing BOTH is the observable mutant)" '(.run_id|type=="string" and length>0) and (.pid|type=="number" and .==floor and .>1 and .<1e15)' '(.run_id|type=="string" and length>0)' 'lo_uint "$pid" || { echo unreadable; return 0; }' ':'
M M45 reap.sh "WF14 R2-1 (ported): the grace wait runs INSIDE the purpose lock (an owner whose exit path takes the lock cannot finish: the dispatch pump shape)" 'printf '"'"'hung %s %s\n'"'"' "$pid" "$pst" >"$SF"; return 0 ;;' 'printf '"'"'hung %s %s\n'"'"' "$pid" "$pst" >"$SF"; n=0; while [ "$n" -lt $((GRACE * 5)) ]; do lo_alive "$pid" "$pst" || break; sleep 0.2; n=$((n+1)); done; return 0 ;;'
M M46 heartbeat.sh "WF14 R2-3: a rebind does not move the claim holder with the op owner" 'if [ "$(jq -r '"'"'.run_id // ""'"'"' <<<"$h" 2>/dev/null)" = "$(jq -r .run_id <<<"$j")" ]; then' 'if false; then'
M M47 reap.sh "WF14 R2-3: reap --purpose ignores a live op of the purpose" 'if lo=$(_live_op_of_purpose); then' 'if false; then'
M M48 lib.sh "WF14 R2-5: lo_pid_ok accepts a zombie" '[ "$(lo_pstate "$1")" != Z ] && ' ''
M M49 lib.sh "WF14 R2-5: lo_pid_ok accepts a kernel thread (process group 0)" ' && g=$(lo_ppgrp "$1") && [[ "$g" =~ ^[0-9]+$ && "$g" -gt 1 ]]; }' '; }'
M M50 lib.sh "WF14 R2-6: the default budget is not validated (0 or junk brings back never-hung)" 'lo_uint "$LO_DEFAULT_NP" && [ "$LO_DEFAULT_NP" -gt 0 ] ||' 'true ||'
M M51 release.sh "WF14 R2-10: release reports a claim that belongs to another run as a failed CAS after the record was written" 'lo_unclaim_own "$purpose" "$runid"' 'lo_unclaim "$purpose" "$runid"'
M M52 reap.sh "WF14 R2-10 (ported): reap of a --no-claim op fails the CAS after the record was written" 'lo_unclaim_own "$purpose" "$own"' 'lo_unclaim "$purpose" "$own"'
M M53 reap.sh "WF14 R2-4: the record's own container_label is not used to find the container" 'case "$clab" in "$opid"|"") ;; *=*) cands+=("$clab") ;; *) cands+=("catalogizer.op_id=$clab" "op_id=$clab") ;; esac' ':'
M M54 lib.sh "WF14 class D (ported): a record whose pid is not a number > 1 passes the shape (pid 1 is classified by lo_alive as dead_owner, not unreadable)" $'      (.pid|type=="number" and .==floor and .>1 and .<1e15)\n      and (.start_time' $'      (.pid|type=="number")\n      and (.start_time'
M M55 lib.sh "WF14: lo_unclaim_own releases a claim that belongs to another run" 'if [ "$cur" = "$run" ]; then rm -rf' 'if true; then rm -rf'
M M56 lib.sh "WF14: the lock wait budget is ignored (fixed 15 s)" '( flock -w "$LO_LOCK_WAIT" 9 || exit 70; "$@" ) 9>' '( flock -w 15 9 || exit 70; "$@" ) 9>'
M M57 lib.sh "WF14 class D (ported): a process holder whose start_time cannot identify a process passes the holder shape" $'    and (.start_time|type=="string" and test("^(0|[1-9][0-9]*)$")) and (.boot_id|type=="string" and length>0)' $'    and (.boot_id|type=="string" and length>0)'
M M58 lib.sh "WF14 class D (ported): a suspended-run holder with a mistyped builds passes the holder shape" '(.run_id|type=="string" and length>0) and (.builds|type=="array" and all(.[]; type=="string"))' '(.run_id|type=="string" and length>0)'
NC CTRL0 lib.sh "negative control: an unmutated copy of lib.sh placed like a mutant" 'export LC_ALL=C'
# ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# 11.4.276 ROUND 5 - (a) the round-3 REVIEWERS' mutants, adopted verbatim (WF17 X1-X5; CONS C01-C07, C11, C12), the ones whose target text the fix rewrote PORTED with the original intent unchanged (C02, C03, C04, C05, C06):
# their pre-fix run on 350372a8 (all SURVIVED) is their RED; (b) one mutant per guard the fix round adds (NM-*), each paired with the test section that kills it.
# ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
M X1 reap.sh "WF17: reap --purpose treats only an ADVANCING op as live (a HUNG op whose owner lives no longer protects its claim)" 'case "$c" in advancing|hung) echo "live:' 'case "$c" in advancing) echo "live:'
M X2 heartbeat.sh "WF17: a rebind rewrites the purpose holder even when the claim belongs to ANOTHER run" 'if [ "$(jq -r '"'"'.run_id // ""'"'"' <<<"$h" 2>/dev/null)" = "$(jq -r .run_id <<<"$j")" ]; then' 'if true; then'
M X3 reap.sh "WF17: the pre-signal start-time re-check (R2-9) is dropped" '[ "$(lo_pstart "$pid")" = "$pst" ] || { echo "refused: pid $pid is no longer' 'true || { echo "refused: pid $pid is no longer'
M X4 reap.sh "WF17: the DEFAULT reap grace is 0 s (an owner whose exit path takes ~10 s, the runner wrapper, is falsely reported reap_survived)" 'GRACE=${LONGOPS_REAP_GRACE_S:-15}' 'GRACE=${LONGOPS_REAP_GRACE_S:-0}'
EQ X5 "WF17 (verbatim reason): step B re-derives the state from the record, so waiting for the process only is a timing-only difference; the final record and exit are identical" reap.sh "WF17: the unlocked wait ignores an op the owner already released (waits for the process only)"
M C01 reap.sh "R2-4 reap: a BARE container_label value (not k=v, not the op id) adds no candidate labels" '*) cands+=("catalogizer.op_id=$clab" "op_id=$clab") ;; esac' '*) ;; esac'
M C02 reap.sh "R2-8 reap (ported): the container listing runs with no timeout (a wedged podman wedges reap) [orig: cid=\$(timeout 30 \"\$PODMAN\" ps --filter]" 'ids=$(timeout "$LO_PODMAN_TIMEOUT" "$PODMAN" ps --filter' 'ids=$("$PODMAN" ps --filter'
M C03 reap.sh "R2-8 reap (ported): the container STOP moves back UNDER the purpose lock (the unlocked stop is removed and a stop is added before the signal in step A)" $'if _clist; then _cstop; else CUNREAD=1; fi\nWAITED=$GRACE' $'WAITED=$GRACE' $'      lo_signal TERM "$pid"; rc=$?' $'      _clist && _cstop\n      lo_signal TERM "$pid"; rc=$?'
M C04 reap.sh "R2-3 guard (ported) ignores the purpose: a live op of ANOTHER purpose blocks the release of a stale claim (false-positive refusal)" $'      [ "$pk" = "$purpose" ] || continue\n      case "$state" in registered|running) ;; *) continue ;; esac\n      c=$(lo_classify_op' $'      :\n      case "$state" in registered|running) ;; *) continue ;; esac\n      c=$(lo_classify_op'
M C05 heartbeat.sh "R2-3 rebind (ported): a FAILED holder write is ignored and the op write still lands (op and holder diverge, exit 0)" '|| { echo "op $opid: the op owner was rebound but the purpose holder could not be rewritten (holder_write_failed); retry the rebind" >&2; return 1; }' '|| :'
M C06 reap.sh "reap --op-id --dry-run of a dead_owner op (ported text) really writes reaped and releases the claim" $'[ "$dry" = 1 ] && { echo "would reap dead $opid ($ev) container=\'${DRYC:-none}\'"; return 0; }' ':'
M C07 reap.sh "reap --purpose --dry-run really releases the claim" '[ "$dry" = 1 ] && { echo "would release stale claim $purpose ($s)"; return 0; }' ':'
M C11 lib.sh "classify: blocked-escape is no longer terminal (a reap/sweep may rewrite a terminal record)" 'case "$st" in complete|failed|reaped|handoff|blocked-escape) echo terminal; return 0 ;; esac' 'case "$st" in complete|failed|reaped|handoff) echo terminal; return 0 ;; esac'
M C12 reap.sh "reap: a REFUSED (unsafe) signal is treated as delivered: waits and records reap_survived instead of exit 7" '[ "$rc" -ne "$RC_UNSAFE" ] || { echo "refused: unsafe signal target pid=$pid (rc $rc)" >&2; return "$RC_UNSAFE"; }' ':'
# --- class A: ONE shape, canonical integers, absent is not unreadable ---
MR A1 NM-A1 lib.sh "A: lo_classify_op judges any JSON object (the record shape is not applied)" $'if (\'"$LO_OP_SHAPE"\') then (if (.state|IN("registered","running"))' $'if (type=="object") then (if (.state|IN("registered","running"))'
MR A1 NM-A2 lib.sh "A: the start_time owner sub-predicate is dropped from LO_OP_SHAPE" $'      and (.start_time|type=="string" and test("^(0|[1-9][0-9]*)$"))\n      and (.boot_id' $'      and (.boot_id'
MR X NM-A3 lib.sh "A: one state (blocked-escape) is dropped from the closed state set of LO_OP_SHAPE" 'IN("registered","running","complete","failed","reaped","handoff","blocked-escape"))' 'IN("registered","running","complete","failed","reaped","handoff"))'
MR A1 NM-A4 reap.sh "A: _live_op_of_purpose skips an unreadable record instead of refusing (purpose filter before the shape check)" 'if [ "$kind" = bad ]; then echo "unreadable:$file"; return 0; fi' 'if [ "$kind" = bad ]; then continue; fi'
MR A NM-A5 lib.sh "A: a suspended-run holder with an absent/null builds is read as an empty array (builds back to // [])" $'(.builds|type=="array" and all(.[]; type=="string"))\n    and (.state' $'((.builds // [])|type=="array" and all(.[]; type=="string"))\n    and (.state'
MR A NM-A6 lib.sh "A: a holder file that exists but cannot be read (empty, mode 000, a directory) is read as ABSENT" '[ -f "$hf" ] || return 2' '[ -f "$hf" ] || return 1' '[ -n "$s" ] || return 2' '[ -n "$s" ] || return 1'
MR A NM-A7 lib.sh "A: lo_uint accepts a leading zero (bash reads 08 as an invalid octal and 010 as 8)" '^(0|[1-9][0-9]{0,14})$ ]]; }' '^[0-9]{1,15}$ ]]; }'
MR A NM-A9a require_verdicts.sh "A: a verdict utc is passed raw to date -d ('', now and next year read as a time)" $'if [[ "$u" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]; then t=$(date -u -d "$u" +%s 2>/dev/null) || t=""; fi' $'t=$(date -u -d "$u" +%s 2>/dev/null) || t=""'
MR A NM-A9b require_verdicts.sh "A: an explicitly empty --fingerprint means no check" '[ "$have_fp" = 0 ] || [ -n "$fp" ] || lo_die' 'true || lo_die'
MR A NM-A9c require_verdicts.sh "A: a verdict dated in the future never expires" 'if [ "$t" -gt $(( $(lo_now) + SKEW )) ]; then' 'if false; then'
MR A1 NM-A12 lib.sh "A: run_id is no longer required of an op record" $'and (.run_id|type=="string" and length>0)\nand (.container_label' $'and (.container_label'
# --- class C: --expect ---
MR D NM-C5 reap.sh "C: --expect is ignored (a dead-owner action may reap a hung op: TERM to a live owner)" '[ -z "$expect" ] || [ "$cls" = "$expect" ] ||' 'true ||'
# --- class D: producer model ---
MR D NM-D3a reap.sh "D: the reap intent is not written before the signal" $'\'.reap_requested_utc=$u|.reap_requested_by=$by\'' $'\'.\''
MR D NM-D3b reap.sh "D: step B no longer converts an owner handoff that answers the reap intent into reaped" $'if [ "$mode" = hung ] && [ "$st" = handoff ] && [ -n "$(jq -r \'.reap_requested_utc // empty\' <<<"$j")" ]; then' 'if false; then'
MR D NM-D4 reap.sh "D: budget.stop_grace_s is ignored by the reap wait" 'if [ -n "$sgv" ] && lo_uint "$sgv" && [ "$sgv" -gt 0 ] && [ $((sgv + 10)) -gt "$WAITED" ]; then WAITED=$((sgv + 10)); fi' ':'
MR D NM-D5a lib.sh "D: the boot id of an op record is not compared (a record of another boot judged by a pid that now belongs to another process)" '[ "$bid" = "$(lo_boot_id)" ] || { echo dead_owner;' 'true || { echo dead_owner;'
MR D NM-D5b lib.sh "D: the boot id of a process holder is not compared" '[ "$bid" = "$(lo_boot_id)" ] || { echo dead; return 0; }' 'true || { echo dead; return 0; }'
MR D NM-D5c lib.sh "D: liveness is judged on the wall clock (LONGOPS_MONO ignored)" 'if [ -n "${LONGOPS_MONO:-}" ]; then echo "$LONGOPS_MONO"; elif' 'if false; then echo "$LONGOPS_MONO"; elif'
M NM-D5d heartbeat.sh "D: a heartbeat does not move last_progress_mono (the judged clock)" '(if $prog==1 then .last_progress_epoch=$now | .last_progress_mono=$mono else . end)' '(if $prog==1 then .last_progress_epoch=$now else . end)'
# --- class E: write paths of every op ---
MR D NM-E3 check_no_build_writing_tracked.sh "E: the tracked-path rule applies to build:* ops only again" $'if [ -n "$(git -C "$ROOT" ls-files -- "$rel" 2>/dev/null | head -1)" ]; then' $'if [ "${purpose#build:}" != "$purpose" ] && [ -n "$(git -C "$ROOT" ls-files -- "$rel" 2>/dev/null | head -1)" ]; then'
MR D NM-E3b check_no_build_writing_tracked.sh "E/D3: a dead-owner row is not a writer again (it blocks until reaped)" 'case "$cls" in terminal) continue ;; esac' 'case "$cls" in terminal|dead_owner) continue ;; esac'
# --- class F: owner gone is not workload gone ---
MR F NM-F1 reap.sh "F: the container list after the stop is ignored (CSURV forced 0)" 'if [ "$CUNREAD" = 0 ]; then if _clist; then CSURV=${#CIDS[@]}; else CUNREAD=1; fi; fi' 'CSURV=0'
MR F NM-F2 reap.sh "F: a dead_owner op skips the container resolution and stop" $'if _clist; then _cstop; else CUNREAD=1; fi\nWAITED=$GRACE' $'if [ "$mode" = hung ]; then if _clist; then _cstop; else CUNREAD=1; fi; fi\nWAITED=$GRACE'
MR F NM-F3 reap.sh "F: only ONE container per label is listed (head -1)" $'--format \'{{.ID}}\' 2>/dev/null) || return 1' $'--format \'{{.ID}}\' 2>/dev/null | head -1) || return 1'
MR F NM-F4a reap.sh "F: the own-run holder is not judged before a dead_owner reap" '_own_holder "$own" || return $?' ':'
MR F NM-F4b heartbeat.sh "F: the holder is written BEFORE the op again (a failed holder write leaves the op unchanged)" $'  lo_wjson "$f" "$j" || return 1\n  if [ -n "$pid" ]; then' $'  if [ -n "$pid" ]; then' $'  fi\n}\nlo_with_lock "$purpose" _hb; rc=$?' $'  fi\n  lo_wjson "$f" "$j"\n}\nlo_with_lock "$purpose" _hb; rc=$?'
# --- class G: transitions with several writes ---
MR G NM-G1a register.sh "G: no rollback trap across the record -> claim window of register.sh" $'trap \'_reg_fail interrupted; exit 143\' TERM INT HUP' ':'
MR G NM-G1b release.sh "G: release.sh does not hold a TERM until record and unclaim are done" "trap 'RELSIG=1' TERM INT HUP" ':'
MR G NM-G2 reap.sh "G: reap --purpose does not reap the dead-owner ops of the purpose first" '    [ "$kind" = ok ] || continue' '    continue'
MR G NM-G3 lib.sh "G: directory entries are not flushed (lo_fsync_dir is a no-op)" 'lo_fsync_dir() { sync -- "$1" 2>/dev/null; return 0; }' 'lo_fsync_dir() { return 0; }'
# --- class H: identity ---
EQ NM-H1 "UNPROVEN equivalence, stated as such (11.4.6): after round 5 the readers are pure bash (no sed/cut/awk over /proc), so no input of test H1 (three comm shapes, run under LANG=en_US.UTF-8) distinguishes the export from its absence; the line is kept as defence in depth. No distinguishing input was found, none is claimed to not exist" lib.sh "H: the C locale is not forced (LC_ALL no longer set by the library)"
# --- class J: scale ---
MR J NM-J1 classify.sh "J: every terminal record is classified individually again (a jq per record)" $'case "$state" in registered|running) ;; *) printf \'%s\\tterminal\\t\\n\' "$oid"; continue ;; esac' ':'
# --- class K: a flag given as the last token ---
MR K NM-K1 register.sh "K: --purpose lost its lo_need guard (a last-token --purpose spins forever)" '--purpose) lo_need "$@"; purpose=$2; shift 2 ;;' '--purpose) purpose=${2:-}; shift 2 ;;'
MR K NM-K1b lib.sh "K: lo_need is a no-op for every script" 'lo_need() { [ "$#" -ge 2 ] || lo_die usage_error "$1 requires a value"; }' 'lo_need() { :; }'
echo "MUTATION RESULT caught=$caught survived=$surv timeout=$tmo equivalent=$eqv total=$tot"
[ "$surv" -eq 0 ] && [ "$tmo" -eq 0 ]
