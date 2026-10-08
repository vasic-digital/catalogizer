#!/usr/bin/env bash
# test_registry_r5.sh - 11.4.276 round 5 (the structural round after the round-3 commit 350372a8): the tests of the classes the round-3 review found STILL OPEN in scripts/longops. Every test drives the REAL
# scripts on a fixture registry (real processes, real flock, real /proc); none re-implements the guarded logic. Each test was observed RED against 350372a8 (RED transcript committed under
# specs/001-full-project-audit-remediation/evidence/wp08/fix-r5-RED-*.txt) and GREEN x3 on the fix tree. Env LONGOPS_SCRIPTS substitutes a copy of the scripts (the RED run and the mutation runner use it).
# Classes: A (raw-typed inputs, ONE shape), D (producer model: boot id, monotonic clock, reap intent, stop grace), E (write paths of every op), F (owner gone is not workload gone), G (multi-write transitions),
# H (process identity), J (all-time registry scale), K (a flag given as the last token). The sweep side of the same classes is scripts/anti-mess/tests/test_sweep_r5.sh.
. "$(dirname "$0")/lib.sh"
ident_header WP08-R5-registry
export LC_ALL=C
ROOTUID=$(id -u)
# want <section>: the mutation runner runs only the sections that can kill a mutant (R5_SECTIONS=A1,D ...); an unset R5_SECTIONS runs everything
want() { [ -z "${R5_SECTIONS:-}" ] || case ",$R5_SECTIONS," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

# reg1 <fixture> <purpose> [register args]: a fresh fixture, a live owner (A) registered under <purpose>; sets A, ID, REC
reg1() { newfx "$1"; mkll; A=$LL; ID=$("$S/register.sh" --purpose "$2" --owner t --pid "$A" --no-progress-s 600 "${@:3}"); REC=$LONGOPS_DIR/ops/$ID.json; }
sha() { sha256sum "$1" 2>/dev/null | cut -c1-64; }
reg_dead() { newfx "$1"; fpod; mkll; A=$LL; ID=$("$S/register.sh" --purpose "$2" --owner t --pid "$A" --no-progress-s 600); REC=$LONGOPS_DIR/ops/$ID.json; kill "$A"; wait "$A" 2>/dev/null; }

if want A1; then
echo "== class A: ONE record shape; every consumer refuses a record with one bad field (T-A1 matrix, T-A2) =="
MUT=(
  'del(.pid)' '.pid=null' '.pid=false' '.pid=0' '.pid=1' '.pid="abc"' '.pid=[1]' '.pid="08"'
  'del(.start_time)' '.start_time=null' '.start_time=false' '.start_time=""' '.start_time=7' '.start_time="08"' '.start_time={"a":1}'
  'del(.last_progress_epoch)' '.last_progress_epoch=null' '.last_progress_epoch="08"' '.last_progress_epoch=false'
  'del(.last_progress_mono)' '.last_progress_mono="x"'
  'del(.boot_id)' '.boot_id=""' '.boot_id=7'
  '.budget.no_progress_s="010"' '.budget.wall_clock_s="x"' 'del(.budget)' '.budget=null'
  '.elapsed_ms=null' '.elapsed_ms="1"' '.progress_offset=null' '.progress_offset="3"'
  '.state="bogus"' '.state="Complete"' '.state=null' 'del(.state)'
  'del(.run_id)' '.run_id=""' '.run_id=7' '.container_label=7' '.container_label="a\tb"'
  '.write_paths="tracked/a.txt"' 'del(.write_paths)' '.started_utc="yesterday"' '.attached_to=0' '.superseded_by=[]' '.cmdline=null'
)
mi=0
for m in "${MUT[@]}"; do
  mi=$((mi+1)); reg1 "ma$mi" "ma:$mi"; jq -c "$m" "$REC" >"$FXN/m.json" && cp "$FXN/m.json" "$REC"; h0=$(sha "$REC"); : >"$LONGOPS_DIR/signals.log"
  r=$("$S/classify.sh" 2>/dev/null | cut -f2); assert_eq "A1.$mi classify of [$m] is unreadable" "$r" unreadable
  "$S/reap.sh" --op-id "$ID" >/dev/null 2>&1; assert_rc "A1.$mi reap --op-id of [$m] refuses (20)" $? 20
  "$S/check_no_build_writing_tracked.sh" >/dev/null 2>&1; assert_rc "A1.$mi check_no_build_writing_tracked of [$m] blocks (1)" $? 1
  kill -0 "$A" 2>/dev/null && [ ! -s "$LONGOPS_DIR/signals.log" ] && [ "$(sha "$REC")" = "$h0" ] && [ -d "$LONGOPS_DIR/claims/ma:$mi" ] && ok "A1.$mi [$m]: owner alive, no signal, record and claim untouched" || bad "A1.$mi [$m]: owner/signal/record/claim changed"
  kill "$A" 2>/dev/null; wait "$A" 2>/dev/null
  "$S/reap.sh" --purpose "ma:$mi" >/dev/null 2>&1; assert_rc "A1.$mi reap --purpose of [$m] with a DEAD holder refuses (20): an unjudgeable record of the registry is never 'no live op'" $? 20
  [ -d "$LONGOPS_DIR/claims/ma:$mi" ] && ok "A1.$mi the claim survived" || bad "A1.$mi the claim was released"
done
reg1 ma0 ma:0; assert_eq "A1.0 control: the unmodified record is advancing" "$("$S/classify.sh" --op-id "$ID" | cut -f2)" advancing
"$S/reap.sh" --op-id "$ID" >/dev/null 2>&1; assert_rc "A1.0b control: reaping the advancing op is refused (5)" $? 5
"$S/check_no_build_writing_tracked.sh" >/dev/null 2>&1; assert_rc "A1.0c control: no tracked path declared: exit 0" $? 0
kill "$A"; wait "$A" 2>/dev/null; "$S/reap.sh" --purpose ma:0 >/dev/null 2>&1; assert_rc "A1.0d control: a dead holder with its dead-owner op is released (0)" $? 0
# T-A2 out-of-set state with a DEAD pid: never rewritten into `reaped`
reg1 ma9 ma:9; kill "$A"; wait "$A" 2>/dev/null
for stv in bogus Complete RUNNING; do jq -c --arg s "$stv" '.state=$s' "$REC" >"$FXN/m.json" && cp "$FXN/m.json" "$REC"; h0=$(sha "$REC")
  assert_eq "A2 state '$stv' with a dead pid classifies unreadable" "$("$S/classify.sh" --op-id "$ID" | cut -f2)" unreadable
  "$S/reap.sh" --op-id "$ID" >/dev/null 2>&1; assert_rc "A2b reap --op-id of state '$stv' (dead pid) refuses (20)" $? 20
  assert_eq "A2c the record is byte-identical (a corrupt record is never rewritten as reaped)" "$(sha "$REC")" "$h0"
done
fi
if want A; then
echo "-- leading zeros and canonical integers (LO-A7) --"
newfx lz; mkll
"$S/register.sh" --purpose lz:a --owner t --pid "$LL" --no-progress-s 08 >/dev/null 2>&1; assert_rc "A7 register --no-progress-s 08 is a usage error (2)" $? 2
"$S/register.sh" --purpose lz:b --owner t --pid "$LL" --wall-s 010 >/dev/null 2>&1; assert_rc "A7b register --wall-s 010 is a usage error (2)" $? 2
assert_eq "A7c neither left a record or a claim" "$(ls "$LONGOPS_DIR/ops" 2>/dev/null | wc -l):$(ls "$LONGOPS_DIR/claims" 2>/dev/null | wc -l)" "0:0"
reg1 lz2 lz:c
"$S/heartbeat.sh" --op-id "$ID" --progress-offset 010 >/dev/null 2>&1; assert_rc "A7d heartbeat --progress-offset 010 is a usage error (2)" $? 2
"$S/heartbeat.sh" --op-id "$ID" --elapsed-ms 007 >/dev/null 2>&1; assert_rc "A7e heartbeat --elapsed-ms 007 is a usage error (2)" $? 2
LONGOPS_NOW=0123 "$S/classify.sh" >/dev/null 2>&1; assert_rc "A7f LONGOPS_NOW=0123 is refused at load (2)" $? 2
LONGOPS_REAP_GRACE_S=08 "$S/reap.sh" --op-id "$ID" >/dev/null 2>&1; assert_rc "A7g LONGOPS_REAP_GRACE_S=08 is a usage error (2)" $? 2
( . "$S/lib.sh"; r=""; for v in 0 7 100000000000000 08 010 00 "" -1 1.5 abc 1000000000000000; do lo_uint "$v"; r="$r$? "; done; echo "$r" ) >"$FXN/o"
assert_eq "A7h lo_uint accepts 0, 7 and 15 digits, refuses a leading zero, an empty value, a sign, a fraction, junk and 16 digits" "$(cat "$FXN/o")" "0 0 0 1 1 1 1 1 1 1 1 "
echo "-- holder files: absent is not unreadable (LO-A5, LO-A6) --"
if [ "$ROOTUID" = 0 ]; then ok "A4 skipped: running as root, mode 000 is readable (UNCONFIRMED here, 11.4.3)"; else
  newfx hf; mkll; A=$LL; "$S/acquire.sh" --purpose hf:x --run-id r1 --pid "$A" >/dev/null; HF=$LONGOPS_DIR/claims/hf:x/holder.json; cp "$HF" "$FXN/hgood"
  for kind in empty mode000 directory; do
    rm -rf "$HF"; case "$kind" in empty) : >"$HF" ;; mode000) cp "$FXN/hgood" "$HF"; chmod 000 "$HF" ;; directory) mkdir "$HF" ;; esac
    "$S/holder.sh" hf:x >/dev/null 2>&1; assert_rc "A4 holder.sh on a holder that is $kind refuses (20), never none" $? 20
    "$S/reap.sh" --purpose hf:x >/dev/null 2>&1; assert_rc "A4b reap --purpose on a holder that is $kind refuses (20), the claim of a LIVE holder is not released" $? 20
    "$S/acquire.sh" --purpose hf:x --run-id r2 --pid "$$" >/dev/null 2>&1; assert_rc "A4c a second acquire over a holder that is $kind refuses (20), not stale_claim (4)" $? 20
    [ -d "$LONGOPS_DIR/claims/hf:x" ] && ok "A4d the claim survives ($kind)" || bad "A4d the claim was released ($kind)"
  done
  chmod 644 "$HF" 2>/dev/null; rm -rf "$HF"
