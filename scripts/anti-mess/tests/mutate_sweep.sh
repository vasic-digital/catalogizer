#!/usr/bin/env bash
# mutate_sweep.sh - paired mutations (G-GATE, 11.4.115 F / 1.1) of scripts/anti-mess/sweep.sh: each mutation breaks ONE load-bearing line in a COPY
# of the sweep; test_sweep.sh run against that copy (env SWEEP) must FAIL. A mutant that leaves the test green survives and fails this runner.
# The copy sits in a scratch tree whose scripts/longops and scripts/repo are symlinks to the real ones (the mutation under test is the sweep's).
# Usage  mutate_sweep.sh [--only S01,S05]     Output: one line per mutation, then `MUTATION RESULT caught=N survived=M total=T`
. "$(dirname "$0")/../../longops/tests/lib.sh"
[ "$(id -u)" != 0 ] || { echo "refusing to run mutations as root"; exit 2; }
ONLY=""; [ "${1:-}" = --only ] && ONLY=,${2:-},
caught=0; surv=0; tot=0
M() {  # M <id> <description> <old> <new>
  local id=$1 desc=$2 old=$3 new=$4
  [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); local d=$FX/mut-$id; rm -rf "$d"; mkdir -p "$d/scripts/anti-mess"
  cp "$TROOT/scripts/anti-mess/catalogue.yaml" "$d/scripts/anti-mess/"; ln -s "$TROOT/scripts/longops" "$d/scripts/longops"; ln -s "$TROOT/scripts/repo" "$d/scripts/repo"
  python3 - "$TROOT/scripts/anti-mess/sweep.sh" "$d/scripts/anti-mess/sweep.sh" "$old" "$new" <<'PY' || { echo "INVALID $id $desc: pattern not found"; surv=$((surv+1)); return; }
import sys
src,dst,old,new=sys.argv[1:5]; s=open(src).read()
if old not in s: sys.exit(1)
open(dst,'w').write(s.replace(old,new,1))
PY
  SWEEP=$d/scripts/anti-mess/sweep.sh bash "$(dirname "$0")/test_sweep.sh" >"$FX/mut-$id.out" 2>&1; local rc=$?
  if [ $rc -ne 0 ]; then caught=$((caught+1)); echo "CAUGHT   $id $desc :: $(grep -m2 '^FAIL' "$FX/mut-$id.out" | cut -c1-110 | tr '\n' '|')"
  else surv=$((surv+1)); echo "SURVIVED $id $desc"; fi
}
M S01 "the heartbeat check is disabled in AM-P1 (a hung op is never reported)" '      hung) emit drift hung_op' '      hung_x) emit drift hung_op'
M S02 "INV-9: the finished-run removal skips the remote check (a held commit no remote holds)" 'if [ ! -s "$d/commits.tsv" ] || _held_ok "$d/commits.tsv"; then' 'if true; then'
M S03 "AM-R1 at S0 treats every path as declared" 'grep -qxF -- "$rel" <<<"$decl" ||' 'true ||'
M S04 "AM-R1 ignores the reviewed exceptions.tsv" '--exceptions "${AM_EXC:-$TOOLS/scripts/repo/exceptions.tsv}"' '--exceptions /dev/null'
M S05 "a catalogued invariant with no detector is reported clean" 'status=not_evaluated; reason=${RSN[$id]}' 'status=clean; reason=${RSN[$id]}'
M S06 "AM-R2 ignores whether a process holds the lock open" 'if h=$(_open_by_any "$lk"); then' 'if false; then'
M S07 "INV-9 reconcile removes an interrupted run" 'never removed by the sweep"; fi' 'never removed by the sweep" "rmdir:$d"; fi'
M S08 "a pending pin move is reported as drift" 'emit info pending_pin_move' 'emit drift pending_pin_move'
M S09 "a blocking core.hooksPath is not refused at S0" '&& [ "$STAGE" = S0 ] && REFUSE=1' '&& [ "$STAGE" = S0_x ] && REFUSE=1'
M S10 "INV-9: a live merge holder is reported as interrupted" 'if lo_alive "$mpid" "$mst"; then emit info live_merge_holder' 'if false; then emit info live_merge_holder'
M S11 "INV-9: a suspended run with a live holder is reported interrupted" 'if [ "$hrun" = "$id" ] && [ "$hstat" = live ]; then emit info suspended_run' 'if false; then emit info suspended_run'
M S12 "AM-P3: the temp dir of a live process is reaped" 'if lo_alive "$pid" "$st"; then emit info build_tmp_live' 'if false; then emit info build_tmp_live'
M S13 "AM-P1: the orphan-container age budget is ignored" 'elif [ "$age" -gt "$budget" ]; then emit drift orphan_container' 'elif true; then emit drift orphan_container'
M S14 "AM-P2: an attached op still counts as a duplicate owner" '|select((.attached_to//"")=="" and (.superseded_by//"")==""))' ')'
M S15 "a failing control needle no longer makes the detector blind" 'else needle=blind; status=blind; reason="control needle failed: $out"; fi' 'else needle=blind; fi'
M S16 "AM-R2 ignores the minimum lock age" 'elif [ "$age" -lt "$minage" ]; then emit info lock_young' 'elif false; then emit info lock_young'
echo "MUTATION RESULT caught=$caught survived=$surv total=$tot"
[ "$surv" -eq 0 ]
