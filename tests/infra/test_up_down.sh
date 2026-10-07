#!/usr/bin/env bash
# test_up_down.sh - T131 (RED first), extended by the WF12 review fixes. Oracle for scripts/test-infra/up.sh and down.sh: the per-compose-project lease and the label-only teardown.
# Real rootless podman, real compose project, real longops registry (no mocks). Oracle strategy (11.4.245): SPECIFIED. The behaviour is declared by T131 and the review:
#   - the lease is keyed per compose project: up.sh refuses a second owner of the SAME project and admits a DIFFERENT project at the same time; the lease is a LIVE keeper process of
#     the owning start (identity AND liveness, WF12 F5), never "the claim directory exists"; a refused second owner leaves the live holder untouched;
#   - down.sh removes only the resources labelled with this run's compose project: an unlabelled foreign container whose NAME starts with the project name survives (carrier case),
#     and the other project's resources survive; the unlabelled podman-compose pod of THIS project and its `-client`/`-seed` output directories are removed, a foreign pod and foreign
#     or adjacent output directories are not (WF12 F2); no pod is created by a start at all;
#   - down.sh is OWNER-CHECKED (WF12 F6): a live holder is torn down only by the caller that names its operation, any other caller is REFUSED (exit 5, reason not_lease_owner) and nothing
#     is touched; a holder PROVEN dead (its keeper gone) is reaped by any caller and its operation becomes `reaped`;
#   - down.sh releases the lease (the project can be started again) and is idempotent;
#   - a cached corpus whose files no longer match its recorded digest is refused (WF12 F15); a published port taken between the choice and the bind is retried with fresh ports (WF12 F16).
# Paired mutations: a copy of the scripts with ONE load-bearing line changed; the same test is re-run against each copy and must FAIL (the reviewer mutant RM4 is adopted verbatim).
# Usage:  test_up_down.sh                       tests, then mutations
#         UPDOWN_NO_MUTATIONS=1 test_up_down.sh  tests only
# Env:    TI_SUT_DIR  repo-relative directory of the scripts under test (default scripts/test-infra; RED is captured against an export of git HEAD)
#         UPDOWN_ONLY  core | cache | retry : run one section only (the mutation runs use it)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SD="${TI_SUT_DIR:-scripts/test-infra}"
UP="$TI_REPO/$SD/up.sh"; DOWN="$TI_REPO/$SD/down.sh"; PROBE="$TI_REPO/$SD/probe.sh"
export TI_ROOT="$TI_REPO"   # a mutation copy lives outside scripts/test-infra: its repository root is explicit
TI_DOWN="$DOWN"
ONLY="${UPDOWN_ONLY:-all}"; want() { [ "$ONLY" = all ] || [ "$ONLY" = "$1" ]; }
need_script "$SD/up.sh" || { ti_summary; exit 1; }
need_script "$SD/down.sh" || { ti_summary; exit 1; }
LOCKIMG=$(python3 -I -c "import yaml;d=yaml.safe_load(open('$TI_REPO/build/containers/images.lock.yaml'));print([i for i in d['images'] if i['id']=='IMG-INFRA-REDIS'][0]['reference']+'@'+[i for i in d['images'] if i['id']=='IMG-INFRA-REDIS'][0]['digest'])")
count() { podman ps -a -q --filter "label=catalogizer.test_project=$1" | wc -l; }
claim() { [ -d "$TI_REPO/.audit/longops/claims/$1" ] && echo held || echo free; }   # only for the ABSENCE of a claim: presence is never read as a lease (ti_lease_state)
podp() { podman pod exists "$1" 2>/dev/null && echo present || echo gone; }
opid_of() { printf '%s\n' "$1" | sed -n 's/^op_id=//p'; }
OUTD="$TI_REPO/.audit/out"
# a tag that only THIS test's foreign fixtures carry: the over-broad-glob mutants below delete by a glob that matches the tag and nothing of another stream on this shared host
TAG="${UPDOWN_TAG:-zz$RANDOM}"; export UPDOWN_TAG="$TAG"

if want core; then
A=$(ti_new_id); B=$(ti_new_id); [ "$A" != "$B" ] || B="${B}b"
PA=$(ti_project "$A"); PB=$(ti_project "$B")
TI_IDS+=("$A" "$B")

