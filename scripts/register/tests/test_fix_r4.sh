#!/usr/bin/env bash
# test_fix_r4.sh - WF15 review fix round 4 (11.4.276: the round AFTER the structural round 3) of the register operations: I1 I2 I3 M1..M7 of
# WF15-REVIEW-register-r3.md. Written FIRST: every section reproduces one reviewed defect (or one member of its class) against the committed
# scripts of round 3 (RED: SUT_DIR / LOCKED / BACKUP / REPLAY point at a copy of commit 67f4d247) and must pass on the fixed ones (GREEN).
# Classes (docs: evidence/wp06/fix-r4-convergence-assessment.md): C2' a decision about the writer taken from an identity that does not cover its whole life
# (script x client x container), C4' the exit status read from a source that can lose it, C6' a skip or exemption decided from a forgeable token.
# Sections: SIG3 (I3), STATUS (I2, stub), FENCE4 (I1 M2 M3 M4, stub), SCOPE (M1, stub), BSIG (M5 M7), M6 (replay), SWEEP (I1, real podman), REALSTAT (I2, real
# podman), REALSCOPE (M1, real podman). Env: ONLY="SIG3 M6" (sections), NO_E2E=1 (skip the real-podman sections).
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"; . "$(dirname "$0")/mutscore.sh"
evhead FIX-R4
ONLY=" ${ONLY:-SIG3 STATUS FENCE4 SCOPE BSIG M6 SWEEP REALSTAT REALSCOPE} "
want() { case "$ONLY" in *" $1 "*) return 0;; esac; return 1; }
e2e() { [ -z "${NO_E2E:-}" ]; }
echo "# sut sha256 locked=$(fsha "$LOCKED" | cut -c1-16) backup=$(fsha "$BACKUP" | cut -c1-16) replay=$(fsha "$REPLAY" | cut -c1-16)"

# ---------- stub machinery (no container): a stub RUNP and a stub podman holding one fake container per op id ----------
STUB_DIR="$T_SCR/stub"; export STUB_DIR; mkdir -p "$STUB_DIR"
STUBRUNP="$T_SCR/stub_runp.sh"
cat >"$STUBRUNP" <<'S'
#!/usr/bin/env bash
# stub RUNP: snapshot calls (op-id *-snap) answer CAT-001 (the 2nd one after $SNAP2_SLEEP seconds); the main call execs $STUB_DIR/main.sh
op=""; a=("$@"); while [ $# -gt 0 ]; do case "$1" in --op-id) op="$2"; shift 2;; --) shift; break;; *) shift;; esac; done
echo "$(date +%s.%N) pid=$$ op=$op argv=${a[*]}" >> "$STUB_DIR/runp.log"
case "$op" in
  *-snap) n=$(cat "$STUB_DIR/snapn" 2>/dev/null || echo 0); n=$((n+1)); echo $n >"$STUB_DIR/snapn"
          if [ "$n" = 2 ] && [ -n "${SNAP2_SLEEP:-}" ]; then awk '/^SigIgn/{print $2}' /proc/$$/status >"$STUB_DIR/aftersnap.sigign"; : >"$STUB_DIR/aftersnap.started"; sleep "$SNAP2_SLEEP"; fi; echo "CAT-001"; exit 0;;
  *) [ -x "$STUB_DIR/main.sh" ] && exec "$STUB_DIR/main.sh" "${a[@]}"; exit 0;;
esac
S
chmod +x "$STUBRUNP"
STUBPODMAN="$T_SCR/stub_podman.sh"
cat >"$STUBPODMAN" <<'S'
#!/usr/bin/env bash
# stub podman (the documented LOCKED_PODMAN test hook): a fake container of op X is the pid in $STUB_DIR/ctr.X.pid, its exit status $STUB_DIR/ctr.X.rc, the /src
# mount of its checkout $STUB_DIR/ctr.X.src. wait.fail = `podman wait` loses the container (--rm removed it), events.fail / events.out steer `podman events`.
echo "$(date +%s.%N) $*" >>"$STUB_DIR/podman.log"
[ -f "$STUB_DIR/podman.fail" ] && exit 125
opof() { local a; for a in "$@"; do case "$a" in label=catalogizer.op_id=*) echo "${a#label=catalogizer.op_id=}";; esac; done; }
case "$1" in
  ps) op=$(opof "$@"); p=$(cat "$STUB_DIR/ctr.$op.pid" 2>/dev/null); if [ -n "$p" ] && kill -0 "$p" 2>/dev/null; then echo "ctr-$op"; fi; exit 0;;
  inspect) id="${*: -1}"; op="${id#ctr-}"; cat "$STUB_DIR/ctr.$op.src" 2>/dev/null; exit 0;;
  wait) [ -f "$STUB_DIR/wait.fail" ] && { echo "Error: no such container" >&2; exit 125; }
        op="${*: -1}"; op="${op#ctr-}"; p=$(cat "$STUB_DIR/ctr.$op.pid" 2>/dev/null); while [ -n "$p" ] && kill -0 "$p" 2>/dev/null; do sleep 0.1; done; cat "$STUB_DIR/ctr.$op.rc" 2>/dev/null || echo 0; exit 0;;
  events) [ -f "$STUB_DIR/events.fail" ] && exit 1
          if [ -f "$STUB_DIR/events.notbefore" ] && [ "$(date +%s)" -lt "$(cat "$STUB_DIR/events.notbefore")" ]; then exit 0; fi   # a lagging events backend: nothing readable yet
          if [ -f "$STUB_DIR/events.out" ]; then cat "$STUB_DIR/events.out"; exit 0; fi
          op=$(opof "$@"); p=$(cat "$STUB_DIR/ctr.$op.pid" 2>/dev/null)
          if [ -f "$STUB_DIR/ctr.$op.rc" ] && ! { [ -n "$p" ] && kill -0 "$p" 2>/dev/null; }; then echo "ctr-$op $(cat "$STUB_DIR/ctr.$op.rc")"; fi; exit 0;;
  rm) id="${*: -1}"; op="${id#ctr-}"; p=$(cat "$STUB_DIR/ctr.$op.pid" 2>/dev/null); if [ -n "$p" ] && [ "$p" -gt 1 ]; then kill -9 "$p" 2>/dev/null; fi; echo killed >"$STUB_DIR/ctr.$op.killed"; exit 0;;
