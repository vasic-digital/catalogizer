#!/usr/bin/env bash
# test_up_down.sh - T131 (RED first). Oracle for scripts/test-infra/up.sh and down.sh: the per-compose-project lease and the label-only teardown.
# Real rootless podman, real compose project, real longops registry (no mocks). Oracle strategy (11.4.245): SPECIFIED. The behaviour is declared by T131:
#   - the lease is keyed per compose project: up.sh refuses a second owner of the SAME project and admits a DIFFERENT project at the same time;
#   - down.sh removes only the resources labelled with this run's compose project: an unlabelled foreign container whose NAME starts with the project
#     name survives (carrier case), and the other project's resources survive;
#   - down.sh releases the lease (the project can be started again) and is idempotent.
# Paired mutations: a copy of the scripts with ONE load-bearing line changed; the body of this test is re-run against each copy and must FAIL.
# Usage:  test_up_down.sh                       tests, then mutations
#         UPDOWN_NO_MUTATIONS=1 test_up_down.sh  tests only
# Env:    TI_SUT_DIR  repo-relative directory of the scripts under test (default scripts/test-infra; RED is captured against an absent directory)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SD="${TI_SUT_DIR:-scripts/test-infra}"
UP="$TI_REPO/$SD/up.sh"; DOWN="$TI_REPO/$SD/down.sh"; PROBE="$TI_REPO/$SD/probe.sh"
export TI_ROOT="$TI_REPO"   # a mutation copy lives outside scripts/test-infra: its repository root is explicit
TI_DOWN="$DOWN"
need_script "$SD/up.sh" || { ti_summary; exit 1; }
need_script "$SD/down.sh" || { ti_summary; exit 1; }
LOCKIMG=$(python3 -I -c "import yaml;d=yaml.safe_load(open('$TI_REPO/build/containers/images.lock.yaml'));print([i for i in d['images'] if i['id']=='IMG-INFRA-REDIS'][0]['reference']+'@'+[i for i in d['images'] if i['id']=='IMG-INFRA-REDIS'][0]['digest'])")
count() { podman ps -a -q --filter "label=catalogizer.test_project=$1" | wc -l; }
claim() { [ -d "$TI_REPO/.audit/longops/claims/$1" ] && echo held || echo free; }

A=$(ti_new_id); B=$(ti_new_id); [ "$A" != "$B" ] || B="${B}b"
PA=$(ti_project "$A"); PB=$(ti_project "$B")
TI_IDS+=("$A" "$B")

out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?
check "up A exits 0" "$rc" 0
case "$out" in *"project=$PA"*) ok "up A prints its project";; *) bad "up A did not print project=$PA ($out)";; esac
check "up A: one container labelled with the project" "$(count "$PA")" 1
check "up A: the lease is held" "$(claim "$PA")" held
holder=$(jq -r .run_id "$TI_REPO/.audit/longops/claims/$PA/holder.json" 2>/dev/null)
case "$holder" in "$PA"-up-*) ok "up A: the holder record names the stack operation ($holder)";; *) bad "up A: the holder record names '$holder'";; esac
# a protocol-level answer from the stack proves it is up (PING -> PONG, from IMG-INFRA-CLIENT on the project network)
if bash "$PROBE" --build-id "$A" --services redis >"$TI_SCRATCH/pa.txt" 2>&1; then ok "A answers its redis probe"; else bad "A does not answer its redis probe: $(tail -2 "$TI_SCRATCH/pa.txt" | tr '\n' ' ')"; fi

out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?
check "a second owner of the SAME project is refused (exit 3)" "$rc" 3
case "$out" in *lease_held*) ok "the refusal names lease_held";; *) bad "the refusal does not name lease_held ($out)";; esac
check "the refused second up changed nothing (still one container)" "$(count "$PA")" 1

out=$(bash "$UP" --build-id "$B" --services redis 2>&1); rc=$?
check "a DIFFERENT project is admitted while A is up (exit 0)" "$rc" 0
[ "$rc" = 0 ] || echo "  up B said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"
check "up B: one container labelled with B" "$(count "$PB")" 1
check "both leases are held at once" "$(claim "$PA")$(claim "$PB")" heldheld
case "$(grep -h '^TI_PORT_REDIS' "$(ti_envfile "$A")" "$(ti_envfile "$B")" | sort -u | wc -l)" in 2) ok "A and B use distinct redis ports";; *) bad "A and B share a redis port";; esac

# foreign fixtures: (1) an UNLABELLED container whose name starts with A's project name (carrier case), (2) nothing of B is touched
fc=$(podman run -d --pull=never --name "$PA-foreign" --entrypoint sleep "$LOCKIMG" 600 2>/dev/null); TI_FOREIGN+=("$fc")
check "fixture: the foreign container is running" "$(podman inspect --format '{{.State.Running}}' "$fc" 2>/dev/null)" true

