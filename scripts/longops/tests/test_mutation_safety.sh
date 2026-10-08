#!/usr/bin/env bash
# test_mutation_safety.sh - WF11 F15: the structural containment of the mutation runners (mutation_safety.sh) is itself tested by running HYPOTHETICAL BAD MUTANTS
# (constitution 11.4.263 D, 11.4.201, 11.4.273). Layer 1 (the pre-execution scan) is exercised on textual mutants that are never run; layer 2 (the BASH_ENV shim and the
# PATH stubs) is exercised with a RECORDER in place of the real kill binary and with signal 0 as the only signal ever named: even if the shim failed, nothing would
# be signalled. Every "nothing reached the kill binary" claim is paired with a control needle: the same recorder DOES record the one call the guard must allow.
. "$(dirname "$0")/lib.sh"
ident_header WF11-F15
. "${MS_LIB:-$(dirname "$0")/mutation_safety.sh}"   # MS_LIB lets mutate_safety.sh substitute a mutated containment library (WF14 R2-T4 RM8)
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

# WF14 round 3: the four bypasses the round-2 review demonstrated (R2-T1 RB5, RB7, RB8) and the reviewer mutant RM8 (any `#` made the whole line a comment)
mk hy; app hy "python3 -c 'import os; os.killpg(1, 9)'"; ms_scan "$PR" "$FX/m-hy" >/dev/null 2>&1; assert_rc "S26 WF14 RB5 python os.killpg(1, 9) (the 11.4.263 incident itself) is aborted" $? 1
mk hz; app hz "python3 -c 'import os; os.killpg(os.getpgid(0), 9)'"; ms_scan "$PR" "$FX/m-hz" >/dev/null 2>&1; assert_rc "S27 WF14 RB5 python os.killpg(os.getpgid(0), 9) is aborted" $? 1
mk ha; app ha "python3 -c 'import psutil; psutil.Process(1).kill()'"; ms_scan "$PR" "$FX/m-ha" >/dev/null 2>&1; assert_rc "S28 WF14 RB5 psutil Process(1).kill() is aborted" $? 1
for spec in 'import psutil as p' 'p.send_signal(9)' 'psutil.Process(1).terminate()'; do
  mk hi; app hi "$spec"; ms_scan "$PR" "$FX/m-hi" >/dev/null 2>&1; assert_rc "S28b WF14 RB5 an interpreter signal API [$spec] is aborted" $? 1
done
for spec in '/usr/bin/k?ll -9 -1' '/bin/k*ll -9 -1' '/usr/bin/[k]ill -9 -1' '/usr/sbin/sh*utdown' '$chroot/usr/bin/k?ll -9 -1'; do   # the last one has an alphanumeric before the slash: only the glob scan (no left boundary) sees it
  n=$((n+1)); mk hb; app hb "$spec"; ms_scan "$PR" "$FX/m-hb" >/dev/null 2>&1; assert_rc "S29 WF14 RB7 a glob-built absolute path [$spec] is aborted" $? 1
done
for spec in 'ms_kill_guard() { "$MS_REAL_KILL" "$@"; }' 'unset -f kill' 'enable kill' 'BASH_ENV=/dev/null bash -c x' 'MS_REAL_KILL=/bin/true' 'ms_blocked() { return 0; }'; do
  mk hc; app hc "$spec"; ms_scan "$PR" "$FX/m-hc" >/dev/null 2>&1; assert_rc "S30 WF14 RB8 a mutant that redefines or disables the containment [$spec] is aborted" $? 1
