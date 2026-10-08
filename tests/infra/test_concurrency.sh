#!/usr/bin/env bash
# test_concurrency.sh - T133 (RED first). Two complete stacks at ONCE do not collide: distinct compose projects, host ports, networks, data directories,
# credentials and containers; both are admitted by the per-project lease of T131 while a second owner of EITHER project is refused; both answer their
# protocol probes at the same time; a marker written to stack A (WebDAV file and PostgreSQL row) is absent from stack B. Real rootless podman, real
# services (no mocks). Oracle strategy (11.4.245): SPECIFIED (distinctness and isolation are the requirement) and INVARIANT (the sets are disjoint).
# Evidence: concurrency.json (see below). Paired mutations: a copy of the scripts whose ports are not random (the two stacks must collide) and a copy
# whose lease is keyed on a constant purpose (the second project must be wrongly refused); the body is re-run against each and must FAIL.
# Usage:  test_concurrency.sh            tests, then mutations
# Env:    TI_SUT_DIR (default scripts/test-infra; RED: an absent directory), CONC_EV (directory that receives concurrency.json), CONC_NO_MUTATIONS=1
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"; TI_DOWN="$TI_REPO/$SD/down.sh"
for f in up.sh down.sh probe.sh run_client.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
UP="$TI_REPO/$SD/up.sh"; DOWN="$TI_REPO/$SD/down.sh"; PROBE="$TI_REPO/$SD/probe.sh"; RCL="$TI_REPO/$SD/run_client.sh"
A=$(ti_new_id); B=$(ti_new_id); [ "$A" != "$B" ] || B="${B}b"; TI_IDS+=("$A" "$B"); PA=$(ti_project "$A"); PB=$(ti_project "$B")
claim() { [ -d "$(ti_regdir)/claims/$1" ] && echo held || echo free; }   # only for the ABSENCE of a claim; a held lease is ti_lease_state (a live keeper of the start), WF12 F5
count() { podman ps -a -q --filter "label=catalogizer.test_project=$1" | wc -l; }