out=$(bash "$DOWN" --build-id "$A" 2>&1); rc=$?
check "down A exits 0" "$rc" 0
check "down A removed every container labelled with A" "$(count "$PA")" 0
check "down A removed A's network" "$(podman network exists "${PA}_test-network" 2>/dev/null && echo present || echo gone)" gone
check "the foreign unlabelled container (name carries A's project name) SURVIVES" "$(podman inspect --format '{{.State.Running}}' "$fc" 2>/dev/null)" true
check "the OTHER project's container survives" "$(count "$PB")" 1
if bash "$PROBE" --build-id "$B" --services redis >"$TI_SCRATCH/pb.txt" 2>&1; then ok "B still answers its redis probe after A went down"; else bad "B broke when A went down: $(tail -2 "$TI_SCRATCH/pb.txt" | tr '\n' ' ')"; fi
check "down A released A's lease" "$(claim "$PA")" free
check "down A left B's lease alone" "$(claim "$PB")" held
[ ! -e "$(ti_envfile "$A")" ] && ok "down A removed A's credentials file" || bad "A's credentials file survived"
out=$(bash "$DOWN" --build-id "$A" 2>&1); check "down A again is a no-op (idempotent, exit 0)" "$?" 0
bash "$DOWN" --build-id 'Bad_Id!' >/dev/null 2>&1; check "down with an invalid build id exits 2" "$?" 2

out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?
check "A can be started again after its lease was released (exit 0)" "$rc" 0
bash "$DOWN" --build-id "$A" >/dev/null 2>&1
bash "$DOWN" --build-id "$B" >/dev/null 2>&1
check "down B removed B's containers" "$(count "$PB")" 0
check "all leases released at the end" "$(claim "$PA")$(claim "$PB")" freefree
check "the foreign container still survives both teardowns" "$(podman inspect --format '{{.State.Running}}' "$fc" 2>/dev/null)" true

# ---------------- paired mutations ----------------
if [ "${UPDOWN_NO_MUTATIONS:-0}" != 1 ] && [ "${UPDOWN_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="${UPDOWN_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  mutate() { # mutate <name> <file> <old> <new> [<old2> <new2>]
    local name=$1 file=$2 d="$TI_REPO/.audit/scratch/ti-mut-$1"; shift 2
    rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/$SD"/*.sh "$d/"; cp -r "$TI_REPO/$SD/client" "$d/client"
    python3 -I - "$d/$file" "$@" <<'PY' || return 1
import sys
p = sys.argv[1]; s = open(p).read(); a = sys.argv[2:]
for i in range(0, len(a), 2):
    if s.count(a[i]) != 1: print("mutation anchor count %d for %r" % (s.count(a[i]), a[i])); sys.exit(1)
    s = s.replace(a[i], a[i + 1])
open(p, "w").write(s)
PY
  }
  run_mut() {
    local name=$1 out rc d=".audit/scratch/ti-mut-$1"
    : >"$TI_SCRATCH/ids-$1.log"
    out=$(TI_IDS_LOG="$TI_SCRATCH/ids-$1.log" TI_SUT_DIR="$d" UPDOWN_TEST_MUTANT=1 UPDOWN_NO_MUTATIONS=1 QUIET=1 bash "${BASH_SOURCE[0]}" 2>&1); rc=$?
    # whatever the mutant left behind is removed by the REAL down.sh (a mutant down.sh may not clean its own project)
    local i; for i in $(cat "$TI_SCRATCH/ids-$1.log"); do bash "$TI_REPO/scripts/test-infra/down.sh" --build-id "$i" >/dev/null 2>&1; done
    if [ "$rc" -ne 0 ]; then ok "mutation $name CAUGHT ($(printf '%s\n' "$out" | grep -m1 '^FAIL' | cut -c1-100))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${TI_REPO:?}/${d:?}"
  }
  mutate down_by_name down.sh 'for c in $(podman ps -a -q --filter "label=catalogizer.test_project=$P" --filter "label=project=catalogizer" 2>/dev/null); do' 'for c in $(podman ps -a -q --filter "name=$P" 2>/dev/null); do' \
         '  [ "$lab" = "$P" ] || { echo "test-infra: skipping $c: label '"'"'$lab'"'"' is not $P" >&2; continue; }' '  true' && run_mut down_by_name || bad "mutation down_by_name: anchor missing"
  mutate down_all_catalogizer down.sh '--filter "label=catalogizer.test_project=$P" --filter "label=project=catalogizer" 2>/dev/null); do' '--filter "label=project=catalogizer" 2>/dev/null); do' \
         '  [ "$lab" = "$P" ] || { echo "test-infra: skipping $c: label '"'"'$lab'"'"' is not $P" >&2; continue; }' '  true' && run_mut down_all_catalogizer || bad "mutation down_all_catalogizer: anchor missing"
  mutate no_lease up.sh 'ti_lo register --purpose "$P" --owner test-infra-up --op-id "$OPID" --pid "$KPID" --container-label "$P" >/dev/null 2>"$S/lease.err.$OPID"; RC=$?' 'RC=0' && run_mut no_lease || bad "mutation no_lease: anchor missing"
  mutate no_release down.sh 'ti_lo release --op-id "$OPID" --state complete --verdict down >/dev/null 2>&1 || true' 'true' && run_mut no_release || bad "mutation no_release: anchor missing"
  [ -z "${UPDOWN_EV:-}" ] || cp "$MUTLOG" "$UPDOWN_EV/updown-mutations.txt"
fi
ti_summary