out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?
check "up A exits 0" "$rc" 0
OPA=$(opid_of "$out")
case "$out" in *"project=$PA"*) ok "up A prints its project";; *) bad "up A did not print project=$PA ($out)";; esac
case "$OPA" in "$PA"-up-*) ok "up A prints the operation it registered ($OPA)";; *) bad "up A printed no op_id of its own start ('$OPA')";; esac
check "up A: one container labelled with the project" "$(count "$PA")" 1
check "up A: the lease is held by a LIVE keeper of this very start (run id, pid > 1, /proc entry, keeper command line)" "$(ti_lease_state "$A" "$OPA")" ok
check "up A: no podman pod was created for the project (WF12 F2: label-scoped teardown cannot see a pod)" "$(podp "pod_$PA")" gone
# a protocol-level answer from the stack proves it is up (PING -> PONG, from IMG-INFRA-CLIENT on the project network)
if bash "$PROBE" --build-id "$A" --services redis >"$TI_SCRATCH/pa.txt" 2>&1; then ok "A answers its redis probe"; else bad "A does not answer its redis probe: $(tail -2 "$TI_SCRATCH/pa.txt" | tr '\n' ' ')"; fi

out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?
check "a second owner of the SAME project is refused (exit 3)" "$rc" 3
case "$out" in *reason=lease_held*) ok "the refusal names reason=lease_held";; *) bad "the refusal does not name reason=lease_held ($out)";; esac
check "the refused second up changed nothing (still one container)" "$(count "$PA")" 1
check "the refused second owner left the LIVE holder alive and the lease intact (WF12 F5, reviewer mutant RM4)" "$(ti_lease_state "$A" "$OPA")" ok
out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?
check "a THIRD up is still refused as lease_held (exit 3), not lease_stale (exit 4)" "$rc" 3
case "$out" in *reason=lease_held*) ok "the third refusal names reason=lease_held";; *) bad "the third refusal is not lease_held ($out)";; esac

out=$(bash "$UP" --build-id "$B" --services redis 2>&1); rc=$?
check "a DIFFERENT project is admitted while A is up (exit 0)" "$rc" 0
[ "$rc" = 0 ] || echo "  up B said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"
OPB=$(opid_of "$out")
check "up B: one container labelled with B" "$(count "$PB")" 1
check "both leases are live keepers of their own start, each its own" "$(ti_lease_state "$A" "$OPA")$(ti_lease_state "$B" "$OPB")" okok
check "A's holder is not B's (distinct operations)" "$([ "$OPA" != "$OPB" ] && echo distinct || echo same)" distinct
case "$(grep -h '^TI_PORT_REDIS' "$(ti_envfile "$A")" "$(ti_envfile "$B")" | sort -u | wc -l)" in 2) ok "A and B use distinct redis ports";; *) bad "A and B share a redis port";; esac

# ownership of the teardown (WF12 F6): a caller that does not hold A's lease cannot destroy A
out=$(bash "$DOWN" --build-id "$A" 2>&1); rc=$?
check "down A by a caller naming no operation is REFUSED (exit 5)" "$rc" 5
case "$out" in *reason=not_lease_owner*) ok "the refusal names reason=not_lease_owner";; *) bad "the refusal does not name not_lease_owner ($out)";; esac
check "the refused down touched nothing: A's container runs and its lease is intact" "$(count "$PA")$(ti_lease_state "$A" "$OPA")" 1ok
out=$(bash "$DOWN" --build-id "$A" --op-id "$OPB" 2>&1); rc=$?
check "down A by the OWNER OF B (a live holder of another project) is REFUSED (exit 5)" "$rc" 5
check "the second refused down touched nothing either" "$(count "$PA")$(ti_lease_state "$A" "$OPA")" 1ok

