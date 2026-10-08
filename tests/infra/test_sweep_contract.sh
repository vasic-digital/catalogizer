#!/usr/bin/env bash
# test_sweep_contract.sh - WF17 fix round 5, CLASS A (REG) closure test (TI-A1..TI-A5). The stack's long-op record and container labels must satisfy the anti-mess contract at EVERY lifecycle point:
# the REAL scripts/anti-mess/sweep.sh with runner_lib's exact arguments (`--stage cadence --only AM-P1,AM-P2,AM-P3`), run against the SAME registry the stack used, must report no drift that names this
# test's stack (its project, operation or containers). The registry is a per-run scratch registry (LONGOPS_DIR) so the verdict is about THIS stack only; the sweep still reads the host's real podman.
# Rows: after up | a live stack past its no-progress budget (keeper heartbeats, TI-A1) | the same stack after its container died (the sweep MUST report hung_op: the instrument sees) | during a client run
# (the client carries the stack's catalogizer.op_id, TI-A4) | after a dead keeper and the recovery (TI-A3, TI-A5) | after down.
# Oracle strategy (11.4.245): SPECIFIED by the anti-mess contract (docs/16 section 13.3) and INVARIANT (a planted hung operation of the same scratch registry MUST be reported through the same filter: control needle).
# Paired mutations: drop the heartbeat (row "live past budget" must FAIL); beat without the liveness check (row "dead stack" must FAIL); client label added instead of replaced (row "client run" must FAIL);
# a dead operation not closed before the next start (row "recovery" must FAIL); a dead holder recorded `complete` instead of `reaped`; a failed start recorded `complete`; identity mutant (must SURVIVE).
# Usage:  test_sweep_contract.sh   (SWC_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR, SWC_EV
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
. "$(dirname "${BASH_SOURCE[0]}")/mutlib.sh"; MUT_ENV=SWC; MUT_SELF="${BASH_SOURCE[0]}"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"; TI_DOWN="$TI_REPO/$SD/down.sh"
for f in up.sh down.sh run_client.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
UP="$TI_REPO/$SD/up.sh"; DOWN="$TI_REPO/$SD/down.sh"; RCL="$TI_REPO/$SD/run_client.sh"
export LONGOPS_DIR="$TI_SCRATCH/lo" LONGOPS_ALLOW_TMPFS=1
BUD=20; export TI_OP_BUDGET_S=$BUD
opid_of() { printf '%s\n' "$1" | sed -n 's/^op_id=//p'; }
# the sweep exactly as runner_lib.sh rl_sweep_gate runs it; ours() keeps only the lines that name this test's stack
sweep() { bash "$TI_REPO/scripts/anti-mess/sweep.sh" --stage cadence --only AM-P1,AM-P2,AM-P3 >"$TI_SCRATCH/sw.out" 2>&1; SWRC=$?; }
ours() { grep -E "drift" "$TI_SCRATCH/sw.out" | grep -E "($TOK)" ; }
A=$(ti_new_id); TI_IDS+=("$A"); P=$(ti_project "$A")

# ---- row 1: after up ----
out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?; check "up exits 0" "$rc" 0
[ "$rc" = 0 ] || { echo "  up said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"; ti_summary; exit 1; }
OP=$(opid_of "$out"); CID=$(podman ps -q --filter "label=catalogizer.op_id=$OP" | head -1); TOK="$P|$OP|${CID:0:12}"
check "the stack's operation is registered in the scratch registry (the sweep reads the SAME registry)" "$(jq -r .state "$LONGOPS_DIR/ops/$OP.json" 2>/dev/null)" running
sweep; case "$SWRC" in 0|10) ok "row after-up: the sweep ran (rc=$SWRC)";; *) bad "row after-up: the sweep exited $SWRC (a refusal or a blind detector)";; esac
[ -z "$(ours)" ] && ok "row after-up: no drift names this stack" || bad "row after-up: drift names this stack: $(ours | head -2 | cut -c1-200)"
# control needle (11.4.201(7)): a hung operation planted in the SAME registry is reported through the SAME filter, so a zero above means "clean", not "blind"
sleep 300 & PH=$!; disown
HUNG="planted-hung-$$"; bash "$TI_REPO/scripts/longops/register.sh" --purpose "planted-hung-purpose-$$" --owner swc-control --op-id "$HUNG" --pid "$PH" --no-progress-s 1 >/dev/null 2>&1
sleep 3; TOK="$TOK|$HUNG"; sweep
[ -n "$(ours | grep "$HUNG")" ] && ok "control needle: the planted hung operation is reported (the filter and the sweep can see a hung_op)" || bad "control needle: the sweep did not report the planted hung operation (blind instrument): $(head -5 "$TI_SCRATCH/sw.out" | tr '\n' ';' | cut -c1-200)"
bash "$TI_REPO/scripts/longops/release.sh" --op-id "$HUNG" --state failed --verdict control_done >/dev/null 2>&1; kill "$PH" 2>/dev/null; TOK="$P|$OP|${CID:0:12}"

