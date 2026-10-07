#!/usr/bin/env bash
# test_mutation_safety.sh - WF11 F15: the structural containment of the mutation runners (mutation_safety.sh) is itself tested by running HYPOTHETICAL BAD MUTANTS
# (constitution 11.4.263 D, 11.4.201, 11.4.273). Layer 1 (the pre-execution scan) is exercised on textual mutants that are never run; layer 2 (the BASH_ENV shim and the
# PATH stubs) is exercised with a RECORDER in place of the real kill binary and with signal 0 as the only signal ever named: even if the shim failed, nothing would
# be signalled. Every "nothing reached the kill binary" claim is paired with a control needle: the same recorder DOES record the one call the guard must allow.
. "$(dirname "$0")/lib.sh"
ident_header WF11-F15
. "$(dirname "$0")/mutation_safety.sh"
export LC_ALL=C
PR=$FX/pristine; mkdir -p "$PR"; cp "$S"/*.sh "$PR/"
mk() {  # mk <name> [<file> <old> <new>]...: a mutant tree (textual only)
  local n=$1; shift; local d=$FX/m-$n; rm -rf "$d"; mkdir -p "$d"; cp "$PR"/*.sh "$d/"
  while [ $# -ge 3 ]; do python3 -I - "$d/$1" "$2" "$3" <<'PY' || { echo "mk $n: pattern not found"; return 1; }
import sys; p,old,new=sys.argv[1:4]; s=open(p).read()
if old not in s: sys.exit(1)
open(p,'w').write(s.replace(old,new,1))
PY
    shift 3; done
}
app() { printf '%s\n' "$2" >>"$FX/m-$1/lib.sh"; }   # app <mutant> <line>

echo "== layer 1: ms_scan on golden-true mutants (must run) and hypothetical bad mutants (must abort) =="
mk g1; ms_scan "$PR" "$FX/m-g1" >/dev/null; assert_rc "S1 golden-true: an identical tree is safe" $? 0
mk g2 lib.sh '[[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return "$RC_UNSAFE"
  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt 1 ]] || return "$RC_UNSAFE"
  printf' 'printf' lib.sh '  kill -s "$sig" -- "$pid" 2>/dev/null
}
# lo_kill_child' '  :
}
# lo_kill_child'
ms_scan "$PR" "$FX/m-g2" >/dev/null; assert_rc "S2 golden-true: the M04 shape (guards removed, the kill replaced by a no-op) carries no signal line and runs" $? 0
mk g3 lib.sh 'flock -w 15' 'flock -w 1'; ms_scan "$PR" "$FX/m-g3" >/dev/null; assert_rc "S3 golden-true: an edit of a line without any signal token runs" $? 0
mk g4; app g4 '# kill -9 -1 would be a comment, not a command'; ms_scan "$PR" "$FX/m-g4" >/dev/null; assert_rc "S4 golden-true: a COMMENT that names kill cannot execute and is allowed" $? 0
mk g5 lib.sh '  kill -s "$sig" -- "$pid" 2>/dev/null
}
# lo_kill_child' '  kill -s "$sig" -- "$pid" 2>/dev/null
}
# lo_kill_child'; ms_scan "$PR" "$FX/m-g5" >/dev/null; assert_rc "S5 golden-true: a signal line kept byte-identical is allowed" $? 0

mk h1 lib.sh '  kill -s "$sig" -- "$pid" 2>/dev/null
}
# lo_kill_child' '  kill -s "$sig" -- -1 2>/dev/null
}
# lo_kill_child'; ms_scan "$PR" "$FX/m-h1" >"$FX/o" 2>&1; assert_rc "S6 H1 kill target replaced by -1, guards intact: aborted (the old grep ran it)" $? 1
grep -q 'kill -s "$sig" -- -1' "$FX/o" && ok "S6b the offending line is printed" || bad "S6b [$(cat "$FX/o")]"
mk h2 lib.sh '  kill -s "$sig" -- "$pid" 2>/dev/null
}
# lo_kill_child' '  kill -s "$sig" -- "-$pid" 2>/dev/null
}
# lo_kill_child'; ms_scan "$PR" "$FX/m-h2" >/dev/null 2>&1; assert_rc "S7 H2 kill target replaced by the process group -pid: aborted" $? 1
mk h3 lib.sh '"$pid" -gt 1 ]] || return "$RC_UNSAFE"
  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt 1 ]] || return "$RC_UNSAFE"
  printf' '"$pid" -gt -2 ]] || return "$RC_UNSAFE"
  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt -2 ]] || return "$RC_UNSAFE"
  printf' lib.sh '  kill -s "$sig" -- "$pid" 2>/dev/null
}
# lo_kill_child' '  kill -s "${sig}" -- "$pid" 2>/dev/null
}
# lo_kill_child'; ms_scan "$PR" "$FX/m-h3" >/dev/null 2>&1; assert_rc "S8 H3 pid guard weakened AND the kill line re-spelled: aborted" $? 1
n=8
for spec in 'command kill -9 -1' 'builtin kill -9 -1' '/bin/kill -9 -1' '/usr/bin/kill -9 -1' 'pkill -u "$USER"' 'killall -u "$USER"' 'xargs kill -9' 'exec kill -9 -1' 'systemctl suspend' 'loginctl terminate-user "$USER"' 'reboot' 'poweroff' 'shutdown -h now' 'echo x; kill -9 -1'; do
  n=$((n+1)); mk hx; app hx "$spec"; ms_scan "$PR" "$FX/m-hx" >/dev/null 2>&1; assert_rc "S$n hypothetical bad mutant appends [$spec]: aborted" $? 1
done
mk hf; printf 'kill -9 -1\n' >"$FX/m-hf/evil.sh"; ms_scan "$PR" "$FX/m-hf" >/dev/null 2>&1; assert_rc "S23 a NEW file carrying a kill line is aborted" $? 1
mk hp; printf 'import os\nos.kill(-1, 9)\n' >"$FX/m-hp/evil.py"; ms_scan "$PR" "$FX/m-hp" >/dev/null 2>&1; assert_rc "S24 a NEW python file with os.kill is aborted" $? 1
mk hq lib.sh 'lo_die() { echo "longops: $1: $2" >&2; exit "${3:-$RC_USAGE}"; }' 'lo_die() { killall -u me; echo "longops: $1: $2" >&2; exit "${3:-$RC_USAGE}"; }'
ms_scan "$PR" "$FX/m-hq" >/dev/null 2>&1; assert_rc "S25 a token smuggled into an existing non-kill line is aborted" $? 1

echo "== layer 2: the BASH_ENV shim and the PATH stubs, with a RECORDER instead of the real kill (signal 0 only) =="
ms_prepare "$FX/shim" || { bad "L0 containment cannot be built on this host"; finish; }
cat >"$FX/rec-kill" <<EOF
#!/bin/sh
printf '%s\n' "\$*" >>"$FX/rec.log"
exit 0
EOF
chmod +x "$FX/rec-kill"; : >"$FX/rec.log"; export MS_REAL_KILL=$FX/rec-kill
assert_eq "L1 canary: inside the containment kill is a FUNCTION" "$(ms_run bash -c 'type -t kill')" function
ms_run bash -c 'builtin kill -0 $$' >/dev/null 2>&1; assert_rc "L1b the kill BUILTIN is disabled (builtin kill fails; signal 0 at our own pid)" $? 1
mkll; P=$LL
ms_run bash -c "kill -0 $P" >/dev/null 2>&1; rc=$?; assert_rc "L2 CONTROL NEEDLE: a legitimate target (a sleeper of ours, pid > 1, pgrp > 1) is allowed through" $rc 0
grep -qx -- "-0 $P" "$FX/rec.log" && ok "L2b the recorder SAW that call: an empty recorder below is evidence, not blindness" || bad "L2b [$(cat "$FX/rec.log")]"
: >"$FX/rec.log"; : >"$MS_LOG"
kt=""; for d in /proc/[0-9]*; do k=${d#/proc/}; [ "$k" -gt 1 ] 2>/dev/null || continue; pg=$(sed 's/^.*) //' "$d/stat" 2>/dev/null | cut -d' ' -f3); [ "$pg" = 0 ] && { kt=$k; break; }; done
nb=0
for a in "-0 -1" "-s 0 -- -1" "-0 0" "-0 1" "-0 -$$" "-s 0 -- -$P" "-0 abc" "-0 %1" "-0 ''" "-0 99999999999999999999" "-0 $P -1" "-0 $P 1" "-- -1" "-0"; do
  ms_run bash -c "kill $a" >/dev/null 2>&1; rc=$?; nb=$((nb+1)); assert_rc "L3 kill $a is refused by the shim (rc 1)" $rc 1
done
if [ -n "$kt" ]; then ms_run bash -c "kill -0 $kt" >/dev/null 2>&1; assert_rc "L3b a kernel thread ($kt, pgrp 0) is refused" $? 1; nb=$((nb+1))
else ok "L3b skipped: no kernel thread (pgrp 0) on this host (UNCONFIRMED here, 11.4.3)"; fi
assert_eq "L4 NOTHING of the above reached the kill binary (recorder empty)" "$(wc -l <"$FX/rec.log")" 0
assert_eq "L4b every refusal was logged as BLOCKED" "$(grep -c BLOCKED "$MS_LOG")" "$nb"
ms_run bash -c 'command kill -0 1' >/dev/null 2>&1; assert_rc "L5 command kill finds only the PATH stub and is refused" $? 1
ms_run bash -c 'env kill -0 1' >/dev/null 2>&1; assert_rc "L5b env kill is refused by the stub" $? 1
ms_run bash -c 'pkill -0 -x nosuchprocess' >/dev/null 2>&1; assert_rc "L5c pkill is refused" $? 1
ms_run bash -c 'killall -0 nosuchprocess' >/dev/null 2>&1; assert_rc "L5d killall is refused" $? 1
ms_run bash -c 'command pkill -0 -x nosuchprocess' >/dev/null 2>&1; assert_rc "L5e command pkill is refused by the stub" $? 1
assert_eq "L5f the recorder is still empty" "$(wc -l <"$FX/rec.log")" 0

echo "== layer 2 on HYPOTHETICAL BAD MUTANTS that layer 1 would also stop: run anyway, with signal 0, to prove the shim alone holds =="
run_mut() {  # run_mut <mutant-name> <pid-arg>: lo_signal of the mutant, signal 0
  ms_run bash -c ". '$FX/m-$1/lib.sh'; LD='$FX/lsig-$1'; mkdir -p \"\$LD\"; lo_signal 0 '$2'; echo rc=\$?" 2>&1 | tail -1
}
: >"$FX/rec.log"; : >"$MS_LOG"
r=$(run_mut h1 "$P"); assert_eq "R1 H1 (target -1) run under the shim: lo_signal fails (rc 1), the target was never delivered" "$r" rc=1
r=$(run_mut h2 "$P"); assert_eq "R2 H2 (target -pid) run under the shim: refused" "$r" rc=1
if [ -n "$kt" ]; then r=$(run_mut h3 "$kt"); assert_eq "R3 H3 (guards weakened to -gt -2, kernel thread $kt) run under the shim: refused by the shim's own pgrp check" "$r" rc=1; fi
mk h4 lib.sh '[[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return "$RC_UNSAFE"
  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt 1 ]] || return "$RC_UNSAFE"
  printf' 'printf'
ms_scan "$PR" "$FX/m-h4" >/dev/null 2>&1; assert_rc "R4a H4 = the ORIGINAL INCIDENT (guards removed, the real kill kept byte-identical): layer 1 lets it run" $? 0
for bad in 0 1 -1 -2; do r=$(run_mut h4 "$bad"); assert_eq "R4 H4 run with pid $bad under the shim: refused, nothing delivered" "$r" rc=1; done
assert_eq "R5 the recorder is STILL empty after four hypothetical bad mutants" "$(wc -l <"$FX/rec.log")" 0
assert_eq "R5b and the shim logged their attempts as BLOCKED" "$(grep -c BLOCKED "$MS_LOG" | awk '$1>=6{print "ok"}')" ok
r=$(run_mut h4 "$P"); assert_eq "R6 control: the same H4 mutant with a legitimate sleeper reaches the recorder" "$r" rc=0
grep -qx -- "-s 0 -- $P" "$FX/rec.log" && ok "R6b the recorder recorded exactly that one call" || bad "R6b [$(cat "$FX/rec.log")]"
unset MS_REAL_KILL
finish