# foreign fixtures: (1) an UNLABELLED container whose name starts with A's project name (carrier case); (2) the unlabelled empty pod podman-compose used to leave for A, a pod of B and a
# pod of a foreign project; (3) output directories: A's client/seed (to be removed), B's client, a foreign project's, A's kept logs and an adjacent name (to survive)
fc=$(podman run -d --pull=never --name "$PA-foreign" --entrypoint sleep "$LOCKIMG" 600 2>/dev/null); TI_FOREIGN+=("$fc")
check "fixture: the foreign container is running" "$(podman inspect --format '{{.State.Running}}' "$fc" 2>/dev/null)" true
FPOD="pod_catalogizer-test-${TAG}foreign"
podman pod create --name "pod_$PA" --infra=false --share= >/dev/null 2>&1; TI_FOREIGN_PODS+=("pod_$PA")
podman pod create --name "pod_$PB" --infra=false --share= >/dev/null 2>&1; TI_FOREIGN_PODS+=("pod_$PB")
podman pod create --name "$FPOD" --infra=false --share= >/dev/null 2>&1; TI_FOREIGN_PODS+=("$FPOD")
check "fixture: A's leaked-style pod, B's pod and the foreign pod exist" "$(podp "pod_$PA")$(podp "pod_$PB")$(podp "$FPOD")" presentpresentpresent
FD="$OUTD/catalogizer-test-${TAG}f-client"
mkdir -p "$OUTD/$PA-client" "$OUTD/$PA-seed" "$OUTD/$PB-client" "$FD" "$OUTD/$PA-logs" "$OUTD/$PA-client2"
for d in "$OUTD/$PA-client" "$OUTD/$PA-seed" "$OUTD/$PB-client" "$FD" "$OUTD/$PA-logs" "$OUTD/$PA-client2"; do echo x >"$d/f"; TI_FOREIGN_DIRS+=("$d"); done

out=$(bash "$DOWN" --build-id "$A" --op-id "$OPA" 2>&1); rc=$?
check "down A by its owner exits 0" "$rc" 0
check "down A removed every container labelled with A" "$(count "$PA")" 0
check "down A removed A's network" "$(podman network exists "${PA}_test-network" 2>/dev/null && echo present || echo gone)" gone
check "the foreign unlabelled container (name carries A's project name) SURVIVES" "$(podman inspect --format '{{.State.Running}}' "$fc" 2>/dev/null)" true
check "the OTHER project's container survives" "$(count "$PB")" 1
if bash "$PROBE" --build-id "$B" --services redis >"$TI_SCRATCH/pb.txt" 2>&1; then ok "B still answers its redis probe after A went down"; else bad "B broke when A went down: $(tail -2 "$TI_SCRATCH/pb.txt" | tr '\n' ' ')"; fi
check "down A released A's lease" "$(claim "$PA")" free
check "down A left B's lease alone (B's live keeper)" "$(ti_lease_state "$B" "$OPB")" ok
check "down A removed A's empty podman-compose pod" "$(podp "pod_$PA")" gone
check "down A left B's pod and the foreign pod alone" "$(podp "pod_$PB")$(podp "$FPOD")" presentpresent
check "down A removed A's -client and -seed output directories" "$([ -e "$OUTD/$PA-client" ] && echo kept || echo gone)$([ -e "$OUTD/$PA-seed" ] && echo kept || echo gone)" gonegone
check "down A left B's, the foreign project's, A's -logs and the adjacent -client2 directory alone" "$([ -e "$OUTD/$PB-client" ] && echo kept)$([ -e "$FD" ] && echo kept)$([ -e "$OUTD/$PA-logs" ] && echo kept)$([ -e "$OUTD/$PA-client2" ] && echo kept)" keptkeptkeptkept
[ ! -e "$(ti_envfile "$A")" ] && ok "down A removed A's credentials file" || bad "A's credentials file survived"
check "the operation of A's start is terminal (complete)" "$(jq -r .state "$TI_REPO/.audit/longops/ops/$OPA.json" 2>/dev/null)" complete
out=$(bash "$DOWN" --build-id "$A" 2>&1); check "down A again is a no-op (idempotent, exit 0)" "$?" 0
bash "$DOWN" --build-id 'Bad_Id!' >/dev/null 2>&1; check "down with an invalid build id exits 2" "$?" 2