# ---- row 2: a live stack past its no-progress budget (TI-A1) ----
sleep $((BUD * 2 + 5)); sweep
check "row live-past-budget: the operation is still running (not hung) after $((BUD*2+5)) s" "$(jq -r .state "$LONGOPS_DIR/ops/$OP.json" 2>/dev/null)" running
[ -z "$(ours)" ] && ok "row live-past-budget: the sweep reports no drift for the live stack ($((BUD*2+5)) s > budget $BUD s)" || bad "row live-past-budget: $(ours | head -2 | cut -c1-200)"

# ---- row 3: the same stack after its container died: the sweep MUST say hung_op ----
podman stop -t 1 "$CID" >/dev/null 2>&1
sleep $((BUD + 25)); sweep
[ -n "$(ours | grep hung_op)" ] && ok "row dead-stack: a stack whose container died turns hung_op after its budget (the keeper stopped beating)" || bad "row dead-stack: the sweep did not report hung_op for $OP: $(grep -E 'AM-P1' "$TI_SCRATCH/sw.out" | head -3 | tr '\n' ';' | cut -c1-240)"
podman start "$CID" >/dev/null 2>&1

# ---- row 4: during a client run (TI-A4) ----
bash "$RCL" --build-id "$A" -- sleep 12 >"$TI_SCRATCH/cl.out" 2>&1 & CP=$!; TI_BG_PIDS+=("$CP")
two_up() { [ "$(podman ps -q --filter "label=catalogizer.test_project=$P" | wc -l)" -ge 2 ]; }
ti_wait_for 20 two_up || true
CC=""; for x in $(podman ps -q --filter "label=catalogizer.test_project=$P"); do [ "$x" = "$CID" ] || CC="$x"; done
if [ -n "$CC" ]; then
  check "row client-run: the client container carries the STACK's catalogizer.op_id (replaced, not added: the sweep reads that label first)" "$(podman inspect --format '{{index .Config.Labels "catalogizer.op_id"}}' "$CC" 2>/dev/null)" "$OP"
  # resolve the client exactly as the sweep does (_P1_JQ): the label it reads, the registry row it finds
  RES=$(podman ps --filter label=project=catalogizer --format json | jq -r --arg c "${CC:0:12}" '.[]? | select(.Id[0:12]==$c) | ((.Labels // {})["catalogizer.op_id"] // (.Labels // {}).op_id // "-")')
  check "row client-run: the sweep's own label resolution maps the client to the stack's operation" "$RES" "$OP"
  [ -f "$LONGOPS_DIR/ops/$RES.json" ] && ok "row client-run: that operation has a row in the registry (the client is not an orphan)" || bad "row client-run: the resolved operation $RES has no registry row"
else bad "row client-run: no client container was seen while it ran (the oracle is blind)"; fi
wait "$CP" 2>/dev/null

