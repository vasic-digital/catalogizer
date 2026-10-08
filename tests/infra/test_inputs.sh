#!/usr/bin/env bash
# test_inputs.sh - WF17 fix round 5, CLASS D (INPUT) closure test (TI-D1..TI-D12): arguments, environment, paths and encodings of every script of scripts/test-infra and the two entry points that call them.
# One table-driven row per member; each row drives the REAL script. Oracle strategy (11.4.245): SPECIFIED (the input contract in the header of lib.sh) and DERIVED for the dotenv row (python-dotenv is the
# independent reference implementation). A real redis stack is started once for the rows that need a live project (ENV, PATH, the forged RUNP_LOCK).
# Rows: ARGV (every valued option LAST must exit 2 within seconds, never spin: TI-D1) | ENV (the caller's TI_*/COMPOSE_* variables do not beat the per-run env file: TI-D2; a forged RUNP_LOCK does not reach
# run_pinned: TI-D4) | PATH (relative path options resolve against the caller's cwd; credentials never land in a tracked path: TI-D3, TI-D5) | HOOK (test hooks need TI_TEST_MODE=1: TI-D6) | LOCALE (TI-D7) |
# NUM (TI-D8) | SERIAL (TI-D9) | OUT (a requested output that was not written is not exit 0: TI-D10) | DOTENV (TI-D11) | EMPTY (TI-D12).
# Controls (11.4.201(7)(b)): every refusal row has a known-good invocation of the same script that is NOT refused for the same reason.
# Paired mutations: ti_optval gone from a parser; the env scrub gone; the RUNP scrub gone; ti_abs gone; a hook un-gated; LC_ALL=C gone; the numeric guard widened; the newline guard gone; the git-ignore guard gone;
# the empty-value guard gone; the evidence-dir write unchecked; blocked_external back on its own sed reader; identity mutant (must SURVIVE).
# Usage:  test_inputs.sh   (INP_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR, INP_EV
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
. "$(dirname "${BASH_SOURCE[0]}")/mutlib.sh"; MUT_ENV=INP; MUT_SELF="${BASH_SOURCE[0]}"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"; TI_DOWN="$TI_REPO/$SD/down.sh"
for f in up.sh down.sh gen_env.sh run_client.sh probe.sh roundtrip.sh blocked_external.sh nfs_attempt.sh nfs_build.sh nfs_terminal_state.sh nfs_fallback_state.sh nas_readonly_leg.sh dotenv_get.py; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
S="$TI_REPO/$SD"
TI_IDS+=(zznum zzser zzempty zzhook zzloc zzleakchk)   # ids the refusal rows use: a mutant that wrongly accepts one starts a real stack, which the cleanup tears down
no_state() { [ ! -e "$(ti_state "$1")" ] && [ ! -d "$(ti_regdir)/claims/$(ti_project "$1")" ]; }   # nothing was created: no state directory, no claim

# ================= ARGV: every valued option LAST must be refused, not spun on (TI-D1) =================
argv_row() { # argv_row <script rel> <option>...
  local sc=$1 o t0 t1 rc out; shift
  for o in "$@"; do
    t0=$(date +%s%N); out=$(timeout 5 bash "$TI_REPO/$sc" "$o" 2>&1); rc=$?; t1=$(date +%s%N)
    if [ "$rc" = 2 ] && [ $(((t1 - t0) / 1000000)) -lt 3000 ]; then ok "ARGV $sc $o last: exit 2 in $(((t1 - t0) / 1000000)) ms"
    else bad "ARGV $sc $o last: rc=$rc after $(((t1 - t0) / 1000000)) ms (124 = it spun): $(printf '%s' "$out" | tail -1 | cut -c1-100)"; fi
  done
}
argv_row "$SD/up.sh" --build-id --services --seed --timeout --ev-dir
argv_row "$SD/down.sh" --build-id --op-id --outcome --reason
argv_row "$SD/gen_env.sh" --build-id --op-id --env-out --ports-out
argv_row "$SD/run_client.sh" --build-id --out --image
argv_row "$SD/probe.sh" --build-id --services
argv_row "$SD/roundtrip.sh" --build-id --protocol --record --iteration
argv_row "$SD/nas_readonly_leg.sh" --hosts --json-out
argv_row "$SD/blocked_external.sh" --out
argv_row "$SD/nfs_attempt.sh" --ev-dir --client-json --build-id
argv_row "$SD/nfs_terminal_state.sh" --client-json --attempt-json --out
argv_row "$SD/nfs_fallback_state.sh" --attempt-json --state --out
argv_row "$SD/nfs_build.sh" --need
argv_row scripts/setup-test-env.sh --build-id --services
argv_row "$SD/seed_corpus.sh" --seed --out --checksum-file   # the one parser that was already guarded: the row proves the table sees a guarded parser as green
# control: a known-good invocation of the same family with an INVALID value exits 2 quickly (the instrument sees exit 2, so a hang is not a quiet pass)
t0=$(date +%s%N); timeout 5 bash "$S/down.sh" --build-id BAD_ID >/dev/null 2>&1; rc=$?; t1=$(date +%s%N); check "ARGV control: down.sh with an invalid id exits 2 (not 124)" "$rc" 2