fi
newfx hg; mkll; A=$LL; "$S/acquire.sh" --purpose hg:x --run-id r1 --pid "$A" >/dev/null; mkdir -p "$LONGOPS_AUDIT/builds/b1"
"$S/acquire.sh" --suspend r1 --purpose hg:x --builds b1 >/dev/null; cp "$LONGOPS_DIR/claims/hg:x/holder.json" "$FXN/sgood"
for fld in 'del(.builds)' '.builds=null' '.builds=false' '.builds=[1]' '.state="bogus"' 'del(.state)' '.callback_state="bogus"'; do
  jq -c "$fld" "$FXN/sgood" >"$LONGOPS_DIR/claims/hg:x/holder.json"
  "$S/holder.sh" hg:x >/dev/null 2>&1; assert_rc "A5 a suspended-run holder with [$fld] is unreadable (20): a run with LIVE builds is not released as stale" $? 20
  "$S/reap.sh" --purpose hg:x >/dev/null 2>&1; assert_rc "A5b reap --purpose does not release it (20)" $? 20
done
jq -c '.state="ready_to_resume"|del(.ready_at)' "$FXN/sgood" >"$LONGOPS_DIR/claims/hg:x/holder.json"; mkdir -p "$LONGOPS_AUDIT/builds/b1/terminal"
export CPA_APPROVED_DIR=$FXN/approved; mkdir -p "$FXN/approved/scripts/repo"; printf 'resume_ttl=60\n' >"$FXN/approved/scripts/repo/commit_push.conf"
"$S/acquire.sh" --expire hg:x --op-id e1 >/dev/null 2>&1; assert_rc "A5c --expire on a ready_to_resume holder with NO ready_at refuses (20), it is never 'expired at epoch 0'" $? 20
[ -d "$LONGOPS_DIR/claims/hg:x" ] && ok "A5d the claim of the run survives --expire" || bad "A5d the live claim was removed"
unset CPA_APPROVED_DIR
echo "-- require_verdicts: a verdict without a fresh machine utc never passes (LO-A9) --"
newfx rv; export LONGOPS_NOW=$(date +%s); now=$(date -u -d "@$LONGOPS_NOW" +%Y-%m-%dT%H:%M:%SZ)
while IFS='|' read -r name body; do printf '%s\n' "$body" >"$FXN/v-$name.json"; "$S/require_verdicts.sh" --file "$FXN/v-$name.json" --max-age-s 86400 >"$FXN/o" 2>&1; assert_rc "A9 verdict with utc [$name] is refused at a day-sized --max-age-s (1)" $? 1
done <<EOF
absent|{"verdict":"PASS","fingerprint":"f"}
empty|{"verdict":"PASS","fingerprint":"f","utc":""}
null|{"verdict":"PASS","fingerprint":"f","utc":null}
now|{"verdict":"PASS","fingerprint":"f","utc":"now"}
nextyear|{"verdict":"PASS","fingerprint":"f","utc":"next year"}
plusday|{"verdict":"PASS","fingerprint":"f","utc":"$(date -u -d "@$((LONGOPS_NOW+86400))" +%Y-%m-%dT%H:%M:%SZ)"}
number|{"verdict":"PASS","fingerprint":"f","utc":$LONGOPS_NOW}
EOF
echo "{\"verdict\":\"PASS\",\"fingerprint\":\"f\",\"utc\":\"$(date -u -d "@$((LONGOPS_NOW-3600))" +%Y-%m-%dT%H:%M:%SZ)\"}" >"$FXN/v-hour.json"
"$S/require_verdicts.sh" --file "$FXN/v-hour.json" --max-age-s 86400 --fingerprint f >/dev/null 2>&1; assert_rc "A9b control: a verdict one hour old at --max-age-s 86400 passes (0)" $? 0
"$S/require_verdicts.sh" --file "$FXN/v-hour.json" --fingerprint "" >/dev/null 2>&1; assert_rc "A9c an explicit empty --fingerprint is a usage error (2), never 'no check'" $? 2
"$S/require_verdicts.sh" --file "$FXN/v-hour.json" --max-age-s "" >/dev/null 2>&1; assert_rc "A9d an explicit empty --max-age-s is a usage error (2)" $? 2
unset LONGOPS_NOW

