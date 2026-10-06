#!/usr/bin/env bash
# mutate_registry.sh - paired mutations (G-GATE, 11.4.115 F / 1.1) of scripts/longops: each mutation breaks ONE load-bearing line in a COPY of
# the scripts; test_registry.sh run against that copy must FAIL (exit non-zero). A mutation that leaves the test green is a surviving mutant
# and fails this runner. The real scripts are never edited. Safety: mutation M04 removes the signal guards AND replaces the real `kill`
# by a no-op, so a mutated lo_signal can never signal anything (a guard-less kill of pid 0 or -1 would hit the whole session).
# Usage  mutate_registry.sh [--only M05,M09]       Output: one line per mutation, then `MUTATION RESULT caught=N survived=M total=T`
. "$(dirname "$0")/lib.sh"   # TROOT, S (the real scripts), FX scratch
[ "$(id -u)" != 0 ] || { echo "refusing to run mutations as root"; exit 2; }
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
  # SAFETY: a mutant that lost the pid/pgid guard of lo_signal must also have lost its real kill (kill of 0, 1 or -1 hits the whole session)
  if ! grep -q '"\$pid" -gt 1 \]\] || return "\$RC_UNSAFE"' "$d/lib.sh" && grep -q 'kill -s "\$sig"' "$d/lib.sh"; then echo "SAFETY-ABORT $id: mutant has a real kill without its guard; not run"; surv=$((surv+1)); return; fi
  LONGOPS_SCRIPTS=$d bash "$(dirname "$0")/test_registry.sh" >"$FX/mut-$id.out" 2>&1; local rc=$?
  if [ $rc -ne 0 ]; then caught=$((caught+1)); echo "CAUGHT   $id $desc :: $(grep -m2 '^FAIL' "$FX/mut-$id.out" | cut -c1-110 | tr '\n' '|')"
  else surv=$((surv+1)); echo "SURVIVED $id $desc"; fi
}
M M01 lib.sh "drop the flock: every CAS runs unlocked (adopt vs expire gives two winners)" '( flock -w 15 9 || exit 70; "$@" ) 9>' '( "$@" ) 9>'
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
echo "MUTATION RESULT caught=$caught survived=$surv total=$tot"
[ "$surv" -eq 0 ]