# ================= a live project for ENV / PATH rows =================
X=$(ti_new_id); TI_IDS+=("$X"); PX=$(ti_project "$X")
out=$(bash "$S/up.sh" --build-id "$X" --services redis 2>&1); rc=$?; check "up exits 0 (live project for the ENV and PATH rows)" "$rc" 0
[ "$rc" = 0 ] || { echo "  up said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"; ti_summary; exit 1; }
OPX=$(printf '%s\n' "$out" | sed -n 's/^op_id=//p')

# ---- ENV: the caller's environment must not beat the per-run env file (TI-D2) ----
E2=$(ti_new_id); TI_IDS+=("$E2"); PE2=$(ti_project "$E2")
VIC="catalogizer-test-victim$RANDOM$RANDOM"; TI_LABEL_PROJECTS+=("$VIC")   # a name no other run or agent shares: a leaked victim stack of an earlier run can neither satisfy nor fail this row
check "ENV control: the exported variables ARE visible to a child (the instrument sees them)" "$(TI_PROJECT=$VIC bash -c 'env | grep -c "^TI_PROJECT="')" 1
out=$(TI_PROJECT=$VIC TI_PORT_REDIS=6379 TI_REDIS_PASSWORD=SENTINELENV TI_DATA_DIR=/tmp/x COMPOSE_PROJECT_NAME=victim bash "$S/up.sh" --build-id "$E2" --services redis 2>&1); rc=$?
check "ENV: up with hostile TI_*/COMPOSE_* variables exported exits 0" "$rc" 0
CE=$(podman ps -q --filter "label=catalogizer.op_id=$(printf '%s\n' "$out" | sed -n 's/^op_id=//p')" | head -1)
check "ENV: the container's project label names THIS project, not the exported victim" "$(podman inspect --format '{{index .Config.Labels "catalogizer.test_project"}}' "$CE" 2>/dev/null)" "$PE2"
check "ENV: the published redis port is the env file's, not the exported fixed 6379" "$(podman inspect --format '{{range $p,$b := .NetworkSettings.Ports}}{{range $b}}{{.HostPort}}{{end}}{{end}}' "$CE" 2>/dev/null)" "$(ti_val "$E2" TI_PORT_REDIS)"
if bash "$S/probe.sh" --build-id "$E2" --services redis >"$TI_SCRATCH/pe2.txt" 2>&1; then ok "ENV: the redis of the hostile-env start answers with the env FILE's password (not the exported sentinel)"; else bad "ENV: probe failed: $(tail -2 "$TI_SCRATCH/pe2.txt" | tr '\n' ' ' | cut -c1-200)"; fi
check "ENV: no network or container of the exported victim project exists" "$(podman ps -a -q --filter "label=catalogizer.test_project=$VIC" | wc -l)$(podman network exists $VIC_test-network 2>/dev/null && echo net)" 0
ti_down "$E2" >/dev/null 2>&1
# ---- ENV: a forged RUNP_LOCK must not reach run_pinned (TI-D4) ----
REALD=$(python3 -I -c "import yaml;d=yaml.safe_load(open('$TI_REPO/build/containers/images.lock.yaml'));print([i['digest'] for i in d['images'] if i['id']=='IMG-INFRA-CLIENT'][0])")
python3 -I - "$TI_REPO/build/containers/images.lock.yaml" "$TI_SCRATCH/forged.lock.yaml" "$REALD" <<'PY'
import sys
s = open(sys.argv[1]).read(); assert s.count(sys.argv[3]) >= 1
open(sys.argv[2], "w").write(s.replace(sys.argv[3], "sha256:" + "ab" * 32))
PY
RUNP_LOCK="$TI_SCRATCH/forged.lock.yaml" bash "$S/run_client.sh" --build-id "$X" -- sleep 6 >"$TI_SCRATCH/rf.out" 2>&1 & RFP=$!; TI_BG_PIDS+=("$RFP")
cc() { [ "$(podman ps -q --filter "label=catalogizer.test_project=$PX" | wc -l)" -ge 2 ]; }; ti_wait_for 20 cc || true
img=""; for x in $(podman ps -q --filter "label=catalogizer.test_project=$PX"); do i=$(podman inspect --format '{{.Config.Image}}' "$x" 2>/dev/null); case "$i" in *infra-client*) img="$i";; esac; done
check "ENV: with a forged RUNP_LOCK exported the client still runs the digest of the REAL lock" "${img##*@}" "$REALD"
wait "$RFP" 2>/dev/null

