#!/usr/bin/env bash
# test_nfs_fallback_state.sh - T134a (RED first). Oracle for scripts/test-infra/nfs_fallback_state.sh, the check that records the terminal state of the owner-host NFS fallback leg.
# Run through `TIC tooling unit`. Oracle strategy (11.4.245): SPECIFIED by T134a: `not_needed` only when the T134 record's `state` field is `pass` AND it proves the kernel mount path (the record cites that
# file's sha256 and its state field); a user-space protocol-only pass is REFUSED nfs_pass_does_not_cover_kernel_mount and recorded `blocked` with the unconfirmed path (WF12 F7); a scratch record holding the structural-impossibility state makes `not_needed` REFUSED `nfs_attempt_not_pass`; a `blocked` claim while the
# attempt passed is refused; `blocked` with reason odg08_unanswered while ODG-08 is unanswered. Paired mutation: a copy that writes `not_needed` without reading the field
# must make the structural-impossibility fixture FAIL (recorded in nfs-fallback-mutation.txt).
# Usage:  test_nfs_fallback_state.sh   (NFSFB_NO_MUTATIONS=1: tests only)  Env: NFSFB_SUT, NFSFB_EV
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SUT_REL="${NFSFB_SUT:-scripts/test-infra/nfs_fallback_state.sh}"
if [ ! -f "$TI_REPO/$SUT_REL" ]; then bad "check absent: $SUT_REL"; ti_summary; exit 1; fi
FX="$TI_REPO/.audit/scratch/nfsfb-fx.$$"; OUTD="$TI_REPO/.audit/out/ti-nfsfb-$$"; mkdir -p "$FX" "$OUTD"
trap 'rm -rf -- "${FX:?}" "${OUTD:?}"; ti_cleanup' EXIT
FXC="/src/.audit/scratch/nfsfb-fx.$$"
# the shape of the record the T134 check really writes for a protocol-only pass (it names what it does not prove), and a pass that DOES prove the kernel path (no producer exists on this host)
echo '{"schema":"nfs-attempt/1","state":"pass","reason":"","does_not_prove":"the application NFS path (syscall.Mount, a kernel NFS client)"}' >"$FX/pass.json"
echo '{"schema":"nfs-attempt/1","state":"pass","reason":"","proves_kernel_mount_path":true}' >"$FX/passkernel.json"
echo '{"schema":"nfs-attempt/1","state":"structural_impossibility","reason":"rootless_cannot_provide_kernel_nfs"}' >"$FX/struct.json"
echo '{"schema":"nfs-attempt/1","state":"blocked","reason":"nfs_client_unverified"}' >"$FX/blocked.json"
echo '{"schema":"nfs-attempt/1"}' >"$FX/nostate.json"
# WF17 TI-E3 / WF17 reviewer E1: records that are NOT what nfs_terminal_state.sh writes are refused attempt_record_malformed, never read leniently
echo '{"schema":"nfs-attempt/1","state":"pass","reason":"","proves_kernel_mount_path":"true"}' >"$FX/e1-string-true.json"       # the string "true" is not true
echo '{"schema":"something-else/9","state":"pass","reason":"","proves_kernel_mount_path":true}' >"$FX/e1-foreign-schema.json"
echo '{"schema":"nfs-attempt/1","state":"pass","reason":"made-up-reason"}' >"$FX/e1-bad-reason.json"
echo '{"schema":"nfs-attempt/1","state":"blocked","reason":"rootless_cannot_provide_kernel_nfs"}' >"$FX/e1-reason-state-mismatch.json"
st() { local sut=$1; shift; rm -f "$OUTD/rec.json"; ti_tic --out "$OUTD" tooling unit -- bash "/src/$sut" "$@" --out /out/rec.json >"$TI_SCRATCH/st.out" 2>"$TI_SCRATCH/st.err"; RC=$?; }
field() { jq -r "$1" "$OUTD/rec.json" 2>/dev/null; }
battery() {
  local sut=$1 n=0 want
  # WF12 F7: a protocol-only pass NEVER waives the owner-host leg: the kernel mount path stays unconfirmed
  st "$sut" --attempt-json "$FXC/pass.json" --state not_needed
  if [ "$RC" -ne 0 ] && grep -q 'reason=nfs_pass_does_not_cover_kernel_mount' "$TI_SCRATCH/st.err" && [ ! -e "$OUTD/rec.json" ]; then :; else echo "FAIL not_needed was written for a user-space protocol pass (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  st "$sut" --attempt-json "$FXC/passkernel.json" --state not_needed; want=$(sha256sum "$FX/passkernel.json" | cut -d' ' -f1)
  if [ "$RC" = 0 ] && [ "$(field .state)" = not_needed ] && [ "$(field .attempt_record.state)" = pass ] && [ "$(field .attempt_record.sha256)" = "$want" ] && [ "$(field .finding)" = D-10 ]; then :; else echo "FAIL a pass that proves the kernel path did not yield not_needed citing its sha256 and state (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  st "$sut" --attempt-json "$FXC/struct.json" --state not_needed
  if [ "$RC" -ne 0 ] && grep -q 'reason=nfs_attempt_not_pass' "$TI_SCRATCH/st.err" && [ ! -e "$OUTD/rec.json" ]; then :; else echo "FAIL not_needed was written for a structural-impossibility record (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  st "$sut" --attempt-json "$FXC/blocked.json" --state not_needed
  if [ "$RC" -ne 0 ] && grep -q 'reason=nfs_attempt_not_pass' "$TI_SCRATCH/st.err"; then :; else echo "FAIL not_needed was written for a blocked record (rc=$RC)"; n=$((n+1)); fi
  st "$sut" --attempt-json "$FXC/struct.json" --state blocked
  if [ "$RC" = 0 ] && [ "$(field .state)" = blocked ] && [ "$(field .reason)" = odg08_unanswered ] && [ "$(field '.owed_to | index("ODG-08")')" = 0 ]; then :; else echo "FAIL a structural-impossibility record did not yield blocked odg08_unanswered (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  st "$sut" --attempt-json "$FXC/pass.json" --state blocked
  if [ "$RC" = 0 ] && [ "$(field .state)" = blocked ] && [ "$(field .reason)" = odg08_unanswered ] && [ "$(field '.owed_to | index("ODG-08")')" = 0 ] && [ "$(field .attempt_record.state)" = pass ] && field .unconfirmed | grep -q 'kernel NFS mount path'; then :; else echo "FAIL a protocol-only pass did not yield blocked odg08_unanswered naming the unconfirmed kernel path (rc=$RC state=$(field .state) unconfirmed=$(field .unconfirmed))"; n=$((n+1)); fi
  st "$sut" --attempt-json "$FXC/passkernel.json" --state blocked
  if [ "$RC" -ne 0 ] && grep -q 'reason=fallback_not_owed' "$TI_SCRATCH/st.err"; then :; else echo "FAIL blocked was written although the attempt proves the kernel path (rc=$RC)"; n=$((n+1)); fi
  st "$sut" --attempt-json "$FXC/absent.json" --state not_needed
  if [ "$RC" -ne 0 ] && grep -q 'reason=attempt_record_missing' "$TI_SCRATCH/st.err"; then :; else echo "FAIL an absent attempt record was not refused (rc=$RC)"; n=$((n+1)); fi
  for f in e1-string-true e1-foreign-schema e1-bad-reason e1-reason-state-mismatch; do
    st "$sut" --attempt-json "$FXC/$f.json" --state not_needed
    if [ "$RC" -ne 0 ] && grep -q 'reason=attempt_record_malformed' "$TI_SCRATCH/st.err" && [ ! -e "$OUTD/rec.json" ]; then :; else echo "FAIL $f was not refused attempt_record_malformed (rc=$RC state=$(field .state))"; n=$((n+1)); fi
  done
  st "$sut" --attempt-json "$FXC/pass.json" --state blocked
  [ "$(field .attempt_record.file)" = ".audit/scratch/nfsfb-fx.$$/pass.json" ] || { echo "FAIL the record's attempt_record.file is '$(field .attempt_record.file)', not the REAL input path"; n=$((n+1)); }
  st "$sut" --attempt-json "$FXC/nostate.json" --state not_needed
  if [ "$RC" -ne 0 ] && grep -q 'reason=attempt_record_malformed' "$TI_SCRATCH/st.err"; then :; else echo "FAIL a record without a state field was not refused (rc=$RC)"; n=$((n+1)); fi
  return "$n"
}
res=$(battery "$SUT_REL"); n=$?
if [ "$n" -eq 0 ]; then ok "all fixtures hold (not_needed on pass; refused on structural / blocked; blocked odg08_unanswered; refusals)"; else bad "$n fixture(s) violated: $(printf '%s' "$res" | tr '\n' ';' | cut -c1-400)"; fi
if [ "${NFSFB_NO_MUTATIONS:-0}" != 1 ] && [ "${NFSFB_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="$TI_SCRATCH/mutations.txt"; : >"$MUTLOG"
  mut() { local name=$1 old=$2 new=$3 dst="$TI_REPO/.audit/scratch/nfsfb-mut-$1.sh" r n
    python3 -I - "$TI_REPO/$SUT_REL" "$dst" "$old" "$new" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[3]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[3]), sys.argv[3])); sys.exit(1)
