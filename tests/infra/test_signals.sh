#!/usr/bin/env bash
# test_signals.sh - WF17 fix round 5, CLASS C (TEARDOWN) closure test for the lifecycle entry points (TI-C1, TI-C7; container-build.sh is driven by test_build_stack.sh, nas_readonly_leg.sh by
# test_nas_readonly_leg.sh). bash runs an EXIT trap on SIGINT, SIGTERM and SIGHUP; only an entry point WITHOUT a trap, or whose trap is armed too late, leaks.
# Rows: up.sh interrupted by INT, TERM and HUP at four OBSERVED stages (the claim exists | the env file exists | a container exists | the readiness probe is running): within 20 s no claim, no keeper process,
# no container of the operation, no state directory, the operation recorded `failed`, and the real anti-mess sweep reports no drift for the stack. setup-test-env.sh interrupted: the same. nfs_attempt.sh
# interrupted while its server stack exists: the same (its trap downs the stack with the operation id up.sh persisted). The test harness itself: TERM while background starts run leaves no container (TI-C7).
# Oracle strategy (11.4.245): SPECIFIED (the up.sh contract: "1 failure (the partial project is torn down)") and INVARIANT (a control: a start that is NOT interrupted leaves a stack the same predicates see).
# The signal goes to the PROCESS GROUP of the entry point (a terminal's Ctrl-C reaches the whole group), delivered at a stage observed in the real stack — not at a sleep.
# Paired mutations: the up.sh EXIT trap removed; armed only after compose up (stage 1 and 2 must FAIL); fail_down without the operation id; identity mutant (must SURVIVE).
# Usage:  test_signals.sh   (SIG_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR, SIG_EV
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
. "$(dirname "${BASH_SOURCE[0]}")/mutlib.sh"; MUT_ENV=SIG; MUT_SELF="${BASH_SOURCE[0]}"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"; TI_DOWN="$TI_REPO/$SD/down.sh"
for f in up.sh down.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
UP="$TI_REPO/$SD/up.sh"; DOWN="$TI_REPO/$SD/down.sh"
export LONGOPS_DIR="$TI_SCRATCH/lo" LONGOPS_ALLOW_TMPFS=1
wait_cond() { local n=$1 i=0; shift; while [ "$i" -lt $((n * 20)) ]; do "$@" && return 0; sleep 0.05; i=$((i+1)); done; return 1; }   # polls every 50 ms
gone() { ! kill -0 "$1" 2>/dev/null; }
sweep_ours() { bash "$TI_REPO/scripts/anti-mess/sweep.sh" --stage cadence --only AM-P1,AM-P2,AM-P3 2>&1 | grep -E "drift" | grep -E "$1"; }
# interrupt <label> <signal> <stage> <entry command...>: starts the entry point in its own process group, signals the group at the observed stage, asserts the end state
interrupt() {
  local label=$1 sig=$2 stage=$3 id P S pid pg op sp; shift 3
  id=$(ti_new_id); TI_IDS+=("$id"); P=$(ti_project "$id"); S=$(ti_state "$id")
  local claim="$(ti_regdir)/claims/$P/holder.json"
  s1() { [ -s "$claim" ] && jq -e .run_id "$claim" >/dev/null 2>&1; };   # stage 1 = a COMPLETE claim record: the file appears before its content is written, and a signal inside that window leaves a truncated holder record that the registry (rightly) refuses to release on a guess s2() { [ -r "$S/env" ]; }; s3() { [ -n "$(podman ps -a -q --filter "label=catalogizer.test_project=$P" 2>/dev/null)" ]; }; s4() { [ -e "$S/ready.log" ]; }
  # a background job of a non-interactive shell inherits SIGINT as IGNORED (and bash cannot reset an inherited ignore): restore the default disposition first, as a foreground Ctrl-C would find it
  setsid python3 -I -c 'import os,signal,sys; [signal.signal(s, signal.SIG_DFL) for s in (signal.SIGINT, signal.SIGHUP, signal.SIGTERM, signal.SIGQUIT)]; os.execvp(sys.argv[1], sys.argv[1:])' "$@" --build-id "$id" >"$TI_SCRATCH/sg-$label.out" 2>&1 & pid=$!; TI_BG_PIDS+=("$pid")
  if ! wait_cond 90 "s$stage"; then bad "$label: stage $stage was never observed (the start ended or stalled first): $(tail -2 "$TI_SCRATCH/sg-$label.out" | tr '\n' ' ' | cut -c1-160)"; wait "$pid" 2>/dev/null; return; fi
  op=$(jq -r '.run_id // empty' "$claim" 2>/dev/null)
  pg=$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d ' ')
  if [ -z "$pg" ] || [ "$pg" -le 1 ]; then bad "$label: cannot resolve the process group of the entry point (pgid '$pg')"; return; fi
  kill -s "$sig" -- "-$pg" 2>/dev/null
  if wait_cond 20 gone "$pid"; then ok "$label: the entry point exited within 20 s of $sig at stage $stage"; else bad "$label: the entry point is still running 20 s after $sig"; return; fi
  wait "$pid" 2>/dev/null
  # the entry point may exit before the processes it started finish their own teardown (setup-test-env.sh has no trap of its own; up.sh, a child of it, tears down): the end state is judged once the whole process group is gone
  pgempty() { [ -z "$(ps -eo pgid= 2>/dev/null | awk -v g="$pg" '$1 == g' | head -1)" ]; }
  wait_cond 90 pgempty && ok "$label: every process of the entry point's group is gone within 90 s" || bad "$label: a process of the entry point's group is still running 90 s after $sig"
  check "$label: no claim of the project is left" "$([ -d "$(ti_regdir)/claims/$P" ] && echo held || echo free)" free
  if [ -n "$op" ]; then
    nokeeper() { [ -z "$(ti_proc_by_arg "lease.keep.$op")" ]; }; wait_cond 6 nokeeper
    check "$label: no lease-keeper process of the operation is left" "$(ti_proc_by_arg "lease.keep.$op" | wc -l)" 0
    check "$label: no container labelled with the operation is left" "$(podman ps -a -q --filter "label=catalogizer.op_id=$op" | wc -l)" 0
    if [ -f "$(ti_regdir)/ops/$op.json" ]; then check "$label: the operation is recorded failed" "$(jq -r .state "$(ti_regdir)/ops/$op.json")" failed; else ok "$label: no operation record was written (the signal came before it)"; fi
  fi
  check "$label: no container of the project is left" "$(podman ps -a -q --filter "label=catalogizer.test_project=$P" | wc -l)" 0
  check "$label: the state directory (credentials, data) is gone" "$([ -e "$S" ] && echo left || echo gone)" gone
  sp=$(sweep_ours "$P|${op:-NOOP}"); [ -z "$sp" ] && ok "$label: the real anti-mess sweep reports no drift for the stack" || bad "$label: sweep drift: $(printf '%s' "$sp" | head -2 | cut -c1-200)"
}
for st in 1 2 3 4; do for sg in INT TERM HUP; do interrupt "up-s$st-$sg" "$sg" "$st" bash "$UP" --services redis; done; done
# control: a start that is NOT interrupted leaves a stack the same predicates see (the instrument is not blind to a live stack)
id=$(ti_new_id); TI_IDS+=("$id"); P=$(ti_project "$id")
out=$(bash "$UP" --build-id "$id" --services redis 2>&1); rc=$?; op=$(printf '%s\n' "$out" | sed -n 's/^op_id=//p')
check "control: an uninterrupted start exits 0" "$rc" 0
check "control: the predicates see its keeper, container and claim" "$(ti_proc_by_arg "lease.keep.$op" | wc -l | sed 's/^0$/none/;s/^[1-9].*/some/')$(podman ps -q --filter "label=catalogizer.op_id=$op" | wc -l)$([ -d "$(ti_regdir)/claims/$P" ] && echo held)" some1held
bash "$DOWN" --build-id "$id" --op-id "$op" >/dev/null 2>&1

