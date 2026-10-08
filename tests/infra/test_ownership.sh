#!/usr/bin/env bash
# test_ownership.sh - WF17 fix round 5, CLASS B (OWN) closure test (TI-B1..TI-B8). A resource is torn down only by the checkout and the operation that own it; a failing podman query is UNKNOWN, never "nothing
# there"; the lease is released LAST; the registry path is the one scripts/longops/lib.sh derives. Every row drives the real scripts on real rootless podman, with real redis stacks.
# Rows: two checkouts (an export of the working tree is a second checkout with its own registry) | the three label layers of the ownership predicate, each with its own fixture container (project label, checkout
# root label, operation of THIS registry) | a down while a start is in progress | the down window (a start during the teardown) | podman answers 125 (fault injection at one edge) | partial labels | a relocated
# registry (LONGOPS_AUDIT / LONGOPS_REPO) | an unreadable holder record.
# Oracle strategy (11.4.245): SPECIFIED (the ownership rule of docs/testing/real-service-stack.md) and INVARIANT (the instrument sees: a control refusal for the owner's own no-op-id down).
# Paired mutations: each label layer relaxed alone; the lock and the start-in-progress check removed; the release moved before the cleanup with the lock gone; the podman status check dropped; the registry path
# hard-coded again; an unreadable holder mapped back to not_lease_owner; identity mutant (must SURVIVE).
# Usage:  test_ownership.sh   (OWN_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR, OWN_EV
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
. "$(dirname "${BASH_SOURCE[0]}")/mutlib.sh"; MUT_ENV=OWN; MUT_SELF="${BASH_SOURCE[0]}"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"; TI_DOWN="$TI_REPO/$SD/down.sh"
for f in up.sh down.sh sweep_leaks.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
UP="$TI_REPO/$SD/up.sh"; DOWN="$TI_REPO/$SD/down.sh"; SWEEP="$TI_REPO/$SD/sweep_leaks.sh"
LOCKIMG=$(python3 -I -c "import yaml;d=yaml.safe_load(open('$TI_REPO/build/containers/images.lock.yaml'));e=[i for i in d['images'] if i['id']=='IMG-INFRA-REDIS'][0];print(e['reference']+'@'+e['digest'])")
count() { podman ps -a -q --filter "label=catalogizer.test_project=$1" | wc -l; }
opid_of() { printf '%s\n' "$1" | sed -n 's/^op_id=//p'; }
running() { podman inspect --format '{{.State.Running}}' "$1" 2>/dev/null; }
# a second checkout: an export of the WORKING TREE files the stack scripts need (so the scripts under test, not HEAD's, run there); its registry, state and root hash are its own
mkcheckout() { local d=$1; mkdir -p "$d/scripts" "$d/build/containers" "$d/.audit"
  cp -r "$TI_REPO/scripts/test-infra" "$TI_REPO/scripts/longops" "$TI_REPO/scripts/anti-mess" "$TI_REPO/scripts/containers" "$d/scripts/"
  cp "$TI_REPO/build/containers/images.lock.yaml" "$d/build/containers/"; cp "$TI_REPO"/docker-compose.test-infra*.yml "$d/"
  if [ -d "$TI_REPO/$SD" ] && [ "$SD" != scripts/test-infra ]; then rm -rf "$d/scripts/test-infra"; cp -r "$TI_REPO/$SD" "$d/scripts/test-infra"; fi; }
CACHE="$(ls -dt "$TI_REPO"/.audit/out/test-infra-corpus-* 2>/dev/null | head -1)"
[ -n "$CACHE" ] || { bad "no seeded corpus cache to reuse (run one up.sh first)"; ti_summary; exit 1; }
THROW="zz-own-op-$$"; bash "$TI_REPO/scripts/longops/register.sh" --purpose "$THROW" --owner own-test --op-id "$THROW" --pid $$ --no-claim >/dev/null 2>&1   # an operation of THIS registry for the fixtures
trap 'bash "$TI_REPO/scripts/longops/release.sh" --op-id "$THROW" --state complete --verdict test_done >/dev/null 2>&1; ti_cleanup' EXIT

