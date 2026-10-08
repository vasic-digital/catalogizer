#!/usr/bin/env bash
# mutate_sweep.sh - paired mutations (G-GATE, 11.4.115 F / 1.1) of scripts/anti-mess/sweep.sh (and, for the lineage classes, of scripts/longops/lib.sh): each mutation breaks ONE load-bearing line in a COPY; the sweep suites
# run against that copy (env SWEEP) must FAIL. A mutant that leaves every suite green survives and fails this runner. The real scripts are never edited.
# SAFETY (WF11 F15, 11.4.263): ms_scan aborts a mutant tree carrying a signal/host-power line that is not byte-identical to the pristine tree, and every suite runs inside the mutation_safety.sh containment
# (BASH_ENV shim: kill builtin disabled, guard function, PATH stubs); the same library as mutate_registry.sh, tested by test_mutation_safety.sh.
# The copy sits in a scratch tree whose scripts/repo is a symlink to the real one; scripts/longops is a symlink too, except for `ML` mutants which get a COPY of the longops scripts.
# SUITES (11.4.276 round 5): `M` runs test_sweep.sh first and, only if the mutant survived it, test_sweep_r5.sh; `MN <sections>` runs the named sections of test_sweep_r5.sh first (S5_SECTIONS), then test_sweep.sh; `ML` is `MN` for a longops/lib.sh
# mutation. Every suite run is bounded by `timeout ${MUT_TIMEOUT_S:-900}`; a timeout is its own result and fails the runner. `EQ` records an EQUIVALENT mutant with its reason. Suites stop at their first failure (MUT_FAILFAST).
# Usage  mutate_sweep.sh [--only S01,S05] [--check]     Output: one line per mutation, then `MUTATION RESULT caught=N survived=M timeout=T equivalent=E total=T`
#        --check applies every mutation and the safety scan but runs no suite (`APPLIES`/`INVALID`: a pattern that is not found is never counted as caught)
. "$(dirname "$0")/../../longops/tests/lib.sh"
. "$(dirname "$0")/../../longops/tests/mutation_safety.sh"
[ "$(id -u)" != 0 ] || { echo "refusing to run mutations as root"; exit 2; }
ms_prepare "$FX/shim" || { echo "SAFETY: the mutant containment cannot be built; no mutant is run"; exit 2; }
ONLY=""; CHECK=${MUT_CHECK_ONLY:-0}
while [ $# -gt 0 ]; do case "$1" in --only) ONLY=,${2:-},; shift 2 ;; --check) CHECK=1; shift ;; *) echo "unknown argument $1"; exit 2 ;; esac; done
TMO=${MUT_TIMEOUT_S:-900}
caught=0; surv=0; tot=0; tmo=0; eqv=0
_mk() {  # _mk <target: sweep|lib> <id> <desc> <old> <new> [<old2> <new2>]: build the mutant tree $MD; 0 ok, 1 invalid or safety abort (counted as survived)
  local tgt=$1 id=$2 desc=$3 old=$4 new=$5 old2=${6:-} new2=${7:-} dst src
  MD=$FX/mut-$id; rm -rf "$MD"; mkdir -p "$MD/scripts/anti-mess"
  cp "$TROOT/scripts/anti-mess/catalogue.yaml" "$TROOT/scripts/anti-mess/sweep.sh" "$MD/scripts/anti-mess/"; ln -s "$TROOT/scripts/repo" "$MD/scripts/repo"
  if [ "$tgt" = lib ]; then mkdir -p "$MD/scripts/longops"; cp "$TROOT"/scripts/longops/*.sh "$MD/scripts/longops/"; dst=$MD/scripts/longops/lib.sh; else ln -s "$TROOT/scripts/longops" "$MD/scripts/longops"; dst=$MD/scripts/anti-mess/sweep.sh; fi
  python3 - "$dst" "$old" "$new" "$old2" "$new2" <<'PY' || { echo "INVALID $id $desc: pattern not found"; surv=$((surv+1)); return 1; }
import sys
p,old,new,old2,new2=sys.argv[1:6]; s=open(p).read()
if old not in s: sys.exit(1)
s=s.replace(old,new,1)
if old2:
    if old2 not in s: sys.exit(1)
    s=s.replace(old2,new2,1)
open(p,'w').write(s)
PY
  if ! ms_scan "$TROOT/scripts/anti-mess" "$MD/scripts/anti-mess" >"$FX/scan-$id.out" 2>&1 || { [ "$tgt" = lib ] && ! ms_scan "$TROOT/scripts/longops" "$MD/scripts/longops" >>"$FX/scan-$id.out" 2>&1; }; then
    echo "SAFETY-ABORT $id: $(head -1 "$FX/scan-$id.out" | cut -c1-200); not run"; surv=$((surv+1)); return 1; fi
  return 0
}
_suite() {  # _suite <id> <test-file> [ENV=VAL...]: one suite against $MD inside the containment, bounded in time; sets SUITE_RC (124 = timeout)
  local id=$1 tf=$2; shift 2
  env "$@" MUT_FAILFAST=1 SWEEP=$MD/scripts/anti-mess/sweep.sh LONGOPS_SCRIPTS=$MD/scripts/longops BASH_ENV=$MS_SHIM PATH=$MS_BIN:$PATH MS_LOG=$MS_LOG MS_REAL_KILL=${MS_REAL_KILL:-/usr/bin/kill} timeout "$TMO" bash "$(dirname "$0")/$tf" >"$FX/mut-$id.$tf.out" 2>&1; SUITE_RC=$?
}
_verdict() {  # _verdict <id> <desc> <rc> <suite-file>
  local id=$1 desc=$2 rc=$3 sn=$4
  if [ "$rc" -eq 124 ]; then tmo=$((tmo+1)); echo "TIMEOUT  $id $desc (suite $sn exceeded ${TMO}s: never counted as caught)"
  elif [ "$rc" -ne 0 ]; then caught=$((caught+1)); echo "CAUGHT   $id $desc [$sn] :: $(grep -m2 '^FAIL' "$FX/mut-$id.$sn.out" | cut -c1-110 | tr '\n' '|')"
  else surv=$((surv+1)); echo "SURVIVED $id $desc"; fi
}
_run_old_then_r5() { _suite "$1" test_sweep.sh; if [ "$SUITE_RC" -eq 0 ]; then _suite "$1" test_sweep_r5.sh; _verdict "$1" "$2" "$SUITE_RC" test_sweep_r5.sh; else _verdict "$1" "$2" "$SUITE_RC" test_sweep.sh; fi; }
M() {  # M <id> <description> <old> <new> [<old2> <new2>]: test_sweep.sh first, then test_sweep_r5.sh
  local id=$1; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); _mk sweep "$@" || return 0
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return 0; }
  _run_old_then_r5 "$id" "$2"
}
MN() {  # MN <sections> <id> <description> <old> <new> [<old2> <new2>]: the named sections of test_sweep_r5.sh first (S5_SECTIONS), then test_sweep.sh
  local sec=$1; shift; local id=$1; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); _mk sweep "$@" || return 0
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return 0; }
  _suite "$id" test_sweep_r5.sh S5_SECTIONS="$sec"; if [ "$SUITE_RC" -eq 0 ]; then _suite "$id" test_sweep.sh; _verdict "$id" "$2" "$SUITE_RC" test_sweep.sh; else _verdict "$id" "$2" "$SUITE_RC" test_sweep_r5.sh; fi
}
ML() {  # ML <sections> <id> <description> <old> <new> [<old2> <new2>]: like MN, the mutation is in scripts/longops/lib.sh (the lineage / classification the sweep consumes)
  local sec=$1; shift; local id=$1; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); _mk lib "$@" || return 0
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return 0; }
  _suite "$id" test_sweep_r5.sh S5_SECTIONS="$sec"; if [ "$SUITE_RC" -eq 0 ]; then _suite "$id" test_sweep.sh; _verdict "$id" "$2" "$SUITE_RC" test_sweep.sh; else _verdict "$id" "$2" "$SUITE_RC" test_sweep_r5.sh; fi
}
NC() {  # NC <id> <description> <text>: the NEGATIVE CONTROL: an UNMUTATED copy (the text replaced by itself) goes through the identical pipeline and MUST PASS both suites (11.4.201 (7)(b))
  local id=$1 desc=$2 text=$3; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); _mk sweep "$id" "$desc" "$text" "$text" || return 0
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return 0; }
  _suite "$id" test_sweep.sh; local r1=$SUITE_RC r2=0
  if [ "$r1" -eq 0 ]; then _suite "$id" test_sweep_r5.sh; r2=$SUITE_RC; fi
  if [ "$r1" -eq 0 ] && [ "$r2" -eq 0 ]; then echo "CONTROL-OK $id $desc (both suites PASSED on the unmutated copy: a CAUGHT elsewhere is a real kill)"; else surv=$((surv+1)); echo "CONTROL-BROKEN $id $desc (suite rc $r1/$r2): the pipeline fails an UNMUTATED tree, every CAUGHT is void"; fi
}
EQ() {  # EQ <id> <reason> <description>: a mutant proven EQUIVALENT is recorded with its reason, never run and never silently dropped
  local id=$1 reason=$2; [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); eqv=$((eqv+1)); echo "EQUIVALENT $id $3 :: $reason"
}
M S01 "the heartbeat check is disabled in AM-P1 (a hung op is never reported)" '      hung) emit drift hung_op' '      hung_x) emit drift hung_op'
M S02 "INV-9 (ported): the finished-run removal skips the remote check (a held commit no remote holds)" 'if [ ! -e "$d/commits.tsv" ] || _held_ok "$d/commits.tsv"; then' 'if true; then'
M S03 "AM-R1 at S0 treats every path as declared" 'grep -qxF -- "$rel" <<<"$decl" ||' 'true ||'
M S04 "AM-R1 ignores the reviewed exceptions.tsv" '--exceptions "${AM_EXC:-$TOOLS/scripts/repo/exceptions.tsv}"' '--exceptions /dev/null'
M S05 "a catalogued invariant with no detector is reported clean" 'status=not_evaluated; reason=${RSN[$id]}' 'status=clean; reason=${RSN[$id]}'
M S06 "AM-R2 ignores whether a process holds the lock open" 'if h=$(_open_by_any "$lk"); then' 'if false; then'
M S07 "INV-9 reconcile removes an interrupted run (the finished-run re-verification is bypassed too)" 'never removed by the sweep"; fi' 'never removed by the sweep" "rmdir:finished_run:$d"; fi' '          [ -s "$rep" ] || { echo skipped_precondition_changed; return; }' '          :'
M S08 "a pending pin move is reported as drift" 'emit info pending_pin_move' 'emit drift pending_pin_move'
M S09 "a blocking core.hooksPath is not refused at S0" '&& [ "$STAGE" = S0 ] && REFUSE=1' '&& [ "$STAGE" = S0_x ] && REFUSE=1'
M S10 "INV-9: a live merge holder is reported as interrupted" 'if lo_alive "$mpid" "$mst"; then emit info live_merge_holder' 'if false; then emit info live_merge_holder'
M S11 "INV-9: a suspended run with a live holder is reported interrupted" 'if [ "$hrun" = "$id" ] && [ "$hstat" = live ]; then emit info suspended_run' 'if false; then emit info suspended_run'
M S12 "AM-P3: the temp dir of a live process is reaped" 'if lo_alive "$pid" "$st"; then emit info build_tmp_live' 'if false; then emit info build_tmp_live'
M S13 "AM-P1: the orphan-container age budget is ignored" 'if [ "$age" -gt "${ANTIMESS_ORPHAN_AGE_S:-300}" ]; then echo orphan; else echo young; fi' 'if true; then echo orphan; else echo young; fi'
M S14 "AM-P2 (ported): an attached op still counts as a duplicate owner" '[ $nt[] | select(attached | not) ]' '[ $nt[] ]'
M S15 "a failing control needle no longer makes the detector blind" 'else needle=blind; status=blind; reason="control needle failed: $out"; fi' 'else needle=blind; fi'
M S16 "AM-R2 ignores the minimum lock age" 'elif [ "$age" -lt "$minage" ]; then emit info lock_young' 'elif false; then emit info lock_young'
M S17 "RMS1 (ported): the rmlock reconcile no longer re-verifies its precondition" 'if [ -e "$p" ] && ! _open_by_any "$p" >/dev/null && [ "$(_git_active)" -eq 0 ] && [ "$age" -ge "${ANTIMESS_LOCK_MIN_AGE:-60}" ]; then rm -f' 'if true; then rm -f'
M S18 "F4 (ported to _unread_op_record): a corrupt op record is reported as info, not unread" '_unread_op_record() { emit unread corrupt_op_record' '_unread_op_record() { emit info corrupt_op_record'
M S19 "F8: an unreadable podman is informational (AM-P1 clean)" 'emit unread containers_unread podman' 'emit info containers_unread podman'
M S20 "F9: a stale claim is informational" 'dead) emit drift stale_claim' 'dead) emit info stale_claim'
M S21 "F9: an un-adopted handoff is informational" 'emit drift handoff_unadopted' 'emit info handoff_unadopted'
M S22 "F10: the container of a handoff op is stopped by --reconcile" '      handoff) emit info container_of_handoff_op "$cid" "op $op is handed off (re-adoptable, 11.4.232 D): its container is never stopped by the sweep" ;;' '      handoff) emit drift container_of_terminal_op "$cid" "x" "stopcontainer:$cid:handoff" ;;'
M S23 "F10: stopcontainer does not re-verify" 'if [ "$now_class" != "$want" ]; then echo' 'if false; then echo'
M S24 "F12: an unknown --only id is accepted" '[ "$_k" = 1 ] || die usage' '[ "$_k" = 1 ] || true'
M S25 "label (ported): the catalogizer.op_id label of the launcher is ignored" 'lab("catalogizer.op_id"),lab("op_id")' 'lab("op_id"),lab("op_id")'
M S26 "F10 (ported): the finished-run removal does not re-check the held commits" 'if [ -e "$pc/commits.tsv" ] && ! _held_ok "$pc/commits.tsv"; then echo skipped_held_commit_not_on_remote; return; fi' ':'
M S27 "F10: initsub does not re-verify" "if git -C \"\$AM_ROOT\" submodule status --recursive -- \"\$arg\" 2>/dev/null | grep -q '^-'; then" 'if true; then'
M S28 "F8: a sweep with only unread sources exits 0" '[ "$UNREAD" -gt 0 ] && exit 11' '[ "$UNREAD" -gt 0 ] && exit 0'
M S29 "F4 (ported): AM-P2 never reports a corrupt op record" $'    if [ "$kind" = bad ]; then _unread_op_record "$file"; continue; fi\n    recs+="$json"' $'    if [ "$kind" = bad ]; then continue; fi\n    recs+="$json"'
M S30 "AM-P4: a claim with no holder record is never reported" 'emit drift claim_without_holder' 'emit info claim_without_holder'
M S31 "AM-P4: an unreadable claim is never reported" 'unreadable) emit drift claim_unreadable' 'unreadable) emit info claim_unreadable'
M S32 "AM-P4: a holder that cannot be judged is informational" 'emit unread claim_holder_unread' 'emit info claim_holder_unread'
ML "" S33 "RM6 (ported, round-2 reviewer): a valid-JSON record with no state passes the record shape: it is skipped silently instead of reported unreadable" 'and (.state|type=="string" and IN("registered","running","complete","failed","reaped","handoff","blocked-escape"))' 'and ((.state // "registered")|type=="string")'
M S34 "RM9 (ported, round-2 reviewer): an unreadable lineage class is treated as terminal (its container is stopped)" 'case "$c" in live) any_live=1 ;; unreadable) any_unread=1 ;;' 'case "$c" in live) any_live=1 ;; unreadable) any_term=1 ;;'
M S35 "WF14 R2-11: an orphan container (no row in THIS registry) is stopped by --reconcile again" 'stop it by hand once its owner is known" ;;' 'stop it by hand once its owner is known" "stopcontainer:$cid:orphan" ;;'
ML "" S36 "WF14 R2-4 (ported): a container is matched to its op by the op id only (the record container_label is ignored)" '([.op_id, lab] | map(select(length>0))' '([.op_id] | map(select(length>0))'
M S37 "WF14 R2-2 (ported): a handoff op re-adopted by the next attempt of its build is still reported un-adopted" $'    | select((($x|base) + "-a" + ((($x|attempt) + 1)|tostring)) as $nx | ($all | any(.[]; .op_id==$nx and .purpose_key==$x.purpose_key and .started_utc >= $x.started_utc)) | not)\n' ''
M S38 "WF14 R2-7: INV-9 asserts a missing live holder it could not read" 'elif [ -n "$hunread" ]; then emit unread suspended_run_holder_unread' 'elif false; then emit unread suspended_run_holder_unread'
ML "" S39 "WF14 class D (ported): a record with no op_id or purpose_key is accepted as readable" 'and (.op_id|sname) and (.purpose_key|sname)' 'and true'
M S40 "WF14 (ported): a handoff op is adopted by ANY op of the purpose, also an earlier one" ' and .started_utc >= $x.started_utc' ''
NC CTRL0 "negative control: an unmutated copy of sweep.sh placed like a mutant" 'set -u'
# ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# 11.4.276 ROUND 5 - the round-3 reviewers' mutants adopted (WF17 Y1-Y3; CONS C08-C10; Y1, Y3, C10 PORTED because the lineage predicate moved into lib.sh), each with the test that kills it, and one NM-* mutant per guard
# the round adds. The pre-fix run on 350372a8 (all SURVIVED) is their RED.
# ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
ML "" Y1 "WF17 (ported to lo_lineage_build): a record container_label of the form op_id=<value> is no longer matched" 'elif startswith("op_id=") then .[6:] else . end' 'else . end'
M Y2 "WF17: a handoff op is adopted only by an op that started STRICTLY later (a same-second successor is not counted)" '.started_utc >= $x.started_utc' '.started_utc > $x.started_utc'
ML "" Y3 "WF17 (ported): when several attempts exist, the LEXICALLY greatest op id decides instead of the numerically latest attempt (-a10 sorts before -a2)" '(max_by(attempt)) as $l' '(max_by(.op_id)) as $l'
M C08 "AM-P1 --reconcile auto-REAPS a HUNG op (TERM to a live owner) instead of reporting it" '      hung) emit drift hung_op "$oid" "purpose $pk: $ev" ;;' '      hung) emit drift hung_op "$oid" "purpose $pk: $ev" "reapop:$oid" ;;'
M C09 "AM-P1: podman ps runs with no timeout (a wedged runtime wedges the sweep)" 'ps=$(timeout "$LO_PODMAN_TIMEOUT" "$PODMAN" ps --filter label=project=catalogizer' 'ps=$("$PODMAN" ps --filter label=project=catalogizer'
ML "" C10 "R2-4 (ported): a BARE container_label value equal to the container label is no longer matched" 'elif startswith("op_id=") then .[6:] else . end' 'elif startswith("op_id=") then .[6:] else "" end'
M NM-A8 "A: attached_to / superseded_by naming NO op still excuses a duplicate owner" 'any(.[]; .op_id==$a and .purpose_key==$o.purpose_key and nonterm)' 'any(.[]; true)'
M NM-A9 "A: a merge.json that fails the record shape is judged anyway (an absent pid read as a gone process: interrupted_merge)" 'if [ -z "$mrec" ]; then emit unread merge_record_unreadable' 'if false; then emit unread merge_record_unreadable'
M NM-A11 "A: an unreadable record does not stop the un-adopted-handoff report (anybad not set)" 'anybad=1; _unread_op_record "$file"; continue; fi' '_unread_op_record "$file"; continue; fi'
M NM-B1 "B: a detector that ended abnormally (no end sentinel) is read as completed" '; : >"$W/ok.$1" ) >"$2" 2>"$W/err.$1"; [ -e "$W/ok.$1" ]; }' '; : >"$W/ok.$1" ) >"$2" 2>"$W/err.$1"; true; }'
M NM-B2 "B: the one-row-per-selected-invariant completeness check is dropped (a half-abandoned driver exits 0)" 'if [ "$NROWS" -ne "$NSEL" ]; then' 'if false; then'
M NM-B3 "B: the final status and exit come from the DETECTION pass (no re-derivation after a reconcile)" 'pass=$((pass+1)); [ "$pass" -le 3 ] || break' 'break'
M NM-B3b "B: a reconcile action may be repeated in one run" '[ -z "${DONE[$act]:-}" ] || continue; DONE[$act]=1' 'DONE[$act]=1'
M NM-C1 "C: an action is run for an invariant whose catalogue entry does not list it" '_action_of_invariant "$id" "$2" || { echo refused_action_not_of_invariant; return; }' ':'
M NM-C2 "C: the dead-owner reap is not bound to the class the sweep detected (--expect dropped)" '--op-id "$arg" --expect dead_owner' '--op-id "$arg"'
M NM-C3 "C: stopcontainer accepts an unsafe container id" 'lo_safe_name "$cid" || { echo refused_unsafe_container_id; return; }' ':'
M NM-C4 "C: the findings stream does not escape a backslash (a name can forge an escape)" 's=${s//\\/\\\\}; s=${s//$'"'"'\t'"'"'/\\t};' 's=${s//$'"'"'\t'"'"'/\\t};'
M NM-C5 "C: the findings stream does not escape LF (a name can start a forged row)" 's=${s//$'"'"'\n'"'"'/\\n}; ' ''
ML "" NM-E1 "E: one label belonging to more than one lineage resolves instead of being unreadable" '[ "$n" -eq 1 ] || { echo unreadable; return; }' ':'
ML "" NM-E2 "E: an unreadable record of ops/ no longer makes every label unreadable" '[ "$LIN_BAD" = 0 ] || { echo unreadable; return; }' ':'
ML "" NM-E4 "E: a handoff lineage is classed terminal (its container is stopped)" 'elif $l.state=="handoff" then "handoff" else "terminal" end' 'else "terminal" end'
M NM-H2 "H: a git lock whose name holds a control character is judged and given an action" 'elif _plain "$lk"; then emit drift stale_git_lock' 'elif true; then emit drift stale_git_lock'
M NM-I1 "I: initsub accepts a path with .. components" '[[ "/$arg/" != */../* ]] || { echo refused_unsafe_path; return; }' ':'
M NM-I2 "I: the build_tmp removal is not contained in the builds directory" 'case "$pc" in "$bd"/*) rel=${pc#"$bd"/} ;; *) echo refused_outside_builds; return ;; esac' 'rel=${pc#"$bd"/}'
M NM-I3 "I: a symlink is followed by the rmdir action" '[ ! -L "$path" ] || { echo refused_symlink; return; }' ':'
echo "MUTATION RESULT caught=$caught survived=$surv timeout=$tmo equivalent=$eqv total=$tot"
[ "$surv" -eq 0 ] && [ "$tmo" -eq 0 ]