# ---- setup-test-env.sh interrupted at the container stage ----
interrupt "setup-test-env-s3-TERM" TERM 3 bash "$TI_REPO/scripts/setup-test-env.sh" --services redis
# ---- nfs_attempt.sh interrupted while its server stack exists ----
if [ -n "$(podman images -q localhost/catalogizer-infra-nfs 2>/dev/null)" ]; then
  NEV="$TI_REPO/.audit/scratch/sig-nfs-$$"; mkdir -p "$NEV"; TI_FOREIGN_DIRS+=("$NEV"); echo '{"verdict":"VERIFIED"}' >"$NEV/client.json"
  interrupt "nfs_attempt-s3-TERM" TERM 3 bash "$TI_REPO/$SD/nfs_attempt.sh" --ev-dir "$NEV/ev" --client-json "$NEV/client.json"
else blocked "nfs_attempt interrupt row: the nfs server image is not built on this host"; fi

# ---- the harness: TERM while background starts run leaves no container (TI-C7) ----
HID=$(ti_new_id); HP=$(ti_project "$HID"); TI_IDS+=("$HID")
cat >"$TI_SCRATCH/harness.sh" <<EOF
#!/usr/bin/env bash
. "$TI_REPO/tests/infra/lib.sh"
export TI_ROOT="$TI_REPO"
TI_IDS+=("$HID")
bash "$UP" --build-id "$HID" --services redis >"$TI_SCRATCH/harness-up.txt" 2>&1 &
TI_BG_PIDS+=(\$!)
wait
EOF
setsid bash "$TI_SCRATCH/harness.sh" >/dev/null 2>&1 & HPID=$!; TI_BG_PIDS+=("$HPID")
hs() { [ -f "$(ti_regdir)/claims/$HP/holder.json" ]; }
if wait_cond 60 hs; then
  kill -s TERM "$HPID" 2>/dev/null   # TERM to the harness shell only: its EXIT trap (ti_cleanup) must wait for the background start, then tear it down
  wait_cond 120 gone "$HPID" && ok "harness: the test shell exited after TERM" || bad "harness: still running 120 s after TERM"
  check "harness: no container of the background start is left after the harness cleanup (TI-C7)" "$(podman ps -a -q --filter "label=catalogizer.test_project=$HP" | wc -l)" 0
  check "harness: no claim is left" "$([ -d "$(ti_regdir)/claims/$HP" ] && echo held || echo free)" free