# ================= row 1: two checkouts =================
X=$(ti_new_id); TI_IDS+=("$X"); PX=$(ti_project "$X"); CB="$TI_SCRATCH/chkB"; mkcheckout "$CB"
BENV=(env TI_ROOT="$CB" TI_CORPUS_CACHE_DIR="$CACHE")
out=$("${BENV[@]}" bash "$CB/scripts/test-infra/up.sh" --build-id "$X" --services redis 2>&1); rc=$?; check "checkout B: up exits 0" "$rc" 0
[ "$rc" = 0 ] || { echo "  B up said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"; ti_summary; exit 1; }
OPB=$(opid_of "$out"); CIDB=$(podman ps -q --filter "label=catalogizer.test_project=$PX" | head -1)
bl_ok() { local f="$CB/.audit/longops/claims/$PX/holder.json" pid; [ -f "$f" ] && pid=$(jq -r .pid "$f") && [ -r "/proc/$pid/cmdline" ] && [ "$(jq -r .run_id "$f")" = "$OPB" ] && echo ok || echo bad; }
check "checkout B holds a live lease of its own registry" "$(bl_ok)" ok
check "B's container carries B's root hash, not A's" "$([ "$(podman inspect --format '{{index .Config.Labels "catalogizer.test_root"}}' "$CIDB")" = "$(env TI_ROOT="$CB" bash -c ". $TI_REPO/$SD/lib.sh; ti_root_hash")" ] && echo b || echo other)" b
out=$(bash "$DOWN" --build-id "$X" 2>&1); rc=$?
check "A: down of B's project, naming no operation, is REFUSED foreign_owner (exit 5)" "$rc" 5
case "$out" in *reason=foreign_owner*) ok "the refusal names reason=foreign_owner";; *) bad "no foreign_owner in: $(printf '%s' "$out" | tail -2 | tr '\n' ' ' | cut -c1-200)";; esac
check "A's refused down touched nothing: B's container still runs" "$(running "$CIDB")" true
check "A's refused down left B's lease intact" "$(bl_ok)" ok
out=$(bash "$UP" --build-id "$X" --services redis 2>&1); rc=$?
check "A: up of the SAME project id is REFUSED foreign_owner (exit 3), not a teardown of B (TI-B1 b)" "$rc" 3
check "A's refused up left B's container running and B's lease intact" "$(running "$CIDB")$(bl_ok)" trueok
check "A's refused up recorded its own operation failed and released its own claim" "$([ -d "$(ti_regdir)/claims/$PX" ] && echo held || echo free)" free
out=$("${BENV[@]}" bash "$CB/scripts/test-infra/down.sh" --build-id "$X" 2>&1); rc=$?
check "control: B's own down WITHOUT its operation id is refused not_lease_owner (exit 5): the instrument sees refusals" "$rc" 5
case "$out" in *reason=not_lease_owner*) ok "control: the refusal names not_lease_owner";; *) bad "control refusal text: $(printf '%s' "$out" | tail -1 | cut -c1-160)";; esac
"${BENV[@]}" bash "$CB/scripts/test-infra/down.sh" --build-id "$X" --op-id "$OPB" >/dev/null 2>&1; check "B's own down by its owner exits 0" "$?" 0
check "default ids carry a per-checkout part (setup-test-env.sh / nfs_attempt.sh): no fixed-second id" "$(grep -c 'ROOT_HASH\|root_hash' "$TI_REPO/scripts/setup-test-env.sh" "$TI_REPO/$SD/nfs_attempt.sh" | awk -F: '{s+=$2} END {print (s>=2) ? "yes" : "no"}')" yes