esac
exit 0
S
chmod +x "$STUBPODMAN"
sreset() { rm -rf "$STUB_DIR"; mkdir -p "$STUB_DIR"; }
sroot() { local r; r=$(mkroot "$1"); echo "stub db" >"$r/docs/workable_items.db"; echo "$r"; }
slk() { LK_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" lk "$@"; }
jrow() { python3 -I -c 'import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]
r=[x for x in rows if x["op_id"]==sys.argv[2]]
print("none" if not r else json.dumps(r[0].get(sys.argv[3])))' "$@"; }
killpid() { case "${1:-}" in ''|*[!0-9]*) return 0;; esac; [ "$1" -gt 1 ] && kill "$1" 2>/dev/null; return 0; }
fakectr() {  # fakectr <op> <seconds> [<rc>]: a fake detached container of op <op> that logs s/e lines and ends with status <rc>
  ( echo "s $(date +%s%N) ctr-$1" >>"$STUB_DIR/ov.log"; sleep "$2"; echo "e $(date +%s%N) ctr-$1" >>"$STUB_DIR/ov.log"; echo "${3:-0}" >"$STUB_DIR/ctr.$1.rc" ) >/dev/null 2>&1 </dev/null 8>&- 9>&- &
  echo $! >"$STUB_DIR/ctr.$1.pid"; disown 2>/dev/null; }
cat >"$T_SCR/main_ctr.sh" <<'S'
#!/usr/bin/env bash
# stub client: logs its start, starts a fake container (writes the database late) when CTR_SLEEP is set, leaves at once with STUB_CLIENT_RC
op=""; while [ $# -gt 0 ]; do case "$1" in --op-id) op="$2"; shift 2;; --) shift; break;; *) shift;; esac; done
echo "w $(date +%s%N) $op" >>"$STUB_DIR/ov.log"
if [ -n "${CTR_SLEEP:-}" ]; then
  ( echo "s $(date +%s%N) ctr-$op" >>"$STUB_DIR/ov.log"; sleep "$CTR_SLEEP"; echo "late write" >>docs/workable_items.db; echo "e $(date +%s%N) ctr-$op" >>"$STUB_DIR/ov.log"; echo "${CTR_RC:-5}" >"$STUB_DIR/ctr.$op.rc" ) >/dev/null 2>&1 </dev/null 8>&- 9>&- &
  echo $! >"$STUB_DIR/ctr.$op.pid"
fi
exit "${STUB_CLIENT_RC:-141}"
S
chmod +x "$T_SCR/main_ctr.sh"
usemain() { cp "$T_SCR/main_ctr.sh" "$STUB_DIR/main.sh"; }
# a stub client that is ATTACHED for $CLIENT_ATTACH seconds and starts its container only after $CTR_DELAY (the I1 window: the client is a live process, its container does not exist yet)
cat >"$T_SCR/main_early.sh" <<'S'
#!/usr/bin/env bash
op=""; while [ $# -gt 0 ]; do case "$1" in --op-id) op="$2"; shift 2;; --) shift; break;; *) shift;; esac; done
sleep "${CTR_DELAY:-0}"
echo "w $(date +%s%N) $op" >>"$STUB_DIR/ov.log"
( echo "s $(date +%s%N) ctr-$op" >>"$STUB_DIR/ov.log"; sleep "${CTR_SLEEP:-4}"; echo "e $(date +%s%N) ctr-$op" >>"$STUB_DIR/ov.log"; echo 0 >"$STUB_DIR/ctr.$op.rc" ) >/dev/null 2>&1 </dev/null 8>&- 9>&- &
echo $! >"$STUB_DIR/ctr.$op.pid"
sleep "${CLIENT_ATTACH:-9}"
exit 0
S
chmod +x "$T_SCR/main_early.sh"
lockfree() { flock -n "$1" true 2>/dev/null; }
ovorder() {  # ovorder <log> <first-end-token> <second-write-op>: 0 when the write of <op> is after the end of <token>
  python3 -I - "$@" <<'PY'
import sys
ev=[l.split() for l in open(sys.argv[1]) if l.strip()]
e1=[int(e[1]) for e in ev if e[0]=="e" and e[2]==sys.argv[2]]
w2=[int(e[1]) for e in ev if e[0]=="w" and e[2]==sys.argv[3]]
assert e1 and w2 and w2[0]>e1[0], (e1,w2)
PY
}
# the signal helpers: start locked.sh through python (restores SIGINT/SIGQUIT to their defaults and execs: the pid stays), as a foreground command sees them
cat >"$T_SCR/sigstart.sh" <<'S'
#!/usr/bin/env bash
# exec chain bash -> python -> env -> bash locked.sh keeps ONE pid; python restores INT QUIT HUP TERM to their defaults first (a background job starts with INT and QUIT ignored, a nohup chain with HUP)
exec python3 -I -c 'import os,signal,sys;[signal.signal(g,signal.SIG_DFL) for g in (signal.SIGINT,signal.SIGQUIT,signal.SIGHUP,signal.SIGTERM)];os.execvp("env",sys.argv[1:])' env LOCKED_TEST_MODE=1 LOCKED_ROOT="$1" LOCKED_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" bash "$LOCKED" "${@:2}"
S
chmod +x "$T_SCR/sigstart.sh"
export STUBRUNP STUBPODMAN LOCKED   # sigstart.sh reads them; it is started DIRECTLY as the background job (a shell function would add a forked subshell and the pid would not be locked.sh)
descendants() { local p c; for c in $(pgrep -P "$1" 2>/dev/null); do echo "$c"; descendants "$c"; done; }

# ================= SIG3 (I3): a signal after the drain never leaves a landed write without its journal row =================
if want SIG3; then echo "== SIG3 TERM INT HUP during the after-snapshot (traps stay installed to the end) =="
  cat >"$STUB_DIR/../main_w.sh" <<'S'
#!/usr/bin/env bash
echo "late write" >> docs/workable_items.db; exit 0
S
  chmod +x "$T_SCR/main_w.sh"
  for S in TERM INT HUP; do
    sreset; cp "$T_SCR/main_w.sh" "$STUB_DIR/main.sh"; R=$(sroot "s3$S")
    SNAP2_SLEEP=5 "$T_SCR/sigstart.sh" "$R" --op-id "s3-$S" -- sh -c 'w' >"$T_SCR/s3.out" 2>&1 &
    P=$!
    for i in $(seq 1 100); do [ -e "$STUB_DIR/aftersnap.started" ] && break; sleep 0.1; done
    if [ -e "$STUB_DIR/aftersnap.started" ] && [ "$P" -gt 1 ]; then kill -s "$S" "$P" 2>/dev/null; else bad "SIG3-$S fixture: the after-snapshot never started"; fi
    wait "$P"; arc=$?; J="$R/.audit/register/journal.jsonl"; N=$(kill -l "$S")
    if [ "$(jrow "$J" "s3-$S" interrupted)" = "\"$S\"" ] && [ "$arc" = $((128+N)) ] && [ ! -e "$R/.audit/register/pending/s3-$S" ]; then ok "SIG3-$S SIG$S during the after-snapshot: the row IS written (interrupted=$S), the marker is removed, exit $arc (the write had landed: db='$(tail -n1 "$R/docs/workable_items.db")')"
    else bad "SIG3-$S exit=$arc row_interrupted=$(jrow "$J" "s3-$S" interrupted) rows=$(wc -l <"$J" 2>/dev/null) marker=$([ -e "$R/.audit/register/pending/s3-$S" ] && echo kept || echo removed)"; fi
  done
  # a terminal Ctrl-C reaches the whole foreground group: locked.sh AND its after-snapshot helper get the INT; the helper must finish (ids_snapshot stays ok)
  sreset; cp "$T_SCR/main_w.sh" "$STUB_DIR/main.sh"; R=$(sroot s3g)
  SNAP2_SLEEP=5 "$T_SCR/sigstart.sh" "$R" --op-id s3-grp -- sh -c 'w' >"$T_SCR/s3g.out" 2>&1 &
  P=$!
  for i in $(seq 1 100); do [ -e "$STUB_DIR/aftersnap.started" ] && break; sleep 0.1; done
  D=$(descendants "$P" | tr '\n' ' '); kill -s INT "$P" 2>/dev/null; for d in $D; do [ "$d" -gt 1 ] && kill -s INT "$d" 2>/dev/null; done
  wait "$P"; arc=$?; J="$R/.audit/register/journal.jsonl"
  SIGIGN=$(cat "$STUB_DIR/aftersnap.sigign" 2>/dev/null); INTIGN=$(python3 -I -c 'import sys;print(1 if int(sys.argv[1] or "0",16)>>1&1 else 0)' "$SIGIGN")   # the helper process itself ignores SIGINT (bit 2 of SigIgn): deterministic proof of the shield, whatever the timing
  if [ -n "$D" ] && [ "$INTIGN" = 1 ] && [ "$(jrow "$J" s3-grp ids_snapshot)" = '"ok"' ] && [ "$(jrow "$J" s3-grp interrupted)" = '"INT"' ]; then ok "SIG3-G INT delivered to locked.sh AND its helper processes (a terminal Ctrl-C): the after-snapshot finishes, ids_snapshot=ok, row written (helpers: $D)"
  else bad "SIG3-G sigign_int=$INTIGN exit=$arc helpers=[$D] ids_snapshot=$(jrow "$J" s3-grp ids_snapshot) interrupted=$(jrow "$J" s3-grp interrupted)"; fi
  # golden-FALSE: no signal, the same call ends 0 with a row and no marker
  sreset; cp "$T_SCR/main_w.sh" "$STUB_DIR/main.sh"; R=$(sroot s3c); SNAP2_SLEEP=1 slk "$R" --op-id s3-ctl -- sh -c 'w' >/dev/null 2>&1; rc=$?
  assert_eq "SIG3-C golden-FALSE: no signal -> exit 0, one row, no marker, no interrupted field" "$rc/$(jrow "$R/.audit/register/journal.jsonl" s3-ctl interrupted)/$([ -e "$R/.audit/register/pending/s3-ctl" ] && echo m || echo n)" '0/null/n'
fi

# ================= STATUS (I2, stub): the exit status is the container's `died` event, never the dead client's, never a guess =================
if want STATUS; then echo "== STATUS exit status from the died event (stub podman; the real-podman twin is REALSTAT) =="
  sreset; usemain; : >"$STUB_DIR/wait.fail"; R=$(sroot st1)
  CTR_SLEEP=2 CTR_RC=5 STUB_CLIENT_RC=141 slk "$R" --op-id st1 -- sh -c 'w' >/dev/null 2>&1; rc=$?; J="$R/.audit/register/journal.jsonl"
  assert_eq "ST-1 podman wait loses the container (--rm removed it): exit = the container's 5 from its died event, row exit=5 source=container_event client_exit=141" "$rc/$(jrow "$J" st1 exit)/$(jrow "$J" st1 container_exit_source)/$(jrow "$J" st1 client_exit)" '5/5/"container_event"/141'
  sreset; usemain; : >"$STUB_DIR/events.fail"; : >"$STUB_DIR/wait.fail"; R=$(sroot st2)
  CTR_SLEEP=1 CTR_RC=5 STUB_CLIENT_RC=141 LOCKED_EVENT_WAIT=2 slk "$R" --op-id st2 -- sh -c 'w' >/dev/null 2>&1; rc=$?; J="$R/.audit/register/journal.jsonl"
  assert_eq "ST-2 neither the died event nor podman wait can be read: exit 22 (not the client's 141, not a guess), source=unknown, the pending marker is kept" "$rc/$(jrow "$J" st2 container_exit_source)/$([ -e "$R/.audit/register/pending/st2" ] && echo kept || echo gone)" '22/"unknown"/kept'
  sreset; usemain; : >"$STUB_DIR/wait.fail"; echo $(( $(date +%s) + 5 )) >"$STUB_DIR/events.notbefore"; R=$(sroot st6)
  CTR_SLEEP=1 CTR_RC=5 STUB_CLIENT_RC=141 slk "$R" --op-id st6 -- sh -c 'w' >/dev/null 2>&1; rc=$?; J="$R/.audit/register/journal.jsonl"
  assert_eq "ST-6 the events backend LAGS (nothing readable for 5 s after the container ended; observed on a busy host) and podman wait lost the container: the status is waited for, not given up: exit 5, source=container_event" "$rc/$(jrow "$J" st6 container_exit_source)" '5/"container_event"'
  sreset; usemain; : >"$STUB_DIR/events.fail"; R=$(sroot st7)
  CTR_SLEEP=1 CTR_RC=5 STUB_CLIENT_RC=141 LOCKED_EVENT_WAIT=2 slk "$R" --op-id st7 -- sh -c 'w' >/dev/null 2>&1; rc=$?; J="$R/.audit/register/journal.jsonl"
  assert_eq "ST-7 the died event cannot be read but podman wait answered 5 while the container ended: exit 5, source=container_wait, the marker is removed" "$rc/$(jrow "$J" st7 container_exit_source)/$([ -e "$R/.audit/register/pending/st7" ] && echo kept || echo gone)" '5/"container_wait"/gone'
  sreset; usemain; R=$(sroot st3)
  STUB_CLIENT_RC=3 slk "$R" --op-id st3 -- sh -c 'w' >/dev/null 2>&1; rc=$?; J="$R/.audit/register/journal.jsonl"
  assert_eq "ST-3 golden-FALSE: no container ever died under the label (RUNP failed before it): the client's own status 3 stands, source=client, marker removed" "$rc/$(jrow "$J" st3 container_exit_source)/$([ -e "$R/.audit/register/pending/st3" ] && echo kept || echo gone)" '3/"client"/gone'
  sreset; usemain; R=$(sroot st8)
  STUB_CLIENT_RC=143 slk "$R" --op-id st8 -- sh -c 'w' >/dev/null 2>&1; rc=$?; J="$R/.audit/register/journal.jsonl"
  assert_eq "ST-8 the client died of a signal (143), no container was ever seen and no died event exists: its own status stands, flagged source=client_unverified, marker removed (nothing is running)" "$rc/$(jrow "$J" st8 container_exit_source)/$([ -e "$R/.audit/register/pending/st8" ] && echo kept || echo gone)" '143/"client_unverified"/gone'
  sreset; usemain; printf 'ctr-other1 9\nctr-other2 8\n' >"$STUB_DIR/events.out"; R=$(sroot st4)
  STUB_CLIENT_RC=141 slk "$R" --op-id st4 -- sh -c 'w' >/dev/null 2>&1; rc=$?
  assert_eq "ST-4 two died events of unseen containers (another checkout, same op id) and a client that left with 141: ambiguous, exit 22, never one of the two guessed" "$rc/$(jrow "$R/.audit/register/journal.jsonl" st4 container_exit_source)" '22/"unknown"'
  sreset; usemain; printf 'ctr-other1 9\nctr-other2 8\n' >"$STUB_DIR/events.out"; R=$(sroot st4b)
  STUB_CLIENT_RC=0 slk "$R" --op-id st4b -- sh -c 'w' >/dev/null 2>&1; rc=$?
  assert_eq "ST-4b the same ambiguity with a client that ended 0 (it saw its container end 0): exit 0, source=client" "$rc/$(jrow "$R/.audit/register/journal.jsonl" st4b container_exit_source)" '0/"client"'
  sreset; usemain; R=$(sroot st5); CTR_SLEEP=2 CTR_RC=5 STUB_CLIENT_RC=141 slk "$R" --op-id st5 -- sh -c 'w' >/dev/null 2>&1 &
  for i in $(seq 1 60); do [ -s "$STUB_DIR/ctr.st5.pid" ] && break; sleep 0.1; done
  printf 'ctr-foreign 9\nctr-st5 5\n' >"$STUB_DIR/events.out"; wait; rc=$?
  assert_eq "ST-5 the container seen by the drain is the one whose died event counts: ctr-st5 5 is taken, the foreign ctr-foreign 9 is ignored" "$(jrow "$R/.audit/register/journal.jsonl" st5 exit)/$(jrow "$R/.audit/register/journal.jsonl" st5 container_exit_source)" '5/"container_event"'
fi

# ================= FENCE4 (I1 M2 M3 M4, stub): the fence proves the writer's CLIENT and CONTAINER gone, under the lock =================
if want FENCE4; then echo "== FENCE4 marker lock (client) + ps (container) =="
  mkmarker() { mkdir -p "$1/.audit/register/pending"; printf '{"op_id":"%s","pid":4194000}\n' "$2" >"$1/.audit/register/pending/$2"; }
  # F4-1: a live process whose command line merely says locked.sh is NOT an owner (M2: the cmdline proxy is gone); nothing is locked, no container: no wait
  sreset; usemain; R=$(sroot f41); mkmarker "$R" carrier; bash -c 'exec -a locked.sh sleep 20' & CAR=$!; sleep 0.3
  printf '{"op_id":"carrier","pid":%s}\n' "$CAR" >"$R/.audit/register/pending/carrier"
  t0=$(date +%s); STUB_CLIENT_RC=0 slk "$R" --op-id f41 -- sh -c 'w' >/dev/null 2>&1; rc=$?; dt=$(( $(date +%s)-t0 ))
  [ $rc -eq 0 ] && [ $dt -lt 5 ] && ok "F4-1 golden-FALSE: a marker whose pid is a live carrier ('locked.sh' in its cmdline) with NO lock held and no container is not waited for (${dt}s)" || bad "F4-1 rc=$rc dt=${dt}s"
  killpid "$CAR"; wait "$CAR" 2>/dev/null
  # F4-2: the marker's lock is held (an orphaned podman client still runs): the next writer waits until it is released
  sreset; usemain; R=$(sroot f42); mkmarker "$R" held; flock "$R/.audit/register/pending/held" sleep 5 & HOLD=$!; sleep 0.4
  t0=$(date +%s); STUB_CLIENT_RC=0 slk "$R" --op-id f42 -- sh -c 'w' >/dev/null 2>&1; rc=$?; dt=$(( $(date +%s)-t0 ))
  [ $rc -eq 0 ] && [ $dt -ge 4 ] && ok "F4-2 the marker's flock is held by an orphaned client (no container exists yet): the new writer waited for its release (${dt}s)" || bad "F4-2 rc=$rc dt=${dt}s (a held marker lock must delay the next writer)"
  wait "$HOLD" 2>/dev/null
  # F4-3 (reviewer MX2): podman cannot answer while a marker exists: the writer is refused, the writer call is never made
  sreset; usemain; R=$(sroot f43); mkmarker "$R" dead3; : >"$STUB_DIR/podman.fail"
  out=$(STUB_CLIENT_RC=0 slk "$R" --op-id f43 -- sh -c 'w' 2>&1); rc=$?
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=writer_state_unverifiable' && ! grep -q ' f43$' "$STUB_DIR/ov.log" 2>/dev/null && ok "F4-3 podman unavailable with a pending marker: 20 writer_state_unverifiable, no writer call (a failed lookup is not 'no container')" || bad "F4-3 rc=$rc [$out]"
  # F4-4 (reviewer M4 / exp5 shape, adapted to the marker lock): B starts BEFORE A holds the lock or has a marker, so a fence taken before the lock sees nothing; A is then SIGKILLed with
  # its client attached and its container running; B gets the lock and must still wait for A's writer
  sreset; cp "$T_SCR/main_early.sh" "$STUB_DIR/main.sh"; R=$(sroot f44)
  echo '{"run_id":"OTHER","pid":4194000}' >"$R/.audit/commit_turn.json"
  printf '#!/bin/sh\nsleep 2\nrm -f .audit/commit_turn.json\n' >"$STUB_DIR/reaper.sh"; chmod +x "$STUB_DIR/reaper.sh"
  ( CPA_HOST_ENTRY="$STUB_DIR/reaper.sh" CTR_DELAY=0 CTR_SLEEP=5 CLIENT_ATTACH=8 exec env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" bash "$LOCKED" --op-id f44a -- sh -c 'w' >"$T_SCR/f44a.out" 2>&1 ) & PA=$!
  sleep 0.5
  ( CPA_HOST_ENTRY="$STUB_DIR/reaper.sh" CTR_DELAY=0 CTR_SLEEP=1 CLIENT_ATTACH=0 exec env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" bash "$LOCKED" --op-id f44b -- sh -c 'w' >"$T_SCR/f44b.out" 2>&1; echo $? >"$T_SCR/f44b.rc" ) & PB=$!
  for i in $(seq 1 100); do [ -s "$STUB_DIR/ctr.f44a.pid" ] && break; sleep 0.1; done
  sleep 0.3; [ "$PA" -gt 1 ] && kill -9 "$PA" 2>/dev/null; wait "$PA" 2>/dev/null; wait "$PB" 2>/dev/null
  ovorder "$STUB_DIR/ov.log" ctr-f44a f44b && ok "F4-4 B started before A had a marker and A was SIGKILLed with its container running: B's write came after A's container ended (the fence runs under the lock)" || bad "F4-4 overlap: $(tr '\n' ' ' <"$STUB_DIR/ov.log") rc=$(cat "$T_SCR/f44b.rc" 2>/dev/null)"
  killpid "$(cat "$STUB_DIR/ctr.f44a.pid" 2>/dev/null)"
  # F4-5 (I1): A is SIGKILLed while its client is alive but its container is not created yet (the container appears 3 s later): the fence must still wait
  sreset; cp "$T_SCR/main_early.sh" "$STUB_DIR/main.sh"; R=$(sroot f45)
  ( CTR_DELAY=3 CTR_SLEEP=4 CLIENT_ATTACH=8 exec env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" bash "$LOCKED" --op-id f45a -- sh -c 'w' >"$T_SCR/f45a.out" 2>&1 ) & PA=$!
  for i in $(seq 1 300); do [ -e "$R/.audit/register/pending/f45a" ] && grep -q ' op=f45a argv=' "$STUB_DIR/runp.log" 2>/dev/null && break; sleep 0.02; done   # the marker exists AND the MAIN call of the stub RUNP has started (' op=f45a argv=' is the main call; ' op=f45a-snap argv=' is the snapshot, logged before the marker): the client is alive, its container is 3 s away
  [ ! -s "$STUB_DIR/ctr.f45a.pid" ] && ok "F4-5a control needle: at the kill the container of A does not exist yet (the I1 window is really hit)" || bad "F4-5a the container already existed at the kill: the window was missed"
  [ "$PA" -gt 1 ] && kill -9 "$PA" 2>/dev/null; wait "$PA" 2>/dev/null
  STUB_CLIENT_RC=0 CTR_DELAY=0 CTR_SLEEP=1 CLIENT_ATTACH=0 slk "$R" --op-id f45b -- sh -c 'w' >"$T_SCR/f45b.out" 2>&1; rc=$?
  ovorder "$STUB_DIR/ov.log" ctr-f45a f45b && ok "F4-5 SIGKILL of A in the client-alive / container-not-yet-created window: B's write came after A's container ended (rc $rc)" || bad "F4-5 overlap: $(tr '\n' ' ' <"$STUB_DIR/ov.log") rc=$rc"
  killpid "$(cat "$STUB_DIR/ctr.f45a.pid" 2>/dev/null)"
fi

# ================= SCOPE (M1, stub): a container of another checkout with the same op id is neither drained nor fenced =================
if want SCOPE; then echo "== SCOPE the /src mount source scopes the container lookup to this root =="
  sreset; usemain; R=$(sroot sc1); fakectr other 7 0; echo /nonexistent/other/checkout >"$STUB_DIR/ctr.other.src"
  t0=$(date +%s); STUB_CLIENT_RC=0 slk "$R" --op-id other -- sh -c 'w' >/dev/null 2>&1; rc=$?; dt=$(( $(date +%s)-t0 ))
  assert_eq "SC-1 a container labelled with this op id whose /src is ANOTHER checkout is not drained: no wait (${dt}s), container_drain=none" "$rc/$([ $dt -lt 5 ] && echo fast || echo slow)/$(jrow "$R/.audit/register/journal.jsonl" other container_drain)" '0/fast/"none"'
  killpid "$(cat "$STUB_DIR/ctr.other.pid" 2>/dev/null)"; wait 2>/dev/null
  sreset; usemain; R=$(sroot sc2); fakectr mine 6 0; realpath "$R" >"$STUB_DIR/ctr.mine.src"
  t0=$(date +%s); STUB_CLIENT_RC=0 slk "$R" --op-id mine -- sh -c 'w' >/dev/null 2>&1; rc=$?; dt=$(( $(date +%s)-t0 ))
  assert_eq "SC-2 golden-FALSE: the same container with /src = THIS root IS drained (waited ${dt}s)" "$rc/$([ $dt -ge 5 ] && echo waited || echo fast)/$(jrow "$R/.audit/register/journal.jsonl" mine container_drain)" '0/waited/"waited"'
  wait 2>/dev/null
fi

# ================= BSIG (M5 M7): backup_db.sh =================
if want BSIG; then echo "== BSIG backup_db.sh: an interrupted backup leaves no unverified file; the empty-checkpoint refusal says what happened =="
  cat >"$T_SCR/locked_stub.sh" <<'S'
#!/usr/bin/env bash
# stub locked.sh for backup_db.sh. BSTUB=hang1 hangs in step 1, hang2 hangs in step 2, empty answers step 1 with an empty checkpoint row
op=""; out=""; while [ $# -gt 0 ]; do case "$1" in --op-id) op="$2"; shift 2;; --out) out="$2"; shift 2;; --) shift; break;; *) shift;; esac; done
if [ -n "$out" ]; then [ "${BSTUB:-}" = hang2 ] && exec sleep 60; exit 0; fi
[ "${BSTUB:-}" = hang1 ] && exec sleep 60
for a in "$@"; do case "$a" in ".backup /src/"*) bak="${a#.backup /src/}";; esac; done
cp docs/workable_items.db "$bak"; mkdir -p ".audit/out/$op"; echo 'COMMIT;' >".audit/out/$op/source.dump.sql"; printf '%s  x\n' "$(printf 'a%.0s' $(seq 64))" >".audit/out/$op/source.sha256"
[ "${BSTUB:-}" = empty ] || echo "0|0|0"
exit 0
S
  chmod +x "$T_SCR/locked_stub.sh"
  for V in hang1 hang2; do
    R=$(mkroot "bs$V"); echo "db" >"$R/docs/workable_items.db"
    env BSTUB=$V LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED="$T_SCR/locked_stub.sh" bash "$BACKUP" --record "$T_SCR/bs$V.rec" >"$T_SCR/bs$V.out" 2>&1 & BP=$!
    for i in $(seq 1 100); do [ "$V" = hang1 ] && [ -n "$(ls "$R"/docs/workable_items.db.bak-* 2>/dev/null)" ] && break; [ "$V" = hang2 ] && ls "$R"/.audit/out/*-verify >/dev/null 2>&1 && break; sleep 0.1; done
    sleep 0.3; before=$(ls "$R"/docs/workable_items.db.bak-* 2>/dev/null | wc -l)
    [ "$BP" -gt 1 ] && kill -s TERM "$BP" 2>/dev/null; wait "$BP"; rc=$?; left=$(ls "$R"/docs/workable_items.db.bak-* 2>/dev/null | wc -l)
    if [ "$before" = 1 ] && [ "$left" = 0 ] && [ "$rc" = 143 ] && [ ! -e "$T_SCR/bs$V.rec" ]; then ok "BS-$V SIGTERM while the backup is in progress ($V): the unverified backup file is removed (was $before, left $left), no record, exit $rc"
    else bad "BS-$V (needle: backup file existed at the signal=$before) left=$left rc=$rc rec=$([ -e "$T_SCR/bs$V.rec" ] && echo y || echo n) out=[$(head -c 200 "$T_SCR/bs$V.out")]"; fi
  done
  R=$(mkroot bsempty); echo "db" >"$R/docs/workable_items.db"
  out=$(env BSTUB=empty LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED="$T_SCR/locked_stub.sh" bash "$BACKUP" --record "$T_SCR/bse.rec" 2>&1); rc=$?
  [ $rc -ne 0 ] && printf '%s' "$out" | grep -q 'EMPTY result row' && ! printf '%s' "$out" | grep -q 'another connection holds the database' && [ "$(ls "$R"/docs/workable_items.db.bak-* 2>/dev/null | wc -l)" = 0 ] && ok "BS-empty an EMPTY checkpoint row is refused saying it was empty (not 'another connection holds the database'), no file left" || bad "BS-empty rc=$rc [$out]"
fi

# ================= M6: replay.sh skips are keyed on the exact shape of the real rows =================
if want M6; then echo "== M6 replay: marker-shaped and install-shaped rows that changed the register are never skipped with an OK verdict =="
  H="$T_SCR/rh"; mkdir -p "$H/root/docs" "$H/stubs"; echo "x" >"$H/root/docs/workable_items.db"
  cat >"$H/stubs/backup.sh" <<'S'
#!/usr/bin/env bash
cp docs/workable_items.db docs/bak; s=$(sha256sum docs/bak | cut -d' ' -f1); printf '{"backup_path":"docs/bak","backup_sha256":"%s","source_sha256":"%s"}' "$s" "$s" > "$2"
S
  cat >"$H/stubs/locked.sh" <<'S'
#!/usr/bin/env bash
echo "$*" >> "$HARNESS_LOG"
case "$*" in *"select atm_id from reg_ids"*) n=$(cat "$HARNESS_IDS.n" 2>/dev/null || echo 0); echo $((n+1)) >"$HARNESS_IDS.n"; if [ "$n" = 0 ]; then cat "$HARNESS_IDS"; else cat "$HARNESS_IDS.final"; fi;; *gate.sh*) echo "GATE OK";; esac; exit 0
S
  printf '#!/bin/sh\nexit 0\n' >"$H/stubs/noop.sh"; chmod +x "$H/stubs/"*.sh
  A=$(printf 'a%.0s' $(seq 64)); B=$(printf 'b%.0s' $(seq 64)); C=$(printf 'c%.0s' $(seq 64))
  mkj() { mkdir -p "$H/j/$1"; python3 -I -c '
import json,sys
a,b,c=sys.argv[2:5]
def row(op,mode,db,bs,as_,minted,argv,ex=0,snap="ok",wal=0,seq=None,**kw):
    r={"op_id":op,"mode":mode,"db":db,"db_sha_before":bs,"db_sha_after":as_,"ids_minted":minted,"argv":argv,"exit":ex,"input_args":{},"inputs":[],"wal_bytes_after":wal,"ids_snapshot":snap}
    if seq is not None: r["seq"]=seq
    r.update(kw); return r
REG="docs/workable_items.db"; M=["sqlite3","/src/"+REG,"MINT"]; MK="# register-regenerate:dump\nset -eo pipefail"
rows=eval(sys.stdin.read())
open(sys.argv[1]+"/journal.jsonl","w").write("".join((l if isinstance(l,str) else json.dumps(l))+"\n" for l in rows))' "$H/j/$1" "$A" "$B" "$C"; }
  rp() { local n=$1 ids=$2 fin="${3:-CAT-001 CAT-002 CAT-003}"; printf '%s\n' "$ids" >"$H/ids_$n.txt"; printf '%s\n' "$fin" >"$H/ids_$n.txt.final"; rm -f "$H/ids_$n.txt.n"; rm -rf "$H/out_$n"
    local o; o=$(env LOCKED_TEST_MODE=1 LOCKED_ROOT="$H/root" LOCKED="$H/stubs/locked.sh" BACKUP="$H/stubs/backup.sh" DUMP="$H/stubs/noop.sh" EXPORT="$H/stubs/noop.sh" HARNESS_LOG="$H/log_$n" HARNESS_IDS="$H/ids_$n.txt" bash "$REPLAY" --onto docs/workable_items.db --since "$A" --out "$H/out_$n" --journal "$H/j/$n/journal.jsonl" 2>&1); echo "rc=$? $o" | cut -c1-420; }
  chk() { local name=$1 r=$2 want=$3 label=$4; case "$r" in $want) ok "$name $label";; *) bad "$name $label :: [$r]";; esac; }
  mkj hand <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("HANDMINT","register",REG,a,b,["CAT-002"],["bash","-c",MK+"\nsqlite3 /src/"+REG+" 'INSERT INTO reg_ids ...'"])]
E
  chk M6-1 "$(rp hand 'CAT-001')" 'rc=0 replay: OK replayed=1 skipped=0 ids_kept=CAT-002 *' "a bash -c row that starts with the marker but MINTED an id is a real write: replayed, its id kept (never dropped as 'regenerated')"
  mkj fregen <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("FAILEDREGEN","register",REG,a,b,["CAT-003"],["sqlite3","/src/"+REG,MK],ex=1)]
E
  chk M6-2 "$(rp fregen 'CAT-001')" 'rc=20 *reason=replay_failed_row_changed_register*FAILEDREGEN*' "a failed row whose marker is a sqlite3 ARGUMENT (not the bash -c script) that minted an id is refused as a failed row that changed the register, not skipped as regenerated"
  mkj inst <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("INSTCH","register",REG,a,b,[],["bash","/src/scripts/register/export.sh","--install"])]
E
  chk M6-3 "$(rp inst 'CAT-001')" 'rc=20 *reason=replay_install_row_changed_register*INSTCH*' "an export.sh --install row that CHANGED the register is refused (export.sh has no such option: the row cannot be a harmless install), nothing is written"
  mkj real <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("REGEN","register",REG,a,b,[],["bash","-c",MK]), row("REGENFAIL","register",REG,b,c,[],["bash","-c",MK],ex=20), row("INST","register",REG,c,c,[],["bash","/src/scripts/register/export.sh","--install"])]
E
  mkj shape <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("ARGMARK","register",REG,a,b,[],["sqlite3","/src/"+REG,MK]), row("MENTBASH","register",REG,b,c,[],["bash","-c","echo done # register-regenerate:fake\nsqlite3 /src/"+REG+" 'UPDATE items SET title=1'"])]
E
  chk M6-5 "$(rp shape 'CAT-001')" 'rc=0 replay: OK replayed=2 skipped=0 *' "a marker that is an argv element of a program that is not bash -c (M6-5), and a bash -c script that only MENTIONS the marker after its first line (M6-6), are real writes: replayed, never skipped"
  chk M6-4 "$(rp real 'CAT-001')" 'rc=0 replay: OK replayed=0 skipped=3 *' "golden-FALSE: the real dump/export row shape (exactly bash -c <marker script>, no id minted, changed or failed) and an UNCHANGED export.sh --install row are still skipped and listed"
fi

# ================= real podman sections =================
if e2e; then
  realroot() { local r; r=$(mkroot "$1"); cdb "$r" workable_items.db || return 1; lk "$r" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1 || return 1; echo "$r"; }
  title_sql() { printf '%s' "sqlite3 /src/docs/workable_items.db \"UPDATE items SET title='$1' WHERE atm_id='CAT-001'\""; }
  ctrs() { podman ps -a -q --filter "label=catalogizer.op_id=$1" 2>/dev/null | wc -l; }
  gettitle() { lk "$1" --out "$T_SCR/q$2" -- sqlite3 -readonly "file:/src/docs/workable_items.db?immutable=1" "select title from items where atm_id='CAT-001'" 2>/dev/null; }
fi

# ================= SWEEP (I1, real podman): SIGKILL of the first writer at offsets across its whole life =================
if want SWEEP && e2e; then echo "== SWEEP real podman: the first writer is SIGKILLed at 10 offsets after its marker exists; the second must never overlap it =="
  R=$(realroot sweep) || { bad "SWEEP setup"; R=""; }
  if [ -n "$R" ]; then pre=0; post=0; overlaps=0; n=0
    for off in 0 0.1 0.3 0.6 1.0 2.0 2.8 3.4 4.0 5.0; do
      n=$((n+1)); OPA="r4sweep-a-$n-$$"; OPB="r4sweep-b-$n-$$"
      ( exec env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$RUNP" PATH="$SHIM_DIR:$PATH" SHIM_LOG="$SHIM_LOG" bash "$LOCKED" --op-id "$OPA" -- sh -c "touch /src/docs/a$n.started; sleep 4; $(title_sql "A-LATE-$n"); touch /src/docs/a$n.finished" ) >"$T_SCR/swa$n.out" 2>&1 & PA=$!
      for i in $(seq 1 500); do [ -e "$R/.audit/register/pending/$OPA" ] && break; sleep 0.01; done
      sleep "$off"
      c=$(ctrs "$OPA"); if [ "$c" = 0 ]; then pre=$((pre+1)); else post=$((post+1)); fi
      [ "$PA" -gt 1 ] && kill -9 "$PA" 2>/dev/null; wait "$PA" 2>/dev/null
      lk "$R" --op-id "$OPB" -- sh -c "$(title_sql "B-$n"); touch /src/docs/b$n.done" >"$T_SCR/swb$n.out" 2>&1; rb=$?
      for i in $(seq 1 100); do [ "$(ctrs "$OPA")" = 0 ] && break; sleep 0.2; done
      T=$(gettitle "$R" "sw$n")
      v=$(python3 -I - "$R/docs" "$n" <<'PY'
import os,sys
d=sys.argv[1]; n=sys.argv[2]
b=os.stat(d+"/b%s.done"%n).st_mtime if os.path.exists(d+"/b%s.done"%n) else None
a0=os.stat(d+"/a%s.started"%n).st_mtime if os.path.exists(d+"/a%s.started"%n) else None
a1=os.stat(d+"/a%s.finished"%n).st_mtime if os.path.exists(d+"/a%s.finished"%n) else None
if b is None: print("B_DID_NOT_WRITE")
elif a0 is None: print("A_NEVER_STARTED")
elif a1 is not None and b < a1: print("OVERLAP")
else: print("ORDERED")
PY
)
      case "$v" in OVERLAP) overlaps=$((overlaps+1)); bad "SWEEP-$n offset=${off}s container-at-kill=$c B rc=$rb final_title=$T :: $v (B wrote while A's container ran)";; *) ok "SWEEP-$n offset=${off}s container-at-kill=$c B rc=$rb final_title=$T :: $v";; esac
    done
    [ "$pre" -ge 1 ] && [ "$post" -ge 1 ] && ok "SWEEP-needle the sweep landed $pre kill(s) BEFORE the container existed and $post AFTER it (both windows of the writer's life were hit: the sweep is not blind)" || bad "SWEEP-needle pre=$pre post=$post: the sweep did not hit both windows, its green says nothing"
    [ "$overlaps" = 0 ] && ok "SWEEP-total 0 overlaps in 10 kill offsets" || bad "SWEEP-total $overlaps overlaps in 10 kill offsets"
  fi
fi

# ================= REALSTAT (I2, real podman): the status of a container that ends while / after the client left =================
if want REALSTAT && e2e; then echo "== REALSTAT real podman, locked.sh | head -n1: the container's own exit status 3 =="
  R=$(realroot rstat) || { bad "REALSTAT setup"; R=""; }
  if [ -n "$R" ]; then n=0; lost=0
    for tail in "sleep 5.0;" "sleep 5.1;" "sleep 5.2;" "sleep 5.3;" "sleep 5.4;" "sleep 5.5;"; do
      n=$((n+1)); rm -f "$T_SCR/rcs$n"
      WR="echo first-line; sleep 1; i=0; while [ \$i -lt 300 ]; do echo filler-\$i; i=\$((i+1)); done; $tail $(title_sql "LATE-$n"); exit 3"
      ( lk "$R" --op-id "rstat-$n" -- sh -c "$WR" 2>"$T_SCR/rse$n"; echo "$?" >"$T_SCR/rcs$n" ) | head -n1 >/dev/null
      for i in $(seq 1 200); do [ -s "$T_SCR/rcs$n" ] && break; sleep 0.1; done
      T=$(gettitle "$R" "rs$n"); J="$R/.audit/register/journal.jsonl"
      src=$(jrow "$J" "rstat-$n" container_exit_source); rowexit=$(jrow "$J" "rstat-$n" exit); lrc=$(cat "$T_SCR/rcs$n" 2>/dev/null)
      if [ "$lrc" = 3 ] && [ "$rowexit" = 3 ] && [ "$T" = "LATE-$n" ]; then ok "REALSTAT-$n the write landed (title $T) and locked.sh exit=$lrc, row exit=$rowexit, source=$src, client_exit=$(jrow "$J" "rstat-$n" client_exit)"
      else lost=$((lost+1)); bad "REALSTAT-$n title=$T locked_rc=$lrc row exit=$rowexit source=$src client_exit=$(jrow "$J" "rstat-$n" client_exit) (the container exited 3)"; fi
    done
    [ "$lost" = 0 ] && ok "REALSTAT-total the container's status 3 was kept in 6 of 6 runs" || bad "REALSTAT-total status lost in $lost of 6 runs"
  fi
fi

# ================= REALSCOPE (M1, real podman): another checkout's container with the same op id =================
if want REALSCOPE && e2e; then echo "== REALSCOPE real podman: two roots, one op id =="
  RX=$(mkroot rsx); RY=$(mkroot rsy); OPS="r4scope-$$"
  lk "$RX" --op-id "$OPS" -- sh -c 'sleep 16' >"$T_SCR/rsx.out" 2>&1 & PX=$!
  for i in $(seq 1 200); do [ "$(ctrs "$OPS")" -ge 1 ] && break; sleep 0.1; done
  c0=$(ctrs "$OPS"); t0=$(date +%s); lk "$RY" --op-id "$OPS" -- true >"$T_SCR/rsy.out" 2>&1; rcy=$?; dt=$(( $(date +%s)-t0 )); c1=$(ctrs "$OPS")
  J="$RY/.audit/register/journal.jsonl"
  [ "$c0" -ge 1 ] && [ "$c1" -ge 1 ] && [ $rcy -eq 0 ] && [ $dt -lt 12 ] && [ "$(jrow "$J" "$OPS" container_drain)" = '"none"' ] && ok "REALSCOPE-1 root Y ('true', same op id) did not drain root X's running container: ${dt}s, container_drain=none, X's container still up ($c1) when Y ended" || bad "REALSCOPE-1 containers before=$c0 after=$c1 rc=$rcy dt=${dt}s drain=$(jrow "$J" "$OPS" container_drain) [$(head -c 200 "$T_SCR/rsy.out")]"
  wait "$PX" 2>/dev/null
  [ "$(jrow "$RX/.audit/register/journal.jsonl" "$OPS" container_drain)" = '"none"' ] && ok "REALSCOPE-2 golden-FALSE: root X's own call ended normally with its own container (drain=none, exit $(jrow "$RX/.audit/register/journal.jsonl" "$OPS" exit))" || bad "REALSCOPE-2 X row: $(jrow "$RX/.audit/register/journal.jsonl" "$OPS" container_drain)"
fi

finish