T0=$(date +%s)
bash "$UP" --build-id "$A" >"$TI_SCRATCH/upA.txt" 2>&1 & pa=$!
bash "$UP" --build-id "$B" >"$TI_SCRATCH/upB.txt" 2>&1 & pb=$!
TI_BG_PIDS+=("$pa" "$pb")   # the cleanup trap waits for background starts before it tears the ids down (WF17 TI-C7)
wait $pa; ra=$?; wait $pb; rb=$?
T1=$(date +%s)
check "stack A starts (all five services) while B starts (exit 0)" "$ra" 0
check "stack B starts (all five services) while A starts (exit 0)" "$rb" 0
if [ "$ra" != 0 ] || [ "$rb" != 0 ]; then echo "  A said: $(tail -2 "$TI_SCRATCH/upA.txt" | tr '\n' ' ' | cut -c1-240)"; echo "  B said: $(tail -2 "$TI_SCRATCH/upB.txt" | tr '\n' ' ' | cut -c1-240)"; ti_summary; exit 1; fi
OPA=$(sed -n 's/^op_id=//p' "$TI_SCRATCH/upA.txt"); OPB=$(sed -n 's/^op_id=//p' "$TI_SCRATCH/upB.txt")
CA5=$(count "$PA"); CB5=$(count "$PB")
check "A runs 5 containers" "$CA5" 5
check "B runs 5 containers" "$CB5" 5
check "distinct compose projects" "$([ "$PA" != "$PB" ] && echo yes || echo no)" yes
# ports: all twelve (six per stack) distinct, none below 1024
ports=$( { grep -h '^TI_PORT_' "$(ti_envfile "$A")" "$(ti_envfile "$B")" | cut -d= -f2; } | sort -n )
check "twelve host ports across the two stacks" "$(printf '%s\n' "$ports" | wc -l)" 12
check "all host ports distinct" "$(printf '%s\n' "$ports" | sort -u | wc -l)" 12
[ "$(printf '%s\n' "$ports" | sort -u | wc -l)" = 12 ] && PORTS_DISTINCT=yes || PORTS_DISTINCT=no
check "no host port below 1024" "$(printf '%s\n' "$ports" | awk '$1 < 1024' | wc -l)" 0
nets=0; podman network exists "$(ti_network "$A")" && nets=$((nets+1)); podman network exists "$(ti_network "$B")" && nets=$((nets+1))
check "two distinct networks exist" "$nets" 2
check "distinct data directories" "$([ "$(ti_val "$A" TI_DATA_DIR)" != "$(ti_val "$B" TI_DATA_DIR)" ] && echo yes || echo no)" yes
check "distinct credentials" "$([ "$(ti_val "$A" TI_POSTGRES_PASSWORD)" != "$(ti_val "$B" TI_POSTGRES_PASSWORD)" ] && echo yes || echo no)" yes
shared=$(comm -12 <(podman ps -a -q --filter "label=catalogizer.test_project=$PA" | sort) <(podman ps -a -q --filter "label=catalogizer.test_project=$PB" | sort) | wc -l)
check "no container belongs to both projects" "$shared" 0
# leases: both held, a second owner of either refused
check "both leases are live keepers of their own start (holder identity and liveness)" "$(ti_lease_state "$A" "$OPA")$(ti_lease_state "$B" "$OPB")" okok
bash "$UP" --build-id "$A" >/dev/null 2>&1; RC2A=$?; check "a second owner of A is refused (exit 3) while both run" "$RC2A" 3
bash "$UP" --build-id "$B" >/dev/null 2>&1; RC2B=$?; check "a second owner of B is refused (exit 3) while both run" "$RC2B" 3
check "the refused second owners left both live holders alive" "$(ti_lease_state "$A" "$OPA")$(ti_lease_state "$B" "$OPB")" okok
check "the refused second owners changed nothing (still 5 + 5 containers)" "$(count "$PA")$(count "$PB")" 55
# both answer at the same time
bash "$PROBE" --build-id "$A" >"$TI_SCRATCH/prA.txt" 2>&1 & qa=$!
bash "$PROBE" --build-id "$B" >"$TI_SCRATCH/prB.txt" 2>&1 & qb=$!
wait $qa; rpa=$?; wait $qb; rpb=$?
check "A answers all five probes while B is up" "$rpa" 0
PRA=$(grep -E '^PROBES ' "$TI_SCRATCH/prA.txt" | tail -1); PRB=$(grep -E '^PROBES ' "$TI_SCRATCH/prB.txt" | tail -1)
PROBE_MINIO=$(grep -E '^PROBE minio ' "$TI_SCRATCH/prA.txt" | head -1 | cut -c1-80)
check "B answers all five probes while A is up" "$rpb" 0
# isolation: a marker written to A is absent from B (and present in A)
MK="m$(date +%s)$RANDOM"
bash "$RCL" --build-id "$A" -- bash /src/scripts/test-infra/client/marker.sh put "$MK" >"$TI_SCRATCH/mk1.txt" 2>&1; check "marker put on A" "$?" 0
bash "$RCL" --build-id "$A" -- bash /src/scripts/test-infra/client/marker.sh has "$MK" >"$TI_SCRATCH/mk2.txt" 2>&1; check "marker present on A" "$?" 0
bash "$RCL" --build-id "$B" -- bash /src/scripts/test-infra/client/marker.sh hasnot "$MK" >"$TI_SCRATCH/mk3.txt" 2>&1; MKB=$?; check "marker absent from B (webdav and postgres)" "$MKB" 0
MKFA=$(ls "$(ti_val "$A" TI_DATA_DIR)/dav" | grep -c "marker-$MK.txt"); MKFB=$(ls "$(ti_val "$B" TI_DATA_DIR)/dav" | grep -c "marker-$MK.txt")
check "host side: the marker file is in A's data directory" "$MKFA" 1
check "host side: the marker file is NOT in B's data directory" "$MKFB" 0
# teardown of A leaves B running and answering
bash "$DOWN" --build-id "$A" >/dev/null 2>&1; check "down A by a caller naming no operation is REFUSED while A's holder lives (exit 5)" "$?" 5
check "the refused down changed nothing (5 + 5 containers)" "$(count "$PA")$(count "$PB")" 55
bash "$DOWN" --build-id "$A" --op-id "$OPB" >/dev/null 2>&1; check "down A by B's owner is REFUSED (exit 5)" "$?" 5
bash "$DOWN" --build-id "$A" --op-id "$OPA" >/dev/null 2>&1; check "down A by its owner exits 0" "$?" 0
AFTER_B=$(count "$PB"); check "after down A: B still has 5 containers" "$AFTER_B" 5
bash "$PROBE" --build-id "$B" >/dev/null 2>&1; check "after down A: B still answers all probes" "$?" 0
check "after down A: A's lease is free and B's is still a live keeper" "$(claim "$PA")$(ti_lease_state "$B" "$OPB")" freeok
# evidence
if [ -n "${CONC_EV:-}" ] && [ "${CONC_TEST_MUTANT:-0}" != 1 ] && [ "$FAILS" -eq 0 ]; then
  mkdir -p "$CONC_EV"
  # every field below is MEASURED in this run (WF12 F11): counts from podman, exit codes of the refused second owners, the PROBES lines the probes printed, the host-side marker counts
  python3 -I - "$CONC_EV/concurrency.json" "$A" "$B" "$PA" "$PB" "$T0" "$T1" "$ports" "$(git -C "$TI_REPO" rev-parse HEAD)" "$MK" "$CA5" "$CB5" "$RC2A" "$RC2B" "$PRA" "$PRB" "$MKB" "$MKFA" "$MKFB" "$PROBE_MINIO" "$PORTS_DISTINCT" "$AFTER_B" <<'PY'
