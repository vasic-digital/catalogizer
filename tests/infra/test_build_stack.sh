#!/usr/bin/env bash
# test_build_stack.sh - WF17 fix round 5: the BUILD stack of scripts/container-build.sh + docker-compose.build.yml (TI-A2, TI-C2, TI-D2 for the build path, TI-F2). The build stack is a registered long
# operation: a lease with a keeper, the op label on every container, `-p <project> --in-pod false` under a scrubbed environment, ONE EXIT trap that on any exit path (compose failure, INT, TERM, HUP) runs
# `down --volumes` for exactly that project, closes the operation and removes the state; redis is password-protected.
# Fault injection at ONE edge (11.4.85): a PATH `podman-compose` wrapper that delegates everything to the REAL tool; its `up` runs the REAL `up -d postgres redis` of the same project and file (real containers),
# holds for a set time, then exits with a set code (WF17 reviewer fixture C1). The real container-build.sh runs from a scratch EXPORT of the working tree (no signing keys, own registry, own state).
# Rows: render (every project=catalogizer service carries the op label of a non-terminal op DURING the run, positive control: the same filter sees them) | failed `up` (exit 7: summary printed, no container, no
# network, no volume, no state, op failed) | INT, TERM, HUP during `up` | a successful run with a hostile exported environment (labels name THIS project; op complete) | two checkouts of the SAME directory name
# building at once (no collision, one's teardown leaves the other's containers) | redis requires authentication (NOAUTH without, PONG with the env file's password).
# Oracle strategy (11.4.245): SPECIFIED by the contract above, INVARIANT (the sweep + the registry agree with podman).
# Paired mutations: the trap's `down` removed (leaks the default network); `-p` dropped; the op not registered; the build compose without its op label; redis without --requirepass; the env scrub removed; identity (must SURVIVE).
# Usage:  test_build_stack.sh   (BLD_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR (mutant dir: scripts + compose/), BLD_EV
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
. "$(dirname "${BASH_SOURCE[0]}")/mutlib.sh"; MUT_ENV=BLD; MUT_SELF="${BASH_SOURCE[0]}"
SD="${TI_SUT_DIR:-scripts/test-infra}"
if [ -n "${TI_SUT_DIR:-}" ]; then CFD="$TI_REPO/$TI_SUT_DIR/compose"; else CFD="$TI_REPO"; fi
CBSH="${CFD}/container-build.sh"; [ -n "${TI_SUT_DIR:-}" ] || CBSH="$TI_REPO/scripts/container-build.sh"
export TI_ROOT="$TI_REPO"
for f in "$TI_REPO/$SD/gen_env.sh" "$CBSH" "$CFD/docker-compose.build.yml"; do [ -f "$f" ] && ok "present: ${f#$TI_REPO/}" || { bad "absent: $f"; ti_summary; exit 1; }; done
LOCKIMG=$(python3 -I -c "import yaml;d=yaml.safe_load(open('$TI_REPO/build/containers/images.lock.yaml'));e=[i for i in d['images'] if i['id']=='IMG-INFRA-REDIS'][0];print(e['reference']+'@'+e['digest'])")
wait_cond() { local n=$1 i=0; shift; while [ "$i" -lt $((n * 20)) ]; do "$@" && return 0; sleep 0.05; i=$((i+1)); done; return 1; }
gone() { ! kill -0 "$1" 2>/dev/null; }
NCO=0
# mkbuildco <dir>: a scratch checkout holding the working-tree scripts the build entry point needs (the compose file and container-build.sh may be a mutant's)
mkbuildco() { local d=$1; mkdir -p "$d/scripts" "$d/.audit" "$d/build/containers"; TI_ROOT_HASHES+=("$(printf '%s' "$(realpath -e -- "$d")" | sha256sum | cut -c1-16)")   # everything this scratch checkout's stacks leave is removed at exit by its root label
  cp -r "$TI_REPO/$SD" "$d/scripts/test-infra"; cp -r "$TI_REPO/scripts/longops" "$d/scripts/"; cp "$CBSH" "$d/scripts/container-build.sh"; cp "$CFD/docker-compose.build.yml" "$d/"
  cp "$TI_REPO/build/containers/images.lock.yaml" "$d/build/containers/"; }