# ================= PATH (TI-D3, TI-D5) =================
mkdir -p "$TI_SCRATCH/cwd"
( cd "$TI_SCRATCH/cwd" && bash "$S/roundtrip.sh" --build-id "$X" --protocol redis --record rel/dir --iteration 1 >"$TI_SCRATCH/rt.out" 2>&1 ); rc=$?
check "PATH: a round trip recorded to a RELATIVE --record exits 0" "$rc" 0
check "PATH: the ledger landed under the CALLER's cwd with exactly one record" "$(grep -c . "$TI_SCRATCH/cwd/rel/dir/ledger.jsonl" 2>/dev/null || echo 0)" 1
out=$(bash "$S/nfs_attempt.sh" --ev-dir "$TI_SCRATCH/ev" --client-json /dev/shm/x.json 2>&1); rc=$?
check "PATH: nfs_attempt --client-json outside the checkout is refused path_outside_repo (exit 2)" "$rc$(printf '%s' "$out" | grep -c 'reason=path_outside_repo')" 21
rm -f -- "$TI_REPO/docs/leakchk.env"; TI_FOREIGN_DIRS+=("$TI_REPO/docs/leakchk.env")   # a mutant that drops the check WRITES this tracked-path file: it is removed before the row and at exit, never left in docs/
( cd "$TI_REPO" && bash "$S/gen_env.sh" --build-id zzleakchk --env-out docs/leakchk.env >/dev/null 2>"$TI_SCRATCH/ge.err" ); rc=$?
check "PATH: gen_env --env-out to a tracked, non-ignored path is refused (exit 2)" "$rc" 2
check "PATH: ... and the file was not written" "$([ -e "$TI_REPO/docs/leakchk.env" ] && echo written || echo absent)" absent
rm -f "$TI_REPO/docs/leakchk.env"
bash "$S/gen_env.sh" --build-id zzleakchk --env-out "$TI_SCRATCH/e.env" --ports-out "$TI_SCRATCH/e.env" >/dev/null 2>&1; check "PATH/OUT: gen_env with --env-out == --ports-out is refused (exit 2)" "$?" 2
bash "$S/gen_env.sh" --build-id zzleakchk --env-out "$TI_SCRATCH/e1.env" --ports-out "$TI_SCRATCH/e2.env" >/dev/null 2>&1; check "PATH control: distinct ignored paths are accepted" "$?" 0
check "PATH control: the env file has mode 600 and the ports file holds no credential" "$(stat -c %a "$TI_SCRATCH/e1.env")$(grep -c PASSWORD "$TI_SCRATCH/e2.env")" 6000
SDIR="$TI_SCRATCH/stdir"; mkdir -p "$SDIR"
TI_STATE_DIR="$SDIR" bash "$TI_REPO/scripts/container-build.sh" --validate-only >"$TI_SCRATCH/cbv.out" 2>&1; rc=$?
check "PATH: container-build.sh --validate-only with TI_STATE_DIR relocated exits 0" "$rc" 0
check "PATH: ... and leaves nothing in the relocated state directory" "$(ls -A "$SDIR" | wc -l)" 0