open(sys.argv[2], "w").write(s.replace(sys.argv[3], sys.argv[4]))
PY
    r=$(battery ".audit/scratch/nfsfb-mut-$name.sh"); n=$?
    if [ "$n" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -f -- "${dst:?}"; }
  mut not_needed_without_reading_state '  not_needed) [ "$STATE" = pass ] || refuse nfs_attempt_not_pass "the T134 record has state '"'"'$STATE'"'"'"' '  not_needed) true'
  mut not_needed_on_protocol_pass '    [ "$KERNEL" = true ] || refuse nfs_pass_does_not_cover_kernel_mount' '    true || refuse nfs_pass_does_not_cover_kernel_mount'
  mut blocked_when_kernel_proven '  blocked) { [ "$STATE" != pass ] || [ "$KERNEL" != true ]; } || refuse fallback_not_owed "the T134 record proves the kernel mount path"' '  blocked) true'
  mut unconfirmed_not_recorded '"listed_in":"T159","unconfirmed":"the application kernel NFS mount path (syscall.Mount): the T134 pass is a user-space protocol round trip only"}' '"listed_in":"T159"}'
  mut schema_and_closed_set_check_dropped 'jq -e '"'"'.schema == "nfs-attempt/1" and ((.state == "pass" and .reason == "")' 'jq -e '"'"'true or ((.state == "pass" and .reason == "")'
  mut provenance_hard_coded 'attempt_record:{file:$afile, state:$st, sha256:$sha}' 'attempt_record:{file:"wp10/nfs-attempt.json", state:$st, sha256:$sha}'
  [ -z "${NFSFB_EV:-}" ] || cp "$MUTLOG" "$NFSFB_EV/nfs-fallback-mutation.txt"
fi
ti_summary
