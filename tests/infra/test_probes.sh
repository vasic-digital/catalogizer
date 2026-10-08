#!/usr/bin/env bash
# test_probes.sh - T128 (RED first). Oracle for scripts/test-infra/probe.sh and run_client.sh: protocol-level probes of a REAL stack.
# SPECIFIED answers (PostgreSQL SELECT 1 -> 1, Redis PING -> PONG, FTP passive login/list/transfer with the corpus manifest sha256, SMB share list,
# WebDAV PROPFIND -> 207), a negative control per protocol (a wrong credential makes the probe FAIL: a probe that cannot fail proves nothing), the
# refusal fixtures of T128 (a probe composed to run on the host, a probe in a service-class image) and the carrier fixture (a listener that accepts TCP
# and speaks nothing: an open port must not pass). MinIO is BLOCKED (image not obtainable) and is reported as BLOCKED, never as a pass.
# Paired mutations: copies of the scripts/probes with ONE load-bearing change; the fixture checks are re-run against each copy and must FAIL.
# Usage:  test_probes.sh                    tests, then mutations
#         PROBES_NO_MUTATIONS=1 ...         tests only
# Env:    TI_SUT_DIR  repo-relative directory of the scripts under test (default scripts/test-infra; RED is captured against an absent directory)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"
TI_DOWN="$TI_REPO/$SD/down.sh"
for f in probe.sh run_client.sh up.sh down.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
UP="$TI_REPO/$SD/up.sh"; DOWN="$TI_REPO/$SD/down.sh"
TESTUTIL=$(python3 -I -c "import yaml;d=yaml.safe_load(open('$TI_REPO/build/containers/images.lock.yaml'));e=[i for i in d['images'] if i['id']=='IMG-TESTUTIL'][0];print(e['reference']+'@'+e['digest'])")

ID=$(ti_new_id); TI_IDS+=("$ID"); P=$(ti_project "$ID")
out=$(bash "$UP" --build-id "$ID" 2>&1); rc=$?; check "up exits 0 (postgres, redis, ftp, smb, webdav)" "$rc" 0
[ "$rc" = 0 ] || { echo "  up said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"; ti_summary; exit 1; }