# ================= HOOK (TI-D6) =================
for h in TI_COMPOSE_FILE TI_CORPUS_CACHE_DIR TI_LOCK TI_SCRIPT_DIR TI_PROBE_CLIENT_DIR TI_RT_CLIENT_DIR TI_TEST_SLEEP_BEFORE_RELEASE TI_TEST_SLEEP_CACHE_INSTALL; do
  out=$(env -u TI_TEST_MODE "$h=/nonexistent" bash "$S/down.sh" --build-id zzhook 2>&1); rc=$?
  check "HOOK: $h without TI_TEST_MODE=1 is refused (exit 2 test_hook_outside_test_mode)" "$rc$(printf '%s' "$out" | grep -c "reason=test_hook_outside_test_mode $h")" 21
done
out=$(TI_TEST_MODE=1 TI_PROBE_CLIENT_DIR=scripts/test-infra/client bash "$S/probe.sh" --build-id zzhook --services redis 2>&1); rc=$?
check "HOOK control: with TI_TEST_MODE=1 the hook is honoured (not refused as a hook; the run fails later: no such project)" "$rc$(printf '%s' "$out" | grep -c test_hook_outside_test_mode)" 10

# ================= LOCALE (TI-D7) =================
if locale -a 2>/dev/null | grep -qi '^en_US\.utf-\?8$'; then
  fw=$(LANG=en_US.UTF-8 LC_ALL= bash -c '[[ "２" =~ ^[0-9]+$ ]] && echo accepted || echo refused')
  check "LOCALE control: this host's UTF-8 locale really accepts a fullwidth digit in [0-9]+ (else the rows below are blind)" "$fw" accepted
  out=$(LANG=en_US.UTF-8 LC_ALL= bash "$S/gen_env.sh" --build-id é 2>&1); check "LOCALE: gen_env --build-id é under en_US.UTF-8 exits 2" "$?" 2
  out=$(LANG=en_US.UTF-8 LC_ALL= bash "$S/up.sh" --build-id zzloc --timeout １５ 2>&1); check "LOCALE: up --timeout (fullwidth digits) under en_US.UTF-8 exits 2" "$?" 2
  out=$(LANG=en_US.UTF-8 LC_ALL= bash "$S/roundtrip.sh" --build-id zzloc --protocol redis --iteration ２ 2>&1); check "LOCALE: roundtrip --iteration (fullwidth digit) under en_US.UTF-8 exits 2" "$?" 2
else blocked "LOCALE rows: no en_US.UTF-8 locale on this host (the instrument would be blind)"; fi

# ================= NUM (TI-D8) =================
for v in 08 010 99999999999999999999 0 -5 1e3; do
  bash "$S/up.sh" --build-id zznum --services redis --timeout "$v" >/dev/null 2>&1; check "NUM: up --timeout $v exits 2" "$?" 2
  no_state zznum && ok "NUM: --timeout $v created no state and no claim" || bad "NUM: --timeout $v left state"
done
TI_TIC_RETRIES=abc bash "$S/up.sh" --build-id zznum --services redis >/dev/null 2>&1; check "NUM: TI_TIC_RETRIES=abc exits 2" "$?" 2
TI_OP_BUDGET_S=08 bash "$S/up.sh" --build-id zznum --services redis >/dev/null 2>&1; check "NUM: TI_OP_BUDGET_S=08 exits 2" "$?" 2
bash "$S/nfs_build.sh" --need 08 >/dev/null 2>&1; check "NUM: nfs_build --need 08 exits 2" "$?" 2

# ================= SERIAL (TI-D9) =================
NL=$'\n'
bash "$S/gen_env.sh" --build-id zzser --op-id "op1${NL}TI_PORT_POSTGRES=5432" --env-out "$TI_SCRATCH/s1.env" >/dev/null 2>&1; check "SERIAL: gen_env --op-id with a newline exits 2" "$?" 2
check "SERIAL: ... and wrote no env file" "$([ -e "$TI_SCRATCH/s1.env" ] && echo written || echo absent)" absent
bash "$S/up.sh" --build-id zzser --services redis --seed "two${NL}lines" >/dev/null 2>&1; check "SERIAL: up --seed with a newline exits 2" "$?" 2
bash "$S/up.sh" --build-id zzser --services redis --seed $'bad\xff' >/dev/null 2>&1; check "SERIAL: up --seed with a non-ASCII byte exits 2" "$?" 2
no_state zzser && ok "SERIAL: the refused starts created no state and no claim" || bad "SERIAL: a refused start left state or a claim"
out=$(bash "$S/run_client.sh" --build-id "$X" -- sh -c "echo FIRST${NL}echo SECOND" 2>&1); rc=$?
check "SERIAL: run_client with a command word holding a newline is refused (argv_word_has_newline)" "$rc$(printf '%s' "$out" | grep -c argv_word_has_newline)" 11
out=$(TI_STATE_DIR="$TI_SCRATCH/a b #c" bash "$S/gen_env.sh" --build-id zzser 2>&1); rc=$?; check "SERIAL: a state path holding ' #' is refused (exit 2)" "$rc" 2
out=$(TI_STATE_DIR="$TI_SCRATCH/a:b" bash "$S/gen_env.sh" --build-id zzser 2>&1); rc=$?; check "SERIAL: a state path holding ':' is refused (exit 2)" "$rc" 2
bash "$S/gen_env.sh" --build-id zzser --op-id ok-op-1 --env-out "$TI_SCRATCH/s2.env" --ports-out "$TI_SCRATCH/s2.ports" >/dev/null 2>&1; check "SERIAL control: a plain op id and path are accepted" "$?" 0