else bad "harness: the background start never took its lease"; fi

# ---- paired mutations ----
if [ "${SIG_NO_MUTATIONS:-0}" != 1 ] && [ "${SIG_TEST_MUTANT:-0}" != 1 ]; then
  mut_batch_begin "${SIG_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"
  FDTRAP='trap '"'"'fail_down interrupted "interrupted or exited early (signal or unexpected exit)"'"'"' EXIT'
  mut no_exit_trap up.sh "$FDTRAP" ':' 'trap '"'"'up_early_cleanup'"'"' EXIT' ':'
  mut trap_armed_after_compose_up up.sh "$FDTRAP" ':' '# ---- wait for protocol-level readiness ----' "$FDTRAP"'
# ---- wait for protocol-level readiness ----'
  mut early_trap_does_not_release_claim up.sh '  ti_lo release --op-id "$OPID" --state failed --verdict interrupted_before_registration >/dev/null 2>&1 || ti_lo release --purpose "$P" --run-id "$OPID" >/dev/null 2>&1 || true' '  :'
  mut signals_not_converted_to_exit up.sh "ti_exit_on_signals
trap 'up_early_cleanup' EXIT" "trap 'up_early_cleanup' EXIT"
  mut_id identity_noop_in_up up.sh 'echo "test-infra: registered op_id=$OPID" >&2' 'echo "test-infra: registered op_id=$OPID" >&2; :'
  mut_batch_end "${SIG_EV:+$SIG_EV/signals-mutations.txt}"
fi
ti_summary