# build_start <checkout> <hold> <up-exit> [VAR=value...]: container-build.sh in its OWN process group with the compose wrapper first in PATH; sets BPID, BPG, WD, CO
build_start() {
  CO=$1; local hold=$2 upx=$3 n; shift 3; NCO=$((NCO+1)); n=$NCO; WD="$TI_SCRATCH/w$n"; ti_wrap_compose "$WD"; echo "$hold" >"$WD/hold"; echo "$upx" >"$WD/up-exit"
  setsid env -u TI_ROOT PATH="$WD:$PATH" "$@" python3 -I -c 'import os,signal,sys; [signal.signal(s, signal.SIG_DFL) for s in (signal.SIGINT, signal.SIGHUP, signal.SIGTERM, signal.SIGQUIT)]; os.execvp("bash", ["bash", "-c", sys.argv[1], "x", sys.argv[2]])' 'cd "$1" && exec bash scripts/container-build.sh 1.0.0 --skip-e2e' "$CO" >"$TI_SCRATCH/cb$n.out" 2>&1 & BPID=$!; TI_BG_PIDS+=("$BPID")
  BOUT="$TI_SCRATCH/cb$n.out"; BPG=$(ps -o pgid= -p "$BPID" 2>/dev/null | tr -d ' ')
}
started() { [ -e "$WD/started" ]; }
# build_facts <checkout>: sets BP (the build project) and BOP (its operation) from the checkout's registry
build_facts() { local f; f=$(ls -t "$1"/.audit/longops/ops/*.json 2>/dev/null | head -1); BOP=$(basename "$f" .json 2>/dev/null); BP=$(jq -r '.purpose_key // empty' "$f" 2>/dev/null); }
leftovers() { # leftovers <project>: containers, pods, networks, volumes of that project
  echo "c=$(podman ps -a -q --filter "label=catalogizer.test_project=$1" | wc -l) p=$(podman pod ls --format '{{.Name}}' | grep -c "^pod_$1\$") n=$(podman network ls --format '{{.Name}}' | grep -c "^${1}_") v=$(podman volume ls -q | grep -c "^${1}_")"; }
sweep_for() { ANTIMESS_ROOT="$1" bash "$TI_REPO/scripts/anti-mess/sweep.sh" --stage cadence --only AM-P1,AM-P2,AM-P3 2>&1 | grep drift | grep -E "$2"; }

# ================= row 1: render with the env file of a real gen_env.sh =================
mkbuildco "$TI_SCRATCH/co0"
GOUT=$(bash "$TI_REPO/$SD/gen_env.sh" --build-id zzrender --op-id "zzrender-op" --env-out "$TI_SCRATCH/render.env" --ports-out "$TI_SCRATCH/render.ports"); TI_IDS+=(zzrender)
ENVR="$TI_SCRATCH/render.env"
podman-compose --in-pod false -p catalogizer-test-zzrender --env-file "$ENVR" -f "$CFD/docker-compose.build.yml" config >"$TI_SCRATCH/render.yml" 2>&1; check "render: the build compose file renders with a real per-run env file" "$?" 0
python3 -I - "$TI_SCRATCH/render.yml" "zzrender-op" <<'PY' >"$TI_SCRATCH/render.verdict" 2>&1
import sys, yaml
d = yaml.safe_load(open(sys.argv[1])); want = sys.argv[2]; bad = []; seen = 0
for n, svc in d["services"].items():
    lab = svc.get("labels") or {}
    if isinstance(lab, list): lab = dict(x.split("=", 1) for x in lab)
    if lab.get("project") == "catalogizer":
        seen += 1
        if lab.get("catalogizer.op_id") != want: bad.append("%s: catalogizer.op_id=%r" % (n, lab.get("catalogizer.op_id")))
        if not lab.get("catalogizer.test_root"): bad.append("%s: no catalogizer.test_root" % n)
print("seen=%d bad=%s" % (seen, ";".join(bad)))
PY
check "render: every service labelled project=catalogizer (postgres, redis) carries the op label of the env file and the checkout label" "$(cat "$TI_SCRATCH/render.verdict")" "seen=2 bad="
check "render control: the filter sees services (a render that listed none would read 'seen=0')" "$(sed -n 's/^seen=\([0-9]*\).*/\1/p' "$TI_SCRATCH/render.verdict" | sed 's/^[1-9][0-9]*$/some/')" some

# ================= row 2: a failed `up` (exit 7) =================
mkbuildco "$TI_SCRATCH/co1"; export LONGOPS_ALLOW_TMPFS=1
build_start "$TI_SCRATCH/co1" 6 7
if wait_cond 120 started; then
  build_facts "$CO"
  check "failed-up: DURING the run the real containers of the build project ARE listed by the label filter (positive control)" "$(podman ps -q --filter "label=catalogizer.test_project=$BP" | wc -l)" 2
  check "failed-up: DURING the run the build's operation is registered and not terminal" "$(jq -r .state "$CO/.audit/longops/ops/$BOP.json" 2>/dev/null | sed 's/^\(registered\|running\)$/live/')" live
  check "failed-up: DURING the run every container carries that operation's label" "$(podman ps -q --filter "label=catalogizer.test_project=$BP" --filter "label=catalogizer.op_id=$BOP" | wc -l)" 2
  d=$(sweep_for "$CO" "$BP|$BOP"); [ -z "$d" ] && ok "failed-up: DURING the run the real sweep reports no drift for the build stack (TI-A2)" || bad "failed-up: sweep drift during the build: $(printf '%s' "$d" | head -2 | cut -c1-200)"
else bad "failed-up: the wrapper's up never started the containers: $(tail -3 "$BOUT" | tr '\n' ' ' | cut -c1-200)"; fi
wait "$BPID"; rc=$?
check "failed-up: container-build.sh exits with the compose exit code 7" "$rc" 7
grep -q 'Build pipeline failed with exit code 7' "$BOUT" && ok "failed-up: the results summary is printed although up failed" || bad "failed-up: no summary: $(tail -3 "$BOUT" | tr '\n' ' ' | cut -c1-200)"
check "failed-up: no container, pod, network or volume of the build project is left" "$(leftovers "$BP")" "c=0 p=0 n=0 v=0"
check "failed-up: the state directory (env file with credentials) is gone" "$([ -e "$CO/.audit/test-infra/$BP" ] && echo left || echo gone)" gone
check "failed-up: the build operation is recorded failed" "$(jq -r .state "$CO/.audit/longops/ops/$BOP.json" 2>/dev/null)" failed
check "failed-up: the lease claim is released" "$([ -d "$CO/.audit/longops/claims/$BP" ] && echo held || echo free)" free
d=$(sweep_for "$CO" "$BP|$BOP"); [ -z "$d" ] && ok "failed-up: the real sweep is clean afterwards" || bad "failed-up: sweep drift afterwards: $(printf '%s' "$d" | head -2 | cut -c1-200)"

# ================= row 3: INT, TERM, HUP during `up` =================
for sg in INT TERM HUP; do
  mkbuildco "$TI_SCRATCH/co-$sg"; build_start "$TI_SCRATCH/co-$sg" 60 0
  if wait_cond 120 started; then
    build_facts "$CO"; kill -s "$sg" -- "-$BPG" 2>/dev/null
    if wait_cond 60 gone "$BPID"; then ok "$sg: container-build.sh exited after the signal"; else bad "$sg: still running 60 s after the signal"; continue; fi
    wait "$BPID" 2>/dev/null
    check "$sg: no container, pod, network or volume of the build project is left" "$(leftovers "$BP")" "c=0 p=0 n=0 v=0"
    check "$sg: the state directory is gone" "$([ -e "$CO/.audit/test-infra/$BP" ] && echo left || echo gone)" gone
    check "$sg: the operation is recorded failed and the claim released" "$(jq -r .state "$CO/.audit/longops/ops/$BOP.json" 2>/dev/null)$([ -d "$CO/.audit/longops/claims/$BP" ] && echo held || echo free)" failedfree
    d=$(sweep_for "$CO" "$BP|$BOP"); [ -z "$d" ] && ok "$sg: the real sweep is clean afterwards" || bad "$sg: sweep drift: $(printf '%s' "$d" | head -2 | cut -c1-200)"
  else bad "$sg: the build never reached its up"; kill -s KILL -- "-$BPG" 2>/dev/null; fi
done

# ================= row 4: a successful run with a HOSTILE exported environment (TI-D2 for the build path) =================
VIC="catalogizer-test-victim$RANDOM$RANDOM"; TI_LABEL_PROJECTS+=("$VIC")   # unique per run (see test_inputs.sh)
mkbuildco "$TI_SCRATCH/co4"; build_start "$TI_SCRATCH/co4" 4 0 TI_PROJECT=$VIC COMPOSE_PROJECT_NAME=victim TI_PORT_REDIS=6379
if wait_cond 120 started; then
  build_facts "$CO"
  check "hostile-env: the containers are labelled with THIS build project, not the exported victim" "$(podman ps -q --filter "label=catalogizer.test_project=$BP" | wc -l)$(podman ps -a -q --filter "label=catalogizer.test_project=$VIC" | wc -l)" 20
  check "hostile-env: the published redis port is the env file's, not the exported 6379" "$(podman ps -q --filter "label=catalogizer.test_project=$BP" --filter ancestor="$LOCKIMG" --format '{{.Ports}}' | grep -c ':6379->')" 0
fi
wait "$BPID"; rc=$?; check "hostile-env: container-build.sh exits 0 on a successful up" "$rc" 0
check "hostile-env: the operation of a successful build is recorded complete" "$(jq -r .state "$CO/.audit/longops/ops/$BOP.json" 2>/dev/null)" complete
check "hostile-env: nothing is left afterwards" "$(leftovers "$BP") state=$([ -e "$CO/.audit/test-infra/$BP" ] && echo left || echo gone)" "c=0 p=0 n=0 v=0 state=gone"

# ================= row 5: two checkouts of the SAME directory name building at once =================
mkdir -p "$TI_SCRATCH/a" "$TI_SCRATCH/b"; mkbuildco "$TI_SCRATCH/a/catalogizer"; mkbuildco "$TI_SCRATCH/b/catalogizer"
build_start "$TI_SCRATCH/a/catalogizer" 4 0; PA_PID=$BPID; WDA=$WD; BA="$BOUT"
build_start "$TI_SCRATCH/b/catalogizer" 14 0; PB_PID=$BPID; WDB=$WD; BB="$BOUT"
sa() { [ -e "$WDA/started" ]; }; sb() { [ -e "$WDB/started" ]; }
if wait_cond 120 sa && wait_cond 120 sb; then
  build_facts "$TI_SCRATCH/a/catalogizer"; PA2=$BP; build_facts "$TI_SCRATCH/b/catalogizer"; PB2=$BP
  check "two-builds: the two checkouts run DISTINCT compose projects" "$([ -n "$PA2" ] && [ "$PA2" != "$PB2" ] && echo distinct || echo same)" distinct
  check "two-builds: both have their containers while both run" "$(podman ps -q --filter "label=catalogizer.test_project=$PA2" | wc -l)$(podman ps -q --filter "label=catalogizer.test_project=$PB2" | wc -l)" 22
  wait "$PA_PID"; check "two-builds: the first build exits 0" "$?" 0
  check "two-builds: the first build's teardown left the SECOND build's containers running" "$(podman ps -q --filter "label=catalogizer.test_project=$PB2" | wc -l)" 2
  check "two-builds: ... and its default network" "$(podman network ls --format '{{.Name}}' | grep -c "^${PB2}_")" 1
else bad "two-builds: a build never started"; fi
wait "$PB_PID" 2>/dev/null; check "two-builds: the second build exits 0" "$?" 0
check "two-builds: nothing of either project is left" "$(leftovers "${PA2:-none}") $(leftovers "${PB2:-none}")" "c=0 p=0 n=0 v=0 c=0 p=0 n=0 v=0"

# ================= row 6: redis of the build stack requires authentication (TI-F2) =================
RID=zzbuildredis; RP=$(ti_project "$RID"); TI_IDS+=("$RID")
bash "$TI_REPO/$SD/gen_env.sh" --build-id "$RID" --op-id "zzbuildredis-op" >/dev/null
RENV=$(ti_envfile "$RID")
# the project's operation must exist for the teardown predicate: register a throwaway one in the registry of this checkout
bash "$TI_REPO/scripts/longops/register.sh" --purpose "$RP" --owner bld-test --op-id zzbuildredis-op --pid $$ --no-claim >/dev/null 2>&1
env -i PATH="$PATH" HOME="$HOME" XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-}" podman-compose --in-pod false -p "$RP" --env-file "$RENV" -f "$CFD/docker-compose.build.yml" up -d redis >"$TI_SCRATCH/redis-up.txt" 2>&1; check "redis: the build compose redis starts" "$?" 0
sed -n 's/^TI_REDIS_PASSWORD=/REDISCLI_AUTH=/p' "$RENV" >"$TI_SCRATCH/redis.auth"
rcli() { podman run --rm --pull=never --network "${RP}_default" "$@" "$LOCKIMG" redis-cli -h redis ping 2>&1; }
ti_wait_for 30 bash -c "podman run --rm --pull=never --network ${RP}_default --env-file $TI_SCRATCH/redis.auth $LOCKIMG redis-cli -h redis ping 2>&1 | grep -q PONG" || true
check "redis: PING without a password is refused (NOAUTH)" "$(rcli | grep -c NOAUTH)" 1
check "redis: PING with the env file's password answers PONG" "$(rcli --env-file "$TI_SCRATCH/redis.auth" | grep -c PONG)" 1
env -i PATH="$PATH" HOME="$HOME" XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-}" podman-compose --in-pod false -p "$RP" --env-file "$RENV" -f "$CFD/docker-compose.build.yml" down --volumes >/dev/null 2>&1
bash "$TI_REPO/scripts/longops/release.sh" --op-id zzbuildredis-op --state complete --verdict test_done >/dev/null 2>&1

