#!/usr/bin/env bash
# mutate_sweep.sh - paired mutations (G-GATE, 11.4.115 F / 1.1) of scripts/anti-mess/sweep.sh: each mutation breaks ONE load-bearing line in a COPY
# of the sweep; test_sweep.sh run against that copy (env SWEEP) must FAIL. A mutant that leaves the test green survives and fails this runner.
# SAFETY (WF11 F15, 11.4.263): ms_scan aborts a mutant tree carrying a signal/host-power line that is not byte-identical to the pristine tree, and the test runs inside the
# mutation_safety.sh containment (BASH_ENV shim: kill builtin disabled, guard function, PATH stubs); the same library as mutate_registry.sh, tested by test_mutation_safety.sh.
# The copy sits in a scratch tree whose scripts/longops and scripts/repo are symlinks to the real ones (the mutation under test is the sweep's).
# Usage  mutate_sweep.sh [--only S01,S05]     Output: one line per mutation, then `MUTATION RESULT caught=N survived=M total=T`
. "$(dirname "$0")/../../longops/tests/lib.sh"
. "$(dirname "$0")/../../longops/tests/mutation_safety.sh"
[ "$(id -u)" != 0 ] || { echo "refusing to run mutations as root"; exit 2; }
ms_prepare "$FX/shim" || { echo "SAFETY: the mutant containment cannot be built; no mutant is run"; exit 2; }
ONLY=""; [ "${1:-}" = --only ] && ONLY=,${2:-},
caught=0; surv=0; tot=0
M() {  # M <id> <description> <old> <new> [<old2> <new2>]
  local id=$1 desc=$2 old=$3 new=$4 old2=${5:-} new2=${6:-}
  [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); local d=$FX/mut-$id; rm -rf "$d"; mkdir -p "$d/scripts/anti-mess"
  cp "$TROOT/scripts/anti-mess/catalogue.yaml" "$d/scripts/anti-mess/"; ln -s "$TROOT/scripts/longops" "$d/scripts/longops"; ln -s "$TROOT/scripts/repo" "$d/scripts/repo"
  python3 - "$TROOT/scripts/anti-mess/sweep.sh" "$d/scripts/anti-mess/sweep.sh" "$old" "$new" "$old2" "$new2" <<'PY' || { echo "INVALID $id $desc: pattern not found"; surv=$((surv+1)); return; }
import sys
src,dst,old,new,old2,new2=sys.argv[1:7]; s=open(src).read()
if old not in s: sys.exit(1)
s=s.replace(old,new,1)
if old2:
    if old2 not in s: sys.exit(1)
    s=s.replace(old2,new2,1)
open(dst,'w').write(s)
PY
  if ! ms_scan "$TROOT/scripts/anti-mess" "$d/scripts/anti-mess" >"$FX/scan-$id.out" 2>&1; then echo "SAFETY-ABORT $id: $(head -1 "$FX/scan-$id.out" | cut -c1-200); not run"; surv=$((surv+1)); return; fi
  SWEEP=$d/scripts/anti-mess/sweep.sh ms_run bash "$(dirname "$0")/test_sweep.sh" >"$FX/mut-$id.out" 2>&1; local rc=$?
  if [ $rc -ne 0 ]; then caught=$((caught+1)); echo "CAUGHT   $id $desc :: $(grep -m2 '^FAIL' "$FX/mut-$id.out" | cut -c1-110 | tr '\n' '|')"
  else surv=$((surv+1)); echo "SURVIVED $id $desc"; fi
}
M S01 "the heartbeat check is disabled in AM-P1 (a hung op is never reported)" '      hung) emit drift hung_op' '      hung_x) emit drift hung_op'
M S02 "INV-9: the finished-run removal skips the remote check (a held commit no remote holds)" 'if [ ! -s "$d/commits.tsv" ] || _held_ok "$d/commits.tsv"; then' 'if true; then'
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
M S14 "AM-P2: an attached op still counts as a duplicate owner" '|select((.attached_to//"")=="" and (.superseded_by//"")==""))' ')'
M S15 "a failing control needle no longer makes the detector blind" 'else needle=blind; status=blind; reason="control needle failed: $out"; fi' 'else needle=blind; fi'
M S16 "AM-R2 ignores the minimum lock age" 'elif [ "$age" -lt "$minage" ]; then emit info lock_young' 'elif false; then emit info lock_young'
M S17 "RMS1: the rmlock reconcile no longer re-verifies its precondition" 'if [ -e "$arg" ] && ! _open_by_any "$arg" >/dev/null && [ "$(_git_active)" -eq 0 ] && [ "$age" -ge "${ANTIMESS_LOCK_MIN_AGE:-60}" ]; then rm -f' 'if true; then rm -f'
M S18 "F4: corrupt op records are never reported" '_emit_unread_ops() { local f;' '_emit_unread_ops() { return 0; local f;'
M S19 "F8: an unreadable podman is informational (AM-P1 clean)" 'emit unread containers_unread podman' 'emit info containers_unread podman'
M S20 "F9: a stale claim is informational" 'dead) emit drift stale_claim' 'dead) emit info stale_claim'
M S21 "F9: an un-adopted handoff is informational" 'emit drift handoff_unadopted' 'emit info handoff_unadopted'
M S22 "F10: the container of a handoff op is stopped by --reconcile" '      handoff) emit info container_of_handoff_op "$cid" "op $op is handed off (re-adoptable, 11.4.232 D): its container is never stopped by the sweep" ;;' '      handoff) emit drift container_of_terminal_op "$cid" "x" "stopcontainer:$cid:handoff" ;;'
M S23 "F10: stopcontainer does not re-verify" 'if [ "$now_class" != "$want" ]; then echo' 'if false; then echo'
M S24 "F12: an unknown --only id is accepted" '[ "$_k" = 1 ] || die usage' '[ "$_k" = 1 ] || true'
M S25 "label: the catalogizer.op_id label of the launcher is ignored" '((.Labels // {})["catalogizer.op_id"] // (.Labels // {}).op_id // "-")' '((.Labels // {}).op_id // "-")'
M S26 "F10: the finished-run removal does not re-check the held commits" 'if [ -s "$path/commits.tsv" ] && ! _held_ok "$path/commits.tsv"; then echo skipped_held_commit_not_on_remote; return; fi' ':'
M S27 "F10: initsub does not re-verify" "if git -C \"\$AM_ROOT\" submodule status --recursive -- \"\$arg\" 2>/dev/null | grep -q '^-'; then" 'if true; then'
M S28 "F8: a sweep with only unread sources exits 0" '[ "$UNREAD" -gt 0 ] && exit 11' '[ "$UNREAD" -gt 0 ] && exit 0'
M S29 "F4: AM-P2 never reports a corrupt op record" 'det_AM_P2() {
  local p
  _emit_unread_ops' 'det_AM_P2() {
  local p'
M S30 "AM-P4: a claim with no holder record is never reported" 'emit drift claim_without_holder' 'emit info claim_without_holder'
M S31 "AM-P4: an unreadable claim is never reported" 'unreadable) emit drift claim_unreadable' 'unreadable) emit info claim_unreadable'
M S32 "AM-P4: a holder that cannot be judged is informational" 'emit unread claim_holder_unread' 'emit info claim_holder_unread'
M S33 "RM6 (round-2 reviewer): _ops_unread checks only type==object: a valid-JSON record with no state is skipped silently" 'continue; jq -e "$_OPS_OK" "$f" >/dev/null 2>&1 || echo "$f"' 'continue; jq -e '"'"'type=="object"'"'"' "$f" >/dev/null 2>&1 || echo "$f"'
M S34 "RM9 (round-2 reviewer): an unreadable op state is classed terminal (its container is stopped)" 'handoff) echo handoff ;; "") echo unreadable ;;' 'handoff) echo handoff ;; "") echo terminal ;;'
M S35 "WF14 R2-11: an orphan container (no row in THIS registry) is stopped by --reconcile again" 'stop it by hand once its owner is known" ;;' 'stop it by hand once its owner is known" "stopcontainer:$cid:orphan" ;;'
M S36 "WF14 R2-4: a container is matched to its op by the op id only (the record container_label is ignored)" 'f=$(lo_op_file "$v"); if [ -e "$f" ]; then echo "$f"; return 0; fi' 'f=$(lo_op_file "$v"); if [ -e "$f" ]; then echo "$f"; return 0; fi; return 1'
M S37 "WF14 R2-2: a handoff op re-adopted by a later op of its purpose is still reported un-adopted" '<<<"$all")" ] && continue' '<<<"$all")" ] && false'
M S38 "WF14 R2-7: INV-9 asserts a missing live holder it could not read" 'elif [ -n "$hunread" ]; then emit unread suspended_run_holder_unread' 'elif false; then emit unread suspended_run_holder_unread'
M S39 "WF14 class D: a record with no op_id or purpose_key is accepted as readable" '_OPS_OK='"'"'type=="object" and (.op_id|type=="string") and (.purpose_key|type=="string") and (.state|type=="string")'"'"'' '_OPS_OK='"'"'type=="object" and (.state|type=="string")'"'"''
M S40 "WF14: a handoff op is adopted by ANY op of the purpose, also an earlier one" "(.started_utc // \"\") >= \$s)" "true)"
echo "MUTATION RESULT caught=$caught survived=$surv total=$tot"
[ "$surv" -eq 0 ]
