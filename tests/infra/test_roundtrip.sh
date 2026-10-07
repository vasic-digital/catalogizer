#!/usr/bin/env bash
# test_roundtrip.sh <protocol> - T132 (RED first). Real round trip against ONE protocol of a REAL stack, run x3, each run recorded as an `ev/1` record by
# tools/evidence/evrec (scripts/test-infra/roundtrip.sh --record). Protocols: postgres redis ftp smb webdav nfs (the T134 user-space server); minio is BLOCKED (image not obtainable).
# Oracle strategy (11.4.245): SPECIFIED. The expected answers are defined by the protocols (INSERT then SELECT returns the value; a stored file reads back with
# the same sha256; a corpus file's sha256 equals the corpus manifest of the seeder; DELETE then GET is 404) and by an exact STEP count per protocol.
# Control: a sabotaged run (the service host replaced by an unresolvable name) must FAIL: a round trip that cannot fail proves nothing.
# Paired mutations: copies of the client scripts with ONE change (the step runner reports ok for everything / a step removed); the same criteria are
# re-evaluated against each copy and must be violated.
# Usage:  test_roundtrip.sh <protocol>              (via the per-protocol wrappers tests/infra/test_roundtrip_<protocol>.sh)
# Env:    RT_EV  evidence directory that receives the ledger (default scratch); RT_RUNS (default 3); RT_NO_MUTATIONS=1; TI_RT_CLIENT_DIR (RED: an absent directory)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
PROTO="${1:-}"
export TI_ROOT="$TI_REPO"
SD=scripts/test-infra
CDIR="${TI_RT_CLIENT_DIR:-$SD/client}"
case "$PROTO" in
  postgres) SVC=postgres; STEPS=10;; redis) SVC=redis; STEPS=7;; ftp) SVC=ftp; STEPS=10;; smb) SVC=smb; STEPS=10;; webdav) SVC=webdav; STEPS=12;;
  nfs) SVC=nfs; STEPS=11;;
  minio) SVC=""; STEPS=0;;
  *) echo "usage: test_roundtrip.sh postgres|redis|ftp|smb|webdav|nfs|minio" >&2; exit 2;;
esac
RUNS="${RT_RUNS:-3}"
for f in roundtrip.sh run_client.sh up.sh down.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
RT="$TI_REPO/$SD/roundtrip.sh"

if [ "$PROTO" = minio ]; then
  o=$(bash "$RT" --build-id none1 --protocol minio 2>&1); rc=$?
  check "minio round trip is BLOCKED, exit 3 (never 0)" "$rc" 3
  case "$o" in "BLOCKED roundtrip minio reason=image_unavailable"*) blocked "minio round trip BLOCKED: image not obtainable (evidence/wp12/minio-blocked.txt); not a pass";; *) bad "minio is not reported BLOCKED: $o";; esac
  if grep -q 'PASS' <<<"$o"; then bad "the minio output contains a PASS"; else ok "the minio output contains no PASS"; fi
  ti_summary; exit $?
fi
[ -f "$TI_REPO/$CDIR/roundtrip_$PROTO.sh" ] && ok "client script present: $CDIR/roundtrip_$PROTO.sh" || { bad "client script absent: $CDIR/roundtrip_$PROTO.sh"; ti_summary; exit 1; }

ID=$(ti_new_id); TI_IDS+=("$ID")
out=$(bash "$TI_REPO/$SD/up.sh" --build-id "$ID" --services "$SVC" 2>&1); rc=$?; check "up ($SVC) exits 0" "$rc" 0
[ "$rc" = 0 ] || { echo "  up said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"; ti_summary; exit 1; }
EVD="${RT_EV:-$TI_SCRATCH/ev}"; mkdir -p "$EVD"; LEDGER="$EVD/ledger-$PROTO"; rm -rf -- "${LEDGER:?}"; mkdir -p "$LEDGER"