import json, sys, datetime
out, a, b, pa, pb, t0, t1, ports, head, mk, ca, cb, r2a, r2b, pra, prb, mkb, mkfa, mkfb, minio, distinct, after_b = sys.argv[1:23]
json.dump({"schema": "wp13-concurrency/2", "task": "T133", "head": head, "run_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
           "build_ids": [a, b], "projects": [pa, pb], "started_epoch": int(t0), "both_up_epoch": int(t1),
           "host_ports_all_distinct": distinct == "yes", "host_ports": [int(x) for x in ports.split()],
           "containers_per_project": {pa: int(ca), pb: int(cb)}, "second_owner_refused": {pa: int(r2a), pb: int(r2b)},
           "probes": {pa: pra, pb: prb}, "isolation_marker": mk,
           "marker_files_found_host_side": {"A": int(mkfa), "B": int(mkfb)}, "marker_probe_on_B_exit": int(mkb),
           "teardown_of_A_left_B_containers": int(after_b), "minio": minio}, open(out, "w"), indent=1)
PY
  ok "evidence written: $CONC_EV/concurrency.json"
fi
bash "$DOWN" --build-id "$B" --op-id "$OPB" >/dev/null 2>&1

# ---------------- paired mutations ----------------
if [ "${CONC_NO_MUTATIONS:-0}" != 1 ] && [ "${CONC_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="${CONC_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  mutate() { local name=$1 file=$2 old=$3 new=$4 d="$TI_REPO/.audit/scratch/ti-mut-conc-$1"
    rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/$SD"/*.sh "$TI_REPO/$SD"/*.py "$d/"; cp -r "$TI_REPO/$SD/client" "$d/client"
    python3 -I - "$d/$file" "$old" "$new" <<'PY'
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[2]), sys.argv[2])); sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]))
PY
  }
  run_mut() { local name=$1 out rc i op d=".audit/scratch/ti-mut-conc-$1"
    : >"$TI_SCRATCH/ids-$1.log"
    out=$(TI_IDS_LOG="$TI_SCRATCH/ids-$1.log" TI_SUT_DIR="$d" CONC_TEST_MUTANT=1 CONC_NO_MUTATIONS=1 TI_FAILFAST=1 QUIET=1 bash "${BASH_SOURCE[0]}" 2>&1); rc=$?
    local i; for i in $(cat "$TI_SCRATCH/ids-$1.log"); do TI_DOWN="$TI_REPO/scripts/test-infra/down.sh" ti_down "$i" >/dev/null 2>&1 || ti_scrap "$i"; done
    # a mutant that keyed the lease on a constant purpose leaves that claim and its operation records behind; a dead holder is reaped through the registry's own tool (proven staleness, no signal), or the anti-mess sweep of every stream reports drift
    local f o; sleep 3; for i in $(cat "$TI_SCRATCH/ids-$1.log"); do   # (the keepers leave within a second of their file going)
      for f in "$(ti_regdir)"/ops/catalogizer-test-"$i"-up-*.json; do [ -e "$f" ] || continue
        o="$(jq -r 'select(.state|IN("complete","failed","reaped","handoff","blocked-escape")|not)|.op_id' "$f" 2>/dev/null)"; [ -z "$o" ] || bash "$TI_REPO/scripts/longops/reap.sh" --op-id "$o" >/dev/null 2>&1
      done
    done
    bash "$TI_REPO/scripts/longops/reap.sh" --purpose catalogizer-test >/dev/null 2>&1
    if [ "$rc" -ne 0 ]; then ok "mutation $name CAUGHT ($(printf '%s\n' "$out" | grep -m1 '^FAIL' | cut -c1-100))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${TI_REPO:?}/${d:?}"; }
  mutate fixed_ports gen_env.sh 'set -- $PORTS' 'set -- 40001 40002 40003 40004 40005 40006' && run_mut fixed_ports || bad "mutation fixed_ports: anchor missing"
  mutate constant_lease up.sh 'ti_lo register --purpose "$P" ' 'ti_lo register --purpose catalogizer-test ' && run_mut constant_lease || bad "mutation constant_lease: anchor missing"
  [ -z "${CONC_EV:-}" ] || cp "$MUTLOG" "$CONC_EV/concurrency-mutations.txt"
fi
ti_summary
