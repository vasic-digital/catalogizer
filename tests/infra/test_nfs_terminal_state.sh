#!/usr/bin/env bash
# test_nfs_terminal_state.sh - T134 (RED first). Oracle for scripts/test-infra/nfs_terminal_state.sh, the check that picks the ONE terminal state of the unprivileged
# NFS attempt. The check runs through `TIC tooling unit` (IMG-TESTUTIL), never the bare host. Oracle strategy (11.4.245): SPECIFIED (T134 defines each fixture and its
# expected state) and INVARIANT (a state record cites the sha256 of the client verdict file, recomputed here independently).
# Fixtures: UNVERIFIED -> blocked nfs_client_unverified; AMBIGUOUS -> the same; VERIFIED + failing SERVER step + client transcript -> structural-impossibility record;
# no verdict file -> REFUSED nfs_client_verdict_missing; plus the refusals and the pass state. Paired mutations: copies of the check (the verdict read skipped, a
# client-side failure accepted as structural, the transcript requirement dropped); every copy must FAIL a fixture.
# Usage:  test_nfs_terminal_state.sh           tests, then mutations (NFSTS_NO_MUTATIONS=1: tests only)
# Env:    NFSTS_SUT (repo-relative path of the check; default scripts/test-infra/nfs_terminal_state.sh), NFSTS_EV (directory that receives nfs-terminal-mutation.txt)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SUT_REL="${NFSTS_SUT:-scripts/test-infra/nfs_terminal_state.sh}"
if [ ! -f "$TI_REPO/$SUT_REL" ]; then bad "check absent: $SUT_REL"; ti_summary; exit 1; fi
FX="$TI_REPO/.audit/scratch/nfsts-fx.$$"; OUTD="$TI_REPO/.audit/out/ti-nfsts-$$"
mkdir -p "$FX" "$OUTD"
trap 'rm -rf -- "${FX:?}" "${OUTD:?}"; ti_cleanup' EXIT
FXC="/src/.audit/scratch/nfsts-fx.$$"   # the same fixture directory as the container sees it
clientjson() { printf '{"task":"T106","dependency":"libnfs-utils","verdict":"%s","version":"4.0.0-1"}\n' "$2" >"$FX/$1.json"; }
clientjson verified VERIFIED; clientjson unverified UNVERIFIED; clientjson ambiguous AMBIGUOUS; clientjson maybe MAYBE
printf 'nfs-ls 4.0.0-1 (transcript)\n' >"$FX/nfs-ls-version.txt"
printf 'bash: line 1: nfs-ls: command not found\n' >"$FX/nfs-ls-notfound.txt"   # a shell error that merely MENTIONS nfs-ls is not a version (proof lens PC-07)
SCH='"schema":"nfs-attempt-observed/1"'
# a ledger as evrec writes it: ev/1 entries, seq 1..3, GREEN runtime passes (the independent witness the cited records must resolve in)
mkdir -p "$FX/ledger-nfs" "$FX/ledger-bad/ledger-nfs"
for q in 1 2 3; do printf '{"schema":"ev/1","seq":%s,"verdict":"pass","polarity":"GREEN","evidence_class":"runtime"}\n' "$q"; done >"$FX/ledger-nfs/ledger.jsonl"
printf '{"schema":"ev/1","seq":1,"verdict":"pass","polarity":"GREEN","evidence_class":"runtime"}\n{"schema":"ev/1","seq":2,"verdict":"fail","polarity":"GREEN","evidence_class":"runtime"}\n{"schema":"ev/1","seq":3,"verdict":"pass","polarity":"GREEN","evidence_class":"runtime"}\n' >"$FX/ledger-bad/ledger-nfs/ledger.jsonl"
cat >"$FX/att-server-fail.json" <<EOF
{$SCH,"round_trips":[{"iteration":1,"ok":false}],"failing_step":{"side":"server","step":"export_data_not_stored","transcript":"$FXC/nfs-ls-version.txt"},"client_version_transcript":"$FXC/nfs-ls-version.txt"}
EOF
cat >"$FX/att-server-fail-notr.json" <<EOF
{$SCH,"round_trips":[{"iteration":1,"ok":false}],"failing_step":{"side":"server","step":"x"}}
EOF
cat >"$FX/att-server-fail-notfound.json" <<EOF
{$SCH,"round_trips":[{"iteration":1,"ok":false}],"failing_step":{"side":"server","step":"x"},"client_version_transcript":"$FXC/nfs-ls-notfound.txt"}
EOF
cat >"$FX/att-client-fail.json" <<EOF
{$SCH,"round_trips":[{"iteration":1,"ok":false}],"failing_step":{"side":"client","step":"nfs_mount_async"},"client_version_transcript":"$FXC/nfs-ls-version.txt"}
EOF
R3='{"iteration":1,"ok":true,"record":"ledger-nfs/ledger.jsonl#seq1"},{"iteration":2,"ok":true,"record":"ledger-nfs/ledger.jsonl#seq2"},{"iteration":3,"ok":true,"record":"ledger-nfs/ledger.jsonl#seq3"}'
echo "{$SCH,\"round_trips\":[$R3]}" >"$FX/att-pass.json"
cp "$FX/att-pass.json" "$FX/ledger-bad/att-pass-badledger.json"
echo "{$SCH,\"round_trips\":[{\"iteration\":1,\"ok\":true,\"record\":\"ledger-nfs/ledger.jsonl#seq1\"},{\"iteration\":2,\"ok\":false,\"record\":null},{\"iteration\":3,\"ok\":true,\"record\":\"ledger-nfs/ledger.jsonl#seq2\"}]}" >"$FX/att-mixed.json"
echo "{$SCH,\"round_trips\":[{\"iteration\":1,\"ok\":true},{\"iteration\":2,\"ok\":true},{\"iteration\":3,\"ok\":true},{\"iteration\":4,\"ok\":false}]}" >"$FX/att-mixed4.json"
echo "{$SCH,\"round_trips\":[{\"iteration\":1,\"ok\":true},{\"iteration\":2,\"ok\":true}]}" >"$FX/att-two.json"
# WF17 TI-E3 adversarial records (each MUST be refused with a named reason)
echo "{$SCH,\"round_trips\":[{\"iteration\":1,\"ok\":true},{\"iteration\":2,\"ok\":true},{\"iteration\":3,\"ok\":true}]}" >"$FX/att-e2-ok-without-record.json"                         # WF17 E2: three {"ok":true} with no record
echo '{"schema":"something-else/9","round_trips":[{"iteration":1,"ok":true,"record":"#seq99"},{"iteration":1,"ok":true,"record":"nonexistent"},{"iteration":1,"ok":true}]}' >"$FX/att-b14-foreign-schema.json"   # inputs B14
echo "{$SCH,\"round_trips\":[{\"iteration\":1,\"ok\":true,\"record\":\"ledger-nfs/ledger.jsonl#seq1\"},{\"iteration\":1,\"ok\":true,\"record\":\"ledger-nfs/ledger.jsonl#seq2\"},{\"iteration\":1,\"ok\":true,\"record\":\"ledger-nfs/ledger.jsonl#seq3\"}]}" >"$FX/att-same-iteration.json"
echo "{$SCH,\"round_trips\":[{\"iteration\":1,\"ok\":true,\"record\":\"ledger-nfs/ledger.jsonl#seq1\"},{\"iteration\":2,\"ok\":true,\"record\":\"ledger-nfs/ledger.jsonl#seq2\"},{\"iteration\":3,\"ok\":true,\"record\":\"ledger-nfs/ledger.jsonl#seq99\"}]}" >"$FX/att-seq-absent.json"
echo "{$SCH,\"round_trips\":[{\"iteration\":\"1\",\"ok\":true},{\"iteration\":2,\"ok\":\"true\"},{\"iteration\":3,\"ok\":true}]}" >"$FX/att-types.json"