# criteria(<client dir>) prints one line per VIOLATED criterion; the exit status is their count
criteria() {
  local cd_=$1 n=0 o rc
  o=$(TI_RT_CLIENT_DIR="$cd_" bash "$RT" --build-id "$ID" --protocol "$PROTO" 2>&1); rc=$?
  if [ "$rc" = 0 ] && printf '%s\n' "$o" | grep -qx "PASS roundtrip $PROTO steps=$STEPS" && [ "$(printf '%s\n' "$o" | grep -c '^STEP .* ok$')" = "$STEPS" ]; then :; else echo "FAIL the round trip did not PASS with exactly $STEPS ok steps (rc=$rc: $(printf '%s' "$o" | tail -1 | cut -c1-120))"; n=$((n+1)); fi
  local hv="TI_PROBE_HOST_$(echo "$PROTO" | tr a-z A-Z)"
  o=$(env "$hv=ti-no-such-host" TI_RT_FAILFAST=1 TI_RT_CLIENT_DIR="$cd_" bash "$RT" --build-id "$ID" --protocol "$PROTO" 2>&1); rc=$?
  if [ "$rc" -ne 0 ] && ! printf '%s\n' "$o" | grep -q '^PASS roundtrip'; then :; else echo "FAIL a sabotaged round trip (service host unresolvable) did not fail (rc=$rc)"; n=$((n+1)); fi
  return "$n"
}
fx=$(criteria "$CDIR"); n=$?
if [ "$n" -eq 0 ]; then ok "criteria hold: PASS with exactly $STEPS ok steps; a sabotaged run FAILs"; else bad "criteria violated: $fx"; fi

# run x3 through the recorder
for i in $(seq 1 "$RUNS"); do
  o=$(TI_RT_CLIENT_DIR="$CDIR" bash "$RT" --build-id "$ID" --protocol "$PROTO" --record "$LEDGER" --iteration "$i" 2>&1); rc=$?
  check "recorded run $i exits 0" "$rc" 0
  case "$o" in *"exit=0 verdict=pass"*) ok "run $i: recorded as ev/1 with exit 0 and verdict pass";; *) bad "run $i: not recorded as a pass ($(printf '%s' "$o" | tail -2 | tr '\n' ' ' | cut -c1-160))";; esac
  # the step-by-step output of the same round trip (the recorder stores the streams as blobs; this plain run is the readable evidence)
  po=$(TI_RT_CLIENT_DIR="$CDIR" bash "$RT" --build-id "$ID" --protocol "$PROTO" 2>&1); prc=$?
  printf '%s\n' "$po" >"$EVD/roundtrip-$PROTO-run$i.txt"
  case "$po" in *"PASS roundtrip $PROTO steps=$STEPS"*) ok "run $i: PASS roundtrip $PROTO steps=$STEPS (plain run, rc=$prc)";; *) bad "run $i: no PASS line ($(printf '%s' "$po" | tail -2 | tr '\n' ' ' | cut -c1-160))";; esac
done
python3 -I - "$LEDGER/ledger.jsonl" "$RUNS" <<'PY' && ok "ledger: $RUNS ev/1 records, GREEN, exit 0, verdict pass, evidence class runtime, oracle specified" || bad "ledger records are wrong"
import json, sys
l = [json.loads(x) for x in open(sys.argv[1])]; n = int(sys.argv[2])
assert len(l) == n, "records %d != %d" % (len(l), n)
for i, r in enumerate(l, 1):
    assert r["schema"] == "ev/1" and r["polarity"] == "GREEN" and r["exit_status"] == 0 and r["verdict"] == "pass", r
    assert r["evidence_class"] == "runtime" and r["oracle"]["strategy"] == "specified" and r["iteration"] == i, r
    assert r["target_class"] == "shell_script" and len(r["target_fingerprint"]) > 8, r
PY
vo=$(cd /tmp && EV_LEDGER="$LEDGER/ledger.jsonl" EV_ANCHOR="$LEDGER/anchor.jsonl" EV_BLOBS="$LEDGER/blobs" EVREC_REPO_ROOT=/tmp python3 "$TI_REPO/tools/evidence/verify" 2>&1); vrc=$?
check "tools/evidence/verify walks the chain (exit 0)" "$vrc" 0
# state delta on the server side: nothing the round trip created remains
case "$PROTO" in
  ftp|smb|webdav) left=$(find "$(ti_state "$ID")/data" -not -path '*/.deleted/*' \( -name 'up-*.bin' -o -name 'ti-*' \) 2>/dev/null | wc -l); check "server-side state: no uploaded file or collection remains (outside the Samba recycle bin)" "$left" 0
    if [ "$PROTO" = smb ]; then rec=$(find "$(ti_state "$ID")/data/smb/.deleted" -name 'up-*.bin' 2>/dev/null | wc -l); if [ "$rec" -ge "$RUNS" ]; then ok "server-side state: the deleted uploads are in the Samba recycle bin ($rec): the deletions really happened on the server"; else bad "server-side state: recycle bin holds $rec deleted uploads, want >= $RUNS"; fi; fi;;
