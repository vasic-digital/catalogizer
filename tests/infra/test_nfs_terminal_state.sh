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
cat >"$FX/att-server-fail.json" <<EOF
{"round_trips":[{"iteration":1,"ok":false}],"failing_step":{"side":"server","step":"export_data_not_stored","transcript":"$FXC/nfs-ls-version.txt"},"client_version_transcript":"$FXC/nfs-ls-version.txt"}
EOF
cat >"$FX/att-server-fail-notr.json" <<EOF
{"round_trips":[{"iteration":1,"ok":false}],"failing_step":{"side":"server","step":"x"}}
EOF
cat >"$FX/att-client-fail.json" <<EOF
{"round_trips":[{"iteration":1,"ok":false}],"failing_step":{"side":"client","step":"nfs_mount_async"},"client_version_transcript":"$FXC/nfs-ls-version.txt"}
EOF
cat >"$FX/att-pass.json" <<'EOF'
{"round_trips":[{"iteration":1,"ok":true,"record":"RUN-134#1"},{"iteration":2,"ok":true,"record":"RUN-134#2"},{"iteration":3,"ok":true,"record":"RUN-134#3"}]}
EOF
cat >"$FX/att-mixed.json" <<'EOF'
{"round_trips":[{"iteration":1,"ok":true,"record":"RUN-134#1"},{"iteration":2,"ok":false,"record":"RUN-134#2"},{"iteration":3,"ok":true,"record":"RUN-134#3"}]}
EOF
cat >"$FX/att-mixed4.json" <<'EOF'
{"round_trips":[{"iteration":1,"ok":true},{"iteration":2,"ok":true},{"iteration":3,"ok":true},{"iteration":4,"ok":false}]}
EOF
cat >"$FX/att-two.json" <<'EOF'
{"round_trips":[{"iteration":1,"ok":true},{"iteration":2,"ok":true}]}
EOF

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
  if [ "$RC" = 0 ] && [ "$(field .state)" = pass ] && [ "$(field '.round_trips | length')" = 3 ] && field .does_not_prove | grep -q 'application NFS path'; then :; else echo "FAIL 3 passing round trips did not yield pass (rc=$RC state=$(field .state))"; n=$((n+1)); fi
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
  mut transcript_not_required '[ -n "$TR" ] && [ -r "$TR" ] && grep -q '"'"'nfs-ls'"'"' "$TR" || refuse client_version_transcript_missing' 'true'
  [ -z "${NFSTS_EV:-}" ] || cp "$MUTLOG" "$NFSTS_EV/nfs-terminal-mutation.txt"
fi
ti_summary