# st <sut-rel> <args...>: runs the check through TIC; sets RC, STDERR; the record goes to $OUTD/rec.json
st() {
  local sut=$1; shift; rm -f "$OUTD/rec.json"
  ti_tic --out "$OUTD" tooling unit -- bash "/src/$sut" "$@" --out /out/rec.json >"$TI_SCRATCH/st.out" 2>"$TI_SCRATCH/st.err"; RC=$?
}
field() { jq -r "$1" "$OUTD/rec.json" 2>/dev/null; }
# battery <sut-rel>: prints one FAIL line per violated fixture; the exit status is their count
battery() {
  local sut=$1 n=0 want
  st "$sut" --client-json "$FXC/absent.json"
  if [ "$RC" -ne 0 ] && grep -q 'reason=nfs_client_verdict_missing' "$TI_SCRATCH/st.err" && [ ! -e "$OUTD/rec.json" ]; then :; else echo "FAIL no verdict file was not refused nfs_client_verdict_missing (rc=$RC)"; n=$((n+1)); fi
  for v in unverified ambiguous; do
    st "$sut" --client-json "$FXC/$v.json" --attempt-json "$FXC/att-server-fail.json"
    want=$(sha256sum "$FX/$v.json" | cut -d' ' -f1)
    if [ "$RC" = 0 ] && [ "$(field .state)" = blocked ] && [ "$(field .reason)" = nfs_client_unverified ] && [ "$(field .round_trip_attempted)" = false ] && [ "$(field .client_verdict.sha256)" = "$want" ] && [ "$(field .finding)" = D-10 ]; then :; else echo "FAIL $v verdict did not yield blocked nfs_client_unverified citing the file sha256 (rc=$RC state=$(field .state) reason=$(field .reason))"; n=$((n+1)); fi
  done
  st "$sut" --client-json "$FXC/verified.json" --attempt-json "$FXC/att-server-fail.json"
  if [ "$RC" = 0 ] && [ "$(field .state)" = structural_impossibility ] && [ "$(field .reason)" = rootless_cannot_provide_kernel_nfs ] && [ -n "$(field .client_version_transcript_sha256)" ] && field .scope | grep -q 'not FTP, SMB or WebDAV' && [ "$(field .finding)" = D-10 ]; then :; else echo "FAIL VERIFIED + failing server step did not yield the bounded structural-impossibility record (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  st "$sut" --client-json "$FXC/verified.json" --attempt-json "$FXC/att-server-fail-notr.json"
  if [ "$RC" -ne 0 ] && grep -q 'reason=client_version_transcript_missing' "$TI_SCRATCH/st.err" && [ ! -e "$OUTD/rec.json" ]; then :; else echo "FAIL a server failure without the client transcript was not refused (rc=$RC)"; n=$((n+1)); fi
  st "$sut" --client-json "$FXC/verified.json" --attempt-json "$FXC/att-client-fail.json"
  if [ "$RC" -ne 0 ] && grep -q 'reason=client_side_failure_is_an_image_defect' "$TI_SCRATCH/st.err" && [ ! -e "$OUTD/rec.json" ]; then :; else echo "FAIL a client-side failure was recorded as a state instead of refused (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  st "$sut" --client-json "$FXC/verified.json" --attempt-json "$FXC/att-pass.json"
  if [ "$RC" = 0 ] && [ "$(field .state)" = pass ] && [ "$(field '.round_trips | length')" = 3 ] && field .does_not_prove | grep -q 'application NFS path'; then :; else echo "FAIL 3 passing round trips whose records resolve in the ledger did not yield pass (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  # provenance: the record names the REAL repository-relative path of the client file it read, not a hard-coded name
  [ "$(field .client_verdict.file)" = ".audit/scratch/nfsts-fx.$$/verified.json" ] || { echo "FAIL the record's client_verdict.file is '$(field .client_verdict.file)', not the real input path"; n=$((n+1)); }
  # WF17 TI-E3: adversarial records are REFUSED with a named reason and write nothing
  refused() { # refused <fixture> <reason> <label>
    st "$sut" --client-json "$FXC/verified.json" --attempt-json "$FXC/$1"
    if [ "$RC" -ne 0 ] && grep -q "reason=$2" "$TI_SCRATCH/st.err" && [ ! -e "$OUTD/rec.json" ]; then :; else echo "FAIL $3 was not refused $2 (rc=$RC state=$(field .state))"; n=$((n+1)); fi; }
  refused att-e2-ok-without-record.json record_not_in_ledger "E2: three {ok:true} round trips with no record"
  refused att-b14-foreign-schema.json attempt_record_malformed "B14: a foreign schema with three entries all iteration 1 and dangling refs"
  refused att-same-iteration.json attempt_record_malformed "three round trips that all claim iteration 1"
  refused att-seq-absent.json record_not_in_ledger "a record that cites a ledger sequence which does not exist"
  refused att-types.json attempt_record_malformed "a string iteration / a string ok"
  refused att-server-fail-notfound.json client_version_transcript_missing "a transcript that holds 'nfs-ls: command not found' (PC-07)"
  st "$sut" --client-json "$FXC/verified.json" --attempt-json "/src/.audit/scratch/nfsts-fx.$$/ledger-bad/att-pass-badledger.json"
  if [ "$RC" -ne 0 ] && grep -q "reason=record_not_in_ledger" "$TI_SCRATCH/st.err"; then :; else echo "FAIL a cited ledger entry whose verdict is fail was accepted (rc=$RC)"; n=$((n+1)); fi
  st "$sut" --client-json "$FXC/verified.json" --attempt-json "$FXC/att-two.json"
  if [ "$RC" -ne 0 ] && grep -q 'reason=attempt_inconclusive' "$TI_SCRATCH/st.err"; then :; else echo "FAIL two passing round trips were not refused as inconclusive (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  # WF12 F20: three or more round trips of which one FAILED are never `pass`; a mixed run must not read as a green one
  for mx in att-mixed att-mixed4; do
    st "$sut" --client-json "$FXC/verified.json" --attempt-json "$FXC/$mx.json"
    if [ "$RC" -ne 0 ] && grep -q 'reason=attempt_inconclusive' "$TI_SCRATCH/st.err" && [ ! -e "$OUTD/rec.json" ]; then :; else echo "FAIL $mx (a run with a failed round trip) was not refused attempt_inconclusive (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  done
  st "$sut" --client-json "$FXC/maybe.json"
  if [ "$RC" -ne 0 ] && grep -q 'reason=nfs_client_verdict_unreadable' "$TI_SCRATCH/st.err"; then :; else echo "FAIL an unknown verdict value was not refused (rc=$RC)"; n=$((n+1)); fi
  return "$n"
}
res=$(battery "$SUT_REL"); n=$?
if [ "$n" -eq 0 ]; then ok "all fixtures hold (missing / UNVERIFIED / AMBIGUOUS / VERIFIED+server failure / transcript / client failure / pass / inconclusive / unreadable)"; else bad "$n fixture(s) violated: $(printf '%s' "$res" | tr '\n' ';' | cut -c1-400)"; fi
# the check does not run on the bare host: TIC is the only runner used above (a static check of this test file)
if ! grep -nE '^[[:space:]]*(bash|sh) +"?\$TI_REPO/scripts/test-infra/nfs_terminal_state' "${BASH_SOURCE[0]}" | grep -v 'ti_tic' | grep -q .; then ok "the check is only ever started through TIC tooling unit"; else bad "the check is started outside TIC"; fi