# fixtures that must hold for ANY probe.sh / run_client.sh (also run against every mutation copy)
fixtures() { # fixtures <repo-relative sut dir> <client dir or ''>; prints one FAIL line per violated fixture
  local d=$1 cd_=${2:-} n=0 o rc cnt
  local pr="$TI_REPO/$d/probe.sh" rcl="$TI_REPO/$d/run_client.sh"
  # host runner refused
  o=$(TI_PROBE_RUNNER=host bash "$pr" --build-id "$ID" --services redis 2>&1); rc=$?
  if [ "$rc" -ne 0 ] && printf '%s' "$o" | grep -q probe_runner_not_container; then :; else echo "FAIL host-composed probe was not refused (rc=$rc)"; n=$((n+1)); fi
  # service-class image refused, and no container is started by the refused call
  cnt=$(podman ps -a -q --filter "label=catalogizer.test_project=$P" | wc -l)   # WF12 F10: measured and asserted (the project's own containers: other streams' containers do not move it)
  o=$(bash "$rcl" --build-id "$ID" --image IMG-INFRA-POSTGRES -- echo hi 2>&1); rc=$?
  if [ "$rc" -ne 0 ] && printf '%s' "$o" | grep -q 'reason=client_image_not_interpreter_class' && ! printf '%s' "$o" | grep -qx hi; then :; else echo "FAIL service-class image was not refused as client_image_not_interpreter_class (rc=$rc)"; n=$((n+1)); fi
  [ "$(podman ps -a -q --filter "label=catalogizer.test_project=$P" | wc -l)" = "$cnt" ] || { echo "FAIL the refused call left a container of the project ($cnt before)"; n=$((n+1)); }
  o=$(bash "$rcl" --build-id "$ID" --image IMG-GO -- echo hi 2>&1); rc=$?
  if [ "$rc" -ne 0 ] && printf '%s' "$o" | grep -q 'reason=client_image_not_interpreter_class\|reason=not_the_infra_client' && ! printf '%s' "$o" | grep -qx hi; then :; else echo "FAIL a non-client image was not refused by name (rc=$rc)"; n=$((n+1)); fi
  # carrier: a listener that accepts TCP and speaks nothing; the postgres probe pointed at it must FAIL
  o=$(TI_PROBE_CLIENT_DIR="${cd_:-scripts/test-infra/client}" TI_PROBE_HOST_POSTGRES=ti-carrier bash "$pr" --build-id "$ID" --services postgres 2>&1); rc=$?
  if [ "$rc" -eq 1 ] && printf '%s' "$o" | grep -q 'PROBE postgres FAIL'; then :; else echo "FAIL the TCP-only carrier passed the postgres probe (rc=$rc: $(printf '%s' "$o" | tail -1 | cut -c1-120))"; n=$((n+1)); fi
  # negative control: a wrong credential makes each authenticated probe FAIL
  local cdir="${cd_:-scripts/test-infra/client}" pv
  # WF12 F8: the FAIL must carry the protocol's own authentication-refusal signal (an unreachable host or a crashed server also "fails" and must not pass as a refusal)
  for pv in "postgres TI_POSTGRES_PASSWORD password.authentication.failed" "redis TI_REDIS_PASSWORD WRONGPASS" "ftp TI_FTP_PASSWORD Login.failed:.530.Login" "smb TI_SMB_PASSWORD NT_STATUS_LOGON_FAILURE" "webdav TI_WEBDAV_PASSWORD HTTP.401"; do
    set -- $pv
    o=$(bash "$rcl" --build-id "$ID" -- env "$2=wrong-credential-1" bash "/src/$cdir/probe_$1.sh" 2>&1); rc=$?
    if [ "$rc" -ne 0 ] && printf '%s' "$o" | grep '^FAIL' | grep -qE "$3"; then :; else echo "FAIL $1 probe did not fail with its refusal signal '$3' on a wrong credential (rc=$rc: $(printf '%s' "$o" | grep '^FAIL' | head -1 | cut -c1-120))"; n=$((n+1)); fi
  done
  # and a probe whose host is unreachable fails for ANOTHER reason: it must not carry the refusal signal (the fixture can tell the two apart)
  o=$(env TI_PROBE_HOST_POSTGRES=ti-no-such-host bash "$rcl" --build-id "$ID" -- env TI_PROBE_HOST_POSTGRES=ti-no-such-host bash "/src/$cdir/probe_postgres.sh" 2>&1)
  if printf '%s' "$o" | grep '^FAIL' | grep -qE 'password.authentication.failed'; then echo "FAIL control: an unreachable host produced the authentication-refusal signal (the fixture cannot tell them apart)"; n=$((n+1)); fi
  return "$n"
}
# the carrier: a container on the project network that accepts TCP connections on 5432 and says nothing; labelled so down.sh removes it
CAR=$(podman run -d --pull=never --name "$P-carrier" --network "$(ti_network "$ID")" --network-alias ti-carrier --label project=catalogizer --label "catalogizer.test_project=$P" --label "catalogizer.test_root=$(ti_test_root)" --label "op_id=$(ti_val "$ID" TI_OP_ID)" --label "catalogizer.op_id=$(ti_val "$ID" TI_OP_ID)" --entrypoint python3 "$TESTUTIL" -c 'import socket,time
s=socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1); s.bind(("0.0.0.0",5432)); s.listen(8)
c=[]
while True:
    c.append(s.accept()[0])' 2>&1) && ok "carrier fixture started (accepts TCP on 5432, speaks nothing)" || bad "carrier fixture did not start: $CAR"
sleep 2
o=$(bash "$TI_REPO/$SD/run_client.sh" --build-id "$ID" -- bash -c 'exec 3<>/dev/tcp/ti-carrier/5432 && echo tcp_open' 2>&1); case "$o" in *tcp_open*) ok "control: a TCP-only check WOULD pass on the carrier (the open-port trap is real)";; *) bad "control: the carrier does not accept TCP ($o)";; esac