done
mk hd; app hd 'kill -9 -1   # cleanup'; ms_scan "$PR" "$FX/m-hd" >/dev/null 2>&1; assert_rc "S31 WF14 RM8 a kill line with a TRAILING comment is code, not a comment: aborted" $? 1
mk hd2; app hd2 'echo "x # y"; kill -9 -1'; ms_scan "$PR" "$FX/m-hd2" >/dev/null 2>&1; assert_rc "S31b a # inside a string before a kill command does not hide it: aborted" $? 1
for spec in '"$PODMAN" stop -t 5 "$(podman ps -q | head -1)"' 'podman rm -f x' '/usr/bin/podman stop x' 'podman system prune -af'; do
  mk he; app he "$spec"; ms_scan "$PR" "$FX/m-he" >/dev/null 2>&1; assert_rc "S32 WF14 R2-T2 a new podman side effect [$spec] is aborted" $? 1
done
mk hg; app hg '# podman stop x, kill -9 -1 : comments never execute'; ms_scan "$PR" "$FX/m-hg" >/dev/null 2>&1; assert_rc "S33 golden-true: a pure comment naming podman stop and kill is allowed" $? 0
mk hh lib.sh 'flock -w 15' 'flock -w 2'; ms_scan "$PR" "$FX/m-hh" >/dev/null 2>&1; assert_rc "S34 golden-true: an ordinary edit still runs after the token set grew" $? 0
# round 5 (WF17 F7, CONS LO-T3 b): a computed absolute path or a quote-split word is refused when it is a NEW line; a pristine line, an ordinary edit and an empty-quote argument are not
mk hj; app hj 'b=/usr/bin; "$b/k""ill" -0 2'; ms_scan "$PR" "$FX/m-hj" >/dev/null 2>&1; assert_rc "S35 WF17 F7 the computed-path line b=/usr/bin; \"\$b/k\"\"ill\" is aborted (golden-bad)" $? 1
mk hk; app hk 'p=/sbin; "${p}/sh""utdown" -h now'; ms_scan "$PR" "$FX/m-hk" >/dev/null 2>&1; assert_rc "S35b a quote-split /sbin command is aborted" $? 1
mk hl; app hl 'c=k""ill; $c -0 2'; ms_scan "$PR" "$FX/m-hl" >/dev/null 2>&1; assert_rc "S35c a quote-split word with no path is aborted" $? 1
mk hm; app hm "c=k''ill; \$c -0 2"; ms_scan "$PR" "$FX/m-hm" >/dev/null 2>&1; assert_rc "S35d a single-quote split word is aborted" $? 1
mk hn; app hn "printf '%s' ''; x=''; y=\"\"; echo done"; ms_scan "$PR" "$FX/m-hn" >/dev/null 2>&1; assert_rc "S36 golden-true: empty-quote ARGUMENTS (not inside a word) are allowed" $? 0
mk ho; app ho 'cd /tmp && ls'; ms_scan "$PR" "$FX/m-ho" >/dev/null 2>&1; assert_rc "S36b golden-true: a path that is not a bin directory is allowed" $? 0
mk hr lib.sh 'exit "${3:-$RC_USAGE}"; }' 'exit "${3:-$RC_USAGE}"; } # was /usr/bin/true'; ms_scan "$PR" "$FX/m-hr" >/dev/null 2>&1; assert_rc "S36c a /usr/bin mention in a TRAILING comment on an edited line is refused conservatively (to the scan a trailing comment is code)" $? 1
# CONS-11: a byte-identical copy of a pristine signal line is admitted anywhere (stated, not hidden): the test records the limit so the doc claim cannot drift
mk hs; app hs '  kill -s "$sig" -- "$pid" 2>/dev/null'; ms_scan "$PR" "$FX/m-hs" >/dev/null 2>&1; assert_rc "S37 CONS-11 stated limit: a byte-identical COPY of a pristine kill line is admitted by layer 1 (layer 2 is what contains it)" $? 0
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
# round 5 (CONS-5 / LO-T3 a): rc 1 alone is satisfied by the HOST kill (EPERM on pid 1) and by pkill (no match), so each refusal is paired with (i) the identity of the resolved command (the INNER stub, never the
# outer containment's stub that sits later on PATH nor the host binary) and (ii) exactly ONE new line in the INNER log. A mutant of the library that does not create a stub is then visible here.
for n in kill pkill killall skill podman; do assert_eq "L5-id the INNER containment resolves '$n' to its own stub" "$(ms_run bash -c "type -P $n")" "$MS_BIN/$n"; done
for n in kill pkill killall podman ms_blocked ms_kill_guard; do assert_eq "L5-ro the containment function '$n' is READ-ONLY (the listing of declare -F carries -fr for it)" "$(ms_run bash -c 'declare -F' | grep -cx "declare -fr $n")" 1; done
refused() {  # refused <label> <command> <expected inner-log words>: rc 1 AND exactly one new inner log line containing the words, recorder untouched
  local l0 l1; l0=$(wc -l <"$MS_LOG"); ms_run bash -c "$2" >/dev/null 2>&1; local rc=$?; l1=$(wc -l <"$MS_LOG")
  assert_rc "$1 rc" $rc 1; assert_eq "$1 exactly ONE new line in the INNER log" "$((l1-l0))" 1
  tail -1 "$MS_LOG" | grep -q -- "$3" && ok "$1 the log line names the refused call" || bad "$1 [$(tail -1 "$MS_LOG")]"
}
refused "L5 command kill finds only the PATH stub" 'command kill -0 1' 'BLOCKED stub kill'
refused "L5b env kill is refused by the stub" 'env kill -0 1' 'BLOCKED stub kill'
refused "L5c pkill is refused" 'pkill -0 -x nosuchprocess' 'pkill'
refused "L5d killall is refused" 'killall -0 nosuchprocess' 'killall'
refused "L5e command pkill is refused by the stub" 'command pkill -0 -x nosuchprocess' 'BLOCKED stub pkill'
refused "L5g command killall is refused by the stub" 'command killall -0 nosuchprocess' 'BLOCKED stub killall'
refused "L5h skill is refused by the stub" 'skill -0 -u nobody' 'BLOCKED stub skill'
assert_eq "L5f the recorder is still empty" "$(wc -l <"$FX/rec.log")" 0