# ================= row 2: the three label layers, each with its own fixture =================
Y=$(ti_new_id); TI_IDS+=("$Y"); PY=$(ti_project "$Y"); ROOTA="$(env TI_ROOT="$TI_REPO" bash -c ". $TI_REPO/$SD/lib.sh; ti_root_hash")"
mkfx() { podman create --pull=never "$@" --entrypoint sleep "$LOCKIMG" 600 2>/dev/null; }
layer() { # layer <name> <expected container label sets...>: one foreign container; down must refuse it (exit 5, foreign_owner naming it) and leave it
  local name=$1 fx rc out; shift
  fx=$(mkfx --name "$PY-$name" "$@"); TI_FOREIGN+=("$fx"); podman start "$fx" >/dev/null 2>&1
  out=$(bash "$DOWN" --build-id "$Y" 2>&1); rc=$?
  check "layer $name: down refuses the foreign container (exit 5)" "$rc" 5
  case "$out" in *reason=foreign_owner*) ok "layer $name: reason=foreign_owner";; *) bad "layer $name: no foreign_owner in $(printf '%s' "$out" | tail -2 | tr '\n' ' ' | cut -c1-200)";; esac
  check "layer $name: the foreign container is untouched (still running)" "$(running "$fx")" true
  podman rm -f "$fx" >/dev/null 2>&1
}
layer no_project_label --label "catalogizer.test_project=$PY" --label "catalogizer.test_root=$ROOTA" --label "catalogizer.op_id=$THROW"
layer other_root --label project=catalogizer --label "catalogizer.test_project=$PY" --label "catalogizer.test_root=deadbeefdeadbeef" --label "catalogizer.op_id=$THROW"
layer op_not_in_registry --label project=catalogizer --label "catalogizer.test_project=$PY" --label "catalogizer.test_root=$ROOTA" --label "catalogizer.op_id=zz-no-such-op-$$"
fo=$(mkfx --name "$PY-owned" --label project=catalogizer --label "catalogizer.test_project=$PY" --label "catalogizer.test_root=$ROOTA" --label "catalogizer.op_id=$THROW"); TI_FOREIGN+=("$fo")
out=$(bash "$DOWN" --build-id "$Y" 2>&1); rc=$?
check "control: a container with ALL layers right IS removed by the same down (exit 0): the layer fixtures above are refusals, not blindness" "$rc$(podman container exists "$fo" 2>/dev/null && echo kept || echo gone)" 0gone
# up refuses a partial-label container (TI-B6) and names it
fp=$(mkfx --name "$PY-partial" --label "catalogizer.test_project=$PY"); TI_FOREIGN+=("$fp")
out=$(bash "$UP" --build-id "$Y" --services redis 2>&1); rc=$?
check "partial labels: up is REFUSED foreign_owner (exit 3), never an endless exit 1" "$rc" 3
case "$out" in *"${fp:0:12}"*|*"$fp"*) ok "partial labels: the refusal names the container";; *) bad "partial labels: container not named in $(printf '%s' "$out" | tail -2 | tr '\n' ' ' | cut -c1-200)";; esac
check "partial labels: the fixture is untouched" "$(podman container exists "$fp" 2>/dev/null && echo kept || echo gone)" kept
podman rm -f "$fp" >/dev/null 2>&1

# ================= row 3: the down window (a start during a teardown) =================
W=$(ti_new_id); TI_IDS+=("$W"); PW=$(ti_project "$W")
out=$(bash "$UP" --build-id "$W" --services redis 2>&1); OPW1=$(opid_of "$out")
( TI_TEST_SLEEP_BEFORE_RELEASE=6 bash "$DOWN" --build-id "$W" --op-id "$OPW1" >"$TI_SCRATCH/dw.txt" 2>&1 ) & DP=$!; TI_BG_PIDS+=("$DP")
sleep 2   # inside the 6 s window: the cleanup of the first start is done or in progress, its lease not yet released (mutant: released)
out=$(bash "$UP" --build-id "$W" --services redis 2>&1); rc=$?; OPW2=$(opid_of "$out")
wait "$DP"; drc=$?
check "the first down exits 0" "$drc" 0
if [ "$rc" = 3 ]; then ok "down window: the second start was REFUSED lease_held while the teardown ran (exit 3)"
elif [ "$rc" = 0 ]; then ok "down window: the second start WAITED for the teardown (project lock) and started cleanly (exit 0)"
else bad "down window: the second start exited $rc ($(printf '%s' "$out" | tail -2 | tr '\n' ' ' | cut -c1-160))"; fi
if [ "$rc" = 0 ]; then
  check "down window: the second owner's lease is a live keeper of its own start" "$(ti_lease_state "$W" "$OPW2")" ok
  check "down window: the second owner's env file survived the first teardown" "$([ -r "$(ti_envfile "$W")" ] && echo present || echo destroyed)" present
  check "down window: the second owner's container runs" "$(count "$PW")" 1
  bash "$DOWN" --build-id "$W" --op-id "$OPW2" >/dev/null 2>&1
else ti_down "$W" >/dev/null 2>&1; fi

# ================= row 4: a down during a start (TI-B3) =================
V=$(ti_new_id); TI_IDS+=("$V"); PV=$(ti_project "$V")
( LONGOPS_TEST_SLEEP_IN_CS=4 bash "$UP" --build-id "$V" --services redis >"$TI_SCRATCH/dv.txt" 2>&1 ) & UPID=$!; TI_BG_PIDS+=("$UPID")
sleep 1
out=$(bash "$DOWN" --build-id "$V" 2>&1); rc=$?
check "down during a start, naming no operation, is REFUSED (exit 5: start_in_progress or not_lease_owner)" "$rc" 5
wait "$UPID"; urc=$?
check "the start finished exit 0 although a down ran during it" "$urc" 0
OPV=$(opid_of "$(cat "$TI_SCRATCH/dv.txt")")
check "the start holds a LIVE lease of its own (holder identity and liveness)" "$(ti_lease_state "$V" "$OPV")" ok
check "its container runs" "$(count "$PV")" 1
ti_down "$V" >/dev/null 2>&1