PO=$(bash "$TI_REPO/$SD/probe.sh" --build-id "$ID" 2>&1); rc=$?
check "probe.sh (all) exits 0" "$rc" 0
for pair in "postgres:SELECT 1 answered 1" "redis:PING answered PONG" "ftp:passive transfer (sha256 equals the corpus manifest)" "smb:testshare" "webdav:PROPFIND answered 207"; do
  proto=${pair%%:*}; want=${pair#*:}
  if printf '%s\n' "$PO" | grep -q "^PROBE $proto PASS .*$want"; then ok "probe $proto PASS with its protocol answer ($want)"; else bad "probe $proto: no PASS line with '$want' (got: $(printf '%s\n' "$PO" | grep "PROBE $proto" | cut -c1-140))"; fi
done
if printf '%s\n' "$PO" | grep -q '^PROBE minio BLOCKED reason=image_unavailable'; then blocked "minio probe BLOCKED (image not obtainable, evidence/wp12/minio-blocked.txt)"; else bad "minio is not reported BLOCKED"; fi
if printf '%s\n' "$PO" | grep -q '^PROBES pass=5 fail=0 blocked=1$'; then ok "summary line: pass=5 fail=0 blocked=1 (blocked is not counted as a pass)"; else bad "summary line wrong: $(printf '%s\n' "$PO" | tail -1)"; fi
# no `exec` into a service container, no host client: static scans of the scripts under test, each form with a control needle (WF17 TI-G9). The regexes read the forms a reviewer found missing:
# `podman container exec`, `podman --remote exec`, `docker exec`, `docker compose exec`, and a host curl / nfs-cat / nfs-cp / smbget / pg_isready / mc call, the client binary list being the packages of
# build/containers/infra-client/Containerfile (the test asserts that the Containerfile still names every package this list is derived from).
EXECRX='(podman|docker)( +--?[a-z-]+(=[^ ]+)?)*( +container)? +exec( |$)|compose( +[^ ]+)* +exec( |$)'
CLIENTBIN='psql|pg_isready|redis-cli|lftp|smbclient|smbget|mc|curl|nfs-ls|nfs-cp|nfs-cat|nfs-io'
# the scanned set is the AREA's own entry points that probe, round-trip, start or tear down (probe.sh roundtrip.sh run_client.sh up.sh down.sh nfs_attempt.sh nas_readonly_leg.sh blocked_external.sh and the client probe_*/roundtrip_* scripts); a fixture script
# another work package adds to the directory (a server fixture that legitimately execs into its own container) is not a probe and is not judged here
INCL=(--include=probe.sh --include=roundtrip.sh --include=run_client.sh --include=up.sh --include=down.sh --include=nfs_attempt.sh --include=nas_readonly_leg.sh --include=blocked_external.sh --include='probe_*.sh' --include='roundtrip_*.sh')
scanx() { grep -rEn "$EXECRX" "$1" "${INCL[@]}" 2>/dev/null | grep -v '^[^:]*:[0-9]*:[[:space:]]*#'; }
scanh() { grep -rEn "(^|[;&|(]|\\$\\()[[:space:]]*($CLIENTBIN) " "$1" "${INCL[@]}" 2>/dev/null | grep -v '/client/' | grep -v -e 'nfs_terminal_state.sh' -e 'nfs_fallback_state.sh' | grep -v '^[^:]*:[0-9]*:[[:space:]]*#'; }
static_findings() { { scanx "$1"; scanh "$1"; } | head -3; }
for pkg in curl postgresql-client redis-tools lftp smbclient libnfs-utils 'minio/mc'; do grep -q "$pkg" "$TI_REPO/build/containers/infra-client/Containerfile" && ok "the client binary list is derived from a package the Containerfile still installs: $pkg" || bad "the Containerfile no longer names $pkg: the host-client scan list is stale"; done
if [ -z "$(static_findings "$TI_REPO/$SD")" ]; then ok "no script of $SD execs into a container or invokes a protocol client on the host"; else bad "a forbidden call exists: $(static_findings "$TI_REPO/$SD" | head -2)"; fi
mkdir -p "$TI_SCRATCH/needle"
for nd in 'podman exec -it somecontainer psql' 'podman container exec somecontainer psql -c x' 'podman --remote exec somecontainer sh' 'docker exec x y' 'docker compose -f f.yml exec svc sh' 'podman-compose exec svc sh' \
          'curl -s http://127.0.0.1:1/' 'x=$(curl -s http://127.0.0.1:1/)' 'nfs-cat nfs://x/y' 'nfs-cp nfs://x/y /tmp/y' 'smbget smb://x/y' 'pg_isready -h x' 'mc ls x' 'psql -h x' 'redis-cli -h x' 'lftp x' 'smbclient //x/y'; do
  printf '%s\n' "$nd" >"$TI_SCRATCH/needle/probe.sh"
  if [ -n "$(static_findings "$TI_SCRATCH/needle")" ]; then ok "control needle: the static scan sees a planted '$nd'"; else bad "control needle: the static scan is BLIND to '$nd'"; fi
done
printf '# podman exec is forbidden here\necho "a curl-like word: curlew"\n' >"$TI_SCRATCH/needle/probe.sh"
[ -z "$(static_findings "$TI_SCRATCH/needle")" ] && ok "control: a comment and a word that merely starts with 'curl' are NOT findings (no false positive)" || bad "control: the static scan flags a comment / 'curlew'"

FX=$(fixtures "$SD" ""); n=$?
if [ "$n" -eq 0 ]; then ok "fixtures: host probe refused, service-class image refused, carrier FAILs, wrong credentials FAIL (5 protocols)"; else bad "fixtures: $n violated: $(printf '%s' "$FX" | tr '\n' ';' | cut -c1-300)"; fi

# ---------------- paired mutations ----------------
if [ "${PROBES_NO_MUTATIONS:-0}" != 1 ] && [ "${PROBES_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="${PROBES_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  mut() { # mut <name> <file under dir> <old> <new>   builds .audit/scratch/ti-mut-<name>/ (scripts + client) with one change; runs the fixtures against it
    local name=$1 file=$2 old=$3 new=$4 d="$TI_REPO/.audit/scratch/ti-mut-$1" res n
    rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/$SD"/*.sh "$TI_REPO/$SD"/*.py "$d/"; cp -r "$TI_REPO/$SD/client" "$d/client"
    python3 -I - "$d/$file" "$old" "$new" <<'PY' || { bad "mutation $name: anchor missing"; rm -rf -- "${d:?}"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[2]), sys.argv[2])); sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]))