fi
if want D; then
echo "== class D: boot id, monotonic clock, reap intent, stop grace (the producers' real behaviour) =="
reg1 d6 d6:x; jq -c '.boot_id="00000000-0000-0000-0000-000000000000"' "$REC" >"$FXN/m.json" && cp "$FXN/m.json" "$REC"
assert_eq "D6 a record of ANOTHER boot whose pid and start ticks match a live process is dead_owner (a pid is only meaningful inside one boot)" "$("$S/classify.sh" --op-id "$ID" | cut -f2)" dead_owner
reg1 d6b d6:y; jq -c '.boot_id="00000000-0000-0000-0000-000000000000"' "$LONGOPS_DIR/claims/d6:y/holder.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/claims/d6:y/holder.json"
assert_eq "D6b a process holder of another boot reads none" "$("$S/holder.sh" d6:y)" none
reg1 d5 d5:x; export LONGOPS_NOW=$(date +%s) LONGOPS_MONO=1000
id5=$("$S/register.sh" --purpose d5:z --owner t --pid "$A" --no-progress-s 600); "$S/heartbeat.sh" --op-id "$id5" --progress-offset 5 >/dev/null
export LONGOPS_NOW=$(( $(date +%s) + 7200 )) LONGOPS_MONO=1005
assert_eq "D5 a wall clock stepped 2 h forward while the monotonic clock moved 5 s: the op is still advancing (no NTP step makes every op hung)" "$("$S/classify.sh" --op-id "$id5" | cut -f2)" advancing
export LONGOPS_MONO=1700; assert_eq "D5b control: 700 s of MONOTONIC time without progress is hung" "$("$S/classify.sh" --op-id "$id5" | cut -f2)" hung
unset LONGOPS_NOW LONGOPS_MONO
echo "-- LO-D2: a hung build is reaped through the REAL pump trap and ends reaped, never re-adoptable --"
newfx d2; export LONGOPS_NOW=2000; H64=$(printf '%064d' 1); A64=$(printf '%064d' 2); BD=$FXN/bld1; mkdir -p "$BD/tmp"
jq -nc --arg p "build:app:lane:$H64:$A64:primary" '{purpose:$p,no_progress_budget_s:10,wallclock_cap_s:3600}' >"$BD/submit.json"
awk '/^reg_adopt\(\)/{p=1} /^reg_beat\(\)/{p=0} p' "$TROOT/scripts/build/dispatch.sh" >"$FXN/reg_adopt.fn"; awk '/^reg_handoff\(\)/{print}' "$TROOT/scripts/build/dispatch.sh" >"$FXN/reg_handoff.fn"
grep -F "trap '[ -d \"\$d/terminal\" ] || reg_handoff; exit 0' TERM" "$TROOT/scripts/build/dispatch.sh" | head -1 | sed 's/^[[:space:]]*//' >"$FXN/pump_trap.line"
{ grep -q 'register.sh' "$FXN/reg_adopt.fn" && grep -q 'release.sh' "$FXN/reg_handoff.fn" && grep -q 'reg_handoff; exit 0' "$FXN/pump_trap.line"; } && ok "D2.0 control needle: the extraction holds the real reg_adopt, reg_handoff and the pump's TERM trap line" || bad "D2.0 the extraction is empty or blind"
cat >"$FXN/pump.sh" <<EOF
LO=$S; LONGOPS_DIR=$LONGOPS_DIR; pump_log() { :; }; OPID=""; d=$BD
. "$FXN/reg_adopt.fn"; . "$FXN/reg_handoff.fn"
reg_adopt "\$d" || exit 3
$(cat "$FXN/pump_trap.line")
echo "\$OPID" >"$FXN/pump.opid"
while :; do sleep 0.2; done
EOF
bash "$FXN/pump.sh" >/dev/null 2>&1 & PUMP=$!; KILLME+=("$PUMP"); for _ in $(seq 1 50); do [ -s "$FXN/pump.opid" ] && break; sleep 0.1; done
assert_eq "D2.1 the real reg_adopt registered the build" "$(cat "$FXN/pump.opid" 2>/dev/null)" bld1
"$S/heartbeat.sh" --op-id bld1 --progress-offset 5 >/dev/null; export LONGOPS_NOW=2100
LONGOPS_REAP_GRACE_S=5 "$S/reap.sh" --op-id bld1 >"$FXN/o" 2>"$FXN/e"; rc=$?
assert_rc "D2.2 reap of the HUNG pump exits 0" $rc 0
assert_eq "D2.3 the record ends reaped with verdict reaped_hung:owner_handoff (the pump's handoff trap answered OUR reap)" "$(jq -r '.state+"/"+.verdict' "$LONGOPS_DIR/ops/bld1.json")" "reaped/reaped_hung:owner_handoff"
assert_eq "D2.4 the owner's own verdict is kept" "$(jq -r '.owner_verdict // "none"' "$LONGOPS_DIR/ops/bld1.json")" driver_stop
grep -q "reaped bld1" "$FXN/o" && ! grep -qi "handoff op" "$FXN/o" && ok "D2.5 the message states the state it found (handoff converted to reaped)" || bad "D2.5 [$(cat "$FXN/o")]"
sw=$(bash "$S/classify.sh" --op-id bld1 | cut -f2); assert_eq "D2.6 the build is terminal: a resumed driver re-adopts nothing" "$sw" terminal
unset LONGOPS_NOW
echo "-- LO-D4: the reap grace covers the owner's own stop grace --"
newfx d4; mkll; SLOWPID=""; cat >"$FXN/slow.sh" <<'EOF'
trap 'sleep "$SLOW_EXIT_S"; exit 0' TERM
while :; do sleep 0.2; done
EOF
export LONGOPS_NOW=2000
SLOW_EXIT_S=3 bash "$FXN/slow.sh" >/dev/null 2>&1 & SP=$!; KILLME+=("$SP"); sleep 0.4
id=$("$S/register.sh" --purpose d4:x --owner t --pid "$SP" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5 >/dev/null; export LONGOPS_NOW=2100
"$S/reap.sh" --op-id "$id" >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "D4 an owner whose exit path takes 3 s is reaped with the DEFAULT grace (0, not reap_survived 8)" $rc 0
SLOW_EXIT_S=19 bash "$FXN/slow.sh" >/dev/null 2>&1 & SP2=$!; KILLME+=("$SP2"); sleep 0.4
id2=$("$S/register.sh" --purpose d4:y --owner t --pid "$SP2" --no-progress-s 10 --stop-grace-s 18); "$S/heartbeat.sh" --op-id "$id2" --progress-offset 5 >/dev/null; export LONGOPS_NOW=2200
assert_eq "D4b register --stop-grace-s records budget.stop_grace_s" "$(jq -r '.budget.stop_grace_s' "$LONGOPS_DIR/ops/$id2.json" 2>/dev/null)" 18
"$S/reap.sh" --op-id "$id2" >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "D4c an owner that declared stop_grace_s 18 and needs 19 s is waited for (grace = max(15, 18 + 10)): exit 0" $rc 0
assert_eq "D4d it ended reaped" "$(jq -r .state "$LONGOPS_DIR/ops/$id2.json")" reaped
unset LONGOPS_NOW
echo "-- LO-D3 (registry side): a dead-owner op keeps its resources until it is REAPED --"
newfx d3; mkll; A=$LL; mkll; B=$LL
id=$("$S/register.sh" --purpose container:go:abc --owner runner --pid "$A" --no-progress-s 600 --write-path tracked/a.txt)
"$S/check_no_build_writing_tracked.sh" >/dev/null 2>&1; assert_rc "D3 a live op declaring a tracked path blocks the commit window (1)" $? 1
kill "$A"; wait "$A" 2>/dev/null
"$S/check_no_build_writing_tracked.sh" >"$FXN/o" 2>&1; assert_rc "D3b the SAME op with a dead owner still blocks until it is reaped (1): owner gone is not workload gone" $? 1
grep -q "$id" "$FXN/o" && ok "D3c the refusal names the op" || bad "D3c [$(cat "$FXN/o")]"
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; "$S/check_no_build_writing_tracked.sh" >/dev/null 2>&1; assert_rc "D3d once it is reaped the window is open (0)" $? 0
echo "-- LO-C2: --expect binds the action to the class the caller judged --"
newfx c2; export LONGOPS_NOW=$(date +%s); mkll; A=$LL; ID=$("$S/register.sh" --purpose c2:x --owner t --pid "$A" --no-progress-s 600)
"$S/reap.sh" --op-id "$ID" --expect dead_owner >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "C2 a dead-owner reap of an op that is ADVANCING is refused (5 class_changed)" $rc 5
grep -q class_changed "$FXN/e" && ok "C2b the refusal names class_changed" || bad "C2b [$(cat "$FXN/e")]"
[ ! -s "$LONGOPS_DIR/signals.log" ] && kill -0 "$A" 2>/dev/null && ok "C2c nothing was signalled, the owner lives" || bad "C2c"
export LONGOPS_NOW=$(( $(date +%s) + 7200 ))
"$S/reap.sh" --op-id "$ID" --expect dead_owner >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "C2d a HUNG op (live owner) reaped with --expect dead_owner is refused (5): no TERM from an 'auto-safe, no signal' action" $rc 5
[ ! -s "$LONGOPS_DIR/signals.log" ] && kill -0 "$A" 2>/dev/null && ok "C2e no signal was recorded, the owner lives" || bad "C2e"
"$S/reap.sh" --op-id "$ID" --expect bogus >/dev/null 2>&1; assert_rc "C2f an unknown --expect class is a usage error (2)" $? 2
unset LONGOPS_NOW

fi
if want F; then
echo "== class F: owner gone is not workload gone =="
reg_dead f1 f1:x; fcont cidAAA "catalogizer.op_id=$ID"; touch "$PODDIR/stubborn" "$PODDIR/stop-rc125"; h0=$(sha "$REC")
"$S/reap.sh" --op-id "$ID" >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "F1 a dead owner whose container will not stop (podman stop exits 125, still listed) is NOT reaped: exit 9 container_survived" $rc 9
assert_eq "F1b the record is not reaped (the only change allowed is the survival marker)" "$(jq -r .state "$REC")" registered
[ -d "$LONGOPS_DIR/claims/f1:x" ] && ok "F1c the claim is kept" || bad "F1c the claim was released"
mkll; "$S/register.sh" --purpose f1:x --owner other --pid "$LL" >/dev/null 2>&1; assert_rc "F1d a second owner of the purpose is refused (3 live holder or 4 stale), never registered" $(( $? == 3 || $? == 4 ? 0 : 1 )) 0
grep -q "stop" "$PODDIR/calls" && ok "F1e podman stop WAS attempted" || bad "F1e"
rm -f "$PODDIR/stubborn" "$PODDIR/stop-rc125"
"$S/reap.sh" --op-id "$ID" >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "F1f once the container can be stopped the dead-owner op is reaped (0)" $rc 0
assert_eq "F1g it is reaped and the container is gone" "$(jq -r .state "$REC"):$(ls "$PODDIR" | grep -c '^c-')" "reaped:0"
reg_dead f2 f2:x; fcont c1 "op_id=$ID"; fcont c2 "op_id=$ID"; fcont c3 "catalogizer.op_id=$ID"
"$S/reap.sh" --op-id "$ID" >/dev/null 2>&1; assert_rc "F1h a dead owner with THREE containers of its op id is reaped (0)" $? 0
assert_eq "F1i all three containers were stopped (not only the first)" "$(grep -c '^stop' "$PODDIR/calls"):$(ls "$PODDIR" | grep -c '^c-')" "3:0"
reg_dead f3 f3:x; touch "$PODDIR/ps-fails"; h0=$(sha "$REC")
"$S/reap.sh" --op-id "$ID" >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "F1j an unreadable container runtime (podman ps fails) refuses the reap (20): the workload is not proven gone" $rc 20
assert_eq "F1k the record and the claim are kept" "$(sha "$REC"):$([ -d "$LONGOPS_DIR/claims/f3:x" ] && echo c)" "$h0:c"
reg_dead f4 f4:x; fcont cidBBB "catalogizer.op_id=catalogizer.op_id=$ID"; printf '#!/bin/sh\nsleep 600\n' >"$FXN/podman-wedged"; chmod +x "$FXN/podman-wedged"
t0=$(date +%s); LONGOPS_PODMAN=$FXN/podman-wedged LONGOPS_PODMAN_TIMEOUT_S=2 timeout 30 "$S/reap.sh" --op-id "$ID" >/dev/null 2>&1; rc=$?; t1=$(date +%s)
[ "$rc" -eq 20 ] && [ $((t1-t0)) -lt 25 ] && ok "F1l a wedged podman is bounded by LONGOPS_PODMAN_TIMEOUT_S: reap returns 20 in $((t1-t0)) s" || bad "F1l rc=$rc after $((t1-t0)) s"
echo "-- LO-F2: the own-run holder is judged; the op is written before the holder --"
newfx f5; mkll; A=$LL; mkll; B=$LL; id=$("$S/register.sh" --purpose f5:x --owner t --pid "$A" --no-progress-s 600); "$S/heartbeat.sh" --op-id "$id" --pid "$B" >/dev/null
jq -c --argjson a "$A" --arg st "$(pstart "$A")" '.pid=$a|.start_time=$st' "$LONGOPS_DIR/ops/$id.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/$id.json"   # the op names A, the holder names B
kill "$A"; wait "$A" 2>/dev/null
"$S/reap.sh" --op-id "$id" >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "F2 an op whose owner is dead but whose own-run HOLDER is live is refused (5 holder_live)" $rc 5
[ -d "$LONGOPS_DIR/claims/f5:x" ] && kill -0 "$B" 2>/dev/null && ok "F2b the claim is kept and the worker lives" || bad "F2b"
grep -q holder_live "$FXN/e" && ok "F2c the refusal names holder_live" || bad "F2c [$(cat "$FXN/e")]"
mkll; "$S/register.sh" --purpose f5:x --owner second --pid "$LL" >/dev/null 2>&1; assert_rc "F2d a second owner is refused (3)" $? 3
if [ "$ROOTUID" = 0 ]; then ok "F2e skipped: running as root (UNCONFIRMED here, 11.4.3)"; else
  newfx f6; mkll; A=$LL; mkll; B=$LL; id=$("$S/register.sh" --purpose f6:x --owner t --pid "$A" --no-progress-s 600); chmod 555 "$LONGOPS_DIR/claims/f6:x"
  "$S/heartbeat.sh" --op-id "$id" --pid "$B" >/dev/null 2>"$FXN/e"; rc=$?; chmod 755 "$LONGOPS_DIR/claims/f6:x"
  assert_rc "F2e a holder write that fails on a rebind exits 1" $rc 1
  assert_eq "F2f the OP was written first: it names the new owner" "$(jq -r .pid "$LONGOPS_DIR/ops/$id.json")" "$B"
  kill "$A"; wait "$A" 2>/dev/null; "$S/reap.sh" --purpose f6:x >/dev/null 2>&1; assert_rc "F2g the stale holder beside the LIVE op is not released (5)" $? 5
fi


echo "-- LO-F4: the own-run holder is judged AGAIN when the terminal write happens (step B), not only in step A --"
reg_dead f7 f7:x; fcont cidW "op_id=$ID"; echo 4 >"$PODDIR/stop-sleep"; mkll; B=$LL; RUN=$(jq -r .run_id "$REC")
"$S/reap.sh" --op-id "$ID" >"$FXN/o" 2>"$FXN/e" & RP=$!
sleep 1.5   # step A judged the holder dead and released the lock; the UNLOCKED container stop is sleeping now
mkholder f7:x "$RUN" "$B"   # a worker came up under the op's own run in the window
wait "$RP"; rc=$?
assert_rc "F4 a holder of the op's own run that turned live between step A and the terminal write is refused (5 holder_live)" $rc 5
[ -d "$LONGOPS_DIR/claims/f7:x" ] && assert_eq "F4b the claim is kept" "$(jq -r .pid "$LONGOPS_DIR/claims/f7:x/holder.json")" "$B" || bad "F4b the claim was released under a live worker"
assert_eq "F4c the op was not written reaped (still registered)" "$(jq -r .state "$REC")" registered

fi
if want G; then
echo "== class G: transitions with several writes =="
newfx g1; export LONGOPS_TEST_SLEEP_IN_CS=1.5; mkll; A=$LL
"$S/register.sh" --purpose g1:x --owner t --op-id g1op --pid "$A" >/dev/null 2>&1 & RP=$!; sleep 0.5; kill -TERM "$RP"; wait "$RP" 2>/dev/null; rc=$?
unset LONGOPS_TEST_SLEEP_IN_CS
assert_eq "G1 a TERM in the window of register.sh leaves no claim without an op record" "$([ -d "$LONGOPS_DIR/claims/g1:x" ] && echo claim || echo noclaim):$([ -e "$LONGOPS_DIR/ops/g1op.json" ] && jq -r .state "$LONGOPS_DIR/ops/g1op.json" || echo norecord)" "noclaim:failed"
assert_eq "G1b the record says why (interrupted)" "$(jq -r '.verdict' "$LONGOPS_DIR/ops/g1op.json" 2>/dev/null)" interrupted
mkll; "$S/register.sh" --purpose g1:x --owner t --pid "$LL" >/dev/null 2>&1; assert_rc "G1c the same purpose can be registered at once afterwards (0)" $? 0
newfx g1c; export LONGOPS_TEST_SLEEP_IN_CS=1.5; mkll; A=$LL
"$S/register.sh" --purpose g1c:x --owner t --op-id g1cop --pid "$A" >/dev/null 2>&1 & RP=$!; sleep 2.2; kill -TERM "$RP"; wait "$RP" 2>/dev/null
unset LONGOPS_TEST_SLEEP_IN_CS
assert_eq "G1d a TERM AFTER the claim rolls back OUR claim and fails the record" "$([ -d "$LONGOPS_DIR/claims/g1c:x" ] && echo claim || echo noclaim):$(jq -r .state "$LONGOPS_DIR/ops/g1cop.json")" "noclaim:failed"
newfx g1e; mkll; A=$LL; mkll; B=$LL; "$S/register.sh" --purpose g1e:x --owner a --pid "$A" >/dev/null; "$S/register.sh" --purpose g1e:x --owner b --op-id g1eb --pid "$B" >/dev/null 2>&1; rc=$?
assert_eq "G1e a refused registration (purpose held, 3) leaves a FAILED record naming the refusal, and the holder's claim untouched" "$rc:$(jq -r '.state+"/"+.verdict' "$LONGOPS_DIR/ops/g1eb.json" 2>/dev/null):$(jq -r .pid "$LONGOPS_DIR/claims/g1e:x/holder.json")" "3:failed/claim_refused:3:$A"
echo "-- release.sh: record and unclaim are one transition (a TERM to its pid is held until both are done) --"
newfx g1f; mkll; A=$LL; id=$("$S/register.sh" --purpose g1f:x --owner t --pid "$A"); export LONGOPS_TEST_SLEEP_IN_CS=1.2
"$S/release.sh" --op-id "$id" --state complete --verdict PASS >/dev/null 2>&1 & RP=$!; sleep 0.5; kill -TERM "$RP"; wait "$RP" 2>/dev/null; rc=$?; unset LONGOPS_TEST_SLEEP_IN_CS
assert_eq "G1f a TERM during release.sh: the record is terminal AND the claim is gone (exit 143 after the transition)" "$(jq -r .state "$LONGOPS_DIR/ops/$id.json"):$([ -d "$LONGOPS_DIR/claims/g1f:x" ] && echo claim || echo noclaim):$rc" "complete:noclaim:143"
echo "-- LO-G2: the hub sequence (a crash, a stale claim, reap --purpose, register again) leaves no dead-owner row --"
newfx g2; mkll; H1=$LL; "$S/register.sh" --purpose event_hub --owner event_hub --op-id hub-aaaa-1 --pid "$H1" --no-progress-s 0 >/dev/null; kill "$H1"; wait "$H1" 2>/dev/null
mkll; H2=$LL; "$S/register.sh" --purpose event_hub --owner event_hub --op-id hub-aaaa-2 --pid "$H2" --no-progress-s 0 >/dev/null 2>&1; assert_rc "G2 hub2 register meets the stale claim (4)" $? 4
"$S/reap.sh" --purpose event_hub >/dev/null 2>&1; assert_rc "G2b reap --purpose releases it (0)" $? 0
"$S/register.sh" --purpose event_hub --owner event_hub --op-id hub-aaaa-2 --pid "$H2" --no-progress-s 0 >/dev/null 2>&1; assert_rc "G2c hub2 registers (0)" $? 0
assert_eq "G2d no dead_owner row is left behind (the old hub op is reaped with its claim)" "$("$S/classify.sh" | cut -f1,2 | tr '\t' ':' | sort | tr '\n' ' ')" "hub-aaaa-1:terminal hub-aaaa-2:advancing "
assert_eq "G2e hub-aaaa-1 ended reaped" "$(jq -r .state "$LONGOPS_DIR/ops/hub-aaaa-1.json")" reaped
echo "-- LO-G3: directory entries are flushed AFTER the link / mkdir that created them --"
newfx g3; mkll; A=$LL
if strace -f -qq -o "$FX/strace.probe" -e trace=fsync true 2>/dev/null; then
  strace -f -qq -s 1024 -o "$FXN/strace.out" -e trace=openat,fsync,fdatasync,link,linkat,mkdir,mkdirat,close,execve "$S/register.sh" --purpose g3:x --owner t --pid "$A" >/dev/null 2>&1
  python3 -I - "$FXN/strace.out" "$LONGOPS_DIR" >"$FXN/flush.txt" <<'PYEOF'
import re, sys
log, ld = sys.argv[1:3]
fds = {}; events = []
for line in open(log, errors="replace"):
    m = re.match(r'^(\d+)\s+(\w+)\((.*)\)\s+=\s+(-?\d+)', line.strip())
    if not m: continue
    pid, sc, args, rv = m.groups()
    if sc == "openat":
        mo = re.search(r'"([^"]+)"', args)
        if mo and int(rv) >= 0: fds[(pid, rv)] = mo.group(1)
    elif sc == "close": fds.pop((pid, args.strip()), None)
    elif sc in ("link", "linkat"):
        names = re.findall(r'"([^"]+)"', args)
        if names and names[-1].startswith(ld + "/ops/") and "/.tmp." not in names[-1] and int(rv) == 0: events.append(("link-ops", None))
    elif sc in ("mkdir", "mkdirat"):
        names = re.findall(r'"([^"]+)"', args)
        if names and names[0].startswith(ld + "/claims/") and int(rv) == 0: events.append(("mkdir-claims", None))
    elif sc in ("fsync", "fdatasync"): events.append(("sync", fds.get((pid, args.strip()))))
    elif sc == "execve":   # an external `sync -- DIR` (this host: uutils coreutils, whose `sync DIR` calls the GLOBAL sync()): attributed to the directory named on its command line, never to every later sync() syscall
        names = re.findall(r'"([^"]+)"', args)
        if names and names[0].endswith("/sync") and len(names) > 1: events.append(("sync", names[-1]))
def flushed_after(kind, d):
    seen = False
    for k, v in events:
        if k == kind: seen = True
        elif seen and k == "sync" and (v == "*" or v == d): return True
    return False
print("ops" if flushed_after("link-ops", ld + "/ops") else "NOT-ops")
print("claims" if flushed_after("mkdir-claims", ld + "/claims") else "NOT-claims")
PYEOF
  grep -qx ops "$FXN/flush.txt" && ok "G3 the op record's directory entry (ops/) is flushed after the link (fsync of the directory, or sync/syncfs)" || bad "G3 nothing flushes ops/ after the link [$(tr '\n' ' ' <"$FXN/flush.txt")]"
  grep -qx claims "$FXN/flush.txt" && ok "G3b the claim's directory entry (claims/) is flushed after the mkdir" || bad "G3b nothing flushes claims/ after the mkdir"
else ok "G3 skipped: strace cannot trace on this host (ptrace refused); UNCONFIRMED, never a grep-only PASS (11.4.3)"; ok "G3b skipped (same reason)"; fi

fi
if want H; then
echo "== class H: process identity =="
H1PY='import ctypes,time; ctypes.CDLL(None).prctl(15, NAME, 0, 0, 0); print("ready", flush=True); time.sleep(600)'
for nm in 'b"ab\ncd\0"' 'b"ab\xffcd\0"' 'b"a) b c\0"'; do
  newfx "h1$RANDOM"; LANG=en_US.UTF-8 python3 -I -c "${H1PY/NAME/$nm}" >"$FXN/ready" 2>/dev/null & CP=$!; KILLME+=("$CP"); for _ in $(seq 1 40); do [ -s "$FXN/ready" ] && break; sleep 0.1; done
  id=$(LANG=en_US.UTF-8 "$S/register.sh" --purpose h1:x --owner t --pid "$CP" --no-progress-s 600 2>"$FXN/e"); rc=$?
  assert_rc "H1 register of a live process whose comm is $nm works (0)" $rc 0
  [ "$rc" -ne 0 ] || assert_eq "H1b under LANG=en_US.UTF-8 the live owner reads advancing, never dead_owner" "$(LANG=en_US.UTF-8 "$S/classify.sh" --op-id "$id" | cut -f2)" advancing
  [ "$rc" -ne 0 ] || { LANG=en_US.UTF-8 "$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "H1c reap --op-id refuses a live advancing owner (5), the claim stays" $? 5; }
  kill "$CP" 2>/dev/null; wait "$CP" 2>/dev/null
done

fi
if want J; then
echo "== class J: the registry grows by about 1000 records a day; a gate must not rescan it record by record =="
newfx j1; mkdir -p "$LONGOPS_DIR/ops"
python3 -I - "$LONGOPS_DIR/ops" "$(bootid)" <<'PY'
import json, sys
d, bid = sys.argv[1:3]
for i in range(1017):
    r = {"op_id": "old-%04d" % i, "purpose_key": "build:t:l:%064d:%064d:primary" % (i, i), "owner": "x", "run_id": "old-%04d" % i, "container_label": "old-%04d" % i, "write_paths": [], "state": ["complete", "failed", "reaped"][i % 3],
         "started_utc": "2026-01-01T00:00:00Z", "verdict": "x", "evidence_path": ""}
    open("%s/old-%04d.json" % (d, i), "w").write(json.dumps(r))
PY
mkll; A=$LL; "$S/register.sh" --purpose j1:a --owner t --pid "$A" --no-progress-s 600 >/dev/null
mkll; B=$LL; "$S/register.sh" --purpose j1:b --owner t --pid "$B" --no-progress-s 600 >/dev/null
mkdir -p "$FXN/shim"; for tool in jq python3; do real=$(command -v "$tool"); printf '#!/bin/sh\necho x >>"%s/spawn.count"\nexec %s "$@"\n' "$FXN" "$real" >"$FXN/shim/$tool"; chmod +x "$FXN/shim/$tool"; done
: >"$FXN/spawn.count"; t0=$(date +%s.%N); PATH=$FXN/shim:$PATH timeout 60 "$S/classify.sh" >"$FXN/o" 2>&1; rc=$?; t1=$(date +%s.%N); n=$(wc -l <"$FXN/spawn.count")
assert_eq "J1 classify.sh over 1017 terminal + 2 live records: 1019 rows" "$(wc -l <"$FXN/o")" 1019
[ "$n" -le 25 ] && ok "J1b the listing spawned $n jq/python3 processes (O(1 + live))" || bad "J1b the listing spawned $n jq/python3 processes"
[ "$(echo "$t1 - $t0 < 8" | bc)" = 1 ] && ok "J1c and took $(printf '%.1f' "$(echo "$t1 - $t0" | bc)") s" || bad "J1c took $(echo "$t1 - $t0" | bc) s"
kill "$A"; wait "$A" 2>/dev/null
: >"$FXN/spawn.count"; t0=$(date +%s.%N); PATH=$FXN/shim:$PATH timeout 60 "$S/reap.sh" --purpose j1:a >"$FXN/o" 2>&1; rc=$?; t1=$(date +%s.%N)
[ "$rc" -eq 0 ] && [ "$(echo "$t1 - $t0 < 4" | bc)" = 1 ] && ok "J1d reap --purpose (the purpose lock is held for its decisions) took $(printf '%.1f' "$(echo "$t1 - $t0" | bc)") s, rc 0" || bad "J1d rc=$rc, $(echo "$t1 - $t0" | bc) s"
[ "$(wc -l <"$FXN/spawn.count")" -le 60 ] && ok "J1e it spawned $(wc -l <"$FXN/spawn.count") jq/python3 processes" || bad "J1e it spawned $(wc -l <"$FXN/spawn.count")"

fi
if want K; then
echo "== class K: a value-taking flag given as the LAST token must not hang the parser =="
newfx k1
python3 -I - "$S" "$TROOT/scripts/anti-mess/sweep.sh" >"$FXN/flags.tsv" <<'PY'
import re, sys, os
out = []
for p in sys.argv[1:]:
    if os.path.isdir(p):
        files = [os.path.join(p, n) for n in sorted(os.listdir(p)) if n.endswith(".sh") and n != "lib.sh"]
    else:
        files = [p]
    for f in files:
        src = open(f).read()
        for seg in src.split(";;"):
            m = re.search(r'(--[a-z][a-z0-9-]*)\)', seg)
            if m and re.search(r'shift 2', seg):
                out.append("%s\t%s" % (os.path.basename(f), m.group(1)))
print("\n".join(sorted(set(out))))
PY
nflags=$(wc -l <"$FXN/flags.tsv"); [ "$nflags" -ge 35 ] && ok "K1.0 control needle: the generated table holds $nflags (script, flag) pairs" || bad "K1.0 the generated table holds only $nflags pairs (a blind enumeration)"
hung=0; tot=0
while IFS=$'\t' read -r sc fl; do
  tot=$((tot+1)); if [ "$sc" = sweep.sh ]; then want=20; cmd=("$TROOT/scripts/anti-mess/sweep.sh"); else want=2; cmd=("$S/$sc"); fi
  case "$sc" in holder.sh) continue ;; esac
  timeout 5 bash "${cmd[0]}" "$fl" >/dev/null 2>&1; rc=$?
  [ "$rc" -eq "$want" ] || { hung=$((hung+1)); echo "  $sc $fl as the last token: rc=$rc (want $want)"; }
done <"$FXN/flags.tsv"
assert_eq "K1 every value-taking flag of every script given as the LAST token is a usage error ($tot checked), never a hang (124)" "$hung" 0

fi
if want X; then
echo "== killers of the round-3 reviewer mutants (WF17 X1-X4, C01-C07, C11, C12), run on the real scripts =="
newfx x1; mkll; A=$LL; export LONGOPS_NOW=2000
id=$("$S/register.sh" --purpose x1:x --owner t --pid "$A" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5 >/dev/null; mkholder x1:x "$id" 999999; export LONGOPS_NOW=2100
"$S/reap.sh" --purpose x1:x >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "X1 a HUNG op whose owner lives keeps its claim even when the holder record is dead: reap --purpose refuses (5)" $rc 5
[ -d "$LONGOPS_DIR/claims/x1:x" ] && ok "X1b the claim is intact" || bad "X1b"
unset LONGOPS_NOW
newfx x4; mkll; A=$LL; mkll; B=$LL; "$S/acquire.sh" --purpose x4:a --run-id r1 --pid "$A" >/dev/null; kill "$A"; wait "$A" 2>/dev/null
"$S/register.sh" --purpose x4:b --owner t --pid "$B" --no-progress-s 600 >/dev/null
"$S/reap.sh" --purpose x4:a >/dev/null 2>&1; rc=$?; assert_rc "C04 a live op of ANOTHER purpose does not block the release of a stale claim (0)" $rc 0
[ ! -d "$LONGOPS_DIR/claims/x4:a" ] && ok "C04b the stale claim was released" || bad "C04b"
newfx x2; mkll; A=$LL; mkll; B=$LL; mkll; C=$LL; "$S/register.sh" --purpose x2:p --owner a --pid "$A" >/dev/null; "$S/register.sh" --purpose x2:p --owner b --op-id x2b --pid "$B" --no-claim >/dev/null
"$S/heartbeat.sh" --op-id x2b --pid "$C" >/dev/null; assert_eq "X2 rebinding a --no-claim op while ANOTHER run holds the purpose leaves that holder untouched" "$(jq -r .pid "$LONGOPS_DIR/claims/x2:p/holder.json")" "$A"
newfx x3; mkll; P=$LL; export LONGOPS_NOW=2000; id=$("$S/register.sh" --purpose x3:p --owner t --pid "$P" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5 >/dev/null; export LONGOPS_NOW=2100; h0=$(sha "$LONGOPS_DIR/ops/$id.json")
LONGOPS_TEST_SLEEP_BEFORE_SIGNAL=1.5 "$S/reap.sh" --op-id "$id" >"$FXN/o" 2>"$FXN/e" & RP=$!; sleep 0.5; kill -9 "$P" 2>/dev/null; wait "$RP"; rc=$?
assert_rc "X3 the owner dies (its pid may be recycled) between the classification and the signal: the start-time re-check refuses (6)" $rc 6
assert_eq "X3b the record is untouched (no reap intent, no state change) and nothing was signalled" "$(sha "$LONGOPS_DIR/ops/$id.json"):$([ -s "$LONGOPS_DIR/signals.log" ] && echo sig || echo none)" "$h0:none"
unset LONGOPS_NOW
newfx xc1; fpod; mkll; P=$LL; id=$("$S/register.sh" --purpose xc1:p --owner t --op-id xc1op --pid "$P" --container-label barex); fcont cidX1 "catalogizer.op_id=barex"; kill "$P"; wait "$P" 2>/dev/null
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; rc=$?; assert_eq "C01 a BARE container_label (neither k=v nor the op id) is resolved as catalogizer.op_id=<label>: the container is stopped before the reap (0, none left)" "$rc:$(ls "$PODDIR" | grep -c '^c-'):$(jq -r .state "$LONGOPS_DIR/ops/$id.json")" "0:0:reaped"
newfx xc3; fpod; mkll; P=$LL; export LONGOPS_NOW=2000; id=$("$S/register.sh" --purpose xc3:p --owner t --pid "$P" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5 >/dev/null; export LONGOPS_NOW=2100
fcont cidSlow "op_id=$id"; echo 4 >"$PODDIR/stop-sleep"
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1 & RP=$!; sleep 1; t0=$(date +%s.%N); "$S/heartbeat.sh" --op-id "$id" --progress-offset 6 >/dev/null 2>&1; hrc=$?; t1=$(date +%s.%N); wait "$RP"
assert_rc "C03 a heartbeat during a SLOW container stop is recorded (0)" $hrc 0
[ "$(echo "$t1 - $t0 < 2.0" | bc)" = 1 ] && ok "C03b it did not wait for the container stop: the purpose lock is not held across podman ($(printf '%.1f' "$(echo "$t1 - $t0" | bc)") s)" || bad "C03b waited $(echo "$t1 - $t0" | bc) s"
unset LONGOPS_NOW
reg_dead xc6 xc6:p; h0=$(sha "$REC"); "$S/reap.sh" --op-id "$ID" --dry-run >"$FXN/o" 2>&1; rc=$?
assert_eq "C06 reap --op-id --dry-run of a dead-owner op writes nothing and releases nothing" "$rc:$(sha "$REC"):$([ -d "$LONGOPS_DIR/claims/xc6:p" ] && echo claim)" "0:$h0:claim"
grep -q "would reap dead" "$FXN/o" && ok "C06b it says what it would do" || bad "C06b [$(cat "$FXN/o")]"
newfx xc7; mkll; A=$LL; "$S/acquire.sh" --purpose xc7:p --run-id r1 --pid "$A" >/dev/null; kill "$A"; wait "$A" 2>/dev/null
"$S/reap.sh" --purpose xc7:p --dry-run >"$FXN/o" 2>&1; rc=$?; assert_eq "C07 reap --purpose --dry-run releases nothing (the claim is kept)" "$rc:$([ -d "$LONGOPS_DIR/claims/xc7:p" ] && echo claim)" "0:claim"
newfx xc11; mkll; A=$LL; id=$("$S/register.sh" --purpose xc11:p --owner t --pid "$A"); "$S/release.sh" --op-id "$id" --state blocked-escape --verdict operator >/dev/null; h0=$(sha "$LONGOPS_DIR/ops/$id.json")
assert_eq "C11 a blocked-escape record is TERMINAL" "$("$S/classify.sh" --op-id "$id" | cut -f2)" terminal
"$S/reap.sh" --op-id "$id" >"$FXN/o" 2>&1; rc=$?; assert_eq "C11b reap of it is 'nothing to reap' (0) and the record is byte-identical" "$rc:$(sha "$LONGOPS_DIR/ops/$id.json")" "0:$h0"
newfx xc12; kt=""; for d in /proc/[0-9]*; do n=${d#/proc/}; [ "$n" -gt 1 ] 2>/dev/null || continue; pg=$(sed 's/^.*) //' "$d/stat" 2>/dev/null | cut -d' ' -f3); [ "$pg" = 0 ] && { kt=$n; break; }; done
if [ -n "$kt" ]; then export LONGOPS_NOW=5000; mkop xkt xkt:p running "$kt" 100 '.cmdline=""'; mkholder xkt:p xkt "$kt"
  LONGOPS_REAP_GRACE_S=1 "$S/reap.sh" --op-id xkt >"$FXN/o" 2>"$FXN/e"; rc=$?
  assert_eq "C12 a hung op owned by a KERNEL THREAD (pid $kt, process group 0) is refused as an unsafe signal target (7) and nothing is signalled" "$rc:$([ -s "$LONGOPS_DIR/signals.log" ] && echo sig || echo none)" "7:none"; unset LONGOPS_NOW
else ok "C12 skipped: no kernel thread (pgrp 0) on this host; UNCONFIRMED here (11.4.3)"; fi
fi

finish