if [ "${NFSTS_NO_MUTATIONS:-0}" != 1 ] && [ "${NFSTS_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="$TI_SCRATCH/mutations.txt"; : >"$MUTLOG"
  mut() { # mut <name> <old> <new>
    local name=$1 old=$2 new=$3 dst="$TI_REPO/.audit/scratch/nfsts-mut-$1.sh" r n
    python3 -I - "$TI_REPO/$SUT_REL" "$dst" "$old" "$new" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[3]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[3]), sys.argv[3])); sys.exit(1)
open(sys.argv[2], "w").write(s.replace(sys.argv[3], sys.argv[4]))
PY
    r=$(battery ".audit/scratch/nfsts-mut-$name.sh"); n=$?
    if [ "$n" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -f -- "${dst:?}"
  }
  mut skip_verdict_read 'VERDICT="$(jq -r '"'"'.verdict // empty'"'"' "$CLIENT" 2>/dev/null)"   # VERDICT-READ' 'VERDICT=VERIFIED'
  # F19: models the DANGEROUS behaviour (a failing CLIENT step recorded as structural_impossibility), not a crash: `client` takes the server branch
  mut client_failure_as_structural '  client) refuse client_side_failure_is_an_image_defect "step $(jq -r '"'"'.failing_step.step // "?"'"'"' "$ATT"): fix IMG-INFRA-CLIENT by a reviewed change, never record it as rootless_cannot_provide_kernel_nfs";;
  server)' '  client|server)'
  # RM8 of the WF12 review, verbatim: a run with a failed round trip records `pass`
  mut rm8_pass_with_a_failed_round_trip 'if [ "$ALLN" -ge 3 ] && [ "$OKN" = "$ALLN" ]; then' 'if [ "$ALLN" -ge 3 ] && [ "$OKN" -ge 1 ]; then'
  mut transcript_not_required '[ -n "$TR" ] && [ -r "$TR" ] && grep -qE '"'"'^nfs-ls .*[0-9]+\.[0-9]+'"'"' "$TR" || refuse client_version_transcript_missing' 'true'
  # WF17: the ledger resolution, the schema check and the real-path provenance are load-bearing
  mut ledger_resolution_dropped '    jq -e -s --argjson q "${BASH_REMATCH[1]}" '"'"'any(.[]; .schema == "ev/1" and .seq == $q and .verdict == "pass" and .polarity == "GREEN" and .evidence_class == "runtime")'"'"' "$LEDGER" >/dev/null 2>&1 || refuse record_not_in_ledger "ledger entry seq ${BASH_REMATCH[1]} is absent or not a GREEN runtime pass in $LEDGER"' '    true'
  mut schema_check_dropped 'jq -e '"'"'.schema == "nfs-attempt-observed/1" and (.round_trips | type == "array")' 'jq -e '"'"'(.round_trips | type == "array")'
  mut iteration_check_dropped 'jq -e '"'"'[.round_trips[].iteration] as $i | ($i | length) >= 1 and ($i | sort) == [range(1; ($i | length) + 1)]'"'"' "$ATT" >/dev/null 2>&1 || refuse' 'true || refuse'
  mut provenance_hard_coded 'client_verdict:{file:$cfile, verdict:$verdict, sha256:$sha}' 'client_verdict:{file:"wp11/nfs-client.json", verdict:$verdict, sha256:$sha}'
  [ -z "${NFSTS_EV:-}" ] || cp "$MUTLOG" "$NFSTS_EV/nfs-terminal-mutation.txt"
fi
ti_summary