# ---- row 5: dead keeper, the recovery and the truthful terminal states (TI-A3, TI-A5) ----
bash "$DOWN" --build-id "$A" --op-id "$OP" >/dev/null 2>&1
A2=$(ti_new_id); TI_IDS+=("$A2"); P2=$(ti_project "$A2")
out=$(bash "$UP" --build-id "$A2" --services redis 2>&1); OP2=$(opid_of "$out"); CID2=$(podman ps -q --filter "label=catalogizer.op_id=$OP2" | head -1)
rm -f "$(ti_state "$A2")/lease.keep.$OP2"      # the keeper leaves by itself (no signal): the holder is now PROVEN dead
lease_dead2() { [ "$(ti_lease_state "$A2" "$OP2")" = dead ]; }
ti_wait_for 30 lease_dead2
TOK="$P2|$OP2|${CID2:0:12}"; sweep
[ -n "$(ours | grep registry_row_dead_owner)" ] && ok "row recovery: control — the dead keeper IS reported (registry_row_dead_owner) before the recovery" || bad "row recovery: the dead holder was not reported (blind): $(grep AM-P1 "$TI_SCRATCH/sw.out" | head -2 | tr '\n' ';' | cut -c1-200)"
# the next start closes the dead operation (reaped) before it registers its own: no duplicate owner, no dead row left (the stack of the dead owner is still there, so this start tears it down and says so: exit 1)
out=$(bash "$UP" --build-id "$A2" --services redis 2>&1); rc=$?
check "row recovery: a start over the stack of a dead holder closes the dead operation and refuses to mix with its containers (exit 1)" "$rc" 1
check "row recovery: the dead operation is recorded REAPED (a dead owner is never recorded complete)" "$(jq -r .state "$LONGOPS_DIR/ops/$OP2.json" 2>/dev/null)" reaped
sweep; bad_lines=$(ours | grep -E 'duplicate_owner|registry_row_dead_owner|hung_op|orphan')
[ -z "$bad_lines" ] && ok "row recovery: no duplicate_owner, no dead row after the recovery" || bad "row recovery: $(printf '%s' "$bad_lines" | head -2 | cut -c1-200)"
out=$(bash "$UP" --build-id "$A2" --services redis 2>&1); rc=$?; check "row recovery: the next start succeeds (exit 0)" "$rc" 0
OP3=$(opid_of "$out"); TOK="$P2|$OP3"; sweep; [ -z "$(ours)" ] && ok "row recovery: the restarted stack is clean" || bad "row recovery: $(ours | head -2 | cut -c1-200)"
# a claim removed by `reap.sh --purpose` (the old printed recovery) leaves a non-terminal dead operation: down.sh closes it as reaped, not complete
rm -f "$(ti_state "$A2")/lease.keep.$OP3"; lease_dead3() { [ "$(ti_lease_state "$A2" "$OP3")" = dead ]; }; ti_wait_for 30 lease_dead3
# the fixture is the STATE a claim-only recovery leaves (claim removed, operation non-terminal); it is built by removing the claim directory itself because the registry tool's own purpose-reap is another work package's
# and its effect on the operation record changed with it (it now also closes the operation): the subject of this row is what down.sh does with that state, not what reap.sh does
rm -rf -- "${LONGOPS_DIR:?}/claims/$P2"
check "fixture: the claim is gone and the operation is still non-terminal" "$(jq -r .state "$LONGOPS_DIR/ops/$OP3.json" 2>/dev/null)$([ -d "$LONGOPS_DIR/claims/$P2" ] && echo claim || echo noclaim)" runningnoclaim
bash "$DOWN" --build-id "$A2" >/dev/null 2>&1; check "down by a caller naming no operation closes the claim-less dead start (exit 0)" "$?" 0
check "TI-A5: the dead operation of the claim-less project is recorded reaped (not complete)" "$(jq -r .state "$LONGOPS_DIR/ops/$OP3.json" 2>/dev/null)" reaped

# ---- row 6: after down ----
A3=$(ti_new_id); TI_IDS+=("$A3"); P3=$(ti_project "$A3")
out=$(bash "$UP" --build-id "$A3" --services redis 2>&1); OP4=$(opid_of "$out")
bash "$DOWN" --build-id "$A3" --op-id "$OP4" >/dev/null 2>&1
TOK="$P3|$OP4"; sweep; [ -z "$(ours)" ] && ok "row after-down: no drift names the torn-down stack" || bad "row after-down: $(ours | head -2 | cut -c1-200)"
check "TI-A5: an owner teardown records the operation complete" "$(jq -r .state "$LONGOPS_DIR/ops/$OP4.json" 2>/dev/null)" complete