# ================= OUT (TI-D10) =================
OI=$(ti_new_id); TI_IDS+=("$OI")
out=$(bash "$S/up.sh" --build-id "$OI" --services redis --ev-dir /proc/no-such-evdir 2>&1); rc=$?
check "OUT: up --ev-dir to an unwritable path is NOT exit 0 (the requested output was not written)" "$([ "$rc" != 0 ] && echo failed || echo claimed-success)" failed
case "$out" in *evidence_not_written*) ok "OUT: the failure names the unwritten evidence";; *) bad "OUT: no evidence_not_written in $(printf '%s' "$out" | tail -2 | tr '\n' ' ' | cut -c1-200)";; esac
bash "$S/blocked_external.sh" --out /proc/forbidden.json >/dev/null 2>&1; check "OUT: blocked_external --out to an unwritable path exits 1" "$?" 1

# ================= DOTENV (TI-D11): python-dotenv is the independent reference =================
if python3 -I -c 'import dotenv' 2>/dev/null; then
  D="$TI_SCRATCH/dot"; mkdir -p "$D"
  printf 'A=1\r\nexport B = two # c\r\nC="fa\\"ke"\nA=3\nD='"'"'x#y'"'"'\nE=a#b\nF=\nSYNOLOGY_SMB_USER=fakeuser\r\nexport SYNOLOGY_SMB_PASSWORD = fakepw\r\nSYNOLOGY_IP_3=192.0.2.3\nSYNOLOGY_IP_4=192.0.2.4 # lab\n' >"$D/mixed.env"
  agree=0; total=0
  for k in A B C D E F SYNOLOGY_SMB_USER SYNOLOGY_SMB_PASSWORD SYNOLOGY_IP_3 SYNOLOGY_IP_4; do
    want=$(python3 -I -c "import sys;from dotenv import dotenv_values as d;v=d(sys.argv[1]).get(sys.argv[2]);print('' if v is None else v)" "$D/mixed.env" "$k")
    got=$(python3 -I "$S/dotenv_get.py" "$D/mixed.env" "$k")
    total=$((total+1)); [ "$want" = "$got" ] && agree=$((agree+1)) || echo "  DOTENV key $k: python-dotenv='$want' dotenv_get='$got'"
  done
  check "DOTENV: dotenv_get.py agrees with python-dotenv on every key of the mixed-dialect file (CRLF, export, spaces, last duplicate, quotes, inline comment)" "$agree/$total" "$total/$total"
  rec=$(TI_ENV_FILE="$D/mixed.env" TI_NFS_FALLBACK=/nonexistent bash "$S/blocked_external.sh")
  check "DOTENV: blocked_external reads the CRLF/export/spaces file as available (the reference reads all four names)" "$(jq -r '.legs[]|select(.leg=="nas_smb_readonly")|.state' <<<"$rec")" available
  printf 'SYNOLOGY_SMB_USER=u\nSYNOLOGY_SMB_PASSWORD=p\nSYNOLOGY_IP_3=192.0.2.3\nSYNOLOGY_IP_4=nas4.local\n' >"$D/hostname.env"
  rec=$(TI_ENV_FILE="$D/hostname.env" TI_NFS_FALLBACK=/nonexistent bash "$S/blocked_external.sh")
  check "DOTENV: a non-IPv4 SYNOLOGY_IP_4 is not 'available' (the leg's client refuses it too): credentials_invalid" "$(jq -r '.legs[]|select(.leg=="nas_smb_readonly")|.state+" "+.reason' <<<"$rec")" "blocked-unavailable credentials_invalid"