out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?
check "A can be started again after its lease was released (exit 0)" "$rc" 0
OPA2=$(opid_of "$out")
check "the restarted A holds a fresh lease of its own operation" "$(ti_lease_state "$A" "$OPA2")" ok
# a holder that dies is PROVEN stale: the keeper leaves when its file goes (no signal is sent), then ANY caller may reap it and its operation becomes `reaped`
rm -f "$(ti_state "$A")/lease.keep.$OPA2"
for _ in $(seq 1 30); do [ "$(ti_lease_state "$A" "$OPA2")" = dead ] && break; sleep 1; done
check "the holder of the restarted A is dead (its keeper file was removed)" "$(ti_lease_state "$A" "$OPA2")" dead
out=$(bash "$DOWN" --build-id "$A" 2>&1); rc=$?
check "down A by a caller naming no operation succeeds when the holder is PROVEN dead (exit 0)" "$rc" 0
check "the dead holder's lease is released" "$(claim "$PA")" free
check "the dead holder's operation is recorded reaped" "$(jq -r .state "$TI_REPO/.audit/longops/ops/$OPA2.json" 2>/dev/null)" reaped
check "the stale project's containers are gone" "$(count "$PA")" 0
# a start that fails AFTER it took the lease releases ITS OWN lease (WF12 F17): the old state of an earlier start (kept with --keep-state) must not be mistaken for the new owner's
out=$(bash "$UP" --build-id "$A" --services redis 2>&1); OPA3=$(opid_of "$out")
bash "$DOWN" --build-id "$A" --op-id "$OPA3" --keep-state >/dev/null 2>&1
check "fixture: the kept state still names the OLD operation" "$(ti_val "$A" TI_OP_ID)" "$OPA3"
fx=$(podman run -d --pull=never --name "$PA-prior" --label project=catalogizer --label "catalogizer.test_project=$PA" --entrypoint sleep "$LOCKIMG" 600 2>/dev/null); TI_FOREIGN+=("$fx")
out=$(bash "$UP" --build-id "$A" --services redis 2>&1); rc=$?
check "a start over pre-existing containers of the project fails (exit 1)" "$rc" 1
case "$out" in *"already exist"*) ok "the failure names the pre-existing containers";; *) bad "unexpected failure text: $(printf '%s' "$out" | tail -2 | tr '\n' ' ' | cut -c1-200)";; esac
check "the failed start released ITS OWN lease (no claim left)" "$(claim "$PA")" free
nonterm=$(for f in "$TI_REPO"/.audit/longops/ops/"$PA"-up-*.json; do [ -e "$f" ] || continue; jq -r 'select(.state|IN("complete","failed","reaped","handoff","blocked-escape")|not)|.op_id' "$f"; done | grep -c .)
check "no operation of the project is left non-terminal" "$nonterm" 0
ti_down "$B" >/dev/null 2>&1
check "down B removed B's containers" "$(count "$PB")" 0
check "all leases released at the end" "$(claim "$PA")$(claim "$PB")" freefree
check "the foreign container still survives all teardowns" "$(podman inspect --format '{{.State.Running}}' "$fc" 2>/dev/null)" true
fi

# ---------------- corpus cache verification (WF12 F15) ----------------
if want cache; then
CK=$(ti_new_id); PK=$(ti_project "$CK"); TI_IDS+=("$CK")
real=$(ls -d "$TI_REPO"/.audit/out/test-infra-corpus-* 2>/dev/null | head -1)
if [ -z "$real" ]; then bad "no seeded corpus cache to copy (run one up.sh first)"; else
  cp -a "$real" "$TI_SCRATCH/cache-bad"; f=$(find "$TI_SCRATCH/cache-bad/corpus" -type f | LC_ALL=C sort | head -1)
  printf 'X' | dd of="$f" bs=1 count=1 conv=notrunc 2>/dev/null
  out=$(TI_CORPUS_CACHE_DIR="$TI_SCRATCH/cache-bad" bash "$UP" --build-id "$CK" --services redis 2>&1); rc=$?
  check "a corrupted corpus cache is refused (exit 1)" "$rc" 1
  case "$out" in *"does not match its recorded digest"*) ok "the refusal says the cache does not match its recorded digest";; *) bad "the refusal does not name the digest mismatch ($(printf '%s' "$out" | tail -2 | tr '\n' ' ' | cut -c1-200))";; esac
  check "the refused start left no container, no lease" "$(count "$PK")$(claim "$PK")" 0free
  cp -a "$real" "$TI_SCRATCH/cache-good"
  out=$(TI_CORPUS_CACHE_DIR="$TI_SCRATCH/cache-good" bash "$UP" --build-id "$CK" --services redis 2>&1); rc=$?
  check "the pristine copy of the cache is accepted (exit 0)" "$rc" 0
  check "the printed corpus digest is the one RECOMPUTED from the files (equals the independent recorded digest)" "$(printf '%s\n' "$out" | sed -n 's/^corpus_sha256=//p')" "$(cut -d' ' -f1 "$real/corpus.sha256")"
  ti_down "$CK" >/dev/null 2>&1