# ================= row 5: podman answers 125 =================
U=$(ti_new_id); TI_IDS+=("$U"); PU=$(ti_project "$U")
out=$(bash "$UP" --build-id "$U" --services redis 2>&1); OPU=$(opid_of "$out"); CIDU=$(podman ps -q --filter "label=catalogizer.test_project=$PU" | head -1)
WP="$TI_SCRATCH/wrapp"; ti_wrap_podman "$WP"; : >"$WP/fault-ps"
out=$(PATH="$WP:$PATH" bash "$DOWN" --build-id "$U" --op-id "$OPU" 2>&1); rc=$?
check "podman unknown: the owner's down is NOT a success while podman cannot list containers (exit 1)" "$rc" 1
check "podman unknown: the lease is still held" "$(ti_lease_state "$U" "$OPU")" ok
check "podman unknown: the state directory (credentials) is still there" "$([ -r "$(ti_envfile "$U")" ] && echo present || echo gone)" present
check "podman unknown: the container still runs" "$(running "$CIDU")" true
RP="zzsw$RANDOM"; mkdir -p "$TI_REPO/.audit/out/catalogizer-test-${RP}-client"; echo x >"$TI_REPO/.audit/out/catalogizer-test-${RP}-client/f"; TI_FOREIGN_DIRS+=("$TI_REPO/.audit/out/catalogizer-test-${RP}-client")
out=$(PATH="$WP:$PATH" bash "$SWEEP" 2>&1); rc=$?
case "$out" in *"KEPT dir catalogizer-test-${RP}-client failed=podman_query_failed"*) ok "podman unknown: the sweep KEEPs a leaked-looking directory (failed=podman_query_failed)";; *) bad "podman unknown: the sweep did not keep the directory for the right reason: $(printf '%s' "$out" | grep "${RP}" | head -1 | cut -c1-200)";; esac
check "podman unknown: the leaked-looking directory is still there" "$([ -e "$TI_REPO/.audit/out/catalogizer-test-${RP}-client" ] && echo kept || echo removed)" kept
rm -f "$WP/fault-ps"
bash "$SWEEP" 2>&1 | grep -q "REMOVED dir catalogizer-test-${RP}-client" && ok "control: with podman answering, the same sweep removes that directory (the fixture is a real leak)" || bad "control: the sweep did not remove the leaked directory once podman answered"
bash "$DOWN" --build-id "$U" --op-id "$OPU" >/dev/null 2>&1; check "with podman answering the owner's down exits 0" "$?" 0

# ================= row 6: a relocated registry (TI-B4) =================
for mode in AUDIT REPO; do
  R=$(ti_new_id); TI_IDS+=("$R"); PR=$(ti_project "$R")
  if [ "$mode" = AUDIT ]; then RELOC="$TI_SCRATCH/reloc-audit"; mkdir -p "$RELOC"; REGDIR="$RELOC/longops"; RENV=(env LONGOPS_AUDIT="$RELOC")
  else RELOC="$TI_SCRATCH/reloc-repo"; mkdir -p "$RELOC/.audit"; REGDIR="$RELOC/.audit/longops"; RENV=(env LONGOPS_REPO="$RELOC"); fi
  out=$("${RENV[@]}" bash "$UP" --build-id "$R" --services redis 2>&1); rc=$?; check "relocated registry ($mode): up exits 0" "$rc" 0
  ROP=$(opid_of "$out")
  check "relocated registry ($mode): the claim is at the relocated path" "$([ -d "$REGDIR/claims/$PR" ] && echo there || echo missing)" there
  out=$("${RENV[@]}" bash "$DOWN" --build-id "$R" 2>&1); rc=$?
  check "relocated registry ($mode): a no-operation-id down is refused (exit 5)" "$rc" 5
  check "relocated registry ($mode): the refused down touched nothing (claim and container)" "$([ -d "$REGDIR/claims/$PR" ] && echo there)$(count "$PR")" there1
  "${RENV[@]}" bash "$DOWN" --build-id "$R" --op-id "$ROP" >/dev/null 2>&1; check "relocated registry ($mode): the owner's down exits 0" "$?" 0
  check "relocated registry ($mode): the owner's down released the claim at the RELOCATED path" "$([ -d "$REGDIR/claims/$PR" ] && echo held || echo free)" free
  check "relocated registry ($mode): the operation is complete at the relocated path" "$(jq -r .state "$REGDIR/ops/$ROP.json" 2>/dev/null)" complete
done