PY
    res=$(fixtures ".audit/scratch/ti-mut-$name" ".audit/scratch/ti-mut-$name/client"); n=$?
    if [ "$n" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$res" | head -1 | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${d:?}"
  }
  mut postgres_tcp_only client/probe_postgres.sh 'out="$(PGPASSWORD="$TI_POSTGRES_PASSWORD" PGCONNECT_TIMEOUT=8 psql -h "$H" -p 5432 -U "$TI_POSTGRES_USER" -d "$TI_POSTGRES_DB" -tAc '"'"'SELECT 1'"'"' 2>&1)"; rc=$?' 'timeout 5 bash -c "exec 3<>/dev/tcp/$H/5432"; rc=$?; out=1'
  mut redis_accepts_any client/probe_redis.sh 'out="$(REDISCLI_AUTH="$TI_REDIS_PASSWORD" timeout 10 redis-cli -h "$H" -p 6379 PING 2>&1)"' 'out="$(timeout 10 redis-cli -h "$H" -p 6379 PING 2>&1)"; [ -n "$out" ] && out=PONG'
  for pr in ftp smb webdav; do
    mut "${pr}_always_pass" "client/probe_$pr.sh" '. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"' '. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"; pass "mutant: no check at all"'
  done
  mut runner_check_removed probe.sh '[ "${TI_PROBE_RUNNER:-container}" = container ] || ti_refuse probe_runner_not_container "TI_PROBE_RUNNER=${TI_PROBE_RUNNER} (a probe never runs on the host or in a service-class image)"' 'true'
  mut class_check_removed run_client.sh '[ "$CLASS" = interpreter ] || ti_refuse client_image_not_interpreter_class "$IMG has class '"'"'$CLASS'"'"'; a probe never runs in a service-class image"
[ "$IMG" = IMG-INFRA-CLIENT ] || ti_refuse not_the_infra_client "every probe client runs in IMG-INFRA-CLIENT, got $IMG (class '"'"'$CLASS'"'"')"' 'true'
  # PS1, PS2, PS3 of the WF17 proof lens: a planted forbidden call in a copy of the scripts; the STATIC scan of that copy must report it
  smut() { # smut <name> <file> <anchor> <planted line>
    local name=$1 file=$2 anchor=$3 line=$4 d="$TI_REPO/.audit/scratch/ti-smut-$1"
    rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/$SD"/*.sh "$d/"; cp -r "$TI_REPO/$SD/client" "$d/client"
    python3 -I - "$d/$file" "$anchor" "$line" <<'PY' || { bad "mutation $name: anchor missing"; rm -rf -- "${d:?}"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: print("anchor count %d" % s.count(sys.argv[2])); sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[2] + "\n" + sys.argv[3]))
PY
    if [ -n "$(static_findings "$d")" ]; then ok "mutation $name CAUGHT ($(static_findings "$d" | head -1 | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${d:?}"
  }
  smut ps1_host_curl_in_probe probe.sh 'HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"' 'curl -s "http://127.0.0.1:${TI_PORT_WEBDAV:-1}/" >/dev/null'
  smut ps2_container_exec_in_roundtrip roundtrip.sh 'HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"' 'podman container exec "$P-postgres" psql -c "select 1"'
  smut ps3_host_nfs_cat_in_up up.sh 'HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"' 'nfs-cat nfs://127.0.0.1/export/x >/dev/null'
  [ -z "${PROBES_EV:-}" ] || cp "$MUTLOG" "$PROBES_EV/probes-mutations.txt"
fi
ti_summary