esac

# ---------------- paired mutations ----------------
if [ "${RT_NO_MUTATIONS:-0}" != 1 ]; then
  MUTLOG="$EVD/roundtrip-$PROTO-mutations.txt"; : >"$MUTLOG"
  mut() { # mut <name> <file> <old> <new>
    local name=$1 file=$2 old=$3 new=$4 d="$TI_REPO/.audit/scratch/ti-rtmut-$PROTO-$1" res n
    rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/$CDIR"/*.sh "$d/"
    python3 -I - "$d/$file" "$old" "$new" <<'PY' || { bad "mutation $name: anchor missing"; rm -rf -- "${d:?}"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[2]), sys.argv[2])); sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]))
PY
    res=$(criteria ".audit/scratch/ti-rtmut-$PROTO-$name"); n=$?
    if [ "$n" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$res" | head -1 | cut -c1-100))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${d:?}"
  }
  mut step_runner_always_ok lib.sh 'step() { local n=$1; shift; local o; STEPS=$((STEPS+1)); if o="$("$@" 2>&1)"; then echo "STEP $n ok"; else BAD=$((BAD+1)); echo "STEP $n FAIL ${o:0:160}"; [ "${TI_RT_FAILFAST:-0}" != 1 ] || { echo "FAIL roundtrip failfast steps=$STEPS failed=$BAD"; exit 1; }; fi; }' 'step() { local n=$1; shift; STEPS=$((STEPS+1)); "$@" >/dev/null 2>&1; echo "STEP $n ok"; }'
  first=$(grep -m1 '^step ' "$TI_REPO/$CDIR/roundtrip_$PROTO.sh")
  mut step_removed "roundtrip_$PROTO.sh" "$first" ':'
  # F8 / RM6 of the WF12 review: the NEGATIVE leg no longer reaches the server (an unreachable host also "fails"); the round trip must not pass on that
  case "$PROTO" in
    postgres) mut rm6_negative_leg_unreachable roundtrip_postgres.sh 'PGPASSWORD=wrong-credential-1 PGCONNECT_TIMEOUT=8 psql -h "$H"' 'PGPASSWORD=wrong-credential-1 PGCONNECT_TIMEOUT=8 psql -h "$H-unreachable"'
                mut negative_dup_not_a_duplicate roundtrip_postgres.sh "q -c \"INSERT INTO ti_rt VALUES (1, 'dup')\" 2>&1" "q -c \"INSERT INTO ti_rt_absent VALUES (1, 'dup')\" 2>&1";;
    ftp)      mut rm6_negative_leg_unreachable roundtrip_ftp.sh '"open -u $TI_FTP_USER,wrong-credential-1 ftp://$H"' '"open -u $TI_FTP_USER,wrong-credential-1 ftp://$H-unreachable"'
              mut not_listed_on_failed_listing roundtrip_ftp.sh "o=\"\$(run 'cls -1' 2>&1)\" || { echo \"the listing failed" "o=\"\$(run 'cls -1 nothing-here' 2>&1)\" || { echo \"the listing failed";;
    smb)      mut rm6_negative_leg_unreachable roundtrip_smb.sh 'timeout 30 smbclient "//$H/testshare" -A "$f"' 'timeout 30 smbclient "//$H-unreachable/testshare" -A "$f"';;
    nfs)      mut rm6_negative_leg_unreachable roundtrip_nfs.sh 'nfs-ls "nfs://$IP/no-such-export"' 'nfs-ls "nfs://$IP-unreachable/no-such-export"';;
  esac
  mut failfast_exits_zero lib.sh 'echo "FAIL roundtrip failfast steps=$STEPS failed=$BAD"; exit 1; }' 'echo "FAIL roundtrip failfast steps=$STEPS failed=$BAD"; exit 0; }'
fi
ti_summary