# ---------------- paired mutations ----------------
if [ "${BLD_NO_MUTATIONS:-0}" != 1 ] && [ "${BLD_TEST_MUTANT:-0}" != 1 ]; then
  mut_batch_begin "${BLD_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"
  mut trap_does_not_run_down compose/container-build.sh '    if [ "$STACK_UP" = 1 ]; then bc_compose $EXTRA_PROFILES down --volumes >/dev/null 2>&1 || true; fi' '    :' '    if [ -n "$BP" ]; then TI_SCAN_ONLY_OP="$BOP" ti_rm_resources "$BP" >/dev/null 2>&1 || true; fi' '    :'
  mut project_flag_dropped compose/container-build.sh 'COMPOSE_BASE=(-p "$BP");' 'COMPOSE_BASE=();'
  mut build_op_not_registered compose/container-build.sh 'if ! ti_lo register --purpose "$BP" --owner container-build --op-id "$BOP" --pid "$KPID" --container-label "$BP" --no-progress-s "$BUDGET" >/dev/null 2>"$BUILD_STATE_DIR/lease.err"; then' 'if false; then'
  mut build_compose_without_op_label compose/docker-compose.build.yml '  catalogizer.op_id: "${TI_OP_ID:?TI_OP_ID is generated by scripts/test-infra/gen_env.sh}"
' ''
  mut build_redis_without_requirepass compose/docker-compose.build.yml '    command: ["sh", "-c", "exec redis-server --save '"''"' --appendonly no --requirepass \"$$REDIS_PASSWORD\""]
' ''
  mut env_scrub_removed compose/container-build.sh 'bc_compose() { ti_envscrub; "${TI_ESC[@]}" $COMPOSE_CMD' 'bc_compose() { $COMPOSE_CMD'
  mut_id identity_noop compose/container-build.sh 'log_info "Compose file is valid"' 'log_info "Compose file is valid"; :'
  mut_batch_end "${BLD_EV:+$BLD_EV/build-stack-mutations.txt}"
fi
ti_summary