# ---- row 7: a start whose compose `up` fails records `failed`, not complete (TI-A5) ----
WD="$TI_SCRATCH/wrap"; ti_wrap_compose "$WD"; echo 7 >"$WD/up-exit"; echo 1 >"$WD/hold"
A4=$(ti_new_id); TI_IDS+=("$A4"); P4=$(ti_project "$A4")
out=$(PATH="$WD:$PATH" bash "$UP" --build-id "$A4" --services redis 2>&1); rc=$?
check "row failed-start: a start whose compose up fails exits 1" "$rc" 1
OP5=$(ls -t "$LONGOPS_DIR"/ops/"$P4"-up-*.json 2>/dev/null | head -1)
check "TI-A5: the operation of the failed start is recorded failed (never complete)" "$(jq -r .state "$OP5" 2>/dev/null)" failed
check "row failed-start: the real containers the failed start created were torn down" "$(podman ps -a -q --filter "label=catalogizer.test_project=$P4" | wc -l)" 0
TOK="$P4"; sweep; [ -z "$(ours)" ] && ok "row failed-start: no drift names the failed start" || bad "row failed-start: $(ours | head -2 | cut -c1-200)"

# ---- paired mutations ----
if [ "${SWC_NO_MUTATIONS:-0}" != 1 ] && [ "${SWC_TEST_MUTANT:-0}" != 1 ]; then
  mut_batch_begin "${SWC_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"
  KEEPER_BEAT='ids=$(podman ps -q --filter "label=catalogizer.op_id=$op" --filter status=running 2>/dev/null) && [ -n "$ids" ] && bash "$root/scripts/longops/heartbeat.sh" --op-id "$op" >/dev/null 2>&1'
  mut drop_the_heartbeat lib.sh "$KEEPER_BEAT" ':'
  mut beat_without_liveness lib.sh "$KEEPER_BEAT" 'bash "$root/scripts/longops/heartbeat.sh" --op-id "$op" >/dev/null 2>&1'
  # the pre-WF17 behaviour: the client keeps run_pinned's own catalogizer.op_id (an operation nobody registered) and the stack's operation is ADDED under another key (op_id), which the sweep reads second
  mut client_label_added_not_replaced run_client.sh 'if [ "$prev" = --label ] && [[ "$a" == catalogizer.op_id=* ]]; then a="catalogizer.op_id=$OPID"; nrep=$((nrep+1)); fi' 'if [ "$prev" = --label ] && [[ "$a" == catalogizer.op_id=* ]]; then nrep=$((nrep+1)); fi' 'NEW+=(--network "$NET" --env-file "$ENVF" --label "catalogizer.test_project=$P"' 'NEW+=(--network "$NET" --env-file "$ENVF" --label "op_id=$OPID" --label "catalogizer.test_project=$P"'
  mut dead_op_not_closed_before_register up.sh '    ti_lo reap --op-id "$o" >/dev/null 2>&1 && echo "test-infra: closed the operation $o of $P (its owner is proven dead) as reaped" >&2 || { [ "$S_NEW" = 0 ] || rmdir "$S" 2>/dev/null; ti_refuse lease_stale "the dead operation $o of $P could not be reaped; run down.sh --build-id $BID" 4; }' '    :'
  mut claimless_dead_op_recorded_complete down.sh '    [ -z "$o" ] || ti_lo reap --op-id "$o" >/dev/null 2>&1 || echo' '    [ -z "$o" ] || ti_lo release --op-id "$o" --state complete --verdict down >/dev/null 2>&1 || echo'
  mut failed_start_recorded_complete down.sh 'ti_lo release --op-id "$HOLDER" --state "$OUTCOME" --verdict "$REASON"' 'ti_lo release --op-id "$HOLDER" --state complete --verdict "$REASON"'
  mut_id identity_whitespace down.sh 'rc=0
ti_rm_resources "$P"; rr=$?' 'rc=0; :
ti_rm_resources "$P"; rr=$?'
  mut_batch_end "${SWC_EV:+$SWC_EV/sweep-contract-mutations.txt}"
fi
ti_summary