else blocked "DOTENV rows: python-dotenv is not installed on this host (no independent reference)"; fi

# ================= EMPTY (TI-D12) =================
for spec in "up.sh --services" "up.sh --seed" "up.sh --ev-dir" "probe.sh --services" "roundtrip.sh --record" "nas_readonly_leg.sh --json-out" "run_client.sh --out"; do
  set -- $spec; sc=$1; op=$2; ex=(); case "$sc" in up.sh|probe.sh|roundtrip.sh|run_client.sh) ex=(--build-id zzempty);; esac; [ "$sc" != roundtrip.sh ] || ex+=(--protocol redis)
  out=$(timeout 10 bash "$S/$sc" "${ex[@]}" "$op" "" 2>&1); rc=$?
  check "EMPTY: $sc $op \"\" exits 2" "$rc" 2
done
no_state zzempty && ok "EMPTY: the refused invocations created no state and no claim" || bad "EMPTY: state or claim left"

bash "$S/down.sh" --build-id "$X" --op-id "$OPX" >/dev/null 2>&1

# ---------------- paired mutations ----------------
if [ "${INP_NO_MUTATIONS:-0}" != 1 ] && [ "${INP_TEST_MUTANT:-0}" != 1 ]; then
  mut_batch_begin "${INP_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"
  mut optval_gone_from_down down.sh '--build-id) ti_optval "$1" $# "${2:-}"; BID=$2; shift 2;;' '--build-id) BID=${2:-}; shift 2;;'
  mut env_scrub_gone lib.sh 'ti_compose() { ti_envscrub; "${TI_ESC[@]}" podman-compose "$@" 9>&-; }' 'ti_compose() { podman-compose "$@" 9>&-; }'
  mut runp_scrub_gone lib.sh '"${TI_ESC[@]}" ${TI_RUNP_PRINT:+RUNP_PRINT_ARGV=1} bash' 'env ${TI_RUNP_PRINT:+RUNP_PRINT_ARGV=1} bash'
  mut abs_gone_from_roundtrip roundtrip.sh '[ -z "$REC" ] || REC="$(ti_abs "$REC")"' ':'
  mut compose_hook_ungated lib.sh 'for _ti_h in TI_COMPOSE_FILE TI_CORPUS_CACHE_DIR' 'for _ti_h in TI_CORPUS_CACHE_DIR'
  mut locale_not_pinned lib.sh 'export LC_ALL=C' ':'
  mut numeric_guard_widened lib.sh 'ti_uint()  { [[ "$1" =~ ^[1-9][0-9]*$ ]]' 'ti_uint()  { [[ "$1" =~ ^[0-9]+$ ]]'
  mut newline_guard_gone run_client.sh '*$'"'"'\n'"'"'*) ti_refuse argv_word_has_newline' '*NOTHINGMATCHES*) ti_refuse argv_word_has_newline'
  mut git_ignore_guard_gone gen_env.sh 'git -C "$TI_ROOT" check-ignore -q -- "$ENVF" 2>/dev/null || ti_refuse output_not_ignored' 'true || ti_refuse output_not_ignored'
  mut empty_value_allowed lib.sh '[ -n "$3" ] || ti_die "$1 needs a non-empty value" 2;' ':'
  mut evidence_write_unchecked up.sh '|| fail_down evidence_not_written "cannot write the requested evidence files to $EVDIR"' '|| true'
  mut blocked_external_own_reader blocked_external.sh 'envval() { [ -r "$ENVF" ] && python3 -I "$HERE/dotenv_get.py" "$ENVF" "$1" 2>/dev/null; }' 'envval() { [ -r "$ENVF" ] && sed -n "s/^$1=//p" "$ENVF" | head -1 | sed -e '"'"'s/^"\(.*\)"$/\1/'"'"'; }'
  mut_id identity_noop_in_gen_env gen_env.sh '[ -n "$OPID" ] || OPID="$P-up"' '[ -n "$OPID" ] || OPID="$P-up"; :'
  mut_batch_end "${INP_EV:+$INP_EV/inputs-mutations.txt}"
fi
ti_summary