fi
fi

# ---------------- bind-race retry (WF12 F16) ----------------
if want retry; then
RID=$(ti_new_id); PR=$(ti_project "$RID"); TI_IDS+=("$RID")
LK="$TI_SCRATCH/listener.alive"; : >"$LK"
PORT=$(python3 -I - "$LK" <<'PY'
import os, socket, subprocess, sys
s = socket.socket(); s.bind(("127.0.0.1", 0)); s.listen(1); port = s.getsockname()[1]; s.close()
# the listener runs until its marker file goes (no signal is ever sent to it)
subprocess.Popen([sys.executable, "-I", "-c", "import os,socket,sys,time\ns=socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 0); s.bind(('127.0.0.1', %d)); s.listen(1)\nwhile os.path.exists(sys.argv[1]): time.sleep(0.3)" % port, sys.argv[1]], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
print(port)
PY
)
sleep 1
check "fixture: the listener holds the port" "$(python3 -I -c "import socket;s=socket.socket();print('free' if s.connect_ex(('127.0.0.1',$PORT)) else 'taken')")" taken
d="$TI_REPO/.audit/scratch/ti-f16-$$"; rm -rf -- "${d:?}"; mkdir -p "$d"; TI_FOREIGN_DIRS+=("$d"); cp "$TI_REPO/$SD"/*.sh "$d/"; cp -r "$TI_REPO/$SD/client" "$d/client"
python3 -I - "$d/gen_env.sh" "$PORT" <<'PY'
import sys
p, port = sys.argv[1:3]; s = open(p).read(); a = "set -- $PORTS\n"
assert s.count(a) == 1
# the FIRST draw of this build id lands on the occupied port (redis = 2nd): the test's stand-in for another process taking a drawn port; later draws are the real random ones
s = s.replace(a, a + 'if [ ! -e "$S/.f16-drawn" ]; then mkdir -p "$S"; : >"$S/.f16-drawn"; set -- "$1" %s "$3" "$4" "$5" "$6"; fi\n' % port)
open(p, "w").write(s)
PY
TI_DOWN="$d/down.sh"
out=$(bash "$d/up.sh" --build-id "$RID" --services redis 2>&1); rc=$?
check "a port taken between the draw and the bind is retried and the start succeeds (exit 0)" "$rc" 0
case "$out" in *"a published port was taken before the bind"*) ok "the retry is announced, never silent";; *) bad "no retry was announced ($(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-200))";; esac
check "the final redis port is not the occupied one" "$([ "$(ti_val "$RID" TI_PORT_REDIS)" != "$PORT" ] && echo fresh || echo same)" fresh
check "the start left exactly one container (the failed attempt was removed)" "$(count "$PR")" 1
bash "$d/down.sh" --build-id "$RID" --op-id "$(ti_val "$RID" TI_OP_ID)" >/dev/null 2>&1
rm -f "$LK"; sleep 1; rm -rf -- "${d:?}"
fi

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
  run_mut() { # run_mut <name> <section>
    local name=$1 sec=${2:-core} out rc d=".audit/scratch/ti-mut-$1"
    : >"$TI_SCRATCH/ids-$1.log"
    out=$(TI_IDS_LOG="$TI_SCRATCH/ids-$1.log" TI_SUT_DIR="$d" UPDOWN_TEST_MUTANT=1 UPDOWN_NO_MUTATIONS=1 UPDOWN_ONLY="$sec" TI_FAILFAST=1 QUIET=1 bash "${BASH_SOURCE[0]}" 2>&1); rc=$?
    # whatever the mutant left behind is removed by the REAL down.sh, as the owner of each start (a mutant down.sh may not clean its own project)
    local i; for i in $(cat "$TI_SCRATCH/ids-$1.log"); do TI_DOWN="$TI_REPO/scripts/test-infra/down.sh" ti_down "$i" >/dev/null 2>&1; done
    if [ "$rc" -ne 0 ]; then ok "mutation $name CAUGHT ($(printf '%s\n' "$out" | grep -m1 '^FAIL' | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${TI_REPO:?}/${d:?}"
  }
  M() { local name=$1 sec=$2 file=$3; shift 3; mutate "$name" "$file" "$@" && run_mut "$name" "$sec" || bad "mutation $name: anchor missing"; }
  M down_by_name core lib.sh 'for c in $(podman ps -a -q --filter "label=catalogizer.test_project=$P" --filter "label=project=catalogizer" 2>/dev/null); do' 'for c in $(podman ps -a -q --filter "name=$P" 2>/dev/null); do' \
         '    [ "$lab" = "$P" ] || { echo "test-infra: skipping $c: label '"'"'$lab'"'"' is not $P" >&2; continue; }' '    true'
  M down_all_catalogizer core lib.sh '--filter "label=catalogizer.test_project=$P" --filter "label=project=catalogizer" 2>/dev/null); do' '--filter "label=project=catalogizer" 2>/dev/null); do' \
         '    [ "$lab" = "$P" ] || { echo "test-infra: skipping $c: label '"'"'$lab'"'"' is not $P" >&2; continue; }' '    true'
  M no_lease core up.sh 'ti_lo register --purpose "$P" --owner test-infra-up --op-id "$OPID" --pid "$KPID" --container-label "$P" >/dev/null 2>"$S/lease.err.$OPID"; RC=$?' 'RC=0'
  M no_release core down.sh 'elif [ -n "$HOLDER" ]; then ti_lo release --op-id "$HOLDER" --state complete --verdict down >/dev/null 2>&1 || ti_lo release --purpose "$P" --run-id "$HOLDER" >/dev/null 2>&1 || true' 'elif [ -n "$HOLDER" ]; then true'
  # RM4 of the WF12 review, verbatim: the keeper file is no longer per attempt, so a refused second owner deletes the LIVE holder's keeper file
  M rm4_shared_keeper_file core up.sh 'KEEP="$S/lease.keep.$OPID"' 'KEEP="$S/lease.keep"'
  M down_no_owner_check core down.sh 'if [ -d "$LD/claims/$P" ]; then
  HOLDER=' 'if false; then
  HOLDER='
  M down_reaps_live_holder core down.sh '  elif DRY="$(ti_lo reap --purpose "$P" --dry-run 2>&1)" && printf '"'"'%s'"'"' "$DRY" | grep -q '"'"'would release stale claim'"'"'; then STALE=1' '  elif true; then STALE=1'
  M down_no_pod_removal core lib.sh '  if podman pod exists "$pod" 2>/dev/null; then' '  if false; then'
  # the two over-broad-glob mutants (pods by prefix, directories by pattern) are restricted to this test's own tag, so a mutant run never deletes anything of another stream
  M down_pod_by_glob core lib.sh '  pod="$(ti_pod "$P")"' '  for pp in $(podman pod ls --format '"'"'{{.Name}}'"'"' | grep "^pod_catalogizer-test-.*'"$TAG"'"); do podman pod rm "$pp" >/dev/null 2>&1; done; pod="$(ti_pod "$P")"'
  M down_out_dirs_glob core lib.sh '  for d in "$TI_ROOT/.audit/out/$P-client" "$TI_ROOT/.audit/out/$P-seed"; do' '  for d in "$TI_ROOT"/.audit/out/catalogizer-test-*"'"$TAG"'"*-client "$TI_ROOT/.audit/out/$P-seed"; do'
  M down_keeps_out_dirs core down.sh 'ti_rm_out_dirs "$P" || rc=1' 'true'
  M in_pod_back core up.sh 'podman-compose --in-pod false -p "$P"' 'podman-compose -p "$P"'
  M fail_down_without_op core up.sh 'bash "$HERE/down.sh" --build-id "$BID" --op-id "$OPID" --keep-logs' 'bash "$HERE/down.sh" --build-id "$BID" --keep-logs'
  M cache_trusted cache up.sh '[ "$CDIGEST" = "$(cut -d'"'"' '"'"' -f1 "$SEEDOUT/corpus.sha256")" ] || fail_down' 'true || fail_down'
  M no_port_retry retry up.sh '&& [ "$attempt" -lt 3 ]; then' '&& false; then'
  [ -z "${UPDOWN_EV:-}" ] || cp "$MUTLOG" "$UPDOWN_EV/updown-mutations.txt"
fi
ti_summary