echo "== WF14 layer 2: podman is contained, the shim cannot be redefined from inside a mutant =="
: >"$FX/rec.log"; : >"$MS_LOG"
assert_eq "L6 WF14 R2-T2 inside the containment 'podman' resolves to the PATH stub, never the host binary" "$(ms_run bash -c 'type -P podman')" "$MS_BIN/podman"
ms_run bash -c 'podman stop wf14-no-such-container' >/dev/null 2>&1; assert_rc "L6b podman stop is refused by the stub (1)" $? 1
ms_run bash -c 'command podman rm -f wf14-no-such-container' >/dev/null 2>&1; assert_rc "L6c command podman rm is refused too (1)" $? 1
ms_run bash -c 'env podman stop wf14-no-such-container' >/dev/null 2>&1; assert_rc "L6c2 env podman stop reaches the PATH stub and is refused (1)" $? 1
assert_eq "L6d CONTROL NEEDLE: a read-only podman ps goes through the stub and lists nothing (0, empty)" "$(ms_run bash -c 'podman ps --format {{.ID}}; echo rc=$?')" "rc=0"
grep -c 'BLOCKED.*podman' "$MS_LOG" | grep -qx 3 && ok "L6e all three refused calls were logged" || bad "L6e [$(cat "$MS_LOG")]"
: >"$FX/rec.log"; : >"$MS_LOG"
ms_run bash -c 'ms_kill_guard() { "$MS_REAL_KILL" "$@"; } 2>/dev/null; kill -0 -1' >/dev/null 2>&1; assert_rc "L7 WF14 RB8 redefining the guard function from inside the containment fails (readonly): kill -0 -1 is still refused" $? 1
ms_run bash -c 'unset -f kill 2>/dev/null; kill -0 -1' >/dev/null 2>&1; assert_rc "L7b unset -f kill fails (readonly): kill -0 -1 is still refused" $? 1
ms_run bash -c 'kill() { "$MS_REAL_KILL" "$@"; } 2>/dev/null; kill -0 -1' >/dev/null 2>&1; assert_rc "L7c redefining kill itself fails (readonly): refused" $? 1
assert_eq "L7d CONTROL: the recorder is still empty; the shim logged the refusals" "$(wc -l <"$FX/rec.log"):$(grep -c BLOCKED "$MS_LOG" | awk '$1>=3{print "ok"}')" "0:ok"
ms_run bash -c "kill -0 $P" >/dev/null 2>&1; assert_rc "L7e CONTROL NEEDLE: a legitimate target still reaches the recorder after the readonly hardening" $? 0
# round 5 (found by the NEGATIVE CONTROL of mutate_registry.sh: CTRL0 failed an unmutated tree): the shim read the process group with a line-oriented sed over /proc/<pid>/stat, so a live process whose comm holds a NEWLINE
# was refused as "process group is ''" (a false-positive refusal of the instrument itself, 11.4.201 (1)): every contained run of a test that signals such a process failed for the wrong reason
: >"$FX/rec.log"; : >"$MS_LOG"
python3 -I -c 'import ctypes,time; ctypes.CDLL(None).prctl(15, b"ab\ncd", 0, 0, 0); print("ready", flush=True); time.sleep(600)' >"$FX/nl.ready" 2>/dev/null & NLP=$!; KILLME+=("$NLP")
for _ in $(seq 1 40); do [ -s "$FX/nl.ready" ] && break; sleep 0.1; done
[ "$(wc -l </proc/$NLP/comm)" -ge 2 ] && ok "L8 the comm of the test process really holds a newline (precondition of L8b)" || bad "L8 the comm holds no newline [$(cat /proc/$NLP/comm)]"
ms_run bash -c "kill -0 $NLP" >/dev/null 2>&1; assert_rc "L8b a live process whose comm holds a NEWLINE is allowed through to the recorder (not refused as 'process group is empty')" $? 0
grep -qx -- "-0 $NLP" "$FX/rec.log" && ok "L8c the recorder saw that call" || bad "L8c [$(cat "$FX/rec.log")] [$(cat "$MS_LOG")]"
KILLME+=("$NLP")   # cleaned at exit with the real kill (MS_REAL_KILL is the recorder until then, so a contained `kill` here would not end it and `wait` would hang)
: >"$FX/rec.log"; : >"$MS_LOG"
: >"$FX/rec.log"; : >"$MS_LOG"

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
mk h5 lib.sh '[[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return "$RC_UNSAFE"
  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt 1 ]] || return "$RC_UNSAFE"
  printf' 'printf'; app h5 'ms_kill_guard() { "$MS_REAL_KILL" "$@"; }'
ms_scan "$PR" "$FX/m-h5" >/dev/null 2>&1; assert_rc "R4b WF14 RB8 H5 = guards removed AND the shim guard redefined: layer 1 aborts it" $? 1
for bad in 0 1 -1 -2; do r=$(run_mut h5 "$bad"); assert_eq "R4c WF14 RB8 H5 run anyway with pid $bad: the readonly shim still refuses" "$r" rc=1; done
assert_eq "R5 the recorder is STILL empty after four hypothetical bad mutants" "$(wc -l <"$FX/rec.log")" 0
assert_eq "R5b and the shim logged their attempts as BLOCKED" "$(grep -c BLOCKED "$MS_LOG" | awk '$1>=6{print "ok"}')" ok
r=$(run_mut h4 "$P"); assert_eq "R6 control: the same H4 mutant with a legitimate sleeper reaches the recorder" "$r" rc=0
grep -qx -- "-s 0 -- $P" "$FX/rec.log" && ok "R6b the recorder recorded exactly that one call" || bad "R6b [$(cat "$FX/rec.log")]"
unset MS_REAL_KILL
finish