# ================= row 7: an unreadable holder record (TI-B8) =================
H=$(ti_new_id); TI_IDS+=("$H"); PH=$(ti_project "$H")
out=$(bash "$UP" --build-id "$H" --services redis 2>&1); OPH=$(opid_of "$out"); HF="$(ti_regdir)/claims/$PH/holder.json"; cp "$HF" "$TI_SCRATCH/holder.bak"; printf '{' >"$HF"
out=$(bash "$DOWN" --build-id "$H" --op-id "$OPH" 2>&1); rc=$?
check "unreadable holder record: the true owner's down is refused (exit 5)" "$rc" 5
case "$out" in *reason=holder_record_unreadable*) ok "unreadable holder record: the reason is holder_record_unreadable (not not_lease_owner)";; *) bad "unreadable holder record: wrong reason: $(printf '%s' "$out" | tail -1 | cut -c1-200)";; esac
cp "$TI_SCRATCH/holder.bak" "$HF"; bash "$DOWN" --build-id "$H" --op-id "$OPH" >/dev/null 2>&1; check "restored record: the owner's down exits 0" "$?" 0

# ---------------- paired mutations ----------------
if [ "${OWN_NO_MUTATIONS:-0}" != 1 ] && [ "${OWN_TEST_MUTANT:-0}" != 1 ]; then
  mut_batch_begin "${OWN_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"
  mut layer_project_label_dropped lib.sh 'if [ "$pj" != catalogizer ] || [ "$tp" != "$P" ]; then' 'if false; then'
  mut layer_root_label_dropped lib.sh 'elif [ "$tr" != "$root" ]; then' 'elif false; then'
  mut layer_registry_op_dropped lib.sh 'elif [ -z "$op" ] || [ ! -e "$TI_LD_CACHE/ops/$op.json" ]; then' 'elif false; then'
  mut release_before_cleanup_no_lock down.sh '[ -z "${TI_TEST_SLEEP_BEFORE_RELEASE:-}" ] || sleep "$TI_TEST_SLEEP_BEFORE_RELEASE"' 'ti_lo release --op-id "$HOLDER" --state complete --verdict down >/dev/null 2>&1; ti_unlock; [ -z "${TI_TEST_SLEEP_BEFORE_RELEASE:-}" ] || sleep "$TI_TEST_SLEEP_BEFORE_RELEASE"'
  mut down_ignores_start_in_progress down.sh 'ti_lock "$P"   # held for the whole run (released with the process): a start of the same project waits, a down of another checkout waits' ':' '  if KF="$(ti_live_keeper_in "$S")"; then ti_refuse start_in_progress "a start of $P is in progress (live keeper $KF); wait for it or interrupt it" 5; fi' '  :'
  # TI-B3: the up-side lease verification AND the down-side lock and start-in-progress check removed together (either side alone is sufficient: defence in depth), so a down during a start destroys the start
  mut up_unverified_and_down_unguarded up.sh 'ti_holder_ok "$P" "$OPID" "$KPID" || fail_down lease_lost "the lease of $P no longer names this start'"'"'s live keeper"' ':' @@FILE down.sh 'ti_lock "$P"   # held for the whole run (released with the process): a start of the same project waits, a down of another checkout waits' ':' '  if KF="$(ti_live_keeper_in "$S")"; then ti_refuse start_in_progress "a start of $P is in progress (live keeper $KF); wait for it or interrupt it" 5; fi' '  :'
  mut podman_status_dropped lib.sh 'ids="$(podman ps -a -q --filter "label=catalogizer.test_project=$P" 2>/dev/null)" || { TI_UNKNOWN="podman ps"; return 3; }' 'ids="$(podman ps -a -q --filter "label=catalogizer.test_project=$P" 2>/dev/null)"'
  mut sweep_live_ignores_podman_failure sweep_leaks.sh '  local ids; ids="$(podman ps -a -q --filter "label=catalogizer.test_project=$1" 2>/dev/null)" || { echo "podman_query_failed"; return; }' '  local ids; ids="$(podman ps -a -q --filter "label=catalogizer.test_project=$1" 2>/dev/null)"'
  mut registry_path_hard_coded down.sh 'ti_ld_init; LD="$TI_LD_CACHE"' 'LD="${LONGOPS_DIR:-$TI_ROOT/.audit/longops}"'
  mut unreadable_holder_not_named down.sh 'if [ -z "$HOLDER" ]; then ti_refuse holder_record_unreadable' 'if false; then ti_refuse holder_record_unreadable'
  mut_id identity_noop_in_down down.sh 'rc=0
ti_rm_resources "$P"; rr=$?' 'rc=0; :
ti_rm_resources "$P"; rr=$?'
  mut_batch_end "${OWN_EV:+$OWN_EV/ownership-mutations.txt}"
fi
ti_summary
